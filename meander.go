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
// ground the size of these maps: twenty litres a second, above the seven to
// seventeen the trunk of the default valley carries over its first three
// seeds, so that no valley's river is held at it. What a river does to its
// banks and its bed goes as the root of its share of this, and no further -
// see greatShare.
//
// The bends are sensitive to it the way a chaotic thing is: at ten and fifteen
// litres the third valley's river came out of forty ages turning on fewer of
// its tiles than it started with (0.454 to 0.433), and at five and twenty on
// more. Twenty is the one of those that leaves no river capped.
//
// It was the greatest flow on each map, read afresh every age, which made how
// hard a river cut a fact about every other river on the map: a stream on a
// globe, a small share of a great trunk, wandered and incised a fraction of
// what the same stream did in a valley, and a map whose trunk silted into a
// lake had every other river on it cut harder the next age. A river's work is
// its own water's.
const greatFlow = 0.02

// greatShare is the share of a great river's work a river carrying q does.
func greatShare(q float64) float64 { return math.Min(1, q/greatFlow) }

// bankCut is how much of the outer bank of a bend a great river - see
// greatFlow - takes in an age of weather, in metres of height. A river migrating a
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
const pointBar = 0.7

// meanderFlow is the least discharge, in cubic metres a second, a channel has
// to carry before it wanders at all: a fiftieth of a great river's, which is
// what it was of the greatest on the map. Below it the water is in a gully cut
// into a hillside, and a gully does not meander: it has no flood plain to move
// about in and no strength to make one.
const meanderFlow = greatFlow / 50

// turn is a quarter turn of a step, one way and the other. A bend's outer
// bank is a quarter turn off the way the water is going, on the side it is
// turning away from.
func turn(d geom.Pos, left bool) geom.Pos {
	if left {
		return geom.Pos{X: d.Y, Y: -d.X}
	}
	return geom.Pos{X: -d.Y, Y: d.X}
}

// meander walks every channel on the map and lets it cut the outside of its
// bends. by is the same count of ages that wear takes, so a history's epoch
// moves a river as far as it moves a hillside.
//
// It reads the drainage as it stands and writes only heights, so the caller
// has to work the water out again afterwards - which is where the river
// actually moves. Nothing here moves it; this only makes the ground it will
// move into.
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
		if f := g.Tiles[i].Flow; f > best[g.Index(q)] {
			best[g.Index(q)], from[g.Index(q)] = f, d
		}
	}

	change := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		if g.Tiles[i].Flow < meanderFlow {
			continue
		}
		share := greatShare(g.Tiles[i].Flow)
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
		outer := turn(out, cross < 0)
		inner := turn(out, cross > 0)

		// Charged by the water and paid by the rock: the same river takes a
		// bend out of shale in an age and hardly marks a granite one. It is
		// the bank's own rock that pays, not the channel's, which is what
		// turns a river aside rather than letting it saw through.
		took := by * bankCut * math.Sqrt(share) / g.rockAt(p, outer)
		if !g.shift(p, outer, -took, change) {
			continue
		}
		// Most of it goes straight onto the inside of the same bend, and the
		// rest goes downstream with the water. Letting the rest simply
		// disappear was the first way this was written and it is a hole in
		// the bottom of the map: a river would take a metre off its banks
		// every age and put seven tenths of it back, and the world would
		// quietly lose the difference for ever.
		g.shift(p, inner, took*pointBar, change)
		g.shift(p, out, took*(1-pointBar), change)
	}

	for i := range g.Tiles {
		if change[i] != 0 {
			g.Tiles[i].Height = math.Max(0, g.Tiles[i].Height+change[i])
		}
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

// rockAt is how hard the rock is one step off p, or a middling rock where
// there is no tile there to ask.
func (g *Grid) rockAt(p, off geom.Pos) float64 {
	q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
	if g.Wrap {
		q = g.Norm(q)
	}
	if !g.In(q) {
		return 1
	}
	return g.At(q).Hard()
}
