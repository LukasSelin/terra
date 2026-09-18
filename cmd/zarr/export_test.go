package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/zarr"
)

var ctx = context.Background()

// Small chunks and shards, so that a small world still has many of each and
// shards that run past the edge of the map.
var small = options{Chunk: 16, Shard: 2, Compress: "zstd", Level: 1}

func read[T zarr.Element](t *testing.T, s zarr.Store, path string) []T {
	t.Helper()
	a, err := zarr.OpenArray(ctx, s, path)
	if err != nil {
		t.Fatal(err)
	}
	v, err := zarr.Read[T](ctx, a, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// legend is what each code of a coded array means, read from its CF flags
// with the underscores made spaces again.
func legend(t *testing.T, s zarr.Store, path string) map[int]string {
	t.Helper()
	a, err := zarr.OpenArray(ctx, s, path)
	if err != nil {
		t.Fatal(err)
	}
	var values []int
	var meanings string
	if ok, err := a.Attribute("flag_values", &values); !ok || err != nil {
		t.Fatalf("%s: no flag_values: %v", path, err)
	}
	if ok, err := a.Attribute("flag_meanings", &meanings); !ok || err != nil {
		t.Fatalf("%s: no flag_meanings: %v", path, err)
	}
	words := strings.Fields(meanings)
	if len(words) != len(values) {
		t.Fatalf("%s: %d flag values and %d meanings", path, len(values), len(words))
	}
	l := map[int]string{}
	for i, v := range values {
		l[v] = strings.ReplaceAll(words[i], "_", " ")
	}
	return l
}

func exported(t *testing.T, land *terra.Land, o options) *zarr.MemoryStore {
	t.Helper()
	s := zarr.NewMemoryStore()
	if _, err := export(ctx, land, s, o); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAMadeWorldReadsBackAsItIs(t *testing.T) {
	land := terra.NewLand(1, terra.AncientTerms())
	g, n := land.Grid, len(land.Grid.Tiles)
	s := exported(t, land, small)

	root, err := zarr.OpenGroup(ctx, s, "")
	if err != nil {
		t.Fatal(err)
	}
	var seed uint64
	var raw string
	var terms terra.Terms
	root.Attribute("seed", &seed)
	root.Attribute("terms", &raw)
	if err := json.Unmarshal([]byte(raw), &terms); err != nil {
		t.Error(err)
	}
	if seed != 1 || terms != land.Terms {
		t.Errorf("seed %d, terms %+v", seed, terms)
	}
	h, err := zarr.OpenArray(ctx, s, "ground/height")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(h.Shape(), []int{g.H, g.W}) || !slices.Equal(h.DimensionNames(), []string{"y", "x"}) || !slices.Equal(h.ShardShape(), []int{32, 32}) {
		t.Errorf("height is %v %v in shards of %v", h.Shape(), h.DimensionNames(), h.ShardShape())
	}

	if !slices.Equal(read[float64](t, s, "ground/height"), g.Height) ||
		!slices.Equal(read[float64](t, s, "ground/flow"), g.Flow) ||
		!slices.Equal(read[float32](t, s, "ground/soil"), g.Soil) ||
		!slices.Equal(read[float64](t, s, "layers/sward"), g.Sward) ||
		!slices.Equal(read[int64](t, s, "layers/kinds"), g.Kinds) {
		t.Error("the ground read back is not the world's")
	}

	terrain, bedrock, carbon := read[uint8](t, s, "tile/terrain"), read[uint8](t, s, "tile/bedrock"), read[float32](t, s, "tile/carbon")
	terrains, rocks := legend(t, s, "tile/terrain"), legend(t, s, "tile/bedrock")
	for i := range n {
		tile := g.Tiles[i]
		if terra.Terrain(terrain[i]) != tile.Terrain || terrains[int(terrain[i])] != tile.Terrain.String() ||
			terra.Bedrock(bedrock[i]) != tile.Bedrock || rocks[int(bedrock[i])] != tile.Bedrock.String() || carbon[i] != tile.Carbon {
			t.Fatalf("tile %d read back as %v, %v, %v", i, terrain[i], bedrock[i], carbon[i])
		}
	}

	koppen, types := read[uint8](t, s, "climate/koppen"), legend(t, s, "climate/koppen")
	mean := read[float64](t, s, "climate/mean")
	for i := range n {
		want := "water"
		if !g.Tiles[i].Wet() {
			want = g.Koppen(g.PosOf(i))
		}
		if types[int(koppen[i])] != want {
			t.Fatalf("tile %d is %q, not %q", i, types[int(koppen[i])], want)
		}
		if m, _, _ := g.YearAt(i); mean[i] != m {
			t.Fatalf("tile %d's mean is %v, not %v", i, mean[i], m)
		}
	}

	count, top, rock := read[uint8](t, s, "strata/count"), read[float32](t, s, "strata/top"), read[uint8](t, s, "strata/rock")
	var beds []terra.Bed
	for i := range n {
		beds = g.AppendBeds(beds[:0], i)
		if int(count[i]) != len(beds) {
			t.Fatalf("tile %d has %d beds, not %d", i, count[i], len(beds))
		}
		for k := range terra.BedsMax {
			at := i*terra.BedsMax + k
			if k < len(beds) && (top[at] != float32(beds[k].Top) || terra.Bedrock(rock[at]) != beds[k].Rock) {
				t.Fatalf("tile %d bed %d is %v %v, not %+v", i, k, top[at], rock[at], beds[k])
			}
			if k >= len(beds) && !math.IsNaN(float64(top[at])) {
				t.Fatalf("tile %d bed %d past its pile has a top of %v", i, k, top[at])
			}
		}
	}

	lift, meeting, meetings := read[float32](t, s, "book/lift"), read[uint8](t, s, "book/meeting"), legend(t, s, "book/meeting")
	for i := range n {
		r, _ := g.Record(i)
		if lift[i] != float32(r.Lift) || meetings[int(meeting[i])] != r.Meeting.String() {
			t.Fatalf("tile %d's record read back as %v %v, not %+v", i, lift[i], meeting[i], r)
		}
	}

	f := g.Features()
	basin, plate := read[int32](t, s, "features/basin"), read[int32](t, s, "features/plate")
	kinds, counts := read[uint8](t, s, "features/table/kind"), read[int64](t, s, "features/table/count")
	if len(kinds) != len(f.All) || len(f.All) == 0 {
		t.Fatalf("%d features in the table, %d in the world", len(kinds), len(f.All))
	}
	for i := range n {
		if basin[i] != int32(g.FeatureOf(i, terra.DrainageBasin)) || plate[i] != int32(g.FeatureOf(i, terra.CrustPlate)) {
			t.Fatalf("tile %d is in basin %d and plate %d", i, basin[i], plate[i])
		}
		if id := basin[i]; id > 0 && terra.FeatureKind(kinds[id-1]) != terra.DrainageBasin {
			t.Fatalf("tile %d's basin %d is a %v in the table", i, id, terra.FeatureKind(kinds[id-1]))
		}
	}
	for i, ft := range f.All {
		if int64(ft.Count) != counts[i] {
			t.Fatalf("feature %d has %d tiles, the table %d", i+1, ft.Count, counts[i])
		}
	}
}

func TestADrawnWorldHasNoBook(t *testing.T) {
	land := terra.NewLand(1, terra.DefaultTerms())
	s := exported(t, land, options{Chunk: 64, Shard: 0, Compress: "none"})
	if _, err := zarr.OpenArray(ctx, s, "book/lift"); err == nil {
		t.Error("a drawn world has a book")
	}
	if !slices.Equal(read[float64](t, s, "ground/height"), land.Grid.Height) {
		t.Error("height read back wrong")
	}
	a, _ := zarr.OpenArray(ctx, s, "ground/height")
	if a.ShardShape() != nil {
		t.Error("sharded with -shard 0")
	}
}

// The same world writes the same store, byte for byte, however many
// goroutines the arrays are written over.
func TestTheSameWorldWritesTheSameStore(t *testing.T) {
	land := terra.NewLand(3, terra.AncientTerms())
	one := func(procs int) *zarr.MemoryStore {
		defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(procs))
		return exported(t, land, small)
	}
	a, b := one(1), one(8)
	keys := a.Keys()
	if !slices.Equal(keys, b.Keys()) {
		t.Fatalf("%d keys, then %d", len(keys), len(b.Keys()))
	}
	for _, k := range keys {
		va, _ := a.Get(ctx, k)
		vb, _ := b.Get(ctx, k)
		if !bytes.Equal(va, vb) {
			t.Errorf("%s differs", k)
		}
	}
}

// Every type the world can give dry ground has a code in the one table, so
// that a code means the same type in every world.
func TestEveryKoppenTypeHasACode(t *testing.T) {
	have := map[string]bool{}
	for _, k := range koppenTypes {
		if have[k] {
			t.Errorf("%s is in the table twice", k)
		}
		have[k] = true
	}
	given := map[string]bool{}
	for _, ice := range []bool{false, true} {
		for hot := -10.0; hot <= 40; hot += 1 {
			for cold := hot - 50; cold <= hot; cold += 1 {
				for rain := 0.0; rain <= 4000; rain += 50 {
					for warm := 0.0; warm <= 1; warm += 0.05 {
						given[terra.KoppenOf((hot+cold)/2, cold, hot, rain, warm, ice)] = true
					}
				}
			}
		}
	}
	for k := range given {
		if !have[k] {
			t.Errorf("the world gives %s, which has no code", k)
		}
	}
	if len(given) != len(koppenTypes) {
		t.Logf("the sweep gave %d types of the table's %d", len(given), len(koppenTypes))
	}
}

func TestAStoreHasCoordinatesAndConsolidatedMetadata(t *testing.T) {
	land := terra.NewLand(1, terra.AncientTerms())
	g := land.Grid
	s := exported(t, land, small)
	for _, grp := range []string{"ground", "tile", "layers", "climate", "strata", "book", "features"} {
		y, x := read[float64](t, s, grp+"/y"), read[float64](t, s, grp+"/x")
		if len(y) != g.H || len(x) != g.W || y[1] != terra.TileSpan || x[g.W-1] != float64(g.W-1)*terra.TileSpan {
			t.Errorf("%s: y %d long, x %d long, y[1] %v", grp, len(y), len(x), y[1])
		}
	}
	if bed := read[uint8](t, s, "strata/bed"); len(bed) != terra.BedsMax || bed[terra.BedsMax-1] != uint8(terra.BedsMax-1) {
		t.Errorf("bed is %v", bed)
	}
	ids := read[int32](t, s, "features/table/feature")
	if len(ids) != len(g.Features().All) || ids[0] != 1 || ids[len(ids)-1] != int32(len(ids)) {
		t.Errorf("feature ids run %d to %d over %d", ids[0], ids[len(ids)-1], len(ids))
	}

	rock, count := read[uint8](t, s, "strata/rock"), read[uint8](t, s, "strata/count")
	for i := range count {
		for k := int(count[i]); k < terra.BedsMax; k++ {
			if rock[i*terra.BedsMax+k] != noBed {
				t.Fatalf("tile %d bed %d past its pile is rock %d", i, k, rock[i*terra.BedsMax+k])
			}
		}
	}

	b, err := s.Get(ctx, "zarr.json")
	if err != nil {
		t.Fatal(err)
	}
	var root struct {
		Consolidated struct {
			Kind     string                     `json:"kind"`
			Metadata map[string]json.RawMessage `json:"metadata"`
		} `json:"consolidated_metadata"`
	}
	if err := json.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	for _, k := range s.Keys() {
		node, ok := strings.CutSuffix(k, "/zarr.json")
		if !ok {
			continue
		}
		own, _ := s.Get(ctx, k)
		var a, c any
		json.Unmarshal(own, &a)
		json.Unmarshal(root.Consolidated.Metadata[node], &c)
		if !reflect.DeepEqual(a, c) {
			t.Errorf("%s: the consolidated metadata is not the node's", node)
		}
	}
	if n := len(root.Consolidated.Metadata); root.Consolidated.Kind != "inline" || n == 0 {
		t.Errorf("consolidated %q with %d nodes", root.Consolidated.Kind, n)
	}
	if _, err := zarr.OpenGroup(ctx, s, ""); err != nil {
		t.Errorf("the consolidated root does not open: %v", err)
	}
}
