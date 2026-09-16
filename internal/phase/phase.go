// Package phase is the per-pass clock: wall-clock time and calls, summed by
// the name of the pass, while a world is made. It is the land's instrument
// and knows nothing about a world; the land names its passes and reads the
// table back. See phases.go in the root package for how it is used.
//
// Every pass of world creation that is worth a line in docs/perf/worklog.md
// starts with
//
//	defer phase.Start("drain")()
//
// and, with TERRA_PHASES=1 in the environment, the wall-clock time and the
// number of calls are summed under that name. All reads the table back;
// BenchmarkNewLand reports each phase as a metric and cmd/overview prints
// the table after making a world.
//
// The times are inclusive: drain's includes the weather, pool and flow it
// calls, and a pass that runs on several goroutines at once (airEnv.vapour
// under rainOn, say) sums its time on each of them, so it can read as more
// than the wall clock of the pass that spread it. Read the table as the
// work-log entries do, top-down.
//
// The instrument is off unless the environment says otherwise, and when it
// is off a pass pays one read of a package-level bool and a deferred call
// of a shared no-op. It is decided once, when the package is initialised,
// which is before the test log starts watching the environment: a go test
// that switches it needs -count=1, or the cache answers for the other
// setting.
//
// On, it asks nothing of the heap while a world is made, so the budget
// holds either way: the entries for the passes the land names are made by
// Prepare when it is initialised, each
// with its own stop function and room for the times it was started at, and
// a call pushes and pops one of those.
package phase

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

var on = os.Getenv("TERRA_PHASES") == "1"

// On reports whether the clock is running: TERRA_PHASES=1 when the program
// started.
func On() bool { return on }

// Prepare makes the entries for passes that will be timed, so that timing
// them asks nothing of the heap while a world is made. A name not prepared
// is still counted; it costs its entry the first time it runs. It does
// nothing when the clock is off.
func Prepare(names ...string) {
	if !on {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	for _, name := range names {
		if table[name] == nil {
			newPhase(name)
		}
	}
}

// A Phase is one row of the table: a pass, how often it ran and how long it
// took on the clock, summed over the calls, inclusive of the passes it
// called.
type Phase struct {
	Name    string
	Calls   int
	Seconds float64
}

// A phaseEntry is a Phase with what phase needs to keep time for it: the
// moments it was started at and not yet stopped, and the function that
// stops it. Started moments are popped last-in first-out; when a pass runs
// on several goroutines at once the pairing may cross, but the sum is the
// same once every start has been stopped.
type phaseEntry struct {
	Phase
	starts []time.Time
	stop   func()
}

var (
	mu    sync.Mutex
	table = map[string]*phaseEntry{}
)

// newPhase makes the entry for name. The caller holds mu.
func newPhase(name string) *phaseEntry {
	p := &phaseEntry{Phase: Phase{Name: name}, starts: make([]time.Time, 0, 8)}
	p.stop = func() {
		now := time.Now()
		mu.Lock()
		if n := len(p.starts); n > 0 {
			p.Seconds += now.Sub(p.starts[n-1]).Seconds()
			p.starts = p.starts[:n-1]
			p.Calls++
		}
		mu.Unlock()
	}
	table[name] = p
	return p
}

func noPhase() {}

// Start starts the clock on name and returns what stops it. Passes run on
// the main goroutine, but the table is under a mutex so that a pass spread
// over goroutines may call it too.
func Start(name string) func() {
	if !on {
		return noPhase
	}
	mu.Lock()
	p := table[name]
	if p == nil {
		p = newPhase(name)
	}
	p.starts = append(p.starts, time.Now())
	stop := p.stop
	mu.Unlock()
	return stop
}

// All is the table so far, longest first: every pass that has run since
// the package was initialised or Reset was last called. It is empty
// when the instrument is off.
func All() []Phase {
	mu.Lock()
	out := make([]Phase, 0, len(table))
	for _, p := range table {
		if p.Calls > 0 {
			out = append(out, p.Phase)
		}
	}
	mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Seconds != out[j].Seconds {
			return out[i].Seconds > out[j].Seconds
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Reset zeroes the table, so that a benchmark or a test reads only
// the world it made. Nothing should be mid-pass when it is called.
func Reset() {
	mu.Lock()
	for _, p := range table {
		p.Calls, p.Seconds = 0, 0
		p.starts = p.starts[:0]
	}
	mu.Unlock()
}

// Table writes phases in the columns of the work-log tables: pass,
// wall seconds, calls, and the share of Generate (or of the longest phase
// when Generate is not among them).
func Table(phases []Phase) string {
	if len(phases) == 0 {
		return ""
	}
	whole := phases[0].Seconds
	for _, p := range phases {
		if p.Name == "Generate" {
			whole = p.Seconds
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-18s %9s %6s %6s\n", "pass", "wall s", "calls", "share")
	for _, p := range phases {
		share := 0.0
		if whole > 0 {
			share = 100 * p.Seconds / whole
		}
		fmt.Fprintf(&b, "%-18s %9.2f %6d %5.0f%%\n", p.Name, p.Seconds, p.Calls, share)
	}
	return b.String()
}
