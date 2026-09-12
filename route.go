package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
)

// Dirs are the eight steps out of a tile, in a fixed order. Everything that
// scans neighbours walks them in this order so that runs repeat exactly -
// which is why it is the land's and is exported: a game scanning the ground
// beside a tile has to walk it in the same order or its own runs will not.
var Dirs = [8]geom.Pos{
	{X: -1, Y: -1}, {X: 0, Y: -1}, {X: 1, Y: -1},
	{X: -1, Y: 0}, {X: 1, Y: 0},
	{X: -1, Y: 1}, {X: 0, Y: 1}, {X: 1, Y: 1},
}

// tie is how much two route costs may differ and still count as equal. Costs
// are small sums of tile costs, so anything under this is float noise.
const tie = 1e-9

// Window is how far a route may run from where it starts, in tiles either
// way. It is the width of the default map, which is the farthest anybody
// there has ever had to walk - to the market from the far edge, or to
// whoever posted a request - so the window holds the whole of that map
// from anywhere on it, and on a bigger map holds a settlement and its
// land. A search kept to a window works on a few hundred kilobytes that
// stay in cache, where one kept over the whole of a big map worked on
// megabytes per worker and missed the cache on every tile it opened.
const Window = 80

// Routes is the cheapest walking cost from one tile out over the ground
// around it, on the ground as it currently stands. It is what lets an agent
// use a road: a route is chosen by what it costs to walk, so a paved way
// three tiles off the straight line wins whenever the paving saves more than
// the detour spends.
//
// Only the tiles the search reached carry a cost; gen and seen say which
// those are, so the same buffers can serve search after search without being
// cleared each time. The tiles are kept by their slot in the window, which
// is their offset from its north-west corner; on a globe the offset goes
// round. Slots run row by row like tile indices, so where two routes cost
// the same and start the same way the one to the earlier slot wins, as the
// one to the earlier tile did.
type Routes struct {
	g    *Grid
	from geom.Pos
	gen  int32
	// x0, y0 is the north-west corner of the window, span its side.
	x0, y0, span int
	seen         []int32
	cost         []float64
	prev         []int32
	rank         []int8 // rank of the first step of this tile's route, for tie-breaks
	// limit is the cost the search was told not to go past: a tile it
	// reached at that cost or more it did not settle, and does not report.
	limit float64
}

// slot is where the tile at map position x,y is kept, and whether it is in
// the window at all.
func (f *Routes) slot(x, y int) (int32, bool) {
	if y < 0 || y >= f.g.H {
		return 0, false
	}
	dy := y - f.y0
	if dy < 0 || dy >= f.span {
		return 0, false
	}
	dx := x - f.x0
	if f.g.Wrap {
		dx %= f.g.W
		if dx < 0 {
			dx += f.g.W
		}
	}
	if dx < 0 || dx >= f.span {
		return 0, false
	}
	return int32(dy*f.span + dx), true
}

// at is the map position kept at slot s.
func (f *Routes) at(s int32) geom.Pos {
	p := geom.Pos{X: f.x0 + int(s)%f.span, Y: f.y0 + int(s)/f.span}
	if f.g.Wrap {
		p.X = f.g.WrapX(p.X)
	}
	return p
}

// Router is the working memory one line of routing runs on: the frontier its
// searches build, and the result buffers a search reads its answer straight
// out of. Keeping it here rather than on the Grid means pathing every agent on
// every tick allocates nothing while still leaving the map itself something
// several goroutines can read at once, each routing on a Router of its own.
//
// A Router may not be shared. The Grid it points at may.
type Router struct {
	g        *Grid
	frontier []routeNode
	scratch  Routes
	// spread is the survey a decision routes off: one spread of the ground
	// around the agent, read by every errand it weighs.
	spread      Routes
	spreadFrom  geom.Pos
	spreadLaden bool
	spreadLimit float64
	surveyed    bool
	// limit is the cost past which the next search need not go, set by
	// Survey and spent by the search that follows it.
	limit float64
	// load is what the walker of the next route is carrying, set by Carrying
	// and spent by the search that follows it.
	load float64
	// Work counts the tiles the router has opened since it was last reset:
	// what a decision has cost in looking, for anybody keeping a budget.
	Work int
	// holder is whose ground the walker of these routes may cross for
	// nothing: their own fields are theirs to walk into. Unlike load it is
	// not spent by a search, because a line of routing is run for one agent
	// at a time and every route in it is that agent's; see Holding.
	holder Holder
	// spreadHolder is who the standing survey was spread for, so that a
	// survey is not read back for somebody whose gates are different.
	spreadHolder Holder
}

// Holding tells the router whose walker it is routing for, so that the fence
// round a farmer's own field is a gate to them and a fence to everybody
// else. It stays set until it is set again, which is what a Router is for: one line of
// routing serves one agent at a time.
func (r *Router) Holding(id Holder) *Router {
	r.holder = id
	return r
}

// Holding routes on the grid's own router, for callers working one at a time.
func (g *Grid) Holding(id Holder) *Router {
	return g.ownRouter().Holding(id)
}

// Router returns a router over this grid, for a caller that needs its own.
func (g *Grid) Router() *Router { return &Router{g: g} }

// Within tells the router how far the walker of the next route is prepared
// to go, in ticks of walking: the search stops as soon as everything left
// to open is at least that far, and a destination at least that far reads
// as having no way to it. It holds for the next route this router runs and
// no longer, like Carrying. Zero is any distance.
func (r *Router) Within(limit float64) *Router {
	r.limit = limit
	return r
}

// Within routes on the grid's own router, for callers working one at a
// time.
func (g *Grid) Within(limit float64) *Router {
	return g.ownRouter().Within(limit)
}

// Reset puts the router back as it was before its last search: no survey
// standing, nothing carried, nothing owed. For after a search that did not
// finish, and before a decision that is to be charged from nothing.
func (r *Router) Reset() {
	r.frontier = r.frontier[:0]
	r.surveyed = false
	r.load, r.limit = 0, 0
	r.Work = 0
}

// offMap is a tile no search will meet, for when there is no step to prefer.
var offMap = geom.Pos{X: -1, Y: -1}

// Routes computes the cheapest way from one tile to every tile within the
// window, in a result the caller may keep.
func (r *Router) Routes(from geom.Pos) *Routes {
	return r.route(&Routes{}, from, offMap, false, offMap)
}

// Routes computes the cheapest way from one tile to every tile within the
// window, in a result the caller may keep. It routes on the grid's own
// router, so it is for callers working one at a time.
func (g *Grid) Routes(from geom.Pos) *Routes {
	return g.ownRouter().Routes(from)
}

// activeLandmarks is how many landmarks a guided search reads at every
// step: the ones that bound the walk best from where it starts. Two of
// eight keep nearly all of what eight give, at a quarter of the reading.
const activeLandmarks = 2

// minMoveCost is the cheapest a tile can be to enter. Routing to a known
// destination uses it to see how far the destination could possibly still be,
// which keeps the search from spreading over ground that cannot be on the
// way. It must never overstate the cheapest tile, or routes stop being the
// cheapest ones.
const minMoveCost = 0.5

// routeNode is one entry of the frontier. Ordering is by cost, then by the
// rank of the route's first step, then by slot, so that equal-cost routes
// resolve the same way on every run.
type routeNode struct {
	cost float64
	rank int8
	idx  int32
}

func before(a, b routeNode) bool {
	if a.cost != b.cost {
		return a.cost < b.cost
	}
	if a.rank != b.rank {
		return a.rank < b.rank
	}
	return a.idx < b.idx
}

// route runs Dijkstra out from `from`. It stops early once `stop` is
// settled, if guided. `prefer`, when it is a neighbour of from, is the first
// step that wins ties, which keeps a walker on the straight line when going
// around costs exactly as much as going through.
//
// The frontier is a hand-rolled binary heap of concrete nodes rather than a
// container/heap: this runs for every agent on every tick, and boxing each
// node into an interface would cost more than the search itself.
func (r *Router) route(f *Routes, from, stop geom.Pos, guided bool, prefer geom.Pos) *Routes {
	g := r.g
	// A load is given to one journey and does not outlive it.
	laden := r.load > SwimLoad
	r.load = 0
	limit := r.limit
	r.limit = 0
	if limit <= 0 {
		limit = math.Inf(1)
	}
	span := 2*Window + 1
	if len(f.seen) != span*span {
		f.seen = make([]int32, span*span)
		f.cost = make([]float64, span*span)
		f.prev = make([]int32, span*span)
		f.rank = make([]int8, span*span)
		f.gen = 0
	}
	from = g.Norm(from)
	f.g, f.from, f.gen, f.limit = g, from, f.gen+1, limit
	f.x0, f.y0, f.span = from.X-Window, from.Y-Window, span
	if !g.In(from) {
		return f
	}
	src, _ := f.slot(from.X, from.Y)
	f.seen[src], f.cost[src], f.prev[src], f.rank[src] = f.gen, 0, -1, -1

	// A destination gives the search a direction: a tile is worth opening
	// only by what the route through it has cost so far plus the least the
	// rest could cost. Without one the search simply spreads outward. A
	// destination outside the window is out of reach, and the search does
	// not start.
	stopSlot := int32(-1)
	if guided {
		stop = g.Norm(stop)
		s, ok := f.slot(stop.X, stop.Y)
		if !ok {
			return f
		}
		stopSlot = s
		// A laden walker cannot leave the ground it stands on except into
		// the tile it is going to, so between two pieces of ground there
		// is no way, and the search that would say so is not run. See
		// region.go.
		if laden && !g.Tiles[g.Index(from)].Deep() && !g.reachesLaden(from, stop) {
			return f
		}
	}
	// What the search knows of the walk that is left from any tile, and
	// whether the landmarks say the destination cannot be reached at all,
	// in which case there is nothing to open. See guess.
	var known guess
	if guided {
		var none bool
		if known, none = r.guessFor(from, stop, laden, f.x0, f.y0, span); none {
			return f
		}
	}
	toGo := func(x, y int) float64 {
		if !guided {
			return 0
		}
		return known.at(x, y)
	}
	prefer = g.Norm(prefer)

	q := append(r.frontier[:0], routeNode{rank: -1, idx: src})
	for len(q) > 0 {
		top := q[0]
		last := len(q) - 1
		q[0] = q[last]
		q = q[:last]
		siftDown(q, 0)

		p := f.at(top.idx)
		px, py := p.X, p.Y
		here := f.cost[top.idx]
		if top.cost > here+toGo(px, py)+tie {
			continue // a cheaper route here turned up after this was queued
		}
		if top.idx == stopSlot {
			break
		}
		// Everything still to open is at least as far as this, and what
		// is left to go is never overstated, so nothing nearer than the
		// limit remains to be found.
		if top.cost >= limit {
			break
		}
		r.Work++
		ti := int32(py*g.W + px)
		for d := range Dirs {
			cx, cy := px+Dirs[d].X, py+Dirs[d].Y
			if cy < 0 || cy >= g.H {
				continue
			}
			if g.Wrap {
				cx = g.WrapX(cx)
			} else if cx < 0 || cx >= g.W {
				continue
			}
			j, ok := f.slot(cx, cy)
			if !ok {
				continue
			}
			tj := int32(cy*g.W + cx)
			t := &g.Tiles[tj]
			// Open water is not dear to a laden walker, it is shut. Two
			// things are still allowed through it. The end of the journey
			// itself, because somebody may wade in from the bank to fish, or
			// to stand in the river and build the bridge that ends the
			// exception. And a step out of water into water, because a walker
			// the river has risen under, or whose bridge has gone, has to be
			// able to get out of it; what is forbidden is walking in, not
			// being in.
			if laden && t.Deep() && j != stopSlot && !g.Tiles[ti].Deep() {
				continue
			}
			// The step carries the climb into the tile, which is what makes
			// a route follow a contour rather than go straight over the hill
			// in the way.
			cost := here + stepInto(g, ti, tj) + g.fenceCost(ti, tj, r.holder)
			rank := top.rank
			if top.idx == src {
				rank = int8(d)
				if cx == prefer.X && cy == prefer.Y {
					rank = -1
				}
			}
			if f.seen[j] == f.gen && cost > f.cost[j]-tie && (cost > f.cost[j]+tie || rank >= f.rank[j]) {
				continue
			}
			f.seen[j], f.cost[j], f.prev[j], f.rank[j] = f.gen, cost, top.idx, rank
			left := toGo(cx, cy)
			if math.IsInf(left, 1) {
				continue // no way on from there, so nothing to open it for
			}
			q = append(q, routeNode{cost: cost + left, rank: rank, idx: j})
			siftUp(q, len(q)-1)
		}
	}
	r.frontier = q[:0]
	return f
}

// Survey spreads out from one tile over everything within limit of it, and
// keeps the answer for the errands the caller is about to cost. It is what
// lets an agent weigh thirty errands off one look at the ground rather than
// walking each of them in its head.
func (r *Router) Survey(from geom.Pos, load, limit float64) {
	r.load, r.limit = load, limit
	r.route(&r.spread, from, offMap, false, offMap)
	r.spreadFrom, r.spreadLaden, r.spreadLimit, r.surveyed = r.spread.from, load > SwimLoad, limit, true
	r.spreadHolder = r.holder
}

// Forget drops the survey, so the next cost is walked afresh.
func (r *Router) Forget() { r.surveyed = false }

// fromSurvey reads a cost straight out of the standing survey. Anything the
// survey did not reach is at least limit away, which is all the caller asked
// to be able to tell.
func (r *Router) fromSurvey(to geom.Pos) float64 {
	f := &r.spread
	g := r.g
	i, ok := f.slot(to.X, to.Y)
	if !ok {
		return r.spreadLimit
	}
	if f.seen[i] == f.gen && f.cost[i] < r.spreadLimit {
		return f.cost[i]
	}
	// The end of a journey may be open water even for a laden walker, so a
	// water tile the spread would not step into is costed from its bank.
	if r.spreadLaden && g.Tiles[g.Index(to)].Deep() {
		best := r.spreadLimit
		ti := int32(g.Index(to))
		for d := range Dirs {
			cx, cy := to.X+Dirs[d].X, to.Y+Dirs[d].Y
			if cy < 0 || cy >= g.H {
				continue
			}
			if g.Wrap {
				cx = g.WrapX(cx)
			} else if cx < 0 || cx >= g.W {
				continue
			}
			j, ok := f.slot(cx, cy)
			if !ok || f.seen[j] != f.gen || f.cost[j] >= r.spreadLimit {
				continue
			}
			c := f.cost[j] + stepInto(g, int32(cy*g.W+cx), ti) + g.fenceCost(int32(cy*g.W+cx), ti, r.holder)
			if c < best {
				best = c
			}
		}
		return best
	}
	return r.spreadLimit
}

// stepInto is what entering one tile from a neighbour costs: the ground
// being entered, and the climb or the descent into it.
func stepInto(g *Grid, from, to int32) float64 {
	t := &g.Tiles[to]
	step := moveCost[t.Terrain]
	if t.Mark != None {
		step = markCost[t.Mark]
	}
	if d := t.Height - g.Tiles[from].Height; d > 0 {
		step += Climb * d
	} else {
		step -= Descend * d
	}
	return step
}

func siftUp(q []routeNode, i int) {
	for i > 0 {
		p := (i - 1) / 2
		if !before(q[i], q[p]) {
			return
		}
		q[i], q[p] = q[p], q[i]
		i = p
	}
}

func siftDown(q []routeNode, i int) {
	for {
		l, best := 2*i+1, i
		if l < len(q) && before(q[l], q[best]) {
			best = l
		}
		if r := l + 1; r < len(q) && before(q[r], q[best]) {
			best = r
		}
		if best == i {
			return
		}
		q[i], q[best] = q[best], q[i]
		i = best
	}
}

// reached is the slot of p if the search settled it, else false.
func (f *Routes) reached(p geom.Pos) (int32, bool) {
	if f.g == nil || !f.g.In(p) {
		return 0, false
	}
	p = f.g.Norm(p)
	i, ok := f.slot(p.X, p.Y)
	if !ok || f.seen[i] != f.gen || f.cost[i] >= f.limit {
		return 0, false
	}
	return i, true
}

// Cost is the ticks of walking from the origin to p, or +Inf if there is no
// way there within the window.
func (f *Routes) Cost(p geom.Pos) float64 {
	i, ok := f.reached(p)
	if !ok {
		return math.Inf(1)
	}
	return f.cost[i]
}

// Step is the first tile of the cheapest route to p. It returns the origin
// itself when p is the origin or cannot be reached.
func (f *Routes) Step(p geom.Pos) geom.Pos {
	i, ok := f.reached(p)
	if !ok || f.prev[i] < 0 {
		return f.from
	}
	src, _ := f.slot(f.from.X, f.from.Y)
	for f.prev[i] != src {
		i = f.prev[i]
	}
	return f.at(i)
}

// Path is the cheapest route to p, origin excluded and p included. It is
// empty when p cannot be reached.
func (f *Routes) Path(p geom.Pos) []geom.Pos {
	i, ok := f.reached(p)
	if !ok || f.prev[i] < 0 {
		return nil
	}
	src, _ := f.slot(f.from.X, f.from.Y)
	var out []geom.Pos
	for i != src {
		out = append(out, f.at(i))
		i = f.prev[i]
	}
	for l, r := 0, len(out)-1; l < r; l, r = l+1, r-1 {
		out[l], out[r] = out[r], out[l]
	}
	return out
}
