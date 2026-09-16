package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LukasSelin/terra"
)

// readWater reads water.json back with every body's fields in one shape.
func readWater(t *testing.T, dir string) []struct {
	Type    string
	Level   float64
	Outline [][2]float64
	Closed  bool
	Points  []riverPoint
} {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, waterName))
	if err != nil {
		t.Fatal(err)
	}
	var w struct {
		Bodies []struct {
			Type    string
			Level   float64
			Outline [][2]float64
			Closed  bool
			Points  []riverPoint
		}
	}
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatal(err)
	}
	return w.Bodies
}

// A river's surface never rises along its reach, every point has a width
// and a depth, and a reach has at least two points.
func TestRiverPointsFallAlongTheReach(t *testing.T) {
	_, dir := valleyExport(t)
	rivers := 0
	for _, b := range readWater(t, dir) {
		if b.Type != "river" {
			continue
		}
		rivers++
		if len(b.Points) < 2 {
			t.Fatalf("a reach of %d points", len(b.Points))
		}
		for k, p := range b.Points {
			if p.Width <= 0 || p.Depth <= 0 {
				t.Fatalf("point %d: width %g, depth %g", k, p.Width, p.Depth)
			}
			if k > 0 && p.Z > b.Points[k-1].Z {
				t.Fatalf("point %d rises from %.4f to %.4f m", k, b.Points[k-1].Z, p.Z)
			}
		}
	}
	if rivers == 0 {
		t.Fatal("the valley has no rivers")
	}
}

// Each lake's outline is a closed loop, and the sea is there when the world
// has one and not when it does not.
func TestLakesAreClosedAndTheSeaIsThere(t *testing.T) {
	m, dir := valleyExport(t)
	oceans := 0
	for _, b := range readWater(t, dir) {
		switch b.Type {
		case "ocean":
			oceans++
		case "lake":
			if len(b.Outline) < 5 {
				t.Fatalf("a lake with an outline of %d corners", len(b.Outline))
			}
			if first, last := b.Outline[0], b.Outline[len(b.Outline)-1]; first != last {
				t.Fatalf("a lake's outline runs from %v to %v and does not close", first, last)
			}
		}
	}
	if (m.SeaLevel != nil) != (oceans == 1) {
		t.Fatalf("sea level %v, but %d ocean bodies", m.SeaLevel, oceans)
	}
}

// A lake's outline traced by hand: a two-by-two block of tiles is a square
// of eight edges, and a hole in a ring is left out.
func TestLakeOutlineTracesTheShore(t *testing.T) {
	g := &terra.Grid{W: 5, H: 5}
	lakeOf := make([]int, 25)
	for i := range lakeOf {
		lakeOf[i] = -1
	}
	for _, i := range []int{1*5 + 1, 1*5 + 2, 2*5 + 1, 2*5 + 2} {
		lakeOf[i] = 0
	}
	out := lakeOutline(g, lakeOf, 0)
	if len(out) != 9 {
		t.Fatalf("a 2x2 lake has an outline of %d corners, want 9", len(out))
	}
	if out[0] != out[8] {
		t.Fatalf("the outline does not close: %v", out)
	}
	for i := range lakeOf {
		lakeOf[i] = -1
	}
	for y := 1; y <= 3; y++ {
		for x := 1; x <= 3; x++ {
			if x != 2 || y != 2 {
				lakeOf[y*5+x] = 0
			}
		}
	}
	if out := lakeOutline(g, lakeOf, 0); len(out) != 13 {
		t.Fatalf("a ring has an outer shore of %d corners, want 13", len(out))
	}
}
