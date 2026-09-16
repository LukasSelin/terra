package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// Every tile's weights sum to 255, whatever mix of shares they came from.
func TestWeightsSumTo255(t *testing.T) {
	land := valleyWorld(t)
	g := land.Grid
	seen := map[int]bool{}
	for i, w := range allWeights(g, riverFlow) {
		sum := 0
		for k, x := range w {
			sum += int(x)
			if x > 0 {
				seen[k] = true
			}
		}
		if sum != 255 {
			t.Fatalf("tile %d (%s): weights %v sum to %d", i, g.Tiles[i].Terrain, w, sum)
		}
	}
	for _, k := range []int{lGrass, lForest} {
		if !seen[k] {
			t.Errorf("no tile of the valley has any %s", layerNames[k])
		}
	}
	cases := [][layerCount]float64{
		{lGrass: 1.0 / 3, lForest: 1.0 / 3, lRock: 1.0 / 3},
		{lSand: 0.999, lMud: 0.001},
		{lIce: 2, lSalt: 2},
		{},
	}
	for _, s := range cases {
		sum := 0
		for _, x := range quantise(s) {
			sum += int(x)
		}
		if sum != 255 {
			t.Errorf("shares %v quantise to a sum of %d", s, sum)
		}
	}
}

func readGray(t *testing.T, path string) *image.Gray {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	g, ok := img.(*image.Gray)
	if !ok {
		t.Fatalf("%s decodes as %T, not 8-bit gray", path, img)
	}
	return g
}

// The weightmaps written sum to 255 at every vertex of every tile, the
// padding included, and neighbouring tiles share their edges.
func TestWeightmapsSumAtEveryVertex(t *testing.T) {
	m, dir := valleyExport(t)
	size := m.TileSize
	for j := 0; j < m.TileGrid[1]; j++ {
		for i := 0; i < m.TileGrid[0]; i++ {
			var layers [layerCount]*image.Gray
			for k := range layers {
				layers[k] = readGray(t, filepath.Join(dir, tileName("valley", layerNames[k], i, j)))
			}
			for r := 0; r < size; r++ {
				for c := 0; c < size; c++ {
					sum := 0
					for k := range layers {
						sum += int(layers[k].GrayAt(c, r).Y)
					}
					if sum != 255 {
						t.Fatalf("tile (%d,%d) vertex (%d,%d): the layers sum to %d", i, j, c, r, sum)
					}
				}
			}
			if i+1 < m.TileGrid[0] {
				for k := range layers {
					right := readGray(t, filepath.Join(dir, tileName("valley", layerNames[k], i+1, j)))
					for r := 0; r < size; r++ {
						if a, b := layers[k].GrayAt(size-1, r).Y, right.GrayAt(0, r).Y; a != b {
							t.Fatalf("%s tile (%d,%d) row %d: east edge %d, neighbour's west edge %d", layerNames[k], i, j, r, a, b)
						}
					}
				}
			}
		}
	}
}
