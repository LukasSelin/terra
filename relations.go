package terra

import (
	"cmp"
	"math"
	"slices"

	"github.com/LukasSelin/terra/geom"
	"github.com/LukasSelin/terra/internal/atmos"
	"github.com/LukasSelin/terra/internal/phase"
)

// Relations: what the features do to one another.
//
// The registry knows which feature each tile is in, and with that alone the
// Gulf Stream and the mild coast it passes are two things that happen to lie
// side by side. A relation is the link between them, with the number that
// makes it true: the current Warms the climate region by so many degrees,
// the upwelling Dries the desert by such a share of its rain. Each is a
// reading of a quantity a pass computed and kept, never a rule laid over the
// world afterwards (see why.go): the degrees are the share of CoastWarmth
// the current's own water makes, taken back out of the blur the weather
// spread it with; the share of rain is what the weather's inversion took;
// the water a basin rains is traced up the wind the weather blew. A relation
// a reading does not give is not made, and where a reading is too weak to
// mean anything a floor below says so.
//
// The layer is general: a relation is any two features and a kind. The sea's
// are read here; the land's - a range's rain shadow on a basin, a basin
// filling a lake, a plate raising a range, a river reaching the sea - are
// read in relations_land.go.
//
// Relations are built with the registry, on one goroutine, feature by
// feature in id order and tile by tile in tile order, so that the same world
// gives the same relations however many goroutines made it.

// RelationKind is what one feature does to another.
type RelationKind uint8

const (
	NoRelation RelationKind = iota
	// Warms and Cools are a current and a climate region whose coast it
	// lies off: Quantity is how many degrees warmer or colder the current's
	// water makes the region's year, on the whole of the region's ground
	// that feels it at all (see coastFloor), as CoastWarmth has it.
	Warms
	Cools
	// Dries is an upwelling and a climate region beside it: Quantity is the
	// share of the rain the inversion over the cold water takes, on the
	// whole of the region's ground the upwelling chills. The share is the
	// inversion's whole, of all the cold water off that coast; the
	// upwelling is the part of it that comes up from under.
	Dries
	// Waters is a current and a drainage basin whose rain the air took up
	// off it: Quantity is the share of the basin's year of rain that falls
	// in phases whose air, walked back up the wind, last came off the
	// current's water.
	Waters
	// PartOf is a current and the gyre it runs in or beside. It carries no
	// quantity.
	PartOf
	// Feeds is a current and one its water runs into: followed along the
	// sea's current from the first one's mouth, or back against it from the
	// second one's head, the next current the water is in. Quantity is the
	// lesser of what the two carry, in sverdrups: the most that can run from
	// the one into the other.
	Feeds
	// Shadows is an uplift belt and a drainage basin or a climate region in
	// its rain shadow: walked back up each phase's wind from the ground's
	// cell to the sea, the belt is crossed and its lift wrings water out of
	// the air there. Quantity is the orographic rain the belt wrung out of
	// that air, in millimetres a year, on the whole of the basin's or the
	// region's ground: see relations_land.go.
	Shadows
	// Fills is a drainage basin and a lake its water stands in: Quantity is
	// the lake's inflow, in cubic metres a second.
	Fills
	// Grows is a climate region and a wood standing in it: Quantity is the
	// share of the region's ground the wood covers.
	Grows
	// Raises is a plate and an uplift belt its meeting raised: Quantity is
	// the most the meeting raised any tile of the belt, in the history's
	// metres (Feature.Lift), below nought for a meeting that let the ground
	// down.
	Raises
	// DrainsInto is a drainage basin and the sea's feature its water reaches:
	// the current, the upwelling or the gyre nearest its outlet. Quantity is
	// the flow at the outlet, in cubic metres a second.
	DrainsInto
	relationKinds
)

var relationKindNames = [relationKinds]string{"none", "warms", "cools", "dries", "waters", "part of", "feeds",
	"shadows", "fills", "grows", "raises", "drains into"}

func (k RelationKind) String() string {
	if int(k) < len(relationKindNames) {
		return relationKindNames[k]
	}
	return "relation?"
}

// Relation is one feature acting on another: From does Kind to To, by
// Quantity in Unit. Unit is nothing where the kind carries no quantity.
type Relation struct {
	From, To FeatureID
	Kind     RelationKind
	Quantity float64
	Unit     string
}

const (
	// coastFloor is the least, in degrees, a feature's water has to make a
	// tile's year warmer or colder for the tile to feel it: a tenth of a
	// degree, under what any thermometer of a year's mean could tell, and
	// what the blur's farthest reach leaves of a weak current.
	coastFloor = 0.1
	// dryFloor is the least share of the rain an inversion has to take for
	// the cold water under it to dry the ground: one part in twenty, about
	// what a year's rain differs by from the next.
	dryFloor = 0.05
	// waterFloor is the least share of a basin's rain the air has to have
	// taken up off a current for the current to water it: a tenth.
	waterFloor = 0.1
	// feedReach is how far, in kilometres, the water from a current's mouth
	// is followed for the current it runs into, and back from its head for
	// the one it came out of: the breadth of an ocean in the westerlies. The
	// North Atlantic Drift runs some four thousand kilometres from the Grand
	// Banks to Norway, and much of the way slower than currentFloor.
	feedReach = 4000.0
)

// Relations is every relation in the registry, by From, then Kind, then To.
func (f *Features) Relations() []Relation {
	if f == nil {
		return nil
	}
	return f.rel
}

// RelationsOf is every relation the feature id is in: those it is From
// first, by Kind and To, and then those it is To, by From and Kind. It is a
// new slice.
func (f *Features) RelationsOf(id FeatureID) []Relation {
	if f == nil || id <= 0 {
		return nil
	}
	lo, _ := slices.BinarySearchFunc(f.rel, id, func(r Relation, id FeatureID) int { return cmp.Compare(r.From, id) })
	hi := lo
	for hi < len(f.rel) && f.rel[hi].From == id {
		hi++
	}
	out := slices.Clone(f.rel[lo:hi])
	at, _ := slices.BinarySearchFunc(f.byTo, id, func(k int32, id FeatureID) int { return cmp.Compare(f.rel[k].To, id) })
	for ; at < len(f.byTo) && f.rel[f.byTo[at]].To == id; at++ {
		out = append(out, f.rel[f.byTo[at]])
	}
	return out
}

// seaRead is what the relations and Why read the sea's features off on the
// air's cells: which of them the water of a cell is, and what each makes of
// a tile's CoastWarmth.
type seaRead struct {
	g *Grid
	f *Features
	e *atmos.Env
	// currents and ups are how many currents and upwellings there are.
	currents, ups int
	// tile is what each feature's water adds to the tile last read, and
	// cell what it adds to the cell being read.
	tile, cell partSums
	// splits is each cell's split, as split read it; splitAt[c] is where
	// cell c's lie in it, from one past its start, and nothing before then.
	splits  []seaPart
	splitAt [][2]int32
}

// partSums is the degrees each current's and each upwelling's water adds to
// a place, by the feature's place among its kind, and curOn and upOn the
// places that have anything, in the order they were first given it.
type partSums struct {
	cur, up       []float64
	curOn, upOn   []uint16
	curHas, upHas []bool
}

func newPartSums(currents, ups int) partSums {
	return partSums{cur: make([]float64, currents+1), up: make([]float64, ups+1),
		curHas: make([]bool, currents+1), upHas: make([]bool, ups+1)}
}

func (s *partSums) add(up bool, p uint16, d float64) {
	if up {
		if !s.upHas[p] {
			s.upHas[p], s.upOn = true, append(s.upOn, p)
		}
		s.up[p] += d
		return
	}
	if !s.curHas[p] {
		s.curHas[p], s.curOn = true, append(s.curOn, p)
	}
	s.cur[p] += d
}

func (s *partSums) clear() {
	for _, p := range s.curOn {
		s.cur[p], s.curHas[p] = 0, false
	}
	for _, p := range s.upOn {
		s.up[p], s.upHas[p] = 0, false
	}
	s.curOn, s.upOn = s.curOn[:0], s.upOn[:0]
}

// seaReader is the sea's features of g as seaRead reads them, or nil on a
// map with no sea's features or no weather.
func (g *Grid) seaReader(f *Features) *seaRead {
	w := g.winds
	if f == nil || f.current == nil || w == nil || w.Env == nil || w.Warm == nil ||
		w.W*w.Cell != g.W || w.H*w.Cell != g.H {
		return nil
	}
	r := &seaRead{g: g, f: f, e: w.Env}
	r.currents = int(f.seaBase[1] - f.seaBase[0])
	for k := int(f.seaBase[2]); k < len(f.All) && f.All[k].Kind == Upwelling; k++ {
		r.ups++
	}
	r.tile, r.cell = newPartSums(r.currents, r.ups), newPartSums(r.currents, r.ups)
	return r
}

// seaTile is whether tile i is under the sea and not a lake: the sea the
// sea's features are read over (see readSea).
func (g *Grid) seaTile(i int) bool {
	return g.underSea(i) && !(len(g.lakeOf) == len(g.Tiles) && g.lakeOf[i] >= 0)
}

// cellTiles visits the tiles of air cell c.
func (r *seaRead) cellTiles(c int, visit func(i int)) {
	e, g := r.e, r.g
	cx, cy := c%e.W, c/e.W
	for dy := 0; dy < e.Cell; dy++ {
		row := (cy*e.Cell + dy) * g.W
		for dx := 0; dx < e.Cell; dx++ {
			visit(row + cx*e.Cell + dx)
		}
	}
}

// seaPart is what one feature of the sea's water adds to the coast warmth a
// cell's country feels: up says an upwelling's, and not a current's.
type seaPart struct {
	place   uint16
	up      bool
	degrees float64
}

// cellParts appends to parts what each current's and each upwelling's water
// adds to Coast over cell c, in degrees: the water of each cell within reach
// of it, CoastFrom's, shared among the open sea tiles of that cell and so
// among the features those tiles are in. The currents' come to Coast less
// what water no current covers, and the upwellings' likewise. A current's
// tile that is also an upwelling's is in both.
func (r *seaRead) cellParts(c int, parts []seaPart) []seaPart {
	if r.e.Coast[c] == 0 {
		return parts
	}
	s := &r.cell
	s.clear()
	felt := r.e.CoastFrom(c, func(j int, d float64) {
		for _, p := range r.split(j) {
			s.add(p.up, p.place, d*p.degrees)
		}
	})
	if felt == 0 {
		return parts
	}
	for _, p := range s.curOn {
		parts = append(parts, seaPart{place: p, degrees: s.cur[p] * felt})
	}
	for _, p := range s.upOn {
		parts = append(parts, seaPart{place: p, up: true, degrees: s.up[p] * felt})
	}
	return parts
}

// split is how the water of cell c is shared among the sea's features: for
// each current and each upwelling with open sea tiles in it, the share of
// the cell's open sea tiles it has, in degrees' place. Where r keeps them
// (splitAt is made) it is read once a cell; otherwise it holds only until
// the next cell is asked for.
func (r *seaRead) split(c int) []seaPart {
	if r.splitAt == nil {
		r.splits = r.splits[:0]
	} else if at := r.splitAt[c]; at[0] > 0 {
		return r.splits[at[0]-1 : at[1]-1]
	}
	n := 0
	r.cellTiles(c, func(i int) {
		if r.g.seaTile(i) {
			n++
		}
	})
	start := len(r.splits)
	r.cellTiles(c, func(i int) {
		if n == 0 || !r.g.seaTile(i) {
			return
		}
		r.splits = addSplit(r.splits, start, r.f.current[i], false, 1/float64(n))
		r.splits = addSplit(r.splits, start, r.f.upwell[i], true, 1/float64(n))
	})
	if r.splitAt != nil {
		r.splitAt[c] = [2]int32{int32(start) + 1, int32(len(r.splits)) + 1}
	}
	return r.splits[start:]
}

// addSplit adds share to feature p's among the splits from start on.
func addSplit(splits []seaPart, start int, p uint16, up bool, share float64) []seaPart {
	if p == 0 {
		return splits
	}
	for k := start; k < len(splits); k++ {
		if splits[k].place == p && splits[k].up == up {
			splits[k].degrees += share
			return splits
		}
	}
	return append(splits, seaPart{place: p, up: up, degrees: share})
}

// tileParts reads into r.tile what each feature's water adds to tile i's
// CoastWarmth, in degrees, as CoastWarmth reads Coast between the cells; of
// gives a cell's parts, and nil reads them afresh.
func (r *seaRead) tileParts(i int, of func(c int) []seaPart) {
	cells, weights := r.e.Corners(i)
	r.tile.clear()
	var buf []seaPart
	for k, c := range cells {
		if weights[k] == 0 {
			continue
		}
		var parts []seaPart
		if of != nil {
			parts = of(c)
		} else {
			buf = r.cellParts(c, buf[:0])
			parts = buf
		}
		for _, p := range parts {
			r.tile.add(p.up, p.place, p.degrees*weights[k])
		}
	}
}

// strongest is the feature whose water adds the most to tile i's
// CoastWarmth either way, and what it adds, after tileParts; 0 where none
// adds coastFloor. Of the upwellings and the currents, whose water overlaps,
// up says which to look among.
func (r *seaRead) strongest(up bool, sign float64) (FeatureID, float64) {
	s := &r.tile
	on, sums, base := s.curOn, s.cur, r.f.seaBase[0]
	if up {
		on, sums, base = s.upOn, s.up, r.f.seaBase[2]
	}
	best, most := uint16(0), 0.0
	for _, p := range on {
		d := sums[p]
		if sign != 0 {
			d *= sign
		} else {
			d = math.Abs(d)
		}
		if d >= coastFloor && (best == 0 || d > most || (d == most && p < best)) {
			best, most = p, d
		}
	}
	if best == 0 {
		return 0, 0
	}
	return base + FeatureID(best), sums[best]
}

// owner is the place of the current whose water cell c is - the one with
// more than half the cell's open sea - or 0.
func (r *seaRead) owner(c int) uint16 {
	n, best, held := 0, uint16(0), 0
	var places [16]uint16
	var counts [16]int
	k := 0
	r.cellTiles(c, func(i int) {
		if !r.g.seaTile(i) {
			return
		}
		n++
		p := r.f.current[i]
		if p == 0 {
			return
		}
		for j := 0; j < k; j++ {
			if places[j] == p {
				counts[j]++
				return
			}
		}
		if k < len(places) {
			places[k], counts[k] = p, 1
			k++
		}
	})
	for j := 0; j < k; j++ {
		if counts[j] > held || (counts[j] == held && places[j] < best) {
			best, held = places[j], counts[j]
		}
	}
	if 2*held <= n {
		return 0
	}
	return best
}

// readRelations fills in the registry's relations: the land's (see
// relations_land.go) and the sea's, read off the weather. It runs after
// every feature has its id and its tiles.
func (g *Grid) readRelations(f *Features) {
	defer phase.Start("readRelations")()
	rel := g.landRelations(f, nil)
	if r := g.seaReader(f); r != nil {
		rel = g.seaRelations(f, r, rel)
	}
	slices.SortFunc(rel, func(a, b Relation) int {
		return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.To, b.To))
	})
	f.rel = slices.Clip(rel)
	f.byTo = make([]int32, len(rel))
	for k := range f.byTo {
		f.byTo[k] = int32(k)
	}
	slices.SortFunc(f.byTo, func(a, b int32) int {
		x, y := &f.rel[a], &f.rel[b]
		return cmp.Or(cmp.Compare(x.To, y.To), cmp.Compare(x.From, y.From), cmp.Compare(x.Kind, y.Kind))
	})
}

// seaRelations appends to rel the sea's relations, read off the weather:
// what the currents and the upwellings do to the climate regions and the
// basins, and to one another.
func (g *Grid) seaRelations(f *Features, r *seaRead, rel []Relation) []Relation {
	e := r.e
	cells := e.W * e.H
	curID := func(p uint16) FeatureID { return f.seaBase[0] + FeatureID(p) }
	upID := func(p uint16) FeatureID { return f.seaBase[2] + FeatureID(p) }

	// Each cell's parts, read once and kept while the relations are built:
	// span[c] is where they lie in kept, from one past its start, and
	// nothing before they are read. Each cell's split is kept the same way.
	r.splitAt = make([][2]int32, cells)
	var kept []seaPart
	span := make([][2]int32, cells)
	of := func(c int) []seaPart {
		if span[c][0] == 0 {
			s := int32(len(kept))
			kept = r.cellParts(c, kept)
			span[c] = [2]int32{s + 1, int32(len(kept)) + 1}
		}
		return kept[span[c][0]-1 : span[c][1]-1]
	}
	// Each region's sums, by place: the degrees and the tiles of each
	// current, and the share taken and the tiles of each upwelling.
	warmSum, warmN := make([]float64, r.currents+1), make([]int32, r.currents+1)
	drySum, dryN := make([]float64, r.ups+1), make([]int32, r.ups+1)
	var warmOn, dryOn []uint16
	for id := range f.All {
		fe := &f.All[id]
		if fe.Kind != ClimateRegion {
			continue
		}
		for _, t := range fe.Tiles {
			i := int(t)
			r.tileParts(i, of)
			for _, p := range r.tile.curOn {
				if d := r.tile.cur[p]; math.Abs(d) >= coastFloor {
					if warmN[p] == 0 {
						warmOn = append(warmOn, p)
					}
					warmSum[p] += d
					warmN[p]++
				}
			}
			taken := e.Inversion(e.CellOfTile(i))
			for _, p := range r.tile.upOn {
				if r.tile.up[p] <= -coastFloor {
					if dryN[p] == 0 {
						dryOn = append(dryOn, p)
					}
					drySum[p] += taken
					dryN[p]++
				}
			}
		}
		slices.Sort(warmOn)
		for _, p := range warmOn {
			d := warmSum[p] / float64(warmN[p])
			kind := Warms
			if d < 0 {
				kind = Cools
			}
			if math.Abs(d) >= coastFloor {
				rel = append(rel, Relation{From: curID(p), To: fe.ID, Kind: kind, Quantity: d, Unit: "°C"})
			}
			warmSum[p], warmN[p] = 0, 0
		}
		slices.Sort(dryOn)
		for _, p := range dryOn {
			if s := drySum[p] / float64(dryN[p]); s >= dryFloor {
				rel = append(rel, Relation{From: upID(p), To: fe.ID, Kind: Dries, Quantity: s, Unit: "share"})
			}
			drySum[p], dryN[p] = 0, 0
		}
		warmOn, dryOn = warmOn[:0], dryOn[:0]
	}
	kept, span, r.splits, r.splitAt = nil, nil, nil, nil

	// The basins: each tile's rain, phase by phase, given to the current the
	// air of that phase last came off.
	if len(g.rain) == len(g.Tiles) && len(g.winds.Budget[0].Rain) == cells {
		w := g.winds
		var reach [atmos.Phases][]int32
		owner := make([]int32, cells)
		for c := range owner {
			owner[c] = -1
		}
		for k := range reach {
			reach[k] = make([]int32, cells)
			for c := range reach[k] {
				reach[k][c] = -2
			}
		}
		from := func(k, c int) uint16 {
			if reach[k][c] == -2 {
				reach[k][c] = -1
				if _, s, ok := g.upwindCell(e, w.U[k], w.V[k], c); ok {
					reach[k][c] = int32(s)
				}
			}
			s := reach[k][c]
			if s < 0 {
				return 0
			}
			if owner[s] < 0 {
				owner[s] = int32(r.owner(int(s)))
			}
			return uint16(owner[s])
		}
		sum := make([]float64, r.currents+1)
		var on []uint16
		for id := range f.All {
			fe := &f.All[id]
			if fe.Kind != DrainageBasin {
				continue
			}
			total := 0.0
			for _, t := range fe.Tiles {
				i := int(t)
				rain := g.rain[i]
				if rain <= 0 {
					continue
				}
				total += rain
				c := e.CellOfTile(i)
				all := 0.0
				for k := range atmos.Phases {
					all += w.Budget[k].Rain[c] + w.Budget[k].Oro[c]
				}
				if all <= 0 {
					continue
				}
				for k := range atmos.Phases {
					p := from(k, c)
					if p == 0 {
						continue
					}
					if sum[p] == 0 {
						on = append(on, p)
					}
					sum[p] += rain * (w.Budget[k].Rain[c] + w.Budget[k].Oro[c]) / all
				}
			}
			slices.Sort(on)
			for _, p := range on {
				if s := sum[p] / total; s >= waterFloor {
					rel = append(rel, Relation{From: curID(p), To: fe.ID, Kind: Waters, Quantity: s, Unit: "share"})
				}
				sum[p] = 0
			}
			on = on[:0]
		}
	}

	// The currents: the gyre each runs in, and the current its water runs
	// into.
	// Each current's water is followed down from its mouth and back up from
	// its head: the one finds where the water goes, the other where it came
	// from, and a current may be fed by one it is not the next of - a drift
	// that spreads at a coast into the current up it and the one down it.
	var feeds [][2]uint16
	for p := 1; p <= r.currents; p++ {
		fe := &f.All[curID(uint16(p))-1]
		if fe.Gyre > 0 {
			rel = append(rel, Relation{From: fe.ID, To: fe.Gyre, Kind: PartOf})
		}
		if len(fe.Path) == 0 {
			continue
		}
		if q := g.follow(f, uint16(p), int(fe.Path[len(fe.Path)-1]), 1); q > 0 {
			feeds = append(feeds, [2]uint16{uint16(p), q})
		}
		if q := g.follow(f, uint16(p), int(fe.Path[0]), -1); q > 0 {
			feeds = append(feeds, [2]uint16{q, uint16(p)})
		}
	}
	slices.SortFunc(feeds, func(a, b [2]uint16) int { return cmp.Or(cmp.Compare(a[0], b[0]), cmp.Compare(a[1], b[1])) })
	for _, pq := range slices.Compact(feeds) {
		a, b := &f.All[curID(pq[0])-1], &f.All[curID(pq[1])-1]
		rel = append(rel, Relation{From: a.ID, To: b.ID, Kind: Feeds, Quantity: float64(min(a.Transport, b.Transport)), Unit: "Sv"})
	}
	return rel
}

// follow is the place of the current the water at tile start, in current
// from, runs into (way 1) or came out of (way -1): followed along the sea's
// current, or back against it, half a tile at a time as far as feedReach,
// until it is in another current, or comes ashore, or stands still. 0 is
// none.
func (g *Grid) follow(f *Features, from uint16, start int, way float64) uint16 {
	w := g.winds
	x, y := float64(start%g.W), float64(start/g.W)
	for km := 0.0; km <= feedReach; {
		q := geom.Pos{X: int(math.Round(x)), Y: int(math.Round(y))}
		if g.Wrap {
			q = g.Norm(q)
		}
		if !g.In(q) {
			return 0
		}
		i := g.Index(q)
		if !g.seaTile(i) {
			return 0
		}
		if p := f.current[i]; p > 0 && p != from {
			return p
		}
		u, v := w.SeaCurrent(i)
		s := math.Hypot(u, v)
		if s < 1e-4 {
			return 0
		}
		dx := g.air.Dx[q.Y]
		step := 0.5 * math.Min(dx, g.air.Dy)
		x += way * u / s * step / dx
		y -= way * v / s * step / g.air.Dy
		km += step
	}
	return 0
}
