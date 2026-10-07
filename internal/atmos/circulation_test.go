package atmos

import (
	"math"
	"testing"
)

// Today's circulation, read off the balance: the Hadley cells end at some
// thirty-two degrees over the year, the subtropical highs move north and
// south with the sun some four or five degrees (the real ones: the Azores
// and Pacific highs stand near thirty in January and near thirty-five to
// thirty-eight in July), and the ITCZ of a planet three tenths land swings
// some nine degrees either side - the real zonal mean's, from some five
// south in January to ten north in August (Waliser and Gautier, 1993). The
// real ITCZ's year's mean stands five degrees north, carried there by the
// Atlantic's overturning taking heat north across the equator (Frierson and
// others, 2013; Marshall and others, 2014), which the balance, carrying heat
// down its gradient alone, does not have: its mean stands a few tenths south,
// where the sun's being nearer in the south's summer puts it. That is logged
// and not held.
func TestTodaysHadleyCellsEndInTheHorseLatitudes(t *testing.T) {
	winter, spring, summer := CirculationUnder(Today(), -1), CirculationUnder(Today(), 0), CirculationUnder(Today(), 1)
	for _, c := range []struct {
		name string
		c    Circulation
	}{{"northern winter", winter}, {"equinox", spring}, {"northern summer", summer}} {
		t.Logf("%s: edges %.1f and %.1f, ITCZ %+.1f (sea %+.1f, land %+.1f)", c.name, c.c.North, c.c.South, c.c.ITCZ, c.c.ITCZSea, c.c.ITCZLand)
	}
	t.Logf("Held and Hou's own edge %.1f degrees under a contrast of %.3f and a tropopause of %.1f km",
		spring.HeldHou, spring.Contrast, spring.Tropopause/1000)
	if mean := (spring.North - spring.South) / 2; math.Abs(mean-hadleyToday) > 0.5 {
		t.Errorf("today's cells end at %.1f degrees, not %v", mean, hadleyToday)
	}
	if winter.North < 24 || winter.North > 31 || summer.North < 33 || summer.North > 40 {
		t.Errorf("the northern highs stand at %.1f in January and %.1f in July", winter.North, summer.North)
	}
	if summer.South < -31 || winter.South > -33 {
		t.Errorf("the southern highs stand at %.1f in January and %.1f in July", winter.South, summer.South)
	}
	swing := (summer.ITCZ - winter.ITCZ) / 2
	if swing < 6 || swing > 12 || summer.ITCZSea-winter.ITCZSea > summer.ITCZ-winter.ITCZ ||
		summer.ITCZ-winter.ITCZ > summer.ITCZLand-winter.ITCZLand {
		t.Errorf("the ITCZ swings %.1f either side, over the sea %.1f and over land %.1f", swing,
			(summer.ITCZSea-winter.ITCZSea)/2, (summer.ITCZLand-winter.ITCZLand)/2)
	}
	if math.Abs(spring.ITCZ) > 1.5 {
		t.Errorf("the ITCZ's year's mean is %+.1f", spring.ITCZ)
	}
	if spring.ITCZ < 3 {
		t.Logf("a known gap: the ITCZ's year's mean is %+.1f degrees, the real one's some five north", spring.ITCZ)
	}
}

// Held and Hou's cell is wider for a larger contrast between its equator and
// its pole, a higher tropopause and a slower spin, as the square root of each
// of the first two and inversely as the spin. A planet tilted further has a
// smaller contrast over its year, and so a narrower cell in the year's mean;
// but its ITCZ goes further into each summer, and its cells' edges with it,
// so that the winter's cell reaches further (Lindzen and Hou, 1988).
func TestTheCellsMoveAsTheoryHasThem(t *testing.T) {
	const c, h = 0.3, 15000.0
	base := heldHou(c, h, omega)
	if got := heldHou(c/2, h, omega) / base; math.Abs(got-math.Sqrt(0.5)) > 1e-12 {
		t.Errorf("half the contrast moves the edge by %.4f, not 1/√2", got)
	}
	if got := heldHou(c, 2*h, omega) / base; math.Abs(got-math.Sqrt2) > 1e-12 {
		t.Errorf("twice the tropopause moves the edge by %.4f, not √2", got)
	}
	if got := heldHou(c, h, omega/2) / base; math.Abs(got-2) > 1e-12 {
		t.Errorf("half the spin moves the edge by %.4f, not 2", got)
	}
	last := CirculationUnder(Today(), 0)
	lastSwing := CirculationUnder(Today(), 1).ITCZ
	for _, deg := range []float64{30, 40} {
		f := Today()
		f.Obliquity = deg * math.Pi / 180
		mean, summer, winter := CirculationUnder(f, 0), CirculationUnder(f, 1), CirculationUnder(f, -1)
		t.Logf("tilted %v degrees: contrast %.3f, edges %.1f in the year's mean, %.1f to %.1f in the north over its year; ITCZ %+.1f to %+.1f",
			deg, mean.Contrast, (mean.North-mean.South)/2, winter.North, summer.North, winter.ITCZ, summer.ITCZ)
		if mean.Contrast >= last.Contrast || mean.North-mean.South >= last.North-last.South {
			t.Errorf("tilted %v degrees the contrast is %.3f and the cells %.1f across, against %.3f and %.1f", deg,
				mean.Contrast, mean.North-mean.South, last.Contrast, last.North-last.South)
		}
		if summer.ITCZ <= lastSwing {
			t.Errorf("tilted %v degrees the ITCZ reaches %.1f in the summer, against %.1f", deg, summer.ITCZ, lastSwing)
		}
		last, lastSwing = mean, summer.ITCZ
	}
}

// The air comes down under the subtropical highs, and not at the equator or
// in the middle latitudes; where it comes down hardest it lays the trade
// inversion a kilometre up, and the air under it holds some third of the
// column's water.
func TestTheAirComesDownUnderTheHighs(t *testing.T) {
	e := &Env{circ: ebm(), Wrap: false}
	b := e.beltsAt(0)
	for _, c := range []struct {
		lat  float64
		down bool
	}{{0, false}, {5, false}, {20, true}, {-20, true}, {25, true}, {50, false}, {-50, false}} {
		w := b.subsidence(c.lat)
		t.Logf("%+v degrees: the air comes down %.1f mm/s, under a lid %.0f m up", c.lat, w*1000, lid(w))
		if (w > 0) != c.down {
			t.Errorf("at %v degrees the air comes down at %.4f m/s", c.lat, w)
		}
	}
	if got := lid(subsideMost); got != lidLeast {
		t.Errorf("under the strongest descent the lid stands %.0f m up", got)
	}
	if keep := lidKeeps(lidLeast, 25); keep < 0.25 || keep > 0.45 {
		t.Errorf("under a lid a kilometre up the air keeps %.2f of its water to rain", keep)
	}
	if lidKeeps(lid(0), 25) != 1 {
		t.Error("air that is not coming down is capped")
	}
}
