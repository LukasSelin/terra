package terra

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/LukasSelin/terra/geom"
)

// The causal record: the book a history keeps, the features joined from it,
// and Why read off both. See ledger.go, features.go and why.go.

// chainsOf is every chain of every aspect for a spread of tiles, which is
// what two worlds are compared on.
func chainsOf(g *Grid) [][]Cause {
	var chains [][]Cause
	for i := 0; i < len(g.Tiles); i += 7 {
		p := g.PosOf(i)
		for a := OfHeight; a < aspects; a++ {
			chains = append(chains, g.Why(p, a))
		}
	}
	return chains
}

// The same seed gives the same chains, and the goroutines the world was
// dealt over do not enter into them: the book is written by passes whose
// results do not depend on Workers, and the registry is built on one.
func TestWhyIsDeterministic(t *testing.T) {
	made := func(workers int) *Grid {
		was := Workers
		Workers = workers
		defer func() { Workers = was }()
		return NewLand(1, AncientTerms()).Grid
	}
	one := made(1)
	again := made(1)
	if !reflect.DeepEqual(chainsOf(one), chainsOf(again)) {
		t.Fatal("the same seed gave different chains")
	}
	if !reflect.DeepEqual(one.Features().All, again.Features().All) {
		t.Fatal("the same seed gave different features")
	}
	four := made(4)
	if !reflect.DeepEqual(chainsOf(one), chainsOf(four)) {
		t.Fatal("four goroutines gave different chains than one")
	}
	if !reflect.DeepEqual(one.Features().All, four.Features().All) {
		t.Fatal("four goroutines gave different features than one")
	}
}

// A belt is the tiles one meeting raised: every tile of it has the same
// two plates and the same epoch in the book, and the belt says which.
func TestABeltIsRaisedByOneMeeting(t *testing.T) {
	g := yardWorld("ancient", 1, AncientTerms())
	f := g.Features()
	if f == nil {
		t.Fatal("a made world has no features")
	}
	belts := 0
	for k := range f.All {
		fe := &f.All[k]
		if fe.Kind != UpliftBelt {
			continue
		}
		belts++
		if fe.Meeting == NoMeeting {
			t.Errorf("belt %d was raised by no meeting", fe.ID)
		}
		if len(fe.Tiles) != fe.Count || fe.Count == 0 {
			t.Errorf("belt %d lists %d tiles and counts %d", fe.ID, len(fe.Tiles), fe.Count)
		}
		for _, i := range fe.Tiles {
			l := &g.ledger[i]
			if l.plates != fe.Plates || l.epoch != fe.Epoch || l.raised() != fe.Meeting {
				t.Errorf("belt %d: tile %d was raised by plates %v in epoch %d (%v), the belt by %v in %d (%v)",
					fe.ID, i, l.plates, l.epoch, l.raised(), fe.Plates, fe.Epoch, fe.Meeting)
			}
			if g.featureAt(int(i), UpliftBelt) != fe.ID {
				t.Errorf("belt %d: tile %d is labelled belt %d", fe.ID, i, g.featureAt(int(i), UpliftBelt))
			}
		}
	}
	if belts == 0 {
		t.Error("the ancient valley has no belts")
	}
}

// The book costs twelve bytes a tile on a made world and nothing on a drawn
// one, which is what docs/perf/worklog.md says it costs.
func TestTheBookCostsWhatItSays(t *testing.T) {
	if size := unsafe.Sizeof(ledger{}); size != 12 {
		t.Errorf("a line of the book is %d bytes, not 12", size)
	}
	made := yardWorld("ancient", 1, AncientTerms())
	if len(made.ledger) != len(made.Tiles) || cap(made.ledger) != len(made.Tiles) {
		t.Errorf("the book has %d lines (capacity %d) for %d tiles", len(made.ledger), cap(made.ledger), len(made.Tiles))
	}
	if made.epochs != 16 {
		t.Errorf("the book says %d epochs, not 16", made.epochs)
	}
	drawn := yardWorld("valley", 1, DefaultTerms())
	if drawn.ledger != nil || drawn.plateRoot != nil {
		t.Error("a drawn world keeps a book")
	}
	if got := drawn.Why(geom.Pos{X: 10, Y: 10}, OfHeight); len(got) != 1 || got[0].Kind != Stands {
		t.Errorf("a drawn world's height chain is %v, not one Stands", got)
	}
}

// The chain for one fixed tile of the ancient preset, seed 1. This is a
// golden test: a change that means to move the world may move it, and then
// says so here.
func TestTheChainForOneTileOfTheAncientValley(t *testing.T) {
	g := yardWorld("ancient", 1, AncientTerms())
	p := g.PosOf(1000)
	want := map[Aspect][]CauseKind{
		OfHeight: {RaisedBy, BetweenPlate, BetweenPlate, WornSince, Stands},
		OfRock:   {BedOf, MeltedAtDepth, BuriedLast},
		OfRain:   {RainOf, LatitudeRain, Orographic},
		OfCover:  {Suits, Drains, Slope},
	}
	for a, kinds := range want {
		chain := g.Why(p, a)
		var got []CauseKind
		for _, c := range chain {
			got = append(got, c.Kind)
		}
		if !reflect.DeepEqual(got, kinds) {
			t.Errorf("%v: the chain is %v, want %v", a, got, kinds)
		}
	}
	chain := g.Why(p, OfHeight)
	if chain[0].Note != "arc" || chain[0].When != 10*epochYears || chain[0].Quantity <= 0 {
		t.Errorf("tile 1000 was raised by %q %v years ago by %v m; want an arc, %v years ago, more than nothing",
			chain[0].Note, chain[0].When, chain[0].Quantity, 10*epochYears)
	}
	if belt := g.Feature(chain[0].Feature); belt == nil || belt.Kind != UpliftBelt || belt.Meeting != Arc {
		t.Errorf("the height's first cause is not an arc's belt: %v", belt)
	}
	for _, c := range chain[1:3] {
		if pl := g.Feature(c.Feature); pl == nil || pl.Kind != CrustPlate {
			t.Errorf("a plate cause names %v", pl)
		}
	}
	// And every cause has a unit where it has a quantity of the ground.
	for a := OfHeight; a < aspects; a++ {
		for _, c := range g.Why(p, a) {
			switch c.Kind {
			case RaisedBy, WornSince, Stands, BedOf, RainOf, LatitudeRain, Orographic, UpwindSea, Slope, Warmth, Evaporation, Drains, SoilDepth:
				if c.Unit == "" {
					t.Errorf("%v: %v has no unit", a, c.Kind)
				}
			}
		}
	}
}
