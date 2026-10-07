package main

import (
	"fmt"
	"html/template"
	"math"
	"os"
	"slices"
	"strings"

	"github.com/LukasSelin/terra"
)

// The couplings page: which pass reads which of the world's fields and
// writes which, the systems that joins and the loops it closes, as
// terra.Couplings declares them; and what this world's features do to one
// another, kind by kind, as its registry has it (terra.Features.Relations).
//
// The declaration is the package's and the same for every world; the
// relations are this world's. Nothing here reads anything the two do not
// say.

// couplingsPage is what the page is made from.
type couplingsPage struct {
	Seed    uint64
	Preset  string
	Systems []string
	// Matrix is, for each system, how many pairs of fields the passes join
	// from it to each other system, in the order of Systems.
	Matrix []couplingRow
	Fields []terra.WorldField
	Passes []terra.Coupling
	Sets   []string
	Loops2 []string
	Loops3 []string
	// Relations is this world's relations, by kind: how many, and the
	// strongest as the why page says it.
	Relations []relationKind
}

type couplingRow struct {
	From   string
	Counts []int
}

type relationKind struct {
	Kind      string
	Count     int
	Strongest string
}

// couplingsOf makes the page for land.
func couplingsOf(land *terra.Land, seed uint64, preset string) couplingsPage {
	pg := couplingsPage{Seed: seed, Preset: preset, Systems: terra.CouplingSystems(), Fields: terra.WorldFields(), Passes: terra.Couplings()}
	system := map[string]string{}
	for _, f := range pg.Fields {
		system[f.Name] = f.System
	}
	at := map[string]int{}
	for k, s := range pg.Systems {
		at[s] = k
	}
	pairs := map[[2]string]bool{}
	for _, c := range pg.Passes {
		for _, r := range c.Reads {
			for _, w := range c.Writes {
				if system[r] != system[w] {
					pairs[[2]string{r, w}] = true
				}
			}
		}
	}
	for _, s := range pg.Systems {
		pg.Matrix = append(pg.Matrix, couplingRow{From: s, Counts: make([]int, len(pg.Systems))})
	}
	for p := range pairs {
		pg.Matrix[at[system[p[0]]]].Counts[at[system[p[1]]]]++
	}
	for _, s := range terra.CouplingSets() {
		pg.Sets = append(pg.Sets, strings.Join(s, ", "))
	}
	for _, l := range terra.CouplingLoops() {
		var parts []string
		for k, f := range l.Fields {
			parts = append(parts, fmt.Sprintf("%s →(%s)", f, strings.Join(l.By[k], ", ")))
		}
		s := strings.Join(parts, " ") + " " + l.Fields[0]
		if len(l.Fields) == 2 {
			pg.Loops2 = append(pg.Loops2, s)
		} else {
			pg.Loops3 = append(pg.Loops3, s)
		}
	}

	g := land.Grid
	if f := g.Features(); f != nil {
		count := map[terra.RelationKind]int{}
		best := map[terra.RelationKind]terra.Relation{}
		var kinds []terra.RelationKind
		for _, r := range f.Relations() {
			if count[r.Kind] == 0 {
				kinds = append(kinds, r.Kind)
			}
			count[r.Kind]++
			if b, ok := best[r.Kind]; !ok || math.Abs(r.Quantity) > math.Abs(b.Quantity) {
				best[r.Kind] = r
			}
		}
		slices.Sort(kinds)
		for _, k := range kinds {
			pg.Relations = append(pg.Relations, relationKind{Kind: k.String(), Count: count[k], Strongest: capital(relationSentence(g, best[k])) + "."})
		}
	}
	return pg
}

var couplingsTmpl = template.Must(template.New("couplings").Parse(`<!doctype html>
<meta charset="utf-8">
<title>Couplings - seed {{.Seed}}, {{.Preset}}</title>
<style>
body{font:15px/1.5 system-ui,sans-serif;max-width:64em;margin:2em auto;padding:0 1em;color:#222}
h1{font-size:1.4em}h2{font-size:1.15em;margin-top:2em}
.meta{color:#666}table{border-collapse:collapse;margin:.5em 0}td,th{border:1px solid #ddd;padding:.2em .5em;text-align:left;vertical-align:top}
th{background:#f4f4f4}td.n{text-align:right}td.z{color:#bbb;text-align:right}li{margin:.15em 0}
</style>
<h1>Couplings: seed {{.Seed}}, the {{.Preset}} preset</h1>
<p class="meta">Which pass of world creation reads which of the world's fields and writes which (terra.Couplings, held to the passes' code by the package's tests), the loops that closes, and what this world's features do to one another.</p>
<h2>System to system</h2>
<p class="meta">How many pairs of fields a pass joins, reading the row's system and writing the column's.</p>
<table><tr><th>from \ to</th>{{range .Systems}}<th>{{.}}</th>{{end}}</tr>
{{range .Matrix}}<tr><th>{{.From}}</th>{{range .Counts}}{{if .}}<td class="n">{{.}}</td>{{else}}<td class="z">·</td>{{end}}{{end}}</tr>
{{end}}</table>
<h2>The fields</h2>
<table><tr><th>System</th><th>Field</th><th>What it is</th></tr>
{{range .Fields}}<tr><td>{{.System}}</td><td>{{.Name}}</td><td>{{.About}}</td></tr>
{{end}}</table>
<h2>The passes</h2>
<table><tr><th>Pass</th><th>Stages</th><th>Reads</th><th>Writes</th></tr>
{{range .Passes}}<tr><td>{{.Pass}}</td><td>{{range $i, $s := .Stages}}{{if $i}}, {{end}}{{$s}}{{end}}</td><td>{{range $i, $s := .Reads}}{{if $i}}, {{end}}{{$s}}{{end}}</td><td>{{range $i, $s := .Writes}}{{if $i}}, {{end}}{{$s}}{{end}}</td></tr>
{{end}}</table>
<h2>The loops</h2>
<p class="meta">Fields that drive one another round some loop:</p>
<ul>{{range .Sets}}<li>{{.}}</li>{{end}}</ul>
<p class="meta">{{len .Loops2}} loops of two fields, each field driving the next through the passes named:</p>
<ul>{{range .Loops2}}<li>{{.}}</li>{{end}}</ul>
<details><summary class="meta">And {{len .Loops3}} of three</summary><ul>{{range .Loops3}}<li>{{.}}</li>{{end}}</ul></details>
<h2>This world's relations</h2>
{{if .Relations}}<table><tr><th>Kind</th><th>How many</th><th>The strongest</th></tr>
{{range .Relations}}<tr><td>{{.Kind}}</td><td class="n">{{.Count}}</td><td>{{.Strongest}}</td></tr>
{{end}}</table>{{else}}<p class="meta">None.</p>{{end}}
`))

// writeCouplings writes the couplings page for land.
func writeCouplings(path string, land *terra.Land, seed uint64, preset string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = couplingsTmpl.Execute(f, couplingsOf(land, seed, preset))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
