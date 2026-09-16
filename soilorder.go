package terra

import "math"

// The soil orders.
//
// Soil Taxonomy (Soil Survey Staff 1999) sorts the world's soils into twelve
// orders by a key: each order is a question asked of the soil in turn, and a
// soil is the first order it answers yes to. The questions are about horizons
// a pit shows - a frozen layer, a peat, a dark topsoil rich in bases, a subsoil
// the rain has stripped to the oxides - and a tile has no pit. What it has is
// what those horizons are made by (pedogenesis.go): how long its surface has
// been forming soil, how much of its bases the water has taken, how much of
// its weatherable minerals are left, the carbonate, the salt and the carbon it
// holds, and the climate it lies under. So each question is asked of those.
//
// The key is asked in Soil Taxonomy's order, less the orders a tile carries
// nothing to tell apart: Spodosols (the iron and humus carried down under
// conifers), Andisols (volcanic glass) and Vertisols (clays that swell and
// crack).
type SoilOrder uint8

const (
	// NoSoil is ground with no soil to classify: an outcrop, a salt pan, or
	// ground the water and the slides have left bare.
	NoSoil SoilOrder = iota
	Gelisol
	Histosol
	Oxisol
	Aridisol
	Ultisol
	Mollisol
	Alfisol
	Inceptisol
	Entisol
	// SoilOrderCount is how many there are, NoSoil among them.
	SoilOrderCount
)

var soilOrderNames = [SoilOrderCount]string{
	NoSoil: "no soil", Gelisol: "Gelisol", Histosol: "Histosol", Oxisol: "Oxisol",
	Aridisol: "Aridisol", Ultisol: "Ultisol", Mollisol: "Mollisol", Alfisol: "Alfisol",
	Inceptisol: "Inceptisol", Entisol: "Entisol",
}

func (o SoilOrder) String() string {
	if o >= SoilOrderCount {
		return "unknown"
	}
	return soilOrderNames[o]
}

// What the key asks, as a tile carries it.
//
// Aridisols are the soils of the aridic moisture regime with a horizon to show
// for it: the UNEP line of the arid, rain under a fifth of what the air could
// take, and a calcic or salic horizon or a surface old enough to have made any
// subsoil at all, which the key counts. A calcic horizon is fifteen centimetres holding fifteen per cent
// carbonate, some 30 kilograms a square metre, and a salic one two per cent
// salt, some 4.5.
//
// An oxic horizon is one the weathering has left under a tenth weatherable
// minerals. A rock is some two fifths of them, so it is read where a quarter of
// what the rock had is left, under the hot humid sky Oxisols form under: the
// hyperthermic line of 22 degrees, and at least half the rain the air could
// take.
//
// An Ultisol's subsoil holds under 35 per cent of its bases, and an Alfisol's
// more; both have a clay subsoil the water has carried down, which takes some
// ten thousand years to show.
//
// A mollic epipedon is a dark topsoil, 25 centimetres or more, holding half its
// bases or more: the prairies' and the steppes'. Dark is read as more carbon in
// the top metre than a boreal forest holds, and it is asked for outside the
// hyperthermic regime, where the heat and the rain leave the same grass over a
// paler soil the key puts with the Alfisols and Ultisols (Buol and others
// 2011).
//
// A Histosol is a peat: ground waterlogged the year round holding the carbon
// that builds. An Inceptisol has a subsoil at all, a thousand years of it; an
// Entisol has nothing yet but its parent material.
const (
	aridicWetness   = 0.2
	calcicCarbonate = 30.0 // kg/m²
	salicSalt       = 4.5  // kg/m²
	subsoilYears    = 10e3
	oxicMinerals    = 0.25
	oxicWarmth      = 22.0 // °C: the hyperthermic line
	oxicWetness     = 0.5
	ultisolLeaching = 0.65
	mollicDepth     = 0.25 // m
	mollicLeaching  = 0.5
	mollicCarbon    = 9.0  // kg C/m²
	histicCarbon    = 20.0 // kg C/m²
	histicSodden    = 0.75
	cambicYears     = 1e3
)

// SoilOrderOf is the order of the soil on tile i.
func (g *Grid) SoilOrderOf(i int) SoilOrder {
	t := &g.Tiles[i]
	if !forms(t) || g.Soil[i] <= 0 {
		return NoSoil
	}
	c := g.pedoClimateOf(i)
	age := float64(t.Exposed)
	switch {
	case c.frozen:
		return Gelisol
	case c.sodden >= histicSodden && float64(t.Carbon) >= histicCarbon:
		return Histosol
	case c.temp >= oxicWarmth && c.wetness >= oxicWetness &&
		math.Exp(-c.weathering*age/mineralYears) < oxicMinerals:
		return Oxisol
	case c.wetness < aridicWetness &&
		(t.Carbonate() >= calcicCarbonate || t.Salinity() >= salicSalt || age >= cambicYears):
		return Aridisol
	case t.Leaching() >= ultisolLeaching && age >= subsoilYears:
		return Ultisol
	case c.temp < oxicWarmth && c.wetness >= aridicWetness && float64(g.Soil[i]) >= mollicDepth &&
		t.Leaching() < mollicLeaching && float64(t.Carbon) >= mollicCarbon:
		return Mollisol
	case age >= subsoilYears:
		return Alfisol
	case age >= cambicYears:
		return Inceptisol
	}
	return Entisol
}
