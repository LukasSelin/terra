package atmos

import (
	"math"
	"testing"
)

// gillGlobe is the air's lattice round a planet of w by h cells, with nothing
// on it: the rows' latitudes, breadths and turning, all a heating's answer
// is worked out from.
func gillGlobe(w, h int) *Env {
	e := &Env{W: w, H: h, Wrap: true}
	e.lat, e.Dx, e.f = make([]float64, h), make([]float64, h), make([]float64, h)
	e.Dy = math.Pi * planetRadius / float64(h)
	for cy := range h {
		lat := 90 - (float64(cy)+0.5)*180/float64(h)
		e.lat[cy] = lat
		e.Dx[cy] = 2 * math.Pi * planetRadius * math.Cos(lat*math.Pi/180) / float64(w)
		e.f[cy] = 2 * omega * math.Sin(lat*math.Pi/180)
	}
	return e
}

// A heating on the equator draws Gill's (1980) answer: a low over it, the
// air running into it from the east along the equator, the Kelvin wave's
// easterlies, a long way, and from the west, the Rossby waves' westerlies,
// a shorter way; and it is the same answer round a parallel whose cells the
// fast transform takes as round one whose cells it does not.
func TestAHeatingDrawsGillsWinds(t *testing.T) {
	for _, w := range []int{128, 96} { // by the transform, and by the sum
		e := gillGlobe(w, 64)
		q := make([]float64, w*e.H)
		mid := w / 2
		for cy := range e.H {
			for cx := range w {
				dx := float64(cx-mid) * e.Dx[cy] / 1e6
				dy := e.lat[cy] * 111.2e3 / 1e6
				q[cy*w+cx] = 0.01 * math.Exp(-dx*dx/(2*1.5*1.5)-dy*dy/(2*0.6*0.6))
			}
		}
		phi, u, _ := e.gill(q)
		eq := e.H / 2 // the row just south of the equator
		low := phi[eq*w+mid]
		east := u[eq*w+(mid+w/8)]
		west := u[eq*w+(mid-w/8)]
		t.Logf("%d cells round: φ over the heating %.1f m²/s², the wind an eighth of the way round east of it %+.2f m/s, west of it %+.2f m/s", w, low, east, west)
		if low >= 0 || east >= 0 || west <= 0 {
			t.Errorf("%d cells round: φ %.1f over the heating, the wind east of it %+.2f and west %+.2f: no low, or no Kelvin easterlies, or no Rossby westerlies", w, low, east, west)
		}
		if -east <= west {
			t.Errorf("%d cells round: the easterlies %.2f are no stronger than the westerlies %.2f, where the Kelvin wave reaches further", w, -east, west)
		}
	}
}
