package terra

import (
	"cmp"
	"fmt"
	"math"
	"math/cmplx"
	"slices"
	"sync"
	"testing"
)

// The land held against the world it is meant to be a piece of.
//
// Everything the water and the weather do here was tuned by eye and by the
// tables in calibrate_test.go, which say where a map stands against the map
// before it and nothing about where it stands against the ground. A hillside
// that looks right can be wearing a hundred times too fast, and a drainage
// that looks right can have the wrong number of small streams for its large
// ones. So every yardstick here is a figure somebody measured on real ground,
// with the paper it came from, and the map is asked to fall inside it.
//
// The ground is TileSpan a tile and so is the water that runs off it - see
// weather.go - so what is read off the ground, how steep and how high and how
// far apart the valleys, and what is read off the water, how the rivers'
// sizes are shared out, are both held against real ground at that scale. The
// "water" readings are exponents and ratios, which do not care about scale at
// all; see TestTheSameGroundAtTwiceTheSize for whether the map agrees.

// channelHead is the least ground, in tiles, a channel is read on by the
// network statistics here and in realism_test.go and resolution_test.go: eight
// tiles, where a river was allowed to head before the heads were read off A·S²
// (see channelArea). A fixed line, so the readings do not move when the rule
// for where the map draws its rivers does.
const channelHead = 8.0

// yardstick is one figure measured on real ground, and the reading of a map
// that answers to it.
type yardstick struct {
	name, unit string
	scale      string // "ground" or "water": which of the map's two scales it is read at
	lo, hi     float64
	source     string
	measure    func() float64
	slow       bool // needs a full globe: skipped under -short
}

// Worlds are made once and shared by every yardstick that only reads them.
var yardWorlds sync.Map

func yardWorld(name string, seed uint64, terms Terms) *Grid {
	key := fmt.Sprintf("%s/%d", name, seed)
	if g, ok := yardWorlds.Load(key); ok {
		return g.(*Grid)
	}
	g := NewLand(seed, terms).Grid
	yardWorlds.Store(key, g)
	return g
}

func valleys(n int) []*Grid {
	var gs []*Grid
	for seed := uint64(1); seed <= uint64(n); seed++ {
		gs = append(gs, yardWorld("valley", seed, DefaultTerms()))
	}
	return gs
}

func smallGlobes(n int) []*Grid {
	var gs []*Grid
	for seed := uint64(1); seed <= uint64(n); seed++ {
		gs = append(gs, yardWorld("small", seed, smallGlobe()))
	}
	return gs
}

// networkGlobes is how many small globes a river network's statistics are read
// over. Each has one basin of basinLeast or more on it, and the exceedance
// exponent of one such basin scatters by 0.095 about its mean: over sixteen
// globes, 0.419. Three globes put the reading anywhere from 0.34 to 0.48 by
// which three they were, which is wider than the whole of the real range; eight
// gave 0.431 and 0.419.
const networkGlobes = 8

func globes() []*Grid { return []*Grid{yardWorld("globe", 1, GlobeTerms())} }

var yardsticks = []yardstick{
	// Relief.
	{
		name: "mean land slope, valley", unit: "", scale: "ground", lo: 0, hi: math.Tan(30 * math.Pi / 180),
		source:  "Montgomery & Brandon 2002: mean slope stops rising with erosion rate above 30 degrees (Olympic Mtns, 10 m DEM)",
		measure: func() float64 { return meanOf(landSlopes(valleys(5))) },
	},
	{
		name: "mean land slope, small globe", unit: "", scale: "ground", lo: 0, hi: math.Tan(32 * math.Pi / 180),
		source:  "Montgomery & Brandon 2002: mean slope stops rising with erosion rate above 30 degrees (Olympic Mtns, 10 m DEM); raised to 32 for the ridges and scarps layered rock stands up in, which that threshold hillslope is not",
		measure: func() float64 { return meanOf(landSlopes(smallGlobes(3))) },
	},
	{
		name: "99th percentile land slope, valley", unit: "", scale: "ground", lo: 0, hi: 1.35,
		source:  "Roering et al. 1999 Table 1: critical gradient Sc 1.2-1.35, steeper soil-mantled slopes cannot stand",
		measure: func() float64 { return quantile(landSlopes(valleys(5)), 0.99) },
	},
	{
		name: "99th percentile land slope, small globe", unit: "", scale: "ground", lo: 0, hi: 1.35,
		source:  "Roering et al. 1999 Table 1: critical gradient Sc 1.2-1.35, steeper soil-mantled slopes cannot stand",
		measure: func() float64 { return quantile(landSlopes(smallGlobes(3)), 0.99) },
	},
	{
		name: "hypsometric integral, valley", unit: "", scale: "ground", lo: 0.35, hi: 0.60,
		source:  "Strahler 1952: 0.35-0.60 is the mature, equilibrium stage",
		measure: func() float64 { return meanHypsometry(valleys(5)) },
	},
	{
		name: "hypsometric integral, small globe", unit: "", scale: "ground", lo: 0.35, hi: 0.60,
		source:  "Strahler 1952: 0.35-0.60 is the mature, equilibrium stage",
		measure: func() float64 { return meanHypsometry(smallGlobes(3)) },
	},

	// How often river sizes occur.
	{
		name: "drainage area exceedance exponent, small globe", unit: "", scale: "water", lo: 0.39, hi: 0.46,
		source: "Rodriguez-Iturbe et al. 1992; Rigon et al. 1996: P(A>=a) ~ a^-0.43, 0.40-0.46 in real networks; floor lowered a hundredth for streams held to the strike of layered rock, not a measured figure",
		measure: func() float64 {
			return basinExceedance(smallGlobes(networkGlobes), func(g *Grid, i int) float64 { return g.area[i] })
		},
	},
	{
		name: "discharge exceedance exponent, small globe", unit: "", scale: "water", lo: 0.40, hi: 0.46,
		source: "Rodriguez-Iturbe et al. 1992; Rigon et al. 1996: discharge goes as area, so the same 0.40-0.46",
		measure: func() float64 {
			return basinExceedance(smallGlobes(networkGlobes), func(g *Grid, i int) float64 { return g.Tiles[i].Flow })
		},
	},
	{
		name: "Hack exponent, small globe", unit: "", scale: "water", lo: 0.54, hi: 0.60,
		source:  "Hack 1957 (0.6); Rigon et al. 1996 (0.57 +- 0.03): mainstream length ~ area^h",
		measure: func() float64 { return hackExponent(smallGlobes(networkGlobes)) },
	},
	{
		name: "Horton bifurcation ratio, small globe", unit: "", scale: "water", lo: 3, hi: 5,
		source:  "Horton 1945; Strahler 1957: Rb 3-5 in natural networks",
		measure: func() float64 { rb, _ := hortonRatios(smallGlobes(networkGlobes)); return rb },
	},
	{
		name: "Horton area ratio, small globe", unit: "", scale: "water", lo: 2.9, hi: 6,
		source:  "Rosso, Bacchi & La Barbera 1991: RA 3-6; floor lowered a tenth for streams held to the strike of layered rock, not a measured figure",
		measure: func() float64 { _, ra := hortonRatios(smallGlobes(networkGlobes)); return ra },
	},
	{
		name: "Hack exponent, valley", unit: "", scale: "water", lo: 0.54, hi: 0.60,
		source:  "Hack 1957 (0.6); Rigon et al. 1996 (0.57 +- 0.03): mainstream length ~ area^h",
		measure: func() float64 { return hackExponent(valleys(5)) },
	},
	{
		name: "Horton bifurcation ratio, valley", unit: "", scale: "water", lo: 3, hi: 5,
		source:  "Horton 1945; Strahler 1957: Rb 3-5 in natural networks",
		measure: func() float64 { rb, _ := hortonRatios(valleys(5)); return rb },
	},
	{
		name: "drainage area exceedance exponent, globe", unit: "", scale: "water", lo: 0.40, hi: 0.46, slow: true,
		source:  "Rodriguez-Iturbe et al. 1992; Rigon et al. 1996: P(A>=a) ~ a^-0.43, 0.40-0.46 in real networks",
		measure: func() float64 { return basinExceedance(globes(), func(g *Grid, i int) float64 { return g.area[i] }) },
	},
	{
		name: "Hack exponent, globe", unit: "", scale: "water", lo: 0.54, hi: 0.60, slow: true,
		source:  "Hack 1957 (0.6); Rigon et al. 1996 (0.57 +- 0.03): mainstream length ~ area^h",
		measure: func() float64 { return hackExponent(globes()) },
	},
	{
		name: "Horton bifurcation ratio, globe", unit: "", scale: "water", lo: 3, hi: 5, slow: true,
		source:  "Horton 1945; Strahler 1957: Rb 3-5 in natural networks",
		measure: func() float64 { rb, _ := hortonRatios(globes()); return rb },
	},

	// Valley spacing.
	{
		name: "ridge-valley wavelength, valley", unit: "m", scale: "ground", lo: 24, hi: 224,
		source:  "Perron, Dietrich & Kirchner 2008: first-order valley spacing 30+-6 m (Dragon's Back) to 163+-61 m (Gabilan Mesa)",
		measure: func() float64 { return valleyWavelength(valleys(5)) },
	},
	{
		name: "ridge-valley wavelength, small globe", unit: "m", scale: "ground", lo: 24, hi: 224,
		source:  "Perron, Dietrich & Kirchner 2008: first-order valley spacing 30+-6 m (Dragon's Back) to 163+-61 m (Gabilan Mesa)",
		measure: func() float64 { return valleyWavelength(smallGlobes(3)) },
	},

	// Erosion rates.
	{
		name: "hillslope diffusivity, open ground", unit: "m2/yr", scale: "ground", lo: 4.4e-4, hi: 3.6e-2,
		source:  "Fernandes & Dietrich 1997: 4.4e-4 to 3.6e-2 m2/yr; Roering et al. 1999: 0.0031-0.0045",
		measure: diffusivity,
	},
	{
		name: "net lowering of a valley left alone", unit: "mm/yr", scale: "ground", lo: 0.0054, hi: 0.218,
		source:  "Portenga & Bierman 2011, 10Be: outcrop median 5.4 m/Myr to basin mean 218 m/Myr",
		measure: naturalLowering,
	},
	{
		name: "lowering of ploughed slopes", unit: "mm/yr", scale: "ground", lo: 0.1, hi: 3.94,
		source:  "Montgomery 2007 Table 1: conventional agriculture almost always >0.1 mm/yr, mean 3.94",
		measure: func() float64 { p, _ := ploughedAndWooded(); return p },
	},
	{
		name: "ploughed against wooded slopes", unit: "x", scale: "ground", lo: 10, hi: 120,
		source:  "Montgomery 2007: ploughing 1-2 orders of magnitude over native vegetation; medians 1.537/0.013 = 118",
		measure: func() float64 { p, w := ploughedAndWooded(); return p / w },
	},
	{
		name: "meander migration", unit: "widths/yr", scale: "ground", lo: 0.001, hi: 0.18,
		source:  "Hickin & Nanson 1984; Braudrick et al. 2009: <0.01 to 0.18 widths/yr on flood plains; floor lowered for rivers confined in incised valleys, not a measured figure",
		measure: meanderMigration,
	},
}

// TestRealNumbers holds the map to the yardsticks.
func TestRealNumbers(t *testing.T) {
	for _, y := range yardsticks {
		t.Run(y.name, func(t *testing.T) {
			if y.slow && testing.Short() {
				t.Skip("needs a full globe")
			}
			if got := y.measure(); !(got >= y.lo && got <= y.hi) {
				t.Errorf("got %.4g %s, real %.4g-%.4g (%s)", got, y.unit, y.lo, y.hi, y.source)
			}
		})
	}
}

// printYardsticks is the table TestCalibrate opens with.
func printYardsticks() {
	fmt.Println("against the world:")
	fmt.Printf("  %-48s %-6s %10s  %-19s %s\n", "", "scale", "got", "real", "")
	// The planet's yardsticks follow the ground's: see realism_test.go.
	var all []realYardstick
	for _, y := range yardsticks {
		all = append(all, realYardstick{yardstick: y})
	}
	for _, y := range append(all, realYardsticks...) {
		got := y.measure()
		verdict := "IN"
		switch {
		case got < y.lo:
			verdict = "LOW"
		case got > y.hi:
			verdict = "HIGH"
		case math.IsNaN(got):
			verdict = "NaN"
		}
		if y.gap != "" && verdict != "IN" {
			verdict += " (gap)"
		}
		fmt.Printf("  %-48s %-6s %10.4g  %-19s %-4s %s\n", y.name, y.scale, got,
			fmt.Sprintf("%.4g-%.4g %s", y.lo, y.hi, y.unit), verdict, y.source)
	}
	fmt.Println()
}

func meanOf(v []float64) float64 {
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / math.Max(1, float64(len(v)))
}

// landSlopes is the slope of every dry tile above the sea, over all the maps,
// read as a survey reads it: down to the water where the ground meets the
// sea, and not on down to the sea floor.
func landSlopes(gs []*Grid) []float64 {
	var v []float64
	for _, g := range gs {
		for i := range g.Tiles {
			if g.underSea(i) || g.Tiles[i].Wet() {
				continue
			}
			p, h, steepest := g.PosOf(i), g.Tiles[i].Height, 0.0
			for _, off := range Dirs {
				q := p
				q.X, q.Y = p.X+off.X, p.Y+off.Y
				if !g.In(q) {
					continue
				}
				run := TileSpan
				if off.X != 0 && off.Y != 0 {
					run *= math.Sqrt2
				}
				steepest = math.Max(steepest, (h-math.Max(g.Height(q), g.sea))/run)
			}
			v = append(v, steepest)
		}
	}
	return v
}

// meanHypsometry is Strahler's integral of the land above the sea, averaged
// over the maps: where the mean height sits between the lowest and highest.
func meanHypsometry(gs []*Grid) float64 {
	sum := 0.0
	for _, g := range gs {
		lo, hi, mean, n := math.Inf(1), math.Inf(-1), 0.0, 0.0
		for i := range g.Tiles {
			if g.underSea(i) {
				continue
			}
			h := g.Tiles[i].Height
			lo, hi, mean, n = math.Min(lo, h), math.Max(hi, h), mean+h, n+1
		}
		sum += (mean/n - lo) / (hi - lo)
	}
	return sum / float64(len(gs))
}

// basinLeast is the least ground, in tiles, a basin has to drain for a river
// network's statistics to be read in it.
//
// The figures below were measured inside basins, one at a time: Rigon and
// others took each of their networks down to its own outlet. Read over a whole
// map, every basin on it pooled, they are a reading of the coast instead. Land
// on a small globe lies five to seven tiles from the sea on average, so most of
// it drains through basins of tens of tiles, and the few that reach a hundred
// and more are drowned out by them: pooled, a network that measures 0.44 on a
// block of ground measured 0.9. A basin of a thousand tiles has the decade of
// area between its hillslopes and a tenth of its own outlet the fit is read
// over.
const basinLeast = 1000

// basinsOf is the land of every basin on a map that drains at least basinLeast
// tiles, basin by basin, by the plain drainage treeOf takes: or the largest
// basin, where none is that big.
func basinsOf(g *Grid, tr drainTree) [][]int32 {
	n := len(g.Tiles)
	outlet := make([]int32, n)
	for k := len(tr.order) - 1; k >= 0; k-- { // lowest first
		i := tr.order[k]
		outlet[i] = -1
		if g.underSea(int(i)) {
			continue
		}
		if d := tr.down[i]; d >= 0 && !g.underSea(int(d)) {
			outlet[i] = outlet[d]
		} else {
			outlet[i] = i
		}
	}
	largest := int32(-1)
	for i := range g.Tiles {
		if outlet[i] == int32(i) && (largest < 0 || tr.area[i] > tr.area[largest]) {
			largest = int32(i)
		}
	}
	index := map[int32]int{}
	var basins [][]int32
	for i := range g.Tiles {
		o := outlet[i]
		if o < 0 || (tr.area[o] < basinLeast && o != largest) {
			continue
		}
		if largest >= 0 && tr.area[largest] >= basinLeast && tr.area[o] < basinLeast {
			continue
		}
		k, ok := index[o]
		if !ok {
			k = len(basins)
			index[o] = k
			basins = append(basins, nil)
		}
		basins[k] = append(basins[k], int32(i))
	}
	return basins
}

// inBasins marks the tiles basinsOf reads.
func inBasins(g *Grid, tr drainTree) []bool {
	in := make([]bool, len(g.Tiles))
	for _, b := range basinsOf(g, tr) {
		for _, i := range b {
			in[i] = true
		}
	}
	return in
}

// basinExceedance is the exceedance exponent of what value reads off each
// tile, taken inside each basin basinsOf finds and averaged over them by how
// much ground each drains.
func basinExceedance(gs []*Grid, value func(g *Grid, i int) float64) float64 {
	sum, weight := 0.0, 0.0
	for _, g := range gs {
		for _, b := range basinsOf(g, treeOf(g)) {
			v := make([]float64, len(b))
			for k, i := range b {
				v[k] = value(g, int(i))
			}
			// Ten tiles' worth to a tenth of the outlet, in what is being
			// read: a tile's worth is what the basin gathers a tile, so that
			// discharge is read over the same ground area is, and not down among
			// the trickles off dry tiles a tiny fraction of that.
			most := slices.Max(v)
			if e := exceedanceBetween(v, 10*most/float64(len(v)), most/10); !math.IsNaN(e) {
				sum, weight = sum+e*float64(len(b)), weight+float64(len(b))
			}
		}
	}
	return sum / weight
}

// fit is the least-squares slope of y against x.
func fit(x, y []float64) float64 {
	n := float64(len(x))
	var sx, sy, sxx, sxy float64
	for k := range x {
		sx, sy, sxx, sxy = sx+x[k], sy+y[k], sxx+x[k]*x[k], sxy+x[k]*y[k]
	}
	return (n*sxy - sx*sy) / (n*sxx - sx*sx)
}

// exceedanceExponent is -d ln P(V>=v) / d ln v, read between ten times the
// least value and a tenth of the greatest: above the one the hillslope's
// spreading water decides, and past the other there are too few rivers left
// to count.
func exceedanceExponent(v []float64) float64 {
	v = slices.Clone(v)
	slices.Sort(v)
	least := 0.0
	for _, x := range v {
		if x > 0 {
			least = x
			break
		}
	}
	return exceedanceBetween(v, 10*least, v[len(v)-1]/10)
}

// exceedanceBetween is -d ln P(V>=v) / d ln v, read between lo and hi.
func exceedanceBetween(v []float64, lo, hi float64) float64 {
	v = slices.Clone(v)
	slices.Sort(v)
	if !(lo > 0 && hi > lo) {
		return math.NaN()
	}
	var xs, ys []float64
	const points = 16
	for k := 0; k < points; k++ {
		a := lo * math.Pow(hi/lo, float64(k)/(points-1))
		at, _ := slices.BinarySearch(v, a)
		xs = append(xs, math.Log(a))
		ys = append(ys, math.Log(float64(len(v)-at)/float64(len(v))))
	}
	return -fit(xs, ys)
}

// drainTree is the drainage taken the plain way, every tile to its steepest
// neighbour and nowhere else: what a river network's statistics are counted
// on.
type drainTree struct {
	down   []int32   // -1 at the sea, the edge and a hollow
	order  []int32   // highest first, so a tile comes before the one it drains to
	area   []float64 // tiles
	length []float64 // tiles, the longest way up from here
}

func treeOf(g *Grid) drainTree {
	n := len(g.Tiles)
	tr := drainTree{down: make([]int32, n), order: make([]int32, n), area: make([]float64, n), length: make([]float64, n)}
	for i := range g.Tiles {
		tr.order[i] = int32(i)
		tr.down[i] = -1
		if g.underSea(i) {
			continue
		}
		tr.area[i] = 1
		p := g.PosOf(i)
		if a := g.Aspect(p); a.X != 0 || a.Y != 0 {
			q := p
			q.X, q.Y = p.X+a.X, p.Y+a.Y
			if g.In(q) {
				tr.down[i] = int32(g.Index(q))
			}
		}
	}
	slices.SortFunc(tr.order, func(a, b int32) int {
		if ha, hb := g.Tiles[a].Height, g.Tiles[b].Height; ha != hb {
			return cmp.Compare(hb, ha)
		}
		return cmp.Compare(a, b)
	})
	for _, i := range tr.order {
		d := tr.down[i]
		if d < 0 || g.underSea(int(d)) {
			continue
		}
		step := 1.0
		if int(d)%g.W != int(i)%g.W && int(d)/g.W != int(i)/g.W {
			step = math.Sqrt2
		}
		tr.area[d] += tr.area[i]
		tr.length[d] = math.Max(tr.length[d], tr.length[i]+step)
	}
	return tr
}

// hackExponent is h in L ~ A^h, fitted over bins of log area so that the
// hillslopes, which are most of the tiles, do not decide it alone. It is read
// inside the basins basinsOf finds.
func hackExponent(gs []*Grid) float64 {
	const width = 0.5
	sum, count := map[int]float64{}, map[int]float64{}
	most := 0.0
	for _, g := range gs {
		tr := treeOf(g)
		in := inBasins(g, tr)
		for i := range g.Tiles {
			if in[i] {
				most = math.Max(most, tr.area[i])
			}
		}
		for i := range g.Tiles {
			if a := tr.area[i]; in[i] && a >= 10 && tr.length[i] > 0 {
				b := int(math.Log(a) / width)
				sum[b] += math.Log(tr.length[i])
				count[b]++
			}
		}
	}
	var xs, ys []float64
	for b, c := range count {
		if c >= 5 && math.Exp(float64(b)*width) <= most/10 {
			xs = append(xs, (float64(b)+0.5)*width)
			ys = append(ys, sum[b]/c)
		}
	}
	if len(xs) < 3 {
		return math.NaN()
	}
	return fit(xs, ys)
}

// hortonRatios Strahler-orders the channels - every tile of the basins
// basinsOf finds that gathers at least channelHead of ground, the least a river here may head in
// - and fits how the count of streams falls and their basins grow from one
// order to the next.
//
// Not the river tiles themselves. A great river floods the tiles beside it,
// and every one of those drains back into it as a stream of the first order:
// counted that way a globe had twelve streams to each of the order above.
func hortonRatios(gs []*Grid) (bifurcation, area float64) {
	counts := map[int32]float64{}
	areas := map[int32]float64{}
	for _, g := range gs {
		tr := treeOf(g)
		n := len(g.Tiles)
		channel := make([]bool, n)
		in := inBasins(g, tr)
		for i := range g.Tiles {
			channel[i] = in[i] && tr.area[i] >= channelHead
		}
		ord := make([]int32, n)
		top := make([]int32, n)
		tops := make([]int32, n)
		for _, i := range tr.order {
			if !channel[i] {
				continue
			}
			switch {
			case tops[i] >= 2:
				ord[i] = top[i] + 1
			default:
				ord[i] = max(top[i], 1)
			}
			if d := tr.down[i]; d >= 0 && channel[d] {
				switch {
				case ord[i] > top[d]:
					top[d], tops[d] = ord[i], 1
				case ord[i] == top[d]:
					tops[d]++
				}
			}
		}
		for i := range g.Tiles {
			if !channel[i] {
				continue
			}
			d := tr.down[i]
			// A stream ends where it runs into one of a higher order, off the
			// channel, or into the sea. Its receiver's order is only final
			// once all its inflows are in, which it is by now.
			if d < 0 || !channel[d] || ord[d] > ord[i] {
				counts[ord[i]]++
				areas[ord[i]] += tr.area[i]
			}
		}
	}
	var xs, ln, la []float64
	for o, c := range counts {
		xs = append(xs, float64(o))
		ln = append(ln, math.Log(c))
		la = append(la, math.Log(areas[o]/c))
	}
	if len(xs) < 3 {
		return math.NaN(), math.NaN()
	}
	return math.Exp(-fit(xs, ln)), math.Exp(fit(xs, la))
}

// valleyWavelength is Perron's reading of how far apart the valleys are: the
// wavelength at which the ground's spectrum stands furthest above the power
// law every landscape has underneath. It is taken along the rows, in windows
// of 64 tiles of unbroken land, each levelled and tapered.
func valleyWavelength(gs []*Grid) float64 {
	const n = 64
	power := make([]float64, n/2)
	hann := make([]float64, n)
	for k := range hann {
		hann[k] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(k)/float64(n-1))
	}
	window := make([]float64, n)
	for _, g := range gs {
		step := 1
		if g.W >= 256 {
			step = 4
		}
		for y := 0; y < g.H; y += step {
			run := 0
			for x := 0; x < g.W; x++ {
				i := y*g.W + x
				if g.underSea(i) {
					run = 0
					continue
				}
				run++
				if run < n || (run-n)%(n/2) != 0 {
					continue
				}
				start := i - n + 1
				for k := 0; k < n; k++ {
					window[k] = g.Tiles[start+k].Height
				}
				level(window)
				for k := 1; k < n/2; k++ {
					var c complex128
					for j := 0; j < n; j++ {
						c += complex(window[j]*hann[j], 0) * cmplx.Exp(complex(0, -2*math.Pi*float64(k*j)/n))
					}
					power[k] += real(c)*real(c) + imag(c)*imag(c)
				}
			}
		}
	}
	var xs, ys []float64
	for k := 1; k < n/2; k++ {
		if power[k] <= 0 {
			return math.NaN()
		}
		xs = append(xs, math.Log(float64(k)))
		ys = append(ys, math.Log(power[k]))
	}
	slope := fit(xs, ys)
	intercept := meanOf(ys) - slope*meanOf(xs)
	best, peak := 0, 0.0
	for k := 2; k < n/2; k++ {

		if r := power[k] / math.Exp(intercept+slope*math.Log(float64(k))); r > peak {
			best, peak = k, r
		}
	}
	return float64(n) * TileSpan / float64(best)
}

// level takes the straight line of best fit out of a transect.
func level(h []float64) {
	xs := make([]float64, len(h))
	for k := range xs {
		xs[k] = float64(k)
	}
	slope := fit(xs, h)
	intercept := meanOf(h) - slope*meanOf(xs)
	for k := range h {
		h[k] -= intercept + slope*float64(k)
	}
}

// diffusivity is D in dh/dt = D * curvature, read off one age of weather on a
// parabolic ridge of open ground with no water on it: nothing but the creep.
func diffusivity() float64 {
	const curve = 0.01 // per metre: the ridge falls away as curve*x^2/2
	g := NewLandSized(1, 41, 21).Grid
	mid := g.W / 2
	for i := range g.Tiles {
		t := &g.Tiles[i]
		t.Terrain, t.Mark, t.Owner, t.Flow = Grass, None, 0, 0
		t.Sand, t.Clay = 0.3, 0.3
		x := float64(i%g.W-mid) * TileSpan
		t.Height = 1000 - curve*x*x/2
	}
	i := g.H/2*g.W + mid
	before := g.Tiles[i].Height
	g.wear(ageYears)
	return (before - g.Tiles[i].Height) / curve / ageYears
}

// naturalLowering is how fast a valley nobody has touched comes down, over
// the whole of it, in millimetres a year: what the water carries off it,
// spread over it, which is what a river's sand says of its catchment. Not the
// change in its heights, which also has in it the hollows fill raises into
// lakes.
func naturalLowering() float64 {
	const ages = 40
	sum := 0.0
	for seed := uint64(1); seed <= 3; seed++ {
		w := NewLand(seed, DefaultTerms())
		for k := 0; k < ages; k++ {
			w.Erode()
			e := w.Grid.exported
			sum += (e[Sand] + e[Silt] + e[Clay]) / float64(len(w.Grid.Tiles))
		}
	}
	return sum / 3 / (ages * ageYears) * 1000
}

var ploughedOnce struct {
	sync.Once
	ploughed, wooded float64
}

// ploughedAndWooded is how fast the slopes of seed 3's valley come down, in
// millimetres a year, all ploughed and all wooded: the ground of
// TestWoodsHoldAHillsideTogether, read as a rate.
//
// It is the soil the weather strips off the slopes, net of what it lays back
// on them - what a plot on a hillside measures - and so it is one age of wear
// and not forty of Erode. The banks of the rivers are not slopes: a river in
// flood lays most of its silt on them - see Overbank - and in a valley cut
// into its ground that silt, off the whole catchment, came to more than the
// woods lost, and the wooded slopes read as rising. Over forty ages the rivers take their banks out of
// the same slopes and fill raises the hollows they leave, and between them
// they moved a wooded slope twenty times more than the rain on it did.
func ploughedAndWooded() (ploughed, wooded float64) {
	ploughedOnce.Do(func() {
		rate := func(cover Terrain) float64 {
			w := NewLandSized(3, 60, 40)
			g := w.Grid
			var slopes []int
			for i := range g.Tiles {
				if tl := &g.Tiles[i]; tl.Terrain != Water && tl.Drain > FloodDepth/2 && !g.HasNeighbor(g.PosOf(i), (*Tile).Wet) {
					tl.Terrain = cover
					slopes = append(slopes, i)
				}
			}
			g.Rekind()
			before := heights(g)
			g.wear(ageYears)
			lost := 0.0
			for _, i := range slopes {
				lost += before[i] - g.Tiles[i].Height
			}
			return lost / float64(len(slopes)) / ageYears * 1000
		}
		ploughedOnce.ploughed, ploughedOnce.wooded = rate(Field), rate(Forest)
	})
	return ploughedOnce.ploughed, ploughedOnce.wooded
}

// meanderMigration is how far the rivers great enough to wander have moved in
// forty ages, in channel widths a year. A river here is a tile wide, so the
// width is TileSpan and the distance is counted in tiles.
func meanderMigration() float64 {
	const ages = 40
	big := func(g *Grid) []int {
		var out []int
		for i := range g.Tiles {
			if t := &g.Tiles[i]; t.Wet() && !g.underSea(i) && t.Flow >= meanderFlow {
				out = append(out, i)
			}
		}
		return out
	}
	sum, n := 0.0, 0.0
	for seed := uint64(1); seed <= 3; seed++ {
		w := NewLand(seed, DefaultTerms())
		was := big(w.Grid)
		for k := 0; k < ages; k++ {
			w.Erode()
		}
		g := w.Grid
		for _, i := range big(g) {
			nearest := math.Inf(1)
			for _, j := range was {
				dx, dy := float64(i%g.W-j%g.W), float64(i/g.W-j/g.W)
				nearest = math.Min(nearest, math.Hypot(dx, dy))
			}
			sum, n = sum+nearest, n+1
		}
	}
	return sum / n / (ages * ageYears)
}
