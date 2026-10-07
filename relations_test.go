package terra

import (
	"math"
	"reflect"
	"testing"

	"github.com/LukasSelin/terra/internal/atmos"
)

// What the features do to one another: see relations.go.

// relatedOceans is twoOceans with its weather, a year written down on every
// tile the way stageCoast writes it but for the sea about the tile - its
// latitude's mean and what the currents offshore make of it - and its
// registry read, so that its climate regions and the sea's features have
// their relations.
func relatedOceans() *Grid {
	g := twoOceans()
	g.weather()
	n := len(g.Tiles)
	g.lakeOf = make([]int32, n)
	g.warm, g.swing = make([]float32, n), make([]float32, n)
	for i := range n {
		g.lakeOf[i] = -1
		y := i / g.W
		g.warm[i] = float32(g.air.Mean[y] + g.CoastWarmth(i))
		g.swing[i] = float32(atmos.SwingAt(g.air.Lat[y], g.contAt(i)))
	}
	g.readFeatures()
	return g
}

// upstream is every current whose water reaches current id along the Feeds
// relations, nearest first, with the way each is reached: for each, the
// current it feeds on the way to id.
func upstream(f *Features, id FeatureID) ([]FeatureID, map[FeatureID]FeatureID) {
	next := map[FeatureID]FeatureID{id: 0}
	queue := []FeatureID{id}
	var found []FeatureID
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		for _, r := range f.RelationsOf(at) {
			if r.Kind != Feeds || r.To != at {
				continue
			}
			if _, seen := next[r.From]; seen {
				continue
			}
			next[r.From] = at
			found = append(found, r.From)
			queue = append(queue, r.From)
		}
	}
	return found, next
}

// related reports the relation of kind k from one feature to another, if
// there is one.
func related(f *Features, from, to FeatureID, k RelationKind) (Relation, bool) {
	for _, r := range f.RelationsOf(from) {
		if r.From == from && r.To == to && r.Kind == k {
			return r, true
		}
	}
	return Relation{}, false
}

// A continent's west coast in the subpolar westerlies is mild for the water
// the drift brings it, and the drift's water came up the far side of the
// ocean in a western boundary current: the coast of Norway, the North
// Atlantic Drift and the Gulf Stream. TestTheSubpolarWestCoastIsMild's
// tile is warmed most by a current, its climate region is Warms-related to
// that current, and the Feeds relations lead back from it to a warm western
// boundary current.
//
// Known gap (M1 x O3, for M3, #22): the gyres are solved linear (Stommel and
// Munk), and no water crosses the line of nought ψ between two gyres. The
// 58°N coast's water comes, along Feeds, from the subpolar drift (-4.64
// degrees, 14 Sv) and the subpolar gyre's own western current, going south
// (-2.34); the warm western current (+4.11, 28 Sv) feeds the subtropical
// gyre's drift at 43°N (+2.84, 6 Sv), which leads nowhere near the coast.
// On the earth the Gulf Stream crosses into the North Atlantic Drift by
// inertial overshoot and the eddies at the gyres' boundary, and the
// overturning carries some fifteen sverdrups north across it. Eddy diffusion
// of the warmth across that boundary (an eddy diffusivity of 1000-2000 m²/s,
// Abernathey and Marshall, 2013), cheap in the warmth's solve, would move
// warmth and not water: Feeds follows the current (Grid.follow), so it would
// leave this chain as it is, and is M3's to add with the rest of its eddy
// mixing. What this test asks for needs water across the boundary: an
// inertial term in the gyres, or the overturning (#23, #24).
func TestTheMildWestCoastIsWarmedFromAWesternCurrent(t *testing.T) {
	g := relatedOceans()
	f := g.Features()
	for _, lat := range []float64{58, -58} {
		y := 0
		for k := range g.H {
			if math.Abs(g.air.Lat[k]-lat) < math.Abs(g.air.Lat[y]-lat) {
				y = k
			}
		}
		i := y*g.W + 130
		region := g.featureAt(i, ClimateRegion)
		var coast Cause
		for _, c := range g.Why(g.PosOf(i), OfWarmth) {
			if c.Kind == CoastWarmth {
				coast = c
			}
		}
		if coast.Feature == 0 || coast.Quantity <= 0 {
			t.Errorf("at %v degrees the coast is warmed %+.2f degrees, by %d", lat, coast.Quantity, coast.Feature)
			continue
		}
		r, ok := related(f, coast.Feature, region, Warms)
		if !ok {
			t.Errorf("at %v degrees %v %d warms the tile and not its climate region %d", lat, g.Feature(coast.Feature).Class, coast.Feature, region)
			continue
		}
		found, next := upstream(f, coast.Feature)
		var west FeatureID
		for _, id := range found {
			if c := g.Feature(id); c.Class == WesternBoundary && c.Warmth > 0 {
				west = id
				break
			}
		}
		if west == 0 {
			t.Errorf("at %v degrees nothing leads back from %v %d to a warm western boundary current", lat, g.Feature(coast.Feature).Class, coast.Feature)
			continue
		}
		chain := ""
		for id := west; id != 0; id = next[id] {
			c := g.Feature(id)
			chain += " " + describeCurrent(g, c) + " ->"
		}
		t.Logf("at %v degrees the coast is %+.2f degrees warmer, its region %+.2f by %d; the water came%s the coast", lat, coast.Quantity, r.Quantity, coast.Feature, chain)
	}
}

// Every relation is found from both its ends, and only there; the kinds
// carry their units; and the relations are read off the sea the same way
// whichever goroutines worked the weather out.
func TestRelationsAreFoundFromBothEnds(t *testing.T) {
	g := relatedOceans()
	f := g.Features()
	all := f.Relations()
	if len(all) == 0 {
		t.Fatal("two oceans have no relations")
	}
	count := map[RelationKind]int{}
	for k, r := range all {
		count[r.Kind]++
		if k > 0 {
			p := all[k-1]
			if p.From > r.From || (p.From == r.From && (p.Kind > r.Kind || (p.Kind == r.Kind && p.To >= r.To))) {
				t.Fatalf("relations %d and %d are out of order: %+v, %+v", k-1, k, p, r)
			}
		}
		unit := map[RelationKind]string{Warms: "°C", Cools: "°C", Dries: "share", Waters: "share", PartOf: "", Feeds: "Sv",
			Shadows: "mm", Fills: "m³/s", Grows: "share", Raises: "m", DrainsInto: "m³/s", Subsides: "mm/s"}[r.Kind]
		if r.Unit != unit {
			t.Errorf("%v has unit %q", r.Kind, r.Unit)
		}
		if (r.Kind == Warms) != (r.Quantity > 0) && (r.Kind == Warms || r.Kind == Cools) {
			t.Errorf("%v by %v", r.Kind, r.Quantity)
		}
		for _, end := range []FeatureID{r.From, r.To} {
			if !reflect.DeepEqual(r, find(f.RelationsOf(end), r)) {
				t.Fatalf("%+v is not among the relations of %d", r, end)
			}
		}
	}
	t.Logf("%d relations: %v", len(all), count)
	for _, k := range []RelationKind{Warms, Cools, PartOf, Feeds} {
		if count[k] == 0 {
			t.Errorf("two oceans have no %v relations", k)
		}
	}
	n := 0
	for id := range f.All {
		n += len(f.RelationsOf(FeatureID(id + 1)))
	}
	if n != 2*len(all) {
		t.Errorf("the features have %d relations between them, not twice %d", n, len(all))
	}

	was := Workers
	defer func() { Workers = was }()
	Workers = 1
	one := relatedOceans().Features().Relations()
	Workers = 8
	eight := relatedOceans().Features().Relations()
	if !reflect.DeepEqual(one, all) || !reflect.DeepEqual(eight, all) {
		t.Error("the relations depend on the goroutines the weather was worked out over")
	}
}

// find is r among rels, or nothing.
func find(rels []Relation, r Relation) Relation {
	for _, s := range rels {
		if s == r {
			return s
		}
	}
	return Relation{}
}

// The warmth of a year comes to its causes: the latitude's mean, the sea
// about the tile, the currents offshore and the height.
func TestTheWarmthComesToItsCauses(t *testing.T) {
	for _, g := range []*Grid{relatedOceans(), yardWorld("valley", 1, DefaultTerms())} {
		for i := 0; i < len(g.Tiles); i += 97 {
			chain := g.Why(g.PosOf(i), OfWarmth)
			if len(chain) < 6 || chain[0].Kind != Warmth || chain[len(chain)-1].Kind != Altitude {
				t.Fatalf("tile %d: the chain is %v", i, chain)
			}
			sum := 0.0
			for _, c := range chain {
				switch c.Kind {
				case LatitudeWarmth, SeaAbout, CoastWarmth, Altitude:
					sum += c.Quantity
				}
			}
			if math.Abs(sum-chain[0].Quantity) > 1e-4 {
				t.Fatalf("tile %d: the causes come to %v and the year's mean is %v", i, sum, chain[0].Quantity)
			}
		}
	}
}

// A valley has no sea's features, and so none of the sea's relations: only
// the land's (see relations_land.go).
func TestAValleyHasNoSeaRelations(t *testing.T) {
	g := yardWorld("valley", 1, DefaultTerms())
	for _, r := range g.Features().Relations() {
		if _, land := landEnds[r.Kind]; !land {
			t.Fatalf("a valley has a %v relation: %+v", r.Kind, r)
		}
	}
}

// The coast of Peru and of Namibia is desert with the sea in sight: the air
// over the cold water that comes up off it lies under an inversion and does
// not rain. On a made globe the largest dry climate region the strongest
// upwelling dries is Dries-related to it, and Why's rain names it at the
// tile of the region it chills most.
func TestTheDesertBesideTheStrongestUpwellingIsDriedByIt(t *testing.T) {
	if testing.Short() {
		t.Skip("a globe; see docs/perf/suite.md")
	}
	g := yardWorld("globe", 3, GlobeTerms())
	f := g.Features()
	var up *Feature
	for k := range f.All {
		if fe := &f.All[k]; fe.Kind == Upwelling && (up == nil || fe.Flow > up.Flow) {
			up = fe
		}
	}
	if up == nil {
		t.Fatal("the globe has no upwelling")
	}
	y := int(up.First) / g.W
	t.Logf("the strongest upwelling, %d, at %.0f degrees: %.2g m/s, %+.2f degrees, %d tiles", up.ID, g.air.Lat[y], up.Flow, up.Warmth, up.Count)
	var desert *Feature
	var dries Relation
	for _, r := range f.RelationsOf(up.ID) {
		if to := g.Feature(r.To); r.Kind == Dries && to.Group == 'B' && (desert == nil || to.Count > desert.Count) {
			desert, dries = to, r
		}
	}
	if desert == nil {
		t.Fatalf("the strongest upwelling dries no desert: %v", f.RelationsOf(up.ID))
	}
	t.Logf("it dries desert %d of %d tiles: the inversion takes %.2f of its rain", desert.ID, desert.Count, dries.Quantity)
	// The tile of the desert it chills most, of those it chills more than
	// any other upwelling does.
	r := g.seaReader(f)
	at, most := -1, 0.0
	for _, t := range desert.Tiles {
		r.tileParts(int(t), nil)
		if id, d := r.strongest(true, -1); id == up.ID && d < most {
			at, most = int(t), d
		}
	}
	if at < 0 {
		t.Fatal("no tile of the desert is chilled most by the strongest upwelling")
	}
	chain := g.Why(g.PosOf(at), OfRain)
	named := false
	for _, c := range chain {
		t.Logf("  %v %d: %.3g %s %s", c.Kind, c.Feature, c.Quantity, c.Unit, c.Note)
		named = named || (c.Kind == Inversion && c.Feature == up.ID && c.Quantity > 0)
	}
	if !named {
		t.Errorf("the rain at tile %d of the desert does not name the upwelling", at)
	}
}

// On a made globe the relations are the same read twice, and every one is
// between features of the kinds its kind joins.
func TestTheGlobesRelations(t *testing.T) {
	if testing.Short() {
		t.Skip("a globe; see docs/perf/suite.md")
	}
	g := yardWorld("globe", 3, GlobeTerms())
	f := g.Features()
	all := f.Relations()
	ends := map[RelationKind][2]FeatureKind{
		Warms: {SeaCurrent, ClimateRegion}, Cools: {SeaCurrent, ClimateRegion}, Dries: {Upwelling, ClimateRegion},
		Waters: {SeaCurrent, DrainageBasin}, PartOf: {SeaCurrent, Gyre}, Feeds: {SeaCurrent, SeaCurrent},
	}
	count := map[RelationKind]int{}
	best := map[RelationKind]Relation{}
	for _, r := range all {
		count[r.Kind]++
		if b, ok := best[r.Kind]; !ok || math.Abs(r.Quantity) > math.Abs(b.Quantity) {
			best[r.Kind] = r
		}
		if _, land := landEnds[r.Kind]; land {
			continue // checkLandRelations's
		}
		if want := ends[r.Kind]; g.Feature(r.From).Kind != want[0] || g.Feature(r.To).Kind != want[1] {
			t.Fatalf("%+v joins a %v and a %v", r, g.Feature(r.From).Kind, g.Feature(r.To).Kind)
		}
	}
	checkLandRelations(t, g)
	t.Logf("%d relations: %v", len(all), count)
	for k := Warms; k < relationKinds; k++ {
		b, ok := best[k]
		if !ok {
			t.Errorf("the globe has no %v relations", k)
			continue
		}
		from, to := g.Feature(b.From), g.Feature(b.To)
		t.Logf("the strongest %v: %v %v %d -> %v %d (%c, %d tiles): %.3g %s", k, from.Class, from.Kind, from.ID, to.Kind, to.ID, to.Group, to.Count, b.Quantity, b.Unit)
	}
	g.readFeatures()
	if !reflect.DeepEqual(all, g.Features().Relations()) {
		t.Error("the relations read twice are not the same")
	}
}
