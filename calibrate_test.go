package terra

import (
	"fmt"
	"math"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/LukasSelin/terra/geom"
)

// The numbers the water and the weather are tuned against, printed rather
// than asserted. The tests elsewhere say what must hold; this says where
// everything actually stands, so that a change to the hydrology can be held
// against what was there before it. It takes a couple of minutes, so it runs
// only when asked:
//
//	TERRA_CALIBRATE=1 go test -run TestCalibrate -v
func TestCalibrate(t *testing.T) {
	if os.Getenv("TERRA_CALIBRATE") == "" {
		t.Skip("set TERRA_CALIBRATE=1 to print the calibration table")
	}

	fmt.Println("valley, seeds 1-5, as made:")
	fmt.Println("  seed  wet%   upland%  broken%  ribs%   maxQ       landP   landR")
	for seed := uint64(1); seed <= 5; seed++ {
		g := NewLand(seed, DefaultTerms()).Grid
		fmt.Printf("  %d     %5.2f  %6.1f   %6.1f   %5.1f   %9.3g  %6.0f  %6.0f\n", seed,
			100*wetShare(g), 100*riverUpland(g), 100*brokenShare(g), 100*ribShare(g),
			maxFlow(g), meanLand(g, g.rain), meanLand(g, g.runoff))
	}

	fmt.Println("\nvalley, after the weather:")
	{
		w := NewLandSized(3, 60, 40)
		g := w.Grid
		before := heights(g)
		high, low := quantile(append([]float64(nil), before...), 0.85), quantile(append([]float64(nil), before...), 0.15)
		for age := 0; age < 40; age++ {
			w.Erode()
		}
		var up, down float64
		for i := range g.Tiles {
			d := g.Tiles[i].Height - before[i]
			switch {
			case before[i] >= high:
				up += d
			case before[i] <= low:
				down += d
			}
		}
		fmt.Printf("  seed 3 60x40, 40 ages: high ground %+.1f m, low ground %+.1f m\n", up, down)
	}
	{
		lost := func(cover Terrain) float64 {
			w := NewLandSized(3, 60, 40)
			g := w.Grid
			var slopes []int
			for i := range g.Tiles {
				if tl := &g.Tiles[i]; tl.Terrain != Water && tl.Drain > FloodDepth/2 {
					tl.Terrain = cover
					slopes = append(slopes, i)
				}
			}
			g.Rekind()
			before := heights(g)
			for age := 0; age < 40; age++ {
				w.Erode()
			}
			var total float64
			for _, i := range slopes {
				if d := before[i] - g.Tiles[i].Height; d > 0 {
					total += d
				}
			}
			return total
		}
		wooded, ploughed := lost(Forest), lost(Field)
		fmt.Printf("  ploughed slopes lost %.1f m, wooded %.1f m: ratio %.2f\n", ploughed, wooded, ploughed/wooded)
	}
	{
		w := NewLand(5, DefaultTerms())
		for age := 0; age < 12; age++ {
			w.Erode()
		}
		g := w.Grid
		var floor, hill, fN, hN float64
		for i := range g.Tiles {
			tl := &g.Tiles[i]
			if tl.Wet() {
				continue
			}
			switch {
			case tl.Drain < FloodDepth/2:
				floor, fN = floor+tl.Sand, fN+1
			case g.Slope(g.PosOf(i)) > 0.1:
				hill, hN = hill+tl.Sand, hN+1
			}
		}
		fmt.Printf("  seed 5, 12 ages: sand on the floor %.3f, on the hillsides %.3f (margin %.3f)\n",
			floor/fN, hill/hN, hill/hN-floor/fN)
	}
	for seed := uint64(1); seed <= 3; seed++ {
		w := NewLand(seed, DefaultTerms())
		young, _ := bends(w.Grid)
		for k := 0; k < 40; k++ {
			w.Erode()
		}
		old, n := bends(w.Grid)
		fmt.Printf("  seed %d bends %.3f -> %.3f over %d river tiles; wet %.2f%%\n", seed, young, old, n, 100*wetShare(w.Grid))
	}

	fmt.Println("\nancient valley against drawn, seeds 1-12:")
	{
		var steep []float64
		var wet []float64
		for seed := uint64(1); seed <= 12; seed++ {
			drawn := shapeOf(NewLand(seed, DefaultTerms()).Grid)
			made := shapeOf(NewLand(seed, historyConfig(16)).Grid)
			steep = append(steep, made.slope90/drawn.slope90)
			wet = append(wet, float64(made.wet)/math.Max(1, float64(drawn.wet)))
		}
		sort.Float64s(steep)
		sort.Float64s(wet)
		fmt.Printf("  slope90 ratio median %.2f (range %.2f-%.2f); wet ratio median %.2f max %.2f\n",
			steep[len(steep)/2], steep[0], steep[len(steep)-1], wet[len(wet)/2], wet[len(wet)-1])
	}

	fmt.Println("\nrain by latitude on land, GlobeTerms seed 1 (mm a year: rain / runoff):")
	if g := NewLand(1, GlobeTerms()).Grid; g.air != nil && len(g.rain) == len(g.Tiles) {
		for _, band := range [][2]float64{{0, 10}, {10, 20}, {20, 30}, {30, 40}, {40, 55}, {55, 65}, {65, 80}, {80, 90}} {
			var p, r, n float64
			for i := range g.Tiles {
				lat := math.Abs(g.air.lat[i/g.W])
				if g.underSea(i) || lat < band[0] || lat >= band[1] {
					continue
				}
				p, r, n = p+g.rain[i], r+g.runoff[i], n+1
			}
			fmt.Printf("  %2.0f-%2.0f  %6.0f / %5.0f   over %.0f tiles\n", band[0], band[1], p/n, r/n, n)
		}
		fmt.Printf("  greatest river %.3g\n", maxFlow(g))
	}

	fmt.Println("\nthe shore, small globes and the full one:")
	for _, c := range []struct {
		name  string
		terms Terms
		seed  uint64
	}{{"small 1", smallGlobe(), 1}, {"small 2", smallGlobe(), 2}, {"small 3", smallGlobe(), 3}, {"globe 1", GlobeTerms(), 1}} {
		g := NewLand(c.seed, c.terms).Grid
		var ranges []float64
		flats := 0
		for i := range g.Tiles {
			if g.Tiles[i].Terrain == Flat {
				flats++
			}
			if r := g.TidalRange(g.PosOf(i)); r > 0 && !g.underSea(i) {
				ranges = append(ranges, r)
			}
		}
		sort.Float64s(ranges)
		if len(ranges) == 0 {
			ranges = []float64{0}
		}
		fmt.Printf("  %-8s flats %.2f%%  spring range on the coast p50 %.2f p90 %.2f max %.2f m\n", c.name,
			100*float64(flats)/float64(len(g.Tiles)), ranges[len(ranges)/2], ranges[len(ranges)*9/10], ranges[len(ranges)-1])
	}

	fmt.Println("\nsmall globe, seeds 1-3:")
	fmt.Println("  seed  secs  slope50  slope90  wet%land  ribs%  sandstone%  maxQ       landP   landR")
	for seed := uint64(1); seed <= 3; seed++ {
		start := time.Now()
		g := NewLand(seed, smallGlobe()).Grid
		took := time.Since(start).Seconds()
		var slopes []float64
		land, sandstone := 0, 0
		for i := range g.Tiles {
			if g.underSea(i) {
				continue
			}
			land++
			slopes = append(slopes, g.Slope(g.PosOf(i)))
			if g.Tiles[i].Bedrock == Sandstone {
				sandstone++
			}
		}
		fmt.Printf("  %d     %4.1f  %.4f   %.4f   %6.2f    %5.1f  %6.1f      %9.3g  %6.0f  %6.0f\n", seed, took,
			quantile(append([]float64(nil), slopes...), 0.5), quantile(slopes, 0.9),
			100*wetShare(g), 100*ribShare(g), 100*float64(sandstone)/float64(land), maxFlow(g),
			meanLand(g, g.rain), meanLand(g, g.runoff))
	}
}

// wetShare is how much of the land is water that is not the sea.
func wetShare(g *Grid) float64 {
	land, wet := 0, 0
	for i := range g.Tiles {
		if g.underSea(i) {
			continue
		}
		land++
		if g.Tiles[i].Wet() {
			wet++
		}
	}
	return float64(wet) / math.Max(1, float64(land))
}

// riverUpland is how much of the river stands in the high fifth of the map.
func riverUpland(g *Grid) float64 {
	hs := heights(g)
	high := quantile(append([]float64(nil), hs...), 1-uplandShare)
	wet, up := 0, 0
	for i := range g.Tiles {
		if g.Tiles[i].Wet() && !g.underSea(i) {
			wet++
			if hs[i] >= high {
				up++
			}
		}
	}
	return float64(up) / math.Max(1, float64(wet))
}

// brokenShare is how much of the river sends its water onto dry ground.
func brokenShare(g *Grid) float64 {
	rivers, broken := 0, 0
	for i := range g.Tiles {
		if !g.Tiles[i].Wet() || g.underSea(i) {
			continue
		}
		rivers++
		p := g.PosOf(i)
		a := g.Aspect(p)
		q := g.Norm(geom.Pos{X: p.X + a.X, Y: p.Y + a.Y})
		if a == (geom.Pos{}) || !g.In(q) {
			continue
		}
		if !g.At(q).Wet() {
			broken++
		}
	}
	return float64(broken) / math.Max(1, float64(rivers))
}

// ribShare is how much of the river has another channel within three tiles
// that its water does not meet within nine steps: streams side by side.
func ribShare(g *Grid) float64 {
	down := func(i int) int {
		p := g.PosOf(i)
		a := g.Aspect(p)
		q := geom.Pos{X: p.X + a.X, Y: p.Y + a.Y}
		if a == (geom.Pos{}) || !g.In(q) {
			return -1
		}
		return g.Index(q)
	}
	wet, ribs := 0, 0
	for i := range g.Tiles {
		if !g.Tiles[i].Wet() || g.underSea(i) {
			continue
		}
		wet++
		var path []int
		for j, k := i, 0; j >= 0 && k <= 9; j, k = down(j), k+1 {
			path = append(path, j)
		}
		on := func(j int) bool {
			for _, x := range path {
				if x == j {
					return true
				}
			}
			return false
		}
		p := g.PosOf(i)
		found := false
		for dy := -3; dy <= 3 && !found; dy++ {
			for dx := -3; dx <= 3 && !found; dx++ {
				q := geom.Pos{X: p.X + dx, Y: p.Y + dy}
				if !g.In(q) {
					continue
				}
				w := g.Index(q)
				if !g.Tiles[w].Wet() || g.underSea(w) || on(w) {
					continue
				}
				joins := false
				for j, k := w, 0; j >= 0 && k <= 9; j, k = down(j), k+1 {
					if on(j) {
						joins = true
						break
					}
				}
				found = !joins
			}
		}
		if found {
			ribs++
		}
	}
	return float64(ribs) / math.Max(1, float64(wet))
}

func maxFlow(g *Grid) float64 {
	most := 0.0
	for i := range g.Tiles {
		most = math.Max(most, g.Tiles[i].Flow)
	}
	return most
}

// meanLand is the mean of a per-tile reading over the land, or nothing where
// the reading does not exist yet.
func meanLand(g *Grid, v []float64) float64 {
	if len(v) != len(g.Tiles) {
		return math.NaN()
	}
	sum, n := 0.0, 0
	for i := range g.Tiles {
		if !g.underSea(i) {
			sum, n = sum+v[i], n+1
		}
	}
	return sum / math.Max(1, float64(n))
}
