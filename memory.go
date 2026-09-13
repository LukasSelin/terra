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
const (
	bytesDrawn = 256
	bytesRun   = 448
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
