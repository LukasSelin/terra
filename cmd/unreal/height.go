package main

import (
	"fmt"
	"image"
	"math"

	"github.com/LukasSelin/terra"
)

// The Landscape and its heights.
//
// Unreal's Landscape is a grid of vertices, one quad between neighbours,
// and it reads its heights from a 16-bit grayscale image with one pixel per
// vertex: 32768 is the actor's own height, and the whole 16-bit range spans
// 512 m times the actor's Z scale over a hundred. Each tile of the world is
// one vertex here, at the tile's centre, so a world W tiles across is W
// vertices across - one more on a globe, where the east edge's neighbour is
// the west edge and the seam column is written twice so the two meet.
//
// One Landscape actor is at most 8129 vertices a side and the sizes that fit
// its components are 1009, 2017, 4033 and 8129, so a large world is cut into
// tiles of one of those, neighbours sharing their edge column and row the way
// Unreal's tiled import expects, and padded out to whole tiles by carrying the
// last real vertex on to the edge.

// tileSizes are the Landscape sizes Unreal recommends, in vertices a side,
// smallest first.
var tileSizes = []int{1009, 2017, 4033, 8129}

// xyScale is the Landscape's XY scale, in Unreal's hundredths of a metre a
// quad: twenty-five metres, one tile. A later milestone makes a metre level
// and imports it at 100.
const xyScale = 100 * terra.TileSpan

// tiling is how a world's vertices are cut into Landscape tiles.
type tiling struct {
	// VertsW and VertsH are how many vertices the world has across and
	// down, the seam column counted on a globe.
	VertsW, VertsH int
	// Size is the vertices a tile has a side, and NX and NY how many tiles
	// there are across and down. Neighbouring tiles share one column or
	// row, so the tiles cover NX*(Size-1)+1 vertices across.
	Size   int
	NX, NY int
}

// tileFor cuts vertsW by vertsH vertices into tiles of the largest of the
// sizes that is not bigger than the world's longer side, or the smallest
// size where none fits, the world padded out to whole tiles.
func tileFor(vertsW, vertsH int, sizes []int) tiling {
	longest := max(vertsW, vertsH)
	size := sizes[0]
	for _, s := range sizes {
		if s <= longest {
			size = s
		}
	}
	across := func(verts int) int { return max(1, (verts-2)/(size-1)+1) }
	return tiling{VertsW: vertsW, VertsH: vertsH, Size: size, NX: across(vertsW), NY: across(vertsH)}
}

// vertsOf is how many vertices a world of g's shape has: one a tile, and one
// more across on a globe for the seam.
func vertsOf(g *terra.Grid) (w, h int) {
	w, h = g.W, g.H
	if g.Wrap {
		w++
	}
	return w, h
}

// origin is the world vertex at the top left of tile (i, j).
func (t tiling) origin(i, j int) (c, r int) {
	return i * (t.Size - 1), j * (t.Size - 1)
}

// vertexTile is the tile of g that world vertex (c, r) reads: the tile at
// (c, r), or the last real one past the world's edge, where the padding
// carries the edge on, and the west edge on the seam column of a globe.
func vertexTile(g *terra.Grid, c, r int) int {
	if c >= g.W {
		if g.Wrap {
			c = 0
		} else {
			c = g.W - 1
		}
	}
	if r >= g.H {
		r = g.H - 1
	}
	return r*g.W + c
}

// zEncoding is how a height in metres becomes a 16-bit pixel.
type zEncoding struct {
	// Scale is the Landscape actor's Z scale, whole so that it can be typed
	// into the editor, and Offset the height in metres a pixel of 32768
	// stands for, which is where the actor is put.
	Scale  int
	Offset float64
}

// zMargin is how much more than the world's range of heights the encoding
// leaves room for, so that neither end is clipped.
const zMargin = 0.1

// encodingFor is the encoding that fits heights from lo to hi metres with a
// margin: the offset in the middle of them, and the smallest whole Z scale
// whose 512 m * scale / 100 holds the range.
func encodingFor(lo, hi float64) zEncoding {
	span := (hi - lo) * (1 + zMargin)
	scale := max(1, int(math.Ceil(span*100/512)))
	return zEncoding{Scale: scale, Offset: (lo + hi) / 2}
}

// Step is the height one pixel step is worth, in metres.
func (e zEncoding) Step() float64 { return float64(e.Scale) / 12800 }

// Encode is the pixel for h metres.
func (e zEncoding) Encode(h float64) uint16 {
	v := 32768 + math.Round((h-e.Offset)*12800/float64(e.Scale))
	return uint16(math.Max(0, math.Min(65535, v)))
}

// Decode is the height a pixel stands for, in metres.
func (e zEncoding) Decode(v uint16) float64 {
	return e.Offset + (float64(v)-32768)*float64(e.Scale)/12800
}

// Formula is Decode written out, for the manifest.
func (e zEncoding) Formula() string {
	return fmt.Sprintf("metres = %g + (value - 32768) * %d / 12800", e.Offset, e.Scale)
}

// heightRange is the lowest and highest ground on the map, the deep floor
// of a watered globe included: the Landscape carries the sea bed too.
func heightRange(g *terra.Grid) (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, h := range g.Height {
		lo, hi = math.Min(lo, h), math.Max(hi, h)
	}
	if lo > hi {
		return 0, 0
	}
	return lo, hi
}

// heightTile is the heightmap of tile (i, j): one 16-bit pixel per vertex.
func heightTile(g *terra.Grid, t tiling, e zEncoding, i, j int) *image.Gray16 {
	img := image.NewGray16(image.Rect(0, 0, t.Size, t.Size))
	c0, r0 := t.origin(i, j)
	for r := 0; r < t.Size; r++ {
		row := img.Pix[r*img.Stride:]
		for c := 0; c < t.Size; c++ {
			v := e.Encode(g.Height[vertexTile(g, c0+c, r0+r)])
			row[2*c], row[2*c+1] = byte(v>>8), byte(v)
		}
	}
	return img
}

// tileName is the file name of tile (i, j) of the layer named layer, or of
// the heightmap when layer is empty: the naming Unreal's tiled import reads
// the grid off.
func tileName(name, layer string, i, j int) string {
	if layer != "" {
		name += "_" + layer
	}
	return fmt.Sprintf("%s_x%d_y%d.png", name, i, j)
}
