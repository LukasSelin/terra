package terra

// Reading out what the map keeps in shapes of its own.
//
// The beds under a tile, its line in the book a history keeps and the
// features it belongs to are held the way the passes that use them want
// them, and those shapes change when the passes do. These read them out in
// shapes that stay put, for a program that keeps or draws a world rather
// than runs one - cmd/zarr, for one. They copy, and change nothing.

// Bed is one bed of the pile of rock under a tile. See strata.go.
type Bed struct {
	// Top is the height of the bed's upper surface, in the metres the ground
	// is measured in. The last bed, the basement, has no floor.
	Top  float64
	Rock Bedrock
	// Formed is the epoch the bed was laid in.
	Formed uint8
	// Sand is the share of sand in a bed the water laid, out of 255.
	Sand uint8
}

// BedsMax is the most beds a tile's pile keeps.
const BedsMax = bedsMax

// AppendBeds appends the beds under tile i to dst, from the top down. A map
// made without a pile of beds has none, and its rock is the tile's Bedrock.
func (g *Grid) AppendBeds(dst []Bed, i int) []Bed {
	if g.strata == nil {
		return dst
	}
	c := &g.strata[i]
	for k := 0; k < int(c.n); k++ {
		dst = append(dst, Bed{Top: float64(c.top[k]), Rock: c.rock[k], Formed: c.formed[k], Sand: c.sand[k]})
	}
	return dst
}

// Burial is what last buried a tile in its history.
type Burial uint8

const (
	Unburied     = Burial(unburied)
	BuriedByFill = Burial(byFill)
	BuriedByMud  = Burial(byMud)
	BuriedByLime = Burial(byLime)
	BuriedByLava = Burial(byLava)
)

func (b Burial) String() string {
	if int(b) < len(buriedNames) {
		return buriedNames[b]
	}
	return "burial?"
}

// NoPlate is the second of a Record's Plates where a hotspot raised the
// tile from under one plate alone.
const NoPlate = noPlate

// Record is a tile's line in the book a history keeps. See ledger.go.
type Record struct {
	// Lift is the metres the meeting that did most to the tile's height
	// raised it by in its epoch, negative where a rift dropped it, and Worn
	// the metres the weather has taken off the tile since, to the nearest
	// ten. Both are the history's metres, not the map's: see ledger.
	Lift, Worn float64
	// Plates are the two plates that meeting was between, as they were
	// numbered in its epoch; see Grid.PlateOf for what they have become.
	Plates  [2]uint8
	Meeting MeetingKind
	Epoch   uint8
	// Burial is what last buried the tile, and BuriedIn the epoch it did.
	Burial   Burial
	BuriedIn uint8
}

// Record is tile i's line in the book its history kept, and false on a map
// that was drawn and keeps none.
func (g *Grid) Record(i int) (Record, bool) {
	if g.ledger == nil {
		return Record{}, false
	}
	l := &g.ledger[i]
	return Record{
		Lift:     float64(l.lift),
		Worn:     l.wornMetres(),
		Plates:   l.plates,
		Meeting:  l.raised(),
		Epoch:    l.epoch,
		Burial:   Burial(l.buried()),
		BuriedIn: l.buriedIn,
	}, true
}

// FeatureOf is the feature of kind k that tile i belongs to, or 0 if none.
func (g *Grid) FeatureOf(i int, k FeatureKind) FeatureID { return g.featureAt(i, k) }
