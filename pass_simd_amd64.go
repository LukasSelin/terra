//go:build goexperiment.simd && amd64

package terra

import "simd/archsimd"

// The day's pass four tiles at a time, on a processor with AVX2. It is
// built only when the go command is asked for it - GOEXPERIMENT=simd - and
// used only where the processor has the instructions; everywhere else the
// pass is pass_noasm.go, one tile at a time. See pass.go.
//
// Every lane comes out bit for bit what the tile would have come to one at
// a time, and it is worth saying how. Adding, multiplying and dividing a
// lane is the same rounding as adding, multiplying and dividing a number.
// The minimums and the clamps are not written with the vector minimum,
// whose answer when the two sides are equal is a fact about the
// instruction rather than about the numbers; they are written as the
// comparison and the choice that Go's min and math.Max come to, so that a
// lane and a number cannot part company on a signed nought. And nothing is
// gated by multiplying by nought or one, which would turn a negative nought
// into a positive one: a lane the pass is not for keeps its bits, chosen
// back by the mask. The test in pass_test.go holds all of it to Ripen and
// Replenish, negative noughts included.
//
// Every pass clears the upper halves of the vector registers before it
// hands the run's tail, and the rest of the day, back to ordinary code.
// The compiler does not yet do that itself, and ordinary code run with
// those halves dirty is several times slower than it should be on the
// processors this was measured on - slower, all told, than never having
// used the vectors at all. See golang/go#80835.

// vector is whether this processor can do the pass four tiles at once.
var vector = archsimd.X86.AVX2()

// lanes is how many tiles one vector holds.
const lanes = 4

func fade(wear []float64, by float64) {
	if !vector {
		fadeScalar(wear, by)
		return
	}
	n := len(wear) &^ (lanes - 1)
	byv := archsimd.BroadcastFloat64x4(by)
	for j := 0; j < n; j += lanes {
		archsimd.LoadFloat64x4(wear[j : j+lanes]).Mul(byv).Store(wear[j : j+lanes])
	}
	archsimd.ClearAVXUpperBits()
	fadeScalar(wear[n:], by)
}

func grow(age []float64, ks []int64, k float64) {
	if !vector {
		growScalar(age, ks, k)
		return
	}
	n := len(age) &^ (lanes - 1)
	kv := archsimd.BroadcastFloat64x4(k)
	// The kinds that age, spread across the lanes once. There are a few,
	// and a tile is one of them or it is not.
	var want [8]archsimd.Int64x4
	for i, kk := range aging[:min(len(aging), len(want))] {
		want[i] = archsimd.BroadcastInt64x4(kk)
	}
	if len(aging) > len(want) {
		growScalar(age, ks, k) // more kinds grow than this was written for
		return
	}
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

// The fillings are one tile at a time on every processor. Each is a closed
// form now - a cube root, a logarithm and an exponential for a stand, a
// division for the rest - and none of those is an instruction the vectors
// have; see stand and logistic. What the vectors still do is the ageing and
// the fading, which are the passes that touch every tile.

func fill(s, age []float64, ks []int64, kind int64, _ bool, full, rate, k float64) {
	fillScalar(s, age, ks, kind, full, rate, k)
}
func shoal(fish []float64, ks []int64, fall float64)      { shoalScalar(fish, ks, fall) }
func rest(fert, rich []float64, ks []int64, fall float64) { restScalar(fert, rich, ks, fall) }
func meadow(sward []float64, ks []int64, fall float64)    { meadowScalar(sward, ks, fall) }
