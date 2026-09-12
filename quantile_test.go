package terra

import (
	"math"
	"math/rand/v2"
	"slices"
	"sort"
	"testing"
)

// A quantile found by partitioning has to be the number a sorted copy has at
// that place, on every kind of list a map hands it: heights that are all
// different, slopes that are mostly zero, ground already in order, and the
// one-value lists a very small map produces.
func TestAQuantileIsWhatTheSortedListHasThere(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	kinds := map[string]func(n int) []float64{
		"spread": func(n int) []float64 {
			v := make([]float64, n)
			for i := range v {
				v[i] = r.Float64()
			}
			return v
		},
		"mostly the same": func(n int) []float64 {
			v := make([]float64, n)
			for i := range v {
				if r.IntN(10) == 0 {
					v[i] = r.Float64()
				}
			}
			return v
		},
		"all the same": func(n int) []float64 {
			return make([]float64, n)
		},
		"in order": func(n int) []float64 {
			v := make([]float64, n)
			for i := range v {
				v[i] = float64(i)
			}
			return v
		},
		"backwards": func(n int) []float64 {
			v := make([]float64, n)
			for i := range v {
				v[i] = float64(n - i)
			}
			return v
		},
	}
	for name, make := range kinds {
		for _, n := range []int{1, 2, 3, 17, 1000} {
			v := make(n)
			want := slices.Clone(v)
			sort.Float64s(want)
			for _, f := range []float64{0, 0.1, 0.5, 0.9, 1} {
				i := int(f * float64(n-1))
				if got := quantile(v, f); got != want[i] {
					t.Fatalf("%s, %d long, at %.1f: got %v, the sorted list has %v", name, n, f, got, want[i])
				}
			}
		}
	}
}

// Several readings off one list are the same readings taken one at a time,
// though the list has been shuffled about by the first of them.
func TestSeveralQuantilesOfOneListAgreeWithOne(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	v := make([]float64, 5000)
	for i := range v {
		v[i] = math.Floor(r.Float64() * 20) // plenty of ties
	}
	fs := []float64{0.1, 0.5, 0.9, 0.95}
	got := quantiles(v, fs...)
	for k, f := range fs {
		if want := quantile(v, f); got[k] != want {
			t.Fatalf("at %.2f: several gave %v, one gave %v", f, got[k], want)
		}
	}
}

// A list with a NaN in it has no order to partition by, and the reading
// falls back on the sort, which puts them first as it always did.
func TestAQuantileOfAListWithNoOrderStillSorts(t *testing.T) {
	v := []float64{3, math.NaN(), 1, 2}
	want := slices.Clone(v)
	sort.Float64s(want)
	for _, f := range []float64{0, 0.34, 0.67, 1} {
		i := int(f * float64(len(v)-1))
		got, wanted := quantile(v, f), want[i]
		if got != wanted && !(math.IsNaN(got) && math.IsNaN(wanted)) {
			t.Fatalf("at %.2f: got %v, the sorted list has %v", f, got, wanted)
		}
	}
}
