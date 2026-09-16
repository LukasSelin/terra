// zarrdiff says where and by how much two Zarr v3 stores differ: which
// arrays each has, and for every array in both, whether its shape, type and
// attributes agree, how many of its elements changed, by how much, and over
// which tiles.
//
//	go run ./zarrdiff base.zarr change.zarr            a summary, most changed first
//	go run ./zarrdiff -json base.zarr change.zarr      the same for a script
//	go run ./zarrdiff -png where base.zarr change.zarr one image a changed map array
//
// It exits 0 when the stores hold the same, 1 when they differ and 2 when
// they cannot be compared. It is run from cmd/zarr; see README.md.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

// Exit statuses.
const (
	same      = 0
	different = 1
	failed    = 2
)

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
	if len(stores) != 2 || o.side < 1 || o.codes < 0 || o.budget < 1 {
		fs.Usage()
		return failed
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
	if r.Same {
		return same
	}
	return different
}
