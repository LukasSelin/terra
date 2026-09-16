package terra

import "testing"

// The read-outs say what the map's own shapes hold, tile for tile.
func TestTheReadOutsAreWhatTheMapKeeps(t *testing.T) {
	g := yardWorld("ancient", 1, AncientTerms())
	if g.strata == nil || g.ledger == nil {
		t.Fatal("a made world keeps no beds or no book")
	}
	var beds []Bed
	lifted := 0
	for i := range g.Tiles {
		beds = g.AppendBeds(beds[:0], i)
		c := &g.strata[i]
		if len(beds) != int(c.n) || len(beds) == 0 || len(beds) > BedsMax {
			t.Fatalf("tile %d: %d beds, the column has %d", i, len(beds), c.n)
		}
		h := g.Height[i]
		rock := beds[len(beds)-1].Rock
		for k := 0; k+1 < len(beds); k++ {
			if beds[k+1].Top < h {
				rock = beds[k].Rock
				break
			}
		}
		if want := g.bedAt(i, h); rock != want {
			t.Fatalf("tile %d: the beds put %v at its surface, the column %v", i, rock, want)
		}

		r, ok := g.Record(i)
		l := g.ledger[i]
		if !ok || r.Lift != float64(l.lift) || r.Worn != l.wornMetres() || r.Plates != l.plates ||
			r.Meeting != l.raised() || r.Epoch != l.epoch || r.Burial != Burial(l.buried()) || r.BuriedIn != l.buriedIn {
			t.Fatalf("tile %d: record %+v, ledger %+v", i, r, l)
		}
		if r.Meeting != NoMeeting {
			lifted++
		}
		for k := NoFeature + 1; k < featureKinds; k++ {
			if got := g.FeatureOf(i, k); got != g.featureAt(i, k) {
				t.Fatalf("tile %d: %v feature %d, not %d", i, k, got, g.featureAt(i, k))
			}
		}
	}
	if lifted == 0 {
		t.Error("no tile's record names a meeting")
	}

	drawn := NewLand(1, DefaultTerms()).Grid
	if _, ok := drawn.Record(0); ok {
		t.Error("a drawn map has a record")
	}
}
