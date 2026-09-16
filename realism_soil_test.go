package terra

import (
	"fmt"
	"math"
	"slices"
)

// The soil held against the earth's.
//
// These are the yardsticks of the soil, kept apart from realism_test.go's
// so that the two can be worked on at once: they join realYardsticks when the
// package is loaded and are held the same way. They read what a tile carries
// today - how deep its soil is, its sand and clay, and its organic carbon - on
// the worlds the other yardsticks have already made - and the order of soil
// those make it, as SoilOrderOf keys it.
func init() {
	realYardsticks = append(realYardsticks, soilYardsticks...)
}

var soilYardsticks = []realYardstick{
	// 9. How deep the soil is. On the uplands a hillslope holds a metre or so
	// over its rock, made as fast as it goes; the valley floors under them
	// hold what came off them, several to tens of metres of it.
	{yardstick: yardstick{
		name: "mean hillslope soil depth, valley", unit: "m", scale: "ground", lo: 0.2, hi: 1.5,
		source:  "Pelletier et al. 2016 (gridded soil and sedimentary deposit thickness): upland hillslope soils mostly under 2 m, typically about 1 m",
		measure: func() float64 { return soilDepths(valleys(5)).hillslope },
	}},
	{yardstick: yardstick{
		name: "mean hillslope soil depth, small globe", unit: "m", scale: "ground", lo: 0.2, hi: 1.5, slow: true,
		source:  "Pelletier et al. 2016 (gridded soil and sedimentary deposit thickness): upland hillslope soils mostly under 2 m, typically about 1 m",
		measure: func() float64 { return soilDepths(smallGlobes(networkGlobes)).hillslope },
	}},
	{yardstick: yardstick{
		name: "mean valley floor soil depth, valley", unit: "m", scale: "ground", lo: 3, hi: 50,
		source:  "Pelletier et al. 2016: lowland valley bottoms hold several to tens of metres of soil and sediment",
		measure: func() float64 { return soilDepths(valleys(5)).floor },
	},
		gap: "known gap: I - a made map's soil is laid at its steady depth, which production holds under ~2 m (soilDeepest), and what its water laid on the floors is height, not soil: 1.9 m",
	},
	{yardstick: yardstick{
		name: "mean valley floor soil depth, small globe", unit: "m", scale: "ground", lo: 3, hi: 50, slow: true,
		source:  "Pelletier et al. 2016: lowland valley bottoms hold several to tens of metres of soil and sediment",
		measure: func() float64 { return soilDepths(smallGlobes(networkGlobes)).floor },
	},
		gap: "known gap: I - a made map's soil is laid at its steady depth, which production holds under ~2 m (soilDeepest), and what its water laid on the floors is height, not soil: 0.70 m",
	},
	{yardstick: yardstick{
		name: "valley floor over hillslope soil depth, valley", unit: "x", scale: "ground", lo: 3, hi: 50,
		source:  "Pelletier et al. 2016: lowland valley bottoms hold several to tens of metres of soil and sediment against ~1 m on the hillslopes above",
		measure: func() float64 { d := soilDepths(valleys(5)); return d.floor / d.hillslope },
	}},
	{yardstick: yardstick{
		name: "valley floor over hillslope soil depth, small globe", unit: "x", scale: "ground", lo: 3, hi: 50, slow: true,
		source:  "Pelletier et al. 2016: lowland valley bottoms hold several to tens of metres of soil and sediment against ~1 m on the hillslopes above",
		measure: func() float64 { d := soilDepths(smallGlobes(networkGlobes)); return d.floor / d.hillslope },
	},
		gap: "known gap: I - the floors' soil is capped with the hillslopes' by production (see the floor depth yardstick), so they stand only a few times deeper: 3.015x on main, 2.11x once the land's winters softened",
	},

	// 10. Clay against the climate. On the same rock a soil formed warm holds
	// more clay than one formed cold, and a wet one more than a dry one: the
	// feldspars weather to clay minerals faster the warmer and the more water
	// goes through them.
	{yardstick: yardstick{
		name: "soil clay, warmest third over coldest third on like rock, globe", unit: "x", scale: "water", lo: 1.3, hi: 3, slow: true,
		source:  "Birkeland 1999 ch. 11: on like parent material clay rises with mean annual temperature (Jenny 1935 Great Plains transects); West 2012: silicate weathering Arrhenius in T",
		measure: func() float64 { return clayByClimate(globes(), false) },
	}},
	{yardstick: yardstick{
		name: "soil clay, wettest third over driest third on like rock, globe", unit: "x", scale: "water", lo: 1.3, hi: 3, slow: true,
		source:  "Birkeland 1999 ch. 11: on like parent material clay rises with precipitation (Jenny 1935 Missouri transect); West 2012: weathering saturating in runoff",
		measure: func() float64 { return clayByClimate(globes(), true) },
	}},

	// 11. Organic carbon by biome. A tile's Carbon is its soil's organic carbon
	// as a stock, in kilograms a square metre - see pedogenesis.go - and so is
	// read against the earth's top metre directly. What open grass comes to
	// under the map's middling climate is set at the earth's grassland figure
	// (carbonMiddle), so the mean over the land says as much about the spread
	// of climates and covers as about that one number. The biomes are read
	// where there is soil, as Jobbágy and Jackson's pits were dug; the mean
	// over the land counts bare ground at nothing.
	{yardstick: yardstick{
		name: "soil organic carbon, forest over desert (PET/P > 5), globe", unit: "x", scale: "water", lo: 1.5, hi: 3.2, slow: true,
		source:  "Jobbágy & Jackson 2000 Table 3: top-metre SOC 9.3-18.6 kg C/m2 under forests (boreal to tropical evergreen) against 6.2 in deserts",
		measure: func() float64 { return carbonByBiome(globes()).forestOverDesert },
	}},
	{yardstick: yardstick{
		name: "soil organic carbon, tundra (-15 to -3 C) over desert (PET/P > 5), globe", unit: "x", scale: "water", lo: 1.5, hi: 3.2, slow: true,
		source:  "Jobbágy & Jackson 2000 Table 3: top-metre SOC 14.2 kg C/m2 in tundra against 6.2 in deserts; cold wet soils keep what grows",
		measure: func() float64 { return carbonByBiome(globes()).tundraOverDesert },
	}},
	{yardstick: yardstick{
		name: "mean soil organic carbon, top metre, land, globe", unit: "kg C/m2", scale: "water", lo: 9, hi: 13, slow: true,
		source:  "Jobbágy & Jackson 2000: 1502 Pg C in the top metre over the ice-free land, ~11 kg C/m2",
		measure: func() float64 { return carbonByBiome(globes()).land },
	}},

	// 12. The soil orders. The earth's ice-free land by the order of soil on
	// it, as SoilOrderOf keys a tile from what it carries - its exposure age,
	// leaching, weatherable minerals, carbonate, salt and carbon, and the
	// climate over it. What the keys ask of a pit, a clay B horizon or the
	// iron and aluminium oxides, is read off what makes them, not carried.
	{yardstick: yardstick{
		name: "land share of Aridisols", unit: "", scale: "ground", lo: 0.09, hi: 0.15, slow: true,
		source:  "Soil Survey Staff 1999; USDA-NRCS global soil regions map: Aridisols ~12% of ice-free land",
		measure: func() float64 { return soilOrderShare(globes(), Aridisol) },
	}},
	{yardstick: yardstick{
		name: "land share of Gelisols", unit: "", scale: "ground", lo: 0.06, hi: 0.11, slow: true,
		source:  "Soil Survey Staff 1999; USDA-NRCS global soil regions map: Gelisols ~8.6% of ice-free land",
		measure: func() float64 { return soilOrderShare(globes(), Gelisol) },
	}},
	{yardstick: yardstick{
		name: "land share of Oxisols", unit: "", scale: "ground", lo: 0.05, hi: 0.10, slow: true,
		source:  "Soil Survey Staff 1999; USDA-NRCS global soil regions map: Oxisols ~7.5% of ice-free land",
		measure: func() float64 { return soilOrderShare(globes(), Oxisol) },
	},
		gap: "known gap: I - the hot humid land is young: its surfaces are a median of fourteen thousand years old, and few have weathered out three quarters of their minerals (SoilOrderOf): 0.024",
	},
	{yardstick: yardstick{
		name: "land share of Mollisols", unit: "", scale: "ground", lo: 0.05, hi: 0.09, slow: true,
		source:  "Soil Survey Staff 1999; USDA-NRCS global soil regions map: Mollisols ~6.9% of ice-free land",
		measure: func() float64 { return soilOrderShare(globes(), Mollisol) },
	}},
}

type soilDepthReading struct{ hillslope, floor float64 }

// soilDepths is the mean depth of soil on the dry land's hillslopes and on its
// valley floors. A hillslope is ground steeper than a tenth standing above the
// flood; a valley floor is ground gentler than a twentieth within the flood's
// reach of the water it drains into (FloodDepth), which is what the map itself
// calls a valley floor. Outcrops count, at nothing, as they would in a survey.
func soilDepths(gs []*Grid) soilDepthReading {
	return remember(fmt.Sprintf("soildepth/%p/%d", gs[0], len(gs)), func() soilDepthReading {
		var hs, hn, fs, fn float64
		for _, g := range gs {
			for i := range g.Tiles {
				t := &g.Tiles[i]
				if g.underSea(i) || t.Wet() || t.Terrain.Tidal() {
					continue
				}
				switch s := g.Slope(g.PosOf(i)); {
				case s > 0.1 && g.Drain[i] > FloodDepth:
					hs, hn = hs+float64(g.Soil[i]), hn+1
				case s < 0.05 && g.Drain[i] <= FloodDepth:
					fs, fn = fs+float64(g.Soil[i]), fn+1
				}
			}
		}
		r := soilDepthReading{math.NaN(), math.NaN()}
		if hn > 0 {
			r.hillslope = hs / hn
		}
		if fn > 0 {
			r.floor = fs / fn
		}
		return r
	})
}

// clayByClimate is how much more clay the soil holds formed at one end of the
// climate than at the other, on the same rock: for each bedrock, the dry
// land's tiles ranked by their year's mean temperature (or by how wet their
// ground is), the mean clay of the top third over that of the bottom third,
// and the ratios averaged by how many tiles each rock has. A rock with too few
// tiles to split is left out.
func clayByClimate(gs []*Grid, wet bool) float64 {
	return remember(fmt.Sprintf("clayclimate/%p/%d/%v", gs[0], len(gs), wet), func() float64 {
		type sample struct{ key, clay float64 }
		byRock := map[Bedrock][]sample{}
		for _, g := range gs {
			for i := range g.Tiles {
				t := &g.Tiles[i]
				if g.underSea(i) || t.Wet() || t.Terrain.Tidal() || t.Terrain == Rock || g.Soil[i] <= 0 {
					continue
				}
				k := g.meanTempOf(i)
				if wet {
					k = g.wetOf(i)
				}
				byRock[t.Bedrock] = append(byRock[t.Bedrock], sample{k, g.Clay[i]})
			}
		}
		sum, weight := 0.0, 0.0
		for _, ss := range byRock {
			if len(ss) < 300 {
				continue
			}
			slices.SortFunc(ss, func(a, b sample) int {
				switch {
				case a.key < b.key:
					return -1
				case a.key > b.key:
					return 1
				}
				return 0
			})
			third := len(ss) / 3
			lo, hi := 0.0, 0.0
			for k := 0; k < third; k++ {
				lo += ss[k].clay
				hi += ss[len(ss)-1-k].clay
			}
			if lo <= 0 {
				continue
			}
			sum += hi / lo * float64(len(ss))
			weight += float64(len(ss))
		}
		if weight == 0 {
			return math.NaN()
		}
		return sum / weight
	})
}

type carbonReading struct{ land, forestOverDesert, tundraOverDesert float64 }

// carbonByBiome reads the soil's organic carbon stock, Carbon, over the dry
// land: its mean over all of it, each tile weighted by the ground it stands
// for on a sphere, outcrops and bare ground counting at what they hold; and
// the mean over forest, over tundra - open dry land whose year's mean is -15
// to -3 C, the range the tundra's is - and over desert, open dry land whose
// aridity (PET over rain, as forestByAridity reads it) passes five, as ratios
// to the desert's.
func carbonByBiome(gs []*Grid) carbonReading {
	return remember(fmt.Sprintf("carbonbiome/%p/%d", gs[0], len(gs)), func() carbonReading {
		var ls, ln, fs, fn, ts, tn, ds, dn float64
		for _, g := range gs {
			if g.air == nil {
				continue
			}
			for i := range g.Tiles {
				t := &g.Tiles[i]
				if g.underSea(i) || t.Wet() || t.Terrain.Tidal() {
					continue
				}
				c := float64(t.Carbon)
				w := math.Cos(latitudeOf(g, i/g.W) * math.Pi / 180)
				ls, ln = ls+w*c, ln+w
				// The biomes are Jobbágy and Jackson's means over soil pits,
				// and nobody digs one in bare ground.
				if !forms(t) || g.Soil[i] <= 0 {
					continue
				}
				temp := g.meanTempOf(i)
				switch {
				case t.Terrain == Forest:
					fs, fn = fs+c, fn+1
				case temp >= -15 && temp <= -3:
					ts, tn = ts+c, tn+1
				case g.pet(i)/math.Max(1e-9, g.Rain(i)) > 5:
					ds, dn = ds+c, dn+1
				}
			}
		}
		r := carbonReading{math.NaN(), math.NaN(), math.NaN()}
		if ln > 0 {
			r.land = ls / ln
		}
		if dn == 0 || ds == 0 {
			return r
		}
		d := ds / dn
		if fn > 0 {
			r.forestOverDesert = fs / fn / d
		}
		if tn > 0 {
			r.tundraOverDesert = ts / tn / d
		}
		return r
	})
}

// soilOrderShare is the share of the worlds' dry land, by area, whose soil is
// of order o. Ground with no soil counts, as the rock and the shifting sand do
// on the USDA's map.
func soilOrderShare(gs []*Grid, o SoilOrder) float64 {
	shares := remember(fmt.Sprintf("soilorders/%p/%d", gs[0], len(gs)), func() [SoilOrderCount]float64 {
		var s [SoilOrderCount]float64
		var land float64
		for _, g := range gs {
			if g.air == nil {
				continue
			}
			for i := range g.Tiles {
				t := &g.Tiles[i]
				if g.underSea(i) || t.Wet() || t.Terrain.Tidal() {
					continue
				}
				w := math.Cos(latitudeOf(g, i/g.W) * math.Pi / 180)
				s[g.SoilOrderOf(i)] += w
				land += w
			}
		}
		for k := range s {
			s[k] /= land
		}
		return s
	})
	return shares[o]
}
