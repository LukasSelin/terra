package terra

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"testing"
)

// Histories kept between test runs. A world with a history is four fifths
// history, and the suite makes the same few dozen worlds on every run: kept,
// a run after the first makes each of them from its history, which is the
// world NewLand makes to the bit (TestAWorldResumedFromItsHistoryIsTheSameWorld).
//
// A history is kept under a key of its seed, its terms and the code that made
// it: every non-test file of the package and of the packages it imports from
// this module, go.mod, the Go version and the build settings. Any change to
// the code misses every kept history and makes them afresh, so a history kept
// here is never read by code that would have made a different one. What that
// costs is that a change to a stage after the history misses too; for that,
//
//	TERRA_HISTORIES=trust go test ...
//
// keys by seed and terms alone and reads whatever was kept last: the
// caller's word that the history stage has not changed. And
//
//	TERRA_HISTORIES=off go test ...
//
// makes every world from nothing, as NewLand does.
//
// The histories live in .cache/histories in the package directory, one
// directory per key of the code, so that two worktrees never share or prune
// each other's. On the first use in a run, directories for any other code
// are removed: what is kept is the histories of this code and no other.
const historiesDir = ".cache/histories"

var (
	historiesMode = os.Getenv("TERRA_HISTORIES")
	historiesOnce sync.Once
	historiesCode string // the key of the code, or "" where nothing is kept
	historyLocks  sync.Map
)

// madeLand is NewLand(seed, terms), from a kept history where there is one
// and keeping the history where there is not. It is for a test that needs a
// world of its own and not a test of how a world is made: those call NewLand.
func madeLand(seed uint64, terms Terms) *Land {
	if terms.Epochs == 0 {
		return NewLand(seed, terms) // drawn: nothing to keep
	}
	historiesOnce.Do(openHistories)
	if historiesCode == "" {
		return NewLand(seed, terms)
	}
	name := fmt.Sprintf("%d-%dx%d-e%d-w%v-s%g-water%g-wet%g-%d%d-g%v.history",
		seed, terms.Width, terms.Height, terms.Epochs, terms.Wrap, terms.SeaShare,
		terms.Water, terms.Wetness, terms.Woods, terms.Growth, terms.Glacial)
	return keptLand(filepath.Join(historiesDir, historiesCode, name), seed, terms)
}

// keptLand is NewLand(seed, terms), from the history kept at path if there is
// a readable one of that seed and those terms, and keeping it there if not.
func keptLand(path string, seed uint64, terms Terms) *Land {
	// One maker a history at a time, so that two tests asking for the same
	// world at once do not both make it.
	mu, _ := historyLocks.LoadOrStore(path, new(sync.Mutex))
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()

	if b, err := os.ReadFile(path); err == nil {
		if l, err := LandFromHistory(bytes.NewReader(b)); err == nil {
			if l.seed == seed && l.Terms == terms {
				return l
			}
		} else {
			fmt.Fprintf(os.Stderr, "histories: %s is not read (%v): making it again\n", path, err)
		}
	}
	var file bytes.Buffer
	l, err := MakeLandKeepingHistory(seed, terms, &file)
	if err != nil {
		panic(err)
	}
	if err := writeAtomically(path, file.Bytes()); err != nil {
		fmt.Fprintf(os.Stderr, "histories: %s could not be kept: %v\n", path, err)
	}
	return l
}

// openHistories decides what is kept, and removes what was kept by other code.
func openHistories() {
	switch historiesMode {
	case "off":
		return
	case "trust":
		historiesCode = "trusted"
		return
	case "":
	default:
		panic(fmt.Sprintf("TERRA_HISTORIES=%q: want off, trust or nothing", historiesMode))
	}
	code, err := codeKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "histories: not kept, the code cannot be read: %v\n", err)
		return
	}
	historiesCode = code
	old, _ := os.ReadDir(historiesDir)
	for _, d := range old {
		if d.IsDir() && d.Name() != code && d.Name() != "trusted" {
			os.RemoveAll(filepath.Join(historiesDir, d.Name()))
		}
	}
}

// codeKey is a hash of everything a history made in this test binary could
// depend on: the module's non-test Go files outside cmd, go.mod, the Go
// version and the build settings.
func codeKey() (string, error) {
	var files []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != "." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "cmd" || d.Name() == "docs" || d.Name() == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") || path == "go.mod" {
			files = append(files, filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	h := sha256.New()
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s %d\n", f, len(b))
		h.Write(b)
	}
	fmt.Fprintln(h, runtime.Version(), runtime.GOARCH)
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if !strings.HasPrefix(s.Key, "vcs") {
				fmt.Fprintln(h, s.Key, s.Value)
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// writeAtomically writes b to path through a file beside it, so that a
// reader never sees half a history.
func writeAtomically(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".partial-*")
	if err != nil {
		return err
	}
	_, err = tmp.Write(b)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}

// A kept history is the world: made the first time and kept, read the second,
// and made again over a file that is not a history, it is the world NewLand
// makes each time.
func TestAKeptHistoryIsTheWorld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ancient.history")
	want := digest(NewLand(1, AncientTerms()))
	if got := digest(keptLand(path, 1, AncientTerms())); got != want {
		t.Fatalf("made and kept: %s, and NewLand made %s", got, want)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("nothing kept: %v", err)
	}
	if got := digest(keptLand(path, 1, AncientTerms())); got != want {
		t.Fatalf("read back: %s, and NewLand made %s", got, want)
	}
	if err := os.WriteFile(path, []byte("not a history"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := digest(keptLand(path, 1, AncientTerms())); got != want {
		t.Fatalf("over a spoilt file: %s, and NewLand made %s", got, want)
	}
	if got := digest(keptLand(path, 1, AncientTerms())); got != want {
		t.Fatalf("read back after the spoilt file: %s, and NewLand made %s", got, want)
	}
}
