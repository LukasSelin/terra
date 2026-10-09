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
// air in each phase, and the rain and the wind its fires read.
type place struct {
	name       string
	lat, mean  float64
	swing      float64
	herb, wood [Phases]float64
	snow       [Phases]float64
	rain       [Phases]float64 // mm in each phase, whose showers bring the lightning
	wind       float64         // m/s near the ground
	treeless   bool
	// open is a place its history has left open ground, run from it rather
	// than from BIOME4's answer; and every, where it is set, is the years
	// between the fires a game's people light on it (Fire.Burn, as
	// SetBurning lights them), on top of the lightning's.
	open       bool
	every      float64
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
// Their rain is the stations' by phase (Walter and Lieth's atlas, to the ten
// mm), and their wind a station's mean: every place reads L4's fires.
var places = []place{
	{name: "Manaus, tropical rainforest", lat: -3, mean: 27, swing: -1, herb: wet, wood: wet,
		rain: [Phases]float64{700, 600, 300, 600}, wind: 2,
		trees: [2]float64{0.8, 0.951}, lai: [2]float64{4.5, 7}, mass: [2]float64{12, 25}, dominant: TropicalEvergreen},
	{name: "Kano, savanna", lat: 12, mean: 26, swing: 4, herb: [Phases]float64{0.1, 0.15, 0.9, 0.5}, wood: [Phases]float64{0.15, 0.25, 0.95, 0.7},
		rain: [Phases]float64{5, 60, 600, 135}, wind: 4,
		trees: [2]float64{0, 0.6}, lai: [2]float64{0.5, 3}, mass: [2]float64{0.5, 8}},
	{name: "Bonn, temperate broadleaf forest", lat: 51, mean: 10, swing: 8, herb: [Phases]float64{1, 1, 0.8, 1}, wood: [Phases]float64{1, 1, 0.9, 1},
		rain: [Phases]float64{150, 160, 220, 170}, wind: 3,
		trees: [2]float64{0.7, 0.95}, lai: [2]float64{3.5, 7}, mass: [2]float64{7, 20}, dominant: TemperateBroadleaf},
	{name: "Yakutsk-ish taiga", lat: 62, mean: -6, swing: 22, herb: [Phases]float64{1, 1, 0.7, 1}, wood: [Phases]float64{1, 1, 0.85, 1}, snow: [Phases]float64{1, 0.8, 0, 0.5},
		rain: [Phases]float64{40, 60, 150, 70}, wind: 4,
		trees: [2]float64{0.5, 0.95}, lai: [2]float64{1.5, 4}, mass: [2]float64{3, 10}, dominant: BorealNeedleleaf},
	kansas,
	{name: "Phoenix, hot desert", lat: 33, mean: 23, swing: 10, herb: [Phases]float64{0.15, 0.08, 0.05, 0.08}, wood: [Phases]float64{0.18, 0.1, 0.06, 0.1},
		rain: [Phases]float64{70, 40, 50, 50}, wind: 3,
		trees: [2]float64{0, 0.05}, lai: [2]float64{0, 0.5}, mass: [2]float64{0, 1}},
	{name: "Barrow-ish tundra", lat: 70, mean: -10, swing: 16, herb: wet, wood: wet, snow: [Phases]float64{1, 1, 0.1, 0.8}, treeless: true,
		rain: [Phases]float64{10, 10, 60, 40}, wind: 5,
		trees: [2]float64{0, 0}, lai: [2]float64{0.2, 1.5}, mass: [2]float64{0.1, 1.5}, dominant: Tundra},
	kansasUnburned,
}

// kansas is the tallgrass prairie's year (Manhattan, Kansas, by Konza
// Prairie), burned by people every second year. Its 850 mm would grow a
// wood: with BIOME1's drought limits read on the bucket's scale (#125) the
// temperate trees establish on its water. It is a grassland because it
// burns, every one to three years before the plough, and is grazed (Knapp
// and others, 1998), and many of those fires were lit by its people (Knapp
// and others, 1998; Anderson, 2006). A world has nobody on it, so those
// fires are a game's to light (SetBurning): here, every second year on top
// of the lightning, through L4's own fuel, spread, kill and trap. It is
// run from its open ground, where the prairie has stood since it spread
// east in the mid-Holocene's drier millennia (Webb, Cushing and Wright,
// 1983).
var kansas = place{name: "Kansas, burned every second year", lat: 39, mean: 12, swing: 13, herb: [Phases]float64{0.9, 0.7, 0.35, 0.5}, wood: [Phases]float64{0.9, 0.75, 0.4, 0.55}, snow: [Phases]float64{0.2, 0, 0, 0},
	rain: [Phases]float64{80, 250, 340, 190}, wind: 5, open: true, every: 2,
	trees: [2]float64{0, 0.3}, lai: [2]float64{0.7, 2.5}, mass: [2]float64{0.3, 3}, dominant: C3Grass}

// kansasUnburned is the same year with nobody on it: only its lightning
// burns it, read through L4 off its rain and wind, a seventh of the prairie
// a year at first. That is too seldom. The trees come in, close over the
// grass and shade out its fuel, and nothing burns after. Konza's watersheds
// burned every four years or less often do the same, going to shrubs and
// then trees within decades (Briggs and others, 2005), and past a fire
// every three or four years the woody state takes over and holds itself
// (Ratajczak and others, 2014). So unburned it is an open broadleaf wood,
// the oak woodland the prairie's edge turns to when its fires stop. Its
// bands are its reading's, widened: trees 0.83, LAI 1.6, 3.8 kg C/m².
var kansasUnburned = func() place {
	pl := kansas
	pl.name, pl.every = "Kansas, nobody burning it", 0
	pl.trees, pl.lai, pl.mass, pl.dominant = [2]float64{0.6, 0.95}, [2]float64{1, 2.5}, [2]float64{2.5, 6}, TemperateBroadleaf
	return pl
}()

func (pl place) climate() Climate {
	sun := sunAt(pl.lat)
	c := Climate{Mean: pl.mean, Swing: pl.swing, Sun: sun, Take: sun, Snow: pl.snow, Herb: pl.herb, Wood: pl.wood, Treeless: pl.treeless, Rain: pl.rain}
	for k := range c.Wind {
		c.Wind[k] = pl.wind
	}
	return c
}

// fire is the place's fires: L4's, lit by its lightning, and its people's
// where it has them.
func (pl place) fire(y *Year) Fire {
	f := y.Fire()
	f.Burn(pl.every)
	return f
}

// run is a place's state run to the steady one under its fires, from
// BIOME4's answer or, open, from its open ground with the trees taken off
// it; and the share of it the steady state's fires burn a year.
func (pl place) run(open bool) (State, [PFTs]Potential, float64) {
	c := pl.climate()
	y := Read(&c)
	var pot [PFTs]Potential
	for p := range PFTs {
		pot[p] = y.Potential(p)
	}
	f := pl.fire(&y)
	s := Equilibrium(&pot)
	if open {
		for p := range PFTs {
			if Kinds[p].Tree {
				s.Cover[p], s.Mass[p] = 0, 0
			}
		}
	}
	Spin(&s, &pot, &f, spinMost)
	return s, pot, Burned(&s, &pot, &f)
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
	Spin(&s, &pot, &f, spinMost)
	return s, pot
}

// spinMost is as many years as the land spins a state for: see vegSpin.
const spinMost = 10000

// Every place's spin settles, from BIOME4's answer and from open ground,
// well inside the years it is given, and stays settled: a year more, or a
// century more, moves no type's cover by more than the spin's test of
// steady allowed. The open ground's types grow at up to sixteen times their
// cover a year, and taken a year at a time with their room closing at the
// year's start they overshot it and fell back, a year up and a year down
// for ever (#136).
func TestTheSpinSettles(t *testing.T) {
	for _, pl := range places {
		for _, open := range []bool{false, true} {
			c := pl.climate()
			y := Read(&c)
			var pot [PFTs]Potential
			for p := range PFTs {
				pot[p] = y.Potential(p)
			}
			f := pl.fire(&y)
			s := Equilibrium(&pot)
			if open {
				for p := range PFTs {
					if Kinds[p].Tree {
						s.Cover[p], s.Mass[p] = 0, 0
					}
				}
			}
			years := Spin(&s, &pot, &f, spinMost)
			if years >= spinMost {
				t.Errorf("%s (open %v): the spin ran its %d years out", pl.name, open, spinMost)
				continue
			}
			was := s
			Grow(&s, &pot, &f, 1)
			year := s
			Grow(&s, &pot, &f, 99)
			for p := range PFTs {
				if d := math.Abs(year.Cover[p] - was.Cover[p]); d > stillCover {
					t.Errorf("%s (open %v): %s moves %.2g of the ground the year after the spin", pl.name, open, Kinds[p].Name, d)
				}
				if d := math.Abs(s.Cover[p] - was.Cover[p]); d > 10*stillCover {
					t.Errorf("%s (open %v): %s moves %.2g of the ground the century after the spin", pl.name, open, Kinds[p].Name, d)
				}
			}
			t.Logf("%-34s open %-5v settled in %4d years", pl.name, open, years)
		}
	}
}

// What the places grow, against what the biomes they are on grow.
func TestThePlacesGrowTheirBiomes(t *testing.T) {
	for _, pl := range places {
		s, pot, burned := pl.run(pl.open)
		t.Logf("%-34s %.3f burned a year", pl.name, burned)
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

// A desert carries its plants sparse, not none: its shrubs stand apart on the
// water of the ground between them, a twentieth to a third or so of its
// ground on an arid year (UNEP's aridity index 0.05-0.2) and next to none in
// a hyper-arid core (under 0.05), each crown in full leaf; and where the
// water would pay for leaves over all the ground, the stand covers it as it
// did.
func TestADesertCarriesItsSparseCover(t *testing.T) {
	phoenix := places[5]
	core := phoenix
	core.name = "hyper-arid core"
	core.herb = [Phases]float64{0.02, 0.01, 0.005, 0.01}
	core.wood = [Phases]float64{0.03, 0.015, 0.01, 0.015}
	for _, c := range []struct {
		pl     place
		lo, hi float64
	}{
		{phoenix, 0.05, 0.4},
		{core, 0, 0.1},
	} {
		s, pot := steady(c.pl.climate())
		var cover float64
		for p := range PFTs {
			cover += s.Cover[p]
		}
		t.Logf("%-20s cover %.3f; shrub room %.3f, a crown's LAI %.2f, NPP %.3f", c.pl.name, cover, pot[Shrub].Room, pot[Shrub].LAI, pot[Shrub].NPP)
		if cover < c.lo || cover > c.hi {
			t.Errorf("%s: cover %.3f, want %.2f-%.2f", c.pl.name, cover, c.lo, c.hi)
		}
		if cover > 0 && pot[Shrub].LAI < 0.5 {
			t.Errorf("%s: the shrubs' crowns put up a leaf area of %.2f, not a crown's", c.pl.name, pot[Shrub].LAI)
		}
	}
	// A grassland's water holds its grass over the whole of it.
	_, pot := steady(places[4].climate())
	if pot[C3Grass].Room != 1 {
		t.Errorf("Kansas's grass stands on %.2f of the ground, want all of it", pot[C3Grass].Room)
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
			// The surplus at leaf area l over a crown, by the same sums
			// Potential takes.
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
					// A sparse stand's crown has the water of its share of
					// the ground (see sparse).
					w := water[ph]
					if pot.Room < 1 {
						w = math.Min(1, w/pot.Room)
					}
					gpp += k.lue * light / 1000 * math.Min(f, w*k.wue)
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
