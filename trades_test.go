package terra

import (
	"math"
	"testing"

	"github.com/LukasSelin/terra/internal/atmos"
)

// windPhases are the wind's phases of the year, in the order Winds keeps
// them: the north's winter, its spring, its summer and its autumn.
var windPhases = [atmos.Phases]string{"January", "April", "July", "October"}

// seaRows is the zonal mean over the sea of each row of g's air cells of the
// wind toward the east in each phase of the year and on the year's mean,
// with each row's latitude. A row with no sea reads NaN.
func seaRows(g *Grid) (lat []float64, phase [atmos.Phases][]float64, year []float64) {
	e := g.winds.Env
	lat = make([]float64, e.H)
	year = make([]float64, e.H)
	for k := range phase {
		phase[k] = make([]float64, e.H)
	}
	for cy := range lat {
		for y := cy * e.Cell; y < (cy+1)*e.Cell; y++ {
			lat[cy] += g.air.Lat[y]
		}
		lat[cy] /= float64(e.Cell)
		var n float64
		var s [atmos.Phases]float64
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			if e.Sea[i] <= 0.5 {
				continue
			}
			for k := range s {
				s[k] += float64(g.winds.U[k][i])
			}
			n++
		}
		year[cy] = 0
		for k := range s {
			phase[k][cy] = s[k] / n
			year[cy] += s[k] / n / atmos.Phases
		}
		if n == 0 {
			year[cy] = math.NaN()
		}
	}
	return lat, phase, year
}

// tradesPeak is the latitude, degrees, at which the easterlies of u are
// strongest within thirty-five degrees of the equator on the side sign is
// (+1 north, -1 south), and how strong: the row at the least u, and the
// parabola through it and its neighbours for where between them.
func tradesPeak(lat, u []float64, sign float64) (at, speed float64) {
	best := -1
	for cy, l := range lat {
		if l*sign <= 0 || math.Abs(l) > 35 || math.IsNaN(u[cy]) {
			continue
		}
		if best < 0 || u[cy] < u[best] {
			best = cy
		}
	}
	if best < 0 {
		return math.NaN(), math.NaN()
	}
	at, speed = lat[best], u[best]
	if best > 0 && best < len(lat)-1 && !math.IsNaN(u[best-1]) && !math.IsNaN(u[best+1]) {
		lo, mid, hi := u[best-1], u[best], u[best+1]
		if d := lo - 2*mid + hi; d > 0 {
			at += 0.5 * (lo - hi) / d * (lat[best+1] - lat[best])
		}
	}
	return at, speed
}

// equatorialMean is the year's mean wind toward the east over the sea within
// reach degrees of the equator, each row weighted alike.
func equatorialMean(lat, year []float64, reach float64) float64 {
	var s, n float64
	for cy, l := range lat {
		if math.Abs(l) < reach && !math.IsNaN(year[cy]) {
			s, n = s+year[cy], n+1
		}
	}
	return s / n
}

// logTrades writes where the trades of g are strongest in each phase and on
// the year's mean, the year's mean wind on the equator, and the ITCZ.
func logTrades(t *testing.T, name string, g *Grid) (north, south float64) {
	t.Helper()
	lat, phase, year := seaRows(g)
	for k := range phase {
		n, nu := tradesPeak(lat, phase[k], 1)
		s, su := tradesPeak(lat, phase[k], -1)
		t.Logf("%s, %s: ITCZ %+.1f; the trades strongest at %.1fN (%.1f m/s) and %.1fS (%.1f m/s)",
			name, windPhases[k], g.winds.ITCZ([atmos.Phases]float64{-1, 0, 1, 0}[k]), n, nu, -s, su)
	}
	n, nu := tradesPeak(lat, year, 1)
	s, su := tradesPeak(lat, year, -1)
	var itcz float64
	for _, sinT := range []float64{-1, 0, 1, 0} {
		itcz += g.winds.ITCZ(sinT) / 4
	}
	t.Logf("%s, the year: ITCZ %+.2f; the trades strongest at %.1fN (%.1f m/s) and %.1fS (%.1f m/s); over the sea within 5 degrees of the equator %+.2f m/s, within 2 %+.2f",
		name, itcz, n, nu, -s, su, equatorialMean(lat, year, 5), equatorialMean(lat, year, 2))
	return n, -s
}

// The trades blow hardest in the inner half of the Hadley cell, some fifteen
// degrees from the equator in either hemisphere, well inside the cell's edge
// at thirty (Peixoto and Oort, 1992); Held and Hou's (1980) cell, whose
// surface winds balance the angular momentum the air aloft carries poleward,
// has its strongest easterlies at 0.43 of the way to its edge. They used to
// blow hardest at twenty-four degrees here, under the steepest of the rise
// to the subtropical highs (#88; see atmos's hadley). On a planet of sea the
// year's mean is held to twelve to eighteen degrees either side. Where the
// trades of two oceans between two continents blow hardest is logged with
// it, and the ITCZ, and the year's mean wind on the equator.
func TestTheTradesBlowHardestInTheCellsInnerHalf(t *testing.T) {
	g := oceanGlobe(256, 128)
	g.weather()
	north, south := logTrades(t, "the planet of sea", g)
	if !(north >= 12 && north <= 18) || !(south >= 12 && south <= 18) {
		t.Errorf("the trades blow hardest at %.1fN and %.1fS on the year's mean, not within 12-18 degrees", north, south)
	}
	two := twoOceans()
	two.weather()
	logTrades(t, "two oceans", two)
}

// The same on the yardsticks' globe, where the land has its say: logged, for
// the readings the trades' issue (#88) asks for.
func TestTheGlobesTrades(t *testing.T) {
	if testing.Short() {
		t.Skip("needs the full globe")
	}
	logTrades(t, "the globe", yardWorld("globe", 1, GlobeTerms()))
}
