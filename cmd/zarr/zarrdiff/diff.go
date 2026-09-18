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

	by     []string // arrays to break the changes down by, -by
	bySide string   // the store they and the masks are read from, "a" or "b"
	top    int      // categories listed of a map of ids
	only   []string // arrays and groups compared; all where empty
	mask   []string // group/array=code[,code], tiles compared
	checks []Check  // the -expect file's
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
	// Only and Mask are the -only and -mask the comparison was held to, and
	// Skipped the arrays in both left out as the masks do not lie over them.
	Only, Mask []string `json:",omitempty"`
	Skipped    []string `json:",omitempty"`
	// Checks is each check of -expect, with what it measured.
	Checks []Result `json:",omitempty"`
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
	// Signed is the measured changes as b-a, for an array of amounts: not
	// codes, not bools.
	Signed *Signed `json:",omitempty"`
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
	// By is the changes by the category of each tile, a Breakdown a -by,
	// for a changed array the -by maps lie over.
	By []*Breakdown `json:",omitempty"`

	sum      float64
	measured int64
	numeric  bool               // amounts, not codes or bools
	namesA   map[int64]string   // a coded array's names in a
	namesB   map[int64]string   // and in b
	pairs    map[[2]int64]int64 // every change of code counted
	overflow int64              // changes of code past codePairs
	scopes   []*tally           // by the setup's scopes; nil where the cuts do not lie over the array
	skipped  bool               // left out by -mask
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
	sa, sb := zarr.NewDirStore(dirA), zarr.NewDirStore(dirB)
	na, err := walk(ctx, sa, dirA)
	if err != nil {
		return nil, err
	}
	nb, err := walk(ctx, sb, dirB)
	if err != nil {
		return nil, err
	}
	r := &Report{A: dirA, B: dirB, Only: o.only, Mask: o.mask}
	e, err := prepare(ctx, sa, sb, o)
	if err != nil {
		return nil, err
	}

	// -only holds the comparison to some arrays, and then groups' attributes
	// are not compared.
	kept := func(p string) bool {
		if len(o.only) == 0 {
			return true
		}
		for _, q := range o.only {
			if p == q || strings.HasPrefix(p, q+"/") {
				return true
			}
		}
		return false
	}
	for _, q := range o.only {
		found := false
		for _, n := range []nodes{na, nb} {
			for p := range n {
				found = found || p == q || strings.HasPrefix(p, q+"/")
			}
		}
		if !found {
			return nil, fmt.Errorf("-only %s: in neither store", q)
		}
	}
	if len(o.only) > 0 {
		na, nb = na.within(kept), nb.within(kept)
	}

	for _, p := range na.of("group") {
		if len(o.only) > 0 {
			break
		}
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
		if na[p] != "group" && len(o.only) == 0 {
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
			errs[i] = compareArray(ctx, sa, sb, d, o, e)
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	arrays := r.Arrays[:0]
	for _, d := range r.Arrays {
		if d.skipped {
			r.Skipped = append(r.Skipped, d.Path)
		} else {
			arrays = append(arrays, d)
		}
	}
	r.Arrays = arrays
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
	if r.Checks, err = evaluate(r, e, o.checks); err != nil {
		return nil, err
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

func compareArray(ctx context.Context, sa, sb zarr.Store, d *Array, o config, e *setup) error {
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
	if e.masked() && !e.fits(d.ShapeA) && !e.fits(d.ShapeB) {
		d.skipped = true
		return nil
	}
	if !slices.Equal(d.ShapeA, d.ShapeB) || d.TypeA != d.TypeB {
		return nil
	}
	d.Compared = true
	var codes map[int64]string
	if coded(a) {
		codes = legend(a)
		d.namesA, d.namesB = codes, legend(b)
	}
	d.numeric = codes == nil && d.TypeA != zarr.Bool
	switch d.TypeA {
	case zarr.Bool:
		err = compareElements[bool](ctx, a, b, d, o, codes, e)
	case zarr.Int8:
		err = compareElements[int8](ctx, a, b, d, o, codes, e)
	case zarr.Int16:
		err = compareElements[int16](ctx, a, b, d, o, codes, e)
	case zarr.Int32:
		err = compareElements[int32](ctx, a, b, d, o, codes, e)
	case zarr.Int64:
		err = compareElements[int64](ctx, a, b, d, o, codes, e)
	case zarr.Uint8:
		err = compareElements[uint8](ctx, a, b, d, o, codes, e)
	case zarr.Uint16:
		err = compareElements[uint16](ctx, a, b, d, o, codes, e)
	case zarr.Uint32:
		err = compareElements[uint32](ctx, a, b, d, o, codes, e)
	case zarr.Uint64:
		err = compareElements[uint64](ctx, a, b, d, o, codes, e)
	case zarr.Float32:
		err = compareElements[float32](ctx, a, b, d, o, codes, e)
	case zarr.Float64:
		err = compareElements[float64](ctx, a, b, d, o, codes, e)
	default:
		return fmt.Errorf("%s: data type %s", d.Path, d.TypeA)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", d.Path, err)
	}
	for i := range d.Codes {
		d.Codes[i].ToName = d.namesB[d.Codes[i].To]
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

// A block is what is read of both stores at once, with the codes of the
// setup's cuts under its tiles and whether each is inside the masks (nil
// where the cuts do not lie over the array, or there are no masks).
type block[T zarr.Element] struct {
	y0, x0, h, w int
	va, vb       []T
	codes        [][]int64
	inside       []bool
}

// scan reads a and b block by block (see blocks), each chunk decoded once
// and no more than a block of each store held at once, and gives visit
// each block, with the codes of the setup's maps where tiled says they lie
// over the array.
func scan[T zarr.Element](ctx context.Context, a, b *zarr.Array, budget int, e *setup, tiled bool, visit func(*block[T])) error {
	shape := a.Shape()
	nd := len(shape)
	return blocks(shape, a.ChunkShape(), budget, func(y0, x0, h, w int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		var start, size []int
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
		bl := &block[T]{y0: y0, x0: x0, h: h, w: w}
		var err error
		if bl.va, err = zarr.Read[T](ctx, a, start, size); err != nil {
			return err
		}
		if bl.vb, err = zarr.Read[T](ctx, b, start, size); err != nil {
			return err
		}
		if tiled {
			if bl.codes, bl.inside, err = e.read(ctx, y0, x0, h, w); err != nil {
				return err
			}
		}
		visit(bl)
		return nil
	})
}

// compareElements reads a and b and counts what differs: over the whole
// array, by the category of each tile in each -by map, and in each check's
// where.
func compareElements[T zarr.Element](ctx context.Context, a, b *zarr.Array, d *Array, o config, codes map[int64]string, e *setup) error {
	shape := a.Shape()
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
	per := int(d.Elements / d.Tiles)
	if e.masked() {
		d.Elements, d.Tiles = 0, 0 // counted as the masks let them in
	}
	tiled := e.fits(shape)
	var pic *picture
	if o.png != "" && nd >= 2 {
		pic = newPicture(shape[0], shape[1], o.side)
	}
	d.pairs = map[[2]int64]int64{}
	total := &tally{keep: d.numeric}
	var cats []map[int64]*tally
	if tiled {
		cats = make([]map[int64]*tally, len(e.by))
		for i := range cats {
			cats[i] = map[int64]*tally{}
		}
		d.scopes = make([]*tally, len(e.scopes))
		for i := range d.scopes {
			d.scopes[i] = &tally{keep: d.numeric}
		}
	}

	box := Box{Y0: math.MaxInt, X0: math.MaxInt, Y1: -1, X1: -1}
	var hit []*tally
	err := scan(ctx, a, b, o.budget, e, tiled, func(bl *block[T]) {
		for t := range bl.h * bl.w {
			if bl.inside != nil {
				if !bl.inside[t] {
					continue
				}
				d.Tiles++
				d.Elements += int64(per)
			}
			changed := false
			mag := float32(0)
			for i := t * per; i < (t+1)*per; i++ {
				x, y := bl.va[i], bl.vb[i]
				if x == y || (x != x && y != y) {
					continue
				}
				if !changed {
					changed = true
					hit = hit[:0]
					if tiled {
						hit = e.hits(hit, bl.codes, t, cats, d.scopes, d.numeric)
					}
				}
				d.Changed++
				for _, h := range hit {
					h.changed++
				}
				if codes != nil {
					k := [2]int64{toInt64(x), toInt64(y)}
					if _, ok := d.pairs[k]; ok || len(d.pairs) < codePairs {
						d.pairs[k]++
					} else {
						d.overflow++
					}
					mag = float32(math.Inf(1))
					continue
				}
				v, ok := signedDiff(x, y)
				if !ok {
					if d.TypeA != zarr.Bool {
						d.Unmeasured++
					}
					mag = float32(math.Inf(1))
					continue
				}
				diff := math.Abs(v)
				d.measured++
				d.sum += diff
				d.MaxAbs = max(d.MaxAbs, diff)
				mag = max(mag, float32(diff))
				total.add(v)
				for _, h := range hit {
					h.add(v)
				}
			}
			if !changed {
				continue
			}
			d.TilesChanged++
			for _, h := range hit {
				h.tilesChanged++
			}
			ty, tx := bl.y0+t/bl.w, bl.x0+t%bl.w
			box.Y0, box.Y1 = min(box.Y0, ty), max(box.Y1, ty)
			box.X0, box.X1 = min(box.X0, tx), max(box.X1, tx)
			if pic != nil {
				pic.mark(ty, tx, mag)
			}
		}
	})
	if err != nil {
		return err
	}

	// A map of ids may have as many categories as tiles, too many to keep a
	// histogram each: its top categories are binned in a second reading.
	if tiled && d.measured > 0 {
		tops := make([]map[int64]*tally, len(e.by))
		again := false
		for i, ci := range e.by {
			if e.cuts[ci].names == nil {
				tops[i] = top(cats[i], o.top)
				again = again || len(tops[i]) > 0
			}
		}
		if again {
			err := scan(ctx, a, b, o.budget, e, tiled, func(bl *block[T]) {
				for t := range bl.h * bl.w {
					if bl.inside != nil && !bl.inside[t] {
						continue
					}
					for i := t * per; i < (t+1)*per; i++ {
						x, y := bl.va[i], bl.vb[i]
						if x == y {
							continue
						}
						v, ok := signedDiff(x, y)
						if !ok {
							continue
						}
						for k, kept := range tops {
							if tl := kept[bl.codes[e.by[k]][t]]; tl != nil {
								tl.bin(v)
							}
						}
					}
				}
			})
			if err != nil {
				return err
			}
		}
	}

	if d.Changed == 0 {
		return nil
	}
	if d.Tiles > 0 {
		d.Share = float64(d.TilesChanged) / float64(d.Tiles)
	}
	if d.measured > 0 {
		d.MeanAbs = d.sum / float64(d.measured)
		d.Signed = total.signed()
	}
	if nd >= 1 {
		if nd == 1 {
			box.X0, box.X1 = 0, 0
		}
		d.Box = &box
	}
	if tiled {
		for i, ci := range e.by {
			d.By = append(d.By, e.breakdown(ci, cats[i], o.top, d.numeric))
		}
	}
	d.Other = d.overflow
	for k, n := range d.pairs {
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

// signedDiff is y - x, b-a, and false where it has no finite value or the
// type has no difference.
func signedDiff[T zarr.Element](x, y T) (float64, bool) {
	var d float64
	switch x := any(x).(type) {
	case bool:
		return 0, false
	case uint64:
		y := any(y).(uint64)
		if y > x {
			d = float64(y - x)
		} else {
			d = -float64(x - y)
		}
	case int64:
		d = float64(any(y).(int64)) - float64(x)
	case float32:
		d = float64(any(y).(float32)) - float64(x)
	case float64:
		d = any(y).(float64) - x
	case int8:
		d = float64(any(y).(int8)) - float64(x)
	case int16:
		d = float64(any(y).(int16)) - float64(x)
	case int32:
		d = float64(any(y).(int32)) - float64(x)
	case uint8:
		d = float64(any(y).(uint8)) - float64(x)
	case uint16:
		d = float64(any(y).(uint16)) - float64(x)
	case uint32:
		d = float64(any(y).(uint32)) - float64(x)
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
