package terra

import (
	"testing"
)

// The vegetation held against the earth's.
//
// What grows on the land (vegetation.go, package veg) is read four ways: the
// biomes it makes against the climates Whittaker (1975) found each biome
// under; the carbon it holds against the earth's; its leaf area, biome by
// biome, against what the satellites read; and whether a wood dies back where
// its ground stops suiting it.
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
}

// landVegetation reads the vegetation off gs's dry land, by area.
func landVegetation(gs []*Grid) vegReading {
	return remember("landVegetation", gs, func() vegReading {
		var r vegReading
		var land, carbon, trees, desert, desertLAI, inside, read float64
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
				if b == HotDesert || b == ColdDesert {
					desert += area
					desertLAI += area * lai
				}
				if b == IceBiome || b == NoBiome {
					continue
				}
				mean, _, _ := g.YearAt(i)
				rain := g.Rain(i) / 10
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
	for b := Biome(1); b < Biomes; b++ {
		if r.share[b] == 0 {
			continue
		}
		t.Logf("%s:   %-27s %.3f of the land, LAI %.2f, %.2f in its Whittaker climate", what, b, r.share[b], r.lai[b], r.inEnvelope[b])
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
