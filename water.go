package terra

import (
	"math"
	"slices"
)

// The sea as an amount of water, and not a share of the map.
//
// A world is given so much water, and the water goes where the ground is low.
// How much of the world it covers is then the ground's to say: a world whose
// continents stand high and whose ocean floors lie deep keeps its water in its
// basins, and one whose crust is mostly low, or whose continents ride only a
// little above the floor, is drowned. It is the plates that decide it, through
// how much of the crust they made continent and how high they left it.
//
// The sea was the lowest share of the map before, which made every world the
// same amount of sea whatever its history had done: a world that had welded
// its continents into one and a world that had torn them into islands came out
// three tenths water alike. It is still that way on a map that was drawn,
// which has no plates and so nothing to decide it with.

// DefaultWater is how much water a world made from its history is given, as
// the depth in metres it would stand to if it were spread evenly over every
// tile, and BasinDepth how far below the foot of the continents the deepest
// ocean floor lies. Between them they are the water and the basins it fills.
//
// The drawn map's heights are what the land is matched to - sixty metres of
// lowland, the high country standing on a fifth of it - and those are the
// heights everything downstream was measured against, so the continents keep
// them. The ocean floor never had any: a drawn map has no sea floor, only its
// lowest ground. So the floor is laid below the land by its place in the
// history's order, from the foot of the continents down to BasinDepth under
// it, and the water is enough to all but fill it. That is the shelves and the
// margins; past them the floor goes on down to the depth its age puts it at,
// kilometres, and holds its own water - see abyss.go - so Water is what stands
// over the shelves, and it is that which says where the coast is.
//
// The floor is laid by rank and not by how far below the continents the
// history left it. A plate's edge is a ramp a few tiles wide from the floor up
// to the continent, and read by height that ramp was half the depth of the
// basin in the space of a coast: the sea came up against a slope of two in ten
// on every seed, and no tidal flat was left anywhere. By rank the ramp is a few
// tiles of the order and a few centimetres of depth.
//
// How deep the basins are is then how gentle the ground is where the water
// meets it, and how much water fills them is where it meets it. Tried over
// eight small globes and the first three full ones, with the sea's share of
// each and the flats the tide lays:
//
//	basins   water   sea, small globes   sea, globes        flats, globes
//	40 m     12 m    .535 - .630         .518 .558 .618     1.10  0     0
//	25 m     7.5 m   .532 - .630         .517 .558 .618     1.94  0     0
//	20 m     6 m     .531 - .629         .517 .558 .618     2.48  0.06  0
//	20 m     5 m     .517 - .577         .512 .552 .564     3.34  3.40  1.88
//
// With more water than the basins hold the sea runs up onto the continents'
// edges, which are steeper than the floor, and the flats are gone. A little
// less, and it stands on the floor below the foot of the continents on every
// seed. The share it comes to is about the share of the crust that is ocean,
// a half to three fifths - the real world is seven tenths - and it is the
// plates and not this figure that move it from one world to the next.
//
// The table was taken when a basin was ocean by its plate's kind at the end of
// the history. The crust has carried its own kind tile by tile since, and it is
// set by the first plates' draw, so the same 7.5 m in 20 m basins now comes to
//
//	sea, small globes 1-8                    sea, globes 1-3
//	.641 .576 .623 .675 .693 .610 .538 .652  .608 .499 .658
//
// with that draw held within crustSlack of the asked share. Without that it
// came to .771 on the first globe and .774 on the fourth small one, whose
// first plates had drawn eight tenths of their ground as floor: see crustSlack.
const (
	DefaultWater = 7.5
	BasinDepth   = 20.0
)

// level is the height the sea stands at when water metres of it are poured
// over the whole map: the level at which the room under it, summed over every
// tile lower than it, comes to that much. The sea finds one level everywhere,
// which is also how the map's sea is read - see underSea - so ground low
// enough anywhere on the map is under it.
//
// It is the heights sorted once and walked up: below the next tile's height
// the room grows as the number of tiles already under water, so each step is
// a straight line and the level falls on one of them.
func (g *Grid) level(water float64) float64 {
	n := len(g.Tiles)
	if n == 0 || water <= 0 {
		return -1
	}
	h := make([]float64, n)
	for i := range g.Tiles {
		h[i] = g.laidHeight(i)
	}
	slices.Sort(h)
	want := water * float64(n)
	below := 0.0 // the heights of the tiles already under water, summed
	for k := 1; k <= n; k++ {
		below += h[k-1]
		// k tiles under water, and the level somewhere from h[k-1] up to the
		// next tile's height: room = k*level - below.
		next := math.Inf(1)
		if k < n {
			next = h[k]
		}
		if room := float64(k)*next - below; room >= want {
			return (want + below) / float64(k)
		}
	}
	return (want + below) / float64(n)
}

// pour floods the map with water metres of it, spread as it would spread, and
// reads the sea level off where it stood. It is flood's other half: the same
// tiles go under and draw their fish in the same order, and only how the level
// is found differs. See seaAt.
//
// The deep sea floor holds its own water besides, all of the room it was
// deepened by, since it lies wholly under the level: so the level is found
// over the ground as it stood before that floor was laid, which is the same
// level to the last bit of rounding. Found over the floor itself, with its
// water added, it came out a millionth of a metre off, and that was enough to
// put a handful of tiles of coast on the other side of the sea and move every
// river network measured on a small globe. See abyss.go and laidHeight.
func (g *Grid) pour(water float64, rng interface{ Float64() float64 }) {
	g.seaAt(g.level(water), rng)
}

// repour finds the level again on the ground as it now lies, for the same
// water. Cutting the valleys below the sea gives the water more room, so the
// level falls; see relevel, which is the same for a sea given as a share.
func (g *Grid) repour(water float64) {
	if g.sea < 0 || water <= 0 {
		return
	}
	g.sea = g.level(water)
	g.base = g.sea
}

// room is how much water the map holds under its sea as it now stands, as the
// depth it would come to spread over every tile.
func (g *Grid) room() float64 {
	if g.sea < 0 || len(g.Tiles) == 0 {
		return 0
	}
	sum := 0.0
	for i := range g.Tiles {
		sum += math.Max(0, g.sea-g.Height[i])
	}
	return sum / float64(len(g.Tiles))
}

// basins rescales a made world's heights the way normalise does, but in two
// parts, so that what its plates made of it survives the rescaling. Ocean
// crust is the ocean floor, and takes depths below the continents by its
// place in the history's order among the floor; continental crust is the
// continents, and takes the drawn map's heights by its place among them,
// lifted to stand on the floor.
//
// The two are ordered apart. Ordered as one, with as many of the lowest tiles
// as lie on ocean crust made floor, whatever floor the history had left
// standing high - a swell in the middle of a plate, a ridge, a hotspot's
// cone - took a continent's heights and came up out of the open ocean as an
// island, while as much low continent went under to make up the count. And
// since the water does not quite fill the basins, the top of the floor stood
// in the air too, as flat banks of basalt far out at sea. Ordered apart, the
// floor is all below the foot of the continents, and what the sea covers
// beyond it is continent: the shelves and the low basins of the land, which
// is what the shallow seas of the earth are.
//
// A margin is still a slope: the floor beside a continent is the highest of
// the floor, having been raised by the continent's ramp, and the continent's
// edge the lowest of the continent, so each side meets the foot from its own.
func (w *Land) basins(g *Grid, ocean []bool) {
	defer phase("basins")()
	n := len(g.Tiles)
	floor := 0
	for i := range g.Tiles {
		if ocean[i] {
			floor++
		}
	}
	spread := w.relief(g)
	slices.Sort(spread)

	order := make([]int32, n)
	for i := range order {
		order[i] = int32(i)
	}
	slices.SortFunc(order, func(a, b int32) int {
		if ocean[a] != ocean[b] {
			if ocean[a] {
				return -1
			}
			return 1
		}
		ha, hb := g.Height[a], g.Height[b]
		switch {
		case ha < hb:
			return -1
		case ha > hb:
			return 1
		}
		return int(a - b) // ties by position, so a world repeats
	})
	if floor == 0 || floor == n {
		for rank, i := range order {
			g.Height[i] = spread[rank]
		}
		return
	}
	// The ocean floor, by its place in the order below the continents' foot:
	// the deepest tile at the bottom of the basins and the shallowest at the
	// foot. Laid as a straight line of the order; a floor that lay deeper for
	// more of its width, at a square or a fourth power, came up steeper at the
	// coast and left the flats on one seed of four.
	for rank := 0; rank < floor; rank++ {
		i := order[rank]
		g.Height[i] = BasinDepth * float64(rank) / math.Max(1, float64(floor-1))
	}
	// The continents, onto the drawn map's heights: the lowest of them onto the
	// lowest the drawn map has, and so on up, over the whole of its spread.
	land := n - floor
	for k := 0; k < land; k++ {
		i := order[floor+k]
		at := int(float64(k) * float64(n-1) / math.Max(1, float64(land-1)))
		g.Height[i] = BasinDepth + spread[at]
	}
}

// SeaLevel is the height the sea stands at, or below nothing on a map with no
// sea.
func (g *Grid) SeaLevel() float64 { return g.sea }
