package main

import (
	"fmt"
	"image/color"
	"math"
	"sort"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/terra/geom"
)

// The ground under the soil map's one number. A world made from its history
// keeps how deep its soil is, how long its surface has been forming soil, and
// what that time has made of it - the bases washed out, the lime and the salt
// the dry years leave, the carbon what grows puts in (see pedogenesis.go) -
// and a world drawn rather than made keeps only the depth. These are those,
// drawn, and the full Köppen–Geiger type the biome map folds together.

// A soil is the one thing that most marks a tile's soil, the way a soil
// survey's first division does: a salt crust, a lime pan, the bases washed
// out of it, or none of those.
const (
	sNone uint8 = iota // water, ice, the tide's mud and bare ground
	sBaseRich
	sLeached
	sStrongly
	sCalcic
	sSaline
	soilKinds
)

var soilKind = [soilKinds]class{
	sNone:     {"no soil forming", color.RGBA{60, 64, 72, 255}},
	sBaseRich: {"base-rich", color.RGBA{150, 132, 86, 255}},
	sLeached:  {"leached", color.RGBA{206, 146, 88, 255}},
	sStrongly: {"strongly leached", color.RGBA{172, 72, 40, 255}},
	sCalcic:   {"calcic", color.RGBA{230, 216, 168, 255}},
	sSaline:   {"saline", color.RGBA{236, 226, 240, 255}},
}

// Where each reading starts to mark a soil. A salt crust of a kilogram a
// square metre is a solonchak's; 25 kg of carbonate a square metre is a
// calcic horizon half a metre down at a twentieth of the soil; and a soil
// with half its bases gone is an acrisol's or a podzol's, a fifth a luvisol's.
const (
	salineAt   = 1.0
	calcicAt   = 25.0
	leachedAt  = 0.2
	stronglyAt = 0.5
)

// soilOf is the kind of soil on t.
func soilOf(t *terra.Tile) uint8 {
	switch {
	case t.Wet() || t.Exposed <= 0:
		return sNone
	case t.Salinity() >= salineAt:
		return sSaline
	case t.Carbonate() >= calcicAt:
		return sCalcic
	case t.Leaching() >= stronglyAt:
		return sStrongly
	case t.Leaching() >= leachedAt:
		return sLeached
	}
	return sBaseRich
}

// soilDrawings are the soil's depth, its kind, its carbon and its age.
func soilDrawings(land *terra.Land, shade func(geom.Pos) float64) []drawing {
	g := land.Grid
	wet := color.RGBA{40, 70, 110, 255}

	kinds := make([]uint8, len(g.Tiles))
	dry, deepest := 0, 0.0
	for i := range g.Tiles {
		kinds[i] = soilOf(&g.Tiles[i])
		if !g.Tiles[i].Wet() {
			dry++
			deepest = math.Max(deepest, float64(g.Soil[i]))
		}
	}
	var legend []share
	tally := make([]int, soilKinds)
	for i, k := range kinds {
		if !g.Tiles[i].Wet() {
			tally[k]++
		}
	}
	for k, c := range tally {
		if c > 0 {
			legend = append(legend, shareOf(soilKind[k].name, c, dry, hex(soilKind[k].col)))
		}
	}
	sort.SliceStable(legend, func(a, b int) bool { return legend[a].Count > legend[b].Count })

	return []drawing{
		{
			file: "soil-depth", title: "Soil depth",
			about: fmt.Sprintf("Metres of soil over the rock, on a root scale from bare rock (grey) to %s m (dark brown), hillshaded. Water dark.", sig(math.Max(deepest, 0.01))),
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				if t.Wet() {
					return wet
				}
				d := float64(g.Soil[i])
				if d < 0.01 {
					return scaleRGB(color.RGBA{150, 146, 140, 255}, shade(p))
				}
				return scaleRGB(ramp(depth, math.Sqrt(d/math.Max(deepest, 0.01))), shade(p))
			},
		},
		{
			file: "soil-kind", title: "Soil chemistry", legend: legend,
			about: fmt.Sprintf("What time has made of the soil, by what most marks it: a salt crust over %s kg/m², a lime horizon over %s kg/m², or the bases the water has carried off, a fifth of them (leached) or half (strongly leached). Base-rich soils have kept theirs. Only a world made from its history keeps this; the shares are of dry land.", sig(salineAt), sig(calcicAt)),
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				if t.Wet() {
					return wet
				}
				return scaleRGB(soilKind[kinds[i]].col, shade(p))
			},
		},
		{
			file: "soil-carbon", title: "Soil carbon",
			about: "Organic carbon in the soil, from none (pale) to 30 kg/m² (black), on a root scale: what grows puts it in and the warmth takes it back out, so it is deepest where it is wet and cool. Only a world made from its history keeps this.",
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				if t.Wet() {
					return wet
				}
				return scaleRGB(ramp(humus, math.Sqrt(float64(t.Carbon)/30)), shade(p))
			},
		},
		{
			file: "surface-age", title: "Surface age",
			about: "How long each tile's surface has been forming soil, on a log scale from a century (dark) to a million years (bright): the time since water, a slide or the cutting of a river last laid it bare or buried it. Grey where no soil is forming. Only a world made from its history keeps this.",
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				if t.Wet() {
					return wet
				}
				if t.Exposed <= 0 {
					return scaleRGB(color.RGBA{128, 128, 128, 255}, shade(p))
				}
				return scaleRGB(ramp(magma, (math.Log10(math.Max(float64(t.Exposed), 1))-2)/4), shade(p))
			},
		},
	}
}

// koppenColor is each Köppen–Geiger type in the colours Beck and others
// (2018) drew the world's in, which is how the type is usually seen.
var koppenColor = map[string]color.RGBA{
	"Af": {0, 0, 255, 255}, "Am": {0, 120, 255, 255}, "Aw": {70, 170, 250, 255},
	"BWh": {255, 0, 0, 255}, "BWk": {255, 150, 150, 255}, "BSh": {245, 165, 0, 255}, "BSk": {255, 220, 100, 255},
	"Csa": {255, 255, 0, 255}, "Csb": {200, 200, 0, 255}, "Csc": {150, 150, 0, 255},
	"Cwa": {150, 255, 150, 255}, "Cwb": {100, 200, 100, 255}, "Cwc": {50, 150, 50, 255},
	"Cfa": {200, 255, 80, 255}, "Cfb": {100, 255, 80, 255}, "Cfc": {50, 200, 0, 255},
	"Dsa": {255, 0, 255, 255}, "Dsb": {200, 0, 200, 255}, "Dsc": {150, 50, 150, 255}, "Dsd": {150, 100, 150, 255},
	"Dwa": {170, 175, 255, 255}, "Dwb": {90, 120, 220, 255}, "Dwc": {75, 80, 180, 255}, "Dwd": {50, 0, 135, 255},
	"Dfa": {0, 255, 255, 255}, "Dfb": {55, 200, 255, 255}, "Dfc": {0, 125, 125, 255}, "Dfd": {0, 70, 95, 255},
	"ET": {178, 178, 178, 255}, "EF": {102, 102, 102, 255},
}

// koppenDrawing is every dry tile's full Köppen–Geiger type.
func koppenDrawing(land *terra.Land, cls classes, shade func(geom.Pos) float64) drawing {
	tally := map[string]int{}
	dry := 0
	for _, k := range cls.Koppen {
		if k != "" {
			tally[k]++
			dry++
		}
	}
	var legend []share
	for k, c := range tally {
		legend = append(legend, shareOf(k, c, dry, hex(koppenColor[k])))
	}
	sort.Slice(legend, func(a, b int) bool {
		if legend[a].Count != legend[b].Count {
			return legend[a].Count > legend[b].Count
		}
		return legend[a].Name < legend[b].Name
	})
	return drawing{
		file: "koppen", title: "Köppen–Geiger", legend: legend,
		about: "Each dry tile's full Köppen–Geiger type, in the colours Beck and others (2018) map the real world's in: A tropical in blue, B dry in red and yellow, C temperate in yellow-green, D continental in violet and cyan, E polar in grey. The biome map folds these into fourteen. Shares are of the dry land's tiles. Water dark, salt flats and tidal mud pale.",
		color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
			k := cls.Koppen[i]
			if k == "" {
				if t.Wet() {
					return color.RGBA{36, 40, 52, 255}
				}
				return color.RGBA{220, 216, 204, 255}
			}
			return scaleRGB(koppenColor[k], 0.4+0.6*shade(p))
		},
	}
}

var (
	depth = []color.RGBA{{226, 214, 176, 255}, {180, 140, 90, 255}, {110, 72, 42, 255}, {60, 36, 22, 255}}
	humus = []color.RGBA{{238, 232, 212, 255}, {170, 150, 110, 255}, {90, 70, 50, 255}, {24, 20, 18, 255}}
)
