package terra

import (
	"math"
	"math/cmplx"
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
	fft(y, false)
	for k := range y {
		want := 0.0
		if k == 5 || k == n-5 {
			want = n / 2
		}
		if cmplx.Abs(y[k]-complex(want, 0)) > 1e-9 {
			t.Fatalf("mode %d of a wave of five is %v, want %v", k, y[k], want)
		}
	}
	fft(y, true)
	for i := range x {
		if cmplx.Abs(y[i]-x[i]) > 1e-12 {
			t.Fatalf("element %d came back %v, was %v", i, y[i], x[i])
		}
	}
}
