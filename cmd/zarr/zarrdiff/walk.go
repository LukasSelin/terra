package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/LukasSelin/zarr"
)

// A store's nodes, by path from its root ("" is the root): "array" or
// "group".
type nodes map[string]string

// walk finds every node of the store s, named name in what it reports. It
// lists the one level under each group and reads the metadata of the names
// it finds, so it costs a listing and a read for each node rather than a
// walk of every key; under an array there is nothing but its chunks, and
// the walk does not go there.
func walk(ctx context.Context, s zarr.Store, name string) (nodes, error) {
	root, err := zarr.OpenGroup(ctx, s, "")
	if err != nil {
		return nil, fmt.Errorf("%s is not a store: %w", name, err)
	}
	found := nodes{"": "group"}
	var visit func(g *zarr.Group) error
	visit = func(g *zarr.Group) error {
		children, err := g.Children(ctx)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		for _, c := range children {
			path := join(g.Path(), c.Name)
			switch c.Type {
			case "array":
				found[path] = "array"
			case "group":
				found[path] = "group"
				sub, err := g.OpenGroup(ctx, c.Name)
				if err != nil {
					return fmt.Errorf("%s: %w", name, err)
				}
				if err := visit(sub); err != nil {
					return err
				}
			default:
				return fmt.Errorf("%s: %s: node type %q", name, path, c.Type)
			}
		}
		return nil
	}
	if err := visit(root); err != nil {
		return nil, err
	}
	return found, nil
}

// join is a child's path under its group's.
func join(path, name string) string {
	if path == "" {
		return name
	}
	return path + "/" + name
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
