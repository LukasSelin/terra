package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The button makes a world and sends the browser to its page, which is there
// to be read, and the home page lists it after.
func TestTheButtonMakesAWorldAndShowsIt(t *testing.T) {
	o := options{Seed: 1, Preset: "valley", W: 32, H: 32, Epochs: -1, Sea: -1, Water: -1, Day: 1}
	srv := httptest.NewServer(newServer(t.TempDir(), o))
	defer srv.Close()
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	res, err := client.Post(srv.URL+"/generate", "application/x-www-form-urlencoded", nil)
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
