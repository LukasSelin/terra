// Package geom is tile arithmetic: where a thing is on a map, and how far
// that is from somewhere else.
//
// It is the bottom of the stack and depends on nothing. The land above it
// is made of these, and so is whatever walks about on the land — which is
// the point of its being its own package rather than a corner of the one
// that defines people. A map, a route across it and the weather over it are
// all the same to a game with settlers on it and a game with one player on
// it, and none of them should have to know what an agent is to say where
// something stands.
//
// A Pos is a raw coordinate. It carries no notion of the edges of a map or
// of a world that wraps round; both of those are the map's business, so the
// questions that depend on them — the distance between two places on a
// cylinder, whether a position is on the map at all — are asked of the grid
// rather than answered here. Dist is the plain answer, for a map with edges
// and for everything measuring within sight of itself.
package geom

// Pos is a tile coordinate on a world grid.
type Pos struct{ X, Y int }

// Dist is the Chebyshev distance: the number of eight-directional steps
// needed to walk from a to b.
func Dist(a, b Pos) int {
	dx, dy := a.X-b.X, a.Y-b.Y
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

// StepToward returns from moved one tile toward to.
func StepToward(from, to Pos) Pos {
	return Pos{X: from.X + sign(to.X-from.X), Y: from.Y + sign(to.Y-from.Y)}
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
