package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/zarr"
)

// What the compressor costs and buys.
//
// A store is written once and read many times - by xarray, by zarrdiff, by
// whatever is built on it - so a compressor is judged on three numbers: the
// bytes it leaves on disk, the time the export spends in it, and the time a
// reader spends undoing it. The table here is all three, over one world made
// once, so that only the codec differs.

// compressor is one row of the table.
type compressor struct {
	Compress string
	Level    int
}

func (c compressor) String() string {
	if c.Compress == "none" {
		return "none"
	}
	return fmt.Sprintf("%s %d", c.Compress, c.Level)
}

func (c compressor) options() options {
	o := defaults
	o.Compress, o.Level = c.Compress, c.Level
	return o
}

// compressors is the table's rows. gzip 5 is what the export wrote before
// it had a choice; zstd's levels are the four klauspost has, which every
// level maps to - 1 and 2 compress as each other, 10 and 22 as each other.
var compressors = []compressor{
	{"none", 0},
	{"gzip", 1}, {"gzip", 5}, {"gzip", 9},
	{"zstd", 1}, {"zstd", 3}, {"zstd", 7}, {"zstd", 11},
}

// readBack reads every array of the store, and says how long it took and
// how many elements it read: what a reader pays to undo the compressor.
func readBack(ctx context.Context, t testing.TB, s zarr.Store) (time.Duration, int64) {
	root, err := zarr.OpenGroup(ctx, s, "")
	if err != nil {
		t.Fatal(err)
	}
	var arrays []*zarr.Array
	var walk func(g *zarr.Group)
	walk = func(g *zarr.Group) {
		children, err := g.Children(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range children {
			switch c.Type {
			case "array":
				a, err := g.OpenArray(ctx, c.Name)
				if err != nil {
					t.Fatal(err)
				}
				arrays = append(arrays, a)
			case "group":
				sub, err := g.OpenGroup(ctx, c.Name)
				if err != nil {
					t.Fatal(err)
				}
				walk(sub)
			}
		}
	}
	walk(root)
	var read int64
	start := time.Now()
	for _, a := range arrays {
		n, err := readWhole(ctx, a)
		if err != nil {
			t.Fatalf("%s: %v", a.Path(), err)
		}
		read += n
	}
	return time.Since(start), read
}

// readWhole reads all of a, and says how many elements it read.
func readWhole(ctx context.Context, a *zarr.Array) (int64, error) {
	switch a.DataType() {
	case zarr.Bool:
		return count(zarr.Read[bool](ctx, a, nil, nil))
	case zarr.Int8:
		return count(zarr.Read[int8](ctx, a, nil, nil))
	case zarr.Int16:
		return count(zarr.Read[int16](ctx, a, nil, nil))
	case zarr.Int32:
		return count(zarr.Read[int32](ctx, a, nil, nil))
	case zarr.Int64:
		return count(zarr.Read[int64](ctx, a, nil, nil))
	case zarr.Uint8:
		return count(zarr.Read[uint8](ctx, a, nil, nil))
	case zarr.Uint16:
		return count(zarr.Read[uint16](ctx, a, nil, nil))
	case zarr.Uint32:
		return count(zarr.Read[uint32](ctx, a, nil, nil))
	case zarr.Uint64:
		return count(zarr.Read[uint64](ctx, a, nil, nil))
	case zarr.Float32:
		return count(zarr.Read[float32](ctx, a, nil, nil))
	case zarr.Float64:
		return count(zarr.Read[float64](ctx, a, nil, nil))
	}
	return 0, fmt.Errorf("%s: data type %q", a.Path(), a.DataType())
}

func count[T zarr.Element](v []T, err error) (int64, error) { return int64(len(v)), err }

// TestCompression is the table in the work log: TERRA_ZARR_COMPRESS=WxH (or
// "max") makes that globe and writes it with each compressor, reporting what
// each leaves, costs and is read back in. TERRA_ZARR_HISTORY names a history
// file to keep the world in, as TestExportPeak uses it.
//
//	TERRA_ZARR_COMPRESS=1024x512 go test -run TestCompression -v -timeout 60m .
//
// TERRA_ZARR_CODECS="none,zstd:3" holds the run to some of them. A
// compressor's encoder is made once and kept for as long as the process
// runs, so a run of several rows charges each row only its own encoder -
// but read the peak of a row from a run of that row alone, where nothing
// else has been compressed first.
func TestCompression(t *testing.T) {
	size := os.Getenv("TERRA_ZARR_COMPRESS")
	if size == "" {
		t.Skip("TERRA_ZARR_COMPRESS is not set")
	}
	terms := terra.GlobeTerms()
	if size == "max" {
		var err error
		if terms, err = terms.Largest(); err != nil {
			t.Fatal(err)
		}
	} else {
		w, h, _ := strings.Cut(size, "x")
		terms.Width, _ = strconv.Atoi(w)
		terms.Height, _ = strconv.Atoi(h)
	}
	land, err := keptLand(terms)
	if err != nil {
		t.Fatal(err)
	}
	rows := compressors
	if only := os.Getenv("TERRA_ZARR_CODECS"); only != "" {
		rows = nil
		for _, name := range strings.Split(only, ",") {
			c, l, _ := strings.Cut(name, ":")
			level, _ := strconv.Atoi(l)
			rows = append(rows, compressor{c, level})
		}
	}
	procs := runtime.NumCPU()
	t.Logf("%dx%d globe, %d goroutines, chunk %d shard %d", terms.Width, terms.Height, procs, defaults.Chunk, defaults.Shard)
	t.Logf("%-8s %10s %8s %8s %10s %10s", "codec", "stored", "write", "read", "of raw", "peak")
	var raw int64
	for _, c := range rows {
		o := c.options()
		if err := o.check(); err != nil {
			t.Fatal(err)
		}
		s := spend(t, land, o, procs)
		if c.Compress == "none" {
			raw = s.Stored
		}
		mem := zarr.NewMemoryStore()
		if _, err := export(ctx, land, mem, o); err != nil {
			t.Fatal(err)
		}
		took, _ := readBack(ctx, t, mem)
		share := "-"
		if raw > 0 {
			share = fmt.Sprintf("%.1f%%", 100*float64(s.Stored)/float64(raw))
		}
		t.Logf("%-8s %10s %8v %8v %10s %10s", c, mib(uint64(s.Stored)),
			s.Took.Round(time.Millisecond), took.Round(time.Millisecond), share, mib(s.Peak))
	}
	runtime.KeepAlive(land)
}
