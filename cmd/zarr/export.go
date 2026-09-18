package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/zarr"
	"github.com/LukasSelin/zarr/zstd"
)

// What a store holds.
//
// Every array of the map is H by W, dimensions "y" and "x", with y the row
// of the grid and x the column; the beds add a third, "bed", from the top
// down. Every group with those dimensions carries them as coordinates of its
// own - y and x in metres from the centre of tile (0, 0), bed from 0 - so
// that xarray finds them whichever group it opens.
//
//	/                 stage, seed, terms, width, height, wrap, sea_level, chunk_side,
//	                  tile_span, and the store's consolidated metadata
//	ground/           height, flow, drain, soil, sand, clay
//	tile/             terrain, bedrock, mark, owner, fenced, plate, formed,
//	                  leached, exposed, lime, salt, carbon
//	layers/           traffic, age, fish, wood, wild, fertility, rich, sward, kinds
//	climate/          mean, coldest, warmest, rain, runoff, rain_warm, koppen
//	strata/           count; top, rock, formed, sand by bed
//	book/             lift, worn, plate_a, plate_b, meeting, epoch,
//	                  burial, buried_in                            (made worlds)
//	features/         belt, basin, lake, plate, climate: each tile's feature id
//	features/table/   one entry a feature, dimension "feature", coordinate id
//
// Elements are kept in the types the world keeps them in, so what is read
// back is the world's own number and not a rounding of it. The attributes
// are CF's, as far as CF goes cheaply: long_name and units on every array;
// flag_values and flag_meanings on every coded one, over every code the
// world can hold and not only the ones this world does; scale_factor where
// the world keeps a share or an amount as a whole number, so that xarray
// reads the amount; and _FillValue where an integer array has elements that
// are no value at all. See README.md.

// options is how the arrays are cut and compressed.
type options struct {
	// Chunk is tiles along a side of a chunk, and Shard chunks along a side
	// of a shard (0, unsharded).
	Chunk, Shard int
	// Compress is what each chunk is compressed with - "zstd", "gzip", or
	// "none" for not at all - and Level the level it compresses at: 1 to 22
	// for zstd, 0 to 9 for gzip.
	Compress string
	Level    int
}

// levels is what each compressor takes, and what it is given when no level
// is asked for. zstd's own default is 3; gzip's is what the export wrote
// before it had a choice.
var levels = map[string]struct{ min, max, dflt int }{
	"zstd": {1, 22, 3},
	"gzip": {0, 9, 5},
	"none": {0, 0, 0},
}

func (o options) check() error {
	if o.Chunk <= 0 || o.Shard < 0 {
		return fmt.Errorf("chunk %d, shard %d: want a chunk above 0 and a shard of 0 or more", o.Chunk, o.Shard)
	}
	l, ok := levels[o.Compress]
	if !ok {
		return fmt.Errorf("compress %q: want zstd, gzip or none", o.Compress)
	}
	if o.Compress == "none" {
		if o.Level != 0 {
			return fmt.Errorf("compress none, level %d: nothing is compressed, so there is no level", o.Level)
		}
		return nil
	}
	if o.Level < l.min || o.Level > l.max {
		return fmt.Errorf("compress %s, level %d: want a level from %d to %d", o.Compress, o.Level, l.min, l.max)
	}
	return nil
}

// exporter writes one land's arrays.
type exporter struct {
	ctx   context.Context
	land  *terra.Land
	g     *terra.Grid
	o     options
	jobs  []func() error
	nodes []string // the path of every group and array, in the order queued
	// bytes is what the job of each index holds while it runs, where it
	// is known; see run.
	bytes map[int]int64
}

// export writes land into store, and says how many arrays it wrote. The
// arrays are written side by side; each is a set of keys of its own, and
// what is written does not depend on the order they finish in. The root's
// metadata is written again last, with every node's in it.
func export(ctx context.Context, land *terra.Land, store zarr.Store, o options) (int, error) {
	return exportStage(ctx, land, land.Grid, "cover", store, o)
}

// exportStage is export of g, the grid land has as the stage named ends (see
// terra.StageWatch), which may be part-way through its making. A group the
// grid has nothing for yet is left out: the climate before the coast stage,
// the features before the cover's end, and the strata and the book where
// the map has none.
func exportStage(ctx context.Context, land *terra.Land, g *terra.Grid, stage string, store zarr.Store, o options) (int, error) {
	if err := o.check(); err != nil {
		return 0, err
	}
	// terms is a JSON string, and wrap a number, because xarray will not
	// write an attribute that is a map or a bool to netCDF.
	terms, err := json.Marshal(land.Terms)
	if err != nil {
		return 0, err
	}
	root, err := zarr.CreateGroup(ctx, store, "", map[string]any{
		"generator":  "terra cmd/zarr",
		"stage":      stage,
		"seed":       land.Seed(),
		"terms":      string(terms),
		"width":      g.W,
		"height":     g.H,
		"wrap":       boolInt(g.Wrap),
		"sea_level":  g.SeaLevel(),
		"chunk_side": terra.ChunkSide,
		"tile_span":  terra.TileSpan,
		"layout":     "tiles are row-major: y is the row of the grid and x its column, and a tile's centre is at (x, y) metres",
	})
	if err != nil {
		return 0, err
	}
	e := &exporter{ctx: ctx, land: land, g: g, o: o}
	groups := map[string]*zarr.Group{}
	group := func(name string, attrs map[string]any) *zarr.Group {
		if err == nil && groups[name] == nil {
			groups[name], err = root.CreateGroup(ctx, name, attrs)
			e.nodes = append(e.nodes, name)
		}
		return groups[name]
	}
	map2 := func(name string, attrs map[string]any) *zarr.Group {
		grp := group(name, attrs)
		if err == nil {
			e.coordinates(grp)
		}
		return grp
	}
	e.ground(map2("ground", map[string]any{"about": "the land itself: its height, its water and its soil"}))
	e.tiles(map2("tile", map[string]any{"about": "what each tile is, and what time has made of its soil"}))
	e.layers(map2("layers", map[string]any{"about": "the ground that changes by the day, as it stood when the world was made"}))
	if hasYear(g) {
		e.climate(map2("climate", map[string]any{"about": "the year on each tile"}))
	}
	if beds := g.AppendBeds(nil, 0); len(beds) > 0 {
		e.strata(map2("strata", map[string]any{"about": "the pile of beds under each tile, from the top down", "beds_max": terra.BedsMax}))
	}
	if _, ok := g.Record(0); ok {
		e.book(map2("book", map[string]any{"about": "the meeting that did most to each tile's height, and its last burial, in the history's metres"}))
	}
	if f := g.Features(); f != nil {
		features := map2("features", map[string]any{"about": "the features each tile belongs to, by id; 0 is none, and the id is the feature coordinate of table"})
		e.features(features, f, group("features/table", map[string]any{"about": "every feature, one entry each"}))
	}
	if err != nil {
		return 0, err
	}
	if err := e.run(); err != nil {
		return 0, err
	}
	return len(e.jobs), consolidate(ctx, store, e.nodes)
}

// hasYear reports whether g's climate is written down: YearAt is nothing on
// every tile until the coast stage writes it.
func hasYear(g *terra.Grid) bool {
	for i := range g.Tiles {
		if m, c, w := g.YearAt(i); m != 0 || c != 0 || w != 0 {
			return true
		}
	}
	return false
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
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

// holding says that the next job queued holds about bytes while it runs.
func (e *exporter) holding(bytes int64) {
	if e.bytes == nil {
		e.bytes = map[int]int64{}
	}
	e.bytes[len(e.jobs)] = bytes
}

// stored is how many elements an object in the store holds, a shard or an
// unsharded chunk, for per elements a tile: what zarr.Write fills and
// encodes at once.
func (e *exporter) stored(per int) int64 {
	side := int64(e.o.Chunk * max(e.o.Shard, 1))
	return side * side * int64(per)
}

// consolidate writes the root's metadata again with every node's metadata
// inline in it, the way zarr-python consolidates a version 3 store, so that
// xarray opens the store from one key without warning that it had to look
// for the rest. It is an extension the specification does not have yet, and
// says it need not be understood; a reader that does not know it reads each
// node's own zarr.json, which is still there.
func consolidate(ctx context.Context, store zarr.Store, nodes []string) error {
	metadata := make(map[string]json.RawMessage, len(nodes))
	for _, path := range nodes {
		b, err := store.Get(ctx, path+"/zarr.json")
		if err != nil {
			return err
		}
		metadata[path] = b
	}
	b, err := store.Get(ctx, "zarr.json")
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		return err
	}
	doc["consolidated_metadata"] = map[string]any{"kind": "inline", "must_understand": false, "metadata": metadata}
	if b, err = json.MarshalIndent(doc, "", "  "); err != nil {
		return err
	}
	return store.Set(ctx, "zarr.json", b)
}

// codes is what each value of a coded array means.
type codes struct {
	values   []int
	meanings []string
}

// flagMeanings is meanings as CF has them: one string, a word for each
// value, with the spaces inside a meaning made underscores.
func (c codes) flagMeanings() string {
	words := make([]string, len(c.meanings))
	for i, m := range c.meanings {
		words[i] = strings.ReplaceAll(m, " ", "_")
	}
	return strings.Join(words, " ")
}

// enum is the codes of a type with count values from 0, named by name.
func enum[T ~uint8](count int, name func(T) string) codes {
	var c codes
	for v := range count {
		c.values = append(c.values, v)
		c.meanings = append(c.meanings, name(T(v)))
	}
	return c
}

// koppenTypes is every Köppen-Geiger type the world gives dry ground, in
// the order Köppen lists them. The code of a type is its place here plus
// one, 0 being water, and is the same in every world: a type is added at
// the end, never in between.
var koppenTypes = []string{
	"Af", "Am", "Aw",
	"BWh", "BWk", "BSh", "BSk",
	"Csa", "Csb", "Csc", "Cwa", "Cwb", "Cwc", "Cfa", "Cfb", "Cfc",
	"Dsa", "Dsb", "Dsc", "Dwa", "Dwb", "Dwc", "Dfa", "Dfb", "Dfc",
	"ET", "EF",
}

var koppenCodes = codes{
	values:   tiles(len(koppenTypes)+1, func(i int) int { return i }),
	meanings: append([]string{"water"}, koppenTypes...),
}

// field is one array to write.
type field struct {
	name, units, about string
	codes              *codes
	// fill is the store's fill_value, and missing the value of an integer
	// array that is no value, which xarray reads as NaN. A float array's
	// fill is NaN and needs no missing.
	fill, missing any
	// scale is what an element is multiplied by to read the amount units
	// are of: 0 is 1.
	scale float64
	// comment says anything more.
	comment string
}

func (f field) attrs() map[string]any {
	a := map[string]any{"long_name": f.about}
	if f.units != "" {
		a["units"] = f.units
	}
	if f.codes != nil {
		a["flag_values"] = f.codes.values
		a["flag_meanings"] = f.codes.flagMeanings()
	}
	if f.missing != nil {
		a["_FillValue"] = f.missing
	}
	if f.scale != 0 {
		a["scale_factor"] = f.scale
	}
	if f.comment != "" {
		a["comment"] = f.comment
	}
	return a
}

// codecs is the chunk codecs of every array.
func (e *exporter) codecs() []zarr.Codec {
	codecs := []zarr.Codec{zarr.BytesCodec{Endian: zarr.Little}}
	switch e.o.Compress {
	case "zstd":
		codecs = append(codecs, zstd.Codec{Level: e.o.Level})
	case "gzip":
		codecs = append(codecs, zarr.GzipCodec{Level: e.o.Level})
	}
	return codecs
}

// arrayOptions is how an array of shape is made: chunked and sharded along
// the map's two dimensions, with any dimension after them whole in a chunk.
func (e *exporter) arrayOptions(shape []int, dims []string, f field, d zarr.DataType) zarr.ArrayOptions {
	chunk := slices.Clone(shape)
	chunk[0], chunk[1] = e.o.Chunk, e.o.Chunk
	o := zarr.ArrayOptions{Shape: shape, ChunkShape: chunk, DataType: d, FillValue: f.fill, Codecs: e.codecs(), DimensionNames: dims, Attributes: f.attrs()}
	if e.o.Shard > 0 {
		o.ShardShape = append([]int{e.o.Chunk * e.o.Shard, e.o.Chunk * e.o.Shard}, chunk[2:]...)
	}
	return o
}

// queue queues a job that writes the array name of grp.
func (e *exporter) queue(grp *zarr.Group, name string, job func() error) {
	e.nodes = append(e.nodes, grp.Path()+"/"+name)
	e.jobs = append(e.jobs, job)
}

// put queues an array of the map, H by W, whose elements data makes.
// It holds the map's elements and a shard's twice, filled and encoded.
func put[T zarr.Element](e *exporter, grp *zarr.Group, f field, data func() []T) {
	e.holding(int64(zarr.DataTypeOf[T]().Size()) * (int64(len(e.g.Tiles)) + 2*e.stored(1)))
	e.queue(grp, f.name, func() error {
		return write(e, grp, f, []int{e.g.H, e.g.W}, []string{"y", "x"}, data())
	})
}

// putBeds queues an array of the map by bed, H by W by BedsMax, whose
// element for a bed pick makes and for a bed past the tile's pile is empty.
// An array of the beds is BedsMax times the map, so none is built whole: it
// is written a shard at a time from the piles, and holds a shard three
// times over, built, filled and encoded.
func putBeds[T zarr.Element](e *exporter, grp *zarr.Group, f field, empty T, pick func(terra.Bed) T) {
	g, per := e.g, terra.BedsMax
	e.holding(3 * int64(zarr.DataTypeOf[T]().Size()) * e.stored(per))
	e.queue(grp, f.name, func() error {
		a, err := grp.CreateArray(e.ctx, f.name, e.arrayOptions([]int{g.H, g.W, per}, []string{"y", "x", "bed"}, f, zarr.DataTypeOf[T]()))
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

// put1 queues an array of one dimension, cut into chunks of 1<<14.
func put1[T zarr.Element](e *exporter, grp *zarr.Group, f field, dim string, data []T) {
	e.queue(grp, f.name, func() error {
		a, err := grp.CreateArray(e.ctx, f.name, zarr.ArrayOptions{
			Shape: []int{len(data)}, ChunkShape: []int{max(1, min(len(data), 1<<14))}, DataType: zarr.DataTypeOf[T](),
			FillValue: f.fill, Codecs: e.codecs(), DimensionNames: []string{dim}, Attributes: f.attrs(),
		})
		if err != nil {
			return fmt.Errorf("%s/%s: %w", grp.Path(), f.name, err)
		}
		return zarr.Write(e.ctx, a, nil, nil, data)
	})
}

// coordinates queues the coordinates of a group of the map: y and x, the
// metres of each row and column's centre from the centre of tile (0, 0),
// and bed for the strata.
func (e *exporter) coordinates(grp *zarr.Group) {
	g := e.g
	span := func(n int) []float64 { return tiles(n, func(i int) float64 { return float64(i) * terra.TileSpan }) }
	put1(e, grp, field{name: "y", units: "m", about: "distance of the row's centre from the centre of row 0, along the rows",
		comment: fmt.Sprintf("row index times the tile span, %g m", terra.TileSpan)}, "y", span(g.H))
	x := field{name: "x", units: "m", about: "distance of the column's centre from the centre of column 0, along the columns",
		comment: fmt.Sprintf("column index times the tile span, %g m", terra.TileSpan)}
	if g.Wrap {
		x.comment += fmt.Sprintf("; the map wraps, and column %d is column 0 again", g.W)
	}
	put1(e, grp, x, "x", span(g.W))
	if grp.Path() == "strata" {
		put1(e, grp, field{name: "bed", about: "place of the bed in the pile, from the top down"}, "bed",
			tiles(terra.BedsMax, func(i int) uint8 { return uint8(i) }))
	}
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

var (
	terrainCodes = enum(int(terra.TerrainCount), terra.Terrain.String)
	bedrockCodes = enum(int(terra.BedrockCount), terra.Bedrock.String)
	meetingCodes = enum(int(terra.Hotspot)+1, terra.MeetingKind.String)
	burialCodes  = enum(int(terra.BuriedByLava)+1, terra.Burial.String)
	kindCodes    = enum(int(terra.ClimateRegion)+1, terra.FeatureKind.String)
	groupCodes   = codes{values: []int{0, 'A', 'B', 'C', 'D', 'E'}, meanings: []string{"none", "A", "B", "C", "D", "E"}}
)

func (e *exporter) ground(grp *zarr.Group) {
	g := e.g
	put(e, grp, field{name: "height", units: "m", about: "height above the lowest ground on the map"}, same(g.Height))
	put(e, grp, field{name: "flow", units: "m3 s-1", about: "the water running through the tile"}, same(g.Flow))
	put(e, grp, field{name: "drain", units: "m", about: "how far the tile stands above the water it drains into"}, same(g.Drain))
	put(e, grp, field{name: "soil", units: "m", about: "soil over the rock"}, same(g.Soil))
	put(e, grp, field{name: "sand", units: "1", about: "share of the soil that is sand"}, same(g.Sand))
	put(e, grp, field{name: "clay", units: "1", about: "share of the soil that is clay; the rest of sand and clay is silt"}, same(g.Clay))
	n := len(g.Tiles)
	put(e, grp, field{name: "floor_age", units: "Myr", about: "how old the ocean crust is; NaN on continental crust or a map with no watered history"},
		func() []float64 { return tiles(n, g.FloorAge) })
	put(e, grp, field{name: "floor_sediment", units: "m", about: "the beds over the ocean crust's basalt; NaN on continental crust"},
		func() []float64 { return tiles(n, g.FloorSediment) })
}

func (e *exporter) tiles(grp *zarr.Group) {
	g, n := e.g, len(e.g.Tiles)
	put(e, grp, field{name: "terrain", about: "what the ground is", codes: &terrainCodes},
		func() []uint8 { return tiles(n, func(i int) uint8 { return uint8(g.Tiles[i].Terrain) }) })
	put(e, grp, field{name: "bedrock", about: "the rock at the surface, under the soil", codes: &bedrockCodes},
		func() []uint8 { return tiles(n, func(i int) uint8 { return uint8(g.Tiles[i].Bedrock) }) })
	put(e, grp, field{name: "mark", about: "what stands on the tile, as the game that built it numbers it; 0 is nothing"},
		func() []uint8 { return tiles(n, func(i int) uint8 { return uint8(g.Tiles[i].Mark) }) })
	put(e, grp, field{name: "owner", about: "who holds the tile; 0 is nobody"},
		func() []int64 { return tiles(n, func(i int) int64 { return int64(g.Tiles[i].Owner) }) })
	put(e, grp, field{name: "fenced", about: "whether the tile lies inside a fence"},
		func() []bool { return tiles(n, func(i int) bool { return g.Tiles[i].Fenced }) })
	put(e, grp, field{name: "plate", about: "the plate number of the crust the tile rides; nothing on a drawn map"},
		func() []uint8 { return tiles(n, func(i int) uint8 { return g.Tiles[i].Plate }) })
	put(e, grp, field{name: "formed", about: "the epoch the tile's rock dates from; nothing on a drawn map"},
		func() []uint8 { return tiles(n, func(i int) uint8 { return g.Tiles[i].Formed }) })
	put(e, grp, field{name: "leached", units: "1", scale: 1.0 / 65535, about: "share of the bases the rock gave the soil that the water has carried off"},
		func() []uint16 { return tiles(n, func(i int) uint16 { return g.Tiles[i].Leached }) })
	put(e, grp, field{name: "exposed", units: "year", about: "how long the surface has been forming soil"},
		func() []float32 { return tiles(n, func(i int) float32 { return g.Tiles[i].Exposed }) })
	put(e, grp, field{name: "lime", units: "kg m-2", scale: 0.01, about: "carbonate the dry years have left in the soil"},
		func() []uint16 { return tiles(n, func(i int) uint16 { return g.Tiles[i].Lime }) })
	put(e, grp, field{name: "salt", units: "kg m-2", scale: 0.001, about: "salt the dry years have left in the soil"},
		func() []uint16 { return tiles(n, func(i int) uint16 { return g.Tiles[i].Salt }) })
	put(e, grp, field{name: "carbon", units: "kg m-2", about: "organic carbon in the soil"},
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
	put(e, grp, field{name: "rain", units: "mm year-1", about: "rain in a year"}, func() []float64 { return tiles(n, g.Rain) })
	put(e, grp, field{name: "runoff", units: "mm year-1", about: "rain in a year that runs off"}, func() []float64 { return tiles(n, g.Runoff) })
	put(e, grp, field{name: "rain_warm", units: "1", about: "share of the year's rain that falls in the warmer half year"}, func() []float64 { return tiles(n, g.RainWarm) })

	// The Köppen type of the dry ground, coded by koppenTypes, which is the
	// same table in every world.
	f := field{name: "koppen", about: "the Köppen-Geiger type of the dry ground", codes: &koppenCodes}
	e.queue(grp, f.name, func() error {
		code := make(map[string]uint8, len(koppenTypes))
		for c, name := range koppenTypes {
			code[name] = uint8(c + 1)
		}
		data := make([]uint8, n)
		for i := range data {
			if g.Tiles[i].Wet() {
				continue
			}
			k := g.Koppen(g.PosOf(i))
			if data[i] = code[k]; data[i] == 0 {
				return fmt.Errorf("climate/koppen: tile %d is %q, which the table of Köppen types does not have", i, k)
			}
		}
		return write(e, grp, f, []int{g.H, g.W}, []string{"y", "x"}, data)
	})
}

// noBed is what rock and formed hold past the bottom of a pile.
const noBed = math.MaxUint8

func (e *exporter) strata(grp *zarr.Group) {
	g, n := e.g, len(e.g.Tiles)
	var pile []terra.Bed
	var err error
	for i := 0; i < n && err == nil; i++ {
		pile = g.AppendBeds(pile[:0], i)
		for k, b := range pile {
			if b.Formed == noBed {
				err = fmt.Errorf("strata/formed: tile %d bed %d was laid in epoch %d, which the store keeps for no bed", i, k, b.Formed)
				break
			}
		}
	}
	put(e, grp, field{name: "count", about: "how many beds the pile holds; beds past it are fill"},
		func() []uint8 {
			var pile []terra.Bed
			return tiles(n, func(i int) uint8 { pile = g.AppendBeds(pile[:0], i); return uint8(len(pile)) })
		})
	nan32 := float32(math.NaN())
	missing := uint8(noBed)
	putBeds(e, grp, field{name: "top", units: "m", about: "height of the bed's upper surface, in the ground's metres", fill: nan32}, nan32,
		func(b terra.Bed) float32 { return float32(b.Top) })
	putBeds(e, grp, field{name: "rock", about: "the rock of the bed", codes: &bedrockCodes, fill: missing, missing: missing}, missing,
		func(b terra.Bed) uint8 { return uint8(b.Rock) })
	putBeds(e, grp, field{name: "formed", about: "the epoch the bed was laid in", fill: missing, missing: missing}, missing,
		func(b terra.Bed) uint8 { return b.Formed })
	putBeds(e, grp, field{name: "sand", units: "1", scale: 1.0 / 255, about: "share of sand in a bed the water laid",
		comment: "0 past the bottom of the pile, where 0 is also a bed without sand: take the beds from count or top"}, 0,
		func(b terra.Bed) uint8 { return b.Sand })
	if err != nil {
		e.jobs = append(e.jobs, func() error { return err })
	}
}

func (e *exporter) book(grp *zarr.Group) {
	g, n := e.g, len(e.g.Tiles)
	records := tiles(n, func(i int) terra.Record { r, _ := g.Record(i); return r })
	history := "in the history's metres, which are the metres of a tile of the history and not of the map"
	put(e, grp, field{name: "lift", units: "m", comment: history, about: "what the meeting that did most to the tile's height raised it by in its epoch; negative where a rift dropped it"},
		func() []float32 { return tiles(n, func(i int) float32 { return float32(records[i].Lift) }) })
	put(e, grp, field{name: "worn", units: "m", comment: history, about: "what the weather has taken off the tile since, to the nearest ten metres"},
		func() []float32 { return tiles(n, func(i int) float32 { return float32(records[i].Worn) }) })
	put(e, grp, field{name: "plate_a", about: "the first plate of the meeting, as numbered in its epoch"},
		func() []uint8 { return tiles(n, func(i int) uint8 { return records[i].Plates[0] }) })
	put(e, grp, field{name: "plate_b", about: fmt.Sprintf("the second plate of the meeting, as numbered in its epoch; %d where a hotspot raised the tile from under one plate", terra.NoPlate)},
		func() []uint8 { return tiles(n, func(i int) uint8 { return records[i].Plates[1] }) })
	put(e, grp, field{name: "meeting", about: "what kind of meeting it was", codes: &meetingCodes},
		func() []uint8 { return tiles(n, func(i int) uint8 { return uint8(records[i].Meeting) }) })
	put(e, grp, field{name: "epoch", about: "the epoch of the meeting"},
		func() []uint8 { return tiles(n, func(i int) uint8 { return records[i].Epoch }) })
	put(e, grp, field{name: "burial", about: "what last buried the tile", codes: &burialCodes},
		func() []uint8 { return tiles(n, func(i int) uint8 { return uint8(records[i].Burial) }) })
	put(e, grp, field{name: "buried_in", about: "the epoch of the last burial"},
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
		put(e, grp, field{name: k.name, about: "the " + k.kind.String() + " the tile belongs to, by feature id; 0 is none"},
			func() []int32 { return tiles(n, func(i int) int32 { return int32(g.FeatureOf(i, k.kind)) }) })
	}

	all := f.All
	col := func(at func(i int) int32) []int32 { return tiles(len(all), at) }
	put1(e, table, field{name: "feature", about: "the feature's id, as the map's arrays of features give it"}, "feature", col(func(i int) int32 { return int32(all[i].ID) }))
	put1(e, table, field{name: "kind", about: "what kind of feature it is", codes: &kindCodes}, "feature", tiles(len(all), func(i int) uint8 { return uint8(all[i].Kind) }))
	put1(e, table, field{name: "count", about: "how many tiles the feature has"}, "feature", tiles(len(all), func(i int) int64 { return int64(all[i].Count) }))
	put1(e, table, field{name: "first", about: "the lowest tile of the feature, y*width+x"}, "feature", col(func(i int) int32 { return all[i].First }))
	put1(e, table, field{name: "plate_a", about: "a belt's first plate"}, "feature", tiles(len(all), func(i int) uint8 { return all[i].Plates[0] }))
	put1(e, table, field{name: "plate_b", about: "a belt's second plate"}, "feature", tiles(len(all), func(i int) uint8 { return all[i].Plates[1] }))
	put1(e, table, field{name: "meeting", about: "a belt's kind of meeting", codes: &meetingCodes}, "feature", tiles(len(all), func(i int) uint8 { return uint8(all[i].Meeting) }))
	put1(e, table, field{name: "epoch", about: "a belt's epoch"}, "feature", tiles(len(all), func(i int) uint8 { return all[i].Epoch }))
	put1(e, table, field{name: "lift", units: "m", comment: "in the history's metres", about: "the most a belt's meeting raised any tile of it"}, "feature", tiles(len(all), func(i int) float64 { return all[i].Lift }))
	put1(e, table, field{name: "top", about: "a belt's highest tile, y*width+x"}, "feature", col(func(i int) int32 { return all[i].Top }))
	put1(e, table, field{name: "height", units: "m", about: "a belt's highest tile's height"}, "feature", tiles(len(all), func(i int) float64 { return all[i].Height }))
	put1(e, table, field{name: "outlet", about: "the tile a basin's water leaves by, y*width+x"}, "feature", col(func(i int) int32 { return all[i].Outlet }))
	put1(e, table, field{name: "flow", units: "m3 s-1", about: "the water at a basin's outlet"}, "feature", tiles(len(all), func(i int) float64 { return all[i].Flow }))
	put1(e, table, field{name: "lake", about: "a lake's index among the map's lakes"}, "feature", col(func(i int) int32 { return all[i].Lake }))
	put1(e, table, field{name: "number", about: "a plate's number"}, "feature", tiles(len(all), func(i int) uint8 { return all[i].Number }))
	put1(e, table, field{name: "group", about: "a climate region's Köppen letter, as its character code", codes: &groupCodes}, "feature", tiles(len(all), func(i int) uint8 { return all[i].Group }))
}
