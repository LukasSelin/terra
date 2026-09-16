package terra

import (
	"bufio"
	"bytes"
	"errors"
	"reflect"
	"runtime"
	"testing"
)

// The world a history is kept from, the world made from the kept history, and
// the world NewLand makes are one world: the same digest, and every field of
// the grid the file keeps equal, the ledger, the lakes and the weather among
// them, and the same features.
func TestAWorldResumedFromItsHistoryIsTheSameWorld(t *testing.T) {
	for _, w := range budgetWorlds {
		t.Run(w.name, func(t *testing.T) {
			if testing.Short() && w.terms().Wrap {
				t.Skip("a globe takes seconds to make, three times over")
			}
			want := NewLand(1, w.terms())
			var file bytes.Buffer
			kept, err := MakeLandKeepingHistory(1, w.terms(), &file)
			if err != nil {
				t.Fatal(err)
			}
			size := file.Len()
			resumed, err := LandFromHistory(&file)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%s: the history is %.1f MiB", w.name, float64(size)/(1<<20))
			for name, l := range map[string]*Land{"kept": kept, "resumed": resumed} {
				if got, wants := digest(l), digest(want); got != wants {
					t.Fatalf("the %s world is %s, and NewLand's is %s", name, got, wants)
				}
				sameGrids(t, name, l.Grid, want.Grid)
				if !reflect.DeepEqual(l.Grid.features, want.Grid.features) {
					t.Errorf("the %s world's features are not NewLand's", name)
				}
				chance, _ := l.source.MarshalBinary()
				wants, _ := want.source.MarshalBinary()
				if l.Forest0 != want.Forest0 || !bytes.Equal(chance, wants) {
					t.Errorf("the %s world's forest or chance is not NewLand's", name)
				}
			}
		})
	}
}

// sameGrids fails t for every field a history keeps that differs between
// got and want, bit for bit: a NaN, which the deep floor marks tiles with,
// is equal to itself here.
func sameGrids(t *testing.T, name string, got, want *Grid) {
	t.Helper()
	g, w := reflect.ValueOf(got).Elem(), reflect.ValueOf(want).Elem()
	for _, f := range historyFields() {
		if !bytes.Equal(bitsOf(t, g.Field(f.Index[0])), bitsOf(t, w.Field(f.Index[0]))) {
			t.Errorf("the %s world's Grid.%s is not NewLand's", name, f.Name)
		}
	}
}

// bitsOf is v as a history file writes it.
func bitsOf(t *testing.T, v reflect.Value) []byte {
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	c := historyCodec{w: w}
	c.encode(v, "v")
	if c.err != nil {
		t.Fatal(c.err)
	}
	w.Flush()
	return b.Bytes()
}

// A history resumes to the same world over any number of goroutines, since
// a world does not depend on them.
func TestAHistoryResumesTheSameOverAnyGoroutines(t *testing.T) {
	terms := AncientTerms()
	var file bytes.Buffer
	if _, err := MakeLandKeepingHistory(1, terms, &file); err != nil {
		t.Fatal(err)
	}
	want := digest(NewLand(1, terms))
	was := Workers
	defer func() { Workers = was }()
	for _, workers := range []int{1, 3, 8} {
		Workers = workers
		l, err := LandFromHistory(bytes.NewReader(file.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		if got := digest(l); got != want {
			t.Errorf("%d goroutines resumed %s, and NewLand made %s", workers, got, want)
		}
	}
}

// A file that is not a whole history this build wrote is refused, and says
// so, rather than read into a world.
func TestAHistoryFileIsRefusedWhenItIsNotOne(t *testing.T) {
	var file bytes.Buffer
	if _, err := MakeLandKeepingHistory(1, DefaultTerms(), &file); err != nil {
		t.Fatal(err)
	}
	whole := file.Bytes()
	header := len(historyMagic) + 8
	cases := map[string][]byte{
		"empty":         nil,
		"not a history": []byte("P6\n80 36\n255\n"),
		"cut short":     whole[:len(whole)/2],
		"another version": func() []byte {
			b := bytes.Clone(whole)
			b[header]++
			return b
		}(),
		"another layout": func() []byte {
			b := bytes.Clone(whole)
			b[header+8+8+len(runtime.GOARCH)]++ // past the version and the architecture
			return b
		}(),
	}
	for name, b := range cases {
		if _, err := LandFromHistory(bytes.NewReader(b)); !errors.Is(err, ErrHistoryFile) {
			t.Errorf("%s: got %v, want ErrHistoryFile", name, err)
		}
	}
}

// Every field historyDropped names is a field of the Grid, so that a field
// renamed or removed does not leave a reason standing for nothing.
func TestTheFieldsAHistoryDropsAreTheGrids(t *testing.T) {
	grid := reflect.TypeFor[Grid]()
	for name, why := range historyDropped {
		if _, ok := grid.FieldByName(name); !ok {
			t.Errorf("historyDropped names %s (%s), which the Grid does not have", name, why)
		}
		if why == "" {
			t.Errorf("historyDropped gives no reason for %s", name)
		}
	}
}
