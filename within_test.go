package terra

import (
	"github.com/LukasSelin/terra/geom"
	"math"
	"math/rand"
	"testing"
)

// A search told how far its walker will go answers the same for every
// destination nearer than that, says there is no way to any destination at
// least that far, and opens fewer tiles finding either out - fewer still
// with the landmarks to tell it early that the rest is too far.
func TestAWalkingLimitStopsASearchShort(t *testing.T) {
	w := NewLand(11, DefaultTerms())
	g := w.Grid
	rng := rand.New(rand.NewSource(11))
	type pair struct {
		from, to geom.Pos
		load     float64
	}
	var pairs []pair
	for len(pairs) < 400 {
		p := pair{
			from: geom.Pos{X: rng.Intn(g.W), Y: rng.Intn(g.H)},
			to:   geom.Pos{X: rng.Intn(g.W), Y: rng.Intn(g.H)},
		}
		if rng.Intn(2) == 0 {
			p.load = 1
		}
		if p.from != p.to {
			pairs = append(pairs, p)
		}
	}
	const limit = 20.0
	r := g.Router()
	check := func(stage string) (limited int) {
		t.Helper()
		free, near, far := 0, 0, 0
		for _, p := range pairs {
			r.Reset()
			want := r.Carrying(p.load).TravelCost(p.from, p.to)
			wantPath := r.Carrying(p.load).Path(p.from, p.to)
			free += r.Work
			r.Reset()
			got := r.Carrying(p.load).Within(limit).TravelCost(p.from, p.to)
			gotPath := r.Carrying(p.load).Within(limit).Path(p.from, p.to)
			limited += r.Work
			switch {
			case want < limit:
				near++
				if got != want {
					t.Fatalf("%s: %v -> %v costs %v within %v, %v without", stage, p.from, p.to, got, limit, want)
				}
				if len(gotPath) != len(wantPath) {
					t.Fatalf("%s: %v -> %v has a %d-step way within %v, %d without", stage, p.from, p.to, len(gotPath), limit, len(wantPath))
				}
			default:
				far++
				if !math.IsInf(got, 1) || len(gotPath) != 0 {
					t.Fatalf("%s: %v -> %v costs %v and has a %d-step way within %v, but costs %v", stage, p.from, p.to, got, len(gotPath), limit, want)
				}
			}
		}
		t.Logf("%s: %d near, %d far; %d tiles opened within the limit, %d without", stage, near, far, limited, free)
		if limited >= free {
			t.Errorf("%s: the limit did not save any opening", stage)
		}
		return limited
	}
	plain := check("straight line")
	g.RefreshLandmarks(0)
	guided := check("landmarks")
	if guided >= plain {
		t.Errorf("the landmarks did not make the limited searches cheaper: %d against %d tiles", guided, plain)
	}
}

// A limit holds for one route and no longer, and a route read back from a
// survey respects it too.
func TestAWalkingLimitIsSpentByOneRoute(t *testing.T) {
	g := NewGrid(40, 10)
	r := g.Router()
	a, b := geom.Pos{X: 2, Y: 5}, geom.Pos{X: 30, Y: 5}
	if c := r.Within(10).TravelCost(a, b); !math.IsInf(c, 1) {
		t.Fatalf("a 28-tile walk within 10 costs %v, want no way", c)
	}
	if c := r.TravelCost(a, b); c != 28 {
		t.Fatalf("the same walk with the limit spent costs %v, want 28", c)
	}
	r.Survey(a, 0, 100)
	if c := r.Within(10).TravelCost(a, b); !math.IsInf(c, 1) {
		t.Fatalf("read off a survey, a 28-tile walk within 10 costs %v, want no way", c)
	}
	if c := r.TravelCost(a, b); c != 28 {
		t.Fatalf("read off a survey with the limit spent, the walk costs %v, want 28", c)
	}
}
