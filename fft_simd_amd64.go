//go:build goexperiment.simd && amd64

package terra

import (
	"unsafe"

	"simd/archsimd"
)

// The transform's butterflies two complex numbers at a time, on a processor
// with AVX2. Like the day's pass in pass_simd_amd64.go it is built only under
// GOEXPERIMENT=simd and used only where the processor has the instructions.
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
// whichever side it is added from. p and q are laid out for every level in
// the plan, once. The test in fft_test.go holds it all to the scalar
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
			for j := 0; j < 2*half; j += 4 {
				a := archsimd.LoadFloat64x4(lo[j : j+4])
				xb := archsimd.LoadFloat64x4(hi[j : j+4])
				crossed := xb.ConcatPermuteScalarsGrouped(1, 0, xb)
				b := xb.Mul(archsimd.LoadFloat64x4(p[j : j+4])).
					Add(crossed.Mul(archsimd.LoadFloat64x4(q[j : j+4])))
				a.Add(b).Store(lo[j : j+4])
				a.Sub(b).Store(hi[j : j+4])
			}
		}
	}
	archsimd.ClearAVXUpperBits()
}
