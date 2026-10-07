package terra

import (
	"math"
	"testing"
)

// A globe's terms carry its forcing to its climate: one left unsaid is
// today's, the same to the bit as TodaysForcing given outright, and one with
// twice the carbon in its air is warmer at every latitude. A valley's weather
// is written down and reads none of it.
func TestTheTermsForcingReachesTheClimate(t *testing.T) {
	globe := GlobeTerms()
	given := globe
	given.Forcing = TodaysForcing()
	warm := globe
	warm.Forcing = TodaysForcing()
	warm.Forcing.CO2 *= 2
	a, b, c := NewClimateOn(globe), NewClimateOn(given), NewClimateOn(warm)
	for y := 0; y < globe.Height; y += 16 {
		if math.Float64bits(a.MeanAt(y)) != math.Float64bits(b.MeanAt(y)) {
			t.Fatalf("row %d: %v unsaid and %v given today's", y, a.MeanAt(y), b.MeanAt(y))
		}
		if c.MeanAt(y) <= a.MeanAt(y) {
			t.Errorf("row %d: %.2f C with twice the carbon, %.2f with today's", y, c.MeanAt(y), a.MeanAt(y))
		}
	}
	valley, hot := DefaultTerms(), DefaultTerms()
	hot.Forcing = warm.Forcing
	if NewClimateOn(valley).MeanAt(3) != NewClimateOn(hot).MeanAt(3) {
		t.Error("a valley's weather moved with its forcing")
	}
	bad := globe
	bad.Forcing = TodaysForcing()
	bad.Forcing.Solar = -1
	if bad.Check() == nil {
		t.Error("a globe under a negative sun passes its check")
	}
}
