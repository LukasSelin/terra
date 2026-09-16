package terra

import (
	"github.com/LukasSelin/terra/geom"
	"math"
)

// Rivers that wander.
//
// The water on this map goes where the ground falls fastest and nowhere else.
// That is where a river starts, and it is not where a river stays: a channel
// with any water in it cuts the outside of every bend it makes and lays what
// it cut on the inside, so the bend grows, the channel walks sideways across
// its own valley, and what began as a line down the fall of the land becomes
// a river with a shape. Nothing here had that. Every channel was the steepest
// way down, drawn afresh each time the ground moved, which on a smooth
// hillside is a straight line along one of eight bearings - and it cut the
// same way whether it was a gully or the drainage of half the map.
//
// So this is the sideways half of what a river does, next to the downward
// half that erode.go already has. It is not a model of a meander in the way a
// hydrologist would want one: there is no bank shear here, no cutoff, and an
// oxbow only ever happens by accident. What it has is the one feedback that
// makes rivers look like rivers - the outside of a bend is where the water
// is fastest, so that is where the ground goes, so the bend gets bigger - and
// it is charged by how much water is doing it, so a great river remakes its
// valley while a stream scratches at it.

// greatFlow is the discharge, in cubic metres a second, of a great river on
// ground the size of these maps: ten litres a second, about what the trunk of
// the default valley carries (seven to seventeen over its first three seeds).
// What a river does to its banks and its bed goes as the root of its share of
// this, and no further - see greatShare.
//
// It was the greatest flow on each map, read afresh every age, which made how
// hard a river cut a fact about every other river on the map: a stream on a
// globe, a small share of a great trunk, wandered and incised a fraction of
// what the same stream did in a valley, and a map whose trunk silted into a
// lake had every other river on it cut harder the next age. A river's work is
// its own water's.
//
// The valley's rivers are sensitive to it the way a chaotic thing is, and the
// figure is picked among values that are all real sizes of a trunk. With the
// soil kept as its own depth (soil.go), at ten litres the third valley's river
// bends more after forty ages and the fifth valley's floor is finer than its
// hillsides; at twenty, thirty and fifty the floor came out the coarser, and
// at thirty the river lost bends. Before the soil was kept, ten and fifteen
// lost bends and twenty did not.
const greatFlow = 0.01

// greatShare is the share of a great river's work a river carrying q does.
func greatShare(q float64) float64 { return math.Min(1, q/greatFlow) }

// bankCut is how much of the outer bank of a bend a great river - see
// greatFlow - takes in an age of weather, in metres of height, off a bank as
// readily worn as the soil is: a bank of rock pays what the rock pays, which
// for a middling rock is bedShare of it, a quarter of a metre an age. That is
// the order real banks go: a river five metres wide moving a twentieth of its
// width a year past banks two metres high takes twelve cubic metres a year off
// them, a fifth of a metre of a tile's height a decade. A river migrating a
// metre or two an age across ground that is being cut by tens is what tips
// the drainage into the new course rather than the old one.
//
// It is charged as the root of the water crossing the tile, like everything
// else here: the difference between a great river and a small one is far less
// than the difference in what they carry, and a gully that wandered as freely
// as the trunk would unpick every hillside on the map.
// Measured rather than argued: over three seeds and forty ages, the share of
// river tiles that turn goes from about a half with no sideways cutting at
// all to two thirds at this figure. Twice as much buys almost nothing more -
// a hundredth on that reading - and starts pulling the hillsides about, and
// half as much buys half the bends.
const bankCut = 2.5

// pointBar is the share of what comes off the outside of a bend that goes
// back on the inside of it. It is not all of it - some goes downstream, which
// is what silts the flood plain - but it is most, because that is what a bend
// does: the water is slow on the inside and drops what it is carrying there.
// Without it a river digs its whole valley wider and lower every age instead
// of moving across it, and the map sinks.
//
// Nine tenths. It was seven, when the rest was laid on the bed of the next tile
// down and never left the valley. Given to the water instead - see bankLoad -
// the rest is carried, and what does not settle goes to the sea, so the share
// is a rate at which a valley's rivers export their own flood plain: at seven
// the low fifth of seed 3's valley lost 29 metres over forty ages and at nine
// tenths 13, nearly all of it the banks. At eight and seven the rivers of the
// first three valleys moved under the thousandth of a width a year the
// migration yardstick asks.
const pointBar = 0.9

// meanderFlow is the least discharge, in cubic metres a second, a channel has
// to carry before it wanders at all: a fiftieth of a great river's, which is
// what it was of the greatest on the map. Below it the water is in a gully cut
// into a hillside, and a gully does not meander: it has no flood plain to move
// about in and no strength to make one.
const meanderFlow = greatFlow / 50

// bend is the inside and the outside of a bend, as steps off the tile it turns
// at, for water that came in stepping in and goes on stepping out. The inside
// is the corner between the two legs of it - back up the way the water came,
// and on the way it goes - and the outside is the other way.
//
// It was a quarter turn off the way out, on the side the water turned from,
// and that was the wrong side: on a bend of a right angle it named the tile
// upstream as the outer bank. Every such bend took its bank cut out of its own
// bed above it and laid its point bar on the outside, which is a river
// straightening itself by digging up its own channel.
func bend(in, out geom.Pos) (inner, outer geom.Pos) {
	sign := func(v int) int { return min(1, max(-1, v)) }
	inner = geom.Pos{X: sign(out.X - in.X), Y: sign(out.Y - in.Y)}
	return inner, geom.Pos{X: -inner.X, Y: -inner.Y}
}

// meander walks every channel on the map and lets it cut the outside of its
// bends. by is the same count of ages that wear takes, so a history's epoch
// moves a river as far as it moves a hillside.
//
// It reads the drainage as it stands and writes heights, and what the water
// carries off - see bankLoad - so the caller has to work the water out again
// afterwards, which is where the river actually moves. Nothing here moves it;
// this only makes the ground it will move into.
func (g *Grid) meander(by float64) {
	// Where each tile's water comes from: of everything draining into it, the
	// one carrying the most, which is the channel and not the hillside.
	from := make([]geom.Pos, len(g.Tiles))
	best := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		p := geom.Pos{X: i % g.W, Y: i / g.W}
		d := g.flowStep(i)
		if d == (geom.Pos{}) {
			continue
		}
		q := geom.Pos{X: p.X + d.X, Y: p.Y + d.Y}
		if g.Wrap {
			q = g.Norm(q)
		}
		if !g.In(q) {
			continue
		}
		if f := g.Flow[i]; f > best[g.Index(q)] {
			best[g.Index(q)], from[g.Index(q)] = f, d
		}
	}

	change := make([]float64, len(g.Tiles))
	if len(g.bankLoad) != len(g.Tiles) {
		g.bankLoad = make([][Grains]float64, len(g.Tiles))
	}
	for i := range g.Tiles {
		if t := &g.Tiles[i]; g.Flow[i] < meanderFlow || t.Mark != None || t.Owner != 0 {
			continue // a trickle, or a reach somebody holds and has embanked
		}
		share := greatShare(g.Flow[i])
		in := from[i]
		if in == (geom.Pos{}) {
			continue // nothing above it: a spring has no bend to cut
		}
		p := geom.Pos{X: i % g.W, Y: i / g.W}
		out := g.flowStep(i)
		if out == (geom.Pos{}) {
			continue
		}
		// Which way the water is turning here. Straight on cuts nothing:
		// a river with no bend in it has no outside.
		cross := in.X*out.Y - in.Y*out.X
		if cross == 0 {
			continue
		}
		inner, outer := bend(in, out)

		// Charged by the water and paid by the rock: the same river takes a
		// bend out of shale in an age and hardly marks a granite one. It is
		// the bank's own rock that pays, not the channel's, which is what
		// turns a river aside rather than letting it saw through.
		took := by * bankCut * math.Sqrt(share) * g.rockAt(p, outer)
		bank, ok := g.bankAt(p, outer) // the index of the bank tile
		if !ok {
			continue
		}
		// A river takes its bank down to its own bed and no further: what is
		// below the water is the bed, and the bed is the downward cutting's.
		// Taken whole whatever stood there, the outside of a bend on a flood
		// plain a metre above the water was dug metres below it, and the
		// point bar opposite stood metres above the plain.
		took = math.Min(took, math.Max(0, g.Height[bank]-g.Height[i]))
		if took <= 0 || !g.shift(p, outer, -took, change) {
			continue
		}
		// Most of it goes straight onto the inside of the same bend, and the
		// rest goes downstream with the water. Letting the rest simply
		// disappear was the first way this was written and it is a hole in
		// the bottom of the map: a river would take a metre off its banks
		// every age and put seven tenths of it back, and the world would
		// quietly lose the difference for ever.
		//
		// Where the inside of the bend is not there to take it - held ground,
		// or the edge of the map - the bar is laid on the channel where it was
		// cut, which is where a bar the water could not put anywhere else ends
		// up. Dropping it was the same hole again.
		if !g.shift(p, inner, took*pointBar, change) {
			change[i] += took * pointBar
		}
		// And what goes downstream goes into the water, to be carried and let
		// settle by the next wear as it carries what it cuts. It was laid on
		// the bed of the next tile down, which is not downstream with the
		// water but a step in the river: a great river put a metre on its own
		// bed at every bend every age, and on the valleys as they were when
		// this was found, forty ages of it took the concavity of their rivers
		// from 0.51 to 0.16, steep where they should have been gentle.
		for gr, part := range g.parts(bank) {
			g.bankLoad[i][gr] += took * (1 - pointBar) * part
		}
	}

	// No floor under it. What a floor at nothing did was raise a bank the river
	// had cut below sea level back up to it, out of nothing: see wear.
	for i := range g.Tiles {
		g.Height[i] += change[i]
	}
}

// shift books a change of height on the tile one step off p, and says whether
// there was one to book. Ground somebody has built on or claimed is left
// alone: a river takes a bank, and a settlement that has put something on
// that bank is holding it against the river for as long as it stands there.
func (g *Grid) shift(p, off geom.Pos, by float64, change []float64) bool {
	q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
	if g.Wrap {
		q = g.Norm(q)
	}
	if !g.In(q) {
		return false
	}
	t := g.At(q)
	if t.Mark != None || t.Owner != 0 {
		return false
	}
	change[g.Index(q)] += by
	return true
}

// bankAt is the tile one step off p, if there is one.
func (g *Grid) bankAt(p, off geom.Pos) (int, bool) {
	q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
	if g.Wrap {
		q = g.Norm(q)
	}
	if !g.In(q) {
		return -1, false
	}
	return g.Index(q), true
}

// rockAt is how readily the rock one step off p wears - see rockErodibility -
// or a middling rock where there is no tile there to ask.
func (g *Grid) rockAt(p, off geom.Pos) float64 {
	q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
	if g.Wrap {
		q = g.Norm(q)
	}
	if !g.In(q) {
		return bedShare
	}
	return rockErodibility(g.At(q))
}

func sign(v int) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	}
	return 0
}
