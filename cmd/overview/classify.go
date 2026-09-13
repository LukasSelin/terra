package main

import (
	"image/color"
	"math"
	"sort"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/terra/geom"
)

// The terrain a world is made with is what a game asks of a tile: can it be
// built on, walked over, cut for timber. It is not what a traveller would
// call the place. This file reads two such names off what the land already
// knows, and neither is written back to it - they are for looking at.
//
// A biome is what the weather makes of a tile: the year's mean temperature
// at its latitude and height against the rain that falls on it, the way
// Whittaker and Köppen divide the world. A landform is what the ground makes
// of it, and that cannot be read off one tile: a peak is a peak because the
// ground round it is lower, a valley because it is higher, a coast because
// the sea is next to it. Each is measured over the tile's neighbourhood.

// A class is one name a tile can be given, with the colour it is drawn in.
type class struct {
	name string
	col  color.RGBA
}

// Water is named the same way in both readings.
const (
	cDeep uint8 = iota
	cShelf
	cLake
	cRiver
	cSeaIce
	waterClasses
)

var waterClass = [waterClasses]class{
	cDeep:   {"deep sea", color.RGBA{30, 62, 118, 255}},
	cShelf:  {"shallow sea", color.RGBA{62, 120, 178, 255}},
	cLake:   {"lake", color.RGBA{70, 150, 190, 255}},
	cRiver:  {"river", color.RGBA{100, 180, 230, 255}},
	cSeaIce: {"sea ice", color.RGBA{214, 230, 242, 255}},
}

// Biomes, after the water.
const (
	bIceCap = waterClasses + iota
	bTundra
	bBoreal
	bColdSteppe
	bTemperateRain
	bTemperateForest
	bGrassland
	bColdDesert
	bRainforest
	bDryForest
	bSavanna
	bHotDesert
	bWetland
	biomeClasses
)

var biomeClass = [biomeClasses - waterClasses]class{
	bIceCap - waterClasses:          {"ice cap", color.RGBA{242, 245, 248, 255}},
	bTundra - waterClasses:          {"tundra", color.RGBA{168, 164, 136, 255}},
	bBoreal - waterClasses:          {"boreal forest", color.RGBA{64, 104, 86, 255}},
	bColdSteppe - waterClasses:      {"cold steppe", color.RGBA{178, 176, 138, 255}},
	bTemperateRain - waterClasses:   {"temperate rainforest", color.RGBA{34, 102, 76, 255}},
	bTemperateForest - waterClasses: {"temperate forest", color.RGBA{86, 142, 70, 255}},
	bGrassland - waterClasses:       {"grassland", color.RGBA{176, 196, 104, 255}},
	bColdDesert - waterClasses:      {"temperate desert", color.RGBA{206, 194, 152, 255}},
	bRainforest - waterClasses:      {"tropical rainforest", color.RGBA{22, 112, 42, 255}},
	bDryForest - waterClasses:       {"seasonal forest", color.RGBA{124, 152, 58, 255}},
	bSavanna - waterClasses:         {"savanna", color.RGBA{212, 190, 96, 255}},
	bHotDesert - waterClasses:       {"hot desert", color.RGBA{236, 208, 142, 255}},
	bWetland - waterClasses:         {"wetland", color.RGBA{92, 138, 118, 255}},
}

// Landforms, after the water.
const (
	fCoast = waterClasses + iota
	fCliff
	fFloodplain
	fPlain
	fUpland
	fHills
	fValley
	fPlateau
	fMountain
	fPeak
	formClasses
)

var formClass = [formClasses - waterClasses]class{
	fCoast - waterClasses:      {"coast", color.RGBA{232, 216, 160, 255}},
	fCliff - waterClasses:      {"sea cliff", color.RGBA{150, 84, 70, 255}},
	fFloodplain - waterClasses: {"floodplain", color.RGBA{120, 176, 120, 255}},
	fPlain - waterClasses:      {"lowland plain", color.RGBA{190, 214, 150, 255}},
	fUpland - waterClasses:     {"upland", color.RGBA{214, 206, 146, 255}},
	fHills - waterClasses:      {"hills", color.RGBA{190, 164, 102, 255}},
	fValley - waterClasses:     {"valley", color.RGBA{94, 140, 96, 255}},
	fPlateau - waterClasses:    {"plateau", color.RGBA{196, 132, 82, 255}},
	fMountain - waterClasses:   {"mountains", color.RGBA{128, 104, 92, 255}},
	fPeak - waterClasses:       {"peaks and ridges", color.RGBA{236, 232, 228, 255}},
}

func biomeOf(k uint8) class {
	if k < waterClasses {
		return waterClass[k]
	}
	return biomeClass[k-waterClasses]
}

func formOf(k uint8) class {
	if k < waterClasses {
		return waterClass[k]
	}
	return formClass[k-waterClasses]
}

// classes is every tile of a map named both ways.
type classes struct {
	Biome, Form []uint8
	// MeanTemp is the year's mean on the ground, in degrees, and LandDist
	// how many tiles off the nearest dry land a tile is.
	MeanTemp, LandDist []float64
	// Shelf is how many tiles out from land the sea is shallow.
	Shelf int
}

// seaColor is the colour of the open sea at tile i, deepening with the
// distance from land rather than stepping at the edge of the shelf.
func (c classes) seaColor(i int) color.RGBA {
	shallow, deep := waterClass[cShelf].col, waterClass[cDeep].col
	return lerpRGB(shallow, deep, (c.LandDist[i]-1)/float64(3*c.Shelf))
}

// classify names every tile of the land.
func classify(land *terra.Land) classes {
	g := land.Grid
	n := len(g.Tiles)
	c := classes{Biome: make([]uint8, n), Form: make([]uint8, n), MeanTemp: make([]float64, n)}

	// How far the neighbourhood reaches follows the size of the map, as the
	// land's own measures do: a hill on a valley is a mountain range on a
	// globe drawn at the same number of tiles.
	near := max(2, g.Span()/256)  // the lie of the ground right round a tile
	wide := max(6, g.Span()/64)   // the country it stands in
	shelf := max(3, g.Span()/128) // how far out the sea stays shallow
	c.Shelf = shelf

	wet := make([]float64, n)
	height := make([]float64, n)
	for i := range g.Tiles {
		t := &g.Tiles[i]
		height[i] = t.Height
		if t.Wet() {
			wet[i] = 1
		}
	}

	// A river is water running through land: a channel narrow enough that
	// most of what is round it is dry. The sea is the rest of the water where
	// it joins up with a great deal more of it; a lake is where it does not.
	wetNear := boxMean(wet, g.W, g.H, 2, g.Wrap)
	channel := func(i int) bool {
		t := &g.Tiles[i]
		return t.Terrain == terra.Water && t.Flow > riverFlow && wetNear[i] < 0.6
	}
	sea := seaOf(g, channel)

	// The year's mean on the ground is the latitude's, less what the height
	// takes off it, and warmed by the sea round about by the same measure
	// the frost is: see terra.Maritime.
	seaShare := make([]float64, n)
	for i := range sea {
		if sea[i] {
			seaShare[i] = 1
		}
	}
	seaShare = boxMean(seaShare, g.W, g.H, g.Span()/6, g.Wrap)
	for i := range g.Tiles {
		c.MeanTemp[i] = land.Climate.MeanAt(i/g.W) - terra.Lapse*height[i] + terra.Maritime*seaShare[i]
	}
	c.LandDist = distance(g, func(i int) bool { return !g.Tiles[i].Wet() })

	// A river floods the low ground either side of it, as far out as the
	// water it carries will spread: a great river has a broad floodplain and
	// a brook none, which is the difference between a plain combed with small
	// channels and a marsh.
	flood := make([]bool, n)
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if !t.Wet() {
			continue
		}
		switch {
		case t.Terrain == terra.Ice:
			c.Biome[i] = cSeaIce
		case channel(i):
			c.Biome[i] = cRiver
			floodOut(g, flood, g.PosOf(i), int(math.Sqrt(t.Flow)/floodReach))
		case !sea[i]:
			c.Biome[i] = cLake
		case c.LandDist[i] > float64(shelf):
			c.Biome[i] = cDeep
		default:
			c.Biome[i] = cShelf
		}
		c.Form[i] = c.Biome[i]
	}

	// The shape of the ground round each tile: how far it stands above the
	// ground near it, and above the country it stands in, and how rough that
	// country is.
	meanNear := boxMean(height, g.W, g.H, near, g.Wrap)
	meanWide := boxMean(height, g.W, g.H, wide, g.Wrap)
	sq := make([]float64, n)
	for i, h := range height {
		sq[i] = h * h
	}
	sqWide := boxMean(sq, g.W, g.H, wide, g.Wrap)

	relief := make([]float64, n)
	slope := make([]float64, n)
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			relief[i] = math.Sqrt(math.Max(0, sqWide[i]-meanWide[i]*meanWide[i]))
			slope[i] = g.Slope(g.PosOf(i))
		}
	})

	// What counts as high, rough or steep is a comparison with the rest of
	// the dry land, the way an outcrop is: a flat world still has its hills.
	var dryH, dryRelief, drySlope []float64
	for i := range g.Tiles {
		if !g.Tiles[i].Wet() {
			dryH = append(dryH, height[i])
			dryRelief = append(dryRelief, relief[i])
			drySlope = append(drySlope, slope[i])
		}
	}
	upAt, highAt := quant(dryH, 0.5), quant(dryH, 0.85)
	roughAt, veryRoughAt := quant(dryRelief, 0.55), quant(dryRelief, 0.8)
	flatAt, steepAt, cliffAt := quant(drySlope, 0.4), quant(drySlope, 0.7), quant(drySlope, 0.85)

	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			t := &g.Tiles[i]
			if t.Wet() {
				continue
			}
			p := g.PosOf(i)
			byRiver := flood[i] && t.Drain < terra.FloodDepth/2
			c.Biome[i] = biome(g, p, c.MeanTemp[i], g.Rain(i), byRiver)

			atSea := false
			for _, d := range terra.Dirs {
				q := g.Norm(geom.Pos{X: p.X + d.X, Y: p.Y + d.Y})
				if g.In(q) && sea[g.Index(q)] {
					atSea = true
					break
				}
			}
			above := height[i] - meanNear[i]
			spread := math.Max(relief[i], 1)
			high := height[i] >= highAt
			switch {
			case atSea && slope[i] >= cliffAt:
				c.Form[i] = fCliff
			case atSea:
				c.Form[i] = fCoast
			case high && above > 0.5*spread && relief[i] >= roughAt:
				c.Form[i] = fPeak
			case byRiver:
				c.Form[i] = fFloodplain
			case high && slope[i] < flatAt && math.Abs(above) < 0.25*spread:
				c.Form[i] = fPlateau
			case high:
				c.Form[i] = fMountain
			case above < -0.4*spread && relief[i] >= roughAt:
				c.Form[i] = fValley
			case relief[i] >= veryRoughAt || slope[i] >= steepAt:
				c.Form[i] = fHills
			case height[i] >= upAt:
				c.Form[i] = fUpland
			default:
				c.Form[i] = fPlain
			}
		}
	})
	return c
}

// biome is what the weather makes of dry ground: a year's mean of temp
// degrees and rain mm. The line between dry and not is Köppen's, which moves
// with the warmth because warm air takes more of the rain back.
func biome(g *terra.Grid, p geom.Pos, temp, rain float64, byRiver bool) uint8 {
	if g.Frozen(p) {
		if temp < terra.Bitter {
			return bIceCap
		}
		return bTundra
	}
	dry := 20 * (temp + 7) // below this, desert; below twice it, steppe
	if byRiver && rain >= dry {
		return bWetland
	}
	switch {
	case temp < 7:
		if rain < 1.5*dry {
			return bColdSteppe
		}
		return bBoreal
	case temp < 15:
		switch {
		case rain < dry:
			return bColdDesert
		case rain < 2*dry:
			return bGrassland
		case rain < 3.5*dry:
			return bTemperateForest
		}
		return bTemperateRain
	}
	switch {
	case rain < dry:
		return bHotDesert
	case rain < 2*dry:
		return bSavanna
	case rain < 3.5*dry:
		return bDryForest
	}
	return bRainforest
}

// floodReach is how many tiles out a river floods for each root of a cubic
// metre a second it carries: four spreads one tile, a hundred and fifty six.
const floodReach = 2.0

// floodOut marks the ground within r tiles of p as in reach of its river.
func floodOut(g *terra.Grid, flood []bool, p geom.Pos, r int) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy > r*r {
				continue
			}
			if q := g.Norm(geom.Pos{X: p.X + dx, Y: p.Y + dy}); g.In(q) {
				flood[g.Index(q)] = true
			}
		}
	}
}

// seaOf marks the water that belongs to a sea: any body of water, not
// counting the channels, holding at least a two-hundredth of the map. The
// rest is lakes.
func seaOf(g *terra.Grid, channel func(i int) bool) []bool {
	n := len(g.Tiles)
	seen := make([]bool, n)
	sea := make([]bool, n)
	var stack, body []int
	for s := range g.Tiles {
		if seen[s] || !g.Tiles[s].Wet() || channel(s) {
			continue
		}
		body = body[:0]
		stack = append(stack[:0], s)
		seen[s] = true
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			body = append(body, i)
			p := g.PosOf(i)
			for _, d := range terra.Dirs {
				q := g.Norm(geom.Pos{X: p.X + d.X, Y: p.Y + d.Y})
				if !g.In(q) {
					continue
				}
				j := g.Index(q)
				if !seen[j] && g.Tiles[j].Wet() && !channel(j) {
					seen[j] = true
					stack = append(stack, j)
				}
			}
		}
		if len(body) >= n/200 {
			for _, i := range body {
				sea[i] = true
			}
		}
	}
	return sea
}

// distance is how far, in tiles, each tile is from the nearest tile from is
// true of. It is taken in two sweeps, down and back up, each step across
// worth one and each step corner to corner the root of two, so that the
// distance round a point is near enough a circle rather than a square. A
// wrapping map is swept twice over, for the distance that goes round.
func distance(g *terra.Grid, from func(i int) bool) []float64 {
	d := make([]float64, len(g.Tiles))
	for i := range d {
		if !from(i) {
			d[i] = math.Inf(1)
		}
	}
	type step struct {
		dx, dy int
		cost   float64
	}
	down := []step{{-1, 0, 1}, {-1, -1, math.Sqrt2}, {0, -1, 1}, {1, -1, math.Sqrt2}}
	up := []step{{1, 0, 1}, {1, 1, math.Sqrt2}, {0, 1, 1}, {-1, 1, math.Sqrt2}}
	relax := func(x, y int, steps []step) {
		i := y*g.W + x
		for _, s := range steps {
			q := g.Norm(geom.Pos{X: x + s.dx, Y: y + s.dy})
			if g.In(q) {
				d[i] = math.Min(d[i], d[g.Index(q)]+s.cost)
			}
		}
	}
	sweeps := 1
	if g.Wrap {
		sweeps = 2
	}
	for range sweeps {
		for y := 0; y < g.H; y++ {
			for x := 0; x < g.W; x++ {
				relax(x, y, down)
			}
		}
		for y := g.H - 1; y >= 0; y-- {
			for x := g.W - 1; x >= 0; x-- {
				relax(x, y, up)
			}
		}
	}
	return d
}

// boxMean is the mean of v over the square of side 2r+1 round each tile,
// joined east to west when the map wraps and cut short at the edges when it
// does not. It is taken across and then down, so it costs the same whatever
// r is.
func boxMean(v []float64, w, h, r int, wrap bool) []float64 {
	across := make([]float64, len(v))
	for y := 0; y < h; y++ {
		row := v[y*w : (y+1)*w]
		sum, cnt := 0.0, 0
		at := func(x int) (float64, bool) {
			if wrap {
				return row[((x%w)+w)%w], true
			}
			if x < 0 || x >= w {
				return 0, false
			}
			return row[x], true
		}
		for x := -r; x <= r; x++ {
			if s, ok := at(x); ok {
				sum += s
				cnt++
			}
		}
		for x := 0; x < w; x++ {
			across[y*w+x] = sum / float64(cnt)
			if s, ok := at(x - r); ok {
				sum -= s
				cnt--
			}
			if s, ok := at(x + r + 1); ok {
				sum += s
				cnt++
			}
		}
	}
	out := make([]float64, len(v))
	for x := 0; x < w; x++ {
		sum, cnt := 0.0, 0
		for y := 0; y <= min(r, h-1); y++ {
			sum += across[y*w+x]
			cnt++
		}
		for y := 0; y < h; y++ {
			out[y*w+x] = sum / float64(cnt)
			if y-r >= 0 {
				sum -= across[(y-r)*w+x]
				cnt--
			}
			if y+r+1 < h {
				sum += across[(y+r+1)*w+x]
				cnt++
			}
		}
	}
	return out
}

func quant(v []float64, f float64) float64 {
	if len(v) == 0 {
		return 0
	}
	c := append([]float64(nil), v...)
	sort.Float64s(c)
	return c[int(f*float64(len(c)-1))]
}

// legendOf is the share of the map each class named in k holds, most first,
// leaving out the ones nothing is.
func legendOf(k []uint8, count uint8, name func(uint8) class) []share {
	tally := make([]int, count)
	for _, x := range k {
		tally[x]++
	}
	var out []share
	for x, c := range tally {
		if c > 0 {
			cl := name(uint8(x))
			out = append(out, shareOf(cl.name, c, len(k), hex(cl.col)))
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Count > out[b].Count })
	return out
}
