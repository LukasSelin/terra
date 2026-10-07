package terra

import (
	"math"
	"slices"

	"github.com/LukasSelin/terra/geom"
	"github.com/LukasSelin/terra/internal/phase"
)

// Denudation: what a history's weather takes off a piece of a planet, and
// where it puts it.
//
// A history's tile is deepSpan across, thirty-seven and a half kilometres on
// the globe preset, and its step is an epoch, four million years. It wore by
// stream power, as a settlement's field does, E = K·Q^m·S, solved implicitly
// (Braun and Willett 2013). At that span and over that step the coefficient
// of the implicit solve, K·√Q·dt/dx, is fifty to two hundred: every tile is
// cut down to the tile its water goes to, and a whole drainage to its outlet,
// in one epoch. The low half of the land ended within 26 to 37 metres of the
// sea, where half the earth's stands under 461 (Cogley 1984), and what the
// rivers took to the sea left the crust, so the continents thinned from 35
// kilometres to 28 (G2, #41).
//
// The law is right for a channel and wrong for a tile. A tile that wide is a
// landscape: its rivers are graded to their outlets within an epoch, and
// what stands between them - most of the ground - comes down only as fast as
// its hillslopes and the regolith on them let it. Over a landscape, how fast
// the ground comes down goes with its relief. Ahnert (1970), over the
// mid-latitude basins whose sediment yield was measured, had denudation
// rising in proportion to the mean local relief, 0.1535 mm a thousand years
// for every metre of it; Montgomery and Brandon (2002), with cosmogenic and
// thermochronometric rates as well, had the same line below a kilometre of
// relief and the rate running away above it, where the slopes stand at the
// angle they fail at and the rivers set how fast they come down. And how
// fast the rivers carry it off goes with the water they have. So a history
// wears its ground as
//
//	E = k · R / (1 − (R/Rc)²) · (Q/Q₀)^½ · r
//
// with R the tile's relief over the ground its water goes to (or the sea, if
// that is lower), k Ahnert's figure, Rc reliefRunaway, Q the water through the
// tile against Q₀, what a tile's own ground sheds at the land's mean runoff,
// and r the rock (rockFactor). It is linear in the height over the receiver,
// as stream power is, so the implicit step takes it as it took stream power;
// on a tile that sheds only its own water the coefficient is six tenths,
// where stream power's was fifty. Two other laws were measured against it -
// stream power cutting the channels with the tile coming down to them at a
// hillslope's pace, and stream power on a tile's channel length in eight
// steps an epoch - and are in docs/perf/worklog.md.
//
// What the rivers bring the sea is not lost either. It is laid on the sea
// floor off the mouth it reached the sea by, as a shelf builds out: the
// floor nearest the mouth first, up to the shelf's top, and on out down a
// shelf's fall until all of it is laid (shelve). It is crust laid on the
// crust - the history thickens the crust under it by what it lays, and the
// plate sinks under the load - and a bed in the pile, sand or mud as the
// rivers sorted it. See TestTheHistoryStandsOnItsCrust.
const (
	// reliefWear is k: the share of its relief a year takes off ground,
	// 0.1535 mm a thousand years for every metre of mean local relief
	// (Ahnert 1970).
	reliefWear = 0.1535 * mm / kyr
	// reliefRunaway is Rc, the relief at which the rate runs away: twice
	// Ahnert's line at a little over a kilometre, where Montgomery and
	// Brandon's (2002) rates leave it. reliefSteepest is how near it the
	// running away is followed: at 0.99 of it the rate is fifty times the
	// line, some eleven millimetres a year off a kilometre and a half of
	// relief shedding its own water, which is what the fastest ranges come
	// down at (five to ten in the Southern Alps and on Taiwan). Held at ten
	// times the line, as the creep is, the highest tile of a globe stood
	// fourteen kilometres over its sea.
	reliefRunaway  = 1.5 * km
	reliefSteepest = 0.99
	// wornRunoff is the runoff Q₀ is read at, in millimetres a year: the
	// land's mean, some forty thousand cubic kilometres a year off a hundred
	// and thirty-five million square kilometres.
	wornRunoff = 300.0
	// shelfTop is how far under the sea the shelf off a mouth is built to,
	// and shelfFall how much deeper it lies for every metre further out: a
	// shelf is under a hundred and fifty metres of water or less to its
	// break, tens of kilometres out (Shepard 1963), and falls at a tenth of a
	// degree.
	shelfTop  = 50 * metre
	shelfFall = 1e-3
)

// deepRate is how hard a history's weather wears tile i, standing relief
// metres over what it drains to, over years: the coefficient of the implicit
// step. See the top of this file.
func (g *Grid) deepRate(i int, relief, years float64) float64 {
	wet := math.Sqrt(math.Max(g.Flow[i], 0) / discharge(wornRunoff, g.span()))
	return years * reliefRate(relief) * wet * rockFactor(&g.Tiles[i])
}

// reliefRate is the share of its relief a year takes off ground standing
// relief metres over what it drains to.
func reliefRate(relief float64) float64 {
	x := math.Min(reliefSteepest, math.Max(0, relief)/reliefRunaway)
	return reliefWear / (1 - x*x)
}

// rockFactor is how much faster than a middling rock the rock of t comes down
// as a landscape: the root of what the water's cutting makes of it. A
// landscape is not its channels' beds, and shale country comes down some three
// and a half times as fast as granite country where a channel cuts the one
// twelve times as fast as the other.
func rockFactor(t *Tile) float64 {
	return math.Sqrt(rockErodibility(t) / bedShare)
}

// shelve lays what the weather sent the sea from each mouth this epoch on the
// sea floor off it (see the top of this file), and books it in the pile and
// in book. A sea it fills to the shelf's top passes the rest on, over the
// land, to the nearest floor that has room: the river goes on across the
// filled basin to the next sea. It returns how much was laid, how much of
// that went on past the first sea, and how much went off the map's edge, in
// metres over a tile.
//
// The mouths are taken in tile order and each lays its own, so what a world
// lays is the same however many goroutines made it.
func (g *Grid) shelve(toSea [][Grains]float64, epoch int, book []record) (laid, spilt, lost float64) {
	defer phase.Start("shelve")()
	n := len(g.Tiles)
	base := math.Max(0, g.base)
	g.stepScratch.seen = sized(g.stepScratch.seen, n)
	seen := g.stepScratch.seen
	for i := range seen {
		seen[i] = -1
	}
	var ring, next, visited []int32
	for m := range toSea {
		load := toSea[m]
		total := carrying(load)
		if total <= 0 {
			continue
		}
		if !g.sunk(m) {
			lost += total // off the map's edge
			continue
		}
		left := total
		ring = append(ring[:0], int32(m))
		visited = visited[:0]
		seen[m] = int32(m)
		d := 0
		over := false // past the first sea, over the land
		for left > 0 {
			if len(ring) == 0 {
				if over {
					break
				}
				over = true
				ring = append(ring, visited...)
				d = 0
			}
			top := base - shelfTop - shelfFall*float64(d)*g.span()
			if over {
				top = base - shelfTop
			}
			for _, i := range ring {
				if !over || d > 0 {
					visited = append(visited, i)
				}
				if left <= 0 || !g.sunk(int(i)) {
					continue
				}
				if room := top - g.Height[i]; room > 0 {
					put := math.Min(room, left)
					g.layShelf(int(i), put, load, total, epoch, book)
					left -= put
					if over {
						spilt += put
					}
				}
			}
			next = next[:0]
			for _, i := range ring {
				p := g.PosOf(int(i))
				for _, off := range Dirs {
					q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
					if g.Wrap {
						q = g.Norm(q)
					}
					if !g.In(q) {
						continue
					}
					j := g.Index(q)
					if seen[j] == int32(m) || (!over && !g.sunk(j)) {
						continue
					}
					seen[j] = int32(m)
					next = append(next, int32(j))
				}
			}
			ring, next = next, ring
			d++
		}
		laid += total - left
		lost += left
	}
	return laid, spilt, lost
}

// layShelf lays put metres of a load on tile i, as a bed of sandstone where
// the load is sandy and of shale where it is not, and books its grains.
func (g *Grid) layShelf(i int, put float64, load [Grains]float64, total float64, epoch int, book []record) {
	was := g.Height[i]
	g.Height[i] += put
	sand := load[Sand] / total
	rock := Shale
	if sand >= sandyBed {
		rock = Sandstone
	}
	if g.strata != nil {
		g.strata[i].lay(rock, uint8(epoch), uint8(max(1, 255*clamp01(sand))), was, g.Height[i])
	}
	if g.ledger != nil {
		g.ledger[i].bury(byMud, epoch)
	}
	if book != nil {
		for gr := range load {
			book[i].laid[gr] += put * load[gr] / total
		}
	}
}

// denudation is what one epoch of weather took off a history's land, in
// millimetres a year: the median over the land, the mean, and the medians of
// its lower half and of its highest twentieth by height. Portenga and Bierman
// (2011), over the cosmogenic rates of the world's basins, have a median of
// 0.054 and a mean of 0.218, the shields and the old cratons a hundredth to a
// tenth, and the active ranges up to millimetres. shelved, spilt and lost are
// what shelve did with what reached the sea, in metres over a tile.
type denudation struct {
	median, mean, low, high float64
	shelved, spilt, lost    float64
}

// readDenudation reads an epoch's denudation off worn, the metres the weather
// took off each tile, and was, the heights before it.
func readDenudation(g *Grid, cr *crust, was, worn []float64) denudation {
	type at struct{ h, r float64 }
	var land []at
	for i := range worn {
		if cr.ocean[i] || was[i] <= g.base {
			continue
		}
		land = append(land, at{was[i], math.Max(0, worn[i]) / epochYears / mm})
	}
	var d denudation
	if len(land) == 0 {
		return d
	}
	median := func(part []at) float64 {
		rs := make([]float64, len(part))
		for k, l := range part {
			rs[k] = l.r
		}
		slices.Sort(rs)
		return rs[len(rs)/2]
	}
	for _, l := range land {
		d.mean += l.r
	}
	d.mean /= float64(len(land))
	d.median = median(land)
	slices.SortStableFunc(land, func(a, b at) int {
		switch {
		case a.h < b.h:
			return -1
		case a.h > b.h:
			return 1
		}
		return 0
	})
	d.low = median(land[:max(1, len(land)/2)])
	d.high = median(land[len(land)-max(1, len(land)/20):])
	return d
}
