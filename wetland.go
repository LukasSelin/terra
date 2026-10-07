package terra

import (
	"math"

	"github.com/LukasSelin/terra/internal/atmos"
)

// The wet ground, and the peat it lays down.
//
// A wetland was ground beside a river, drawn so by the overview, and a peat
// was a soil order's test: ground within a metre or two of its water holding
// twenty kilograms of carbon. Neither was a thing the ground did.
//
// Wetlands. Water gathers where much ground drains through a place and the
// place is too flat to send it on. TOPMODEL (Beven and Kirkby, 1979) reads
// that off the topographic wetness index λ = ln(a/tan β) - see twi - with a
// the ground draining through a place per metre of its width and β its
// slope: the water coming down through the ground to a place, a·r for a
// recharge r, is more than the ground there can pass on, T·tan β for a
// transmissivity T, where λ ≥ ln(T/r), and there the water table stands at
// the surface (Beven, 1997). The global wetland models read their wetlands'
// extent the same way, off the water of a grid cell and its spread of the
// index (Kleinen and others, 2012; Stocker and others, 2014). Here the
// recharge is what the soil's bucket sheds in each phase of the year
// (soilwater.go), the rain and the melt it cannot hold, and a tile is a
// wetland when its water table reaches the surface for wetlandSeason of its
// thawed year.
//
// T is the ground's conductivity through the depth that drains: drainK
// through drainDepth, the weathered layer the soil is made from. Permafrost
// blocks the drainage. Only the active layer over it drains, so over frozen
// ground T is the active layer's share of it, and the water of the thaw
// stands on the flats of the tundra where a deeper ground would have taken
// it: the polygons and thaw ponds of the Arctic lowlands.
//
// Peat. What a waterlogged ground grows is laid down faster than it rots: the
// living layer at the top, the acrotelm, is aired as the water table falls
// and rises, and most of what grows rots in it; what gets through to the
// waterlogged catotelm under it rots a thousand times more slowly. Clymo
// (1984) wrote the catotelm's carbon as dM/dt = p − αM, a steady passage p
// into it and a slow decay α of all of it, which comes to p/α and gets there
// as e^(−αt). Here p is peatPass of the tile's NPP (vegetation.go), as much
// of its thawed year as it is waterlogged past wetlandSeason; both the acrotelm's
// rot, which lets less through, and α go up by peatQ10 every ten degrees,
// the soil's carbon's figure across the world's sites (see carbonQ10); and α
// is slowed on permafrost as the soil's carbon is (carbonFrozen). Ground that
// is no longer wet loses its peat peatDrained times as fast, as a drained
// peat does.
//
// It rises, and holds its water. A peat's catotelm hardly lets water through
// at all, so the water table in a peat stands in its top, held there by the
// peat itself, however the ground under it drains (Ingram, 1982; Frolking and
// others, 2010): the ground that drains under a peat thicker than its
// acrotelm is the acrotelm and no more, and a peat once laid keeps itself wet. Its surface
// stands its depth over the mineral ground; that is kept as the peat's depth
// and not laid on the heights the rivers are worked out on.
//
// The peats of the world began when the ice and the cold of the last glacial
// let them: northern peats date from the Holocene, mostly from eight to
// twelve thousand years ago (MacDonald and others, 2006). A map is laid with
// its peat as peatYears would have left it, or as long as its surface has
// been forming soil where that is less.

// The water table. drainK is the saturated conductivity of the ground that
// drains, in metres a day: a loam's is some tenth of a metre to a metre a
// day, and a weathered regolith's with its root channels and cracks more
// (Beven, 1997, fits T of a few to some tens of square metres a day over
// whole catchments). wetlandSeason is the share of the thawed year a
// wetland's water table stands at its surface: half of it, chosen.
const drainK = 1.0 // m/day

const (
	drainDepth    = regolithDepth
	wetlandSeason = 0.25
	peatSeason    = 0.75
)

// The peat. peatPass is the share of what grows that gets through the
// acrotelm at nought degrees, and peatDecay the catotelm's α there: Clymo
// (1984) has the catotelm's decay at a few in a hundred thousand to a few in
// ten thousand a year, and some tenth of the acrotelm's production reaching
// it. Together they put a boreal bog's long-term accumulation at Yu and
// others' (2010) mean for the northern peatlands, 18.6 grams of carbon a
// square metre a year. peatDensity is the carbon in a cubic metre of peat
// (Loisel and others, 2014: 0.118 g/cm³ of peat, 0.052 of it carbon).
// acrotelm is the depth of the aired layer; histicPeat the depth of peat
// Soil Taxonomy keys as a Histosol, organic soil materials 40 centimetres or
// more of the top 80 (Soil Survey Staff, 1999), and what Yu and others
// count as a peatland.
const (
	peatPass    = 0.1
	peatDecay   = 1e-4 // a year
	peatQ10     = carbonQ10
	peatDrained = 10.0
	peatDensity = 52.0 // kg C/m³
	peatYears   = 11.7e3
	acrotelm    = 0.3 // m
	histicPeat  = 0.4 // m
)

// waterTable is the share of tile i's thawed year its water table stands at
// its surface, and false where it is not known: water, the tide's ground,
// bare stone and salt, and a map whose water has not been read.
func (g *Grid) waterTable(i int) (float64, bool) {
	n := len(g.Tiles)
	if i < 0 || i >= n || !forms(&g.Tiles[i]) || g.sunk(i) ||
		len(g.runoffIn) != n*atmos.Phases || len(g.warm) != n {
		return 0, false
	}
	// How deep the ground drains: the weathered layer, the active layer
	// over the share of it that is frozen, and a peat's acrotelm over a
	// peat.
	drained := drainDepth
	if f, ok := g.groundFrost(i); ok {
		s := frostShareOf(f.ttop)
		drained = (1-s)*drainDepth + s*math.Min(drainDepth, f.thaw)
	}
	if g.PeatDepth(i) >= acrotelm {
		drained = math.Min(drained, acrotelm)
	}
	// The water table is at the surface in a phase whose recharge, in metres
	// a day, is over T/e^λ.
	need := drainK * math.Max(drained, 1e-3) / math.Exp(g.twi(i))
	days := daysAYear / atmos.Phases
	mean, swing := g.meanOn(i, g.Height[i]), float64(g.swing[i])
	var thawed, wet float64
	for k := range atmos.Phases {
		if mean+swing*atmos.SummerPeak*phaseSin[k] <= 0 {
			continue
		}
		thawed++
		if float64(g.runoffIn[i*atmos.Phases+k])/1000/days >= need {
			wet++
		}
	}
	if thawed == 0 {
		return 0, true
	}
	return wet / thawed, true
}

// phaseSin is how far into the north's summer each phase of the year stands
// at its middle: see atmos.PhaseDegreeDays.
var phaseSin = [atmos.Phases]float64{-1, 0, 1, 0}

// Wetland reports whether tile i is a wetland: ground whose water table
// stands at its surface for wetlandSeason of its thawed year or more.
func (g *Grid) Wetland(i int) bool {
	s, ok := g.waterTable(i)
	return ok && s >= wetlandSeason
}

// WaterTable is the share of tile i's thawed year its water table stands at
// its surface: see waterTable. It is nothing where that is not known.
func (g *Grid) WaterTable(i int) float64 {
	s, _ := g.waterTable(i)
	return s
}

// PeatDepth is how deep the peat on tile i is, in metres: how far its surface
// stands over the mineral ground under it.
func (g *Grid) PeatDepth(i int) float64 {
	if i < 0 || i >= len(g.peat) {
		return 0
	}
	return float64(g.peat[i]) / peatDensity
}

// PeatCarbon is the carbon in tile i's peat, in kg a square metre.
func (g *Grid) PeatCarbon(i int) float64 {
	if i < 0 || i >= len(g.peat) {
		return 0
	}
	return float64(g.peat[i])
}

// PeatAge is how long tile i's peat has been laying down, in years: the age
// of its base.
func (g *Grid) PeatAge(i int) float64 {
	if i < 0 || i >= len(g.peatAge) {
		return 0
	}
	return float64(g.peatAge[i])
}

// Peatland reports whether tile i is a peatland: histicPeat of peat or more.
func (g *Grid) Peatland(i int) bool { return g.PeatDepth(i) >= histicPeat }

// nppOf is what tile i grows in a year, in kg C a square metre: its
// vegetation's, or the Miami model's for its climate where none has been
// laid (Lieth 1975: the lesser of its warmth's and its rain's terms, in
// grams of dry matter), at 0.45 of the dry matter carbon.
func (g *Grid) nppOf(i int, c pedoClimate) float64 {
	if len(g.npp) == len(g.Tiles) {
		return float64(g.npp[i])
	}
	warm := 3000 / (1 + math.Exp(1.315-0.119*c.temp))
	wet := 3000 * -math.Expm1(-0.000664*math.Max(0, c.rain))
	return 0.45 * math.Min(warm, wet) / 1000
}

// ripenPeat moves tile i's peat on by years under the water and the climate
// it has now: see the top of this file.
func (g *Grid) ripenPeat(i int, c pedoClimate, years float64) {
	if len(g.peat) != len(g.Tiles) {
		return
	}
	wet, ok := g.waterTable(i)
	if !ok {
		g.peat[i], g.peatAge[i] = 0, 0
		return
	}
	m, age := float64(g.peat[i]), float64(g.peatAge[i])
	if m == 0 {
		years = math.Min(years, peatYears) // a peat starts no earlier than the Holocene
	}
	q := math.Pow(peatQ10, c.temp/10)
	decay := peatDecay * q
	if f, ok := g.groundFrost(i); ok {
		decay *= 1 - frostShareOf(f.ttop)*(1-carbonFrozen)
	} else if c.frozen {
		decay *= carbonFrozen
	}
	if soaked := ramp(wet, peatSeason, 1); soaked > 0 {
		pass := g.nppOf(i, c) * peatPass * soaked / q
		m = toward(m, pass/decay, decay, years)
		if m > 0 {
			age += years
		}
	} else {
		m *= math.Exp(-decay * peatDrained * years)
	}
	if m < 1e-3 {
		m, age = 0, 0
	}
	g.peat[i], g.peatAge[i] = float32(m), float32(age)
}
