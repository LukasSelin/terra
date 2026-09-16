package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/zarr"
)

// The same world made twice and written twice is the same to zarrdiff, and
// a world of another seed is not: zarrdiff is built and run as a script
// would run it, for its exit status.
func TestZarrdiffSeesTheSameWorldAsTheSame(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go command to build zarrdiff with")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "zarrdiff")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command(gobin, "build", "-o", bin, "./zarrdiff").CombinedOutput(); err != nil {
		t.Fatalf("building zarrdiff: %v\n%s", err, out)
	}
	store := func(name string, seed uint64) string {
		path := filepath.Join(dir, name)
		if _, err := export(ctx, terra.NewLand(seed, terra.AncientTerms()), zarr.NewDirStore(path), small); err != nil {
			t.Fatal(err)
		}
		return path
	}
	a, b, c := store("a.zarr", 3), store("b.zarr", 3), store("c.zarr", 4)

	status := func(x, y string) int {
		out, err := exec.Command(bin, x, y).CombinedOutput()
		var exit *exec.ExitError
		switch {
		case err == nil:
			return 0
		case errors.As(err, &exit):
			t.Logf("zarrdiff %s %s:\n%s", filepath.Base(x), filepath.Base(y), out)
			return exit.ExitCode()
		}
		t.Fatal(err)
		return -1
	}
	if got := status(a, b); got != 0 {
		t.Errorf("the same world exits %d", got)
	}
	if got := status(a, c); got != 1 {
		t.Errorf("another seed exits %d", got)
	}
}
