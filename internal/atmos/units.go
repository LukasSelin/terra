package atmos

import (
	"math"

	"github.com/LukasSelin/terra/clock"
	"github.com/LukasSelin/terra/internal/par"
)

// Year is the calendar's year, over which the air's seasons turn: see
// clock.Year.
const Year = clock.Year

// The figures the air is worked out in, which the land has too and under the
// same names: the acceleration of gravity in m/s², a kilometre in metres, the
// seconds in a year, and how many cells of a lattice a pass has before it is
// spread over goroutines.
const (
	gravity        = 9.81
	km             = 1000.0
	secondsPerYear = 365.25 * 24 * 3600
	spreadTiles    = 1 << 14
)

// clamp01 is v held to [0, 1].
func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// workers is how many goroutines a pass over n things may use: one, until the
// land says otherwise with SetWorkers.
var workers = func(n int) int { return 1 }

// SetWorkers says how many goroutines a pass over n things may be spread
// over. The land sets it once, to its own reading of terra.Workers; the air
// is the same whatever it says.
func SetWorkers(f func(n int) int) { workers = f }

func workersFor(n int) int { return workers(n) }

func inParallel(n, w int, f func(i, worker int)) { par.InParallel(n, w, f) }
