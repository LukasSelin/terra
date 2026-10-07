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

// GMRES over only the stretches of its unknowns that are not their own
// nought (gmresOn) is GMRES over all of them, to the bit, where the rest
// are: as the land's are in the sea's two layers.
func TestGMRESOverTheSeaIsGMRESOverAll(t *testing.T) {
	const n = 64
	land := func(i int) bool { return i%16 < 3 || (i >= 40 && i < 47) }
	apply := func(x, out []float64) {
		for i := range n {
			if land(i) {
				out[i] = x[i]
				continue
			}
			s := 4 * x[i]
			if i > 0 && !land(i-1) {
				s -= 1.5 * x[i-1]
			}
			if i < n-1 && !land(i+1) {
				s -= 0.5 * x[i+1]
			}
			out[i] = s
		}
	}
	precondition := func(in, out []float64) {
		for i := range n {
			out[i] = in[i] / 3
		}
	}
	b := make([]float64, n)
	var spans [][2]int
	lo := -1
	for i := range n {
		if !land(i) {
			b[i] = math.Sin(float64(i))
			if lo < 0 {
				lo = i
			}
		} else if lo >= 0 {
			spans, lo = append(spans, [2]int{lo, i}), -1
		}
	}
	if lo >= 0 {
		spans = append(spans, [2]int{lo, n})
	}
	all, d1, r1 := gmres(b, apply, precondition, 1e-12, 7, 100, nil)
	sea, d2, r2 := gmresOn(b, apply, precondition, 1e-12, 7, 100, nil, spans)
	if d1 != d2 || math.Float64bits(r1) != math.Float64bits(r2) {
		t.Fatalf("over all: %d directions, %g left; over the sea: %d, %g", d1, r1, d2, r2)
	}
	for i := range all {
		if math.Float64bits(all[i]) != math.Float64bits(sea[i]) {
			t.Fatalf("unknown %d: %v over all and %v over the sea", i, all[i], sea[i])
		}
	}
}
