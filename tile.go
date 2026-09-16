package terra

import "github.com/LukasSelin/terra/tile"

// A tile, by the names the land has always given it.
//
// What one cell of the map is - its ground, what stands on it, whose it is
// and the rock under it - is package tile's, and it knows nothing about a
// map. These are its words as the land speaks them, so that terra.Tile and
// terra.Forest go on meaning what they meant to everything built on the land.

type (
	Tile      = tile.Tile
	Terrain   = tile.Terrain
	KindSet   = tile.KindSet
	Mark      = tile.Mark
	MarkDef   = tile.MarkDef
	Holder    = tile.Holder
	Bedrock   = tile.Bedrock
	Chemistry = tile.Chemistry
)

// The kinds of ground. See tile.Terrain.
const (
	Grass        = tile.Grass
	Forest       = tile.Forest
	Water        = tile.Water
	Field        = tile.Field
	Rock         = tile.Rock
	Ice          = tile.Ice
	Flat         = tile.Flat
	Salt         = tile.Salt
	Pan          = tile.Pan
	TerrainCount = tile.TerrainCount
)

// Bare ground, and how many marks the land carries. See tile.Mark.
const (
	None      = tile.None
	MarkCount = tile.MarkCount
)

// The kinds of rock. See tile.Bedrock.
const (
	Granite      = tile.Granite
	Limestone    = tile.Limestone
	Sandstone    = tile.Sandstone
	Shale        = tile.Shale
	Basalt       = tile.Basalt
	Schist       = tile.Schist
	BedrockCount = tile.BedrockCount
)

// Terrains is every kind of ground. See tile.Terrains.
func Terrains() []Terrain { return tile.Terrains() }

// Kinds is the set holding just these. See tile.Kinds.
func Kinds(ts ...Terrain) KindSet { return tile.Kinds(ts...) }

// SetMark says what a mark does to the ground. See tile.SetMark.
func SetMark(m Mark, d MarkDef) { tile.SetMark(m, d) }

// Bedrocks is every kind of rock. See tile.Bedrocks.
func Bedrocks() []Bedrock { return tile.Bedrocks() }
