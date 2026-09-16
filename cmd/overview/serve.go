package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LukasSelin/terra"
)

// listen serves the page that makes worlds at addr until it fails. The page's
// form starts filled in with the options; each world is drawn into its own
// directory under dir.
func listen(addr, dir string, o options) error {
	fmt.Printf("serving at http://localhost%s, worlds into %s\n", addr, dir)
	return http.ListenAndServe(addr, newServer(dir, o))
}

// A server makes a world when asked and serves every world it has made.
type server struct {
	dir string
	o   options
	// one holds the worlds to one at a time: the namer is the package's,
	// and a world takes every core it is given.
	one sync.Mutex
}

func newServer(dir string, o options) http.Handler {
	s := &server{dir: dir, o: o}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("POST /generate", s.generate)
	mux.Handle("GET /runs/", http.StripPrefix("/runs/", http.FileServer(http.Dir(dir))))
	return mux
}

// settingsFile is where a run keeps the options it was made from, so that
// the home page can say what it is and fill the form in from it.
const settingsFile = "settings.json"

// The most pixels a side of a drawn layer may have when the page asks.
const maxPixels = 16384

// home is the form, filled in with the server's options or with those in the
// query, and the worlds made before, newest first.
func (s *server) home(w http.ResponseWriter, r *http.Request) {
	o, err := s.o, error(nil)
	if len(r.URL.Query()) > 0 {
		o, err = optionsFrom(r.URL.Query())
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	s.page(w, http.StatusOK, o.values(), msg)
}

// generate makes the world the form asks for and sends the browser to its
// page, or shows the form again with what was wrong.
func (s *server) generate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.page(w, http.StatusBadRequest, nil, err.Error())
		return
	}
	o, err := optionsFrom(r.PostForm)
	if err != nil {
		s.page(w, http.StatusUnprocessableEntity, r.PostForm, err.Error())
		return
	}
	if err := check(o); err != nil {
		s.page(w, http.StatusUnprocessableEntity, r.PostForm, err.Error())
		return
	}

	s.one.Lock()
	defer s.one.Unlock()
	run := fmt.Sprintf("%s-%s-seed%d", time.Now().Format("20060102-150405.000"), o.Preset, o.Seed)
	out := filepath.Join(s.dir, run)
	if _, err := generate(o, out); err != nil {
		os.RemoveAll(out)
		status := http.StatusInternalServerError
		if errors.Is(err, terra.ErrTooBig) {
			status = http.StatusUnprocessableEntity
		}
		s.page(w, status, r.PostForm, "making the world: "+err.Error())
		return
	}
	if b, err := json.MarshalIndent(o, "", "  "); err == nil {
		os.WriteFile(filepath.Join(out, settingsFile), b, 0o644)
	}
	// The directory and not its index.html, which the file server would send
	// back to the directory.
	http.Redirect(w, r, "/runs/"+run+"/", http.StatusSeeOther)
}

// check turns away, before any making starts, what the terms would refuse
// and pictures too big to draw.
func check(o options) error {
	t, err := o.terms()
	if err != nil {
		return err
	}
	if err := t.Check(); err != nil {
		return err
	}
	if o.Scale > 0 && max(t.Width, t.Height)*o.Scale > maxPixels {
		return fmt.Errorf("scale: %d pixels a tile makes a %dx%d world %d pixels across, and the most is %d", o.Scale, t.Width, t.Height, max(t.Width, t.Height)*o.Scale, maxPixels)
	}
	return nil
}

// optionsFrom reads options from the form's fields. A field left empty is
// the preset's own, as a flag left off is.
func optionsFrom(f url.Values) (options, error) {
	o := options{Seed: 1, Preset: "valley", Epochs: -1, Sea: -1, Water: -1, Day: 30}
	var errs []error
	field := func(name string) string { return strings.TrimSpace(f.Get(name)) }
	whole := func(name string, to *int, lo, hi int) {
		if v := field(name); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < lo || n > hi {
				errs = append(errs, fmt.Errorf("%s: want a whole number from %d to %d, not %q", name, lo, hi, v))
				return
			}
			*to = n
		}
	}
	number := func(name string, to *float64, lo, hi float64) {
		if v := field(name); v != "" {
			x, err := strconv.ParseFloat(v, 64)
			if err != nil || !(x >= lo && x <= hi) {
				errs = append(errs, fmt.Errorf("%s: want a number from %g to %g, not %q", name, lo, hi, v))
				return
			}
			*to = x
		}
	}

	if v := field("preset"); v != "" {
		o.Preset = v
	}
	if !slices.Contains(presets, o.Preset) {
		errs = append(errs, fmt.Errorf("preset: want one of %s, not %q", strings.Join(presets, ", "), o.Preset))
	}
	if v := field("seed"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			errs = append(errs, fmt.Errorf("seed: want a whole number from 0 up, not %q", v))
		}
		o.Seed = n
	}
	whole("w", &o.W, 1, 1<<16)
	whole("h", &o.H, 1, 1<<16)
	whole("epochs", &o.Epochs, 0, 1024)
	number("sea", &o.Sea, 0, 1)
	number("water", &o.Water, 0, 1e5)
	o.Wrap = field("wrap") != ""
	whole("scale", &o.Scale, 0, 64)
	whole("day", &o.Day, 0, 3650)
	return o, errors.Join(errs...)
}

// values is the options as the form's fields, leaving out those that are
// the preset's own. It is optionsFrom run backwards.
func (o options) values() url.Values {
	v := url.Values{}
	v.Set("preset", o.Preset)
	v.Set("seed", strconv.FormatUint(o.Seed, 10))
	set := func(name string, on bool, s string) {
		if on {
			v.Set(name, s)
		}
	}
	set("w", o.W > 0, strconv.Itoa(o.W))
	set("h", o.H > 0, strconv.Itoa(o.H))
	set("epochs", o.Epochs >= 0, strconv.Itoa(o.Epochs))
	set("sea", o.Sea >= 0, strconv.FormatFloat(o.Sea, 'g', -1, 64))
	set("water", o.Water >= 0, strconv.FormatFloat(o.Water, 'g', -1, 64))
	set("wrap", o.Wrap, "on")
	set("scale", o.Scale > 0, strconv.Itoa(o.Scale))
	set("day", o.Day != 30, strconv.Itoa(o.Day))
	return v
}

var presets = []string{"valley", "ancient", "globe"}

// A preset as the form shows it: the numbers an empty field stands for.
type presetView struct {
	Name, W, H, Epochs, Sea, Water string
	Wrap                           bool
}

// A run is a world made before.
type run struct {
	Name, About string
	// Tune is the home page's query that fills the form in with the run's
	// options, when it kept them.
	Tune string
}

func (s *server) page(w http.ResponseWriter, status int, form url.Values, msg string) {
	var views []presetView
	for _, p := range presets {
		t, _ := options{Preset: p, Epochs: -1, Sea: -1, Water: -1}.terms()
		views = append(views, presetView{p, strconv.Itoa(t.Width), strconv.Itoa(t.Height), strconv.Itoa(t.Epochs),
			strconv.FormatFloat(t.SeaShare, 'g', -1, 64), strconv.FormatFloat(t.Water, 'g', -1, 64), t.Wrap})
	}
	if form == nil {
		form = s.o.values()
	}
	fields := map[string]string{}
	for k := range form {
		fields[k] = form.Get(k)
	}
	if fields["day"] == "" {
		fields["day"] = "30"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	homeTmpl.Execute(w, struct {
		F       map[string]string
		Presets []presetView
		Error   string
		Runs    []run
		Chunk   int
	}{fields, views, msg, s.runs(), terra.ChunkSide})
}

// runs is the worlds made before, newest first.
func (s *server) runs() []run {
	entries, _ := os.ReadDir(s.dir)
	var runs []run
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		r := run{Name: e.Name()}
		if b, err := os.ReadFile(filepath.Join(s.dir, e.Name(), settingsFile)); err == nil {
			var o options
			if json.Unmarshal(b, &o) == nil {
				v := o.values()
				r.Tune = "/?" + v.Encode()
				var about []string
				for _, k := range []string{"preset", "seed", "w", "h", "epochs", "sea", "water", "wrap", "scale", "day"} {
					if v.Has(k) {
						about = append(about, k+" "+v.Get(k))
					}
				}
				r.About = strings.Join(about, " · ")
			}
		}
		runs = append(runs, r)
	}
	// The names begin with the time they were made, so they sort by it.
	slices.SortFunc(runs, func(a, b run) int { return strings.Compare(b.Name, a.Name) })
	return runs
}

var homeTmpl = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<title>terra</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
:root{--bg:#f6f5f1;--fg:#1d1d1b;--mut:#6b6a64;--card:#fff;--line:#e2e0d8;--bad:#a8321f}
@media (prefers-color-scheme:dark){:root{--bg:#141412;--fg:#ecebe6;--mut:#9a9890;--card:#1e1e1b;--line:#2e2e2a;--bad:#f0826e}}
body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.45 system-ui,sans-serif}
main{max-width:720px;margin:0 auto;padding:28px 16px 60px}
h1{font-size:22px;margin:0 0 4px} h2{font-size:14px;margin:28px 0 6px} .mut{color:var(--mut)}
form{background:var(--card);border:1px solid var(--line);border-radius:8px;padding:14px 16px;margin-top:18px}
.fields{display:grid;grid-template-columns:repeat(auto-fill,minmax(150px,1fr));gap:12px 14px}
label{display:flex;flex-direction:column;gap:3px;font-size:13px}
label small{color:var(--mut);font-size:12px}
input,select{font:inherit;color:var(--fg);background:var(--bg);border:1px solid var(--line);border-radius:6px;padding:5px 8px;min-width:0}
.check{flex-direction:row;align-items:center;gap:6px;align-self:end;padding-bottom:6px}
.seed{display:flex;gap:4px} .seed input{flex:1}
button{font:inherit;border:1px solid var(--line);background:var(--card);color:var(--fg);border-radius:6px;padding:4px 8px;cursor:pointer}
button[type=submit]{font-size:16px;border:0;background:var(--fg);color:var(--bg);border-radius:8px;padding:10px 18px;margin-top:16px}
button:disabled{opacity:.6;cursor:progress}
.error{color:var(--bad);white-space:pre-wrap;margin:14px 0 0}
ul{padding:0;list-style:none;margin:0} li{padding:6px 0;border-top:1px solid var(--line)}
a{color:inherit}
</style></head><body><main>
<h1>terra</h1>
<div class="mut">Choose how the world is made. An empty field takes the preset's value, shown greyed.</div>
{{if .Error}}<p class="error" role="alert">{{.Error}}</p>{{end}}
<form method="post" action="/generate" id="make">
<div class="fields">
 <label>Preset<select name="preset" id="preset">{{range .Presets}}<option value="{{.Name}}" data-w="{{.W}}" data-h="{{.H}}" data-epochs="{{.Epochs}}" data-sea="{{.Sea}}" data-water="{{.Water}}" data-wrap="{{.Wrap}}"{{if eq .Name (index $.F "preset")}} selected{{end}}>{{.Name}}</option>{{end}}</select></label>
 <label>Seed<span class="seed"><input name="seed" inputmode="numeric" value="{{index .F "seed"}}" id="seed"><button type="button" id="dice" title="A random seed">🎲</button></span></label>
 <label>Width<input name="w" type="number" min="1" value="{{index .F "w"}}"><small>tiles; a globe's is a multiple of {{.Chunk}}</small></label>
 <label>Height<input name="h" type="number" min="1" value="{{index .F "h"}}"><small>tiles</small></label>
 <label>Epochs<input name="epochs" type="number" min="0" value="{{index .F "epochs"}}"><small>ages of history; 0 draws the map</small></label>
 <label>Sea share<input name="sea" type="number" min="0" max="1" step="any" value="{{index .F "sea"}}"><small>of the ground, 0–1</small></label>
 <label>Water<input name="water" type="number" min="0" step="any" value="{{index .F "water"}}"><small>metres over the map; 0 floods by sea share</small></label>
 <label>Day<input name="day" type="number" min="0" max="3650" value="{{index .F "day"}}"><small>whose weather is drawn</small></label>
 <label>Scale<input name="scale" type="number" min="0" max="64" value="{{index .F "scale"}}" placeholder="auto"><small>pixels a tile</small></label>
 <label class="check"><input name="wrap" type="checkbox" id="wrap"{{if index .F "wrap"}} checked{{end}}>Wrap east to west</label>
</div>
<button type="submit">Generate world</button>
</form>
{{if .Runs}}<h2 class="mut">Made before</h2>
<ul>{{range .Runs}}<li><a href="/runs/{{.Name}}/">{{.Name}}</a> · <a class="mut" href="/runs/{{.Name}}/why.html">why</a>{{if .Tune}} · <a class="mut" href="{{.Tune}}">tune from this</a><br><span class="mut">{{.About}}</span>{{end}}</li>{{end}}</ul>{{end}}
</main>
<script>
const preset=document.getElementById('preset'), form=document.getElementById('make');
function placeholders(){
 const d=preset.selectedOptions[0].dataset;
 for(const k of ['w','h','epochs','sea','water'])form.elements[k].placeholder=d[k];
 // A globe always wraps; leaving one gives the box back as it was.
 const wrap=document.getElementById('wrap'), forced=d.wrap==='true';
 if(forced&&!wrap.disabled){wrap.dataset.was=wrap.checked;wrap.checked=true}
 if(!forced&&wrap.disabled)wrap.checked=wrap.dataset.was==='true';
 wrap.disabled=forced;
}
preset.onchange=placeholders; placeholders();
document.getElementById('dice').onclick=()=>{document.getElementById('seed').value=Math.floor(Math.random()*4294967296)};
form.onsubmit=()=>{const b=form.querySelector('button[type=submit]');b.disabled=true;b.textContent='Making a world…'};
</script></body></html>
`))
