package terra

import (
	"github.com/LukasSelin/terra/geom"
	"math"
	"math/rand"
	"testing"
)

// A landmark bound is only ever at or below the true cost of the walk, so a
// search that reads it settles the destination at the same cost, by the
// same first step, as one that reads nothing but the straight line - only
// sooner. This holds the search to that on a made valley: with the tables
// fresh, after the ground has got cheaper under them, after water has
// opened to a laden walker, after they are taken again, and after an age of
// weather has moved the ground.
func TestLandmarkBoundsNeverOverstateTheWalk(t *testing.T) {
	w := NewLand(7, DefaultTerms())
	g := w.Grid
	rng := rand.New(rand.NewSource(7))
	type pair struct {
		from, to geom.Pos
		load     float64
	}
	var pairs []pair
	for len(pairs) < 600 {
		p := pair{
			from: geom.Pos{X: rng.Intn(g.W), Y: rng.Intn(g.H)},
			to:   geom.Pos{X: rng.Intn(g.W), Y: rng.Intn(g.H)},
		}
		if rng.Intn(2) == 0 {
			p.load = 1
		}
		if p.from != p.to {
			pairs = append(pairs, p)
		}
	}

	r := g.Router()
	// route runs the search for one pair and says what it cost, what it
	// opened, and which way it set out.
	route := func(p pair) (cost float64, step geom.Pos, path []geom.Pos, work int) {
		r.Reset()
		cost = r.Carrying(p.load).TravelCost(p.from, p.to)
		work = r.Work
		step = r.Carrying(p.load).StepToward(p.from, p.to)
		path = r.Carrying(p.load).Path(p.from, p.to)
		return
	}
	// check holds every route to the same cost and first step with the
	// tables as without, and where the tables are fresh, to fewer tiles
	// opened; slack takes the bounds down toward the straight line, so a
	// stage carrying a lot of it proves only that they never overstate.
	check := func(stage string, fresh bool) {
		t.Helper()
		l := &g.landmarks
		if !l.usable() {
			t.Fatalf("%s: the tables are not usable, so nothing is being checked", stage)
		}
		plainWork, guidedWork, moved := 0, 0, 0
		for _, p := range pairs {
			l.built = false
			wantCost, wantStep, wantPath, w1 := route(p)
			l.built = true
			gotCost, gotStep, gotPath, w2 := route(p)
			plainWork += w1
			guidedWork += w2
			if math.IsInf(wantCost, 1) != math.IsInf(gotCost, 1) || math.Abs(wantCost-gotCost) > tie {
				t.Fatalf("%s: %v -> %v carrying %v costs %v with landmarks, %v without", stage, p.from, p.to, p.load, gotCost, wantCost)
			}
			if gotStep != wantStep {
				t.Fatalf("%s: %v -> %v carrying %v sets out to %v with landmarks, %v without", stage, p.from, p.to, p.load, gotStep, wantStep)
			}
			if len(gotPath) != len(wantPath) {
				moved++
				continue
			}
			for i := range gotPath {
				if gotPath[i] != wantPath[i] {
					moved++
					break
				}
			}
		}
		t.Logf("%s: %d tiles opened with the landmarks, %d without; %d of %d routes went another way of the same cost", stage, guidedWork, plainWork, moved, len(pairs))
		if fresh && guidedWork > plainWork {
			t.Errorf("%s: the landmarks made the searches open more, not fewer", stage)
		}
	}

	day := 0
	refresh := func() { day += LandmarkRest; g.RefreshLandmarks(day) }
	refresh()
	if got := len(g.Landmarks()); got != LandmarkCount {
		t.Fatalf("a valley got %d landmarks, want %d", got, LandmarkCount)
	}
	check("fresh", true)

	// The ground gets cheaper under the tables: a street is laid, and a
	// wood cleared, and neither is told to the tables beyond the slack.
	laid, cleared := 0, 0
	for x := 5; x < g.W-5; x++ {
		p := geom.Pos{X: x, Y: g.H / 2}
		if t := g.At(p); !t.Wet() && t.Mark == None {
			g.Build(p, paving)
			laid++
		}
	}
	for i := range g.Tiles {
		if g.Tiles[i].Terrain == Forest && cleared < 40 {
			g.Turn(g.PosOf(i), Grass)
			cleared++
		}
	}
	if laid == 0 || cleared == 0 {
		t.Fatalf("nothing changed on the ground: %d laid, %d cleared", laid, cleared)
	}
	if g.landmarks.Builds != 1 {
		t.Fatalf("building took the tables again: %d builds", g.landmarks.Builds)
	}
	some := false
	for _, s := range g.landmarks.slack {
		some = some || s > 0
	}
	if !some {
		t.Fatal("cheaper ground left no slack")
	}
	check("cheaper", false)

	// Water opens to a laden walker: a bridge.
	bridged := false
	for i := range g.Tiles {
		if g.Tiles[i].Deep() {
			g.Build(g.PosOf(i), paving)
			bridged = true
			break
		}
	}
	if !bridged {
		t.Fatal("no water to bridge")
	}
	if g.landmarks.ladenOK {
		t.Fatal("a bridge left the laden tables in use")
	}
	check("bridged", false)

	refresh()
	if g.landmarks.Builds != 2 || !g.landmarks.ladenOK {
		t.Fatalf("after a bridge the tables were not taken again: %d builds, laden ok %v", g.landmarks.Builds, g.landmarks.ladenOK)
	}
	check("retaken", true)

	// An age of weather moves the ground itself.
	w.Erode()
	if g.landmarks.usable() {
		t.Fatal("an age of weather left the tables in use")
	}
	refresh()
	if g.landmarks.Builds != 3 {
		t.Fatalf("after an age the tables were not taken again: %d builds", g.landmarks.Builds)
	}
	check("weathered", true)
}

// Enough cheaper ground in one chunk has the tables taken again at the
// next refresh once they have rested, and less does not; and nothing but
// the ground moving has them taken again before they have rested.
func TestLandmarksAreTakenAgainOnceTheGroundIsMuchCheaper(t *testing.T) {
	g := NewGrid(40, 24) // all grass, so every road saves exactly half a tile
	g.RefreshLandmarks(0)
	g.RefreshLandmarks(LandmarkRest)
	if g.landmarks.Builds != 1 {
		t.Fatalf("a refresh with nothing changed took the tables again: %d builds", g.landmarks.Builds)
	}
	for x := 0; x < 4; x++ {
		g.Build(geom.Pos{X: x, Y: 3}, paving)
	}
	g.RefreshLandmarks(LandmarkRest)
	if g.landmarks.Builds != 1 {
		t.Fatalf("two tiles' worth of saving took the tables again: %d builds", g.landmarks.Builds)
	}
	for x := 4; x < 10; x++ {
		g.Build(geom.Pos{X: x, Y: 3}, paving)
	}
	g.RefreshLandmarks(LandmarkRest - 1)
	if g.landmarks.Builds != 1 {
		t.Fatalf("five tiles' worth of saving took the tables again before they had rested: %d builds", g.landmarks.Builds)
	}
	g.RefreshLandmarks(LandmarkRest)
	if g.landmarks.Builds != 2 {
		t.Fatalf("five tiles' worth of saving did not take the tables again: %d builds", g.landmarks.Builds)
	}
	// The ground moving does not wait.
	g.Rekind()
	g.RefreshLandmarks(LandmarkRest + 1)
	if g.landmarks.Builds != 3 {
		t.Fatalf("the ground moving did not take the tables again at once: %d builds", g.landmarks.Builds)
	}
}

// On a globe the window a search runs in may go round the seam, and the
// slack it carries has to come from the chunks on both sides of it.
func TestSlackIsReadRoundTheSeam(t *testing.T) {
	g := NewGrid(256, 128)
	g.Wrap = true
	g.RefreshLandmarks(0)
	g.Build(geom.Pos{X: 2, Y: 10}, paving)   // chunk column 0
	g.Build(geom.Pos{X: 250, Y: 10}, paving) // chunk column 3
	// A window of 161 whose corner is at x = 200 runs to x = 360, which is
	// round to x = 104: it touches columns 3, 0 and 1.
	if got := g.slackWithin(200, 0, 161); math.Abs(got-1.0) > 1e-12 {
		t.Fatalf("slack round the seam is %v, want 1.0", got)
	}
	// One that stays between them touches neither.
	if got := g.slackWithin(70, 0, 50); got != 0 {
		t.Fatalf("slack away from the roads is %v, want 0", got)
	}
}
