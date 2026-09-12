package terra

import (
	"github.com/LukasSelin/terra/geom"
	"sync"
	"testing"
)

// The point of a Router is that the map can be read by several at once. Each
// goroutine routing on its own Router over one shared Grid must get exactly
// what it would have got routing alone, and must not touch anything the
// others are touching. Run under -race, this is what says the seam holds.
func TestRoutersShareAGridSafely(t *testing.T) {
	w := NewLandSized(4, 40, 24)
	for _, p := range []geom.Pos{{X: 10, Y: 10}, {X: 11, Y: 10}, {X: 12, Y: 11}} {
		w.Grid.Build(p, blocking)
	}
	for x := 5; x < 35; x++ {
		w.Grid.Build(geom.Pos{X: x, Y: 12}, paving)
	}

	// Every pair a worker will be asked about, and the answer arrived at one
	// at a time on the grid's own router.
	var pairs [][2]geom.Pos
	for y := 2; y < 22; y += 3 {
		for x := 2; x < 38; x += 5 {
			pairs = append(pairs, [2]geom.Pos{{X: x, Y: y}, {X: 38 - x, Y: 22 - y}})
		}
	}
	wantCost := make([]float64, len(pairs))
	wantStep := make([]geom.Pos, len(pairs))
	for i, p := range pairs {
		wantCost[i] = w.Grid.TravelCost(p[0], p[1])
		wantStep[i] = w.Grid.StepToward(p[0], p[1])
	}

	const workers = 8
	gotCost := make([]float64, len(pairs))
	gotStep := make([]geom.Pos, len(pairs))
	var wg sync.WaitGroup
	for k := 0; k < workers; k++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			r := w.Grid.Router()
			for i := k; i < len(pairs); i += workers {
				gotCost[i] = r.TravelCost(pairs[i][0], pairs[i][1])
				gotStep[i] = r.StepToward(pairs[i][0], pairs[i][1])
			}
		}(k)
	}
	wg.Wait()

	for i := range pairs {
		if gotCost[i] != wantCost[i] {
			t.Fatalf("%v: cost %v routed in parallel, %v routed alone", pairs[i], gotCost[i], wantCost[i])
		}
		if gotStep[i] != wantStep[i] {
			t.Fatalf("%v: step %v routed in parallel, %v routed alone", pairs[i], gotStep[i], wantStep[i])
		}
	}
}

// A router's buffers are reused between searches, so a stale answer would
// show up as one search leaking into the next. Ask the same router for a run
// of different routes and check each against a router that has done nothing
// else.
func TestARouterIsNotConfusedByItsLastSearch(t *testing.T) {
	w := NewLandSized(5, 30, 18)
	for x := 4; x < 26; x++ {
		w.Grid.Build(geom.Pos{X: x, Y: 9}, paving)
	}
	reused := w.Grid.Router()
	for y := 1; y < 17; y += 2 {
		for x := 1; x < 29; x += 4 {
			from, to := geom.Pos{X: 1, Y: y}, geom.Pos{X: x, Y: 17 - y}
			fresh := w.Grid.Router()
			if got, want := reused.TravelCost(from, to), fresh.TravelCost(from, to); got != want {
				t.Fatalf("%v->%v: reused router said %v, a fresh one %v", from, to, got, want)
			}
			if got, want := reused.StepToward(from, to), fresh.StepToward(from, to); got != want {
				t.Fatalf("%v->%v: reused router stepped %v, a fresh one %v", from, to, got, want)
			}
		}
	}
}

// On flat open ground every route of the same length costs the same, so
// which of them a walker takes is settled entirely by the tie-break. It
// has to be the straight one. This is not a nicety: the ground remembers
// where people walked, and roads are laid where it is worn, so a tie-break
// that leant in some direction of its own would wear the country in that
// direction and lay the roads along it. The order the eight steps are
// tried in is such a lean - it starts north-west - and what stands between
// that order and the map is the preference for the step toward where the
// walker is actually going. Take it away and nine routes in ten bend off
// the straight line, most of them north-west.
func TestARouteKeepsToTheStraightLineWhenGoingRoundCostsTheSame(t *testing.T) {
	g := NewGrid(80, 80)
	from := geom.Pos{X: 40, Y: 40}
	for _, d := range Dirs {
		to := geom.Pos{X: from.X + d.X*12, Y: from.Y + d.Y*12}
		path := g.Path(from, to)
		if len(path) == 0 {
			t.Fatalf("no way from %v to %v over open grass", from, to)
		}
		if want := g.Toward(from, to); path[0] != want {
			t.Fatalf("going from %v to %v the first step is %v, not %v toward it",
				from, to, path[0], want)
		}
		if len(path) != g.Dist(from, to) {
			t.Fatalf("going from %v to %v takes %d steps; %d is the whole way",
				from, to, len(path), g.Dist(from, to))
		}
	}
}
