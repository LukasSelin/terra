package terra

import "math"

// The water in the air: where the rain falls, how much of it the air takes
// back, and how much is left to run off the ground.
//
// Rain used to be a reading of height and nothing else - three times as much
// on the highest ground as on the lowest, shared out so that it added to one
// - which gave every map the same climate and made every river a share of a
// whole rather than an amount of water. Here it is carried: the wind brings
// water off the sea, the ground it crosses wrings some of it out on the way
// up, what is left over the top is dry, and the warmth of a place decides how
// much of what fell goes straight back into the air. What runs off is what the
// rivers carry, in real quantities, so a wet country and a dry one have
// different rivers and not just different colours.
//
// The scale of the air is a fiction and worth being plain about. A tile is
// TileSpan across for everything that is walked or ploughed, and a globe of a
// thousand of them is twenty-five kilometres round - a size at which there are
// no trade winds and no rain belts. So the air reads the map as a planet: a
// globe's rows are latitudes, from pole to pole, and a row is as long as that
// parallel is; and a valley's rows are airSpan apart, so that its ranges lift
// the wind over kilometres and not over the few hundred metres they really
// stand across. What falls is a depth of water a year, which does not care how
// wide the tile is, so the fiction stays in the air.
//
// It used to reach the water too. A tile's runoff was counted off HydroSpan,
// twelve hundred metres, of catchment - 2304 times its own ground - so that a
// globe's trunk rivers would carry what trunk rivers carry. That made every
// figure read off a river a figure about the fiction: the erodibility, the
// power a bed is cut at and the flow a load settles out of were all fitted to
// discharges no ground this size can shed. The water now runs off the ground
// the tile is, and those figures are what the same rivers need at their real
// size: see Erodibility, channelPower and settleFlow. A valley's greatest
// river is a brook of ten or twenty litres a second, which is what two
// kilometres of country makes of a metre of rain.

// airSpan is how far apart, in metres, a valley's air reads its tiles as
// standing: see the remark on scale above. A globe's air reads them off the
// planet instead.
const airSpan = 1200.0

// Air is what the weather of a map is, row by row: the parts of the climate
// that do not change from one day to the next and that the water is read off.
// It is made once, with the map, and shared by every copy of it.
type Air struct {
	lat  []float64 // degrees
	mean []float64 // the year's mean temperature at the foot of the map
	dx   []float64 // kilometres of the planet one tile is, along the row
	dy   float64   // and across the rows
	// wetness is what the rain the air's budget gives is multiplied by. See
	// weather.
	wetness float64
	// pet is how much water the air could take up in a year, in mm, on each
	// row at each whole degree of the year's mean from petLo up: see petOf.
	pet [][]float64
}

// The degrees the evaporation table covers. Colder than petLo the air takes
// up nothing worth counting; warmer than petHi is warmer than anywhere a map
// has.
const (
	petLo = -50
	petHi = 45
)

// airFor is the air over a map of g's shape in climate c, with its rain
// multiplied by wetness. A globe is read by latitude; every row of a valley
// is the temperate latitude its weather is the weather of.
func (c Climate) airFor(g *Grid, wetness float64) *Air {
	if wetness <= 0 {
		wetness = 1
	}
	a := &Air{
		lat: make([]float64, g.H), mean: make([]float64, g.H),
		dx: make([]float64, g.H), pet: make([][]float64, g.H),
		wetness: wetness,
	}
	a.dy = airSpan / km
	if c.globe {
		a.dy = 20015 / float64(g.H)
	}
	for y := 0; y < g.H; y++ {
		lat, mean := Temperate, MeanTemp
		dx := airSpan / km
		if c.globe {
			lat, mean = c.latitude(y), c.MeanAt(y)
			dx = 40030 * math.Max(0.05, math.Cos(lat*math.Pi/180)) / float64(g.W)
		}
		a.lat[y], a.mean[y], a.dx[y] = lat, mean, dx
		a.pet[y] = petTable(lat)
	}
	return a
}

// defaultAir is the air a grid made by hand breathes: the valley's.
func defaultAir(g *Grid) *Air {
	return Climate{rows: g.H}.airFor(g, 1)
}

// weather reads the air over the map as it now lies: how much rain each tile
// has in a year, and how much of it runs off. It is read afresh whenever the
// drainage is, because the ground the air crosses is part of what it does.
//
// The wind is worked out first, for each phase of the year - see wind.go -
// and the water is carried along it as a budget on the air cells: taken up
// off the sea and the land, rained out as the column nears saturation and
// where the air gathers, and wrung out by the ground (vapour.go and
// orographic.go). Then each tile's rain is the column's over it and what its
// own ground wrings out, and the year's rain is the four phases' taken
// together: a monsoon coast is wet for the summer's onshore wind whatever the
// winter's offshore one does.
func (g *Grid) weather() {
	defer phase("weather")()
	if g.air == nil {
		g.air = defaultAir(g)
	}
	if len(g.rain) != len(g.Tiles) {
		g.rain = make([]float64, len(g.Tiles))
		g.runoff = make([]float64, len(g.Tiles))
	}
	if len(g.rainWarm) != len(g.Tiles) {
		g.rainWarm = make([]float32, len(g.Tiles))
		g.dayRange = make([]float32, len(g.Tiles))
	}
	was := g.winds
	g.winds = windsFor(g)
	if was != nil {
		g.winds.budget = was.budget
	}
	g.rainOn()
}

// rainOn is the rain and the runoff of g under the winds it has.
func (g *Grid) rainOn() {
	defer phase("rainOn")()
	a := g.air
	w := g.winds
	e := w.airEnv
	n := e.w * e.h

	// The ground the air rises over, tile by tile, in metres above the water
	// the air takes its fill from.
	base := math.Max(0, g.base)
	ground := make([]float64, len(g.Tiles))
	for i := range ground {
		if !g.sunk(i) {
			ground[i] = math.Max(0, g.Tiles[i].Height-base)
		}
	}

	// What each phase's budget is worked out over: the warmth of the air and
	// the sea, how fast the air near the ground gathers, and how much of its
	// rain air held down by the cold water under it keeps.
	var temp, sst [phases][]float64
	for k := range phases - 1 {
		temp[k] = e.airTemp(phaseSin[k])
		sst[k] = make([]float64, n)
		for cy := 0; cy < e.h; cy++ {
			season := seasonTemp(e.hemi[cy], phaseSin[k], 0)
			for cx := 0; cx < e.w; cx++ {
				i := cy*e.w + cx
				sst[k][i] = e.mean[cy] + season + seaOverAir
				if e.warm != nil {
					// The current warms or chills the sea and the shallow air
					// over it, under the inversion, and not the column above:
					// see inversion.
					sst[k][i] += e.warm[i]
				}
			}
		}
	}
	temp[3], sst[3] = temp[1], sst[1]
	// What the ground's lift would rain out of saturated air in each phase,
	// tile by tile; see orographic.go.
	var lift [phases][]float32
	for k := range phases - 1 {
		lift[k] = g.orographic(e, w.u[k], w.v[k], temp[k], ground)
	}
	lift[3] = lift[1]
	liftCell := func(k int) []float64 {
		c := make([]float64, n)
		for i, r := range lift[k] {
			c[e.cellOfTile(g, i)] += float64(r) / float64(e.cell*e.cell)
		}
		return c
	}
	var liftCells [phases][]float64
	for k := range phases - 1 {
		liftCells[k] = liftCell(k)
	}
	liftCells[3] = liftCells[1]
	var stable []float64
	if e.coast != nil {
		stable = make([]float64, n)
		for i := range stable {
			stable[i] = inversion(e.coast[i])
		}
	}

	// What the land could send back to the air in a year, and how that is
	// shared out over the phases: as Hargreaves shares it, by the sun at the
	// top of the air and the warmth over freezing.
	pet := make([]float64, n)
	var share [phases][]float64
	for k := range share {
		share[k] = make([]float64, n)
	}
	for cy := 0; cy < e.h; cy++ {
		row := min(cy*e.cell+e.cell/2, g.H-1)
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			pet[i] = petAt(a.pet[row], e.mean[cy]-Lapse*e.height[i])
			if r := annualRain(w.budget, i); r > 0 {
				pet[i] *= diurnal(cellCont(e, i), pet[i]/r)
			} else {
				pet[i] *= diurnal(cellCont(e, i), 1)
			}
			var total float64
			var each [phases]float64
			for k := range phases {
				if t := temp[k][i] - Lapse*e.height[i]; t > 0 {
					// The autumn's sun is the spring's.
					day := springDay + float64(dayOf[min(k, 2)])*365.25/Year
					each[k] = math.Max(0, insolation(e.lat[cy]*math.Pi/180, day)) * (t + 17.8)
				}
				total += each[k]
			}
			for k := range phases {
				if total > 0 {
					share[k][i] = phases * each[k] / total
				}
			}
		}
	}

	// The budget, and the land's rain and what it sends back worked out
	// against each other a few times over.
	// Where the air was last worked out over much the same ground - a history
	// rains on its world every age - its columns and its land's rain are
	// where this one starts, and once round is enough.
	annual := make([]float64, n)
	budget := w.budget
	rounds := recycleRounds
	if last := budget[1].rain; len(last) == n {
		for i := range annual {
			for k := range phases {
				annual[i] += (budget[k].rain[i] + budget[k].oro[i]) * secondsPerYear / phases
			}
		}
		rounds = 1
	} else {
		budget = [phases]vapourOut{}
		for i := range annual {
			annual[i] = firstRain
		}
	}
	workers := 1
	if n >= spreadTiles {
		workers = WorkersFor(phases)
	}
	var landEvap [phases][]float64
	for range rounds {
		for k := range phases {
			landEvap[k] = make([]float64, n)
			for i := range annual {
				landEvap[k][i] = fu(annual[i], pet[i]) * share[k][i] / secondsPerYear
			}
		}
		InParallel(phases-1, workers, func(k, _ int) {
			// The ground wrings out of air as near saturation as the column
			// last stood.
			oro := make([]float64, n)
			for i := range oro {
				oro[i] = liftCells[k][i] * humidity(budget[k], i)
			}
			budget[k] = e.vapour(vapourIn{
				u: w.u[k], v: w.v[k], temp: temp[k], sst: sst[k],
				landEvap: landEvap[k], stable: stable, oro: oro, w: budget[k].w,
			})
		})
		budget[3] = budget[1]
		for i := range annual {
			annual[i] = 0
			for k := range phases {
				annual[i] += (budget[k].rain[i] + budget[k].oro[i]) * secondsPerYear / phases
			}
		}
	}
	w.budget = budget

	// How much of what the ground would wring out of each cell's air its
	// column gave: all of it, unless the air ran dry.
	var given [phases][]float64
	for k := range phases - 1 {
		given[k] = make([]float64, n)
		for i := range given[k] {
			if want := liftCells[k][i]; want > 0 {
				given[k][i] = budget[k].oro[i] / want
			}
		}
	}
	given[3] = given[1]

	// What each phase's air rains on low ground, in mm a year.
	var carried [phases][]float32
	for k := range carried {
		carried[k] = make([]float32, n)
		for i, r := range budget[k].rain {
			carried[k][i] = float32(r * secondsPerYear * a.wetness)
		}
	}

	// Each tile's rain: the column's over it, and what its own ground wrings
	// out of the air there.
	g.EachRow(func(y int) {
		fy := (float64(y)+0.5)/float64(e.cell) - 0.5
		for x := 0; x < g.W; x++ {
			i := y*g.W + x
			fx := (float64(x)+0.5)/float64(e.cell) - 0.5
			cell := e.cellOfTile(g, i)
			var p float64
			var each [phases]float64
			for k := range phases {
				air := e.sample32(carried[k], fx, fy)
				if r := lift[k][i]; r > 0 {
					air += float64(r) * given[k][cell] * secondsPerYear * a.wetness
				}
				p += air / phases
				each[k] = air
			}
			// The warmer half of the year is its summer phase and half of each
			// turn either side of it: the north's summer is the south's winter.
			summer := each[2]
			if a.lat[y] < 0 {
				summer = each[0]
			}
			g.rainWarm[i] = 0.5
			if total := each[0] + each[1] + each[2] + each[3]; total > 0 {
				g.rainWarm[i] = float32((summer + (each[1]+each[3])/2) / total)
			}
			g.rain[i], g.runoff[i], g.dayRange[i] = p, 0, 1
			if !g.sunk(i) {
				t := a.mean[y] - Lapse*g.Tiles[i].Height
				pe := petAt(a.pet[y], t)
				g.dayRange[i] = float32(diurnal(g.rangeCont(i), pe/math.Max(p, 1e-9)))
				g.runoff[i] = p - fu(p, pe*float64(g.dayRange[i]))
			}
		}
	})
}

// calm is how little wind, in metres a second, has no way it blows: the air
// over a calm place is the air of that place and not of anywhere upwind.
const calm = 0.3

// humidity is how near saturation the column over cell i stood in a phase's
// budget, as the ground's lift reads it: no more than saturated, and the air
// off the sea where the budget has not been worked out.
func humidity(b vapourOut, i int) float64 {
	if b.sat == nil {
		return boundaryHumidity
	}
	return math.Min(1, b.w[i]/b.sat[i])
}

// cellOfTile is the air cell tile i of g lies in.
func (e *airEnv) cellOfTile(g *Grid, i int) int {
	return min(i/g.W/e.cell, e.h-1)*e.w + min(i%g.W/e.cell, e.w-1)
}

// sunk reports whether tile i is under the water the air takes its fill from:
// the sea, where there is one.
func (g *Grid) sunk(i int) bool {
	return g.base >= 0 && g.Tiles[i].Height <= g.base
}

// Rain is how much rain falls on tile i in a year, in mm, as the air last
// read it. Runoff is how much of that runs off.
func (g *Grid) Rain(i int) float64 {
	if i >= len(g.rain) {
		return 0
	}
	return g.rain[i]
}

// Runoff is how much of the year's rain on tile i runs off, in mm.
func (g *Grid) Runoff(i int) float64 {
	if i >= len(g.runoff) {
		return 0
	}
	return g.runoff[i]
}

// budykoShape is ω in Fu's form of Budyko's curve: how readily the ground
// holds water back for the air to take. Two and six tenths is the figure
// fitted to the world's catchments taken together (Fu, 1981; Zhang and
// others, 2004).
const budykoShape = 2.6

// fu is how much of p the air takes back in a year where it could take up
// pet, both in mm: Budyko's curve in Fu's form. Where the air could take
// little, it takes nearly all it could; where it could take a great deal, it
// takes nearly all the rain. It is never more than either.
func fu(p, pet float64) float64 {
	if p <= 0 || pet <= 0 {
		return 0
	}
	phi := pet / p
	return p * (1 + phi - math.Pow(1+math.Pow(phi, budykoShape), 1/budykoShape))
}

// petTable is how much water the air at a latitude could take up in a year,
// for each whole degree of the year's mean from petLo to petHi: Hargreaves's
// reading (Hargreaves and Samani, 1985), month by month, with the sun's reach
// at the top of the air by FAO-56 and the year's swing turning over south of
// the equator as the temperature does.
func petTable(lat float64) []float64 {
	const (
		solar = 0.0820 // MJ a square metre a minute
		span  = tableRange
	)
	phi := lat * math.Pi / 180
	// The evaporation keeps the year it was calibrated on - the temperate
	// swing capped at Temperate's - and not solarSwing's, which the ground and
	// the wind read. Read at the growing swing, the high latitudes' longer
	// warm months took up more of the rain, and on eight small globes the
	// discharge exceedance exponent went from 0.447 to 0.502, out of the real
	// networks' 0.40-0.46 (Rodriguez-Iturbe et al., 1992) while the drainage
	// area's stayed in it: the rivers are tuned to this year, and moving it is
	// a question for the water and not for the thresholds.
	hemi := math.Copysign(math.Min(1, math.Abs(lat)/Temperate), lat)
	var ra, swing [12]float64
	days := [12]float64{31, 28.25, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	day := 0.0
	for m := range ra {
		j := day + days[m]/2
		day += days[m]
		d := 0.409 * math.Sin(2*math.Pi*j/365.25-1.39)
		dr := 1 + 0.033*math.Cos(2*math.Pi*j/365.25)
		ws := math.Acos(math.Max(-1, math.Min(1, -math.Tan(phi)*math.Tan(d))))
		ra[m] = 24 * 60 / math.Pi * solar * dr * (ws*math.Sin(phi)*math.Sin(d) + math.Cos(phi)*math.Cos(d)*math.Sin(ws))
		swing[m] = hemi * Swing * math.Cos(2*math.Pi*(j-196)/365.25)
	}
	out := make([]float64, petHi-petLo+1)
	for k := range out {
		mean := float64(petLo + k)
		total := 0.0
		for m := range ra {
			t := mean + swing[m]
			if t <= 0 || ra[m] <= 0 {
				continue
			}
			total += 0.0023 * (ra[m] / 2.45) * (t + 17.8) * math.Sqrt(span) * days[m]
		}
		out[k] = total
	}
	return out
}

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

// diurnal is how many times the evaporation table's a place's evaporation
// is, for the day's range of its temperature: cont of the country round it
// land, and pet over rain its dryness at the table's range.
func diurnal(cont, dryness float64) float64 {
	span := rangeMoist + rangeInner*clamp01(cont) + rangeDry*clamp01((dryness-1)/4)
	return math.Sqrt(span / tableRange)
}

// pet is how much water the air could take up in a year on tile i, in mm: the
// row's table at the tile's year and height, for the day's range the tile
// has. It is the table's where the rain has not been read.
func (g *Grid) pet(i int) float64 {
	y := i / g.W
	p := petAt(g.air.pet[y], g.air.mean[y]-Lapse*g.Tiles[i].Height)
	if i < len(g.dayRange) {
		p *= float64(g.dayRange[i])
	}
	return p
}

// petAt reads a row's evaporation table at a year's mean of t degrees.
func petAt(table []float64, t float64) float64 {
	f := math.Max(0, math.Min(float64(len(table)-1), t-petLo))
	k := int(f)
	if k >= len(table)-1 {
		return table[len(table)-1]
	}
	return table[k] + (table[k+1]-table[k])*(f-float64(k))
}

// springDay is the day of the calendar year, from the first of January, that
// tick zero - the spring equinox - falls on; and firstRain the rain, mm a year,
// the land is taken to have before any has been worked out.
const (
	springDay = 80.0
	firstRain = 700.0
)

// annualRain is the rain, mm a year, a budget last gave cell i, or nothing
// where it has not been worked out.
func annualRain(b [phases]vapourOut, i int) float64 {
	if len(b[1].rain) <= i {
		return 0
	}
	var r float64
	for k := range phases {
		r += (b[k].rain[i] + b[k].oro[i]) * secondsPerYear / phases
	}
	return r
}

// rangeCont is the continentality the day's range at tile i is read at: the
// land round it on a globe, and a middling amount on a map with no ocean to be
// near or far from, whose year is a temperate latitude's.
func (g *Grid) rangeCont(i int) float64 {
	if !g.Wrap {
		return contMiddling
	}
	return g.contAt(i)
}

// cellCont is rangeCont for air cell i.
func cellCont(e *airEnv, i int) float64 {
	if !e.wrap {
		return contMiddling
	}
	return e.cont[i]
}
