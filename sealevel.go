package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
)

// The sea's level through the ice ages.
//
// For the last million years the sea has not stood still. Every hundred
// thousand years or so the ice sheets have grown over the northern continents
// for most of the cycle and melted in a few thousand years at the end of it,
// and the sea has gone down with the water they held and come back up when
// they let it go: the sawtooth of the oxygen isotopes (Broecker and van Donk
// 1970), a hundred and twenty metres or more below today's at the last glacial
// maximum (Lambeck and others 2014 have 134 m, twenty-one thousand years ago),
// and the rise mostly done by seven thousand years ago.
//
// A coast is what that leaves. While the sea was low the rivers ran out across
// the shelf to it and cut their valleys down toward it; when it came back it
// drowned them, and a drowned valley is an estuary, or a ria where the ground is
// steep. The floodplains the rivers cut down through are left standing over
// them as terraces. And a sandy shore the sea rises against is moved up and
// back with it, which is the Bruun rule.
//
// On a map here the fall is read as water, not as metres. What the ice takes is
// a share of the ocean's water, and a map whose sea is twenty metres deep would
// have no sea at all a hundred and twenty metres down: the share is the
// glacial fall over the depth of the real ocean, and the map's sea gives up
// that share of what it holds.

// The glacial cycle, as the sea's fall below today's over the years before the
// present. glacialLow is the fall at a glacial maximum, and the cycle is a
// sawtooth glacialCycle long: the sea stands at today's for holocene, came up
// from glacialLow over termination before that, and went down to glacialLow
// over the rest of the cycle before that.
const (
	glacialLow   = 120 * metre
	glacialCycle = 100 * kyr
	holocene     = 7 * kyr
	termination  = 13 * kyr
)

// oceanDepth is the real ocean's mean depth, 3682 metres (Charette and Smith
// 2010): what the fall of the sea is a share of the ocean's water against.
const oceanDepth = 3682 * metre

// glacialFall is how far, in metres, the sea stood below today's the given
// number of years before the present, on the sawtooth.
func glacialFall(before float64) float64 {
	u := mod(before, glacialCycle)
	switch {
	case u < holocene:
		return 0
	case u < holocene+termination:
		return glacialLow * (u - holocene) / termination
	default:
		return glacialLow * (glacialCycle - u) / (glacialCycle - holocene - termination)
	}
}

// glacialStages is how many stages a valley cut through the last glacial cycle
// is cut in, each glacialCycle over it long, with the sea at the level the
// middle of the stage had.
const glacialStages = 10

// cutThroughCycle cuts the valleys of a freshly drawn map as cutValleys does,
// through the last glacial cycle instead of the last two thousand years: in
// glacialStages stages, with the sea at each stage's level, and the shore moved
// up and back by the Bruun rule where the sea comes up. It ends with the sea
// where it began.
func (g *Grid) cutThroughCycle(rng interface{ Float64() float64 }) {
	g.cutThrough(rng, glacialFall)
}

// cutThrough is cutThroughCycle with the sea's fall, in metres below today's,
// given as a function of the years before the present.
func (g *Grid) cutThrough(rng interface{ Float64() float64 }, fallen func(before float64) float64) {
	if g.sea < 0 {
		g.cutValleys(rng)
		return
	}
	present := g.sea
	water := g.room()
	years := glacialCycle / glacialStages
	for k := range glacialStages {
		before := glacialCycle * (1 - (float64(k)+0.5)/glacialStages)
		level := present
		if fall := fallen(before); fall > 0 {
			level = g.level(water * (1 - fall/oceanDepth))
		}
		rise := level - g.sea
		g.sea, g.base = level, level
		if rise > 0 {
			g.shoreUp(g.surfOf(nil), rise)
		}
		g.carve(rng)
		g.wear(years)
		g.landslide(false)
		g.expose()
		g.drain()
	}
	g.sea, g.base = present, present
}

// shoreUp moves the shore up and back as the sea rises by rise over it, by
// Bruun's (1962) rule: the whole profile of the shore, from its berm B over the
// sea out to the depth of closure h, L across, goes up with the sea, and the
// ground that takes is the ground its upper part loses, S·L over each length
// of shore, laid down on its lower part; so the shore goes back R = S·L/(B + h).
// From the ground behind each cell of the surf it takes that S·L, and lays it
// on the shoreface in front of the cell, out to L. L is how far out Dean's
// profile of the ground's grain comes to the depth of closure, (h/A)^(3/2).
// Nothing is taken below the new sea, nor off ground somebody has built on.
func (g *Grid) shoreUp(s *surf, rise float64) {
	if rise <= 0 || len(s.cells) == 0 {
		return
	}
	by := g.facing(s)
	span := g.span()
	for j := range g.Tiles {
		c := int(by[j])
		t := &g.Tiles[j]
		if c < 0 || s.slot[j] >= 0 || g.underSea(j) || t.Mark != None || s.closure[c] <= 0 || s.out[c] == ([2]float64{}) {
			continue
		}
		a := deanCoeff * math.Pow(math.Max(g.medianGrain(j), deanFinest), deanPower)
		across := math.Pow(s.closure[c]/a, 1.5)
		cut := math.Min(rise*across/span, g.Height[j]-g.sea)
		if cut <= 0 {
			continue
		}
		// Out along the way to the sea, a tile for every span of L.
		from := g.PosOf(int(s.cells[c]))
		var at []int
		for m := 0; m == 0 || float64(m)*span < across; m++ {
			q := g.Norm(geom.Pos{
				X: from.X + int(math.Round(float64(m)*s.out[c][0])),
				Y: from.Y + int(math.Round(float64(m)*s.out[c][1])),
			})
			if !g.In(q) || !g.underSea(g.Index(q)) {
				break
			}
			at = append(at, g.Index(q))
		}
		if len(at) == 0 {
			continue // the shoreface in front has already been built up out of the sea
		}
		got := g.takeGround(j, cut)
		for _, i := range at {
			var laid [Grains]float64
			for gr := range got {
				laid[gr] = got[gr] / float64(len(at))
			}
			tile := &g.Tiles[i]
			mix(tile, float64(g.Soil[i]), laid)
			d := carrying(laid)
			g.Soil[i] += float32(d)
			g.Height[i] += d
		}
	}
}
