package atmos

import (
	"math"
	"math/cmplx"

	"github.com/LukasSelin/terra/internal/kernel"
)

// The tropical atmosphere's answer to the heat its rain lets go of.
//
// Over the warmest water of the tropics the air rises in deep convection,
// and the heat of its rain warms the whole column and draws the air in near
// the ground. How the air answers is Gill's (1980): the first baroclinic mode
// of the column, a shallow layer of air damped as the cumulus mix it, under
// the planet's turning, at its steady state. East of the heating a Kelvin
// wave carries easterlies a long way along the equator; west of it the
// Rossby waves carry westerlies a shorter way, in two lobes either side of
// it; and under it the pressure is low. It is the atmosphere of the coupled
// models of the tropical Pacific (Zebiak and Cane, 1987), and its constants
// are theirs.
const (
	// gillSpeed is c, how fast the mode's waves go, metres a second; and
	// gillDamp ε, how fast, per second, its wind and its warmth are damped:
	// in two days.
	gillSpeed = 30.0
	gillDamp  = 1 / (2 * 86400.0)
	// gillReach is how far from the equator, in degrees, the answer is
	// worked out: past it the heating is nothing (tropicShare) and the
	// answer has died away.
	gillReach = 50.0
	// rainSlope is how much more it rains, in millimetres a day, over a sea
	// a degree warmer than the rest of its row: the rain of the tropical
	// oceans climbs some three millimetres a day a degree over the warmest
	// water (Graham and Barnett, 1987; Back and Bretherton, 2009).
	rainSlope = 3.0
	// heatAir is the heat a kilogram of air takes a degree at constant
	// pressure, J/(kg K); columnMass the air over a square metre, kg.
	heatAir    = 1004.0
	columnMass = 1e4
	// convectionTop is the pressure, in hPa, the deep convection's heating
	// reaches, near the tropopause.
	convectionTop = 150.0
)

// rainHeat is the heating of Gill's equations, m² s⁻³, for each degree the
// sea stands over the rest of its row: rainSlope's rain, its latent heat
// spread over the column (L P / (c_p M), kelvin a second), as the first
// baroclinic mode feels it, the thickness R ln(p_s/p_t) of the column's
// warming shared between its lower and upper halves.
var rainHeat = rainSlope / 86400 * latentHeat / (heatAir * columnMass) *
	dryGas * math.Log(beltMean/convectionTop) / 2

// gill is Gill's answer to a heating q, m² s⁻³ on each cell: the
// geopotential phi of the layer near the ground, m² s⁻², and its wind u, v,
// metres a second. The equations are
//
//	ε u − f v = −∂φ/∂x,  ε v + f u = −∂φ/∂y,  ε φ + c² ∇·(u, v) = −q
//
// The wind is eliminated, u = −(a φx + b φy), v = b φx − a φy with
// a = ε/(ε²+f²) and b = f/(ε²+f²), which leaves one equation for φ whose
// coefficients change from row to row only:
//
//	ε φ − c² [a φxx − (∂y b) φx + (1/dx) ∂y(dx a ∂y φ)] = −q
//
// Along the rows it is the same at every column, so each wavenumber round
// the parallels is a three-banded system down the rows of its own, solved
// exactly (Thomas's algorithm): no iterating, and nothing that depends on
// the goroutines. The rows past gillReach are held still, with nothing
// crossing into them. It is for a globe only.
func (e *Env) gill(q []float64) (phi, u, v []float64) {
	n := e.W * e.H
	phi, u, v = make([]float64, n), make([]float64, n), make([]float64, n)
	lo, hi := -1, -1
	for cy := 0; cy < e.H; cy++ {
		if math.Abs(e.lat[cy]) < gillReach {
			if lo < 0 {
				lo = cy
			}
			hi = cy
		}
	}
	if lo < 0 || !e.Wrap {
		return
	}
	rows, w := hi-lo+1, e.W
	eps, c2 := gillDamp, gillSpeed*gillSpeed
	a, b := make([]float64, e.H), make([]float64, e.H)
	for cy := range e.H {
		d := eps*eps + e.f[cy]*e.f[cy]
		a[cy], b[cy] = eps/d, e.f[cy]/d
	}
	// The faces between the rows: dx a there, and nought past the band.
	face := make([]float64, rows+1)
	for r := 1; r < rows; r++ {
		cy := lo + r
		face[r] = (e.Dx[cy]*a[cy] + e.Dx[cy-1]*a[cy-1]) / 2
	}
	// The symbols of the second and the centred first difference along a
	// row, by transforming them applied to a single cell, so that they
	// agree with whatever convention the transform keeps.
	second, first := make([]complex128, w), make([]complex128, w)
	second[0], second[1], second[w-1] = -2, 1, 1
	first[1], first[w-1] = -0.5, 0.5
	transform(second, false)
	transform(first, false)

	spec := make([][]complex128, rows)
	for r := range spec {
		cy := lo + r
		spec[r] = make([]complex128, w)
		for cx := range w {
			spec[r][cx] = complex(-q[cy*w+cx], 0)
		}
		transform(spec[r], false)
	}
	dy2 := e.Dy * e.Dy
	sub, diag, sup := make([]complex128, rows), make([]complex128, rows), make([]complex128, rows)
	rhs := make([]complex128, rows)
	for k := range w {
		for r := range rows {
			cy := lo + r
			dx := e.Dx[cy]
			// ∂y b, north being the row before.
			var db float64
			switch {
			case rows == 1:
			case r == 0:
				db = (b[cy] - b[cy+1]) / e.Dy
			case r == rows-1:
				db = (b[cy-1] - b[cy]) / e.Dy
			default:
				db = (b[cy-1] - b[cy+1]) / (2 * e.Dy)
			}
			along := complex(a[cy]/(dx*dx), 0)*second[k] - complex(db/dx, 0)*first[k]
			north, south := face[r]/(dx*dy2), face[r+1]/(dx*dy2)
			diag[r] = complex(eps, 0) - complex(c2, 0)*(along-complex(north+south, 0))
			sub[r] = complex(-c2*north, 0) // the row before
			sup[r] = complex(-c2*south, 0) // the row after
			rhs[r] = spec[r][k]
		}
		thomas(sub, diag, sup, rhs)
		for r := range rows {
			spec[r][k] = rhs[r]
		}
	}
	for r := range rows {
		transform(spec[r], true)
		cy := lo + r
		for cx := range w {
			phi[cy*w+cx] = real(spec[r][cx])
		}
	}
	for r := range rows {
		cy := lo + r
		for cx := range w {
			i := cy*w + cx
			px := (phi[cy*w+(cx+1)%w] - phi[cy*w+(cx+w-1)%w]) / (2 * e.Dx[cy])
			nn, ss := phi[i], phi[i]
			if r > 0 {
				nn = phi[i-w]
			}
			if r < rows-1 {
				ss = phi[i+w]
			}
			py := (nn - ss) / (2 * e.Dy)
			u[i] = -(a[cy]*px + b[cy]*py)
			v[i] = b[cy]*px - a[cy]*py
		}
	}
	return phi, u, v
}

// thomas solves the three-banded system sub, diag, sup in place of rhs:
// sub[r] multiplies the row before r, sup[r] the row after.
func thomas(sub, diag, sup, rhs []complex128) {
	n := len(rhs)
	c := make([]complex128, n)
	d := diag[0]
	c[0] = sup[0] / d
	rhs[0] /= d
	for r := 1; r < n; r++ {
		d = diag[r] - sub[r]*c[r-1]
		c[r] = sup[r] / d
		rhs[r] = (rhs[r] - sub[r]*rhs[r-1]) / d
	}
	for r := n - 2; r >= 0; r-- {
		rhs[r] -= c[r] * rhs[r+1]
	}
}

// transform is the discrete Fourier transform of x in place, or its
// inverse: kernel.FFT's where the length is a power of two, and the sum
// written out where it is not.
func transform(x []complex128, inverse bool) {
	n := len(x)
	if kernel.PowerOfTwo(n) {
		kernel.FFT(x, inverse)
		return
	}
	sign := -1.0
	if inverse {
		sign = 1
	}
	out := make([]complex128, n)
	for k := range n {
		var s complex128
		for j := range n {
			s += x[j] * cmplx.Exp(complex(0, sign*2*math.Pi*float64(j*k%n)/float64(n)))
		}
		out[k] = s
		if inverse {
			out[k] /= complex(float64(n), 0)
		}
	}
	copy(x, out)
}
