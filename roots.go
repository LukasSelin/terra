package terra

import (
	"math"

	"github.com/LukasSelin/terra/internal/veg"
)

// What grows holds the ground.
//
// It used to hold it by two figures a terrain: open ground held its soil
// against the water at twenty pascals and slowed its creep to six hundredths
// of bare earth's, and a wood held at twenty-five and three hundredths,
// whatever grew there, in a desert or a rainforest alike. Now open ground and
// woods are held by what the vegetation has standing on them (vegetation.go):
// how much of the ground each plant functional type covers, how much leaf it
// puts up over it, and how much of it the fires have lately burned off.
//
// Three things are read off it.
//
//   - The stress the water has to clear before it takes anything, τc in
//     Istanbulluoglu and Bras's (2005) E = K(τ - τc): bare loam's where
//     nothing grows, and rising with the cover to the full figures the
//     terrains had, open ground's for the herbs and a wood's for the woody
//     plants. Collins and others (2004) and Istanbulluoglu and Bras take
//     the threshold as bare ground's plus the vegetation's, in proportion to
//     the cover; so is it here. See shearAt and criticalFall.
//   - How fast the ground creeps: open ground's, slowed toward a wood's by
//     the woody plants' cover. See hold and holdBare.
//   - The roots' cohesion, which lets a hillside stand steeper than loose
//     ground could before it slides. See rootRise and landslide.
//
// A sparse cover is a weak one, so semi-arid ground, with enough rain to
// run off in a storm and too little to close a cover over the soil, is where
// the water cuts the most ground: Langbein and Schumm's (1958) peak in
// sediment yield, and the drainage density that goes with it (Melton 1957).
//
// Where the vegetation has not been laid it is proxied by the climate: the
// valleys a new map is cut with (stageCut) are cut under the cover the rain
// against the air's demand gives open ground. Not in a history. A history's
// epochs read the ground as they always have, until plants come to deep time
// with the carbon cycle (docs/earth-system-plan.md, owner decision 3): its
// water clears no stress at all (see waterStep), its creep is open ground's
// hold everywhere, and its slides are the making's, which read no roots.

// leafFull is the leaf area, square metres of leaf over each square metre a
// type covers, at which its cover holds the ground as fully as it can: a
// type with less leaf than this over its own ground holds it as the share of
// it its leaf would cover once over. Gyssels and others (2005) find the
// water's detachment of soil falling away with the cover of the canopy and
// the density of the roots under it, and a plant's roots go with its leaf.
const leafFull = 1.0

// openTrees is the most tree cover open ground is held by: a savanna's,
// under half the ground (Staver and others, 2011). The vegetation is the
// ground's own and a felled wood keeps its trees on the books (see
// vegetation.go), so a closed canopy on ground the map calls open is a wood
// somebody felled, or one the founding luck never laid; either way its trees
// are not holding it, and what grows back in the clearing is counted as
// grass.
const openTrees = 0.5

// After a fire. The ground a fire leaves is bare until what grew on it grows
// back: the grass in a year (Wright and Bailey, 1982, on grasslands back to
// their cover in one to three growing seasons) and the litter, the shrubs
// and the trees' undergrowth over four (Shakesby and Doerr, 2006, on the
// erosion after wildfire falling back toward what it was within two to five
// years). A tile that burns a share b of its ground a year has, at a steady
// rate, 1 - exp(-b·T) of it burned within the last T years, and that much of
// its cover is gone.
const (
	herbBack = 1.0 // years
	woodBack = 4.0
)

// The root cohesion, in pascals, a full cover of woody plants and of herbs
// lends the soil at the depth a slide fails at. Schmidt and others (2001)
// measured lateral root cohesion in the Oregon Coast Range of a few
// kilopascals in clear-cuts to some twenty in industrial forest and more in
// old growth; ten is a forest's. A grass's fine roots hold far less, and
// near the surface: a kilopascal.
const (
	woodCohesion = 10e3
	herbCohesion = 1e3
)

// slideDepth is the least depth, in metres, a slide's plane is taken at:
// the one to two metres of soil shallow landslides fail through (Montgomery
// and Dietrich, 1994). Where the soil is deeper the plane is at its foot.
// And soilWeight is the soil's, wet, in newtons a cubic metre.
const (
	slideDepth = 2.0
	soilWeight = 1.9e3 * gravity
)

// rootMost is the most the roots may raise the fall a slope fails at: from
// the bottom of Roering and others' (1999) range of critical gradients,
// 1.2, which is Critical, to its top, 1.35. A slope with its trees on it
// stands at the top of the range, and one cleared of them at the bottom.
// Past that the infinite slope's cohesion term would have a wooded
// hillside stand at sixty degrees, which ground a tile across does not.
const rootMost = 1.35 - Critical

// holdBare is how much of the creep bare earth would give up natural ground
// with nothing growing on it gives up: open grass's. A plough's (a Field's,
// one) is tillage and not the weather. Bare ground loses what roots hold, and
// takes the rain splash leaves would have (Dunne, Malmon and Mudd, 2010); but
// it loses as well what roots and burrows heave downhill, which is most of a
// vegetated hillside's creep (Gabet and others, 2003), and the two are taken
// to cancel. It was twice grass's for a while, and the small globes' cut
// valleys, whose dry ground then crept faster, left their hillslopes' soil a
// tenth thinner (0.223 m to 0.200, under the yardstick's floor of 0.2) and
// the globe's vegetation 4 Gt C lighter. So what the cover does to the creep
// is the woods': a wood's roots hold its ground to half of open ground's.
const holdBare = 0.06

// natural reports whether ground of kind t is held by what grows on it: open
// ground and woods. A field is ploughed, an outcrop is rubble, and the mud
// of a flat and a salt crust hold by what they are; those keep their kinds'
// figures.
func natural(t Terrain) bool { return t == Grass || t == Forest }

// covers is how much of tile i's ground the herbs and the woody plants on it
// hold, each from nothing to one, between them no more than one: each type's
// cover, as much of it as its leaf holds (see leafFull), less what the fires
// have lately bared (see herbBack), with the trees on open ground no more
// than openTrees. ok is false where nothing says: in a history, and on a map
// with neither its vegetation laid nor its rain read.
func (g *Grid) covers(i int) (herb, wood float64, ok bool) {
	if g.deep > 0 {
		return 0, 0, false
	}
	if !g.vegLaid() {
		return g.coverProxy(i)
	}
	var herbs, woods, trees float64
	at := i * int(PFTs)
	for p := range PFTs {
		c := float64(g.vegCover[at+int(p)]) * coverStep
		if c <= 0 {
			continue
		}
		c *= math.Min(1, float64(g.vegLeaf[at+int(p)])*leafStep/leafFull)
		switch k := veg.Kinds[p]; {
		case k.Tree:
			trees += c
		case k.Woody:
			woods += c
		default:
			herbs += c
		}
	}
	if g.Tiles[i].Terrain == Grass && trees > openTrees {
		herbs += trees - openTrees
		trees = openTrees
	}
	woods += trees
	if b := g.Burned(i); b > 0 && herbs+woods > 0 {
		back := (herbs*herbBack + woods*woodBack) / (herbs + woods)
		left := math.Exp(-b * back)
		herbs, woods = herbs*left, woods*left
	}
	if s := herbs + woods; s > 1 {
		herbs, woods = herbs/s, woods/s
	}
	return herbs, woods, true
}

// The climate's proxy for the cover. UNEP's aridity index, the year's rain
// over what the air could take up, is under 0.05 in a hyper-arid desert and
// over 0.65 in humid ground (Middleton and Thomas, 1997); open ground's
// cover is taken as rising from nothing at the one to whole at the other.
const (
	proxyBare = 0.05
	proxyFull = 0.65
)

// coverProxy is covers where the vegetation has not been laid: herbs, by the
// aridity index (see proxyBare), and no woody plants, since no wood has
// been laid yet either. ok is false where the rain has not been read.
func (g *Grid) coverProxy(i int) (herb, wood float64, ok bool) {
	if len(g.rain) != len(g.Tiles) || g.air == nil || g.sunk(i) {
		return 0, 0, false
	}
	pe := g.pet(i)
	if pe <= 0 {
		return 1, 0, true
	}
	return clamp01((g.rain[i]/pe - proxyBare) / (proxyFull - proxyBare)), 0, true
}

// shearAt is the critical shear stress, in pascals, the water has to put on
// tile i in a flood before it takes any of it: bare loam's, a field's, and
// with the herbs' and the woody plants' cover over it, up to open ground's
// and a wood's. See covers, and Terrain.Shear for the figures.
func (g *Grid) shearAt(i int) float64 {
	t := g.Tiles[i].Terrain
	if !natural(t) {
		return t.Shear()
	}
	herb, wood, ok := g.covers(i)
	if !ok {
		return t.Shear()
	}
	bare := Field.Shear()
	return bare + (Grass.Shear()-bare)*herb + (Forest.Shear()-bare)*wood
}

// holdAt is how much of the creep bare earth would give up tile i's ground
// gives up, by what grows on it alone: holdBare where nothing does, and open
// ground's and a wood's under a full cover of herbs and of woody plants.
func (g *Grid) holdAt(i int) float64 {
	t := g.Tiles[i].Terrain
	if !natural(t) {
		return t.Hold()
	}
	herb, wood, ok := g.covers(i)
	if !ok {
		return t.Hold()
	}
	return holdBare*(1-herb-wood) + Grass.Hold()*herb + Forest.Hold()*wood
}

// rootRise is how much steeper than loose ground tile i's stands for the
// roots in it, as a fall, with soil metres of soil on it, at a slope that
// would fail at critical without them: the cohesion term of the infinite slope, c/(γ·z·cos²θ), with c the
// roots' cohesion under the cover (see woodCohesion), as much of it as
// reaches the plane the slide would fail on, z that plane's depth (see
// slideDepth) and θ the slope. No more than rootMost. Nothing on ground a
// field or an outcrop, and nothing in a history.
func (g *Grid) rootRise(i int, soil, critical float64) float64 {
	if !natural(g.Tiles[i].Terrain) {
		return 0
	}
	herb, wood, ok := g.covers(i)
	if !ok || herb+wood <= 0 {
		return 0
	}
	z := math.Max(slideDepth, soil)
	c := woodCohesion*wood*math.Min(1, rootWood/z) + herbCohesion*herb*math.Min(1, rootHerb/z)
	cos2 := 1 / (1 + critical*critical)
	return math.Min(rootMost, c/(soilWeight*z*cos2))
}
