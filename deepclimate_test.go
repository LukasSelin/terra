package terra

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/LukasSelin/terra/internal/atmos"
	"github.com/LukasSelin/terra/internal/phase"
)

// The deep-time climate's cost study (#57, docs/deep-time-climate.md): a
// history made with deepClimate off and under each of its variants, timed
// in total, by epoch and by pass, with each epoch's climate and the ground
// it leaves. It makes several globes and is run by hand:
//
//	TERRA_DEEP_CLIMATE=1 TERRA_PHASES=1 go test -run TestDeepClimateCost -count=1 -v -timeout 60m .
//
// TERRA_DEEP_CLIMATE_TERMS is small, globe or both (the default), and
// TERRA_DEEP_CLIMATE_RUNS how many times each variant is made (one).

// The switch is off, and a history with it off reads no epoch's climate.
func TestDeepClimateIsOff(t *testing.T) {
	if deepClimate.on {
		t.Fatal("the deep-time climate's switch is on outside its study")
	}
	g := NewGrid(64, 32)
	g.Wrap = true
	g.air = NewClimateOn(Terms{Width: 64, Height: 32, Wrap: true}).airFor(g, 1)
	if newEpochClimate(g) != nil {
		t.Fatal("an epoch climate was made with the switch off")
	}
}

// The land of each band is read off the grid's rows: a grid all above its
// sea is land in every band, and one with only its northern quarter up is
// land only in the bands that quarter covers.
func TestLandBandsReadTheRows(t *testing.T) {
	g := NewGrid(64, 32)
	g.Wrap = true
	g.air = NewClimateOn(Terms{Width: 64, Height: 32, Wrap: true}).airFor(g, 1)
	g.base = 0
	for i := range g.Height {
		g.Height[i] = 1
	}
	land, _ := landBands(g)
	for k, l := range land {
		if math.Abs(l-1) > 1e-12 {
			t.Fatalf("band %d of an all-land grid is %.3f land", k, l)
		}
	}
	for i := range g.Height {
		if i/g.W >= g.H/4 {
			g.Height[i] = -1
		}
	}
	land, _ = landBands(g)
	if land[zonalBand(80)] != 1 || land[zonalBand(0)] != 0 || land[zonalBand(-60)] != 0 {
		t.Fatalf("a northern quarter's land read as %.2f at 80N, %.2f at the equator, %.2f at 60S",
			land[zonalBand(80)], land[zonalBand(0)], land[zonalBand(-60)])
	}
}

type deepVariant struct {
	name  string
	terms deepClimateTerms
}

var deepVariants = []deepVariant{
	{"off (today)", deepClimateTerms{}},
	{"EBM each epoch", deepClimateTerms{on: true}},
	{"EBM each epoch, warm 3 yr", deepClimateTerms{on: true, warm: true, years: 3}},
	{"EBM every other epoch", deepClimateTerms{on: true, every: 2}},
	{"EBM each epoch, weather kept", deepClimateTerms{on: true, keepWeather: true}},
	{"shift by land share", deepClimateTerms{on: true, shift: true}},
	{"EBM each epoch, air 2x coarser", deepClimateTerms{on: true, coarser: 2}},
	{"today's air, 2x coarser", deepClimateTerms{on: true, today: true, coarser: 2}},
	{"today's air +0.01 C (noise floor)", deepClimateTerms{on: true, today: true, nudge: 0.01}},
	{"EBM each epoch, 4x CO2 (+7.4 W/m2)", deepClimateTerms{on: true, forcing: 7.42}},
}

// epochReading is what an epoch's climate and ground were.
type epochReading struct {
	seconds              float64
	land, polar          float64 // share of the planet above the sea; of the planet poleward of 60
	meanT, landT, landRn float64
	landRo               float64
	polarT               float64 // the area mean poleward of 60, both poles
	// and the same under today's air, over the same ground
	meanT0, polarT0, landT0 float64
}

type climateRun struct {
	name    string
	history float64
	epochs  []epochReading
	phases  map[string]phase.Phase
	height  []float64
	ground  groundReading
	lands   [][atmos.ZonalBands]float64 // each epoch's land share by band, as the climate read it
	// the land's rain and runoff each epoch under today's air, on the
	// epoch's own ground: see todaysRainOn
	todayRn, todayRo []float64
}

type groundReading struct {
	land                   float64
	q10, q50, q90, q99, hi float64
	channels, largest      float64
	water, rain, runoff    float64
}

// readGround is the hypsometry and the drainage of a history's grid as its
// last epoch left it. The land is what stands over the history's own sea
// level (g.base, where the planet's water filled the basins as the epoch
// began: see pourSea; it was the lowest historySea of the ground read
// afresh, historyBase, before the sea was poured); the rain is over the
// tiles the last epoch's air read as land.
func readGround(g *Grid) groundReading {
	var r groundReading
	var hs []float64
	var nLand, ch, nAired int
	var big, rain, runoff float64
	base := g.base
	for i := range g.Tiles {
		if len(g.aired) == len(g.Tiles) && g.aired[i] >= 0 {
			nAired++
			rain += g.rain[i]
			runoff += g.runoff[i]
		}
		if g.Height[i] <= base {
			continue
		}
		nLand++
		hs = append(hs, g.Height[i]-base)
		if g.area[i] >= 100 {
			ch++
		}
		big = math.Max(big, g.area[i])
	}
	r.land = float64(nLand) / float64(len(g.Tiles))
	q := quantiles(hs, 0.1, 0.5, 0.9, 0.99)
	r.q10, r.q50, r.q90, r.q99 = q[0], q[1], q[2], q[3]
	lo, hi := slices.Min(hs), slices.Max(hs)
	var mean float64
	for _, h := range hs {
		mean += h
	}
	mean /= float64(len(hs))
	r.hi = (mean - lo) / (hi - lo)
	r.channels = float64(ch) / float64(nLand)
	r.largest = big / float64(nLand)
	r.water = g.water
	r.rain, r.runoff = rain/math.Max(1, float64(nAired)), runoff/math.Max(1, float64(nAired))
	return r
}

// readEpoch is the climate an epoch ran under, on the ground its air read
// (g.aired: the land is what was over the water then), beside what today's
// air would have been over the same ground.
func readEpoch(g *Grid) epochReading {
	var r epochReading
	var wsum, tsum, t0sum, polarW, polarL, pw, pt, pt0 float64
	var nLand int
	var landT, landT0, landRn, landRo float64
	base := math.Max(0, g.base)
	for y := 0; y < g.H; y++ {
		lat := g.air.Lat[y]
		today := atmos.ZonalMean(lat)
		w := math.Cos(lat * math.Pi / 180)
		wsum += w
		tsum += w * g.air.Mean[y]
		t0sum += w * today
		if math.Abs(lat) >= 60 {
			pw += w
			pt += w * g.air.Mean[y]
			pt0 += w * today
		}
		for i := y * g.W; i < (y+1)*g.W; i++ {
			up := g.aired[i] >= 0
			if math.Abs(lat) >= 60 {
				polarW++
				if up {
					polarL++
				}
			}
			if up {
				nLand++
				h := base + float64(g.aired[i])
				landT += g.air.Mean[y] - Lapse*h
				landT0 += today - Lapse*h
				landRn += g.rain[i]
				landRo += g.runoff[i]
			}
		}
	}
	r.meanT, r.polarT, r.meanT0, r.polarT0 = tsum/wsum, pt/pw, t0sum/wsum, pt0/pw
	r.land = float64(nLand) / float64(len(g.Tiles))
	r.polar = polarL / math.Max(1, polarW)
	r.landT, r.landT0, r.landRn, r.landRo = landT/float64(nLand), landT0/float64(nLand), landRn/float64(nLand), landRo/float64(nLand)
	return r
}

func phaseMap() map[string]phase.Phase {
	m := map[string]phase.Phase{}
	for _, p := range phase.All() {
		m[p.Name] = p
	}
	return m
}

// runClimateStudy makes the history of a world of terms under variant v.
func runClimateStudy(t *testing.T, seed uint64, terms Terms, v deepVariant, sameGround bool) climateRun {
	var today *Air
	run := climateRun{name: v.name}
	deepClimate = v.terms
	deepClimate.watch = func(g *Grid, e int, land, sea *[atmos.ZonalBands]float64, air *Air) {
		run.lands = append(run.lands, *land)
		if sameGround {
			rn, ro := todaysRainOn(g, today)
			if os.Getenv("TERRA_DEEP_CLIMATE_SANITY") == "1" {
				rn2, _ := todaysRainOn(g, g.air)
				rn3, _ := todaysRainOn(g, today)
				t.Logf("epoch %d: today on a copy %.0f, again %.0f; the epoch's own air on a copy %.0f", e, rn, rn3, rn2)
			}
			run.todayRn = append(run.todayRn, rn)
			run.todayRo = append(run.todayRo, ro)
		}
	}
	defer func() { deepClimate = deepClimateTerms{} }()
	var last time.Time
	var at map[string]phase.Phase
	epochWatch = func(g *Grid, cr *crust, plates []Plate, e int) {
		now := time.Now()
		if e < 0 {
			at = phaseMap()
		} else {
			r := readEpoch(g)
			r.seconds = now.Sub(last).Seconds()
			run.epochs = append(run.epochs, r)
		}
		if e == terms.Epochs-1 {
			end := phaseMap()
			run.phases = map[string]phase.Phase{}
			for name, p := range end {
				q := at[name]
				run.phases[name] = phase.Phase{Name: name, Calls: p.Calls - q.Calls, Seconds: p.Seconds - q.Seconds}
			}
			run.height = slices.Clone(g.Height)
			run.ground = readGround(g)
		}
		last = time.Now()
	}
	defer func() { epochWatch = nil }()
	w := unmade(seed, terms)
	g := w.newGround(terms)
	hg := w.historyGround(g, terms)
	today = hg.air
	start := time.Now()
	w.history(hg, terms.Epochs, terms.SeaShare, terms.Water)
	run.history = time.Since(start).Seconds()
	return run
}

func TestDeepClimateCost(t *testing.T) {
	if os.Getenv("TERRA_DEEP_CLIMATE") != "1" {
		t.Skip("the deep-time climate's cost study is run by hand: TERRA_DEEP_CLIMATE=1")
	}
	if !phase.On() {
		t.Log("TERRA_PHASES=1 is off: the passes are not timed")
	}
	which := os.Getenv("TERRA_DEEP_CLIMATE_TERMS")
	runs := 1
	seed := uint64(1)
	fmt.Sscan(os.Getenv("TERRA_DEEP_CLIMATE_SEED"), &seed)
	fmt.Sscan(os.Getenv("TERRA_DEEP_CLIMATE_RUNS"), &runs)
	type world struct {
		name  string
		terms Terms
	}
	var worlds []world
	if which != "globe" {
		worlds = append(worlds, world{"small globe", smallGlobe()})
	}
	if which != "small" {
		worlds = append(worlds, world{"globe", GlobeTerms()})
	}
	// TERRA_DEEP_CLIMATE_VARIANTS picks variants by their place in
	// deepVariants, comma-separated; the first two are always run.
	chosen := deepVariants
	if pick := os.Getenv("TERRA_DEEP_CLIMATE_VARIANTS"); pick != "" {
		chosen = deepVariants[:2]
		for _, f := range strings.Split(pick, ",") {
			var k int
			if _, err := fmt.Sscan(f, &k); err == nil && k >= 2 && k < len(deepVariants) {
				chosen = append(chosen, deepVariants[k])
			}
		}
	}
	var out strings.Builder
	for _, wd := range worlds {
		fmt.Fprintf(&out, "\n## %s %dx%d, seed %d, %d epochs, %d runs each\n", wd.name, wd.terms.Width, wd.terms.Height, seed, wd.terms.Epochs, runs)
		results := map[string][]climateRun{}
		for r := 0; r < runs; r++ {
			for _, v := range chosen {
				run := runClimateStudy(t, seed, wd.terms, v, false)
				results[v.name] = append(results[v.name], run)
				t.Logf("%s / %s: history %.2f s", wd.name, v.name, run.history)
			}
		}
		// And once more, untimed, reading today's rain on each epoch's ground
		// beside the epoch's own: a second weather an epoch.
		on := runClimateStudy(t, seed, wd.terms, deepVariants[1], true)
		off := results[deepVariants[0].name]
		offMin := minHistory(off)
		fmt.Fprintf(&out, "\n| variant | history s (min of runs) | x today | per epoch s | weather calls | weather s | windsFor s | currents s | vapour s | orographic s | rainOn s | EBM s | deepClimate s |\n|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
		for _, v := range chosen {
			rs := results[v.name]
			best := rs[0]
			for _, r := range rs {
				if r.history < best.history {
					best = r
				}
			}
			p := best.phases
			var ep float64
			for _, e := range best.epochs {
				ep += e.seconds
			}
			fmt.Fprintf(&out, "| %s | %.2f | %.2f | %.2f | %d | %.2f | %.2f | %.2f | %.2f | %.2f | %.2f | %.2f | %.2f |\n",
				v.name, minHistory(rs), minHistory(rs)/offMin, ep/float64(len(best.epochs)),
				p["weather"].Calls, p["weather"].Seconds, p["windsFor"].Seconds, p["airEnv.currents"].Seconds,
				p["airEnv.vapour"].Seconds, p["orographic"].Seconds, p["rainOn"].Seconds, p["deepClimate.ebm"].Seconds, p["deepClimate"].Seconds)
		}
		o := off[0]
		fmt.Fprintf(&out, "\n### Each epoch's climate (EBM each epoch against today's air over the same ground)\n\n"+
			"| epoch | land the air read | land poleward of 60 | mean T, today | EBM | T poleward of 60, today | EBM | land T, today | EBM | land rain mm, today | EBM | land runoff mm, today | EBM | land rain mm, off run's own ground |\n"+
			"|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
		for e := range on.epochs {
			a, b := o.epochs[e], on.epochs[e]
			fmt.Fprintf(&out, "| %d | %.3f | %.3f | %.2f | %.2f | %.2f | %.2f | %.2f | %.2f | %.0f | %.0f | %.0f | %.0f | %.0f |\n",
				e, b.land, b.polar, b.meanT0, b.meanT, b.polarT0, b.polarT, b.landT0, b.landT, on.todayRn[e], b.landRn, on.todayRo[e], b.landRo, a.landRn)
		}
		fmt.Fprintf(&out, "\n### The ground the history leaves\n\n"+
			"| variant | land h q10 | q50 | q90 | q99 | hypsometric integral | channel share (area>=100) | largest basin / land | water m3/s | land rain mm | land runoff mm | RMS height diff from off, m | epochs' mean T | epochs' land rain mm | epochs' land runoff mm |\n"+
			"|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
		for _, v := range chosen {
			r := results[v.name][0]
			gr := r.ground
			var d2 float64
			for i := range r.height {
				d := r.height[i] - o.height[i]
				d2 += d * d
			}
			var mt, mr, mo float64
			for _, e := range r.epochs {
				mt += e.meanT / float64(len(r.epochs))
				mr += e.landRn / float64(len(r.epochs))
				mo += e.landRo / float64(len(r.epochs))
			}
			fmt.Fprintf(&out, "| %s | %.1f | %.1f | %.1f | %.1f | %.4f | %.4f | %.4f | %.4g | %.0f | %.0f | %.1f | %.2f | %.0f | %.0f |\n",
				v.name, gr.q10, gr.q50, gr.q90, gr.q99, gr.hi, gr.channels, gr.largest, gr.water, gr.rain, gr.runoff, math.Sqrt(d2/float64(len(r.height))), mt, mr, mo)
		}
		if runs > 1 {
			fmt.Fprintf(&out, "\nEvery run's history, s:\n\n")
			for _, v := range chosen {
				fmt.Fprintf(&out, "- %s:", v.name)
				for _, r := range results[v.name] {
					fmt.Fprintf(&out, " %.2f", r.history)
				}
				fmt.Fprintf(&out, "\n")
			}
		}
		// The cheaper variants' climates against the solve each epoch, on
		// the land each epoch of the solve's run read.
		fmt.Fprintf(&out, "\n### The cheaper climates against a cold solve each epoch (degrees, worst over the bands)\n\n"+
			"| epoch | warm 3 yr | warm 1 yr | every other epoch | shift by land share | today's |\n|---|---|---|---|---|---|\n")
		var warm3, warm1 *atmos.ZonalYear
		today, _ := shiftSensitivity()
		var prev *atmos.ZonalYear
		var worst [5]float64
		for e, land := range on.lands {
			sea := land
			for k := range sea {
				sea[k] = 1 - land[k]
			}
			cold := atmos.SolveZonal(&land, &sea, atmos.ZonalYears, nil)
			if warm3 == nil {
				warm3, warm1 = cold, cold
			} else {
				warm3 = atmos.SolveZonal(&land, &sea, 3, warm3)
				warm1 = atmos.SolveZonal(&land, &sea, 1, warm1)
			}
			other := cold
			if e%2 == 1 {
				other = prev
			}
			_, sens := shiftSensitivity()
			var d [5]float64
			for k := range atmos.ZonalBands {
				lat := math.Asin(-1+(float64(k)+0.5)*2/atmos.ZonalBands) * 180 / math.Pi
				c := cold.Mean(lat)
				shifted := today.Mean(lat) + sens[k]*(land[k]-todayLand)
				for j, v := range []float64{warm3.Mean(lat), warm1.Mean(lat), other.Mean(lat), shifted, today.Mean(lat)} {
					d[j] = math.Max(d[j], math.Abs(v-c))
				}
			}
			for j := range d {
				worst[j] = math.Max(worst[j], d[j])
			}
			fmt.Fprintf(&out, "| %d | %.2f | %.2f | %.2f | %.2f | %.2f |\n", e, d[0], d[1], d[2], d[3], d[4])
			if e%2 == 0 {
				prev = cold
			}
		}
		fmt.Fprintf(&out, "| worst | %.2f | %.2f | %.2f | %.2f | %.2f |\n", worst[0], worst[1], worst[2], worst[3], worst[4])
	}
	fmt.Print(out.String())
	if path := os.Getenv("TERRA_DEEP_CLIMATE_OUT"); path != "" {
		os.WriteFile(path, []byte(out.String()), 0o644)
	}
}

func minHistory(rs []climateRun) float64 {
	m := math.Inf(1)
	for _, r := range rs {
		m = math.Min(m, r.history)
	}
	return m
}

// todaysRainOn is the mean rain and runoff over g's land, in mm a year, as
// the air today would rain on g's ground as it now lies: on a copy, so that
// g is as it was.
func todaysRainOn(g *Grid, today *Air) (rain, runoff float64) {
	c := g.Clone()
	c.air = today
	c.aired = nil
	c.weather()
	var n int
	for i := range c.Tiles {
		if c.aired[i] >= 0 {
			n++
			rain += c.rain[i]
			runoff += c.runoff[i]
		}
	}
	return rain / float64(n), runoff / float64(n)
}
