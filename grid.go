package terra

import (
	"math"
	"slices"

	"github.com/LukasSelin/terra/geom"
)

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
	// tables that have to carry a row for each; see kind.go.
	TerrainCount
)

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
	return markWay[t.Mark] && t.Wet()
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

// Grid is the world map, row-major. With Wrap the east edge is joined to the
// west and the map is a globe drawn as a cylinder; without it the map is a
// valley with edges. See globe.go.
type Grid struct {
	W, H  int
	Wrap  bool
	Tiles []Tile
	// Height is metres above the lowest ground on the map, one entry per
	// tile and indexed as Tiles is. It is beside the map rather than in the
	// tile because it is what every pass that moves the ground reads and
	// writes over every tile, and a run of heights is what a kernel takes
	// (kernel.go); the tile keeps what is read one tile at a time. HeightAt
	// reads it by position, off the map included.
	Height []float64
	// Flow is the water running through each tile in cubic metres a second,
	// indexed as Tiles is and beside the map for the same reason.
	Flow []float64
	// Drain is how far each tile stands above the water it drains into, in
	// metres. It is what makes a valley floor a water meadow and a hillside
	// dry, and it is the ground truth the soil is read from.
	Drain []float64
	// Soil is how many metres of soil there are over each tile's rock. It is
	// single precision because it is a thickness of a few metres read to a
	// tenth of a millimetre. See soil.go.
	Soil []float32
	// Sand and Clay are the shares of each tile's soil that are one and the
	// other, the rest being silt. See bedrock.go.
	Sand, Clay []float64
	// Layers is the ground that changes by the day, one slice per reading
	// and indexed as Tiles is; see layers.go.
	Layers
	// Chunks is the map in pieces, CW across and CH down. See chunk.go.
	CW, CH int
	Chunks []Chunk
	// patches is the map in smaller pieces, PW across and PH down, each
	// counting what kind of ground its tiles are. It is what a search for
	// ground asks before walking, and it is the only tally of what kind of
	// ground the map holds where. See patch.go.
	PW, PH  int
	patches []patch
	// Active is which chunks are awake today, set by World.Wake. Empty
	// until the first day, when every chunk is read as awake.
	Active []bool
	// lenders is, for each tile, how many of its eight neighbours have
	// something standing on them or are somebody's: the neighbours that
	// lend a tile their wear when the case for a road on it is read. Kept
	// by Build and Claim, so that the reading can pass over the tiles that
	// have no wear of their own and nobody to lend them any. See Draw.
	lenders []uint8

	// steepAt, steepLine and woodsLine are the map's measure of its own
	// ground: what counts as steep on it, the slope above which nothing
	// wooded will hold, and how well a tile must suit trees before one will
	// take there. All are read by readWoods; woodsRead says whether they
	// have been. See woods.go.
	steepAt   float64
	steepLine float64
	woodsLine float64
	woodsRead bool
	// climateWoods says the woods are read off the climate rather than shared
	// out, and twiMean is the mean wetness index of the dry land the climate's
	// woods are read against. See Terms.Woods and WoodsAt.
	climateWoods bool
	twiMean      float64
	// holds is whether trees will take on each tile, read at the same time
	// as the lines above and from the same ground. See readHolds.
	holds []bool

	// strata is, for each tile, the pile of beds its rock is: what Bedrock is
	// read off as the ground wears into it. Nil on a map made without one,
	// whose tiles keep the rock they were given. See strata.go.
	strata []column

	// seam and seamQueue are the working memory a history's plate boundaries
	// are spread with, kept here so that an epoch allocates nothing. See
	// history.go.
	seam      []seam
	seamQueue []int32

	// hot is where a world's hotspots are: the places fed from below rather
	// than at a plate's edge. Drawn once when a history starts and fixed for
	// the life of the world. See history.go.
	hot []geom.Pos
	// welds is how many times two plates became one while the history ran.
	// It is a count of what happened and not something anything downstream
	// reads: see TestContinentsWeldIntoOnePlate.
	welds int

	// warm is, for each tile, the year's mean temperature at sea level there,
	// and swing half the distance from its coldest day to its warmest. They
	// are the weather's, not the ground's, but they are kept here because
	// everything that asks them is a question about a tile: they are written
	// once, when the land is made, by the world that knows what climate this
	// map has. They are by tile and not by row because the sea moderates the
	// ground near it, so the lines the frost and the trees keep follow a coast
	// rather than a parallel. Every such line is read off these and the
	// height, which the weather moves: see Frozen, Treeless, Barren, Freezing,
	// Climate.seaMeanAt and Maritime.
	warm, swing []float32

	// sea is the height of the sea, or below zero on a map with none. See
	// flood in relief.go.
	sea float64

	// base is the level the air takes its water from and the rivers cut down
	// to: the sea once there is one, the sea a history is running against
	// while it runs, and below zero on a map with neither. See weather.go.
	base float64

	// deep is how wide, in metres, a tile is read as while a history runs,
	// and nothing otherwise: see span and deepSpan.
	deep float64

	// abyss is, for each tile of the deep sea floor, the height it stood at
	// before its crust's age laid it kilometres lower, and NaN on every other
	// tile; uplift is how fast a history left each tile's rock
	// rising, in metres a year. Both are a watered history's, and nil on any
	// other map. See abyss.go.
	abyss  []float64
	uplift []float64

	// pedons says the soil's age and chemistry have been laid and are kept
	// from here on, which they are from the end of the making of a map. See
	// pedogenesis.go.
	pedons bool

	// tide is the day's sea the map is read against: see tide.go. tidal is,
	// for each tile, how many times the open ocean's tide it has, and ebb, on
	// a flat, how far under mean sea it lies in those tides. Both are laid
	// with the coast, and are nothing on a map with no sea. See shore.go.
	tide  Tide
	tidal []float32
	ebb   []float32

	// air is the map's climate row by row, and rain and runoff are, for each
	// tile, how much falls on it in a year and how much of that the ground
	// sends on after the air has taken its share back, in millimetres. See
	// weather.go.
	air          *Air
	rain, runoff []float64
	// dayRange is how many times its row's evaporation table each tile's is,
	// for the range of its day's temperature: see diurnal.
	dayRange []float32
	// rainWarm is how much of each tile's year of rain falls in its warmer
	// half, which is what tells a monsoon from a Mediterranean winter rain.
	rainWarm []float32
	// winds is the climate of the wind the rain was last read from. It is
	// never changed once made, so copies of the map share it. See wind.go.
	winds *Winds
	// aired is the ground the weather was last read over: see weatherStale.
	// A copy of the map starts without it, and reads its weather afresh.
	aired []float32
	// area is how many tiles drain through each tile, and water is how much
	// the whole map runs off, in cubic metres a second. Both are drain's.
	area  []float64
	water float64
	// exported is what the last age of weather carried off the land into the
	// sea or off the edge of the map, grain by grain, in metres over a tile.
	exported [Grains]float64
	// bankLoad is what the rivers took off the outside of their bends in the
	// last meander and did not lay on the inside, by tile and grain, in metres
	// over a tile: ground in the water, waiting for the next wear to carry it.
	bankLoad [][Grains]float64

	// The standing water, and the way all the water goes. level is the
	// surface of the lake a tile lies under; lakeOf says which lake that is,
	// -1 where it lies under none, and pans which tiles are the dry salt floor
	// of one. down is the tile each tile's water goes to next once it has
	// gathered, -1 where it goes no further, and route is every tile in an
	// order that has each one after the tile its water goes to. See lake.go.
	lakeLevel []float64
	lakeOf    []int32
	pans      []bool
	Lakes     []Lake
	down      []int32
	route     []int32

	// regions is which laden-walkable ground each tile is part of, and
	// regionsStale whether the water has moved since it was worked out.
	// See region.go.
	regions      []int32
	regionStack  []int32
	regionsStale bool
	// waters counts the times the water has moved, so that an answer
	// about whether there is a way somewhere can be dated. See NoWay.
	waters int

	// router is the working memory the grid's own routing runs on. It serves
	// callers routing one after another; anything routing at the same time as
	// something else needs a Router of its own.
	router *Router

	// landmarks are the tables a guided search bounds the rest of the walk
	// from. Taken by RefreshLandmarks; see landmark.go.
	landmarks Landmarks

	// Scratch: the working memory of the passes that make and wear the
	// ground, kept between calls so that a pass called thirty times over a
	// history does not make its slices afresh each time. None of it means
	// anything between two calls; every pass fills or clears what it reads
	// before it reads it, and Clone leaves it nil for the pass to remake.
	// floodScratch backs the queue flow floods the map from, and slideScratch
	// the one cutBack and fillFrom do; fillScratch is fillFrom's done. The
	// rest are the tile-sized slices of pool, flow, waterStep and creep,
	// which were made afresh on every call and are now fitted once: see sized.
	floodScratch []floodNode
	slideScratch []slideAt
	fillScratch  []bool
	poolScratch  poolScratch
	flowScratch  flowScratch
	stepScratch  stepScratch
	creepScratch creepScratch

	// islanded is set on a view of the map an island acts on for a day,
	// which mends no reading of its own - the water's labels are read as
	// they stood when the day's acting began. See island.go.
	islanded bool
}

// ownRouter is the grid's router, made on first use. It is not safe to reach
// for from two goroutines at once, which is the whole reason Router can be
// held by somebody else.
//
// It comes back holding nothing. A Router keeps whose walker it is routing
// for until it is told otherwise - see Holding - and this one is picked up by
// anybody in turn, so a walker's own gates must not be left standing open for
// whoever asks next.
func (g *Grid) ownRouter() *Router {
	if g.router == nil {
		g.router = &Router{g: g}
	}
	g.router.holder = 0
	return g.router
}

// NewGrid returns an all-grass grid.
func NewGrid(w, h int) *Grid {
	g := &Grid{W: w, H: h, Tiles: make([]Tile, w*h), Height: make([]float64, w*h), Flow: make([]float64, w*h), Drain: make([]float64, w*h), Soil: make([]float32, w*h), Sand: make([]float64, w*h), Clay: make([]float64, w*h), Layers: NewLayers(w * h), lenders: make([]uint8, w*h), sea: -1, base: -1}
	g.layChunks()
	g.layPatches()
	g.repatch()
	return g
}

// In reports whether p is on the map. On a globe every column is; only a
// row past a pole is off it.
func (g *Grid) In(p geom.Pos) bool {
	if p.Y < 0 || p.Y >= g.H {
		return false
	}
	return g.Wrap || (p.X >= 0 && p.X < g.W)
}

// At returns the tile at p. The caller must check In first.
func (g *Grid) At(p geom.Pos) *Tile {
	return &g.Tiles[g.Index(p)]
}

// TileView is one tile read whole, for a reader that asks by tile: what the
// tile keeps and what the map keeps beside it, as Height is, behind one
// name. It is what a game reads a tile through - g.Tile(i).Height() - so
// that which fields sit in the Tile and which sit in a slice on the Grid
// is the map-maker's business and moves without the game moving. It is
// read-only: what is beside the map is written on the map, g.Height[i].
// The tile's own fields and methods come through it as they are.
type TileView struct {
	*Tile
	g *Grid
	i int
}

// Tile is the tile at index i, read whole.
func (g *Grid) Tile(i int) TileView { return TileView{&g.Tiles[i], g, i} }

// TileAt is the tile at p, read whole. The caller must check In first.
func (g *Grid) TileAt(p geom.Pos) TileView { return g.Tile(g.Index(p)) }

// Index is which tile this is.
func (v TileView) Index() int { return v.i }

// Height is metres above the lowest ground on the map: Grid.Height at
// this tile.
func (v TileView) Height() float64 { return v.g.Height[v.i] }

// Flow is the water running through this tile in cubic metres a second:
// Grid.Flow at this tile.
func (v TileView) Flow() float64 { return v.g.Flow[v.i] }

// Drain is how far this tile stands above the water it drains into, in
// metres: Grid.Drain at this tile.
func (v TileView) Drain() float64 { return v.g.Drain[v.i] }

// Soil is how many metres of soil there are over this tile's rock:
// Grid.Soil at this tile.
func (v TileView) Soil() float32 { return v.g.Soil[v.i] }

// Sand and Clay are the shares of this tile's soil that are one and the
// other: Grid.Sand and Grid.Clay at this tile. Silt, Loam and Wash are read
// off the two together; see bedrock.go.
func (v TileView) Sand() float64 { return v.g.Sand[v.i] }
func (v TileView) Clay() float64 { return v.g.Clay[v.i] }
func (v TileView) Silt() float64 { return v.g.siltAt(v.i) }
func (v TileView) Loam() float64 { return v.g.loamAt(v.i) }
func (v TileView) Wash() float64 { return v.g.washAt(v.i) }

// Clone returns a deep copy, for snapshots.
func (g *Grid) Clone() *Grid {
	c := &Grid{W: g.W, H: g.H, Wrap: g.Wrap, Tiles: make([]Tile, len(g.Tiles)), Height: slices.Clone(g.Height), Flow: slices.Clone(g.Flow), Drain: slices.Clone(g.Drain), Soil: slices.Clone(g.Soil), Sand: slices.Clone(g.Sand), Clay: slices.Clone(g.Clay), Layers: g.Layers.Copy(), sea: g.sea, base: g.base, air: g.air, winds: g.winds, tide: g.tide,
		lakeLevel: slices.Clone(g.lakeLevel), lakeOf: slices.Clone(g.lakeOf), pans: slices.Clone(g.pans),
		Lakes: slices.Clone(g.Lakes), down: slices.Clone(g.down), route: slices.Clone(g.route)}
	copy(c.Tiles, g.Tiles)
	c.strata = slices.Clone(g.strata)
	c.abyss, c.uplift = g.abyss, g.uplift // laid once, and never written again
	c.warm = append([]float32(nil), g.warm...)
	c.swing = append([]float32(nil), g.swing...)
	c.rainWarm = append([]float32(nil), g.rainWarm...)
	c.climateWoods = g.climateWoods
	c.pedons = g.pedons
	c.tidal = append([]float32(nil), g.tidal...)
	c.ebb = append([]float32(nil), g.ebb...)
	c.rain = append([]float64(nil), g.rain...)
	c.runoff = append([]float64(nil), g.runoff...)
	c.area = append([]float64(nil), g.area...)
	c.water = g.water
	c.lenders = make([]uint8, len(g.Tiles))
	// The scratch fields - floodScratch, slideScratch, fillScratch and the
	// four structs of them - are left nil: they mean nothing between calls,
	// and the pass that needs one remakes it.
	c.layChunks()
	c.layPatches()
	c.Recount()
	return c
}

// sized is s at length n: s itself where it already is, and a fresh slice
// where it is not. It is how a pass's scratch is kept on the Grid between
// calls without the pass making it again each time. What comes back holds
// whatever the last call left in it: the pass writes every entry it reads,
// or clears it first where it relied on make's zeroing.
func sized[T any](s []T, n int) []T {
	if len(s) != n {
		return make([]T, n)
	}
	return s
}

// Count returns how many tiles satisfy ok.
func (g *Grid) Count(ok func(*Tile) bool) int {
	n := 0
	for i := range g.Tiles {
		if ok(&g.Tiles[i]) {
			n++
		}
	}
	return n
}

// Nearest finds the closest tile to from, within maxR steps, that satisfies
// ok. It walks square rings outward in a fixed order, so results are
// deterministic and ties resolve the same way every run.
func (g *Grid) Nearest(from geom.Pos, maxR int, ok func(p geom.Pos, t *Tile) bool) (geom.Pos, bool) {
	return g.NearestOfKind(from, maxR, 0, ok)
}

// NearestOfKind is Nearest told what ground could possibly satisfy it. It
// is for a search whose answer can only ever stand on ground of one of
// these kinds - somewhere with timber standing on it is a wood, and
// nothing else is - and it uses that twice.
//
// Once before it starts: no ground of these kinds within the radius means
// no answer within the radius, so there is nothing to walk. And then on
// every tile it would otherwise ask about: a tile whose patch holds none
// of these kinds cannot be the answer, so it is stepped over without the
// tile being read or the predicate being run. The rings are walked in the
// order they always were and the tiles that could match are asked in the
// order they always were, so the answer is the answer Nearest would have
// given. What changes is only how much ground is read to reach it.
//
// The patch a tile is in is worked out once per run of tiles that share
// one, which on a ring is fifteen tiles in sixteen.
//
// A kinds of zero means nothing is known about what could satisfy the
// search, and every tile is asked, as Nearest does. Passing kinds that
// the answer could stand *near* rather than *on* would be wrong: a bank
// is dry ground beside water, and no patch of it need hold any water.
func (g *Grid) NearestOfKind(from geom.Pos, maxR int, kinds KindSet, ok func(p geom.Pos, t *Tile) bool) (geom.Pos, bool) {
	from = g.Norm(from)
	if kinds != 0 && !g.AnyWithin(from, maxR, kinds) {
		return geom.Pos{}, false
	}
	check := func(p geom.Pos) bool { return ok(p, g.At(p)) }
	if kinds != 0 {
		was, held := -1, false
		check = func(p geom.Pos) bool {
			if i := g.patchAt(p); i != was {
				was, held = i, g.patchHolds(i, kinds)
			}
			if !held {
				return false
			}
			return ok(p, g.At(p))
		}
	}
	if g.In(from) && check(from) {
		return from, true
	}
	if g.Wrap {
		return g.nearestRound(from, maxR, check)
	}
	for r := 1; r <= maxR; r++ {
		if from.X-r < 0 && from.Y-r < 0 && from.X+r >= g.W && from.Y+r >= g.H {
			break
		}
		// The ring is clipped to the map before it is walked rather than
		// tile by tile as it is. A search that reaches to the far side of
		// the map spends most of its rings off the edge of it, and asking
		// after each of those tiles in turn was the greater part of the
		// cost of not finding anything.
		x0, x1 := max(-r, -from.X), min(r, g.W-1-from.X)
		top, bottom := from.Y-r >= 0, from.Y+r < g.H
		for dx := x0; dx <= x1; dx++ {
			if top {
				if p := (geom.Pos{X: from.X + dx, Y: from.Y - r}); check(p) {
					return p, true
				}
			}
			if bottom {
				if p := (geom.Pos{X: from.X + dx, Y: from.Y + r}); check(p) {
					return p, true
				}
			}
		}
		y0, y1 := max(-r+1, -from.Y), min(r-1, g.H-1-from.Y)
		left, right := from.X-r >= 0, from.X+r < g.W
		for dy := y0; dy <= y1; dy++ {
			if left {
				if p := (geom.Pos{X: from.X - r, Y: from.Y + dy}); check(p) {
					return p, true
				}
			}
			if right {
				if p := (geom.Pos{X: from.X + r, Y: from.Y + dy}); check(p) {
					return p, true
				}
			}
		}
	}
	return geom.Pos{}, false
}

// HasNeighbor reports whether any of the eight tiles around p satisfies ok.
func (g *Grid) HasNeighbor(p geom.Pos, ok func(*Tile) bool) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			q := geom.Pos{X: p.X + dx, Y: p.Y + dy}
			if g.In(q) && ok(g.At(q)) {
				return true
			}
		}
	}
	return false
}

// Roofed reports whether a tile is something somebody stands inside rather
// than on. A way is not - a path beside a door is what a door is for. See
// MarkDef.Roofs, where a game says which of its marks have a roof.
func (t *Tile) Roofed() bool { return markRoofs[t.Mark] }

// Frozen reports whether the ground here is permafrost: high enough, or far
// enough toward the pole, or far enough from the sea, that the year's mean
// stays under Permafrost. It is one rule where there were three - the poles
// were bare because they were cold, the peaks were green because nobody had
// told the weather they were high, and the ice edge was a ruled line because
// nobody had told it where the water was.
//
// Frozen ground is not bare ground. The taiga of Siberia stands on permafrost
// a hundred metres deep; what keeps a tree off the ground is the summer, not
// the year, and that is Treeless, and what keeps everything off it is the ice,
// and that is Barren. The line was the growing frost for all three, which put
// a fifth of a globe's land under bare rock.
//
// It is a fact about the ground and the latitude, both of which the weather
// wanders around rather than changes, so it is read off the year written down
// when the land was made and the height the ground now has. Water is not
// frozen ground: see Freezing.
func (g *Grid) Frozen(p geom.Pos) bool {
	i, ok := g.yearIndex(p)
	return ok && !g.Tiles[i].Wet() && g.meanOn(i, g.Height[i]) < Permafrost
}

// Treeless reports whether the summer here is too short or too cool for a
// tree: above the tree line, by Köppen's warmest month or Körner's growing
// season, whichever is the stricter. See treeMean.
func (g *Grid) Treeless(p geom.Pos) bool {
	i, ok := g.yearIndex(p)
	return ok && !g.Tiles[i].Wet() && g.meanOn(i, g.Height[i]) < treeLineMean(float64(g.swing[i]))
}

// Barren reports whether the ground here is under ice: a summer too cold to
// melt the snow its year brings, by Ohmura's equilibrium line. See iceSummer.
// It is the ground that grows nothing, which is what an outcrop is.
func (g *Grid) Barren(p geom.Pos) bool {
	i, ok := g.yearIndex(p)
	if !ok || g.Tiles[i].Wet() {
		return false
	}
	summer := g.meanOn(i, g.Height[i]) + summerPeak*math.Abs(float64(g.swing[i]))
	return summer < iceSummer(g.Rain(i))
}

// Freezing reports whether the water at p never thaws: high enough, or far
// enough toward the pole, or far enough from the open sea, that the year's
// mean at its surface is under the point sea water freezes. An enclosed polar
// sea freezes over while an open one at the same latitude does not, which is
// Maritime doing to the ice what it does to the tree line.
//
// A river is asked the same question as the sea and gets the same answer. A
// channel at eleven degrees below freezing is ice whatever is upstream of it,
// and the fish in it were the last thing left at the pole that had no
// business being there. See SeaFreeze.
func (g *Grid) Freezing(p geom.Pos) bool {
	i, ok := g.yearIndex(p)
	return ok && g.Tiles[i].Wet() && g.meanOn(i, g.Surface(i)) < SeaFreeze
}

// yearIndex is the index of p, and whether the map has a year written down
// for it at all. A grid made by hand has none, and nothing on it freezes.
func (g *Grid) yearIndex(p geom.Pos) (int, bool) {
	if len(g.warm) != len(g.Tiles) || !g.In(p) {
		return 0, false
	}
	return g.Index(p), true
}

// meanOn is the year's mean on tile i at a height of h metres.
func (g *Grid) meanOn(i int, h float64) float64 {
	return float64(g.warm[i]) - Lapse*h
}

// YearAt is the shape of the year on the ground at tile i, as the lines the
// frost and the trees keep read it: its mean, and the mean of its coldest and
// its warmest month, in degrees. It is nothing on a map with no climate
// written down.
func (g *Grid) YearAt(i int) (mean, coldest, warmest float64) {
	if len(g.warm) != len(g.Tiles) || i < 0 || i >= len(g.Tiles) {
		return 0, 0, 0
	}
	mean = g.meanOn(i, g.Height[i])
	d := monthPeak * math.Abs(float64(g.swing[i]))
	return mean, mean - d, mean + d
}

// RainWarm is the share of the year's rain on tile i that falls in its warmer
// half year. A half is rain the year round.
func (g *Grid) RainWarm(i int) float64 {
	if i < 0 || i >= len(g.rainWarm) {
		return 0.5
	}
	return float64(g.rainWarm[i])
}

// contAt is how much of the country round tile i is land, as the air reads
// it, and a middling amount where the air has not been read.
func (g *Grid) contAt(i int) float64 {
	if g.winds == nil {
		return contMiddling
	}
	e := g.winds.airEnv
	fx, fy := e.cellAt(g, i)
	return e.sample(e.cont, fx, fy)
}

// freeze turns the water that never thaws to ice, and gives back to the water
// any ice that has stopped being either. It is run when the land is made and
// again after every age of weather, because an age moves both the coast and
// the ground under it: water that has come out from under the ice has no fish
// in it yet and gets them back the way any water does, and ground the water
// has left is carve's to name.
func (g *Grid) freeze() {
	if len(g.warm) != len(g.Tiles) {
		return
	}
	g.EachRow(func(y int) {
		for x := 0; x < g.W; x++ {
			p := geom.Pos{X: x, Y: y}
			i := y*g.W + x
			t := &g.Tiles[i]
			switch frozen := g.Freezing(p); {
			case frozen && (t.Terrain == Water || t.Terrain == Salt):
				t.Terrain, g.Fish[i] = Ice, 0
			case !frozen && t.Terrain == Ice:
				// A salt lake thaws back into a salt lake.
				t.Terrain, g.Fish[i] = Water, 0
				if g.closedLake(i) {
					t.Terrain = Salt
				}
			}
		}
	})
}

// RoomToBuild reports whether p is open ground with open ground all round
// it: no building on any of the eight tiles that touch it. Roofs raised
// wherever there was a gap grew into one solid block with no way through
// it, which is a settlement nobody can lay a road in. Kept a tile apart,
// every house keeps its own sides clear, and the gaps between neighbours
// line up into the lanes a road is later laid along.
func (g *Grid) RoomToBuild(p geom.Pos) bool {
	return g.In(p) && g.At(p).Buildable() && !g.HasNeighbor(p, (*Tile).Roofed)
}

// Raze takes down what stands on p and gives the ground back: the tile keeps
// its terrain and loses its building and its owner, and a field goes back to
// grass. The market is the one thing that cannot come down, being the root
// of everything else. A settlement that could only ever add to itself would
// be stuck for good with every choice its founders made on ground they had
// only just arrived on, so what has been built has to be able to go.
func (g *Grid) Raze(p geom.Pos) bool {
	if !g.In(p) {
		return false
	}
	t := g.At(p)
	if markFixed[t.Mark] {
		return false
	}
	if t.Mark == None && t.Owner == 0 {
		return false
	}
	if t.Terrain == Field {
		g.Turn(p, Grass)
		g.Age[g.Index(p)], t.Fenced = 0, false // the crop and the hedge go with the claim
	}
	g.Build(p, None)
	g.Claim(p, 0)
	return true
}

// nearestRound is Nearest on a globe. A ring's top and bottom rows run the
// whole way round once the ring is wider than the map, each column once;
// its sides are still a column each while there is a column that far off,
// and the same column when the map is exactly two rings wide. What is
// walked is walked in a fixed order, so runs repeat.
func (g *Grid) nearestRound(from geom.Pos, maxR int, check func(geom.Pos) bool) (geom.Pos, bool) {
	half := g.W / 2
	for r := 1; r <= maxR; r++ {
		top, bottom := from.Y-r >= 0, from.Y+r < g.H
		if !top && !bottom && r > half {
			break
		}
		x0, x1 := -r, r
		if 2*r+1 >= g.W {
			x0, x1 = -half, g.W-1-half
		}
		for dx := x0; dx <= x1; dx++ {
			x := g.WrapX(from.X + dx)
			if top {
				if p := (geom.Pos{X: x, Y: from.Y - r}); check(p) {
					return p, true
				}
			}
			if bottom {
				if p := (geom.Pos{X: x, Y: from.Y + r}); check(p) {
					return p, true
				}
			}
		}
		if r > half {
			continue
		}
		y0, y1 := max(-r+1, -from.Y), min(r-1, g.H-1-from.Y)
		left, right := g.WrapX(from.X-r), g.WrapX(from.X+r)
		for dy := y0; dy <= y1; dy++ {
			if p := (geom.Pos{X: left, Y: from.Y + dy}); check(p) {
				return p, true
			}
			if right != left {
				if p := (geom.Pos{X: right, Y: from.Y + dy}); check(p) {
					return p, true
				}
			}
		}
	}
	return geom.Pos{}, false
}

// FloorDiv divides rounding down, so that ground west of the map's west edge
// numbers -1 and not 0. It is the arithmetic every band over the map is cut
// by - chunks, patches, the cells a game files its people in - and it lives
// here because a map that wraps is the reason any of them need it.
func FloorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}
