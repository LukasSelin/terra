package terra

import "testing"

// A shelf is as wide as its margin is quiet. A strip of continent across a
// map at a full globe's tile, with the floor of its own plate to the west of
// it and another plate's to the east: the floor goes down nearer the coast to
// the east, where the margin is active, than to the west, where it is
// passive, and never beside the coast. Weld the two plates into one and the
// east is a quiet margin too. See floorDepths.
func TestAShelfIsWideWhereItsMarginIsQuiet(t *testing.T) {
	const w, h, west, east = 64, 8, 28, 36 // the continent is x in [west, east)
	g := NewGrid(w, h)
	g.deep = 37.5 * km // a full globe's tile: see deepSpan
	cr := newCrust(g)
	for i := range g.Tiles {
		x := i % w
		cr.ocean[i] = x < west || x >= east
		if x >= east {
			g.Tiles[i].Plate = 1
		}
	}
	// How many tiles out from the coast tile at x, stepping by step, the
	// floor first goes down.
	first := func(share []float64, x, step int) int {
		y := h / 2
		for d := 1; ; d++ {
			if share[y*w+x+step*d] > 0 {
				return d
			}
		}
	}

	g.keepPlates([]Plate{{into: 0}, {into: 1}})
	_, share, _, _ := g.floorDepths(cr, 16, nil, nil)
	quiet, active := first(share, west, -1), first(share, east-1, 1)
	if !(active < quiet) {
		t.Errorf("the floor goes down %d tiles out of the active margin and %d out of the quiet one", active, quiet)
	}
	if active < 2 {
		t.Errorf("the floor goes down %d tile out of the active margin, beside the coast", active)
	}

	g.keepPlates([]Plate{{into: 0}, {into: 0}})
	_, share, _, _ = g.floorDepths(cr, 16, nil, nil)
	if welded := first(share, east-1, 1); welded != quiet {
		t.Errorf("welded to the continent's plate, the east floor goes down %d tiles out, and the west %d", welded, quiet)
	}
}
