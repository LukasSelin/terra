package terra

import (
	"math"
	"math/bits"
	"sync"
)

// fft is the discrete Fourier transform of x in place, or its inverse, by the
// iterative radix-2 algorithm (Cormen and others, Introduction to Algorithms,
// ch. 30). The length of x is a power of two. The inverse divides by the
// length, so that fft then its inverse gives x back.
func fft(x []complex128, inverse bool) {
	n := len(x)
	if n <= 1 {
		return
	}
	pl := planFor(n)
	for i, j := range pl.swap {
		if int(j) > i {
			x[i], x[j] = x[j], x[i]
		}
	}
	butterflies(x, pl, inverse) // the kernel: see kernel.go
	if inverse {
		inv := complex(1/float64(n), 0)
		for i := range x {
			x[i] *= inv
		}
	}
}

// fftPlan is what a transform of one length reads over and over: where each
// element is swapped to, and the roots of unity forward and back.
type fftPlan struct {
	swap        []int32
	roots, back []complex128
	// wide is the roots again, laid out for the vectors: see widen in
	// kernel_simd_amd64.go. It is nil in a build without them.
	wide *fftWide
}

var fftPlans sync.Map // length to *fftPlan

func planFor(n int) *fftPlan {
	if p, ok := fftPlans.Load(n); ok {
		return p.(*fftPlan)
	}
	p := &fftPlan{swap: make([]int32, n), roots: make([]complex128, n/2), back: make([]complex128, n/2)}
	shift := 64 - uint(bits.TrailingZeros(uint(n)))
	for i := range p.swap {
		p.swap[i] = int32(bits.Reverse64(uint64(i)) >> shift)
	}
	for k := range p.roots {
		ang := 2 * math.Pi * float64(k) / float64(n)
		p.roots[k] = complex(math.Cos(ang), -math.Sin(ang))
		p.back[k] = complex(math.Cos(ang), math.Sin(ang))
	}
	p.wide = widen(p)
	got, _ := fftPlans.LoadOrStore(n, p)
	return got.(*fftPlan)
}

// fft2 is the two-dimensional transform of a field w by h, each a power of
// two, laid out row by row: each row, then each column. col is room for a
// column, h long.
func fft2(x []complex128, w, h int, inverse bool, col []complex128) {
	for y := 0; y < h; y++ {
		fft(x[y*w:(y+1)*w], inverse)
	}
	for c := 0; c < w; c++ {
		for y := 0; y < h; y++ {
			col[y] = x[y*w+c]
		}
		fft(col, inverse)
		for y := 0; y < h; y++ {
			x[y*w+c] = col[y]
		}
	}
}

// powerOfTwo reports whether n is one.
func powerOfTwo(n int) bool { return n > 0 && n&(n-1) == 0 }
