package veg

import "testing"

// The places' fires (see places: their rain and wind are the stations').
var (
	// A wet savanna of 1200 mm, its dry season the north's winter: the
	// Guinea savanna's year, where Staver and others (2011) find a savanna
	// and a forest each holding itself.
	guinea = place{name: "Guinea savanna, 1200 mm", lat: 10, mean: 25, swing: 3,
		herb: [Phases]float64{0.05, 0.3, 1, 0.6}, wood: [Phases]float64{0.15, 0.45, 1, 0.8},
		rain: [Phases]float64{10, 120, 750, 320}, wind: 5}
	kano    = places[1]
	manaus  = places[0]
	yakutsk = places[3]
)

// A wet savanna's year holds either a forest or a savanna: run from a
// closed canopy, the trees shade the grass out and nothing burns; run from
// open ground, the grass burns every few years and the fires hold the trees
// down.
func TestASavannaAndAForestEachHoldThemselves(t *testing.T) {
	forest, _, fb := guinea.run(false)
	savanna, _, sb := guinea.run(true)
	t.Logf("%s: from a forest, trees %.2f and %.3f burned a year; from open ground, trees %.2f and %.3f burned", guinea.name, forest.Trees(), fb, savanna.Trees(), sb)
	if forest.Trees() < 0.6 || fb > 0.05 {
		t.Errorf("the forest did not hold: trees %.2f, %.3f burned a year", forest.Trees(), fb)
	}
	if savanna.Trees() > 0.45 || sb < 0.15 {
		t.Errorf("the savanna did not hold: trees %.2f, %.3f burned a year", savanna.Trees(), sb)
	}
}

// The fires burn a savanna every few years, a boreal forest every century
// or two, and a rainforest not at all: GFED's burned area (Giglio and
// others, 2013).
func TestTheFiresBurnWhereTheyShould(t *testing.T) {
	for _, c := range []struct {
		pl     place
		lo, hi float64
	}{
		{kano, 0.1, 0.45},
		{yakutsk, 0.001, 0.02},
		{manaus, 0, 0.005},
	} {
		s, _, b := c.pl.run(false)
		t.Logf("%-34s trees %.2f, %.4f burned a year", c.pl.name, s.Trees(), b)
		if b < c.lo || b > c.hi {
			t.Errorf("%s: %.4f burned a year, want %.3f-%.3f", c.pl.name, b, c.lo, c.hi)
		}
	}
}

// A grass fire runs at its full rate through cured grass and barely through
// green: Cruz and others' (2015) coefficient is one at full curing and a
// sixth at half, and the grass is cured as far as its soil is dry. So a
// green sward, the same fuel, burns less than a dry season's.
func TestGreenGrassCarriesLittle(t *testing.T) {
	for _, c := range []struct{ w, lo, hi float64 }{
		{0, 0.99, 1},      // dry soil: all of the grass cured
		{0.1, 0.99, 1},    // SPITFIRE's live grass dry at a tenth
		{0.55, 0.15, 0.2}, // half of it green: Cruz's 0.166 at 50% cured
		{1, 0, 0.01},      // wet soil: green
	} {
		if got := curing(c.w); got < c.lo || got > c.hi {
			t.Errorf("curing at water %.2f: %.3f, want %.3f-%.3f", c.w, got, c.lo, c.hi)
		}
	}
	for w := 0.0; w < 1; w += 0.01 {
		if curing(w+0.01) > curing(w) {
			t.Fatalf("curing rises with the water at %.2f", w)
		}
	}
	// Kano's year, its grass as it stands, burned once in its dry season's
	// water and once in its wet season's.
	c := kano.climate()
	y := Read(&c)
	var pot [PFTs]Potential
	for p := range PFTs {
		pot[p] = y.Potential(p)
	}
	s, _, _ := kano.run(true)
	burn := func(w float64) float64 {
		f := Fire{Reach: 0.2, Litter: litterYears / q10(c.Mean)}
		f.Cured, f.Cured2 = f.Reach*curing(w), f.Reach*curing(w)*curing(w)
		return Burned(&s, &pot, &f)
	}
	dry, wet := burn(0.1), burn(0.6)
	t.Logf("Kano's grass: %.3f burned at a dry season's water, %.3f at a wet season's", dry, wet)
	if wet >= dry/2 {
		t.Errorf("a green sward burns %.3f, a cured one %.3f", wet, dry)
	}
}

// A fire through a savanna's trees and a rainforest's kills the rainforest's
// the more, and burns the grass to the ground without taking its cover.
func TestAFireKillsByTheBark(t *testing.T) {
	r, e, g := fireTraits[TropicalRaingreen].resist, fireTraits[TropicalEvergreen].resist, fireTraits[C4Grass].resist
	if r <= e || g != 1 {
		t.Errorf("a fire leaves %.2f of the savanna's trees, %.2f of the rainforest's, %.2f of the grass", r, e, g)
	}
}
