package main

import (
	"context"
	"fmt"
	"maps"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/LukasSelin/zarr"
)

// Breakdown is an array's changes by the category of each tile in a map
// of codes, as -by asks: book/meeting, climate/koppen, features/belt.
type Breakdown struct {
	// By is the map's path and Side the store it is read from, "a" or "b".
	By, Side string
	// Categories is every category of a coded map with tiles, in order of
	// code; of a map of ids, the categories with most tiles changed, most
	// first, and More of them not listed, with MoreTilesChanged between them.
	Categories       []Category
	More             int   `json:",omitempty"`
	MoreTilesChanged int64 `json:",omitempty"`
}

// Category is the changes to the tiles of one code.
type Category struct {
	Code int64
	Name string `json:",omitempty"`
	// Tiles is the tiles of the code, TilesChanged those with any element
	// changed, Share the one over the other, and Changed the elements.
	Tiles, TilesChanged, Changed int64
	Share                        float64
	Signed                       *Signed `json:",omitempty"`
}

// A cut is a map of codes that sorts tiles into categories: an array of a
// -by, a -mask, or the where of a check, read from one side.
type cut struct {
	path  string
	chunk [2]int           // its chunks' height and width
	names map[int64]string // nil where the map is not coded
	tiles map[int64]int64  // tiles of each code, within the masks
}

// A scope is some of a cut's codes: those in set, or with not, those
// outside it.
type scope struct {
	cut int
	set map[int64]bool
	not bool
}

func (s scope) in(code int64) bool { return s.set[code] != s.not }

// tiles is the tiles in the scope.
func (s scope) tiles(c *cut) int64 {
	n := int64(0)
	for code, k := range c.tiles {
		if s.in(code) {
			n += k
		}
	}
	return n
}

// setup is what -by, -mask and the checks' wheres need, shared by every
// array: the cuts, all maps of one shape h by w from one store.
type setup struct {
	side  string
	store zarr.Store
	h, w  int
	cuts  []*cut
	by    []int   // the cuts broken down by, in order of -by
	masks []scope // tiles outside any are left out
	// scopes is the where of each check that has one, by check.
	scopes  []scope
	scopeOf map[int]int

	// The cuts' chunks, decoded, are shared between the arrays: every array
	// reads the same maps, and reading them afresh for each is most of the
	// time. At most held codes are kept, the oldest let go first.
	mu     sync.Mutex
	cached map[[3]int]*codeChunk // by cut, chunk row, chunk column
	queue  [][3]int
	held   int
	limit  int
}

// codeChunk is a chunk of a cut's codes, read once however many ask.
type codeChunk struct {
	once  sync.Once
	codes []int64
	err   error
}

func prepare(ctx context.Context, sa, sb zarr.Store, o config) (*setup, error) {
	e := &setup{side: o.bySide, store: sa, scopeOf: map[int]int{},
		cached: map[[3]int]*codeChunk{}, limit: o.budget * runtime.GOMAXPROCS(0)}
	if o.bySide == "b" {
		e.store = sb
	}
	add := func(path string) (int, error) {
		for i, c := range e.cuts {
			if c.path == path {
				return i, nil
			}
		}
		arr, err := zarr.OpenArray(ctx, e.store, path)
		if err != nil {
			return 0, fmt.Errorf("%s in %s: %w", path, e.side, err)
		}
		shape := arr.Shape()
		switch {
		case len(shape) != 2:
			return 0, fmt.Errorf("%s is not a map of tiles: its shape is %v", path, shape)
		case arr.DataType() == zarr.Float32 || arr.DataType() == zarr.Float64:
			return 0, fmt.Errorf("%s holds amounts, not codes", path)
		case len(e.cuts) == 0:
			e.h, e.w = shape[0], shape[1]
		case shape[0] != e.h || shape[1] != e.w:
			return 0, fmt.Errorf("%s is %v, not %d by %d as %s is", path, shape, e.h, e.w, e.cuts[0].path)
		}
		c := &cut{path: path, tiles: map[int64]int64{}, chunk: [2]int(arr.ChunkShape())}
		if coded(arr) {
			c.names = legend(arr)
		}
		e.cuts = append(e.cuts, c)
		return len(e.cuts) - 1, nil
	}
	codes := func(i int, tokens []string) (map[int64]bool, error) {
		set := map[int64]bool{}
		for _, t := range tokens {
			c, err := lookup(e.cuts[i].names, t)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", e.cuts[i].path, err)
			}
			set[c] = true
		}
		return set, nil
	}

	for _, p := range o.by {
		i, err := add(p)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(e.by, i) {
			e.by = append(e.by, i)
		}
	}
	for _, m := range o.mask {
		path, list, ok := strings.Cut(m, "=")
		if !ok || list == "" {
			return nil, fmt.Errorf("-mask %q is not group/array=code[,code]", m)
		}
		i, err := add(path)
		if err != nil {
			return nil, err
		}
		set, err := codes(i, strings.Split(list, ","))
		if err != nil {
			return nil, err
		}
		e.masks = append(e.masks, scope{cut: i, set: set})
	}
	for k, c := range o.checks {
		if c.Where == nil {
			continue
		}
		i, err := add(c.Where.By)
		if err != nil {
			return nil, err
		}
		s := scope{cut: i}
		tokens := c.Where.Is
		if len(c.Where.Not) > 0 {
			tokens, s.not = c.Where.Not, true
		}
		if s.set, err = codes(i, tokenStrings(tokens)); err != nil {
			return nil, err
		}
		e.scopeOf[k] = len(e.scopes)
		e.scopes = append(e.scopes, s)
	}
	if len(e.cuts) == 0 {
		return e, nil
	}

	// The tiles of each code, within the masks.
	err := blocks([]int{e.h, e.w}, e.cuts[0].chunk[:], o.budget, func(y0, x0, h, w int) error {
		codes, inside, err := e.read(ctx, y0, x0, h, w)
		if err != nil {
			return err
		}
		for t := range h * w {
			if inside == nil || inside[t] {
				for i, c := range e.cuts {
					c.tiles[codes[i][t]]++
				}
			}
		}
		return nil
	})
	return e, err
}

// masked is whether -mask leaves tiles out.
func (e *setup) masked() bool { return len(e.masks) > 0 }

// fits is whether an array of shape is a map the cuts lie over: its first
// two dimensions are theirs.
func (e *setup) fits(shape []int) bool {
	return len(e.cuts) > 0 && len(shape) >= 2 && shape[0] == e.h && shape[1] == e.w
}

// read is the codes of each cut over the tiles from y0, x0, h by w, and
// whether each tile is inside every mask; nil where there is no mask.
func (e *setup) read(ctx context.Context, y0, x0, h, w int) ([][]int64, []bool, error) {
	codes := make([][]int64, len(e.cuts))
	for i, c := range e.cuts {
		codes[i] = make([]int64, h*w)
		ch, cw := c.chunk[0], c.chunk[1]
		for cy := y0 / ch; cy*ch < y0+h; cy++ {
			for cx := x0 / cw; cx*cw < x0+w; cx++ {
				chunk, err := e.chunk(ctx, i, cy, cx)
				if err != nil {
					return nil, nil, err
				}
				// The chunk's tiles within the block, row by row.
				cy0, cx0 := cy*ch, cx*cw
				cols := min(cw, e.w-cx0)
				for y := max(y0, cy0); y < min(y0+h, cy0+ch, e.h); y++ {
					from := max(x0, cx0)
					to := min(x0+w, cx0+cols)
					copy(codes[i][(y-y0)*w+from-x0:(y-y0)*w+to-x0], chunk[(y-cy0)*cols+from-cx0:(y-cy0)*cols+to-cx0])
				}
			}
		}
	}
	if !e.masked() {
		return codes, nil, nil
	}
	inside := make([]bool, h*w)
	for t := range inside {
		inside[t] = true
		for _, m := range e.masks {
			inside[t] = inside[t] && m.in(codes[m.cut][t])
		}
	}
	return codes, inside, nil
}

// chunk is the codes of cut i's chunk at row cy and column cx, cut at the
// map's edges, read or found in the cache.
func (e *setup) chunk(ctx context.Context, i, cy, cx int) ([]int64, error) {
	c := e.cuts[i]
	key := [3]int{i, cy, cx}
	e.mu.Lock()
	k := e.cached[key]
	if k == nil {
		k = &codeChunk{}
		e.cached[key] = k
		e.queue = append(e.queue, key)
		e.held += c.chunk[0] * c.chunk[1]
		for e.held > e.limit && len(e.queue) > 1 {
			old := e.queue[0]
			e.queue = e.queue[1:]
			delete(e.cached, old)
			e.held -= e.cuts[old[0]].chunk[0] * e.cuts[old[0]].chunk[1]
		}
	}
	e.mu.Unlock()
	k.once.Do(func() {
		a, err := zarr.OpenArray(ctx, e.store, c.path)
		if err != nil {
			k.err = fmt.Errorf("%s in %s: %w", c.path, e.side, err)
			return
		}
		y, x := cy*c.chunk[0], cx*c.chunk[1]
		size := []int{min(c.chunk[0], e.h-y), min(c.chunk[1], e.w-x)}
		if k.codes, err = readCodes(ctx, a, []int{y, x}, size); err != nil {
			k.err = fmt.Errorf("%s: %w", c.path, err)
		}
	})
	return k.codes, k.err
}

// hits appends the tallies a changed tile t counts in: its category in each
// breakdown, made if it is the first, and each scope it is in.
func (e *setup) hits(hit []*tally, codes [][]int64, t int, cats []map[int64]*tally, scopes []*tally, numeric bool) []*tally {
	for i, ci := range e.by {
		c := codes[ci][t]
		tl := cats[i][c]
		if tl == nil {
			// A map of ids may have a category a tile; they are binned in a
			// second reading, the top ones alone.
			tl = &tally{keep: numeric && e.cuts[ci].names != nil}
			cats[i][c] = tl
		}
		hit = append(hit, tl)
	}
	for i, s := range e.scopes {
		if s.in(codes[s.cut][t]) {
			hit = append(hit, scopes[i])
		}
	}
	return hit
}

// top is the n categories of a map of ids with most tiles changed.
func top(cats map[int64]*tally, n int) map[int64]*tally {
	order := slices.Collect(maps.Keys(cats))
	sortByChange(order, cats)
	kept := map[int64]*tally{}
	for _, c := range order[:min(n, len(order))] {
		kept[c] = cats[c]
	}
	return kept
}

func sortByChange(order []int64, cats map[int64]*tally) {
	sort.Slice(order, func(i, j int) bool {
		a, b := cats[order[i]], cats[order[j]]
		if a.tilesChanged != b.tilesChanged {
			return a.tilesChanged > b.tilesChanged
		}
		return order[i] < order[j]
	})
}

// breakdown is the Breakdown of the cut ci from its categories' tallies.
func (e *setup) breakdown(ci int, cats map[int64]*tally, n int, numeric bool) *Breakdown {
	c := e.cuts[ci]
	bd := &Breakdown{By: c.path, Side: e.side}
	row := func(code int64) Category {
		r := Category{Code: code, Name: c.names[code], Tiles: c.tiles[code]}
		if tl := cats[code]; tl != nil {
			r.TilesChanged, r.Changed = tl.tilesChanged, tl.changed
			if numeric {
				r.Signed = tl.signed()
			}
		}
		if r.Tiles > 0 {
			r.Share = float64(r.TilesChanged) / float64(r.Tiles)
		}
		return r
	}
	var order []int64
	if c.names != nil {
		order = slices.Sorted(maps.Keys(c.tiles))
	} else {
		order = slices.Collect(maps.Keys(cats))
		sortByChange(order, cats)
		for _, code := range order[min(n, len(order)):] {
			bd.More++
			bd.MoreTilesChanged += cats[code].tilesChanged
		}
		order = order[:min(n, len(order))]
	}
	for _, code := range order {
		bd.Categories = append(bd.Categories, row(code))
	}
	return bd
}

// lookup is the code a token names: a meaning of the map's flags, matched
// without regard to case and with spaces as underscores, or a number.
func lookup(names map[int64]string, token string) (int64, error) {
	want := strings.ReplaceAll(strings.TrimSpace(token), " ", "_")
	codes := slices.Sorted(maps.Keys(names))
	for _, c := range codes {
		if strings.EqualFold(strings.ReplaceAll(names[c], " ", "_"), want) {
			return c, nil
		}
	}
	if c, err := strconv.ParseInt(want, 10, 64); err == nil {
		return c, nil
	}
	if names == nil {
		return 0, fmt.Errorf("%q is not a number, and the array names no codes", token)
	}
	return 0, fmt.Errorf("no code is named %q", token)
}

// readCodes reads the elements of an integer or bool array as codes.
func readCodes(ctx context.Context, a *zarr.Array, start, size []int) ([]int64, error) {
	switch a.DataType() {
	case zarr.Bool:
		return codesOf[bool](ctx, a, start, size)
	case zarr.Int8:
		return codesOf[int8](ctx, a, start, size)
	case zarr.Int16:
		return codesOf[int16](ctx, a, start, size)
	case zarr.Int32:
		return codesOf[int32](ctx, a, start, size)
	case zarr.Int64:
		return codesOf[int64](ctx, a, start, size)
	case zarr.Uint8:
		return codesOf[uint8](ctx, a, start, size)
	case zarr.Uint16:
		return codesOf[uint16](ctx, a, start, size)
	case zarr.Uint32:
		return codesOf[uint32](ctx, a, start, size)
	case zarr.Uint64:
		return codesOf[uint64](ctx, a, start, size)
	}
	return nil, fmt.Errorf("data type %s is not codes", a.DataType())
}

func codesOf[T zarr.Element](ctx context.Context, a *zarr.Array, start, size []int) ([]int64, error) {
	v, err := zarr.Read[T](ctx, a, start, size)
	if err != nil {
		return nil, err
	}
	c := make([]int64, len(v))
	for i, x := range v {
		c[i] = toInt64(x)
	}
	return c, nil
}

// blocks calls visit with each block an array of shape and chunk is read
// in: whole chunks along its first two dimensions, one chunk high and as
// many wide as the budget of elements allows (one at least), and every
// element of the rest. A one-dimensional array is read in runs of whole
// chunks; w is 1 for it, and h and w are 1 for a scalar.
func blocks(shape, chunk []int, budget int, visit func(y0, x0, h, w int) error) error {
	nd := len(shape)
	rows, cols, stepY, stepX := 1, 1, 1, 1
	switch {
	case nd >= 2:
		per := 1
		for _, n := range shape[2:] {
			per *= n
		}
		rows, cols, stepY, stepX = shape[0], shape[1], chunk[0], chunk[1]
		if n := budget / max(1, chunk[0]*chunk[1]*per); n > 1 {
			stepX = min(cols, chunk[1]*n)
		}
	case nd == 1:
		rows, stepY = shape[0], chunk[0]*max(1, budget/chunk[0])
	}
	for y0 := 0; y0 < rows; y0 += stepY {
		for x0 := 0; x0 < cols; x0 += stepX {
			if err := visit(y0, x0, min(stepY, rows-y0), min(stepX, cols-x0)); err != nil {
				return err
			}
		}
	}
	return nil
}
