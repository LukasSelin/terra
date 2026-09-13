package terra

import (
	"math"
	"testing"

	"github.com/LukasSelin/terra/geom"
)

// coast is a square map with an ocean over its western half, dry ground over
// its eastern half, a long inlet of sea running into the land, and a small
// basin under sea level shut away inland.
func coast() *Grid {
	g := NewGrid(128, 128)
	for i := range g.Tiles {
		p := g.PosOf(i)
		h := 20.0 + 0.1*float64(p.X-64) // dry ground, rising gently inland
		switch {
		case p.X < 64:
			h = 2 // the open sea, eight metres deep
		case p.Y >= 62 && p.Y <= 66 && p.X < 110:
			h = 2 // the inlet
		case p.X >= 96 && p.X <= 100 && p.Y >= 10 && p.Y <= 14:
			h = 2 // a hollow under sea level with no way to the sea
		}
		g.Tiles[i].Height = h
	}
	g.sea = 10
	for i := range g.Tiles {
		if g.underSea(i) {
			g.Tiles[i].Terrain = Water
		}
	}
	return g
}

// A bay that narrows gathers the tide: the head of the inlet has more of it
// than the open coast the inlet opens off. And a sea the tide cannot get into
// has hardly any.
func TestTheTideGathersInABay(t *testing.T) {
	g := coast()
	f := g.tidalReach()
	at := func(x, y int) float64 { return float64(f[g.Index(geom.Pos{X: x, Y: y})]) }
	head, open, shut := at(108, 64), at(62, 20), at(98, 12)
	if !(head > 1.5*open) {
		t.Errorf("the head of the inlet has %.2f of the ocean's tide and the open coast %.2f", head, open)
	}
	if !(shut < 0.3*open) {
		t.Errorf("a sea with no way in has %.2f of the ocean's tide against the open coast's %.2f", shut, open)
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
		g.Tiles[i].Height = h
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
			if h := g.Tiles[i].Height; h > g.sea+float64(f[i])*TideMax {
				t.Fatalf("at %d the tide reaches a bed %.2f m high, above its highest water %.2f", x, h, g.sea+float64(f[i])*TideMax)
			}
		}
	}
	if last < 41 || last >= g.W-1 {
		t.Errorf("the tide reached %d tiles up a river that climbs out of its reach", last-39)
	}
}

// A globe has flats, and they are where the tide comes and goes: within an
// ordinary spring's reach of mean sea, where there is a tide worth the name,
// with nothing growing on them and no ice. A valley has no sea and no tide.
func TestTheTideLaysFlatsOnlyWhereItReaches(t *testing.T) {
	g := NewLand(1, smallGlobe()).Grid
	flats := 0
	for i := range g.Tiles {
		tile := &g.Tiles[i]
		if tile.Terrain != Flat {
			continue
		}
		flats++
		f := float64(g.tidal[i])
		if f <= 0 || 2*f*(TideM2+TideS2) < FlatMinRange-1e-6 {
			t.Fatalf("a flat at %v has a tide of %.2f", g.PosOf(i), f)
		}
		if math.Abs(tile.Height-g.sea) > f*flatTide+1e-6 {
			t.Fatalf("a flat at %v stands %.2f m from the sea, beyond the %.2f m its tide reaches", g.PosOf(i), tile.Height-g.sea, f*flatTide)
		}
		if g.Fish[i] != 0 || g.Wood[i] != 0 || g.Wild[i] != 0 || g.Fertility[i] != 0 {
			t.Fatalf("a flat at %v has something growing on it", g.PosOf(i))
		}
	}
	if flats == 0 {
		t.Fatal("a globe with a sea has no flats")
	}
	if got := g.Count(func(t *Tile) bool { return t.Terrain == Flat }); got != flats {
		t.Fatalf("counted %d flats and %d", got, flats)
	}
	if share := float64(flats) / float64(len(g.Tiles)); share > 0.05 {
		t.Errorf("flats are %.1f%% of the globe", 100*share)
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
	g := NewLand(1, smallGlobe()).Grid
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
