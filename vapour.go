package terra

import (
	"math"

	"github.com/LukasSelin/terra/internal/kernel"
)

// The water in the column of air over each cell, kept as a budget.
//
// The rain was a belt's figure, read off the latitude, times how much of the
// sea's moisture the air still carried and how hard it gathered - each of the
// three a share, tuned against the rain the real world's belts have. Nothing
// had to add up: the sea gave no water and the rain took none.
//
// Here the water is counted. W, the water in a column of air in kg/m², obeys
//
//	∂W/∂t + ∇·(W V) = E - P
//
// on the air cells, for each phase of the year in its steady state: it is
// carried by the wind, taken up off the sea by the bulk formula and off the
// land by what the land's rain and warmth let it send back, and rained out as
// the column nears saturation, where the air near the ground gathers and goes
// up, and where the ground lifts it (orographic.go). What falls anywhere was
// taken up somewhere, and over a globe the two come to the same.
//
// The flux is taken across the faces between cells, from the cell upwind of
// each, so that what leaves one cell is exactly what enters the next.
//
// The wind near the ground is not the wind of the whole column. It gathers
// under the rising air of the equator and spreads under the sinking air of the
// horse latitudes, and the air it gathers goes up and comes back aloft. So
// only the water of the air near the ground, convLayer of the column, is
// carried by that wind as it is; the water above it goes with the part of the
// wind that turns and does not gather - the wind less the gradient of a
// potential whose Laplacian is its gathering, Helmholtz's split of a flow.
// What the air near the ground gathers goes up and rains out (Kuo, 1974), so
// the air gathering over a cell brings its water to rain and not to pile up.

// The column and the sea's evaporation.
const (
	// vapourHeight is the scale height of the water in the air, in metres:
	// the column holds the water of vapourHeight metres of air at the
	// humidity it has at the ground. Some two kilometres (Peixoto and Oort,
	// 1992, ch. 12: 1.5-2.5 km).
	vapourHeight = 2000.0
	// vapourGas is the gas constant of water vapour, J/kg/K.
	vapourGas = 461.5
	// exchangeCoeff is C_E, the bulk transfer coefficient of water between a
	// sea and the wind over it (Large and Pond, 1982: 1.2e-3 in moderate
	// winds).
	exchangeCoeff = 1.2e-3
	// gustLeast is the least wind, in metres a second, the sea's evaporation
	// is taken at: a mean wind of nothing is a wind of eddies and gusts, and
	// the sea gives its water to those (Miller and others, 1992, on the
	// gustiness a model's calm needs).
	gustLeast = 3.0
	// coldest is the coldest air, in degrees, the air's water is read at:
	// colder than this a column holds nothing worth counting, and the formula
	// for its vapour has a pole at -243.5.
	coldest = -90.0
)

// The rain of a column. Bretherton, Peters and Back (2004), from four years of
// satellite readings over the tropical oceans, found the rain of a column to
// go as the exponential of how near saturation the whole column is:
// P = exp(rainSteep·(r - rainHalf)) mm a day, r being W over the water the
// column would hold saturated. Read as a timescale, W/P, that is some ten
// days at r of seven tenths, the planet's mean residence time of its water
// (Trenberth, 1998), two days at eight tenths and a month at six. It was
// fitted over columns that hold some rainColumn kg/m² saturated; a colder
// column rains in proportion to what it can hold.
const (
	rainSteep  = 15.6
	rainHalf   = 0.603
	rainColumn = 65.0
)

// The rest of the budget.
const (
	// boundaryHumidity is how near saturation the air coming in over the edge
	// of a map that is not a globe is: maritime air, straight off the sea.
	boundaryHumidity = 0.7
	// vapourRounds is the most rounds of four sweeps the columns are given to
	// settle, and vapourSettled the mean change in kg/m² over the cells in a
	// round that is settled.
	vapourRounds  = 4
	vapourSettled = 3e-2
	// eddyVapour is K, m²/s, how fast the storms of the middle latitudes mix
	// the water of the air down its gradient on a globe, which no wind of an
	// ordinary year carries: the transient eddies' poleward flux of latent heat,
	// a petawatt at forty degrees (Peixoto and Oort, 1992, ch. 13), is some
	// thirteen kilograms of water a metre of the parallel a second against a
	// fall of fifteen kg/m² of column over the twenty-five degrees from the
	// subtropics to the storm tracks: 2.4e6. A valley is smaller than an eddy.
	eddyVapour = 2.4e6
	// recycleRounds is how many times the land's rain and what it sends back
	// to the air are worked out against each other.
	recycleRounds = 2
)

// convLayer is the share of the column's water in the air near the ground,
// layerDepth deep under a humidity falling off over vapourHeight.
var convLayer = 1 - math.Exp(-layerDepth/vapourHeight)

// saturatedColumn is the water, kg/m², a column of air at temp degrees at its
// foot holds saturated: the vapour density of saturation there by the
// Clausius-Clapeyron relation in Bolton's (1980) form, over vapourHeight.
func saturatedColumn(temp float64) float64 {
	temp = math.Max(temp, coldest)
	e := 611.2 * math.Exp(17.67*temp/(temp+243.5)) // Pa
	return e / (vapourGas * (temp + 273.15)) * vapourHeight
}

// columnRain is the rain, kg/m²/s, of a column holding w of the ws it would
// hold saturated: Bretherton's, less what his curve gives an empty column so
// that nothing rains out of nothing.
func columnRain(w, ws float64) float64 {
	p, _ := columnRainSlope(w, ws)
	return p
}

// rainEmpty is what Bretherton's curve gives a column with nothing in it.
var rainEmpty = math.Exp(-rainSteep * rainHalf)

// columnRainSlope is columnRain and its rate of change with w. Bretherton's
// curve is read to saturation and no further: past it, what a column carried
// into air too cold to hold it has over saturation falls out at the rate the
// curve reaches there, in a quarter of an hour.
func columnRainSlope(w, ws float64) (p, slope float64) {
	r := w / ws
	ex := rainCurve(r)
	scale := ws / rainColumn / 86400
	p = scale * (ex - rainEmpty)
	slope = scale * rainSteep / ws * ex
	if r > rainMost {
		p += slope * (w - rainMost*ws)
	}
	return p, slope
}

// rainMost is how near saturation the curve is read to, and rainCurve
// exp(rainSteep·(r - rainHalf)) read off a table of it: the budget asks for it
// millions of times a map, and the table's straight lines between its entries
// are within a part in thirty thousand of it.
const (
	rainMost  = 1.0
	rainSteps = 1000
)

var rainTable = func() []float64 {
	t := make([]float64, rainSteps+2)
	for k := range t {
		t[k] = math.Exp(rainSteep * (float64(k)*rainMost/rainSteps - rainHalf))
	}
	return t
}()

func rainCurve(r float64) float64 {
	f := math.Max(0, math.Min(r, rainMost)) * rainSteps / rainMost
	k := int(f)
	return rainTable[k] + (rainTable[k+1]-rainTable[k])*(f-float64(k))
}

// vapourIn is what one phase's budget is worked out from, on the cells.
type vapourIn struct {
	u, v     []float32 // the wind near the ground, m/s
	temp     []float64 // the air at sea level, degrees
	sst      []float64 // the sea's surface, degrees
	landEvap []float64 // what the land sends up, kg/m²/s
	stable   []float64 // how much of its rain air held down by cold water keeps; nil for all
	oro      []float64 // what the ground's lift would wring out, kg/m²/s; nil for none
	w        []float64 // where to start the columns from; nil for a fresh start
}

// vapourOut is one phase's settled budget, on the cells: the water in each
// column in kg/m², and in kg/m²/s what it took up, what it rained out - as a
// column and where the air gathered - and what of the ground's lift it could
// give.
type vapourOut struct {
	w, evap, rain, oro []float64
	// sat is the water each column would hold saturated, kg/m².
	sat []float64
}

// vapourCell is one cell's equation for its column's water, as the sweeps
// read it: what it is given and loses whatever its water, where its water
// comes from and how hard, and its rain curve. See vapour.
type vapourCell struct {
	give, lose        float64
	toStep, rainScale float64
	slope             float64
	from              [4]int32
	share             [4]float64
}

// vapour settles one phase's budget.
func (e *airEnv) vapour(in vapourIn) vapourOut {
	defer phase("airEnv.vapour")()
	n := e.w * e.h
	dy := e.dy
	f := e.vapourFluxes(in.u, in.v)
	east, north, gather := f.east, f.north, f.gather
	westOf, southOf := f.westOf, f.southOf

	// Each cell's equation, whatever W is: what it is given, and at what
	// rate it loses its own water to the faces, to the sea and to the air
	// gathering; its neighbours upwind are read as the sweeps go.
	// Each is laid out in a vapourCell, so that a visit reads one run of
	// memory.
	cells := make([]vapourCell, n)
	seaA := make([]float64, n)  // the sea's evaporation at W of nothing, kg/m²/s
	seaB := make([]float64, n)  // and what each kg/m² of W takes off it, a second
	satW := make([]float64, n)  // the saturated column
	rainK := make([]float64, n) // what of the column's rain the air keeps
	for cy := 0; cy < e.h; cy++ {
		area := e.dx[cy] * dy
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			// The column is read against the air at sea level: what the
			// ground's height does to the air climbing it is the ground's lift
			// to wring out (orographic.go), and not the column's to rain again.
			ws := saturatedColumn(in.temp[i])
			satW[i] = ws
			speed := math.Max(gustLeast, math.Hypot(float64(in.u[i]), float64(in.v[i])))
			// The bulk formula, with the humidity at the ground the column's
			// water over its scale height.
			bulk := airDensity * exchangeCoeff * speed * e.sea[i]
			seaA[i] = bulk * saturation(in.sst[i])
			seaB[i] = bulk / (airDensity * vapourHeight)
			cells[i].give = seaA[i] + (1-e.sea[i])*in.landEvap[i]
			out := math.Max(0, east[i]) + math.Max(0, -westOf(cx, cy)) + math.Max(0, north[i]) + math.Max(0, -southOf(cx, cy))
			cells[i].lose = out/area + seaB[i] + gather[i]
			if !e.wrap {
				// Air coming in over the edge brings the sea's water with it.
				bnd := boundaryHumidity * ws
				if cx == e.w-1 {
					cells[i].give += math.Max(0, -east[i]) / area * bnd
				}
				if cx == 0 {
					cells[i].give += math.Max(0, westOf(cx, cy)) / area * bnd
				}
				if cy == 0 {
					cells[i].give += math.Max(0, -north[i]) / area * bnd
				}
				if cy == e.h-1 {
					cells[i].give += math.Max(0, southOf(cx, cy)) / area * bnd
				}
			}
			rainK[i] = 1
			if in.stable != nil {
				rainK[i] = in.stable[i]
			}
		}
	}

	w := make([]float64, n)
	if in.w != nil {
		copy(w, in.w)
	} else {
		for i := range w {
			w[i] = boundaryHumidity * satW[i]
		}
	}
	// Where each cell's water comes from: up to four cells upwind, and the
	// part a second of each one's water that crosses into it.
	const none = -1
	for cy := 0; cy < e.h; cy++ {
		area := e.dx[cy] * dy
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			c := &cells[i]
			c.from = [4]int32{none, none, none, none}
			if f := westOf(cx, cy); f > 0 && (cx > 0 || e.wrap) {
				c.from[0], c.share[0] = int32(e.at(cx-1, cy)), f/area
			}
			if f := east[i]; f < 0 && (cx+1 < e.w || e.wrap) {
				c.from[1], c.share[1] = int32(e.at(cx+1, cy)), -f/area
			}
			if f := southOf(cx, cy); f > 0 && cy+1 < e.h {
				c.from[2], c.share[2] = int32(i+e.w), f/area
			}
			if f := north[i]; f < 0 && cy > 0 {
				c.from[3], c.share[3] = int32(i-e.w), -f/area
			}
			if e.wrap {
				// The eddies' share, both ways across every face.
				across := eddyVapour / (e.dx[cy] * e.dx[cy])
				c.from[0], c.share[0] = int32(e.at(cx-1, cy)), c.share[0]+across
				c.from[1], c.share[1] = int32(e.at(cx+1, cy)), c.share[1]+across
				c.lose += 2 * across
				if cy+1 < e.h {
					d := eddyVapour * 0.5 * (e.dx[cy] + e.dx[cy+1]) / dy / area
					c.from[2], c.share[2] = int32(i+e.w), c.share[2]+d
					c.lose += d
				}
				if cy > 0 {
					d := eddyVapour * 0.5 * (e.dx[cy] + e.dx[cy-1]) / dy / area
					c.from[3], c.share[3] = int32(i-e.w), c.share[3]+d
					c.lose += d
				}
			}
		}
	}
	// The rain curve of each column, kept ready: how far along its table a
	// kg/m² of water moves it, what the column's rain is scaled by, and the
	// part of its slope that does not depend on the water. Every product is
	// the one the sweep used to take, in the order it took it, so the sweeps
	// settle on the same bits.
	for i := range cells {
		c := &cells[i]
		c.toStep = rainSteps / rainMost / satW[i]
		c.rainScale = rainK[i] * satW[i] / rainColumn / 86400
		c.slope = c.rainScale * rainSteep * c.toStep * rainMost / rainSteps
	}
	taken := make([]float64, n)
	type order struct{ x0, x1, dx, y0, y1, dy int }
	orders := [4]order{
		{0, e.w, 1, 0, e.h, 1}, {e.w - 1, -1, -1, 0, e.h, 1},
		{0, e.w, 1, e.h - 1, -1, -1}, {e.w - 1, -1, -1, e.h - 1, -1, -1},
	}
	// Each visit to a cell takes one step of Newton's method on its own
	// equation, lose·w + keep·P(w) = what it is given, with its neighbours
	// as they stand: the left side only grows with w and is convex, so the
	// steps never run away.
	for round := 0; round < vapourRounds; round++ {
		most := 0.0
		for _, o := range orders {
			for cy := o.y0; cy != o.y1; cy += o.dy {
				for cx := o.x0; cx != o.x1; cx += o.dx {
					i := cy*e.w + cx
					c := &cells[i]
					sum := c.give
					for j, from := range c.from {
						if from != none {
							sum += c.share[j] * w[from]
						}
					}
					if in.oro != nil {
						// The ground's lift wrings out what it would, or all
						// the column is given if that is less. The builtin
						// min and max are math.Min and math.Max to the bit,
						// and are not a call.
						taken[i] = max(0, min(in.oro[i], sum))
						sum -= taken[i]
					}
					// columnRainSlope, written out for the few million times
					// a map asks it.
					wi := w[i]
					f := wi * c.toStep
					if f > rainSteps {
						f = rainSteps
					}
					kk := int(f)
					ex := rainTable[kk] + (rainTable[kk+1]-rainTable[kk])*(f-float64(kk))
					p := c.rainScale * (ex - rainEmpty)
					dp := c.slope * ex
					if over := wi - rainMost*satW[i]; over > 0 {
						p += dp * over
					}
					next := wi - (c.lose*wi+p-sum)/(c.lose+dp)
					if next < 0 {
						next = 0
					}
					most += math.Abs(next - w[i])
					w[i] = next
				}
			}
		}
		if e.wrap && e.h > 2 {
			e.zonalCorrection(w, cells, in.oro, satW)
		}
		if most < vapourSettled*float64(n) {
			break
		}
	}

	out := vapourOut{w: w, evap: make([]float64, n), rain: make([]float64, n), oro: taken, sat: satW}
	for i := range w {
		out.evap[i] = seaA[i] - seaB[i]*w[i] + (1-e.sea[i])*in.landEvap[i]
		out.rain[i] = rainK[i]*columnRain(w[i], satW[i]) + gather[i]*w[i]
	}
	return out
}

// zonalCorrection moves each row of columns by as much as its row's budget,
// taken together, is still out: the slow part of the sweeps' settling is the
// mixing of the water between the latitudes, which a sweep moves a cell at a
// time, and one tridiagonal solve down the rows moves it all at once. It is a
// coarse grid of one cell a row under the sweeps.
func (e *airEnv) zonalCorrection(w []float64, cells []vapourCell, oro, satW []float64) {
	const none = -1
	h := e.h
	diag, up, down, res := make([]float64, h), make([]float64, h), make([]float64, h), make([]float64, h)
	for cy := 0; cy < h; cy++ {
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			c := &cells[i]
			sum := c.give
			for j, from := range c.from {
				if from != none {
					sum += c.share[j] * w[from]
				}
			}
			if oro != nil {
				sum -= max(0, min(oro[i], sum))
			}
			wi := w[i]
			f := min(wi*c.toStep, rainSteps)
			kk := int(f)
			ex := rainTable[kk] + (rainTable[kk+1]-rainTable[kk])*(f-float64(kk))
			p := c.rainScale * (ex - rainEmpty)
			dp := c.slope * ex
			if over := wi - rainMost*satW[i]; over > 0 {
				p += dp * over
			}
			res[cy] += sum - c.lose*wi - p
			diag[cy] += c.lose + dp
			// A row moved as one gives itself back what crosses along it.
			if c.from[0] != none {
				diag[cy] -= c.share[0]
			}
			if c.from[1] != none {
				diag[cy] -= c.share[1]
			}
			if c.from[2] != none {
				down[cy] += c.share[2]
			}
			if c.from[3] != none {
				up[cy] += c.share[3]
			}
		}
	}
	// diag δ_y - up δ_{y-1} - down δ_{y+1} = res
	for cy := 1; cy < h; cy++ {
		m := -up[cy] / diag[cy-1]
		diag[cy] -= m * -down[cy-1]
		res[cy] -= m * res[cy-1]
	}
	delta := make([]float64, h)
	delta[h-1] = res[h-1] / diag[h-1]
	for cy := h - 2; cy >= 0; cy-- {
		delta[cy] = (res[cy] + down[cy]*delta[cy+1]) / diag[cy]
	}
	for cy := 0; cy < h; cy++ {
		d := delta[cy]
		if d != d || math.IsInf(d, 0) {
			continue
		}
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			w[i] = math.Max(0, w[i]+d)
		}
	}
}

// vapourFluxes is how the wind u, v carries the water of a column across the
// faces of the cells, in m²/s - the face on the east of each cell and the face
// on its north - and how fast, a part a second, the air near the ground
// gathering over each takes the column's water up to rain.
//
// The air near the ground is carried as a layer of the depth it has over each
// cell, read against layerDepth: over high ground less air goes. Of that flux
// the water near the ground goes with all of it, and the water above with the
// part that does not gather (see the remark at the top).
func (e *airEnv) vapourFluxes(u, v []float32) vapourFlux {
	n := e.w * e.h
	dy := e.dy
	east, north := make([]float64, n), make([]float64, n)
	carry := func(i int, s []float32) float64 { return float64(s[i]) * e.depth[i] / layerDepth }
	for cy := 0; cy < e.h; cy++ {
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			switch {
			case cx+1 < e.w:
				east[i] = 0.5 * (carry(i, u) + carry(i+1, u)) * dy
			case e.wrap:
				east[i] = 0.5 * (carry(i, u) + carry(cy*e.w, u)) * dy
			}
			if cy > 0 {
				north[i] = 0.5 * (carry(i, v) + carry(i-e.w, v)) * 0.5 * (e.dx[cy] + e.dx[cy-1])
			}
		}
	}
	// The edges of a map that is not a globe are open: the air crosses each
	// as it crosses the face inside it, so that the edge neither gathers nor
	// spreads it.
	f := vapourFlux{east: east, north: north, wrap: e.wrap, w: e.w, h: e.h}
	if !e.wrap {
		f.west, f.south = make([]float64, e.h), make([]float64, e.w)
		for cy := 0; cy < e.h; cy++ {
			row := cy * e.w
			if e.w > 1 {
				east[row+e.w-1] = east[row+e.w-2]
				f.west[cy] = east[row]
			} else {
				east[row] = carry(row, u) * dy
				f.west[cy] = east[row]
			}
		}
		for cx := 0; cx < e.w; cx++ {
			if e.h > 1 {
				north[cx] = north[e.w+cx]
				f.south[cx] = north[(e.h-1)*e.w+cx]
			} else {
				north[cx] = carry(cx, v) * e.dx[0]
				f.south[cx] = north[cx]
			}
		}
	}
	// What leaves each cell through its faces, net, in m²/s.
	div := make([]float64, n)
	for cy := 0; cy < e.h; cy++ {
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			div[i] = east[i] - f.westOf(cx, cy) + north[i] - f.southOf(cx, cy)
		}
	}
	// The gathering at the scale of the weather, which the air near the
	// ground goes up and rains out, and what is left at the scale of a cell -
	// the air squeezed round a hill, or hurried off the edge of a valley -
	// which goes round rather than up, the whole column with it.
	rate := make([]float64, n)
	for cy := 0; cy < e.h; cy++ {
		area := e.dx[cy] * dy
		for cx := 0; cx < e.w; cx++ {
			rate[cy*e.w+cx] = div[cy*e.w+cx] / area
		}
	}
	rate = e.blur(rate, synopticReach)
	gather := make([]float64, n)
	remove := make([]float64, n)
	for cy := 0; cy < e.h; cy++ {
		area := e.dx[cy] * dy
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			broad := rate[i] * area
			remove[i] = div[i] - convLayer*broad
			gather[i] = convLayer * math.Max(0, -rate[i])
		}
	}
	gx, gy := e.gatheringFlux(remove)
	for i := range east {
		east[i] -= gx[i]
		north[i] -= gy[i]
	}
	f.gather = gather
	return f
}

// vapourFlux is how the water of the columns crosses the faces of the cells,
// in m²/s: east across the face on the east of each cell, north across the face
// on its north, and on a map that is not a globe west and south across its
// western and southern edges; and gather, how fast, a part a second, the air
// gathering over each cell takes its water up to rain.
type vapourFlux struct {
	east, north, west, south, gather []float64
	wrap                             bool
	w, h                             int
}

// westOf is the flux across the face on the west of cell cx, cy.
func (f vapourFlux) westOf(cx, cy int) float64 {
	switch {
	case cx > 0:
		return f.east[cy*f.w+cx-1]
	case f.wrap:
		return f.east[cy*f.w+f.w-1]
	}
	return f.west[cy]
}

// southOf is the flux across the face on the south of cell cx, cy.
func (f vapourFlux) southOf(cx, cy int) float64 {
	switch {
	case cy+1 < f.h:
		return f.north[(cy+1)*f.w+cx]
	case f.wrap:
		return 0
	}
	return f.south[cx]
}

// gatheringFlux is the gathering part of a flux whose net outflow from each
// cell is div, m²/s, across the face on the east of each cell and on its
// north: the flux down the gradient of a potential χ whose Laplacian over the
// same faces is div less its mean, so that the flux less it leaves each cell
// with no more than the mean. None of it crosses a pole or the edge of a map.
//
// On a globe whose rows are a power of two cells round, each row is taken to
// its Fourier modes and each mode solved down the column exactly; a valley's
// few cells are relaxed.
func (e *airEnv) gatheringFlux(div []float64) (gx, gy []float64) {
	n := e.w * e.h
	dy := e.dy
	var mean float64
	for _, d := range div {
		mean += d
	}
	mean /= float64(n)
	// The conductance across the face on the east of each row's cells, and
	// across the face on the north of each row: none across a pole.
	cx := make([]float64, e.h)
	cn := make([]float64, e.h+1)
	for cy := 0; cy < e.h; cy++ {
		cx[cy] = dy / e.dx[cy]
		if cy > 0 {
			cn[cy] = 0.5 * (e.dx[cy] + e.dx[cy-1]) / dy
		}
	}
	chi := make([]float64, n)
	if e.wrap && kernel.PowerOfTwo(e.w) && e.h > 1 {
		rows := make([]complex128, n)
		for i, d := range div {
			rows[i] = complex(d-mean, 0)
		}
		for cy := 0; cy < e.h; cy++ {
			kernel.FFT(rows[cy*e.w:(cy+1)*e.w], false)
		}
		lo, mid, hi, rhs := make([]complex128, e.h), make([]complex128, e.h), make([]complex128, e.h), make([]complex128, e.h)
		for m := 0; m < e.w; m++ {
			eig := 2*math.Cos(2*math.Pi*float64(m)/float64(e.w)) - 2
			for cy := 0; cy < e.h; cy++ {
				lo[cy], hi[cy] = complex(cn[cy], 0), complex(cn[cy+1], 0)
				mid[cy] = complex(-cn[cy]-cn[cy+1]+cx[cy]*eig, 0)
				rhs[cy] = rows[cy*e.w+m]
			}
			if m == 0 {
				// A constant can be added to χ: hold its first row at nothing.
				mid[0], hi[0], rhs[0] = 1, 0, 0
				lo[1] = 0
			}
			for cy := 1; cy < e.h; cy++ {
				f := lo[cy] / mid[cy-1]
				mid[cy] -= f * hi[cy-1]
				rhs[cy] -= f * rhs[cy-1]
			}
			rhs[e.h-1] /= mid[e.h-1]
			for cy := e.h - 2; cy >= 0; cy-- {
				rhs[cy] = (rhs[cy] - hi[cy]*rhs[cy+1]) / mid[cy]
			}
			for cy := 0; cy < e.h; cy++ {
				rows[cy*e.w+m] = rhs[cy]
			}
		}
		for cy := 0; cy < e.h; cy++ {
			kernel.FFT(rows[cy*e.w:(cy+1)*e.w], true)
		}
		for i := range chi {
			chi[i] = real(rows[i])
		}
	} else if !e.wrap && e.uniformRows() {
		e.cosinePotential(chi, div, mean)
	} else {
		e.relaxPotential(chi, div, mean, cx, cn)
	}
	gx, gy = make([]float64, n), make([]float64, n)
	for cy := 0; cy < e.h; cy++ {
		for c := 0; c < e.w; c++ {
			i := cy*e.w + c
			switch {
			case c+1 < e.w:
				gx[i] = cx[cy] * (chi[i+1] - chi[i])
			case e.wrap:
				gx[i] = cx[cy] * (chi[cy*e.w] - chi[i])
			}
			if cy > 0 {
				gy[i] = cn[cy] * (chi[i-e.w] - chi[i])
			}
		}
	}
	return gx, gy
}

// uniformRows reports whether every row of cells is as wide as every other.
func (e *airEnv) uniformRows() bool {
	for _, d := range e.dx {
		if d != e.dx[0] {
			return false
		}
	}
	return true
}

// cosinePotential solves for χ on a lattice that is not a globe and whose
// rows are all one width: no flux crosses its edges, and the cosines that
// are flat at the edges are what the Laplacian there is made of, so each is
// solved for on its own (the discrete cosine transform of type II).
func (e *airEnv) cosinePotential(chi, div []float64, mean float64) {
	w, h := e.w, e.h
	cx := e.dy / e.dx[0]
	cn := e.dx[0] / e.dy
	table := func(n int) []float64 {
		t := make([]float64, n*n)
		for p := 0; p < n; p++ {
			for x := 0; x < n; x++ {
				t[p*n+x] = math.Cos(math.Pi * float64(p) * (float64(x) + 0.5) / float64(n))
			}
		}
		return t
	}
	tx, ty := table(w), table(h)
	norm := func(p, n int) float64 {
		if p == 0 {
			return float64(n)
		}
		return float64(n) / 2
	}
	// Along the rows, then down the columns.
	rows := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for p := 0; p < w; p++ {
			var sum float64
			for x := 0; x < w; x++ {
				sum += (div[y*w+x] - mean) * tx[p*w+x]
			}
			rows[y*w+p] = sum / norm(p, w)
		}
	}
	coef := make([]float64, w*h)
	for p := 0; p < w; p++ {
		for q := 0; q < h; q++ {
			var sum float64
			for y := 0; y < h; y++ {
				sum += rows[y*w+p] * ty[q*h+y]
			}
			eig := cx*(2*math.Cos(math.Pi*float64(p)/float64(w))-2) + cn*(2*math.Cos(math.Pi*float64(q)/float64(h))-2)
			if p == 0 && q == 0 {
				continue
			}
			coef[q*w+p] = sum / norm(q, h) / eig
		}
	}
	for y := 0; y < h; y++ {
		for p := 0; p < w; p++ {
			var sum float64
			for q := 0; q < h; q++ {
				sum += coef[q*w+p] * ty[q*h+y]
			}
			rows[y*w+p] = sum
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sum float64
			for p := 0; p < w; p++ {
				sum += rows[y*w+p] * tx[p*w+x]
			}
			chi[y*w+x] = sum
		}
	}
}

// relaxPotential solves for χ by over-relaxation, on a lattice the transform
// does not fit.
func (e *airEnv) relaxPotential(chi, div []float64, mean float64, cx, cn []float64) {
	const rounds, over = 2000, 1.9
	for range rounds {
		most := 0.0
		for cy := 0; cy < e.h; cy++ {
			for c := 0; c < e.w; c++ {
				i := cy*e.w + c
				var sum, diag float64
				if c > 0 || e.wrap {
					sum += cx[cy] * chi[e.at(c-1, cy)]
					diag += cx[cy]
				}
				if c+1 < e.w || e.wrap {
					sum += cx[cy] * chi[e.at(c+1, cy)]
					diag += cx[cy]
				}
				if cy > 0 {
					sum += cn[cy] * chi[i-e.w]
					diag += cn[cy]
				}
				if cy+1 < e.h {
					sum += cn[cy+1] * chi[i+e.w]
					diag += cn[cy+1]
				}
				if diag == 0 {
					continue
				}
				d := over * ((sum-(div[i]-mean))/diag - chi[i])
				chi[i] += d
				most = math.Max(most, math.Abs(d))
			}
		}
		big := 0.0
		for _, c := range chi {
			big = math.Max(big, math.Abs(c))
		}
		if most <= 1e-4*big {
			return
		}
	}
}
