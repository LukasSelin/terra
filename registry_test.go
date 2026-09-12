package terra

import "testing"

// A growth registered is a growth the day's pass knows about. The pass keeps
// tables of its own, laid out the way it wants them and read off the growth
// table whenever that is set - which has to happen when it is set, because
// nothing about the order a package's variables and inits are made in can be
// relied on to put a game's registering before the land's reading. See
// SetGrowth and readGrowth.
func TestRegisteringAGrowthReachesTheDaysPass(t *testing.T) {
	// Whatever the tests around this have registered, put back afterwards.
	was := growth[blocking][Grass]
	defer SetGrowth(blocking, Grass, was)

	stock := func(g *Grid) []float64 { return g.Wild }
	SetGrowth(blocking, Grass, []Growth{{Full: 100, Rate: 0.01, Stock: stock}})

	if !alive[blocking][Grass] {
		t.Error("ground something was said to grow on carries nothing")
	}
	if ripe[blocking][Grass] != 100 {
		t.Errorf("it comes on in %v, not 100", ripe[blocking][Grass])
	}
	k := kindOf(&Tile{Mark: blocking, Terrain: Grass})
	if !ages[k] {
		t.Error("the pass does not age it")
	}
	var found bool
	for _, e := range stocked {
		if e.kind == k && e.full == 100 && e.rate == 0.01 {
			found = true
		}
	}
	if !found {
		t.Error("the pass fills no stock for it")
	}

	// And taking it away again takes it out of the pass.
	SetGrowth(blocking, Grass, nil)
	if alive[blocking][Grass] || ages[k] {
		t.Error("ground nothing grows on still ages")
	}
	for _, e := range stocked {
		if e.kind == k {
			t.Error("the pass still fills a stock nobody grows")
		}
	}
}
