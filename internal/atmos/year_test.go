package atmos

import (
	"math"
	"testing"
)

// Nelson and Outcalt's frost index read at the ground's surface crosses a half
// exactly where the year's mean air temperature crosses Permafrost, whatever
// the swing: the index and the isotherm are the same line, which is why the
// map needs only one of them. See frostIndex.
func TestTheFrostIndexIsThePermafrostLine(t *testing.T) {
	for _, swing := range []float64{0.5, 5, 15, 30} {
		if f := frostIndex(Permafrost-0.1, swing); f <= 0.5 {
			t.Errorf("swing %.1f: a mean just under the line has a frost index of %.3f", swing, f)
		}
		if f := frostIndex(Permafrost+0.1, swing); f >= 0.5 {
			t.Errorf("swing %.1f: a mean just over the line has a frost index of %.3f", swing, f)
		}
	}
}

// The tree line is the summer's. At the mean treeMean gives, the warmest month
// is at least ten degrees and the growing season at least 6.4, and one of the
// two is on its line; and a coast with a small swing needs a warmer year to
// grow a tree than a continent does, which is why the real tree line bends
// south along a cold coast.
func TestTheTreeLineIsTheSummers(t *testing.T) {
	for _, swing := range []float64{2, 8, 15, 25} {
		m := treeMean(swing)
		month := m + MonthPeak*swing
		season, ok := seasonMean(m, swing)
		if !ok || month < treeMonth-1e-6 || season < treeSeason-1e-6 {
			t.Fatalf("swing %.0f: at the tree line's mean of %.2f the warmest month is %.2f and the season %.2f", swing, m, month, season)
		}
		if math.Abs(month-treeMonth) > 1e-6 && math.Abs(season-treeSeason) > 1e-3 {
			t.Errorf("swing %.0f: the tree line at %.2f is on neither Köppen's line nor Körner's", swing, m)
		}
		if got := TreeLineMean(swing); math.Abs(got-m) > 0.05 {
			t.Errorf("swing %.0f: the table reads %.2f against %.2f", swing, got, m)
		}
	}
	if !(treeMean(3) > treeMean(20)) {
		t.Errorf("a maritime year needs %.1f to grow a tree and a continental one %.1f", treeMean(3), treeMean(20))
	}
}

// The seasons lag the sun by a month on land and two at sea, and the swing
// grows toward the poles and inland. The temperate latitude keeps exactly the
// swing the valley was tuned on.
func TestTheYearLagsAndSwingsByPlace(t *testing.T) {
	if got, want := LagAt(1), lagLand*Year/365.25; math.Abs(got-want) > 1e-9 {
		t.Errorf("land lags %.2f days, want %.2f", got, want)
	}
	if got, want := LagAt(0), lagSea*Year/365.25; math.Abs(got-want) > 1e-9 {
		t.Errorf("sea lags %.2f days, want %.2f", got, want)
	}
	if solarSwing(Temperate) != 1 || math.Abs(SwingAt(Temperate, ContMiddling)-Swing) > 1e-9 {
		t.Fatalf("the temperate latitude swings %.4f of the sun's and %.4f degrees", solarSwing(Temperate), SwingAt(Temperate, ContMiddling))
	}
	if !(SwingAt(70, 0.5) > SwingAt(45, 0.5) && SwingAt(45, 0.5) > SwingAt(10, 0.5)) {
		t.Error("the swing does not grow toward the pole")
	}
	// Twice and not three times: with the land and the sea trading six W/m²K
	// a degree between them, the balance's continent at sixty swings 19.2
	// and its sea 7.9. The real world's ratio there is larger - Yakutia
	// against the open North Atlantic - so this is a floor on the balance's
	// continentality, not a measure of it.
	if !(SwingAt(60, 1) > 2*SwingAt(60, 0)) {
		t.Errorf("a continent swings %.1f and the open sea %.1f at sixty degrees", SwingAt(60, 1), SwingAt(60, 0))
	}
	if SwingAt(-60, 1) >= 0 {
		t.Error("the south's year does not turn over")
	}
}

// A year of the climate's growth is what the Miami model says the place grows,
// against the temperate year's, and the temperate year with a metre of rain
// grows what it always did.
func TestAYearOfClimateGrowthIsItsNPP(t *testing.T) {
	for _, c := range []struct{ mean, swing, rain float64 }{{MeanTemp, Swing, 1000}, {25, 2, 2500}, {0, 20, 400}, {20, 8, 150}} {
		var sum float64
		for tick := 0; tick < Year; tick++ {
			sum += ClimateGrowth(c.mean+c.swing*SeasonAt(tick, 0), c.mean, c.swing, c.rain)
		}
		want := miamiNPP(c.mean, c.rain) / nppRef
		if got := sum / Year; math.Abs(got-want) > 0.01*want+1e-9 {
			t.Errorf("mean %.0f, swing %.0f, rain %.0f: a year grows %.3f, want %.3f", c.mean, c.swing, c.rain, got, want)
		}
	}
	if g := ClimateGrowth(-10, -10, 5, 500); g != 0 {
		t.Errorf("a year that never reaches the growing base grows %.3f", g)
	}
}
