package terra

import (
	"github.com/LukasSelin/terra/internal/atmos"
)

// The snow on the ground through the year, and the ice where it outlasts
// the year.
//
// The year's cold days lay their precipitation down as snow, and the warm
// ones melt it, by degree-days (see atmos.BucketCold); the melt goes into the
// soil's bucket and on to the rivers in the season it melts in, which is
// what puts a snow-fed river's high water in the spring and early summer.
// Where more snow falls than the year melts or the air takes back, it
// outlasts the year, and the ground is under ice: that is Barren, read off
// the snow's own mass balance in place of Ohmura's line, which is now the
// check on it (TestTheIceIsOhmuras).
//
// None of it flows. A glacier's ice spreads down from where the snow outlasts
// the year to where it melts, and what keeps a valley glacier's tongue in the
// forest is its flow; that is the ice of the history's to work out, and here
// the ice stands where the balance is over nothing and nowhere else.

// snowYear is the year tile i's snow is read on: the year's mean at its
// ground and its swing, signed by hemisphere. t is the air's year's mean at
// its height, which a map read before its tiles have their own years - a
// history's - reads its snow at. Once they have them, the snow reads the
// tile's own, with the sea and the currents round it, as the frost and the
// tree line do; the swing is the one the air's year has there either way.
func (g *Grid) snowYear(i int, t float64) (mean, swing float64) {
	swing = atmos.Swing
	if g.Wrap && g.air != nil {
		y := i / g.W
		swing = atmos.SwingUnder(g.air.Forcing, g.air.Lat[y], g.contAt(i))
	}
	if len(g.warm) == len(g.Tiles) {
		t = g.meanOn(i, g.laidHeight(i))
	}
	return t, swing
}

// phased reads tile i's figure for phase of the year out of v, laid out as
// the soil's water is, and nothing where it has not been read.
func phased(v []float32, i, phase int) float64 {
	if i < 0 || (i+1)*atmos.Phases > len(v) || phase < 0 || phase >= atmos.Phases {
		return 0
	}
	return float64(v[i*atmos.Phases+phase])
}

// SnowWater is the water, in mm, of the snow lying on tile i on the mean
// through phase of the year (see SoilWater for the phases). On a glacier it
// is the year's snow over the ice: what lies on it over the least it holds.
func (g *Grid) SnowWater(i, phase int) float64 { return phased(g.snowWater, i, phase) }

// SnowCover is the share of tile i's ground the snow covers on the mean
// through phase of the year, from nothing to one: what the ground's albedo is
// to be read off (Roesch and others, 2001). A glacier is covered the year
// round.
func (g *Grid) SnowCover(i, phase int) float64 { return phased(g.snowCover, i, phase) }

// MeltIn is how much of tile i's snow, in mm of water, melts in phase of the
// year, into its soil and on to its rivers.
func (g *Grid) MeltIn(i, phase int) float64 { return phased(g.meltIn, i, phase) }

// IceBalance is the mass balance of tile i's snow, in mm of water a year:
// what the snow that outlasts the year gains in it, and nothing where the
// year's snow melts out. Ground with any is under ice: see Barren.
func (g *Grid) IceBalance(i int) float64 {
	if i < 0 || i >= len(g.ice) {
		return 0
	}
	return float64(g.ice[i])
}
