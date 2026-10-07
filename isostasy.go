package terra

import (
	"math"

	"github.com/LukasSelin/terra/internal/kernel"
	"github.com/LukasSelin/terra/internal/phase"
)

// What holds the ground up: the crust's thickness, floating on the mantle, and
// the plate bending under what is laid on it.
//
// A history's plates used to float at two fixed levels, one for continent and
// one for ocean floor, and a plate came a share of the way to its own level
// every epoch (settleTime), the whole plate by what its middle was short of; a
// field of noise a kilometre either way (the bow) warped the ground inside a
// plate. The code called it a stand-in for the isostasy the model did not
// have, and the history's land showed it. The wear took the ranges down and
// nothing buoyed them back, so a globe's continental crust ended a plateau a
// few hundred metres over its sea (median 331 m, the highest 1.9 km) with its
// ocean floor at the land's own height: G1 (#40) had to give the map the
// earth's curve in the history's order, since the history's own spread was not
// a planet's. With the crust floating, the country is the history's own height
// (hypsometry.go).
//
// So the crust has a thickness now, carried with it as the plates carry their
// ground: continent starts at continentCrust and ocean floor at oceanCrust,
// and new floor at a ridge is made at oceanCrust. What a meeting does is laid
// on it - a collision, an arc or islands thicken the crust by what they raise,
// a rift thins it in proportion to what is left, a hotspot piles its lava on
// top - and so does what the weather does: the wear takes its metres off the
// crust and the fill lays them on. A trench is the one meeting that is not a
// thickness: it is the floor bent down by the slab pulling it, and is laid on
// the ground alone.
//
// And how high a column floats is Airy's (Turcotte and Schubert, Geodynamics,
// ch. 2): crust of crustDensity on mantle of mantleDensity stands
// airyRise of its thickness higher for every metre it is thicker, and the
// root under it goes down by the rest. A column seaCrust thick floats at the
// sea (seaDatum). Below the sea the water fills what the column is short of,
// and the shortfall is deeper by mantleDensity / (mantleDensity -
// seaDensity). Ocean floor stands where its age puts it, by abyss.go's GDH1
// (Stein and Stein 1992), which is the cooling plate's own isostasy, plus what
// its crust is thicker than oceanCrust by, read under water. See levelOf.
//
// A plate is not a set of columns, though. It is an elastic shell, and it
// holds up a load narrower than its flexural wavelength where a column alone
// would sink under it: a valley cut a tile wide is not refilled from below,
// and a range is held up partly by the plate either side of it, which bends
// down into a moat. So the ground is brought to its level through the
// flexure of a thin plate of elasticThickness (Watts 2001): the
// deflection under a load is the load's Airy deflection filtered by
//
//	1 / (1 + D k⁴ / (ρm g)),   D = E Te³ / 12(1 - ν²)
//
// for k the wavenumber of the load, which is all of it at the scale of a
// continent and little of it at the scale of a valley. The deflection is the
// plate's state and goes with the crust (sag): what the deflection is filtered
// from is the ground as the surface processes have left it, so a load the
// plate holds up stays held up, and only what changes is answered - but not
// for ever: the plate lets go of what it holds over relaxTime, as a
// viscoelastic plate does. See isostasy.
//
// What that gives the weather to work against is the rebound. A broad range
// worn down by a metre is a metre lighter, and its root floats it back up by
// crustDensity / mantleDensity of a metre, five sixths (Molnar and England
// 1990): the surface comes down by a sixth of what was taken off it, and the
// rock rises by five sixths. A range wears down slowly for that reason, and
// stands on a root that goes on holding it up after the plates have stopped
// pushing.
const (
	crustDensity  = 2800.0 // kg/m³, the continental crust's mean
	mantleDensity = 3300.0 // kg/m³, the upper mantle's
	seaDensity    = 1030.0 // kg/m³, the sea's
	// continentCrust and oceanCrust are what the two kinds of crust are when
	// the plates first break, and what a ridge makes: some 35 km of continent
	// (the reference column of Turcotte and Schubert, near the mean of
	// Christensen and Mooney's 1995 compilation, 41 km with the shields) and
	// 7 km of ocean floor (White, McKenzie and O'Nions 1992).
	continentCrust = 35 * km
	oceanCrust     = 7 * km
	// seaCrust is the thickness of continental crust that floats at the sea.
	// It is what puts the continents where the earth's stand: the crust they
	// start at stands airyRise times five kilometres over it, 0.76 km, where
	// the earth's land averages some 0.8 (Cogley 1984), and the shelves, the
	// continent's edge thinned by its rifting, stand at it or under it.
	seaCrust = 30 * km
	// elasticThickness is how thick an elastic plate bends as the lithosphere
	// does. Watts (2001) has 20 to 40 km for most continents and the old
	// ocean floor, more under the shields and less in the young rifts; one
	// figure is taken for the whole of a world, since the bending is solved
	// across the whole of it at once.
	elasticThickness = 30 * km
	youngsModulus    = 100e9 // Pa, Watts 2001
	poissonRatio     = 0.25
	// crustMost is the thickest crust a collision can pile up. Under Tibet and
	// the high Andes it is seventy kilometres and a little more (Owens and
	// Zandt 1997; Beck and Zandt 2002) and nowhere is it much thicker: below
	// that the root's basalt turns to eclogite, heavier than the mantle round
	// it, and founders (Bird 1979; Kay and Kay 1993), and the lower crust of a
	// plateau that high flows out from under it rather than thicken further
	// (Royden and others 1997). Without it a range that went on being pushed
	// went on standing on its root, to a hundred and twenty kilometres of
	// crust and the history's highest ground thirty kilometres over its sea.
	crustMost = 70 * km
	// relaxTime is how long the plate takes to let go of a load it holds up.
	// A plate is elastic over the time a load is laid on it and not for ever:
	// Walcott (1970) read the rigidity the continents show under a load
	// falling with the load's age, and Beaumont (1981) built the foreland
	// basins on a plate that relaxes, as a Maxwell body does, toward Airy's
	// columns. Held up for good, a plate kept every step the plates' moves
	// put in it: two crusts carried side by side, one fifteen kilometres
	// thicker than the other, were held fifteen kilometres apart in height a
	// tile apart, and the thick side's edge, bent up by the plate, stood
	// thirteen kilometres over the sea. Ten million
	// years lets go of a third of it an epoch: what was laid this epoch is
	// held as the plate holds it, and what was laid a few epochs ago stands
	// near enough on its own root. It is chosen, not measured.
	relaxTime = 10 * myr
	// trenchDeepest is the deepest a trench is pulled down under the sea: the
	// Challenger Deep is 10.9 km (Gardner and others 2014). A trench is the
	// plate bent down by the slab hanging off it, which pulls as long as the
	// slab goes down; laid on the ground every epoch with nothing to hold it,
	// the trenches of a long subduction went on deepening, to twenty-eight
	// kilometres under the sea.
	trenchDeepest = 11 * km
	// seaDatum is the height, over the history's nothing, that the crust
	// floats against: high enough that the oldest floor and the trench in
	// front of an arc are still ground. It is where the sea is first poured,
	// and the sea a history runs against stands wherever its water fills the
	// basins after that: see seawater.go.
	seaDatum = 12 * km
)

// airyRise is how far a column of crust stands higher for every metre it is
// thicker, out of the water: (ρm - ρc) / ρm. wetRise is the same where the
// sea fills over it, (ρm - ρc) / (ρm - ρw).
const (
	airyRise = (mantleDensity - crustDensity) / mantleDensity
	wetRise  = (mantleDensity - crustDensity) / (mantleDensity - seaDensity)
)

// flexuralRigidity is D, in newton metres.
const flexuralRigidity = youngsModulus * elasticThickness * elasticThickness * elasticThickness / (12 * (1 - poissonRatio*poissonRatio))

// flexuralParameter is α = (4D / ρm g)^¼, in metres: how far a plate's bend
// under a narrow load reaches. A load's moat is at about 2α from it, and the
// bend is gone by about 4α.
var flexuralParameter = math.Pow(4*flexuralRigidity/(mantleDensity*gravity), 0.25)

// levelOf is the height a column of crust thick metres floats at, ocean floor
// where ocean says so and of age years: Airy's for a continent, about seaCrust,
// and GDH1's for a floor, about oceanCrust. Both are read under water where
// they stand below the sea.
func levelOf(thick float64, ocean bool, age float64) float64 {
	if ocean {
		return seaDatum - floorDepth(age/myr) + wetRise*(thick-oceanCrust)
	}
	e := airyRise * (thick - seaCrust)
	if e < 0 {
		e *= mantleDensity / (mantleDensity - seaDensity)
	}
	return seaDatum + e
}

// ageAt is how old tile i's crust is in epoch epoch, in years, reading it as
// floorDepths does: the middle of the epoch it was made in, and the first
// plates' floor as old as the history so far and as old again as it already
// was.
func (cr *crust) ageAt(i, epoch int) float64 {
	if cr.born[i] == 0 {
		return float64(epoch+1)*epochYears + float64(cr.aged[i])
	}
	return (float64(epoch-int(cr.born[i])) + 0.5) * epochYears
}

// levelAt is the height tile i's crust floats at in epoch epoch.
func (cr *crust) levelAt(i, epoch int) float64 {
	return levelOf(float64(cr.thick[i]), cr.ocean[i], cr.ageAt(i, epoch))
}

// layCrust gives the first plates their crust and floats them on it. The two
// kinds meet over a margin a handful of tiles wide: the crust's thickness is
// spread over marginRamp passes, so that the edge of a continent is
// continental crust thinned as a rifted margin's is, under the sea, and the
// floor beside it a little thicker. The molten era's lumps are kept on top as
// what they were, a few tens of metres of texture.
func (cr *crust) layCrust(g *Grid) {
	n := len(g.Tiles)
	t := make([]float64, n)
	for i := range t {
		t[i] = continentCrust
		if cr.ocean[i] {
			t[i] = oceanCrust
		}
	}
	for k := 0; k < g.passes(marginRamp); k++ {
		t = g.spread(t)
	}
	for i := range g.Tiles {
		cr.thick[i] = float32(t[i])
		cr.sag[i] = 0
		by := cr.levelAt(i, 0)
		g.Height[i] += by
		g.strata[i].lift(by)
	}
}

// isostasy brings the ground toward the level its crust floats at, through the
// bending of the plate. What is filtered is the Airy deflection the ground as
// it stands would have, column by column - how far it is above its level,
// plus how far the plate is already bent down under it - and the plate's new
// bend is that, filtered; the ground rises by however much less it is bent
// than it was. A load the plate held up last epoch is held up still, less what
// relax lets go of: what has changed since is answered, all of what is broad
// and little of what is narrow. See the top of this file.
//
// relax is the share of what the plate holds up that it lets go of: see
// relaxTime. It is nothing for a load the plate has only just been given.
//
// worn, where it is given, is what the epoch's weather took off each tile
// (negative where it laid ground down), and what the plate's bending lifts
// the land it was taken from by is added to the history's reading of its
// rebound: see crust.rebound.
func (g *Grid) isostasy(cr *crust, epoch int, worn []float64, relax float64) {
	defer phase.Start("isostasy")()
	n := len(g.Tiles)
	if len(cr.local) != n {
		cr.local = make([]float64, n)
	}
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			cr.local[i] = g.Height[i] + float64(cr.sag[i]) - cr.levelAt(i, epoch)
		}
	})
	bend := cr.flexure(g, cr.local)
	for i := range g.Tiles {
		if worn != nil && worn[i] > 0 && !cr.ocean[i] && g.Height[i] > g.base {
			cr.eroded += worn[i]
			cr.rebound += float64(cr.sag[i]) - bend[i]
		}
		sag := bend[i] + relax*(cr.local[i]-bend[i])
		rise := float64(cr.sag[i]) - sag
		cr.sag[i] = float32(sag)
		g.Height[i] += rise
		cr.lifted[i] += rise
		g.strata[i].lift(rise)
	}
}

// worn is the crust's room for what an epoch's weather takes off each tile,
// holding the heights as they stand before it.
func (cr *crust) worn(g *Grid) []float64 {
	if len(cr.wear) != len(g.Tiles) {
		cr.wear = make([]float64, len(g.Tiles))
	}
	copy(cr.wear, g.Height)
	return cr.wear
}

// relaxing is the share of what the plate holds up elastically that it lets
// go of in an epoch: see relaxTime.
var relaxing = -math.Expm1(-epochYears / relaxTime)

// flexure is load, a field of Airy deflections in metres, filtered as an
// elastic plate of flexuralRigidity bends under it: by the transform, the
// filter, and the transform back, over a field padded to a power of two. Round
// a world that goes round, the field goes round, where its width is a power
// of two already; anywhere else - its poles, and the sides of a map that
// does not - the field is mirrored into the padding, as a plate whose edge is
// free to bend and is not held. The result is the plate's, and is overwritten
// by the next call.
func (cr *crust) flexure(g *Grid, load []float64) []float64 {
	f := cr.flexed(g)
	bw, bh, px, py := f.w, f.h, f.px, f.py
	buf := f.buf
	srcX := func(c int) int {
		x := c - px
		if g.Wrap && px == 0 {
			return x
		}
		if g.Wrap {
			return ((x % g.W) + g.W) % g.W
		}
		return mirror(x, g.W)
	}
	InParallel(bh, f.workers, func(r, _ int) {
		y := mirror(r-py, g.H)
		row := buf[r*bw : (r+1)*bw]
		for c := range row {
			row[c] = complex(load[y*g.W+srcX(c)], 0)
		}
		kernel.FFT(row, false)
	})
	f.columns(false)
	for i, k := range f.filter {
		buf[i] *= complex(k, 0)
	}
	f.columns(true)
	if len(f.out) != len(load) {
		f.out = make([]float64, len(load))
	}
	InParallel(g.H, f.workers, func(y, _ int) {
		row := buf[(y+py)*bw : (y+py+1)*bw]
		kernel.FFT(row, true)
		for x := 0; x < g.W; x++ {
			f.out[y*g.W+x] = real(row[x+px])
		}
	})
	return f.out
}

// mirror reads a place s along a line n long, reflected back in at either
// end, and held to the line however far off it is.
func mirror(s, n int) int {
	if s < 0 {
		s = -s - 1
	}
	if s >= n {
		s = 2*n - s - 1
	}
	return max(0, min(n-1, s))
}

// flexPlan is what flexure works in: the padded field, its size, how far
// in the map's field starts, the filter at each wavenumber, and room for a
// column for each worker.
type flexPlan struct {
	w, h, px, py int
	workers      int
	buf          []complex128
	filter       []float64
	cols         [][]complex128
	out          []float64
}

// columns transforms every column of the plan's field, forward or back.
func (f *flexPlan) columns(inverse bool) {
	InParallel(f.w, len(f.cols), func(c, worker int) {
		col := f.cols[worker]
		for r := 0; r < f.h; r++ {
			col[r] = f.buf[r*f.w+c]
		}
		kernel.FFT(col, inverse)
		for r := 0; r < f.h; r++ {
			f.buf[r*f.w+c] = col[r]
		}
	})
}

// flexed is the crust's plan for g, made the first time it is asked for.
func (cr *crust) flexed(g *Grid) *flexPlan {
	if cr.plan != nil {
		return cr.plan
	}
	span := g.span()
	// Padding as far as the plate's bend reaches, four flexural parameters.
	pad := max(2, int(math.Ceil(4*flexuralParameter/span)))
	f := &flexPlan{}
	if g.Wrap && kernel.PowerOfTwo(g.W) {
		f.w = g.W
	} else {
		f.px = pad
		f.w = nextPow2(g.W + 2*pad)
	}
	f.py = pad
	f.h = nextPow2(g.H + 2*pad)
	f.buf = make([]complex128, f.w*f.h)
	f.filter = make([]float64, f.w*f.h)
	ratio := flexuralRigidity / (mantleDensity * gravity)
	for r := 0; r < f.h; r++ {
		rr := r
		if rr >= f.h/2 {
			rr -= f.h
		}
		ky := 2 * math.Pi * float64(rr) / (float64(f.h) * span)
		for c := 0; c < f.w; c++ {
			cc := c
			if cc >= f.w/2 {
				cc -= f.w
			}
			kx := 2 * math.Pi * float64(cc) / (float64(f.w) * span)
			k2 := kx*kx + ky*ky
			f.filter[r*f.w+c] = 1 / (1 + ratio*k2*k2)
		}
	}
	// Spread over goroutines where there is ground enough to be worth it, as
	// EachRow is.
	f.workers = 1
	if len(g.Tiles) >= spreadTiles {
		f.workers = WorkersFor(f.w)
	}
	f.cols = make([][]complex128, f.workers)
	for k := range f.cols {
		f.cols[k] = make([]complex128, f.h)
	}
	cr.plan = f
	return f
}

// nextPow2 is the least power of two no less than n.
func nextPow2(n int) int {
	p := 1
	for p < n {
		p *= 2
	}
	return p
}

// thicken lays by metres on tile i's crust: what a meeting raises, a hotspot
// piles up or the weather lays down, or, negative, what it takes off. The
// ground is moved by the caller, and the plate answers both at the next
// isostasy. No crust is thicker than crustMost - what a collision piles on past
// it founders - and none is thinner than nothing. It is what was laid, which is
// what the caller moves the ground by.
func (cr *crust) thicken(i int, by float64) float64 {
	was := float64(cr.thick[i])
	now := math.Max(0, math.Min(math.Max(was, crustMost), was+by))
	cr.thick[i] = float32(now)
	return now - was
}
