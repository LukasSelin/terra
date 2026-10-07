package terra

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// The land's relations: see relations_land.go.

// landEnds is the kinds of feature each of the land's relations joins: the
// first of its From's, the rest of its To's.
var landEnds = map[RelationKind][]FeatureKind{
	Shadows:    {UpliftBelt, DrainageBasin, ClimateRegion},
	Fills:      {DrainageBasin, StandingLake},
	Grows:      {ClimateRegion, Woodland},
	Raises:     {CrustPlate, UpliftBelt},
	DrainsInto: {DrainageBasin, SeaCurrent, Upwelling, Gyre},
}

// landUnits is the unit each of the land's relations carries.
var landUnits = map[RelationKind]string{Shadows: "mm", Fills: "m³/s", Grows: "share", Raises: "m", DrainsInto: "m³/s"}

// checkLandRelations fails t where one of g's land relations joins features
// of kinds it does not join, carries the wrong unit or is under its floor,
// or is not what the record it reads says; and returns the count of each
// kind.
func checkLandRelations(t *testing.T, g *Grid) map[RelationKind]int {
	t.Helper()
	f := g.Features()
	count := map[RelationKind]int{}
	for _, r := range f.Relations() {
		ends, ok := landEnds[r.Kind]
		if !ok {
			continue
		}
		count[r.Kind]++
		from, to := g.Feature(r.From), g.Feature(r.To)
		if from.Kind != ends[0] || !slices.Contains(ends[1:], to.Kind) {
			t.Fatalf("%+v joins a %v and a %v", r, from.Kind, to.Kind)
		}
		if r.Unit != landUnits[r.Kind] {
			t.Errorf("%v has unit %q", r.Kind, r.Unit)
		}
		switch r.Kind {
		case Shadows:
			if r.Quantity < shadowFloor {
				t.Errorf("%+v is under the floor", r)
			}
		case Fills:
			if in := g.Lakes[to.Lake].Inflow; r.Quantity != in || in <= 0 || g.featureAt(int(to.First), DrainageBasin) != from.ID {
				t.Errorf("%+v: the lake's inflow is %v, its basin %d", r, in, g.featureAt(int(to.First), DrainageBasin))
			}
		case Grows:
			n := 0
			for _, i := range from.Tiles {
				if g.featureAt(int(i), Woodland) == to.ID {
					n++
				}
			}
			if s := float64(n) / float64(from.Count); s != r.Quantity {
				t.Errorf("%+v: the wood covers %v of the region", r, s)
			}
		case Raises:
			if r.Quantity != to.Lift || (g.PlateOf(to.Plates[0]) != r.From && g.PlateOf(to.Plates[1]) != r.From) {
				t.Errorf("%+v: the belt was raised %v between plates %v", r, to.Lift, to.Plates)
			}
		case DrainsInto:
			if r.Quantity != from.Flow || float64(from.Count) < drainTiles {
				t.Errorf("%+v: the basin's outlet carries %v", r, from.Flow)
			}
		}
	}
	// Every lake with water coming into it is filled, and every belt is
	// raised by a plate.
	filled, raised := map[FeatureID]bool{}, map[FeatureID]bool{}
	for _, r := range f.Relations() {
		filled[r.To] = filled[r.To] || r.Kind == Fills
		raised[r.To] = raised[r.To] || r.Kind == Raises
	}
	for k := range f.All {
		fe := &f.All[k]
		switch {
		case fe.Kind == StandingLake && g.Lakes[fe.Lake].Inflow > 0 && g.featureAt(int(fe.First), DrainageBasin) > 0 && !filled[fe.ID]:
			t.Errorf("lake %d has %v m³/s coming in and no basin fills it", fe.ID, g.Lakes[fe.Lake].Inflow)
		case fe.Kind == UpliftBelt && !raised[fe.ID]:
			t.Errorf("belt %d, between plates %v, is raised by no plate", fe.ID, fe.Plates)
		}
	}
	return count
}

// checkShadowWhy fails t unless Why's rain, at the tile of the strongest
// shadow's ground the belt shadows most, names a range that wrung out at
// least what the relation says it wrung out on the whole of that ground.
func checkShadowWhy(t *testing.T, g *Grid) {
	t.Helper()
	f := g.Features()
	var best Relation
	for _, r := range f.Relations() {
		if r.Kind == Shadows && r.Quantity > best.Quantity {
			best = r
		}
	}
	if best.Kind != Shadows {
		t.Error("no range shadows anything")
		return
	}
	s := g.shadowReader(f)
	at, most := -1, 0.0
	for _, i := range g.Feature(best.To).Tiles {
		for _, p := range s.cell(s.e.CellOfTile(int(i))) {
			if p.belt == best.From && p.mm > most {
				at, most = int(i), p.mm
			}
		}
	}
	if at < 0 {
		t.Fatalf("%+v: no tile of the ground is in the belt's shadow", best)
	}
	for _, c := range g.Why(g.PosOf(at), OfRain) {
		if c.Kind == RainShadow {
			if c.Quantity < best.Quantity || c.Feature == 0 {
				t.Errorf("tile %d: the rain shadow is %v mm of %d, under the %v mm %d wrings out on its ground", at, c.Quantity, c.Feature, best.Quantity, best.From)
			}
			t.Logf("the strongest shadow, %d on %v %d, %.0f mm; at tile %d the chain names %d, %.0f mm", best.From, g.Feature(best.To).Kind, best.To, best.Quantity, at, c.Feature, c.Quantity)
			return
		}
	}
	t.Errorf("tile %d: the rain's chain names no rain shadow", at)
}

// A valley run through its history has its belts raised by its plates, its
// lakes filled by their basins, its regions growing their woods and its
// ranges' rain shadows, each with the number it reads; and none of them
// depends on the goroutines the world was made over.
func TestTheLandsRelations(t *testing.T) {
	g := yardWorld("ancient", 1, AncientTerms())
	count := checkLandRelations(t, g)
	t.Logf("the ancient valley's land relations: %v", count)
	for _, k := range []RelationKind{Raises, Fills, Grows} {
		if count[k] == 0 {
			t.Errorf("an ancient valley has no %v relations", k)
		}
	}
	checkShadowWhy(t, g)
	drawn := yardWorld("valley", 1, DefaultTerms())
	dc := checkLandRelations(t, drawn)
	t.Logf("the drawn valley's: %v", dc)
	if dc[Raises] != 0 || dc[Shadows] != 0 {
		t.Errorf("a drawn valley has belts: %v", dc)
	}

	all := g.Features().Relations()
	was := Workers
	defer func() { Workers = was }()
	for _, w := range []int{1, 8} {
		Workers = w
		if other := madeLand(1, AncientTerms()).Grid.Features().Relations(); !reflect.DeepEqual(other, all) {
			t.Errorf("over %d goroutines the relations are not the same", w)
		}
	}
}

// The deserts are explained: on a made globe every dry climate region has a
// recorded reason to be dry - the cold water off its coast (Dries) or a
// range upwind of it (Shadows) - or is listed here as one that has none.
// What is left is the dry country the subtropical high makes on its own,
// which no feature does: the air sinking over the Sahara is the air's and
// not a range's or a current's. The test fails only where a region of a
// thousand tiles or more has no reason, which is a desert the size of a
// country with nothing recorded for it.
func TestTheDryRegionsAreExplained(t *testing.T) {
	if testing.Short() {
		t.Skip("a globe; see docs/perf/suite.md")
	}
	g := yardWorld("globe", 3, GlobeTerms())
	f := g.Features()
	checkLandRelations(t, g)
	checkShadowWhy(t, g)
	all := map[RelationKind]int{}
	for _, r := range f.Relations() {
		all[r.Kind]++
	}
	t.Logf("%d relations: %v", len(f.Relations()), all)

	var explained, unexplained []*Feature
	ways := map[string]int{}
	tiles := [2]int{}
	for k := range f.All {
		fe := &f.All[k]
		if fe.Kind != ClimateRegion || fe.Group != 'B' {
			continue
		}
		var why []string
		by := map[RelationKind]bool{}
		for _, r := range f.RelationsOf(fe.ID) {
			if r.To == fe.ID && (r.Kind == Dries || r.Kind == Shadows) {
				why = append(why, fmt.Sprintf("%v by %v %d (%.3g %s)", r.Kind, g.Feature(r.From).Kind, r.From, r.Quantity, r.Unit))
				by[r.Kind] = true
			}
		}
		switch {
		case by[Dries] && by[Shadows]:
			ways["both"]++
		case by[Dries]:
			ways["dries alone"]++
		case by[Shadows]:
			ways["shadows alone"]++
		}
		lat := g.air.Lat[int(fe.First)/g.W]
		if len(why) > 0 {
			explained = append(explained, fe)
			tiles[0] += fe.Count
			if fe.Count >= 100 {
				t.Logf("dry region %d, %d tiles at %.0f degrees: %s", fe.ID, fe.Count, lat, strings.Join(why, "; "))
			}
			continue
		}
		unexplained = append(unexplained, fe)
		tiles[1] += fe.Count
	}
	t.Logf("%d dry regions of %d tiles are explained (%v), %d of %d tiles are not", len(explained), tiles[0], ways, len(unexplained), tiles[1])
	slices.SortFunc(unexplained, func(a, b *Feature) int { return b.Count - a.Count })
	for _, fe := range unexplained {
		lat := g.air.Lat[int(fe.First)/g.W]
		rain := 0.0
		for _, i := range fe.Tiles {
			rain += g.rain[i]
		}
		t.Logf("known gap: dry region %d, %d tiles at %.0f degrees, %.0f mm a year", fe.ID, fe.Count, lat, rain/float64(fe.Count))
		if fe.Count >= 1000 {
			t.Errorf("dry region %d of %d tiles at %.0f degrees has nothing recorded to make it dry", fe.ID, fe.Count, lat)
		}
	}
}
