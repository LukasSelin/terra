package kernel

import (
	"fmt"
	"math"
	"math/cmplx"
	"math/rand/v2"
	"slices"
	"testing"
)

// The transform and its inverse give a field back, and the transform of a
// single wave is a single spike where its wavenumber is.
func TestTheTransformGivesTheFieldBack(t *testing.T) {
	const n = 64
	x := make([]complex128, n)
	for i := range x {
		x[i] = complex(math.Cos(2*math.Pi*5*float64(i)/n), 0)
	}
	y := append([]complex128(nil), x...)
	FFT(y, false)
	for k := range y {
		want := 0.0
		if k == 5 || k == n-5 {
			want = n / 2
		}
		if cmplx.Abs(y[k]-complex(want, 0)) > 1e-9 {
			t.Fatalf("mode %d of a wave of five is %v, want %v", k, y[k], want)
		}
	}
	FFT(y, true)
	for i := range x {
		if cmplx.Abs(y[i]-x[i]) > 1e-12 {
			t.Fatalf("element %d came back %v, was %v", i, y[i], x[i])
		}
	}
}

// The butterflies a build does - four lanes at a time under GOEXPERIMENT=simd
// on a processor with AVX2, one element at a time otherwise - come out bit for
// bit what the scalar statement of them does, forward and back, at every
// length a map asks for and on fields with negative noughts, spikes and
// numbers far apart in size in them.
func TestTheButterfliesAreTheScalarOnes(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 9))
	draw := func() float64 {
		switch rng.IntN(8) {
		case 0:
			return math.Copysign(0, -1)
		case 1:
			return 0
		case 2:
			return (rng.Float64() - 0.5) * 1e12
		case 3:
			return (rng.Float64() - 0.5) * 1e-12
		}
		return rng.Float64()*2 - 1
	}
	for n := 1; n <= 4096; n <<= 1 {
		for _, inverse := range []bool{false, true} {
			for trial := 0; trial < 4; trial++ {
				x := make([]complex128, n)
				for i := range x {
					x[i] = complex(draw(), draw())
				}
				want := slices.Clone(x)
				pl := planFor(n)
				butterflies(x, pl, inverse)
				butterfliesScalar(want, pl, inverse, 1, n)
				for i := range x {
					if math.Float64bits(real(x[i])) != math.Float64bits(real(want[i])) ||
						math.Float64bits(imag(x[i])) != math.Float64bits(imag(want[i])) {
						t.Fatalf("length %d, inverse %v: element %d is %v, and one at a time %v", n, inverse, i, x[i], want[i])
					}
				}
			}
		}
	}
}

// How long a transform of a patch's row takes, for holding the butterflies
// four lanes at a time against the same done one at a time: run it with and
// without GOEXPERIMENT=simd.
func BenchmarkFFT(b *testing.B) {
	for _, n := range []int{64, 256, 1024} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			x := make([]complex128, n)
			for i := range x {
				x[i] = complex(math.Sin(float64(i)), math.Cos(3*float64(i)))
			}
			for b.Loop() {
				FFT(x, false)
				FFT(x, true)
			}
		})
	}
}
