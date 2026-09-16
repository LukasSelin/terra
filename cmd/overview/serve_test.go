package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LukasSelin/terra"
)

// The button queues a world and sends the browser to the job's page, which
// goes on to the world's page once it is made; that is there to be read, and
// the home page lists it after.
func TestTheButtonMakesAWorldAndShowsIt(t *testing.T) {
	srv := httptest.NewServer(newServer(t.TempDir(), options{Preset: "valley"}))
	defer srv.Close()
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	form := url.Values{"preset": {"valley"}, "seed": {"7"}, "w": {"32"}, "h": {"24"}, "epochs": {""}, "sea": {"0.2"}, "day": {"1"}}
	res, err := client.PostForm(srv.URL+"/generate", form)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("generate answered %s, want 303 See Other", res.Status)
	}
	page := made(t, srv, res)
	if !strings.HasPrefix(page, "/runs/") || !strings.HasSuffix(page, "/") {
		t.Fatalf("the job went on to %q, want /runs/<run>/", page)
	}

	for _, p := range []string{page, page + "why.html", page + "terrain.png"} {
		res, err := client.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Errorf("GET %s answered %s", p, res.Status)
		}
	}

	res, err = client.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `href="`+page+`"`) {
		t.Errorf("the home page does not list the world at %s", page)
	}
	if !strings.Contains(string(body), "preset valley · seed 7 · w 32 · h 24 · sea 0.2 · day 1") {
		t.Errorf("the home page does not say what the world was made from:\n%s", body)
	}
}

// What the form cannot make is turned back with the reason, and nothing is
// made.
func TestTheFormTurnsAwayWhatCannotBeMade(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(newServer(dir, options{Preset: "valley"}))
	defer srv.Close()
	for _, c := range []struct {
		form url.Values
		says string
	}{
		{url.Values{"preset": {"moon"}}, "preset"},
		{url.Values{"seed": {"-3"}}, "seed"},
		{url.Values{"w": {"wide"}}, "w:"},
		{url.Values{"sea": {"1.5"}}, "sea"},
		{url.Values{"preset": {"globe"}, "w": {"100"}}, "multiple of"},
		{url.Values{"w": {"4096"}, "scale": {"8"}}, "scale"},
		{url.Values{"wetness": {"0"}}, "wetness"},
		{url.Values{"woods": {"sometimes"}}, "woods"},
	} {
		res, err := srv.Client().PostForm(srv.URL+"/generate", c.form)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(string(body), c.says) {
			t.Errorf("%v answered %s, want 422 saying %q", c.form, res.Status, c.says)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) > 0 {
		t.Errorf("turned-away forms made %d runs", len(entries))
	}
}

// The form's fields read back as the options they were written from, so
// "tune from this" fills the form in with what a world was made from.
func TestTheFormReadsBackWhatItWrote(t *testing.T) {
	for _, o := range []options{
		{Seed: 1, Preset: "valley", Epochs: -1, Sea: -1, Water: -1, Day: 30},
		{Seed: 1 << 63, Preset: "globe", W: 256, H: 128, Epochs: 4, Sea: 0.3, Water: 0, Wrap: true, Wetness: 1.5, Woods: "tuned", Growth: "climate", Glacial: true, Scale: 2, Day: 0},
	} {
		got, err := optionsFrom(o.values())
		if err != nil || got != o {
			t.Errorf("%+v read back as %+v, %v", o, got, err)
		}
	}
}

// A world is only made when asked: a GET of /generate makes nothing.
func TestGenerateWantsAPost(t *testing.T) {
	srv := httptest.NewServer(newServer(t.TempDir(), options{Preset: "valley"}))
	defer srv.Close()
	res, err := srv.Client().Get(srv.URL + "/generate")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /generate answered %s, want 405", res.Status)
	}
}

// A tile on a run's map answers for itself, and gives the same answer from a
// world made again as from the one kept.
func TestATileSaysWhyItIsSo(t *testing.T) {
	dir := t.TempDir()
	h := newServer(dir, options{Preset: "valley"})
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	form := url.Values{"preset": {"valley"}, "seed": {"3"}, "w": {"32"}, "h": {"24"}, "day": {"1"}}
	res, err := client.PostForm(srv.URL+"/generate", form)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	page := made(t, srv, res)

	ask := func(query string) (int, string) {
		t.Helper()
		res, err := client.Get(srv.URL + page + "tile?" + query)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(body)
	}

	status, kept := ask("x=5&y=7")
	if status != http.StatusOK {
		t.Fatalf("tile answered %d: %s", status, kept)
	}
	var place whyPlace
	if err := json.Unmarshal([]byte(kept), &place); err != nil {
		t.Fatal(err)
	}
	if place.Pos.X != 5 || place.Pos.Y != 7 || place.Terrain == "" || len(place.Aspects) != 4 {
		t.Errorf("tile answered %+v, want tile (5, 7) with its terrain and four aspects", place)
	}
	if len(place.Aspects[0].Sentences) == 0 {
		t.Errorf("the tile gave no reason for its height: %+v", place)
	}

	// Let the world go, and ask again.
	h.keptMu.Lock()
	h.kept = nil
	h.keptMu.Unlock()
	if status, again := ask("x=5&y=7"); status != http.StatusOK || again != kept {
		t.Errorf("the world made again answered %d:\n%s\nwant\n%s", status, again, kept)
	}

	for _, c := range []struct {
		path string
		want int
	}{
		{page + "tile?x=32&y=0", http.StatusBadRequest},
		{page + "tile?x=a&y=0", http.StatusBadRequest},
		{"/runs/nothing/tile?x=0&y=0", http.StatusNotFound},
		{"/runs/.hidden/tile?x=0&y=0", http.StatusNotFound},
	} {
		res, err := client.Get(srv.URL + c.path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != c.want {
			t.Errorf("GET %s answered %s, want %d", c.path, res.Status, c.want)
		}
	}
}

// made follows a press of the button: the job it queued, asked after until
// it is done. It is the page of the world made.
func made(t *testing.T, srv *httptest.Server, res *http.Response) string {
	t.Helper()
	job := res.Header.Get("Location")
	if res.StatusCode != http.StatusSeeOther || !strings.HasPrefix(job, "/jobs/") {
		t.Fatalf("generate answered %s to %q, want 303 to /jobs/<id>", res.Status, job)
	}
	for deadline := time.Now().Add(2 * time.Minute); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		st := statusOf(t, srv, job)
		switch st.State {
		case jobDone:
			return st.Page
		case jobFailed, jobCancelled:
			t.Fatalf("job %s: %s %s", job, st.State, st.Error)
		}
	}
	t.Fatalf("job %s was not made in two minutes", job)
	return ""
}

func statusOf(t *testing.T, srv *httptest.Server, job string) jobStatus {
	t.Helper()
	res, err := srv.Client().Get(srv.URL + job + "/status")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var st jobStatus
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	return st
}

// Jobs wait their turn behind the one being made, say how many are ahead of
// them, and one still waiting can be called off and makes nothing.
func TestJobsWaitTheirTurnAndCanBeCalledOff(t *testing.T) {
	dir := t.TempDir()
	h := newServer(dir, options{Preset: "valley"})
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	// Hold the worker off, as a world being made would.
	h.one.Lock()
	var jobs []string
	for _, seed := range []string{"1", "2", "3"} {
		res, err := client.PostForm(srv.URL+"/generate", url.Values{"seed": {seed}, "w": {"24"}, "h": {"16"}, "day": {"0"}})
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		jobs = append(jobs, res.Header.Get("Location"))
	}
	for k, job := range jobs {
		if st := statusOf(t, srv, job); st.State != jobQueued || st.Ahead != k {
			t.Errorf("job %d is %s with %d ahead, want queued with %d", k, st.State, st.Ahead, k)
		}
	}
	res, err := client.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(body), "Being made") || !strings.Contains(string(body), `href="`+jobs[2]+`"`) {
		t.Errorf("the home page does not list the jobs waiting")
	}

	res, err = client.Post(srv.URL+jobs[1]+"/cancel", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Errorf("calling off a queued job answered %s", res.Status)
	}
	if st := statusOf(t, srv, jobs[2]); st.Ahead != 1 {
		t.Errorf("the last job has %d ahead once the middle one is called off, want 1", st.Ahead)
	}
	h.one.Unlock()

	for _, k := range []int{0, 2} {
		res, _ := client.Get(srv.URL + jobs[k])
		res.Body.Close()
		made(t, srv, &http.Response{StatusCode: http.StatusSeeOther, Header: http.Header{"Location": {jobs[k]}}})
	}
	if st := statusOf(t, srv, jobs[1]); st.State != jobCancelled {
		t.Errorf("the called-off job is %s", st.State)
	}
	res, err = client.Post(srv.URL+jobs[0]+"/cancel", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusConflict {
		t.Errorf("calling off a job already made answered %s, want 409", res.Status)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Errorf("%d runs made, want the 2 not called off", len(entries))
	}
	// A job made sends its page on to the world.
	res, err = client.Get(srv.URL + jobs[0])
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if loc := res.Header.Get("Location"); res.StatusCode != http.StatusSeeOther || !strings.HasPrefix(loc, "/runs/") {
		t.Errorf("a made job's page answered %s to %q, want 303 to its world", res.Status, loc)
	}
}

// A world that cannot be made fails its job with the reason, and leaves no
// run behind.
func TestAJobThatFailsSaysWhy(t *testing.T) {
	dir := t.TempDir()
	h := newServer(dir, options{Preset: "valley"})
	j, ok := h.queue(options{Seed: 1, Preset: "valley", W: 40000, H: 40000, Epochs: -1, Sea: -1, Water: -1})
	if !ok {
		t.Fatal("the queue is full")
	}
	for deadline := time.Now().Add(time.Minute); ; time.Sleep(20 * time.Millisecond) {
		h.jobsMu.Lock()
		state, msg := j.State, j.Err
		h.jobsMu.Unlock()
		if state == jobFailed {
			if !strings.Contains(msg, "memory") {
				t.Errorf("the job failed saying %q, want it to be about memory", msg)
			}
			break
		}
		if state == jobDone || time.Now().After(deadline) {
			t.Fatalf("the job is %s, want failed", state)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) > 0 {
		t.Errorf("a failed job left %d runs", len(entries))
	}
}

// The settings for the climate and the cover reach the world: a wetter one
// rains more, and each lands on the terms of its name.
func TestTheClimateSettingsReachTheWorld(t *testing.T) {
	o := options{Seed: 5, Preset: "valley", W: 48, H: 32, Epochs: -1, Sea: -1, Water: -1, Day: 0}
	wet := o
	wet.Wetness, wet.Woods, wet.Growth, wet.Glacial = 2, "climate", "tuned", true
	tm, err := wet.terms()
	if err != nil {
		t.Fatal(err)
	}
	if tm.Wetness != 2 || tm.Woods != terra.ByClimate || tm.Growth != terra.Tuned || !tm.Glacial {
		t.Errorf("the terms are %+v, want wetness 2, woods by climate, growth tuned, glacial", tm)
	}
	if bad := (options{Preset: "valley", Woods: "sometimes"}); func() error { _, err := bad.terms(); return err }() == nil {
		t.Error("an unknown woods rule made terms")
	}

	rain := func(o options) float64 {
		land, _, _, err := makeWorld(o, nil)
		if err != nil {
			t.Fatal(err)
		}
		return measure(land).LandRain
	}
	o.Glacial, wet.Glacial = false, false
	wet.Woods, wet.Growth = "", ""
	if dry, wetter := rain(o), rain(wet); !(wetter > 1.5*dry) {
		t.Errorf("rain on land is %.0f mm at wetness 2 against %.0f mm at 1, want well over half again", wetter, dry)
	}
}
