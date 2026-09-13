package terra

import (
	"errors"
	"math"
	"runtime/debug"
	"testing"
)

// A world asked for bigger than there is memory for is refused before any of
// it is made, rather than killing the program half way through the making.
func TestAWorldTooBigIsRefusedNotStarted(t *testing.T) {
	for _, bad := range []Terms{
		{Width: 0, Height: 10},
		{Width: 10, Height: -1},
		{Width: 100, Height: 64, Wrap: true},
		{Width: 1 << 20, Height: 1 << 20},
	} {
		if _, err := MakeLand(1, bad); err == nil {
			t.Errorf("%+v: made", bad)
		}
	}

	old := debug.SetMemoryLimit(64 << 20)
	defer debug.SetMemoryLimit(old)
	big := Terms{Width: 1024, Height: 1024, Epochs: 16}
	if _, err := MakeLand(1, big); !errors.Is(err, ErrTooBig) {
		t.Errorf("%+v under a 64 MiB limit: %v, want ErrTooBig", big, err)
	}
	debug.SetMemoryLimit(math.MaxInt64)
	if _, err := MakeLand(1, DefaultTerms()); err != nil {
		t.Errorf("the default valley: %v", err)
	}
}

// The largest world is as big as the memory will take and no bigger, in the
// shape it was asked for.
func TestTheLargestWorldFillsTheMemoryInItsShape(t *testing.T) {
	const limit = 256 << 20
	old := debug.SetMemoryLimit(limit)
	defer debug.SetMemoryLimit(old)
	for _, asked := range []Terms{GlobeTerms(), AncientTerms(), {Width: 300, Height: 900}} {
		got, err := asked.Largest()
		if err != nil {
			t.Fatalf("%+v: %v", asked, err)
		}
		if got.Bytes() > limit || got.Bytes() < limit/2 {
			t.Errorf("%+v: %dx%d needs %d of %d", asked, got.Width, got.Height, got.Bytes(), limit)
		}
		if err := got.Check(); err != nil {
			t.Errorf("%+v: %v", asked, err)
		}
		want := float64(asked.Width) / float64(asked.Height)
		if r := float64(got.Width) / float64(got.Height); math.Abs(r-want) > want*0.05 {
			t.Errorf("%+v: %dx%d is not its shape", asked, got.Width, got.Height)
		}
		if got.Wrap != asked.Wrap || got.Epochs != asked.Epochs {
			t.Errorf("%+v: became %+v", asked, got)
		}
	}
	debug.SetMemoryLimit(1 << 10)
	if _, err := GlobeTerms().Largest(); !errors.Is(err, ErrTooBig) {
		t.Errorf("a globe in a kilobyte: %v, want ErrTooBig", err)
	}
}
