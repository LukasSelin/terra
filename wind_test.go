package terra

import (
	"math"
	"testing"
)

// oceanGlobe is a globe w by h tiles with nothing on it but the sea.
func oceanGlobe(w, h int) *Grid {
	g := NewGrid(w, h)
	g.Wrap = true
	g.air = Climate{rows: h, globe: true}.airFor(g, 1)
	g.sea, g.base = 0, 0
	return g
}

// zonalWind is the mean wind toward the east and the north over the tiles of
// g between two latitudes, in one phase of the year.
func zonalWind(g *Grid, lo, hi float64, day int) (east, north float64) {
	var n float64
	for i := range g.Tiles {
		lat := g.air.lat[i/g.W]
		if lat < lo || lat >= hi {
			continue
		}
		u, v := g.WindOn(i, day)
		east, north, n = east+u, north+v, n+1
	}
	return east / n, north / n
}

// The planet's circulation, on a world with nothing to get in its way: the
// trades blow from the east and toward the equator, the westerlies from the
// west and toward the pole, and the polar easterlies from the east again - the
// same way round in both hemispheres, because the turning of the planet is
// the other way round in the south and so is the slope of the belts.
func TestTheTradesBlowFromTheEastAndTheWesterliesFromTheWest(t *testing.T) {
	g := oceanGlobe(256, 128)
	g.weather()
	for _, c := range []struct {
		name           string
		lo, hi         float64
		east, poleward float64 // the signs wanted
	}{
		{"northern trades", 8, 25, -1, -1},
		{"southern trades", -25, -8, -1, -1},
		{"northern westerlies", 38, 55, 1, 1},
		{"southern westerlies", -55, -38, 1, 1},
		{"northern polar easterlies", 68, 82, -1, 0},
		{"southern polar easterlies", -82, -68, -1, 0},
	} {
		u, v := zonalWind(g, c.lo, c.hi, Year/2)
		if c.lo < 0 {
			v = -v
		}
		t.Logf("%s: %.1f m/s east, %.1f poleward", c.name, u, v)
		if u*c.east <= 0 {
			t.Errorf("the %s blow %.1f m/s toward the east", c.name, u)
		}
		if c.poleward != 0 && v*c.poleward <= 0 {
			t.Errorf("the %s blow %.1f m/s toward the pole", c.name, v)
		}
		if s := math.Hypot(u, v); s < 2 || s > 15 {
			t.Errorf("the %s blow at %.1f m/s", c.name, s)
		}
	}
}

// Rough ground drags on the wind harder than the sea does, so over land the
// wind is slower and blows further across the isobars toward the low.
func TestTheWindCrossesTheIsobarsMoreOverLandThanSea(t *testing.T) {
	sea := oceanGlobe(256, 128)
	sea.weather()
	land := oceanGlobe(256, 128)
	for i := range land.Tiles {
		land.Tiles[i].Height = 50 + 40*float64((i*7919)%13)
	}
	land.weather()
	us, vs := zonalWind(sea, 40, 50, Year/2)
	ul, vl := zonalWind(land, 40, 50, Year/2)
	angle := func(u, v float64) float64 { return math.Atan2(v, u) * 180 / math.Pi }
	t.Logf("sea %.1f m/s at %.0f degrees across; land %.1f m/s at %.0f", math.Hypot(us, vs), angle(us, vs), math.Hypot(ul, vl), angle(ul, vl))
	if math.Hypot(ul, vl) >= math.Hypot(us, vs) {
		t.Errorf("the wind over land is no slower than over the sea")
	}
	if angle(ul, vl) <= angle(us, vs)+5 {
		t.Errorf("the wind over land crosses the isobars at %.0f degrees and over the sea at %.0f", angle(ul, vl), angle(us, vs))
	}
}

// continent is an ocean globe with a square of low land centred on the given
// latitude, a quarter of the way round.
func continent(lat float64) *Grid {
	g := oceanGlobe(256, 128)
	c := Climate{rows: g.H, globe: true}
	for i := range g.Tiles {
		x, y := i%g.W, i/g.W
		if l := c.latitude(y); math.Abs(l-lat) < 20 && x >= 64 && x < 128 {
			g.Tiles[i].Height = 60
		}
	}
	return g
}

// A continent warms through its summer far more than the sea beside it and
// draws a low over itself, and the sea's air blows in toward it: the monsoon.
// In its winter it cools and the wind blows out.
func TestAHotContinentDrawsTheSeaWindInInSummer(t *testing.T) {
	g := continent(25)
	g.weather()
	// The mean wind across the continent's southern coast, toward the north.
	onshore := func(day int) float64 {
		c := Climate{rows: g.H, globe: true}
		var v, n float64
		for i := range g.Tiles {
			x, y := i%g.W, i/g.W
			if l := c.latitude(y); math.Abs(l-7) < 3 && x >= 72 && x < 120 {
				_, north := g.WindOn(i, day)
				v, n = v+north, n+1
			}
		}
		return v / n
	}
	summer, winter := onshore(Year/4), onshore(3*Year/4)
	t.Logf("across the south coast: %.1f m/s toward land in summer, %.1f in winter", summer, winter)
	if summer <= 0 || summer <= winter+1 {
		t.Errorf("the summer wind blows %.1f m/s onto the land and the winter wind %.1f", summer, winter)
	}
	low := func(day int) float64 {
		return g.PressureOn(g.W*32+96, day) - g.PressureOn(g.W*32+200, day)
	}
	if low(Year/4) >= 0 || low(3*Year/4) <= 0 {
		t.Errorf("the continent against the sea is %+.1f hPa in summer and %+.1f in winter", low(Year/4), low(3*Year/4))
	}
}

// A low turns anticlockwise in the north and clockwise in the south, and
// the wind round it blows a little in toward its middle in both.
func TestALowTurnsTheOtherWaySouthOfTheEquator(t *testing.T) {
	for _, lat := range []float64{45, -45} {
		e := newAirEnv(oceanGlobe(256, 128).withAir())
		c := Climate{rows: e.h, globe: true}
		cy := 0
		for y := 0; y < e.h; y++ {
			if math.Abs(c.latitude(y)-lat) < math.Abs(c.latitude(cy)-lat) {
				cy = y
			}
		}
		cx := e.w / 2
		extra := make([]float64, e.w*e.h)
		for y := 0; y < e.h; y++ {
			for x := 0; x < e.w; x++ {
				dx := float64(x-cx) * e.dx[y] / 1000
				dy := float64(y-cy) * e.dy / 1000
				extra[y*e.w+x] = -25 * math.Exp(-(dx*dx+dy*dy)/(2*800*800))
			}
		}
		n := e.w * e.h
		u, v, p := make([]float32, n), make([]float32, n), make([]float32, n)
		e.solve(0, extra, nil, u, v, p)
		// East of the middle, a wind turning anticlockwise blows north, and
		// one blowing in blows west.
		i := cy*e.w + cx + int(math.Round(800e3/e.dx[cy]))
		east, north := float64(u[i]), float64(v[i])
		t.Logf("at %v degrees, east of the low: %.1f m/s east, %.1f north", lat, east, north)
		if math.Copysign(1, lat)*north <= 0 {
			t.Errorf("at %v degrees the wind east of a low blows %.1f m/s north", lat, north)
		}
		if math.Hypot(east, north) < 5 {
			t.Errorf("a 25 hPa low at %v degrees has a wind of %.1f m/s", lat, math.Hypot(east, north))
		}
	}
}

// withAir gives a grid made by hand the air it would have.
func (g *Grid) withAir() *Grid {
	if g.air == nil {
		g.air = defaultAir(g)
	}
	return g
}

// A range too high for the wind to climb turns it aside along its face; a
// hill the wind can climb it simply crosses.
func TestAHighRangeTurnsTheWindAside(t *testing.T) {
	across := func(height float64) float64 {
		g := NewGrid(120, 60)
		for i := range g.Tiles {
			x := float64(i%g.W) - 60
			g.Tiles[i].Height = 20 + height*math.Exp(-x*x/(2*6*6))
		}
		g.weather()
		// The share of the wind blowing east on the windward face.
		var east, speed float64
		for y := 20; y < 40; y++ {
			for x := 48; x <= 54; x++ {
				u, v := g.MeanWind(y*g.W + x)
				east, speed = east+u, speed+math.Hypot(u, v)
			}
		}
		return east / speed
	}
	hill, wall := across(150), across(3000)
	t.Logf("share of the wind blowing at the range: under a 150 m hill %.2f, under a 3000 m wall %.2f", hill, wall)
	if hill < 0.6 {
		t.Errorf("a 150 m hill turns the wind aside: %.2f of it still blows at it", hill)
	}
	if wall > hill-0.3 {
		t.Errorf("a 3000 m wall leaves %.2f of the wind blowing at it, against %.2f for a hill", wall, hill)
	}
}

// Every copy of the wind is the same wind, however the work was dealt out.
func TestTheWindDoesNotDependOnTheGoroutines(t *testing.T) {
	sum := func(workers int) float64 {
		was := Workers
		Workers = workers
		defer func() { Workers = was }()
		g := continent(30)
		g.weather()
		var s float64
		for k := range phases {
			for i := range g.winds.u[k] {
				s += float64(g.winds.u[k][i])*float64(i%97) + float64(g.winds.v[k][i]) + float64(g.winds.p[k][i])
			}
		}
		return s
	}
	if a, b := sum(1), sum(8); a != b {
		t.Errorf("one goroutine made %v and eight %v", a, b)
	}
}
