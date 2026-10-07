package atmos

// The working memory of a reading of the weather.
//
// A reading works out a few dozen fields on the air cells that do not
// outlive it, one after another: each phase's pressure and wind before the
// ground has had its say, the stress of the year's wind on the sea and the
// gyres' solve under it, and then each phase's vapour - the fluxes between
// the cells, the equations of the columns - and the ground's lift. Each was
// made new where it was needed and left to the collector when it was done
// with, so a reading asked the heap for several times what it ever held at
// once. A Scratch is the memory a reading is worked out in: each field takes
// a slot, and a field begun once another is done with takes that one's slot
// rather than new memory. The wind's fields and the rain's take the same
// slots in turn, the currents take the first phase's once its wind is
// worked out and the gyres' solve the next two phases', and within the rain
// a field done with early hands its slot on (see the slots below). Where the
// sea and the air are worked out together (coupled.go), each round works the
// Walker circulation out in the first phase's memory before that phase's
// wind, and the currents in it after, as the first round did: the rounds
// ask the heap again only for what the currents let go of so that the most
// it holds at once stays what it was.
//
// It is made for one reading and let go of with it (terra.Grid.weather). It
// was tried kept from one reading to the next, which asks the heap for less
// again, but then it is held while the land's other passes run, and through
// the gyres' solve, and the most the heap held while a globe was made rose
// by a third: that is what bounds the size of world a machine can make, and
// the churn is not. Fields the collector would have had back at the point
// they are done with are dropped from their slots there (drop, lend), so
// that the most it holds at once is what it was.
//
// Nothing a reading gives back lives in a Scratch: the winds, the budget and
// the lift are allocated as they always were. And nothing read out of a
// Scratch is read before it is written: every slice it hands out is cleared
// first, as a new one would be, so a world made with one is the world made
// without one, to the bit. A nil *Scratch makes every field new, as before.
//
// The phases of the year that are worked out side by side each have their
// own part of it.
type Scratch struct {
	// phase is the working memory of each phase of the year, for the passes
	// that work the phases out side by side; shared is the rest's.
	phase  [Phases]work
	shared work
}

// phaseWork is the working memory of phase k, nil where s is.
func (s *Scratch) phaseWork(k int) *work {
	if s == nil {
		return nil
	}
	return &s.phase[k]
}

// sharedWork is the working memory of the passes that do not run side by
// side, nil where s is.
func (s *Scratch) sharedWork() *work {
	if s == nil {
		return nil
	}
	return &s.shared
}

// lend is count vectors n long, all nought, for the gyres' solve, in the slots
// of the phases after the first, whose wind is worked out by then; nil where
// s is. What else those phases hold is let go of.
func (s *Scratch) lend(count, n int) [][]float64 {
	if s == nil {
		return nil
	}
	out := make([][]float64, 0, count)
	for k := 1; k < Phases; k++ {
		w := &s.phase[k]
		for slot := range w.f64 {
			if len(out) < count {
				out = append(out, w.floats(slot, n))
			} else {
				w.f64[slot] = nil
			}
		}
	}
	for len(out) < count {
		out = append(out, make([]float64, n))
	}
	return out
}

// drop lets go of every slot of w outside [lo, hi).
func (w *work) drop(lo, hi int) {
	if w == nil {
		return
	}
	for slot := range w.f64 {
		if slot < lo || slot >= hi {
			w.f64[slot] = nil
		}
	}
}

// let lets go of slots of w, and its complex numbers, whose fields are done
// with and which nothing after takes: the collector has them back there, as
// it did before there was a Scratch.
func (w *work) let(slots ...int) {
	if w == nil {
		return
	}
	for _, slot := range slots {
		w.f64[slot] = nil
	}
	w.cplx = nil
}

// The fields of a work, each a slot that one pass writes and reads while it
// runs and no other pass touches meanwhile. The wind is worked out before
// the rain, and nothing the one leaves in its slots is read by the other, so
// the slots of the two share their numbers: a reading holds no more at once
// than the larger of the two needs.
const (
	// Both: the air's warmth at sea level in the phase, and box's rows.
	slotAirTemp = iota
	slotBoxMid
	slotsBoth
)

// The wind: Solve, phase by phase, and then the currents, all at once, in
// the first phase's work, whose Solve is done with by then.
const (
	slotTempBlur = slotsBoth + iota
	slotTemp
	slotWarmBlur
	slotWarm
	slotPres
	slotFreeU
	slotFreeV
	slotWindU
	slotWindV
	slotsWind
)

// channel's fields take the slots of what the pressure was worked out from,
// which is done with by then: the air's warmth and its blurs.
const (
	slotChannelU = slotTempBlur
	slotChannelV = slotWarmBlur
	slotPush     = slotWarm
	slotLam      = slotAirTemp
)

// The air's warmth with the sea's under it (Solve) takes the pressure's slot:
// it is blurred into slotTempBlur before the pressure is begun.
const slotSeaAir = slotPres

const (
	slotStressX = slotsBoth + iota
	slotStressY
	slotCurrentU
	slotCurrentV
	slotDeep
	slotLand
	slotWaterTemp
	slotShore
	// The gyres' forcing and the residual their solve starts from, done with
	// once the gyres are solved; the transport they solve for, done with once
	// the currents are read off it; and the thermocline, read to the end.
	slotForcing
	slotFlowRest
	slotPsi
	slotThermo
	// The gyres' own current, which the two layers of the sea are carried by
	// (slab.go), and how fast the drifts' meeting presses water down.
	slotGyreU
	slotGyreV
	slotSunk
	slotsCurrents
)

// The currents' fields taken over once what was in them is done with: the
// thermocline's guided stress and its rows' level take the gyres' forcing
// and residual; the pumping's drift takes them in turn once the thermocline
// is worked out, and its sum the transport's.
const (
	slotGuided = slotForcing
	slotLevel  = slotFlowRest
	slotDriftX = slotForcing
	slotDriftY = slotFlowRest
	slotPumped = slotPsi
)

// The Walker circulation's (coupled.go, gill.go), in the shared work, between
// the wind's solves: the sea's warmth over its row's, the trades' layer's
// warmth blurred, its pressure, the rain's heating, and Gill's answer to it.
const (
	slotAnomaly = slotsBoth + iota
	slotWalkBlur
	slotWalkWarm
	slotWalkPres
	slotHeat
	slotGillPhi
	slotGillU
	slotGillV
	slotsWalker
)

// The waves (waves.go), before the wind: the ground's height smoothed, in
// the shared work, and in each phase's the waves' forcing and their
// streamfunction, beside the heating and Gill's answer to it in the Walker
// circulation's slots.
const (
	slotWaveHeight = slotsBoth + iota
	slotWaveBlur
)

const (
	slotWaveForce = slotWalkBlur
	slotWavePsi   = slotWalkWarm
)

// The rain: RainCells and orographic phase by phase, and the vapour's budget
// within each; and RainCells all the phases at once.
const (
	slotSST = slotsBoth + iota
	slotLiftCells
	slotStable
	slotLandEvap
	slotShare
	slotOro
	slotEast
	slotNorth
	slotGather
	// vapourFluxes' own, done with before the vapour's sweeps begin; the
	// sweeps, and what follows them, take their slots over (below).
	slotDiv
	slotRate
	slotRemove
	slotRateBlur
	slotChi
	slotGx
	slotGy
	slotsRain
)

// The slots taken over once what was in them is done with: the vapour's own
// fields take vapourFluxes', the rain's share given takes the blurred rate
// once the sweeps are over, and the ground's lift, which is worked out before
// any vapour, lends χ's slot to its sum.
const (
	slotSeaA   = slotDiv
	slotSeaB   = slotRate
	slotRainK  = slotRemove
	slotGiven  = slotRateBlur
	slotOroAcc = slotChi
)

const (
	slotPet = slotsBoth + iota
	slotAnnual
	slotSoilDepth
	slotSoilWater
	slotSoilCount
	slotsRainAll
)

// workSlots is how many slots a work has: the most any reading uses.
const workSlots = max(slotsWind, slotsCurrents, slotsRain, slotsRainAll, slotsWalker)

// The fields of a work in single precision.
const (
	slot32Carried = iota
	work32Slots
)

// work is the working memory of one pass at a time: see Scratch.
type work struct {
	f64   [workSlots][]float64
	f32   [work32Slots][]float32
	cells []vapourCell
	cplx  []complex128
}

// floats is slot's field, n long and all nought; a new one where w is nil.
func (w *work) floats(slot, n int) []float64 {
	if w == nil {
		return make([]float64, n)
	}
	w.f64[slot] = grow(w.f64[slot], n)
	return w.f64[slot]
}

// floats32 is floats in single precision.
func (w *work) floats32(slot, n int) []float32 {
	if w == nil {
		return make([]float32, n)
	}
	w.f32[slot] = grow(w.f32[slot], n)
	return w.f32[slot]
}

// vapourCells is room for n cells' equations, all nought.
func (w *work) vapourCells(n int) []vapourCell {
	if w == nil {
		return make([]vapourCell, n)
	}
	w.cells = grow(w.cells, n)
	return w.cells
}

// complexes is room for n complex numbers, all nought.
func (w *work) complexes(n int) []complex128 {
	if w == nil {
		return make([]complex128, n)
	}
	w.cplx = grow(w.cplx, n)
	return w.cplx
}

// grow is s made n long and cleared, in its own memory where that is room
// enough.
func grow[T any](s []T, n int) []T {
	if cap(s) < n {
		return make([]T, n)
	}
	s = s[:n]
	clear(s)
	return s
}
