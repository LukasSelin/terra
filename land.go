package terra

import "math/rand/v2"

// Land is the world itself: the ground, the weather over it, what grows on
// it, and the way across it. It is everything a map is before anybody stands
// on it.
//
// What is here and what is not is one rule: the land may not name anything a
// game invented. It does not know what a house is, what a market is, what
// anybody wants, or that a settlement exists. It knows heights and rock and
// soil and rivers, the temperature at a latitude in a season, how far a
// stand of timber has come, and how to get from one tile to another. A game
// names all of that freely; none of it names the game.
//
// It is separated because the map is worth more than the settlement on it. A
// world made out of its own history - plates that collided, rock that dates
// from the collision, water that has had an age to find its way down - is a
// thing several different games would want, and a settlement of farmers is
// only the first one asked for. See docs/architecture.md.
//
// World embeds it, so every reading a settlement takes of the ground - w.Grid,
// w.Climate, w.Tick - is the same sentence it always was. The separation is
// what may be written, not what has to be spelled out.
type Land struct {
	// Tick is the day. It is here rather than on the game because the
	// ground ages whether or not anybody is watching it: the woods grow,
	// the weather turns, and the chunks nobody is standing on are caught up
	// against this.
	Tick int
	// RNG is the world's chance. Everything drawn from it is drawn on one
	// goroutine in a fixed order, which is what makes a seed a world; see
	// the package comment. A game's own draws come through here too, so
	// that the land and the game share one stream and one history.
	RNG *rand.Rand
	// source is the stream RNG draws from, kept so that where the stream has
	// got to can be written down with a history and taken up again. See
	// historyfile.go.
	source *rand.PCG

	Grid    *Grid
	Climate Climate // the weather over the whole map this tick
	// Weather is the day's weather - the systems moving through and the wind
	// they make - once AdvanceWeather has been asked for it, and nil before.
	// A copy of a land shares it. See synoptic.go.
	Weather *Weather

	// Terms are the terms this land was made on.
	Terms Terms
	// seed is what the world was made from, kept for the streams of chance
	// that are drawn apart from the main one; see island.go.
	seed uint64
	// moon is where the moon stood on the founding day. See tide.go.
	moon Epoch

	// Forest0 is how much forest the world was made with, so that how much
	// of it has been taken can be told.
	Forest0 int

	// Growing is the growing weather the world has had since it was made,
	// in growing days, and swept is where the sweep of sleeping chunks has
	// got to; see active.go.
	Growing []float64 // by chunk, because the weather goes by latitude and height
	swept   int
	rates   []float64

	// Awake says why the ground is awake, for a runner's timing line.
	Awake AwakeCount

	// routers is the working memory deciding routes on, one per goroutine.
	routers []*Router
}

// NewLand makes a world's ground and hands it over, with nobody on it. It is
// the whole of what a game has to do to have a country: a settlement is put
// down on one of these by NewWith, and anything else that wants the same
// continents - a player walking into them, a map nobody plays on at all -
// starts here and builds its own on top.
//
// A globe is a whole number of chunks round: the nine chunks around a place
// hold everything within a chunk of it only if no chunk is narrower than the
// rest.
//
// It panics on terms that fail Check, and it does not ask whether the world
// will fit in memory: terms that come from outside the program - a flag, a
// file, a player - go through MakeLand instead.
func NewLand(seed uint64, t Terms) *Land {
	if err := t.Check(); err != nil {
		panic(err)
	}
	l := unmade(seed, t)
	l.Generate(t)
	l.handOver()
	return l
}

// unmade is a land on its terms and its seed, with its chance at the start of
// the stream and no ground yet.
func unmade(seed uint64, t Terms) *Land {
	src := rand.NewPCG(seed, seed*0x9E3779B97F4A7C15+1)
	l := &Land{
		seed:    seed,
		RNG:     rand.New(src),
		source:  src,
		Climate: NewClimateOn(t),
		Terms:   t,
	}
	l.moon = epochOf(seed)
	return l
}

// handOver is what a land does once its ground is made, before anybody has it.
func (l *Land) handOver() {
	l.Grid.tide = l.Tide()
	l.Growing = make([]float64, len(l.Grid.Chunks))
}

// DefaultWidth and DefaultHeight size the map when none is given. They fit a
// standard terminal beside a stats panel.
const (
	DefaultWidth  = 80
	DefaultHeight = 36
)

// Terms are what a land is made on: how big the ground is, what shape, how
// much of it is sea and how it came to be that shape. Nothing in them
// changes once the land is made, and nothing in them is about a game - what
// a game wants put on the ground once it exists it asks for separately.
type Terms struct {
	Width, Height int
	// Wrap joins the east edge to the west: the map is a globe drawn as a
	// cylinder rather than a valley with edges. See Grid.
	Wrap bool
	// SeaShare is how much of the ground lies under the sea. A valley has
	// none: its water leaves at the edges. A globe has no edges but the
	// poles, and without a sea every river on it runs to a pole and every
	// laden walker is cut off by one.
	//
	// A world made from its history and given Water has its sea from the
	// water instead, and SeaShare only says how much of its first crust is
	// ocean: see water.go.
	SeaShare float64
	// Water is how much water a world made from its history has, as the
	// depth in metres it would stand to spread evenly over the whole map. It
	// fills the basins the plates made, so how much of the world is sea is the
	// ground's to decide. It is the water over the shelves and the low ground:
	// the deep sea floor such a world lays, kilometres down, is filled with
	// water of its own besides - see abyss.go. Nothing is no water, and the
	// sea is SeaShare's. A map
	// that was drawn has no plates to decide anything, and takes SeaShare
	// whatever this says.
	Water float64
	// Epochs is how many ages of the earth to run before the land is handed
	// over: 0 draws it, and anything else makes it out of its own history.
	// See history.go.
	Epochs int
	// Wetness is how much rain the world's air carries, against the real
	// world's: two is a world twice as wet at every latitude, a half one half
	// as wet. Nothing, as a Terms written without it says, is the real world's.
	// See weather.go.
	Wetness float64
	// Woods and Growth say whether where trees stand and how fast green things
	// grow are read off the climate, the way the real world's are, or off the
	// rules a settlement game was tuned on: a fixed share of the land wooded,
	// and a growth that never stops in winter. The rules are what give a
	// settlement open ground to farm and a winter it can live through; the
	// climate is what gives a globe a desert with no forest in it and an
	// Arctic that grows nothing in January. Left unsaid, each is the climate's
	// on a globe and the rules' on a valley. See Rule.
	Woods, Growth Rule
	// Glacial cuts a drawn map's valleys through the last glacial cycle, with
	// the sea going down and coming back up as the ice took the ocean's water
	// and gave it back, rather than through the last two thousand years at
	// today's sea. It leaves drowned valleys at the coast and terraces over
	// the rivers that cut down to the low sea, and a hundred thousand years of
	// the water's work on all the rest of the ground. See sealevel.go.
	Glacial bool
}

// Rule is which of two readings of the land a world is made with: its tuned
// rules or its climate.
type Rule uint8

const (
	// ByShape is the climate on a globe and the tuned rules on a valley.
	ByShape Rule = iota
	// Tuned is the rules a settlement game was tuned on. See woodsShare and
	// WinterGrowth.
	Tuned
	// ByClimate is the climate: woods where the rain outruns what the air
	// could take back, and growth by the warm days and the rain. See
	// Grid.WoodsAt and climateGrowth.
	ByClimate
)

// climate reports whether r, on a map that wraps or not, reads the climate.
func (r Rule) climate(wrap bool) bool {
	return r == ByClimate || (r == ByShape && wrap)
}

// DefaultTerms is the valley: the default size, with edges, drawn rather
// than run.
func DefaultTerms() Terms {
	return Terms{Width: DefaultWidth, Height: DefaultHeight}
}

// GlobeTerms is a cylinder sixteen chunks round and eight down, a third of
// it sea, made out of its own history. See Globe for why a world this size
// is run rather than drawn.
func GlobeTerms() Terms {
	return Terms{Width: 1024, Height: 512, Wrap: true, SeaShare: 0.3, Epochs: 16, Water: DefaultWater}
}

// Routers returns n routers over this land's map, made once and kept between
// ticks so that deciding allocates nothing. Each is for one goroutine.
func (l *Land) Routers(n int) []*Router {
	for len(l.routers) < n {
		l.routers = append(l.routers, l.Grid.Router())
	}
	return l.routers[:n]
}

// AncientTerms is the valley made out of its own history rather than drawn:
// the same ground a DefaultTerms world has, arrived at instead of composed.
// It is here so that the making can be run and looked at without being the
// only kind there is. See history.go.
func AncientTerms() Terms {
	t := DefaultTerms()
	t.Epochs = 16
	return t
}

// NewLandSized makes a land of the given size on otherwise default terms.
func NewLandSized(seed uint64, width, height int) *Land {
	return NewLand(seed, Terms{Width: width, Height: height})
}
