package terra

import (
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// An epoch run in steps is still read once: its sea and its weather's
// denudation are one reading each an epoch, and every tile has a height. That
// a history in one step an epoch is the history before there were steps is
// TestWorldDigest's to prove. See epochSteps.
func TestAnEpochInSeveralStepsKeepsOneReadingAnEpoch(t *testing.T) {
	was := historySteps
	historySteps = 4
	defer func() { historySteps = was }()
	terms := GlobeTerms()
	terms.Width, terms.Height = 64, 32
	var cr *crust
	epochWatch = func(g *Grid, c *crust, plates []Plate, epoch int) { cr = c }
	defer func() { epochWatch = nil }()
	l := NewLand(1, terms)
	if len(cr.seas) != terms.Epochs || len(cr.denuded) != terms.Epochs {
		t.Errorf("%d epochs read %d seas and %d denudations: want one of each an epoch", terms.Epochs, len(cr.seas), len(cr.denuded))
	}
	for i, h := range l.Grid.Height {
		if math.IsNaN(h) || math.IsInf(h, 0) {
			t.Fatalf("tile %d stands at %v", i, h)
		}
	}
}

// TestHistoryStepsStudy makes globes in several counts of steps an epoch and
// reads what the steps buy and cost: the history's time, the land's
// hypsometry and the width of its high ground in the history's tiles. It is a
// study and not a test, and runs only where TERRA_STEPS_STUDY is set:
//
//	TERRA_STEPS_STUDY=1,2,4,8 go test -run TestHistoryStepsStudy -timeout 60m .
//
// TERRA_STEPS_WIDTH is the globe's width (256 if unset; 0 for the preset's),
// and TERRA_STEPS_SEEDS how many seeds (2 if unset).
func TestHistoryStepsStudy(t *testing.T) {
	list := os.Getenv("TERRA_STEPS_STUDY")
	if list == "" {
		t.Skip("set TERRA_STEPS_STUDY to the counts of steps to read, e.g. 1,2,4,8")
	}
	width, seeds := 256, 2
	if s := os.Getenv("TERRA_STEPS_WIDTH"); s != "" {
		width, _ = strconv.Atoi(s)
	}
	if s := os.Getenv("TERRA_STEPS_SEEDS"); s != "" {
		seeds, _ = strconv.Atoi(s)
	}
	terms := GlobeTerms()
	if width > 0 {
		terms.Width, terms.Height = width, width/2
	}
	was := historySteps
	defer func() { historySteps = was; epochWatch = nil }()
	t.Logf("globe %dx%d, %d epochs, %d seeds; history and making in seconds, widths in history tiles", terms.Width, terms.Height, terms.Epochs, seeds)
	t.Logf("%5s %5s %8s %8s %6s %6s %6s %6s %7s %7s %7s %7s", "steps", "seed", "history", "making", "land", "hyps", "cont", "ocean", "high", "active", "relief", "top")
	for _, f := range strings.Split(list, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || n < 1 {
			t.Fatalf("TERRA_STEPS_STUDY=%q: %q is not a count of steps", list, f)
		}
		historySteps = n
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			var began, ended time.Time
			var r orogenReading
			epochWatch = func(g *Grid, cr *crust, plates []Plate, epoch int) {
				switch epoch {
				case -1:
					began = time.Now()
				case terms.Epochs - 1:
					ended = time.Now()
					r = readOrogens(g, cr)
				}
			}
			start := time.Now()
			l := NewLand(seed, terms)
			making := time.Since(start)
			cont, ocean := hypsometricModes([]*Grid{l.Grid})
			land := 0.0
			for i := range l.Grid.Tiles {
				if !l.Grid.underSea(i) {
					land++
				}
			}
			t.Logf("%5d %5d %8.1f %8.1f %6.3f %6.3f %6.2f %6.2f %7.2f %7.2f %7.0f %7.0f", n, seed,
				ended.Sub(began).Seconds(), making.Seconds(), land/float64(len(l.Grid.Tiles)),
				meanHypsometry([]*Grid{l.Grid}), cont, ocean, r.high, r.active, r.relief, r.top)
		}
	}
}

// orogenReading is how wide a history's high ground is at its end, in its
// tiles: see readOrogens.
type orogenReading struct {
	// high is the width of the land over the ninetieth percentile of the
	// land's height, and active of the land rising faster than activeRise,
	// each read as twice its area over its edge: a band w wide and long
	// against w has an area of w a unit of length and an edge of two.
	high, active float64
	// relief is the mean height of the high ground over the land's median,
	// and top its ninety-ninth percentile over the sea, in metres.
	relief, top float64
}

// activeRise is how fast land has to be rising to be read as a range being
// raised: a millimetre a year, a quarter of a collision's rate.
const activeRise = 1 * mm / yr

func readOrogens(g *Grid, cr *crust) orogenReading {
	var heights []float64
	for i := range g.Tiles {
		if g.Height[i] > g.base {
			heights = append(heights, g.Height[i])
		}
	}
	if len(heights) == 0 {
		return orogenReading{}
	}
	slices.Sort(heights)
	at := func(q float64) float64 { return heights[min(len(heights)-1, int(q*float64(len(heights))))] }
	p50, p90, p99 := at(0.5), at(0.9), at(0.99)
	width := func(in func(i int) bool) float64 {
		area, edge := 0, 0
		for i := range g.Tiles {
			if !in(i) {
				continue
			}
			area++
			x, y := i%g.W, i/g.W
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				qx, qy := x+d[0], y+d[1]
				if qy < 0 || qy >= g.H {
					edge++
					continue
				}
				if !in(qy*g.W + g.WrapX(qx)) {
					edge++
				}
			}
		}
		if edge == 0 {
			return 0
		}
		return 2 * float64(area) / float64(edge)
	}
	high := func(i int) bool { return g.Height[i] > p90 }
	r := orogenReading{
		high:   width(high),
		active: width(func(i int) bool { return g.Height[i] > g.base && cr.rise[i] > activeRise }),
		top:    p99 - g.base,
	}
	n := 0.0
	for i := range g.Tiles {
		if high(i) {
			r.relief += g.Height[i] - p50
			n++
		}
	}
	if n > 0 {
		r.relief /= n
	}
	return r
}
