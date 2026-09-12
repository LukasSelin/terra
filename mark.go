package terra

// What a game has put on a tile.
//
// The land carries one number per tile saying that something stands there,
// and does not know what. A settlement builds houses, markets, granaries,
// taverns and roads; a game with one player in it would put doors, chests
// and a camp on the same ground, and the land would carry those exactly as
// well, because all it ever does with the number is look up what it changes
// about the ground: what the tile costs to enter, how hard it is on the
// body, whether it is a way somebody laid to be walked on, and whether it
// makes the place a place.
//
// That list is the whole of it, and it is worth seeing how short it is. A
// road is not fast because the land knows what a road is. It is fast
// because a game said Cost 0.5 when it registered the mark, and the router
// has read a number ever since.

// Mark is what has been built on a tile. Zero is bare ground.
type Mark uint8

// None is bare ground: the land's own, and the mark of a tile nobody has
// touched.
const None Mark = 0

// MarkCount is how many marks the land will carry, None among them. It
// sizes the tables that keep a row for each, and it is the one number here
// a game cannot choose: raise it if a game wants more.
const MarkCount = 16

// MarkDef is what a mark does to the ground under it.
type MarkDef struct {
	// Cost is what entering the tile costs in ticks, in place of what the
	// terrain would have cost. It is not added to the terrain: what is
	// built over ground stands instead of it, so a road over a marsh is a
	// road.
	Cost float64
	// Drain is how hard on the body a tick of walking here is, as a share
	// of the usual. Zero means the same as bare ground, which is what
	// almost everything is: only something laid to be walked on is easier
	// underfoot than the ground it covers.
	Drain float64
	// Way says this mark is something laid to be walked on rather than
	// something standing in the way. The land asks it of water it is
	// carried over - a way over water is walked, not swum - and of wear,
	// which a way carries itself instead of lending to the ground beside
	// it.
	Way bool
	// Settles says ground carrying this mark is a place rather than a
	// building: it is awake every day and it wakes the ground around it, so
	// that what is read across a chunk's edge is read off ground the day
	// has passed over. One mark of a game's usually has it - whatever it
	// calls the middle of a town.
	Settles bool
	// Roofs says this mark is something a body stands inside rather than
	// something it stands on. The land asks it only to answer the question
	// for whoever wants it - it does not care about the weather itself -
	// but the question is about a tile, and the answer is a fact about the
	// mark rather than about the tile, so it is kept with the rest.
	Roofs bool
	// Fixed says this mark cannot be taken down. A game usually has at most
	// one: the root of the place, which everything else was raised around
	// and which nothing is left standing without. Raze refuses it.
	Fixed bool
}

// What each mark does, by mark. Filled by SetMark and read by the day.
var (
	markCost    [MarkCount]float64
	markWay     [MarkCount]bool
	markSettles [MarkCount]bool
	markRoofs   [MarkCount]bool
	markFixed   [MarkCount]bool
)

// Bare ground drains what walking drains, and so does everything a game has
// not said otherwise about - which is why this starts at one rather than at
// nothing.
//
// It is a variable and not an init, and the difference matters. Package
// variables are made before any init runs and inits run in the order the
// files are named, so a table a game fills from its own init must not be
// given its defaults by an init of the land's: whichever file sorted later
// would win, which is no way to decide what a road costs. Everything here
// that has a default has it this way for that reason.
var markDrain = func() (d [MarkCount]float64) {
	for i := range d {
		d[i] = 1
	}
	return
}()

// SetMark says what a mark does to the ground. It is called once for each
// mark a game uses, before anything runs, and it is the whole of what the
// land is told about what gets built on it.
func SetMark(m Mark, d MarkDef) {
	markCost[m] = d.Cost
	if d.Drain != 0 {
		markDrain[m] = d.Drain
	}
	markWay[m] = d.Way
	markSettles[m] = d.Settles
	markRoofs[m] = d.Roofs
	markFixed[m] = d.Fixed
}

// Way reports whether this mark is something laid to be walked on.
func (m Mark) Way() bool { return markWay[m] }

// Cost is what entering a tile carrying this mark costs, in place of what
// the terrain under it would have cost.
func (m Mark) Cost() float64 { return markCost[m] }

// Drain is how hard on the body a tick of walking on this mark is, as a
// share of what bare ground costs.
func (m Mark) Drain() float64 { return markDrain[m] }
