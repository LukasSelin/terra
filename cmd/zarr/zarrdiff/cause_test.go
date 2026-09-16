package main

import (
	"bytes"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// The percentiles read off a tally's histogram are within a bin of the
// true ones, whatever the magnitudes and signs.
func TestTallyPercentiles(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for _, spread := range []float64{1e-9, 1, 1e6} {
		tl := &tally{keep: true}
		var all []float64
		for range 20000 {
			v := spread * math.Exp(rng.NormFloat64()*3)
			if rng.IntN(3) == 0 {
				v = -v
			}
			tl.add(v)
			all = append(all, v)
		}
		slices.Sort(all)
		s := tl.signed()
		for _, p := range []struct {
			p   float64
			got float64
		}{{0.05, s.P5}, {0.5, s.P50}, {0.95, s.P95}} {
			want := all[int(math.Ceil(p.p*float64(len(all))))-1]
			if math.Abs(p.got-want) > 0.011*math.Abs(want) {
				t.Errorf("spread %g: percentile %g is %g, not %g", spread, p.p, p.got, want)
			}
		}
		if s.Min != all[0] || s.Max != all[len(all)-1] || s.Up+s.Down != int64(len(all)) {
			t.Errorf("spread %g: %+v", spread, s)
		}
	}
	// One change: every percentile is it, exactly.
	tl := &tally{keep: true}
	tl.add(-3.25)
	if s := tl.signed(); s.P5 != -3.25 || s.P50 != -3.25 || s.P95 != -3.25 || s.Mean != -3.25 || s.Down != 1 {
		t.Errorf("%+v", s)
	}
}

// meetings is base with book/meeting and features/belt: columns 0-3 a
// collision, in belts 100, 101 and 102 by row; columns 4-7 a rift; the rest
// no meeting and no belt.
func meetings() *store {
	s := base()
	n := s.h * s.w
	s.meeting, s.belt = make([]uint8, n), make([]int32, n)
	for i := range n {
		y, x := i/s.w, i%s.w
		switch {
		case x < 4:
			s.meeting[i], s.belt[i] = 1, int32(100+y%3)
		case x < 8:
			s.meeting[i] = 4
		}
	}
	return s
}

// tweak raises the collision 10 m; with rift, it also lowers the rift by
// its row plus one. It turns tile 0 from Water to Rock.
func tweak(s *store, rift bool) *store {
	for i := range s.height {
		y, x := i/s.w, i%s.w
		switch {
		case x < 4:
			s.height[i] += 10
		case x < 8 && rift:
			s.height[i] -= float64(y + 1)
		}
	}
	s.terrain[0] = 3
	return s
}

func near(got, want float64) bool { return math.Abs(got-want) <= 0.011*math.Abs(want) }

func TestSignedChange(t *testing.T) {
	a, b := meetings().write(t), tweak(meetings(), true).write(t)
	r := diff(t, different, a, b)
	h := array(t, r, "ground/height")
	s := h.Signed
	// 40 tiles up 10, and 4 of each of -1 to -10.
	if s == nil || s.Up != 40 || s.Down != 40 || s.Mean != 2.25 || s.Min != -10 || s.Max != 10 ||
		!near(s.P5, -10) || !near(s.P50, -1) || !near(s.P95, 10) {
		t.Errorf("%+v", s)
	}
	if array(t, r, "tile/terrain").Signed != nil {
		t.Error("codes have a signed change")
	}
	if out := text(t, different, a, b); !strings.Contains(out, "b-a: mean +2.25, 5% -10, 50% -1") || !strings.Contains(out, "40 up, 40 down") {
		t.Errorf("text:\n%s", out)
	}
}

func TestByCause(t *testing.T) {
	a, b := meetings().write(t), tweak(meetings(), true).write(t)
	r := diff(t, different, "-by", "book/meeting", "-by", "features/belt", "-top", "2", a, b)
	h := array(t, r, "ground/height")
	if len(h.By) != 2 {
		t.Fatalf("%+v", h.By)
	}
	m := h.By[0]
	if m.By != "book/meeting" || m.Side != "a" || len(m.Categories) != 3 {
		t.Fatalf("%+v", m)
	}
	none, coll, rift := m.Categories[0], m.Categories[1], m.Categories[2]
	if none.Name != "no_meeting" || none.Tiles != 50 || none.TilesChanged != 0 || none.Signed != nil {
		t.Errorf("%+v", none)
	}
	if coll.Name != "collision" || coll.Tiles != 40 || coll.TilesChanged != 40 || coll.Share != 1 ||
		coll.Signed.Mean != 10 || coll.Signed.P5 != 10 || coll.Signed.Down != 0 {
		t.Errorf("%+v %+v", coll, coll.Signed)
	}
	if rift.Code != 4 || rift.TilesChanged != 40 || rift.Signed.Mean != -5.5 || !near(rift.Signed.P50, -6) {
		t.Errorf("%+v %+v", rift, rift.Signed)
	}

	// A map of ids lists its top categories by tiles changed.
	belt := h.By[1]
	if len(belt.Categories) != 2 || belt.Categories[0].Code != 0 || belt.Categories[0].TilesChanged != 40 ||
		belt.Categories[1].Code != 100 || belt.Categories[1].TilesChanged != 16 || belt.Categories[1].Tiles != 16 ||
		belt.Categories[1].Signed.P95 != 10 || belt.More != 2 || belt.MoreTilesChanged != 24 {
		t.Errorf("%+v", belt)
	}
	// The strata did not change, and a one-dimensional array is not broken down.
	if array(t, r, "strata/top").By != nil || array(t, r, "features/table/kind").By != nil {
		t.Error("a breakdown of an unchanged or unmapped array")
	}
	// The coded array is broken down too, without signed changes.
	if tr := array(t, r, "tile/terrain"); len(tr.By) != 2 || tr.By[0].Categories[1].TilesChanged != 1 || tr.By[0].Categories[1].Signed != nil {
		t.Errorf("%+v", tr.By)
	}

	// Read an element at a time, with the maps' chunks let go as soon as
	// read, the report is the same.
	if small := diff(t, different, "-budget", "1", "-by", "book/meeting", "-by", "features/belt", "-top", "2", a, b); !reflect.DeepEqual(small, r) {
		t.Errorf("at a budget of 1:\n%+v\nnot\n%+v", small, r)
	}

	out := text(t, different, "-by", "book/meeting", a, b)
	for _, line := range []string{"by book/meeting in a", "1 collision", "4 rift", "100.00%", "+10", "-5.5"} {
		if !strings.Contains(out, line) {
			t.Errorf("no %q in\n%s", line, out)
		}
	}

	// From the changed store, where the meetings are gone.
	c := tweak(meetings(), true)
	clear(c.meeting)
	r = diff(t, different, "-by", "book/meeting", "-by-side", "b", a, c.write(t))
	if cats := array(t, r, "ground/height").By[0].Categories; len(cats) != 1 || cats[0].Tiles != 130 || cats[0].TilesChanged != 80 {
		t.Errorf("%+v", cats)
	}
	for _, args := range [][]string{
		{"-by", "ground/height"},       // amounts
		{"-by", "strata/top"},          // not a map
		{"-by", "book/nothing"},        // not there
		{"-by-side", "c"},              // no such side
		{"-by", "features/table/kind"}, // one dimension
	} {
		text(t, failed, append(args, a, b)...)
	}
}

func TestOnlyAndMask(t *testing.T) {
	a, b := meetings().write(t), tweak(meetings(), true).write(t)
	r := diff(t, different, "-only", "ground", "-only", "tile/terrain", a, b)
	if len(r.Arrays) != 2 || r.Arrays[0].Path != "ground/height" || r.Arrays[1].Path != "tile/terrain" {
		t.Errorf("%+v", r.Arrays)
	}
	diff(t, same, "-only", "strata/top", a, b)
	text(t, failed, "-only", "nothing/here", a, b)

	r = diff(t, different, "-mask", "book/meeting=collision", a, b)
	h := array(t, r, "ground/height")
	if h.Tiles != 40 || h.Elements != 40 || h.TilesChanged != 40 || h.Share != 1 || h.Signed.Mean != 10 || h.Signed.Down != 0 {
		t.Errorf("%+v %+v", h, h.Signed)
	}
	if top := array(t, r, "strata/top"); top.Tiles != 40 || top.Elements != 40*beds {
		t.Errorf("%+v", top)
	}
	if !slices.Equal(r.Skipped, []string{"features/table/kind"}) {
		t.Errorf("skipped %v", r.Skipped)
	}
	// Tile 0 is a collision, so its change of terrain is inside.
	if tr := array(t, r, "tile/terrain"); tr.Changed != 1 {
		t.Errorf("%+v", tr)
	}
	// Two masks: a tile must be in both.
	r = diff(t, different, "-mask", "book/meeting=1,4", "-mask", "features/belt=100", a, b)
	if h := array(t, r, "ground/height"); h.Tiles != 16 || h.Signed.Mean != 10 {
		t.Errorf("%+v", h)
	}
	// Outside the collision only the rift changed, all down.
	r = diff(t, different, "-mask", "book/meeting=no meeting,rift", a, b)
	if h := array(t, r, "ground/height"); h.Tiles != 90 || h.TilesChanged != 40 || h.Signed.Up != 0 {
		t.Errorf("%+v", h)
	}
	text(t, failed, "-mask", "book/meeting=subduction", a, b)
	text(t, failed, "-mask", "book/meeting", a, b)
}

func expectations(t *testing.T, json string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "expect.json")
	if err := os.WriteFile(path, []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExpectations(t *testing.T) {
	a := meetings().write(t)
	meant := tweak(meetings(), false).write(t)
	more := tweak(meetings(), true).write(t)
	checks := expectations(t, `{"checks": [
		{"name": "only the collision", "array": "ground/height", "where": {"by": "book/meeting", "not": ["collision"]}, "measure": "share", "max": 0.001},
		{"array": "ground/height", "where": {"by": "book/meeting", "is": [1]}, "measure": "mean", "above": 0},
		{"array": "tile/terrain", "measure": "share", "max": 0.02},
		{"array": "strata/top", "measure": "changed", "max": 0},
		{"array": "tile/terrain", "measure": "code", "from": "water", "to": "rock", "max": 1000},
		{"array": "ground/height", "measure": "p50", "min": 9, "max": 11}
	]}`)

	r := diff(t, same, "-expect", checks, a, meant)
	if len(r.Checks) != 6 {
		t.Fatalf("%+v", r.Checks)
	}
	for i, want := range []float64{0, 10, 1.0 / 130, 0, 1} {
		if c := r.Checks[i]; !c.Pass || c.Value == nil || *c.Value != want {
			t.Errorf("check %d: %+v", i+1, c)
		}
	}

	r = diff(t, unmet, "-expect", checks, a, more)
	if c := r.Checks[0]; c.Pass || *c.Value != 40.0/90 {
		t.Errorf("%+v", c)
	}
	if c := r.Checks[5]; c.Pass {
		t.Errorf("the median of +10 and -1 to -10 passes: %+v", c)
	}
	out := text(t, unmet, "-expect", checks, a, more)
	for _, line := range []string{
		"checks: 4 of 6 hold",
		"FAIL  only the collision: ground/height where book/meeting is not collision share = 0.4444, expected <= 0.001",
		"pass  ground/height where book/meeting is 1 mean = 10, expected > 0",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("no %q in\n%s", line, out)
		}
	}
	// With -only, a check's array is compared still.
	if r := diff(t, unmet, "-only", "strata", "-expect", checks, a, more); len(r.Arrays) != 3 {
		t.Errorf("%+v", r.Arrays)
	}
	// A check that cannot measure fails with a note.
	r = diff(t, unmet, "-expect", expectations(t, `{"checks": [{"array": "tile/terrain", "measure": "mean", "above": 0},
		{"array": "no/such", "measure": "changed", "max": 0}]}`), a, meant)
	if r.Checks[0].Note == "" || r.Checks[1].Note != "in neither store" {
		t.Errorf("%+v", r.Checks)
	}

	for _, bad := range []string{
		`{"checks": [{"array": "ground/height", "measure": "median", "max": 1}]}`,
		`{"checks": [{"array": "ground/height", "measure": "share"}]}`,
		`{"checks": [{"array": "ground/height", "measure": "share", "most": 1}]}`,
		`{"checks": [{"array": "tile/terrain", "measure": "code", "from": "water", "max": 1}]}`,
		`{"checks": [{"array": "tile/terrain", "measure": "code", "from": "water", "to": "sky", "max": 1}]}`,
		`{"checks": [{"array": "ground/height", "where": {"by": "book/meeting", "is": ["subduction"]}, "measure": "share", "max": 1}]}`,
		`{"checks": [{"array": "ground/height", "where": {"by": "book/meeting"}, "measure": "share", "max": 1}]}`,
		`not json`,
	} {
		var out, errs bytes.Buffer
		if got := run(ctx, []string{"-expect", expectations(t, bad), a, meant}, &out, &errs); got != failed {
			t.Errorf("%s exits %d", bad, got)
		}
	}
}
