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
	wind []float64 // which way the air moves along the row: +1 east, -1 west
	belt []float64 // rain on low ground by the sea, mm a year
	dx   []float64 // kilometres of the planet one tile is, along the row
	dy   float64   // and across the rows
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
		lat: make([]float64, g.H), mean: make([]float64, g.H), wind: make([]float64, g.H),
		belt: make([]float64, g.H), dx: make([]float64, g.H), pet: make([][]float64, g.H),
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
		a.wind[y] = windAt(lat)
		a.belt[y] = wetness * beltRain(lat)
		a.pet[y] = petTable(lat)
	}
	return a
}

// defaultAir is the air a grid made by hand breathes: the valley's.
func defaultAir(g *Grid) *Air {
	return Climate{rows: g.H}.airFor(g, 1)
}

// windAt is which way the air near the ground moves at a latitude, east or
// west. The three cells of each hemisphere: the trade winds blow toward the
// west between the equator and thirty degrees, the westerlies toward the east
// between thirty and sixty, and the polar easterlies west again beyond that.
// Only the east-west part is kept, because that is the part that carries the
// sea's water over a continent.
func windAt(lat float64) float64 {
	if -math.Sin(6*math.Abs(lat)*math.Pi/180) >= 0 {
		return 1
	}
	return -1
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
	// continent against the rain on its coast. At a half, a globe's equatorial
	// land came out at 1270 mm and its westerlies at 630, against the real
	// world's two metres and some seven hundred millimetres.
	inlandShare = 0.65
	// smoothReach is how far along the wind the ground is averaged before the
	// air is asked how much it has risen. Air rises over a range and not over
	// every bump in it; read tile by tile, a valley's upland would be wrung
	// out by its own roughness.
	smoothReach = 25.0
	// acrossReach is how far north and south the rain a range takes along one
	// parallel is shared with the parallels beside it. See spreadRain. At
	// smoothReach, a globe's rows - forty kilometres each - were hardly
	// averaged at all, and every range still cast a shadow one row wide.
	acrossReach = 150.0
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
	//	100 m    1104       1241          1029
	//	300 m    1180       1590           984
	//	800 m    1370       2353           882
	//
	// A third less and the three hundred metre ridge's windward face gets 1.2
	// times its lee's rain, which is no shadow anybody would notice; a third
	// more and the valley's own upland takes a metre and a half a year.
	wringReach = 400.0
)

// weather reads the air over the map as it now lies: how much rain each tile
// has in a year, and how much of it runs off. It is read afresh whenever the
// drainage is, because the ground the air crosses is part of what it does.
//
// Each row is taken downwind, which is the only order in which a tile's air
// has already crossed everything upwind of it. A valley's air comes in over
// its upwind edge straight off the sea. A globe's row starts at the sea and
// goes once round; a row with no sea on it at all - a history has none yet, and
// a globe can be made without one - goes round twice from anywhere, and it is
// the second lap that is kept.
func (g *Grid) weather() {
	if g.air == nil {
		g.air = defaultAir(g)
	}
	if len(g.rain) != len(g.Tiles) {
		g.rain = make([]float64, len(g.Tiles))
		g.runoff = make([]float64, len(g.Tiles))
	}
	a := g.air
	g.EachRow(func(y int) {
		row := y * g.W
		step, first := 1, 0
		if a.wind[y] < 0 {
			step, first = -1, g.W-1
		}
		laps := 1
		w := 1.0
		if g.Wrap {
			start := -1
			for k := 0; k < g.W; k++ {
				x := (first + step*k + g.W) % g.W
				if g.sunk(row + x) {
					start = x
					break
				}
			}
			if start < 0 {
				laps, w = 2, inlandShare
			} else {
				first = start
			}
		}
		dx := a.dx[y]
		toSea := 1 - math.Exp(-dx/seaReach)
		toLand := 1 - math.Exp(-dx/landReach)
		lifted := g.liftedRow(y, 1-math.Exp(-dx/smoothReach))
		for lap := 0; lap < laps; lap++ {
			for k := 0; k < g.W; k++ {
				x := first + step*k
				if g.Wrap {
					x = (x + g.W) % g.W
				}
				i := row + x
				rise := 0.0
				if k > 0 || lap > 0 {
					before := x - step
					if g.Wrap {
						before = (before + g.W) % g.W
					}
					rise = math.Max(0, lifted[x]-lifted[before])
				}
				// What is written here is the rain against the belt's: see
				// the second pass below.
				if g.sunk(i) {
					w += (1 - w) * toSea
					g.rain[i] = w
					continue
				}
				w += (inlandShare - w) * toLand
				wrung := w * (1 - math.Exp(-rise/wringHeight))
				g.rain[i] = w + wrung*wringReach/dx
				w -= wrung
			}
		}
	})
	g.spreadRain(1 - math.Exp(-a.dy/acrossReach))
	g.EachRow(func(y int) {
		belt := a.belt[y]
		for i := y * g.W; i < (y+1)*g.W; i++ {
			p := g.rain[i] * belt
			g.rain[i], g.runoff[i] = p, 0
			if !g.sunk(i) {
				t := a.mean[y] - Lapse*g.Tiles[i].Height
				g.runoff[i] = p - fu(p, petAt(a.pet[y], t))
			}
		}
	})
}

// spreadRain averages the rain along each column, north and south, with a
// weight falling off by keep a row either way, as liftedRow does along the
// row. Each row's air is carried along the row by itself, and left at that
// every range cast its shadow as a line one row wide and as long as a
// continent, with the row beside it untouched: a map of streaks. Air is not
// laid in rows, and the rain a range takes from one parallel it takes from
// the next as well. It is the rain against the belt's that is averaged, so
// that the belts themselves stay where the latitude puts them.
func (g *Grid) spreadRain(keep float64) {
	if keep >= 1 || g.H < 2 {
		return
	}
	for x := 0; x < g.W; x++ {
		v := g.rain[x]
		for y := 0; y < g.H; y++ {
			i := y*g.W + x
			v += (g.rain[i] - v) * keep
			g.rain[i] = v
		}
		v = g.rain[(g.H-1)*g.W+x]
		for y := g.H - 1; y >= 0; y-- {
			i := y*g.W + x
			v += (g.rain[i] - v) * keep
			g.rain[i] = v
		}
	}
}

// liftedRow is the height of row y above the sea, averaged along the row with
// a weight falling off by keep a tile either way: the shape the air rises
// over, which is the shape of the range and not of every bump in it. It is
// taken forward and then back, so that the average is centred on each tile;
// taken one way only, it lags behind the ground, and the air goes on rising
// for twenty tiles past the crest and rains on the side it should leave dry.
// A globe's row is taken twice round each way, so that where it starts does
// not show.
func (g *Grid) liftedRow(y int, keep float64) []float64 {
	row := y * g.W
	base := math.Max(0, g.base)
	h := make([]float64, g.W)
	for x := range h {
		h[x] = math.Max(0, g.Tiles[row+x].Height-base)
	}
	laps := 1
	if g.Wrap {
		laps = 2
	}
	for _, step := range []int{1, -1} {
		first := 0
		if step < 0 {
			first = g.W - 1
		}
		v := h[first]
		for lap := 0; lap < laps; lap++ {
			for k := 0; k < g.W; k++ {
				x := first + step*k
				v += (h[x] - v) * keep
				h[x] = v
			}
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
