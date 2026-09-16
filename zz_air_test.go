package terra

import (
	"math"
	"testing"
)

// A dry continent's day swings further than a humid coast's, and the air over
// it takes up more for it: Hargreaves's evaporation goes as the root of the
// range, six degrees on a humid coast and sixteen in a dry interior.
func TestADryInteriorsDaySwingsWider(t *testing.T) {
	coast, desert := Diurnal(0, 0.5), Diurnal(1, 10)
	if math.Abs(coast-math.Sqrt(0.6)) > 1e-9 || math.Abs(desert-math.Sqrt(1.6)) > 1e-9 {
		t.Errorf("a humid coast evaporates %.3f of the table and a dry interior %.3f", coast, desert)
	}
}
