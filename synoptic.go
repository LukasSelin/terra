package terra

import (
	"math"
	"math/rand/v2"

	"github.com/LukasSelin/terra/geom"
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
	winds *Winds // the climate the day's weather stands on
	env   *airEnv
	// u, v and p are the day's wind toward the east and the north, in metres
	// a second, and pressure at sea level in hPa, on the air cells; warm is
	// how many degrees warmer than an ordinary day of the year the air there
	// is, for what the wind has carried in.
	u, v, p []float32
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
	// stormSea is the warmth, in degrees, the sea under a storm has to have
	// for it to be born or live. The real threshold is twenty-six and a half,
	// a couple of degrees under the warmest open ocean; a world here has
	// cooler tropics than that - see LatSwing - so the threshold is read the
	// same way against its own: a degree and a half under the year's mean at
	// the equator. In their summer the storms live out to some thirty
	// degrees, which is where the real ones die too.
	stormSea = MeanTemp + LatSwing*(1-math.Sqrt2/2) - 1.5
	// westerlyAloft is how fast, in metres a second, the westerlies aloft
	// carry a system east in the middle latitudes, and tradesAloft how fast
	// the trades carry one west. A third again in winter and a third less in
	// summer, when the difference in warmth between pole and equator that
	// drives them is greater and less.
	westerlyAloft = 10.0
	tradesAloft   = 5.0
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

// AdvanceWeather moves the day's weather on to today, w.Tick. A game calls it
// once a day, beside Climate.Advance; nothing about the land needs it to have
// been called, and it draws nothing from the world's chance. The first call
// starts the weather with some weeks of systems already behind it.
func (w *Land) AdvanceWeather() {
	g := w.Grid
	if g.winds == nil {
		g.weather()
	}
	if w.Weather == nil || w.Weather.env != g.winds.airEnv {
		wx := &Weather{
			rng:   rand.New(rand.NewPCG(w.seed^weatherStream, w.seed*0x9E3779B97F4A7C15+weatherStream)),
			winds: g.winds,
			env:   g.winds.airEnv,
			Day:   w.Tick - 1,
		}
		if w.Weather != nil {
			// The ground has changed under the weather: keep its systems
			// and its chance, and start the air over.
			wx.rng, wx.Systems = w.Weather.rng, w.Weather.Systems
		} else {
			for d := spinUp; d > 0; d-- {
				wx.step(w.Tick - d)
			}
		}
		w.Weather = wx
	}
	wx := w.Weather
	wx.step(w.Tick)
	wx.solve(w.Tick)
	wx.Day = w.Tick
}

// yearSin is how far into the north's summer day is.
func yearSin(day int) float64 { return math.Sin(2 * math.Pi * float64(day) / Year) }

// lon is the longitude of cell cx on row cy.
func (e *airEnv) lon(cx, cy int) float64 {
	if e.wrap {
		return 360*(float64(cx)+0.5)/float64(e.w) - 180
	}
	return (float64(cx) + 0.5 - float64(e.w)/2) * e.dx[cy] / (111320 * math.Cos(e.lat[cy]*math.Pi/180))
}

// cellOf is where a latitude and longitude lie among the cells, in cells, and
// whether that is over the map at all.
func (e *airEnv) cellOf(lat, lon float64) (fx, fy float64, on bool) {
	if e.wrap {
		fy = (90-lat)/180*float64(e.h) - 0.5
		fx = (wrapLon(lon)+180)/360*float64(e.w) - 0.5
		return fx, fy, true
	}
	mid := (e.lat[0] + e.lat[e.h-1]) / 2
	cy := float64(e.h)/2 - 0.5 - (lat-mid)*111195/e.dy
	row := min(max(int(math.Round(cy)), 0), e.h-1)
	cx := float64(e.w)/2 - 0.5 + lon*111320*math.Cos(e.lat[row]*math.Pi/180)/e.dx[row]
	return cx, cy, cx >= -0.5 && cx <= float64(e.w)-0.5 && cy >= -0.5 && cy <= float64(e.h)-0.5
}

// wrapLon is a longitude brought round into [-180, 180).
func wrapLon(lon float64) float64 {
	return math.Mod(math.Mod(lon+180, 360)+360, 360) - 180
}

// step moves the systems on to day: carried by the air aloft, aged, the dead
// taken away, and the day's new ones born.
func (wx *Weather) step(day int) {
	e := wx.env
	sinT := yearSin(day)
	live := wx.Systems[:0]
	for _, s := range wx.Systems {
		hemi := math.Copysign(1, s.Lat)
		winter := -sinT * hemi
		a := math.Abs(s.Lat)
		east := -tradesAloft + (tradesAloft+westerlyAloft*(1+winter/3))*smoothstep(18, 35, a) - 0.6*westerlyAloft*smoothstep(62, 80, a)
		pole := drift
		if s.Kind == High {
			pole = -drift / 3
		}
		s.Lat += hemi * pole * kmADay / 111.2
		s.Lon = wrapLon(s.Lon + east*kmADay/(111.32*math.Max(0.1, math.Cos(s.Lat*math.Pi/180))))
		fx, fy, _ := e.cellOf(s.Lat, s.Lon)
		sea := e.sample(e.sea, fx, fy)
		s.Age++
		switch s.Kind {
		case Low:
			s.Age += 0.3 * (1 - sea)
		case Storm:
			s.Age += 2 * (1 - sea)
			if e.seaTemp(fx, fy, sinT) < stormSea {
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
					fx, fy, on := e.cellOf(lat, lon)
					if on && e.sample(e.sea, fx, fy) > 0.8 && e.seaTemp(fx, fy, sinT) >= stormSea {
						wx.Systems = append(wx.Systems, System{Kind: Storm, Lat: lat, Lon: lon,
							Depth: 25 + 45*r.Float64(), Radius: 120 + 180*r.Float64(), Life: 6 + 6*r.Float64()})
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
	e := wx.env
	for range 8 {
		lat, lon = hemi*(32+30*r.Float64()), 360*r.Float64()-180
		fx, fy, _ := e.cellOf(lat, lon)
		// How fast the warmth changes over a cell, against the planet's own
		// pole-to-equator fall of some seven tenths of a degree in a hundred
		// kilometres.
		dt := math.Abs(e.seaTempAt(fx+1, fy, sinT)-e.seaTempAt(fx-1, fy, sinT))/(2*e.dx[e.row(fy)]) +
			math.Abs(e.seaTempAt(fx, fy-1, sinT)-e.seaTempAt(fx, fy+1, sinT))/(2*e.dy)
		if r.Float64() < math.Max(0.2, math.Min(1, dt*1e5/0.7)) {
			return lat, lon
		}
	}
	return lat, lon
}

// row is the row of cells nearest fy, held on the map.
func (e *airEnv) row(fy float64) int { return min(max(int(math.Round(fy)), 0), e.h-1) }

// seaTempAt is the temperature of the air at sea level at a place among the
// cells, in an ordinary year sinT of the way into the north's summer.
func (e *airEnv) seaTempAt(fx, fy, sinT float64) float64 {
	cy := e.row(fy)
	cont := e.sample(e.cont, fx, fy)
	t := e.mean[cy] + e.hemi[cy]*Swing*sinT*(swingSea+(swingLand-swingSea)*cont)
	if e.coast != nil {
		t += e.sample(e.coast, fx, fy)
	}
	return t
}

// seaTemp is the warmth of the sea at a place among the cells: the air's over
// it, with the sea's own small swing and what the currents have brought.
func (e *airEnv) seaTemp(fx, fy, sinT float64) float64 {
	cy := e.row(fy)
	t := e.mean[cy] + e.hemi[cy]*Swing*sinT*swingSea
	if e.warm != nil {
		t += e.sample(e.warm, fx, fy)
	}
	return t
}

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

// solve works the day's wind out: the climate's pressure for the day with
// the systems added, and the warmth the air has carried in since yesterday.
func (wx *Weather) solve(day int) {
	e := wx.env
	n := e.w * e.h
	sinT := yearSin(day)
	if len(wx.u) != n {
		wx.u, wx.v, wx.p = make([]float32, n), make([]float32, n), make([]float32, n)
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
		for cy := 0; cy < e.h; cy++ {
			if math.Abs(e.lat[cy]-s.Lat) > dlat {
				continue
			}
			ky := (e.lat[cy] - s.Lat) * 111.2
			coslat := math.Cos((e.lat[cy] + s.Lat) / 2 * math.Pi / 180)
			for cx := 0; cx < e.w; cx++ {
				kx := wrapLon(e.lon(cx, cy)-s.Lon) * 111.32 * coslat
				if math.Abs(kx) > reach {
					continue
				}
				extra[cy*e.w+cx] += depth * math.Exp(-(kx*kx+ky*ky)/(2*s.Radius*s.Radius))
			}
		}
	}
	e.solve(sinT, extra, wx.warm, wx.u, wx.v, wx.p)
}

// carry moves the warmth of the air on by a day of yesterday's wind, and lets
// it settle toward the warmth of the place. What the wind carries in is not
// the warmth the climate's own wind would have brought - that is already the
// place's - but what it brings over and above it.
func (wx *Weather) carry(day int) {
	e := wx.env
	n := e.w * e.h
	clim := e.airTemp(yearSin(day))
	// The climate's own wind today, to be taken from the day's.
	cu, cv := make([]float32, n), make([]float32, n)
	for k, m := range seasonWeights(day) {
		for i := range cu {
			cu[i] += float32(m) * wx.winds.u[k][i]
			cv[i] += float32(m) * wx.winds.v[k][i]
		}
	}
	next := make([]float64, n)
	keep := math.Exp(-1 / warmTime)
	e.rows(func(cy int) {
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			du, dv := float64(wx.u[i]-cu[i]), float64(wx.v[i]-cv[i])
			if wx.p[i] == 0 {
				du, dv = 0, 0 // no day has been worked out yet
			}
			// Where the air over this cell was a day ago, in cells.
			fx := float64(cx) - du*86400/e.dx[cy]
			fy := float64(cy) + dv*86400/e.dy
			w := e.sample(wx.warm, fx, fy) + e.sample(clim, fx, fy) - clim[i]
			next[i] = math.Max(-warmMost, math.Min(warmMost, w*keep))
		}
	})
	wx.warm = next
}

// sampleDay reads a field of the day's weather at tile i.
func (w *Land) sampleDay(field []float32, i int) float64 {
	e := w.Weather.env
	fx, fy := e.cellAt(w.Grid, i)
	return e.sample32(field, fx, fy)
}

// today reports whether the day's weather has been worked out for the ground
// the map now has.
func (w *Land) today() bool {
	return w.Weather != nil && len(w.Weather.u) > 0 && w.Grid.winds != nil && w.Weather.env == w.Grid.winds.airEnv
}

// WindAt is the wind near the ground at p today, in metres a second toward
// the east and toward the north: the day's weather where AdvanceWeather has
// been asked for it, and the climate's for the day of the year where it has
// not.
func (w *Land) WindAt(p geom.Pos) (east, north float64) {
	g := w.Grid
	if !g.In(p) {
		return 0, 0
	}
	i := g.Index(p)
	if !w.today() {
		return g.WindOn(i, w.Tick)
	}
	return w.sampleDay(w.Weather.u, i), w.sampleDay(w.Weather.v, i)
}

// PressureAt is the pressure at sea level over p today, in hPa.
func (w *Land) PressureAt(p geom.Pos) float64 {
	g := w.Grid
	if !g.In(p) {
		return 0
	}
	i := g.Index(p)
	if !w.today() {
		return g.PressureOn(i, w.Tick)
	}
	return w.sampleDay(w.Weather.p, i)
}

// GustAt is how hard the wind at p gusts today, in metres a second: the
// wind, and what the eddies the ground stirs up in it add. Open sea adds a
// third, and broken country more than half again.
func (w *Land) GustAt(p geom.Pos) float64 {
	g := w.Grid
	u, v := w.WindAt(p)
	s := math.Hypot(u, v)
	if g.winds == nil || !g.In(p) {
		return s
	}
	e := g.winds.airEnv
	fx, fy := e.cellAt(g, g.Index(p))
	land := 1 - e.sample(e.sea, fx, fy)
	rough := math.Min(1, e.sample(e.rough, fx, fy)/dragRough)
	return s * (1.35 + land*(0.15+0.3*rough))
}

// WarmthAt is how many degrees warmer than an ordinary day of the year the
// air at p is today, for what the day's wind has carried in. It is nothing
// where the day's weather has not been asked for.
func (w *Land) WarmthAt(p geom.Pos) float64 {
	if !w.today() || !w.Grid.In(p) {
		return 0
	}
	e := w.Weather.env
	fx, fy := e.cellAt(w.Grid, w.Grid.Index(p))
	return e.sample(w.Weather.warm, fx, fy)
}

// Place is where a system stands on the map, in tiles, and whether that is
// on the map at all.
func (w *Land) Place(s System) (x, y float64, on bool) {
	g := w.Grid
	if g.winds == nil {
		return 0, 0, false
	}
	e := g.winds.airEnv
	fx, fy, on := e.cellOf(s.Lat, s.Lon)
	k := float64(e.cell)
	return (fx + 0.5) * k, (fy + 0.5) * k, on
}
