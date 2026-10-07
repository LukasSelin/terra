package terra

import (
	"math"
	"slices"

	"github.com/LukasSelin/terra/geom"
	"github.com/LukasSelin/terra/internal/atmos"
	"github.com/LukasSelin/terra/internal/phase"
)

// The sea's features: its currents, its gyres, and where its water comes up.
//
// The weather works out the sea's current, its upwelling and its water's
// warmth on the air's cells (atmos/ocean.go), and the land reads them tile by
// tile (sky.go). What it had nothing of was anything to point at: the Gulf
// Stream was a run of fast warm water in a field. Here the field is joined
// into features as the ground is into belts and basins, tile by tile in tile
// order, so that the same seed gives the same ids however many goroutines
// made the world.
//
// It is read from the current alone - where the water runs fast and the same
// way, where it goes round, where it comes up - and from nothing about how
// the current was worked out: a gyre is a hill or a hollow of the
// streamfunction the current makes (atmos/stream.go), not the rows the
// weather solves the gyres along, and a western boundary current is fast
// water running along a shore with the land to its west, wherever that is.
// So the features stay what they are when the current is worked out
// otherwise.

// SeaClass is what kind of current, or of gyre, a feature of the sea is.
type SeaClass uint8

const (
	NoSeaClass SeaClass = iota
	// WesternBoundary is a current running along an ocean's western shore:
	// the Gulf Stream and the Kuroshio up their oceans warm, the Labrador
	// and the Oyashio down them cold.
	WesternBoundary
	// EasternBoundary is a current running along an ocean's eastern shore:
	// the Canary and the California, coming down cold.
	EasternBoundary
	// Drift is a current across open water: the North Atlantic Drift, the
	// equatorial currents of the trades.
	Drift
	// Equatorial, Circumpolar and Throughflow are a current along the
	// equator, one all the way round the planet, and one between islands
	// from one ocean to the next. A drift that goes all the way round is
	// circumpolar; the others are for a current worked out in two
	// dimensions to give.
	Equatorial
	Circumpolar
	Throughflow
	// Subtropical, Subpolar and Tropical are a gyre's: one turning
	// clockwise in the north and anticlockwise in the south between the
	// trades and the westerlies, one turning the other way poleward of it,
	// and one either way nearer the equator than either (see tropicEdge).
	Subtropical
	Subpolar
	Tropical
	seaClasses
)

var seaClassNames = [seaClasses]string{"none", "western boundary", "eastern boundary", "drift",
	"equatorial", "circumpolar", "throughflow", "subtropical", "subpolar", "tropical"}

func (c SeaClass) String() string {
	if int(c) < len(seaClassNames) {
		return seaClassNames[c]
	}
	return "sea class?"
}

const (
	// currentFloor is how fast, in metres a second, water has to run to be
	// part of a current: a tenth of a metre a second is a slow drift's
	// speed, and the Gulf Stream runs at one and more.
	currentFloor = 0.1
	// eastFloor is the same for a current along an ocean's eastern shore.
	// What a gyre drives toward the equator across the breadth of an ocean
	// comes back toward the pole narrow and fast against its western shore,
	// and the eastern side of the gyre is the broad slow drift of the rest
	// (Stommel, 1948): the Canary current runs at a tenth of the Gulf
	// Stream's speed, and the weather's eastern currents at a hundredth or
	// two of a metre a second.
	eastFloor = 0.01
	// currentTurn is how many degrees two neighbouring tiles' currents may
	// run apart and still be one current.
	currentTurn = 45.0
	// currentLeast is the least sea, in square kilometres, a current is: a
	// run of fast water a hundred kilometres across and three hundred long.
	currentLeast = 3e4
	// shoreReach is how far, in kilometres, along a parallel from a shore a
	// current running north or south is a boundary current of the ocean on
	// that side.
	shoreReach = 500.0
	// gyreFloor is how much water, in sverdrups, has to go round between a
	// tile and the shore for the tile to be in a gyre: the hill of the
	// streamfunction is cut off at this height. A subtropical gyre carries
	// some thirty to a hundred and fifty.
	gyreFloor = 5.0
	// gyreLeast is the least sea, in square kilometres, a gyre is: some
	// thousand kilometres across.
	gyreLeast = 1e6
	// gyreReach is how far, in kilometres, along a parallel from a gyre a
	// current outside it can be one of its currents.
	gyreReach = 1000.0
	// tropicEdge and subpolarEdge are how far from the equator, in degrees,
	// a gyre's centre has to lie for it to be subtropical or subpolar: the
	// real ones turn round some thirty degrees and some fifty-five. Nearer
	// the equator it is tropical, whichever way it turns: the water the
	// trades and the drift along the equator turn between them.
	tropicEdge   = 15.0
	subpolarEdge = 35.0
	// riseFloor is how fast, in metres a second, water has to come up for a
	// coast to be upwelling: some forty centimetres a day. Off Peru and
	// Namibia it comes up at a metre a day and more.
	riseFloor = 5e-6
	// upwellLeast is the least sea, in square kilometres, an upwelling is:
	// a run of coast some three hundred kilometres long.
	upwellLeast = 2e4
)

// seaPlaces is the most features of one kind of the sea's a map keeps: a
// tile keeps its place among them in a uint16. The least sizes above keep
// the planet's sea to some thousands of each.
const seaPlaces = math.MaxUint16

// seaLabel is the tiles' places among the features of the sea's kind k.
func (f *Features) seaLabel(k FeatureKind) []uint16 {
	switch k {
	case SeaCurrent:
		return f.current
	case Gyre:
		return f.gyre
	case Upwelling:
		return f.upwell
	}
	return nil
}

// eachSea visits every tile of every feature of the sea, kind by kind and in
// tile order.
func (f *Features) eachSea(visit func(id FeatureID, i int)) {
	for k := SeaCurrent; k <= Upwelling; k++ {
		base := f.seaBase[k-SeaCurrent]
		for i, p := range f.seaLabel(k) {
			if p > 0 {
				visit(base+FeatureID(p), i)
			}
		}
	}
}

// seaRun is what a run of tiles seaRuns joined comes to: its lowest tile, its
// sea in square kilometres, and the columns it reaches west and east, laid
// out flat from its lowest tile.
type seaRun struct {
	first      int32
	area       float64
	west, east int32
}

// seaRuns labels the tiles in says are anything with the connected runs of
// them that join says go together, eight ways round and round the seam, in
// tile order, so that run k (from 1) is the k-th by its lowest tile. Into run
// it writes each tile's run, and into round how many times the way from the
// run's lowest tile to the tile crossed the seam eastward less westward, so
// that a run can be laid out flat. It gives back what each run comes to.
func (g *Grid) seaRuns(run []int32, round []int8, stack []int32,
	in func(i int) bool, join func(i, j int) bool) ([]seaRun, []int32) {
	clear(run)
	clear(round)
	var runs []seaRun
	for i := range g.Tiles {
		if run[i] != 0 || !in(i) {
			continue
		}
		runs = append(runs, seaRun{first: int32(i)})
		id := int32(len(runs))
		run[i] = id
		stack = append(stack[:0], int32(i))
		for len(stack) > 0 {
			j := int(stack[len(stack)-1])
			stack = stack[:len(stack)-1]
			p := g.PosOf(j)
			for _, off := range Dirs {
				q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
				r := round[j]
				if g.Wrap {
					switch {
					case q.X < 0:
						r--
					case q.X >= g.W:
						r++
					}
					q = g.Norm(q)
				}
				if !g.In(q) {
					continue
				}
				m := g.Index(q)
				if run[m] != 0 || !in(m) || !join(j, m) {
					continue
				}
				run[m], round[m] = id, r
				stack = append(stack, int32(m))
			}
		}
	}
	for k := range runs {
		r := &runs[k]
		x := r.first % int32(g.W)
		r.west, r.east = x, x
	}
	for i, id := range run {
		if id == 0 {
			continue
		}
		r := &runs[id-1]
		y := i / g.W
		r.area += g.air.Dx[y] * g.air.Dy
		x := int32(i%g.W) + int32(round[i])*int32(g.W)
		r.west, r.east = min(r.west, x), max(r.east, x)
	}
	return runs, stack
}

// roundMap reports whether a run goes all the way round the map.
func (g *Grid) roundMap(r seaRun) bool { return g.Wrap && int(r.east-r.west) >= g.W-1 }

// keepRuns gives each run its place among the features made of them, from 1,
// for those keep says are features, in their order, and 0 for the rest; and
// labels each tile of a kept run with its place. It gives back how many it
// kept.
func keepRuns(runs []seaRun, run []int32, label []uint16, keep func(r seaRun) bool) ([]uint16, int) {
	place := make([]uint16, len(runs)+1)
	kept := 0
	for k, r := range runs {
		if kept < seaPlaces && keep(r) {
			kept++
			place[k+1] = uint16(kept)
		}
	}
	for i, id := range run {
		label[i] = place[id]
	}
	return place, kept
}

// readSeaFeatures reads the sea's features off the currents of the weather w and adds
// them to all: the currents, then the gyres, then the upwellings, each kind in
// the order of its lowest tile. A map with no currents worked out - a valley,
// or one whose weather has not been read - has none.
func (g *Grid) readSeaFeatures(f *Features, all []Feature, stack []int32, w *Winds) ([]Feature, []int32) {
	n := len(g.Tiles)
	if w == nil || w.Cu == nil || w.W*w.Cell != g.W || w.H*w.Cell != g.H {
		return all, stack
	}
	defer phase.Start("readSeaFeatures")()

	// The sea under each tile - not a lake, whatever its height - and the
	// current over it, read between the air's cells; and the water going
	// round between it and the shore, in sverdrups.
	sea := make([]bool, n)
	u, v, psi := make([]float32, n), make([]float32, n), make([]float32, n)
	layer := make([]float32, n)
	stream := w.Stream()
	for i := range n {
		if !g.underSea(i) || (len(g.lakeOf) == n && g.lakeOf[i] >= 0) {
			continue
		}
		sea[i] = true
		fx, fy := w.CellAt(i)
		u[i], v[i] = float32(w.Sample32(w.Cu, fx, fy)), float32(w.Sample32(w.Cv, fx, fy))
		psi[i] = float32(w.Sample(stream, fx, fy)) // sverdrups: see atmos.Env.Stream
		layer[i] = float32(math.Max(atmos.FlowLeast, w.ThermoclineAt(i)))
	}
	run, round := make([]int32, n), make([]int8, n)
	f.current, f.gyre, f.upwell = make([]uint16, n), make([]uint16, n), make([]uint16, n)

	// The gyres: the sea where more than gyreFloor goes round one way
	// between the tile and the shore, joined where it goes round the same
	// way. A run of it that goes all the way round the planet goes round no
	// centre, and is no gyre.
	var runs []seaRun
	runs, stack = g.seaRuns(run, round, stack,
		func(i int) bool { return sea[i] && math.Abs(float64(psi[i])) >= gyreFloor },
		func(i, j int) bool { return (psi[i] > 0) == (psi[j] > 0) })
	gyrePlace, gyres := keepRuns(runs, run, f.gyre, func(r seaRun) bool { return r.area >= gyreLeast && !g.roundMap(r) })
	gyreFeatures := make([]Feature, gyres)
	gyreWarm := make([]float64, gyres)
	for k, r := range runs {
		if p := gyrePlace[k+1]; p > 0 {
			gyreFeatures[p-1] = Feature{Kind: Gyre, First: r.first, Centre: r.first}
		}
	}
	for i, p := range f.gyre {
		if p == 0 {
			continue
		}
		fe := &gyreFeatures[p-1]
		if math.Abs(float64(psi[i])) > math.Abs(float64(psi[fe.Centre])) {
			fe.Centre = int32(i)
		}
		gyreWarm[p-1] += w.WaterWarmth(i)
		fe.Count++
	}
	for k := range gyreFeatures {
		fe := &gyreFeatures[k]
		c := psi[fe.Centre]
		// A hill of ψ is turned round clockwise.
		fe.Sense = 1
		if c > 0 {
			fe.Sense = -1
		}
		fe.Transport = float32(math.Abs(float64(c)))
		fe.Warmth = float32(gyreWarm[k] / float64(fe.Count))
		lat := g.air.Lat[int(fe.Centre)/g.W]
		// Clockwise in the north and anticlockwise in the south is the
		// way the trades and the westerlies turn the water between them.
		anticyclonic := (fe.Sense < 0) == (lat > 0)
		switch {
		case anticyclonic && math.Abs(lat) >= tropicEdge:
			fe.Class = Subtropical
		case !anticyclonic && math.Abs(lat) >= subpolarEdge:
			fe.Class = Subpolar
		default:
			fe.Class = Tropical
		}
	}

	// The currents: the sea where the water runs faster than currentFloor,
	// or than eastFloor along an eastern shore, each tile's of a class,
	// joined where the class is the same and the water runs within
	// currentTurn of the same way.
	class := make([]SeaClass, n)
	for i := range n {
		if !sea[i] {
			continue
		}
		e, no := float64(u[i]), float64(v[i])
		speed := math.Hypot(e, no)
		if speed < eastFloor {
			continue
		}
		c := Drift
		if math.Abs(no) > math.Abs(e) {
			// Running north or south is running along a shore that runs
			// north and south, if there is one near: the ocean's western
			// boundary if the land is to the west, its eastern if to the
			// east.
			west, east := g.toShore(i, -1, sea), g.toShore(i, 1, sea)
			switch {
			case west <= shoreReach && west <= east:
				c = WesternBoundary
			case east <= shoreReach:
				c = EasternBoundary
			}
		}
		if c == EasternBoundary || speed >= currentFloor {
			class[i] = c
		}
	}
	turn := math.Cos(currentTurn * math.Pi / 180)
	runs, stack = g.seaRuns(run, round, stack,
		func(i int) bool { return class[i] != NoSeaClass },
		func(i, j int) bool {
			if class[i] != class[j] {
				return false
			}
			a, b := float64(u[i])*float64(u[j])+float64(v[i])*float64(v[j]),
				math.Hypot(float64(u[i]), float64(v[i]))*math.Hypot(float64(u[j]), float64(v[j]))
			return a >= turn*b
		})
	currentPlace, currents := keepRuns(runs, run, f.current, func(r seaRun) bool { return r.area >= currentLeast })
	currentFeatures := make([]Feature, currents)
	for k, r := range runs {
		if p := currentPlace[k+1]; p > 0 {
			fe := &currentFeatures[p-1]
			*fe = Feature{Kind: SeaCurrent, First: r.first, Class: class[r.first]}
			if fe.Class == Drift && g.roundMap(r) {
				fe.Class = Circumpolar
			}
		}
	}
	// Each current's tiles, for its path, its heading and its gyre.
	head := make([]int32, currents+2)
	for _, p := range f.current {
		if p > 0 {
			head[p+1]++
		}
	}
	for k := 1; k < len(head); k++ {
		head[k] += head[k-1]
	}
	member := make([]int32, head[len(head)-1])
	at := slices.Clone(head[:currents+1])
	for i, p := range f.current {
		if p > 0 {
			member[at[p]] = int32(i)
			at[p]++
		}
	}
	votes := make([]int32, gyres+1)
	var lay layScratch
	var paths []int32
	spans := make([][2]int32, currents)
	for k := range currentFeatures {
		fe := &currentFeatures[k]
		tiles := member[head[k+1]:head[k+2]]
		var su, sv, warm, most float64
		for _, t := range tiles {
			e, no := float64(u[t]), float64(v[t])
			su, sv = su+e, sv+no
			most = math.Max(most, math.Hypot(e, no))
			warm += w.WaterWarmth(int(t))
			votes[g.nearGyre(int(t), f.gyre)]++
		}
		fe.Count = len(tiles)
		fe.Flow = most
		fe.Warmth = float32(warm / float64(len(tiles)))
		hx, hy := su, sv
		if s := math.Hypot(hx, hy); s > 0 {
			hx, hy = hx/s, hy/s
		}
		fe.Heading = float32(math.Mod(math.Atan2(hx, hy)*180/math.Pi+360, 360))
		// The gyre more of it lies in or beside than any other: a boundary
		// current runs where the gyre's streamfunction climbs from the
		// shore, short of gyreFloor, and an eastern one well short of it.
		best, held := 0, int32(0)
		for p, c := range votes {
			if p > 0 && c > held {
				best, held = p, c
			}
			votes[p] = 0
		}
		fe.Gyre = FeatureID(best)
		start := int32(len(paths))
		var transport float64
		paths, transport = g.layCurrent(tiles, round, u, v, layer, hx, hy, paths, &lay)
		fe.Transport = float32(transport)
		spans[k] = [2]int32{start, int32(len(paths))}
	}
	for k := range currentFeatures {
		s := spans[k]
		currentFeatures[k].Path = paths[s[0]:s[1]:s[1]]
	}

	// The upwellings: the sea where the water comes up faster than
	// riseFloor, joined where it touches.
	rise := psi
	for i := range n {
		rise[i] = 0
		if sea[i] {
			rise[i] = float32(w.Upwelling(i))
		}
	}
	runs, stack = g.seaRuns(run, round, stack,
		func(i int) bool { return rise[i] >= riseFloor },
		func(i, j int) bool { return true })
	upPlace, ups := keepRuns(runs, run, f.upwell, func(r seaRun) bool { return r.area >= upwellLeast })
	upFeatures := make([]Feature, ups)
	upWarm, upRise := make([]float64, ups), make([]float64, ups)
	for k, r := range runs {
		if p := upPlace[k+1]; p > 0 {
			upFeatures[p-1] = Feature{Kind: Upwelling, First: r.first}
		}
	}
	for i, p := range f.upwell {
		if p == 0 {
			continue
		}
		upFeatures[p-1].Count++
		upRise[p-1] += float64(rise[i])
		upWarm[p-1] += w.WaterWarmth(i)
	}
	for k := range upFeatures {
		fe := &upFeatures[k]
		fe.Flow = upRise[k] / float64(fe.Count)
		fe.Warmth = float32(upWarm[k] / float64(fe.Count))
	}

	// Into the registry, kind by kind; a current's gyre is named by its id.
	f.seaBase[0] = FeatureID(len(all))
	f.seaBase[1] = f.seaBase[0] + FeatureID(currents)
	f.seaBase[2] = f.seaBase[1] + FeatureID(gyres)
	for k := range currentFeatures {
		if p := currentFeatures[k].Gyre; p > 0 {
			currentFeatures[k].Gyre = f.seaBase[1] + p
		}
	}
	all = append(all, currentFeatures...)
	all = append(all, gyreFeatures...)
	all = append(all, upFeatures...)
	return all, stack
}

// toShore is how far, in kilometres, along tile i's row toward the east (dx
// 1) or the west (-1) the nearest tile that is not sea lies, looked for as
// far as shoreReach; past that it is infinite.
func (g *Grid) toShore(i, dx int, sea []bool) float64 {
	x, y := i%g.W, i/g.W
	step := g.air.Dx[y]
	reach := int(math.Ceil(shoreReach / step))
	for k := 1; k <= reach; k++ {
		q := geom.Pos{X: x + k*dx, Y: y}
		if g.Wrap {
			q = g.Norm(q)
		}
		if !g.In(q) {
			break
		}
		if !sea[g.Index(q)] {
			return float64(k) * step
		}
	}
	return math.Inf(1)
}

// nearGyre is the place of the gyre tile i lies in, or else of the one
// nearest it along its row through the sea within gyreReach, the nearer way
// first and the west on a tie; or 0.
func (g *Grid) nearGyre(i int, gyre []uint16) uint16 {
	if gyre[i] > 0 {
		return gyre[i]
	}
	x, y := i%g.W, i/g.W
	reach := int(math.Ceil(gyreReach / g.air.Dx[y]))
	west, east := true, true
	for k := 1; k <= reach && (west || east); k++ {
		for _, dx := range [2]int{-k, k} {
			if (dx < 0 && !west) || (dx > 0 && !east) {
				continue
			}
			q := geom.Pos{X: x + dx, Y: y}
			if g.Wrap {
				q = g.Norm(q)
			}
			if !g.In(q) {
				west, east = west && dx > 0, east && dx < 0
				continue
			}
			j := g.Index(q)
			if p := gyre[j]; p > 0 {
				return p
			}
			if !g.underSea(j) {
				// Not across land.
				west, east = west && dx > 0, east && dx < 0
			}
		}
	}
	return 0
}

// layScratch is what layCurrent works in, kept from one current to the next.
type layScratch struct {
	count            []int32
	cross, flux, off []float64
	best             []int32
}

// layCurrent lays a current's tiles out flat and along the way it runs, hx
// toward the east and hy the north, and cuts it into sections a row's breadth
// apart across that way. Its path is the tile nearest the middle of each
// section, from its head downstream, appended to path; and the water it
// carries is what goes through its narrowest section in the middle half of
// its length - the ends of a current are where it gathers and spreads, and
// carry little - in sverdrups over the depth the current is spread over
// there (layer: the thermocline's, never less than atmos.FlowLeast).
func (g *Grid) layCurrent(tiles []int32, round []int8, u, v, layer []float32, hx, hy float64,
	path []int32, s *layScratch) ([]int32, float64) {
	dy := g.air.Dy
	// Where each tile lies along the way, in rows' breadths, and across it.
	along := func(t int32) (float64, float64) {
		x, y := int(t)%g.W, int(t)/g.W
		px := float64(x+int(round[t])*g.W) * g.air.Dx[y] / dy
		py := -float64(y)
		return px*hx + py*hy, -px*hy + py*hx
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, t := range tiles {
		a, _ := along(t)
		lo, hi = math.Min(lo, a), math.Max(hi, a)
	}
	nb := int(hi-lo) + 1
	grow := func(f []float64) []float64 {
		if cap(f) < nb {
			return make([]float64, nb)
		}
		f = f[:nb]
		clear(f)
		return f
	}
	s.cross, s.flux, s.off = grow(s.cross), grow(s.flux), grow(s.off)
	if cap(s.count) < nb {
		s.count, s.best = make([]int32, nb), make([]int32, nb)
	}
	s.count, s.best = s.count[:nb], s.best[:nb]
	clear(s.count)
	for _, t := range tiles {
		a, c := along(t)
		b := int(a - lo)
		y := int(t) / g.W
		s.count[b]++
		s.cross[b] += c
		// What the tile carries the way the current runs, over its area: a
		// section's sum of it, over the section's breadth along the way, is
		// what goes through the section.
		s.flux[b] += (float64(u[t])*hx + float64(v[t])*hy) * float64(layer[t]) * g.air.Dx[y] * dy * 1e6
		s.best[b] = -1
	}
	for b := range nb {
		if s.count[b] > 0 {
			s.cross[b] /= float64(s.count[b])
		}
	}
	for _, t := range tiles {
		a, c := along(t)
		b := int(a - lo)
		if d := math.Abs(c - s.cross[b]); s.best[b] < 0 || d < s.off[b] {
			s.best[b], s.off[b] = t, d
		}
	}
	for b := range nb {
		if s.count[b] > 0 {
			path = append(path, s.best[b])
		}
	}
	first, last := 0, nb
	if nb >= 4 {
		first, last = nb/4, nb-nb/4
	}
	narrow := -1
	for b := first; b < last; b++ {
		if s.count[b] > 0 && (narrow < 0 || s.count[b] < s.count[narrow]) {
			narrow = b
		}
	}
	if narrow < 0 {
		return path, 0
	}
	return path, s.flux[narrow] / (dy * 1e3) / 1e6
}
