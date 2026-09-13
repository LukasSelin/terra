package terra

// What a kind of ground is, in one place.
//
// Before this there was no such place. What a terrain was called existed
// nowhere at all - a Terrain printed as a number - and the rest of it was
// spread over a map in stock.go and a switch in erode.go, with two more
// switches in other packages keyed off the same five values. Adding a kind of
// ground meant finding four of those and being right about all four, and
// nothing would have said so if one was missed: a switch with no case for the
// new terrain falls through to whatever the default happens to be, and a map
// with no row returns the zero value, which for a class is nil.
//
// So the facts that are the ground's own live here, one row each, and the
// things that read them read the row. What is deliberately not here is
// anything belonging to somebody else: how fast a terrain puts back what is
// taken off it is tuning and belongs to the system that runs it, how a
// terrain is drawn belongs to the renderer, and what a terrain is in the
// ontology's terms belongs to the settlement that speaks the ontology - see
// classes.go. Those keep their own tables, over the same terrains, and a
// test in each says the table covers them all.
type terrain struct {
	// name is what this ground is called. It is the word whatever game is
	// being played uses for it rather than a second vocabulary of the map's
	// own: what the code calls Forest, a settlement calls a wood, and there
	// is no gain in the map having a third name for the same thing.
	name string
	// wet says this is water and not ground at all. It is the one thing about
	// a terrain that half the map-maker asks and none of it used to be able
	// to: trees do not grow on it, silt does not settle on it, it stands
	// above nothing so it has no drain, and nobody stands on it either.
	//
	// It is not the same question as whether a river runs here. That is Flow,
	// and it is a number on the tile, because a lake is water that does not
	// flow and a river in spate is the same channel carrying more. The two
	// coincide while there is one kind of water and stop the moment there are
	// two, which is why they are apart before rather than after.
	//
	// It is here and not a trait of the ontology's on purpose. The ontology's
	// traits are what verbs test, and no act asks whether the ground is wet -
	// what an act wants of water is the fish, and Affords says that already.
	// This is the map's own fact about its own ground.
	wet bool
	// hold is how well the ground holds its soil against the weather, from
	// nothing to all of it. Bare rock keeps almost none, a wood most of what
	// falls on it, and a channel and a worked field are counted whole - the
	// one because cutting down is how a valley deepens, the other because a
	// field is soil by definition.
	hold float64
	// tidal says this ground belongs to the tide: the sea covers it one day
	// and leaves it the next, so whether it can be walked is a question for
	// the day and not for the ground. Nothing grows on it. See shore.go.
	tidal bool
}

var terrains = [TerrainCount]terrain{
	Grass:  {name: "open", hold: 0.6},
	Forest: {name: "wood", hold: 0.25},
	Water:  {name: "water", wet: true, hold: 1},
	Field:  {name: "field", hold: 1},
	Rock:   {name: "outcrop", hold: 0.15},
	// Ice is wet: it is the sea, and the map-maker's questions about water
	// all have the sea's answer here. Nothing grows on it, nothing settles
	// on it, and it stands above nothing, so it has no drain. What it does
	// not share with open water is that somebody can walk on it; that is
	// Tile.Deep, which is the walker's question and not the map-maker's.
	Ice: {name: "ice", wet: true, hold: 1},
	// A flat is not wet. Silt settles on it - that is what made it - and it
	// stands above the water it drains into at low tide, so the map-maker's
	// questions have ground's answers. The sea's half is the day's: see
	// Grid.Covered. Mud holds its soil about as well as open grass does.
	Flat: {name: "flat", hold: 0.5, tidal: true},
}

// String is what this ground is called.
func (t Terrain) String() string {
	if int(t) >= len(terrains) {
		return "unknown"
	}
	return terrains[t].name
}

// Wet reports whether this is water rather than ground.
func (t Terrain) Wet() bool {
	if int(t) >= len(terrains) {
		return false
	}
	return terrains[t].wet
}

// Tidal reports whether this is ground the tide covers and leaves.
func (t Terrain) Tidal() bool {
	if int(t) >= len(terrains) {
		return false
	}
	return terrains[t].tidal
}

// Hold is how much of its soil this ground keeps against the weather.
func (t Terrain) Hold() float64 {
	if int(t) >= len(terrains) {
		return 0
	}
	return terrains[t].hold
}

// Cost is what entering bare ground of this kind costs in ticks, before
// anything built on it is taken into account. It is the other half of
// Mark.Cost: what stands on a tile charges instead of the ground, so
// between them they are what a step onto a tile is worth. See MoveCost.
func (t Terrain) Cost() float64 {
	if int(t) >= len(moveCost) {
		return 0
	}
	return moveCost[t]
}

// Terrains is every kind of ground, for the tables that have to cover them
// all and the tests that check they do.
func Terrains() []Terrain {
	out := make([]Terrain, 0, TerrainCount)
	for t := Terrain(0); t < TerrainCount; t++ {
		out = append(out, t)
	}
	return out
}

// KindSet is a set of kinds of ground, one bit each. It is what a search
// over the ground says it is looking for, so that the search can be
// answered from the chunk counts before it is walked.
type KindSet uint32

// Kinds is the set holding just these.
func Kinds(ts ...Terrain) KindSet {
	var s KindSet
	for _, t := range ts {
		s |= 1 << t
	}
	return s
}

// Has reports whether t is in the set.
func (s KindSet) Has(t Terrain) bool { return s&(1<<t) != 0 }
