package terra

import (
	"math"
	"testing"
)

// Rain falls in belts: most under the equator, least in the horse latitudes
// where the air sinks, and more again in the westerlies. On land, which is
// where anyone would notice.
func TestRainFallsInBelts(t *testing.T) {
	g := NewLand(1, smallGlobe()).Grid
	band := func(lo, hi float64) float64 {
		var sum, n float64
		for i := range g.Tiles {
			lat := math.Abs(g.air.lat[i/g.W])
			if g.underSea(i) || lat < lo || lat >= hi {
				continue
			}
			sum, n = sum+g.rain[i], n+1
		}
		if n == 0 {
			t.Fatalf("no land between %v and %v degrees", lo, hi)
		}
		return sum / n
	}
	tropics, horse, westerlies, polar := band(0, 10), band(20, 30), band(40, 55), band(70, 90)
	if !(tropics > westerlies && westerlies > horse && westerlies > polar) {
		t.Errorf("rain on land by latitude: tropics %.0f, horse latitudes %.0f, westerlies %.0f, polar %.0f mm",
			tropics, horse, westerlies, polar)
	}
}

// ridged is a flat valley with one ridge of the given height running north to
// south across the middle of it.
func ridged(height float64) *Grid {
	g := NewGrid(120, 20)
	for i := range g.Tiles {
		x := float64(i%g.W) - 60
		g.Tiles[i].Height = 20 + height*math.Exp(-x*x/(2*8*8))
	}
	return g
}

// meanRain is the mean rain over columns lo to hi of g.
func meanRain(g *Grid, lo, hi int) float64 {
	var sum, n float64
	for i := range g.Tiles {
		if x := i % g.W; x >= lo && x <= hi {
			sum, n = sum+g.rain[i], n+1
		}
	}
	return sum / n
}

// Air made to rise over a ridge gives up its water on the way up, so the side
// the wind comes from is wet and the side it leaves by is dry. Turn the wind
// round and the shadow goes with it.
func TestAMountainCastsARainShadow(t *testing.T) {
	g := ridged(300)
	g.weather()
	upwind, lee := meanRain(g, 45, 58), meanRain(g, 62, 75)
	if upwind < 1.3*lee {
		t.Errorf("under the westerlies the west face gets %.0f mm and the east %.0f", upwind, lee)
	}
	if far := meanRain(g, 0, 10); upwind <= far {
		t.Errorf("the west face gets %.0f mm and the plain upwind of it %.0f", upwind, far)
	}

	// The same valley under the polar easterlies.
	g = ridged(300)
	g.air = defaultAir(g)
	for y := range g.air.lat {
		g.air.lat[y] = 75
	}
	g.weather()
	if east, west := meanRain(g, 62, 75), meanRain(g, 45, 58); east < 1.3*west {
		t.Errorf("under easterlies the east face gets %.0f mm and the west %.0f", east, west)
	}
}

// The shadow is cast along the wind and not along the rows the map happens to
// be drawn in. The westerlies blow toward the north-east as well as the east,
// so a range running from the north-west to the south-east stands square
// across them: wet on its south-western face, dry on its north-eastern.
func TestARangeAtAnAngleCastsItsShadowDownwind(t *testing.T) {
	g := NewGrid(120, 120)
	// across is how far a place is across the range, which runs along x = y,
	// toward its north-eastern side.
	across := func(i int) (float64, float64) {
		x, y := float64(i%g.W)-60, float64(i/g.W)-60
		return (x - y) / math.Sqrt2, (x + y) / math.Sqrt2
	}
	for i := range g.Tiles {
		d, _ := across(i)
		g.Tiles[i].Height = 20 + 300*math.Exp(-d*d/(2*8*8))
	}
	g.weather()
	var wet, dry, nw, nd float64
	for i := range g.Tiles {
		d, along := across(i)
		if math.Abs(along) > 40 {
			continue
		}
		switch {
		case d > -14 && d < -3:
			wet, nw = wet+g.rain[i], nw+1
		case d > 3 && d < 14:
			dry, nd = dry+g.rain[i], nd+1
		}
	}
	wet, dry = wet/nw, dry/nd
	if wet < 1.2*dry {
		t.Errorf("the south-west face gets %.0f mm and the north-east %.0f", wet, dry)
	} else {
		t.Logf("the south-west face gets %.0f mm and the north-east %.0f", wet, dry)
	}
}

// What runs off is what fell less what the air took back, and the air never
// takes back more than fell or more than it could hold.
func TestRunoffIsWhatTheRainLeaves(t *testing.T) {
	g := NewLand(2, smallGlobe()).Grid
	for i := range g.Tiles {
		if r, p := g.runoff[i], g.rain[i]; r < -1e-9 || r > p+1e-9 {
			t.Fatalf("tile %d has %.1f mm of rain and %.1f of runoff", i, p, r)
		}
	}
	for _, c := range []struct{ p, pet float64 }{{1000, 10}, {1000, 800}, {1000, 5000}, {10, 1000}} {
		e := fu(c.p, c.pet)
		if e < 0 || e > c.p || e > c.pet {
			t.Errorf("fu(%v, %v) = %v", c.p, c.pet, e)
		}
	}
	// Where the air could take hardly anything it takes nearly all of it, and
	// where it could take everything it takes nearly all the rain.
	if e := fu(1000, 10); e < 9 {
		t.Errorf("with 10 mm the air could take, it took %v of 1000", e)
	}
	if e := fu(10, 10000); e < 9.9 {
		t.Errorf("with 10 mm of rain and a desert's air, it took back %v", e)
	}
}

// Wetness is the rain the air carries, and nothing else: a world twice as wet
// has twice the rain everywhere, and more than twice the runoff, because the
// air takes back no more for there being more to take.
func TestWetnessScalesTheRain(t *testing.T) {
	dry := NewLand(4, DefaultTerms()).Grid
	wetTerms := DefaultTerms()
	wetTerms.Wetness = 2
	wet := NewLand(4, wetTerms).Grid
	pd, pw := meanLand(dry, dry.rain), meanLand(wet, wet.rain)
	if math.Abs(pw/pd-2) > 0.1 {
		t.Errorf("twice the wetness gave %.0f mm of rain against %.0f", pw, pd)
	}
	if rd, rw := meanLand(dry, dry.runoff), meanLand(wet, wet.runoff); rw < 2*rd {
		t.Errorf("twice the wetness gave %.0f mm of runoff against %.0f", rw, rd)
	}
}

// A valley's weather is one latitude's: the air reads the same on every row.
func TestAValleyHasOneLatitude(t *testing.T) {
	g := NewLand(1, DefaultTerms()).Grid
	for y := 1; y < g.H; y++ {
		if g.air.lat[y] != g.air.lat[0] || g.air.mean[y] != g.air.mean[0] || g.air.dx[y] != g.air.dx[0] {
			t.Fatalf("row %d of a valley has different air from row 0", y)
		}
	}
}

// What the air rains out over a globe it took up somewhere: the evaporation
// and the rain of each phase, over the whole planet, come to the same within
// a part in a hundred (a planet's water cycle closes: Trenberth and others,
// 2007).
func TestTheWaterTheAirTakesUpFallsAgain(t *testing.T) {
	for _, g := range []*Grid{oceanGlobe(256, 128), twoOceans(), continent(25)} {
		g.weather()
		e := g.winds.airEnv
		for k := range phases {
			b := g.winds.budget[k]
			var evap, rain float64
			for cy := 0; cy < e.h; cy++ {
				area := e.dx[cy] * e.dy
				for cx := 0; cx < e.w; cx++ {
					i := cy*e.w + cx
					evap += area * b.evap[i]
					rain += area * (b.rain[i] + b.oro[i])
				}
			}
			if math.Abs(rain/evap-1) > 0.01 {
				t.Errorf("phase %d: the air takes up %.4g kg/s and rains %.4g", k, evap, rain)
			}
			// And a planet's worth: some two and a half to three and a half mm
			// a day (Trenberth and others, 2007: 2.7).
			var area float64
			for cy := 0; cy < e.h; cy++ {
				area += e.dx[cy] * e.dy * float64(e.w)
			}
			if mm := evap / area * 86400; mm < 2 || mm > 4 {
				t.Errorf("phase %d: the planet evaporates %.2f mm a day", k, mm)
			}
		}
	}
}

// The shadow goes on past the crest: the lee's rain is less than the plain's
// upwind all the way down the far side and some way beyond its foot, because
// the air that comes over has left its water on the windward face.
func TestTheShadowReachesPastTheFootOfTheRange(t *testing.T) {
	g := ridged(800)
	g.weather()
	plain := meanRain(g, 0, 20)
	// The ridge's crest is column 60, and its foot some three widths - 24
	// columns - east of it; the lee is read from just over the top to eight
	// columns past the foot.
	for x := 64; x <= 88; x += 4 {
		if r := meanRain(g, x, x+3); r >= plain {
			t.Errorf("column %d, %d past the crest, has %.0f mm against the plain's %.0f", x, x-60, r, plain)
		}
	}
}
