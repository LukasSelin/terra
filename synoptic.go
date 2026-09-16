package terra

import (
	"math"
	"math/rand/v2"
)

// The day's weather.
//
// The climate of the wind is what the wind is in an ordinary year, and no day
// is ordinary. What makes a day is the weather systems moving through it:
// the lows that form where warm air and cold meet, deepen for a day or two and
// fill as they are carried east by the westerlies aloft; the highs that sit
// between them; and, over warm tropical sea in its summer, the storms that
// form there, drift west and poleward on the trades, turn east again past the
// subtropical ridge, and die when they reach land or cold water.
//
// Each day the systems are moved on, aged and sometimes born, and the wind is
// worked out again from what they add to the climate's pressure, with the
// same balance and the same ground as the climate's own - so a low turns the
// right way in each hemisphere without being told, and a range turns its wind
// aside as it turns the climate's. And the air carries its warmth: where the
// day's wind blows harder from the pole or the equator than the climate's
// does, the air it brings is colder or warmer than the place, and it takes
// days to settle back. That warmth weighs on the pressure in its turn.
//
// A day's weather draws its chance from a stream of its own, tied to the
// seed and not to the world's chance, so that a game that never asks for the
// weather has the same history as one that does.

// SystemKind is what sort of weather system a System is.
type SystemKind uint8

const (
	// Low is a depression of the middle latitudes: a thousand kilometres
	// across, deepening for two days and filling for four.
	Low SystemKind = iota
	// High is an anticyclone: wider and shallower than a low, and slower.
	High
	// Storm is a tropical cyclone: a few hundred kilometres across and
	// far deeper than any low, living only over warm sea.
	Storm
)

// String names the kind of system.
func (k SystemKind) String() string {
	return [...]string{"low", "high", "storm"}[k]
}

// A System is one weather system: where it is, how deep it will get, how
// wide it is, and how far through its life it has come.
type System struct {
	Kind     SystemKind
	Lat, Lon float64 // degrees; a valley stands at longitude nought
	Depth    float64 // hPa it takes off or adds to the pressure at its fullest
	Radius   float64 // km from its middle to where its pressure is a part in root e of its middle's
	Age      float64 // days, or as good as days: a storm over land ages fast
	Life     float64 // days it lives
}

// Strength is how many hPa the system adds to the pressure at its middle
// today: less than nought for a low or a storm.
func (s System) Strength() float64 {
	t := s.Age / s.Life
	if t < 0 || t >= 1 {
		return 0
	}
	var f float64
	switch s.Kind {
	case High:
		f = math.Sin(math.Pi * t)
	default:
		// A third of its life deepening and the rest filling.
		if t < 1.0/3 {
			f = 3 * t
		} else {
			f = 1.5 * (1 - t)
		}
	}
	if s.Kind == High {
		return s.Depth * f
	}
	return -s.Depth * f
}

// Weather is the day's weather over a land: the systems moving through it,
// and the wind, the pressure and the warmth the air has with them in it.
type Weather struct {
	// Systems is every system on the planet, in the order they were born.
	Systems []System
	// Day is the tick the weather was last worked out for.
	Day int

	rng   *rand.Rand
	Winds *Winds // the climate the day's weather stands on
	Env   *Env
	// U, V and P are the day's wind toward the east and the north, in metres
	// a second, and pressure at sea level in hPa, on the air cells; warm is
	// how many degrees warmer than an ordinary day of the year the air there
	// is, for what the wind has carried in.
	U, V, P []float32
	warm    []float64
}

// The day's systems.
const (
	// lowsADay, highsADay and stormsADay are how many of each are born in a
	// hemisphere on an ordinary day. Lows live some six days, so a hemisphere
	// has a dozen at once, which is about what the real world's middle
	// latitudes carry; storms are born only in their hemisphere's summer, and
	// some eighty a year between the two is the real world's count.
	lowsADay   = 2.0
	highsADay  = 0.8
	stormsADay = 0.45
	// StormSea is the warmth, in degrees, the sea under a storm has to have
	// for it to be born or live: twenty-six and a half, the real threshold
	// (Gray, 1968; Dare and McBride, 2011). It used to be read against a
	// world whose tropics were seven degrees too cool, as a degree and a half
	// under its equator's mean; the energy balance gives the tropics their
	// real warmth, and the threshold its real figure.
	StormSea = 26.5
	// steerHeight is the height, in metres, of the wind that carries a system:
	// the 500 hPa level's, some five and a half kilometres up (the steering
	// level of both lows and tropical cyclones; Holton, 2004; Chan and Gray,
	// 1982). It is the wind near the ground and the thermal wind's shear over
	// that height, g/(f T) times the fall of the air's warmth across it.
	steerHeight = 5500.0
	// frontShare is how much of the steering wind a low or a high goes at.
	frontShare = 0.65
	// steerLeast is the latitude, in degrees, the turning of the planet is read
	// at no less than for the thermal wind: nearer the equator the balance does
	// not hold, and the steering is the trades'.
	steerLeast = 15.0
	// eadyN is N, the buoyancy frequency of the troposphere the Eady growth
	// rate is read at, per second, and eadyRef the growth rate, per second,
	// of the planet's own fall of warmth of seven tenths of a degree in a
	// hundred kilometres at 280 K: 0.31 g |∇T| / (N T) (Eady, 1949; Lindzen
	// and Farrell, 1980). A low is born where the rate is high.
	eadyN = 0.01
	// stormOutflow is T_o, the temperature in kelvin at which a tropical
	// cyclone's air flows out at the top, and stormExchange C_k/C_D, the
	// ratio of the sea's exchange of enthalpy to its drag (Emanuel, 1986;
	// Bister and Emanuel, 1998: 200 K and 0.9).
	stormOutflow  = 200.0
	stormExchange = 0.9
	// hollandB is the shape of a cyclone's pressure profile, and the
	// pressure it takes off for its wind is ρ e V² / B (Holland, 1980).
	hollandB = 1.5
	// decayRate, per hour, and decayFloor, m/s, are how a tropical cyclone's
	// wind falls over land: V = Vb + (V0 - Vb) exp(-α t) (Kaplan and DeMaria,
	// 1995: α = 0.095 h⁻¹, Vb = 26.7 kt).
	decayRate  = 0.095
	decayFloor = 13.7
	// drift is how fast, in metres a second, a low or a storm wanders toward
	// its pole, and a high toward the equator.
	drift = 1.5
	// warmTime is how many days the air takes to settle back to the warmth of
	// its place and season, and warmMost the most degrees it strays from it.
	warmTime = 5.0
	warmMost = 15.0
	// spinUp is how many days of systems a world's weather has behind it on
	// the first day it is asked for, so that its first day is not a clear one.
	spinUp = 20
	// kmADay is how many kilometres a metre a second is in a day.
	kmADay = 86.4
)

// weatherStream is what the seed is mixed with for the weather's chance.
const weatherStream = 0x5745415448455221

// YearSin is how far into the north's summer day is.
func YearSin(day int) float64 { return math.Sin(2 * math.Pi * float64(day) / Year) }

// lon is the longitude of cell cx on row cy.
func (e *Env) lon(cx, cy int) float64 {
	if e.Wrap {
		return 360*(float64(cx)+0.5)/float64(e.W) - 180
	}
	return (float64(cx) + 0.5 - float64(e.W)/2) * e.Dx[cy] / (111320 * math.Cos(e.lat[cy]*math.Pi/180))
}

// CellOf is where a latitude and longitude lie among the cells, in cells, and
// whether that is over the map at all.
func (e *Env) CellOf(lat, lon float64) (fx, fy float64, on bool) {
	if e.Wrap {
		fy = (90-lat)/180*float64(e.H) - 0.5
		fx = (wrapLon(lon)+180)/360*float64(e.W) - 0.5
		return fx, fy, true
	}
	mid := (e.lat[0] + e.lat[e.H-1]) / 2
	cy := float64(e.H)/2 - 0.5 - (lat-mid)*111195/e.Dy
	row := min(max(int(math.Round(cy)), 0), e.H-1)
	cx := float64(e.W)/2 - 0.5 + lon*111320*math.Cos(e.lat[row]*math.Pi/180)/e.Dx[row]
	return cx, cy, cx >= -0.5 && cx <= float64(e.W)-0.5 && cy >= -0.5 && cy <= float64(e.H)-0.5
}

// wrapLon is a longitude brought round into [-180, 180).
func wrapLon(lon float64) float64 {
	return math.Mod(math.Mod(lon+180, 360)+360, 360) - 180
}

// Step moves the systems on to day: carried by the air aloft, aged, the dead
// taken away, and the day's new ones born.
func (wx *Weather) Step(day int) {
	e := wx.Env
	sinT := YearSin(day)
	live := wx.Systems[:0]
	temp := e.blur(e.blur(e.AirTemp(sinT), synopticReach), synopticReach)
	for _, s := range wx.Systems {
		hemi := math.Copysign(1, s.Lat)
		east, north := wx.aloft(temp, s.Lat, s.Lon, day)
		if s.Kind != Storm {
			// The weather of the middle latitudes goes at some two thirds of
			// the wind at its steering level (Palmén and Newton, 1969;
			// Carlson, 1991); a tropical cyclone goes with it.
			east, north = east*frontShare, north*frontShare
		}
		pole := drift
		if s.Kind == High {
			pole = -drift / 3
		}
		s.Lat += (hemi*pole + north) * kmADay / 111.2
		s.Lat = math.Max(-89, math.Min(89, s.Lat))
		s.Lon = wrapLon(s.Lon + east*kmADay/(111.32*math.Max(0.1, math.Cos(s.Lat*math.Pi/180))))
		fx, fy, _ := e.CellOf(s.Lat, s.Lon)
		sea := e.Sample(e.Sea, fx, fy)
		s.Age++
		switch s.Kind {
		case Low:
			s.Age += 0.3 * (1 - sea)
		case Storm:
			// Over land the wind runs down as Kaplan and DeMaria found it
			// does, and a storm whose wind is down to its floor is gone.
			if land := 1 - sea; land > 0 {
				v := stormWindOf(s.Depth)
				v = decayFloor + (v-decayFloor)*math.Exp(-decayRate*24*land)
				s.Depth = stormDepthOf(v)
				if v < 1.3*decayFloor {
					s.Age = s.Life
				}
			}
			if e.SeaTemp(fx, fy, sinT) < StormSea {
				s.Age++
			}
		}
		if s.Age < s.Life && math.Abs(s.Lat) < 85 {
			live = append(live, s)
		}
	}
	wx.Systems = live

	r := wx.rng
	for _, hemi := range []float64{1, -1} {
		winter := -sinT * hemi
		for range poisson(r, lowsADay*(1+0.3*winter)) {
			lat, lon := wx.baroclinic(r, hemi, sinT)
			wx.Systems = append(wx.Systems, System{Kind: Low, Lat: lat, Lon: lon,
				Depth: 12 + 20*r.Float64(), Radius: 600 + 500*r.Float64(), Life: 4 + 4*r.Float64()})
		}
		for range poisson(r, highsADay) {
			wx.Systems = append(wx.Systems, System{Kind: High, Lat: hemi * (25 + 20*r.Float64()), Lon: 360*r.Float64() - 180,
				Depth: 6 + 10*r.Float64(), Radius: 1000 + 800*r.Float64(), Life: 5 + 5*r.Float64()})
		}
		if summer := -winter; summer > 0.2 {
			for range poisson(r, stormsADay*summer) {
				// Where the sea is warm enough, if it is found in a few
				// looks; a map with no warm sea has no storms.
				for range 12 {
					lat, lon := hemi*(7+13*r.Float64()), 360*r.Float64()-180
					fx, fy, on := e.CellOf(lat, lon)
					if sst := e.SeaTemp(fx, fy, sinT); on && e.Sample(e.Sea, fx, fy) > 0.8 && sst >= StormSea {
						// Few storms reach the most the sea could make of them:
						// the share they do is spread from a fifth to four fifths
						// (Emanuel, 2000).
						wx.Systems = append(wx.Systems, System{Kind: Storm, Lat: lat, Lon: lon,
							Depth: potentialDepth(sst) * (0.2 + 0.6*r.Float64()), Radius: 120 + 180*r.Float64(), Life: 6 + 6*r.Float64()})
						break
					}
				}
			}
		}
	}
}

// baroclinic is where a low is born in a hemisphere: somewhere in the middle
// latitudes, and more readily where the warmth of the air changes fastest
// from one place to the next, which is where lows get their energy - the
// polar front, and the edge of a continent in winter.
func (wx *Weather) baroclinic(r *rand.Rand, hemi, sinT float64) (lat, lon float64) {
	e := wx.Env
	for range 8 {
		lat, lon = hemi*(32+30*r.Float64()), 360*r.Float64()-180
		fx, fy, _ := e.CellOf(lat, lon)
		// How fast a wave on the front would grow here, by Eady's rate,
		// against how fast it grows under the planet's own fall of warmth.
		gx := (e.seaTempAt(fx+1, fy, sinT) - e.seaTempAt(fx-1, fy, sinT)) / (2 * e.Dx[e.row(fy)])
		gy := (e.seaTempAt(fx, fy-1, sinT) - e.seaTempAt(fx, fy+1, sinT)) / (2 * e.Dy)
		t := e.seaTempAt(fx, fy, sinT)
		if r.Float64() < math.Max(0.2, math.Min(1, eady(math.Hypot(gx, gy), t)/eady(0.7e-5, 7))) {
			return lat, lon
		}
	}
	return lat, lon
}

// eady is the Eady growth rate, per second, of a wave on air at temp degrees
// whose warmth falls grad degrees a metre: 0.31 f |∂u/∂z| / N, with the shear
// the thermal wind's, g |∇T| / (f T), so that f goes out.
func eady(grad, temp float64) float64 {
	return 0.31 * gravity * grad / (eadyN * (temp + 273.15))
}

// aloft is the wind, m/s toward the east and the north, that carries a
// system at lat, lon on day: the climate's wind near the ground there and the
// thermal wind over steerHeight, read off the warmth of the air near the
// ground, temp, blurred to the scale of the weather. A valley's air has the
// planet's fall of warmth across its latitude and not its own.
func (wx *Weather) aloft(temp []float64, lat, lon float64, day int) (east, north float64) {
	e := wx.Env
	fx, fy, _ := e.CellOf(lat, lon)
	for k, m := range seasonWeights(day) {
		east += m * e.Sample32(wx.Winds.U[k], fx, fy)
		north += m * e.Sample32(wx.Winds.V[k], fx, fy)
	}
	// The fall of warmth toward the pole, along the row: the planet's, which
	// is what the westerlies aloft stand on. A coast's contrast between land
	// and sea is a sea breeze's, and not the jet's.
	var gy float64
	if e.Wrap {
		cy := e.row(fy)
		north, south := max(cy-1, 0), min(cy+1, e.H-1)
		gy = (rowMean(temp, e.W, north) - rowMean(temp, e.W, south)) / (float64(south-north) * e.Dy)
	} else {
		gy = (ZonalMean(lat+0.5) - ZonalMean(lat-0.5)) / 111195
	}
	// The balance holds poleward of the tropics, and comes in over steerLeast
	// to twice that.
	a := math.Max(math.Abs(lat), steerLeast)
	f := 2 * omega * math.Sin(a*math.Pi/180)
	shear := gravity / (f * (e.Sample(temp, fx, fy) + 273.15)) * steerHeight * smoothstep(steerLeast, 2*steerLeast, math.Abs(lat))
	return east - math.Copysign(shear, lat)*gy, north
}

// rowMean is the mean of v over row cy of a lattice w cells across.
func rowMean(v []float64, w, cy int) float64 {
	var s float64
	for _, x := range v[cy*w : (cy+1)*w] {
		s += x
	}
	return s / float64(w)
}

// potentialDepth is how many hPa the deepest tropical cyclone warm sea at sst
// degrees could make takes off the pressure: the wind of Emanuel's potential
// intensity, V² = C_k/C_D (T_s - T_o)/T_o L (q*_s - q), with the air over the
// sea a degree cooler and four fifths saturated, turned to a fall of pressure
// by Holland's profile.
func potentialDepth(sst float64) float64 {
	ts := sst + 273.15
	dq := saturation(sst) - 0.8*saturation(sst-1)
	v2 := stormExchange * (ts - stormOutflow) / stormOutflow * latentHeat * math.Max(0, dq)
	return stormDepthOf(math.Sqrt(v2))
}

// stormDepthOf is the fall of pressure, hPa, of a tropical cyclone whose
// wind is v m/s, and stormWindOf its inverse: Holland's ρ e V² / B.
func stormDepthOf(v float64) float64 { return airDensity * math.E * v * v / hollandB / 100 }

func stormWindOf(depth float64) float64 {
	return math.Sqrt(math.Max(0, depth) * 100 * hollandB / (airDensity * math.E))
}

// row is the row of cells nearest fy, held on the map.
func (e *Env) row(fy float64) int { return min(max(int(math.Round(fy)), 0), e.H-1) }

// seaTempAt is the temperature of the air at sea level at a place among the
// cells, in an ordinary year sinT of the way into the north's summer.
func (e *Env) seaTempAt(fx, fy, sinT float64) float64 {
	cy := e.row(fy)
	cont := e.Sample(e.Cont, fx, fy)
	t := e.Mean[cy] + seasonTemp(e.hemi[cy], sinT, cont)
	if e.Coast != nil {
		t += e.Sample(e.Coast, fx, fy)
	}
	return t
}

// SeaTemp is the warmth of the sea at a place among the cells: the air's over
// it, with the sea's own small swing and what the currents have brought.
func (e *Env) SeaTemp(fx, fy, sinT float64) float64 {
	cy := e.row(fy)
	t := e.Mean[cy] + seasonTemp(e.hemi[cy], sinT, 0) + seaOverAir
	if e.Warm != nil {
		t += e.Sample(e.Warm, fx, fy)
	}
	return t
}

// seaOverAir is how many degrees the sea's surface stands over the air just
// above it: the air is warmed from the sea, and over the open ocean the sea
// is some one degree the warmer (Kara, Wallcraft and Hurlburt, 2007).
const seaOverAir = 1.0

// smoothstep is 0 below lo, 1 above hi, and a smooth step between.
func smoothstep(lo, hi, x float64) float64 {
	t := math.Max(0, math.Min(1, (x-lo)/(hi-lo)))
	return t * t * (3 - 2*t)
}

// poisson is a count drawn from a Poisson distribution of mean m.
func poisson(r *rand.Rand, m float64) int {
	limit, k, p := math.Exp(-m), 0, r.Float64()
	for p > limit {
		k++
		p *= r.Float64()
	}
	return k
}

// Solve works the day's wind out: the climate's pressure for the day with
// the systems added, and the warmth the air has carried in since yesterday.
func (wx *Weather) Solve(day int) {
	e := wx.Env
	n := e.W * e.H
	sinT := YearSin(day)
	if len(wx.U) != n {
		wx.U, wx.V, wx.P = make([]float32, n), make([]float32, n), make([]float32, n)
		wx.warm = make([]float64, n)
	}
	wx.carry(day)

	extra := make([]float64, n)
	for _, s := range wx.Systems {
		depth := s.Strength()
		if depth == 0 {
			continue
		}
		reach := 3 * s.Radius
		dlat := reach / 111.2
		for cy := 0; cy < e.H; cy++ {
			if math.Abs(e.lat[cy]-s.Lat) > dlat {
				continue
			}
			ky := (e.lat[cy] - s.Lat) * 111.2
			coslat := math.Cos((e.lat[cy] + s.Lat) / 2 * math.Pi / 180)
			for cx := 0; cx < e.W; cx++ {
				kx := wrapLon(e.lon(cx, cy)-s.Lon) * 111.32 * coslat
				if math.Abs(kx) > reach {
					continue
				}
				extra[cy*e.W+cx] += depth * math.Exp(-(kx*kx+ky*ky)/(2*s.Radius*s.Radius))
			}
		}
	}
	e.Solve(sinT, e.airTempOn(day), extra, wx.warm, wx.U, wx.V, wx.P)
}

// carry moves the warmth of the air on by a day of yesterday's wind, and lets
// it settle toward the warmth of the place. What the wind carries in is not
// the warmth the climate's own wind would have brought - that is already the
// place's - but what it brings over and above it.
func (wx *Weather) carry(day int) {
	e := wx.Env
	n := e.W * e.H
	clim := e.airTempOn(day)
	// The climate's own wind today, to be taken from the day's.
	cu, cv := make([]float32, n), make([]float32, n)
	for k, m := range seasonWeights(day) {
		for i := range cu {
			cu[i] += float32(m) * wx.Winds.U[k][i]
			cv[i] += float32(m) * wx.Winds.V[k][i]
		}
	}
	next := make([]float64, n)
	keep := math.Exp(-1 / warmTime)
	e.rows(func(cy int) {
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			du, dv := float64(wx.U[i]-cu[i]), float64(wx.V[i]-cv[i])
			if wx.P[i] == 0 {
				du, dv = 0, 0 // no day has been worked out yet
			}
			// Where the air over this cell was a day ago, in cells.
			fx := float64(cx) - du*86400/e.Dx[cy]
			fy := float64(cy) + dv*86400/e.Dy
			w := e.Sample(wx.warm, fx, fy) + e.Sample(clim, fx, fy) - clim[i]
			next[i] = math.Max(-warmMost, math.Min(warmMost, w*keep))
		}
	})
	wx.warm = next
}

// NewWeather is the day's weather over the winds w, on the day before day,
// for a world of seed: with some weeks of systems already behind it, or,
// where was is the weather the ground stood under before it changed, with
// was's systems and its chance, and the air started over.
func NewWeather(seed uint64, w *Winds, day int, was *Weather) *Weather {
	wx := StillWeather(rand.New(rand.NewPCG(seed^weatherStream, seed*0x9E3779B97F4A7C15+weatherStream)), w)
	wx.Day = day - 1
	if was != nil {
		// The ground has changed under the weather: keep its systems
		// and its chance, and start the air over.
		wx.rng, wx.Systems = was.rng, was.Systems
	} else {
		for d := spinUp; d > 0; d-- {
			wx.Step(day - d)
		}
	}
	return wx
}

// StillWeather is the weather over the winds w drawing its chance from rng,
// with no systems in it and no days behind it.
func StillWeather(rng *rand.Rand, w *Winds) *Weather {
	return &Weather{rng: rng, Winds: w, Env: w.Env}
}

// Over reports whether the weather stands Over the winds w: whether it was
// worked out for the ground they were.
func (wx *Weather) Over(w *Winds) bool {
	return wx != nil && w != nil && wx.Env == w.Env
}

// Advance moves the weather on to day.
func (wx *Weather) Advance(day int) {
	wx.Step(day)
	wx.Solve(day)
	wx.Day = day
}

// Worked reports whether the day's air has been Worked out at all.
func (wx *Weather) Worked() bool { return len(wx.U) > 0 }

// sampleTile reads a field of the day's weather at tile i.
func (wx *Weather) sampleTile(field []float32, i int) float64 {
	e := wx.Env
	fx, fy := e.CellAt(i)
	return e.Sample32(field, fx, fy)
}

// WindAt, pressureAt and warmthAt are the day's wind, pressure and warmth at
// tile i: see Land.WindAt, Land.PressureAt and Land.WarmthAt.
func (wx *Weather) WindAt(i int) (east, north float64) {
	return wx.sampleTile(wx.U, i), wx.sampleTile(wx.V, i)
}

func (wx *Weather) PressureAt(i int) float64 { return wx.sampleTile(wx.P, i) }

func (wx *Weather) WarmthAt(i int) float64 {
	e := wx.Env
	fx, fy := e.CellAt(i)
	return e.Sample(wx.warm, fx, fy)
}

// Gust is how hard a wind of speed s gusts over tile i: see Land.GustAt.
func (e *Env) Gust(i int, s float64) float64 {
	fx, fy := e.CellAt(i)
	land := 1 - e.Sample(e.Sea, fx, fy)
	rough := math.Min(1, e.Sample(e.rough, fx, fy)/dragRough)
	return s * (1.35 + land*(0.15+0.3*rough))
}

// Place is where a system at lat, lon stands on the map, in tiles, and
// whether that is on the map at all: see Land.Place.
func (e *Env) Place(lat, lon float64) (x, y float64, on bool) {
	fx, fy, on := e.CellOf(lat, lon)
	k := float64(e.Cell)
	return (fx + 0.5) * k, (fy + 0.5) * k, on
}
