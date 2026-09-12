package terra

import (
	"github.com/LukasSelin/terra/geom"
	"math"
	"testing"
)

func TestDistanceIsShortestRoundTheCylinder(t *testing.T) {
	g := NewGrid(100, 20)
	a, b := geom.Pos{X: 2, Y: 5}, geom.Pos{X: 97, Y: 5}
	if d := g.Dist(a, b); d != 95 {
		t.Fatalf("a valley 100 wide puts them %d apart, want 95", d)
	}
	g.Wrap = true
	if d := g.Dist(a, b); d != 5 {
		t.Fatalf("a globe 100 round puts them %d apart, want 5", d)
	}
	if d := g.Delta(a, b); d != (geom.Pos{X: -5, Y: 0}) {
		t.Fatalf("the short way from 2 to 97 is %v, want five steps west", d)
	}
	if p := g.Toward(geom.Pos{X: 0, Y: 5}, b); p != (geom.Pos{X: 99, Y: 5}) {
		t.Fatalf("a step from the west edge toward 97 lands on %v, want 99", p)
	}
	if p := g.Norm(geom.Pos{X: -1, Y: 3}); p != (geom.Pos{X: 99, Y: 3}) {
		t.Fatalf("west of the west edge is %v, want 99", p)
	}
	if g.In(geom.Pos{X: 500, Y: 3}) != true || g.In(geom.Pos{X: 3, Y: -1}) {
		t.Fatal("every column is on a globe; a row past the pole is not")
	}
}

func TestNearestFindsGroundAcrossTheSeam(t *testing.T) {
	g := NewGrid(40, 10)
	g.Wrap = true
	g.Turn(geom.Pos{X: 38, Y: 5}, Rock)
	p, ok := g.Nearest(geom.Pos{X: 1, Y: 5}, 10, func(_ geom.Pos, t *Tile) bool { return t.Terrain == Rock })
	if !ok || p != (geom.Pos{X: 38, Y: 5}) {
		t.Fatalf("the outcrop three tiles west across the seam was found at %v, %v", p, ok)
	}
	// A ring wider than the map reads each column once and still finds it.
	p, ok = g.Nearest(geom.Pos{X: 20, Y: 0}, 30, func(_ geom.Pos, t *Tile) bool { return t.Terrain == Rock })
	if !ok || p != (geom.Pos{X: 38, Y: 5}) {
		t.Fatalf("from the far side of the map the outcrop was found at %v, %v", p, ok)
	}
}

func TestAWrappedMapRoutesAcrossTheSeam(t *testing.T) {
	g := NewGrid(60, 12)
	g.Wrap = true
	from, to := geom.Pos{X: 1, Y: 6}, geom.Pos{X: 58, Y: 6}
	path := g.Path(from, to)
	if len(path) != 3 {
		t.Fatalf("the way from 1 to 58 round the seam is %d steps: %v", len(path), path)
	}
	for _, p := range path {
		if p.X < 0 || p.X >= g.W {
			t.Fatalf("a step off the map in the route: %v", path)
		}
	}
	if c := g.TravelCost(from, to); c != 3 {
		t.Fatalf("three tiles of grass round the seam cost %v", c)
	}
	g.Wrap = false
	if len(g.Path(from, to)) != 57 {
		t.Fatalf("the same way across a valley is %d steps", len(g.Path(from, to)))
	}
}

func TestAWindowHoldsTheWholeDefaultMap(t *testing.T) {
	w := NewLand(1, DefaultTerms())
	g := w.Grid
	corners := []geom.Pos{{X: 0, Y: 0}, {X: g.W - 1, Y: 0}, {X: 0, Y: g.H - 1}, {X: g.W - 1, Y: g.H - 1}}
	for _, from := range corners {
		f := g.Routes(from)
		for i := range g.Tiles {
			p := g.PosOf(i)
			if _, ok := f.slot(p.X, p.Y); !ok {
				t.Fatalf("from %v the window does not hold %v", from, p)
			}
		}
	}
}

func TestNothingRoutesPastTheWindow(t *testing.T) {
	g := NewGrid(300, 10)
	from := geom.Pos{X: 10, Y: 5}
	if c := g.TravelCost(from, geom.Pos{X: 10 + Window, Y: 5}); c != float64(Window) {
		t.Fatalf("the far edge of the window costs %v, want %d tiles of grass", c, Window)
	}
	if c := g.TravelCost(from, geom.Pos{X: 11 + Window, Y: 5}); c != math.Inf(1) {
		t.Fatalf("a tile past the window costs %v, want no way there", c)
	}
	if p := g.Path(from, geom.Pos{X: 11 + Window, Y: 5}); p != nil {
		t.Fatalf("a way past the window was found: %v", p)
	}
	// On a globe the window goes round the seam with the routes.
	g = NewGrid(128, 10)
	g.Wrap = true
	if c := g.TravelCost(geom.Pos{X: 2, Y: 5}, geom.Pos{X: 125, Y: 5}); c != 5 {
		t.Fatalf("round the seam costs %v, want 5", c)
	}
}
