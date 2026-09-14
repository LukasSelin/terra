package terra

import (
	"math"

	"github.com/LukasSelin/terra/clock"
)

// The ground that is awake. Every day the weather passes over every tile
// of the map - wear fades, stands grow, fish come back, worn soil rests,
// what nobody keeps falls down - and on a map big enough to hold more than
// one settlement nearly all of that ground has nobody on it and nothing
// built on it. A chunk with nobody in it, nothing standing on it, nobody
// across it these two seasons, and none of the first two next door, is
// asleep: the passes skip it, and what the weather would have done to it
// is done in one go when it wakes, or on a slow sweep so that nothing
// sleeps longer than a season.
//
// Catching up is the same arithmetic as the day's pass with a season's
// growing weather in place of a day's, so it is deterministic, but it is
// not the pass taken a day at a time to the last bit: a stand that would
// have been held back by its age one day and let go the next comes out a
// hair different. So the map a settlement is measured on must never have
// a chunk asleep, and on the default map none ever is - the market's chunk
// is always occupied and the other is beside it. A test says so.

// Growing is the growing weather the world has had since it was made, in
// growing days: the sum over every day of what that day let green things
// grow. A chunk stamps it when it is passed over, and what the chunk is
// owed is the difference.
//
// Weathered is the day a chunk was last weathered, for the wear that fades
// by the day whatever the season.

// sweepOver is how long the sweep takes to reach every sleeping chunk once.
const sweepOver = clock.Season

// settledAt is how much has to stand on a chunk, or be held, for it to be
// a settlement whose neighbours are woken: a house and a holding of fields
// is a farmstead, and what its people read across the edge of its chunk
// is read stale; a market, or a dozen such tiles, is a town. The market
// always counts, because the map a settlement is measured on is two chunks
// with the market in one, and the other must never sleep.
const settledAt = 12

// wearMemory is how long a crossing keeps ground awake. Wear fades by a
// third in a season, so after one a single crossing is well under what
// counts as a way, and what is left of it can be faded when the ground
// next wakes.
const wearMemory = clock.Season

// Waking is what passes between a game and the ground as the ground wakes.
//
// Both halves of it are things only the game can answer or wants told.
// Peopled says whether anybody at all is standing on chunk i: a settlement
// answers it off its index of agents, a game with one player answers it by
// looking at where he is, and the land has to be told because it does not
// know what it is carrying. Slept is the other direction - the land saying
// that a chunk awake yesterday is not awake today - so that whatever the
// game keeps against awake ground can be let go of. Either may be nil.
type Waking struct {
	Peopled func(i int) bool
	Slept   func(i int)
}

// Wake works out which chunks are awake this day, catches up any that were
// asleep and are not now, and sweeps a few that still are.
func (w *Land) Wake(k Waking) {
	g := w.Grid
	// The day's sea, first, so that everything the day reads of the shore
	// reads the same tide.
	g.tide = w.Tide()
	if len(g.Active) != len(g.Chunks) {
		g.Active = make([]bool, len(g.Chunks))
	}
	// Settled ground - a town, see settledAt - is awake and wakes its
	// neighbours, so that what a settlement reads across a chunk's edge is
	// read off ground the day has passed over. Ground with people on it, or
	// walked on lately, or with a farmstead on it, is awake on its own
	// account and wakes nothing: a scout on the far side of the country is
	// not a settlement, and neither is the trail behind it.
	settled := make([]bool, len(g.Chunks))
	for i := range g.Chunks {
		c := &g.Chunks[i]
		if c.Trodden {
			c.Trodden, c.Trod = false, w.Tick
		}
		settled[i] = c.Places > 0 || c.Built+c.Owned >= settledAt
	}
	w.Awake = AwakeCount{Chunks: len(g.Chunks)}
	for i := range g.Chunks {
		was := g.Active[i]
		c := &g.Chunks[i]
		on, worn := k.Peopled != nil && k.Peopled(i), c.Trod >= 0 && w.Tick-c.Trod <= wearMemory
		g.Active[i] = on || worn || c.Built > 0 || c.Owned > 0
		cx, cy := i%g.CW, i/g.CW
		for dy := -1; dy <= 1 && !g.Active[i]; dy++ {
			y := cy + dy
			if y < 0 || y >= g.CH {
				continue
			}
			for dx := -1; dx <= 1; dx++ {
				x := cx + dx
				if g.Wrap {
					x = ((x % g.CW) + g.CW) % g.CW
				} else if x < 0 || x >= g.CW {
					continue
				}
				if settled[y*g.CW+x] {
					g.Active[i] = true
					break
				}
			}
		}
		switch {
		case settled[i]:
			w.Awake.Settled++
		case on:
			w.Awake.Peopled++
		case worn:
			w.Awake.Worn++
		case g.Active[i]:
			w.Awake.Beside++
		}
		switch {
		case g.Active[i] && !was:
			w.CatchUp(i)
		case was && !g.Active[i] && k.Slept != nil:
			// This chunk has gone to sleep, and whatever the game was
			// keeping against it can go with it.
			k.Slept(i)
		}
	}
	// The sweep: enough sleeping chunks a day that every one is caught up
	// once a season, taken in turn.
	n := (len(g.Chunks) + sweepOver - 1) / sweepOver
	for k := 0; k < n; k++ {
		i := (w.swept + k) % len(g.Chunks)
		if !g.Active[i] {
			w.CatchUp(i)
		}
	}
	w.swept = (w.swept + n) % len(g.Chunks)
}

// CatchUp does to chunk i what the days it slept through would have done,
// and stamps it as weathered up to yesterday. The day's own pass follows.
func (w *Land) CatchUp(i int) {
	g := w.Grid
	c := &g.Chunks[i]
	growth := w.Growing[i] - c.Grown
	days := w.Tick - 1 - c.Weathered
	if growth > 0 || days > 0 {
		fade := math.Pow(Fade, float64(max(0, days)))
		g.rowsIn(i, func(lo, hi int) {
			if days > 0 {
				g.FadeWear(lo, hi, fade)
			}
			if growth > 0 {
				g.Grow(lo, hi, growth)
			}
		})
	}
	c.Grown, c.Weathered = w.Growing[i], w.Tick-1
}

// CatchUpAll catches up every sleeping chunk, for before the whole ground is
// remade at once.
func (w *Land) CatchUpAll() {
	g := w.Grid
	for i := range g.Chunks {
		if len(g.Active) == len(g.Chunks) && !g.Active[i] {
			w.CatchUp(i)
		}
	}
}

// Stamp marks every awake chunk as passed over today with the growing
// weather so far. The day's passes call it when they are done.
func (g *Grid) Stamp(growing []float64, tick int) {
	for i := range g.Chunks {
		if len(g.Active) != len(g.Chunks) || g.Active[i] {
			g.Chunks[i].Grown, g.Chunks[i].Weathered = growing[i], tick
		}
	}
}

// Rates is what this day's weather lets green things grow on each chunk, at
// the latitude of its middle row and the mean height of its ground: the
// weather goes by both, and a chunk is the finest the sleeping ground is
// reckoned by. It was by chunk row, which was the whole of the weather when
// the whole of the weather was latitude. On a valley with level ground every
// chunk reads the same.
// The land is told how fast things come back rather than asked: a
// settlement that has learnt something about husbandry grows more on the
// same weather, and what it has learnt is the game's business and not the
// weather's. So Rates takes the multiplier, and the World below passes its
// own.
func (w *Land) Rates(regrowth float64) []float64 {
	g := w.Grid
	if len(w.rates) != len(g.Chunks) {
		w.rates = make([]float64, len(g.Chunks))
	}
	byClimate := w.Terms.Growth.climate(g.Wrap)
	for i := range w.rates {
		c := &g.Chunks[i]
		mid := min(g.H-1, c.Y0+c.H/2)
		temp := w.Climate.TempAt(mid) - Lapse*c.Height
		if !byClimate {
			w.rates[i] = regrowth * growthOf(temp)
			continue
		}
		// Under the climate's rules the year of the chunk's middle tile is
		// read, at the chunk's mean height: see climateGrowth.
		t := mid*g.W + min(g.W-1, c.X0+c.W/2)
		mean, swing := w.yearAt(t)
		mean += Lapse * (g.Tiles[t].Height - c.Height)
		w.rates[i] = regrowth * climateGrowth(temp, mean, swing, g.Rain(t))
	}
	return w.rates
}

// EachActive visits every tile of every awake chunk that only admits, or of
// every awake chunk when only is nil, in the order a walk over the whole
// map would visit them: row by row, and along each row. The caller is told
// which chunk each tile is in, which it would otherwise have to divide for.
// That order is the order the world's chance is drawn in by what nobody
// keeps, so it is kept exactly. A map that has never been woken is read as
// all awake, so that the day's systems can be run on their own.
func (g *Grid) EachActive(only func(c int) bool, f func(i, c int, t *Tile)) {
	all := len(g.Active) != len(g.Chunks)
	for cy := 0; cy < g.CH; cy++ {
		y0, y1 := cy*ChunkSide, min(g.H, (cy+1)*ChunkSide)
		for y := y0; y < y1; y++ {
			row := y * g.W
			for cx := 0; cx < g.CW; cx++ {
				c := cy*g.CW + cx
				if (!all && !g.Active[c]) || (only != nil && !only(c)) {
					continue
				}
				x0, x1 := cx*ChunkSide, min(g.W, (cx+1)*ChunkSide)
				for i := row + x0; i < row+x1; i++ {
					f(i, c, &g.Tiles[i])
				}
			}
		}
	}
}

// rowsIn hands f each row of chunk c as the run of tile indices [lo, hi)
// it takes up: a chunk is a square of a row-major map, so its ground is a
// run per row and not one run.
func (g *Grid) rowsIn(c int, f func(lo, hi int)) {
	ch := &g.Chunks[c]
	for y := ch.Y0; y < ch.Y0+ch.H; y++ {
		lo := y*g.W + ch.X0
		f(lo, lo+ch.W)
	}
}

// eachIn visits every tile of chunk i, row by row.
func (g *Grid) eachIn(i int, f func(j int, t *Tile)) {
	c := &g.Chunks[i]
	for y := c.Y0; y < c.Y0+c.H; y++ {
		row := y * g.W
		for j := row + c.X0; j < row+c.X0+c.W; j++ {
			f(j, &g.Tiles[j])
		}
	}
}

// Awake reports whether chunk i is awake. A map never woken is all awake.
func (g *Grid) Awake(i int) bool {
	return len(g.Active) != len(g.Chunks) || g.Active[i]
}

// worn is which chunks anybody has walked in these two seasons, or that
// are beside one, as of day tick. It is what the case for a road is read
// over: wear a step away is the most a tile's case can be made of.
func (g *Grid) Worn(tick int) []bool {
	lately := make([]bool, len(g.Chunks))
	for i := range g.Chunks {
		c := &g.Chunks[i]
		lately[i] = c.Trodden || (c.Trod >= 0 && tick-c.Trod <= wearMemory)
	}
	return g.beside(lately)
}

// beside is which chunks are marked or beside a marked one.
func (g *Grid) beside(mark []bool) []bool {
	out := make([]bool, len(g.Chunks))
	for i := range g.Chunks {
		cx, cy := i%g.CW, i/g.CW
		for dy := -1; dy <= 1 && !out[i]; dy++ {
			y := cy + dy
			if y < 0 || y >= g.CH {
				continue
			}
			for dx := -1; dx <= 1; dx++ {
				x := cx + dx
				if g.Wrap {
					x = ((x % g.CW) + g.CW) % g.CW
				} else if x < 0 || x >= g.CW {
					continue
				}
				if mark[y*g.CW+x] {
					out[i] = true
					break
				}
			}
		}
	}
	return out
}

// AwakeCount is how many chunks are awake and on what account, for a runner
// that says what a day was spent on.
type AwakeCount struct {
	Chunks, Settled, Beside, Peopled, Worn int
}

// spreadFrom is how many chunks have to be awake before a pass over the
// ground is worth handing to goroutines. A chunk is four thousand tiles and
// handing out work costs a few microseconds, so the bar is low - but it is
// not nothing: the default valley is two chunks all told, and a pass over it
// is done sooner than it could be delegated.
const spreadFrom = 4

// EachActiveOver is EachActive spread over goroutines, a chunk to each. It
// visits exactly the tiles EachActive visits and hands each of them the same
// three things; what it does not keep is the order they are visited in.
//
// So f must be a pass that does not care about that order, which is to say:
// f may read anything on the map, and may write the tile it was given and a
// slice at that tile's index, and nothing else. It must draw no chance - the
// order the world's chance is drawn in is a fact about the settlement, and
// this order is not a fact about anything. A pass that cannot keep to that
// belongs in EachActive, which is most of them; two keep to it, and they are
// among the passes the ground costs most in. The day's own pass over the
// ground keeps to it too, and takes the ground a row of a chunk at a time
// rather than a tile at a time: see EachActiveRow. See parallel.go.
//
// only, where it is given, is asked from several goroutines and must be safe
// to ask that way.
func (g *Grid) EachActiveOver(only func(c int) bool, f func(i, c int, t *Tile)) {
	awake := 0
	for c := range g.Chunks {
		if g.Awake(c) && (only == nil || only(c)) {
			awake++
		}
	}
	if awake < spreadFrom {
		g.EachActive(only, f)
		return
	}
	InParallel(len(g.Chunks), WorkersFor(awake), func(c, _ int) {
		if !g.Awake(c) || (only != nil && !only(c)) {
			return
		}
		ch := &g.Chunks[c]
		for y := ch.Y0; y < ch.Y0+ch.H; y++ {
			row := y * g.W
			for i := row + ch.X0; i < row+ch.X0+ch.W; i++ {
				f(i, c, &g.Tiles[i])
			}
		}
	})
}

// EachActiveChunk is EachActiveOver for a pass that wants the chunk whole:
// f is handed each awake chunk that only admits, a chunk to a goroutine
// where enough of the ground is awake to be worth it, and may read
// anything on the map and write anything of that chunk's and nothing
// else. It is for a pass that keeps a fact per chunk about what it found
// there, which a pass handed tiles one at a time cannot.
func (g *Grid) EachActiveChunk(only func(c int) bool, f func(c int)) {
	awake := 0
	for c := range g.Chunks {
		if g.Awake(c) && (only == nil || only(c)) {
			awake++
		}
	}
	workers := 1
	if awake >= spreadFrom {
		workers = WorkersFor(awake)
	}
	InParallel(len(g.Chunks), workers, func(c, _ int) {
		if !g.Awake(c) || (only != nil && !only(c)) {
			return
		}
		f(c)
	})
}

// EachActiveRow is EachActiveOver for a pass that works on runs of tiles
// rather than on tiles: it hands f every row of every awake chunk that
// only admits, as the run of tile indices [lo, hi) and the chunk it is in.
// The rows are spread over goroutines a chunk to each where enough of the
// ground is awake to be worth it, and taken chunk by chunk where it is not;
// the order is not kept either way, so f must be a pass that does not care
// - one that draws no chance, reads what it likes, and writes only the
// layers at [lo, hi). That is the day's pass over the ground: see
// system.Land, and Grow and FadeWear in pass.go.
func (g *Grid) EachActiveRow(only func(c int) bool, f func(lo, hi, c int)) {
	awake := 0
	for c := range g.Chunks {
		if g.Awake(c) && (only == nil || only(c)) {
			awake++
		}
	}
	workers := 1
	if awake >= spreadFrom {
		workers = WorkersFor(awake)
	}
	InParallel(len(g.Chunks), workers, func(c, _ int) {
		if !g.Awake(c) || (only != nil && !only(c)) {
			return
		}
		// The rows of the chunk, as rowsIn hands them out, written out here
		// so that a day makes no closure per chunk to hand them on with.
		ch := &g.Chunks[c]
		for y := ch.Y0; y < ch.Y0+ch.H; y++ {
			lo := y*g.W + ch.X0
			f(lo, lo+ch.W, c)
		}
	})
}
