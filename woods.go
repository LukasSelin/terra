package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
	"github.com/LukasSelin/terra/internal/atmos"
)

// woodsShare is how much of a map's dry land will hold a wood. It is the
// answer to a settlement watching the forest close over it: with nothing to
// say where trees can stand, every tile a seed fell on became one, and after
// six thousand ticks the woods covered three quarters of the map and the
// fields had gone from forty strips to one, because a field can only be
// broken on open ground and there was none left.
//
// A tree line is what the land actually has. Woods stand on the damp gentle
// ground and stop where the ground turns dry, steep or cold, and that line
// is read off the map the same way the founding woods were placed: the
// wettest, gentlest fifth of the land will hold a wood, and nothing else
// will. The founding woods take a little over half of that, so a settlement
// still watches the forest creep back over what it cleared - just not over
// the ground it farms.
const woodsShare = 0.2

// woodsSteep is how much of a map is too steep to hold a wood at all. Slope
// enters the reading below as a discount - steep ground suits trees less than
// flat ground of the same dampness - and a discount is not a limit: a damp
// enough bank came out above the tree line however sharply it fell away, so
// woods crept up the sides of the gullies the water had just cut. A tree
// needs ground to stand its roots in, and ground that steep is on its way
// downhill. The steepest fifth of the map holds nothing, whatever else is
// true of it.
const woodsSteep = 0.2

// soilCritical is the steepest a soil-mantled hillside stands, as a rise over
// a run: Roering, Kirchner and Dietrich (1999) fit a critical gradient of 1.2
// to 1.35 to the hillslopes of the Oregon Coast Range, beyond which soil moves
// downhill as fast as it is made. It is the climate's woods' slope limit.
const soilCritical = 1.25

// luck is how much a founding wood's draw can add to how well its ground
// suits trees. See Land.Generate.
const luck = 0.35

// The woods the climate makes. A forest stands where the rain outruns what
// the air could take back up: Budyko's dryness index φ, the potential
// evaporation over the rain, is under one under every closed forest on Earth
// and over two under none (Budyko, 1974), and Holdridge's life zones put moist
// and wet forest under a ratio of one and dry forest between one and two
// (Holdridge, 1967). Under the ice and above the tree line nothing is a
// forest, and nor is anything under Holdridge's polar biotemperature.
//
// Within a climate the water is not spread evenly. It gathers where much
// ground drains through a place and the place is too flat to send it on, and
// the topographic wetness index ln(a/tan β) of Beven and Kirkby (1979) is that
// reading - a the catchment above a tile per metre of its width, β its slope.
// TOPMODEL reads a place's share of its catchment's water as its index against
// the catchment's mean, and so is this: the water a tile has is the rain over
// what the air could take, times its index over the land's mean. The wet
// hollows of a dry country hold a wood and its dry ridges do not, which is a
// gallery forest; in a wet one only the sharpest crests are too dry.
//
// So a tile suits trees by the water it has against the water a forest needs,
// and WoodsAt gives that as a half at a ratio of one, which is climateLine.
const climateLine = 0.5

// twiSlope is the least slope the wetness index is read at, as a rise over a
// run: ground flatter than a thousandth drains as though it fell a thousandth,
// and a perfectly flat tile would otherwise be infinitely wet.
const twiSlope = 1e-3

// twi is the topographic wetness index of tile i.
func (g *Grid) twi(i int) float64 {
	a := 1.0
	if i < len(g.area) {
		a = math.Max(1, g.area[i])
	}
	return math.Log(a * TileSpan / math.Max(twiSlope, g.Slope(g.PosOf(i))))
}

// waterRatio is the water tile i has against what a forest on it needs: the
// rain over what the air could take up, times how much of its catchment's
// water it gathers. Nothing over nothing is no water.
func (g *Grid) waterRatio(i int) float64 {
	if g.air == nil || i >= len(g.rain) || g.twiMean <= 0 {
		return 0
	}
	pet := g.pet(i)
	if pet <= 0 {
		return math.Inf(1)
	}
	return g.rain[i] / pet * g.twi(i) / g.twiMean
}

// WoodsAt is how well a tile's ground suits trees: damp enough to grow them
// and gentle enough to hold the soil they grow in. It is the reading the map
// was made with, and it moves when the weather moves the ground under it.
//
// Under the climate's rules - see Terms.Woods - it is the climate's reading
// instead, over the same limit of slope: half of the water tile has against
// what a forest needs, up to one, and nothing at all under the ice, above the
// tree line or under Holdridge's polar biotemperature. See climateLine.
func (g *Grid) WoodsAt(p geom.Pos) float64 {
	if !g.In(p) || g.TooSteep(p) {
		return 0
	}
	if g.climateWoods {
		i := g.Index(p)
		if g.Treeless(p) || g.Barren(p) {
			return 0
		}
		if len(g.warm) == len(g.Tiles) && atmos.Biotemperature(g.meanOn(i, g.Height[i]), float64(g.swing[i])) < atmos.HoldridgePolar {
			return 0
		}
		return clamp01(climateLine * g.waterRatio(i))
	}
	damp := clamp01(1 - g.Drain[g.Index(p)]/(2*FloodDepth))
	steep := clamp01(g.Slope(p) / max(1e-12, g.steepAt))
	return damp * (1 - 0.6*steep)
}

// TooSteep reports whether the ground falls away too fast for a wood to hold
// on it. It is a comparison with the rest of the map, like everything else
// read off the land: what counts as a bank on a map of hills is a cliff on a
// map of water meadows.
func (g *Grid) TooSteep(p geom.Pos) bool {
	if !g.In(p) {
		return true
	}
	if !g.woodsRead {
		g.readWoods()
	}
	// Under the climate's rules the line is not a share of the map but the
	// slope soil stops standing on. A share of every slope on a globe is a
	// share of a map most of which is flat sea, and the steepest fifth of that
	// was nine tenths of the humid land.
	if g.climateWoods {
		return g.Slope(p) > soilCritical
	}
	// Steeper than the line, not at it: on ground with no slope in it at all
	// the line is zero, and flat ground is the last thing that should read as
	// too steep to hold a tree.
	return g.Slope(p) > g.steepLine
}

// HoldsWood reports whether trees will take on this tile: whether its ground
// is above the tree line. What is already wooded is left alone - a standing
// wood is a fact about the map, however it got there - so this is asked of
// ground that is about to become a wood, by seeding or by planting, and not
// of ground that is one.
func (g *Grid) HoldsWood(p geom.Pos) bool {
	if !g.In(p) {
		return false
	}
	if !g.woodsRead {
		g.readWoods()
	}
	return g.holds[g.Index(p)]
}

// readWoods takes the map's measure of itself: how steep its steep ground is,
// and where the tree line falls on it. Both are comparisons with the rest of
// the map rather than fixed numbers, because a fixed cutoff gives one map a
// forest and the next a heath. It is run when the land is made and again
// whenever the weather has moved it.
func (g *Grid) readWoods() {
	// A slope is the eight heights round a tile, taken half a million times
	// over; it is the dearest reading a map takes of itself, and it reads
	// the ground and writes only its own answer. See Grid.EachRow.
	slopes := make([]float64, len(g.Tiles))
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			slopes[i] = g.laidSlope(geom.Pos{X: i % g.W, Y: i / g.W}) // see laidHeight
		}
	})
	q := quantiles(slopes, 0.9, 1-woodsSteep)
	g.steepAt, g.steepLine = q[0], q[1]
	g.woodsRead = true // the slope readings are in; WoodsAt may be asked now

	// How well each tile suits trees, then gathered into the list the tree
	// line is read off. The reading is spread and the gathering is not, so
	// the list is in tile order however the rows were worked. WoodsAt asks
	// TooSteep, which reads the map afresh if the slopes are not in yet -
	// they are, three lines above, or none of this would mean anything.
	if g.climateWoods {
		// The land's mean wetness index, which the climate's woods are read
		// against: see twi.
		twis := make([]float64, len(g.Tiles))
		g.EachRow(func(y int) {
			for i := y * g.W; i < (y+1)*g.W; i++ {
				twis[i] = g.twi(i)
			}
		})
		var sum, n float64
		for i := range g.Tiles {
			if !g.Tiles[i].Wet() && !g.Tiles[i].Terrain.Tidal() {
				sum, n = sum+twis[i], n+1
			}
		}
		if g.twiMean = 0; n > 0 {
			g.twiMean = sum / n
		}
	}
	suit := make([]float64, len(g.Tiles))
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			if g.Tiles[i].Wet() || g.Tiles[i].Terrain.Tidal() {
				continue // the river is not ground trees might have had, nor the tide's mud
			}
			suit[i] = g.WoodsAt(geom.Pos{X: i % g.W, Y: i / g.W})
		}
	})
	suits := make([]float64, 0, len(g.Tiles))
	for i := range g.Tiles {
		if !g.Tiles[i].Wet() && !g.Tiles[i].Terrain.Tidal() {
			suits = append(suits, suit[i])
		}
	}
	if len(suits) > 0 {
		g.woodsLine = quantile(suits, 1-woodsShare)
	}
	if g.climateWoods {
		g.woodsLine = climateLine
	}
	g.readHolds()
}

// readHolds writes down, for every tile, whether trees will take on it. The
// answer is a reading of the ground and of nothing else, so it moves only
// when the ground does - which is why it is taken here, where the tree line
// itself is taken, and not again until the weather has been over the map.
//
// Somebody looking for somewhere to plant asks this of a few hundred tiles
// at a time, and it costs a slope and a dampness on each of them; asked
// afresh every time, that search was half of all the looking the settlement
// did for anywhere to do anything.
func (g *Grid) readHolds() {
	if len(g.holds) != len(g.Tiles) {
		g.holds = make([]bool, len(g.Tiles))
	}
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			p := geom.Pos{X: i % g.W, Y: i / g.W}
			// Ground nothing about suits trees is below the line wherever the
			// line is: where less than woodsShare of the land suits trees at
			// all, the line reads as nothing and every tile ties with it - seed
			// 1 of the valley, cut by the water rather than by incise, came out
			// four fifths wood.
			suits := g.WoodsAt(p)
			g.holds[i] = !g.Tiles[i].Terrain.Tidal() && !g.TooSteep(p) && !g.Treeless(p) && suits >= g.woodsLine && suits > 0
		}
	})
}
