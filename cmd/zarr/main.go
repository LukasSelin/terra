// zarr makes a world and writes everything it knows, tile by tile, into a
// Zarr v3 store: one array for each thing a tile has, cut into chunks of the
// world's own chunks and kept in shards, readable from Go with
// github.com/LukasSelin/zarr and from Python with zarr or xarray.
//
//	go run . -out valley.zarr                       the valley, drawn
//	go run . -preset ancient -out ancient.zarr      made from its history
//	go run . -preset globe -out globe.zarr
//	go run . -preset globe -max -out big.zarr       as big as memory allows
//
// It is run from cmd/zarr, which is a module of its own (see go.mod), or from
// the root as go run -C cmd/zarr . with -out given as a path from cmd/zarr.
// What it writes is laid out in README.md and export.go.
package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/zarr"
)

func main() {
	var (
		seed    = flag.Uint64("seed", 1, "the seed the world is made from")
		preset  = flag.String("preset", "valley", "valley, ancient or globe")
		w       = flag.Int("w", 0, "width in tiles (overrides the preset)")
		h       = flag.Int("h", 0, "height in tiles (overrides the preset)")
		epochs  = flag.Int("epochs", -1, "ages of history to run (overrides the preset)")
		sea     = flag.Float64("sea", -1, "share of the ground under the sea (overrides the preset)")
		water   = flag.Float64("water", -1, "metres of water a made world is given (overrides the preset)")
		wrap    = flag.Bool("wrap", false, "join the east edge to the west (forced on by -preset globe)")
		biggest = flag.Bool("max", false, "make the world as big as memory allows")
		out     = flag.String("out", "world.zarr", "the store to write, a directory that must not exist yet")
		chunk   = flag.Int("chunk", terra.ChunkSide, "tiles along a side of a chunk")
		shard   = flag.Int("shard", 16, "chunks along a side of a shard; 0 keeps every chunk in a file of its own")
		gzip    = flag.Int("gzip", 5, "gzip level for each chunk, 0 to 9; -1 is uncompressed")
	)
	flag.Parse()

	var t terra.Terms
	switch *preset {
	case "valley":
		t = terra.DefaultTerms()
	case "ancient":
		t = terra.AncientTerms()
	case "globe":
		t = terra.GlobeTerms()
	default:
		fail(fmt.Errorf("unknown preset %q: want valley, ancient or globe", *preset))
	}
	if *w > 0 {
		t.Width = *w
	}
	if *h > 0 {
		t.Height = *h
	}
	if *epochs >= 0 {
		t.Epochs = *epochs
	}
	if *sea >= 0 {
		t.SeaShare = *sea
	}
	if *water >= 0 {
		t.Water = *water
	}
	if *wrap {
		t.Wrap = true
	}
	if *biggest {
		var err error
		if t, err = t.Largest(); err != nil {
			fail(err)
		}
	}
	o := options{Chunk: *chunk, Shard: *shard, Gzip: *gzip}
	if err := o.check(); err != nil {
		fail(err)
	}
	if _, err := os.Stat(*out); err == nil {
		fail(fmt.Errorf("%s is already there: write somewhere new, or remove it first", *out))
	}

	fmt.Printf("making a %dx%d world from seed %d (epochs %d, sea %.2f, water %.1f m, wrap %v)...\n", t.Width, t.Height, *seed, t.Epochs, t.SeaShare, t.Water, t.Wrap)
	start := time.Now()
	land, err := terra.MakeLand(*seed, t)
	if err != nil {
		fail(err)
	}
	fmt.Printf("made in %v\n", time.Since(start).Round(time.Millisecond))

	start = time.Now()
	arrays, err := export(context.Background(), land, zarr.NewDirStore(*out), o)
	if err != nil {
		fail(err)
	}
	files, bytes := sizeOf(*out)
	fmt.Printf("wrote %d arrays to %s in %v: %d files, %.1f MiB\n", arrays, *out, time.Since(start).Round(time.Millisecond), files, float64(bytes)/(1<<20))
}

func sizeOf(dir string) (files int, bytes int64) {
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				files++
				bytes += info.Size()
			}
		}
		return nil
	})
	return files, bytes
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "zarr:", err)
	os.Exit(1)
}
