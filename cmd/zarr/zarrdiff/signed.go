package main

import "math"

// Signed is how an array's measured elements changed, as b-a: how many
// went up and down, the mean, the least and most, and the 5th, 50th and
// 95th percentiles. The percentiles are read off a histogram (see tally)
// and are within about 1.1% of the true ones, clamped to Min and Max.
type Signed struct {
	Up, Down     int64
	Mean         float64
	P5, P50, P95 float64
	Min, Max     float64
}

// The histogram of signed changes has a fixed set of bins, so a tally
// holds the same whatever it counts: binsPerOctave bins to each doubling of
// the magnitude, from 2^-octaves to 2^octaves, on either side of zero. A
// magnitude outside that range is counted in the bin at its end. That is
// 8192 counts, 64 KiB, for a tally that keeps one.
const (
	binsPerOctave = 32
	octaves       = 64
	binsPerSign   = 2 * octaves * binsPerOctave
)

// tally is what is counted of the changes to an array, or to the tiles of
// one category of it.
type tally struct {
	tilesChanged, changed int64 // tiles and elements changed
	up, down, n           int64 // measured changes: above, below zero, all
	sum, lo, hi           float64
	// keep is whether add bins each change for the percentiles; hist is
	// nil until the first.
	keep bool
	hist []int64
}

// add counts a measured change v, b-a.
func (t *tally) add(v float64) {
	switch {
	case v > 0:
		t.up++
	case v < 0:
		t.down++
	}
	if t.n == 0 {
		t.lo, t.hi = v, v
	}
	t.n++
	t.sum += v
	t.lo, t.hi = min(t.lo, v), max(t.hi, v)
	if t.keep {
		t.bin(v)
	}
}

// bin counts v in the histogram alone.
func (t *tally) bin(v float64) {
	if t.hist == nil {
		t.hist = make([]int64, 2*binsPerSign)
	}
	t.hist[binOf(v)]++
}

// binOf is v's bin: the bins of negative changes first, the largest
// magnitude first, then the positive ones, so that the bins run in order
// of value.
func binOf(v float64) int {
	m := math.Abs(v)
	e := 0 // zero goes with the smallest positive magnitudes
	if m > 0 {
		e = int(math.Floor(math.Log2(m)*binsPerOctave)) + binsPerSign/2
	}
	e = min(max(e, 0), binsPerSign-1)
	if v < 0 {
		return binsPerSign - 1 - e
	}
	return binsPerSign + e
}

// valueOf is the middle of a bin, geometrically.
func valueOf(bin int) float64 {
	e, sign := bin-binsPerSign, 1.0
	if bin < binsPerSign {
		e, sign = binsPerSign-1-bin, -1
	}
	return sign * math.Pow(2, (float64(e-binsPerSign/2)+0.5)/binsPerOctave)
}

// percentile is the nearest-rank p-th percentile, p in (0, 1], of the
// binned changes, clamped to the least and most.
func (t *tally) percentile(p float64) float64 {
	if t.hist == nil || t.n == 0 {
		return 0
	}
	rank := min(max(int64(math.Ceil(p*float64(t.n))), 1), t.n)
	seen := int64(0)
	for b, c := range t.hist {
		if seen += c; seen >= rank {
			return min(max(valueOf(b), t.lo), t.hi)
		}
	}
	return t.hi
}

// signed is the tally's Signed, nil where nothing was measured.
func (t *tally) signed() *Signed {
	if t.n == 0 {
		return nil
	}
	return &Signed{
		Up: t.up, Down: t.down,
		Mean: t.sum / float64(t.n),
		P5:   t.percentile(0.05), P50: t.percentile(0.5), P95: t.percentile(0.95),
		Min: t.lo, Max: t.hi,
	}
}
