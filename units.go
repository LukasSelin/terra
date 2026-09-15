package terra

// Units: what the numbers on this map are numbers of.
//
// A figure without a unit can be tuned and cannot be checked. "Forty-five a
// closing" says nothing a geologist can hold up against a mountain; "five
// millimetres a year where two continents close at five centimetres a year"
// says something the Himalaya answers (Lavé and Avouac 2001 have 4 to 8 mm/yr
// of rock uplift across the front of it). So every rate here is written in a
// length and a time, the lengths in metres and the times in years, and turned
// into what one step of one pass does at the place it is used - never before,
// so that the figure a reader finds at the declaration is the real one.
//
// The calendar a settlement lives by is package clock's, where a tick is a day
// and Year is so many ticks. What the ground and the earth do is on a longer
// clock, and it is written in years of the calendar's length: yr below.

// Lengths, in metres.
const (
	metre = 1.0
	km    = 1000 * metre
	cm    = metre / 100
	mm    = metre / 1000
)

// Durations, in years.
const (
	yr  = 1.0
	kyr = 1000 * yr
	myr = 1000 * kyr
)

// secondsPerYear turns a year's water into a flow.
const secondsPerYear = 365.25 * 24 * 3600

// TileSpan is how wide a tile is on the ground, in metres. It is what turns a
// difference in height into a slope, and so the only reason heights and
// distances can be spoken of in the same breath.
const TileSpan = 25.0

// ageYears is how long an age of weather is: what Erode passes, and what a
// rate in metres a year is multiplied by to be what one age of it does. See
// erode.go.
const ageYears = 10 * yr

// span is how wide a tile of g is on the ground, in metres: TileSpan, except
// while a history runs, when a tile is a piece of a planet. See deepSpan.
func (g *Grid) span() float64 {
	if g.deep > 0 {
		return g.deep
	}
	return TileSpan
}

// tilesAcross is how many tiles span metres wide a length of metres is. A
// distance on the ground is written in metres at its declaration and turned
// into tiles here, at the point of use, so that a map read at another span
// reads it as the same ground.
func tilesAcross(metres, span float64) float64 { return metres / span }

// discharge is the flow, in cubic metres a second, of mmPerYear of runoff off
// a tile span metres across.
func discharge(mmPerYear, span float64) float64 {
	return mmPerYear * span * span / 1000 / secondsPerYear
}
