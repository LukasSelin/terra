package terra

import (
	"slices"

	"github.com/LukasSelin/terra/geom"
)

// Features: the things the tiles make up.
//
// A tile knows its height, its rock, its plate and, on a made world, the
// meeting that raised it - see ledger.go - and what it does not know is what
// it is part of: which range, which river's country, which lake, which
// climate. Those are joined here, once the land is made and again after
// each age of Erode, into features with an id, an extent and a few numbers,
// so that a game can point at a range and a question about a tile (see Why)
// can name the thing the tile belongs to.
//
// Nothing here is a second reading of the world. A belt is the tiles the
// book says one meeting raised; a basin is the tree the water's own way
// down makes; a lake is the lake the drainage found; a plate is the plate
// the history left; a climate region is a run of Köppen's one letter. The
// registry joins, and invents nothing.
//
// Ids are deterministic: each kind's features are numbered in the order of
// their lowest tile, kind by kind in the order of FeatureKind, so the same
// seed gives the same ids. terra does not name them - a name is a culture's
// - but SetNamer hands the naming to whoever has one.

// FeatureKind is what kind of thing a feature is.
type FeatureKind uint8

const (
	NoFeature FeatureKind = iota
	// UpliftBelt is connected ground whose strongest meeting was the same
	// pair of plates, of the same kind, in the same epoch: a range, an arc,
	// a rift valley, a volcanic province. What "the Arken Mountains" is.
	// Made worlds only.
	UpliftBelt
	// DrainageBasin is every tile whose water leaves by one outlet - a river
	// mouth on the sea, a closed lake, the edge of a valley - with its trunk
	// river.
	DrainageBasin
	// StandingLake is one of Grid.Lakes with water in it.
	StandingLake
	// CrustPlate is one piece of the crust, by the plate number the tiles
	// carry. Made worlds only.
	CrustPlate
	// ClimateRegion is connected dry land of one Köppen group: A, B, C, D
	// or E.
	ClimateRegion
	featureKinds
)

var featureKindNames = [featureKinds]string{"none", "uplift belt", "drainage basin", "lake", "plate", "climate region"}

func (k FeatureKind) String() string {
	if int(k) < len(featureKindNames) {
		return featureKindNames[k]
	}
	return "feature?"
}

// FeatureID names a feature in its map's registry, from 1; 0 is none.
type FeatureID int32

// Feature is one thing the tiles make up. Which of the numbers mean anything
// depends on Kind; the rest are zero.
type Feature struct {
	ID   FeatureID
	Kind FeatureKind
	// Name is what the namer called it, or nothing. See SetNamer.
	Name string
	// Tiles is every tile of the feature, lowest index first, and Count how
	// many there are. A plate keeps no list - its tiles are those whose
	// Plate is its Number - and has only the count.
	Tiles []int32
	Count int
	// First is the lowest tile of the feature, which orders the ids.
	First int32

	// A belt: Plates are the two the meeting was between, as the history
	// numbered them (each may since have been welded into another; see
	// Grid.PlateOf), the second noPlate for a hotspot; Meeting is what kind
	// of meeting; Epoch when; Lift the most the meeting raised any tile of
	// the belt in that epoch, in the history's metres (see ledger); Top the
	// tile that now stands highest and Height its height on the map.
	Plates  [2]uint8
	Meeting MeetingKind
	Epoch   uint8
	Lift    float64
	Top     int32
	Height  float64

	// A basin: Outlet is the tile its water leaves by - the sea tile at a
	// river's mouth, the lowest tile of a closed lake, or a tile on the
	// edge of a valley; Trunk the tiles of its trunk river from its head to
	// the outlet, the way with the most water at every fork; Flow the water
	// at the outlet in cubic metres a second.
	Outlet int32
	Trunk  []int32
	Flow   float64

	// A lake: Lake is its index in Grid.Lakes.
	Lake int32

	// A plate: Number is the number its tiles carry.
	Number uint8

	// A climate region: Group is Köppen's letter.
	Group byte
}

// Features is a map's registry: every feature by id, and which feature of
// each kind each tile belongs to.
type Features struct {
	// All is every feature, All[id-1].
	All []Feature
	// belt, basin and climate are each tile's feature of that kind, or 0;
	// lake is each of Grid.Lakes' feature, or 0 for a dry one; plate is
	// each plate number's.
	belt, basin, climate []FeatureID
	lake                 []FeatureID
	plate                [plateCap]FeatureID
	// tiles and trunks back every feature's Tiles and Trunk, one slice each
	// rather than one a feature.
	tiles, trunks []int32
}

// namer is what names the features, if anything does. See SetNamer.
var namer func(Feature) string

// SetNamer sets what names a feature. It is called once for each feature
// when a registry is built - at the end of making a world and after each
// Erode - with the feature filled in but for its Name, and what it returns
// is the name. Nil, the default, names nothing. A namer that is to name a
// world is set before the world is made.
func SetNamer(f func(Feature) string) { namer = f }

// Features is the registry of the things the tiles make up, or nil before
// the land is made.
func (g *Grid) Features() *Features { return g.features }

// Feature is the feature with the given id, or nil.
func (g *Grid) Feature(id FeatureID) *Feature {
	f := g.features
	if f == nil || id <= 0 || int(id) > len(f.All) {
		return nil
	}
	return &f.All[id-1]
}

// FeaturesAt is every feature the tile at p is part of: its belt, its
// basin, its lake, its plate and its climate region, in that order, each
// only where it has one.
func (g *Grid) FeaturesAt(p geom.Pos) []FeatureID {
	if g.features == nil || !g.In(p) {
		return nil
	}
	i := g.Index(p)
	var ids []FeatureID
	for k := UpliftBelt; k < featureKinds; k++ {
		if id := g.featureAt(i, k); id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

// featureAt is tile i's feature of kind k, or 0.
func (g *Grid) featureAt(i int, k FeatureKind) FeatureID {
	f := g.features
	if f == nil {
		return 0
	}
	switch k {
	case UpliftBelt:
		if i < len(f.belt) {
			return f.belt[i]
		}
	case DrainageBasin:
		if i < len(f.basin) {
			return f.basin[i]
		}
	case StandingLake:
		if i < len(g.lakeOf) && g.lakeOf[i] >= 0 && int(g.lakeOf[i]) < len(f.lake) {
			return f.lake[g.lakeOf[i]]
		}
	case CrustPlate:
		if g.ledger != nil {
			return f.plate[g.Tiles[i].Plate]
		}
	case ClimateRegion:
		if i < len(f.climate) {
			return f.climate[i]
		}
	}
	return 0
}

// PlateOf is the feature of the plate that number k has become part of:
// the plate itself, or the one it was welded into. See Plate.into.
func (g *Grid) PlateOf(k uint8) FeatureID {
	if g.features == nil || k == noPlate {
		return 0
	}
	return g.features.plate[g.rootPlate(k)]
}

// readFeatures builds the registry over the ground as it now lies. It runs
// on one goroutine in tile order, so that the ids are the same on every
// run: see the file comment.
func (g *Grid) readFeatures() {
	defer phase("readFeatures")()
	f := &Features{}
	n := len(g.Tiles)
	stack := make([]int32, 0, 1024)

	// Belts, where there is a book: the tiles one meeting raised, joined
	// where they touch.
	if g.ledger != nil {
		f.belt = make([]FeatureID, n)
		f.All, stack = g.components(f.All, f.belt, stack, func(i int) (uint32, bool) {
			l := &g.ledger[i]
			if l.raised() == NoMeeting {
				return 0, false
			}
			// The pair, the kind and the epoch: one pair of plates can be
			// closing at one end of its seam and parting at the other, and
			// an arc and a rift are not one meeting.
			return uint32(l.plates[0])<<24 | uint32(l.plates[1])<<16 | uint32(l.raised())<<8 | uint32(l.epoch), true
		}, func(fe *Feature, i int) {
			l := &g.ledger[i]
			fe.Kind = UpliftBelt
			fe.Plates, fe.Meeting, fe.Epoch = l.plates, l.raised(), l.epoch
		})
		for i := range g.Tiles {
			if id := f.belt[i]; id > 0 {
				fe := &f.All[id-1]
				l := &g.ledger[i]
				if abs32(l.lift) > abs32(float32(fe.Lift)) {
					fe.Lift = float64(l.lift)
				}
				if fe.Count == 0 || g.Height[i] > fe.Height {
					fe.Top, fe.Height = int32(i), g.Height[i]
				}
				fe.Count++
			}
		}
	}

	// Basins: the route trees, by where their water ends.
	f.basin = make([]FeatureID, n)
	if len(g.down) == n {
		f.All, stack = g.readBasins(f, f.All, stack)
	}

	// Lakes, from the drainage's own list, in the order of their lowest tile.
	f.lake = make([]FeatureID, len(g.Lakes))
	first := make([]int32, len(g.Lakes))
	for k := range first {
		first[k] = -1
	}
	for i := n - 1; i >= 0; i-- {
		if k := g.lakeOf[i]; k >= 0 {
			first[k] = int32(i)
		}
	}
	order := make([]int32, 0, len(g.Lakes))
	for k := range g.Lakes {
		if first[k] >= 0 {
			order = append(order, int32(k))
		}
	}
	slices.SortFunc(order, func(a, b int32) int { return int(first[a] - first[b]) })
	for _, k := range order {
		f.All = append(f.All, Feature{Kind: StandingLake, Lake: k, First: first[k], Count: g.Lakes[k].Tiles})
		f.lake[k] = FeatureID(len(f.All))
	}

	// Plates, where there is a history: one for each number the tiles carry.
	if g.ledger != nil {
		var count [plateCap]int
		var plateFirst [plateCap]int32
		for i := n - 1; i >= 0; i-- {
			k := g.Tiles[i].Plate
			count[k]++
			plateFirst[k] = int32(i)
		}
		order = order[:0]
		for k := range count {
			if count[k] > 0 {
				order = append(order, int32(k))
			}
		}
		slices.SortFunc(order, func(a, b int32) int { return int(plateFirst[a] - plateFirst[b]) })
		for _, k := range order {
			f.All = append(f.All, Feature{Kind: CrustPlate, Number: uint8(k), First: plateFirst[k], Count: count[k]})
			f.plate[k] = FeatureID(len(f.All))
		}
	}

	// Climate regions: dry land of one Köppen group, joined where it
	// touches. The group is read the way cmd/overview reads it, which asks
	// the tile's year; a world with no year has no climate.
	if len(g.warm) == n && len(g.rain) == n {
		f.climate = make([]FeatureID, n)
		f.All, stack = g.components(f.All, f.climate, stack, func(i int) (uint32, bool) {
			t := &g.Tiles[i]
			if t.Wet() || t.Terrain == Flat {
				return 0, false
			}
			return uint32(g.koppenGroup(i)), true
		}, func(fe *Feature, i int) {
			fe.Kind, fe.Group = ClimateRegion, g.koppenGroup(i)
		})
	}

	// Every feature's tiles, lowest first, out of one slice. A plate's are
	// not listed.
	for id := range f.All {
		f.All[id].ID = FeatureID(id + 1)
	}
	counts := make([]int32, len(f.All)+1)
	each := func(label []FeatureID, visit func(id FeatureID, i int)) {
		for i, id := range label {
			if id > 0 {
				visit(id, i)
			}
		}
	}
	tally := func(id FeatureID, i int) { counts[id]++ }
	each(f.belt, tally)
	each(f.basin, tally)
	each(f.climate, tally)
	for i := range g.Tiles {
		if k := g.lakeOf[i]; k >= 0 && int(k) < len(f.lake) && f.lake[k] > 0 {
			counts[f.lake[k]]++
		}
	}
	total := int32(0)
	for id := range counts {
		total += counts[id]
	}
	f.tiles = make([]int32, total)
	at := make([]int32, len(f.All)+1)
	next := int32(0)
	for id := 1; id < len(counts); id++ {
		at[id] = next
		fe := &f.All[id-1]
		if fe.Kind != CrustPlate {
			fe.Tiles = f.tiles[next : next+counts[id] : next+counts[id]]
			fe.Count = int(counts[id])
		}
		next += counts[id]
	}
	fill := func(id FeatureID, i int) { f.tiles[at[id]] = int32(i); at[id]++ }
	each(f.belt, fill)
	each(f.basin, fill)
	each(f.climate, fill)
	for i := range g.Tiles {
		if k := g.lakeOf[i]; k >= 0 && int(k) < len(f.lake) && f.lake[k] > 0 {
			fill(f.lake[k], i)
		}
	}

	if namer != nil {
		for id := range f.All {
			f.All[id].Name = namer(f.All[id])
		}
	}
	g.features = f
}

// components labels the tiles that key says are something with the
// connected runs of one key, eight ways round and round the map where it
// wraps, in tile order, so that each feature's id is the order of its lowest
// tile. begin fills in the kind and the numbers of a new feature from its
// first tile.
func (g *Grid) components(all []Feature, label []FeatureID, stack []int32,
	key func(i int) (uint32, bool), begin func(fe *Feature, i int)) ([]Feature, []int32) {
	for i := range g.Tiles {
		if label[i] != 0 {
			continue
		}
		k, ok := key(i)
		if !ok {
			continue
		}
		// Appended first and filled in in place, so that nothing is made on
		// the heap for each one.
		all = append(all, Feature{First: int32(i)})
		id := FeatureID(len(all))
		begin(&all[id-1], i)
		label[i] = id
		stack = append(stack[:0], int32(i))
		for len(stack) > 0 {
			j := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			p := g.PosOf(int(j))
			for _, off := range Dirs {
				q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
				if g.Wrap {
					q = g.Norm(q)
				}
				if !g.In(q) {
					continue
				}
				m := g.Index(q)
				if label[m] != 0 {
					continue
				}
				if km, ok := key(m); !ok || km != k {
					continue
				}
				label[m] = id
				stack = append(stack, int32(m))
			}
		}
	}
	return all, stack
}

// readBasins joins the tiles by where their water ends, and finds each
// basin's trunk: from the outlet up, the way with the most water at every
// fork. The tiles under the sea are nobody's; a river's mouth is the sea
// tile its last tile of land flows into.
func (g *Grid) readBasins(f *Features, all []Feature, stack []int32) ([]Feature, []int32) {
	n := len(g.Tiles)
	// Where each tile's water ends: followed down once, with every tile on
	// the way written as it is found. A closed lake ends at its lowest
	// tile, whichever of its tiles the water reached, so that the lake is
	// one basin's end and not one a tile.
	lakeFirst := make([]int32, len(g.Lakes))
	for k := range lakeFirst {
		lakeFirst[k] = -1
	}
	for i := n - 1; i >= 0; i-- {
		if k := g.lakeOf[i]; k >= 0 && g.Lakes[k].Closed {
			lakeFirst[k] = int32(i)
		}
	}
	end := make([]int32, n)
	for i := range end {
		end[i] = -1
	}
	for i := 0; i < n; i++ {
		if end[i] >= 0 {
			continue
		}
		stack = stack[:0]
		j := int32(i)
		for end[j] < 0 && g.down[j] >= 0 {
			stack = append(stack, j)
			j = g.down[j]
		}
		r := end[j]
		if r < 0 {
			r = j
			if k := g.lakeOf[j]; k >= 0 && lakeFirst[k] >= 0 {
				r = lakeFirst[k]
			}
			end[j] = r
		}
		for _, s := range stack {
			end[s] = r
		}
	}

	// A basin for each end that has ground draining to it, numbered by its
	// lowest tile of ground.
	of := make([]FeatureID, n) // each end's basin
	for i := 0; i < n; i++ {
		if g.underSea(i) {
			continue
		}
		r := end[i]
		if of[r] == 0 {
			all = append(all, Feature{Kind: DrainageBasin, Outlet: r, First: int32(i), Flow: g.Flow[r]})
			of[r] = FeatureID(len(all))
		}
		f.basin[i] = of[r]
	}

	// The tributaries of every tile, in tile order, for the trunks.
	head := make([]int32, n+1)
	for i := range g.Tiles {
		if d := g.down[i]; d >= 0 {
			head[d+1]++
		}
	}
	for i := 1; i <= n; i++ {
		head[i] += head[i-1]
	}
	into := make([]int32, head[n])
	at := slices.Clone(head[:n])
	for i := range g.Tiles {
		if d := g.down[i]; d >= 0 {
			into[at[d]] = int32(i)
			at[d]++
		}
	}
	// Each trunk, from the outlet up and then turned round, out of one
	// slice: the offsets are kept while the slice may still grow.
	starts := make([][2]int32, 0, 64)
	ids := make([]FeatureID, 0, 64)
	for r := 0; r < n; r++ {
		id := of[r]
		if id == 0 {
			continue
		}
		start := int32(len(f.trunks))
		cur := int32(r)
		f.trunks = append(f.trunks, cur)
		for {
			best, most := int32(-1), -1.0
			for _, t := range into[head[cur]:head[cur+1]] {
				if g.Flow[t] > most {
					best, most = t, g.Flow[t]
				}
			}
			if best < 0 {
				break
			}
			f.trunks = append(f.trunks, best)
			cur = best
		}
		slices.Reverse(f.trunks[start:])
		starts = append(starts, [2]int32{start, int32(len(f.trunks))})
		ids = append(ids, id)
	}
	for k, id := range ids {
		s := starts[k]
		all[id-1].Trunk = f.trunks[s[0]:s[1]:s[1]]
	}
	return all, stack
}
