package terra

import (
	"math"
	"testing"

	"github.com/LukasSelin/terra/clock"
	"github.com/LukasSelin/terra/geom"
)

// roundOver is the curvature over tile i as the creep reads it: the height of
// its neighbours over it, a diagonal counting half. Negative on a crest.
func roundOver(g *Grid, i int) float64 {
	p := g.PosOf(i)
	round := 0.0
	for _, off := range Dirs {
		q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
		if !g.In(q) {
			continue
		}
		near := 1.0
		if off.X != 0 && off.Y != 0 {
			near = 0.5
		}
		round += near * (g.Height[g.Index(q)] - g.Height[i])
	}
	return round
}

// Soil is a skin on the crests and the steep ground, where the ground is being
// taken away as fast as the rock can make it, and deep in the hollows the creep
// fills: Heimsath and others (1997) measured exactly that pattern down the
// hillslopes it fitted the production function to. On a fresh map, and still
// after the ages have moved it.
func TestSoilIsThinOnCrestsAndDeepInHollows(t *testing.T) {
	check := func(name string, g *Grid) {
		t.Helper()
		var crest, hollow, steep, gentle float64
		var nc, nh, ns, ng float64
		for i := range g.Tiles {
			tl := &g.Tiles[i]
			if tl.Wet() || tl.Terrain.Tidal() || tl.Mark != None {
				continue
			}
			h := float64(g.Soil[i])
			switch round := roundOver(g, i); {
			case round < -2:
				crest, nc = crest+h, nc+1
			case round > 2:
				hollow, nh = hollow+h, nh+1
			}
			switch s := g.Slope(g.PosOf(i)); {
			case s > 0.5:
				steep, ns = steep+h, ns+1
			case s < 0.1:
				gentle, ng = gentle+h, ng+1
			}
		}
		if nc == 0 || nh == 0 || ns == 0 || ng == 0 {
			t.Fatalf("%s: %v crests, %v hollows, %v steep, %v gentle tiles", name, nc, nh, ns, ng)
		}
		crest, hollow, steep, gentle = crest/nc, hollow/nh, steep/ns, gentle/ng
		if !(crest < hollow/2) {
			t.Errorf("%s: crests hold %.2f m of soil and hollows %.2f; want the hollows twice as deep at least", name, crest, hollow)
		}
		if !(steep < gentle/2) {
			t.Errorf("%s: steep ground holds %.2f m and gentle %.2f; want the gentle twice as deep at least", name, steep, gentle)
		}
		if !(hollow > 0.5 && hollow < 3) {
			t.Errorf("%s: hollows hold %.2f m; real soil-mantled hollows hold a metre or two", name, hollow)
		}
	}
	for seed := uint64(1); seed <= 3; seed++ {
		w := NewLand(seed, DefaultTerms())
		check("fresh", w.Grid)
		for age := 0; age < 20; age++ {
			w.Erode()
		}
		check("twenty ages on", w.Grid)
	}
}

// ridge is a wrapped grid a few tiles round with a ridge running round it:
// every row a line of equal heights, the first and last rows its feet, open
// grass everywhere with depth of soil on it.
func ridge(soil float32) *Grid {
	g := NewGrid(4, 21)
	g.Wrap = true
	for i := range g.Tiles {
		t := &g.Tiles[i]
		t.Terrain, g.Soil[i], g.Sand[i], g.Clay[i] = Grass, soil, 0.3, 0.3
	}
	return g
}

// Roering and others (2007): where a hillslope is lowering at E with soil
// creeping over it, the crest is rounded to a curvature of −E/D, so how sharp
// the hilltops are is a reading of how fast the ground is going. Held here on
// a ridge raised at a steady rate against feet held still, with the creep the
// only thing moving it, run to a steady state: the crest's curvature against
// what that says. And with twice the soil, half the curvature, since the soil
// carries the flux in proportion to how much of it there is (Johnstone and
// Hilley 2015).
func TestACrestIsAsRoundAsItsLoweringOverItsDiffusivity(t *testing.T) {
	const lift = 2e-6             // m/yr: a crest gentle enough that its flanks creep near linearly
	const years = 1000 * ageYears // a step: the creep is taken implicitly
	crest := func(soil float32) float64 {
		g := ridge(soil)
		n := len(g.Tiles)
		for step := 0; step < 4000; step++ {
			change := make([]float64, n)
			gained := make([][Grains]float64, n)
			lost := make([]float64, n)
			g.creep(years, change, gained, lost)
			for i := range g.Tiles {
				if y := i / g.W; y == 0 || y == g.H-1 {
					continue // the feet, which the rivers at them hold where they are
				}
				g.Height[i] += change[i] + lift*years
			}
		}
		top := g.H / 2 * g.W
		above, below := g.Height[top-g.W], g.Height[top+g.W]
		return (above - 2*g.Height[top] + below) / (TileSpan * TileSpan)
	}
	// D at SoilScale of soil: see Diffusivity.
	d := Diffusivity * Grass.Hold()
	want := -lift / d
	if got := crest(SoilScale); math.Abs(got-want) > 0.03*math.Abs(want) {
		t.Errorf("under %.1f m of soil the crest is curved %.3g a metre; lowering over diffusivity says %.3g", SoilScale, got, want)
	}
	if got := crest(2 * SoilScale); math.Abs(got-want/2) > 0.03*math.Abs(want/2) {
		t.Errorf("under %.1f m of soil the crest is curved %.3g a metre; want half of %.3g", 2*SoilScale, got, want)
	}
}

// The creep moves ground and makes none, and no tile gives more soil than it
// has: the rock under it does not creep.
func TestTheCreepMovesOnlySoilItHas(t *testing.T) {
	g := ridge(0.01)
	for i := range g.Tiles {
		y := float64(i/g.W) - 10
		g.Height[i] = 400 - 0.4*TileSpan*math.Abs(y) // a sharp crest with steep flanks
	}
	n := len(g.Tiles)
	change := make([]float64, n)
	gained := make([][Grains]float64, n)
	lost := make([]float64, n)
	g.creep(5000, change, gained, lost)
	sum, moved := 0.0, 0.0
	for i := range change {
		sum += change[i]
		moved += math.Abs(change[i])
		if lost[i] > float64(g.Soil[i])*(1+1e-9) {
			t.Errorf("tile %d gave %.4f m of soil and had %.4f", i, lost[i], g.Soil[i])
		}
	}
	if moved == 0 {
		t.Fatal("nothing crept")
	}
	if math.Abs(sum) > 1e-12*moved {
		t.Errorf("the creep moved %.6f m about and made %.3g", moved, sum)
	}
}

// Ground that fails comes down onto the ground below it; none of it is lost.
// Held on a made valley with cliffs cut into it and on a made history's small
// globe, to the rounding of the sum: and after it, nothing stands steeper than
// ground can.
func TestLandslidesConserveTheGround(t *testing.T) {
	try := func(name string, g *Grid) {
		t.Helper()
		before, scale, soil := 0.0, 0.0, 0.0
		for i := range g.Tiles {
			before += g.Height[i]
			scale += math.Abs(g.Height[i])
			soil += float64(g.Soil[i])
		}
		g.landslide(true)
		after, soilAfter := 0.0, 0.0
		for i := range g.Tiles {
			after += g.Height[i]
			soilAfter += float64(g.Soil[i])
		}
		if math.Abs(after-before) > 1e-12*scale {
			t.Errorf("%s: the ground came to %.9g before the slides and %.9g after", name, before, after)
		}
		if soilAfter < soil*(1-1e-6) {
			t.Errorf("%s: the soil came to %.6g before the slides and %.6g after; what fails only ever becomes soil", name, soil, soilAfter)
		}
		for i := range g.Tiles {
			p := g.PosOf(i)
			for _, off := range Dirs {
				q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
				// The deep sea floor is none of the slides' business. See abyssal.
				if !g.In(q) || g.abyssal(i) || g.abyssal(g.Index(q)) {
					continue
				}
				run := TileSpan
				if off.X != 0 && off.Y != 0 {
					run *= math.Sqrt2
				}
				if fall := (g.Height[i] - g.Height[g.Index(q)]) / run; fall > standMost*Critical+slideLeast/run+1e-9 {
					t.Fatalf("%s: tile %v stands %.3f over %v after the slides", name, p, fall, q)
				}
			}
		}
	}
	w := NewLand(4, DefaultTerms())
	g := w.Grid
	for i := range g.Tiles {
		if p := g.PosOf(i); p.X%9 == 4 && p.Y%5 == 2 {
			g.Height[i] += 300 // a pillar no ground stands as
		}
	}
	try("valley", g)
	globe := madeLand(3, smallGlobe()).Grid
	for i := range globe.Tiles {
		globe.Height[i] *= 3 // the ranges stood up three times as steep
	}
	try("small globe", globe)
}

// The water takes the soil at the soil's rate and the rock under it at the
// rock's: a reach whose rock the water cannot cut loses its soil and no more.
func TestTheWaterTakesTheSoilBeforeTheRock(t *testing.T) {
	c, _ := chain(40, Erodibility, 50)
	n := len(c.h)
	c.soil, c.rock, c.eff = make([]float64, n), make([]float64, n), make([]float64, n)
	for i := range c.soil {
		c.soil[i] = 0.05
	}
	next := c.solve(settleIters)
	for i := 1; i < n; i++ {
		if cut := c.cutAt(next, int32(i)); cut > c.soil[i]*(1+1e-6)+1e-9 {
			t.Fatalf("tile %d lost %.4f m with %.4f of soil over rock nothing cuts", i, cut, c.soil[i])
		}
	}
	// With the rock as easy to cut as the soil it is Braun and Willett again.
	d, _ := chain(40, Erodibility, 50)
	d.soil, d.rock, d.eff = make([]float64, n), append([]float64(nil), d.f...), make([]float64, n)
	plain, _ := chain(40, Erodibility, 50)
	a, b := d.solve(settleIters), plain.solve(settleIters)
	for i := range a {
		if math.Abs(a[i]-b[i]) > 1e-12*math.Max(1, b[i]) {
			t.Fatalf("tile %d: %v over soil on rock of the same rate, %v over one ground", i, a[i], b[i])
		}
	}
}

// Making soil over an age at once is making it a year at a time.
func TestSoilIsMadeTheSameInOneGoAsInMany(t *testing.T) {
	for _, h := range []float64{0, 0.1, 0.8, 3} {
		once := soilMade(h, 1000, SoilMaking)
		many := h
		for y := 0; y < 1000; y++ {
			many = soilMade(many, 1, SoilMaking)
		}
		if math.Abs(once-many) > 1e-9 {
			t.Errorf("from %.1f m: %.9f in one go and %.9f a year at a time", h, once, many)
		}
		if once <= h {
			t.Errorf("from %.1f m a thousand years made nothing", h)
		}
	}
}

// Clay comes with the weathering: the same rock under a warmer, wetter sky
// leaves a heavier soil, and the three shares still make one.
func TestWarmerWetterGroundWeathersToClay(t *testing.T) {
	for _, b := range Bedrocks() {
		r := weathers[b]
		_, coldClay := weathered(r.sand, r.clay, 0.3)
		_, warmClay := weathered(r.sand, r.clay, 3)
		if !(coldClay < r.clay && r.clay < warmClay) {
			t.Errorf("%v: clay %.2f cold, %.2f middling, %.2f warm and wet", b, coldClay, r.clay, warmClay)
		}
		for _, w := range []float64{0.01, 0.3, 1, 3, weatherMost} {
			s, c := weathered(r.sand, r.clay, w)
			if s < 0 || c < 0 || s+c > 1+1e-12 {
				t.Errorf("%v at %v: sand %v clay %v", b, w, s, c)
			}
		}
		if sand, _ := weathered(r.sand, r.clay, 1); math.Abs(sand-r.sand) > 1e-12 {
			t.Errorf("%v: the middling weathering moves the sand from %v to %v", b, r.sand, sand)
		}
	}
}

// A slope that faces the equator is the sunny one: south in the north, north in
// the south.
func TestTheSunnySideFacesTheEquator(t *testing.T) {
	g := NewGrid(64, 60)
	g.Wrap = true
	g.air = Climate{rows: g.H, globe: true}.airFor(g, 1)
	face := func(y int, southward bool) float64 {
		for i := range g.Tiles {
			ty := float64(i / g.W)
			g.Height[i] = 1000 + 8*ty
			if southward {
				g.Height[i] = 1000 - 8*ty
			}
		}
		return g.Sunlight(geom.Pos{X: 10, Y: y})
	}
	north, south := 10, 50
	if !(g.air.Lat[north] > tropic && g.air.Lat[south] < -tropic) {
		t.Fatalf("rows %d and %d are at %.0f and %.0f", north, south, g.air.Lat[north], g.air.Lat[south])
	}
	if !(face(north, true) > face(north, false)) {
		t.Error("in the north a south-facing slope is not the sunnier")
	}
	if !(face(south, false) > face(south, true)) {
		t.Error("in the south a north-facing slope is not the sunnier")
	}
}

// Ground asleep for a season and caught up in one go comes to what the days
// would have made of it, every stand, shoal, sward and worn field, to a
// millionth of a millionth: the fillings are closed forms, so how the growing
// weather is cut up does not matter. Both kinds of stand are in it - one whose
// stock comes back faster than the stand comes on, and one slower - and stocks
// over their ceiling, at nothing, and part full.
func TestCatchingUpIsTheDaysTakenAtOnce(t *testing.T) {
	wood := func(g *Grid) []float64 { return g.Wood }
	wild := func(g *Grid) []float64 { return g.Wild }
	was := growth[None][Forest]
	defer SetGrowth(None, Forest, was)
	SetGrowth(None, Forest, []Growth{
		{Full: 700, Rate: 0.003, Stock: wild},   // comes back faster than the stand comes on
		{Full: 2000, Rate: 0.0002, Stock: wood}, // and slower
	})

	g := NewGrid(64, 8)
	kinds := []Terrain{Forest, Water, Field, Grass}
	for i := range g.Tiles {
		t := &g.Tiles[i]
		t.Terrain = kinds[i%len(kinds)]
		g.Age[i] = float64(i%23) * 120
		g.Wood[i] = float64(i%11) / 10 // 0 to 1, some over what a young stand carries
		g.Wild[i] = float64(i%7) / 6
		g.Fish[i] = float64(i%5) / 5
		g.Sward[i] = float64(i%9) / 9
		g.Rich[i] = 0.3 + 0.1*float64(i%6)
		g.Fertility[i] = g.Rich[i] * float64(i%4) / 4
	}
	g.Rekind()

	byDay, atOnce, byTile := g.Clone(), g.Clone(), g.Clone()
	n := len(g.Tiles)
	total := 0.0
	for day := 0; day < clock.Season; day++ {
		k := 0.4 + 0.8*math.Abs(math.Sin(float64(day)/9)) // the weather comes and goes
		if day%17 == 0 {
			k = 0 // and some days grow nothing
		}
		total += k
		byDay.Grow(0, n, k)
		for i := 0; i < n; i++ {
			byTile.Ripen(i, k)
			byTile.Replenish(i, k)
		}
	}
	atOnce.Grow(0, n, total)
	same(t, "the day's pass and the tile's", byDay, byTile)

	layers := []struct {
		name        string
		day, asleep []float64
	}{
		{"age", byDay.Age, atOnce.Age}, {"wood", byDay.Wood, atOnce.Wood}, {"wild", byDay.Wild, atOnce.Wild},
		{"fish", byDay.Fish, atOnce.Fish}, {"sward", byDay.Sward, atOnce.Sward}, {"fertility", byDay.Fertility, atOnce.Fertility},
	}
	for _, l := range layers {
		changed := false
		for i := range l.day {
			if math.Abs(l.day[i]-l.asleep[i]) > 1e-12*math.Max(1, math.Abs(l.day[i])) {
				t.Errorf("%s on tile %d: %.17g by the day and %.17g caught up", l.name, i, l.day[i], l.asleep[i])
				break
			}
			changed = changed || l.day[i] != g.Layers.Read(i).field(l.name)
		}
		if !changed {
			t.Errorf("%s: a season changed nothing", l.name)
		}
	}
}

// field is one reading by the name the test above gives it.
func (r Readings) field(name string) float64 {
	switch name {
	case "age":
		return r.Age
	case "wood":
		return r.Wood
	case "wild":
		return r.Wild
	case "fish":
		return r.Fish
	case "sward":
		return r.Sward
	case "fertility":
		return r.Fertility
	}
	return math.NaN()
}
