package atmos

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
)

// What falls on a cold year leaves it: the air's take, off the snow and the
// soil, and the rivers', with a glacier's ice, add to the precipitation, the
// pack's year is steady, the air takes no more than it could, and a
// glacier is never bare.
func TestSnowYearBalances(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	var glaciers, seasonal int
	worst := 0.0
	for range 5000 {
		var rain, pet [Phases]float64
		var p, e float64
		for k := range Phases {
			rain[k] = 800 * r.Float64() * r.Float64()
			pet[k] = 300 * r.Float64() * r.Float64()
			p += rain[k]
			e += pet[k]
		}
		if p < 10 {
			continue
		}
		mean, swing := -25+35*r.Float64(), (2+25*r.Float64())*math.Copysign(1, r.Float64()-0.3)
		hold := 10 + 300*r.Float64()
		b := BucketCold(hold, &rain, &pet, mean, swing)
		gone := b.Shed() + b.Evaporated()
		worst = math.Max(worst, math.Abs(gone-p)/p)
		if b.Evaporated() > e*(1+1e-9)+1e-9 {
			t.Fatalf("mean %.1f swing %.1f: the air took %.2f of %.2f it could", mean, swing, b.Evaporated(), e)
		}
		var fell, melted float64
		for k := range Phases {
			fell += b.Snowfall[k]
			melted += b.Melt[k]
			if b.Snow[k] < 0 || b.Cover[k] < 0 || b.Cover[k] > 1 || b.Runoff[k] < 0 || b.Water[k] > hold*(1+1e-12) {
				t.Fatalf("mean %.1f swing %.1f: phase %d %+v", mean, swing, k, b)
			}
		}
		if b.Ice > 0 {
			glaciers++
			if min(b.Cover[0], b.Cover[1], b.Cover[2], b.Cover[3]) <= 0 {
				t.Fatalf("a glacier covered %v", b.Cover)
			}
		} else if fell > 0 {
			seasonal++
			// What falls as snow melts or goes to the air within the year.
			if melted > fell*(1+1e-9) {
				t.Fatalf("mean %.1f swing %.1f: %.2f mm melted of %.2f fallen", mean, swing, melted, fell)
			}
		}
	}
	t.Logf("%d glaciers, %d seasonal packs; unbalanced by %.2g of the precipitation at worst", glaciers, seasonal, worst)
	if worst > 1e-3 {
		t.Errorf("the year lost %.4f of its precipitation", worst)
	}
}

// A year too warm to snow is the bucket's without snow, to the bit.
func TestWarmYearHasNoSnow(t *testing.T) {
	rain, pet := seasonal(900, 0.5), seasonal(700, 0.8)
	if a, b := Bucket(120, &rain, &pet), BucketCold(120, &rain, &pet, 25, 5); a != b {
		t.Errorf("a warm year's bucket %+v, without snow %+v", b, a)
	}
}

// A cold country's rivers run in the spring and early summer, off the
// winter's snow, and not in the winter it fell in: the bucket alone sheds a
// winter's precipitation in the winter.
func TestSnowmeltRunsInTheSpring(t *testing.T) {
	rain := even(600)
	pet := petShared(-2, 16, 350)
	b := BucketCold(150, &rain, &pet, -2, 16)
	w := Bucket(150, &rain, &pet)
	t.Logf("mean -2, swing 16, 600 mm even: runoff by phase (winter, spring, summer, autumn) %.0f %.0f %.0f %.0f with snow, %.0f %.0f %.0f %.0f without; melt %.0f %.0f %.0f %.0f; snow %.0f %.0f %.0f %.0f; cover %.2f %.2f %.2f %.2f",
		b.Runoff[0], b.Runoff[1], b.Runoff[2], b.Runoff[3], w.Runoff[0], w.Runoff[1], w.Runoff[2], w.Runoff[3],
		b.Melt[0], b.Melt[1], b.Melt[2], b.Melt[3], b.Snow[0], b.Snow[1], b.Snow[2], b.Snow[3],
		b.Cover[0], b.Cover[1], b.Cover[2], b.Cover[3])
	most := 0
	for k := range Phases {
		if b.Runoff[k] > b.Runoff[most] {
			most = k
		}
	}
	if most != 1 && most != 2 {
		t.Errorf("the snow-fed rivers ran highest in phase %d", most)
	}
	if b.Runoff[0] >= w.Runoff[0] || b.Snow[0] <= b.Snow[2] {
		t.Errorf("the winter shed %.1f mm with snow and %.1f without; it held %.1f mm of snow, the summer %.1f",
			b.Runoff[0], w.Runoff[0], b.Snow[0], b.Snow[2])
	}
}

// Snow that outlasts the year is a glacier, and the south's winter is its
// own.
func TestGlacierAndHemispheres(t *testing.T) {
	rain, pet := even(1500), even(50)
	b := BucketCold(100, &rain, &pet, -12, 10)
	if b.Ice <= 0 {
		t.Errorf("a year at -12 under 1500 mm kept no ice: %+v", b)
	}
	t.Logf("glacier at -12, 1500 mm: balance %.0f mm a year, runoff %.0f %.0f %.0f %.0f", b.Ice,
		b.Runoff[0], b.Runoff[1], b.Runoff[2], b.Runoff[3])
	north := BucketCold(100, &rain, &pet, 0, 12)
	south := BucketCold(100, &rain, &pet, 0, -12)
	if north.Snow[0] <= north.Snow[2] || south.Snow[2] <= south.Snow[0] {
		t.Errorf("snow north %v, south %v", north.Snow, south.Snow)
	}
}

// elaRain is the year's precipitation, spread evenly over the phases, at
// which ground whose year swings swing either side of a mean that puts its
// summer quarter at summer degrees just keeps its snow: where the mass
// balance is nothing. The air takes pet a year off it, shared as the phases'
// warmth is.
func elaRain(summer, swing, pet float64) float64 {
	mean := summer - SummerPeak*swing
	take := petShared(mean, swing, pet)
	lo, hi := 0.0, 50000.0
	for range 60 {
		mid := (lo + hi) / 2
		rain := even(mid)
		var in, dry [Phases * bucketSteps]float64
		var out BucketYear
		snowYear(&rain, &take, mean, swing, bucketSteps, in[:], dry[:], &out)
		if out.Ice > 0 {
			hi = mid
		} else {
			lo = mid
		}
	}
	return (lo + hi) / 2
}

// petShared is pet mm a year shared over the phases as their warmth over
// freezing is, as RainCells shares it (without the sun).
func petShared(mean, swing, pet float64) [Phases]float64 {
	var each [Phases]float64
	var total float64
	for k, s := range phaseSin {
		each[k] = math.Max(0, mean+swing*s)
		total += each[k]
	}
	for k := range each {
		if total > 0 {
			each[k] *= pet / total
		}
	}
	return each
}

// The snow's equilibrium line is Ohmura's: the precipitation at which a
// year's snow just lasts it, against the summer's warmth, is what Ohmura,
// Kasser and Funk (1992) fitted at seventy glaciers' lines to within
// two fifths where the year swings ten degrees or more either way, and to
// within twice over a maritime year's six, over the summers from
// freezing to six degrees their glaciers mostly span.
func TestSnowLineIsOhmuras(t *testing.T) {
	for _, swing := range []float64{6, 10, 15, 20} {
		for _, pet := range []float64{0, 150} {
			line := ""
			worst := 0.0
			for summer := 0.0; summer <= 6; summer += 1 {
				ohmura := 645 + 296*summer + 9*summer*summer
				p := elaRain(summer, swing, pet)
				line += fmt.Sprintf(" %.0f°:%.0f/%.0f", summer, p, ohmura)
				worst = math.Max(worst, math.Abs(math.Log(p/ohmura)))
			}
			t.Logf("swing %2.0f, pet %3.0f: precipitation at the line, model/Ohmura:%s", swing, pet, line)
			limit := 1.4
			if swing < 10 {
				limit = 2.1
			}
			if math.Exp(worst) > limit {
				t.Errorf("swing %.0f, pet %.0f: the line is %.2f times off Ohmura's", swing, pet, math.Exp(worst))
			}
		}
	}
}

// The table the snow is read off is the snow's own functions, to a part in a
// hundred thousand.
func TestSnowTable(t *testing.T) {
	worst := 0.0
	for x := -40.0; x <= 40; x += 0.0137 {
		share, days := snowAt(x)
		s := snowShare(x)
		if s < snowLeast {
			s = 0
		}
		worst = math.Max(worst, math.Max(math.Abs(share-s), math.Abs(days-degreeDays(x))))
	}
	if worst > 1e-5 {
		t.Errorf("the table is off by %.2g", worst)
	}
}
