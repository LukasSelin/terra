package main

import (
	"sync"
	"testing"

	"github.com/LukasSelin/terra"
)

// The tests share one world, the valley on seed 1 as the command makes it
// with no flags, and one export of it, so that making a world is paid once.

var (
	valleyOnce sync.Once
	valley     *terra.Land
	valleyErr  error
)

// valleyWorld is the valley on seed 1.
func valleyWorld(t *testing.T) *terra.Land {
	t.Helper()
	valleyOnce.Do(func() {
		valley, valleyErr = terra.MakeLand(1, terra.DefaultTerms())
	})
	if valleyErr != nil {
		t.Fatal(valleyErr)
	}
	return valley
}

var (
	exportOnce sync.Once
	exported   *manifest
	exportDir  string
	exportErr  error
)

// valleyExport is the valley exported once, into a directory that lives for
// the test binary, with tiles small enough that the valley has several of
// them and their shared edges can be checked.
func valleyExport(t *testing.T) (*manifest, string) {
	t.Helper()
	land := valleyWorld(t)
	exportOnce.Do(func() {
		exportDir, exportErr = mkTemp()
		if exportErr != nil {
			return
		}
		exported, exportErr = export(land, options{Seed: 1, Preset: "valley", Name: "valley", Out: exportDir, RiverFlow: riverFlow, sizes: []int{17, 33}})
	})
	if exportErr != nil {
		t.Fatal(exportErr)
	}
	return exported, exportDir
}
