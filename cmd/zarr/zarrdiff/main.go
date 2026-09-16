// zarrdiff says where and by how much two Zarr v3 stores differ: which
// arrays each has, and for every array in both, whether its shape, type and
// attributes agree, how many of its elements changed, by how much, and over
// which tiles.
//
//	go run ./zarrdiff base.zarr change.zarr            a summary, most changed first
//	go run ./zarrdiff -json base.zarr change.zarr      the same for a script
//	go run ./zarrdiff -png where base.zarr change.zarr one image a changed map array
//	go run ./zarrdiff -by book/meeting base.zarr change.zarr
//	                                                   each array's changes by the meeting under each tile
//	go run ./zarrdiff -expect tweak.json base.zarr change.zarr
//	                                                   whether the changes are the ones meant
//
// It exits 0 when the stores hold the same, 1 when they differ and 2 when
// they cannot be compared. With -expect it exits 0 when every check holds
// and 3 when one does not. It is run from cmd/zarr; see README.md.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

// Exit statuses.
const (
	same      = 0
	different = 1
	failed    = 2
	unmet     = 3 // a check of -expect does not hold
)

// list is a flag that may be given more than once.
type list []string

func (l *list) String() string     { return strings.Join(*l, " ") }
func (l *list) Set(s string) error { *l = append(*l, s); return nil }

// run is the command: it writes its report to stdout and says how it went.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("zarrdiff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var o config
	asJSON := fs.Bool("json", false, "write the report as JSON")
	fs.StringVar(&o.png, "png", "", "write a PNG of where each changed map array changed into this directory")
	fs.IntVar(&o.side, "png-side", 4096, "longest side of a PNG in pixels; a larger map is shrunk, a pixel lit if any tile under it changed")
	fs.IntVar(&o.codes, "codes", 5, "code changes listed for each coded array")
	fs.IntVar(&o.budget, "budget", 1<<20, "elements read from each store at once, rounded out to whole chunks")
	fs.Var((*list)(&o.by), "by", "break each array's changes down by the category of each tile in this `group/array` of codes; repeatable")
	fs.StringVar(&o.bySide, "by-side", "a", "the store the -by, -mask and where maps are read from, a or b")
	fs.IntVar(&o.top, "top", 10, "categories listed of a -by map of ids, most tiles changed first")
	fs.Var((*list)(&o.only), "only", "compare only this `group/array`, or the arrays under this group; repeatable")
	fs.Var((*list)(&o.mask), "mask", "compare only the tiles whose code in `group/array=code[,code]` is one of these, by name or number; repeatable, and a tile must be in every mask")
	expect := fs.String("expect", "", "check the changes against this JSON `file` of expectations")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: zarrdiff [flags] a.zarr b.zarr")
		fs.PrintDefaults()
	}
	// Flags may come before, between or after the two stores.
	var stores []string
	for rest := args; ; {
		if err := fs.Parse(rest); err != nil {
			return failed
		}
		if fs.NArg() == 0 {
			break
		}
		stores = append(stores, fs.Arg(0))
		rest = fs.Args()[1:]
	}
	if len(stores) != 2 || o.side < 1 || o.codes < 0 || o.budget < 1 || o.top < 1 || (o.bySide != "a" && o.bySide != "b") {
		fs.Usage()
		return failed
	}
	if *expect != "" {
		var err error
		if o.checks, err = readExpectations(*expect); err != nil {
			fmt.Fprintln(stderr, "zarrdiff:", err)
			return failed
		}
		// A check's array is compared whatever -only says.
		for _, c := range o.checks {
			if len(o.only) > 0 && !slices.Contains(o.only, c.Array) {
				o.only = append(o.only, c.Array)
			}
		}
	}

	r, err := compare(ctx, stores[0], stores[1], o)
	if err != nil {
		fmt.Fprintln(stderr, "zarrdiff:", err)
		return failed
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		err = enc.Encode(r)
	} else {
		err = r.write(stdout)
	}
	if err != nil {
		fmt.Fprintln(stderr, "zarrdiff:", err)
		return failed
	}
	if len(o.checks) > 0 {
		for _, c := range r.Checks {
			if !c.Pass {
				return unmet
			}
		}
		return same
	}
	if r.Same {
		return same
	}
	return different
}
