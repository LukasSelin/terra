package terra

import (
	"math"
	"testing"
)

// The potential the cosine solve finds on a valley has, over the faces between
// its cells, the Laplacian it was asked for: the gathering less its mean.
func TestTheCosineSolveHasTheLaplacianAskedFor(t *testing.T) {
	e := envOf(ridged(300).withAir())
	n := e.W * e.H
	div := make([]float64, n)
	var mean float64
	for i := range div {
		div[i] = math.Sin(float64(i)*0.37) * 1000
		mean += div[i] / float64(n)
	}
	chi := make([]float64, n)
	e.CosinePotential(chi, div, mean)
	cx, cn := e.Dy/e.Dx[0], e.Dx[0]/e.Dy
	for cy := 0; cy < e.H; cy++ {
		for c := 0; c < e.W; c++ {
			i := cy*e.W + c
			var lap float64
			if c > 0 {
				lap += cx * (chi[i-1] - chi[i])
			}
			if c+1 < e.W {
				lap += cx * (chi[i+1] - chi[i])
			}
			if cy > 0 {
				lap += cn * (chi[i-e.W] - chi[i])
			}
			if cy+1 < e.H {
				lap += cn * (chi[i+e.W] - chi[i])
			}
			if want := div[i] - mean; math.Abs(lap-want) > 1e-6*1000 {
				t.Fatalf("cell %d, %d: the Laplacian is %v, want %v", c, cy, lap, want)
			}
		}
	}
}
