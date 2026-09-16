package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
)

// littoral drives the sand along the shore: the sand each cell of the surf is
// given - supply - and the sand it takes up off its own bed where the drive
// could carry more than it is given, carried on to the next cell along the
// shore the drift goes toward, and laid down where the drive can carry less
// than it holds, up to the berm.
//
// Each cell sends its sand to one neighbour, the one of the eight that lies
// most nearly the way its drift goes, keeping to the shore where the shore
// goes nearly that way and the waves drive sand along it there too. Where
// nothing lies that way but ground, the sand stops: a beach at the head of a
// bay. Where the shore turns away, and what lies ahead is open water or a
// shore in the lee the waves do not reach, the sand goes on straight into it,
// and fills the water ahead to the berm a tile at a time for as far as the sea
// there is no deeper than the waves move sand at: a spit. Sand that comes to
// water deeper than that is gone.
//
// The cells are taken so that every cell comes after all that send it sand,
// and where two send to each other, or a ring of them round an island does,
// the ring is broken at its first cell in tile order, which keeps what reaches
// it. So each grain is booked once, wherever it ends, and the drive is solved
// exactly for the step: a stretch of shore that could carry more than a
// step's sand passes on everything it is given and all of its bed it can,
// however long the step.
func (g *Grid) littoral(s *surf, years float64, supply []float64) {
	m := len(s.cells)
	span := g.span()
	littoral := func(c int) bool {
		i := s.cells[c]
		return g.sea-g.Height[i] <= s.closure[c]
	}
	// carries is whether the waves drive sand along the shore at cell c at all:
	// a cell in the lee of the land has none coming in, and sand driven into
	// it has come to quiet water.
	carries := func(c int) bool {
		return littoral(c) && s.drift[c] != [2]float64{}
	}
	recv := make([]int32, m) // a cell, or -1, or -2-i for tile i of open water ahead
	indeg := make([]int32, m)
	for c := range m {
		recv[c] = -1
		d := s.drift[c]
		mag := math.Hypot(d[0], d[1])
		if !carries(c) {
			continue
		}
		p := g.PosOf(int(s.cells[c]))
		best, bestScore := -1, 0.0
		ahead := false
		for _, off := range Dirs {
			q := g.Norm(geom.Pos{X: p.X + off.X, Y: p.Y + off.Y})
			if !g.In(q) {
				continue
			}
			j := g.Index(q)
			if !g.seaCell(j) {
				continue
			}
			// Within about seventy degrees of the drift, and a shore that carries
			// sand on is taken over open water that lies up to forty degrees
			// nearer the drift's way: how a line of sand keeps to a shore drawn
			// in steps of eight ways, which is the grid's and not the sea's.
			l := math.Hypot(float64(off.X), float64(off.Y))
			dot := (float64(off.X)*d[0] + float64(off.Y)*d[1]) / (l * mag)
			if dot < 0.3 {
				continue
			}
			score := dot
			shore := s.slot[j] >= 0 && carries(int(s.slot[j]))
			if shore {
				score += 0.25
			}
			if score > bestScore {
				best, bestScore, ahead = j, score, !shore
			}
		}
		switch {
		case best < 0:
		case ahead:
			recv[c] = int32(-2 - best)
		default:
			recv[c] = s.slot[best]
			indeg[recv[c]]++
		}
	}
	load := make([]float64, m)
	done := make([]bool, m)
	queue := make([]int32, 0, m)
	for c := range m {
		if indeg[c] == 0 {
			queue = append(queue, int32(c))
		}
	}
	perSecond := years * secondsPerYear / (span * span)
	lay := func(i int, d float64) {
		if d <= 0 {
			return
		}
		t := &g.Tiles[i]
		mix(t, float64(g.Soil[i]), [Grains]float64{Sand: d})
		g.Soil[i] += float32(d)
		g.Height[i] += d
	}
	// seen marks the cells a search for a ring has walked through, with the
	// number of the search, so that no cell is walked from twice.
	seen := make([]int32, m)
	walks := int32(0)
	next := 0
	for len(queue) > 0 || next < m {
		if len(queue) == 0 {
			// Everything left is a ring or waits on one. Walk down the drift
			// from the first cell left until a cell comes round again, and break
			// that ring at its first cell in tile order.
			for next < m && (done[next] || seen[next] != 0) {
				next++
			}
			if next >= m {
				break
			}
			walks++
			c := next
			for recv[c] >= 0 && seen[c] == 0 {
				seen[c] = walks
				c = int(recv[c])
			}
			if recv[c] < 0 || seen[c] != walks {
				continue // the walk ran out, or into a ring already broken
			}
			first := c
			for k := int(recv[c]); k != c; k = int(recv[k]) {
				first = min(first, k)
			}
			r := recv[first]
			recv[first] = -1
			if indeg[r]--; indeg[r] == 0 {
				queue = append(queue, r)
			}
			continue
		}
		c := int(queue[0])
		queue = queue[1:]
		if done[c] {
			continue
		}
		done[c] = true
		i := int(s.cells[c])
		carried := load[c] + supply[c]
		if !littoral(c) {
			g.exported[Sand] += carried
			continue
		}
		t := &g.Tiles[i]
		room := math.Max(0, g.berm(s, c)-g.Height[i])
		r := recv[c]
		if r < 0 {
			d := math.Min(carried, room)
			lay(i, d)
			carried -= d
			if r <= -2 {
				// Into the open water ahead, a tile at a time.
				j, from := int(-2-r), g.PosOf(i)
				step := g.Delta(from, g.PosOf(j))
				for carried > 0 && g.seaCell(j) && (s.slot[j] < 0 || !carries(int(s.slot[j]))) && g.sea-g.Height[j] <= s.closure[c] {
					d := math.Min(carried, math.Max(0, g.berm(s, c)-g.Height[j]))
					lay(j, d)
					carried -= d
					q := g.Norm(geom.Pos{X: g.PosOf(j).X + step.X, Y: g.PosOf(j).Y + step.Y})
					if !g.In(q) {
						break
					}
					j = g.Index(q)
				}
			}
			g.exported[Sand] += carried
			continue
		}
		capacity := math.Hypot(s.drift[c][0], s.drift[c][1]) * perSecond
		if carried > capacity {
			d := math.Min(carried-capacity, room)
			lay(i, d)
			carried -= d
		} else if t.Mark == None {
			bed := float64(g.Soil[i]) * t.Sand
			e := math.Min(capacity-carried, bed)
			if e > 0 {
				soil := float64(g.Soil[i])
				rest := soil - e
				if rest > 1e-12 {
					t.Clay = t.Clay * soil / rest
					t.Sand = (bed - e) / rest
				} else {
					rest = 0
				}
				g.Soil[i] = float32(rest)
				g.Height[i] -= e
				carried += e
			}
		}
		load[r] += carried
		if indeg[r]--; indeg[r] == 0 {
			queue = append(queue, r)
		}
	}
}
