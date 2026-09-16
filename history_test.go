package terra

import (
	"github.com/LukasSelin/terra/geom"
	"sort"
	"testing"
)

// historyConfig is the default valley made out of its own history rather than
// drawn, which is the comparison every test here is about.
func historyConfig(epochs int) Terms {
	cfg := DefaultTerms()
	cfg.Epochs = epochs
	return cfg
}

// shape is the few numbers that say what kind of map something is: how much
// of it can be ploughed, how much is water, and how the ground lies.
type shape struct {
	open, wet int
	slope50   float64
	slope90   float64
}

func shapeOf(g *Grid) shape {
	var s shape
	slopes := make([]float64, 0, len(g.Tiles))
	for i := range g.Tiles {
		p := geom.Pos{X: i % g.W, Y: i / g.W}
		slopes = append(slopes, g.Slope(p))
		if g.Tiles[i].Buildable() {
			s.open++
		}
		if g.Tiles[i].Wet() {
			s.wet++
		}
	}
	s.slope50, s.slope90 = quantile(slopes, 0.5), quantile(slopes, 0.9)
	return s
}

// The whole settlement model is tuned against maps the drawn generator makes:
// how much of one can be ploughed, how steep the steep part of it is, how far
// a person walks uphill to get anywhere. A history has no reason to land on
// any of that and every reason not to, so normalise makes it - and this is
// the test of that join rather than of the history.
//
// The bands are wide on purpose. What is being asked is not that a made world
// is a drawn one, which would make the whole thing pointless, but that it is
// the same kind of place: a valley somebody could live in, with mountains at
// the edges of it rather than through the middle of everything.
func TestAHistoryLeavesAMapTheSettlementCanUse(t *testing.T) {
	var steep []float64
	const seeds = 12
	for seed := uint64(1); seed <= seeds; seed++ {
		drawn := shapeOf(NewLand(seed, DefaultTerms()).Grid)
		made := shapeOf(NewLand(seed, historyConfig(16)).Grid)
		steep = append(steep, made.slope90/drawn.slope90)

		if made.open < drawn.open*8/10 {
			t.Errorf("seed %d: %d tiles can be ploughed on a made world against %d on a drawn one",
				seed, made.open, drawn.open)
		}
		if made.wet > drawn.wet*3 {
			t.Errorf("seed %d: %d tiles of water against %d", seed, made.wet, drawn.wet)
		}
	}
	// The steepest tenth is what decides whether a map is country or a set of
	// walls, and it is the reading that caught every wrong turn this generator
	// took: plate settling flattening the interiors, seams raised as knife
	// edges, and a normalise that squeezed the lowland while leaving the
	// mountains alone.
	//
	// It is asked of the middle of a dozen seeds and not of each one, because
	// one seed says almost nothing. Over thirty of them the made world runs
	// from 0.63 times its drawn twin to 5.12, with the middle at 1.42 - so a
	// bar on a single seed is a bar on the draw, and the three seeds this
	// asked before were three that happened to clear it. It failed the moment
	// anything shifted what the world's own luck handed out, which is what any
	// change to the ground does.
	//
	// Two and a half on the middle, which is most of a doubling of room. That
	// a made valley is the steeper place is true and is worth tightening - but
	// by making gentler ground, not by moving this line, and the number to
	// tighten toward is the 1.42.
	sort.Float64s(steep)
	if mid := steep[len(steep)/2]; mid > 2.5 {
		t.Errorf("the middling made world has a steepest tenth %.2f times its drawn twin's, over %d seeds",
			mid, seeds)
	}
}

// Basalt and schist are the rocks that have to happen to a place: one comes
// up and the other is buried and squeezed, and no lattice that knows only
// where a tile is can lay either. A history that makes neither has not made a
// history.
func TestAHistoryMakesTheRocksThatHaveToHappen(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3} {
		w := madeLand(seed, historyConfig(16))
		var seen [BedrockCount]int
		for i := range w.Grid.Tiles {
			seen[w.Grid.Tiles[i].Bedrock]++
		}
		for _, b := range []Bedrock{Basalt, Schist} {
			if seen[b] == 0 {
				t.Errorf("seed %d made no %s", seed, b)
			}
		}
		kinds := 0
		for _, b := range Bedrocks() {
			if seen[b] > 0 {
				kinds++
			}
		}
		if kinds < 4 {
			t.Errorf("seed %d came out with %d kinds of rock on it", seed, kinds)
		}
	}
}

// A rock knows when it was made, and not all of it was made at once. If every
// tile dated from the same epoch there would be no history in the record,
// only in the making of it.
func TestRockIsDatedToWhenItWasMade(t *testing.T) {
	w := madeLand(1, historyConfig(16))
	seen := map[uint8]int{}
	for i := range w.Grid.Tiles {
		seen[w.Grid.Tiles[i].Formed]++
	}
	if len(seen) < 3 {
		t.Errorf("the whole map dates from %d epochs: %v", len(seen), seen)
	}
}

// Every tile rides a plate, and a history that ran at all has more than one.
func TestEveryTileRidesAPlate(t *testing.T) {
	w := madeLand(1, historyConfig(16))
	seen := map[uint8]bool{}
	for i := range w.Grid.Tiles {
		seen[w.Grid.Tiles[i].Plate] = true
	}
	if len(seen) < 3 {
		t.Errorf("the map broke into %d plates", len(seen))
	}
}

// A history is drawn from the world's own luck and nothing else, so the same
// seed has to give the same world - the whole model is deterministic and a
// generator that was not would take that away from it.
func TestTheSameSeedRunsTheSameHistory(t *testing.T) {
	a := NewLand(7, historyConfig(12)).Grid
	b := NewLand(7, historyConfig(12)).Grid
	for i := range a.Tiles {
		if a.Tiles[i] != b.Tiles[i] || a.Read(i) != b.Read(i) {
			t.Fatalf("tile %d came out %+v %+v one time and %+v %+v the next", i, a.Tiles[i], a.Read(i), b.Tiles[i], b.Read(i))
		}
	}
}

// A drawn world is what it always was: no plates, no dated rock, and none of
// the two rocks a history makes. Nothing about this change may reach a map
// that did not ask for it.
func TestADrawnWorldIsUntouched(t *testing.T) {
	w := NewLand(1, DefaultTerms())
	for i := range w.Grid.Tiles {
		t2 := &w.Grid.Tiles[i]
		if t2.Plate != 0 || t2.Formed != 0 {
			t.Fatalf("tile %d of a drawn world rides plate %d and dates from %d", i, t2.Plate, t2.Formed)
		}
		if t2.Bedrock == Basalt || t2.Bedrock == Schist {
			t.Fatalf("tile %d of a drawn world is %s", i, t2.Bedrock)
		}
	}
}

// A quiet sea floor is lime where the water is warm and mud where it is cold,
// and on a globe that is a matter of latitude: limestone across the tropics
// and the temperate seas, a thicker bed of it the warmer the water, and shale
// beyond the polar front, which quietFloor puts near sixty degrees. A
// valley's sea is a temperate one, and keeps its limestone.
func TestLimestoneIsLaidInWarmSeas(t *testing.T) {
	g := NewGrid(4, 90)
	g.air = Climate{rows: g.H, globe: true}.airFor(g, 1)
	var equator, forty float64
	for y := 0; y < g.H; y++ {
		lat := g.air.lat[y]
		rock, bed := g.quietFloor(y * g.W)
		want := Limestone
		if lat > 66 || lat < -66 {
			want = Shale
		}
		if (lat > 50 && lat < 66) || (lat < -50 && lat > -66) {
			continue // the polar front falls somewhere in here
		}
		if rock != want {
			t.Errorf("a quiet floor at %.0f degrees, %.1f degrees warm, lays %s", lat, g.air.mean[y], rock)
		}
		if bed <= 0 {
			t.Errorf("a quiet floor at %.0f degrees lays nothing", lat)
		}
		if lat > 0 && lat < 2 {
			equator = bed
		}
		if lat > 40 && lat < 42 {
			forty = bed
		}
	}
	if !(equator > forty) {
		t.Errorf("the equator's lime is %.0f m an epoch and forty degrees' %.0f", equator, forty)
	}
	v := NewGrid(4, 10)
	v.air = defaultAir(v)
	if rock, _ := v.quietFloor(0); rock != Limestone {
		t.Errorf("a valley's temperate sea lays %s", rock)
	}
}
