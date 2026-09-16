package atmos

import (
	"math"
	"testing"
)

// The energy balance, read by latitude, is the real world's zonal mean
// surface temperature to a couple of degrees from the equator to sixty
// degrees (Legates and Willmott, 1990; Peixoto and Oort, 1992, Fig. 7.4),
// falls all the way to the pole, and is the same in both hemispheres.
func TestTheSunGivesEachLatitudeItsWarmth(t *testing.T) {
	for _, c := range []struct{ lat, want float64 }{{0, 26.5}, {15, 25.5}, {30, 20.5}, {45, 11}, {60, 1}} {
		if got := ZonalMean(c.lat); math.Abs(got-c.want) > 2 {
			t.Errorf("%v degrees: the balance's mean is %.1f C, the real world's %.1f", c.lat, got, c.want)
		}
		if n, s := ZonalMean(c.lat), ZonalMean(-c.lat); math.Abs(n-s) > 1.5 {
			t.Errorf("%v degrees: %.1f C in the north and %.1f in the south", c.lat, n, s)
		}
	}
	for lat := 0.0; lat < 89; lat++ {
		if ZonalMean(lat+1) > ZonalMean(lat)+1e-9 {
			t.Fatalf("the mean rises from %.0f to %.0f degrees", lat, lat+1)
		}
	}
}

// A continent's year is large and early and the sea's small and late, for the
// heat each holds and nothing else: at forty-five degrees the balance's land
// swings some eighteen degrees either side and peaks a month after the sun,
// its sea a few degrees and two or three months after (Hartmann 2016, ch. 2:
// 30-60 day lags over land, 60-90 over the mixed layer).
func TestTheLandsYearIsLargeAndEarly(t *testing.T) {
	e := ebm()
	land, sea := e.at(&e.swingL, Temperate), e.at(&e.swingS, Temperate)
	if land < 12 || land > 25 || sea < 1.5 || sea > 6 {
		t.Errorf("at %v degrees the land swings %.1f and the sea %.1f", Temperate, land, sea)
	}
	if lagLand < 20 || lagLand > 50 || lagSea < 55 || lagSea > 95 {
		t.Errorf("the land lags %.0f days and the sea %.0f", lagLand, lagSea)
	}
}

// The balance's ice is the sea's: poleward of where the summer's warmth runs
// out, the sea is under ice and its surface never over melting.
func TestTheSeaIceHoldsItsSurfaceAtMelting(t *testing.T) {
	if got := seaSurface(-1e8, heatSea, 500); got > 0 {
		t.Errorf("ice under a polar summer's sun stands at %.1f C", got)
	}
	if got := seaSurface(-1e8, heatSea, 0); got > -20 {
		t.Errorf("ice in the polar night stands at %.1f C", got)
	}
	if got := seaSurface(heatSea*5, heatSea, 0); math.Abs(got-5) > 1e-9 {
		t.Errorf("open water holding five degrees of heat stands at %.1f C", got)
	}
}
