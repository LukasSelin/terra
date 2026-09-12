package terra

import (
	"github.com/LukasSelin/terra/geom"
	"math"
	"sort"
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

// Wash is how much soil an age of weather - a decade of it; see
// system.ErodeEvery - takes off a tile, given the water crossing it and the
// steepness of it. What the water can lift goes as the root of how much of it
// there is rather than in proportion: taken in proportion, the valley floor
// carries so much of the map's water that it scoured itself out instead of
// silting up, which is the opposite of what a flood plain is. The root is the
// usual reading, and with it the channel still cuts down while the ground
// beside it fills.
//
// The size of it is what makes the ground move at the speed ground moves: a
// ploughed slope loses a few centimetres of soil a decade and a wooded one a
// few millimetres, so a hillside farmed hard is worn out in a century or two
// and one left standing keeps what it has for longer than anybody watching it
// will be alive.
const Wash = 12

// Settle is the share of what the water is carrying that it puts down on
// gentle ground each tile it crosses. Steep ground keeps its load moving.
//
// It is one figure no longer: water sorts what it carries, and that sorting
// is most of why one field is sand and the next is clay. A grain of sand goes
// down at the first slackening; silt travels to where the river spills; clay
// stays up in the water almost as long as there is any water moving at all.
// So the share is the grain's, and the three of them average within a
// hundredth of the single figure this was, so that a map silts up at about
// the rate the whole model was measured at and what is new is where each
// grain of it lands rather than how much of it settles.
var settleOf = [Grains]float64{Sand: 0.62, Silt: 0.33, Clay: 0.10}

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

	// Highest ground first, so that what a tile sheds is in the water before
	// the tile below it is asked what the water is carrying.
	order := make([]int32, n)
	for i := range order {
		order[i] = int32(i)
	}
	sort.Slice(order, func(a, b int) bool {
		ha, hb := g.Tiles[order[a]].Height, g.Tiles[order[b]].Height
		if ha != hb {
			return ha > hb
		}
		return order[a] < order[b] // ties by position, so an age repeats
	})

	load := make([][Grains]float64, n)   // soil in the water leaving each tile
	change := make([]float64, n)         // metres gained or lost
	gained := make([][Grains]float64, n) // what was laid down here, by grain
	for _, i := range order {
		t := &g.Tiles[i]
		p := geom.Pos{X: int(i) % g.W, Y: int(i) / g.W}
		slope := g.Slope(p)

		// What the water lays down here: more of it the gentler the ground,
		// and more of the coarse than of the fine, which is the sorting. A
		// tile takes the mixture the water had left to give it, not the
		// mixture that came off the hill.
		if carrying(load[i]) > 0 {
			var settled [Grains]float64
			slack := clamp01(1 - slope/SettleSlope)
			for k := range settled {
				settled[k] = load[i][k] * settleOf[k] * slack
				load[i][k] -= settled[k]
			}
			if t.Wet() {
				// A river in flood puts most of its silt over the bank. That
				// is what a flood plain is: not ground the river spared, but
				// ground the river made. Without it the silt stays in the
				// channel, the bed rises, and the good land beside it slowly
				// washes away instead of being fed.
				var bank []int
				for _, off := range Dirs {
					c := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
					if !g.In(c) {
						continue
					}
					if b := g.At(c); !b.Wet() && b.Drain < FloodDepth {
						bank = append(bank, g.Index(c))
					}
				}
				if len(bank) > 0 {
					for k := range settled {
						over := settled[k] * Overbank
						for _, j := range bank {
							change[j] += over / float64(len(bank))
							gained[j][k] += over / float64(len(bank))
						}
						settled[k] -= over
					}
				}
			}
			for k := range settled {
				change[i] += settled[k]
				gained[i][k] += settled[k]
			}
		}
		// What it takes away. A stripping takes the soil as it finds it: the
		// water carries off the mixture that was there, and the sorting
		// happens where it puts it down again rather than where it picks it
		// up.
		stripped := by * Wash * math.Sqrt(t.Flow) * slope * hold(t)
		change[i] -= stripped
		was := parts(t)
		for k := range load[i] {
			load[i][k] += stripped * was[k]
		}

		a := g.Aspect(p)
		if a == (geom.Pos{}) {
			continue // the water and everything in it leaves the map here
		}
		down := int32(g.Index(geom.Pos{X: p.X + a.X, Y: p.Y + a.Y}))
		for k := range load[i] {
			load[down][k] += load[i][k]
		}
	}

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
