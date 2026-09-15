package terra

import (
	"fmt"
	"math"
	"os"
	"sort"
	"testing"
	"unsafe"
)

// fertilityReading is what a settlement reads off a map's ground, in the terms
// the fertility tests elsewhere ask it: the mean over the dry ground, the
// riverbank against inland (grid_test.go), the valley floor against the
// hillside (relief_test.go), and how much of the ground is as good as a field
// wants.
type fertilityReading struct {
	mean, bank, inland, floor, hill, good, rich float64
}

func readFertility(g *Grid) fertilityReading {
	var r fertilityReading
	var n, nb, ni, nf, nh float64
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if t.Wet() || t.Terrain.Tidal() {
			continue
		}
		f := g.Fertility[i]
		r.mean += f
		r.rich += g.Rich[i]
		n++
		if f >= 0.6 {
			r.good++
		}
		if g.HasNeighbor(g.PosOf(i), func(n *Tile) bool { return n.Terrain == Water }) {
			r.bank, nb = r.bank+f, nb+1
		} else {
			r.inland, ni = r.inland+f, ni+1
		}
		switch {
		case t.Drain < 2:
			r.floor, nf = r.floor+f, nf+1
		case t.Drain > 20:
			r.hill, nh = r.hill+f, nh+1
		}
	}
	div := func(a, b float64) float64 {
		if b == 0 {
			return 0
		}
		return a / b
	}
	return fertilityReading{div(r.mean, n), div(r.bank, nb), div(r.inland, ni), div(r.floor, nf), div(r.hill, nh), div(r.good, n), div(r.rich, n)}
}

func (r fertilityReading) String() string {
	return fmt.Sprintf("mean %.4f  bank %.4f  inland %.4f  floor %.4f  hill %.4f  good %.4f  rich %.4f",
		r.mean, r.bank, r.inland, r.floor, r.hill, r.good, r.rich)
}

// The fertility a settlement is handed, printed rather than asserted, so that
// a change to the soil can be held against what the ground read before it:
//
//	TERRA_SOIL=1 go test -run TestSoilReadings -v
func TestSoilReadings(t *testing.T) {
	if os.Getenv("TERRA_SOIL") == "" {
		t.Skip("set TERRA_SOIL=1 to print the soil readings")
	}
	var fresh, aged fertilityReading
	add := func(a *fertilityReading, r fertilityReading, k float64) {
		a.mean += r.mean / k
		a.bank += r.bank / k
		a.inland += r.inland / k
		a.floor += r.floor / k
		a.hill += r.hill / k
		a.good += r.good / k
		a.rich += r.rich / k
	}
	const seeds = 5
	for seed := uint64(1); seed <= seeds; seed++ {
		w := NewLand(seed, DefaultTerms())
		r := readFertility(w.Grid)
		fmt.Printf("valley %d fresh:   %v\n", seed, r)
		add(&fresh, r, seeds)
		for age := 0; age < 20; age++ {
			w.Erode()
		}
		r = readFertility(w.Grid)
		fmt.Printf("valley %d 20 ages: %v\n", seed, r)
		add(&aged, r, seeds)
	}
	fmt.Printf("valleys fresh:   %v\n", fresh)
	fmt.Printf("valleys 20 ages: %v\n", aged)
	for seed := uint64(1); seed <= 2; seed++ {
		fmt.Printf("made valley %d:   %v\n", seed, readFertility(NewLand(seed, historyConfig(16)).Grid))
		fmt.Printf("small globe %d:   %v\n", seed, readFertility(NewLand(seed, smallGlobe()).Grid))
	}
	soilStateReadings()
}

// soilStateReadings prints what time has made of the soil on a few maps: the
// surface ages, the leaching, the carbonate and salt, and the carbon under
// each cover.
func soilStateReadings() {
	show := func(name string, g *Grid) {
		var ages []float64
		var leach, n, limed, salted float64
		carbon := map[Terrain][2]float64{}
		for i := range g.Tiles {
			t := &g.Tiles[i]
			if !forms(t) || t.Soil <= 0 {
				continue
			}
			ages = append(ages, float64(t.Exposed))
			leach += t.Leaching()
			n++
			if t.Carbonate() > 1 {
				limed++
			}
			if t.Salinity() > 1 {
				salted++
			}
			c := carbon[t.Terrain]
			carbon[t.Terrain] = [2]float64{c[0] + float64(t.Carbon), c[1] + 1}
		}
		if n == 0 {
			fmt.Printf("%s: no soil\n", name)
			return
		}
		sort.Float64s(ages)
		q := func(f float64) float64 { return ages[int(f*float64(len(ages)-1))] / 1e3 }
		fmt.Printf("%s: exposed kyr p10 %.1f p50 %.1f p90 %.1f  leached %.3f  limed %.3f  salted %.3f  carbon",
			name, q(0.1), q(0.5), q(0.9), leach/n, limed/n, salted/n)
		for _, k := range []Terrain{Grass, Forest, Field} {
			if c := carbon[k]; c[1] > 0 {
				fmt.Printf(" %v %.2f", k, c[0]/c[1])
			}
		}
		fmt.Println()
	}
	w := NewLand(1, DefaultTerms())
	show("valley 1 fresh", w.Grid)
	for age := 0; age < 20; age++ {
		w.Erode()
	}
	show("valley 1 20 ages", w.Grid)
	show("made valley 1", NewLand(1, historyConfig(16)).Grid)
	show("small globe 1", NewLand(1, smallGlobe()).Grid)
}

// The state fits in the padding the tile already had: a map pays nothing a
// tile for its soil's age and chemistry. See memory.go.
func TestTheSoilStateCostsATileNothing(t *testing.T) {
	if got := unsafe.Sizeof(Tile{}); got != 72 {
		t.Errorf("a tile is %d bytes; it was 72 before the soil kept its age", got)
	}
}

// one is a grid of a single open tile of basalt under the given water: the
// rain on it and what of that goes through the soil, in mm a year.
func one(rain, runoff float64) *Grid {
	g := NewGrid(1, 1)
	g.rain, g.runoff = []float64{rain}, []float64{runoff}
	t := &g.Tiles[0]
	t.Terrain, t.Bedrock, t.Soil, t.Drain = Grass, Basalt, 1, 50
	return g
}

// Chadwick and others' (1999) chronosequence: basalt under two and a half
// metres of rain keeps its bases for the first few thousand years, has lost
// most of them by a hundred and fifty thousand, and nearly all by four
// million. And the same ages under a dry sky leach far less.
func TestBasaltUnderHeavyRainLosesItsBasesWithAge(t *testing.T) {
	leached := func(rain, runoff, years float64) float64 {
		g := one(rain, runoff)
		g.laySoilState(0, 1, 1, math.Inf(1))
		clearSoil(&g.Tiles[0])
		g.ripenSoil(0, years)
		return g.Tiles[0].Leaching()
	}
	young, middle, old := leached(2500, 1800, 2e3), leached(2500, 1800, 150e3), leached(2500, 1800, 4.1e6)
	if !(young < 0.15 && middle > 0.6 && old > 0.95) {
		t.Errorf("wet basalt leached %.2f at 2 kyr, %.2f at 150 kyr, %.2f at 4.1 Myr", young, middle, old)
	}
	if dry := leached(300, 10, 150e3); !(dry < middle/4) {
		t.Errorf("at 150 kyr dry basalt leached %.2f against wet %.2f", dry, middle)
	}
}

// Carbonate builds up where the air takes back more than the rain gives, and
// salt only where it takes back far more; where water goes through the soil,
// neither does.
func TestTheDryYearsLeaveLimeAndSalt(t *testing.T) {
	stock := func(rain, runoff float64) (lime, salt float64) {
		g := one(rain, runoff)
		g.ripenSoil(0, 50e3)
		return g.Tiles[0].Carbonate(), g.Tiles[0].Salinity()
	}
	// wetness is read off the rain and what the air could take, which a grid
	// with no air reads as even; set it by hand through the climate.
	c := func(wetness, runoff float64) (lime, salt float64) {
		g := one(1, runoff)
		t := &g.Tiles[0]
		pc := g.pedoClimateOf(0)
		pc.wetness = wetness
		for k := 0; k < 50; k++ {
			t.setCarbonate(gathered(t.Carbonate(), limeRate*dryness(pc.wetness, limeWetter, limeDrier), pc.water, limeWater, 1e3))
			t.setSalinity(gathered(t.Salinity(), saltRate*dryness(pc.wetness, saltWetter, saltDrier), pc.water, saltWater, 1e3))
		}
		return t.Carbonate(), t.Salinity()
	}
	if lime, salt := stock(800, 300); lime != 0 || salt != 0 {
		t.Errorf("a humid soil built %.2f kg of carbonate and %.2f of salt", lime, salt)
	}
	steppeLime, steppeSalt := c(0.35, 5)
	desertLime, desertSalt := c(0.08, 0.5)
	if !(steppeLime > 20 && steppeSalt == 0) {
		t.Errorf("a steppe soil holds %.1f kg of carbonate and %.2f of salt over 50 kyr", steppeLime, steppeSalt)
	}
	if !(desertSalt > 5 && desertLime >= steppeLime) {
		t.Errorf("a desert soil holds %.1f kg of carbonate and %.2f of salt over 50 kyr", desertLime, desertSalt)
	}
}

// Under the same sky grass keeps its carbon and a field loses more than half
// of it to the plough (Guo and Gifford 2002); conifers take the bases faster
// than grass does.
func TestWhatGrowsLeavesItsMarkOnTheSoil(t *testing.T) {
	under := func(k Terrain, temp float64) *Tile {
		g := one(900, 400)
		g.Tiles[0].Terrain = k
		c := g.pedoClimateOf(0)
		c.temp = temp
		cv := g.coverOf(0, temp)
		level, _ := g.carbonLevel(0, c, cv)
		g.Tiles[0].Carbon = float32(level)
		l, _ := g.leachLevel(0, c, cv, 20e3)
		rate := c.water * cv.acid / leachWater
		g.Tiles[0].setLeaching(l * -math.Expm1(-rate/l*20e3))
		return &g.Tiles[0]
	}
	grass, field := under(Grass, MeanTemp), under(Field, MeanTemp)
	if share := float64(field.Carbon / grass.Carbon); !(share > 0.3 && share < 0.5) {
		t.Errorf("a field holds %.2f of a grassland's carbon", share)
	}
	pine := under(Forest, -1)
	cold := under(Grass, -1)
	if !(pine.Leaching() > 1.3*cold.Leaching()) {
		t.Errorf("under conifers the soil leached %.3f in 20 kyr, under grass %.3f", pine.Leaching(), cold.Leaching())
	}
}

// Ground laid on a soil is younger than the soil, in the share it comes in,
// and ground cut through to the rock starts again.
func TestNewGroundIsYoungGround(t *testing.T) {
	g := one(900, 400)
	g.ripenSoil(0, 100e3)
	t0 := g.Tiles[0]
	tl := &g.Tiles[0]
	mix(tl, 1, [Grains]float64{Silt: 1})
	if got := float64(tl.Exposed); math.Abs(got-50e3) > 1 {
		t.Errorf("a metre laid on a metre of soil 100 kyr old leaves it %.0f years old", got)
	}
	if !(tl.Leaching() < t0.Leaching()) || tl.Carbon != t0.Carbon {
		t.Errorf("laid on: leaching %.3f from %.3f, carbon %.2f from %.2f", tl.Leaching(), t0.Leaching(), tl.Carbon, t0.Carbon)
	}
	strip(tl, 0.5)
	if math.Abs(float64(tl.Carbon/t0.Carbon)-0.5) > 1e-6 {
		t.Errorf("half the soil taken leaves %.2f of its carbon", tl.Carbon/t0.Carbon)
	}
	strip(tl, 1)
	if tl.Exposed != 0 || tl.Carbon != 0 || tl.Leached != 0 {
		t.Errorf("soil cut to the rock keeps %+v", *tl)
	}
}

// On a made valley the soil's age follows the ground: the hollows the creep
// fills are older than the crests it takes from, the floors the river renews
// are young, and nothing under water has any. And it is still so after the
// weather has had twenty ages of it.
func TestTheSoilIsOldWhereTheGroundIsStill(t *testing.T) {
	check := func(name string, g *Grid, fresh bool) {
		t.Helper()
		var crest, hollow, floor, nc, nh, nf float64
		for i := range g.Tiles {
			tl := &g.Tiles[i]
			if !forms(tl) {
				if tl.Exposed != 0 || tl.Carbon != 0 {
					t.Fatalf("%s: %v tile %d has a soil %+v", name, tl.Terrain, i, *tl)
				}
				continue
			}
			if tl.Mark != None || tl.Soil <= 0 {
				continue
			}
			age := float64(tl.Exposed)
			switch round := roundOver(g, i); {
			case tl.Drain < 2:
				floor, nf = floor+age, nf+1
			case round < -2:
				crest, nc = crest+age, nc+1
			case round > 2:
				hollow, nh = hollow+age, nh+1
			}
		}
		if nc == 0 || nh == 0 || nf == 0 {
			t.Fatalf("%s: %v crests, %v hollows, %v floors", name, nc, nh, nf)
		}
		crest, hollow, floor = crest/nc, hollow/nh, floor/nf
		// A map's floors are read as terraces when it is made; ground the ages
		// then bring within the river's reach keeps the age it had, less what
		// the floods lay on it, so after them the floors are only asked to be
		// younger than the hollows.
		young := floor < crest
		if !fresh {
			young = floor < hollow
		}
		if !(hollow > 2*crest && young) {
			t.Errorf("%s: soils %.0f years old on the crests, %.0f in the hollows, %.0f on the floors", name, crest, hollow, floor)
		}
	}
	w := NewLand(2, DefaultTerms())
	check("fresh", w.Grid, true)
	for age := 0; age < 20; age++ {
		w.Erode()
	}
	check("twenty ages on", w.Grid, false)
}
