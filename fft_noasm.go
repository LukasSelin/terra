//go:build !(goexperiment.simd && amd64)

package terra

// The transform's butterflies one element at a time: every build that is not
// asked for the vector arithmetic. See fft_simd_amd64.go for the other case.

type fftWide struct{}

func widen(*fftPlan) *fftWide { return nil }

func butterflies(x []complex128, pl *fftPlan, inverse bool) {
	butterfliesScalar(x, pl, inverse, 1, len(x))
}
