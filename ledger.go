package terra

// The book a history keeps of itself, after the history is over.
//
// A world made from its own history is already causal: plates meet, the
// crust crumples, the ground rises, the water fills the basins between. Every
// pass computes an effect from causes it reads, and while the history runs
// it keeps a record of what it has done to each tile - see record - which
// settleRock reads back as geology and then drops, leaving the tile its
// rock, its plate and the epoch the rock dates from. What was dropped with
// it was the answer to "why is this here": which meeting raised the ground,
// how much, when, and what buried it last.
//
// The ledger keeps that. It is a few bytes a tile, written by the passes
// that compute the lift and the burial at the moment they compute them, and
// never invented afterwards: an explanation of the ground (see Why) is a
// reading of these fields, so it is as true as the history and no truer.
// See docs/perf/scaling-plan.md, track P.

// raised is what kind of meeting did most to a tile's height in one epoch.
// It is the seam's made, told apart where made does not tell: a rift and an
// island arc both leave melt, and a hotspot is no meeting at all.
type raised uint8

const (
	unraised    raised = iota
	byCollision        // two continents, crumpling: crushed
	byArc              // a floor going under a continent: arc
	byIslands          // two floors closing: melt out of open water
	byRift             // two plates parting: the ground drops and floors with melt
	byHotspot          // melt up through the middle of a plate
)

// buried is what an epoch's last burial of a tile was: what the water or
// the fire laid over it. See keepBook and tectonics.
type buried uint8

const (
	unburied buried = iota
	byFill          // a river's fill, coarse or fine as the water sorted it
	byMud           // mud off a shore, on a sea bed something is washed into
	byLime          // what lived in a quiet warm sea, or the mud of a cold one
	byLava          // a bed of lava over the pile, from a rift or a hotspot
)

// ledger is one tile's line in the book: the meeting that did most to its
// height over the whole history, and its last burial. It is twelve bytes,
// which TestTheBookCostsWhatItSays holds it to.
//
// lift is the metres that meeting raised the tile by in its epoch, negative
// where a rift dropped it, and worn the metres the weather has taken off
// the tile since, kept in tens of metres: each epoch's wear is rounded to
// the ten as it is added, so a history of sixteen epochs is right to within
// eighty metres of wear that runs to tens of thousands.
//
// Both are the history's metres, and the history's metres are a planet's:
// its rates are real - millimetres a year of rock uplift over four million
// years an epoch, see epochYears - so a seam raises tens of kilometres in
// an epoch and the weather and the plate's settling take most of it back
// before the next, while the ground stands a few kilometres high. The
// finished heights are handed to the map by rank alone (see normalise and
// basins), so the map's metres say nothing of these and these nothing of
// the map's; a reading of the book says whose metres it is giving. They are
// not rescaled with the heights, because a rescaling of standing heights
// applied to an increment larger than any height is a number that means
// nothing: tried, it made a 45 kilometre lift into 4.5 on a map 258 metres
// high.
//
// plates are the two the meeting was between, as they were numbered in
// that epoch - a plate's number never changes, though it may since have
// been welded into another; see Grid.plateRoot - with noPlate for the
// second where a hotspot raised the tile from under one plate alone.
type ledger struct {
	lift     float32
	worn     uint16
	plates   [2]uint8
	kinds    uint8 // raised in the low four bits, buried in the high four
	epoch    uint8 // of the meeting
	buriedIn uint8 // the epoch of the last burial
}

func (l *ledger) raised() raised { return raised(l.kinds & 0xf) }
func (l *ledger) buried() buried { return buried(l.kinds >> 4) }

// meet writes down a meeting that did by metres to the tile in epoch e,
// where that is more than any meeting before it did. What is compared is
// the size of what was done, so that a rift that dropped a tile a kilometre
// is the answer over a collision that raised it a metre; the sign is kept.
// A new answer starts the wear since again.
func (l *ledger) meet(a, b uint8, kind raised, e int, by float64) {
	if abs32(float32(by)) <= abs32(l.lift) {
		return
	}
	if a > b {
		a, b = b, a
	}
	l.lift, l.worn = float32(by), 0
	l.plates = [2]uint8{a, b}
	l.kinds = l.kinds&0xf0 | uint8(kind)
	l.epoch = uint8(e)
}

// bury writes down a burial in epoch e.
func (l *ledger) bury(kind buried, e int) {
	l.kinds = l.kinds&0x0f | uint8(kind)<<4
	l.buriedIn = uint8(e)
}

// wornUnit is what one of worn is, in metres.
const wornUnit = 10.0

// wear adds metres the weather took off the tile.
func (l *ledger) wear(m float64) {
	if m <= 0 {
		return
	}
	w := float64(l.worn) + m/wornUnit + 0.5
	if w > 65535 {
		w = 65535
	}
	l.worn = uint16(w)
}

// wornMetres is the metres the weather has taken off since the meeting.
func (l *ledger) wornMetres() float64 { return float64(l.worn) * wornUnit }

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// openBook starts the book for a history of epochs ages: one line a tile.
func (g *Grid) openBook(epochs int) {
	g.ledger = make([]ledger, len(g.Tiles))
	g.epochs = uint8(min(epochs, 255))
}

// keepPlates keeps which plate each number has been welded into, so that a
// number the book wrote down in an early epoch can be followed to the plate
// that stands at the end. See Plate.into.
func (g *Grid) keepPlates(plates []Plate) {
	g.plateRoot = make([]uint8, len(plates))
	for k := range plates {
		g.plateRoot[k] = rootOf(plates, uint8(k))
	}
}

// rootPlate is the plate that number k has become part of, or k itself on a
// world with no history.
func (g *Grid) rootPlate(k uint8) uint8 {
	if int(k) < len(g.plateRoot) {
		return g.plateRoot[k]
	}
	return k
}
