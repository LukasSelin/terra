package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/LukasSelin/terra"
)

// diffStages compares two directories -stages wrote, stage by stage in the
// order the stages run, and writes a line for each to w: the same, or the
// nodes that differ. It reports whether any stage differs.
//
// Two stores are the same where every file but the root's zarr.json is the
// same, byte for byte: a store is written the same every time from the same
// grid (TestTheSameWorldWritesTheSameStore), so a byte is a change. The root's
// zarr.json is left out because it holds the terms, and a world made on other
// terms whose grid has not moved yet is the same at that stage. Both
// directories have to be written with the same -chunk, -shard, -compress
// and -level.
// zarrdiff says by how much a pair differs.
func diffStages(w io.Writer, a, b string) (differ bool, err error) {
	first := ""
	for i, stage := range terra.Stages() {
		pa, pb := stagePath(a, i, stage), stagePath(b, i, stage)
		nodes, err := diffStore(pa, pb)
		if err != nil {
			return differ, err
		}
		if len(nodes) == 0 {
			fmt.Fprintf(w, "%d-%-6s the same\n", i+1, stage)
			continue
		}
		if first == "" {
			first = fmt.Sprintf("first differs at %d-%s; to see how:\n  go run ./zarrdiff %s %s", i+1, stage, pa, pb)
		}
		differ = true
		shown := nodes[:min(len(nodes), 8)]
		more := ""
		if len(nodes) > len(shown) {
			more = fmt.Sprintf(" and %d more", len(nodes)-len(shown))
		}
		fmt.Fprintf(w, "%d-%-6s differs: %s%s\n", i+1, stage, strings.Join(shown, ", "), more)
	}
	if first != "" {
		fmt.Fprintln(w, first)
	}
	return differ, nil
}

// diffStore is the nodes of two stores whose files differ, or that one store
// has and the other not, by path from the store's root and sorted.
func diffStore(a, b string) ([]string, error) {
	fa, err := storeFiles(a)
	if err != nil {
		return nil, err
	}
	fb, err := storeFiles(b)
	if err != nil {
		return nil, err
	}
	changed := map[string]bool{}
	for name := range fa {
		if _, ok := fb[name]; !ok {
			changed[nodeOf(name)] = true
		}
	}
	for name := range fb {
		if _, ok := fa[name]; !ok {
			changed[nodeOf(name)] = true
			continue
		}
		if changed[nodeOf(name)] {
			continue
		}
		same, err := sameFile(filepath.Join(a, name), filepath.Join(b, name))
		if err != nil {
			return nil, err
		}
		if !same {
			changed[nodeOf(name)] = true
		}
	}
	nodes := make([]string, 0, len(changed))
	for n := range changed {
		nodes = append(nodes, n)
	}
	slices.Sort(nodes)
	return nodes, nil
}

// storeFiles is every file of the store at dir but its root zarr.json, by
// slash path from dir.
func storeFiles(dir string) (map[string]bool, error) {
	if _, err := os.Stat(filepath.Join(dir, "zarr.json")); err != nil {
		return nil, fmt.Errorf("%s is not a store -stages wrote: %w", dir, err)
	}
	files := map[string]bool{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if rel = filepath.ToSlash(rel); err == nil && rel != "zarr.json" {
			files[rel] = true
		}
		return err
	})
	return files, err
}

// nodeOf is the group or array a file of a store belongs to: the path up to
// its zarr.json, or up to the c/ its chunks are kept under.
func nodeOf(name string) string {
	if i := strings.Index(name, "/c/"); i >= 0 {
		return name[:i]
	}
	if strings.HasPrefix(name, "c/") {
		return "/"
	}
	return path.Dir(name)
}

func sameFile(a, b string) (bool, error) {
	x, err := os.ReadFile(a)
	if err != nil {
		return false, err
	}
	y, err := os.ReadFile(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(x, y), nil
}
