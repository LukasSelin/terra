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
//   - The thermocline. The gyres run in a layer of warm water over a cold,
//     still deep, and the layer is thickened where they drive the water
//     toward the equator and thinned where they drive it toward the pole,
//     and the trades hold it up in the east of the tropics and down in the
//     west. See thermocline.go.
//   - Upwelling. The water the wind drives goes to the right of it in the
//     north and the left in the south (Ekman, 1905), and where that takes it
//     off a shore, or where the drifts part in the open ocean, on the
//     equator and under the subpolar lows, cold water comes up from under to
//     take its place: as cold as the thermocline is shallow there.
//   - The warmth all of that carries, in the mixed layer and the water
//     under it: the water keeps the warmth of where it came from, and gives
//     it up to the air over some months. What it carries toward the poles is
//     handed to the energy balance, which says how much warmer or colder the
//     air over each latitude of the sea stands for it. See slab.go.
//
// It is done once a year's wind, from the stress of each season's wind, and
// only on a globe: a valley is a few dozen kilometres of country and has no
// ocean to have gyres in, and is untouched by any of this to the bit.

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
	// upwellCalm is how near the equator, in degrees, a coast has no
	// upwelling of its own: the turning of the planet that sends the water
	// the wind drives off a shore goes to nothing there.
	upwellCalm = 5.0
	// seaIce is the temperature, in degrees, sea water freezes at
	// (terra.SeaFreeze).
	seaIce = -1.8
	// polarReach is the latitude, in degrees, whose cells' breadth is the
	// least the current toward the poles is read across: see currents.
	polarReach = 60.0
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

// currents works out the water under the wind u, v of each phase of the
// year and gives each cell's warmth: how many degrees the sea there stands over the mean of
// its latitude, and nothing on land. The current, the upwelling and the
// water's temperature it works out on the way are kept on e: see Env.Cu.
// ocean is the gyres' equations for the ground (newFlow), or nil; s is the
// reading's working memory, or nil (see Scratch).
func (e *Env) currents(u, v [Phases][]float32, ocean *flow, s *Scratch) []float64 {
	defer phase.Start("airEnv.currents")()
	n := e.W * e.H
	wet := func(i int) bool { return e.Sea[i] > 0.5 }

	// The wind's stress on the sea, in newtons a square metre, over the
	// year; and the current it drifts the surface at.
	// They are worked out in the first phase's memory, whose wind is worked out by
	// now; what of it they do not take is let go of. See Scratch.
	all := s.phaseWork(0)
	all.drop(slotStressX, slotCurrentV+1)
	tx, ty := all.floats(slotStressX, n), all.floats(slotStressY, n)
	cu, cv := all.floats(slotCurrentU, n), all.floats(slotCurrentV, n)
	// The stress goes as the square of the wind, so the year's is the mean of
	// each season's, and not the stress of the year's mean wind: a coastal
	// wind that blows hard along a shore in one season and slack in the next
	// drives more water off it than a steady wind of their mean (the
	// upwelling seasons of Bakun, 1990). Read off the year's mean wind, the
	// summer's coastal jets off the subtropical west coasts counted as a
	// moderate wind all the year (#130).
	for i := range n {
		var sx, sy float64
		for k := range Phases {
			uu, vv := float64(u[k][i]), float64(v[k][i])
			s := math.Hypot(uu, vv)
			sx += s * uu
			sy += s * vv
		}
		tx[i] = airDensity * stressDrag * sx / Phases
		ty[i] = airDensity * stressDrag * sy / Phases
	}
	ekman := func(i, cy int) (east, north float64) {
		f := 2 * omega * math.Max(math.Sin(math.Abs(e.lat[cy])*math.Pi/180), math.Sin(upwellLow*math.Pi/180))
		f = math.Copysign(f, e.lat[cy])
		return ty[i] / (SeaDensity * f), -tx[i] / (SeaDensity * f)
	}

	// The gyres: the transport the wind drives round each ocean, ψ, the
	// thermocline that it and the wind tilt, and the current ψ makes spread
	// over the warm water above the thermocline, which is what moves, though
	// never over less than flowLeast. See flow.go and thermocline.go. The
	// current toward the pole or the equator is ψ's rise toward the east
	// across a reach of the row never narrower than a cell is at polarReach
	// degrees: toward a pole the cells narrow to a few hundred metres, and
	// a tenth of a sverdrup of the solve's error across one of them would
	// read as a current of metres a second. The models of the ocean and the
	// air on a grid of parallels filter their rows near the poles for the
	// same reason (Arakawa and Lamb, 1977).
	psi := e.gyres(tx, ty, ocean, s)
	thermo := e.thermocline(psi, tx, all)
	gu, gv := all.floats(slotGyreU, n), all.floats(slotGyreV, n)
	widest := 0.0
	for _, dx := range e.Dx {
		widest = math.Max(widest, dx)
	}
	for cy := 0; cy < e.H; cy++ {
		up, down, dy := cy-1, cy+1, 2*e.Dy
		if up < 0 {
			up, dy = cy, e.Dy
		}
		if down >= e.H {
			down, dy = cy, e.Dy
		}
		dx := e.Dx[cy]
		reach := max(1, int(math.Round(widest*math.Cos(polarReach*math.Pi/180)/dx)))
		reach = min(reach, e.W/4)
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			layer := math.Max(flowLeast, thermo[i])
			gu[i] = -(psi[up*e.W+cx] - psi[down*e.W+cx]) / dy / layer
			gv[i] = (psi[e.at(cx+reach, cy)] - psi[e.at(cx-reach, cy)]) / (2 * float64(reach) * dx) / layer
		}
	}
	e.Psi = grow(e.Psi, n)
	for i, p := range psi {
		e.Psi[i] = float32(p / Sverdrup)
	}
	for i := range n {
		if !wet(i) {
			gu[i], gv[i] = 0, 0
		}
		mx, my := ekman(i, i/e.W)
		cu[i] = gu[i] + mx/ekmanDepth
		cv[i] = gv[i] + my/ekmanDepth
		if !wet(i) {
			cu[i], cv[i] = 0, 0
		}
		cu[i] = math.Max(-currentMost, math.Min(currentMost, cu[i]))
		cv[i] = math.Max(-currentMost, math.Min(currentMost, cv[i]))
	}

	// Upwelling: how fast, in metres a second, the water the wind drives off a
	// shore, or the water its drift parts over in the open ocean, is replaced
	// from under; and sinking, how fast the water it drives onto a shore, or
	// that its drifts meet over, is pressed down under the mixed layer.
	rise, sink := e.pumping(u, v, all)
	land := all.floats(slotLand, n)
	for i := range land {
		land[i] = 1 - e.Sea[i]
	}
	for cy := 0; cy < e.H; cy++ {
		lat := e.lat[cy]
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
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
			w := off / math.Min(e.Dx[cy], e.Dy) * smoothstep(upwellCalm, upwellLow, math.Abs(lat))
			if off > 0 {
				rise[i] += w
			} else {
				sink[i] -= w
			}
		}
	}

	// The warmth of the water, in its two layers: see slab.go. What the sea
	// carries is handed to the energy balance, and the air over the sea is
	// as much warmer or colder as that makes it.
	temp := all.floats(slotWaterTemp, n)
	deep := all.floats(slotDeep, n)
	done := phase.Start("airEnv.slab")
	// The coupled round's solve starts from where the first left the sea,
	// under a wind a little changed (#28): it solves for the round's
	// correction only.
	var was *seaSlab
	var from []float64
	if ocean != nil {
		was, from = ocean.slab, ocean.sea
	}
	slab := e.newSlab(was, gu, gv, rise, sink, thermo, func(i int) (east, north float64) { return ekman(i, i/e.W) })
	slab.solve(temp, deep, s, from)
	// The air over the sea is as much warmer or colder as the energy
	// balance makes it for what the sea carries, and the sea is relaxed
	// toward the air over it: so the water is solved again under the air the
	// balance gives, and what it carries then handed to the balance again,
	// until the two agree. See seaSlab.air.
	var x []float64
	for round := 1; ; round++ {
		e.Carried = slab.carried(temp, deep)
		e.SeaHeat = e.seaHeat(e.Carried)
		dq := e.SeaHeat
		for k := range dq {
			dq[k] -= e.circ.seaIn[k]
		}
		e.seaShift = e.circ.respond(&dq)
		moved := 0.0
		for cy := range slab.air {
			moved = math.Max(moved, math.Abs(ebmRead(&e.seaShift, e.lat[cy])-slab.air[cy]))
		}
		if moved < airSettled || round > airRounds {
			break
		}
		slab.rewarm(func(cy int) float64 {
			return slab.air[cy] + airStep*(ebmRead(&e.seaShift, e.lat[cy])-slab.air[cy])
		})
		x = grow(x, 2*n)
		copy(x, temp)
		copy(x[n:], deep)
		slab.solve(temp, deep, s, x)
	}
	if ocean != nil {
		ocean.slab = slab
		ocean.sea = grow(ocean.sea, 2*n)
		copy(ocean.sea, temp)
		copy(ocean.sea[n:], deep)
	}
	done()
	// What the balance's last answer is over the air the water was last
	// solved under, under airSettled, is added to it as it stands.
	for cy := 0; cy < e.H; cy++ {
		d := ebmRead(&e.seaShift, e.lat[cy]) - slab.air[cy]
		for i := cy * e.W; i < (cy+1)*e.W; i++ {
			if wet(i) {
				temp[i] += d
				deep[i] += d
			}
		}
	}
	e.Cu, e.Cv, e.Rise, e.Thermocline = narrowInto(e.Cu, cu), narrowInto(e.Cv, cv), narrowInto(e.Rise, rise), narrowInto(e.Thermocline, thermo)

	// Water colder than seaIce is under ice, and the air over ice is not
	// warmed by the water under it: there the sea is worth no more than its
	// latitude, though its cold still counts. Without it the gyres, which
	// now reach the poles, carried water a few degrees warmer than the air
	// into the polar seas, and the polar lands beside them came out four
	// degrees milder than their latitude under a sea that was ice (see
	// terra.Grid.Freezing, which reads the same mean). The sea's own ice is
	// M7's (docs/ocean-model-plan.md). Elsewhere the warmth is the water's
	// temperature less its latitude's mean, as far from it as it stands: it
	// used to be held to ten degrees either way, the Gulf Stream's over the
	// water beside it at the Grand Banks, about the most the real world has,
	// and that is now a reading the sea is held to (realism_ocean_test.go).
	warm := make([]float64, n)
	for i := range warm {
		if wet(i) {
			warm[i] = temp[i] - e.Mean[i/e.W]
			if temp[i] < seaIce {
				warm[i] = math.Min(0, warm[i])
			}
		}
	}
	// The land along a shore is given the warmth of the sea beside it, so
	// that the sea read between the cells right up to the coast is the sea's
	// and not half the land's nothing: the coldest water there is lies against
	// the shore.
	// The water's own temperature is given to the shore the same way; the
	// land away from any sea keeps its latitude's mean.
	shore := all.floats(slotShore, n)
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
	e.WaterTemp = narrowInto(e.WaterTemp, temp)
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

// narrowInto is narrow written over out where out is as long as v, as it is
// where the currents are worked out again in a round of the coupled solve
// (coupled.go): what the round before kept is not read again.
func narrowInto(out []float32, v []float64) []float32 {
	if len(out) != len(v) {
		return narrow(v)
	}
	for i, x := range v {
		out[i] = float32(x)
	}
	return out
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

// WaterWarmth is how many degrees the water over tile i stands over the mean
// of its latitude: SeaWarmth, where the water is not under ice. It is
// nothing where there is no water worked out.
func (w *Winds) WaterWarmth(i int) float64 {
	if w.WaterTemp == nil {
		return 0
	}
	fx, fy := w.CellAt(i)
	y0 := math.Floor(fy)
	t := fy - y0
	a, b := min(max(int(y0), 0), w.H-1), min(max(int(y0)+1, 0), w.H-1)
	return w.Sample32(w.WaterTemp, fx, fy) - (w.Mean[a] + (w.Mean[b]-w.Mean[a])*t)
}

// CoastFrom hands visit what the water of each cell adds to Coast[c], the
// warmth the country round cell c feels off the sea: coastal's blur undone
// for the one cell, so that what each current makes of a coast can be read
// off it. What it hands is in degrees once taken times what it gives back,
// which is how much the country round c feels of the sea about it and is
// known only once every cell has been handed. Over every cell, the degrees
// come to Coast[c] to rounding. A valley, and a cell with no sea within
// coastReach, gives back nothing. Nothing in the weather calls it.
func (e *Env) CoastFrom(c int, visit func(cell int, degrees float64)) (felt float64) {
	if e.Coast == nil || e.Warm == nil || c < 0 || c >= len(e.Coast) || e.Coast[c] == 0 {
		return 0
	}
	cx, cy := c%e.W, c/e.W
	// The rows and the cells of each that box took the mean over, and how
	// many: see box.
	down := min(int(math.Round(coastReach/(e.Dy/1000))), e.H)
	y0, y1 := max(cy-down, 0), min(cy+down, e.H-1)
	rows := float64(y1 - y0 + 1)
	span := func(y int) (x0, x1 int, k float64) {
		r := int(math.Round(coastReach / (e.Dx[y] / 1000)))
		switch {
		case e.Wrap && 2*r+1 >= e.W:
			return 0, e.W - 1, float64(e.W)
		case e.Wrap:
			return cx - r, cx + r, float64(2*r + 1)
		}
		x0, x1 = max(cx-r, 0), min(cx+r, e.W-1)
		return x0, x1, float64(x1 - x0 + 1)
	}
	share := 0.0
	for y := y0; y <= y1; y++ {
		x0, x1, k := span(y)
		w := 1 / (k * rows)
		for x := x0; x <= x1; x++ {
			j := e.at(x, y)
			share += e.Sea[j] * w
			if d := e.Warm[j] * e.Sea[j]; d != 0 {
				visit(j, d*w)
			}
		}
	}
	if share <= 1e-6 {
		return 0
	}
	return smoothstep(0, coastShare, share) / share
}

// Inversion is how much of the rain over cell c the inversion takes: the
// share the air held down by the cold water off the coast does not rain out,
// as the budget read it (see RainCells). Nothing over warm water, and nothing
// on a valley.
func (e *Env) Inversion(c int) float64 {
	if e.Coast == nil || c < 0 || c >= len(e.Coast) {
		return 0
	}
	return 1 - inversion(e.Coast[c])
}

// Damp is how many times the water the sea over cell c gives the air in
// phase k it gives for its current's warmth: the sea's saturation at the
// temperature the budget read it at (see RainCells), over its saturation
// without what the current brings. One where the water stands at its
// latitude's mean, and on a valley.
func (e *Env) Damp(c, k int) float64 {
	if e.Warm == nil || c < 0 || c >= len(e.Warm) || k < 0 || k >= Phases {
		return 1
	}
	if k == Phases-1 {
		k = 1 // the autumn is the spring
	}
	cy := c / e.W
	sst := e.Mean[cy] + e.seasonTemp(cy, phaseSin[k], 0) + seaOverAir
	return saturation(sst+e.Warm[c]) / saturation(sst)
}

// Corners is the cells a reading of tile i is taken between, and how much
// of each it takes: Sample's, so that what a sum over the cells comes to at
// a tile is what Sample would read of it.
func (e *Env) Corners(i int) (cells [4]int, weights [4]float64) {
	fx, fy := e.CellAt(i)
	x0, y0 := math.Floor(fx), math.Floor(fy)
	tx, ty := fx-x0, fy-y0
	x, y := int(x0), int(y0)
	return [4]int{e.at(x, y), e.at(x+1, y), e.at(x, y+1), e.at(x+1, y+1)},
		[4]float64{(1 - tx) * (1 - ty), tx * (1 - ty), (1 - tx) * ty, tx * ty}
}

// ThermoclineAt is terra.Grid.Thermocline for a tile of the map.
func (w *Winds) ThermoclineAt(i int) float64 {
	if w.Thermocline == nil {
		return 0
	}
	fx, fy := w.CellAt(i)
	return w.Sample32(w.Thermocline, fx, fy)
}
