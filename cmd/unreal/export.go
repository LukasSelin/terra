package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/LukasSelin/terra"
)

// riverFlow is how much water, in cubic metres a second, a channel has to
// carry to be exported as a river: a fifth of a litre a second, the line
// cmd/overview draws running water at, so the rivers here are the rivers
// there.
const riverFlow = 2e-4

// options are what an export is asked for beyond the world itself.
type options struct {
	Seed   uint64
	Preset string
	// Name is the stem every tile file is named from: <Name>_x0_y0.png.
	Name string
	// Out is the directory written into.
	Out string
	// RiverFlow is the least flow a river reach carries: see riverFlow.
	RiverFlow float64
	// sizes are the tile sizes to choose from, tileSizes unless a test
	// wants tiles small enough to have neighbours on a small world.
	sizes []int
}

// exporter is one export in progress: the world, the choices made about it
// and the manifest growing as files are written.
type exporter struct {
	land *terra.Land
	g    *terra.Grid
	opt  options
	tile tiling
	enc  zEncoding
	m    *manifest
}

// export writes the world into opt.Out and returns the manifest of what it
// wrote, which is also written there as manifest.json.
func export(land *terra.Land, opt options) (*manifest, error) {
	if opt.sizes == nil {
		opt.sizes = tileSizes
	}
	if opt.Name == "" {
		opt.Name = "world"
	}
	if err := os.MkdirAll(opt.Out, 0o755); err != nil {
		return nil, err
	}
	g := land.Grid
	vw, vh := vertsOf(g)
	lo, hi := heightRange(g)
	x := &exporter{land: land, g: g, opt: opt, tile: tileFor(vw, vh, opt.sizes), enc: encodingFor(lo, hi)}
	x.m = newManifest(x, lo, hi)

	if err := x.heightmaps(); err != nil {
		return nil, err
	}
	if err := x.weightmaps(); err != nil {
		return nil, err
	}
	if err := x.m.write(filepath.Join(opt.Out, manifestName)); err != nil {
		return nil, err
	}
	x.m.add(manifestName, "manifest", "", -1, -1, filepath.Join(opt.Out, manifestName))
	return x.m, nil
}

// heightmaps writes one 16-bit tile per Landscape tile.
func (x *exporter) heightmaps() error {
	for j := 0; j < x.tile.NY; j++ {
		for i := 0; i < x.tile.NX; i++ {
			name := tileName(x.opt.Name, "", i, j)
			if err := x.writePNG(name, heightTile(x.g, x.tile, x.enc, i, j)); err != nil {
				return err
			}
			x.m.add(name, "heightmap", "", i, j, filepath.Join(x.opt.Out, name))
		}
	}
	return nil
}

// weightmaps writes one 8-bit tile per layer per Landscape tile.
func (x *exporter) weightmaps() error {
	w := allWeights(x.g, x.opt.RiverFlow)
	for k := 0; k < layerCount; k++ {
		for j := 0; j < x.tile.NY; j++ {
			for i := 0; i < x.tile.NX; i++ {
				name := tileName(x.opt.Name, layerNames[k], i, j)
				if err := x.writePNG(name, weightTile(x.g, w, x.tile, k, i, j)); err != nil {
					return err
				}
				x.m.add(name, "weightmap", layerNames[k], i, j, filepath.Join(x.opt.Out, name))
			}
		}
	}
	return nil
}

// writePNG writes img into the output directory as name.
func (x *exporter) writePNG(name string, img image.Image) error {
	f, err := os.Create(filepath.Join(x.opt.Out, name))
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", name, err)
	}
	return f.Close()
}
