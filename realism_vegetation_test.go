package terra

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/LukasSelin/terra/internal/veg"
)

// The vegetation held against the earth's.
//
// What grows on the land (vegetation.go, package veg) is read six ways: the
// biomes it makes against the climates Whittaker (1975) found each biome
// under; the carbon it holds against the earth's; its leaf area, biome by
// biome, against what the satellites read; whether a wood dies back where
// its ground stops suiting it; the share of each biome its fires burn a
// year, against what the satellites map burned; and whether the tropics'
// tree cover, over the band of rain a savanna and a forest each hold
// themselves in, falls into the two states Staver and others (2011) found
// it in.
func init() {
	realYardsticks = append(realYardsticks, vegetationYardsticks...)
}

var vegetationYardsticks = []realYardstick{
	{yardstick: yardstick{
		name: "vegetation carbon, scaled to the earth's land, globe", unit: "Gt C", scale: "water", lo: 450, hi: 650, slow: true,
		source:  "IPCC AR6 WG1 fig. 5.12 (Canadell et al. 2021): 450-650 Pg C in the land's vegetation; read as the globe's mean carbon a square metre of land over the earth's 148.9 million km2",
		measure: func() float64 { return landVegetation(globes()).carbon },
	}},
	{yardstick: yardstick{
		name: "leaf area index, tropical rainforest, globe", unit: "", scale: "water", lo: 4, hi: 7, slow: true,
		source:  "Myneni et al. 2002 (MODIS LAI): broadleaf evergreen forest 4.5-6 through the year; Asner, Scurlock & Hicke 2003: tropical evergreen forest 4.9 +- 1.7",
		measure: func() float64 { return landVegetation(globes()).lai[TropicalRainforest] },
	}},
	{yardstick: yardstick{
		name: "leaf area index, boreal forest, globe", unit: "", scale: "water", lo: 2, hi: 4, slow: true,
		source:  "Myneni et al. 2002 (MODIS LAI): needleleaf forest ~3 at the summer's peak, 1-2 on the year's mean; Asner, Scurlock & Hicke 2003: boreal evergreen needleleaf 2.7 +- 1.3",
		measure: func() float64 { return landVegetation(globes()).lai[BorealForest] },
	}},
	{yardstick: yardstick{
		name: "leaf area index, temperate grassland, globe", unit: "", scale: "water", lo: 0.5, hi: 2.5, slow: true,
		source:  "Myneni et al. 2002 (MODIS LAI): grasses and cereal crops 1-2; Asner, Scurlock & Hicke 2003: grasslands 1.7 +- 1.2",
		measure: func() float64 { return landVegetation(globes()).lai[TemperateGrassland] },
	}},
	{yardstick: yardstick{
		name: "leaf area index, deserts, globe", unit: "", scale: "water", lo: 0, hi: 0.5, slow: true,
		source:  "Myneni et al. 2002 (MODIS LAI): barren and sparsely vegetated land under 0.5",
		measure: func() float64 { return landVegetation(globes()).desertLAI },
	}},
	{yardstick: yardstick{
		name: "share of the savannas burned a year, globe", unit: "", scale: "water", lo: 0.1, hi: 0.45, slow: true,
		source:  "Giglio, Randerson & van der Werf 2013 (GFED4 burned area): the savannas and grasslands of Africa, Australia and South America burn 20-40% a year where they burn most, and the savanna biome as a whole a tenth and more",
		measure: func() float64 { return landVegetation(globes()).burned[Savanna] },
	}},
	{yardstick: yardstick{
		name: "share of the boreal forest burned a year, globe", unit: "", scale: "water", lo: 0.002, hi: 0.015, slow: true,
		source:  "Giglio, Randerson & van der Werf 2013 (GFED4 burned area): boreal forest 0.5-1% a year; widened to the spread between Eurasia's and North America's",
		measure: func() float64 { return landVegetation(globes()).burned[BorealForest] },
	}},
	{yardstick: yardstick{
		name: "share of the tropical rainforest burned a year, globe", unit: "", scale: "water", lo: 0, hi: 0.01, slow: true,
		source:  "Giglio, Randerson & van der Werf 2013 (GFED4 burned area): intact tropical rainforest burns near nothing; what burns at its edges is lit by people clearing it",
		measure: func() float64 { return landVegetation(globes()).burned[TropicalRainforest] },
	}},
	{yardstick: yardstick{
		name: "bimodality of tropical tree cover at 1000-2500 mm of rain, globe", unit: "", scale: "water", lo: 5.0 / 9, hi: 1, slow: true,
		source:  "Staver, Archibald & Levin 2011 (MODIS tree cover over Africa, Australia and South America): tree cover is bimodal between 1000 and 2500 mm of rain, savanna under 50% and forest over; read as Sarle's bimodality coefficient of the area-weighted tree cover, over 5/9 bimodal (Pfister and others, 2013)",
		measure: func() float64 { return landVegetation(globes()).bimodality },
	}},
	{yardstick: yardstick{
		name: "land in its biome's Whittaker climate, area-weighted, globe", unit: "", scale: "water", lo: 0.6, hi: 1, slow: true,
		source:  "Whittaker 1975 fig. 4.10, as Ricklefs 2008 redraws it: each biome's stations fall in its envelope of mean temperature and rain; the band's floor is not a measured figure, the envelopes being drawn round most of a biome's stations and read here as boxes",
		measure: func() float64 { return landVegetation(globes()).whittaker },
	}},
}

// whittakerBox is a climate Whittaker (1975) found a biome under: the year's
// mean, in degrees, and its rain, in cm, each between two figures. The boxes
// are read off his figure as Ricklefs (2008) redraws it, to the nearest few
// degrees and ten centimetres, and they overlap as his envelopes do.
type whittakerBox struct{ tlo, thi, plo, phi float64 }

var (
	wTundra          = whittakerBox{-15, -4, 0, 100}
	wBoreal          = whittakerBox{-10, 5, 25, 230}
	wTemperateDesert = whittakerBox{-5, 21, 0, 60}
	wWoodland        = whittakerBox{0, 22, 30, 150}
	wTemperateForest = whittakerBox{3, 22, 70, 230}
	wTemperateRain   = whittakerBox{8, 22, 180, 350}
	wTropicalSeason  = whittakerBox{18, 30, 50, 280}
	wSubtropicDesert = whittakerBox{16, 30, 0, 70}
	wTropicalRain    = whittakerBox{20, 30, 230, 450}
)

// whittakerOf is the envelopes each biome falls under in Whittaker's scheme.
var whittakerOf = [Biomes][]whittakerBox{
	TundraBiome:              {wTundra},
	BorealForest:             {wBoreal},
	TemperateConiferForest:   {wTemperateForest, wTemperateRain},
	TemperateBroadleafForest: {wTemperateForest, wTemperateRain},
	TemperateWoodland:        {wWoodland, wTemperateForest},
	TemperateGrassland:       {wTemperateDesert, wWoodland},
	Shrubland:                {wWoodland, wTemperateDesert, wSubtropicDesert},
	ColdDesert:               {wTemperateDesert, wTundra},
	HotDesert:                {wSubtropicDesert},
	TropicalRainforest:       {wTropicalRain},
	TropicalSeasonalForest:   {wTropicalSeason},
	Savanna:                  {wTropicalSeason},
}

// earthLandArea is the earth's land, in million km2.
const earthLandArea = 148.9

// vegReading is what landVegetation reads off the land.
type vegReading struct {
	// The share of the land, by area, each biome holds, and the share of
	// each biome's area whose climate is in its Whittaker envelope; and the
	// whole land's, ice left out.
	share, inEnvelope [Biomes]float64
	whittaker         float64
	// The mean leaf area index of each biome, and of the two deserts.
	lai       [Biomes]float64
	desertLAI float64
	// The vegetation's carbon, its mean over the land times the earth's
	// land, Gt; and the share of the land under trees.
	carbon, trees float64
	// The share of each biome's area burned a year, and of the whole land's,
	// and the land burned a year scaled to the earth's, million km2.
	burned              [Biomes]float64
	burnedAll, burnedKm float64
	// The tropics' tree cover where 1000 to 2500 mm of rain falls: by area,
	// in tenths of the ground, and Sarle's bimodality coefficient of it. And
	// the same at each band of rain, in 500s of mm from 500 to 3000.
	cover       [10]float64
	bimodality  float64
	coverByRain [5][10]float64
}

// The tropics' band of rain over which Staver and others (2011) found a
// savanna and a forest each holding itself, mm a year, and what a tropical
// tile is: a year's mean of tropicalMean or more.
const (
	staverLo     = 1000.0
	staverHi     = 2500.0
	tropicalMean = 18.0
)

// bimodality is Sarle's bimodality coefficient of the weighted values: the
// square of their skewness, plus one, over their kurtosis. It is 5/9 for a
// uniform spread, under it for a single hump, and over it for two.
func bimodality(v, w []float64) float64 {
	var sw, m float64
	for i := range v {
		sw += w[i]
		m += w[i] * v[i]
	}
	if sw <= 0 {
		return 0
	}
	m /= sw
	var m2, m3, m4 float64
	for i := range v {
		d := v[i] - m
		m2 += w[i] * d * d
		m3 += w[i] * d * d * d
		m4 += w[i] * d * d * d * d
	}
	m2, m3, m4 = m2/sw, m3/sw, m4/sw
	if m2 <= 0 {
		return 0
	}
	skew := m3 / math.Pow(m2, 1.5)
	kurt := m4 / (m2 * m2)
	return (skew*skew + 1) / kurt
}

// landVegetation reads the vegetation off gs's dry land, by area.
func landVegetation(gs []*Grid) vegReading {
	return remember("landVegetation", gs, func() vegReading {
		var r vegReading
		var land, carbon, trees, desert, desertLAI, inside, read, burned float64
		var band, bandArea []float64
		for _, g := range gs {
			for i := range g.Tiles {
				if g.sunk(i) || g.Tiles[i].Wet() || g.Tiles[i].Terrain.Tidal() {
					continue
				}
				y := i / g.W
				area := g.air.Dx[y] * g.air.Dy
				b := g.BiomeAt(i)
				land += area
				r.share[b] += area
				lai := g.LAI(i)
				r.lai[b] += area * lai
				carbon += area * g.VegCarbon(i)
				trees += area * g.TreeCover(i)
				r.burned[b] += area * g.Burned(i)
				burned += area * g.Burned(i)
				if b == HotDesert || b == ColdDesert {
					desert += area
					desertLAI += area * lai
				}
				if b == IceBiome || b == NoBiome {
					continue
				}
				mean, _, _ := g.YearAt(i)
				rain := g.Rain(i) / 10
				if mean >= tropicalMean {
					tc := g.TreeCover(i)
					bin := min(9, int(tc*10))
					if mm := rain * 10; mm >= staverLo && mm < staverHi {
						band, bandArea = append(band, tc), append(bandArea, area)
						r.cover[bin] += area
					}
					if k := int(rain*10/500) - 1; k >= 0 && k < len(r.coverByRain) {
						r.coverByRain[k][bin] += area
					}
				}
				read += area
				for _, w := range whittakerOf[b] {
					if mean >= w.tlo && mean <= w.thi && rain >= w.plo && rain <= w.phi {
						r.inEnvelope[b] += area
						inside += area
						break
					}
				}
			}
		}
		for b := range Biomes {
			if r.share[b] > 0 {
				r.burned[b] /= r.share[b]
				r.lai[b] /= r.share[b]
				r.inEnvelope[b] /= r.share[b]
			}
			if land > 0 {
				r.share[b] /= land
			}
		}
		if land > 0 {
			r.carbon = carbon / land * earthLandArea
			r.trees = trees / land
			r.burnedAll = burned / land
			r.burnedKm = r.burnedAll * earthLandArea
		}
		r.bimodality = bimodality(band, bandArea)
		normal := func(h *[10]float64) {
			var sum float64
			for _, a := range h {
				sum += a
			}
			for k := range h {
				if sum > 0 {
					h[k] /= sum
				}
			}
		}
		normal(&r.cover)
		for k := range r.coverByRain {
			normal(&r.coverByRain[k])
		}
		if desert > 0 {
			r.desertLAI = desertLAI / desert
		}
		if read > 0 {
			r.whittaker = inside / read
		}
		return r
	})
}

// logVegetation writes what landVegetation reads off gs.
func logVegetation(t *testing.T, what string, gs []*Grid) vegReading {
	r := landVegetation(gs)
	t.Logf("%s: carbon %.0f Gt C scaled to the earth's land, %.3f of the land under trees, %.3f of it in its biome's Whittaker climate", what, r.carbon, r.trees, r.whittaker)
	t.Logf("%s: %.4f of the land burned a year, %.2f million km2 scaled to the earth's", what, r.burnedAll, r.burnedKm)
	for b := Biome(1); b < Biomes; b++ {
		if r.share[b] == 0 {
			continue
		}
		t.Logf("%s:   %-27s %.3f of the land, LAI %.2f, %.2f in its Whittaker climate, %.4f burned a year", what, b, r.share[b], r.lai[b], r.inEnvelope[b], r.burned[b])
	}
	hist := func(h *[10]float64) string {
		var b strings.Builder
		for _, a := range h {
			fmt.Fprintf(&b, " %4.2f", a)
		}
		return b.String()
	}
	t.Logf("%s: tropical tree cover at %.0f-%.0f mm, by tenths of the ground:%s; bimodality %.3f (5/9 uniform)", what, staverLo, staverHi, hist(&r.cover), r.bimodality)
	for k := range r.coverByRain {
		t.Logf("%s:   at %4d-%4d mm:%s", what, 500*(k+1), 500*(k+2), hist(&r.coverByRain[k]))
	}
	return r
}

// The vegetation on the globes, logged: the biomes, their leaf area and
// climates, the carbon. The yardsticks above read the globe's.
func TestTheVegetationOnTheGlobes(t *testing.T) {
	if testing.Short() {
		t.Skip("needs a globe")
	}
	logVegetation(t, "small globes 1-2", smallGlobes(2))
	logVegetation(t, "globe", globes())
	logVegetation(t, "valleys 1-2", valleys(2))
	either, savanna := historyOfTheBand(globes()[0])
	t.Logf("globe: %.3f of the tropics' land at %.0f-%.0f mm holds either a savanna or a forest, by where it starts from; %.3f of that is savanna as laid", either, staverLo, staverHi, savanna)
}

// historyOfTheBand is the share of g's tropical land in Staver's band of
// rain whose year holds either a savanna or a forest - a forest when run
// from a closed canopy, and a savanna, its trees under half the ground, when
// run from open ground - and the share of that land laid as a savanna: the
// land whose state is its history's.
func historyOfTheBand(g *Grid) (either, savanna float64) {
	twiMean := g.landTwiMean()
	throw := g.winds.Env.Throw()
	var band, both, open float64
	for y := 0; y < g.H; y++ {
		sea, land := g.snowSwings(y)
		sun := g.rowSun(y)
		area := g.air.Dx[y] * g.air.Dy
		for i := y * g.W; i < (y+1)*g.W; i++ {
			if g.sunk(i) || g.Tiles[i].Wet() || g.Tiles[i].Terrain.Tidal() {
				continue
			}
			mean, _, _ := g.YearAt(i)
			if rain := g.Rain(i); mean < tropicalMean || rain < staverLo || rain >= staverHi {
				continue
			}
			c, ok := g.vegClimate(i, sea, land, &sun, twiMean, throw, 1)
			if !ok {
				continue
			}
			band += area
			yr := veg.Read(&c)
			pot, fire := potentials(&yr)
			closed := veg.Equilibrium(&pot)
			bare := closed
			for p := range PFTs {
				if veg.Kinds[p].Tree {
					bare.Cover[p], bare.Mass[p] = 0, 0
				}
			}
			veg.Spin(&closed, &pot, &fire, vegSpin)
			veg.Spin(&bare, &pot, &fire, vegSpin)
			if closed.Trees() >= closedCanopy && bare.Trees() < 0.5 {
				both += area
				if g.TreeCover(i) < 0.5 {
					open += area
				}
			}
		}
	}
	if band > 0 {
		either = both / band
	}
	if both > 0 {
		savanna = open / both
	}
	return either, savanna
}

// A wood dies back where its ground stops suiting it. A small globe's
// climate is dried to a third of its rain and run on from the vegetation it
// has for the decades an age of erosion is, a few times over: the trees die
// back where the dry ground no longer holds them, and the woods that stood
// on that ground are woods no longer.
func TestWoodsDieBackWhereTheGroundStopsSuitingThem(t *testing.T) {
	if testing.Short() {
		t.Skip("needs a small globe")
	}
	g := smallGlobes(1)[0].Clone()
	if !g.climateWoods {
		t.Fatal("a globe's woods are not the climate's")
	}
	trees := func() (c float64, woods int) {
		for i := range g.Tiles {
			if !g.Tiles[i].Wet() && !g.sunk(i) {
				c += g.TreeCover(i)
			}
			if g.Tiles[i].Terrain == Forest {
				woods++
			}
		}
		return c, woods
	}
	before, woods := trees()
	for i := range g.rainIn {
		g.rainIn[i] /= 3
	}
	died := 0
	for range 5 {
		g.growVegetation(int(ageYears / yr))
		died += g.dieBackWoods()
	}
	after, left := trees()
	t.Logf("tree cover %.0f tiles' worth before, %.0f after fifty dry years; %d woods, %d died back, %d left", before, after, woods, died, left)
	if after > 0.6*before {
		t.Errorf("the trees covered %.0f tiles' worth and still cover %.0f on a third of the rain", before, after)
	}
	if died == 0 || left != woods-died {
		t.Errorf("%d of %d woods died back, %d left", died, woods, left)
	}
}
