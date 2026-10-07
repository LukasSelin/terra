package terra

import (
	"math"
	"testing"
)

// A step of the pools is a step: two steps are one step as long as both,
// and a long enough one is the steady state.
func TestThePoolsStepExactly(t *testing.T) {
	g := one(900, 400)
	c := g.pedoClimateOf(0)
	a := g.carbonMatrix(0, c, g.coverOf(0, c.temp))
	var zero [4]float64
	for _, years := range []float64{3, 10, 250, 4e3} {
		once := carbonStep(a, zero, 2*years)
		twice := carbonStep(a, carbonStep(a, zero, years), years)
		for k := range once {
			if math.Abs(once[k]-twice[k]) > 1e-9*math.Max(1, once[k]) {
				t.Errorf("pool %d after %g years: %g in one step, %g in two", k, 2*years, once[k], twice[k])
			}
		}
	}
	steady, long := carbonSteady(a), carbonStep(a, zero, 1e6)
	for k := range steady {
		if math.Abs(steady[k]-long[k]) > 1e-6*math.Max(1, steady[k]) {
			t.Errorf("pool %d: steady state %g, a million years %g", k, steady[k], long[k])
		}
	}
	// And the steady state is input over turnover: what goes in a year
	// comes out a year.
	if !(steady[0] > 0 && steady[1] > 0 && steady[2] > steady[1] && steady[3] > 0) {
		t.Errorf("steady pools %v", steady)
	}
}

// The soil keeps more carbon where it is cold, and where it is waterlogged,
// and less where it burns; and only one reading of it is read: the fertility's
// organic matter is the pools'.
func TestTheSoilKeepsItsCarbonWhereItRotsSlowly(t *testing.T) {
	steady := func(temp, sodden float64) float64 {
		g := one(900, 400)
		c := g.pedoClimateOf(0)
		c.temp, c.sodden = temp, sodden
		x := carbonSteady(g.carbonMatrix(0, c, g.coverOf(0, temp)))
		return (x[1] + x[2] + x[3]) / g.nppOf(0, c)
	}
	// Per unit of what grows, so that it is the turnover being read.
	warm, cold, wet := steady(20, 0), steady(0.5, 0), steady(20, 1)
	if !(cold > 1.5*warm) {
		t.Errorf("carbon a unit of NPP: %.1f years at 0.5 °C against %.1f at 20", cold, warm)
	}
	if want := 1 / anaerobic; math.Abs(wet/warm-want) > 1e-6 {
		t.Errorf("waterlogged soil keeps %.2fx the aired soil's carbon, want %.2f", wet/warm, want)
	}

	g := one(900, 400)
	g.ripenSoil(0, 50e3)
	g.pedons = true
	o := g.organic(0)
	g.Tiles[0].Carbon *= 0.5
	if !(g.organic(0) < o) {
		t.Errorf("organic matter %.3f with the soil's carbon halved, %.3f before", g.organic(0), o)
	}
}

// The land's carbon, read against the earth's: the soil's top metre, the
// permafrost region's, the peat's and the vegetation's, each scaled from the
// globe's mean over its land to the earth's 148.9 million km²; and how long
// each biome's soil holds what falls on it.
func TestLandCarbonReadings(t *testing.T) {
	if testing.Short() {
		t.Skip("makes a globe")
	}
	g := globes()[0]
	s := g.LandCarbon()
	if s.Area <= 0 {
		t.Fatal("no land")
	}
	gt := earthLandArea * 1e12 / s.Area // kg on the map to kg on the earth
	var region, regionArea float64
	var stock, input [Biomes]float64
	for i := range g.Tiles {
		tl := &g.Tiles[i]
		if tl.Wet() || tl.Terrain.Tidal() || g.sunk(i) {
			continue
		}
		area := g.tileArea(i)
		if g.FrostShare(i) > 0 {
			regionArea += area
			region += area * (float64(tl.Carbon) + g.Litter(i) + g.FrozenCarbon(i) + g.PeatCarbon(i))
		}
		// The turnover is read where a soil has formed: bare ground keeps no
		// pools at all (see laySoilState).
		if forms(tl) && tl.Exposed > 0 {
			c := g.pedoClimateOf(i)
			in := g.inputOf(i, c, g.coverOf(i, c.temp))
			b := g.BiomeAt(i)
			stock[b] += area * (float64(tl.Carbon) + g.Litter(i))
			input[b] += area * in.litter
		}
	}
	report := func(what string, v, lo, hi float64, source string) {
		mark := "inside"
		if v < lo || v > hi {
			mark = "OUTSIDE"
		}
		t.Logf("%-46s %8.1f Gt   %s %g-%g (%s)", what, v, mark, lo, hi, source)
	}
	report("soil organic carbon, top metre", s.Soil()*gt/1e12, 1350, 1650, "Jobbágy & Jackson 2000: 1502")
	report("permafrost region soil carbon, 0-3 m", region*gt/1e12, 1000, 1700, "Hugelius et al. 2014")
	report("peat carbon", s.Peat*gt/1e12, 500, 600, "Yu et al. 2010")
	report("vegetation carbon", s.Vegetation*gt/1e12, 450, 650, "IPCC AR6 fig. 5.12")
	t.Logf("%-46s %8.1f Gt", "litter", s.Litter*gt/1e12)
	t.Logf("%-46s %8.1f Gt", "frozen ground under the top metre", s.Permafrost*gt/1e12)
	t.Logf("%-46s %8.1f Gt (%.1f%% of the land)", "  of which the region's top metre, litter, peat", (region-s.Permafrost)*gt/1e12, 100*regionArea/s.Area)
	t.Logf("%-46s fast %.1f, slow %.1f, passive %.1f Gt", "soil pools", s.Fast*gt/1e12, s.Slow*gt/1e12, s.Passive*gt/1e12)
	t.Logf("%-46s %8.2f Gt C a year (GFED4s, van der Werf et al. 2017: 2.2)", "fire emissions", s.Fire*gt/1e12)
	t.Logf("mean soil and litter turnover, years (stock over litterfall), by biome:")
	for b := range Biomes {
		if input[b] > 0 {
			t.Logf("  %-28s %7.0f", b, stock[b]/input[b])
		}
	}
}
