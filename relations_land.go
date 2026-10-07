package terra

import (
	"slices"

	"github.com/LukasSelin/terra/geom"
	"github.com/LukasSelin/terra/internal/atmos"
)

// The land's relations: what a range does to the country behind it, what a
// basin does to its lake and to the sea, what a climate grows, and which
// plates raised which range.
//
// Each is a reading, as the sea's are (see relations.go). A range's rain
// shadow is the orographic rain the air's budget kept (Budget.Oro), on the
// cells of the range the air crossed before it came to the ground, walked
// back up each phase's wind the way Why walks it; a basin fills its lake
// with the inflow the drainage pooled into it; a region grows the woods the
// cover left standing on it; a plate raised a belt because the book says
// its meeting did (Feature.Plates); and a basin drains into the sea's
// feature its outlet lies in or nearest. None is a rule laid over the world
// afterwards: where the record has no number for it, there is no relation.

const (
	// shadowReach is how far, in kilometres, the air over a basin or a
	// climate region is walked back up the wind for the ranges it crossed:
	// about the breadth of the dry country behind the world's great ranges,
	// the Great Basin behind the Sierra Nevada, Patagonia behind the Andes,
	// the Tarim behind the Tibetan plateau's rim. The walk ends sooner at
	// the sea, which gives the air its water back.
	shadowReach = 1500.0
	// beltCell is the least share of an air cell's dry ground one belt has
	// to hold for the rain the cell's lift wrings out to be that belt's: the
	// air reads the ground a cell at a time, and a quarter of it is a range
	// the cell's lift is made of and not a spur at its edge.
	beltCell = 0.25
	// shadowFloor is the least orographic rain, in millimetres a year, a
	// belt has to wring out of the air on its way to a basin's or a region's
	// ground, on the whole of that ground, for the ground to lie in its
	// shadow: fifty millimetres, a tenth of what a dry climate's line is
	// drawn at in a warm country or under, and about what the rain differs by from
	// one year to the next in a dry one.
	shadowFloor = 50.0
	// A lake is filled by its basin whenever any water comes into it, and a
	// region grows every wood standing on it, however little of the region
	// the wood covers: an inflow and a share are exact, and there is no
	// noise for a floor to keep out. The flows are the map's own, at its
	// tile span (see units.go), and not the planet's, so no floor in cubic
	// metres a second would mean the same on a valley and a globe.
	//
	// drainTiles is the least ground, in tiles, a basin has to drain for its
	// water to be followed into the sea's features: a river's, as shape.go
	// reads one (textureChannel), and not a gully's off the coast.
	drainTiles = textureChannel
	// drainReach is how far, in kilometres, the sea is searched from a
	// basin's outlet for the current, the upwelling or the gyre its water
	// goes into: a river's plume, which the Amazon's carries some five
	// hundred kilometres out before the sea has it.
	drainReach = 500.0
	// highFloor is the least year's mean descent, in metres a second at 500
	// hPa, the air has to come down at over a tile for it to lie under a
	// subtropical high: a quarter of the Hadley cell's strongest (see
	// atmos.subsideMost), where the lid the descent lays (Lilly's, about a
	// kilometre at the strongest) has risen to some four kilometres and
	// holds back little of the column's water.
	highFloor = 1e-3
	// subsideFloor is the least descent, in millimetres a second, a high
	// has to bring down on the whole of a climate region's ground for the
	// region to lie under it: the same quarter of the strongest, so that a
	// region only the high's fringe reaches is not counted.
	subsideFloor = 1.0
)

// landRelations appends to rel the land's relations, kind by kind: Raises,
// Fills, Grows, Subsides, Shadows and DrainsInto. f is the registry being read, which
// g.features is not yet.
func (g *Grid) landRelations(f *Features, rel []Relation) []Relation {
	n := len(g.Tiles)

	// Each belt is raised by the plates of its meeting, as they are now:
	// the plate each has been welded into, once.
	for k := range f.All {
		fe := &f.All[k]
		if fe.Kind != UpliftBelt {
			continue
		}
		var by [2]FeatureID
		for j, p := range fe.Plates {
			if p == noPlate {
				continue
			}
			by[j] = f.plate[g.rootPlate(p)]
		}
		if by[1] == by[0] {
			by[1] = 0
		}
		for _, id := range by {
			if id > 0 {
				rel = append(rel, Relation{From: id, To: fe.ID, Kind: Raises, Quantity: fe.Lift, Unit: "m"})
			}
		}
	}

	// Each lake is filled by the basin its water stands in, with what the
	// drainage pooled into it.
	if len(f.basin) == n {
		for k := range f.All {
			fe := &f.All[k]
			if fe.Kind != StandingLake {
				continue
			}
			b := f.basin[fe.First]
			if in := g.Lakes[fe.Lake].Inflow; b > 0 && in > 0 {
				rel = append(rel, Relation{From: b, To: fe.ID, Kind: Fills, Quantity: in, Unit: "m³/s"})
			}
		}
	}

	// Each climate region grows the woods on its ground, by the share of
	// the ground each covers.
	if len(f.climate) == n && len(f.wood) == n {
		count := make([]int32, len(f.All)+1)
		var on []FeatureID
		for k := range f.All {
			fe := &f.All[k]
			if fe.Kind != ClimateRegion || fe.Count == 0 {
				continue
			}
			for _, t := range fe.Tiles {
				if w := f.wood[t]; w > 0 {
					if count[w] == 0 {
						on = append(on, w)
					}
					count[w]++
				}
			}
			slices.Sort(on)
			for _, w := range on {
				if s := float64(count[w]) / float64(fe.Count); s > 0 {
					rel = append(rel, Relation{From: fe.ID, To: w, Kind: Grows, Quantity: s, Unit: "share"})
				}
				count[w] = 0
			}
			on = on[:0]
		}
	}

	// Each climate region lies under the subtropical high whose air comes
	// down over it, by the year's mean descent on the whole of its ground.
	if len(f.climate) == n && len(f.high) == n {
		sum := make([]float64, len(f.All)+1)
		var on []FeatureID
		for k := range f.All {
			fe := &f.All[k]
			if fe.Kind != ClimateRegion || fe.Count == 0 {
				continue
			}
			for _, t := range fe.Tiles {
				if h := f.high[t]; h > 0 {
					if sum[h] == 0 {
						on = append(on, h)
					}
					sum[h] += f.descent[t]
				}
			}
			slices.Sort(on)
			for _, h := range on {
				if d := 1000 * sum[h] / float64(fe.Count); d >= subsideFloor {
					rel = append(rel, Relation{From: h, To: fe.ID, Kind: Subsides, Quantity: d, Unit: "mm/s"})
				}
				sum[h] = 0
			}
			on = on[:0]
		}
	}

	// The ranges' rain shadows, on the basins and the climate regions.
	if s := g.shadowReader(f); s != nil {
		sum := make([]float64, len(f.All)+1)
		var on []FeatureID
		for k := range f.All {
			fe := &f.All[k]
			if (fe.Kind != DrainageBasin && fe.Kind != ClimateRegion) || fe.Count == 0 {
				continue
			}
			for _, t := range fe.Tiles {
				for _, p := range s.cell(s.e.CellOfTile(int(t))) {
					if sum[p.belt] == 0 {
						on = append(on, p.belt)
					}
					sum[p.belt] += p.mm
				}
			}
			slices.Sort(on)
			for _, b := range on {
				if mm := sum[b] / float64(fe.Count); mm >= shadowFloor {
					rel = append(rel, Relation{From: b, To: fe.ID, Kind: Shadows, Quantity: mm, Unit: "mm"})
				}
				sum[b] = 0
			}
			on = on[:0]
		}
	}

	// Each river's water into the sea's feature at its mouth, or the
	// nearest to it within drainReach.
	if f.current != nil && len(f.basin) == n && g.air != nil {
		reach := int(drainReach / g.air.Dy)
		seen := make([]int32, n)
		var queue, next []int32
		for k := range f.All {
			fe := &f.All[k]
			if fe.Kind != DrainageBasin || float64(fe.Count) < drainTiles || !g.seaTile(int(fe.Outlet)) {
				continue
			}
			stamp := int32(k + 1)
			queue = append(queue[:0], fe.Outlet)
			seen[fe.Outlet] = stamp
			to := FeatureID(0)
			for ring := 0; ring <= reach && to == 0 && len(queue) > 0; ring++ {
				next = next[:0]
				for _, i := range queue {
					if to = f.seaAt(int(i)); to > 0 {
						break
					}
					p := g.PosOf(int(i))
					for _, d := range Dirs {
						q := geom.Pos{X: p.X + d.X, Y: p.Y + d.Y}
						if g.Wrap {
							q = g.Norm(q)
						}
						if !g.In(q) {
							continue
						}
						if j := g.Index(q); seen[j] != stamp && g.seaTile(j) {
							seen[j] = stamp
							next = append(next, int32(j))
						}
					}
				}
				queue, next = next, queue
			}
			if to > 0 {
				rel = append(rel, Relation{From: fe.ID, To: to, Kind: DrainsInto, Quantity: fe.Flow, Unit: "m³/s"})
			}
		}
	}
	return rel
}

// seaAt is the sea's feature the water of tile i is in, of the current, the
// upwelling and the gyre the first it is in, or 0.
func (f *Features) seaAt(i int) FeatureID {
	switch {
	case i < len(f.current) && f.current[i] > 0:
		return f.seaBase[0] + FeatureID(f.current[i])
	case i < len(f.upwell) && f.upwell[i] > 0:
		return f.seaBase[2] + FeatureID(f.upwell[i])
	case i < len(f.gyre) && f.gyre[i] > 0:
		return f.seaBase[1] + FeatureID(f.gyre[i])
	}
	return 0
}

// shadowRead is what the rain shadows are read off: the belt each air cell's
// lift is, and, cell by cell as they are asked for, the belts the air over
// the cell crossed and what each wrung out of it.
type shadowRead struct {
	e *atmos.Env
	w *Winds
	// belt is each cell's belt, or 0: the one with the most of the cell's
	// dry ground, where it has beltCell of it.
	belt []FeatureID
	// span[c] is where cell c's shadows lie in kept, from one past its
	// start, and nothing before they are read.
	span [][2]int32
	kept []shadowPart
	// phase is one phase's most from each belt, while a cell is read, oro
	// that phase's orographic rain, and visit the step of the walk that
	// reads them, made once.
	phase []shadowPart
	oro   []float64
	visit func(at int, km float64) bool
}

// shadowPart is what one belt wrung out of the air on its way to a cell, in
// millimetres a year.
type shadowPart struct {
	belt FeatureID
	mm   float64
}

// shadowReader is the rain shadows of g's belts as f has them, or nil on a
// map with no belts or no budget of the air's water.
func (g *Grid) shadowReader(f *Features) *shadowRead {
	w := g.winds
	if f == nil || len(f.belt) != len(g.Tiles) || w == nil || w.Env == nil ||
		w.W*w.Cell != g.W || w.H*w.Cell != g.H {
		return nil
	}
	e := w.Env
	cells := e.W * e.H
	for k := range atmos.Phases {
		if len(w.Budget[k].Oro) != cells || len(w.U[k]) != cells {
			return nil
		}
	}
	s := &shadowRead{e: e, w: w, belt: make([]FeatureID, cells), span: make([][2]int32, cells), kept: make([]shadowPart, 0, 2*cells)}
	found := false
	var ids [16]FeatureID
	var counts [16]int
	for c := range cells {
		k, dry := 0, 0
		cx, cy := c%e.W, c/e.W
		for dy := 0; dy < e.Cell; dy++ {
			row := (cy*e.Cell + dy) * g.W
			for dx := 0; dx < e.Cell; dx++ {
				i := row + cx*e.Cell + dx
				if g.underSea(i) {
					continue
				}
				dry++
				b := f.belt[i]
				if b == 0 {
					continue
				}
				j := 0
				for j < k && ids[j] != b {
					j++
				}
				if j == k {
					if k == len(ids) {
						continue
					}
					ids[k], counts[k] = b, 0
					k++
				}
				counts[j]++
			}
		}
		best, held := FeatureID(0), 0
		for j := 0; j < k; j++ {
			if counts[j] > held || (counts[j] == held && ids[j] < best) {
				best, held = ids[j], counts[j]
			}
		}
		if held > 0 && float64(held) >= beltCell*float64(dry) {
			s.belt[c], found = best, true
		}
	}
	if !found {
		return nil
	}
	return s
}

// cell is the belts the air over cell c crossed, walked back up each phase's
// wind to the sea or as far as shadowReach, and what each wrung out of it: in
// each phase the most the belt's lift wrung out of the air on any of its
// cells the walk crossed, the year's mean of that over the phases, in
// millimetres a year. The cell's own lift is not its shadow, and is not
// counted. It is kept, so that a cell is read once.
func (s *shadowRead) cell(c int) []shadowPart {
	if at := s.span[c]; at[0] > 0 {
		return s.kept[at[0]-1 : at[1]-1]
	}
	start := len(s.kept)
	if s.visit == nil {
		s.visit = func(at int, km float64) bool {
			if km > shadowReach {
				return false
			}
			b, oro := s.belt[at], s.oro[at]
			if b == 0 || oro <= 0 {
				return true
			}
			for j := range s.phase {
				if s.phase[j].belt == b {
					s.phase[j].mm = max(s.phase[j].mm, oro)
					return true
				}
			}
			s.phase = append(s.phase, shadowPart{belt: b, mm: oro})
			return true
		}
	}
	for k := range atmos.Phases {
		s.oro, s.phase = s.w.Budget[k].Oro, s.phase[:0]
		upwindWalk(s.e, s.w.U[k], s.w.V[k], c, s.visit)
		for _, p := range s.phase {
			mm := p.mm * secondsPerYear / atmos.Phases
			j := start
			for j < len(s.kept) && s.kept[j].belt != p.belt {
				j++
			}
			if j == len(s.kept) {
				s.kept = append(s.kept, shadowPart{belt: p.belt})
			}
			s.kept[j].mm += mm
		}
	}
	// By belt, so that what is summed from them is summed in one order.
	slices.SortFunc(s.kept[start:], func(a, b shadowPart) int { return int(a.belt - b.belt) })
	s.span[c] = [2]int32{int32(start) + 1, int32(len(s.kept)) + 1}
	return s.kept[start:]
}
