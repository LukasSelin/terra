package main

import (
	"image"
	"image/png"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/terra/geom"
)

// mkTemp is a directory for one test binary's files, removed when the tests
// have run: os.MkdirTemp under the test's own temp root.
func mkTemp() (string, error) {
	return os.MkdirTemp("", "terra-unreal-")
}

func TestMain(m *testing.M) {
	code := m.Run()
	if exportDir != "" {
		os.RemoveAll(exportDir)
	}
	os.Exit(code)
}

// The encoding takes any height in its range back to within one step, at
// both ends of the range and in the middle, and the range it fits has room
// beyond the world's own.
func TestHeightEncodingRoundTrips(t *testing.T) {
	for _, r := range []struct{ lo, hi float64 }{{0, 300}, {-5200, 4100}, {12, 12.5}, {0, 0}, {-1, 1}} {
		e := encodingFor(r.lo, r.hi)
		step := e.Step()
		if got := float64(e.Scale) / 100 * 512; got < (r.hi-r.lo)*(1+zMargin) {
			t.Errorf("range %v: Z scale %d spans %.1f m, under the range with its margin", r, e.Scale, got)
		}
		rng := rand.New(rand.NewPCG(1, 2))
		for k := 0; k < 10000; k++ {
			h := r.lo + rng.Float64()*(r.hi-r.lo)
			if k == 0 {
				h = r.lo
			}
			if k == 1 {
				h = r.hi
			}
			back := e.Decode(e.Encode(h))
			if math.Abs(back-h) > step/2+1e-9 {
				t.Fatalf("range %v: %.6f m encodes to %d and decodes to %.6f m, off by more than half a step of %.6f", r, h, e.Encode(h), back, step)
			}
		}
	}
}

// The tiling picks the largest size that is not wider than the world and
// covers the world with tiles that share an edge.
func TestTilingCoversTheWorld(t *testing.T) {
	cases := []struct {
		vw, vh       int
		size, nx, ny int
	}{
		{80, 36, 1009, 1, 1},
		{257, 128, 1009, 1, 1},
		{1009, 1009, 1009, 1, 1},
		{1010, 500, 1009, 2, 1},
		{1025, 512, 1009, 2, 1},
		{2017, 1009, 2017, 1, 1},
		{2018, 1009, 2017, 2, 1},
		{25600, 12800, 8129, 4, 2},
	}
	for _, c := range cases {
		got := tileFor(c.vw, c.vh, tileSizes)
		if got.Size != c.size || got.NX != c.nx || got.NY != c.ny {
			t.Errorf("%dx%d vertices: got %d tiles of %d in %dx%d, want %d in %dx%d", c.vw, c.vh, got.NX*got.NY, got.Size, got.NX, got.NY, c.size, c.nx, c.ny)
		}
		if covered := got.NX*(got.Size-1) + 1; covered < c.vw {
			t.Errorf("%dx%d vertices: %d tiles of %d cover %d across", c.vw, c.vh, got.NX, got.Size, covered)
		}
		if covered := got.NY*(got.Size-1) + 1; covered < c.vh {
			t.Errorf("%dx%d vertices: %d tiles of %d cover %d down", c.vw, c.vh, got.NY, got.Size, covered)
		}
	}
}

// readGray16 reads a 16-bit heightmap back.
func readGray16(t *testing.T, path string) *image.Gray16 {
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
	g, ok := img.(*image.Gray16)
	if !ok {
		t.Fatalf("%s decodes as %T, not 16-bit gray", path, img)
	}
	return g
}

// Neighbouring heightmap tiles share their edge column and row exactly, the
// padding past the world carries its edge on, and every real vertex is the
// tile's height to within a step.
func TestHeightmapTilesShareTheirEdges(t *testing.T) {
	m, dir := valleyExport(t)
	land := valleyWorld(t)
	g := land.Grid
	tl := tiling{VertsW: m.Vertices[0], VertsH: m.Vertices[1], Size: m.TileSize, NX: m.TileGrid[0], NY: m.TileGrid[1]}
	if tl.NX < 2 || tl.NY < 2 {
		t.Fatalf("the test export has %dx%d tiles; it needs neighbours both ways", tl.NX, tl.NY)
	}
	e := zEncoding{Scale: m.ZScale, Offset: m.ZOffset}
	tiles := make([][]*image.Gray16, tl.NY)
	for j := range tiles {
		tiles[j] = make([]*image.Gray16, tl.NX)
		for i := range tiles[j] {
			tiles[j][i] = readGray16(t, filepath.Join(dir, tileName("valley", "", i, j)))
		}
	}
	for j := 0; j < tl.NY; j++ {
		for i := 0; i < tl.NX; i++ {
			img := tiles[j][i]
			if i+1 < tl.NX {
				right := tiles[j][i+1]
				for r := 0; r < tl.Size; r++ {
					if a, b := img.Gray16At(tl.Size-1, r), right.Gray16At(0, r); a != b {
						t.Fatalf("tile (%d,%d) row %d: east edge %d, neighbour's west edge %d", i, j, r, a.Y, b.Y)
					}
				}
			}
			if j+1 < tl.NY {
				below := tiles[j+1][i]
				for c := 0; c < tl.Size; c++ {
					if a, b := img.Gray16At(c, tl.Size-1), below.Gray16At(c, 0); a != b {
						t.Fatalf("tile (%d,%d) column %d: south edge %d, neighbour's north edge %d", i, j, c, a.Y, b.Y)
					}
				}
			}
			c0, r0 := tl.origin(i, j)
			for r := 0; r < tl.Size; r++ {
				for c := 0; c < tl.Size; c++ {
					want := g.Height[vertexTile(g, c0+c, r0+r)]
					got := e.Decode(img.Gray16At(c, r).Y)
					if math.Abs(got-want) > e.Step()/2+1e-9 {
						t.Fatalf("tile (%d,%d) vertex (%d,%d): %.4f m, want %.4f m", i, j, c, r, got, want)
					}
				}
			}
		}
	}
}

// On a globe the column past the east edge is the west edge again, so the
// seam meets; on a valley it is the east edge carried on.
func TestSeamColumnOfAGlobe(t *testing.T) {
	g := &terra.Grid{Map: geom.Map{W: 8, H: 4, Wrap: true}}
	if got := vertexTile(g, 8, 2); got != 2*8+0 {
		t.Errorf("wrapped seam column reads tile %d, want the west edge %d", got, 2*8)
	}
	if got := vertexTile(g, 9, 2); got != 2*8+0 {
		t.Errorf("padding past the seam reads tile %d, want the west edge %d", got, 2*8)
	}
	if got := vertexTile(g, 3, 7); got != 3*8+3 {
		t.Errorf("padding below reads tile %d, want the south edge %d", got, 3*8+3)
	}
	g.Wrap = false
	if got := vertexTile(g, 8, 2); got != 2*8+7 {
		t.Errorf("valley column past the edge reads tile %d, want the east edge %d", got, 2*8+7)
	}
	if vw, _ := vertsOf(&terra.Grid{Map: geom.Map{W: 8, H: 4, Wrap: true}}); vw != 9 {
		t.Errorf("a globe 8 across has %d vertices across, want 9", vw)
	}
}
