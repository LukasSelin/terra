package terra

import "math"

// The first continents.
//
// They were the first plates': each plate was drawn ocean or continent by a
// coin, and its crust was all of the one kind. A continent was then a plate,
// and its coast the plate's wall - a wall the fractures drew straight (see
// fractureWall) - carried whole across the world for the length of the
// history. On the first globe the continent in the middle of the map was
// one plate edge for edge, and a coast read as what it was: the outline of
// a piece of a drawing.
//
// The earth's are not. Its plates carry both kinds: the South American
// plate is a continent and half the South Atlantic, and most of the earth's
// coasts are the quiet margins inside a plate, where a continent rifted and
// the floor that opened stayed welded to it (Bird 2003). How much of a
// planet is continent is how much light rock its mantle has melted out of
// it, and where that rock lies is what the oceans' opening and closing have
// done with it before the history takes it up - neither is a thing a plate
// is drawn with.
//
// So the first crust's kind is laid off a field of the planet's own,
// independent of where its plates break: a continent is where the field
// stands highest, as much of it as the planet has continent, and the plates
// are drawn over it and carry whatever lies under them. The sea is not told
// where the coast is either. It is poured, as it always was, onto whatever
// height the crust floats at, so a coast is where the water runs out against
// the continent's thinned margin and not a line anybody drew.
//
// The field is octaves of the same lattice the rest of the history draws
// from, from continentSpan down to a tile, each continentRough the weight of
// the one twice as wide: a coast rough at every scale, as Mandelbrot's
// Britain is, and dominated by its largest bends.
const (
	// continentSpan is how wide the field's broadest swell is, in metres: a
	// continent's width, which on the earth is three to eight thousand
	// kilometres.
	continentSpan = 6000 * km
	// continentRough is how much each octave of the field weighs against the
	// one twice its width: two to the minus 0.8, a coast of fractal
	// dimension about 1.2, which is the west coast of Britain's (Mandelbrot
	// 1967) and a little rougher than South Africa's.
	continentRough = 0.57
	// continentLeastArea is the smallest piece of continent the field may leave
	// on its own in the ocean, and seaLeastArea the smallest piece of floor it
	// may leave inside a continent, in square metres. A thresholded field
	// leaves crumbs of both, and a crumb of continent out in the floor is an
	// island with no cause: it is taken back into the floor, and a pond of
	// floor in a continent into the continent. Microcontinents the size of
	// Madagascar - 600 by 300 km - and inland floors the size of the Black
	// Sea's basin are what is left.
	continentLeastArea = 400 * km * 400 * km
	seaLeastArea       = 600 * km * 600 * km
)

// firstContinents says of every tile of g whether its first crust is ocean
// floor: ocean of the world's tiles are, laid where the continent field
// stands lowest.
func (w *Land) firstContinents(g *Grid, ocean float64) []bool {
	n := len(g.Tiles)
	field := make([]float64, n)
	top := math.Min(float64(g.Span())/2, tilesAcross(continentSpan, g.span()))
	weight := 1.0
	for span := top; span >= 1; span /= 2 {
		o := w.lattice(g, span)
		for i := range field {
			field[i] += weight * o[i]
		}
		weight *= continentRough
	}
	level := quantile(append([]float64(nil), field...), clamp01(ocean))
	floor := make([]bool, n)
	for i, f := range field {
		floor[i] = f < level
	}
	tile := g.span() * g.span()
	g.fillPieces(floor, false, continentLeastArea/tile)
	g.fillPieces(floor, true, seaLeastArea/tile)
	return floor
}

// fillPieces turns every piece of tiles whose kind is kind, joined across
// their edges and corners, that is smaller than least tiles to the other
// kind.
func (g *Grid) fillPieces(of []bool, kind bool, least float64) {
	seen := make([]bool, len(of))
	var piece, stack []int
	for s := range of {
		if of[s] != kind || seen[s] {
			continue
		}
		piece, stack = piece[:0], append(stack[:0], s)
		seen[s] = true
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			piece = append(piece, i)
			g.eachNear(i, func(j int) {
				if of[j] == kind && !seen[j] {
					seen[j] = true
					stack = append(stack, j)
				}
			})
		}
		if float64(len(piece)) < least {
			for _, i := range piece {
				of[i] = !kind
			}
		}
	}
}
