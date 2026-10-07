package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
	"github.com/LukasSelin/terra/internal/atmos"
	"github.com/LukasSelin/terra/internal/phase"
	"github.com/LukasSelin/terra/internal/veg"
)

// What grows on the land, as a state.
//
// Every tile carries the share of its ground each plant functional type
// covers, the carbon each holds there and the leaf area each puts up, and
// they are what the land grows, not a label it was given: established,
// growing and dying through the years by the year's warmth, its light, its
// snow and the water the soil's bucket holds through the seasons, and by one
// another. See package veg for how; this is where a tile's year is read for
// it and what it leaves is kept.
//
// The game's ground is read off it. Under the climate's rules a wood stands
// where the trees' canopy closes over the ground (see WoodsAt), and dies back
// where the ground stops suiting it (see Erode); the soil's bucket is as deep
// as what stands on it lets its roots reach (see rootOf); and a tile's biome
// is what stands on it (see BiomeAt). The vegetation is the land's own, as a
// wood a settlement felled is still the ground's to grow back: a Field
// carries the vegetation its ground would hold.
//
// It burns. A year has its fires, lit by the lightning and run through the
// grass and the litter as far as their dryness and the wind take them, and
// they kill the trees as their bark lets them; a drought kills, and the
// storms throw trees down (see package veg's fires). Over a band of rain a
// savanna the fires hold open and a forest that shades the grass out are
// each a state that holds itself, and which a place has is its history's:
// the land is laid as the last glacial's drier centuries left it, and run on
// under today's year (see glacialRain).
//
// It is today's. A history's epochs are rained on with the roots the dryness
// gives (atmos.RootDepth): plants come to deep time with the carbon cycle
// that needs them (docs/earth-system-plan.md, owner decision 3).

// PFT is a plant functional type: see package veg.
type PFT = veg.PFT

// The plant functional types.
const (
	TropicalEvergreen   = veg.TropicalEvergreen
	TropicalRaingreen   = veg.TropicalRaingreen
	TemperateBroadleaf  = veg.TemperateBroadleaf
	TemperateNeedleleaf = veg.TemperateNeedleleaf
	BorealNeedleleaf    = veg.BorealNeedleleaf
	C3Grass             = veg.C3Grass
	C4Grass             = veg.C4Grass
	Shrub               = veg.Shrub
	Tundra              = veg.Tundra
	PFTs                = veg.PFTs
)

// PFTName is what a plant functional type is called.
func PFTName(p PFT) string { return veg.Kinds[p].Name }

// How the state is kept: a byte for each type's cover, in 255ths of the
// tile; two for its carbon, in grams a square metre of the tile, to 65 kg;
// and a byte for its leaf area over its own cover, in twentieths, to 12.75.
// Thirty-six bytes a tile, and two more for the share of it its fires burn
// a year (see Burned).
const (
	coverStep = 1.0 / 255
	massStep  = 1e-3 // kg
	leafStep  = 0.05
)

// vegSpin is how many years the types compete for from BIOME4's answer
// before the state is read as the climate's steady one. LPJ spins up for a
// thousand; most of a tile's change is over in its trees' first century, and
// what is left is types of near the same worth trading the last tenths of
// their ground.
const vegSpin = 300

// parShare is the share of the sun at the top of the air that reaches the
// ground as the light plants grow by: the surface takes some 0.55 of it
// through the clouds and the air over the year (Wild and others, 2013: 185 of
// 340 W/m²), and photosynthetically active radiation is 0.48 of that
// (McCree, 1972; Frouin and Pinker, 1995).
const parShare = 0.55 * 0.48

// Roots. A herb's and a grass's roots reach a metre, and a tree's or a
// shrub's two: Schenk and Jackson (2002); see atmos.RootDepth.
const (
	rootHerb = 1.0
	rootWood = 2.0
)

// wetHollow bounds the topographic wetness the vegetation's water is read
// with: a hollow the water gathers in gives its plants up to twice the
// soil's share of what the air could take, and a crest it runs off down to
// half. See twi, and the gallery forests of a dry country.
const wetHollow = 2.0

// vegLaid reports whether the vegetation has been laid on g.
func (g *Grid) vegLaid() bool { return len(g.vegCover) == len(g.Tiles)*int(PFTs) && len(g.Tiles) > 0 }

// Cover is the share of tile i's ground under plant functional type p's
// leaves, its foliage-projective cover, from nothing to one. The trees'
// together are under 0.95; the rest share what the trees leave.
func (g *Grid) Cover(i int, p PFT) float64 {
	if !g.vegLaid() || i < 0 || i >= len(g.Tiles) {
		return 0
	}
	return float64(g.vegCover[i*int(PFTs)+int(p)]) * coverStep
}

// Biomass is the carbon plant functional type p holds on tile i, in kg a
// square metre of the tile.
func (g *Grid) Biomass(i int, p PFT) float64 {
	if !g.vegLaid() || i < 0 || i >= len(g.Tiles) {
		return 0
	}
	return float64(g.vegMass[i*int(PFTs)+int(p)]) * massStep
}

// LeafArea is the leaf area p puts up over its own cover on tile i, square
// metres of leaf a square metre: the leaf area that does best by it in the
// tile's year (see veg.Year.Potential).
func (g *Grid) LeafArea(i int, p PFT) float64 {
	if !g.vegLaid() || i < 0 || i >= len(g.Tiles) {
		return 0
	}
	return float64(g.vegLeaf[i*int(PFTs)+int(p)]) * leafStep
}

// LAI is tile i's leaf area index: the leaf area of everything on it over
// the whole of its ground, which is what a satellite reads.
func (g *Grid) LAI(i int) float64 {
	var l float64
	for p := range PFTs {
		l += g.Cover(i, p) * g.LeafArea(i, p)
	}
	return l
}

// VegCarbon is the carbon everything on tile i holds, in kg a square metre.
func (g *Grid) VegCarbon(i int) float64 {
	var c float64
	for p := range PFTs {
		c += g.Biomass(i, p)
	}
	return c
}

// TreeCover is the share of tile i's ground under trees.
func (g *Grid) TreeCover(i int) float64 {
	var c float64
	for p := range PFTs {
		if veg.Kinds[p].Tree {
			c += g.Cover(i, p)
		}
	}
	return c
}

// stateOf is tile i's state as kept.
func (g *Grid) stateOf(i int) veg.State {
	var s veg.State
	for p := range PFTs {
		s.Cover[p] = g.Cover(i, p)
		s.Mass[p] = g.Biomass(i, p)
	}
	return s
}

// keep writes tile i's state and its types' leaf area down.
func (g *Grid) keep(i int, s *veg.State, pot *[PFTs]veg.Potential) {
	at := i * int(PFTs)
	for p := range PFTs {
		c := s.Cover[p]
		if c < coverStep/2 {
			c = 0
		}
		g.vegCover[at+int(p)] = uint8(math.Round(math.Min(1, c) / coverStep))
		g.vegMass[at+int(p)] = uint16(math.Round(math.Min(65.535, s.Mass[p]) / massStep))
		l := 0.0
		if c > 0 {
			l = pot[p].LAI
		}
		g.vegLeaf[at+int(p)] = uint8(math.Round(math.Min(12.75, l) / leafStep))
	}
}

// rootOf is how deep, in metres, the roots on tile i reach: a woody plant's
// under the trees and the shrubs, a herb's under the rest and on bare ground.
// It is the dryness's reading (atmos.RootDepth) where nothing has been laid,
// as through a history.
func (g *Grid) rootOf(i int, phi float64) float64 {
	if !g.vegLaid() {
		return atmos.RootDepth(phi)
	}
	var woody float64
	at := i * int(PFTs)
	for p := range PFTs {
		if veg.Kinds[p].Woody {
			woody += float64(g.vegCover[at+int(p)]) * coverStep
		}
	}
	return rootHerb + (rootWood-rootHerb)*clamp01(woody)
}

// soilYear is tile i's year of water as the air last read it - each phase's
// rain and what the air could take up in it, and the year's mean and swing
// its snow is read on - for a soil's bucket to be run through. ok is false
// under the water and where the weather has not been read.
func (g *Grid) soilYear(i int, sea, land float64) (rain, take [atmos.Phases]float64, mean, swing, pe float64, ok bool) {
	if g.sunk(i) || len(g.rainIn) != len(g.Tiles)*atmos.Phases || g.air == nil {
		return rain, take, 0, 0, 0, false
	}
	y := i / g.W
	a := g.air
	t := a.Mean[y] - Lapse*g.lapseHeight(i)
	pe = atmos.PetAt(a.PET[y], t, g.yearCont(i)) * float64(g.dayRange[i])
	cell := -1
	if g.winds != nil {
		cell = g.winds.Env.CellOfTile(i)
	}
	var shares float64
	if cell >= 0 && len(g.petShare[0]) > cell {
		for k := range atmos.Phases {
			shares += g.petShare[k][cell]
		}
	}
	for k := range atmos.Phases {
		rain[k] = g.RainIn(i, k)
		take[k] = pe / atmos.Phases
		if shares > 0 {
			take[k] *= g.petShare[k][cell] * atmos.Phases / shares
		}
	}
	mean, swing = g.snowYearOn(i, t, sea, land)
	return rain, take, mean, swing, pe, true
}

// rewater runs every tile's bucket through its year again with the roots
// what stands on it gives them, and writes down what it holds and sheds:
// the soil's water and the snow's as rainOn writes them, without reading the
// air again.
func (g *Grid) rewater() {
	defer phase.Start("rewater")()
	if len(g.soilWater) != len(g.Tiles)*atmos.Phases {
		return
	}
	g.soilBucket()
	g.EachRow(func(y int) {
		sea, land := g.snowSwings(y)
		for i := y * g.W; i < (y+1)*g.W; i++ {
			rain, take, mean, swing, pe, ok := g.soilYear(i, sea, land)
			if !ok {
				continue
			}
			p := g.Rain(i)
			hold := atmos.Hold(float64(g.Soil[i]), float64(g.paw[i]), g.rootOf(i, pe/math.Max(p, 1e-9)))
			b := atmos.BucketCold(hold, &rain, &take, mean, swing)
			// The phases' rain is kept in float32 (rainIn), and where the air
			// takes nothing back their sum can come over the year's rain by a
			// rounding: what runs off is never more than fell.
			g.runoff[i], g.soilHold[i], g.ice[i] = math.Min(b.Shed(), p), float32(hold), float32(b.Ice)
			at := i * atmos.Phases
			for k := range atmos.Phases {
				g.soilWater[at+k], g.runoffIn[at+k] = float32(b.Water[k]), float32(b.Runoff[k])
				g.snowWater[at+k], g.snowCover[at+k], g.meltIn[at+k] = float32(b.Snow[k]), float32(b.Cover[k]), float32(b.Melt[k])
			}
		}
	})
}

// vegClimate is tile i's year as the plants read it (see veg.Climate), and
// false where nothing grows: under the water, or where the weather has not
// been read. sun is the row's light by phase, twiMean the land's mean
// wetness index, and throw the storms' throw over the air's cells (nil
// where there are none). wet is the share of its rain the year has: one for
// today's, less for a drier past's.
func (g *Grid) vegClimate(i int, sea, land float64, sun *[atmos.Phases]float64, twiMean float64, throw []float64, wet float64) (veg.Climate, bool) {
	var c veg.Climate
	rain, take, mean, swing, _, ok := g.soilYear(i, sea, land)
	if !ok {
		return c, false
	}
	for k := range atmos.Phases {
		rain[k] *= wet
	}
	c.Mean, c.Swing, c.Sun, c.Take, c.Rain = mean, swing, *sun, take, rain
	if g.winds != nil {
		cell := g.winds.Env.CellOfTile(i)
		for k := range atmos.Phases {
			if len(g.winds.U[k]) > cell {
				c.Wind[k] = math.Hypot(float64(g.winds.U[k][cell]), float64(g.winds.V[k][cell]))
			}
		}
		if len(throw) > cell {
			c.Throw = throw[cell]
		}
	}
	p := g.PosOf(i)
	c.Bare = g.Barren(p)
	c.Treeless = g.Treeless(p) || g.Slope(p) > soilCritical
	// A hollow the water gathers in is wetter than the soil's own bucket, and
	// a crest drier, by its wetness index against the land's: see twi.
	hollow := 1.0
	if twiMean > 0 {
		hollow = math.Max(1/wetHollow, math.Min(wetHollow, g.twi(i)/twiMean))
	}
	soil, paw := float64(g.Soil[i]), float64(g.paw[i])
	for _, lot := range [2]struct {
		root float64
		into *[atmos.Phases]float64
	}{{rootHerb, &c.Herb}, {rootWood, &c.Wood}} {
		b := atmos.BucketCold(atmos.Hold(soil, paw, lot.root), &rain, &take, mean, swing)
		for k := range atmos.Phases {
			lot.into[k] = 1
			if take[k] > 0 {
				lot.into[k] = math.Min(1, hollow*b.Evap[k]/take[k])
			}
		}
	}
	for k := range atmos.Phases {
		c.Snow[k] = g.SnowCover(i, k)
	}
	return c, true
}

// landTwiMean is the mean wetness index of the dry land (see twi).
func (g *Grid) landTwiMean() float64 {
	twis := make([]float64, len(g.Tiles))
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			if !g.Tiles[i].Wet() && !g.Tiles[i].Terrain.Tidal() && !g.sunk(i) {
				twis[i] = g.twi(i)
			}
		}
	})
	var sum, n float64
	for i, v := range twis {
		if !g.Tiles[i].Wet() && !g.Tiles[i].Terrain.Tidal() && !g.sunk(i) {
			sum, n = sum+v, n+1
		}
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

// rowSun is the light plants grow by on row y through each phase, in MJ/m²
// over the phase.
func (g *Grid) rowSun(y int) (sun [atmos.Phases]float64) {
	top := atmos.PhaseSun(g.air.Forcing, g.air.Lat[y])
	for k := range atmos.Phases {
		sun[k] = top[k] * parShare * secondsPerYear / atmos.Phases / 1e6
	}
	return sun
}

// growVegetation runs every tile's vegetation on through years of its year
// as the air last read it, with its fires and its storms: from its history
// to the climate's steady state where nothing has been laid (veg.Spin, years
// at the most, see glacialRain), and from what stands where it has.
func (g *Grid) growVegetation(years int) {
	defer phase.Start("growVegetation")()
	if len(g.rainIn) != len(g.Tiles)*atmos.Phases || g.air == nil {
		return
	}
	fresh := !g.vegLaid()
	if fresh {
		n := len(g.Tiles) * int(PFTs)
		g.vegCover, g.vegMass, g.vegLeaf = make([]uint8, n), make([]uint16, n), make([]uint8, n)
	}
	if len(g.burned) != len(g.Tiles) {
		g.burned = make([]uint16, len(g.Tiles))
	}
	g.soilBucket()
	twiMean := g.landTwiMean()
	var throw []float64
	if g.winds != nil {
		throw = g.winds.Env.Throw()
	}
	g.EachRow(func(y int) {
		sea, land := g.snowSwings(y)
		sun := g.rowSun(y)
		for i := y * g.W; i < (y+1)*g.W; i++ {
			c, ok := g.vegClimate(i, sea, land, &sun, twiMean, throw, 1)
			if !ok {
				clear(g.vegCover[i*int(PFTs) : (i+1)*int(PFTs)])
				clear(g.vegMass[i*int(PFTs) : (i+1)*int(PFTs)])
				clear(g.vegLeaf[i*int(PFTs) : (i+1)*int(PFTs)])
				g.burned[i] = 0
				continue
			}
			yr := veg.Read(&c)
			pot, fire := potentials(&yr)
			var s veg.State
			if fresh {
				// The history: BIOME4's answer under the glacial's drier
				// year, run to the state the glacial's fires held it at,
				// and run on from there under today's.
				was, _ := g.vegClimate(i, sea, land, &sun, twiMean, throw, glacialRain)
				wyr := veg.Read(&was)
				wpot, wfire := potentials(&wyr)
				s = veg.Equilibrium(&wpot)
				veg.Spin(&s, &wpot, &wfire, years)
				veg.Spin(&s, &pot, &fire, years)
			} else {
				s = g.stateOf(i)
				veg.Grow(&s, &pot, &fire, years)
			}
			g.keep(i, &s, &pot)
			g.burned[i] = uint16(math.Round(veg.Burned(&s, &pot, &fire) / burnedStep))
		}
	})
}

// potentials is what each type would make of a year, and the year's fires
// and storms.
func potentials(yr *veg.Year) (pot [PFTs]veg.Potential, fire veg.Fire) {
	for p := range PFTs {
		pot[p] = yr.Potential(p)
	}
	return pot, yr.Fire()
}

// glacialRain is the share of today's rain the land's history leaves it
// with: the vegetation laid on a new map is the state it came to through the
// last glacial's drier centuries, run on under today's year. Which of a
// savanna and a forest a place in the band of rain that holds either has is
// its history's: a savanna whose fires held it open through the glacial
// holds itself open still, and a forest that closed over the grass keeps
// it shaded out (Staver and others, 2011). The tropics' land was some
// fifth to a third drier at the last glacial maximum than today, and the
// savannas wider (Bartlein and others, 2011; Anhuf and others, 2006): three
// quarters of the rain.
const glacialRain = 0.75

// burnedStep is what Burned is kept in: 65535ths.
const burnedStep = 1.0 / 65535

// Burned is the share of tile i's ground its fires burn in a year, as the
// vegetation on it last ran: see package veg's fires.
func (g *Grid) Burned(i int) float64 {
	if i < 0 || i >= len(g.burned) {
		return 0
	}
	return float64(g.burned[i]) * burnedStep
}

// layVegetation lays the land's vegetation at its steady state under
// today's climate, and runs the soil's water again under the roots it has.
// The rivers are worked out again on what that sheds.
func (g *Grid) layVegetation() {
	g.growVegetation(vegSpin)
	g.rewater()
	g.pool()
	g.flow()
}

// dieBack is the share of a wood's ground under trees below which it is no
// longer a wood: where its trees have died back to less than this, the
// ground is open. Under the climate's rules a wood is laid where the canopy
// covers half the ground or so (see climateLine); a fifth is the open
// woodland a savanna is.
const dieBack = 0.2

// dieBackWoods turns the woods whose trees have died back to open ground:
// see dieBack. It is the climate's rules', and only theirs: under the tuned
// rules a standing wood is a fact about the map. It reports how many died.
func (g *Grid) dieBackWoods() int {
	if !g.climateWoods || !g.vegLaid() {
		return 0
	}
	n := 0
	for i := range g.Tiles {
		if g.Tiles[i].Terrain != Forest || g.TreeCover(i) >= dieBack {
			continue
		}
		g.Turn(g.PosOf(i), Grass)
		g.Wood[i], g.Wild[i] = 0, 0
		g.Sow(i)
		n++
	}
	return n
}

// broadleaf is the share of tile i's trees' carbon that is broadleaf, and
// false where no tree stands on it or nothing has been laid.
func (g *Grid) broadleaf(i int) (float64, bool) {
	var broad, all float64
	for p := range PFTs {
		if !veg.Kinds[p].Tree {
			continue
		}
		m := g.Biomass(i, p)
		all += m
		if p == TropicalEvergreen || p == TropicalRaingreen || p == TemperateBroadleaf {
			broad += m
		}
	}
	if all <= 0 {
		return 0, false
	}
	return broad / all, true
}

// woodsOn is WoodsAt under the climate's rules on a map whose vegetation has
// been laid: how much of the canopy's room its trees take.
func (g *Grid) woodsOn(p geom.Pos) float64 {
	return clamp01(g.TreeCover(g.Index(p)) / 0.95)
}
