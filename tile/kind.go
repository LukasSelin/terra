package tile

// Terrain is what a tile is made of.
type Terrain uint8

const (
	Grass Terrain = iota
	Forest
	Water
	Field
	Rock // an outcrop: stone to cut, nothing to grow
	Ice  // sea that never thaws: nothing to take, and walked over, not swum
	Flat // mud the tide covers and leaves: see shore.go
	Salt // a lake with no outlet, where the air takes all the water brings
	Pan  // the dry floor of one: a crust of salt nothing grows on
	// TerrainCount is how many kinds of ground there are. It sizes the
	// tables that have to carry a row for each; see terrains.
	TerrainCount
)

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
	// hold is how much of the creep bare earth would give up this ground gives
	// up, from nothing to all of it: the slow slumping of a hillside, which
	// roots slow and a plough does not. A channel and a worked field are
	// counted whole. See Creep.
	//
	// It was also what the water was charged, and it is not any more. The
	// water is held back by what grows not as a share of its work but as a
	// stress it has to clear before it does any: see shear.
	//
	// The figures are the ones the water was set by, kept for the creep, which
	// was measured against them: Montgomery (2007) has ground under what grows
	// there of itself wearing a hundred times slower than ploughed fields, a wood
	// holding half again what grass does, and bare rock slower again.
	hold float64
	// shear is the critical shear stress, in pascals, what covers this ground
	// holds it together against: the stress running water has to put on it
	// before it takes any of it. Istanbulluoglu and Bras (2005) write the
	// water's work on a vegetated hillside as E = K(τ - τc)^a, with what grows
	// raising τc; that is how it is taken here, with a at one - see
	// criticalFall.
	//
	// The figures are from Fischenich's (2001) permissible shear stresses for
	// channel linings, which are what engineers measure a cover's hold on the
	// ground as: bare loam a pascal or two; short native grass 34 to 45 and
	// long 57 to 81; hardwood plantings 20 to 120; gravel of a few centimetres
	// about 30 and cobbles of fifteen about 100; stiff clay and colloidal silt
	// about 12. Where in those ranges is set by what real ground loses: at the
	// top of them a wood on this map's slopes lost nothing at all, where
	// Montgomery (2007) has native cover wearing a tenth to a hundredth as fast
	// as a ploughed field. Over the slopes of seed 3's valley, all ploughed
	// against all wooded, the water read in a storm (see floodFlow), by the
	// critical stress of the cover:
	//
	//	τc, Pa    15     20     25     30
	//	ratio    12.9   28.8   88.2   247
	//
	// So a wood is at twenty-five, the low end of the plantings. Open grass is
	// at twenty, under the poorest turf Fischenich tables, because open ground
	// here is anything short of a wood and holds less than a wood does. The
	// rubble an outcrop weathers to is at sixty; a field is bare loam; a tidal
	// flat's mud and a salt crust are stiff clay. Water and ice are the channel
	// itself: what holds a bed is its rock, and the rock is rockErodibility.
	shear float64
	// tidal says this ground belongs to the tide: the sea covers it one day
	// and leaves it the next, so whether it can be walked is a question for
	// the day and not for the ground. Nothing grows on it. See shore.go.
	tidal bool
}

var terrains = [TerrainCount]terrain{
	Grass:  {name: "open", hold: 0.06, shear: 20},
	Forest: {name: "wood", hold: 0.03, shear: 25},
	Water:  {name: "water", wet: true, hold: 1},
	Field:  {name: "field", hold: 1, shear: 2},
	Rock:   {name: "outcrop", hold: 0.01, shear: 60},
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
	Flat: {name: "flat", hold: 0.5, shear: 12, tidal: true},
	// Salt is a lake with no way out: water that arrives and never leaves
	// except into the air, and leaves what it carried behind. It is water in
	// every sense the map-maker means and nothing lives in it.
	Salt: {name: "salt lake", wet: true, hold: 1},
	// Pan is the floor of a lake that the air keeps dry, or the ring a salt
	// lake has shrunk back from: flat, crusted and bare. It is ground and not
	// water. A salt crust is cemented rather than loose, and it is counted as
	// holding what open grass holds.
	Pan: {name: "salt flat", hold: 0.06, shear: 12},
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

// Hold is how much of the creep bare earth would give up this ground gives up.
func (t Terrain) Hold() float64 {
	if int(t) >= len(terrains) {
		return 0
	}
	return terrains[t].hold
}

// Shear is the critical shear stress, in pascals, running water has to put on
// this ground before it takes any of it.
func (t Terrain) Shear() float64 {
	if int(t) >= len(terrains) {
		return 0
	}
	return terrains[t].shear
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

// moveCost is the effort of entering a tile of each terrain, measured in
// ticks. Grass is the unit. Ground that fights back costs more of both things
// an agent has to spend: time, because a slow tile is several ticks of walking
// instead of one, and body, because the exertion drains the physiological
// tier in proportion. Terrain is therefore not decoration; it is a standing
// tax on every plan that crosses it, and the map shapes where people settle,
// what they walk to, and which side of the river they give up on.
//
// Water stays where it was, and the reason is worth recording. A settlement
// grows on both banks, because the ground worth farming is the ground near
// the river, so a quarter of its people were spending their lives wading. The
// obvious fix was to make the water dearer. It was tried at 5, 7 and 9 and it
// was the wrong fix: a river nobody can afford to cross is a river nobody
// wears a ford in, and a ford nobody wears is a ford nobody bridges. Dearer
// water cut the wading barely at all and cost up to a quarter of the
// population. What answers a river is a bridge, and the cheapest water is
// what gets one built.
var moveCost = [TerrainCount]float64{
	Grass:  1,
	Field:  1.3,
	Forest: 2.2,
	Water:  3.5,
	Rock:   1.8,
	// Ice is flat and it is treacherous, and the two nearly cancel: a little
	// dearer than open grass and cheaper than anything with a slope on it.
	// It is not water's 3.5 because nobody is swimming - see Tile.Deep.
	Ice: 1.4,
	// A flat the tide has left is mud: dearer than grass or ice, cheaper than
	// a wood. Covered, it is waded like water; see Grid.Covered.
	Flat: 1.6,
	// A salt lake is swum like any other water. A salt flat is level and
	// firm, and costs what open grass does.
	Salt: 3.5,
	Pan:  1,
}

// Recovers reports whether ground of this kind puts something back on its
// own when it is left alone, besides what grows on it by its age: the fish
// in the water, the rest a worn field gets, the grass on open ground. See
// Replenish, which is where the pace is; an outcrop is stone and does not
// grow, and that is meant.
func (k Terrain) Recovers() bool {
	switch k {
	case Water, Field, Grass:
		return true
	}
	return false
}
