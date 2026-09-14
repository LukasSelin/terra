package terra

import (
	"github.com/LukasSelin/terra/geom"
	"testing"
)

// Water leaves a tile down the steepest fall, and not toward whichever
// neighbour happens to sit lowest. On ground that falls evenly to the east,
// the three eastern neighbours are all the same height and a diagonal one is
// half again as far off, so picking the lowest picks a diagonal - the same
// diagonal every time, because the first one offered wins a tie. What that
// drew was a set of parallel lines at forty-five degrees ruled across every
// plain on the map.
func TestWaterLeavesDownTheSteepestFall(t *testing.T) {
	g := NewGrid(40, 40)
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			g.At(geom.Pos{X: x, Y: y}).Height = float64(40 - x)
		}
	}
	if got := g.Aspect(geom.Pos{X: 10, Y: 10}); got != (geom.Pos{X: 1}) {
		t.Fatalf("on ground falling due east the water leaves toward %v, want due east", got)
	}
	// A slope that really does fall hardest on the diagonal still reads as one.
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			g.At(geom.Pos{X: x, Y: y}).Height = float64(80 - x - y)
		}
	}
	if got := g.Aspect(geom.Pos{X: 10, Y: 10}); got != (geom.Pos{X: 1, Y: 1}) {
		t.Fatalf("on ground falling to the south-east the water leaves toward %v", got)
	}
}

// A river goes somewhere. Every tile of one has its next tile downstream in
// water too, the whole way to the sea or off the edge of the map - which is
// the one thing about a river that is not a matter of degree, and which the
// reading a channel is chosen by cannot promise on its own: a trunk crossing
// its own flood plain carries all the water on the map and has no fall at all.
func TestARiverRunsAllTheWayDown(t *testing.T) {
	for _, cfg := range []Terms{DefaultTerms(), smallGlobe()} {
		g := NewLand(3, cfg).Grid
		broken, rivers := 0, 0
		for i := range g.Tiles {
			if !g.Tiles[i].Wet() || g.underSea(i) || g.lakeOf[i] >= 0 {
				continue // the sea, and standing water, which a lake is
			}
			rivers++
			q, ok := g.Downstream(g.PosOf(i))
			if !ok {
				continue // off the map, which is where a valley's water goes
			}
			if !g.At(q).Wet() {
				broken++
			}
		}
		// Not none, and the bar is a share rather than a count. A settlement
		// embanks the ground it holds, so a channel that wants to run through
		// a field or a house is stopped there by carve and starts again below
		// it; and a great river spreads onto the bank beside it, and a tile of
		// bank drains along the bank as often as into the channel. Both are
		// deliberate and both are a few tiles in a hundred. A network that has
		// come apart is a third of them.
		if share := float64(broken) / float64(max(1, rivers)); share > 0.08 {
			t.Errorf("%dx%d: %d of %d river tiles send their water onto dry ground (%.0f%%)",
				g.W, g.H, broken, rivers, 100*share)
		}
	}
}

// And it heads in the high ground, because that is where the water is and
// because steep ground needs less of a catchment to cut a channel than flat
// ground does. Chosen on how much water alone, the high fifth of a map held
// almost no river at all: the flow that picks a river out is the flow at the
// bottom of a basin, and the bottom of a basin is not a mountain.
func TestRiversHeadInTheHighGround(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3} {
		g := NewLand(seed, DefaultTerms()).Grid
		hs := make([]float64, len(g.Tiles))
		for i := range g.Tiles {
			hs[i] = g.Tiles[i].Height
		}
		high := quantile(hs, 1-uplandShare)
		var wet, up int
		for i := range g.Tiles {
			if !g.Tiles[i].Wet() {
				continue
			}
			wet++
			if hs[i] >= high {
				up++
			}
		}
		if wet == 0 {
			t.Fatalf("seed %d has no rivers", seed)
		}
		// Over five seeds this runs from an eighth to a fifth of the network -
		// 17.0, 19.2, 11.6, 18.9 and 17.4 per cent - and the line is under the
		// worst of them. What is being caught is a map whose high country is
		// dry, which is what this was: on flow alone it was none, one and none.
		// See channelTheta, which is what moved it.
		if share := float64(up) / float64(wet); share < 0.08 {
			t.Errorf("seed %d: %.1f%% of the river is in the high fifth of the map",
				seed, 100*share)
		}
	}
}

// The share of a map that comes out as watercourse is the share it is meant to
// have. Laying each channel from its head down to the sea adds tiles nobody
// counted - the trunks - and counting only the heads put nine tiles in a
// hundred of a valley under water against the four and a half it asks for.
//
// The lakes are not in the share. They are as big as the hollows the ground
// has, which is a fact about the ground and not a figure asked for, and on
// the default valley they are another three to eight tiles in a hundred.
func TestAMapGetsTheWaterItAsksFor(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3} {
		g := NewLand(seed, DefaultTerms()).Grid
		wet := 0
		for i := range g.Tiles {
			if g.Tiles[i].Wet() && g.lakeOf[i] < 0 {
				wet++
			}
		}
		share := float64(wet) / float64(len(g.Tiles))
		if share < waterShare/2 || share > 2*waterShare {
			t.Errorf("seed %d came out %.1f%% water, against the %.1f%% asked for",
				seed, 100*share, 100*waterShare)
		}
	}
}

// smallGlobe is a wrapped world with a history behind it, at a size a test can
// afford: the full preset is fourteen seconds.
func smallGlobe() Terms {
	cfg := GlobeTerms()
	cfg.Width, cfg.Height = 256, 128
	return cfg
}
