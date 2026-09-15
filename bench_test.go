package terra

import "testing"

// How long a world takes to make, and how much it asks of the heap while it
// is made. These are the yardsticks docs/perf/worklog.md is kept against:
// run them before and after a change, a handful of times each, and hand both
// runs to benchstat. See docs/perf/README.md for the commands.
//
// The worlds are the presets a game is actually made on, and one between
// them: a quarter-scale globe, which runs every pass the globe does over a
// sixteenth of its tiles, so that a change can be measured in seconds and
// only confirmed on the full globe.
//
// With TERRA_PHASES=1 in the environment each pass is reported as a metric
// too, s/<pass>, so that benchstat can compare passes between runs, and the
// table is logged with the calls. See phases.go.
var benchWorlds = []struct {
	name  string
	terms func() Terms
}{
	{"valley", DefaultTerms},
	{"ancient", AncientTerms},
	{"globe256", func() Terms {
		t := GlobeTerms()
		t.Width, t.Height = 256, 128
		return t
	}},
	{"globe", GlobeTerms},
}

func BenchmarkNewLand(b *testing.B) {
	for _, w := range benchWorlds {
		b.Run(w.name, func(b *testing.B) {
			terms := w.terms()
			b.ReportAllocs()
			ResetPhases()
			for b.Loop() {
				NewLand(1, terms)
			}
			// Per tile, so that worlds of different sizes can be read
			// against one another.
			tiles := float64(terms.Width * terms.Height)
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/tiles, "ns/tile")
			if phases := Phases(); len(phases) > 0 {
				runs := float64(b.N)
				for i := range phases {
					phases[i].Seconds /= runs
					phases[i].Calls = int(float64(phases[i].Calls)/runs + 0.5)
					b.ReportMetric(phases[i].Seconds, "s/"+phases[i].Name)
				}
				b.Logf("phases, per world:\n%s", PhaseTable(phases))
			}
		})
	}
}
