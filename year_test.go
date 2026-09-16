package terra

import (
	"math"
	"testing"

	"github.com/LukasSelin/terra/geom"
)

// climateGlobe is the globe the climate tests read: the yardsticks' globe,
// made once for the whole suite.
func climateGlobe() *Land {
	return yardLand("globe", 1, GlobeTerms())
}

// A globe's frozen ground, its tundra and its ice keep to the high latitudes,
// and hold a sane share of the land: some, and not the fifth of it that went
// to bare rock when the growing frost was the permafrost line.
//
// The shares are of the land's area, each tile weighed by the cosine of its
// latitude: a globe's rows are all as many tiles long, so a tile near the pole
// is a sliver of the ground one at the equator is. Counted by the tile, seed
// 1's permafrost was 30.9% of the land once the sea about a place stopped
// warming every tile four degrees over its latitude; by the area it is 12.7%,
// against some fifteen per cent of the real world's exposed land (Obu and
// others, 2021).
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
		area := math.Cos(lat * math.Pi / 180)
		land += area
		if g.Frozen(p) {
			frozen += area
			if lat < 45 {
				t.Fatalf("permafrost at %.0f degrees, %v", lat, p)
			}
		}
		if g.Treeless(p) {
			treeless += area
			if lat < 45 {
				t.Fatalf("above the tree line at %.0f degrees, %v", lat, p)
			}
			if g.Tiles[i].Terrain == Forest {
				t.Fatalf("a wood stands above the tree line at %v", p)
			}
		}
		if g.Barren(p) {
			bare += area
		}
	}
	t.Logf("of the land's area: %.1f%% permafrost, %.1f%% above the tree line, %.1f%% under ice",
		100*frozen/land, 100*treeless/land, 100*bare/land)
	if frozen == 0 || frozen/land > 0.25 {
		t.Errorf("%.1f%% of the land is permafrost", 100*frozen/land)
	}
	if treeless == 0 || treeless/land > 0.25 {
		t.Errorf("%.1f%% of the land is above the tree line", 100*treeless/land)
	}
	if bare > treeless {
		t.Errorf("more ground is under ice (%.0f tiles' worth) than above the tree line (%.0f)", bare, treeless)
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
