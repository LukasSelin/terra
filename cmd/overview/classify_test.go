package main

import (
	"math"
	"testing"

	"github.com/LukasSelin/terra"
)

// groups is the share of the dry land in each of Köppen's five groups.
//
// On a globe it is a share of the land's area and not of its tiles. The rows
// are equal steps of latitude, so a tile near a pole covers less of the
// planet than one at the equator by the cosine of its latitude, as the weather
// reads it; counted a tile apiece, the land poleward of sixty stood for three
// tenths of a globe's land against the eighth of a sphere it is, and the real
// shares this is held to are shares of area.
func groups(t *testing.T, land *terra.Land) map[byte]float64 {
	c := classify(land)
	g := land.Grid
	out := map[byte]float64{}
	n := 0.0
	for i, k := range c.Koppen {
		if k == "" {
			continue
		}
		w := 1.0
		if g.Wrap {
			lat := 90 - 180*(float64(i/g.W)+0.5)/float64(g.H)
			w = math.Cos(lat * math.Pi / 180)
		}
		out[k[0]] += w
		n += w
	}
	if n == 0 {
		t.Fatal("no dry land was classified")
	}
	for g := range out {
		out[g] /= n
	}
	return out
}

// The valley is one temperate latitude's weather, and all of it is C.
func TestTheValleyIsTemperate(t *testing.T) {
	g := groups(t, terra.NewLand(1, terra.DefaultTerms()))
	if g['C'] < 0.99 {
		t.Errorf("the valley's land is %.0f%% C: %v", 100*g['C'], g)
	}
}

// A globe has the world's climates: its dry belts, its temperate coasts and
// its continental interiors, each on a real share of the land. The real
// world's land is some 30% B, 13% C and 25% D by Peel and others (2007).
func TestAGlobeHasTheWorldsClimates(t *testing.T) {
	if testing.Short() {
		t.Skip("a globe takes a while to make")
	}
	g := groups(t, terra.NewLand(1, terra.GlobeTerms()))
	t.Logf("Köppen groups of the land: A %.1f%%, B %.1f%%, C %.1f%%, D %.1f%%, E %.1f%%",
		100*g['A'], 100*g['B'], 100*g['C'], 100*g['D'], 100*g['E'])
	for _, k := range []byte{'B', 'C', 'D'} {
		if g[k] < 0.05 {
			t.Errorf("group %c holds %.1f%% of the land", k, 100*g[k])
		}
	}
	if g['B'] > 0.6 {
		t.Errorf("%.0f%% of the land is dry", 100*g['B'])
	}
}

// The line between desert and steppe is half the line between steppe and
// anything else, and the line itself moves with when the rain falls.
func TestDesertIsHalfTheSteppeLine(t *testing.T) {
	cases := []struct {
		mean, rain, warm float64
		want             string
	}{
		{20, 250, 0.5, "BWh"}, // even rain: line at 540, desert under 270
		{20, 300, 0.5, "BSh"},
		{20, 500, 0.8, "BSh"}, // summer rain: line at 680
		{20, 500, 0.2, "C"},   // winter rain: line at 400
		{5, 100, 0.5, "BWk"},
	}
	for _, c := range cases {
		got := classOf(c.mean, c.rain, c.warm)
		if got[:len(c.want)] != c.want {
			t.Errorf("mean %.0f, rain %.0f, %.0f%% in summer: %s, want %s", c.mean, c.rain, 100*c.warm, got, c.want)
		}
	}
}

// classOf is terra.KoppenOf for a year swinging five degrees either side of mean.
func classOf(mean, rain, warm float64) string {
	return terra.KoppenOf(mean, mean-5, mean+5, rain, warm, false)
}
