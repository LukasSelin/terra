package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// A world drawn from a history -keep-history wrote is the world drawn while
// it was written: every map and both pages the same, but for the time the
// making took.
func TestAWorldFromItsHistoryDrawsTheSame(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "ancient.history")
	kept := options{Seed: 3, Preset: "ancient", Epochs: -1, Sea: -1, Water: -1, KeepHistory: file}
	if _, err := generate(kept, filepath.Join(dir, "kept")); err != nil {
		t.Fatal(err)
	}
	resumed := options{Preset: "valley", Epochs: -1, Sea: -1, Water: -1, FromHistory: file}
	if _, err := generate(resumed, filepath.Join(dir, "resumed")); err != nil {
		t.Fatal(err)
	}
	pngs, err := filepath.Glob(filepath.Join(dir, "kept", "*.png"))
	if err != nil || len(pngs) == 0 {
		t.Fatalf("no maps drawn: %v", err)
	}
	for _, name := range append(pngs, filepath.Join(dir, "kept", "why.html")) {
		name = filepath.Base(name)
		a, errA := os.ReadFile(filepath.Join(dir, "kept", name))
		b, errB := os.ReadFile(filepath.Join(dir, "resumed", name))
		if errA != nil || errB != nil {
			t.Fatal(errA, errB)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("%s differs between the world kept and the world resumed", name)
		}
	}
}
