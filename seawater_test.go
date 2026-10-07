package terra

import (
	"cmp"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// The sea stands where the planet's water fills its basins (seawater.go).
// Read epoch by epoch on three small globes and the first globe: the sea's
// level over the first epoch's, the land share, how much of the continental
// crust is under the sea, the floor's mean age, and what the rise is made
// of: the floor's depth by its age (Pitman's ridge volume) and the margins'
// sediment.
//
// The plates slow through a history (slow), so its floor ages from first to
// last, and the sediment the rivers lay on the margins piles up from first to
// last: both run one way, and the level is the two against each other, with
// the continents' own growing and shrinking. So the level less what the
// sediment holds it up by is held to fall as the floor ages, the correlation
// between the two under nought on every globe.
//
// And the land share is what comes of it, not a rule: held at the last epoch
// within the band the made globes' land share is held to (0.2 to 0.4, about
// the earth's 0.29), and through the history within 0.15 to 0.5. The first
// epoch's is the old rule's (firstSea), which leaves 0.47 of small globe 3
// dry.
func TestTheSeaStandsWhereItsWaterFills(t *testing.T) {
	if testing.Short() {
		t.Skip("makes globes' histories")
	}
	for _, c := range []struct {
		name  string
		seed  uint64
		terms Terms
	}{{"small globe 1", 1, smallGlobe()}, {"small globe 2", 2, smallGlobe()}, {"small globe 3", 3, smallGlobe()}, {"globe 1", 1, GlobeTerms()}} {
		seas := historySeas(c.seed, c.terms, nil)
		var b strings.Builder
		fmt.Fprintf(&b, "%s\n%6s %9s %7s %7s %10s %8s %8s %8s %8s\n", c.name, "epoch", "rise, m", "land", "shelf", "floor, Myr", "ridge", "sed.", "cooling", "rest")
		var xs, ys []float64
		for e, r := range seas {
			cooled := r.heat - seas[0].heat
			fmt.Fprintf(&b, "%6d %9.0f %7.3f %7.3f %10.1f %8.0f %8.0f %8.0f %8.0f\n", e, r.rise, r.land, r.shelf, r.floorAge, r.ridge, r.sediment, cooled, r.rise-r.ridge-r.sediment-cooled)
			xs, ys = append(xs, r.floorAge), append(ys, r.rise-r.sediment)
		}
		corr := correlation(xs, ys)
		fmt.Fprintf(&b, "the level less the sediment's rise against the floor's age: r = %.2f", corr)
		t.Log(b.String())
		if corr >= 0 {
			t.Errorf("%s: the sea's level less its sediment rose with the floor's age (r = %.2f)", c.name, corr)
		}
		for e, r := range seas {
			if r.land < 0.15 || r.land > 0.5 {
				t.Errorf("%s: epoch %d's land share %.3f, outside 0.15 to 0.5", c.name, e, r.land)
			}
		}
		if last := seas[len(seas)-1].land; last < 0.2 || last > 0.4 {
			t.Errorf("%s: the last epoch's land share %.3f, outside 0.2 to 0.4", c.name, last)
		}
	}
}

// historySeas runs the history of seed and terms and returns what it wrote
// down of its sea, epoch by epoch. watch, where it is given, is called at the
// end of every epoch.
func historySeas(seed uint64, terms Terms, watch func(g *Grid, cr *crust, e int)) []seaReading {
	var seas []seaReading
	epochWatch = func(g *Grid, cr *crust, _ []Plate, e int) {
		if e < 0 {
			return
		}
		seas = cr.seas
		if watch != nil {
			watch(g, cr, e)
		}
	}
	defer func() { epochWatch = nil }()
	w := unmade(seed, terms)
	to := w.newGround(terms)
	from := w.historyGround(to, terms)
	w.history(from, terms.Epochs, terms.SeaShare, terms.Water)
	return seas
}

// A world spreading faster has younger floor, which stands higher, and the
// same water poured into its shallower basins stands higher and floods the
// continents: Pitman (1978), who read the Cretaceous seas so, and Müller and
// others (2008), who put 250 m of the sea's fall since on the ridges. Read on
// the ground three small globes' histories end on, with the floor made as
// young as it would be had the plates spread twice as fast - each tile of it
// raised by the depth half its age stands at less the depth its age does -
// and the planet's water poured again on the same ground before and after:
// the sea rises, and more of the continents are drowned. It rises by no more
// than waterLoad of the room the floor gave up over the sea's area, since
// the sea spreads over the land it drowns, and by no less than three fifths
// of it. Pitman's Cretaceous, with the floor some 30% younger than today's,
// stood 100 to 250 m higher; halved here, it stands 240 to 310 m higher, and
// half of the continents are under it.
func TestYoungFloorFloodsTheContinents(t *testing.T) {
	if testing.Short() {
		t.Skip("makes globes' histories")
	}
	for seed := uint64(1); seed <= 3; seed++ {
		terms := smallGlobe()
		historySeas(seed, terms, func(g *Grid, cr *crust, e int) {
			if e != terms.Epochs-1 {
				return
			}
			was, sea := g.Height, g.base
			defer func() { g.Height, g.base = was, sea }()
			pour := func() seaReading {
				still := g.seaOver(cr.oceanWater)
				g.base = cr.firstLevel + waterLoad*(still-cr.firstLevel)
				return g.readSea(cr, e, 0)
			}
			before := pour()
			young := slices.Clone(was)
			gave, wet := 0.0, 0.0
			for i := range young {
				if young[i] <= before.level {
					wet += g.areaOf(i)
				}
				if !cr.ocean[i] {
					continue
				}
				age := cr.ageAt(i, e) / myr
				up := floorDepth(age) - floorDepth(age/2)
				young[i] += up
				gave += up * g.areaOf(i)
			}
			g.Height = young
			after := pour()
			rise, want := after.level-before.level, waterLoad*gave/wet
			t.Logf("small globe %d: the floor's mean age %.1f Myr made %.1f; the sea rose %.0f m (%.0f over the sea's area), the land share %.3f to %.3f, the continents drowned %.3f to %.3f",
				seed, before.floorAge, before.floorAge/2, rise, want, before.land, after.land, before.shelf, after.shelf)
			if rise < 0.6*want || rise > want {
				t.Errorf("small globe %d: the sea rose %.0f m on younger floor, against %.0f", seed, rise, want)
			}
			if after.shelf <= before.shelf {
				t.Errorf("small globe %d: younger floor drowned %.3f of the continents, against %.3f", seed, after.shelf, before.shelf)
			}
		})
	}
}

// correlation is Pearson's r between xs and ys.
func correlation(xs, ys []float64) float64 {
	n := float64(len(xs))
	var mx, my float64
	for k := range xs {
		mx += xs[k] / n
		my += ys[k] / n
	}
	var sxy, sxx, syy float64
	for k := range xs {
		sxy += (xs[k] - mx) * (ys[k] - my)
		sxx += (xs[k] - mx) * (xs[k] - mx)
		syy += (ys[k] - my) * (ys[k] - my)
	}
	return sxy / math.Sqrt(sxx*syy)
}

// The sea poured into any ground stands where the room under it holds the
// water: read against the heights sorted and walked up, which is exact, on
// rough ground with flats and steps in it, on a globe and on a map that is
// not one, for a little water and for a great deal.
func TestTheSeaIsPouredToItsLevel(t *testing.T) {
	rng := rand.New(rand.NewPCG(4, 2))
	for _, wrap := range []bool{false, true} {
		g := NewGrid(96, 48)
		g.Wrap = wrap
		for i := range g.Height {
			g.Height[i] = math.Round(rng.NormFloat64()*2000) + 4000*math.Floor(rng.Float64()*2)
		}
		total := 0.0
		for _, a := range g.rowWeights() {
			total += a * float64(g.W)
		}
		for _, depth := range []float64{0.5, 30, 1000, 4000, 20000} {
			water := depth * total
			got, want := g.seaOver(water), walkedSea(g, water)
			if math.Abs(got-want) > 1e-6 {
				t.Errorf("wrap %v, %g m of water: the sea stands at %.9f, and walked up at %.9f", wrap, depth, got, want)
			}
		}
	}
}

// walkedSea is the level seaOver finds, by the heights sorted and walked up.
func walkedSea(g *Grid, water float64) float64 {
	type tile struct{ h, a float64 }
	ts := make([]tile, len(g.Tiles))
	for i := range ts {
		ts[i] = tile{g.Height[i], g.areaOf(i)}
	}
	slices.SortFunc(ts, func(a, b tile) int { return cmp.Compare(a.h, b.h) })
	under, below := 0.0, 0.0
	for k, x := range ts {
		under += x.a
		below += x.a * x.h
		if k+1 == len(ts) || under*ts[k+1].h-below >= water {
			return (water + below) / under
		}
	}
	return math.NaN()
}
