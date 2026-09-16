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
// is the bed of the region's pile the ground has worn down to - see
// strata.go - and what the soil on top is
// made of, which starts as what that rock weathers to and is then moved
// about by every age of weather. Everything else here is read off those two
// rather than stored, in the way the rivers and the going underfoot are read
// off the height.

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

// tensileRef is the tensile strength at which a rock wears at Erodibility. It is
// set so that the four rocks a drawn map lays, in the equal shares it lays
// them in, wear on average at bedShare of it: the mean of (tensileRef/σ)²
// over granite, limestone, sandstone and shale is bedShare.
const tensileRef = 3.16 * megapascal * bedShareRoot

// bedShare is how fast the water cuts a middling rock against the soil over
// it: K_b over K for a drawn map's rocks on average. Read as the law on drainage
// area, K for soil at the valley's runoff is 1.3e-4 a year, and this puts the
// rock at 1.4e-5, inside the 1e-7 to 1e-4 real channels in rock are fitted at
// (Stock and Montgomery 1999; Harel, Mudd and Attal 2016).
//
// Where in that range is set by what a history makes of it, because a history
// is millions of years of the rock being cut and the drawn maps' decades are
// not. At the soil's own K a small globe's ranges came out a quarter of the
// height they had; at 0.06, what open grass held when it was what the rock was
// cut at, their valleys came out 320 metres apart. Over the eight small globes
// the drainage yardsticks read, by the share:
//
//	share   area exc.   Hack    hypsometry   valley spacing, m   mean slope
//	0.030    0.404      0.553     0.345           133              0.463
//	0.045    0.431      0.591     0.345           133              0.498
//	0.060    0.453      0.527     0.349           320              0.577
//	0.080    0.440      0.561     0.377           133              0.498
//	0.100    0.431      0.577     0.379           146              0.492
//	0.120    0.426      0.605     0.396           133              0.486
//
// Read against the whole of the suite, 0.11 was the one of those near it that
// also brought the discharge exponent inside, and the meandering reaches of the
// small globes to the wavelength and sinuosity Leopold and Wolman give.
const (
	bedShare     = 0.11
	bedShareRoot = 0.33166247903554 // √bedShare
)

// rockErodibility is how hard the water cuts the rock under a tile once its
// soil is gone, against Erodibility: K_b over K, (σ_ref/σ_T)², the inverse
// square of the rock's tensile strength (Sklar and Dietrich 2001), which is
// bedShare for the middling rock of a drawn map. Shale goes
// twelve times as fast as granite under the same water, which is the
// difference between a vale and the scarp above it.
//
// It is the rock's figure and nothing else: what grows on the ground holds the
// soil and not the rock under it, and holds it as a stress the water has to
// clear rather than as a share of what it takes - see criticalFall.
func rockErodibility(t *Tile) float64 {
	s := tensileRef / t.Bedrock.Tensile()
	return s * s
}

// siltAt is the share of tile i's soil that is neither sand nor clay, and
// TileView.Silt the same for a reader that asks by tile. It is not
// stored, because three shares of one thing are two numbers and a
// subtraction, and storing the third is storing a chance to disagree.
func (g *Grid) siltAt(i int) float64 { return siltOf(g.Sand[i], g.Clay[i]) }

// siltOf is the silt share of a soil with the given sand and clay.
func siltOf(sand, clay float64) float64 { return clamp01(1 - sand - clay) }

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

// loamAt is how near tile i's soil is to the mixture that grows most, in
// [0,1]; TileView.Loam is the same for a reader that asks by tile. It
// is what the fertility of a tile is multiplied by: the same damp, gentle,
// sunny ground is a good field over a loam and a poor one over sand that will
// not hold what it is given or clay that will not let go of it.
func (g *Grid) loamAt(i int) float64 { return loamOf(g.Sand[i], g.Clay[i]) }

// loamOf is Loam for a soil with the given sand and clay.
func loamOf(sand, clay float64) float64 {
	d := math.Hypot(sand-bestSand, clay-bestClay) / loamSpan
	return clamp01(1 - d)
}

// washAt is how fast tile i's soil moves for what it is made of; TileView.Wash
// is the same for a reader that asks by tile. Sand is loose
// grains and goes; clay sticks to itself and stays. It multiplies what an age
// of weather strips off a tile, alongside what is growing on it - see hold in
// erode.go - and it is the term that lets a settlement's own choice of where
// to plough decide how fast its hillsides come down.
//
// Balanced ground reads 1, so that a map whose soils average out to as much
// sand as clay weathers at the rate the whole thing was measured at. Pure
// sand reads 1.6 and pure clay 0.4: the reading is a multiplier and is not
// held to a share, only to being positive.
func (g *Grid) washAt(i int) float64 { return washOf(g.Sand[i], g.Clay[i]) }

// washOf is Wash for a soil with the given sand and clay.
func washOf(sand, clay float64) float64 { return math.Max(0, 1+0.6*(sand-clay)) }

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
//
// That crossing is the floor of the country. Over it lies a pile of beds -
// see strata.go and coverBeds - hard and soft by turns, tipped the way the
// whole country leans and warped into broad swells, so that what the ground
// is made of depends on how high it stands as well as where: the lowlands
// cut down to the floor, and the high ground is the pile, capped by whichever
// hard bed it has not yet worn through.
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
	// The older, harder half of the country stands up through the pile: it
	// is where the floor was raised, and the beds over it were the first to
	// go. See coverShield.
	shield := make([]float64, len(g.Tiles))
	for i := range shield {
		shield[i] = coverShield * smooth(clamp01((hard[i]-hardLine)/coverShieldEdge))
	}
	w.coverBeds(g, shield)
}

// The pile a drawn map is given. coverBeds is the rocks of it from the
// bottom up: soft beds between hard ones, which is the only arrangement that
// makes a scarp. Each is coverThin to coverThick metres, and the foot of the
// pile lies at coverFoot of the way up the map's ground, so that about the
// lowest third of the country is worn through to the floor.
//
// coverDip is how steeply the whole pile leans, as rise over run, at most;
// coverSwell how many metres its broad warps lift and drop it, and
// coverSwellSpan how many tiles across they are. coverShield is how many
// metres higher the pile's foot lies over the older, harder half of the
// floor, and coverShieldEdge how far into that half, as a reading of its
// lattice, it takes to get there.
var coverRocks = []Bedrock{Shale, Sandstone, Shale, Limestone, Shale, Sandstone}

const (
	coverThin       = 14.0
	coverThick      = 40.0
	coverFoot       = 0.4
	coverDip        = 0.03
	coverSwell      = 30.0
	coverSwellSpan  = 24.0
	coverShield     = 40.0
	coverShieldEdge = 0.15
)

// coverBeds lays the pile over the floor layBedrock has drawn. It draws
// nothing from the world's chance - its luck is the seed's own, hashed - so
// the world that comes out of a seed is the one it always was everywhere the
// pile does not reach.
func (w *Land) coverBeds(g *Grid, shield []float64) {
	luck := func(k uint64) float64 { return unit(splitmix(w.seed ^ 0x62656473 ^ k*0x9E3779B97F4A7C15)) }
	heights := g.heights()
	foot := quantile(heights, coverFoot)
	lean := 2 * math.Pi * luck(1)
	dip := coverDip * (0.2 + 0.8*luck(2))
	thick := make([]float64, len(coverRocks))
	for k := range thick {
		thick[k] = coverThin + (coverThick-coverThin)*luck(uint64(10+k))
	}
	g.strata = make([]column, len(g.Tiles))
	g.EachRow(func(y int) {
		for x := 0; x < g.W; x++ {
			i := y*g.W + x
			c := basement(g.Tiles[i].Bedrock, 0, heights[i])
			// Where the foot of the pile is under this tile: the lean of the
			// country, measured from its middle, and the swells over it.
			along := (float64(x-g.W/2)*math.Cos(lean) + float64(y-g.H/2)*math.Sin(lean)) * TileSpan
			at := foot + dip*along + coverSwell*(2*g.swellAt(w.seed, x, y)-1) + shield[i]
			for k, rock := range coverRocks {
				c.lay(rock, 0, 0, at, at+thick[k])
				at += thick[k]
			}
			// The top bed goes on up for ever: whatever stands higher than
			// the pile was drawn is the pile's top rock.
			c.top[0] = float32(math.Max(float64(c.top[0]), heights[i]))
			g.strata[i] = c
		}
	})
	g.expose()
}

// swellAt is the broad warp of a drawn map's pile at x, y, in [0,1]: a smooth
// blend of hashed corners coverSwellSpan tiles apart, going round a globe.
func (g *Grid) swellAt(seed uint64, x, y int) float64 {
	cols := int(math.Ceil(float64(g.W)/coverSwellSpan)) + 1
	fx, fy := float64(x)/coverSwellSpan, float64(y)/coverSwellSpan
	cx, cy := int(fx), int(fy)
	tx, ty := smooth(fx-float64(cx)), smooth(fy-float64(cy))
	corner := func(dx, dy int) float64 {
		c := cx + dx
		if g.Wrap {
			c %= max(1, cols-1)
		}
		return unit(splitmix(seed ^ 0x7377656c ^ uint64(c)*0x9E3779B97F4A7C15 ^ uint64(cy+dy)*0xC2B2AE3D27D4EB4F))
	}
	top := corner(0, 0) + tx*(corner(1, 0)-corner(0, 0))
	bottom := corner(0, 1) + tx*(corner(1, 1)-corner(0, 1))
	return top + ty*(bottom-top)
}

// soilTexture sets every tile's soil to what its ground weathers to. It runs
// when the map is made, once the heights and the drainage are settled,
// because what the water has laid down is part of the answer.
func (g *Grid) soilTexture() {
	for i := range g.Tiles {
		p := geom.Pos{X: i % g.W, Y: i / g.W}
		g.Sand[i], g.Clay[i] = g.TextureAt(p)
	}
}
