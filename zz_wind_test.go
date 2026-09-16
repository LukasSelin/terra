package terra

import (
	"math"
	"testing"
)

// A kilometre of air a degree warmer weighs some four tenths of an hPa less
// (the hypsometric equation: p g H / (R_d T²)), and the monsoon above is
// drawn by no more than the three and a half kilometres a continent's summer
// heats: the gain is the physics', not a figure raised until the wind turned.
func TestAWarmColumnWeighsWhatTheHypsometricEquationSays(t *testing.T) {
	if got := hypsometric(1013.25, 15, 1000); math.Abs(got-0.4165) > 0.005 {
		t.Errorf("a kilometre a degree warmer at 15 C weighs %.4f hPa less", got)
	}
	if most := hypsometric(beltMean, 30, boundaryWarm); most >= 1.5 {
		t.Errorf("a summer continent's layer at 30 C gains %.2f hPa a degree", most)
	}
}
