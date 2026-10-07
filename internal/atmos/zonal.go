package atmos

import "math"

// The energy balance of an epoch: ebm.go's, worked out for a planet whose
// land share differs band by band, and which can be carried on from where an
// earlier epoch's left off.
//
// ebm.go's balance is solved once, for a planet with ebmLand of every band
// under land, and read by latitude: the warmth of a latitude is the same
// whatever the continents are doing. A history's epochs move their
// continents from the equator to a pole and back, and a polar continent is a
// cold one: its small heat store and its snow give it a colder year than the
// sea it replaced. This is the balance under a land share per band, for the
// deep-time climate's cost study (docs/deep-time-climate.md). Nothing in the
// making of a world reads it unless the history's deepClimate switch is on,
// and it is off.
//
// It is a copy of solveEBMWith's loop with the land and the sea of each band
// as arrays and not constants, rather than a change to it, so that today's
// balance is untouched: TestZonalYearOfTodaysLandIsTodays holds the copy to
// the original, bit for bit, under a uniform ebmLand.

// ZonalBands is how many bands of equal area the balance is solved on: band
// k spans the sines of latitude from -1 + 2k/ZonalBands to -1 +
// 2(k+1)/ZonalBands.
const ZonalBands = ebmBands

// ZonalYears is how many years the balance runs from a uniform start before
// its last year is read.
const ZonalYears = ebmYears

// ZonalYear is a settled energy-balance year under a land share per band,
// with the state it ended in, from which another can be carried on.
type ZonalYear struct {
	c          *ebmClimate
	land, sea  [ebmBands]float64
	tl, ts, es [ebmBands]float64
}

// TodaysLand is ebm.go's land share in every band, as the land and the sea of
// a ZonalYear are given.
func TodaysLand() (land, sea [ZonalBands]float64) {
	for k := range land {
		land[k], sea[k] = ebmLand, 1-ebmLand
	}
	return land, sea
}

// SolveZonal works out the balance under land and sea, each band's shares,
// over years years: from a uniform start where from is nil, and from the
// state from ended in where it is not, which settles in a few years rather
// than twenty when the land has moved a little.
func SolveZonal(land, sea *[ZonalBands]float64, years int, from *ZonalYear) *ZonalYear {
	return SolveZonalForced(land, sea, years, from, 0)
}

// SolveZonalForced is SolveZonal with forcing W/m² more held back from
// space than today: Budyko's A less forcing. A doubling of the air's carbon
// is some 3.7 (5.35 ln 2, Myhre et al. 1998). It stands in for A0's Forcing
// until that is merged: see docs/deep-time-climate.md.
func SolveZonalForced(land, sea *[ZonalBands]float64, years int, from *ZonalYear, forcing float64) *ZonalYear {
	return solveZonalWith(ebmReference(landSeaExchange), land, sea, years, from, forcing)
}

// Mean is the year's mean at sea level at a latitude, land and sea together
// in the band's own shares.
func (z *ZonalYear) Mean(lat float64) float64 { return z.c.at(&z.c.mean, lat) }

// MeanLand and MeanSea are the year's mean over the land and over the sea of
// the band at a latitude.
func (z *ZonalYear) MeanLand(lat float64) float64 { return z.c.at(&z.c.meanL, lat) }
func (z *ZonalYear) MeanSea(lat float64) float64  { return z.c.at(&z.c.meanS, lat) }

// GlobalMean is the area mean of the year's mean over the planet: the bands
// are of equal area.
func (z *ZonalYear) GlobalMean() float64 {
	var s float64
	for k := range z.c.mean {
		s += z.c.mean[k]
	}
	return s / ebmBands
}

func solveZonalWith(p ebmParams, land, sea *[ebmBands]float64, years int, from *ZonalYear, forcing float64) *ZonalYear {
	a := olrA - forcing
	const n = ebmBands
	dx := 2.0 / n
	var x, phi [n]float64
	for k := range x {
		x[k] = -1 + (float64(k)+0.5)*dx
		phi[k] = math.Asin(x[k])
	}
	var face, faceSea [n + 1]float64
	for k := 1; k < n; k++ {
		xf := -1 + float64(k)*dx
		face[k] = p.d * (1 - xf*xf) / (dx * dx)
		faceSea[k] = p.ds * (1 - xf*xf) / (dx * dx)
	}
	// The sea's own carrying (M3, #22), as ebm.go has it: down the warmth of
	// its water, per square metre of sea, or what a planet's own sea brings
	// each band; across a face it is the two bands' sea's mean share.
	perFace := 2 * math.Pi * planetRadius * planetRadius * dx
	var flow [n + 1]float64
	var airNorth, seaNorth [n + 1]float64
	var seaIn [n]float64
	z := &ZonalYear{land: *land, sea: *sea}
	tl, ts, es := &z.tl, &z.ts, &z.es
	// The calendar's year, as ebm.go steps it since A1 (#71): clock.Year
	// days from the spring equinox, each a Year'th of secondsPerYear.
	days := float64(Year)
	dt := secondsPerYear / days / ebmSteps
	stepsYear := Year * ebmSteps
	out := &ebmClimate{params: p}
	var sumL, sumS, cL, sL, cS, sS [n]float64
	var eqSum, eqC, eqS [3]float64
	var band [n]float64
	sunDays := Year
	sunOf := make([]float64, sunDays*n)
	today := Today()
	for d := 0; d < sunDays; d++ {
		for k := 0; k < n; k++ {
			sunOf[d*n+k] = today.insolation(phi[k], float64(d)+0.5)
		}
	}
	var annualSun [n]float64
	for k := range tl {
		var q float64
		for d := 0; d < sunDays; d++ {
			q += sunOf[d*n+k] / days
		}
		annualSun[k] = q
		if from == nil {
			t0 := (q*(1-iceAlbedo(x[k], 10)) - a) / (olrB + 2*p.d)
			tl[k], ts[k], es[k] = t0, t0, t0*p.cs
		}
	}
	if from != nil {
		*tl, *ts, *es = from.tl, from.ts, from.es
	}
	for year := 0; year < years; year++ {
		last := year == years-1
		for s := 0; s < stepsYear; s++ {
			j := float64(s) / ebmSteps
			sun := sunOf[int(j)*n:]
			for k := range tl {
				fl := sun[k]*(1-iceAlbedoWith(p, x[k], tl[k])) - (a + olrB*tl[k]) + p.nu*sea[k]*(ts[k]-tl[k])
				alb := albedoIce
				if es[k] > 0 {
					alb = iceAlbedoWith(p, x[k], ts[k])
				}
				fs := sun[k]*(1-alb) - (a + olrB*ts[k]) + p.nu*land[k]*(tl[k]-ts[k])
				if es[k] < 0 {
					fs += seaIceBelow
				}
				tl[k] += dt * fl / p.cl
				es[k] += dt * fs
			}
			diffuseZonal(&face, tl, ts, es, land, sea, p, dt)
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
				ts[k] = seaSurfaceUnder(es[k], p.cs, sun[k], a)
			}
			if last {
				for k := 1; k < n; k++ {
					hs := moistEnergy(land[k-1]*tl[k-1] + sea[k-1]*ts[k-1])
					hn := moistEnergy(land[k]*tl[k] + sea[k]*ts[k])
					airNorth[k] += face[k] * (hs - hn) * perFace
					seaNorth[k] += (sea[k-1] + sea[k]) / 2 * flow[k] * perFace
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
					band[k] = land[k]*tl[k] + sea[k]*ts[k]
				}
				for q, field := range [3]*[n]float64{&band, tl, ts} {
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
	for k := range tl {
		out.meanL[k], out.meanS[k] = sumL[k]/m, sumS[k]/m
		out.mean[k] = land[k]*out.meanL[k] + sea[k]*out.meanS[k]
		ampLag := func(c, s float64) (float64, float64) {
			a, b := 2*c/m, 2*s/m
			amp := math.Hypot(a, b)
			peak := math.Atan2(b, a) / (2 * math.Pi) * days
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
	// The rest of the balance's reading as ebm.go has it since A1 (#71): the
	// warmest latitude's year, Held and Hou's contrast, the tropopause and
	// the land's and the sea's time constants.
	for q := range out.equator {
		a, b := 2*eqC[q]/m, 2*eqS[q]/m
		amp := math.Hypot(a, b)
		peak := math.Atan2(b, a) / (2 * math.Pi) * days
		lag := math.Mod(peak-days/4+2*days, days)
		if lag > days/2 {
			lag -= days
		}
		if math.Abs(lag) > days/4 {
			amp = -amp
			lag = math.Mod(lag+days/2+2*days, days)
			if lag > days/2 {
				lag -= days
			}
		}
		out.equator[q] = yearOf{mean: eqSum[q] / m, swing: amp, lag: lag}
	}
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
	z.c = out
	return z
}

// diffuseZonal is diffuse with the land and the sea of each band its own.
// Each band's heat capacity per degree of its mean is its own, and the heat
// that crosses a face is shared by the band's land and sea a square metre
// each, as in diffuse.
func diffuseZonal(face *[ebmBands + 1]float64, tl, ts, es, land, sea *[ebmBands]float64, p ebmParams, dt float64) {
	const n = ebmBands
	var gamma, h, slope, lo, mid, hi, rhs [n]float64
	for k := range h {
		gamma[k] = land[k]/p.cl + sea[k]/p.cs
		t := land[k]*tl[k] + sea[k]*ts[k]
		h[k] = moistEnergy(t)
		slope[k] = (moistEnergy(t+0.05) - moistEnergy(t-0.05)) / 0.1
	}
	for k := 0; k < n; k++ {
		mid[k] = 1
		if k > 0 {
			c := dt * gamma[k] * face[k]
			mid[k] += c * slope[k]
			lo[k] = -c * slope[k-1]
			rhs[k] += c * (h[k-1] - h[k])
		}
		if k < n-1 {
			c := dt * gamma[k] * face[k+1]
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
		heat := dT[k] / gamma[k]
		tl[k] += heat / p.cl
		es[k] += heat
	}
}

// seaSurfaceUnder is seaSurface with Budyko's A at a.
func seaSurfaceUnder(e, cs, sun, a float64) float64 {
	if e >= 0 {
		return e / cs
	}
	h := -e / seaIceLatent
	k := seaIceConduct / h
	t0 := (sun*(1-albedoIce) - a) / (olrB + k)
	return math.Min(0, t0)
}
