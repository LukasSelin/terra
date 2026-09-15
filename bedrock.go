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

// tensile is each rock's tensile strength, in megapascals. It is what makes a
// landscape have a shape at all: where the rocks differ, the soft ones go and
// the hard ones are left standing, so a scarp is a hard bed with a soft one
// under it and a gorge is a river that found something it could cut.
//
// Tensile strength and not a ranking, because it is the one figure a river's
// wear has been measured against. Sklar and Dietrich (2001) wore discs of
// twenty-odd rocks under saltating gravel in a flume and found the wear going
// as the inverse square of the rock's tensile strength, over more than two
// orders of magnitude of it - from mudstones under a megapascal to quartzites
// over ten. The figures here are middling values of the lithologies in their
// range and the rock-mechanics tables it was drawn from: basalt and granite
// the strongest, schist split along its foliation, limestone and sandstone
// cemented well or badly, and shale barely rock at all.
//
// It was a hardness a quarryman would have ranked, 1.5 for granite down to
// 0.45 for shale.
var tensile = [BedrockCount]float64{
	Basalt:    10 * megapascal,
	Granite:   7 * megapascal,
	Schist:    5 * megapascal,
	Limestone: 4.5 * megapascal,
	Sandstone: 3.5 * megapascal,
	Shale:     2 * megapascal,
}

// Chemistry is what a rock gives the soil over it to live on, as shares of
// the rock's mass. It is read and not yet used: a soil made of a rock is still
// only its sand and its clay - see weathers - and this is what the soil will
// ask of it when it is also sour or sweet, rich or starved.
//
// Quartz is the mineral share that does not weather at all: what a soil keeps
// as sand for ever, and what makes the soil on a rock poor however long it
// has. Bases is the calcium, magnesium, potassium and sodium, counted as their
// oxides the way a rock is analysed - CaO, MgO, K2O and Na2O - which is what
// the weather frees to hold a soil's acidity back and to feed what grows in
// it. Carbonate is the share that dissolves outright rather than weathering to
// clay, and Phosphorus the element itself, not its oxide: the one nutrient a
// soil cannot take from the air and has only from its rock.
type Chemistry struct {
	Quartz, Bases, Carbonate, Phosphorus float64
}

// chemistry is each rock's, from averages of many analyses rather than any one
// rock. The oxides of granite and basalt are Le Maitre's (1976) averages of
// the analyses of each he gathered: 1.84 per cent CaO, 0.71 MgO, 3.68 Na2O,
// 4.07 K2O and 0.12 P2O5 in granite, and 9.47, 6.73, 2.91, 1.10 and 0.35 in
// basalt. Those of limestone, sandstone and shale are
// Clarke's (1924) averages as Pettijohn (Sedimentary Rocks, 1975) tabulates
// them, with their carbonate read off the CO2 - 41.5, 5.0 and 2.6 per cent -
// as calcite, and the magnesia of the limestone as dolomite: limestone 42.6
// CaO, 7.9 MgO, 0.33 K2O, 0.05 Na2O and 0.04 P2O5; sandstone 5.5, 1.2, 1.3,
// 0.45 and 0.08, most of its lime the cement between the grains; shale 3.1,
// 2.4, 3.2, 1.3 and 0.17. Phosphorus is 0.436 of the P2O5.
//
// The quartz is a mineral share, which an analysis does not give. Granite's is
// the third it typically has, inside the fifth to three fifths that the rock's
// definition allows it (Streckeisen 1976); basalt has none; shale's is Shaw
// and Weaver's (1965) average of 31 per cent, with 3.6 per cent carbonate,
// which is taken over Clarke's for the minerals; sandstone's the two thirds of
// an average sandstone, and limestone's the few per cent of its silica that is
// quartz and chert.
//
// Schist is shale cooked. A pelitic schist has the analysis of the mudrock it
// was, less its water and carbon dioxide (Shaw 1956), so its row is shale's
// with the carbonate driven off - the lime stays, in the silicates it has made
// - and a little more quartz for the granite and sandstone the crushing takes
// in with the mud.
var chemistry = [BedrockCount]Chemistry{
	Granite:   {Quartz: 0.30, Bases: 0.103, Carbonate: 0, Phosphorus: 0.00052},
	Limestone: {Quartz: 0.04, Bases: 0.509, Carbonate: 0.91, Phosphorus: 0.00017},
	Sandstone: {Quartz: 0.65, Bases: 0.085, Carbonate: 0.11, Phosphorus: 0.00035},
	Shale:     {Quartz: 0.31, Bases: 0.100, Carbonate: 0.036, Phosphorus: 0.00074},
	Basalt:    {Quartz: 0, Bases: 0.202, Carbonate: 0, Phosphorus: 0.00153},
	Schist:    {Quartz: 0.35, Bases: 0.095, Carbonate: 0.005, Phosphorus: 0.00070},
}

// Chemistry is what this rock gives a soil: see Chemistry.
func (b Bedrock) Chemistry() Chemistry {
	if b >= BedrockCount {
		return Chemistry{}
	}
	return chemistry[b]
}

// tensileHard is the tensile strength a rock of hardness one has: what
// hardness is read against. At five megapascals a drawn map's four rocks come
// out at 1.4, 0.9, 0.7 and 0.4, near the 1.5, 0.65, 0.8 and 0.45 they were
// ranked at, so the slopes the rock holds up - see stand - and the beds a
// history lays keep the contrasts they had.
const tensileHard = 5 * megapascal

// hardness is each rock's tensile strength against tensileHard. Everything
// that asks how hard the rock is asks it against the map's middling rock - see
// meanHard - so it is the contrasts that count and not the scale.
var hardness = func() (h [BedrockCount]float64) {
	for b := range h {
		h[b] = tensile[b] / tensileHard
	}
	return h
}()

// Hard is how well the rock under this tile stands up to being worn away. It
// holds up how steep the ground can stand and steepens the fall a channel is
// shaped to, and it is what the water pays to cut the rock: see
// rockErodibility.
func (t *Tile) Hard() float64 { return hardness[t.Bedrock] }

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
	s := tensileRef / tensile[t.Bedrock]
	return s * s
}

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
		g.Tiles[i].Sand, g.Tiles[i].Clay = g.TextureAt(p)
	}
}
