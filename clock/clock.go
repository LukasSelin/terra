// Package clock is the world's calendar: the one place that says how long
// anything takes.
//
// A tick is a day. That is the whole of the convention, and everything else
// here follows from it. It was not always so - a year was once a hundred
// ticks, a season twenty-five - and the trouble with that was not that it
// was unrealistic but that it was unlivable: a walk across the map cost
// eighty ticks, so a settler who set out for a plot in the spring arrived in
// the autumn, and nobody could sow, cut and store within a season because a
// season was shorter than an errand. Time has to be long enough that a life
// fits inside it.
//
// So: a day is what an agent plans in, a season is ninety of them, and a
// year is four seasons. A walk of forty tiles is six weeks, which is a
// journey; a house goes up in three days, which is a barn-raising rather
// than a career; a crop stands a season. Durations are written here or
// against these units - "a fortnight", "three years" - and never as bare
// tick counts, so that changing the calendar changes the world with it.
//
// The one place the calendar is compressed is a life: an agent is grown at
// five and old at fifty, which is not a human childhood. It is what lets a
// run of twenty thousand days turn a settlement over three or four times,
// which is the only way to see whether what one generation believed
// outlived it. See entity.Lifespan.
package clock

import "fmt"

// The units of world time, in ticks. Nothing shorter than a day exists: an
// agent's smallest act - eating, a step, a look about - fills one.
const (
	Day       = 1
	Week      = 7 * Day
	Fortnight = 2 * Week
	Month     = 30 * Day
	Season    = 3 * Month
	Year      = 4 * Season
)

// Season names, from the one tick zero falls in. A world is founded in early
// spring, with the growing season ahead of it rather than behind it.
type Quarter uint8

const (
	Spring Quarter = iota
	Summer
	Autumn
	Winter
)

var quarterNames = [4]string{"spring", "summer", "autumn", "winter"}

// String names the season.
func (q Quarter) String() string { return quarterNames[q%4] }

// SeasonOf names the quarter of the year tick falls in.
func SeasonOf(tick int) Quarter { return Quarter(floorMod(tick, Year) / Season) }

// Date is a tick told as a date. Years, days and the day of the season all
// count from one, the way a calendar does: the first day of the world is day
// one of spring in year one.
type Date struct {
	Year    int     // years since the founding, from 1
	Season  Quarter // the quarter of the year
	Day     int     // day of the season, from 1
	YearDay int     // day of the year, from 1
}

// At tells the date at a tick.
func At(tick int) Date {
	y := floorDiv(tick, Year)
	d := floorMod(tick, Year)
	return Date{
		Year:    y + 1,
		Season:  Quarter(d / Season),
		Day:     d%Season + 1,
		YearDay: d + 1,
	}
}

// String is the date as it would be said: "summer 12, year 3".
func (d Date) String() string {
	return fmt.Sprintf("%s %d, year %d", d.Season, d.Day, d.Year)
}

// Years is how many whole years a span of ticks comes to. It is how an age
// is told: an agent born eleven thousand days ago is thirty.
func Years(ticks int) int { return floorDiv(ticks, Year) }

// Days is how many days into its current year a span of ticks reaches, so
// that an age can be given as years and days.
func Days(ticks int) int { return floorMod(ticks, Year) }

// floorDiv and floorMod divide toward negative infinity, so that the
// calendar runs backwards as sensibly as it runs forwards. Ticks before the
// founding turn up wherever an agent's age is asked for before it is born.
func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func floorMod(a, b int) int {
	m := a % b
	if m != 0 && (m < 0) != (b < 0) {
		m += b
	}
	return m
}
