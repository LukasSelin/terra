package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
)

// Waves: the wind's work on a coast.
//
// A wave is the wind's energy put into the water over the distance it blows
// across it, the fetch, and taken out again where the wave breaks. How big the
// sea gets is a matter of how hard the wind blows and for how far: a gale over
// a pond raises a chop, the same gale over an ocean a swell that breaks a
// harbour wall. What a coast is made of - sand or shingle, beach or cliff, mud
// that stays or mud that is carried off - is decided by how much of that energy
// arrives on it, and from which way.
//
// So each tile of sea along a coast reads the wind over it in each phase of the
// year, the open water upwind of it along that wind, and the sea that wind
// raises there: its height and its period, what height it breaks at, and at
// what angle to the shore. From that come the sand the breakers drive along the
// shore (see coast.go), the cliffs they cut back and the fines they will not let
// settle.
//
// What is left out is the sea's own depth over the fetch. The seas of a map
// here are a few tens of metres deep at most, and a wave in water shallower
// than half its length is slowed and grows less than it would offshore (the
// Shore Protection Manual's shallow-water forms, 1984). A wave of the few
// seconds these fetches raise is some metres long, so over all but the
// shallowest of these seas it is a deep-water wave, and the forms here are.

// Water, sand and the breaking of a wave.
const (
	// grainDensity is quartz's density, ρ_s, in kilograms a cubic metre; the
	// water's, ρ, is seaDensity.
	grainDensity = 2650.0
	// sandPorosity is n, the share of a bed of sand that is the water between
	// its grains (0.4 for a natural sand, as the CERC formula takes it).
	sandPorosity = 0.4
	// breakerIndex is γ, the height a wave breaks at against the depth it
	// breaks in: 0.78, McCowan's (1894) solitary wave.
	breakerIndex = 0.78
)

// How the wind raises a sea. A wave grows with the fetch until it is as big as
// that wind can make it, and then grows no more: Hasselmann and others' (1973)
// JONSWAP growth, gH/U² = 0.0016·(gF/U²)^½ and gT/U = 0.2857·(gF/U²)^⅓, up to
// Pierson and Moskowitz's (1964) fully developed sea, gH/U² = 0.2433 and
// gT/U = 8.134 - both as the Shore Protection Manual (1984) writes them. H is
// the significant height, the mean of the highest third of the waves, and T
// the period at the peak of the spectrum.
const (
	growHeight = 0.0016
	growPeriod = 0.2857
	fullHeight = 0.2433
	fullPeriod = 8.134
)

// openFetch is the fetch, in metres, of water that runs off the map or all the
// way round it: more than any wind here raises a sea over before the sea is as
// big as that wind makes it, which at twenty metres a second is about four
// hundred kilometres.
const openFetch = 1000 * km

// windSpeeds is how many speeds the wind in each phase of the year is read at.
// The wind is not its mean: it blows at every speed about it, and the waves go
// as a power of the speed, so a coast is worked mostly by the few days it blows
// hardest. Over a season the speed of the wind near the ground is close to a
// Rayleigh distribution, a Weibull of shape two (Justus and others 1978), and it
// is read at the middles of that many equally likely slices of it.
const windSpeeds = 8

// rayleighSpeeds is the speed at the middle of each slice, as a multiple of the
// mean: the Rayleigh distribution's quantile, √(−(4/π)·ln(1−p)).
var rayleighSpeeds = func() (s [windSpeeds]float64) {
	for q := range s {
		p := (float64(q) + 0.5) / windSpeeds
		s[q] = math.Sqrt(-4 / math.Pi * math.Log(1-p))
	}
	return s
}()

// stormShare is the share of the year Hallermeier's (1978) effective wave is
// exceeded for: twelve hours of it. The depth that wave reaches the bed at is
// how deep a coast's sand is moved at all. See closure.
const stormShare = 12.0 / (365.25 * 24)

// stormSpeed is the wind, as a multiple of its mean, exceeded for stormShare of
// the year.
var stormSpeed = math.Sqrt(-4 / math.Pi * math.Log(stormShare))

// waveOf is the sea wind blowing at u metres a second raises over fetch f
// metres: its significant height and its peak period.
func waveOf(u, f float64) (h, period float64) {
	if u <= 0 || f <= 0 {
		return 0, 0
	}
	x := gravity * f / (u * u)
	h = u * u / gravity * math.Min(growHeight*math.Sqrt(x), fullHeight)
	period = u / gravity * math.Min(growPeriod*math.Cbrt(x), fullPeriod)
	return h, period
}

// breaker is the height, in metres, a wave h high and period seconds long in
// deep water breaks at: Komar and Gaughan's (1972) H_b = 0.39·g^⅕·(T·H₀²)^⅖,
// which is linear shoaling to the breakerIndex taken as a power law.
func breaker(h, period float64) float64 {
	if h <= 0 || period <= 0 {
		return 0
	}
	return 0.39 * math.Pow(gravity, 0.2) * math.Pow(period*h*h, 0.4)
}

// closure is the depth of closure, in metres, of a coast whose effective wave -
// the one exceeded twelve hours a year - is h high and period seconds long:
// Hallermeier's (1978) h_c = 2.28·H − 68.5·H²/(g·T²), the depth past which the
// waves do not move the sand from one year to the next. Sand carried past it is
// gone from the shore.
func closure(h, period float64) float64 {
	if h <= 0 || period <= 0 {
		return 0
	}
	return math.Max(0, 2.28*h-68.5*h*h/(gravity*period*period))
}

// The longshore transport of sand. The CERC formula (Shore Protection Manual
// 1984) has the sand carried along a beach go as the wave's energy flux along
// the shore at breaking:
//
//	Q = K·ρ·√(g/γ)·H_b^(5/2)·sin 2α_b / (16·(ρ_s − ρ)·(1 − n))
//
// in cubic metres of bed a second, with H_b the significant breaker height, α_b
// the angle the breaking crests make with the shore, and K 0.39 for significant
// heights (Komar and Inman's 0.77 is for the root-mean-square height).
const cercK = 0.39

// cercScale is everything in the CERC formula that is not the wave.
var cercScale = cercK * seaDensity * math.Sqrt(gravity/breakerIndex) /
	(16 * (grainDensity - seaDensity) * (1 - sandPorosity))

// surf is the wave climate along a coast as its ground now lies. Its cells are
// the tiles of open sea that touch ground, in tile order - the surf zone at the
// scale of a tile - and for each it keeps which way is out to sea, the waves
// that arrive, and what they do.
type surf struct {
	cells []int32
	// slot is each tile's place among the cells, and -1 for a tile that is not
	// one.
	slot []int32
	// out is the unit vector, in tiles east and south, pointing from the ground
	// to the sea.
	out [][2]float64
	// breaker is the breaker height over the year, in metres: the mean of
	// every wind's, the still ones and the ones blowing off the land counted as
	// nothing. storm is the breaker height of the effective wave, and period its
	// period, and closure the depth of closure that wave has.
	breaker, storm, period, closure []float64
	// drift is the sand the waves carry along the shore past each cell, in
	// cubic metres a second over the year, as a vector in tiles east and south.
	drift [][2]float64
	// attack is, for each rock and for soil after them, Sunamura's ln(F_w/F_r)
	// over the year - how hard the waves work a cliff of that stuff, the times
	// they are too weak to count as nothing. See cliffs.
	attack [][BedrockCount + 1]float64
}

// phaseWind is the wind near the ground over tile i in phase k of the year, in
// metres a second east and north.
func (g *Grid) phaseWind(i, k int) (east, north float64) {
	w := g.winds
	if w == nil {
		return 0, 0
	}
	fx, fy := w.cellAt(g, i)
	return w.sample32(w.u[k], fx, fy), w.sample32(w.v[k], fx, fy)
}

// seaCell reports whether tile i is open sea the wind raises waves on: sea, and
// not frozen over.
func (g *Grid) seaCell(i int) bool {
	return g.underSea(i) && g.Tiles[i].Terrain != Ice
}

// surfOf reads the waves along the coast of g under the winds it has, or under
// wind where that is given - wind(i, k) is the wind over tile i in phase k, east
// and north. A map with no sea, or with no wind, has no surf worth the name, and
// comes back with no cells.
func (g *Grid) surfOf(wind func(i, k int) (east, north float64)) *surf {
	s := &surf{}
	if g.sea < 0 || g.deep > 0 {
		return s
	}
	if wind == nil {
		if g.winds == nil {
			return s
		}
		wind = g.phaseWind
	}
	n := len(g.Tiles)
	s.slot = make([]int32, n)
	for i := range g.Tiles {
		s.slot[i] = -1
		if !g.seaCell(i) {
			continue
		}
		p := g.PosOf(i)
		for _, d := range Dirs {
			q := g.Norm(geom.Pos{X: p.X + d.X, Y: p.Y + d.Y})
			if g.In(q) && !g.underSea(g.Index(q)) {
				s.slot[i] = int32(len(s.cells))
				s.cells = append(s.cells, int32(i))
				break
			}
		}
	}
	if len(s.cells) == 0 {
		return s
	}
	fetch := g.fetches()
	m := len(s.cells)
	s.out = make([][2]float64, m)
	s.breaker = make([]float64, m)
	s.storm = make([]float64, m)
	s.period = make([]float64, m)
	s.closure = make([]float64, m)
	s.drift = make([][2]float64, m)
	s.attack = make([][BedrockCount + 1]float64, m)
	span := g.span()
	InParallel(m, WorkersFor(m), func(c, _ int) {
		i := int(s.cells[c])
		out := g.seaward(i)
		s.out[c] = out
		if out == ([2]float64{}) {
			return
		}
		along := [2]float64{-out[1], out[0]}
		var storm, stormT float64
		for k := range phases {
			east, north := wind(i, k)
			u := math.Hypot(east, north)
			if u < 1e-3 {
				continue
			}
			// The waves go the way the wind blows; on the map south is down.
			dir := [2]float64{east / u, -north / u}
			onshore := -(dir[0]*out[0] + dir[1]*out[1])
			if onshore <= 0 {
				continue // blowing off the land: no sea reaches this shore
			}
			f := effectiveFetch(fetch, i, dir, span)
			// The wave exceeded twelve hours a year, over the phases.
			if h, t := waveOf(u*stormSpeed, f); h > storm {
				storm, stormT = h, t
			}
			// The angle the waves come in at, off the line straight onto the
			// shore, signed toward along.
			a0 := math.Atan2(dir[0]*along[0]+dir[1]*along[1], onshore)
			for _, r := range rayleighSpeeds {
				h, t := waveOf(u*r, f)
				hb := breaker(h, t)
				if hb <= 0 {
					continue
				}
				// Refracted on the way in, by Snell's law between the deep
				// water's speed and the speed at the breaking depth.
				c0 := gravity * t / (2 * math.Pi)
				cb := math.Sqrt(gravity * hb / breakerIndex)
				ab := math.Asin(math.Max(-1, math.Min(1, math.Sin(a0)*cb/c0)))
				w := 1.0 / float64(phases*windSpeeds)
				s.breaker[c] += hb * w
				q := cercScale * math.Pow(hb, 2.5) * math.Sin(2*ab) * w
				s.drift[c][0] += q * along[0]
				s.drift[c][1] += q * along[1]
				force := seaDensity * gravity * hb
				for b := range s.attack[c] {
					if x := force / (cliffThreshold * strength(b)); x > 1 {
						s.attack[c][b] += math.Log(x) * w
					}
				}
			}
		}
		s.storm[c] = breaker(storm, stormT)
		s.period[c] = stormT
		s.closure[c] = closure(storm, stormT)
	})
	return s
}

// seaward is the unit vector from the ground to the sea at tile i, in tiles east
// and south: the pull of the sea against the ground over the five tiles either
// way of it, each tile weighed by how near it lies. It is nothing where the two
// balance.
func (g *Grid) seaward(i int) [2]float64 {
	const reach = 2
	p := g.PosOf(i)
	var x, y float64
	for dy := -reach; dy <= reach; dy++ {
		for dx := -reach; dx <= reach; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			q := g.Norm(geom.Pos{X: p.X + dx, Y: p.Y + dy})
			if !g.In(q) {
				continue
			}
			w := 1 / math.Hypot(float64(dx), float64(dy))
			if !g.underSea(g.Index(q)) {
				w = -w
			}
			d := math.Hypot(float64(dx), float64(dy))
			x += w * float64(dx) / d
			y += w * float64(dy) / d
		}
	}
	l := math.Hypot(x, y)
	if l < 1e-9 {
		return [2]float64{}
	}
	return [2]float64{x / l, y / l}
}

// fetches is, for each of the eight ways in Dirs and each tile, how far the
// open sea runs from the tile that way, in metres, not counting the tile. A
// valley's edge is open sea beyond it, because the sea goes on off the map; a
// globe's poles are not, because nothing does.
func (g *Grid) fetches() [8][]float32 {
	var out [8][]float32
	n := len(g.Tiles)
	span := g.span()
	for k, d := range Dirs {
		f := make([]float32, n)
		out[k] = f
		step := float32(span)
		if d.X != 0 && d.Y != 0 {
			step *= math.Sqrt2
		}
		// next is the fetch along d from the tile past (x, y), given the tile
		// past it has been read.
		next := func(x, y int) float32 {
			q := g.Norm(geom.Pos{X: x + d.X, Y: y + d.Y})
			if !g.In(q) {
				if g.Wrap {
					return 0
				}
				return openFetch
			}
			j := g.Index(q)
			if !g.seaCell(j) {
				return 0
			}
			return min(openFetch, step+f[j])
		}
		rows := make([]int, g.H)
		for y := range rows {
			rows[y] = y
			if d.Y > 0 {
				rows[y] = g.H - 1 - y
			}
		}
		for _, y := range rows {
			if d.Y != 0 {
				for x := 0; x < g.W; x++ {
					f[y*g.W+x] = next(x, y)
				}
				continue
			}
			// Along a row the tile past each is in the same row, so the row is
			// read from its far end back; round a globe, from a tile that is not
			// sea, and a row that is all sea is open all the way round.
			start := -1
			if !g.Wrap {
				start = g.W - 1
				if d.X < 0 {
					start = 0
				}
			} else {
				for x := 0; x < g.W; x++ {
					if !g.seaCell(y*g.W + x) {
						start = x
						break
					}
				}
				if start < 0 {
					for x := 0; x < g.W; x++ {
						f[y*g.W+x] = openFetch
					}
					continue
				}
			}
			for k := 0; k < g.W; k++ {
				x := start - d.X*k
				if g.Wrap {
					x = g.WrapX(x)
				}
				f[y*g.W+x] = next(x, y)
			}
		}
	}
	return out
}

// effectiveFetch is the fetch at tile i for a wind blowing along dir, in tiles
// east and south: the Shore Protection Manual's (1977) effective fetch, the
// fetches of the radials either side of the one the wind comes along weighed by
// the square of the cosine of the angle between them, over the sum of the
// cosines. The radials are the eight ways across the grid, and each counts
// for as much of the upwind half as it lies in.
func effectiveFetch(fetch [8][]float32, i int, dir [2]float64, span float64) float64 {
	var num, den float64
	for k, d := range Dirs {
		l := math.Hypot(float64(d.X), float64(d.Y))
		c := -(float64(d.X)*dir[0] + float64(d.Y)*dir[1]) / l
		if c <= 0 {
			continue
		}
		// Half the tile itself lies along the radial too.
		num += c * c * (float64(fetch[k][i]) + span*l/2)
		den += c
	}
	if den <= 0 {
		return 0
	}
	return num / den
}
