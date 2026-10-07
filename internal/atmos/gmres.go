package atmos

import "math"

// gmres is x with A x = b, from nought, to settled of b, A being apply and
// M⁻¹ precondition, which it is preconditioned by on the right: GMRES (Saad
// and Schultz, 1986), started again every restart directions from where it
// got, and taking at most most directions. It gives the directions it took
// and the residual left, as a share of b. Every sum is taken in one order.
func gmres(b []float64, apply, precondition func(in, out []float64), settled float64, restart, most int) ([]float64, int, float64) {
	n := len(b)
	x := make([]float64, n)
	bn := norm(b)
	if bn == 0 {
		return x, 0, 0
	}
	m := restart
	v := make([][]float64, m+1)
	for k := range v {
		v[k] = make([]float64, n)
	}
	hess := make([][]float64, m+1)
	for k := range hess {
		hess[k] = make([]float64, m)
	}
	cs, sn, g := make([]float64, m), make([]float64, m), make([]float64, m+1)
	z, t, r := make([]float64, n), make([]float64, n), make([]float64, n)
	copy(r, b)
	done := 0
	for done < most {
		beta := norm(r)
		if beta <= settled*bn {
			break
		}
		for k := range v[0] {
			v[0][k] = r[k] / beta
		}
		clear(g)
		g[0] = beta
		k := 0
		for k < m && done < most {
			precondition(v[k], z)
			wv := v[k+1]
			apply(z, wv)
			for j := 0; j <= k; j++ {
				hj := dot(wv, v[j])
				hess[j][k] = hj
				for q := range wv {
					wv[q] -= hj * v[j][q]
				}
			}
			hk := norm(wv)
			hess[k+1][k] = hk
			if hk != 0 {
				for q := range wv {
					wv[q] /= hk
				}
			}
			for j := 0; j < k; j++ {
				a, c := hess[j][k], hess[j+1][k]
				hess[j][k] = cs[j]*a + sn[j]*c
				hess[j+1][k] = -sn[j]*a + cs[j]*c
			}
			a, c := hess[k][k], hess[k+1][k]
			d := math.Hypot(a, c)
			cs[k], sn[k] = a/d, c/d
			hess[k][k], hess[k+1][k] = d, 0
			g[k+1] = -sn[k] * g[k]
			g[k] = cs[k] * g[k]
			k++
			done++
			if math.Abs(g[k]) <= settled*bn || hk == 0 {
				break
			}
		}
		// The combination of the directions that leaves the least residual,
		// and back through the preconditioner to the unknowns.
		y := make([]float64, k)
		for j := k - 1; j >= 0; j-- {
			s := g[j]
			for q := j + 1; q < k; q++ {
				s -= hess[j][q] * y[q]
			}
			y[j] = s / hess[j][j]
		}
		clear(t)
		for j := range k {
			for q := range t {
				t[q] += y[j] * v[j][q]
			}
		}
		precondition(t, z)
		for q := range x {
			x[q] += z[q]
		}
		apply(x, r)
		for q := range r {
			r[q] = b[q] - r[q]
		}
	}
	return x, done, norm(r) / bn
}

func dot(a, b []float64) float64 {
	var s float64
	for i, x := range a {
		s += x * b[i]
	}
	return s
}

func norm(a []float64) float64 { return math.Sqrt(dot(a, a)) }
