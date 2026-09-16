//go:build goexperiment.simd && amd64

package terra

import "simd/archsimd"

// The kernels four lanes at a time, on a processor with AVX2. This file is
// built only when the go command is asked for it - GOEXPERIMENT=simd - and
// its kernels are used only where the processor has the instructions;
// everywhere else the kernel is its statement in kernel.go, which is also
// the tail of every run here. See kernel.go for what a lane may be held to.
//
// Every kernel clears the upper halves of the vector registers before it
// hands the run's tail, and the rest of the pass, back to ordinary code.
// The compiler does not yet do that itself, and ordinary code run with
// those halves dirty is several times slower than it should be on the
// processors this was measured on - slower, all told, than never having
// used the vectors at all. See golang/go#80835.

// vector is whether this processor can do a kernel four lanes at once.
var vector = archsimd.X86.AVX2()

// lanes is how many numbers one vector holds.
const lanes = 4

// whole is the length of the part of a run of n that is done in whole
// vectors; the rest is the tail.
func whole(n int) int { return n &^ (lanes - 1) }

func fade(wear []float64, by float64) {
	if !vector {
		fadeScalar(wear, by)
		return
	}
	n := whole(len(wear))
	byv := archsimd.BroadcastFloat64x4(by)
	for j := 0; j < n; j += lanes {
		archsimd.LoadFloat64x4(wear[j : j+lanes]).Mul(byv).Store(wear[j : j+lanes])
	}
	archsimd.ClearAVXUpperBits()
	fadeScalar(wear[n:], by)
}

// grow gates on the tile's kind: the kinds that age, spread across the lanes
// once, and each lane compared against all of them. A lane the pass is not
// for keeps its bits, chosen back by the mask, rather than being multiplied
// by nought or one, which would turn a negative nought positive.
func grow(age []float64, ks []int64, k float64) {
	if !vector {
		growScalar(age, ks, k)
		return
	}
	var want [8]archsimd.Int64x4
	if len(aging) == 0 || len(aging) > len(want) {
		// Nothing ages, or more kinds grow than this was written for. The
		// first is not idle: compared against no kind at all, the lanes
		// would be compared against nought, which is a kind - open grass
		// with nothing on it - and every such tile would age.
		growScalar(age, ks, k)
		return
	}
	for i, kk := range aging {
		want[i] = archsimd.BroadcastInt64x4(kk)
	}
	n := whole(min(len(age), len(ks)))
	kv := archsimd.BroadcastFloat64x4(k)
	for j := 0; j < n; j += lanes {
		kind := archsimd.LoadInt64x4(ks[j : j+lanes])
		m := kind.Equal(want[0])
		for i := 1; i < len(aging); i++ {
			m = m.Or(kind.Equal(want[i]))
		}
		a := archsimd.LoadFloat64x4(age[j : j+lanes])
		a.Add(kv).IfElse(m, a).Store(age[j : j+lanes])
	}
	archsimd.ClearAVXUpperBits()
	growScalar(age[n:], ks[n:], k)
}
