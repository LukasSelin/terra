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

// Generate is GenerateTerrain on the given terms.
func (w *Land) Generate(cfg Terms) {
	width, height := cfg.Width, cfg.Height
	g := NewGrid(width, height)
	g.Wrap = cfg.Wrap
	// The air is the climate's, read row by row, and it is set before the
	// ground is made because a history rains on its ground while it runs.
	g.air = w.Climate.airFor(g, cfg.Wetness)

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
	// The sea: poured, where there is a history to say how deep the basins
	// are, and otherwise the lowest share of the ground. See water.go.
	poured := cfg.Epochs > 0 && cfg.Water > 0
	if poured {
		g.pour(cfg.Water, w.RNG)
	} else {
		g.flood(cfg.SeaShare, w.RNG)
	}
	g.drain()
	// The water cuts its valley before the valley is asked where the water
	// goes: incise moves the ground, so the drainage has to be taken again on
	// the ground it left. See Incise.
	g.incise()
	// And the sea is levelled again on the ground the cutting left. See
	// Grid.relevel.
	if poured {
		g.repour(cfg.Water)
	} else {
		g.relevel(cfg.SeaShare)
	}
	g.drain()
	g.carve(w.RNG)
	g.height()
	// The heights are settled and the coast is where it is going to be, so
	// where the ground is too cold to grow anything can be written down: the
	// year's mean at that latitude, warmed by how much sea lies round the
	// tile. It is read by the woods below, by the tree line seed falls on
	// for the rest of the run, and by anybody asking what a tile is; see
	// Grid.Frozen and Maritime.
	sea := g.seaNear(g.Span() / maritimeSpan)
	g.frost = make([]float64, len(g.Tiles))
	g.EachRow(func(y int) {
		for i := y * width; i < (y+1)*width; i++ {
			g.frost[i] = w.Climate.frostlineAt(y, maritime(sea[i]))
		}
	})
	// And with it, where the sea itself never thaws. See Grid.freeze.
	g.freeze()
	// The tide's reach, and the flats it covers and uncovers, before the woods
	// are shared out: a wood's share is a share of ground trees could have.
	g.tides()

	// Woods stand where the ground is damp enough to grow them and gentle
	// enough to hold soil: the valley sides above the flood, not the crown of
	// the ridge and not the bed of the river. Each tile is scored on how well
	// it suits trees, with a little luck thrown in so that two maps with the
	// same bones are not the same map, and the best of them are wooded. The
	// share is fixed rather than the score, because how wet a map is depends
	// on the shape of it and a settlement needs roughly the same timber
	// whatever ground it was given.
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
		score[i] = suit[i] + 0.35*w.RNG.Float64()
		if g.Tiles[i].Terrain == Grass {
			wooded = append(wooded, score[i])
		}
	}
	treeLine := quantile(wooded, 1-forestShare)
	for i := range g.Tiles {
		t := &g.Tiles[i]
		p := geom.Pos{X: i % width, Y: i / width}
		if t.Terrain != Grass || score[i] < treeLine || g.TooSteep(p) {
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
			heights[i] = g.Tiles[i].Height
			slopes[i] = g.Slope(geom.Pos{X: i % width, Y: i / width})
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
	// The poles are bare, and so are the peaks. Ground whose year never warms
	// past the frost grows nothing, and an outcrop is the ground that grows
	// nothing. This was written of the poles alone, because latitude was the
	// only thing the weather knew; with the height in it too, the same
	// sentence puts snow on a mountain and does not have to name one.
	g.EachRow(func(y int) {
		for x := 0; x < width; x++ {
			p := geom.Pos{X: x, Y: y}
			if i := g.Index(p); g.Frozen(p) && g.Tiles[i].Terrain != Flat {
				g.Tiles[i].Terrain, g.Wood[i], g.Wild[i] = Rock, 0, 0
			}
		}
	})

	// What the soil is made of, before what it will grow is asked: the
	// fertility below reads the mixture, so the mixture has to be there.
	g.soilTexture()

	// Good soil is where the water has been and stopped: the flat of a valley,
	// damp from what drains through it, facing the sun, over a mixture that
	// will hold what it is given. See Grid.SoilAt, which is the same reading
	// the weather takes every age.
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
}
