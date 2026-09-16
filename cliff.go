package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
)

// cliffs cuts back the ground standing over the sea behind every cell of the
// surf by Sunamura's rate - all of it but a beach, which is the drift's - and books its sand to the cell's supply and its
// fines to the sea.
//
// A cliff goes back into itself: over a step longer than it takes to cut one
// tile down to the sea, the next tile behind it is cut too, and so on for as
// long as the step lasts. What is left in front of the cliff is a platform at
// mean sea, and a wave crossing it breaks on it: what reaches the foot of the
// cliff behind is no higher than the breakerIndex allows in the water over the
// platform at high tide, so a platform shelters its cliff, and on a coast with
// no tide a cliff behind one stops. That is Trenhaile's (2000) account of why
// platforms are as wide as they are.
//
// Each tile of ground is cut by the cell beside it that sends it the biggest
// storms - see facing - once, in tile order. Ground somebody has built on is not cut.
func (g *Grid) cliffs(s *surf, years float64, supply []float64) {
	by := g.facing(s)
	span := g.span()
	tide := func(i int) float64 {
		if len(g.tidal) != len(g.Tiles) {
			return 0
		}
		return float64(g.tidal[i]) * MeanHigh
	}
	for j0 := range g.Tiles {
		c := int(by[j0])
		if c < 0 || g.underSea(j0) {
			continue
		}
		from := g.PosOf(int(s.cells[c]))
		step := g.Delta(from, g.PosOf(j0))
		run := span * math.Hypot(float64(step.X), float64(step.Y))
		left := years
		attack := s.attack[c]
		for j, first := j0, true; left > 0; first = false {
			t := &g.Tiles[j]
			if t.Mark != None || g.underSea(j) || g.beach(j) {
				break
			}
			if !first {
				// Behind a platform, the waves that reach the cliff are the
				// broken ones the high water over the platform can carry.
				force := seaDensity * gravity * breakerIndex * tide(j)
				for b := range attack {
					attack[b] = 0
					if x := force / (cliffThreshold * strength(b)); x > 1 {
						attack[b] = math.Min(s.attack[c][b], math.Log(x))
					}
				}
			}
			face := g.Height[j] - g.sea
			stuff := int(t.Bedrock)
			if float64(g.Soil[j]) >= face {
				stuff = int(BedrockCount)
			}
			rate := cliffRate * attack[stuff]
			if rate <= 0 {
				break
			}
			cut := face
			if need := run / rate; need <= left {
				left -= need
			} else {
				cut = face * rate * left / run
				left = 0
			}
			got := g.takeGround(j, cut)
			supply[c] += got[Sand]
			g.exported[Silt] += got[Silt]
			g.exported[Clay] += got[Clay]
			q := g.Norm(geom.Pos{X: g.PosOf(j).X + step.X, Y: g.PosOf(j).Y + step.Y})
			if !g.In(q) {
				break
			}
			j = g.Index(q)
		}
	}
}

// beachSand is the least share of sand a beach is: USDA's sand, whose class
// begins at eighty-five parts in a hundred.
const beachSand = 0.85

// beach reports whether tile i is a beach and not a cliff: its face, from the
// sea up, all soil, and that soil sand.
func (g *Grid) beach(i int) bool {
	return g.Sand[i] >= beachSand && float64(g.Soil[i]) >= g.Height[i]-g.sea
}
