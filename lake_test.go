package terra

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/LukasSelin/terra/geom"
)

// perMM turns a millimetre a year off one tile into cubic metres a second.
var perMM = discharge(1, TileSpan)

// airOf is a hand-made valley's air, with its rain multiplied by wetness.
func airOf(g *Grid, wetness float64) {
	g.air = Climate{rows: g.H}.airFor(g, wetness)
}

// bowl is a valley map with one hollow in the middle of it: a floor at four
// metres rising to a ring of ridge at twelve, broken on the east by a notch
// whose lowest point is nine and a half metres, and falling away outside the
// ring to the edges of the map, which is where the water leaves. Its rain is
// the valley's times wetness.
func bowl(wetness float64) *Grid {
	g := NewGrid(25, 25)
	airOf(g, wetness)
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			r := math.Hypot(float64(x-12), float64(y-12))
			h := 4 + r
			switch {
			case r >= 6 && r < 7.5:
				h = 12
			case r >= 7.5:
				h = math.Max(0.5, 12-(r-7.5))
			}
			// Millimetres of unevenness, so that no two tiles of the floor
			// stand at quite the same height; the notch is left exact.
			h += 1e-3 * float64((x*7+y*13)%11)
			if y == 12 && x >= 17 {
				h = math.Min(h, 9.5-0.5*float64(x-18))
			}
			g.At(geom.Pos{X: x, Y: y}).Height = h
		}
	}
	return g
}

// settle works the water out on a hand-made map and names the ground by it.
func settle(g *Grid) {
	g.drain()
	g.carve(rand.New(rand.NewPCG(1, 2)))
	g.height()
}

// A hollow in wet country fills to the lowest point of its rim and spills
// over it: a lake, open, standing at exactly the height of the notch, with the
// whole of its water going out of the one tile it leaves by.
func TestAHollowInWetCountryFillsToItsRim(t *testing.T) {
	g := bowl(1)
	was := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		was[i] = g.Tiles[i].Height
	}
	settle(g)
	for i := range g.Tiles {
		if g.Tiles[i].Height != was[i] {
			t.Fatalf("working the water out moved the ground at %v from %v to %v", g.PosOf(i), was[i], g.Tiles[i].Height)
		}
	}
	centre := geom.Pos{X: 12, Y: 12}
	l, ok := g.LakeAt(centre)
	if !ok {
		t.Fatal("the hollow holds no lake")
	}
	if l.Closed || l.Level != 9.5 {
		t.Fatalf("the lake stands at %v, closed %v; want it open at the notch, 9.5", l.Level, l.Closed)
	}
	for i := range g.Tiles {
		p := g.PosOf(i)
		inside := math.Hypot(float64(p.X-12), float64(p.Y-12)) < 6
		under := g.lakeOf[i] >= 0
		if inside && g.Tiles[i].Height < 9.5 && !under {
			t.Errorf("%v lies below the water and is not in the lake", p)
		}
		if under && g.Tiles[i].Terrain != Water {
			t.Errorf("%v is in an open lake and is %v", p, g.Tiles[i].Terrain)
		}
		if under {
			if q, ok := g.Downstream(p); !ok || g.Index(q) != int(l.Outlet) {
				t.Fatalf("the water at %v goes to %v and not to the lake's outlet", p, q)
			}
		}
	}
	// What the lake passes on is what reached it, less what its surface gave
	// the air.
	given := 0.0
	for i := range g.Tiles {
		if g.lakeOf[i] >= 0 {
			given += g.loss(i) * perMM
		}
	}
	out := g.Tiles[l.Outlet].Flow
	want := l.Inflow - given
	if want <= 0 || out < want-1e-9 {
		t.Fatalf("the outlet carries %.4f m3/s; the lake was given %.4f and its surface took %.4f", out, l.Inflow, given)
	}
	if o := g.PosOf(int(l.Outlet)); o.Y != 12 || o.X < 16 {
		t.Fatalf("the lake leaves by %v, which is not the notch", o)
	}
}

// The same hollow in dry country does not fill. It stands where what its
// surface gives the air is what runs into it, below the notch, with nothing
// going out, and what it leaves behind is salt.
func TestAHollowInDryCountryStopsWhereTheAirTakesItsWater(t *testing.T) {
	g := bowl(dryBowl)
	settle(g)
	l, ok := g.LakeAt(geom.Pos{X: 12, Y: 12})
	if !ok {
		t.Fatal("the hollow holds no lake")
	}
	if !l.Closed || l.Level >= 9.5 {
		t.Fatalf("the lake stands at %v, closed %v; want it closed, below the notch", l.Level, l.Closed)
	}
	if l.Outlet >= 0 {
		t.Fatalf("a closed lake has an outlet at %v", g.PosOf(int(l.Outlet)))
	}
	// Balanced: what runs into the hollow against what its surface gives
	// back, to within one tile's worth.
	var in, given, most float64
	for i := range g.Tiles {
		loss := g.loss(i)
		most = math.Max(most, loss)
		if g.lakeOf[i] >= 0 {
			given += loss
		}
		p := g.PosOf(i)
		for {
			q, ok := g.Downstream(p)
			if !ok {
				break
			}
			p = q
		}
		if j := g.Index(p); g.lakeOf[j] >= 0 || g.pans[j] {
			in += g.runoff[i]
		}
	}
	if math.Abs(in-given) > most {
		t.Fatalf("%.0f runs into the lake and its surface gives back %.0f; want them within %.0f", in, given, most)
	}
	salt, pan := 0, 0
	for i := range g.Tiles {
		switch g.Tiles[i].Terrain {
		case Salt:
			salt++
		case Pan:
			pan++
		}
	}
	if salt != l.Tiles || salt == 0 {
		t.Fatalf("%d tiles of salt lake against %d under the lake", salt, l.Tiles)
	}
	if pan == 0 {
		t.Fatal("a salt lake with no salt flat round it")
	}
}

// And in country where nothing runs off at all, the hollow is dry: a salt
// flat, and no water standing anywhere in it. (A map still gets its share of
// watercourse wherever the ground would cut one - see carve - so this asks
// only after the hollow.)
func TestAHollowWithNothingRunningIntoItIsASaltFlat(t *testing.T) {
	g := bowl(1e-9)
	settle(g)
	centre := g.Index(geom.Pos{X: 12, Y: 12})
	if len(g.Lakes) != 1 || g.Lakes[0].Tiles != 0 || !g.Lakes[0].Closed {
		t.Fatalf("the lakes on a map nothing runs on are %+v; want one, dry", g.Lakes)
	}
	for i := range g.Tiles {
		if g.lakeOf[i] >= 0 || g.Tiles[i].Terrain == Salt {
			t.Fatalf("water stands at %v in a hollow nothing runs into", g.PosOf(i))
		}
	}
	if g.Tiles[centre].Terrain != Pan {
		t.Fatalf("the floor of a dry hollow is %v", g.Tiles[centre].Terrain)
	}
}

// steps is a trough running east to the edge of the map with two hollows in
// it: an upper one whose floor is at eight metres and whose lip, at ten, lets
// it out into a lower one, floored at two, which spills at six over a sill
// down to the edge.
func steps(wetness float64) *Grid {
	g := NewGrid(30, 11)
	airOf(g, wetness)
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			var h float64
			switch {
			case x < 10:
				h = 8 + 0.4*math.Abs(float64(x-4))
				if x == 0 {
					h = 30
				}
			case x < 25:
				h = 2 + 0.5*math.Abs(float64(x-17))
			default:
				h = 6 - float64(x-25)
			}
			// Millimetres of unevenness on the floors, so that no two tiles
			// of either stand at quite the same height.
			if x != 9 && x != 25 {
				h += 1e-3 * float64((x*7+y*13)%11)
			}
			if y < 3 || y > 7 {
				h = 30
			}
			g.At(geom.Pos{X: x, Y: y}).Height = h
		}
	}
	return g
}

// Two hollows, one above the other. The upper spills into the lower, and the
// lower fills with its own water and the upper's before it goes on.
func TestAnUpperLakeSpillsIntoTheOneBelowIt(t *testing.T) {
	g := steps(1)
	settle(g)
	upper, ok := g.LakeAt(geom.Pos{X: 4, Y: 5})
	if !ok {
		t.Fatal("the upper hollow holds no lake")
	}
	lower, ok := g.LakeAt(geom.Pos{X: 17, Y: 5})
	if !ok {
		t.Fatal("the lower hollow holds no lake")
	}
	if upper.Closed || upper.Level != 10 || lower.Closed || lower.Level != 6 {
		t.Fatalf("upper at %v closed %v, lower at %v closed %v; want both open at 10 and 6",
			upper.Level, upper.Closed, lower.Level, lower.Closed)
	}
	p := g.PosOf(int(upper.Outlet))
	for {
		q, ok := g.Downstream(p)
		if !ok {
			break
		}
		if l, in := g.LakeAt(q); in {
			if l != lower {
				t.Fatalf("the upper lake's water reaches a lake at %v, not the lower one", l.Level)
			}
			break
		}
		p = q
	}
	if lower.Inflow <= upper.Inflow {
		t.Fatalf("the lower lake gets %.4f and the upper %.4f; the lower has the upper's water and its own", lower.Inflow, upper.Inflow)
	}
}

// The water does not settle whose side a hollow is on by what it happened to
// run into. The lower hollow here is far too big for what falls on it to keep
// it full in dry country, and the upper one is small; what the upper one gets
// that it cannot keep has to go into the lower one and be counted there, or
// the lower one comes out drier than the water reaching it says.
func TestOverflowIsCountedWhereItLands(t *testing.T) {
	// At this dryness the upper hollow runs over and the lower one does not.
	g := steps(dryStep)
	settle(g)
	lower, ok := g.LakeAt(geom.Pos{X: 17, Y: 5})
	upper, uok := g.LakeAt(geom.Pos{X: 4, Y: 5})
	if !ok || !uok || !lower.Closed || upper.Closed {
		t.Fatalf("want the upper lake open and the lower one closed; lakes are %+v", g.Lakes)
	}
	// What its surface gives back is everything that reaches it, the upper
	// lake's overflow included.
	var given, most float64
	k := g.lakeOf[g.Index(geom.Pos{X: 17, Y: 5})]
	for i := range g.Tiles {
		loss := g.loss(i)
		most = math.Max(most, loss)
		if g.lakeOf[i] == k {
			given += loss
		}
	}
	var in float64
	for i := range g.Tiles {
		p := g.PosOf(i)
		for {
			q, ok := g.Downstream(p)
			if !ok {
				break
			}
			p = q
		}
		// The lower hollow is the ground from ten to twenty-four across.
		if j := g.Index(p); g.lakeOf[j] == k || (g.pans[j] && p.X >= 10 && p.X < 25) {
			in += g.runoff[i]
		}
	}
	// The upper lake's surface gave some of what it was given to the air.
	for i := range g.Tiles {
		if l, ok := g.LakeAt(g.PosOf(i)); ok && l == upper {
			in -= g.loss(i)
		}
	}
	// Within two tiles: a level is placed to the nearest tile, and no two
	// tiles give the air quite the same. Counted as lost at the saddle
	// instead, the upper lake's overflow left the lower one short by twelve.
	if math.Abs(in-given) > 2*most {
		t.Fatalf("%.0f reaches the lower lake and its surface gives back %.0f; want them within %.0f", in, given, 2*most)
	}
}

// A globe has its deserts where the air comes down dry: a salt lake stands
// only where the air takes more off open water than falls.
//
// Over three globes and not one. Whether a given globe has a hollow in a
// desert is a matter of where its history put its ranges: with the history
// on a clock of millions of years, the second small globe has none, and the
// first has twenty-two tiles of salt lake and the seventh 272.
func TestSaltLakesStandInDryCountry(t *testing.T) {
	closed := 0
	for _, seed := range []uint64{1, 2, 3} {
		g := NewLand(seed, smallGlobe()).Grid
		for i := range g.Tiles {
			if !g.closedLake(i) {
				continue
			}
			closed++
			// The air could take up more than falls where what it takes off
			// open water is more than what runs off the ground.
			if g.loss(i) <= g.runoff[i] {
				t.Fatalf("a salt lake at %v, where %.0f falls and %.0f runs off", g.PosOf(i), g.rain[i], g.runoff[i])
			}
		}
	}
	if closed == 0 {
		t.Fatal("three globes with dry belts on them have no salt lake anywhere")
	}
	// And the default valley, at the temperate latitude, has none at all.
	for _, seed := range []uint64{1, 2, 3} {
		v := NewLand(seed, DefaultTerms()).Grid
		for i := range v.Tiles {
			if k := v.Tiles[i].Terrain; k == Salt || k == Pan {
				t.Fatalf("seed %d: a temperate valley has %v at %v", seed, k, v.PosOf(i))
			}
		}
	}
}

// dryBowl, dryStep and dryValley are how much of a temperate valley's rain the
// dry cases below are given.
const (
	dryBowl   = 0.4
	dryStep   = 0.7
	dryValley = 0.3
)

// Asked to be dry, a valley keeps its water: some of its hollows hold salt
// lakes, or salt flats, where the same valley in its latitude's rain held
// lakes that ran on to the edge.
func TestADryValleyKeepsItsWater(t *testing.T) {
	cfg := DefaultTerms()
	cfg.Wetness = dryValley
	for _, seed := range []uint64{1, 2, 3} {
		g := NewLand(seed, cfg).Grid
		if g.Count(func(t *Tile) bool { return t.Terrain == Salt || t.Terrain == Pan }) == 0 {
			t.Errorf("seed %d: a dry valley with no salt anywhere on it", seed)
		}
		drainsSomewhere(t, g)
	}
}

// The same seed makes the same lakes.
func TestTheSameSeedFillsTheSameLakes(t *testing.T) {
	cfg := DefaultTerms()
	cfg.Wetness = dryValley
	a, b := NewLand(4, cfg).Grid, NewLand(4, cfg).Grid
	if len(a.Lakes) != len(b.Lakes) {
		t.Fatalf("%d lakes one time and %d the next", len(a.Lakes), len(b.Lakes))
	}
	for k := range a.Lakes {
		if a.Lakes[k] != b.Lakes[k] {
			t.Fatalf("lake %d is %+v one time and %+v the next", k, a.Lakes[k], b.Lakes[k])
		}
	}
	for i := range a.Tiles {
		if a.Tiles[i].Flow != b.Tiles[i].Flow || a.down[i] != b.down[i] {
			t.Fatalf("the water at tile %d went two ways", i)
		}
	}
}
