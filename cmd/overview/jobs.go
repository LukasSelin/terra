package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Jobs: worlds made in the background.
//
// A press of the button queues a job and sends the browser to the job's
// page, which asks after it every second and goes on to the world's page
// when it is drawn. One worker makes the jobs in the order they came, so a
// globe that takes two minutes holds up nothing but the worlds behind it,
// and a job still waiting can be called off. Jobs live as long as the
// server: a job queued when it stops is gone, and so is its world.

// The job states.
const (
	jobQueued    = "queued"
	jobRunning   = "running"
	jobDone      = "done"
	jobFailed    = "failed"
	jobCancelled = "cancelled"
)

// queueCap is how many jobs may wait at once.
const queueCap = 64

// A job is one world asked for.
type job struct {
	ID    string
	O     options
	Tiles int
	State string
	// Stage is what the running job is doing: see generate.
	Stage                     string
	Err                       string
	Queued, Started, Finished time.Time
}

// jobStatus is a job as its page reads it.
type jobStatus struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Stage string `json:"stage,omitempty"`
	Error string `json:"error,omitempty"`
	// Ahead is how many jobs are queued or running before a queued one.
	Ahead int `json:"ahead"`
	// Elapsed is the seconds a job has run, or waited while queued; Left is
	// the seconds it is guessed still to need, or -1 where there is nothing
	// to guess from.
	Elapsed float64 `json:"elapsed"`
	Left    float64 `json:"left"`
	// Page is the world's page, once it is drawn.
	Page string `json:"page,omitempty"`
	// Tune fills the form in with the job's options.
	Tune string `json:"tune"`
}

// rateKey is what a job's pace is guessed by: a history costs more a tile
// than a drawn map, and a globe more than either.
func rateKey(o options) string {
	t, _ := o.terms()
	return fmt.Sprintf("epochs %v, wrap %v", t.Epochs > 0, t.Wrap)
}

// queue takes a job and hands it to the worker, or says the queue is full.
func (s *server) queue(o options) (*job, bool) {
	t, _ := o.terms()
	j := &job{
		ID:     fmt.Sprintf("%s-%s-seed%d", time.Now().Format("20060102-150405.000"), o.Preset, o.Seed),
		O:      o,
		Tiles:  t.Width * t.Height,
		State:  jobQueued,
		Queued: time.Now(),
	}
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	// Two presses in one millisecond would share a name.
	for s.jobs[j.ID] != nil {
		j.ID += "b"
	}
	select {
	case s.work <- j:
	default:
		return nil, false
	}
	s.jobs[j.ID] = j
	s.order = append(s.order, j.ID)
	return j, true
}

// worker makes the queued jobs one at a time, for as long as the server runs.
func (s *server) worker() {
	for j := range s.work {
		s.run(j)
	}
}

func (s *server) run(j *job) {
	s.one.Lock()
	defer s.one.Unlock()
	if !s.setJob(j, func(j *job) bool {
		if j.State != jobQueued {
			return false
		}
		j.State, j.Started = jobRunning, time.Now()
		return true
	}) {
		return
	}

	out := filepath.Join(s.dir, j.ID)
	land, _, err := generate(j.O, out, func(stage string) {
		s.setJob(j, func(j *job) bool { j.Stage = stage; return true })
	})
	if err == nil {
		var b []byte
		if b, err = json.MarshalIndent(j.O, "", "  "); err == nil {
			err = os.WriteFile(filepath.Join(out, settingsFile), b, 0o644)
		}
	}
	if err != nil {
		os.RemoveAll(out)
		s.setJob(j, func(j *job) bool {
			j.State, j.Err, j.Stage, j.Finished = jobFailed, err.Error(), "", time.Now()
			return true
		})
		return
	}
	s.keep(j.ID, land)
	s.setJob(j, func(j *job) bool {
		j.State, j.Stage, j.Finished = jobDone, "", time.Now()
		s.rates[rateKey(j.O)] = j.Finished.Sub(j.Started).Seconds() / float64(max(j.Tiles, 1))
		return true
	})
}

// setJob changes a job under the lock, and says whether it did.
func (s *server) setJob(j *job, change func(*job) bool) bool {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	return change(j)
}

// status is the job as its page reads it. The caller holds jobsMu.
func (s *server) status(j *job) jobStatus {
	st := jobStatus{ID: j.ID, State: j.State, Stage: j.Stage, Error: j.Err, Left: -1, Tune: "/?" + j.O.values().Encode()}
	now := time.Now()
	switch j.State {
	case jobQueued:
		st.Elapsed = now.Sub(j.Queued).Seconds()
		// By the order they came: two jobs may share a clock reading.
		for _, id := range s.order {
			if id == j.ID {
				break
			}
			if o := s.jobs[id]; o.State == jobRunning || o.State == jobQueued {
				st.Ahead++
			}
		}
	case jobRunning:
		st.Elapsed = now.Sub(j.Started).Seconds()
		if rate, ok := s.rates[rateKey(j.O)]; ok {
			st.Left = max(0, rate*float64(j.Tiles)-st.Elapsed)
		}
	case jobDone:
		st.Elapsed = j.Finished.Sub(j.Started).Seconds()
		st.Page = "/runs/" + j.ID + "/"
	case jobFailed:
		st.Elapsed = j.Finished.Sub(j.Started).Seconds()
	}
	return st
}

// active is the jobs not yet finished, oldest first.
func (s *server) active() []jobStatus {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	var out []jobStatus
	for _, id := range s.order {
		if j := s.jobs[id]; j.State == jobQueued || j.State == jobRunning {
			out = append(out, s.status(j))
		}
	}
	return out
}

// unfinished is whether a run's directory belongs to a job still at work,
// and so is not yet a world to list.
func (s *server) unfinished(id string) bool {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	j := s.jobs[id]
	return j != nil && (j.State == jobQueued || j.State == jobRunning)
}

func (s *server) jobOf(r *http.Request) (jobStatus, bool) {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	j := s.jobs[r.PathValue("id")]
	if j == nil {
		return jobStatus{}, false
	}
	return s.status(j), true
}

// jobPage is the page that waits on a job.
func (s *server) jobPage(w http.ResponseWriter, r *http.Request) {
	st, ok := s.jobOf(r)
	if !ok {
		http.Error(w, "no such job: the server keeps its jobs only while it runs", http.StatusNotFound)
		return
	}
	if st.State == jobDone {
		http.Redirect(w, r, st.Page, http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	jobTmpl.Execute(w, st)
}

// jobState is the job as JSON, for its page to ask after.
func (s *server) jobState(w http.ResponseWriter, r *http.Request) {
	st, ok := s.jobOf(r)
	if !ok {
		jsonError(w, http.StatusNotFound, "no such job")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(st)
}

// cancel calls off a job that has not started. One already running is made
// to the end: nothing in the making of a world can be stopped part way.
func (s *server) cancel(w http.ResponseWriter, r *http.Request) {
	s.jobsMu.Lock()
	j := s.jobs[r.PathValue("id")]
	ok := j != nil && j.State == jobQueued
	if ok {
		j.State, j.Finished = jobCancelled, time.Now()
	}
	s.jobsMu.Unlock()
	if !ok {
		http.Error(w, "only a job still queued can be called off", http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

var jobTmpl = template.Must(template.New("job").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<title>terra · making {{.ID}}</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<noscript><meta http-equiv="refresh" content="2"></noscript>
<style>
:root{--bg:#f6f5f1;--fg:#1d1d1b;--mut:#6b6a64;--card:#fff;--line:#e2e0d8;--bad:#a8321f}
@media (prefers-color-scheme:dark){:root{--bg:#141412;--fg:#ecebe6;--mut:#9a9890;--card:#1e1e1b;--line:#2e2e2a;--bad:#f0826e}}
body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.45 system-ui,sans-serif}
main{max-width:720px;margin:0 auto;padding:28px 16px 60px}
h1{font-size:22px;margin:0 0 4px;overflow-wrap:anywhere} .mut{color:var(--mut)} a{color:inherit}
.card{background:var(--card);border:1px solid var(--line);border-radius:8px;padding:14px 16px;margin-top:18px}
#state{font-size:16px;margin:0 0 6px} #stage{margin:0 0 12px;min-height:1.45em}
.bar{height:6px;border-radius:3px;background:var(--line);overflow:hidden}
.bar i{display:block;height:100%;width:0;background:var(--fg);transition:width .9s linear}
.bar.guess i{width:30%;animation:slide 1.4s ease-in-out infinite alternate}
@keyframes slide{from{margin-left:0}to{margin-left:70%}}
#error{color:var(--bad);white-space:pre-wrap}
button{font:inherit;border:1px solid var(--line);background:var(--card);color:var(--fg);border-radius:6px;padding:4px 10px;cursor:pointer;margin-top:12px}
</style></head><body><main>
<p class="mut"><a href="/">← terra</a></p>
<h1>{{.ID}}</h1>
<div class="card">
 <p id="state">{{.State}}</p>
 <p id="stage" class="mut">{{.Stage}}</p>
 <div class="bar guess" id="bar"><i></i></div>
 <p id="times" class="mut"></p>
 <p id="error" hidden>{{.Error}}</p>
 <p id="again" hidden><a href="{{.Tune}}">Back to the form, filled in with these settings</a></p>
 <form method="post" action="/jobs/{{.ID}}/cancel" id="cancel"{{if ne .State "queued"}} hidden{{end}}><button>Call it off</button></form>
</div>
</main>
<script>
const id={{.ID}};
const $=i=>document.getElementById(i);
const secs=s=>s<60?Math.round(s)+' s':Math.floor(s/60)+' min '+Math.round(s%60)+' s';
function show(st){
 if(st.state==='done'){location.replace(st.page);return true}
 $('state').textContent={queued:'Waiting its turn',running:'Being made',failed:'It could not be made',cancelled:'Called off'}[st.state]||st.state;
 $('stage').textContent=st.state==='queued'?(st.ahead===1?'1 job ahead of it':st.ahead+' jobs ahead of it'):(st.stage||'');
 $('cancel').hidden=st.state!=='queued';
 const bar=$('bar');
 bar.hidden=st.state==='failed'||st.state==='cancelled';
 if(st.state==='running'&&st.left>=0){
  bar.classList.remove('guess');
  bar.firstChild.style.width=Math.min(99,100*st.elapsed/(st.elapsed+st.left))+'%';
 }else bar.classList.add('guess');
 $('times').textContent=st.state==='queued'?'queued '+secs(st.elapsed)+' ago'
  :st.state==='running'?secs(st.elapsed)+' so far'+(st.left>=0?', about '+secs(st.left)+' to go':'')
  :st.state==='failed'?'after '+secs(st.elapsed):'';
 $('error').hidden=!st.error; $('error').textContent=st.error||'';
 $('again').hidden=st.state!=='failed'&&st.state!=='cancelled';
 return st.state==='failed'||st.state==='cancelled';
}
async function poll(){
 try{
  const r=await fetch('/jobs/'+encodeURIComponent(id)+'/status');
  if(r.status===404){$('state').textContent='The server no longer knows this job';$('stage').textContent='It may have been restarted.';$('bar').hidden=true;return}
  if(show(await r.json()))return;
 }catch(e){$('stage').textContent='Cannot reach the server; trying again.'}
 setTimeout(poll,1000);
}
poll();
</script></body></html>
`))
