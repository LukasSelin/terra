package terra

import "math"

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
// The one thing a lane is not held to is which NaN it is: a sum or a product
// of two NaNs keeps the first operand's bits on this processor, and the
// compiler is free to put either operand of a sum or a product first, so
// the bits of a NaN are not a fact about the statement. That it is a NaN
// is, and the tests hold that.

// lanes is how many numbers one vector holds, and whole the length of the
// part of a run of n that is done in whole vectors, the rest being the
// tail. They are facts about the statements too: a sum over a run is a
// tree by lanes on both paths.
const lanes = 4

func whole(n int) int { return n &^ (lanes - 1) }

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

// axpyScalar adds a times x onto y, entry by entry: y += a*x. The product is
// rounded before it is added, on both paths.
func axpyScalar(y, x []float64, a float64) {
	x = x[:len(y)]
	for i := range y {
		y[i] = float64(y[i] + float64(a*x[i]))
	}
}

// lerpScalar blends a toward b by t, entry by entry: dst = a + (b-a)*t, each
// operation rounded, on both paths. dst may be a or b.
func lerpScalar(dst, a, b []float64, t float64) {
	a, b = a[:len(dst)], b[:len(dst)]
	for i := range dst {
		dst[i] = float64(a[i] + float64(float64(b[i]-a[i])*t))
	}
}

// clampScalar holds every entry of v to [lo, hi]: what is under lo is lo,
// what is over hi is hi. It is a comparison and a choice, twice, on both
// paths - not a minimum and a maximum, whose answer on a NaN or on a nought
// of either sign is the instruction's rather than the number's. So a NaN
// stays a NaN, and a negative nought held to [0, 1] stays a negative nought,
// which is not less than nought.
func clampScalar(v []float64, lo, hi float64) {
	for i, x := range v {
		if x < lo {
			x = lo
		}
		if x > hi {
			x = hi
		}
		v[i] = x
	}
}

// sumTreeScalar is the sum of v in a fixed order, the same on both paths: the
// entries are dealt round the lanes and each lane summed on its own, the
// lane sums are added as a tree - the first two, the last two, then those -
// and the tail, what is left over after the whole vectors, is added on last
// one by one. A sum is only the same number if it is added in the same
// order, and this is the order a vector adds in, so the statement adds in
// it too. It is not the order a plain loop adds in: a sum a pass takes with
// a loop today comes to different bits summed this way, so it is for sums
// that are new or that mean to move.
func sumTreeScalar(v []float64) float64 {
	var s [lanes]float64
	n := whole(len(v))
	for i := 0; i < n; i += lanes {
		s[0] = float64(s[0] + v[i])
		s[1] = float64(s[1] + v[i+1])
		s[2] = float64(s[2] + v[i+2])
		s[3] = float64(s[3] + v[i+3])
	}
	return sumTail(s, v[n:])
}

// sumTail is the tree over the lane sums and the tail after it.
func sumTail(s [lanes]float64, tail []float64) float64 {
	total := float64(float64(s[0]+s[1]) + float64(s[2]+s[3]))
	for _, x := range tail {
		total = float64(total + x)
	}
	return total
}

// stencil5Scalar is a five-point stencil over a row, reading old values into
// a new slice: for each entry, c times the entry itself and s times the sum
// of its four neighbours, west, east, north and south. row carries a halo -
// it is two longer than dst, the entry west of the first and east of the
// last - so that a row of a map can be done without asking about its ends,
// and up and down are the rows above and below, as long as dst. The
// neighbours are summed as (west + east) + (up + down), then scaled, then
// added to the scaled centre, each operation rounded, on both paths. It
// reads old values only, so a map of rows can be done in any order and on
// any goroutine; it is not a sweep.
func stencil5Scalar(dst, up, row, down []float64, c, s float64) {
	up, down, row = up[:len(dst)], down[:len(dst)], row[:len(dst)+2]
	for i := range dst {
		h := float64(row[i] + row[i+2])
		v := float64(up[i] + down[i])
		dst[i] = float64(float64(c*row[i+1]) + float64(s*float64(h+v)))
	}
}

// minmaxSelectScalar is the least and the greatest of v, by comparison and
// choice: an entry is taken as the least only if it is less than what is
// held, so a NaN is never taken and, among entries that are equal - a
// negative nought and a positive - the first seen stays. Which is first is
// a fact about the order, so the order is fixed and the same on both
// paths: the entries are dealt round the lanes and each lane keeps its
// own, the lanes are then read in order, and the tail last, one by one.
// The least starts at +Inf and the greatest at -Inf, so a run of nothing
// comes back as (+Inf, -Inf) and a run of NaNs the same.
func minmaxSelectScalar(v []float64) (lo, hi float64) {
	var los, his [lanes]float64
	for l := range los {
		los[l], his[l] = math.Inf(1), math.Inf(-1)
	}
	n := whole(len(v))
	for i := 0; i < n; i += lanes {
		for l := range los {
			x := v[i+l]
			if x < los[l] {
				los[l] = x
			}
			if x > his[l] {
				his[l] = x
			}
		}
	}
	return minmaxTail(los, his, v[n:])
}

// minmaxTail reads the lanes in order and then the tail.
func minmaxTail(los, his [lanes]float64, tail []float64) (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for l := range los {
		if los[l] < lo {
			lo = los[l]
		}
		if his[l] > hi {
			hi = his[l]
		}
	}
	for _, x := range tail {
		if x < lo {
			lo = x
		}
		if x > hi {
			hi = x
		}
	}
	return lo, hi
}
