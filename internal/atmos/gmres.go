package atmos

import "math"

// gmres is x with A x = b, from nought, to settled of b, A being apply and
// M⁻¹ precondition, which it is preconditioned by on the right: GMRES (Saad
// and Schultz, 1986), started again every restart directions from where it
// got, and taking at most most directions. It gives the directions it took
// and the residual left, as a share of b. Every sum is taken in one order.
//
// room is where its vectors are kept: gmresRoom(restart) of them, each as
// long as b and all nought, the first of them x. Where it is nil they are
// made.
func gmres(b []float64, apply, precondition func(in, out []float64), settled float64, restart, most int, room [][]float64) ([]float64, int, float64) {
	return gmresOn(b, apply, precondition, settled, restart, most, room, nil)
}

// gmresOn is gmres over only the stretches of the unknowns in spans, each
// [lo, hi), in order; nil is all of them. It is for equations some of whose
// unknowns are their own right-hand side and nought, as the land's are in
// the sea's two layers (slab.go): b is nought outside the spans, and apply
// and precondition leave nought outside them what is nought there, so that
// every vector it works with is nought there too, and every sum it takes
// over them adds to what it took over the spans only noughts. It is the same
// to the bit as gmres over all of them, and works only on the sea.
//
// The Gram-Schmidt of each direction against those before is modified
// Gram-Schmidt, taken in one pass a direction (orthogonalize): each
// direction's part is taken off the new one in the same pass that takes the
// next's part of it, and the last pass its length. Each of those sums is
// taken in the order and the form it was in a pass of its own, so that the
// answer is the same to the bit; but the sum's chain of additions, which
// is what a pass waits on, takes the subtraction with it.
func gmresOn(b []float64, apply, precondition func(in, out []float64), settled float64, restart, most int, room [][]float64, spans [][2]int) ([]float64, int, float64) {
	n := len(b)
	m := restart
	if room == nil {
		room = make([][]float64, gmresRoom(m))
		for k := range room {
			room[k] = make([]float64, n)
		}
	}
	if spans == nil {
		spans = [][2]int{{0, n}}
	}
	x := room[0]
	bn := math.Sqrt(dotOn(b, b, spans))
	if bn == 0 {
		return x, 0, 0
	}
	v := room[1 : m+2]
	hess := make([][]float64, m+1)
	for k := range hess {
		hess[k] = make([]float64, m)
	}
	cs, sn, g := make([]float64, m), make([]float64, m), make([]float64, m+1)
	z, t, r := room[m+2], room[m+3], room[m+4]
	for _, sp := range spans {
		copy(r[sp[0]:sp[1]], b[sp[0]:sp[1]])
	}
	done := 0
	for done < most {
		beta := math.Sqrt(dotOn(r, r, spans))
		if beta <= settled*bn {
			break
		}
		for _, sp := range spans {
			v0, rs := v[0][sp[0]:sp[1]], r[sp[0]:sp[1]]
			for k := range v0 {
				v0[k] = rs[k] / beta
			}
		}
		clear(g)
		g[0] = beta
		k := 0
		for k < m && done < most {
			precondition(v[k], z)
			wv := v[k+1]
			apply(z, wv)
			hk := orthogonalize(wv, v[:k+1], hess, k, spans)
			hess[k+1][k] = hk
			if hk != 0 {
				for _, sp := range spans {
					ws := wv[sp[0]:sp[1]]
					for q := range ws {
						ws[q] /= hk
					}
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
		for _, sp := range spans {
			ts := t[sp[0]:sp[1]]
			clear(ts)
			for j := range k {
				vj := v[j][sp[0]:sp[1]]
				for q := range ts {
					ts[q] += y[j] * vj[q]
				}
			}
		}
		precondition(t, z)
		for _, sp := range spans {
			xs, zs := x[sp[0]:sp[1]], z[sp[0]:sp[1]]
			for q := range xs {
				xs[q] += zs[q]
			}
		}
		apply(x, r)
		for _, sp := range spans {
			rs, bs := r[sp[0]:sp[1]], b[sp[0]:sp[1]]
			for q := range rs {
				rs[q] = bs[q] - rs[q]
			}
		}
	}
	return x, done, math.Sqrt(dotOn(r, r, spans)) / bn
}

// orthogonalize takes from wv its part along each of the directions v, by
// modified Gram-Schmidt, writing each part to hess[j][k], and is the length
// of what is left. The part along v[j+1] is summed in the same pass that
// takes v[j]'s off, from what that pass has just written, and the length in
// the pass that takes the last off: each is the sum dot would take, term for
// term in the same order, so this is the same to the bit as a pass for each
// sum and each subtraction.
func orthogonalize(wv []float64, v [][]float64, hess [][]float64, k int, spans [][2]int) float64 {
	h := dotOn(wv, v[0], spans)
	for j := 0; j <= k; j++ {
		hess[j][k] = h
		var s float64
		for _, sp := range spans {
			ws, vj := wv[sp[0]:sp[1]], v[j][sp[0]:sp[1]]
			vj = vj[:len(ws)]
			if j < k {
				vn := v[j+1][sp[0]:sp[1]]
				vn = vn[:len(ws)]
				for q := range ws {
					ws[q] -= h * vj[q]
					x := ws[q]
					s += x * vn[q]
				}
			} else {
				for q := range ws {
					ws[q] -= h * vj[q]
					x := ws[q]
					s += x * x
				}
			}
		}
		h = s
	}
	return math.Sqrt(h)
}

// dotOn is dot over the spans, in order.
func dotOn(a, b []float64, spans [][2]int) float64 {
	var s float64
	for _, sp := range spans {
		bs := b[sp[0]:sp[1]]
		for i, x := range a[sp[0]:sp[1]] {
			s += x * bs[i]
		}
	}
	return s
}

// gmresRoom is how many vectors gmres keeps when it starts again every
// restart directions: the answer, the directions and one more, and three for
// its working.
func gmresRoom(restart int) int { return restart + 5 }

func dot(a, b []float64) float64 {
	var s float64
	for i, x := range a {
		s += x * b[i]
	}
	return s
}

func norm(a []float64) float64 { return math.Sqrt(dot(a, a)) }
