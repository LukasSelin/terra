package atmos

import "math"

// The planet's general circulation, worked out from the energy balance.
//
// The belts of pressure used to be written down: the equatorial trough on
// the equator, the subtropical highs at thirty-two degrees, the subpolar lows
// at sixty-two and the polar highs at eighty-eight, all moved five degrees
// north and south with the sun. A hotter planet, a slower one, a tilted one
// or a continent across the equator could not move them. Here each is placed
// by what puts it there:
//
//   - The Hadley cell's edge, where the air that rose at the equator comes
//     down, is Held and Hou's (1980): the air aloft carries the angular
//     momentum it rose with, and the cell reaches as far as the warmth its
//     own wind is in balance with stays under the radiative equilibrium's.
//     That gives φ_H = (5/3 g H Δ_H / Ω²a²)^½, wider for a larger contrast
//     Δ_H between the equilibrium's equator and its pole, for a higher
//     tropopause H and for a slower spin Ω. The contrast is the balance's
//     own radiative equilibrium - its sun on its ice-free ground against
//     Budyko's line, with nothing carried - and the tropopause stands where
//     the tropics' air, cooling at the lapse rate from the balance's
//     equator, reaches the anvils' fixed temperature (Hartmann and Larson,
//     2002). The theory's own figure for today's planet is some forty-three
//     degrees - an inviscid cell's, which no eddies stop short - and the
//     real cell's edge some thirty to thirty-three, by whichever measure
//     (Hu and Fu, 2007; Davis and Rosenlof, 2012): the theory is taken for
//     how the edge moves, and the real edge for where it stands today.
//   - The ITCZ, where the trades meet and the air rises, is on the energy
//     flux equator, where the heat the air carries poleward turns round
//     (Kang and others, 2008; Bischoff and Schneider, 2014), which in the
//     balance is where its warmth stands highest: the balance's year carries
//     it into each summer's hemisphere. How far depends on the land. A
//     planet of sea would have the balance's sea's, some five degrees
//     either side; a band of the balance, three tenths land, has nine; and a
//     planet of land would have the balance's land's, which a summer takes
//     some twenty-four degrees toward its pole. A map's ITCZ is read between
//     them by the land under the tropics of the hemisphere the summer is in.
//     It is the zonal mean's: the trough that bends over a continent in its
//     summer, and the monsoon it draws, are the heating's (Gill, 1980): see
//     waves.go.
//   - The subtropical highs stand at the cell's edges, which follow the ITCZ
//     north and south by edgeShare of its swing: the summer's cell shrinks
//     and the winter's reaches across the equator (Lindzen and Hou, 1988).
//   - The subpolar lows, and the band the day's lows are born in, lie
//     poleward of the edge by the breadth of the Ferrel cell.
//
// How deep they are goes with the cell too: the trough and the highs with
// the square of the edge, which is how the angular momentum the air aloft
// carries grows with it, and the subpolar lows with the fall of warmth across
// the middle latitudes that feeds their storms; both against today's.
//
// The air that comes down in the Hadley cell's descending branch is dry and
// warm, and lies over the cool, moist air the trades blow over the sea as a
// lid: the trade-wind inversion (Riehl, 1954). The boundary layer under it
// deepens as fast as it takes in the dry air above (Lilly, 1968), and the
// lid settles where the descent brings the air down as fast, some five
// hundred metres to two kilometres up. Convection under it goes no higher,
// and a range in the trades rains only from the air under it: Hawaii's
// windward slopes are the wettest on earth to two kilometres, and dry above.

// The circulation's figures.
const (
	// hadleyToday is the edge of today's Hadley cell over the year, degrees
	// from the equator: where the mean meridional streamfunction at 500 hPa
	// crosses nought, some thirty to thirty-one degrees, and where the
	// surface's easterlies end and the subtropical highs' ridge stands, some
	// thirty-one to thirty-three (Hu and Fu, 2007; Davis and Rosenlof,
	// 2012). The ridge is what the belts lay down, and thirty-two is where
	// they always stood: the valley's rivers and lakes were tuned under it.
	hadleyToday = 32.0
	// tropopauseTemp is the temperature, in kelvin, at which the tropics'
	// deep convection stops and its anvils spread: fixed, near two hundred,
	// whatever the warmth under it (Hartmann and Larson, 2002).
	tropopauseTemp = 200.0
	// edgeShare is how far the Hadley cells' edges follow the ITCZ, as a
	// share of its swing: the subtropical highs move some four or five
	// degrees over the year where the ITCZ moves some eight or nine.
	edgeShare = 0.5
	// ferrelBreadth is how many degrees poleward of the Hadley cell's edge
	// the subpolar lows lie: today's, thirty-two and sixty-two.
	ferrelBreadth = 30.0
	// beltWidth is the breadth, in degrees, of the subtropical highs'
	// poleward flank and of the subpolar lows, and capWidth of the polar
	// highs. The trough and the highs' equatorward flank are the Hadley
	// cell's rise: see hadley.
	beltWidth = 10.0
	capWidth  = 12.0
	// beltPole is the latitude of the polar highs.
	beltPole = 88.0
	// edgeLeast and edgeMost hold the edge to a cell a planet could have.
	edgeLeast = 10.0
	edgeMost  = 60.0
	// tropicReach is how far from the equator, in degrees, the land is
	// counted that carries the ITCZ toward the land's: the Hadley cells' own
	// reach.
	tropicReach = 30.0
	// ascentHalf is how many degrees either side of the ITCZ the air rises:
	// the descent begins beyond it.
	ascentHalf = 8.0
	// descentSpill is how many degrees past the edge the descent of the
	// subtropical highs reaches.
	descentSpill = 5.0
	// subsideMost is how fast the air comes down where the Hadley cell's
	// descent is strongest, metres a second at 500 hPa: some thirty hPa a
	// day, the zonal mean's under the subtropical highs (Peixoto and Oort,
	// 1992).
	subsideMost = 0.004
	// lidLeast is where the trade-wind inversion stands, in metres, under the
	// strongest descent: the boundary layer's entrainment, some four
	// millimetres a second, over the divergence under the lid, some four in
	// a million a second (Lilly, 1968; Stevens and others, 2005), a
	// kilometre. Under a weaker descent it stands as much higher as the
	// descent is weaker.
	lidLeast = 1000.0
)

// belts is the circulation on a day of the year: the ITCZ, the Hadley cells'
// northern and southern edges, and how deep the belts are against today's.
type belts struct {
	itcz         float64 // degrees
	shift        float64 // how far the edges have followed the ITCZ, degrees
	north, south float64 // the edges, degrees
	depth, storm float64 // the Hadley belts' and the subpolar lows' depth against today's
}

// legendre2 is the second Legendre polynomial at x.
func legendre2(x float64) float64 { return 1.5*x*x - 0.5 }

// heldHou is Held and Hou's (1980) edge of the Hadley cell, in radians, for a
// radiative equilibrium whose equator stands contrast of the planet's mean
// warmth over its pole, under a tropopause height metres up, on a planet
// spinning at spin radians a second.
func heldHou(contrast, height, spin float64) float64 {
	return math.Sqrt(5.0 / 3 * gravity * height * contrast / (spin * spin * planetRadius * planetRadius))
}

// hadleyEdge is the edge of the Hadley cell under the balance c, degrees:
// today's real edge, moved as Held and Hou's moves from today's.
func hadleyEdge(c *ebmClimate) float64 {
	t := ebm()
	edge := hadleyToday * heldHou(c.contrast, c.tropopause, omega) / heldHou(t.contrast, t.tropopause, omega)
	return math.Max(edgeLeast, math.Min(edgeMost, edge))
}

// midlatitudeFall is the balance's fall of warmth from thirty degrees to
// sixty, which the storms of the middle latitudes live on.
func midlatitudeFall(c *ebmClimate) float64 {
	return c.at(&c.mean, 30) - c.at(&c.mean, 60)
}

// on is a reading's year sinT of the way into the north's summer, at the
// crest of its own year as the wind's phases are.
func (y yearOf) on(sinT float64) float64 { return y.mean + y.swing*sinT }

// itcz is the energy flux equator under the balance c, degrees, sinT of the
// way into the north's summer, on a planet whose tropics are land of land:
// the balance's sea's with none, its band's at its own ebmLand, its land's
// with nothing but land, and between them in proportion.
func (c *ebmClimate) itcz(land, sinT float64) float64 {
	sea, band, ground := c.equator[2].on(sinT), c.equator[0].on(sinT), c.equator[1].on(sinT)
	land = clamp01(land)
	if land <= ebmLand {
		return sea + (band-sea)*land/ebmLand
	}
	return band + (ground-band)*(land-ebmLand)/(1-ebmLand)
}

// beltsAt is the circulation sinT of the way into the north's summer.
func (e *Env) beltsAt(sinT float64) belts {
	c := e.circ
	// The land under the tropics of the summer's hemisphere; a valley is one
	// latitude's weather, and its air has the balance's band's.
	land := float64(ebmLand)
	if e.Wrap {
		north := (1 + sinT) / 2
		land = e.tropicN*north + e.tropicS*(1-north)
	}
	b := belts{itcz: c.itcz(land, sinT)}
	edge := hadleyEdge(c)
	b.shift = edgeShare * b.itcz
	b.north, b.south = edge+b.shift, -edge+b.shift
	r := edge / hadleyToday
	b.depth = r * r
	b.storm = midlatitudeFall(c) / midlatitudeFall(ebm())
	return b
}

// pressure is the pressure at sea level the circulation alone lays down at a
// latitude, with the year sinT of the way into the north's summer: the
// equatorial trough on the ITCZ, the subtropical highs on the Hadley cells'
// edges, the subpolar lows a Ferrel cell further, deeper in their own
// winter, and the polar highs.
func (b *belts) pressure(lat, sinT float64) float64 {
	bump := func(x float64) float64 { return math.Exp(-x * x) }
	l := lat - b.shift
	a := math.Abs(l)
	winter := 1 - beltWinter*sinT*math.Copysign(1, l)
	sub := b.north - b.shift + ferrelBreadth
	if l < 0 {
		sub = b.shift - b.south + ferrelBreadth
	}
	return b.hadley(lat) - beltPolar*b.storm*winter*bump((a-sub)/beltWidth) + beltCap*bump((a-beltPole)/capWidth)
}

// hadley is the pressure at sea level the Hadley cells lay down at a
// latitude: the equatorial trough on the ITCZ, rising across each cell to the
// subtropical high on its edge, and falling off the high's poleward flank.
//
// The rise across the cell is what drives the trades. The wind near the
// ground is the balance of the pull down the gradient, the turning of the
// planet and the drag (see Solve), and over the sea the drag matches the
// turning at some fifteen degrees: nearer the equator the drag holds the
// wind back, and further out the turning does, as 1/f. So the trades blow
// hardest a little equatorward of where the rise is steepest. The earth's
// blow hardest in the inner half of the cell, some fifteen degrees out
// (Peixoto and Oort, 1992), which is where Held and Hou's (1980) cell has
// them too: its surface wind balances the angular momentum the air aloft
// carries poleward, and its easterlies are strongest at 0.43 of the way to
// its edge, fourteen degrees on today's thirty-two (Lindzen and Nigam,
// 1987, read the same easterlies off the boundary layer's pressure).
//
// The rise used to be the trough and the highs as two bumps beltWidth
// broad. Their sum rises twice as steeply on the highs' own flank, at
// twenty-two to twenty-seven degrees, as within ten of the trough, and the
// trades blew hardest at twenty-four, where the earth's are past their best,
// with the tropical band of cyclonic curl, and the open ocean's upwelling
// under it, at nine to twenty. Here the cell's whole rise, the same
// beltEquator and beltHorse, is laid as the square of a sine from the ITCZ
// to the edge: level in the doldrums and under the high's crest, steepest
// half-way. On a planet of sea the trades then blow hardest at some
// seventeen degrees either side.
func (b *belts) hadley(lat float64) float64 {
	edge := b.north
	if lat < b.itcz {
		edge = b.south
	}
	trough, high := beltMean-beltEquator*b.depth, beltMean+beltHorse*b.depth
	x := (lat - b.itcz) / (edge - b.itcz)
	if x >= 1 {
		d := (lat - edge) / beltWidth
		return beltMean + beltHorse*b.depth*math.Exp(-d*d)
	}
	s := math.Sin(math.Pi / 2 * x)
	return trough + (high-trough)*s*s
}

// subsidence is how fast the Hadley cell's air comes down at a latitude,
// metres a second at 500 hPa: nothing where it rises, within ascentHalf of
// the ITCZ, and nothing poleward of the subtropical highs; between, a hump
// over each hemisphere's descending branch.
func (b *belts) subsidence(lat float64) float64 {
	hump := func(x, from, to float64) float64 {
		if to <= from || x <= from || x >= to {
			return 0
		}
		s := math.Sin(math.Pi * (x - from) / (to - from))
		return s * s
	}
	s := hump(lat, b.itcz+ascentHalf, b.north+descentSpill) + hump(-lat, -b.itcz+ascentHalf, -b.south+descentSpill)
	return subsideMost * b.depth * s
}

// stormBand is the band of latitude, degrees from the equator in the
// hemisphere hemi, the day's lows are born in: from the Hadley cell's edge,
// where the subtropical jet stands, a Ferrel cell's breadth poleward -
// today's thirty-two to sixty-two in the year's mean.
func (b *belts) stormBand(hemi float64) (from, to float64) {
	edge := b.north
	if hemi < 0 {
		edge = -b.south
	}
	return edge, math.Min(edge+ferrelBreadth, 85)
}

// lid is the height of the trade-wind inversion, metres, under air coming
// down at w metres a second, and infinite where none does.
func lid(w float64) float64 {
	if w <= 0 {
		return math.Inf(1)
	}
	return lidLeast * subsideMost / w
}

// lidKeeps is how much of the rain the column of air at temp degrees would
// give it gives under a lid at height metres: the share of its water, which
// thins with height over the vapour's scale height, that lies under the lid.
// The air over it is the descent's, and dry.
func lidKeeps(height, temp float64) float64 {
	if math.IsInf(height, 1) {
		return 1
	}
	return 1 - math.Exp(-height/vapourScale(temp))
}

// vapourScale is H_w, the height over which the water in air at temp degrees
// near the ground thins by e, metres: Smith and Barstad's (2004).
func vapourScale(temp float64) float64 {
	tk := temp + 273.15
	return vapourGas * tk * tk / (latentHeat * Lapse)
}

// descents works out the land under each hemisphere's tropics, and the
// Hadley cell's descent over each cell in each phase of the year.
func (e *Env) descents() {
	n := e.W * e.H
	if e.Wrap {
		var landN, areaN, landS, areaS float64
		for cy := 0; cy < e.H; cy++ {
			lat := e.lat[cy]
			if math.Abs(lat) >= tropicReach {
				continue
			}
			var land float64
			for cx := 0; cx < e.W; cx++ {
				land += 1 - e.Sea[cy*e.W+cx]
			}
			// A row's area goes as its cells' breadth.
			land *= e.Dx[cy]
			area := float64(e.W) * e.Dx[cy]
			if lat >= 0 {
				landN, areaN = landN+land, areaN+area
			} else {
				landS, areaS = landS+land, areaS+area
			}
		}
		e.tropicN, e.tropicS = landN/math.Max(areaN, 1), landS/math.Max(areaS, 1)
	}
	for k := range Phases {
		b := e.beltsAt(phaseSin[k])
		e.Subsides[k] = make([]float64, n)
		for cy := 0; cy < e.H; cy++ {
			w := b.subsidence(e.lat[cy])
			for cx := 0; cx < e.W; cx++ {
				e.Subsides[k][cy*e.W+cx] = w
			}
		}
	}
}

// SubsidenceOn is how fast the air comes down over tile i of the map on a day
// of the year, metres a second at 500 hPa: the Hadley cell's descending
// branch, under the subtropical highs.
func (w *Winds) SubsidenceOn(i, day int) float64 {
	fx, fy := w.CellAt(i)
	var s float64
	for k, m := range seasonWeights(day) {
		s += m * w.Sample(w.Subsides[k], fx, fy)
	}
	return s
}

// SubsideMost is how fast the air comes down where the Hadley cell's descent
// is strongest, metres a second at 500 hPa. See subsideMost.
const SubsideMost = subsideMost

// DescentAt is how fast the air comes down over tile i of the map on the
// year's mean, metres a second at 500 hPa: SubsidenceOn's four phases taken
// alike. Nothing where the descent is not worked out.
func (w *Winds) DescentAt(i int) float64 {
	fx, fy := w.CellAt(i)
	var s float64
	for k := range Phases {
		if w.Subsides[k] == nil {
			return 0
		}
		s += w.Sample(w.Subsides[k], fx, fy) / Phases
	}
	return s
}

// InversionOn is the height of the trade-wind inversion over tile i of the
// map on a day of the year, metres over the sea, and infinite where the air
// does not come down.
func (w *Winds) InversionOn(i, day int) float64 { return lid(w.SubsidenceOn(i, day)) }

// Circulation is the general circulation under a forcing, as degrees of
// latitude, sinT of the way into the north's summer: the Hadley cells'
// northern and southern edges and the ITCZ on a planet whose tropics are the
// balance's band's, three tenths land; the ITCZ on one all sea and one all
// land; and Held and Hou's own edge before it is set against today's real
// one, with the contrast and the tropopause it is worked out from.
type Circulation struct {
	North, South            float64
	ITCZ, ITCZSea, ITCZLand float64
	HeldHou                 float64
	Contrast, Tropopause    float64
}

// CirculationUnder is the circulation under forcing f, sinT of the way into
// the north's summer.
func CirculationUnder(f Forcing, sinT float64) Circulation {
	c := ebmUnder(f.OrDefault())
	edge := hadleyEdge(c)
	itcz := c.itcz(ebmLand, sinT)
	shift := edgeShare * itcz
	return Circulation{
		North: edge + shift, South: -edge + shift,
		ITCZ: itcz, ITCZSea: c.itcz(0, sinT), ITCZLand: c.itcz(1, sinT),
		HeldHou:  heldHou(c.contrast, c.tropopause, omega) * 180 / math.Pi,
		Contrast: c.contrast, Tropopause: c.tropopause,
	}
}

// ITCZ is the latitude of the ITCZ over the map, degrees, sinT of the way
// into the north's summer: the energy flux equator, read by the land under
// the summer hemisphere's tropics. See beltsAt.
func (e *Env) ITCZ(sinT float64) float64 { return e.beltsAt(sinT).itcz }
