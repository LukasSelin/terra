package atmos

import (
	"math"

	"github.com/LukasSelin/terra/internal/phase"
)

// The waves the land and its heat stand in the air.
//
// The belts lay the pressure down the same all the way round each parallel,
// and the warmth of the air near the ground makes its thermal lows and highs
// over a continent in its summer and its winter (Solve); the sea's warmth
// along the equator makes the Walker circulation (coupled.go). Nothing else
// told the air one longitude from another: nothing bent the westerlies round
// a plateau, and the rain of a monsoon let go of its heat into nothing. The
// real climate owes much of its difference from east to west to the two
// answers worked out here, both linear and steady, both on the air's
// lattice, and both added to what the belts lay down.
//
//   - The tropics' answer to their heating (Gill, 1980), as the warm pool's
//     is worked out in coupled.go, but to the heat the ground gives the air
//     over the land and the sea alike: the latent heat of the water it sends
//     up, as the air's budget last took it up, and the heat the land gives
//     the air in its summer and takes from it in its winter. Where the air
//     convects it spreads what comes up from the ground through the column,
//     and it is that, the energy coming into the column, and not the rain it
//     lets fall, that the tropical circulation answers (Neelin and Held,
//     1987): the rain is mostly the water the circulation gathers in, its
//     own doing. Heated by its own rain, the air ran away from the ground
//     under it: a continent on the equator a little drier than the sea
//     beside it drew the air out, and rained less, and drew more out, reading
//     after reading, from eight millimetres a day to half of one. A
//     continent hot in its summer draws the sea's air in under a low
//     reaching west of it, and the easterlies of its Kelvin wave run along
//     the equator east of it; a continent heated more than the sea on the
//     equator is a Walker cell's rising branch whatever the sea east of it
//     does (Webster, 1972; Gill, 1980). Its wind and its pressure are added to
//     the wind the ground has had its say over, as the Walker
//     circulation's are: near the equator the planet's turning cannot hold a
//     pressure in balance, and the wind is the heating's own.
//   - The waves the westerlies stand downstream of a plateau and of the
//     heating (Hoskins and Karoly, 1981; Held, Ting and Wang, 2002): the
//     barotropic vorticity equation about the zonal mean wind aloft,
//     linearised, damped and steady. Air blowing up the windward side of a
//     range is squeezed under the tropopause and turns anticyclonically over
//     it, and comes off the lee side stretched and turning the other way: a
//     ridge over the plateau and a trough downstream of it, the East Asian
//     trough east of Tibet and the North American one east of the Rockies.
//     The heating sends the air aloft out of it, and the planet's turning
//     makes a vortex of that outflow (the Rossby wave source of Sardeshmukh
//     and Hoskins, 1988). The answer aloft reaches the ground as the
//     equivalent barotropic structure of the real stationary waves does,
//     weaker in the share the wind near the ground bears to the wind aloft,
//     and its pressure is added to the belts before the wind is worked out
//     from them, so the ground has its say over the wind it drives.
//
// Both are worked out once a reading, for each phase of the year, from the
// water the air's budget took up the last time the weather was read over the
// same map: what the ground gives the air is the air's own answer to the
// wind, and so it is read a reading late, as the coupled solve reads the sea
// (carry). A map read for the first time has the land's warmth and the
// ground's waves, and no water's.
// Each is solved exactly (Thomas's algorithm down the rows for each
// wavenumber round the parallels), every sum in one order, so the world is
// the same on any number of goroutines. It is done only on a globe; a valley
// is untouched to the bit.

// The heating.
const (
	// sensibleExchange is how much heat, in W/m², a degree's difference
	// gives the air over land: ρ c_p C_H |U|, a bulk exchange coefficient of
	// 1.5e-3 under a wind of five metres a second (Garratt, 1992). The land
	// stands over its air in its summer and under it in its winter by about
	// as much as the continent's air stands over or under its row's, which
	// is the difference it is read off.
	sensibleExchange = airDensity * heatAir * 1.5e-3 * 5
)

// The waves aloft.
const (
	// waveLevel is the pressure, hPa, of the level the waves are worked out
	// on: the upper troposphere, where the stationary waves are strongest
	// and the heating's outflow is (Sardeshmukh and Hoskins, 1988).
	waveLevel = 300.0
	// waveFrom and waveTo bound, in degrees from the equator, the band of
	// each hemisphere the waves are worked out over: from where the planet's
	// turning first holds a wind in balance with the pressure, to short of
	// the pole. Nothing crosses either edge.
	waveFrom, waveTo = 15.0, 80.0
	// waveDamp is how fast, per second, the waves are damped: in five days,
	// the order the linear stationary-wave models take, which lets a wave
	// run some thousands of kilometres downstream in the westerlies before
	// it dies.
	waveDamp = 1 / (5 * 86400.0)
)

// gillPerRain is the heating of Gill's equations, m² s⁻³, of rain falling
// at a kilogram a square metre a second: its latent heat spread over the
// column, as the first baroclinic mode feels it (see rainHeat).
var gillPerRain = latentHeat / (heatAir * columnMass) * dryGas * math.Log(beltMean/convectionTop) / 2

// Wave is what the heating and the ground add to the air in a phase of the
// year, on each air cell.
type Wave struct {
	// U and V are the wind toward the east and the north, metres a second,
	// and P the pressure, hPa, of Gill's answer to the heating: added to the
	// wind the ground has had its say over, as the Walker circulation's is.
	U, V, P []float32
	// Aloft is how far, in metres, the waveLevel surface stands over its
	// row's mean on each cell: low in a trough and high in a ridge.
	Aloft []float32
	// share is how much of the waves aloft reaches the ground on each row:
	// the zonal mean wind near the ground over the wind aloft, as the real
	// stationary waves' equivalent barotropic structure has it, between
	// nought and one.
	share []float64
}

// waves works the Wave of each phase of the year out into e.Waves, from the
// water the air's budget took up when the wind was, was, worked out over
// the same lattice of cells, or none. It is worked out in s (see Scratch),
// before the wind is: what it leaves in the phases' memory the wind does
// not read.
func (e *Env) waves(was *Winds, s *Scratch) {
	defer phase.Start("airEnv.waves")()
	n := e.W * e.H
	var budget *[Phases]vapourOut
	if was != nil && was.Env != nil && was.W == e.W && was.H == e.H && len(was.Budget[1].Rain) == n {
		budget = &was.Budget
	}
	sh := s.sharedWork()
	height := e.blurIn(sh, slotWaveHeight, e.blurIn(sh, slotWaveBlur, e.Height, synopticReach), synopticReach)
	workers := 1
	if n >= spreadTiles {
		workers = workersFor(Phases)
	}
	inParallel(Phases-1, workers, func(k, _ int) {
		wk := s.phaseWork(k)
		temp := e.airTempIn(wk, slotAirTemp, phaseSin[k])
		var evap []float64
		if budget != nil {
			evap = budget[k].Evap
		}
		q := e.heating(temp, evap, e.Subsides[k], wk)
		phi, gu, gv := e.gill(q, wk)
		w := &e.Waves[k]
		w.U, w.V, w.P, w.Aloft = make([]float32, n), make([]float32, n), make([]float32, n), make([]float32, n)
		for i := range n {
			w.U[i], w.V[i] = float32(gu[i]), float32(gv[i])
			w.P[i] = float32(airDensity * phi[i] / 100)
		}
		psi := e.stationary(phaseSin[k], temp, height, gu, gv, w, wk)
		for cy := range e.H {
			for cx := range e.W {
				i := cy*e.W + cx
				w.Aloft[i] = float32(e.f[cy] * psi[i] / gravity)
			}
		}
	})
	e.Waves[3] = e.Waves[1]
}

// heating is Gill's heating on each cell, m² s⁻³, over the warmth of the air
// temp of a phase and the water evap, kg/m² a second, the air's budget took
// up in it, or none: the energy the ground gives the column. On land it is
// the latent heat of what the land sends up and the heat it gives the air,
// sensibleExchange for each degree its air stands over its row's; over the
// sea the latent heat of what the row's sea sends up on its mean, whose
// warmth along the row is the Walker circulation's (walker). The latent
// heat is the column's only where the air rises, out of reach of the
// Hadley cell's descent (Subsides, of the phase); the land's own heat warms
// the air over it wherever it is. Each row's
// mean is taken off, which is the belts', and the whole of it is held to
// the tropics, as the Walker circulation's is.
func (e *Env) heating(temp, evap, descent []float64, wk *work) []float64 {
	q := wk.floats(slotHeat, e.W*e.H)
	for cy := 0; cy < e.H; cy++ {
		share := e.tropicShare(cy)
		if share == 0 {
			continue
		}
		row := cy * e.W
		// Only where the air rises does it carry the water's heat up
		// through the column: under the Hadley cell's descent what the sea
		// sends up is carried off to the ITCZ, and rains there.
		deep := 1 - clamp01(descent[row]/subsideMost)
		var tz, se, sk float64
		for cx := 0; cx < e.W; cx++ {
			i := row + cx
			tz += temp[i]
			if evap != nil {
				se, sk = se+e.Sea[i]*evap[i], sk+e.Sea[i]
			}
		}
		tz /= float64(e.W)
		var sea float64
		if sk > 0 {
			sea = se / sk
		}
		var mean float64
		for cx := 0; cx < e.W; cx++ {
			i := row + cx
			land := 1 - e.Sea[i]
			wet := sea * e.Sea[i]
			if evap != nil {
				wet += land * evap[i]
			}
			heat := land * sensibleExchange * (temp[i] - tz) / latentHeat
			q[i] = share * gillPerRain * (deep*wet + heat)
			mean += q[i]
		}
		mean /= float64(e.W)
		for cx := 0; cx < e.W; cx++ {
			q[row+cx] -= mean
		}
	}
	return q
}

// stationary is the streamfunction, m²/s, of the waves the westerlies stand
// at waveLevel in a phase sinT of the way into the north's summer, over air
// near the ground of temp degrees, ground of height metres, smoothed, and
// Gill's wind gu, gv near the ground; and the share of them each row's
// ground feels, into w. The equation is the barotropic vorticity equation's,
// linearised about the zonal mean wind aloft U, damped at r and steady:
//
//	(U ∂x + r) ∇²ψ + β* ∂xψ = −(f/H) U₀ ∂h/∂x + f D
//
// with β* = β − ∂²U/∂y², U₀ the zonal mean wind near the ground, H the
// scale height of the air, and D the divergence of the heating's wind near
// the ground, which is the convergence of its outflow aloft. U is U₀ and
// the thermal wind over the column to waveLevel, read off the fall of the
// zonal mean warmth across the rows; U₀ is the belts' geostrophic wind.
//
// Along the rows its coefficients are the same at every column, so each
// wavenumber round the parallels is a three-banded system down the rows of
// its own, as Gill's is (gill), solved exactly; each hemisphere's band is
// solved on its own, with nothing crossing its edges, and the zonal mean,
// which is the belts', is left out.
func (e *Env) stationary(sinT float64, temp, height, gu, gv []float64, w *Wave, wk *work) []float64 {
	n := e.W * e.H
	psi := wk.floats(slotWavePsi, n)
	w.share = make([]float64, e.H)
	if !e.Wrap {
		return psi
	}
	b := e.beltsAt(sinT)
	// The zonal means of each row: the warmth and the belts' pressure, and
	// from them the winds.
	tz, pz := make([]float64, e.H), make([]float64, e.H)
	for cy := range e.H {
		var s float64
		for cx := range e.W {
			s += temp[cy*e.W+cx]
		}
		tz[cy] = s / float64(e.W)
		pz[cy] = b.pressure(e.lat[cy], sinT)
	}
	thick := dryGas * math.Log(beltMean/waveLevel)
	ground, aloft := make([]float64, e.H), make([]float64, e.H)
	for cy := range e.H {
		f := e.f[cy]
		if math.Abs(e.lat[cy]) < waveFrom-5 || cy == 0 || cy == e.H-1 {
			continue
		}
		// North is the row before.
		dp := (pz[cy-1] - pz[cy+1]) * 100 / (2 * e.Dy)
		dt := (tz[cy-1] - tz[cy+1]) / (2 * e.Dy)
		ground[cy] = -dp / (airDensity * f)
		aloft[cy] = ground[cy] - thick*dt/f
	}
	force := wk.floats(slotWaveForce, n)
	for _, band := range e.waveBands() {
		lo, hi := band[0], band[1]
		// The winds, smoothed down the rows, since they are read off
		// differences and their curvature is wanted.
		smoothRows(ground, lo, hi)
		smoothRows(aloft, lo, hi)
		for cy := lo; cy <= hi; cy++ {
			if aloft[cy] > 0 {
				w.share[cy] = clamp01(ground[cy] / aloft[cy])
			}
			f := e.f[cy]
			scale := dryGas * (tz[cy] + 273.15) / gravity
			row := cy * e.W
			for cx := range e.W {
				i := row + cx
				hx := (height[e.at(cx+1, cy)] - height[e.at(cx-1, cy)]) / (2 * e.Dx[cy])
				force[i] = -f/scale*ground[cy]*hx + f*e.div(gu, gv, cx, cy)
			}
		}
		e.vorticity(lo, hi, aloft, force, psi, wk)
	}
	return psi
}

// waveBands is the rows of each hemisphere's band the waves are worked out
// over, first and last: the north's and the south's, where there are any.
func (e *Env) waveBands() [][2]int {
	var bands [][2]int
	for _, sign := range []float64{1, -1} {
		lo, hi := -1, -1
		for cy := range e.H {
			if l := e.lat[cy] * sign; l >= waveFrom && l <= waveTo {
				if lo < 0 {
					lo = cy
				}
				hi = cy
			}
		}
		if lo >= 0 && hi > lo {
			bands = append(bands, [2]int{lo, hi})
		}
	}
	return bands
}

// smoothRows is v between rows lo and hi, its running mean over three rows
// taken twice, held at the ends.
func smoothRows(v []float64, lo, hi int) {
	for range 2 {
		prev := v[lo]
		for cy := lo; cy <= hi; cy++ {
			next := v[min(cy+1, hi)]
			here := v[cy]
			v[cy] = (prev + here + next) / 3
			prev = here
		}
	}
}

// vorticity solves the damped barotropic vorticity equation (stationary) on
// rows lo to hi for the streamfunction, into psi, under the zonal mean wind
// u of each row and the forcing force.
func (e *Env) vorticity(lo, hi int, u, force, psi []float64, wk *work) {
	rows, w := hi-lo+1, e.W
	second, first := make([]complex128, w), make([]complex128, w)
	second[0], second[1], second[w-1] = -2, 1, 1
	first[1], first[w-1] = -0.5, 0.5
	transform(second, false)
	transform(first, false)

	spec := make([][]complex128, rows)
	room := wk.complexes(rows * w)
	for r := range spec {
		cy := lo + r
		spec[r] = room[r*w : (r+1)*w : (r+1)*w]
		for cx := range w {
			spec[r][cx] = complex(force[cy*w+cx], 0)
		}
		transform(spec[r], false)
	}
	dy2 := e.Dy * e.Dy
	// β*, the fall of the absolute vorticity of the zonal mean flow toward
	// the south, on each row: north is the row before.
	beta := make([]float64, rows)
	for r := range rows {
		cy := lo + r
		up, down := max(cy-1, lo), min(cy+1, hi)
		beta[r] = (e.f[max(cy-1, 0)]-e.f[min(cy+1, e.H-1)])/(2*e.Dy) -
			(u[up]-2*u[cy]+u[down])/dy2
	}
	sub, diag, sup := make([]complex128, rows), make([]complex128, rows), make([]complex128, rows)
	rhs, c := make([]complex128, rows), make([]complex128, rows)
	for k := 1; k < w; k++ {
		for r := range rows {
			cy := lo + r
			dx := e.Dx[cy]
			// The faces' breadths: the Laplacian on the sphere, as the flux
			// between rows through faces as broad as their parallels.
			fn := (e.Dx[cy] + e.Dx[max(cy-1, 0)]) / 2 / (dx * dy2)
			fs := (e.Dx[cy] + e.Dx[min(cy+1, e.H-1)]) / 2 / (dx * dy2)
			d1 := first[k] / complex(dx, 0)
			d2 := second[k] / complex(dx*dx, 0)
			a := complex(u[cy], 0)*d1 + complex(waveDamp, 0)
			diag[r] = a*(d2-complex(fn+fs, 0)) + complex(beta[r], 0)*d1
			sub[r] = a * complex(fn, 0) // the row before
			sup[r] = a * complex(fs, 0) // the row after
			rhs[r] = spec[r][k]
		}
		thomas(sub, diag, sup, rhs, c)
		for r := range rows {
			spec[r][k] = rhs[r]
		}
	}
	for r := range rows {
		spec[r][0] = 0
		transform(spec[r], true)
		cy := lo + r
		for cx := range w {
			psi[cy*w+cx] = real(spec[r][cx])
		}
	}
}

// waveWeights is how much each phase's waves are worth sinT of the way into
// the north's summer: the equinox's and the solstice's on that side, in
// proportion.
func waveWeights(sinT float64) [Phases]float64 {
	var m [Phases]float64
	if sinT >= 0 {
		m[1], m[2] = 1-sinT, sinT
	} else {
		m[1], m[0] = 1+sinT, -sinT
	}
	return m
}

// addWaves adds what the waves bring to the pressure before the wind is
// worked out (before) or to the wind and the pressure after (after), the
// phases' in the proportions m. Nothing where there are none.
func (e *Env) addWaves(m *[Phases]float64, pres []float64, wind *[2][]float64, before bool) {
	if m == nil || e.Waves[0].U == nil {
		return
	}
	for k := range Phases - 1 {
		wt := m[k]
		if k == 1 {
			wt += m[3]
		}
		if wt == 0 {
			continue
		}
		wv := &e.Waves[k]
		if before {
			for cy := range e.H {
				s := wt * airDensity * gravity * wv.share[cy] / 100
				if s == 0 {
					continue
				}
				row := cy * e.W
				for cx := range e.W {
					pres[row+cx] += s * float64(wv.Aloft[row+cx])
				}
			}
			continue
		}
		for i := range pres {
			wind[0][i] += wt * float64(wv.U[i])
			wind[1][i] += wt * float64(wv.V[i])
			pres[i] += wt * float64(wv.P[i])
		}
	}
}
