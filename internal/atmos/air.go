package atmos

import (
	"math"

	"github.com/LukasSelin/terra/geom"
)

// The air's arithmetic that the rain is read off: the record of a map's
// weather row by row, how much water the air could take back up, and the
// air's half of the rain, worked out on the air cells. The grid's half - the
// rain and the runoff on each tile - is weather.go's, in the root package.

// Air is what the weather of a map is, row by row: the parts of the climate
// that do not change from one day to the next and that the water is read off.
// It is made once, with the map, and shared by every copy of it.
type Air struct {
	Lat  []float64 // degrees
	Mean []float64 // the year's mean temperature at the foot of the map
	Dx   []float64 // kilometres of the planet one tile is, along the row
	Dy   float64   // and across the rows
	// Wetness is what the rain the air's budget gives is multiplied by. See
	// weather.
	Wetness float64
	// PET is how much water the air could take up in a year, in mm, on each
	// row at each whole degree of the year's mean from petLo up, on ground of
	// each of petConts: see PetTable and PetAt.
	PET [][]float64
	// Forcing is the sun, the orbit and the air's carbon the air's year is
	// worked out under: the map's on a globe, and the zero one, today's, on
	// a valley.
	Forcing Forcing
}

// The degrees the evaporation table covers. Colder than petLo the air takes
// up nothing worth counting; warmer than petHi is warmer than anywhere a map
// has.
const (
	petLo = -50
	petHi = 45
)

// calm is how little wind, in metres a second, has no way it blows: the air
// over a calm place is the air of that place and not of anywhere upwind.
const calm = 0.3

// CellOfTile is the air cell tile i of the map lies in.
func (e *Env) CellOfTile(i int) int {
	across := e.W * e.Cell
	return min(i/across/e.Cell, e.H-1)*e.W + min(i%across/e.Cell, e.W-1)
}

// budykoShape is ω in Fu's form of Budyko's curve: how readily the ground
// holds water back for the air to take. Two and six tenths is the figure
// fitted to the world's catchments taken together (Fu, 1981; Zhang and
// others, 2004).
const budykoShape = 2.6

// Fu is how much of p the air takes back in a year where it could take up
// pet, both in mm: Budyko's curve in Fu's form. Where the air could take
// little, it takes nearly all it could; where it could take a great deal, it
// takes nearly all the rain. It is never more than either.
//
// The curve meets p from under it, and in floating point it can cross by a
// rounding where the air could take far more than falls: a runoff a
// rounding under nothing, whose discharge the water's cutting took the root
// of, and a planet's ground went to NaN from the one tile. So it is held to
// its own promise.
//
// It was what the land's runoff and its evaporation into the air were read
// off. They are the soil's bucket's now (see Bucket), whose unseasonal year
// is this curve; it is kept as the bucket's measure.
func Fu(p, pet float64) float64 {
	if p <= 0 || pet <= 0 {
		return 0
	}
	phi := pet / p
	return math.Min(math.Min(p, pet), p*(1+phi-math.Pow(1+math.Pow(phi, budykoShape), 1/budykoShape)))
}

// PetTable is how much water the air at a latitude could take up in a year,
// under forcing f, for each whole degree of the year's mean from petLo to
// petHi and on ground of each of petConts: Hargreaves's reading (Hargreaves
// and Samani, 1985), month by month of the calendar, with the sun's reach at
// the top of the air the energy balance's (see insolation) and the year's
// swing and lag the ground's - SwingUnder and LagUnder at that latitude - so
// that the evaporation's summer is the summer the ground and the wind have.
//
// It used to keep a year of its own: the valley's twelve degrees, growing
// with the latitude up to Temperate and no further, the same over the sea as
// inside a continent, and peaking on the same day everywhere. That was the
// year the rivers were tuned on; read at the ground's, the high latitudes'
// longer warm months took up more of the rain, and moving it was left as a
// question for the water. It is answered here, with the rest of the year.
func PetTable(f Forcing, lat float64) []float64 {
	const span = tableRange
	f = f.OrDefault()
	phi := lat * math.Pi / 180
	// The sun on the middle day of each month, in MJ a square metre a day of
	// the sun's, and how many of the sun's days a month of the calendar is.
	const months = Year / Month
	var ra [months]float64
	for m := range ra {
		ra[m] = f.insolation(phi, float64(m*Month)+Month/2.0) * 86400 / 1e6
	}
	sunDays := float64(Month) * secondsPerYear / 86400 / Year
	out := make([]float64, len(petConts)*petRows)
	for c, cont := range petConts {
		swing, lag := SwingUnder(f, lat, cont), LagUnder(f, cont)
		var t [months]float64
		for m := range t {
			t[m] = swing * SeasonAt(m*Month+Month/2, lag)
		}
		table := out[c*petRows : (c+1)*petRows]
		for k := range table {
			mean := float64(petLo + k)
			total := 0.0
			for m := range ra {
				tm := mean + t[m]
				if tm <= 0 || ra[m] <= 0 {
					continue
				}
				total += 0.0023 * (ra[m] / 2.45) * (tm + 17.8) * math.Sqrt(span) * sunDays
			}
			table[k] = total
		}
	}
	return out
}

// petConts are the continentalities PetTable works a row's evaporation out
// on - the open sea, the middle and a continent's heart - and PetAt reads
// between. The swing is a line in the continentality and the evaporation
// nearly one in the swing, bent where a month crosses freezing, so three are
// enough.
var petConts = [...]float64{0, 0.5, 1}

// petRows is the length of one continentality's table.
const petRows = petHi - petLo + 1

// The day's range of temperature Hargreaves's reading takes the sun's
// strength from. The table is read at tableRange, and each place at its own:
// some six degrees on a humid coast, where the sea and the clouds hold the
// night's warmth in, and sixteen in a dry continent's interior, under clear
// skies over dry ground (Dai, Trenberth and Karl, 1999: 5-8 over the oceans'
// coasts and humid tropics, 12-18 in the deserts and the continents' dry
// interiors). Evaporation goes as its root.
const (
	tableRange = 10.0
	rangeMoist = 6.0 // a maritime, humid place
	rangeInner = 6.0 // what a continent's interior adds
	rangeDry   = 4.0 // and what an arid place adds, reached at PET five times the rain
)

// Diurnal is how many times the evaporation table's a place's evaporation
// is, for the day's range of its temperature: cont of the country round it
// land, and pet over rain its dryness at the table's range.
func Diurnal(cont, dryness float64) float64 {
	span := rangeMoist + rangeInner*clamp01(cont) + rangeDry*clamp01((dryness-1)/4)
	return math.Sqrt(span / tableRange)
}

// PetAt reads a row's evaporation tables at a year's mean of t degrees, on
// ground cont continental.
func PetAt(tables []float64, t, cont float64) float64 {
	c := clamp01(cont) * float64(len(petConts)-1)
	k := min(int(c), len(petConts)-2)
	lo := petRow(tables[k*petRows:(k+1)*petRows], t)
	hi := petRow(tables[(k+1)*petRows:(k+2)*petRows], t)
	return lo + (hi-lo)*(c-float64(k))
}

// petRow reads one continentality's table at a year's mean of t degrees.
func petRow(table []float64, t float64) float64 {
	f := math.Max(0, math.Min(float64(len(table)-1), t-petLo))
	k := int(f)
	if k >= len(table)-1 {
		return table[len(table)-1]
	}
	return table[k] + (table[k+1]-table[k])*(f-float64(k))
}

// firstRain is the rain, mm a year, the land is taken to have before any has
// been worked out.
const firstRain = 700.0

// cellCont is rangeCont for air cell i.
func cellCont(e *Env, i int) float64 {
	if !e.Wrap {
		return ContValley
	}
	return e.Cont[i]
}

// yearCont is the continentality the evaporation's year over air cell i is
// read at: the country round it on a globe, and on a valley the middling
// ground whose year is the valley's own, Swing (see ContMiddling), so that
// the water a valley's air takes up has the summer its ground has.
func yearCont(e *Env, i int) float64 {
	if !e.Wrap {
		return ContMiddling
	}
	return e.Cont[i]
}

// RainCells is the air's half of the rain on a map m under the air a and the
// winds w, whose budget it works out and keeps: what each phase's air rains on
// low ground, carried, in mm a year on the air cells; what the ground's lift
// would rain out of saturated air in each phase, lift, tile by tile; and how
// much of that each cell's column gave, given; and how what the air could
// take up off each cell's land in a year is shared over the phases, share,
// at one for a phase that takes a quarter of it. ground is the height of
// each tile over the water the air takes its fill from.
//
// soil and paw are the land's bucket, tile by tile: how many metres of soil
// it has, and the share of the soil's volume that is water the roots can
// take (see Hold). A tile with no paw is not land and is not counted; with
// none given at all the land is taken to have soilMiddling of loam.
//
// s is the working memory it is worked out in, which the wind was worked out
// in before it (see Scratch); with none it makes its own. carried, given and
// share live in s, and are the caller's to read until s is next used; lift
// and the budget kept on w are their own.
func RainCells(m *geom.Map, a *Air, w *Winds, ground []float64, soil, paw []float32, s *Scratch) (carried, lift [Phases][]float32, given, share [Phases][]float64) {
	e := w.Env
	n := e.W * e.H
	all := s.sharedWork()

	// What each phase's budget is worked out over: the warmth of the air and
	// the sea, how fast the air near the ground gathers, and how much of its
	// rain air held down by the cold water under it keeps.
	var temp, sst [Phases][]float64
	for k := range Phases - 1 {
		wk := s.phaseWork(k)
		temp[k] = e.airTempIn(wk, slotAirTemp, phaseSin[k])
		sst[k] = wk.floats(slotSST, n)
		for cy := 0; cy < e.H; cy++ {
			season := e.seasonTemp(cy, phaseSin[k], 0)
			for cx := 0; cx < e.W; cx++ {
				i := cy*e.W + cx
				sst[k][i] = e.Mean[cy] + season + seaOverAir
				if e.Warm != nil {
					// The current warms or chills the sea and the shallow air
					// over it, under the inversion, and not the column above:
					// see inversion.
					sst[k][i] += e.Warm[i]
				}
			}
		}
	}
	temp[3], sst[3] = temp[1], sst[1]
	// What the ground's lift would rain out of saturated air in each phase,
	// tile by tile; see orographic.go.
	for k := range Phases - 1 {
		lift[k] = orographic(m, a, e, w.U[k], w.V[k], temp[k], ground, e.Subsides[k], s.phaseWork(k))
	}
	lift[3] = lift[1]
	liftCell := func(k int) []float64 {
		c := s.phaseWork(k).floats(slotLiftCells, n)
		for i, r := range lift[k] {
			c[e.CellOfTile(i)] += float64(r) / float64(e.Cell*e.Cell)
		}
		return c
	}
	var liftCells [Phases][]float64
	for k := range Phases - 1 {
		liftCells[k] = liftCell(k)
	}
	liftCells[3] = liftCells[1]
	// How much of its rain the air keeps: held down by the cold water under
	// it, on a globe, and under the subtropical highs capped by the
	// trade-wind inversion, which its convection goes no higher than (see
	// circulation.go).
	var stable [Phases][]float64
	for k := range Phases - 1 {
		stable[k] = s.phaseWork(k).floats(slotStable, n)
		for i := range stable[k] {
			stable[k][i] = lidKeeps(lid(e.Subsides[k][i]), temp[k][i])
			if e.Coast != nil {
				stable[k][i] *= inversion(e.Coast[i])
			}
		}
	}
	stable[3] = stable[1]

	// What the land could send back to the air in a year, and how that is
	// shared out over the phases: as Hargreaves shares it, by the sun at the
	// top of the air and the warmth over freezing. How much of it the day's
	// range adds is read off the land's rain as the budget settles: see
	// below.
	pet0 := make([]float64, n)
	// share is given back, and the grid keeps it for the vegetation's
	// bucket (terra's petShare), so it is new memory and not the
	// Scratch's: nothing a reading gives back lives in one.
	for k := range share {
		share[k] = make([]float64, n)
	}
	for cy := 0; cy < e.H; cy++ {
		row := min(cy*e.Cell+e.Cell/2, m.H-1)
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			pet0[i] = PetAt(a.PET[row], e.Mean[cy]-Lapse*e.Height[i], yearCont(e, i))
			var total float64
			var each [Phases]float64
			for k := range Phases {
				if t := temp[k][i] - Lapse*e.Height[i]; t > 0 {
					// Each phase's own sun. This read dayOf[min(k, 2)], for
					// the autumn's sun to be the spring's, from when the
					// phases were laid out in another order; with the
					// autumn third it was the summer's, and the autumn's
					// air took up more than the spring's off the same
					// warmth, which the soil's year put its rivers' peak
					// before the rain's for.
					day := float64(dayOf[k])
					each[k] = math.Max(0, e.forcing.insolation(e.lat[cy]*math.Pi/180, day)) * (t + 17.8)
				}
				total += each[k]
			}
			for k := range Phases {
				if total > 0 {
					share[k][i] = Phases * each[k] / total
				}
			}
		}
	}

	// The budget, and the land's rain and what it sends back worked out
	// against each other until they are settled (see settleRounds). What it
	// sends back is what the soil's bucket on the cell's land gives the air
	// phase by phase (see Bucket), so that a wet season's rain goes back up
	// through the season and the one after, and a dry season's air gets what
	// the soil kept. Where the air was last worked out over much the same
	// ground - a history rains on its world every age - its columns and its
	// land's rain are where this one starts, and it settles in fewer rounds;
	// where it starts moves where it settles by no more than settledShare.
	annual := make([]float64, n)
	budget := w.Budget
	var from [Phases][]float64
	if last := budget[1].Rain; len(last) == n {
		for i := range annual {
			for k := range Phases {
				annual[i] += (budget[k].Rain[i] + budget[k].Oro[i]) * secondsPerYear / Phases
			}
		}
		for k := range Phases - 1 {
			from[k] = budget[k].w
		}
	} else {
		budget = [Phases]vapourOut{}
		for i := range annual {
			annual[i] = firstRain
		}
	}
	// The budget is settled in up to settleRounds rounds, each a pass over
	// the cells for each phase and a year of a bucket's seventy steps for
	// each cell of land, and so it is spread over the goroutines on any map:
	// on a valley, too small for a single pass over the lattice to be worth
	// spreading, the rounds are the most of a reading (#89).
	workers := workersFor(Phases)
	var phases [Phases - 1]*vapourBudget
	inParallel(Phases-1, workers, func(k, _ int) {
		phases[k] = e.newVapour(vapourIn{
			u: w.U[k], v: w.V[k], temp: temp[k], sst: sst[k],
			stable: stable[k], lift: liftCells[k], w: from[k],
		}, s.phaseWork(k))
	})
	cellSoil, cellPaw := soilCells(e, soil, paw, all)
	var landEvap [Phases][]float64
	for k := range Phases {
		landEvap[k] = s.phaseWork(k).floats(slotLandEvap, n)
	}
	pet := make([]float64, n)
	was := make([]float64, n)
	// What the land sends up in each phase is what its bucket gives the air
	// through its year, under the rain the last round gave it. The spring's
	// budget stands for the autumn's too (see temp above), so it takes up
	// what the land gives in both, between them.
	landYear := func(i int) {
		if annual[i] > 0 {
			pet[i] = pet0[i] * Diurnal(cellCont(e, i), pet0[i]/annual[i])
		} else {
			pet[i] = pet0[i] * Diurnal(cellCont(e, i), 1)
		}
		if e.Sea[i] >= 1 {
			return
		}
		var rain, take [Phases]float64
		for k := range Phases {
			if len(budget[k].Rain) == n {
				rain[k] = (budget[k].Rain[i] + budget[k].Oro[i]) * secondsPerYear / Phases
			} else {
				rain[k] = annual[i] / Phases
			}
			take[k] = pet[i] * share[k][i] / Phases
		}
		phi := 1.0
		if annual[i] > 0 {
			phi = pet[i] / annual[i]
		}
		// On the cell's year at its ground, so that a cold cell's winter
		// lies as snow and goes up off it or into the soil in the spring.
		mean := temp[1][i] - Lapse*e.Height[i]
		swing := (temp[2][i] - temp[0][i]) / 2
		b := BucketCold(Hold(cellSoil[i], cellPaw[i], RootDepth(phi)), &rain, &take, mean, swing)
		for k := range Phases {
			landEvap[k][i] = b.Evap[k] * Phases / secondsPerYear
		}
		landEvap[1][i] = (landEvap[1][i] + landEvap[3][i]) / 2
	}
	rowWorkers := workersFor(e.H)
	for round := range settleRounds {
		inParallel(e.H, rowWorkers, func(cy, _ int) {
			for i := cy * e.W; i < (cy+1)*e.W; i++ {
				landYear(i)
			}
		})
		inParallel(Phases-1, workers, func(k, _ int) {
			phases[k].round(landEvap[k])
			var into vapourOut
			if round > 0 {
				into = budget[k] // this call's own, from the last round
			}
			budget[k] = phases[k].out(into)
		})
		budget[3] = budget[1]
		copy(was, annual)
		var moved, land float64
		for i := range annual {
			annual[i] = 0
			for k := range Phases {
				annual[i] += (budget[k].Rain[i] + budget[k].Oro[i]) * secondsPerYear / Phases
			}
			if l := 1 - e.Sea[i]; l > 0 {
				moved += l * math.Abs(annual[i]-was[i])
				land += l * annual[i]
			}
		}
		if round+1 >= settleLeast && moved <= settledShare*land {
			break
		}
	}
	w.Budget = budget

	// How much of what the ground would wring out of each cell's air its
	// column gave: all of it, unless the air ran dry.
	for k := range Phases - 1 {
		given[k] = s.phaseWork(k).floats(slotGiven, n)
		for i := range given[k] {
			if want := liftCells[k][i]; want > 0 {
				given[k][i] = budget[k].Oro[i] / want
			}
		}
	}
	given[3] = given[1]

	// What each phase's air rains on low ground, in mm a year.
	for k := range carried {
		carried[k] = s.phaseWork(k).floats32(slot32Carried, n)
		for i, r := range budget[k].Rain {
			carried[k][i] = float32(r * secondsPerYear * a.Wetness)
		}
	}
	return carried, lift, given, share
}

// soilMiddling and pawMiddling are the bucket of land whose soil is not
// known: a metre of loam, whose plant-available water Saxton and Rawls
// (2006) put at fourteen parts in a hundred of its volume.
const (
	soilMiddling = 1.0
	pawMiddling  = 0.14
)

// soilCells is the land's bucket on each air cell: the mean depth of soil
// and plant-available water of the land tiles in it, soilMiddling of
// pawMiddling where there are none or none were given.
func soilCells(e *Env, soil, paw []float32, wk *work) (depth, water []float64) {
	n := e.W * e.H
	depth, water = wk.floats(slotSoilDepth, n), wk.floats(slotSoilWater, n)
	count := wk.floats(slotSoilCount, n)
	if len(soil) == len(paw) {
		for i := range soil {
			if paw[i] <= 0 {
				continue
			}
			c := e.CellOfTile(i)
			depth[c] += float64(soil[i])
			water[c] += float64(paw[i])
			count[c]++
		}
	}
	for c := range n {
		if count[c] == 0 {
			depth[c], water[c] = soilMiddling, pawMiddling
			continue
		}
		depth[c] /= count[c]
		water[c] /= count[c]
	}
	wk.let(slotSoilCount)
	return depth, water
}
