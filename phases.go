package terra

import "github.com/LukasSelin/terra/internal/phase"

// The per-pass clock, as the land uses it. Every pass of world creation that
// is worth a line in docs/perf/worklog.md starts with
//
//	defer phase.Start("drain")()
//
// and, with TERRA_PHASES=1 in the environment, the wall-clock time and the
// number of calls are summed under that name. Phases reads the table back;
// BenchmarkNewLand reports each phase as a metric and cmd/overview prints
// the table after making a world. How the clock keeps time, and what it
// costs on and off, is package phase's: see internal/phase.

// The passes that are timed, so that their entries can be made before any
// world is. A name not here is still counted; it costs its entry the first
// time it runs.
var phaseNames = []string{
	"Generate", "history", "move", "joinUp", "tectonics", "reshape",
	"keepBook", "settleRock", "basins", "drain", "weather", "windsFor",
	"rainOn", "orographic", "airEnv.vapour", "airEnv.currents", "pool",
	"flow", "wear", "waterStep", "fluvial.solve", "creep", "landslide",
	"shape", "cutValleys", "silt", "tides", "readFeatures",
	"stage.ground", "stage.sea", "stage.shape", "stage.cut", "stage.coast",
	"stage.cover",
}

func init() { phase.Prepare(phaseNames...) }

// A Phase is one row of the table: see phase.Phase.
type Phase = phase.Phase

// Phases is the table so far, longest first: every pass that has run since
// the package was initialised or ResetPhases was last called. It is empty
// when the instrument is off.
func Phases() []Phase { return phase.All() }

// ResetPhases zeroes the table, so that a benchmark or a test reads only
// the world it made. Nothing should be mid-pass when it is called.
func ResetPhases() { phase.Reset() }

// PhaseTable writes phases in the columns of the work-log tables: pass,
// wall seconds, calls, and the share of Generate (or of the longest phase
// when Generate is not among them).
func PhaseTable(phases []Phase) string { return phase.Table(phases) }
