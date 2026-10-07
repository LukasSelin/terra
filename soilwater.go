package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
	"github.com/LukasSelin/terra/internal/atmos"
)

// The water in the soil through the year.
//
// The ground holds the rain of a wet season into a dry one, and what it
// cannot hold goes to the rivers: a bucket on each tile, run through the
// year's four phases (see atmos.Bucket). How much it holds is the soil's: how
// deep it is, and how much water its mixture keeps where the roots can take
// it back. A deep loam in a hollow carries a spring's rain into the summer; a
// skin of sand on a ridge holds a week's.

// The plant-available water of a soil, from its sand and clay, by Saxton and
// Rawls's (2006) regressions on the soils of the USDA's national soil
// characterisation data: the water held at field capacity, 33 kPa, less what is held at the
// wilting point, 1500 kPa, each a share of the soil's volume. The organic
// matter is taken at their middling two and a half per cent by weight, which
// is what the soils of the map are read at until the soil's carbon is a
// stock (see pedogenesis.go). They give 0.05 for a sand, 0.14 for a loam and
// 0.11 for a clay.
//
// Their regressions are fitted inside the USDA's texture triangle and not
// at its corners: a soil of pure silt would come out at nearly three tenths,
// which no soil holds. pawLeast and pawMost bound it to the range their own
// table of classes spans.
const (
	soilOrganic = 2.5 // % by weight
	pawLeast    = 0.04
	pawMost     = 0.20
)

// plantWater is the plant-available water of a soil sand and clay of it, as
// a share of its volume. See soilOrganic.
func plantWater(sand, clay float64) float64 {
	s, c, om := clamp01(sand), clamp01(clay), soilOrganic
	wilt := -0.024*s + 0.487*c + 0.006*om + 0.005*s*om - 0.013*c*om + 0.068*s*c + 0.031
	wilt += 0.14*wilt - 0.02
	field := -0.251*s + 0.195*c + 0.011*om + 0.006*s*om - 0.027*c*om + 0.452*s*c + 0.299
	field += 1.283*field*field - 0.374*field - 0.015
	return math.Max(pawLeast, math.Min(pawMost, field-wilt))
}

// pawOf is the plant-available water of tile i's soil, or nothing where the
// tile is under the water the air takes its fill from. A soil whose mixture
// has not been laid - a map before its soils are, which has neither sand
// nor clay - is read at the mixture its rock weathers to.
func (g *Grid) pawOf(i int) float64 {
	if g.sunk(i) {
		return 0
	}
	sand, clay := g.Sand[i], g.Clay[i]
	if sand == 0 && clay == 0 {
		sand, clay = g.TextureAt(geom.Pos{X: i % g.W, Y: i / g.W})
	}
	return plantWater(sand, clay)
}

// soilBucket lays out what the air's bucket reads of the ground: each tile's
// plant-available water into g.paw, nothing under the water.
func (g *Grid) soilBucket() {
	if len(g.paw) != len(g.Tiles) {
		g.paw = make([]float32, len(g.Tiles))
	}
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			g.paw[i] = float32(g.pawOf(i))
		}
	})
}

// SoilWater is the water tile i's soil holds, in mm, on the mean through
// phase of the year (see atmos.Phases: 0 the north's winter, 1 the spring, 2
// the north's summer, 3 the autumn), as the air last read it. It is nothing
// under water and where the weather has not been read.
func (g *Grid) SoilWater(i, phase int) float64 {
	if i < 0 || (i+1)*atmos.Phases > len(g.soilWater) || phase < 0 || phase >= atmos.Phases {
		return 0
	}
	return float64(g.soilWater[i*atmos.Phases+phase])
}

// RainIn is how much of tile i's rain, in mm, falls in phase of the year.
// The four add to Rain.
func (g *Grid) RainIn(i, phase int) float64 {
	if i < 0 || (i+1)*atmos.Phases > len(g.rainIn) || phase < 0 || phase >= atmos.Phases {
		return 0
	}
	return float64(g.rainIn[i*atmos.Phases+phase])
}

// SoilHold is the most water tile i's soil holds where the roots can take
// it, in mm: its bucket. See atmos.Hold.
func (g *Grid) SoilHold(i int) float64 {
	if i < 0 || i >= len(g.soilHold) {
		return 0
	}
	return float64(g.soilHold[i])
}

// RunoffIn is how much of tile i's runoff, in mm, it sheds in phase of the
// year. The four add to Runoff.
func (g *Grid) RunoffIn(i, phase int) float64 {
	if i < 0 || (i+1)*atmos.Phases > len(g.runoffIn) || phase < 0 || phase >= atmos.Phases {
		return 0
	}
	return float64(g.runoffIn[i*atmos.Phases+phase])
}
