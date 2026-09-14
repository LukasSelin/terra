package terra

import (
	"math"
	"slices"

	"github.com/LukasSelin/terra/geom"
)

// Shaping: the ground as the water would have left it.
//
// A drawn map is noise, and a made one is its history's heights handed the
// drawn map's spread by rank. Neither was ever worn by water, and the water on
// them showed it when it was held against real ground - see yardstick_test.go.
// Its basins came out short, wide and too many, P(A>=a) falling as a^-0.8
// where real networks fall as a^-0.43, and mainstreams as area^0.34 where
// Hack found area^0.57. The land sat low under its peaks - a Strahler integral
// of 0.2 on the valley and 0.1 on a small globe, against the 0.35 to 0.6 of
// ground that has been worn to equilibrium - and the only regular spacing of
// its ridges was the noise's own, 320 metres apart, where Perron and others
// find first-order valleys 30 to 160.
//
// So the ground is laid again as a landscape wearing at the rate it rises
// would lay it. The map as it came is the uplift: high ground rises fastest,
// the lowland at shapeFloor of that. Every tile sends its water down the
// steepest fall, and stands above the tile it drains to by the fall stream
// power holds a channel at in steady state, which eases with the ground the
// channel drains as area^-shapeConcave (Whipple and Tucker 1999; the concavity
// of real rivers is 0.4 to 0.6). Walked from the sea and the map's edges
// upward that is every height on the map; taken a few times over, each time
// down the falls the last one left, the drainage finds the network the heights
// agree with. Before the first, each tile is given a hand's breadth of
// roughness: on the drawn map's smooth swells the water otherwise ran down
// parallel lines that never met.
//
// The result is put back onto the map's own scale, shapeTop of the height its
// ground stood to above the water it drains to, and lifted at the middle -
// 1-(1-x)^shapeLift of the way up - which is the higher, steeper lowland a
// worn country has and the drawn one did not.
//
// What drains nowhere is left as it was. A hollow whose open water gives the
// air more than its whole catchment sends it is a closed basin, and a closed
// basin is not graded to the sea: its ground is not shaped, and it keeps its
// salt lake. Everywhere wetter the hollows fill and spill, and join the rivers.
//
// The constants were searched for over five valleys and eight small globes at
// seeds 1 and 6, scored on every yardstick the ground and the water answer to
// and on the dry country keeping its salt. They sit close together: rounded to
// three figures, the small globes' discharge exponent went from inside the
// real range to 0.506, which is why they are written out to the figure the
// search found. Of the yardsticks, only the small globes' hypsometric integral
// at seed 6 falls outside, at 0.338.
const (
	shapeFloor   = 0.6473611255017592 // the least uplift, against the most
	shapeRounds  = 3
	shapeFall    = 0.8 // the fall at a channel head, before uplift and scale
	shapeHead    = 4.0 // tiles a channel head drains
	shapeOcean   = 1   // tiles a body of sea has to be for the ground to be graded to it
	shapeLift    = 2.6523767491920225
	shapeRough   = 1.2763006735954712 // metres of roughness each tile is given before the water is routed
	shapeTop     = 0.5233050488643692
	shapeConcave = 0.5119282740307869
)

// shape lays the ground again as the water would have left it, and returns how
// many tiles each tile drains, by the drainage it was laid down.
func (g *Grid) shape() (area []float64) {
	n := len(g.Tiles)
	root := make([]bool, n)
	for i := range g.Tiles {
		if p := g.PosOf(i); g.outlet(p.X, p.Y) {
			root[i] = true
		}
	}
	g.openSea(root)
	g.closedBasins(root)

	lo, hi := math.Inf(1), math.Inf(-1)
	for i := range g.Tiles {
		if !root[i] {
			h := g.Tiles[i].Height
			lo, hi = math.Min(lo, h), math.Max(hi, h)
		}
	}
	if !(hi > lo) {
		return nil
	}
	if g.sea >= 0 {
		lo = math.Max(lo, g.sea)
	}
	uplift := make([]float64, n)
	h := make([]float64, n)
	for i := range g.Tiles {
		h[i] = g.Tiles[i].Height
		if !root[i] {
			h[i] += shapeRough * roughAt(i, h[i])
		}
		uplift[i] = shapeFloor + (1-shapeFloor)*clamp01((h[i]-lo)/(hi-lo))
	}
	g.fillFrom(h, root)

	recv := make([]int32, n)
	run := make([]float64, n)
	area = make([]float64, n)
	base := make([]float64, n) // the height of the water each tile drains to
	for round := 0; round < shapeRounds; round++ {
		for i := range recv {
			recv[i], run[i] = int32(i), TileSpan
			if root[i] {
				continue
			}
			p := g.PosOf(i)
			steepest := 0.0
			for _, off := range Dirs {
				q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
				if !g.In(q) {
					continue
				}
				j := g.Index(q)
				d := TileSpan
				if off.X != 0 && off.Y != 0 {
					d *= math.Sqrt2
				}
				if fall := (h[i] - h[j]) / d; fall > steepest {
					recv[i], run[i], steepest = int32(j), d, fall
				}
			}
		}
		stack := stackOf(recv)
		for i := range area {
			area[i] = 1
		}
		for k := len(stack) - 1; k >= 0; k-- {
			if i := stack[k]; recv[i] != i {
				area[recv[i]] += area[i]
			}
		}
		for _, i := range stack {
			r := recv[i]
			if r == i {
				base[i] = h[i]
				continue
			}
			gathered := math.Max(area[i], shapeHead) / shapeHead
			fall := shapeFall / math.Sqrt(shapeHead) * uplift[i] * math.Pow(gathered, -shapeConcave)
			h[i] = h[r] + math.Min(Repose, fall)*run[i]
			base[i] = base[r]
		}
	}

	top, most := math.Inf(-1), math.Inf(-1)
	for i := range g.Tiles {
		if !root[i] {
			top = math.Max(top, h[i]-base[i])
			most = math.Max(most, g.Tiles[i].Height-base[i])
		}
	}
	if !(top > 0 && most > 0) {
		return area
	}
	for i := range g.Tiles {
		if root[i] {
			continue
		}
		x := clamp01((h[i] - base[i]) / top)
		g.Tiles[i].Height = base[i] + shapeTop*most*(1-math.Pow(1-x, shapeLift))
	}
	return area
}

// hollowDeep and hollowLeast are how deep, in metres, and how broad, in
// tiles, a hollow has to be to be asked whether it is a closed basin.
const (
	hollowDeep  = 0.3
	hollowLeast = 8
)

// closedBasins roots every tile of every closed basin: a hollow, and all the
// ground whose water runs into it, where the air takes off the hollow at least
// what the ground sends it.
func (g *Grid) closedBasins(root []bool) {
	n := len(g.Tiles)
	g.weather()
	if len(g.rain) != n {
		return
	}
	h := make([]float64, n)
	for i := range g.Tiles {
		h[i] = g.Tiles[i].Height
	}
	filled := slices.Clone(h)
	g.fillFrom(filled, slices.Clone(root))
	in := make([]bool, n)
	for i := range h {
		in[i] = !root[i] && filled[i]-h[i] > 0.01
	}
	// Each hollow deep and broad enough, numbered from one.
	hollow := make([]int32, n)
	seen := make([]bool, n)
	count := int32(0)
	for i := range h {
		if !in[i] || seen[i] {
			continue
		}
		body := []int32{int32(i)}
		seen[i] = true
		deepest := 0.0
		for k := 0; k < len(body); k++ {
			j := body[k]
			deepest = math.Max(deepest, filled[j]-h[j])
			p := g.PosOf(int(j))
			for _, off := range Dirs {
				q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
				if !g.In(q) {
					continue
				}
				if m := g.Index(q); in[m] && !seen[m] {
					seen[m] = true
					body = append(body, int32(m))
				}
			}
		}
		if deepest < hollowDeep || len(body) < hollowLeast {
			continue
		}
		count++
		for _, j := range body {
			hollow[j] = count
		}
	}
	if count == 0 {
		return
	}
	// Where each tile's water ends, walked from the lowest ground up.
	ends := make([]int32, n)
	order := make([]int32, n)
	for i := range order {
		order[i] = int32(i)
	}
	slices.SortFunc(order, func(a, b int32) int {
		if h[a] != h[b] {
			if h[a] < h[b] {
				return -1
			}
			return 1
		}
		return int(a - b) // ties by position, so a world repeats
	})
	for _, i := range order {
		switch {
		case hollow[i] > 0:
			ends[i] = hollow[i]
		case root[i]:
		default:
			p := g.PosOf(int(i))
			if a := g.Aspect(p); a != (geom.Pos{}) {
				if q := (geom.Pos{X: p.X + a.X, Y: p.Y + a.Y}); g.In(q) {
					ends[i] = ends[g.Index(q)]
				}
			}
		}
	}
	sent, taken := make([]float64, count+1), make([]float64, count+1)
	for i, k := range ends {
		if k == 0 {
			continue
		}
		sent[k] += g.runoff[i]
		if hollow[i] > 0 {
			taken[k] += g.loss(i) + g.runoff[i]
		}
	}
	for i, k := range ends {
		if k > 0 && taken[k] >= sent[k] {
			root[i] = true
		}
	}
}

// openSea roots the tiles of every body of sea of at least shapeOcean tiles.
func (g *Grid) openSea(root []bool) {
	n := len(g.Tiles)
	seen := make([]bool, n)
	var body []int32
	for i := range g.Tiles {
		if seen[i] || !g.underSea(i) {
			continue
		}
		body = append(body[:0], int32(i))
		seen[i] = true
		for k := 0; k < len(body); k++ {
			p := g.PosOf(int(body[k]))
			for _, off := range Dirs {
				q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
				if !g.In(q) {
					continue
				}
				if j := g.Index(q); !seen[j] && g.underSea(j) {
					seen[j] = true
					body = append(body, int32(j))
				}
			}
		}
		if len(body) >= shapeOcean {
			for _, j := range body {
				root[j] = true
			}
		}
	}
}

// fillFrom raises every hollow in h to a hair above where it spills, from the
// roots inward, so that every tile has somewhere lower to send its water.
func (g *Grid) fillFrom(h []float64, root []bool) {
	const hair = 1e-4
	done := make([]bool, len(h))
	var q slideQueue
	roots := 0
	for i := range h {
		if root[i] {
			q.push(h[i], int32(i))
			roots++
		}
	}
	if roots == 0 {
		// Nowhere to drain to: the lowest tile is where it all goes.
		k := slices.Index(h, slices.Min(h))
		root[k] = true
		q.push(h[k], int32(k))
	}
	for len(q.at) > 0 {
		i := q.pop()
		if done[i] {
			continue
		}
		done[i] = true
		p := g.PosOf(int(i))
		for _, off := range Dirs {
			nb := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
			if !g.In(nb) {
				continue
			}
			j := int32(g.Index(nb))
			if done[j] || root[j] {
				continue
			}
			h[j] = math.Max(h[j], h[i]+hair)
			q.push(h[j], j)
		}
	}
}

// roughAt is a number in [0,1) fixed by the tile and its height, and by
// nothing drawn from the world's chance: shaping draws nothing, so a seed
// means what it meant.
func roughAt(i int, h float64) float64 {
	x := uint64(i)*0x9E3779B97F4A7C15 ^ math.Float64bits(h)
	x ^= x >> 30
	x *= 0xBF58476D1CE4E5B9
	x ^= x >> 27
	x *= 0x94D049BB133111EB
	x ^= x >> 31
	return float64(x>>11) / (1 << 53)
}

// The first-order valleys.
//
// Shaped, the ground's spectrum still stood highest at the size of its basins:
// a valley every 320 metres on the default map, where Perron, Dietrich and
// Kirchner (2008) find first-order valleys 30 metres apart at the Dragon's Back
// and 160 at Gabilan Mesa. At 25 metres a tile those are one to six tiles, and
// shaping lays every tile as a channel, so the spacing has to be drawn.
//
// It is drawn as what it is: valleys at one spacing, running down the ground.
// Each patch of the map is given a wave across the way the ground falls there,
// with its troughs down the fall, and the patches blend. The phase of each wave
// is taken from where the tile is and not drawn apart for each patch, so that
// neighbouring patches that face the same way are the same valleys: with a
// phase of its own, the patches cancelled one another and the spacing spread
// into a hump the size of the basins drowned. It is cut in proportion to how
// steep the ground is, up to textureSteep, because a plain has no valleys, and
// not at all on a river, whose course is its own.
//
// It is kept shallower than the fall of the slope it is cut into, so that the
// water on a hillside still goes down the hill and not along a trough: cut any
// deeper, every hillside drained down parallel gullies and the basins came
// apart into strips.
const (
	textureDepth   = 4.570010309744883 // metres either side of the ground, on ground at textureSteep
	textureSpacing = 135.0726952692895 // metres from valley to valley: 5.4029 tiles as searched
	textureReach   = 2.571232958955619 // how far each patch's waves carry, in patches
	textureChannel = 64.0              // tiles of drainage above which a tile is a river's and is left alone
	textureSteep   = 0.3
)

// texture cuts the first-order valleys into the ground. area is what shape
// found each tile drains.
func (w *Land) texture(g *Grid, area []float64) {
	n := len(g.Tiles)
	lie := make([]float64, n)
	for i := range g.Tiles {
		lie[i] = g.Tiles[i].Height
	}
	for k := 0; k < 3; k++ {
		lie = g.spread(lie)
	}
	span := tilesAcross(textureSpacing, TileSpan) // tiles from valley to valley
	cell := int(math.Max(2, math.Round(span)))
	sigma := textureReach * float64(cell)
	reach := int(math.Ceil(2 * textureReach))
	cols, rows := (g.W+cell-1)/cell, (g.H+cell-1)/cell
	type wave struct{ nx, ny float64 }
	waves := make([]wave, cols*rows)
	for cy := 0; cy < rows; cy++ {
		for cx := 0; cx < cols; cx++ {
			x, y := min(g.W-1, cx*cell+cell/2), min(g.H-1, cy*cell+cell/2)
			gx := g.lieAt(lie, x+1, y) - g.lieAt(lie, x-1, y)
			gy := g.lieAt(lie, x, y+1) - g.lieAt(lie, x, y-1)
			// Across the fall, so that the troughs run down it.
			nx, ny := -gy, gx
			if l := math.Hypot(nx, ny); l > 1e-9 {
				nx, ny = nx/l, ny/l
			} else {
				a := 2 * math.Pi * roughAt(cy*cols+cx+n, 1)
				nx, ny = math.Cos(a), math.Sin(a)
			}
			waves[cy*cols+cx] = wave{nx, ny}
		}
	}
	cut := make([]float64, n)
	g.EachRow(func(y int) {
		for x := 0; x < g.W; x++ {
			i := y*g.W + x
			if g.underSea(i) {
				continue
			}
			depth := textureDepth * clamp01(g.Slope(g.PosOf(i))/textureSteep)
			if area != nil {
				depth *= clamp01((textureChannel - area[i]) / (textureChannel - shapeHead))
			}
			if depth <= 0 {
				continue
			}
			sum, weight := 0.0, 0.0
			cx0, cy0 := x/cell, y/cell
			for dy := -reach; dy <= reach; dy++ {
				for dx := -reach; dx <= reach; dx++ {
					cy, cx := cy0+dy, cx0+dx
					if cy < 0 || cy >= rows {
						continue
					}
					ox := float64(x - (cx*cell + cell/2))
					if g.Wrap {
						cx = (cx%cols + cols) % cols
					} else if cx < 0 || cx >= cols {
						continue
					}
					oy := float64(y - (cy*cell + cell/2))
					k := waves[cy*cols+cx]
					wgt := math.Exp(-(ox*ox + oy*oy) / (2 * sigma * sigma))
					sum += wgt * math.Cos(2*math.Pi*(float64(x)*k.nx+float64(y)*k.ny)/span)
					weight += wgt * wgt
				}
			}
			cut[i] = depth * sum / math.Sqrt(math.Max(weight, 1e-12))
		}
	})
	for i := range g.Tiles {
		g.Tiles[i].Height -= cut[i]
	}
}

// lieAt is the broad lie of the ground at x, y, held to the map's edge.
func (g *Grid) lieAt(lie []float64, x, y int) float64 {
	p := geom.Pos{X: x, Y: y}
	if !g.In(p) {
		p = geom.Pos{X: min(max(x, 0), g.W-1), Y: min(max(y, 0), g.H-1)}
	}
	return lie[g.Index(p)]
}
