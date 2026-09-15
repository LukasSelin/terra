package terra

import (
	"github.com/LukasSelin/terra/geom"
	"math"
	"testing"
	"time"
)

// A valley reads the same weather on every row, to the bit: what a globe
// does by latitude a valley must not do at all.
func TestTheDefaultClimateReadsTheSameAtEveryRow(t *testing.T) {
	w := NewLand(1, DefaultTerms())
	for tick := 0; tick < 400; tick++ {
		w.Climate.Advance(tick, w.RNG)
		for y := 0; y < w.Grid.H; y++ {
			if w.Climate.TempAt(y) != w.Climate.Temp || w.Climate.GrowthAt(y) != w.Climate.Growth() || w.Climate.ChillAt(y) != w.Climate.Chill() {
				t.Fatalf("row %d reads differently from the map on day %d", y, tick)
			}
		}
	}
}

// On a globe the year is warmer toward the middle and colder toward the
// poles, the middle has no winter, and the south's summer is the north's
// winter.
func TestGrowthIsSlowerTowardThePoles(t *testing.T) {
	c := NewClimateOn(GlobeTerms())
	c.Temp = seasonal(0)
	equator, temperate, pole := c.rows/2, c.rows/4, 0
	if !(c.MeanAt(equator) > c.MeanAt(temperate) && c.MeanAt(temperate) > c.MeanAt(pole)) {
		t.Fatalf("means: equator %.1f temperate %.1f pole %.1f", c.MeanAt(equator), c.MeanAt(temperate), c.MeanAt(pole))
	}
	if c.MeanAt(pole) >= Frost {
		t.Fatalf("the pole averages %.1f, above the frost", c.MeanAt(pole))
	}
	// Midsummer in the north is midwinter in the south.
	c.Temp, c.tick = seasonal(Year/4), Year/4
	north, south := c.TempAt(c.rows/4), c.TempAt(3*c.rows/4)
	c.Temp, c.tick = seasonal(3*Year/4), 3*Year/4
	northLater, southLater := c.TempAt(c.rows/4), c.TempAt(3*c.rows/4)
	if !(north > northLater && south < southLater) {
		t.Fatalf("north %.1f then %.1f, south %.1f then %.1f: the seasons do not turn over", north, northLater, south, southLater)
	}
	var lo, hi float64 = 100, -100
	for tick := 0; tick < Year; tick++ {
		c.Temp, c.tick = seasonal(tick), tick
		lo, hi = min(lo, c.TempAt(equator)), max(hi, c.TempAt(equator))
	}
	// The sun crosses the equator twice a year and stands nearly as high all
	// of it: the energy balance's equator swings a degree or two, as the real
	// one's monthly means do (Hartmann, 2016, ch. 2: 1-3 C).
	if hi-lo > 3 {
		t.Fatalf("the equator swings %.1f degrees over the year", hi-lo)
	}
}

// A globe has a sea, its rivers reach it or a pole, and nothing but the
// sea lies on the seam that its ground does not continue across.
func TestAGlobeHasASeaItsRiversReach(t *testing.T) {
	if testing.Short() {
		t.Skip("a globe takes a second or two to make")
	}
	start := time.Now()
	w := NewLand(1, GlobeTerms())
	made := time.Since(start)
	g := w.Grid
	sea := 0
	for i := range g.Tiles {
		if g.underSea(i) {
			sea++
		}
	}
	// How much of it is sea is the plates' to say, since the globe is given
	// water and not a share: see water.go. Over its first three seeds it is
	// between a half and two thirds, which is where the ocean crust puts it,
	// and the crust is held near the share asked for: see crustSlack.
	if share := float64(sea) / float64(len(g.Tiles)); share < 0.4 || share > 0.7 {
		t.Fatalf("the globe's water covers %.2f of it", share)
	}
	// Follow the water down from every watercourse: it ends in the sea, or
	// in a lake with no outlet, and nowhere else. A pole is not an outlet -
	// see Grid.outlet - so a river that ends at one has been left hanging over
	// the top of the map, and a river that ends anywhere else is in a hollow
	// the water should have filled.
	stranded, poleward := 0, 0
	for i := range g.Tiles {
		if !g.Tiles[i].Wet() || g.underSea(i) {
			continue
		}
		p := g.PosOf(i)
		for steps := 0; steps < len(g.Tiles); steps++ {
			q, ok := g.Downstream(p)
			if !ok {
				j := g.Index(p)
				switch {
				case g.underSea(j), g.closedLake(j), g.pans[j]:
				case p.Y == 0 || p.Y == g.H-1:
					poleward++
				default:
					stranded++
				}
				break
			}
			p = q
		}
	}
	if stranded > 0 {
		t.Errorf("%d river tiles drain into nowhere", stranded)
	}
	if poleward > 0 {
		t.Errorf("%d river tiles run off the top or the bottom of the map", poleward)
	}
	// The poles are bare and the middle is not.
	switch g.At(geom.Pos{X: 100, Y: 0}).Terrain {
	case Rock, Water, Ice:
	default:
		t.Fatal("the pole is not bare")
	}
	// A globe is made out of its own history now, which is sixteen epochs of
	// plates, weather and drainage over half a million tiles - see Globe. It
	// was about fourteen seconds on the machine this was written on against
	// about one for a drawn map; what has been added to the making since has
	// brought it to about sixty-seven seconds run alone and seventy-five in
	// the whole suite. The budget is set at about twice that
	// because what it is for is catching something that has gone quadratic,
	// not policing a second either way.
	if made > 150*time.Second {
		t.Fatalf("the globe took %v to make", made)
	}
	t.Logf("a globe of %d tiles, %d sea, %d forest, made in %v", len(g.Tiles), sea, g.Forest(), made)
}

// The mask that says where the high country stands has to have enough
// corners in it to say anything. On a map smaller than a whole range it is
// the half-span it always was; on a globe it is far finer than that, and the
// difference shows up as ground that varies along a row instead of a row that
// is all one height. See Grid.UplandLattice.
func TestTheUplandMaskIsFinerThanTheMap(t *testing.T) {
	valley := &Grid{W: DefaultWidth, H: DefaultHeight}
	if got, want := valley.UplandLattice(), float64(valley.Span())/2; got != want {
		t.Fatalf("the valley's upland lattice is %.3f, want the %.3f it always was", got, want)
	}
	globe := &Grid{W: 1024, H: 512, Wrap: true}
	corners := math.Ceil(float64(globe.W)/globe.UplandLattice()) * (math.Floor(float64(globe.H)/globe.UplandLattice()) + 2)
	if corners < 30 {
		t.Fatalf("a globe draws its topography from %.0f corners, which is a tilt and not a world", corners)
	}
	if testing.Short() {
		t.Skip("a globe takes a second or two to make")
	}
	// Along a row of a globe the ground rises and falls, because the high
	// country is a region of it and not a band across it. The reading is the
	// spread along a row set against the spread between the rows, and not a
	// height, because a height is a statement about how tall this map's
	// mountains happen to be: at a plain half-span the mountains were three
	// thousand metres and any row through one cleared a hundred metres of
	// spread while saying nothing at all about where the high ground was.
	// What is wanted is that a row is not the unit the ground varies in.
	// Drawn and not run: what is being read here is the mask the drawn
	// generator lays its high ground with, and the preset runs a history over
	// that and leaves little of it to read. It also saves the test the
	// fourteen seconds a history costs, for a reading the history would only
	// muddy.
	drawn := GlobeTerms()
	drawn.Epochs = 0
	g := NewLand(1, drawn).Grid
	along := make([]float64, g.H)
	across := make([]float64, g.H)
	for y := 0; y < g.H; y++ {
		var sum, sq float64
		for x := 0; x < g.W; x++ {
			h := g.At(geom.Pos{X: x, Y: y}).Height
			sum, sq = sum+h, sq+h*h
		}
		n := float64(g.W)
		across[y] = sum / n
		along[y] = math.Sqrt(math.Max(0, sq/n-(sum/n)*(sum/n)))
	}
	var sum, sq float64
	for _, m := range across {
		sum, sq = sum+m, sq+m*m
	}
	n := float64(len(across))
	between := math.Sqrt(math.Max(0, sq/n-(sum/n)*(sum/n)))
	within := quantile(along, 0.5)
	if within < between {
		t.Errorf("the ground varies by %.0f metres along the middling row and the rows differ by %.0f: "+
			"the high country is a band of latitude, not a country", within, between)
	}
}

// The sea moderates the ground it lies about, so the ice edge follows a coast
// rather than a parallel. A map with no sea has no water anywhere near it and
// its frostline is untouched. See Maritime.
func TestTheIceEdgeIsNotALineOfLatitude(t *testing.T) {
	c := NewClimateOn(GlobeTerms())
	if warm := maritime(0); warm != 0 {
		t.Fatalf("ground with no sea about it is warmed by %v of it", warm)
	}
	for y := 0; y < c.rows; y += 37 {
		if c.seaMeanAt(y, maritime(0)) != c.MeanAt(y) {
			t.Fatalf("row %d with no sea about it reads a different year", y)
		}
	}
	if testing.Short() {
		t.Skip("a globe takes a second or two to make")
	}
	// The first row down from the north pole that is ground somebody could
	// stand on and grow something, taken column by column. Read off the
	// latitude alone this was row 88 on every seed, and on the worst of them
	// 681 of 1024 columns turned green on that one row: a line ruled across
	// the map, and the same line on every map.
	//
	// Both poles are read, counting rows in from each, since a latitude's
	// weather is the same north and south and a ruled edge would be too.
	edge := func(g *Grid, north bool) (first, most int) {
		rows := map[int]int{}
		first = g.H
		for x := 0; x < g.W; x++ {
			for k := 0; k < g.H; k++ {
				y := k
				if !north {
					y = g.H - 1 - k
				}
				p := geom.Pos{X: x, Y: y}
				if t := g.At(p); !t.Wet() && !g.Frozen(p) {
					rows[k], first = rows[k]+1, min(first, k)
					break
				}
			}
		}
		for _, n := range rows {
			most = max(most, n)
		}
		return first, most
	}
	// Where the green ground begins nearest each pole is wherever that
	// world's coasts happen to put it, so two of the six can fall on the same
	// row by chance: seeds 1 and 3 both begin at row 49 in the north. A fixed
	// latitude puts all six on one row. So no row may be where more than two
	// of them begin.
	seen := map[int]int{}
	for _, seed := range []uint64{1, 2, 3} {
		g := NewLand(seed, GlobeTerms()).Grid
		for _, north := range []bool{true, false} {
			first, most := edge(g, north)
			if most > 300 {
				t.Errorf("seed %d turns green on one row in %d of %d columns: the edge is ruled", seed, most, g.W)
			}
			seen[first]++
		}
	}
	for row, n := range seen {
		if n > 2 {
			t.Errorf("%d of six poles put the end of their green ground on row %d: the ice begins at a fixed latitude", n, row)
		}
	}
}

// The sea freezes where the year never comes up to the point it thaws at, so
// a pole is ice rather than the open, fish-rich ocean it used to be, and a
// river that reaches one is ice too. See SeaFreeze.
func TestThePolarSeaIsIce(t *testing.T) {
	if SeaFreeze >= 0 {
		t.Fatalf("sea water freezes at %.1f degrees", SeaFreeze)
	}
	if testing.Short() {
		t.Skip("a globe takes a second or two to make")
	}
	for _, seed := range []uint64{1, 2, 3} {
		g := NewLand(seed, GlobeTerms()).Grid
		for _, y := range []int{0, g.H - 1} {
			open, ice := 0, 0
			for x := 0; x < g.W; x++ {
				switch g.At(geom.Pos{X: x, Y: y}).Terrain {
				case Water:
					open++
				case Ice:
					ice++
				}
			}
			if open > 0 {
				t.Errorf("seed %d has %d tiles of open water on row %d, where the year averages %.0f degrees",
					seed, open, y, NewClimateOn(GlobeTerms()).MeanAt(y))
			}
			if ice == 0 {
				t.Errorf("seed %d has no ice at all on row %d", seed, y)
			}
		}
		// Nothing frozen holds a fish, and nothing that holds a fish is
		// frozen. The polar ocean used to be the best fishing on the map.
		for i := range g.Tiles {
			if g.Tiles[i].Terrain == Ice && g.Fish[i] > 0 {
				t.Fatalf("seed %d keeps %.2f fish under the ice at %v", seed, g.Fish[i], g.PosOf(i))
			}
		}
		// Ice is crossed on foot, laden and all: it is the one water a
		// walker with a sack does not have to go round.
		frozen := false
		for i := range g.Tiles {
			if g.Tiles[i].Terrain == Ice {
				if g.Tiles[i].Deep() {
					t.Fatalf("seed %d makes a walker swim the ice at %v", seed, g.PosOf(i))
				}
				frozen = true
			}
		}
		if !frozen {
			t.Errorf("seed %d froze nothing anywhere", seed)
		}
	}
}

// A globe has no edge for its water to leave by, so only its sea is an
// outlet - unless it has no sea, in which case its poles are all it has and
// a map with no outlet at all cannot be filled. See Grid.outlet.
func TestOnlyTheSeaDrainsAGlobe(t *testing.T) {
	valley := &Grid{W: 8, H: 4}
	if !valley.outlet(0, 2) || !valley.outlet(3, 0) || valley.outlet(3, 2) {
		t.Fatal("a valley drains at its four edges and nowhere else")
	}
	globe := &Grid{W: 8, H: 4, Wrap: true, sea: 12}
	for y := 0; y < globe.H; y++ {
		for x := 0; x < globe.W; x++ {
			if globe.outlet(x, y) {
				t.Fatalf("a globe with a sea drains off the map at %d,%d", x, y)
			}
		}
	}
	dry := &Grid{W: 8, H: 4, Wrap: true, sea: -1}
	if !dry.outlet(3, 0) || !dry.outlet(3, 3) || dry.outlet(3, 1) {
		t.Fatal("a globe with no sea falls back on its poles")
	}
	if testing.Short() {
		t.Skip("a globe takes a second or two to make")
	}
	// And a globe with no sea is still a map: filled from its poles alone,
	// its ground still has a range and still grows something.
	g := NewLand(1, Terms{Width: 256, Height: 128, Wrap: true}).Grid
	lo, hi := math.Inf(1), math.Inf(-1)
	for i := range g.Tiles {
		lo, hi = math.Min(lo, g.Tiles[i].Height), math.Max(hi, g.Tiles[i].Height)
	}
	if hi-lo < Relief || g.Forest() == 0 {
		t.Errorf("a globe with no sea came out %.0f to %.0f metres with %d forest", lo, hi, g.Forest())
	}
}
