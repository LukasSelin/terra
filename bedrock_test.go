package terra

import (
	"math"
	"testing"
)

// Every rock has to say what it weathers to, and what it says has to be a
// mixture: three shares of one soil, none of them negative and none of them
// more than the whole.
func TestEveryRockWeathersToAMixture(t *testing.T) {
	for _, b := range Bedrocks() {
		w := weathers[b]
		if w.sand < 0 || w.clay < 0 || w.sand+w.clay > 1 {
			t.Errorf("%s weathers to sand %v clay %v, which is not a mixture", b, w.sand, w.clay)
		}
		if b.String() == "rock" {
			t.Errorf("bedrock %d has no name", b)
		}
	}
}

// Every rock says what it gives a soil, and says it as shares of itself: the
// quartz and the carbonate are minerals and cannot come to more than the rock
// between them, and a rock with no bases and no phosphorus is no rock a soil
// has ever come out of. And the table has to rank the rocks the way the
// analyses do - limestone the carbonate one, sandstone the quartz one, basalt
// the richest of the silicate rocks in both bases and phosphorus - or it is a
// table of numbers and not of rocks.
func TestEveryRockHasAChemistry(t *testing.T) {
	for _, b := range Bedrocks() {
		c := b.Chemistry()
		if c.Quartz < 0 || c.Carbonate < 0 || c.Quartz+c.Carbonate > 1 || c.Bases <= 0 || c.Bases > 1 ||
			c.Phosphorus <= 0 || c.Phosphorus > 0.01 {
			t.Errorf("%s has chemistry %+v, which is not a rock's", b, c)
		}
	}
	most := func(of func(Chemistry) float64, among []Bedrock) Bedrock {
		best := among[0]
		for _, b := range among {
			if of(b.Chemistry()) > of(best.Chemistry()) {
				best = b
			}
		}
		return best
	}
	silicate := []Bedrock{Granite, Sandstone, Shale, Basalt, Schist}
	for _, c := range []struct {
		what  string
		of    func(Chemistry) float64
		among []Bedrock
		want  Bedrock
	}{
		{"carbonate", func(c Chemistry) float64 { return c.Carbonate }, Bedrocks(), Limestone},
		{"quartz", func(c Chemistry) float64 { return c.Quartz }, Bedrocks(), Sandstone},
		{"bases among the silicate rocks", func(c Chemistry) float64 { return c.Bases }, silicate, Basalt},
		{"phosphorus", func(c Chemistry) float64 { return c.Phosphorus }, Bedrocks(), Basalt},
	} {
		if got := most(c.of, c.among); got != c.want {
			t.Errorf("the most %s is %s's and not %s's", c.what, got, c.want)
		}
	}
}

// The lines are cut at each lattice's own middle, so a map gets some of all
// four of the rocks a lattice lays however its noise happened to fall. A map
// with one rock on it is a map with one soil on it, which is what all this is
// for undoing.
//
// Basalt and schist are not among them, and that is the point of them: they
// are rocks that have to happen to a place. Nothing that comes up and nothing
// that is buried and squeezed can be laid down by a lattice that knows only
// where a tile is, so they belong to a world made by its history.
func TestAMapCarriesAllTheRocksALatticeLays(t *testing.T) {
	laid := []Bedrock{Granite, Limestone, Sandstone, Shale}
	for _, seed := range []uint64{1, 2, 3, 7} {
		w := NewLand(seed, DefaultTerms())
		var seen [BedrockCount]int
		for i := range w.Grid.Tiles {
			seen[w.Grid.Tiles[i].Bedrock]++
		}
		for _, b := range laid {
			if seen[b] == 0 {
				t.Errorf("seed %d has no %s on it", seed, b)
			}
		}
		for _, b := range []Bedrock{Basalt, Schist} {
			if seen[b] != 0 {
				t.Errorf("seed %d has %s on it, which nothing without a history can make", seed, b)
			}
		}
	}
}

// A soil is a mixture before an age of weather and after it. Nothing the
// water does may take a share below nothing or put the two of them above the
// whole, however much it moves.
func TestSoilStaysAMixtureThroughTheWeather(t *testing.T) {
	w := NewLand(5, DefaultTerms())
	for age := 0; age < 8; age++ {
		w.Erode()
	}
	for i := range w.Grid.Tiles {
		t2 := &w.Grid.Tiles[i]
		if t2.Sand < 0 || t2.Clay < 0 || t2.Sand+t2.Clay > 1+1e-9 || t2.Silt() < 0 {
			t.Fatalf("tile %d is sand %v silt %v clay %v", i, t2.Sand, t2.Silt(), t2.Clay)
		}
	}
}

// What the water sorts, it sorts by weight. A river in flood keeps its sand
// on its bed and spills its silt and clay over the bank, where the flood runs
// shallow and slow over its plain: so what a flood lays on the ground beside a
// river is finer than what it lays in it, and that difference is not put there
// by anything that knows where a valley is - only by which grains the water
// holds up.
//
// It was asked of a whole valley after twelve ages: that its floor had come out
// finer than its hillsides. At real rates twelve ages is a hundred and twenty
// years, a few millimetres of silt on a metre of soil, and the floor of seed 5
// went from sand 0.291 to 0.292 against the hillsides' 0.277 to 0.281 - the
// order it started in. Over twelve thousand years both coarsened, the floor to
// 0.306 and the hillsides to 0.289, because clay does not settle out of running
// water at all and goes to the sea from everywhere; the flood plains carry the
// most water and lose the most of it. The rate that sorted the valley in a
// century was the settling at a tenth of every grain a tile, whatever the tile
// and the water were.
func TestTheWaterSortsWhatItCarries(t *testing.T) {
	g := NewLand(5, DefaultTerms()).Grid
	river := -1
	for i := range g.Tiles {
		if g.Tiles[i].Wet() && !g.standing(i) && !g.underSea(i) && g.floodWidth(i) > 0 {
			river = i
			break
		}
	}
	if river < 0 {
		t.Fatal("no river with a flood plain beside it")
	}
	n := len(g.Tiles)
	change := make([]float64, n)
	gained := make([][Grains]float64, n)
	g.overbank(river, [Grains]float64{Sand: 1, Silt: 1, Clay: 1}, change, gained)
	var bed, bank [Grains]float64
	total := 0.0
	for i := range gained {
		for gr := range gained[i] {
			total += gained[i][gr]
			if i == river {
				bed[gr] += gained[i][gr]
			} else {
				bank[gr] += gained[i][gr]
			}
		}
	}
	if math.Abs(total-3) > 1e-12 {
		t.Errorf("three metres laid and %.12f booked", total)
	}
	if bed[Sand] != 1 || bank[Sand] != 0 {
		t.Errorf("the bed kept %.3f of the sand and the bank got %.3f; the sand stays on the bed", bed[Sand], bank[Sand])
	}
	if !(bank[Silt] > bed[Silt] && bank[Clay] > bed[Clay]) {
		t.Errorf("the bank got silt %.3f and clay %.3f, the bed %.3f and %.3f; a flood spills its fines", bank[Silt], bank[Clay], bed[Silt], bed[Clay])
	}
	// And the flood plain lets fall what the channel alone would not: silt
	// read over the plain settles faster than over the bed, and sand is read
	// over the bed alone.
	t0 := &g.Tiles[river]
	q := g.Flow[river] * floodFlow
	w := flowWidth(t0, q, g.Slope(g.PosOf(river)), TileSpan)
	if !(settleShare(fallSpeed[Silt], TileSpan, w+g.floodWidth(river), q) > settleShare(fallSpeed[Silt], TileSpan, w, q)) {
		t.Error("silt settles no faster over a flood plain than in its channel")
	}
}

// The best ground is a mixture and not an extreme, so the reading has to
// peak in the middle and fall off both ways. A soil of nothing but sand and
// one of nothing but clay are both poor, and neither is poorer for being
// further past the best than the other is short of it.
func TestLoamIsBestInTheMiddle(t *testing.T) {
	best := &Tile{Sand: bestSand, Clay: bestClay}
	if math.Abs(best.Loam()-1) > 1e-9 {
		t.Errorf("the best mixture reads %v, want 1", best.Loam())
	}
	sand := &Tile{Sand: 1}
	clay := &Tile{Clay: 1}
	for _, poor := range []*Tile{sand, clay} {
		if !(poor.Loam() < best.Loam()) {
			t.Errorf("sand %v clay %v reads %v, as good as a loam", poor.Sand, poor.Clay, poor.Loam())
		}
	}
}

// Sand moves and clay stays, and the ground in between moves at the rate the
// weathering was measured at before any of this existed.
func TestSandWashesAndClayHolds(t *testing.T) {
	sandy := &Tile{Sand: 1}
	heavy := &Tile{Clay: 1}
	even := &Tile{Sand: 0.35, Clay: 0.35}
	if !(sandy.Wash() > even.Wash() && even.Wash() > heavy.Wash()) {
		t.Errorf("sand %v, even %v, clay %v: they should fall in that order",
			sandy.Wash(), even.Wash(), heavy.Wash())
	}
	if math.Abs(even.Wash()-1) > 1e-9 {
		t.Errorf("ground with as much sand as clay washes at %v, want 1", even.Wash())
	}
}
