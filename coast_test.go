package terra

import (
	"math"
	"testing"

	"github.com/LukasSelin/terra/geom"
)

// headland is a coast of cliffs ten metres over a shallow sea open to the
// ocean off the west edge, the northern half of shale and the southern of
// granite, with no soil, a tide of the open ocean's, and a wind onshore.
func headland() (*Grid, func(i, k int) (float64, float64)) {
	g := NewGrid(60, 40)
	g.sea = 10
	g.tidal = make([]float32, len(g.Tiles))
	for i := range g.Tiles {
		p := g.PosOf(i)
		t := &g.Tiles[i]
		g.tidal[i] = 1
		t.Height, t.Bedrock = 20, Shale
		if p.Y >= 20 {
			t.Bedrock = Granite
		}
		if p.X < 20 {
			t.Height, t.Terrain = 8, Water
		}
	}
	return g, func(i, k int) (float64, float64) { return 8, 0 }
}

// heightsIn is the sum of the heights of tiles x0 to x1 across and y0 to y1 down.
func heightsIn(g *Grid, x0, x1, y0, y1 int) float64 {
	sum := 0.0
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			sum += g.At(geom.Pos{X: x, Y: y}).Height
		}
	}
	return sum
}

// A cliff of soft rock goes back faster than one of hard under the same sea,
// and leaves a wider platform at the level of the sea; and all it was is on the
// shore or gone to the sea, to the grain.
func TestCliffsRetreatFasterInSoftRock(t *testing.T) {
	g, wind := headland()
	s := g.surfOf(wind)
	before := heightsIn(g, 0, 59, 0, 39)
	supply := make([]float64, len(s.cells))
	g.cliffs(s, 200*yr, supply)
	after := heightsIn(g, 0, 59, 0, 39)

	shale := heightsIn(g, 20, 59, 5, 14)
	granite := heightsIn(g, 20, 59, 25, 34)
	full := 20.0 * 40 * 10
	if !(full-shale > 2*(full-granite)) || !(full-granite > 0) {
		t.Errorf("the shale cliff lost %.1f m of ground and the granite %.1f", full-shale, full-granite)
	}
	platform := func(y int) int {
		n := 0
		for x := 20; x < 60 && g.underSea(g.Index(geom.Pos{X: x, Y: y})); x++ {
			n++
		}
		return n
	}
	if !(platform(10) > platform(30)) {
		t.Errorf("the shale platform is %d tiles wide and the granite %d", platform(10), platform(30))
	}
	sand := 0.0
	for _, v := range supply {
		sand += v
	}
	gone := g.exported[Silt] + g.exported[Clay]
	if lost := before - after; lost <= 0 || math.Abs(lost-sand-gone) > 1e-9*lost {
		t.Errorf("the cliffs lost %.4f m of ground: %.4f of sand on the shore and %.4f of fines to sea", lost, sand, gone)
	}
}

// spit is a straight coast running north to south down the west of the map to
// a point, past which the shore turns away east, over a sea a metre deep and
// open to the ocean off the west edge.
func spit() *Grid {
	g := NewGrid(60, 60)
	g.sea = 10
	for i := range g.Tiles {
		p := g.PosOf(i)
		t := &g.Tiles[i]
		t.Height = 9
		t.Terrain = Water
		if p.X >= 30 && p.Y < 30 {
			t.Height, t.Terrain = 14, Grass
		}
	}
	return g
}

// The sand the waves drive along a shore goes the way they drive it, and past
// the point where the shore turns away it goes on into the water ahead: a
// spit, pointing down the drift. Turn the wind about and it points the other
// way. Every grain given to the shore is laid on it or gone to the sea.
func TestSpitsPointDownDrift(t *testing.T) {
	for _, c := range []struct {
		name  string
		north float64
		south bool
	}{{"from the north-west", -6, true}, {"from the south-west", 6, false}} {
		g := spit()
		s := g.surfOf(func(i, k int) (float64, float64) { return 6, c.north })
		supply := make([]float64, len(s.cells))
		at := s.slot[g.Index(geom.Pos{X: 29, Y: 15})]
		if at < 0 {
			t.Fatal("no surf on the coast")
		}
		const given = 40.0
		supply[at] = given
		before := heightsIn(g, 0, 59, 0, 59)
		g.littoral(s, 10*yr, supply)
		laid := heightsIn(g, 0, 59, 0, 59) - before
		if math.Abs(laid+g.exported[Sand]-given) > 1e-9*given {
			t.Errorf("%s: %.4f given, %.4f laid and %.4f gone to sea", c.name, given, laid, g.exported[Sand])
		}
		var down, up, land int
		for i := range g.Tiles {
			p := g.PosOf(i)
			if g.Tiles[i].Height <= 9 || p.X >= 30 && p.Y < 30 {
				continue
			}
			if p.Y > 15 {
				down++
			} else if p.Y < 15 {
				up++
			}
			if !g.underSea(i) && p.Y >= 30 {
				land++
			}
		}
		if !c.south {
			down, up = up, down
		}
		if !(down > 0 && up == 0) {
			t.Errorf("%s: sand laid on %d tiles down the drift and %d up it", c.name, down, up)
		}
		if c.south && land == 0 {
			t.Errorf("%s: no spit stands above the sea past the point", c.name)
		}
	}
}

// The waves' whole work on a coast - the cliffs, the drift and the winnowing
// together - makes and loses no ground: what the rivers brought to the surf and
// the ground the coast was, less what went to the sea, is what the coast is.
func TestTheCoastConservesTheGround(t *testing.T) {
	g, _ := headland()
	for i := range g.Tiles {
		t := &g.Tiles[i]
		t.Soil, t.Sand, t.Clay = 2, 0.5, 0.2
		if p := g.PosOf(i); p.X < 20 && p.X > 14 {
			t.Height = 9.5 // a shallow shelf the sand moves over
		}
	}
	s := g.surfOf(func(i, k int) (float64, float64) { return 7, -4 })
	sands := make([]float64, len(g.Tiles))
	brought := 0.0
	for y := 2; y < 38; y += 6 {
		sands[g.Index(geom.Pos{X: 19, Y: y})] = 3
		brought += 3
	}
	before := heightsIn(g, 0, 59, 0, 39)
	g.coast(s, 50*yr, sands)
	after := heightsIn(g, 0, 59, 0, 39)
	gone := g.exported[Sand] + g.exported[Silt] + g.exported[Clay]
	if math.Abs(before+brought-after-gone) > 1e-9*(before+brought) {
		t.Errorf("%.6f of ground and %.6f brought, %.6f left and %.6f gone to sea", before, brought, after, gone)
	}
	if before-after+brought <= 0 || gone <= 0 {
		t.Errorf("the coast did nothing: %.4f lost, %.4f gone", before-after, gone)
	}
}
