package terra

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"
)

// The checkpointed worlds: the valley made from its history, and a small
// globe, whose history runs on the map's own grid with a sea poured on it.
var checkpointWorlds = []struct {
	name  string
	terms func() Terms
	stops []int // the epochs a making is stopped after
}{
	{"ancient", AncientTerms, []int{1, 2, 7, 15, 16}},
	{"globe128", func() Terms {
		t := GlobeTerms()
		t.Width, t.Height = 128, 64
		return t
	}, []int{1, 8, 16}},
}

// A world made keeping a checkpoint after every epoch is the world made
// straight through; and a world taken up from any of those checkpoints - a
// making stopped after that epoch - is that world too: the same digest, every
// field of the grid a history keeps equal bit for bit, the same features and
// the land's chance at the same place.
func TestAWorldResumedFromACheckpointIsTheSameWorld(t *testing.T) {
	for _, w := range checkpointWorlds {
		t.Run(w.name, func(t *testing.T) {
			terms := w.terms()
			if testing.Short() && terms.Wrap {
				t.Skip("a globe takes seconds to make, many times over")
			}
			want := madeLand(1, terms)
			dir := t.TempDir()
			path := filepath.Join(dir, "world.ckpt")
			var seen []int
			kept, err := MakeLandWith(1, terms, Making{Checkpoint: path, Epochs: func(e EpochDone) {
				seen = append(seen, e.Epoch)
				if e.Kept == 0 {
					t.Errorf("epoch %d was not kept", e.Epoch)
				}
				if slices.Contains(w.stops, e.Epoch) {
					b, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if e.Epoch == 1 {
						t.Logf("%s: a checkpoint is %.1f MiB and took %v to write", w.name, float64(len(b))/(1<<20), e.Kept)
					}
					if err := os.WriteFile(fmt.Sprintf("%s.%d", path, e.Epoch), b, 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}})
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(seen, seq(1, terms.Epochs)) {
				t.Errorf("the watch was told of epochs %v", seen)
			}
			sameWorld(t, "kept", kept, want)
			for _, k := range w.stops {
				file := fmt.Sprintf("%s.%d", path, k)
				var from []int
				resumed, err := MakeLandWith(1, terms, Making{Checkpoint: file, Epochs: func(e EpochDone) {
					from = append(from, e.From)
				}})
				if err != nil {
					t.Fatal(err)
				}
				if len(from) != terms.Epochs-k || (len(from) > 0 && from[0] != k) {
					t.Errorf("taken up after epoch %d, the history ran %d epochs from %v", k, len(from), from)
				}
				sameWorld(t, fmt.Sprintf("taken up after epoch %d", k), resumed, want)
			}
		})
	}
}

// seq is from, from+1, ..., to.
func seq(from, to int) []int {
	var s []int
	for i := from; i <= to; i++ {
		s = append(s, i)
	}
	return s
}

// sameWorld fails t where got is not want, to the bit.
func sameWorld(t *testing.T, name string, got, want *Land) {
	t.Helper()
	if g, w := digest(got), digest(want); g != w {
		t.Fatalf("the %s world is %s, and NewLand's is %s", name, g, w)
	}
	sameGrids(t, name, got.Grid, want.Grid)
	if !reflect.DeepEqual(got.Grid.features, want.Grid.features) {
		t.Errorf("the %s world's features are not NewLand's", name)
	}
	chance, _ := got.source.MarshalBinary()
	wants, _ := want.source.MarshalBinary()
	if got.Forest0 != want.Forest0 || !bytes.Equal(chance, wants) {
		t.Errorf("the %s world's forest or chance is not NewLand's", name)
	}
}

// A making stopped part-way - here by a panic out of the watch, as a crash
// would stop it - leaves its last checkpoint whole, and the same making run
// again on the same file takes up the history after it and makes the world.
func TestAStoppedMakingGoesOnFromItsLastEpoch(t *testing.T) {
	terms := AncientTerms()
	path := filepath.Join(t.TempDir(), "ancient.ckpt")
	const stop = 5
	func() {
		defer func() {
			if r := recover(); r != "stopped" {
				t.Fatalf("the making ended with %v", r)
			}
		}()
		MakeLandWith(1, terms, Making{Checkpoint: path, Epochs: func(e EpochDone) {
			if e.Epoch == stop {
				panic("stopped")
			}
		}})
	}()
	if seed, got, done, err := CheckpointTerms(path); err != nil || seed != 1 || got != terms || done != stop {
		t.Fatalf("the checkpoint holds seed %d, %d epochs of %+v (%v)", seed, done, got, err)
	}
	if leftover, _ := filepath.Glob(path + ".*.tmp"); len(leftover) > 0 {
		t.Errorf("writing it left %v", leftover)
	}
	var first EpochDone
	l, err := MakeLandWith(1, terms, Making{Checkpoint: path, Epochs: func(e EpochDone) {
		if first.Epoch == 0 {
			first = e
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	if first.Epoch != stop+1 || first.From != stop || first.Epochs != terms.Epochs {
		t.Errorf("the history was taken up at %+v", first)
	}
	sameWorld(t, "taken up", l, madeLand(1, terms))
	// And once the whole history is kept, a making on it runs no epoch.
	ran := 0
	l, err = MakeLandWith(1, terms, Making{Checkpoint: path, Epochs: func(EpochDone) { ran++ }})
	if err != nil {
		t.Fatal(err)
	}
	if ran != 0 {
		t.Errorf("a whole history kept ran %d epochs again", ran)
	}
	sameWorld(t, "made on a whole history", l, madeLand(1, terms))
}

// A checkpoint of another world, or a file that is not a whole checkpoint
// this build wrote, is refused with ErrCheckpoint rather than taken up.
func TestACheckpointOfAnotherWorldIsRefused(t *testing.T) {
	terms := AncientTerms()
	terms.Epochs = 2
	dir := t.TempDir()
	path := filepath.Join(dir, "ancient.ckpt")
	if _, err := MakeLandWith(1, terms, Making{Checkpoint: path}); err != nil {
		t.Fatal(err)
	}
	whole, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	other := terms
	other.Epochs = 3
	wider := terms
	wider.Width += 8
	cases := []struct {
		name  string
		file  []byte
		seed  uint64
		terms Terms
	}{
		{"another seed", whole, 2, terms},
		{"more epochs", whole, 1, other},
		{"a wider map", whole, 1, wider},
		{"not a checkpoint", []byte("terra history\n"), 1, terms},
		{"cut short", whole[:len(whole)/2], 1, terms},
		{"another layout", func() []byte {
			b := bytes.Clone(whole)
			b[8+len(checkpointMagic)+8+8+len(runtime.GOARCH)]++ // past the version and the architecture
			return b
		}(), 1, terms},
	}
	for _, c := range cases {
		file := filepath.Join(dir, "case.ckpt")
		if err := os.WriteFile(file, c.file, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := MakeLandWith(c.seed, c.terms, Making{Checkpoint: file}); !errors.Is(err, ErrCheckpoint) {
			t.Errorf("%s: got %v, want ErrCheckpoint", c.name, err)
		}
	}
}

// A history run on a grid coarser than the map is kept and taken up on its
// own grid, and the world made from it is the world made straight through.
func TestACheckpointOfAHistoryOnACoarserGrid(t *testing.T) {
	was := historyShrink
	historyShrink = 2
	defer func() { historyShrink = was }()
	terms := AncientTerms()
	want := NewLand(1, terms)
	path := filepath.Join(t.TempDir(), "coarse.ckpt")
	func() {
		defer func() { recover() }()
		MakeLandWith(1, terms, Making{Checkpoint: path, Epochs: func(e EpochDone) {
			if e.Epoch == 6 {
				panic("stopped")
			}
		}})
	}()
	l, err := MakeLandWith(1, terms, Making{Checkpoint: path})
	if err != nil {
		t.Fatal(err)
	}
	sameWorld(t, "taken up on a coarser grid", l, want)
}

// Every field checkpointDropped names is a field of a struct a checkpoint
// holds, one the codec writes field by field, and has a reason.
func TestEveryFieldACheckpointDropsIsThere(t *testing.T) {
	fields := map[string]bool{}
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(ty reflect.Type) {
		if seen[ty] {
			return
		}
		seen[ty] = true
		switch ty.Kind() {
		case reflect.Struct:
			for i := range ty.NumField() {
				if !flat(ty) {
					fields[ty.Name()+"."+ty.Field(i).Name] = true
				}
				walk(ty.Field(i).Type)
			}
		case reflect.Array, reflect.Slice, reflect.Pointer:
			walk(ty.Elem())
		}
	}
	walk(reflect.TypeFor[running]())
	for _, f := range historyFields() {
		fields["Grid."+f.Name] = true
		walk(f.Type)
	}
	for name, why := range checkpointDropped {
		if !fields[name] {
			t.Errorf("checkpointDropped names %s, which a checkpoint does not hold field by field", name)
		}
		if why == "" {
			t.Errorf("checkpointDropped gives no reason for %s", name)
		}
	}
}

// What a checkpoint drops is not read: a history with every field
// checkpointDropped names spoilt after every epoch - numbers made NaN or
// flipped, pointers let go - makes the same world as one left alone.
func TestWhatACheckpointDropsIsNotRead(t *testing.T) {
	for _, w := range checkpointWorlds {
		t.Run(w.name, func(t *testing.T) {
			terms := w.terms()
			if testing.Short() && terms.Wrap {
				t.Skip("a globe takes seconds to make")
			}
			want := madeLand(1, terms)
			checkpointDropWatch = func(g *Grid, h *running) {
				spoilDropped(reflect.ValueOf(g).Elem())
				spoilDropped(reflect.ValueOf(h).Elem())
			}
			defer func() { checkpointDropWatch = nil }()
			got, err := MakeLandWith(1, terms, Making{Epochs: func(EpochDone) {}})
			if err != nil {
				t.Fatal(err)
			}
			sameWorld(t, "spoilt", got, want)
		})
	}
}

// spoilDropped spoils every field of v, a struct, that checkpointDropped
// names, and looks for more in the structs v points at.
func spoilDropped(v reflect.Value) {
	ty := v.Type()
	for i := range ty.NumField() {
		f := reflect.NewAt(ty.Field(i).Type, v.Field(i).Addr().UnsafePointer()).Elem()
		if _, ok := checkpointDropped[ty.Name()+"."+ty.Field(i).Name]; ok {
			spoil(f)
			continue
		}
		if f.Kind() == reflect.Pointer && !f.IsNil() && f.Elem().Kind() == reflect.Struct && f.Type() != reflect.TypeFor[*Grid]() {
			spoilDropped(f.Elem())
		}
	}
}

// spoil makes v nonsense: its numbers NaN or flipped, its bools turned,
// its pointers nil.
func spoil(v reflect.Value) {
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		v.SetFloat(math.NaN())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(v.Int() ^ 0x55)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		v.SetUint(v.Uint() ^ 0x55)
	case reflect.Bool:
		v.SetBool(!v.Bool())
	case reflect.Complex64, reflect.Complex128:
		v.SetComplex(complex(math.NaN(), math.NaN()))
	case reflect.Pointer:
		v.SetZero()
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			spoil(v.Index(i))
		}
	case reflect.Struct:
		for i := range v.NumField() {
			spoil(reflect.NewAt(v.Field(i).Type(), v.Field(i).Addr().UnsafePointer()).Elem())
		}
	}
}

// What the packing writes comes back as it was: slices of noughts, slices
// shared with another, elements of every size and slices of many blocks,
// through the compression.
func TestPackedSlicesComeBackAsTheyWere(t *testing.T) {
	type odd struct {
		a float32
		b [3]uint8
		c float64
	}
	type held struct {
		Zero   []float64
		Many   []float64
		Shared []float64
		Odd    []odd
		Flags  []bool
		None   []int32
		Empty  []uint16
		Phases [4][]float32
		Gone   []float64
	}
	var h held
	h.Zero = make([]float64, 1000)
	h.Many = make([]float64, shuffleBlock*shuffleSide*2+77)
	for i := range h.Many {
		h.Many[i] = math.Sin(float64(i) / 1000)
	}
	h.Shared = h.Many
	h.Odd = make([]odd, shuffleBlock+5)
	for i := range h.Odd {
		h.Odd[i] = odd{float32(i), [3]uint8{uint8(i), 1, 2}, -float64(i)}
	}
	h.Flags = []bool{true, false, true}
	h.Empty = []uint16{}
	for k := range h.Phases {
		h.Phases[k] = make([]float32, 300)
		h.Phases[k][k] = float32(k)
	}
	h.Phases[3] = h.Phases[1]
	h.Gone = []float64{1, 2, 3}
	drop := map[string]string{"held.Gone": "a test's"}

	var file bytes.Buffer
	body := newPackWriter(&file)
	w := bufio.NewWriter(body)
	e := historyCodec{w: w, drop: drop, pack: true}
	e.encode(reflect.ValueOf(&h).Elem(), "held")
	if e.err != nil {
		t.Fatal(e.err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	if raw := 8 * len(h.Many); file.Len() >= raw {
		t.Errorf("packed in %d bytes, where the one slice of it that is not noughts or shared is %d", file.Len(), raw)
	}

	r := newPackReader(bufio.NewReader(&file))
	defer r.Close()
	var got held
	got.Gone = []float64{9}
	d := historyCodec{r: bufio.NewReader(r), limit: 1 << 30, drop: drop, pack: true}
	d.decode(reflect.ValueOf(&got).Elem(), "held")
	if d.err != nil {
		t.Fatal(d.err)
	}
	want := h
	want.Gone = []float64{9}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the packing did not come back as it was")
	}
	if &got.Shared[0] != &got.Many[0] || &got.Phases[3][0] != &got.Phases[1][0] {
		t.Errorf("the slices shared are not shared again")
	}
	if got.None != nil || got.Empty == nil {
		t.Errorf("a nil slice came back %v and an empty one %v", got.None, got.Empty)
	}
}
