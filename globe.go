package terra

import "github.com/LukasSelin/terra/geom"

// The shape of the map. A valley has edges: walk off the side of it and
// there is nothing there. A globe has none in one direction - it is drawn
// as a cylinder, the east edge joined to the west, and the poles are the
// top and bottom rows. Everything that turns a position into a tile, or
// asks how far apart two positions are, comes through here, so that the
// rest of the world can go on thinking in X and Y and be right on both.
//
// Nothing in this file changes what an unwrapped map does: with Wrap off
// every function here is the plain arithmetic it replaced.

// WrapX brings a column onto the map by going round it.
func (g *Grid) WrapX(x int) int {
	if x >= 0 && x < g.W {
		return x
	}
	x %= g.W
	if x < 0 {
		x += g.W
	}
	return x
}

// Norm is p as the map holds it: the same column gone round the map to
// where it is stored. Rows are never moved; a row off the map is off it.
func (g *Grid) Norm(p geom.Pos) geom.Pos {
	if g.Wrap {
		p.X = g.WrapX(p.X)
	}
	return p
}

// Index is where the tile at p is kept.
func (g *Grid) Index(p geom.Pos) int {
	if g.Wrap {
		return p.Y*g.W + g.WrapX(p.X)
	}
	return p.Y*g.W + p.X
}

// PosOf is the position of the tile kept at i.
func (g *Grid) PosOf(i int) geom.Pos {
	return geom.Pos{X: i % g.W, Y: i / g.W}
}

// Dist is the Chebyshev distance between two positions, going round the
// map where that is shorter.
func (g *Grid) Dist(a, b geom.Pos) int {
	dx, dy := a.X-b.X, a.Y-b.Y
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if g.Wrap {
		dx %= g.W
		if back := g.W - dx; back < dx {
			dx = back
		}
	}
	if dx > dy {
		return dx
	}
	return dy
}

// Delta is the shortest signed step from one position to another: how far
// east and how far south, with east going round the map where the way
// round is shorter than the way across.
func (g *Grid) Delta(from, to geom.Pos) geom.Pos {
	d := geom.Pos{X: to.X - from.X, Y: to.Y - from.Y}
	if g.Wrap {
		d.X %= g.W
		if d.X > g.W/2 {
			d.X -= g.W
		} else if d.X < -g.W/2 {
			d.X += g.W
		}
	}
	return d
}

// Toward is from moved one tile straight toward to, the short way round.
func (g *Grid) Toward(from, to geom.Pos) geom.Pos {
	d := g.Delta(from, to)
	return g.Norm(geom.Pos{X: from.X + sign(d.X), Y: from.Y + sign(d.Y)})
}

func sign(v int) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	}
	return 0
}

// Columns is the columns within radius of x, as offsets from it: clipped to
// the map on a valley, and on a globe the whole way round at most, so that
// a window wider than the map reads each column once.
func (g *Grid) Columns(x, radius int) (lo, hi int) {
	if !g.Wrap {
		return max(0, x-radius) - x, min(g.W-1, x+radius) - x
	}
	if 2*radius+1 >= g.W {
		half := g.W / 2
		return -half, g.W - 1 - half
	}
	return -radius, radius
}
