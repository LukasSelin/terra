package terra

import (
	"math"
	"math/cmplx"
	"slices"

	"github.com/LukasSelin/terra/geom"
)

// The shore: how big the tide is on each coast, and the ground it covers and
// uncovers.
//
// The tide the moon raises in the open ocean is well under a metre. What makes
// one coast's tide ten metres and another's nothing is the shape of the water
// it comes into. A tide is a long wave, and a long wave that runs into a
// channel that narrows or shoals keeps its flux of energy, so it rises: Green's
// law (Green 1838; Lamb 1932), the height going as the width of the channel to
// the minus a half and its depth to the minus a quarter. One that has to find
// its way into a sea through a strait arrives spent, because what comes through
// the strait is all there is to spread over the sea behind it. A basin a
// quarter of the tide's wavelength long rings with it, the wave reflected off
// its head coming back in step - the Bay of Fundy's sixteen metres. And the bed
// takes some of all of it, the more the shallower the water.
//
// Taken at its word that is little, on a map here. The tide is a wave hundreds
// of kilometres long in water tens of metres deep, and the longest bay on the
// largest map is a few kilometres, so nothing here rings: kL is a few
// hundredths, and the reflection is worth a part in a thousand. What a bay on a
// map this size does to the tide is squeeze it, and that is all that comes out.
//
// Where the tide reaches ground between its lowest low water and its highest
// high, that ground is a tidal flat, if the tide and not the waves shape it and
// it lies no steeper than the sea grades such ground. It is laid once an age,
// when the coast is, and never changes from day to day - the day is asked
// instead. See Covered.

// TideFactorMax is how many times the open ocean's tide the most gathered tide
// on the planet is, as a scale to draw ranges against: the Bay of Fundy's
// spring range of sixteen metres against the open ocean's here. Nothing holds a
// coast to it; see tidalReach.
const TideFactorMax = 16 / (2 * (TideM2 + TideS2))

// m2Period is how long the moon's semidiurnal tide takes, in seconds: 12.4206
// hours (Doodson 1921). tideOmega is its angular frequency, ω, which turns a
// length of channel into a phase of the tide.
const (
	m2Period  = 12.4206012 * 3600
	tideOmega = 2 * math.Pi / m2Period
)

// bedDrag is c_D, the drag of a sandy or muddy bed on the water over it, as the
// stress over ρU²: 0.0025 (Soulsby 1997).
const bedDrag = 0.0025

// tideDominated is the relative tide range past which the tide and not the waves
// shapes the ground between low water and high: Masselink and Short's (1993)
// RTR, the spring range over the breaker height. Under three a shore is a beach;
// over it the ground the tide uncovers is terraces and flats, and over fifteen
// flats alone.
const tideDominated = 3.0

// flatTide is how far from mean sea a flat reaches, at an open coast.
const flatTide = TideM2 + TideS2

// The shoreface. Dean's (1977) equilibrium profile has the sea deepen with the
// distance y from the water's edge as h = A·y^(2/3), which is the profile over
// which breaking waves spend their energy evenly on every metre of bed, and
// Moore's (1982) fit of A to the grain, as Dean (1987) gives it, is
// A = 0.21·D^0.48, with D in millimetres and A in m^(1/3). Fine ground is graded
// gentler than coarse. The fit is to sands, and ground finer than deanFinest is
// read at it.
const (
	deanCoeff  = 0.21
	deanPower  = 0.48
	deanFinest = 0.063 // mm: where sand ends and silt begins
)

// grainSize is how big each grain is, in millimetres: what fallSpeed was read
// at.
var grainSize = [Grains]float64{Sand: 0.13, Silt: 0.01, Clay: 0.001}

// deanSlope is how steeply the equilibrium profile of ground whose middling
// grain is d millimetres falls at y metres from the water's edge:
// dh/dy = (2/3)·A·y^(−1/3). Ground no steeper than this is ground the sea has
// graded, and ground steeper is ground it has not yet, or cannot: rock, or a
// shore whose own ground is steeper than the sea could lay it.
func deanSlope(d, y float64) float64 {
	a := deanCoeff * math.Pow(math.Max(d, deanFinest), deanPower)
	return 2.0 / 3 * a / math.Cbrt(y)
}

// medianGrain is the middling grain of tile i's ground, in millimetres: the
// three sizes' logarithms, weighed by how much of each there is in its soil, or
// in the soil its rock makes where it has none.
func (g *Grid) medianGrain(i int) float64 {
	t := &g.Tiles[i]
	share := parts(t)
	if t.Soil <= 0 {
		sand, clay := g.TextureAt(g.PosOf(i))
		share = [Grains]float64{Sand: sand, Silt: clamp01(1 - sand - clay), Clay: clay}
	}
	l := 0.0
	for gr := range share {
		l += share[gr] * math.Log(grainSize[gr])
	}
	return math.Exp(l)
}

// tides reads the tide's reach off the coast as it now lies, and lays the flats
// it covers and uncovers. It runs when the land is made and after every age of
// weather, because the weather moves the coast. A map with no sea has no tide,
// and nothing here touches it.
//
// A flat is ground within an ordinary spring's reach of mean sea, where the
// tide shapes it rather than the waves - see tideDominated - lying no steeper
// than Dean's profile of its grain at its distance from the water's edge. It
// was ground no steeper than one in a hundred where the spring range was a
// metre and a half or more, whatever it was made of or the sea did to it; at a
// hundred metres from the water, Dean's slope for mud is one in a hundred and
// twenty.
//
// It draws no chance and writes nothing but the tiles it turns, so it may run
// anywhere in the making of a world without moving a seed.
func (g *Grid) tides() {
	defer phase("tides")()
	if g.sea < 0 {
		g.tidal, g.ebb = nil, nil
		return
	}
	n := len(g.Tiles)
	f := g.tidalReach()

	// Where a river meets the sea, its channel is kept open through the mud:
	// a creek. Without one a river ends at the top of its own flats and the
	// water it carries has nowhere drawn to go.
	creek := make([]bool, n)
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if t.Terrain != Water || g.underSea(i) || f[i] <= 0 || t.Flow < settleFlow {
			continue
		}
		for j, steps := i, 0; steps < g.W; steps++ {
			p := g.PosOf(j)
			a := g.Aspect(p)
			q := geom.Pos{X: p.X + a.X, Y: p.Y + a.Y}
			if a == (geom.Pos{}) || !g.In(q) {
				break
			}
			j = g.Index(q)
			if !g.underSea(j) {
				continue
			}
			if g.Height[j] <= g.sea-float64(f[j])*flatTide || creek[j] {
				break
			}
			creek[j] = true
		}
	}

	s := g.surfOf(nil)
	shore, near := g.fromShore(s)
	if len(g.ebb) != n {
		g.ebb = make([]float32, n)
	}
	for i := range g.Tiles {
		t := &g.Tiles[i]
		g.ebb[i] = 0
		reach := float64(f[i]) * flatTide
		band := f[i] > 0 && g.Height[i] > g.sea-reach && g.Height[i] <= g.sea+reach &&
			g.Slope(g.PosOf(i)) <= deanSlope(g.medianGrain(i), shore[i])
		if c := near[i]; band && c >= 0 && s.breaker[c] > 0 {
			band = 2*reach/s.breaker[c] >= tideDominated
		}
		held := t.Mark != None || t.Owner != 0
		under := g.underSea(i)
		turns := false
		switch t.Terrain {
		case Grass, Forest, Rock, Flat:
			turns = !g.Frozen(g.PosOf(i))
		case Water:
			turns = under && !creek[i]
		}
		switch {
		case band && turns && !held:
			if t.Terrain != Flat {
				t.Terrain, t.Fenced = Flat, false
				g.Wood[i], g.Wild[i], g.Age[i], g.Fish[i] = 0, 0, 0, 0
				g.Fertility[i], g.Rich[i] = 0, 0
			}
			g.ebb[i] = float32((g.sea - g.Height[i]) / float64(f[i]))
		case t.Terrain == Flat && !band:
			if under {
				t.Terrain = Water // with no fish yet; they come back as water's do
			} else {
				t.Terrain = Grass
			}
		}
	}
	g.tidal = f
}

// fromShore is, for every tile, how far it lies from the water's edge, in
// metres, and the cell of the surf that faces the stretch of edge nearest it,
// or -1 where none does. The edge runs between the sea and the ground, so the
// tiles either side of it lie half a tile from it.
func (g *Grid) fromShore(s *surf) ([]float64, []int32) {
	n := len(g.Tiles)
	dist := make([]float64, n)
	near := make([]int32, n)
	span := g.span()
	var by []int32
	if len(s.cells) > 0 {
		by = g.facing(s)
	}
	queue := make([]int32, 0, n/8)
	for i := range g.Tiles {
		dist[i], near[i] = math.Inf(1), -1
		p := g.PosOf(i)
		under := g.underSea(i)
		for _, d := range Dirs {
			q := g.Norm(geom.Pos{X: p.X + d.X, Y: p.Y + d.Y})
			if g.In(q) && g.underSea(g.Index(q)) != under {
				dist[i] = span / 2
				if by != nil {
					near[i] = by[i]
				}
				queue = append(queue, int32(i))
				break
			}
		}
	}
	for k := 0; k < len(queue); k++ {
		i := int(queue[k])
		p := g.PosOf(i)
		for _, d := range Dirs {
			q := g.Norm(geom.Pos{X: p.X + d.X, Y: p.Y + d.Y})
			if !g.In(q) {
				continue
			}
			j := g.Index(q)
			if !math.IsInf(dist[j], 1) {
				continue
			}
			dist[j], near[j] = dist[i]+span*math.Hypot(float64(d.X), float64(d.Y)), near[i]
			queue = append(queue, int32(j))
		}
	}
	return dist, near
}

// rossbyWidth is the width, in metres, past which water is open to the tide
// rather than a channel it runs up: the Rossby radius of deformation, √(gh)/f,
// of a shelf fifty metres deep at forty-five degrees, some two hundred
// kilometres. Water narrower than that holds a long wave from wall to wall.
const rossbyWidth = 200 * km

// chords are the four ways across the grid, each as the two of Dirs that go
// along it: east and west, south and north, and the two diagonals.
var chords = [4][2]int{{4, 3}, {6, 1}, {7, 0}, {5, 2}}

// tidalReach is, for every tile, how many times the open ocean's tide it has:
// nothing where the tide does not come, one on open water, and more or less up
// a channel as the channel gathers it or spends it. The sea has it, the ground
// the tide covers has it, and a river has it up its channel until its bed
// stands above the highest water - the tidal limit, which is where an estuary
// stops being one.
//
// The ocean is the water the tide comes in off, and a map's ocean is its large
// bodies of sea - a quarter the size of all its sea or more. A sea with no way
// to one has only the tide its own few kilometres raise, which is nothing to
// speak of. On an ocean the tide is the open ocean's over its widest water, or
// over all of it that is wider than rossbyWidth: water narrower than that holds
// the tide from wall to wall as a channel does, and the rest of the water is
// channels off its widest, which have the open coast's tide where they open off
// it.
//
// Up the channels the tide is the long wave's, solved. From the open water the
// tide reaches every tile it can from one it has already reached, first in tile
// order, and that makes a tree of channels whose every branch ends at a head.
// Along it the height η of the tide is (Lamb 1932, for a channel of width b
// and depth h, with linearised friction r):
//
//	(b·h·η′ / (1 − i·r/ω))′ + (ω²/g)·b·η = 0
//
// the water's flux through each section being what fills everything beyond it,
// and the slope of the water what drives that flux against its own inertia and
// the bed. Its two limits are the laws the tide is usually told by. Where a
// channel changes slowly over a wavelength its height goes by Green's law,
// b^(−½)·h^(−¼); and a uniform channel shut at its head, a length L up it, has
// cos k(L−x)/cos kL of the tide at its mouth, k = ω/√(gh) - the quarter-wave
// resonance, 1/|cos kL| at its head. On a bay a few kilometres long, kL is a few
// hundredths, and the whole bay rises and falls with the sea at its mouth, very
// nearly: neither law has the length to act.
//
// The solve is exact on the tree, a pass from the heads down and one back up:
// each tile's tide is a share α of its parent's, α = C/(C + Σ C_child·(1 −
// α_child) − (ω²/g)·A), with A the tile's area and C = b·h/(Δx·(1 − i·r/ω)) the
// section it is reached through, at the narrower and shallower of the two tiles
// either side of it. The width is the water's narrowest chord of the four ways
// across the grid, a tile's where it is not sea, and the depth what stands on the
// tile at an ordinary high water. The friction is Lorentz's (1926), r =
// 8·c_D·U/3πh, at the speed the spring tide's prism of everything beyond a
// section comes through it at: U = ω·a·(area beyond)/(b·h).
//
// The tide goes no further than ground standing above the highest water the open
// ocean's tide could bring, and at the end no tile has it that stands above the
// highest water its own tide brings, nor any tile beyond one that does.
func (g *Grid) tidalReach() []float32 {
	n := len(g.Tiles)
	span := g.span()
	f := make([]float32, n)

	// The narrowest chord across the water through each tile of sea, and the
	// widest on each body of it. What is not sea is a tile wide.
	fetch := g.fetches()
	body, sizes := g.seaBodies()
	width := make([]float64, n)
	widest := make([]float64, len(sizes))
	for i := range g.Tiles {
		width[i] = span
		if body[i] < 0 || !g.seaCell(i) {
			continue
		}
		w := math.Inf(1)
		for _, ax := range chords {
			d := Dirs[ax[0]]
			across := float64(fetch[ax[0]][i]) + float64(fetch[ax[1]][i]) + span*math.Hypot(float64(d.X), float64(d.Y))
			w = math.Min(w, across)
		}
		width[i] = math.Min(w, rossbyWidth)
		widest[body[i]] = math.Max(widest[body[i]], width[i])
	}
	total := 0
	for _, size := range sizes {
		total += size
	}
	depth := func(i int) float64 {
		return math.Max(0, g.sea-g.Height[i]) + MeanHigh
	}

	// The tree of channels off the open water.
	open := make([]bool, n)
	reached := make([]bool, n)
	from := make([]int32, n)
	run := make([]float64, n)
	queue := make([]int32, 0, n/4)
	for i := range g.Tiles {
		from[i] = -1
		b := body[i]
		if b < 0 || 4*sizes[b] < total || !g.seaCell(i) || g.Freezing(g.PosOf(i)) {
			continue
		}
		if width[i] >= widest[b] {
			open[i], reached[i] = true, true
			queue = append(queue, int32(i))
		}
	}
	for k := 0; k < len(queue); k++ {
		i := queue[k]
		p := g.PosOf(int(i))
		for _, off := range Dirs {
			q := g.Norm(geom.Pos{X: p.X + off.X, Y: p.Y + off.Y})
			if !g.In(q) {
				continue
			}
			j := g.Index(q)
			if reached[j] {
				continue
			}
			t := &g.Tiles[j]
			if t.Terrain == Ice || (t.Wet() && g.Freezing(q)) || g.Height[j] > g.sea+TideMax {
				continue
			}
			reached[j], from[j] = true, i
			run[j] = span * math.Hypot(float64(off.X), float64(off.Y))
			queue = append(queue, int32(j))
		}
	}

	// From the heads down: the area beyond each section, and the share of its
	// parent's tide each tile has.
	area := make([]float64, n)
	load := make([]complex128, n)
	share := make([]complex128, n)
	storage := complex(tideOmega*tideOmega/gravity*span*span, 0)
	for _, i := range queue {
		area[i] = span * span
	}
	for k := len(queue) - 1; k >= 0; k-- {
		j := queue[k]
		if open[j] {
			continue
		}
		p := from[j]
		b := math.Min(width[j], width[p])
		h := math.Min(depth(int(j)), depth(int(p)))
		u := tideOmega * flatTide * area[j] / (b * h)
		r := 8 * bedDrag * u / (3 * math.Pi * h)
		c := complex(b*h/run[j], 0) / complex(1, -r/tideOmega)
		share[j] = c / (c + load[j] - storage)
		if !open[p] {
			load[p] += c * (1 - share[j])
			area[p] += area[j]
		}
	}
	// And from the open water up.
	tide := make([]complex128, n)
	for _, i := range queue {
		if open[i] {
			tide[i], f[i] = 1, 1
			continue
		}
		p := from[i]
		if f[p] <= 0 {
			continue // cut off below
		}
		tide[i] = share[i] * tide[p]
		got := cmplx.Abs(tide[i])
		if g.Height[i] > g.sea+got*TideMax {
			continue
		}
		f[i] = float32(got)
	}
	return f
}

// seaBodies labels each connected body of sea, eight ways round and across the
// seam, in tile order, and says how many tiles each is. Tiles that are not sea
// get -1.
func (g *Grid) seaBodies() ([]int32, []int) {
	n := len(g.Tiles)
	body := make([]int32, n)
	for i := range body {
		body[i] = -1
	}
	var sizes []int
	var stack []int32
	for i := range g.Tiles {
		if body[i] >= 0 || !g.underSea(i) {
			continue
		}
		b := int32(len(sizes))
		size := 0
		body[i] = b
		stack = append(stack[:0], int32(i))
		for len(stack) > 0 {
			j := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			size++
			p := g.PosOf(int(j))
			for _, off := range Dirs {
				q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
				if !g.In(q) {
					continue
				}
				k := g.Index(q)
				if body[k] < 0 && g.underSea(k) {
					body[k] = b
					stack = append(stack, int32(k))
				}
			}
		}
		sizes = append(sizes, size)
	}
	return body, sizes
}

// TidalRange is the spring range at p, in metres: how far the water falls from
// the highest high water of an ordinary spring tide to its lowest low. It is
// nothing where the tide does not come.
func (g *Grid) TidalRange(p geom.Pos) float64 {
	if len(g.tidal) != len(g.Tiles) || !g.In(p) {
		return 0
	}
	return 2 * float64(g.tidal[g.Index(p)]) * (TideM2 + TideS2)
}

// HighWater and LowWater are how high the day's tide comes at p and how low it
// falls, as heights on the map. Where the tide does not come, both are the
// sea's level, or below the map where there is no sea.
func (g *Grid) HighWater(p geom.Pos) float64 { return g.waterAt(p, g.tide.High) }

// LowWater is how low the day's tide falls at p. See HighWater.
func (g *Grid) LowWater(p geom.Pos) float64 { return g.waterAt(p, g.tide.Low) }

func (g *Grid) waterAt(p geom.Pos, h float64) float64 {
	if len(g.tidal) != len(g.Tiles) || !g.In(p) {
		return g.sea
	}
	return g.sea + float64(g.tidal[g.Index(p)])*h
}

// Covered reports whether the day's sea lies over the flat at p even at low
// water: whether, today, there is no walking across it. A flat the tide leaves
// at low water is open, however high it came in the morning - a day is the
// smallest thing there is, and somebody who can cross at low water does.
// Anything built on a flat - a causeway - stands above the water every day, as
// a bridge stands above a river.
func (g *Grid) Covered(p geom.Pos) bool {
	if len(g.ebb) != len(g.Tiles) || !g.In(p) {
		return false
	}
	i := g.Index(p)
	return g.covered(i, &g.Tiles[i])
}

func (g *Grid) covered(i int, t *Tile) bool {
	return t.Terrain == Flat && t.Mark == None && len(g.ebb) == len(g.Tiles) && float64(g.ebb[i]) >= g.tide.High
}

// shut is Shut by index, for the search.
func (g *Grid) shut(i int) bool {
	t := &g.Tiles[i]
	return t.Deep() || g.covered(i, t)
}

// Shut reports whether a laden walker is kept off p today: open water, or a
// flat the day's tide covers. See Tile.Deep, which is the same question asked
// of the water alone, without the day.
func (g *Grid) Shut(p geom.Pos) bool {
	if !g.In(p) {
		return false
	}
	i := g.Index(p)
	return g.Tiles[i].Deep() || g.covered(i, &g.Tiles[i])
}

// How the tide lays mud on a flat. The water that floods a flat stands over it
// through the turn of the tide, and what it carries falls out of it as fast as
// it falls through it, while the flow over the bed is too slow to keep it up:
// Krone's (1962) deposition, D = w_s·C·(1 − τ_b/τ_cd), for τ_b under τ_cd and
// nothing over it.
//
// The fall is Stokes's, w_s = (ρ_s − ρ)·g·D²/18μ. In salt water silt and clay
// do not fall as grains: they clot into flocs, open aggregates far larger than
// the grains and far lighter than the rock they are made of, and a floc falls
// as its size times its grains' (Winterwerp 1998, for flocs of fractal
// dimension two, (ρ_s − ρ)·g·D_p·D_f / 18μ). flocSize is D_f: estuarine flocs are
// from tens of microns to half a millimetre (Manning and Dyer 1999), and a
// tenth of one is taken. So clay falls at a hundred times its grain's speed, and
// silt at ten; sand falls as itself.
//
// τ_cd is the stress under which a floc that reaches the bed stays there:
// Krone's 0.06 to 0.08 pascals for the muds of San Francisco Bay. Sand stays
// where the flow cannot move it again, under its threshold for motion.
//
// The flow over a flat is what fills the flat behind it: the water rising at
// ω·a over the width of flat between the tile and high water, (a − z)/S on a
// flat of slope S, has to come across the tile, at a depth of what stands on it.
//
// It was a share for each grain, 0.62 for sand, 0.60 for silt and 0.45 for
// clay, of what passed, times the spring range over four metres: a coast whose
// tide hardly moves was hardly a trap for anything. A flat whose tide hardly
// moves is now one the water hardly stands on.
const (
	flocSize     = 0.1 * mm
	seaViscosity = 1.08e-3 // μ, Pa·s: sea water at twenty degrees
	kroneStress  = 0.07 * pascal
	// sandStress is the threshold for motion of fine sand, 0.15 Pa (Soulsby
	// 1997, off Shields's curve at 0.13 mm).
	sandStress = 0.15 * pascal
)

// tidalFall is how fast each grain falls through salt water, in metres a second.
var tidalFall = func() (w [Grains]float64) {
	for gr := range w {
		w[gr] = fallSpeed[gr]
		if Grain(gr) != Sand {
			floc := (grainDensity - seaDensity) * gravity * grainSize[gr] * mm * flocSize / (18 * seaViscosity)
			w[gr] = math.Max(w[gr], floc)
		}
	}
	return w
}()

// depositStress is τ_cd for each grain.
var depositStress = [Grains]float64{Sand: sandStress, Silt: kroneStress, Clay: kroneStress}

// flatShare is, for each grain, the share of what the water over flat tile i
// carries that an ordinary tide lays on it, where the tide is f times the open
// ocean's: 1 − exp(−w_s·P·t/h), with t the time a tide stands over the flat, h
// how deep it stands on average while it does, and P Krone's 1 − τ_b/τ_cd over
// the tide, the flow going as the sine of the tide's phase.
func (g *Grid) flatShare(i int, f float64) [Grains]float64 {
	var out [Grains]float64
	a := f * MeanHigh
	z := g.Height[i] - g.sea
	if a <= 0 || z >= a {
		return out
	}
	half := math.Acos(math.Max(-1, z/a)) // half the phase of a tide the flat is under
	depth := a*math.Sin(half)/half - z
	if depth <= 0 {
		return out
	}
	slope := math.Max(g.Slope(g.PosOf(i)), leastFall)
	flow := tideOmega * a * (a - z) / slope / depth
	under := half / math.Pi * m2Period
	for gr := range out {
		still := math.Sqrt(depositStress[gr] / (seaDensity * bedDrag))
		out[gr] = -math.Expm1(-tidalFall[gr] * kroneMean(flow/still) * under / depth)
	}
	return out
}

// kroneMean is the mean over a tide of Krone's max(0, 1 − τ/τ_cd) for a flow
// whose stress is r² times τ_cd at its strongest: of max(0, 1 − r²·sin²θ).
func kroneMean(r float64) float64 {
	if r <= 1 {
		return 1 - r*r/2
	}
	th := math.Asin(1 / r)
	return 2 / math.Pi * (th - r*r*(th/2-math.Sin(2*th)/4))
}

// tideWork sets how the tide works the ground this age into the water's step:
// that no river is cut below the tide's high water where the tide reaches it,
// that flats catch what the water brings them - see flatShare - and that a flat
// under mean sea keeps what reaches it until it stands at high water. On a map
// with no tide it does nothing, and the water's step is what it was.
//
// What is left out is the tide's own scour: the water that fills and empties a
// bay twice a day keeps the channel it comes through open, and deeper the more
// of the bay there is to fill. Nothing here cuts a channel for the tide; a
// creek is only kept open where a river already runs.
func (g *Grid) tideWork(c *fluvial, recv []int32) {
	if len(g.tidal) != len(g.Tiles) || g.sea < 0 {
		return
	}
	n := len(g.Tiles)
	c.floor = make([]float64, n)
	c.keep = make([]float64, n)
	c.room = make([]float64, n)
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			f := float64(g.tidal[i])
			c.floor[i] = math.Inf(-1)
			if f <= 0 {
				continue
			}
			high := g.sea + f*MeanHigh
			c.floor[i] = high
			t := &g.Tiles[i]
			if t.Terrain != Flat || t.Mark != None {
				continue
			}
			share := g.flatShare(i, f)
			if int(recv[i]) == i {
				// What a root keeps it keeps whole, and silt is the middle of it.
				c.keep[i] = share[Silt]
				c.room[i] = math.Max(0, high-g.Height[i])
				continue
			}
			for gr := range share {
				c.settle[i][gr] = math.Max(c.settle[i][gr], share[gr])
			}
		}
	})
	g.bays(c, recv)
}

// siltYears is how long the rivers have been bringing mud down to the coast
// when a world is made, and siltRounds how many times over the tide's reach is
// read again in the course of it. The seas of the earth stopped rising six to
// seven thousand years ago, and the flats and the deltas that stand on its
// coasts today are what has built up since (Stanley and Warne 1994).
const (
	siltYears  = 6000 * yr
	siltRounds = 6
)

// silt lays siltYears of the rivers' mud on the shoals, and nothing else: the
// land is left as its shaping graded it and its valleys were cut, because
// cutting them longer spoils them - see valleyYears - and only what settles in
// the tidal water is kept of each round. The flats are then read again on the
// coast the mud has made. The mud takes up room under the sea, so level is
// called after each round to find the sea's level again, for the same water.
func (g *Grid) silt(level func()) {
	defer phase("silt")()
	if g.sea < 0 {
		return
	}
	n := len(g.Tiles)
	for range siltRounds {
		if len(g.tidal) != n {
			return
		}
		c := g.waterStep(siltYears / siltRounds)
		if c.bay == nil {
			return
		}
		next := c.solve(settleIters)
		change := make([]float64, n)
		gained := make([][Grains]float64, n)
		c.account(next, change, gained, func(int32, [Grains]float64) {})
		for b := range c.shoal {
			for _, i := range c.shoal[b] {
				t := &g.Tiles[i]
				g.Height[i] += change[i]
				mix(t, float64(t.Soil), gained[i])
				t.Soil += float32(carrying(gained[i]))
			}
		}
		level()
		g.expose()
		g.drain()
		g.height()
		g.tides()
	}
}

// Mud from the rivers. A flat is not a shape the coast happens to have: it is
// the mud a river brings down, carried back and forth by the tide until it
// finds slack water shallow enough to let it fall, and built up there, a few
// millimetres a year, until the ground stands at the high water that covers it
// and no higher (Krone 1962; Friedrichs 2011 has flats' surfaces tracking high
// water as they build). A flat under mean sea keeping only what reached it
// down its own river laid a tongue of mud one tile wide out into deep water, and
// a tongue is not a flat: so what reaches the sea is carried by the tide over
// all the shoals of the body of water it reaches - the tide's excursion is
// kilometres and the map's bays are not - and settles over them by their
// area. Mixed through the deep water as well, the shoals of a small globe got
// a millimetre in a thousand years; most of what the earth's rivers carry is
// kept on its shelves and coasts (Milliman and Syvitski 1992), and what the
// shoals do not trap goes on to the deep water.
//
// A shoal is sea shallower than a spring tide's range below mean sea: the
// water that runs slack over it at the turn of the tide, which the deep water
// never does. What falls into the deep water goes on to the sea.
func (g *Grid) bays(c *fluvial, recv []int32) {
	n := len(g.Tiles)
	in := func(i int) bool {
		return int(recv[i]) == i && g.underSea(i) && g.tidal[i] > 0
	}
	c.bay = make([]int32, n)
	c.trap = make([][Grains]float64, n)
	for i := range c.bay {
		c.bay[i] = -1
	}
	c.shoal = nil
	var stack []int
	for start := range n {
		if c.bay[start] >= 0 || !in(start) {
			continue
		}
		b := int32(len(c.shoal))
		var shoal []int32
		c.bay[start] = b
		stack = append(stack[:0], start)
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			t := &g.Tiles[i]
			f := float64(g.tidal[i])
			spring := 2 * f * (TideM2 + TideS2)
			if t.Mark == None && g.sea-g.Height[i] <= spring && !g.Frozen(g.PosOf(i)) {
				c.trap[i] = g.flatShare(i, f)
				c.room[i] = math.Max(0, g.sea+f*MeanHigh-g.Height[i])
				if carrying(c.trap[i]) > 0 && c.room[i] > 0 {
					shoal = append(shoal, int32(i))
				}
			}
			g.eachNear(i, func(j int) {
				if c.bay[j] < 0 && in(j) {
					c.bay[j] = b
					stack = append(stack, j)
				}
			})
		}
		slices.Sort(shoal) // the order a world repeats in
		c.shoal = append(c.shoal, shoal)
	}
}
