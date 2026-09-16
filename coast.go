package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
)

// The coast: what the waves do to the ground they break on.
//
// Three things, each as the literature has it, and each booked grain by grain
// against what the water takes and lays, so that the ground as a whole loses
// only what goes to the sea.
//
// The breakers stir the bed they break on and keep what is fine in it up in the
// water, where it is carried out past the surf and lost to the shore: a beach is
// sand because the waves will not let anything finer lie on it. See winnow.
//
// They drive the sand along the shore, the way the waves come in slantwise
// (the CERC formula; see wave.go), from where the shore gives it up to where
// the drive slackens, or the shore turns away and the sand goes on straight
// into the water ahead of it: a beach at the head of a bay, a spit off a
// headland, a bar across a river's mouth. See littoral.
//
// And they cut back the ground that stands above them, as fast as the force
// of a breaking wave outruns what the ground is made of (Sunamura 1977, 1992),
// leaving a platform at the level of the sea that the next wave has to cross
// to reach the cliff. See cliffs.

// Cliff retreat. Sunamura (1977, 1992) has a cliff go back at
//
//	dx/dt = C·ln(F_w/F_r)    while F_w > F_r
//
// with F_w the force of the waves, ρ·g·H_b, and F_r the strength of the rock.
// It is the logarithm because a stronger sea does not break more rock in
// proportion, it breaks what it could not before. The strength is the rock's
// tensile strength - see tensile - because what a wave does to a cliff is lever
// blocks out along their joints and fatigue what it cannot lever, and both are
// tension; and for a face of soil it is soilStrength.
//
// A cliff face is not the rock in a laboratory: it is jointed, weathered and
// wet, and a wave breaking against it is a jet into its cracks, so the stress
// at which it gives is a small share of the intact rock's. That share is
// cliffThreshold, and it and C are set by what cliffs do: Sunamura (1992) and
// Prémaillon, Regard, Dewez and Auda's (2018) global compilation have weak
// rock going back some tens of centimetres a year and hard rock a few
// centimetres or none, and a bank of till on an exposed coast a metre or two
// (Holderness). Under breakers a metre and a half high all year:
//
//	          shale  sandstone  limestone  schist  granite  basalt  soil
//	m/yr      0.20     0.12       0.08      0.07    0.02     0     0.86
const (
	cliffRate      = 0.144 * metre / yr
	cliffThreshold = 1.88e-3
	// soilStrength is what holds a face of soil up, in pascals: not a tensile
	// strength, which loose ground has none of to speak of, but its cohesion,
	// some tens of kilopascals in a stiff clayey till.
	soilStrength = 20e3 * pascal
)

// strength is F_r's σ for rock b, or for soil where b is BedrockCount.
func strength(b int) float64 {
	if b >= int(BedrockCount) {
		return soilStrength
	}
	return tensile[b]
}

// mixingDepth is how deep a storm's breakers stir the bed, as a share of their
// height: Ferreira and others (2000), over steep and gentle foreshores, 0.27.
const mixingDepth = 0.27

// coast does what the waves do to g's coast in years - the cliffs, the drift of
// the sand and the winnowing, in that order - with sands the sand the
// rivers brought to each tile of the sea this step, in metres over a tile. What
// goes out of reach of the shore is added to what the step sent to the sea.
func (g *Grid) coast(s *surf, years float64, sands []float64) {
	if len(s.cells) == 0 {
		return
	}
	supply := make([]float64, len(s.cells))
	if sands != nil {
		for c, i := range s.cells {
			supply[c] = sands[i]
		}
	}
	g.cliffs(s, years, supply)
	g.littoral(s, years, supply)
	g.winnow(s, years)
}

// berm is how high the waves at cell c build the sand of a beach, as a height
// on the map: to the high water of an ordinary tide there, and above it by as
// far as the effective wave runs up, which is about its breaker height.
func (g *Grid) berm(s *surf, c int) float64 {
	i := s.cells[c]
	high := 0.0
	if len(g.tidal) == len(g.Tiles) {
		high = float64(g.tidal[i]) * MeanHigh
	}
	return g.sea + high + s.storm[c]
}

// facing is, for every tile, the cell of the surf the waves reach it from: a
// cell itself, and for the ground beside the surf the cell beside it with the
// biggest storm waves, the first in tile order among equals. Everything else
// gets -1.
func (g *Grid) facing(s *surf) []int32 {
	by := make([]int32, len(g.Tiles))
	for i := range by {
		by[i] = -1
	}
	for c, i := range s.cells {
		by[i] = int32(c)
	}
	for c, i := range s.cells {
		p := g.PosOf(int(i))
		for _, d := range Dirs {
			q := g.Norm(geom.Pos{X: p.X + d.X, Y: p.Y + d.Y})
			if !g.In(q) {
				continue
			}
			j := g.Index(q)
			if g.underSea(j) {
				continue
			}
			if b := by[j]; b < 0 || s.storm[c] > s.storm[b] {
				by[j] = int32(c)
			}
		}
	}
	return by
}

// takeGround takes d metres off the top of tile i, its soil first and the rock
// after, and says what it was made of by grain.
func (g *Grid) takeGround(i int, d float64) [Grains]float64 {
	soil := math.Min(float64(g.Soil[i]), d)
	was := g.parts(i)
	var out [Grains]float64
	for gr := range out {
		out[gr] = soil * was[gr]
	}
	if rock := d - soil; rock > 0 {
		sand, clay := g.TextureAt(g.PosOf(i))
		out[Sand] += rock * sand
		out[Silt] += rock * clamp01(1-sand-clay)
		out[Clay] += rock * clay
	}
	g.Height[i] -= d
	g.Soil[i] = float32(math.Max(0, float64(g.Soil[i])-soil))
	return out
}

// winnow carries off what is finer than sand from the bed the breakers stir,
// on every cell of the surf shallow enough for the waves to move its bed and on
// the beach behind it: the ground beside the surf that stands no higher than
// the berm. In the surf the water is never still enough for silt, falling a
// tenth of a millimetre a second, or clay, falling a thousandth, to settle
// through it, so what the waves stir up of either goes out past the surf with
// the backwash and the rips, and is lost to the shore.
//
// The effective wave is by its definition the storm of the year, and each
// year's storm stirs mixingDepth of its breaker height of the soil and takes
// the fines out of that. The soil here is one mixture however deep it is, so
// the fines in it go as exp(−z·t/H): after a year, the stir z of a soil H deep
// has lost z/H of its fines, and after a step of any length the same as after
// that many years one at a time. The more a shore is exposed, the deeper its
// storms stir it and the sooner it is sand.
func (g *Grid) winnow(s *surf, years float64) {
	by := g.facing(s)
	for i := range g.Tiles {
		c := int(by[i])
		if c < 0 {
			continue
		}
		t := &g.Tiles[i]
		if t.Mark != None || g.Soil[i] <= 0 {
			continue
		}
		if g.underSea(i) {
			if g.sea-g.Height[i] > s.closure[c] {
				continue
			}
		} else if g.Height[i] > g.berm(s, c) {
			continue
		}
		soil := float64(g.Soil[i])
		share := -math.Expm1(-mixingDepth * s.storm[c] * years / yr / soil)
		silt, clay := share*soil*g.siltAt(i), share*soil*g.Clay[i]
		gone := silt + clay
		if gone <= 0 {
			continue
		}
		rest := soil - gone
		sand := soil * g.Sand[i]
		if rest > 1e-12 {
			g.Sand[i] = sand / rest
			g.Clay[i] = (soil*g.Clay[i] - clay) / rest
		}
		g.Soil[i] = float32(math.Max(0, rest))
		g.Height[i] -= gone
		g.exported[Silt] += silt
		g.exported[Clay] += clay
	}
}
