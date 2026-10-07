package atmos

import "math"

// The sea's warmth in two layers, and the heat it carries.
//
// The water's warmth was one layer's, relaxed back toward the latitude's
// mean with the currents carrying an anomaly about it: the ocean moved no
// heat of its own, and a world whose Gulf Stream ran twice as strong had the
// same equator and the same poles. Here it is the dynamical slab of Codron
// (2012), as the Generic-PCM carries it: a mixed layer the air warms and
// cools, over a layer of water under it that the air never touches.
//
//   - The wind drives the top of the sea a quarter turn to its right in the
//     north (Ekman, 1905), in the mixed layer; the water it takes away comes
//     back under it, in the layer below, the other way. Under the trades the
//     surface water goes poleward warm and comes back equatorward cool, and
//     that pair of currents, the subtropical cells, is most of the heat the
//     tropical oceans carry (Klinger and Marotzke, 2000).
//   - Where the drift parts, water comes up from the layer below, or through
//     it from the thermocline where that is shallow (thermocline.go); where
//     the drifts meet, the mixed layer's water is pressed down into it. The
//     water under the subtropical gyres is the surface water of their
//     poleward side, pressed down and carried toward the equator under the
//     warm surface: the ventilated thermocline (Luyten, Pedlosky and Stommel,
//     1983). Toward the equator the thermocline comes up into the layer, and
//     the drift's return runs in it, in water colder than any at the surface
//     (McCreary and Lu, 1994): that is what the subtropical cells carry back,
//     and why the water that comes up on the equator is cold.
//   - Toward the poles the winter's storms stir the layer under the mixed
//     layer into it.
//   - The gyres (flow.go) move both layers, their transport spread over the
//     warm water above the thermocline.
//   - The eddies stir both layers along them, as a diffusion: some 1000-2000
//     m²/s over most of the ocean at the surface (Abernathey and Marshall,
//     2013).
//
// The mixed layer is as deep as it was, some fifty metres under the trades
// and three hundred where the winter storms stir it, and the layer under it
// Codron's hundred and fifty. Each is solved at its steady state under the
// year's mean wind, cell by cell, the two layers of a row together.
//
// What the two layers carry toward the poles is read off them (carried),
// handed to the energy balance by latitude (seaHeat), and the balance says
// how much warmer or colder the air over each band of the sea stands for the
// sea's carrying against what it was worked out with (ebm.go: respond).

const (
	// deepLayer is how thick, in metres, the water under the mixed layer is,
	// in which the wind's drift comes back: Codron's (2012) 150 m.
	deepLayer = 150.0
	// eddyStir is the eddies' diffusion of the sea's warmth along its
	// layers, m²/s: Abernathey and Marshall's (2013) 1000-2000 over most of
	// the ocean's surface.
	eddyStir = 1500.0
	// layerMix is the stirring between the two layers across the base of the
	// mixed layer, m²/s: the 10⁻⁵ Ledwell, Watson and Law (1993) measured
	// across the main thermocline. Munk's (1966) 10⁻⁴ is the abyss's on the
	// whole; at that, a sixth of the tropical mixed layer's pull was toward
	// the cool water under it, and the western tropics stood half a degree
	// colder than the air over them asks.
	layerMix = 1e-5
	// convectDays is how many days the winter's storms take to stir the
	// layer under the mixed layer into it where the mixed layer is deepest,
	// poleward of mixedHigh: a winter. Toward the tropics it goes as the
	// mixed layer's depth does, to nothing at mixedLow. Without it the water
	// under the mixed layer of the middle latitudes, fed by the warm water
	// the gyres and the drift's return bring poleward under it, stood warmer
	// than the surface over it.
	convectDays = 90.0
	// belowMost is the most of the layer under the mixed layer read as the
	// cold under the thermocline.
	belowMost = 0.9
	// slabSettled is how small a share of the equations' right-hand side
	// the residual is left at, slabRestart how many directions GMRES takes
	// before it starts again, and slabMost the most it takes. See solve.
	slabSettled = 1e-3
	slabRestart = 12
	slabMost    = 60
)

// seaSlab is each sea cell's two equations, for the warmth of the mixed
// layer t and of the water under it d:
//
//	take₀·t_i = base₀ + west₀·t_w + east₀·t_e + north₀·t_n + south₀·t_s + corner₀·t_c + cross₀·d_i
//	take₁·d_i = base₁ + west₁·d_w + ... + cross₁·t_i
//
// with w, e, n and s the cells either side along the row and the column,
// and c a cell across a diagonal, which water coming round a corner of the
// shore comes from. Land takes nothing.
type seaSlab struct {
	e                                    *Env
	take, base, west, east, north, south [2][]float64
	corner                               [2][]float64
	from                                 [2][]int32 // the corner's cell, or -1
	cross                                [2][]float64
	carry                                [2][]float64 // each layer's transport toward the north, m²/s
	depth                                []float64    // the mixed layer's depth on each row, metres
	f                                    *slabFactor
	// below is how much of the layer under the mixed layer on each cell is
	// the cold under the thermocline, and cold how cold that is on each row.
	below, cold []float64
}

// newSlab writes each sea cell's two equations down, from the gyres' current
// gu, gv, the water drawn up rise and pressed down sink, metres a second, the
// thermocline thermo, metres, and the wind's drift ekman, m²/s. They are
// written over l's where l is not nil, as they are where the currents are
// worked out again in a round of the coupled solve (coupled.go): the
// equations of the round before are not read again. Each row's are its own,
// and the rows are written side by side.
func (e *Env) newSlab(l *seaSlab, gu, gv, rise, sink, thermo []float64, ekman func(i int) (east, north float64)) *seaSlab {
	n := e.W * e.H
	if l == nil || l.e != e {
		l = &seaSlab{e: e}
	}
	l.f = nil
	l.depth, l.cold, l.below = grow(l.depth, e.H), grow(l.cold, e.H), grow(l.below, n)
	for q := range 2 {
		l.take[q], l.base[q] = grow(l.take[q], n), grow(l.base[q], n)
		l.west[q], l.east[q] = grow(l.west[q], n), grow(l.east[q], n)
		l.north[q], l.south[q] = grow(l.north[q], n), grow(l.south[q], n)
		l.corner[q], l.from[q] = grow(l.corner[q], n), grow(l.from[q], n)
		l.cross[q], l.carry[q] = grow(l.cross[q], n), grow(l.carry[q], n)
	}
	wet := func(i int) bool { return e.Sea[i] > 0.5 }
	clampSpeed := func(v float64) float64 { return math.Max(-currentMost, math.Min(currentMost, v)) }
	dy := e.Dy
	e.rows(func(cy int) {
		h1 := mixedTropic + (mixedPolar-mixedTropic)*smoothstep(mixedLow, mixedHigh, math.Abs(e.lat[cy]))
		l.depth[cy] = h1
		relax := seaExchange / (seaHeat * h1)
		c := math.Cos(e.lat[cy] * math.Pi / 180)
		cold := e.Mean[cy] - deepContrast*c*c
		l.cold[cy] = cold
		mix := layerMix/((h1+deepLayer)/2) + deepLayer/(convectDays*86400)*smoothstep(mixedLow, mixedHigh, math.Abs(e.lat[cy]))
		dx := e.Dx[cy]
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			l.from[0][i], l.from[1][i] = -1, -1
			if !wet(i) {
				continue
			}
			mx, my := ekman(i)
			// How much of the water that comes up is the layer under the
			// mixed layer's, and how much the thermocline's: all of it the
			// thermocline's where that lies at the foot of the mixed layer, as
			// off Peru, and none where it lies hundreds of metres down.
			th := 1 / (1 + math.Exp((thermo[i]-thermoMid)/thermoSpread))
			up := rise[i] / h1
			down := sink[i] / deepLayer
			l.take[0][i] = relax + up + mix/h1
			l.base[0][i] = relax*e.Mean[cy] + up*th*cold
			l.cross[0][i] = up*(1-th) + mix/h1

			l.take[1][i] = down + mix/deepLayer
			l.cross[1][i] = down + mix/deepLayer
			// Each layer's current: the gyres' through both, the wind's drift
			// in the mixed layer and its return under it. Of the gyres'
			// transport, spread over the warm water above the thermocline,
			// the mixed layer carries its own depth's share and the water
			// under it the rest.
			layer := math.Max(flowLeast, thermo[i])
			u := [2]float64{clampSpeed(gu[i] + mx/h1), clampSpeed(gu[i] - mx/deepLayer)}
			v := [2]float64{clampSpeed(gv[i] + my/h1), clampSpeed(gv[i] - my/deepLayer)}
			l.carry[0][i] = gv[i]*math.Min(h1, layer) + my
			l.carry[1][i] = gv[i]*math.Max(0, layer-h1) - my
			for q := range 2 {
				l.flows(q, cx, cy, u[q], v[q], dx, dy)
			}
			// Where the thermocline lies within the layer under the mixed
			// layer, that much of the water there is the cold under it: b,
			// the share of the layer under the thermocline, a step
			// thermoSpread metres either way. The warmth solved for is the
			// warm water's, and what the layer carries is b of the cold's and
			// the rest of the warm's (see carried): the cold is the deep's,
			// kept cold by the overturning (M5), and is not warmed or cooled
			// here. Taken into the layer's own warmth instead, it was a sink
			// that the winter's stirring and the gyres spread poleward, and
			// the whole sea of the middle latitudes stood four or five
			// degrees under its air. Toward the poles, where the winter stirs
			// the layer into the mixed layer, the water under the mixed layer
			// is the winter's own and b goes to nothing as the stirring comes
			// in.
			b := thermoSpread / deepLayer * (softplus((h1+deepLayer-thermo[i])/thermoSpread) - softplus((h1-thermo[i])/thermoSpread))
			l.below[i] = math.Min(b, belowMost) * (1 - smoothstep(mixedLow, mixedHigh, math.Abs(e.lat[cy])))
		}
	})
	return l
}

// softplus is ln(1 + eˣ), taken so that it does not overflow.
func softplus(x float64) float64 {
	if x > 30 {
		return x
	}
	return math.Log1p(math.Exp(x))
}

// flows adds to layer q's equation at cx, cy what the current u, v carries
// into it from upstream, and what the eddies stir into it from either side.
func (l *seaSlab) flows(q, cx, cy int, u, v, dx, dy float64) {
	e := l.e
	i := cy*e.W + cx
	wet := func(j int) bool { return e.Sea[j] > 0.5 }
	// The eddies, to each side that is sea.
	kx, ky := eddyStir/(dx*dx), eddyStir/(dy*dy)
	if j := e.at(cx-1, cy); wet(j) && j != i {
		l.west[q][i] += kx
		l.take[q][i] += kx
	}
	if j := e.at(cx+1, cy); wet(j) && j != i {
		l.east[q][i] += kx
		l.take[q][i] += kx
	}
	if cy > 0 && wet(i-e.W) {
		l.north[q][i] += ky
		l.take[q][i] += ky
	}
	if cy < e.H-1 && wet(i+e.W) {
		l.south[q][i] += ky
		l.take[q][i] += ky
	}
	// The current, from the cell upstream along the row, and down the
	// column. Toward the north is up the map, so water going north comes
	// from the row below. The current runs along the coast, so where the
	// cell behind it in the column is land, the water came round the corner:
	// from the cell behind it across the diagonal, on the side the current
	// comes from along the row.
	if a := math.Abs(u) / dx; a > 0 {
		dir := -int(math.Copysign(1, u))
		if j := e.at(cx+dir, cy); wet(j) && j != i {
			l.take[q][i] += a
			if dir < 0 {
				l.west[q][i] += a
			} else {
				l.east[q][i] += a
			}
		}
	}
	if b := math.Abs(v) / dy; b > 0 {
		uy := cy + int(math.Copysign(1, v))
		if uy < 0 || uy >= e.H {
			return
		}
		j := e.at(cx, uy)
		if !wet(j) && u != 0 {
			j = e.at(cx-int(math.Copysign(1, u)), uy)
			if wet(j) {
				l.take[q][i] += b
				l.corner[q][i], l.from[q][i] = b, int32(j)
			}
			return
		}
		if wet(j) {
			l.take[q][i] += b
			if uy < cy {
				l.north[q][i] += b
			} else {
				l.south[q][i] += b
			}
		}
	}
}

// solve settles the two layers' warmth, t the mixed layer's and d the water
// under it, by GMRES (gmres.go) on the two layers' equations together,
// preconditioned by a sweep of the rows north to south and back in which each
// row's two layers are solved together and exactly along the row for the
// rows either side, as a chain of two-by-two blocks, or a ring of them round
// a parallel of sea all the way round (see sweep). Swept alone, as the one
// layer's warmth was, the two layers took some fifty rounds on two oceans
// and more on a globe: in the middle latitudes the mixed layer is deep and
// the air's pull on it weak against the gyres that carry it round, and the
// water under it is touched by the air only through it, so the warmth of a
// whole gyre settles by a few hundredths a round. GMRES takes that out in
// some twenty directions on a globe. Sweeping the columns as well as the
// rows, or each row's mean first, took as many. Its vectors are lent from s,
// as the gyres' are.
func (l *seaSlab) solve(t, d []float64, s *Scratch) {
	e := l.e
	n := e.W * e.H
	b := make([]float64, 2*n)
	copy(b, l.base[0])
	copy(b[n:], l.base[1])
	l.factor()
	rows := make([]*slabRow, slabBands(e.H))
	for k := range rows {
		rows[k] = newSlabRow(e.W)
	}
	precondition := func(in, out []float64) {
		clear(out)
		l.sweep(out, in, rows)
	}
	x, _, _ := gmresOn(b, l.applyRows, precondition, slabSettled, slabRestart, slabMost, s.lend(gmresRoom(slabRestart), 2*n), l.spans())
	copy(t, x[:n])
	copy(d, x[n:])
}

// spans are the runs of sea in the two layers' unknowns, the mixed layer's
// and then the water's under it, as gmresOn takes them: on land each
// unknown is its own right-hand side, nought, and nothing reads it.
func (l *seaSlab) spans() [][2]int {
	n := len(l.take[0])
	var out [][2]int
	for q := range 2 {
		lo := -1
		for i, c := range l.take[q] {
			switch {
			case c != 0 && lo < 0:
				lo = i
			case c == 0 && lo >= 0:
				out = append(out, [2]int{q*n + lo, q*n + i})
				lo = -1
			}
		}
		if lo >= 0 {
			out = append(out, [2]int{q*n + lo, q*n + n})
		}
	}
	return out
}

// apply is the two layers' equations on x, the mixed layer's warmth and then
// the water's under it, into out; on land each is its own warmth.
func (l *seaSlab) applyBand(x, out []float64, lo, hi int) {
	e := l.e
	w, n := e.W, e.W*e.H
	for q := range 2 {
		f, o, other := x[q*n:(q+1)*n], out[q*n:(q+1)*n], x[(1-q)*n:(2-q)*n]
		take, west, east, north, south := l.take[q], l.west[q], l.east[q], l.north[q], l.south[q]
		corner, from, cross := l.corner[q], l.from[q], l.cross[q]
		for cy := lo; cy < hi; cy++ {
			row := cy * w
			for cx := range w {
				i := row + cx
				if take[i] == 0 {
					o[i] = f[i]
					continue
				}
				v := take[i]*f[i] - cross[i]*other[i]
				if c := west[i]; c != 0 {
					j := i - 1
					if cx == 0 {
						j = row + w - 1
					}
					v -= c * f[j]
				}
				if c := east[i]; c != 0 {
					j := i + 1
					if cx == w-1 {
						j = row
					}
					v -= c * f[j]
				}
				if c := north[i]; c != 0 {
					v -= c * f[i-w]
				}
				if c := south[i]; c != 0 {
					v -= c * f[i+w]
				}
				if j := from[i]; j >= 0 {
					v -= corner[i] * f[j]
				}
				o[i] = v
			}
		}
	}
}

// slabRow is a row's working: each cell's right-hand side, the
// elimination's, and the solution's.
type slabRow struct {
	rhs, g, p [][2]float64
}

func newSlabRow(w int) *slabRow {
	return &slabRow{rhs: make([][2]float64, w), g: make([][2]float64, w), p: make([][2]float64, w)}
}

// slabFactor is the rows' blocks eliminated once, since the equations do
// not change while they are solved: each sea cell's eliminated block's
// inverse along its chain, from the land west of it, or along the ring from
// the row's second cell; and for a ring, what each cell's warmth takes of
// the first cell's (ringQ), and the first cell's own eliminated block's
// inverse (ringFirst).
type slabFactor struct {
	inv       [][4]float64
	ringQ     [][][2][2]float64 // nil on a row that is not a ring
	ringFirst [][4]float64
	// chains are each row's runs of sea, as the cells they start and end
	// at, east from a cell of land; a ring is one run from 0 to W-1.
	chains [][][2]int32
}

// block is cell i's own two-by-two block.
func (l *seaSlab) block(i int) [4]float64 {
	return [4]float64{l.take[0][i], -l.cross[0][i], -l.cross[1][i], l.take[1][i]}
}

func inv2(m [4]float64) [4]float64 {
	det := m[0]*m[3] - m[1]*m[2]
	return [4]float64{m[3] / det, -m[1] / det, -m[2] / det, m[0] / det}
}

func mul2(m [4]float64, v [2]float64) [2]float64 {
	return [2]float64{m[0]*v[0] + m[1]*v[1], m[2]*v[0] + m[3]*v[1]}
}

// factor eliminates every row's blocks: block Thomas along each chain, the
// west and east weights being one number a layer. A row's are its own, and
// the rows are eliminated side by side.
func (l *seaSlab) factor() {
	e := l.e
	w := e.W
	n := w * e.H
	f := &slabFactor{inv: make([][4]float64, n), ringQ: make([][][2][2]float64, e.H), ringFirst: make([][4]float64, e.H), chains: make([][][2]int32, e.H)}
	l.f = f
	e.rows(func(cy int) {
		row := cy * w
		sea := func(x int) bool { return l.take[0][row+x] != 0 }
		start := -1
		for x := range w {
			if !sea(x) {
				start = x
				break
			}
		}
		if start < 0 {
			f.chains[cy] = [][2]int32{{0, int32(w - 1)}}
			l.factorRing(cy)
			return
		}
		first := -1
		for k := 1; k <= w; k++ {
			x := (start + k) % w
			if sea(x) {
				if first < 0 {
					first = x
				}
				continue
			}
			if first >= 0 {
				last := (x - 1 + w) % w
				f.chains[cy] = append(f.chains[cy], [2]int32{int32(first), int32(last)})
				l.eliminate(row, first, last)
				first = -1
			}
		}
	})
}

// eliminate eliminates the chain of row from cell first east to cell last,
// round the seam if it has to.
func (l *seaSlab) eliminate(row, first, last int) {
	w := l.e.W
	inv := l.f.inv
	prev := -1
	for x := first; ; x = (x + 1) % w {
		i := row + x
		m := l.block(i)
		if prev >= 0 {
			pi := row + prev
			a := inv[pi]
			w0, w1 := l.west[0][i], l.west[1][i]
			e0, e1 := l.east[0][pi], l.east[1][pi]
			// m -= W · inv · E, with W and E diagonal.
			m[0] -= w0 * a[0] * e0
			m[1] -= w0 * a[1] * e1
			m[2] -= w1 * a[2] * e0
			m[3] -= w1 * a[3] * e1
		}
		inv[i] = inv2(m)
		if x == last {
			return
		}
		prev = x
	}
}

// factorRing eliminates a row of sea all the way round. The warmth of its
// first cell, x0, is taken as unknown: every other cell's, along the open
// chain from the second to the last, is p + Q x0, with p from the row's
// right-hand side and Q from x0's pull on the second cell and the last; and
// the first cell's own equation then gives x0.
func (l *seaSlab) factorRing(cy int) {
	w := l.e.W
	row := cy * w
	l.eliminate(row, 1, w-1)
	inv := l.f.inv
	// Forward and back for Q, column c for x0's layer c.
	q := make([][2][2]float64, w)
	var g [2][2]float64
	for x := 1; x < w; x++ {
		i := row + x
		var r [2][2]float64
		if x == 1 {
			r[0][0], r[1][1] = l.west[0][i], l.west[1][i]
		}
		if x == w-1 {
			r[0][0] += l.east[0][i]
			r[1][1] += l.east[1][i]
		}
		if x > 1 {
			for c := range 2 {
				r[c][0] += l.west[0][i] * g[c][0]
				r[c][1] += l.west[1][i] * g[c][1]
			}
		}
		for c := range 2 {
			g[c] = mul2(inv[i], r[c])
			q[x][c] = g[c]
		}
	}
	for x := w - 2; x >= 1; x-- {
		i := row + x
		e0, e1 := l.east[0][i], l.east[1][i]
		for c := range 2 {
			nq := q[x+1][c]
			v := mul2(inv[i], [2]float64{e0 * nq[0], e1 * nq[1]})
			q[x][c][0] += v[0]
			q[x][c][1] += v[1]
		}
	}
	m := l.block(row)
	w0, w1 := l.west[0][row], l.west[1][row]
	e0, e1 := l.east[0][row], l.east[1][row]
	m[0] -= w0*q[w-1][0][0] + e0*q[1][0][0]
	m[1] -= w0*q[w-1][1][0] + e0*q[1][1][0]
	m[2] -= w1*q[w-1][0][1] + e1*q[1][0][1]
	m[3] -= w1*q[w-1][1][1] + e1*q[1][1][1]
	l.f.ringQ[cy] = q
	l.f.ringFirst[cy] = inv2(m)
}

// row solves row cy's two layers, t and d, exactly for the rows either
// side, with b0 and b1 the two layers' equations' right-hand sides.
func (l *seaSlab) row(cy int, t, d, b0, b1 []float64, r *slabRow) {
	e := l.e
	w := e.W
	row := cy * w
	field, base := [2][]float64{t, d}, [2][]float64{b0, b1}
	for _, c := range l.f.chains[cy] {
		for x := int(c[0]); ; x = (x + 1) % w {
			i := row + x
			for q := range 2 {
				s := base[q][i]
				if c := l.north[q][i]; c != 0 {
					s += c * field[q][i-w]
				}
				if c := l.south[q][i]; c != 0 {
					s += c * field[q][i+w]
				}
				if j := l.from[q][i]; j >= 0 {
					s += l.corner[q][i] * field[q][j]
				}
				r.rhs[x][q] = s
			}
			if x == int(c[1]) {
				break
			}
		}
	}
	if q := l.f.ringQ[cy]; q != nil {
		l.ringRow(cy, t, d, q, r)
		return
	}
	for _, c := range l.f.chains[cy] {
		l.chain(row, int(c[0]), int(c[1]), t, d, r)
	}
}

// chain solves the cells of row from first east to last, with land at
// either end, on the factored blocks.
func (l *seaSlab) chain(row, first, last int, t, d []float64, r *slabRow) {
	w := l.e.W
	inv := l.f.inv
	prev := -1
	for x := first; ; x = (x + 1) % w {
		i := row + x
		g := r.rhs[x]
		if prev >= 0 {
			pg := r.g[prev]
			g[0] += l.west[0][i] * pg[0]
			g[1] += l.west[1][i] * pg[1]
		}
		r.g[x] = mul2(inv[i], g)
		if x == last {
			break
		}
		prev = x
	}
	var next [2]float64
	for x := last; ; x = (x - 1 + w) % w {
		i := row + x
		v := r.g[x]
		if x != last {
			c := mul2(inv[i], [2]float64{l.east[0][i] * next[0], l.east[1][i] * next[1]})
			v[0] += c[0]
			v[1] += c[1]
		}
		t[i], d[i] = v[0], v[1]
		next = v
		if x == first {
			return
		}
	}
}

// ringRow solves a row of sea all the way round on its factors: see
// factorRing.
func (l *seaSlab) ringRow(cy int, t, d []float64, q [][2][2]float64, r *slabRow) {
	w := l.e.W
	row := cy * w
	inv := l.f.inv
	for x := 1; x < w; x++ {
		i := row + x
		g := r.rhs[x]
		if x > 1 {
			g[0] += l.west[0][i] * r.g[x-1][0]
			g[1] += l.west[1][i] * r.g[x-1][1]
		}
		r.g[x] = mul2(inv[i], g)
	}
	for x := w - 1; x >= 1; x-- {
		p := r.g[x]
		if x < w-1 {
			i := row + x
			np := r.p[x+1]
			c := mul2(inv[i], [2]float64{l.east[0][i] * np[0], l.east[1][i] * np[1]})
			p[0] += c[0]
			p[1] += c[1]
		}
		r.p[x] = p
	}
	rhs := r.rhs[0]
	rhs[0] += l.west[0][row]*r.p[w-1][0] + l.east[0][row]*r.p[1][0]
	rhs[1] += l.west[1][row]*r.p[w-1][1] + l.east[1][row]*r.p[1][1]
	x0 := mul2(l.f.ringFirst[cy], rhs)
	t[row], d[row] = x0[0], x0[1]
	for x := 1; x < w; x++ {
		p, c := r.p[x], q[x]
		t[row+x] = p[0] + c[0][0]*x0[0] + c[1][0]*x0[1]
		d[row+x] = p[1] + c[0][1]*x0[0] + c[1][1]*x0[1]
	}
}

// carried is the heat the sea carries toward the north across each row of
// cells, in watts over the whole parallel, from the mixed layer's warmth t
// and the water's under it d: each layer's transport times its warmth
// against the row's mean, which takes out what a row that does not close
// its water across the parallel would carry for nothing, and the eddies'
// stirring down the gradient of each layer.
func (l *seaSlab) carried(t, d []float64) []float64 {
	e := l.e
	out := make([]float64, e.H)
	field := [2][]float64{t, d}
	for cy := 0; cy < e.H; cy++ {
		row := cy * e.W
		var mean, k float64
		for x := range e.W {
			if l.take[0][row+x] != 0 {
				mean += t[row+x]
				k++
			}
		}
		if k == 0 {
			continue
		}
		mean /= k
		thick := [2]float64{l.depth[cy], deepLayer}
		var s float64
		for x := range e.W {
			i := row + x
			if l.take[0][i] == 0 {
				continue
			}
			// The water under the mixed layer carries the cold under the
			// thermocline where that lies within it.
			carriedAt := [2]float64{water(t[i]), water((1-l.below[i])*d[i] + l.below[i]*l.cold[cy])}
			for q := range 2 {
				f := field[q]
				s += l.carry[q][i] * (carriedAt[q] - mean)
				// The eddies, down the gradient toward the north.
				var grad, span float64
				hi, lo := i, i
				if cy > 0 && l.take[0][i-e.W] != 0 {
					hi, span = i-e.W, span+e.Dy
				}
				if cy < e.H-1 && l.take[0][i+e.W] != 0 {
					lo, span = i+e.W, span+e.Dy
				}
				if span > 0 {
					grad = (water(f[hi]) - water(f[lo])) / span
				}
				s -= eddyStir * thick[q] * grad
			}
		}
		out[cy] = seaHeat * s * e.Dx[cy]
	}
	return out
}

// water is the warmth of sea water solved for as t: the slab's warmth is
// relaxed toward the air's over the polar seas too, and stands colder than
// the sea can, but water colder than seaIce is ice over water at seaIce, and
// what the currents carry under it is water at seaIce. The sea's own ice is
// M7's (docs/ocean-model-plan.md).
func water(t float64) float64 { return math.Max(seaIce, t) }

// seaHeat is the heat the sea's carrying leaves in each band of the energy
// balance's sea column, W a square metre of sea, from carried, the heat
// carried across each row of cells. What crosses each face between the bands
// is read off the rows by latitude, nothing at the poles; and since the
// balance's bands are ebmLand land everywhere, the heat a band is left is
// spread over that much sea, so that each band of the balance is given what
// the sea leaves in it, whatever share of it the map's sea is.
func (e *Env) seaHeat(carried []float64) [ebmBands]float64 {
	const n = ebmBands
	at := func(lat float64) float64 {
		// The rows run from the north down.
		if lat >= e.lat[0] {
			return carried[0] * (90 - lat) / (90 - e.lat[0])
		}
		if lat <= e.lat[e.H-1] {
			return carried[e.H-1] * (lat + 90) / (e.lat[e.H-1] + 90)
		}
		for cy := 1; cy < e.H; cy++ {
			if lat >= e.lat[cy] {
				f := (lat - e.lat[cy]) / (e.lat[cy-1] - e.lat[cy])
				return carried[cy] + (carried[cy-1]-carried[cy])*f
			}
		}
		return 0
	}
	var face [n + 1]float64
	for k := 1; k < n; k++ {
		face[k] = at(math.Asin(-1+float64(k)*2/n) * 180 / math.Pi)
	}
	area := 4 * math.Pi * planetRadius * planetRadius / n * (1 - ebmLand)
	var q [n]float64
	for k := range q {
		q[k] = (face[k] - face[k+1]) / area
	}
	return q
}

// SeaShift is how many degrees warmer the air over the sea at lat stands, in
// the year's mean, for the heat this map's sea carries against what the
// energy balance's own sea carries: the balance's response to the difference,
// which the sea's warmth is given (see currents). Nought on a valley.
func (e *Env) SeaShift(lat float64) float64 {
	if e.Carried == nil {
		return 0
	}
	return ebmRead(&e.seaShift, lat)
}

// Balance is a settled year of the energy balance, read for the heat the air
// and the sea carry toward the poles and for where its warmth stands highest.
type Balance struct {
	c   *ebmClimate
	sea [ebmBands + 1]float64 // what the sea carries north across each face, W
	e   *Env
}

// SeaBalance is the energy balance worked out again, the whole of its seasonal
// year, with this map's sea in place of the balance's own: SeaHeat in each
// band's sea column and the air's diffusion as it is. The world is not made
// from it - the air's belts are laid down before the sea is solved, and the
// sea is given only the balance's response, linearized (SeaShift) - so it is
// a reading of what the map's sea would do to the energy balance, the
// ITCZ's year among it: most of a second. Nil on a valley.
func (e *Env) SeaBalance() *Balance {
	if e.Carried == nil {
		return nil
	}
	p := ebmReference(landSeaExchange)
	p.ds = 0
	q := e.SeaHeat
	p.sea = &q
	b := &Balance{c: solveEBMWith(p, e.forcing), e: e}
	area := 4 * math.Pi * planetRadius * planetRadius / ebmBands * (1 - ebmLand)
	for k := range q {
		b.sea[k+1] = b.sea[k] - q[k]*area
	}
	return b
}

// Reference is the energy balance a map's air is worked out from, under its
// forcing, with the balance's own sea.
func (e *Env) Reference() *Balance {
	b := &Balance{c: e.circ, e: e}
	b.sea = e.circ.seaNorth
	return b
}

// faceRead reads a field on the balance's faces at a latitude in degrees.
func faceRead(field *[ebmBands + 1]float64, lat float64) float64 {
	g := (math.Sin(lat*math.Pi/180) + 1) / (2.0 / ebmBands)
	g = math.Max(0, math.Min(ebmBands, g))
	k := min(int(g), ebmBands-1)
	return field[k] + (field[k+1]-field[k])*(g-float64(k))
}

// AirCarries and SeaCarries are the heat the balance's air and sea carry
// toward the north across the parallel at lat, in watts, over the year.
func (b *Balance) AirCarries(lat float64) float64 { return faceRead(&b.c.airNorth, lat) }
func (b *Balance) SeaCarries(lat float64) float64 { return faceRead(&b.sea, lat) }

// ITCZ is the balance's energy flux equator sinT of the way into the north's
// summer, over the map's tropics: see Env.ITCZ.
func (b *Balance) ITCZ(sinT float64) float64 {
	land := float64(ebmLand)
	if b.e.Wrap {
		north := (1 + sinT) / 2
		land = b.e.tropicN*north + b.e.tropicS*(1-north)
	}
	return b.c.itcz(land, sinT)
}

// MeanAt is the balance's year's mean at lat.
func (b *Balance) MeanAt(lat float64) float64 { return b.c.at(&b.c.mean, lat) }

// slabBands is how many bands of rows a sweep is cut into: some sixteen
// rows each. It is a count of rows and not of goroutines, so the sweep is
// the same however many there are.
func slabBands(h int) int { return max(1, h/16) }

// sweep is one sweep of the rows over x for the right-hand side b: the rows
// cut into bands, each band's rows swept north to south and back, the
// even bands side by side and then the odd ones, each reading the bands
// either side as they stand. A band's rows read only their own band's and
// the rows either side of it, so the even bands never read what another is
// writing, nor the odd ones; and since which band a row is in does not
// depend on the goroutines, neither does the sweep.
func (l *seaSlab) sweep(x, b []float64, rows []*slabRow) {
	e := l.e
	n := e.W * e.H
	nb := len(rows)
	for parity := range 2 {
		half := (nb + 1 - parity) / 2
		inParallel(half, workersFor(half), func(k, _ int) {
			band := 2*k + parity
			lo, hi := band*e.H/nb, (band+1)*e.H/nb
			r := rows[band]
			for cy := lo; cy < hi; cy++ {
				l.row(cy, x[:n], x[n:], b[:n], b[n:], r)
			}
			for cy := hi - 1; cy >= lo; cy-- {
				l.row(cy, x[:n], x[n:], b[:n], b[n:], r)
			}
		})
	}
}

// applyRows is apply, its rows spread over goroutines.
func (l *seaSlab) applyRows(x, out []float64) {
	e := l.e
	nb := slabBands(e.H)
	inParallel(nb, workersFor(nb), func(band, _ int) {
		l.applyBand(x, out, band*e.H/nb, (band+1)*e.H/nb)
	})
}
