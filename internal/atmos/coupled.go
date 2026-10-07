package atmos

import (
	"math"

	"github.com/LukasSelin/terra/internal/phase"
)

// The sea and the air, solved together.
//
// The wind was worked out from the warmth of the air, the currents from the
// wind, and the sea's warmth from the currents, and there it stopped: the air
// read the sea back for its rain and its storms but never for its pressure,
// so the sea's warmth had no say in the wind that made it. The real ocean
// and the air over it are one system, and some of the largest things in the
// climate live in the loop between them. A sea warmer in the west of an
// ocean than in its east draws the air along the equator toward the warm
// water, and the air that rises over the warm pool and comes down over the
// cold east closes the loop aloft: the Walker circulation (Walker, 1924;
// Bjerknes, 1969). The easterlies that blow along the equator to the warm
// pool are the trades that hold the thermocline up in the east and pile the
// warm water up in the west (thermocline.go), and the shallow thermocline in
// the east is what makes the water that comes up there cold. A little
// contrast makes a little easterly, which makes more contrast: Bjerknes's
// feedback, which keeps the cold tongue of the Pacific cold and its warm pool
// warm.
//
// The sea's warmth reaches the wind two ways (Back and Bretherton, 2009, find
// the two of a size over the tropical oceans):
//
//   - Through the air near the ground. Air over a cold sea is cold and heavy
//     and the pressure under it high; over a warm sea light, and the pressure
//     low. Away from the tropics that is the warmth the pressure is read off
//     (Solve), over the marine layer's kilometre. Within the Hadley cells the
//     air the sea warms is the trades' whole layer, cumulus and all, up to
//     some seven hundred hPa, and the sea's warmth falls away through it to
//     nothing at its top; and the wind it drives there is slowed by the drag
//     of the sea on the whole of that layer, in some two and a half days
//     (Lindzen and Nigam, 1987). See walker.
//   - Through the rain. Over the warmest water of a row the air rises in deep
//     convection, and the heat its rain lets go of warms the whole column
//     and draws the air in near the ground: the first baroclinic mode of the
//     tropical atmosphere's answer to a heating, the easterlies of a Kelvin
//     wave east of it and the westerlies of the Rossby waves west of it
//     (Gill, 1980; Zebiak and Cane, 1987). See gill.go.
//
// The wind, the currents, the thermocline and the sea's warmth are then
// worked out again under it, round by round. Each round the air reads only
// coupleDamp of the change the sea made since the round before: the wind
// answers the sea at once in the solve, where the real air takes days to it
// and the real ocean months to the wind, and a loop answered in full each
// round swings from round to round, the warm pool and the cold tongue
// trading places, rather than settling. And the air starts from the sea's
// warmth as it read it the last time the wind was worked out over the same
// map (carry), so that the rounds of every reading of the weather through
// the history add up: the coupled state is carried through the ages as the
// rain's own budget is (Grid.weather), and settles over them. A map whose
// weather is read once has the rounds of one reading.
//
// What holds the feedback is the sea's own physics, and not a cap of the
// loop's: the thermocline comes no nearer the surface than thermoLeast, and
// the water that comes up from under it can be no colder than the deep
// (upwelled), so however strong the easterlies grow, the east of an ocean
// can be no colder than the water under it; the sea's warmth is held to
// seaWarmMost of its latitude's; and the air over ice reads none of the
// water's warmth under it.
//
// The rounds are a fixed number, so the world is the same whatever they
// came to, on any number of goroutines: each round's solves are the ones
// WindsFor makes, written cell by cell or row by row, every sum in one order.
// It is done only on a globe, which is all that has currents; a valley is
// untouched by it to the bit.

const (
	// coupleRounds is how many times each reading of the weather works the
	// wind and the sea out again, after the first, with the air reading the
	// sea's warmth. A reading that starts from the last (carry) has taken a
	// round more before its first.
	coupleRounds = 1
	// coupleDamp is how much of the change in the sea's warmth since the
	// round before the air reads in each round. At a half the warm pool and
	// the cold tongue of the first globe's broadest ocean swung from round
	// to round and did not settle in sixteen rounds; at three tenths they
	// swung wider each time round, from four degrees one way to six the
	// other; and cells a few degrees off the equator whose upwelling the
	// wind turns on and off traded places every round. At three twentieths
	// they settle.
	coupleDamp = 0.15
)

// The trades' layer (Lindzen and Nigam, 1987).
const (
	// tradesDepth is how deep, in metres, the air the tropical sea warms is:
	// the trade cumulus layer, up to some seven hundred hPa. The sea's
	// warmth falls away through it to nothing at its top, so the pressure
	// at the ground is a layer of half the depth's at the full warmth.
	tradesDepth = 3000.0
	// tradesDrag is how fast, per second, the sea's drag slows the wind of
	// the whole layer: in two and a half days.
	tradesDrag = 1 / (2.5 * 86400)
)

// couple works the wind w and the sea under it out together, as above. The
// first wind and currents are already worked out, under the sea's warmth as
// carry gave it, or none; the residual of each round, how far the sea's
// warmth stood from what the air read, is kept in Coupled, and what was
// left after the last. ocean is the gyres' equations for the ground, and s
// the reading's working memory (see Scratch).
func (w *Winds) couple(ocean *flow, s *Scratch) {
	defer phase.Start("airEnv.couple")()
	e := w.Env
	n := e.W * e.H
	if e.seaAir == nil {
		e.seaAir = make([]float64, n)
	}
	e.Coupled = e.Coupled[:0]
	for range coupleRounds {
		e.Coupled = append(e.Coupled, e.residual())
		for i := range n {
			e.seaAir[i] += coupleDamp * (e.Sea[i]*e.Warm[i] - e.seaAir[i])
		}
		e.walker(s.phaseWork(0))
		w.solve(s)
		e.Warm = e.currents(w.U, w.V, ocean, s)
	}
	e.Coupled = append(e.Coupled, e.residual())
}

// carry starts the air reading the sea where the wind was, was, left it, if
// it was worked out on the same lattice of cells: the warmth the air read of
// the sea, and coupleDamp of the way from it to the warmth the sea then had,
// which is a round of the coupled solve taken across the two readings. Each
// cell that is still sea has it for as much of the cell as is still sea;
// where the land has come up out of the sea there is none.
//
// The Walker circulation it starts from is worked out in the first phase's
// memory, before that phase's wind is (see walker).
func (e *Env) carry(was *Winds, s *Scratch) {
	n := e.W * e.H
	if was == nil || was.Env == nil || was.W != e.W || was.H != e.H || len(was.seaAir) != n || len(was.Warm) != n {
		return
	}
	e.seaAir = make([]float64, n)
	for i, s := range was.Sea {
		if s > 0 {
			read := was.seaAir[i] + coupleDamp*(s*was.Warm[i]-was.seaAir[i])
			e.seaAir[i] = read / s * math.Min(s, e.Sea[i])
		}
	}
	e.walker(s.phaseWork(0))
}

// residual is how far the sea's warmth stands from what the air reads of
// it, over the sea.
func (e *Env) residual() Residual {
	var r Residual
	var sq, k, tq, tk float64
	for i := range e.W * e.H {
		if e.Sea[i] <= 0.5 {
			continue
		}
		d := e.Sea[i]*e.Warm[i] - e.seaAir[i]
		r.Most = math.Max(r.Most, math.Abs(d))
		sq, k = sq+d*d, k+1
		if e.tropicShare(i/e.W) == 1 {
			tq, tk = tq+d*d, tk+1
		}
	}
	r.RMS = math.Sqrt(sq / math.Max(k, 1))
	r.Tropics = math.Sqrt(tq / math.Max(tk, 1))
	return r
}

// Residual is how far, in degrees, the sea's warmth the air read in a round
// of the coupled solve stood from what the sea then had: the most over any
// cell of sea, the root mean square over them all, and the root mean square
// over the sea within the Hadley cells.
type Residual struct{ Most, RMS, Tropics float64 }

// tropicShare is how much of the sea's warmth on row cy the trades' layer
// and the rain carry (walker) rather than the air near the ground the belts
// drive (Solve): all of it within the Hadley cells, none a belt's breadth
// past their edges.
func (e *Env) tropicShare(cy int) float64 {
	edge := hadleyEdge(e.circ)
	return 1 - smoothstep(edge-beltWidth/2, edge+beltWidth/2, math.Abs(e.lat[cy]))
}

// walker works out the pressure and the wind the tropical sea's warmth
// makes, from seaAir, into Walk: the trades' layer's and the rain's. Each
// reads the sea's warmth over the mean of its row's sea, which is nothing on
// land and comes to nothing along the row: the warmth of the whole row is
// the belts' (circulation.go), and only how it lies along the row is the
// Walker circulation's.
//
// It is worked out in wk (see Scratch), the first phase's memory, which
// holds nothing then that is read again: the wind is worked out in it
// next, and the currents before it are kept on e. Only Walk is kept.
func (e *Env) walker(wk *work) {
	n := e.W * e.H
	anomaly := wk.floats(slotAnomaly, n)
	for cy := 0; cy < e.H; cy++ {
		share := e.tropicShare(cy)
		if share == 0 {
			continue
		}
		row := cy * e.W
		var s, k float64
		for cx := 0; cx < e.W; cx++ {
			s, k = s+e.seaAir[row+cx], k+e.Sea[row+cx]
		}
		if k == 0 {
			continue
		}
		for cx := 0; cx < e.W; cx++ {
			i := row + cx
			anomaly[i] = share * (e.seaAir[i] - e.Sea[i]*s/k)
		}
	}

	// The trades' layer: the warmth spread as the air near the ground's is
	// (Solve), and the pressure it takes off the ground.
	a := e.blurIn(wk, slotWalkWarm, e.blurIn(wk, slotWalkBlur, anomaly, synopticReach), synopticReach)
	p := wk.floats(slotWalkPres, n)
	for cy := 0; cy < e.H; cy++ {
		row := cy * e.W
		var zonal float64
		for cx := 0; cx < e.W; cx++ {
			zonal += a[row+cx]
		}
		zonal /= float64(e.W)
		for cx := 0; cx < e.W; cx++ {
			p[row+cx] = -hypsometric(beltMean, e.Mean[cy], tradesDepth/2) * (a[row+cx] - zonal)
		}
	}

	// The rain's.
	q := wk.floats(slotHeat, n)
	for i, t := range anomaly {
		q[i] = rainHeat * t
	}
	phi, gu, gv := e.gill(q, wk)

	for k := range e.Walk {
		if len(e.Walk[k]) != n {
			e.Walk[k] = make([]float32, n)
		}
	}
	r := tradesDrag
	e.rows(func(cy int) {
		f := e.f[cy]
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			px, py := e.grad(p, cx, cy)
			px, py = px*100/airDensity, py*100/airDensity // hPa to Pa, and a force on a kilogram
			d := r*r + f*f
			// The trades' layer is the sea's: over land the wind is the
			// ground's to drag.
			uu, vv := (-r*px-f*py)/d*e.Sea[i], (f*px-r*py)/d*e.Sea[i]
			e.Walk[0][i] = float32(uu + gu[i])
			e.Walk[1][i] = float32(vv + gv[i])
			e.Walk[2][i] = float32(p[i] + airDensity*phi[i]/100)
		}
	})
}
