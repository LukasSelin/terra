package terra

import (
	"cmp"
	"math"
	"slices"

	"github.com/LukasSelin/terra/geom"
)

// Standing water, and the way the water goes.
//
// A hollow in the ground fills until it spills over the lowest point of its
// rim, or until the air takes off its surface as much as runs into it,
// whichever comes first. That is the whole rule, and it is what decides
// whether a hollow is a lake with a river out of it, a salt lake with no way
// out at all, or a dry crust of salt that holds water for a week after a
// storm. Wet country fills everything it has and every lake in it drains to
// the sea; dry country keeps its water where it falls.
//
// This used to be done by raising the ground. Every hollow was filled in to
// the height it would spill at and the filled height was written over the
// real one, so that the floor of a lake was gone and in its place was a flat
// of ground a hair of a millimetre per tile out of level, with a line of
// water ruled across it wherever the flow happened to go. Nothing ever held
// water that it did not pass on, and every epoch of a history filled in
// whatever the last one had hollowed out, for good. Now the ground is the
// ground, a lake has a level over it, and the water crossing a lake goes in
// at one side and out at its outlet as one body.
//
// The hollows are worked out as the tree they form: each one is a basin,
// and two basins that meet at a saddle are one bigger basin above that
// saddle's height. The water is then poured in - what runs into each hollow
// goes into it, what a full one cannot hold spills over its saddle into the
// basin beside it, and what two full basins cannot hold goes up into the one
// they make together - and a basin holds what its surface gives back to the
// air. This is the depression hierarchy and Fill-Spill-Merge of Barnes,
// Callaghan and Wickert (Earth Surface Dynamics, 2020), balanced here on what
// a lake's surface evaporates rather than on what its volume holds, because
// what is wanted is the lake a climate keeps and not the lake one storm
// leaves.

// LakeDepth is how deep a hollow has to be at its deepest, in metres, before
// the water standing in it is a lake. The ground is a field of noise, and a
// field of noise has a dimple in it every few tiles that would hold a puddle;
// a puddle is not a lake, and its water goes on across it as though it were
// not there.
const LakeDepth = 1.0

// saltShore is how far above the water of a lake with no outlet its salt flat
// reaches, in metres: the ground a wetter year floods and a dry one leaves
// white.
const saltShore = 2.0

// Lake is one body of standing water.
type Lake struct {
	// Level is the height of its surface, in metres.
	Level float64
	// Outlet is the tile its water leaves by, or -1 if nothing leaves it but
	// what the air takes: a salt lake, or a dry one.
	Outlet int32
	// Closed is whether it has no outlet.
	Closed bool
	// Tiles is how many tiles lie under its water, and Floor how many of salt
	// flat lie round it. A dry lake is a closed one with no tiles of water.
	Tiles, Floor int
	// Inflow is the water that reaches it, in cubic metres a second.
	Inflow float64
}

// Surface is the height of whatever is on top at tile i: the water, where it
// lies under a lake, and otherwise the ground. It is what a walker crossing
// the tile stands on or swims at, and what the water running off it runs off.
func (g *Grid) Surface(i int) float64 {
	h := g.Tiles[i].Height
	if len(g.lakeOf) == len(g.Tiles) && g.lakeOf[i] >= 0 && g.lakeLevel[i] > h {
		return g.lakeLevel[i]
	}
	return h
}

// Downstream is where the water at p goes next, and whether it goes anywhere:
// the next tile down, or the outlet of the lake p lies under. It does not go
// on at the sea, off the edge of a valley, into a lake with no outlet or onto
// a salt flat.
func (g *Grid) Downstream(p geom.Pos) (geom.Pos, bool) {
	if len(g.down) != len(g.Tiles) || !g.In(p) {
		return geom.Pos{}, false
	}
	d := g.down[g.Index(p)]
	if d < 0 {
		return geom.Pos{}, false
	}
	return g.PosOf(int(d)), true
}

// LakeAt is the lake the tile at p lies under, if it lies under one.
func (g *Grid) LakeAt(p geom.Pos) (*Lake, bool) {
	if len(g.lakeOf) != len(g.Tiles) || !g.In(p) {
		return nil, false
	}
	k := g.lakeOf[g.Index(p)]
	if k < 0 {
		return nil, false
	}
	return &g.Lakes[k], true
}

// closedLake reports whether tile i lies under a lake with no outlet.
func (g *Grid) closedLake(i int) bool {
	if len(g.lakeOf) != len(g.Tiles) || g.lakeOf[i] < 0 {
		return false
	}
	return g.Lakes[g.lakeOf[i]].Closed
}

// standing reports whether tile i is still water or a salt flat: somewhere
// the water stops, and whatever it is carrying stops with it.
func (g *Grid) standing(i int) bool {
	if len(g.lakeOf) != len(g.Tiles) {
		return false
	}
	return g.lakeOf[i] >= 0 || g.pans[i]
}

// flowStep is the step from tile i to the tile its water goes to, or the
// zero step where that is not a neighbour: nowhere, or across a lake.
func (g *Grid) flowStep(i int) geom.Pos {
	if len(g.down) != len(g.Tiles) {
		return g.Aspect(g.PosOf(i))
	}
	d := g.down[i]
	if d < 0 {
		return geom.Pos{}
	}
	step := g.Delta(g.PosOf(i), g.PosOf(int(d)))
	if step.X < -1 || step.X > 1 || step.Y < -1 || step.Y > 1 {
		return geom.Pos{}
	}
	return step
}

// loss is what the air takes off tile i in a year where it lies under open
// water, over and above what it takes off the ground there anyway, in mm:
// the evaporation of open water, less what the ground was already giving
// back. Counted that way, what runs off every tile - see Grid.Runoff - can be
// counted the same whether or not the tile ends up wet, and a lake's surface
// takes the rest.
func (g *Grid) loss(i int) float64 {
	if len(g.rain) != len(g.Tiles) || g.sunk(i) || g.air == nil {
		return 0
	}
	pet := g.pet(i)
	return math.Max(0, pet-(g.rain[i]-g.runoff[i]))
}

// basin is one hollow in the tree of them. The sea and the edges of the map
// are basin nought, which never fills.
type basin struct {
	// parent is the basin this one merges into above its spill height, -1
	// until it has merged; kids are the two it was made of, or -1 for a
	// hollow with nothing under it.
	parent int32
	kids   [2]int32
	// spill is the height it merges at.
	spill float64
	// saddle is the tile on its sibling's side of the saddle it spills
	// over, and entry the basin that water enters by, worked out from it.
	saddle, entry int32
	// capacity is how much water a year the basin's surface gives the air
	// when it stands full; held is how much is standing in it.
	capacity, held float64
	// own is where this basin's own tiles - the ones it took in itself,
	// rather than through a basin it was made of - lie in basins.tiles.
	first, end int32
}

// basins is the tree of hollows over a map.
type basins struct {
	b     []basin
	uf    []int32 // which basin each basin has since been merged into
	tiles []int32 // every basin's own tiles, lowest first, basin by basin
}

func (t *basins) find(x int32) int32 {
	for t.uf[x] != x {
		t.uf[x] = t.uf[t.uf[x]]
		x = t.uf[x]
	}
	return x
}

func (t *basins) full(x int32) bool {
	if x <= 0 {
		return false
	}
	return t.b[x].held >= t.b[x].capacity*(1-1e-12)
}

// drain works out where the water stands and where it goes, from the ground
// as it now is: the lakes, the salt flats, which way each tile's water leaves,
// and how much of the map's water passes through each tile, as Flow.
//
// What each tile starts with is its own runoff, off the ground the tile is -
// see weather.go - and alongside the water it counts the ground: area is how
// many tiles drain through each one, which is what the guards against the
// grid's own patterns are read in. See spreadUntil.
func (g *Grid) drain() {
	defer phase("drain")()
	g.weather()
	g.pool()
	g.flow()
}

// pool finds every hollow on the map and how full the weather keeps it, and
// writes down the lakes and the salt flats that leaves.
func (g *Grid) pool() {
	defer phase("pool")()
	n := len(g.Tiles)
	order := make([]heightNode, n)
	for i := range order {
		order[i] = heightNode{h: g.Tiles[i].Height, idx: int32(i)}
	}
	slices.SortFunc(order, func(a, b heightNode) int {
		if a.h != b.h {
			return cmp.Compare(a.h, b.h)
		}
		return cmp.Compare(a.idx, b.idx)
	})

	// The tree, lowest ground first. A tile with nothing lower beside it
	// already taken in starts a hollow of its own; one beside a single
	// hollow joins it; and one beside two is the saddle between them, where
	// they become one. The sea and the edges of the map are one basin from
	// the start, at whatever height each of them stands.
	t := &basins{b: []basin{{parent: -1, kids: [2]int32{-1, -1}, spill: math.Inf(1)}}, uf: []int32{0}}
	own := make([]int32, n)
	for i := range own {
		own[i] = -1
	}
	type side struct{ root, low int32 }
	var sides []side
	for _, nd := range order {
		i := int(nd.idx)
		p := g.PosOf(i)
		sides = sides[:0]
		for _, off := range Dirs {
			q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
			if !g.In(q) {
				continue
			}
			j := int32(g.Index(q))
			if own[j] < 0 {
				continue
			}
			r := t.find(own[j])
			found := false
			for k := range sides {
				if sides[k].root == r {
					if g.Tiles[j].Height < g.Tiles[sides[k].low].Height {
						sides[k].low = j
					}
					found = true
					break
				}
			}
			if !found {
				sides = append(sides, side{r, j})
			}
		}
		cur, low := int32(-1), int32(-1)
		if g.sunk(i) || g.outlet(p.X, p.Y) {
			cur = 0 // and its water leaves from this tile: no low to go by
		}
		for _, s := range sides {
			switch {
			case cur < 0:
				cur, low = s.root, s.low
			case s.root == cur:
			case cur == 0 || s.root == 0:
				// Toward the sea, or the edge: over this saddle the water
				// leaves, by way of whatever lies between here and there.
				x, across := cur, s.low
				if cur == 0 {
					x, across = s.root, low
				}
				t.b[x].parent, t.b[x].spill, t.b[x].saddle = 0, nd.h, across
				t.uf[x] = 0
				if low < 0 || (s.low >= 0 && g.Tiles[s.low].Height < g.Tiles[low].Height) {
					low = s.low
				}
				cur = 0
			default:
				p := int32(len(t.b))
				t.b = append(t.b, basin{parent: -1, kids: [2]int32{cur, s.root}, spill: math.Inf(1), saddle: -1})
				t.uf = append(t.uf, p)
				t.b[cur].parent, t.b[cur].spill, t.b[cur].saddle = p, nd.h, s.low
				t.b[s.root].parent, t.b[s.root].spill, t.b[s.root].saddle = p, nd.h, low
				t.uf[cur], t.uf[s.root] = p, p
				if g.Tiles[s.low].Height < g.Tiles[low].Height {
					low = s.low
				}
				cur = p
			}
		}
		if cur < 0 {
			cur = int32(len(t.b))
			t.b = append(t.b, basin{parent: -1, kids: [2]int32{-1, -1}, spill: math.Inf(1), saddle: -1})
			t.uf = append(t.uf, cur)
		}
		own[i] = cur
	}
	// Anything that never reached the sea or an edge - a map with neither -
	// holds whatever reaches it, however much that is.
	for x := 1; x < len(t.b); x++ {
		if t.b[x].parent < 0 {
			t.b[x].parent = 0
		}
	}

	// Each basin's own tiles, lowest first: the order they were taken in.
	count := make([]int32, len(t.b)+1)
	for i := range own {
		count[own[i]+1]++
	}
	for x := 1; x <= len(t.b); x++ {
		count[x] += count[x-1]
	}
	t.tiles = make([]int32, n)
	for x := range t.b {
		t.b[x].first, t.b[x].end = count[x], count[x]
	}
	for _, nd := range order {
		x := own[nd.idx]
		t.tiles[t.b[x].end] = nd.idx
		t.b[x].end++
	}

	// What reaches each hollow, and what a full one gives back. The water
	// runs down the steepest fall of the ground itself to the bottom of
	// whatever hollow it is in. What falls on a tile that ends up under
	// water reaches the lake whole, and the lake gives back what the air
	// takes; accounted as what runs off the tile plus what the lake gives
	// back over and above that, the two come to the same thing, and every
	// tile can be counted the same way whether or not it ends up wet.
	runoff := make([]float64, n)
	loss := make([]float64, n)
	for i := range g.Tiles {
		runoff[i], loss[i] = g.runoff[i], g.loss(i)
	}
	for x := 1; x < len(t.b); x++ {
		b := &t.b[x]
		for _, k := range b.kids {
			if k >= 0 {
				b.capacity += t.b[k].capacity
			}
		}
		for _, j := range t.tiles[b.first:b.end] {
			if g.Tiles[j].Height >= b.spill {
				break
			}
			b.capacity += loss[j]
		}
	}
	fall := make([]int32, n)
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			p := g.PosOf(i)
			a := g.Aspect(p)
			if a == (geom.Pos{}) {
				fall[i] = -1
				continue
			}
			fall[i] = int32(g.Index(geom.Pos{X: p.X + a.X, Y: p.Y + a.Y}))
		}
	})
	// Where the steepest fall from each tile ends, lowest ground first so
	// that the tile below is always settled before the one above it.
	bottom := make([]int32, n)
	for _, nd := range order {
		if f := fall[nd.idx]; f >= 0 {
			bottom[nd.idx] = bottom[f]
		} else {
			bottom[nd.idx] = nd.idx
		}
	}
	// Where each basin's overflow goes: down the fall from the far side of
	// its saddle, to the bottom of whatever hollow that is in. Nowhere, if
	// the far side is the sea or the edge itself.
	for x := 1; x < len(t.b); x++ {
		b := &t.b[x]
		if b.parent < 0 || (b.parent == 0 && b.saddle < 0) {
			continue
		}
		if e := own[bottom[b.saddle]]; !t.within(e, int32(x)) {
			b.entry = e
		}
	}
	gathered := make([]float64, n)
	for k := n - 1; k >= 0; k-- {
		i := order[k].idx
		gathered[i] += runoff[i]
		if f := fall[i]; f >= 0 {
			gathered[f] += gathered[i]
		} else if own[i] > 0 {
			t.pour(own[i], gathered[i])
		}
	}

	// Where the water stands, from the top of the tree down: a full basin is
	// a lake at its spill height, spilling into its neighbour or out to the
	// sea; one that is not full but whose hollows all are is a lake at the
	// height its surface gives back what it holds; and one with hollows not
	// yet full is those hollows.
	if len(g.lakeLevel) != n {
		g.lakeLevel, g.lakeOf, g.pans = make([]float64, n), make([]int32, n), make([]bool, n)
	}
	for i := range g.lakeLevel {
		g.lakeLevel[i], g.lakeOf[i], g.pans[i] = -1, -1, false
	}
	g.Lakes = g.Lakes[:0]
	var stack []int32
	for x := int32(len(t.b)) - 1; x > 0; x-- {
		if t.b[x].parent == 0 {
			stack = append(stack, x)
		}
	}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		b := &t.b[x]
		kids := b.kids[0] < 0 || (t.full(b.kids[0]) && t.full(b.kids[1]))
		switch {
		case t.full(x):
			g.stand(t, x, b.spill, false)
		case kids:
			g.stand(t, x, t.levelOf(x, g), true)
		default:
			stack = append(stack, b.kids[1], b.kids[0])
		}
	}
}

// pour puts w of water into basin x, where it runs down into whichever of the
// hollows under x are not yet full, fills them, and goes over the saddle into
// the basin beside them when they are. What gets to the sea or off the map is
// gone.
func (t *basins) pour(x int32, w float64) {
	// Every spill goes over a saddle into ground that was joined up lower
	// down, so the water cannot come back round; the count is only there so
	// that a mistake in that is a lost lake and not a hung world.
	for guard := 4*len(t.b) + 8; w > 0 && x > 0 && guard > 0; guard-- {
		for t.b[x].kids[0] >= 0 {
			if k := t.b[x].kids[0]; !t.full(k) {
				x = k
			} else if k := t.b[x].kids[1]; !t.full(k) {
				x = k
			} else {
				break
			}
		}
		take := math.Min(w, math.Max(0, t.b[x].capacity-t.b[x].held))
		for m := x; m > 0; m = t.b[m].parent {
			t.b[m].held += take
		}
		w -= take
		if w <= 0 {
			return
		}
		// Full, so over the saddle: into the hollow beside it if that has
		// room, and up into the two together if it has not.
		p := t.b[x].parent
		if p <= 0 {
			// Toward the sea: through any hollow on the way that joined it
			// lower down, and gone if there is none.
			if e := t.b[x].entry; p == 0 && e > 0 {
				x = e
				continue
			}
			return
		}
		sib := t.b[p].kids[0]
		if sib == x {
			sib = t.b[p].kids[1]
		}
		if !t.full(sib) {
			// The saddle is the highest ground x could spill over; the tile
			// it spills onto is in its sibling. Should the water's path from
			// there not reach one of the sibling's own hollows, it goes up
			// into the pair.
			if e := t.b[x].entry; e > 0 && t.within(e, sib) {
				x = e
				continue
			}
			x = sib
			continue
		}
		x = p
	}
}

// within reports whether basin x is basin top or one of those it was made of.
func (t *basins) within(x, top int32) bool {
	for ; x > 0; x = t.b[x].parent {
		if x == top {
			return true
		}
	}
	return false
}

// levelOf is the height a basin that is not full stands at: its hollows all
// full, and its own tiles taken in lowest first until their surface gives the
// air what is left.
func (t *basins) levelOf(x int32, g *Grid) float64 {
	b := &t.b[x]
	left := b.held
	floor := math.Inf(-1)
	for _, k := range b.kids {
		if k >= 0 {
			left -= t.b[k].capacity
			floor = math.Max(floor, t.b[k].spill)
		}
	}
	own := t.tiles[b.first:b.end]
	for _, j := range own {
		h := g.Tiles[j].Height
		if h >= b.spill {
			return b.spill
		}
		l := g.loss(int(j))
		if left < l/2 {
			return math.Max(floor, h)
		}
		left -= l
	}
	if len(own) > 0 {
		return math.Min(b.spill, g.Tiles[own[len(own)-1]].Height)
	}
	return floor
}

// stand puts the water of basin x at level, over every tile under it that
// lies lower. A lake too shallow to be one is left as the ground it is; a
// closed basin deep enough to matter gets its salt flat whether or not any
// water is standing in it.
func (g *Grid) stand(t *basins, x int32, level float64, closed bool) {
	// The ground taken in: under the water, and for a closed basin up to
	// saltShore above it, short of the rim.
	spill := t.b[x].spill
	top := level
	if closed {
		top = math.Min(spill, level+saltShore)
	}
	var under, shore []int32
	floor := math.Inf(1)
	stack := []int32{x}
	for len(stack) > 0 {
		m := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, j := range t.tiles[t.b[m].first:t.b[m].end] {
			h := g.Tiles[j].Height
			if h >= top {
				break
			}
			floor = math.Min(floor, h)
			if h < level {
				under = append(under, j)
			} else {
				shore = append(shore, j)
			}
		}
		for _, k := range t.b[m].kids {
			if k >= 0 {
				stack = append(stack, k)
			}
		}
	}
	if math.IsInf(floor, 1) {
		return
	}
	if !closed {
		if level-floor < LakeDepth {
			return
		}
		g.Lakes = append(g.Lakes, Lake{Level: level, Outlet: -1, Tiles: len(under)})
		k := int32(len(g.Lakes) - 1)
		for _, j := range under {
			g.lakeLevel[j], g.lakeOf[j] = level, k
		}
		return
	}
	// A closed basin shallower than a lake is ground with a dip in it, wet or
	// dry. Deeper, it is a salt lake with a flat round it if water stands in
	// it deep enough to be one, and a flat and nothing else if not.
	if math.IsInf(spill, 1) {
		spill = top
	}
	if spill-floor < LakeDepth {
		return
	}
	wet := level-floor >= LakeDepth
	g.Lakes = append(g.Lakes, Lake{Level: level, Outlet: -1, Closed: true})
	k := int32(len(g.Lakes) - 1)
	l := &g.Lakes[k]
	if !wet {
		l.Level = floor
		shore = append(shore, under...)
		under = nil
	}
	for _, j := range under {
		g.lakeLevel[j], g.lakeOf[j] = level, k
		l.Tiles++
	}
	for _, j := range shore {
		g.pans[j] = true
		l.Floor++
	}
}

// floodNode is a tile waiting in the flood that settles which way the water
// goes, by the height the water there stands at and then by when it was
// reached, so that a flat is crossed outward from where it was entered.
type floodNode struct {
	h   float64
	seq int32
	idx int32
}

// flow sends every tile's water somewhere and adds up what passes through.
//
// It floods inward from everywhere the water stops - the sea, the edges of a
// valley, the lakes with no outlet and the salt flats - always from the
// lowest water reached so far, which is the order water itself would come
// back up a landscape in. A tile with lower ground beside it sends its water
// down the steepest fall to it, as it always has. A tile with nothing lower -
// the flat of a hollow too shallow to be a lake, the top of a rim - sends it
// back the way the flood came. And every tile of a lake that has an outlet
// sends its water to that outlet, which is the first place the flood came
// into the lake from, because a lake is one body and not a field of tiles.
//
// Each tile is reached after the tile its water goes to, so the order it was
// reached in, backwards, is an order in which everything above a tile is
// finished before the tile is.
func (g *Grid) flow() {
	defer phase("flow")()
	n := len(g.Tiles)
	stand := make([]float64, n) // the height the water stands at, flooded
	reached := make([]bool, n)
	from := make([]int32, n)
	if len(g.down) != n {
		g.down, g.route = make([]int32, n), make([]int32, 0, n)
	}
	g.route = g.route[:0]
	var q floodQueue
	seq := int32(0)
	for i := range g.Tiles {
		p := g.PosOf(i)
		closed := g.lakeOf[i] >= 0 && g.Lakes[g.lakeOf[i]].Closed
		if g.sunk(i) || g.outlet(p.X, p.Y) || closed || g.pans[i] {
			stand[i], reached[i], from[i] = g.Surface(i), true, -1
			q.push(floodNode{h: stand[i], seq: seq, idx: int32(i)})
			seq++
		}
	}
	for q.len() > 0 {
		nd := q.pop()
		g.route = append(g.route, nd.idx)
		p := g.PosOf(int(nd.idx))
		for _, off := range Dirs {
			c := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
			if !g.In(c) {
				continue
			}
			j := g.Index(c)
			if reached[j] {
				continue
			}
			stand[j], reached[j], from[j] = math.Max(g.Surface(j), nd.h), true, nd.idx
			q.push(floodNode{h: stand[j], seq: seq, idx: int32(j)})
			seq++
		}
	}

	exit := make([]int32, len(g.Lakes))
	for k := range exit {
		exit[k] = -1
	}
	for _, i := range g.route {
		if k := g.lakeOf[i]; k >= 0 && !g.Lakes[k].Closed && exit[k] < 0 {
			exit[k] = from[i]
		}
	}
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			if k := g.lakeOf[i]; k >= 0 {
				g.down[i] = -1
				if !g.Lakes[k].Closed {
					g.down[i] = exit[k]
				}
				continue
			}
			p := g.PosOf(i)
			if g.sunk(i) || g.pans[i] {
				g.down[i] = -1
				continue
			}
			// The steepest fall of the water's own surface, divided by how
			// far off the neighbour is: see Aspect.
			best, steepest := int32(-1), 0.0
			for _, off := range Dirs {
				c := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
				if !g.In(c) {
					continue
				}
				run := 1.0
				if off.X != 0 && off.Y != 0 {
					run = math.Sqrt2
				}
				j := g.Index(c)
				if d := (stand[i] - stand[j]) / run; d > steepest {
					best, steepest = int32(j), d
				}
			}
			if best < 0 {
				best = from[i]
			}
			g.down[i] = best
		}
	})
	for k := range g.Lakes {
		g.Lakes[k].Outlet = exit[k]
	}

	// Adding it up. What runs off each tile goes down - spread over every
	// lower neighbour while it is a sheet on a hillside, and to the one it
	// goes to alone once it has gathered; see spreadUntil - and what reaches
	// an open lake goes out of its outlet less what the lake's surface gives
	// the air. What reaches a closed lake or a salt flat stays there.
	if len(g.area) != n {
		g.area = make([]float64, n)
	}
	perMM := discharge(1, g.span())
	water := 0.0
	for i := range g.Tiles {
		g.Tiles[i].Flow, g.area[i] = 0, 0
		if !g.underSea(i) {
			g.Tiles[i].Flow, g.area[i] = g.runoff[i]*perMM, 1
			water += g.Tiles[i].Flow
		}
	}
	g.water = water
	pooled := make([]float64, len(g.Lakes))
	pooledArea := make([]float64, len(g.Lakes))
	given := make([]float64, len(g.Lakes))
	for i := range g.Tiles {
		if k := g.lakeOf[i]; k >= 0 {
			given[k] += g.loss(i) * perMM
		}
	}
	// Which lakes spill out through each tile, so their water can be added
	// to it before it is passed on.
	outs := make(map[int32][]int32)
	for k, e := range exit {
		if e >= 0 {
			outs[e] = append(outs[e], int32(k))
		}
	}
	var share [8]float64
	var to [8]int32
	for k := len(g.route) - 1; k >= 0; k-- {
		i := g.route[k]
		t := &g.Tiles[i]
		for _, l := range outs[i] {
			t.Flow += math.Max(0, pooled[l]-given[l])
			g.area[i] += pooledArea[l]
		}
		if l := g.lakeOf[i]; l >= 0 {
			pooled[l] += t.Flow
			pooledArea[l] += g.area[i]
			continue
		}
		d := g.down[i]
		if d < 0 {
			continue
		}
		if g.area[i] >= spreadUntil {
			g.Tiles[d].Flow += t.Flow
			g.area[d] += g.area[i]
			continue
		}
		// A sheet: every lower neighbour, by how far it falls. Lower is read
		// off where the water stands, so that every neighbour it spreads to
		// was reached before this tile and is added up after it.
		p := g.PosOf(int(i))
		m, sum := 0, 0.0
		for _, off := range Dirs {
			q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
			if !g.In(q) {
				continue
			}
			j := g.Index(q)
			drop := stand[i] - stand[j]
			if drop <= 0 {
				continue
			}
			if off.X != 0 && off.Y != 0 {
				drop /= math.Sqrt2
			}
			share[m], to[m] = drop, int32(j)
			sum += share[m]
			m++
		}
		if sum <= 0 {
			g.Tiles[d].Flow += t.Flow
			g.area[d] += g.area[i]
			continue
		}
		for q := 0; q < m; q++ {
			g.Tiles[to[q]].Flow += t.Flow * share[q] / sum
			g.area[to[q]] += g.area[i] * share[q] / sum
		}
	}
	for i := range g.Tiles {
		if l := g.lakeOf[i]; l >= 0 {
			g.Tiles[i].Flow, g.area[i] = pooled[l], pooledArea[l]
		}
	}
	for k := range g.Lakes {
		g.Lakes[k].Inflow = pooled[k]
	}
}

// floodQueue is a smallest-first heap of floodNodes.
type floodQueue []floodNode

func (q *floodQueue) len() int { return len(*q) }

func floodBefore(a, b floodNode) bool {
	if a.h != b.h {
		return a.h < b.h
	}
	return a.seq < b.seq
}

func (q *floodQueue) push(n floodNode) {
	*q = append(*q, n)
	i := len(*q) - 1
	for i > 0 {
		p := (i - 1) / 2
		if !floodBefore((*q)[i], (*q)[p]) {
			break
		}
		(*q)[i], (*q)[p] = (*q)[p], (*q)[i]
		i = p
	}
}

func (q *floodQueue) pop() floodNode {
	old := *q
	top := old[0]
	last := len(old) - 1
	old[0] = old[last]
	old = old[:last]
	*q = old
	i := 0
	for {
		l, best := 2*i+1, i
		if l < len(old) && floodBefore(old[l], old[best]) {
			best = l
		}
		if r := l + 1; r < len(old) && floodBefore(old[r], old[best]) {
			best = r
		}
		if best == i {
			break
		}
		old[i], old[best] = old[best], old[i]
		i = best
	}
	return top
}
