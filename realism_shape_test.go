package terra

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/LukasSelin/terra/internal/kernel"
)

// The shape of the world held against the earth's.
//
// A world can answer to every yardstick in realism_test.go and still not look
// like the earth. Those read how much of a thing there is and how it goes with
// latitude - how high the land stands, how deep the floor lies, how wet each
// belt is - and a map can have the earth's share of everything and lay it out
// as nothing on the earth is laid out: continents cut with a ruler, every
// coast in the same pale halo of shelf, the same hills from one shore to the
// other. These read the layout instead. How the land is cut up into
// continents and islands and how rough its coasts run; how the shelf lies
// along them; how the relief is gathered across the land. Each is a figure
// somebody measured on the earth, and each is read off a world the way it was
// read off the earth, at a planet's scale - a globe's tile is deepSpan, 37.5
// km, which is where a continent's shape is decided - or with no scale in it
// at all.
//
// They read the three full globes TestThePolarSeaIsIce makes, and not the one
// the climate's yardsticks read: a globe has a handful of continents, and
// what one globe's handful say about the shape of continents is mostly which
// globe it was. The readings are pooled over the three, except the land's
// share, where the furthest of the three from the earth's is what is held.
//
// Every measure here is read first off drawn shapes whose answers are known
// - a disc, a square, a Koch island, a fractional Brownian relief, a cascade
// - in TestTheShapeMeasuresReadDrawnShapes, so that a reading that moves is
// the world moving and not the instrument.
func init() {
	realYardsticks = append(realYardsticks, shapeYardsticks...)
}

// shapeGlobes is how many full globes the shape of the world is read over.
const shapeGlobes = 3

func threeGlobes() []*Grid {
	var gs []*Grid
	for seed := uint64(1); seed <= shapeGlobes; seed++ {
		gs = append(gs, yardWorld("globe", seed, GlobeTerms()))
	}
	return gs
}

// earthLand is the share of the earth's surface that is land: 148.9 of 510.1
// million square kilometres.
const earthLand = 148.9 / 510.1

var shapeYardsticks = []realYardstick{
	// 13. How the land is cut up. The earth's land is in a few great pieces
	// and a scatter of islands, the pieces neither round nor ruled, and the
	// coasts round all of them rough at every scale they have been measured at.
	{yardstick: yardstick{
		name: "land share of the surface, furthest of three globes", unit: "", scale: "ground", lo: 0.2, hi: 0.4, slow: true,
		source: "the earth's land is 29.2% of its surface; its continental crust, shelves and all, is about four tenths of it (Taylor & McLennan 1995), so a sea that drowned none of it would leave 0.4 land; the floor is not a measured figure. These three globes come out in the band by the luck of their first plates' draw, 0.33, 0.38 and 0.37: a globe's land is its continental crust, 97 to 99 hundredths of it above the sea, and the crust is drawn at 45% continent and put right only past crustSlack, counting tiles and not the sphere's area",
		measure: func() float64 {
			far := math.NaN()
			for _, g := range threeGlobes() {
				s := landOf(g).share
				if math.IsNaN(far) || math.Abs(s-earthLand) > math.Abs(far-earthLand) {
					far = s
				}
			}
			return far
		},
	}},
	{yardstick: yardstick{
		name: "remoteness of the continents, three globes", unit: "", scale: "ground", lo: 0.4, hi: 0.7, slow: true,
		source:  "Garcia-Castellanos & Lombardo 2007: the continental poles of inaccessibility lie 2510 km from the sea in Eurasia, 1814 Africa, 1650 North America, 1504 South America, 920 Australia - 0.58-0.65 of the radius of a disc of each one's area; 0.48 and 0.45 for Afro-Eurasia and the Americas, joined at Suez and Panama as a map at this scale joins them. A disc reads 1, a square 0.89",
		measure: func() float64 { return quantile(remoteness(threeGlobes()), 0.5) },
	}},
	{yardstick: yardstick{
		name: "island size exponent, three globes", unit: "", scale: "ground", lo: 0.45, hi: 0.85, slow: true,
		source:  "Korcak 1938; Mandelbrot 1982: N(A>=a) ~ a^-B over the world's islands, B 0.65 on the mean, 0.5 for Africa's to 0.75 for Indonesia's; 0.65 again over 131,063 islands from 1 to 10^5 km2 (ASTER GDEM, arXiv 2512.16659). This reading takes islands from two tiles to 3% of the land",
		measure: func() float64 { return islandExponent(threeGlobes()) },
	}},
	{yardstick: yardstick{
		name: "coast dimension, three globes", unit: "", scale: "ground", lo: 1.1, hi: 1.3, slow: true,
		source:  "Richardson 1961, read by Mandelbrot 1967: walked with dividers from tens to thousands of km, the west coast of Britain is 1.25, Australia's 1.13, South Africa's 1.02, the smoothest he had; the band is a world's coasts together, between Australia's and Britain's and widened a little each way, not a measured figure. Read at openings of 2 to 32 tiles, 75 to 1200 km",
		measure: func() float64 { return coastDimension(threeGlobes()) },
	}},
	{yardstick: yardstick{
		name: "grid lock of the coasts, three globes", unit: "", scale: "ground", lo: 0, hi: 0.1, slow: true,
		source:  "a coast does not know which way the survey's grid runs: how far the directions of the coast lean to the map's axes or its diagonals, the larger of |<cos 4θ>| and |<cos 8θ>|, is nothing on the earth's own ground. A drawn square reads 0.99; fractional Brownian coasts drawn on this map, up to 0.07",
		measure: func() float64 { return gridLock(threeGlobes(), landEdge) },
	}},
	{yardstick: yardstick{
		name: "right angles of the continents' coasts, three globes", unit: "", scale: "ground", lo: 0, hi: 0.15, slow: true,
		source:  "how strongly a continent's coasts gather at four bearings a right angle apart, whichever way it is turned. No figure for the earth's has been read this way; a Brownian relief's coasts read 0.02-0.07, a hexagon's 0.02, the cells of a Voronoi 0.45 and a square 1, and the band is not a measured figure: the first globes' continents were cut in rectangles at 0.18",
		measure: func() float64 { return cornerLock(threeGlobes()) },
	},
		gap: "known gap: K - since the history's ground comes down by its relief (denude.go) it keeps two to two and a half times the land over its sea, and the continents' outlines are its margins, which gather at right angles a shade more than the band, itself not a measured figure: 0.153 on G2b, 0.153-0.161 over its builds, 0.156 with the country the history's own height",
	},

	// 14. The shelf. Where the land meets the sea the floor runs out shallow
	// for a while before it falls away, and how far it runs is the margin's own
	// story: narrow where a plate goes down off the coast and the ground is
	// young and shaken, wide where the margin has been quiet and filling with
	// the land's mud since it rifted.
	{yardstick: yardstick{
		name: "sea floor within 200 m of the sea, three globes", unit: "", scale: "ground", lo: 0.05, hi: 0.11, slow: true,
		source:  "GEBCO 2019 (NOAA 2020): 7.3% of the sea floor lies within 200 m of the surface; Harris et al. 2014: the shelf out to its break, 8.9% of the ocean",
		measure: func() float64 { return shelvesOf(threeGlobes()).share },
	}},
	{yardstick: yardstick{
		name: "mean shelf width, three globes", unit: "km", scale: "ground", lo: 35, hi: 110, slow: true,
		source:  "Harris et al. 2014 (Geomorphology of the oceans): shelves are 57 km wide on the mean over all the oceans, 37 in the Indian Ocean to 110 in the South Pacific. Read here from each coast to the nearest floor deeper than 200 m",
		measure: func() float64 { return shelvesOf(threeGlobes()).mean },
	},
		gap: "known gap: K - a shelf is laid a tile and a half wide at the least, 56 km, and a quiet margin's is 88, so the first floor under 200 m is two tiles out of an active coast and three out of a quiet one; and an eighth of the coasts read are shores of hollows in the continents below the sea, with no deep floor anywhere in them, 1000 km and more: 276 km; 223, 251 and 332 globe by globe",
	},
	{yardstick: yardstick{
		name: "shelf width, quiet margins over active, three globes", unit: "x", scale: "ground", lo: 1.8, hi: 4.5, slow: true,
		source:  "Harris et al. 2014: shelves are 88.2 km wide on passive margins and 31 on active ones, 2.85 times; the band is not a measured figure. A margin is read as active where a plate boundary runs within 150 km of its coast",
		measure: func() float64 { s := shelvesOf(threeGlobes()); return s.quiet / s.active },
	},
		// It read 1.88 until the fractures bent by the plate moved every coast,
		// then 1.24 (2.00, 0.94 and 1.23 globe by globe), and 1.37 on the rift's
		// halves (#98). With the sea standing where its water fills the basins
		// (seawater.go) it read 1.85, inside the band by a hair, and the marker
		// came off. With the basins subsiding and filling with what arrives
		// (subside.go, #45) it fell out again: 1.13 to 1.67 over five builds of
		// that change, three of which lay their shelves alike and differ only in
		// whether the rifts and the margins carry their heat and the plates break.
		gap: "known gap: K - an active shelf is laid no narrower than a tile and a half, 56 km, against a quiet one's 88, and most coasts with a seam within 150 km are quiet margins as laid, so the active column reads mostly quiet shelves: 1.13 to 1.85 over builds that lay their shelves alike",
	},
	{yardstick: yardstick{
		name: "grid lock of the sea floor off the coasts, three globes", unit: "", scale: "ground", lo: 0, hi: 0.05, slow: true,
		source:  "the floor's slope within 300 km of a coast leans no more to the map's axes and diagonals than the coast does to the earth's lines of latitude; fractional Brownian relief drawn on this map reads within 0.025 of nothing. The band is not a measured figure",
		measure: func() float64 { return gridLock(threeGlobes(), offshoreFloor) },
	}},

	// 15. The relief. The earth's heights are rough in the same way at every
	// scale from a hillside to a hemisphere, and that roughness is gathered:
	// the steep ground is a few long ranges with wide plains between, not a
	// hill on every tile.
	{yardstick: yardstick{
		name: "land relief spectral exponent, three globes", unit: "", scale: "ground", lo: 1.8, hi: 2.4, slow: true,
		source:  "Gagnon, Lovejoy & Schertzer 2006 (Multifractal earth topography): P(k) ~ k^-beta with beta = 1 + 2H - K(2) = 2.1 on the continents (H 0.66, C1 0.12, alpha 1.79), 2.3 on their margins, from 40 m to planetary scales. Read along the rows and columns of windows of 64 tiles wholly on land",
		measure: func() float64 { return reliefSpectrum(threeGlobes()) },
	}},
	// It was a known gap, K, at 0.075: the history left its relief gathered,
	// 0.175 as the shaping took it up, and the shaping laid it again as one
	// hillslope on every tile off the rivers. With the history's ground
	// floating on its crust (isostasy.go) the uplift the shaping grades by is
	// the rock's, rebound and all, and it reads 0.091. It opened again when
	// the history's ground came down by its relief (denude.go).
	{yardstick: yardstick{
		name: "land relief intermittency C1, three globes", unit: "", scale: "ground", lo: 0.08, hi: 0.18, slow: true,
		source: "Gagnon, Lovejoy & Schertzer 2006: the earth's relief is a multifractal of C1 0.12 and alpha 1.79, how sparsely its roughness is gathered. Read by trace moments of the gradient in windows of 32 tiles wholly on land; a relief rough everywhere alike, a fractional Brownian one, reads 0.037 on this reading, and the band's width is not a measured figure",
		seeded: &seeded{worlds: eachOf(threeGlobes), read: pooled(intermittencyC1)},
	},
		// It was a known gap (K: the history leaves its relief gathered, 0.175 as
		// the shaping takes it up, and the shaping lays it again as one hillslope
		// on every tile off the rivers; 0.075) until the air came to swing the
		// energy balance's year (see atmos.Env.seasonTemp); it read 0.087 then,
		// near the floor, and nothing in the shaping changed. That was before the
		// rock stack; on it the gap is main's.
		gap: "known gap: K - since the history's ground comes down by its relief (denude.go) its interiors are plateaus behind escarpments, and the shaping lays the map's ground again from their order alone: the country (hypsometry.go) is the history's height and Height is not, so it does not reach this. 0.091 on G2, 0.052-0.057 on four builds since, its three globes 0.040-0.064 a globe: 0.052",
	},
}

// rowWeight is how much of a sphere's surface row y of g stands for, against
// a row on the equator.
func rowWeight(g *Grid, y int) float64 { return math.Cos(latitudeOf(g, y) * math.Pi / 180) }

// landmasses is a world's land cut into the pieces it stands in: what is
// above the sea, joined across a corner as well as a side.
type landmasses struct {
	of    []int32   // the piece each tile is in, -1 under the sea
	tiles []float64 // how many tiles each piece has
	polar []bool    // whether a piece reaches the first or last row
	land  float64   // tiles of land
	share float64   // of the surface on a sphere
}

func landOf(g *Grid) landmasses {
	return remember("landmasses", []*Grid{g}, func() landmasses {
		n := len(g.Tiles)
		r := landmasses{of: make([]int32, n)}
		for i := range r.of {
			r.of[i] = -1
		}
		var surface, dry float64
		var stack []int
		for i := range g.Tiles {
			w := rowWeight(g, i/g.W)
			surface += w
			if g.underSea(i) {
				continue
			}
			dry += w
			if r.of[i] >= 0 {
				continue
			}
			id := int32(len(r.tiles))
			r.tiles, r.polar = append(r.tiles, 0), append(r.polar, false)
			r.of[i] = id
			stack = append(stack[:0], i)
			for len(stack) > 0 {
				j := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				r.tiles[id]++
				if y := j / g.W; y == 0 || y == g.H-1 {
					r.polar[id] = true
				}
				g.eachNear(j, func(k int) {
					if r.of[k] < 0 && !g.underSea(k) {
						r.of[k] = id
						stack = append(stack, k)
					}
				})
			}
			r.land += r.tiles[id]
		}
		r.share = dry / surface
		return r
	})
}

// continentLeast is the least share of a world's land a piece must hold to be
// read as a continent. The earth's smallest, Australia, is 5.2% of its land
// and its largest island, Greenland, 1.5%.
const continentLeast = 0.03

// remoteness is, for every continent of the worlds, how far its remotest
// ground lies from the sea over the radius of a disc of its area: 1 for a
// disc, 0.89 for a square, less the more it is cut into by the sea. A piece
// that reaches the first or last row is not read: on a cylinder its far side
// is a pole and not a coast.
func remoteness(gs []*Grid) []float64 {
	var out []float64
	for _, g := range gs {
		lm := landOf(g)
		away := g.awayFrom(g.underSea)
		far := make([]float64, len(lm.tiles))
		for i, c := range lm.of {
			if c >= 0 {
				// From the middle of the tile to the line between it and the sea.
				far[c] = math.Max(far[c], away[i]-0.5)
			}
		}
		for c, t := range lm.tiles {
			if t >= continentLeast*lm.land && !lm.polar[c] {
				out = append(out, far[c]/math.Sqrt(t/math.Pi))
			}
		}
	}
	if len(out) == 0 {
		return []float64{math.NaN()}
	}
	return out
}

// islandExponent is Korcak's B, -d ln N(A>=a) / d ln a, over the islands of
// the worlds pooled: every piece of land from two tiles to continentLeast of
// its world's land.
func islandExponent(gs []*Grid) float64 {
	var a []float64
	for _, g := range gs {
		lm := landOf(g)
		for _, t := range lm.tiles {
			if t >= 2 && t < continentLeast*lm.land {
				a = append(a, t)
			}
		}
	}
	if len(a) < 20 {
		return math.NaN()
	}
	return exceedanceBetween(a, 2, slices.Max(a))
}

// coastlines are the lines between the land and the sea as marching squares
// draws them through the middles of the tiles: each a run of points in tiles,
// carried on round the seam rather than jumping back across the map. A line
// that comes back to its start ends on it; one that runs off the first or
// last row ends there. Land that touches land at a corner is one piece, as
// landOf reads it.
func coastlines(g *Grid, land func(i int) bool) [][][2]float64 {
	W, H := g.W, g.H
	cols := W
	if !g.Wrap {
		cols = W - 1
	}
	// An edge between two tiles' middles is its own number: along a row from
	// x, or down a column from y.
	along := func(x, y int) int { return 2 * (y*W + x) }
	down := func(x, y int) int { return 2*(y*W+x) + 1 }
	at := func(e int) [2]float64 {
		x, y := float64(e/2%W), float64(e/2/W)
		if e%2 == 0 {
			return [2]float64{x + 0.5, y}
		}
		return [2]float64{x, y + 0.5}
	}
	link := map[int][]int{}
	join := func(a, b int) {
		link[a], link[b] = append(link[a], b), append(link[b], a)
	}
	for y := 0; y < H-1; y++ {
		for x := 0; x < cols; x++ {
			x1 := (x + 1) % W
			tl, tr := land(y*W+x), land(y*W+x1)
			bl, br := land((y+1)*W+x), land((y+1)*W+x1)
			top, bottom, left, right := along(x, y), along(x, y+1), down(x, y), down(x1, y)
			var cut []int
			if tl != tr {
				cut = append(cut, top)
			}
			if tr != br {
				cut = append(cut, right)
			}
			if br != bl {
				cut = append(cut, bottom)
			}
			if bl != tl {
				cut = append(cut, left)
			}
			switch {
			case len(cut) == 2:
				join(cut[0], cut[1])
			case len(cut) == 4 && tl:
				// Land on one diagonal and sea on the other: the land is joined
				// across, so each corner of sea is cut off on its own.
				join(top, right)
				join(bottom, left)
			case len(cut) == 4:
				join(top, left)
				join(right, bottom)
			}
		}
	}
	unwrap := func(p, last [2]float64) [2]float64 {
		for p[0]-last[0] > float64(W)/2 {
			p[0] -= float64(W)
		}
		for p[0]-last[0] < -float64(W)/2 {
			p[0] += float64(W)
		}
		return p
	}
	seen := map[int]bool{}
	var lines [][][2]float64
	walk := func(start int) {
		pts := [][2]float64{at(start)}
		seen[start] = true
		prev, e := -1, start
		for {
			next := -1
			for _, n := range link[e] {
				if n != prev && !seen[n] {
					next = n
					break
				}
			}
			if next < 0 {
				if len(pts) > 2 && slices.Contains(link[e], start) {
					pts = append(pts, unwrap(at(start), pts[len(pts)-1]))
				}
				break
			}
			pts = append(pts, unwrap(at(next), pts[len(pts)-1]))
			seen[next] = true
			prev, e = e, next
		}
		lines = append(lines, pts)
	}
	edges := make([]int, 0, len(link))
	for e := range link {
		edges = append(edges, e)
	}
	slices.Sort(edges)
	// The lines that run off the map first, from an end, and then the rings.
	for _, e := range edges {
		if len(link[e]) == 1 && !seen[e] {
			walk(e)
		}
	}
	for _, e := range edges {
		if !seen[e] {
			walk(e)
		}
	}
	return lines
}

func lineLength(pts [][2]float64) float64 {
	s := 0.0
	for k := 1; k < len(pts); k++ {
		s += math.Hypot(pts[k][0]-pts[k-1][0], pts[k][1]-pts[k-1][1])
	}
	return s
}

// dividerLength is the length of a line walked with dividers opened to r, as
// Richardson walked his coasts: each step to the first point further along
// the line that lies r from where the last step came down, and what is left
// at the end counted as a part of a step.
func dividerLength(pts [][2]float64, r float64) float64 {
	c := pts[0]
	seg, t, steps := 0, 0.0, 0.0
	for {
		found := false
		for s := seg; s < len(pts)-1 && !found; s++ {
			a, b := pts[s], pts[s+1]
			dx, dy := b[0]-a[0], b[1]-a[1]
			fx, fy := a[0]-c[0], a[1]-c[1]
			A, B, C := dx*dx+dy*dy, 2*(fx*dx+fy*dy), fx*fx+fy*fy-r*r
			disc := B*B - 4*A*C
			if A == 0 || disc < 0 {
				continue
			}
			from := 0.0
			if s == seg {
				from = t
			}
			sq := math.Sqrt(disc)
			for _, u := range [2]float64{(-B - sq) / (2 * A), (-B + sq) / (2 * A)} {
				if u > from+1e-9 && u <= 1 {
					c, seg, t, found = [2]float64{a[0] + u*dx, a[1] + u*dy}, s, u, true
					break
				}
			}
		}
		if !found {
			end := pts[len(pts)-1]
			return (steps + math.Hypot(end[0]-c[0], end[1]-c[1])/r) * r
		}
		steps++
	}
}

// dividerOpenings are the openings, in tiles, the coasts are walked at.
var dividerOpenings = []float64{2, 4, 8, 16, 32}

// coastDimension is Richardson's dimension of the worlds' coasts: one more
// than how fast their length falls as the dividers open, -d ln L / d ln r,
// the lengths of every coast long enough to be walked at the widest opening
// eight times over summed at each opening.
func coastDimension(gs []*Grid) float64 {
	widest := dividerOpenings[len(dividerOpenings)-1]
	sum := make([]float64, len(dividerOpenings))
	for _, g := range gs {
		for _, l := range coastlines(g, func(i int) bool { return !g.underSea(i) }) {
			if lineLength(l) < 8*widest {
				continue
			}
			for k, r := range dividerOpenings {
				sum[k] += dividerLength(l, r)
			}
		}
	}
	var xs, ys []float64
	for k, r := range dividerOpenings {
		if sum[k] == 0 {
			return math.NaN()
		}
		xs, ys = append(xs, math.Log(r)), append(ys, math.Log(sum[k]))
	}
	return 1 - fit(xs, ys)
}

// lockLatitude is how far from the equator grid lock is read: past it a
// cylinder's rows stand for less and less of a sphere, and what is drawn
// there is stretched along them.
const lockLatitude = 60.0

// gridLock is how far the directions a field slopes in lean to the map's axes
// or its diagonals: the field eased over a tile and a half, and the direction
// θ of its gradient on every tile it is read on, weighted by how steep the
// gradient is there. A field that does not know the grid has <cos 4θ> and
// <cos 8θ> of nothing; one drawn in squares has <cos 4θ> of 1, in squares
// turned to the diagonal -1, and one drawn in both, as octagons are, <cos 8θ>
// of 1. The reading is the larger of the two in size, over the worlds
// together, within lockLatitude of the equator.
//
// A gradient under lockLeast is not read: the flat either side of an edge,
// or a floor that does not slope, has no direction worth the name.
func gridLock(gs []*Grid, field lockField) float64 {
	const lockLeast = 0.05
	var sw, s4, s8 float64
	for _, g := range gs {
		f, at := field(g)
		f = blur(g, f, 1.5)
		for y := 1; y < g.H-1; y++ {
			if math.Abs(latitudeOf(g, y)) > lockLatitude {
				continue
			}
			for x := 0; x < g.W; x++ {
				i := y*g.W + x
				if (!g.Wrap && (x == 0 || x == g.W-1)) || (at != nil && !at(i)) {
					continue
				}
				gx := (f[y*g.W+g.WrapX(x+1)] - f[y*g.W+g.WrapX(x-1)]) / 2
				gy := (f[i+g.W] - f[i-g.W]) / 2
				if m := math.Hypot(gx, gy); m >= lockLeast {
					th := math.Atan2(gy, gx)
					sw, s4, s8 = sw+m, s4+m*math.Cos(4*th), s8+m*math.Cos(8*th)
				}
			}
		}
	}
	if sw == 0 {
		return math.NaN()
	}
	return math.Max(math.Abs(s4/sw), math.Abs(s8/sw))
}

// cornerLock is how strongly the continents' coasts gather at four bearings a
// right angle apart, whichever way each continent is turned: |<e^{4iθ}>| of
// the directions its eased edge faces, weighted by the edge, averaged over
// the continents by their size. Where gridLock asks whether the coasts know
// which way the map runs, this asks whether a continent is a rectangle. A
// square reads 1 and a rectangle near enough as much, whichever way it is
// turned; a hexagon or a triangle 0.02, its corners being sixty degrees; a
// disc 0.01; the cells of a Voronoi 0.45; and the coasts of a Brownian relief
// 0.02 to 0.07, which is how far from nothing a continent's few long runs of
// rough coast come by chance. It is read, as gridLock is, within
// lockLatitude of the equator.
func cornerLock(gs []*Grid) float64 {
	const least = 0.05
	var sum, area float64
	for _, g := range gs {
		lm := landOf(g)
		for c, n := range lm.tiles {
			if n < continentLeast*lm.land {
				continue
			}
			f := make([]float64, len(g.Tiles))
			for i, k := range lm.of {
				if k == int32(c) {
					f[i] = 1
				}
			}
			f = blur(g, f, 1.5)
			var re, im, w float64
			for y := 1; y < g.H-1; y++ {
				if math.Abs(latitudeOf(g, y)) > lockLatitude {
					continue
				}
				for x := 0; x < g.W; x++ {
					if !g.Wrap && (x == 0 || x == g.W-1) {
						continue
					}
					i := y*g.W + x
					gx := (f[y*g.W+g.WrapX(x+1)] - f[y*g.W+g.WrapX(x-1)]) / 2
					gy := (f[i+g.W] - f[i-g.W]) / 2
					if m := math.Hypot(gx, gy); m >= least {
						th := 4 * math.Atan2(gy, gx)
						re, im, w = re+m*math.Cos(th), im+m*math.Sin(th), w+m
					}
				}
			}
			if w > 0 {
				sum += n * math.Hypot(re, im) / w
				area += n
			}
		}
	}
	if area == 0 {
		return math.NaN()
	}
	return sum / area
}

// lockField is a field gridLock reads the slope of, and the tiles it reads it
// on, or nil for all of them.
type lockField func(g *Grid) (f []float64, at func(i int) bool)

// landEdge is the edge of the land: one on land and nothing under the sea.
// Eased, it slopes only along the coast.
func landEdge(g *Grid) ([]float64, func(i int) bool) {
	f := make([]float64, len(g.Tiles))
	for i := range f {
		if !g.underSea(i) {
			f[i] = 1
		}
	}
	return f, nil
}

// offshoreReach is how far off the coast the sea floor's grid lock is read:
// the shelf and the slope, and past them a little.
const offshoreReach = 300 * km

// offshoreFloor is the depth of the sea, read within offshoreReach of the
// land.
func offshoreFloor(g *Grid) ([]float64, func(i int) bool) {
	near := g.awayFrom(func(i int) bool { return !g.underSea(i) })
	reach := tilesAcross(offshoreReach, deepSpan(g))
	f := make([]float64, len(g.Tiles))
	for i := range f {
		f[i] = math.Max(0, g.sea-g.Height[i])
	}
	return f, func(i int) bool { return g.underSea(i) && near[i] <= reach }
}

// blur eases f with a Gaussian of sigma tiles, round the seam where the map
// wraps and held at its value at the edges.
func blur(g *Grid, f []float64, sigma float64) []float64 {
	r := int(math.Ceil(3 * sigma))
	k := make([]float64, 2*r+1)
	sum := 0.0
	for j := -r; j <= r; j++ {
		k[j+r] = math.Exp(-float64(j*j) / (2 * sigma * sigma))
		sum += k[j+r]
	}
	for j := range k {
		k[j] /= sum
	}
	across, out := make([]float64, len(f)), make([]float64, len(f))
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			v := 0.0
			for j := -r; j <= r; j++ {
				xx := x + j
				if g.Wrap {
					xx = g.WrapX(xx)
				} else {
					xx = max(0, min(g.W-1, xx))
				}
				v += k[j+r] * f[y*g.W+xx]
			}
			across[y*g.W+x] = v
		}
	}
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			v := 0.0
			for j := -r; j <= r; j++ {
				v += k[j+r] * across[max(0, min(g.H-1, y+j))*g.W+x]
			}
			out[y*g.W+x] = v
		}
	}
	return out
}

type shelfReading struct {
	share         float64 // of the sea floor, within shelfBreak of the sea
	mean          float64 // km, the mean width at the coast
	active, quiet float64 // km, the mean width at active and quiet margins
}

// activeReach is how near a coast a plate boundary has to run for its margin
// to be read as active: a trench lies a hundred kilometres or two off the
// coast it goes down under.
const activeReach = 150 * km

// shelvesOf reads the shelves of the worlds. A shelf's width at a coast is how
// far the coast's tile lies from the nearest floor deeper than shelfBreak,
// from its middle to its edge, at the planet's scale; a coast is every tile of
// land beside the sea, and a margin is active where a tile beside a tile of
// another plate lies within activeReach of it.
func shelvesOf(gs []*Grid) shelfReading {
	return remember("shelves", gs, func() shelfReading {
		var shallow, sea float64
		var all, active, quiet []float64
		for _, g := range gs {
			span := deepSpan(g)
			deep := g.awayFrom(func(i int) bool { return g.underSea(i) && g.sea-g.Height[i] >= shelfBreak })
			seam := g.awayFrom(func(i int) bool {
				on := false
				g.eachNear(i, func(j int) { on = on || g.Tiles[j].Plate != g.Tiles[i].Plate })
				return on
			})
			for i := range g.Tiles {
				if g.underSea(i) {
					w := rowWeight(g, i/g.W)
					sea += w
					if g.sea-g.Height[i] < shelfBreak {
						shallow += w
					}
					continue
				}
				coast := false
				g.eachNear(i, func(j int) { coast = coast || g.underSea(j) })
				if !coast {
					continue
				}
				width := math.Max(0, deep[i]-0.5) * span / km
				all = append(all, width)
				if seam[i]*span <= activeReach {
					active = append(active, width)
				} else {
					quiet = append(quiet, width)
				}
			}
		}
		return shelfReading{shallow / sea, meanOf(all), meanOf(active), meanOf(quiet)}
	})
}

// landWindows are the corners of the squares of side n wholly on dry land,
// laid on a lattice of half a square, within seventy degrees of the equator.
func landWindows(g *Grid, n int) [][2]int {
	var out [][2]int
	for y0 := 0; y0+n <= g.H; y0 += n / 2 {
		if math.Abs(latitudeOf(g, y0)) > 70 || math.Abs(latitudeOf(g, y0+n-1)) > 70 {
			continue
		}
	square:
		for x0 := 0; x0+n <= g.W; x0 += n / 2 {
			for y := y0; y < y0+n; y++ {
				for x := x0; x < x0+n; x++ {
					if g.underSea(y*g.W + x) {
						continue square
					}
				}
			}
			out = append(out, [2]int{x0, y0})
		}
	}
	return out
}

// reliefSpectrum is beta in P(k) ~ k^-beta of the land's heights, read along
// every row and column of the windows of 64 tiles wholly on land, each line
// levelled and tapered, the power summed over all of them and fitted from a
// whole window's wavelength to a quarter of it. Below that the valleys
// shape.go cuts at TileSpan stand above the line, as Perron found real
// valleys do (see valleyWavelength).
func reliefSpectrum(gs []*Grid) float64 {
	return remember("spectrum", gs, func() float64 {
		const n = 64
		power := make([]float64, n/4+1)
		hann := make([]float64, n)
		for k := range hann {
			hann[k] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(k)/float64(n-1))
		}
		line := make([]float64, n)
		for _, g := range gs {
			for _, w := range landWindows(g, n) {
				for across := 0; across < n; across++ {
					for _, rows := range []bool{true, false} {
						for j := range line {
							x, y := w[0]+j, w[1]+across
							if !rows {
								x, y = w[0]+across, w[1]+j
							}
							line[j] = g.Height[y*g.W+x]
						}
						level(line)
						for k := 1; k < len(power); k++ {
							var re, im float64
							for j := range line {
								a := -2 * math.Pi * float64(k*j) / n
								re += line[j] * hann[j] * math.Cos(a)
								im += line[j] * hann[j] * math.Sin(a)
							}
							power[k] += re*re + im*im
						}
					}
				}
			}
		}
		var xs, ys []float64
		for k := 1; k < len(power); k++ {
			if power[k] <= 0 {
				return math.NaN()
			}
			xs, ys = append(xs, math.Log(float64(k))), append(ys, math.Log(power[k]))
		}
		return -fit(xs, ys)
	})
}

// intermittencyWindow is the side of the squares of land the relief's
// intermittency is read in: big enough for five octaves of boxes, small
// enough that a globe has a hundred and more of them.
const intermittencyWindow = 32

// reliefIntermittency is C1 and alpha of the land's relief, as Gagnon and
// others read the earth's: the flux is the size of the gradient on every tile
// of a window wholly on land, each window taken over its own mean, and its
// moments read by traceMoments.
func reliefIntermittency(gs []*Grid) (c1, alpha float64) {
	type ca struct{ c1, alpha float64 }
	r := remember("intermittency", gs, func() ca {
		const n = intermittencyWindow
		var fields [][]float64
		for _, g := range gs {
			for _, w := range landWindows(g, n) {
				eps := make([]float64, n*n)
				for y := 0; y < n; y++ {
					for x := 0; x < n; x++ {
						// Differences inside the window: forward, and back at
						// its last row and column.
						i := (w[1]+y)*g.W + w[0] + x
						right, below := i+1, i+g.W
						if x == n-1 {
							right = i - 1
						}
						if y == n-1 {
							below = i - g.W
						}
						eps[y*n+x] = math.Hypot(g.Height[right]-g.Height[i], g.Height[below]-g.Height[i])
					}
				}
				fields = append(fields, eps)
			}
		}
		c, a := traceMoments(fields, n)
		return ca{c, a}
	})
	return r.c1, r.alpha
}

// traceMoments reads the universal multifractal parameters of fields n on a
// side (Schertzer and Lovejoy 1987). Each field is a flux taken over its own
// mean; averaged over boxes of every power of two up to the field, the mean of
// its q-th power goes as the number of boxes across to the K(q), and K is
// fitted in q at 0.9, 1.1 and 2. C1 is K's slope at 1, and alpha is what makes
// K(2)/C1 = (2^alpha - 2)/(alpha - 1). A field with nothing gathered in it,
// every box like every other, has C1 of nothing; the earth's relief has 0.12.
func traceMoments(fields [][]float64, n int) (c1, alpha float64) {
	qs := []float64{0.9, 1.1, 2}
	octaves := 0
	for 1<<octaves < n {
		octaves++
	}
	sum := make([][]float64, len(qs))
	for k := range sum {
		sum[k] = make([]float64, octaves+1)
	}
	count := make([]float64, octaves+1)
	for _, eps := range fields {
		mean := meanOf(eps)
		if mean == 0 {
			continue
		}
		for l := 0; l <= octaves; l++ {
			s := 1 << l
			for by := 0; by < n; by += s {
				for bx := 0; bx < n; bx += s {
					a := 0.0
					for y := by; y < by+s; y++ {
						for x := bx; x < bx+s; x++ {
							a += eps[y*n+x]
						}
					}
					a /= float64(s*s) * mean
					for k, q := range qs {
						sum[k][l] += math.Pow(a, q)
					}
					count[l]++
				}
			}
		}
	}
	if count[0] == 0 {
		return math.NaN(), math.NaN()
	}
	K := make([]float64, len(qs))
	for k := range qs {
		var xs, ys []float64
		for l := 0; l <= octaves; l++ {
			xs = append(xs, math.Log(float64(n)/float64(int(1)<<l)))
			ys = append(ys, math.Log(sum[k][l]/count[l]))
		}
		K[k] = fit(xs, ys)
	}
	c1 = (K[1] - K[0]) / (qs[1] - qs[0])
	shape := func(a float64) float64 {
		if math.Abs(a-1) < 1e-6 {
			return 2 * math.Ln2
		}
		return (math.Pow(2, a) - 2) / (a - 1)
	}
	lo, hi := 0.0, 2.0
	for range 60 {
		a := (lo + hi) / 2
		if shape(a) < K[2]/c1 {
			lo = a
		} else {
			hi = a
		}
	}
	return c1, (lo + hi) / 2
}

// The measures, read off drawn shapes whose answers are known. A disc is as
// remote from its sea as a shape of its area can be, and its coast is a line
// of dimension one with no direction to it; a square's corners bring the sea
// nearer, 0.886 of a disc's reach, and every edge of it lies along the grid.
// Koch's island has a coast of dimension log 4 / log 3 at every scale it is
// drawn to. A fractional Brownian relief of Hurst exponent H has a spectrum
// falling as k^-(1+2H) and nothing gathered in its roughness; a cascade of
// lognormal weights has C1 of sigma^2 / 2 ln 2.
func TestTheShapeMeasuresReadDrawnShapes(t *testing.T) {
	const n = 512
	mid := float64(n) / 2
	disc := drawnLand(n, n, func(x, y float64) bool { return math.Hypot(x-mid, y-mid) < 150 })
	square := drawnLand(n, n, func(x, y float64) bool { return math.Abs(x-mid) < 130 && math.Abs(y-mid) < 130 })
	diamond := drawnLand(n, n, func(x, y float64) bool { return math.Abs(x-mid)+math.Abs(y-mid) < 180 })
	kochIn := polygonFill(1024, 1024, kochIsland(512, 540, 760, 5))
	koch := drawnLand(1024, 1024, func(x, y float64) bool { return kochIn[int(y)*1024+int(x)] })

	near := func(what string, got, want, within float64) {
		t.Helper()
		if !(math.Abs(got-want) <= within) {
			t.Errorf("%s reads %.3f, want %.3f within %.3f", what, got, want, within)
		}
	}
	near("a disc's remoteness", remoteness([]*Grid{disc})[0], 1, 0.03)
	near("a square's remoteness", remoteness([]*Grid{square})[0], math.Sqrt(math.Pi)/2, 0.03)
	near("a disc's coast dimension", coastDimension([]*Grid{disc}), 1, 0.02)
	near("a square's coast dimension", coastDimension([]*Grid{square}), 1, 0.02)
	near("Koch's island's coast dimension", coastDimension([]*Grid{koch}), math.Log(4)/math.Log(3), 0.05)
	near("a disc's grid lock", gridLock([]*Grid{disc}, landEdge), 0, 0.03)
	near("a square's grid lock", gridLock([]*Grid{square}, landEdge), 1, 0.05)
	near("a diamond's grid lock", gridLock([]*Grid{diamond}, landEdge), 1, 0.05)
	hexIn := polygonFill(n, n, func() [][2]float64 {
		var p [][2]float64
		for k := range 6 {
			a := 0.3 + math.Pi*float64(k)/3
			p = append(p, [2]float64{mid + 160*math.Cos(a), mid + 160*math.Sin(a)})
		}
		return p
	}())
	hexagon := drawnLand(n, n, func(x, y float64) bool { return hexIn[int(y)*n+int(x)] })
	near("a square's right angles", cornerLock([]*Grid{square}), 1, 0.05)
	near("a diamond's right angles", cornerLock([]*Grid{diamond}), 1, 0.05)
	near("a hexagon's right angles", cornerLock([]*Grid{hexagon}), 0, 0.05)
	near("a disc's right angles", cornerLock([]*Grid{disc}), 0, 0.03)

	for _, hurst := range []float64{0.5, 0.8} {
		f := fbmField(1024, 512, hurst, 1)
		// Seven tenths land, for squares of it to read the relief in.
		g := drawnHeights(1024, 512, f, quantile(f, 0.3))
		near(fmt.Sprintf("the spectrum of a Brownian relief of H %.1f", hurst), reliefSpectrum([]*Grid{g}), 1+2*hurst, 0.2)
		if c1, _ := reliefIntermittency([]*Grid{g}); !(c1 < 0.06) {
			t.Errorf("a Brownian relief of H %.1f, rough everywhere alike, reads C1 %.3f: the reading "+
				"cannot tell it from the earth's 0.12", hurst, c1)
		}
		// Three tenths land, for coasts.
		g = drawnHeights(1024, 512, f, quantile(f, 0.7))
		if lock := gridLock([]*Grid{g}, landEdge); !(lock < 0.1) {
			t.Errorf("the coasts of a Brownian relief of H %.1f lean %.3f to the grid", hurst, lock)
		}
		if corners := cornerLock([]*Grid{g}); !(corners < 0.1) {
			t.Errorf("the continents of a Brownian relief of H %.1f gather %.3f at right angles", hurst, corners)
		}
	}

	r := rand.New(rand.NewPCG(9, 9))
	for _, sigma := range []float64{0.2, 0.4} {
		const side = 64
		var fields [][]float64
		for range 200 {
			eps := make([]float64, side*side)
			for i := range eps {
				eps[i] = 1
			}
			for s := side / 2; s >= 1; s /= 2 {
				for by := 0; by < side; by += s {
					for bx := 0; bx < side; bx += s {
						w := math.Exp(sigma*r.NormFloat64() - sigma*sigma/2)
						for y := by; y < by+s; y++ {
							for x := bx; x < bx+s; x++ {
								eps[y*side+x] *= w
							}
						}
					}
				}
			}
			fields = append(fields, eps)
		}
		c1, _ := traceMoments(fields, side)
		near(fmt.Sprintf("a lognormal cascade of sigma %.1f's C1", sigma), c1, sigma*sigma/(2*math.Ln2), 0.015)
	}
}

// The sea floor is laid at the distances awayFrom reads, and those are the
// distances: from any tile to the nearest of a scatter, straight across and
// round the seam, as Pythagoras has it. And round coasts that lean to
// neither the map's axes nor its diagonals - a Brownian relief's - a floor
// laid at them leans no more than its coast does. Laid at a walk to the
// eight tiles round each, which counted a diagonal step as one, it leaned to
// the grid by 0.22 to 0.25 round coasts that read 0.014 to 0.067.
func TestTheFloorLeansAsItsCoastDoes(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 5))
	g := NewGrid(61, 23)
	g.Wrap = true
	var at []int
	for range 7 {
		at = append(at, r.IntN(len(g.Tiles)))
	}
	away := g.awayFrom(func(i int) bool { return slices.Contains(at, i) })
	for i := range g.Tiles {
		want := math.Inf(1)
		for _, j := range at {
			dx := math.Abs(float64(i%g.W - j%g.W))
			dx = math.Min(dx, float64(g.W)-dx)
			want = math.Min(want, math.Hypot(dx, float64(i/g.W-j/g.W)))
		}
		if math.Abs(away[i]-want) > 1e-9 {
			t.Fatalf("tile %d is %.4f tiles from the nearest, and awayFrom says %.4f", i, want, away[i])
		}
	}

	for _, hurst := range []float64{0.5, 0.8} {
		f := fbmField(1024, 512, hurst, 3)
		g := drawnHeights(1024, 512, f, quantile(f, 0.7))
		near := g.awayFrom(func(i int) bool { return !g.underSea(i) })
		floor := func(*Grid) ([]float64, func(i int) bool) {
			depth := make([]float64, len(g.Tiles))
			for i := range depth {
				if g.underSea(i) {
					depth[i] = 4000 * smooth(clamp01((near[i]-2)/4))
				}
			}
			return depth, func(i int) bool { return g.underSea(i) && near[i] <= 8 }
		}
		coast, sea := gridLock([]*Grid{g}, landEdge), gridLock([]*Grid{g}, floor)
		if !(sea <= coast+0.02) {
			t.Errorf("round Brownian coasts of H %.1f that lean %.3f to the grid, the floor leans %.3f", hurst, coast, sea)
		}
	}
}

// drawnLand is a map w by h whose land is where land says, by the middle of
// each tile, and whose sea is everywhere else.
func drawnLand(w, h int, land func(x, y float64) bool) *Grid {
	g := NewGrid(w, h)
	g.sea = 0.5
	for i := range g.Tiles {
		if land(float64(i%w)+0.5, float64(i/w)+0.5) {
			g.Height[i] = 1
		}
	}
	return g
}

// drawnHeights is a map that wraps, w by h, of the heights given and a sea at
// sea.
func drawnHeights(w, h int, height []float64, sea float64) *Grid {
	g := NewGrid(w, h)
	g.Wrap = true
	g.sea = sea
	copy(g.Height, height)
	return g
}

// fbmField is a fractional Brownian surface of Hurst exponent hurst, w by h
// and periodic both ways: white noise, each wave of it scaled to k^-(1+H).
func fbmField(w, h int, hurst float64, seed uint64) []float64 {
	r := rand.New(rand.NewPCG(seed, 7))
	x := make([]complex128, w*h)
	for ky := 0; ky < h; ky++ {
		fy := float64(ky)
		if ky > h/2 {
			fy -= float64(h)
		}
		for kx := 0; kx < w; kx++ {
			fx := float64(kx)
			if kx > w/2 {
				fx -= float64(w)
			}
			if k := math.Hypot(fx/float64(w), fy/float64(h)); k > 0 {
				a := math.Pow(k, -(hurst + 1))
				x[ky*w+kx] = complex(a*r.NormFloat64(), a*r.NormFloat64())
			}
		}
	}
	kernel.FFT2(x, w, h, true, make([]complex128, h))
	out := make([]float64, w*h)
	for i := range out {
		out[i] = real(x[i])
	}
	return out
}

// polygonFill is which tiles of a map w by h have their middles inside a
// polygon, a row at a time.
func polygonFill(w, h int, poly [][2]float64) []bool {
	in := make([]bool, w*h)
	var xs []float64
	for y := 0; y < h; y++ {
		yc := float64(y) + 0.5
		xs = xs[:0]
		for k := range poly {
			a, b := poly[k], poly[(k+1)%len(poly)]
			if (a[1] <= yc) != (b[1] <= yc) {
				xs = append(xs, a[0]+(yc-a[1])*(b[0]-a[0])/(b[1]-a[1]))
			}
		}
		slices.Sort(xs)
		for k := 0; k+1 < len(xs); k += 2 {
			for x := max(0, int(math.Ceil(xs[k]-0.5))); x < w && float64(x)+0.5 <= xs[k+1]; x++ {
				in[y*w+x] = true
			}
		}
	}
	return in
}

// kochIsland is Koch's snowflake about (cx, cy) with sides of side before the
// first bend, bent depth times.
func kochIsland(cx, cy, side float64, depth int) [][2]float64 {
	var pts [][2]float64
	for k := range 3 {
		a := -math.Pi/2 + 2*math.Pi*float64(k)/3
		pts = append(pts, [2]float64{cx + side/math.Sqrt(3)*math.Cos(a), cy + side/math.Sqrt(3)*math.Sin(a)})
	}
	c, s := math.Cos(-math.Pi/3), math.Sin(-math.Pi/3)
	for range depth {
		var next [][2]float64
		for k := range pts {
			a, b := pts[k], pts[(k+1)%len(pts)]
			dx, dy := (b[0]-a[0])/3, (b[1]-a[1])/3
			p1 := [2]float64{a[0] + dx, a[1] + dy}
			next = append(next, a, p1, [2]float64{p1[0] + dx*c - dy*s, p1[1] + dx*s + dy*c}, [2]float64{a[0] + 2*dx, a[1] + 2*dy})
		}
		pts = next
	}
	return pts
}
