package terra

import (
	"math"
	"slices"
	"sort"
)

// Strata: the rock is a pile, not a floor.
//
// A tile's rock was one kind, fixed when the map was made, and however far
// the water cut into it the water found the same rock all the way down. Real
// country is not like that. Rock is laid down in beds - a basin fills with
// mud, the sea over it leaves lime, a river brings sand, a rift floods it all
// with lava - and the next age buries the last. The plates then lift the pile,
// tip it and crumple it, and the weather takes the top off. What a hillside is
// made of is where its surface happens to cut that pile, and what shape it has
// is how the beds it cuts stand up to the weather: a hard bed over a soft one
// is a scarp, a flat hard bed left over soft ground is a mesa, and a steeply
// tipped hard bed is a hogback ridge running the way the beds strike. A river
// crossing the edge of a hard bed onto a soft one is a waterfall.
//
// So every tile keeps a column of beds. Each bed is a rock, the height of its
// upper surface, and the epoch it was laid; the lowest is the basement, which
// goes on down for ever. The heights are absolute - the same metres the ground
// is measured in - so that when the plates lift a range the beds in it rise
// with it, and a belt lifted more at its middle than at its feet leaves them
// tipped. Bedrock on the tile is the bed its surface lies in, read again
// whenever the ground moves; see expose.

// bedsMax is how many beds a column keeps. A history lays one every epoch it
// buries anything, and a pile deeper than this keeps its thickest beds and
// folds the thinnest into the one beneath: see column.merge.
const bedsMax = 8

// column is one tile's pile of beds, from the top down. Bed k lies between
// top[k+1] and top[k]; the last bed has no floor. sand is the share of sand
// in a bed laid by water, out of 255, which is what decides at the end of a
// history whether that bed is sandstone or shale; see settleRock.
type column struct {
	top    [bedsMax]float32
	rock   [bedsMax]Bedrock
	formed [bedsMax]uint8
	sand   [bedsMax]uint8
	n      uint8
}

// basement is a column with nothing laid on it: one rock, all the way down.
func basement(rock Bedrock, formed uint8, top float64) column {
	c := column{n: 1}
	c.top[0], c.rock[0], c.formed[0] = float32(top), rock, formed
	return c
}

// at is which bed the ground at height h lies in: the first whose floor is
// below it.
func (c *column) at(h float64) int {
	for k := 0; k+1 < int(c.n); k++ {
		if float64(c.top[k+1]) < h {
			return k
		}
	}
	return int(c.n) - 1
}

// rockAt is the rock at height h.
func (c *column) rockAt(h float64) Bedrock { return c.rock[c.at(h)] }

// truncate takes off every bed that lies wholly above h: what the weather has
// taken off is gone, and ground laid on the same place later is new rock.
func (c *column) truncate(h float64) {
	if k := c.at(h); k > 0 {
		c.drop(0, k)
	}
	if float64(c.top[0]) > h {
		c.top[0] = float32(h)
	}
}

// drop removes count beds starting at bed k.
func (c *column) drop(k, count int) {
	n := int(c.n)
	copy(c.top[k:n], c.top[k+count:n])
	copy(c.rock[k:n], c.rock[k+count:n])
	copy(c.formed[k:n], c.formed[k+count:n])
	copy(c.sand[k:n], c.sand[k+count:n])
	c.n = uint8(n - count)
}

// lift raises every bed by by metres, as the plates raise the ground over it.
func (c *column) lift(by float64) {
	for k := 0; k < int(c.n); k++ {
		c.top[k] += float32(by)
	}
}

// thick is how thick bed k is. The basement is thick without end.
func (c *column) thick(k int) float64 {
	if k+1 >= int(c.n) {
		return math.Inf(1)
	}
	return float64(c.top[k] - c.top[k+1])
}

// lay puts a bed of rock down on ground standing at bottom, up to top: what
// stood above bottom is taken off first, since the new bed is where it was.
// A bed of the same rock as the one it lands on only thickens that one.
func (c *column) lay(rock Bedrock, formed, sand uint8, bottom, top float64) {
	if !(top > bottom) {
		return
	}
	c.truncate(bottom)
	// And what it is laid on reaches up to it: the bed beneath stands to the
	// new bed's floor, wherever the pile's top was.
	c.top[0] = float32(bottom)
	if c.rock[0] == rock {
		if c.n > 1 {
			c.top[0], c.formed[0] = float32(top), formed
			c.sand[0] = uint8((int(c.sand[0]) + int(sand)) / 2)
		}
		return
	}
	if int(c.n) == bedsMax {
		c.merge()
	}
	n := int(c.n)
	copy(c.top[1:n+1], c.top[:n])
	copy(c.rock[1:n+1], c.rock[:n])
	copy(c.formed[1:n+1], c.formed[:n])
	copy(c.sand[1:n+1], c.sand[:n])
	c.top[0], c.rock[0], c.formed[0], c.sand[0] = float32(top), rock, formed, sand
	c.n++
}

// bury lays a bed thick metres deep with its top at top, the ground under it
// sinking to make room rather than being taken off. It is what a basin does:
// it goes down under what it is given, which is how a pile of beds a
// kilometre thick comes to lie under ground that was never more than a few
// metres above the sea.
func (c *column) bury(rock Bedrock, formed, sand uint8, top, thick float64) {
	if !(thick > 0) {
		return
	}
	c.lift(-thick)
	c.lay(rock, formed, sand, top-thick, top)
}

// merge folds the thinnest bed above the basement into the one under it,
// which keeps the rock of whichever of the two is thicker. It makes room for
// another bed at the cost of the least of the pile.
func (c *column) merge() {
	if c.n < 2 {
		return
	}
	thin := 0
	for k := 1; k+1 < int(c.n); k++ {
		if c.thick(k) < c.thick(thin) {
			thin = k
		}
	}
	if c.thick(thin+1) >= c.thick(thin) {
		c.rock[thin], c.formed[thin], c.sand[thin] = c.rock[thin+1], c.formed[thin+1], c.sand[thin+1]
	}
	c.drop(thin+1, 1)
}

// tidy joins neighbouring beds of the same rock into one.
func (c *column) tidy() {
	for k := 0; k+1 < int(c.n); {
		if c.rock[k] == c.rock[k+1] {
			if k+2 >= int(c.n) {
				// Joined to the basement, the bed is the basement.
				c.formed[k], c.sand[k] = c.formed[k+1], c.sand[k+1]
			}
			c.drop(k+1, 1)
			continue
		}
		k++
	}
}

// cook turns every bed lying wholly below height below into rock, and the
// basement with them: what a collision buries deep enough is squeezed into
// schist, and what an arc melts under itself cools as granite.
func (c *column) cook(below float64, rock Bedrock, formed uint8) {
	for k := int(c.n) - 1; k >= 0; k-- {
		if k+1 < int(c.n) && float64(c.top[k]) > below {
			break
		}
		c.rock[k], c.formed[k] = rock, formed
	}
	c.tidy()
}

// piles gives a map that has none a pile under every tile of the one rock the
// tile already is, for the passes of a history that lay and lift beds.
func (g *Grid) piles() {
	if g.strata != nil {
		return
	}
	g.strata = make([]column, len(g.Tiles))
	for i := range g.Tiles {
		g.strata[i] = basement(g.Tiles[i].Bedrock, g.Tiles[i].Formed, g.Height[i])
	}
}

// hardAt is how hard the rock is at height h under tile i. A map with no
// strata answers with the tile's own rock.
func (g *Grid) hardAt(i int, h float64) float64 {
	return hardness[g.bedAt(i, h)]
}

// bedAt is the rock at height h under tile i. A map with no strata answers
// with the tile's own rock.
func (g *Grid) bedAt(i int, h float64) Bedrock {
	if g.strata == nil {
		return g.Tiles[i].Bedrock
	}
	return g.strata[i].rockAt(h)
}

// meanHard is how hard the map's exposed rock is on average. The rock charges
// the water and holds up the slopes against this and not against a fixed
// figure, so that a map of all one rock wears exactly as it did before there
// were beds, and the rates the yardsticks measure are the map's own.
//
// The deep floor is not in it: nothing the weather does reaches it (see
// abyssal), and it was, so that the sediment laid on the floor - limestone
// and shale over its basalt - softened the mean and moved every hillside on
// the land, and a small globe's rivers with them.
func (g *Grid) meanHard() float64 {
	sum, n := 0.0, 0
	for i := range g.Tiles {
		if g.abyssal(i) {
			continue
		}
		sum += g.Tiles[i].Hard()
		n++
	}
	return sum / math.Max(1, float64(n))
}

// expose takes off every tile's pile whatever the ground no longer reaches,
// and writes the bed its surface now lies in onto the tile: its rock and the
// epoch that rock dates from. It is run whenever the ground has moved.
func (g *Grid) expose() {
	if g.strata == nil {
		return
	}
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			t, c := &g.Tiles[i], &g.strata[i]
			c.truncate(g.Height[i])
			t.Bedrock, t.Formed = c.rock[0], c.formed[0]
		}
	})
}

// Denuding: what the ages take off the soft rock and leave on the hard.
//
// The water shapes a channel steeper where it crosses a hard bed, but a
// channel runs down the country and a bed may run across it, so what that
// makes is a step in a river and not a ridge along the bed. Real ground gets
// its ridges from the whole surface coming down, over far longer than any
// age a map is made or played in: the soft beds go faster wherever they come
// to the surface, rivers find the lines they make and hollow them out, and
// the hard beds between are left standing - a hogback where the beds are
// tipped, a scarp where they lie flat and end, and a mesa where a flat cap
// is all that is left. None of the passes that make a map runs long enough
// for that at the rates the yardsticks hold them to, so it is done here as
// what it adds up to.
//
// Each tile comes down by how much softer its rock is than its own pile over
// the next denudeWindow metres down, and goes up by as much where it is
// harder, so what the pass moves is one bed against the beds it lies among
// and not one country against the next: a soft bed is hollowed along its
// whole outcrop, a hard one stands proud of it, and ground of one rock all
// the way down is left where it was. Read against the ground round a tile
// instead, the edge of every province of one rock became a step, and on a
// made globe the drainage came apart along them. It is taken in denudePasses
// steps, reading the bed each step bares, because what lies under a soft bed
// that has come down is the next bed along.
//
// denudeDepth is how many metres, over all the passes, a rock as soft again
// as its pile comes down. It is small, because it is paid for in slope: at
// six metres the made small globes' mean slope came out at 0.60, over the
// thirty degrees they had been held to, while hard beds on a test dome stood
// two metres proud of their strike valleys; see TestHogbacksRunAlongTheStrike
// and the yardsticks it loosened.
const (
	denudeDepth  = 6.0
	denudePasses = 4
	denudeWindow = 60.0
	denudeFall   = 0.002 // the least fall, as rise over run, left down a way the water went
)

// denude takes the soft rock down against the hard. See denudeDepth.
func (g *Grid) denude() {
	if g.strata == nil {
		return
	}
	// The water keeps the way it had. A soft bed hollowed out across a river
	// would otherwise be a pit the river ends in, and the network the ground
	// was shaped to would come apart into a basin at every outcrop: so no
	// tile comes down below a hair's fall over the tile its water went to.
	recv, run := g.receivers()
	stack := stackOf(recv)
	for pass := 0; pass < denudePasses; pass++ {
		for i := range g.Tiles {
			if g.underSea(i) {
				continue
			}
			by := denudeDepth / denudePasses * (1/g.Tiles[i].Hard()/g.strata[i].soft(g.Height[i]) - 1)
			g.Height[i] -= by
			if g.sea >= 0 && by > 0 {
				g.Height[i] = math.Max(g.Height[i], math.Min(g.sea, g.Height[i]+by))
			}
		}
		for _, i := range stack {
			if r := recv[i]; r != i {
				g.Height[i] = math.Max(g.Height[i], g.Height[r]+denudeFall*run[i])
			}
		}
		g.expose()
	}
}

// restrata carries the beds through a pass that hands the ground new heights
// by rank - normalise, basins, shape - so that they keep their place against
// the ground. A height of the old ground is taken to the height of the new
// ground at the same place in the order, and every bed surface goes with it:
// so the ground a tile stands on lies in the same bed after as before, and a
// flat bed stays flat wherever the ground over it was spread evenly. Past the
// ends of the order the spread of the whole is kept.
//
// group, where it is given, orders two kinds of ground apart, as basins does
// the ocean floor and the continents. The deep sea floor is counted where it
// was laid from, and its beds are not carried: no pass hands it heights by
// rank, and counted at its kilometres, it moved the beds under every hill on
// the map. See abyssal and laidHeight.
func (g *Grid) restrata(from, to []float64, group []bool) {
	if g.strata == nil {
		return
	}
	a := make([]float64, 0, len(from))
	b := make([]float64, 0, len(from))
	for _, kind := range []bool{false, true} {
		if group == nil && kind {
			break
		}
		a, b = a[:0], b[:0]
		for i := range from {
			switch {
			case group != nil && group[i] != kind:
			case g.abyssal(i):
				a = append(a, g.laidHeight(i))
				b = append(b, g.laidHeight(i))
			default:
				a = append(a, from[i])
				b = append(b, to[i])
			}
		}
		if len(a) == 0 {
			continue
		}
		slices.Sort(a)
		slices.Sort(b)
		m := len(a) - 1
		s := 1.0
		if a[m] > a[0] {
			s = (b[m] - b[0]) / (a[m] - a[0])
		}
		carry := func(v float64) float64 {
			switch {
			case v <= a[0]:
				return b[0] + (v-a[0])*s
			case v >= a[m]:
				return b[m] + (v-a[m])*s
			}
			k := sort.SearchFloat64s(a, v)
			lo, hi := a[k-1], a[k]
			if hi <= lo {
				return b[k]
			}
			f := (v - lo) / (hi - lo)
			return b[k-1] + f*(b[k]-b[k-1])
		}
		g.EachRow(func(y int) {
			for i := y * g.W; i < (y+1)*g.W; i++ {
				if (group != nil && group[i] != kind) || g.abyssal(i) {
					continue
				}
				c := &g.strata[i]
				for k := 0; k < int(c.n); k++ {
					c.top[k] = float32(carry(float64(c.top[k])))
				}
			}
		})
	}
}

// heights is every tile's height, in tile order.
func (g *Grid) heights() []float64 {
	h := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		h[i] = g.Height[i]
	}
	return h
}

// soft is how soft the pile is, on average through its thickness, over the
// denudeWindow metres below h: what the ground at h is weathering down into.
func (c *column) soft(h float64) float64 {
	sum, top := 0.0, h
	for k := c.at(h); k < int(c.n) && top > h-denudeWindow; k++ {
		floor := h - denudeWindow
		if k+1 < int(c.n) {
			floor = math.Max(floor, float64(c.top[k+1]))
		}
		sum += (top - floor) / hardness[c.rock[k]]
		top = floor
	}
	return sum / denudeWindow
}
