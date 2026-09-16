package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A valley export on seed 1 completes, and its manifest lists every file
// in the directory with the size it has on disk, and nothing else.
func TestValleyExportListsEveryFile(t *testing.T) {
	m, dir := valleyExport(t)
	listed := map[string]file{}
	for _, f := range m.Files {
		if _, dup := listed[f.Name]; dup {
			t.Fatalf("%s is listed twice", f.Name)
		}
		listed[f.Name] = f
		st, err := os.Stat(filepath.Join(dir, f.Name))
		if err != nil {
			t.Fatalf("%s is listed but %v", f.Name, err)
		}
		if f.Name != manifestName && st.Size() != f.Bytes {
			t.Fatalf("%s is %d bytes on disk and %d in the manifest", f.Name, st.Size(), f.Bytes)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if _, ok := listed[e.Name()]; !ok {
			t.Fatalf("%s was written but is not in the manifest", e.Name())
		}
	}
	if len(entries) != len(m.Files) {
		t.Fatalf("%d files on disk, %d in the manifest", len(entries), len(m.Files))
	}

	// The manifest on disk says what the one in memory does, and the
	// heightmap, the weightmaps and the other files are all there.
	b, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		t.Fatal(err)
	}
	var back manifest
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Seed != 1 || back.Preset != "valley" || back.Width != 80 || back.Height != 36 || back.XYScale != xyScale || back.ZScale != m.ZScale || back.ZOffset != m.ZOffset {
		t.Fatalf("the manifest read back is %+v", back)
	}
	if back.HeightFormula == "" || len(back.Layers) != layerCount {
		t.Fatalf("the manifest read back has formula %q and %d layers", back.HeightFormula, len(back.Layers))
	}
	kinds := map[string]int{}
	for _, f := range back.Files {
		kinds[f.Kind]++
	}
	tiles := m.TileGrid[0] * m.TileGrid[1]
	if kinds["heightmap"] != tiles || kinds["weightmap"] != tiles*layerCount || kinds["water"] != 1 || kinds["foliage"] != 1 {
		t.Fatalf("the manifest lists %v over %d tiles", kinds, tiles)
	}
}
