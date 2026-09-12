package terra

import "slices"

// The ground that changes by the day, kept beside the map rather than on it.
//
// A tile is a good deal of ground that never moves - what it is, whose it
// is, what the rock under it is and how high it stands - and a little that
// the weather and the settlement move every day: how worn it is, how far
// what grows on it has come, what stands on it to be taken. The day's pass
// over the map reads and writes only the second kind, and it reads them of
// every awake tile, so they are kept as one slice each, the whole map's
// wear in a row, the whole map's growth in a row, so that the pass streams
// them rather than picking each out of a tile it wants nothing else of.
// The slices are indexed as Tiles is, and are made and copied wherever it
// is; nothing else about the map knows they are not on the tile.
type Layers struct {
	// Traffic is how worn the ground is: it rises with every crossing and
	// fades when nobody comes that way. It is not a cost - walking a beaten
	// path is no quicker - it is a record of where the settlement's errands
	// actually run, which is what somebody deciding to lay a road reads.
	Traffic []float64
	// Age is how much growing weather what stands on this tile has had, in
	// growing ticks. It is what makes a thicket different from a wood and a
	// sown strip different from one in ear; see grow.go.
	Age []float64
	// Fish, Wood and Wild are what stands on the ground to be taken, where
	// the ground is the kind that keeps a count: the fish in the water, the
	// timber in a wood, and the berries and the game under it, which are
	// one count between them. See stock.go for which is read for what.
	Fish, Wood, Wild []float64
	// Fertility is what a field has in it this year, and Rich what the
	// ground could have at best: worked ground wears down toward nothing
	// and rests back up toward Rich. See grow.go.
	Fertility, Rich []float64
	// Sward is the grass standing on open ground, which nobody takes but a
	// grazing creature: full on a map as it is made, drawn down by grazing,
	// and put back by the growing weather like the fish. It is read only
	// where the ground is open, and it is not a crop - open grass carries
	// nothing anybody can take, which is what Green says of it - so it is
	// a stock beside the map like the others and not a process.
	Sward []float64
	// Kinds is what each tile is, as the one word the day's pass gates its
	// arithmetic on: the structure and the terrain together, see kindOf in
	// pass.go. It is a reading of the tile kept beside it, so that the pass
	// need not pick it out of the tile every day: Build and Turn keep it,
	// and Recount takes it afresh. Nought is open grass with nothing on it,
	// which is what a tile made and never touched is.
	Kinds []int64
}

// NewLayers is the layers of a map of n tiles, all at nothing.
func NewLayers(n int) Layers {
	return Layers{
		Traffic:   make([]float64, n),
		Age:       make([]float64, n),
		Fish:      make([]float64, n),
		Wood:      make([]float64, n),
		Wild:      make([]float64, n),
		Fertility: make([]float64, n),
		Rich:      make([]float64, n),
		Sward:     full(n),
		Kinds:     make([]int64, n),
	}
}

// full is a layer of n at one: the grass a map is made with is standing.
func full(n int) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = 1
	}
	return s
}

// Copy is a copy of every layer, for a snapshot. It is not called Clone so
// that a Grid, which carries the layers, keeps its own Clone.
func (l Layers) Copy() Layers {
	return Layers{
		Traffic:   slices.Clone(l.Traffic),
		Age:       slices.Clone(l.Age),
		Fish:      slices.Clone(l.Fish),
		Wood:      slices.Clone(l.Wood),
		Wild:      slices.Clone(l.Wild),
		Fertility: slices.Clone(l.Fertility),
		Rich:      slices.Clone(l.Rich),
		Sward:     slices.Clone(l.Sward),
		Kinds:     slices.Clone(l.Kinds),
	}
}

// Readings is what the layers hold of one tile, gathered up so that two maps
// can be compared tile by tile. It is for that and for nothing the day does.
type Readings struct {
	Traffic, Age, Fish, Wood, Wild, Fertility, Rich, Sward float64
}

// Read is what the layers hold of tile i.
func (l Layers) Read(i int) Readings {
	return Readings{
		Traffic: l.Traffic[i], Age: l.Age[i], Fish: l.Fish[i], Wood: l.Wood[i],
		Wild: l.Wild[i], Fertility: l.Fertility[i], Rich: l.Rich[i], Sward: l.Sward[i],
	}
}
