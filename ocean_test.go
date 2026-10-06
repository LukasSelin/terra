package terra

import (
	"encoding/binary"
	"hash/fnv"
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
	if e := g.winds.Env; e.Cu != nil || e.Cv != nil || e.Rise != nil || e.WaterTemp != nil || e.Psi != nil {
		t.Fatal("a valley keeps a current, an upwelling or a sea's temperature")
	}
	for i := range g.Tiles {
		if g.SeaWarmth(i) != 0 || g.CoastWarmth(i) != 0 {
			t.Fatalf("tile %d of a valley is warmed by the sea", i)
		}
		if u, v := g.SeaCurrent(i); u != 0 || v != 0 || g.Upwelling(i) != 0 || g.SeaTemp(i) != 0 {
			t.Fatalf("tile %d of a valley has a current under it", i)
		}
	}
}

// The current, the upwelling and the water's temperature are kept beside the
// warmth they make, and keeping them changes it not at all: the hash is of
// Warm and Coast on twoOceans. It was taken on 53eb8bf, before they were
// kept, and taken again when the gyres were solved in two dimensions
// (docs/ocean-model-plan.md, M1), which moves them on purpose. And the warmth
// is the kept temperature over its latitude's mean, held to seaWarmMost, and
// nothing where the water is under ice.
func TestKeepingTheCurrentsLeavesTheWarmthAsItWas(t *testing.T) {
	g := twoOceans()
	g.weather()
	e := g.winds.Env
	h := fnv.New64a()
	var b [8]byte
	for _, s := range [][]float64{e.Warm, e.Coast} {
		for _, x := range s {
			binary.LittleEndian.PutUint64(b[:], math.Float64bits(x))
			h.Write(b[:])
		}
	}
	if got, want := h.Sum64(), uint64(0xf475ac6e8056c45f); got != want {
		t.Errorf("the sea's warmth hashes to %#x, and was %#x", got, want)
	}
	const most = 10 // atmos.seaWarmMost
	for i, w := range e.Warm {
		if e.Sea[i] <= 0.5 {
			continue
		}
		if float64(e.WaterTemp[i]) < SeaFreeze {
			// Under ice: the air over it takes nothing from the water.
			if w != 0 {
				t.Fatalf("cell %d: water at %.2f degrees, under ice, warms the air %+.2f", i, e.WaterTemp[i], w)
			}
			continue
		}
		over := float64(e.WaterTemp[i]) - e.Mean[i/e.W]
		if math.Abs(over) >= most {
			over = math.Copysign(most, over)
		}
		// The temperature is kept in single precision: some microdegrees.
		if math.Abs(over-w) > 1e-4 {
			t.Fatalf("cell %d: the water stands %+.6f over its latitude and its warmth is %+.6f", i, over, w)
		}
	}
}

// The water a gyre drives toward the equator across an ocean comes back
// toward the pole in the narrow current against its western shore: north in
// the north, south in the south. The interior drifts the other way, slower.
// The interior is the gyre's own flow, read off its streamfunction: the
// surface water there also drifts with the wind, and under the trades that
// runs toward the pole faster than the gyre's interior runs toward the
// equator, so that the two together come to about nothing between twenty
// degrees and forty.
func TestTheWesternBoundaryCurrentRunsPoleward(t *testing.T) {
	g := twoOceans()
	g.weather()
	e := g.winds.Env
	if e.Cell != 1 {
		t.Fatalf("twoOceans has %d tiles to a cell", e.Cell)
	}
	north := func(i int) float64 { _, v := g.SeaCurrent(i); return v }
	// The gyre's flow toward the north, ψ's rise toward the east over the
	// depth the gyre goes to (atmos.gyreDepth).
	const depth = 300
	gyre := func(i int) float64 {
		return float64(e.Psi[i+1]-e.Psi[i-1]) * atmos.Sverdrup / (2 * e.Dx[i/e.W]) / depth
	}
	for _, hemi := range []float64{1, -1} {
		lo, hi := min(20*hemi, 40*hemi), max(20*hemi, 40*hemi)
		west := band(g, lo, hi, 40, 44, north)
		inside := band(g, lo, hi, 70, 110, gyre)
		t.Logf("at 20 to 40 degrees %+v: the western current runs %+.3f m/s north, the interior %+.3f", hemi, west, inside)
		if west*hemi < 0.05 {
			t.Errorf("at 20 to 40 degrees %+v the western current runs %+.3f m/s north", hemi, west)
		}
		if inside*hemi > 0 || math.Abs(inside) > math.Abs(west) {
			t.Errorf("at 20 to 40 degrees %+v the interior runs %+.3f m/s north against the western current's %+.3f", hemi, inside, west)
		}
	}
	// And the cold coast is where the water comes up.
	up := band(g, 15, 30, 124, 128, g.Upwelling) + band(g, -30, -15, 124, 128, g.Upwelling)
	west := band(g, 15, 30, 40, 44, g.Upwelling) + band(g, -30, -15, 40, 44, g.Upwelling)
	t.Logf("upwelling off the eastern shore %.2g m/s, off the western %.2g", up/2, west/2)
	if up <= west || up <= 0 {
		t.Errorf("the water comes up at %.2g m/s off the eastern shore and %.2g off the western", up/2, west/2)
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
		e := g.winds.Env
		for i, w := range e.Warm {
			s += w*float64(i%89) + e.Coast[i]
			s += float64(e.Cu[i])*float64(i%83) + float64(e.Cv[i])*float64(i%79) +
				float64(e.Rise[i])*1e6 + float64(e.WaterTemp[i])*float64(i%73) +
				float64(e.Psi[i])*float64(i%71)
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

// Every landmass is a shore the water cannot cross, so the gyres' stream-
// function is one level all round it: nought on the largest, and on every
// other the level the island rule gives it.
func TestALandmassHasOneLevel(t *testing.T) {
	g := twoOceans()
	g.weather()
	e := g.winds.Env
	// The two continents are the same size, and the first found, the one
	// over the seam, is the mainland.
	var first, second []float32
	for i, p := range e.Psi {
		if e.Sea[i] > 0.5 {
			continue
		}
		if x := i % e.W; x < 40 {
			first = append(first, p)
		} else {
			second = append(second, p)
		}
	}
	for _, p := range first {
		if p != 0 {
			t.Fatalf("the mainland stands at %v Sv", p)
		}
	}
	for _, p := range second {
		if p != second[0] {
			t.Fatalf("the other continent stands at %v and %v Sv", second[0], p)
		}
	}
	t.Logf("the other continent stands at %+.1f Sv: what goes round the poles between them", second[0])
}

// ringWorld is an ocean globe with a southern continent from the pole to
// sixty-four degrees south, and a continent sixty columns wide from
// forty-four degrees south to seventy north: an ocean all the way round the
// planet between them, under the westerlies, as the Southern Ocean is.
func ringWorld() *Grid {
	g := oceanGlobe(256, 128)
	c := Climate{rows: g.H, globe: true}
	for i := range g.Tiles {
		x, y := i%g.W, i/g.W
		if lat := c.latitude(y); lat < -64 || (lat > -44 && lat < 70 && x < 60) {
			g.Height[i] = 60
		}
	}
	return g
}

// An ocean all the way round the planet has no shore for its water to turn
// back at, and the westerlies drive it round the planet to the east, against
// the friction all the way round: the Antarctic Circumpolar Current, a
// hundred and thirty to a hundred and seventy million cubic metres a second
// through Drake Passage (Donohue and others, 2016). The southern continent's
// level over the northern one's is how much goes round.
func TestASeaAllTheWayRoundCarriesItsCurrentRoundThePlanet(t *testing.T) {
	g := ringWorld()
	g.weather()
	e := g.winds.Env
	south := float64(e.Psi[(e.H-1)*e.W])
	var east, n float64
	for y := 0; y < e.H; y++ {
		if lat := g.air.Lat[y]; lat > -60 && lat < -48 {
			for x := 0; x < e.W; x++ {
				u, _ := g.SeaCurrent(y*g.W + x)
				east, n = east+u, n+1
			}
		}
	}
	east /= n
	t.Logf("%.0f Sv go round the planet; the ring of sea runs %+.2f m/s east", south, east)
	if south < 50 || south > 500 {
		t.Errorf("%.0f Sv go round the planet", south)
	}
	if east < 0.05 {
		t.Errorf("the ring of sea runs %+.2f m/s east", east)
	}
}
