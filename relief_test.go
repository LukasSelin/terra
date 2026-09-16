package terra

import (
	"github.com/LukasSelin/terra/geom"
	"math"
	"testing"
)

// Every tile must have somewhere to send its water, or the drainage that
// everything else is read from would be built on a landscape with holes in it.
// Somewhere is the edge of the map, the sea, or a lake the air empties: never
// a hollow in dry ground, and never round in a circle.
func TestAllGroundDrainsSomewhere(t *testing.T) {
	g := NewLandSized(4, 60, 40).Grid
	drainsSomewhere(t, g)
}

// drainsSomewhere follows every tile's water to wherever it stops, and fails
// if that is anywhere but the sea, the edge of the map, a closed lake or a
// salt flat.
func drainsSomewhere(t *testing.T, g *Grid) {
	t.Helper()
	for i := range g.Tiles {
		p := g.PosOf(i)
		for steps := 0; ; steps++ {
			if steps > len(g.Tiles) {
				t.Fatalf("the water from %v goes round in a circle", g.PosOf(i))
			}
			q, ok := g.Downstream(p)
			if !ok {
				break
			}
			p = q
		}
		j := g.Index(p)
		edge := g.outlet(p.X, p.Y)
		if !edge && !g.sunk(j) && !g.closedLake(j) && !g.pans[j] {
			t.Fatalf("the water from %v stops at %v, in a hollow with nowhere to go", g.PosOf(i), p)
		}
	}
}

// flowOnlyGathers fails if water that has come together carries less on the
// tile it goes to than on the tile it left. The one place it may is the
// outlet of a lake, which passes on what the lake was given less what its
// surface gave the air, so lakes are not asked.
func flowOnlyGathers(t *testing.T, g *Grid) {
	t.Helper()
	for i := range g.Tiles {
		if g.lakeOf[i] >= 0 || g.area[i] < spreadUntil {
			continue
		}
		p := g.PosOf(i)
		down, ok := g.Downstream(p)
		if !ok {
			continue
		}
		if g.Flow[g.Index(down)] < g.Flow[g.Index(p)]-1e-9 {
			t.Fatalf("water thins going downhill, %v (%.4f) to %v (%.4f)",
				p, g.Flow[g.Index(p)], down, g.Flow[g.Index(down)])
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
	flowOnlyGathers(t, g)
	// The whole of the map's rain leaves it, or goes back to the air off a
	// lake: what goes off the edge is what the edge tiles carry, and none of
	// the rest disappears on the way.
	out := 0.0
	for i := range g.Tiles {
		p := g.PosOf(i)
		switch {
		case g.lakeOf[i] >= 0:
		case g.outlet(p.X, p.Y), g.pans[i]:
			out += g.Flow[i]
		}
	}
	perMM := discharge(1, TileSpan)
	for k, l := range g.Lakes {
		given := 0.0
		for i := range g.Tiles {
			if g.lakeOf[i] == int32(k) {
				given += g.loss(i) * perMM
			}
		}
		if l.Closed {
			given = l.Inflow
		}
		out += math.Min(l.Inflow, given)
	}
	if out < g.water*(1-1e-6) {
		t.Fatalf("only %.4f of the map's %.4f m3/s reaches the edge or the air", out, g.water)
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
		if f := g.Flow[i]; f > most {
			start, most = geom.Pos{X: i % g.W, Y: i / g.W}, f
		}
	}
	p := start
	for step := 0; step < len(g.Tiles); step++ {
		q, ok := g.Downstream(p)
		if !ok {
			if p.X == 0 || p.Y == 0 || p.X == g.W-1 || p.Y == g.H-1 {
				return // it reached the edge and left
			}
			t.Fatalf("the river stops at %v, which is not the edge of the map", p)
		}
		p = q
	}
	t.Fatal("following the river downhill never ended")
}

// Slope and aspect read the lie of the land: on a ramp that falls to the east
// the slope is the fall per tile and the aspect points east.
func TestSlopeAndAspectReadTheRamp(t *testing.T) {
	g := NewGrid(10, 5)
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			g.Height[g.Index(geom.Pos{X: x, Y: y})] = float64(10 - x) // falls eastward
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
			g.Height[g.Index(geom.Pos{X: x, Y: y})] = float64(y) * 8 // falls northward
		}
	}
	shaded := g.Sunlight(geom.Pos{X: 5, Y: 2})
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			g.Height[g.Index(geom.Pos{X: x, Y: y})] = float64(g.H-y) * 8 // falls southward
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
			g.Height[g.Index(geom.Pos{X: x, Y: y})] = math.Max(0, 40-4*math.Abs(float64(x-10)))
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
		if a.Grid.Height[i] != b.Grid.Height[i] {
			t.Fatalf("tile %d came out at %v and %v", i, a.Grid.Height[i], b.Grid.Height[i])
		}
		if a.Grid.Tiles[i].Terrain != b.Grid.Tiles[i].Terrain {
			t.Fatalf("tile %d is %v one time and %v the next", i, a.Grid.Tiles[i].Terrain, b.Grid.Tiles[i].Terrain)
		}
	}
}
