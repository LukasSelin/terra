package terra

import (
	"fmt"
	"math"
	"testing"
)

// The same ground at twice the size.
//
// A figure written in tiles means something different on a map twice as many
// tiles across, and a figure fitted to the map's own largest river means
// something different on a map whose largest river is larger. A world written
// in metres and years should not care: a map of twice the ground, made from the
// same seed, is a different piece of country, but the laws of a piece of
// country - how a mainstream lengthens with its basin, how a channel eases as
// it gathers, how the ground stands between its floor and its peaks, how much
// rain falls on it - are the same laws. So each is read at one size and at
// twice, and asked to agree to within what real ground scatters by.

// resolutionSizes are the two maps each reading is taken on, and the seeds.
const resolutionSeeds = 3

type resolutionReading struct {
	hack, concavity, hypsometry, rain, rainSpread float64
}

func readResolution(gs []*Grid) resolutionReading {
	var r resolutionReading
	r.hack = hackExponent(gs)
	r.concavity = concavity(gs)
	r.hypsometry = meanHypsometry(gs)
	var sum, sq, n float64
	for _, g := range gs {
		for i := range g.Tiles {
			if g.underSea(i) {
				continue
			}
			sum, sq, n = sum+g.rain[i], sq+g.rain[i]*g.rain[i], n+1
		}
	}
	r.rain = sum / n
	r.rainSpread = math.Sqrt(math.Max(0, sq/n-r.rain*r.rain)) / r.rain
	return r
}

// concavity is θ in Flint's law, S ~ A^-θ: how a channel's fall eases with the
// ground it drains, fitted over bins of log area on the channels - every tile
// of the basins basinsOf finds that drains channelHead or more - so that the
// headwaters, which are most of them, do not decide it alone. The fall is read
// down the plain drainage treeOf takes, over the ground.
func concavity(gs []*Grid) float64 {
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
			d := tr.down[i]
			if !in[i] || d < 0 || tr.area[i] < channelHead {
				continue
			}
			run := TileSpan
			if int(d)%g.W != i%g.W && int(d)/g.W != i/g.W {
				run *= math.Sqrt2
			}
			s := (g.Tiles[i].Height - g.Tiles[d].Height) / run
			if s <= 0 {
				continue
			}
			b := int(math.Log(tr.area[i]) / width)
			sum[b] += math.Log(s)
			count[b]++
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
	return -fit(xs, ys)
}

// TestTheSameGroundAtTwiceTheSize reads the valley at its default size and at
// twice as many tiles each way, and a small globe at its size and twice, over
// the same seeds, and asks the two to agree.
//
// The tolerances are what real ground scatters by, and not what the map
// happens to: Hack's exponent is 0.57 +- 0.03 across real networks (Rigon et
// al. 1996), so two readings of one law may differ by 0.06; concavity runs
// 0.35 to 0.6 in rivers that are not being tilted or blocked (Whipple and Tucker
// 1999; Tucker and Whipple 2002), so by 0.15; the Strahler integral of mature
// ground runs 0.35 to 0.6 (Strahler 1952), so by 0.1; and a region's mean rain
// is a fact about its latitude and its ranges, not its size, so to a fifth.
func TestTheSameGroundAtTwiceTheSize(t *testing.T) {
	if testing.Short() {
		t.Skip("makes twelve worlds, two of them at four times a small globe")
	}
	const (
		hackTol  = 0.06
		concTol  = 0.15
		hypsoTol = 0.10
		rainTol  = 0.20
	)
	small := smallGlobe()
	big := small
	big.Width, big.Height = 2*small.Width, 2*small.Height
	valley := DefaultTerms()
	wide := valley
	wide.Width, wide.Height = 2*valley.Width, 2*valley.Height
	for _, c := range []struct {
		name     string
		one, two Terms
	}{{"valley", valley, wide}, {"small globe", small, big}} {
		t.Run(c.name, func(t *testing.T) {
			var ones, twos []*Grid
			for seed := uint64(1); seed <= resolutionSeeds; seed++ {
				ones = append(ones, NewLand(seed, c.one).Grid)
				twos = append(twos, NewLand(seed, c.two).Grid)
			}
			a, b := readResolution(ones), readResolution(twos)
			if testing.Verbose() {
				fmt.Printf("%s 1x: %+v\n%s 2x: %+v\n", c.name, a, c.name, b)
			}
			check := func(what string, x, y, tol float64) {
				if math.IsNaN(x) || math.IsNaN(y) || math.Abs(x-y) > tol {
					t.Errorf("%s: %.3f at one size and %.3f at twice; want within %.3f", what, x, y, tol)
				}
			}
			check("Hack exponent", a.hack, b.hack, hackTol)
			check("concavity", a.concavity, b.concavity, concTol)
			check("hypsometric integral", a.hypsometry, b.hypsometry, hypsoTol)
			check("mean rain on land, mm a year", a.rain, b.rain, rainTol*a.rain)
			check("spread of rain on land, as a share of its mean", a.rainSpread, b.rainSpread, rainTol)
		})
	}
}
