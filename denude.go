package terra

import (
	"math"
	"slices"

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
	// shelfPool is how many tiles across the blocks are whose mouths lay
	// what they bring together: see pool. Four is a hundred and fifty
	// kilometres at the globe's span.
	shelfPool = 4
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
// in book. A sea it fills to the shelf's top - a basin under the sea's level
// with no way out to the ocean but over the land - passes the rest on to the
// ocean by the shortest way over the land: the river goes on across the
// filled basin to the open sea. It returns how much was laid, how much of
// that went on past a filled sea, and how much went off the map's edge or
// found no room, in metres over a tile.
//
// The mouths are taken in tile order and each lays its own, so what a world
// lays is the same however many goroutines made it.
func (g *Grid) shelve(toSea [][Grains]float64, epoch int, book []record) (laid, spilt, lost float64) {
	defer phase.Start("shelve")()
	sh := g.shelfWork()
	sh.pool(g, toSea)
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
		rem := load
		left := sh.fill(g, m, &rem, epoch, book, false)
		if left > 0 && !sh.ocean(m) {
			// A filled basin: on over the land to the ocean, and laid off
			// where the way reaches it.
			at := int32(m)
			for !sh.ocean(int(at)) && sh.way[at] >= 0 {
				at = sh.way[at]
			}
			if sh.ocean(int(at)) {
				was := left
				left = sh.fill(g, int(at), &rem, epoch, book, true)
				spilt += was - left
			}
		}
		laid += total - left
		lost += left
	}
	return laid, spilt, lost
}

// pool gathers what the mouths within a block shelfPool tiles across send
// into one body of water onto the one of them that sends the most, which lays
// it all. A coast's currents carry what its rivers bring along it, and laid
// mouth by mouth, each of the thousands of tiles of a globe's coast walked out
// across the whole width of its shelf to find room, a ninth of the making of
// a globe.
func (sh *shelfScratch) pool(g *Grid, toSea [][Grains]float64) {
	for by := 0; by < g.H; by += shelfPool {
		for bx := 0; bx < g.W; bx += shelfPool {
			best, most := -1, 0.0
			for y := by; y < min(by+shelfPool, g.H); y++ {
				for x := bx; x < min(bx+shelfPool, g.W); x++ {
					m := y*g.W + x
					if c := carrying(toSea[m]); c > most && g.sunk(m) {
						best, most = m, c
					}
				}
			}
			if best < 0 {
				continue
			}
			for y := by; y < min(by+shelfPool, g.H); y++ {
				for x := bx; x < min(bx+shelfPool, g.W); x++ {
					m := y*g.W + x
					if m == best || !g.sunk(m) || sh.body[m] != sh.body[best] {
						continue
					}
					for gr := range toSea[m] {
						toSea[best][gr] += toSea[m][gr]
					}
					toSea[m] = [Grains]float64{}
				}
			}
		}
	}
}

// shelfScratch is shelve's working, kept on the Grid while a history runs:
// which body of water under the sea's level each tile is in, which of them is
// the ocean, the way from every other tile toward the ocean, and a fill's
// rings.
type shelfScratch struct {
	body             []int32
	way              []int32
	seen             []int32
	stamp            int32
	main             int32
	ring, next, todo []int32
}

// ocean reports whether tile i is in the ocean: the largest body of water.
func (sh *shelfScratch) ocean(i int) bool { return sh.main >= 0 && sh.body[i] == sh.main }

// shelfWork reads the bodies of water and the ways to the ocean for an
// epoch's shelving.
func (g *Grid) shelfWork() *shelfScratch {
	n := len(g.Tiles)
	var near [8]int32
	sh := &g.stepScratch.shelf
	sh.body, sh.way, sh.seen = sized(sh.body, n), sized(sh.way, n), sized(sh.seen, n)
	for i := range n {
		sh.body[i], sh.way[i], sh.seen[i] = -1, -1, 0
	}
	sh.stamp = 0
	// The bodies, each flooded from its first tile.
	var sizes []int
	for i := range n {
		if sh.body[i] >= 0 || !g.sunk(i) {
			continue
		}
		k := int32(len(sizes))
		sh.body[i] = k
		sh.todo = append(sh.todo[:0], int32(i))
		count := 0
		for len(sh.todo) > 0 {
			t := sh.todo[len(sh.todo)-1]
			sh.todo = sh.todo[:len(sh.todo)-1]
			count++
			c := g.around(int(t), &near)
			for _, j := range near[:c] {
				if sh.body[j] < 0 && g.sunk(int(j)) {
					sh.body[j] = k
					sh.todo = append(sh.todo, j)
				}
			}
		}
		sizes = append(sizes, count)
	}
	sh.main = -1
	for k, c := range sizes {
		if sh.main < 0 || c > sizes[sh.main] {
			sh.main = int32(k)
		}
	}
	// The ways: breadth first out of the ocean over everything else.
	sh.ring = sh.ring[:0]
	for i := range n {
		if sh.ocean(i) {
			sh.ring = append(sh.ring, int32(i))
		}
	}
	for len(sh.ring) > 0 {
		sh.next = sh.next[:0]
		for _, t := range sh.ring {
			c := g.around(int(t), &near)
			for _, j := range near[:c] {
				if !sh.ocean(int(j)) && sh.way[j] < 0 {
					sh.way[j] = t
					sh.next = append(sh.next, j)
				}
			}
		}
		sh.ring, sh.next = sh.next, sh.ring
	}
	return sh
}

// around writes the tiles round tile i into near, round the world where it
// goes round, and returns how many there are.
func (g *Grid) around(i int, near *[8]int32) int {
	x, y := i%g.W, i/g.W
	k := 0
	for dy := -1; dy <= 1; dy++ {
		ny := y + dy
		if ny < 0 || ny >= g.H {
			continue
		}
		for dx := -1; dx <= 1; dx++ {
			nx := x + dx
			if dx == 0 && dy == 0 {
				continue
			}
			if nx < 0 || nx >= g.W {
				if !g.Wrap {
					continue
				}
				nx = (nx + g.W) % g.W
			}
			near[k] = int32(ny*g.W + nx)
			k++
		}
	}
	return k
}

// fill lays left metres of a mouth's load of total on the floor of the body
// of water out from tile m, the nearest floor first, up to the shelf's top:
// each ring out a shelf's fall deeper, or level for what a filled basin
// passed on. rem is what is left of the load, grain by grain, and each ring
// takes out of it what settles crossing it, as far as it has room: the sand
// within a few tens of kilometres of the mouth and the mud carried on further
// (shelfReach). What a ring has no room for goes on past it, so a shelf
// filled to its top passes its sand on to its edge, and builds out. It
// returns what it had no room for, once the whole body is filled.
func (sh *shelfScratch) fill(g *Grid, m int, rem *[Grains]float64, epoch int, book []record, passed bool) float64 {
	base := math.Max(0, g.base)
	var near [8]int32
	sh.stamp++
	sh.ring = append(sh.ring[:0], int32(m))
	sh.seen[m] = sh.stamp
	left := carrying(*rem)
	last := settleLast * left
	for d := 0; len(sh.ring) > 0 && left > 0; d++ {
		top := base - shelfTop
		if !passed {
			top -= shelfFall * float64(d) * g.span()
		}
		room := 0.0
		for _, i := range sh.ring {
			room += math.Max(0, top-g.Height[i])
		}
		if room > 0 {
			// What settles crossing the ring, or the last of the load.
			var want [Grains]float64
			all := 0.0
			for gr := range rem {
				want[gr] = rem[gr] * settleWeight(Grain(gr), g.span())
				all += want[gr]
			}
			if left-all <= last {
				want, all = *rem, left
			}
			// Where the ring has less room than that, what settles first
			// takes it, and the finer goes on past: the sand fills a shelf
			// to its top and the mud is carried over it.
			free := room
			for gr := range want {
				want[gr] = math.Min(want[gr], free)
				free -= want[gr]
			}
			for _, i := range sh.ring {
				if r := top - g.Height[i]; r > 0 {
					var part [Grains]float64
					for gr := range part {
						part[gr] = want[gr] * r / room
					}
					g.layShelf(int(i), part, epoch, book)
				}
			}
			for gr := range rem {
				rem[gr] = math.Max(0, rem[gr]-want[gr])
			}
			left = carrying(*rem)
		}
		if left <= 0 {
			break
		}
		sh.next = sh.next[:0]
		for _, t := range sh.ring {
			c := g.around(int(t), &near)
			for _, j := range near[:c] {
				if sh.seen[j] != sh.stamp && g.sunk(int(j)) {
					sh.seen[j] = sh.stamp
					sh.next = append(sh.next, j)
				}
			}
		}
		sh.ring, sh.next = sh.next, sh.ring
	}
	return left
}

// layShelf lays part, a bed's grains in metres, on tile i, as a bed of
// sandstone where it is sandy and of shale where it is not (laidAs), and books
// its grains.
func (g *Grid) layShelf(i int, part [Grains]float64, epoch int, book []record) {
	put := carrying(part)
	if put <= 0 {
		return
	}
	was := g.Height[i]
	g.Height[i] += put
	sand := part[Sand] / put
	if g.strata != nil {
		g.strata[i].lay(laidAs(sand), uint8(epoch), uint8(max(1, 255*clamp01(sand))), was, g.Height[i])
	}
	if g.ledger != nil {
		g.ledger[i].bury(byMud, epoch)
	}
	if book != nil {
		for gr := range part {
			book[i].laid[gr] += part[gr]
		}
	}
}

// shelfReach is how far out from a mouth each grain is carried over a shelf
// before it settles, in metres: the sand within a few tens of kilometres, on
// the shoreface and the inner shelf, and the mud out over the middle and the
// outer shelf and down the slope, where most of what the rivers bring the sea
// ends up (McCave 1972; Walsh and Nittrouer 2009 have the mud of the great
// rivers' shelves laid from tens to a couple of hundred kilometres off their
// mouths). Each ring out from a mouth takes the share of what is still
// carried that settles crossing it (settleWeight), so the sand is laid first
// and nearest and the clay last and furthest.
//
// settleLast is the share of a load left carried at which the rest of it is
// laid where it is: a mouth's mud thinned out over the whole of an ocean is
// rings without end for metres that are not there.
var shelfReach = [Grains]float64{Sand: 20 * km, Silt: 100 * km, Clay: 200 * km}

const settleLast = 0.05

// settleWeight is the share of what is still carried of grain gr that settles
// crossing a ring span metres wide.
func settleWeight(gr Grain, span float64) float64 {
	return -math.Expm1(-span / shelfReach[gr])
}

// laidAs is the rock a bed laid by water comes out as, by the share of sand
// in it: sandstone where sand is at least half of it, as Folk (1954) draws
// the line between a sand and a mud, and shale where it is not.
func laidAs(sand float64) Bedrock {
	if sand >= sandyBed {
		return Sandstone
	}
	return Shale
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
