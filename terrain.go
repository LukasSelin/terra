package terra

import (
	"github.com/LukasSelin/terra/geom"
	"math"
)

// panSoil is what a salt flat will grow: next to nothing, because what the
// water left behind on it is salt.
const panSoil = 0.05

// GenerateTerrain raises the ground, lets the water find its way down it, and
// reads everything else off what that leaves: the woods where it is damp and
// not too steep, the outcrops where it is high and bare, the good soil on the
// valley floors and the sunny slopes. Nothing here is drawn on top of the
// land; see relief.go for the shape of it.
func (w *Land) GenerateTerrain(width, height int) {
	w.Generate(Terms{Width: width, Height: height})
}

// valleyYears is how long a drawn map's valleys are cut for before anybody
// lives in them, and valleyRounds how many times over the channels, the slides,
// the rock and the drainage are worked out again in the course of it.
//
// A shaped map is graded as water wearing at the rate the land rises would
// grade it - see shape.go - and nothing had cut into it: a river lay on the
// surface of its country like a line drawn on it. It used to be cut by a pass
// of its own, incise, that took twelve metres times the root of a tile's share
// of a great river's flow off every tile, divided by a hardness, and blurred
// seventy-five metres wide. Nothing in that was the water's work: it did not
// know how steeply the river fell, how long it had had, or what grew beside it.
//
// Now the valleys are cut by the same water that wears them for the rest of the
// run - stream power over the rock, less what grows, and the settling - and at
// the real rate that water cuts at, which is slow. Two thousand years does
// little, and what it does it does to the channels. Over five valleys and the
// small globes, by the years, when it was set:
//
//	years     mean slope   Hack    Rb     small globes' valley spacing, m   seed 1 turning
//	     0       0.463     0.589   3.17              133                          0.39
//	  2000       0.466     0.591   3.29              146                          0.38
//	 10000        -        0.599   3.46              320                          0.44
//	 20000       0.463     0.578   4.54              320                          0.48
//	100000       0.504     0.522   3.00              320                          0.56
//
// Past a few thousand years the small globes' first-order valleys came out
// three hundred metres apart and the mainstreams too short for their basins.
// Cutting without the lift the shaping assumed takes the grading out of the
// ground, too: the valleys' concavity - see realism_test.go - goes from 0.34
// uncut to 0.30 at two thousand years.
const (
	valleyYears  = 2000 * yr
	valleyRounds = 4
)

// cutValleys runs valleyYears of weather over a freshly drawn map, working the
// channels, the slides, the rock and the drainage out again each round.
func (g *Grid) cutValleys(rng interface{ Float64() float64 }) {
	defer phase("cutValleys")()
	for range valleyRounds {
		g.carve(rng)
		g.wear(valleyYears / float64(valleyRounds))
		g.landslide(false)
		g.expose()
		g.drain()
	}
}

// Generate is GenerateTerrain on the given terms: every stage, in order, on
// a new grid. See stages.go.
func (w *Land) Generate(cfg Terms) {
	defer phase("Generate")()
	w.generateFrom(w.newGround(cfg), cfg, stageGround, len(stages))
}

// newGround is the grid a world is made on, before any stage has run.
func (w *Land) newGround(cfg Terms) *Grid {
	g := NewGrid(cfg.Width, cfg.Height)
	g.Wrap = cfg.Wrap
	// The air is the climate's, read row by row, and it is set before the
	// ground is made because a history rains on its ground while it runs.
	g.air = w.Climate.airFor(g, cfg.Wetness)
	return g
}

// stageGround is the ground and the rock under it: a history's, or drawn.
func (w *Land) stageGround(g *Grid, cfg Terms) {
	if cfg.Epochs > 0 {
		// A world that made itself: the land and the rock under it are both
		// what its history left. See history.go.
		w.history(g, cfg.Epochs, cfg.SeaShare, cfg.Water)
	} else {
		w.raise(g)
		// What is under the ground is laid down with the ground, and before
		// the water has been anywhere: a river runs over the rock it finds.
		w.layBedrock(g)
	}
}

// poured reports whether a world's sea is poured from its water rather than
// flooded to a share. See water.go.
func (cfg Terms) poured() bool { return cfg.Epochs > 0 && cfg.Water > 0 }

// stageSea is the sea: poured, where there is a history to say how deep the
// basins are, and otherwise the lowest share of the ground. See water.go.
func (w *Land) stageSea(g *Grid, cfg Terms) {
	if cfg.poured() {
		g.pour(cfg.Water, w.RNG)
	} else {
		g.flood(cfg.SeaShare, w.RNG)
	}
}

// stageShape is the ground laid again at a map's tile span, and the rock and
// the drainage read off it.
func (w *Land) stageShape(g *Grid, cfg Terms) {
	// The ground as the water would have worn it, with the first-order valleys
	// cut into its hillsides, and what that leaves too steep to stand brought
	// down. See shape.go and slide.go.
	area := g.shape()
	w.texture(g, area)
	// And the soft beds taken down against the hard ones, which is where the
	// ridges and the scarps of layered country come from. See denude.
	g.denude()
	g.landslide(false)
	// The ground has moved into the beds under it, so the rock it is made of
	// has too. See strata.go.
	g.expose()
	g.drain()
}

// stageCut is the valleys cut, and the sea levelled on what that leaves.
func (w *Land) stageCut(g *Grid, cfg Terms) {
	// The water cuts its valley before the valley is asked where the water
	// goes: the cutting moves the ground, so the drainage has to be taken
	// again on the ground it left. See valleyYears.
	if cfg.Glacial {
		g.cutThroughCycle(w.RNG)
	} else {
		g.cutValleys(w.RNG)
	}
	// And the sea is levelled again on the ground the cutting left. See
	// Grid.relevel.
	if cfg.poured() {
		g.repour(cfg.Water)
	} else {
		g.relevel(cfg.SeaShare)
	}
	g.drain()
	g.carve(w.RNG)
	g.height()
}

// stageCoast is each tile's year, the sea's ice, the tide and the mud it
// lays.
func (w *Land) stageCoast(g *Grid, cfg Terms) {
	width := g.W
	// The heights are settled and the coast is where it is going to be, so the
	// year each tile has can be written down: the year's mean at that
	// latitude, warmed by how much sea lies round the tile and warmed or
	// chilled by the currents off its coast, and the swing round it that the
	// land about it allows. It is read by the woods below, by the tree line
	// seed falls on for the rest of the run, and by anybody asking what a tile
	// is; see Grid.Frozen, Grid.Treeless, Maritime and Grid.CoastWarmth.
	sea := g.seaNear(g.Span() / maritimeSpan)
	g.warm, g.swing = make([]float32, len(g.Tiles)), make([]float32, len(g.Tiles))
	g.EachRow(func(y int) {
		// The sea about a place is read against the sea about its row: the
		// latitude's mean is the energy balance's, which is the land's and the
		// sea's together already, so more sea than the row has is a warmer
		// year and less a colder one, and the row as a whole keeps its mean.
		// Read as a warming of every tile by all the sea about it, every
		// globe's ground stood some four degrees over its latitude, which
		// pushed the subtropics' winters over eighteen and their dry line
		// eighty millimetres up. A current is warm or cold the year round.
		var row float64
		for i := y * width; i < (y+1)*width; i++ {
			row += sea[i]
		}
		row /= float64(width)
		for i := y * width; i < (y+1)*width; i++ {
			m := maritime(sea[i] - row)
			g.warm[i] = float32(w.Climate.seaMeanAt(y, m+g.CoastWarmth(i)))
			g.swing[i] = Swing
			if w.Climate.globe {
				// The sea's moderation of the swing is swingAt's, off the land
				// round about; taking the maritime share off it as well counted
				// the same sea twice.
				g.swing[i] = float32(swingAt(w.Climate.latitude(y), g.contAt(i)))
			}
		}
	})
	// And with it, where the sea itself never thaws. See Grid.freeze.
	g.freeze()
	// The tide's reach, and the flats it covers and uncovers, before the woods
	// are shared out: a wood's share is a share of ground trees could have.
	g.tides()
	// And the mud the rivers have brought the tide since the sea stood where
	// it stands, which is what a flat is made of. See silt.
	g.silt(func() {
		if cfg.poured() {
			g.repour(cfg.Water)
		} else {
			g.relevel(cfg.SeaShare)
		}
	})
}

// stageCover is what stands on the ground and what it will grow: the woods,
// the outcrops, the soil; and the things the tiles make up.
func (w *Land) stageCover(g *Grid, cfg Terms) {
	width := g.W

	// Woods stand where the ground is damp enough to grow them and gentle
	// enough to hold soil: the valley sides above the flood, not the crown of
	// the ridge and not the bed of the river. Each tile is scored on how well
	// it suits trees, with a little luck thrown in so that two maps with the
	// same bones are not the same map, and the best of them are wooded. The
	// share is fixed rather than the score, because how wet a map is depends
	// on the shape of it and a settlement needs roughly the same timber
	// whatever ground it was given.
	//
	// That is the tuned rule. Under the climate's there is no share: a wood
	// stands where the climate and the lie of the ground let one stand, which
	// is most of a wet country and the river bottoms of a dry one and nothing
	// of a desert. The founding woods still take a little over half of the
	// ground that could hold one, the best of it by the same luck. See
	// Terms.Woods and Grid.WoodsAt.
	g.climateWoods = cfg.Woods.climate(cfg.Wrap)
	g.readWoods()
	// How well the ground suits trees is read over the rows; the luck
	// thrown on top of it is drawn here, one to a tile in tile order, as it
	// always was. What a seed means is the order its chance comes out in,
	// so the drawing never goes anywhere but this goroutine.
	suit := make([]float64, len(g.Tiles))
	g.EachRow(func(y int) {
		for i := y * width; i < (y+1)*width; i++ {
			suit[i] = g.WoodsAt(geom.Pos{X: i % width, Y: i / width})
		}
	})
	wooded := make([]float64, 0, len(g.Tiles))
	score := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		score[i] = suit[i] + luck*w.RNG.Float64()
		if g.Tiles[i].Terrain == Grass {
			wooded = append(wooded, score[i])
		}
	}
	treeLine := quantile(wooded, 1-forestShare)
	if g.climateWoods {
		// The line the climate draws, and the luck over it: ground just on the
		// line is wooded where its draw is in the best forestShare/woodsShare
		// of draws, and better ground more often.
		treeLine = climateLine + luck*(1-forestShare/woodsShare)
	}
	for i := range g.Tiles {
		t := &g.Tiles[i]
		p := geom.Pos{X: i % width, Y: i / width}
		if t.Terrain != Grass || score[i] < treeLine || g.TooSteep(p) {
			continue
		}
		if g.climateWoods && !g.HoldsWood(p) {
			continue
		}
		t.Terrain = Forest
		g.Wood[i] = 0.6 + 0.4*w.RNG.Float64()
		g.Wild[i] = 0.6 + 0.4*w.RNG.Float64()
		g.Standing(i) // the woods a map is made with are old woods
	}

	// Outcrops are where the soil has gone: high, steep ground the water runs
	// off rather than soaks into. Scored and shared the same way, because an
	// outcrop is a comparison with the rest of the map, not a measurement.
	heights := make([]float64, len(g.Tiles))
	slopes := make([]float64, len(g.Tiles))
	g.EachRow(func(y int) {
		for i := y * width; i < (y+1)*width; i++ {
			// Read as the map stood before its deep floor was laid: see laidHeight.
			heights[i] = g.laidHeight(i)
			slopes[i] = g.laidSlope(geom.Pos{X: i % width, Y: i / width})
		}
	})
	highAt := quantile(heights, 0.6)
	bare := make([]float64, len(g.Tiles))
	open := make([]float64, 0, len(g.Tiles))
	for i := range g.Tiles {
		bare[i] = clamp01(slopes[i]/math.Max(1e-12, g.steepAt)) +
			clamp01((heights[i]-highAt)/math.Max(1e-12, g.Skyline()-highAt))
		if g.Tiles[i].Terrain == Grass {
			open = append(open, bare[i])
		}
	}
	stoneLine := quantile(open, 1-rockShare)
	for i := range g.Tiles {
		if t := &g.Tiles[i]; t.Terrain == Grass && bare[i] >= stoneLine {
			t.Terrain = Rock
		}
	}
	// The ice caps are bare, and so are the peaks under snow. Ground whose
	// summer never melts its snow grows nothing, and an outcrop is the ground
	// that grows nothing. This was written of the poles alone, because
	// latitude was the only thing the weather knew; with the height in it too,
	// the same sentence puts snow on a mountain and does not have to name one.
	//
	// And above the tree line what stands is not a wood. It was the whole of
	// the ground under the growing frost that went to bare rock, which made
	// the tundra an outcrop; tundra is open ground that grows little, and it
	// is left open.
	g.EachRow(func(y int) {
		for x := 0; x < width; x++ {
			p := geom.Pos{X: x, Y: y}
			i := g.Index(p)
			switch t := &g.Tiles[i]; {
			case t.Terrain == Flat:
			case g.Barren(p):
				t.Terrain, g.Wood[i], g.Wild[i] = Rock, 0, 0
			case t.Terrain == Forest && g.Treeless(p):
				t.Terrain, g.Wood[i], g.Wild[i] = Grass, 0, 0
			}
		}
	})

	// What the soil is made of, before what it will grow is asked: the
	// fertility below reads the mixture, so the mixture has to be there. And
	// how much of it there is, which reads what grows on the ground, so the
	// woods and the outcrops come first. See soil.go.
	g.soilTexture()
	g.laySoil(cfg.Epochs > 0)

	// Good soil is deep soil with water in it: the flat of a valley, damp from
	// what drains through it, facing the sun, over a mixture that will hold
	// what it is given, in a climate that grows something to feed it. See
	// Grid.SoilAt, which is the same reading the weather takes every age.
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if t.Wet() || t.Terrain.Tidal() {
			continue
		}
		if t.Terrain == Pan {
			g.Fertility[i], g.Rich[i] = panSoil, panSoil
			continue
		}
		g.Fertility[i] = g.SoilAt(geom.Pos{X: i % width, Y: i / width})
		g.Rich[i] = g.Fertility[i]
	}
	g.Recount()
	w.Forest0 = g.Forest()

	w.Grid = g
	// The things the tiles make up, joined once the ground and the water
	// are where they are going to be. See features.go.
	g.readFeatures()
}
