package terra

import (
	"math"
	"slices"
	"testing"

	"github.com/LukasSelin/terra/clock"
	"github.com/LukasSelin/terra/geom"
)

// varied is a generated map with every kind of ground laid along its first
// two rows, with something on every layer of those tiles: ages before and
// past anything a stand takes, and a negative nought, which nothing that
// happens on a map ever writes but which a clamp written two ways would
// disagree on; stocks part full, empty and over full; fields part worn.
// Laying tiles by hand puts the chunk counts out, and nothing here reads
// them.
func varied() *Grid {
	g := NewLand(5, DefaultTerms()).Grid
	i := 0
	for s := 0; s <= MarkCount-1; s++ {
		for t := 0; t < int(TerrainCount); t++ {
			for _, age := range []float64{float64(i-15) * 200, math.Copysign(0, -1)} {
				g.Tiles[i].Mark, g.Tiles[i].Terrain = Mark(s), Terrain(t)
				g.Age[i] = age
				g.Wood[i], g.Wild[i], g.Fish[i] = 0.1*float64(i%7), 0.05*float64(i%9), 0.2*float64(i%5)
				g.Rich[i], g.Fertility[i] = 0.6, 0.1*float64(i%6)
				g.Sward[i] = 0.15 * float64(i%8)
				i++
			}
		}
	}
	g.Rekind() // the tiles were laid by hand, so the kinds are told again
	return g
}

// The kind kept beside each tile is the tile's kind, on a map as it is
// made, after the ground is turned, built on and razed, and after an age of
// weather has remade it.
func TestKindsFollowTheGround(t *testing.T) {
	w := NewLand(7, DefaultTerms())
	g := w.Grid
	check := func(when string) {
		t.Helper()
		for i := range g.Tiles {
			if g.Kinds[i] != kindOf(&g.Tiles[i]) {
				t.Fatalf("%s: tile %d is kept as kind %d and is kind %d", when, i, g.Kinds[i], kindOf(&g.Tiles[i]))
			}
		}
	}
	check("as made")
	p := geom.Pos{X: 10, Y: 10}
	g.Turn(p, Field)
	g.Claim(p, 3)
	g.Build(geom.Pos{X: 11, Y: 10}, blocking)
	g.Turn(geom.Pos{X: 12, Y: 10}, Forest)
	check("turned and built on")
	g.Raze(p)
	check("razed")
	w.Erode()
	check("weathered")
}

// layers is every layer of a map by name, for comparing two maps.
func layers(g *Grid) []struct {
	name string
	v    []float64
} {
	return []struct {
		name string
		v    []float64
	}{
		{"traffic", g.Traffic}, {"age", g.Age}, {"fish", g.Fish}, {"wood", g.Wood},
		{"wild", g.Wild}, {"fertility", g.Fertility}, {"rich", g.Rich}, {"sward", g.Sward},
	}
}

// same fails the test where any layer of a and b differs in any bit.
func same(t *testing.T, what string, a, b *Grid) {
	t.Helper()
	la, lb := layers(a), layers(b)
	for n := range la {
		if !slices.Equal(la[n].v, lb[n].v) {
			for i := range la[n].v {
				if la[n].v[i] != lb[n].v[i] {
					t.Fatalf("%s: %s of tile %d came to %v flat and %v tile by tile", what, la[n].name, i, la[n].v[i], lb[n].v[i])
				}
			}
		}
	}
}

// The flat pass is Ripen and then Replenish, tile by tile, to the last bit:
// over a day's weather and a season's at once, over runs of any length and
// alignment, and over and over so that the ceilings on the stocks bite.
func TestGrowIsRipenAndReplenishTileByTile(t *testing.T) {
	for _, k := range []float64{0, 0.37, 0.8 * float64(clock.Season)} {
		flat := varied()
		byTile := flat.Clone()
		for round := 0; round < 4; round++ {
			n := len(flat.Tiles)
			for lo := 0; lo < n; {
				hi := min(n, lo+37+round)
				flat.Grow(lo, hi, k)
				lo = hi
			}
			for i := range byTile.Tiles {
				byTile.Ripen(i, k)
				byTile.Replenish(i, k)
			}
		}
		same(t, "growing", flat, byTile)
	}
}

// Fading the wear without asking whether there is any is the same fade to
// the last bit: ground with none is left with none.
func TestFadeWearIsTheBranchedFade(t *testing.T) {
	flat := NewGrid(64, 2)
	for i, v := range []float64{0, 1e-300, 1e-9, 0.5, 1, 4, 100, 600, 1e6} {
		flat.Traffic[i] = v
	}
	byTile := flat.Clone()
	for round := 0; round < 5000; round++ {
		flat.FadeWear(0, 64, Fade)
		flat.FadeWear(64, 128, Fade)
		for i := range byTile.Traffic {
			if byTile.Traffic[i] > 0 {
				byTile.Traffic[i] *= Fade
			}
		}
	}
	same(t, "fading", flat, byTile)
}

// How long the day's pass takes over one chunk's width of varied ground,
// for holding the arithmetic done four tiles at once against the same
// done one at a time: run it with and without GOEXPERIMENT=simd.
func BenchmarkGrowRow(b *testing.B) {
	g := varied()
	for range b.N {
		g.FadeWear(0, ChunkSide, Fade)
		g.Grow(0, ChunkSide, 0.37)
	}
}
