package terra

import (
	"testing"

	"github.com/LukasSelin/terra/internal/phase"
)

// How many times each of the heavy passes runs for a preset is pinned here,
// so that a branch which adds a drain - a realism change that asks for one
// more settling of the water, say - shows it in a test diff rather than in
// the next benchmark. The clock says a world got slower; this says why.
//
// The counts are read from the per-pass clock in phases.go. Run the test
// with TERRA_PHASES=1 -count=1: the clock is decided at init from the
// environment, and without it the test skips before it makes a world.
//
// The table was read on 2026-09-16 on main at 639d7d5, over budgetWorkers
// goroutines like the budget; the counts do not depend on the goroutines.
// A branch that means to change a count rewrites the row with the change,
// and the work-log entry says what the extra pass bought. The first thing
// the table showed was the weather gate (41bd904): drain reads the weather
// only when the ground has moved from under it, and the weather count fell
// from 7, 25 and 31 to 2, 20 and 23 while every other pass held.
var passCounts = func() map[string]int {
	counts := map[string]int{}
	for _, p := range Phases() {
		counts[p.Name] = p.Calls
	}
	return counts
}

// resetPassCounts zeroes the clock before a world, so that the counts are
// that world's alone.
var resetPassCounts = ResetPhases

var pinnedPassCounts = map[string]map[string]int{
	"valley":   {"drain": 6, "weather": 2, "wear": 4, "landslide": 5},
	"ancient":  {"drain": 24, "weather": 20, "wear": 20, "landslide": 6},
	"globe128": {"drain": 30, "weather": 23, "wear": 20, "landslide": 6},
}

func TestPassCountsArePinned(t *testing.T) {
	if !phase.On() {
		t.Skip("the per-pass clock is off; run with TERRA_PHASES=1 -count=1")
	}
	was := Workers
	Workers = budgetWorkers
	defer func() { Workers = was }()
	for _, w := range budgetWorlds {
		t.Run(w.name, func(t *testing.T) {
			if testing.Short() && w.name == "globe128" {
				t.Skip("a globe takes seconds to make")
			}
			want, ok := pinnedPassCounts[w.name]
			if !ok {
				t.Fatalf("%s has no pinned counts", w.name)
			}
			resetPassCounts()
			NewLand(1, w.terms())
			got := passCounts()
			for name, n := range want {
				if got[name] != n {
					t.Errorf("%s ran %d times, and the table pins %d: if the extra pass is meant, rewrite the row with the change", name, got[name], n)
				}
			}
		})
	}
}
