package terra

import "testing"

// The moon takes nothing from the world's chance: a world whose moon has been
// asked about a thousand times draws the same next number as one whose moon
// was never looked at. And the moon is the seed's: the same seed, the same
// moon; another seed, another.
func TestTheMoonDrawsNoChance(t *testing.T) {
	a, b := NewLand(3, DefaultTerms()), NewLand(3, DefaultTerms())
	for tick := 0; tick < 1000; tick++ {
		a.Tick = tick
		_ = a.Tide()
		_ = a.Moon()
	}
	if x, y := a.RNG.Float64(), b.RNG.Float64(); x != y {
		t.Fatalf("after asking the moon, the world drew %v against %v", x, y)
	}
	if a.moon != b.moon {
		t.Errorf("one seed founded two moons: %+v and %+v", a.moon, b.moon)
	}
	if c := NewLand(4, DefaultTerms()); c.moon == a.moon {
		t.Errorf("seeds 3 and 4 have the same moon")
	}
}

// The day's sea is the land's to set each morning, and a copy of the map an
// island acts on reads the day it was taken on and cannot be moved off it.
func TestTheDaysTideIsSetOnWaking(t *testing.T) {
	w := NewLand(5, DefaultTerms())
	w.Tick = 40
	w.Wake(Waking{})
	if got, want := w.Grid.Tide(), w.Tide(); got != want {
		t.Fatalf("after waking on day 40 the map reads %+v, want %+v", got, want)
	}
	var island Grid
	w.Grid.Apart(&island, nil, w.Grid.Router(), nil)
	if island.Tide() != w.Grid.Tide() {
		t.Errorf("the island reads another day's sea")
	}
	if island.SetTide(TideOn(41, w.moon)) {
		t.Errorf("an island's day was moved")
	}
	if !w.Grid.SetTide(TideOn(41, w.moon)) || w.Grid.Tide().Tick != 41 {
		t.Errorf("the map's own day could not be set")
	}
}
