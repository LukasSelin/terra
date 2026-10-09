package terra

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
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
// stage rather than after it, and it is written and read by the same codec:
// every field of the Grid a history file keeps and every field of running,
// by reflection, under a header that says whose memory it is. A field added
// to running is kept without anybody remembering to keep it. Two things are
// done that a history file does not do, because a globe's checkpoint as
// memory lies was 548 MiB: the working memory the epochs rewrite before
// they read it is left out (checkpointDropped), and the rest is packed and
// compressed (checkpointpack.go). A globe's is now about a hundred MiB.
//
// It is written to a file beside it and renamed over it, so that a making
// stopped while one is being written leaves the last one whole.

const (
	checkpointMagic   = "terra checkpoint\n"
	checkpointVersion = 2
)

// checkpointDropped are the fields of a history's state a checkpoint does not
// keep, by "Type.field", and why the epochs after it, and the stages after
// the last of them, do not need them: each is written before it is read
// again. A field of the Grid is left as the history's grid is made (see
// historyGround), and a field of the crust or the floods as newCrust or
// newFlooding makes it.
// TestWhatACheckpointDropsIsNotRead runs a history with every one of them
// spoilt after every epoch and makes the same world.
var checkpointDropped = map[string]string{
	"Grid.toSea": "each epoch clears it before the weather adds to it",

	"crust.tiles":   "move copies the grid's tiles into it before it reads it",
	"crust.height":  "move copies the grid's heights into it before it reads it",
	"crust.soil":    "move copies the grid's soil into it before it reads it",
	"crust.sand":    "move copies the grid's sand into it before it reads it",
	"crust.clay":    "move copies the grid's clay into it before it reads it",
	"crust.book":    "move copies the book into it before it reads it",
	"crust.strata":  "move copies the grid's strata into it before it reads it",
	"crust.nrift":   "move's working copy: turn writes every tile of it before it is swapped in",
	"crust.nrise":   "move's working copy: turn writes every tile of it before it is swapped in",
	"crust.nthick":  "move's working copy: turn writes every tile of it before it is swapped in",
	"crust.nsag":    "move's working copy: turn writes every tile of it before it is swapped in",
	"crust.nsed":    "move's working copy: turn writes every tile of it before it is swapped in",
	"crust.nplate":  "move's working copy: turn writes every tile of it before it is swapped in",
	"crust.norg":    "move's working copy: turn writes every tile of it before it is swapped in",
	"crust.nfresh":  "move's working copy: turn writes every tile of it before it is swapped in",
	"crust.nborn":   "move's working copy: turn writes every tile of it before it is swapped in",
	"crust.naged":   "move's working copy: turn writes every tile of it before it is swapped in",
	"crust.nocean":  "move's working copy: turn writes every tile of it before it is swapped in",
	"crust.noff":    "move's working copy: turn writes every tile of it before it is swapped in",
	"crust.mark":    "move's fill of the gaps clears it before it marks it",
	"crust.ring":    "move's fill of the gaps empties it before it uses it",
	"crust.next":    "move's fill of the gaps empties it before it uses it",
	"crust.lifted":  "tectonics sets every tile of it to nought before anything adds to it",
	"crust.was":     "each epoch copies what the weather wore into it before it reads it",
	"crust.step":    "the shelves copy the heights into it before they read it",
	"crust.local":   "isostasy writes every tile of it before it reads it",
	"crust.load":    "isostasy writes every tile of it before it reads it",
	"crust.wear":    "worn copies the heights into it before it is read",
	"crust.landed":  "laidBy writes every tile of it before keepBook reads it",
	"crust.shelved": "laidBy writes every tile of it before keepBook reads it",
	"crust.plan":    "the flexure's transform and its working, made afresh the first time it is asked for",

	"flooding.dist": "partition sets every tile of it before a flood reads it",
	"flooding.from": "partition sets it on every tile a flood reaches before it reads it there",
	"flooding.done": "partition clears it before a flood reads it",
	"flooding.head": "the floods' frontier, which reset empties and a flood leaves empty",
	"flooding.next": "the floods' frontier, which reset empties and a flood leaves empty",
	"flooding.at":   "the floods' frontier, which reset empties and a flood leaves empty",

	"vapourOut.sat": "the air's budget writes it and nothing reads it",
}

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

// checkpointDropWatch, where it is not nil, is shown the history's grid and
// running after each epoch of a making with an EpochWatch or a checkpoint,
// once the checkpoint is kept: TestWhatACheckpointDropsIsNotRead spoils what
// checkpointDropped names in it.
var checkpointDropWatch func(g *Grid, h *running)

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
	if checkpointDropWatch != nil {
		checkpointDropWatch(g, h)
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
	body := newPackReader(r)
	defer body.Close()
	d := historyCodec{r: bufio.NewReaderSize(body, 1<<20), limit: checkpointLimit(w, ht), drop: checkpointDropped, pack: true,
		fresh: func(t reflect.Type) (reflect.Value, bool) {
			// The crust's and the floods' working memory is made as the
			// history makes it.
			switch t {
			case reflect.TypeFor[*crust]():
				return reflect.ValueOf(newCrust(hg)), true
			case reflect.TypeFor[*flooding]():
				return reflect.ValueOf(newFlooding(len(hg.Tiles))), true
			}
			return reflect.Value{}, false
		}}
	for _, f := range checkpointFields() {
		d.decode(reflect.ValueOf(hg).Elem().Field(f.Index[0]), "Grid."+f.Name)
	}
	d.decode(reflect.ValueOf(run).Elem(), "running")
	if d.err == nil {
		if _, err := d.r.ReadByte(); err != io.EOF {
			d.err = fmt.Errorf("more than the history in it")
		}
	}
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
	if e.err != nil {
		return e.err
	}
	if err := w.Flush(); err != nil {
		return err
	}
	// The body is packed: see checkpointpack.go.
	body := newPackWriter(w)
	e.w = bufio.NewWriterSize(body, 1<<20)
	e.drop, e.pack = checkpointDropped, true
	for _, f := range checkpointFields() {
		e.encode(reflect.ValueOf(g).Elem().Field(f.Index[0]), "Grid."+f.Name)
	}
	e.encode(reflect.ValueOf(h).Elem(), "running")
	if e.err == nil {
		e.err = e.w.Flush()
	}
	if err := body.Close(); e.err == nil {
		e.err = err
	}
	if e.err != nil {
		return e.err
	}
	return w.Flush()
}

// checkpointFields are the Grid's fields a checkpoint keeps: a history
// file's, but for the ones checkpointDropped names.
func checkpointFields() []reflect.StructField {
	var kept []reflect.StructField
	for _, f := range historyFields() {
		if _, dropped := checkpointDropped["Grid."+f.Name]; !dropped {
			kept = append(kept, f)
		}
	}
	return kept
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
	fmt.Fprintf(h, "%q", slices.Sorted(maps.Keys(checkpointDropped)))
	return h.Sum64()
}
