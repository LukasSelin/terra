package main

import (
	"image"
	"math"

	"github.com/LukasSelin/terra"
)

// The surface layers.
//
// A Landscape paints its material by weightmaps, one 8-bit image per layer
// at the heightmap's resolution, and at every vertex the layers' weights
// are meant to sum to 255. Each layer here is one kind of surface a game
// would give a material: what a tile is made of as the map knows it, read
// off Terrain, what stands on it as Wood and Sward, what its soil is made
// of as Sand and Clay, and how steep it is. At 25 m a vertex the blend is
// coarse; the metre level conditions it on the ground under it.

// The layers, in the order they are written.
const (
	lGrass = iota
	lForest
	lRock
	lSand
	lMud
	lIce
	lSalt
	lField
	lRiverbed
	lLakebed
	layerCount
)

var layerNames = [layerCount]string{
	lGrass:    "grass",
	lForest:   "forest",
	lRock:     "rock",
	lSand:     "sand",
	lMud:      "mud",
	lIce:      "ice",
	lSalt:     "salt",
	lField:    "field",
	lRiverbed: "riverbed",
	lLakebed:  "lakebed",
}

// weights are one vertex's share of each layer, summing to 255.
type weights [layerCount]uint8

// How steep ground turns to bare rock, and how thin its soil has to be.
const (
	// rockFrom is the slope, as a rise over a run, at which rock starts to
	// show through the cover, and rockAt the slope at which nothing else is
	// left: a fall of one in three is a hillside you would not plough, one
	// in one a crag.
	rockFrom = 0.35
	rockAt   = 1.0
	// thinSoil is how few metres of soil leave the rock showing, and
	// bareSoil how few leave nothing else.
	thinSoil = 0.3
	bareSoil = 0.05
)

// shares are the layers of tile i before they are quantised, as shares of
// one.
func shares(g *terra.Grid, i int, riverFlow float64) [layerCount]float64 {
	var s [layerCount]float64
	t := &g.Tiles[i]
	p := g.PosOf(i)
	underLake := false
	if lk, ok := g.LakeAt(p); ok && lk.Level > g.Height[i] {
		underLake = true
	}
	switch {
	case t.Terrain == terra.Ice:
		s[lIce] = 1
	case t.Terrain == terra.Salt || t.Terrain == terra.Pan:
		s[lSalt] = 1
	case t.Wet() && underLake:
		s[lLakebed] = 1
	case t.Wet() && g.Flow[i] > riverFlow:
		s[lRiverbed] = 1
	case t.Terrain == terra.Flat:
		s[lMud] = 1
	case t.Terrain == terra.Rock:
		s[lRock] = 1
	case !t.Wet() && g.Barren(p):
		s[lIce] = 1
	default:
		// Ground with something on it, or the sea's bed: what covers it,
		// and under the cover the rock where it is steep or the soil is
		// thin, and otherwise the soil itself by what it is made of.
		var forest, grass, field float64
		switch t.Terrain {
		case terra.Forest:
			forest = 0.5 + 0.5*clamp(g.Wood[i])
			grass = 1 - forest
		case terra.Grass:
			grass = clamp(g.Sward[i])
		case terra.Field:
			field = 1
		}
		rock := clamp((g.Slope(p) - rockFrom) / (rockAt - rockFrom))
		if soil := float64(g.Soil[i]); soil < thinSoil {
			rock = math.Max(rock, clamp((thinSoil-soil)/(thinSoil-bareSoil)))
		}
		ground := 1 - rock
		cover := forest + grass + field
		bare := 1 - cover
		s[lRock] = rock
		s[lForest] = ground * forest
		s[lGrass] = ground * grass
		s[lField] = ground * field
		s[lSand] = ground * bare * clamp(g.Sand[i])
		s[lMud] = ground * bare * (1 - clamp(g.Sand[i]))
	}
	return s
}

func clamp(x float64) float64 { return math.Max(0, math.Min(1, x)) }

// quantise turns shares of one into weights summing to 255 exactly: each
// layer takes the floor of its share, and what is left goes one at a time
// to the layers whose share was rounded down the most.
func quantise(s [layerCount]float64) weights {
	var w weights
	total := 0.0
	for _, x := range s {
		total += x
	}
	if total <= 0 {
		w[lRock] = 255
		return w
	}
	var frac [layerCount]float64
	left := 255
	for k, x := range s {
		v := x / total * 255
		w[k] = uint8(math.Floor(v))
		frac[k] = v - math.Floor(v)
		left -= int(w[k])
	}
	for ; left > 0; left-- {
		best := 0
		for k := 1; k < layerCount; k++ {
			if frac[k] > frac[best] {
				best = k
			}
		}
		w[best]++
		frac[best] = -1
	}
	return w
}

// allWeights is every tile's weights.
func allWeights(g *terra.Grid, riverFlow float64) []weights {
	w := make([]weights, len(g.Tiles))
	for i := range g.Tiles {
		w[i] = quantise(shares(g, i, riverFlow))
	}
	return w
}

// weightTile is the weightmap of layer k over tile (i, j): one 8-bit pixel
// per vertex, read off the same tiles the heightmap is.
func weightTile(g *terra.Grid, w []weights, t tiling, k, i, j int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, t.Size, t.Size))
	c0, r0 := t.origin(i, j)
	for r := 0; r < t.Size; r++ {
		row := img.Pix[r*img.Stride:]
		for c := 0; c < t.Size; c++ {
			row[c] = w[vertexTile(g, c0+c, r0+r)][k]
		}
	}
	return img
}
