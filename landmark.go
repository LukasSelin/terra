package terra

import (
	"github.com/LukasSelin/terra/geom"
	"math"
)

// Landmarks are a few tiles of the map with the cost of walking to each of
// them from everywhere else written down, so that how far apart any two
// tiles are can be bounded from below without walking between them. The
// walk from a to b is at least the walk from a to a landmark less the walk
// from b to the same landmark, because going by way of b is one way of
// getting there. A guided search reads the bound as its estimate of what is
// left, and with a landmark somewhere beyond the destination the estimate
// sees the lake in the way, where the straight-line estimate sees nothing
// until the search has opened the lake tile by tile. See route.
//
// The tables hold for the ground as it stood when they were taken. Ground
// that gets dearer to cross does not trouble them - a bound below the old
// cost is below the new - but where a step gets cheaper, a road laid or a
// wood cleared, a bound read across it could overstate, so every saving is
// added up by chunk and taken off each bound read over a window that
// touches the chunk, until the tables are taken again. Deep water opening
// to a laden walker, by a bridge or by ice or by the river moving, is a
// saving with no size, so it puts the laden tables out of use instead; and
// an age of weather moves the ground itself, so it puts them all out.
type Landmarks struct {
	at []geom.Pos
	// free holds, for tile i and landmark k at i*len(at)+k, the cost of
	// walking from i to the landmark with nothing much in hand; laden the
	// same carrying a load, when deep water is shut. Kept as float32 for
	// the room; what the rounding could overstate, the reader takes off.
	free, laden []float32
	built       bool
	ladenOK     bool
	stale       bool
	// slack is, by chunk, how much cheaper a step onto the ground there may
	// have become since the tables were taken.
	slack []float64
	// Builds counts the times the tables have been taken, for anybody
	// asking what they cost, and taken is the tick they were last taken.
	Builds int
	taken  int
	// dists and heaps are the working memory the tables are taken on, one
	// of each to a goroutine, kept between takings.
	dists [][]float64
	heaps [][]routeNode
}

// LandmarkCount is how many landmarks a map is given. A bound is only as
// good as its best landmark, which is one that lies beyond the destination
// as seen from the walker, so more of them round the edges of the map give
// more directions a good bound. Each costs a table over the whole map to
// take and to keep.
const LandmarkCount = 8

// LandmarkSlack is how much cheaper the ground in any one chunk may have
// become before the tables are taken again. A window's bounds carry the
// slack of every chunk it touches, so at this much in one of them the
// bounds have lost a few tiles' worth of walking, which is about what
// taking the tables again is worth.
const LandmarkSlack = 3.0

// LandmarkRest is the fewest days between one taking of the tables and the
// next on account of slack or of water moving: a year. Taking them costs a
// walk over the whole map for every landmark, twice, and on a globe of a
// hundred settlements some chunk has laid its few roads every few days;
// taken as often as that the tables cost more than they save. Ground that
// has moved under them is another matter: those are taken again at once.
const LandmarkRest = 360

// halfULP32 bounds the rounding of a float64 into a float32, relative to
// the number rounded. A bound is a difference of two rounded numbers, so
// it is taken down by this much of their sum.
const halfULP32 = 1.0 / (1 << 23)

// enterCost is what stepping onto flat ground of this kind costs.
func enterCost(t *Tile) float64 {
	if t.Mark != None {
		return markCost[t.Mark]
	}
	return moveCost[t.Terrain]
}

// tableCost is what the tables charge for stepping onto flat ground of this
// kind: what it costs, or what grass costs if that is less. Dry ground that
// is dearer than grass - a wood, a field, a house, an outcrop - is charged
// as grass, so that clearing the wood or taking the house down, which make
// the ground cheaper, leave the tables as good as they were. Water and ice
// are charged as they are, because a swim is most of what a bound is for,
// and the water only moves with an age of weather or a bridge. So only a
// road, which is cheaper than grass, puts slack on the tables.
func tableCost(t *Tile) float64 {
	c := enterCost(t)
	if t.Wet() && t.Mark == None {
		return c
	}
	return math.Min(c, moveCost[Grass])
}

// tableStep is stepInto as the tables charge it: the ground entered at
// tableCost, and the climb or the descent exactly as a walker pays it.
func tableStep(g *Grid, from, to int32) float64 {
	t := &g.Tiles[to]
	step := tableCost(t)
	if d := t.Height - g.Tiles[from].Height; d > 0 {
		step += Climb * d
	} else {
		step -= Descend * d
	}
	return step
}

// cheapened notes that the ground at chunk c has gone from costing was to
// costing now to step onto. Only a saving counts.
func (l *Landmarks) cheapened(c int, was, now float64) {
	if now < was && c < len(l.slack) {
		l.slack[c] += was - now
	}
}

// moved notes that the ground itself has changed shape, so that nothing in
// the tables can be relied on until they are taken again.
func (l *Landmarks) moved() { l.stale = true }

// usable reports whether a search may read the tables at all.
func (l *Landmarks) usable() bool { return l.built && !l.stale }

// RefreshLandmarks takes the landmark tables again where they are missing
// or the ground has moved under them; and, once LandmarkRest days have
// passed since they were last taken, where deep water has opened or the
// saving they carry has grown past LandmarkSlack in some chunk. tick is
// today. It writes what every search reads, so it is for a moment when
// nothing is routing: the start of the day's deciding.
func (g *Grid) RefreshLandmarks(tick int) {
	l := &g.landmarks
	if l.built && !l.stale && len(l.slack) == len(g.Chunks) {
		if tick-l.taken < LandmarkRest {
			return
		}
		worst := 0.0
		for _, s := range l.slack {
			worst = math.Max(worst, s)
		}
		if worst < LandmarkSlack && l.ladenOK {
			return
		}
	}
	g.buildLandmarks(tick)
}

// Landmarks is where the map's landmarks stand, in the order they were
// chosen. Empty until the tables have been taken.
func (g *Grid) Landmarks() []geom.Pos { return g.landmarks.at }

// LandmarkBuilds is how many times the landmark tables have been taken, for
// anybody asking what keeping them is costing.
func (g *Grid) LandmarkBuilds() int { return g.landmarks.Builds }

// buildLandmarks chooses the landmarks and takes their tables. The first
// landmark is the dry tile farthest from the middle of the map, and each
// after it the dry tile farthest from all those chosen so far, as the crow
// flies, so that they end up spread round the edges; a walk to any one of
// them can be bounded that way from most directions. Ties go to the earlier
// tile, so the same map gets the same landmarks. The tables are then taken
// one to a goroutine, each on working memory of its own, and each is the
// same whichever goroutine takes it.
func (g *Grid) buildLandmarks(tick int) {
	l := &g.landmarks
	n := len(g.Tiles)
	if len(l.slack) != len(g.Chunks) {
		l.slack = make([]float64, len(g.Chunks))
	}
	clear(l.slack)
	l.at = g.chooseLandmarks(l.at[:0])
	l.free, l.laden = nil, nil
	l.built, l.stale, l.ladenOK = true, false, true
	l.Builds++
	l.taken = tick
	K := len(l.at)
	if K == 0 {
		return
	}
	tables := make([][]float32, 2*K)
	workers := WorkersFor(2 * K)
	if len(l.dists) < workers {
		l.dists = make([][]float64, workers)
		l.heaps = make([][]routeNode, workers)
	}
	InParallel(2*K, workers, func(i, worker int) {
		if len(l.dists[worker]) != n {
			l.dists[worker] = make([]float64, n)
		}
		to := int32(g.Index(l.at[i/2]))
		l.heaps[worker] = g.costsTo(to, i%2 == 1, l.dists[worker], l.heaps[worker])
		tables[i] = rounded(l.dists[worker])
	})
	// Laid out tile by tile so that a search reading every landmark's
	// number for one tile reads them side by side.
	free := make([]float32, 0, n*K)
	laden := make([]float32, 0, n*K)
	for i := 0; i < n; i++ {
		for k := 0; k < K; k++ {
			free = append(free, tables[2*k][i])
			laden = append(laden, tables[2*k+1][i])
		}
	}
	l.free, l.laden = free, laden
}

// chooseLandmarks picks LandmarkCount dry tiles, farthest-first as the crow
// flies from the middle of the map, into at. Fewer if the map has fewer
// dry tiles.
func (g *Grid) chooseLandmarks(at []geom.Pos) []geom.Pos {
	n := len(g.Tiles)
	least := make([]int, n)
	middle := geom.Pos{X: g.W / 2, Y: g.H / 2}
	for i := range least {
		least[i] = g.Dist(middle, g.PosOf(i))
	}
	for len(at) < LandmarkCount {
		best, pick := -1, -1
		for i := range g.Tiles {
			if least[i] > best && !g.Tiles[i].Deep() {
				best, pick = least[i], i
			}
		}
		if pick < 0 || (len(at) > 0 && best == 0) {
			break // no dry ground, or none that is not a landmark already
		}
		p := g.PosOf(pick)
		at = append(at, p)
		for i := range least {
			least[i] = min(least[i], g.Dist(p, g.PosOf(i)))
		}
	}
	return at
}

// rounded is dist as float32s, in a slice of its own.
func rounded(dist []float64) []float32 {
	out := make([]float32, len(dist))
	for i, d := range dist {
		out[i] = float32(d)
	}
	return out
}

// costsTo writes into dist the cheapest cost of walking from every tile to
// the tile at to, by the step costs the tables charge - see tableStep - and
// with no fences, which only ever add. It works on the heap it is given
// and hands it back for the next. Laden shuts deep water the way the search
// shuts it: a walker may not step off dry ground into it. Tiles with no
// way to to are left at +Inf.
func (g *Grid) costsTo(to int32, laden bool, dist []float64, heap []routeNode) []routeNode {
	for i := range dist {
		dist[i] = math.Inf(1)
	}
	dist[to] = 0
	q := append(heap[:0], routeNode{idx: to})
	for len(q) > 0 {
		top := q[0]
		last := len(q) - 1
		q[0] = q[last]
		q = q[:last]
		siftDown(q, 0)
		u := top.idx
		if top.cost > dist[u] {
			continue
		}
		ux, uy := int(u)%g.W, int(u)/g.W
		uDeep := g.Tiles[u].Deep()
		for d := range Dirs {
			vx, vy := ux+Dirs[d].X, uy+Dirs[d].Y
			if vy < 0 || vy >= g.H {
				continue
			}
			if g.Wrap {
				vx = g.WrapX(vx)
			} else if vx < 0 || vx >= g.W {
				continue
			}
			v := int32(vy*g.W + vx)
			// v is stepping into u. A walker on a flat may be standing in the
			// day's tide, and one in the water may step on into the water, so
			// the tables - which have to be good for every day - let a flat
			// step into the water as the sea itself does. See shore.go.
			if vt := &g.Tiles[v]; laden && uDeep && !vt.Deep() && !(vt.Terrain == Flat && vt.Mark == None) {
				continue
			}
			c := dist[u] + tableStep(g, v, u)
			if c < dist[v] {
				dist[v] = c
				q = append(q, routeNode{cost: c, idx: v})
				siftUp(q, len(q)-1)
			}
		}
	}
	return q[:0]
}

// slackWithin is the saving to take off every bound read over the window
// whose north-west corner is x0, y0 and whose side is span: the slack of
// every chunk the window touches.
func (g *Grid) slackWithin(x0, y0, span int) float64 {
	l := &g.landmarks
	if len(l.slack) != len(g.Chunks) {
		return 0
	}
	total := 0.0
	yLo, yHi := max(0, y0), min(g.H-1, y0+span-1)
	for cy := yLo / ChunkSide; cy <= yHi/ChunkSide; cy++ {
		for cx := 0; cx < g.CW; cx++ {
			if g.columnsMeet(cx*ChunkSide, min(g.W, cx*ChunkSide+ChunkSide)-1, x0, x0+span-1) {
				total += l.slack[cy*g.CW+cx]
			}
		}
	}
	return total
}

// columnsMeet reports whether any of the columns lo..hi of the map are
// among the columns x0..x1 of a window, which on a globe may run off either
// edge and round.
func (g *Grid) columnsMeet(lo, hi, x0, x1 int) bool {
	if !g.Wrap {
		return hi >= x0 && lo <= x1
	}
	if x1-x0+1 >= g.W {
		return true
	}
	x0, x1 = g.WrapX(x0), g.WrapX(x1)
	if x0 <= x1 {
		return hi >= x0 && lo <= x1
	}
	return hi >= x0 || lo <= x1 // the window goes round the seam
}

// guess is the least a walk to one destination could cost from any tile,
// as a guided search reads it at every tile it opens: the straight line
// at the cheapest ground there is, or what the landmarks say if that is
// more. Every bound is at or below the true cost, so the search settles
// the destination with the cheapest route before it settles anything
// costlier, the same as when it had nothing to go on but the straight
// line.
type guess struct {
	g      *Grid
	sx, sy int
	// guide is the table read - free or laden - K wide; k the landmarks
	// read from it, n of them, and dt the destination's walk to each.
	guide []float32
	K, n  int
	k     [activeLandmarks]int
	dt    [activeLandmarks]float64
	slack float64
}

// guessFor is what a search from from to stop, laden or not, over the
// window whose corner is x0, y0 and side span, can read off the landmarks.
// none says the landmarks show there is no way from from to stop at all.
//
// A laden walker reads the laden tables, unless the destination is deep
// water, which the laden tables shut but the walker may wade into at the
// end: the free tables bound that walk too, from below, being over the
// same ground with more ways open. Of the landmarks the destination has a
// way to, the few that say the most from where the walker stands are the
// ones read all the way: the landmark that sees the lake from here goes on
// seeing it, and reading them all would cost more a step than it saved. A
// landmark the destination has a way to and the walker has none to says
// there is no way to the destination either.
func (r *Router) guessFor(from, stop geom.Pos, laden bool, x0, y0, span int) (q guess, none bool) {
	g := r.g
	q = guess{g: g, sx: stop.X, sy: stop.Y}
	l := &g.landmarks
	if !l.usable() || len(l.at) == 0 {
		return q, false
	}
	q.K = len(l.at)
	q.guide = l.free
	if laden && l.ladenOK && !g.Tiles[g.Index(stop)].Deep() {
		q.guide = l.laden
	}
	si, fi := g.Index(stop)*q.K, g.Index(from)*q.K
	var best [activeLandmarks]float64
	for k := 0; k < q.K; k++ {
		dt, dv := float64(q.guide[si+k]), float64(q.guide[fi+k])
		if math.IsInf(dt, 1) {
			continue
		}
		if math.IsInf(dv, 1) {
			return q, true
		}
		b := dv - dt
		j := q.n
		if j < activeLandmarks {
			q.n++
		} else if b <= best[j-1] {
			continue
		} else {
			j--
		}
		for j > 0 && b > best[j-1] {
			best[j], q.k[j], q.dt[j] = best[j-1], q.k[j-1], q.dt[j-1]
			j--
		}
		best[j], q.k[j], q.dt[j] = b, k, dt
	}
	q.slack = g.slackWithin(x0, y0, span)
	return q, false
}

// at is the least the walk from x, y to the destination could still cost.
// +Inf says there is no way from here at all: no way to a landmark the
// destination has a way to, so none to the destination either.
func (q *guess) at(x, y int) float64 {
	g := q.g
	dx, dy := x-q.sx, y-q.sy
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if g.Wrap && g.W-dx < dx {
		dx = g.W - dx // the short way round
	}
	if dy > dx {
		dx = dy
	}
	est := minMoveCost * float64(dx)
	if q.n == 0 {
		return est
	}
	row := q.guide[(y*g.W+x)*q.K : (y*g.W+x+1)*q.K]
	for j := 0; j < q.n; j++ {
		dv := float64(row[q.k[j]])
		if math.IsInf(dv, 1) {
			return dv
		}
		dt := q.dt[j]
		if b := dv - dt - (dv+dt)*halfULP32 - q.slack; b > est {
			est = b
		}
	}
	return est
}

// AtLeast is the least the walk from one tile to another could cost, read
// off the landmarks and the straight line without walking it: what a
// guided search starts out knowing, for the load the router was told of.
// +Inf where there is known to be no way. It is for looking at what the
// search knows, not for costing a journey; TravelCost is that.
func (r *Router) AtLeast(from, to geom.Pos) float64 {
	g := r.g
	laden := r.load > SwimLoad
	r.load = 0
	from, to = g.Norm(from), g.Norm(to)
	if !g.In(from) || !g.In(to) {
		return math.Inf(1)
	}
	q, none := r.guessFor(from, to, laden, from.X-Window, from.Y-Window, 2*Window+1)
	if none {
		return math.Inf(1)
	}
	return q.at(from.X, from.Y)
}
