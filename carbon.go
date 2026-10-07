package terra

import (
	"math"

	"github.com/LukasSelin/terra/internal/atmos"
	"github.com/LukasSelin/terra/internal/veg"
)

// The land's carbon, as stocks.
//
// What grows was a multiplier on what a soil holds, and the soil's carbon one
// stock that came to its level in a century; the fertility read the climate's
// organic matter over again with a Q10 of its own. Now the carbon is kept as
// CENTURY keeps it (Parton and others, 1987), in pools that each turn over in
// their own time, and everything that reads a soil's organic matter reads the
// pools.
//
//   - The vegetation's carbon is the vegetation's (vegetation.go): it is a
//     stock already, and it sheds what it grows each year as litter, less what
//     its fires send to the air.
//   - Litter: what has fallen and not yet rotted, on and in the top of the
//     soil. Its lignin goes to the slow pool, the rest to the fast, and the
//     fires burn it where they run.
//   - Fast, slow and passive: CENTURY's active, slow and passive soil organic
//     matter, the microbes and what they leave, what the minerals hold for
//     decades, and what they hold for a millennium. Between them they are the
//     soil's organic carbon in its top metre, Tile.Carbon.
//   - Peat: the catotelm's carbon, laid down under water (wetland.go).
//   - Permafrost: the carbon of the frozen ground under the active layer, as
//     far down as three metres: what the frost's churning has carried down
//     out of the thawing soil and frozen there (cryoturbation).
//
// Each pool's turnover goes up by carbonQ10 for every ten degrees of the
// phase of the year it is read in, and stops in a phase whose ground is
// frozen; it goes as the soil's water in the phase, as RothC reads it; it is
// slowed where the water table stands at the surface, as an anaerobic soil's
// is; and only the share of the top metre that thaws turns over at all.
//
// The pools are linear in what they hold, so a step of any length is taken
// exactly, as the exponential of the pools' matrix (see carbonStep), and a
// surface old enough is at the steady state of the climate it has now.

// Turnover times at the map's middling warmth, MeanTemp, through every phase
// of the year, with the soil's water unstressed, aired and thawed: the
// geometric middles of Parton and others' (1987) ranges, structural litter
// one to five years and the active, slow and passive soil organic matter two
// to five, twenty to fifty and four hundred to two thousand (the ranges of
// the issue that asked for the pools, #55, which are Parton's with the slow
// and passive widened to what the radiocarbon studies read; Trumbore 2000).
const (
	litterYears  = 2.2 // √(1·5)
	fastYears    = 3.2 // √(2·5)
	slowYears    = 32  // √(20·50)
	passiveYears = 894 // √(400·2000)
)

// Where the carbon goes when a pool turns over, after Parton and others
// (1987) and CENTURY 4's defaults (Metherell and others, 1993):
//
//   - litter: its lignin, 0.7 of it, to the slow pool, and of the rest
//     litterToFast to the fast pool; the rest is breathed out.
//   - fast: 0.85 − 0.68·T breathed out, T the soil's silt and clay together;
//     0.003 + 0.032·clay to the passive pool; the rest to the slow. And it
//     turns over as 1 − 0.75·T, read here against a loam's T of 0.6, which is
//     the soil the turnover times above are a middling soil's for.
//   - slow: slowBreath breathed out, 0.003 + 0.009·clay to the passive pool,
//     the rest back to the fast pool.
//   - passive: passiveBreath breathed out, the rest back to the fast pool.
//
// lignin is the lignin share of the litter: the herbs' and grasses' and the
// woody plants' (CENTURY's grassland, Parton and others 1987, and its forest
// version's leaf and fine-root litter, Parton and others 1993).
const (
	ligninToSlow  = 0.7
	litterToFast  = 0.45
	slowBreath    = 0.55
	passiveBreath = 0.55
	ligninHerb    = 0.15
	ligninWood    = 0.25
	loamFines     = 0.6
)

// The soil's water. RothC (Coleman and Jenkinson, 1996) has the decay go on
// at its full rate until the topsoil has lost 0.444 of the water it could
// lose, and fall from there in a straight line to 0.2 of it at the driest:
// here the water is the bucket's in each phase (soilwater.go), over what it
// holds. CENTURY slows the decay of a waterlogged soil to as little as 0.3 of
// its aired rate (its anaerobic factor; Parton and others, 1993); here the
// share of the thawed year the water table stands at the surface is
// waterlogged (wetland.go).
const (
	rothDry    = 0.2
	rothFull   = 1 - 0.444
	anaerobic  = 0.3
	carbonDeep = 3.0 // m: how deep the frozen ground's carbon is counted, as Hugelius and others (2014) count it
)

// Cryoturbation. The frost churns the active layer's carbon down into the
// ground under it, which freezes it there: CLM reads that as a diffusion of
// the soil's carbon at 5 cm² a year where the ground is permafrost (Koven and
// others, 2009, 2013). Diffusion with no decay below fills the frozen ground
// to the density of the soil over it, and gets there in the diffusion's time
// across it, z²/D: some ten thousand years for two metres.
const churnRate = 5e-4 // m² a year

// The fires. What a fire burns of what it runs through: the leaves, the
// grasses and the litter nearly all of, and the stems of the woody plants a
// fraction (van der Werf and others, 2010, the middles of their combustion
// completeness: 0.8–1 for the leaves and the litter, 0.2–0.4 for the stems).
//
// And only what is above the ground: a fire burns none of the roots, which
// are some two thirds of a grassland's carbon and a fifth of a forest's
// (Mokany and others, 2006: root to shoot ratios of about 2 under grass and
// 0.25 under trees), and of the litter it burns what lies on the ground,
// taken as the same share of it.
const (
	burnHerb  = 0.9
	burnWood  = 0.3
	aboveHerb = 1.0 / 3
	aboveWood = 0.8
)

// carbonPools is the carbon a tile holds that is not its vegetation's, in kg
// a square metre: its litter, its soil's fast, slow and passive pools in the
// top metre, and its frozen ground's under that. The soil's three are
// Tile.Carbon between them.
type carbonPools struct{ litter, fast, slow, passive, frozen float32 }

// carbonInput is what tile i puts into its soil in a year, and how: the
// litter that falls, in kg C a square metre a year; its lignin share; and
// what its fires send to the air off the vegetation, and the share of its
// ground they burn.
type carbonInput struct{ litter, lignin, fire, burned, surface float64 }

// inputOf is what tile i sheds into its soil under cover cv: its
// vegetation's NPP, less what the fires burn of the vegetation, where the
// vegetation has been laid, and the Miami model's under that cover where it
// has not (see nppOf). A field's harvest takes cv.input of it off.
func (g *Grid) inputOf(i int, c pedoClimate, cv cover) carbonInput {
	var in carbonInput
	t := &g.Tiles[i]
	if !g.vegLaid() || len(g.npp) != len(g.Tiles) {
		in.litter = g.nppOf(i, c) * cv.input
		in.lignin, in.surface = ligninHerb, aboveHerb
		if t.Terrain == Forest {
			in.lignin, in.surface = ligninWood, aboveWood
		}
		return in
	}
	npp := float64(g.npp[i])
	var woody, all, mass, burn float64
	for p := range PFTs {
		cov, m := g.Cover(i, p), g.Biomass(i, p)
		cc := burnHerb * aboveHerb
		if veg.Kinds[p].Woody {
			woody += cov
			cc = burnWood * aboveWood
		}
		all += cov
		mass += m
		burn += m * cc
	}
	in.burned = g.Burned(i)
	in.fire = math.Min(npp, in.burned*burn)
	in.lignin, in.surface = ligninHerb, aboveHerb
	if all > 0 {
		in.lignin = ligninHerb + (ligninWood-ligninHerb)*woody/all
		in.surface = aboveHerb + (aboveWood-aboveHerb)*woody/all
	}
	in.litter = npp - in.fire
	if t.Terrain == Field {
		in.litter *= fieldCover.input
	}
	return in
}

// carbonEnv is how fast tile i's soil turns its carbon over against the
// middling soil's at one: the warmth and the water of each phase of its
// year, the share of its top metre that thaws, and how waterlogged it is.
func (g *Grid) carbonEnv(i int, c pedoClimate) float64 {
	n := len(g.Tiles)
	swing := 0.0
	if len(g.swing) == n {
		swing = float64(g.swing[i])
	}
	soaked := len(g.soilWater) == n*atmos.Phases && len(g.soilHold) == n && g.soilHold[i] > 0
	var turn float64
	for k := range atmos.Phases {
		temp := c.temp + swing*atmos.SummerPeak*phaseSin[k]
		if temp <= 0 {
			continue // frozen ground does not rot
		}
		w := math.Min(1, c.wetness) // a map with no bucket: the rain against what the air takes
		if soaked {
			w = float64(g.soilWater[i*atmos.Phases+k]) / float64(g.soilHold[i])
		}
		turn += math.Pow(carbonQ10, (temp-MeanTemp)/10) * rothMoisture(w)
	}
	turn /= atmos.Phases
	// Only what thaws of the top metre turns over: under the permafrost's
	// share of the ground, the active layer's share of it.
	if f, ok := g.groundFrost(i); ok {
		s := frostShareOf(f.ttop)
		top := math.Max(1e-3, math.Min(1, g.carbonSoil(i)))
		turn *= (1 - s) + s*math.Min(1, f.thaw/top)
	} else if c.frozen {
		turn *= carbonFrozen
	}
	wet := c.sodden
	if s, ok := g.waterTable(i); ok {
		wet = s
	}
	return turn * (1 - (1-anaerobic)*wet)
}

// rothMoisture is RothC's rate modifier for a soil holding w of what it can.
func rothMoisture(w float64) float64 {
	return rothDry + (1-rothDry)*math.Min(1, math.Max(0, w)/rothFull)
}

// carbonSoil is how deep the ground tile i's soil carbon is held in is, in
// metres: its soil, and on permafrost, over the share of it that is frozen,
// the top metre of its soil and the weathered layer under it, which the
// frost churns the carbon down through (Bockheim 2007; Ping and others 2008:
// the Gelisols' carbon is in their cryoturbated mineral horizons, under
// however thin a soil the slopes leave).
func (g *Grid) carbonSoil(i int) float64 {
	soil := float64(g.Soil[i])
	f, ok := g.groundFrost(i)
	if !ok {
		return soil
	}
	s := frostShareOf(f.ttop)
	return (1-s)*soil + s*math.Max(soil, math.Min(1, soil+regolithDepth))
}

// carbonHeld is the share of a metre's soil carbon a soil depth metres deep
// holds: Jobbágy and Jackson's (2000) profiles have half of a metre's carbon
// in its top twenty centimetres, which is an e-fold of carbonDepth.
func carbonHeld(depth float64) float64 {
	return math.Min(1, -math.Expm1(-math.Max(0, depth)/carbonDepth)/-math.Expm1(-1/carbonDepth))
}

// carbonMatrix is tile i's pools as a linear system, dx/dt = A·x + b, over
// litter, fast, slow and passive, with the input b in its last column.
func (g *Grid) carbonMatrix(i int, c pedoClimate, cv cover) [5][5]float64 {
	in := g.inputOf(i, c, cv)
	env := g.carbonEnv(i, c)
	if g.Tiles[i].Terrain == Field {
		env *= fieldCover.decay // the plough
	}
	sand, clay := g.textureOf(i)
	fines := clamp01(1 - sand)
	held := carbonHeld(g.carbonSoil(i))

	kl := env/litterYears + in.burned*burnHerb*in.surface
	kf := env / fastYears * (1 - 0.75*fines) / (1 - 0.75*loamFines)
	ks := env / slowYears
	kp := env / passiveYears

	// Where each pool's turnover goes, as shares of it.
	rot := 0.0
	if kl > 0 {
		rot = env / litterYears / kl // the rest of the litter's turnover is its fires
	}
	lToS := rot * ligninToSlow * in.lignin * held
	lToF := rot * litterToFast * (1 - in.lignin) * held
	fToP := 0.003 + 0.032*clay
	fToS := math.Max(0, 1-(0.85-0.68*fines)-fToP)
	sToP := 0.003 + 0.009*clay
	sToF := 1 - slowBreath - sToP
	pToF := 1 - passiveBreath

	var a [5][5]float64
	a[0][0] = -kl
	a[1][0], a[2][0] = lToF*kl, lToS*kl
	a[1][1], a[2][1], a[3][1] = -kf, fToS*kf, fToP*kf
	a[1][2], a[2][2], a[3][2] = sToF*ks, -ks, sToP*ks
	a[1][3], a[3][3] = pToF*kp, -kp
	a[0][4] = in.litter
	return a
}

// carbonStep is the pools x after years under the system a: x(t) =
// e^(A·t)·x(0), the input carried as a fifth state held at one. The
// exponential is taken by scaling and squaring a Taylor series, so a step of
// ten years and one of a million are taken the same way and exactly.
func carbonStep(a [5][5]float64, x [4]float64, years float64) [4]float64 {
	if years <= 0 {
		return x
	}
	var norm float64
	for r := range 5 {
		var s float64
		for k := range 5 {
			s += math.Abs(a[r][k])
		}
		norm = math.Max(norm, s)
	}
	norm *= years
	squarings := 0
	if norm > 0.5 {
		squarings = int(math.Ceil(math.Log2(norm / 0.5)))
	}
	scale := years / math.Ldexp(1, squarings)
	var m, e, term [5][5]float64
	for r := range 5 {
		for k := range 5 {
			m[r][k] = a[r][k] * scale
		}
		e[r][r], term[r][r] = 1, 1
	}
	for n := 1; n <= 12; n++ {
		term = matMul(&term, &m)
		for r := range 5 {
			for k := range 5 {
				term[r][k] /= float64(n)
				e[r][k] += term[r][k]
			}
		}
	}
	for range squarings {
		e = matMul(&e, &e)
	}
	var out [4]float64
	for r := range 4 {
		v := e[r][4]
		for k := range 4 {
			v += e[r][k] * x[k]
		}
		out[r] = math.Max(0, v)
	}
	return out
}

func matMul(p, q *[5][5]float64) (o [5][5]float64) {
	for r := range 5 {
		for k := range 5 {
			var s float64
			for j := range 5 {
				s += p[r][j] * q[j][k]
			}
			o[r][k] = s
		}
	}
	return o
}

// carbonSteady is the pools' steady state under the system a: A·x = −b,
// solved by elimination. It is nothing where nothing turns over.
func carbonSteady(a [5][5]float64) [4]float64 {
	var m [4][5]float64
	for r := range 4 {
		for k := range 4 {
			m[r][k] = a[r][k]
		}
		m[r][4] = -a[r][4]
	}
	for col := range 4 {
		piv := col
		for r := col + 1; r < 4; r++ {
			if math.Abs(m[r][col]) > math.Abs(m[piv][col]) {
				piv = r
			}
		}
		if math.Abs(m[piv][col]) < 1e-300 {
			return [4]float64{}
		}
		m[col], m[piv] = m[piv], m[col]
		for r := range 4 {
			if r == col {
				continue
			}
			f := m[r][col] / m[col][col]
			for k := col; k < 5; k++ {
				m[r][k] -= f * m[col][k]
			}
		}
	}
	var x [4]float64
	for r := range 4 {
		x[r] = math.Max(0, m[r][4]/m[r][r])
	}
	return x
}

// ensurePools makes the pools' room on g. laySoil makes it before any tile
// is ripened over the rows; a grid made by hand gets it on its first.
func (g *Grid) ensurePools() {
	if len(g.pools) != len(g.Tiles) {
		g.pools = make([]carbonPools, len(g.Tiles))
	}
}

// ripenCarbon moves tile i's carbon pools on by years under the climate and
// cover it has now, and writes the soil's three into Tile.Carbon. The frozen
// ground's carbon heads for the soil's density over it through as much of
// the top carbonDeep metres as stays frozen, in the churning's own time.
func (g *Grid) ripenCarbon(i int, c pedoClimate, cv cover, years float64) {
	g.ensurePools()
	p := &g.pools[i]
	a := g.carbonMatrix(i, c, cv)
	x := carbonStep(a, [4]float64{float64(p.litter), float64(p.fast), float64(p.slow), float64(p.passive)}, years)
	p.litter, p.fast, p.slow, p.passive = float32(x[0]), float32(x[1]), float32(x[2]), float32(x[3])
	g.Tiles[i].Carbon = p.fast + p.slow + p.passive

	frozen := 0.0
	if f, ok := g.groundFrost(i); ok {
		ground := float64(g.Soil[i]) + regolithDepth
		if z := math.Min(carbonDeep, ground) - math.Max(1, f.thaw); z > 0 {
			density := float64(g.Tiles[i].Carbon) / math.Max(1e-3, math.Min(1, g.carbonSoil(i)))
			level := frostShareOf(f.ttop) * density * z
			frozen = toward(float64(p.frozen), level, churnRate/(z*z), years)
		}
	}
	p.frozen = float32(frozen)
}

// stripPools takes the share gone of tile i's soil off its carbon pools, from
// above, as strip does its soil's state; all of it where the whole of it went.
func (g *Grid) stripPools(i int, gone float64) {
	if i >= len(g.pools) || gone <= 0 {
		return
	}
	if gone >= 1 {
		g.pools[i] = carbonPools{}
		return
	}
	keep := float32(1 - gone)
	p := &g.pools[i]
	p.litter, p.fast, p.slow, p.passive, p.frozen = p.litter*keep, p.fast*keep, p.slow*keep, p.passive*keep, p.frozen*keep
}

// soilCarbonOf is what tile i's soil holds in its top metre, kg C a square
// metre: Tile.Carbon where the soil's state has been laid, and the steady
// state of the pools under its climate and cover now where it has not.
func (g *Grid) soilCarbonOf(i int) float64 {
	if g.pedons {
		return float64(g.Tiles[i].Carbon)
	}
	if !forms(&g.Tiles[i]) {
		return 0
	}
	c := g.pedoClimateOf(i)
	x := carbonSteady(g.carbonMatrix(i, c, g.coverOf(i, c.temp)))
	return x[1] + x[2] + x[3]
}

// Litter is the carbon in tile i's litter, in kg a square metre.
func (g *Grid) Litter(i int) float64 {
	if i < 0 || i >= len(g.pools) {
		return 0
	}
	return float64(g.pools[i].litter)
}

// SoilPools is the carbon in tile i's soil's fast, slow and passive pools, in
// kg a square metre of its top metre: see the top of carbon.go. They add up
// to Tile.Carbon.
func (g *Grid) SoilPools(i int) (fast, slow, passive float64) {
	if i < 0 || i >= len(g.pools) {
		return 0, 0, 0
	}
	p := g.pools[i]
	return float64(p.fast), float64(p.slow), float64(p.passive)
}

// FrozenCarbon is the carbon in the permafrost under tile i's top metre, down
// to carbonDeep, in kg a square metre.
func (g *Grid) FrozenCarbon(i int) float64 {
	if i < 0 || i >= len(g.pools) {
		return 0
	}
	return float64(g.pools[i].frozen)
}

// SoilTurnover is how long tile i's soil and litter take to turn their carbon
// over, in years: what they hold over what goes into them in a year, which is
// what they breathe out a year at the steady state. It is nothing where
// nothing goes in.
func (g *Grid) SoilTurnover(i int) float64 {
	if i < 0 || i >= len(g.pools) || !forms(&g.Tiles[i]) {
		return 0
	}
	c := g.pedoClimateOf(i)
	in := g.inputOf(i, c, g.coverOf(i, c.temp))
	if in.litter <= 0 {
		return 0
	}
	p := g.pools[i]
	return float64(p.litter+p.fast+p.slow+p.passive) / in.litter
}

// CarbonStocks is the carbon a land holds, pool by pool, in kg, over Area
// square metres of it.
type CarbonStocks struct {
	Vegetation, Litter, Fast, Slow, Passive, Peat, Permafrost float64
	// Fire is what the land's fires send to the air a year off its
	// vegetation and its litter, kg C a year.
	Fire float64
	Area float64
}

// Soil is the soil's organic carbon in the top metre: the fast, slow and
// passive pools.
func (s CarbonStocks) Soil() float64 { return s.Fast + s.Slow + s.Passive }

// Total is all of it.
func (s CarbonStocks) Total() float64 {
	return s.Vegetation + s.Litter + s.Soil() + s.Peat + s.Permafrost
}

// LandCarbon is the carbon g's dry land holds, pool by pool, each tile
// weighed by the ground it stands for: what the land has drawn out of the air
// and keeps, for a step of deep time to book against the ocean's and the
// rock's. It reads what has been laid; a map whose vegetation and soil have
// not been holds nothing.
func (g *Grid) LandCarbon() CarbonStocks {
	var s CarbonStocks
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if t.Wet() || t.Terrain.Tidal() || g.sunk(i) {
			continue
		}
		area := g.tileArea(i)
		s.Area += area
		s.Vegetation += area * g.VegCarbon(i)
		s.Peat += area * g.PeatCarbon(i)
		if i < len(g.pools) {
			p := g.pools[i]
			s.Litter += area * float64(p.litter)
			s.Fast += area * float64(p.fast)
			s.Slow += area * float64(p.slow)
			s.Passive += area * float64(p.passive)
			s.Permafrost += area * float64(p.frozen)
		}
		if forms(t) && g.vegLaid() && len(g.npp) == len(g.Tiles) {
			c := g.pedoClimateOf(i)
			in := g.inputOf(i, c, g.coverOf(i, c.temp))
			s.Fire += area * (in.fire + in.burned*burnHerb*in.surface*g.Litter(i))
		}
	}
	return s
}

// tileArea is the ground tile i stands for, in square metres: a piece of the
// planet on a globe, and a tile's span squared on a valley.
func (g *Grid) tileArea(i int) float64 {
	y := i / g.W
	if g.air != nil && y < len(g.air.Dx) {
		return g.air.Dx[y] * g.air.Dy * 1e6
	}
	s := g.span()
	return s * s
}
