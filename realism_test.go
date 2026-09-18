package terra

import (
	"fmt"
	"math"
	"slices"
	"sync"
	"testing"
)

// The world held against the planet it is meant to be a piece of.
//
// The yardsticks in yardstick_test.go read the ground at the scale of a
// hillside and the water at the scale of a catchment, and some of what they
// read the making hands them whatever happens: a made map's heights are the
// drawn map's heights put back by rank - see normalise and basins - so its
// hypsometric integral is the drawn map's, and the valleys shape.go bends into
// the ground come out at the spacing they were drawn at. The figures here are
// ones that nothing is matched to. Each is read off what the processes left:
// how the sea floor's depth goes with the age of its crust, how a river's
// slope falls with the ground it drains, how the plates are sized, where the
// rain falls and the woods stand, and whether any of that holds when the same
// world is made at twice the resolution.
//
// Where the map does not yet answer to one, the yardstick stays and says so:
// it is skipped as a known gap with the reading it got, so that the gap is in
// front of whoever runs the tests rather than in a note somewhere. A gap that
// closes fails, so that the marker comes off and the yardstick starts holding.

// realYardstick is a yardstick that may be a known gap: gap says what is
// missing, and the letter of the workstream expected to close it.
type realYardstick struct {
	yardstick
	gap string
}

var realYardsticks = []realYardstick{
	// 1. Hypsometry. The earth's heights fall in two heaps, the continents
	// just above the sea and the abyssal plains four and a half kilometres
	// under it, with little between: the one fact about a planet's relief that
	// says it has two kinds of crust floating at two levels.
	{yardstick: yardstick{
		name: "continental hypsometric mode, globe", unit: "km", scale: "ground", lo: -0.3, hi: 0.5, slow: true,
		source:  "Wegener 1915; Amante & Eakins 2009 (ETOPO1): the earth's elevations peak near +0.1 km",
		measure: func() float64 { hi, _ := hypsometricModes(globes()); return hi },
	}},
	{yardstick: yardstick{
		name: "oceanic hypsometric mode, globe", unit: "km", scale: "ground", lo: -5.0, hi: -3.8, slow: true,
		source:  "Wegener 1915; Amante & Eakins 2009 (ETOPO1): the second peak of the earth's elevations near -4.4 km",
		measure: func() float64 { _, lo := hypsometricModes(globes()); return lo },
	},
		gap: "known gap: T5 - the floor has the earth's ages (firstFloorAges), its sediment (floorSediment), some 800 m, and GDH1's depths, but no plateaus, seamounts or hotspot swells: old floor, most of the ocean, flattens toward 5.65 km under its sediment, and a sixth of the deep floor lies at 5.25-5.5: -5.375 km",
	},

	// 2. The sea floor sinks as it cools. New floor at a ridge stands two and a
	// half kilometres under the sea and sinks as the root of its age for the
	// first seventy million years, then flattens as the plate reaches the
	// thickness it can hold.
	{yardstick: yardstick{
		name: "ridge crest depth, globe", unit: "km", scale: "ground", lo: 2.0, hi: 3.0, slow: true,
		source:  "Parsons & Sclater 1977: d = 2500 + 350 sqrt(t) m; Stein & Stein 1992 (GDH1): 2600 m at the ridge",
		measure: func() float64 { return seafloorSubsidence(globes()).ridge / 1000 },
	}},
	{yardstick: yardstick{
		name: "sea floor subsidence to 70 Myr, globe", unit: "m/sqrt(Myr)", scale: "ground", lo: 250, hi: 450, slow: true,
		source:  "Parsons & Sclater 1977: d = 2500 + 350 sqrt(t) m to ~70 Myr; Stein & Stein 1992: 365 sqrt(t)",
		measure: func() float64 { return seafloorSubsidence(globes()).young },
	}},
	{yardstick: yardstick{
		name: "sea floor flattening past 70 Myr, globe", unit: "old/young", scale: "ground", lo: -0.2, hi: 0.6, slow: true,
		source: "Parsons & Sclater 1977; Stein & Stein 1992 (GDH1): depth goes as sqrt(age) to ~70 Myr and flattens after",
		measure: func() float64 {
			// Flattening is only a reading of floor that sinks in the first place.
			s := seafloorSubsidence(globes())
			if !(s.young >= 250) {
				return math.NaN()
			}
			return s.old / s.young
		},
	}},

	// 3. Plate sizes. Past the handful of great plates the earth's plates
	// follow a power law in area.
	{yardstick: yardstick{
		name: "plate area cumulative exponent, plate world", unit: "", scale: "ground", lo: 0.15, hi: 0.35, slow: true,
		source:  "Bird 2003 Fig. 19: N(>=A) ~ A^-0.25 for plates of 0.002-1 sr; Sornette & Pisarenko 2003",
		measure: func() float64 { return plateAreaExponent(plateWorlds(3)) },
	}},

	// 4. River profiles. A river worn by its water falls less steeply the more
	// ground it drains, as a power of that ground, and a profile that has
	// come to terms with its uplift is a straight line in chi.
	{yardstick: yardstick{
		name: "channel concavity, valley", unit: "", scale: "water", lo: 0.35, hi: 0.60,
		source:  "Flint 1974; Tucker & Whipple 2002; Whipple 2004: S ~ A^-theta, theta 0.35-0.6 in bedrock and mixed channels",
		measure: func() float64 { th, _ := flint(valleys(5)); return th },
	},
		gap: "known gap: B - valleys cut two thousand years by stream power with no lift fall less concave than they were shaped: theta 0.30, and 0.34 uncut (see valleyYears)",
	},
	{yardstick: yardstick{
		name: "channel concavity, small globe", unit: "", scale: "water", lo: 0.35, hi: 0.60, slow: true,
		source:  "Flint 1974; Tucker & Whipple 2002; Whipple 2004: S ~ A^-theta, theta 0.35-0.6 in bedrock and mixed channels",
		measure: func() float64 { th, _ := flint(smallGlobes(networkGlobes)); return th },
		// The gap this carried - profiles less concave than stream power
		// carves them - closed when the crust was broken into fractures before
		// its plates were grown (see fractureWall): 0.29 with the warm-sea
		// limestone, 0.367 with the deep floor laid, 0.3528 now. It is barely
		// inside and the reading swings with the coast, so a change that puts
		// it back under 0.35 has not broken anything new - it has reopened a
		// gap that was open for most of this map's life.
	}},
	{yardstick: yardstick{
		name: "Flint's law fit R2, valley", unit: "", scale: "water", lo: 0.85, hi: 1,
		source:  "Flint 1974; Wobus et al. 2006: binned log S against log A is a straight line in steady channels",
		measure: func() float64 { _, r2 := flint(valleys(5)); return r2 },
	}},
	{yardstick: yardstick{
		name: "Flint's law fit R2, small globe", unit: "", scale: "water", lo: 0.85, hi: 1, slow: true,
		source:  "Flint 1974; Wobus et al. 2006: binned log S against log A is a straight line in steady channels",
		measure: func() float64 { _, r2 := flint(smallGlobes(networkGlobes)); return r2 },
	}},
	{yardstick: yardstick{
		name: "chi-plot linearity R2, valley", unit: "", scale: "water", lo: 0.90, hi: 1,
		source:  "Perron & Royden 2013: a steady trunk profile is linear in chi (theta_ref 0.45)",
		measure: func() float64 { return chiLinearity(valleys(5)) },
	}},
	{yardstick: yardstick{
		name: "chi-plot linearity R2, small globe", unit: "", scale: "water", lo: 0.90, hi: 1, slow: true,
		source:  "Perron & Royden 2013: a steady trunk profile is linear in chi (theta_ref 0.45)",
		measure: func() float64 { return chiLinearity(smallGlobes(networkGlobes)) },
	}},

	// 5. Meanders. A river wandering on its flood plain bends at ten to fourteen
	// of its own widths and runs a good deal longer than its valley.
	{yardstick: yardstick{
		name: "meander wavelength, small globe", unit: "widths", scale: "ground", lo: 10, hi: 14, slow: true,
		source:  "Leopold & Wolman 1960: meander wavelength 10-14 channel widths",
		measure: func() float64 { return meanders(smallGlobes(networkGlobes)).wavelength },
	}},
	{yardstick: yardstick{
		name: "sinuosity of low-gradient reaches, small globe", unit: "", scale: "ground", lo: 1.2, hi: 3, slow: true,
		source:  "Leopold & Wolman 1957, 1960: meandering reaches 1.5 and over, braided and straight below; 1.2-3 on flood plains",
		measure: func() float64 { return meanders(smallGlobes(networkGlobes)).sinuosity },
	}},
	{yardstick: yardstick{
		name: "meander migration", unit: "widths/yr", scale: "ground", lo: 0.001, hi: 0.18,
		source:  "Hickin & Nanson 1984; Braudrick et al. 2009: <0.01 to 0.18 widths/yr on flood plains; floor lowered for rivers confined in incised valleys, not a measured figure",
		measure: meanderMigration,
	},
		gap: "known gap: G - since the sun and the ranges' rain, valleys 1 and 3 have lost their shifts of three to six tiles, their one-tile migration as it was, and valleys 4 and 5 hardly move at all: 0.0005 over five valleys",
	},

	// 6. The climate by latitude.
	{yardstick: yardstick{
		name: "annual mean temperature, equator, globe", unit: "C", scale: "water", lo: 24, hi: 28, slow: true,
		source:  "Legates & Willmott 1990; Peixoto & Oort 1992 Fig. 7.4: zonal mean surface air ~26 C at 0-5 deg",
		measure: func() float64 { return zonalTemp(globes(), 0, 5) },
	}},
	{yardstick: yardstick{
		name: "annual mean temperature, 60 deg, globe", unit: "C", scale: "water", lo: -4, hi: 4, slow: true,
		source:  "Legates & Willmott 1990; Peixoto & Oort 1992 Fig. 7.4: zonal mean surface air ~0 C at 60 deg",
		measure: func() float64 { return zonalTemp(globes(), 55, 65) },
	}},
	{yardstick: yardstick{
		name: "annual mean temperature, poles, globe", unit: "C", scale: "water", lo: -60, hi: -20, slow: true,
		source:  "Legates & Willmott 1990; Peixoto & Oort 1992: -18 C over the Arctic, -50 C over Antarctica; both poles under -20",
		measure: func() float64 { return zonalTemp(globes(), 80, 90) },
	},
		gap: "known gap: G - the energy balance's poles are -11 C: one column a band has no polar inversion and no ice sheet standing kilometres high",
	},
	{yardstick: yardstick{
		name: "equatorial over subtropical rain, globe", unit: "x", scale: "water", lo: 1.8, hi: 4, slow: true,
		source:  "Adler et al. 2003 (GPCP): zonal rain ~5.5 mm/d under the ITCZ against ~2.2 mm/d at 20-30 deg",
		measure: func() float64 { return zonalRain(globes(), 0, 10) / zonalRain(globes(), 20, 30) },
	},
		gap: "known gap: G - the column budget gathers the trades' water into a narrow ITCZ under a mean wind with no transient convection spreading it: 5x the subtropics, not 2-3x",
	},
	{yardstick: yardstick{
		name: "midlatitude over subtropical rain, globe", unit: "x", scale: "water", lo: 1.1, hi: 2, slow: true,
		source:  "Adler et al. 2003 (GPCP): the storm tracks at 40-60 deg rain ~2.8 mm/d against ~2.2 mm/d at 20-30 deg",
		measure: func() float64 { return zonalRain(globes(), 40, 60) / zonalRain(globes(), 20, 30) },
	}},
	{yardstick: yardstick{
		name: "latitude of the driest belt, globe", unit: "deg", scale: "water", lo: 15, hi: 35, slow: true,
		source:  "Adler et al. 2003 (GPCP); Peixoto & Oort 1992: the subtropical minimum of zonal rain lies at 20-30 deg",
		measure: func() float64 { return driestBelt(globes()) },
	}},

	// 7. Woods against the dryness of the air.
	{yardstick: yardstick{
		name: "forest share of arid land (PET/P > 5), globe", unit: "", scale: "water", lo: 0, hi: 0.05, slow: true,
		source:  "Middleton & Thomas 1997 (UNEP aridity index AI < 0.2); Hansen et al. 2013: closed forest is all but absent in arid land",
		measure: func() float64 { return forestByAridity(globes(), 5, math.Inf(1)) },
	},
	},
	{yardstick: yardstick{
		name: "forest share, dry (PET/P 1.5-5) over humid (PET/P < 1), globe", unit: "x", scale: "water", lo: 0, hi: 0.5, slow: true,
		source: "Middleton & Thomas 1997 aridity classes; Bastin et al. 2017: forest cover falls toward nothing as PET/P passes 1",
		measure: func() float64 {
			return forestByAridity(globes(), 1.5, 5) / forestByAridity(globes(), 0, 1)
		},
	},
	},

	// 8. The same world at twice the resolution. None of these are a property
	// of the grid, so none should move much when the grid does.
	{yardstick: yardstick{
		name: "Hack exponent, 2x less 1x, small globe", unit: "", scale: "water", lo: -0.05, hi: 0.05, slow: true,
		source:  "Hack 1957; Rigon et al. 1996: h is a property of the network, not of the survey's resolution",
		measure: func() float64 { return hackExponent(doubleGlobes()) - hackExponent(singleGlobes()) },
	}},
	{yardstick: yardstick{
		name: "channel concavity, 2x less 1x, small globe", unit: "", scale: "water", lo: -0.1, hi: 0.1, slow: true,
		source: "Wobus et al. 2006; Perron & Royden 2013: theta is a property of the channels, not of the DEM",
		measure: func() float64 {
			a, _ := flint(doubleGlobes())
			b, _ := flint(singleGlobes())
			return a - b
		},
	},
	// It was a known gap (B: 0.20 with the softened winters, the deep floor
	// and the warm-sea limestone merged) until the plates were carried the
	// part of a tile a whole step leaves over; it read 0.077 then.
	},
	{yardstick: yardstick{
		name: "hypsometric integral, 2x less 1x, small globe", unit: "", scale: "ground", lo: -0.05, hi: 0.05, slow: true,
		source:  "Strahler 1952: the integral is dimensionless and read the same off any faithful map of the ground",
		measure: func() float64 { return meanHypsometry(doubleGlobes()) - meanHypsometry(singleGlobes()) },
	}},
	{yardstick: yardstick{
		name: "mean land rain, 2x over 1x, small globe", unit: "x", scale: "water", lo: 0.85, hi: 1.15, slow: true,
		source:  "Adler et al. 2003 (GPCP): a planet's rain is the planet's, however finely it is gridded",
		measure: func() float64 { return landRain(doubleGlobes()) / landRain(singleGlobes()) },
	}},
}

// TestTheRealWorld holds the map to the planet's yardsticks.
func TestTheRealWorld(t *testing.T) {
	for _, y := range realYardsticks {
		t.Run(y.name, func(t *testing.T) {
			if y.slow && testing.Short() {
				t.Skip("needs a full globe")
			}
			got := y.measure()
			in := got >= y.lo && got <= y.hi
			switch {
			case y.gap != "" && in:
				t.Errorf("got %.4g %s, inside %.4g-%.4g: the gap has closed, take the marker off (%s)",
					got, y.unit, y.lo, y.hi, y.gap)
			case y.gap != "":
				t.Skipf("%s (got %.4g %s, real %.4g-%.4g)", y.gap, got, y.unit, y.lo, y.hi)
			case !in:
				t.Errorf("got %.4g %s, real %.4g-%.4g (%s)", got, y.unit, y.lo, y.hi, y.source)
			}
		})
	}
}

// memo keeps a reading that several yardsticks share, made once.
var memo sync.Map

func remember[T any](key string, f func() T) T {
	type once struct {
		sync.Once
		v any
	}
	o, _ := memo.LoadOrStore(key, &once{})
	c := o.(*once)
	c.Do(func() { c.v = f() })
	return c.v.(T)
}

// plateWorlds are the worlds plate_test.go reads, kept.
func plateWorlds(n int) []*Grid {
	var gs []*Grid
	for seed := uint64(1); seed <= uint64(n); seed++ {
		gs = append(gs, yardWorld("small", seed, smallGlobe())) // plateWorld's terms
	}
	return gs
}

// resolutionSeeds is how many small globes the resolution yardsticks compare.
// A network reading off one globe scatters by near a tenth - see
// networkGlobes - and the tolerances are tighter than that.
const resolutionSeeds = 4

func singleGlobes() []*Grid { return smallGlobes(resolutionSeeds) }

func doubleGlobes() []*Grid {
	var gs []*Grid
	cfg := smallGlobe()
	cfg.Width, cfg.Height = 2*cfg.Width, 2*cfg.Height
	for seed := uint64(1); seed <= resolutionSeeds; seed++ {
		gs = append(gs, yardWorld("double", seed, cfg))
	}
	return gs
}

// latitudeOf is the latitude of row y on a globe, as Climate reads it.
func latitudeOf(g *Grid, y int) float64 { return 90 - 180*(float64(y)+0.5)/float64(g.H) }

// hypsometricModes is where the heights of a world heap up above and below two
// kilometres under its sea, in kilometres from the sea: the fullest bin of a
// quarter kilometre on each side, each tile weighted by the ground it stands
// for on a sphere. A side with nothing on it is NaN.
func hypsometricModes(gs []*Grid) (continent, ocean float64) {
	const bin, lo, hi = 250.0, -11000.0, 9000.0
	count := make([]float64, int((hi-lo)/bin))
	for _, g := range gs {
		for i := range g.Tiles {
			e := g.Height[i] - g.sea
			k := int(math.Floor((e - lo) / bin))
			if k >= 0 && k < len(count) {
				count[k] += math.Cos(latitudeOf(g, i/g.W) * math.Pi / 180)
			}
		}
	}
	mode := func(from, to int) float64 {
		best, at := 0.0, -1
		for k := from; k < to; k++ {
			if count[k] > best {
				best, at = count[k], k
			}
		}
		if at < 0 {
			return math.NaN()
		}
		return (lo + (float64(at)+0.5)*bin) / 1000
	}
	split := int((-2000 - lo) / bin)
	return mode(split, len(count)), mode(0, split)
}

// epochMyr is how long an epoch of a globe's history is, in millions of years,
// for reading its sea floor against the earth's.
//
// It was taken to be 180 Myr over the globe's sixteen epochs, eleven and a
// quarter each, on the grounds that nothing in the history said how long an
// epoch was and that the earth's oldest floor still in place is some 180 Myr
// old (Müller et al. 2008). The history says now - an epoch is epochYears,
// four million years, and every rate in it is quoted on that clock - so the
// reading was dating every floor two and four fifths times as old as the
// history made it, and a sixteen-epoch globe's oldest floor is 64 Myr and not
// 180. Past seventy there is then no floor at all to read, which is a truth
// about how long a globe's history runs and not about how its floor sinks.
var epochMyr = epochYears / myr

type subsidence struct {
	ridge float64 // metres under the sea at the youngest floor
	young float64 // metres of sinking per root Myr, to 70 Myr
	old   float64 // and past 70
}

// seafloorSubsidence reads how deep the sea floor lies against how old its
// rock is: the tiles under the sea whose basement is basalt, dated by their
// crust, the mean depth of each four million years' floor fitted against the
// root of its age in two halves either side of seventy million years. Floor
// the history made is the middle of its epoch old, to the end of the last: the
// youngest floor is two million years old and not new.
//
// The ridge is where the young half's line meets no age, which is what
// Parsons and Sclater's 2500 m is. Read as the youngest epoch's mean, it was
// the depth of floor two million years old, three hundred metres under the
// crest, and it was the depth of every tile of that epoch's floor however
// near a continent's shelf it lay.
//
// The age is the crust's, which is what a drill that went down through the
// ooze to the basement would read (the Deep Sea Drilling Project dated the
// floor so). It was read off the rock at the surface, floor that was still
// basalt, but the sea lays its limestone and its mud on the floor every epoch
// - see keepBook - and on a small globe four tiles of eighteen thousand of the
// deep floor came out bare. It was then the epoch of the basalt at the foot of
// the pile, which dates nothing from before the history: the first plates'
// floor was all one epoch. It is now the grid's floorAge, which the history
// dates that floor by as well (see firstFloorAges).
//
// And it is the ocean's floor that is read, not the shelves': ground under less
// water than a shelf's edge stands at, some two hundred metres (Shepard 1963
// has 130 on the mean), is a continent's margin whatever crust it rides, as it
// was for Parsons and Sclater, who fitted the deep floor. Read with them, a
// globe's floor sank 173 m per root Myr, the mean of each epoch dragged toward
// the shelf by the tiles a rift had floored in the middle of a continent. Nor
// the slopes': the floor is laid down its age's depth only past a shelf and a
// slope's width from continental crust (see floorDepths), and read within it,
// once the first plates' floor had its ages, the floor of 50 to 75 Myr - much
// of it on the margins of the first rifts - came out a kilometre shallow, and
// the old floor sank 0.83 as fast as the young. So a tile is read only where
// it lies that far out.
//
// And it is the basement's depth, as theirs was: the floor sounded, with what
// the sediment on it has raised it by put back (see floorSediment). Read at
// the top of the sediment, the young floor's first twenty million years of
// turbidites flattened the young line, and the old sank 0.63 as fast.
const shelfBreak = 200.0

func seafloorSubsidence(gs []*Grid) subsidence {
	return remember(fmt.Sprintf("subsidence/%p", gs[0]), func() subsidence {
		const bins = 256
		var sum, n [bins]float64
		for _, g := range gs {
			if g.floorAge == nil || g.strata == nil {
				continue
			}
			away := g.awayFrom(func(i int) bool { return math.IsNaN(g.floorAge[i]) })
			margin := tilesAcross(shelfWidth+slopeWidth, deepSpan(g))
			for i := range g.Tiles {
				if g.sea-g.Height[i] < shelfBreak || math.IsNaN(g.floorAge[i]) || away[i] < margin {
					continue
				}
				c := &g.strata[i]
				if c.rock[int(c.n)-1] != Basalt {
					continue
				}
				k := min(bins-1, int(g.floorAge[i]/epochMyr))
				sum[k] += g.sea - g.Height[i] + sedimentLoad*g.sedimentOn(i)
				n[k]++
			}
		}
		var xy, xo, yy, yo []float64
		for k := range bins {
			if n[k] < 20 {
				continue
			}
			age := (float64(k) + 0.5) * epochMyr
			d := sum[k] / n[k]
			if age <= 70 {
				xy, yy = append(xy, math.Sqrt(age)), append(yy, d)
			}
			if age >= 70 {
				xo, yo = append(xo, math.Sqrt(age)), append(yo, d)
			}
		}
		s := subsidence{ridge: math.NaN(), young: math.NaN(), old: math.NaN()}
		if len(xy) >= 3 {
			s.young = fit(xy, yy)
			s.ridge = meanOf(yy) - s.young*meanOf(xy)
		}
		if len(xo) >= 3 {
			s.old = fit(xo, yo)
		}
		return s
	})
}

// plateAreaExponent is -d ln N(>=A) / d ln A over the plates of the worlds,
// pooled, each plate's area its share of the sphere: fitted over the plates
// Bird's power law holds for, a sixth of a thousandth of the sphere up to
// the least of the great plates - a thirteenth of it - and so past the
// handful that are a fifth of the world each.
func plateAreaExponent(gs []*Grid) float64 {
	var areas []float64
	for _, g := range gs {
		held := map[uint8]float64{}
		total := 0.0
		for i := range g.Tiles {
			w := math.Cos(latitudeOf(g, i/g.W) * math.Pi / 180)
			held[g.Tiles[i].Plate] += w
			total += w
		}
		for _, a := range held {
			areas = append(areas, a/total)
		}
	}
	slices.Sort(areas)
	const lo, hi = 0.002 / (4 * math.Pi), 1 / (4 * math.Pi)
	var xs, ys []float64
	for k, a := range areas {
		if a < lo || a > hi {
			continue
		}
		// N(>=a) per world: this plate and every larger one.
		xs = append(xs, math.Log(a))
		ys = append(ys, math.Log(float64(len(areas)-k)/float64(len(gs))))
	}
	if len(xs) < 3 {
		return math.NaN()
	}
	return -fit(xs, ys)
}

// flint is the concavity and the goodness of Flint's law in the basins
// basinsOf finds: the slope down the plain drainage against the ground each
// channel tile drains, the mean log slope taken in bins of a fifth of a
// decade of area, and the line fitted through the bins from the head of a
// channel to a tenth of the largest outlet. Ground under a lake has no slope
// of its own and is left out.
func flint(gs []*Grid) (theta, r2 float64) {
	type fl struct{ theta, r2 float64 }
	r := remember(fmt.Sprintf("flint/%p/%d", gs[0], len(gs)), func() fl {
		const width = math.Ln10 / 5
		sum, count := map[int]float64{}, map[int]float64{}
		most := 0.0
		for _, g := range gs {
			tr := treeOf(g)
			in := inBasins(g, tr)
			for i := range g.Tiles {
				if !in[i] {
					continue
				}
				most = math.Max(most, tr.area[i])
				d := tr.down[i]
				if tr.area[i] < channelHead || d < 0 || g.underSea(int(d)) || (g.lakeOf != nil && g.lakeOf[i] >= 0) {
					continue
				}
				run := TileSpan
				if int(d)%g.W != i%g.W && int(d)/g.W != i/g.W {
					run *= math.Sqrt2
				}
				s := (g.Height[i] - g.Height[d]) / run
				if s <= 0 {
					continue
				}
				b := int(math.Floor(math.Log(tr.area[i]) / width))
				sum[b] += math.Log(s)
				count[b]++
			}
		}
		var xs, ys []float64
		for b, c := range count {
			if c >= 5 && math.Exp(float64(b)*width) <= most/10 {
				xs = append(xs, (float64(b)+0.5)*width)
				ys = append(ys, sum[b]/c)
			}
		}
		if len(xs) < 3 {
			return fl{math.NaN(), math.NaN()}
		}
		return fl{-fit(xs, ys), rSquared(xs, ys)}
	})
	return r.theta, r.r2
}

// rSquared is how much of y's variance the line of best fit on x explains.
func rSquared(x, y []float64) float64 {
	slope := fit(x, y)
	mx, my := meanOf(x), meanOf(y)
	var res, tot float64
	for k := range x {
		p := my + slope*(x[k]-mx)
		res += (y[k] - p) * (y[k] - p)
		tot += (y[k] - my) * (y[k] - my)
	}
	if tot == 0 {
		return math.NaN()
	}
	return 1 - res/tot
}

// chiRef is the reference concavity chi is integrated with: Perron & Royden's
// 0.45, and the middle of the real range.
const chiRef = 0.45

// chiLinearity is how straight each basin's trunk is against chi - the
// integral up it of (A0/A)^theta over distance - as the R2 of height against
// chi, averaged over the basins basinsOf finds by the ground each drains. The
// trunk is walked up from the outlet along the inflow that drains the most, to
// where the channel heads.
func chiLinearity(gs []*Grid) float64 {
	return remember(fmt.Sprintf("chi/%p/%d", gs[0], len(gs)), func() float64 {
		sum, weight := 0.0, 0.0
		for _, g := range gs {
			tr := treeOf(g)
			up := make([]int32, len(g.Tiles))
			for i := range up {
				up[i] = -1
			}
			for i, d := range tr.down {
				if d >= 0 && (up[d] < 0 || tr.area[i] > tr.area[up[d]]) {
					up[d] = int32(i)
				}
			}
			for _, b := range basinsOf(g, tr) {
				outlet := b[0]
				for _, i := range b {
					if tr.area[i] > tr.area[outlet] {
						outlet = i
					}
				}
				var chi, z []float64
				c := 0.0
				for i := outlet; i >= 0 && tr.area[i] >= channelHead; i = up[i] {
					if len(chi) > 0 {
						prev := tr.down[i]
						run := 1.0
						if int(prev)%g.W != int(i)%g.W && int(prev)/g.W != int(i)/g.W {
							run = math.Sqrt2
						}
						c += math.Pow(1/tr.area[i], chiRef) * run * TileSpan
					}
					chi, z = append(chi, c), append(z, g.Height[i])
				}
				if len(chi) < 10 {
					continue
				}
				if r := rSquared(chi, z); !math.IsNaN(r) {
					sum, weight = sum+r*float64(len(b)), weight+float64(len(b))
				}
			}
		}
		return sum / weight
	})
}

type meanderReading struct {
	wavelength, sinuosity float64
	// reaches is how many reaches the reading rests on.
	reaches int
}

// meanderReach is how many steps of a river a meander is read over.
const meanderReach = 16

// minMeanderReaches is the fewest reaches a meander reading may rest on:
// fewer, and it reads as NaN rather than as one river's chance bends.
const minMeanderReaches = 8

// meanderingSlope is the steepest a river carrying q cubic metres a second
// can fall and still meander rather than braid: Leopold & Wolman 1957's line,
// S = 0.06 Q^-0.44 with Q in cubic feet a second, which is 0.0125 Q^-0.44 in
// cubic metres.
func meanderingSlope(q float64) float64 { return 0.0125 * math.Pow(math.Max(q, 1e-9), -0.44) }

// meanders reads the rivers great enough to wander - meanderFlow and more -
// reach by reach along the way the model sends their water, on
// reaches of meanderReach steps gentle enough by meanderingSlope to meander.
// Sinuosity is the length along the river over the straight line between the
// ends of the reach. Wavelength is read off the river's offset from that line,
// eased over three steps: twice the length of the line over how many times the
// river crosses it. A river here is a tile wide, so tiles are widths.
func meanders(gs []*Grid) meanderReading {
	return remember(fmt.Sprintf("meanders/%p/%d", gs[0], len(gs)), func() meanderReading {
		var wl, sn []float64
		for _, g := range gs {
			// A lake is no reach of a river, but an open one is no end of it
			// either: the river goes on from where the lake lets it out.
			river := func(i int) bool {
				return !g.underSea(i) && g.lakeOf[i] < 0 && g.Flow[i] >= meanderFlow
			}
			next := func(i int) int {
				j := int(g.down[i])
				for n := 0; j >= 0 && g.lakeOf[j] >= 0 && n < len(g.Lakes); n++ {
					j = int(g.Lakes[g.lakeOf[j]].Outlet)
				}
				if j >= 0 && g.lakeOf[j] >= 0 {
					return -1
				}
				return j
			}
			// The rivers as streams: each walked up from its mouth along the
			// inflow carrying the most, its other inflows mouths of their own.
			// A stream is a river from its head to where it meets a greater
			// one, which is what a reach of it is read along.
			inflows := map[int][]int{}
			var mouths []int
			for i := range g.Tiles {
				if !river(i) {
					continue
				}
				if j := next(i); j >= 0 && river(j) {
					inflows[j] = append(inflows[j], i)
				} else {
					mouths = append(mouths, i)
				}
			}
			for len(mouths) > 0 {
				mouth := mouths[len(mouths)-1]
				mouths = mouths[:len(mouths)-1]
				var stem []int
				for i := mouth; i >= 0; {
					stem = append(stem, i)
					main := -1
					for _, j := range inflows[i] {
						if main < 0 || g.Flow[j] > g.Flow[main] {
							main = j
						}
					}
					for _, j := range inflows[i] {
						if j != main {
							mouths = append(mouths, j)
						}
					}
					i = main
				}
				slices.Reverse(stem)
				// Unwrapped, so a globe's seam is no jump.
				xs, ys, hs, qs := make([]float64, len(stem)), make([]float64, len(stem)), make([]float64, len(stem)), make([]float64, len(stem))
				x, y := float64(stem[0]%g.W), float64(stem[0]/g.W)
				for k, i := range stem {
					xs[k], ys[k], hs[k], qs[k] = x, y, g.Height[i], g.Flow[i]
					if k+1 < len(stem) {
						d := g.Delta(g.PosOf(i), g.PosOf(stem[k+1]))
						x, y = x+float64(d.X), y+float64(d.Y)
					}
				}
				for s := 0; s+meanderReach < len(xs); s += meanderReach / 2 {
					e := s + meanderReach
					dx, dy := xs[e]-xs[s], ys[e]-ys[s]
					chord := math.Hypot(dx, dy)
					along := 0.0
					for k := s; k < e; k++ {
						along += math.Hypot(xs[k+1]-xs[k], ys[k+1]-ys[k])
					}
					if chord < meanderReach/4 || (hs[s]-hs[e])/(along*TileSpan) > meanderingSlope(meanOf(qs[s:e+1])) {
						continue
					}
					sn = append(sn, along/chord)
					off := make([]float64, e-s+1)
					for k := range off {
						off[k] = ((xs[s+k]-xs[s])*dy - (ys[s+k]-ys[s])*dx) / chord
					}
					eased := make([]float64, len(off))
					for k := range off {
						a, b := max(0, k-1), min(len(off)-1, k+1)
						eased[k] = (off[a] + off[k] + off[b]) / 3
					}
					level(eased)
					crossings := 0
					for k := 1; k < len(eased); k++ {
						if (eased[k-1] < 0) != (eased[k] < 0) {
							crossings++
						}
					}
					if crossings > 0 {
						wl = append(wl, 2*chord/float64(crossings))
					}
				}
			}
		}
		if len(sn) < minMeanderReaches {
			return meanderReading{math.NaN(), math.NaN(), len(sn)}
		}
		if len(wl) == 0 { // reaches gentle enough, and not one of them bending
			return meanderReading{math.NaN(), meanOf(sn), len(sn)}
		}
		return meanderReading{quantile(wl, 0.5), meanOf(sn), len(sn)}
	})
}

// zonalTemp is the year's mean temperature over both hemispheres between two
// latitudes, at the ground: the latitude's mean, less the lapse over land
// standing above the sea, with the currents' warmth off the coast added.
func zonalTemp(gs []*Grid, lo, hi float64) float64 {
	sum, n := 0.0, 0.0
	for _, g := range gs {
		c := NewClimateOn(Terms{Height: g.H, Wrap: true})
		for y := 0; y < g.H; y++ {
			if l := math.Abs(latitudeOf(g, y)); l < lo || l > hi {
				continue
			}
			w := math.Cos(latitudeOf(g, y) * math.Pi / 180)
			for i := y * g.W; i < (y+1)*g.W; i++ {
				t := c.MeanAt(y) - Lapse*math.Max(0, g.Height[i]-g.sea) + g.CoastWarmth(i)
				sum, n = sum+w*t, n+w
			}
		}
	}
	return sum / n
}

// zonalRain is the year's rain over land and sea alike between two
// latitudes, both hemispheres, in mm.
func zonalRain(gs []*Grid, lo, hi float64) float64 {
	sum, n := 0.0, 0.0
	for _, g := range gs {
		for y := 0; y < g.H; y++ {
			if l := math.Abs(latitudeOf(g, y)); l < lo || l > hi {
				continue
			}
			w := math.Cos(latitudeOf(g, y) * math.Pi / 180)
			for i := y * g.W; i < (y+1)*g.W; i++ {
				sum, n = sum+w*g.Rain(i), n+w
			}
		}
	}
	return sum / n
}

// driestBelt is the latitude, both hemispheres together, of the driest band
// of five degrees between ten and forty-five.
func driestBelt(gs []*Grid) float64 {
	best, at := math.Inf(1), math.NaN()
	for lo := 10.0; lo < 45; lo += 2.5 {
		if r := zonalRain(gs, lo, lo+5); r < best {
			best, at = r, lo+2.5
		}
	}
	return at
}

// forestByAridity is the share of the dry land whose aridity - what the air
// could take up in a year over the rain, both as the model reads them -
// falls between lo and hi that is forest.
func forestByAridity(gs []*Grid, lo, hi float64) float64 {
	forest, n := 0.0, 0.0
	for _, g := range gs {
		for i := range g.Tiles {
			t := &g.Tiles[i]
			if g.underSea(i) || t.Wet() || t.Terrain.Tidal() || g.air == nil {
				continue
			}
			pet := g.pet(i)
			ai := pet / math.Max(1e-9, g.Rain(i))
			if ai < lo || ai >= hi {
				continue
			}
			n++
			if t.Terrain == Forest {
				forest++
			}
		}
	}
	if n == 0 {
		return math.NaN()
	}
	return forest / n
}

// landRain is the mean year's rain on the dry land of the worlds, in mm.
func landRain(gs []*Grid) float64 {
	sum, n := 0.0, 0.0
	for _, g := range gs {
		for i := range g.Tiles {
			if !g.underSea(i) {
				sum, n = sum+g.Rain(i), n+1
			}
		}
	}
	return sum / n
}
