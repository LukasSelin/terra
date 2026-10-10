package terra

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The coasts' geometry, read off made globes for a change to the continents'
// outlines to be held against: how rough the coasts run (Richardson), how
// much they gather at right angles (cornerLock) or at the map's axes
// (gridLock), how remote the continents' interiors are, how the islands'
// sizes fall off, the land's share, how much of the continental crust is
// dry, and the longest straight run of a continent's coast over the root of
// its area. It also draws each globe's Elevation shaded, the coasts and the
// seaways to be looked at.
//
// It is a report and not a test: it runs only where TERRA_COASTGEO names a
// directory to write it to, and fails nothing.
func TestCoastGeometryReport(t *testing.T) {
	dir := os.Getenv("TERRA_COASTGEO")
	if dir == "" {
		t.Skip("TERRA_COASTGEO is not set")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	type world struct {
		name string
		g    *Grid
	}
	var full, small []world
	seeds := []uint64{1, 7}
	if s := os.Getenv("TERRA_COASTGEO_SEEDS"); s != "" {
		seeds = nil
		for _, f := range strings.Split(s, ",") {
			var v uint64
			fmt.Sscan(f, &v)
			seeds = append(seeds, v)
		}
	}
	for _, seed := range seeds {
		full = append(full, world{fmt.Sprintf("globe%d", seed), yardWorld("globe", seed, GlobeTerms())})
	}
	if os.Getenv("TERRA_COASTGEO_SMALL") != "0" {
		for seed := uint64(1); seed <= 8; seed++ {
			small = append(small, world{fmt.Sprintf("small%d", seed), plateWorld(seed)})
		}
	}
	var out strings.Builder
	row := func(w world) {
		gs := []*Grid{w.g}
		lm := landOf(w.g)
		fmt.Fprintf(&out, "%-8s land %.3f  crustDry %.3f  coastD %.3f  corner %.3f  grid %.3f  remote %s  korcak %.3f  coastRun %s  islands %d\n",
			w.name, lm.share, crustDry(w.g), coastDimension(gs), cornerLock(gs), gridLock(gs, landEdge),
			fmtList(remoteness(gs)), islandExponent(gs), fmtList(coastRuns(w.g, runCorridor)), len(lm.tiles))
	}
	for _, w := range full {
		row(w)
		if err := shadedRelief(w.g, filepath.Join(dir, w.name+".png")); err != nil {
			t.Error(err)
		}
	}
	for _, w := range small {
		row(w)
		if err := shadedRelief(w.g, filepath.Join(dir, w.name+".png")); err != nil {
			t.Error(err)
		}
	}
	pool := func(ws []world) []*Grid {
		var gs []*Grid
		for _, w := range ws {
			gs = append(gs, w.g)
		}
		return gs
	}
	for _, set := range []struct {
		name string
		ws   []world
	}{{"full", full}, {"small", small}} {
		if len(set.ws) == 0 {
			continue
		}
		gs := pool(set.ws)
		var runs []float64
		for _, g := range gs {
			runs = append(runs, coastRuns(g, runCorridor)...)
		}
		sort.Float64s(runs)
		fmt.Fprintf(&out, "pooled %-5s coastD %.3f  corner %.3f  grid %.3f  remote(med) %.3f  korcak %.3f  coastRun(med) %.3f\n",
			set.name, coastDimension(gs), cornerLock(gs), gridLock(gs, landEdge), quantile(remoteness(gs), 0.5),
			islandExponent(gs), quantile(runs, 0.5))
	}
	// The shape yardsticks' own readings, over the three globes they read.
	var three []*Grid
	for _, w := range full {
		for _, s := range []string{"globe1", "globe2", "globe3"} {
			if w.name == s {
				three = append(three, w.g)
			}
		}
	}
	if len(three) == 3 {
		far := math.NaN()
		for _, g := range three {
			if s := landOf(g).share; math.IsNaN(far) || math.Abs(s-earthLand) > math.Abs(far-earthLand) {
				far = s
			}
		}
		sh := shelvesOf(three)
		fmt.Fprintf(&out, "yardsticks (globes 1-3): land(far) %.3f  remote %.3f  korcak %.3f  coastD %.3f  grid %.3f  corner %.3f  shelf200 %.3f  shelfMean %.0f km  quiet/active %.2f  floorGrid %.3f\n",
			far, quantile(remoteness(three), 0.5), islandExponent(three), coastDimension(three), gridLock(three, landEdge),
			cornerLock(three), sh.share, sh.mean, sh.quiet/sh.active, gridLock(three, offshoreFloor))
	}
	if len(small) > 0 {
		var walls []float64
		for _, w := range small {
			walls = append(walls, wallRun(w.g, runCorridor))
		}
		fmt.Fprintf(&out, "plate walls (small 1-8): %s  median %.3f\n", fmtList(walls), quantile(walls, 0.5))
	}
	t.Log("\n" + out.String())
	if err := os.WriteFile(filepath.Join(dir, "report.txt"), []byte(out.String()), 0o644); err != nil {
		t.Error(err)
	}
}

func fmtList(v []float64) string {
	var s []string
	for _, x := range v {
		s = append(s, fmt.Sprintf("%.2f", x))
	}
	return "[" + strings.Join(s, " ") + "]"
}

// crustDry is the share of a world's continental crust, by the sphere's
// area, that stands above the sea.
func crustDry(g *Grid) float64 {
	var crust, dry float64
	for i := range g.Tiles {
		if !math.IsNaN(g.FloorAge(i)) {
			continue
		}
		w := rowWeight(g, i/g.W)
		crust += w
		if !g.underSea(i) {
			dry += w
		}
	}
	if crust == 0 {
		return math.NaN()
	}
	return dry / crust
}

// coastRuns is, for each continent of g, the longest stretch of its coast
// that stays inside a straight corridor wide tiles across, over the root of
// the continent's area: wallRun's reading, taken on the line between a
// continent and the sea rather than between two plates. A disc reads 0.48
// and a square 1.
func coastRuns(g *Grid, wide float64) []float64 {
	const bearings = 36
	lm := landOf(g)
	edge := map[int32][][2]float64{}
	for i, c := range lm.of {
		if c < 0 || lm.tiles[c] < continentLeast*lm.land || lm.polar[c] {
			continue
		}
		on := false
		g.eachNear(i, func(j int) {
			if lm.of[j] < 0 {
				on = true
			}
		})
		if on {
			edge[c] = append(edge[c], [2]float64{float64(i % g.W), float64(i / g.W)})
		}
	}
	keys := make([]int32, 0, len(edge))
	for c := range edge {
		keys = append(keys, c)
	}
	sort.Slice(keys, func(a, b int) bool { return keys[a] < keys[b] })
	var runs []float64
	for _, c := range keys {
		pts := edge[c]
		best := 0.0
		for b := 0; b < bearings; b++ {
			a := math.Pi * float64(b) / bearings
			ux, uy := math.Cos(a), math.Sin(a)
			for _, shift := range []float64{0, wide} {
				lanes := map[int][]float64{}
				for _, q := range pts {
					dx, dy := g.across(q[0]-pts[0][0]), q[1]-pts[0][1]
					lane := int(math.Floor((-dx*uy + dy*ux + shift) / (2 * wide)))
					lanes[lane] = append(lanes[lane], dx*ux+dy*uy)
				}
				for _, lane := range lanes {
					sort.Float64s(lane)
					run, from := 0.0, lane[0]
					for k := 1; k < len(lane); k++ {
						if lane[k]-lane[k-1] > 3 {
							from = lane[k]
							continue
						}
						run = math.Max(run, lane[k]-from)
					}
					best = math.Max(best, run)
				}
			}
		}
		runs = append(runs, best/math.Sqrt(lm.tiles[c]))
	}
	return runs
}

// shadedRelief draws g's Elevation lit from the north-west: the land in
// greens and browns by height, the sea in blues by depth.
func shadedRelief(g *Grid, path string) error {
	img := image.NewRGBA(image.Rect(0, 0, g.W, g.H))
	e := make([]float64, len(g.Tiles))
	for i := range e {
		e[i] = g.Elevation(i) - g.sea
	}
	hi := 1.0
	for _, v := range e {
		hi = math.Max(hi, v)
	}
	// Vertical exaggeration for the shading, in metres per tile.
	const z = 1.0 / 400
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			i := y*g.W + x
			l := e[y*g.W+g.WrapX(x-1)]
			r := e[y*g.W+g.WrapX(x+1)]
			u, d := e[i], e[i]
			if y > 0 {
				u = e[i-g.W]
			}
			if y < g.H-1 {
				d = e[i+g.W]
			}
			nx, ny := -(r-l)*z/2, -(d-u)*z/2
			n := math.Sqrt(nx*nx + ny*ny + 1)
			shade := (nx*-0.6 + ny*-0.6 + 0.53) / n / 0.53
			shade = math.Max(0.35, math.Min(1.3, shade))
			var c [3]float64
			if e[i] <= 0 {
				t := math.Min(1, -e[i]/6000)
				c = [3]float64{30 + 60*(1-t), 70 + 90*(1-t), 140 + 90*(1-t)}
			} else {
				t := math.Pow(e[i]/hi, 0.5)
				c = [3]float64{70 + 150*t, 130 + 60*t - 40*t*t, 60 + 80*t}
			}
			px := color.RGBA{A: 255}
			px.R = uint8(math.Min(255, c[0]*shade))
			px.G = uint8(math.Min(255, c[1]*shade))
			px.B = uint8(math.Min(255, c[2]*shade))
			img.Set(x, y, px)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// TestMarginProfileReport reads a history's last epoch across its continents'
// margins: by how far a tile lies from the edge of the continental crust, in
// tiles (inland positive), the mean, the 10th and the 90th percentile of its
// height over the history's sea, and the share of it under that sea. It runs
// only where TERRA_COASTGEO is set.
func TestMarginProfileReport(t *testing.T) {
	if os.Getenv("TERRA_COASTGEO") == "" {
		t.Skip("TERRA_COASTGEO is not set")
	}
	for _, c := range []struct {
		name  string
		seed  uint64
		terms Terms
	}{{"small1", 1, smallGlobe()}, {"small2", 2, smallGlobe()}, {"globe1", 1, GlobeTerms()}} {
		var out strings.Builder
		var last seaReading
		historySeas(c.seed, c.terms, func(g *Grid, cr *crust, e int) {
			if e != c.terms.Epochs-1 {
				return
			}
			last = cr.seas[len(cr.seas)-1]
			in, _ := g.nearestTo(func(i int) bool { return cr.ocean[i] }, false)
			off, _ := g.nearestTo(func(i int) bool { return !cr.ocean[i] }, false)
			bins := map[int][]float64{}
			for i := range g.Tiles {
				d := in[i]
				if cr.ocean[i] {
					d = -off[i]
				}
				b := int(math.Round(d))
				if b < -8 || b > 24 {
					continue
				}
				bins[b] = append(bins[b], g.Height[i]-g.base)
			}
			for b := -8; b <= 24; b++ {
				v := bins[b]
				if len(v) == 0 {
					continue
				}
				wet := 0
				sum := 0.0
				for _, x := range v {
					sum += x
					if x <= 0 {
						wet++
					}
				}
				fmt.Fprintf(&out, "%4d %7d mean %7.0f p10 %7.0f p50 %7.0f p90 %7.0f wet %.2f\n", b, len(v), sum/float64(len(v)),
					quantile(append([]float64(nil), v...), 0.1), quantile(append([]float64(nil), v...), 0.5),
					quantile(append([]float64(nil), v...), 0.9), float64(wet)/float64(len(v)))
			}
		})
		t.Logf("%s: land %.3f, continental crust drowned %.3f\n%s", c.name, last.land, last.shelf, out.String())
	}
}

// TestCoastByStageReport reads the coast's dimension and the land's share as
// each stage of a globe's making ends. It runs only where TERRA_COASTGEO is
// set.
func TestCoastByStageReport(t *testing.T) {
	dir := os.Getenv("TERRA_COASTGEO")
	if dir == "" {
		t.Skip("TERRA_COASTGEO is not set")
	}
	os.MkdirAll(dir, 0o755)
	_, err := MakeLandWatching(7, GlobeTerms(), nil, func(stage string, l *Land, g *Grid) {
		if g.sea < 0 {
			return
		}
		gs := []*Grid{g}
		t.Logf("%-7s land %.3f crustDry %.3f coastD %.3f corner %.3f", stage, landOf(g).share, crustDry(g), coastDimension(gs), cornerLock(gs))
		shadedRelief(g, filepath.Join(dir, "stage-"+stage+".png"))
	})
	if err != nil {
		t.Fatal(err)
	}
}
