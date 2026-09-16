package terra

// The kernels: the arithmetic a pass does to a run of numbers at once, each
// written twice. Here is the statement of what it computes, one number at a
// time, and kernel_simd_amd64.go is the same arithmetic four lanes at a time
// on a processor with AVX2, built only under GOEXPERIMENT=simd; kernel_noasm.go
// is every other build, where the kernel is its statement and nothing else.
// The tests in kernel_test.go hold every lane of the vector to the statement
// bit for bit, over runs of every length and alignment, with negative
// noughts, NaNs and numbers of very different size in them.
//
// What a kernel may be is set by what a vector can be held to. Adding,
// subtracting, multiplying and dividing a lane is the same rounding as the
// same done to a number, so those are written freely. A minimum is not: the
// vector minimum's answer when the two sides are equal - a negative nought
// against a positive, or a NaN against anything - is a fact about the
// instruction, so every minimum and clamp here is a comparison and a choice,
// on both paths. A sum over a run is a fixed tree of additions, the same tree
// on both paths, because a sum is only the same number if it is added in the
// same order. And nothing is fused: a multiply and the add after it are two
// roundings on both paths, which the statement says outright with a
// conversion round each product, since Go allows a compiler to fuse them
// otherwise (and the amd64 compiler does, above the default GOAMD64).
//
// A statement is what a kernel computes, and it is the tail of every run on
// the vector path: a run is done in whole vectors and what is left over is
// handed to the statement, so the two never disagree about the last few.

// fadeScalar multiplies every entry of wear by by. It is FadeWear over a
// run: the ground forgetting its marking.
func fadeScalar(wear []float64, by float64) {
	for i := range wear {
		wear[i] *= by
	}
}

// growScalar puts k of weather on the age of every tile that has something
// growing on it, which is every tile whose kind is one that ages.
func growScalar(age []float64, ks []int64, k float64) {
	for j, kk := range ks {
		if ages[kk] {
			age[j] += k
		}
	}
}

// butterfliesScalar is every level of the transform's butterflies, from the
// pairs up, one element at a time: the statement the vector butterflies are
// held to. Levels whose half is under from are done; the rest are left. See
// fft.go for the transform and the plan.
func butterfliesScalar(x []complex128, pl *fftPlan, inverse bool, from, to int) {
	n := len(x)
	roots := pl.roots
	if inverse {
		roots = pl.back
	}
	for size := 2; size <= n; size <<= 1 {
		half := size / 2
		if half < from || half >= to {
			continue
		}
		stride := n / size
		for start := 0; start < n; start += size {
			for k := 0; k < half; k++ {
				a, b := x[start+k], x[start+k+half]*roots[k*stride]
				x[start+k], x[start+k+half] = a+b, a-b
			}
		}
	}
}
