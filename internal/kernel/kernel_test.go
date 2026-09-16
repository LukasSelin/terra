package kernel

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
			Fade(got, by)
			fadeScalar(want, by)
			sameBits(t, "fade", n, got, want)
		}
	})
}

// kindSpan is how many kinds the grow tests draw from: more than a map has.
const kindSpan = 256

// growing is a table of the kinds that age, as a growth table would give one:
// some kinds of kindSpan, drawn at random, and the same kinds listed. It is
// sometimes none and sometimes more than the vectors are written for, which
// are the two cases the vectors hand to the statement.
func growing(rng *rand.Rand) (ages []bool, aging []int64) {
	ages = make([]bool, kindSpan)
	for range rng.IntN(11) {
		k := rng.IntN(kindSpan)
		if !ages[k] {
			ages[k] = true
			aging = append(aging, int64(k))
		}
	}
	return ages, aging
}

func FuzzGrow(f *testing.F) {
	seeds(f)
	f.Fuzz(func(t *testing.T, seed uint64) {
		rng := rand.New(rand.NewPCG(seed, 2))
		for _, n := range lengths(rng) {
			ages, aging := growing(rng)
			k := draw(rng)
			want := run(rng, n)
			ks := make([]int64, n)
			for i := range ks {
				// Mostly kinds that age, so that the mask is seen to choose.
				if len(aging) > 0 && rng.IntN(2) == 0 {
					ks[i] = aging[rng.IntN(len(aging))]
				} else {
					ks[i] = int64(rng.IntN(kindSpan))
				}
			}
			got := slices.Clone(want)
			Grow(got, ks, k, ages, aging)
			growScalar(want, ks, k, ages)
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
	// The three kinds a settlement's map grows on, or near enough.
	ages := make([]bool, kindSpan)
	aging := []int64{1, 2, 3}
	for _, k := range aging {
		ages[k] = true
	}
	b.Run("fade", func(b *testing.B) {
		y := slices.Clone(x)
		for b.Loop() {
			Fade(y, 0.999)
		}
	})
	b.Run("grow", func(b *testing.B) {
		y := slices.Clone(x)
		for b.Loop() {
			Grow(y, ks, 0.37, ages, aging)
		}
	})
	b.Run("axpy", func(b *testing.B) {
		y := slices.Clone(x)
		for b.Loop() {
			Axpy(y, x, 0.5)
		}
	})
	b.Run("lerp", func(b *testing.B) {
		y := slices.Clone(x)
		for b.Loop() {
			Lerp(y, y, x, 0.25)
		}
	})
	b.Run("clamp", func(b *testing.B) {
		y := slices.Clone(x)
		for b.Loop() {
			Clamp(y, -0.5, 0.5)
		}
	})
	b.Run("sumTree", func(b *testing.B) {
		var total float64
		for b.Loop() {
			total += SumTree(x)
		}
		_ = total
	})
	b.Run("stencil5", func(b *testing.B) {
		y := make([]float64, n-2)
		for b.Loop() {
			Stencil5(y, x[:n-2], x, x[2:], 0.5, 0.125)
		}
	})
	b.Run("minmaxSelect", func(b *testing.B) {
		var lo, hi float64
		for b.Loop() {
			lo, hi = MinmaxSelect(x)
		}
		_, _ = lo, hi
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
			Axpy(got, x, a)
			axpyScalar(want, x, a)
			sameBits(t, "axpy", n, got, want)
		}
	})
}

func FuzzLerp(f *testing.F) {
	seeds(f)
	f.Fuzz(func(t *testing.T, seed uint64) {
		rng := rand.New(rand.NewPCG(seed, 5))
		for _, n := range lengths(rng) {
			tt := draw(rng)
			a, b := run(rng, n+rng.IntN(3)), run(rng, n+rng.IntN(3))
			got, want := make([]float64, n), make([]float64, n)
			Lerp(got, a, b, tt)
			lerpScalar(want, a, b, tt)
			sameBits(t, "lerp", n, got, want)
			// And in place, over a itself.
			got, want = slices.Clone(a[:n]), slices.Clone(a[:n])
			Lerp(got, got, b, tt)
			lerpScalar(want, want, b, tt)
			sameBits(t, "lerp in place", n, got, want)
		}
	})
}

func FuzzClamp(f *testing.F) {
	seeds(f)
	f.Fuzz(func(t *testing.T, seed uint64) {
		rng := rand.New(rand.NewPCG(seed, 6))
		for _, n := range lengths(rng) {
			lo, hi := draw(rng), draw(rng)
			if rng.IntN(2) == 0 {
				lo, hi = 0, 1 // the bounds a share is held to, often
			}
			want := run(rng, n)
			got := slices.Clone(want)
			Clamp(got, lo, hi)
			clampScalar(want, lo, hi)
			sameBits(t, "clamp", n, got, want)
		}
	})
}

func FuzzSumTree(f *testing.F) {
	seeds(f)
	f.Fuzz(func(t *testing.T, seed uint64) {
		rng := rand.New(rand.NewPCG(seed, 7))
		for _, n := range lengths(rng) {
			v := run(rng, n)
			got, want := SumTree(v), sumTreeScalar(v)
			sameBits(t, "sumTree", n, []float64{got}, []float64{want})
		}
	})
}

func FuzzStencil5(f *testing.F) {
	seeds(f)
	f.Fuzz(func(t *testing.T, seed uint64) {
		rng := rand.New(rand.NewPCG(seed, 8))
		for _, n := range lengths(rng) {
			c, s := draw(rng), draw(rng)
			up, row, down := run(rng, n+rng.IntN(3)), run(rng, n+2+rng.IntN(3)), run(rng, n+rng.IntN(3))
			got, want := make([]float64, n), make([]float64, n)
			Stencil5(got, up, row, down, c, s)
			stencil5Scalar(want, up, row, down, c, s)
			sameBits(t, "stencil5", n, got, want)
		}
	})
}

func FuzzMinmaxSelect(f *testing.F) {
	seeds(f)
	f.Fuzz(func(t *testing.T, seed uint64) {
		rng := rand.New(rand.NewPCG(seed, 9))
		for _, n := range lengths(rng) {
			v := run(rng, n)
			if rng.IntN(2) == 0 {
				// Mostly noughts of both signs, so that ties are the rule.
				for i := range v {
					if rng.IntN(4) > 0 {
						v[i] = math.Copysign(0, float64(rng.IntN(2)*2-1))
					}
				}
			}
			lo, hi := MinmaxSelect(v)
			wlo, whi := minmaxSelectScalar(v)
			sameBits(t, "minmaxSelect", n, []float64{lo, hi}, []float64{wlo, whi})
		}
	})
}
