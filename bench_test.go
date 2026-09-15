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
			for b.Loop() {
				NewLand(1, terms)
			}
			// Per tile, so that worlds of different sizes can be read
			// against one another.
			tiles := float64(terms.Width * terms.Height)
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/tiles, "ns/tile")
		})
	}
}
