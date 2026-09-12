package clock

import "testing"

// The calendar has to be long enough to live in. A season has to outlast a
// journey across the map - eighty tiles of open ground is eighty days - and
// several ordinary errands besides, because a season nobody can sow, cut and
// store within is a season nobody can act on. This is the whole reason the
// year was lengthened, and it is the one thing here worth a test.
func TestASeasonHoldsAJourney(t *testing.T) {
	const (
		mapCrossing = 80 * Day // the long way, corner to corner
		errand      = 10 * Day // the ordinary way, out to a field and back
	)
	if Season <= mapCrossing {
		t.Errorf("a season is %d days and a walk across the map %d; a season has to outlast one", Season, mapCrossing)
	}
	if Season < 6*errand {
		t.Errorf("a season is %d days and an errand %d; a season has to hold a season's work", Season, errand)
	}
	if Year != 4*Season {
		t.Errorf("a year is %d days and a season %d; a year is four seasons", Year, Season)
	}
}

// Tick zero is the first day of spring in the first year, and the seasons
// come round in order.
func TestTheYearTurns(t *testing.T) {
	d := At(0)
	if d.Year != 1 || d.Season != Spring || d.Day != 1 || d.YearDay != 1 {
		t.Errorf("the world begins on %v, want spring 1 of year 1", d)
	}
	for i, want := range []Quarter{Spring, Summer, Autumn, Winter} {
		if got := SeasonOf(i*Season + Season/2); got != want {
			t.Errorf("the middle of quarter %d is %s, want %s", i, got, want)
		}
	}
	if got := At(2*Year + Season + 4); (got != Date{Year: 3, Season: Summer, Day: 5, YearDay: Season + 5}) {
		t.Errorf("got %#v", got)
	}
}

// A year on, the date is the same date. This is what lets anything be said
// in years and read in days.
func TestAYearOnIsTheSameDay(t *testing.T) {
	for _, tick := range []int{0, 17, Season - 1, 3*Season + 40} {
		a, b := At(tick), At(tick+Year)
		if a.Season != b.Season || a.Day != b.Day || b.Year != a.Year+1 {
			t.Errorf("%v and %v are a year apart but not the same day of it", a, b)
		}
		if Years(tick+Year) != Years(tick)+1 {
			t.Errorf("a year on from day %d is not one year older", tick)
		}
	}
}

// The calendar runs backwards too. An agent's age is asked for before it is
// born wherever a newcomer is spawned with a birthday in the past.
func TestTheCalendarRunsBackwards(t *testing.T) {
	if d := At(-1); d.Year != 0 || d.Season != Winter || d.Day != Season {
		t.Errorf("the day before the founding is %v, want the last day of winter", d)
	}
	if got := Years(-Year); got != -1 {
		t.Errorf("a year before the founding is %d years, want -1", got)
	}
	if got := SeasonOf(-Season); got != Winter {
		t.Errorf("a season before the founding is %s, want winter", got)
	}
}
