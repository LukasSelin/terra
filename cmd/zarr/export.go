package main

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"slices"
	"sort"
	"sync"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/zarr"
)

// What a store holds.
//
// Every array of the map is H by W, dimensions "y" and "x", with y the row
// of the grid and x the column; the beds add a third, "bed", from the top
// down. Every array says its units and what it is in its attributes, and
// one that holds a code carries a legend of what each code means.
//
//	/                 seed, terms, width, height, wrap, sea_level, chunk_side
//	ground/           height, flow, drain, soil, sand, clay
//	tile/             terrain, bedrock, mark, owner, fenced, plate, formed,
//	                  leached, exposed, lime, salt, carbon
//	layers/           traffic, age, fish, wood, wild, fertility, rich, sward, kinds
//	climate/          mean, coldest, warmest, rain, runoff, rain_warm, koppen
//	strata/           count; top, rock, formed, sand by bed
//	book/             lift, worn, plate_a, plate_b, meeting, epoch,
//	                  burial, buried_in                            (made worlds)
//	features/         belt, basin, lake, plate, climate: each tile's feature id
//	features/table/   one entry a feature, dimension "feature", id - 1
//
// Elements are kept in the types the world keeps them in, so what is read
// back is the world's own number and not a rounding of it.

// options is how the arrays are cut and compressed.
type options struct {
	// Chunk is tiles along a side of a chunk, Shard chunks along a side of
	// a shard (0, unsharded), and Gzip the level each chunk is compressed at
	// (-1, not at all).
	Chunk, Shard, Gzip int
}

func (o options) check() error {
	if o.Chunk <= 0 || o.Shard < 0 || o.Gzip < -1 || o.Gzip > 9 {
		return fmt.Errorf("chunk %d, shard %d, gzip %d: want a chunk above 0, a shard of 0 or more, and gzip from -1 to 9", o.Chunk, o.Shard, o.Gzip)
	}
	return nil
}

// exporter writes one land's arrays.
type exporter struct {
	ctx  context.Context
	land *terra.Land
	g    *terra.Grid
	o    options
	jobs []func() error
	// bytes is what the job of each index holds while it runs, where it
	// is known; see run.
	bytes map[int]int64
}

// export writes land into store, and says how many arrays it wrote. The
// arrays are written side by side; each is a set of keys of its own, and
// what is written does not depend on the order they finish in.
func export(ctx context.Context, land *terra.Land, store zarr.Store, o options) (int, error) {
	if err := o.check(); err != nil {
		return 0, err
	}
	g := land.Grid
	root, err := zarr.CreateGroup(ctx, store, "", map[string]any{
		"generator":  "terra cmd/zarr",
		"seed":       land.Seed(),
		"terms":      land.Terms,
		"width":      g.W,
		"height":     g.H,
		"wrap":       g.Wrap,
		"sea_level":  g.SeaLevel(),
		"chunk_side": terra.ChunkSide,
		"layout":     "tiles are row-major: y is the row of the grid and x its column",
	})
	if err != nil {
		return 0, err
	}
	e := &exporter{ctx: ctx, land: land, g: g, o: o}
	groups := map[string]*zarr.Group{}
	group := func(name string, attrs map[string]any) *zarr.Group {
		if err == nil && groups[name] == nil {
			groups[name], err = root.CreateGroup(ctx, name, attrs)
		}
		return groups[name]
	}
	e.ground(group("ground", map[string]any{"about": "the land itself: its height, its water and its soil"}))
	e.tiles(group("tile", map[string]any{"about": "what each tile is, and what time has made of its soil"}))
	e.layers(group("layers", map[string]any{"about": "the ground that changes by the day, as it stood when the world was made"}))
	e.climate(group("climate", map[string]any{"about": "the year on each tile"}))
	if beds := g.AppendBeds(nil, 0); len(beds) > 0 {
		e.strata(group("strata", map[string]any{"about": "the pile of beds under each tile, from the top down", "beds_max": terra.BedsMax}))
	}
	if _, ok := g.Record(0); ok {
		e.book(group("book", map[string]any{"about": "the meeting that did most to each tile's height, and its last burial, in the history's metres"}))
	}
	if f := g.Features(); f != nil {
		features := group("features", map[string]any{"about": "the features each tile belongs to, by id; 0 is none, and feature id is entry id - 1 of table"})
		e.features(features, f, group("features/table", map[string]any{"about": "every feature, one entry each"}))
	}
	if err != nil {
		return 0, err
	}
	return len(e.jobs), e.run()
}

// run runs the jobs side by side and returns the first error in the order
// the jobs were given. Jobs start in that order, as many at once as there
// are processors and as the bytes they hold fit in exportBytes and
// exportBytesPerTile a tile; a job bigger than that runs when nothing else
// does. A job is let go once it starts, so that what it alone holds goes
// with it.
func (e *exporter) run() error {
	errs := make([]error, len(e.jobs))
	procs, budget := runtime.GOMAXPROCS(0), exportBytes+exportBytesPerTile*int64(len(e.g.Tiles))
	var (
		mu      sync.Mutex
		done    = sync.NewCond(&mu)
		held    int64
		running int
		wg      sync.WaitGroup
	)
	for i, job := range e.jobs {
		e.jobs[i] = nil
		need := e.bytes[i]
		mu.Lock()
		for running > 0 && (running >= procs || held+need > budget) {
			done.Wait()
		}
		held, running = held+need, running+1
		mu.Unlock()
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = job()
			mu.Lock()
			held, running = held-need, running-1
			mu.Unlock()
			done.Signal()
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// What the jobs running at once may hold between them: enough on a
// 1024 by 512 globe that no job waits for another, and on a bigger map
// room for two float64 copies of the map beside the shards being encoded.
// Without it every processor holds a copy of the map, and a world made as
// big as memory allows cannot be written. See the work log, 2026-09-16.
const (
	exportBytes        = 256 << 20
	exportBytesPerTile = 16
)

// add queues a job that holds about bytes while it runs.
func (e *exporter) add(bytes int64, job func() error) {
	if e.bytes == nil {
		e.bytes = map[int]int64{}
	}
	e.bytes[len(e.jobs)] = bytes
	e.jobs = append(e.jobs, job)
}

// stored is how many elements an object in the store holds, a shard or an
// unsharded chunk, for per elements a tile: what zarr.Write fills and
// encodes at once.
func (e *exporter) stored(per int) int64 {
	side := int64(e.o.Chunk * max(e.o.Shard, 1))
	return side * side * int64(per)
}

// field is one array to write.
type field struct {
	name, units, about string
	legend             map[int]string
	fill               any
}

func (f field) attrs() map[string]any {
	a := map[string]any{"about": f.about}
	if f.units != "" {
		a["units"] = f.units
	}
	if f.legend != nil {
		a["legend"] = f.legend
	}
	return a
}

// arrayOptions is how an array of shape is made: chunked and sharded along
// the map's two dimensions, with any dimension after them whole in a chunk.
func (e *exporter) arrayOptions(shape []int, dims []string, f field, d zarr.DataType) zarr.ArrayOptions {
	chunk := slices.Clone(shape)
	chunk[0], chunk[1] = e.o.Chunk, e.o.Chunk
	codecs := []zarr.Codec{zarr.BytesCodec{Endian: zarr.Little}}
	if e.o.Gzip >= 0 {
		codecs = append(codecs, zarr.GzipCodec{Level: e.o.Gzip})
	}
	o := zarr.ArrayOptions{Shape: shape, ChunkShape: chunk, DataType: d, FillValue: f.fill, Codecs: codecs, DimensionNames: dims, Attributes: f.attrs()}
	if e.o.Shard > 0 {
		o.ShardShape = append([]int{e.o.Chunk * e.o.Shard, e.o.Chunk * e.o.Shard}, chunk[2:]...)
	}
	return o
}

// put queues an array of the map, H by W, whose elements data makes.
// It holds the map's elements and a shard's twice, filled and encoded.
func put[T zarr.Element](e *exporter, grp *zarr.Group, f field, data func() []T) {
	size := int64(zarr.DataTypeOf[T]().Size())
	e.add(size*(int64(len(e.g.Tiles))+2*e.stored(1)), func() error {
		return write(e, grp, f, []int{e.g.H, e.g.W}, []string{"y", "x"}, data())
	})
}

func write[T zarr.Element](e *exporter, grp *zarr.Group, f field, shape []int, dims []string, data []T) error {
	a, err := grp.CreateArray(e.ctx, f.name, e.arrayOptions(shape, dims, f, zarr.DataTypeOf[T]()))
	if err != nil {
		return fmt.Errorf("%s/%s: %w", grp.Path(), f.name, err)
	}
	if err := zarr.Write(e.ctx, a, nil, nil, data); err != nil {
		return fmt.Errorf("%s/%s: %w", grp.Path(), f.name, err)
	}
	return nil
}

// tiles is n elements, the i-th of which at is.
func tiles[T any](n int, at func(i int) T) []T {
	s := make([]T, n)
	for i := range s {
		s[i] = at(i)
	}
	return s
}

func same[T any](s []T) func() []T { return func() []T { return s } }

// legendOf names every value from 0 to the largest in data.
func legendOf[T ~uint8](data []uint8, name func(T) string) map[int]string {
	top := uint8(0)
	for _, v := range data {
		top = max(top, v)
	}
	legend := map[int]string{}
	for v := 0; v <= int(top); v++ {
		legend[v] = name(T(v))
	}
	return legend
}

func (e *exporter) ground(grp *zarr.Group) {
	g := e.g
	put(e, grp, field{name: "height", units: "m", about: "height above the lowest ground on the map"}, same(g.Height))
	put(e, grp, field{name: "flow", units: "m3/s", about: "the water running through the tile"}, same(g.Flow))
	put(e, grp, field{name: "drain", units: "m", about: "how far the tile stands above the water it drains into"}, same(g.Drain))
	put(e, grp, field{name: "soil", units: "m", about: "soil over the rock"}, same(g.Soil))
	put(e, grp, field{name: "sand", units: "1", about: "share of the soil that is sand"}, same(g.Sand))
	put(e, grp, field{name: "clay", units: "1", about: "share of the soil that is clay; the rest of sand and clay is silt"}, same(g.Clay))
}

func (e *exporter) tiles(grp *zarr.Group) {
	g, n := e.g, len(e.g.Tiles)
	terrain := tiles(n, func(i int) uint8 { return uint8(g.Tiles[i].Terrain) })
	bedrock := tiles(n, func(i int) uint8 { return uint8(g.Tiles[i].Bedrock) })
	put(e, grp, field{name: "terrain", about: "what the ground is", legend: legendOf(terrain, terra.Terrain.String)}, same(terrain))
	put(e, grp, field{name: "bedrock", about: "the rock at the surface, under the soil", legend: legendOf(bedrock, terra.Bedrock.String)}, same(bedrock))
	put(e, grp, field{name: "mark", about: "what stands on the tile, as the game that built it numbers it; 0 is nothing"},
		func() []uint8 { return tiles(n, func(i int) uint8 { return uint8(g.Tiles[i].Mark) }) })
	put(e, grp, field{name: "owner", about: "who holds the tile; 0 is nobody"},
		func() []int64 { return tiles(n, func(i int) int64 { return int64(g.Tiles[i].Owner) }) })
	put(e, grp, field{name: "fenced", about: "whether the tile lies inside a fence"},
		func() []bool { return tiles(n, func(i int) bool { return g.Tiles[i].Fenced }) })
	put(e, grp, field{name: "plate", about: "the plate number of the crust the tile rides; nothing on a drawn map"},
		func() []uint8 { return tiles(n, func(i int) uint8 { return g.Tiles[i].Plate }) })
	put(e, grp, field{name: "formed", units: "epoch", about: "the epoch the tile's rock dates from; nothing on a drawn map"},
		func() []uint8 { return tiles(n, func(i int) uint8 { return g.Tiles[i].Formed }) })
	put(e, grp, field{name: "leached", units: "1/65535", about: "how much of the bases the rock gave the soil the water has carried off"},
		func() []uint16 { return tiles(n, func(i int) uint16 { return g.Tiles[i].Leached }) })
	put(e, grp, field{name: "exposed", units: "years", about: "how long the surface has been forming soil"},
		func() []float32 { return tiles(n, func(i int) float32 { return g.Tiles[i].Exposed }) })
	put(e, grp, field{name: "lime", units: "0.01 kg/m2", about: "carbonate the dry years have left in the soil"},
		func() []uint16 { return tiles(n, func(i int) uint16 { return g.Tiles[i].Lime }) })
	put(e, grp, field{name: "salt", units: "0.001 kg/m2", about: "salt the dry years have left in the soil"},
		func() []uint16 { return tiles(n, func(i int) uint16 { return g.Tiles[i].Salt }) })
	put(e, grp, field{name: "carbon", units: "kg/m2", about: "organic carbon in the soil"},
		func() []float32 { return tiles(n, func(i int) float32 { return g.Tiles[i].Carbon }) })
}

func (e *exporter) layers(grp *zarr.Group) {
	g := e.g
	for _, l := range []struct {
		name, about string
		data        []float64
	}{
		{"traffic", "how worn the ground is by the crossings over it", g.Traffic},
		{"age", "how much growing weather what stands on the tile has had, in growing ticks", g.Age},
		{"fish", "the fish in the water, to be taken", g.Fish},
		{"wood", "the timber in a wood, to be taken", g.Wood},
		{"wild", "the berries and game under a wood, one count between them", g.Wild},
		{"fertility", "what a field has in it this year", g.Fertility},
		{"rich", "what the ground could have at best", g.Rich},
		{"sward", "the grass standing on open ground", g.Sward},
	} {
		put(e, grp, field{name: l.name, about: l.about}, same(l.data))
	}
	put(e, grp, field{name: "kinds", about: "what the tile is, structure and terrain together, as the day's pass reads it"}, same(g.Kinds))
}

func (e *exporter) climate(grp *zarr.Group) {
	g, n := e.g, len(e.g.Tiles)
	year := func(pick func(mean, coldest, warmest float64) float64) func() []float64 {
		return func() []float64 {
			return tiles(n, func(i int) float64 { return pick(g.YearAt(i)) })
		}
	}
	put(e, grp, field{name: "mean", units: "degC", about: "the mean of the year"}, year(func(m, _, _ float64) float64 { return m }))
	put(e, grp, field{name: "coldest", units: "degC", about: "the mean of the coldest month"}, year(func(_, c, _ float64) float64 { return c }))
	put(e, grp, field{name: "warmest", units: "degC", about: "the mean of the warmest month"}, year(func(_, _, w float64) float64 { return w }))
	put(e, grp, field{name: "rain", units: "mm/year", about: "rain in a year"}, func() []float64 { return tiles(n, g.Rain) })
	put(e, grp, field{name: "runoff", units: "mm/year", about: "rain in a year that runs off"}, func() []float64 { return tiles(n, g.Runoff) })
	put(e, grp, field{name: "rain_warm", units: "1", about: "share of the year's rain that falls in the warmer half year"}, func() []float64 { return tiles(n, g.RainWarm) })

	// The Köppen type of the dry ground, coded in the order of the types'
	// names so that the same world has the same codes.
	types := make([]string, n)
	seen := map[string]bool{}
	for i := range types {
		if !g.Tiles[i].Wet() {
			types[i] = g.Koppen(g.PosOf(i))
			seen[types[i]] = true
		}
	}
	names := make([]string, 0, len(seen))
	for k := range seen {
		names = append(names, k)
	}
	sort.Strings(names)
	code, legend := map[string]uint8{}, map[int]string{0: "water"}
	for c, name := range names {
		code[name] = uint8(c + 1)
		legend[c+1] = name
	}
	put(e, grp, field{name: "koppen", about: "the Köppen-Geiger type of the dry ground", legend: legend},
		func() []uint8 {
			return tiles(n, func(i int) uint8 { return code[types[i]] })
		})
}

func (e *exporter) strata(grp *zarr.Group) {
	g, n := e.g, len(e.g.Tiles)
	var pile []terra.Bed
	rocks := uint8(0)
	for i := range n {
		pile = g.AppendBeds(pile[:0], i)
		for _, b := range pile {
			rocks = max(rocks, uint8(b.Rock))
		}
	}
	put(e, grp, field{name: "count", about: "how many beds the pile holds; beds past it are fill"},
		func() []uint8 {
			var pile []terra.Bed
			return tiles(n, func(i int) uint8 { pile = g.AppendBeds(pile[:0], i); return uint8(len(pile)) })
		})
	nan32 := float32(math.NaN())
	beds(e, grp, field{name: "top", units: "m", about: "height of the bed's upper surface, in the ground's metres", fill: nan32}, nan32,
		func(b terra.Bed) float32 { return float32(b.Top) })
	beds(e, grp, field{name: "rock", about: "the rock of the bed", legend: legendOf([]uint8{rocks}, terra.Bedrock.String)}, 0,
		func(b terra.Bed) uint8 { return uint8(b.Rock) })
	beds(e, grp, field{name: "formed", units: "epoch", about: "the epoch the bed was laid in"}, 0,
		func(b terra.Bed) uint8 { return b.Formed })
	beds(e, grp, field{name: "sand", units: "1/255", about: "share of sand in a bed the water laid"}, 0,
		func(b terra.Bed) uint8 { return b.Sand })
}

// beds queues an array of the beds, H by W by BedsMax, whose element for a
// bed pick makes and for a bed past the tile's pile is empty. An array of
// the beds is BedsMax times the map, so none is built whole: it is written
// a shard at a time from the piles, and holds a shard three times over,
// built, filled and encoded.
func beds[T zarr.Element](e *exporter, grp *zarr.Group, f field, empty T, pick func(terra.Bed) T) {
	g, per := e.g, terra.BedsMax
	shape := []int{g.H, g.W, per}
	e.add(3*int64(zarr.DataTypeOf[T]().Size())*e.stored(per), func() error {
		a, err := grp.CreateArray(e.ctx, f.name, e.arrayOptions(shape, []string{"y", "x", "bed"}, f, zarr.DataTypeOf[T]()))
		if err != nil {
			return fmt.Errorf("%s/%s: %w", grp.Path(), f.name, err)
		}
		side := e.o.Chunk * max(e.o.Shard, 1)
		var (
			buf  []T
			pile []terra.Bed
		)
		for y0 := 0; y0 < g.H; y0 += side {
			for x0 := 0; x0 < g.W; x0 += side {
				h, w := min(side, g.H-y0), min(side, g.W-x0)
				buf = slices.Grow(buf[:0], h*w*per)[:h*w*per]
				at := 0
				for y := y0; y < y0+h; y++ {
					for x := x0; x < x0+w; x++ {
						pile = g.AppendBeds(pile[:0], y*g.W+x)
						for k := range per {
							buf[at] = empty
							if k < len(pile) {
								buf[at] = pick(pile[k])
							}
							at++
						}
					}
				}
				if err := zarr.Write(e.ctx, a, []int{y0, x0, 0}, []int{h, w, per}, buf); err != nil {
					return fmt.Errorf("%s/%s: %w", grp.Path(), f.name, err)
				}
			}
		}
		return nil
	})
}

func (e *exporter) book(grp *zarr.Group) {
	g, n := e.g, len(e.g.Tiles)
	records := tiles(n, func(i int) terra.Record { r, _ := g.Record(i); return r })
	meeting := tiles(n, func(i int) uint8 { return uint8(records[i].Meeting) })
	burial := tiles(n, func(i int) uint8 { return uint8(records[i].Burial) })
	put(e, grp, field{name: "lift", units: "m (history)", about: "what the meeting that did most to the tile's height raised it by in its epoch; negative where a rift dropped it"},
		func() []float32 { return tiles(n, func(i int) float32 { return float32(records[i].Lift) }) })
	put(e, grp, field{name: "worn", units: "m (history)", about: "what the weather has taken off the tile since, to the nearest ten metres"},
		func() []float32 { return tiles(n, func(i int) float32 { return float32(records[i].Worn) }) })
	put(e, grp, field{name: "plate_a", about: "the first plate of the meeting, as numbered in its epoch"},
		func() []uint8 { return tiles(n, func(i int) uint8 { return records[i].Plates[0] }) })
	put(e, grp, field{name: "plate_b", about: fmt.Sprintf("the second plate of the meeting, as numbered in its epoch; %d where a hotspot raised the tile from under one plate", terra.NoPlate)},
		func() []uint8 { return tiles(n, func(i int) uint8 { return records[i].Plates[1] }) })
	put(e, grp, field{name: "meeting", about: "what kind of meeting it was", legend: legendOf(meeting, terra.MeetingKind.String)}, same(meeting))
	put(e, grp, field{name: "epoch", units: "epoch", about: "the epoch of the meeting"},
		func() []uint8 { return tiles(n, func(i int) uint8 { return records[i].Epoch }) })
	put(e, grp, field{name: "burial", about: "what last buried the tile", legend: legendOf(burial, terra.Burial.String)}, same(burial))
	put(e, grp, field{name: "buried_in", units: "epoch", about: "the epoch of the last burial"},
		func() []uint8 { return tiles(n, func(i int) uint8 { return records[i].BuriedIn }) })
}

func (e *exporter) features(grp *zarr.Group, f *terra.Features, table *zarr.Group) {
	g, n := e.g, len(e.g.Tiles)
	for _, k := range []struct {
		name string
		kind terra.FeatureKind
	}{
		{"belt", terra.UpliftBelt},
		{"basin", terra.DrainageBasin},
		{"lake", terra.StandingLake},
		{"plate", terra.CrustPlate},
		{"climate", terra.ClimateRegion},
	} {
		put(e, grp, field{name: k.name, about: "the " + k.kind.String() + " the tile belongs to, by feature id"},
			func() []int32 { return tiles(n, func(i int) int32 { return int32(g.FeatureOf(i, k.kind)) }) })
	}

	all := f.All
	kinds := make([]uint8, len(all))
	for i := range all {
		kinds[i] = uint8(all[i].Kind)
	}
	column := func(fd field, data any) {
		e.jobs = append(e.jobs, func() error {
			switch d := data.(type) {
			case []uint8:
				return writeTable(e, table, fd, d)
			case []int32:
				return writeTable(e, table, fd, d)
			case []int64:
				return writeTable(e, table, fd, d)
			case []float64:
				return writeTable(e, table, fd, d)
			}
			return fmt.Errorf("features table: %T", data)
		})
	}
	column(field{name: "kind", about: "what kind of feature it is", legend: legendOf(kinds, terra.FeatureKind.String)}, kinds)
	column(field{name: "count", units: "tiles", about: "how many tiles the feature has"}, tiles(len(all), func(i int) int64 { return int64(all[i].Count) }))
	column(field{name: "first", units: "tile", about: "the lowest tile of the feature, y*width+x"}, tiles(len(all), func(i int) int32 { return all[i].First }))
	column(field{name: "plate_a", about: "a belt's first plate"}, tiles(len(all), func(i int) uint8 { return all[i].Plates[0] }))
	column(field{name: "plate_b", about: "a belt's second plate"}, tiles(len(all), func(i int) uint8 { return all[i].Plates[1] }))
	column(field{name: "meeting", about: "a belt's kind of meeting", legend: legendOf(tiles(len(all), func(i int) uint8 { return uint8(all[i].Meeting) }), terra.MeetingKind.String)},
		tiles(len(all), func(i int) uint8 { return uint8(all[i].Meeting) }))
	column(field{name: "epoch", units: "epoch", about: "a belt's epoch"}, tiles(len(all), func(i int) uint8 { return all[i].Epoch }))
	column(field{name: "lift", units: "m (history)", about: "the most a belt's meeting raised any tile of it"}, tiles(len(all), func(i int) float64 { return all[i].Lift }))
	column(field{name: "top", units: "tile", about: "a belt's highest tile, y*width+x"}, tiles(len(all), func(i int) int32 { return all[i].Top }))
	column(field{name: "height", units: "m", about: "a belt's highest tile's height"}, tiles(len(all), func(i int) float64 { return all[i].Height }))
	column(field{name: "outlet", units: "tile", about: "the tile a basin's water leaves by, y*width+x"}, tiles(len(all), func(i int) int32 { return all[i].Outlet }))
	column(field{name: "flow", units: "m3/s", about: "the water at a basin's outlet"}, tiles(len(all), func(i int) float64 { return all[i].Flow }))
	column(field{name: "lake", about: "a lake's index among the map's lakes"}, tiles(len(all), func(i int) int32 { return all[i].Lake }))
	column(field{name: "number", about: "a plate's number"}, tiles(len(all), func(i int) uint8 { return all[i].Number }))
	column(field{name: "group", about: "a climate region's Köppen letter, as its character code"}, tiles(len(all), func(i int) uint8 { return all[i].Group }))
}

// writeTable writes a column of the features table: one dimension, cut
// into chunks of the table's own.
func writeTable[T zarr.Element](e *exporter, grp *zarr.Group, f field, data []T) error {
	codecs := []zarr.Codec{zarr.BytesCodec{Endian: zarr.Little}}
	if e.o.Gzip >= 0 {
		codecs = append(codecs, zarr.GzipCodec{Level: e.o.Gzip})
	}
	a, err := grp.CreateArray(e.ctx, f.name, zarr.ArrayOptions{
		Shape: []int{len(data)}, ChunkShape: []int{1 << 14}, DataType: zarr.DataTypeOf[T](),
		Codecs: codecs, DimensionNames: []string{"feature"}, Attributes: f.attrs(),
	})
	if err != nil {
		return fmt.Errorf("%s/%s: %w", grp.Path(), f.name, err)
	}
	return zarr.Write(e.ctx, a, nil, nil, data)
}
