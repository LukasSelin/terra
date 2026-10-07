package terra

import (
	"math"

	"github.com/LukasSelin/terra/internal/atmos"
)

// The frozen ground, read off the ground's own heat.
//
// Permafrost was read off Nelson and Outcalt's frost index over the air's
// year, with the ground's surface taken a fixed two degrees warmer than the
// air for the snow that blankets it in the winter (see atmos.FrostShare). Two
// degrees is the snow of nowhere in particular: a windswept tundra with a
// hand's depth of snow freezes nearly as hard as its air, and a taiga under a
// metre of it hardly freezes at all. And the ground was frozen or not, with
// nothing to say how deep the summer thawed it.
//
// Here the ground is read as the maps of the northern permafrost read it
// (Obu and others, 2019), by the temperature at the top of the permafrost
// (Smith and Riseborough, 1996; Riseborough and others, 2008):
//
//	TTOP = (rk·nT·DDT − nF·DDF) / P
//
// DDT and DDF are the year's thawing and freezing degree-days in the air and
// P the days of the year. nF is how much of the winter's cold gets through the
// snow lying on the ground, phase by phase, as the snowpack (snow.go) lays it;
// nT the same for the summer's warmth, taken as one. rk is the active layer's
// conductivity thawed over its conductivity frozen: ice conducts heat four
// times as well as water, so a wet ground lets the winter's cold in more
// readily than it lets the summer's warmth, and holds permafrost under a
// year whose mean is over freezing; a peat, which is mostly water and dry at
// its top in the summer, the more so. The ground below the active layer stays
// frozen where TTOP is under nought.
//
// How deep each summer thaws is Stefan's solution (Lunardini, 1981; Nelson
// and others, 1997, read the CALM sites by it): the thaw goes down as the
// summer's degree-days carry heat through what has thawed, and every metre
// of it costs the latent heat of the water in it, so z = √(2·k·DDT/(L·θ)) for
// a conductivity k and a water content θ. The water is the soil's bucket's
// (soilwater.go) through the summer; over a peat the thaw goes through the
// peat first, whose conductivity is a third of a mineral soil's, and then on
// into the ground under it.
//
// A map whose snow has not been read - and a history, whose ground knows no
// snow - is read as it was, off the frost index.

// The heat. latentFusion is what melts the ice of a cubic metre of water, in
// joules; frostSpread how far either side of nought, in degrees, a tile's
// TTOP spreads over its ground (see FrostShare).
const (
	latentFusion = 3.34e8 // J/m³
	frostSpread  = 1.5    // °C
)

// The snow. A pack's depth is its water over its density, which is some 0.2
// to 0.35 of water's in the seasonal snow of the tundra and the taiga (Sturm
// and others, 2010); snowDensity is the middle of it. The share of the
// winter's cold that gets through it, nF, falls off with its depth: Kade and
// others (2006) measured a third to a half under 40 to 60 centimetres of
// tundra snow, and Gisnås and others (2016) mapped Norway's permafrost with
// nF falling from one on bare ground to a fifth under a metre. An e-fold
// every snowDamp of depth is between them.
const (
	snowDensity = 250.0 // kg/m³
	snowDamp    = 0.5   // m
)

// The ground's conductivity, in W/m/K, by Johansen's (1975) reading as
// Farouki (1981) gives it: the dry ground's, the ground's with its pores full
// of water, and of ice, and between them by how full the pores are - in
// proportion for frozen ground, and as the saturation's logarithm, one over
// it, for thawed ground, whose water bridges the grains first. A mineral soil
// is taken with two fifths of its grains quartz and its pores 0.45 of it; a
// peat with its pores nine tenths of it, its solids organic, and its pores
// soaked to peatSoaked through the thaw.
const (
	mineralPores  = 0.45
	mineralDry    = 0.2
	mineralThawed = 1.5
	mineralFrozen = 2.8
	peatPores     = 0.9
	peatDry       = 0.05
	peatThawed    = 0.52
	peatFrozen    = 1.77
	peatSoaked    = 0.85
	// thawLeast is the least water, a share of the volume, a ground that
	// thaws is read at: a dry gravel's.
	thawLeast = 0.05
	// Soil Taxonomy keys a soil as a Gelisol with permafrost within
	// gelicNear of its surface, or within gelicDepth where the frost has
	// churned the soil over it into gelic materials (Soil Survey Staff,
	// 1999). The churning is the ice that segregates in a freezing ground,
	// and a ground segregates ice where its pores hold water enough to feed
	// the lenses: churnSoaked of them, chosen, through the thaw.
	gelicNear   = 1.0 // m
	gelicDepth  = 2.0 // m
	churnSoaked = 0.5
)

// daysAYear is the days of a year of the planet's.
const daysAYear = secondsPerYear / 86400

// frost is a tile's frozen ground as its heat reads it: the temperature at
// the top of its permafrost, in degrees; how deep its summer thaws, in
// metres; and how full of water the pores of its mineral ground are through
// the thaw.
type frost struct{ ttop, thaw, soaked float64 }

// groundFrost is tile i's frost. ok is false where the snow has not been
// read, under water and through a history: see the top of this file.
func (g *Grid) groundFrost(i int) (f frost, ok bool) {
	n := len(g.Tiles)
	if g.deep > 0 || len(g.warm) != n || len(g.snowWater) != n*atmos.Phases || len(g.soilHold) != n ||
		i < 0 || i >= n || g.Tiles[i].Wet() {
		return f, false
	}
	mean, swing := g.meanOn(i, g.Elevation(i)), float64(g.swing[i])
	warm, cold := atmos.PhaseDegreeDays(mean, swing)
	var ddt, ddf, wet float64
	hold := float64(g.soilHold[i])
	for k := range atmos.Phases {
		ddt += warm[k]
		depth := float64(g.snowWater[i*atmos.Phases+k]) / snowDensity
		ddf += math.Exp(-depth/snowDamp) * cold[k]
		if hold > 0 {
			wet += warm[k] * float64(g.soilWater[i*atmos.Phases+k]) / hold
		}
	}
	fill := 0.5
	if hold > 0 && ddt > 0 {
		fill = clamp01(wet / ddt)
	}
	// The mineral ground's water through the thaw: what the soil holds at its
	// wilting point, and the bucket's fill of what it holds over that.
	wilt, field := saxtonRawls(g.textureOf(i))
	water := math.Max(thawLeast, math.Min(mineralPores, wilt+fill*math.Max(0, field-wilt)))
	kt, kf := conductivity(water/mineralPores, mineralDry, mineralThawed, mineralFrozen)
	pt, pf := conductivity(peatSoaked, peatDry, peatThawed, peatFrozen)
	peat := g.PeatDepth(i)
	thaw := stefan(ddt, peat, peatPores*peatSoaked, pt, water, kt)
	// The active layer's rk is the peat's as far as the thaw goes through
	// peat, and the mineral ground's for the rest.
	inPeat := 0.0
	if thaw > 0 {
		inPeat = math.Min(1, peat/thaw)
	}
	rk := (1-inPeat)*kt/kf + inPeat*pt/pf
	return frost{(rk*ddt - ddf) / daysAYear, thaw, water / mineralPores}, true
}

// conductivity is a ground's conductivity thawed and frozen with its pores
// soaked to share full, between dry and its pores full of water or of ice
// (Johansen, 1975).
func conductivity(soaked, dry, thawed, frozen float64) (kt, kf float64) {
	soaked = clamp01(soaked)
	ke := 0.0
	if soaked > 0 {
		ke = math.Max(0, 1+math.Log10(soaked))
	}
	return dry + (thawed-dry)*ke, dry + (frozen-dry)*soaked
}

// stefan is how deep a summer of ddt thawing degree-days thaws a ground of
// peat metres of peat holding peatWater of its volume as water and
// conducting peatK, over mineral ground holding water and conducting k. Each
// layer's thaw is Stefan's; under the peat the heat comes through the thawed
// peat and the thawed ground below it in series.
func stefan(ddt, peat, peatWater, peatK, water, k float64) float64 {
	q := ddt * 86400 // degree-seconds
	if q <= 0 {
		return 0
	}
	if peat > 0 {
		through := latentFusion * peatWater * peat * peat / (2 * peatK)
		if q <= through {
			return math.Sqrt(2 * peatK * q / (latentFusion * peatWater))
		}
		q -= through
	}
	a := latentFusion * water / (2 * k)
	b := latentFusion * water * math.Max(0, peat) / peatK
	if peat <= 0 {
		b = 0
	}
	return math.Max(0, peat) + (-b+math.Sqrt(b*b+4*a*q))/(2*a)
}

// frostShareOf is the share of a ground whose top of the permafrost stands at
// ttop degrees that is frozen: a half at nought, as the zones of the maps are
// read off the share of their ensemble of TTOP that is under it (Obu and
// others, 2019), and none and all frostSpread either side, as the snow
// drifts, the slopes face and the peat lies across a tile. frostSpread is
// chosen.
func frostShareOf(ttop float64) float64 {
	t := clamp01((frostSpread - ttop) / (2 * frostSpread))
	return t * t * (3 - 2*t)
}

// permafrost reports whether most of tile i's ground stays frozen under its
// active layer: TTOP under nought, or the frost index's line where the snow
// has not been read.
func (g *Grid) permafrost(i int) bool {
	if f, ok := g.groundFrost(i); ok {
		return f.ttop < 0
	}
	return g.Frozen(g.PosOf(i))
}

// gelic reports whether tile i's soil is a Gelisol's: permafrost under most
// of it, within gelicDepth of the surface.
func (g *Grid) gelic(i int) bool {
	if f, ok := g.groundFrost(i); ok {
		return f.ttop < 0 && (f.thaw <= gelicNear || f.thaw <= gelicDepth && f.soaked >= churnSoaked)
	}
	return g.Frozen(g.PosOf(i))
}

// ActiveLayer is how deep, in metres, tile i's ground thaws each summer over
// its permafrost: Stefan's depth for the summer's thawing degree-days, the
// ground's water through the thaw and the peat on it. It is nothing where
// there is no permafrost, and where the snow has not been read.
func (g *Grid) ActiveLayer(i int) float64 {
	f, ok := g.groundFrost(i)
	if !ok || frostShareOf(f.ttop) <= 0 {
		return 0
	}
	return f.thaw
}
