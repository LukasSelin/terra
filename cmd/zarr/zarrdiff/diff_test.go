package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LukasSelin/zarr"
)

var ctx = context.Background()

// store is what a test store holds: a small map of H by W, cut into chunks
// of 4 and shards of 2 chunks so that both run past its edges.
type store struct {
	h, w    int
	height  []float64 // ground/height, H by W
	terrain []uint8   // tile/terrain, H by W, with a legend
	top     []float32 // strata/top, H by W by beds, NaN fill
	kind    []int32   // features/table/kind, one dimension
	missing bool      // leave tile/terrain out
	meeting []uint8   // book/meeting, H by W with CF flags; nil for none
	belt    []int32   // features/belt, H by W, ids; nil for none
}

const beds = 3

func base() *store {
	s := &store{h: 10, w: 13}
	n := s.h * s.w
	s.height = make([]float64, n)
	s.terrain = make([]uint8, n)
	s.top = make([]float32, n*beds)
	for i := range n {
		s.height[i] = float64(i%17) * 1.5
		s.terrain[i] = uint8(i % 3)
		for k := range beds {
			s.top[i*beds+k] = float32(math.NaN())
			if k <= i%beds {
				s.top[i*beds+k] = float32(i - k)
			}
		}
	}
	s.kind = []int32{1, 2, 3, 4, 5, 6, 7}
	return s
}

func (s *store) write(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "s.zarr")
	st := zarr.NewDirStore(dir)
	root, err := zarr.CreateGroup(ctx, st, "", map[string]any{"seed": 1})
	if err != nil {
		t.Fatal(err)
	}
	opts := func(shape []int, d zarr.DataType, fill any, attrs map[string]any) zarr.ArrayOptions {
		chunk, shard := slices.Clone(shape), slices.Clone(shape)
		dims := []string{"y", "x", "bed"}[:len(shape)]
		if len(shape) == 1 {
			chunk[0], shard[0], dims = 4, 8, []string{"feature"}
		} else {
			chunk[0], chunk[1], shard[0], shard[1] = 4, 4, 8, 8
		}
		return zarr.ArrayOptions{Shape: shape, ChunkShape: chunk, ShardShape: shard, DataType: d, FillValue: fill,
			Codecs: []zarr.Codec{zarr.BytesCodec{Endian: zarr.Little}, zarr.GzipCodec{Level: 1}}, DimensionNames: dims, Attributes: attrs}
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	group := func(name string) *zarr.Group {
		g, err := root.CreateGroup(ctx, name, nil)
		must(err)
		return g
	}
	a, err := group("ground").CreateArray(ctx, "height", opts([]int{s.h, s.w}, zarr.Float64, nil, map[string]any{"units": "m"}))
	must(err)
	must(zarr.Write(ctx, a, nil, nil, s.height))
	if !s.missing {
		a, err = group("tile").CreateArray(ctx, "terrain", opts([]int{s.h, s.w}, zarr.Uint8, nil,
			map[string]any{"legend": map[int]string{0: "Water", 1: "Grass", 2: "Forest", 3: "Rock"}}))
		must(err)
		must(zarr.Write(ctx, a, nil, nil, s.terrain))
	}
	a, err = group("strata").CreateArray(ctx, "top", opts([]int{s.h, s.w, beds}, zarr.Float32, float32(math.NaN()), nil))
	must(err)
	must(zarr.Write(ctx, a, nil, nil, s.top))
	if s.meeting != nil {
		a, err = group("book").CreateArray(ctx, "meeting", opts([]int{s.h, s.w}, zarr.Uint8, nil,
			map[string]any{"flag_values": []int{0, 1, 2, 3, 4, 5}, "flag_meanings": "no_meeting collision arc islands rift hotspot"}))
		must(err)
		must(zarr.Write(ctx, a, nil, nil, s.meeting))
	}
	features, err := root.CreateGroup(ctx, "features", nil)
	must(err)
	if s.belt != nil {
		a, err = features.CreateArray(ctx, "belt", opts([]int{s.h, s.w}, zarr.Int32, nil, nil))
		must(err)
		must(zarr.Write(ctx, a, nil, nil, s.belt))
	}
	tab, err := features.CreateGroup(ctx, "table", nil)
	must(err)
	a, err = tab.CreateArray(ctx, "kind", opts([]int{len(s.kind)}, zarr.Int32, nil, nil))
	must(err)
	must(zarr.Write(ctx, a, nil, nil, s.kind))
	return dir
}

// diff runs the command and reads its JSON report.
func diff(t *testing.T, want int, args ...string) *Report {
	t.Helper()
	var out, errs bytes.Buffer
	if got := run(ctx, append([]string{"-json"}, args...), &out, &errs); got != want {
		t.Fatalf("zarrdiff %v exits %d, not %d: %s", args, got, want, errs.String())
	}
	r := &Report{}
	if err := json.Unmarshal(out.Bytes(), r); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	return r
}

func text(t *testing.T, want int, args ...string) string {
	t.Helper()
	var out, errs bytes.Buffer
	if got := run(ctx, args, &out, &errs); got != want {
		t.Fatalf("zarrdiff %v exits %d, not %d: %s", args, got, want, errs.String())
	}
	return out.String()
}

func array(t *testing.T, r *Report, path string) *Array {
	t.Helper()
	for _, a := range r.Arrays {
		if a.Path == path {
			return a
		}
	}
	t.Fatalf("no %s in %+v", path, r.Arrays)
	return nil
}

func TestTheSameStoresAreTheSame(t *testing.T) {
	a, b := base().write(t), base().write(t)
	r := diff(t, same, a, b)
	if !r.Same || len(r.Arrays) != 4 || len(r.OnlyA)+len(r.OnlyB) != 0 {
		t.Errorf("%+v", r)
	}
	for _, d := range r.Arrays {
		if !d.Compared || d.Changed != 0 || d.Box != nil {
			t.Errorf("%+v", d)
		}
	}
	if got := array(t, r, "strata/top"); got.Elements != 10*13*beds || got.Tiles != 10*13 {
		t.Errorf("strata/top counts %d elements over %d tiles", got.Elements, got.Tiles)
	}
	if out := text(t, same, a, b); !strings.Contains(out, "4 arrays in both, 0 differ") || !strings.HasSuffix(out, "the same\n") {
		t.Errorf("text:\n%s", out)
	}
}

func TestOneElementChanged(t *testing.T) {
	s := base()
	a := s.write(t)
	s.height[3*13+7] += 2.5 // y 3, x 7
	s.terrain[9*13+12] = 3  // the last tile, from Water to Rock
	was := base().terrain[9*13+12]
	b := s.write(t)

	for _, budget := range []string{"1", "4194304"} {
		r := diff(t, different, "-budget", budget, a, b)
		h := array(t, r, "ground/height")
		if r.Same || h.Changed != 1 || h.TilesChanged != 1 || h.MaxAbs != 2.5 || h.MeanAbs != 2.5 ||
			*h.Box != (Box{Y0: 3, Y1: 3, X0: 7, X1: 7}) || h.Share != 1.0/130 {
			t.Errorf("budget %s: height %+v box %+v", budget, h, h.Box)
		}
		tr := array(t, r, "tile/terrain")
		want := []Code{{From: int64(was), To: 3, FromName: []string{"Water", "Grass", "Forest"}[was], ToName: "Rock", Count: 1}}
		if tr.Changed != 1 || !slices.Equal(tr.Codes, want) || *tr.Box != (Box{Y0: 9, Y1: 9, X0: 12, X1: 12}) {
			t.Errorf("budget %s: terrain %+v", budget, tr)
		}
		if top := array(t, r, "strata/top"); top.Changed != 0 {
			t.Errorf("NaN is not NaN: %+v", top)
		}
		// The most changed first: both share 1 in 130, so the order is by path.
		if r.Arrays[0].Path != "ground/height" || r.Arrays[1].Path != "tile/terrain" {
			t.Errorf("order %s, %s", r.Arrays[0].Path, r.Arrays[1].Path)
		}
	}

	out := text(t, different, a, b, "-png", filepath.Join(t.TempDir(), "where"))
	for _, line := range []string{
		"4 arrays in both, 2 differ",
		"ground/height     0.77%         2.5         2.5  3-3          7-7\n               1 of 130 tiles, 1 of 130 elements\n",
		"tile/terrain      0.77%                          9-9          12-12\n",
		"      1  0 Water -> 3 Rock\n",
		filepath.Join("where", "ground_height.png") + "\n",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("no %q in\n%s", line, out)
		}
	}
	r := diff(t, different, "-png", filepath.Join(t.TempDir(), "where"), a, b)
	f, err := os.Open(array(t, r, "ground/height").PNG)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 13 || b.Dy() != 10 {
		t.Fatalf("picture of %v", b)
	}
	for y := range 10 {
		for x := range 13 {
			r, _, _, _ := img.At(x, y).RGBA()
			if lit := r > 0; lit != (y == 3 && x == 7) {
				t.Errorf("pixel %d,%d is %d", x, y, r>>8)
			}
		}
	}
	if array(t, r, "strata/top").PNG != "" {
		t.Error("a picture of an array that did not change")
	}
}

func TestNaNAgainstANumber(t *testing.T) {
	s := base()
	a := s.write(t)
	s.top[3*beds+2] = 7            // a bed past the pile given a top
	s.top[0] = float32(math.NaN()) // and the first tile's only bed taken away
	s.top[4*beds+1] += 0.5
	b := s.write(t)

	r := diff(t, different, a, b)
	top := array(t, r, "strata/top")
	if top.Changed != 3 || top.Unmeasured != 2 || top.TilesChanged != 3 || top.MaxAbs != 0.5 || top.MeanAbs != 0.5 ||
		*top.Box != (Box{Y0: 0, Y1: 0, X0: 0, X1: 4}) {
		t.Errorf("%+v %+v", top, top.Box)
	}
	if out := text(t, different, a, b); !strings.Contains(out, "2 to or from NaN") {
		t.Errorf("text:\n%s", out)
	}
}

func TestAShapeMismatch(t *testing.T) {
	a := base().write(t)
	s := base()
	s.kind = append(s.kind, 8)
	b := s.write(t)

	r := diff(t, different, a, b)
	k := array(t, r, "features/table/kind")
	if k.Compared || !slices.Equal(k.ShapeA, []int{7}) || !slices.Equal(k.ShapeB, []int{8}) {
		t.Errorf("%+v", k)
	}
	if r.Arrays[0] != k {
		t.Errorf("an array not compared is not first: %s", r.Arrays[0].Path)
	}
	if out := text(t, different, a, b); !strings.Contains(out, "features/table/kind  not compared: int32 [7] against int32 [8]\n") {
		t.Errorf("text:\n%s", out)
	}
}

func TestAnArrayInOneStoreAlone(t *testing.T) {
	a := base().write(t)
	s := base()
	s.missing = true
	b := s.write(t)

	r := diff(t, different, a, b)
	if !slices.Equal(r.OnlyA, []string{"tile/terrain"}) || len(r.OnlyB) != 0 || !slices.Equal(r.GroupsOnlyA, []string{"tile"}) || len(r.Arrays) != 3 {
		t.Errorf("%+v", r)
	}
	r = diff(t, different, b, a)
	if !slices.Equal(r.OnlyB, []string{"tile/terrain"}) {
		t.Errorf("%+v", r)
	}
	if out := text(t, different, a, b); !strings.Contains(out, "only in a:  tile/terrain") {
		t.Errorf("text:\n%s", out)
	}
}

func TestAttributesDiffer(t *testing.T) {
	a := base().write(t)
	b := base().write(t)
	g, err := zarr.OpenGroup(ctx, zarr.NewDirStore(b), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := g.SetAttributes(ctx, map[string]any{"seed": 2}); err != nil {
		t.Fatal(err)
	}
	h, err := zarr.OpenArray(ctx, zarr.NewDirStore(b), "ground/height")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.SetAttributes(ctx, map[string]any{"units": "ft"}); err != nil {
		t.Fatal(err)
	}
	r := diff(t, different, a, b)
	if len(r.Groups) != 1 || r.Groups[0].Path != "" || !slices.Equal(r.Groups[0].Attributes, []string{"seed"}) ||
		!slices.Equal(array(t, r, "ground/height").Attributes, []string{"units"}) {
		t.Errorf("%+v", r)
	}
}

func TestWhatCannotBeCompared(t *testing.T) {
	a := base().write(t)
	var out, errs bytes.Buffer
	for _, args := range [][]string{
		{a},
		{a, a, a},
		{a, filepath.Join(t.TempDir(), "nothing.zarr")},
		{a, t.TempDir()},
		{"-nonsense", a, a},
	} {
		if got := run(ctx, args, &out, &errs); got != failed {
			t.Errorf("zarrdiff %v exits %d", args, got)
		}
	}
}
