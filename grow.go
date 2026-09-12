package terra

// What grows on a tile takes time to come on, and that time is not the same
// for everything growing. This is where that time is kept against an actual
// tile and advanced; system.Land is what advances it.
//
// These are growing ticks rather than ticks: what is measured is how much
// growing weather a stand has had, so a wood raised in the autumn stands
// still until the thaw.
//
// What grows on what, and how long each thing takes, is not said here. The
// land is told once, before anything runs: a game hands it a table, and the
// arithmetic below runs over the table without ever knowing a wood from a
// cornfield. A settlement fills it out of the ontology - a crop takes a
// month, brush a couple of years, timber six - and growth.go is the only
// place those words are said. Another game fills it with whatever grows in
// its own country, and none of this changes.

// Growth is one thing that grows on a kind of ground. Full is how much
// growing weather it takes to come on. Rate is how much of a full stock a
// growing tick puts back, where what grows is a quantity to be drawn down;
// it is zero for anything cut once and wholly, a crop being the case. Stock
// is where the ground keeps the count of it, and nil where the ground keeps
// no count and how far along the stand is is the whole answer.
type Growth struct {
	Full  float64
	Rate  float64
	Stock func(*Grid) []float64
}

// growth is what grows on each kind of ground, ripe how much growing weather
// the slowest of it needs, and alive whether anything grows there at all.
// The last is kept apart from the first because every tile on the map is
// asked it on every tick and the answer is one bit.
var (
	growth [MarkCount][TerrainCount][]Growth
	ripe   [MarkCount][TerrainCount]float64
	alive  [MarkCount][TerrainCount]bool
)

// SetGrowth says what grows on a kind of ground. It is the whole of what the
// land has to be told about growing things, and it is told before anything
// runs.
//
// The order of the list is the caller's and it is read: Green is the mean
// over it, so naming the same growths in another order reads the same ground
// differently.
func SetGrowth(s Mark, t Terrain, gs []Growth) {
	growth[s][t] = gs
	alive[s][t] = len(gs) > 0
	ripe[s][t] = 0
	for _, g := range gs {
		ripe[s][t] = max(ripe[s][t], g.Full)
	}
	// The day's pass over the ground keeps tables of its own, read off this
	// one and laid out the way the pass wants them; see readGrowth.
	readGrowth()
}

// Alive reports whether this tile carries a standing crop, which is to say
// something that had to grow before it could be taken. It is exactly the
// ground something was named to grow on, read as one bit.
func (t *Tile) Alive() bool { return alive[t.Mark][t.Terrain] }

// The age of what stands on a tile is kept in a layer beside the map - see
// Layers - so what asks after it asks the grid, by the tile's index.

// Grown is how far along what grows on tile i is, in [0,1], against the
// time such a thing takes to come on. The weather only raises it: a wood
// that has made its timber holds it, and a stand does not go over and take
// the wood with it. What sets it back is something eating what is coming
// on - a sounder in a strip, a herd browsing a thicket - and what starts it
// again is the ground being cleared and something else sown on it.
func (g *Grid) Grown(i int, full float64) float64 {
	if full <= 0 {
		return 1
	}
	return clamp01(g.Age[i] / full)
}

// Sow starts whatever is to grow on tile i over: the ground is bare, and
// what stands on it from now on is this year's, not last year's. It is
// called wherever the terrain changes hands - a wood seeded or planted, a
// wood felled to a clearing, a strip broken, a strip harvested, a road laid
// over any of them - so that nothing inherits the age of what it replaced.
func (g *Grid) Sow(i int) { g.Age[i] = 0 }

// Standing puts tile i's growth at full, for ground that is meant to have
// been there all along: the woods a map is made with are old woods. Full is
// the slowest thing that grows on such ground, because a tile has one age
// and everything on it is read off that: a wood as old as its timber has
// long since made its brush. Ground where nothing grows is put at no age at
// all, which is what bare ground is.
func (g *Grid) Standing(i int) {
	t := &g.Tiles[i]
	g.Age[i] = ripe[t.Mark][t.Terrain]
}

// grown puts back what growing weather puts back, up to what the stand's
// age accounts for. Age bounds what a stand grows into, and nothing else:
// it never takes away what is already standing, so a wood is only ever
// held back from filling out, never thinned by the calendar.
func grown(have, ceiling, by float64) float64 {
	if have >= ceiling {
		return have
	}
	return min(ceiling, have+by)
}

// Ripen advances what is growing on this tile by k of growing weather: the
// stand gets that much older, and whatever it is coming on toward fills a
// little further, bounded by the age it has had. A wood does both of these
// twice over, on two clocks - the brush under it within a few years, the
// timber over a lifetime - which is why the age is the tile's and the
// filling is the process's.
//
// A process whose yield the ground keeps no count of only ages the tile. A
// field is the case: a crop is cut once and wholly rather than drawn down,
// so what a strip has to give is read off how far along it is and there is
// no stock to put back.
//
// It is the day's pass with k a day's weather, and the catching up of a
// chunk that slept with k a season's; see active.go.
func (g *Grid) Ripen(i int, k float64) {
	t := &g.Tiles[i]
	ps := growth[t.Mark][t.Terrain]
	if len(ps) == 0 {
		return
	}
	// A stand ages by the weather it gets, not by the calendar: what a
	// winter gives it is nothing, and that is the same clock everything
	// else growing keeps.
	g.Age[i] += k
	for _, f := range ps {
		if f.Rate == 0 || f.Stock == nil {
			continue
		}
		s := f.Stock(g)
		s[i] = grown(s[i], g.Grown(i, f.Full), f.Rate*k)
	}
}

// Green is how much of what could be growing here is standing, in [0,1]. It
// is the reading a satellite takes rather than the one a surveyor takes: not
// what the ground could grow, which is Rich and does not change from one year
// to the next, but what is on it this morning.
//
// A stand that keeps a count of itself is read off the count, because that is
// what a taking draws down: a wood gathered to nothing reads as nothing while
// the trees are still called a wood. A stand that keeps no count - a crop is
// cut once and wholly - is read off how far along it is, so a sown strip is
// bare, a strip in ear is full, and the same strip is bare again the day
// after the harvest. Where a tile has more than one thing growing on it, as a
// wood has its brush and its timber, the reading is the mean of them.
//
// Ground with nothing growing on it at all is nothing: bare rock, open water,
// what is under a roof, and open grass, which carries no crop anybody can
// take. That last one is the reading disagreeing with the eye, and it is the
// simulation's own answer rather than a picture of one: what this map shades
// is what there is to be had.
func (g *Grid) Green(i int) float64 {
	t := &g.Tiles[i]
	ps := growth[t.Mark][t.Terrain]
	if len(ps) == 0 {
		return 0
	}
	sum := 0.0
	for _, f := range ps {
		if f.Stock != nil {
			sum += clamp01(f.Stock(g)[i])
			continue
		}
		sum += g.Grown(i, f.Full)
	}
	return sum / float64(len(ps))
}

// How much a water tile's fish come back per growing day, and how much
// worn fertility a field recovers per growing day toward what the land
// can hold. Neither of these is a process: a shoal is a stock that
// replenishes, not a crop that has to come on, and worn soil is resting
// rather than growing.
const (
	FishRegrowth = 0.0012
	Fallow       = 0.0006
	// SwardRegrowth is how much of a full sward a growing day puts back on
	// open ground. Grass is the quickest thing the year makes: a lawn
	// grazed to nothing is most of the way back within a season. It is the
	// whole of what bounds a warren where nothing hunts it.
	SwardRegrowth = 0.002
	// SeedTakes is how much sward open ground must carry for a wood's seed
	// to take in it. A seed takes among shoots, and ground grazed below
	// this has none: a warren at the edge of a wood holds the meadow open,
	// and the wood comes back over it when the warren is gone.
	SeedTakes = 0.5
)

// SeedTakes reports whether a wood's seed would take on tile i: whether
// there are shoots enough for it. Ground nobody grazes always has them,
// since nothing but a grazing creature draws the sward down.
func (g *Grid) SeedTakes(i int) bool { return g.Sward[i] >= SeedTakes }

// Replenish is what k of growing weather puts back on tile i that is not a
// stand coming on: the fish in the water and the rest a worn field gets.
func (g *Grid) Replenish(i int, k float64) {
	switch g.Tiles[i].Terrain {
	case Water:
		g.Fish[i] = min(1, g.Fish[i]+FishRegrowth*k)
	case Field:
		g.Fertility[i] = min(g.Rich[i], g.Fertility[i]+Fallow*k)
	case Grass:
		g.Sward[i] = min(1, g.Sward[i]+SwardRegrowth*k)
	}
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
