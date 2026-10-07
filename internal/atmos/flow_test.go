package atmos

import (
	"math"
	"testing"
)

// A chain solves its band exactly, open at its ends or joined round a ring:
// the preconditioner's rows are solved by it, and a row all the way round a
// parallel is a ring.
func TestAChainSolvesItsBand(t *testing.T) {
	seed := uint64(1)
	next := func() float64 {
		seed = seed*6364136223846793005 + 1442695040888963407
		return float64(seed>>11)/(1<<53) - 0.5
	}
	for _, ring := range []bool{false, true} {
		for _, n := range []int{1, 2, 5, 17} {
			if ring && n < 5 {
				continue
			}
			var a [5][]float64
			for d := range a {
				a[d] = make([]float64, n)
			}
			for p := range n {
				for d := range a {
					a[d][p] = next()
				}
				// The friction's and the stirring's diagonal, with the
				// planet's turning off it, against each other.
				a[2][p] = 6 + next()
				a[1][p] += 2
				a[3][p] -= 2
			}
			want := make([]float64, n)
			for p := range want {
				want[p] = next()
			}
			r := make([]float64, n)
			for p := range n {
				for d := -2; d <= 2; d++ {
					q := p + d
					if q < 0 || q >= n {
						if !ring {
							continue
						}
						q = (q + n) % n
					}
					r[p] += a[d+2][p] * want[q]
				}
			}
			cells := make([]int32, n)
			newChain(cells, a, ring).solve(r)
			for p := range n {
				if math.Abs(r[p]-want[p]) > 1e-12 {
					t.Fatalf("ring %v, %d cells: cell %d is %v, not %v", ring, n, p, r[p], want[p])
				}
			}
		}
	}
}

// GMRES solves what it is given, here a band with the planet's turning in it
// and nothing to precondition it, to what it was asked.
func TestGMRESSettles(t *testing.T) {
	const n = 50
	apply := func(x, out []float64) {
		for i := range n {
			s := 4 * x[i]
			if i > 0 {
				s -= 1.5 * x[i-1]
			}
			if i < n-1 {
				s -= 0.5 * x[i+1]
			}
			out[i] = s
		}
	}
	b := make([]float64, n)
	for i := range b {
		b[i] = math.Sin(float64(i))
	}
	x, done, res := gmres(b, apply, func(in, out []float64) { copy(out, in) }, 1e-10, 10, 200, nil)
	r := make([]float64, n)
	apply(x, r)
	for i := range r {
		r[i] -= b[i]
	}
	if res > 1e-10 || norm(r) > 1e-9*norm(b) {
		t.Errorf("after %d directions the residual is %.3g, said to be %.3g", done, norm(r)/norm(b), res)
	}
}
