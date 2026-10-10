package terra

import (
	"math"

	"github.com/LukasSelin/terra/internal/phase"
)

// The sea a history runs against: so much water, poured into the basins its
// crust leaves.
//
// The sea used to be a rule. It stood where 30 km of continental crust floats
// (seaDatum), or higher where that left less than historySea of the ground
// under it, and so a history's land share was whatever its ground made of
// that rule, epoch by epoch, with no water behind it. The earth's sea is not a
// rule. It is a fixed amount of water - the ocean has held about as much as it
// does now since the Archean (Wise 1974) - and where it stands is wherever the
// basins it is poured into run out of room. What sets their room is the floor:
// young floor is hot and stands high, two and a half kilometres under the sea
// at a ridge against five and a half for the oldest (abyss.go's GDH1), so a
// world whose plates are spreading fast has more young floor, shallower
// basins, and the same water standing higher in them. That is Pitman's (1978)
// reading of the Cretaceous seas, which stood a hundred to two hundred and
// fifty metres over today's and drowned a third of the continents: the ridges
// of the Cretaceous were longer and faster, and the mean floor younger, than
// now (Müller and others 2008 have the ridges alone worth 250 m since).
//
// So a history pours its ocean once, in the first epoch, where the old rule
// stood it: that is how much water the planet has (oceanWater), and nothing
// after makes or loses any. Every epoch the level is the one at which the
// room under it, over the ground as the plates and the weather have left it,
// holds that water (seaOver). The land share is what comes of it, and each
// epoch's is written down (seaReading).
//
// The room is read off the ground itself, and the ground has everything on it
// that takes room from the sea. What the rivers bring the sea is laid on the
// margins (shelve), and a wedge of sediment off a continent is a wedge of the
// basin the water no longer has: the sea stands higher for every metre of it
// laid under the sea, which is the rise the reading counts as sediment. Where
// new continent is made out of floor, at an arc or a collision, the basins
// lose that floor's room too.
//
// The water does not all stand where it was poured. The extra depth of a
// higher sea loads the floor under it, and the floor sinks by
// seaDensity/mantleDensity of what the water rises by, which gives the water
// room again: the sea over the continents rises by (ρm - ρw)/ρm of what it
// would have over a floor that held still (waterLoad; Pitman 1978, Kominz
// 1984). The floor's heights are not moved for it - each epoch's isostasy
// floats them on the crust against seaDatum - so the load is read into the
// level: the first epoch's level, plus waterLoad of how far the water would
// stand from it over a floor that did not sink.
//
// And some of the water may be ice. On the earth the ice sheets held a
// hundred and twenty metres of the sea at the last glacial maximum, and
// Antarctica's ice holds sixty metres of it today (Fretwell and others 2013).
// A history has no ice yet: iceHeld is where it will take its water from,
// once the ice sheets are grown through the epochs (G8, #47) and the ice ages
// are worked from the climate (X5, #61). Until then it holds none. The map's
// own glacial sawtooth (sealevel.go) is drawn on its sea and has no ice
// behind it either; X5 is what replaces it.
//
// The level is the bathtub's: every tile under it is under the sea, whether
// or not the sea reaches it, as the old rule's share was. A hollow in the
// middle of a continent deeper than the sea is the sea's (the Caspian is the
// earth's, and lies under it).

// waterLoad is how far the sea over the continents rises for every metre the
// water would rise over a floor that held still: the floor sinks under the
// extra water by ρw/ρm of it.
const waterLoad = (mantleDensity - seaDensity) / mantleDensity

// seaReading is what an epoch's sea was. level is its height over the
// history's nothing, in metres, and rise how far it stood over the first
// epoch's. land is the share of the planet's surface standing over it, and
// shelf the share of the continental crust under it: the continents'
// flooding. floorAge is the mean age of the ocean floor, in millions of
// years. The shares and the means are of the surface a tile stands for on a
// sphere.
//
// And what the rise is made of, in metres of it, read apart from the level
// and not used to make it. ridge is the floor's: how far the sea stands
// higher for the floor's mean depth by its age (floorDepth) being shallower
// than the first epoch's, over the share of the sea that is floor - Pitman's
// ridge volume. sediment is the margins': the sediment under the sea, floated
// on the crust as it is (wetRise of it takes the water's room), over the
// sea's area. ice is what the ice holds out of it. heat is what the
// stretched continent under the sea still stands up by for the heat its
// rifting brought up (subside.go), over the sea's area: what it gives the
// sea as it cools is how far heat falls from the first epoch's. Each is
// loaded as the level is (waterLoad). What is left of the rise is the rest of
// what the basins did: the continents grown or shrunk, the trenches, the
// floor bent under what is laid on it.
type seaReading struct {
	level, rise          float64
	land, shelf          float64
	floorAge             float64
	ridge, sediment, ice float64
	heat                 float64
}

// areaOf is how much of the planet tile i stands for, against a tile on the
// equator: a row's on a globe, and every tile's the same on a map that is not
// one.
func (g *Grid) areaOf(i int) float64 {
	if !g.Wrap {
		return 1
	}
	return g.rowArea(i / g.W)
}

// rowWeights is areaOf for each row of g, worked out once for a pass over
// every tile.
func (g *Grid) rowWeights() []float64 {
	w := make([]float64, g.H)
	for y := range w {
		w[y] = 1
		if g.Wrap {
			w[y] = g.rowArea(y)
		}
	}
	return w
}

// roomUnder is how much water g's ground holds under a sea at level, in metres
// over a tile on the equator.
func (g *Grid) roomUnder(level float64) float64 {
	room, _ := g.roomAt(level, g.rowWeights())
	return room
}

// roomAt is roomUnder, and the area under the sea at level beside it, which
// is how fast the room grows as the level rises.
func (g *Grid) roomAt(level float64, rows []float64) (room, wet float64) {
	for y, a := range rows {
		for _, h := range g.Height[y*g.W : (y+1)*g.W] {
			if d := level - h; d > 0 {
				room += d * a
				wet += a
			}
		}
	}
	return room, wet
}

// seaOver is the level at which the room under it over g's ground comes to
// water, in the same measure as roomUnder. The room is a straight line in the
// level between one tile's height and the next, steeper at every height, so
// Newton's step taken from over the answer never goes past it: from where the
// water would stand over a planet all under it, each step goes down to where
// the room at the present slope would hold the water, and the steps end on
// the straight piece the level lies on. It was the heights sorted and walked
// up, which is exact in one walk, and cost a globe two seconds of its history
// for the sort; this is a pass over the tiles a step, and some ten steps.
func (g *Grid) seaOver(water float64) float64 {
	rows := g.rowWeights()
	total, top := 0.0, math.Inf(-1)
	for y, a := range rows {
		total += a * float64(g.W)
		for _, h := range g.Height[y*g.W : (y+1)*g.W] {
			top = math.Max(top, h)
		}
	}
	level := top + water/total
	for range 200 {
		room, wet := g.roomAt(level, rows)
		if wet == 0 || room-water <= 1e-12*math.Max(1, water) {
			break
		}
		next := level - (room-water)/wet
		if next >= level {
			break
		}
		level = next
	}
	return level
}

// iceHeld is how much of the planet's water the ice holds in epoch epoch, in
// the measure of roomUnder. It is none: the hook the ice sheets (G8) and the
// ice ages (X5) will take the sea's water out through.
func (g *Grid) iceHeld(cr *crust, epoch int) float64 {
	return 0
}

// pourSea sets the sea a history runs against in epoch epoch, and writes down
// what it was. The first step of the first epoch pours the planet's water
// where the old rule stood the sea (firstSea); every step after stands the
// same water, less what the ice holds, in the basins as they are. See the top
// of this file. first is whether this is an epoch's first step.
func (g *Grid) pourSea(cr *crust, epoch int, first bool) {
	defer phase.Start("pourSea")()
	if (epoch == 0 && first) || cr.oceanWater <= 0 {
		cr.firstLevel = g.firstSea()
		cr.oceanWater = g.roomUnder(cr.firstLevel)
		_, _, cr.firstFloor = g.meanFloor(cr, epoch)
	}
	ice := g.iceHeld(cr, epoch)
	still := g.seaOver(math.Max(0, cr.oceanWater-ice))
	g.base = cr.firstLevel + waterLoad*(still-cr.firstLevel)
	cr.seas = append(cr.seas, g.readSea(cr, epoch, ice))
}

// firstSea is where the planet's water stands when it is first poured: where
// the crust floats it (seaDatum), or higher where that leaves less than
// historySea of the ground under it. It was every epoch's sea until the
// water was kept.
//
// It was the lowest historySea of the ground and nothing else before that.
// With the ground standing where its crust floats it at, a share is the wrong
// reading on a world that is mostly ocean floor: its sea came out kilometres
// down the floor, with the young floor along every ridge standing out of it
// as land and the continents five kilometres over it. The sea stands at the
// continents' edges, and has for as long as there have been continents (Wise
// 1974), and that is where seaDatum is. A world of little floor - a valley's,
// a quarter of its plates ocean - still drowns historySea of itself, so that
// something is always a sea bed.
func (g *Grid) firstSea() float64 {
	h := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		h[i] = g.Height[i]
	}
	return math.Max(seaDatum, quantile(h, historySea))
}

// readSea reads the sea g.base stands at in epoch epoch: see seaReading.
func (g *Grid) readSea(cr *crust, epoch int, ice float64) seaReading {
	r := seaReading{level: g.base, rise: g.base - cr.firstLevel}
	var all, dry, cont, drowned, wet, sed, heat float64
	for i := range g.Tiles {
		a := g.areaOf(i)
		all += a
		if g.Height[i] > g.base {
			dry += a
		} else {
			wet += a
			sed += a * float64(cr.sed[i])
			if !cr.ocean[i] {
				heat += a * float64(cr.rift[i].warm)
			}
		}
		if !cr.ocean[i] {
			cont += a
			if g.Height[i] <= g.base {
				drowned += a
			}
		}
	}
	floor, age, depth := g.meanFloor(cr, epoch)
	r.floorAge = age
	if all > 0 {
		r.land = dry / all
	}
	if cont > 0 {
		r.shelf = drowned / cont
	}
	if wet > 0 {
		r.ridge = waterLoad * (cr.firstFloor - depth) * floor / wet
		r.sediment = waterLoad * wetRise * sed / wet
		r.ice = waterLoad * ice / wet
		r.heat = waterLoad * heat / wet
	}
	return r
}

// meanFloor is how much of the planet is ocean floor in epoch epoch, in the
// measure of roomUnder, and the floor's mean age in millions of years and
// mean depth by its age (floorDepth) in metres.
func (g *Grid) meanFloor(cr *crust, epoch int) (floor, age, depth float64) {
	for i := range g.Tiles {
		if !cr.ocean[i] {
			continue
		}
		a := g.areaOf(i)
		t := cr.ageAt(i, epoch) / myr
		floor += a
		age += a * t
		depth += a * floorDepth(t)
	}
	if floor > 0 {
		age /= floor
		depth /= floor
	}
	return floor, age, depth
}
