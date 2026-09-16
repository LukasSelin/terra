// Package tile is one cell of the map: what the ground is, what stands on
// it, whose it is and the rock under it. It is what the land and whatever
// reads the land both mean by a tile, and it knows nothing about a map - the
// height, the rivers and the weather over a tile are kept beside the map by
// the land, in the order the tiles are kept in. See terra.Grid.
//
// The comments here were written in the land and still speak its language:
// a file, a pass or a field of the Grid they name is the root package's.
package tile

import "math"

// Tile is one cell of the world: what the ground is, what stands on it and
// whose it is, and the land itself - its height, its drainage, the rock
// under it and the soil over that. What changes on it by the day - how
// worn it is, how far what grows on it has come, what it has to give and
// what a field has in it - is kept beside the map rather than on the tile;
// see Layers.
type Tile struct {
	Terrain Terrain
	Mark    Mark
	// Leached, Exposed, Lime, Salt and Carbon are what time has made of the
	// soil, beyond how deep it is and what it is made of: how long the
	// surface has been forming soil, in years; how much of the bases the
	// rock gave it the water has since carried off, out of 65535; the
	// carbonate and the salt the dry years have left in it, in hundredths
	// and thousandths of a kilogram a square metre; and its organic carbon,
	// in kilograms a square metre. Each sits in padding the tile already
	// had, which is why they lie where they do. See pedogenesis.go.
	Leached uint16
	Exposed float32
	Owner   Holder

	// The height and the flow, which between them are the land itself - the
	// rivers, the fertility and the going underfoot are all read off them -
	// are kept beside the map as Grid.Height and Grid.Flow. See relief.go.
	// Drain, how far the tile stands above the water it drains into, is beside
	// the map too, as Grid.Drain.

	// Bedrock is the rock under this tile, and Sand and Clay the shares of
	// the soil over it that are one and the other, the rest being silt. The
	// rock is the bed of the pile under the tile that its surface lies in,
	// and changes as the weather wears down into the next one - see
	// strata.go; what is made of it moves with every age of weather, sorted
	// by the water that carries it. Between them they are
	// what the ground is made of, and the fertility, the drainage and how
	// fast a hillside comes down are all read off them. See bedrock.go.
	//
	// Soil is how many metres of that soil there are over the rock: made
	// out of the rock by the weather, taken off by the water, the creep and
	// the slides before any rock is, and laid down again where they stop.
	// It is kept beside the map as Grid.Soil. See soil.go.
	Bedrock Bedrock
	// Fenced is whether this tile lies inside a fence: a strip of a block of
	// worked ground large enough that somebody hedged it. It is not a
	// structure and not a terrain - the ground under it is still field, and
	// the fence itself is the line round the block rather than anything
	// standing on a tile. See fence.go. It lies here, in the byte after
	// Bedrock, so that the soil's Lime can have the two after it.
	Fenced bool
	Lime   uint16

	// Plate is which piece of the crust this tile rides, and Formed the
	// epoch its rock dates from. Both are written by a world made from its
	// own history and are nothing on a world that was drawn; see history.go.
	// They are kept because what a later change wants to ask of a map -
	// where the ore is, where the ground still shakes - is a question about
	// which plate and how old, and neither can be worked out afterwards.
	Plate  uint8
	Formed uint8
	Salt   uint16
	Carbon float32
}

// Buildable reports whether a tile is open ground nobody has claimed. A road
// is not buildable: once a way is laid, it stays a way.
func (t *Tile) Buildable() bool {
	return t.Terrain == Grass && t.Mark == None && t.Owner == 0
}

// Pavable reports whether a road may be laid on this tile. Roads go over open
// ground, through woods, which they clear, over outcrops, which the quarrymen
// go on cutting from underneath, and across water, where the road is a
// bridge. They do not take another building's place or run over land somebody
// has claimed.
func (t *Tile) Pavable() bool {
	return t.Mark == None && t.Owner == 0
}

// Wet reports whether this tile is water rather than ground, whatever has
// been carried over it. It is the question the map-maker asks of water nine
// times over - what will not grow trees, what silt runs off, what nobody
// stands on - and it is not the question of whether a river runs here, which
// is Flow.
func (t *Tile) Wet() bool { return t.Terrain.Wet() }

// Bridged reports whether this tile is a way carried over water.
func (t *Tile) Bridged() bool {
	return t.Mark.Way() && t.Wet()
}

// Deep reports whether crossing this tile means swimming: water with nothing
// built over it. A bridge is not deep, because the walker is on the road and
// the water is underneath. Neither is ice: it is wet in every sense the
// map-maker means - nothing grows on it, no silt settles on it, it stands
// above nothing - and in none of the senses a walker means. A frozen sea is
// something you cross on your feet with a sack on your back, which is why the
// ice is the one place a laden walker may cross open water.
func (t *Tile) Deep() bool {
	return t.Wet() && t.Terrain != Ice && t.Mark == None
}

// Roofed reports whether a tile is something somebody stands inside rather
// than on. A way is not - a path beside a door is what a door is for. See
// MarkDef.Roofs, where a game says which of its marks have a roof.
func (t *Tile) Roofed() bool { return t.Mark.Roofs() }

// Hard is how well the rock under this tile stands up to being worn away. It
// holds up how steep the ground can stand and steepens the fall a channel is
// shaped to, and it is what the water pays to cut the rock: see
// rockErodibility.
func (t *Tile) Hard() float64 { return hardness[t.Bedrock] }

// Alive reports whether this tile carries a standing crop, which is to say
// something that had to grow before it could be taken. It is exactly the
// ground something was named to grow on, read as one bit.
func (t *Tile) Alive() bool { return alive[t.Mark][t.Terrain] }

// Leaching is how much of the bases this tile's soil once held the water has
// carried off, 0 to 1: base saturation turned round.
func (t *Tile) Leaching() float64 { return float64(t.Leached) / math.MaxUint16 }

// Carbonate is the carbonate built up in this tile's soil, in kilograms a
// square metre.
func (t *Tile) Carbonate() float64 { return float64(t.Lime) * limeUnit }

// Salinity is the salt built up in this tile's soil, in kilograms a square
// metre.
func (t *Tile) Salinity() float64 { return float64(t.Salt) * saltUnit }

// What a unit of Lime and of Salt is, in kilograms a square metre. The most
// either can hold is 65535 of them: 655 of carbonate, which is past the
// petrocalcic horizons of the oldest desert soils, and 65 of salt.
const (
	limeUnit = 0.01
	saltUnit = 0.001
)

// SetLeaching, SetCarbonate and SetSalinity write the three back, rounded to
// what the tile keeps and held inside what it can.
func (t *Tile) SetLeaching(v float64) { t.Leached = uint16(math.Round(clamp01(v) * math.MaxUint16)) }
func (t *Tile) SetCarbonate(v float64) {
	t.Lime = uint16(math.Round(math.Max(0, math.Min(math.MaxUint16, v/limeUnit))))
}
func (t *Tile) SetSalinity(v float64) {
	t.Salt = uint16(math.Round(math.Max(0, math.Min(math.MaxUint16, v/saltUnit))))
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }
