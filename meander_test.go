package terra

import (
	"github.com/LukasSelin/terra/geom"
	"math"
	"testing"
)

// bends counts how often a channel changes bearing, over the channel tiles
// that have water coming into them and going out: a river of straight runs
// scores near nothing and one that wanders scores near one.
func bends(g *Grid) (float64, int) {
	from := make([]geom.Pos, len(g.Tiles))
	best := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		p := geom.Pos{X: i % g.W, Y: i / g.W}
		d := g.flowStep(i)
		if d == (geom.Pos{}) {
			continue
		}
		q := geom.Pos{X: p.X + d.X, Y: p.Y + d.Y}
		if !g.In(q) {
			continue
		}
		if f := g.Flow[i]; f > best[g.Index(q)] {
			best[g.Index(q)], from[g.Index(q)] = f, d
		}
	}
	turns, n := 0, 0
	for i := range g.Tiles {
		if !g.Tiles[i].Wet() || from[i] == (geom.Pos{}) {
			continue
		}
		out := g.flowStep(i)
		if out == (geom.Pos{}) {
			continue
		}
		n++
		if from[i] != out {
			turns++
		}
	}
	if n == 0 {
		return 0, 0
	}
	return float64(turns) / float64(n), n
}

// A river that has been somewhere a while has a shape. Left to the drainage
// alone a channel is the steepest way down, which on a smooth hillside is a
// straight line along one of eight bearings; cutting the outside of its own
// bends is what turns that into a river.
//
// What is asked is that the ages put bends in rather than take them out.
// Without any sideways cutting at all the same maps sat at about half their
// tiles turning and stayed there however long they ran, because nothing was
// making new bends - only the ground moving under the old ones.
//
// And that an old river is not a ruled line. It was asked to turn on half its
// tiles, which a river wandering over a flood plain does. The valleys are cut
// into their ground now - see shape.go - and a river at the bottom of a valley
// that steep has its bends set by the valley and moves across it slowly. A
// third is what is asked.
//
// The ages are asked of the three seeds together and not of each. The outside
// of a bend was cut on the wrong side until it was put on the outside - see
// bend - and a river cutting its own upstream bed straightened itself; put
// right, and the valleys cut by the water rather than by incise, seeds 2 and 3
// turn on 0.549 and 0.427 of their tiles young and 0.615 and 0.504 old, and
// seed 1 on 52 of 136 young and 52 of 139 old - one tile of river more and not
// one bend fewer, read seed by seed as straightening.
func TestRiversWanderAsTheyAge(t *testing.T) {
	var youngTurns, youngTiles, oldTurns, oldTiles float64
	for _, seed := range []uint64{1, 2, 3} {
		w := NewLand(seed, DefaultTerms())
		young, ny := bends(w.Grid)
		for k := 0; k < 40; k++ {
			w.Erode()
		}
		old, n := bends(w.Grid)
		if n < 50 {
			t.Fatalf("seed %d has only %d river tiles to read", seed, n)
		}
		if old < 1.0/3 {
			t.Errorf("seed %d: only %.3f of an old river turns; it is running in straight lines", seed, old)
		}
		youngTurns, youngTiles = youngTurns+young*float64(ny), youngTiles+float64(ny)
		oldTurns, oldTiles = oldTurns+old*float64(n), oldTiles+float64(n)
	}
	if young, old := youngTurns/youngTiles, oldTurns/oldTiles; !(old > young) {
		t.Errorf("%.3f of the rivers of three valleys turned when they were young and %.3f after forty ages", young, old)
	}
}

// The water charges the same and the rock pays differently. This is what
// gives a worn country a shape instead of a slope: the soft banks go, the
// hard ones turn the river aside, and what is left standing is the hard rock.
//
// It is the water and not the weather that this belongs to. Charging the
// hillsides for their rock as well was tried, and it took away the one cost
// the model puts on clearing a slope to farm it - ploughed ground over hard
// rock came out richer after forty ages than it started, because the soil
// could no longer leave faster than the ground made more. See hold.
func TestARiverCutsSoftRockFasterThanHard(t *testing.T) {
	cut := func(rock Bedrock) float64 {
		w := NewLand(3, DefaultTerms())
		g := w.Grid
		for i := range g.Tiles {
			g.Tiles[i].Bedrock = rock
		}
		was := make([]float64, len(g.Tiles))
		for i := range g.Tiles {
			was[i] = g.Height[i]
		}
		g.meander(20)
		moved := 0.0
		for i := range g.Tiles {
			moved += math.Abs(g.Height[i] - was[i])
		}
		return moved
	}
	soft, hard := cut(Shale), cut(Granite)
	if !(soft > hard) {
		t.Errorf("a river moved %.0f metres of shale and %.0f of granite", soft, hard)
	}
	// By a good deal and not by exactly the ratio of the two rocks: what a
	// bend puts back on its inside and sends downstream is charged to the
	// river rather than to the bank it came off, so the ground that moves
	// runs a little behind the strength of the rock. Three and a third
	// between these two rocks comes out as about two and three quarters.
	if got := soft / hard; got < 2 {
		t.Errorf("shale went only %.2f times as fast as granite", got)
	}
}

// A river takes its own bank and not a neighbour's wall. Ground somebody has
// built on or claimed stands against the water for as long as it stands.
func TestAMeanderLeavesHeldGroundAlone(t *testing.T) {
	w := NewLand(1, DefaultTerms())
	g := w.Grid
	var held []int
	for i := range g.Tiles {
		if g.Tiles[i].Wet() {
			continue
		}
		if len(held) < 12 {
			g.Tiles[i].Owner = 7
			held = append(held, i)
		}
	}
	was := make([]float64, len(held))
	for k, i := range held {
		was[k] = g.Height[i]
	}
	g.meander(40)
	for k, i := range held {
		if g.Height[i] != was[k] {
			t.Fatalf("held tile %d went from %v to %v", i, was[k], g.Height[i])
		}
	}
}

// The outside of a bend is the outside. Water coming in from the west and going
// on to the south turns about the south-west corner, so that is where it lays
// its bar and the north-east is the bank it cuts - and neither is the channel
// it came down or the one it goes on in.
func TestTheOutsideOfABendIsNotTheRiver(t *testing.T) {
	cases := []struct{ in, out, inner geom.Pos }{
		{geom.Pos{X: 1}, geom.Pos{Y: 1}, geom.Pos{X: -1, Y: 1}},
		{geom.Pos{X: 1}, geom.Pos{Y: -1}, geom.Pos{X: -1, Y: -1}},
		{geom.Pos{X: 1}, geom.Pos{X: 1, Y: 1}, geom.Pos{X: 0, Y: 1}},
		{geom.Pos{Y: 1}, geom.Pos{X: -1, Y: 1}, geom.Pos{X: -1, Y: 0}},
	}
	for _, c := range cases {
		inner, outer := bend(c.in, c.out)
		up := geom.Pos{X: -c.in.X, Y: -c.in.Y}
		if inner != c.inner || outer != (geom.Pos{X: -c.inner.X, Y: -c.inner.Y}) {
			t.Errorf("in %v out %v: inner %v outer %v, want inner %v", c.in, c.out, inner, outer, c.inner)
		}
		for _, side := range []geom.Pos{inner, outer} {
			if side == up || side == c.out {
				t.Errorf("in %v out %v: a bank at %v is the channel itself", c.in, c.out, side)
			}
		}
	}
}
