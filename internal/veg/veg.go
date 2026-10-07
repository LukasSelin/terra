// Package veg is what grows on the land, as a state: plant functional types
// with the share of the ground each covers, the carbon each holds, and the
// leaf area each puts up, established, growing and dying by the climate, the
// water and one another.
//
// Woods were a Forest or a Grass label read off the dryness, placed once and
// never dying back, and a biome was a name for a Köppen code. Here the land
// grows what its year lets it. It is done the way the models the climate's
// vegetation is mapped with do it, cut down to what a tile's year as the air
// reads it carries:
//
//   - Each type has bioclimatic limits: the coldest month it survives, the
//     warmest winter it establishes under, the growing degree-days it needs,
//     the warmest summer the tundra's plants still win in. They are LPJ's
//     (Sitch and others, 2003, table 3), where LPJ has the type.
//   - Each type that may grow is given the leaf area that does best by it:
//     what it gains in light against what its leaves cost to keep and to
//     replace, where the water its roots reach holds the leaves it can keep
//     open. That is BIOME4's step (Kaplan and others, 2003): leaf area
//     optimised for the year's productivity, a type's productivity is what
//     it is ranked by, and the water is what bounds a dry country's leaf
//     area (Woodward, 1987).
//   - The types then compete through the years: the trees for the canopy,
//     the grasses, the shrubs and the tundra's plants for the open ground
//     under it, establishing where there is room, growing by what they make,
//     dying of age, of starving and of a winter too cold, and losing ground
//     to whatever does better on it. That is LPJ's (Sitch and others, 2003)
//     establishment, mortality and competition for space, with a year as its
//     step. It is run from BIOME4's answer to a steady state, so that a wood
//     whose ground stops suiting it dies back.
//
// The year a type reads is the air's four phases: the sun through each, the
// temperature of each fortnight of it, the share of the ground its snow
// covers, and how much of what the air could take up the soil's bucket gives
// it (atmos.BucketCold), with the bucket as deep as a herb's roots or a
// woody plant's.
package veg

import (
	"math"

	"github.com/LukasSelin/terra/internal/atmos"
)

// Phases is the air's phases of the year: see atmos.Phases.
const Phases = atmos.Phases

// A PFT is a plant functional type.
type PFT uint8

// The plant functional types.
const (
	TropicalEvergreen PFT = iota
	TropicalRaingreen
	TemperateBroadleaf
	TemperateNeedleleaf
	BorealNeedleleaf
	C3Grass
	C4Grass
	Shrub
	Tundra
	PFTs
)

// phenology is when a type is in leaf.
type phenology uint8

const (
	evergreen   phenology = iota // the year round
	summergreen                  // the phases warm enough to grow in
	raingreen                    // the phases wet enough to grow in
)

// Kind is what a plant functional type is.
type Kind struct {
	Name string
	// Tree is a type of the canopy, which shades whatever is under it; the
	// rest share the open ground the trees leave. Woody is a tree or a shrub,
	// whose roots reach as deep as a woody plant's (see Climate.Wood).
	Tree, Woody bool
	// The bioclimatic limits, in degrees and degree-days over five: the
	// coldest month a type survives (coldDies), the coldest months it
	// establishes between (coldFrom, coldTo), the growing degree-days it needs
	// to establish (gdd), and the warmest month it establishes under (warmTo).
	coldDies, coldFrom, coldTo, gdd, warmTo float64
	// dry is the least of what the air could take up in a year the soil
	// under the type's roots must give it for the type to establish, its
	// evaporation over its potential, Priestley and Taylor's α: BIOME1's
	// limits (Prentice and others, 1992, table 1), which BIOME3 and BIOME4
	// keep in their drought tolerances. A type survives down to dryKeep of it.
	dry float64
	// leafWarm is the least a phase's mean may be for a summergreen type to
	// be in leaf through it.
	leafWarm float64
	// temp is the photosynthesis's temperatures: none under the first or over
	// the fourth, all of it between the second and the third, and straight
	// lines between. They are LPJ's table 2.
	temp [4]float64
	// lue is the carbon fixed for each MJ of light the leaves take up at the
	// best temperature, in grams, and wue how many times a C3 plant's carbon
	// the type fixes for the water it transpires: a C4 grass's pathway, a
	// desert shrub's thick leaves and shut stomata.
	lue, wue float64
	phen     phenology
	// laiMost is the most leaf area the type puts up whatever the year.
	laiMost float64
	// leafKeep is what a unit of leaf area costs to keep in leaf for a year at
	// ten degrees, in grams of carbon - the leaves' maintenance respiration and
	// the fine roots' that supply them - and leafTurn what it costs to replace
	// a year, out of what the plant makes. woodKeep is what the stems of a
	// square metre of the type's cover cost to keep for a year at ten degrees:
	// the sapwood's respiration. All three are LPJ's in effect: its tissue
	// respiration and turnover over its allometry, read per unit of leaf area
	// and of cover.
	leafKeep, leafTurn, woodKeep float64
	// residence is how many years the carbon the type makes stays in it: the
	// biomass over the productivity.
	residence float64
	// seed is how many kilograms of carbon a square metre of new cover takes,
	// and mortality how much of its cover the type loses a year to age and
	// accident.
	seed, mortality float64
}

// Kinds is every plant functional type, by PFT.
var Kinds = [PFTs]Kind{
	TropicalEvergreen: {
		Name: "tropical evergreen", Tree: true, Woody: true,
		dry: 0.8, leafWarm: 5,
		coldDies: 15.5, coldFrom: 15.5, coldTo: inf, warmTo: inf,
		temp: [4]float64{2, 25, 30, 55}, lue: lueC3, wue: 1, phen: evergreen,
		laiMost: 7, leafKeep: 22, leafTurn: 35, woodKeep: 190,
		residence: 15, seed: 2, mortality: 0.02,
	},
	TropicalRaingreen: {
		Name: "tropical raingreen", Tree: true, Woody: true,
		dry: 0.45, leafWarm: 5,
		coldDies: 15.5, coldFrom: 15.5, coldTo: inf, warmTo: inf,
		temp: [4]float64{2, 25, 30, 55}, lue: lueC3, wue: 1, phen: raingreen,
		laiMost: 6, leafKeep: 22, leafTurn: 70, woodKeep: 190,
		residence: 12, seed: 2, mortality: 0.025,
	},
	TemperateBroadleaf: {
		Name: "temperate broadleaf", Tree: true, Woody: true,
		dry: 0.65, leafWarm: 5,
		coldDies: -17, coldFrom: -17, coldTo: 18.8, gdd: 1200, warmTo: inf,
		temp: [4]float64{-4, 20, 30, 42}, lue: lueC3, wue: 1, phen: summergreen,
		laiMost: 6, leafKeep: 22, leafTurn: 40, woodKeep: 170,
		residence: 15, seed: 2, mortality: 0.02,
	},
	TemperateNeedleleaf: {
		Name: "temperate needleleaf", Tree: true, Woody: true,
		dry: 0.65, leafWarm: 5,
		coldDies: -2, coldFrom: -2, coldTo: 22, gdd: 900, warmTo: inf,
		temp: [4]float64{-4, 20, 30, 42}, lue: lueC3, wue: 1, phen: evergreen,
		laiMost: 6, leafKeep: 22, leafTurn: 50, woodKeep: 190,
		residence: 18, seed: 2, mortality: 0.015,
	},
	BorealNeedleleaf: {
		Name: "boreal needleleaf", Tree: true, Woody: true,
		dry: 0.65, leafWarm: 5,
		coldDies: -inf, coldFrom: -inf, coldTo: -2, gdd: 350, warmTo: inf,
		temp: [4]float64{-4, 15, 25, 38}, lue: lueC3, wue: 1, phen: evergreen,
		laiMost: 5, leafKeep: 22, leafTurn: 45, woodKeep: 220,
		residence: 18, seed: 2, mortality: 0.015,
	},
	C3Grass: {
		Name: "C3 grass",
		dry:  0.33, leafWarm: 5,
		coldDies: -inf, coldFrom: -inf, coldTo: 15.5, gdd: 300, warmTo: inf,
		temp: [4]float64{-4, 10, 30, 45}, lue: lueC3, wue: 1, phen: raingreen,
		laiMost: 4, leafKeep: 22, leafTurn: 80, woodKeep: 0,
		residence: 1.5, seed: 0.1, mortality: 0.1,
	},
	C4Grass: {
		Name: "C4 grass",
		dry:  0.18, leafWarm: 5,
		coldDies: 15.5, coldFrom: 15.5, coldTo: inf, warmTo: inf,
		temp: [4]float64{6, 20, 45, 55}, lue: lueC4, wue: 1.5, phen: raingreen,
		laiMost: 4, leafKeep: 22, leafTurn: 80, woodKeep: 0,
		residence: 1.5, seed: 0.1, mortality: 0.1,
	},
	Shrub: {
		Name: "shrub", Woody: true,
		dry: 0, leafWarm: 5,
		coldDies: -inf, coldFrom: -inf, coldTo: inf, gdd: 500, warmTo: inf,
		temp: [4]float64{-2, 20, 35, 50}, lue: lueC3, wue: 1.2, phen: evergreen,
		laiMost: 2.5, leafKeep: 22, leafTurn: 50, woodKeep: 50,
		residence: 5, seed: 0.5, mortality: 0.05,
	},
	Tundra: {
		Name: "tundra",
		dry:  0.33, leafWarm: 0,
		coldDies: -inf, coldFrom: -inf, coldTo: inf, warmTo: 15,
		temp: [4]float64{-4, 8, 18, 30}, lue: lueC3, wue: 1, phen: summergreen,
		laiMost: 1.5, leafKeep: 22, leafTurn: 40, woodKeep: 0,
		residence: 5, seed: 0.2, mortality: 0.06,
	},
}

var inf = math.Inf(1)

// The light-use efficiencies, grams of carbon fixed for each MJ of light
// the leaves take up at the best temperature, unstressed: the C3 pathway's,
// and the C4's, which wastes none of its light to photorespiration in the
// heat. Monteith's (1977) efficiency, as MODIS's productivity reads it over
// the biomes (Running and others, 2004), a gram to a gram and a half.
const (
	lueC3 = 0.75
	lueC4 = 1.0
)

// growthCost is the share of what a plant fixes, less what its tissue costs
// to keep, spent on making new tissue: LPJ's growth respiration.
const growthCost = 0.25

// extinction is the light a canopy takes up for its leaf area, Beer's law's
// k: the share of the light it takes up is 1 - e^(-k·LAI).
const extinction = 0.5

// Climate is a tile's year as the plants read it.
type Climate struct {
	// Mean is the year's mean on the ground, in degrees, and Swing how far
	// either side of it the year swings, signed by hemisphere as
	// atmos.SwingAt is: the north's summer is the third phase.
	Mean, Swing float64
	// Sun is the light plants grow by through each phase, in MJ/m² over the
	// phase.
	Sun [Phases]float64
	// Snow is the share of the ground the snow covers on the mean through
	// each phase, which shades all but the trees.
	Snow [Phases]float64
	// Herb and Wood are how much of what the air could take up in each phase
	// the soil's bucket gives it, its evaporation over its potential, with
	// the bucket as deep as a herb's roots and a woody plant's. One is all
	// of it; it is what bounds a dry country's leaf area.
	Herb, Wood [Phases]float64
	// Take is what the air could take up in each phase, in mm, which the
	// year's α weighs the phases' by.
	Take [Phases]float64
	// Rain is what falls in each phase, in mm, whose showers bring the
	// lightning that lights the fires; Wind is the wind near the ground
	// through each phase, in metres a second, which drives them; and Throw
	// is the share of a canopy the storms blow down in a year. See Fire.
	Rain, Wind [Phases]float64
	Throw      float64
	// Treeless is ground above the tree line (atmos.TreeLineMean) or too
	// steep to hold the soil a tree stands in; Bare is ground nothing grows
	// on, under ice.
	Treeless, Bare bool
}

// Potential is what a type would make of a tile's year.
type Potential struct {
	// Establish says the type may take on the ground, Survive that the
	// ground does not kill what stands on it.
	Establish, Survive bool
	// LAI is the leaf area the type puts up over its own cover, and NPP what
	// a square metre of its cover makes in a year at that leaf area, in kg of
	// carbon. Surplus is NPP less what replacing its leaves takes: what it
	// has to grow by.
	LAI, NPP, Surplus float64
	// Drought is the share of its cover a woody type loses in a year to
	// the drought of a year whose water is under what it establishes on:
	// see droughtMost.
	Drought float64
}

// stepAngle is the place in the year of step j of steps of phase k, as the
// soil's bucket reads it: see atmos.
func stepAngle(k, j, steps int) float64 {
	return float64(k-1)*math.Pi/2 + ((float64(j)+0.5)/float64(steps)-0.5)*math.Pi/2
}

// steps is how many steps a phase's temperature is read at: a fortnight's.
const steps = 6

var stepSin = func() (s [Phases][steps]float64) {
	for k := range Phases {
		for j := range steps {
			s[k][j] = math.Sin(stepAngle(k, j, steps))
		}
	}
	return s
}()

// ramp is the photosynthesis's response to temperature t on the type's four
// temperatures.
func ramp(t float64, c *[4]float64) float64 {
	switch {
	case t <= c[0] || t >= c[3]:
		return 0
	case t < c[1]:
		return (t - c[0]) / (c[1] - c[0])
	case t > c[2]:
		return (c[3] - t) / (c[3] - c[2])
	}
	return 1
}

// q10 is how much faster tissue respires at t degrees than at ten: twice as
// fast for every ten degrees (LPJ's Q10 of two), and no faster past thirty-
// five.
func q10(t float64) float64 { return math.Exp2((math.Min(t, 35) - 10) / 10) }

// The leaf seasons: a summergreen type is in leaf through a phase whose mean
// is over its leafWarm, a raingreen one through a phase whose soil gives the
// air leafWet or more of what it could take and that is not frozen.
const leafWet = 0.3

// dryKeep is how far under the water a type establishes at it survives:
// a wood stands a run of dry years its saplings would not take in.
const dryKeep = 0.85

// yearAlpha is the year's α of the soil's water, the phases' each weighed by
// what the air could take up in it: one where it could take up nothing.
func yearAlpha(water, take *[Phases]float64) float64 {
	var e, pe float64
	for k := range Phases {
		e += water[k] * take[k]
		pe += take[k]
	}
	if pe <= 0 {
		return 1
	}
	return e / pe
}

// Year is a tile's year read once for every type: the phases' temperatures
// and what the climate's limits say.
type Year struct {
	c                Climate
	cold, warm, gdd5 float64
	temp             [Phases][steps]float64
	phase            [Phases]float64
}

// Read reads a climate's year.
func Read(c *Climate) Year {
	y := Year{c: *c}
	d := atmos.MonthPeak * math.Abs(c.Swing)
	y.cold, y.warm = c.Mean-d, c.Mean+d
	y.gdd5 = atmos.DegreeDays(c.Mean, c.Swing, 5)
	for k := range Phases {
		var sum float64
		for j := range steps {
			t := c.Mean + c.Swing*stepSin[k][j]
			y.temp[k][j] = t
			sum += t
		}
		y.phase[k] = sum / steps
	}
	return y
}

// Cold and Warm are the year's coldest and warmest month, in degrees, and
// GDD its growing degree-days over five.
func (y *Year) Cold() float64 { return y.cold }
func (y *Year) Warm() float64 { return y.warm }
func (y *Year) GDD() float64  { return y.gdd5 }

// Potential is what type p would make of the year.
//
// Over a phase k, a canopy of leaf area L fixes a_k·min(F(L), s_k): a_k the
// light it would fix with all of it taken up, at the phase's temperatures and
// under its snow, F(L) = 1 - e^(-L/2) the share it takes up, and s_k the
// most of it the water lets it use - the soil's evaporation over the air's
// potential, times the type's water-use efficiency. A canopy that would take
// up more light than its water lets it use keeps its stomata shut for the
// difference. Each unit of leaf area costs its upkeep, at the temperature,
// through the phases it is in leaf, and its replacement; the stems cost
// theirs. The leaf area is the one that leaves the most to grow by, found
// exactly: what one more unit of leaf area gains falls as the canopy closes
// and as the phases' water runs out one after another, so the best is where
// it falls to what the unit costs.
func (y *Year) Potential(p PFT) Potential {
	k := &Kinds[p]
	c := &y.c
	var pot Potential
	if c.Bare || (k.Tree && c.Treeless) {
		return pot
	}
	water := &c.Herb
	if k.Woody {
		water = &c.Wood
	}
	alpha := yearAlpha(water, &c.Take)
	pot.Survive = y.cold >= k.coldDies && alpha >= dryKeep*k.dry
	pot.Establish = pot.Survive && y.cold >= k.coldFrom && y.cold <= k.coldTo && y.gdd5 >= k.gdd && y.warm <= k.warmTo && alpha >= k.dry
	if k.Woody && k.dry > 0 && alpha < k.dry {
		pot.Drought = droughtMost * math.Min(1, (k.dry-alpha)/((1-dryKeep)*k.dry))
	}
	var a, s [Phases]float64
	var leafQ, woodQ float64 // the year's upkeep factors, leaves while in leaf and stems all year
	for ph := range Phases {
		woodQ += q10(y.phase[ph]) / Phases
		on := true
		switch k.phen {
		case summergreen:
			on = y.phase[ph] > k.leafWarm
		case raingreen:
			on = water[ph] >= leafWet && y.phase[ph] > 0
		}
		if !on {
			continue
		}
		leafQ += q10(y.phase[ph]) / Phases
		var f float64
		for j := range steps {
			f += ramp(y.temp[ph][j], &k.temp)
		}
		light := c.Sun[ph] * f / steps
		if !k.Tree {
			light *= 1 - c.Snow[ph]
		}
		a[ph] = k.lue * light / 1000 // kg C
		s[ph] = water[ph] * k.wue
	}
	leafCost := (1-growthCost)*k.leafKeep*leafQ/1000 + k.leafTurn/1000
	wood := k.woodKeep * woodQ / 1000

	fBest := bestCover(a, s, leafCost, 1-math.Exp(-extinction*k.laiMost))
	if fBest <= 0 {
		return pot
	}
	lai := -math.Log(1-fBest) / extinction
	var gpp float64
	for ph := range Phases {
		gpp += a[ph] * math.Min(fBest, s[ph])
	}
	pot.LAI = lai
	pot.NPP = (1 - growthCost) * (gpp - k.leafKeep*leafQ*lai/1000 - wood)
	pot.Surplus = pot.NPP - k.leafTurn*lai/1000
	return pot
}

// bestCover is the share of the light a canopy takes up that leaves it the
// most to grow by, up to most: the F at which (1-growthCost)·Σ a_k·(1-F)·k
// over the phases whose water is not yet spent, s_k over F, falls to what
// a unit of leaf area costs, cost - the derivative of the gain in leaf area,
// since dF/dL = k·(1-F). The gain's derivative falls with F, so the first
// place it falls to the cost is the best, and nothing where it starts under.
func bestCover(a, s [Phases]float64, cost, most float64) float64 {
	// The phases by the water they give out at, least first.
	order := [Phases]int{0, 1, 2, 3}
	for i := 1; i < Phases; i++ {
		for j := i; j > 0 && s[order[j]] < s[order[j-1]]; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
	var gain float64 // the light of the phases still unspent
	for _, ph := range order {
		if s[ph] > 0 {
			gain += a[ph]
		}
	}
	lo := 0.0
	for _, ph := range order {
		hi := math.Min(s[ph], most)
		if hi > lo {
			if gain <= 0 {
				return lo
			}
			f := 1 - cost/((1-growthCost)*extinction*gain)
			switch {
			case f <= lo:
				return lo
			case f < hi:
				return f
			}
			lo = hi
		}
		if s[ph] > 0 {
			gain -= a[ph]
		}
		if lo >= most {
			return most
		}
	}
	// Past every phase's water the gain is nothing.
	return lo
}

// State is what stands on a tile: each type's share of the ground under its
// leaves (its foliage-projective cover), and the carbon it holds there, in
// kg a square metre of the tile.
type State struct {
	Cover, Mass [PFTs]float64
}

// The canopy's limits. Trees close to treesMost of the ground at the most,
// as LPJ's FPC does; what they leave is the open ground the rest share.
const treesMost = 0.95

// The rates of the years' competition, a year: what of the room left new
// plants take in it where the type may establish (seeding, LPJ's sapling
// establishment, which slows as the ground closes), how fast a type starving
// - making less than its leaves cost - loses its cover, how fast a winter
// colder than it survives kills it, and how fast a type crowded by better
// ones loses ground to them.
const (
	seedTrees = 0.03
	seedOpen  = 0.2
	starving  = 0.3
	killed    = 0.5
	crowding  = 0.3
)

// Equilibrium is BIOME4's answer: in each of the canopy and the open
// ground, the type that makes the most of the year where it may establish,
// at the cover its growth holds against its losses, and the carbon its
// productivity holds against its residence.
func Equilibrium(pot *[PFTs]Potential) State {
	var s State
	trees := settle(&s, pot, true, treesMost)
	settle(&s, pot, false, 1-trees)
	return s
}

// settle puts the best of the canopy's or the open ground's types on room
// of the ground, at its steady cover, and reports the cover.
func settle(s *State, pot *[PFTs]Potential, tree bool, room float64) float64 {
	best := -1
	for p := range PFTs {
		if Kinds[p].Tree == tree && pot[p].Establish && pot[p].Surplus > 0 && (best < 0 || pot[p].Surplus > pot[best].Surplus) {
			best = int(p)
		}
	}
	if best < 0 {
		return 0
	}
	k := &Kinds[best]
	grow := pot[best].Surplus / k.seed
	c := room * math.Max(0, 1-k.mortality/grow)
	s.Cover[best] = c
	s.Mass[best] = c * pot[best].NPP * k.residence
	return c
}

// Grow runs a state through years of the year whose types' potentials are
// pot and whose fires and storms are f. Each year the trees take the canopy,
// and the rest the ground the trees leave: each type grows its cover by what
// it has to grow by into the room left, takes some of what is left by
// seeding where it may establish, and loses cover to age, to starving, to a
// winter it does not survive, to the better types it is crowded by, and to
// the year's fires, drought and storms; and its carbon follows its cover and
// its productivity. The fires are the state's own: what it has standing at
// the year's start is their fuel (see Burned).
//
// A state that has stopped moving - no type's cover changing by stillCover
// of the ground in a year nor its carbon by stillMass - is left where it is:
// the years after would only creep on by less than the land keeps the state
// to (a 255th of the ground, a gram of carbon) every few decades.
func Grow(s *State, pot *[PFTs]Potential, f *Fire, years int) {
	var loss [PFTs]float64
	for range years {
		burned := Burned(s, pot, f)
		trees := grow(s, pot, true, treesMost, 1, burned, f.Throw, &loss)
		grow(s, pot, false, 1-trees, 1, burned, 0, &loss)
	}
}

// Spin is Grow run to the state's steady one: until no type's cover moves by
// stillCover of the ground in a year, for years at the most - its first
// spinYears a year at a time, and then spinStride years to a step, each step
// implicit in what the type loses so that a long one cannot take more than
// there is - and then each
// type's carbon put where its cover, its productivity and its losses hold
// it. The carbon follows the cover by its residence, decades for a tree, and
// left to the years it takes centuries to settle to the gram after the cover
// has; its steady value is had at once.
func Spin(s *State, pot *[PFTs]Potential, f *Fire, years int) {
	const (
		stillCover = 1e-5
		spinYears  = 30 // taken a year at a time, before spinStride at a time
		spinStride = 5
	)
	var loss [PFTs]float64
	for y := 0; y < years; {
		dt := 1
		if y >= spinYears {
			dt = spinStride
		}
		y += dt
		was := s.Cover
		burned := Burned(s, pot, f)
		trees := grow(s, pot, true, treesMost, float64(dt), burned, f.Throw, &loss)
		grow(s, pot, false, 1-trees, float64(dt), burned, 0, &loss)
		still := true
		for p := range PFTs {
			if math.Abs(s.Cover[p]-was[p]) > stillCover*float64(dt) {
				still = false
				break
			}
		}
		if still {
			break
		}
	}
	for p := range PFTs {
		if loss[p] > 0 {
			s.Mass[p] = math.Max(0, s.Cover[p]*pot[p].NPP/loss[p])
		}
	}
}

// grow is one year of the canopy's or the open ground's types on room, and
// reports their cover at its end: dt years of it, taken at once. burned is
// the share of the ground the year's fires burn, and throw the share of the
// cover the storms blow down. It writes into loss the share of each type's
// carbon a year takes.
func grow(s *State, pot *[PFTs]Potential, tree bool, room, dt, burned, throw float64, loss *[PFTs]float64) float64 {
	var held, best float64
	for p := range PFTs {
		if Kinds[p].Tree == tree {
			held += s.Cover[p]
			if s.Cover[p] > 0 {
				best = math.Max(best, pot[p].Surplus)
			}
		}
	}
	free := math.Max(0, room-held)
	young := 1.0 // what the fires leave of the canopy's young
	if tree && burned > 0 {
		young = math.Exp(-trap * burned)
	}
	crowd := 0.0
	if room > 0 {
		crowd = math.Min(1, held/room)
	}
	var total float64
	for p := range PFTs {
		k := &Kinds[p]
		if k.Tree != tree {
			continue
		}
		c, m := s.Cover[p], k.mortality
		pt := &pot[p]
		gain := 0.0
		if pt.Surplus > 0 {
			gain = pt.Surplus / k.seed * c * free / math.Max(room, 1e-9)
			if pt.Establish {
				seed := seedOpen
				if tree {
					seed = seedTrees
				}
				gain += seed * free
			}
			if best > 0 {
				m += crowding * (1 - pt.Surplus/best) * crowd
			}
		} else {
			m += starving
		}
		if !pt.Survive {
			m += killed
		}
		// The fires kill what they burn as its bark lets them, and hold a
		// canopy's young down; the drought and the storms take their share.
		m += burned*(1-fireTraits[p].resist) + throw + pt.Drought
		gain *= young
		next := math.Max(0, (c+dt*gain)/(1+dt*m))
		// The carbon: what the cover makes, less what passes through it, and
		// what dies with the cover lost past the type's own turnover; and
		// what the fires burn of the grass and the shrubs that live through
		// them.
		extra := m - k.mortality
		switch {
		case !k.Woody:
			extra += burned * burnHerb
		case !k.Tree:
			extra += burned * burnShrub
		}
		loss[p] = 1/k.residence + extra
		mass := (s.Mass[p] + dt*next*pt.NPP) / (1 + dt*loss[p])
		s.Cover[p], s.Mass[p] = next, math.Max(0, mass)
		total += next
	}
	// Seeding and growing into the same room in one step can overfill it by
	// a little; the year's newcomers are taken back in proportion, and the
	// carbon stands on the ground that is left.
	if total > room && total > 0 {
		f := room / total
		for p := range PFTs {
			if Kinds[p].Tree == tree {
				s.Cover[p] *= f
			}
		}
		total = room
	}
	return total
}

// Trees is the share of the ground under the canopy's trees, and Woody under
// trees and shrubs.
func (s *State) Trees() float64 {
	var c float64
	for p := range PFTs {
		if Kinds[p].Tree {
			c += s.Cover[p]
		}
	}
	return c
}

func (s *State) Woody() float64 {
	var c float64
	for p := range PFTs {
		if Kinds[p].Woody {
			c += s.Cover[p]
		}
	}
	return c
}
