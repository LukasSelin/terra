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

// overbankDecay is the distance, in metres, over which what a river lays
// down on its flood plain thins to a third (1/e) of what it lays at its own
// channel. Pizzuto (1987), taking the silt a flood carries onto its plain as
// diffusing away from the channel while it settles, has what is laid thinning
// exponentially with the distance from it, which is how the deposits of floods
// are found. How fast is a fact about a river and its plain, and fifty metres -
// two tiles - is a choice, not a measurement.
//
// It was a share, seven tenths of everything laid, shared evenly among
// whichever of the eight neighbours stood dry: a river with one dry tile of
// bank beside it put the whole seven tenths on that tile, and nothing past the
// tiles touching the channel got any.
const overbankDecay = 50 * metre

// overbankReach is how many tiles out from its channel a flood lays anything,
// past which exp(-d/overbankDecay) is too little to book.
const overbankReach = 3

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

// hold is how much of the soil on a tile the creep moves in an age, by what is
// growing or standing on it and by what the soil itself is made of. Roots are
// what hold a hillside together against its own weight; a roof or a road takes
// the ground it covers out of the weather altogether; and loose sand goes where
// clay stays, whatever is growing on either.
//
// The water is charged differently: by the rock, which is rockErodibility, and
// by what grows, as a stress to clear and not a share - see criticalFall.
func hold(t *Tile) float64 {
	if t.Mark != None {
		return 0
	}
	return t.Terrain.Hold() * t.Wash()
}

// Water on the ground: how deep it runs, how wide, and the stress it puts on
// its bed.
const (
	// waterDensity is ρ, in kilograms a cubic metre, and gravity is g.
	waterDensity = 1000.0
	gravity      = 9.81
	// manning is the Manning roughness the depth of running water is read
	// with: a natural channel, clean and winding (Chow 1959, 0.033 to 0.045).
	manning = 0.035
	// widthCoeff is α in W = α·Q^(3/8)·S^(-3/16), Finnegan and others' (2005)
	// width of a channel cut into its bed. Their law gives the exponents; the
	// coefficient is set so that ten cubic metres a second on a fall of one in
	// a hundred is eleven metres across, which is what Leopold and Maddock's
	// (1953) W = 3.5·Q^(1/2) gives a river that size.
	widthCoeff = 2.0
	// leastFall is the fall the width and the depth are read at on ground that
	// falls less, so that a river on a dead flat is not read as infinitely
	// wide or deep.
	leastFall = 1e-4
)

// flowWidth is how wide q cubic metres a second crossing a tile span metres
// across runs, in metres. On a channel it is Finnegan's width, never wider than
// the tile; anywhere else the water is a sheet, and a sheet is as wide as the
// ground it runs over.
func flowWidth(t *Tile, q, fall, span float64) float64 {
	if !t.Wet() {
		return span
	}
	return math.Min(span, channelWidth(q, fall))
}

// channelWidth is Finnegan and others' (2005) width of a channel carrying q
// cubic metres a second down a fall of s, in metres: see widthCoeff.
func channelWidth(q, s float64) float64 {
	return widthCoeff * math.Pow(q, 3.0/8) * math.Pow(math.Max(s, leastFall), -3.0/16)
}

// criticalFall is the fall at which q cubic metres a second, running w metres
// wide, puts tc pascals on its bed: the fall the water has to be on before
// what covers the ground lets it take anything.
//
// The stress is ρ·g·d·S, with the depth d by Manning, (n·q/(w·√S))^(3/5), so
// τ = ρ·g·(n·q/w)^(3/5)·S^(7/10) and the fall that makes it tc is that turned
// round. Istanbulluoglu and Bras (2005) write the water's work as
// E = K(τ - τc)^a; with a at one, and read on the fall, that is
// E = K·Q^m·(S - Sc) - stream power, less the fall the cover soaks up - which
// is linear in the fall, so the implicit step takes it as it takes the rest:
// the water cuts toward its receiver's height raised by Sc over the run.
// Nothing growing, Sc is nothing, and it is Braun and Willett's step exactly.
func criticalFall(q, w, tc float64) float64 {
	if tc <= 0 {
		return 0
	}
	if q <= 0 {
		return math.Inf(1)
	}
	k := waterDensity * gravity * math.Pow(manning*q/w, 0.6)
	return math.Pow(tc/k, 1/0.7)
}

// floodFlow is how many times its mean discharge a tile carries in the storms
// that do the water's work, which is the discharge the stress on the ground,
// the width of a channel and the settling of what the water carries are read
// at. Stream power's K is set against the mean flow, because it is fitted to
// rates over years; but whether grass lets go of a hillside, and whether silt
// comes out of the water, is decided in the hour the water is running, and the
// work is done by the floods (Wolman and Miller 1960).
//
// A thousand: fifty millimetres an hour running off a tile in a storm, against
// the default valley's four hundred and fifty millimetres a year. Read at the
// mean, the water on a hillside ran a millimetre deep and put a fifth of a
// pascal on the ground, and nothing that grows would ever let go of it. By the
// multiple, over seed 3's slopes with the cover at the grass and wood of the
// time:
//
//	floodFlow   ploughed mm/yr   ploughed against wooded   valley lowering, mm/yr
//	   300           0.27               366                        0.062
//	  1000           0.46               339                        0.090
//	  3000           0.60                28                        0.139
//
// A storm's runoff and not a flood's discharge off a catchment, which is a
// smaller multiple of its mean the larger the catchment; the same multiple is
// taken everywhere, which errs toward a great river's floods.
const floodFlow = 1000.0

// Settling velocities, in metres a second, of the three grains: fine sand,
// silt and clay, from Ferguson and Church's (2004) law for natural grains of
// quartz in water, read at 0.13, 0.01 and 0.001 millimetres across: a
// centimetre a second, a tenth of a millimetre, and a thousandth of one.
var fallSpeed = [Grains]float64{Sand: 1e-2, Silt: 1e-4, Clay: 1e-6}

// settleShare is how much of a grain the water carries over a run of span
// metres, w wide at q cubic metres a second, lets fall out: 1 - exp(-Vs·span·w/q).
//
// The load is spread through the water, so what crosses the bed in a second
// is q of it and what falls on the bed is Vs·span·w, and the load decays as
// it goes. It is Yuan and others' (2019) deposition, G·Qs/Q, taken over a
// tile rather than a point; and because it is an exponential in the distance,
// two tiles of half the span let fall what one tile of the whole did. How big
// a tile is does not decide how far the sand gets.
//
// It was a share for each grain - 0.62, 0.33, 0.10 of what passed - faded
// linearly to nothing on a fall of twelve in a hundred and held below nine
// tenths: a tile's worth of settling, whatever the tile was worth.
func settleShare(vs, span, w, q float64) float64 {
	if q <= 0 {
		return 1
	}
	return 1 - math.Exp(-vs*span*w/q)
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
	was := g.soils()
	g.wear(ageYears)
	// And sideways: a river cuts the outside of its bends while the weather
	// takes the hillsides down. See meander.go.
	g.meander(1)
	// What the age has left steeper than ground can stand on comes down, and
	// lies below the slope it came off. See slide.go.
	g.landslide(true)
	// What the age wore away has bared the bed beneath it here and there, and
	// the soil is made again from that. See strata.go.
	g.expose()
	g.drain()
	g.carve(w.RNG)
	g.height()
	// The coast has moved, so the tide's reach has, and the flats with it.
	g.tides()
	// And what is water or salt now, which the weathering read as ground,
	// holds no soil to age.
	g.drownSoils()
	g.resoil(was)
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
// years is how long this pass is. An age is a decade for a settlement and
// Erode passes ageYears; a history passes an epoch, which is millions of years,
// on the same K and the same creep - see epochYears - with its tiles read at
// the width of a piece of a planet rather than of a field.
//
// What goes comes off the soil first and the rock after, and the rock goes on
// making soil under whatever is left: see soil.go.
//
// It leaves the drainage stale on purpose: the caller says when the water is
// worked out again, because doing it here would do it twice in Erode.
func (g *Grid) wear(years float64) {
	defer phase("wear")()
	n := len(g.Tiles)
	c := g.waterStep(years)
	// The waves on the coast as it stands at the start of the step, and the
	// sand the rivers bring them. See coast.go.
	s := g.surfOf(nil)
	if len(s.cells) > 0 {
		c.surf, c.sands = s.slot, make([]float64, n)
	}
	next := c.solve(settleIters)
	change := make([]float64, n)
	gained := make([][Grains]float64, n)
	g.exported = c.account(next, change, gained, func(i int32, laid [Grains]float64) {
		if t := &g.Tiles[i]; t.Wet() && !g.standing(int(i)) {
			g.overbank(int(i), laid, change, gained)
			return
		}
		for gr := range laid {
			change[i] += laid[gr]
			gained[i][gr] += laid[gr]
		}
	})
	// The soil the water took: all of the cut, as far as the soil went.
	lost := make([]float64, n)
	for i := range lost {
		lost[i] = math.Min(c.soil[i], c.cutAt(next, int32(i)))
	}
	g.creep(years, change, gained, lost)

	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			t := &g.Tiles[i]
			// No floor under it. The water is never cut below the ground it
			// runs into - see fluvial.go - and the creep never takes a tile
			// below the one it gives to, so nothing here digs a hole; what a
			// floor at the foot of the map did was make ground out of nothing,
			// first by lifting a sea bed that lay under it and then by refusing
			// to let the creep ease one down a hand's breadth further.
			g.Height[i] += change[i]
			// What is left of the soil, what arrived on it, and what the rock
			// made under it over the age. What arrives is worked into what was
			// there; what the rock makes is the rock's own mixture.
			h := math.Max(0, c.soil[i]-lost[i])
			laid := carrying(gained[i])
			surface := !t.Wet() && !t.Terrain.Tidal()
			// What time has made of the soil goes with the soil: what was
			// taken took its share of it, and what cut through into the rock
			// has left a fresh surface. See pedogenesis.go.
			if g.pedons && g.deep == 0 {
				switch {
				case c.soil[i] <= 0 || h <= 0:
					clearSoil(t)
				default:
					strip(t, lost[i]/c.soil[i])
				}
			}
			if surface {
				mix(t, h, gained[i])
			}
			h += laid
			if surface {
				made := soilMade(h, years, SoilMaking*g.weathering(i)) - h
				if made > 0 {
					sand, clay := g.TextureAt(g.PosOf(i))
					blend(t, h, [Grains]float64{Sand: made * sand, Silt: made * clamp01(1-sand-clay), Clay: made * clay})
					h += made
				}
			}
			g.Soil[i] = float32(h)
			switch {
			case g.deep > 0:
				g.deepExposure(i, -change[i], years)
			case g.pedons:
				g.ripenSoil(i, years)
			}
		}
	})
	g.coast(s, years, c.sands)
}

// overbank books what a river lays down at channel tile i: on its own bed, and
// on the dry ground within the flood's reach around it, thinning away from the
// channel as exp(-d/overbankDecay) with d the distance from it (Pizzuto 1987).
// What goes over the bank is what the flood holds up in the water, its silt
// and clay, which is what Pizzuto's diffusion is of; its sand goes along the
// bed and is laid there, as the bars of the channel.
// A river in flood puts its silt over the bank; that is what a flood plain is -
// not ground the river spared, but ground the river made. Without it the silt
// stays in the channel, the bed rises, and the good land beside it washes away
// instead of being fed.
//
// The weights are shared out, so whatever is laid is booked whole: a river in a
// gorge, with no dry ground low enough beside it, keeps all of it on its bed,
// and one on a broad plain keeps a fifteenth.
func (g *Grid) overbank(i int, laid [Grains]float64, change []float64, gained [][Grains]float64) {
	p := g.PosOf(i)
	var at [(2*overbankReach + 1) * (2*overbankReach + 1)]int32
	var weight [len(at)]float64
	at[0], weight[0] = int32(i), 1
	k, total := 1, 1.0
	for dy := -overbankReach; dy <= overbankReach; dy++ {
		for dx := -overbankReach; dx <= overbankReach; dx++ {
			q := geom.Pos{X: p.X + dx, Y: p.Y + dy}
			if (dx == 0 && dy == 0) || !g.In(q) {
				continue
			}
			if b := g.At(q); b.Wet() || g.Drain[g.Index(q)] >= FloodDepth || b.Mark != None {
				continue
			}
			wt := math.Exp(-math.Hypot(float64(dx), float64(dy)) * g.span() / overbankDecay)
			at[k], weight[k] = int32(g.Index(q)), wt
			k++
			total += wt
		}
	}
	change[i] += laid[Sand]
	gained[i][Sand] += laid[Sand]
	for m := range k {
		j := at[m]
		share := weight[m] / total
		for _, gr := range [...]Grain{Silt, Clay} {
			change[j] += laid[gr] * share
			gained[j][gr] += laid[gr] * share
		}
	}
}

// floodWidth is how wide, in metres, the ground beside channel tile i is that a
// flood spreads over: the dry ground within overbankReach that stands within
// FloodDepth of its water, as a width along the river - its area over the
// length of river it lies along.
//
// It is what makes a flood plain a flood plain. The water in a channel runs
// deep and fast and keeps its silt and clay up, which settle at a tenth of a
// millimetre and a thousandth of one a second; spread a hundred metres wide
// over a plain it runs shallow and slow, and they come out of it there. Read
// over the channel alone, nothing finer than sand settled anywhere on a valley,
// and over forty ages its low ground lost ground while its hills did.
func (g *Grid) floodWidth(i int) float64 {
	p := g.PosOf(i)
	n := 0
	for dy := -overbankReach; dy <= overbankReach; dy++ {
		for dx := -overbankReach; dx <= overbankReach; dx++ {
			q := geom.Pos{X: p.X + dx, Y: p.Y + dy}
			if (dx == 0 && dy == 0) || !g.In(q) {
				continue
			}
			if b := g.At(q); !b.Wet() && g.Drain[g.Index(q)] < FloodDepth && b.Mark == None {
				n++
			}
		}
	}
	return float64(n) * g.span() / float64(2*overbankReach+1)
}

// Diffusivity is how fast the ground creeps, in square metres a year, on ground
// that holds nothing back: D in dh/dt = D·curvature. What a pass moves between
// two neighbours is a share of the difference in height between them - see
// creepShare - and on a field's tiles an age of it is four in a thousand. It is
// the slow slumping of a hillside under its own weight - frost heave, burrows,
// rain splash - and it is the other half of what shapes a slope: the water
// cuts, and the ground either side of the cut falls in after it.
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
// The figure is the real one. Roering and others (1999), calibrating
// hillslopes in the Oregon Coast Range against their erosion rates, have
// 0.0031 to 0.0045 square metres a year; Fernandes and Dietrich (1997) put
// the world between 0.00044 and 0.036. At this figure, times what open grass
// holds, grass creeps at 0.0036.
//
// It was a share of 0.1 an age, which on grass as it held then was nearly a square metre a
// year, two hundred and sixty times Roering's. That figure was measured
// against trenches rather than against ground: on the high fifth of a half
// globe over three seeds and sixty ages, the deepest hundredth of the ground
// lay 17.2, 18.5 and 25.2 metres below the ground either side of it without
// creep, and 7.7, 8.8 and 15.4 with it. What was cutting those trenches was
// water wearing ten times too fast; with the water at the real figure too -
// see Erodibility - a history weathers as it did. That share was 0.00384,
// which is this.
const Diffusivity = 0.06

// creepShare is the share of the difference in height between two neighbouring
// tiles that years of creep move from the higher to the lower, on ground that
// holds nothing back: diffusion taken on the grid, where the ground at a tile
// goes as a quarter of the share times its curvature in tiles.
func (g *Grid) creepShare(years float64) float64 {
	s := g.span()
	return 4 * Diffusivity * years / (s * s)
}

// soilActive is the most soil the creep draws on: the layer roots, burrows and
// frost stir, which is about a metre deep however much lies under it (Roering
// and others 2002, on the depth of the mixed layer on soil-mantled slopes).
const soilActive = 1.0

// creepSteepest is how near Critical the creep's speeding up is followed
// before it is held: at nineteen twentieths of it the flux is ten times the
// linear one, and past it the ground is not creeping but about to fail, which
// is the landslides' business.
const creepSteepest = 0.95

// creepSweeps is how many sweeps the creep's step is solved with. It is fixed,
// so an age comes out the same however close it came; each sweep takes the
// error down by Σk/(1+Σk), which on the steepest ground a history creeps is a
// third, so twelve leave a millionth.
const creepSweeps = 12

// creep books what an age of creep moves onto change and gained, and the soil
// it takes off each tile onto lost.
//
// The flux between two neighbours is Roering and others' (1999) nonlinear
// creep with the depth of soil in it, as Johnstone and Hilley (2015) found it:
//
//	q = D·(H/H0)·∇z / (1 − (|∇z|/Sc)²)
//
// so the soil goes faster the deeper it is, and without limit as the slope
// comes up to the critical one. D is Diffusivity charged to what holds the ground,
// H the soil on the higher tile up to soilActive, H0 SoilScale and Sc Critical.
// A tile with no soil on it has nothing to creep.
//
// Not in a history. A tile there is a piece of a planet read over millions of
// years - see deep - and how deep its soil is is not a fact about any hillside
// on it: under an epoch every soil-mantled slope in it has long been at its
// steady depth. So a history creeps at SoilScale everywhere, as it crept
// before the soil was kept; charged to the tile's own soil, a history's
// ground crept less wherever its first epochs had made little, and the small
// globes' coasts and dry basins came out differently enough that the first
// seed lost its tidal flats or the second its salt lake, whichever way the
// slides at the end of the history were taken.
//
// The step is taken implicitly, as Perron (2011) takes it: the heights at the
// end of the age are what the fluxes are read off, with the coefficients read
// off the heights at the start. The system is solved by sweeping; what it
// books is read off the pairs, so whatever the sweeps come to, what one tile
// gives its neighbour takes and no ground is made or lost. And no tile gives
// more soil than it has: a tile whose fluxes would take more is taken down to
// rock and no further.
//
// The rock does not slow it, for the reason given at hold; what is growing
// does, because roots are what hold a hillside together. Whatever somebody has
// built on stays where it is, and nothing slumps onto it.
func (g *Grid) creep(years float64, change []float64, gained [][Grains]float64, lost []float64) {
	defer phase("creep")()
	// A river great enough to wander has banks that are its own business: see
	// meander, which takes the outside of a bend and builds the inside, and
	// whose bends creep would otherwise ease back out as fast as they are cut.
	wander := meanderFlow
	share := g.creepShare(years)
	span := g.span()
	// How deep the soil the creep carries is counted, against SoilScale: see
	// above for why a history does not count it.
	depth := func(i int) float64 {
		if g.deep > 0 {
			return 1
		}
		return math.Min(float64(g.Soil[i]), soilActive) / SoilScale
	}
	// Half the pairs, so that each is taken once: east, and the three below.
	pairs := [...]struct {
		off       geom.Pos
		near, run float64
	}{
		{geom.Pos{X: 1, Y: 0}, 1, span},
		{geom.Pos{X: -1, Y: 1}, 0.5, span * math.Sqrt2},
		{geom.Pos{X: 0, Y: 1}, 1, span},
		{geom.Pos{X: 1, Y: 1}, 0.5, span * math.Sqrt2},
	}
	n := len(g.Tiles)
	// The creep's scratch is the Grid's, kept between ages: see creepScratch.
	// nb, diag and z are written on every tile below; k is only written on
	// the pairs that creep and only read on them, and is cleared anyway.
	cs := &g.creepScratch
	cs.fit(n, len(pairs))
	nb, k, diag, z := cs.nb, cs.k, cs.diag, cs.z
	clear(k)
	for i := range g.Tiles {
		z[i], diag[i] = g.Height[i], 1
	}
	for i := range g.Tiles {
		a := &g.Tiles[i]
		p := g.PosOf(i)
		for e, pr := range pairs {
			at := len(pairs)*i + e
			nb[at] = -1
			q := geom.Pos{X: p.X + pr.off.X, Y: p.Y + pr.off.Y}
			if !g.In(q) {
				continue
			}
			j := g.Index(q)
			b := &g.Tiles[j]
			if a.Mark != None || b.Mark != None || g.abyssal(i) || g.abyssal(j) {
				continue
			}
			if (a.Wet() && g.Flow[i] >= wander) || (b.Wet() && g.Flow[j] >= wander) {
				continue
			}
			top, over := a, i
			if g.Height[j] > g.Height[i] {
				top, over = b, j
			}
			fall := math.Min(math.Abs(g.Height[i]-g.Height[j])/pr.run, creepSteepest*Critical) / Critical
			// An eighth each, so that a tile standing above all eight of its
			// neighbours on SoilScale of soil gives up no more than the share of
			// its height over them.
			kk := share / 8 * pr.near * hold(top) * depth(over) / (1 - fall*fall)
			if kk <= 0 {
				continue
			}
			nb[at], k[at] = int32(j), kk
			diag[i] += kk
			diag[j] += kk
		}
	}
	// Jacobi sweeps on (1 + Σk)·z'_i − Σ k·z'_j = z_i.
	next, sum := cs.next, cs.sum
	copy(next, z)
	for s := 0; s < creepSweeps; s++ {
		copy(sum, z)
		for at, j := range nb {
			if j < 0 {
				continue
			}
			i := at / len(pairs)
			sum[i] += k[at] * next[j]
			sum[j] += k[at] * next[i]
		}
		for i := range next {
			next[i] = sum[i] / diag[i]
		}
	}
	// What each tile would give, and how much of that its soil covers.
	gives := cs.gives
	clear(gives)
	for at, j := range nb {
		if j < 0 {
			continue
		}
		i := at / len(pairs)
		if d := k[at] * (next[i] - next[j]); d > 0 {
			gives[i] += d
		} else {
			gives[j] -= d
		}
	}
	for at, j := range nb {
		if j < 0 {
			continue
		}
		i := int32(at / len(pairs))
		hi, lo := i, j
		moved := k[at] * (next[i] - next[j])
		if moved < 0 {
			hi, lo, moved = j, i, -moved
		}
		if moved <= 0 {
			continue
		}
		if have := float64(g.Soil[hi]) - lost[hi]; gives[hi] > have {
			moved *= math.Max(0, have) / gives[hi]
		}
		change[hi] -= moved
		change[lo] += moved
		lost[hi] += moved
		was := parts(&g.Tiles[hi])
		for gr := range was {
			gained[lo][gr] += moved * was[gr]
		}
	}
}

// creepScratch is creep's working memory, kept on the Grid between ages: see
// fit. nb and k are the pairs, four to a tile; the rest are by tile.
type creepScratch struct {
	nb                           []int32
	k, diag, z, next, sum, gives []float64
}

// fit gives the scratch its size for n tiles with pairs pairs each.
func (s *creepScratch) fit(n, pairs int) {
	s.nb = sized(s.nb, pairs*n)
	s.k = sized(s.k, pairs*n)
	s.diag = sized(s.diag, n)
	s.z = sized(s.z, n)
	s.next = sized(s.next, n)
	s.sum = sized(s.sum, n)
	s.gives = sized(s.gives, n)
}

// soils is what SoilAt reads on every tile, for resoil to read the age's
// difference against.
func (g *Grid) soils() []float64 {
	out := make([]float64, len(g.Tiles))
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			out[i] = g.SoilAt(g.PosOf(i))
		}
	})
	return out
}

// resoil moves what the ground could grow by what the age did to it: Rich
// goes up or down by as much as SoilAt did, which is to say by the soil the
// age took away or laid down, the water it brought nearer or took further
// off, and what the soil is now made of.
//
// By the difference, and not to the reading itself. What a settlement has
// done to its ground is in Rich and not in SoilAt - a field somebody has cut a
// channel to holds more than the bare ground around it would, and that is the
// whole point of having dug it - and an earlier version pulled every tile
// toward what its drainage alone would give, which quietly undid irrigation
// every age and cost the settlements that had invested in it dearly. A
// difference keeps what they added and charges them for what the weather
// took.
//
// It used to add the change in height over three metres, a depth of soil the
// same everywhere, and pull the rest up a twelfth of the way toward the
// reading each age. The soil is kept now, so both of those are the soil.
func (g *Grid) resoil(was []float64) {
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if t.Wet() || t.Terrain.Tidal() || t.Terrain == Pan {
			continue // salt grows nothing, however much the river feeds it
		}
		g.Rich[i] = clamp01(g.Rich[i] + g.SoilAt(g.PosOf(i)) - was[i])
		g.Fertility[i] = math.Min(g.Fertility[i], g.Rich[i])
	}
}

// carrying is how much soil of every grain a load has in it.
func carrying(load [Grains]float64) float64 {
	return load[Sand] + load[Silt] + load[Clay]
}

// mix works what has just been laid down on a tile into the held metres of
// soil already there. What arrives does not replace what was there; it is
// ploughed and burrowed and frozen into the top of it, so the tile ends up
// somewhere between the two, nearer the newcomer the more of it there is
// against what there was: a river that lays down a centimetre on a metre of
// soil barely moves what the field is made of, and one that lays it on bare
// rock has made the field.
//
// And what arrives is younger than what was there: see buryIn.
func mix(t *Tile, held float64, laid [Grains]float64) {
	blend(t, held, laid)
	buryIn(t, held, carrying(laid))
}

// blend is mix for the mixture alone: what the rock makes under a soil is part
// of that soil's forming and not new ground laid on it.
func blend(t *Tile, held float64, laid [Grains]float64) {
	d := carrying(laid)
	if d <= 0 {
		return
	}
	held = math.Max(0, held)
	t.Sand = (t.Sand*held + laid[Sand]) / (held + d)
	t.Clay = (t.Clay*held + laid[Clay]) / (held + d)
}

// waterStep is the water's step over years, set up to be solved: how hard it cuts
// each tile, what settles where, and what the tide, the map's edges and the
// still water do with what reaches them. See wear.
func (g *Grid) waterStep(years float64) fluvial {
	defer phase("waterStep")()
	n := len(g.Tiles)
	// The step's slices are the Grid's scratch, kept from step to step: see
	// stepScratch. h, soil and parts are written on every tile below; the
	// rest are only written where the water cuts, and are cleared first.
	s := &g.stepScratch
	s.fit(n)
	recv, run := g.receiversInto(s.recv, s.run)
	c := fluvial{
		h:       s.h,
		recv:    recv,
		stack:   stackInto(recv, &s.stack),
		f:       s.f,
		drop:    s.drop,
		settle:  s.settle,
		parts:   s.parts,
		supply:  g.bankLoad,
		soil:    s.soil,
		rock:    s.rock,
		eff:     s.eff,
		abrade:  s.abrade,
		lasts:   s.lasts,
		scratch: &s.solve,
	}
	clear(c.f)
	clear(c.drop)
	clear(c.settle)
	clear(c.rock)
	clear(c.eff)
	clear(c.abrade)
	for i := range c.lasts {
		c.lasts[i] = 1
	}
	// What the rivers took off their banks is carried from here: see meander.
	g.bankLoad = nil
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			t := &g.Tiles[i]
			c.h[i] = g.Height[i]
			c.soil[i] = float64(g.Soil[i])
			// What the water takes off a tile is its soil, or its rock where it
			// has none, which is the soil the rock would make.
			c.parts[i] = parts(t)
			if g.Soil[i] <= 0 {
				sand, clay := g.TextureAt(g.PosOf(i))
				c.parts[i] = [Grains]float64{Sand: sand, Silt: clamp01(1 - sand - clay), Clay: clay}
			}
			// Ground somebody has built on is out of the water's reach: it is
			// neither cut nor settled on.
			if int(recv[i]) == i || t.Mark != None {
				continue
			}
			// How hard the water cuts: stream power, charged to what the soil
			// is made of for the soil and to the rock for the rock. See
			// fluvial.go and rockErodibility.
			power := years * Erodibility * math.Sqrt(g.Flow[i]) / run[i]
			c.f[i] = power * t.Wash()
			c.rock[i] = power * rockErodibility(t)
			c.abrade[i] = abrasion(run[i])
			// What grows on it holds its soil until the water's stress in a
			// flood clears what it stands, and what settles is what a flood
			// lets fall: see floodFlow, criticalFall and settleShare.
			q := g.Flow[i] * floodFlow
			fall := (g.Height[i] - g.Height[recv[i]]) / run[i]
			w := flowWidth(t, q, fall, run[i])
			if g.deep > 0 {
				// A tile of a history is a piece of a planet, a hundred
				// kilometres across, and its water runs in a network of
				// channels too fine for it to draw. Read as a sheet that wide
				// it put no stress on anything, and read as settling over the
				// whole run it laid every grain of sand back where it was cut:
				// either way the ranges rose for ever, to two hundred
				// kilometres by the sixteenth epoch of a small globe. What a
				// planet's rivers carry off a tile settles where they stop, in
				// its basins and its seas - see stillWork - and the beds of its
				// channels grow nothing.
				continue
			}
			c.drop[i] = math.Min(criticalFall(q, w, t.Terrain.Shear())*run[i], math.MaxFloat64)
			// A river in flood is not the width of its channel. Its sand goes
			// along the bed and settles there, over the channel; its silt and
			// clay are held up in the water, which spreads over the ground
			// beside it the flood reaches, and settle over all of that. See
			// floodWidth and overbank.
			plain := w
			if t.Wet() && !g.standing(i) {
				plain += g.floodWidth(i)
			}
			for gr := range fallSpeed {
				over := plain
				if Grain(gr) == Sand {
					over = w
				}
				c.settle[i][gr] = settleShare(fallSpeed[gr], run[i], over, q)
			}
		}
	})
	g.tideWork(&c, recv)
	g.edgeWork(&c, recv, years)
	g.stillWork(&c, recv)
	return c
}
