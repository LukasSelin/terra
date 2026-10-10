package terra

import (
	"fmt"
	"math"
	"os"
	"slices"
	"testing"
)

// TestWetlandBreakdown breaks the first globe's wetland down by the rate its
// rock rises, how far it lies from the sea and how deep its alluvium is, and
// reads what the wetland would be with the alluvium gone, halved or doubled.
// It is a measurement and holds nothing: TERRA_WETDIAG=1 runs it.
func TestWetlandBreakdown(t *testing.T) {
	if os.Getenv("TERRA_WETDIAG") == "" {
		t.Skip()
	}
	g := yardWorld("globe", 1, GlobeTerms())
	n := len(g.Tiles)
	coast := g.awayFrom(func(i int) bool { return g.underSea(i) })
	var ups []float64
	for i := range g.Tiles {
		if !g.underSea(i) && i < len(g.uplift) {
			ups = append(ups, math.Max(0, g.uplift[i]))
		}
	}
	slices.Sort(ups)
	q := func(v []float64, p float64) float64 { return v[int(p*float64(len(v)-1))] }
	p95 := q(ups, 0.95)
	fmt.Printf("uplift land: p10 %.3g p25 %.3g p50 %.3g p75 %.3g p90 %.3g p95 %.3g p99 %.3g\n",
		q(ups, .1), q(ups, .25), q(ups, .5), q(ups, .75), q(ups, .9), p95, q(ups, .99))
	type bin struct{ land, wet, peat, slope, fill float64 }
	byUp := map[int]*bin{}
	byCoast := map[int]*bin{}
	byFill := map[int]*bin{}
	var land, wet, peat float64
	var elev []float64
	for i := range g.Tiles {
		tt := &g.Tiles[i]
		if g.underSea(i) || tt.Wet() || tt.Terrain.Tidal() || g.Barren(g.PosOf(i)) {
			continue
		}
		w := math.Cos(latitudeOf(g, i/g.W) * math.Pi / 180)
		land += w
		ww, pp := 0.0, 0.0
		if g.Wetland(i) {
			ww = w
		}
		if g.Peatland(i) {
			pp = w
		}
		wet += ww
		peat += pp
		elev = append(elev, g.Elevation(i)-g.sea)
		r := math.Max(0, g.uplift[i]) / p95
		ub := -1
		if r > 0 {
			ub = int(math.Floor(math.Log10(r) * 2))
		}
		cb := int(math.Min(coast[i], 64))
		switch {
		case cb > 16:
			cb = 32
		case cb > 4:
			cb = 8
		}
		fb := int(g.fillAt(i) / 10)
		for _, m := range []struct {
			m map[int]*bin
			k int
		}{{byUp, ub}, {byCoast, cb}, {byFill, fb}} {
			b := m.m[m.k]
			if b == nil {
				b = &bin{}
				m.m[m.k] = b
			}
			b.land += w
			b.wet += ww
			b.peat += pp
			b.slope += w * g.Slope(g.PosOf(i))
			b.fill += w * g.fillAt(i)
		}
	}
	fmt.Printf("land %.0f tiles of %d: wetland %.4f peat %.4f\n", land, n, wet/land, peat/land)
	slices.Sort(elev)
	fmt.Printf("elevation land: p05 %.0f p10 %.0f p25 %.0f p50 %.0f; share<100m %.3f\n",
		q(elev, .05), q(elev, .1), q(elev, .25), q(elev, .5), shareBelow(elev, 100))
	show := func(name string, m map[int]*bin) {
		var ks []int
		for k := range m {
			ks = append(ks, k)
		}
		slices.Sort(ks)
		fmt.Println(name)
		for _, k := range ks {
			b := m[k]
			fmt.Printf("  %4d  land %.3f  wet %.3f (of all %.4f)  peat %.3f  slope %.4f  fill %.1f\n",
				k, b.land/land, b.wet/b.land, b.wet/land, b.peat/b.land, b.slope/b.land, b.fill/b.land)
		}
	}
	{
		var sl, tw []float64
		for i := range g.Tiles {
			tt := &g.Tiles[i]
			if g.underSea(i) || tt.Wet() || tt.Terrain.Tidal() || g.Barren(g.PosOf(i)) {
				continue
			}
			sl = append(sl, g.Slope(g.PosOf(i)))
			tw = append(tw, g.twi(i))
		}
		slices.Sort(sl)
		slices.Sort(tw)
		fmt.Printf("slope land: p05 %.4f p10 %.4f p25 %.4f p50 %.4f  twi p50 %.2f p90 %.2f p95 %.2f\n", q(sl, .05), q(sl, .1), q(sl, .25), q(sl, .5), q(tw, .5), q(tw, .9), q(tw, .95))
		fill := g.fill
		for _, k := range []float64{0, 0.5, 2} {
			g.fill = make([]float32, len(fill))
			for i := range fill {
				g.fill[i] = fill[i] * float32(k)
			}
			var l, w float64
			for i := range g.Tiles {
				tt := &g.Tiles[i]
				if g.underSea(i) || tt.Wet() || tt.Terrain.Tidal() || g.Barren(g.PosOf(i)) {
					continue
				}
				c := math.Cos(latitudeOf(g, i/g.W) * math.Pi / 180)
				l += c
				if g.Wetland(i) {
					w += c
				}
			}
			fmt.Printf("fill x%.1f: wetland %.4f\n", k, w/l)
		}
		g.fill = fill
	}
	show("by log10(uplift/p95)*2 (-1: none)", byUp)
	show("by tiles from the sea", byCoast)
	show("by fill/10 m", byFill)
}

func shareBelow(sorted []float64, below float64) float64 {
	k, _ := slices.BinarySearch(sorted, below)
	return float64(k) / float64(len(sorted))
}
