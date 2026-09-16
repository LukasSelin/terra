package terra

import (
	"math"
	"testing"
)

// The potential the cosine solve finds on a valley has, over the faces between
// its cells, the Laplacian it was asked for: the gathering less its mean.
func TestTheCosineSolveHasTheLaplacianAskedFor(t *testing.T) {
	e := envOf(ridged(300).withAir())
	n := e.w * e.h
	div := make([]float64, n)
	var mean float64
	for i := range div {
		div[i] = math.Sin(float64(i)*0.37) * 1000
		mean += div[i] / float64(n)
	}
	chi := make([]float64, n)
	e.cosinePotential(chi, div, mean)
	cx, cn := e.dy/e.dx[0], e.dx[0]/e.dy
	for cy := 0; cy < e.h; cy++ {
		for c := 0; c < e.w; c++ {
			i := cy*e.w + c
			var lap float64
			if c > 0 {
				lap += cx * (chi[i-1] - chi[i])
			}
			if c+1 < e.w {
				lap += cx * (chi[i+1] - chi[i])
			}
			if cy > 0 {
				lap += cn * (chi[i-e.w] - chi[i])
			}
			if cy+1 < e.h {
				lap += cn * (chi[i+e.w] - chi[i])
			}
			if want := div[i] - mean; math.Abs(lap-want) > 1e-6*1000 {
				t.Fatalf("cell %d, %d: the Laplacian is %v, want %v", c, cy, lap, want)
			}
		}
	}
}
