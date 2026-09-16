package terra

import (
	"math"
	"sync"
)

// The warmth of a latitude, worked out from the sun.
//
// The globe's year used to be written down: the temperate mean, warmer toward
// the equator and colder toward the poles by thirty degrees times how far the
// cosine of the latitude stood from its value at forty-five. That gave an
// equator of nineteen degrees and poles of minus eleven, against the real
// world's twenty-six and its minus twenty and minus fifty, and nothing about it
// could be asked why.
//
// Here it is the diffusive energy-balance climate of Budyko (1969), Sellers
// (1969) and North (1975), in its seasonal form (North and Coakley, 1979):
//
//	C ∂T/∂t = Q s(φ,t) (1 - α(T)) - (A + B T) + ∂/∂x [D (1 - x²) ∂T/∂x]
//
// with x the sine of the latitude. The sun each band has on each day is the
// daily mean at the top of the air (Berger, 1978; FAO-56's form of it, which
// petTable reads too); what the ground sends back to space is Budyko's line
// fitted to the satellites, A + B T; the heat the air and the sea carry toward
// the poles is a diffusion; and the albedo has the step at the edge of the ice
// that makes such a climate's ice caps (Budyko's, and North's "small ice cap
// instability").
//
// Each band is two columns - the land, holding the heat of the air over it and
// a little ground, and the sea, holding its mixed layer's - sharing the band's
// diffusion and trading heat with one another. The land's small store is what
// gives a continent its large, early year and the sea's large one its small,
// late one, which is continentality read off the heat capacities rather than
// written down: see swingAt and lagAt.
//
// It is worked out once, for a planet with ebmLand of each band under land,
// and read by latitude.

// The energy balance.
const (
	// solarConstant is the sun's flux at the planet's mean distance, W/m²
	// (Kopp and Lean, 2011).
	solarConstant = 1361.0
	// olrA and olrB are Budyko's outgoing longwave line, A + B T with T in
	// degrees: North and Coakley's (1979) fit, 203.3 W/m² and 2.09 W/m² a
	// degree.
	olrA = 203.3
	olrB = 2.09
	// ebmDiffusion is D, the heat the air and the sea carry down the gradient,
	// in W/m² a degree in North's form: 0.44, the figure of North and
	// Coakley's seasonal model. It is the gradient of the moist energy it is
	// carried down here and not of the temperature (see moistEnergy); carried
	// down the temperature's at 0.44, the equator came out at thirty-seven
	// degrees and the poles at minus twenty-three, and no figure between 0.44
	// and 1 gave a tropics under twenty-eight and poles under minus ten.
	ebmDiffusion = 0.44
	// albedoA0 and albedoA2 are the ice-free albedo's mean and its second
	// Legendre term in x, which carries the clouds and the low sun of the high
	// latitudes (North, Cahalan and Coakley, 1981, fit a0 near a third and a2
	// a quarter to a third). The pair here is the one of a0 0.32-0.35 and a2
	// 0.25-0.35 that came closest to the zonal means of Legates and Willmott
	// (1990) under the rest of the balance.
	albedoA0 = 0.33
	albedoA2 = 0.30
	// albedoIce is the albedo of ground or sea under ice and snow (North, 1975:
	// 0.62), full under iceEdge degrees less iceWidth and none over it plus.
	albedoIce = 0.62
	iceEdge   = -10.0
	iceWidth  = 3.0
	// heatLand is the heat a square metre of land's column holds a degree:
	// the air's, cp p/g, some ten million joules, and the top half metre of
	// the ground the year reaches. heatSea is the ocean mixed layer's, some
	// fifty metres of water (de Boyer Montégut, 2004).
	heatLand = 1.5e7
	heatSea  = 50 * seaHeat
	// landSeaExchange is how many W/m² a degree the land and the sea of a band
	// trade for the difference between them: the sea breeze and the monsoon,
	// the transient eddies that cross a coast. A few W/m²K, as in North, Short
	// and Mengel's (1983) two-dimensional model. At three the land's year swung
	// twenty-eight degrees either side of its mean at fifty-five and twenty-one
	// at forty-five - a globe's midlatitude land, some three quarters land
	// round about, came out at twenty-two, where the real continents' zonal
	// mean is some fifteen - and the winters that left were too cold for any
	// temperate coast poleward of forty-five. At six they are 21.5 and 16.
	landSeaExchange = 6.0
	// ebmLand is how much of each band is land. The earth's is some three
	// tenths; a globe's own varies with its seed, and the continentality of a
	// place is read off the ground about it instead.
	ebmLand = 0.3
)

// The lattice the balance is solved on.
const (
	ebmBands = 90 // bands of equal area, uniform in x
	ebmSteps = 6  // steps a day
	ebmYears = 20 // years run to settle the ocean from a uniform start
)

// ebmClimate is the balance's settled year: at each band, the annual mean of
// the band and, for the land and the sea of it, the swing and the lag of their
// year's first harmonic.
type ebmClimate struct {
	mean           [ebmBands]float64
	swingL, swingS [ebmBands]float64 // degrees, half the range of the first harmonic
	lagL, lagS     [ebmBands]float64 // days of 365.25 after the solstice of the band's hemisphere
	meanL, meanS   [ebmBands]float64
}

var (
	ebmOnce sync.Once
	ebmOut  *ebmClimate
)

// ebm is the settled energy-balance year, worked out the first time it is
// asked for.
func ebm() *ebmClimate {
	ebmOnce.Do(func() { ebmOut = solveEBM() })
	return ebmOut
}

// insolation is the daily mean sun at the top of the air at latitude phi
// (radians) on day j of 365.25 counted from the first of January, in W/m².
func insolation(phi, j float64) float64 {
	d := 0.409 * math.Sin(2*math.Pi*j/365.25-1.39)
	dr := 1 + 0.033*math.Cos(2*math.Pi*j/365.25)
	ws := math.Acos(math.Max(-1, math.Min(1, -math.Tan(phi)*math.Tan(d))))
	return solarConstant / math.Pi * dr * (ws*math.Sin(phi)*math.Sin(d) + math.Cos(phi)*math.Cos(d)*math.Sin(ws))
}

// iceAlbedo is the albedo at x, the sine of the latitude, of ground at temp.
func iceAlbedo(x, temp float64) float64 {
	return iceAlbedoWith(ebmParams{a0: albedoA0, a2: albedoA2}, x, temp)
}

func iceAlbedoWith(p ebmParams, x, temp float64) float64 {
	free := p.a0 + p.a2*(1.5*x*x-0.5)
	w := 0.5 * (1 + math.Tanh((temp-iceEdge)/iceWidth))
	return albedoIce + (free-albedoIce)*w
}

// solveEBM runs the balance from a uniform start until its year repeats and
// reads the last year's harmonics.
func solveEBM() *ebmClimate {
	return solveEBMWith(ebmParams{ebmDiffusion, albedoA0, albedoA2, heatLand, heatSea, landSeaExchange})
}

// The sea ice, as Wagner and Eisenman (2015) put it into the seasonal
// balance: the sea's column is an enthalpy, open water where it is over
// nothing and ice of thickness -E/seaIceLatent where it is under; the surface
// of the ice stands where the heat conducted up through it balances what the
// air takes off it, and never over melting. A polar summer melts ice instead
// of warming the air, which is what keeps the poles cold.
const (
	seaIceLatent  = 9.5 * secondsPerYear // J/m³, the latent heat of a cubic metre of sea ice
	seaIceConduct = 2.0                  // W/m/K through the ice
	seaIceBelow   = 4.0                  // W/m² the ocean under the ice brings up to it
)

// seaSurface is the temperature of the sea's surface for a column of enthalpy
// e J/m² with a mixed layer holding cs J/m²K, under sun W/m².
func seaSurface(e, cs, sun float64) float64 {
	if e >= 0 {
		return e / cs
	}
	h := -e / seaIceLatent
	k := seaIceConduct / h
	t0 := (sun*(1-albedoIce) - olrA) / (olrB + k)
	return math.Min(0, t0)
}

// diffuse carries the heat of one step down the gradient of the bands' moist
// energy, backward in time so that the step can be as long as the sea wants
// rather than as short as the land and the tropics' steep moist energy would
// make it: each band's mean temperature is solved for with its moist energy
// taken as linear about where it stands, and the heat that moves is given to
// the land and the sea of the band alike, a square metre each.
func diffuse(face *[ebmBands + 1]float64, tl, ts, es *[ebmBands]float64, p ebmParams, dt float64) {
	const n = ebmBands
	gamma := ebmLand/p.cl + (1-ebmLand)/p.cs
	var h, slope, lo, mid, hi, rhs [n]float64
	for k := range h {
		t := ebmLand*tl[k] + (1-ebmLand)*ts[k]
		h[k] = moistEnergy(t)
		slope[k] = (moistEnergy(t+0.05) - moistEnergy(t-0.05)) / 0.1
	}
	// dT_k = dt γ Σ face (h_j + h'_j dT_j - h_k - h'_k dT_k)
	for k := 0; k < n; k++ {
		mid[k] = 1
		if k > 0 {
			c := dt * gamma * face[k]
			mid[k] += c * slope[k]
			lo[k] = -c * slope[k-1]
			rhs[k] += c * (h[k-1] - h[k])
		}
		if k < n-1 {
			c := dt * gamma * face[k+1]
			mid[k] += c * slope[k]
			hi[k] = -c * slope[k+1]
			rhs[k] += c * (h[k+1] - h[k])
		}
	}
	for k := 1; k < n; k++ {
		w := lo[k] / mid[k-1]
		mid[k] -= w * hi[k-1]
		rhs[k] -= w * rhs[k-1]
	}
	var dT [n]float64
	dT[n-1] = rhs[n-1] / mid[n-1]
	for k := n - 2; k >= 0; k-- {
		dT[k] = (rhs[k] - hi[k]*dT[k+1]) / mid[k]
	}
	for k := range dT {
		heat := dT[k] / gamma
		tl[k] += heat / p.cl
		es[k] += heat
	}
}

// moistEnergy is the moist static energy of air near the ground at temp
// degrees, over its heat capacity: the temperature and the latent heat its
// water carries at a relative humidity of moistHumidity. What the air carries
// poleward is this, and not its warmth alone (Frierson, Held and
// Zurita-Gotor, 2007; Hwang and Frierson, 2010), which is why the tropics are
// flat - a little warmth there is a great deal of water - and the high
// latitudes steep.
func moistEnergy(temp float64) float64 {
	return temp + latentHeat/airHeat*moistHumidity*saturation(temp)
}

// saturation is the specific humidity of saturated air at temp degrees and
// the mean pressure at sea level, kg/kg: Bolton's (1980) form of the
// Clausius-Clapeyron relation for the vapour pressure.
func saturation(temp float64) float64 {
	temp = math.Max(temp, coldest)
	e := 6.112 * math.Exp(17.67*temp/(temp+243.5))
	return 0.622 * e / (beltMean - 0.378*e)
}

// The air's water.
const (
	latentHeat    = 2.5e6  // J/kg, of vaporisation
	airHeat       = 1004.0 // J/kg/K, of dry air at constant pressure
	moistHumidity = 0.8    // the relative humidity of the air near the ground
)

// ebmParams are the balance's figures, gathered so that they can be probed.
type ebmParams struct{ d, a0, a2, cl, cs, nu float64 }

func solveEBMWith(p ebmParams) *ebmClimate {
	const n = ebmBands
	dx := 2.0 / n
	var x, phi [n]float64
	for k := range x {
		x[k] = -1 + (float64(k)+0.5)*dx
		phi[k] = math.Asin(x[k])
	}
	// The diffusion's conductance across the face above each band.
	var face [n + 1]float64
	for k := 1; k < n; k++ {
		xf := -1 + float64(k)*dx
		face[k] = p.d * (1 - xf*xf) / (dx * dx)
	}
	var tl, ts, es [n]float64
	dt := 86400.0 / ebmSteps
	days := 365.25
	stepsYear := int(math.Round(days * ebmSteps))
	out := &ebmClimate{}
	var sumL, sumS, cL, sL, cS, sS [n]float64
	// The sun of each band on each day, which the steps of the day share.
	sunDays := int(math.Ceil(days))
	sunOf := make([]float64, sunDays*n)
	for d := 0; d < sunDays; d++ {
		for k := 0; k < n; k++ {
			sunOf[d*n+k] = insolation(phi[k], float64(d)+0.5)
		}
	}
	// A start near the settled one: each band's annual sun against Budyko's line.
	for k := range tl {
		var q float64
		for d := 0; d < 365; d++ {
			q += sunOf[d*n+k] / 365
		}
		t0 := (q*(1-iceAlbedo(x[k], 10)) - olrA) / (olrB + 2*p.d)
		tl[k], ts[k], es[k] = t0, t0, t0*p.cs
	}
	for year := 0; year < ebmYears; year++ {
		last := year == ebmYears-1
		for s := 0; s < stepsYear; s++ {
			j := float64(s) / ebmSteps
			sun := sunOf[min(int(j), sunDays-1)*n:]
			for k := range tl {
				fl := sun[k]*(1-iceAlbedoWith(p, x[k], tl[k])) - (olrA + olrB*tl[k]) + p.nu*(1-ebmLand)*(ts[k]-tl[k])
				// The sea: open water holding its mixed layer's heat, or ice
				// over it. See seaSurface.
				alb := albedoIce
				if es[k] > 0 {
					alb = iceAlbedoWith(p, x[k], ts[k])
				}
				fs := sun[k]*(1-alb) - (olrA + olrB*ts[k]) + p.nu*ebmLand*(tl[k]-ts[k])
				if es[k] < 0 {
					fs += seaIceBelow
				}
				tl[k] += dt * fl / p.cl
				es[k] += dt * fs
			}
			diffuse(&face, &tl, &ts, &es, p, dt)
			for k := range ts {
				ts[k] = seaSurface(es[k], p.cs, sun[k])
			}
			if last {
				th := 2 * math.Pi * j / days
				c, sn := math.Cos(th), math.Sin(th)
				for k := range tl {
					sumL[k] += tl[k]
					sumS[k] += ts[k]
					cL[k] += tl[k] * c
					sL[k] += tl[k] * sn
					cS[k] += ts[k] * c
					sS[k] += ts[k] * sn
				}
			}
		}
	}
	m := float64(stepsYear)
	// The northern solstice falls on day 172 of the calendar; a band's lag is
	// read from its own hemisphere's.
	for k := range tl {
		out.meanL[k], out.meanS[k] = sumL[k]/m, sumS[k]/m
		out.mean[k] = ebmLand*out.meanL[k] + (1-ebmLand)*out.meanS[k]
		ampLag := func(c, s float64) (float64, float64) {
			a, b := 2*c/m, 2*s/m // T ≈ mean + a cos θ + b sin θ
			amp := math.Hypot(a, b)
			peak := math.Atan2(b, a) / (2 * math.Pi) * days // the day of the warmest
			solstice := 172.0
			if x[k] < 0 {
				solstice += days / 2
			}
			lag := math.Mod(peak-solstice+2*days, days)
			if lag > days/2 {
				lag -= days
			}
			return amp, lag
		}
		out.swingL[k], out.lagL[k] = ampLag(cL[k], sL[k])
		out.swingS[k], out.lagS[k] = ampLag(cS[k], sS[k])
	}
	return out
}

// at reads a field of the balance at a latitude in degrees, between the
// bands' centres.
func (c *ebmClimate) at(field *[ebmBands]float64, lat float64) float64 {
	x := math.Sin(lat * math.Pi / 180)
	f := (x+1)/(2.0/ebmBands) - 0.5
	f = math.Max(0, math.Min(ebmBands-1, f))
	k := int(f)
	if k >= ebmBands-1 {
		return field[ebmBands-1]
	}
	return field[k] + (field[k+1]-field[k])*(f-float64(k))
}

// ZonalMean is the year's mean at sea level at a latitude on a globe: the energy
// balance's zonal mean there. It was MeanTemp and thirty degrees times how
// far the cosine of the latitude stood from its value at Temperate, which put
// the equator at nineteen degrees and the poles at minus eleven.
func ZonalMean(lat float64) float64 {
	e := ebm()
	return e.at(&e.mean, lat)
}
