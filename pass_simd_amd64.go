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

func fill(s, age []float64, ks []int64, kind int64, spanned bool, full, by float64) {
	if !vector {
		fillScalar(s, age, ks, kind, full, by)
		return
	}
	n := len(s) &^ (lanes - 1)
	// Whether the process has a span comes in settled: a scalar comparison
	// of full here would be encoded the old way and paid for, as the
	// remarks above say - and inside the loops it would be paid for on
	// every turn. The nought is one less one for the same reason: a nought
	// made the ordinary way is made the old way.
	want := archsimd.BroadcastInt64x4(kind)
	byv := archsimd.BroadcastFloat64x4(by)
	one := archsimd.BroadcastFloat64x4(1)
	zero := one.Sub(one)
	if spanned {
		fullv := archsimd.BroadcastFloat64x4(full)
		for j := 0; j < n; j += lanes {
			m := archsimd.LoadInt64x4(ks[j : j+lanes]).Equal(want)
			have := archsimd.LoadFloat64x4(s[j : j+lanes])
			// The ceiling is the age over the span, clamped to [0, 1];
			// see Grown. The clamp is math.Max(0, math.Min(1, v)) written
			// out: v where v is under one, else one; that where it is
			// over nought, else a positive nought.
			v := archsimd.LoadFloat64x4(age[j : j+lanes]).Div(fullv)
			v = v.IfElse(v.Less(one), one)
			ceiling := v.IfElse(v.Greater(zero), zero)
			filled(have, ceiling, byv).IfElse(m, have).Store(s[j : j+lanes])
		}
	} else {
		for j := 0; j < n; j += lanes {
			m := archsimd.LoadInt64x4(ks[j : j+lanes]).Equal(want)
			have := archsimd.LoadFloat64x4(s[j : j+lanes])
			filled(have, one, byv).IfElse(m, have).Store(s[j : j+lanes])
		}
	}
	archsimd.ClearAVXUpperBits()
	fillScalar(s[n:], age[n:], ks[n:], kind, full, by)
}

// filled is grown over four lanes: what is standing already, where it is
// at or over the ceiling; else what is standing plus what the day puts on,
// up to the ceiling - min written as the comparison and the choice.
func filled(have, ceiling, by archsimd.Float64x4) archsimd.Float64x4 {
	sum := have.Add(by)
	up := sum.IfElse(sum.Less(ceiling), ceiling)
	return have.IfElse(have.GreaterEqual(ceiling), up)
}

func shoal(fish []float64, ks []int64, by float64) {
	if !vector {
		shoalScalar(fish, ks, by)
		return
	}
	n := len(fish) &^ (lanes - 1)
	byv, one := archsimd.BroadcastFloat64x4(by), archsimd.BroadcastFloat64x4(1)
	low, water := archsimd.BroadcastInt64x4(kindMask), archsimd.BroadcastInt64x4(int64(Water))
	for j := 0; j < n; j += lanes {
		m := archsimd.LoadInt64x4(ks[j : j+lanes]).And(low).Equal(water)
		f := archsimd.LoadFloat64x4(fish[j : j+lanes])
		sum := f.Add(byv)
		sum.IfElse(sum.Less(one), one).IfElse(m, f).Store(fish[j : j+lanes])
	}
	archsimd.ClearAVXUpperBits()
	shoalScalar(fish[n:], ks[n:], by)
}

// meadow is shoal for the grass: the same sum and the same clamp, gated on
// open ground rather than water.
func meadow(sward []float64, ks []int64, by float64) {
	if !vector {
		meadowScalar(sward, ks, by)
		return
	}
	n := len(sward) &^ (lanes - 1)
	byv, one := archsimd.BroadcastFloat64x4(by), archsimd.BroadcastFloat64x4(1)
	low, grass := archsimd.BroadcastInt64x4(kindMask), archsimd.BroadcastInt64x4(int64(Grass))
	for j := 0; j < n; j += lanes {
		m := archsimd.LoadInt64x4(ks[j : j+lanes]).And(low).Equal(grass)
		f := archsimd.LoadFloat64x4(sward[j : j+lanes])
		sum := f.Add(byv)
		sum.IfElse(sum.Less(one), one).IfElse(m, f).Store(sward[j : j+lanes])
	}
	archsimd.ClearAVXUpperBits()
	meadowScalar(sward[n:], ks[n:], by)
}

func rest(fert, rich []float64, ks []int64, by float64) {
	if !vector {
		restScalar(fert, rich, ks, by)
		return
	}
	n := len(fert) &^ (lanes - 1)
	byv := archsimd.BroadcastFloat64x4(by)
	low, field := archsimd.BroadcastInt64x4(kindMask), archsimd.BroadcastInt64x4(int64(Field))
	for j := 0; j < n; j += lanes {
		m := archsimd.LoadInt64x4(ks[j : j+lanes]).And(low).Equal(field)
		f := archsimd.LoadFloat64x4(fert[j : j+lanes])
		r := archsimd.LoadFloat64x4(rich[j : j+lanes])
		sum := f.Add(byv)
		sum.IfElse(sum.Less(r), r).IfElse(m, f).Store(fert[j : j+lanes])
	}
	archsimd.ClearAVXUpperBits()
	restScalar(fert[n:], rich[n:], ks[n:], by)
}
