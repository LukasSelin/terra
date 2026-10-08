package atmos

import (
	"math"
	"math/rand/v2"
	"testing"
)

// even is a year with total mm spread evenly over its phases.
func even(total float64) [Phases]float64 {
	var y [Phases]float64
	for k := range y {
		y[k] = total / Phases
	}
	return y
}

// seasonal is a year of total mm whose phase k has 1 + swing·cos of its
// place in the year from the north's summer, phase 2.
func seasonal(total, swing float64) [Phases]float64 {
	var y [Phases]float64
	for k, at := range [Phases]float64{-1, 0, 1, 0} { // as phaseSin, from summer
		y[k] = total / Phases * (1 + swing*at)
	}
	return y
}

// An unseasonal bucket sheds what Fu's curve at ω = 2.6 leaves of the rain,
// to within three parts in a hundred of the rain, whatever its size.
func TestBucketUnseasonalIsFu(t *testing.T) {
	const p = 1000.0
	worst := 0.0
	for _, hold := range []float64{20, 150, 400} {
		for phi := 0.1; phi <= 5; phi *= 1.2 {
			rain, pet := even(p), even(phi*p)
			b := Bucket(hold, &rain, &pet)
			fu := p - Fu(p, phi*p)
			d := math.Abs(b.Shed()-fu) / p
			worst = math.Max(worst, d)
			if d > 0.03 {
				t.Errorf("hold %.0f, φ %.2f: shed %.1f mm, Fu leaves %.1f", hold, phi, b.Shed(), fu)
			}
		}
	}
	t.Logf("unseasonal bucket against Fu at ω=2.6: worst %.4f of the rain", worst)
}

// The year read is the steady year, and what goes in comes out, to within a
// part in a thousand of the rain. Fortnightly steps are the year at fine
// steps to within three parts in a hundred of the rain at worst - a bucket of
// a few centimetres under a phase's rain of tens of them - and half a part
// in a hundred on the mean.
func TestBucketSteadyAndStepped(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	worstSteady, worstStep, worstBalance, meanStep := 0.0, 0.0, 0.0, 0.0
	n := 0
	for range 2000 {
		var rain, pet [Phases]float64
		var p float64
		for k := range Phases {
			rain[k] = 400 * r.Float64() * r.Float64()
			pet[k] = 500 * r.Float64()
			p += rain[k]
		}
		if p < 10 {
			continue
		}
		hold := 10 + 400*r.Float64()
		b := Bucket(hold, &rain, &pet)
		steady := bucketRun(hold, &rain, &pet, 30, bucketSteps, nil)
		fine := bucketRun(hold, &rain, &pet, 30, 200, nil)
		worstSteady = math.Max(worstSteady, math.Abs(b.Shed()-steady.Shed())/p)
		step := math.Abs(steady.Shed()-fine.Shed()) / p
		worstStep = math.Max(worstStep, step)
		meanStep += step
		n++
		worstBalance = math.Max(worstBalance, math.Abs(p-b.Shed()-b.Evaporated())/p)
	}
	meanStep /= float64(n)
	t.Logf("worst against 30 years: %.5f; against 200 steps a phase: %.5f worst, %.5f mean; unbalanced %.5f",
		worstSteady, worstStep, meanStep, worstBalance)
	if worstSteady > 0.001 || worstStep > 0.03 || meanStep > 0.005 || worstBalance > 0.001 {
		t.Errorf("worst: %.5f against the steady year, %.5f against fine steps (mean %.5f), %.5f unbalanced",
			worstSteady, worstStep, meanStep, worstBalance)
	}
}

// The air never takes more than fell or more than it could take: the year
// stays inside Budyko's limits, and the bucket never holds more than it
// holds.
func TestBucketInsideBudykosLimits(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	for range 5000 {
		var rain, pet [Phases]float64
		var p, e float64
		for k := range Phases {
			rain[k] = 1000 * r.Float64() * r.Float64()
			pet[k] = 800 * r.Float64()
			p += rain[k]
			e += pet[k]
		}
		hold := 400 * r.Float64()
		b := Bucket(hold, &rain, &pet)
		if ev := b.Evaporated(); ev < 0 || ev > math.Min(p, e)*(1+1e-3)+1e-9 {
			t.Fatalf("rain %v, pet %v, hold %.1f: the air took %.3f of %.3f rain and %.3f it could", rain, pet, hold, ev, p, e)
		}
		for k := range Phases {
			if b.Water[k] < 0 || b.Water[k] > hold*(1+1e-12) || b.Runoff[k] < 0 || b.Evap[k] > pet[k]*(1+1e-12) {
				t.Fatalf("rain %v, pet %v, hold %.1f: phase %d %+v", rain, pet, hold, k, b)
			}
		}
	}
}

// What the seasons do: rain that comes with the heat gives the air more than
// the same rain spread evenly, and rain that comes in the cold gives it
// less; and a deeper bucket carries more of a wet season over into a dry
// one, so it gives the air more of a seasonal year's rain.
func TestBucketSeasons(t *testing.T) {
	const p = 800.0
	pet := seasonal(900, 0.9)
	for _, hold := range []float64{50, 150, 300} {
		flat, with, against := even(p), seasonal(p, 0.9), seasonal(p, -0.9)
		bf, bw, ba := Bucket(hold, &flat, &pet), Bucket(hold, &with, &pet), Bucket(hold, &against, &pet)
		t.Logf("hold %3.0f mm: runoff %.0f even, %.0f rain with the heat, %.0f rain in the cold", hold, bf.Shed(), bw.Shed(), ba.Shed())
		if !(bw.Shed() < bf.Shed() && bf.Shed() < ba.Shed()) {
			t.Errorf("hold %.0f: runoff %.1f with the heat, %.1f even, %.1f against", hold, bw.Shed(), bf.Shed(), ba.Shed())
		}
	}
	against := seasonal(p, -0.9)
	shallow, deep := Bucket(60, &against, &pet), Bucket(300, &against, &pet)
	if deep.Evaporated() <= shallow.Evaporated() {
		t.Errorf("a deep bucket gave the air %.1f mm, a shallow one %.1f", deep.Evaporated(), shallow.Evaporated())
	}
}

// The rivers of a seasonal climate run highest in or after the wet season,
// never before it: the bucket fills first and sheds most once it is full.
func TestBucketRunsAfterTheRain(t *testing.T) {
	pet := seasonal(1000, 0.8)
	rain := [Phases]float64{50, 150, 700, 300} // a summer monsoon
	b := Bucket(150, &rain, &pet)
	most := 0
	for k := range Phases {
		if b.Runoff[k] > b.Runoff[most] {
			most = k
		}
	}
	t.Logf("monsoon: rain %v, runoff %.0f %.0f %.0f %.0f, water %.0f %.0f %.0f %.0f", rain,
		b.Runoff[0], b.Runoff[1], b.Runoff[2], b.Runoff[3], b.Water[0], b.Water[1], b.Water[2], b.Water[3])
	if most != 2 && most != 3 {
		t.Errorf("the monsoon's rivers ran highest in phase %d", most)
	}
	if b.Runoff[3] <= b.Runoff[1] {
		t.Errorf("the rivers ran %.1f mm after the monsoon and %.1f before it", b.Runoff[3], b.Runoff[1])
	}
}

// A year with no dry season needs no store to carry it; a monsoon's year
// needs its dry phases' use of the year's water, and a deeper rooted bucket
// under it gives the air more of its wet season's rain.
func TestDrySeasonRoots(t *testing.T) {
	rain, pet := even(1200), even(1000)
	if need := DryNeed(&rain, &pet); need != 0 {
		t.Errorf("unseasonal wet year needs %.1f mm of store, want none", need)
	}
	// Nine tenths of 900 mm in the north's summer, under 1600 mm of PET
	// spread evenly: the cover uses 900 over the year, 225 a phase, and the
	// three dry phases' 675 less their 90 of rain are what it draws down.
	rain = [Phases]float64{30, 30, 810, 30}
	pet = even(1600)
	need := DryNeed(&rain, &pet)
	if math.Abs(need-585) > 1e-9 {
		t.Errorf("monsoon year needs %.1f mm of store, want 585", need)
	}
	const soil, paw = 1.0, 0.15
	if r := Reach(soil, paw, 2, 0); r != 2 {
		t.Errorf("no need reaches %.2f m, want the cover's own 2", r)
	}
	r := Reach(soil, paw, 2, need)
	if got := Hold(soil, paw, r); r < rootDeepest && math.Abs(got-need) > 1e-6 {
		t.Errorf("reach %.2f m holds %.1f mm, want %.1f", r, got, need)
	}
	if r > rootDeepest || r <= 2 {
		t.Errorf("reach %.2f m, want deeper than 2 and no deeper than %.0f", r, rootDeepest)
	}
	shallow := Bucket(Hold(soil, paw, 2), &rain, &pet)
	deep := Bucket(Hold(soil, paw, r), &rain, &pet)
	if deep.Evaporated() <= shallow.Evaporated() {
		t.Errorf("deep roots give the air %.0f mm, shallow %.0f: want more", deep.Evaporated(), shallow.Evaporated())
	}
	t.Logf("monsoon year: need %.0f mm, roots %.1f m, evaporation %.0f mm against %.0f at 2 m", need, r, deep.Evaporated(), shallow.Evaporated())
}
