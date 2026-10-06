package atmos

import (
	"math"

	"github.com/LukasSelin/terra/internal/phase"
)

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
//     its western shore (Stommel, 1948; Munk, 1950). The water goes round the
//     whole ocean at once, so the western current turns out across the ocean
//     where its gyre ends, the Gulf Stream into the North Atlantic Drift; it
//     goes round islands and through the straits between them, and where the
//     sea runs all the way round the planet, round the planet. See flow.go.
//     On top of it the surface water drifts a few hundredths of the speed of
//     the wind over it.
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
	// SeaDensity is the density of sea water, kg a cubic metre.
	SeaDensity = 1025.0
	// Sverdrup is a million cubic metres a second, the ocean's measure of
	// the water a current carries.
	Sverdrup = 1e6
	// planetRadius is the planet's radius, in metres.
	planetRadius = 6.371e6
	// stressDrag is the drag of the sea surface on the wind over it: the
	// ordinary bulk figure for a moderate wind (Large and Pond).
	stressDrag = 1.3e-3
	// gyreDepth is how deep, in metres, the water driven round a gyre goes: its
	// transport over this is how fast the surface of it goes. Thirty million
	// cubic metres a second in a current a hundred and fifty kilometres wide
	// comes to some seventy centimetres a second, which is what the Gulf
	// Stream's surface runs at off the Carolinas. The thermocline's depth will
	// take its place (docs/ocean-model-plan.md, M2).
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
	// upwellCalm is how near the equator, in degrees, a coast has no
	// upwelling of its own: the turning of the planet that sends the water
	// the wind drives off a shore goes to nothing there.
	upwellCalm = 5.0
	// upwellLow is how far from the equator, in degrees, a coast's upwelling
	// comes into its own: the Benguela and the Humboldt are strongest from
	// fifteen degrees to thirty.
	upwellLow = 15.0
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
// its latitude, and nothing on land. The current, the upwelling and the
// water's temperature it works out on the way are kept on e: see Env.Cu.
func (e *Env) currents(u, v [Phases][]float32) []float64 {
	defer phase.Start("airEnv.currents")()
	n := e.W * e.H
	wet := func(i int) bool { return e.Sea[i] > 0.5 }

	// The wind's stress on the sea, in newtons a square metre, from the year's
	// mean wind; and the current it drifts the surface at.
	tx, ty := make([]float64, n), make([]float64, n)
	cu, cv := make([]float64, n), make([]float64, n)
	for i := range n {
		var mu, mv float64
		for k := range Phases {
			mu += float64(u[k][i]) / Phases
			mv += float64(v[k][i]) / Phases
		}
		s := math.Hypot(mu, mv)
		tx[i] = airDensity * stressDrag * s * mu
		ty[i] = airDensity * stressDrag * s * mv
	}
	ekman := func(i, cy int) (east, north float64) {
		f := 2 * omega * math.Max(math.Sin(math.Abs(e.lat[cy])*math.Pi/180), math.Sin(upwellLow*math.Pi/180))
		f = math.Copysign(f, e.lat[cy])
		return ty[i] / (SeaDensity * f), -tx[i] / (SeaDensity * f)
	}

	// The gyres: the transport the wind drives round each ocean, ψ, and the
	// current it makes over the depth the gyres go to. See flow.go.
	psi := e.gyres(tx, ty)
	for cy := 0; cy < e.H; cy++ {
		up, down, dy := cy-1, cy+1, 2*e.Dy
		if up < 0 {
			up, dy = cy, e.Dy
		}
		if down >= e.H {
			down, dy = cy, e.Dy
		}
		dx := e.Dx[cy]
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			cu[i] = -(psi[up*e.W+cx] - psi[down*e.W+cx]) / dy / gyreDepth
			cv[i] = (psi[e.at(cx+1, cy)] - psi[e.at(cx-1, cy)]) / (2 * dx) / gyreDepth
		}
	}
	e.Psi = make([]float32, n)
	for i, p := range psi {
		e.Psi[i] = float32(p / Sverdrup)
	}
	for i := range n {
		mx, my := ekman(i, i/e.W)
		cu[i] += mx / ekmanDepth
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
		land[i] = 1 - e.Sea[i]
	}
	for cy := 0; cy < e.H; cy++ {
		lat := e.lat[cy]
		c := math.Cos(lat * math.Pi / 180)
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			deep[i] = e.Mean[cy] - upwellContrast*c*c
			if !wet(i) || math.Abs(lat) < upwellCalm {
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
			mx, my := ty[i]/(SeaDensity*f), -tx[i]/(SeaDensity*f)
			off := -(mx*gx + my*gy) / g
			// Near the equator the turning that sends the water off the
			// shore goes to nothing, and what sends it there instead is the
			// open ocean's business rather than a coast's.
			if off > 0 {
				rise[i] = off / math.Min(e.Dx[cy], e.Dy) * smoothstep(upwellCalm, upwellLow, math.Abs(lat))
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
		temp[i] = e.Mean[i/e.W]
	}
	depth, relax := make([]float64, e.H), make([]float64, e.H)
	for cy := range depth {
		depth[cy] = mixedTropic + (mixedPolar-mixedTropic)*smoothstep(mixedLow, mixedHigh, math.Abs(e.lat[cy]))
		relax[cy] = seaExchange / (seaHeat * depth[cy])
	}
	// Each cell's equation does not change while it is solved - the currents,
	// the coast and the upwelling are what they are - so where its water comes
	// from and how hard is found once: the pull toward the latitude and up from
	// under, the cell upstream along the row, and the cell upstream down the
	// column, or across the diagonal where that is land. Land takes nothing.
	sea := e.seaLinks(cu, cv, rise, deep, depth, relax)
	sea.gaussSeidel(temp)
	e.Cu, e.Cv, e.Rise = narrow(cu), narrow(cv), narrow(rise)

	warm := make([]float64, n)
	for i := range warm {
		if wet(i) {
			warm[i] = math.Max(-seaWarmMost, math.Min(seaWarmMost, temp[i]-e.Mean[i/e.W]))
		}
	}
	// The land along a shore is given the warmth of the sea beside it, so
	// that the sea read between the cells right up to the coast is the sea's
	// and not half the land's nothing: the coldest water there is lies against
	// the shore.
	// The water's own temperature is given to the shore the same way; the
	// land away from any sea keeps its latitude's mean.
	shore := make([]float64, n)
	for cy := 0; cy < e.H; cy++ {
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			if wet(i) {
				continue
			}
			var s, st, k float64
			for dy := -1; dy <= 1; dy++ {
				if cy+dy < 0 || cy+dy >= e.H {
					continue
				}
				for dx := -1; dx <= 1; dx++ {
					if j := e.at(cx+dx, cy+dy); wet(j) {
						s, st, k = s+warm[j], st+temp[j], k+1
					}
				}
			}
			if k > 0 {
				// Only the sea's temp is read here, so the land's is written as it goes.
				shore[i], temp[i] = s/k, st/k
			}
		}
	}
	for i, s := range shore {
		if !wet(i) {
			warm[i] = s
		}
	}
	e.WaterTemp = narrow(temp)
	return warm
}

// narrow is v in single precision.
func narrow(v []float64) []float32 {
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(x)
	}
	return out
}

// seaRounds is the most rounds of sweeps, down the rows and back, the water's
// warmth is given to settle, and seaSettled the change in degrees in a round
// that is settled. A current along the rows carries its warmth the length of
// an ocean, or round the planet, in a sweep; one across them a row a sweep.
// Near the poles, where the air's pull on three hundred metres of water takes
// more than a year and the currents go round in a few months, the warmth goes
// round its gyre many times before it settles, and on a globe that has made
// its history the rounds run out first, a hundredth of a degree or so short,
// as they did before the gyres were solved in two dimensions. GMRES on the
// equations, with the sweeps its preconditioner, took three times as long to
// settle it.
const (
	seaRounds  = 40
	seaSettled = 1e-3
)

// seaLinks is each cell's equation for the water's warmth,
//
//	take·t_i = base + wa·t_ja + wb·t_jb
//
// with ja and jb -1 where no water comes in that way, and take nought on land.
type seaLinks struct {
	w, h       int
	base, take []float64
	wa, wb     []float64
	ja, jb     []int32
	// up is ja as a column of its own row, or -1.
	up []int32
}

// seaLinks writes each cell's equation down. It takes over deep for its own
// take, since nothing reads it once the warmth is being solved, and writes
// its base and weights to scratch of its own: the currents and the upwelling
// are kept (Env.Cu). A cell's are the only ones it reads.
func (e *Env) seaLinks(cu, cv, rise, deep, depth, relax []float64) *seaLinks {
	n := e.W * e.H
	l := &seaLinks{
		w: e.W, h: e.H,
		base: make([]float64, n), take: deep,
		wa: make([]float64, n), wb: make([]float64, n),
		ja: make([]int32, n), jb: make([]int32, n),
	}
	dy := e.Dy
	for cy := 0; cy < e.H; cy++ {
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			l.ja[i], l.jb[i] = -1, -1
			if e.Sea[i] <= 0.5 {
				l.take[i] = 0
				continue
			}
			r := rise[i] / depth[cy]
			sum := relax[cy]*e.Mean[cy] + r*deep[i]
			take := relax[cy] + r
			a := math.Abs(cu[i]) / e.Dx[cy]
			b := math.Abs(cv[i]) / dy
			cvi := cv[i]
			if a > 0 {
				ux := cx - int(math.Copysign(1, cu[i]))
				if e.Wrap || (ux >= 0 && ux < e.W) {
					if j := e.at(ux, cy); e.Sea[j] > 0.5 {
						take += a
						l.ja[i], l.wa[i] = int32(j), a
					}
				}
			}
			// Toward the north is up the map, so water going north comes from
			// the row below. The current runs along the coast, so where the cell
			// behind it in the column is land, the water came round the corner:
			// from the cell behind it across the diagonal, on the side the
			// current comes from along the row.
			if b > 0 {
				if uy := cy + int(math.Copysign(1, cvi)); uy >= 0 && uy < e.H {
					j := e.at(cx, uy)
					if e.Sea[j] <= 0.5 && cu[i] != 0 {
						j = e.at(cx-int(math.Copysign(1, cu[i])), uy)
					}
					if e.Sea[j] > 0.5 {
						take += b
						l.jb[i], l.wb[i] = int32(j), b
					}
				}
			}
			l.base[i], l.take[i] = sum, take
		}
	}
	l.up = make([]int32, n)
	for i, j := range l.ja {
		l.up[i] = -1
		if j >= 0 {
			l.up[i] = j % int32(l.w)
		}
	}
	return l
}

// gaussSeidel settles t in place, a row at a time, the rows swept north to
// south and back. Along its row each cell's water comes from the cell east or
// west of it, so the row's warmths given the rows either side are a chain, or
// a ring of chains all the way round a parallel, solved exactly in one pass:
// a current carries its warmth the length of an ocean, or round the planet,
// in one, where swept a cell at a time in the four orders a current can run
// in, the water going round a ring of sea took a round for each time round
// it. Read from the round before (Jacobi), so that a round
// could be spread over goroutines and vectors, the warmth moved a cell a
// round: on a quarter globe that took 4.7 times the sweeps, was slower all
// told, and settled on warmths up to 7.6 degrees apart. See
// docs/perf/worklog.md.
func (l *seaLinks) gaussSeidel(t []float64) {
	c, g := make([]float64, l.w), make([]float64, l.w)
	state := make([]int8, l.w)
	stack := make([]int, 0, l.w)
	// A row is swept again only while it, or a row either side, which is
	// all a row's water comes from, changed by more than seaSettled in the
	// round before: on a globe that has made its history, all but a few
	// rows near the poles have settled within a few rounds.
	moved, active := make([]bool, l.h), make([]bool, l.h)
	for cy := range active {
		active[cy] = true
	}
	for round := 0; round < seaRounds; round++ {
		most := 0.0
		clear(moved)
		for k := 0; k < 2*l.h; k++ {
			cy := k
			if k >= l.h {
				cy = 2*l.h - 1 - k
			}
			if !active[cy] {
				continue
			}
			d := l.row(cy, t, c, g, state, stack)
			most = math.Max(most, d)
			if d >= seaSettled {
				moved[cy] = true
			}
		}
		if most < seaSettled {
			return
		}
		for cy := range active {
			active[cy] = moved[cy] || (cy > 0 && moved[cy-1]) || (cy < l.h-1 && moved[cy+1])
		}
	}
}

// row solves row cy's warmths exactly for the warmths of the rows either
// side, and is the most any of them changed. Each cell's is c + g times the
// warmth of the cell upstream of it along the row: each is found by walking
// upstream to a cell already found, or to where no water comes in along the
// row, or round a ring back to itself, where the ring's warmth is the one
// that comes back to itself.
func (l *seaLinks) row(cy int, t, c, g []float64, state []int8, stack []int) float64 {
	row := cy * l.w
	ts := t[row : row+l.w]
	up := l.up[row : row+l.w]
	for x := range l.w {
		i := row + x
		if l.take[i] == 0 {
			state[x] = 2
			continue
		}
		state[x] = 0
		s := l.base[i]
		if j := l.jb[i]; j >= 0 {
			s += l.wb[i] * t[j]
		}
		c[x], g[x] = s/l.take[i], l.wa[i]/l.take[i]
	}
	most := 0.0
	for x0 := range l.w {
		if state[x0] == 2 {
			continue
		}
		stack = stack[:0]
		x := x0
		for x >= 0 && state[x] == 0 {
			state[x] = 1
			stack = append(stack, x)
			x = int(up[x])
		}
		if x >= 0 && state[x] == 1 {
			// A ring: the warmth at x, carried round it, comes back as
			// a + b times itself.
			a, b := c[x], g[x]
			for y := int(up[x]); y != x; y = int(up[y]) {
				a += b * c[y]
				b *= g[y]
			}
			v := a / (1 - b)
			most = math.Max(most, math.Abs(v-ts[x]))
			ts[x], state[x] = v, 2
		}
		for k := len(stack) - 1; k >= 0; k-- {
			y := stack[k]
			if state[y] == 2 {
				continue
			}
			v := c[y]
			if u := up[y]; u >= 0 {
				v += g[y] * ts[u]
			}
			most = math.Max(most, math.Abs(v-ts[y]))
			ts[y], state[y] = v, 2
		}
	}
	return most
}

// coastal is the sea's warmth as the country round each cell feels it: the
// mean warmth of the sea within coastReach of it, felt in full where a third
// of that country is sea and less where less is. A coast in a warm current
// has all of it, and a place a few hundred kilometres inland none.
func (e *Env) coastal(warm []float64) []float64 {
	out := make([]float64, len(warm))
	for i, t := range warm {
		out[i] = t * e.Sea[i]
	}
	out = e.blur(out, coastReach)
	share := e.blur(e.Sea, coastReach)
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

// SeaWarmth is terra.Grid.SeaWarmth for a tile of the map.
func (w *Winds) SeaWarmth(i int) float64 {
	if w.Warm == nil {
		return 0
	}
	fx, fy := w.CellAt(i)
	return w.Sample(w.Warm, fx, fy)
}

// CoastWarmth is terra.Grid.CoastWarmth for a tile of the map.
func (w *Winds) CoastWarmth(i int) float64 {
	if w.Coast == nil {
		return 0
	}
	fx, fy := w.CellAt(i)
	return w.Sample(w.Coast, fx, fy)
}

// SeaCurrent is terra.Grid.SeaCurrent for a tile of the map.
func (w *Winds) SeaCurrent(i int) (east, north float64) {
	if w.Cu == nil {
		return 0, 0
	}
	fx, fy := w.CellAt(i)
	return w.Sample32(w.Cu, fx, fy), w.Sample32(w.Cv, fx, fy)
}

// Upwelling is terra.Grid.Upwelling for a tile of the map.
func (w *Winds) Upwelling(i int) float64 {
	if w.Rise == nil {
		return 0
	}
	fx, fy := w.CellAt(i)
	return w.Sample32(w.Rise, fx, fy)
}

// WaterTempAt is terra.Grid.SeaTemp for a tile of the map.
func (w *Winds) WaterTempAt(i int) float64 {
	if w.WaterTemp == nil {
		return 0
	}
	fx, fy := w.CellAt(i)
	return w.Sample32(w.WaterTemp, fx, fy)
}
