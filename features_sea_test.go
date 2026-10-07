package terra

import (
	"fmt"
	"math"
	"reflect"
	"testing"
)

// The sea's features: the currents, the gyres and the upwellings, read off
// the weather's currents. See features_sea.go.

// seaOf is the sea's features of g read afresh off the weather w, numbered
// from 1, and the registry they were read into.
func seaOf(g *Grid, w *Winds) ([]Feature, *Features) {
	f := &Features{}
	all, _ := g.readSeaFeatures(f, nil, nil, w)
	for k := range all {
		all[k].ID = FeatureID(k + 1)
	}
	return all, f
}

// poleward reports whether a current heading that many degrees from north
// runs toward the pole at latitude lat.
func poleward(heading float32, lat float64) bool {
	return math.Cos(float64(heading)*math.Pi/180)*lat > 0
}

// On two oceans between two continents that run from pole to pole, each
// ocean has a gyre in each hemisphere turning the way the trades and the
// westerlies turn it - clockwise in the north, anticlockwise in the south -
// with a warm current up its western side toward the pole, and a cold one
// down its eastern side.
//
// Known gap (M1 x O2, #120): the cold current is there, and is not counted
// as the gyre's. M1's gyres (#19) gather the water they turn against the
// western shore, Stommel's and Munk's way, and a current is one of a gyre's
// only if it lies within gyreReach, 1000 km, of the gyre's tiles, those with
// 5 Sv or more between them and the shore. Down the eastern shore the water
// runs equatorward and cold, -1.7 degrees under its latitude's mean at 0.02
// m/s, from 51 degrees toward the gyre's centre, but some 1800 km east of
// where the gyre's tiles end. The remedy is the reading's (#120), not the
// flow's. Until then the test holds what is there: every gyre has its warm
// western current, and every gyre without a cold current of its own has one
// down the eastern shore of its ocean, cold and running equatorward, that is
// no gyre's. coldUncounted is how many gyres read so; it fails if a cold
// current goes, and fails when the gap closes, so that the marker comes off.
func TestTwoOceansHaveTheirGyresAndTheirCurrents(t *testing.T) {
	// Four of the four on M1 (#73). On M2 (#79) the thermocline's Ekman
	// pumping carries the gyre's water to the eastern shore, and every gyre
	// counts its cold current: the gap is closed here, and the marker holds
	// it closed.
	const coldUncounted = 0
	g := twoOceans()
	g.weather()
	all, reg := seaOf(g, g.winds)
	var gyres []Feature
	for _, fe := range all {
		if fe.Kind == Gyre && fe.Class == Subtropical {
			gyres = append(gyres, fe)
		}
	}
	// Two oceans, two hemispheres.
	if len(gyres) != 4 {
		t.Fatalf("%d subtropical gyres, not 4", len(gyres))
	}
	uncounted := 0
	for _, gy := range gyres {
		lat := g.air.Lat[int(gy.Centre)/g.W]
		x := int(gy.Centre) % g.W
		t.Logf("gyre %d at %.0f degrees, column %d: sense %+d, %.1f Sv", gy.ID, lat, x, gy.Sense, gy.Transport)
		want := int8(-1) // clockwise in the north
		if lat < 0 {
			want = 1
		}
		if gy.Sense != want {
			t.Errorf("the gyre at %.0f degrees turns %+d", lat, gy.Sense)
		}
		var warm, cold bool
		for _, c := range all {
			if c.Kind != SeaCurrent || c.Gyre != gy.ID {
				continue
			}
			switch c.Class {
			case WesternBoundary:
				warm = warm || (poleward(c.Heading, lat) && c.Warmth > 0)
			case EasternBoundary:
				cold = cold || (!poleward(c.Heading, lat) && c.Warmth < 0)
			}
		}
		if !warm {
			t.Errorf("the gyre at %.0f degrees, column %d: a warm current up its west %v, a cold one down its east %v", lat, x, warm, cold)
		}
		if cold {
			continue
		}
		// The gap: a cold current down the eastern shore of the gyre's ocean,
		// in the rows the gyre spans, that is no gyre's.
		top, bottom := g.H, -1
		for i, p := range reg.gyre {
			if p > 0 && reg.seaBase[1]+FeatureID(p) == gy.ID {
				top, bottom = min(top, i/g.W), max(bottom, i/g.W)
			}
		}
		var east *Feature
		for k := range all {
			c := &all[k]
			if c.Kind != SeaCurrent || c.Class != EasternBoundary || c.Gyre != 0 ||
				poleward(c.Heading, lat) || c.Warmth >= 0 || g.air.Lat[int(c.First)/g.W]*lat <= 0 {
				continue
			}
			if dx := (int(c.First)%g.W - x + g.W) % g.W; dx == 0 || dx >= g.W/2 {
				continue
			}
			for _, i := range c.Path {
				if y := int(i) / g.W; y >= top && y <= bottom {
					east = c
					break
				}
			}
			if east != nil {
				break
			}
		}
		if east == nil {
			t.Errorf("the gyre at %.0f degrees, column %d: no cold current down its east, its own or any", lat, x)
			continue
		}
		uncounted++
		t.Logf("known gap (#120): the gyre at %.0f degrees, column %d: its cold eastern current is no gyre's: %s", lat, x, describeCurrent(g, east))
	}
	switch {
	case uncounted > coldUncounted:
		t.Errorf("%d gyres have a cold eastern current that is not theirs, where the known gap (#120) is %d", uncounted, coldUncounted)
	case uncounted < coldUncounted:
		t.Errorf("%d gyres have a cold eastern current that is not theirs, where the known gap (#120) is %d: it has closed, so lower coldUncounted or take the marker off", uncounted, coldUncounted)
	}
}

// The real world's oceans each have a warm current running up their western
// side toward the pole - the Gulf Stream, the Kuroshio, the Brazil, the East
// Australian, the Agulhas - and a cold one coming back down their eastern
// side. On a made globe every subtropical gyre has at least one western
// boundary current, warm and running poleward, warmer than each of the
// eastern boundary currents of the same gyre.
func TestEveryOceanHasItsWarmWesternCurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("a globe; see docs/perf/suite.md")
	}
	g := yardWorld("globe", 3, GlobeTerms())
	f := g.Features()
	count := map[FeatureKind]int{}
	classes := map[SeaClass]int{}
	for _, fe := range f.All {
		count[fe.Kind]++
		if fe.Kind == SeaCurrent || fe.Kind == Gyre {
			classes[fe.Class]++
		}
	}
	t.Logf("%d currents, %d gyres, %d upwellings: %v", count[SeaCurrent], count[Gyre], count[Upwelling], classes)
	gyres := 0
	for _, gy := range f.All {
		if gy.Kind != Gyre || gy.Class != Subtropical {
			continue
		}
		gyres++
		lat := g.air.Lat[int(gy.Centre)/g.W]
		var west *Feature
		east := math.Inf(-1)
		easts := 0
		for k := range f.All {
			c := &f.All[k]
			if c.Kind != SeaCurrent || c.Gyre != gy.ID {
				continue
			}
			switch c.Class {
			case WesternBoundary:
				if poleward(c.Heading, lat) && c.Warmth > 0 && (west == nil || c.Warmth > west.Warmth) {
					west = c
				}
			case EasternBoundary:
				east = math.Max(east, float64(c.Warmth))
				easts++
			}
		}
		t.Logf("the gyre at %.0f degrees, %.0f Sv: its warmest western current %v, the warmest of its %d eastern %+.2f", lat, gy.Transport, describeCurrent(g, west), easts, east)
		switch {
		case west == nil:
			t.Errorf("the gyre at %.0f degrees has no warm western current running poleward", lat)
		case easts == 0:
			t.Errorf("the gyre at %.0f degrees has no eastern current", lat)
		case float64(west.Warmth) <= east:
			t.Errorf("the gyre at %.0f degrees: its western current is %+.2f degrees, an eastern one %+.2f", lat, west.Warmth, east)
		}
	}
	if gyres == 0 {
		t.Error("the globe has no subtropical gyre")
	}
}

// describeCurrent is a current's numbers, for the log.
func describeCurrent(g *Grid, c *Feature) string {
	if c == nil {
		return "none"
	}
	x, y := int(c.First)%g.W, int(c.First)/g.W
	return fmt.Sprintf("%d from (%d, %d) at %.0f degrees: %.2f m/s at %.0f degrees, %.1f Sv, %+.2f degrees, %d tiles",
		c.ID, x, y, g.air.Lat[y], c.Flow, c.Heading, c.Transport, c.Warmth, c.Count)
}

// The same weather gives the same features, however many goroutines worked
// the weather out, and reading them twice gives them twice.
func TestTheSeaFeaturesDoNotDependOnTheGoroutines(t *testing.T) {
	if testing.Short() {
		t.Skip("a globe's weather twice over; see docs/perf/suite.md")
	}
	g := yardWorld("globe", 3, GlobeTerms())
	read := func(workers int) ([]Feature, *Features) {
		was := Workers
		Workers = workers
		defer func() { Workers = was }()
		return seaOf(g, g.windsFor())
	}
	one, fone := read(1)
	again, _ := seaOf(g, g.winds)
	twice, _ := seaOf(g, g.winds)
	if !reflect.DeepEqual(again, twice) {
		t.Error("the same weather read twice gave different features")
	}
	if len(again) == 0 {
		t.Fatal("the globe's sea has no features")
	}
	eight, feight := read(8)
	if !reflect.DeepEqual(one, eight) || !reflect.DeepEqual(fone.current, feight.current) ||
		!reflect.DeepEqual(fone.gyre, feight.gyre) || !reflect.DeepEqual(fone.upwell, feight.upwell) {
		t.Error("the weather worked out on one goroutine and on eight gave different features")
	}
	// And the registry's are the ones the registry was built with.
	f := g.Features()
	for k := range again {
		fe, mine := f.All[int(f.seaBase[0])+k], again[k]
		if fe.Kind != mine.Kind || fe.First != mine.First || fe.Class != mine.Class || fe.Flow != mine.Flow {
			t.Fatalf("the registry's sea feature %d is %v %v at %d, read again %v %v at %d",
				fe.ID, fe.Kind, fe.Class, fe.First, mine.Kind, mine.Class, mine.First)
		}
	}
}

// A valley has no ocean worked out, and so no sea's features.
func TestAValleyHasNoSeaFeatures(t *testing.T) {
	g := yardWorld("valley", 1, DefaultTerms())
	for _, fe := range g.Features().All {
		if fe.Kind >= SeaCurrent && fe.Kind <= Upwelling {
			t.Fatalf("a valley has a %v", fe.Kind)
		}
	}
	for i := range g.Tiles {
		for k := SeaCurrent; k <= Upwelling; k++ {
			if id := g.FeatureOf(i, k); id != 0 {
				t.Fatalf("tile %d of a valley is in %v %d", i, k, id)
			}
		}
	}
}
