# cmd/unreal

Makes a world with `cmd/overview`'s flags and writes what Unreal Engine's
Landscape import takes of it. This is milestone U1 of
`docs/perf/scaling-plan.md`: the whole path from a seed into the editor
at 25 m a tile, faceted, before the metre level exists.

```
go run ./cmd/unreal                                  the valley, into unreal/
go run ./cmd/unreal -preset globe -out globe         the 1024x512 globe
go run ./cmd/unreal -seed 7 -w 256 -h 128 -epochs 16 -sea 0.3 -wrap
```

Flags: `-preset`, `-seed`, `-w`, `-h`, `-epochs`, `-sea`, `-water`, `-wrap`,
`-out` as `cmd/overview` has them, and `-river`, the least flow in cubic
metres a second a channel carries to be written as a river (a fifth of a
litre a second by default, where overview draws running water). The
package is standard library only, like the rest of terra.

## The files

Everything is in `-out`, named from the preset. Coordinates in every file
are metres from the world origin, which is the centre of tile (0, 0): x
runs with the tile's x, y with the tile's y, and z is up, the same metres
the heightmap decodes to.

**Heightmaps**, `<name>_x<i>_y<j>.png`. 16-bit grayscale, one pixel per
vertex, one vertex per tile at the tile's centre. The world is cut into
Landscape tiles of 1009, 2017, 4033 or 8129 vertices a side, the largest
that is not wider than the world's longer side, or 1009 when none fits.
Neighbouring tiles share their edge column and row, as the tiled import
expects, and the world is padded out to whole tiles by carrying its last
column and row on. On a globe the east edge's neighbour is the west edge,
so the seam column is written twice and the two ends meet.

A pixel is `32768 + round((height_m - z_offset) * 12800 / z_scale)`, and
the manifest's `height_formula` is the way back:
`metres = z_offset + (value - 32768) * z_scale / 12800`. The Z scale is
the smallest whole number whose `512 m * z_scale / 100` holds the world's
range of heights with a tenth of margin, and the Z offset is the middle
of that range. On a watered globe the range includes the deep sea floor
kilometres down, which is why its Z scale is in the thousands and its
step near a decimetre; the valley's step is millimetres.

**Weightmaps**, `<name>_<layer>_x<i>_y<j>.png`. 8-bit grayscale at the
heightmap's tiling, one per layer, and at every vertex the ten layers sum
to 255. The layers, in order: `grass`, `forest`, `rock`, `sand`, `mud`
(tidal flat and bare fine soil), `ice` (sea ice and ground under an ice
cap), `salt` (salt lake and salt flat), `field`, `riverbed` (wet tiles
carrying more than the river threshold), `lakebed` (tiles under a lake's
level). Dry ground is its cover - forest by `Wood`, grass by `Sward`,
field - over rock where the slope passes a third or the soil is thinner
than 30 cm, and what is bare is sand or mud by the soil's `Sand` share.

**Water**, `water.json`. A list of bodies for the Water plugin:

- `{"type": "ocean", "level"}`, when the world has a sea.
- `{"type": "lake", "level", "outline", "closed", "salt", "tiles"}`, one
  per lake in `Grid.Lakes` with water in it. The outline is the outer
  shore as a closed polygon of tile corners, the first corner repeated
  last; islands are not cut out. A closed lake is salt.
- `{"type": "river", "points": [{"x", "y", "z", "width", "depth"}]}`, one
  per reach. A reach starts at a source - a tile over the threshold no
  river tile drains into - and follows the drainage to the sea, a lake,
  the edge of the world or a reach already written, which it joins at its
  last point, so the splines meet. `z` is the water surface and never
  rises along a reach: where the ground rises over a dimple too shallow
  to be a lake, the surface is what the water backs up to. `width` is
  Finnegan's `2 Q^(3/8) S^(-3/16)` and `depth` Manning's with n = 0.035,
  the constants `erode.go` reads a channel with, both at the mean flow
  and on a fall of no less than 1e-4.

**Foliage**, `foliage.csv`. One row per forest tile: `x, y, z, species,
scale, yaw, tile`. The tree stands at the tile's centre with a
deterministic jitter of up to 10 m, at the tile's height. The species is
by the tile's biome, read the way `cmd/overview/classify.go` reads it
(Köppen, with wetland where a river floods): spruce, pine, oak, holm oak,
willow, kapok, teak, baobab, acacia, juniper, dwarf birch. The scale is
by `Wood` and `Age`, the yaw by a hash of the seed and the tile. At 25 m
a tile this is one marker a tile, not a wood; the file header says so,
and the metre level replaces it.

**Manifest**, `manifest.json`. The seed, the terms, the world in tiles
and vertices, the tile size and grid, the XY scale (2500, 25 m a quad),
the Z scale and offset, the height range and step, the formula, the sea
level (or null), the origin and how a tile maps to a vertex, the layers
in order, the river threshold, and every file written with its kind,
its tile and its size.

## Importing into the editor

Unreal 5, with the Landscape and Water plugins enabled.

1. **Landscape mode.** Select mode (Shift+2), Landscape, Manage, New,
   *Import from File*. Pick the heightmap. For a world of one tile pick
   `<name>_x0_y0.png`; for more the file dialog takes any one tile and
   the importer reads the `_x<i>_y<j>` grid off the names, which is the
   tiled import World Partition expects (enable World Partition on the
   level first for a world of more than one tile, and let the import
   make its landscape regions).
2. **Scale.** Set the Location's Z to `z_offset * 100` and the Scale to
   X = Y = `xy_scale` (2500) and Z = `z_scale` from the manifest. With
   that, a vertex's height in the editor is its height in metres, times
   a hundred. Leave the section size and component count as the importer
   picks them for the tile size; 1009 is 63 quads, 1x1 sections, 16x16
   components.
3. **Layers.** In the same import panel add one layer per weightmap
   name, in the manifest's order, and for each choose *Weight-Blended
   Layer* and create its Layer Info Object when asked (or make them
   beforehand as `LI_<layer>` assets and pick them). Point each at its
   `<name>_<layer>_x<i>_y<j>.png`; the importer reads the tile grid off
   the names as it did for the heights. The material the landscape gets
   should read the same layer names.
4. **Import.** The Landscape is the world at 25 m a quad: faceted, and
   right in every catchment-scale fact.
5. **Water.** `water.json` is read by an editor Python script that makes
   a WaterBodyOcean at the ocean's level, a WaterBodyLake per lake with
   its outline as the spline and its level as the height, and a
   WaterBodyRiver per reach with the points as spline points and the
   width and depth per point. That script belongs with the game, not
   here: it depends on the game's water materials and its actor classes,
   and terra stays standard library only.
6. **Foliage.** `foliage.csv` is read by the same script, or by a PCG
   graph with a CSV source, into instanced static meshes per species.

The editor-side script is milestone U3, in the game's repository.

## What the metre level needs

Milestone U2 derives a metre-level chunk from the 25 m map and streams it
into these same tiles. Reading the map through the public API for this
export showed what the root package would have to expose for that:

- **The channel's constants.** `channelWidth`, `widthCoeff`, `manning`
  and `leastFall` are unexported in `erode.go` and are copied here. The
  metre level cuts a parabolic bed from them and should read the one
  definition.
- **The lake floor.** `Grid.LakeAt` gives the lake, but which tiles lie
  under its water is inferred here as `Level > Height`; `lakeOf` and
  `lakeLevel` are unexported. A `Grid.LakeIndex(i) int` or an iterator
  over a lake's tiles would let the metre level flatten the floor and
  the outline without inferring.
- **The tidal flat's ebb.** `ebb` and `tidal` are unexported;
  `Grid.TidalRange`, `HighWater` and `LowWater` are the public reading.
  The metre level's flat profile needs the ebb height per tile.
- **The deep floor.** `laidHeight` and `abyssal` are unexported. The
  heightmap here carries the abyss as it is in `Height`; a metre level
  that wants to stop at the shelf, or shade the abyss differently, needs
  to know which tiles are abyssal.
- **The strata.** Ledges at the bed tops with their dip (step 4 of the
  plan) need the strata under a tile - bed tops and dip - which
  `strata.go` keeps unexported.
- **The meander state.** The centreline bent by the meander state (step
  2) needs the meander's phase per tile, which is not public.
- **The biome.** The species here re-implement `cmd/overview`'s Köppen
  reading because a command cannot be imported. A `terra.Koppen(i)` or a
  shared `classify` package would let overview, the export and the game
  read one biome.
- **Slope at a metre.** `Grid.Slope` is the steepest drop to a
  neighbour; the metre level's microrelief wants the gradient vector,
  which `Aspect` gives only as a step.
- **Chunk derivation.** `DetailChunk(seed, chunk) -> heights, weights`
  itself, written in kernels, streamed chunk by chunk into the tiles
  here: this command then reads chunks instead of tiles and imports at
  an XY scale of 100.
