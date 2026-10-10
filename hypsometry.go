package terra

import (
	"math"
	"slices"

	"github.com/LukasSelin/terra/internal/phase"
)

// The height a globe's land stands at: the planet's, and the map's on top of
// it.
//
// A globe's history is a planet. Its tiles are deepSpan wide while it runs,
// thirty-seven and a half kilometres on the globe preset, and its heights are
// in a planet's metres: its rates are real ones, and the step it keeps
// between a continent and the ocean floor is the earth's four and a half
// kilometres. The map the history is laid on is the same tiles read at
// TileSpan, twenty-five metres. What the two share is the metre upright and
// nothing across: a fall of a kilometre between two tiles is a slope of one
// in thirty-seven on the planet and of forty in one on the map. So a height
// can cross from the history to the map unchanged - the metre is the one
// unit they have in common - but a slope cannot, and every pass of the map
// that reads a slope (the shaping, the slides, the creep, the cutting, the
// woods on steep ground) reads it at the map's span.
//
// That is why the history's heights used to be thrown away. They were handed
// to the map by rank onto the drawn map's spread, sixty metres of lowland and
// the high country 260 above it (see basins and normalise), and a globe's
// highest range stood a few hundred metres over its sea. It is also why they
// cannot simply be kept in Height: a range kilometres high on tiles
// twenty-five metres wide is a wall, the slides cut it back to Critical, and
// the shaping, which lays the ground to the scale of the highest of it, makes
// every hillside on the map forty times steeper.
//
// So a globe's land has two heights, one for each scale. Height is the map's:
// the ground at TileSpan as the water and the slides have left it, at the
// drawn map's spread, and everything that reads a slope reads it. country is
// the planet's: how high the country a tile lies in stands, in metres above
// the sea, at the history's span, where a kilometre between two tiles is a
// gentle fall. The map's ground is the detail below the history's grid and
// stands on the country; a tile's elevation is the two together (Elevation),
// and that is what the air reads its warmth off, since a glacier on a range
// four kilometres high is there for the height and not for the slope.
//
// What the country is: the history's own height over its own sea, one metre
// for one (countryOf, handed down as uplift is). The history's crust floats
// on its thickness (isostasy.go) and its ground comes down at the pace its
// relief sets (denude.go), so the heights it ends with are a planet's land:
// half of it under 361 to 474 m on the first globe and the first two small
// ones against the earth's 461, and each share of it from a quarter to the
// ninety-ninth hundredth within 0.6 to 1.6 times the earth's
// (TestTheHistoryStandsOnItsCrust). Before the crust floated the history's
// land was a plateau a few hundred metres over its sea, and the country took
// the history's order and the earth's spread: each tile was given, by its
// rank, the height the earth's land has at the same rank. Nothing ranks it
// now.
//
// The map's sea was not the history's. The map poured its own onto the
// ground basins laid, and left more of the planet dry: 0.26 to 0.43 of its
// tiles on those three worlds, against the 0.22 to 0.36 the history had over
// its sea. It is the history's now, near enough: basins lays the continental
// crust the history's sea drowned with the floor (drownedCrust), and the map's
// water, which more than fills the floor, stands at its edge. What follows is
// how it was read before that. Moved until as much of it stood over nothing as the map has dry land,
// the history's height put its shore 230 to 620 m down its margins, and the
// land the map has dry over that shelf lifted all the rest: half the land
// stood over 705 to 1,030 m. So the shore is the history's sea, and the land
// the map has dry where the history had sea stands on no country of its own:
// it is the shelf the map's lower sea has left dry, the earth's coastal
// plains and deltas, which the history's rivers do not build.
//
// What that leaves short is the low tail: the lowest tenth of the land
// stood within 13 to 67 m of the sea, against 71 on the earth, and the
// lowest twentieth within 8 to 20 m. So below the fifth of the land the
// history holds lowest, the country is laid at least as high as the earth's
// land stands at the same share: its lowest band, two hundred metres under
// 28 in a hundred of the land, laid evenly (lowShare, lowTop and lowFloor).
// Once the coast was the history's sea contour (drownedCrust) that floor was
// not enough: the history's country is steep at its sea, and the lowest
// tenth stood 168 to 296 m up. So up to half the land the country is drawn
// down to the floor, the more the lower (lowBlend), and within some hundred
// kilometres of the shore it comes down to the sea (coastWidth).
// That is all of the earth's curve that is left in the making; the rest of
// it is a test's (cogleyLand). Above that fifth the history's own height is
// the country, and on the five worlds TestTheGlobeStandsAtTheEarthsHeights
// reads half the land stands under 326 to 512 m.
//
// And the water does not climb it. The rivers run on Height and the
// country is the history's, so where the two disagree about which way is
// down - a hollow the history left that the map's water fills and spills,
// a flat the shaping's roughness turned - a river's step climbs the country:
// laid off the history and not graded, 7 to 12 in a hundred of the water's
// steps over dry land climbed in Elevation, against 0.06 to 0.2 in Height.
// So once the drainage is taken the country is graded along it
// (gradeCountry): the nearest country, in least squares, that never rises
// from a tile to the tile its water goes to. A river crossing a range the
// history left across its way is a gorge through it, the basin behind the
// range a little higher, both at the mean of what they were. Graded so, the
// share that climbs in Elevation is the share that climbs in Height.
//
// Only on a globe. A valley's history is a planet two hundred kilometres a
// tile read onto eighty tiles of a field, and a valley keeps its drawn
// spread whole (the owner's decision, 2026-09-15: real heights are
// globe-only).
//
// What the air makes of it. The lapse takes the warmth off the high country,
// six and a half degrees a kilometre: the year, the frost, the tree line,
// the warmth the soil forms under and the warmth PET is read at all read the
// elevation (lapseHeight). The winds' ground and the rain's lift do not: they
// read the map's ground, as they did before there was a country.
//
// They were tried on it. Read off the country, the ranges wring rain out of
// the air that climbs them by Smith and Barstad's linear theory at the air's
// own span, which is a planet's (internal/atmos/orographic.go), and the first
// globe's land rain went from 1075 mm a year to 1326 - the earth's land has
// some 800 - its horse latitudes' from 747 to 1232 against the westerlies'
// 712 to 969: the subtropics' ranges rain on the trades, which carry the
// most water and have nothing in this air to cap them, as the earth's trade
// inversion keeps Hawaii's lee and the Atacama dry. Blurring the country
// eight passes took it only to 1185, so it is the ranges and not the steps
// between tiles. Five yardsticks of the rivers and the soil moved out of
// their bands with it. The cap is the air's work (A2 and A3), and until it is
// there the ranges' rain waits for it.

// The earth's lowest land, which the country is laid no lower than below
// lowShare of the way up the land's order: lowFloor of the land stands under
// lowTop metres, spread evenly (ETOPO5's land, NOAA 1988, in its lowest band
// split at 200 m as Cogley 1984 draws it: its elevations peak near 100 m).
const (
	lowShare = 0.2
	lowFloor = 0.28
	lowTop   = 200.0
)

// lowBlend is how far up the land's order the country comes to the history's
// own height from the earth's lowest land. Below it the history's height
// over that floor is let in smoothly, nothing at the foot of the order and
// all of it at lowBlend: since the coast became the history's sea contour
// the history's lowest land stands well over the floor, which then set only
// a minimum. Higher, the middle of the land comes down with it - 687 m to
// 672 on the first globe at 0.6 and to 551 at 0.8 - and the middle is the
// history's to say.
const lowBlend = 0.5

// coastWidth is how far from the shore, in a planet's metres, the country
// comes up to its full height: from nothing at the water's edge, smoothly.
// The land low in the history's order is not its coast's - the coast is the
// history's sea contour, and the country beside it stands where the
// history's margin did, 377 to 540 m in the middle within three tiles of the
// sea on the first three globes, while the map's ground there is 7 to 12 m -
// so the floor and lowBlend, which go by the order, do not reach it. The
// earth's coasts are its plains and deltas, and its land under 100 m lies
// along them. With both, the five worlds' lowest tenth stands within 63 to
// 90 m of the sea and 11 to 14 in a hundred of their land under 100 m (the
// earth's: 71 m and 14), where by the order alone it was 115 to 142 m and 7
// to 8.5, and with the floor alone 168 to 296 m and 1.2 to 2.7; the land
// within 50 km of the shore stands at 43 to 81 m in the middle, where it
// stood at 211 to 375. At 100 km the share under 100 m was 10 to 13 in a
// hundred, at 150 the lowest twentieth stood within 14 to 19 m against the
// earth's 36.
const coastWidth = 125e3

// countryOf is how high each tile of a globe's history stands over the
// history's sea as its last epoch ends, in a planet's metres, and below
// nothing under that sea. It is softened as the heights are when the history
// is over, so that it lies where they do (see smoothing).
func (g *Grid) countryOf() []float64 {
	c := make([]float64, len(g.Tiles))
	for i := range c {
		c[i] = g.Height[i] - g.base
	}
	for k := 0; k < g.passes(smoothing); k++ {
		c = g.spread(c)
	}
	return c
}

// drownedCrust is which tiles of continental crust a globe's history left
// under its sea, by the country they stand at: below nothing. They are the
// earth's shelves and drowned margins - its continental crust is some four
// tenths of its surface and its land under three - and the map lays them
// with the floor, so that its coast is where the history's sea stood on the
// continents' own ground and not the edge of their crust. See basins.
func drownedCrust(country []float64, ocean []bool) []bool {
	out := make([]bool, len(country))
	for i, c := range country {
		out[i] = !ocean[i] && c <= 0
	}
	return out
}

// layCountry gives a globe's dry land the height its country stands at. It is
// laid once, when the sea has been poured and before the ground is shaped,
// off the heights the history ended at (planetHeight), which it lets go of.
//
// The map's own ground already stands some way above the sea, at the drawn
// spread, and the country is the rest of the way to the history's height:
// that height less the map's, and never less than nothing. Below lowShare of
// the land, in the history's order and by the ground each tile stands for on
// a sphere, the height it is laid to is at least the earth's at that share;
// above it, at least the earth's at lowShare, so that the order is kept. And
// below lowBlend of the land what the history has over that floor is let in
// by degrees, so that the lowest land stands near the earth's and not on
// the history's own; and near the shore all of it comes down to the sea,
// the nearer the lower (coastWidth).
func (g *Grid) layCountry() {
	defer phase.Start("layCountry")()
	deep := g.planetHeight
	g.planetHeight = nil
	if g.sea < 0 || !g.Wrap || len(deep) != len(g.Tiles) {
		return
	}
	var dry []int32
	total := 0.0
	for i := range g.Tiles {
		if !g.sunk(i) {
			dry = append(dry, int32(i))
			total += g.rowArea(i / g.W)
		}
	}
	if len(dry) == 0 {
		return
	}
	slices.SortFunc(dry, func(a, b int32) int {
		switch da, db := deep[a], deep[b]; {
		case da < db:
			return -1
		case da > db:
			return 1
		}
		return int(a - b) // ties by position, so a world repeats
	})
	g.country = make([]float64, len(g.Tiles))
	inland := g.fromTheSea(coastWidth)
	run := 0.0
	for _, i := range dry {
		w := g.rowArea(int(i) / g.W)
		r := (run + w/2) / total
		run += w
		low := lowTop * math.Min(r, lowShare) / lowFloor
		at := math.Max(deep[i], low)
		at = low + (at-low)*smooth(clamp01(r/lowBlend))
		at *= smooth(clamp01(inland[i] / coastWidth))
		g.country[i] = math.Max(0, at-(g.Height[i]-g.sea))
	}
}

// fromTheSea is how far each tile of a globe lies from the nearest tile
// under its sea, in a planet's metres from the shore - half a tile less than
// from the tile's middle - and reach for any further than that. The tiles
// are deepSpan apart north and south and that much by the cosine of the
// latitude east and west (rowArea); the sea's own tiles are nothing.
func (g *Grid) fromTheSea(reach float64) []float64 {
	span := deepSpan(g)
	out := make([]float64, len(g.Tiles))
	ry := int(math.Ceil(reach/span)) + 1
	g.EachRow(func(y int) {
		for x := 0; x < g.W; x++ {
			i := y*g.W + x
			if g.sunk(i) {
				continue
			}
			best := reach + span/2
			for dy := -ry; dy <= ry; dy++ {
				yy := y + dy
				if yy < 0 || yy >= g.H {
					continue
				}
				north := float64(dy) * span
				if math.Abs(north) >= best {
					continue
				}
				// East and west at the wider of the two rows, so that no
				// tile is read nearer than it is.
				c := math.Max(g.rowArea(y), g.rowArea(yy))
				rx := min(g.W/2, int(math.Ceil(best/(span*math.Max(c, 1e-3)))))
				for dx := -rx; dx <= rx; dx++ {
					if !g.sunk(yy*g.W + ((x+dx)%g.W+g.W)%g.W) {
						continue
					}
					if d := math.Hypot(north, float64(dx)*span*c); d < best {
						best = d
					}
				}
			}
			out[i] = math.Min(reach, math.Max(0, best-span/2))
		}
	})
	return out
}

// rowArea is how much of a sphere's surface a tile of row y of a globe
// stands for, against a tile on the equator.
func (g *Grid) rowArea(y int) float64 {
	return math.Cos((90 - 180*(float64(y)+0.5)/float64(g.H)) * math.Pi / 180)
}

// countryAt is the height of the country under tile i, or nothing where
// there is none: on a map that is not a globe, and under the sea.
func (g *Grid) countryAt(i int) float64 {
	if g.country == nil || g.sunk(i) {
		return 0
	}
	return g.country[i]
}

// lapseHeight is the height the air's warmth is read at on tile i: the ground
// as the weather stands on it (see laidHeight), on the country it lies in. It
// is what the lapse takes the warmth down by. The winds and the rain rise
// over laidHeight alone: see above.
func (g *Grid) lapseHeight(i int) float64 { return g.laidHeight(i) + g.countryAt(i) }

// Elevation is how high tile i stands, in metres on the map's scale of
// Height, with the country it lies in: on a globe, the planet's height of
// the land under the map's own ground (see layCountry), and on any other map,
// or under the sea, Height itself. A slope is read off Height; how high a
// place is, and how cold, off this.
func (g *Grid) Elevation(i int) float64 { return g.Height[i] + g.countryAt(i) }

// Elevation is how high this tile stands, with the country it lies in:
// Grid.Elevation at this tile.
func (v TileView) Elevation() float64 { return v.g.Elevation(v.i) }

// gradeCountry lays a globe's country along the drainage as it stands: the
// country nearest the one laid, in least squares, that never rises from a
// tile to the tile its water goes to. The sea's tiles are outside it, and a
// tile whose water goes to the sea, or nowhere, is free of anything below
// it. It is taken again wherever the drainage is taken for good: at the end
// of the shaping, of the cutting and of the mud the tide lays (see silt).
// See the note above.
func (g *Grid) gradeCountry() {
	defer phase.Start("gradeCountry")()
	n := len(g.Tiles)
	if g.country == nil || len(g.down) != n || len(g.route) != n {
		return
	}
	var t isotone
	t.fit(g.country, func(i int32) int32 {
		if g.sunk(int(i)) {
			return -2
		}
		d := g.down[i]
		if d < 0 || g.sunk(int(d)) {
			return -1
		}
		return d
	}, g.route)
}

// isotone is the least-squares fit to values on a forest that never rises
// from a tile to the one below it (Pardalos and Xue 1999): each tile starts a
// block of its own, and a block that stands over the lowest of the blocks
// above it takes that one in, until none does. The blocks above a block are
// kept in a leftist heap by their mean.
type isotone struct {
	sum, w      []float64
	left, right []int32
	dist        []int32
	heap        []int32 // the heap of the blocks above each tile's block
	owner       []int32 // the block a tile's block was taken into, or itself
}

func (t *isotone) mean(b int32) float64 { return t.sum[b] / t.w[b] }

func (t *isotone) meld(a, b int32) int32 {
	if a < 0 {
		return b
	}
	if b < 0 {
		return a
	}
	if t.mean(b) < t.mean(a) || t.mean(b) == t.mean(a) && b < a {
		a, b = b, a
	}
	t.right[a] = t.meld(t.right[a], b)
	l, r := t.left[a], t.right[a]
	if l < 0 || r >= 0 && t.dist[l] < t.dist[r] {
		t.left[a], t.right[a] = r, l
	}
	if t.right[a] < 0 {
		t.dist[a] = 0
	} else {
		t.dist[a] = t.dist[t.right[a]] + 1
	}
	return a
}

// fit fits v in place. below is the tile below each tile: -1 at a root, -2
// for a tile outside the forest. order has every tile after the one below
// it.
func (t *isotone) fit(v []float64, below func(int32) int32, order []int32) {
	n := len(v)
	t.sum, t.w = make([]float64, n), make([]float64, n)
	t.left, t.right, t.dist = make([]int32, n), make([]int32, n), make([]int32, n)
	t.heap, t.owner = make([]int32, n), make([]int32, n)
	for i := range v {
		t.sum[i], t.w[i] = v[i], 1
		t.left[i], t.right[i], t.heap[i], t.owner[i] = -1, -1, -1, int32(i)
	}
	for k := len(order) - 1; k >= 0; k-- {
		i := order[k]
		d := below(i)
		if d == -2 {
			continue
		}
		h := t.heap[i]
		for h >= 0 && t.mean(h) < t.mean(i) {
			t.sum[i] += t.sum[h]
			t.w[i] += t.w[h]
			t.owner[h] = i
			rest := t.meld(t.left[h], t.right[h])
			h = t.meld(rest, t.heap[h])
		}
		t.heap[i] = h
		t.left[i], t.right[i], t.dist[i] = -1, -1, 0
		if d >= 0 {
			t.heap[d] = t.meld(t.heap[d], i)
		}
	}
	for _, i := range order {
		if below(i) == -2 {
			continue
		}
		if o := t.owner[i]; o != i {
			t.owner[i] = t.owner[o] // the block below in the order is settled first
		}
	}
	for _, i := range order {
		if below(i) != -2 {
			v[i] = t.mean(t.owner[i])
		}
	}
}
