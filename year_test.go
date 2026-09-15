package terra

import (
	"math"
	"sync"
	"testing"

	"github.com/LukasSelin/terra/geom"
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
		month := m + monthPeak*swing
		season, ok := seasonMean(m, swing)
		if !ok || month < treeMonth-1e-6 || season < treeSeason-1e-6 {
			t.Fatalf("swing %.0f: at the tree line's mean of %.2f the warmest month is %.2f and the season %.2f", swing, m, month, season)
		}
		if math.Abs(month-treeMonth) > 1e-6 && math.Abs(season-treeSeason) > 1e-3 {
			t.Errorf("swing %.0f: the tree line at %.2f is on neither Köppen's line nor Körner's", swing, m)
		}
		if got := treeLineMean(swing); math.Abs(got-m) > 0.05 {
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
	if got, want := lagAt(1), lagLand*Year/365.25; math.Abs(got-want) > 1e-9 {
		t.Errorf("land lags %.2f days, want %.2f", got, want)
	}
	if got, want := lagAt(0), lagSea*Year/365.25; math.Abs(got-want) > 1e-9 {
		t.Errorf("sea lags %.2f days, want %.2f", got, want)
	}
	if solarSwing(Temperate) != 1 || math.Abs(swingAt(Temperate, contMiddling)-Swing) > 1e-9 {
		t.Fatalf("the temperate latitude swings %.4f of the sun's and %.4f degrees", solarSwing(Temperate), swingAt(Temperate, contMiddling))
	}
	if !(swingAt(70, 0.5) > swingAt(45, 0.5) && swingAt(45, 0.5) > swingAt(10, 0.5)) {
		t.Error("the swing does not grow toward the pole")
	}
	if !(swingAt(60, 1) > 3*swingAt(60, 0)) {
		t.Errorf("a continent swings %.1f and the open sea %.1f at sixty degrees", swingAt(60, 1), swingAt(60, 0))
	}
	if swingAt(-60, 1) >= 0 {
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
			sum += climateGrowth(c.mean+c.swing*seasonAt(tick, 0), c.mean, c.swing, c.rain)
		}
		want := miamiNPP(c.mean, c.rain) / nppRef
		if got := sum / Year; math.Abs(got-want) > 0.01*want+1e-9 {
			t.Errorf("mean %.0f, swing %.0f, rain %.0f: a year grows %.3f, want %.3f", c.mean, c.swing, c.rain, got, want)
		}
	}
	if g := climateGrowth(-10, -10, 5, 500); g != 0 {
		t.Errorf("a year that never reaches the growing base grows %.3f", g)
	}
}

var (
	climateGlobeOnce sync.Once
	climateGlobeLand *Land
)

// climateGlobe is the globe the climate tests read, made once.
func climateGlobe() *Land {
	climateGlobeOnce.Do(func() { climateGlobeLand = NewLand(1, GlobeTerms()) })
	return climateGlobeLand
}

// A globe's frozen ground, its tundra and its ice keep to the high latitudes,
// and hold a sane share of the land: some, and not the fifth of it that went
// to bare rock when the growing frost was the permafrost line.
func TestTheColdKeepsToThePoles(t *testing.T) {
	if testing.Short() {
		t.Skip("a globe takes a while to make")
	}
	w := climateGlobe()
	g := w.Grid
	var land, frozen, treeless, bare float64
	for i := range g.Tiles {
		if g.Tiles[i].Wet() {
			continue
		}
		p := g.PosOf(i)
		lat := math.Abs(w.Climate.latitude(p.Y))
		land++
		if g.Frozen(p) {
			frozen++
			if lat < 45 {
				t.Fatalf("permafrost at %.0f degrees, %v", lat, p)
			}
		}
		if g.Treeless(p) {
			treeless++
			if lat < 45 {
				t.Fatalf("above the tree line at %.0f degrees, %v", lat, p)
			}
			if g.Tiles[i].Terrain == Forest {
				t.Fatalf("a wood stands above the tree line at %v", p)
			}
		}
		if g.Barren(p) {
			bare++
		}
	}
	t.Logf("of the land: %.1f%% permafrost, %.1f%% above the tree line, %.1f%% under ice",
		100*frozen/land, 100*treeless/land, 100*bare/land)
	if frozen == 0 || frozen/land > 0.25 {
		t.Errorf("%.1f%% of the land is permafrost", 100*frozen/land)
	}
	if treeless == 0 || treeless/land > 0.25 {
		t.Errorf("%.1f%% of the land is above the tree line", 100*treeless/land)
	}
	if bare > treeless {
		t.Errorf("more ground is under ice (%.0f tiles) than above the tree line (%.0f)", bare, treeless)
	}
}

// Forest follows the water. Ground where the rain outruns what the air could
// take back is wooded far more often than ground where the air could take
// twice the rain, and a desert, where it could take four times, holds almost
// nothing - the old share put its woods round the lakes of the desert belt.
func TestForestFollowsTheWater(t *testing.T) {
	if testing.Short() {
		t.Skip("a globe takes a while to make")
	}
	g := climateGlobe().Grid
	var humid, humidWood, dry, dryWood, desert, desertWood float64
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if t.Wet() || t.Terrain.Tidal() || g.Treeless(g.PosOf(i)) {
			continue
		}
		pet := g.pet(i)
		if pet <= 0 || g.Rain(i) <= 0 {
			continue
		}
		wood := 0.0
		if t.Terrain == Forest {
			wood = 1
		}
		switch phi := pet / g.Rain(i); {
		case phi < 1:
			humid, humidWood = humid+1, humidWood+wood
		case phi > 4:
			desert, desertWood = desert+1, desertWood+wood
		case phi > 2:
			dry, dryWood = dry+1, dryWood+wood
		}
	}
	if humid == 0 || dry == 0 {
		t.Fatalf("a globe with %.0f humid tiles and %.0f dry ones", humid, dry)
	}
	h, d := humidWood/humid, dryWood/dry
	t.Logf("wooded: %.1f%% of humid ground, %.1f%% of dry, %.2f%% of desert", 100*h, 100*d, 100*desertWood/max(desert, 1))
	if h < 0.3 {
		t.Errorf("only %.1f%% of the humid ground is wooded", 100*h)
	}
	if h < 3*d {
		t.Errorf("humid ground is %.1f%% wooded and dry ground %.1f%%", 100*h, 100*d)
	}
	if desert > 0 && desertWood/desert > 0.01 {
		t.Errorf("%.1f%% of the desert is wooded", 100*desertWood/desert)
	}
}

// The spell is the week's weather drawn for the whole planet, and the day's
// weather is the week's weather that actually moved: once the day's weather
// has been asked for, a globe reads the one and not both.
func TestTheDaysWeatherReplacesTheSpell(t *testing.T) {
	if testing.Short() {
		t.Skip("a globe takes a while to make")
	}
	w := climateGlobe()
	p := geom.Pos{X: 300, Y: 140}
	saved := w.Climate.Spell
	defer func() { w.Climate.Spell = saved }()
	if w.today() {
		t.Fatal("the shared globe already has the day's weather")
	}
	w.Climate.Spell = 0
	calm := w.TempAt(p)
	w.Climate.Spell = 5
	if got := w.TempAt(p) - calm; math.Abs(got-5) > 1e-9 {
		t.Fatalf("with no day's weather a spell of five adds %.3f", got)
	}
	c := w.Grid.Clone()
	day := *w
	day.Grid, day.Weather = c, nil
	day.AdvanceWeather()
	day.Climate.Spell = 0
	calm = day.TempAt(p)
	day.Climate.Spell = 5
	if got := day.TempAt(p) - calm; got != 0 {
		t.Fatalf("with the day's weather a spell of five still adds %.3f", got)
	}
}
