package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/zarr"
)

// The xarray test runs testdata/read_xarray.py under the Python named by
// ZARR_PYTHON, which must have xarray, zarr and numpy installed, and is
// skipped without it:
//
//	python -m venv .venv && .venv/bin/pip install xarray zarr numpy
//	ZARR_PYTHON=.venv/bin/python go test -run Xarray
//
// A small made world is written as the command writes it, and xarray opens
// it: read_xarray.py checks it against what the world says, in expect.json.
func TestXarrayReadsAStore(t *testing.T) {
	py := os.Getenv("ZARR_PYTHON")
	if py == "" {
		t.Skip("ZARR_PYTHON is not set")
	}
	terms := terra.AncientTerms()
	land := terra.NewLand(1, terms)
	g := land.Grid
	dir := t.TempDir()
	store := filepath.Join(dir, "world.zarr")
	if _, err := export(ctx, land, zarr.NewDirStore(store), small); err != nil {
		t.Fatal(err)
	}

	// A dry tile with beds under it, near the middle.
	at := -1
	for i := g.W*g.H/2 + g.W/2; i < len(g.Tiles); i++ {
		if !g.Tiles[i].Wet() && len(g.AppendBeds(nil, i)) > 0 {
			at = i
			break
		}
	}
	if at < 0 {
		t.Fatal("no dry tile with beds")
	}
	missing := 0
	for i := range g.Tiles {
		missing += terra.BedsMax - len(g.AppendBeds(nil, i))
	}
	flags := map[int]string{}
	for i, m := range koppenCodes.meanings {
		flags[koppenCodes.values[i]] = m
	}
	p := g.PosOf(at)
	expect := map[string]any{
		"groups":       []string{"ground", "tile", "layers", "climate", "strata", "book", "features", "features/table"},
		"koppen":       map[string]any{"x": p.X, "y": p.Y, "type": g.Koppen(p), "flags": flags},
		"tile_span":    terra.TileSpan,
		"x_last":       float64(g.W-1) * terra.TileSpan,
		"height":       g.Height[at],
		"beds_missing": missing,
		"rock_top":     int(g.AppendBeds(nil, at)[0].Rock),
		"leached":      float64(g.Tiles[at].Leached) / 65535,
	}
	b, err := json.Marshal(expect)
	if err != nil {
		t.Fatal(err)
	}
	expectPath := filepath.Join(dir, "expect.json")
	if err := os.WriteFile(expectPath, b, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(py, filepath.Join("testdata", "read_xarray.py"), store, expectPath).CombinedOutput()
	t.Logf("xarray: %s", strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("xarray failed: %v", err)
	}
}
