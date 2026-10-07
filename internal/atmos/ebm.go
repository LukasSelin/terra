package atmos

import (
	"math"
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
// PetTable reads too); what the ground sends back to space is Budyko's line
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
// It is worked out once for each forcing it is asked under - the sun, the
// orbit and the air's carbon: see forcing.go - for a planet with ebmLand of
// each band under land, and read by latitude.
//
// Its year is the calendar's. It used to step 365.25 days from the first of
// January while the calendar counted 360 from the spring equinox, and the
// lags it gave were turned from the one year into the other on their way out.
// Now it steps clock.Year days from the spring equinox, tick zero, so that a
// day of its year is a day of the calendar and its lags are days the calendar
// counts. The year itself is no shorter for it: a year is secondsPerYear
// whatever it is cut into, so the heat a day brings is a 360th of the year's,
// and every rate written "a year" - the rain's, the rock's - keeps its
// meaning. A day of the calendar is some twenty-four hours and twenty-one
// minutes of the sun's, which nothing that lives by it could tell.

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
	// ebmDiffusion is D, the heat the air carries down the gradient, in W/m²
	// a degree in North's form. It is the gradient of the moist energy it is
	// carried down here and not of the temperature (see moistEnergy); carried
	// down the temperature's at North and Coakley's 0.44, the equator came
	// out at thirty-seven degrees and the poles at minus twenty-three, and no
	// figure between 0.44 and 1 gave a tropics under twenty-eight and poles
	// under minus ten.
	//
	// It carried the sea's heat too, at 0.44, until the sea came to carry its
	// own (slab.go). It is now the air's share: fitted, with the sea's
	// ebmSeaDiffusion, so that the sea carries the share of the whole at
	// thirty-five degrees that the real one does on the mean of the two
	// hemispheres, 15% (22% in the north and 8% in the south, Trenberth and
	// Caron, 2001), and the balance's zonal means still stand within half a
	// degree of Legates and Willmott's (1990) at the equator, fifteen,
	// thirty, forty-five and sixty degrees. 0.38 and 0.25 give the sea 14.8%
	// at 35 and the five 27.3, 25.4, 20.0, 11.3 and 0.6 (0.49 rms), where
	// 0.44 alone gave 27.3, 25.5, 20.1, 11.2 and 0.0 (0.60). Pairs a little
	// nearer the five (0.39 and 0.27, 0.44 rms) stood the tropics three
	// tenths colder, 25.2 at fifteen, and a storm over the warm western
	// water at eighteen degrees north no longer found the 26.5 it needs. The
	// balance's whole carrying at 35 is 7.4 PW, against the real world's
	// 5.5-6: the albedo and Budyko's line it was fitted with ask for more
	// than the earth's, and the shares are what the fit holds to.
	ebmDiffusion = 0.38
	// ebmSeaDiffusion is the sea's carrying in a balance that has no sea of
	// its own worked out, D in North's form down the warmth of the sea's
	// column, the water under ice at the freezing point: the balance a
	// planet's climate is read from before its own sea is solved, and the
	// history's. A map's own sea is handed in in its place: see slab.go.
	ebmSeaDiffusion = 0.25
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
	lagL, lagS     [ebmBands]float64 // days of the calendar after the solstice of the band's hemisphere
	meanL, meanS   [ebmBands]float64
	// tauLand and tauSea are the heat capacity over the loss, C/B in days,
	// that gives the land's and the sea's lag at Temperate: see LagAt.
	tauLand, tauSea float64
	// equator is where the balance's warmth stands highest over its year -
	// the band's, its land's and its sea's - which is where the heat the air
	// carries poleward turns round: see circulation.go.
	equator [3]yearOf
	// airNorth and seaNorth are the heat the air and the sea carry toward the
	// north across the face below each band, watts over the whole parallel,
	// over the year: the faces at the poles carry nothing. seaIn is the heat
	// the sea's carrying leaves in each band's sea column, W a square metre of
	// sea. Where the balance was given a sea's heat (ebmParams.sea) seaIn is
	// that and seaNorth nought.
	airNorth, seaNorth [ebmBands + 1]float64
	seaIn              [ebmBands]float64
	// params are the figures it was worked out with.
	params ebmParams
	// contrast is Held and Hou's Δ_H: how far the radiative equilibrium's
	// equator stands over its pole, as a share of the planet's mean warmth in
	// kelvin. tropopause is how high the tropics' tropopause stands, metres.
	// See circulation.go.
	contrast, tropopause float64
}

// yearOf is a reading's year as its first harmonic: the mean, the swing
// either side of it, positive where the reading stands north of its mean
// in the north's summer, and the days of the calendar its crest falls after
// the north's solstice.
type yearOf struct{ mean, swing, lag float64 }

// insolation is the daily mean sun at the top of the air at latitude phi
// (radians) on day j of the calendar, counted from the spring equinox, in
// W/m², under f: Berger's (1978) daily mean in FAO-56's form, with the
// declination the obliquity times the sine of the sun's longitude and the
// inverse square of the distance 1 + 2e cos of its anomaly.
//
// The calendar is the equinox's, as a real one is: the sun's longitude is
// nought on tick zero and goes round once in clock.Year days, so the
// declination crosses the equator northward on the first day of the year and
// stands highest a quarter of the way into it, whatever the orbit; and
// perihelion moves through the year with the precession, falling on the day
// the sun's longitude is f.Perihelion less π. The form is first order in the
// eccentricity and takes the sun round the sky at an even pace, so that a
// season's length does not change with where perihelion falls; at the largest
// eccentricity of the last million years, some 0.05, the distance it leaves
// out is under one part in a hundred of the sun.
func (f Forcing) insolation(phi, j float64) float64 {
	lon := 2 * math.Pi * j / Year
	d := f.Obliquity * math.Sin(lon)
	dr := 1 + 2*f.Eccentricity*math.Cos(lon-f.Perihelion+math.Pi)
	ws := math.Acos(math.Max(-1, math.Min(1, -math.Tan(phi)*math.Tan(d))))
	return f.Solar / math.Pi * dr * (ws*math.Sin(phi)*math.Sin(d) + math.Cos(phi)*math.Cos(d)*math.Sin(ws))
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

// solveEBMUnder runs the balance under forcing f from a uniform start until
// its year repeats and reads the last year's harmonics.
func solveEBMUnder(f Forcing) *ebmClimate {
	return solveEBMWith(ebmReference(landSeaExchange), f)
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
// e J/m² with a mixed layer holding cs J/m²K, under sun W/m², sending back
// a + olrB T to space.
func seaSurface(e, cs, sun, a float64) float64 {
	if e >= 0 {
		return e / cs
	}
	h := -e / seaIceLatent
	k := seaIceConduct / h
	t0 := (sun*(1-albedoIce) - a) / (olrB + k)
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

// seaWater is the temperature of the water of a sea column of enthalpy e
// J/m² with a mixed layer holding cs J/m²K: its surface's, or the freezing
// point under ice.
func seaWater(e, cs float64) float64 { return math.Max(0, e/cs) }

// respond is how many degrees warmer each band of the balance c stands, in
// the mean of its year, for dq more watts a square metre of heat brought into
// its sea column: the balance linearized about its year's mean and settled,
//
//	B δT_k - Σ face (s_j δT_j - s_k δT_k) = (1 - ebmLand) δq_k
//
// with s the slope of the moist energy at the band's mean, the air carrying
// the heat on down its gradient as diffuse does. The land and the sea of a
// band share it, as the balance's columns share the air's carrying; what the
// heat does to the ice's albedo and its season is left out. It is a few
// microseconds, where the balance's twenty years are most of a second.
func (c *ebmClimate) respond(dq *[ebmBands]float64) [ebmBands]float64 {
	const n = ebmBands
	dx := 2.0 / n
	var face [n + 1]float64
	for k := 1; k < n; k++ {
		xf := -1 + float64(k)*dx
		face[k] = c.params.d * (1 - xf*xf) / (dx * dx)
	}
	var s, lo, mid, hi, rhs [n]float64
	for k := range s {
		t := c.mean[k]
		s[k] = (moistEnergy(t+0.05) - moistEnergy(t-0.05)) / 0.1
	}
	for k := range mid {
		mid[k] = olrB + (face[k]+face[k+1])*s[k]
		if k > 0 {
			lo[k] = -face[k] * s[k-1]
		}
		if k < n-1 {
			hi[k] = -face[k+1] * s[k+1]
		}
		rhs[k] = (1 - ebmLand) * dq[k]
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
	return dT
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

// ebmParams are the balance's figures, gathered so that they can be probed:
// the air's diffusion d, the albedo's a0 and a2, the land's and the sea's heat
// cl and cs, the exchange nu between them, the sea's own diffusion ds, and
// sea, where it is not nil, the heat the ocean brings each band's sea column,
// W a square metre of sea, in place of what ds carries.
type ebmParams struct {
	d, a0, a2, cl, cs, nu, ds float64
	sea                       *[ebmBands]float64
}

// ebmReference is the balance a planet's climate is read from before its own
// sea is known: the air's share carried down the moist energy, and the sea's
// down its own warmth. See ebmSeaDiffusion.
func ebmReference(nu float64) ebmParams {
	return ebmParams{d: ebmDiffusion, a0: albedoA0, a2: albedoA2, cl: heatLand, cs: heatSea, nu: nu, ds: ebmSeaDiffusion}
}

// solveEBMWith is the balance's settled year with figures p under forcing f.
func solveEBMWith(p ebmParams, f Forcing) *ebmClimate {
	const n = ebmBands
	f = f.OrDefault()
	a := f.olrA() // Budyko's A, less what the air's carbon holds back
	dx := 2.0 / n
	var x, phi [n]float64
	for k := range x {
		x[k] = -1 + (float64(k)+0.5)*dx
		phi[k] = math.Asin(x[k])
	}
	// The diffusion's conductance across the face above each band.
	var face, faceSea [n + 1]float64
	for k := 1; k < n; k++ {
		xf := -1 + float64(k)*dx
		face[k] = p.d * (1 - xf*xf) / (dx * dx)
		faceSea[k] = p.ds * (1 - xf*xf) / (dx * dx)
	}
	// A watt a square metre across a face, in the units the faces carry it
	// in, is this many watts toward the north across the whole parallel.
	perFace := 2 * math.Pi * planetRadius * planetRadius * dx
	var flow [n + 1]float64 // the sea's carrying, W a square metre of sea
	var airNorth, seaNorth [n + 1]float64
	var seaIn [n]float64
	var tl, ts, es [n]float64
	// The calendar's year, of which a day is a Year'th: see above.
	days := float64(Year)
	dt := secondsPerYear / days / ebmSteps
	stepsYear := Year * ebmSteps
	out := &ebmClimate{params: p}
	var sumL, sumS, cL, sL, cS, sS [n]float64
	// The warmest latitude of the band, its land and its sea, step by step
	// over the last year, gathered as sums for its harmonic.
	var eqSum, eqC, eqS [3]float64
	var band [n]float64
	// The sun of each band on each day, which the steps of the day share.
	sunDays := Year
	sunOf := make([]float64, sunDays*n)
	for d := 0; d < sunDays; d++ {
		for k := 0; k < n; k++ {
			sunOf[d*n+k] = f.insolation(phi[k], float64(d)+0.5)
		}
	}
	// A start near the settled one: each band's annual sun against Budyko's line.
	var annualSun [n]float64
	for k := range tl {
		var q float64
		for d := 0; d < sunDays; d++ {
			q += sunOf[d*n+k] / days
		}
		annualSun[k] = q
		t0 := (q*(1-iceAlbedo(x[k], 10)) - a) / (olrB + 2*p.d)
		tl[k], ts[k], es[k] = t0, t0, t0*p.cs
	}
	for year := 0; year < ebmYears; year++ {
		last := year == ebmYears-1
		for s := 0; s < stepsYear; s++ {
			j := float64(s) / ebmSteps
			sun := sunOf[int(j)*n:]
			for k := range tl {
				fl := sun[k]*(1-iceAlbedoWith(p, x[k], tl[k])) - (a + olrB*tl[k]) + p.nu*(1-ebmLand)*(ts[k]-tl[k])
				// The sea: open water holding its mixed layer's heat, or ice
				// over it. See seaSurface.
				alb := albedoIce
				if es[k] > 0 {
					alb = iceAlbedoWith(p, x[k], ts[k])
				}
				fs := sun[k]*(1-alb) - (a + olrB*ts[k]) + p.nu*ebmLand*(tl[k]-ts[k])
				if es[k] < 0 {
					fs += seaIceBelow
				}
				tl[k] += dt * fl / p.cl
				es[k] += dt * fs
			}
			diffuse(&face, &tl, &ts, &es, p, dt)
			// The sea's own carrying: down the warmth of its water, which
			// under ice is at the freezing point, or what a planet's own sea
			// brings each band.
			if p.sea == nil {
				for k := 1; k < n; k++ {
					flow[k] = faceSea[k] * (seaWater(es[k-1], p.cs) - seaWater(es[k], p.cs))
				}
				for k := range es {
					es[k] += dt * (flow[k] - flow[k+1])
				}
			} else {
				for k := range es {
					es[k] += dt * p.sea[k]
				}
			}
			for k := range ts {
				ts[k] = seaSurface(es[k], p.cs, sun[k], a)
			}
			if last {
				for k := 1; k < n; k++ {
					hs := moistEnergy(ebmLand*tl[k-1] + (1-ebmLand)*ts[k-1])
					hn := moistEnergy(ebmLand*tl[k] + (1-ebmLand)*ts[k])
					airNorth[k] += face[k] * (hs - hn) * perFace
					seaNorth[k] += (1 - ebmLand) * flow[k] * perFace
				}
				for k := range seaIn {
					if p.sea != nil {
						seaIn[k] += p.sea[k]
					} else {
						seaIn[k] += flow[k] - flow[k+1]
					}
				}
				th := 2 * math.Pi * j / days
				c, sn := math.Cos(th), math.Sin(th)
				for k := range band {
					band[k] = ebmLand*tl[k] + (1-ebmLand)*ts[k]
				}
				for q, field := range [3]*[n]float64{&band, &tl, &ts} {
					lat := warmestLat(field, &x)
					eqSum[q] += lat
					eqC[q] += lat * c
					eqS[q] += lat * sn
				}
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
	for k := range airNorth {
		out.airNorth[k], out.seaNorth[k] = airNorth[k]/m, seaNorth[k]/m
	}
	for k := range seaIn {
		out.seaIn[k] = seaIn[k] / m
	}
	// The northern solstice falls a quarter of the way into the calendar's
	// year; a band's lag is read from its own hemisphere's.
	for k := range tl {
		out.meanL[k], out.meanS[k] = sumL[k]/m, sumS[k]/m
		out.mean[k] = ebmLand*out.meanL[k] + (1-ebmLand)*out.meanS[k]
		ampLag := func(c, s float64) (float64, float64) {
			a, b := 2*c/m, 2*s/m // T ≈ mean + a cos θ + b sin θ
			amp := math.Hypot(a, b)
			peak := math.Atan2(b, a) / (2 * math.Pi) * days // the day of the warmest
			solstice := days / 4
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
	for q := range out.equator {
		a, b := 2*eqC[q]/m, 2*eqS[q]/m
		amp := math.Hypot(a, b)
		peak := math.Atan2(b, a) / (2 * math.Pi) * days
		lag := math.Mod(peak-days/4+2*days, days)
		if lag > days/2 {
			lag -= days
		}
		if math.Abs(lag) > days/4 {
			// Its crest falls in the north's winter: the swing is the other
			// way round.
			amp = -amp
			lag = math.Mod(lag+days/2+2*days, days)
			if lag > days/2 {
				lag -= days
			}
		}
		out.equator[q] = yearOf{mean: eqSum[q] / m, swing: amp, lag: lag}
	}
	// The radiative equilibrium the circulation works against: each band's
	// year's sun on ground of the ice-free albedo, against Budyko's line with
	// no heat carried in or out, and its second Legendre term, which is Held
	// and Hou's equilibrium's shape. See circulation.go.
	var p2 float64
	for k := range annualSun {
		re := (annualSun[k]*(1-(p.a0+p.a2*legendre2(x[k]))) - a) / olrB
		p2 += re * legendre2(x[k]) * 5 / n
	}
	var global float64
	for k := range out.mean {
		global += out.mean[k] / n
	}
	out.contrast = -1.5 * p2 / (global + 273.15)
	out.tropopause = (out.at(&out.mean, 0) + 273.15 - tropopauseTemp) / Lapse
	out.tauLand = math.Tan(yearOmega*out.at(&out.lagL, Temperate)) / yearOmega
	out.tauSea = math.Tan(yearOmega*out.at(&out.lagS, Temperate)) / yearOmega
	return out
}

// equatorReach is how far from the equator, in degrees, the warmest
// latitude is looked for: a summer's warmth further poleward than this is a
// continent's heat, and not where the air rises.
const equatorReach = 40.0

// warmestLat is the latitude, in degrees, at which a field of the balance
// stands highest within equatorReach of the equator: the band at the top, and
// the parabola through it and its neighbours for where between them.
func warmestLat(field, x *[ebmBands]float64) float64 {
	reach := math.Sin(equatorReach * math.Pi / 180)
	best := -1
	for k := range field {
		if math.Abs(x[k]) <= reach && (best < 0 || field[k] > field[best]) {
			best = k
		}
	}
	xm := x[best]
	if best > 0 && best < ebmBands-1 {
		lo, mid, hi := field[best-1], field[best], field[best+1]
		if d := lo - 2*mid + hi; d < 0 {
			xm += 0.5 * (lo - hi) / d * (x[best+1] - x[best])
		}
	}
	return math.Asin(math.Max(-1, math.Min(1, xm))) * 180 / math.Pi
}

// at reads a field of the balance at a latitude in degrees, between the
// bands' centres.
func (c *ebmClimate) at(field *[ebmBands]float64, lat float64) float64 {
	return ebmRead(field, lat)
}

// ebmRead is a field on the balance's bands read at a latitude in degrees,
// between the bands' centres.
func ebmRead(field *[ebmBands]float64, lat float64) float64 {
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
// the equator at nineteen degrees and the poles at minus eleven. It is
// today's: ZonalMeanUnder reads it under another forcing.
func ZonalMean(lat float64) float64 {
	e := ebm()
	return e.at(&e.mean, lat)
}
