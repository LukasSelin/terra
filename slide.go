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
// It only ever lowers ground. The foot of a slide is the lower tile, which is
// already where it was, so nothing is raised to meet it: what came down is
// carried off, as it is off any real scarp, by the rivers at the bottom of it.

// Critical is the fall, as rise over run, past which ground fails: the bottom
// of Roering's range. Repose is the fall ground that has failed is left at:
// thirty-five degrees, about where Montgomery and Brandon (2002) found the mean
// slope of ranges wearing fast enough to be held at their threshold stops
// rising.
const (
	Critical = 1.2
	Repose   = 0.7
)

// landslide brings down every tile standing more than Critical above a
// neighbour to Repose above it. Taken from the lowest ground up, each tile is
// final by the time it is reached - a tile is only ever lowered by one below
// it - so one pass over the tiles in order of their heights settles the whole
// map, however far up a slope the failing runs.
func (g *Grid) landslide() {
	n := len(g.Tiles)
	h := make([]float64, n)
	done := make([]bool, n)
	var q slideQueue
	for i := range g.Tiles {
		h[i] = g.Tiles[i].Height
		q.push(h[i], int32(i))
	}
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
			if done[j] {
				continue
			}
			run := TileSpan
			if off.X != 0 && off.Y != 0 {
				run *= math.Sqrt2
			}
			if h[j] > h[i]+Critical*run {
				h[j] = h[i] + Repose*run
				q.push(h[j], j)
			}
		}
	}
	for i := range g.Tiles {
		g.Tiles[i].Height = h[i]
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
