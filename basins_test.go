package terra

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
)

// A column stretched in one step and left to cool sinks as McKenzie's (1978)
// does: the crust's own sinking less the heat still under it (subside.go),
// against his initial subsidence and its thermal tail, for continents
// stretched by a quarter to four times. Within a third of a kilometre of
// McKenzie's at every age and every β: the difference is the mantle's own
// thinning, which the crust's reading does not keep (see the top of
// subside.go).
func TestARiftSinksAsMcKenzieHas(t *testing.T) {
	var b strings.Builder
	fmt.Fprintf(&b, "water-loaded subsidence, km: here against McKenzie (1978)\n%6s", "Myr")
	betas := []float64{1.25, 1.5, 2, 3, 4}
	for _, beta := range betas {
		fmt.Fprintf(&b, " %15s", fmt.Sprintf("β %.2f", beta))
	}
	b.WriteString("\n")
	for k := 0; k <= 40; k++ {
		years := float64(k) * epochYears
		if k%5 != 0 && k > 2 {
			continue
		}
		fmt.Fprintf(&b, "%6.0f", years/myr)
		for _, beta := range betas {
			cr := &crust{thick: []float32{continentCrust}, rift: []rifted{{}}}
			cr.thick[0] = float32(continentCrust / beta)
			cr.stretch(0, continentCrust, 0)
			for range k {
				cr.cool(1)
			}
			here := wetRise*continentCrust*(1-1/beta) - float64(cr.rift[0].warm)
			want := mcKenzie(continentCrust, beta, years)
			fmt.Fprintf(&b, " %7.2f / %5.2f", here/km, want/km)
			if math.Abs(here-want) > 0.35*km {
				t.Errorf("β %.2f, %.0f Myr after: sank %.2f km, McKenzie %.2f", beta, years/myr, here/km, want/km)
			}
		}
		b.WriteString("\n")
	}
	t.Log(b.String())
	// And a stretching in steps is a stretching: twice by √2 is once by 2.
	cr := &crust{thick: []float32{continentCrust}, rift: []rifted{{}}}
	cr.thick[0] = float32(continentCrust / math.Sqrt2)
	cr.stretch(0, continentCrust, 0)
	was := float64(cr.thick[0])
	cr.thick[0] = float32(continentCrust / 2)
	cr.stretch(0, was, 0)
	if got := cr.rift[0].beta(); math.Abs(got-2) > 1e-5 {
		t.Errorf("stretched twice by √2, β is %.6f", got)
	}
	if got, want := float64(cr.rift[0].warm), heatOf(2); math.Abs(got-want) > 1e-3 {
		t.Errorf("stretched twice by √2, the heat under it is %.1f m against %.1f", got, want)
	}
}

// basinReading is what a history's basins did, read epoch by epoch. See
// readBasins.
type basinReading struct {
	// rifts, by epochs since a tile was last stretched: how many tiles, and
	// the means of β, of the crust's water-loaded sinking and McKenzie's for
	// the same β and age, and of the fill on it.
	rifts [16]struct{ n, beta, sank, mcKenzie, fill float64 }
	// axis is the mean fill on a rift's tiles by how far they were stretched.
	axis [4]struct{ n, fill float64 }
	// fore is the ground about the collisions, by side (0 the plate that
	// went under, 1 the plate that stayed up) and by foreBin from the
	// suture: how many tile-epochs, how many of them in the belt the
	// collision raises, how far under its column's level the ground stood
	// (the plate's bend, m), and the fill the epoch laid (m).
	fore [2][24]foreReading
	// margin is the sediment on the passive margins at the end, in metres:
	// the tiles under the sea within marginWidth of a continent's land and
	// away from any seam. held is the mean over them of how far the plate
	// holds the ground over its column's level, and fill the mean sediment.
	margin     []float64
	held, fill float64
	// shed, kept and shelved are what the land lost to the weather, what of
	// that the land kept in its own basins, and what was laid on the sea's
	// floor, in metres an epoch over the land as it stood; land is the land's
	// share of the planet at the end.
	shed, kept, shelved, land float64
	span                      float64
}

// foreReading is a band of ground about the collisions: see basinReading.
type foreReading struct{ n, belt, moat, laid float64 }

// marginWidth is how far from a continent's land the passive margins are
// read, in metres: the width of the earth's wedges of sediment off their
// coasts, a couple of hundred kilometres (Allen and Allen ch. 3).
const marginWidth = 200 * km

// foreBin is how wide the bands the ground about a collision is read in are.
const foreBin = 50 * km

// readBasins runs the history of seed and terms and reads its basins.
func readBasins(seed uint64, terms Terms) basinReading {
	var r basinReading
	var landArea float64
	historySeas(seed, terms, func(g *Grid, cr *crust, e int) {
		r.span = g.span()
		for i := range g.Tiles {
			a := g.areaOf(i)
			worn := cr.wear[i]
			if cr.was[i] > g.base {
				landArea += a
				r.shed += a * math.Max(0, worn)
				if g.Height[i] > g.base {
					r.kept += a * math.Max(0, -worn)
				}
			}
			if g.Height[i] <= g.base && cr.was[i] <= g.base {
				r.shelved += a * math.Max(0, -worn)
			}
			if f := cr.fore[i]; f != 0 && !cr.ocean[i] {
				side := 0
				if f < 0 {
					side = 1
				}
				d := int((math.Abs(float64(f)) - 1) * g.span() / foreBin)
				if d < len(r.fore[side]) {
					x := &r.fore[side][d]
					x.n++
					if g.seam[i].found {
						x.belt++
					}
					x.moat += cr.levelAt(i, e) - g.Height[i]
					x.laid += math.Max(0, -worn)
				}
			}
		}
		if e != terms.Epochs-1 {
			return
		}
		var all, dry float64
		for i := range g.Tiles {
			all += g.areaOf(i)
			if g.Height[i] > g.base {
				dry += g.areaOf(i)
			}
			if cr.ocean[i] {
				continue
			}
			rf := cr.rift[i]
			if rf.at == 0 || rf.beta() < 1.1 {
				continue
			}
			beta := rf.beta()
			fill := float64(cr.sed[i])
			before := (float64(cr.thick[i]) - fill) * beta
			if before < 25*km || before > 45*km {
				continue // read where it was a continent's crust when stretched
			}
			k := e - (int(rf.at) - 1)
			x := &r.rifts[k]
			x.n++
			x.beta += beta
			x.sank += wetRise*before*(1-1/beta) - float64(rf.warm)
			x.mcKenzie += mcKenzie(before, beta, float64(k)*epochYears)
			x.fill += fill
			band := 0
			switch {
			case beta >= 2:
				band = 3
			case beta >= 1.5:
				band = 2
			case beta >= 1.25:
				band = 1
			}
			r.axis[band].n++
			r.axis[band].fill += fill
		}
		r.land = dry / all
		away := g.awayFrom(func(i int) bool { return !cr.ocean[i] && g.Height[i] > g.base })
		for i := range g.Tiles {
			if g.Height[i] > g.base || away[i]*g.span() > marginWidth || g.seam[i].found || cr.fore[i] != 0 {
				continue
			}
			r.margin = append(r.margin, float64(cr.sed[i]))
			r.held += g.Height[i] - cr.levelAt(i, e)
			r.fill += float64(cr.sed[i])
		}
		if n := float64(len(r.margin)); n > 0 {
			r.held /= n
			r.fill /= n
		}
	})
	// An epoch's, over the land as it stood each epoch.
	r.shed /= landArea
	r.kept /= landArea
	r.shelved /= landArea
	return r
}

// The basins of a history (subside.go), read on three small globes and the
// first globe.
//
// The rifts: the tiles a rift stretched, by how long ago it last stretched
// them, with the crust's water-loaded sinking beside McKenzie's for the same
// β and age; and the fill on them by how far they were stretched, which
// thickens toward a rift's axis, where β is greatest.
//
// The forelands: the ground about a collision, on the plate that went under
// it and on the plate that stayed up, by how far it is from the suture: how
// far under its own column's level the plate's bend holds it, and how much
// the epoch laid on it. In front of the range, from two flexural parameters
// out, the plate that went under is bent further under its columns and
// filled faster than the plate that stayed up: the foredeep is away from the
// overriding plate.
//
// The passive margins: the sediment on the sea floor within marginWidth of a
// continent, away from the seams. The earth's old margins carry up to ten to
// fifteen kilometres of it (the Gulf of Mexico, the Niger delta, the Bay of
// Bengal: Allen and Allen ch. 3; Straume and others 2019).
//
// And the sediment's budget: what the land shed, what it kept in its own
// basins, and what reached the sea's floor, in millimetres a year over the
// land.
func TestBasinsSubsideAndFill(t *testing.T) {
	if testing.Short() {
		t.Skip("makes globes' histories")
	}
	for _, c := range []struct {
		name  string
		seed  uint64
		terms Terms
	}{{"small globe 1", 1, smallGlobe()}, {"small globe 2", 2, smallGlobe()}, {"small globe 3", 3, smallGlobe()}, {"globe 1", 1, GlobeTerms()}} {
		r := readBasins(c.seed, c.terms)
		var b strings.Builder
		fmt.Fprintf(&b, "%s (a tile %.1f km)\n", c.name, r.span/km)
		fmt.Fprintf(&b, "rifts, by how long since last stretched: water-loaded sinking of the crust against McKenzie's, and the fill on it\n")
		fmt.Fprintf(&b, "%6s %6s %6s %10s %10s %8s\n", "Myr", "tiles", "β", "sank, km", "McKenzie", "fill, km")
		for k, x := range r.rifts {
			if x.n == 0 {
				continue
			}
			fmt.Fprintf(&b, "%6d %6.0f %6.2f %10.2f %10.2f %8.2f\n", k*int(epochYears/myr), x.n, x.beta/x.n, x.sank/x.n/km, x.mcKenzie/x.n/km, x.fill/x.n/km)
		}
		fmt.Fprintf(&b, "a rift's fill by how far it was stretched, km (tiles):")
		for k, band := range []string{"β 1.1-1.25", "1.25-1.5", "1.5-2", "2+"} {
			if x := r.axis[k]; x.n > 0 {
				fmt.Fprintf(&b, "  %s %.2f (%.0f)", band, x.fill/x.n/km, x.n)
			}
		}
		b.WriteString("\n")
		if lo, hi := r.axis[0], r.axis[3]; lo.n > 0 && hi.n > 0 && hi.fill/hi.n <= lo.fill/lo.n {
			t.Errorf("%s: a rift's fill is %.2f km where it was stretched most and %.2f where least", c.name, hi.fill/hi.n/km, lo.fill/lo.n/km)
		}
		fmt.Fprintf(&b, "about the collisions, by km from the suture: the ground under its column's level (the plate's bend), m; the fill an epoch, m; the share in the belt; tile-epochs\n")
		fmt.Fprintf(&b, "%9s %30s %30s\n", "km", "plate going under", "plate staying up")
		row := func(x foreReading) string {
			n := math.Max(1, x.n)
			return fmt.Sprintf("%8.0f %6.0f %6.2f %7.0f", x.moat/n, x.laid/n, x.belt/n, x.n)
		}
		// In front of the range: from two flexural parameters out, past the
		// high ground at the suture, to the edge of the reach.
		var under, up foreReading
		for d := range r.fore[0] {
			lo, hi := r.fore[0][d], r.fore[1][d]
			if lo.n == 0 && hi.n == 0 {
				continue
			}
			fmt.Fprintf(&b, "%4.0f-%-4.0f %30s %30s\n", float64(d)*foreBin/km, float64(d+1)*foreBin/km, row(lo), row(hi))
			if float64(d)*foreBin < 2*flexuralParameter {
				continue
			}
			under.n, under.moat, under.laid = under.n+lo.n, under.moat+lo.moat, under.laid+lo.laid
			up.n, up.moat, up.laid = up.n+hi.n, up.moat+hi.moat, up.laid+hi.laid
		}
		if under.n > 0 && up.n > 0 {
			fmt.Fprintf(&b, "in front of the range: the plate going under bent %.0f m under its columns, and filled %.0f m an epoch; the plate staying up %.0f m, and %.0f m\n",
				under.moat/under.n, under.laid/under.n, up.moat/up.n, up.laid/up.n)
			if under.moat/under.n <= up.moat/up.n || under.laid/under.n <= up.laid/up.n {
				t.Errorf("%s: the foredeep is on the plate staying up: bent %.0f m and filled %.0f an epoch, against %.0f and %.0f on the plate going under",
					c.name, up.moat/up.n, up.laid/up.n, under.moat/under.n, under.laid/under.n)
			}
		}
		slices.Sort(r.margin)
		if n := len(r.margin); n > 0 {
			q := func(f float64) float64 { return r.margin[min(n-1, int(f*float64(n)))] / km }
			fmt.Fprintf(&b, "sediment on the passive margins, km (%d tiles): median %.2f, 90%% %.2f, 99%% %.2f, most %.2f; the plate holds them %.0f m over their columns' level, under a mean %.2f km of it\n",
				n, q(0.5), q(0.9), q(0.99), r.margin[n-1]/km, r.held, r.fill/km)
		}
		years := epochYears // the budget is an epoch's, over the land as it stood
		fmt.Fprintf(&b, "the land shed %.3f mm/yr, kept %.3f of it in its own basins, and laid %.3f on the sea's floor; land %.3f of the planet at the end\n",
			r.shed/years/mm, r.kept/years/mm, r.shelved/years/mm, r.land)
		t.Log(b.String())
	}
}
