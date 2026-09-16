// unreal makes a world and writes what Unreal Engine's Landscape import
// takes of it: a 16-bit heightmap and an 8-bit weightmap per surface layer,
// cut into Landscape-sized tiles; the water bodies as JSON; the trees as a
// CSV of instances; and a manifest that says how to read them all back into
// metres.
//
//	go run ./cmd/unreal                       the valley, into unreal/
//	go run ./cmd/unreal -preset globe -out globe
//	go run ./cmd/unreal -seed 7 -w 256 -h 128 -epochs 16 -sea 0.3
//
// The flags are cmd/overview's, so the world it writes is the world overview
// draws. At 25 m a tile the Landscape is faceted: this is milestone U1 of
// docs/perf/scaling-plan.md, the whole path into the editor before the metre
// level exists. See README.md beside this file for the import steps.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LukasSelin/terra"
)

func main() {
	var (
		seed   = flag.Uint64("seed", 1, "the seed the world is made from")
		preset = flag.String("preset", "valley", "valley, ancient or globe")
		w      = flag.Int("w", 0, "width in tiles (overrides the preset)")
		h      = flag.Int("h", 0, "height in tiles (overrides the preset)")
		epochs = flag.Int("epochs", -1, "ages of history to run (overrides the preset)")
		sea    = flag.Float64("sea", -1, "share of the ground under the sea, for a drawn map or a made one given no water (overrides the preset)")
		water  = flag.Float64("water", -1, "metres of water a made world is given, spread over the whole map; 0 floods by -sea instead (overrides the preset)")
		wrap   = flag.Bool("wrap", false, "join the east edge to the west (forced on by -preset globe)")
		out    = flag.String("out", "unreal", "directory to write into")
		river  = flag.Float64("river", riverFlow, "cubic metres a second a channel has to carry to be exported as a river")
	)
	flag.Parse()

	var t terra.Terms
	switch *preset {
	case "valley":
		t = terra.DefaultTerms()
	case "ancient":
		t = terra.AncientTerms()
	case "globe":
		t = terra.GlobeTerms()
	default:
		fail(fmt.Errorf("unknown preset %q: want valley, ancient or globe", *preset))
	}
	if *w > 0 {
		t.Width = *w
	}
	if *h > 0 {
		t.Height = *h
	}
	if *epochs >= 0 {
		t.Epochs = *epochs
	}
	if *sea >= 0 {
		t.SeaShare = *sea
	}
	if *water >= 0 {
		t.Water = *water
	}
	if *wrap {
		t.Wrap = true
	}

	fmt.Printf("making a %dx%d world from seed %d (epochs %d, sea %.2f, water %.1f m, wrap %v)...\n", t.Width, t.Height, *seed, t.Epochs, t.SeaShare, t.Water, t.Wrap)
	start := time.Now()
	land, err := terra.MakeLand(*seed, t)
	if err != nil {
		fail(err)
	}
	fmt.Printf("made in %v\n", time.Since(start).Round(time.Millisecond))

	start = time.Now()
	m, err := export(land, options{Seed: *seed, Preset: *preset, Name: *preset, Out: *out, RiverFlow: *river})
	if err != nil {
		fail(err)
	}
	abs, _ := filepath.Abs(*out)
	fmt.Printf("exported in %v to %s\n\n", time.Since(start).Round(time.Millisecond), abs)
	fmt.Print(m.summary())
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "unreal:", err)
	os.Exit(1)
}
