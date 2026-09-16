package terra

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"reflect"
	"runtime"
	"unsafe"
)

// A history file is a world stopped between its ground stage and its sea
// (see stages.go): the grid as the history left it and where the land's
// chance had got to, with the seed and the terms it was made on. The stages
// after it are then run from the file instead of from the plates, and the
// world they make is the world NewLand makes on the same seed and terms, to
// the bit: TestAWorldResumedFromItsHistoryIsTheSameWorld holds that.
//
// The history is two thirds of making a globe, and none of what comes after
// it reads anything but the hand-off. So a history is a thing to keep: a
// change to the shaping, the cutting, the coast or the cover is run again on
// a kept history in the time those stages take.
//
// What is kept is every field of the Grid but the ones historyDropped names,
// found by reflection, so that a field added to the Grid is kept without
// anybody remembering to keep it. A field of a kind the file cannot hold - a
// map, a function, an interface - fails the writing, and has either to be
// made one it can or to be named in historyDropped with the reason. Nothing
// is left out quietly.
//
// The file is memory as it lies: numbers in the machine's own order and
// structs with their padding. It says so in its header - the architecture,
// and a fingerprint of the layout of every type it holds - and a reader
// refuses a file whose header is not its own. What the header cannot say is
// whether the code that wrote it made the same history this code would: a
// file kept across a change to the history is that change's history, not
// this one's. It is a cache of a history, not an interchange format.

// historyDropped are the Grid's fields a history file does not keep, and why
// the stages after the history do not need them.
var historyDropped = map[string]string{
	"floodScratch": "working memory: the flood clears what it reads",
	"slideScratch": "working memory: the slides clear what they read",
	"fillScratch":  "working memory: the fill clears what it reads",
	"poolScratch":  "working memory: the order it repairs is total, so any order it starts from ends the same",
	"flowScratch":  "working memory: flow clears what it reads",
	"stepScratch":  "working memory: the step clears what it reads",
	"creepScratch": "working memory: creep clears what it reads",
	"router":       "made the first time a route is asked for",
	"landmarks":    "a game's, laid once the world is handed over",
	"features":     "read at the end of the cover stage",
}

const (
	historyMagic   = "terra history\n"
	historyVersion = 1
)

// ErrHistoryFile is what reading a history fails with when the file is not
// one this build wrote, or not a history at all.
var ErrHistoryFile = errors.New("not a history this build can read")

// MakeLandKeepingHistory is MakeLand, and writes the world's history to out
// once its ground stage is over, before the rest of the world is made. See
// LandFromHistory.
func MakeLandKeepingHistory(seed uint64, t Terms, out io.Writer) (*Land, error) {
	return MakeLandWatching(seed, t, out, nil)
}

// MakeLandWatching is MakeLand, keeping the world's history in out where out
// is not nil, as MakeLandKeepingHistory does, and telling watch of each stage
// as it ends where watch is not nil. The world is the same world either way.
func MakeLandWatching(seed uint64, t Terms, out io.Writer, watch StageWatch) (*Land, error) {
	if err := t.Check(); err != nil {
		return nil, err
	}
	if err := t.fits(); err != nil {
		return nil, err
	}
	defer phase("Generate")()
	l := unmade(seed, t)
	g := l.newGround(t)
	l.generateFrom(g, t, stageGround, stageSea, watch)
	if out != nil {
		if err := l.writeHistory(out, g, stageSea); err != nil {
			return nil, err
		}
	}
	l.generateFrom(g, t, stageSea, len(stages), watch)
	l.handOver()
	return l, nil
}

// LandFromHistory makes the world a history file was kept from: the stages
// after the history, run on it. It is the land MakeLandKeepingHistory made
// when it wrote the file.
func LandFromHistory(in io.Reader) (*Land, error) {
	return LandFromHistoryWatching(in, nil)
}

// LandFromHistoryWatching is LandFromHistory, telling watch, where it is not
// nil, of the stage the history stopped after, with the grid as the file
// kept it, and then of each stage run on it as it ends. See StageWatch.
func LandFromHistoryWatching(in io.Reader, watch StageWatch) (*Land, error) {
	r := bufio.NewReaderSize(in, 1<<20)
	h, err := readHistoryHeader(r)
	if err != nil {
		return nil, err
	}
	if err := h.terms.fits(); err != nil {
		return nil, err
	}
	l := unmade(h.seed, h.terms)
	if err := l.source.UnmarshalBinary(h.chance); err != nil {
		return nil, fmt.Errorf("%w: the chance: %v", ErrHistoryFile, err)
	}
	g := new(Grid)
	d := historyCodec{r: r, limit: 64*uint64(h.terms.Width)*uint64(h.terms.Height) + 1<<20}
	for _, f := range historyFields() {
		d.decode(reflect.ValueOf(g).Elem().Field(f.Index[0]), "Grid."+f.Name)
	}
	if d.err != nil {
		return nil, d.err
	}
	if g.W != h.terms.Width || g.H != h.terms.Height {
		return nil, fmt.Errorf("%w: a %dx%d grid under %dx%d terms", ErrHistoryFile, g.W, g.H, h.terms.Width, h.terms.Height)
	}
	defer phase("Generate")()
	if watch != nil {
		watch(Stages()[h.next-1], l, g)
	}
	l.generateFrom(g, h.terms, h.next, len(stages), watch)
	l.handOver()
	return l, nil
}

// HistoryTerms reads the seed and the terms a history file was made on, and
// nothing more of it: what a caller needs before the world is made from it.
func HistoryTerms(in io.Reader) (seed uint64, t Terms, err error) {
	h, err := readHistoryHeader(bufio.NewReader(in))
	return h.seed, h.terms, err
}

// writeHistory writes g, stopped before the stage numbered next, to out.
func (l *Land) writeHistory(out io.Writer, g *Grid, next int) error {
	w := bufio.NewWriterSize(out, 1<<20)
	chance, err := l.source.MarshalBinary()
	if err != nil {
		return err
	}
	e := historyCodec{w: w}
	e.bytes([]byte(historyMagic))
	e.uint(historyVersion)
	e.bytes([]byte(runtime.GOARCH))
	e.uint(historyLayout())
	e.uint(l.seed)
	e.encode(reflect.ValueOf(&l.Terms).Elem(), "Terms")
	e.uint(uint64(next))
	e.bytes(chance)
	for _, f := range historyFields() {
		e.encode(reflect.ValueOf(g).Elem().Field(f.Index[0]), "Grid."+f.Name)
	}
	if e.err != nil {
		return e.err
	}
	return w.Flush()
}

type historyHeader struct {
	seed   uint64
	terms  Terms
	next   int
	chance []byte
}

func readHistoryHeader(r *bufio.Reader) (historyHeader, error) {
	var h historyHeader
	d := historyCodec{r: r, limit: 1 << 16}
	if string(d.readBytes()) != historyMagic || d.err != nil {
		return h, fmt.Errorf("%w: no history header", ErrHistoryFile)
	}
	if v := d.readUint(); v != historyVersion {
		return h, fmt.Errorf("%w: version %d, and this build reads %d", ErrHistoryFile, v, historyVersion)
	}
	if arch := string(d.readBytes()); arch != runtime.GOARCH {
		return h, fmt.Errorf("%w: written on %s, read on %s", ErrHistoryFile, arch, runtime.GOARCH)
	}
	if lay := d.readUint(); lay != historyLayout() {
		return h, fmt.Errorf("%w: the grid's layout has changed since it was written", ErrHistoryFile)
	}
	h.seed = d.readUint()
	d.decode(reflect.ValueOf(&h.terms).Elem(), "Terms")
	h.next = int(d.readUint())
	h.chance = d.readBytes()
	if d.err != nil {
		return h, d.err
	}
	if h.next <= stageGround || h.next >= len(stages) {
		return h, fmt.Errorf("%w: stopped before stage %d", ErrHistoryFile, h.next)
	}
	if err := h.terms.Check(); err != nil {
		return h, fmt.Errorf("%w: %v", ErrHistoryFile, err)
	}
	return h, nil
}

// historyFields are the Grid's fields a history file keeps, in the order
// they are declared.
func historyFields() []reflect.StructField {
	t := reflect.TypeFor[Grid]()
	var kept []reflect.StructField
	for i := range t.NumField() {
		if _, dropped := historyDropped[t.Field(i).Name]; !dropped {
			kept = append(kept, t.Field(i))
		}
	}
	return kept
}

// historyLayout is a fingerprint of how everything a history file holds is
// laid out in memory: every type's name, size and kind, and every field's
// name, type and offset, down through the slices and pointers.
func historyLayout() uint64 {
	h := fnv.New64a()
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		fmt.Fprintf(h, "%s/%d/%d{", t.String(), t.Kind(), t.Size())
		switch t.Kind() {
		case reflect.Struct:
			for i := range t.NumField() {
				f := t.Field(i)
				fmt.Fprintf(h, "%s@%d:", f.Name, f.Offset)
				walk(f.Type)
			}
		case reflect.Array, reflect.Slice, reflect.Pointer:
			walk(t.Elem())
		}
		fmt.Fprint(h, "}")
	}
	walk(reflect.TypeFor[Terms]())
	for _, f := range historyFields() {
		fmt.Fprintf(h, "%s:", f.Name)
		walk(f.Type)
	}
	return h.Sum64()
}

// historyCodec writes or reads values by their type: a value with no
// pointers in it as its bytes, a slice as its length and then its elements
// (as one run of bytes where they hold no pointers), a pointer as whether it
// is nil and then what it points at, a struct field by field. The first error
// sticks and every later call is a no-op.
type historyCodec struct {
	w     *bufio.Writer
	r     *bufio.Reader
	limit uint64 // the most bytes one slice read may ask for
	err   error
}

// flat reports whether a value of type t is only numbers: no pointer, slice,
// string, map or anything else that refers to memory elsewhere.
func flat(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return true
	case reflect.Array:
		return flat(t.Elem())
	case reflect.Struct:
		for i := range t.NumField() {
			if !flat(t.Field(i).Type) {
				return false
			}
		}
		return true
	}
	return false
}

// raw is the n bytes at p.
func raw(p unsafe.Pointer, n uintptr) []byte {
	if n == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(p), n)
}

func (c *historyCodec) uint(v uint64) {
	if c.err == nil {
		c.err = binary.Write(c.w, binary.LittleEndian, v)
	}
}

func (c *historyCodec) bytes(b []byte) {
	c.uint(uint64(len(b)))
	if c.err == nil {
		_, c.err = c.w.Write(b)
	}
}

func (c *historyCodec) readUint() uint64 {
	var v uint64
	if c.err == nil {
		c.err = binary.Read(c.r, binary.LittleEndian, &v)
	}
	return v
}

func (c *historyCodec) readBytes() []byte {
	n := c.readUint()
	if c.err != nil {
		return nil
	}
	if n > c.limit {
		c.err = fmt.Errorf("%w: a run of %d bytes", ErrHistoryFile, n)
		return nil
	}
	b := make([]byte, n)
	c.fill(b)
	return b
}

func (c *historyCodec) fill(b []byte) {
	if c.err == nil && len(b) > 0 {
		if _, err := io.ReadFull(c.r, b); err != nil {
			c.err = fmt.Errorf("%w: cut short: %v", ErrHistoryFile, err)
		}
	}
}

// encode writes v, which has to be addressable, named path for the error.
func (c *historyCodec) encode(v reflect.Value, path string) {
	if c.err != nil {
		return
	}
	t := v.Type()
	if flat(t) {
		_, c.err = c.w.Write(raw(unsafe.Pointer(v.UnsafeAddr()), t.Size()))
		return
	}
	switch t.Kind() {
	case reflect.Slice:
		if v.IsNil() {
			c.uint(^uint64(0))
			return
		}
		c.uint(uint64(v.Len()))
		if flat(t.Elem()) {
			if c.err == nil {
				_, c.err = c.w.Write(raw(v.UnsafePointer(), uintptr(v.Len())*t.Elem().Size()))
			}
			return
		}
		for i := range v.Len() {
			c.encode(v.Index(i), fmt.Sprintf("%s[%d]", path, i))
		}
	case reflect.Array:
		for i := range v.Len() {
			c.encode(v.Index(i), fmt.Sprintf("%s[%d]", path, i))
		}
	case reflect.Struct:
		for i := range t.NumField() {
			c.encode(v.Field(i), path+"."+t.Field(i).Name)
		}
	case reflect.Pointer:
		if v.IsNil() {
			c.uint(0)
			return
		}
		c.uint(1)
		c.encode(v.Elem(), "*"+path)
	default:
		c.err = fmt.Errorf("a history cannot keep %s, a %s: make it a kind it can, or name it in historyDropped", path, t)
	}
}

// decode reads into v, which has to be addressable, what encode wrote of a
// value of its type.
func (c *historyCodec) decode(v reflect.Value, path string) {
	if c.err != nil {
		return
	}
	t := v.Type()
	// A settable view of v, which may be an unexported field.
	v = reflect.NewAt(t, unsafe.Pointer(v.UnsafeAddr())).Elem()
	if flat(t) {
		c.fill(raw(unsafe.Pointer(v.UnsafeAddr()), t.Size()))
		return
	}
	switch t.Kind() {
	case reflect.Slice:
		n := c.readUint()
		if c.err != nil {
			return
		}
		if n == ^uint64(0) {
			v.SetZero()
			return
		}
		if size := uint64(max(1, t.Elem().Size())); n > c.limit/size {
			c.err = fmt.Errorf("%w: %s has %d elements", ErrHistoryFile, path, n)
			return
		}
		s := reflect.MakeSlice(t, int(n), int(n))
		if flat(t.Elem()) {
			c.fill(raw(s.UnsafePointer(), uintptr(n)*t.Elem().Size()))
		} else {
			for i := range int(n) {
				c.decode(s.Index(i), fmt.Sprintf("%s[%d]", path, i))
			}
		}
		v.Set(s)
	case reflect.Array:
		for i := range v.Len() {
			c.decode(v.Index(i), fmt.Sprintf("%s[%d]", path, i))
		}
	case reflect.Struct:
		for i := range t.NumField() {
			c.decode(v.Field(i), path+"."+t.Field(i).Name)
		}
	case reflect.Pointer:
		switch c.readUint() {
		case 0:
			v.SetZero()
		case 1:
			p := reflect.New(t.Elem())
			c.decode(p.Elem(), "*"+path)
			v.Set(p)
		default:
			c.err = fmt.Errorf("%w: %s", ErrHistoryFile, path)
		}
	default:
		c.err = fmt.Errorf("a history cannot hold %s, a %s", path, t)
	}
}
