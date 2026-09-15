package terra

import (
	"math"
	"math/rand"
	"testing"

	"github.com/LukasSelin/terra/geom"
)

// The sea goes down slowly through a glacial cycle and comes back up fast at
// the end of it: at today's level through the Holocene, a hundred and twenty
// metres down at the last glacial maximum twenty thousand years ago, and the
// same again a hundred thousand years before that. What that fall takes is a
// thirtieth of the ocean's water.
func TestTheSeaGoesDownSlowlyAndComesUpFast(t *testing.T) {
	for _, c := range []struct{ before, want float64 }{
		{0, 0}, {5 * kyr, 0}, {20 * kyr, 120}, {120 * kyr, 120}, {13.5 * kyr, 60}, {60 * kyr, 60},
	} {
		if got := glacialFall(c.before); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%.1f kyr ago the sea stood %.1f m down, want %.1f", c.before/kyr, got, c.want)
		}
	}
	// The fall takes eighty thousand years and the rise thirteen.
	if fall, rise := glacialFall(30*kyr)-glacialFall(40*kyr), glacialFall(15*kyr)-glacialFall(10*kyr); !(rise/5 > 4*fall/10) {
		t.Errorf("the sea rose %.1f m in five thousand years and fell %.1f in ten", rise, fall)
	}
	if share := glacialLow / oceanDepth; share < 0.03 || share > 0.035 {
		t.Errorf("the ice takes %.3f of the ocean's water", share)
	}
}

// bruunShore is a shore of soil over a shoreface half a metre deep, out to
// deeper water and a far shore seven hundred metres off, under an onshore wind,
// with the soil's grain given by its sand, and the sea just risen by a metre
// over the shoreface.
func bruunShore(sand float64) (*Grid, *surf) {
	g := NewGrid(60, 30)
	g.sea = 10
	for i := range g.Tiles {
		p := g.PosOf(i)
		t := &g.Tiles[i]
		t.Soil, t.Sand, t.Clay = 5, sand, 0.05
		switch {
		case p.X < 2 || p.Y == 0 || p.Y == 29:
			t.Height = 13
		case p.X < 10:
			t.Height, t.Terrain = 0, Water
		case p.X < 30:
			t.Height, t.Terrain = 9.5, Water
		default:
			t.Height = 13
		}
	}
	return g, g.surfOf(func(i, k int) (float64, float64) { return 6, 0 })
}

// A rising sea moves a shore up and back, the Bruun rule: the ground its upper
// shoreface loses is laid on its lower, and none is made or lost. A shore of
// finer ground has a gentler profile, wider for the same depth of closure, and
// goes back further for the same rise.
func TestARisingSeaMovesTheShoreUpAndBack(t *testing.T) {
	moved := func(sand float64) (lost, gained float64) {
		g, s := bruunShore(sand)
		before := heightsIn(g, 0, 59, 0, 29)
		g.shoreUp(s, 1)
		if after := heightsIn(g, 0, 59, 0, 29); math.Abs(after-before) > 1e-9*before {
			t.Errorf("sand %.2f: the ground went from %.6f to %.6f", sand, before, after)
		}
		return 13*20 - heightsIn(g, 30, 30, 5, 24), heightsIn(g, 20, 29, 5, 24) - 10*20*9.5
	}
	coarseLost, coarseGained := moved(0.9)
	fineLost, _ := moved(0.3)
	if !(coarseLost > 0) || !(coarseGained > 0) || coarseLost >= 20*3 {
		t.Errorf("a rise took %.4f m off the shore and laid %.4f on the shoreface", coarseLost, coarseGained)
	}
	if !(fineLost > coarseLost) {
		t.Errorf("a rise took %.4f m off a fine shore and %.4f off a coarse one", fineLost, coarseLost)
	}
}

// shelfValley is a valley running west down a slope of dry ground to the sea,
// over a shelf falling to sixty metres deep at the west edge of the map.
func shelfValley() *Land {
	w := NewLandSized(1, 96, 64)
	g := w.Grid
	const sea = 100.0
	for i := range g.Tiles {
		p := g.PosOf(i)
		t := &g.Tiles[i]
		t.Height = sea + 2 + float64(p.X-24) - math.Max(0, 6-1.5*math.Abs(float64(p.Y-32)))
		if p.X < 24 {
			t.Height = sea - 5 - 55*float64(24-p.X)/24
		}
		t.Terrain = Grass
	}
	g.sea, g.base = sea, sea
	for i := range g.Tiles {
		if g.underSea(i) {
			g.Tiles[i].Terrain = Water
		}
	}
	g.drain()
	return w
}

// The rivers cut down toward the low sea of a glacial and the sea comes back
// over what they cut: the valley's floor at the coast ends up drowned further
// inland than the same valley cut for as long at today's sea. The shelf's sea
// is shallow, so the ice is taken to hold half of it at its greatest, which
// puts its low sea tens of metres down, as a real ocean's is.
func TestTheLowSeaLeavesDrownedValleys(t *testing.T) {
	drowned := func(fallen func(float64) float64) (int, float64) {
		w := shelfValley()
		g := w.Grid
		low := math.Inf(1)
		g.cutThrough(rand.New(rand.NewSource(1)), func(before float64) float64 {
			f := fallen(before)
			if f > 0 {
				low = math.Min(low, g.sea)
			}
			return f
		})
		n := 0
		for x := 24; x < g.W; x++ {
			if g.underSea(g.Index(geom.Pos{X: x, Y: 32})) {
				n++
			}
		}
		return n, low
	}
	still, _ := drowned(func(float64) float64 { return 0 })
	glacial, low := drowned(func(before float64) float64 { return glacialFall(before) / glacialLow * oceanDepth / 2 })
	if !(glacial > still) {
		t.Errorf("after a glacial cycle the valley is drowned %d tiles inland, and cut at today's sea %d", glacial, still)
	}
	if !(low < 100-10) {
		t.Errorf("the low sea stood at %.1f against today's 100", low)
	}
}

// A drawn map asked to be cut through the last glacial cycle is: it is not the
// map cut through the last two thousand years, and it keeps its sea.
func TestAGlacialMapIsCutThroughTheCycle(t *testing.T) {
	terms := Terms{Width: 64, Height: 64, SeaShare: 0.3}
	plain := NewLand(3, terms).Grid
	terms.Glacial = true
	glacial := NewLand(3, terms).Grid
	if glacial.sea < 0 || glacial.Count(func(t *Tile) bool { return t.Terrain == Water }) == 0 {
		t.Fatalf("a glacial map lost its sea: level %.2f", glacial.sea)
	}
	same := true
	for i := range plain.Tiles {
		if plain.Tiles[i].Height != glacial.Tiles[i].Height {
			same = false
			break
		}
	}
	if same {
		t.Errorf("cut through a glacial cycle, the map came out the same as cut through two thousand years")
	}
}
