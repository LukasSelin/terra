package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// listen serves the page that makes worlds at addr until it fails. Each world
// the button makes is the one the options describe, drawn into its own
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

// home is the button, and the worlds made before, newest first.
func (s *server) home(w http.ResponseWriter, r *http.Request) {
	var runs []string
	if entries, err := os.ReadDir(s.dir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				runs = append(runs, e.Name())
			}
		}
	}
	// The names begin with the time they were made, so they sort by it.
	slices.Sort(runs)
	slices.Reverse(runs)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	homeTmpl.Execute(w, struct {
		O    options
		Runs []string
	}{s.o, runs})
}

// generate makes a world and sends the browser to its page.
func (s *server) generate(w http.ResponseWriter, r *http.Request) {
	s.one.Lock()
	defer s.one.Unlock()
	run := fmt.Sprintf("%s-seed%d", time.Now().Format("20060102-150405.000"), s.o.Seed)
	if _, err := generate(s.o, filepath.Join(s.dir, run)); err != nil {
		http.Error(w, "making the world: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// The directory and not its index.html, which the file server would send
	// back to the directory.
	http.Redirect(w, r, "/runs/"+run+"/", http.StatusSeeOther)
}

var homeTmpl = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<title>terra</title>
<style>
:root{--bg:#f6f5f1;--fg:#1d1d1b;--mut:#6b6a64;--card:#fff;--line:#e2e0d8}
@media (prefers-color-scheme:dark){:root{--bg:#141412;--fg:#ecebe6;--mut:#9a9890;--card:#1e1e1b;--line:#2e2e2a}}
body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.45 system-ui,sans-serif}
main{max-width:720px;margin:0 auto;padding:28px 20px 60px}
h1{font-size:22px;margin:0 0 4px} .mut{color:var(--mut)}
button{font:inherit;font-size:16px;border:0;background:var(--fg);color:var(--bg);border-radius:8px;padding:10px 18px;margin:20px 0;cursor:pointer}
button:disabled{opacity:.6;cursor:progress}
ul{padding:0;list-style:none} li{padding:6px 0;border-top:1px solid var(--line)}
a{color:inherit}
</style></head><body><main>
<h1>terra</h1>
<div class="mut">Preset {{.O.Preset}} · seed {{.O.Seed}}</div>
<form method="post" action="/generate" onsubmit="this.querySelector('button').disabled=true;this.querySelector('button').textContent='Making a world…'">
<button type="submit">Generate world</button>
</form>
{{if .Runs}}<h2 class="mut" style="font-size:14px">Made before</h2>
<ul>{{range .Runs}}<li><a href="/runs/{{.}}/">{{.}}</a> · <a class="mut" href="/runs/{{.}}/why.html">why</a></li>{{end}}</ul>{{end}}
</main></body></html>
`))
