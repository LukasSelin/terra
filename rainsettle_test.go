package terra

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// How far a history's ground moves for a nudge to its air (#84). The
// history's rain was found to be chaotic by the deep-time climate's cost
// study (#83): a hundredth of a degree on today's air moved the map's
// heights by some five hundred metres. This makes a history with its air as
// it is and again with the air warmer everywhere by each of airNudges, and
// logs the land's rain epoch by epoch and how far the heights it leaves have
// moved. It makes several histories and is run by hand:
//
//	TERRA_RAIN_SENSITIVITY=1 go test -run TestTheHistorysRainSensitivity -count=1 -v -timeout 60m .
//
// TERRA_RAIN_SENSITIVITY_TERMS is small, globe or both (the default).

// TERRA_RAIN_SENSITIVITY_NUDGES, comma-separated degrees, reads others.
var airNudges = nudgesFromEnv()

func nudgesFromEnv() []float64 {
	v := os.Getenv("TERRA_RAIN_SENSITIVITY_NUDGES")
	if v == "" {
		return []float64{0.01, 0.1, 0.3}
	}
	var out []float64
	for _, f := range strings.Split(v, ",") {
		var d float64
		if _, err := fmt.Sscan(f, &d); err == nil {
			out = append(out, d)
		}
	}
	return out
}

// nudgedHistory is the history of seed and terms with its air d degrees
// warmer everywhere: the land's rain each epoch, over the ground the epoch's
// air read as land, the heights it leaves, and how long it took.
func nudgedHistory(seed uint64, terms Terms, d float64) (rain []float64, height []float64, seconds float64) {
	epochWatch = func(g *Grid, _ *crust, _ []Plate, e int) {
		if e < 0 {
			return
		}
		var sum float64
		var n int
		for i := range g.Tiles {
			if len(g.aired) == len(g.Tiles) && g.aired[i] >= 0 {
				sum += g.rain[i]
				n++
			}
		}
		rain = append(rain, sum/math.Max(1, float64(n)))
		if e == terms.Epochs-1 {
			height = slices.Clone(g.Height)
		}
	}
	defer func() { epochWatch = nil }()
	w := unmade(seed, terms)
	g := w.newGround(terms)
	hg := w.historyGround(g, terms)
	if d != 0 {
		a := *hg.air
		a.Mean = slices.Clone(a.Mean)
		for y := range a.Mean {
			a.Mean[y] += d
		}
		hg.air = &a
	}
	start := time.Now()
	w.history(hg, terms.Epochs, terms.SeaShare, terms.Water)
	return rain, height, time.Since(start).Seconds()
}

func TestTheHistorysRainSensitivity(t *testing.T) {
	if os.Getenv("TERRA_RAIN_SENSITIVITY") != "1" {
		t.Skip("the history's rain sensitivity is read by hand: TERRA_RAIN_SENSITIVITY=1")
	}
	type world struct {
		name  string
		seed  uint64
		terms Terms
	}
	which := os.Getenv("TERRA_RAIN_SENSITIVITY_TERMS")
	var worlds []world
	if which != "globe" {
		for _, s := range []uint64{1, 2, 3} {
			worlds = append(worlds, world{fmt.Sprintf("small globe %d", s), s, smallGlobe()})
		}
	}
	if which != "small" {
		worlds = append(worlds, world{"globe 1", 1, GlobeTerms()})
	}
	var out strings.Builder
	fmt.Fprintf(&out, "\n| world | nudge C | height RMS m | land rain, worst epoch ratio | epochs off by >10%% | epochs off by 2x | history s |\n|---|---|---|---|---|---|---|\n")
	var epochs strings.Builder
	for _, wd := range worlds {
		rain0, h0, s0 := nudgedHistory(wd.seed, wd.terms, 0)
		fmt.Fprintf(&out, "| %s | 0 | - | - | - | - | %.2f |\n", wd.name, s0)
		fmt.Fprintf(&epochs, "\n%s, land rain mm by epoch:\n  0     :", wd.name)
		for _, r := range rain0 {
			fmt.Fprintf(&epochs, " %5.0f", r)
		}
		for _, d := range airNudges {
			rain, h, s := nudgedHistory(wd.seed, wd.terms, d)
			var d2 float64
			for i := range h {
				x := h[i] - h0[i]
				d2 += x * x
			}
			worst, off10, off2 := 1.0, 0, 0
			fmt.Fprintf(&epochs, "\n  %-6.2f:", d)
			for e := range rain {
				fmt.Fprintf(&epochs, " %5.0f", rain[e])
				r := rain[e] / rain0[e]
				if r < 1 {
					r = 1 / r
				}
				worst = math.Max(worst, r)
				if r > 1.1 {
					off10++
				}
				if r >= 2 {
					off2++
				}
			}
			fmt.Fprintf(&out, "| %s | %g | %.1f | %.3f | %d of %d | %d | %.2f |\n",
				wd.name, d, math.Sqrt(d2/float64(len(h))), worst, off10, len(rain), off2, s)
			t.Logf("%s +%.2f: done", wd.name, d)
		}
		epochs.WriteString("\n")
	}
	t.Log(out.String())
	t.Log(epochs.String())
}
