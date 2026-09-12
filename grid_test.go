package terra

import (
	"github.com/LukasSelin/terra/geom"
	"testing"
)

func TestNearestWalksOutward(t *testing.T) {
	g := NewGrid(20, 20)
	g.Turn(geom.Pos{X: 10, Y: 13}, Forest) // distance 3
	g.Turn(geom.Pos{X: 15, Y: 10}, Forest) // distance 5
	p, ok := g.Nearest(geom.Pos{X: 10, Y: 10}, 10, func(_ geom.Pos, tile *Tile) bool {
		return tile.Terrain == Forest
	})
	if !ok || p != (geom.Pos{X: 10, Y: 13}) {
		t.Fatalf("nearest = %v ok=%v, want (10,13)", p, ok)
	}
	if _, ok := g.Nearest(geom.Pos{X: 0, Y: 0}, 2, func(_ geom.Pos, tile *Tile) bool { return tile.Terrain == Forest }); ok {
		t.Fatal("found a forest beyond the search radius")
	}
}

func TestTerrainHasRiverAndForest(t *testing.T) {
	w := NewLand(5, DefaultTerms())
	g := w.Grid
	if g.Count(func(tile *Tile) bool { return tile.Terrain == Water }) < g.H {
		t.Fatal("river should span the map")
	}
	if g.Count(func(tile *Tile) bool { return tile.Terrain == Forest }) == 0 {
		t.Fatal("no forest generated")
	}
	// Fertility should fall away from the river.
	var nearSum, farSum float64
	var near, far int
	for i := range g.Tiles {
		tile := &g.Tiles[i]
		if tile.Terrain == Water {
			continue
		}
		p := geom.Pos{X: i % g.W, Y: i / g.W}
		if g.HasNeighbor(p, func(n *Tile) bool { return n.Terrain == Water }) {
			nearSum += g.Fertility[i]
			near++
		} else {
			farSum += g.Fertility[i]
			far++
		}
	}
	if near == 0 || far == 0 || nearSum/float64(near) <= farSum/float64(far) {
		t.Fatalf("riverbank fertility %.2f should exceed inland %.2f", nearSum/float64(near), farSum/float64(far))
	}
}

func TestRazingGivesTheGroundBack(t *testing.T) {
	g := NewGrid(10, 10)
	p := geom.Pos{X: 4, Y: 4}
	g.Build(p, blocking)
	g.Claim(p, 7)
	tile := g.At(p)
	if !g.Raze(p) {
		t.Fatal("a house could not be taken down")
	}
	if !tile.Buildable() {
		t.Fatalf("the ground under a razed house is not open: %+v", *tile)
	}
	if g.Raze(p) {
		t.Fatal("open ground was razed")
	}
	field := geom.Pos{X: 5, Y: 4}
	g.Turn(field, Field)
	g.Claim(field, 7)
	if !g.Raze(field) || !g.At(field).Buildable() {
		t.Fatalf("a razed field is not open ground: %+v", *g.At(field))
	}
}

// The market is the root of the settlement: streets are measured from it and
// everything else is placed around it, so it is the one thing that stays.
func TestTheMarketCannotBeRazed(t *testing.T) {
	g := NewGrid(10, 10)
	p := geom.Pos{X: 4, Y: 4}
	g.Build(p, rooted)
	if g.Raze(p) || g.At(p).Mark != rooted {
		t.Fatal("the market was taken down")
	}
}
