package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
)

// The air over the land, as the land reads it: the climate of the wind on a
// tile of the grid, and the day's weather at a place on the land. What the air
// is, and how it is worked out, is the atmosphere's; see wind.go.

// WindOn is the wind near the ground on tile i on a day of the year, in
// metres a second toward the east and toward the north: the climate of the
// wind, what it is on that day in an ordinary year. It is nothing on a map
// whose weather has not been read.
func (g *Grid) WindOn(i, day int) (east, north float64) {
	w := g.winds
	if w == nil || i < 0 || i >= len(g.Tiles) {
		return 0, 0
	}
	return w.WindOn(i, day)
}

// MeanWind is the wind near the ground on tile i over the whole year, in
// metres a second toward the east and toward the north.
func (g *Grid) MeanWind(i int) (east, north float64) {
	w := g.winds
	if w == nil || i < 0 || i >= len(g.Tiles) {
		return 0, 0
	}
	return w.MeanWind(i)
}

// PressureOn is the pressure at sea level over tile i on a day of the year,
// in hPa, in an ordinary year. It is nothing on a map whose weather has not
// been read.
func (g *Grid) PressureOn(i, day int) float64 {
	w := g.winds
	if w == nil || i < 0 || i >= len(g.Tiles) {
		return 0
	}
	return w.PressureOn(i, day)
}

// SeaWarmth is how many degrees the sea over tile i stands warmer than the
// mean of its latitude, for the currents: warm in a western current, cold in
// an eastern one and colder where the water comes up from under. It is
// nothing on land, on a valley, and on a map whose weather has not been read.
func (g *Grid) SeaWarmth(i int) float64 {
	w := g.winds
	if w == nil || i < 0 || i >= len(g.Tiles) {
		return 0
	}
	return w.SeaWarmth(i)
}

// CoastWarmth is how many degrees the currents offshore make the year's mean
// on tile i warmer or colder: Norway's mildness, and the chill of the fog off
// Peru. It is nothing on a valley and on a map whose weather has not been
// read.
func (g *Grid) CoastWarmth(i int) float64 {
	w := g.winds
	if w == nil || i < 0 || i >= len(g.Tiles) {
		return 0
	}
	return w.CoastWarmth(i)
}

// AdvanceWeather moves the day's weather on to today, w.Tick. A game calls it
// once a day, beside Climate.Advance; nothing about the land needs it to have
// been called, and it draws nothing from the world's chance. The first call
// starts the weather with some weeks of systems already behind it.
func (w *Land) AdvanceWeather() {
	g := w.Grid
	if g.winds == nil {
		g.weather()
	}
	if !w.Weather.Over(g.winds) {
		w.Weather = NewWeather(w.seed, g.winds, w.Tick, w.Weather)
	}
	w.Weather.Advance(w.Tick)
}

// today reports whether the day's weather has been worked out for the ground
// the map now has.
func (w *Land) today() bool {
	return w.Weather.Over(w.Grid.winds) && w.Weather.Worked()
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
	return w.Weather.WindAt(i)
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
	return w.Weather.PressureAt(i)
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
	return g.winds.Gust(g.Index(p), s)
}

// WarmthAt is how many degrees warmer than an ordinary day of the year the
// air at p is today, for what the day's wind has carried in. It is nothing
// where the day's weather has not been asked for.
func (w *Land) WarmthAt(p geom.Pos) float64 {
	if !w.today() || !w.Grid.In(p) {
		return 0
	}
	return w.Weather.WarmthAt(w.Grid.Index(p))
}

// Place is where a system stands on the map, in tiles, and whether that is
// on the map at all.
func (w *Land) Place(s System) (x, y float64, on bool) {
	g := w.Grid
	if g.winds == nil {
		return 0, 0, false
	}
	return g.winds.Place(s.Lat, s.Lon)
}
