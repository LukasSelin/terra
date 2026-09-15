package terra

import (
	"errors"
	"fmt"
	"math"
	"runtime/debug"
)

// How big a world may be asked for.
//
// A land is made all at once: every tile is on the map from the first
// moment, and the making reads and writes all of them together. There is
// no making part of one, so a map that will not fit in memory is not made
// slowly - the allocation for it fails, and a Go program that cannot have
// its memory does not return an error, it dies. The only place to say no
// is before anything is allocated, and that is what Check and MakeLand do.

// bytesDrawn and bytesRun are what making a land costs at its height, per
// tile: drawn, and made out of its own history, which keeps the seams and
// the plates beside the ground while the epochs run. They are measured at
// a little over 200 and 350, and rounded up so that the reading is of what
// a land takes at most rather than on the day it was measured.
//
// The run was raised when the water began to be cut implicitly: the solver
// keeps its order, its receivers and a load of each grain beside the ground
// while each epoch wears, and the rain and runoff are kept on the map. Peak
// heap on a 1024 by 512 globe rose by 29 per cent against the same world
// before it, and a drawn valley's by 4; 350 grown by that is 450, over the
// line it was rounded to.
// The air keeps fields of its own beside the ground - the wind and the
// pressure of each phase of the year, and what the ground under each air cell
// is - and an air cell is several tiles, so all of it comes to some forty
// bytes a tile. Measured on a 1024 by 512 globe, the peak heap rose by five
// and a half per cent when the wind came in, which these figures cover.
//
// The rock became a pile of beds under every tile - see strata.go - which is
// sixty bytes a tile kept, a second copy of it while the plates move, and the
// sorted heights the beds are carried through each rescaling on. Measured
// against the same worlds before it, the peak heap of a drawn 1024 by 512
// globe rose by 36 per cent and a made one's by 8, which these are raised by.
//
// The run was raised again when the soil began to be kept. The soil itself is
// free - a tile's thickness sits in padding the tile already had - but each
// epoch's wear keeps the soil, the rock's rate, the rate the step settles on
// and the sand's wear beside the ground, and the creep is solved implicitly
// with a coefficient and a neighbour for each pair of tiles. Peak heap sampled
// over the making of a 1024 by 512 globe, before the beds, rose from 754 bytes
// a tile to 835, eleven per cent; 576 grown by that is 638. A drawn map, which
// wears nothing while it is made, read 257 before and after.
//
// What time has made of the soil - its age, its leaching, its carbonate, salt
// and carbon - is five more numbers a tile, and costs nothing either: they lie
// in the padding between the tile's fields, and a tile is 72 bytes with them
// as it was without (see TestTheSoilStateCostsATileNothing).
const (
	bytesDrawn = 384
	bytesRun   = 640
)

// ErrTooBig is what MakeLand wraps when a world would not fit in the memory
// there is to make it in.
var ErrTooBig = errors.New("terra: the world will not fit in memory")

// Check says whether these are terms a land can be made on at all: whether
// the ground has a size, whether a globe is a whole number of chunks round,
// and whether the map is small enough to be counted. It says nothing about
// the machine; see MakeLand for that.
func (t Terms) Check() error {
	if t.Width <= 0 || t.Height <= 0 {
		return fmt.Errorf("terra: a world must have ground: %dx%d", t.Width, t.Height)
	}
	if t.Wrap && t.Width%ChunkSide != 0 {
		return fmt.Errorf("terra: a globe must be a whole number of chunks round: width %d is not a multiple of %d", t.Width, ChunkSide)
	}
	if t.Woods > ByClimate || t.Growth > ByClimate {
		return fmt.Errorf("terra: no such rule: woods %d, growth %d", t.Woods, t.Growth)
	}
	if t.Water < 0 {
		return fmt.Errorf("terra: a world cannot have less than no water: %v", t.Water)
	}
	// Tiles are counted in int32 wherever a list of them is kept - the
	// order the water runs down in, the stack the regions are filled from -
	// so no map may hold more than one of those can count.
	if int64(t.Width)*int64(t.Height) > math.MaxInt32 {
		return fmt.Errorf("terra: a world may have at most %d tiles: %dx%d has %d", math.MaxInt32, t.Width, t.Height, int64(t.Width)*int64(t.Height))
	}
	return nil
}

// Bytes is about the most memory making a land on these terms will take,
// at its height. It is not what the land keeps once it is made, which is
// less, nor what a game adds by routing and building over it, which is
// more.
func (t Terms) Bytes() uint64 {
	per := uint64(bytesDrawn)
	if t.Epochs > 0 {
		per = bytesRun
	}
	return uint64(max(t.Width, 0)) * uint64(max(t.Height, 0)) * per
}

// MakeLand is NewLand for terms that come from somebody who may ask for more
// than there is: it checks the terms, and then whether the making would fit
// in the memory there is to make it in - the program's memory limit, if one
// is set, and what the machine has free, where that can be read - and says
// so rather than starting on a world it cannot finish. An error about memory
// wraps ErrTooBig.
func MakeLand(seed uint64, t Terms) (*Land, error) {
	if err := t.Check(); err != nil {
		return nil, err
	}
	if err := t.fits(); err != nil {
		return nil, err
	}
	return NewLand(seed, t), nil
}

// fits says whether making a land on these terms would fit in memory.
func (t Terms) fits() error {
	need := t.Bytes()
	if limit := debug.SetMemoryLimit(-1); limit != math.MaxInt64 && need > uint64(limit) {
		return fmt.Errorf("%w: %dx%d needs about %s, and the memory limit is %s", ErrTooBig, t.Width, t.Height, size(need), size(uint64(limit)))
	}
	if free, ok := freeMemory(); ok && need > free {
		return fmt.Errorf("%w: %dx%d needs about %s, and %s is free", ErrTooBig, t.Width, t.Height, size(need), size(free))
	}
	return nil
}

// Largest is these terms made as big as the memory there is will take: the
// same shape of ground - Width to Height in the proportion asked for, a
// globe still a whole number of chunks round - with as many tiles as making
// it can have, bigger or smaller than asked. It keeps an eighth of the
// memory back for whatever the program holds besides the land. Terms with
// no size are read as the default valley's proportions.
//
// It fails if the memory there is cannot be read - on a system where free
// memory is not known and no memory limit is set - or if not even the
// smallest world of this shape would fit, which wraps ErrTooBig.
func (t Terms) Largest() (Terms, error) {
	if t.Width <= 0 || t.Height <= 0 {
		t.Width, t.Height = DefaultWidth, DefaultHeight
	}
	budget, ok := uint64(0), false
	if limit := debug.SetMemoryLimit(-1); limit != math.MaxInt64 {
		budget, ok = uint64(limit), true
	}
	if free, known := freeMemory(); known && (!ok || free < budget) {
		budget, ok = free, true
	}
	if !ok {
		return t, errors.New("terra: cannot tell how much memory there is: set GOMEMLIMIT")
	}
	budget -= budget / 8

	per := t.Bytes() / (uint64(t.Width) * uint64(t.Height))
	tiles := min(float64(budget/per), math.MaxInt32)
	ratio := float64(t.Width) / float64(t.Height)
	w := int(math.Sqrt(tiles * ratio))
	if t.Wrap {
		w -= w % ChunkSide
	}
	h := min(int(float64(w)/ratio), int(tiles)/max(w, 1))
	if w < 1 || h < 1 || (t.Wrap && w < ChunkSide) {
		return t, fmt.Errorf("%w: not even the smallest world of this shape fits in %s", ErrTooBig, size(budget))
	}
	t.Width, t.Height = w, h
	return t, nil
}

// size is a count of bytes as somebody would say it.
func size(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTP"[exp])
}
