package terra

import (
	"github.com/LukasSelin/terra/geom"
	"testing"
)

// heights returns a copy of the ground, for comparing before with after.
func heights(g *Grid) []float64 {
	out := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		out[i] = g.Height[i]
	}
	return out
}

// Weather takes the hills down and puts them in the valleys. Nothing is moved
// by hand: the high ground loses what the water carries off it and the low
// ground gains what the water stops carrying.
func TestWeatherMovesSoilDownhill(t *testing.T) {
	w := NewLandSized(3, 60, 40)
	g := w.Grid
	before := heights(g)
	high := quantile(before, 0.85)
	low := quantile(before, 0.15)

	for age := 0; age < 40; age++ {
		w.Erode()
	}

	var lostUp, gainedDown float64
	for i := range g.Tiles {
		d := g.Height[i] - before[i]
		switch {
		case before[i] >= high:
			lostUp += d
		case before[i] <= low:
			gainedDown += d
		}
	}
	if lostUp >= 0 {
		t.Fatalf("the high ground gained %.1f m; weather should take it down", lostUp)
	}
	if gainedDown <= 0 {
		t.Fatalf("the low ground lost %.1f m; the hills have to end up somewhere", gainedDown)
	}
}

// The one thing about the weather that is the settlement's own doing: woods
// hold a hillside together and a ploughed field does not, so a people that
// clears its slopes to farm them washes those slopes into its own river.
//
// The slopes are the hillsides and not the banks, at the start or at the end.
// In a valley cut into its ground the tiles beside a river are as steep as any,
// and the river takes them whatever grows on them - see meander. Counted in, the
// woods lost 1923 metres against 3047 ploughed, less than twice; with the
// rivers' own banks set aside, the rain's share of it is what is left.
func TestWoodsHoldAHillsideTogether(t *testing.T) {
	lost := func(cover Terrain) float64 {
		w := NewLandSized(3, 60, 40)
		g := w.Grid
		var slopes []int
		for i := range g.Tiles {
			if tl := &g.Tiles[i]; tl.Terrain != Water && tl.Drain > FloodDepth/2 && !g.HasNeighbor(g.PosOf(i), (*Tile).Wet) {
				tl.Terrain = cover
				slopes = append(slopes, i)
			}
		}
		g.Rekind()
		before := heights(g)
		for age := 0; age < 40; age++ {
			w.Erode()
		}
		var total float64
		for _, i := range slopes {
			if g.Tiles[i].Wet() || g.HasNeighbor(g.PosOf(i), (*Tile).Wet) {
				continue // the river came to it, and took it as a bank
			}
			if d := before[i] - g.Height[i]; d > 0 {
				total += d
			}
		}
		return total
	}
	wooded, ploughed := lost(Forest), lost(Field)
	if ploughed <= wooded*2 {
		t.Fatalf("ploughed slopes lost %.0f m and wooded ones %.0f m; clearing a hillside should cost it dearly",
			ploughed, wooded)
	}
}

// After the ground has moved, the drainage has to still make sense: every
// tile with somewhere to send its water, and the water gathering as it goes.
// Everything the map decides is read off these, so a hollow left behind by
// the weather would be a hole in the world.
//
// Gathering is asked only of water that has come together. A sheet on a
// hillside goes down every way that falls, so the tile below one takes only
// its share of it and may carry less; see spreadUntil.
func TestTheDrainageSurvivesWeathering(t *testing.T) {
	w := NewLandSized(8, 50, 40)
	g := w.Grid
	for age := 0; age < 25; age++ {
		w.Erode()
	}
	drainsSomewhere(t, g)
	flowOnlyGathers(t, g)
}

// A river may take a course it did not have, but it does not take the market
// with it. A settlement embanks what it stands on.
func TestWeatherDoesNotDrownWhatIsBuilt(t *testing.T) {
	w := NewLandSized(9, 50, 40)
	g := w.Grid
	// Put a house on the wettest ground there is, short of the river itself.
	var bank geom.Pos
	var most float64
	for i := range g.Tiles {
		p := geom.Pos{X: i % g.W, Y: i / g.W}
		if g.Tiles[i].Terrain == Water {
			continue
		}
		if f := g.Flow[i]; f > most {
			bank, most = p, f
		}
	}
	g.Build(bank, blocking)
	field := geom.Pos{X: bank.X, Y: bank.Y}
	if q := (geom.Pos{X: bank.X + 1, Y: bank.Y}); g.In(q) && g.At(q).Terrain != Water {
		field = q
		g.Turn(q, Field)
		g.Claim(q, 7)
	}

	for age := 0; age < 30; age++ {
		w.Erode()
	}
	if g.At(bank).Terrain == Water || g.At(bank).Mark != blocking {
		t.Fatalf("the house at %v was washed away: %+v", bank, *g.At(bank))
	}
	if field != bank && g.At(field).Terrain == Water {
		t.Fatalf("the claimed field at %v was flooded", field)
	}
}

// Weathering is part of a run, so it has to come out the same every time.
func TestTheSameSeedWeathersTheSameWay(t *testing.T) {
	run := func() []float64 {
		w := NewLandSized(12, 40, 30)
		for age := 0; age < 15; age++ {
			w.Erode()
		}
		return heights(w.Grid)
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("tile %d weathered to %v one time and %v the next", i, a[i], b[i])
		}
	}
}

// Soil goes with the ground it was in. A slope stripped by the weather holds
// less than it did, which is the whole cost of clearing a hillside to farm
// it: not that the ground is gone, but that what would grow on it is.
//
// The valley floor is not asserted to grow richer, and it does not. It starts
// near as rich as ground gets, so there is nowhere for silt to take it; and as
// the channel cuts down, the flat beside it stands further above the water
// each age and becomes a terrace rather than a water meadow. Drying out is
// what happens to a flood plain the river has left below it.
func TestSoilGoesWithTheGround(t *testing.T) {
	w := NewLandSized(3, 60, 40)
	g := w.Grid
	var slopes []int
	for i := range g.Tiles {
		if tl := &g.Tiles[i]; tl.Terrain != Water && tl.Drain > FloodDepth {
			tl.Terrain = Field // bared for the plough
			slopes = append(slopes, i)
		}
	}
	g.Rekind()
	was := make(map[int]float64, len(slopes))
	for _, i := range slopes {
		was[i] = g.Rich[i]
	}
	for age := 0; age < 40; age++ {
		w.Erode()
	}
	// Only the ground that is still a slope at the end, and slope is asked of
	// the fall and not of the height above a river. A river wanders across its
	// own valley now - see meander.go - so some of what started above the
	// flood plain is under one by the fortieth age, and ground the river has
	// reached is ground the river has fed; that is the flood plain doing its
	// work rather than this claim failing.
	//
	// Standing above the flood by FloodDepth was how that was asked at first,
	// and it is not the same question. Drain is the height above the nearest
	// water, so what counts as "off the flood plain" moves whenever the amount
	// of water on the map moves - and when the banks of the great rivers were
	// tightened, ground that the river still feeds stopped clearing the line
	// and stayed in the reckoning. The whole set then came out richer while
	// every part of it that was really a hillside came out poorer: over a
	// tenth of a fall, 0.150 to 0.054; over a fifth, 0.150 to 0.027; and at
	// twice FloodDepth, 0.150 to 0.141. The fall is what a slope is.
	var before, after, kept float64
	for _, i := range slopes {
		if g.Tiles[i].Terrain == Water || g.Slope(g.PosOf(i)) <= 0.10 {
			continue
		}
		before += was[i]
		after += g.Rich[i]
		kept++
	}
	if len(slopes) == 0 || kept == 0 {
		t.Fatal("the map has no ground that is still a hillside")
	}
	b, a := before/kept, after/kept
	if a >= b {
		t.Fatalf("ploughed slopes hold %.3f after forty ages of weather and held %.3f before; want them the poorer for it", a, b)
	}
}

// The water charges the rock under a map for what it takes: the same valley,
// all shale and bared of its soil, sends the sea several times what it sends
// all granite in an age. With its soil on, an age takes the soil and not the
// rock, and the rock under it does not come into it.
func TestTheWaterWearsShaleBeforeGranite(t *testing.T) {
	sent := func(rock Bedrock) float64 {
		w := NewLandSized(3, 60, 40)
		g := w.Grid
		for i := range g.Tiles {
			g.Tiles[i].Bedrock = rock
			g.Tiles[i].Soil = 0
		}
		g.wear(ageYears)
		return g.exported[Sand] + g.exported[Silt] + g.exported[Clay]
	}
	soft, hard := sent(Shale), sent(Granite)
	if !(soft > 3*hard) {
		t.Errorf("shale sent %.2f m to the sea and granite %.2f", soft, hard)
	}
}
