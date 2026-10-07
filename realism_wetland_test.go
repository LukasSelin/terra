package terra

import (
	"math"
	"slices"
	"testing"

	"github.com/LukasSelin/terra/internal/atmos"
)

// The frozen ground, the wet ground and the peat held against the earth's.
//
// They read the yardsticks' globe, each tile weighed by the ground it stands
// for on a sphere, over the land that is not under ice: the exposed land the
// maps of the permafrost and the inventories of the wetlands and the peat are
// shares of. See frost.go and wetland.go.
func init() {
	realYardsticks = append(realYardsticks, wetYardsticks...)
}

var wetYardsticks = []realYardstick{
	// 13. The frozen ground.
	{yardstick: yardstick{
		name: "permafrost share of exposed land, by area, globe", unit: "", scale: "ground", lo: 0.12, hi: 0.18, slow: true,
		source:  "Obu et al. 2019: 13.9 million km2 of the northern hemisphere's ground underlain by permafrost, the share of each 1 km cell weighed in, about 15% of the exposed land",
		measure: func() float64 { return wetReadings(globes()).permafrost },
	}},
	{yardstick: yardstick{
		name: "median active-layer depth over permafrost, globe", unit: "m", scale: "ground", lo: 0.3, hi: 2, slow: true,
		source:  "Brown, Hinkel & Nelson 2000 (the CALM network): end-of-season thaw depths of 0.3 to 2 m over the tundra and the boreal permafrost, deeper in bedrock and gravel",
		measure: func() float64 { return wetReadings(globes()).activeLayer },
	}},
	// 14. The wet ground, and the peat.
	{yardstick: yardstick{
		name: "wetland share of exposed land, globe", unit: "", scale: "water", lo: 0.06, hi: 0.09, slow: true,
		source:  "Lehner & Döll 2004 (GLWD): wetlands 8.2-10.1 million km2, 6.2-7.6% of the land outside Antarctica and Greenland; Davidson & Finlayson 2018: up to about 9% with the seasonal wetlands",
		measure: func() float64 { return wetReadings(globes()).wetland },
	}},
	{yardstick: yardstick{
		name: "peatland share of exposed land, globe", unit: "", scale: "water", lo: 0.02, hi: 0.045, slow: true,
		source:  "Yu et al. 2010: peatlands of 40 cm of peat or more cover about 4.2 million km2, some 3% of the land",
		measure: func() float64 { return wetReadings(globes()).peatland },
	}},
	{yardstick: yardstick{
		name: "share of peatland in the cool humid belt and the wet tropics, globe", unit: "", scale: "water", lo: 0.85, hi: 1, slow: true,
		source:  "Yu et al. 2010; Page et al. 2011: some 87% of the world's peat poleward of 45 degrees and 11% within the tropics, a few in a hundred between; read as the share of the peatland area poleward of 45 degrees or within 23.4",
		measure: func() float64 { return wetReadings(globes()).belts },
	}},
	{yardstick: yardstick{
		name: "share of peatland in the tropics, globe", unit: "", scale: "water", lo: 0.1, hi: 0.45, slow: true,
		source:  "Page et al. 2011: tropical peatlands 0.44 million km2, 11% of the world's; Gumbricht et al. 2017, mapping the Amazon's and the Congo's: 1.7 million km2, over a third",
		measure: func() float64 { return wetReadings(globes()).tropics },
	}},
}

type wetReading struct {
	permafrost, activeLayer, wetland, peatland, belts, tropics float64
}

// wetReadings reads the frozen and the wet ground off the exposed land of
// worlds gs, each tile weighed by the cosine of its latitude.
func wetReadings(gs []*Grid) wetReading {
	return remember("wetground", gs, func() wetReading {
		type thaw struct{ depth, w float64 }
		var land, frozen, wet, peat, belts, tropics float64
		var thaws []thaw
		for _, g := range gs {
			for i := range g.Tiles {
				t := &g.Tiles[i]
				if g.underSea(i) || t.Wet() || t.Terrain.Tidal() || g.Barren(g.PosOf(i)) {
					continue
				}
				lat := math.Abs(latitudeOf(g, i/g.W))
				w := math.Cos(lat * math.Pi / 180)
				land += w
				s := g.FrostShare(i)
				frozen += s * w
				if s >= 0.5 {
					thaws = append(thaws, thaw{g.ActiveLayer(i), w})
				}
				if g.Wetland(i) {
					wet += w
				}
				if g.Peatland(i) {
					peat += w
					if lat >= 45 || lat < 23.4 {
						belts += w
					}
					if lat < 23.4 {
						tropics += w
					}
				}
			}
		}
		r := wetReading{math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN()}
		if land > 0 {
			r.permafrost, r.wetland, r.peatland = frozen/land, wet/land, peat/land
		}
		if peat > 0 {
			r.belts, r.tropics = belts/peat, tropics/peat
		}
		slices.SortFunc(thaws, func(a, b thaw) int {
			switch {
			case a.depth < b.depth:
				return -1
			case a.depth > b.depth:
				return 1
			}
			return 0
		})
		var all, run float64
		for _, h := range thaws {
			all += h.w
		}
		for _, h := range thaws {
			if run += h.w; run >= all/2 {
				r.activeLayer = h.depth
				break
			}
		}
		return r
	})
}

// The summer thaws the ground deeper where it is warmer: over the globe's
// permafrost the active layer under its warmest summers is deeper than under
// its coldest, as the CALM sites' are from the High Arctic to the boreal
// forest. And a peat holds its water: the peat of the globe stands almost
// wholly on ground whose water table is at its surface through most of the
// thawed year.
func TestTheThawFollowsTheSummerAndThePeatItsWater(t *testing.T) {
	if testing.Short() {
		t.Skip("a globe takes a while to make")
	}
	g := globes()[0]
	type site struct{ ddt, depth float64 }
	var sites []site
	var peat, soaked float64
	for i := range g.Tiles {
		if g.underSea(i) || g.Tiles[i].Wet() {
			continue
		}
		if g.FrostShare(i) >= 0.5 && g.PeatDepth(i) == 0 {
			mean, swing := g.meanOn(i, g.Height[i]), float64(g.swing[i])
			sites = append(sites, site{atmos.DegreeDays(mean, swing, 0), g.ActiveLayer(i)})
		}
		if g.Peatland(i) {
			peat++
			if g.WaterTable(i) >= peatSeason {
				soaked++
			}
		}
	}
	if len(sites) < 100 || peat == 0 {
		t.Fatalf("%d permafrost sites and %.0f peat tiles: nothing to read", len(sites), peat)
	}
	slices.SortFunc(sites, func(a, b site) int {
		switch {
		case a.ddt < b.ddt:
			return -1
		case a.ddt > b.ddt:
			return 1
		}
		return 0
	})
	third := len(sites) / 3
	var cold, warm float64
	for _, s := range sites[:third] {
		cold += s.depth
	}
	for _, s := range sites[len(sites)-third:] {
		warm += s.depth
	}
	cold, warm = cold/float64(third), warm/float64(third)
	t.Logf("active layer under the coldest third of summers %.2f m, the warmest %.2f m; %.0f%% of %.0f peat tiles waterlogged through their thawed year",
		cold, warm, 100*soaked/peat, peat)
	if warm <= 1.2*cold {
		t.Errorf("the warmest summers thaw %.2f m against the coldest's %.2f", warm, cold)
	}
	if soaked < 0.9*peat {
		t.Errorf("only %.0f of %.0f peat tiles are waterlogged", soaked, peat)
	}
}

// Stefan's thaw: deeper for a warmer summer as its square root, shallower in
// a wetter ground, and shallower again under a peat, which conducts a third
// as well as the mineral ground.
func TestStefansThaw(t *testing.T) {
	kt, _ := conductivity(0.6, mineralDry, mineralThawed, mineralFrozen)
	bare := stefan(1000, 0, 0, 1, 0.27, kt)
	if bare < 0.8 || bare > 2 {
		t.Errorf("a thousand degree-days thaw a loam %.2f m", bare)
	}
	if four := stefan(4000, 0, 0, 1, 0.27, kt); math.Abs(four/bare-2) > 1e-9 {
		t.Errorf("four times the degree-days thaw %.3f times as deep", four/bare)
	}
	if wet := stefan(1000, 0, 0, 1, 0.4, kt); wet >= bare {
		t.Errorf("a wetter ground thaws %.2f m against %.2f", wet, bare)
	}
	pt, pf := conductivity(peatSoaked, peatDry, peatThawed, peatFrozen)
	under := stefan(1000, 0.4, peatPores*peatSoaked, pt, 0.27, kt)
	if under >= bare {
		t.Errorf("under 40 cm of peat the ground thaws %.2f m against %.2f bare", under, bare)
	}
	if thin := stefan(1000, 0.001, peatPores*peatSoaked, pt, 0.27, kt); math.Abs(thin-bare) > 0.01 {
		t.Errorf("a millimetre of peat thaws %.3f m against %.3f bare", thin, bare)
	}
	_, kf := conductivity(0.6, mineralDry, mineralThawed, mineralFrozen)
	if rk, rp := kt/kf, pt/pf; rk < 0.55 || rk > 0.95 || rp < 0.25 || rp > 0.55 || rp >= rk {
		t.Errorf("thawed over frozen conductivity %.2f for a loam and %.2f for a peat", rk, rp)
	}
	if frostShareOf(0) != 0.5 || frostShareOf(frostSpread) != 0 || frostShareOf(-frostSpread) != 1 {
		t.Errorf("the share at nought, and either side of the spread: %v %v %v", frostShareOf(0), frostShareOf(frostSpread), frostShareOf(-frostSpread))
	}
}
