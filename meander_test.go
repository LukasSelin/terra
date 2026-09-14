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
		if f := g.Tiles[i].Flow; f > best[g.Index(q)] {
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
func TestRiversWanderAsTheyAge(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3} {
		w := NewLand(seed, DefaultTerms())
		young, _ := bends(w.Grid)
		for k := 0; k < 40; k++ {
			w.Erode()
		}
		old, n := bends(w.Grid)
		if n < 50 {
			t.Fatalf("seed %d has only %d river tiles to read", seed, n)
		}
		if !(old > young) {
			t.Errorf("seed %d: %.3f of the river turned when it was young and %.3f after forty ages",
				seed, young, old)
		}
		if old < 0.5 {
			t.Errorf("seed %d: only %.3f of an old river turns; it is running in straight lines", seed, old)
		}
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
			was[i] = g.Tiles[i].Height
		}
		g.meander(20)
		moved := 0.0
		for i := range g.Tiles {
			moved += math.Abs(g.Tiles[i].Height - was[i])
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
		was[k] = g.Tiles[i].Height
	}
	g.meander(40)
	for k, i := range held {
		if g.Tiles[i].Height != was[k] {
			t.Fatalf("held tile %d went from %v to %v", i, was[k], g.Tiles[i].Height)
		}
	}
}
