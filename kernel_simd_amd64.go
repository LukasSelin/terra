//go:build goexperiment.simd && amd64

package terra

import (
	"unsafe"

	"simd/archsimd"
)

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

// The transform's butterflies two complex numbers at a time.
//
// A complex128 is its real part then its imaginary part, so a run of them is
// a run of float64s, and a vector of four is two of them. The butterfly is
//
//	b = x[k+half] * root
//	x[k], x[k+half] = x[k]+b, x[k]-b
//
// and the adding and subtracting are the vector's, lane by lane. The product
// is not a lane-by-lane product - its real part takes the imaginary parts too
// - so the lanes are crossed first: with x = (re, im) and the lanes swapped
// s = (im, re),
//
//	x*p + s*q,  p = (Re r, Re r),  q = (-Im r, Im r)
//
// is (re·Re r - im·Im r, im·Re r + re·Im r), which is what Go writes for the
// complex product, to the bit: x - y is x + (-y) in IEEE arithmetic, a
// product of a negated factor is the negated product, and a sum is the same
// whichever side it is added from. The one thing it is not held to is which
// NaN's bits come out where both sides of that sum are NaN: the processor
// keeps the first operand's, and the compiler may put either side first in
// a sum, so that is not a fact about the statement either. No field a map
// transforms holds a NaN. p and q are laid out for every level in the plan,
// once. fft_test.go and FuzzButterflies hold it all to the scalar
// butterflies.

// fftWide is a plan's roots for the vectors: for each level whose half is two
// or more, p and q above, forward and back, one pair of lanes per root.
type fftWide struct {
	p, q [2][][]float64 // [inverse][level][2*half]
}

// widen lays a plan's roots out for the vectors. Levels are counted by their
// half: level l has half 1<<l.
func widen(pl *fftPlan) *fftWide {
	if !vector {
		return nil
	}
	n := 2 * len(pl.roots)
	w := &fftWide{}
	for dir, roots := range [2][]complex128{pl.roots, pl.back} {
		for half := 1; half < n; half <<= 1 {
			stride := n / (2 * half)
			p, q := make([]float64, 2*half), make([]float64, 2*half)
			for k := 0; k < half; k++ {
				r := roots[k*stride]
				p[2*k], p[2*k+1] = real(r), real(r)
				q[2*k], q[2*k+1] = -imag(r), imag(r)
			}
			w.p[dir] = append(w.p[dir], p)
			w.q[dir] = append(w.q[dir], q)
		}
	}
	return w
}

func butterflies(x []complex128, pl *fftPlan, inverse bool) {
	n := len(x)
	if pl.wide == nil || n < 4 {
		butterfliesScalar(x, pl, inverse, 1, n)
		return
	}
	// The pairs, whose half is one root, are not worth a vector.
	butterfliesScalar(x, pl, inverse, 1, 2)
	dir := 0
	if inverse {
		dir = 1
	}
	f := unsafe.Slice((*float64)(unsafe.Pointer(unsafe.SliceData(x))), 2*n)
	level := 1
	for half := 2; half < n; half <<= 1 {
		p, q := pl.wide.p[dir][level], pl.wide.q[dir][level]
		level++
		for start := 0; start < n; start += 2 * half {
			lo := f[2*start : 2*(start+half)]
			hi := f[2*(start+half) : 2*(start+2*half)]
			for j := 0; j < 2*half; j += lanes {
				a := archsimd.LoadFloat64x4(lo[j : j+lanes])
				xb := archsimd.LoadFloat64x4(hi[j : j+lanes])
				crossed := xb.ConcatPermuteScalarsGrouped(1, 0, xb)
				b := xb.Mul(archsimd.LoadFloat64x4(p[j : j+lanes])).
					Add(crossed.Mul(archsimd.LoadFloat64x4(q[j : j+lanes])))
				a.Add(b).Store(lo[j : j+lanes])
				a.Sub(b).Store(hi[j : j+lanes])
			}
		}
	}
	archsimd.ClearAVXUpperBits()
}

func axpy(y, x []float64, a float64) {
	if !vector {
		axpyScalar(y, x, a)
		return
	}
	x = x[:len(y)]
	n := whole(len(y))
	av := archsimd.BroadcastFloat64x4(a)
	for j := 0; j < n; j += lanes {
		archsimd.LoadFloat64x4(y[j : j+lanes]).Add(archsimd.LoadFloat64x4(x[j : j+lanes]).Mul(av)).Store(y[j : j+lanes])
	}
	archsimd.ClearAVXUpperBits()
	axpyScalar(y[n:], x[n:], a)
}

func lerp(dst, a, b []float64, t float64) {
	if !vector {
		lerpScalar(dst, a, b, t)
		return
	}
	a, b = a[:len(dst)], b[:len(dst)]
	n := whole(len(dst))
	tv := archsimd.BroadcastFloat64x4(t)
	for j := 0; j < n; j += lanes {
		av := archsimd.LoadFloat64x4(a[j : j+lanes])
		av.Add(archsimd.LoadFloat64x4(b[j : j+lanes]).Sub(av).Mul(tv)).Store(dst[j : j+lanes])
	}
	archsimd.ClearAVXUpperBits()
	lerpScalar(dst[n:], a[n:], b[n:], t)
}

func clamp(v []float64, lo, hi float64) {
	if !vector {
		clampScalar(v, lo, hi)
		return
	}
	n := whole(len(v))
	lov, hiv := archsimd.BroadcastFloat64x4(lo), archsimd.BroadcastFloat64x4(hi)
	for j := 0; j < n; j += lanes {
		x := archsimd.LoadFloat64x4(v[j : j+lanes])
		x = lov.IfElse(x.Less(lov), x)
		x = hiv.IfElse(x.Greater(hiv), x)
		x.Store(v[j : j+lanes])
	}
	archsimd.ClearAVXUpperBits()
	clampScalar(v[n:], lo, hi)
}

func sumTree(v []float64) float64 {
	if !vector {
		return sumTreeScalar(v)
	}
	n := whole(len(v))
	acc := archsimd.BroadcastFloat64x4(0)
	for j := 0; j < n; j += lanes {
		acc = acc.Add(archsimd.LoadFloat64x4(v[j : j+lanes]))
	}
	var s [lanes]float64
	acc.StoreArray(&s)
	archsimd.ClearAVXUpperBits()
	return sumTail(s, v[n:])
}
