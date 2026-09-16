package atmos

import (
	"math"
	"testing"
)

// The most a tropical cyclone can blow over a sea at twenty-eight degrees is
// some seventy to ninety metres a second, and it grows with the sea (Emanuel,
// 1986; the observed maximum intensities are near it); over a sea too cool to
// feed one the most is little.
func TestTheWarmerSeaMakesTheDeeperStorm(t *testing.T) {
	v := stormWindOf(potentialDepth(28))
	if v < 60 || v > 95 {
		t.Errorf("a sea at 28 C makes a storm of %.0f m/s at most", v)
	}
	if !(potentialDepth(30) > potentialDepth(28) && potentialDepth(28) > potentialDepth(24)) {
		t.Error("the potential intensity does not grow with the sea")
	}
	if got := stormWindOf(stormDepthOf(40)); math.Abs(got-40) > 1e-9 {
		t.Errorf("a 40 m/s storm's depth reads back as %.3f m/s", got)
	}
}
