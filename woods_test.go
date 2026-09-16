package terra

import (
	"github.com/LukasSelin/terra/geom"
	"testing"
)

// The tree line is a share of the land, read off the map rather than fixed:
// the ground that suits trees best will hold a wood and the rest will not,
// whatever shape the map happens to have.
func TestTheTreeLineCoversAShareOfTheLand(t *testing.T) {
	for seed := uint64(1); seed <= 4; seed++ {
		w := NewLand(seed, DefaultTerms())
		g := w.Grid
		land, holds := 0, 0
		for i := range g.Tiles {
			if g.Tiles[i].Terrain == Water {
				continue
			}
			land++
			if g.HoldsWood(geom.Pos{X: i % g.W, Y: i / g.W}) {
				holds++
			}
		}
		if share := float64(holds) / float64(land); share < woodsShare*0.8 || share > woodsShare*1.3 {
			t.Fatalf("seed %d: %.2f of the land holds wood, want about %.2f", seed, share, woodsShare)
		}
	}
}

// Woods want damp ground. Across a slope running from the water's edge up to
// a dry shoulder, the wood stands on the damp end of it and stops: the line
// is where the ground stops suiting trees, not where the map ends.
func TestWoodsStopWhereTheGroundDries(t *testing.T) {
	g := NewGrid(20, 20)
	for i := range g.Tiles {
		// Dry in proportion to the distance from the water along the left.
		g.Drain[i] = 2 * FloodDepth * float64(i%20) / 19
	}
	g.readWoods()
	if !g.HoldsWood(geom.Pos{X: 0, Y: 10}) {
		t.Fatal("the ground at the water's edge will not hold a wood")
	}
	if g.HoldsWood(geom.Pos{X: 19, Y: 10}) {
		t.Fatal("the dry shoulder holds a wood")
	}
	// And the line sits where the share says, a fifth of the way up.
	edge := 0
	for x := 0; x < 20; x++ {
		if g.HoldsWood(geom.Pos{X: x, Y: 10}) {
			edge = x
		}
	}
	if edge < 2 || edge > 6 {
		t.Fatalf("the wood reaches x=%d up the slope, want about a fifth of 20", edge)
	}
}

// Steep ground holds no wood however damp it is. A bank the water has cut
// into is wet to the roots and still on its way downhill, and before the
// slope was a limit rather than a discount the woods climbed straight up it.
func TestSteepGroundHoldsNoWood(t *testing.T) {
	g := NewGrid(20, 20)
	for i := range g.Tiles {
		// A flat plain with one gully cut across it, damp everywhere: only
		// the slope tells the tiles apart.
		if x := i % 20; x >= 17 {
			g.Height[i] = float64(x-16) * 20
		}
	}
	g.readWoods()
	flat, bank := geom.Pos{X: 5, Y: 10}, geom.Pos{X: 17, Y: 10}
	if g.TooSteep(flat) || !g.HoldsWood(flat) {
		t.Fatal("the flat damp ground will not hold a wood")
	}
	if !g.TooSteep(bank) {
		t.Fatalf("the bank falls %.2f a tile and is not too steep", g.Slope(bank))
	}
	if g.HoldsWood(bank) {
		t.Fatal("a wood takes on the bank")
	}
	if s := g.WoodsAt(bank); s != 0 {
		t.Fatalf("ground too steep for a wood scores %v, want nothing", s)
	}
}

// The founding woods keep off the steep ground too: the map is made with the
// same reading that governs what grows on it later.
func TestNoFoundingWoodStandsOnSteepGround(t *testing.T) {
	for seed := uint64(1); seed <= 4; seed++ {
		g := NewLand(seed, DefaultTerms()).Grid
		for i := range g.Tiles {
			p := geom.Pos{X: i % g.W, Y: i / g.W}
			if g.Tiles[i].Terrain == Forest && g.TooSteep(p) {
				t.Fatalf("seed %d: a founding wood stands on a slope of %.3f, over the line at %.3f",
					seed, g.Slope(p), g.steepLine)
			}
		}
	}
}
