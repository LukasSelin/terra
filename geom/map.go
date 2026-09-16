package geom

// The shape of the map. A valley has edges: walk off the side of it and
// there is nothing there. A globe has none in one direction - it is drawn
// as a cylinder, the east edge joined to the west, and the poles are the
// top and bottom rows. Everything that turns a position into a tile, or
// asks how far apart two positions are, comes through here, so that the
// rest of the world can go on thinking in X and Y and be right on both.
//
// Nothing in this file changes what an unwrapped map does: with Wrap off
// every function here is the plain arithmetic it replaced.

// Map is the size of a map and whether it goes round: W columns and H rows,
// kept row-major, with the east edge joined to the west when Wrap is set.
// It is the whole of what a position needs to be turned into a tile, and
// the land's Grid is one with the ground laid over it.
type Map struct {
	W, H int
	Wrap bool
}

// In reports whether p is on the map. On a globe every column is; only a
// row past a pole is off it.
func (m *Map) In(p Pos) bool {
	if p.Y < 0 || p.Y >= m.H {
		return false
	}
	return m.Wrap || (p.X >= 0 && p.X < m.W)
}

// WrapX brings a column onto the map by going round it.
func (m *Map) WrapX(x int) int {
	if x >= 0 && x < m.W {
		return x
	}
	x %= m.W
	if x < 0 {
		x += m.W
	}
	return x
}

// Norm is p as the map holds it: the same column gone round the map to
// where it is stored. Rows are never moved; a row off the map is off it.
func (m *Map) Norm(p Pos) Pos {
	if m.Wrap {
		p.X = m.WrapX(p.X)
	}
	return p
}

// Index is where the tile at p is kept.
func (m *Map) Index(p Pos) int {
	if m.Wrap {
		return p.Y*m.W + m.WrapX(p.X)
	}
	return p.Y*m.W + p.X
}

// PosOf is the position of the tile kept at i.
func (m *Map) PosOf(i int) Pos {
	return Pos{X: i % m.W, Y: i / m.W}
}

// Dist is the Chebyshev distance between two positions, going round the
// map where that is shorter.
func (m *Map) Dist(a, b Pos) int {
	dx, dy := a.X-b.X, a.Y-b.Y
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if m.Wrap {
		dx %= m.W
		if back := m.W - dx; back < dx {
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
func (m *Map) Delta(from, to Pos) Pos {
	d := Pos{X: to.X - from.X, Y: to.Y - from.Y}
	if m.Wrap {
		d.X %= m.W
		if d.X > m.W/2 {
			d.X -= m.W
		} else if d.X < -m.W/2 {
			d.X += m.W
		}
	}
	return d
}

// Toward is from moved one tile straight toward to, the short way round.
func (m *Map) Toward(from, to Pos) Pos {
	d := m.Delta(from, to)
	return m.Norm(Pos{X: from.X + sign(d.X), Y: from.Y + sign(d.Y)})
}

// Columns is the columns within radius of x, as offsets from it: clipped to
// the map on a valley, and on a globe the whole way round at most, so that
// a window wider than the map reads each column once.
func (m *Map) Columns(x, radius int) (lo, hi int) {
	if !m.Wrap {
		return max(0, x-radius) - x, min(m.W-1, x+radius) - x
	}
	if 2*radius+1 >= m.W {
		half := m.W / 2
		return -half, m.W - 1 - half
	}
	return -radius, radius
}
