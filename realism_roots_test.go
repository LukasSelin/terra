package terra

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"testing"
)

// What grows holding the ground, held against the earth's.
//
// The vegetation holds the ground three ways (roots.go): it raises the stress
// the water has to clear, it slows the creep, and its roots let a hillside
// stand steeper. Three readings follow it: where the water cuts the ground
// against how much rain falls on it, against Langbein and Schumm's (1958)
// peak in semi-arid ground; how steep a wooded slope stands against the same
// slope cleared; and how much more ground the water takes after a fire.
func init() {
	realYardsticks = append(realYardsticks, rootsYardsticks...)
}

var rootsYardsticks = []realYardstick{
	{yardstick: yardstick{
		name: "rain where the drainage density peaks, valleys", unit: "mm", scale: "water", lo: 250, hi: 750,
		source: "Langbein & Schumm 1958: sediment yield peaks at some 300 mm of effective rain, where there is rain enough to run off and too little to close a cover over the ground; Melton 1957 and Abrahams 1984 find drainage density highest in semi-arid country. Read as the mean rain of the drawn valleys, seeds 1-3, at the wetness at which the share of their land a storm's water cuts through its cover (cutGround) is greatest, over wetness 0.15-2.5",
		measure: func() float64 {
			rows := drainageByRain()
			best := rows[0]
			for _, r := range rows[1:] {
				if r.dd > best.dd {
					best = r
				}
			}
			return best.rain
		},
	},
		gap: "known gap: L5 - a flood is a thousand times a tile's mean flow everywhere (floodFlow), and a dry country's bucket sheds near nothing over its year, so the storms its sparse cover has to stand are read too small; the cover does what Langbein and Schumm's peak asks of it, raising the share of the ground the water cuts 5.6 times at 280 mm and 2.3 times at 400 against open ground's full cover everywhere, but the water does not, and the peak waits on a storm's runoff from the day's rain (A7, #39): the wettest valleys, 2600 mm",
	},
}

// ddWetness is the wetness the drawn valleys of drainageByRain are made at:
// from a desert's to a rainforest's rain.
var ddWetness = []float64{0.15, 0.25, 0.35, 0.5, 0.75, 1, 1.5, 2.5}

// ddRow is the drainage at one wetness: the valleys' mean rain on land, mm;
// the share of their land the water cuts in a storm, under the cover they
// have and under open ground's full cover everywhere; and both as a drainage
// density, km a km².
type ddRow struct {
	wetness, rain    float64
	share, open      float64
	dd, ddOpen, tcMn float64
}

var drainageOnce struct {
	sync.Once
	rows []ddRow
}

// drainageByRain reads cutGround over the drawn valleys, seeds 1-3, at each
// of ddWetness.
func drainageByRain() []ddRow {
	drainageOnce.Do(func() {
		for _, wet := range ddWetness {
			terms := DefaultTerms()
			terms.Wetness = wet
			r := ddRow{wetness: wet}
			var cut, open, land, rain, tc float64
			for seed := uint64(1); seed <= 3; seed++ {
				g := yardWorld(fmt.Sprintf("valley-wet%g", wet), seed, terms)
				c, l, rn, t := cutGround(g, g.shearAt)
				o, _, _, _ := cutGround(g, func(i int) float64 {
					if natural(g.Tiles[i].Terrain) {
						return Grass.Shear()
					}
					return g.Tiles[i].Terrain.Shear()
				})
				cut, open, land, rain, tc = cut+c, open+o, land+l, rain+rn, tc+t
			}
			r.rain, r.tcMn = rain/land, tc/land
			r.share, r.open = cut/land, open/land
			// A tile of channelled ground is a channel TileSpan long on
			// TileSpan² of ground.
			r.dd, r.ddOpen = r.share/TileSpan*1000, r.open/TileSpan*1000
			drainageOnce.rows = append(drainageOnce.rows, r)
		}
	})
	return drainageOnce.rows
}

// cutGround is how much of g's land the water cuts in a storm: the tiles on
// which a flood (floodFlow) puts more stress on the ground than shear says
// holds it, which is where the water takes ground and where a channel heads
// (Istanbulluoglu and Bras, 2005: the channel network is the ground over the
// threshold). And the land read, its rain and the stress holding it, summed.
func cutGround(g *Grid, shear func(int) float64) (cut, land, rain, tc float64) {
	recv, run := g.receivers()
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if t.Wet() || t.Terrain.Tidal() || g.sunk(i) || int(recv[i]) == i || t.Mark != None {
			continue
		}
		land++
		rain += g.Rain(i)
		tc += shear(i)
		q := g.Flow[i] * floodFlow
		fall := (g.Height[i] - g.Height[recv[i]]) / run[i]
		if fall > criticalFall(q, flowWidth(t, q, fall, run[i]), shear(i)) {
			cut++
		}
	}
	return cut, land, rain, tc
}

// Where the cover is thin the water cuts more of the ground than it would
// under a full cover, and thin cover is a dry country's. Read on the drawn
// valleys at each wetness, against the same valleys held everywhere by open
// ground's full cover: the share the cover adds is greatest in semi-arid
// ground and nothing in a humid one.
func TestTheCoverSetsWhereTheWaterCuts(t *testing.T) {
	rows := drainageByRain()
	var b strings.Builder
	fmt.Fprintf(&b, "wetness   rain, mm   cut share   under full cover   cover adds   DD, km/km²   mean τc, Pa\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "%7.2f   %8.0f   %9.4f   %16.4f   %10.2fx   %10.2f   %11.1f\n", r.wetness, r.rain, r.share, r.open, r.share/math.Max(r.open, 1e-9), r.dd, r.tcMn)
	}
	t.Logf("the valleys' ground the water cuts in a storm, by rain:\n%s", b.String())
	var dry, humid float64
	for _, r := range rows {
		add := r.share / math.Max(r.open, 1e-9)
		switch {
		case r.rain >= 250 && r.rain < 750 && r.open > 0:
			dry = math.Max(dry, add)
		case r.rain >= 1000:
			humid = math.Max(humid, add)
		}
	}
	if dry <= 1.5 || dry <= humid {
		t.Errorf("the cover adds %.2fx to the ground the water cuts in semi-arid valleys and %.2fx in humid ones: a thin cover is not a weak one", dry, humid)
	}
	if humid > 1.1 {
		t.Errorf("the cover adds %.2fx to the ground the water cuts in humid valleys, where it is whole", humid)
	}
}

// Wooded slopes stand steeper. A hillside of one even fall is laid over a
// valley's ground, with soil on it a slide fails through, and let slide as
// the ages of weather slide it: under a closed wood, under grass, and
// ploughed. At a fall of 1.3, between the foot of Roering and others' (1999)
// range of critical gradients and its top, the wood's roots hold it where it
// stands (rootRise), and the grass's and the field's do not.
//
// And the slopes the land's own cover lets its ground stand at, the valleys'
// and a small globe's: wooded ground, with trees or shrubs over half of it,
// against open ground, with herbs over half and woody plants under a tenth.
func TestWoodedSlopesStandSteeper(t *testing.T) {
	const fall = 1.3
	hill := func(cover Terrain, p PFT, leaf float64) float64 {
		g := yardWorld("valley", 1, DefaultTerms()).Clone()
		g.strata = nil // the rock alike everywhere: see stand
		for i := range g.Tiles {
			at := g.PosOf(i)
			g.Height[i] = fall * TileSpan * float64(at.X)
			g.Soil[i] = 1.5
			g.Tiles[i].Terrain = cover
			g.burned[i] = 0
			clear(g.vegCover[i*int(PFTs) : (i+1)*int(PFTs)])
			clear(g.vegLeaf[i*int(PFTs) : (i+1)*int(PFTs)])
			g.vegCover[i*int(PFTs)+int(p)] = uint8(math.Round(0.9 / coverStep))
			g.vegLeaf[i*int(PFTs)+int(p)] = uint8(math.Round(leaf / leafStep))
		}
		g.landslide(true)
		// The steepest the hillside stands, off the map's edges.
		var most float64
		for i := range g.Tiles {
			at := g.PosOf(i)
			if at.X < 2 || at.X >= g.W-2 || at.Y < 2 || at.Y >= g.H-2 {
				continue
			}
			most = math.Max(most, g.Slope(at))
		}
		return most
	}
	wood := hill(Forest, TemperateBroadleaf, 4)
	grass := hill(Grass, C3Grass, 2)
	field := hill(Field, C3Grass, 2)
	t.Logf("a hillside at a fall of %.2f, let slide: under a wood it stands at %.3f, under grass %.3f, ploughed %.3f (Critical %.2f, rootMost %.2f)", fall, wood, grass, field, Critical, rootMost)
	if wood < fall-1e-9 {
		t.Errorf("a wood's hillside at %.2f slid to %.3f: its roots hold nothing", fall, wood)
	}
	if grass >= fall || field >= fall {
		t.Errorf("a hillside at %.2f stands at %.3f under grass and %.3f ploughed: they hold as a wood does", fall, grass, field)
	}

	stands := func(gs []*Grid) (wooded, open []float64) {
		for _, g := range gs {
			for i := range g.Tiles {
				if !natural(g.Tiles[i].Terrain) {
					continue
				}
				herb, wood, _ := g.covers(i)
				at := math.Min(standMost*Critical, Critical+g.rootRise(i, float64(g.Soil[i]), Critical))
				switch {
				case wood >= 0.5:
					wooded = append(wooded, at)
				case herb >= 0.5 && wood < 0.1:
					open = append(open, at)
				}
			}
		}
		return wooded, open
	}
	deg := func(v []float64) string {
		if len(v) == 0 {
			return "none"
		}
		slices.Sort(v)
		m := v[len(v)/2]
		return fmt.Sprintf("%.3f (%.1f°) over %d tiles", m, math.Atan(m)*180/math.Pi, len(v))
	}
	read := func(what string, gs []*Grid) {
		w, o := stands(gs)
		t.Logf("%s: the median slope the ground stands at, wooded %s, open %s", what, deg(w), deg(o))
		if len(w) > 0 && len(o) > 0 {
			slices.Sort(w)
			slices.Sort(o)
			if w[len(w)/2] <= o[len(o)/2] {
				t.Errorf("%s: wooded ground stands at %.3f and open at %.3f", what, w[len(w)/2], o[len(o)/2])
			}
		}
	}
	read("valleys 1-3", valleys(3))
	if !testing.Short() {
		read("small globe 1", smallGlobes(1))
	}
}

// The ground a fire bares is the water's. Three valleys are worn an age as
// they grow, and again with a share of their ground burned every year: the
// slopes come down faster the more of them burns. And on a small globe, the
// ground its own fires burn, worn with them and without.
func TestFireOpensTheGroundToTheWater(t *testing.T) {
	lowering := func(g *Grid, burned func(i int) float64, on func(i int) bool) float64 {
		g = g.Clone()
		for i := range g.burned {
			g.burned[i] = uint16(math.Round(burned(i) / burnedStep))
		}
		was := heights(g)
		g.wear(ageYears)
		var lost, n float64
		for i := range g.Tiles {
			if on(i) {
				lost += was[i] - g.Height[i]
				n++
			}
		}
		return lost / math.Max(n, 1) / (ageYears / yr) * 1000
	}
	// The slopes: natural ground off the valley floors and the banks, as
	// ploughedAndWooded reads them.
	slopes := func(g *Grid) func(int) bool {
		return func(i int) bool {
			return natural(g.Tiles[i].Terrain) && g.Drain[i] > FloodDepth/2 && !g.HasNeighbor(g.PosOf(i), (*Tile).Wet)
		}
	}
	shares := []float64{0, 0.03, 0.1, 0.3}
	rates := make([]float64, len(shares))
	for seed := uint64(1); seed <= 3; seed++ {
		g := yardWorld("valley", seed, DefaultTerms())
		for k, b := range shares {
			rates[k] += lowering(g, func(int) float64 { return b }, slopes(g)) / 3
		}
	}
	var b strings.Builder
	for k, s := range shares {
		fmt.Fprintf(&b, " %.2f a year: %.4f mm/yr (%.2fx);", s, rates[k], rates[k]/rates[0])
	}
	t.Logf("valleys 1-3, slopes lowered with this share of the ground burned:%s", b.String())
	for k := 1; k < len(rates); k++ {
		if !(rates[k] > rates[k-1]) {
			t.Errorf("the slopes came down %.4f mm/yr with %.2f burned a year and %.4f with %.2f", rates[k], shares[k], rates[k-1], shares[k-1])
		}
	}
	if testing.Short() {
		return
	}
	g := smallGlobes(1)[0]
	burnt := func(i int) bool { return natural(g.Tiles[i].Terrain) && g.Burned(i) >= 0.05 }
	with := lowering(g, g.Burned, burnt)
	without := lowering(g, func(int) float64 { return 0 }, burnt)
	t.Logf("small globe 1, ground burning 5%% and more a year: lowered %.4f mm/yr with its fires and %.4f without, %.2fx", with, without, with/without)
	if !(with > without) {
		t.Errorf("small globe 1's burned ground came down %.4f mm/yr with its fires and %.4f without", with, without)
	}
}
