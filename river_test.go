package terra

import (
	"github.com/LukasSelin/terra/geom"
	"math"
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
			g.Height[g.Index(geom.Pos{X: x, Y: y})] = float64(40 - x)
		}
	}
	if got := g.Aspect(geom.Pos{X: 10, Y: 10}); got != (geom.Pos{X: 1}) {
		t.Fatalf("on ground falling due east the water leaves toward %v, want due east", got)
	}
	// A slope that really does fall hardest on the diagonal still reads as one.
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			g.Height[g.Index(geom.Pos{X: x, Y: y})] = float64(80 - x - y)
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
	for _, c := range []struct {
		name string
		cfg  Terms
	}{{"valley", DefaultTerms()}, {"small", smallGlobe()}} {
		g := yardWorld(c.name, 3, c.cfg) // read only, so shared
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

// And it heads in the high ground, because steep ground needs less of a
// catchment to cut a channel than flat ground does. Chosen on how much water
// alone, the high fifth of a map held almost no river at all: the flow that
// picks a river out is the flow at the bottom of a basin, and the bottom of a
// basin is not a mountain.
//
// Asked of five valleys taken together, and not of each. Since the rivers are
// where the water has the power to cut and not a share of the map, the high
// country of a valley can be dry for its own reasons: the ridges of seed 3 are
// steep and narrow, and at no power a valley can live with does any of them
// gather enough ground to hold a bed. Over the five, the high fifth holds 6.9,
// 14.3, 0, 4.9 and 9.4 per cent of the river.
//
// That was on the drawn ground, whose high country was steep noise standing on
// a gentle lowland. Shaped - see shape.go - a valley's ground falls toward its
// rivers the whole way from the ridge, and a stream gathers the water to cut a
// bed only well down its valley, below the high fifth: over the five, 4 river
// tiles in 1048 stand in it. What is asked is that the high ground is not dry of
// rivers altogether.
func TestRiversHeadInTheHighGround(t *testing.T) {
	var wet, up int
	for _, seed := range []uint64{1, 2, 3, 4, 5} {
		g := NewLand(seed, DefaultTerms()).Grid
		hs := make([]float64, len(g.Tiles))
		for i := range g.Tiles {
			hs[i] = g.Height[i]
		}
		high := quantile(hs, 1-uplandShare)
		for i := range g.Tiles {
			if !g.Tiles[i].Wet() {
				continue
			}
			wet++
			if hs[i] >= high {
				up++
			}
		}
	}
	if wet == 0 {
		t.Fatal("five valleys have no rivers")
	}
	if share := float64(up) / float64(wet); up == 0 {
		t.Errorf("%.1f%% of the river of five valleys is in their high fifth", 100*share)
	}
}

// A valley in the rain of the real world has a river running through it and
// is not a marsh. How much of it is water is for the rain to say, but it is the
// rain of a temperate valley, and that is a few tiles in a hundred.
//
// The lakes are not in it. How big they are is how big the hollows are, which
// is a fact about the ground and not about the rain once the rain fills them;
// on seed 2 of the valley they are another eight tiles in a hundred.
//
// Eleven in a hundred and not ten. Seed 1 stood at 9.9 before the rock was
// laid in beds, and a valley whose soft beds are hollowed out between hard
// ones floods a little more of its floor: it came out at 10.4 with them.
func TestAValleyHasTheWaterItsRainGives(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3} {
		g := NewLand(seed, DefaultTerms()).Grid
		if share := riverShare(g); share < 0.02 || share > 0.11 {
			t.Errorf("seed %d came out %.1f%% water", seed, 100*share)
		}
	}
}

// A drier world has fewer rivers and a wetter one more, from nothing but the
// rain: nobody tells either how much river to have.
func TestADrierWorldHasFewerRivers(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3} {
		dry, wet := DefaultTerms(), DefaultTerms()
		dry.Wetness, wet.Wetness = 0.5, 2
		d, w := riverShare(NewLand(seed, dry).Grid), riverShare(NewLand(seed, wet).Grid)
		if !(w > 1.5*d) {
			t.Errorf("seed %d: half the rain gives %.1f%% water and twice the rain %.1f%%", seed, 100*d, 100*w)
		}
		if w > 0.25 {
			t.Errorf("seed %d: twice the rain drowns %.1f%% of the valley", seed, 100*w)
		}
	}
}

// riverShare is how much of the land is running water: wet, and neither the
// sea nor a lake.
func riverShare(g *Grid) float64 {
	land, wet := 0, 0
	for i := range g.Tiles {
		if g.underSea(i) {
			continue
		}
		land++
		if g.Tiles[i].Wet() && g.lakeOf[i] < 0 {
			wet++
		}
	}
	return float64(wet) / max(1, float64(land))
}

// smallGlobe is a wrapped world with a history behind it, at a size a test can
// afford: the full preset is fourteen seconds.
func smallGlobe() Terms {
	cfg := GlobeTerms()
	cfg.Width, cfg.Height = 256, 128
	return cfg
}

// Rivers worn by their own water fall as rivers do. Flint's law: along a
// channel the fall eases as a power of the ground it drains, S = ks·A^-θ, and
// over the world's rivers the concavity θ sits between about 0.35 and 0.7
// (Flint 1974; Whipple 2004, 0.4 to 0.6 in most). Read on the river tiles of
// three valleys after forty ages of weather, binned by the ground they drain.
func TestWornRiversKeepFlintsLaw(t *testing.T) {
	sum, count := map[int]float64{}, map[int]float64{}
	for _, seed := range []uint64{1, 2, 3} {
		w := NewLand(seed, DefaultTerms())
		for k := 0; k < 40; k++ {
			w.Erode()
		}
		g := w.Grid
		for i := range g.Tiles {
			if !g.Tiles[i].Wet() || g.underSea(i) || g.lakeOf[i] >= 0 || g.down[i] < 0 {
				continue
			}
			d := int(g.down[i])
			run := TileSpan
			if i%g.W != d%g.W && i/g.W != d/g.W {
				run *= math.Sqrt2
			}
			s := (g.Height[i] - g.Height[d]) / run
			if s <= 0 || g.area[i] <= 0 {
				continue
			}
			a := math.Log(g.area[i] * TileSpan * TileSpan)
			b := int(a / 0.5)
			sum[b] += math.Log(s)
			count[b]++
		}
	}
	var xs, ys []float64
	for b, n := range count {
		if n >= 10 {
			xs = append(xs, (float64(b)+0.5)*0.5)
			ys = append(ys, sum[b]/n)
		}
	}
	if len(xs) < 4 {
		t.Fatalf("only %d bins of river to read", len(xs))
	}
	if theta := -fit(xs, ys); !(theta >= 0.35 && theta <= 0.7) {
		t.Errorf("worn rivers fall as area^-%.3f; real rivers are 0.35 to 0.7", theta)
	}
}
