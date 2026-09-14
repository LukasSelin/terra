package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
)

// Landslides: ground steeper than soil can stand on does not stand.
//
// A made world takes its heights by rank from the drawn map - see basins and
// normalise - and a history raises its ranges narrower than a drawn map does,
// so the same few hundred metres of high country land on belts a handful of
// tiles across. Read on three small globes, a seventh of the land fell more
// than 1.35 in one and the steepest hundredth fell ten: cliffs a quarter of a
// kilometre high, a tile apart, a few tiles in from every coast.
//
// Real ground does not do that. Roering and others (1999) found soil-mantled
// hillsides creeping faster and faster as they steepen, without limit, as the
// gradient comes up to between 1.2 and 1.35; past it the hillside fails. So the
// ground is let fail where it stands steeper than Critical, and what fails
// comes down to Repose.
//
// Down to Repose and not to Critical, because a slide does not stop at the
// slope it failed at. Cut back only as far as Critical, every flank that had
// failed anywhere came out a plane at 1.2 from its foot to its crest, ground
// that had stood at nine in ten above a cliff included: the steepest tenth of a
// made valley rose from 2.36 times its drawn twin's to 2.66, over the twelve
// seeds TestAHistoryLeavesAMapTheSettlementCanUse reads. By what a failed slope
// is left at:
//
//	left at   made valley's steepest tenth, against drawn   small globes' steepest hundredth
//	1.2                  2.66                                        1.22
//	0.9                  2.02                                        1.14
//	0.7                  1.61                                        1.02
//	0.5                  1.18                                        1.05
//
// What fails in the ages of weather is not lost. It used to be: the scar was
// cut down to Repose and the ground that had stood in it went nowhere. Now it
// runs down the fall of the ground, as a
// debris flow runs down the gully below a slide, and comes to rest where the
// ground under it flattens out - on the fan at the foot of the gully, in the
// hollow, on the valley floor, in the sea - and it arrives as soil, because a
// heap of broken ground is what soil is before anything grows in it.
//
// What fails while a map is being made is still let go, and that is on
// purpose. The slides there are not an event in the ground's history: they put
// right what a rescaling of the heights did - see basins and normalise, and
// shape - and a rescaling makes and takes away ground by the million cubic
// metres without conserving anything, so keeping the debris of what it
// overshot would be keeping ground nothing made. It was tried, and cost the
// made networks what they were measured against. Over three small globes,
// piled at the foot at Repose the debris stood in aprons thirty-five degrees
// steep down every flank and the land's mean slope rose from 0.60 to 0.73;
// run out to Settles it filled the valleys, and Hack's exponent went from
// within the real range to 0.62 on the valleys and 0.63 on a globe, and
// Horton's area ratio on the small globes fell from 4.1 to 2.9.

// Critical is the fall, as rise over run, past which ground fails: the bottom
// of Roering's range. Repose is the fall ground that has failed is left at:
// thirty-five degrees, about where Montgomery and Brandon (2002) found the mean
// slope of ranges wearing fast enough to be held at their threshold stops
// rising.
const (
	Critical = 1.2
	Repose   = 0.7
)

// Settles is the fall below which what came down comes to rest: debris flows
// stop and build their fans where the channel they run in flattens to about
// ten degrees (Takahashi 1981; Hungr and others 1984 put the start of
// deposition on debris torrents' fans at about the same). What comes down fills the ground it reaches up to that
// fall above the ground below it and runs on with the rest.
const Settles = 0.18

// slideLeast is the least excess over Critical, in metres, that is worth a
// slide: ground that stands a millimetre too steep is finished, and without it
// a slope would go on handing itself down by whatever the arithmetic could
// still tell apart.
const slideLeast = 1e-3

// runoutMost is how many tiles what came down may run before it is laid where
// it has got to: far more than any gully, and a bound on a walk across a flat
// sea floor that has room for a little at every step.
const runoutMost = 4096

// landslide brings down every tile standing more than Critical above a
// neighbour to Repose above it, and runs what came down on down the ground
// until it settles. With keep, the ground on the map adds up to the same before
// and after, to the rounding of the sums, which is how the ages of weather run
// it; without, what came down is let go, which is how the making of a map runs
// it. See above for why.
//
// Each tile a slide cuts or lays anything on is looked at again, and so are the
// neighbours of the one that failed, whose fall to it has just grown; the
// tiles are taken in the order they were queued, so a world repeats.
func (g *Grid) landslide(keep bool) {
	n := len(g.Tiles)
	h := make([]float64, n)
	soil := make([]float64, n)
	queued := make([]bool, n)
	queue := make([]int32, 0, n)
	seen := make([]int32, n) // which runout last passed each tile, by number
	for i := range g.Tiles {
		h[i], soil[i] = g.Tiles[i].Height, float64(g.Tiles[i].Soil)
		queue = append(queue, int32(i))
		queued[i] = true
	}
	push := func(i int32) {
		if !queued[i] {
			queued[i] = true
			queue = append(queue, i)
		}
	}
	neighbours := func(i int32, f func(j int32, run float64)) {
		p := g.PosOf(int(i))
		for _, off := range Dirs {
			q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
			if !g.In(q) {
				continue
			}
			run := TileSpan
			if off.X != 0 && off.Y != 0 {
				run *= math.Sqrt2
			}
			f(int32(g.Index(q)), run)
		}
	}
	runs := int32(0)
	for head := 0; head < len(queue); head++ {
		// Take back the front of the queue now and then, so that it does not
		// grow for as long as the slides go on.
		if head >= n && 2*head >= len(queue) {
			queue = append(queue[:0], queue[head:]...)
			head = 0
		}
		i := queue[head]
		queued[i] = false
		// The neighbour it stands steepest above, past what it can stand at.
		to, run, worst := int32(-1), 0.0, slideLeast
		neighbours(i, func(j int32, r float64) {
			if over := h[i] - h[j] - Critical*r; over > worst {
				to, run, worst = j, r, over
			}
		})
		if to < 0 {
			continue
		}
		d := h[i] - h[to] - Repose*run
		// What comes down: the soil first, then the rock the soil was made
		// of, as the mixture that rock makes.
		fromSoil := math.Min(d, soil[i])
		came := parts(&g.Tiles[i])
		for gr := range came {
			came[gr] *= fromSoil / d
		}
		if rock := d - fromSoil; rock > 0 {
			sand, clay := g.TextureAt(g.PosOf(int(i)))
			came[Sand] += rock / d * sand
			came[Silt] += rock / d * clamp01(1-sand-clay)
			came[Clay] += rock / d * clay
		}
		h[i] -= d
		soil[i] -= fromSoil
		push(i)
		neighbours(i, func(j int32, _ float64) { push(j) })

		// And it runs: down the steepest fall from the foot of the scar,
		// laying down at each tile as much as brings it up to Settles above
		// the ground it is running on to. Out of a hollow it runs over the
		// lowest of the rim it has not yet crossed.
		runs++
		at, left := to, d
		if !keep {
			left = 0
		}
		seen[i] = runs
		for step := 0; left > 0; step++ {
			seen[at] = runs
			next, nextRun, best := int32(-1), 0.0, math.Inf(-1)
			neighbours(at, func(j int32, r float64) {
				if seen[j] == runs {
					return
				}
				if fall := (h[at] - h[j]) / r; fall > best {
					next, nextRun, best = j, r, fall
				}
			})
			lay := left
			if next >= 0 && step < runoutMost {
				lay = math.Min(left, math.Max(0, h[next]+Settles*nextRun-h[at]))
			}
			if lay > 0 {
				laid := came
				for gr := range laid {
					laid[gr] *= lay
				}
				mix(&g.Tiles[at], soil[at], laid)
				h[at] += lay
				soil[at] += lay
				left -= lay
				push(at)
			}
			at = next
		}
	}
	for i := range g.Tiles {
		g.Tiles[i].Height, g.Tiles[i].Soil = h[i], float32(soil[i])
	}
}

// slideQueue is a binary heap of tiles, lowest first, each held at the height
// it was pushed at. A tile lowered after it was pushed is pushed again, and the
// stale entry is passed over when it comes up: see done in landslide.
type slideQueue struct {
	at []slideAt
}

type slideAt struct {
	h float64
	i int32
}

func (a slideAt) less(b slideAt) bool {
	if a.h != b.h {
		return a.h < b.h
	}
	return a.i < b.i // ties by position, so a world repeats
}

func (q *slideQueue) push(h float64, i int32) {
	q.at = append(q.at, slideAt{h, i})
	k := len(q.at) - 1
	for k > 0 {
		up := (k - 1) / 2
		if !q.at[k].less(q.at[up]) {
			break
		}
		q.at[k], q.at[up] = q.at[up], q.at[k]
		k = up
	}
}

func (q *slideQueue) pop() int32 {
	top := q.at[0].i
	last := len(q.at) - 1
	q.at[0] = q.at[last]
	q.at = q.at[:last]
	k := 0
	for {
		l, r, least := 2*k+1, 2*k+2, k
		if l < last && q.at[l].less(q.at[least]) {
			least = l
		}
		if r < last && q.at[r].less(q.at[least]) {
			least = r
		}
		if least == k {
			return top
		}
		q.at[k], q.at[least] = q.at[least], q.at[k]
		k = least
	}
}
