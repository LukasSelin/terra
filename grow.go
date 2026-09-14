package terra

import "math"

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

// How a stand fills. A wood does not put on the same timber every year: it
// comes on slowly while it is saplings, fastest in the middle of its life and
// slower again as it closes up against what the ground will carry. That is
// the Chapman-Richards curve foresters fit to stands (Richards 1959; Pienaar
// and Turnbull 1973),
//
//	V(t) = Vmax·(1 − e^(−kt))^p,
//
// with Vmax a full stock of one, p standShape, and k set so the curve has come
// to standReach of full when the time a Growth names has passed.
//
// A stand is filled on two of these clocks. The stock's own, over the time
// its Rate says a full stock takes to come back - one over the rate, as the
// straight filling it replaces took - which is how fast what was taken grows
// back; and the stand's age, over the time its Full says, which is the most
// a stand that old can carry. What is standing follows its own curve up to
// the one its age allows, and then follows that one.
//
// Both are curves of the same shape, so a stock's place on one is its place
// on the other times how much faster one clock runs than the other, and the
// whole of a stand's filling over any stretch of growing weather has a closed
// form. It is what lets ground that slept for a season be caught up in one
// go to what the season's days would have done one at a time - to the
// rounding, and not to a hair either side of wherever the ceiling caught up.
const (
	standShape = 3.0
	standReach = 0.99
)

// standPace is k·t at the time the curve comes to standReach: the curve's
// rate, in units of that time.
var standPace = -math.Log(1 - math.Cbrt(standReach)) // Cbrt is the root at standShape

// stand is what a stock at have comes to over k of growing weather, filling
// back at rate of a full stock per growing tick, on a stand that is age old at
// the end of it and comes on in full. A full of nought is ground whose stand
// needs no age, and only the stock's clock binds. It never takes away what is
// already standing, so a wood is only ever held back from filling out, never
// thinned by the calendar.
//
// In the curve's root, w = V^(1/p), a stock's own clock is 1 − w falling by
// e^(−k·rate·k) and the age's ceiling is 1 − e^(−k·age/full). Where the stock's
// clock runs faster than the stand's - rate·full of one or more - a stock that
// reaches its ceiling is held to it and rides it up, and is a min and a max.
// Where it runs slower, a stock can only be above its ceiling by having been
// left there, and it waits where it is until the ceiling passes it and then
// goes on at its own pace: wait is how much growing weather that takes.
func stand(have, age, k, full, rate float64) float64 {
	if !(have < 1) {
		return have
	}
	w := 0.0
	if have > 0 {
		w = math.Cbrt(have)
	}
	next := 1 - (1-w)*math.Exp(-standPace*rate*k)
	if full > 0 {
		if rate*full >= 1 {
			ceiling := 1 - math.Exp(-standPace*math.Max(0, age)/full)
			next = math.Max(w, math.Min(next, ceiling))
		} else {
			wait := -math.Log1p(-w)*full/standPace - math.Max(0, age-k)
			next = 1 - (1-w)*math.Exp(-standPace*rate*math.Max(0, k-math.Max(0, wait)))
		}
	}
	if !(next > w) {
		return have
	}
	return math.Max(have, next*next*next)
}

// refuge is the share of a full stock that comes back from outside: the fish
// that swim in from the next water, the seed that blows onto grazed ground, the
// dung and the dust a worn field is given whether or not anybody gives it
// anything. It is what brings a stock back from nothing, which the logistic
// curve alone never does.
const refuge = 0.01

// regrow is how much a logistic filling at rate is left short of its ceiling
// over k of growing weather, as the factor it is cut by: e^(−r·(1+refuge)·k).
// The pass works it out once for a run and a tile works it out for itself,
// the same product of the same numbers either way.
func regrow(rate, k float64) float64 { return math.Exp(-rate * (1 + refuge) * k) }

// logistic is what a stock at have under a ceiling of most comes to where it
// fills as Schaefer's (1954) surplus production has a fishery fill: at a rate
// in proportion to what there is and to the room left, dB/dt = r·B·(1 − B/K),
// with refuge of the ceiling always arriving from outside. That has a closed
// form, x' = M·x / (x + (M − x)·fall), in x = B/K + refuge and M = 1 + refuge,
// so a season of it at once is a season of days of it. At or over the ceiling
// a stock is the ceiling, as it was when the filling was straight.
func logistic(have, most, fall float64) float64 {
	if !(have < most) || most <= 0 {
		return most
	}
	x := math.Max(0, have)/most + refuge
	x = (1 + refuge) * x / (x + (1+refuge-x)*fall)
	return math.Max(have, math.Min(most, (x-refuge)*most))
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
		s[i] = stand(s[i], g.Age[i], k, f.Full, f.Rate)
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

// How fast a water tile's fish come back, and how fast worn fertility on a
// field comes back toward what the land can hold, as the intrinsic rate r
// of a logistic filling per growing day: see logistic. Neither of these is a
// process: a shoal is a stock that replenishes, not a crop that has to come
// on, and worn soil is resting rather than growing.
//
// They were a straight share of full put back each growing day - 0.0012 for
// the fish, 0.0006 for a field and 0.002 for the grass - and each is now the
// rate that brings a stock from nothing to within a hundredth of full in the
// same time the straight filling took: ln(10⁴)/((1+refuge)·days). Ground that
// was only a little worn comes back slower than it did and ground half worn
// faster, which is what a stock that grows out of itself does.
const (
	FishRegrowth = 0.011
	Fallow       = 0.0055
	// SwardRegrowth is how fast a sward comes back on open ground. Grass is
	// the quickest thing the year makes: a lawn grazed to nothing is most of
	// the way back within a season. It is the whole of what bounds a warren
	// where nothing hunts it.
	SwardRegrowth = 0.018
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
		g.Fish[i] = logistic(g.Fish[i], 1, regrow(FishRegrowth, k))
	case Field:
		g.Fertility[i] = logistic(g.Fertility[i], g.Rich[i], regrow(Fallow, k))
	case Grass:
		g.Sward[i] = logistic(g.Sward[i], 1, regrow(SwardRegrowth, k))
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
