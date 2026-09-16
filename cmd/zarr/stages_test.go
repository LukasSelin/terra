package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/zarr"
)

// writeStages writes a store at the end of every stage of the world on seed
// and t into dir, as -stages does, and returns the land.
func writeStages(t *testing.T, dir string, seed uint64, terms terra.Terms) *terra.Land {
	t.Helper()
	var err error
	land, merr := terra.MakeLandWatching(seed, terms, nil, func(stage string, l *terra.Land, g *terra.Grid) {
		if err == nil {
			_, err = exportStage(ctx, l, g, stage, zarr.NewDirStore(stagePath(dir, stageIndex(stage), stage)), small)
		}
	})
	if merr != nil {
		t.Fatal(merr)
	}
	if err != nil {
		t.Fatal(err)
	}
	return land
}

// Every stage of a drawn world and of a made one writes a store, with the
// stage named at its root and the groups the grid has nothing for yet left
// out, and the last stage's store is the finished world's store to the byte.
func TestEveryStageWritesAStore(t *testing.T) {
	for name, terms := range map[string]terra.Terms{"valley": terra.DefaultTerms(), "ancient": terra.AncientTerms()} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			land := writeStages(t, dir, 1, terms)
			for i, stage := range terra.Stages() {
				s := zarr.NewDirStore(stagePath(dir, i, stage))
				root, err := zarr.OpenGroup(ctx, s, "")
				if err != nil {
					t.Fatal(err)
				}
				var got string
				if ok, err := root.Attribute("stage", &got); !ok || err != nil || got != stage {
					t.Errorf("%s: the root's stage is %q (%v, %v)", stage, got, ok, err)
				}
				_, climate := s.Get(ctx, "climate/zarr.json")
				_, features := s.Get(ctx, "features/zarr.json")
				if wants := i >= stageIndex("coast"); (climate == nil) != wants {
					t.Errorf("%s: a climate group is %v, want %v", stage, climate == nil, wants)
				}
				if wants := stage == "cover"; (features == nil) != wants {
					t.Errorf("%s: a features group is %v, want %v", stage, features == nil, wants)
				}
				if h := read[float64](t, s, "ground/height"); len(h) != len(land.Grid.Tiles) {
					t.Errorf("%s: %d heights for %d tiles", stage, len(h), len(land.Grid.Tiles))
				}
			}
			whole := filepath.Join(dir, "whole.zarr")
			if _, err := export(ctx, land, zarr.NewDirStore(whole), small); err != nil {
				t.Fatal(err)
			}
			nodes, err := diffStore(stagePath(dir, len(terra.Stages())-1, "cover"), whole)
			if err != nil {
				t.Fatal(err)
			}
			if len(nodes) > 0 {
				t.Errorf("the cover stage's store is not the finished world's: %v", nodes)
			}
		})
	}
}

// Two experiments on the same seed are the same up to the stage a change
// first reaches, and -stages-diff names it: a wetter air is first read when
// the ground is shaped and drained, and nothing before.
func TestStagesDiffFindsTheFirstStage(t *testing.T) {
	base, same, wet := t.TempDir(), t.TempDir(), t.TempDir()
	terms := terra.DefaultTerms()
	writeStages(t, base, 1, terms)
	writeStages(t, same, 1, terms)
	terms.Wetness = 2
	writeStages(t, wet, 1, terms)

	var out strings.Builder
	if differ, err := diffStages(&out, base, same); err != nil || differ {
		t.Errorf("the same world twice differs: %v, %v\n%s", differ, err, out.String())
	}
	out.Reset()
	differ, err := diffStages(&out, base, wet)
	if err != nil || !differ {
		t.Fatalf("a wetter valley is the same: %v, %v\n%s", differ, err, out.String())
	}
	t.Logf("\n%s", out.String())
	lines := strings.Split(out.String(), "\n")
	if !strings.Contains(out.String(), "first differs at 3-shape") {
		t.Error("the first stage that differs is not named as shape")
	}
	for i, stage := range terra.Stages() {
		wants := "the same"
		if i >= stageIndex("shape") {
			wants = "differs"
		}
		if !strings.Contains(lines[i], wants) {
			t.Errorf("stage %s: %q, want %s", stage, lines[i], wants)
		}
	}
}

// -terms reads a whole Terms, and what it names is what the world is made on.
func TestTermsReadFromAFile(t *testing.T) {
	want := terra.GlobeTerms()
	want.Wetness, want.Woods, want.Growth, want.Glacial = 1.5, terra.Tuned, terra.ByClimate, true
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "terms.json")
	if err := os.WriteFile(file, b, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := presetTerms("valley", file)
	if err != nil || got != want {
		t.Errorf("read %+v (%v), want %+v", got, err, want)
	}
	if _, err := presetTerms("valley", filepath.Join(t.TempDir(), "none.json")); err == nil {
		t.Error("a file that is not there reads")
	}
}
