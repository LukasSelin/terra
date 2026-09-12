package terra

import (
	"github.com/LukasSelin/terra/geom"
	"testing"
)

func TestStepTowardPrefersEasierGround(t *testing.T) {
	g := NewGrid(10, 10)
	from, to := geom.Pos{X: 2, Y: 5}, geom.Pos{X: 8, Y: 5}
	g.Turn(geom.Pos{X: 3, Y: 5}, Forest) // straight ahead, slow
	if p := g.StepToward(from, to); p != (geom.Pos{X: 3, Y: 4}) && p != (geom.Pos{X: 3, Y: 6}) {
		t.Fatalf("step = %v, want a detour around the forest", p)
	}
	for y := 0; y < 10; y++ {
		g.Turn(geom.Pos{X: 3, Y: y}, Forest)
	}
	if p := g.StepToward(from, to); p != (geom.Pos{X: 3, Y: 5}) {
		t.Fatalf("step = %v, want the straight line when there is no way round", p)
	}
}

// A road is the fastest ground there is, so a walker leaves the straight line
// to get on one and stays on it as long as it is going their way.
func TestStepTowardTakesTheRoad(t *testing.T) {
	g := NewGrid(20, 9)
	from, to := geom.Pos{X: 1, Y: 4}, geom.Pos{X: 18, Y: 4}
	for x := 0; x < 20; x++ {
		g.Build(geom.Pos{X: x, Y: 2}, paving)
	}
	straight := geom.Pos{X: 2, Y: 4}
	if p := g.StepToward(from, to); p == straight {
		t.Fatalf("step = %v, want a step toward the road at y=2", p)
	}
	onRoad := geom.Pos{X: 5, Y: 2}
	if p := g.StepToward(onRoad, to); p != (geom.Pos{X: 6, Y: 2}) {
		t.Fatalf("step from the road = %v, want to stay on it", p)
	}
}

// Paving is what makes a route cheap: the same walk costs half as much once
// there is a road under it.
func TestRoadsHalveTheCostOfWalking(t *testing.T) {
	g := NewGrid(12, 3)
	from, to := geom.Pos{X: 0, Y: 1}, geom.Pos{X: 8, Y: 1}
	overGrass := g.TravelCost(from, to)
	for x := 0; x < 12; x++ {
		g.Build(geom.Pos{X: x, Y: 1}, paving)
	}
	paved := g.TravelCost(from, to)
	if paved >= overGrass/1.5 {
		t.Fatalf("paved travel cost = %v, grass %v; want the road markedly cheaper", paved, overGrass)
	}
	if d := g.MoveDrain(to); d >= 1 {
		t.Fatalf("road drain = %v, want less than ordinary ground", d)
	}
	if d := g.MoveDrain(geom.Pos{X: 8, Y: 0}); d != 1 {
		t.Fatalf("unpaved drain = %v, want 1", d)
	}
}

// A house is somewhere to live, not a way through. Walking across one costs
// more than walking round it, which is what stops the settlement being a
// shortcut for everybody crossing it.
func TestHousesAreNotThoroughfares(t *testing.T) {
	g := NewGrid(5, 5)
	p := geom.Pos{X: 2, Y: 2}
	open := g.MoveCost(p)
	g.Build(p, blocking)
	if built := g.MoveCost(p); built <= open {
		t.Fatalf("crossing a house costs %v, open ground %v; want the house dearer", built, open)
	}
}

func TestTravelCostRisesWithHardGround(t *testing.T) {
	g := NewGrid(10, 3)
	from, to := geom.Pos{X: 0, Y: 1}, geom.Pos{X: 5, Y: 1}
	open := g.TravelCost(from, to)
	if open != 5 {
		t.Fatalf("open travel cost = %v, want 5", open)
	}
	for y := 0; y < 3; y++ {
		g.Turn(geom.Pos{X: 3, Y: y}, Water)
	}
	if crossed := g.TravelCost(from, to); crossed <= open {
		t.Fatalf("travel cost across the river = %v, want more than %v", crossed, open)
	}
	if c := g.MoveCost(geom.Pos{X: -1, Y: 0}); !isInf(c) {
		t.Fatalf("off-map cost = %v, want infinite", c)
	}
}

func isInf(v float64) bool { return v > 1e308 }

// A river across the map, from bank to bank, with nothing else in the way.
func riverMap() (*Grid, geom.Pos, geom.Pos) {
	g := NewGrid(10, 3)
	for y := 0; y < 3; y++ {
		g.Turn(geom.Pos{X: 3, Y: y}, Water)
	}
	return g, geom.Pos{X: 0, Y: 1}, geom.Pos{X: 5, Y: 1}
}

// Somebody with their hands free wades; somebody with an armful cannot, and
// on a map where the river runs the whole way across there is simply no way
// to the far bank until a bridge is built.
func TestALadenWalkerCannotSwim(t *testing.T) {
	g, from, to := riverMap()
	if c := g.Carrying(0).TravelCost(from, to); isInf(c) {
		t.Fatalf("empty-handed travel cost = %v, want a crossing", c)
	}
	if c := g.Carrying(SwimLoad).TravelCost(from, to); isInf(c) {
		t.Fatalf("travel cost with a crumb = %v, want a crossing", c)
	}
	if c := g.Carrying(1).TravelCost(from, to); !isInf(c) {
		t.Fatalf("laden travel cost = %v, want no way across", c)
	}
	if p := g.Carrying(1).Path(from, to); len(p) != 0 {
		t.Fatalf("laden path = %v, want none", p)
	}
}

// A bridge is a road, and a road is walked on rather than swum, so the load
// that shut the river off is carried straight over it.
func TestALadenWalkerCrossesABridge(t *testing.T) {
	g, from, to := riverMap()
	g.Build(geom.Pos{X: 3, Y: 1}, paving)
	if c := g.Carrying(1).TravelCost(from, to); isInf(c) {
		t.Fatalf("laden travel cost over the bridge = %v, want a crossing", c)
	}
}

// The water is shut to a laden walker as a way through, not as a place to
// work: somebody may wade in from the bank with the timber to bridge it, or
// with a line to fish it.
func TestALadenWalkerMayWadeInToWork(t *testing.T) {
	g, from, _ := riverMap()
	ford := geom.Pos{X: 3, Y: 1}
	if c := g.Carrying(1).TravelCost(from, ford); isInf(c) {
		t.Fatalf("laden travel cost to the ford itself = %v, want a way in", c)
	}
}

// A load is given to one journey and does not outlive it: the next walker to
// use the same router is not carrying the last one's sack.
func TestALoadDoesNotOutliveItsJourney(t *testing.T) {
	g, from, to := riverMap()
	if c := g.Carrying(1).TravelCost(from, to); !isInf(c) {
		t.Fatalf("laden travel cost = %v, want no way across", c)
	}
	if c := g.TravelCost(from, to); isInf(c) {
		t.Fatalf("empty-handed travel cost after a laden one = %v, want a crossing", c)
	}
}

// What is forbidden is walking into the water, not being in it. A river that
// rises under somebody, or a bridge that goes from under them, must leave
// them a way out with what they are holding.
func TestALadenWalkerInTheWaterCanGetOut(t *testing.T) {
	g := NewGrid(10, 3)
	for y := 0; y < 3; y++ {
		for _, x := range []int{3, 4} {
			g.Turn(geom.Pos{X: x, Y: y}, Water)
		}
	}
	midstream, bank := geom.Pos{X: 3, Y: 1}, geom.Pos{X: 5, Y: 1}
	if c := g.Carrying(1).TravelCost(midstream, bank); isInf(c) {
		t.Fatalf("laden travel cost out of the river = %v, want a way out", c)
	}
}
