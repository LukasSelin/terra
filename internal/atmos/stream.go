package atmos

import "math"

// The sea's current read as a whole: where it turns, and round what.
//
// The current on each cell says which way the water there goes and nothing
// about the ocean it goes round. What does is the streamfunction: the current
// drawn as the contours of one surface, the water running along them, fast
// where they crowd and round and round a hill or a hollow of it. A gyre is a
// hill or a hollow; its western current is where the contours crowd against
// the shore. It is worked out here from the current alone - from how the
// water spins on each cell, put back together over the sea - so that it says
// the same of a current however the current was worked out.

// GyreDepth is how deep, in metres, the water the gyres drive round goes: the
// depth a current's speed is taken over for the water it carries. See
// gyreDepth.
const GyreDepth = gyreDepth

// Stream is the sea's current as a streamfunction ψ over each cell, in square
// metres a second: the current toward the east is -∂ψ/∂y and toward the north
// ∂ψ/∂x, so the water goes round a hill of ψ clockwise, seen from above, and
// round a hollow the other way, and between two contours each metre of depth
// carries their difference. It is solved from the current's spin (the
// relative vorticity, ∂v/∂x - ∂u/∂y): ∇²ψ is the spin over the sea, and ψ is
// held at nought on the land and past the map's top and bottom rows. The
// water the wind drives straight off a shore (the Ekman drift's spreading,
// which has no spin) is not in it. Every shore is held at the same nought,
// which is right for a continent and only nearly so for an island in a strong
// current, round which the real ψ stands a little higher or lower.
//
// A gyre is thousands of kilometres across, and ψ is solved on cells of up
// to streamCell rather than the air's own and read back between them: the
// shape of an ocean's turning is in it, and a strait narrower than its cells
// is not.
//
// It is nil where there is no current, and solved in a fixed order on one
// goroutine, so it is the same on every run.
func (e *Env) Stream() []float64 {
	if e.Cu == nil {
		return nil
	}
	n := e.W * e.H
	wet := func(i int) bool { return e.Sea[i] > 0.5 }
	spin := make([]float64, n)
	for cy := 0; cy < e.H; cy++ {
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			if !wet(i) {
				continue
			}
			// Against a shore the change is read one-sided, from the water
			// alone: the land has no current to be a change from.
			d := func(a, b int, v []float32, span float64) float64 {
				wa, wb := a != i && wet(a), b != i && wet(b)
				switch {
				case wa && wb:
					return (float64(v[b]) - float64(v[a])) / (2 * span)
				case wa:
					return (float64(v[i]) - float64(v[a])) / span
				case wb:
					return (float64(v[b]) - float64(v[i])) / span
				}
				return 0
			}
			dvdx := d(e.at(cx-1, cy), e.at(cx+1, cy), e.Cv, e.Dx[cy])
			var dudy float64
			if cy > 0 && cy < e.H-1 {
				// The rows run from the north down, so the north is cy-1.
				dudy = d(e.at(cx, cy+1), e.at(cx, cy-1), e.Cu, e.Dy)
			}
			spin[i] = dvdx - dudy
		}
	}
	return e.poisson(spin, wet)
}

const (
	// streamCell is the most, in metres, a side of the cells ψ is solved on
	// may be: two hundred kilometres, a tenth of the narrowest gyre. On the
	// globe preset that is the air's cells two by two.
	streamCell = 200e3
	// streamStart is how many lattices coarser still the solve is started
	// on, each one's ψ the start of the next finer one's.
	streamStart = 2
	// streamRounds is the most rounds a lattice is given to settle, and
	// streamSettled the change in a round, as a share of the largest ψ, that
	// is settled.
	streamRounds  = 4000
	streamSettled = 1e-4
)

// lattice is a grid of cells for the solve: w by h of them, dx metres across
// along each row and dy down, the source f on each and which are sea.
type lattice struct {
	w, h int
	dx   []float64
	dy   float64
	f    []float64
	sea  []bool
}

// coarser is l with its cells joined two by two. A joined cell is sea where
// three of the four it covers are, and its source is theirs.
func (l lattice) coarser() lattice {
	c := lattice{w: l.w / 2, h: l.h / 2, dx: make([]float64, l.h/2), dy: l.dy * 2}
	c.f, c.sea = make([]float64, c.w*c.h), make([]bool, c.w*c.h)
	for y := 0; y < c.h; y++ {
		c.dx[y] = l.dx[2*y] + l.dx[2*y+1]
		for x := 0; x < c.w; x++ {
			var s float64
			var k int
			for _, j := range [4]int{2*y*l.w + 2*x, 2*y*l.w + 2*x + 1, (2*y+1)*l.w + 2*x, (2*y+1)*l.w + 2*x + 1} {
				if l.sea[j] {
					s += l.f[j]
					k++
				}
			}
			if k > 2 {
				c.sea[y*c.w+x], c.f[y*c.w+x] = true, s/float64(k)
			}
		}
	}
	return c
}

// halves reports whether l can be joined two by two and keep a few cells
// either way.
func (l lattice) halves() bool { return l.w%2 == 0 && l.h%2 == 0 && l.h/2 >= 8 && l.w/2 >= 8 }

// poisson is ψ with ∇²ψ = f on the cells wet says are water and ψ nought on
// the rest and past the top and bottom rows, by successive over-relaxation
// in row order on cells of up to streamCell, started from the solve on
// cells coarser still, and read back onto the air's cells.
func (e *Env) poisson(f []float64, wet func(int) bool) []float64 {
	air := lattice{w: e.W, h: e.H, dx: e.Dx, dy: e.Dy, f: f, sea: make([]bool, e.W*e.H)}
	for i := range air.sea {
		air.sea[i] = wet(i)
	}
	solve, k := air, 1
	for 2*solve.dy <= streamCell && solve.halves() {
		solve, k = solve.coarser(), 2*k
	}
	ladder := []lattice{solve}
	for len(ladder) <= streamStart && ladder[len(ladder)-1].halves() {
		ladder = append(ladder, ladder[len(ladder)-1].coarser())
	}
	var psi []float64
	for l := len(ladder) - 1; l >= 0; l-- {
		g := ladder[l]
		p := make([]float64, g.w*g.h)
		if psi != nil {
			// The coarser solution, each of its cells given to the four it
			// covers.
			for y := 0; y < g.h; y++ {
				for x := 0; x < g.w; x++ {
					if g.sea[y*g.w+x] {
						p[y*g.w+x] = psi[(y/2)*(g.w/2)+x/2]
					}
				}
			}
		}
		e.relax(p, g)
		psi = p
	}
	if k == 1 {
		return psi
	}
	// Read back onto the air's cells between the centres of the solve's, and
	// nought on land.
	out := make([]float64, e.W*e.H)
	at := func(x, y int) float64 {
		y = min(max(y, 0), solve.h-1)
		if e.Wrap {
			x = ((x % solve.w) + solve.w) % solve.w
		} else {
			x = min(max(x, 0), solve.w-1)
		}
		return psi[y*solve.w+x]
	}
	for cy := 0; cy < e.H; cy++ {
		fy := (float64(cy)+0.5)/float64(k) - 0.5
		y0 := int(math.Floor(fy))
		ty := fy - float64(y0)
		for cx := 0; cx < e.W; cx++ {
			i := cy*e.W + cx
			if !air.sea[i] {
				continue
			}
			fx := (float64(cx)+0.5)/float64(k) - 0.5
			x0 := int(math.Floor(fx))
			tx := fx - float64(x0)
			a := at(x0, y0) + (at(x0+1, y0)-at(x0, y0))*tx
			b := at(x0, y0+1) + (at(x0+1, y0+1)-at(x0, y0+1))*tx
			out[i] = a + (b-a)*ty
		}
	}
	return out
}

// relax settles p toward ∇²p = f on the lattice g, in place.
func (e *Env) relax(p []float64, g lattice) {
	w, h := g.w, g.h
	// The best over-relaxation for a lattice this long (Young, 1954).
	omega := 2 / (1 + math.Sin(math.Pi/float64(max(w, h))))
	iy := 1 / (g.dy * g.dy)
	for round := 0; round < streamRounds; round++ {
		var most, top float64
		for y := 0; y < h; y++ {
			ix := 1 / (g.dx[y] * g.dx[y])
			row := y * w
			for x := 0; x < w; x++ {
				i := row + x
				if !g.sea[i] {
					continue
				}
				var west, east, north, south float64
				switch {
				case x > 0:
					west = p[i-1]
				case e.Wrap:
					west = p[row+w-1]
				}
				switch {
				case x < w-1:
					east = p[i+1]
				case e.Wrap:
					east = p[row]
				}
				if y > 0 {
					north = p[i-w]
				}
				if y < h-1 {
					south = p[i+w]
				}
				next := ((west+east)*ix + (north+south)*iy - g.f[i]) / (2*ix + 2*iy)
				d := omega * (next - p[i])
				p[i] += d
				most = math.Max(most, math.Abs(d))
				top = math.Max(top, math.Abs(p[i]))
			}
		}
		if most <= streamSettled*top {
			return
		}
	}
}
