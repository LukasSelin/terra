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
// wind's own slow drift across the parallels. The drifts part on the equator
// there too, but with no eastern shore to tilt it against, the thermocline
// lies a hundred and fifty metres down all the way round, and what comes up
// from above it is hardly colder than the surface.
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
	if e := g.winds.Env; e.Cu != nil || e.Cv != nil || e.Rise != nil || e.WaterTemp != nil || e.Psi != nil || e.Thermocline != nil {
		t.Fatal("a valley keeps a current, an upwelling, a thermocline or a sea's temperature")
	}
	for i := range g.Tiles {
		if g.SeaWarmth(i) != 0 || g.CoastWarmth(i) != 0 {
			t.Fatalf("tile %d of a valley is warmed by the sea", i)
		}
		if u, v := g.SeaCurrent(i); u != 0 || v != 0 || g.Upwelling(i) != 0 || g.SeaTemp(i) != 0 || g.Thermocline(i) != 0 {
			t.Fatalf("tile %d of a valley has a current under it", i)
		}
	}
}

// The current, the upwelling and the water's temperature are kept beside the
// warmth they make, and keeping them changes it not at all: the hash is of
// Warm and Coast on twoOceans. It was taken on 53eb8bf, before they were
// kept, and taken again each time the world was moved on purpose: when the
// air came to swing the energy balance's year (see atmos.Env.seasonTemp),
// when its belts came to be placed by the circulation (see
// atmos.Env.beltsAt), both of which move the wind the currents are driven
// by; when the gyres were solved in two dimensions (docs/ocean-model-plan.md,
// M1); when the water that comes up was given the thermocline's depth (M2);
// and on the integration branch, where all four meet. And the warmth is the
// kept temperature over its latitude's mean, held to seaWarmMost, and to
// nothing over it where the water is under ice.
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
	if got, want := h.Sum64(), uint64(0xe032409d6d6e389e); got != want {
		t.Errorf("the sea's warmth hashes to %#x, and was %#x", got, want)
	}
	const most = 10 // atmos.seaWarmMost
	for i, w := range e.Warm {
		if e.Sea[i] <= 0.5 {
			continue
		}
		over := float64(e.WaterTemp[i]) - e.Mean[i/e.W]
		if float64(e.WaterTemp[i]) < SeaFreeze {
			// Under ice: the air over it takes none of the water's warmth.
			over = math.Min(0, over)
		}
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
	// warm water above the thermocline, never less than two hundred metres
	// (atmos.flowLeast).
	gyre := func(i int) float64 {
		depth := math.Max(200, float64(e.Thermocline[i]))
		return float64(e.Psi[i+1]-e.Psi[i-1]) * atmos.Sverdrup / (2 * e.Dx[i/e.W]) / depth
	}
	for _, hemi := range []float64{1, -1} {
		lo, hi := min(20*hemi, 40*hemi), max(20*hemi, 40*hemi)
		west := band(g, lo, hi, 40, 44, north)
		inside := band(g, lo, hi, 70, 110, gyre)
		t.Logf("at 20 to 40 degrees %+v: the western current runs %+.3f m/s north, the interior %+.4f", hemi, west, inside)
		if west*hemi < 0.05 {
			t.Errorf("at 20 to 40 degrees %+v the western current runs %+.3f m/s north", hemi, west)
		}
		if inside*hemi > 0 || math.Abs(inside) > math.Abs(west) {
			t.Errorf("at 20 to 40 degrees %+v the interior runs %+.4f m/s north against the western current's %+.3f", hemi, inside, west)
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
				float64(e.Psi[i])*float64(i%71) + float64(e.Thermocline[i])*float64(i%67)
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
//
// Known gap (A2 x M2): the water six degrees off the western shore at 18°N is
// 26.0, under the 26.5 a storm needs. M2's pumping has the right sign: read
// across the western half of the first ocean, the year's mean Ekman pumping
// is down (-0.03 to -0.14 m/day) from 21.8°N to 38.7°N, under the subtropical
// gyre between the trades and the westerlies, and up from 20.4°N to the
// equator. It is up at 18°N (+0.044 m/day on the year's mean, +0.054 with the
// phases' downwelling dropped) because A2's trades are strongest at 22°N,
// where the earth's are at about 15, so the tropical band of cyclonic curl,
// whose upwelling raises the earth's thermocline ridge at about 10°N, lies at
// 9-20°N here; ψ is negative there, the thermocline rises to 44 m at 17.6°N
// and to its floor, 10 m, at 16°N and below, and what comes up is 16-17°C.
// That makes the water 0.6 under the latitude's mean where the warm western
// water should be. The remedies are A2's - the trades' maximum where the
// earth's is - and the thermocline ridge's depth (M3, #22), not the sign.
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

// The warm water the gyres run in lies over a cold deep, and the wind tilts
// the step between them, the thermocline: the subtropical gyres pile the warm
// water up some hundreds of metres deep in the west of their oceans and the
// subpolar gyres draw it away until the thermocline comes up to the surface
// in theirs, and against an ocean's eastern shore it lies some fifty metres
// down. Along the equator the trades either side of it hold it up in the
// east, and the water is drawn up where their drifts part. On Earth the
// thermocline lies fifty metres down off Peru and two hundred under the warm
// pool, and the water drawn up in the east is the cold tongue (Wyrtki,
// 1981). The year's mean wind here has the doldrums on the equator, with no
// easterlies on it, and the trades either side tilt it only a little: the
// tilt is asserted, and the cold tongue, which the trades on the equator make
// and the Walker circulation over it keeps (Bjerknes, 1969), is a reading
// until the sea and the air are solved together (#28).
func TestTheThermoclineTiltsUnderTheTrades(t *testing.T) {
	g := twoOceans()
	g.weather()
	e := g.winds.Env
	if e.Cell != 1 {
		t.Fatalf("twoOceans has %d tiles to a cell", e.Cell)
	}
	h := func(i int) float64 { return float64(e.Thermocline[i]) }
	day := func(i int) float64 { return float64(e.Rise[i]) * 86400 }
	east := func(i int) float64 { u, _ := g.SeaCurrent(i); return u }
	// The first ocean runs from column 40 to 127.
	westH, eastH := band(g, -3, 3, 40, 62, h), band(g, -3, 3, 106, 128, h)
	westT, eastT := band(g, -3, 3, 40, 62, g.SeaTemp), band(g, -3, 3, 106, 128, g.SeaTemp)
	up, down := band(g, -3, 3, 60, 110, day), band(g, 22, 32, 60, 110, day)
	t.Logf("on the equator the thermocline lies %.0f m down in the west of the ocean and %.0f in the east; the water stands %.2f degrees in the west and %.2f in the east, and comes up %.2f m a day (%.2f under the subtropical high)",
		westH, eastH, westT, eastT, up, down)
	if westH <= eastH {
		t.Errorf("on the equator the thermocline lies %.0f m down in the west and %.0f in the east", westH, eastH)
	}
	if up <= 0 || up <= down {
		t.Errorf("the water comes up %.2f m a day on the equator and %.2f under the subtropical high", up, down)
	}
	for _, hemi := range []float64{1, -1} {
		lo, hi := min(22*hemi, 32*hemi), max(22*hemi, 32*hemi)
		gyre, shore := band(g, lo, hi, 44, 60, h), band(g, lo, hi, 126, 128, h)
		plo, phi := min(50*hemi, 62*hemi), max(50*hemi, 62*hemi)
		polar := band(g, plo, phi, 44, 60, h)
		t.Logf("at %+.0f degrees: the thermocline lies %.0f m down in the west of the subtropical gyre, %.0f against the eastern shore, and %.0f in the west of the subpolar gyre",
			hemi, gyre, shore, polar)
		if gyre < 150 || gyre < shore+100 {
			t.Errorf("at %+.0f degrees the thermocline lies %.0f m down in the subtropical gyre and %.0f against the eastern shore", hemi, gyre, shore)
		}
		if polar > 50 {
			t.Errorf("at %+.0f degrees the thermocline lies %.0f m down in the subpolar gyre", hemi, polar)
		}
	}
	// The current along the equator, and either side of it, where the real
	// oceans have their countercurrents: a reading.
	t.Logf("the surface runs %+.3f m/s east on the equator, %+.3f and %+.3f at 2 to 6 degrees north and south, %+.3f and %+.3f at 6 to 12",
		band(g, -2, 2, 60, 110, east), band(g, 2, 6, 60, 110, east), band(g, -6, -2, 60, 110, east),
		band(g, 6, 12, 60, 110, east), band(g, -12, -6, 60, 110, east))
}
