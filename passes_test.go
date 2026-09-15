package terra

import "testing"

// How many times each of the heavy passes runs for a preset is pinned here,
// so that a branch which adds a drain - a realism change that asks for one
// more settling of the water, say - shows it in a test diff rather than in
// the next benchmark. The clock says a world got slower; this says why.
//
// The counts are read from the per-pass clock in phases.go, which is
// session 0's file (docs/perf/briefs/0-instrument.md) and not yet on main.
// Until it is, passCounts is nil and the test skips. When phases.go lands,
// replace the nil with
//
//	var passCounts = func() map[string]int {
//		counts := map[string]int{}
//		for _, p := range Phases() {
//			counts[p.Name] = p.Calls
//		}
//		return counts
//	}
//
// and run the test with TERRA_PHASES=1 -count=1: the clock is decided at
// init from the environment, and without it Phases() is empty. The test
// says so rather than failing when the clock is off.
//
// The table was read on 2026-09-16 from claude/perf-instrument at e946062,
// over budgetWorkers goroutines like the budget; the counts do not depend
// on the goroutines. A branch that means to change a count rewrites the
// row with the change, and the work-log entry says what the extra pass
// bought.
var passCounts func() map[string]int

// resetPassCounts zeroes the clock before a world, so that the counts are
// that world's alone. Set it to ResetPhases beside passCounts.
var resetPassCounts func()

var pinnedPassCounts = map[string]map[string]int{
	"valley":   {"drain": 6, "weather": 7, "wear": 4, "landslide": 5},
	"ancient":  {"drain": 24, "weather": 25, "wear": 20, "landslide": 6},
	"globe128": {"drain": 30, "weather": 31, "wear": 20, "landslide": 6},
}

func TestPassCountsArePinned(t *testing.T) {
	if passCounts == nil {
		t.Skip("phases.go is not on main yet; set passCounts to read Phases() (see the top of this file)")
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
			if resetPassCounts != nil {
				resetPassCounts()
			}
			NewLand(1, w.terms())
			got := passCounts()
			if len(got) == 0 {
				t.Skip("the per-pass clock is off; run with TERRA_PHASES=1 -count=1")
			}
			for name, n := range want {
				if got[name] != n {
					t.Errorf("%s ran %d times, and the table pins %d: if the extra pass is meant, rewrite the row with the change", name, got[name], n)
				}
			}
		})
	}
}
