package atmos

import (
	"math"
	"slices"
)

// The gyres in two dimensions.
//
// What the wind drives round an ocean is read off one field, the transport
// streamfunction ψ: the water between two places flows past them at ψ's
// difference between them, in cubic metres a second, with the higher ψ on
// its right. The water does not pile up anywhere, so one field says it all.
// It is the steady state of the barotropic vorticity equation,
//
//	r ∇²ψ − A ∂⁴ψ/∂x⁴ + β ∂ψ/∂x = curl τ / ρ
//
// in which the wind's turning (curl τ) is balanced by the water being carried
// toward the pole or the equator across the planet's turning (β, Sverdrup,
// 1947), and where that is not enough, against a shore, by the drag of the sea
// floor (r, Stommel, 1948) and by the water's own sideways stirring (A, Munk,
// 1950). In the open ocean the first two balance and the water drifts slowly
// toward the equator under the subtropical highs; against an ocean's western
// shore the friction takes over in a current a few cells wide, which carries
// all of it back: the Gulf Stream. Because it is solved over the map in both
// directions at once, the current turns the corners of the coast, runs round
// islands and through the straits between them, and an ocean all the way
// round a parallel carries its current round the planet.
//
// The stirring is along the parallels only. Munk's layer has to be a few
// cells wide to be resolved, and a stirring that strong in every direction
// slows the open ocean's own gyres, whose breadth north to south is a few
// cells times ten: on the air's cells of a coarse globe it halved them. Ocean
// models stir harder east and west than north and south for the same reason
// (Large and others, 2001; Smith and McWilliams, 2003).
//
// Every landmass is a shore the water cannot cross, so ψ is the same all
// round it. The largest is held at nought; every other has a level of its
// own, which is set by the island rule (Godfrey, 1989): the wind's pull all
// the way round an island is taken up by the friction all the way round it.
// Here that is the island's own equation, the sum of the vorticity equation
// over the island's cells, solved with the rest - the same constants as one
// more solve for each island and a circulation condition, without the
// solves, which on a globe of a few hundred islands would be a few hundred.
// The southern continent of a world with an ocean all the way round it
// stands at its own level, and the difference between it and the next
// continent north is the circumpolar current.
//
// It is worked out on the air's cells, on the sphere: the cells grow narrow
// toward the poles, the faces between the rows are as long as the parallel
// they lie on, and nothing crosses the pole. β is the planet's turning's
// change with latitude, largest on the equator, where f is nought but the
// balance is not, and nothing is skipped there.
//
// The solve is GMRES (Saad and Schultz, 1986), preconditioned by one
// multigrid cycle that coarsens the rows two into one and never the
// columns, and smooths by solving each row exactly along itself, the even
// rows and then the odd (Trottenberg, Oosterlee and Schüller, 2001, on
// semicoarsening with line relaxation for equations far stronger one way
// than the other, as this one is: β and the stirring are along the rows).
// The coarse equations are the fine ones summed (Galerkin), the islands
// carried down whole. Every sum is taken in one fixed order, and the result
// is the same however many goroutines the land allows.

const (
	// bottomDrag is r, how fast, per second, the drag of the sea floor slows
	// the water the wind drives: a spin-down of some three weeks. That is
	// more than the floor of a deep, flat ocean gives, standing for what a
	// flat floor leaves out, the mountains on the real one that the deep
	// currents push against (Munk and Palmén, 1951); and less than would
	// hold a current all the way round the planet to the real one's hundred
	// and fifty million cubic metres a second, which would slow the gyres by
	// half: the friction on a gyre's north to south breadth is r k² against
	// the planet's turning βk across it. Under it the current against an
	// ocean's western shore would be r/β, some twenty-five kilometres, wide
	// (Stommel, 1948): Munk's layer is what sets its width.
	bottomDrag = 5e-7
	// munkCells is how many cells across Munk's layer, (A/β)^⅓, is on every
	// row: A is set from it rather than held the same everywhere, so that the
	// western current is a few cells wide and resolved at every latitude
	// (Bryan, Manabe and Pacanowski, 1975, whose criterion is that the layer
	// be at least a cell). On the air's cells of eighty kilometres it is some
	// a hundred and sixty kilometres, a little wider than the Gulf Stream.
	munkCells = 2.0
	// flowSettled is how small the residual of the equations is to be, as a
	// share of their forcing; flowRestart how many directions GMRES keeps
	// before it starts again from where it got, and flowMost how many it
	// takes at most all told.
	flowSettled = 1e-3
	flowRestart = 30
	flowMost    = 300
	// coarsestSweeps is how many times the single row the multigrid comes
	// down to, and the islands with it, are solved in turn.
	coarsestSweeps = 4
	// spreadFlow is how many unknowns a level has before its sweeps are
	// spread over goroutines.
	spreadFlow = 1024
)

// flow is the vorticity equation over the sea of a map, written down once
// and solved for one wind.
type flow struct {
	e       *Env
	w, h, n int
	// mass is the landmass each cell belongs to: -1 for sea, 0 for the
	// largest landmass and k for the kth other, an island.
	mass    []int32
	islands [][]int32
	// unknown is each cell's unknown: its own on the sea, its island's level
	// on an island, and -1 on the mainland.
	unknown []int32

	// The operator's coefficients on each row. The Laplacian of the sea,
	// integrated over a cell, is wx·(the cells east and west) + wn·(north) +
	// ws·(south) + kd·(the cell); ab is A over the cell's area and bc β times
	// the cell's height over two.
	wx, wn, ws, kd, ab, bc []float64
	// shift is added to every sea cell's own coefficient where there is no
	// land at all, and nothing holds ψ's level: too little to see.
	shift float64

	levels []*level
}

// level is the equations at one coarseness: the unknowns of the cells, row
// by row, then the islands'.
type level struct {
	w, rows int
	cells   int // the cells' unknowns; the islands' follow
	n       int
	at      []int32 // each cell's unknown, row by row, or -1
	// The equations, a row of coefficients for each unknown.
	start []int32
	col   []int32
	val   []float64
	// chains is each row's stretches of sea, factored.
	chains [][]*chain
	// down is each unknown's on the next level.
	down    []int32
	x, b, r []float64
	bufs    [][]float64 // a row's worth for each goroutine
}

// gyres is the transport streamfunction ψ, in cubic metres a second, under
// the wind's stress tx, ty in newtons a square metre, on every cell: on land
// it is the level of the landmass. It is for a globe only.
func (e *Env) gyres(tx, ty []float64) []float64 {
	f := e.newFlow()
	x := f.solve(f.forcing(tx, ty))
	return f.spread(x)
}

// newFlow labels the landmasses and writes down the equations at every
// coarseness.
func (e *Env) newFlow() *flow {
	w, h := e.W, e.H
	n := w * h
	f := &flow{e: e, w: w, h: h, n: n}
	f.label()

	f.wx, f.wn, f.ws, f.kd = make([]float64, h), make([]float64, h), make([]float64, h), make([]float64, h)
	f.ab, f.bc = make([]float64, h), make([]float64, h)
	for cy := 0; cy < h; cy++ {
		dx := e.Dx[cy]
		f.wx[cy] = e.Dy / dx
		if cy > 0 {
			f.wn[cy] = (e.Dx[cy-1] + dx) / 2 / e.Dy
		}
		if cy < h-1 {
			f.ws[cy] = (e.Dx[cy+1] + dx) / 2 / e.Dy
		}
		f.kd[cy] = -(2*f.wx[cy] + f.wn[cy] + f.ws[cy])
		beta := 2 * omega * math.Cos(e.lat[cy]*math.Pi/180) / planetRadius
		width := munkCells * dx
		f.ab[cy] = beta * width * width * width / (dx * e.Dy)
		f.bc[cy] = beta * e.Dy / 2
	}
	if len(f.islands) == 0 && !slices.ContainsFunc(f.mass, func(m int32) bool { return m >= 0 }) {
		f.shift = 1e-9 * bottomDrag
	}
	f.levels = []*level{f.finest()}
	for l := f.levels[0]; l.rows > 1; {
		l = l.coarser()
		f.levels = append(f.levels, l)
	}
	for _, l := range f.levels {
		l.factor()
	}
	return f
}

// label finds the landmasses, four cells to a side joined, round the seam of
// the globe but not over the poles, in the order of their first cell; the
// largest, the first of the largest on a tie, is the mainland. It numbers
// the unknowns: the sea's cells row by row, then the islands.
func (f *flow) label() {
	e := f.e
	f.mass = make([]int32, f.n)
	for i := range f.mass {
		f.mass[i] = -1
	}
	var masses [][]int32
	var stack []int32
	for i := range f.mass {
		if e.Sea[i] > 0.5 || f.mass[i] >= 0 {
			continue
		}
		k := int32(len(masses))
		var cells []int32
		f.mass[i] = k
		stack = append(stack[:0], int32(i))
		for len(stack) > 0 {
			c := int(stack[len(stack)-1])
			stack = stack[:len(stack)-1]
			cells = append(cells, int32(c))
			cx, cy := c%f.w, c/f.w
			for _, j := range [4]int{
				cy*f.w + (cx+1)%f.w, cy*f.w + (cx-1+f.w)%f.w,
				(cy-1)*f.w + cx, (cy+1)*f.w + cx,
			} {
				if j < 0 || j >= f.n || e.Sea[j] > 0.5 || f.mass[j] >= 0 {
					continue
				}
				f.mass[j] = k
				stack = append(stack, int32(j))
			}
		}
		masses = append(masses, cells)
	}
	main := 0
	for k, m := range masses {
		if len(m) > len(masses[main]) {
			main = k
		}
	}
	// The mainland is 0 and the islands 1 on, in the order they were found.
	for k, m := range masses {
		label := int32(0)
		if k != main {
			f.islands = append(f.islands, m)
			label = int32(len(f.islands))
		}
		for _, c := range m {
			f.mass[c] = label
		}
	}
	f.unknown = make([]int32, f.n)
	sea := int32(0)
	for i, m := range f.mass {
		if m < 0 {
			f.unknown[i] = sea
			sea++
		}
	}
	for i, m := range f.mass {
		switch {
		case m == 0:
			f.unknown[i] = -1
		case m > 0:
			f.unknown[i] = sea + m - 1
		}
	}
}

// spread lays the unknowns x out over the map as ψ: the sea's own, each
// island's level on its cells, and nought on the mainland.
func (f *flow) spread(x []float64) []float64 {
	psi := make([]float64, f.n)
	for i, u := range f.unknown {
		if u >= 0 {
			psi[i] = x[u]
		}
	}
	return psi
}

// stencil gives fn each cell the equation of cell i reaches and its
// coefficient there: the drag's Laplacian, the stirring along the row, and
// the planet's turning, all integrated over the cell. Its coefficients sum
// to nought, so a landmass's interior adds nothing to its island's equation.
func (f *flow) stencil(i int, fn func(j int, v float64)) {
	w := f.w
	cx, cy := i%w, i/w
	row := cy * w
	wx, ab, bc := f.wx[cy], f.ab[cy], f.bc[cy]
	stir := wx * wx * ab
	fn(i, -bottomDrag*f.kd[cy]+6*stir)
	fn(row+(cx+1)%w, -bottomDrag*wx-4*stir-bc)
	fn(row+(cx-1+w)%w, -bottomDrag*wx-4*stir+bc)
	fn(row+(cx+2)%w, stir)
	fn(row+(cx-2+w)%w, stir)
	if cy > 0 {
		fn(i-w, -bottomDrag*f.wn[cy])
	}
	if cy < f.h-1 {
		fn(i+w, -bottomDrag*f.ws[cy])
	}
}

// rowBuilder gathers one row of coefficients by column.
type rowBuilder struct {
	acc  []float64
	seen []bool
	cols []int32
}

func newRowBuilder(n int) *rowBuilder {
	return &rowBuilder{acc: make([]float64, n), seen: make([]bool, n)}
}

func (rb *rowBuilder) add(c int32, v float64) {
	if !rb.seen[c] {
		rb.seen[c] = true
		rb.cols = append(rb.cols, c)
	}
	rb.acc[c] += v
}

// flush writes the row to l, its columns in order, and starts the next.
func (rb *rowBuilder) flush(l *level) {
	slices.Sort(rb.cols)
	for _, c := range rb.cols {
		l.col = append(l.col, c)
		l.val = append(l.val, rb.acc[c])
		rb.acc[c], rb.seen[c] = 0, false
	}
	rb.cols = rb.cols[:0]
	l.start = append(l.start, int32(len(l.col)))
}

// finest is the equations on the air's cells: each sea cell's, and each
// island's summed over its cells.
func (f *flow) finest() *level {
	isl := len(f.islands)
	l := &level{w: f.w, rows: f.h, at: make([]int32, f.n)}
	for i, m := range f.mass {
		l.at[i] = -1
		if m < 0 {
			l.at[i] = f.unknown[i]
			l.cells++
		}
	}
	l.n = l.cells + isl
	l.start = append(make([]int32, 0, l.n+1), 0)
	l.col = make([]int32, 0, 7*l.cells)
	l.val = make([]float64, 0, 7*l.cells)
	rb := newRowBuilder(l.n)
	add := func(j int, v float64) {
		if u := f.unknown[j]; u >= 0 {
			rb.add(u, v)
		}
	}
	for i, m := range f.mass {
		if m < 0 {
			f.stencil(i, add)
			rb.add(f.unknown[i], f.shift)
			rb.flush(l)
		}
	}
	for k, cells := range f.islands {
		mine := int32(k + 1)
		for _, c := range cells {
			// A cell whose equation reaches only its own island adds
			// nothing to the island's: the coefficients sum to nought.
			inland := true
			f.stencil(int(c), func(j int, _ float64) {
				if f.mass[j] != mine {
					inland = false
				}
			})
			if !inland {
				f.stencil(int(c), add)
			}
		}
		rb.flush(l)
	}
	return l
}

// coarser is the level with this one's rows taken two into one: a cell of it
// is sea where either of the two under it is, and its equation is theirs
// summed, in the unknowns of the coarse level. The islands come down whole.
func (l *level) coarser() *level {
	w := l.w
	c := &level{w: w, rows: (l.rows + 1) / 2}
	c.at = make([]int32, c.rows*w)
	l.down = make([]int32, l.n)
	for cr := 0; cr < c.rows; cr++ {
		for x := 0; x < w; x++ {
			k := cr*w + x
			c.at[k] = -1
			for r := 2 * cr; r < min(2*cr+2, l.rows); r++ {
				if u := l.at[r*w+x]; u >= 0 {
					if c.at[k] < 0 {
						c.at[k] = int32(c.cells)
						c.cells++
					}
					l.down[u] = c.at[k]
				}
			}
		}
	}
	isl := l.n - l.cells
	for k := range isl {
		l.down[l.cells+k] = int32(c.cells + k)
	}
	c.n = c.cells + isl
	c.start = append(make([]int32, 0, c.n+1), 0)
	c.col = make([]int32, 0, len(l.col)*3/4)
	c.val = make([]float64, 0, len(l.col)*3/4)
	rb := newRowBuilder(c.n)
	from := func(u int32) {
		for e := l.start[u]; e < l.start[u+1]; e++ {
			rb.add(l.down[l.col[e]], l.val[e])
		}
	}
	for cr := 0; cr < c.rows; cr++ {
		for x := 0; x < w; x++ {
			if c.at[cr*w+x] < 0 {
				continue
			}
			for r := 2 * cr; r < min(2*cr+2, l.rows); r++ {
				if u := l.at[r*w+x]; u >= 0 {
					from(u)
				}
			}
			rb.flush(c)
		}
	}
	for k := range isl {
		from(int32(l.cells + k))
		rb.flush(c)
	}
	return c
}

// coef is the coefficient of unknown v in u's equation.
func (l *level) coef(u, v int32) float64 {
	cols := l.col[l.start[u]:l.start[u+1]]
	if k, ok := slices.BinarySearch(cols, v); ok {
		return l.val[int(l.start[u])+k]
	}
	return 0
}

// factor writes down each row's stretches of sea as chains, and factors
// them, and makes the level's scratch.
func (l *level) factor() {
	w := l.w
	l.chains = make([][]*chain, l.rows)
	type scratch struct {
		a  [5][]float64
		xs []int
	}
	band := func(sc *scratch, r int, xs []int, ring bool) *chain {
		cells := make([]int32, len(xs))
		for p, x := range xs {
			cells[p] = l.at[r*w+x]
		}
		var a [5][]float64
		for d := range a {
			a[d] = sc.a[d][:len(xs)]
			clear(a[d])
		}
		for p, x := range xs {
			for d := -2; d <= 2; d++ {
				if !ring && (p+d < 0 || p+d >= len(xs)) {
					continue
				}
				a[d+2][p] = l.coef(cells[p], l.at[r*w+(x+d+w)%w])
			}
		}
		return newChain(cells, a, ring)
	}
	row := func(sc *scratch, r int) {
		start := -1
		for x := 0; x < w; x++ {
			if l.at[r*w+x] < 0 {
				start = x
				break
			}
		}
		xs := sc.xs[:0]
		if start < 0 {
			for x := range w {
				xs = append(xs, x)
			}
			l.chains[r] = append(l.chains[r], band(sc, r, xs, true))
			return
		}
		for k := 1; k <= w; k++ {
			x := (start + k) % w
			if l.at[r*w+x] >= 0 {
				xs = append(xs, x)
				continue
			}
			if len(xs) > 0 {
				l.chains[r] = append(l.chains[r], band(sc, r, xs, false))
				xs = xs[:0]
			}
		}
	}
	workers := 1
	if l.n >= spreadFlow {
		workers = workersFor(l.rows)
	}
	scr := make([]scratch, workers)
	for k := range scr {
		for d := range scr[k].a {
			scr[k].a[d] = make([]float64, w)
		}
		scr[k].xs = make([]int, 0, w)
	}
	if workers == 1 {
		for r := range l.rows {
			row(&scr[0], r)
		}
	} else {
		inParallel(l.rows, workers, func(r, worker int) { row(&scr[worker], r) })
	}
	l.x, l.b, l.r = make([]float64, l.n), make([]float64, l.n), make([]float64, l.n)
}

// residual is b less the equations' left side at x, for unknown u.
func (l *level) residual(u int32) float64 {
	s := l.b[u]
	for e := l.start[u]; e < l.start[u+1]; e++ {
		s -= l.val[e] * l.x[l.col[e]]
	}
	return s
}

// residuals writes every unknown's residual to r.
func (l *level) residuals() {
	if l.n < spreadFlow {
		for u := range l.n {
			l.r[u] = l.residual(int32(u))
		}
		return
	}
	const chunk = 4096
	chunks := (l.n + chunk - 1) / chunk
	inParallel(chunks, workersFor(chunks), func(k, _ int) {
		for u := k * chunk; u < min(l.n, (k+1)*chunk); u++ {
			l.r[u] = l.residual(int32(u))
		}
	})
}

// relaxRow solves row r's stretches exactly for what x leaves of b along
// them, the rest of x as it is.
func (l *level) relaxRow(r int, buf []float64) {
	for _, c := range l.chains[r] {
		for p, u := range c.cells {
			buf[p] = l.residual(u)
		}
		c.solve(buf)
		for p, u := range c.cells {
			l.x[u] += buf[p]
		}
	}
}

// relaxIslands solves each island's equation for its level in turn.
func (l *level) relaxIslands() {
	for u := int32(l.cells); u < int32(l.n); u++ {
		l.x[u] += l.residual(u) / l.coef(u, u)
	}
}

// relax is a sweep of the even rows, the odd, and the islands, or the same
// backward. A row's equations reach only the rows either side and the
// islands, so the rows of one parity are solved at once.
func (l *level) relax(forward bool) {
	parities := [2]int{0, 1}
	if !forward {
		parities = [2]int{1, 0}
		l.relaxIslands()
	}
	for _, par := range parities {
		rows := (l.rows - par + 1) / 2
		if l.n < spreadFlow || rows < 2 {
			if len(l.bufs) == 0 {
				l.bufs = append(l.bufs, make([]float64, l.w))
			}
			buf := l.bufs[0]
			for r := par; r < l.rows; r += 2 {
				l.relaxRow(r, buf)
			}
			continue
		}
		w := workersFor(rows)
		for len(l.bufs) < w {
			l.bufs = append(l.bufs, make([]float64, l.w))
		}
		inParallel(rows, w, func(k, worker int) { l.relaxRow(par+2*k, l.bufs[worker]) })
	}
	if forward {
		l.relaxIslands()
	}
}

// cycle sets level k's x to the multigrid's answer to its b, from nought.
func (f *flow) cycle(k int) {
	l := f.levels[k]
	clear(l.x)
	if k == len(f.levels)-1 {
		for range coarsestSweeps {
			l.relax(true)
		}
		return
	}
	l.relax(true)
	c := f.levels[k+1]
	clear(c.b)
	l.residuals()
	for u, d := range l.down {
		c.b[d] += l.r[u]
	}
	f.cycle(k + 1)
	for u, d := range l.down {
		l.x[u] += c.x[d]
	}
	l.relax(false)
}

// apply is the finest equations' left side at x, written to out.
func (f *flow) apply(x, out []float64) {
	l := f.levels[0]
	body := func(u int) {
		var s float64
		for e := l.start[u]; e < l.start[u+1]; e++ {
			s += l.val[e] * x[l.col[e]]
		}
		out[u] = s
	}
	if l.n < spreadFlow {
		for u := range l.n {
			body(u)
		}
		return
	}
	const chunk = 4096
	chunks := (l.n + chunk - 1) / chunk
	inParallel(chunks, workersFor(chunks), func(k, _ int) {
		for u := k * chunk; u < min(l.n, (k+1)*chunk); u++ {
			body(u)
		}
	})
}

// precondition is z = M⁻¹r, one cycle of the multigrid.
func (f *flow) precondition(r, z []float64) {
	l := f.levels[0]
	copy(l.b, r)
	f.cycle(0)
	copy(z, l.x)
}

// forcing is the wind's turning integrated over each sea cell, and over each
// island, as the right side of the equations: the wind's pull round the
// cell's edges, each edge taking the stress of the sea beside it so that the
// wind over the land, slowed by it, is not read as a turning at the coast.
func (f *flow) forcing(tx, ty []float64) []float64 {
	e, w, h := f.e, f.w, f.h
	wet := func(i int) bool { return f.mass[i] < 0 }
	face := func(t []float64, i, j int) float64 {
		switch a, b := wet(i), wet(j); {
		case a && !b:
			return t[i]
		case b && !a:
			return t[j]
		}
		return (t[i] + t[j]) / 2
	}
	b := make([]float64, f.levels[0].n)
	for cy := 0; cy < h; cy++ {
		row := cy * w
		north := f.wn[cy] * e.Dy // the length of the row's northern edge
		south := f.ws[cy] * e.Dy
		for cx := 0; cx < w; cx++ {
			i := row + cx
			u := f.unknown[i]
			if u < 0 {
				continue
			}
			east, west := row+(cx+1)%w, row+(cx-1+w)%w
			s := e.Dy * (face(ty, i, east) - face(ty, i, west))
			if cy < h-1 {
				s += south * face(tx, i, i+w)
			}
			if cy > 0 {
				s -= north * face(tx, i, i-w)
			}
			b[u] -= s / SeaDensity
		}
	}
	return b
}

// solve is x with A x = b, to flowSettled of b.
func (f *flow) solve(b []float64) []float64 {
	x, done, _ := gmres(b, f.apply, f.precondition, flowSettled, flowRestart, flowMost)
	flowIterations = done
	return x
}

// flowIterations is how many directions the last solve took: for the tests.
var flowIterations int

// chain is a line of cells and the operator between them, a band of two
// either side of the diagonal, factored for solving: L U without exchanging
// rows, which the band's diagonal, the friction's and the stirring's, is
// heavy enough to stand. A chain all the way round a parallel joins its ends,
// and that join is taken in after (Sherman and Morrison, 1950, in Woodbury's
// form for four).
type chain struct {
	cells      []int32
	l1, l2     []float64
	u0, u1, u2 []float64
	ring       *ringJoin
}

type ringJoin struct {
	corner [4][4]float64 // the join's coefficients between the ends' cells
	z      [4][]float64  // the chain's solve for each end cell
	cap    [4][4]float64 // I + corner·(the ends of z), inverted
}

// newChain factors the band a, a[2] the diagonal and a[d] the coefficient d-2
// cells along, over cells; ring joins the last cells to the first.
func newChain(cells []int32, a [5][]float64, ring bool) *chain {
	n := len(cells)
	slab := make([]float64, 5*n)
	c := &chain{
		cells: cells,
		l1:    slab[:n:n], l2: slab[n : 2*n : 2*n],
		u0: slab[2*n : 3*n : 3*n], u1: slab[3*n : 4*n : 4*n], u2: slab[4*n:],
	}
	for i := range n {
		var l1, l2 float64
		sub := a[1][i] // the coefficient one back, as elimination leaves it
		dia := a[2][i]
		if i >= 2 {
			l2 = a[0][i] / c.u0[i-2]
			sub -= l2 * c.u1[i-2]
			dia -= l2 * c.u2[i-2]
		}
		up := a[3][i]
		if i >= 1 {
			l1 = sub / c.u0[i-1]
			dia -= l1 * c.u1[i-1]
			up -= l1 * c.u2[i-1]
		}
		c.l1[i], c.l2[i] = l1, l2
		c.u0[i], c.u1[i], c.u2[i] = dia, up, a[4][i]
	}
	if i := n - 1; i >= 0 {
		c.u1[i], c.u2[i] = 0, 0
	}
	if n >= 2 {
		c.u2[n-2] = 0
	}
	if ring && n >= 5 {
		j := &ringJoin{}
		ends := [4]int{0, 1, n - 2, n - 1}
		where := func(p int) int {
			for k, e := range ends {
				if e == p {
					return k
				}
			}
			return -1
		}
		for k, p := range ends {
			for d := -2; d <= 2; d++ {
				q := p + d
				if q >= 0 && q < n {
					continue
				}
				q = (q + n) % n
				j.corner[k][where(q)] += a[d+2][p]
			}
		}
		for k, p := range ends {
			j.z[k] = make([]float64, n)
			j.z[k][p] = 1
			c.solveBand(j.z[k])
		}
		var m [4][4]float64
		for r := range 4 {
			for s := range 4 {
				var v float64
				for q := range 4 {
					v += j.corner[r][q] * j.z[s][ends[q]]
				}
				if r == s {
					v++
				}
				m[r][s] = v
			}
		}
		j.cap = invert4(m)
		c.ring = j
	}
	return c
}

// solve solves the chain for r in place.
func (c *chain) solve(r []float64) {
	r = r[:len(c.cells)]
	c.solveBand(r)
	if j := c.ring; j != nil {
		n := len(r)
		ends := [4]int{0, 1, n - 2, n - 1}
		var t, w [4]float64
		for k := range 4 {
			for q := range 4 {
				t[k] += j.corner[k][q] * r[ends[q]]
			}
		}
		for k := range 4 {
			for q := range 4 {
				w[k] += j.cap[k][q] * t[q]
			}
		}
		for k := range 4 {
			for p := range r {
				r[p] -= j.z[k][p] * w[k]
			}
		}
	}
}

// solveBand solves the band alone, without the ring's join, in place.
func (c *chain) solveBand(r []float64) {
	n := len(c.cells)
	for i := 1; i < n; i++ {
		r[i] -= c.l1[i] * r[i-1]
		if i >= 2 {
			r[i] -= c.l2[i] * r[i-2]
		}
	}
	for i := n - 1; i >= 0; i-- {
		s := r[i]
		if i+1 < n {
			s -= c.u1[i] * r[i+1]
		}
		if i+2 < n {
			s -= c.u2[i] * r[i+2]
		}
		r[i] = s / c.u0[i]
	}
}

// invert4 is the inverse of a four by four matrix, by Gauss and Jordan with
// the largest pivot in each column.
func invert4(m [4][4]float64) [4][4]float64 {
	var inv [4][4]float64
	for i := range 4 {
		inv[i][i] = 1
	}
	for col := range 4 {
		p := col
		for r := col + 1; r < 4; r++ {
			if math.Abs(m[r][col]) > math.Abs(m[p][col]) {
				p = r
			}
		}
		m[col], m[p] = m[p], m[col]
		inv[col], inv[p] = inv[p], inv[col]
		d := m[col][col]
		for s := range 4 {
			m[col][s] /= d
			inv[col][s] /= d
		}
		for r := range 4 {
			if r == col {
				continue
			}
			k := m[r][col]
			for s := range 4 {
				m[r][s] -= k * m[col][s]
				inv[r][s] -= k * inv[col][s]
			}
		}
	}
	return inv
}
