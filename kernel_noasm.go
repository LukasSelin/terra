//go:build !(goexperiment.simd && amd64)

package terra

// The kernels on every build that is not asked for the vector arithmetic, or
// is not on a processor that has it: each is its statement in kernel.go and
// nothing else. See kernel_simd_amd64.go for the other case.

func fade(wear []float64, by float64)           { fadeScalar(wear, by) }
func grow(age []float64, ks []int64, k float64) { growScalar(age, ks, k) }
