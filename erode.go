package terra

import (
	"github.com/LukasSelin/terra/geom"
	"math"
)

// Weathering: the land does not hold still.
//
// An age of weather - a decade of it - strips soil off the ground in
// proportion to how much water crosses it, how steeply it lies, and how
// little is holding it down; carries what it strips downhill; and lays it
// down again where the water slows. The
// heights change, so the drainage is worked out again, so the rivers are where
// the new ground sends them. Nothing is moved by hand.
//
// What makes this worth having is not that hills wear down. It is that how
// fast they wear down is partly the settlement's doing. Woods hold a hillside
// together and a ploughed field does not, so a people that clears its slopes
// to farm them washes those slopes into its own river, silts its own valley,
// and finds the soil it depended on in a different place from where it left
// it. Nobody decides that; it falls out of where they chose to put their
// fields.

// SettleSlope is the slope above which water carries everything it has and
// lays down nothing.
const SettleSlope = 0.12

// Overbank is the share of what a river lays down that it lays down outside
// its own channel, on the low ground either side.
const Overbank = 0.7

// Grain is which of the three a load of soil is, coarsest first. The order
// is the order they come out of the water, which is the whole of what sorting
// is.
type Grain uint8

const (
	Sand Grain = iota
	Silt
	Clay
	// Grains is how many there are, for the loads that carry one of each.
	Grains
)

// parts is what a tile's soil is made of, as the three shares. It is the
// composition a stripping takes away and a deposit arrives with.
func parts(t *Tile) [Grains]float64 {
	return [Grains]float64{Sand: t.Sand, Silt: t.Silt(), Clay: t.Clay}
}

// hold is how much of the soil on a tile moves in an age, by what is growing
// or standing on it and by what the soil itself is made of.
//
// The rock underneath is deliberately not in it. Soil comes off a hillside at
// a rate set by what is holding it down and what it is made of, and how hard
// the rock beneath happens to be does not keep a ploughed slope's earth on
// it. Dividing this by the rock was tried, and it took away the one cost the
// whole model charges for clearing a hillside: over forty ages the ploughed
// slopes on hard rock came out richer than they started, because the slow
// weathering of the ground could no longer keep up with the soil going. What
// the rock decides is what the water cuts - see incise and meander - which is
// where a difference in strength shows as a difference in shape. Woods are what
// hold a hillside together; a ploughed field is bare earth by another name; a
// roof or a road takes the ground it covers out of the weather altogether;
// and loose sand goes where clay stays, whatever is growing on either.
func hold(t *Tile) float64 {
	if t.Mark != None {
		return 0
	}
	return t.Terrain.Hold() * t.Wash()
}

// Erode weathers the map by one age and works the drainage out again. It is
// the one thing that changes the shape of the land after the map is made, and
// everything the shape decides - where the rivers run, what the soil will
// hold, how dear it is to walk - follows from it without being told to.
func (w *Land) Erode() {
	g := w.Grid
	// The whole ground moves at once, so the ground asleep is brought up to
	// date first; what the age does to it is done to it as it now stands.
	w.CatchUpAll()
	g.wear(1)
	// And sideways: a river cuts the outside of its bends while the weather
	// takes the hillsides down. See meander.go.
	g.meander(1)
	g.fill()
	g.drain()
	g.carve(w.RNG)
	g.height()
	g.resoil()
	// The ground has moved, so the tree line has moved with it: what was a
	// dry shoulder may now be damp enough to hold a wood, and what the water
	// has cut into may not.
	g.readWoods()
	// The coast has moved, so the ice on it has: sea that was land is frozen
	// if it is cold enough, and ice that is no longer sea is water again.
	g.freeze()
	g.Recount() // the water has moved, and the woods with it
}

// wear is the moving of the ground itself: what an age of weather takes off
// each tile, what it carries downhill, and where it puts it down again. It is
// the whole of erosion that is about soil rather than about a settlement, and
// it is its own function because the making of a world runs it too - a history
// is ages of weather in between the ages of everything else, and there is no
// settlement there to catch up and no tree line yet to re-read.
//
// by is how many ages of weather this pass is worth. An age is a decade for a
// settlement and Erode passes 1; a history passes more, because an epoch of
// the earth is not a decade and mountains that are never worn down are a map
// of knife edges nobody can walk over.
//
// It leaves the drainage stale on purpose: the caller says when the water is
// worked out again, because doing it here would do it twice in Erode.
func (g *Grid) wear(by float64) {
	n := len(g.Tiles)
	recv, run := g.receivers()
	c := fluvial{
		h:      make([]float64, n),
		recv:   recv,
		stack:  stackOf(recv),
		f:      make([]float64, n),
		settle: make([][Grains]float64, n),
		parts:  make([][Grains]float64, n),
	}
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			t := &g.Tiles[i]
			c.h[i] = t.Height
			c.parts[i] = parts(t)
			if int(recv[i]) == i {
				continue
			}
			// How hard the water cuts: stream power, charged to what holds
			// the ground down. See fluvial.go.
			c.f[i] = by * Erodibility * math.Sqrt(t.Flow) * hold(t) / run[i]
			// What it lets settle: more of it the gentler the ground, less of
			// it the more water there is to keep it up, and nothing on ground
			// somebody has built on.
			if t.Mark != None {
				continue
			}
			slack := clamp01(1 - g.Slope(g.PosOf(i))/SettleSlope)
			held := math.Sqrt(settleFlow / math.Max(settleFlow, t.Flow))
			for gr := range depositOf {
				c.settle[i][gr] = math.Min(0.9, depositOf[gr]*slack*held)
			}
		}
	})
	next := c.solve(settleIters)

	change := make([]float64, n)
	gained := make([][Grains]float64, n)
	g.exported = c.account(next, change, gained, func(i int32, laid [Grains]float64) {
		t := &g.Tiles[i]
		if t.Wet() {
			// A river in flood puts most of its silt over the bank. That is
			// what a flood plain is: not ground the river spared, but ground
			// the river made. Without it the silt stays in the channel, the
			// bed rises, and the good land beside it slowly washes away
			// instead of being fed.
			p := g.PosOf(int(i))
			var bank [8]int
			banks := 0
			for _, off := range Dirs {
				q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
				if !g.In(q) {
					continue
				}
				if b := g.At(q); !b.Wet() && b.Drain < FloodDepth {
					bank[banks] = g.Index(q)
					banks++
				}
			}
			if banks > 0 {
				for gr := range laid {
					over := laid[gr] * Overbank
					for _, j := range bank[:banks] {
						change[j] += over / float64(banks)
						gained[j][gr] += over / float64(banks)
					}
					laid[gr] -= over
				}
			}
		}
		for gr := range laid {
			change[i] += laid[gr]
			gained[i][gr] += laid[gr]
		}
	})
	g.creep(by, change, gained)

	for i := range g.Tiles {
		t := &g.Tiles[i]
		t.Height = math.Max(0, t.Height+change[i])
		// Soil goes with the ground it was in. What washes off a slope is
		// what that slope could have grown; what lands on the flat is what
		// makes a flood plain worth farming.
		if !t.Wet() {
			g.Rich[i] = clamp01(g.Rich[i] + change[i]/SoilDepth)
			g.Fertility[i] = math.Min(g.Fertility[i], g.Rich[i])
			mix(t, gained[i])
		}
	}
}

// Creep is the share of the difference in height between two neighbouring
// tiles that an age of weather moves from the higher to the lower, on ground
// that holds nothing back. It is the slow slumping of a hillside under its own
// weight - frost heave, burrows, rain splash - and it is the other half of what
// shapes a slope: the water cuts, and the ground either side of the cut falls
// in after it.
//
// Without it a channel one tile wide has walls that never come down, so every
// line of water down the flank of a range cut itself a trench of its own and
// the mountains came out combed. With it a gully's walls go as fast as its bed
// and only the water that gathers enough to outrun the slumping keeps a
// valley, which is what sets how far apart a range's streams are.
//
// It falls hardest on the smallest shapes and hardly at all on the large: a
// trench one tile across loses a few hundredths of its depth an age, and a
// range twenty tiles across a hundred times less.
//
// Measured on the high fifth of a half globe over three seeds and sixty ages:
// the deepest hundredth of the ground lay 17.2, 18.5 and 25.2 metres below
// the ground either side of it without creep, and 7.7, 8.8 and 15.4 with it,
// for ten metres off the highest summit. Half as much held the trenches at
// the depth they started; twice as much was not tried, because at this figure
// the great rivers' bends were already easing out as fast as they were cut
// until their banks were left to meander.
const Creep = 0.1

// creep books what an age of creep moves onto change and gained. Each pair of
// neighbours is taken once, and what one gives the other takes, so no ground
// is made or lost. The rock does not slow it, for the reason given at hold;
// what is growing does, because roots are what hold a hillside together.
// Whatever somebody has built on stays where it is, and nothing slumps onto it.
func (g *Grid) creep(by float64, change []float64, gained [][Grains]float64) {
	// A river great enough to wander has banks that are its own business: see
	// meander, which takes the outside of a bend and builds the inside, and
	// whose bends creep would otherwise ease back out as fast as they are cut.
	most := 0.0
	for i := range g.Tiles {
		most = math.Max(most, g.Tiles[i].Flow)
	}
	wander := meanderFlow * most
	// Half the pairs, so that each is taken once: east, and the three below.
	pairs := [...]struct {
		off  geom.Pos
		near float64
	}{
		{geom.Pos{X: 1, Y: 0}, 1},
		{geom.Pos{X: -1, Y: 1}, 0.5},
		{geom.Pos{X: 0, Y: 1}, 1},
		{geom.Pos{X: 1, Y: 1}, 0.5},
	}
	for i := range g.Tiles {
		a := &g.Tiles[i]
		p := g.PosOf(i)
		for _, pr := range pairs {
			q := geom.Pos{X: p.X + pr.off.X, Y: p.Y + pr.off.Y}
			if !g.In(q) {
				continue
			}
			j := g.Index(q)
			b := &g.Tiles[j]
			if a.Mark != None || b.Mark != None {
				continue
			}
			if (a.Wet() && a.Flow >= wander) || (b.Wet() && b.Flow >= wander) {
				continue
			}
			hi, lo := i, j
			if b.Height > a.Height {
				hi, lo = j, i
			}
			top := &g.Tiles[hi]
			// An eighth each, so that a tile standing above all eight of its
			// neighbours gives up no more than Creep of its height over them.
			moved := by * Creep / 8 * pr.near * (top.Height - g.Tiles[lo].Height) * hold(top)
			if moved <= 0 {
				continue
			}
			change[hi] -= moved
			change[lo] += moved
			was := parts(top)
			for k := range was {
				gained[lo][k] += moved * was[k]
			}
		}
	}
}

// SoilDepth is how many metres of ground make the difference between land
// that will grow anything and land that will grow nothing. A settlement can
// strip a hillside of it in a few lifetimes of hard farming.
const SoilDepth = 3.0

// resoil lets ground that the moving water has made better become better:
// a flat newly within reach of the flood comes up toward what such ground
// holds, a little each age rather than overnight.
//
// It only ever raises. What lowers soil is the weather taking it away, above,
// and nothing else should: a field somebody has cut a channel to holds more
// than the bare ground around it would, and that is the whole point of having
// dug it. An earlier version pulled every tile toward what its drainage alone
// would give, which quietly undid irrigation every age and cost the
// settlements that had invested in it dearly.
func (g *Grid) resoil() {
	const toward = 0.08
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if t.Wet() {
			continue
		}
		p := geom.Pos{X: i % g.W, Y: i / g.W}
		// The rock underneath goes on making soil out of itself, so ground
		// the water has stripped comes back toward what its own rock
		// weathers to rather than keeping whatever was last washed onto it.
		// It is the same slow pull as the fertility above, on the same
		// clock, because it is the same weathering doing both.
		sand, clay := g.TextureAt(p)
		t.Sand += toward * (sand - t.Sand)
		t.Clay += toward * (clay - t.Clay)
		if can := g.SoilAt(p); can > g.Rich[i] {
			g.Rich[i] += toward * (can - g.Rich[i])
		}
		g.Fertility[i] = math.Min(g.Fertility[i], g.Rich[i])
	}
}

// carrying is how much soil of every grain a load has in it.
func carrying(load [Grains]float64) float64 {
	return load[Sand] + load[Silt] + load[Clay]
}

// mix works what has just been laid down on a tile into the soil already
// there. What arrives does not replace what was there; it is ploughed and
// burrowed and frozen into the top of it, so the tile ends up somewhere
// between the two, nearer the newcomer the more of it there is.
//
// The soil already there is weighed as SoilDepth metres of it, which is the
// same depth the fertility is reckoned in: a river that lays down a
// centimetre in an age barely moves what the field is made of, and one that
// buries a bank in three metres of silt has made new ground.
func mix(t *Tile, laid [Grains]float64) {
	d := carrying(laid)
	if d <= 0 {
		return
	}
	held := SoilDepth
	t.Sand = (t.Sand*held + laid[Sand]) / (held + d)
	t.Clay = (t.Clay*held + laid[Clay]) / (held + d)
}
