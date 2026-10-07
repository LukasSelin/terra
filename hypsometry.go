package terra

import (
	"math"
	"slices"
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
// What the country is. Read straight off the history - a planet's metres to
// the map's metres, one for one - it is not a planet's land. On the first
// globe and the first two small ones, the history's continental crust stood
//
//	                    1%      5%     25%    50%    75%    95%    99%    top
//	globe 1         -3,229  -1,562     153    331    456    547    620   1,915
//	small globe 1   -1,787    -832      87    403    794    932  1,004   1,254
//	small globe 2   -1,169    -552     108    656    787    882  1,006   1,901
//
// metres above the history's own sea, and its ocean crust a couple of
// hundred metres under its land: the median of the globe's floor stood 143 m
// above its sea. That is a plateau with a tail going down, where the earth's
// land is a lowland with a tail going up, and the floor at the land's height
// says why: the history has
// no isostasy. Its freeboard is two fixed levels and a plate settling toward
// them over 25 Myr (settleTime), and four million years of wear an epoch
// takes the ranges down with nothing to buoy the root back up, so the
// highest of a planet's land stands a kilometre over its sea and its floor
// is not where its age puts it either (abyss.go lays the floor again by age
// for that reason). G2 is the isostasy. Until then the history's heights are
// right in their order - the ranges are where the plates met, the lowlands
// where they did not - and wrong in their spread.
//
// So the country keeps the history's order and takes the earth's spread: the
// land is ranked by its height as the history left it, and the tile at each
// place in that order is given the height the earth's land has at that place
// in its own (earthHeights). The highest tile of the history's land is the
// highest of the earth's, the hundredth-highest the hundredth, and the
// lowlands a few hundred metres up, with the tail to several kilometres
// that Cogley (1984) has for the continents. When G2 makes the history's
// heights a planet's, earthHeights comes out and the history's own heights are
// the country, one metre for one.
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

// earthHeights is the earth's land by height: the share of the land above the
// sea that stands lower than each height, in metres. It is ETOPO5's (NOAA
// 1988, five-minute grid) land area in bands of 500 m, which is the curve
// Cogley (1984) draws for the continents, read above the sea: half the land
// under 460 m, nine tenths under 2.2 km, one hundredth over 4.7 km,
// and the highest at 8 km. ETOPO5 reads the top of the ice, so Antarctica's
// and Greenland's sheets stand in the 2 to 4 km bands, which a map with no
// ice sheets (G8) will be a little high in. The lowest band is split at 200
// m, 28 in a hundred of the land under it, so that the land heaps up just
// above the sea, as the earth's does: its elevations peak near 100 m
// (Britannica's hypsometry; ETOPO1, Amante & Eakins 2009).
var earthHeights = [...]struct{ share, height float64 }{
	{0, 0},
	{0.28, 200},
	{0.53315, 500},
	{0.72805, 1000},
	{0.83184, 1500},
	{0.88328, 2000},
	{0.91815, 2500},
	{0.94883, 3000},
	{0.97095, 3500},
	{0.98211, 4000},
	{0.98706, 4500},
	{0.99299, 5000},
	{0.99878, 5500},
	{0.99988, 6000},
	{0.99999, 6500},
	{1, 8000},
}

// earthHeightAt is the height of the earth's land at share f of the way up its
// order: the height that much of the land stands lower than. Between two
// heights of the table the land is spread evenly.
func earthHeightAt(f float64) float64 {
	f = clamp01(f)
	for k := 1; k < len(earthHeights); k++ {
		lo, hi := earthHeights[k-1], earthHeights[k]
		if f <= hi.share {
			return lo.height + (hi.height-lo.height)*(f-lo.share)/(hi.share-lo.share)
		}
	}
	return earthHeights[len(earthHeights)-1].height
}

// layCountry gives a globe's dry land the height its country stands at. It is
// laid once, when the sea has been poured and before the ground is shaped:
// the heights the map has then are the history's, put in its order by basins
// and laid at the drawn map's spread, so ranking them is ranking the history.
//
// The map's own ground already stands some way above the sea, at the drawn
// spread, and the country is the rest of the way to the earth's height at that
// place in the order: the earth's height less the map's at the same rank, and
// never less than nothing. The map's ground is then laid again by the shaping,
// at the same scale but in its own order, so the two heights added are the
// earth's curve as near as the shaping leaves it; see TestTheGlobeStandsAtTheEarthsHeights,
// which logs it.
//
// Each rank's country is at least the one below it, so the country is in the
// history's order whatever the map's spread did between them.
func (g *Grid) layCountry() {
	if g.sea < 0 || !g.Wrap {
		return
	}
	var dry []int32
	for i := range g.Tiles {
		if !g.sunk(i) {
			dry = append(dry, int32(i))
		}
	}
	if len(dry) == 0 {
		return
	}
	slices.SortFunc(dry, func(a, b int32) int {
		ha, hb := g.Height[a], g.Height[b]
		switch {
		case ha < hb:
			return -1
		case ha > hb:
			return 1
		}
		return int(a - b) // ties by position, so a world repeats
	})
	g.country = make([]float64, len(g.Tiles))
	least := 0.0
	for k, i := range dry {
		f := (float64(k) + 0.5) / float64(len(dry))
		least = math.Max(least, earthHeightAt(f)-(g.Height[i]-g.sea))
		g.country[i] = least
	}
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
