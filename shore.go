package terra

import (
	"math"
	"slices"

	"github.com/LukasSelin/terra/geom"
)

// The shore: how big the tide is on each coast, and the ground it covers and
// uncovers.
//
// The tide the moon raises in the open ocean is well under a metre. What makes
// one coast's tide ten metres and another's nothing is the shape of the water
// it comes into. A tide that runs into a bay that narrows is squeezed, and
// rises; one that runs onto a shallow shelf is slowed, and rises; one that has
// to find its way into a sea through a strait arrives spent, and hardly moves
// at all. Green's law says how: the height goes as the width of the channel to
// the minus a half and its depth to the minus a quarter.
//
// A map is far too small for that to be taken literally. A tide is a wave some
// hundreds of kilometres long, and a globe here is twenty-five kilometres round,
// so no bay on any map is long enough to ring with it. What is kept is the
// shape of the rule rather than its lengths: how much sea is about a place
// close to, against how much further off, stands for how the channel narrows;
// how deep the water is close to, against further off, stands for the shelf;
// and how much of the world's sea a body of water is stands for whether the
// tide can get into it. The reach of each is a share of the map, as Maritime's
// is, for the same reason.
//
// Where the tide reaches ground between its lowest low water and its highest
// high, that ground is a tidal flat: mud the sea covers and leaves. It is laid
// once an age, when the coast is, and never changes from day to day - the day
// is asked instead. See Covered.

// How much the shape of a coast gathers the tide, as multiples of the open
// ocean's: a narrowing bay at most three times, a headland as little as half,
// and a shallow shelf at most half as much again. TideFactorMax is the most
// the two come to together - a spring range of seven and a half metres, which
// is the Wash or Morecambe Bay. The Severn's and Brittany's twelve and the Bay
// of Fundy's sixteen are resonances of basins far bigger than any map here.
const (
	funnelLow, funnelHigh = 0.5, 3.0
	shelfHigh             = 1.6
	TideFactorMax         = funnelHigh * shelfHigh
)

// FlatMinRange is the least spring range, in metres, that lays flats, and
// flatSlope the steepest ground they lie on. The flats are the ground between
// an ordinary spring's low water and its high - what the tide leaves and
// covers every fortnight - where it is gentle enough for mud to lie: a shore
// steeper than one in a hundred is shingle or rock, and the tide runs up and
// off it without leaving anything anybody would call a flat.
//
// The slope is what holds the flats to a share of the map at all. The made sea
// is shallow and its floor gentle, so a band a tide's reach either side of
// mean sea takes in a great deal of it, and the more a coast gathers the tide
// the wider its band. Tried on a globe and two small ones, with the funnel read
// as the ratio of near to far sea and the coasts' spring range at its middle,
// its tenth highest and its most:
//
//	funnel      least range   steepest   flats, globe   small globes    range
//	root           1.5 m         any        3.03%       3.33, 2.45%     1.95 / 2.82 / 3.84 m
//	ratio          1.5 m         any        4.85%       4.00, 3.06%     2.33 / 4.01 / 5.89 m
//	ratio          2.0 m         any        4.33%       3.24, 2.69%     2.33 / 4.01 / 5.89 m
//	ratio          1.5 m         0.02       3.01%       1.58, 0.48%     2.33 / 4.01 / 5.89 m
//	ratio          1.5 m         0.01       2.10%       1.01, 0.12%     2.33 / 4.01 / 5.89 m
//
// The root of the ratio - Green's law taken at its word, when the ratio is not
// a width - gave every coast nearly the same tide. Taken whole, the head of a
// bay has twice the open coast's and a headland half.
const (
	FlatMinRange = 1.5
	flatSlope    = 0.01
)

// flatTide is how far from mean sea a flat reaches, at an open coast.
const flatTide = TideM2 + TideS2

// tides reads the tide's reach off the coast as it now lies, and lays the flats
// it covers and uncovers. It runs when the land is made and after every age of
// weather, because the weather moves the coast. A map with no sea has no tide,
// and nothing here touches it.
//
// It draws no chance and writes nothing but the tiles it turns, so it may run
// anywhere in the making of a world without moving a seed.
func (g *Grid) tides() {
	if g.sea < 0 {
		g.tidal, g.ebb = nil, nil
		return
	}
	n := len(g.Tiles)
	f := g.tidalReach()

	// Where a river meets the sea, its channel is kept open through the mud:
	// a creek. Without one a river ends at the top of its own flats and the
	// water it carries has nowhere drawn to go.
	creek := make([]bool, n)
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if t.Terrain != Water || g.underSea(i) || f[i] <= 0 || t.Flow < settleFlow {
			continue
		}
		for j, steps := i, 0; steps < g.W; steps++ {
			p := g.PosOf(j)
			a := g.Aspect(p)
			q := geom.Pos{X: p.X + a.X, Y: p.Y + a.Y}
			if a == (geom.Pos{}) || !g.In(q) {
				break
			}
			j = g.Index(q)
			if !g.underSea(j) {
				continue
			}
			if g.Tiles[j].Height <= g.sea-float64(f[j])*flatTide || creek[j] {
				break
			}
			creek[j] = true
		}
	}

	if len(g.ebb) != n {
		g.ebb = make([]float32, n)
	}
	for i := range g.Tiles {
		t := &g.Tiles[i]
		g.ebb[i] = 0
		reach := float64(f[i]) * flatTide
		band := f[i] > 0 && 2*float64(f[i])*(TideM2+TideS2) >= FlatMinRange &&
			t.Height > g.sea-reach && t.Height <= g.sea+reach && g.Slope(g.PosOf(i)) <= flatSlope
		held := t.Mark != None || t.Owner != 0
		under := g.underSea(i)
		turns := false
		switch t.Terrain {
		case Grass, Forest, Rock, Flat:
			turns = !g.Frozen(g.PosOf(i))
		case Water:
			turns = under && !creek[i]
		}
		switch {
		case band && turns && !held:
			if t.Terrain != Flat {
				t.Terrain, t.Fenced = Flat, false
				g.Wood[i], g.Wild[i], g.Age[i], g.Fish[i] = 0, 0, 0, 0
				g.Fertility[i], g.Rich[i] = 0, 0
			}
			g.ebb[i] = float32((g.sea - t.Height) / float64(f[i]))
		case t.Terrain == Flat && !band:
			if under {
				t.Terrain = Water // with no fish yet; they come back as water's do
			} else {
				t.Terrain = Grass
			}
		}
	}
	g.tidal = f
}

// tidalReach is, for every tile, how many times the open ocean's tide it has:
// nothing where the tide does not come, and up to TideFactorMax at the head of
// a bay. The sea has it, the ground the tide covers has it, and a river has it
// up its channel until its bed stands above the highest water - the tidal
// limit, which is where an estuary stops being one.
func (g *Grid) tidalReach() []float32 {
	n := len(g.Tiles)
	span := g.Span()
	near := max(2, span/64)
	far := max(near+1, span/8)

	sea := make([]float64, n)
	depth := make([]float64, n)
	for i := range g.Tiles {
		if g.underSea(i) {
			sea[i], depth[i] = 1, g.sea-g.Tiles[i].Height
		}
	}
	seaNear, seaFar := g.boxMean(sea, near), g.boxMean(sea, far)
	depthNear, depthFar := g.boxMean(depth, near), g.boxMean(depth, far)

	// How much of the world's sea each body of water is. A sea the tide has to
	// come into through somebody else's coast gets a share of it; one with no
	// way to the ocean at all gets almost none.
	body, sizes := g.seaBodies()
	total := 0
	for _, s := range sizes {
		total += s
	}
	enclosed := make([]float64, len(sizes))
	for b, s := range sizes {
		enclosed[b] = math.Min(1, math.Sqrt(float64(s)/(0.25*float64(total))))
	}

	// The shape of the water about each tile, as what it does to a tide.
	shape := make([]float64, n)
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			// Water close about a place against water further off: at the
			// head of an inlet the one is most of the ground and the other
			// little of it, and the tide has been squeezed to get there; off
			// a headland or an island it is the other way about.
			funnel := seaNear[i] / math.Max(seaFar[i], 0.02)
			funnel = math.Max(funnelLow, math.Min(funnelHigh, funnel))
			shelf := 1.0
			if seaNear[i] > 0 && seaFar[i] > 0 {
				d := (depthFar[i] / seaFar[i]) / math.Max(depthNear[i]/seaNear[i], 0.1)
				shelf = math.Max(1, math.Min(shelfHigh, math.Pow(d, 0.25)))
			}
			shape[i] = funnel * shelf
		}
	})

	// From the sea inland. Every tile the tide reaches is reached from a tile
	// it has already reached, first in tile order, and it carries that tile's
	// sea with it; the tide dies away with every step, slowly up a channel and
	// quickly over ground, and stops wherever the ground stands above the
	// highest water it could bring.
	f := make([]float32, n)
	from := make([]int32, n)
	queue := make([]int32, 0, n/4)
	for i := range g.Tiles {
		from[i] = -1
		if g.underSea(i) && !g.Freezing(g.PosOf(i)) && g.Tiles[i].Terrain != Ice {
			f[i] = float32(math.Min(TideFactorMax, enclosed[body[i]]*shape[i]))
			from[i] = body[i]
			queue = append(queue, int32(i))
		}
	}
	alongWater := math.Exp(-16 / float64(span))
	overGround := math.Exp(-64 / float64(span))
	for k := 0; k < len(queue); k++ {
		i := queue[k]
		p := g.PosOf(int(i))
		for _, off := range Dirs {
			q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
			if !g.In(q) {
				continue
			}
			j := g.Index(q)
			if from[j] >= 0 {
				continue
			}
			t := &g.Tiles[j]
			if t.Terrain == Ice || (t.Wet() && g.Freezing(q)) {
				continue
			}
			decay := overGround
			if t.Terrain == Water || t.Terrain == Flat {
				decay = alongWater
			}
			// The tide a tile gets is the sea's, dying away, but shaped by
			// where the tile is: a creek at the head of a narrowing bay gets
			// more than the mouth of it.
			own := enclosed[from[i]] * shape[j]
			carried := float64(f[i]) * decay
			got := math.Min(TideFactorMax, math.Max(carried, math.Min(own, carried*funnelHigh)))
			if t.Height > g.sea+got*TideMax {
				continue
			}
			f[j], from[j] = float32(got), from[i]
			queue = append(queue, int32(j))
		}
	}
	return f
}

// seaBodies labels each connected body of sea, eight ways round and across the
// seam, in tile order, and says how many tiles each is. Tiles that are not sea
// get -1.
func (g *Grid) seaBodies() ([]int32, []int) {
	n := len(g.Tiles)
	body := make([]int32, n)
	for i := range body {
		body[i] = -1
	}
	var sizes []int
	var stack []int32
	for i := range g.Tiles {
		if body[i] >= 0 || !g.underSea(i) {
			continue
		}
		b := int32(len(sizes))
		size := 0
		body[i] = b
		stack = append(stack[:0], int32(i))
		for len(stack) > 0 {
			j := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			size++
			p := g.PosOf(int(j))
			for _, off := range Dirs {
				q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
				if !g.In(q) {
					continue
				}
				k := g.Index(q)
				if body[k] < 0 && g.underSea(k) {
					body[k] = b
					stack = append(stack, int32(k))
				}
			}
		}
		sizes = append(sizes, size)
	}
	return body, sizes
}

// boxMean is, for each tile, the mean of v over the square window reach tiles
// either way of it, going round the seam and clipped at the poles. It is
// seaNear's summed table, asked of any reading.
func (g *Grid) boxMean(v []float64, reach int) []float64 {
	stride := g.W + 1
	sum := make([]float64, stride*(g.H+1))
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			sum[(y+1)*stride+x+1] = v[y*g.W+x] + sum[y*stride+x+1] + sum[(y+1)*stride+x] - sum[y*stride+x]
		}
	}
	box := func(x0, x1, y0, y1 int) float64 {
		return sum[(y1+1)*stride+x1+1] - sum[y0*stride+x1+1] - sum[(y1+1)*stride+x0] + sum[y0*stride+x0]
	}
	out := make([]float64, len(g.Tiles))
	g.EachRow(func(y int) {
		y0, y1 := max(0, y-reach), min(g.H-1, y+reach)
		rows := y1 - y0 + 1
		for x := 0; x < g.W; x++ {
			var s float64
			var n int
			switch {
			case !g.Wrap:
				x0, x1 := max(0, x-reach), min(g.W-1, x+reach)
				s, n = box(x0, x1, y0, y1), (x1-x0+1)*rows
			case 2*reach+1 >= g.W:
				s, n = box(0, g.W-1, y0, y1), g.W*rows
			default:
				x0, x1 := x-reach, x+reach
				switch {
				case x0 < 0:
					s = box(0, x1, y0, y1) + box(x0+g.W, g.W-1, y0, y1)
				case x1 >= g.W:
					s = box(x0, g.W-1, y0, y1) + box(0, x1-g.W, y0, y1)
				default:
					s = box(x0, x1, y0, y1)
				}
				n = (2*reach + 1) * rows
			}
			out[y*g.W+x] = s / float64(n)
		}
	})
	return out
}

// TidalRange is the spring range at p, in metres: how far the water falls from
// the highest high water of an ordinary spring tide to its lowest low. It is
// nothing where the tide does not come.
func (g *Grid) TidalRange(p geom.Pos) float64 {
	if len(g.tidal) != len(g.Tiles) || !g.In(p) {
		return 0
	}
	return 2 * float64(g.tidal[g.Index(p)]) * (TideM2 + TideS2)
}

// HighWater and LowWater are how high the day's tide comes at p and how low it
// falls, as heights on the map. Where the tide does not come, both are the
// sea's level, or below the map where there is no sea.
func (g *Grid) HighWater(p geom.Pos) float64 { return g.waterAt(p, g.tide.High) }

// LowWater is how low the day's tide falls at p. See HighWater.
func (g *Grid) LowWater(p geom.Pos) float64 { return g.waterAt(p, g.tide.Low) }

func (g *Grid) waterAt(p geom.Pos, h float64) float64 {
	if len(g.tidal) != len(g.Tiles) || !g.In(p) {
		return g.sea
	}
	return g.sea + float64(g.tidal[g.Index(p)])*h
}

// Covered reports whether the day's sea lies over the flat at p even at low
// water: whether, today, there is no walking across it. A flat the tide leaves
// at low water is open, however high it came in the morning - a day is the
// smallest thing there is, and somebody who can cross at low water does.
// Anything built on a flat - a causeway - stands above the water every day, as
// a bridge stands above a river.
func (g *Grid) Covered(p geom.Pos) bool {
	if len(g.ebb) != len(g.Tiles) || !g.In(p) {
		return false
	}
	i := g.Index(p)
	return g.covered(i, &g.Tiles[i])
}

func (g *Grid) covered(i int, t *Tile) bool {
	return t.Terrain == Flat && t.Mark == None && len(g.ebb) == len(g.Tiles) && float64(g.ebb[i]) >= g.tide.High
}

// shut is Shut by index, for the search.
func (g *Grid) shut(i int) bool {
	t := &g.Tiles[i]
	return t.Deep() || g.covered(i, t)
}

// Shut reports whether a laden walker is kept off p today: open water, or a
// flat the day's tide covers. See Tile.Deep, which is the same question asked
// of the water alone, without the day.
func (g *Grid) Shut(p geom.Pos) bool {
	if !g.In(p) {
		return false
	}
	i := g.Index(p)
	return g.Tiles[i].Deep() || g.covered(i, &g.Tiles[i])
}

// How the tide works the ground it covers. tidalSettle is how readily each
// grain comes out of the water over a flat, as a share of what passes it: the
// sand as readily as anywhere, and the silt and the clay far more readily than
// in a river, because slack water at the turn of the tide lets them fall and
// salt water makes clay clot into flakes that fall faster than its grains
// would. trapRange is the spring range at which a flat takes all of that: a
// coast whose tide hardly moves is hardly a trap for anything.
//
// Over twenty ages of two small globes, with the tide working the ground and
// without it:
//
//	            sent to the sea        river and sea tiles
//	seed 1     18178 m / 19409 m      8997 -> 9086 / 8997 -> 9344
//	seed 2     16783 m / 17970 m      8710 -> 8788 / 8710 -> 8948
//
// A fifteenth of what the rivers carry stays on the coast, and the rivers stop
// widening their mouths below high water - which is most of the difference in
// how much of the map becomes water.
//
// What is left out is the tide's own scour: the water that fills and empties a
// bay twice a day keeps the channel it comes through open, and deeper the more
// of the bay there is to fill. Nothing here cuts a channel for the tide; a
// creek is only kept open where a river already runs.
var tidalSettle = [Grains]float64{Sand: 0.62, Silt: 0.60, Clay: 0.45}

const trapRange = 4.0

// tideWork sets how the tide works the ground this age into the water's step:
// that no river is cut below the tide's high water where the tide reaches it,
// that flats above mean sea catch the fine stuff the water brings, and that a
// flat under mean sea keeps what reaches it until it stands at high water. On
// a map with no tide it does nothing, and the water's step is what it was.
func (g *Grid) tideWork(c *fluvial, recv []int32) {
	if len(g.tidal) != len(g.Tiles) || g.sea < 0 {
		return
	}
	n := len(g.Tiles)
	c.floor = make([]float64, n)
	c.keep = make([]float64, n)
	c.room = make([]float64, n)
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			f := float64(g.tidal[i])
			c.floor[i] = math.Inf(-1)
			if f <= 0 {
				continue
			}
			high := g.sea + f*MeanHigh
			c.floor[i] = high
			t := &g.Tiles[i]
			if t.Terrain != Flat || t.Mark != None {
				continue
			}
			trap := clamp01(2 * f * (TideM2 + TideS2) / trapRange)
			if int(recv[i]) == i {
				c.keep[i] = trap
				c.room[i] = math.Max(0, high-t.Height)
				continue
			}
			for gr := range tidalSettle {
				c.settle[i][gr] = math.Max(c.settle[i][gr], math.Min(0.9, tidalSettle[gr]*trap))
			}
		}
	})
	g.bays(c, recv)
}

// siltYears is how long the rivers have been bringing mud down to the coast
// when a world is made, and siltRounds how many times over the tide's reach is
// read again in the course of it. The seas of the earth stopped rising six to
// seven thousand years ago, and the flats and the deltas that stand on its
// coasts today are what has built up since (Stanley and Warne 1994).
const (
	siltYears  = 6000 * yr
	siltRounds = 6
)

// silt lays siltYears of the rivers' mud on the shoals, and nothing else: the
// land is left as its shaping graded it and its valleys were cut, because
// cutting them longer spoils them - see valleyYears - and only what settles in
// the tidal water is kept of each round. The flats are then read again on the
// coast the mud has made. The mud takes up room under the sea, so level is
// called after each round to find the sea's level again, for the same water.
func (g *Grid) silt(level func()) {
	if g.sea < 0 {
		return
	}
	n := len(g.Tiles)
	for range siltRounds {
		if len(g.tidal) != n {
			return
		}
		c := g.waterStep(siltYears / siltRounds)
		if c.bay == nil {
			return
		}
		next := c.solve(settleIters)
		change := make([]float64, n)
		gained := make([][Grains]float64, n)
		c.account(next, change, gained, func(int32, [Grains]float64) {})
		for b := range c.shoal {
			for _, i := range c.shoal[b] {
				t := &g.Tiles[i]
				t.Height += change[i]
				mix(t, float64(t.Soil), gained[i])
				t.Soil += float32(carrying(gained[i]))
			}
		}
		level()
		g.expose()
		g.drain()
		g.height()
		g.tides()
	}
}

// Mud from the rivers. A flat is not a shape the coast happens to have: it is
// the mud a river brings down, carried back and forth by the tide until it
// finds slack water shallow enough to let it fall, and built up there, a few
// millimetres a year, until the ground stands at the high water that covers it
// and no higher (Krone 1962; Friedrichs 2011 has flats' surfaces tracking high
// water as they build). A flat under mean sea keeping only what reached it
// down its own river laid a tongue of mud one tile wide out into deep water, and
// a tongue is not a flat: so what reaches the sea is carried by the tide over
// all the shoals of the body of water it reaches - the tide's excursion is
// kilometres and the map's bays are not - and settles over them by their
// area. Mixed through the deep water as well, the shoals of a small globe got
// a millimetre in a thousand years; most of what the earth's rivers carry is
// kept on its shelves and coasts (Milliman and Syvitski 1992), and what the
// shoals do not trap goes on to the deep water.
//
// A shoal is sea shallower than a spring tide's range below mean sea: the
// water that runs slack over it at the turn of the tide, which the deep water
// never does. What falls into the deep water goes on to the sea.
func (g *Grid) bays(c *fluvial, recv []int32) {
	n := len(g.Tiles)
	in := func(i int) bool {
		return int(recv[i]) == i && g.underSea(i) && g.tidal[i] > 0
	}
	c.bay = make([]int32, n)
	c.trap = make([]float64, n)
	for i := range c.bay {
		c.bay[i] = -1
	}
	c.shoal = nil
	var stack []int
	for start := range n {
		if c.bay[start] >= 0 || !in(start) {
			continue
		}
		b := int32(len(c.shoal))
		var shoal []int32
		c.bay[start] = b
		stack = append(stack[:0], start)
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			t := &g.Tiles[i]
			f := float64(g.tidal[i])
			spring := 2 * f * (TideM2 + TideS2)
			if t.Mark == None && g.sea-t.Height <= spring && !g.Frozen(g.PosOf(i)) {
				c.trap[i] = clamp01(spring / trapRange)
				c.room[i] = math.Max(0, g.sea+f*MeanHigh-t.Height)
				if c.trap[i] > 0 && c.room[i] > 0 {
					shoal = append(shoal, int32(i))
				}
			}
			g.eachNear(i, func(j int) {
				if c.bay[j] < 0 && in(j) {
					c.bay[j] = b
					stack = append(stack, j)
				}
			})
		}
		slices.Sort(shoal) // the order a world repeats in
		c.shoal = append(c.shoal, shoal)
	}
}
