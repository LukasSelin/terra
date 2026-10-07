package atmos

import (
	"math"

	"github.com/LukasSelin/terra/geom"
)

// The wind, worked out rather than written down.
//
// The air used to blow one way along each row, east or west by which of the
// three cells of its hemisphere the row lay in, and nothing about the ground
// under it or the warmth of it had any say. Here it is read the way a
// meteorologist reads it, in the order the physics runs:
//
//   - The temperature of the air near the ground: the latitude's mean, the
//     year's swing - large deep inside a continent, small over the sea, which
//     holds its heat - and the anomaly the day's weather has carried in.
//   - The pressure at sea level: the belts the planet's circulation lays down,
//     low under the rising air of the ITCZ and the subpolar lows and high
//     under the sinking air at the Hadley cells' edges and at the poles,
//     each where the energy balance puts it and following the ITCZ north
//     and south with the sun (see circulation.go); and on top of them what
//     the warmth does, because a
//     warm column of air is a light one and a cold column a heavy one. A
//     continent in summer draws a low over itself and in winter sits under a
//     high, which is what a monsoon is.
//   - The wind that pressure drives, where the pull down the gradient, the
//     turning of the planet and the drag of the ground balance: along the
//     isobars aloft, and across them toward the low by an angle the ground
//     decides - a fifth of a right angle over the open sea and more than a
//     third over rough country (Holton, An Introduction to Dynamic
//     Meteorology, on the Ekman layer).
//   - What the ground does to it. Air meeting a range too high for the wind to
//     lift it over goes round (Smith, 1979, on the Froude number of flow over
//     mountains); what cannot go round or over is pushed through the gaps, as
//     a diagnostic wind model does it by holding the air to its mass (the
//     CALMET and WindNinja models); crests stand in more wind than hollows;
//     and cold air drains off an ice cap under its own weight.
//
// All of it is done on a coarser lattice than the tiles - an air cell some
// eighty kilometres across on a globe, a tile on a valley - because wind is
// a thing of scales far larger than a tile of a globe, and because the rain is
// read off it every time the drainage is.

// The seasons the wind is worked out for, as Phases of the year: midwinter in
// the north, the spring equinox (tick zero), midsummer, and the autumn
// equinox. Any day between is read off the four by the year's first harmonic;
// see seasonWeights.
const Phases = 4

// phaseSin is sin of each phase's place in the year, which is how far into
// the summer of the north the year stands there: see seasonal.
var phaseSin = [Phases]float64{-1, 0, 1, 0}

// dayOf is the day of the year at the middle of each phase.
var dayOf = [Phases]int{3 * Year / 4, 0, Year / 4, Year / 2}

// The planet's air.
const (
	// omega is how fast the planet turns, in radians a second.
	omega = 7.2921e-5
	// airDensity is the density of the air near the ground, kg a cubic metre.
	airDensity = 1.2
	// airReach is how many kilometres across an air cell would be. A map's
	// cells are the largest power of two in tiles that keeps under it, and
	// keeps airLeast cells either way.
	airReach = 80.0
)

// The belts of pressure, in hPa. The real world's zonal means at sea level:
// a trough a little under a thousand and ten at the equator, the subtropical
// highs at some thousand and twenty, the subpolar lows at a thousand and
// less, and a weak high over each pole. Where they lie, and how deep they
// are on another planet than today's, is the circulation's: see
// circulation.go.
const (
	beltMean    = 1012.0
	beltEquator = 5.0  // how far under beltMean the equatorial trough lies
	beltHorse   = 9.0  // how far over it the subtropical highs stand
	beltPolar   = 13.0 // how far under it the subpolar lows lie
	beltCap     = 5.0  // how far over it the polar highs stand
	// beltWinter is how much deeper a subpolar low is in its own winter, as a
	// share: the Icelandic and Aleutian lows are a third again as deep in
	// January as in July.
	beltWinter = 0.25
)

// What warmth does to the pressure.
const (
	// contReach is how far round a place, in kilometres, the land is counted
	// that makes its climate continental.
	contReach = 750.0
	// synopticReach is how far, in kilometres, the warmth of the air is
	// averaged before the pressure is read off it: a thermal low is the size
	// of a country and not of a valley. It is taken twice.
	synopticReach = 500.0
	// The pressure a column's warmth takes off the ground under it is the
	// hypsometric equation's: a layer H deep warmer by ΔT at T kelvin weighs
	// p g H ΔT / (R_d T²) less. H is the depth of the air the ground warms or
	// chills, which is the depth of its boundary layer: the marine layer's
	// kilometre over the sea (Stull, 1988), the dry convective layer's three
	// and more over a continent in its summer - the Saharan and Indian heat
	// lows reach four or five (Lavaysse and others, 2009) - and the cold
	// dome's one and a half under a continent's winter high (Ding, 1990). It
	// used to be one figure, 1.5 hPa a degree, raised until a continent at
	// twenty-five degrees drew the sea wind in in its summer.
	boundarySea  = 1000.0
	boundaryWarm = 3500.0
	boundaryCold = 1500.0
	// warmDepth is how deep, in metres, the warmth the day's weather has
	// carried in reaches: a warm sector lasts days, not a season, and the air
	// above its lowest kilometre has not all warmed.
	warmDepth = 1000.0
)

// How the ground drags on the air. A drag is written as a rate, per second:
// the wind across the isobars is atan(drag/f) off them.
const (
	// dragSea is the drag of the open sea on a light wind, and dragLand that
	// of open country: at forty-five degrees and an ordinary wind they turn
	// the air some twenty and some thirty-five degrees off the isobars.
	dragSea  = 2.3e-5
	dragLand = 5.2e-5
	// dragRough is how many metres of relief make open country's drag twice
	// what it was.
	dragRough = 300.0
	// dragQuadratic is how much more drag each metre a second of wind brings:
	// the bulk drag of a surface, which goes as the square of the wind. It is
	// what keeps a hurricane's wind at fifty metres a second rather than two
	// hundred. Land's is twice the sea's.
	dragQuadratic = 2.0e-6
	// WindMost is more wind than the air near the ground ever has, in metres a
	// second, as a guard against the arithmetic.
	WindMost = 85.0
)

// What the ground does to the wind.
const (
	// buoyancy is N, how stiffly the air resists being lifted, per second:
	// the ordinary figure for a stable lower atmosphere. The wind goes over a
	// range of height h only if it is faster than N times h.
	buoyancy = 0.01
	// blockReach is how far ahead, in kilometres, the air looks for ground it
	// would have to climb, in blockSteps steps of no more than blockCells
	// cells each.
	blockReach = 300.0
	blockSteps = 8
	blockCells = 4.0
	// ExposeReach is how many cells round count as the country a cell stands
	// over or sinks under. A hill h over it and some ExposeReach cells to its
	// half-height quickens the wind on its crest by 2h/L (Jackson and Hunt,
	// 1975; Taylor and Lee, 1984: ΔS ≈ 1.6-2 h/L), and a hollow slows it as
	// much; the wind is never less than exposeLeast of itself nor more than
	// exposeMost.
	ExposeReach = 2
	exposeLeast = 0.5
	exposeMost  = 2.0
	// The wind that drains off an ice cap under its own weight: a layer
	// katabaticDepth metres deep, as cold under the air above as the ground
	// under freezing, to katabaticCold degrees at most, running down a slope of
	// sine s at sqrt(g (Δθ/θ) H s / C_D) against the drag and the air it drags
	// along with it, katabaticDrag (Ball, 1956; Parish and Bromwich, 1987:
	// 10-20 m/s off Antarctica's coastal slopes of a few in a hundred). It
	// turns katabaticTurn radians to the right of downhill in the north and to
	// the left in the south.
	katabaticDepth = 100.0
	katabaticDrag  = 5e-3
	katabaticCold  = 20.0
	katabaticTurn  = 0.5
	// layerDepth is how deep the air near the ground is, in metres, over the
	// sea; over high ground it is layerScale metres shallower by a factor of e.
	// The air the mountains squeeze has to go faster, or somewhere else.
	layerDepth = 1000.0
	layerScale = 2500.0
	// channelRounds is how many rounds of relaxation the air is given to find
	// its way round what the ground put in its way. The rounds spread a push
	// a handful of cells, which is the reach of a gap in a range, and leave
	// the convergence of the planet's own circulation - which is real, and is
	// where the rain belts are - alone.
	channelRounds = 12
	// channelOver is how far past each round's answer a round is taken, which
	// is what lets a dozen rounds do the work of some fifty.
	channelOver = 1.6
)

// Winds is the climate of the wind over a map as its ground now lies: for
// each phase of the year, the wind near the ground and the pressure at sea
// level on each air cell. It is made afresh whenever the rain is, and is not
// changed afterwards, so copies of a map share it.
type Winds struct {
	*Env
	// U is the wind toward the east and V toward the north, in metres a
	// second; P is the pressure at sea level in hPa.
	U, V, P [Phases][]float32
	// Budget is the water in the air in each phase, as the rain was last
	// worked out over the wind: see vapour.go.
	Budget [Phases]vapourOut
}

// Env is the ground as the air reads it: the lattice of air cells and
// everything about the ground under each that the wind is worked out from.
type Env struct {
	Cell, W, H int // tiles to a cell's side, and cells across and down
	Wrap       bool

	lat []float64 // the latitude of each row of cells on the planet, degrees
	// swingSea and swingLand are the energy balance's swing on each row over
	// the open sea and deep inside a continent, signed by hemisphere: see
	// seasonTemp.
	swingSea, swingLand []float64
	// forcing is the one the air's year is worked out under: the map's, or
	// today's on a valley; circ is the balance under it, which the
	// circulation is worked out from.
	forcing Forcing
	circ    *ebmClimate
	// tropicN and tropicS are how much of the ground within tropicReach of
	// the equator is land, north of it and south: see beltsAt.
	tropicN, tropicS float64

	Mean []float64 // the year's mean temperature at sea level on each row
	Dx   []float64 // metres across a cell along each row
	Dy   float64   // and down one
	f    []float64 // the Coriolis parameter on each row, per second

	Sea    []float64 // how much of each cell lies under the water the air takes its fill from
	Cont   []float64 // how much of the country round each cell is land
	Height []float64 // the mean height of each cell above that water, metres
	Gx, Gy []float64 // the lie of the smoothed ground, metres a metre, rising east and north
	rough  []float64 // how broken the ground in each cell is, metres
	Expose []float64 // how far each cell stands over the country round it, metres
	depth  []float64 // how deep the air near the ground is over each cell, metres
	// climb is the most ground within blockReach of each cell stands over it,
	// in metres, whichever way the wind comes: no wind faster than buoyancy
	// times this is blocked there, and the looking ahead is spared.
	climb []float64

	// Warm is how many degrees the sea over each cell stands over its
	// latitude's mean for the currents, and Coast what that is worth to the
	// country round it: see currents. Both are nil on a valley.
	Warm, Coast []float64
	// Cu and Cv are the sea's current over each cell, metres a second toward
	// the east and the north; Rise how fast water comes up from under it,
	// metres a second, off a shore and in the open ocean; and WaterTemp the water's temperature, degrees: the
	// latitude's mean and Warm, before Warm is held to seaWarmMost. Land
	// has no current and no upwelling; the land along a shore is given the
	// temperature of the sea beside it, and the land away from the sea its
	// latitude's mean. All are nil on a valley.
	Cu, Cv, Rise, WaterTemp []float32

	// Subsides is how fast the Hadley cell's air comes down over each cell
	// in each phase of the year, metres a second at 500 hPa, which lays the
	// trade-wind inversion over it. See circulation.go.
	Subsides [Phases][]float64
	// Psi is the transport streamfunction the gyres are read off, in
	// sverdrups: the water between two cells flows past them at the
	// difference of their Psi, with the higher on its right, and a gyre is
	// a closed contour of it. On land it is the level of the landmass,
	// nought on the largest. See flow.go. Nil on a valley.
	Psi []float32
	// Thermocline is how deep the warm water over the cold deep goes under
	// each cell, metres: shallow against an ocean's eastern shore and under
	// the subpolar gyres, deep in the west of the tropics and under the
	// subtropical gyres. Nought on land. See thermocline.go. Nil on a
	// valley.
	Thermocline []float32

	// seaAir is the sea's warmth as the air over each cell reads it for its
	// pressure: how many degrees the air there stands over its latitude's
	// for the water under it, the sea's share of the cell times its warmth.
	// Nil until the sea and the air are solved together, and on a valley.
	// See coupled.go.
	seaAir []float64
	// Coupled is the residual of each round of the coupled solve, the last
	// what was left after the last: see couple. Nil on a valley.
	Coupled []Residual
	// Walk is the wind toward the east and the north, metres a second, and
	// the pressure, hPa, the tropical sea's warmth makes over each cell
	// through the trades' layer and the heat of the rain over warm water:
	// the Walker circulation's. See walker. Nil on a valley.
	Walk [3][]float32
}

// airCell is how many tiles a side the air cells over a map m are. The map's
// width and height are whole numbers of cells.
func airCell(m *geom.Map, a *Air) int {
	cell := 1
	for float64(2*cell)*a.Dy <= airReach*1.2 && m.W%(2*cell) == 0 && m.H%(2*cell) == 0 &&
		m.W/(2*cell) >= airLeast && m.H/(2*cell) >= airLeast {
		cell *= 2
	}
	return cell
}

// airLeast is how few cells a map may be read on either way. A valley is a
// couple of kilometres of ground to a cell rather than the eighty airReach
// asks for, because a valley is not eighty kilometres across; what it must
// not be is so few cells that the ground has nothing to say to the wind.
const airLeast = 16

// NewEnv reads the ground of a map m as the air sees it, under the air a:
// above is how far each tile stands over the water the air takes its fill
// from, and wet is one where the tile is under that water and nothing where
// it is not.
func NewEnv(m *geom.Map, a *Air, above, wet []float64) *Env {
	cell := airCell(m, a)
	e := &Env{Cell: cell, W: m.W / cell, H: m.H / cell, Wrap: m.Wrap, forcing: a.Forcing.OrDefault()}
	e.circ = ebmUnder(e.forcing)
	n := e.W * e.H
	e.lat, e.Mean, e.Dx, e.f = make([]float64, e.H), make([]float64, e.H), make([]float64, e.H), make([]float64, e.H)
	e.swingSea, e.swingLand = make([]float64, e.H), make([]float64, e.H)
	e.Dy = a.Dy * 1000 * float64(cell)
	for cy := 0; cy < e.H; cy++ {
		var lat, mean, dx float64
		for y := cy * cell; y < (cy+1)*cell; y++ {
			lat += a.Lat[y]
			mean += a.Mean[y]
			dx += a.Dx[y]
		}
		k := float64(cell)
		lat, mean, dx = lat/k, mean/k, dx/k
		// The air's year is the ground's: the energy balance's at the row's
		// latitude, under the map's forcing. See seasonTemp.
		e.swingSea[cy] = SwingUnder(e.forcing, lat, 0)
		e.swingLand[cy] = SwingUnder(e.forcing, lat, 1)
		if !m.Wrap {
			// A valley is one latitude's weather, but the planet under it
			// is still round: the pressure the belts lay down still falls
			// across it from south to north, or the air would not move.
			lat -= (float64(cy) + 0.5 - float64(e.H)/2) * e.Dy / 111195
		}
		e.lat[cy], e.Mean[cy] = lat, mean
		e.Dx[cy] = dx * 1000 * k
		e.f[cy] = 2 * omega * math.Sin(lat*math.Pi/180)
	}

	// The ground, tile by tile, then gathered into cells.
	broken := make([]float64, m.W*m.H)
	eachTileRow(m, func(y int) {
		for x := 0; x < m.W; x++ {
			i := y*m.W + x
			var sum, sq, k float64
			for dy := -1; dy <= 1; dy++ {
				yy := y + dy
				if yy < 0 || yy >= m.H {
					continue
				}
				for dx := -1; dx <= 1; dx++ {
					xx := x + dx
					if m.Wrap {
						xx = (xx + m.W) % m.W
					} else if xx < 0 || xx >= m.W {
						continue
					}
					h := above[yy*m.W+xx]
					sum, sq, k = sum+h, sq+h*h, k+1
				}
			}
			m := sum / k
			broken[i] = math.Sqrt(math.Max(0, sq/k-m*m))
		}
	})
	e.Height, e.Sea, e.rough = e.gather(above), e.gather(wet), e.gather(broken)

	land := make([]float64, n)
	for i := range land {
		land[i] = 1 - e.Sea[i]
	}
	e.Cont = e.blur(land, contReach)
	smooth := e.blurCells(e.Height, 1)
	e.Expose = make([]float64, n)
	round := e.blurCells(e.Height, ExposeReach)
	e.Gx, e.Gy, e.depth = make([]float64, n), make([]float64, n), make([]float64, n)
	for cy := 0; cy < e.H; cy++ {
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			e.Gx[i], e.Gy[i] = e.grad(smooth, cx, cy)
			e.Expose[i] = e.Height[i] - round[i]
			e.depth[i] = layerDepth * math.Exp(-e.Height[i]/layerScale)
		}
	}
	e.climb = e.climbs()
	e.descents()
	return e
}

// climbs is climb for every cell: the highest ground in the square of cells a
// wind at the cell could look ahead over, less the cell's own.
func (e *Env) climbs() []float64 {
	n := e.W * e.H
	stepKm := blockReach / blockSteps
	ry := int(math.Ceil(math.Min(stepKm*1000/e.Dy, blockCells)*blockSteps)) + 1
	mid := make([]float64, n)
	e.rows(func(cy int) {
		rx := int(math.Ceil(math.Min(stepKm*1000/e.Dx[cy], blockCells)*blockSteps)) + 1
		rx = min(rx, e.W)
		for cx := 0; cx < e.W; cx++ {
			top := 0.0
			for k := -rx; k <= rx; k++ {
				top = math.Max(top, e.Height[e.at(cx+k, cy)])
			}
			mid[cy*e.W+cx] = top
		}
	})
	out := make([]float64, n)
	e.rows(func(cy int) {
		for cx := 0; cx < e.W; cx++ {
			top := 0.0
			for k := max(cy-ry, 0); k <= min(cy+ry, e.H-1); k++ {
				top = math.Max(top, mid[k*e.W+cx])
			}
			i := cy*e.W + cx
			out[i] = top - e.Height[i]
		}
	})
	return out
}

// gather is the mean of a reading of the map's tiles over each air cell.
func (e *Env) gather(v []float64) []float64 {
	if e.Cell == 1 {
		return append([]float64(nil), v...)
	}
	out := make([]float64, e.W*e.H)
	k := float64(e.Cell * e.Cell)
	across := e.W * e.Cell
	for cy := 0; cy < e.H; cy++ {
		for cx := 0; cx < e.W; cx++ {
			var s float64
			for y := cy * e.Cell; y < (cy+1)*e.Cell; y++ {
				for x := cx * e.Cell; x < (cx+1)*e.Cell; x++ {
					s += v[y*across+x]
				}
			}
			out[cy*e.W+cx] = s / k
		}
	}
	return out
}

// rows runs f for every row of cells, spread over goroutines where there are
// cells enough to be worth it, under the same rule as terra.Grid.EachRow: f writes
// only at its own row's cells.
func (e *Env) rows(f func(cy int)) {
	if e.W*e.H < spreadTiles {
		for cy := 0; cy < e.H; cy++ {
			f(cy)
		}
		return
	}
	inParallel(e.H, workersFor(e.H), func(cy, _ int) { f(cy) })
}

// at is the cell cx, cy, with the column taken round the seam or held at the
// edge and the row held at the poles or the edge.
func (e *Env) at(cx, cy int) int {
	if uint(cx) < uint(e.W) && uint(cy) < uint(e.H) {
		return cy*e.W + cx
	}
	if e.Wrap {
		cx = ((cx % e.W) + e.W) % e.W
	} else {
		cx = min(max(cx, 0), e.W-1)
	}
	cy = min(max(cy, 0), e.H-1)
	return cy*e.W + cx
}

// grad is the slope of v at a cell, per metre, rising toward the east and
// toward the north.
func (e *Env) grad(v []float64, cx, cy int) (east, north float64) {
	east = (v[e.at(cx+1, cy)] - v[e.at(cx-1, cy)]) / (2 * e.Dx[cy])
	north = (v[e.at(cx, cy-1)] - v[e.at(cx, cy+1)]) / (2 * e.Dy)
	return east, north
}

// Sample is v read between the cells, at a place given in cells.
func (e *Env) Sample(v []float64, fx, fy float64) float64 {
	x0, y0 := math.Floor(fx), math.Floor(fy)
	tx, ty := fx-x0, fy-y0
	x, y := int(x0), int(y0)
	a := v[e.at(x, y)] + (v[e.at(x+1, y)]-v[e.at(x, y)])*tx
	b := v[e.at(x, y+1)] + (v[e.at(x+1, y+1)]-v[e.at(x, y+1)])*tx
	return a + (b-a)*ty
}

// blur is v averaged over the square reach kilometres either way of each
// cell, which is a different number of cells along a row near a pole than at
// the equator.
func (e *Env) blur(v []float64, reach float64) []float64 {
	return e.blurIn(nil, 0, v, reach)
}

// blurIn is blur worked out in w, with the answer in w's slot: see Scratch.
func (e *Env) blurIn(w *work, slot int, v []float64, reach float64) []float64 {
	across := make([]int, e.H)
	for cy := range across {
		across[cy] = int(math.Round(reach / (e.Dx[cy] / 1000)))
	}
	return e.boxIn(w, slot, v, across, int(math.Round(reach/(e.Dy/1000))))
}

// blurCells is v averaged over the square r cells either way of each cell.
func (e *Env) blurCells(v []float64, r int) []float64 {
	across := make([]int, e.H)
	for cy := range across {
		across[cy] = r
	}
	return e.box(v, across, r)
}

// box is the running mean of v, across[cy] cells either way along each row
// and down cells either way down each column, clipped at the edges and taken
// round the seam.
func (e *Env) box(v []float64, across []int, down int) []float64 {
	return e.boxIn(nil, 0, v, across, down)
}

// boxIn is box worked out in w, with the answer in w's slot: see Scratch.
func (e *Env) boxIn(w *work, slot int, v []float64, across []int, down int) []float64 {
	mid := w.floats(slotBoxMid, len(v))
	e.rows(func(cy int) {
		r := across[cy]
		row := cy * e.W
		if e.Wrap && 2*r+1 >= e.W {
			var s float64
			for cx := 0; cx < e.W; cx++ {
				s += v[row+cx]
			}
			for cx := 0; cx < e.W; cx++ {
				mid[row+cx] = s / float64(e.W)
			}
			return
		}
		var s float64
		var k int
		for cx := -r; cx <= r; cx++ {
			if e.Wrap || (cx >= 0 && cx < e.W) {
				s += v[e.at(cx, cy)]
				k++
			}
		}
		for cx := 0; cx < e.W; cx++ {
			mid[row+cx] = s / float64(k)
			out, in := cx-r, cx+r+1
			if e.Wrap || out >= 0 {
				s -= v[e.at(out, cy)]
				k--
			}
			if e.Wrap || in < e.W {
				s += v[e.at(in, cy)]
				k++
			}
		}
	})
	out := w.floats(slot, len(v))
	down = min(down, e.H)
	for cx := 0; cx < e.W; cx++ {
		var s float64
		var k int
		for cy := 0; cy <= down && cy < e.H; cy++ {
			s += mid[cy*e.W+cx]
			k++
		}
		for cy := 0; cy < e.H; cy++ {
			out[cy*e.W+cx] = s / float64(k)
			if leave := cy - down; leave >= 0 {
				s -= mid[leave*e.W+cx]
				k--
			}
			if enter := cy + down + 1; enter < e.H {
				s += mid[enter*e.W+cx]
				k++
			}
		}
	}
	return out
}

// WindsFor works out the climate of the wind over a map m as its ground now
// lies: see newAirEnv for above and wet. was is the wind as it was last
// worked out over the same map, or nil: the sea's warmth the air read then is
// where the sea and the air are worked out together from (see carry).
//
// s is the working memory the reading is worked out in, which the rain is
// worked out in after it (see Scratch); with none it makes its own.
func WindsFor(m *geom.Map, a *Air, above, wet []float64, was *Winds, s *Scratch) *Winds {
	e := NewEnv(m, a, above, wet)
	w := &Winds{Env: e}
	n := e.W * e.H
	for k := range Phases {
		w.U[k], w.V[k], w.P[k] = make([]float32, n), make([]float32, n), make([]float32, n)
	}
	// On a globe the air reads the sea as the wind was last worked out over
	// it, where it was, and the wind and the water are then worked out
	// together. See ocean.go and coupled.go.
	if e.Wrap {
		e.carry(was)
	}
	w.solve(s)
	if e.Wrap {
		// The gyres' equations are the ground's, and are written down once
		// for every round.
		ocean := e.newFlow()
		e.Warm = e.currents(w.U, w.V, ocean, s)
		w.couple(ocean, s)
		e.Coast = e.coastal(e.Warm)
	}
	return w
}

// solve works the wind of each phase of the year out. The phases are
// independent of one another and are worked out side by side; each writes
// only its own slices. The two equinoxes are the same day to the air, so
// the autumn's is the spring's. Each phase is worked out in its own part of
// s (see Scratch).
func (w *Winds) solve(s *Scratch) {
	e := w.Env
	workers := 1
	if e.W*e.H >= spreadTiles {
		workers = workersFor(Phases)
	}
	inParallel(Phases-1, workers, func(k, _ int) {
		wk := s.phaseWork(k)
		e.solve(wk, phaseSin[k], e.airTempIn(wk, slotAirTemp, phaseSin[k]), nil, nil, w.U[k], w.V[k], w.P[k])
	})
	copy(w.U[3], w.U[1])
	copy(w.V[3], w.V[1])
	copy(w.P[3], w.P[1])
}

// AirTemp is the temperature of the air at sea level over each cell in an
// ordinary year, sinT of the way into the north's summer. The phases of the
// wind's year are its thermal seasons - the warmest, the coldest and the turn
// between - so each cell is read at the crest of its own swing, whatever its
// lag behind the sun. The swing is the one terra.Land.TempAt reads: see seasonTemp.
func (e *Env) AirTemp(sinT float64) []float64 {
	return e.airTempIn(nil, 0, sinT)
}

// airTempIn is AirTemp in w's slot: see Scratch.
func (e *Env) airTempIn(w *work, slot int, sinT float64) []float64 {
	temp := w.floats(slot, e.W*e.H)
	for cy := 0; cy < e.H; cy++ {
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			temp[i] = e.Mean[cy] + e.seasonTemp(cy, sinT, e.Cont[i])
		}
	}
	return temp
}

// airTempOn is airTemp on a day of the calendar: each cell at its own place in
// its swing, lagging the sun by as much as the land round it makes it lag. A
// valley's year has no lag, and is airTemp at the day's sun to the bit.
func (e *Env) airTempOn(day int) []float64 {
	if !e.Wrap {
		return e.AirTemp(YearSin(day))
	}
	temp := make([]float64, e.W*e.H)
	for cy := 0; cy < e.H; cy++ {
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			temp[i] = e.Mean[cy] + e.seasonTemp(cy, SeasonAt(day, LagUnder(e.forcing, e.Cont[i])), e.Cont[i])
		}
	}
	return temp
}

// seasonTemp is what the year adds to the mean on row cy, phase of the way
// from its mean to its crest, over ground cont continental: the swing the
// wind, the storms and the evaporation read, which is the ground's - SwingAt's,
// the energy balance's sea and land at the row's latitude, between in
// proportion.
//
// It used to be a swing of its own: the valley's twelve degrees, times the
// share of the temperate latitude's the row had - growing as the latitude
// and stopping at Temperate - times a third over the open sea and one and
// three fifths deep in a continent. That kept the air at forty-five's year all
// the way to the pole while the ground under it swung the balance's, and the
// sea's year at four degrees where the balance's mixed layer gives three at
// forty-five and ten at sixty-five, where the ice comes and goes.
func (e *Env) seasonTemp(cy int, phase, cont float64) float64 {
	sea := e.swingSea[cy]
	return phase * (sea + (e.swingLand[cy]-sea)*cont)
}

// hypsometric is how many hPa a layer depth metres deep over ground at p hPa
// weighs less for each degree it stands warmer, at temp degrees.
func hypsometric(p, temp, depth float64) float64 {
	t := temp + 273.15
	return p * gravity * depth / (dryGas * t * t)
}

// Solve works out the wind over the cells with the year sinT of the way into
// the north's summer, over air at sea level of temp degrees. extra is pressure added to what the climate lays down,
// in hPa, and warm the degrees the day's weather has carried in; either may be
// nil. The wind and the pressure are written to u, v and p.
func (e *Env) Solve(sinT float64, temp, extra, warm []float64, u, v, p []float32) {
	e.solve(nil, sinT, temp, extra, warm, u, v, p)
}

// solve is Solve worked out in w: see Scratch.
func (e *Env) solve(w *work, sinT float64, temp, extra, warm []float64, u, v, p []float32) {
	n := e.W * e.H

	// The warmth of the air at sea level, with what the sea under it adds
	// once the sea and the air are solved together, outside the tropics:
	// within them it is the trades' layer's and the rain's (coupled.go,
	// walker). Then the pressure it and the belts make between them.
	if e.seaAir != nil {
		over := make([]float64, n)
		for i := range over {
			over[i] = temp[i] + e.seaAir[i]*(1-e.tropicShare(i/e.W))
		}
		temp = over
	}
	temp = e.blurIn(w, slotTemp, e.blurIn(w, slotTempBlur, temp, synopticReach), synopticReach)
	if warm != nil {
		warm = e.blurIn(w, slotWarm, e.blurIn(w, slotWarmBlur, warm, synopticReach), synopticReach)
	}
	pres := w.floats(slotPres, n)
	b := e.beltsAt(sinT)
	for cy := 0; cy < e.H; cy++ {
		row := cy * e.W
		var zonal float64
		for cx := 0; cx < e.W; cx++ {
			zonal += temp[row+cx]
		}
		zonal /= float64(e.W)
		belt := b.pressure(e.lat[cy], sinT)
		for cx := 0; cx < e.W; cx++ {
			i := row + cx
			dt := temp[i] - zonal
			depth := boundaryCold
			if dt > 0 {
				depth = boundaryWarm
			}
			depth = boundarySea + (depth-boundarySea)*e.Cont[i]
			pres[i] = belt - hypsometric(belt, temp[i], depth)*dt
			if warm != nil {
				pres[i] -= hypsometric(belt, temp[i], warmDepth) * warm[i]
			}
			if extra != nil {
				pres[i] += extra[i]
			}
		}
	}

	// The wind the pressure drives, against the turning of the planet and the
	// drag of the ground; then what the ground in its way does to it.
	free := [2][]float64{w.floats(slotFreeU, n), w.floats(slotFreeV, n)}
	wind := [2][]float64{w.floats(slotWindU, n), w.floats(slotWindV, n)}
	e.rows(func(cy int) {
		f := e.f[cy]
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			px, py := e.grad(pres, cx, cy)
			px, py = px*100/airDensity, py*100/airDensity // hPa to Pa, and a force on a kilogram
			land := 1 - e.Sea[i]
			r0 := dragSea + (dragLand*(1+e.rough[i]/dragRough)-dragSea)*land
			q := dragQuadratic * (1 + land)
			r := r0
			var uu, vv float64
			for range 3 {
				d := r*r + f*f
				uu, vv = (-r*px-f*py)/d, (f*px-r*py)/d
				r = (r + r0 + q*math.Hypot(uu, vv)) / 2
			}
			free[0][i], free[1][i] = uu, vv
			uu, vv = e.Ground(cx, cy, uu, vv)
			wind[0][i], wind[1][i] = uu, vv
		}
	})
	e.channel(w, free, wind)

	// What the sea's warmth does in the trades' layer, where the sea and the
	// air are solved together: see walker.
	if e.Walk[0] != nil {
		for i := 0; i < n; i++ {
			wind[0][i] += float64(e.Walk[0][i])
			wind[1][i] += float64(e.Walk[1][i])
			pres[i] += float64(e.Walk[2][i])
		}
	}
	for i := 0; i < n; i++ {
		uu, vv := wind[0][i], wind[1][i]
		if s := math.Hypot(uu, vv); s > WindMost {
			uu, vv = uu*WindMost/s, vv*WindMost/s
		}
		u[i], v[i], p[i] = float32(uu), float32(vv), float32(pres[i])
	}
}

// Ground is the wind uu, vv at a cell after the Ground there has had its say:
// turned aside by a range it cannot climb, quickened on a crest and slowed in
// a hollow, and joined by the air draining off the ice.
func (e *Env) Ground(cx, cy int, uu, vv float64) (float64, float64) {
	i := cy*e.W + cx
	gx, gy := e.Gx[i], e.Gy[i]
	slope := math.Hypot(gx, gy)

	// Blocking. The ground ahead along the wind, and how much of it the wind
	// would have to climb; where it is too slow to climb it, the part of it
	// blowing up the slope is turned along the slope instead.
	if s := math.Hypot(uu, vv); s > 0.1 && slope > 1e-6 && s < buoyancy*e.climb[i] {
		ex, ny := uu/s, vv/s
		stepKm := blockReach / blockSteps
		sx := math.Min(stepKm*1000/e.Dx[cy], blockCells) // cells a step along the row
		sy := math.Min(stepKm*1000/e.Dy, blockCells)
		here, top := e.Height[i], e.Height[i]
		for k := 1; k <= blockSteps; k++ {
			h := e.Sample(e.Height, float64(cx)+ex*sx*float64(k), float64(cy)-ny*sy*float64(k))
			top = math.Max(top, h)
		}
		if climb := top - here; climb > 0 {
			block := math.Max(0, 1-s/(buoyancy*climb))
			nx, nz := gx/slope, gy/slope
			if up := uu*nx + vv*nz; up > 0 {
				uu -= block * up * nx
				vv -= block * up * nz
			}
		}
	}

	// Exposure: Jackson and Hunt's speed-up over a hill of the country's
	// breadth.
	half := float64(ExposeReach) * math.Min(e.Dx[cy], e.Dy)
	k := math.Max(exposeLeast, math.Min(exposeMost, 1+2*e.Expose[i]/half))
	uu, vv = uu*k, vv*k

	// The ice's own wind.
	if slope > 1e-6 {
		air := e.Mean[cy] - Lapse*e.Height[i]
		cold := math.Min(katabaticCold, -air)
		if cold > 0 {
			sine := slope / math.Sqrt(1+slope*slope)
			s := math.Sqrt(gravity * cold / (math.Max(air, coldest) + 273.15) * katabaticDepth * sine / katabaticDrag)
			dx, dz := -gx/slope, -gy/slope
			turn := -math.Copysign(katabaticTurn, e.f[cy])
			c, sn := math.Cos(turn), math.Sin(turn)
			uu += s * (c*dx - sn*dz)
			vv += s * (sn*dx + c*dz)
		}
	}
	return uu, vv
}

// channel holds the air to its mass. What the ground did to the wind in
// ground - the climbs refused, the crests and hollows, the squeezing of the
// air over high ground into a shallower layer - leaves air piling up in some
// places and missing from others that the pressure never put there, and the
// wind is given channelRounds rounds of relaxation to carry it off: through
// the gaps in a range and round its ends. The convergence the free wind had
// of its own is left, because that is the planet's circulation and not the
// ground's doing.
func (e *Env) channel(w *work, free, wind [2][]float64) {
	n := e.W * e.H
	fu, fv := w.floats(slotChannelU, n), w.floats(slotChannelV, n)
	for i := range fu {
		fu[i], fv[i] = e.depth[i]*wind[0][i], e.depth[i]*wind[1][i]
	}
	// push is how much more air the ground made converge on each cell than
	// the free wind did: the divergence of the flux the air near the ground
	// now has, less that of the free wind at the depth over the sea.
	push := w.floats(slotPush, n)
	e.rows(func(cy int) {
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			push[i] = e.div(fu, fv, cx, cy) - layerDepth*e.div(free[0], free[1], cx, cy)
		}
	})
	// Relaxed red and black in turn, over-relaxed: each pass writes the cells
	// of one colour and reads only those of the other, so the rows can be
	// taken on as many goroutines as there are and still come out the same.
	lam := w.floats(slotLam, n)
	for range channelRounds {
		for colour := range 2 {
			e.rows(func(cy int) {
				ax := 1 / (e.Dx[cy] * e.Dx[cy])
				ay := 1 / (e.Dy * e.Dy)
				norm := 1 / (2*ax + 2*ay)
				row := cy * e.W
				north, south := max(cy-1, 0)*e.W, min(cy+1, e.H-1)*e.W
				for cx := (cy + colour) % 2; cx < e.W; cx += 2 {
					west, east := cx-1, cx+1
					switch {
					case west < 0 && e.Wrap:
						west = e.W - 1
					case west < 0:
						west = 0
					}
					switch {
					case east == e.W && e.Wrap:
						east = 0
					case east >= e.W:
						east = e.W - 1
					}
					sx := lam[row+east] + lam[row+west]
					sy := lam[north+cx] + lam[south+cx]
					want := (ax*sx + ay*sy - push[row+cx]) * norm
					lam[row+cx] += channelOver * (want - lam[row+cx])
				}
			})
		}
	}
	e.rows(func(cy int) {
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			gx, gy := e.grad(lam, cx, cy)
			wind[0][i] = (fu[i] - gx) / e.depth[i]
			wind[1][i] = (fv[i] - gy) / e.depth[i]
		}
	})
}

// div is the divergence of the field fu, fv at a cell, per metre, with the
// parallels shortening toward the poles.
func (e *Env) div(fu, fv []float64, cx, cy int) float64 {
	east := (fu[e.at(cx+1, cy)] - fu[e.at(cx-1, cy)]) / (2 * e.Dx[cy])
	nr, sr := max(cy-1, 0), min(cy+1, e.H-1)
	north := (fv[e.at(cx, nr)]*e.Dx[nr] - fv[e.at(cx, sr)]*e.Dx[sr]) / (2 * e.Dy * e.Dx[cy])
	return east + north
}

// seasonWeights is how much each phase of the year is worth on a day: the
// year's mean and its first harmonic, read off the four phases, which lie a
// quarter of a year apart.
func seasonWeights(day int) [Phases]float64 {
	th := 2 * math.Pi * float64(day) / Year
	s, c := math.Sin(th), math.Cos(th)
	// A field over the year is m + S sin + C cos, and the phases are its values
	// at sin -1, cos 1, sin 1 and cos -1.
	return [Phases]float64{0.25 - s/2, 0.25 + c/2, 0.25 + s/2, 0.25 - c/2}
}

// CellAt is where tile i's centre lies among the air cells, in cells.
func (e *Env) CellAt(i int) (fx, fy float64) {
	across := e.W * e.Cell
	x, y := i%across, i/across
	k := float64(e.Cell)
	return (float64(x)+0.5)/k - 0.5, (float64(y)+0.5)/k - 0.5
}

// Sample32 is sample for a reading kept in single precision.
func (e *Env) Sample32(v []float32, fx, fy float64) float64 {
	x0, y0 := math.Floor(fx), math.Floor(fy)
	tx, ty := fx-x0, fy-y0
	x, y := int(x0), int(y0)
	at := func(cx, cy int) float64 { return float64(v[e.at(cx, cy)]) }
	a := at(x, y) + (at(x+1, y)-at(x, y))*tx
	b := at(x, y+1) + (at(x+1, y+1)-at(x, y+1))*tx
	return a + (b-a)*ty
}

// WindOn is terra.Grid.WindOn for a tile of the map.
func (w *Winds) WindOn(i, day int) (east, north float64) {
	fx, fy := w.CellAt(i)
	for k, m := range seasonWeights(day) {
		east += m * w.Sample32(w.U[k], fx, fy)
		north += m * w.Sample32(w.V[k], fx, fy)
	}
	return east, north
}

// MeanWind is terra.Grid.MeanWind for a tile of the map.
func (w *Winds) MeanWind(i int) (east, north float64) {
	fx, fy := w.CellAt(i)
	for k := range Phases {
		east += w.Sample32(w.U[k], fx, fy) / Phases
		north += w.Sample32(w.V[k], fx, fy) / Phases
	}
	return east, north
}

// PressureOn is terra.Grid.PressureOn for a tile of the map.
func (w *Winds) PressureOn(i, day int) float64 {
	fx, fy := w.CellAt(i)
	var p float64
	for k, m := range seasonWeights(day) {
		p += m * w.Sample32(w.P[k], fx, fy)
	}
	return p
}

// eachTileRow runs f for every row of the map's tiles, spread over goroutines
// under the same rule as terra.Grid.EachRow: f writes only at its own row's tiles.
func eachTileRow(m *geom.Map, f func(y int)) {
	if m.W*m.H < spreadTiles {
		for y := 0; y < m.H; y++ {
			f(y)
		}
		return
	}
	inParallel(m.H, workersFor(m.H), func(y, _ int) { f(y) })
}
