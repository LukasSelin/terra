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
// A history is two thirds of making a globe. -keep-history writes it to a
// file as the world is made, and -from-history makes the world again from
// the file, on the seed and terms it carries, in the time the stages after
// it take. -stages writes a store at the end of every stage, and
// -stages-diff says which stage two such directories first differ at:
//
//	go run . -preset globe -keep-history globe.history -out base.zarr
//	go run . -from-history globe.history -stages tweak
//	go run . -stages-diff base tweak
//
// It is run from cmd/zarr, which is a module of its own (see go.mod), or from
// the root as go run -C cmd/zarr . with -out given as a path from cmd/zarr.
// What it writes is laid out in README.md and export.go.
package main

import (
	"context"
	"encoding/json"
	"errors"
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
		seed        = flag.Uint64("seed", 1, "the seed the world is made from")
		preset      = flag.String("preset", "valley", "valley, ancient or globe")
		termsFile   = flag.String("terms", "", "a JSON file of a whole terra.Terms to start from instead of the preset; the flags below override it")
		w           = flag.Int("w", 0, "width in tiles (overrides the preset)")
		h           = flag.Int("h", 0, "height in tiles (overrides the preset)")
		epochs      = flag.Int("epochs", -1, "ages of history to run (overrides the preset)")
		sea         = flag.Float64("sea", -1, "share of the ground under the sea (overrides the preset)")
		water       = flag.Float64("water", -1, "metres of water a made world is given (overrides the preset)")
		wrap        = flag.Bool("wrap", false, "join the east edge to the west (forced on by -preset globe)")
		wetness     = flag.Float64("wetness", 0, "rain the air carries against the real world's, above 0 (overrides the preset)")
		woods       = flag.String("woods", "", "where trees stand: shape, tuned or climate (overrides the preset)")
		growth      = flag.String("growth", "", "how fast green things grow: shape, tuned or climate (overrides the preset)")
		glacial     = flag.Bool("glacial", false, "cut a drawn map's valleys through the last glacial cycle")
		biggest     = flag.Bool("max", false, "make the world as big as memory allows")
		keepHistory = flag.String("keep-history", "", "write the world's history to this file as it is made")
		fromHistory = flag.String("from-history", "", "make the world from a history file -keep-history wrote, on the seed and terms it carries, instead of from the flags")
		stagesDir   = flag.String("stages", "", "write a store at the end of every stage into this directory, 1-ground.zarr to 6-cover.zarr, and -out only if it is given")
		stagesDiff  = flag.Bool("stages-diff", false, "compare two -stages directories, given as arguments, and say the first stage they differ at")
		out         = flag.String("out", "world.zarr", "the store to write, a directory that must not exist yet")
		chunk       = flag.Int("chunk", terra.ChunkSide, "tiles along a side of a chunk")
		shard       = flag.Int("shard", 16, "chunks along a side of a shard; 0 keeps every chunk in a file of its own")
		compress    = flag.String("compress", "zstd", "what each chunk is compressed with: zstd, gzip or none")
		level       = flag.Int("level", -1, "the level it compresses at, 1 to 22 for zstd and 0 to 9 for gzip; -1 is the compressor's default")
	)
	flag.Parse()
	given := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { given[f.Name] = true })

	if *stagesDiff {
		if flag.NArg() != 2 {
			fail(errors.New("-stages-diff takes two directories -stages wrote"))
		}
		differ, err := diffStages(os.Stdout, flag.Arg(0), flag.Arg(1))
		if err != nil {
			fmt.Fprintln(os.Stderr, "zarr:", err)
			os.Exit(2)
		}
		if differ {
			os.Exit(1)
		}
		return
	}

	var t terra.Terms
	from := ""
	if *fromHistory != "" {
		// The seed and the terms are the history's: a flag that would change
		// them is a mistake, not something to ignore quietly.
		for _, name := range []string{"seed", "preset", "terms", "w", "h", "epochs", "sea", "water", "wrap", "wetness", "woods", "growth", "glacial", "max", "keep-history"} {
			if given[name] {
				fail(fmt.Errorf("-%s with -from-history: a world made from a history is made on the seed and terms the history carries", name))
			}
		}
		var err error
		if *seed, t, err = historyTerms(*fromHistory); err != nil {
			fail(err)
		}
		from = " from the history in " + *fromHistory
	} else {
		var err error
		if t, err = presetTerms(*preset, *termsFile); err != nil {
			fail(err)
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
		if *wetness > 0 {
			t.Wetness = *wetness
		}
		if err := setRule(&t.Woods, "woods", *woods); err != nil {
			fail(err)
		}
		if err := setRule(&t.Growth, "growth", *growth); err != nil {
			fail(err)
		}
		if *glacial {
			t.Glacial = true
		}
		if *biggest {
			if t, err = t.Largest(); err != nil {
				fail(err)
			}
		}
	}
	o := options{Chunk: *chunk, Shard: *shard, Compress: *compress, Level: *level}
	if l, ok := levels[o.Compress]; ok && !given["level"] {
		o.Level = l.dflt
	}
	if err := o.check(); err != nil {
		fail(err)
	}
	writeOut := *stagesDir == "" || given["out"]
	if _, err := os.Stat(*out); writeOut && err == nil {
		fail(fmt.Errorf("%s is already there: write somewhere new, or remove it first", *out))
	}
	var watch terra.StageWatch
	var watchErr error
	if *stagesDir != "" {
		for i, name := range terra.Stages() {
			if _, err := os.Stat(stagePath(*stagesDir, i, name)); err == nil {
				fail(fmt.Errorf("%s is already there: write somewhere new, or remove it first", stagePath(*stagesDir, i, name)))
			}
		}
		if err := os.MkdirAll(*stagesDir, 0o755); err != nil {
			fail(err)
		}
		watch = func(stage string, land *terra.Land, g *terra.Grid) {
			if watchErr != nil {
				return
			}
			path := stagePath(*stagesDir, stageIndex(stage), stage)
			start := time.Now()
			arrays, err := exportStage(context.Background(), land, g, stage, zarr.NewDirStore(path), o)
			if err != nil {
				watchErr = fmt.Errorf("%s: %w", path, err)
				return
			}
			fmt.Printf("  %-6s wrote %d arrays to %s in %v\n", stage, arrays, path, time.Since(start).Round(time.Millisecond))
		}
	}

	fmt.Printf("making a %dx%d world from seed %d (epochs %d, sea %.2f, water %.1f m, wrap %v)%s...\n", t.Width, t.Height, *seed, t.Epochs, t.SeaShare, t.Water, t.Wrap, from)
	start := time.Now()
	land, err := makeLand(*seed, t, *keepHistory, *fromHistory, watch)
	if err != nil {
		fail(err)
	}
	if watchErr != nil {
		fail(watchErr)
	}
	fmt.Printf("made in %v\n", time.Since(start).Round(time.Millisecond))
	if !writeOut {
		return
	}

	start = time.Now()
	arrays, err := export(context.Background(), land, zarr.NewDirStore(*out), o)
	if err != nil {
		fail(err)
	}
	files, bytes := sizeOf(*out)
	fmt.Printf("wrote %d arrays to %s in %v: %d files, %.1f MiB\n", arrays, *out, time.Since(start).Round(time.Millisecond), files, float64(bytes)/(1<<20))
}

// presetTerms is the terms the world starts from: the file's, where one is
// named, and the preset's otherwise.
func presetTerms(preset, file string) (terra.Terms, error) {
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return terra.Terms{}, err
		}
		var t terra.Terms
		if err := json.Unmarshal(b, &t); err != nil {
			return t, fmt.Errorf("%s: %w", file, err)
		}
		return t, nil
	}
	switch preset {
	case "valley":
		return terra.DefaultTerms(), nil
	case "ancient":
		return terra.AncientTerms(), nil
	case "globe":
		return terra.GlobeTerms(), nil
	}
	return terra.Terms{}, fmt.Errorf("unknown preset %q: want valley, ancient or globe", preset)
}

// rules is the woods and growth rules by the names the flags give them, as
// cmd/overview names them, with shape for terra.ByShape.
var rules = map[string]terra.Rule{"shape": terra.ByShape, "tuned": terra.Tuned, "climate": terra.ByClimate}

// setRule sets *r to the rule named, where one is.
func setRule(r *terra.Rule, flag, name string) error {
	if name == "" {
		return nil
	}
	rule, ok := rules[name]
	if !ok {
		return fmt.Errorf("unknown %s rule %q: want shape, tuned or climate", flag, name)
	}
	*r = rule
	return nil
}

// historyTerms is the seed and the terms of the history file at path.
func historyTerms(path string) (uint64, terra.Terms, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, terra.Terms{}, err
	}
	defer f.Close()
	return terra.HistoryTerms(f)
}

// makeLand makes the land on t: from the history file from if it names one,
// keeping its history in the file keep if that names one, and telling watch
// of each stage where there is a watch.
func makeLand(seed uint64, t terra.Terms, keep, from string, watch terra.StageWatch) (*terra.Land, error) {
	if from != "" {
		f, err := os.Open(from)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return terra.LandFromHistoryWatching(f, watch)
	}
	if keep == "" {
		return terra.MakeLandWatching(seed, t, nil, watch)
	}
	f, err := os.Create(keep)
	if err != nil {
		return nil, err
	}
	land, err := terra.MakeLandWatching(seed, t, f, watch)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return land, err
}

// stagePath is the store of the i-th stage, named name, in dir.
func stagePath(dir string, i int, name string) string {
	return filepath.Join(dir, fmt.Sprintf("%d-%s.zarr", i+1, name))
}

func stageIndex(name string) int {
	for i, s := range terra.Stages() {
		if s == name {
			return i
		}
	}
	return -1
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
