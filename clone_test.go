package terra

import (
	"math"
	"testing"
)

// A clone reads as its original does: the same evaporation, runoff, year,
// soil and soil climate on every tile. Clone once dropped dayRange, and the
// clone's evaporation - and the lime, salt and leaching laid off it - read
// otherwise on a few per cent of a globe's tiles.
func TestACloneReadsAsItsOriginal(t *testing.T) {
	worlds := []struct {
		name string
		g    *Grid
	}{{"valley 1", yardWorld("valley", 1, DefaultTerms())}}
	if !testing.Short() {
		worlds = append(worlds, struct {
			name string
			g    *Grid
		}{"small globe 1", yardWorld("small", 1, smallGlobe())})
	}
	same := func(a, b float64) bool { return math.Float64bits(a) == math.Float64bits(b) }
	for _, w := range worlds {
		g := w.g
		c := g.Clone()
		if len(c.dayRange) != len(g.dayRange) {
			t.Errorf("%s: the clone has %d day ranges and the original %d", w.name, len(c.dayRange), len(g.dayRange))
		}
		wrong := 0
		for i := range g.Tiles {
			p := g.PosOf(i)
			gm, gc, gw := g.YearAt(i)
			cm, cc, cw := c.YearAt(i)
			gp, cp := g.pedoClimateOf(i), c.pedoClimateOf(i)
			ok := same(g.pet(i), c.pet(i)) && same(g.Runoff(i), c.Runoff(i)) &&
				same(gm, cm) && same(gc, cc) && same(gw, cw) &&
				same(g.SoilAt(p), c.SoilAt(p)) &&
				same(gp.water, cp.water) && same(gp.rain, cp.rain) && same(gp.wetness, cp.wetness) &&
				same(gp.weathering, cp.weathering) && same(gp.temp, cp.temp) && same(gp.sodden, cp.sodden) &&
				gp.frozen == cp.frozen &&
				same(g.FloorAge(i), c.FloorAge(i))
			if !ok {
				if wrong == 0 {
					t.Errorf("%s: tile %v reads pet %g, runoff %g, climate %+v on the original and pet %g, runoff %g, climate %+v on the clone",
						w.name, p, g.pet(i), g.Runoff(i), gp, c.pet(i), c.Runoff(i), cp)
				}
				wrong++
			}
		}
		if wrong > 0 {
			t.Errorf("%s: %d of %d tiles read otherwise on the clone", w.name, wrong, len(g.Tiles))
		}
	}
}
