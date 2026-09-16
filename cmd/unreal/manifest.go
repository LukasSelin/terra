package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/LukasSelin/terra"
)

// manifestName is the file the manifest is written as.
const manifestName = "manifest.json"

// manifest is what an export wrote and how to read it: the world it is of,
// the scales that turn a pixel back into metres, and every file.
type manifest struct {
	Seed   uint64      `json:"seed"`
	Preset string      `json:"preset"`
	Terms  terra.Terms `json:"terms"`
	// Width and Height are the world in tiles, and Vertices the Landscape
	// in vertices across and down, the seam column counted on a globe.
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Wrap     bool   `json:"wrap"`
	Vertices [2]int `json:"vertices"`
	// TileSize is the vertices a Landscape tile has a side and TileGrid how
	// many tiles there are across and down; neighbours share an edge.
	TileSize int    `json:"tile_size"`
	TileGrid [2]int `json:"tile_grid"`
	// XYScale and ZScale are the Landscape actor's scale, in Unreal's
	// hundredths of a metre: XY is the metres a quad spans times a hundred,
	// Z is what makes the 16-bit range 512 m * ZScale / 100.
	XYScale float64 `json:"xy_scale"`
	ZScale  int     `json:"z_scale"`
	// ZOffset is the height in metres a heightmap value of 32768 stands
	// for: the Landscape actor's Z location, in metres.
	ZOffset float64 `json:"z_offset"`
	// HeightMin and HeightMax are the lowest and highest ground, in metres,
	// and HeightStep what one heightmap value is worth.
	HeightMin  float64 `json:"height_min"`
	HeightMax  float64 `json:"height_max"`
	HeightStep float64 `json:"height_step"`
	// HeightFormula recovers metres from a heightmap value.
	HeightFormula string `json:"height_formula"`
	// SeaLevel is the height the sea stands at, in metres, or null on a
	// world with no sea.
	SeaLevel *float64 `json:"sea_level"`
	Origin   origin   `json:"origin"`
	// Layers are the weightmap layers, in the order they are numbered, and
	// RiverFlow the least flow a river reach carries, in cubic metres a
	// second.
	Layers    []string `json:"layers"`
	RiverFlow float64  `json:"river_flow"`
	// Files is every file written, the manifest itself last.
	Files []file `json:"files"`
}

// origin is where the world's coordinates start, and which way they run.
type origin struct {
	// X, Y and Z are the metres from the Landscape actor's location to the
	// world origin, which is the centre of tile (0, 0): the actor stands at
	// the origin, ZOffset metres up.
	X, Y, Z float64
	// Note says how a tile maps to a vertex.
	Note string `json:"note"`
}

// file is one file written.
type file struct {
	Name string `json:"name"`
	// Kind is heightmap, weightmap, water, foliage or manifest, and Layer
	// the surface layer of a weightmap.
	Kind  string `json:"kind"`
	Layer string `json:"layer,omitempty"`
	// TileX and TileY are which Landscape tile a heightmap or weightmap is,
	// and -1 for a file that is not a tile.
	TileX int   `json:"tile_x"`
	TileY int   `json:"tile_y"`
	Bytes int64 `json:"bytes"`
}

func newManifest(x *exporter, lo, hi float64) *manifest {
	m := &manifest{
		Seed:          x.opt.Seed,
		Preset:        x.opt.Preset,
		Terms:         x.land.Terms,
		Width:         x.g.W,
		Height:        x.g.H,
		Wrap:          x.g.Wrap,
		Vertices:      [2]int{x.tile.VertsW, x.tile.VertsH},
		TileSize:      x.tile.Size,
		TileGrid:      [2]int{x.tile.NX, x.tile.NY},
		XYScale:       xyScale,
		ZScale:        x.enc.Scale,
		ZOffset:       x.enc.Offset,
		HeightMin:     lo,
		HeightMax:     hi,
		HeightStep:    x.enc.Step(),
		HeightFormula: x.enc.Formula(),
		Origin: origin{
			Z:    x.enc.Offset,
			Note: fmt.Sprintf("vertex (c, r) of the padded Landscape is the centre of tile (c, r), at (c * %g m, r * %g m); columns and rows past the world carry its edge on, and column %d of a globe is column 0 again", terra.TileSpan, terra.TileSpan, x.g.W),
		},
	}
	if sea := x.g.SeaLevel(); sea >= 0 {
		m.SeaLevel = &sea
	}
	m.Layers = append(m.Layers, layerNames[:]...)
	m.RiverFlow = x.opt.RiverFlow
	return m
}

// add records a file written, sized off the disk.
func (m *manifest) add(name, kind, layer string, i, j int, path string) {
	var size int64 = -1
	if st, err := os.Stat(path); err == nil {
		size = st.Size()
	}
	m.Files = append(m.Files, file{Name: name, Kind: kind, Layer: layer, TileX: i, TileY: j, Bytes: size})
}

// write writes the manifest as JSON. The manifest lists itself with the
// size it has after this, which is not known yet, so it is listed last with
// its size read back by the caller.
func (m *manifest) write(path string) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// summary is the manifest in a few lines for the terminal.
func (m *manifest) summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%dx%d tiles, %dx%d vertices, %dx%d Landscape tiles of %d\n", m.Width, m.Height, m.Vertices[0], m.Vertices[1], m.TileGrid[0], m.TileGrid[1], m.TileSize)
	fmt.Fprintf(&b, "XY scale %g, Z scale %d, Z offset %.2f m: heights %.1f..%.1f m in steps of %.4f m\n", m.XYScale, m.ZScale, m.ZOffset, m.HeightMin, m.HeightMax, m.HeightStep)
	if m.SeaLevel != nil {
		fmt.Fprintf(&b, "sea at %.2f m\n", *m.SeaLevel)
	}
	var total int64
	kinds := map[string]int{}
	for _, f := range m.Files {
		total += max(f.Bytes, 0)
		kinds[f.Kind]++
	}
	fmt.Fprintf(&b, "%d files, %.1f MB:", len(m.Files), float64(total)/1e6)
	for _, k := range []string{"heightmap", "weightmap", "water", "foliage", "manifest"} {
		if kinds[k] > 0 {
			fmt.Fprintf(&b, " %d %s", kinds[k], k)
		}
	}
	b.WriteString("\n")
	return b.String()
}
