package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
)

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
}

// Saving is what a road laid on p would take off each crossing of it, as a
// multiple of what a road on grass takes off. It is the other half of what a
// length of road is worth, and until it was asked the only half being read
// was how many people walk there.
//
// A road costs the same timber wherever it goes but does not save the same
// amount. On grass it turns a 1 into a 0.5; through a wood a 2.2, and over
// water a 3.5, and that last is why a bridge is worth six lengths of ordinary
// street to the people who cross it. Read on wear alone a settlement paves
// the flat ground it was already crossing easily and leaves the marsh, the
// thicket and the river - the places where the going is dear, which is to say
// the places where a road is the whole point. Weighing the wear by this is
// what lets a crossing win on its merits: it earned an exception before,
// because the bar it could not clear was written for a lane.
//
// Grass is the unit, so open ground reads exactly as worn as it is walked.
// The bodily saving is not in here: a road is easier underfoot as well as
// quicker, but roadDrain is the same wherever the road lies, so it says
// nothing about which ground is worth paving. Nor is the climb, which a road
// does not flatten - see StepCost, where the slope is added whatever is
// built on the tile.
// The mark being laid is the caller's to name, because what a way is called
// and what it costs are a game's to say; the land only divides one by the
// other.
func (g *Grid) Saving(p geom.Pos, laying Mark) float64 {
	if !g.In(p) {
		return 0
	}
	paved := markCost[laying]
	return (moveCost[g.At(p).Terrain] - paved) / (moveCost[Grass] - paved)
}

// SwimLoad is the most a walker may be carrying and still take to the water.
// It is not a heavy pack; it is nothing at all, near enough. People are poor
// swimmers with both arms free, and a person holding a sack of grain over a
// river is a person drowning: what they do in life is put the sack down or
// walk to the bridge. So the water is not dear to a laden walker, it is shut,
// and the threshold is here only so that a crumb left in a pocket does not
// count as cargo.
//
// This is what makes a bridge worth its timber to somebody who already lives
// beside a ford. Wading was always slow; now it is the difference between
// carrying the harvest home and not carrying it at all, and the far bank is
// only part of the settlement for as long as the crossing stands.
const SwimLoad = 0.1

// Carrying tells the router how much the walker it is about to route for is
// holding, so that a laden walker is routed round open water instead of
// through it. It holds for the next route this router runs and no longer,
// which is what keeps a load from leaking into somebody else's journey.
func (r *Router) Carrying(load float64) *Router {
	r.load = load
	return r
}

// Carrying routes on the grid's own router, for callers working one at a
// time.
func (g *Grid) Carrying(load float64) *Router {
	return g.ownRouter().Carrying(load)
}

// MoveCost returns the ticks of effort needed to enter p. Tiles off the map
// are infinitely expensive, which keeps agents inside it.
func (g *Grid) MoveCost(p geom.Pos) float64 {
	if !g.In(p) {
		return math.Inf(1)
	}
	t := g.At(p)
	if t.Mark != None {
		return markCost[t.Mark]
	}
	return moveCost[t.Terrain]
}

// Climb and Descend are the ticks a metre of rise and a metre of fall add to
// a step. Going up is what costs: a steep tile on this map rises seven metres
// or so, which is most of another tile's walking on top of the ground itself,
// and that is what makes a route round the shoulder of a hill cheaper than a
// route over it. Coming down is charged a little too, because a walker picks
// their way down a bank rather than running at it, and because a step that
// cost nothing downhill would make a zigzag look free.
const (
	Climb   = 0.10
	Descend = 0.02
)

// StepCost is the effort of moving from one tile to the next: the ground
// being entered, plus the climb or the descent into it, plus the fence
// between them if there is one. It is what routing costs a journey by, so
// agents round a hill rather than going over it, walk round a hedged holding
// rather than through the corn, and so the ways they wear - and the roads
// they lay on those ways - follow the contours and the valley floors the way
// real ones do.
func (g *Grid) StepCost(from, to geom.Pos) float64 {
	return g.StepCostFor(from, to, 0)
}

// StepCostFor is StepCost for a named walker, whose own fields are theirs to
// walk into. Everybody else's cost them the climb over the fence.
func (g *Grid) StepCostFor(from, to geom.Pos, holder Holder) float64 {
	c := g.MoveCost(to)
	if math.IsInf(c, 1) || !g.In(from) {
		return c
	}
	c += g.fenceCost(int32(from.Y*g.W+from.X), int32(to.Y*g.W+to.X), holder)
	if d := g.Height(to) - g.Height(from); d > 0 {
		return c + Climb*d
	} else {
		return c - Descend*d
	}
}

// MoveDrain returns how hard on the body a tick of walking into p is, as a
// multiple of the ordinary cost. Only paving changes it.
func (g *Grid) MoveDrain(p geom.Pos) float64 {
	if !g.In(p) {
		return 1
	}
	return markDrain[g.At(p).Mark]
}

// StepToward returns the tile an agent at from should enter next on its way
// to to: the first step of the cheapest route there. Because the route is
// costed rather than guessed at, a walker rounds a thicket, fords a river
// only where fording beats going round, and joins a road that runs its way
// even when the road starts off to one side. Ties keep the straight-line
// step, so runs repeat.
func (r *Router) StepToward(from, to geom.Pos) geom.Pos {
	g := r.g
	if from == to || !g.In(to) {
		r.load, r.limit = 0, 0
		return from
	}
	return r.route(&r.scratch, from, to, true, g.Toward(from, to)).Step(to)
}

// StepToward routes on the grid's own router, for callers working one at a
// time.
func (g *Grid) StepToward(from, to geom.Pos) geom.Pos {
	return g.ownRouter().StepToward(from, to)
}

// Path is the cheapest way from one tile to another, from excluded and to
// included. It is what an agent is given to walk when it settles on a plan,
// so that the way is worked out once rather than re-asked at every step.
func (r *Router) Path(from, to geom.Pos) []geom.Pos {
	g := r.g
	if from == to || !g.In(to) {
		r.load, r.limit = 0, 0
		return nil
	}
	return r.route(&r.scratch, from, to, true, g.Toward(from, to)).Path(to)
}

// Path routes on the grid's own router, for callers working one at a time.
func (g *Grid) Path(from, to geom.Pos) []geom.Pos {
	return g.ownRouter().Path(from, to)
}

// TravelCost is the ticks of walking from one tile to another along the route
// the agent would actually take. Deciding uses it in place of raw distance,
// so a target across the water is judged as far as the wading makes it, and
// one along a street as near as the paving makes it.
func (r *Router) TravelCost(from, to geom.Pos) float64 {
	g := r.g
	if from == to {
		r.load, r.limit = 0, 0
		return 0
	}
	if !g.In(to) {
		r.load, r.limit = 0, 0
		return math.Inf(1)
	}
	if r.surveyed && from == r.spreadFrom && (r.load > SwimLoad) == r.spreadLaden && r.holder == r.spreadHolder {
		limit := r.limit
		r.load, r.limit = 0, 0
		if c := r.fromSurvey(to); limit <= 0 || c < limit {
			return c
		}
		return math.Inf(1)
	}
	return r.route(&r.scratch, from, to, true, g.Toward(from, to)).Cost(to)
}

// TravelCost routes on the grid's own router, for callers working one at a
// time.
func (g *Grid) TravelCost(from, to geom.Pos) float64 {
	return g.ownRouter().TravelCost(from, to)
}

// FenceToll is the ticks of climbing a hedge, in and out. It has to beat the
// detour round a block of the size anybody bothers to enclose or the fence
// is decoration: such a block is two or three tiles across, so going round
// it costs two or three tiles of walking, and at 4 the way round wins. See
// fenceSize, which is where a game decides what is worth hedging; this is
// only what the line costs to cross once it is there.
//
// It is a toll and not a wall. A walker who has no way round - a farmer
// whose neighbours' strips lie between the lane and their own, somebody cut
// off by a river on the other side - climbs over and pays for it, which is
// what people do. Nothing on this map is ever made unreachable by anything
// anybody built.
// FenceToll is exported because a game that draws boundaries has to know
// what crossing one is worth to whoever it charges.
const FenceToll = 4

// fenceCost is what crossing the line between two tiles costs. It is paid
// where one side is inside a fence and the other is not, and it is not paid
// by the holder of the ground: a farmer has a gate into their own field, and
// charging them to reach the strip they live off would only have made
// farming dearer than it is.
//
// The land charges it and does not draw it. Which ground lies inside a
// hedge, and whose it is, are marks a game makes on the map; that a boundary
// costs something to step over is the price of a step, and the price of a
// step is routing's.
func (g *Grid) fenceCost(from, to int32, holder Holder) float64 {
	f, t := &g.Tiles[from], &g.Tiles[to]
	if f.Fenced == t.Fenced {
		return 0 // both in the same enclosure, or both outside one
	}
	enclosed := f
	if t.Fenced {
		enclosed = t
	}
	if holder != 0 && enclosed.Owner == holder {
		return 0
	}
	return FenceToll
}

// What a crossing marks the ground.
//
// That ground remembers being walked on is the ground's own business, and
// so is forgetting it again - see Fade. How much of a mark one crossing
// makes is not: a game says what a walker of its own is worth, and whether
// what it is carrying counts for anything. So the two numbers are handed
// over once and the treading below reads them.
var (
	wearCrossing = 1.0
	wearHaul     = 0.0
)

// SetWear says what one crossing marks the ground, and what each armful
// carried over it marks it on top of that. Zero and zero would be a world
// whose ground keeps no memory of being walked, which is allowed: nothing
// here requires anybody to wear a path.
func SetWear(crossing, haul float64) {
	wearCrossing, wearHaul = crossing, haul
}

// Tread records that somebody crossed this tile carrying load armfuls.
func (g *Grid) Tread(p geom.Pos, load float64) {
	if g.In(p) {
		i := g.Index(p)
		g.Traffic[i] += wearCrossing + wearHaul*load
		g.Chunks[g.ChunkOf(i)].Trodden = true
	}
}

// Weather fades every tile's wear by one tick's worth, on the ground that
// is awake; ground asleep has no wear, having never been crossed.
func (g *Grid) Weather() {
	g.EachActiveRow(nil, func(lo, hi, _ int) { g.FadeWear(lo, hi, Fade) })
}
