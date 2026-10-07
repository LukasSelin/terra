package atmos

import "math"

// The water the ground holds from one season to the next.
//
// What the air took back off the land was Budyko's curve in Fu's form, read
// on the year: the rain and what the air could take up, both a year's, and
// one shape, ω, for the whole planet. A year has no seasons in it, so a
// monsoon coast whose rain comes with its heat and a Mediterranean one whose
// rain comes in the cold were the same coast, and the rivers of both ran the
// same all year.
//
// Here the ground is a bucket (Manabe, 1969), run through the year's four
// phases in their order. The phase's rain fills it; the air takes from it
// what it could take up, all of it while the bucket is over bucketDry full
// and in proportion to its fill under that, which is Manabe's β; and what
// it cannot hold runs off. It does not only run off when it is brim full: a
// soil half full sheds some of a storm off the ground that is already wet,
// and the share of the rain that goes on to the rivers is the bucket's fill
// to the power bucketShape - the soil routine of the HBV model (Bergström,
// 1992), whose recharge is the rain times (SM/FC)^β. When the bucket is full
// all of the rain goes.
//
// With rain and evaporation the same all year, the bucket settles where
// what it sheds and what the air takes add to the rain:
//
//	x² + (φ/bucketDry)·x = 1,   runoff over rain = x²
//
// for its fill x and the dryness φ, PET over rain, whatever the bucket's
// size. That is Fu's curve at the ω of 2.6 the world's catchments fit
// (Fu, 1981; Zhang and others, 2004) to within three parts in a hundred of
// the rain from φ of a tenth to φ of five: the two shapes below were chosen
// for it, within the ranges their authors give. What the seasons do to it is
// what the
// bucket adds: where the rain comes with the heat the ground gives the air
// more of it, where the rain comes in the cold and the heat in the dry it
// gives less, and a deeper bucket carries more of the wet season into the
// dry one. So the size of the bucket - how deep the roots reach into how much
// soil - is what ω was standing in for, as Milly (1994) and Zhang and others
// (2001) found it.
//
// Storms are not in it: a phase's rain falls evenly through the phase. A
// storm on a part-filled bucket sheds more than the same rain spread out,
// which is why a deeper rooted cover takes more of an unseasonal climate's
// rain too (Porporato and others, 2004). That waits for a day's rain from the
// weather.
const (
	// bucketDry is the share of a full bucket under which the ground gives
	// the air less than it could take up, in proportion to its fill.
	// Manabe (1969) took three quarters of field capacity; four fifths is
	// what fits Fu's curve best with bucketShape at two, to 2.6 parts in a
	// hundred of the rain against 4.2 at three quarters.
	bucketDry = 0.8
	// bucketShape is the HBV soil routine's β: the share of the rain the
	// bucket sheds goes as its fill to this power. Bergström (1992) finds
	// calibrated catchments between one and six; two is what makes the
	// unseasonal bucket Fu's curve at ω = 2.6. The steps below square the
	// fill rather than raise it to a power, so it is a whole two.
	bucketShape = 2
	// bucketSteps is how many steps a phase of the year is taken in: a
	// fortnight's. bucketSpins is how many rounds of two years the bucket is
	// run through before the year it is read on, from where an unseasonal
	// year would leave it: see bucketRun. The tests hold both to the year
	// at fine steps and to the steady year.
	bucketSteps = 6
	bucketSpins = 1
)

// yearOrder is the phases in the order the year takes them: the spring
// equinox, the north's summer, the autumn and the north's winter. See dayOf.
var yearOrder = [Phases]int{1, 2, 3, 0}

// BucketYear is a bucket's steady year, phase by phase, in mm: the water it
// held on the mean through the phase, what the air took from it, and what it
// shed to the rivers in the phase.
//
// Where the year is cold enough to snow (see BucketCold) it is the snow's
// year too: the snow water lying on the mean through the phase, what of it
// melted in the phase, how much of the phase's precipitation fell as snow,
// and the share of the ground the snow covered on the mean. What the air
// took off the snow is in Evap with what it took off the soil, and what a
// glacier's ice carried off is in Runoff. Ice is the ground's mass balance
// where the snow outlasts the year, the mm of water a year it gains as ice,
// and nothing where it melts out.
type BucketYear struct {
	Water, Evap, Runoff         [Phases]float64
	Snow, Melt, Snowfall, Cover [Phases]float64
	Ice                         float64
}

// Evaporated is what the air took from the bucket in the year, in mm.
func (b *BucketYear) Evaporated() float64 {
	return b.Evap[0] + b.Evap[1] + b.Evap[2] + b.Evap[3]
}

// Shed is what the bucket sent to the rivers in the year, in mm.
func (b *BucketYear) Shed() float64 {
	return b.Runoff[0] + b.Runoff[1] + b.Runoff[2] + b.Runoff[3]
}

// Bucket runs a bucket holding up to hold mm through its steady year, with
// rain mm falling in each phase and the air able to take up pet mm in each.
//
// Each step is implicit in the fill it ends at: the fill y a step ends at
// meets y + a·y² + d·β(y) = x + a, for the fill x it started at and a and d
// the step's rain and evaporation over the bucket's size. That is a
// quadratic with one root in [0, 1], so the step can neither overfill nor
// empty the bucket however long it is, and what goes in is what comes out
// and what stays, to the rounding.
func Bucket(hold float64, rain, pet *[Phases]float64) BucketYear {
	return bucketRun(hold, rain, pet, bucketSpins, bucketSteps, nil)
}

// BucketCold is Bucket on ground whose year has a mean of mean degrees at the
// ground, swinging swing either side of it, signed by hemisphere as SwingAt
// is: the phase's precipitation falls as snow on its cold days, lies, and
// reaches the bucket as it melts (see snowYear). A year too warm ever to snow
// is Bucket's to the bit.
func BucketCold(hold float64, rain, pet *[Phases]float64, mean, swing float64) BucketYear {
	if SnowFree(mean, swing) {
		return Bucket(hold, rain, pet)
	}
	return bucketRun(hold, rain, pet, bucketSpins, bucketSteps, &snowYearOf{mean, swing})
}

// snowYearOf is the year a bucket's snow is read on: its mean at the ground
// and its signed swing.
type snowYearOf struct{ mean, swing float64 }

// bucketRun is Bucket, taken in steps steps a phase, with spins rounds of
// spinning up before the year read, under the snow of year cold where it is
// not nil.
//
// The fill a year ends at goes toward the steady year's by much the same
// share each year, so after two years the rest of the way is a geometric
// series, and the next round starts where it sums to (Aitken's
// extrapolation). A bucket in a cold dry country, whose air takes little a
// year from a deep store, would otherwise take tens of years to come to its
// steady year.
func bucketRun(hold float64, rain, pet *[Phases]float64, spins, steps int, cold *snowYearOf) BucketYear {
	var out BucketYear
	// The water that reaches the soil in each step and what the air could
	// still take up off it, in mm, in the year's order: the phase's rain
	// and evaporation evenly over its steps where there is no snow, and
	// the rain and the melt, and what the snow left the air wanting, where
	// there is.
	n := Phases * steps
	var buf [2 * Phases * bucketSteps]float64
	var in, dry []float64
	if 2*n <= len(buf) {
		in, dry = buf[:n], buf[n:2*n]
	} else {
		in, dry = make([]float64, n), make([]float64, n)
	}
	if cold == nil || !snowYear(rain, pet, cold.mean, cold.swing, steps, in, dry, &out) {
		for s := range n {
			k := yearOrder[s/steps]
			in[s] = math.Max(0, rain[k]) / float64(steps)
			dry[s] = math.Max(0, pet[k]) / float64(steps)
		}
	}
	if hold <= 0 {
		// No store: the air takes what it can of the water as it reaches
		// the ground and the rest goes.
		for s := range n {
			k := yearOrder[s/steps]
			e := math.Max(0, math.Min(in[s], dry[s]))
			out.Evap[k] += e
			out.Runoff[k] += math.Max(0, in[s]-e)
		}
		return out
	}
	var liquid, take [Phases]float64
	for s := range n {
		k := yearOrder[s/steps]
		liquid[k] += in[s]
		take[k] += dry[s]
		in[s] /= hold
		dry[s] /= hold
	}
	x := bucketStart(&liquid, &take)
	for range spins {
		x1 := bucketYear(x, in, dry, steps, nil)
		x2 := bucketYear(x1, in, dry, steps, nil)
		if gone := x1 - x; math.Abs(gone) > 1e-12 {
			if s := (x2 - x1) / gone; s > 0 && s < 1 {
				x2 = clamp01(x2 + (x2-x1)*s/(1-s))
			}
		}
		x = x2
	}
	var soil BucketYear
	bucketYear(x, in, dry, steps, &soil)
	for k := range Phases {
		out.Water[k] = soil.Water[k] * hold
		out.Evap[k] += soil.Evap[k] * hold
		out.Runoff[k] += soil.Runoff[k] * hold
	}
	return out
}

// bucketYear runs a bucket a year from fill x, with in and dry each step's
// water and evaporation over the bucket's size, steps steps a phase in the
// year's order, and gives the fill it ends at. Where out is not nil the year
// is written into it, over the bucket's size.
func bucketYear(x float64, in, dry []float64, steps int, out *BucketYear) float64 {
	s := 0
	for _, k := range yearOrder {
		var water, evap, runoff float64
		for range steps {
			a, d := in[s], dry[s]
			s++
			// Under bucketDry, the air takes in proportion to the fill.
			b := 1 + d/bucketDry
			y := 2 * (x + a) / (b + math.Sqrt(b*b+4*a*(x+a)))
			e := d * y / bucketDry
			if y > bucketDry {
				// Over it, the air takes all it could.
				s := x + a - d
				y = 2 * s / (1 + math.Sqrt(1+4*a*s))
				e = d
			}
			water += y
			evap += e
			runoff += a * y * y
			x = y
		}
		if out != nil {
			out.Water[k] = water / float64(steps)
			out.Evap[k] = evap
			out.Runoff[k] = runoff
		}
	}
	return x
}

// bucketStart is the fill an unseasonal year with the same rain and
// evaporation would hold the bucket at: the root of x² + (φ/bucketDry)·x = 1
// under bucketDry, and of x² = 1 − φ over it.
func bucketStart(rain, pet *[Phases]float64) float64 {
	var p, e float64
	for k := range Phases {
		p += math.Max(0, rain[k])
		e += math.Max(0, pet[k])
	}
	if p <= 0 {
		return 0
	}
	phi := e / p
	if 1-phi >= bucketDry*bucketDry {
		return math.Sqrt(1 - phi)
	}
	q := phi / bucketDry
	return 2 / (q + math.Sqrt(q*q+4))
}

// The size of the bucket. It is the water the soil holds between what drains
// out of it and what the roots cannot pull back, its plant-available water,
// over the depth the roots reach; and where they reach past the soil into the
// weathered rock, the water that rock holds over the rest.
//
// The roots reach as deep as the cover is tall, near enough. Schenk and
// Jackson (2002) put 95 parts in a hundred of the roots of grasslands and
// shrublands in the top metre or so and of forests in the top one to two
// metres, and Canadell and others (1996) find the deepest roots under forests
// metres deeper than under grass. A forest's deeper bucket is why it takes
// more of the same rain back than grass (Zhang and others, 2001, whose
// plant-available water coefficient is 2 for forest and 0.5 for grass).
// Where the land's vegetation has been laid the cover is what stands on the
// ground (see the land's vegetation.go); before it has - through a history,
// whose epochs have no plants of their own yet - the cover is the climate's:
// a forest where the rain outruns what the air could take, none where the
// air could take twice the rain, and between in proportion (Budyko, 1974;
// Holdridge, 1967). That is RootDepth.
//
// Weathered rock under the soil holds water the roots take too - rock
// moisture, a few to eight parts in a hundred of its volume that the trees of
// a dry summer live on (Rempe and Dietrich, 2018). rockWater is the low end.
const (
	rootOpen   = 1.0  // m, grass and open ground
	rootForest = 2.0  // m, a closed forest
	rockWater  = 0.05 // plant-available water of weathered rock, m a metre
)

// RootDepth is how deep, in metres, the roots of the cover reach on ground
// whose dryness index - what the air could take up over the rain - is phi.
func RootDepth(phi float64) float64 {
	return rootOpen + (rootForest-rootOpen)*clamp01(2-phi)
}

// RootDepthOn is RootDepth on ground of soil metres holding paw of its
// volume as water the roots can take, where the year's dry season needs need
// mm of store (DryNeed): the woody share of the cover reaches as deep as
// Reach takes it.
func RootDepthOn(phi, soil, paw, need float64) float64 {
	return rootOpen + (Reach(soil, paw, rootForest, need)-rootOpen)*clamp01(2-phi)
}

// Hold is the bucket a ground holds, in mm, with soil metres of soil over
// the rock that holds paw of its volume as water the roots can take, under a
// cover whose roots reach root metres.
func Hold(soil, paw, root float64) float64 {
	soil = math.Max(0, soil)
	return 1000 * (paw*math.Min(soil, root) + rockWater*math.Max(0, root-soil))
}

// The roots a dry season grows.
//
// A woody cover's roots do not stop where its height says they would. Where
// the year has a dry season the trees and shrubs that live through it reach
// as deep as the water that carries them through it lies: the root zone a
// catchment's cover keeps is the store its driest season draws down, and no
// more (Gao and others, 2014, who read it off 300 catchments' water
// balances; Wang-Erlandsson and others, 2016, off the world's evaporation;
// Kleidon and Heimann, 1998). It is how the seasonal tropics' forests and
// savannas keep transpiring months into the dry season (Nepstad and others,
// 1994: Amazonian roots past eight metres). A cover sized only by its
// height - a metre or two - holds a hundred millimetres or so, sheds the
// rest of a monsoon's wet season to the rivers, and has nothing to give the
// air or its own leaves through the dry season.
//
// So the woody cover's roots reach at least as deep as the bucket its dry
// season needs (DryNeed), down to rootDeepest: Canadell and others (1996)
// find trees' deepest roots at seven metres on the mean of the world's
// biomes, shrubs' at five. Below the soil the roots take the weathered
// rock's water (rockWater).
const rootDeepest = 7.0

// DryNeed is the store, in mm, a cover has to draw down to carry itself
// through the dry part of a year with rain mm falling in each phase and the
// air able to take up pet mm in each: the deepest the year's running
// deficit of what the cover gives the air under the rain goes, phase after
// phase in the year's order (Gao and others, 2014, the memory method). What
// the cover gives the air is its share of the year's water: pet in each
// phase, scaled so that the year's is no more than the year's rain.
func DryNeed(rain, pet *[Phases]float64) float64 {
	var p, e float64
	for k := range Phases {
		p += math.Max(0, rain[k])
		e += math.Max(0, pet[k])
	}
	if p <= 0 || e <= 0 {
		return 0
	}
	use := math.Min(1, p/e)
	// Twice round the year, so that a dry season across the year's turn is
	// counted whole.
	var short, most float64
	for range 2 {
		for _, k := range yearOrder {
			short = math.Max(0, short+use*math.Max(0, pet[k])-math.Max(0, rain[k]))
			most = math.Max(most, short)
		}
	}
	return most
}

// Reach is how deep, in metres, a woody cover whose roots would reach root
// metres by its height reaches on ground of soil metres holding paw of its
// volume as water the roots can take, where its dry season needs need mm of
// store: as deep as Hold gives need, and no deeper than rootDeepest nor
// shallower than root.
func Reach(soil, paw, root, need float64) float64 {
	if root >= rootDeepest || Hold(soil, paw, root) >= need {
		return root
	}
	soil = math.Max(0, soil)
	// Hold grows by paw a metre in the soil and rockWater a metre under it.
	z := root
	if z < soil {
		if paw > 0 && need <= 1000*paw*soil {
			return math.Min(rootDeepest, math.Max(root, need/(1000*paw)))
		}
		z = soil
	}
	z += (need - Hold(soil, paw, z)) / (1000 * rockWater)
	return math.Min(rootDeepest, math.Max(root, z))
}
