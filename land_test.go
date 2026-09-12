package terra

import "testing"

// The whole of the split, said as a test: a country is made, a season passes
// over it, and at no point is there a settlement, an agent, a market or a
// technology anywhere in the sentence. If this ever needs one of those to
// compile, the land has learnt something it has no business knowing.
func TestTheLandIsMadeAndAgedWithNobodyOnIt(t *testing.T) {
	l := NewLand(7, DefaultTerms())

	g := l.Grid
	if g == nil || len(g.Tiles) != DefaultWidth*DefaultHeight {
		t.Fatalf("no ground: %v", g)
	}
	if l.Forest0 == 0 {
		t.Error("a country with no woods on it at all")
	}
	var land, water int
	for i := range g.Tiles {
		if g.Tiles[i].Wet() {
			water++
		} else {
			land++
		}
	}
	if land == 0 {
		t.Error("all sea")
	}

	// A season of days over ground nobody stands on. Nothing is peopled,
	// which is the only thing the land asks and the answer a map with no
	// game on it gives.
	none := func(int) bool { return false }
	for day := 0; day < 90; day++ {
		l.Tick++
		l.Wake(Waking{Peopled: none})
	}
	if l.Awake.Chunks != len(g.Chunks) {
		t.Errorf("awake over %d chunks of %d", l.Awake.Chunks, len(g.Chunks))
	}
	if l.Awake.Peopled != 0 {
		t.Errorf("%d chunks peopled on an empty world", l.Awake.Peopled)
	}
	// The sweep reaches every sleeping chunk once a season, so after a
	// season of empty days no ground anywhere is more than a season behind.
	for i := range g.Chunks {
		if c := g.Chunks[i]; c.Weathered == 0 || l.Tick-c.Weathered > sweepOver {
			t.Fatalf("chunk %d weathered to %d, the day is %d", i, c.Weathered, l.Tick)
		}
	}
}

// The same seed makes the same country, with no game involved in saying so.
func TestTheSameSeedMakesTheSameLand(t *testing.T) {
	a, b := NewLand(11, DefaultTerms()), NewLand(11, DefaultTerms())
	for i := range a.Grid.Tiles {
		if a.Grid.Tiles[i] != b.Grid.Tiles[i] {
			t.Fatalf("tile %d differs: %+v vs %+v", i, a.Grid.Tiles[i], b.Grid.Tiles[i])
		}
	}
	if c := NewLand(12, DefaultTerms()); sameTiles(a.Grid, c.Grid) {
		t.Error("two seeds made the same country")
	}
}

func sameTiles(a, b *Grid) bool {
	if len(a.Tiles) != len(b.Tiles) {
		return false
	}
	for i := range a.Tiles {
		if a.Tiles[i] != b.Tiles[i] {
			return false
		}
	}
	return true
}
