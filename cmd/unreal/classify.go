package main

import (
	"math"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/terra/geom"
)

// The biome of a tile, as cmd/overview reads it in classify.go: the Köppen
// type of its year and its rain (Köppen 1936, the thresholds as Peel,
// Finlayson and McMahon 2007 give them), and wetland where a river floods
// ground that is neither dry nor polar. It is here again rather than
// imported because cmd/overview is a command, and the reading is kept the
// same so that the species a tree gets here is the biome overview draws
// the tile in. Nothing is written back to the land.

// biomes are the names a dry tile can be given.
const (
	bIceCap          = "ice cap"
	bTundra          = "tundra"
	bBoreal          = "boreal forest"
	bContinental     = "continental forest"
	bColdSteppe      = "cold steppe"
	bHotSteppe       = "hot steppe"
	bColdDesert      = "cold desert"
	bHotDesert       = "hot desert"
	bMediterranean   = "mediterranean"
	bTemperateForest = "temperate forest"
	bRainforest      = "tropical rainforest"
	bMonsoon         = "monsoon forest"
	bSavanna         = "savanna"
	bWetland         = "wetland"
)

// floodReach is how many tiles out a river floods for each root of a cubic
// metre a second it carries: under two litres a second spreads one tile.
const floodReach = 2.0 / 48

// biomes names every dry tile of the land, and leaves the wet ones empty.
func biomes(g *terra.Grid, riverFlow float64) []string {
	n := len(g.Tiles)
	out := make([]string, n)

	// A river is water running through land: a channel narrow enough that
	// most of what is round it is dry. It floods the low ground either side
	// of it as far out as the water it carries will spread.
	wet := make([]float64, n)
	for i := range g.Tiles {
		if g.Tiles[i].Wet() {
			wet[i] = 1
		}
	}
	wetNear := boxMean(wet, g.W, g.H, 2, g.Wrap)
	flood := make([]bool, n)
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if t.Terrain == terra.Water && g.Flow[i] > riverFlow && wetNear[i] < 0.6 {
			floodOut(g, flood, g.PosOf(i), int(math.Sqrt(g.Flow[i])/floodReach))
		}
	}
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if t.Wet() || t.Terrain == terra.Flat {
			continue
		}
		p := g.PosOf(i)
		byRiver := flood[i] && g.Drain[i] < terra.FloodDepth/2
		out[i] = biome(koppen(g, p), byRiver)
	}
	return out
}

// koppen is the Köppen–Geiger type of the dry ground at p.
func koppen(g *terra.Grid, p geom.Pos) string {
	i := g.Index(p)
	mean, cold, hot := g.YearAt(i)
	return koppenOf(mean, cold, hot, g.Rain(i), g.RainWarm(i), g.Barren(p))
}

// koppenOf is the Köppen–Geiger type of a year with the given mean, coldest
// and warmest month, rain, and share of that rain in the warmer half, on
// ground under ice or not. The months are read off the year as a sine and
// the rain as one too: see cmd/overview/classify.go.
func koppenOf(mean, cold, hot, rain, warm float64, ice bool) string {
	if hot < 10 {
		if hot < 0 || ice {
			return "EF"
		}
		return "ET"
	}
	a := math.Max(-1, math.Min(1, math.Pi*(warm-0.5)))
	sDry, sWet, wDry, wWet := math.Inf(1), 0.0, math.Inf(1), 0.0
	for k := range 12 {
		th := (float64(k)+0.5)*math.Pi/6 - math.Pi
		m := rain / 12 * (1 + a*math.Cos(th))
		if math.Abs(th) < math.Pi/2 {
			sDry, sWet = math.Min(sDry, m), math.Max(sWet, m)
		} else {
			wDry, wWet = math.Min(wDry, m), math.Max(wWet, m)
		}
	}
	dry := math.Min(sDry, wDry)

	threshold := 20*mean + 140
	switch {
	case warm >= 0.7:
		threshold = 20*mean + 280
	case warm <= 0.3:
		threshold = 20 * mean
	}
	if rain < threshold {
		kind, heat := "BS", "k"
		if rain < threshold/2 {
			kind = "BW"
		}
		if mean >= 18 {
			heat = "h"
		}
		return kind + heat
	}

	if cold >= 18 {
		switch {
		case dry >= 60:
			return "Af"
		case dry >= 100-rain/25:
			return "Am"
		}
		return "Aw"
	}
	group := "C"
	if cold <= -3 {
		group = "D"
	}
	season := "f"
	switch {
	case sDry < 40 && sDry < wWet/3:
		season = "s"
	case wDry < sWet/10:
		season = "w"
	}
	summer := "c"
	switch {
	case hot >= 22:
		summer = "a"
	case warmMonths(mean, hot) >= 4:
		summer = "b"
	}
	return group + season + summer
}

// warmMonths is how many months of a sinusoidal year with the given mean and
// warmest month stand at ten degrees or more.
func warmMonths(mean, hot float64) int {
	amp := (hot - mean) / (math.Sin(math.Pi/12) / (math.Pi / 12))
	n := 0
	for k := range 12 {
		if mean+amp*math.Cos((float64(k)+0.5)*math.Pi/6-math.Pi) >= 10 {
			n++
		}
	}
	return n
}

// biome is the name a Köppen type is given, and wetland where a river
// floods ground that is neither dry nor polar.
func biome(k string, byRiver bool) string {
	if byRiver && k[0] != 'B' && k[0] != 'E' {
		return bWetland
	}
	switch {
	case k == "EF":
		return bIceCap
	case k == "ET":
		return bTundra
	case k == "BWh":
		return bHotDesert
	case k == "BWk":
		return bColdDesert
	case k == "BSh":
		return bHotSteppe
	case k == "BSk":
		return bColdSteppe
	case k == "Af":
		return bRainforest
	case k == "Am":
		return bMonsoon
	case k[0] == 'A':
		return bSavanna
	case k[0] == 'C' && k[1] == 's':
		return bMediterranean
	case k[0] == 'C':
		return bTemperateForest
	case k[2] == 'a':
		return bContinental
	}
	return bBoreal
}

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

// boxMean is the mean of v over the square of side 2r+1 round each tile,
// joined east to west when the map wraps and cut short at the edges when it
// does not.
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
