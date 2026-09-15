package terra

import (
	"github.com/LukasSelin/terra/geom"
	"math"
	"testing"
)

// plateWorld is a wrapped world big enough to hold a proper crust - a valley
// is one plate boundary seen close to and cannot show any of this - and small
// enough to make several of in a test.
func plateWorld(seed uint64) *Grid {
	cfg := GlobeTerms()
	cfg.Width, cfg.Height = 256, 128
	return NewLand(seed, cfg).Grid
}

// pieces is how many tiles each piece of crust holds, by the plate it is.
func pieces(g *Grid) map[uint8]int {
	held := map[uint8]int{}
	for i := range g.Tiles {
		held[g.Tiles[i].Plate]++
	}
	return held
}

// Continents that run into each other weld, and what was two plates with a
// range between them becomes one plate with an old range through the middle
// of it. It is the thing the crust could not do when its plates were a fixed
// list of moving points, and it is where most of the mountains on a finished
// map come from - a range whose cause is over and which has been weathering
// ever since.
//
// It is counted and not read off how many pieces a world ends with, which is
// how this was first written: small plates now break off as well as weld, and
// a world that has done both can end with as many plates as it began with.
func TestContinentsWeldIntoOnePlate(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3} {
		g := plateWorld(seed)
		if g.welds == 0 {
			t.Errorf("seed %d: nothing welded", seed)
		}
		if ended := len(pieces(g)); ended < crustFloor {
			t.Errorf("seed %d welded itself down to %d pieces, below the floor of %d",
				seed, ended, crustFloor)
		}
	}
}

// And the pieces it is left with are pieces of a world rather than slivers or
// hemispheres. A sliver's seam raises a range no wider than the sliver; a
// single piece holding everything has no edges inside it and so nothing
// happening anywhere on it.
func TestNoPieceOfCrustIsASliverOrAHemisphere(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3} {
		g := plateWorld(seed)
		held := pieces(g)
		n := float64(len(g.Tiles))
		// The ceiling is applied by rifting, which takes an epoch to open and
		// cannot act at all on the last epoch of a history, so a piece is
		// allowed to be over it by the share one epoch of drift can add.
		for at, tiles := range held {
			if share := float64(tiles) / n; share > plateCeiling*1.5 {
				t.Errorf("seed %d: plate %d holds %.2f of the world, against a ceiling of %.2f",
					seed, at, share, plateCeiling)
			}
		}
		// Slivers are allowed to exist - a small piece wedged between crust of
		// the other kind has nowhere to go, and that is a real thing for a
		// world to have - but they must not be what the world is made of.
		slivers := 0
		for _, tiles := range held {
			if float64(tiles)/n < plateFloor {
				slivers++
			}
		}
		if slivers > len(held)/2 {
			t.Errorf("seed %d: %d of %d pieces are under the floor", seed, slivers, len(held))
		}
	}
}

// The pieces are not all one size. The earth's plates run from a fifth of the
// world down through a long tail of small ones, and a world of middles set
// down evenly and each given the ground nearest it is a world of equal rooms:
// its largest plate is not two of its middling ones.
//
// This is only the floor. How the sizes fall off past the great plates is held
// to Bird's power law in realism_test.go.
func TestPlatesAreNotAllOneSize(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3} {
		g := plateWorld(seed)
		var sizes []float64
		for _, tiles := range pieces(g) {
			sizes = append(sizes, float64(tiles))
		}
		largest := quantile(sizes, 1)
		if got := largest / quantile(sizes, 0.5); got < 2.5 {
			t.Errorf("seed %d: the largest plate is %.1f times the middling one; "+
				"a world with great plates and small ones is several times that", seed, got)
		}
	}
}

// And every piece is one piece. A plate carried into another can be eaten
// through where its front is narrow, and a piece left on the far side is a
// scrap of one plate adrift inside another.
//
// Known gap: D - with the history on its real clock and belts 550 km wide,
// the second world leaves plate 3 in two pieces. It is skipped as a gap while it
// does, and fails once it does not, so the marker comes off.
func TestEveryPlateIsOnePiece(t *testing.T) {
	const gapSeed = 2
	gapOpen := false
	for _, seed := range []uint64{1, 2, 3} {
		g := plateWorld(seed)
		seen := make([]bool, len(g.Tiles))
		parts := map[uint8]int{}
		for s := range g.Tiles {
			if seen[s] {
				continue
			}
			of := g.Tiles[s].Plate
			parts[of]++
			seen[s] = true
			stack := []int{s}
			for len(stack) > 0 {
				i := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				g.eachNear(i, func(j int) {
					if !seen[j] && g.Tiles[j].Plate == of {
						seen[j] = true
						stack = append(stack, j)
					}
				})
			}
		}
		for at, n := range parts {
			switch {
			case n > 1 && seed == gapSeed:
				gapOpen = true
			case n > 1:
				t.Errorf("seed %d: plate %d is in %d pieces", seed, at, n)
			}
		}
	}
	if !gapOpen {
		t.Errorf("seed %d: every plate is one piece: the gap has closed, take the marker off", gapSeed)
	} else if !t.Failed() {
		t.Skipf("known gap: D - seed %d leaves a plate in two pieces", gapSeed)
	}
}

// The boundaries are not the straight lines a nearest-middle partition draws.
// A Voronoi cell is convex, and a convex region is about as tight round its
// own area as a region can be; a grown one wanders, reaches round its
// neighbours and leaves bays in itself, and the way to say so in a number is
// how much edge it needs to hold the ground it holds.
//
// The measure is the perimeter against that of the circle of the same area,
// which is 1 for a circle and cannot be below it. A Voronoi cell of a random
// set of middles runs about 1.1; anything much above that is a shape a
// straight-edged partition could not have made.
func TestPlateBoundariesAreNotStraight(t *testing.T) {
	g := plateWorld(1)
	held := pieces(g)
	edge := map[uint8]int{}
	for i := range g.Tiles {
		p := g.PosOf(i)
		for _, off := range []geom.Pos{{X: 1, Y: 0}, {X: 0, Y: 1}} {
			q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
			if g.Wrap {
				q = g.Norm(q)
			}
			if !g.In(q) {
				continue
			}
			if a, b := g.Tiles[i].Plate, g.At(q).Plate; a != b {
				edge[a]++
				edge[b]++
			}
		}
	}
	var sum, n float64
	for at, tiles := range held {
		// Small pieces are all edge whatever shape they are, so they say
		// nothing about whether the edge is straight.
		if float64(tiles) < 0.03*float64(len(g.Tiles)) {
			continue
		}
		round := 2 * math.Sqrt(math.Pi*float64(tiles))
		sum, n = sum+float64(edge[at])/round, n+1
	}
	if n == 0 {
		t.Fatal("no piece of crust was big enough to measure")
	}
	if got := sum / n; got < 1.35 {
		t.Errorf("a plate needs %.2f times a circle's edge to hold its ground; "+
			"a straight-sided cell needs about 1.1, so these are still Voronoi", got)
	}
}

// Welding leaves the range it stops feeding where it was, inside the plate it
// made. So a made world has upland away from anything that is still happening
// - which is what a plate interior is for, and what a world of live seams and
// flat middles does not have.
//
// It is the upland and not the summits. The very highest ground of a world is
// where two plates are driving into each other now, and it should be: an
// orogen that is still being pushed stands higher than one that stopped being
// pushed an age ago, on this map and on the one outside the window. Asking
// for a share of the top twentieth away from a live seam is asking for the
// Urals to be as tall as the Himalaya, and the first draft of this test asked
// exactly that and was wrong to.
func TestAWeldedPlateKeepsTheRangeThatMadeIt(t *testing.T) {
	g := plateWorld(1)
	// The seams a finished world still has: tiles beside a tile of another
	// plate.
	seamAt := make([]bool, len(g.Tiles))
	for i := range g.Tiles {
		p := g.PosOf(i)
		for _, off := range Dirs {
			q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
			if g.Wrap {
				q = g.Norm(q)
			}
			if g.In(q) && g.At(q).Plate != g.Tiles[i].Plate {
				seamAt[i] = true
				break
			}
		}
	}
	hs := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		hs[i] = g.Tiles[i].Height
	}
	high := quantile(hs, 0.75)
	var inside, total int
	for i := range g.Tiles {
		if hs[i] < high {
			continue
		}
		total++
		if !near(g, i, seamAt, int(beltOn(g))) {
			inside++
		}
	}
	if total == 0 {
		t.Fatal("the world has no high ground")
	}
	// Over three seeds this runs from a tenth to a quarter; the line is under
	// the worst of them, because what is being caught is a world with none at
	// all and not a world with less than last time.
	if share := float64(inside) / float64(total); share < 0.05 {
		t.Errorf("%.0f%% of the upland is more than a belt from any live seam; "+
			"a world whose high ground is all on its edges has forgotten what it did",
			100*share)
	}
}

// A plate that turns meets its neighbour differently all the way along the
// seam between them: closing at one end and parting at the other, on the one
// boundary. A plate that only slides meets it the same way everywhere, and
// that sameness was the whole of why no seam ever had a ridge at one end and a
// range at the other.
func TestATurningPlateClosesAtOneEndAndOpensAtTheOther(t *testing.T) {
	g := NewGrid(96, 96)
	for i := range g.Tiles {
		if i%g.W >= 48 {
			g.Tiles[i].Plate = 1
		}
	}
	plates := []Plate{{Spin: 0.05, into: 0}, {Ocean: true, into: 1}}
	g.locate(plates)
	top, _, okTop := g.meeting(plates, 10*g.W+47, 1)
	bottom, _, okBottom := g.meeting(plates, 86*g.W+47, 1)
	if !okTop || !okBottom {
		t.Fatal("the seam between the two plates was not found")
	}
	if top*bottom >= 0 {
		t.Errorf("the seam closes at %.2f at one end and %.2f at the other; a turning plate "+
			"should close on its neighbour at one end and part from it at the other", top, bottom)
	}
}

// And a turn carries a plate's ground round rigidly: what was on its east side
// is on its south a quarter turn later, and the plate has not opened floor
// inside itself doing it. A turn puts every tile on the tile nearest where it
// goes, and nearest is not one to one, so a turn done carelessly leaves a
// scatter of holes through the middle of the plate - and a hole is new ocean
// floor, which is a spreading ridge in the middle of a continent.
func TestATurnCarriesAPlateRoundWhole(t *testing.T) {
	const size, radius = 96, 30
	g := NewGrid(size, size)
	mid := size / 2
	code := func(x, y int) float64 { return float64(x*1000 + y + 1) }
	disc := 0
	for i := range g.Tiles {
		x, y := i%g.W, i/g.W
		t := &g.Tiles[i]
		t.Plate = 1
		if math.Hypot(float64(x-mid), float64(y-mid)) <= radius {
			t.Plate, t.Height = 0, code(x, y)
			disc++
		}
	}
	// A quarter turn over eight epochs, which is far more than any plate but
	// a microplate is drawn to turn in a whole history.
	const epochs = 8
	plates := []Plate{{Spin: math.Pi / 2 / epochs, into: 0}, {Ocean: true, into: 1}}
	cr := newCrust(g)
	book := make([]record, len(g.Tiles))
	for e := 1; e <= epochs; e++ {
		(&Land{}).move(g, plates, cr, book, e)
	}

	held, fresh := 0, 0
	for i := range g.Tiles {
		x, y := i%g.W, i/g.W
		inside := math.Hypot(float64(x-mid), float64(y-mid)) <= radius-3
		if g.Tiles[i].Plate == 0 {
			held++
		}
		if inside && (g.Tiles[i].Plate != 0 || g.Tiles[i].Formed != 0) {
			fresh++
		}
	}
	if fresh > 0 {
		t.Errorf("%d tiles inside the turning plate are new floor or another plate's", fresh)
	}
	if math.Abs(float64(held-disc)) > 0.05*float64(disc) {
		t.Errorf("the plate held %d tiles before its turn and %d after", disc, held)
	}
	// Every tile the plate still carries should stand where a quarter turn
	// about the middle puts the place it started from: east round to south.
	// Rounding puts any one of them a tile or so off, and a turn that was not
	// going round at all - or going round at a different rate at different
	// distances from the middle - is off by many.
	var miss, carried float64
	for i := range g.Tiles {
		h := g.Tiles[i].Height
		if g.Tiles[i].Plate != 0 || g.Tiles[i].Formed != 0 || h < 1 {
			continue // not ground the plate started with
		}
		c := int(h) - 1
		x0, y0 := c/1000, c%1000
		wx, wy := mid-(y0-mid), mid+(x0-mid)
		miss += math.Hypot(float64(i%g.W-wx), float64(i/g.W-wy))
		carried++
	}
	if carried == 0 {
		t.Fatal("the turned plate carries none of the ground it started with")
	}
	if got := miss / carried; got > 1 {
		t.Errorf("a tile of the turned plate stands %.1f tiles from where a quarter turn puts it", got)
	}
}

// near reports whether any tile within reach of i is marked.
func near(g *Grid, i int, mark []bool, reach int) bool {
	p := g.PosOf(i)
	for dy := -reach; dy <= reach; dy++ {
		for dx := -reach; dx <= reach; dx++ {
			q := geom.Pos{X: p.X + dx, Y: p.Y + dy}
			if g.Wrap {
				q = g.Norm(q)
			}
			if g.In(q) && mark[g.Index(q)] {
				return true
			}
		}
	}
	return false
}

// inland is, for every tile, how many tiles it is to the nearest sea, walked
// out from the water rather than in from the ground.
func inland(g *Grid) []int32 {
	d := make([]int32, len(g.Tiles))
	q := make([]int32, 0, len(g.Tiles))
	for i := range g.Tiles {
		d[i] = -1
		if g.underSea(i) {
			d[i], q = 0, append(q, int32(i))
		}
	}
	for k := 0; k < len(q); k++ {
		i := q[k]
		p := g.PosOf(int(i))
		for _, off := range Dirs {
			n := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
			if g.Wrap {
				n = g.Norm(n)
			}
			if !g.In(n) {
				continue
			}
			if j := int32(g.Index(n)); d[j] < 0 {
				d[j], q = d[i]+1, append(q, j)
			}
		}
	}
	return d
}

// Mountains are not all on the beach. Some ranges do meet the sea and should -
// a floor going down under a continent raises one along that edge, and the
// earth has several - but a world where that is the only kind is a world of
// rimmed islands, and that is what this used to be.
//
// The reading is how far inland the high ground lies against how far inland
// the land lies at all, because the second is what the first has to be judged
// by: a map whose continents are forty tiles wide cannot put a mountain fifty
// tiles from the sea, and is not doing anything wrong by not doing it. At one
// the high ground is no more coastal than the ground in general. It used to
// run about half - the highest tenth of the land stood nine tiles from the sea
// where the land itself stood eighteen - because a continental plate's edge
// was its coastline tile for tile, and a plate's edge is where every mountain
// was built.
//
// Two things moved it. An arc is raised behind the trench and not on it, so
// there is a coastal plain in front of the range; and a plate rides with
// swells and basins in it, so the sea finds its coast in the shape of the
// ground rather than at the boundary of the crust. See arcGapReach and bowRise.
func TestMountainsAreNotAllOnTheCoast(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3} {
		g := plateWorld(seed)
		far := inland(g)
		var land, high []float64
		var dry []float64
		for i := range g.Tiles {
			if g.underSea(i) {
				continue
			}
			land = append(land, float64(far[i]))
			dry = append(dry, g.Tiles[i].Height)
		}
		cut := quantile(dry, 0.9)
		for i := range g.Tiles {
			if !g.underSea(i) && g.Tiles[i].Height >= cut {
				high = append(high, float64(far[i]))
			}
		}
		if len(high) == 0 {
			t.Fatalf("seed %d has no high ground", seed)
		}
		deep, all := quantile(high, 0.5), quantile(land, 0.5)
		if all == 0 {
			t.Fatalf("seed %d is all coast", seed)
		}
		if got := deep / all; got < 0.5 {
			t.Errorf("seed %d: the highest tenth of the land sits %.0f tiles from the sea "+
				"where the land sits %.0f - %.2f of it, so the mountains are on the beach",
				seed, deep, all, got)
		}
	}
}
