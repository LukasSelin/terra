// Package par spreads work over goroutines. It is the loop the land and its
// passes share; how many goroutines a pass may use is the land's to say, as
// terra.Workers.
package par

import (
	"sync"
	"sync/atomic"
)

// InParallel runs f for every index below n, spread over workers goroutines,
// and returns when the last of them is done. The indices are dealt out one
// at a time to whichever worker is free, rather than cut into stripes
// beforehand: the work is not all the same size - one agent's deciding can
// cost a hundred others' - and a stripe that happened to hold the dear
// pieces held the day up while the other workers stood idle.
//
// f must not write anything another call to f can see; results belong in a
// slice indexed by i. Which worker gets which index is not a fact about
// anything, and nothing may depend on it; the worker is passed so that f
// can use scratch that is that worker's own.
func InParallel(n, workers int, f func(i, worker int)) {
	if workers <= 1 {
		for i := 0; i < n; i++ {
			f(i, 0)
		}
		return
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	for k := 0; k < workers; k++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			for {
				i := int(next.Add(1)) - 1
				if i >= n {
					return
				}
				f(i, k)
			}
		}(k)
	}
	wg.Wait()
}
