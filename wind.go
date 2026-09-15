package terra

import "math"

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
//     low under the rising air at the equator and at sixty degrees and high
//     under the sinking air at thirty and at the poles, shifting north and
//     south with the sun; and on top of them what the warmth does, because a
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

// The seasons the wind is worked out for, as phases of the year: midwinter in
// the north, the spring equinox (tick zero), midsummer, and the autumn
// equinox. Any day between is read off the four by the year's first harmonic;
// see seasonWeights.
const phases = 4

// phaseSin is sin of each phase's place in the year, which is how far into
// the summer of the north the year stands there: see seasonal.
var phaseSin = [phases]float64{-1, 0, 1, 0}

// dayOf is the day of the year at the middle of each phase.
var dayOf = [phases]int{3 * Year / 4, 0, Year / 4, Year / 2}

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
// less, and a weak high over each pole.
const (
	beltMean    = 1012.0
	beltEquator = 5.0  // how far under beltMean the equatorial trough lies
	beltHorse   = 9.0  // how far over it the subtropical highs stand
	beltPolar   = 13.0 // how far under it the subpolar lows lie
	beltCap     = 5.0  // how far over it the polar highs stand
	// beltShift is how many degrees the belts follow the sun north and south
	// over the year. The real trough wanders further over the continents, and
	// the continents are what move it there: see thermalGain.
	beltShift = 5.0
	// beltWinter is how much deeper a subpolar low is in its own winter, as a
	// share: the Icelandic and Aleutian lows are a third again as deep in
	// January as in July.
	beltWinter = 0.25
)

// beltPressure is the pressure at sea level the circulation alone lays down
// at a latitude, with the year sinT of the way into the north's summer.
func beltPressure(lat, sinT float64) float64 {
	l := lat - beltShift*sinT
	a := math.Abs(l)
	winter := 1 - beltWinter*sinT*math.Copysign(1, l)
	bump := func(x float64) float64 { return math.Exp(-x * x) }
	return beltMean - beltEquator*bump(l/10) + beltHorse*bump((a-32)/10) -
		beltPolar*winter*bump((a-62)/10) + beltCap*bump((a-88)/12)
}

// What warmth does to the pressure.
const (
	// swingSea and swingLand are how much of the year's swing air over the
	// open sea and air deep inside a continent have. Over the ocean the
	// year's range is a few degrees; in the middle of Asia it is fifty, which
	// is some twice the swing the default valley has.
	swingSea  = 0.35
	swingLand = 1.6
	// contReach is how far round a place, in kilometres, the land is counted
	// that makes its climate continental.
	contReach = 750.0
	// synopticReach is how far, in kilometres, the warmth of the air is
	// averaged before the pressure is read off it: a thermal low is the size
	// of a country and not of a valley. It is taken twice.
	synopticReach = 500.0
	// thermalGain is how many hPa the pressure falls for each degree the air
	// stands warmer than its latitude's mean. A column of air warmer by a
	// degree through the lowest few kilometres weighs some half an hPa less by
	// the hypsometric equation; the Siberian high, a winter's twenty degrees
	// colder than its latitude, is some twenty hPa over it. The warmth here is
	// smoothed over a thousand kilometres before it is read, which takes the
	// edge off a continent's, and at one a continent a quarter of the way
	// round the planet at twenty-five degrees still had the trades blowing off
	// its southern coast at midsummer: at one and a half the wind there turns
	// onshore in summer and blows out at eight or nine metres a second in
	// winter, which is the south Asian monsoon's wind both ways.
	thermalGain = 1.5
	// warmGain is what a degree the day's weather has carried in is worth,
	// which is less: a warm sector lasts days, not a season, and the air
	// above it has not all warmed.
	warmGain = 0.5
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
	// windMost is more wind than the air near the ground ever has, in metres a
	// second, as a guard against the arithmetic.
	windMost = 85.0
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
	// exposeHeight is how many metres a crest must stand over the country round
	// it to have half as much wind again, and a hollow under it to have half
	// as much less; exposeReach is how many cells round count as that country.
	exposeHeight = 1000.0
	exposeReach  = 2
	// katabatic is how hard, in metres a second, the air drains off an ice cap:
	// the winds off Antarctica's coast blow at ten or twenty day in and day out.
	// It is full on a slope of katabaticSlope and in air katabaticCold degrees
	// under freezing or colder over the year, and it turns katabaticTurn radians to the right
	// of downhill in the north and to the left in the south.
	katabatic      = 12.0
	katabaticSlope = 0.004
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
	*airEnv
	// u is the wind toward the east and v toward the north, in metres a
	// second; p is the pressure at sea level in hPa.
	u, v, p [phases][]float32
	// budget is the water in the air in each phase, as the rain was last
	// worked out over the wind: see vapour.go.
	budget [phases]vapourOut
}

// airEnv is the ground as the air reads it: the lattice of air cells and
// everything about the ground under each that the wind is worked out from.
type airEnv struct {
	cell, w, h int // tiles to a cell's side, and cells across and down
	wrap       bool

	lat  []float64 // the latitude of each row of cells on the planet, degrees
	hemi []float64 // how much of the temperate year's swing a row's air has, signed by hemisphere
	mean []float64 // the year's mean temperature at sea level on each row
	dx   []float64 // metres across a cell along each row
	dy   float64   // and down one
	f    []float64 // the Coriolis parameter on each row, per second

	sea    []float64 // how much of each cell lies under the water the air takes its fill from
	cont   []float64 // how much of the country round each cell is land
	height []float64 // the mean height of each cell above that water, metres
	gx, gy []float64 // the lie of the smoothed ground, metres a metre, rising east and north
	rough  []float64 // how broken the ground in each cell is, metres
	expose []float64 // how far each cell stands over the country round it, metres
	depth  []float64 // how deep the air near the ground is over each cell, metres
	// climb is the most ground within blockReach of each cell stands over it,
	// in metres, whichever way the wind comes: no wind faster than buoyancy
	// times this is blocked there, and the looking ahead is spared.
	climb []float64

	// warm is how many degrees the sea over each cell stands over its
	// latitude's mean for the currents, and coast what that is worth to the
	// country round it: see currents. Both are nil on a valley.
	warm, coast []float64
}

// airCell is how many tiles a side the air cells over g are.
func airCell(g *Grid, a *Air) int {
	cell := 1
	for float64(2*cell)*a.dy <= airReach*1.2 && g.W%(2*cell) == 0 && g.H%(2*cell) == 0 &&
		g.W/(2*cell) >= airLeast && g.H/(2*cell) >= airLeast {
		cell *= 2
	}
	return cell
}

// airLeast is how few cells a map may be read on either way. A valley is a
// couple of kilometres of ground to a cell rather than the eighty airReach
// asks for, because a valley is not eighty kilometres across; what it must
// not be is so few cells that the ground has nothing to say to the wind.
const airLeast = 16

// newAirEnv reads the ground of g as the air sees it.
func newAirEnv(g *Grid) *airEnv {
	a := g.air
	cell := airCell(g, a)
	e := &airEnv{cell: cell, w: g.W / cell, h: g.H / cell, wrap: g.Wrap}
	n := e.w * e.h
	e.lat, e.hemi, e.mean, e.dx, e.f = make([]float64, e.h), make([]float64, e.h), make([]float64, e.h), make([]float64, e.h), make([]float64, e.h)
	e.dy = a.dy * 1000 * float64(cell)
	for cy := 0; cy < e.h; cy++ {
		var lat, mean, dx float64
		for y := cy * cell; y < (cy+1)*cell; y++ {
			lat += a.lat[y]
			mean += a.mean[y]
			dx += a.dx[y]
		}
		k := float64(cell)
		lat, mean, dx = lat/k, mean/k, dx/k
		// The wind keeps the year its rivers were calibrated on, the temperate
		// swing capped at Temperate's, and not solarSwing's: read at the
		// growing swing, the high latitudes' continents drove thermal lows
		// hard enough to move the rain, and the small globe at twice the
		// resolution cut its channels to a concavity of 0.14 against 0.24,
		// breaking the resolution yardstick. The ground's year and the air's
		// share seasonTemp and differ only in this factor poleward of
		// Temperate; bringing them together is a question for the water.
		e.hemi[cy] = math.Copysign(math.Min(1, math.Abs(lat)/Temperate), lat)
		if !g.Wrap {
			// A valley is one latitude's weather, but the planet under it
			// is still round: the pressure the belts lay down still falls
			// across it from south to north, or the air would not move.
			lat -= (float64(cy) + 0.5 - float64(e.h)/2) * e.dy / 111195
		}
		e.lat[cy], e.mean[cy] = lat, mean
		e.dx[cy] = dx * 1000 * k
		e.f[cy] = 2 * omega * math.Sin(lat*math.Pi/180)
	}

	// The ground, tile by tile, then gathered into cells.
	base := math.Max(0, g.base)
	above := make([]float64, len(g.Tiles))
	wet := make([]float64, len(g.Tiles))
	broken := make([]float64, len(g.Tiles))
	g.EachRow(func(y int) {
		for x := 0; x < g.W; x++ {
			i := y*g.W + x
			above[i] = math.Max(0, g.Tiles[i].Height-base)
			if g.sunk(i) {
				wet[i] = 1
			}
		}
	})
	g.EachRow(func(y int) {
		for x := 0; x < g.W; x++ {
			i := y*g.W + x
			var sum, sq, k float64
			for dy := -1; dy <= 1; dy++ {
				yy := y + dy
				if yy < 0 || yy >= g.H {
					continue
				}
				for dx := -1; dx <= 1; dx++ {
					xx := x + dx
					if g.Wrap {
						xx = (xx + g.W) % g.W
					} else if xx < 0 || xx >= g.W {
						continue
					}
					h := above[yy*g.W+xx]
					sum, sq, k = sum+h, sq+h*h, k+1
				}
			}
			m := sum / k
			broken[i] = math.Sqrt(math.Max(0, sq/k-m*m))
		}
	})
	e.height, e.sea, e.rough = e.gather(g, above), e.gather(g, wet), e.gather(g, broken)

	land := make([]float64, n)
	for i := range land {
		land[i] = 1 - e.sea[i]
	}
	e.cont = e.blur(land, contReach)
	smooth := e.blurCells(e.height, 1)
	e.expose = make([]float64, n)
	round := e.blurCells(e.height, exposeReach)
	e.gx, e.gy, e.depth = make([]float64, n), make([]float64, n), make([]float64, n)
	for cy := 0; cy < e.h; cy++ {
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			e.gx[i], e.gy[i] = e.grad(smooth, cx, cy)
			e.expose[i] = e.height[i] - round[i]
			e.depth[i] = layerDepth * math.Exp(-e.height[i]/layerScale)
		}
	}
	e.climb = e.climbs()
	return e
}

// climbs is climb for every cell: the highest ground in the square of cells a
// wind at the cell could look ahead over, less the cell's own.
func (e *airEnv) climbs() []float64 {
	n := e.w * e.h
	stepKm := blockReach / blockSteps
	ry := int(math.Ceil(math.Min(stepKm*1000/e.dy, blockCells)*blockSteps)) + 1
	mid := make([]float64, n)
	e.rows(func(cy int) {
		rx := int(math.Ceil(math.Min(stepKm*1000/e.dx[cy], blockCells)*blockSteps)) + 1
		rx = min(rx, e.w)
		for cx := 0; cx < e.w; cx++ {
			top := 0.0
			for k := -rx; k <= rx; k++ {
				top = math.Max(top, e.height[e.at(cx+k, cy)])
			}
			mid[cy*e.w+cx] = top
		}
	})
	out := make([]float64, n)
	e.rows(func(cy int) {
		for cx := 0; cx < e.w; cx++ {
			top := 0.0
			for k := max(cy-ry, 0); k <= min(cy+ry, e.h-1); k++ {
				top = math.Max(top, mid[k*e.w+cx])
			}
			i := cy*e.w + cx
			out[i] = top - e.height[i]
		}
	})
	return out
}

// gather is the mean of a reading of g's tiles over each air cell.
func (e *airEnv) gather(g *Grid, v []float64) []float64 {
	if e.cell == 1 {
		return append([]float64(nil), v...)
	}
	out := make([]float64, e.w*e.h)
	k := float64(e.cell * e.cell)
	for cy := 0; cy < e.h; cy++ {
		for cx := 0; cx < e.w; cx++ {
			var s float64
			for y := cy * e.cell; y < (cy+1)*e.cell; y++ {
				for x := cx * e.cell; x < (cx+1)*e.cell; x++ {
					s += v[y*g.W+x]
				}
			}
			out[cy*e.w+cx] = s / k
		}
	}
	return out
}

// rows runs f for every row of cells, spread over goroutines where there are
// cells enough to be worth it, under the same rule as Grid.EachRow: f writes
// only at its own row's cells.
func (e *airEnv) rows(f func(cy int)) {
	if e.w*e.h < spreadTiles {
		for cy := 0; cy < e.h; cy++ {
			f(cy)
		}
		return
	}
	InParallel(e.h, WorkersFor(e.h), func(cy, _ int) { f(cy) })
}

// at is the cell cx, cy, with the column taken round the seam or held at the
// edge and the row held at the poles or the edge.
func (e *airEnv) at(cx, cy int) int {
	if uint(cx) < uint(e.w) && uint(cy) < uint(e.h) {
		return cy*e.w + cx
	}
	if e.wrap {
		cx = ((cx % e.w) + e.w) % e.w
	} else {
		cx = min(max(cx, 0), e.w-1)
	}
	cy = min(max(cy, 0), e.h-1)
	return cy*e.w + cx
}

// grad is the slope of v at a cell, per metre, rising toward the east and
// toward the north.
func (e *airEnv) grad(v []float64, cx, cy int) (east, north float64) {
	east = (v[e.at(cx+1, cy)] - v[e.at(cx-1, cy)]) / (2 * e.dx[cy])
	north = (v[e.at(cx, cy-1)] - v[e.at(cx, cy+1)]) / (2 * e.dy)
	return east, north
}

// sample is v read between the cells, at a place given in cells.
func (e *airEnv) sample(v []float64, fx, fy float64) float64 {
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
func (e *airEnv) blur(v []float64, reach float64) []float64 {
	across := make([]int, e.h)
	for cy := range across {
		across[cy] = int(math.Round(reach / (e.dx[cy] / 1000)))
	}
	return e.box(v, across, int(math.Round(reach/(e.dy/1000))))
}

// blurCells is v averaged over the square r cells either way of each cell.
func (e *airEnv) blurCells(v []float64, r int) []float64 {
	across := make([]int, e.h)
	for cy := range across {
		across[cy] = r
	}
	return e.box(v, across, r)
}

// box is the running mean of v, across[cy] cells either way along each row
// and down cells either way down each column, clipped at the edges and taken
// round the seam.
func (e *airEnv) box(v []float64, across []int, down int) []float64 {
	mid := make([]float64, len(v))
	e.rows(func(cy int) {
		r := across[cy]
		row := cy * e.w
		if e.wrap && 2*r+1 >= e.w {
			var s float64
			for cx := 0; cx < e.w; cx++ {
				s += v[row+cx]
			}
			for cx := 0; cx < e.w; cx++ {
				mid[row+cx] = s / float64(e.w)
			}
			return
		}
		var s float64
		var k int
		for cx := -r; cx <= r; cx++ {
			if e.wrap || (cx >= 0 && cx < e.w) {
				s += v[e.at(cx, cy)]
				k++
			}
		}
		for cx := 0; cx < e.w; cx++ {
			mid[row+cx] = s / float64(k)
			out, in := cx-r, cx+r+1
			if e.wrap || out >= 0 {
				s -= v[e.at(out, cy)]
				k--
			}
			if e.wrap || in < e.w {
				s += v[e.at(in, cy)]
				k++
			}
		}
	})
	out := make([]float64, len(v))
	down = min(down, e.h)
	for cx := 0; cx < e.w; cx++ {
		var s float64
		var k int
		for cy := 0; cy <= down && cy < e.h; cy++ {
			s += mid[cy*e.w+cx]
			k++
		}
		for cy := 0; cy < e.h; cy++ {
			out[cy*e.w+cx] = s / float64(k)
			if leave := cy - down; leave >= 0 {
				s -= mid[leave*e.w+cx]
				k--
			}
			if enter := cy + down + 1; enter < e.h {
				s += mid[enter*e.w+cx]
				k++
			}
		}
	}
	return out
}

// windsFor works out the climate of the wind over g as its ground now lies.
// The phases are independent of one another and are worked out side by side;
// each writes only its own slices.
func windsFor(g *Grid) *Winds {
	e := newAirEnv(g)
	w := &Winds{airEnv: e}
	n := e.w * e.h
	for k := range phases {
		w.u[k], w.v[k], w.p[k] = make([]float32, n), make([]float32, n), make([]float32, n)
	}
	workers := 1
	if n >= spreadTiles {
		workers = WorkersFor(phases)
	}
	// The two equinoxes are the same day to the air, so the autumn's is the
	// spring's.
	InParallel(phases-1, workers, func(k, _ int) {
		e.solve(phaseSin[k], e.airTemp(phaseSin[k]), nil, nil, w.u[k], w.v[k], w.p[k])
	})
	copy(w.u[3], w.u[1])
	copy(w.v[3], w.v[1])
	copy(w.p[3], w.p[1])
	// The water under the year's wind, on a globe. See ocean.go.
	if e.wrap {
		e.warm = e.currents(w.u, w.v)
		e.coast = e.coastal(e.warm)
	}
	return w
}

// airTemp is the temperature of the air at sea level over each cell in an
// ordinary year, sinT of the way into the north's summer. The phases of the
// wind's year are its thermal seasons - the warmest, the coldest and the turn
// between - so each cell is read at the crest of its own swing, whatever its
// lag behind the sun. The swing is the one Land.TempAt reads: see seasonTemp.
func (e *airEnv) airTemp(sinT float64) []float64 {
	temp := make([]float64, e.w*e.h)
	for cy := 0; cy < e.h; cy++ {
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			temp[i] = e.mean[cy] + seasonTemp(e.hemi[cy], sinT, e.cont[i])
		}
	}
	return temp
}

// airTempOn is airTemp on a day of the calendar: each cell at its own place in
// its swing, lagging the sun by as much as the land round it makes it lag. A
// valley's year has no lag, and is airTemp at the day's sun to the bit.
func (e *airEnv) airTempOn(day int) []float64 {
	if !e.wrap {
		return e.airTemp(yearSin(day))
	}
	temp := make([]float64, e.w*e.h)
	for cy := 0; cy < e.h; cy++ {
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			temp[i] = e.mean[cy] + seasonTemp(e.hemi[cy], seasonAt(day, lagAt(e.cont[i])), e.cont[i])
		}
	}
	return temp
}

// solve works out the wind over the cells with the year sinT of the way into
// the north's summer, over air at sea level of temp degrees. extra is pressure added to what the climate lays down,
// in hPa, and warm the degrees the day's weather has carried in; either may be
// nil. The wind and the pressure are written to u, v and p.
func (e *airEnv) solve(sinT float64, temp, extra, warm []float64, u, v, p []float32) {
	n := e.w * e.h

	// The warmth of the air at sea level, and the pressure it and the belts
	// make between them.
	temp = append([]float64(nil), temp...)
	if warm != nil {
		for i := range temp {
			temp[i] += warm[i] * warmGain / thermalGain
		}
	}
	temp = e.blur(e.blur(temp, synopticReach), synopticReach)
	pres := make([]float64, n)
	for cy := 0; cy < e.h; cy++ {
		row := cy * e.w
		var zonal float64
		for cx := 0; cx < e.w; cx++ {
			zonal += temp[row+cx]
		}
		zonal /= float64(e.w)
		belt := beltPressure(e.lat[cy], sinT)
		for cx := 0; cx < e.w; cx++ {
			i := row + cx
			pres[i] = belt - thermalGain*(temp[i]-zonal)
			if extra != nil {
				pres[i] += extra[i]
			}
		}
	}

	// The wind the pressure drives, against the turning of the planet and the
	// drag of the ground; then what the ground in its way does to it.
	free := [2][]float64{make([]float64, n), make([]float64, n)}
	wind := [2][]float64{make([]float64, n), make([]float64, n)}
	e.rows(func(cy int) {
		f := e.f[cy]
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			px, py := e.grad(pres, cx, cy)
			px, py = px*100/airDensity, py*100/airDensity // hPa to Pa, and a force on a kilogram
			land := 1 - e.sea[i]
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
			uu, vv = e.ground(cx, cy, uu, vv)
			wind[0][i], wind[1][i] = uu, vv
		}
	})
	e.channel(free, wind)

	for i := 0; i < n; i++ {
		uu, vv := wind[0][i], wind[1][i]
		if s := math.Hypot(uu, vv); s > windMost {
			uu, vv = uu*windMost/s, vv*windMost/s
		}
		u[i], v[i], p[i] = float32(uu), float32(vv), float32(pres[i])
	}
}

// ground is the wind uu, vv at a cell after the ground there has had its say:
// turned aside by a range it cannot climb, quickened on a crest and slowed in
// a hollow, and joined by the air draining off the ice.
func (e *airEnv) ground(cx, cy int, uu, vv float64) (float64, float64) {
	i := cy*e.w + cx
	gx, gy := e.gx[i], e.gy[i]
	slope := math.Hypot(gx, gy)

	// Blocking. The ground ahead along the wind, and how much of it the wind
	// would have to climb; where it is too slow to climb it, the part of it
	// blowing up the slope is turned along the slope instead.
	if s := math.Hypot(uu, vv); s > 0.1 && slope > 1e-6 && s < buoyancy*e.climb[i] {
		ex, ny := uu/s, vv/s
		stepKm := blockReach / blockSteps
		sx := math.Min(stepKm*1000/e.dx[cy], blockCells) // cells a step along the row
		sy := math.Min(stepKm*1000/e.dy, blockCells)
		here, top := e.height[i], e.height[i]
		for k := 1; k <= blockSteps; k++ {
			h := e.sample(e.height, float64(cx)+ex*sx*float64(k), float64(cy)-ny*sy*float64(k))
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

	// Exposure.
	k := math.Max(0.5, math.Min(1.5, 1+0.5*e.expose[i]/exposeHeight))
	uu, vv = uu*k, vv*k

	// The ice's own wind.
	if slope > 1e-6 {
		cold := math.Max(0, math.Min(1, (Lapse*e.height[i]-e.mean[cy])/katabaticCold))
		if cold > 0 {
			s := katabatic * cold * math.Min(1, slope/katabaticSlope)
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
func (e *airEnv) channel(free, wind [2][]float64) {
	n := e.w * e.h
	fu, fv := make([]float64, n), make([]float64, n)
	for i := range fu {
		fu[i], fv[i] = e.depth[i]*wind[0][i], e.depth[i]*wind[1][i]
	}
	// push is how much more air the ground made converge on each cell than
	// the free wind did: the divergence of the flux the air near the ground
	// now has, less that of the free wind at the depth over the sea.
	push := make([]float64, n)
	e.rows(func(cy int) {
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			push[i] = e.div(fu, fv, cx, cy) - layerDepth*e.div(free[0], free[1], cx, cy)
		}
	})
	// Relaxed red and black in turn, over-relaxed: each pass writes the cells
	// of one colour and reads only those of the other, so the rows can be
	// taken on as many goroutines as there are and still come out the same.
	lam := make([]float64, n)
	for range channelRounds {
		for colour := range 2 {
			e.rows(func(cy int) {
				ax := 1 / (e.dx[cy] * e.dx[cy])
				ay := 1 / (e.dy * e.dy)
				norm := 1 / (2*ax + 2*ay)
				row := cy * e.w
				north, south := max(cy-1, 0)*e.w, min(cy+1, e.h-1)*e.w
				for cx := (cy + colour) % 2; cx < e.w; cx += 2 {
					west, east := cx-1, cx+1
					switch {
					case west < 0 && e.wrap:
						west = e.w - 1
					case west < 0:
						west = 0
					}
					switch {
					case east == e.w && e.wrap:
						east = 0
					case east >= e.w:
						east = e.w - 1
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
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			gx, gy := e.grad(lam, cx, cy)
			wind[0][i] = (fu[i] - gx) / e.depth[i]
			wind[1][i] = (fv[i] - gy) / e.depth[i]
		}
	})
}

// div is the divergence of the field fu, fv at a cell, per metre, with the
// parallels shortening toward the poles.
func (e *airEnv) div(fu, fv []float64, cx, cy int) float64 {
	east := (fu[e.at(cx+1, cy)] - fu[e.at(cx-1, cy)]) / (2 * e.dx[cy])
	nr, sr := max(cy-1, 0), min(cy+1, e.h-1)
	north := (fv[e.at(cx, nr)]*e.dx[nr] - fv[e.at(cx, sr)]*e.dx[sr]) / (2 * e.dy * e.dx[cy])
	return east + north
}

// seasonWeights is how much each phase of the year is worth on a day: the
// year's mean and its first harmonic, read off the four phases, which lie a
// quarter of a year apart.
func seasonWeights(day int) [phases]float64 {
	th := 2 * math.Pi * float64(day) / Year
	s, c := math.Sin(th), math.Cos(th)
	// A field over the year is m + S sin + C cos, and the phases are its values
	// at sin -1, cos 1, sin 1 and cos -1.
	return [phases]float64{0.25 - s/2, 0.25 + c/2, 0.25 + s/2, 0.25 - c/2}
}

// cellAt is where tile i's centre lies among the air cells, in cells.
func (e *airEnv) cellAt(g *Grid, i int) (fx, fy float64) {
	x, y := i%g.W, i/g.W
	k := float64(e.cell)
	return (float64(x)+0.5)/k - 0.5, (float64(y)+0.5)/k - 0.5
}

// sample32 is sample for a reading kept in single precision.
func (e *airEnv) sample32(v []float32, fx, fy float64) float64 {
	x0, y0 := math.Floor(fx), math.Floor(fy)
	tx, ty := fx-x0, fy-y0
	x, y := int(x0), int(y0)
	at := func(cx, cy int) float64 { return float64(v[e.at(cx, cy)]) }
	a := at(x, y) + (at(x+1, y)-at(x, y))*tx
	b := at(x, y+1) + (at(x+1, y+1)-at(x, y+1))*tx
	return a + (b-a)*ty
}

// WindOn is the wind near the ground on tile i on a day of the year, in
// metres a second toward the east and toward the north: the climate of the
// wind, what it is on that day in an ordinary year. It is nothing on a map
// whose weather has not been read.
func (g *Grid) WindOn(i, day int) (east, north float64) {
	w := g.winds
	if w == nil || i < 0 || i >= len(g.Tiles) {
		return 0, 0
	}
	fx, fy := w.cellAt(g, i)
	for k, m := range seasonWeights(day) {
		east += m * w.sample32(w.u[k], fx, fy)
		north += m * w.sample32(w.v[k], fx, fy)
	}
	return east, north
}

// MeanWind is the wind near the ground on tile i over the whole year, in
// metres a second toward the east and toward the north.
func (g *Grid) MeanWind(i int) (east, north float64) {
	w := g.winds
	if w == nil || i < 0 || i >= len(g.Tiles) {
		return 0, 0
	}
	fx, fy := w.cellAt(g, i)
	for k := range phases {
		east += w.sample32(w.u[k], fx, fy) / phases
		north += w.sample32(w.v[k], fx, fy) / phases
	}
	return east, north
}

// PressureOn is the pressure at sea level over tile i on a day of the year,
// in hPa, in an ordinary year. It is nothing on a map whose weather has not
// been read.
func (g *Grid) PressureOn(i, day int) float64 {
	w := g.winds
	if w == nil || i < 0 || i >= len(g.Tiles) {
		return 0
	}
	fx, fy := w.cellAt(g, i)
	var p float64
	for k, m := range seasonWeights(day) {
		p += m * w.sample32(w.p[k], fx, fy)
	}
	return p
}
