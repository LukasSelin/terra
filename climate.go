package terra

import (
	"math"
	"math/rand/v2"

	"github.com/LukasSelin/terra/clock"
	"github.com/LukasSelin/terra/geom"
)

// The climate. A settlement that is founded in one weather and lives in it
// forever is a settlement with nothing to save for. Here the year turns:
// things grow fast in the warm half and slowly in the cold one, so the wild
// food an idle summer left standing is most of the wild food there is in
// February, and a roof is worth most in the months a body loses heat
// fastest. On top of the year sit two slower wanderings, so no two winters
// are the same and no two decades are either.
//
// The place is temperate. Midsummer is warm and midwinter bites without
// being fatal on its own; what kills is a winter met without a roof.

// Year is the length of a year, and Season a quarter of it. A life is some
// fifty of these, so a run long enough to develop sees dozens of winters.
//
// The calendar itself is package clock's, where a tick is a day. The
// ontology names it too, because what grows is measured in it; these are
// that same calendar under the names the weather already calls it by.
const (
	Year   = clock.Year
	Season = clock.Season
)

// The shape of the year. MeanTemp is the annual mean in degrees, Swing half
// the distance from midwinter to midsummer. Tick zero is early spring:
// people arrive with the growing season ahead of them, not behind them.
const (
	MeanTemp = 10.0
	Swing    = 12.0
)

// The two wanderings, each an AR(1) process. Drift is the slow one, a run of
// kind or unkind decades; Spell is the fast one, the warm week and the cold
// snap. Keep is how much of the anomaly survives a tick - one part in the
// process's timescale is shed - and Shock the standard deviation of what is
// added, chosen so the anomaly settles at a standard deviation of Wander.
const (
	driftTime   = 8.0 * clock.Year // a run of kind or unkind decades
	driftWander = 1.2              // degrees
	spellTime   = 1.0 * clock.Week // a warm week, a cold snap
	spellWander = 2.5              // degrees
)

var (
	driftKeep  = 1 - 1/driftTime
	driftShock = driftWander * math.Sqrt(1-driftKeep*driftKeep)
	spellKeep  = 1 - 1/spellTime
	spellShock = spellWander * math.Sqrt(1-spellKeep*spellKeep)
)

// Climate is the weather of the whole map at one tick. It is one temperature
// for everywhere: the settlement is small enough that the difference between
// its ends is nothing beside the difference between its seasons.
type Climate struct {
	Temp  float64 // this tick's temperature, in degrees, in the temperate latitudes
	Drift float64 // the slow anomaly, a kind or unkind decade
	Spell float64 // the fast anomaly, a warm week or a cold snap
	// rows is how many rows the map has, and globe whether it is one. On a
	// globe the weather is read by latitude - see TempAt - and Temp is the
	// weather of the temperate latitudes, which is the weather a valley
	// has everywhere.
	rows  int
	globe bool
	// tick is the day the weather was last advanced to. A globe's seasons lag
	// the sun by different amounts on different ground, so the day, and not
	// just how far into its swing Temp is, is what a globe is read by.
	tick int
}

// NewClimate is the weather a world is founded in: an ordinary early spring,
// with neither wandering underway.
func NewClimate() Climate {
	return Climate{Temp: seasonal(0)}
}

// NewClimateOn is the weather a world of the given shape is founded in.
func NewClimateOn(cfg Terms) Climate {
	c := NewClimate()
	c.rows, c.globe = cfg.Height, cfg.Wrap
	return c
}

// The globe's weather. Temperate is the latitude the default map's weather
// is the weather of. A globe's year is the energy balance's, by latitude: see
// ebm.go.
const Temperate = 45.0

// latitude is the latitude of row y in degrees, from ninety at the top
// row to minus ninety at the bottom.
func (c Climate) latitude(y int) float64 {
	return 90 - 180*(float64(y)+0.5)/float64(c.rows)
}

// zonalMean is the year's mean at sea level at a latitude on a globe: the energy
// balance's zonal mean there. It was MeanTemp and thirty degrees times how
// far the cosine of the latitude stood from its value at Temperate, which put
// the equator at nineteen degrees and the poles at minus eleven.
func zonalMean(lat float64) float64 {
	e := ebm()
	return e.at(&e.mean, lat)
}

// TempAt is this tick's temperature on row y. On a valley it is Temp
// everywhere, to the bit.
//
// On a globe it is the row's year read on ground of middling continentality,
// since a row is not a place: the swing its latitude has and the lag that
// ground has behind the sun. Where a place is known, Land.TempAt reads the
// ground's own.
func (c Climate) TempAt(y int) float64 {
	if !c.globe {
		return c.Temp
	}
	lat := c.latitude(y)
	season := swingAt(lat, contMiddling) * seasonAt(c.tick, lagAt(contMiddling))
	return c.MeanAt(y) + season + c.Drift + c.Spell
}

// MeanAt is the mean temperature of row y over a year.
func (c Climate) MeanAt(y int) float64 {
	if !c.globe {
		return MeanTemp
	}
	return zonalMean(c.latitude(y))
}

// GrowthAt is Growth on row y, and ChillAt is Chill there.
func (c Climate) GrowthAt(y int) float64 {
	if !c.globe {
		return c.Growth()
	}
	return growthOf(c.TempAt(y))
}

// ChillAt is Chill on row y.
func (c Climate) ChillAt(y int) float64 {
	if !c.globe {
		return c.Chill()
	}
	return chillOf(c.TempAt(y))
}

// seasonal is the temperature the turning year alone would give at tick t.
func seasonal(tick int) float64 {
	return MeanTemp + Swing*math.Sin(2*math.Pi*float64(tick)/Year)
}

// Advance moves the weather on one tick. It consumes exactly two numbers
// from rng, so a seed still reproduces a run.
func (c *Climate) Advance(tick int, rng *rand.Rand) {
	c.Drift = driftKeep*c.Drift + driftShock*rng.NormFloat64()
	c.Spell = spellKeep*c.Spell + spellShock*rng.NormFloat64()
	c.Temp = seasonal(tick) + c.Drift + c.Spell
	c.tick = tick
}

// The thresholds the living world reads temperature by. Green things grow
// at their slowest below Frost and at their fullest from Thrive up. Cold
// begins to be felt below Mild and presses as hard as it ever does at
// Bitter.
//
// Frost is a threshold of growth and of nothing else. It was the line the
// ground froze at too, which put the permafrost under ground with a mean of
// four degrees - the latitude of Oslo - and the tundra with it. The frozen
// ground has its own line now; see Permafrost and the tree line in year.go.
const (
	Frost  = 4.0
	Thrive = 14.0
	Mild   = 12.0
	Bitter = -5.0
)

// WinterGrowth is what the land still gives with the year at its coldest.
// It is not zero: this is a temperate place, not an arctic one, and a winter
// that stops the forest dead is a winter the settlement cannot get through.
// At zero the year became a siege - median population over 24 seeds fell
// from 128 to 21, with the first extinction the sweep had seen - and at 0.35
// seed 1 still bred not once in 2500 ticks and died with its founders. Half
// is a winter that pauses a settlement's growth without ending it: the land
// gives about two thirds of the old flat rate in the depth of it and a third
// again as much at midsummer.
const WinterGrowth = 0.5

// growthNorm holds the year's total growth where it was before the seasons
// existed. Averaged over a year the ramp between Frost and Thrive is worth
// 0.527 of full growth, so the whole curve averages WinterGrowth + 0.527 of
// what is above it, and full growth is worth the reciprocal of that: what
// changed is when a forest grows, not how much it grows in a year.
const growthNorm = 1 / (WinterGrowth + (1-WinterGrowth)*0.527)

// Growth is how much the season lets green things grow, in multiples of the
// old year-round rate. It is two thirds of that in the depth of winter and a
// third again as much at midsummer, and averages 1 over a year, so tuning
// done before the seasons still holds.
func (c Climate) Growth() float64 { return growthOf(c.Temp) }

func growthOf(temp float64) float64 {
	return growthNorm * (WinterGrowth + (1-WinterGrowth)*ramp(temp, Frost, Thrive))
}

// Chill is how hard the cold presses on a body, 0 in mild weather and 1 in
// the bitterest cold this place sees.
func (c Climate) Chill() float64 { return chillOf(c.Temp) }

func chillOf(temp float64) float64 { return 1 - ramp(temp, Bitter, Mild) }

// ramp is x placed on [0,1] between lo and hi, clamped at both ends.
func ramp(x, lo, hi float64) float64 {
	switch {
	case x <= lo:
		return 0
	case x >= hi:
		return 1
	}
	return (x - lo) / (hi - lo)
}

// SeasonOf names the quarter of the year tick falls in. It is the calendar's
// answer; this is here so that callers reading the weather need not reach
// past it for the date.
func SeasonOf(tick int) clock.Quarter { return clock.SeasonOf(tick) }

// The lapse rate: how much colder the air is for standing higher up.
//
// The weather is one temperature for a latitude, and it was one temperature
// for a latitude at every height, which made the top of a mountain exactly as
// warm as the valley it stands over. Height was the one thing the ground
// carried that the weather never read - see Tile.Height, off which the
// rivers, the soil and the going underfoot are all already read - so the high
// country was hard to live on for its slope alone, and a wood grew on a peak
// as readily as on the valley floor.
//
// Lapse is the real figure, six and a half degrees a kilometre, and it is
// deliberately not tuned. What it is worth depends on what a map has standing
// on it, and that follows from the map's own size: on the default valley the
// skyline is Relief plus Upland, some three hundred metres, so the highest
// ground on it is two degrees colder than the river and no more - which is
// why nothing measured on the valley moves much, and why the want of this was
// never felt there. On a globe the same rule over mountains ten times as high
// is the difference between a tree line and no tree line.
const Lapse = 0.0065

// TempAt is the temperature on the ground at p: the weather of its latitude,
// less what the height of the ground takes off it. It is the reading anything
// standing on a tile or living on it should ask; Climate.TempAt is the
// weather of the row, which is that reading at the foot of the map.
//
// On a globe the year is the ground's own: the swing and the lag of a place
// with as much land round it as p has (see swingAt and lagAt), so the middle of
// a continent has a hotter summer, a colder winter and an earlier midsummer
// than a coast at the same latitude. The currents off its coast are added,
// which is the one part of the sea's moderation that is felt in what grows and
// not only in what freezes: a coast in a warm current is mild the year round.
//
// And where the day's weather has been asked for, the warmth the day's wind
// has carried in is added - a cold snap behind a low, a warm spell in a
// southerly - in place of the climate's own spell, not on top of it: the two
// are the same week's weather told twice, once as a number drawn for the
// whole planet and once as the air that actually moved. The slow drift is
// kept, since no day's wind carries a decade. A valley's weather is one
// temperature for everywhere - see Climate - and is left to its own spells.
func (w *Land) TempAt(p geom.Pos) float64 {
	g, c := w.Grid, w.Climate
	h := g.At(p).Height
	if !c.globe {
		t := c.TempAt(p.Y) - Lapse*h
		if g.Wrap {
			t += w.WarmthAt(p) + g.CoastWarmth(p.Y*g.W+p.X)
		}
		return t
	}
	i := p.Y*g.W + p.X
	cont := g.contAt(i)
	t := c.MeanAt(p.Y) + swingAt(c.latitude(p.Y), cont)*seasonAt(c.tick, lagAt(cont)) + c.Drift - Lapse*h
	if g.Wrap {
		t += g.CoastWarmth(i)
	}
	if w.today() {
		t += w.WarmthAt(p)
	} else {
		t += c.Spell
	}
	return t
}

// yearAt is the shape of an ordinary year on the ground at tile i: its mean,
// and half the distance from its coldest day to its warmest. It is TempAt's
// climate with the day's weather taken out.
func (w *Land) yearAt(i int) (mean, swing float64) {
	g, c := w.Grid, w.Climate
	y := i / g.W
	mean = c.MeanAt(y) - Lapse*g.Tiles[i].Height
	if !c.globe {
		return mean, Swing
	}
	if g.Wrap {
		mean += g.CoastWarmth(i)
	}
	return mean, swingAt(c.latitude(y), g.contAt(i))
}

// GrowthAt is how much the weather at p lets green things grow, and ChillAt
// how hard the cold presses on a body there. Under the tuned rules it is the
// ramp between Frost and Thrive; under the climate's, the day's share of the
// place's growing degree-days and what its warmth and rain let a year grow.
// See Terms.Growth.
func (w *Land) GrowthAt(p geom.Pos) float64 {
	temp := w.TempAt(p)
	if !w.Terms.Growth.climate(w.Grid.Wrap) {
		return growthOf(temp)
	}
	i := w.Grid.Index(p)
	mean, swing := w.yearAt(i)
	return climateGrowth(temp, mean, swing, w.Grid.Rain(i))
}

// ChillAt is Chill at p.
func (w *Land) ChillAt(p geom.Pos) float64 { return chillOf(w.TempAt(p)) }

// SeaFreeze is the temperature the sea stops being water at, in degrees.
// Salt water freezes below fresh, and minus one and eight tenths is the real
// figure.
//
// It was not asked before. The frost had nothing to say about water at all -
// Grid.Frozen gave up on anything wet, because "what a frozen sea is belongs
// to the sea, and nothing here has an answer for it yet" - so the pole came
// out as open ocean lying against permafrost, and not merely open: flood
// gives every sea tile its fish, so row zero of a globe was 440 tiles of
// water carrying an average of 0.85 fish each, at eleven degrees below
// freezing. The best fishing on the map was on the ice cap.
//
// Water is ice where the year's mean at its surface is under SeaFreeze. See
// Grid.Freezing.
const SeaFreeze = -1.8

// seaMeanAt is the year's mean at sea level on row y for ground the sea and
// its currents are worth warm degrees to. It is a constant of the map rather
// than of the day, because the drift and the spell wander around it and
// average out; every line the ground freezes or stops growing trees at is
// read off it and the height. See Maritime.
func (c Climate) seaMeanAt(y int, warm float64) float64 {
	return c.MeanAt(y) + warm
}

// The sea's moderation. Water is a store of heat that land is not, so ground
// with a lot of sea about it does not freeze as readily as ground the same
// distance from the equator but deep inside a continent. It is why the tree
// line in the real world follows a coast rather than a parallel.
//
// Without it the frost was a ruled line, and the same ruled line on every
// seed. The lowland is Relief tall - sixty metres - while a single row of a
// globe five hundred deep is worth some twenty-four metres of frostline in
// the latitudes the ice edge falls in, so the line swept through the entire
// height of ordinary ground in four rows: row 86 came out 45 per cent rock
// and 0 per cent grass, row 90 nothing but grass and forest, and 377 of a
// thousand columns turned green on one single row. Maritime is what gives the
// edge something to be ragged about, and what it is ragged about is the
// shape of the sea, which is different on every map.
//
// It is read as a share of the country round a place rather than as the
// distance to the nearest water, because that is the difference between a
// spit of land in the open ocean and the head of a long inlet reaching into
// a continent: the two are equally near the sea and are not equally warmed by
// it. Taken as a distance the ice edge still began at one fixed latitude on
// every seed, because there is coast at every latitude on a globe and every
// yard of it was worth the same.
//
// It moderates what freezes and not what grows (the currents, which are a
// different thing, are felt in both: see Grid.CoastWarmth): Climate.TempAt is the weather
// of a latitude at a height and is read every tick by everything alive, and
// the sea is a fact about where the permafrost stops. A map with no sea - the
// valley, and every map measured on it - is untouched to the bit, because
// there is no water anywhere near to be warmed by.
const (
	// Maritime is what ground the sea surrounds entirely would be worth, in
	// degrees. An even coast, half land and half water within reach of it,
	// gets half.
	Maritime = 6.0
	// maritimeSpan is the fraction of the map across that counts as within
	// reach. How far off the sea is still felt depends on how big the land
	// is, which follows the size of the map, so the reach is quoted as a
	// share of it rather than in tiles. A sixth is continental: it is the
	// scale at which one part of a world is oceanic and another is not, and
	// narrowing it to a sixteenth put the frost back on one row in more than
	// half a thousand columns of the worst of five seeds, against two hundred
	// at a sixth.
	maritimeSpan = 6
)

// maritime is what having share of the country round a place under the sea
// adds to the year's mean there. On a map with no sea the share is zero and
// so is this, exactly.
func maritime(share float64) float64 { return Maritime * share }
