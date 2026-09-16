package terra

import (
	"math"

	"github.com/LukasSelin/terra/internal/atmos"
	"github.com/LukasSelin/terra/internal/phase"
)

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

// airFor is the air over a map of g's shape in climate c, with its rain
// multiplied by wetness. A globe is read by latitude; every row of a valley
// is the temperate latitude its weather is the weather of.
func (c Climate) airFor(g *Grid, wetness float64) *Air {
	if wetness <= 0 {
		wetness = 1
	}
	a := &Air{
		Lat: make([]float64, g.H), Mean: make([]float64, g.H),
		Dx: make([]float64, g.H), PET: make([][]float64, g.H),
		Wetness: wetness,
	}
	a.Dy = airSpan / km
	if c.globe {
		a.Dy = 20015 / float64(g.H)
	}
	for y := 0; y < g.H; y++ {
		lat, mean := Temperate, MeanTemp
		dx := airSpan / km
		if c.globe {
			lat, mean = c.latitude(y), c.MeanAt(y)
			dx = 40030 * math.Max(0.05, math.Cos(lat*math.Pi/180)) / float64(g.W)
		}
		a.Lat[y], a.Mean[y], a.Dx[y] = lat, mean, dx
		a.PET[y] = atmos.PetTable(lat)
	}
	return a
}

// defaultAir is the air a grid made by hand breathes: the valley's.
func defaultAir(g *Grid) *Air {
	return Climate{rows: g.H}.airFor(g, 1)
}

// weather reads the air over the map as it now lies: how much rain each tile
// has in a year, and how much of it runs off. The drainage reads it afresh
// whenever the ground has moved far enough to matter, because the ground the
// air crosses is part of what it does: see weatherStale.
//
// The wind is worked out first, for each phase of the year - see package atmos -
// and the water is carried along it as a budget on the air cells: taken up
// off the sea and the land, rained out as the column nears saturation and
// where the air gathers, and wrung out by the ground (atmos/vapour.go and
// atmos/orographic.go). Then each tile's rain is the column's over it and what its
// own ground wrings out, and the year's rain is the four phases' taken
// together: a monsoon coast is wet for the summer's onshore wind whatever the
// winter's offshore one does.
func (g *Grid) weather() {
	defer phase.Start("weather")()
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
	g.winds = g.windsFor()
	if was != nil {
		g.winds.Budget = was.Budget
	}
	g.rainOn()
	g.aired = g.airedGround(g.aired)
}

// The weather is the dearest thing the drainage asks for - the winds and the
// rain over every phase of the year, the better part of making a globe - and
// most of the times it is asked the air would not know the difference. An
// epoch of the plates moves a fifth of the coast and half the relief, and the
// air has to be read again; a round of valley cutting, or of mud laid on the
// flats, moves a few tiles' shore and a few decimetres of ground, and the
// winds and the rain it gave are the same winds and rain to well within what
// either is known to. So the ground the air was last read over is kept, and
// the weather is only read again once the ground has moved from it by more
// than these: weatherFlips of the tiles gone under the water or come out of
// it, or weatherDrift of the height the air rises over, measured tile by tile
// against the last reading and not the last drainage, so that small moves
// cannot add up unseen.
const (
	weatherFlips = 0.005
	weatherDrift = 0.01
)

// airedGround is the ground as the air reads it, tile by tile - the height
// over the water the air takes its fill from, and -1 where the tile is under
// that water - written into into where it has room.
func (g *Grid) airedGround(into []float32) []float32 {
	if len(into) != len(g.Tiles) {
		into = make([]float32, len(g.Tiles))
	}
	base := math.Max(0, g.base)
	for i := range g.Tiles {
		into[i] = -1
		if !g.sunk(i) {
			into[i] = float32(math.Max(0, g.laidHeight(i)-base)) // see laidHeight
		}
	}
	return into
}

// windsFor works out the climate of the wind over g as its ground now lies.
func (g *Grid) windsFor() *Winds {
	defer phase.Start("windsFor")()
	above, wet := g.airGround()
	return atmos.WindsFor(&g.Map, g.air, above, wet)
}

// airGround is the ground of g as the air reads it, tile by tile: how far each
// tile stands over the water the air takes its fill from, and one where the
// tile is under that water.
//
// The slices are locals and not named results: a named result is assigned
// after it is declared, so the closure below would share it on the heap
// rather than copy it, which is two allocations a reading.
func (g *Grid) airGround() ([]float64, []float64) {
	base := math.Max(0, g.base)
	above := make([]float64, len(g.Tiles))
	wet := make([]float64, len(g.Tiles))
	g.EachRow(func(y int) {
		for x := 0; x < g.W; x++ {
			i := y*g.W + x
			above[i] = math.Max(0, g.laidHeight(i)-base) // see laidHeight
			if g.sunk(i) {
				wet[i] = 1
			}
		}
	})
	return above, wet
}

// weatherStale reports whether the ground has moved far enough from where the
// air was last read over it that the weather has to be read again. See
// weatherFlips.
func (g *Grid) weatherStale() bool {
	if len(g.aired) != len(g.Tiles) || len(g.rain) != len(g.Tiles) || g.winds == nil {
		return true
	}
	base := math.Max(0, g.base)
	var flips int
	var moved, stood float64
	for i := range g.Tiles {
		was := g.aired[i]
		if g.sunk(i) != (was < 0) {
			flips++
			continue
		}
		if was < 0 {
			continue
		}
		h := math.Max(0, g.laidHeight(i)-base)
		moved += math.Abs(h - float64(was))
		stood += h
	}
	return float64(flips) > weatherFlips*float64(len(g.Tiles)) || moved > weatherDrift*stood
}

// rainOn is the rain and the runoff of g under the winds it has.
func (g *Grid) rainOn() {
	defer phase.Start("rainOn")()
	a := g.air
	w := g.winds
	e := w.Env

	// The ground the air rises over, tile by tile, in metres above the water
	// the air takes its fill from.
	base := math.Max(0, g.base)
	ground := make([]float64, len(g.Tiles))
	for i := range ground {
		if !g.sunk(i) {
			ground[i] = math.Max(0, g.laidHeight(i)-base) // see laidHeight
		}
	}

	carried, lift, given := atmos.RainCells(&g.Map, a, w, ground)

	// Each tile's rain: the column's over it, and what its own ground wrings
	// out of the air there.
	g.EachRow(func(y int) {
		fy := (float64(y)+0.5)/float64(e.Cell) - 0.5
		for x := 0; x < g.W; x++ {
			i := y*g.W + x
			fx := (float64(x)+0.5)/float64(e.Cell) - 0.5
			cell := e.CellOfTile(i)
			var p float64
			var each [atmos.Phases]float64
			for k := range atmos.Phases {
				air := e.Sample32(carried[k], fx, fy)
				if r := lift[k][i]; r > 0 {
					air += float64(r) * given[k][cell] * secondsPerYear * a.Wetness
				}
				p += air / atmos.Phases
				each[k] = air
			}
			// The warmer half of the year is its summer phase and half of each
			// turn either side of it: the north's summer is the south's winter.
			summer := each[2]
			if a.Lat[y] < 0 {
				summer = each[0]
			}
			g.rainWarm[i] = 0.5
			if total := each[0] + each[1] + each[2] + each[3]; total > 0 {
				g.rainWarm[i] = float32((summer + (each[1]+each[3])/2) / total)
			}
			g.rain[i], g.runoff[i], g.dayRange[i] = p, 0, 1
			if !g.sunk(i) {
				t := a.Mean[y] - Lapse*g.laidHeight(i)
				pe := atmos.PetAt(a.PET[y], t)
				g.dayRange[i] = float32(atmos.Diurnal(g.rangeCont(i), pe/math.Max(p, 1e-9)))
				g.runoff[i] = p - atmos.Fu(p, pe*float64(g.dayRange[i]))
			}
		}
	})
}

// sunk reports whether tile i is under the water the air takes its fill from:
// the sea, where there is one.
func (g *Grid) sunk(i int) bool {
	return g.base >= 0 && g.Height[i] <= g.base
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

// pet is how much water the air could take up in a year on tile i, in mm: the
// row's table at the tile's year and height, for the day's range the tile
// has. It is the table's where the rain has not been read.
func (g *Grid) pet(i int) float64 {
	y := i / g.W
	p := atmos.PetAt(g.air.PET[y], g.air.Mean[y]-Lapse*g.laidHeight(i))
	if i < len(g.dayRange) {
		p *= float64(g.dayRange[i])
	}
	return p
}

// rangeCont is the continentality the day's range at tile i is read at: the
// land round it on a globe, and a middling amount on a map with no ocean to be
// near or far from, whose year is a temperate latitude's.
func (g *Grid) rangeCont(i int) float64 {
	if !g.Wrap {
		return atmos.ContValley
	}
	return g.contAt(i)
}
