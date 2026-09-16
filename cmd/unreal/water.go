package main

import (
	"encoding/json"
	"math"
	"os"
	"sort"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/terra/geom"
)

// The water, for Unreal's Water plugin: the sea as an ocean body at its
// level, each lake as a body with a level and an outline, and each river as
// a spline with a width and a depth at every point. Coordinates are metres
// from the world origin, which is the centre of tile (0, 0), and z is
// metres, the same metres the heightmap decodes to.

// waterName is the file the water bodies are written as.
const waterName = "water.json"

// ocean is the sea.
type ocean struct {
	Type  string  `json:"type"`
	Level float64 `json:"level"`
}

// lakeBody is one lake: its surface, and its outline as a closed polygon of
// tile corners, the first corner repeated last. A lake with islands has
// only its outer shore here.
type lakeBody struct {
	Type    string       `json:"type"`
	Level   float64      `json:"level"`
	Outline [][2]float64 `json:"outline"`
	Closed  bool         `json:"closed"`
	Salt    bool         `json:"salt"`
	Tiles   int          `json:"tiles"`
}

// river is one reach: a run of tiles down from a source to the sea, a lake,
// the edge of the world or a reach already written, which it joins at its
// last point.
type river struct {
	Type   string       `json:"type"`
	Points []riverPoint `json:"points"`
}

// riverPoint is one tile of a reach: its centre, the water's surface there,
// and the channel's width and depth.
type riverPoint struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Z     float64 `json:"z"`
	Width float64 `json:"width"`
	Depth float64 `json:"depth"`
}

// waterFile is what water.json holds.
type waterFile struct {
	// Units says what the numbers are.
	Units string `json:"units"`
	// Bodies are the ocean, then the lakes, then the rivers.
	Bodies []any `json:"bodies"`
	Lakes  int   `json:"lakes"`
	Rivers int   `json:"rivers"`
}

// The channel's geometry, with the constants erode.go reads a channel with:
// Finnegan and others' (2005) width W = widthCoeff * Q^(3/8) * S^(-3/16),
// set so that ten cubic metres a second on a fall of one in a hundred is
// eleven metres across, and Manning's depth with the roughness of a clean
// winding natural channel. Both are read at the mean flow, the river as it
// runs on an ordinary day, and on a fall of no less than leastFall so a
// river on a dead flat is not read as infinitely wide.
const (
	widthCoeff = 2.0
	manning    = 0.035
	leastFall  = 1e-4
)

// channelWidth is the width in metres of q cubic metres a second on a fall
// of s.
func channelWidth(q, s float64) float64 {
	return widthCoeff * math.Pow(q, 3.0/8) * math.Pow(math.Max(s, leastFall), -3.0/16)
}

// channelDepth is Manning's depth in metres of q cubic metres a second
// running w metres wide down a fall of s: (n q / (w sqrt(s)))^(3/5).
func channelDepth(q, w, s float64) float64 {
	s = math.Max(s, leastFall)
	return math.Pow(manning*q/(w*math.Sqrt(s)), 3.0/5)
}

// metres is the world position of the centre of tile p.
func metres(p geom.Pos) (x, y float64) {
	return float64(p.X) * terra.TileSpan, float64(p.Y) * terra.TileSpan
}

// waterOf reads the bodies off the world.
func waterOf(g *terra.Grid, riverFlow float64) waterFile {
	w := waterFile{Units: "metres from the world origin, the centre of tile (0, 0); x east with the tile's x, y with the tile's y, z up"}
	n := len(g.Tiles)

	// Which lake each tile lies under, by the lake's index, and whether it
	// is under the sea.
	lakeOf := make([]int, n)
	byPtr := map[*terra.Lake]int{}
	for k := range g.Lakes {
		byPtr[&g.Lakes[k]] = k
	}
	sea := g.SeaLevel()
	underSea := make([]bool, n)
	for i := range g.Tiles {
		lakeOf[i] = -1
		p := g.PosOf(i)
		if lk, ok := g.LakeAt(p); ok && lk.Level > g.Height[i] {
			lakeOf[i] = byPtr[lk]
		}
		underSea[i] = sea >= 0 && g.Tiles[i].Wet() && lakeOf[i] < 0 && g.Height[i] <= sea
	}

	if sea >= 0 {
		w.Bodies = append(w.Bodies, ocean{Type: "ocean", Level: sea})
	}
	for k := range g.Lakes {
		lk := &g.Lakes[k]
		outline := lakeOutline(g, lakeOf, k)
		if len(outline) == 0 {
			continue
		}
		tiles := 0
		for i := range lakeOf {
			if lakeOf[i] == k {
				tiles++
			}
		}
		w.Bodies = append(w.Bodies, lakeBody{Type: "lake", Level: lk.Level, Outline: outline, Closed: lk.Closed, Salt: lk.Closed, Tiles: tiles})
		w.Lakes++
	}
	for _, r := range reaches(g, lakeOf, underSea, riverFlow) {
		w.Bodies = append(w.Bodies, r)
		w.Rivers++
	}
	return w
}

// lakeOutline is the outer shore of lake k as a closed loop of tile
// corners in metres: every edge between a tile under the lake and one not,
// chained into loops, and the longest loop kept.
func lakeOutline(g *terra.Grid, lakeOf []int, k int) [][2]float64 {
	in := func(x, y int) bool {
		if y < 0 || y >= g.H || x < 0 || x >= g.W {
			return false
		}
		return lakeOf[y*g.W+x] == k
	}
	type corner struct{ x, y int }
	// Each edge runs clockwise round the lake with the lake on its right
	// as the map is drawn, y down.
	next := map[corner][]corner{}
	edges := 0
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			if !in(x, y) {
				continue
			}
			add := func(a, b corner) {
				next[a] = append(next[a], b)
				edges++
			}
			if !in(x, y-1) {
				add(corner{x, y}, corner{x + 1, y})
			}
			if !in(x+1, y) {
				add(corner{x + 1, y}, corner{x + 1, y + 1})
			}
			if !in(x, y+1) {
				add(corner{x + 1, y + 1}, corner{x, y + 1})
			}
			if !in(x-1, y) {
				add(corner{x, y + 1}, corner{x, y})
			}
		}
	}
	if edges == 0 {
		return nil
	}
	starts := make([]corner, 0, len(next))
	for c := range next {
		starts = append(starts, c)
	}
	sort.Slice(starts, func(a, b int) bool {
		if starts[a].y != starts[b].y {
			return starts[a].y < starts[b].y
		}
		return starts[a].x < starts[b].x
	})
	var best []corner
	for _, s := range starts {
		for len(next[s]) > 0 {
			loop := []corner{s}
			at := s
			for {
				outs := next[at]
				if len(outs) == 0 {
					break
				}
				// Where two edges leave one corner the lake touches
				// itself cornerways; take the first, which keeps the
				// walk deterministic, and the other is another loop.
				to := outs[0]
				next[at] = outs[1:]
				loop = append(loop, to)
				at = to
				if at == s {
					break
				}
			}
			if len(loop) > len(best) {
				best = loop
			}
		}
	}
	out := make([][2]float64, len(best))
	for i, c := range best {
		out[i] = [2]float64{(float64(c.x) - 0.5) * terra.TileSpan, (float64(c.y) - 0.5) * terra.TileSpan}
	}
	return out
}

// reaches walks every river from its source down. A tile is river where it
// carries more than riverFlow and lies under neither a lake nor the sea; a
// source is a river tile no river tile drains into. Each reach runs until
// the water reaches the sea, a lake, the edge or a reach already walked,
// and that tile is its last point, so the splines meet.
func reaches(g *terra.Grid, lakeOf []int, underSea []bool, riverFlow float64) []river {
	n := len(g.Tiles)
	isRiver := func(i int) bool {
		return g.Flow[i] > riverFlow && lakeOf[i] < 0 && !underSea[i]
	}
	// down is where each tile's water goes if it goes to a neighbour, -1
	// otherwise: a jump across a lake ends a reach.
	down := make([]int, n)
	in := make([]int, n)
	for i := range g.Tiles {
		down[i] = -1
		p := g.PosOf(i)
		q, ok := g.Downstream(p)
		if !ok {
			continue
		}
		if d := g.Delta(p, q); d.X < -1 || d.X > 1 || d.Y < -1 || d.Y > 1 {
			continue
		}
		down[i] = g.Index(q)
		if isRiver(i) && isRiver(down[i]) {
			in[down[i]]++
		}
	}
	// A tile's channel: its width and depth at the mean flow on the fall
	// to its receiver.
	geometry := func(i int) (width, depth float64) {
		q := g.Flow[i]
		fall := leastFall
		if d := down[i]; d >= 0 {
			p, dp := g.PosOf(i), g.PosOf(d)
			run := terra.TileSpan
			if delta := g.Delta(p, dp); delta.X != 0 && delta.Y != 0 {
				run *= math.Sqrt2
			}
			fall = math.Max(fall, (g.Height[i]-g.Height[d])/run)
		}
		width = channelWidth(q, fall)
		depth = channelDepth(q, width, fall)
		return width, depth
	}
	level := func(i int) float64 {
		if lakeOf[i] >= 0 {
			return g.Lakes[lakeOf[i]].Level
		}
		if underSea[i] {
			return g.SeaLevel()
		}
		return g.Height[i]
	}

	walked := make([]bool, n)
	var out []river
	for s := range g.Tiles {
		if !isRiver(s) || in[s] > 0 {
			continue
		}
		r := river{Type: "river"}
		z := math.Inf(1)
		at := s
		for {
			x, y := metres(g.PosOf(at))
			width, depth := geometry(at)
			var surface float64
			if isRiver(at) {
				surface = g.Height[at] + depth
			} else {
				surface = level(at)
			}
			// The water cannot run uphill: where the ground does, over a
			// dimple too shallow to be a lake, the surface is the level
			// it backs up to, which is the surface upstream of it.
			z = math.Min(z, surface)
			r.Points = append(r.Points, riverPoint{X: x, Y: y, Z: z, Width: width, Depth: depth})
			if !isRiver(at) || walked[at] {
				break
			}
			walked[at] = true
			if down[at] < 0 {
				break
			}
			at = down[at]
		}
		if len(r.Points) >= 2 {
			out = append(out, r)
		}
	}
	return out
}

// writeWater writes the water bodies as JSON.
func writeWater(path string, w waterFile) error {
	b, err := json.MarshalIndent(w, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
