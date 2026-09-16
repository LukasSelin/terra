package terra

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"runtime/metrics"
	"testing"
	"time"
)

// What making a world asks of the heap is held to a budget, so that a change
// that makes it ask for much more is noticed by the suite rather than by the
// next person to profile it. See docs/perf/README.md.
//
// The heap is what can be held to a budget in a test and the clock is not:
// the bytes a world is made with are the same on every machine and every run
// to a few hundredths of a percent once the goroutines are fixed, and the time
// it takes is not the same on one machine twice in an afternoon. The time is
// written down beside the bytes and logged against them, and held to account
// by scripts/perf.sh with benchstat, where there are enough runs to tell drift
// from noise.
//
// A change that is meant to move the heap - a pass that keeps its scratch now,
// or one that genuinely needs more - rewrites the budget with
//
//	TERRA_PERF_UPDATE=1 go test -run TestWorldCreationBudget .
//
// and the diff of docs/perf/budget.json goes in the same commit, which is how
// a reviewer sees what the change cost.
const budgetFile = "docs/perf/budget.json"

// The worlds budgeted are small enough to run in the suite and between them
// go through every pass: a drawn valley, a valley run through its history,
// and a globe - wrapped, poured, with orographic patches - at an eighth of
// the preset's width.
var budgetWorlds = []struct {
	name  string
	terms func() Terms
}{
	{"valley", DefaultTerms},
	{"ancient", AncientTerms},
	{"globe128", func() Terms {
		t := GlobeTerms()
		t.Width, t.Height = 128, 64
		return t
	}},
}

// budgetWorkers is how many goroutines the budgeted worlds are made over.
// Each goroutine a pass is dealt over allocates a little of its own, so the
// count is fixed here rather than left to the machine's cores.
const budgetWorkers = 4

// How far over its budget a world may come before the test fails. The bytes
// move by a few hundredths of a percent between runs and the allocation count
// by a few tenths, so these catch a pass that starts making a tile-sized
// slice it did not, and not the goroutines' scheduling.
const (
	bytesSlack  = 0.01
	allocsSlack = 0.03
)

// The peak is written and logged but not held to a slack, because no
// reading of it taken from outside the world is steady enough to hold: on
// 2026-09-15, with two other test runs on the machine, three runs put the
// ancient valley's peak at 6.2, 14.1 and 8.3 MiB. That is not the sampler
// missing the top - reading the live heap at the collector's marks, and
// forcing a mark every MiB allocated, both spread as wide - it is that
// whatever the world allocates during a concurrent mark is counted live,
// and a mark takes longer on a loaded machine. A peak that does not move
// with load needs the world to hold still while it is read: a hook at each
// pass boundary, in phases.go, that collects and reads the heap when this
// test asks. When that exists, set peakSlack and check it here like the
// bytes. See docs/perf/README.md.
const peakSlack = 0 // not checked; see above

// peakEvery is how often the sampler reads the heap while a world is made.
// A valley is made in a hundred and some milliseconds, so it is not every
// hundred; the read is not a stop-the-world one, so it can be every one.
const peakEvery = time.Millisecond

// budget is one world's line in docs/perf/budget.json.
type budget struct {
	Bytes  uint64 `json:"bytes"`
	Allocs uint64 `json:"allocs"`
	// Peak is the most the heap held at once while the world was made, over
	// what it held before: the highest HeapAlloc the sampler saw, less the
	// HeapAlloc after the collection that precedes the world. Bytes is what
	// the world churns through; Peak is what a machine has to have, and so
	// what bounds the size of world a machine can make. It is logged, not
	// checked, until it can be read with the world held still; see peakSlack.
	Peak uint64 `json:"peak"`
	// Nanos is what the world took on the machine the budget was written on.
	// It is not checked; see the top of the file.
	Nanos int64 `json:"nanos"`
}

type budgets struct {
	Note    string            `json:"note"`
	Workers int               `json:"workers"`
	Worlds  map[string]budget `json:"worlds"`
}

// spend makes the world once and says what that cost. The collector is run
// first so that what was already on the heap is not counted, and nothing
// else runs meanwhile: this test is not parallel, and a test that is waits
// for every one that is not.
func spend(terms Terms) budget {
	was := Workers
	Workers = budgetWorkers
	defer func() { Workers = was }()
	// The sampler and its channels are made before the heap is read, so that
	// what they allocate is not put down to the world.
	stop := make(chan struct{})
	peaked := make(chan uint64)
	go samplePeak(stop, peaked)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	NewLand(1, terms)
	took := time.Since(start)
	close(stop)
	peak := <-peaked
	runtime.ReadMemStats(&after)
	if peak < before.HeapAlloc {
		peak = before.HeapAlloc
	}
	return budget{
		Bytes:  after.TotalAlloc - before.TotalAlloc,
		Allocs: after.Mallocs - before.Mallocs,
		Peak:   peak - before.HeapAlloc,
		Nanos:  took.Nanoseconds(),
	}
}

// samplePeak reads the heap every peakEvery until stop is closed, reads it
// once more, and sends the most it saw. It reads through runtime/metrics
// rather than ReadMemStats so that a sample does not stop the world; the
// objects metric is MemStats.HeapAlloc by another name. The loop allocates
// nothing after its first sleep, so it does not show in the world's count.
func samplePeak(stop <-chan struct{}, peaked chan<- uint64) {
	samples := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	var peak uint64
	for {
		metrics.Read(samples)
		if v := samples[0].Value.Uint64(); v > peak {
			peak = v
		}
		select {
		case <-stop:
			peaked <- peak
			return
		default:
		}
		time.Sleep(peakEvery)
	}
}

func TestWorldCreationBudget(t *testing.T) {
	update := os.Getenv("TERRA_PERF_UPDATE") != ""
	var want budgets
	if !update {
		raw, err := os.ReadFile(budgetFile)
		if err != nil {
			t.Fatalf("no budget to hold the worlds to (%v); write one with TERRA_PERF_UPDATE=1", err)
		}
		if err := json.Unmarshal(raw, &want); err != nil {
			t.Fatalf("%s: %v", budgetFile, err)
		}
	}
	got := budgets{
		Note:    "Written by TERRA_PERF_UPDATE=1 go test -run TestWorldCreationBudget. Bytes and allocs are checked; peak and nanos are for reference only. See docs/perf/README.md.",
		Workers: budgetWorkers,
		Worlds:  map[string]budget{},
	}
	for _, w := range budgetWorlds {
		t.Run(w.name, func(t *testing.T) {
			if testing.Short() && w.name == "globe128" {
				t.Skip("a globe takes seconds to make")
			}
			spent := spend(w.terms())
			got.Worlds[w.name] = spent
			if update {
				t.Logf("%s: %s", w.name, describe(spent))
				return
			}
			have, ok := want.Worlds[w.name]
			if !ok {
				t.Fatalf("%s has no budget; write one with TERRA_PERF_UPDATE=1", w.name)
			}
			t.Logf("spent %s against a budget of %s (time %+.0f%%, peak %+.0f%%, not checked)",
				describe(spent), describe(have), 100*(float64(spent.Nanos)/float64(have.Nanos)-1),
				100*(float64(spent.Peak)/float64(have.Peak)-1))
			over(t, "bytes", spent.Bytes, have.Bytes, bytesSlack)
			over(t, "allocations", spent.Allocs, have.Allocs, allocsSlack)
			if peakSlack > 0 {
				over(t, "peak", spent.Peak, have.Peak, peakSlack)
			}
		})
	}
	if update {
		if testing.Short() {
			t.Fatal("a budget written under -short would have no globe in it")
		}
		raw, err := json.MarshalIndent(got, "", "\t")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(budgetFile, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", budgetFile)
	}
}

// over fails t where spent is more than slack over have, and says so where it
// is more than slack under, since a budget nobody lowers stops catching
// anything.
func over(t *testing.T, what string, spent, have uint64, slack float64) {
	t.Helper()
	ratio := float64(spent) / float64(have)
	switch {
	case ratio > 1+slack:
		t.Errorf("%s: %d against a budget of %d, %+.2f%% (slack is %.0f%%): a pass is asking the heap for more than it did. If that is meant, rewrite the budget with TERRA_PERF_UPDATE=1",
			what, spent, have, 100*(ratio-1), 100*slack)
	case ratio < 1-slack:
		t.Logf("%s: %d against a budget of %d, %+.2f%%: under budget - rewrite it with TERRA_PERF_UPDATE=1 so the saving is kept",
			what, spent, have, 100*(ratio-1))
	}
}

func describe(b budget) string {
	return fmt.Sprintf("%.1f MiB in %d allocations, %.1f MiB at the peak, %v", float64(b.Bytes)/(1<<20), b.Allocs,
		float64(b.Peak)/(1<<20), time.Duration(b.Nanos).Round(time.Millisecond))
}
