package atmos

import "math"

// The thermocline.
//
// The sea is warm only at the top. Under a few tens to a few hundreds of
// metres of water the sun and the air have warmed, the temperature falls in a
// few tens of metres more to the cold of the deep, and that step, the
// thermocline, is where the water of the gyres runs. It is taken here as
// Zebiak and Cane (1987) take it: one layer of warm water of thickness h over
// a deep that is cold and still, the warm water lighter by g', at its steady
// state. Where the water a cell's upwelling brings up comes from is then a
// depth, and how cold it is depends on how far under that depth the
// thermocline lies: off Peru, where the trades have drawn the warm water away
// west and the thermocline is a few tens of metres down, the water that comes
// up is the thermocline's own and some ten degrees colder than the surface;
// in the western Pacific, where they have piled it up two hundred metres
// deep, what comes up is the warm layer and hardly colder at all.
//
// The deep is still, so the whole of the gyres' transport ψ (flow.go) is
// carried in the warm layer, and the pull of the wind on it and the planet's
// turning are balanced by the tilt of the thermocline under it:
//
//	g' h ∇h = f k × (h u) + τ/ρ
//
// Along a parallel that is ∂Φ/∂x = f ∂ψ/∂x + τx/ρ, with Φ = g'h²/2. Away from
// the equator the first part is the larger: where the subtropical gyres drive
// the water toward the equator the layer thickens toward the west, and where
// the subpolar gyres drive it toward the pole it thins, until in the west of
// a subpolar ocean the thermocline comes up to the surface (Parsons, 1969;
// Luyten, Pedlosky and Stommel, 1983). On the equator f is nought and the
// second part is all of it: the trades hold the thermocline up in the east
// and down in the west against its own weight (g'H ∂h/∂x = τx/ρ, linearized),
// the equatorial balance of Zebiak and Cane. The one form goes smoothly from
// the one balance to the other, and is integrated west along each row of sea
// from its eastern shore, where the layer is thermoEast deep, as the
// boundary waves of the real ocean hold it (Anderson and Gill, 1975). A
// parallel of sea all the way round has no eastern shore: the wind's pull
// along it is borne by the ridges of the sea floor rather than by a tilt,
// and the layer is thermoMean deep on its mean.
//
// Where the water is pumped up from under is worked out from the wind too:
// the water the wind drives in the top fifty metres goes a quarter turn to
// the right of it in the north and the left in the south (Ekman, 1905), and
// where those drifts part, water comes up from under to fill the gap. They
// part on the equator, where the trades drive the water north on its
// northern side and south on its southern; and under the subpolar lows. On
// the equator itself the turning that sends the water sideways is nought,
// and the drift is taken as Zebiak and Cane take it, a layer damped in two
// days, so that close to the equator it runs downwind rather than sideways.

const (
	// reducedGravity is g', how much lighter the warm water is than the deep
	// as a share of the planet's pull, in metres a second squared: Zebiak and
	// Cane's (1987) Kelvin wave speed of 2.9 m/s over a layer 150 m deep.
	reducedGravity = 2.9 * 2.9 / 150
	// thermoEast is how deep, in metres, the thermocline lies against an
	// ocean's eastern shore, where the waves along the shore hold it level
	// (Anderson and Gill, 1975): some fifty metres off Peru and Namibia
	// (Fiedler and Talley, 2006).
	thermoEast = 50.0
	// thermoMean is how deep, in metres, the thermocline lies on a parallel
	// of sea all the way round, which has no eastern shore to hold it to:
	// the mean depth of Zebiak and Cane's layer.
	thermoMean = 150.0
	// thermoLeast is the least depth, in metres, of the warm layer: where
	// the thermocline would come up through the surface, in the west of the
	// subpolar gyres, the water is the deep's from the top. thermoMost is
	// the most: under the subtropical gyres the warm water is some five or
	// six hundred metres deep (Levitus, 1982).
	thermoLeast = 10.0
	thermoMost  = 700.0
	// flowLeast is the least depth, in metres, the gyres' transport is
	// spread over to give a surface current: where the thermocline comes up
	// toward the surface, as it does in the subpolar gyres and against a
	// western shore, the water under it is not still, and the current
	// reaches down through it.
	flowLeast = 200.0
	// ekmanDamp is how fast, per second, the drift of the top of the sea
	// under the wind is damped: in two days (Zebiak and Cane, 1987). Within a
	// couple of degrees of the equator, where the turning of the planet is
	// less than this, the drift runs downwind rather than sideways.
	ekmanDamp = 1 / (2 * 86400.0)
	// equatorRadius is the equatorial radius of deformation, √(c/β), in
	// metres, with c the speed of the layer's waves, √(g'·thermoMean), and β
	// the planet's turning's change with latitude on the equator: some three
	// hundred and fifty kilometres (Gill, 1982).
	equatorRadius = 355e3
	// deepContrast is how much colder, in degrees, the water under the
	// thermocline is than the surface at the equator: some fifteen degrees
	// across the equatorial Pacific's (Fiedler and Talley, 2006). It falls
	// away toward the poles as the square of the cosine of the latitude,
	// where the ocean is mixed from top to bottom in winter. The water that
	// comes up is drawn from under the mixed layer, some fifty metres down:
	// where the thermocline lies thermoMid metres down, what comes up is
	// half the way from the surface's warmth to the deep's cold, and the
	// change from one to the other is over some thermoSpread metres either
	// way. Off Peru, the thermocline fifty metres down, it comes up some
	// twelve degrees colder than the surface; under the warm pool, a hundred
	// and fifty metres down, one.
	deepContrast = 16.0
	thermoMid    = 80.0
	thermoSpread = 25.0
)

// thermocline is the depth of the warm layer on every cell, in metres, from
// the gyres' transport psi in cubic metres a second and the wind's stress
// along the parallels tx in newtons a square metre. It is nought on land.
// It and what it works out on the way are in all's slots (see Scratch).
func (e *Env) thermocline(psi, tx []float64, all *work) []float64 {
	n := e.W * e.H
	h := all.floats(slotThermo, n)
	wet := func(i int) bool { return e.Sea[i] > 0.5 }
	east := reducedGravity * thermoEast * thermoEast / 2
	ring := reducedGravity * thermoMean * thermoMean / 2
	tx = e.guided(tx, all.floats(slotGuided, len(tx)))
	// Each row works in its own stretch of these, so that the rows can be
	// spread over goroutines.
	phi, runs := all.floats(slotLevel, n), make([]int, n)
	e.rows(func(cy int) {
		row := cy * e.W
		dx, f := e.Dx[cy], e.f[cy]
		shore := -1
		for cx := 0; cx < e.W; cx++ {
			if !wet(row + cx) {
				shore = cx
				break
			}
		}
		p := phi[row : row+e.W]
		if shore < 0 {
			// All the way round: Φ is f ψ and the wind's pull less its mean
			// along the parallel, about thermoMean's on the mean.
			var mean float64
			for cx := range e.W {
				mean += tx[row+cx]
			}
			mean /= float64(e.W)
			var pull, sum float64
			for cx := range e.W {
				i := row + cx
				if cx > 0 {
					pull += ((tx[i-1]+tx[i])/2 - mean) * dx / SeaDensity
				}
				p[cx] = f*psi[i] + pull
				sum += p[cx]
			}
			sum /= float64(e.W)
			for cx := range e.W {
				p[cx] += ring - sum
			}
		} else {
			// West along the row from each eastern shore, round the seam.
			// Near the equator the layer's warm water is held to its mean
			// along each stretch of sea, as it is along a parallel all the way
			// round: there the waves that run along the equator and back
			// along the eastern shore keep how much warm water an ocean has,
			// and the trades only move it from the one side to the other
			// (Zebiak and Cane, 1987). Away from the equator the shore holds
			// the layer at thermoEast.
			y := e.lat[cy] * math.Pi / 180 * planetRadius
			keep := math.Exp(-y * y / (2 * equatorRadius * equatorRadius))
			level, pull := psi[row+shore], 0.0
			run := runs[row : row : row+e.W]
			for k := 1; k <= e.W; k++ {
				cx := ((shore-k)%e.W + e.W) % e.W
				i := row + cx
				if !wet(i) {
					level = psi[i]
					if len(run) > 0 {
						var mean float64
						for _, x := range run {
							mean += p[x]
						}
						mean /= float64(len(run))
						for _, x := range run {
							p[x] += keep * (ring - mean)
						}
						run = run[:0]
					}
					continue
				}
				run = append(run, cx)
				j := row + (cx+1)%e.W
				if !wet(j) {
					pull = tx[i] * dx / 2 / SeaDensity
				} else {
					pull += (tx[i] + tx[j]) / 2 * dx / SeaDensity
				}
				p[cx] = east + f*(psi[i]-level) - pull
			}
		}
		for cx := range e.W {
			if i := row + cx; wet(i) {
				h[i] = math.Max(thermoLeast, math.Min(thermoMost, math.Sqrt(math.Max(0, 2*p[cx]/reducedGravity))))
			}
		}
	})
	return h
}

// pumping is how fast, in metres a second over the year, the water under
// each sea cell is drawn up by the parting of the wind's drift over it, from
// the wind of each phase of the year u, v: the divergence of the drift, the
// drift against a shore left to the coast's own upwelling. Where the drifts
// meet the surface water is pressed down, which leaves it as warm as it was,
// so a phase's pumping counts only where it draws water up: the equator,
// where the year's mean wind is the doldrums' calm and its drifts meet, has
// the trades of one hemisphere or the other blowing across it for half the
// year, and the water comes up then.
func (e *Env) pumping(u, v [Phases][]float32, all *work) []float64 {
	n := e.W * e.H
	mx, my := all.floats(slotDriftX, n), all.floats(slotDriftY, n)
	w := all.floats(slotPumped, n)
	wet := func(i int) bool { return e.Sea[i] > 0.5 }
	for k := range Phases {
		for i := range n {
			uu, vv := float64(u[k][i]), float64(v[k][i])
			s := math.Hypot(uu, vv)
			tx, ty := airDensity*stressDrag*s*uu, airDensity*stressDrag*s*vv
			f := e.f[i/e.W]
			d := SeaDensity * (ekmanDamp*ekmanDamp + f*f)
			mx[i] = (ekmanDamp*tx + f*ty) / d
			my[i] = (ekmanDamp*ty - f*tx) / d
		}
		e.rows(func(cy int) {
			north, south := max(cy-1, 0), min(cy+1, e.H-1)
			for cx := 0; cx < e.W; cx++ {
				i := cy*e.W + cx
				if !wet(i) {
					continue
				}
				// A neighbour on land is read as the cell itself, so that the
				// drift onto or off a shore is not counted here: it is the
				// coast's.
				pick := func(j int) int {
					if wet(j) {
						return j
					}
					return i
				}
				ea, we := pick(e.at(cx+1, cy)), pick(e.at(cx-1, cy))
				no, so := pick(north*e.W+cx), pick(south*e.W+cx)
				div := (mx[ea]-mx[we])/(2*e.Dx[cy]) +
					(my[no]*e.Dx[no/e.W]-my[so]*e.Dx[so/e.W])/(2*e.Dy*e.Dx[cy])
				w[i] += math.Max(0, div) / Phases
			}
		})
	}
	return w
}

// upwelled is the temperature, in degrees, of the water that comes up from
// under a cell on row cy where the thermocline is h metres down.
func (e *Env) upwelled(cy int, h float64) float64 {
	c := math.Cos(e.lat[cy] * math.Pi / 180)
	return e.Mean[cy] - deepContrast*c*c/(1+math.Exp((h-thermoMid)/thermoSpread))
}

// guided is the wind's stress along the parallels tx as the thermocline
// under each sea cell feels it: averaged across the parallels over the
// distance the thermocline's own waves reach from where they are made, the
// radius of deformation, c/|f| with c the speed of the layer's waves. On the
// equator, where f is nought, that is the equatorial radius, √(c/β), some
// three hundred and fifty kilometres (Gill, 1982): the thermocline along the
// equator is tilted by the trades either side of it as well as by the wind on
// it, which under the rising air of the doldrums is little. Land takes no
// part.
func (e *Env) guided(tx, out []float64) []float64 {
	c := math.Sqrt(reducedGravity * thermoMean)
	e.rows(func(cy int) {
		beta := 2 * omega * math.Cos(e.lat[cy]*math.Pi/180) / planetRadius
		r := math.Sqrt(c / beta)
		if f := math.Abs(e.f[cy]); f > 0 {
			r = math.Min(r, c/f)
		}
		reach := int(3 * r / e.Dy)
		lo, hi := max(0, cy-reach), min(e.H-1, cy+reach)
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			if e.Sea[i] <= 0.5 {
				out[i] = tx[i]
				continue
			}
			var s, w float64
			for k := lo; k <= hi; k++ {
				j := k*e.W + cx
				if e.Sea[j] <= 0.5 {
					continue
				}
				d := float64(k-cy) * e.Dy / r
				g := math.Exp(-d * d / 2)
				s, w = s+g*tx[j], w+g
			}
			out[i] = s / w
		}
	})
	return out
}
