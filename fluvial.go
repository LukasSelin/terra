package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
)

// The water cutting the ground, and putting it down again.
//
// Stream power: a river takes its bed down at a rate set by how much water it
// carries and how steeply it falls, E = K·Q^m·S, with m a half. That was
// always the law here. What was wrong was the way it was taken. Each age read
// the fall as it stood before the age began and took off what that fall was
// worth, so a tile could be cut below the ground it drains into - a steep
// short reach, taken at the rate its fall was at the start, overshoots - and
// the only guard against it was that nothing was ever asked to move fast.
//
// Braun and Willett (2013) take the fall as it stands at the end of the step
// instead. Walked from the sea upward, every tile's receiver is already
// settled when the tile is reached, so the new height of the tile is one line
// of arithmetic, and it can never fall below the ground its water goes to
// however long the step. It is the method FastScape is built on, and it is
// O(n): one ordering of the tiles and one pass.
//
// What the water carries it puts down again, and that is Yuan and others
// (2019): the share of what passes a tile that settles there goes as how
// quickly it falls out of the water against how much water there is to keep
// it up. Each grain settles at its own rate - see depositOf - which is the
// sorting; and since what a tile receives depends on what settled above it,
// the two are solved together, a few sweeps up and down the same order.
// Everything the water takes is booked to where it lands or to the sea, grain
// by grain, and nothing is made or lost on the way.

// Erodibility is K in E = K·Q^m·S: metres of ground a year takes off a tile
// with one cubic metre a second running over it down a fall of one in one,
// before what is growing on it and what it is made of - see hold - have their
// say. It is not a rock's figure. The fall is read over a tile and the water
// off the tile's own ground, and the size of it is set by what real ground
// loses: a ploughed slope a millimetre and a half of soil a year, the median
// of Montgomery's (2007) compilation, and a valley nobody farms a twentieth of
// a millimetre, the median Portenga and Bierman (2011) read off the sand of
// real catchments. A hillside farmed hard loses its soil in a couple of
// thousand years and one left standing keeps it for a hundred thousand.
//
// It was two, which had ploughed slopes losing twelve millimetres a year and
// the untouched valley more than a millimetre - both about ten times anything
// measured. Ploughed is one age of weather on the slopes of seed 3's 60 by 40
// valley; untouched is what the water carried off valleys 1 to 3 over forty
// ages, spread over them. With grass and wood holding what they hold now:
//
//	K      ploughed mm/yr   untouched mm/yr
//	0.25        1.64             0.053
//
// Before that it was Wash, twelve, charged against the root of the share of the
// map's water a tile carried - which made how fast a hillside wore a fact
// about how big the map was: a globe's hillsides, each a smaller share of a
// larger whole, wore eleven times slower than the valley's. Charged against
// the water itself it is the same on every map, and it is set so that the
// valley wears as it did. Seed 3 of a 60 by 40 valley over forty ages:
//
//	K      high ground   low ground   ploughed against wooded   sorting, seed 5
//	was       -4003         +4355              2.83                  0.023
//	1.5       -3019         +3656              2.99                  0.019
//	2.0       -3690         +3762              2.94                  0.027
//
// The figures in both tables were read with the water off twelve hundred
// metres of catchment a tile, 2304 times the ground the tile is: see
// weather.go. The water now runs off the tile, and since the cutting goes as
// the root of the water, the same cutting is had at 48 times the figure - K
// is Whipple and Tucker's (1999) coefficient, and its units - metres of
// ground a year for each root of a cubic metre a second, down a fall of one -
// are what dimensional analysis carries from one discharge to the other, and
// from one clock to another. 0.25 an age off the old water is 12 an age off
// the real water, which is 1.2 a year.
//
// Read as the law on drainage area, E = K_A·A^m·S, it is K_A = K·r^m for
// runoff r in metres a second: at the valley's four hundred millimetres a
// year, 1.3e-4 a year, and times what grass holds, 8e-6 - inside the 1e-7 to
// 1e-4 real channels in rock of every kind are fitted at (Stock and
// Montgomery 1999; Harel, Mudd and Attal 2016). So a history, which is millions
// of years of it, wears with the same K on the same clock: see epochYears.
const Erodibility = 1.2

// depositOf is how readily each grain comes out of the water on ground that
// lets it, as a share of what passes: sand at the first slackening, silt where
// the river spills, clay hardly at all while there is water moving. The
// figures are what the settling was before it was solved with the cutting,
// kept so that a map sorts its soil as it did.
//
// Steep ground keeps its load moving whatever the grain: see SettleSlope.
var depositOf = [Grains]float64{Sand: 0.62, Silt: 0.33, Clay: 0.10}

// settleFlow is the discharge, in cubic metres a second, above which a river
// has the water to keep more of its load up: the share it lets settle goes as
// the root of settleFlow over its flow. Below it every trickle is the same.
// It is Yuan's G/q with the flow taken per unit width of a bed as wide as the
// root of its discharge, which is how the world's rivers widen.
//
// It was one, read off water gathered from 2304 times a tile's ground - see
// weather.go - and it is the same line at the water the ground really sheds:
// under half a litre a second, a trickle off the first few hundred square
// metres of a hillside. At ten and at a hundred times that - letting every
// river up to that size drop its load as a trickle would - the valley wore
// and sorted within a few hundredths of the same, and sent a sixth less to the
// sea.
const settleFlow = 1.0 / 2304

// settleIters is how many sweeps up and down the order the cutting and the
// settling are solved with. It is fixed, so an age comes out the same however
// close it came.
const settleIters = 4

// receivers is where each tile's water goes, by index, and how far that is on
// the ground. A tile whose water goes nowhere - a hollow, the edge of a valley,
// the sea, still water - is its own receiver and is a root: its height is what
// everything above it is cut down toward.
func (g *Grid) receivers() (recv []int32, run []float64) {
	if g.deep > 0 {
		return g.deepReceivers()
	}
	n := len(g.Tiles)
	recv = make([]int32, n)
	run = make([]float64, n)
	span := g.span()
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			recv[i], run[i] = int32(i), span
			if g.sunk(i) || g.standing(i) {
				continue
			}
			p := g.PosOf(i)
			a := g.Aspect(p)
			if a == (geom.Pos{}) {
				continue
			}
			q := geom.Pos{X: p.X + a.X, Y: p.Y + a.Y}
			if !g.In(q) {
				continue
			}
			recv[i] = int32(g.Index(q))
			if a.X != 0 && a.Y != 0 {
				run[i] = span * math.Sqrt2
			}
		}
	})
	return recv, run
}

// deepReceivers is receivers for a history: where each tile's water goes
// across the ground with its hollows filled to where they spill, so that the
// only roots are the sea and the edge of the map.
//
// A settlement's decade has lakes in it, and a lake is where the cutting
// stops. An epoch is millions of years, and on that clock a hollow does not
// stay one: it fills with what the water brings it until it spills, and its
// outlet is cut down, or it is a basin of the desert whose floor is the only
// ground in it that is not worn. Read with its lakes as roots, a history's
// ranges - raised kilometres an epoch at real rates - stood in their own
// hollows where no water ever left them, and rose without end: six hundred
// kilometres by the eighth epoch of a valley. Routing over the filled ground
// is what FastScape does for the same reason (Cordonnier, Bovy and Braun
// 2019), and the solve already leaves a tile at or under the ground it drains
// to uncut, so the floor of a filled hollow is only ever built up.
func (g *Grid) deepReceivers() (recv []int32, run []float64) {
	n := len(g.Tiles)
	recv = make([]int32, n)
	run = make([]float64, n)
	span := g.span()
	h := make([]float64, n)
	root := make([]bool, n)
	for i := range g.Tiles {
		h[i] = g.Tiles[i].Height
		if p := g.PosOf(i); g.sunk(i) || g.outlet(p.X, p.Y) {
			root[i] = true
		}
	}
	g.fillFrom(h, root)
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			recv[i], run[i] = int32(i), span
			if root[i] {
				continue
			}
			p := g.PosOf(i)
			steepest := 0.0
			for _, off := range Dirs {
				q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
				if g.Wrap {
					q = g.Norm(q)
				}
				if !g.In(q) {
					continue
				}
				j := g.Index(q)
				d := span
				if off.X != 0 && off.Y != 0 {
					d *= math.Sqrt2
				}
				if fall := (h[i] - h[j]) / d; fall > steepest {
					recv[i], run[i], steepest = int32(j), d, fall
				}
			}
		}
	})
	return recv, run
}

// stackOf orders the tiles so that every tile comes after the one its water
// goes to: the roots first, in index order, and then everything that drains
// into each, breadth first. Walked forward it goes from the sea upstream;
// walked backward, from the ridges down.
func stackOf(recv []int32) []int32 {
	n := len(recv)
	count := make([]int32, n+1)
	for i, r := range recv {
		if int(r) != i {
			count[r+1]++
		}
	}
	for i := 0; i < n; i++ {
		count[i+1] += count[i]
	}
	donors := make([]int32, count[n])
	next := append([]int32(nil), count[:n]...)
	for i, r := range recv {
		if int(r) != i {
			donors[next[r]] = int32(i)
			next[r]++
		}
	}
	stack := make([]int32, 0, n)
	for i, r := range recv {
		if int(r) == i {
			stack = append(stack, int32(i))
		}
	}
	for k := 0; k < len(stack); k++ {
		i := stack[k]
		stack = append(stack, donors[count[i]:count[i+1]]...)
	}
	return stack
}

// fluvial is one step of the water's work on a column of ground: the heights,
// where each tile's water goes and in what order, how hard the water cuts on
// each (F, the implicit coefficient K·Q^m/run times the step), how readily
// each grain settles there, and what each tile is made of.
//
// On a coast with a tide there are two more. floor is the least height the
// water cuts toward at each tile: an estuary's river runs into a sea that
// stands at high water half the day, and does not cut its bed below it. keep
// is how much of what reaches a root the root holds, and room how much it has
// room for: a flat under mean sea is where the tide lays its mud, and it lays
// it until the flat stands at high water. Both are nil where there is no tide.
//
// And where the ground has soil on it there are three more. soil is how deep
// it is, f is how hard the water cuts it, and rock how hard the water cuts
// the rock under it: a tile the water takes more than its soil off in a step
// is cut at the one until the soil is gone and at the other after. eff is the
// rate that comes to over the step, which is what the step is booked at. All
// three are nil where the ground is one thing all the way down.
//
// abrade is the share of the sand passing each tile that the passage wears
// down to silt: see Sternberg. It is nil where nothing is worn.
//
// edge is, in a history, the height a root on the map's edge cuts toward - the
// lower ground its water leaves for - and +Inf where a root cuts toward
// nothing. It is nil outside a history. See edgeWork.
type fluvial struct {
	h      []float64
	recv   []int32
	stack  []int32
	f      []float64
	settle [][Grains]float64
	parts  [][Grains]float64
	floor  []float64
	keep   []float64
	room   []float64
	soil   []float64
	rock   []float64
	eff    []float64
	abrade []float64
	edge   []float64
}

// edgeCut is how much the water takes off root i, whose height at the end of
// the step is next: nothing, unless it is an edge that cuts toward lower ground.
func (c *fluvial) edgeCut(i int32, next float64) float64 {
	if c.edge == nil || !(next > c.edge[i]) {
		return 0
	}
	return c.f[i] * (next - c.edge[i])
}

// Sternberg is how fast what a river carries is worn finer as it goes: the
// size of its grains falls as D = D0·e^(−αx) with the distance x it has come
// (Sternberg 1875). Gravel-bed rivers fine at a hundredth to a tenth of a
// kilometre (Hoey and Ferguson 1994); the lower end is taken, for the sand of
// a river rather than its cobbles. Per kilometre.
const Sternberg = 0.01

// sandFolds is how many e-foldings of grain size the sand spans, from two
// millimetres down to the sixteenth of one where silt begins: ln 32. Grains
// spread evenly over that span in the logarithm of their size cross into silt
// at α over it per unit of distance, which is how the share of sand turned to
// silt is read off a rate that is about sizes.
var sandFolds = math.Log(32)

// abrasion is the share of the sand that run metres of river wears to silt.
func abrasion(run float64) float64 {
	return -math.Expm1(-Sternberg * run / 1000 / sandFolds)
}

// rate is how hard the water cuts tile i over the step, given how far above
// the ground it drains into the tile stood at the last sweep: its soil's rate
// for the share of the step the soil lasts, and its rock's for the rest. The
// soil lasts the whole step if the cut at its rate is no deeper than it is.
func (c *fluvial) rate(i int32, above float64) float64 {
	if c.rock == nil {
		return c.f[i]
	}
	f := c.f[i]
	if cut := f * math.Max(0, above); cut > c.soil[i] {
		lasts := c.soil[i] / cut
		f = lasts*f + (1-lasts)*c.rock[i]
	}
	c.eff[i] = f
	return f
}

// booked is the rate a tile's step is booked at: what rate settled on.
func (c *fluvial) booked(i int32) float64 {
	if c.eff == nil {
		return c.f[i]
	}
	return c.eff[i]
}

// pass sends what is carried off tile i on to r, wearing the sand as it goes.
func (c *fluvial) pass(load [][Grains]float64, i, r int32, gr int, passing float64) {
	if gr == int(Sand) && c.abrade != nil {
		worn := passing * c.abrade[i]
		load[r][Sand] += passing - worn
		load[r][Silt] += worn
		return
	}
	load[r][gr] += passing
}

// cutAt is how much the water took off tile i in the step that settled on
// next.
func (c *fluvial) cutAt(next []float64, i int32) float64 {
	r := c.recv[i]
	if r == i {
		return 0
	}
	return c.booked(i) * math.Max(0, next[i]-c.below(next, r))
}

// below is the height the water at a tile draining into r cuts toward: its
// receiver's, or the tide's high water there if that is higher.
func (c *fluvial) below(next []float64, r int32) float64 {
	if c.floor == nil {
		return next[r]
	}
	return math.Max(next[r], c.floor[r])
}

// solve is the heights at the end of the step, cut and filled together. With
// nothing settling it is exactly Braun and Willett: h' = (h + F·h'_r)/(1 + F),
// taken from the sea upward.
func (c *fluvial) solve(iters int) []float64 {
	n := len(c.h)
	next := append([]float64(nil), c.h...)
	cut := make([]float64, n)
	load := make([][Grains]float64, n)
	for it := 0; it < iters; it++ {
		// What arrives at each tile, from the ridges down, off the last
		// sweep's cutting and settling.
		for i := range load {
			load[i] = [Grains]float64{}
		}
		for k := len(c.stack) - 1; k >= 0; k-- {
			i := c.stack[k]
			r := c.recv[i]
			if r == i {
				continue
			}
			for gr := range load[i] {
				carried := load[i][gr] + cut[i]*c.parts[i][gr]
				c.pass(load, i, r, gr, carried*(1-c.settle[i][gr]))
			}
		}
		// The heights, from the sea up, with what arrives held where the last
		// sweep left it.
		for _, i := range c.stack {
			r := c.recv[i]
			if r == i {
				next[i], cut[i] = c.h[i], 0
				if c.edge != nil && c.h[i] > c.edge[i] {
					next[i] = (c.h[i] + c.f[i]*c.edge[i]) / (1 + c.f[i])
					cut[i] = c.edgeCut(i, next[i])
				}
				continue
			}
			hr := c.below(next, r)
			// next[i] is still the last sweep's here, which is what says how
			// much of the step the soil lasts.
			f := c.rate(i, next[i]-hr)
			var share, laid float64
			for gr := range load[i] {
				share += c.settle[i][gr] * c.parts[i][gr]
				laid += c.settle[i][gr] * load[i][gr]
			}
			fa := f * (1 - share)
			if c.h[i] <= hr {
				fa = 0 // at or under the water it runs into: nothing to cut toward
			}
			next[i] = (c.h[i] + fa*hr + laid) / (1 + fa)
			cut[i] = f * math.Max(0, next[i]-hr)
		}
	}
	return next
}

// account books the step: with the heights solve settled on, what each tile
// loses to the water, what settles on it by grain, and what goes to the sea.
// It is taken from the ridges down in one pass, so every grain the water takes
// is somewhere at the end of it.
func (c *fluvial) account(next []float64, change []float64, gained [][Grains]float64, lay func(i int32, laid [Grains]float64)) (exported [Grains]float64) {
	n := len(c.h)
	load := make([][Grains]float64, n)
	for k := len(c.stack) - 1; k >= 0; k-- {
		i := c.stack[k]
		r := c.recv[i]
		if r == i {
			var kept [Grains]float64
			if c.keep != nil && c.keep[i] > 0 {
				total := load[i][Sand] + load[i][Silt] + load[i][Clay]
				if hold := math.Min(c.keep[i]*total, c.room[i]); total > 0 && hold > 0 {
					for gr := range kept {
						kept[gr] = load[i][gr] * hold / total
					}
					if lay != nil {
						lay(i, kept)
					} else {
						for gr := range kept {
							change[i] += kept[gr]
							gained[i][gr] += kept[gr]
						}
					}
				}
			}
			cut := c.edgeCut(i, next[i])
			change[i] -= cut
			for gr := range load[i] {
				exported[gr] += load[i][gr] - kept[gr] + cut*c.parts[i][gr]
			}
			continue
		}
		cut := c.cutAt(next, i)
		change[i] -= cut
		var laid [Grains]float64
		for gr := range load[i] {
			carried := load[i][gr] + cut*c.parts[i][gr]
			laid[gr] = carried * c.settle[i][gr]
			c.pass(load, i, r, gr, carried-laid[gr])
		}
		if lay != nil {
			lay(i, laid)
		} else {
			for gr := range laid {
				change[i] += laid[gr]
				gained[i][gr] += laid[gr]
			}
		}
	}
	return exported
}

// edgeWork has, in a history, the roots on the map's edge cut toward the sea
// the history runs against. Outside a history an edge is where a valley's
// water leaves it and the ground there holds its height, which for a decade is
// true. For an epoch it is not: raised kilometres by a seam and never cut, the
// edges of a made valley and the poles of a made globe stood tens of
// kilometres high, the tallest ground on the map, because they were the only
// ground the water could not wear. The water leaving the map goes on down to
// the sea somewhere off it, and the edge is cut toward that.
func (g *Grid) edgeWork(c *fluvial, recv []int32, years float64) {
	if g.deep <= 0 {
		return
	}
	n := len(g.Tiles)
	c.edge = make([]float64, n)
	base := math.Max(0, g.base)
	for i := range n {
		c.edge[i] = math.Inf(1)
		if int(recv[i]) != i || g.sunk(i) {
			continue
		}
		t := &g.Tiles[i]
		if p := g.PosOf(i); !g.outlet(p.X, p.Y) || t.Height <= base {
			continue
		}
		c.edge[i] = base
		c.f[i] = years * Erodibility * math.Sqrt(t.Flow) * hold(t) / g.span()
	}
}

// stillWork has still water keep what reaches it. A lake, a salt flat and the
// bottom of a hollow are roots of the water's step, because the water goes no
// further down the ground from them; and a root sends what reaches it to the
// sea, which is right for the sea and the edge of a valley and nowhere else.
// Anywhere else what the water carried stops where the water does, and that
// is how a lake silts up and a basin fills.
func (g *Grid) stillWork(c *fluvial, recv []int32) {
	n := len(g.Tiles)
	for i := range n {
		if int(recv[i]) != i || g.sunk(i) {
			continue
		}
		if p := g.PosOf(i); g.outlet(p.X, p.Y) {
			continue
		}
		if c.keep == nil {
			c.keep, c.room = make([]float64, n), make([]float64, n)
		}
		c.keep[i], c.room[i] = 1, math.Inf(1)
	}
}
