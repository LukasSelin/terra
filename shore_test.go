package terra

import (
	"math"
	"math/rand"
	"testing"

	"github.com/LukasSelin/terra/geom"
)

// coast is a square map with an ocean over its western half running off the
// map, dry ground over its eastern half, a bay that narrows from forty tiles
// across at its mouth to a few at its head running into the land, and a small
// basin under sea level shut away inland.
func coast() *Grid {
	g := NewGrid(128, 128)
	for i := range g.Tiles {
		p := g.PosOf(i)
		h := 20.0 + 0.1*float64(p.X-64) // dry ground, rising gently inland
		half := 20 - 19*float64(p.X-64)/46
		switch {
		case p.X < 64:
			h = 2 // the open sea, eight metres deep
		case p.X < 110 && math.Abs(float64(p.Y-64)) <= half:
			h = 2 // the bay
		case p.X >= 96 && p.X <= 100 && p.Y >= 10 && p.Y <= 14:
			h = 2 // a hollow under sea level with no way to the sea
		}
		g.Height[i] = h
	}
	g.sea = 10
	for i := range g.Tiles {
		if g.underSea(i) {
			g.Tiles[i].Terrain = Water
		}
	}
	return g
}

// A bay that narrows gathers the tide: the tide rises all the way up it, and
// the head of the bay has more of it than the open coast the bay opens off -
// by very little on a bay a kilometre long, which is a small part of a tide's
// wavelength. And a sea the tide cannot get into has none.
func TestTheTideRisesTowardTheHeadOfAFunnelBay(t *testing.T) {
	g := coast()
	f := g.tidalReach()
	at := func(x, y int) float64 { return float64(f[g.Index(geom.Pos{X: x, Y: y})]) }
	open, shut := at(62, 20), at(98, 12)
	last := open
	for _, x := range []int{66, 76, 86, 96, 106, 109} {
		if got := at(x, 64); !(got > last) {
			t.Errorf("%d tiles up the bay the tide is %.6f of the ocean's, against %.6f below it", x-64, got, last)
		} else {
			last = got
		}
	}
	if head := at(109, 64); !(head > open) || head > 1.01*open {
		t.Errorf("the head of a bay a kilometre long has %.4f of the ocean's tide and the open coast %.4f", head, open)
	}
	if shut != 0 {
		t.Errorf("a sea with no way in has %.2f of the ocean's tide", shut)
	}
}

// A channel a good part of a quarter of the tide's wavelength long rings with
// it: at its head the tide is 1/cos kL of the tide at its mouth. It takes a
// channel hundreds of kilometres long, so it is read on tiles of four.
func TestALongChannelRingsWithTheTide(t *testing.T) {
	g := NewGrid(140, 21)
	g.deep = 4 * km
	g.sea = 300
	const mouth, head = 20, 114
	for i := range g.Tiles {
		p := g.PosOf(i)
		g.Height[i] = 400
		if p.X < mouth || (p.Y >= 8 && p.Y <= 12 && p.X <= head) {
			g.Height[i], g.Tiles[i].Terrain = 100, Water // two hundred metres deep
		}
	}
	f := g.tidalReach()
	k := tideOmega / math.Sqrt(gravity*(200+MeanHigh))
	kl := k * float64(head-mouth+1) * g.deep
	want := 1 / math.Cos(kl)
	got := float64(f[g.Index(geom.Pos{X: head, Y: 10})])
	if math.Abs(got-want) > 0.1*want {
		t.Errorf("at the head of a channel kL = %.2f long the tide is %.2f of the ocean's; a channel shut at its head has %.2f", kl, got, want)
	}
	mid := float64(f[g.Index(geom.Pos{X: (mouth + head) / 2, Y: 10})])
	if !(mid > 1 && mid < got) {
		t.Errorf("half way up the channel the tide is %.2f, and at its head %.2f", mid, got)
	}
}

// A channel of the same width all the way along does not gather the tide: what
// comes in at its mouth is what reaches its head, less a little to the bed and
// more by as little as the reflection off its head gives a channel a small part
// of a tide's wavelength long.
func TestAStraightInletDoesNotGatherTheTide(t *testing.T) {
	g := NewGrid(128, 64)
	for i := range g.Tiles {
		p := g.PosOf(i)
		g.Height[i] = 20
		if p.X < 40 || (p.Y >= 30 && p.Y <= 34) {
			g.Height[i], g.Tiles[i].Terrain = 2, Water
		}
	}
	g.sea = 10
	f := g.tidalReach()
	for x := 41; x < 120; x += 13 {
		if got := float64(f[g.Index(geom.Pos{X: x, Y: 32})]); math.Abs(got-1) > 0.02 {
			t.Errorf("%d tiles up a straight inlet the tide is %.3f of the ocean's", x-40, got)
		}
	}
}

// A tide runs up a river until its bed stands above the highest water it can
// bring, and no further: the tidal limit.
func TestATideStopsAtTheTidalLimit(t *testing.T) {
	g := NewGrid(160, 20)
	for i := range g.Tiles {
		p := g.PosOf(i)
		h := 2.0
		if p.X >= 40 {
			h = 30 // the land
			if p.Y == 10 {
				h = 9.6 + 0.02*float64(p.X-40) // the river bed, rising a little each tile
				g.Tiles[i].Terrain = Water
			}
		}
		g.Height[i] = h
	}
	g.sea = 10
	for i := range g.Tiles {
		if g.underSea(i) {
			g.Tiles[i].Terrain = Water
		}
	}
	f := g.tidalReach()
	last := -1
	for x := 40; x < g.W; x++ {
		i := g.Index(geom.Pos{X: x, Y: 10})
		if f[i] > 0 {
			if last != x-1 && last >= 0 {
				t.Fatalf("the tide skips up the river from %d to %d", last, x)
			}
			last = x
			if h := g.Height[i]; h > g.sea+float64(f[i])*TideMax {
				t.Fatalf("at %d the tide reaches a bed %.2f m high, above its highest water %.2f", x, h, g.sea+float64(f[i])*TideMax)
			}
		}
	}
	if last < 41 || last >= g.W-1 {
		t.Errorf("the tide reached %d tiles up a river that climbs out of its reach", last-39)
	}
}

// estuary is an ocean over the west of the map, running off it, a shore of mud
// rising one in five hundred out of it from a metre and a quarter under mean
// sea to more than a metre over it, and dry ground rising steeply behind, with
// the tide laid on it.
func estuary() *Grid {
	g := NewGrid(128, 64)
	g.sea = 10
	for i := range g.Tiles {
		p := g.PosOf(i)
		t := &g.Tiles[i]
		g.Soil[i], g.Sand[i], t.Clay = 1, 0.1, 0.4
		switch {
		case p.X < 40:
			g.Height[i] = 2
		case p.X < 88:
			g.Height[i] = 8.75 + 0.05*float64(p.X-40)
		default:
			g.Height[i] = 11.2 + 0.5*float64(p.X-88)
		}
		if g.underSea(i) {
			t.Terrain = Water
		}
	}
	g.tides()
	g.Recount()
	return g
}

// Flats are where the tide comes and goes: within an ordinary spring's reach of
// mean sea, no steeper than the sea grades their ground, with nothing growing
// on them and no ice. A gentle shore has them from the one side of mean sea to
// the other; a globe's has them only where its coast is gentle, which on a
// globe cut into steep country is not everywhere, and they are no great part of
// it. A valley has no sea and no tide.
//
// A small globe with flats on it: the third. The fourth had sixteen while its
// whole ocean floor lay within twenty metres of the sea and every tile of it
// was surf the littoral drift carried sand across; with the deep floor laid at
// its age's depth - see abyss.go - that sand stays on the shelves, and the
// fourth has none.
func TestTheTideLaysFlatsOnlyWhereItReaches(t *testing.T) {
	for _, c := range []struct {
		name string
		g    *Grid
	}{{"the estuary", estuary()}, {"small globe 3", yardWorld("small", 3, smallGlobe())}} {
		g := c.g
		flats, above := 0, 0
		shore, _ := g.fromShore(&surf{})
		for i := range g.Tiles {
			tile := &g.Tiles[i]
			if tile.Terrain != Flat {
				continue
			}
			flats++
			if g.Height[i] > g.sea {
				above++
			}
			f := float64(g.tidal[i])
			if f <= 0 {
				t.Fatalf("%s: a flat at %v has a tide of %.2f", c.name, g.PosOf(i), f)
			}
			if math.Abs(g.Height[i]-g.sea) > f*flatTide+1e-6 {
				t.Fatalf("%s: a flat at %v stands %.2f m from the sea, beyond the %.2f m its tide reaches", c.name, g.PosOf(i), g.Height[i]-g.sea, f*flatTide)
			}
			if s, most := g.Slope(g.PosOf(i)), deanSlope(g.medianGrain(i), shore[i]); s > most {
				t.Fatalf("%s: a flat at %v lies at %.4f, steeper than the %.4f the sea grades it to", c.name, g.PosOf(i), s, most)
			}
			if g.Fish[i] != 0 || g.Wood[i] != 0 || g.Wild[i] != 0 || g.Fertility[i] != 0 {
				t.Fatalf("%s: a flat at %v has something growing on it", c.name, g.PosOf(i))
			}
		}
		if flats == 0 {
			t.Fatalf("%s has no flats", c.name)
		}
		if got := g.Count(func(t *Tile) bool { return t.Terrain == Flat }); got != flats {
			t.Fatalf("%s: counted %d flats and %d", c.name, got, flats)
		}
		if share := float64(flats) / float64(len(g.Tiles)); share > 0.05 && c.name != "the estuary" {
			t.Errorf("%s: flats are %.1f%% of it", c.name, 100*share)
		}
		if c.name == "the estuary" && (above == 0 || above == flats) {
			t.Errorf("the estuary's %d flats have %d above mean sea", flats, above)
		}
	}

	v := NewLand(1, DefaultTerms()).Grid
	if v.tidal != nil || v.Count(func(t *Tile) bool { return t.Terrain == Flat }) > 0 {
		t.Errorf("a valley with no sea has a tide")
	}
}

// A flat under mean sea is covered on a day the tide hardly falls and open on
// a day it falls far; a flat above mean sea is never covered at low water; and
// a causeway over either is dry every day.
func TestAFlatIsOpenAtSpringsAndShutAtNeaps(t *testing.T) {
	g := estuary()
	var deep, high = -1, -1
	for i := range g.Tiles {
		if g.Tiles[i].Terrain != Flat {
			continue
		}
		switch e := g.ebb[i]; {
		case deep < 0 && e > 0.4 && e < 0.7:
			deep = i
		case high < 0 && e < 0:
			high = i
		}
	}
	if deep < 0 || high < 0 {
		t.Fatalf("no flats to try: %d under mean sea, %d over", deep, high)
	}
	neap, spring := Tide{High: 0.3, Low: -0.3}, Tide{High: 0.8, Low: -0.8}
	pd, ph := g.PosOf(deep), g.PosOf(high)
	g.SetTide(neap)
	if !g.Covered(pd) || !g.Shut(pd) {
		t.Errorf("at neaps the flat %.2f tides under mean sea is open", g.ebb[deep])
	}
	if g.Covered(ph) {
		t.Errorf("a flat above mean sea is covered at low water")
	}
	if lo, hi := g.LowWater(pd), g.HighWater(pd); !(lo < g.sea && hi > g.sea) {
		t.Errorf("low water %.2f and high %.2f do not straddle the sea at %.2f", lo, hi, g.sea)
	}
	g.SetTide(spring)
	if g.Covered(pd) || g.Shut(pd) {
		t.Errorf("at springs the flat %.2f tides under mean sea is covered", g.ebb[deep])
	}
	g.SetTide(neap)
	g.Build(pd, paving)
	if g.Covered(pd) {
		t.Errorf("a causeway is under water")
	}
}

// strait is a map with a band of flats right across it between two shores, all
// lying the same way under mean sea, so that there is no way from one shore to
// the other but over the mud.
func strait() *Grid {
	g := NewGrid(40, 12)
	g.ebb = make([]float32, len(g.Tiles))
	g.tidal = make([]float32, len(g.Tiles))
	for i := range g.Tiles {
		g.Height[i] = 10
		if x := i % g.W; x >= 15 && x < 25 {
			g.Tiles[i].Terrain = Flat
			g.ebb[i], g.tidal[i] = 0.5, 1
		}
	}
	g.Recount()
	return g
}

// Across the flats a laden walker goes at springs and does not at neaps; one
// with nothing to carry wades them at neaps at the water's price and walks
// them at springs at the mud's. Anybody may wade out onto a covered flat and
// stop there, and a causeway is dry whatever the moon is doing. None of it
// moves the map's water: the regions are the same every day.
func TestTheFlatsAreAFortnightlyCrossing(t *testing.T) {
	g := strait()
	neap, spring := Tide{Tick: 1, High: 0.3, Low: -0.3}, Tide{Tick: 2, High: 0.8, Low: -0.8}
	from, to := geom.Pos{X: 5, Y: 6}, geom.Pos{X: 34, Y: 6}
	waters := g.Waters()

	g.SetTide(spring)
	if c := g.Carrying(1).TravelCost(from, to); math.IsInf(c, 1) {
		t.Errorf("at springs a laden walker cannot cross the flats")
	}
	open := g.TravelCost(from, to)
	if got := g.MoveCost(geom.Pos{X: 20, Y: 6}); got != moveCost[Flat] {
		t.Errorf("an open flat costs %v, want %v", got, moveCost[Flat])
	}

	g.SetTide(neap)
	if c := g.Carrying(1).TravelCost(from, to); !math.IsInf(c, 1) {
		t.Errorf("at neaps a laden walker crossed covered flats for %v", c)
	}
	if got := g.MoveCost(geom.Pos{X: 20, Y: 6}); got != moveCost[Water] {
		t.Errorf("a covered flat costs %v, want %v", got, moveCost[Water])
	}
	if shut := g.TravelCost(from, to); !(shut > open) {
		t.Errorf("wading the covered flats cost %v and walking them open %v", shut, open)
	}
	if c := g.Carrying(1).TravelCost(from, geom.Pos{X: 15, Y: 6}); math.IsInf(c, 1) {
		t.Errorf("a laden walker may not wade out onto a covered flat to stop there")
	}

	for x := 15; x < 25; x++ {
		g.Build(geom.Pos{X: x, Y: 6}, paving)
	}
	if c := g.Carrying(1).TravelCost(from, to); math.IsInf(c, 1) {
		t.Errorf("at neaps a laden walker cannot cross by the causeway")
	}
	if g.Waters() != waters {
		t.Errorf("the tide moved the map's water: %d, then %d", waters, g.Waters())
	}
}

// A survey is of one day's sea. Spread at springs, it is not read for a cost
// at neaps.
func TestASurveyIsOfOneTide(t *testing.T) {
	g := strait()
	from, to := geom.Pos{X: 5, Y: 6}, geom.Pos{X: 34, Y: 6}
	r := g.Router()
	g.SetTide(Tide{Tick: 2, High: 0.8, Low: -0.8})
	r.Survey(from, 1, 200)
	if c := r.Carrying(1).TravelCost(from, to); math.IsInf(c, 1) {
		t.Fatalf("the springs survey found no way over")
	}
	g.SetTide(Tide{Tick: 3, High: 0.3, Low: -0.3})
	if c := r.Carrying(1).TravelCost(from, to); !math.IsInf(c, 1) {
		t.Errorf("a neap day's cost was read off the springs survey: %v", c)
	}
}

// The landmark tables are taken over every flat as open ground, which is the
// most open the map ever is, so whatever the day, a search that reads them
// costs every walk the same and sets out the same way as one that does not.
func TestLandmarkBoundsHoldAtEveryTide(t *testing.T) {
	g := estuary()
	var flats []geom.Pos
	for i := range g.Tiles {
		if g.Tiles[i].Terrain == Flat {
			flats = append(flats, g.PosOf(i))
		}
	}
	if len(flats) == 0 {
		t.Fatal("no flats to route over")
	}
	rng := rand.New(rand.NewSource(1))
	type pair struct {
		from, to geom.Pos
		load     float64
	}
	var pairs []pair
	for len(pairs) < 300 {
		a := flats[rng.Intn(len(flats))]
		b := g.Norm(geom.Pos{X: a.X + rng.Intn(41) - 20, Y: a.Y + rng.Intn(41) - 20})
		c := g.Norm(geom.Pos{X: a.X + rng.Intn(41) - 20, Y: a.Y + rng.Intn(41) - 20})
		if !g.In(b) || !g.In(c) || b == c {
			continue
		}
		pairs = append(pairs, pair{from: b, to: c, load: float64(rng.Intn(2))})
	}
	g.RefreshLandmarks(LandmarkRest)
	r := g.Router()
	for _, day := range []Tide{{Tick: 1, High: 0.3, Low: -0.3}, {Tick: 2, High: 0.55, Low: -0.55}, {Tick: 3, High: 0.85, Low: -0.85}} {
		g.SetTide(day)
		for _, p := range pairs {
			g.landmarks.built = false
			r.Reset()
			want := r.Carrying(p.load).TravelCost(p.from, p.to)
			wantStep := r.Carrying(p.load).StepToward(p.from, p.to)
			g.landmarks.built = true
			r.Reset()
			got := r.Carrying(p.load).TravelCost(p.from, p.to)
			gotStep := r.Carrying(p.load).StepToward(p.from, p.to)
			if math.IsInf(want, 1) != math.IsInf(got, 1) || math.Abs(want-got) > tie {
				t.Fatalf("high water %.2f: %v -> %v carrying %v costs %v with landmarks, %v without", day.High, p.from, p.to, p.load, got, want)
			}
			if gotStep != wantStep {
				t.Fatalf("high water %.2f: %v -> %v carrying %v sets out to %v with landmarks, %v without", day.High, p.from, p.to, p.load, gotStep, wantStep)
			}
		}
	}
}

// Mud comes out of the water over a flat at the turn of the tide: in salt water
// clay clots into flocs that fall a hundred times as fast as its grains, so a
// quiet flat takes nearly all the silt and clay the tide brings over it. Over a
// flat so gentle that the flood has to run fast across it to fill the ground
// behind, the flow keeps more of the clay up; and ground over high water takes
// nothing.
func TestMudSettlesOnAFlatAtSlackWater(t *testing.T) {
	if r := tidalFall[Clay] / fallSpeed[Clay]; r < 50 {
		t.Errorf("a floc of clay falls %.0f times as fast as its grain", r)
	}
	ramp := func(slope float64) (*Grid, int) {
		g := NewGrid(40, 5)
		g.sea = 10
		for i := range g.Tiles {
			g.Height[i] = 10 + slope*TileSpan*float64(i%g.W-20)
		}
		return g, g.Index(geom.Pos{X: 20, Y: 2})
	}
	quiet, i := ramp(1.0 / 500)
	q := quiet.flatShare(i, 1)
	if q[Silt] < 0.9 || q[Clay] < 0.9 {
		t.Errorf("a quiet flat at mean sea takes %.2f of the silt and %.2f of the clay", q[Silt], q[Clay])
	}
	fast, j := ramp(1.0 / 20000)
	if s := fast.flatShare(j, 1); !(s[Clay] < q[Clay]) {
		t.Errorf("a flat the flood runs fast over takes %.3f of the clay, and a quiet one %.3f", s[Clay], q[Clay])
	}
	dry, k := ramp(1.0 / 500)
	dry.Height[k] = 10 + MeanHigh + 0.01
	if s := dry.flatShare(k, 1); s != ([Grains]float64{}) {
		t.Errorf("ground over high water takes %v", s)
	}
}
