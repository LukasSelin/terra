package terra

import (
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// The coupling graph is held to what the passes touch: see couplings.go.
//
// Two ways, because neither alone is enough. The source (see
// couplings_source_test.go) is read pass by pass, so it says which pass of a
// stage reads and writes what, and it reads the passes that a test world
// does not run - a glacial cycle, a drawn map's rock - as surely as those it
// does; but it reads the code's shape and not its types, and a write it
// cannot see is a write it cannot hold. The run makes worlds stage by stage
// and hashes every field of the Grid after each, so it sees every write that
// happens, however it is made; but only stage by stage, and only on the
// worlds it makes. Between them a pass cannot write a field the declaration
// does not say it writes and go unnoticed: the source catches it by pass,
// the run by stage.

// stageRoots is the functions each stage is: the stage's own, and for the
// ground the making of the grid it starts on.
var stageRoots = map[string][]string{
	"ground": {"Land.newGround", "Land.stageGround"},
	"sea":    {"Land.stageSea"},
	"shape":  {"Land.stageShape"},
	"cut":    {"Land.stageCut"},
	"coast":  {"Land.stageCoast"},
	"cover":  {"Land.stageCover"},
}

// fieldOwner is the world field each Grid field belongs to, "" for the
// frame's.
func fieldOwner(t *testing.T) map[string]string {
	t.Helper()
	owner := map[string]string{}
	place := func(g, w string) {
		if was, ok := owner[g]; ok {
			t.Fatalf("Grid field %s is both %q and %q", g, was, w)
		}
		owner[g] = w
	}
	for _, f := range worldFields {
		if !slices.Contains(systems, f.system) {
			t.Errorf("field %s is of system %q, which is not one", f.name, f.system)
		}
		for _, g := range f.grid {
			place(g, f.name)
		}
	}
	for _, g := range frameFields {
		place(g, "")
	}
	return owner
}

// Every field of the Grid, and of a Tile, is one world field's or the
// frame's, and once: a new field has to be placed before the graph can hold
// it.
func TestEveryGridFieldIsPlaced(t *testing.T) {
	owner := fieldOwner(t)
	for f := range gridFields() {
		if _, ok := owner[f]; !ok {
			t.Errorf("Grid field %s is in no world field and not the frame's: place it in couplings.go", f)
		}
	}
	for f := range owner {
		if !gridFields()[f] {
			t.Errorf("couplings.go places %s, which the Grid does not have", f)
		}
	}
}

// worldOf is the world fields the Grid fields of set are, a bare write of
// the tiles being a write of every world field a tile holds.
func worldOf(owner map[string]string, set map[string]bool, bareTiles bool) []string {
	seen := map[string]bool{}
	for f := range set {
		if w := owner[f]; w != "" {
			seen[w] = true
		}
		if f == "Tiles" && bareTiles {
			for g, w := range owner {
				if strings.HasPrefix(g, "Tiles.") && w != "" {
					seen[w] = true
				}
			}
		}
	}
	return ordered(seen)
}

// ordered is the world fields of set in the order of worldFields.
func ordered(set map[string]bool) []string {
	var out []string
	for _, f := range worldFields {
		if set[f.name] {
			out = append(out, f.name)
		}
	}
	return out
}

// passTouch is what one pass's own code reads and writes of the world, and
// the stages it runs in, as the source has them.
type passTouch struct {
	reads, writes, stages []string
}

// sourceTouches reads every declared pass out of the source.
func sourceTouches(t *testing.T) map[string]passTouch {
	t.Helper()
	src, err := readSource()
	if err != nil {
		t.Fatal(err)
	}
	owner := fieldOwner(t)
	stop := map[string]bool{}
	for _, c := range couplings {
		for _, f := range c.funcs {
			if _, ok := src.funcs[f]; !ok {
				t.Fatalf("pass %s is function %s, which the package does not have", c.pass, f)
			}
			stop[f] = true
		}
	}
	reach := map[string]map[string]bool{}
	for s, roots := range stageRoots {
		reach[s] = map[string]bool{}
		src.reach(roots, reach[s])
	}
	out := map[string]passTouch{}
	for _, c := range couplings {
		r, w, _ := src.touched(c.funcs, stop)
		var stages []string
		for _, s := range Stages() {
			for _, f := range c.funcs {
				if reach[s][f] && !slices.Contains(stages, s) {
					stages = append(stages, s)
				}
			}
		}
		out[c.pass] = passTouch{reads: worldOf(owner, r, false), writes: worldOf(owner, w, true), stages: stages}
	}
	return out
}

// reach adds to seen every function from reaches.
func (src *packageSource) reach(from []string, seen map[string]bool) {
	for _, k := range from {
		if seen[k] {
			continue
		}
		sf, ok := src.funcs[k]
		if !ok {
			continue
		}
		seen[k] = true
		src.reach(sf.calls, seen)
	}
}

// Each pass reads and writes what couplings.go declares it does, as its own
// code has it, and runs in the stages declared; and nothing a stage runs
// writes a field of the world outside a declared pass.
func TestTheCouplingsAreWhatThePassesTouch(t *testing.T) {
	got := sourceTouches(t)
	names := map[string]bool{}
	for _, c := range couplings {
		if names[c.pass] {
			t.Errorf("pass %s is declared twice", c.pass)
		}
		names[c.pass] = true
		for _, f := range slices.Concat(c.reads, c.writes) {
			if !slices.ContainsFunc(worldFields, func(w worldField) bool { return w.name == f }) {
				t.Errorf("pass %s couples through %q, which is not a world field", c.pass, f)
			}
		}
		g := got[c.pass]
		for _, d := range []struct {
			what       string
			want, have []string
		}{{"reads", ordered(setOf(c.reads)), g.reads}, {"writes", ordered(setOf(c.writes)), g.writes}, {"runs in", stageOrder(c.stages), g.stages}} {
			if !slices.Equal(d.want, d.have) {
				t.Errorf("pass %s %s %v and is declared to %s %v:\n\t{%q, is(%s), in(%s), reads(%s), writes(%s)},",
					c.pass, d.what, d.have, d.what, d.want,
					c.pass, quoted(c.funcs), quoted(g.stages), quoted(g.reads), quoted(g.writes))
			}
		}
	}

	// The stages' own code, down to the passes.
	src, _ := readSource()
	owner := fieldOwner(t)
	stop := map[string]bool{}
	for _, c := range couplings {
		for _, f := range c.funcs {
			stop[f] = true
		}
	}
	for s, roots := range stageRoots {
		var own []string
		for _, r := range roots {
			if !stop[r] {
				own = append(own, r)
			}
		}
		if _, w, _ := src.touched(own, stop); len(worldOf(owner, w, true)) > 0 {
			t.Errorf("stage %s writes %v outside any pass", s, worldOf(owner, w, true))
		}
	}
}

func setOf(s []string) map[string]bool {
	m := map[string]bool{}
	for _, v := range s {
		m[v] = true
	}
	return m
}

// stageOrder is the stages of s in the order they run.
func stageOrder(s []string) []string {
	var out []string
	for _, n := range Stages() {
		if slices.Contains(s, n) {
			out = append(out, n)
		}
	}
	return out
}

func quoted(s []string) string {
	q := make([]string, len(s))
	for k, v := range s {
		q[k] = fmt.Sprintf("%q", v)
	}
	return strings.Join(q, ", ")
}

// stageWorlds are the worlds the run makes stage by stage: the budget's,
// which between them go through every pass a world made without a glacial
// cycle runs.
var stageWorlds = budgetWorlds

// Every field of the world a stage changes, on every one of stageWorlds, is
// one a pass of that stage declares it writes. The Grid is hashed field by
// field, a tile's fields each on its own, after the grid is made and after
// each stage.
func TestTheStagesWriteWhatTheyDeclare(t *testing.T) {
	owner := fieldOwner(t)
	declared := map[string]map[string]bool{}
	for _, c := range couplings {
		for _, s := range c.stages {
			if declared[s] == nil {
				declared[s] = map[string]bool{}
			}
			for _, w := range c.writes {
				declared[s][w] = true
			}
		}
	}
	seen := map[string]map[string]bool{}
	for _, sw := range stageWorlds {
		terms := sw.terms()
		l := unmade(1, terms)
		g := l.newGround(terms)
		was := hashGrid(g)
		l.generateFrom(g, terms, stageGround, len(stages), func(stage string, _ *Land, g *Grid) {
			now := hashGrid(g)
			for f, h := range now {
				if h == was[f] {
					continue
				}
				w, ok := owner[f]
				if !ok {
					t.Errorf("%s: stage %s changes Grid field %s, which is not placed", sw.name, stage, f)
					continue
				}
				if w == "" {
					continue
				}
				if seen[stage] == nil {
					seen[stage] = map[string]bool{}
				}
				seen[stage][w] = true
				if !declared[stage][w] {
					t.Errorf("%s: stage %s changes %s (Grid field %s), and no pass of it declares it writes it", sw.name, stage, w, f)
				}
			}
			was = now
		})
	}
	for _, s := range Stages() {
		t.Logf("stage %s changed %v", s, ordered(seen[s]))
	}
}

// hashGrid is a hash of each of g's fields, by the name gridFields gives it.
func hashGrid(g *Grid) map[string]uint64 {
	out := map[string]uint64{}
	v := reflect.ValueOf(g).Elem()
	var walk func(v reflect.Value, prefix string)
	walk = func(v reflect.Value, prefix string) {
		t := v.Type()
		for k := range t.NumField() {
			f := t.Field(k)
			if f.Anonymous && f.Type.Kind() == reflect.Struct {
				walk(v.Field(k), prefix)
				continue
			}
			h := fnv.New64a()
			hashValue(h, v.Field(k), map[uintptr]bool{})
			out[prefix+f.Name] = h.Sum64()
		}
	}
	walk(v, "")
	// A tile's fields, each over every tile.
	tiles := v.FieldByName("Tiles")
	tt := reflect.TypeFor[Tile]()
	for k := range tt.NumField() {
		h := fnv.New64a()
		for i := range tiles.Len() {
			hashValue(h, tiles.Index(i).Field(k), nil)
		}
		out["Tiles."+tt.Field(k).Name] = h.Sum64()
	}
	return out
}

type hashWriter interface{ Write([]byte) (int, error) }

// hashValue writes v into h, following pointers once each.
func hashValue(h hashWriter, v reflect.Value, seen map[uintptr]bool) {
	var b [8]byte
	put := func(u uint64) {
		for k := range b {
			b[k] = byte(u >> (8 * k))
		}
		h.Write(b[:])
	}
	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			put(1)
		} else {
			put(0)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		put(uint64(v.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		put(v.Uint())
	case reflect.Float32, reflect.Float64:
		put(math.Float64bits(v.Float()))
	case reflect.String:
		h.Write([]byte(v.String()))
	case reflect.Slice, reflect.Array:
		put(uint64(v.Len()))
		for i := range v.Len() {
			hashValue(h, v.Index(i), seen)
		}
	case reflect.Struct:
		for k := range v.NumField() {
			hashValue(h, v.Field(k), seen)
		}
	case reflect.Pointer:
		if v.IsNil() {
			put(0)
			return
		}
		if seen == nil {
			seen = map[uintptr]bool{}
		}
		if seen[v.Pointer()] {
			return
		}
		seen[v.Pointer()] = true
		put(1)
		hashValue(h, v.Elem(), seen)
	case reflect.Interface:
		if !v.IsNil() {
			hashValue(h, v.Elem(), seen)
		}
	case reflect.Map:
		put(uint64(v.Len()))
	case reflect.Func, reflect.Chan:
		if v.IsNil() {
			put(0)
		} else {
			put(1)
		}
	}
}

// The graph has the loop the history turns on: the rain wears the ground,
// and the ground lifts the air that rains.
func TestTheGraphHasTheRainLoop(t *testing.T) {
	found := false
	for _, l := range CouplingLoops() {
		if slices.Equal(l.Fields, []string{"rain", "height"}) {
			found = slices.Contains(l.By[0], "wear") && slices.Contains(l.By[1], "weather")
		}
	}
	if !found {
		t.Errorf("no loop rain -(wear)-> height -(weather)-> rain among %d", len(CouplingLoops()))
	}
}

// docs/couplings.md is what the declaration makes of it.
func TestCouplingsDoc(t *testing.T) {
	const path = "docs/couplings.md"
	want := CouplingsMarkdown()
	if os.Getenv("TERRA_COUPLINGS") == "write" {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s: %d loops", path, len(CouplingLoops()))
		return
	}
	have, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (TERRA_COUPLINGS=write makes it)", err)
	}
	if string(have) != want {
		t.Errorf("%s is not what couplings.go makes: TERRA_COUPLINGS=write go test -run TestCouplingsDoc -timeout 60m .", path)
	}
}
