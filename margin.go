package terra

import "math"

// The continents' edges, as the sea finds them.
//
// The coast of the earth is a contour. It is where the sea happens to stand
// on ground that was rough before the sea came to it: the continental crust
// thins over a margin a hundred to three hundred kilometres wide and slopes
// gently under the water, and the sea goes up the valleys in that slope and
// leaves the ridges standing as headlands and the hills as islands. A contour
// of ground that is rough in the same way at every scale, as the earth's is
// with a Hurst exponent of about 0.7, is itself rough at every scale, with a
// dimension of 2 - H, which is the 1.2 to 1.3 Richardson walked off the
// earth's coasts (Mandelbrot 1975, 1982).
//
// A history's continents had neither. Their crust ends where the plate that
// carries it was drawn, its thickness spread over a few tiles (layCrust), and
// the ground on it is the history's weathering of what its seams raised: so
// the margin falls a kilometre over two tiles from the continent to the
// floor, the ground on either side is smooth at the scale of a coast, and
// whatever level the sea stands at, the coast is the edge of the crust, a
// plate's outline run along at a fixed distance in.
//
// So when a history is over, its continents are given a roughness: a
// fractional Brownian relief, octaves from marginLongest down to
// marginShortest, each marginHurst of the one above it in amplitude for being
// half its size, marginRough metres of spread over them all. It is laid in
// full at the edge of the crust and under the sea, and less the further a
// tile is in from the edge (by e over marginInland) and the higher it stands
// (by e over marginHigh), so that a continent's interior and its ranges keep
// the heights the history gave them and the sea, wherever it stands on the
// margin, finds bays and headlands and islands.
//
// It is rougher than the earth's ground, whose Hurst exponent is about 0.7,
// because it does not stand alone: it lies over the history's own ground,
// which is smooth at the scale of a coast and slopes, and a contour across a
// slope is pinned by it. At 0.7 the coasts of the first and seventh globes
// read 1.15 against 1.12 with none; at 0.3 and 550 m, 1.17, a quarter of a
// continent's crust under the sea as the earth's is, and no lower: a margin
// lowered toward the crust's edge, 450 m at it and none 250 km in, ran the
// coast along the edge again, by the slope it added.
//
// The ranges along a coast pin it too. A coast at the foot of a range a
// tile or two from the edge of the crust is the range's outline, straight as
// the range is, and the history raises its ranges along its plates' edges;
// that is the coasts of the first globe, which read 1.12 to 1.14 whatever
// this is set to.
const (
	marginRough    = 550.0
	marginHurst    = 0.3
	marginLongest  = 2400 * km
	marginShortest = 37.5 * km
	marginInland   = 600 * km
	marginHigh     = 800.0
)

// roughMargins gives a finished history's continents their roughness: see
// above. g.base is the history's sea, and ocean which tiles
// are ocean crust; the floor is left as it is.
func (w *Land) roughMargins(g *Grid, ocean []bool) {
	n := len(g.Tiles)
	span := g.span()
	away, _ := g.nearestTo(func(i int) bool { return ocean[i] }, false)
	rough := make([]float64, n)
	amp, norm := marginRough, 0.0
	for long := marginLongest; long >= marginShortest*0.99; long /= 2 {
		f := w.lattice(g, math.Max(1.5, long/span))
		for i := range rough {
			rough[i] += amp * 2 * (f[i] - 0.5)
		}
		norm += amp * amp
		amp *= math.Pow(0.5, marginHurst)
	}
	// Value noise on a lattice stands at a third of its amplitude in its
	// spread; held to marginRough over all the octaves together.
	scale := marginRough / math.Sqrt(norm/3)
	inland := marginInland / span
	for i := range g.Tiles {
		if ocean[i] {
			continue
		}
		d := math.Max(0, away[i]-0.5)
		over := math.Max(0, g.Height[i]-g.base)
		by := scale * rough[i] * math.Exp(-d/inland) * math.Exp(-over/marginHigh)
		g.Height[i] += by
		g.strata[i].lift(by)
	}
}
