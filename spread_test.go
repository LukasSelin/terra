package terra

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
)

// A reading off one world and a reading off another made the same way are
// not the same reading. The river network a small globe ends with is its
// history's, and a change anywhere upstream of it - the rain, the rock, how
// the plates were cut - redraws every network on every seed: the reading
// moves as far as one globe's rivers differ from another's, whether the
// change made the world any more like the earth or not. Read pooled over a
// handful of globes, the network readings swapped between pass and fail from
// one branch to the next: the small globes' concavity, pooled over eight,
// reads from -0.03 to 0.95 one globe at a time (#74).
//
// So the readings a history's chaos moves are read over seeds, with an
// interval, and a yardstick so read fails only when its interval lies
// wholly outside the band: when the seeds say the world is outside it, and
// not when one draw of them does. The reading and its interval are had one
// of two ways:
//
//   - seed by seed (the default): each seed's worlds give a reading of
//     their own, fitted on their own, and the yardstick reads the median of
//     them, its interval the seeds' readings either side of it that hold the
//     median of all seeds with 90% (spreadOf). A pooled fit is decided by
//     the one or two globes with the most ground in its bins; the median is
//     not, and nor is its interval by a seed whose reading is wild.
//   - pooled: one fit over every seed's worlds together, as the yardstick
//     read before, its interval 95% by Student's t on the jackknife's
//     standard error over the seeds - how far the fit moves with each seed
//     left out. This is for a reading that is not the same reading off one
//     globe: a fit to a handful of bins of one globe's basin is a worse fit
//     (Flint's R2) or a much noisier one (Hack's exponent) than the fit to
//     the same bins over sixteen globes.
//
// A reading whose interval on main is wider than its band cannot tell a
// world in the band from one outside it, and seeds enough to narrow it would
// cost more than the suite has (docs/perf/suite.md). It is advisory: read
// and logged with its interval, never a failure. docs/yardsticks.md lists
// every reading so read, how much it scatters and whether it gates.

// seeded is a reading taken over seeds.
type seeded struct {
	// worlds is the worlds of each seed: its one world, or for a reading
	// that compares two resolutions, its double and its single.
	worlds func() [][]*Grid
	// read is the reading over any set of seeds, their worlds pooled.
	read func(seeds [][]*Grid) float64
	// pooled gates on the reading over all the seeds together and its
	// jackknife interval, rather than on the median of the seeds' own.
	pooled bool
}

// spread is a reading over its seeds, with its interval.
type spread struct {
	seeds  []float64 // one reading a seed (NaN where a seed gave none); for a pooled reading, each seed left out
	median float64   // the reading: the median of the seeds, or the pooled fit
	se     float64   // its standard error; 0 for a reading of one figure
	lo, hi float64   // the interval: the reading -+ t95(n-1) se
	n      int       // seeds with a reading
}

// tQuantile95 is Student's t at 97.5% for df degrees of freedom: the
// half-width, in standard errors, of a 95% interval.
func tQuantile95(df int) float64 {
	table := []float64{math.Inf(1), 12.706, 4.303, 3.182, 2.776, 2.571, 2.447, 2.365, 2.306, 2.262,
		2.228, 2.201, 2.179, 2.160, 2.145, 2.131, 2.120, 2.110, 2.101, 2.093, 2.086}
	switch {
	case df < 0:
		return math.Inf(1)
	case df < len(table):
		return table[df]
	case df < 30:
		return 2.06
	default:
		return 1.96
	}
}

// medianCover is how sure the interval of a median of seeds is to hold the
// median of every seed there could be.
const medianCover = 0.9

// spreadOf is the median of the seeds' readings and its interval: the k-th
// least reading to the k-th greatest, for the greatest k that holds the
// median of all seeds with medianCover or more (Conover 1999, 3.2). It asks
// nothing of how the readings scatter, and one seed's river network drawn
// three times as long as the rest moves it no further than any other seed
// above the median would: the readings these are for have such seeds. Where
// no k is that sure - three seeds are 75% sure of their median at most - it
// is the least to the greatest.
func spreadOf(seeds []float64) spread {
	v := numbers(seeds)
	s := spread{seeds: seeds, n: len(v), median: math.NaN(), se: math.NaN(), lo: math.NaN(), hi: math.NaN()}
	if len(v) == 0 {
		return s
	}
	s.median = medianOf(v)
	if len(v) == 1 {
		s.lo, s.hi = math.Inf(-1), math.Inf(1)
		return s
	}
	sorted := slices.Clone(v)
	slices.Sort(sorted)
	k := medianRank(len(v), medianCover)
	s.lo, s.hi = sorted[k-1], sorted[len(v)-k]
	return s
}

// medianRank is the greatest k for which the k-th least and k-th greatest of
// n readings hold their median with cover or more: 1 - 2 P(B < k) for B the
// count of n fair coins that come up heads. It is 1 where none does.
func medianRank(n int, cover float64) int {
	best := 1
	below := 0.0                   // P(B < k)
	p := math.Pow(0.5, float64(n)) // P(B = k-1)
	for k := 1; 2*k <= n+1; k++ {
		below += p
		p *= float64(n-k+1) / float64(k)
		if 1-2*below >= cover {
			best = k
		}
	}
	return best
}

// widen sets the interval from the reading and its standard error. One seed
// says nothing of how far another would fall from it: its interval is
// everything.
func (s *spread) widen() {
	if s.n < 2 || math.IsNaN(s.se) {
		s.se, s.lo, s.hi = math.Inf(1), math.Inf(-1), math.Inf(1)
		return
	}
	half := tQuantile95(s.n-1) * s.se
	s.lo, s.hi = s.median-half, s.median+half
}

// pointSpread is a reading of one figure with nothing to say how it scatters:
// its interval is the point.
func pointSpread(x float64) spread {
	n := 1
	if math.IsNaN(x) {
		n = 0
	}
	return spread{seeds: []float64{x}, median: x, se: 0, lo: x, hi: x, n: n}
}

func numbers(v []float64) []float64 {
	var out []float64
	for _, x := range v {
		if !math.IsNaN(x) {
			out = append(out, x)
		}
	}
	return out
}

// medianOf is the middle of v, the mean of the middle two where there are an
// even number.
func medianOf(v []float64) float64 {
	c := slices.Clone(v)
	slices.Sort(c)
	n := len(c)
	if n%2 == 1 {
		return c[n/2]
	}
	return (c[n/2-1] + c[n/2]) / 2
}

// sdOf is the sample standard deviation.
func sdOf(v []float64) float64 {
	if len(v) < 2 {
		return math.NaN()
	}
	m := meanOf(v)
	ss := 0.0
	for _, x := range v {
		ss += (x - m) * (x - m)
	}
	return math.Sqrt(ss / float64(len(v)-1))
}

// outside is whether the interval lies wholly outside lo-hi, and so whether
// the seeds say the world is outside the band. A reading with no seed is
// outside every band.
func (s spread) outside(lo, hi float64) bool {
	return s.n == 0 || math.IsNaN(s.median) || s.hi < lo || s.lo > hi
}

// within is whether the interval lies wholly inside lo-hi.
func (s spread) within(lo, hi float64) bool {
	return s.n > 0 && s.lo >= lo && s.hi <= hi
}

func (s spread) String() string {
	switch {
	case s.se == 0:
		return fmt.Sprintf("%.4g", s.median)
	case math.IsNaN(s.se):
		return fmt.Sprintf("%.4g [%.4g, %.4g] over %d seeds", s.median, s.lo, s.hi, s.n)
	}
	return fmt.Sprintf("%.4g [%.4g, %.4g] over %d seeds (se %.3g)", s.median, s.lo, s.hi, s.n, s.se)
}

// bySeed is the reading seed by seed and the median of them.
func (r seeded) bySeed() spread {
	ws := r.worlds()
	v := make([]float64, len(ws))
	for k := range ws {
		v[k] = r.read(ws[k : k+1])
	}
	return spreadOf(v)
}

// jackknife is the reading over every seed pooled, and its interval by the
// jackknife: how far it moves with each seed left out.
func (r seeded) jackknife() spread {
	ws := r.worlds()
	s := spread{median: r.read(ws), n: len(ws)}
	for k := range ws {
		s.seeds = append(s.seeds, r.read(append(slices.Clone(ws[:k]), ws[k+1:]...)))
	}
	if loo := numbers(s.seeds); len(loo) == len(ws) && len(ws) > 1 {
		m := meanOf(loo)
		ss := 0.0
		for _, x := range loo {
			ss += (x - m) * (x - m)
		}
		s.se = math.Sqrt(float64(len(ws)-1) / float64(len(ws)) * ss)
	} else {
		s.se = math.NaN()
	}
	s.widen()
	return s
}

// spread is the reading the way it gates.
func (r seeded) spread() spread {
	if r.pooled {
		return r.jackknife()
	}
	return r.bySeed()
}

// eachOf is the worlds one to a seed.
func eachOf(gs func() []*Grid) func() [][]*Grid {
	return func() [][]*Grid {
		var out [][]*Grid
		for _, g := range gs() {
			out = append(out, []*Grid{g})
		}
		return out
	}
}

// pooled reads f over every seed's worlds together.
func pooled(f func([]*Grid) float64) func([][]*Grid) float64 {
	return func(ws [][]*Grid) float64 {
		var gs []*Grid
		for _, w := range ws {
			gs = append(gs, w...)
		}
		return f(gs)
	}
}

// overSmallGlobes is f read over spreadGlobes small globes.
func overSmallGlobes(f func([]*Grid) float64) *seeded {
	return &seeded{worlds: eachOf(func() []*Grid { return smallGlobes(spreadGlobes) }), read: pooled(f)}
}

// pooledOverSmallGlobes is f read over spreadGlobes small globes pooled, its
// interval by the jackknife.
func pooledOverSmallGlobes(f func([]*Grid) float64) *seeded {
	r := overSmallGlobes(f)
	r.pooled = true
	return r
}

// pairs is the double and the single globe of each resolution seed.
func pairs() [][]*Grid {
	d, s := doubleGlobes(), singleGlobes()
	var out [][]*Grid
	for k := range d {
		out = append(out, []*Grid{d[k], s[k]})
	}
	return out
}

// difference reads f over the doubles of the seeds together less f over
// their singles.
func difference(f func([]*Grid) float64) func([][]*Grid) float64 {
	return func(ws [][]*Grid) float64 {
		var d, s []*Grid
		for _, w := range ws {
			d, s = append(d, w[0]), append(s, w[1])
		}
		return f(d) - f(s)
	}
}

func flintTheta(gs []*Grid) float64 { th, _ := flint(gs); return th }
func flintR2(gs []*Grid) float64    { _, r2 := flint(gs); return r2 }
func areaExceedance(gs []*Grid) float64 {
	return basinExceedance(gs, func(g *Grid, i int) float64 { return g.area[i] })
}
func flowExceedance(gs []*Grid) float64 {
	return basinExceedance(gs, func(g *Grid, i int) float64 { return g.Flow[i] })
}
func intermittencyC1(gs []*Grid) float64 { c1, _ := reliefIntermittency(gs); return c1 }

// read is the yardstick's reading: over its seeds where it has them, and its
// one figure where it does not.
func (y yardstick) read() spread {
	if y.seeded != nil {
		return y.seeded.spread()
	}
	return pointSpread(y.measure())
}

// check holds a reading to its yardstick's band: it fails where the interval
// lies wholly outside it, logs the interval where there is one, and reads an
// advisory yardstick without holding it.
func (y yardstick) check(t *testing.T) {
	t.Helper()
	got := y.read()
	if y.seeded != nil {
		what := "seeds"
		if y.seeded.pooled {
			what = "each seed left out"
		}
		t.Logf("%s %s, real %.4g-%.4g; %s %s", got, y.unit, y.lo, y.hi, what, seedList(got.seeds))
	}
	switch {
	case y.advisory != "":
		t.Skipf("advisory, %s the band: %s (got %s %s, real %.4g-%.4g)", got.against(y.lo, y.hi), y.advisory, got, y.unit, y.lo, y.hi)
	case got.outside(y.lo, y.hi):
		t.Errorf("got %s %s, real %.4g-%.4g (%s)", got, y.unit, y.lo, y.hi, y.source)
	}
}

// against is where the interval lies against a band: in, out, or straddling it.
func (s spread) against(lo, hi float64) string {
	switch {
	case s.within(lo, hi):
		return "in"
	case s.outside(lo, hi):
		return "out"
	}
	return "straddles"
}

func seedList(v []float64) string {
	var b strings.Builder
	for k, x := range v {
		if k > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%.4g", x)
	}
	return b.String()
}

func TestTheSpreadOfAReading(t *testing.T) {
	near := func(what string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 1e-9*math.Max(1, math.Abs(want)) {
			t.Errorf("%s: got %.6g, want %.6g", what, got, want)
		}
	}
	s := spreadOf([]float64{1, 2, 3, 4, math.NaN()})
	near("median", s.median, 2.5)
	near("n", float64(s.n), 4)
	near("lo", s.lo, 1) // four seeds are 87.5% sure of their median, least to greatest
	near("hi", s.hi, 4)
	if !s.outside(10, 20) || s.outside(0, 2) || s.within(0, 2) || !s.within(-10, 10) {
		t.Errorf("an interval %.3g-%.3g is held wrongly against its bands", s.lo, s.hi)
	}
	// Conover's table: sixteen seeds hold their median between the fifth
	// least and the fifth greatest 92.3% of the time, and the fourth 97.9%;
	// eight between the second 93.0%; fifteen between the fourth 96.5%, and
	// the fifth only 88.2%.
	for _, c := range []struct{ n, k int }{{16, 5}, {8, 2}, {15, 4}, {3, 1}, {2, 1}, {30, 11}} {
		if k := medianRank(c.n, medianCover); k != c.k {
			t.Errorf("%d seeds: the interval is the %d-th from each end, want the %d-th", c.n, k, c.k)
		}
	}
	// One seed's wild reading moves the interval no further than any seed
	// above the median would.
	wild := spreadOf([]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 1e9})
	near("wild hi", wild.hi, 12)
	if one := spreadOf([]float64{0.5}); one.outside(0, 0.1) {
		t.Errorf("a reading of one seed fails a band: %s", one)
	}
	if none := spreadOf([]float64{math.NaN()}); !none.outside(0, 1) {
		t.Errorf("a reading of no seed passes a band")
	}
	if p := pointSpread(0.5); !p.outside(0, 0.4) || p.outside(0, 0.6) {
		t.Errorf("a point reading is not held as a point: %s", p)
	}

	// The jackknife of a mean is the standard error of the mean, s/sqrt(n).
	// A world stands in for each seed by its width, which mean reads.
	xs := []float64{3, 5, 6, 10}
	r := seeded{
		worlds: func() [][]*Grid {
			var ws [][]*Grid
			for _, x := range xs {
				ws = append(ws, []*Grid{{W: int(x)}})
			}
			return ws
		},
		read: func(ws [][]*Grid) float64 {
			sum := 0.0
			for _, w := range ws {
				sum += float64(w[0].W)
			}
			return sum / float64(len(ws))
		},
		pooled: true,
	}
	j := r.spread()
	near("jackknife reading", j.median, 6)
	near("jackknife se", j.se, sdOf(xs)/2)
	near("jackknife hi", j.hi, 6+3.182*sdOf(xs)/2)
	r.pooled = false
	near("median of seeds", r.spread().median, 5.5)
}

// The noise in the readings, for docs/yardsticks.md. Off unless
//
//	TERRA_YARDSTICK_SPREAD=<file.json> go test -run TestTheReadingsSpread -timeout 60m .
//
// which reads every yardstick that has seeds both ways - the median of the
// seeds and the pooled fit with its jackknife - and the reading it was before
// it had them, pooled over the seeds it read then, with that reading's
// jackknife; and writes them to the file and the log. It reads no world the
// yardsticks do not.
func TestTheReadingsSpread(t *testing.T) {
	path := os.Getenv("TERRA_YARDSTICK_SPREAD")
	if path == "" {
		t.Skip("set TERRA_YARDSTICK_SPREAD to a file to read the noise into")
	}
	clean := func(x float64) any {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil // JSON has no NaN or infinity
		}
		return x
	}
	cleanAll := func(v []float64) []any {
		out := make([]any, len(v))
		for k, x := range v {
			out[k] = clean(x)
		}
		return out
	}
	half := func(s spread) map[string]any {
		return map[string]any{"reading": clean(s.median), "se": clean(s.se), "lo": clean(s.lo), "hi": clean(s.hi), "n": s.n, "seeds": cleanAll(s.seeds)}
	}
	var out []map[string]any
	all := slices.Clone(yardsticks)
	for _, y := range realYardsticks {
		all = append(all, y.yardstick)
	}
	for _, y := range all {
		if y.seeded == nil {
			continue
		}
		gate := y.read()
		med, jack := y.seeded.bySeed(), y.seeded.jackknife()
		sd := sdOf(numbers(med.seeds))
		// Seeds that would hold the reading's standard error to a tenth of
		// the band: the median's, 1.2533 sd / sqrt(n) for seeds that scatter
		// as a bell does, or the pooled fit's jackknife, as 1 / sqrt(n).
		tenth := math.Ceil(math.Pow(1.2533*sd/(0.1*(y.hi-y.lo)), 2))
		if y.seeded.pooled {
			tenth = math.Ceil(float64(jack.n) * math.Pow(jack.se/(0.1*(y.hi-y.lo)), 2))
		}
		row := map[string]any{
			"name": y.name, "lo": y.lo, "hi": y.hi, "advisory": y.advisory != "", "pooled": y.seeded.pooled,
			"reading": half(gate), "verdict": gate.against(y.lo, y.hi),
			"median": half(med), "jackknife": half(jack), "sd": clean(sd), "seedsForATenth": clean(tenth),
		}
		was := "-"
		if p, ok := pooledReadings[y.name]; ok {
			b := p.jackknife()
			row["before"] = half(b)
			was = fmt.Sprintf("%.4g (%d) se %.3g", b.median, b.n, b.se)
		}
		t.Logf("%-50s gate %-48s | median %-48s | pooled %-48s | sd %.3g tenth %.0f | before %s",
			y.name, gate, med, jack, sd, tenth, was)
		out = append(out, row)
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// pooledReadings are the readings as they stood before they had seeds, by
// yardstick: the same fits, pooled over the seeds they read then.
var pooledReadings = map[string]seeded{
	"hypsometric integral, small globe":              {worlds: eachOf(func() []*Grid { return smallGlobes(3) }), read: pooled(meanHypsometry)},
	"drainage area exceedance exponent, small globe": {worlds: eachOf(func() []*Grid { return smallGlobes(exceedanceGlobes) }), read: pooled(areaExceedance)},
	"discharge exceedance exponent, small globe":     {worlds: eachOf(func() []*Grid { return smallGlobes(exceedanceGlobes) }), read: pooled(flowExceedance)},
	"Hack exponent, small globe":                     {worlds: eachOf(func() []*Grid { return smallGlobes(networkGlobes) }), read: pooled(hackExponent)},
	"Hack exponent, globe":                           {worlds: eachOf(globes), read: pooled(hackExponent)},
	"ridge-valley wavelength, small globe":           {worlds: eachOf(func() []*Grid { return smallGlobes(3) }), read: pooled(valleyWavelength)},
	"channel concavity, small globe":                 {worlds: eachOf(func() []*Grid { return smallGlobes(networkGlobes) }), read: pooled(flintTheta)},
	"Flint's law fit R2, small globe":                {worlds: eachOf(func() []*Grid { return smallGlobes(networkGlobes) }), read: pooled(flintR2)},
	"Hack exponent, 2x less 1x, small globe":         {worlds: firstPairs(4), read: difference(hackExponent)},
	"channel concavity, 2x less 1x, small globe":     {worlds: firstPairs(4), read: difference(flintTheta)},
	"hypsometric integral, 2x less 1x, small globe":  {worlds: firstPairs(4), read: difference(meanHypsometry)},
	"land relief intermittency C1, three globes":     {worlds: eachOf(threeGlobes), read: pooled(intermittencyC1)},
}

// firstPairs is the first n resolution seeds' pairs: as many as the
// resolution yardsticks read before they had seeds.
func firstPairs(n int) func() [][]*Grid {
	return func() [][]*Grid { return pairs()[:min(n, resolutionSeeds)] }
}
