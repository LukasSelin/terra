package terra

import "github.com/LukasSelin/terra/geom"

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

// WoodsAt is how well a tile's ground suits trees: damp enough to grow them
// and gentle enough to hold the soil they grow in. It is the reading the map
// was made with, and it moves when the weather moves the ground under it.
func (g *Grid) WoodsAt(p geom.Pos) float64 {
	if !g.In(p) || g.TooSteep(p) {
		return 0
	}
	t := g.At(p)
	damp := clamp01(1 - t.Drain/(2*FloodDepth))
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
			slopes[i] = g.Slope(geom.Pos{X: i % g.W, Y: i / g.W})
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
	suit := make([]float64, len(g.Tiles))
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			if g.Tiles[i].Wet() {
				continue // the river is not ground trees might have had
			}
			suit[i] = g.WoodsAt(geom.Pos{X: i % g.W, Y: i / g.W})
		}
	})
	suits := make([]float64, 0, len(g.Tiles))
	for i := range g.Tiles {
		if !g.Tiles[i].Wet() {
			suits = append(suits, suit[i])
		}
	}
	if len(suits) > 0 {
		g.woodsLine = quantile(suits, 1-woodsShare)
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
			g.holds[i] = !g.TooSteep(p) && !g.Frozen(p) && g.WoodsAt(p) >= g.woodsLine
		}
	})
}
