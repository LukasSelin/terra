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
)

// The button makes a world and sends the browser to its page, which is there
// to be read, and the home page lists it after.
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
	page := res.Header.Get("Location")
	if !strings.HasPrefix(page, "/runs/") || !strings.HasSuffix(page, "/") {
		t.Fatalf("generate sent the browser to %q, want /runs/<run>/", page)
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
		{Seed: 1 << 63, Preset: "globe", W: 256, H: 128, Epochs: 4, Sea: 0.3, Water: 0, Wrap: true, Scale: 2, Day: 0},
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
	page := res.Header.Get("Location")

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
