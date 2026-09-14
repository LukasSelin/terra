package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
)

// How much soil there is.
//
// The ground was a height and a mixture, and how much of the mixture there
// was on the rock was one figure for the whole map: three metres, everywhere,
// against which every age's loss or gain of ground was read as a share of
// what the ground could grow. A ridge and a hollow had the same soil under
// them, and the only way to lose it was for the height to go down.
//
// Real soil is a thickness, and it is kept. The rock underneath turns into
// it at a pace that slows as it deepens - the more soil there is over the
// rock, the less of the weather reaches the rock - and the water, the creep
// and the slides take it off the top and put it down somewhere else. What is
// on a tile is what was made there, less what went, plus what came. So a
// ridge that is losing ground fast holds a skin of it, a hollow that
// everything above it creeps into holds metres, and a hillside the plough
// has bared is down to rock while the valley under it is buried in what came
// off it. None of that is set; it is what the making and the moving leave.

// SoilMaking and SoilScale are Heimsath and others' (1997) soil production
// function, dH/dt = P0·e^(−H/H0): bare rock turns into soil at a tenth to
// three tenths of a millimetre a year, and every half metre or so of soil over
// it cuts that by e. The same form has held since on ranges from California to
// south-eastern Australia (Heimsath and others 2012). The middle of the range
// is taken for P0, and H0 at half a metre.
//
// SoilMaking is the pace at the map's middling climate. How much faster warm
// wet ground makes it is weathering.
const (
	SoilMaking = 0.0002 // m/yr off bare rock
	SoilScale  = 0.5    // m of soil over which the making falls by e
)

// soilDeepest is the least lowering, in metres a year, any ground has to be
// making soil against, and so what bounds how deep it gets where nothing is
// taking it away: Portenga and Bierman's (2011) median for bare outcrop, 5.4 m
// a million years. A hollow that the creep only ever fills comes to
// SoilScale·ln(SoilMaking/soilDeepest), a little under two metres, and a
// history of a few thousand years makes nothing deeper by production alone.
const soilDeepest = 5.4e-6

// soilMade is how deep h metres of soil is after years of making at pace
// metres a year off bare rock. It is the production function integrated,
// which has a closed form: e^(H/H0) grows by P0·t/H0 exactly. So an age taken
// at once and the same age taken a year at a time come out the same.
func soilMade(h, years, pace float64) float64 {
	if years <= 0 || pace <= 0 {
		return h
	}
	if h > 30*SoilScale {
		return h // e^-60 of a millimetre: nothing, and e^60 would be the only thing left to go wrong
	}
	return SoilScale * math.Log(math.Exp(h/SoilScale)+pace*years/SoilScale)
}

// The weathering. How fast rock turns into soil, and how far soil is turned
// into clay, both go as the chemistry does: faster warm than cold, and faster
// the more water goes through it to carry away what dissolves. West (2012)
// puts both in one form, an Arrhenius term in the temperature against a
// reference and a saturating term in the runoff,
//
//	W = exp(Ea/R·(1/T0 − 1/T)) · (1 − e^(−q/q*)) / (1 − e^(−q0/q*)).
//
// weatherEnergy is Ea. Silicate dissolution measured on catchments comes out
// near sixty kilojoules a mole (White and Blum 1995), which is taken. T0 and
// q0 are the map's middling climate, the temperature a valley's weather is and
// three tenths of a metre of runoff a year, so that the middling ground reads
// one. weatherRunoff is q*, the runoff past which more
// water stops mattering because what dissolves is carried off as fast as it
// can dissolve.
const (
	weatherEnergy = 60e3  // J/mol
	gasConstant   = 8.314 // J/(mol·K)
	weatherRunoff = 400.0 // mm/yr
	middleRunoff  = 300.0 // mm/yr
	kelvin        = 273.15
)

// weatherMost holds the reading short of what a wet equator over basalt would
// make it, which is not ground the map has anywhere else to compare with.
const weatherMost = 4.0

// meanTempOf is the year's mean temperature on tile i: the latitude's, less
// what the height of the ground takes off it, with what the currents offshore
// add. It is what the soil is formed under, which is the year and not the day.
func (g *Grid) meanTempOf(i int) float64 {
	y := i / g.W
	mean := MeanTemp
	if g.air != nil && y < len(g.air.mean) {
		mean = g.air.mean[y]
	}
	t := mean - Lapse*g.Tiles[i].Height
	if g.Wrap {
		t += g.CoastWarmth(i)
	}
	return t
}

// wetOf is how much water goes through tile i's ground, as West's runoff term
// in [0,1]. A map whose water has not been worked out - the middle of a
// history, a grid made by hand - is read as middling.
func (g *Grid) wetOf(i int) float64 {
	if len(g.runoff) != len(g.Tiles) {
		return 1 - math.Exp(-middleRunoff/weatherRunoff)
	}
	return 1 - math.Exp(-g.runoff[i]/weatherRunoff)
}

// weathering is W on tile i: how hard the climate there weathers rock, against
// the map's middling ground at one.
func (g *Grid) weathering(i int) float64 {
	t := kelvin + g.meanTempOf(i)
	arr := math.Exp(weatherEnergy / gasConstant * (1/(kelvin+MeanTemp) - 1/math.Max(t, kelvin-60)))
	wet := g.wetOf(i) / (1 - math.Exp(-middleRunoff/weatherRunoff))
	return math.Min(weatherMost, arr*wet)
}

// clayGain is how strongly the weathering turns what the rock left into clay:
// the odds of a grain of soil being clay go as W to this power. Feldspars and
// micas weather to clay minerals and quartz does not, so the clay is taken out
// of the silt and sand in the proportion they were in. A half puts a
// granite's clay at a fifth where the map's weather is middling, at a sixth
// on ground half as weathered, and at a third on ground four times as
// weathered. Clay rising with the warmth and the wet a soil formed under, on
// the same rock, is the pattern Birkeland (1999) lays out; the power is chosen
// to give a span of about that size and is not fitted.
const clayGain = 0.5

// weathered is the mixture a rock leaving sand and clay at the middling
// weathering comes to under weathering w.
func weathered(sand, clay, w float64) (float64, float64) {
	if w <= 0 || clay <= 0 {
		return sand, clay
	}
	odds := clay / (1 - clay) * math.Pow(w, clayGain)
	c := odds / (1 + odds)
	return sand * (1 - c) / (1 - clay), c
}

// soilDepthOf is how deep soil stands on tile i when it is making what it
// loses: the steady state of the production function against what the water
// and the creep take off it, on the ground as it lies. It is how deep a fresh
// map's soil is set - there being no history of it to have left any - and it
// is reckoned the same way the ages then move it, so they start level with
// it.
//
// The water takes soil at the rate it takes ground: Erodibility·√Q·hold·S.
// The creep takes it in proportion to how deep it is and how sharply the
// ground is rounded over the tile (see creep); where the ground is hollowed
// it brings soil in instead. Production falls with depth and the creep's
// taking rises with it, so there is one depth where they meet, and it is
// found by halving.
func (g *Grid) soilDepthOf(i int) float64 {
	t := &g.Tiles[i]
	if t.Wet() || t.Terrain == Rock {
		return 0 // water, and the outcrops: ground the soil has already gone from
	}
	p := g.PosOf(i)
	making := SoilMaking * g.weathering(i)
	deepest := SoilScale * math.Log(math.Max(1, making/soilDeepest))
	if making <= soilDeepest {
		return 0
	}
	// The curvature over the tile, as the creep reads it: the height of the
	// neighbours over this tile, a diagonal counting half.
	round := 0.0
	for _, off := range Dirs {
		q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
		if !g.In(q) {
			continue
		}
		near := 1.0
		if off.X != 0 && off.Y != 0 {
			near = 0.5
		}
		round += near * (g.At(q).Height - t.Height)
	}
	water := Erodibility * math.Sqrt(t.Flow) * hold(t) * g.Slope(p) / ageYears
	// Creep at a metre of soil per metre of soil, and in from the hollow.
	creepy := Creep / 8 * hold(t) / SoilScale / ageYears
	taken := func(h float64) float64 {
		return water - creepy*math.Min(h, soilActive)*round
	}
	if taken(deepest) <= soilDeepest {
		return deepest
	}
	lo, hi := 0.0, deepest
	for k := 0; k < 40; k++ {
		mid := (lo + hi) / 2
		if making*math.Exp(-mid/SoilScale) > math.Max(soilDeepest, taken(mid)) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}

// laySoil sets every tile's soil to its steady depth. It runs when a map is
// made, once the ground, the water and what grows on it are settled.
func (g *Grid) laySoil() {
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			g.Tiles[i].Soil = float32(g.soilDepthOf(i))
		}
	})
}

// ageYears is how many years an age of weather is: a decade. See Erode.
const ageYears = 10.0
