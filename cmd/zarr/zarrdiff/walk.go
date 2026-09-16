package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// A store's nodes, by path from its root ("" is the root): "array" or
// "group".
type nodes map[string]string

// walk finds every node of the directory store at dir. zarr.Store cannot
// list its keys, so the directories are walked for zarr.json. Under an
// array there is nothing but its chunks, and the walk does not go there.
func walk(dir string) (nodes, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory store", dir)
	}
	found := nodes{}
	var visit func(path string) error
	visit = func(path string) error {
		at := filepath.Join(dir, filepath.FromSlash(path))
		b, err := os.ReadFile(filepath.Join(at, "zarr.json"))
		switch {
		case err == nil:
			var head struct {
				NodeType string `json:"node_type"`
			}
			if err := json.Unmarshal(b, &head); err != nil {
				return fmt.Errorf("%s: %w", filepath.Join(at, "zarr.json"), err)
			}
			if head.NodeType != "array" && head.NodeType != "group" {
				return fmt.Errorf("%s: node type %q", filepath.Join(at, "zarr.json"), head.NodeType)
			}
			found[path] = head.NodeType
			if head.NodeType == "array" {
				return nil
			}
		case !errors.Is(err, fs.ErrNotExist):
			return err
		}
		entries, err := os.ReadDir(at)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				child := e.Name()
				if path != "" {
					child = path + "/" + child
				}
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(""); err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("%s holds no zarr.json", dir)
	}
	return found, nil
}

// of is the paths of the nodes of a type, sorted.
func (n nodes) of(kind string) []string {
	var paths []string
	for p, k := range n {
		if k == kind {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths
}

// within is the nodes whose paths keep says to keep.
func (n nodes) within(keep func(string) bool) nodes {
	kept := nodes{}
	for p, k := range n {
		if keep(p) {
			kept[p] = k
		}
	}
	return kept
}
