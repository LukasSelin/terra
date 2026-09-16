package terra

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/LukasSelin/terra/geom"
	"github.com/LukasSelin/terra/internal/atmos"
)

// weatherOver is the day's weather over a grid made by hand, with no systems
// in it yet.
func weatherOver(g *Grid) *Weather {
	g.weather()
	return atmos.StillWeather(rand.New(rand.NewPCG(1, 2)), g.winds)
}

// A low in the middle latitudes is carried east by the westerlies aloft, some
// seven or eight hundred kilometres a day, and wanders toward its pole.
func TestStormsMoveEastInTheWesterlies(t *testing.T) {
	wx := weatherOver(oceanGlobe(256, 128))
	wx.Systems = []System{{Kind: Low, Lat: 45, Lon: 0, Depth: 20, Radius: 800, Life: 100}}
	for day := range 3 {
		wx.Step(Year/2 + day)
	}
	s := wx.Systems[0]
	km := s.Lon * 111.32 * math.Cos(45*math.Pi/180)
	t.Logf("after three days the low is %.0f km east and at %.1f degrees", km, s.Lat)
	if km < 1500 || km > 3500 {
		t.Errorf("a low in the westerlies went %.0f km east in three days", km)
	}
	if s.Lat <= 45 {
		t.Errorf("a northern low went from 45 degrees to %.1f", s.Lat)
	}
}

// A tropical storm lives on warm sea and dies on reaching land.
func TestHurricanesDieOverLand(t *testing.T) {
	g := continent(15)
	wx := weatherOver(g)
	storm := func(lon float64) System {
		return System{Kind: Storm, Lat: 15, Lon: lon, Depth: 50, Radius: 200, Life: 8}
	}
	// The continent is the second quarter of the way round, from -90 to 0.
	wx.Systems = []System{storm(-45), storm(90)}
	for day := range 3 {
		wx.Step(Year/4 + day)
	}
	var land, sea bool
	for _, s := range wx.Systems {
		if s.Kind != Storm {
			continue
		}
		switch {
		case s.Lon < 0 && s.Lon > -100:
			land = true
		case s.Lon > 0:
			sea = true
		}
	}
	if land {
		t.Errorf("a storm over land lived three days")
	}
	if !sea {
		t.Errorf("a storm over warm sea in summer died within three days")
	}
}

// The day's weather comes from a stream of chance of its own: the same seed
// gives the same weather, and asking for the weather leaves the world's own
// chance where it was.
func TestTheWeatherIsTheSameForTheSameSeed(t *testing.T) {
	run := func(ask bool) (*Land, uint64) {
		w := NewLand(7, DefaultTerms())
		for tick := range 40 {
			w.Tick = tick
			w.Climate.Advance(tick, w.RNG)
			if ask {
				w.AdvanceWeather()
			}
		}
		return w, w.RNG.Uint64()
	}
	a, ra := run(true)
	b, rb := run(true)
	_, rc := run(false)
	if ra != rc || rb != rc {
		t.Errorf("asking for the weather moved the world's chance")
	}
	if len(a.Weather.Systems) != len(b.Weather.Systems) {
		t.Fatalf("the same seed had %d and %d systems", len(a.Weather.Systems), len(b.Weather.Systems))
	}
	for i := range a.Weather.Systems {
		if a.Weather.Systems[i] != b.Weather.Systems[i] {
			t.Fatalf("system %d differs between two runs of the same seed", i)
		}
	}
	for y := 0; y < a.Grid.H; y += 5 {
		for x := 0; x < a.Grid.W; x += 7 {
			p := geom.Pos{X: x, Y: y}
			au, av := a.WindAt(p)
			bu, bv := b.WindAt(p)
			if au != bu || av != bv || a.PressureAt(p) != b.PressureAt(p) {
				t.Fatalf("the wind at %v differs between two runs of the same seed", p)
			}
		}
	}
}

// Day after day the weather stays weather: pressures in the range the real
// world's sea level has, winds a gale at most outside the storms, lows and
// highs coming and going, and the wind on one day not the wind of the next.
func TestTheWeatherChangesFromDayToDay(t *testing.T) {
	if testing.Short() {
		t.Skip("makes a globe and a season of its weather")
	}
	w := madeLand(3, smallGlobe())
	g := w.Grid
	probe := geom.Pos{X: 40, Y: 30}
	var lowest, highest, most float64 = 2000, 0, 0
	var last [2]float64
	changed, storms := 0, 0
	for tick := range 60 {
		w.Tick = tick + Year/5
		w.AdvanceWeather()
		for i := range g.Tiles {
			p := g.PosOf(i)
			pr := w.PressureAt(p)
			lowest, highest = math.Min(lowest, pr), math.Max(highest, pr)
			u, v := w.WindAt(p)
			most = math.Max(most, math.Hypot(u, v))
		}
		u, v := w.WindAt(probe)
		if math.Hypot(u-last[0], v-last[1]) > 1 {
			changed++
		}
		last = [2]float64{u, v}
		for _, s := range w.Weather.Systems {
			if s.Kind == Storm {
				storms++
			}
		}
	}
	t.Logf("pressure %.0f to %.0f hPa, wind at most %.0f m/s, the probe's wind changed on %d of 60 days, %d storm-days",
		lowest, highest, most, changed, storms)
	// The sea-level extremes on record: 870 hPa in Typhoon Tip (1979), 1084 in
	// Siberia (1968). It was 930 to 1060, and a storm over the shaped ground of
	// seed 3 deepened to 924 - see shape.go.
	if lowest < 870 || highest > 1084 {
		t.Errorf("pressure ran from %.0f to %.0f hPa", lowest, highest)
	}
	if most > atmos.WindMost || most < 10 {
		t.Errorf("the strongest wind in sixty days was %.0f m/s", most)
	}
	if changed < 20 {
		t.Errorf("the wind at %v changed on only %d of sixty days", probe, changed)
	}
}

// The day's weather is the same however the work was dealt out.
func TestTheWeatherDoesNotDependOnTheGoroutines(t *testing.T) {
	run := func(workers int) []float32 {
		was := Workers
		Workers = workers
		defer func() { Workers = was }()
		wx := weatherOver(continent(35))
		for day := range 12 {
			wx.Step(day)
			wx.Solve(day)
		}
		return append(append(append([]float32(nil), wx.U...), wx.V...), wx.P...)
	}
	one, many := run(1), run(8)
	for i := range one {
		if one[i] != many[i] {
			t.Fatalf("one goroutine and eight made different weather at %d: %v and %v", i, one[i], many[i])
		}
	}
}
