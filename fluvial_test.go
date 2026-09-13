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
func TestAnUpliftedRiverReachesItsSteadySlope(t *testing.T) {
	const uplift = 0.5 // metres a step
	for _, by := range []float64{1, 2000} {
		c, q := chain(200, Erodibility, by)
		for step := 0; step < 4000; step++ {
			for i := 1; i < len(c.h); i++ {
				c.h[i] += uplift
			}
			next := c.solve(1)
			copy(c.h, next)
		}
		for i := 1; i < len(c.h); i++ {
			slope := (c.h[i] - c.h[i-1]) / TileSpan
			want := uplift / (by * Erodibility * math.Sqrt(q[i]))
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
	g.fill()
	g.drain()
	before := 0.0
	for i := range g.Tiles {
		before += g.Tiles[i].Height
	}
	g.wear(5)
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
