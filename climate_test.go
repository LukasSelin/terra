package terra

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/LukasSelin/terra/clock"
)

// The year should be a year: warm in the middle of summer, cold in the
// middle of winter, and back where it started when it has gone round.
func TestTheYearTurns(t *testing.T) {
	summer := seasonal(Year / 4)
	winter := seasonal(3 * Year / 4)
	if summer <= winter {
		t.Fatalf("summer %.1f is not warmer than winter %.1f", summer, winter)
	}
	if got := summer - winter; math.Abs(got-2*Swing) > 0.01 {
		t.Errorf("swing from winter to summer is %.2f, want %.2f", got, 2*Swing)
	}
	if got := seasonal(Year) - seasonal(0); math.Abs(got) > 0.01 {
		t.Errorf("a year on, the season is %.2f off where it began", got)
	}
	if SeasonOf(0) != clock.Spring || SeasonOf(Year/4) != clock.Summer || SeasonOf(3*Year/4) != clock.Winter {
		t.Errorf("seasons are misnamed: %s %s %s", SeasonOf(0), SeasonOf(Year/4), SeasonOf(3*Year/4))
	}
}

// A frost is lean and the warmth is generous, but over a whole year the land
// puts back what it put back before there were seasons. That is what lets
// the tuning done without them stand.
func TestAYearOfGrowthIsWhatItWas(t *testing.T) {
	frost := (Climate{Temp: Frost - 1}).Growth()
	warmth := (Climate{Temp: Thrive + 5}).Growth()
	if frost <= 0 {
		t.Errorf("growth in a frost is %.2f; a temperate winter still gives something", frost)
	}
	if warmth < 1.8*frost {
		t.Errorf("warmth grows %.2f against a frost's %.2f, which is barely a season", warmth, frost)
	}
	if math.Abs(warmth-growthNorm) > 1e-9 {
		t.Errorf("growth in full warmth is %.2f, want %.2f", warmth, growthNorm)
	}
	var sum float64
	for tick := 0; tick < Year; tick++ {
		sum += (Climate{Temp: seasonal(tick)}).Growth()
	}
	if mean := sum / Year; math.Abs(mean-1) > 0.03 {
		t.Errorf("a year averages %.3f of the old growth rate, want 1", mean)
	}
}

// Cold is what a body feels, not what the thermometer says: nothing in mild
// weather, all of it at the bottom of the year.
func TestChillIsFeltOnlyInTheCold(t *testing.T) {
	if c := (Climate{Temp: Mild + 5}).Chill(); c != 0 {
		t.Errorf("mild weather chills %.2f, want 0", c)
	}
	if c := (Climate{Temp: Bitter - 5}).Chill(); c != 1 {
		t.Errorf("bitter weather chills %.2f, want 1", c)
	}
	cold := (Climate{Temp: 0}).Chill()
	cool := (Climate{Temp: 8}).Chill()
	if cold <= cool {
		t.Errorf("freezing (%.2f) does not press harder than cool (%.2f)", cold, cool)
	}
}

// A run of years is a temperate one: no two alike, but the wandering stays
// within a few degrees of the turning year and never runs away.
func TestWeatherWandersWithoutRunningAway(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	c := NewClimate()
	var worst float64
	summers := make([]float64, 0, 20)
	for tick := 1; tick <= 20*Year; tick++ {
		c.Advance(tick, rng)
		if d := math.Abs(c.Temp - seasonal(tick)); d > worst {
			worst = d
		}
		if tick%Year == Year/4 {
			summers = append(summers, c.Temp)
		}
	}
	if worst > 12 {
		t.Errorf("weather strayed %.1f degrees from the season, which is a different climate", worst)
	}
	var spread float64
	for i := 1; i < len(summers); i++ {
		spread += math.Abs(summers[i] - summers[i-1])
	}
	if spread/float64(len(summers)-1) < 0.5 {
		t.Errorf("consecutive midsummers differ by %.2f degrees on average; the years are all alike", spread/float64(len(summers)-1))
	}
}

// The same seed is the same weather, whatever else happened in between.
func TestWeatherIsReproducible(t *testing.T) {
	run := func() []float64 {
		rng := rand.New(rand.NewPCG(7, 9))
		c := NewClimate()
		out := make([]float64, 0, 200)
		for tick := 1; tick <= 200; tick++ {
			c.Advance(tick, rng)
			out = append(out, c.Temp)
		}
		return out
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("tick %d: %.6f then %.6f", i+1, a[i], b[i])
		}
	}
}
