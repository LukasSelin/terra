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
