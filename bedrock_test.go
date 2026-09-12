package terra

import (
	"github.com/LukasSelin/terra/geom"
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

// What the water sorts, it sorts by weight: sand goes down at the first
// slackening and the fine stuff travels on. So the ground the water reaches
// and leaves something on - the flat of the valley, within the flood - ends
// up finer than the slopes it came off, and that difference is not put there
// by anything that knows where a valley is.
func TestTheWaterSortsWhatItCarries(t *testing.T) {
	w := NewLand(5, DefaultTerms())
	for age := 0; age < 12; age++ {
		w.Erode()
	}
	g := w.Grid
	var floor, hillside, floorN, hillN float64
	for i := range g.Tiles {
		tile := &g.Tiles[i]
		if tile.Wet() {
			continue
		}
		p := geom.Pos{X: i % g.W, Y: i / g.W}
		switch {
		case tile.Drain < FloodDepth/2:
			floor += tile.Sand
			floorN++
		case g.Slope(p) > 0.1:
			hillside += tile.Sand
			hillN++
		}
	}
	if floorN == 0 || hillN == 0 {
		t.Fatalf("nothing to compare: %v of valley floor, %v of hillside", floorN, hillN)
	}
	floor, hillside = floor/floorN, hillside/hillN
	if !(floor < hillside) {
		t.Errorf("the valley floor is sand %.3f and the hillsides %.3f; the water should have left the coarse stuff up the hill",
			floor, hillside)
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
