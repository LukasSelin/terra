package terra

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// The level the sea is poured to holds exactly the water poured, on any
// ground: the room under it, summed over the map, is the water.
func TestTheSeaHoldsTheWaterPoured(t *testing.T) {
	g := NewGrid(60, 40)
	r := rand.New(rand.NewPCG(1, 2))
	for i := range g.Tiles {
		g.Tiles[i].Height = 100 * r.Float64() * r.Float64()
	}
	for _, water := range []float64{0.5, 5, 20, 200} {
		g.sea = g.level(water)
		if got := g.room(); math.Abs(got-water) > 1e-9*water {
			t.Errorf("poured %v m and the sea at %.3f holds %v", water, g.sea, got)
		}
	}
}

// The same water over a world whose continents stand higher covers less of
// it: the share is the ground's, not the water's.
func TestHighContinentsDrownLess(t *testing.T) {
	covered := func(continent float64) float64 {
		g := NewGrid(100, 50)
		for i := range g.Tiles {
			x := i % g.W
			h := 10 * float64(x%50) / 50 // two ocean basins, deepening westward
			if x >= 25 && x < 50 || x >= 75 {
				h = continent + float64(x%25)/5 // two continents
			}
			g.Tiles[i].Height = h
		}
		g.sea = g.level(6)
		under := 0
		for i := range g.Tiles {
			if g.underSea(i) {
				under++
			}
		}
		return float64(under) / float64(len(g.Tiles))
	}
	low, high := covered(8), covered(40)
	if !(high < low) {
		t.Errorf("low continents are %.2f under the sea and high ones %.2f", low, high)
	}
}

// A made world keeps the water it was given however its valleys were cut,
// and how much of it is sea differs from one world to the next, as the plates
// did. The deep floor holds its own water besides - see abyss.go - and that
// is kilometres of it and not metres.
func TestTheSeaIsTheWorldsToSay(t *testing.T) {
	if testing.Short() {
		t.Skip("six small globes; see docs/perf/suite.md")
	}
	var shares []float64
	for seed := uint64(1); seed <= 6; seed++ {
		g := yardWorld("small", seed, smallGlobe())
		deep := g.abyssWater()
		if got := g.room() - deep; math.Abs(got-DefaultWater) > 1e-6 {
			t.Errorf("seed %d holds %v m of water over its deep floor, given %v", seed, got, DefaultWater)
		}
		if deep < 1000 || deep > 5000 {
			t.Errorf("seed %d's deep floor holds %.0f m of water; the earth's oceans spread over the whole earth come to some 2,600", seed, deep)
		}
		under := 0
		for i := range g.Tiles {
			if g.underSea(i) {
				under++
			}
		}
		shares = append(shares, float64(under)/float64(len(g.Tiles)))
	}
	slices.Sort(shares)
	if spread := shares[len(shares)-1] - shares[0]; spread < 0.02 {
		t.Errorf("six worlds are all about the same sea: %v", shares)
	}
	// Seeds 1 to 6 come out .617 .549 .724 .769 .652 .471. They were .611
	// .541 .723 .750 .649 .452 while every epoch of a history filled its
	// hollows in for good; with the hollows left for the water to stand in,
	// the ground the sea is poured over holds a little less, and every world
	// is a point or two wetter.
	if shares[0] < 0.35 || shares[len(shares)-1] > 0.80 {
		t.Errorf("the sea covers between %.2f and %.2f of six worlds", shares[0], shares[len(shares)-1])
	}
}

// Pouring the sea to a level floods the same tiles, with the same fish, as
// flooding it to the share that puts it at that level: the chance drawn does
// not care how the level was found.
func TestPouringFloodsAsFloodingDoes(t *testing.T) {
	make := func() *Grid {
		g := NewGrid(50, 30)
		r := rand.New(rand.NewPCG(3, 4))
		for i := range g.Tiles {
			g.Tiles[i].Height = 50 * r.Float64()
		}
		return g
	}
	a, b := make(), make()
	ra, rb := rand.New(rand.NewPCG(5, 6)), rand.New(rand.NewPCG(5, 6))
	a.pour(4, ra)
	under := 0
	for i := range a.Tiles {
		if a.underSea(i) {
			under++
		}
	}
	b.seaAt(a.sea, rb)
	for i := range a.Tiles {
		if a.Tiles[i].Terrain != b.Tiles[i].Terrain || a.Fish[i] != b.Fish[i] {
			t.Fatalf("tile %d: poured %v fish %v, flooded %v fish %v", i, a.Tiles[i].Terrain, a.Fish[i], b.Tiles[i].Terrain, b.Fish[i])
		}
	}
	if ra.Float64() != rb.Float64() || under == 0 {
		t.Errorf("pouring drew differently from flooding, or drowned nothing (%d)", under)
	}
}
