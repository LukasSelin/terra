package main

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/LukasSelin/terra"
)

// One tree per forest tile, inside its tile, with a known species and a
// scale and yaw in range; and the same trees every time.
func TestTreesStandOnTheirTiles(t *testing.T) {
	land := valleyWorld(t)
	g := land.Grid
	forest := 0
	for i := range g.Tiles {
		if g.Tiles[i].Terrain == terra.Forest {
			forest++
		}
	}
	ts := trees(g, 1, riverFlow)
	if len(ts) != forest {
		t.Fatalf("%d trees on %d forest tiles", len(ts), forest)
	}
	known := map[string]bool{}
	for _, s := range species {
		known[s] = true
	}
	for _, tr := range ts {
		cx, cy := metres(g.PosOf(tr.Tile))
		if math.Abs(tr.X-cx) > terra.TileSpan/2 || math.Abs(tr.Y-cy) > terra.TileSpan/2 {
			t.Fatalf("tree on tile %d stands at (%.1f, %.1f), off its tile at (%.1f, %.1f)", tr.Tile, tr.X, tr.Y, cx, cy)
		}
		if !known[tr.Species] {
			t.Fatalf("tree on tile %d is a %q", tr.Tile, tr.Species)
		}
		if tr.Scale <= 0 || tr.Scale > 1.5 || tr.Yaw < 0 || tr.Yaw >= 360 {
			t.Fatalf("tree on tile %d: scale %g, yaw %g", tr.Tile, tr.Scale, tr.Yaw)
		}
		if tr.Z != g.Height[tr.Tile] {
			t.Fatalf("tree on tile %d stands at %g m on ground at %g m", tr.Tile, tr.Z, g.Height[tr.Tile])
		}
	}
	again := trees(g, 1, riverFlow)
	for k := range ts {
		if ts[k] != again[k] {
			t.Fatalf("tree %d differs between two readings: %v and %v", k, ts[k], again[k])
		}
	}
	other := trees(g, 2, riverFlow)
	same := 0
	for k := range ts {
		if ts[k].X == other[k].X && ts[k].Yaw == other[k].Yaw {
			same++
		}
	}
	if same == len(ts) {
		t.Error("another seed jitters every tree the same way")
	}
}

// The CSV written has a header, and one parseable row per tree.
func TestFoliageFileParses(t *testing.T) {
	_, dir := valleyExport(t)
	land := valleyWorld(t)
	f, err := os.Open(filepath.Join(dir, foliageName))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, comments, header := 0, 0, false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "#"):
			comments++
		case line == "x,y,z,species,scale,yaw,tile":
			header = true
		default:
			cols := strings.Split(line, ",")
			if len(cols) != 7 {
				t.Fatalf("row %q has %d columns", line, len(cols))
			}
			for _, k := range []int{0, 1, 2, 4, 5} {
				if _, err := strconv.ParseFloat(cols[k], 64); err != nil {
					t.Fatalf("row %q: %v", line, err)
				}
			}
			if _, err := strconv.Atoi(cols[6]); err != nil {
				t.Fatalf("row %q: %v", line, err)
			}
			rows++
		}
	}
	if !header || comments == 0 {
		t.Fatal("the file has no header")
	}
	if want := len(trees(land.Grid, 1, riverFlow)); rows != want {
		t.Fatalf("%d rows, %d trees", rows, want)
	}
}

// The Köppen reading is cmd/overview's: a few years read the same way.
func TestKoppenReadsAsOverviewDoes(t *testing.T) {
	cases := []struct {
		mean, cold, hot, rain, warm float64
		ice                         bool
		want                        string
	}{
		{-10, -30, 8, 300, 0.6, false, "ET"},
		{-15, -35, -2, 300, 0.6, false, "EF"},
		{0, -20, 20, 600, 0.6, false, "Dfb"},
		{5, -15, 24, 700, 0.6, false, "Dfa"},
		{10, 2, 18, 800, 0.4, false, "Cfb"},
		{16, 8, 24, 500, 0.15, false, "Csa"},
		{26, 22, 28, 2500, 0.5, false, "Af"},
		{26, 22, 28, 1200, 0.9, false, "Aw"},
		{20, 10, 30, 150, 0.5, false, "BWh"},
		{5, -5, 15, 200, 0.5, false, "BSk"},
	}
	for _, c := range cases {
		if got := koppenOf(c.mean, c.cold, c.hot, c.rain, c.warm, c.ice); got != c.want {
			t.Errorf("koppenOf(%v) = %q, want %q", c, got, c.want)
		}
	}
	if biome("Cfb", true) != bWetland || biome("BSk", true) != bColdSteppe || biome("Dfa", false) != bContinental {
		t.Error("biome names differ from overview's")
	}
}
