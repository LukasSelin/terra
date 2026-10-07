package veg

import "testing"

// firePlace is a place with the rain and the wind its fires read.
type firePlace struct {
	place
	rain [Phases]float64
	wind float64
}

func (pl firePlace) climate() Climate {
	c := pl.place.climate()
	c.Rain = pl.rain
	for k := range c.Wind {
		c.Wind[k] = pl.wind
	}
	return c
}

// The places' fires: their rain is the stations' by season (Walter and
// Lieth's atlas, to the ten mm), and their wind a station's mean.
var (
	// A wet savanna of 1200 mm, its dry season the north's winter: the
	// Guinea savanna's year, where Staver and others (2011) find a savanna
	// and a forest each holding itself.
	guinea = firePlace{place: place{name: "Guinea savanna, 1200 mm", lat: 10, mean: 25, swing: 3,
		herb: [Phases]float64{0.05, 0.3, 1, 0.6}, wood: [Phases]float64{0.15, 0.45, 1, 0.8}},
		rain: [Phases]float64{10, 120, 750, 320}, wind: 5}
	kano    = firePlace{place: places[1], rain: [Phases]float64{5, 60, 600, 135}, wind: 4}
	manaus  = firePlace{place: places[0], rain: [Phases]float64{700, 600, 300, 600}, wind: 2}
	yakutsk = firePlace{place: places[3], rain: [Phases]float64{40, 60, 150, 70}, wind: 4}
)

// run is a place's state run to the steady one, from BIOME4's answer or,
// open, from its open ground with the trees taken off it; and the share of
// it the steady state's fires burn a year.
func (pl firePlace) run(open bool) (State, float64) {
	c := pl.climate()
	y := Read(&c)
	var pot [PFTs]Potential
	for p := range PFTs {
		pot[p] = y.Potential(p)
	}
	f := y.Fire()
	s := Equilibrium(&pot)
	if open {
		for p := range PFTs {
			if Kinds[p].Tree {
				s.Cover[p], s.Mass[p] = 0, 0
			}
		}
	}
	Spin(&s, &pot, &f, 300)
	return s, Burned(&s, &pot, &f)
}

// A wet savanna's year holds either a forest or a savanna: run from a
// closed canopy, the trees shade the grass out and nothing burns; run from
// open ground, the grass burns every few years and the fires hold the trees
// down.
func TestASavannaAndAForestEachHoldThemselves(t *testing.T) {
	forest, fb := guinea.run(false)
	savanna, sb := guinea.run(true)
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
		pl     firePlace
		lo, hi float64
	}{
		{kano, 0.1, 0.45},
		{yakutsk, 0.001, 0.02},
		{manaus, 0, 0.005},
	} {
		s, b := c.pl.run(false)
		t.Logf("%-34s trees %.2f, %.4f burned a year", c.pl.name, s.Trees(), b)
		if b < c.lo || b > c.hi {
			t.Errorf("%s: %.4f burned a year, want %.3f-%.3f", c.pl.name, b, c.lo, c.hi)
		}
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
