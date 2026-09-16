package terra

import (
	"math"

	"github.com/LukasSelin/terra/internal/kernel"
)

// The rain the ground wrings out of the wind.
//
// Air made to climb a slope cools, and what it cannot hold condenses, drifts
// on while it grows into drops, and falls - some way downwind of the slope
// that lifted it. The rain on a range used to be a tile's own rise along the
// wind, turned into rain by a scale height shortened for these maps' low
// mountains and a length of coast fitted so that a three-hundred-metre ridge
// cast a shadow somebody would notice, and none of it came out of the air.
//
// Here it is Smith and Barstad's (2004) linear theory of orographic
// precipitation. The ground's shape, taken to its Fourier modes, lifts the air
// at the rate the wind climbs it; the lift is carried up through the air as
// the mountain waves carry it, stronger or weaker with height by the moist
// stability of the air; the vapour it condenses takes τc to become cloud and
// τf to fall, and is carried downwind all the while:
//
//	P̂(k,l) = Cw iσĥ / [(1 - i m Hw)(1 + iστc)(1 + iστf)],  σ = Uk + Vl
//
// with m the vertical wavenumber of the waves. What that gives less than
// nothing - the air coming down the lee, drying - is none, and what it gives is
// taken out of the column of water over the ground (vapour.go), which cannot
// give more than it has: the range's rain is its windward air's water, and the
// air that comes over it has that much less to rain further on.
//
// The theory is for a wind that is the same everywhere. The wind of a planet
// is not, so the ground is read in patches - a thousand kilometres or so of a
// globe, a few dozen of a valley - each with the wind over its middle, tapered
// so that the patches overlapping by half add up to the ground again. A globe's
// rows go round, and so do its patches.

// The air the ground lifts.
const (
	// moistStability is N_m, the Brunt-Väisälä frequency of saturated air,
	// per second: Smith and Barstad's 0.005, half the dry air's.
	moistStability = 0.005
	// cloudTime and fallTime are τc and τf, the seconds vapour takes to turn to
	// cloud and cloud to fall out: some thousand each (Smith and Barstad,
	// 2004; Jiang and Smith, 2003).
	cloudTime = 1000.0
	fallTime  = 1000.0
	t         // dryGas is R_d, the gas constant of dry air, J/kg/K.
	dryGas    = 287.0
	// stormWind is the wind, m/s, the storms that rain on a range blow at:
	// the moist flows Smith and Barstad's theory was held to on the Alps and
	// the Coast Ranges blew at ten to twenty (Smith and Barstad, 2004;
	// Barstad and Smith, 2005). It is the least the lift is read at.
	stormWind = 10.0
	// reliefLeast is how many metres of relief a patch has to have for its
	// lift to be worked out: a slope of ten metres over a patch rains less
	// than a millimetre a year.
	reliefLeast = 10.0
	// patchReach is how many kilometres a patch of ground is read over, at
	// least: enough for the air to go some way past the slope that lifted it,
	// which at ten metres a second over τc and τf is twenty kilometres.
	patchReach = 150.0
	// patchLeast and patchMost are the fewest and most tiles a patch is across.
	patchLeast = 16
	patchMost  = 64
)

// moistLapse is Γ_m, the rate saturated air at temp degrees near the ground
// cools as it rises, K/m (the moist adiabat; American Meteorological Society,
// Glossary of Meteorology).
func moistLapse(temp float64) float64 {
	t := math.Max(temp, coldest) + 273.15
	rs := saturation(temp) / (1 - saturation(temp))
	return gravity * (1 + latentHeat*rs/(dryGas*t)) /
		(airHeat + latentHeat*latentHeat*rs*0.622/(dryGas*t*t))
}

// orographicPatch is the size in tiles of the patches g's ground is read in:
// a power of two, patchReach across or more, and no more than patchMost.
func (g *Grid) orographicPatch() int {
	a := g.air
	span := a.dy // km a tile, down the rows
	p := patchLeast
	for p < patchMost && float64(p)*span < patchReach {
		p *= 2
	}
	return p
}

// orographic is what the ground's lift would rain out of saturated air, in
// kg/m²/s on each tile, under the wind u, v of one phase over air at sea level
// of temp degrees, both on the air cells. ground is the height of each tile
// over the water the air takes its fill from.
func (g *Grid) orographic(e *airEnv, u, v []float32, temp, ground []float64) []float32 {
	defer phase("orographic")()
	a := g.air
	if !g.Wrap {
		return g.orographicWhole(e, u, v, temp, ground)
	}
	size := g.orographicPatch()
	step := size / 2
	out := make([]float32, len(g.Tiles))
	// The taper: sin² over a patch, which with the patches half a patch apart
	// adds to one everywhere.
	taper := make([]float64, size)
	for k := range taper {
		s := math.Sin(math.Pi * (float64(k) + 0.5) / float64(size))
		taper[k] = s * s
	}
	// The patches' corners, each half a patch from the last, from half a patch
	// before the map; a globe's go round and end where they began.
	var xs, ys []int
	if g.Wrap {
		for x := 0; x < g.W; x += step {
			xs = append(xs, x-step)
		}
	} else {
		for x := -step; x < g.W; x += step {
			xs = append(xs, x)
		}
	}
	for y := -step; y < g.H; y += step {
		ys = append(ys, y)
	}
	type patch struct{ x0, y0 int }
	var patches []patch
	for _, y0 := range ys {
		for _, x0 := range xs {
			patches = append(patches, patch{x0, y0})
		}
	}
	// Each patch writes only to its own sum, and the sums are added in order
	// afterwards, so the rain does not depend on how the patches were dealt.
	sums := make([][]float32, len(patches))
	workers := 1
	if len(g.Tiles) >= spreadTiles {
		workers = WorkersFor(len(patches))
	}
	bufs := make([][]complex128, workers)
	cols := make([][]complex128, workers)
	InParallel(len(patches), workers, func(pi, worker int) {
		pt := patches[pi]
		at := func(x, y int) int {
			y = min(max(y, 0), g.H-1)
			if g.Wrap {
				x = ((x % g.W) + g.W) % g.W
			} else {
				x = min(max(x, 0), g.W-1)
			}
			return y*g.W + x
		}
		top, bottom := 0.0, math.Inf(1)
		for dy := 0; dy < size; dy++ {
			for dx := 0; dx < size; dx++ {
				h := ground[at(pt.x0+dx, pt.y0+dy)]
				top, bottom = math.Max(top, h), math.Min(bottom, h)
			}
		}
		if top-bottom < reliefLeast {
			// Ground as flat as this lifts nothing worth a transform.
			return
		}
		// The air over the middle of the patch.
		mx, my := pt.x0+step, min(max(pt.y0+step, 0), g.H-1)
		fx, fy := e.cellAt(g, at(mx, my))
		// The patch is laid in the middle of a field twice its size, so that
		// what the waves and the drifting cloud carry past its edges is not
		// carried round onto its other side.
		box := 2 * size
		pad := size / 2
		if bufs[worker] == nil {
			bufs[worker], cols[worker] = make([]complex128, box*box), make([]complex128, box)
		}
		buf, col := bufs[worker], cols[worker]
		clear(buf)
		for dy := 0; dy < size; dy++ {
			for dx := 0; dx < size; dx++ {
				buf[(dy+pad)*box+dx+pad] = complex(ground[at(pt.x0+dx, pt.y0+dy)]*taper[dx]*taper[dy], 0)
			}
		}
		if !liftField(buf, col, box, box, e.sample32(u, fx, fy), e.sample32(v, fx, fy), e.sample(temp, fx, fy), a.dx[my]*km, a.dy*km) {
			return
		}
		sum := make([]float32, box*box)
		for i := range sum {
			sum[i] = float32(real(buf[i]))
		}
		sums[pi] = sum
	})
	acc := make([]float64, len(g.Tiles))
	for pi, sum := range sums {
		if sum == nil {
			continue
		}
		pt := patches[pi]
		box, pad := 2*size, size/2
		for dy := 0; dy < box; dy++ {
			y := pt.y0 - pad + dy
			if y < 0 || y >= g.H {
				continue
			}
			for dx := 0; dx < box; dx++ {
				x := pt.x0 - pad + dx
				if g.Wrap {
					x = ((x % g.W) + g.W) % g.W
				} else if x < 0 || x >= g.W {
					continue
				}
				acc[y*g.W+x] += float64(sum[dy*box+dx])
			}
		}
	}
	for i, p := range acc {
		out[i] = float32(math.Max(0, p))
	}
	return out
}

// liftField turns buf, a field of ground bw by bh tiles of dxm by dym metres
// laid row by row, into what its lift rains out of saturated air at temp
// degrees in the wind uu, vv, in kg/m²/s, in place: Smith and Barstad's
// transfer function between the transform and its inverse. col is room for a
// column. It is false, and buf is left as it was, where there is no wind.
func liftField(buf, col []complex128, bw, bh int, uu, vv, temp, dxm, dym float64) bool {
	speed := math.Hypot(uu, vv)
	if speed < calm {
		return false
	}
	// The rain on a range falls from the storms that drive moist air at it,
	// and not in the phase's mean wind: the waves and the drifting cloud are
	// read at the storm's wind, the way the mean wind blows, and what they
	// give is what the mean wind's flux up the slope gives.
	storm := math.Max(1, stormWind/speed)
	uu, vv = uu*storm, vv*storm
	tk := temp + 273.15
	cw := saturatedColumn(temp) / vapourHeight * moistLapse(temp) / Lapse
	hw := vapourGas * tk * tk / (latentHeat * Lapse)
	kernel.FFT2(buf, bw, bh, false, col)
	n2 := moistStability * moistStability
	for r := 0; r < bh; r++ {
		rr := r
		if rr >= bh/2 {
			rr -= bh
		}
		// Down the rows is toward the south.
		l := -2 * math.Pi * float64(rr) / (float64(bh) * dym)
		for c := 0; c < bw; c++ {
			cc := c
			if cc >= bw/2 {
				cc -= bw
			}
			k := 2 * math.Pi * float64(cc) / (float64(bw) * dxm)
			sigma := uu*k + vv*l
			i := r*bw + c
			if math.Abs(sigma) < 1e-12 {
				buf[i] = 0
				continue
			}
			kk := k*k + l*l
			var m complex128
			if s2 := sigma * sigma; s2 < n2 {
				m = complex(math.Copysign(math.Sqrt((n2-s2)/s2*kk), sigma), 0)
			} else {
				m = complex(0, math.Sqrt((s2-n2)/s2*kk))
			}
			den := (1 - 1i*m*complex(hw, 0)) * complex(1, sigma*cloudTime) * complex(1, sigma*fallTime)
			buf[i] *= complex(0, cw*sigma/storm) / den
		}
	}
	kernel.FFT2(buf, bw, bh, true, col)
	return true
}

// orographicWhole is orographic on a map that is not a globe: a valley is a
// few score kilometres, under one wind, and is taken whole, in a field with
// room round it into which its edges fall away to nothing.
func (g *Grid) orographicWhole(e *airEnv, u, v []float32, temp, ground []float64) []float32 {
	out := make([]float32, len(g.Tiles))
	padX, padY := patchLeast, patchLeast
	bw, bh := nextPowerOfTwo(g.W+2*padX), nextPowerOfTwo(g.H+2*padY)
	fall := func(d, pad int) float64 {
		if d <= 0 {
			return 1
		}
		if d >= pad {
			return 0
		}
		c := math.Cos(math.Pi / 2 * float64(d) / float64(pad))
		return c * c
	}
	buf := make([]complex128, bw*bh)
	top, bottom := 0.0, math.Inf(1)
	for by := 0; by < bh; by++ {
		y := by - padY
		cy := min(max(y, 0), g.H-1)
		wy := fall(max(-y, y-(g.H-1)), padY)
		for bx := 0; bx < bw; bx++ {
			x := bx - padX
			cx := min(max(x, 0), g.W-1)
			h := ground[cy*g.W+cx]
			if x >= 0 && x < g.W && y >= 0 && y < g.H {
				top, bottom = math.Max(top, h), math.Min(bottom, h)
			}
			buf[by*bw+bx] = complex(h*wy*fall(max(-x, x-(g.W-1)), padX), 0)
		}
	}
	if top-bottom < reliefLeast {
		return out
	}
	n := e.w * e.h
	var uu, vv, t float64
	for i := range n {
		uu += float64(u[i]) / float64(n)
		vv += float64(v[i]) / float64(n)
		t += temp[i] / float64(n)
	}
	col := make([]complex128, max(bw, bh))
	if !liftField(buf, col[:bh], bw, bh, uu, vv, t, g.air.dx[g.H/2]*km, g.air.dy*km) {
		return out
	}
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			out[y*g.W+x] = float32(math.Max(0, real(buf[(y+padY)*bw+x+padX])))
		}
	}
	return out
}

// nextPowerOfTwo is the least power of two no less than n.
func nextPowerOfTwo(n int) int {
	p := 1
	for p < n {
		p *= 2
	}
	return p
}
