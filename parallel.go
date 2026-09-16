package terra

import (
	"runtime"

	"github.com/LukasSelin/terra/internal/par"
)

// Reading the world on several goroutines at once.
//
// Exactly one goroutine may change a World and several may read one; see the
// remarks at the top of world.go. This is the little that both halves of a
// tick need in order to keep to that - how many goroutines a pass that only
// reads may spread over, and the one way of spreading it.
//
// Every such pass obeys the same rule, and it is the rule that keeps a run
// the same however the goroutines happen to be scheduled: a worker writes
// only where no other worker looks, nothing a worker writes is read until
// every worker has finished, and no worker draws the world's chance. What
// comes out lands in a slice indexed by the work, and the world is changed
// from it afterwards, in one order, on one goroutine. The passes spread
// this way are agents deciding, in system.Decide; the passes over the
// ground, in EachActiveOver and EachActiveRow; the cheap passes over the
// population - who stands beside whom in system.Beliefs, the wearing of
// each body in system.Decay, everybody's situation in system.Discover; and
// the finding of the fields in Fence. Each is held against a run of the
// same seed that spread over nothing.
//
// The one pass that changes the world from several goroutines at once is
// the day's acting, and it keeps the rule another way: the people are cut
// into islands that cannot touch the same ground, and what any island does
// to the settlement as a whole is taken down apart and put together
// afterwards in one order. See island.go.

// Workers is how many goroutines the read-only passes of a day may spread
// over: the deciding, which is the bulk of a day where there are people,
// and the passes over the ground, which are the bulk of it where there is
// country. Set it to 1 to do everything one at a time.
var Workers = runtime.NumCPU()

// WorkersFor is how many goroutines to spread n pieces of work over: no
// more than there is work, and never none.
func WorkersFor(n int) int {
	k := Workers
	if n < k {
		k = n
	}
	if k < 1 {
		k = 1
	}
	return k
}

// InParallel runs f for every index below n, spread over workers goroutines,
// and returns when the last of them is done. See par.InParallel.
func InParallel(n, workers int, f func(i, worker int)) { par.InParallel(n, workers, f) }

// spreadAgents is how many people a pass over the population is worth a
// goroutine for. Deciding is one agent to a goroutine, because a decision
// costs far more than handing one over; a look at who is standing beside
// somebody costs a few hundred nanoseconds, and handing two hundred of
// those to twenty-four goroutines cost the valley more than doing them
// in turn.
const spreadAgents = 64

// WorkersOver is how many goroutines to spread a cheap pass over n people
// over: one for every spreadAgents of them, and never more than Workers.
// Under spreadAgents it is one, and the pass is done in turn.
func WorkersOver(n int) int { return WorkersFor(n / spreadAgents) }

// spreadTiles is how much ground a pass has to cover before it is worth
// handing to goroutines. The default valley is under three thousand tiles
// and is passed over quicker than the work could be dealt out; a globe is
// half a million and is not.
const spreadTiles = 1 << 14

// EachRow runs f for every row of the grid, spread over goroutines where
// there is ground enough to be worth it. It is how the passes that make a
// world are spread - the ground is drawn, then measured, then read, and each
// of those is the same arithmetic done half a million times over.
//
// f may read anything on the map, and may write the tiles of the row it was
// given and slices at those tiles' indices, and nothing else. It must draw no
// chance: the order the world's chance is drawn in while a world is being
// made is the whole of what the seed means. Where a pass does draw, the
// drawing stays in a walk of its own and only the arithmetic comes here.
func (g *Grid) EachRow(f func(y int)) {
	if len(g.Tiles) < spreadTiles {
		for y := 0; y < g.H; y++ {
			f(y)
		}
		return
	}
	InParallel(g.H, WorkersFor(g.H), func(y, _ int) { f(y) })
}
