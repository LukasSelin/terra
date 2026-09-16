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

// standRock is how much steeper ground stands for the rock it is made of, as
// a power of its hardness against the map's middling rock, and standMost and
// standLeast the most and least that may make of Critical and Repose. The
// most is held to the top of Roering's range, 1.35 over 1.2: past that a
// tile is a wall and not a hillside, and a scarp a tile wide is steeper than
// the grid can say anything true about.
const (
	standRock  = 0.3
	standMost  = 1.35 / Critical
	standLeast = 0.7
)

// stand is what the rock makes of the slopes ground fails at and is left at,
// given its hardness against the map's middling rock.
func stand(hard float64) float64 {
	return math.Max(standLeast, math.Min(standMost, math.Pow(hard, standRock)))
}

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
// it - see above for why, and cutBack for how.
//
// What stands steep depends on what it is made of. The slope a tile fails at
// and the slope it is left at go with the rock at its surface against the
// map's middling rock - see stand - so a cap of hard rock holds a cliff over
// the soft beds beneath it, and those beds slump back to a gentler foot. That
// is the whole shape of a scarp and of the rim of a mesa.
//
// Each tile a slide cuts or lays anything on is looked at again, and so are the
// neighbours of the one that failed, whose fall to it has just grown; the
// tiles are taken in the order they were queued, so a world repeats.
func (g *Grid) landslide(keep bool) {
	defer phase("landslide")()
	if !keep {
		g.cutBack()
		return
	}
	n := len(g.Tiles)
	h := make([]float64, n)
	soil := make([]float64, n)
	queued := make([]bool, n)
	queue := make([]int32, 0, n)
	seen := make([]int32, n) // which runout last passed each tile, by number
	for i := range g.Tiles {
		h[i], soil[i] = g.Height[i], float64(g.Soil[i])
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
			// The deep sea floor is no neighbour of anything a slide does:
			// nothing fails off it and nothing runs out onto it. See abyssal.
			if j := g.Index(q); g.abyssal(j) || g.abyssal(int(i)) {
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
	soft := 1 / g.meanHard()
	// What each rock stands at, against the map's middling rock: asked of
	// every edge of every tile the slides pass, and there are six rocks.
	var stands [BedrockCount]float64
	for b := range stands {
		stands[b] = stand(hardness[b] * soft)
	}
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
		to, run, worst, rests := int32(-1), 0.0, slideLeast, Repose
		neighbours(i, func(j int32, r float64) {
			critical, repose := Critical, Repose
			if g.strata != nil {
				// Whether the edge fails is the rock the edge is made of;
				// what it is left at is the rock the failure bares.
				s := stands[g.bedAt(int(i), h[i])]
				critical = Critical * s
				repose = Repose * stands[g.bedAt(int(i), h[j]+Repose*s*r)]
			}
			if over := h[i] - h[j] - critical*r; over > worst {
				to, run, worst, rests = j, r, over, repose
			}
		})
		if to < 0 {
			continue
		}
		d := h[i] - h[to] - rests*run
		// What comes down: the soil first, then the rock the soil was made
		// of, as the mixture that rock makes.
		fromSoil := math.Min(d, soil[i])
		came := g.parts(int(i))
		for gr := range came {
			came[gr] *= fromSoil / d
		}
		if rock := d - fromSoil; rock > 0 {
			sand, clay := g.TextureAt(g.PosOf(int(i)))
			came[Sand] += rock / d * sand
			came[Silt] += rock / d * clamp01(1-sand-clay)
			came[Clay] += rock / d * clay
		}
		// The scar takes what time had made of the soil it took, and where it
		// went through into the rock it has left a fresh face: see strip.
		if soil[i] > 0 {
			strip(&g.Tiles[i], d/soil[i])
		} else {
			clearSoil(&g.Tiles[i])
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
				g.mix(int(at), soil[at], laid)
				h[at] += lay
				soil[at] += lay
				left -= lay
				push(at)
			}
			at = next
		}
	}
	for i := range g.Tiles {
		g.Height[i], g.Soil[i] = h[i], float32(soil[i])
	}
}

// slideQueue is a heap of tiles, lowest first, each held at the height it was
// pushed at. A tile lowered after it was pushed is pushed again, and the stale
// entry is passed over when it comes up: see done in cutBack and fillFrom.
//
// It is a 4-ary heap rather than a binary one: a pop walks half the ladder,
// and the four children it asks at each rung lie in one cache line. The order
// the tiles come out in does not depend on the heap's shape, because less is a
// total order - by height, then by index - so every pop is the one least
// entry, and two entries that compare equal are the same tile at the same
// height. Its backing is kept on the Grid between calls: see slideScratch.
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
	n := slideAt{h, i}
	k := len(q.at)
	q.at = append(q.at, n)
	at := q.at
	for k > 0 {
		up := (k - 1) / 4
		if !n.less(at[up]) {
			break
		}
		at[k] = at[up]
		k = up
	}
	at[k] = n
}

func (q *slideQueue) pop() int32 {
	at := q.at
	top := at[0].i
	last := len(at) - 1
	x := at[last]
	at = at[:last]
	q.at = at
	if last == 0 {
		return top
	}
	k := 0
	for {
		first := 4*k + 1
		if first >= last {
			break
		}
		least := first
		for c := first + 1; c < first+4 && c < last; c++ {
			if at[c].less(at[least]) {
				least = c
			}
		}
		if !at[least].less(x) {
			break
		}
		at[k] = at[least]
		k = least
	}
	at[k] = x
	return top
}

// cutBack is landslide for the making of a map: every tile standing more than
// Critical above a neighbour is cut back to Repose above it, and what is cut
// is let go. Taken from the lowest ground up, each tile is final by the time it
// is reached - a tile is only ever lowered by one below it - so one pass over
// the tiles in order of their heights settles the whole map, however far up a
// slope the failing runs.
//
// It is kept as it was, rather than landslide with the debris dropped, because
// the order the failing is taken in shapes what is left. Taken off a queue in
// the order the tiles were reached and cut toward the steepest neighbour
// first, the same scars lay the coast of a small globe's first seed a few
// metres differently, and its tidal flats, which are the ground within a
// spring tide of the sea, were gone.
func (g *Grid) cutBack() {
	n := len(g.Tiles)
	h := make([]float64, n)
	done := make([]bool, n)
	q := slideQueue{at: g.slideScratch[:0]}
	for i := range g.Tiles {
		h[i] = g.Height[i]
		q.push(h[i], int32(i))
	}
	soft := 1 / g.meanHard()
	for len(q.at) > 0 {
		i := q.pop()
		if done[i] {
			continue
		}
		done[i] = true
		p := g.PosOf(int(i))
		for _, off := range Dirs {
			nb := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
			if !g.In(nb) {
				continue
			}
			j := int32(g.Index(nb))
			// Nothing is cut back off the deep sea floor. See abyssal.
			if done[j] || g.abyssal(int(i)) || g.abyssal(int(j)) {
				continue
			}
			run := TileSpan
			if off.X != 0 && off.Y != 0 {
				run *= math.Sqrt2
			}
			critical, repose := Critical, Repose
			if g.strata != nil {
				// Whether the edge fails is the rock the edge is made of;
				// what it is left at is the rock the failure bares.
				s := stand(g.hardAt(int(j), h[j]) * soft)
				critical, repose = critical*s, repose*s
				left := h[i] + repose*run
				repose = Repose * stand(g.hardAt(int(j), left)*soft)
			}
			if h[j] > h[i]+critical*run {
				h[j] = h[i] + repose*run
				q.push(h[j], j)
			}
		}
	}
	g.slideScratch = q.at[:0]
	for i := range g.Tiles {
		g.Height[i] = h[i]
	}
}
