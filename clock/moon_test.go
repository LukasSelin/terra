package clock

import (
	"math"
	"testing"
)

// The moon comes round again: a synodic month on, it is the same moon.
func TestTheMoonComesRound(t *testing.T) {
	e := EpochOf(7)
	for tick := 0; tick < 400; tick += 13 {
		a := MoonOn(tick, e)
		// A whole number of months later, a whole number of days later - the
		// month is not a whole number of days, so step by enough months that
		// the days come out whole to within the test's tolerance.
		later := MoonOn(tick+int(math.Round(49*SynodicMonth)), e)
		drift := math.Abs(later.Age - a.Age)
		drift = math.Min(drift, SynodicMonth-drift)
		if want := math.Abs(49*SynodicMonth - math.Round(49*SynodicMonth)); math.Abs(drift-want) > 1e-6 {
			t.Fatalf("tick %d: the moon is %.4f days out after 49 months, want %.4f", tick, drift, want)
		}
	}
	if n := (Moon{Phase: 0}).Name(); n != "new" {
		t.Errorf("phase 0 is %q", n)
	}
	if n := (Moon{Phase: 0.5}).Name(); n != "full" {
		t.Errorf("phase 0.5 is %q", n)
	}
}

// Springs and neaps: the sea swings between M2+S2 when the sun and moon pull
// together and M2-S2 when they pull across, SpringLag days after the moon is
// new or full and a quarter of a month from it. The moon's distance moves both
// by up to TideN2Share of the moon's part.
func TestSpringsAndNeaps(t *testing.T) {
	e := EpochOf(11)
	lo, hi := math.Inf(1), math.Inf(-1)
	for tick := 0; float64(tick) < 3*SynodicMonth; tick++ {
		tide := TideOn(tick, e)
		if tide.Low != -tide.High {
			t.Fatalf("tick %d: high %v and low %v are not the same distance from mean sea", tick, tide.High, tide.Low)
		}
		if tide.Springs < 0 || tide.Springs > 1 {
			t.Fatalf("tick %d: springs reads %v", tick, tide.Springs)
		}
		lo, hi = math.Min(lo, tide.High), math.Max(hi, tide.High)
	}
	if hi > TideMax+1e-12 || hi < TideM2*(1-TideN2Share)+TideS2 {
		t.Errorf("the highest springs came to %.3f m, want between %.3f and %.3f", hi, TideM2*(1-TideN2Share)+TideS2, TideMax)
	}
	if lo < TideM2*(1-TideN2Share)-TideS2-1e-12 || lo > TideM2*(1+TideN2Share)-TideS2 {
		t.Errorf("the weakest neaps came to %.3f m", lo)
	}
	// On a day the moon is exactly SpringLag past new, the two tides are in
	// line and the range is the moon's plus the sun's, whatever the distance.
	// Set the founding moon so that tick 0 is that day.
	e.Synodic = SpringLag
	tide := TideOn(0, e)
	near := math.Cos(2 * math.Pi * e.Anomalistic / AnomalisticMonth)
	if want := TideM2*(1+TideN2Share*near) + TideS2; math.Abs(tide.High-want) > 1e-12 {
		t.Errorf("at springs the sea rose %.6f m, want %.6f", tide.High, want)
	}
}
