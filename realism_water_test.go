package terra

import (
	"math"
	"testing"

	"github.com/LukasSelin/terra/internal/atmos"
)

// The land's water through the year held against the earth's.
//
// The soil's bucket (soilwater.go, atmos.Bucket) decides what of the land's
// rain the air takes back and when the rest reaches the rivers. Three things
// are read off it: how much of the land's rain goes back to the air, how
// the year's runoff stands against Budyko's curve, and whether a seasonal
// climate's rivers run high in and after its wet season and not before it.
func init() {
	realYardsticks = append(realYardsticks, waterYardsticks...)
}

var waterYardsticks = []realYardstick{
	{yardstick: yardstick{
		name: "land evaporation over land rain, globe", unit: "", scale: "water", lo: 0.55, hi: 0.70, slow: true,
		source:  "Oki & Kanae 2006: 65,500 of the land's 111,000 km3 of rain a year evaporates, 0.59; Trenberth et al. 2007: 73 of 113, 0.65",
		measure: func() float64 { return landWater(globes()).evapShare },
	}},
	{yardstick: yardstick{
		name: "Budyko-Fu shape fitted to the land, globe", unit: "w", scale: "water", lo: 1.8, hi: 3.6, slow: true,
		source:  "Zhang et al. 2004: Fu's w fitted to catchments the world over, 1.8-3.6 for most, 2.6 taken together",
		measure: func() float64 { return landWater(globes()).omega },
	}},
	{yardstick: yardstick{
		name: "seasonal land whose runoff peaks in or just after its wet season, globe", unit: "", scale: "water", lo: 0.8, hi: 1, slow: true,
		source:  "Dettinger & Diaz 2000: rain-fed rivers peak in their basin's wet season or within a few months after it; snowmelt rivers later",
		measure: func() float64 { return landWater(globes()).inOrAfter },
	}},
}

// waterReading is what landWater reads off the land.
type waterReading struct {
	evapShare float64 // what the air takes back of the land's rain
	omega     float64 // Fu's ω the land's years fit best
	// Where the rain is seasonal - half the year's in one phase - the share
	// of the tiles whose runoff is highest in the wettest phase or the next,
	// and how many tiles that is.
	inOrAfter float64
	seasonal  int
	// Where it is seasonal and the year never freezes, so that the phases
	// either side of the wet one have the same rain and the same sun (see
	// atmos.RainCells: the autumn's air is the spring's), the runoff of the
	// phase after the wet one over the phase before it: the soil's memory of
	// the wet season, and nothing else.
	afterOverBefore float64
}

// landWater reads the land's water off gs: its dry ground, the tiles not
// under the sea or a lake and not a river's.
func landWater(gs []*Grid) waterReading {
	return remember("landWater", gs, func() waterReading {
		var r waterReading
		var rain, evap float64
		var phis, index []float64
		var peaked int
		var after, before float64
		for _, g := range gs {
			for i := range g.Tiles {
				if g.sunk(i) || g.Tiles[i].Wet() {
					continue
				}
				p, q := g.Rain(i), g.Runoff(i)
				if p <= 0 {
					continue
				}
				rain += p
				evap += p - q
				if pet := g.pet(i); pet > 0 {
					phis = append(phis, pet/p)
					index = append(index, (p-q)/p)
				}
				wet, ok := wetSeason(g, i)
				if !ok || q < 1 {
					continue
				}
				r.seasonal++
				most := 0
				for k := range atmos.Phases {
					if g.RunoffIn(i, k) > g.RunoffIn(i, most) {
						most = k
					}
				}
				next, last := (wet+1)%atmos.Phases, (wet+atmos.Phases-1)%atmos.Phases
				if most == wet || most == next {
					peaked++
				}
				if (wet == 0 || wet == 2) && g.unfrozen(i) {
					after += g.RunoffIn(i, next)
					before += g.RunoffIn(i, last)
				}
			}
		}
		r.evapShare = evap / rain
		r.omega = fitFu(phis, index)
		if r.seasonal > 0 {
			r.inOrAfter = float64(peaked) / float64(r.seasonal)
		}
		if before > 0 {
			r.afterOverBefore = after / before
		}
		return r
	})
}

// wetSeason is tile i's wettest phase, where half its year's rain or more
// falls in it.
func wetSeason(g *Grid, i int) (int, bool) {
	var total float64
	most := 0
	for k := range atmos.Phases {
		total += g.RainIn(i, k)
		if g.RainIn(i, k) > g.RainIn(i, most) {
			most = k
		}
	}
	return most, total > 0 && g.RainIn(i, most) >= total/2
}

// unfrozen reports whether tile i's ground is over freezing through the
// whole year: its year's mean less its swing.
func (g *Grid) unfrozen(i int) bool {
	if len(g.swing) != len(g.Tiles) {
		return false
	}
	return g.meanOn(i, g.Height[i])-float64(g.swing[i]) > 0
}

// fitFu is the ω of Fu's curve the evaporative indices index, at dryness
// indices phis, fit best in least squares.
func fitFu(phis, index []float64) float64 {
	cost := func(w float64) float64 {
		var s float64
		for j, phi := range phis {
			d := index[j] - (1 + phi - math.Pow(1+math.Pow(phi, w), 1/w))
			s += d * d
		}
		return s
	}
	lo, hi := 1.05, 12.0
	for range 60 {
		a, b := lo+(hi-lo)/3, hi-(hi-lo)/3
		if cost(a) < cost(b) {
			hi = b
		} else {
			lo = a
		}
	}
	return (lo + hi) / 2
}

// The year of every tile of a made valley stays inside Budyko's limits -
// the air takes no more than fell and no more than it could take - and the
// rain and the runoff of its phases add to the year's.
func TestSoilWaterInsideBudykosLimits(t *testing.T) {
	g := yardWorld("valley", 1, DefaultTerms())
	var held, hold, n float64
	for i := range g.Tiles {
		if g.sunk(i) {
			continue
		}
		p, q, pet := g.Rain(i), g.Runoff(i), g.pet(i)
		e := p - q
		if e < -1e-3*p || e > math.Min(p, pet)*1.001+1e-6 {
			t.Fatalf("tile %d: rain %.1f, runoff %.1f, pet %.1f: the air took %.1f", i, p, q, pet, e)
		}
		var fell, shed float64
		for k := range atmos.Phases {
			fell += g.RainIn(i, k)
			shed += g.RunoffIn(i, k)
			if w := g.SoilWater(i, k); w < 0 || w > g.SoilHold(i)*1.0001 {
				t.Fatalf("tile %d phase %d: %.1f mm held in a bucket of %.1f", i, k, w, g.SoilHold(i))
			}
			held += g.SoilWater(i, k) / atmos.Phases
		}
		if math.Abs(shed-q) > 1e-3*math.Max(1, q) || math.Abs(fell-p) > 1e-3*math.Max(1, p) {
			t.Fatalf("tile %d: the phases rained %.3f mm and shed %.3f, the year %.3f and %.3f", i, fell, shed, p, q)
		}
		hold += g.SoilHold(i)
		n++
	}
	r := landWater([]*Grid{g})
	t.Logf("valley: bucket %.0f mm, holding %.0f on the mean; evaporation %.3f of the rain; Fu's ω %.2f; %d seasonal tiles, %.2f peaking in or after the wet season",
		hold/n, held/n, r.evapShare, r.omega, r.seasonal, r.inOrAfter)
}

// A seasonal climate's rivers run higher after its wet season than before
// it, on the same rain and the same sun: the soil holds the wet season's
// water over. Read on the small globes, where the tropics' and subtropics'
// wet seasons are.
func TestSeasonalRiversRunAfterTheRain(t *testing.T) {
	if testing.Short() {
		t.Skip("needs a small globe")
	}
	r := landWater(smallGlobes(4))
	t.Logf("small globes: %d seasonal tiles, %.2f peaking in or after the wet season; runoff after the wet season %.2f times before it, where it never freezes",
		r.seasonal, r.inOrAfter, r.afterOverBefore)
	if r.afterOverBefore <= 1 {
		t.Errorf("the rivers ran %.2f times as high after the wet season as before it", r.afterOverBefore)
	}
}
