package veg

import (
	"math"
	"testing"

	"github.com/LukasSelin/terra/internal/atmos"
)

// sunAt is the light plants grow by at latitude lat through each phase, in
// MJ/m² over the phase, read as the land reads it.
func sunAt(lat float64) (sun [Phases]float64) {
	top := atmos.PhaseSun(atmos.Today(), lat)
	for k := range Phases {
		sun[k] = top[k] * 0.55 * 0.48 * 365.25 * 86400 / Phases / 1e6
	}
	return sun
}

// place is a year somewhere on the earth, with the water its soil gives the
// air in each phase.
type place struct {
	name       string
	lat, mean  float64
	swing      float64
	herb, wood [Phases]float64
	snow       [Phases]float64
	treeless   bool
	trees, lai [2]float64 // the tree cover and leaf area it should come to
	mass       [2]float64 // and its carbon, kg a square metre
	dominant   PFT
}

var wet = [Phases]float64{1, 1, 1, 1}

// The places: their years are the climate stations' (Walter and Lieth's
// atlas, read to the degree), their water a share of the potential the
// stations' water balances give (Willmott and others' climatology, read to
// the tenth), and what they should grow is the biome's: tree cover from the
// MODIS vegetation continuous fields (Hansen and others, 2003), leaf area
// from MODIS (Myneni and others, 2002) and carbon from the IPCC's tier-1
// tables (Ruesch and Gibbs, 2008), each widened to the spread of the biome.
var places = []place{
	{name: "Manaus, tropical rainforest", lat: -3, mean: 27, swing: -1, herb: wet, wood: wet,
		trees: [2]float64{0.8, 0.951}, lai: [2]float64{4.5, 7}, mass: [2]float64{12, 25}, dominant: TropicalEvergreen},
	{name: "Kano, savanna", lat: 12, mean: 26, swing: 4, herb: [Phases]float64{0.1, 0.15, 0.9, 0.5}, wood: [Phases]float64{0.15, 0.25, 0.95, 0.7},
		trees: [2]float64{0, 0.6}, lai: [2]float64{0.5, 3}, mass: [2]float64{0.5, 8}},
	{name: "Bonn, temperate broadleaf forest", lat: 51, mean: 10, swing: 8, herb: [Phases]float64{1, 1, 0.8, 1}, wood: [Phases]float64{1, 1, 0.9, 1},
		trees: [2]float64{0.7, 0.95}, lai: [2]float64{3.5, 7}, mass: [2]float64{7, 20}, dominant: TemperateBroadleaf},
	{name: "Yakutsk-ish taiga", lat: 62, mean: -6, swing: 22, herb: [Phases]float64{1, 1, 0.7, 1}, wood: [Phases]float64{1, 1, 0.85, 1}, snow: [Phases]float64{1, 0.8, 0, 0.5},
		trees: [2]float64{0.5, 0.95}, lai: [2]float64{1.5, 4}, mass: [2]float64{3, 10}, dominant: BorealNeedleleaf},
	{name: "Kansas, temperate grassland", lat: 39, mean: 12, swing: 13, herb: [Phases]float64{0.9, 0.7, 0.35, 0.5}, wood: [Phases]float64{0.9, 0.75, 0.4, 0.55}, snow: [Phases]float64{0.2, 0, 0, 0},
		trees: [2]float64{0, 0.3}, lai: [2]float64{0.7, 2.5}, mass: [2]float64{0.3, 3}},
	{name: "Phoenix, hot desert", lat: 33, mean: 23, swing: 10, herb: [Phases]float64{0.15, 0.08, 0.05, 0.08}, wood: [Phases]float64{0.18, 0.1, 0.06, 0.1},
		trees: [2]float64{0, 0.05}, lai: [2]float64{0, 0.5}, mass: [2]float64{0, 1}},
	{name: "Barrow-ish tundra", lat: 70, mean: -10, swing: 16, herb: wet, wood: wet, snow: [Phases]float64{1, 1, 0.1, 0.8}, treeless: true,
		trees: [2]float64{0, 0}, lai: [2]float64{0.2, 1.5}, mass: [2]float64{0.1, 1.5}, dominant: Tundra},
}

func (pl place) climate() Climate {
	sun := sunAt(pl.lat)
	return Climate{Mean: pl.mean, Swing: pl.swing, Sun: sun, Take: sun, Snow: pl.snow, Herb: pl.herb, Wood: pl.wood, Treeless: pl.treeless}
}

// steady is a place's state run from BIOME4's answer to the steady one.
func steady(c Climate) (State, [PFTs]Potential) {
	y := Read(&c)
	var pot [PFTs]Potential
	for p := range PFTs {
		pot[p] = y.Potential(p)
	}
	s := Equilibrium(&pot)
	f := y.Fire()
	Spin(&s, &pot, &f, 300)
	return s, pot
}

// What the places grow, against what the biomes they are on grow.
func TestThePlacesGrowTheirBiomes(t *testing.T) {
	for _, pl := range places {
		s, pot := steady(pl.climate())
		var lai, mass, npp float64
		best := PFT(0)
		for p := range PFTs {
			lai += s.Cover[p] * pot[p].LAI
			mass += s.Mass[p]
			npp += s.Cover[p] * pot[p].NPP
			if s.Mass[p] > s.Mass[best] {
				best = p
			}
		}
		t.Logf("%-34s trees %.2f, LAI %.2f, carbon %.2f kg/m2, NPP %.3f kg/m2/yr; most carbon in %s", pl.name, s.Trees(), lai, mass, npp, Kinds[best].Name)
		for p := range PFTs {
			if pot[p].Establish || s.Cover[p] > 0 {
				t.Logf("    %-22s cover %.2f mass %6.2f  LAI %.2f NPP %.3f surplus %.3f", Kinds[p].Name, s.Cover[p], s.Mass[p], pot[p].LAI, pot[p].NPP, pot[p].Surplus)
			}
		}
		check := func(what string, v float64, r [2]float64) {
			if v < r[0] || v > r[1] {
				t.Errorf("%s: %s %.2f, want %.2f-%.2f", pl.name, what, v, r[0], r[1])
			}
		}
		check("tree cover", s.Trees(), pl.trees)
		check("leaf area", lai, pl.lai)
		check("carbon", mass, pl.mass)
		if pl.dominant != 0 || pl.name[:6] == "Manaus" {
			if best != pl.dominant {
				t.Errorf("%s: most carbon in %s, want %s", pl.name, Kinds[best].Name, Kinds[pl.dominant].Name)
			}
		}
	}
}

// A wood whose ground dries dies back to what the dry ground holds: the
// temperate forest, its water cut to the grassland's, comes in a few decades
// to the grassland's state, and the trees' carbon goes with them.
func TestAWoodDiesBackWhereItsGroundDries(t *testing.T) {
	forest, grass := places[2], places[4]
	s, _ := steady(forest.climate())
	before := s.Trees()
	dry := forest.climate()
	dry.Herb, dry.Wood = grass.herb, grass.wood
	for k := range dry.Herb {
		dry.Herb[k] *= 0.5
		dry.Wood[k] *= 0.5
	}
	y := Read(&dry)
	var pot [PFTs]Potential
	for p := range PFTs {
		pot[p] = y.Potential(p)
	}
	f := y.Fire()
	Grow(&s, &pot, &f, 30)
	after := s.Trees()
	t.Logf("tree cover %.2f, thirty years after its water was cut %.2f", before, after)
	if before < 0.7 || after > 0.2*before {
		t.Errorf("tree cover %.2f went to %.2f", before, after)
	}
}

// A frost the tropical trees do not survive kills them, and what stands
// after is what the cold ground holds.
func TestAFrostKillsTheTropicalTrees(t *testing.T) {
	s, _ := steady(places[0].climate())
	cold := places[0].climate()
	cold.Mean, cold.Swing = 14, -8
	y := Read(&cold)
	var pot [PFTs]Potential
	for p := range PFTs {
		pot[p] = y.Potential(p)
	}
	f := y.Fire()
	Grow(&s, &pot, &f, 20)
	if c := s.Cover[TropicalEvergreen]; c > 0.01 {
		t.Errorf("tropical evergreen covers %.3f twenty years into frost", c)
	}
}

// The best leaf area is where one more unit gains what it costs: checked
// against a search over leaf area.
func TestTheBestLeafAreaIsTheBest(t *testing.T) {
	for _, pl := range places {
		c := pl.climate()
		y := Read(&c)
		for p := range PFTs {
			pot := y.Potential(p)
			if pot.LAI <= 0 {
				continue
			}
			k := &Kinds[p]
			// The surplus at leaf area l, by the same sums Potential takes.
			at := func(l float64) float64 {
				f := 1 - math.Exp(-extinction*l)
				var gpp, leafQ, woodQ float64
				water := &c.Herb
				if k.Woody {
					water = &c.Wood
				}
				for ph := range Phases {
					woodQ += q10(y.phase[ph]) / Phases
					on := true
					switch k.phen {
					case summergreen:
						on = y.phase[ph] > k.leafWarm
					case raingreen:
						on = water[ph] >= leafWet && y.phase[ph] > 0
					}
					if !on {
						continue
					}
					leafQ += q10(y.phase[ph]) / Phases
					var fT float64
					for j := range steps {
						fT += ramp(y.temp[ph][j], &k.temp)
					}
					light := c.Sun[ph] * fT / steps
					if !k.Tree {
						light *= 1 - c.Snow[ph]
					}
					gpp += k.lue * light / 1000 * math.Min(f, water[ph]*k.wue)
				}
				npp := (1 - growthCost) * (gpp - k.leafKeep*leafQ*l/1000 - k.woodKeep*woodQ/1000)
				return npp - k.leafTurn*l/1000
			}
			best := at(pot.LAI)
			for l := 0.0; l <= k.laiMost; l += 0.01 {
				if at(l) > best+1e-9 {
					t.Errorf("%s, %s: surplus %.5f at LAI %.2f over %.5f at the chosen %.3f", pl.name, k.Name, at(l), l, best, pot.LAI)
					break
				}
			}
		}
	}
}
