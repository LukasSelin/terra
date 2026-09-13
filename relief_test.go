package terra

import (
	"github.com/LukasSelin/terra/geom"
	"math"
	"testing"
)

// Every tile must have somewhere to send its water, or the drainage that
// everything else is read from would be built on a landscape with holes in it.
func TestAllGroundDrainsSomewhere(t *testing.T) {
	w := NewLandSized(4, 60, 40)
	g := w.Grid
	for y := 1; y < g.H-1; y++ {
		for x := 1; x < g.W-1; x++ {
			p := geom.Pos{X: x, Y: y}
			if g.Aspect(p) == (geom.Pos{}) {
				t.Fatalf("the ground at %v has nowhere lower to go; a hollow was left unfilled", p)
			}
		}
	}
}

// Water gathers as it goes: a tile carries everything its uphill neighbours
// sent it, so flow only ever grows downstream. That is what makes the rivers
// come out where they do rather than where they were put.
//
// Once it has come together, that is. Before then it is a sheet on a hillside
// and goes down every way that falls, so the tile below takes only a share of
// it; see spreadUntil. What is asked of the sheet is that none of it is lost.
func TestFlowOnlyGathers(t *testing.T) {
	w := NewLandSized(5, 50, 40)
	g := w.Grid
	gathered := spreadUntil / float64(g.landTiles())
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			p := geom.Pos{X: x, Y: y}
			a := g.Aspect(p)
			if a == (geom.Pos{}) {
				continue
			}
			down := geom.Pos{X: p.X + a.X, Y: p.Y + a.Y}
			if g.At(p).Flow >= gathered && g.At(down).Flow < g.At(p).Flow-1e-9 {
				t.Fatalf("water thins going downhill, %v (%.4f) to %v (%.4f)",
					p, g.At(p).Flow, down, g.At(down).Flow)
			}
		}
	}
	// The whole of the map's rain leaves it: what goes off the edge is what
	// the edge tiles carry, and none of it disappears on the way.
	out := 0.0
	for i := range g.Tiles {
		p := g.PosOf(i)
		if g.outlet(p.X, p.Y) {
			out += g.Tiles[i].Flow
		}
	}
	if out < 1-1e-6 {
		t.Fatalf("only %.4f of the map's rain reaches the edge", out)
	}
}

// The rivers are not drawn, they are what is left when the water has run: the
// wettest ground on the map, and it runs off the edge rather than stopping in
// the middle of nowhere.
func TestRiversRunOffTheMap(t *testing.T) {
	w := NewLandSized(6, 60, 40)
	g := w.Grid
	water := g.Count(func(t *Tile) bool { return t.Terrain == Water })
	if water == 0 {
		t.Fatal("the map has no water at all")
	}
	// Follow the wettest tile downhill; it must leave the map.
	var start geom.Pos
	var most float64
	for i := range g.Tiles {
		if f := g.Tiles[i].Flow; f > most {
			start, most = geom.Pos{X: i % g.W, Y: i / g.W}, f
		}
	}
	p := start
	for step := 0; step < len(g.Tiles); step++ {
		a := g.Aspect(p)
		if a == (geom.Pos{}) {
			if p.X == 0 || p.Y == 0 || p.X == g.W-1 || p.Y == g.H-1 {
				return // it reached the edge and left
			}
			t.Fatalf("the river stops at %v, which is not the edge of the map", p)
		}
		p = geom.Pos{X: p.X + a.X, Y: p.Y + a.Y}
	}
	t.Fatal("following the river downhill never ended")
}

// Slope and aspect read the lie of the land: on a ramp that falls to the east
// the slope is the fall per tile and the aspect points east.
func TestSlopeAndAspectReadTheRamp(t *testing.T) {
	g := NewGrid(10, 5)
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			g.At(geom.Pos{X: x, Y: y}).Height = float64(10 - x) // falls eastward
		}
	}
	p := geom.Pos{X: 4, Y: 2}
	if got, want := g.Slope(p), 1/TileSpan; math.Abs(got-want) > 1e-9 {
		t.Fatalf("slope = %v, want %v: a metre of fall over a tile", got, want)
	}
	if a := g.Aspect(p); a.X != 1 {
		t.Fatalf("aspect = %v, want it pointing east, the way the ground falls", a)
	}
	// A slope facing south catches the sun; one facing north stands in shade.
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			g.At(geom.Pos{X: x, Y: y}).Height = float64(y) * 8 // falls northward
		}
	}
	shaded := g.Sunlight(geom.Pos{X: 5, Y: 2})
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			g.At(geom.Pos{X: x, Y: y}).Height = float64(g.H-y) * 8 // falls southward
		}
	}
	if sunny := g.Sunlight(geom.Pos{X: 5, Y: 2}); sunny <= shaded {
		t.Fatalf("a south-facing slope catches %.2f and a north-facing one %.2f; want the south sunnier", sunny, shaded)
	}
}

// Height above the water is what makes a valley floor a water meadow. It is
// nothing on the river and grows as the ground rises away from it.
func TestGroundStandsAboveItsRiver(t *testing.T) {
	w := NewLandSized(7, 60, 40)
	g := w.Grid
	var floor, hill int
	for i := range g.Tiles {
		switch t := &g.Tiles[i]; {
		case t.Terrain == Water:
		case t.Drain < 2:
			floor++
		case t.Drain > 20:
			hill++
		}
	}
	if floor == 0 || hill == 0 {
		t.Fatalf("the map is all one level: %d tiles near the water, %d well above it", floor, hill)
	}
	// The valley floor is the good soil.
	var lowFert, highFert float64
	var lowN, highN int
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if t.Terrain == Water {
			continue
		}
		if t.Drain < 2 {
			lowFert += g.Fertility[i]
			lowN++
		} else if t.Drain > 20 {
			highFert += g.Fertility[i]
			highN++
		}
	}
	if lowFert/float64(lowN) <= highFert/float64(highN) {
		t.Fatalf("the valley floor grows %.2f and the hillside %.2f; want the floor the better soil",
			lowFert/float64(lowN), highFert/float64(highN))
	}
}

// Walking uphill costs more than walking on the level, and going down costs
// less than going up. It is what sends a route round the shoulder of a hill.
func TestClimbingCostsMoreThanContouring(t *testing.T) {
	g := NewGrid(20, 20)
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			// A ridge across the middle, north to south.
			g.At(geom.Pos{X: x, Y: y}).Height = math.Max(0, 40-4*math.Abs(float64(x-10)))
		}
	}
	level := g.StepCost(geom.Pos{X: 1, Y: 5}, geom.Pos{X: 1, Y: 6})
	up := g.StepCost(geom.Pos{X: 5, Y: 5}, geom.Pos{X: 6, Y: 5})
	down := g.StepCost(geom.Pos{X: 6, Y: 5}, geom.Pos{X: 5, Y: 5})
	if !(up > down && down >= level) {
		t.Fatalf("uphill %v, downhill %v, level %v; want uphill dearest and level cheapest", up, down, level)
	}

	// Crossing the ridge is dearer than the same distance along its foot.
	over := g.TravelCost(geom.Pos{X: 4, Y: 10}, geom.Pos{X: 16, Y: 10})
	along := g.TravelCost(geom.Pos{X: 1, Y: 4}, geom.Pos{X: 1, Y: 16})
	if over <= along {
		t.Fatalf("over the ridge costs %v and the same distance on the flat %v; want the climb dearer", over, along)
	}
}

// The same seed must raise the same ground, or nothing downstream of it
// repeats either.
func TestTheSameSeedRaisesTheSameGround(t *testing.T) {
	a, b := NewLandSized(11, 40, 30), NewLandSized(11, 40, 30)
	for i := range a.Grid.Tiles {
		if a.Grid.Tiles[i].Height != b.Grid.Tiles[i].Height {
			t.Fatalf("tile %d came out at %v and %v", i, a.Grid.Tiles[i].Height, b.Grid.Tiles[i].Height)
		}
		if a.Grid.Tiles[i].Terrain != b.Grid.Tiles[i].Terrain {
			t.Fatalf("tile %d is %v one time and %v the next", i, a.Grid.Tiles[i].Terrain, b.Grid.Tiles[i].Terrain)
		}
	}
}
