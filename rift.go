package terra

import (
	"math"
	"slices"
)

// How a plate that has grown too large tears in two.
//
// A continent does not part along a ruled line. It parts where it is weakest,
// and where it is weakest is where it was put together: the Atlantic opened
// along the sutures of the Appalachian and Caledonian ranges that had closed
// the ocean before it, and the Red Sea and the East African rift follow the
// Pan-African belts (Wilson 1966; Vauchez and others 1997). Between the old
// weaknesses it finds the next best thing, the lines the crust broke along
// when it went rigid, which are what the plates' own walls stand on. And the
// tear is not one line but a flight of steps: straight stretches of rift
// offset by transforms running the way the halves part, which is how the
// Equatorial Atlantic's margins still read a hundred million years on.
//
// So a rift is grown and not drawn. A staircase is laid across the plate,
// rifts square to the way the halves will part and transforms along it (a
// riftPlan), and the rift is the cheapest path across the plate from one
// edge to the other: cheap through an old suture, cheap along the weak
// lines, dear the further it strays from the staircase. What lies on the
// far side of it is the new plate.
//
// The floods it replaces, from two seeds across the plate, drew the line where two
// floods met, and two floods from two points at one rate meet on the line
// half-way between them: across a continent, ruler-straight, and every
// seaway a rift opened was a straight channel with parallel sides.

// riftPlan is a staircase a rift is laid along: the plate parts the way a
// says, and the rift runs square to it in steps, the transform offsets
// between them steps[k] of the plate's breadth along the rift.
type riftPlan struct {
	a     float64
	steps []float64
}

// The staircase. A rift is drawn in riftLeast to riftMost stretches, each
// offset from the last by up to riftOffset of its own length either way, and
// the path strays from it at a cost of riftHold for every riftWidth of its
// length it strays by, squared.
//
// riftWeak is how much a weak line eases the path, as the power of how dear
// the same line is for a plate's flood to cross (see flood): the fractures
// stand some e^6.5 over the ground round them for a flood, and a rift
// crosses them at e^-6.5·riftWeak the cost. riftSuture is how much cheaper an
// old suture is, at its fullest, as the power of e: a tile crushed by
// sutureFull metres of meeting plates or more is.
const (
	riftLeast  = 2
	riftMost   = 4
	riftOffset = 0.35
	riftWidth  = 0.08
	riftHold   = 1.0
	riftWeak   = 0.3
	riftSuture = 2.0
	sutureFull = 2000.0
)

// drawRift draws the staircase of a rift at a random bearing.
func (w *Land) drawRift() riftPlan {
	p := riftPlan{a: 2 * math.Pi * w.RNG.Float64()}
	n := riftLeast + w.RNG.IntN(riftMost-riftLeast+1)
	p.steps = make([]float64, n)
	at := 0.0
	for k := 1; k < n; k++ {
		at += riftOffset / float64(n) * 2 * (w.RNG.Float64() - 0.5)
		p.steps[k] = at
	}
	// About the plate's middle line, so the halves stay near even.
	mean := 0.0
	for _, s := range p.steps {
		mean += s
	}
	mean /= float64(n)
	for k := range p.steps {
		p.steps[k] -= mean
	}
	return p
}

// riftPath rifts plates of and to, which were one plate, along plan, and
// says what share of it the smaller half is. It fails where the plate is too
// small to cross or the path does not part it.
func (g *Grid) riftPath(fl *flooding, plates []Plate, book []record, of, to uint8, plan riftPlan) (float64, bool) {
	for i := range g.Tiles {
		if g.Tiles[i].Plate == to {
			g.Tiles[i].Plate = of
		}
	}
	ux, uy := math.Cos(plan.a), math.Sin(plan.a)
	// Where each tile of the plate lies across the rift (t, the way the
	// halves part) and along it (s), in tiles.
	var across, along func(i int) float64
	if g.Wrap {
		r := float64(g.W) / (2 * math.Pi)
		at, as := g.lineAcross(&plates[of], ux, uy), g.lineAcross(&plates[of], -uy, ux)
		across = func(i int) float64 { return r * at(i) }
		along = func(i int) float64 { return r * as(i) }
	} else {
		cx, cy := plates[of].cx, plates[of].cy
		across = func(i int) float64 { return (float64(i%g.W)-cx)*ux + (float64(i/g.W)-cy)*uy }
		along = func(i int) float64 { return -(float64(i%g.W)-cx)*uy + (float64(i/g.W)-cy)*ux }
	}
	tiles := make([]int32, 0, int(g.plateArea(of)))
	for i := range g.Tiles {
		if g.Tiles[i].Plate == of {
			tiles = append(tiles, int32(i))
		}
	}
	if len(tiles) < 16 {
		return 0, false
	}
	// Each tile of the plate's place in tiles, and -1 off the plate: what the
	// path keeps is kept by that place.
	at := make([]int32, len(g.Tiles))
	for i := range at {
		at[i] = -1
	}
	m := len(tiles)
	t, s := make([]float64, m), make([]float64, m)
	ts, ss := make([]float64, m), make([]float64, m)
	for k, i := range tiles {
		at[i] = int32(k)
		t[k], s[k] = across(int(i)), along(int(i))
		ts[k], ss[k] = t[k], s[k]
	}
	slices.Sort(ts)
	slices.Sort(ss)
	mid, lo, hi := ts[m/2], ss[0], ss[m-1]
	long := hi - lo
	if long < 4 {
		return 0, false
	}
	// The staircase: the line the rift is held to at each place along it,
	// and how far each tile of the plate lies off it.
	wide := math.Max(1.5, riftWidth*long)
	off := make([]float64, m)
	cost := make([]float64, m)
	for k, i := range tiles {
		step := int(float64(len(plan.steps)) * (s[k] - lo) / long)
		step = max(0, min(len(plan.steps)-1, step))
		off[k] = t[k] - (mid + plan.steps[step]*long)
		c := math.Pow(float64(fl.cost[i]), -riftWeak)
		if book != nil {
			c *= math.Exp(-riftSuture * math.Min(1, book[i].crush/sutureFull))
		}
		cost[k] = c * (1 + riftHold*(off[k]/wide)*(off[k]/wide))
	}
	// The ends: the edge of the plate within a corridor of the staircase,
	// below and above its middle along it.
	smid := ss[m/2]
	end := make([]bool, m)
	from := make([]int32, 0, m)
	ends := 0
	for k, i := range tiles {
		if math.Abs(off[k]) > 2*wide {
			continue
		}
		on := false
		g.eachNear(int(i), func(j int) { on = on || g.Tiles[j].Plate != of })
		if !on {
			continue
		}
		if s[k] < smid {
			from = append(from, int32(k))
		} else {
			end[k] = true
			ends++
		}
	}
	if len(from) == 0 || ends == 0 {
		return 0, false
	}
	// The cheapest path from one end to the other, over the plate.
	dist := make([]float64, m)
	back := make([]int32, m)
	for k := range dist {
		dist[k], back[k] = math.Inf(1), -1
	}
	q := make(riftHeap, 0, m)
	for _, k := range from {
		dist[k] = 0
		q.push(riftNode{0, k})
	}
	reached := int32(-1)
	for len(q) > 0 {
		n := q.pop()
		if n.d > dist[n.i] {
			continue
		}
		if end[n.i] {
			reached = n.i
			break
		}
		i := int(tiles[n.i])
		x, y := i%g.W, i/g.W
		for _, dir := range Dirs {
			qx, qy := x+dir.X, y+dir.Y
			if qy < 0 || qy >= g.H {
				continue
			}
			if qx < 0 || qx >= g.W {
				if !g.Wrap {
					continue
				}
				qx = g.WrapX(qx)
			}
			k := at[qy*g.W+qx]
			if k < 0 {
				continue
			}
			l := 1.0
			if dir.X != 0 && dir.Y != 0 {
				l = math.Sqrt2
			}
			if nd := n.d + l*(cost[n.i]+cost[k])/2; nd < dist[k] {
				dist[k], back[k] = nd, n.i
				q.push(riftNode{nd, k})
			}
		}
	}
	if reached < 0 {
		return 0, false
	}
	path := end // the ends are spent; the space is reused for the path
	clear(path)
	for k := reached; k >= 0; k = back[k] {
		path[k] = true
	}
	// The far side of the path, the way the halves part, is the new plate:
	// a flood through the sides of tiles only, which a path that steps
	// across corners still holds.
	far, near := -1, -1
	for k := range tiles {
		if path[k] {
			continue
		}
		if far < 0 || off[k] > off[far] {
			far = k
		}
		if near < 0 || off[k] < off[near] {
			near = k
		}
	}
	if far < 0 {
		return 0, false
	}
	side := make([]int32, 1, m)
	side[0] = tiles[far]
	g.Tiles[tiles[far]].Plate = to
	four := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for len(side) > 0 {
		i := side[len(side)-1]
		side = side[:len(side)-1]
		x, y := int(i)%g.W, int(i)/g.W
		for _, d := range four {
			qx, qy := x+d[0], y+d[1]
			if qy < 0 || qy >= g.H {
				continue
			}
			if qx < 0 || qx >= g.W {
				if !g.Wrap {
					continue
				}
				qx = g.WrapX(qx)
			}
			j := qy*g.W + qx
			if k := at[j]; k >= 0 && !path[k] && g.Tiles[j].Plate == of {
				g.Tiles[j].Plate = to
				side = append(side, int32(j))
			}
		}
	}
	if g.Tiles[tiles[near]].Plate == to {
		// The path went round and did not part the plate.
		for _, i := range tiles {
			g.Tiles[i].Plate = of
		}
		return 0, false
	}
	g.onePiece(tiles, at, of, to)
	return g.riftShare(of, to), true
}

// onePiece gives every piece of plate of but its largest to plate to, which
// it borders: a rift's near side is what the far side's flood left, and a
// plate that wrapped round a neighbour can leave more than one piece of it.
// tiles are the plate's tiles before the rift, and at each tile's place in
// them.
func (g *Grid) onePiece(tiles, at []int32, of, to uint8) {
	label := make([]int32, len(tiles))
	for k := range label {
		label[k] = -1
	}
	var sizes []int
	stack := make([]int32, 0, len(tiles))
	for s := range tiles {
		if g.Tiles[tiles[s]].Plate != of || label[s] >= 0 {
			continue
		}
		id := int32(len(sizes))
		sizes = append(sizes, 0)
		label[s] = id
		stack = append(stack[:0], int32(s))
		for len(stack) > 0 {
			k := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			sizes[id]++
			g.eachNear(int(tiles[k]), func(j int) {
				if q := at[j]; q >= 0 && label[q] < 0 && g.Tiles[j].Plate == of {
					label[q] = id
					stack = append(stack, q)
				}
			})
		}
	}
	if len(sizes) < 2 {
		return
	}
	big := int32(0)
	for k, n := range sizes {
		if n > sizes[big] {
			big = int32(k)
		}
	}
	for k, id := range label {
		if id >= 0 && id != big {
			g.Tiles[tiles[k]].Plate = to
		}
	}
}

type riftNode struct {
	d float64
	i int32
}

// riftHeap is a binary heap of the path's frontier, least first, ties by
// tile so that a world repeats.
type riftHeap []riftNode

func (h riftHeap) less(a, b int) bool {
	return h[a].d < h[b].d || h[a].d == h[b].d && h[a].i < h[b].i
}

func (h *riftHeap) push(n riftNode) {
	*h = append(*h, n)
	k := len(*h) - 1
	for k > 0 {
		p := (k - 1) / 2
		if !h.less(k, p) {
			break
		}
		(*h)[k], (*h)[p] = (*h)[p], (*h)[k]
		k = p
	}
}

func (h *riftHeap) pop() riftNode {
	old := *h
	top := old[0]
	last := len(old) - 1
	old[0] = old[last]
	*h = old[:last]
	k := 0
	for {
		l, r := 2*k+1, 2*k+2
		m := k
		if l < last && h.less(l, m) {
			m = l
		}
		if r < last && h.less(r, m) {
			m = r
		}
		if m == k {
			break
		}
		(*h)[k], (*h)[m] = (*h)[m], (*h)[k]
		k = m
	}
	return top
}
