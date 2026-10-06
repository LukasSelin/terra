package terra

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
)

// The earth's land curve goes up, and each of its heights is a height.
func TestTheEarthsLandRisesAllTheWay(t *testing.T) {
	last := -1.0
	for k := 0; k <= 1000; k++ {
		h := earthHeightAt(float64(k) / 1000)
		if h < last {
			t.Fatalf("the earth's land falls from %.1f m to %.1f m at %.3f of the way up", last, h, float64(k)/1000)
		}
		last = h
	}
	if lo, hi := earthHeightAt(0), earthHeightAt(1); lo != 0 || hi != 8000 {
		t.Errorf("the earth's land runs from %.0f m to %.0f m, not from the sea to 8 km", lo, hi)
	}
}

// A map that is not a globe has no country under it, and its elevation is
// its height: a valley keeps its drawn spread whole.
func TestAValleyStandsOnNoCountry(t *testing.T) {
	g := yardWorld("valley", 1, DefaultTerms())
	if g.country != nil {
		t.Fatal("a valley was given a country")
	}
	for i := range g.Tiles {
		if g.Elevation(i) != g.Height[i] || g.airHeight(i) != g.laidHeight(i) {
			t.Fatalf("tile %d stands at %.2f m on a valley whose ground is %.2f m", i, g.Elevation(i), g.Height[i])
		}
	}
}

// landHeights is the dry land of g by elevation above its sea, in metres,
// lowest first, each tile weighted by the ground it stands for on a sphere.
func landHeights(g *Grid) (h, weight []float64) {
	type at struct{ h, w float64 }
	var land []at
	for i := range g.Tiles {
		if g.Tiles[i].Wet() || g.sunk(i) {
			continue
		}
		land = append(land, at{g.Elevation(i) - g.sea, math.Cos(latitudeOf(g, i/g.W) * math.Pi / 180)})
	}
	slices.SortFunc(land, func(a, b at) int {
		switch {
		case a.h < b.h:
			return -1
		case a.h > b.h:
			return 1
		}
		return 0
	})
	for _, l := range land {
		h, weight = append(h, l.h), append(weight, l.w)
	}
	return h, weight
}

// weightedAt is the height share f of the weight of the land stands lower
// than.
func weightedAt(h, weight []float64, f float64) float64 {
	total := 0.0
	for _, w := range weight {
		total += w
	}
	run := 0.0
	for k, w := range weight {
		run += w
		if run >= f*total {
			return h[k]
		}
	}
	return h[len(h)-1]
}

// The globe's land against the earth's: a lowland heaped just above the sea
// and a tail to several kilometres (Cogley 1984), which is what the country
// under it is laid to and what the map's own ground on top of it moves it
// from. Logged at the earth's quantiles, beside the earth's, with the share
// of the land in each of ETOPO5's bands; held only to the acceptance of G1,
// that the highest ranges stand in kilometres and the middle of the land a
// few hundred metres up.
func TestTheGlobeStandsAtTheEarthsHeights(t *testing.T) {
	if testing.Short() {
		t.Skip("makes globes")
	}
	shares := []float64{0.05, 0.1, 0.25, 0.5, 0.75, 0.9, 0.95, 0.99, 0.999, 1}
	var b strings.Builder
	fmt.Fprintf(&b, "%-16s", "land, m")
	for _, f := range shares {
		fmt.Fprintf(&b, "%8g", f)
	}
	fmt.Fprintf(&b, "\n%-16s", "earth")
	for _, f := range shares {
		fmt.Fprintf(&b, "%8.0f", earthHeightAt(f))
	}
	bands := []float64{0, 200, 500, 1000, 2000, 3000, 4000, 5000, math.Inf(1)}
	type reading struct {
		name   string
		middle float64
		top    float64
	}
	var read []reading
	var banded []string
	for _, c := range []struct {
		name  string
		seed  uint64
		terms Terms
	}{{"globe", 1, GlobeTerms()}, {"globe", 2, GlobeTerms()}, {"globe", 3, GlobeTerms()}, {"small", 1, smallGlobe()}, {"small", 2, smallGlobe()}} {
		g := yardWorld(c.name, c.seed, c.terms)
		h, w := landHeights(g)
		if len(h) == 0 {
			t.Fatalf("%s %d has no land", c.name, c.seed)
		}
		name := fmt.Sprintf("%s %d", c.name, c.seed)
		fmt.Fprintf(&b, "\n%-16s", name)
		for _, f := range shares {
			fmt.Fprintf(&b, "%8.0f", weightedAt(h, w, f))
		}
		read = append(read, reading{name, weightedAt(h, w, 0.5), h[len(h)-1]})
		var in strings.Builder
		fmt.Fprintf(&in, "%-16s", name)
		total := 0.0
		for _, x := range w {
			total += x
		}
		for k := 1; k < len(bands); k++ {
			s := 0.0
			for j, x := range h {
				if x >= bands[k-1] && x < bands[k] || k == 1 && x < 0 {
					s += w[j]
				}
			}
			fmt.Fprintf(&in, "%8.3f", s/total)
		}
		banded = append(banded, in.String())
	}
	var e strings.Builder
	fmt.Fprintf(&e, "%-16s", "earth")
	for k := 1; k < len(bands); k++ {
		lo := earthShareUnder(bands[k-1])
		fmt.Fprintf(&e, "%8.3f", earthShareUnder(bands[k])-lo)
	}
	t.Logf("the land's elevation above its sea, at shares of the land:\n%s", b.String())
	t.Logf("the share of the land in bands of 0, 200, 500 m, 1, 2, 3, 4, 5 km and over:\n%s\n%s", e.String(), strings.Join(banded, "\n"))
	for _, r := range read {
		if r.top < 3000 {
			t.Errorf("%s: the highest land stands %.0f m above the sea, short of the kilometres its ranges were raised to", r.name, r.top)
		}
		if r.middle < 100 || r.middle > 1000 {
			t.Errorf("%s: half the land stands under %.0f m, where the earth's is under %.0f", r.name, r.middle, earthHeightAt(0.5))
		}
	}
}

// earthShareUnder is the share of the earth's land under h metres.
func earthShareUnder(h float64) float64 {
	if math.IsInf(h, 1) {
		return 1
	}
	for k := 1; k < len(earthHeights); k++ {
		lo, hi := earthHeights[k-1], earthHeights[k]
		if h <= hi.height {
			return lo.share + (hi.share-lo.share)*(h-lo.height)/(hi.height-lo.height)
		}
	}
	return 1
}
