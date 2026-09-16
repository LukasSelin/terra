package terra

import (
	"math"
	"sort"
)

// A history runs on a grid of its own. Today that grid is the map itself:
// historyShrink is one, historyGround hands the map back, and nothing is
// handed down. What this file is for is the day it is not - phase 3 of
// docs/perf/scaling-plan.md - when the plates move over a grid sized by the
// planet and the epochs rather than by the map a game plays on, and a
// finished history is laid onto the map by handDown.
//
// What crosses, and how:
//
//   - the heights, the rate the rock is rising and the depth of the floor
//     by its age are fields, read between the history's tiles (bilinear);
//   - a tile - its plate, the epoch its rock and its surface date from, the
//     soil the history left - and the book's line and the pile of beds under
//     it are the nearest history tile's, the beds moved up or down by what
//     the reading between the tiles moved the ground;
//   - which tiles are ocean crust is the nearest tile's;
//   - the water, the weather and the lakes do not cross: the map reads them
//     afresh on its own ground, in settleHistory's drain.
//
// The planet is the map's: how many plates it breaks into, how many hotspots
// it has, how far a plate goes in an epoch and so how many kilometres a tile
// is while the history runs (deepSpan) all follow from the map's size, as they
// did when the history ran on the map. A history grid is that planet read
// coarser, and whatever the history counts in tiles - the grain of the crust
// and of the ranges, the swells the plates ride in, the fewest tiles a belt is
// drawn over, the fold's wave, the passes that soften a margin - is counted in
// the map's tiles and read in the history grid's through coarseness, inTiles
// and passes. On the map each of those is the number it always was, to the
// bit. What cannot be the same on a coarser grid is the chance: a draw a tile
// is a different stream over fewer tiles, so a coarser history is the same
// planet in its measures and not in its bits.

// historyShrink is how many map tiles a side one history tile is. One is the
// map itself. It is a variable so that the tests can run a history on a grid
// of another size; nothing else sets it.
var historyShrink = 1

// historyGround is the grid a history runs on for the map g: g itself, or a
// grid historyShrink times coarser with an air of its own.
func (w *Land) historyGround(g *Grid, cfg Terms) *Grid {
	hw, hh := max(1, g.W/historyShrink), max(1, g.H/historyShrink)
	if hw == g.W && hh == g.H {
		return g
	}
	h := NewGrid(hw, hh)
	h.Wrap = g.Wrap
	h.planet = g.Span()
	// The air reads the rows as latitudes, so it is the climate of a map as
	// many rows high as the history's grid.
	h.air = NewClimateOn(Terms{Width: hw, Height: hh, Wrap: cfg.Wrap}).airFor(h, cfg.Wetness)
	return h
}

// reading is where a map tile's centre falls on a history's grid: the
// nearest history tile, and the four it lies between with its weights.
type reading struct {
	near   int32
	x0, x1 int32
	y0, y1 int32
	tx, ty float64
}

// handDown lays the history run on from onto the map to, and returns what
// the history hands on beside the grid, read onto the map's tiles. to has to
// be a new grid: nothing on it is kept.
func handDown(from, to *Grid, d *deepStage) *deepStage {
	n := len(to.Tiles)
	sx, sy := float64(from.W)/float64(to.W), float64(from.H)/float64(to.H)
	at := make([]reading, n)
	to.EachRow(func(y int) {
		fy := (float64(y)+0.5)*sy - 0.5
		ny := clampInt(int(math.Floor((float64(y)+0.5)*sy)), 0, from.H-1)
		y0 := int(math.Floor(fy))
		ty := fy - float64(y0)
		y1 := clampInt(y0+1, 0, from.H-1)
		y0 = clampInt(y0, 0, from.H-1)
		for x := 0; x < to.W; x++ {
			fx := (float64(x)+0.5)*sx - 0.5
			nx := clampInt(int(math.Floor((float64(x)+0.5)*sx)), 0, from.W-1)
			x0 := int(math.Floor(fx))
			tx := fx - float64(x0)
			x1 := x0 + 1
			if from.Wrap {
				x0, x1 = (x0+from.W)%from.W, x1%from.W
			} else {
				x0, x1 = clampInt(x0, 0, from.W-1), clampInt(x1, 0, from.W-1)
			}
			at[y*to.W+x] = reading{
				near: int32(ny*from.W + nx),
				x0:   int32(x0), x1: int32(x1),
				y0: int32(y0 * from.W), y1: int32(y1 * from.W),
				tx: tx, ty: ty,
			}
		}
	})
	field := func(src []float64) []float64 {
		if src == nil {
			return nil
		}
		out := make([]float64, n)
		to.EachRow(func(y int) {
			for i := y * to.W; i < (y+1)*to.W; i++ {
				r := &at[i]
				a := src[r.y0+r.x0] + r.tx*(src[r.y0+r.x1]-src[r.y0+r.x0])
				b := src[r.y1+r.x0] + r.tx*(src[r.y1+r.x1]-src[r.y1+r.x0])
				out[i] = a + r.ty*(b-a)
			}
		})
		return out
	}

	copy(to.Height, field(from.Height))
	if from.strata != nil {
		to.strata = make([]column, n)
	}
	if from.ledger != nil {
		to.ledger = make([]ledger, n)
	}
	to.EachRow(func(y int) {
		for i := y * to.W; i < (y+1)*to.W; i++ {
			j := int(at[i].near)
			to.Tiles[i] = from.Tiles[j]
			to.Soil[i], to.Sand[i], to.Clay[i] = from.Soil[j], from.Sand[j], from.Clay[j]
			if to.strata != nil {
				to.strata[i] = from.strata[j]
				to.strata[i].lift(to.Height[i] - from.Height[j])
			}
			if to.ledger != nil {
				to.ledger[i] = from.ledger[j]
			}
		}
	})
	to.plateRoot, to.epochs = from.plateRoot, from.epochs
	if d.book != nil {
		layFeet(from, to, d, at, field)
	}
	to.repatch()

	out := &deepStage{depths: field(d.depths), shares: field(d.shares), uplift: field(d.uplift)}
	if d.ocean != nil {
		out.ocean = make([]bool, n)
		for i := range out.ocean {
			out.ocean[i] = d.ocean[at[i].near]
		}
	}
	return out
}

// planetSpan is the Span of the planet g's history is of: the map's.
func (g *Grid) planetSpan() int {
	if g.planet > 0 {
		return g.planet
	}
	return g.Span()
}

// coarseness is how many of the map's tiles one of g's is across: one on the
// map, and exactly one, so that a count read through it is the count.
func (g *Grid) coarseness() float64 {
	if g.planet <= 0 {
		return 1
	}
	return float64(g.planet) / float64(g.Span())
}

// inTiles is a length counted in the map's tiles, read in g's, and never less
// than one of them: nothing is drawn finer than a tile.
func (g *Grid) inTiles(mapTiles float64) float64 {
	if g.planet <= 0 {
		return mapTiles
	}
	return math.Max(1, mapTiles/g.coarseness())
}

// passes is how many passes of the nine-tile spread on g soften as far as n
// passes do on the map: a spread reaches as the root of its passes, so a grid
// c times coarser needs n over c squared.
func (g *Grid) passes(n int) int {
	if g.planet <= 0 {
		return n
	}
	c := g.coarseness()
	return int(math.Round(float64(n) / (c * c)))
}

// bandShare is how much of a tile of g lies in a band beside a seam that is
// drawn in the map's tiles. On the map it is the map's own rule, onMap, whole
// or not at all: a tile at distance a from the seam is in the band where
// mapLo <= a <= mapHi. On a grid c times coarser a tile at distance A stands
// for c of the map's tiles across, at distances A·c, A·c+1, ... A·c+c-1, and
// its share is how many of those the map's rule takes, over c: at a
// coarseness of one, the map's rule itself.
//
// Counted whole instead, a band narrower than a tile was a tile wide and
// schist doubled with every halving of the history grid; counted as the
// share of the tile's reach [A·c, (A+1)·c] that lies in [mapLo, mapHi+1], a
// band with edges between two tiles took a tile more than the map's rule
// gives it, and an arc's crushing and fire came out a quarter wider than on
// the map.
func (g *Grid) bandShare(onMap bool, away, mapLo, mapHi float64) float64 {
	if g.planet <= 0 {
		if onMap {
			return 1
		}
		return 0
	}
	c := g.coarseness()
	base := away * c
	first := math.Ceil(math.Max(0, mapLo-base))
	last := math.Floor(math.Min(c-1, mapHi-base))
	return math.Max(0, last-first+1) / c
}

// layFeet lays the foot of every pile on the map, where the history ran on a
// coarser grid and left the feet to be laid: each map tile by the book of the
// history tile nearest it, less whatever banded making does not reach it.
// Of the map tiles under one history tile, a band that covered a share of
// that tile reaches the same share of them, the ones nearest the band's
// middle by the distance read between the history's tiles, the lower index
// first where two are as near. So a band keeps its width on the map whatever
// the size of the grid the history ran on.
func layFeet(from, to *Grid, d *deepStage, at []reading, field func([]float64) []float64) {
	n := len(to.Tiles)
	// The map tiles under each history tile, in index order.
	under := make([][]int32, len(from.Tiles))
	for i := range at {
		j := at[i].near
		under[j] = append(under[j], int32(i))
	}
	reach := make([][makings]bool, n)
	for k := 0; k < makings; k++ {
		near := make([]float64, len(from.Tiles))
		for j := range near {
			near[j] = math.MaxFloat32 // nothing of this making: as far as can be
			if b := &d.book[j]; b.share[k] > 0 {
				near[j] = float64(b.near[k])
			}
		}
		onMap := field(near)
		var kids []int32
		for j := range from.Tiles {
			b := &d.book[j]
			if b.whole[k] {
				for _, i := range under[j] {
					reach[i][k] = true
				}
				continue
			}
			if b.share[k] <= 0 || len(under[j]) == 0 {
				continue
			}
			kids = append(kids[:0], under[j]...)
			sort.SliceStable(kids, func(a, c int) bool { return onMap[kids[a]] < onMap[kids[c]] })
			take := int(math.Round(float64(b.share[k]) * float64(len(kids))))
			for _, i := range kids[:min(take, len(kids))] {
				reach[i][k] = true
			}
		}
	}
	for i := range to.Tiles {
		j := at[i].near
		b := d.book[j]
		if !reach[i][makingMelt] {
			b.melt = 0
		}
		if !reach[i][makingPluton] {
			b.pluton = 0
		}
		if !reach[i][makingCrush] {
			b.crush = 0
		}
		to.cookFoot(i, b, d.ocean[j])
	}
	to.expose()
}

func clampInt(v, lo, hi int) int { return min(hi, max(lo, v)) }
