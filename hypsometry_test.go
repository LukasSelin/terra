package terra

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// The earth's land curve goes up, and each of its heights is a height.
func TestTheEarthsLandRisesAllTheWay(t *testing.T) {
	last := -1.0
	for k := 0; k <= 1000; k++ {
		h := cogleyAt(float64(k) / 1000)
		if h < last {
			t.Fatalf("the earth's land falls from %.1f m to %.1f m at %.3f of the way up", last, h, float64(k)/1000)
		}
		last = h
	}
	if lo, hi := cogleyAt(0), cogleyAt(1); lo != 0 || hi != 8000 {
		t.Errorf("the earth's land runs from %.0f m to %.0f m, not from the sea to 8 km", lo, hi)
	}
}

// The floor the country's lowest fifth is laid to is the earth's own land at
// those shares.
func TestTheCountrysFloorIsTheEarthsLowland(t *testing.T) {
	for k := 0; k <= 100; k++ {
		f := lowShare * float64(k) / 100
		if got, want := lowTop*f/lowFloor, cogleyAt(f); math.Abs(got-want) > 1e-9 {
			t.Fatalf("at %.3f of the land the floor is %.2f m, the earth's land %.2f m", f, got, want)
		}
	}
}

// The country graded along a drainage never rises from a tile to the one its
// water goes to, keeps what it had in all, and pools what it has to pool at
// the mean: a tile standing over the tile above it takes it in.
func TestTheGradedCountryNeverRisesDownstream(t *testing.T) {
	// 0 <- 1 <- 2, and 3 <- 4, 3 <- 5; 6 is outside.
	below := []int32{-1, 0, 1, -1, 3, 3, -2}
	v := []float64{5, 1, 3, 4, 1, 6, 9}
	var fit isotone
	fit.fit(v, func(i int32) int32 { return below[i] }, []int32{0, 3, 6, 1, 4, 5, 2})
	want := []float64{3, 3, 3, 2.5, 2.5, 6, 9}
	for i := range v {
		if math.Abs(v[i]-want[i]) > 1e-12 {
			t.Fatalf("graded %v, want %v", v, want)
		}
	}

	// And on a forest drawn at random.
	r := rand.New(rand.NewPCG(1, 2))
	n := 5000
	below = make([]int32, n)
	v = make([]float64, n)
	order := make([]int32, n)
	sum := 0.0
	for i := range below {
		below[i] = -1
		if i > 0 && r.Float64() < 0.98 {
			below[i] = int32(r.IntN(i))
		}
		v[i] = 1000 * r.Float64()
		order[i] = int32(i)
		sum += v[i]
	}
	fit.fit(v, func(i int32) int32 { return below[i] }, order)
	got := 0.0
	for i := range v {
		got += v[i]
		if d := below[i]; d >= 0 && v[d] > v[i]+1e-9 {
			t.Fatalf("tile %d stands at %.3f under the %.3f of the tile below it", i, v[i], v[d])
		}
	}
	if math.Abs(got-sum) > 1e-6*sum {
		t.Fatalf("the country held %.3f in all and holds %.3f graded", sum, got)
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
		if g.Elevation(i) != g.Height[i] || g.lapseHeight(i) != g.laidHeight(i) {
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
// and a tail to several kilometres (Cogley 1984). The country is the
// history's own height and not the earth's curve (only its lowest fifth is
// laid no lower than the earth's), so this is the curve held as a test, and
// the map's own ground on top of the country moves it a little further.
// Logged at the earth's quantiles, beside the earth's, with the share of the
// land in each of ETOPO5's bands; held to the acceptance of G1, that the
// highest ranges stand in kilometres and the middle of the land a few hundred
// metres up.
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
		fmt.Fprintf(&b, "%8.0f", cogleyAt(f))
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
			t.Errorf("%s: half the land stands under %.0f m, where the earth's is under %.0f", r.name, r.middle, cogleyAt(0.5))
		}
	}
}

// The rivers are graded on Height, which is the map's ground, and the
// country is the history's, which is not the shaping's: so a river's step
// from one tile to the next could climb in Elevation where it falls in
// Height. The country is graded along the drainage for that (gradeCountry),
// and the water climbs it no more often than it climbs the ground. Logged
// as a share of the steps the water takes over dry land, of every tile's
// and of the rivers' - the tiles carrying meanderFlow and more - beside the
// same share in Height, which is the grading's own (a step into a lake's
// hollow, or along its flat, can climb there). Held: the steps that climb
// in Elevation and not in Height are no more than countryClimbs of them.
// Laid off the history and not graded, 7 to 12 in a hundred did (and 10 to
// 11 in a hundred when the country was the earth's curve by rank, #67).
func TestHowOftenARiverClimbsTheCountry(t *testing.T) {
	if testing.Short() {
		t.Skip("makes globes")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-10s %10s %12s %12s %12s %12s %12s", "", "steps", "climb, all", "in Height", "rivers", "in Height", "country only")
	for _, c := range []struct {
		name  string
		seed  uint64
		terms Terms
	}{{"globe", 1, GlobeTerms()}, {"globe", 2, GlobeTerms()}, {"globe", 3, GlobeTerms()}, {"small", 1, smallGlobe()}, {"small", 2, smallGlobe()}} {
		g := yardWorld(c.name, c.seed, c.terms)
		var steps, climbs, heightClimbs, rivers, riverClimbs, riverHeightClimbs, country int
		for i := range g.Tiles {
			if g.Tiles[i].Wet() || g.sunk(i) {
				continue
			}
			q, ok := g.Downstream(g.PosOf(i))
			if !ok {
				continue
			}
			j := g.Index(q)
			up := g.Elevation(j) > g.Elevation(i)
			upHeight := g.Height[j] > g.Height[i]
			steps++
			if up {
				climbs++
			}
			if upHeight {
				heightClimbs++
			}
			if up && !upHeight {
				country++
			}
			if g.Flow[i] >= meanderFlow {
				rivers++
				if up {
					riverClimbs++
				}
				if upHeight {
					riverHeightClimbs++
				}
			}
		}
		share := func(k, n int) float64 { return float64(k) / math.Max(1, float64(n)) }
		fmt.Fprintf(&b, "\n%-10s %10d %12.4f %12.4f %12.4f %12.4f %12.4f", fmt.Sprintf("%s %d", c.name, c.seed), steps,
			share(climbs, steps), share(heightClimbs, steps), share(riverClimbs, rivers), share(riverHeightClimbs, rivers),
			share(country, steps))
		if share(country, steps) > countryClimbs {
			t.Errorf("%s %d: %d of %d steps climb the country where the ground falls", c.name, c.seed, country, steps)
		}
	}
	t.Logf("the share of the water's steps over dry land that climb in Elevation, and in Height:\n%s", b.String())
}

// countryClimbs is the share of the water's steps over dry land that may
// climb the country where the ground under them falls. The country is graded
// on the drainage as the tide's mud leaves it (see silt and stageCoast), and
// none does on the five worlds read; it was graded only after the cutting,
// and up to 0.0015 did on the small globes once the plates rifted into
// halves (#87).
const countryClimbs = 0.001

// earthShareUnder is the share of the earth's land under h metres.
func earthShareUnder(h float64) float64 {
	if math.IsInf(h, 1) {
		return 1
	}
	for k := 1; k < len(cogleyLand); k++ {
		lo, hi := cogleyLand[k-1], cogleyLand[k]
		if h <= hi.height {
			return lo.share + (hi.share-lo.share)*(h-lo.height)/(hi.height-lo.height)
		}
	}
	return 1
}
