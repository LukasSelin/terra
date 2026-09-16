package terra

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// Every kernel is held to its statement: the kernel a build does - four
// lanes at a time under GOEXPERIMENT=simd on a processor with AVX2, the
// statement itself otherwise - comes out bit for bit what the statement in
// kernel.go does, over runs of every length up to 4096, so that every
// alignment of the tail is tried, and over numbers that a lane and a number
// could part company on: negative noughts, NaNs, infinities and values far
// apart in size. Each is a fuzz test so that go test -fuzz can go on looking
// past the seeds; under go test alone the seeds run, and each seed draws a
// few hundred runs.

// draw is a number a kernel could be given: mostly ordinary, and every so
// often one of the special ones.
func draw(rng *rand.Rand) float64 {
	switch rng.IntN(12) {
	case 0:
		return math.Copysign(0, -1)
	case 1:
		return 0
	case 2:
		return math.NaN()
	case 3:
		return math.Inf(1 - 2*rng.IntN(2))
	case 4:
		return (rng.Float64() - 0.5) * 1e300
	case 5:
		return (rng.Float64() - 0.5) * 1e-300
	case 6:
		return (rng.Float64() - 0.5) * 1e12
	case 7:
		return (rng.Float64() - 0.5) * 1e-12
	}
	return rng.Float64()*2 - 1
}

// run is n numbers drawn.
func run(rng *rand.Rand, n int) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = draw(rng)
	}
	return s
}

// sameBits fails the test where got and want differ in any bit, except that a
// NaN is any NaN: which NaN's bits a sum of two keeps is the compiler's
// choice, see kernel.go.
func sameBits(t *testing.T, what string, n int, got, want []float64) {
	t.Helper()
	for i := range want {
		if !sameOrNaN(got[i], want[i]) {
			t.Fatalf("%s, length %d: entry %d is %v (%#x) in lanes, and %v (%#x) one at a time",
				what, n, i, got[i], math.Float64bits(got[i]), want[i], math.Float64bits(want[i]))
		}
	}
}

// lengths is the run lengths one seed tries: every length under twice the
// lanes, so that every tail is tried, and then random ones up to 4096.
func lengths(rng *rand.Rand) []int {
	ls := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 15, 16, 17}
	for range 40 {
		ls = append(ls, rng.IntN(4097))
	}
	return ls
}

// seeds is what go test runs of each fuzz test without -fuzz.
func seeds(f *testing.F) {
	for _, s := range []uint64{1, 2, 3, 5, 8, 13, 21, 34} {
		f.Add(s)
	}
}

func FuzzFade(f *testing.F) {
	seeds(f)
	f.Fuzz(func(t *testing.T, seed uint64) {
		rng := rand.New(rand.NewPCG(seed, 1))
		for _, n := range lengths(rng) {
			by := draw(rng)
			want := run(rng, n)
			got := slices.Clone(want)
			fade(got, by)
			fadeScalar(want, by)
			sameBits(t, "fade", n, got, want)
		}
	})
}

func FuzzGrow(f *testing.F) {
	seeds(f)
	f.Fuzz(func(t *testing.T, seed uint64) {
		rng := rand.New(rand.NewPCG(seed, 2))
		// Kinds that age and kinds that do not, as the growth table has them.
		var kinds []int64
		for k := range int64(kindSpan) {
			kinds = append(kinds, k)
		}
		for _, n := range lengths(rng) {
			k := draw(rng)
			want := run(rng, n)
			ks := make([]int64, n)
			for i := range ks {
				ks[i] = kinds[rng.IntN(len(kinds))]
			}
			got := slices.Clone(want)
			grow(got, ks, k)
			growScalar(want, ks, k)
			sameBits(t, "grow", n, got, want)
		}
	})
}

// How long each kernel takes over one chunk's width, for holding the
// arithmetic four lanes at a time against the statement: run it with and
// without GOEXPERIMENT=simd, or through scripts/perf.sh simd.
func BenchmarkKernel(b *testing.B) {
	const n = 1024
	rng := rand.New(rand.NewPCG(7, 7))
	x := run(rng, n)
	ks := make([]int64, n)
	for i := range ks {
		ks[i] = int64(rng.IntN(kindSpan))
	}
	b.Run("fade", func(b *testing.B) {
		y := slices.Clone(x)
		for b.Loop() {
			fade(y, Fade)
		}
	})
	b.Run("grow", func(b *testing.B) {
		y := slices.Clone(x)
		for b.Loop() {
			grow(y, ks, 0.37)
		}
	})
	b.Run("axpy", func(b *testing.B) {
		y := slices.Clone(x)
		for b.Loop() {
			axpy(y, x, 0.5)
		}
	})
}

// The butterflies, at every length a map asks for, forward and back; the
// plan's roots are the same on both paths, so a run of complex numbers is a
// run of float64s drawn like any other. A NaN is held to be a NaN and not to
// its bits: the complex product is a sum of two products, and where both are
// NaN the bits kept are the first operand's, which the compiler is free to
// choose for a sum. See the vector butterflies.
func FuzzButterflies(f *testing.F) {
	seeds(f)
	f.Fuzz(func(t *testing.T, seed uint64) {
		rng := rand.New(rand.NewPCG(seed, 3))
		for n := 1; n <= 4096; n <<= 1 {
			for _, inverse := range []bool{false, true} {
				want := make([]complex128, n)
				for i := range want {
					want[i] = complex(draw(rng), draw(rng))
				}
				got := slices.Clone(want)
				pl := planFor(n)
				butterflies(got, pl, inverse)
				butterfliesScalar(want, pl, inverse, 1, n)
				for i := range want {
					if !sameOrNaN(real(got[i]), real(want[i])) || !sameOrNaN(imag(got[i]), imag(want[i])) {
						t.Fatalf("butterflies, length %d, inverse %v: element %d is %v in lanes, and %v one at a time", n, inverse, i, got[i], want[i])
					}
				}
			}
		}
	})
}

// sameOrNaN is whether a and b are the same bits, or both NaN.
func sameOrNaN(a, b float64) bool {
	return math.Float64bits(a) == math.Float64bits(b) || (math.IsNaN(a) && math.IsNaN(b))
}

func FuzzAxpy(f *testing.F) {
	seeds(f)
	f.Fuzz(func(t *testing.T, seed uint64) {
		rng := rand.New(rand.NewPCG(seed, 4))
		for _, n := range lengths(rng) {
			a := draw(rng)
			x := run(rng, n+rng.IntN(3)) // x may be longer than y
			want := run(rng, n)
			got := slices.Clone(want)
			axpy(got, x, a)
			axpyScalar(want, x, a)
			sameBits(t, "axpy", n, got, want)
		}
	})
}
