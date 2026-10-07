package atmos

import (
	"math"
	"sync"
	"testing"
)

// writtenInsolation is the daily sun as ebm.go wrote it down before there was
// a Forcing, kept here to hold Today's to it.
func writtenInsolation(phi, j float64) float64 {
	d := 0.409 * math.Sin(2*math.Pi*j/365.25-1.39)
	dr := 1 + 0.033*math.Cos(2*math.Pi*j/365.25)
	ws := math.Acos(math.Max(-1, math.Min(1, -math.Tan(phi)*math.Tan(d))))
	return solarConstant / math.Pi * dr * (ws*math.Sin(phi)*math.Sin(d) + math.Cos(phi)*math.Cos(d)*math.Sin(ws))
}

// Today's forcing is the sun and the air the balance was worked out under
// before it could be anything else, to the bit: the daily sun at every
// latitude on every day, and the outgoing longwave's constant.
func TestTodaysForcingIsTheWrittenOne(t *testing.T) {
	for _, f := range []Forcing{Today(), {}} {
		for lat := -90.0; lat <= 90; lat += 0.5 {
			phi := lat * math.Pi / 180
			for j := 0.0; j < 366; j += 0.25 {
				if got, want := f.OrDefault().insolation(phi, j), writtenInsolation(phi, j); math.Float64bits(got) != math.Float64bits(want) {
					t.Fatalf("%v degrees, day %v: the sun is %v and was %v", lat, j, got, want)
				}
			}
		}
		if got := f.OrDefault().olrA(); math.Float64bits(got) != math.Float64bits(olrA) {
			t.Errorf("today's outgoing longwave is %v + B T, and was %v", got, olrA)
		}
	}
	if !(Forcing{}).today() || !Today().today() {
		t.Error("the zero forcing and Today are not today's")
	}
}

// Doubling the carbon in the air warms the planet by the balance's
// sensitivity: 5.35 ln 2 = 3.7 W/m² over Budyko's B of 2.09 is 1.8 degrees
// with nothing to answer it, and the ice's retreat adds to that. The real
// figure is 2.5 to 4 (Sherwood et al., 2020), with the water vapour and the
// clouds this balance has no say over.
func TestDoublingTheCarbonWarmsThePlanet(t *testing.T) {
	today := Today()
	double := today
	double.CO2 *= 2
	half := today
	half.CO2 /= 2
	t0, t2, th := GlobalMeanUnder(today), GlobalMeanUnder(double), GlobalMeanUnder(half)
	t.Logf("global mean %.2f C at %v ppm, %.2f at %v (+%.2f), %.2f at %v (%.2f)",
		t0, today.CO2, t2, double.CO2, t2-t0, th, half.CO2, th-t0)
	if t2-t0 < 1.5 || t2-t0 > 5 {
		t.Errorf("doubling the carbon warms the planet %.2f degrees", t2-t0)
	}
	if th >= t0 {
		t.Errorf("halving the carbon warms the planet %.2f degrees", th-t0)
	}
	for _, lat := range []float64{0, 45, 75} {
		t.Logf("%v degrees: %.2f C, %.2f doubled", lat, ZonalMeanUnder(today, lat), ZonalMeanUnder(double, lat))
	}
}

// A planet tilted less gives its high latitudes less summer sun: at
// sixty-five degrees, 22.1 degrees of tilt - the least of the last cycle -
// against 24.5, the most. Its yearly sun at the pole is less too, and its
// equator's more.
func TestLessTiltIsLessHighSummerSun(t *testing.T) {
	low, high := Today(), Today()
	low.Obliquity, high.Obliquity = 22.1*degree, 24.5*degree
	north, south := 65*degree, -65*degree
	summer := func(f Forcing, phi, from float64) float64 {
		var s float64
		for j := from; j < from+91; j++ {
			s += f.insolation(phi, j) / 91
		}
		return s
	}
	ln, hn := summer(low, north, 141), summer(high, north, 141) // the northern summer's quarter about the solstice
	ls, hs := summer(low, south, 324), summer(high, south, 324)
	t.Logf("65N summer %.1f W/m² at 22.1 degrees, %.1f at 24.5; 65S %.1f and %.1f", ln, hn, ls, hs)
	if ln >= hn || ls >= hs {
		t.Errorf("summer sun at 65 degrees: %.1f N, %.1f S under 22.1 of tilt, %.1f, %.1f under 24.5", ln, ls, hn, hs)
	}
	year := func(f Forcing, phi float64) float64 {
		var s float64
		for j := 0.0; j < 365; j++ {
			s += f.insolation(phi, j+0.5) / 365
		}
		return s
	}
	if year(low, 89*degree) >= year(high, 89*degree) || year(low, 0) <= year(high, 0) {
		t.Errorf("a year's sun under 22.1 of tilt and 24.5: %.1f and %.1f at the pole, %.1f and %.1f at the equator",
			year(low, 89*degree), year(high, 89*degree), year(low, 0), year(high, 0))
	}
}

// The precession: with perihelion in the northern summer - the sun at
// longitude ninety degrees, the planet at two hundred and seventy - the
// northern midsummer sun is stronger than today's, with perihelion in
// January, and the southern one weaker.
func TestPerihelionInSummerIsAStrongerSummer(t *testing.T) {
	today := Today()
	june := today
	june.Perihelion = 270 * degree
	june.Eccentricity = 0.05
	n0, n1 := today.insolation(65*degree, 172), june.insolation(65*degree, 172)
	s0, s1 := today.insolation(-65*degree, 355), june.insolation(-65*degree, 355)
	t.Logf("midsummer at 65N %.1f W/m² today, %.1f with perihelion in June; at 65S %.1f and %.1f", n0, n1, s0, s1)
	if n1 <= n0 || s1 >= s0 {
		t.Errorf("perihelion in June: 65N's midsummer %.1f against %.1f, 65S's %.1f against %.1f", n1, n0, s1, s0)
	}
}

// The balances are kept by forcing: the same forcing is the same balance,
// asked from many goroutines at once; another is another; and the zero
// forcing is today's.
func TestTheBalancesAreKeptByForcing(t *testing.T) {
	if ebmUnder(Forcing{}) != ebm() || ebmUnder(Today()) != ebm() {
		t.Fatal("the zero forcing and Today are not today's balance")
	}
	f := Today()
	f.Solar = 1360
	var wg sync.WaitGroup
	got := make([]*ebmClimate, 8)
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i] = ebmUnder(f)
		}()
	}
	wg.Wait()
	for i, e := range got {
		if e != got[0] {
			t.Fatalf("goroutine %d was given a balance of its own", i)
		}
	}
	if got[0] == ebm() {
		t.Fatal("a dimmer sun is today's balance")
	}
	g := f
	g.Perihelion += 1e-9
	if ebmUnder(g) == got[0] {
		t.Error("forcings a nanoradian of perihelion apart are kept as one")
	}
	// A balance worked out again is the same balance, to the bit.
	again := solveEBMUnder(f)
	for k := range again.mean {
		if math.Float64bits(again.mean[k]) != math.Float64bits(got[0].mean[k]) || again.swingL[k] != got[0].swingL[k] {
			t.Fatalf("band %d: worked out twice, the balance is %v and %v", k, again.mean[k], got[0].mean[k])
		}
	}
	if d := GlobalMeanUnder(f) - GlobalMeanUnder(Today()); d >= 0 {
		t.Errorf("a sun one W/m² dimmer warms the planet %.3f degrees", d)
	}
}

// The orbit of the past is Berger's: today's (1950's), the last glacial
// maximum's and the middle Holocene's as the Paleoclimate Modelling
// Intercomparison Project sets them from Berger (1978) - an eccentricity of
// 0.018994, a tilt of 22.949 and a perihelion of 114.42 at 21 000 years, and
// 0.018682, 24.105 and 0.87 at 6000 (Braconnot et al., 2007).
func TestTheOrbitOfThePast(t *testing.T) {
	for _, c := range []struct{ years, ecc, obl, peri float64 }{
		{0, 0.016724, 23.446, 102.04},
		{21000, 0.018994, 22.949, 114.42},
		{6000, 0.018682, 24.105, 0.87},
	} {
		o := Today().OrbitBefore(c.years)
		obl, peri := o.Obliquity/degree, o.Perihelion/degree
		dp := math.Mod(peri-c.peri+540, 360) - 180
		t.Logf("%v years ago: eccentricity %.6f, tilt %.3f, perihelion %.2f", c.years, o.Eccentricity, obl, peri)
		if math.Abs(o.Eccentricity-c.ecc) > 2e-4 || math.Abs(obl-c.obl) > 0.02 || math.Abs(dp) > 0.5 {
			t.Errorf("%v years ago: eccentricity %.6f, tilt %.3f, perihelion %.2f; want %v, %v, %v",
				c.years, o.Eccentricity, obl, peri, c.ecc, c.obl, c.peri)
		}
		if o.CO2 != Today().CO2 || o.Solar != Today().Solar {
			t.Errorf("%v years ago: the orbit moved the sun or the air", c.years)
		}
		if err := o.Check(); err != nil {
			t.Error(err)
		}
	}
	// Over the last million years the tilt keeps between 22 and 24.5 degrees
	// and the eccentricity under 0.06.
	for y := 0.0; y <= 1e6; y += 1000 {
		o := Today().OrbitBefore(y)
		if obl := o.Obliquity / degree; obl < 22 || obl > 24.6 || o.Eccentricity < 0 || o.Eccentricity > 0.06 {
			t.Fatalf("%v years ago: tilt %.3f, eccentricity %.4f", y, obl, o.Eccentricity)
		}
	}
}

// A forcing that cannot be worked out under is turned away.
func TestAForcingIsChecked(t *testing.T) {
	if err := (Forcing{}).Check(); err != nil {
		t.Error(err)
	}
	if err := Today().Check(); err != nil {
		t.Error(err)
	}
	for _, bad := range []func(*Forcing){
		func(f *Forcing) { f.CO2 = 0 },
		func(f *Forcing) { f.CO2 = math.NaN() },
		func(f *Forcing) { f.Eccentricity = -0.01 },
		func(f *Forcing) { f.Eccentricity = 0.5 },
		func(f *Forcing) { f.Obliquity = 2 },
		func(f *Forcing) { f.Perihelion = math.Inf(1) },
		func(f *Forcing) { f.Solar = 0 },
	} {
		f := Today()
		bad(&f)
		if f.Check() == nil {
			t.Errorf("%+v passes", f)
		}
	}
}
