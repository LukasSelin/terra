package terra

import (
	"sync/atomic"
	"testing"
)

// The indices are dealt out one at a time to whoever is free, so which
// worker does what is not a fact about anything. What is a fact is that
// every index is done exactly once, by a worker that exists, however
// uneven the work - and that nothing is left undone when one index costs
// as much as all the others.
func TestInParallelDealsEveryIndexOnce(t *testing.T) {
	const n = 1000
	for _, workers := range []int{1, 2, 7, 24, 64} {
		var done [n]atomic.Int32
		var wrong atomic.Int32
		InParallel(n, workers, func(i, worker int) {
			if worker < 0 || worker >= workers {
				wrong.Add(1)
			}
			done[i].Add(1)
			if i%97 == 0 {
				// One dear piece of work among a hundred cheap ones.
				x := 0
				for k := 0; k < 20000; k++ {
					x += k * i
				}
				_ = x
			}
		})
		if wrong.Load() != 0 {
			t.Fatalf("%d workers: %d calls named a worker that does not exist", workers, wrong.Load())
		}
		for i := range done {
			if got := done[i].Load(); got != 1 {
				t.Fatalf("%d workers: index %d was done %d times", workers, i, got)
			}
		}
	}
}

// A cheap pass over the population is spread a goroutine to every
// spreadAgents of them, and not at all under that: handing a few dozen
// lookups to two dozen goroutines cost more than the lookups.
func TestWorkersOverSpreadsOnlyACrowd(t *testing.T) {
	was := Workers
	Workers = 24
	defer func() { Workers = was }()
	for _, c := range []struct{ n, want int }{{0, 1}, {63, 1}, {64, 1}, {128, 2}, {640, 10}, {2000, 24}} {
		if got := WorkersOver(c.n); got != c.want {
			t.Errorf("WorkersOver(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}
