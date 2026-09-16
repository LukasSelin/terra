package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
)

// The climate of a tile, as Köppen named it.
//
// A biome is what the weather makes of a tile: its year's warmest and
// coldest month and its mean at its latitude and height, against the rain
// that falls on it and when in the year that rain falls, the way Köppen
// divides the world (Köppen, 1936; the thresholds as Peel, Finlayson and
// McMahon, 2007, give them, with Köppen's own three degrees under freezing
// between C and D). It was cmd/overview's reading, for looking at; it is
// the land's now because the features join the tiles by it - see
// ClimateRegion - and it computes what it computed there.

// Koppen is the Köppen–Geiger type of the dry ground at p: "BWh", "Cfb",
// "ET" and the rest.
//
// The year's months are read off the tile's year as a sine, and its rain as
// one too: a share w of the year's rain falling in the warmer half is a monthly
// rain of P/12 (1 + a cos θ) with a = π(w - ½), θ the month's distance from
// midsummer. That is enough to tell a summer rain from a winter one and a dry
// season from none, which is all Köppen's second letters ask.
func (g *Grid) Koppen(p geom.Pos) string {
	c, n := g.koppenCode(g.Index(p))
	return string(c[:n])
}

// koppenGroup is the first letter of the type at tile i: A, B, C, D or E.
func (g *Grid) koppenGroup(i int) byte {
	c, _ := g.koppenCode(i)
	return c[0]
}

// koppenCode is Koppen at tile i without the string.
func (g *Grid) koppenCode(i int) ([3]byte, int) {
	mean, cold, hot := g.YearAt(i)
	return koppenCode(mean, cold, hot, g.Rain(i), g.RainWarm(i), g.Barren(g.PosOf(i)))
}

// KoppenOf is the Köppen–Geiger type of a year with the given mean, coldest
// and warmest month, rain, and share of that rain in the warmer half, on
// ground under ice or not.
func KoppenOf(mean, cold, hot, rain, warm float64, ice bool) string {
	c, n := koppenCode(mean, cold, hot, rain, warm, ice)
	return string(c[:n])
}

// koppenCode is KoppenOf as the letters and how many of them, so that a pass
// over every tile allocates nothing.
func koppenCode(mean, cold, hot, rain, warm float64, ice bool) (code [3]byte, n int) {
	if hot < 10 {
		if hot < 0 || ice {
			return [3]byte{'E', 'F'}, 2
		}
		return [3]byte{'E', 'T'}, 2
	}
	a := math.Max(-1, math.Min(1, math.Pi*(warm-0.5)))
	sDry, sWet, wDry, wWet := math.Inf(1), 0.0, math.Inf(1), 0.0
	for k := range 12 {
		th := (float64(k)+0.5)*math.Pi/6 - math.Pi
		m := rain / 12 * (1 + a*math.Cos(th))
		if math.Abs(th) < math.Pi/2 {
			sDry, sWet = math.Min(sDry, m), math.Max(sWet, m)
		} else {
			wDry, wWet = math.Min(wDry, m), math.Max(wWet, m)
		}
	}
	dry := math.Min(sDry, wDry)

	// The line between dry and not moves with the warmth, because warm air
	// takes more of the rain back, and with when the rain falls, because rain
	// in the summer is taken back sooner than rain in the winter. Under half
	// the line is desert, and under the line steppe.
	threshold := 20*mean + 140
	switch {
	case warm >= 0.7:
		threshold = 20*mean + 280
	case warm <= 0.3:
		threshold = 20 * mean
	}
	if rain < threshold {
		code = [3]byte{'B', 'S', 'k'}
		if rain < threshold/2 {
			code[1] = 'W'
		}
		if mean >= 18 {
			code[2] = 'h'
		}
		return code, 3
	}

	if cold >= 18 {
		switch {
		case dry >= 60:
			return [3]byte{'A', 'f'}, 2
		case dry >= 100-rain/25:
			return [3]byte{'A', 'm'}, 2
		}
		return [3]byte{'A', 'w'}, 2
	}
	code[0] = 'C'
	if cold <= -3 {
		code[0] = 'D'
	}
	code[1] = 'f'
	switch {
	case sDry < 40 && sDry < wWet/3:
		code[1] = 's'
	case wDry < sWet/10:
		code[1] = 'w'
	}
	code[2] = 'c'
	switch {
	case hot >= 22:
		code[2] = 'a'
	case warmMonths(mean, hot) >= 4:
		code[2] = 'b'
	}
	return code, 3
}

// warmMonths is how many months of a sinusoidal year with the given mean and
// warmest month stand at ten degrees or more.
func warmMonths(mean, hot float64) int {
	amp := (hot - mean) / (math.Sin(math.Pi/12) / (math.Pi / 12))
	n := 0
	for k := range 12 {
		if mean+amp*math.Cos((float64(k)+0.5)*math.Pi/6-math.Pi) >= 10 {
			n++
		}
	}
	return n
}
