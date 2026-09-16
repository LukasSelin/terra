package terra

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// The per-pass clock. Every pass of world creation that is worth a line in
// docs/perf/worklog.md starts with
//
//	defer phase("drain")()
//
// and, with TERRA_PHASES=1 in the environment, the wall-clock time and the
// number of calls are summed under that name. Phases reads the table back;
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
// holds either way: the entries for the passes below are made at init, each
// with its own stop function and room for the times it was started at, and
// a call pushes and pops one of those.
var phasesOn = os.Getenv("TERRA_PHASES") == "1"

// The passes that are timed, so that their entries can be made before any
// world is. A name not here is still counted; it costs its entry the first
// time it runs.
var phaseNames = []string{
	"Generate", "history", "move", "joinUp", "tectonics", "reshape",
	"keepBook", "settleRock", "basins", "drain", "weather", "windsFor",
	"rainOn", "orographic", "airEnv.vapour", "airEnv.currents", "pool",
	"flow", "wear", "waterStep", "fluvial.solve", "creep", "landslide",
	"shape", "cutValleys", "silt", "tides", "readFeatures",
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
	phaseMu    sync.Mutex
	phaseTable = map[string]*phaseEntry{}
)

func init() {
	if !phasesOn {
		return
	}
	for _, name := range phaseNames {
		newPhase(name)
	}
}

// newPhase makes the entry for name. The caller holds phaseMu, or is init.
func newPhase(name string) *phaseEntry {
	p := &phaseEntry{Phase: Phase{Name: name}, starts: make([]time.Time, 0, 8)}
	p.stop = func() {
		now := time.Now()
		phaseMu.Lock()
		if n := len(p.starts); n > 0 {
			p.Seconds += now.Sub(p.starts[n-1]).Seconds()
			p.starts = p.starts[:n-1]
			p.Calls++
		}
		phaseMu.Unlock()
	}
	phaseTable[name] = p
	return p
}

func noPhase() {}

// phase starts the clock on name and returns what stops it. Passes run on
// the main goroutine, but the table is under a mutex so that a pass spread
// over goroutines may call it too.
func phase(name string) func() {
	if !phasesOn {
		return noPhase
	}
	phaseMu.Lock()
	p := phaseTable[name]
	if p == nil {
		p = newPhase(name)
	}
	p.starts = append(p.starts, time.Now())
	stop := p.stop
	phaseMu.Unlock()
	return stop
}

// Phases is the table so far, longest first: every pass that has run since
// the package was initialised or ResetPhases was last called. It is empty
// when the instrument is off.
func Phases() []Phase {
	phaseMu.Lock()
	out := make([]Phase, 0, len(phaseTable))
	for _, p := range phaseTable {
		if p.Calls > 0 {
			out = append(out, p.Phase)
		}
	}
	phaseMu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Seconds != out[j].Seconds {
			return out[i].Seconds > out[j].Seconds
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// ResetPhases zeroes the table, so that a benchmark or a test reads only
// the world it made. Nothing should be mid-pass when it is called.
func ResetPhases() {
	phaseMu.Lock()
	for _, p := range phaseTable {
		p.Calls, p.Seconds = 0, 0
		p.starts = p.starts[:0]
	}
	phaseMu.Unlock()
}

// PhaseTable writes phases in the columns of the work-log tables: pass,
// wall seconds, calls, and the share of Generate (or of the longest phase
// when Generate is not among them).
func PhaseTable(phases []Phase) string {
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
