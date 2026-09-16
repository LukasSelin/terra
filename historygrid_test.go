package terra

import (
	"bytes"
	"math"
	"testing"

	"github.com/LukasSelin/terra/geom"
)

// deepRun is a history run on a grid shrink times coarser than the map terms
// describe, stopped before it is handed down: the grid it ran on, what it
// hands on, and a new map to hand it down onto.
func deepRun(seed uint64, terms Terms, shrink int) (from *Grid, d *deepStage, to *Grid) {
	was := historyShrink
	historyShrink = shrink
	defer func() { historyShrink = was }()
	w := unmade(seed, terms)
	to = w.newGround(terms)
	from = w.historyGround(to, terms)
	if shrink == 1 {
		// The map is the history's grid at one; hand it down onto a copy of
		// the map's own size, so that the reading between tiles is what runs.
		from = to
		to = w.newGround(terms)
	}
	d = w.history(from, terms.Epochs, terms.SeaShare, terms.Water)
	return from, d, to
}

// Handed down onto a map of its own size, a history is itself: every height,
// tile, bed, line of the book and field it hands on, to the bit. The reading
// between tiles reads a tile's own value where a map tile's centre is a
// history tile's centre.
func TestAHistoryHandedDownOntoItsOwnSizeIsItself(t *testing.T) {
	small := GlobeTerms()
	small.Width, small.Height = 128, 64
	for name, terms := range map[string]Terms{"ancient": AncientTerms(), "globe128": small} {
		t.Run(name, func(t *testing.T) {
			if testing.Short() && terms.Wrap {
				t.Skip("a globe's history takes seconds")
			}
			from, d, to := deepRun(1, terms, 1)
			got := handDown(from, to, d)
			for i := range to.Tiles {
				if math.Float64bits(to.Height[i]) != math.Float64bits(from.Height[i]) ||
					to.Tiles[i] != from.Tiles[i] || to.strata[i] != from.strata[i] ||
					to.Soil[i] != from.Soil[i] || to.Sand[i] != from.Sand[i] || to.Clay[i] != from.Clay[i] {
					t.Fatalf("tile %d is not the history's", i)
				}
				if from.ledger != nil && to.ledger[i] != from.ledger[i] {
					t.Fatalf("tile %d's line of the book is not the history's", i)
				}
			}
			for _, f := range []struct {
				name      string
				got, want []float64
			}{{"depths", got.depths, d.depths}, {"shares", got.shares, d.shares}, {"uplift", got.uplift, d.uplift}} {
				if len(f.got) != len(f.want) {
					t.Fatalf("%s: %d tiles handed on, %d kept", f.name, len(f.got), len(f.want))
				}
				for i := range f.got {
					if math.Float64bits(f.got[i]) != math.Float64bits(f.want[i]) {
						t.Fatalf("%s at %d: %v, and the history's %v", f.name, i, f.got[i], f.want[i])
					}
				}
			}
			for i := range got.ocean {
				if got.ocean[i] != d.ocean[i] {
					t.Fatalf("tile %d's crust is not the history's", i)
				}
			}
		})
	}
}

// Handed down from a grid half as fine, the map is the history read finer:
// no height outside the history's, every tile riding the plate of the history
// tile nearest it with its line of the book and its crust, the top of its pile
// standing where it stood against the ground, and every plate of the history
// on the map.
func TestAHistoryHandedDownFromACoarserGridIsTheHistoryReadFiner(t *testing.T) {
	small := GlobeTerms()
	small.Width, small.Height = 128, 64
	for name, terms := range map[string]Terms{"ancient": AncientTerms(), "globe128": small} {
		t.Run(name, func(t *testing.T) {
			if testing.Short() && terms.Wrap {
				t.Skip("a globe's history takes seconds")
			}
			from, d, to := deepRun(1, terms, 2)
			if from.W != to.W/2 || from.H != to.H/2 {
				t.Fatalf("the history ran on %dx%d under a %dx%d map", from.W, from.H, to.W, to.H)
			}
			got := handDown(from, to, d)
			lo, hi := math.Inf(1), math.Inf(-1)
			for i := range from.Tiles {
				lo, hi = math.Min(lo, from.Height[i]), math.Max(hi, from.Height[i])
			}
			plates := map[uint8]bool{}
			for i := range to.Tiles {
				p := to.PosOf(i)
				j := from.Index(geom.Pos{X: p.X / 2, Y: p.Y / 2})
				if h := to.Height[i]; h < lo || h > hi {
					t.Fatalf("tile %v stands at %.3f, outside the history's %.3f..%.3f", p, h, lo, hi)
				}
				if to.Tiles[i].Plate != from.Tiles[j].Plate || to.ledger[i] != from.ledger[j] || got.ocean[i] != d.ocean[j] {
					t.Fatalf("tile %v is not the nearest history tile's", p)
				}
				// The foot of the pile is laid on the map (see layFeet), so
				// what is below may have been remade; the top of it stands
				// where it stood against the ground.
				a := float64(to.strata[i].top[0]) - to.Height[i]
				b := float64(from.strata[j].top[0]) - from.Height[j]
				if math.Abs(a-b) > 1e-3*math.Max(1, math.Abs(b)) {
					t.Fatalf("tile %v: the top bed stands %.4f under the ground, and %.4f in the history", p, a, b)
				}
				plates[to.Tiles[i].Plate] = true
			}
			for j := range from.Tiles {
				if !plates[from.Tiles[j].Plate] {
					t.Fatalf("plate %d of the history is not on the map", from.Tiles[j].Plate)
				}
			}
			if len(got.uplift) != len(to.Tiles) && d.uplift != nil {
				t.Fatalf("the uplift was handed on over %d tiles of %d", len(got.uplift), len(to.Tiles))
			}
		})
	}
}

// A world whose history ran on a coarser grid is made, and does not depend on
// the goroutines it was made over, and resumes from its history file to the
// bit.
func TestAWorldOnACoarserHistoryIsAWorld(t *testing.T) {
	was, workers := historyShrink, Workers
	historyShrink = 2
	defer func() { historyShrink, Workers = was, workers }()
	for _, terms := range []Terms{AncientTerms()} {
		Workers = 1
		one := NewLand(1, terms)
		Workers = 8
		var file bytes.Buffer
		eight, err := MakeLandKeepingHistory(1, terms, &file)
		if err != nil {
			t.Fatal(err)
		}
		if digest(one) != digest(eight) {
			t.Fatal("one goroutine and eight made different worlds on a coarser history")
		}
		resumed, err := LandFromHistory(&file)
		if err != nil {
			t.Fatal(err)
		}
		if digest(resumed) != digest(one) {
			t.Fatal("a world on a coarser history did not resume to itself")
		}
		if len(one.Grid.Tiles) != terms.Width*terms.Height || one.Grid.Forest() == 0 {
			t.Fatalf("a world on a coarser history came out %d tiles with %d forest", len(one.Grid.Tiles), one.Grid.Forest())
		}
	}
}

// A band laid on the map keeps its share: of the map tiles under a history
// tile a band covered a share of, that share is reached, the nearest the
// band's middle first.
func TestABandLaidOnTheMapKeepsItsShare(t *testing.T) {
	from, to := NewGrid(2, 1), NewGrid(8, 4)
	from.strata, to.strata = make([]column, 2), make([]column, 8*4)
	for j := range from.strata {
		from.strata[j] = basement(Granite, 0, 0)
	}
	book := make([]record, 2)
	book[0].crush = 10 * madeEnough
	book[0].banded(makingCrush, 0.25, 0.5)
	book[1].crush = 10 * madeEnough
	book[1].banded(makingCrush, 1, 0)
	d := &deepStage{ocean: []bool{false, false}, book: book}
	got := handDown(from, to, d)
	_ = got
	schist := [2]int{}
	for i := range to.Tiles {
		if to.strata[i].rock[to.strata[i].n-1] == Schist {
			schist[to.PosOf(i).X/4]++
		}
	}
	if schist[0] != 4 || schist[1] != 16 {
		t.Fatalf("schist on %v of 16 map tiles under each history tile, want 4 and 16", schist)
	}
}
