package terra

import "math"

// The sea's own weather: where the water goes, and the warmth it takes there.
//
// The sea was one sea. Its warmth was its latitude's and its season's, every
// cell of it filled the air the same, and a coast was as mild as the share of
// water round it whichever side of an ocean it stood on. The real world is not
// like that, and the difference is some of the largest on the map. The Gulf
// Stream and the Kuroshio carry tropical water up the western side of their
// oceans, and the coasts they pass are wet and mild; the water that comes back
// down the eastern side is cold, and where the wind drives the surface water
// off an eastern shore colder water still comes up from under it, and the
// coast beside it - the Atacama, the Namib, Baja, the western Sahara - is a
// desert with the sea in sight. The wind belts alone give none of it.
//
// What the water does is worked out from the wind the air already has, on the
// air's own cells, in the order an oceanographer reads it:
//
//   - The wind's pull on the sea, the stress, which goes as the square of the
//     wind (Large and Pond, 1981).
//   - The gyres. Where that pull turns - the trades one way, the westerlies the
//     other - the water across an ocean is driven toward the equator or the
//     pole by Sverdrup's balance, and what is driven one way across the
//     breadth of the ocean comes back the other in a narrow current against
//     its western shore (Stommel, 1948; Munk, 1950). A parallel with no land
//     on it has no shore to turn at, and no gyre.
//   - The water the gyres drive toward the poles and the equator has to come
//     from somewhere, and goes east and west to get there: this is what takes
//     the western current out across the ocean where its gyre ends, the Gulf
//     Stream into the North Atlantic Drift. On top of it the surface water
//     drifts a few hundredths of the speed of the wind over it.
//   - Upwelling. The water the wind drives goes to the right of it in the
//     north and the left in the south (Ekman, 1905), and where that takes it
//     off a shore, cold water comes up from under to take its place.
//   - The warmth all of that carries: the water keeps the warmth of where it
//     came from, and gives it up to the air over some months.
//
// It is done once a year's wind, from the year's mean wind, and only on a
// globe: a valley is a few dozen kilometres of country and has no ocean to
// have gyres in, and is untouched by any of this to the bit.

// The sea.
const (
	// seaDensity is the density of sea water, kg a cubic metre.
	seaDensity = 1025.0
	// planetRadius is the planet's radius, in metres.
	planetRadius = 6.371e6
	// stressDrag is the drag of the sea surface on the wind over it: the
	// ordinary bulk figure for a moderate wind (Large and Pond).
	stressDrag = 1.3e-3
	// westWall is how wide, in metres, the current against an ocean's western
	// shore is. The Gulf Stream off Carolina is a hundred kilometres across
	// and the Kuroshio a little more; it is never narrower than a cell.
	westWall = 150e3
	// gyreDepth is how deep, in metres, the water driven round a gyre goes: its
	// transport over this is how fast the surface of it goes. Thirty million
	// cubic metres a second in a current a hundred and fifty kilometres wide
	// comes to some seventy centimetres a second, which is what the Gulf
	// Stream's surface runs at off the Carolinas.
	gyreDepth = 300.0
	// ekmanDepth is how deep, in metres, the water the wind drives straight
	// off is: the Ekman layer, some fifty metres. What it carries goes a
	// quarter turn to the right of the wind in the north, and to the left in
	// the south, and the turning is read as never less than it is at
	// upwellLow degrees, where it is gone on the equator.
	ekmanDepth = 50.0
	// currentMost is more than any surface current runs at, in metres a
	// second, as a guard against the arithmetic near the poles.
	currentMost = 2.0
	// mixedTropic and mixedPolar are how deep, in metres over the year, the
	// water the air warms and cools is: the ocean's mixed layer, some fifty
	// metres under the steady trades and some three hundred where the winter
	// storms of the fifties and sixties stir it (de Boyer Montégut, 2004). It
	// deepens between mixedLow and mixedHigh degrees.
	mixedTropic = 50.0
	mixedPolar  = 300.0
	mixedLow    = 20.0
	mixedHigh   = 60.0
	// seaExchange is how many watts a square metre the sea and the air trade
	// for each degree between them, and seaHeat how many joules a cubic metre
	// of sea water holds a degree. Fifty metres of water gives up its warmth by
	// a factor of e in some two and a half months, three hundred in some a
	// year and a third: which is why the warmth the Gulf Stream brings north
	// is still in the water when it reaches Norway.
	seaExchange = 30.0
	seaHeat     = 4.1e6
	// upwellContrast is how much colder, in degrees, the water under the mixed
	// layer is than the surface at the equator's side of the subtropics; it
	// falls away toward the poles, where the ocean is mixed from top to bottom
	// in winter. The water off Peru and Namibia comes up seven or eight
	// degrees colder than the open ocean at its latitude.
	upwellContrast = 10.0
	// seaWarmMost is the most degrees the sea stands warmer or colder than its
	// latitude: the Gulf Stream at the Grand Banks, some eight or ten over the
	// water beside it, is about the most the real world has.
	seaWarmMost = 10.0
	// gyreCalm is how near the equator, in degrees, the gyres are not worked
	// out: the turning of the planet that holds them to their balance goes to
	// nothing there. And gyreCap is how near the pole.
	gyreCalm = 5.0
	gyreCap  = 80.0
	// upwellLow is how far from the equator, in degrees, a coast's upwelling
	// comes into its own: the Benguela and the Humboldt are strongest from
	// fifteen degrees to thirty.
	upwellLow = 15.0
	// cornerReach is how far along a row, in metres, a current running up or
	// down a coast that slants looks for the water it came from.
	cornerReach = 600e3
	// gyreRows is how many rows of cells either way the gyres' currents are
	// taken over.
	gyreRows = 2
	// gyreOpen is how much of the way round a parallel a stretch of sea has to
	// run for its shores not to close a gyre.
	gyreOpen = 0.8
	// coastReach is how far round a place on land, in kilometres, the water
	// off its coast is felt: a sea breeze's reach and a little more.
	coastReach = 300.0
)

// What the water's warmth does to the air's rain.
const (
	// seaDamp is how much more water, as a share a degree, air takes off a
	// warmer sea: the Clausius-Clapeyron seven per cent.
	seaDamp = 0.07
	// inversionCold is how many degrees under its latitude cold water has to lie
	// for the air over it to be held down half as hard as it can be. Air
	// cooled from under over cold water is stable, lies under an inversion and
	// does not rise: the coast of Peru is under cloud half the year and has a
	// few millimetres of rain in it.
	inversionCold = 1.5
	// inversionMost is how much of the rain the steadiest inversion takes.
	inversionMost = 0.9
)

// currents works out the water under the year's mean wind u, v and gives
// each cell's warmth: how many degrees the sea there stands over the mean of
// its latitude, and nothing on land.
func (e *airEnv) currents(u, v [phases][]float32) []float64 {
	n := e.w * e.h
	wet := func(i int) bool { return e.sea[i] > 0.5 }

	// The wind's stress on the sea, in newtons a square metre, from the year's
	// mean wind; and the current it drifts the surface at.
	tx, ty := make([]float64, n), make([]float64, n)
	cu, cv := make([]float64, n), make([]float64, n)
	for i := range n {
		var mu, mv float64
		for k := range phases {
			mu += float64(u[k][i]) / phases
			mv += float64(v[k][i]) / phases
		}
		s := math.Hypot(mu, mv)
		tx[i] = airDensity * stressDrag * s * mu
		ty[i] = airDensity * stressDrag * s * mv
	}
	ekman := func(i, cy int) (east, north float64) {
		f := 2 * omega * math.Max(math.Sin(math.Abs(e.lat[cy])*math.Pi/180), math.Sin(upwellLow*math.Pi/180))
		f = math.Copysign(f, e.lat[cy])
		return ty[i] / (seaDensity * f), -tx[i] / (seaDensity * f)
	}

	// The gyres. On each row, each stretch of sea between shores is driven
	// toward the equator or the pole by the turning of the wind's stress, and
	// what that takes one way comes back the other against the western shore.
	for cy := 0; cy < e.h; cy++ {
		lat := e.lat[cy]
		a := math.Abs(lat)
		if a < gyreCalm || a > gyreCap {
			continue
		}
		beta := 2 * omega * math.Cos(lat*math.Pi/180) / planetRadius
		row := cy * e.w
		// Where the row's first shore is: the eastern end of a stretch of land,
		// from which the stretches of sea can be walked eastward round the
		// seam. A row with no land has no shore.
		start := -1
		for cx := 0; cx < e.w; cx++ {
			if !wet(row+cx) && wet(e.at(cx+1, cy)) {
				start = cx + 1
				break
			}
		}
		if start < 0 {
			continue
		}
		if !e.wrap {
			start = 0
		}
		dx := e.dx[cy]
		wall := max(1, int(math.Round(westWall/dx)))
		span := e.w
		for k := 0; k < span; {
			i := e.at(start+k, cy)
			if !wet(i) {
				k++
				continue
			}
			// A stretch of sea, from its western shore to its eastern.
			first := k
			for k < span && wet(e.at(start+k, cy)) {
				k++
			}
			last := k - 1
			if !e.wrap && (first == 0 || last == span-1) {
				// A valley's sea runs off the map, and the map has no say in
				// where its gyre closes.
				continue
			}
			if last-first+1 <= wall || float64(last-first+1) > gyreOpen*float64(e.w) {
				// Too narrow to turn in, or an ocean so nearly all the way
				// round that the islands in it do not close it: the Southern
				// Ocean's current goes round Drake Passage, not back up it.
				continue
			}
			var interior float64
			for j := first + wall; j <= last; j++ {
				c := e.at(start+j, cy)
				cx := c - row
				curl := (ty[e.at(cx+1, cy)]-ty[e.at(cx-1, cy)])/(2*dx) -
					(tx[e.at(cx, cy-1)]-tx[e.at(cx, cy+1)])/(2*e.dy)
				flow := curl / (seaDensity * beta)
				cv[c] = flow / gyreDepth
				interior += flow * dx
			}
			back := -interior / (float64(wall) * dx) / gyreDepth
			for j := first; j < first+wall; j++ {
				cv[e.at(start+j, cy)] = back
			}
		}
	}
	// Each row's balance is its own, and a ragged coast gives neighbouring rows
	// oceans of different breadths and currents that differ row by row far
	// more than the water does: the currents are taken over gyreRows rows of
	// sea either way.
	{
		mask := make([]float64, n)
		for i := range mask {
			if wet(i) {
				mask[i] = 1
			}
		}
		flat := make([]int, e.h)
		held, share := e.box(cv, flat, gyreRows), e.box(mask, flat, gyreRows)
		for i := range cv {
			if wet(i) && share[i] > 0 {
				cv[i] = held[i] / share[i]
			}
		}
	}
	// And the water the gyres drive north and south has to go east and west to
	// get there: none of it goes through an eastern shore, so what leaves each
	// cell toward the poles or the equator is made up from the cell east of
	// it. This is what takes the western current across the ocean where the
	// gyre it runs round ends, the Gulf Stream into the North Atlantic Drift.
	gu := make([]float64, n)
	for cy := 0; cy < e.h; cy++ {
		nr, sr := max(cy-1, 0), min(cy+1, e.h-1)
		row := cy * e.w
		dx := e.dx[cy]
		// Walked westward from each eastern shore, round the seam on a globe.
		start := -1
		for cx := 0; cx < e.w; cx++ {
			if !wet(row+cx) && wet(e.at(cx-1, cy)) {
				start = (cx - 1 + e.w) % e.w
				break
			}
		}
		if start < 0 {
			continue
		}
		// The next row's ocean is not the same breadth as this one's, and
		// what is left over at the western shore is shared back across the
		// stretch rather than run into the land.
		var flow float64
		var stretch []int
		shut := func() {
			for k, i := range stretch {
				gu[i] -= flow * float64(k+1) / float64(len(stretch))
			}
			flow, stretch = 0, stretch[:0]
		}
		for k := 0; k <= e.w; k++ {
			cx := ((start-k)%e.w + e.w) % e.w
			i := row + cx
			if !wet(i) || k == e.w {
				shut()
				continue
			}
			north := (cv[nr*e.w+cx] - cv[sr*e.w+cx]) / (2 * e.dy)
			flow += north * dx
			gu[i] = flow
			stretch = append(stretch, i)
		}
	}
	for i := range n {
		mx, my := ekman(i, i/e.w)
		cu[i] += gu[i] + mx/ekmanDepth
		cv[i] += my / ekmanDepth
		if !wet(i) {
			cu[i], cv[i] = 0, 0
		}
		cu[i] = math.Max(-currentMost, math.Min(currentMost, cu[i]))
		cv[i] = math.Max(-currentMost, math.Min(currentMost, cv[i]))
	}

	// Upwelling: how fast, in metres a second, the water the wind drives off a
	// shore is replaced from under, and how much colder what comes up is.
	rise := make([]float64, n)
	deep := make([]float64, n)
	land := make([]float64, n)
	for i := range land {
		land[i] = 1 - e.sea[i]
	}
	for cy := 0; cy < e.h; cy++ {
		lat := e.lat[cy]
		c := math.Cos(lat * math.Pi / 180)
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			deep[i] = e.mean[cy] - upwellContrast*c*c
			if !wet(i) || math.Abs(lat) < gyreCalm {
				continue
			}
			gx, gy := e.grad(land, cx, cy)
			g := math.Hypot(gx, gy)
			if g == 0 {
				continue
			}
			// The water the wind drives, a quarter turn to the right of it in
			// the north and to the left in the south, and how much of it goes
			// away from the land.
			f := e.f[cy]
			mx, my := ty[i]/(seaDensity*f), -tx[i]/(seaDensity*f)
			off := -(mx*gx + my*gy) / g
			// Near the equator the turning that sends the water off the
			// shore goes to nothing, and what sends it there instead is the
			// open ocean's business rather than a coast's.
			if off > 0 {
				rise[i] = off / math.Min(e.dx[cy], e.dy) * smoothstep(gyreCalm, upwellLow, math.Abs(lat))
			}
		}
	}

	// The warmth of the water, where each cell's is what the water coming into
	// it carries, what comes up from under, and the air's pull back toward the
	// latitude's own, all in balance. One equation a cell, swept in the four
	// orders a current can run in until it settles, the same as the air's
	// moisture.
	temp := make([]float64, n)
	for i := range temp {
		temp[i] = e.mean[i/e.w]
	}
	depth, relax := make([]float64, e.h), make([]float64, e.h)
	for cy := range depth {
		depth[cy] = mixedTropic + (mixedPolar-mixedTropic)*smoothstep(mixedLow, mixedHigh, math.Abs(e.lat[cy]))
		relax[cy] = seaExchange / (seaHeat * depth[cy])
	}
	dy := e.dy
	type order struct{ x0, x1, dx, y0, y1, dy int }
	orders := [4]order{
		{0, e.w, 1, 0, e.h, 1}, {e.w - 1, -1, -1, 0, e.h, 1},
		{0, e.w, 1, e.h - 1, -1, -1}, {e.w - 1, -1, -1, e.h - 1, -1, -1},
	}
	for round := 0; round < seaRounds; round++ {
		most := 0.0
		for _, o := range orders {
			for cy := o.y0; cy != o.y1; cy += o.dy {
				row := cy * e.w
				for cx := o.x0; cx != o.x1; cx += o.dx {
					i := row + cx
					if !wet(i) {
						continue
					}
					r := rise[i] / depth[cy]
					sum := relax[cy]*e.mean[cy] + r*deep[i]
					take := relax[cy] + r
					if a := math.Abs(cu[i]) / e.dx[cy]; a > 0 {
						ux := cx - int(math.Copysign(1, cu[i]))
						if e.wrap || (ux >= 0 && ux < e.w) {
							if j := e.at(ux, cy); wet(j) {
								sum, take = sum+a*temp[j], take+a
							}
						}
					}
					// Toward the north is up the map, so water going north
					// comes from the row below.
					// A current running along a coast that does not run due
					// north and south comes in round the corner of it: from the
					// nearest sea along the row behind it, within cornerReach.
					if b := math.Abs(cv[i]) / dy; b > 0 {
						if uy := cy + int(math.Copysign(1, cv[i])); uy >= 0 && uy < e.h {
							reach := int(math.Ceil(cornerReach / e.dx[uy]))
							for side := 0; side <= reach; side++ {
								if j := e.at(cx+side, uy); wet(j) {
									sum, take = sum+b*temp[j], take+b
									break
								}
								if j := e.at(cx-side, uy); side > 0 && wet(j) {
									sum, take = sum+b*temp[j], take+b
									break
								}
							}
						}
					}
					next := sum / take
					if d := math.Abs(next - temp[i]); d > most {
						most = d
					}
					temp[i] = next
				}
			}
		}
		if most < seaSettled {
			break
		}
	}

	warm := make([]float64, n)
	for i := range warm {
		if wet(i) {
			warm[i] = math.Max(-seaWarmMost, math.Min(seaWarmMost, temp[i]-e.mean[i/e.w]))
		}
	}
	// The land along a shore is given the warmth of the sea beside it, so
	// that the sea read between the cells right up to the coast is the sea's
	// and not half the land's nothing: the coldest water there is lies against
	// the shore.
	shore := make([]float64, n)
	for cy := 0; cy < e.h; cy++ {
		for cx := 0; cx < e.w; cx++ {
			i := cy*e.w + cx
			if wet(i) {
				continue
			}
			var s, k float64
			for dy := -1; dy <= 1; dy++ {
				if cy+dy < 0 || cy+dy >= e.h {
					continue
				}
				for dx := -1; dx <= 1; dx++ {
					if j := e.at(cx+dx, cy+dy); wet(j) {
						s, k = s+warm[j], k+1
					}
				}
			}
			if k > 0 {
				shore[i] = s / k
			}
		}
	}
	for i, s := range shore {
		if !wet(i) {
			warm[i] = s
		}
	}
	return warm
}

// seaRounds is the most rounds of the four sweeps the water's warmth is given
// to settle, and seaSettled the change in degrees in a round that is settled.
// A western current carries its warmth the length of an ocean in a round; the
// slow drift across the interior takes more.
const (
	seaRounds  = 40
	seaSettled = 1e-3
)

// coastal is the sea's warmth as the country round each cell feels it: the
// mean warmth of the sea within coastReach of it, felt in full where a third
// of that country is sea and less where less is. A coast in a warm current
// has all of it, and a place a few hundred kilometres inland none.
func (e *airEnv) coastal(warm []float64) []float64 {
	out := make([]float64, len(warm))
	for i, t := range warm {
		out[i] = t * e.sea[i]
	}
	out = e.blur(out, coastReach)
	share := e.blur(e.sea, coastReach)
	for i, s := range share {
		if s > 1e-6 {
			out[i] = out[i] / s * smoothstep(0, coastShare, s)
		} else {
			out[i] = 0
		}
	}
	return out
}

// coastShare is how much of the country round a place has to be sea for the
// place to feel the sea's warmth in full.
const coastShare = 1.0 / 3

// damp is how many times the air's ordinary fill of water the sea gives it
// where the water stands warm degrees over its latitude.
func damp(warm float64) float64 { return math.Exp(seaDamp * warm) }

// inversion is how much of the rain the air over a place keeps when the sea
// round it stands warm degrees over its latitude: all of it over warm water,
// and much less where cold water holds the air down.
func inversion(warm float64) float64 {
	if warm >= 0 {
		return 1
	}
	c := warm / inversionCold
	return 1 - inversionMost*c*c/(1+c*c)
}

// SeaWarmth is how many degrees the sea over tile i stands warmer than the
// mean of its latitude, for the currents: warm in a western current, cold in
// an eastern one and colder where the water comes up from under. It is
// nothing on land, on a valley, and on a map whose weather has not been read.
func (g *Grid) SeaWarmth(i int) float64 {
	w := g.winds
	if w == nil || w.warm == nil || i < 0 || i >= len(g.Tiles) {
		return 0
	}
	fx, fy := w.cellAt(g, i)
	return w.sample(w.warm, fx, fy)
}

// CoastWarmth is how many degrees the currents offshore make the year's mean
// on tile i warmer or colder: Norway's mildness, and the chill of the fog off
// Peru. It is nothing on a valley and on a map whose weather has not been
// read.
func (g *Grid) CoastWarmth(i int) float64 {
	w := g.winds
	if w == nil || w.coast == nil || i < 0 || i >= len(g.Tiles) {
		return 0
	}
	fx, fy := w.cellAt(g, i)
	return w.sample(w.coast, fx, fy)
}
