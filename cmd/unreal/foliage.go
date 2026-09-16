package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/LukasSelin/terra"
)

// The trees, as instance points for foliage or PCG: one row per forest
// tile, at the tile's centre with a deterministic jitter, a species by the
// tile's biome, a scale by how much wood stands there and how long it has
// grown, and a yaw from a hash of the tile so that a world always gives
// the same trees. At 25 m a tile one instance a tile is a marker, not a
// wood; the metre level places the stand by the cover at a metre.

// foliageName is the file the trees are written as.
const foliageName = "foliage.csv"

// species is the tree a biome grows, by the name cmd/overview gives the
// biome. A forest tile in a biome with no trees of its own - a desert, a
// steppe, the tundra - gets the hardiest thing that would stand there.
var species = map[string]string{
	bIceCap:          "dwarf_birch",
	bTundra:          "dwarf_birch",
	bBoreal:          "spruce",
	bContinental:     "pine",
	bColdSteppe:      "juniper",
	bHotSteppe:       "acacia",
	bColdDesert:      "juniper",
	bHotDesert:       "acacia",
	bMediterranean:   "holm_oak",
	bTemperateForest: "oak",
	bRainforest:      "kapok",
	bMonsoon:         "teak",
	bSavanna:         "baobab",
	bWetland:         "willow",
}

// unknownSpecies is the tree of a tile with no year written down.
const unknownSpecies = "oak"

// jitter is how far from its tile's centre a tree may stand, as a share of
// the tile's span each way: within the tile, clear of its edges.
const jitter = 0.4

// tree is one instance.
type tree struct {
	X, Y, Z float64
	Species string
	Scale   float64
	Yaw     float64
	Tile    int
}

// hash is splitmix64 of x, the mixing clock/moon.go draws the moon with, so that
// a tree's place and turn are the seed's and the tile's and nothing else's.
func hash(x uint64) uint64 {
	x += 0x9E3779B97F4A7C15
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	return x ^ (x >> 31)
}

// unit is a hash as a number in [0, 1).
func unit(x uint64) float64 { return float64(x>>11) / (1 << 53) }

// trees is every forest tile's instance.
func trees(g *terra.Grid, seed uint64, riverFlow float64) []tree {
	names := biomes(g, riverFlow)
	ageRef := 0.0
	for i := range g.Tiles {
		if g.Tiles[i].Terrain == terra.Forest {
			ageRef = max(ageRef, g.Age[i])
		}
	}
	var out []tree
	for i := range g.Tiles {
		if g.Tiles[i].Terrain != terra.Forest {
			continue
		}
		p := g.PosOf(i)
		x, y := metres(p)
		h := hash(seed ^ hash(uint64(i)))
		dx := (unit(h) - 0.5) * 2 * jitter * terra.TileSpan
		dy := (unit(hash(h)) - 0.5) * 2 * jitter * terra.TileSpan
		yaw := unit(hash(h+1)) * 360
		name, ok := species[names[i]]
		if !ok {
			name = unknownSpecies
		}
		grown := 1.0
		if ageRef > 0 {
			grown = clamp(g.Age[i] / ageRef)
		}
		scale := (0.5 + 0.5*clamp(g.Wood[i])) * (0.7 + 0.3*grown)
		out = append(out, tree{X: x + dx, Y: y + dy, Z: g.Height[i], Species: name, Scale: scale, Yaw: yaw, Tile: i})
	}
	return out
}

// writeFoliage writes the trees as CSV, with a header that says what the
// columns are and that the metre level replaces the file.
func writeFoliage(path string, ts []tree) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "# terra foliage: one tree instance per forest tile at %g m, at the tile's centre with a deterministic jitter of up to %g m.\n", terra.TileSpan, jitter*terra.TileSpan)
	fmt.Fprintln(w, "# x, y and z are metres from the world origin, the centre of tile (0, 0); scale is a multiplier on the species' mesh; yaw is degrees; tile is the tile's index.")
	fmt.Fprintln(w, "# The metre level (milestone U2) replaces this file with instances placed by the cover at a metre.")
	fmt.Fprintln(w, "x,y,z,species,scale,yaw,tile")
	for _, t := range ts {
		fmt.Fprintf(w, "%.3f,%.3f,%.3f,%s,%.3f,%.1f,%d\n", t.X, t.Y, t.Z, t.Species, t.Scale, t.Yaw, t.Tile)
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
