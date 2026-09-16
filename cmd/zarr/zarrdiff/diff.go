package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/LukasSelin/zarr"
)

// config is what the flags ask for.
type config struct {
	png    string // directory for the PNGs, "" for none
	side   int    // longest side of a PNG
	codes  int    // code changes kept for a coded array
	budget int    // elements read from a store at once
}

// Report is what differs between stores A and B.
type Report struct {
	A, B string
	// Same is whether the stores hold the same: the same nodes, attributes,
	// shapes, types and elements. How the arrays are chunked, sharded and
	// compressed is not compared.
	Same bool
	// Arrays is every array in both stores, most changed first.
	Arrays []*Array
	// OnlyA and OnlyB are the arrays in one store alone.
	OnlyA, OnlyB []string `json:",omitempty"`
	// Groups is the groups in both whose attributes differ, and GroupsOnlyA
	// and GroupsOnlyB the groups in one store alone.
	Groups                   []Group  `json:",omitempty"`
	GroupsOnlyA, GroupsOnlyB []string `json:",omitempty"`
}

// Group is a group whose attributes differ.
type Group struct {
	Path       string
	Attributes []string
}

// Array is one array in both stores.
type Array struct {
	Path string
	// Shapes and types of each side. Elements are compared only where both
	// agree.
	ShapeA, ShapeB []int
	TypeA, TypeB   zarr.DataType
	// Attributes names the attributes that differ, with "dimension_names"
	// and "fill_value" among them when those do.
	Attributes []string `json:",omitempty"`
	Compared   bool

	// Elements is how many the array has and Changed how many differ; NaN
	// equals NaN.
	Elements, Changed int64
	// Tiles is the cells of the array's first two dimensions (its elements,
	// if it has fewer), and TilesChanged those with any element changed;
	// Share is the one over the other.
	Tiles, TilesChanged int64
	Share               float64
	// MaxAbs and MeanAbs are of the absolute differences of the changed
	// elements that have one: not where one side is NaN or the difference
	// infinite (Unmeasured), and not in a bool array.
	MaxAbs, MeanAbs float64
	Unmeasured      int64 `json:",omitempty"`
	// Box is the changed tiles' bounding box, inclusive, in the first two
	// dimensions (y and x, in a terra store); a one-dimensional array has
	// only Y.
	Box *Box `json:",omitempty"`
	// Codes is the commonest changes of a coded array (one with a legend or
	// flag_values), and Other the changes not among them.
	Codes []Code `json:",omitempty"`
	Other int64  `json:",omitempty"`
	// PNG is the image written of where the array changed.
	PNG string `json:",omitempty"`

	sum      float64
	measured int64
}

type Box struct {
	Y0, Y1 int
	X0, X1 int `json:",omitempty"`
}

// Code is one change of code, with each code's name where it has one.
type Code struct {
	From, To         int64
	FromName, ToName string `json:",omitempty"`
	Count            int64
}

func (a *Array) differs() bool {
	return !a.Compared || len(a.Attributes) > 0 || a.Changed > 0
}

// compare compares the directory stores at dirA and dirB.
func compare(ctx context.Context, dirA, dirB string, o config) (*Report, error) {
	na, err := walk(dirA)
	if err != nil {
		return nil, err
	}
	nb, err := walk(dirB)
	if err != nil {
		return nil, err
	}
	sa, sb := zarr.NewDirStore(dirA), zarr.NewDirStore(dirB)
	r := &Report{A: dirA, B: dirB}

	for _, p := range na.of("group") {
		if nb[p] != "group" {
			r.GroupsOnlyA = append(r.GroupsOnlyA, p)
			continue
		}
		ga, err := zarr.OpenGroup(ctx, sa, p)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", dirA, err)
		}
		gb, err := zarr.OpenGroup(ctx, sb, p)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", dirB, err)
		}
		if d := attributes(ga.Metadata().Attributes, gb.Metadata().Attributes); len(d) > 0 {
			r.Groups = append(r.Groups, Group{Path: p, Attributes: d})
		}
	}
	for _, p := range nb.of("group") {
		if na[p] != "group" {
			r.GroupsOnlyB = append(r.GroupsOnlyB, p)
		}
	}
	for _, p := range na.of("array") {
		if nb[p] != "array" {
			r.OnlyA = append(r.OnlyA, p)
			continue
		}
		r.Arrays = append(r.Arrays, &Array{Path: p})
	}
	for _, p := range nb.of("array") {
		if na[p] != "array" {
			r.OnlyB = append(r.OnlyB, p)
		}
	}

	// The arrays side by side; each holds a budget of elements from each
	// store at a time.
	errs := make([]error, len(r.Arrays))
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for i, d := range r.Arrays {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			errs[i] = compareArray(ctx, sa, sb, d, o)
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	sort.SliceStable(r.Arrays, func(i, j int) bool {
		a, b := r.Arrays[i], r.Arrays[j]
		if a.Compared != b.Compared {
			return !a.Compared
		}
		if a.Share != b.Share {
			return a.Share > b.Share
		}
		return a.differs() && !b.differs()
	})
	r.Same = len(r.OnlyA)+len(r.OnlyB)+len(r.Groups)+len(r.GroupsOnlyA)+len(r.GroupsOnlyB) == 0
	for _, d := range r.Arrays {
		r.Same = r.Same && !d.differs()
	}
	return r, nil
}

// attributes names the attributes that differ between a and b, as JSON
// values: 1 and 1.0 are the same.
func attributes(a, b map[string]json.RawMessage) []string {
	var d []string
	for k, va := range a {
		if vb, ok := b[k]; !ok || !sameJSON(va, vb) {
			d = append(d, k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			d = append(d, k)
		}
	}
	sort.Strings(d)
	return d
}

func sameJSON(a, b json.RawMessage) bool {
	var va, vb any
	if json.Unmarshal(a, &va) != nil || json.Unmarshal(b, &vb) != nil {
		return string(a) == string(b)
	}
	return reflect.DeepEqual(va, vb)
}

func compareArray(ctx context.Context, sa, sb zarr.Store, d *Array, o config) error {
	a, err := zarr.OpenArray(ctx, sa, d.Path)
	if err != nil {
		return fmt.Errorf("a: %s: %w", d.Path, err)
	}
	b, err := zarr.OpenArray(ctx, sb, d.Path)
	if err != nil {
		return fmt.Errorf("b: %s: %w", d.Path, err)
	}
	ma, mb := a.Metadata(), b.Metadata()
	d.ShapeA, d.ShapeB, d.TypeA, d.TypeB = a.Shape(), b.Shape(), a.DataType(), b.DataType()
	d.Attributes = attributes(ma.Attributes, mb.Attributes)
	if !slices.Equal(a.DimensionNames(), b.DimensionNames()) {
		d.Attributes = append(d.Attributes, "dimension_names")
	}
	if !sameJSON(ma.FillValue, mb.FillValue) {
		d.Attributes = append(d.Attributes, "fill_value")
	}
	if !slices.Equal(d.ShapeA, d.ShapeB) || d.TypeA != d.TypeB {
		return nil
	}
	d.Compared = true
	var codes map[int64]string
	if coded(a) {
		codes = legend(a)
	}
	switch d.TypeA {
	case zarr.Bool:
		err = compareElements[bool](ctx, a, b, d, o, codes)
	case zarr.Int8:
		err = compareElements[int8](ctx, a, b, d, o, codes)
	case zarr.Int16:
		err = compareElements[int16](ctx, a, b, d, o, codes)
	case zarr.Int32:
		err = compareElements[int32](ctx, a, b, d, o, codes)
	case zarr.Int64:
		err = compareElements[int64](ctx, a, b, d, o, codes)
	case zarr.Uint8:
		err = compareElements[uint8](ctx, a, b, d, o, codes)
	case zarr.Uint16:
		err = compareElements[uint16](ctx, a, b, d, o, codes)
	case zarr.Uint32:
		err = compareElements[uint32](ctx, a, b, d, o, codes)
	case zarr.Uint64:
		err = compareElements[uint64](ctx, a, b, d, o, codes)
	case zarr.Float32:
		err = compareElements[float32](ctx, a, b, d, o, codes)
	case zarr.Float64:
		err = compareElements[float64](ctx, a, b, d, o, codes)
	default:
		return fmt.Errorf("%s: data type %s", d.Path, d.TypeA)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", d.Path, err)
	}
	if codes != nil && d.Changed > 0 {
		names := legend(b)
		for i := range d.Codes {
			d.Codes[i].ToName = names[d.Codes[i].To]
		}
	}
	return nil
}

// coded is whether an integer array's elements are codes rather than
// amounts: it carries a legend, or CF's flag_values.
func coded(a *zarr.Array) bool {
	if t := a.DataType(); t == zarr.Float32 || t == zarr.Float64 {
		return false
	}
	attrs := a.Metadata().Attributes
	_, l := attrs["legend"]
	_, f := attrs["flag_values"]
	return l || f
}

// legend is the name of each code of a coded array, empty but not nil
// where it names none.
func legend(a *zarr.Array) map[int64]string {
	names := map[int64]string{}
	var l map[string]string
	if ok, err := a.Attribute("legend", &l); ok && err == nil {
		for k, v := range l {
			if c, err := strconv.ParseInt(k, 10, 64); err == nil {
				names[c] = v
			}
		}
	}
	var values []int64
	var meanings string
	if ok, err := a.Attribute("flag_values", &values); ok && err == nil {
		if ok, err := a.Attribute("flag_meanings", &meanings); ok && err == nil {
			m := strings.Fields(meanings)
			for i, v := range values {
				if i < len(m) {
					names[v] = m[i]
				}
			}
		}
	}
	return names
}

// codePairs is most distinct code changes counted before the rest are
// Other.
const codePairs = 1 << 12

// compareElements reads a and b block by block and counts what differs.
// A block is whole chunks along the first two dimensions, as many as the
// budget allows (one at least), and every element of the rest: each chunk
// is decoded once, and no more than a block of each store is held at once.
func compareElements[T zarr.Element](ctx context.Context, a, b *zarr.Array, d *Array, o config, codes map[int64]string) error {
	shape, chunk := a.Shape(), a.ChunkShape()
	nd := len(shape)
	d.Elements, d.Tiles = 1, 1
	for k, n := range shape {
		d.Elements *= int64(n)
		if k < 2 {
			d.Tiles *= int64(n)
		}
	}
	if d.Elements == 0 {
		return nil
	}
	var pic *picture
	if o.png != "" && nd >= 2 {
		pic = newPicture(shape[0], shape[1], o.side)
	}
	pairs := map[[2]int64]int64{}

	// rows and cols are the extents of the block's first two dimensions,
	// per the elements under each of their cells.
	rows, cols, per := 1, 1, 1
	stepY, stepX := 1, 1
	if nd >= 1 {
		rows, stepY = shape[0], chunk[0]
	}
	if nd >= 2 {
		cols, stepX = shape[1], chunk[1]
		per = int(d.Elements / d.Tiles)
		stepY = chunk[0]
		if n := o.budget / max(1, chunk[0]*chunk[1]*per); n > 1 {
			stepX = min(cols, chunk[1]*n)
		}
	} else if nd == 1 {
		stepY = chunk[0] * max(1, o.budget/chunk[0])
	}

	box := Box{Y0: math.MaxInt, X0: math.MaxInt, Y1: -1, X1: -1}
	for y0 := 0; y0 < rows; y0 += stepY {
		for x0 := 0; x0 < cols; x0 += stepX {
			if err := ctx.Err(); err != nil {
				return err
			}
			var start, size []int
			h, w := min(stepY, rows-y0), min(stepX, cols-x0)
			switch nd {
			case 0:
			case 1:
				start, size = []int{y0}, []int{h}
			default:
				start = make([]int, nd)
				start[0], start[1] = y0, x0
				size = slices.Clone(shape)
				size[0], size[1] = h, w
			}
			va, err := zarr.Read[T](ctx, a, start, size)
			if err != nil {
				return err
			}
			vb, err := zarr.Read[T](ctx, b, start, size)
			if err != nil {
				return err
			}
			for t := range h * w {
				changed := false
				mag := float32(0)
				for e := t * per; e < (t+1)*per; e++ {
					x, y := va[e], vb[e]
					if x == y || (x != x && y != y) {
						continue
					}
					changed = true
					d.Changed++
					if codes != nil {
						k := [2]int64{toInt64(x), toInt64(y)}
						if _, ok := pairs[k]; ok || len(pairs) < codePairs {
							pairs[k]++
						} else {
							d.Other++
						}
						mag = float32(math.Inf(1))
						continue
					}
					diff, ok := absDiff(x, y)
					if !ok {
						if d.TypeA != zarr.Bool {
							d.Unmeasured++
						}
						mag = float32(math.Inf(1))
						continue
					}
					d.measured++
					d.sum += diff
					d.MaxAbs = max(d.MaxAbs, diff)
					mag = max(mag, float32(diff))
				}
				if !changed {
					continue
				}
				d.TilesChanged++
				ty, tx := y0+t/w, x0+t%w
				box.Y0, box.Y1 = min(box.Y0, ty), max(box.Y1, ty)
				box.X0, box.X1 = min(box.X0, tx), max(box.X1, tx)
				if pic != nil {
					pic.mark(ty, tx, mag)
				}
			}
		}
	}

	if d.Changed == 0 {
		return nil
	}
	d.Share = float64(d.TilesChanged) / float64(d.Tiles)
	if d.measured > 0 {
		d.MeanAbs = d.sum / float64(d.measured)
	}
	if nd >= 1 {
		if nd == 1 {
			box.X0, box.X1 = 0, 0
		}
		d.Box = &box
	}
	for k, n := range pairs {
		d.Codes = append(d.Codes, Code{From: k[0], To: k[1], FromName: codes[k[0]], Count: n})
	}
	sort.Slice(d.Codes, func(i, j int) bool {
		ci, cj := d.Codes[i], d.Codes[j]
		if ci.Count != cj.Count {
			return ci.Count > cj.Count
		}
		if ci.From != cj.From {
			return ci.From < cj.From
		}
		return ci.To < cj.To
	})
	if len(d.Codes) > o.codes {
		for _, c := range d.Codes[o.codes:] {
			d.Other += c.Count
		}
		d.Codes = d.Codes[:o.codes]
	}
	if pic != nil {
		name, err := pic.write(o.png, d.Path)
		if err != nil {
			return err
		}
		d.PNG = name
	}
	return nil
}

// absDiff is |x - y|, and false where it has no finite value or the type
// has no difference.
func absDiff[T zarr.Element](x, y T) (float64, bool) {
	var d float64
	switch x := any(x).(type) {
	case bool:
		return 0, false
	case uint64:
		y := any(y).(uint64)
		if x > y {
			d = float64(x - y)
		} else {
			d = float64(y - x)
		}
	case int64:
		d = math.Abs(float64(x) - float64(any(y).(int64)))
	case float32:
		d = math.Abs(float64(x) - float64(any(y).(float32)))
	case float64:
		d = math.Abs(x - any(y).(float64))
	case int8:
		d = math.Abs(float64(x) - float64(any(y).(int8)))
	case int16:
		d = math.Abs(float64(x) - float64(any(y).(int16)))
	case int32:
		d = math.Abs(float64(x) - float64(any(y).(int32)))
	case uint8:
		d = math.Abs(float64(x) - float64(any(y).(uint8)))
	case uint16:
		d = math.Abs(float64(x) - float64(any(y).(uint16)))
	case uint32:
		d = math.Abs(float64(x) - float64(any(y).(uint32)))
	}
	if math.IsNaN(d) || math.IsInf(d, 0) {
		return 0, false
	}
	return d, true
}

// toInt64 is a code as a number; a uint64 past int64 wraps.
func toInt64[T zarr.Element](x T) int64 {
	switch x := any(x).(type) {
	case bool:
		if x {
			return 1
		}
		return 0
	case int8:
		return int64(x)
	case int16:
		return int64(x)
	case int32:
		return int64(x)
	case int64:
		return x
	case uint8:
		return int64(x)
	case uint16:
		return int64(x)
	case uint32:
		return int64(x)
	case uint64:
		return int64(x)
	}
	return 0
}
