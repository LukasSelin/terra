package terra

import (
	"math"
	"testing"

	"github.com/LukasSelin/terra/internal/atmos"
)

// twoOceans is an ocean globe with two continents running from seventy degrees
// south to seventy north, each forty columns wide, and two oceans between
// them: the land in columns 0 to 39 and 128 to 167.
func twoOceans() *Grid {
	g := oceanGlobe(256, 128)
	c := Climate{rows: g.H, globe: true}
	for i := range g.Tiles {
		x, y := i%g.W, i/g.W
		if math.Abs(c.latitude(y)) < 70 && (x < 40 || (x >= 128 && x < 168)) {
			g.Height[i] = 60
		}
	}
	return g
}

// band is the mean of read over the tiles of g between two latitudes and in
// the columns from x0 up to x1.
func band(g *Grid, lo, hi float64, x0, x1 int, read func(i int) float64) float64 {
	var s, n float64
	for y := 0; y < g.H; y++ {
		if l := g.air.Lat[y]; l < lo || l >= hi {
			continue
		}
		for x := x0; x < x1; x++ {
			s, n = s+read(y*g.W+x), n+1
		}
	}
	return s / n
}

// The water the trades and the westerlies drive round an ocean comes back up
// its western side warm from the tropics, and down its eastern side cold, and
// colder still where the wind drives it off the shore: the Gulf Stream and
// the Canaries, the Brazil and the Benguela.
func TestTheWarmCurrentRunsUpTheWestSideOfAnOcean(t *testing.T) {
	g := twoOceans()
	g.weather()
	for _, hemi := range []float64{1, -1} {
		lat := 30 * hemi
		west := band(g, 35*hemi-7.5, 35*hemi+7.5, 40, 44, g.SeaWarmth)
		east := band(g, 20*hemi-7.5, 20*hemi+7.5, 124, 128, g.SeaWarmth)
		t.Logf("at %v degrees: the sea off the western shore %+.1f degrees, off the eastern %+.1f", lat, west, east)
		if west < 2 {
			t.Errorf("at %v degrees the western current is %+.1f degrees", lat, west)
		}
		if east > -1 {
			t.Errorf("at %v degrees the eastern current is %+.1f degrees", lat, east)
		}
	}
}

// A parallel of open sea all the way round has no shore for a gyre to turn
// at, and no western current to warm it or upwelling to chill it: only the
// wind's own slow drift across the parallels.
func TestAnOpenOceanHasNoGyre(t *testing.T) {
	g := oceanGlobe(256, 128)
	g.weather()
	for i := range g.Tiles {
		// The drift across the parallels carries the fall of warmth with it,
		// and the energy balance's fall at fifty degrees is some eight
		// tenths of a degree a degree - twice the old cosine's: two or so and no more.
		if w := g.SeaWarmth(i); math.Abs(w) > 2.5 {
			t.Fatalf("the open ocean at tile %d stands %+.2f degrees over its latitude", i, w)
		}
	}
}

// The coast beside the cold water of an eastern current is a desert, where the
// coast across the continent from it, on the warm western side of the next
// ocean, is not: the Atacama against Brazil, the Namib against Mozambique.
func TestTheColdCoastIsADesert(t *testing.T) {
	g := twoOceans()
	g.weather()
	still := twoOceans()
	still.weather()
	// The same ground and wind with the sea left the one warmth.
	still.winds.Warm, still.winds.Coast = nil, nil
	still.rainOn()
	for _, lat := range []float64{22, -22} {
		lo, hi := lat-8, lat+8
		cold := band(g, lo, hi, 128, 130, g.Rain)
		warm := band(g, lo, hi, 166, 168, g.Rain)
		was := band(still, lo, hi, 128, 130, still.Rain)
		t.Logf("at %v degrees: the cold coast %.0f mm, the warm coast %.0f mm; the cold coast with no currents %.0f mm", lat, cold, warm, was)
		if cold > warm/2 {
			t.Errorf("at %v degrees the cold coast has %.0f mm against the warm coast's %.0f", lat, cold, warm)
		}
		// The horse latitudes' west coasts are deserts under the sinking air
		// whatever the water does; the cold water keeps them so, and takes a
		// coast that is not one yet a third of the way there.
		if cold > was*2/3 && cold > desertCoast {
			t.Errorf("at %v degrees the currents take the cold coast from %.0f mm only to %.0f", lat, was, cold)
		}
	}
}

// desertCoast is the most rain, mm a year, a coast counts as a desert with:
// the Atacama's and the Namib's coasts have a few millimetres to a few tens.
const desertCoast = 50.0

// The water that crosses an ocean in the westerlies keeps the warmth it
// brought up from the tropics, and the coast it comes ashore on is milder
// than the coast across the continent, which the cold water from the pole runs
// down: Norway against Labrador, British Columbia against Kamchatka.
func TestTheSubpolarWestCoastIsMild(t *testing.T) {
	g := twoOceans()
	g.weather()
	for _, lat := range []float64{58, -58} {
		lo, hi := lat-5, lat+5
		west := band(g, lo, hi, 128, 132, g.CoastWarmth)
		east := band(g, lo, hi, 164, 168, g.CoastWarmth)
		t.Logf("at %v degrees: the continent's west coast %+.1f degrees, its east coast %+.1f", lat, west, east)
		if west < east+1.5 {
			t.Errorf("at %v degrees the west coast is %+.1f degrees and the east %+.1f", lat, west, east)
		}
	}
}

// A valley has no ocean and no currents, and its rain is what it was.
func TestAValleyHasNoCurrents(t *testing.T) {
	g := ridged(300)
	g.weather()
	if g.winds.Warm != nil || g.winds.Coast != nil {
		t.Fatal("a valley has currents")
	}
	for i := range g.Tiles {
		if g.SeaWarmth(i) != 0 || g.CoastWarmth(i) != 0 {
			t.Fatalf("tile %d of a valley is warmed by the sea", i)
		}
	}
}

// Every copy of the sea is the same sea, however the work was dealt out.
func TestTheCurrentsDoNotDependOnTheGoroutines(t *testing.T) {
	sum := func(workers int) float64 {
		was := Workers
		Workers = workers
		defer func() { Workers = was }()
		g := twoOceans()
		g.weather()
		var s float64
		for i, w := range g.winds.Warm {
			s += w*float64(i%89) + g.winds.Coast[i]
		}
		for i, r := range g.rain {
			s += r * float64(i%97)
		}
		return s
	}
	if a, b := sum(1), sum(8); a != b {
		t.Errorf("one goroutine made %v and eight %v", a, b)
	}
}

// A tropical storm lives on warm water, and the water off an ocean's eastern
// shore, where the current comes down from the pole and the cold water comes
// up from under, is too cold to keep one: the eastern Pacific's storms die as
// they come north toward California, where the western Pacific's go on to
// Japan. The same storm over the same water with the currents left out lives.
func TestAStormDiesOverTheColdCurrent(t *testing.T) {
	// A storm is set down on the water a degree off the eastern shore, and
	// six off the western, so that a day of the trades that steer it leaves
	// it over the water off each.
	day := func(lon float64, currents bool) (age, sea, warm float64) {
		wx := weatherOver(twoOceans())
		if !currents {
			wx.Env.Warm, wx.Env.Coast = nil, nil
		}
		wx.Systems = []System{{Kind: Storm, Lat: 18, Lon: lon, Depth: 50, Radius: 200, Life: 100}}
		wx.Step(Year / 4)
		s := wx.Systems[0]
		fx, fy, _ := wx.Env.CellOf(s.Lat, s.Lon)
		return s.Age, wx.Env.Sample(wx.Env.Sea, fx, fy), wx.Env.SeaTemp(fx, fy, atmos.YearSin(Year/4))
	}
	// The first ocean runs from -123.75 degrees to 0.
	cold, coldSea, coldWarm := day(-1, true)
	still, _, stillWarm := day(-1, false)
	warm, warmSea, warmWarm := day(-117.5, true)
	t.Logf("off the eastern shore the sea is %.1f degrees and a storm ages %.0f days in a day; with no currents %.1f and %.0f; off the western shore %.1f and %.0f (a storm needs %.1f)",
		coldWarm, cold, stillWarm, still, warmWarm, warm, atmos.StormSea)
	if coldSea < 0.9 || warmSea < 0.9 {
		t.Fatalf("the storms came down on sea %.2f and %.2f, not open water", coldSea, warmSea)
	}
	if coldWarm >= atmos.StormSea || warmWarm < atmos.StormSea || stillWarm < atmos.StormSea {
		t.Errorf("the sea is %.1f off the eastern shore, %.1f off the western and %.1f with no currents, against the %.1f a storm needs",
			coldWarm, warmWarm, stillWarm, atmos.StormSea)
	}
	if cold <= still || cold <= warm {
		t.Errorf("a storm over the cold current aged %.0f days, over the same water with no currents %.0f, over the warm western water %.0f", cold, still, warm)
	}
}
