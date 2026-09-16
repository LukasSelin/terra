package main

import (
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
