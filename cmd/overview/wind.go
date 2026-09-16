package main

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/terra/clock"
	"github.com/LukasSelin/terra/geom"
)

// The wind, drawn. A wind is a direction and a speed at every tile, which a
// colour per tile can only half show, so it is drawn twice: once as a colour
// wheel, hue for the way it blows and brightness for how hard, and once as
// streamlines laid over the ground - short traces along the wind from a
// scatter of starting points, longer where it blows harder, with a head on
// the end that points the way it goes.

// windField is the wind over every tile of a map, in metres a second toward
// the east and the north.
type windField struct {
	w, h   int
	wrap   bool
	u, v   []float64
	across []float64 // kilometres a tile is along each row
	down   float64   // and down one
}

// windOf reads the wind on day of the year off g, or the year's mean wind
// if day is below zero.
func windOf(g *terra.Grid, day int) *windField {
	f := &windField{w: g.W, h: g.H, wrap: g.Wrap, u: make([]float64, len(g.Tiles)), v: make([]float64, len(g.Tiles))}
	for i := range g.Tiles {
		if day < 0 {
			f.u[i], f.v[i] = g.MeanWind(i)
		} else {
			f.u[i], f.v[i] = g.WindOn(i, day)
		}
	}
	f.across = make([]float64, g.H)
	f.down = 1
	if g.Wrap {
		f.down = 20015 / float64(g.H)
		for y := range f.across {
			lat := 90 - 180*(float64(y)+0.5)/float64(g.H)
			f.across[y] = math.Max(0.05, math.Cos(lat*math.Pi/180)) * 40030 / float64(g.W)
		}
	} else {
		for y := range f.across {
			f.across[y] = 1
		}
	}
	return f
}

// at is the wind between the tiles, at a place given in tiles.
func (f *windField) at(x, y float64) (u, v float64) {
	x, y = x-0.5, y-0.5
	x0, y0 := math.Floor(x), math.Floor(y)
	tx, ty := x-x0, y-y0
	idx := func(cx, cy int) int {
		if f.wrap {
			cx = ((cx % f.w) + f.w) % f.w
		} else {
			cx = min(max(cx, 0), f.w-1)
		}
		cy = min(max(cy, 0), f.h-1)
		return cy*f.w + cx
	}
	ix, iy := int(x0), int(y0)
	a, b, c, d := idx(ix, iy), idx(ix+1, iy), idx(ix, iy+1), idx(ix+1, iy+1)
	lerp := func(s []float64) float64 {
		top := s[a] + (s[b]-s[a])*tx
		bot := s[c] + (s[d]-s[c])*tx
		return top + (bot-top)*ty
	}
	return lerp(f.u), lerp(f.v)
}

// speedMost is the speed the colours of the wind are full at, m/s.
const speedMost = 14.0

// windColor is the colour-wheel reading of a wind: hue for the way it blows,
// brightness for how hard.
func windColor(u, v float64) color.RGBA {
	s := math.Hypot(u, v)
	// Toward the east is red, the north yellow-green, the west cyan, the
	// south violet.
	h := math.Atan2(v, u) / (2 * math.Pi)
	return hue(h, 0.75, 0.25+0.75*clamp(s/speedMost, 0, 1))
}

// streamlines draws the wind f over img, px pixels to a tile.
func streamlines(img *image.RGBA, f *windField, px int) {
	spacing := 26.0
	if px > 1 {
		spacing = math.Max(26, 3*float64(px))
	}
	width, height := float64(f.w*px), float64(f.h*px)
	ink := color.RGBA{20, 24, 32, 255}
	for sy := spacing / 2; sy < height; sy += spacing {
		for sx := spacing / 2; sx < width; sx += spacing {
			// A starting point scattered about its place in the lattice, the
			// same scatter every time.
			jx, jy := jitter(int(sx), int(sy))
			x, y := sx+(jx-0.5)*spacing*0.8, sy+(jy-0.5)*spacing*0.8
			u, v := f.at(x/float64(px), y/float64(px))
			s := math.Hypot(u, v)
			if s < 0.3 {
				continue
			}
			length := clamp(s/speedMost, 0.3, 1.2) * spacing * 1.8
			var tail [][2]float64
			for done := 0.0; done < length; done += 0.5 {
				tail = append(tail, [2]float64{x, y})
				// Two steps of half a pixel along the wind as the map draws it:
				// a kilometre east is fewer pixels near the equator than near
				// the pole.
				dx, dy := f.step(x, y, px)
				mx, my := x+dx*0.25, y+dy*0.25
				dx, dy = f.step(mx, my, px)
				x, y = x+dx*0.5, y+dy*0.5
				if y < 0 || y >= height || (!f.wrap && (x < 0 || x >= width)) {
					break
				}
			}
			if len(tail) < 4 {
				continue
			}
			for k, p := range tail {
				splat(img, p[0], p[1], ink, 0.25+0.75*float64(k)/float64(len(tail)), f.wrap)
			}
			// The head: two barbs back from the end, either side of the way it
			// was going.
			end, before := tail[len(tail)-1], tail[len(tail)-4]
			ang := math.Atan2(end[1]-before[1], end[0]-before[0])
			barb := math.Max(3, spacing*0.22)
			for _, side := range []float64{-0.5, 0.5} {
				a := ang + math.Pi + side
				for t := 0.0; t <= barb; t += 0.5 {
					splat(img, end[0]+math.Cos(a)*t, end[1]+math.Sin(a)*t, ink, 1, f.wrap)
				}
			}
		}
	}
}

// step is the way the wind blows at pixel x, y, as a unit step in pixels.
func (f *windField) step(x, y float64, px int) (dx, dy float64) {
	u, v := f.at(x/float64(px), y/float64(px))
	row := min(max(int(y/float64(px)), 0), f.h-1)
	// Tiles a second east and south, which is what the map draws.
	tx, ty := u/f.across[row], -v/f.down
	n := math.Hypot(tx, ty)
	if n == 0 {
		return 0, 0
	}
	return tx / n, ty / n
}

// splat lays ink over the pixel at x, y with the given strength, shared among
// the four pixels round it.
func splat(img *image.RGBA, x, y float64, ink color.RGBA, strength float64, wrap bool) {
	b := img.Bounds()
	x0, y0 := math.Floor(x), math.Floor(y)
	tx, ty := x-x0, y-y0
	for _, c := range []struct {
		dx, dy int
		w      float64
	}{{0, 0, (1 - tx) * (1 - ty)}, {1, 0, tx * (1 - ty)}, {0, 1, (1 - tx) * ty}, {1, 1, tx * ty}} {
		px, py := int(x0)+c.dx, int(y0)+c.dy
		if wrap {
			px = ((px % b.Dx()) + b.Dx()) % b.Dx()
		}
		if px < 0 || py < 0 || px >= b.Dx() || py >= b.Dy() {
			continue
		}
		a := clamp(c.w*strength*1.6, 0, 1)
		off := img.PixOffset(px, py)
		pix := img.Pix[off : off+3]
		pix[0] = byte(float64(pix[0]) + (float64(ink.R)-float64(pix[0]))*a)
		pix[1] = byte(float64(pix[1]) + (float64(ink.G)-float64(pix[1]))*a)
		pix[2] = byte(float64(pix[2]) + (float64(ink.B)-float64(pix[2]))*a)
	}
}

// jitter is two numbers in [0,1) that follow from a place and nothing else.
func jitter(x, y int) (float64, float64) {
	h := uint64(x)*0x9E3779B97F4A7C15 ^ uint64(y)*0xC2B2AE3D27D4EB4F
	h ^= h >> 29
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 32
	return float64(h&0xFFFF) / 65536, float64((h>>16)&0xFFFF) / 65536
}

// weatherDrawing is the day's weather: the wind as streamlines over the
// pressure, with the systems in it marked - L for a low, H for a high, and a
// ring for a tropical storm.
func weatherDrawing(land *terra.Land, shade func(geom.Pos) float64) drawing {
	g := land.Grid
	f := &windField{w: g.W, h: g.H, wrap: g.Wrap, u: make([]float64, len(g.Tiles)), v: make([]float64, len(g.Tiles))}
	pres := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		p := g.PosOf(i)
		f.u[i], f.v[i] = land.WindAt(p)
		pres[i] = land.PressureAt(p)
	}
	f.across, f.down = windOf(g, 0).across, windOf(g, 0).down
	counts := map[terra.SystemKind]int{}
	if land.Weather != nil {
		for _, s := range land.Weather.Systems {
			if s.Strength() != 0 {
				counts[s.Kind]++
			}
		}
	}
	date := clock.At(land.Tick)
	return drawing{
		file: "weather", title: "Weather today",
		about: fmt.Sprintf("The weather on %v: the wind near the ground as streamlines, over the pressure at sea level from 970 hPa (violet) through 1013 (pale) to 1040 (amber), with isobars every 4 hPa. %d lows (L), %d highs (H) and %d tropical storms (rings) are on the planet; the lows turn anticlockwise north of the equator and clockwise south of it, and are carried east by the westerlies.",
			date, counts[terra.Low], counts[terra.High], counts[terra.Storm]),
		color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
			c := ramp(pressure, (pres[i]-970)/70)
			// An isobar where the pressure crosses a multiple of four
			// between this tile and the next one along or down.
			// The next tile along, which on a valley stops at the edge.
			q := geom.Pos{X: p.X + 1, Y: p.Y}
			if g.Wrap {
				q = g.Norm(q)
			} else {
				q.X = min(q.X, g.W-1)
			}
			r := geom.Pos{X: p.X, Y: min(p.Y+1, g.H-1)}
			band := func(v float64) int { return int(math.Floor(v / 4)) }
			if band(pres[i]) != band(pres[g.Index(q)]) || band(pres[i]) != band(pres[g.Index(r)]) {
				c = scaleRGB(c, 0.72)
			}
			if t.Wet() {
				return scaleRGB(c, 0.9)
			}
			return scaleRGB(c, shade(p))
		},
		overlay: func(img *image.RGBA, px int) {
			streamlines(img, f, px)
			if land.Weather == nil {
				return
			}
			for _, s := range land.Weather.Systems {
				if s.Strength() == 0 {
					continue
				}
				x, y, on := land.Place(s)
				if !on {
					continue
				}
				mark(img, x*float64(px), y*float64(px), s, g.Wrap)
			}
		},
	}
}

var pressure = []color.RGBA{{150, 110, 190, 255}, {205, 190, 225, 255}, {240, 240, 236, 255}, {240, 215, 160, 255}, {220, 160, 70, 255}}

// glyphs are the letters the systems are marked with, five pixels by seven.
var glyphs = map[terra.SystemKind][7]string{
	terra.Low:  {"#....", "#....", "#....", "#....", "#....", "#....", "#####"},
	terra.High: {"#...#", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
}

// mark draws a system at pixel x, y: its letter, or a ring for a storm.
func mark(img *image.RGBA, x, y float64, s terra.System, wrap bool) {
	ink := color.RGBA{190, 30, 40, 255}
	if s.Kind == terra.High {
		ink = color.RGBA{30, 60, 170, 255}
	}
	if s.Kind == terra.Storm {
		ink = color.RGBA{150, 20, 120, 255}
		for a := 0.0; a < 2*math.Pi; a += 0.08 {
			for _, r := range []float64{5, 6} {
				splat(img, x+r*math.Cos(a), y+r*math.Sin(a), ink, 1, wrap)
			}
		}
		splat(img, x, y, ink, 1, wrap)
		return
	}
	const k = 2
	g := glyphs[s.Kind]
	for gy, row := range g {
		for gx, c := range row {
			if c != '#' {
				continue
			}
			for dy := 0; dy < k; dy++ {
				for dx := 0; dx < k; dx++ {
					splat(img, x+float64((gx-2)*k+dx), y+float64((gy-3)*k+dy), ink, 1, wrap)
				}
			}
		}
	}
}
