package terra

import (
	"math"
	"math/rand/v2"
	"testing"
)

// A pile is read from the top down: the ground at a height lies in the first
// bed whose floor is under it, and the basement goes on for ever.
func TestAPileIsReadFromTheTopDown(t *testing.T) {
	c := basement(Granite, 0, 0)
	c.lay(Shale, 1, 1, 0, 10)
	c.lay(Sandstone, 2, 1, 10, 25)
	for _, want := range []struct {
		h    float64
		rock Bedrock
	}{{30, Sandstone}, {25, Sandstone}, {11, Sandstone}, {10, Shale}, {1, Shale}, {0, Granite}, {-500, Granite}} {
		if got := c.rockAt(want.h); got != want.rock {
			t.Errorf("at %v m the pile is %s, want %s", want.h, got, want.rock)
		}
	}
	// Worn to 8 m, the sandstone is gone and does not come back when the
	// ground is built up again: what lands there later is new rock.
	c.truncate(8)
	if c.n != 2 || c.rockAt(20) != Shale {
		t.Errorf("worn to 8 m the pile is %d beds with %s on top", c.n, c.rockAt(20))
	}
	// Lifted, the beds go up with the ground.
	c.lift(100)
	if c.rockAt(104) != Shale || c.rockAt(99) != Granite {
		t.Errorf("lifted a hundred metres the shale is not where it was put")
	}
}

// A basin sinks under what it is given, so what was laid before is buried and
// not taken off.
func TestABasinBuriesWhatItWasGiven(t *testing.T) {
	c := basement(Basalt, 0, 50)
	c.bury(Shale, 1, 1, 50, 9)
	c.bury(Limestone, 2, 0, 50, 6)
	if c.n != 3 {
		t.Fatalf("two beds buried on a basement made %d beds", c.n)
	}
	if c.rockAt(49) != Limestone || c.rockAt(43) != Shale || c.rockAt(30) != Basalt {
		t.Errorf("the pile is %v over %v", c.rock[:c.n], c.top[:c.n])
	}
}

// A pile that is full folds its thinnest bed into the one under it rather
// than refusing the next.
func TestAFullPileKeepsItsThickestBeds(t *testing.T) {
	c := basement(Granite, 0, 0)
	rocks := []Bedrock{Shale, Sandstone}
	for k := 0; k < 3*bedsMax; k++ {
		thick := 10.0
		if k == 3 {
			thick = 0.5
		}
		c.bury(rocks[k%2], uint8(k), 1, 0, thick)
		if int(c.n) > bedsMax {
			t.Fatalf("the pile holds %d beds", c.n)
		}
	}
	if c.n != bedsMax {
		t.Errorf("a full pile holds %d beds, want %d", c.n, bedsMax)
	}
	if c.rockAt(-1) != rocks[(3*bedsMax-1)%2] {
		t.Errorf("the newest bed is not on top")
	}
}

// Cooking remakes what lies deep and leaves what lies shallow.
func TestACollisionCooksWhatItBuries(t *testing.T) {
	c := basement(Granite, 0, 0)
	c.lay(Shale, 1, 1, 0, 10)
	c.lay(Sandstone, 2, 1, 10, 30)
	c.cook(12, Schist, 3)
	if c.rockAt(20) != Sandstone || c.rockAt(5) != Schist || c.rockAt(-5) != Schist {
		t.Errorf("cooked below 12 m the pile is %v over %v", c.rock[:c.n], c.top[:c.n])
	}
	if c.n != 2 {
		t.Errorf("the cooked beds were not joined: %d beds", c.n)
	}
}

// A pass that hands the ground new heights by rank carries the beds with it,
// so every tile stands in the same bed after it as before.
func TestRestrataKeepsEveryTileInItsBed(t *testing.T) {
	g := NewGrid(24, 24)
	r := rand.New(rand.NewPCG(3, 4))
	g.strata = make([]column, len(g.Tiles))
	from := make([]float64, len(g.Tiles))
	to := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		from[i] = 200 * r.Float64()
		c := basement(Granite, 0, from[i])
		at := from[i] - 60*r.Float64()
		for k := 0; k < 4; k++ {
			c.lay(Bedrock(1+k%3), 0, 1, at, at+15)
			at += 15
		}
		g.strata[i] = c
		// A squeeze of the low ground and a stretch of the high: monotone,
		// and nothing like a straight line.
		to[i] = math.Pow(from[i]/200, 2.5) * 320
	}
	before := make([]Bedrock, len(g.Tiles))
	for i := range g.Tiles {
		before[i] = g.strata[i].rockAt(from[i])
	}
	g.restrata(from, to, nil)
	for i := range g.Tiles {
		// A hair under the ground, since the ground may lie on a bed's top.
		if got := g.strata[i].rockAt(to[i] - 1e-3); got != before[i] && g.strata[i].rockAt(to[i]) != before[i] {
			t.Fatalf("tile %d stood in %s and now stands in %s", i, before[i], got)
		}
	}
}

// A hard cap holds a steeper edge than the soft beds under it, which is the
// shape of a scarp and of the rim of a mesa.
func TestAHardCapHoldsItsEdge(t *testing.T) {
	g := NewGrid(40, 9)
	g.strata = make([]column, len(g.Tiles))
	for i := range g.Tiles {
		x := i % g.W
		// A block standing a hundred metres over a plain, with a wall for
		// an edge: the landslide has everything to bring down.
		h := 0.0
		if x >= 20 {
			h = 100
		}
		g.Tiles[i].Height = h
		c := basement(Shale, 0, h)
		c.lay(Basalt, 0, 0, 50, 1000)
		g.strata[i] = c
		g.Tiles[i].Bedrock = c.rockAt(h)
	}
	g.landslide(false)
	y := g.H / 2
	var capFall, footFall []float64
	for x := 1; x < g.W; x++ {
		hi, lo := g.Tiles[y*g.W+x].Height, g.Tiles[y*g.W+x-1].Height
		fall := (hi - lo) / TileSpan
		if fall <= 0 {
			continue
		}
		// Judged by the rock the upper tile of each pair stands in.
		if hi > 50 {
			capFall = append(capFall, fall)
		} else {
			footFall = append(footFall, fall)
		}
	}
	if len(capFall) == 0 || len(footFall) == 0 {
		t.Fatalf("no cap and foot to compare: %v %v", capFall, footFall)
	}
	if meanOf(capFall) <= meanOf(footFall)*1.15 {
		t.Errorf("the basalt cap stands at %.2f and the shale under it at %.2f", meanOf(capFall), meanOf(footFall))
	}
}

// A river crossing a hard bed stands steeper over it than over the soft
// ground either side, so it drops over the hard bed's edge.
func TestARiverStepsDownOverAHardBed(t *testing.T) {
	strip := func(band Bedrock) *Grid {
		g := NewGrid(64, 5)
		g.strata = make([]column, len(g.Tiles))
		for i := range g.Tiles {
			x := i % g.W
			g.Tiles[i].Height = 4 * float64(x)
			c := basement(Shale, 0, g.Tiles[i].Height)
			c.lay(band, 0, 0, 60, 90)
			c.lay(Shale, 0, 1, 90, 1000)
			g.strata[i] = c
			g.Tiles[i].Bedrock = c.rockAt(g.Tiles[i].Height)
		}
		g.shape()
		return g
	}
	// The same strip with and without a hard band in it, shaped alike.
	layered, plain := strip(Basalt), strip(Shale)
	y := layered.H / 2
	var hard, soft []float64
	for x := 1; x < layered.W; x++ {
		i := y*layered.W + x
		if layered.strata[i].rockAt(layered.Tiles[i].Height) != Basalt {
			continue
		}
		hard = append(hard, layered.Tiles[i].Height-layered.Tiles[i-1].Height)
		soft = append(soft, plain.Tiles[i].Height-plain.Tiles[i-1].Height)
	}
	if len(hard) == 0 {
		t.Fatalf("the shaped river never crosses the basalt")
	}
	if meanOf(hard) <= 1.1*meanOf(soft) {
		t.Errorf("the river falls %.2f m a tile over basalt, and %.2f over the same reach in shale", meanOf(hard), meanOf(soft))
	}
}

// A history leaves its ground layered: most tiles stand on a pile of more
// than one bed, and on a good share of them the weather has cut through to
// a bed that is not the one on top.
func TestAHistoryLeavesItsBedsInLayers(t *testing.T) {
	g := NewLand(1, AncientTerms()).Grid
	beds, layered := 0, 0
	kinds := map[Bedrock]bool{}
	for i := range g.Tiles {
		c := &g.strata[i]
		beds += int(c.n)
		if c.n > 1 {
			layered++
		}
		if g.Tiles[i].Bedrock != c.rock[0] {
			t.Fatalf("tile %d reads %s over a pile whose top bed is %s", i, g.Tiles[i].Bedrock, c.rock[0])
		}
		for k := 0; k < int(c.n); k++ {
			kinds[c.rock[k]] = true
		}
	}
	if share := float64(layered) / float64(len(g.Tiles)); share < 0.5 {
		t.Errorf("only %.0f%% of the map stands on more than one bed", 100*share)
	}
	if mean := float64(beds) / float64(len(g.Tiles)); mean < 2 {
		t.Errorf("a pile is %.2f beds deep on average", mean)
	}
	if len(kinds) < 5 {
		t.Errorf("the piles hold only %d kinds of rock", len(kinds))
	}
}

// A drawn map is layered too, and what a tile is made of depends on how high
// it stands: the high ground is the pile and the low ground the floor.
func TestADrawnMapIsCutIntoItsPile(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3} {
		g := NewLand(seed, DefaultTerms()).Grid
		// How much of the low half and the high half stands on the pile
		// rather than the floor it lies on.
		line := quantile(g.heights(), 0.5)
		var pile, all [2]int
		for i := range g.Tiles {
			c := &g.strata[i]
			half := 0
			if g.Tiles[i].Height >= line {
				half = 1
			}
			all[half]++
			if c.n > 1 {
				pile[half]++
			}
		}
		low, high := float64(pile[0])/float64(all[0]), float64(pile[1])/float64(all[1])
		if high <= 2*low {
			t.Errorf("seed %d: %.0f%% of the high half stands on the pile and %.0f%% of the low half", seed, 100*high, 100*low)
		}
	}
}

// The weather goes on baring new beds after the map is made.
func TestTheWeatherBaresTheBedsBeneath(t *testing.T) {
	w := NewLand(2, AncientTerms())
	before := make([]Bedrock, len(w.Grid.Tiles))
	for i := range w.Grid.Tiles {
		before[i] = w.Grid.Tiles[i].Bedrock
	}
	for age := 0; age < 40; age++ {
		w.Erode()
	}
	changed := 0
	for i := range w.Grid.Tiles {
		if w.Grid.Tiles[i].Bedrock != before[i] {
			changed++
		}
	}
	if changed == 0 {
		t.Errorf("forty ages of weather bared no new rock anywhere")
	}
}

// Tipped beds wear into ridges that run the way the beds strike: the hard beds
// stand proud of the soft ones either side of them, and the ground along a
// hard bed is the same bed and stays level with it. The same dome is made with
// its beds striking north and then east, so that whatever the drainage of the
// dome does to one it does to the other, and only the beds are turned.
func TestHogbacksRunAlongTheStrike(t *testing.T) {
	dome := func(strikeNorth bool) *Grid {
		const side = 64
		g := NewGrid(side, side)
		g.strata = make([]column, len(g.Tiles))
		for i := range g.Tiles {
			x, y := float64(i%side), float64(i/side)
			dx, dy := (x-side/2)/(side/2), (y-side/2)/(side/2)
			h := 20 + 200*math.Max(0, 1-(dx*dx+dy*dy)/2)
			g.Tiles[i].Height = h
			across := x
			if !strikeNorth {
				across = y
			}
			// Basalt and shale by turns, dipping at 0.8 across the strike,
			// laid only where the dome's ground can reach them so that no
			// pile has to fold its beds together.
			c := basement(Shale, 0, h)
			at := -4000 + 0.8*across*TileSpan
			for k := 0; at < 240; k++ {
				rock, thick := Shale, 50.0
				if k%2 == 0 {
					rock, thick = Basalt, 30.0
				}
				if at+thick > 0 {
					c.lay(rock, 0, 0, at, at+thick)
				}
				at += thick
			}
			g.strata[i] = c
			g.Tiles[i].Bedrock = c.rockAt(h)
		}
		g.shape()
		g.denude()
		g.landslide(false)
		g.expose()
		return g
	}
	// How far the hard tiles stand above their neighbours east and west, and
	// north and south.
	proud := func(g *Grid) (ew, ns float64) {
		n := 0
		for y := 1; y < g.H-1; y++ {
			for x := 1; x < g.W-1; x++ {
				i := y*g.W + x
				if g.Tiles[i].Bedrock != Basalt {
					continue
				}
				h := g.Tiles[i].Height
				ew += h - (g.Tiles[i-1].Height+g.Tiles[i+1].Height)/2
				ns += h - (g.Tiles[i-g.W].Height+g.Tiles[i+g.W].Height)/2
				n++
			}
		}
		if n == 0 {
			t.Fatal("no basalt came to the surface")
		}
		return ew / float64(n), ns / float64(n)
	}
	for _, c := range []struct {
		name        string
		strikeNorth bool
	}{{"north", true}, {"east", false}} {
		ew, ns := proud(dome(c.strikeNorth))
		across, along := ew, ns
		if !c.strikeNorth {
			across, along = ns, ew
		}
		if across < 1.5 || across < 2*along {
			t.Errorf("beds striking %s: the hard beds stand %.2f m over the ground across the strike and %.2f m along it",
				c.name, across, along)
		}
	}
}
