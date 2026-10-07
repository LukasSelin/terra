package atmos

import (
	"math"

	"github.com/LukasSelin/terra/internal/kernel"
	"github.com/LukasSelin/terra/internal/phase"
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
	// eddyVapour is K, m²/s, how fast the storms of the middle latitudes mix
	// the water of the air down its gradient on a globe, which no wind of an
	// ordinary year carries: the transient eddies' poleward flux of latent heat,
	// a petawatt at forty degrees (Peixoto and Oort, 1992, ch. 13), is some
	// thirteen kilograms of water a metre of the parallel a second against a
	// fall of fifteen kg/m² of column over the twenty-five degrees from the
	// subtropics to the storm tracks: 2.4e6. A valley is smaller than an eddy.
	eddyVapour = 2.4e6
)

// How the budget is settled. The columns, the ground's lift and the land's
// rain and what it sends back are worked out against each other round by
// round, one round of the sweeps and one year of the land's bucket at a time,
// until a round moves the land's rain by less than settledShare of it, and for
// settleRounds at the most.
//
// They used to be worked out a fixed few times over, from wherever the last
// budget had left them: a history rains on its world every age, and the age
// before's columns and land's rain were taken as near enough to start from
// and one round as enough. It was not. The ground's lift was read off how
// near saturation the column had stood in the last round, and wrung the more
// out of air the wetter it had been, which left it the drier for the next: a
// round that rained a column out on the ground's lift left the next round
// nothing to lift, and the one after a wet column again. Every round swung the
// land's rain the other way, and a history ran one round an age, so its land's
// rain swung from one age to the next by as much as half (the globe's from
// 1822 mm to 1580, 1800, 1615 and on, to 2178 and 1250), and a hundredth of a
// degree on the air reshuffled where it swung: the map's heights moved by
// 1274 m RMS (#84). Now the lift wrings out of the column as near saturation
// as it stands while it is settled, and the budget is a fixed point of the
// age's own ground and air: where it starts moves it by no more than the
// share it is settled to.
const (
	settleRounds = 40
	settledShare = 1e-3
	// settleLeast is the fewest rounds: a budget started from the last one
	// is read at least once more after its land has had a year of its rain.
	settleLeast = 2
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
	u, v   []float32 // the wind near the ground, m/s
	temp   []float64 // the air at sea level, degrees
	sst    []float64 // the sea's surface, degrees
	stable []float64 // how much of its rain air held down by cold water keeps; nil for all
	lift   []float64 // what the ground's lift would wring out of saturated air, kg/m²/s; nil for none
	w      []float64 // where to start the columns from; nil for a fresh start
}

// vapourOut is one phase's settled budget, on the cells: the water in each
// column in kg/m², and in kg/m²/s what it took up, what it rained out - as a
// column and where the air gathered - and what of the ground's lift it could
// give.
type vapourOut struct {
	w, Evap, Rain, Oro []float64
	// sat is the water each column would hold saturated, kg/m².
	sat []float64
}

// vapourCell is one cell's equation for its column's water, as the sweeps
// read it: what it is given and loses whatever its water, where its water
// comes from and how hard, and its rain curve. See round.
type vapourCell struct {
	give, lose        float64
	toStep, rainScale float64
	slope             float64
	from              [4]int32
	share             [4]float64
}

// vapourBudget is one phase's budget as it is settled: each cell's equation,
// laid out once for the phase's wind, and the columns as they stand.
type vapourBudget struct {
	e     *Env
	cells []vapourCell
	// fixed is what each cell is given whatever its land sends up: the sea's
	// water at a column of nothing, and the air's coming in over the edge.
	fixed                   []float64
	seaA, seaB, satW, rainK []float64
	gather, lift            []float64
	w, landEvap             []float64
	// taken is what the ground's lift wrung out of each column in the last
	// sweep, kg/m²/s.
	taken []float64
	// scratch is the coarse corrections' working memory, kept between
	// rounds.
	scratch []float64
	blocks  vapourBlocks
	orders  [4]sweepOrder
}

type sweepOrder struct{ x0, x1, dx, y0, y1, dy int }

// noCell is a vapourCell's from where no water comes from that side.
const noCell = -1

// newVapour lays out one phase's budget: each cell's equation, and the
// columns at in.w, or near saturation as the air off the sea is. Its
// fluxes are worked out in wk (see Scratch).
func (e *Env) newVapour(in vapourIn, wk *work) *vapourBudget {
	defer phase.Start("airEnv.vapour.setup")()
	n := e.W * e.H
	dy := e.Dy
	f := e.vapourFluxes(in.u, in.v, wk)
	// What the fluxes were worked out through is done with, and what of it
	// the sweeps do not take is let go of: see Scratch.
	wk.let(slotRateBlur, slotChi, slotGx, slotGy, slotBoxMid)
	east, north, gather := f.east, f.north, f.gather
	westOf, southOf := f.westOf, f.southOf

	// Each cell's equation, whatever W is: what it is given, and at what
	// rate it loses its own water to the faces, to the sea and to the air
	// gathering; its neighbours upwind are read as the sweeps go.
	// Each is laid out in a vapourCell, so that a visit reads one run of
	// memory.
	b := &vapourBudget{
		e:      e,
		cells:  make([]vapourCell, n),
		fixed:  make([]float64, n),
		seaA:   make([]float64, n), // the sea's evaporation at W of nothing, kg/m²/s
		seaB:   make([]float64, n), // and what each kg/m² of W takes off it, a second
		satW:   make([]float64, n), // the saturated column
		rainK:  make([]float64, n), // what of the column's rain the air keeps
		gather: gather,
		lift:   in.lift,
		w:      make([]float64, n),
		taken:  make([]float64, n),
	}
	cells, seaA, seaB, satW, rainK := b.cells, b.seaA, b.seaB, b.satW, b.rainK
	for cy := 0; cy < e.H; cy++ {
		area := e.Dx[cy] * dy
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			// The column is read against the air at sea level: what the
			// ground's height does to the air climbing it is the ground's lift
			// to wring out (orographic.go), and not the column's to rain again.
			ws := saturatedColumn(in.temp[i])
			satW[i] = ws
			speed := math.Max(gustLeast, math.Hypot(float64(in.u[i]), float64(in.v[i])))
			// The bulk formula, with the humidity at the ground the column's
			// water over its scale height.
			bulk := airDensity * exchangeCoeff * speed * e.Sea[i]
			seaA[i] = bulk * saturation(in.sst[i])
			seaB[i] = bulk / (airDensity * vapourHeight)
			b.fixed[i] = seaA[i]
			out := math.Max(0, east[i]) + math.Max(0, -westOf(cx, cy)) + math.Max(0, north[i]) + math.Max(0, -southOf(cx, cy))
			cells[i].lose = out/area + seaB[i] + gather[i]
			if !e.Wrap {
				// Air coming in over the edge brings the sea's water with it.
				bnd := boundaryHumidity * ws
				if cx == e.W-1 {
					b.fixed[i] += math.Max(0, -east[i]) / area * bnd
				}
				if cx == 0 {
					b.fixed[i] += math.Max(0, westOf(cx, cy)) / area * bnd
				}
				if cy == 0 {
					b.fixed[i] += math.Max(0, -north[i]) / area * bnd
				}
				if cy == e.H-1 {
					b.fixed[i] += math.Max(0, southOf(cx, cy)) / area * bnd
				}
			}
			rainK[i] = 1
			if in.stable != nil {
				rainK[i] = in.stable[i]
			}
		}
	}

	if in.w != nil {
		copy(b.w, in.w)
	} else {
		for i := range b.w {
			b.w[i] = boundaryHumidity * satW[i]
		}
	}
	// Where each cell's water comes from: up to four cells upwind, and the
	// part a second of each one's water that crosses into it.
	for cy := 0; cy < e.H; cy++ {
		area := e.Dx[cy] * dy
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			c := &cells[i]
			c.from = [4]int32{noCell, noCell, noCell, noCell}
			if f := westOf(cx, cy); f > 0 && (cx > 0 || e.Wrap) {
				c.from[0], c.share[0] = int32(e.at(cx-1, cy)), f/area
			}
			if f := east[i]; f < 0 && (cx+1 < e.W || e.Wrap) {
				c.from[1], c.share[1] = int32(e.at(cx+1, cy)), -f/area
			}
			if f := southOf(cx, cy); f > 0 && cy+1 < e.H {
				c.from[2], c.share[2] = int32(i+e.W), f/area
			}
			if f := north[i]; f < 0 && cy > 0 {
				c.from[3], c.share[3] = int32(i-e.W), -f/area
			}
			if e.Wrap {
				// The eddies' share, both ways across every face.
				across := eddyVapour / (e.Dx[cy] * e.Dx[cy])
				c.from[0], c.share[0] = int32(e.at(cx-1, cy)), c.share[0]+across
				c.from[1], c.share[1] = int32(e.at(cx+1, cy)), c.share[1]+across
				c.lose += 2 * across
				if cy+1 < e.H {
					d := eddyVapour * 0.5 * (e.Dx[cy] + e.Dx[cy+1]) / dy / area
					c.from[2], c.share[2] = int32(i+e.W), c.share[2]+d
					c.lose += d
				}
				if cy > 0 {
					d := eddyVapour * 0.5 * (e.Dx[cy] + e.Dx[cy-1]) / dy / area
					c.from[3], c.share[3] = int32(i-e.W), c.share[3]+d
					c.lose += d
				}
			}
		}
	}
	// The rain curve of each column, kept ready: how far along its table a
	// kg/m² of water moves it, what the column's rain is scaled by, and the
	// part of its slope that does not depend on the water.
	for i := range cells {
		c := &cells[i]
		c.toStep = rainSteps / rainMost / satW[i]
		c.rainScale = rainK[i] * satW[i] / rainColumn / 86400
		c.slope = c.rainScale * rainSteep * c.toStep * rainMost / rainSteps
	}
	b.orders = [4]sweepOrder{
		{0, e.W, 1, 0, e.H, 1}, {e.W - 1, -1, -1, 0, e.H, 1},
		{0, e.W, 1, e.H - 1, -1, -1}, {e.W - 1, -1, -1, e.H - 1, -1, -1},
	}
	b.blocks = e.vapourBlocks(cells)
	return b
}

// balance is cell i's equation as its column and its neighbours' stand: what
// it is given less what the ground's lift wrings out, and how fast the lift's
// share grows with its column's water; and its rain and the rain's slope.
//
// The ground's lift wrings out of the column as near saturation as it is, and
// all the column is given if that is less. The builtin min and max are
// math.Min and math.Max to the bit, and are not a call.
func (b *vapourBudget) balance(i int) (sum, taken, dTaken, p, dp float64) {
	c := &b.cells[i]
	sum = c.give
	for j, from := range c.from {
		if from != noCell {
			sum += c.share[j] * b.w[from]
		}
	}
	wi, ws := b.w[i], b.satW[i]
	if b.lift != nil {
		taken = b.lift[i] * min(1, wi/ws)
		if taken < sum {
			if wi < ws {
				dTaken = b.lift[i] / ws
			}
		} else {
			taken = max(0, sum)
		}
		sum -= taken
	}
	// columnRainSlope, written out for the few million times a map asks it.
	f := min(wi*c.toStep, rainSteps)
	kk := int(f)
	ex := rainTable[kk] + (rainTable[kk+1]-rainTable[kk])*(f-float64(kk))
	p = c.rainScale * (ex - rainEmpty)
	dp = c.slope * ex
	if over := wi - rainMost*ws; over > 0 {
		p += dp * over
	}
	return sum, taken, dTaken, p, dp
}

// round is one round of the budget's settling under what the land sends up,
// landEvap in kg/m²/s: four sweeps over the cells, each from another corner,
// and then the coarse corrections. It returns the mean change in a column, in
// kg/m².
//
// Each visit to a cell takes one step of Newton's method on its own
// equation, lose·w + keep·P(w) + lift·w/ws = what it is given, with its
// neighbours as they stand: the left side only grows with w and is convex, so
// the steps never run away.
func (b *vapourBudget) round(landEvap []float64) float64 {
	defer phase.Start("airEnv.vapour")()
	e := b.e
	n := len(b.cells)
	b.landEvap = landEvap
	for i := range b.cells {
		b.cells[i].give = b.fixed[i] + (1-e.Sea[i])*landEvap[i]
	}
	w, lift := b.w, b.lift
	most := 0.0
	for _, o := range b.orders {
		for cy := o.y0; cy != o.y1; cy += o.dy {
			for cx := o.x0; cx != o.x1; cx += o.dx {
				i := cy*e.W + cx
				c := &b.cells[i]
				// balance, written out for the few million times a map
				// asks it.
				sum := c.give
				for j, from := range c.from {
					if from != noCell {
						sum += c.share[j] * w[from]
					}
				}
				wi, ws := w[i], b.satW[i]
				var dTaken float64
				if lift != nil {
					taken := lift[i] * min(1, wi/ws)
					if taken < sum {
						if wi < ws {
							dTaken = lift[i] / ws
						}
					} else {
						taken = max(0, sum)
					}
					sum -= taken
					b.taken[i] = taken
				}
				f := min(wi*c.toStep, rainSteps)
				kk := int(f)
				ex := rainTable[kk] + (rainTable[kk+1]-rainTable[kk])*(f-float64(kk))
				p := c.rainScale * (ex - rainEmpty)
				dp := c.slope * ex
				if over := wi - rainMost*ws; over > 0 {
					p += dp * over
				}
				next := wi - (c.lose*wi+p-sum)/(c.lose+dp+dTaken)
				if next < 0 {
					next = 0
				}
				most += math.Abs(next - wi)
				w[i] = next
			}
		}
	}
	b.blockCorrection()
	if e.Wrap && e.H > 2 {
		b.zonalCorrection()
	}
	return most / float64(n)
}

// out is the budget as it stands, written into into where it has room.
func (b *vapourBudget) out(into vapourOut) vapourOut {
	n := len(b.cells)
	o := into
	if len(o.Rain) != n {
		o = vapourOut{w: make([]float64, n), Evap: make([]float64, n), Rain: make([]float64, n), Oro: make([]float64, n)}
	}
	o.sat = b.satW
	copy(o.w, b.w)
	copy(o.Oro, b.taken)
	for i := range b.w {
		o.Evap[i] = b.seaA[i] - b.seaB[i]*b.w[i] + (1-b.e.Sea[i])*b.landEvap[i]
		o.Rain[i] = b.rainK[i]*columnRain(b.w[i], b.satW[i]) + b.gather[i]*b.w[i]
	}
	return o
}

// The sweeps carry the water a cell a step, and the slow part of their
// settling is the water's spreading over many cells: a sweep moves what is
// out over a block of cells a cell at a time. So after each round the columns
// of each block of vapourBlock cells a side are moved together by as much as
// the block's budget, taken together, is still out, solved for over all the
// blocks at once; and on a globe each row is then moved likewise (see
// zonalCorrection), which is the same correction with a row for a block. It is
// a coarse grid under the sweeps, read with each cell's columns where they
// stand (Galerkin's, for a coarse grid of cells moved as one).
const (
	vapourBlock = 4
	// blockSweeps is how many pairs of sweeps, forth and back, the blocks'
	// own equations are given.
	blockSweeps = 20
)

// vapourBlocks is how the cells are gathered into blocks: which block each
// cell is in, how much of each block's water stays in it from one cell to
// the next, and how much a second each block takes of its four neighbours'.
type vapourBlocks struct {
	w, h   int
	of     []int32
	inside []float64
	from   [][4]float64 // west, east, south (the next row), north
}

func (e *Env) vapourBlocks(cells []vapourCell) vapourBlocks {
	bw, bh := (e.W+vapourBlock-1)/vapourBlock, (e.H+vapourBlock-1)/vapourBlock
	v := vapourBlocks{w: bw, h: bh, of: make([]int32, len(cells)), inside: make([]float64, bw*bh), from: make([][4]float64, bw*bh)}
	for cy := 0; cy < e.H; cy++ {
		for cx := 0; cx < e.W; cx++ {
			v.of[cy*e.W+cx] = int32(cy/vapourBlock*bw + cx/vapourBlock)
		}
	}
	for i := range cells {
		c := &cells[i]
		k := v.of[i]
		for j, from := range c.from {
			if from == noCell {
				continue
			}
			if v.of[from] == k {
				v.inside[k] += c.share[j]
			} else {
				v.from[k][j] += c.share[j]
			}
		}
	}
	return v
}

// scratchOf is n of the budget's scratch, cleared.
func (b *vapourBudget) scratchOf(n int) []float64 {
	if cap(b.scratch) < n {
		b.scratch = make([]float64, n)
	}
	s := b.scratch[:n]
	clear(s)
	return s
}

// blockCorrection moves each block of columns by as much as its block's
// budget is out. See vapourBlock.
func (b *vapourBudget) blockCorrection() {
	v := &b.blocks
	nb := v.w * v.h
	work := b.scratchOf(3 * nb)
	diag, res, delta := work[:nb], work[nb:2*nb], work[2*nb:]
	for i := range b.cells {
		c := &b.cells[i]
		sum, _, dTaken, p, dp := b.balance(i)
		k := v.of[i]
		res[k] += sum - c.lose*b.w[i] - p
		diag[k] += c.lose + dp + dTaken
	}
	for k := range diag {
		diag[k] -= v.inside[k]
	}
	at := func(bx, by int) int { return by*v.w + (bx+v.w)%v.w }
	for range blockSweeps {
		for _, back := range [2]bool{false, true} {
			for s := range nb {
				k := s
				if back {
					k = nb - 1 - s
				}
				if diag[k] <= 0 {
					continue
				}
				bx, by := k%v.w, k/v.w
				fr := &v.from[k]
				sum := res[k]
				if fr[0] != 0 {
					sum += fr[0] * delta[at(bx-1, by)]
				}
				if fr[1] != 0 {
					sum += fr[1] * delta[at(bx+1, by)]
				}
				if fr[2] != 0 {
					sum += fr[2] * delta[at(bx, by+1)]
				}
				if fr[3] != 0 {
					sum += fr[3] * delta[at(bx, by-1)]
				}
				delta[k] = sum / diag[k]
			}
		}
	}
	for i := range b.w {
		if d := delta[v.of[i]]; d == d && !math.IsInf(d, 0) {
			b.w[i] = math.Max(0, b.w[i]+d)
		}
	}
}

// zonalCorrection moves each row of columns by as much as its row's budget,
// taken together, is still out: the slow part of the sweeps' settling is the
// mixing of the water between the latitudes, which a sweep moves a cell at a
// time, and one tridiagonal solve down the rows moves it all at once. It is a
// coarse grid of one cell a row under the sweeps.
func (b *vapourBudget) zonalCorrection() {
	e := b.e
	h := e.H
	work := b.scratchOf(5 * h)
	diag, up, down, res, delta := work[:h], work[h:2*h], work[2*h:3*h], work[3*h:4*h], work[4*h:]
	for cy := 0; cy < h; cy++ {
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			c := &b.cells[i]
			sum, _, dTaken, p, dp := b.balance(i)
			res[cy] += sum - c.lose*b.w[i] - p
			diag[cy] += c.lose + dp + dTaken
			// A row moved as one gives itself back what crosses along it.
			if c.from[0] != noCell {
				diag[cy] -= c.share[0]
			}
			if c.from[1] != noCell {
				diag[cy] -= c.share[1]
			}
			if c.from[2] != noCell {
				down[cy] += c.share[2]
			}
			if c.from[3] != noCell {
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
	delta[h-1] = res[h-1] / diag[h-1]
	for cy := h - 2; cy >= 0; cy-- {
		delta[cy] = (res[cy] + down[cy]*delta[cy+1]) / diag[cy]
	}
	for cy := 0; cy < h; cy++ {
		d := delta[cy]
		if d != d || math.IsInf(d, 0) {
			continue
		}
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			b.w[i] = math.Max(0, b.w[i]+d)
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
//
// It is worked out in wk, and its fields live there: see Scratch.
func (e *Env) vapourFluxes(u, v []float32, wk *work) vapourFlux {
	n := e.W * e.H
	dy := e.Dy
	east, north := wk.floats(slotEast, n), wk.floats(slotNorth, n)
	carry := func(i int, s []float32) float64 { return float64(s[i]) * e.depth[i] / layerDepth }
	for cy := 0; cy < e.H; cy++ {
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			switch {
			case cx+1 < e.W:
				east[i] = 0.5 * (carry(i, u) + carry(i+1, u)) * dy
			case e.Wrap:
				east[i] = 0.5 * (carry(i, u) + carry(cy*e.W, u)) * dy
			}
			if cy > 0 {
				north[i] = 0.5 * (carry(i, v) + carry(i-e.W, v)) * 0.5 * (e.Dx[cy] + e.Dx[cy-1])
			}
		}
	}
	// The edges of a map that is not a globe are open: the air crosses each
	// as it crosses the face inside it, so that the edge neither gathers nor
	// spreads it.
	f := vapourFlux{east: east, north: north, wrap: e.Wrap, w: e.W, h: e.H}
	if !e.Wrap {
		f.west, f.south = make([]float64, e.H), make([]float64, e.W)
		for cy := 0; cy < e.H; cy++ {
			row := cy * e.W
			if e.W > 1 {
				east[row+e.W-1] = east[row+e.W-2]
				f.west[cy] = east[row]
			} else {
				east[row] = carry(row, u) * dy
				f.west[cy] = east[row]
			}
		}
		for cx := 0; cx < e.W; cx++ {
			if e.H > 1 {
				north[cx] = north[e.W+cx]
				f.south[cx] = north[(e.H-1)*e.W+cx]
			} else {
				north[cx] = carry(cx, v) * e.Dx[0]
				f.south[cx] = north[cx]
			}
		}
	}
	// What leaves each cell through its faces, net, in m²/s.
	div := wk.floats(slotDiv, n)
	for cy := 0; cy < e.H; cy++ {
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			div[i] = east[i] - f.westOf(cx, cy) + north[i] - f.southOf(cx, cy)
		}
	}
	// The gathering at the scale of the weather, which the air near the
	// ground goes up and rains out, and what is left at the scale of a cell -
	// the air squeezed round a hill, or hurried off the edge of a valley -
	// which goes round rather than up, the whole column with it.
	rate := wk.floats(slotRate, n)
	for cy := 0; cy < e.H; cy++ {
		area := e.Dx[cy] * dy
		for cx := 0; cx < e.W; cx++ {
			rate[cy*e.W+cx] = div[cy*e.W+cx] / area
		}
	}
	rate = e.blurIn(wk, slotRateBlur, rate, synopticReach)
	gather := wk.floats(slotGather, n)
	remove := wk.floats(slotRemove, n)
	for cy := 0; cy < e.H; cy++ {
		area := e.Dx[cy] * dy
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			broad := rate[i] * area
			remove[i] = div[i] - convLayer*broad
			gather[i] = convLayer * math.Max(0, -rate[i])
		}
	}
	gx, gy := e.gatheringFlux(remove, wk)
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
//
// It is worked out in wk, and gx and gy live there: see Scratch.
func (e *Env) gatheringFlux(div []float64, wk *work) (gx, gy []float64) {
	n := e.W * e.H
	dy := e.Dy
	var mean float64
	for _, d := range div {
		mean += d
	}
	mean /= float64(n)
	// The conductance across the face on the east of each row's cells, and
	// across the face on the north of each row: none across a pole.
	cx := make([]float64, e.H)
	cn := make([]float64, e.H+1)
	for cy := 0; cy < e.H; cy++ {
		cx[cy] = dy / e.Dx[cy]
		if cy > 0 {
			cn[cy] = 0.5 * (e.Dx[cy] + e.Dx[cy-1]) / dy
		}
	}
	chi := wk.floats(slotChi, n)
	if e.Wrap && kernel.PowerOfTwo(e.W) && e.H > 1 {
		rows := wk.complexes(n)
		for i, d := range div {
			rows[i] = complex(d-mean, 0)
		}
		for cy := 0; cy < e.H; cy++ {
			kernel.FFT(rows[cy*e.W:(cy+1)*e.W], false)
		}
		lo, mid, hi, rhs := make([]complex128, e.H), make([]complex128, e.H), make([]complex128, e.H), make([]complex128, e.H)
		for m := 0; m < e.W; m++ {
			eig := 2*math.Cos(2*math.Pi*float64(m)/float64(e.W)) - 2
			for cy := 0; cy < e.H; cy++ {
				lo[cy], hi[cy] = complex(cn[cy], 0), complex(cn[cy+1], 0)
				mid[cy] = complex(-cn[cy]-cn[cy+1]+cx[cy]*eig, 0)
				rhs[cy] = rows[cy*e.W+m]
			}
			if m == 0 {
				// A constant can be added to χ: hold its first row at nothing.
				mid[0], hi[0], rhs[0] = 1, 0, 0
				lo[1] = 0
			}
			for cy := 1; cy < e.H; cy++ {
				f := lo[cy] / mid[cy-1]
				mid[cy] -= f * hi[cy-1]
				rhs[cy] -= f * rhs[cy-1]
			}
			rhs[e.H-1] /= mid[e.H-1]
			for cy := e.H - 2; cy >= 0; cy-- {
				rhs[cy] = (rhs[cy] - hi[cy]*rhs[cy+1]) / mid[cy]
			}
			for cy := 0; cy < e.H; cy++ {
				rows[cy*e.W+m] = rhs[cy]
			}
		}
		for cy := 0; cy < e.H; cy++ {
			kernel.FFT(rows[cy*e.W:(cy+1)*e.W], true)
		}
		for i := range chi {
			chi[i] = real(rows[i])
		}
	} else if !e.Wrap && e.uniformRows() {
		e.CosinePotential(chi, div, mean)
	} else {
		e.relaxPotential(chi, div, mean, cx, cn)
	}
	gx, gy = wk.floats(slotGx, n), wk.floats(slotGy, n)
	for cy := 0; cy < e.H; cy++ {
		for c := 0; c < e.W; c++ {
			i := cy*e.W + c
			switch {
			case c+1 < e.W:
				gx[i] = cx[cy] * (chi[i+1] - chi[i])
			case e.Wrap:
				gx[i] = cx[cy] * (chi[cy*e.W] - chi[i])
			}
			if cy > 0 {
				gy[i] = cn[cy] * (chi[i-e.W] - chi[i])
			}
		}
	}
	return gx, gy
}

// uniformRows reports whether every row of cells is as wide as every other.
func (e *Env) uniformRows() bool {
	for _, d := range e.Dx {
		if d != e.Dx[0] {
			return false
		}
	}
	return true
}

// CosinePotential solves for χ on a lattice that is not a globe and whose
// rows are all one width: no flux crosses its edges, and the cosines that
// are flat at the edges are what the Laplacian there is made of, so each is
// solved for on its own (the discrete cosine transform of type II).
func (e *Env) CosinePotential(chi, div []float64, mean float64) {
	w, h := e.W, e.H
	cx := e.Dy / e.Dx[0]
	cn := e.Dx[0] / e.Dy
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
func (e *Env) relaxPotential(chi, div []float64, mean float64, cx, cn []float64) {
	const rounds, over = 2000, 1.9
	for range rounds {
		most := 0.0
		for cy := 0; cy < e.H; cy++ {
			for c := 0; c < e.W; c++ {
				i := cy*e.W + c
				var sum, diag float64
				if c > 0 || e.Wrap {
					sum += cx[cy] * chi[e.at(c-1, cy)]
					diag += cx[cy]
				}
				if c+1 < e.W || e.Wrap {
					sum += cx[cy] * chi[e.at(c+1, cy)]
					diag += cx[cy]
				}
				if cy > 0 {
					sum += cn[cy] * chi[i-e.W]
					diag += cn[cy]
				}
				if cy+1 < e.H {
					sum += cn[cy+1] * chi[i+e.W]
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
