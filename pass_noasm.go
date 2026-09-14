//go:build !(goexperiment.simd && amd64)

package terra

// The day's pass one tile at a time: every build that is not asked for
// the vector arithmetic, or is not on a processor that has it. See
// pass.go, and pass_simd_amd64.go for the other case.

func fade(wear []float64, by float64)           { fadeScalar(wear, by) }
func grow(age []float64, ks []int64, k float64) { growScalar(age, ks, k) }
func fill(s, age []float64, ks []int64, kind int64, _ bool, full, rate, k float64) {
	fillScalar(s, age, ks, kind, full, rate, k)
}
func shoal(fish []float64, ks []int64, fall float64)      { shoalScalar(fish, ks, fall) }
func rest(fert, rich []float64, ks []int64, fall float64) { restScalar(fert, rich, ks, fall) }
func meadow(sward []float64, ks []int64, fall float64)    { meadowScalar(sward, ks, fall) }
