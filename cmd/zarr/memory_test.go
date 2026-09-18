package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"runtime/metrics"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/zarr"
)

// What the export costs beside the world.
//
// A -max world is sized to fill the memory there is, so what the export
// holds on top of the land is what decides whether it can be written at all.
// The readings here are of the export alone: the land is made first, the
// collector run, and the heap read from there.

// discard is a store that keeps nothing of the arrays but a count of the
// bytes it was given, so that what is read is the export's and not the
// store's. It keeps the metadata, which the export reads back to
// consolidate it.
type discard struct {
	bytes atomic.Int64
	meta  sync.Map
}

func (d *discard) Get(_ context.Context, key string) ([]byte, error) {
	if v, ok := d.meta.Load(key); ok {
		return v.([]byte), nil
	}
	return nil, zarr.ErrNotFound
}

func (d *discard) Set(_ context.Context, key string, v []byte) error {
	if path.Base(key) == "zarr.json" {
		d.meta.Store(key, v)
	}
	d.bytes.Add(int64(len(v)))
	return nil
}

func (d *discard) Delete(_ context.Context, key string) error {
	d.meta.Delete(key)
	return nil
}

// List yields the metadata, which is all this store keeps: the chunks it
// was given are gone, and a store is not asked to list what it does not
// hold.
func (d *discard) List(_ context.Context, prefix string, fn func(key string) error) error {
	var err error
	d.meta.Range(func(k, _ any) bool {
		key := k.(string)
		if !strings.HasPrefix(key, prefix) {
			return true
		}
		err = fn(key)
		return err == nil
	})
	return err
}

// spent is what one export cost.
type spent struct {
	// Peak is the most the heap's live objects held at once over what they
	// held before, and Mapped the same of the memory the runtime had from
	// the system and had not given back: the nearer of the two to RSS.
	Peak, Mapped uint64
	// Bytes is all the export asked of the allocator.
	Bytes  uint64
	Took   time.Duration
	Stored int64
}

// spend exports land into a discarding store over procs goroutines. The
// sampler reads every millisecond; see samplePeak in the root's
// budget_test.go for why that and not ReadMemStats.
func spend(t testing.TB, land *terra.Land, o options, procs int) spent {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(procs))
	samples := []metrics.Sample{
		{Name: "/memory/classes/heap/objects:bytes"},
		{Name: "/memory/classes/total:bytes"},
		{Name: "/memory/classes/heap/released:bytes"},
	}
	read := func() (live, mapped uint64) {
		metrics.Read(samples)
		return samples[0].Value.Uint64(), samples[1].Value.Uint64() - samples[2].Value.Uint64()
	}
	stop, done := make(chan struct{}), make(chan [2]uint64)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	live0, mapped0 := read()
	go func() {
		var peak [2]uint64
		for {
			live, mapped := read()
			peak[0], peak[1] = max(peak[0], live), max(peak[1], mapped)
			select {
			case <-stop:
				done <- peak
				return
			case <-time.After(time.Millisecond):
			}
		}
	}()
	s := &discard{}
	start := time.Now()
	if _, err := export(ctx, land, s, o); err != nil {
		t.Fatal(err)
	}
	took := time.Since(start)
	close(stop)
	peak := <-done
	runtime.ReadMemStats(&after)
	return spent{
		Peak:   peak[0] - min(peak[0], live0),
		Mapped: peak[1] - min(peak[1], mapped0),
		Bytes:  after.TotalAlloc - before.TotalAlloc,
		Took:   took,
		Stored: s.bytes.Load(),
	}
}

// defaults is what main writes with when no flag says otherwise.
var defaults = options{Chunk: terra.ChunkSide, Shard: 16, Compress: "zstd", Level: 3}

// BenchmarkExport writes a 512 by 256 globe, made once, as main would, and
// reports the export's peak heap over the world as peak-B/op.
func BenchmarkExport(b *testing.B) {
	t := terra.GlobeTerms()
	t.Width, t.Height = 512, 256
	land := terra.NewLand(1, t)
	b.ResetTimer()
	var most uint64
	for range b.N {
		most = max(most, spend(b, land, defaults, runtime.GOMAXPROCS(0)).Peak)
	}
	b.ReportMetric(float64(most), "peak-B/op")
}

// TestExportPeak is the table in the work log: TERRA_ZARR_PEAK=WxH (or
// "max") makes that globe and exports it at 1, 4 and GOMAXPROCS goroutines.
// TERRA_ZARR_HISTORY names a history file to keep the world's history in,
// or to read it from if it is there, so that a second run skips the making.
//
//	TERRA_ZARR_PEAK=1024x512 go test -run TestExportPeak -v -timeout 60m .
func TestExportPeak(t *testing.T) {
	size := os.Getenv("TERRA_ZARR_PEAK")
	if size == "" {
		t.Skip("TERRA_ZARR_PEAK is not set")
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
	var ms runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&ms)
	base := ms.HeapAlloc
	start := time.Now()
	land, err := keptLand(terms)
	if err != nil {
		t.Fatal(err)
	}
	made := time.Since(start)
	runtime.GC()
	runtime.ReadMemStats(&ms)
	world := ms.HeapAlloc - base
	t.Logf("%dx%d globe made in %v; the land holds %s", terms.Width, terms.Height, made.Round(time.Millisecond), mib(world))
	for _, procs := range []int{1, 4, runtime.NumCPU()} {
		s := spend(t, land, defaults, procs)
		t.Logf("procs %2d: peak %s (%.0f%% of the land), mapped %s, allocated %s, %v, stored %s",
			procs, mib(s.Peak), 100*float64(s.Peak)/float64(world), mib(s.Mapped), mib(s.Bytes), s.Took.Round(time.Millisecond), mib(uint64(s.Stored)))
	}
	runtime.KeepAlive(land)
}

// keptLand is MakeLand, through the history file TERRA_ZARR_HISTORY names
// if it names one.
func keptLand(terms terra.Terms) (*terra.Land, error) {
	path := os.Getenv("TERRA_ZARR_HISTORY")
	if path == "" {
		return terra.MakeLand(1, terms)
	}
	if f, err := os.Open(path); err == nil {
		defer f.Close()
		return terra.LandFromHistory(bufio.NewReader(f))
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	w := bufio.NewWriter(f)
	land, err := terra.MakeLandKeepingHistory(1, terms, w)
	if err == nil {
		err = w.Flush()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return land, err
}

func mib(b uint64) string { return fmt.Sprintf("%.0f MiB", float64(b)/(1<<20)) }
