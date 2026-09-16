//go:build !(goexperiment.simd && amd64)

package kernel

// The kernels on every build that is not asked for the vector arithmetic, or
// is not on a processor that has it: each is its statement in kernel.go and
// nothing else. See kernel_simd_amd64.go for the other case.

func Fade(wear []float64, by float64) { fadeScalar(wear, by) }

// Grow puts k on the age of every entry whose kind in ks ages: see
// growScalar. aging is the kinds ages says yes to, which the vectors read.
func Grow(age []float64, ks []int64, k float64, ages []bool, aging []int64) {
	growScalar(age, ks, k, ages)
}

type fftWide struct{}

func widen(*fftPlan) *fftWide { return nil }

func butterflies(x []complex128, pl *fftPlan, inverse bool) {
	butterfliesScalar(x, pl, inverse, 1, len(x))
}
func Axpy(y, x []float64, a float64)      { axpyScalar(y, x, a) }
func Lerp(dst, a, b []float64, t float64) { lerpScalar(dst, a, b, t) }
func Clamp(v []float64, lo, hi float64)   { clampScalar(v, lo, hi) }
func SumTree(v []float64) float64         { return sumTreeScalar(v) }
func Stencil5(dst, up, row, down []float64, c, s float64) {
	stencil5Scalar(dst, up, row, down, c, s)
}
func MinmaxSelect(v []float64) (lo, hi float64) { return minmaxSelectScalar(v) }
