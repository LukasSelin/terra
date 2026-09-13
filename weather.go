package terra

import "math"

// The water in the air: where the rain falls, how much of it the air takes
// back, and how much is left to run off the ground.
//
// Rain used to be a reading of height and nothing else - three times as much
// on the highest ground as on the lowest, shared out so that it added to one
// - which gave every map the same climate and made every river a share of a
// whole rather than an amount of water. Here it is carried: the wind brings
// moisture off the sea, the ground it crosses wrings some of it out on the way
// up, what is left over the top is dry, and the warmth of a place decides how
// much of what fell goes straight back into the air. What runs off is what the
// rivers carry, in real quantities, so a wet country and a dry one have
// different rivers and not just different colours.
//
// The scale of it is a fiction and worth being plain about. A tile is
// TileSpan across for everything that is walked or ploughed, and a globe of a
// thousand of them is twenty-five kilometres round - a size at which there are
// no trade winds and no rain belts, and at which the greatest river the rain
// could make would be a brook. So the air reads the map as a planet: a globe's
// rows are latitudes, from pole to pole, and a row is as long as that parallel
// is. And a tile's rain is the rain of HydroSpan of catchment rather than of
// the tile, which is what lets a globe's trunk rivers carry what trunk rivers
// carry. The ground keeps its own scale; the air and the water keep theirs.

// HydroSpan is how far across, in metres, the ground is whose rain one tile
// gathers. See the remark on scale above.
const HydroSpan = 1200.0

// secondsPerYear turns a year's water into a flow.
const secondsPerYear = 365.25 * 24 * 3600

// Air is what the weather of a map is, row by row: the parts of the climate
// that do not change from one day to the next and that the water is read off.
// It is made once, with the map, and shared by every copy of it.
type Air struct {
	lat  []float64 // degrees
	mean []float64 // the year's mean temperature at the foot of the map
	dx   []float64 // kilometres of the planet one tile is, along the row
	dy   float64   // and across the rows
	// wetness is what the rain of every belt is multiplied by; the belts
	// themselves move with the year, so they are read where they stand on the
	// day rather than kept here. See beltRain and weather.
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
	a.dy = HydroSpan / 1000
	if c.globe {
		a.dy = 20015 / float64(g.H)
	}
	for y := 0; y < g.H; y++ {
		lat, mean := Temperate, MeanTemp
		dx := HydroSpan / 1000
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

// beltRain is how much rain falls on low ground by the sea at a latitude, in
// mm a year. Air rises at the equator and at the polar front and rains as it
// goes; it sinks at thirty degrees, which is where the world's deserts are, and
// over the poles, which are deserts too. The figures are the zonal means of
// the real world near its coasts: some two and a half metres under the
// equator, a few hundred millimetres in the horse latitudes, a metre in the
// westerlies, and little at the poles.
func beltRain(lat float64) float64 {
	l := math.Abs(lat)
	c := math.Cos(lat * math.Pi / 180)
	return 150 + 2300*math.Exp(-(l/10)*(l/10)) + 900*math.Exp(-((l-50)/14)*((l-50)/14)) + 250*c*c
}

// How the air carries its water over the land. Each of these is a length on
// the planet in kilometres, or a height in metres.
const (
	// seaReach is how far over open water the air goes to take up its fill.
	seaReach = 800.0
	// landReach is how far inland the air goes before what it carries is
	// what the continent itself sends back up, rather than what came off the
	// sea: the air deep inside a continent is not dry, but it is not maritime.
	landReach = 2000.0
	// inlandShare is how much of the sea's moisture that is: the rain deep in a
	// continent against the rain on its coast. At this, a globe's equatorial
	// land comes out at some 1450 mm a year and its westerlies at 760, against
	// the real world's two metres and some seven hundred millimetres.
	inlandShare = 0.65
	// smoothReach is how far, in every direction, the ground is averaged before
	// the air is asked how much it has risen. Air rises over a range and not
	// over every bump in it; read tile by tile, a valley's upland would be
	// wrung out by its own roughness.
	smoothReach = 25.0
	// wringHeight is how far the air rises to give up all but a part in e of
	// what it carries. The real figure is about two kilometres - the scale
	// height of water vapour - and it is shortened here because the mountains
	// on these maps stand well under a kilometre.
	wringHeight = 1500.0
	// wringReach is how much of what the rising air gives up falls on the
	// slope that lifted it, written as the length of country whose rain that
	// is: the moisture the wind carries over a ridge, as rain spread over so
	// many kilometres of coast. Most of what a real column condenses is carried
	// on and falls or evaporates beyond, which is why it is not the whole flux.
	//
	// Measured on a ridge standing across a flat valley under the westerlies,
	// against the plain upwind of it, with wringHeight at 1500:
	//
	//	ridge    plain   windward face   lee face
	//	100 m    1063       1180          1023
	//	300 m    1128       1472          1015
	//	800 m    1292       2080          1004
	//
	// A third less and the three hundred metre ridge's windward face gets 1.2
	// times its lee's rain, which is no shadow anybody would notice; a third
	// more and the valley's own upland takes a metre and a half a year. The
	// westerlies of this valley blow toward the north-east and not due east,
	// so its lee is drier than these figures for a ridge standing square
	// across them: the shadow follows the wind. See TestARangeAtAnAngle.
	wringReach = 400.0
)

// weather reads the air over the map as it now lies: how much rain each tile
// has in a year, and how much of it runs off. It is read afresh whenever the
// drainage is, because the ground the air crosses is part of what it does.
//
// The wind is worked out first, for each phase of the year - see wind.go -
// and the water is carried along it. The air over the sea takes up its fill,
// the air over land trades the sea's moisture for the continent's own, and
// air made to rise over the ground wrings itself out: all of that on the air
// cells, where what the air carries over a cell is what it carried over the
// cells upwind of it. Then each tile's rain is read off the air over it, with
// what its own slope wrings out of the wind added, and the year's rain is the
// four phases' taken together: a monsoon coast is wet for the summer's
// onshore wind whatever the winter's offshore one does.
func (g *Grid) weather() {
	if g.air == nil {
		g.air = defaultAir(g)
	}
	if len(g.rain) != len(g.Tiles) {
		g.rain = make([]float64, len(g.Tiles))
		g.runoff = make([]float64, len(g.Tiles))
	}
	a := g.air
	w := windsFor(g)
	g.winds = w
	e := w.airEnv

	// The ground the air rises over, tile by tile and cell by cell, in metres a
	// kilometre.
	lifted := g.lifted()
	cellLift := e.gather(g, lifted)
	n := e.w * e.h
	cgx, cgy := make([]float64, n), make([]float64, n)
	for cy := 0; cy < e.h; cy++ {
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			gx, gy := e.grad(cellLift, cx, cy)
			cgx[i], cgy[i] = gx*1000, gy*1000
		}
	}

	// What the air carries over each cell in each phase, against what it
	// carries straight off the sea, as the rain that makes on low ground: with
	// the belts where the sun has them, and the air's gathering taken in.
	var carried [phases][]float32
	for k := range carried {
		carried[k] = make([]float32, n)
	}
	workers := 1
	if n >= spreadTiles {
		workers = WorkersFor(phases)
	}
	InParallel(phases-1, workers, func(k, _ int) {
		q := e.moisture(w.u[k], w.v[k], cgx, cgy)
		conv := e.convergence(w.u[k], w.v[k])
		for cy := 0; cy < e.h; cy++ {
			belt := a.wetness * beltRain(e.rainLat(a, cy)-beltShift*phaseSin[k])
			for cx := 0; cx < e.w; cx++ {
				i := cy*e.w + cx
				carried[k][i] = float32(q[i] * conv[i] * belt)
			}
		}
	})
	copy(carried[3], carried[1])

	// Each tile's rain: the air over it, and what its own slope wrings out.
	dy := a.dy
	g.EachRow(func(y int) {
		dx := a.dx[y]
		fy := (float64(y)+0.5)/float64(e.cell) - 0.5
		up, down := max(y-1, 0), min(y+1, g.H-1)
		for x := 0; x < g.W; x++ {
			i := y*g.W + x
			fx := (float64(x)+0.5)/float64(e.cell) - 0.5
			var gx, gy float64
			if !g.sunk(i) {
				west, east := x-1, x+1
				if g.Wrap {
					west, east = (west+g.W)%g.W, east%g.W
				} else {
					west, east = max(west, 0), min(east, g.W-1)
				}
				gx = (lifted[y*g.W+east] - lifted[y*g.W+west]) / (2 * dx)
				gy = (lifted[up*g.W+x] - lifted[down*g.W+x]) / (2 * dy)
			}
			cell := e.at(int(math.Round(fx)), int(math.Round(fy)))
			var p float64
			for k := range phases {
				air := e.sample32(carried[k], fx, fy)
				if gx != 0 || gy != 0 {
					if ex, ny, ok := unitWind(w.u[k][cell], w.v[k][cell]); ok {
						if rise := ex*gx + ny*gy; rise > 0 {
							// The rise over a tile's length along the wind,
							// and the rain what that wrings out makes on it.
							ds := 1 / (math.Abs(ex)/dx + math.Abs(ny)/dy)
							air *= 1 + (1-math.Exp(-rise*ds/wringHeight))*wringReach/ds
						}
					}
				}
				p += air / phases
			}
			g.rain[i], g.runoff[i] = p, 0
			if !g.sunk(i) {
				t := a.mean[y] - Lapse*g.Tiles[i].Height
				g.runoff[i] = p - fu(p, petAt(a.pet[y], t))
			}
		}
	})
}

// unitWind is the way a wind blows, as a unit step east and north, or not ok
// where there is too little of it to have a way.
func unitWind(u, v float32) (east, north float64, ok bool) {
	s := math.Hypot(float64(u), float64(v))
	if s < calm {
		return 0, 0, false
	}
	return float64(u) / s, float64(v) / s, true
}

// calm is how little wind, in metres a second, has no way it blows: the air
// over a calm place is the air of that place and not of anywhere upwind.
const calm = 0.3

// rainLat is the latitude row cy of the cells has its rain belt at: the
// air's, averaged over the rows of tiles in it.
func (e *airEnv) rainLat(a *Air, cy int) float64 {
	var lat float64
	for y := cy * e.cell; y < (cy+1)*e.cell; y++ {
		lat += a.lat[y]
	}
	return lat / float64(e.cell)
}

// moisture is what the air carries over each cell, as a share of what it
// carries straight off the sea, when it blows u, v over ground rising gx, gy
// metres a kilometre toward the east and the north.
//
// Along the wind, a kilometre of sea brings the air a part in seaReach of the
// way to its fill, a kilometre of land a part in landReach of the way to
// inlandShare, and a metre of rise takes away a part in wringHeight of what it
// has. Written for a cell from what the cells upwind of it carry, that is one
// equation a cell, and the cells are swept in each of the four orders a wind
// can blow in until what they carry settles: a sweep in the order the wind
// blows settles all of that wind in one pass. Air coming in over the edge of a
// valley comes straight off the sea.
func (e *airEnv) moisture(u, v []float32, gx, gy []float64) []float64 {
	n := e.w * e.h
	// Each cell's equation, written once: what it is given whatever its
	// neighbours carry, how much of each upwind neighbour it takes, and what
	// all of that is divided by. An upwind neighbour off the edge of a valley
	// is the sea's air, which is folded into what the cell is given.
	const none = -1
	from := make([]float64, n)
	share := make([]float64, 2*n)
	upwind := make([]int32, 2*n)
	dykm := e.dy / 1000
	e.rows(func(cy int) {
		dxkm := e.dx[cy] / 1000
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			sea := e.sea[i]
			gain := sea/seaReach + (1-sea)/landReach
			f := sea/seaReach + (1-sea)*inlandShare/landReach
			upwind[2*i], upwind[2*i+1] = none, none
			if ex, ny, ok := unitWind(u[i], v[i]); ok {
				if rise := ex*gx[i] + ny*gy[i]; rise > 0 {
					gain += rise / wringHeight
				}
				// Upwind along the row, and up or down the column: toward
				// the north is up the map, so the air comes from the row
				// below.
				if a := math.Abs(ex) / dxkm; a > 0 {
					ux := cx - int(math.Copysign(1, ex))
					switch {
					case e.wrap:
						upwind[2*i], share[2*i] = int32(cy*e.w+(ux+e.w)%e.w), a
					case ux >= 0 && ux < e.w:
						upwind[2*i], share[2*i] = int32(cy*e.w+ux), a
					default:
						f += a
					}
					gain += a
				}
				if b := math.Abs(ny) / dykm; b > 0 {
					uy := cy + int(math.Copysign(1, ny))
					switch {
					case uy >= 0 && uy < e.h:
						upwind[2*i+1], share[2*i+1] = int32(uy*e.w+cx), b
						gain += b
					case !e.wrap:
						f += b
						gain += b
					}
				}
			}
			from[i] = f / gain
			share[2*i] /= gain
			share[2*i+1] /= gain
		}
	})

	q := make([]float64, n)
	for i := range q {
		q[i] = inlandShare + (1-inlandShare)*e.sea[i]
	}
	type order struct{ x0, x1, dx, y0, y1, dy int }
	orders := [4]order{
		{0, e.w, 1, 0, e.h, 1}, {e.w - 1, -1, -1, 0, e.h, 1},
		{0, e.w, 1, e.h - 1, -1, -1}, {e.w - 1, -1, -1, e.h - 1, -1, -1},
	}
	for round := 0; round < moistureRounds; round++ {
		most := 0.0
		for _, o := range orders {
			for cy := o.y0; cy != o.y1; cy += o.dy {
				row := cy * e.w
				for cx := o.x0; cx != o.x1; cx += o.dx {
					i := row + cx
					next := from[i]
					if j := upwind[2*i]; j != none {
						next += share[2*i] * q[j]
					}
					if j := upwind[2*i+1]; j != none {
						next += share[2*i+1] * q[j]
					}
					if d := math.Abs(next - q[i]); d > most {
						most = d
					}
					q[i] = next
				}
			}
		}
		if most < moistureSettled {
			break
		}
	}
	return q
}

// moistureRounds is the most rounds of the four sweeps the air is given to
// settle what it carries, and moistureSettled the change in a round that is
// settled. Two or three settle an ordinary map; a globe's air going round a
// parallel with no sea on it takes more.
const (
	moistureRounds  = 12
	moistureSettled = 1e-3
)

// convergence is how many times its belt's rain each cell has for the air
// gathering over it in the wind u, v: air that gathers has to go up, and air
// that goes up rains. It is read against the mean of its row, because the
// belts already are the rain of the planet's own gathering, and what is
// wanted here is where the land and the ranges bend it.
func (e *airEnv) convergence(u, v []float32) []float64 {
	n := e.w * e.h
	fu, fv := make([]float64, n), make([]float64, n)
	for i := range fu {
		fu[i], fv[i] = e.depth[i]*float64(u[i]), e.depth[i]*float64(v[i])
	}
	out := make([]float64, n)
	for cy := 0; cy < e.h; cy++ {
		for cx := 0; cx < e.w; cx++ {
			// It is the air near the ground that has to go up, and over high
			// ground there is less of it: a wind quickening up a slope into a
			// shallower layer is not air leaving.
			i := cy*e.w + cx
			out[i] = e.div(fu, fv, cx, cy) / e.depth[i]
		}
	}
	// Read at the scale of the weather and not of the ground: the air a
	// single slope lifts is wrung out by the slope, and counted there.
	out = e.blur(out, synopticReach)
	for cy := 0; cy < e.h; cy++ {
		row := cy * e.w
		var mean float64
		for cx := 0; cx < e.w; cx++ {
			mean += out[row+cx]
		}
		mean /= float64(e.w)
		for cx := 0; cx < e.w; cx++ {
			out[row+cx] = math.Max(convLeast, math.Min(convMost, 1-convGain*(out[row+cx]-mean)))
		}
	}
	return out
}

// The rain of gathering air. convGain is how many times its belt's rain a cell
// gains for each part a second the air over it gathers faster than its row's
// does: a continent's summer low gathers air at some five parts in a million a
// second, and has half as much rain again for it. The rain is never less than
// convLeast of the belt's for this, nor more than convMost of it.
const (
	convGain  = 1e5
	convLeast = 0.5
	convMost  = 2.0
)

// lifted is the height of each tile above the sea, averaged over the country
// round it with a weight falling off by a part in smoothReach a kilometre
// either way along the row and down the column: the shape the air rises over,
// which is the shape of the range and not of every bump in it. It is taken
// forward and back along each, so that the average is centred on each tile;
// taken one way only it lags behind the ground, and the air goes on rising
// past the crest and rains on the side it should leave dry. A globe's row is
// taken twice round each way, so that where it starts does not show.
func (g *Grid) lifted() []float64 {
	a := g.air
	base := math.Max(0, g.base)
	h := make([]float64, len(g.Tiles))
	g.EachRow(func(y int) {
		row := h[y*g.W : (y+1)*g.W]
		for x := range row {
			row[x] = math.Max(0, g.Tiles[y*g.W+x].Height-base)
		}
		keep := 1 - math.Exp(-a.dx[y]/smoothReach)
		laps := 1
		if g.Wrap {
			laps = 2
		}
		for _, step := range []int{1, -1} {
			first := 0
			if step < 0 {
				first = g.W - 1
			}
			v := row[first]
			for lap := 0; lap < laps; lap++ {
				for k := 0; k < g.W; k++ {
					x := first + step*k
					v += (row[x] - v) * keep
					row[x] = v
				}
			}
		}
	})
	keep := 1 - math.Exp(-a.dy/smoothReach)
	if g.H < 2 {
		return h
	}
	for x := 0; x < g.W; x++ {
		v := h[x]
		for y := 0; y < g.H; y++ {
			i := y*g.W + x
			v += (h[i] - v) * keep
			h[i] = v
		}
		v = h[(g.H-1)*g.W+x]
		for y := g.H - 1; y >= 0; y-- {
			i := y*g.W + x
			v += (h[i] - v) * keep
			h[i] = v
		}
	}
	return h
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
		span  = 10.0   // degrees between the day's warmest and coldest
	)
	phi := lat * math.Pi / 180
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

// petAt reads a row's evaporation table at a year's mean of t degrees.
func petAt(table []float64, t float64) float64 {
	f := math.Max(0, math.Min(float64(len(table)-1), t-petLo))
	k := int(f)
	if k >= len(table)-1 {
		return table[len(table)-1]
	}
	return table[k] + (table[k+1]-table[k])*(f-float64(k))
}
