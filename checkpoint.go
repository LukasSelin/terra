package terra

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"time"

	"github.com/LukasSelin/terra/internal/phase"
)

// A checkpoint is a history stopped between two of its epochs: the grid the
// history runs on, what it carries from one epoch to the next (running), and
// where the land's chance had got to, with the seed and the terms. A making
// that is stopped - killed, crashed, or the machine shut - is taken up again
// from the last epoch kept, and the world it ends with is the world made
// straight through, to the bit: TestAWorldResumedFromACheckpointIsTheSameWorld
// holds that for several epochs.
//
// It is a history file (historyfile.go) made part-way through the ground
// stage rather than after it, and it is written and read the same way: every
// field of the Grid a history file keeps and every field of running, by
// reflection, as memory lies, under a header that says whose memory it is. A
// field added to running is kept without anybody remembering to keep it.
//
// It is written to a file beside it and renamed over it, so that a making
// stopped while one is being written leaves the last one whole.

const (
	checkpointMagic   = "terra checkpoint\n"
	checkpointVersion = 1
)

// ErrCheckpoint is what taking up a checkpoint fails with when the file is
// not one this build wrote, or was kept from another world: another seed,
// other terms, or a grid laid out another way.
var ErrCheckpoint = errors.New("not a checkpoint of this world this build can read")

// Making is how a making is followed and kept, for MakeLandWith. Its zero
// value is MakeLand.
type Making struct {
	// History, where it is not nil, is written the world's history once the
	// ground stage is over, as MakeLandKeepingHistory writes it.
	History io.Writer
	// Stages, where it is not nil, is told of each stage as it ends.
	Stages StageWatch
	// Epochs, where it is not nil, is told of each epoch of the history as
	// it ends.
	Epochs EpochWatch
	// Checkpoint, where it is not "", is a file the history is kept in after
	// each epoch, and taken up from where it already holds one: the making
	// then runs only the epochs the file has not, and the stages after. A
	// file kept from another seed or other terms is refused with
	// ErrCheckpoint. It is left behind when the world is made, holding the
	// whole history, and a making on it again runs only the stages after.
	Checkpoint string
	// CheckpointEvery is the least time between two checkpoints: an epoch
	// that ends sooner than that after the last one written is not kept,
	// unless it is the last. Nought keeps every epoch.
	CheckpointEvery time.Duration
}

// EpochDone is what an EpochWatch is told as an epoch of a history ends.
type EpochDone struct {
	// Epoch is how many epochs have been run, the one ending among them, of
	// the Epochs the history runs.
	Epoch, Epochs int
	// From is how many had been run when this making took the history up:
	// nought, or the epoch a checkpoint was kept at.
	From int
	// Took is the time since this making took the history up.
	Took time.Duration
	// Kept is how long writing the checkpoint took, where one was written
	// after this epoch, and nought where none was.
	Kept time.Duration
}

// Left is a guess at how long the epochs still to run will take: as long
// each as the ones this making has run took on the mean.
func (e EpochDone) Left() time.Duration {
	run := e.Epoch - e.From
	if run <= 0 {
		return 0
	}
	return e.Took / time.Duration(run) * time.Duration(e.Epochs-e.Epoch)
}

// An EpochWatch is told of each epoch of a history as it ends: a way to show
// how far a long making has got. It is told nothing of the world, and the
// world is the same world whether it is there or not.
type EpochWatch func(EpochDone)

// MakeLandWith is MakeLand, followed and kept as m says. The world is the
// same world whatever m is.
func MakeLandWith(seed uint64, t Terms, m Making) (land *Land, err error) {
	if err := t.Check(); err != nil {
		return nil, err
	}
	if err := t.fits(); err != nil {
		return nil, err
	}
	if m.Checkpoint != "" && t.Epochs == 0 {
		return nil, fmt.Errorf("a drawn map has no history to keep in %s", m.Checkpoint)
	}
	defer phase.Start("Generate")()
	l := unmade(seed, t)
	g := l.newGround(t)
	if m.Epochs != nil || m.Checkpoint != "" {
		mk := &making{watch: m.Epochs, path: m.Checkpoint, every: m.CheckpointEvery}
		if m.Checkpoint != "" {
			if err := mk.take(l, g, t); err != nil {
				return nil, err
			}
		}
		l.making = mk
		defer func() { l.making = nil }()
		// A checkpoint that cannot be written stops the making there, rather
		// than an hour later with nothing kept.
		defer func() {
			if r := recover(); r != nil {
				f, ok := r.(checkpointFailed)
				if !ok {
					panic(r)
				}
				land, err = nil, f.err
			}
		}()
	}
	l.generateFrom(g, t, stageGround, stageSea, m.Stages)
	if m.History != nil {
		if err := l.writeHistory(m.History, g, stageSea); err != nil {
			return nil, err
		}
	}
	l.generateFrom(g, t, stageSea, len(stages), m.Stages)
	l.handOver()
	return l, nil
}

// CheckpointTerms reads the seed and the terms a checkpoint was kept on, and
// how many epochs of the history it holds.
func CheckpointTerms(path string) (seed uint64, t Terms, done int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, t, 0, err
	}
	defer f.Close()
	h, err := readCheckpointHeader(bufio.NewReader(f))
	return h.seed, h.terms, h.next, err
}

// making is what a land being made with MakeLandWith carries while its
// history runs: whom to tell of the epochs, where to keep them, and the
// history taken up from a checkpoint until the ground stage takes it.
type making struct {
	watch EpochWatch
	path  string
	every time.Duration

	from  int       // the epoch this making took the history up at
	start time.Time // and when
	wrote time.Time // when the last checkpoint was written

	grid *Grid
	run  *running
}

// checkpointFailed is how a checkpoint that cannot be written stops a making
// from inside its history. See MakeLandWith.
type checkpointFailed struct{ err error }

// taken is the history's grid and its running taken up from a checkpoint,
// once, or nil where there is none.
func (mk *making) taken() (*Grid, *running) {
	if mk == nil || mk.grid == nil {
		return nil, nil
	}
	g, h := mk.grid, mk.run
	mk.grid, mk.run = nil, nil
	return g, h
}

// begin notes that the history has been taken up at epoch from.
func (mk *making) begin(from int) {
	if mk == nil {
		return
	}
	mk.from, mk.start = from, time.Now()
	mk.wrote = mk.start
}

// ended keeps a checkpoint of l's history after an epoch where it is asked
// for, and tells the watch. g is the grid the history runs on and h where it
// has got to. climate is whether the deep climate's study is on, whose state
// a checkpoint does not keep.
func (mk *making) ended(l *Land, g *Grid, h *running, epochs int, climate bool) {
	if mk == nil {
		return
	}
	done := EpochDone{Epoch: h.next, Epochs: epochs, From: mk.from}
	if mk.path != "" && (h.next == epochs || time.Since(mk.wrote) >= mk.every) {
		if climate {
			panic(checkpointFailed{fmt.Errorf("a checkpoint cannot keep the deep climate's study")})
		}
		began := time.Now()
		if err := l.keepCheckpoint(mk.path, g, h); err != nil {
			panic(checkpointFailed{err})
		}
		mk.wrote = time.Now()
		done.Kept = mk.wrote.Sub(began)
	}
	done.Took = time.Since(mk.start)
	if mk.watch != nil {
		mk.watch(done)
	}
}

// take takes up the checkpoint at mk.path for l, whose map is g, where there
// is one: l's chance is put where the checkpoint's was, and the history's
// grid and running are kept for the ground stage. Where there is no file
// there is nothing to take up.
func (mk *making) take(l *Land, g *Grid, t Terms) error {
	f, err := os.Open(mk.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	h, err := readCheckpointHeader(r)
	if err != nil {
		return fmt.Errorf("%s: %w", mk.path, err)
	}
	if h.seed != l.seed || !bytes.Equal(termsBits(h.terms), termsBits(t)) {
		return fmt.Errorf("%s: %w: it was kept from seed %d on other terms (%dx%d, %d epochs); move it aside to make this one",
			mk.path, ErrCheckpoint, h.seed, h.terms.Width, h.terms.Height, h.terms.Epochs)
	}
	if h.next > t.Epochs {
		return fmt.Errorf("%s: %w: %d epochs of a history of %d", mk.path, ErrCheckpoint, h.next, t.Epochs)
	}
	hg := l.historyGround(g, t)
	w, ht := hg.W, hg.H
	run := new(running)
	d := historyCodec{r: r, limit: checkpointLimit(w, ht)}
	for _, f := range historyFields() {
		d.decode(reflect.ValueOf(hg).Elem().Field(f.Index[0]), "Grid."+f.Name)
	}
	d.decode(reflect.ValueOf(run).Elem(), "running")
	if d.err != nil {
		return fmt.Errorf("%s: %w: %w", mk.path, ErrCheckpoint, d.err)
	}
	if hg.W != w || hg.H != ht || run.next != h.next {
		return fmt.Errorf("%s: %w: a %dx%d grid at epoch %d where a %dx%d one at %d was meant", mk.path, ErrCheckpoint, hg.W, hg.H, run.next, w, ht, h.next)
	}
	if err := l.source.UnmarshalBinary(h.chance); err != nil {
		return fmt.Errorf("%s: %w: the chance: %v", mk.path, ErrCheckpoint, err)
	}
	mk.grid, mk.run = hg, run
	return nil
}

// checkpointLimit is the most bytes one slice of a checkpoint of a w by h
// history grid may ask for: the crust's flexure works in a padded field of
// complex numbers, and the book holds most of a hundred bytes a tile.
func checkpointLimit(w, h int) uint64 {
	return 1024*uint64(w)*uint64(h) + 1<<24
}

// termsBits is t as a file holds it, which is what two terms are compared by.
func termsBits(t Terms) []byte {
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	c := historyCodec{w: w}
	c.encode(reflect.ValueOf(&t).Elem(), "Terms")
	w.Flush()
	return b.Bytes()
}

// keepCheckpoint writes l's history, run on g as far as h, to path: to a file
// beside it first, synced, and then renamed over it.
func (l *Land) keepCheckpoint(path string, g *Grid, h *running) (err error) {
	defer phase.Start("checkpoint")()
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(f.Name())
		}
	}()
	if err := l.writeCheckpoint(f, g, h); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		os.Remove(f.Name())
		return err
	}
	return nil
}

// writeCheckpoint writes l's history, run on g as far as h, to out.
func (l *Land) writeCheckpoint(out io.Writer, g *Grid, h *running) error {
	w := bufio.NewWriterSize(out, 1<<20)
	chance, err := l.source.MarshalBinary()
	if err != nil {
		return err
	}
	e := historyCodec{w: w}
	e.bytes([]byte(checkpointMagic))
	e.uint(checkpointVersion)
	e.bytes([]byte(runtime.GOARCH))
	e.uint(checkpointLayout())
	e.uint(l.seed)
	e.encode(reflect.ValueOf(&l.Terms).Elem(), "Terms")
	e.uint(uint64(h.next))
	e.bytes(chance)
	for _, f := range historyFields() {
		e.encode(reflect.ValueOf(g).Elem().Field(f.Index[0]), "Grid."+f.Name)
	}
	e.encode(reflect.ValueOf(h).Elem(), "running")
	if e.err != nil {
		return e.err
	}
	return w.Flush()
}

func readCheckpointHeader(r *bufio.Reader) (historyHeader, error) {
	var h historyHeader
	d := historyCodec{r: r, limit: 1 << 16}
	if string(d.readBytes()) != checkpointMagic || d.err != nil {
		return h, fmt.Errorf("%w: no checkpoint header", ErrCheckpoint)
	}
	if v := d.readUint(); v != checkpointVersion {
		return h, fmt.Errorf("%w: version %d, and this build reads %d", ErrCheckpoint, v, checkpointVersion)
	}
	if arch := string(d.readBytes()); arch != runtime.GOARCH {
		return h, fmt.Errorf("%w: written on %s, read on %s", ErrCheckpoint, arch, runtime.GOARCH)
	}
	if lay := d.readUint(); lay != checkpointLayout() {
		return h, fmt.Errorf("%w: the history's layout has changed since it was written", ErrCheckpoint)
	}
	h.seed = d.readUint()
	d.decode(reflect.ValueOf(&h.terms).Elem(), "Terms")
	h.next = int(d.readUint())
	h.chance = d.readBytes()
	if d.err != nil {
		return h, fmt.Errorf("%w: %v", ErrCheckpoint, d.err)
	}
	if err := h.terms.Check(); err != nil {
		return h, fmt.Errorf("%w: %v", ErrCheckpoint, err)
	}
	return h, nil
}

// checkpointLayout is historyLayout with running's layout beside it.
func checkpointLayout() uint64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%d:", historyLayout())
	layoutOf(h, reflect.TypeFor[running]())
	return h.Sum64()
}
