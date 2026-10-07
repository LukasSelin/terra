package terra

import "github.com/LukasSelin/terra/internal/veg"

// The biome of a tile, read off what grows on it.
//
// A biome was a name for a Köppen code: what the climate would let grow,
// given the name of what grows there on the earth under that climate. Now it
// is what does grow, the way BIOME4 (Kaplan and others, 2003) names its
// output: by how much of the ground the trees cover, which trees they are,
// and what holds the open ground - a closed canopy is a forest, a canopy
// over a third of the ground or so a woodland or a savanna, and ground with
// next to no leaf on it a desert. The Köppen type is still the climate's own
// reading: see Koppen.

// Biome is a kind of vegetation a tile carries.
type Biome uint8

const (
	// NoBiome is water, and ground whose vegetation has not been laid.
	NoBiome Biome = iota
	IceBiome
	TundraBiome
	BorealForest
	TemperateConiferForest
	TemperateBroadleafForest
	TemperateWoodland
	TemperateGrassland
	Shrubland
	ColdDesert
	HotDesert
	TropicalRainforest
	TropicalSeasonalForest
	Savanna
	Biomes
)

var biomeNames = [Biomes]string{
	NoBiome:                  "none",
	IceBiome:                 "ice",
	TundraBiome:              "tundra",
	BorealForest:             "boreal forest",
	TemperateConiferForest:   "temperate conifer forest",
	TemperateBroadleafForest: "temperate broadleaf forest",
	TemperateWoodland:        "temperate woodland",
	TemperateGrassland:       "temperate grassland",
	Shrubland:                "shrubland",
	ColdDesert:               "cold desert",
	HotDesert:                "hot desert",
	TropicalRainforest:       "tropical rainforest",
	TropicalSeasonalForest:   "tropical seasonal forest",
	Savanna:                  "savanna",
}

func (b Biome) String() string {
	if b >= Biomes {
		return "?"
	}
	return biomeNames[b]
}

// The lines a biome is read at. A forest's trees cover closedCanopy of the
// ground or more, which is the 60 per cent MODIS's land cover draws its
// forests at (Friedl and others, 2010); a woodland's or a savanna's
// openCanopy, its savannas' and woody savannas' 10 to 30 per cent widened to
// what a tile's trees read at the edge of a forest; and ground whose leaf
// area is under desertLeaf is a desert, the leaf area MODIS reads over
// barren and sparse ground (Myneni and others, 2002). A desert is hot where
// the year's mean is hotDesert or more, Köppen's line between BWh and BWk.
// A forest of the boreal needleleaf trees whose year's mean is coolConifer
// or more is not a boreal forest but BIOME4's cool conifer forest, the
// mixed and conifer woods of a temperate country's cold winters; it is
// named with the temperate conifers. Whittaker's boreal forest stands
// under a mean of five degrees.
const (
	closedCanopy = 0.6
	openCanopy   = 0.25
	desertLeaf   = 0.3
	hotDesert    = 18.0
	coolConifer  = 5.0
)

// BiomeAt is the biome of tile i: what grows on it, named. It is NoBiome on
// the water and where nothing has been laid.
func (g *Grid) BiomeAt(i int) Biome {
	if !g.vegLaid() || i < 0 || i >= len(g.Tiles) || g.Tiles[i].Wet() || g.sunk(i) {
		return NoBiome
	}
	if g.Barren(g.PosOf(i)) {
		return IceBiome
	}
	var trees, lai float64
	var tree, open PFT
	var treeMass, openCover float64
	for p := range PFTs {
		c := g.Cover(i, p)
		lai += c * g.LeafArea(i, p)
		if veg.Kinds[p].Tree {
			trees += c
			if m := g.Biomass(i, p); m > treeMass {
				tree, treeMass = p, m
			}
		} else if c > openCover {
			open, openCover = p, c
		}
	}
	mean, _, _ := g.YearAt(i)
	tropical := tree == TropicalEvergreen || tree == TropicalRaingreen
	switch {
	case trees >= closedCanopy:
		switch tree {
		case TropicalEvergreen:
			return TropicalRainforest
		case TropicalRaingreen:
			return TropicalSeasonalForest
		case TemperateBroadleaf:
			return TemperateBroadleafForest
		case TemperateNeedleleaf:
			return TemperateConiferForest
		}
		if mean >= coolConifer {
			return TemperateConiferForest
		}
		return BorealForest
	case trees >= openCanopy:
		switch {
		case tropical:
			return Savanna
		case tree == BorealNeedleleaf:
			return BorealForest
		}
		return TemperateWoodland
	case lai < desertLeaf:
		if open == Tundra && openCover > 0 {
			return TundraBiome
		}
		if mean >= hotDesert {
			return HotDesert
		}
		return ColdDesert
	}
	switch open {
	case Tundra:
		return TundraBiome
	case Shrub:
		return Shrubland
	case C4Grass:
		return Savanna
	}
	return TemperateGrassland
}
