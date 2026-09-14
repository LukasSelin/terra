package terra

import (
	"github.com/LukasSelin/terra/geom"
	"math"
)

// What the ground is made of.
//
// The map had one soil. A tile was fertile or it was not, and the number said
// how much a field there would give and nothing else - so two tiles of the
// same fertility were the same ground in every way that mattered, whatever
// the country they were in. Real ground is not like that. Soil is what is
// left of the rock underneath it after the weather has had a few thousand
// years, sorted by the water that carried it, and what a piece of it is made
// of decides how it drains, what it grows, how fast it washes away and how
// hard it is to work.
//
// So there are two new things on a tile: what the rock beneath it is, which
// is a fact about the region and never changes, and what the soil on top is
// made of, which starts as what that rock weathers to and is then moved
// about by every age of weather. Everything else here is read off those two
// rather than stored, in the way the rivers and the going underfoot are read
// off the height.

// Bedrock is the rock a tile's soil is weathering out of. Four kinds is not
// geology; it is the fewest that give a map somewhere sandy, somewhere heavy,
// somewhere sweet and somewhere sour, which is what the ground needs to stop
// being uniform.
type Bedrock uint8

const (
	// Granite is the hard, sour rock of the high country. It weathers to a
	// coarse gritty soil - grus, what is left when the crystals let go of
	// each other - which drains freely and is nobody's best field.
	Granite Bedrock = iota
	// Limestone dissolves rather than crumbles, so what it leaves behind is
	// the little that would not dissolve: a fine, sticky residue over rock
	// that the water runs away through.
	Limestone
	// Sandstone is sand that was buried and is now sand again.
	Sandstone
	// Shale is mud that was buried and is now mud again: the heaviest ground
	// on the map, and the ground a plough dreads in a wet spring.
	Shale
	// Basalt is what comes up: the floor a world cools into, what fills a
	// rift, and what a volcano leaves. It weathers to a dark soil that holds
	// water and what is dissolved in it, which is why people farm the flanks
	// of volcanoes knowing exactly what they are.
	Basalt
	// Schist is rock that was something else and was then buried, cooked and
	// squeezed by a collision. It is the rock of an old mountain range, and
	// finding it is finding where two plates met.
	Schist
	// BedrockCount is how many kinds there are, for the tables that have to
	// carry a row for each.
	BedrockCount
)

// weathers is what each rock leaves behind when the weather has finished with
// it: the shares of sand and clay in the soil over it, the silt being what is
// left of the one, where the weathering is middling - see weathered, which
// moves them with the climate. These are what the soil the rock makes is made
// of, wherever the water has stripped what was there and the rock underneath
// has to make it again.
//
// They are not measurements of any one soil. Each row is put inside the USDA
// texture class that the residual soil on that rock falls in under a temperate
// climate, as Buol and others (Soil Genesis and Classification) and
// Birkeland (Soils and Geomorphology, 1999) describe them: granite to grus and
// a gritty loam, sandstone to a sandy loam, shale to clay, limestone to the clayey residue that is left when the lime
// has gone - terra rossa at its reddest - basalt to a clay loam, and schist
// to a loam between the granite it is often made of and the shale it often
// was. Where in its class a row sits is a choice, and it was the table the map
// already had.
var weathers = [BedrockCount]struct{ sand, clay float64 }{
	Granite:   {0.45, 0.20},
	Limestone: {0.20, 0.35},
	Sandstone: {0.70, 0.08},
	Shale:     {0.10, 0.55},
	Basalt:    {0.30, 0.30},
	Schist:    {0.35, 0.28},
}

// hardness is how well each rock stands up to weather and water, against a
// middling rock at one. It is what makes a landscape have a shape at all:
// where the rocks differ, the soft ones go and the hard ones are left
// standing, so a scarp is a hard bed with a soft one under it and a gorge is
// a river that found something it could cut. Everything on the map used to
// wear at the same rate, whatever it was made of, and a country where
// everything wears evenly wears flat.
//
// The order is the order a quarryman would give: granite and basalt are what
// people build with, schist splits, sandstone and limestone are soft enough
// to cut and hard enough to stand, and shale is barely rock at all.
var hardness = [BedrockCount]float64{
	Granite:   1.5,
	Basalt:    1.4,
	Schist:    1.1,
	Sandstone: 0.8,
	Limestone: 0.65,
	Shale:     0.45,
}

// Hard is how well the rock under this tile stands up to being worn away. It
// divides what an age of weather takes off, so ground over shale comes down
// three times as fast as ground over granite and the difference between them
// is a hillside.
func (t *Tile) Hard() float64 { return hardness[t.Bedrock] }

// String is what a rock is called.
func (b Bedrock) String() string {
	switch b {
	case Granite:
		return "granite"
	case Limestone:
		return "limestone"
	case Sandstone:
		return "sandstone"
	case Shale:
		return "shale"
	case Basalt:
		return "basalt"
	case Schist:
		return "schist"
	}
	return "rock"
}

// Bedrocks is every kind of rock, for the tables that have to cover them all
// and the tests that check they do.
func Bedrocks() []Bedrock {
	out := make([]Bedrock, 0, BedrockCount)
	for b := Bedrock(0); b < BedrockCount; b++ {
		out = append(out, b)
	}
	return out
}

// Silt is the share of the soil that is neither sand nor clay. It is not
// stored, because three shares of one thing are two numbers and a
// subtraction, and storing the third is storing a chance to disagree.
func (t *Tile) Silt() float64 { return clamp01(1 - t.Sand - t.Clay) }

// The mixture that grows most: a sandy loam, enough sand to drain and work
// and enough clay to hold water and what is dissolved in it. loamSpan is how
// far from it a soil can be before it grows nothing extra at all, in the same
// units - a share of the soil - so that a reading of how good a mixture is
// falls off with distance from the best one rather than at a line.
const (
	bestSand = 0.40
	bestClay = 0.20
	loamSpan = 0.55
)

// Loam is how near this soil is to the mixture that grows most, in [0,1]. It
// is what the fertility of a tile is multiplied by: the same damp, gentle,
// sunny ground is a good field over a loam and a poor one over sand that will
// not hold what it is given or clay that will not let go of it.
func (t *Tile) Loam() float64 {
	d := math.Hypot(t.Sand-bestSand, t.Clay-bestClay) / loamSpan
	return clamp01(1 - d)
}

// Wash is how fast this soil moves for what it is made of. Sand is loose
// grains and goes; clay sticks to itself and stays. It multiplies what an age
// of weather strips off a tile, alongside what is growing on it - see hold in
// erode.go - and it is the term that lets a settlement's own choice of where
// to plough decide how fast its hillsides come down.
//
// Balanced ground reads 1, so that a map whose soils average out to as much
// sand as clay weathers at the rate the whole thing was measured at. Pure
// sand reads 1.6 and pure clay 0.4: the reading is a multiplier and is not
// held to a share, only to being positive.
func (t *Tile) Wash() float64 { return math.Max(0, 1+0.6*(t.Sand-t.Clay)) }

// TextureAt is what the soil the rock at p makes is made of: what the rock
// weathers to, turned further toward clay the warmer and wetter the ground
// is. See weathered.
//
// It used to be moved toward a fixed river silt on low ground as well, by how
// far the tile stood under FloodDepth, which laid a flood plain's texture on
// by a line rather than by a river. What a river lays down is now what the
// river carried, sorted as it settles and worn finer the further it went -
// see depositOf and Sternberg - and it is laid where the river lays it. So
// this is the rock's part only, which is what a fresh map's soils are set to
// and what the rock adds to a soil as it makes more of it.
func (g *Grid) TextureAt(p geom.Pos) (sand, clay float64) {
	i := g.Index(p)
	w := weathers[g.Tiles[i].Bedrock]
	return weathered(w.sand, w.clay, g.weathering(i))
}

// layBedrock writes down what rock is under every tile. Two lattices far
// coarser than anything in the relief, crossed: one says how much of the
// country is the older, harder rock and the other how much of it was laid
// down under water, and the four quarters of that crossing are the four
// rocks. Coarse, because geology is regions - a map speckled with four rocks
// a tile at a time would be four soils averaged everywhere and no soil
// anywhere.
//
// The lines are drawn at each lattice's own middle rather than at a fixed
// height, so that every map gets some of all four however its noise happened
// to fall. It is the same cut every other share on this map is made with;
// see forestShare.
func (w *Land) layBedrock(g *Grid) {
	hard := w.lattice(g, float64(g.Span())/2)
	sunk := w.lattice(g, float64(g.Span())/3)
	hardLine, sunkLine := quantile(hard, 0.5), quantile(sunk, 0.5)
	for i := range g.Tiles {
		switch {
		case hard[i] >= hardLine && sunk[i] < sunkLine:
			g.Tiles[i].Bedrock = Granite
		case hard[i] >= hardLine:
			g.Tiles[i].Bedrock = Limestone
		case sunk[i] < sunkLine:
			g.Tiles[i].Bedrock = Sandstone
		default:
			g.Tiles[i].Bedrock = Shale
		}
	}
}

// soilTexture sets every tile's soil to what its ground weathers to. It runs
// when the map is made, once the heights and the drainage are settled,
// because what the water has laid down is part of the answer.
func (g *Grid) soilTexture() {
	for i := range g.Tiles {
		p := geom.Pos{X: i % g.W, Y: i / g.W}
		g.Tiles[i].Sand, g.Tiles[i].Clay = g.TextureAt(p)
	}
}
