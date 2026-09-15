package terra

import (
	"math"
	"testing"
)

// chain is a river a tile wide: n tiles, each draining into the one before,
// the first a root, with flow gathering downstream as a river's does.
func chain(n int, k, by float64) (fluvial, []float64) {
	c := fluvial{
		h:      make([]float64, n),
		recv:   make([]int32, n),
		f:      make([]float64, n),
		settle: make([][Grains]float64, n),
		parts:  make([][Grains]float64, n),
	}
	q := make([]float64, n)
	for i := 0; i < n; i++ {
		c.recv[i] = int32(max(0, i-1))
		q[i] = 0.05 * float64(n-i)
		if i > 0 {
			c.f[i] = by * k * math.Sqrt(q[i]) / TileSpan
		}
		c.h[i] = 10 * float64(i)
		c.parts[i] = [Grains]float64{Sand: 0.4, Silt: 0.4, Clay: 0.2}
	}
	c.stack = stackOf(c.recv)
	return c, q
}

// With nothing settling the solver is Braun and Willett's, to the last bit
// that arithmetic allows: each tile's new height is its old one pulled toward
// its receiver's new height by F.
func TestNothingSettlingIsBraunAndWillett(t *testing.T) {
	c, _ := chain(50, Erodibility, 3)
	got := c.solve(settleIters)
	want := make([]float64, len(c.h))
	want[0] = c.h[0]
	for i := 1; i < len(c.h); i++ {
		want[i] = (c.h[i] + c.f[i]*want[i-1]) / (1 + c.f[i])
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-12*math.Max(1, want[i]) {
			t.Fatalf("tile %d: solved %v, Braun and Willett %v", i, got[i], want[i])
		}
	}
}

// Raised steadily and cut steadily, a river settles on the slope where the two
// balance: S = U/(K·√Q). The implicit step gets there at any size of step, and
// a step a thousand times too long for an explicit one is no different.
//
// K is the test's own and not Erodibility. This is a question about the
// solver, and how many steps a river takes to get to its slope goes as one
// over K: at the map's figure, set by what real ground wears at, four
// thousand steps of one age leave it still on the way.
func TestAnUpliftedRiverReachesItsSteadySlope(t *testing.T) {
	const uplift = 0.5 // metres a step
	const k = 2.0
	for _, by := range []float64{1, 2000} {
		c, q := chain(200, k, by)
		for step := 0; step < 4000; step++ {
			for i := 1; i < len(c.h); i++ {
				c.h[i] += uplift
			}
			next := c.solve(1)
			copy(c.h, next)
		}
		for i := 1; i < len(c.h); i++ {
			slope := (c.h[i] - c.h[i-1]) / TileSpan
			want := uplift / (by * k * math.Sqrt(q[i]))
			if math.IsNaN(slope) || math.Abs(slope-want) > 1e-3*want {
				t.Fatalf("by %v, tile %d: slope %v, want %v", by, i, slope, want)
			}
			if c.h[i] <= c.h[i-1] {
				t.Fatalf("by %v, tile %d stands at %v below its receiver's %v", by, i, c.h[i], c.h[i-1])
			}
		}
	}
}

// However long the step, a river is never cut below the water it runs into.
func TestAnAgeCannotCutBelowTheOutlet(t *testing.T) {
	c, _ := chain(100, Erodibility, 1e6)
	for i := range c.settle {
		c.settle[i] = [Grains]float64{Sand: 0.62, Silt: 0.33, Clay: 0.1}
	}
	next := c.solve(settleIters)
	for i := 1; i < len(next); i++ {
		if next[i] < next[0] || next[i] < next[i-1]-1e-9 || math.IsNaN(next[i]) {
			t.Fatalf("tile %d came out at %v, its receiver at %v and the outlet at %v", i, next[i], next[i-1], next[0])
		}
	}
}

// Every grain the water takes is somewhere at the end of the age: laid down on
// a tile, or gone to the sea. Nothing is made and nothing lost.
func TestTheWaterLosesNoGrain(t *testing.T) {
	c, _ := chain(120, Erodibility, 5)
	for i := range c.settle {
		s := clamp01(1 - float64(i)/40) // gentle at the foot, steep above
		c.settle[i] = [Grains]float64{Sand: 0.62 * s, Silt: 0.33 * s, Clay: 0.1 * s}
		if i < 40 {
			c.h[i] = 0.01 * float64(i) // a flood plain
		}
	}
	next := c.solve(settleIters)
	change := make([]float64, len(c.h))
	gained := make([][Grains]float64, len(c.h))
	exported := c.account(next, change, gained, nil)
	var taken, laid [Grains]float64
	for i := range c.h {
		r := c.recv[i]
		if int(r) == i {
			continue
		}
		cut := c.f[i] * math.Max(0, next[i]-next[r])
		for gr := range taken {
			taken[gr] += cut * c.parts[i][gr]
			laid[gr] += gained[i][gr]
		}
	}
	for gr := range taken {
		if taken[gr] <= 0 {
			t.Fatalf("grain %d: nothing was taken", gr)
		}
		if math.Abs(laid[gr]+exported[gr]-taken[gr]) > 1e-9*taken[gr] {
			t.Errorf("grain %d: %v taken, %v laid and %v gone to the sea", gr, taken[gr], laid[gr], exported[gr])
		}
	}
	sum := 0.0
	for i := range change {
		sum += change[i]
	}
	if all := exported[Sand] + exported[Silt] + exported[Clay]; math.Abs(sum+all) > 1e-9*(taken[Sand]+taken[Silt]+taken[Clay]) {
		t.Errorf("the ground changed by %v and %v went to the sea", sum, all)
	}
}

// Where a steep reach meets the flat, the sand comes out first and the clay
// last: the sand is laid nearer the foot of the slope than the silt, and the
// silt nearer than the clay.
func TestSandSettlesFirst(t *testing.T) {
	c, _ := chain(120, Erodibility, 5)
	for i := range c.h {
		if i < 60 {
			c.h[i] = 0.01 * float64(i)
			c.settle[i] = [Grains]float64{Sand: 0.62, Silt: 0.33, Clay: 0.1}
			c.f[i] = 0
		}
	}
	next := c.solve(settleIters)
	change := make([]float64, len(c.h))
	gained := make([][Grains]float64, len(c.h))
	c.account(next, change, gained, nil)
	var reach [Grains]float64
	for gr := range reach {
		var sum, w float64
		for i := 0; i < 60; i++ {
			sum += gained[i][gr] * float64(60-i)
			w += gained[i][gr]
		}
		reach[gr] = sum / w
	}
	if !(reach[Sand] < reach[Silt] && reach[Silt] < reach[Clay]) {
		t.Errorf("mean distance from the foot of the slope: sand %.1f, silt %.1f, clay %.1f tiles",
			reach[Sand], reach[Silt], reach[Clay])
	}
}

// The ground as a whole loses what went to the sea and nothing else: the
// water, the creep and the banks move it about, but none of them make it.
func TestWearingAMapConservesTheGround(t *testing.T) {
	w := NewLandSized(6, 40, 30)
	g := w.Grid
	for i := range g.Tiles {
		g.Tiles[i].Height += 20 // well clear of the floor at nothing
	}
	g.drain()
	before := 0.0
	for i := range g.Tiles {
		before += g.Tiles[i].Height
	}
	g.wear(5 * ageYears)
	after := 0.0
	for i := range g.Tiles {
		after += g.Tiles[i].Height
	}
	gone := g.exported[Sand] + g.exported[Silt] + g.exported[Clay]
	if gone <= 0 {
		t.Fatal("nothing left the map in five ages")
	}
	if math.Abs(before-after-gone) > 1e-6*gone {
		t.Errorf("the ground went from %.3f to %.3f, and %.3f went off the map", before, after, gone)
	}
}

// An estuary's river is not cut below the tide's high water, however long the
// age: a river runs into a sea that stands at high water half of every day.
func TestAnEstuaryIsNotCutBelowHighWater(t *testing.T) {
	c, _ := chain(80, Erodibility, 1e4)
	c.floor = make([]float64, len(c.h))
	for i := range c.floor {
		c.floor[i] = math.Inf(-1)
		if i < 20 {
			c.floor[i] = 15 // high water, up the first twenty tiles
		}
	}
	next := c.solve(settleIters)
	for i := 1; i < len(next); i++ {
		if c.h[i] > 15 && next[i] < 15-1e-9 {
			t.Fatalf("tile %d stood at %.2f and was cut to %.2f, under the high water at 15", i, c.h[i], next[i])
		}
		if c.h[i] <= 15 && i < 21 && next[i] < c.h[i]-1e-9 {
			t.Fatalf("tile %d, already under high water, was cut from %.2f to %.2f", i, c.h[i], next[i])
		}
	}
}

// A flat under mean sea keeps what the water brings it until it has no room
// left, and the rest goes to the sea; nothing is lost either way.
func TestAFlatKeepsWhatItHasRoomFor(t *testing.T) {
	c, _ := chain(60, Erodibility, 5)
	c.keep, c.room = make([]float64, len(c.h)), make([]float64, len(c.h))
	c.keep[0], c.room[0] = 0.8, 0.05
	next := c.solve(settleIters)
	change := make([]float64, len(c.h))
	gained := make([][Grains]float64, len(c.h))
	exported := c.account(next, change, gained, nil)
	var taken, laid float64
	for i := 1; i < len(c.h); i++ {
		cut := c.f[i] * math.Max(0, next[i]-next[c.recv[i]])
		taken += cut
	}
	for i := range gained {
		laid += gained[i][Sand] + gained[i][Silt] + gained[i][Clay]
	}
	gone := exported[Sand] + exported[Silt] + exported[Clay]
	if kept := gained[0][Sand] + gained[0][Silt] + gained[0][Clay]; kept > c.room[0]+1e-12 || kept <= 0 {
		t.Errorf("the flat kept %.4f m with room for %.4f", kept, c.room[0])
	}
	if math.Abs(laid+gone-taken) > 1e-9*taken {
		t.Errorf("%.6f taken, %.6f laid and %.6f gone to sea", taken, laid, gone)
	}
}

// On a globe, with the tide at work, the ground as a whole still loses only
// what went to the sea, and the same seed weathers the same way.
func TestATidalCoastConservesTheGround(t *testing.T) {
	run := func() ([]float64, float64, float64) {
		w := NewLand(2, smallGlobe())
		g := w.Grid
		before := 0.0
		for i := range g.Tiles {
			before += g.Tiles[i].Height
		}
		g.wear(3 * ageYears)
		after := 0.0
		for i := range g.Tiles {
			after += g.Tiles[i].Height
		}
		gone := g.exported[Sand] + g.exported[Silt] + g.exported[Clay]
		return heights(g), before - after, gone
	}
	a, lost, gone := run()
	if gone <= 0 {
		t.Fatal("nothing went to the sea")
	}
	if math.Abs(lost-gone) > 1e-6*math.Max(gone, 1) {
		t.Errorf("the ground lost %.4f and %.4f went to the sea", lost, gone)
	}
	b, _, _ := run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("tile %d weathered to %v one time and %v the next", i, a[i], b[i])
		}
	}
}

// How far a grain gets before it settles is a fact about the river, not about
// how big a tile is. A load let go at the head of a reach of two hundred metres
// reaches the foot in the same share whether the reach is eight tiles of
// twenty-five metres or sixty-four of three: exp(-Vs·L·W/Q), grain by grain,
// through the solver and the books both.
func TestHowFarAGrainGetsDoesNotDependOnTheTile(t *testing.T) {
	const reach, width, q = 200.0, 10.0, 50.0
	through := func(span float64) [Grains]float64 {
		n := int(reach/span) + 2
		c := fluvial{
			h:      make([]float64, n),
			recv:   make([]int32, n),
			f:      make([]float64, n),
			settle: make([][Grains]float64, n),
			parts:  make([][Grains]float64, n),
		}
		for i := 0; i < n; i++ {
			c.recv[i] = int32(max(0, i-1))
			c.parts[i] = [Grains]float64{Sand: 0.4, Silt: 0.4, Clay: 0.2}
			if i > 0 && i < n-1 {
				for gr := range fallSpeed {
					c.settle[i][gr] = settleShare(fallSpeed[gr], span, width, q)
				}
			}
		}
		// Only the head is cut, and it lets nothing settle where it is cut.
		c.h[n-1], c.f[n-1] = 10, 1
		c.stack = stackOf(c.recv)
		next := c.solve(settleIters)
		change := make([]float64, n)
		gained := make([][Grains]float64, n)
		exported := c.account(next, change, gained, nil)
		cut := c.f[n-1] * (next[n-1] - next[n-2])
		var share [Grains]float64
		for gr := range share {
			share[gr] = exported[gr] / (cut * c.parts[n-1][gr])
		}
		return share
	}
	coarse, fine := through(25), through(3.125)
	for gr := range coarse {
		want := math.Exp(-fallSpeed[gr] * reach * width / q)
		if math.Abs(coarse[gr]-want) > 1e-9 || math.Abs(fine[gr]-want) > 1e-9 {
			t.Errorf("grain %d: %.6f through tiles of 25 m, %.6f through tiles of 3 m, want %.6f", gr, coarse[gr], fine[gr], want)
		}
	}
}

// A hard band in a soft country is an escarpment. Raised steadily and cut
// steadily, a river falls where it crosses the band as steeply as it must to
// cut granite as fast as the ground rises - S = U/(K_b·√Q), and K_b for granite
// is a twelfth of shale's - and eases again below and above it: the band holds
// a knickpoint for as long as the river runs, and the break in the fall is at
// the rock and nowhere else.
func TestAHardBandHoldsAnEscarpment(t *testing.T) {
	const uplift, k, by = 0.5, 2.0, 50.0
	c, q := chain(150, k, by)
	shale, granite := &Tile{Bedrock: Shale}, &Tile{Bedrock: Granite}
	band := func(i int) bool { return i >= 60 && i < 90 }
	for i := 1; i < len(c.h); i++ {
		rock := shale
		if band(i) {
			rock = granite
		}
		c.f[i] = by * k * rockErodibility(rock) * math.Sqrt(q[i]) / TileSpan
	}
	for step := 0; step < 3000; step++ {
		for i := 1; i < len(c.h); i++ {
			c.h[i] += uplift
		}
		copy(c.h, c.solve(1))
	}
	slope := func(i int) float64 { return (c.h[i] - c.h[i-1]) / TileSpan }
	mean := func(from, to int) float64 {
		s := 0.0
		for i := from; i < to; i++ {
			s += slope(i)
		}
		return s / float64(to-from)
	}
	below, on, above := mean(50, 60), mean(60, 90), mean(90, 100)
	ratio := rockErodibility(shale) / rockErodibility(granite)
	if on < 0.8*ratio*math.Max(below, above) {
		t.Errorf("the river falls %.4f across the granite and %.4f and %.4f on the shale either side; want about %.1f times as steep",
			on, below, above, ratio)
	}
	steepest := 1
	for i := 2; i < len(c.h); i++ {
		if slope(i) > slope(steepest) {
			steepest = i
		}
	}
	if !band(steepest) {
		t.Errorf("the steepest fall is at tile %d, off the granite band", steepest)
	}
}
