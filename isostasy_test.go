package terra

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
)

// flatCrust is a world of continent continentCrust thick, wrapped,
// at the globe preset's deep span, with nothing on it: the ground a test lays
// loads on and takes them off again.
func flatCrust(w, h int) (*Grid, *crust) {
	g := NewGrid(w, h)
	g.Wrap = true
	g.deep = 37.5 * km
	g.strata = make([]column, len(g.Tiles))
	cr := newCrust(g)
	for i := range g.Tiles {
		cr.thick[i] = float32(continentCrust)
		g.Height[i] = cr.levelAt(i, 0)
	}
	g.base = seaDatum
	return g, cr
}

// A range worn down broadly comes back up by five sixths of what is taken off
// it, the crust's density over the mantle's (Molnar and England 1990); a
// valley cut a tile wide is held up by the plate either side of it and comes
// back by a fraction of that. Both are the plate answering a change of load,
// and what it answers is set by the load's breadth against the plate's
// flexural wavelength: some 2πα, half a thousand kilometres at
// elasticThickness. A tile is 37.5 km here, half of α, so a valley a tile
// wide still comes back by a fifth of what was cut: on the plate's scale it
// is not narrow.
func TestAWornRangeRisesByFiveSixthsOfWhatIsTakenOff(t *testing.T) {
	const cut = 1000.0
	for _, c := range []struct {
		name     string
		radius   float64 // tiles
		lo, hi   float64 // the rebound at the middle, per metre cut
		valleyed bool
	}{
		{"a range 1,500 km across", 20, 0.80, crustDensity/mantleDensity + 0.01, false},
		{"a valley a tile wide", 0, 0, 0.3, true},
	} {
		g, cr := flatCrust(256, 128)
		worn := make([]float64, len(g.Tiles))
		mid := 64*256 + 128
		for i := range g.Tiles {
			p := g.PosOf(i)
			in := math.Hypot(float64(p.X-128), float64(p.Y-64)) <= c.radius
			if c.valleyed {
				in = p.X == 128 && p.Y > 32 && p.Y < 96
			}
			if in {
				g.Height[i] -= cut
				cr.thicken(i, -cut)
				worn[i] = cut
			}
		}
		was := g.Height[mid]
		g.isostasy(cr, 0, worn, 0)
		per := (g.Height[mid] - was) / cut
		t.Logf("%s: the middle rose %.3f of each metre cut (ρc/ρm = %.3f); %.0f m cut over the history's land, %.0f m risen",
			c.name, per, crustDensity/mantleDensity, cr.eroded, cr.rebound)
		if per < c.lo || per > c.hi {
			t.Errorf("%s: the middle rose %.3f of each metre cut, outside [%.2f, %.3f]", c.name, per, c.lo, c.hi)
		}
	}
}

// A range raised on the crust sinks into a root, and the plate either side
// of it bends down into a moat: thickened over a belt thirteen tiles wide,
// half a thousand kilometres, the axis comes up by about airyRise of the
// thickening, and the ground just beyond the belt goes down. The axis is a
// little under Airy's, since an elastic plate under a strip overshoots its
// middle where the strip is a few α wide.
func TestARangeStandsOnARoot(t *testing.T) {
	g, cr := flatCrust(256, 128)
	const by = 10 * km
	for i := range g.Tiles {
		p := g.PosOf(i)
		if d := math.Abs(float64(p.X - 128)); d <= 6 {
			g.Height[i] += by
			cr.thicken(i, by)
		}
	}
	g.isostasy(cr, 0, nil, 0)
	at := func(x int) float64 { return g.Height[64*256+x] - levelOf(continentCrust, false, 0) }
	axis, beside := at(128), at(128+6+3)
	t.Logf("a belt 13 tiles wide thickened by %.0f km: its axis stands %.0f m up, %.0f m on Airy's alone; %.0f m beside it",
		by/km, axis, airyRise*by, beside)
	if math.Abs(axis-airyRise*by) > 0.25*airyRise*by {
		t.Errorf("the axis stands %.0f m up: want within a quarter of Airy's %.0f", axis, airyRise*by)
	}
	if beside >= 0 {
		t.Errorf("the ground three tiles off the belt stands %.0f m: want a moat", beside)
	}
}

// The flexure is the same whatever the goroutines: each row and each column
// of the transform is its own.
func TestThePlateBendsTheSameOverAnyGoroutines(t *testing.T) {
	was := Workers
	defer func() { Workers = was }()
	var got [][]float64
	for _, workers := range []int{1, 7} {
		Workers = workers
		g, cr := flatCrust(256, 128)
		load := make([]float64, len(g.Tiles))
		for i := range load {
			load[i] = math.Sin(float64(i)*0.37) * 100
		}
		got = append(got, slices.Clone(cr.flexure(g, load)))
	}
	if !slices.Equal(got[0], got[1]) {
		t.Fatal("the plate bent differently over one goroutine and seven")
	}
}

// cogleyLand is the earth's land by height, the share of the land above the
// sea lower than each height in metres: ETOPO5's land in 500 m bands, which
// is the curve Cogley (1984) draws for the continents, the lowest band split
// at 200 m. It is the same table G1 (#40) lays its country to, kept here
// under a name of its own until the two meet.
var cogleyLand = [...]struct{ share, height float64 }{
	{0, 0}, {0.28, 200}, {0.53315, 500}, {0.72805, 1000}, {0.83184, 1500},
	{0.88328, 2000}, {0.91815, 2500}, {0.94883, 3000}, {0.97095, 3500},
	{0.98211, 4000}, {0.98706, 4500}, {0.99299, 5000}, {0.99878, 5500},
	{0.99988, 6000}, {0.99999, 6500}, {1, 8000},
}

func cogleyAt(f float64) float64 {
	f = clamp01(f)
	for k := 1; k < len(cogleyLand); k++ {
		lo, hi := cogleyLand[k-1], cogleyLand[k]
		if f <= hi.share {
			return lo.height + (hi.height-lo.height)*(f-lo.share)/(hi.share-lo.share)
		}
	}
	return cogleyLand[len(cogleyLand)-1].height
}

// historyEnd is what a history leaves at the end of its last epoch, before
// the map takes its heights by rank: the ground, its crust, and the sea it ran
// against.
type historyEnd struct {
	w, h            int
	height, thick   []float64
	ocean           []bool
	sea             float64
	eroded, rebound float64
	denuded         []denudation
}

// endOfHistory makes the world of seed and terms afresh and keeps what its
// history left at the end of its last epoch.
func endOfHistory(seed uint64, terms Terms) historyEnd {
	var end historyEnd
	epochWatch = func(g *Grid, cr *crust, _ []Plate, e int) {
		if e != terms.Epochs-1 {
			return
		}
		end = historyEnd{w: g.W, h: g.H, height: g.heights(), sea: g.base, eroded: cr.eroded, rebound: cr.rebound, denuded: slices.Clone(cr.denuded)}
		end.thick = make([]float64, len(g.Tiles))
		for i := range end.thick {
			end.thick[i] = float64(cr.thick[i])
		}
		end.ocean = slices.Clone(cr.ocean)
	}
	defer func() { epochWatch = nil }()
	NewLand(seed, terms)
	return end
}

// weightedQuantile is the value share f of the weight lies under, of v sorted
// with its weights.
func weightedQuantile(v, wt []float64, f float64) float64 {
	total := 0.0
	for _, x := range wt {
		total += x
	}
	run := 0.0
	for k, x := range wt {
		run += x
		if run >= f*total {
			return v[k]
		}
	}
	return v[len(v)-1]
}

// The history's ground, as its crust floats it, against the earth's. Logged:
// the mean height of the land over the history's sea (the earth's is some
// 0.8 km, Cogley 1984), the continental freeboard - the mean of the
// continental crust's surface over the mean of the ocean floor's, four and a
// half to five kilometres on the earth - the land's hypsometry beside
// Cogley's at the shares G1 reads it at, the crust and the root under the
// highest of the land, how far the land rose in the epochs the weather
// wore it, per metre worn, and how fast the weather wore it, epoch by epoch,
// beside Portenga and Bierman's (2011) rates. Each tile is weighted by the
// ground it stands for on a sphere, as G1 weighs the map's.
//
// Read so at the end of main's history (2c51bea), with the sea a share of the
// ground and the plates settling to two fixed levels, the land stood 119, 238
// and 36 m over its sea on the mean, and the continents 0.66, 0.48 and 0.74
// km over the floor. Held to the acceptance of G2 (#41), loosely: the land's
// mean over 150 m, the freeboard between three and a half and six
// kilometres, the highest land on a root, and the rebound per metre worn near
// ρc/ρm, within a quarter under it or a tenth over. It came out over on G2,
// by the plate bending up the land beside what it lost as well as under it,
// and comes out under now, 0.71 to 0.77: the wear falls on narrow, steep
// ground, which the plate holds up, and not on whole drainages.
//
// And to G2b's (#78): the land's median in the hundreds of metres, between
// two hundred and a thousand; the land from its 25th to its 99th hundredth
// within a factor of two of Cogley's at the same share; and the continental
// crust within three kilometres of the 35 it starts at. With G2's stream
// power the land's median stood 26 to 37 metres over its sea, every drainage
// cut to its outlet each epoch, and the crust thinned to 27 to 29
// kilometres, since what the rivers took to the sea left it. See denude.go.
//
// The lowest twentieth and tenth of the land stand two to three times the
// earth's, 70 to 110 metres against 36 and 130 to 185 against 71: the earth's
// lowest land is coastal plain and delta, built by its rivers at the sea,
// and a history's rivers lay nothing on land but in its hollows. That is
// logged and not held.
func TestTheHistoryStandsOnItsCrust(t *testing.T) {
	if testing.Short() {
		t.Skip("makes globes")
	}
	shares := []float64{0.05, 0.1, 0.25, 0.5, 0.75, 0.9, 0.95, 0.99, 0.999, 1}
	var hyps, crust, wear strings.Builder
	fmt.Fprintf(&wear, "%-14s epoch: median, mean, lower half's and highest twentieth's, mm/yr", "")
	fmt.Fprintf(&hyps, "%-14s", "land, m")
	for _, f := range shares {
		fmt.Fprintf(&hyps, "%8g", f)
	}
	fmt.Fprintf(&hyps, "\n%-14s", "earth")
	for _, f := range shares {
		fmt.Fprintf(&hyps, "%8.0f", cogleyAt(f))
	}
	fmt.Fprintf(&crust, "%-14s %9s %9s %9s %9s %9s %9s %9s %9s %9s %9s %9s",
		"", "land", "mean land", "cont.", "floor", "freeboard", "cont.", "top 1%", "crust", "Moho", "root", "rebound")
	fmt.Fprintf(&crust, "\n%-14s %9s %9s %9s %9s %9s %9s %9s %9s %9s %9s %9s",
		"", "share", "m", "mean, m", "mean, m", "m", "crust, km", "m", "km", "km", "km", "per m")
	for _, c := range []struct {
		name  string
		seed  uint64
		terms Terms
	}{{"small globe 1", 1, smallGlobe()}, {"small globe 2", 2, smallGlobe()}, {"globe 1", 1, GlobeTerms()}} {
		e := endOfHistory(c.seed, c.terms)
		type at struct{ h, t, w float64 }
		var land []at
		var all, landW, contSum, contThick, contW, floorSum, floorW float64
		for i := range e.height {
			lat := 90 - 180*(float64(i/e.w)+0.5)/float64(e.h)
			wt := math.Cos(lat * math.Pi / 180)
			all += wt
			up := e.height[i] - e.sea
			if e.ocean[i] {
				floorSum, floorW = floorSum+wt*up, floorW+wt
			} else {
				contSum, contThick, contW = contSum+wt*up, contThick+wt*e.thick[i], contW+wt
			}
			if up > 0 {
				land = append(land, at{up, e.thick[i], wt})
				landW += wt
			}
		}
		if len(land) == 0 {
			t.Fatalf("%s: the history left no land", c.name)
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
		hs, ws := make([]float64, len(land)), make([]float64, len(land))
		mean := 0.0
		for k, l := range land {
			hs[k], ws[k] = l.h, l.w
			mean += l.h * l.w
		}
		mean /= landW
		fmt.Fprintf(&hyps, "\n%-14s", c.name)
		for _, f := range shares {
			fmt.Fprintf(&hyps, "%8.0f", weightedQuantile(hs, ws, f))
		}
		// The highest hundredth of the land: how high, on how much crust,
		// with its Moho how far under the sea, and its root how far under
		// the Moho of crust that floats at the sea.
		top := land[len(land)-max(1, len(land)/100):]
		var th, tt, tw float64
		for _, l := range top {
			th, tt, tw = th+l.h*l.w, tt+l.t*l.w, tw+l.w
		}
		th, tt = th/tw, tt/tw
		moho := tt - th
		root := moho - seaCrust
		cont, floor := contSum/contW, floorSum/floorW
		rebound := e.rebound / math.Max(1e-9, e.eroded)
		fmt.Fprintf(&crust, "\n%-14s %9.3f %9.0f %9.0f %9.0f %9.0f %9.1f %9.0f %9.1f %9.1f %9.1f %9.3f",
			c.name, landW/all, mean, cont, floor, cont-floor, contThick/contW/km, th, tt/km, moho/km, root/km, rebound)
		if mean < 150 || mean > 1600 {
			t.Errorf("%s: the land stands %.0f m over its sea on the mean, where the earth's stands some 800", c.name, mean)
		}
		if f := cont - floor; f < 3500 || f > 6000 {
			t.Errorf("%s: the continents stand %.0f m over the floor on the mean, where the earth's stand 4,500 to 5,000", c.name, f)
		}
		if root <= 0 {
			t.Errorf("%s: the highest land stands on no root: its Moho is %.1f km down", c.name, moho/km)
		}
		if per := crustDensity / mantleDensity; rebound < 0.75*per || rebound > 1.1*per {
			t.Errorf("%s: the land rose %.3f of each metre worn off it", c.name, rebound)
		}
		if med := weightedQuantile(hs, ws, 0.5); med < 200 || med > 1000 {
			t.Errorf("%s: half the land stands within %.0f m of its sea, where half the earth's stands within 461", c.name, med)
		}
		for _, f := range []float64{0.25, 0.5, 0.75, 0.9, 0.95, 0.99} {
			if got, want := weightedQuantile(hs, ws, f), cogleyAt(f); got < want/2 || got > 2*want {
				t.Errorf("%s: the land at %g of the way up its order stands %.0f m over its sea, where the earth's stands %.0f", c.name, f, got, want)
			}
		}
		if k := contThick / contW; math.Abs(k-continentCrust) > 3*km {
			t.Errorf("%s: the continental crust is %.1f km thick on the mean, where it starts at %.0f", c.name, k/km, continentCrust/km)
		}
		fmt.Fprintf(&wear, "\n%-14s", c.name)
		var shelved, spilt, lost float64
		for k, d := range e.denuded {
			shelved, spilt, lost = shelved+d.shelved, spilt+d.spilt, lost+d.lost
			if k%3 == 0 || k == len(e.denuded)-1 {
				fmt.Fprintf(&wear, "  %2d: %.3f %.3f %.3f %.2f", k, d.median, d.mean, d.low, d.high)
			}
		}
		fmt.Fprintf(&wear, "\n%-14s laid on the margins %.0f km over a tile, %.0f of it past a filled sea; %.0f off the map",
			"", shelved/km, spilt/km, lost/km)
	}
	t.Logf("the history's land over its sea, at shares of the land, against Cogley's (1984):\n%s", hyps.String())
	t.Logf("the history's crust:\n%s", crust.String())
	t.Logf("what the weather took off the land, against Portenga and Bierman's (2011) median of 0.054 mm/yr over the world's basins, 0.01 to 0.1 on the cratons and up to millimetres in active ranges:\n%s", wear.String())
}
