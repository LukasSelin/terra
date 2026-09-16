package terra

import (
	"math"
	"testing"

	"github.com/LukasSelin/terra/geom"
)

// A sea grows with its fetch as the square root of it, and stops growing when
// it is as big as its wind makes it: JONSWAP's ten metres a second over ten
// kilometres raise half a metre of sea three seconds long, and Pierson and
// Moskowitz's fully developed sea under that wind is two and a half metres.
func TestTheWindRaisesASeaByItsFetch(t *testing.T) {
	h, period := waveOf(10, 10*km)
	if math.Abs(h-0.511) > 0.005 || math.Abs(period-2.90) > 0.03 {
		t.Errorf("10 m/s over 10 km: %.3f m, %.2f s; want 0.511 m, 2.90 s", h, period)
	}
	h4, _ := waveOf(10, 40*km)
	if math.Abs(h4/h-2) > 1e-9 {
		t.Errorf("four times the fetch raised %.3f times the sea, want twice", h4/h)
	}
	full, fullT := waveOf(10, openFetch)
	if math.Abs(full-2.48) > 0.01 || math.Abs(fullT-8.29) > 0.01 {
		t.Errorf("a fully developed sea under 10 m/s: %.2f m, %.2f s; want 2.48 m, 8.29 s", full, fullT)
	}
	if still, _ := waveOf(0, 10*km); still != 0 {
		t.Errorf("a still day raised %.2f m of sea", still)
	}
}

// pond is a map of dry ground with a rectangle of sea in it, x0 to x1 across and
// y0 to y1 down.
func pond(w, h, x0, x1, y0, y1 int, wrap bool) *Grid {
	g := NewGrid(w, h)
	g.Wrap = wrap
	g.sea = 10
	for i := range g.Tiles {
		p := g.PosOf(i)
		t := &g.Tiles[i]
		g.Height[i] = 12
		if p.X >= x0 && p.X <= x1 && p.Y >= y0 && p.Y <= y1 {
			g.Height[i], t.Terrain = 5, Water
		}
	}
	return g
}

// The fetch along a way is the open water that lies that way, and no more: to
// the far shore of a pond, off the edge of a valley's map, and round a globe
// that is sea all the way round.
func TestTheFetchIsTheOpenWaterThatWay(t *testing.T) {
	dir := func(x, y int) int {
		for k, d := range Dirs {
			if d.X == x && d.Y == y {
				return k
			}
		}
		panic("no such way")
	}
	g := pond(40, 20, 10, 29, 5, 14, false)
	f := g.fetches()
	at := func(k, x, y int) float64 { return float64(f[k][g.Index(geom.Pos{X: x, Y: y})]) }
	if got := at(dir(-1, 0), 29, 10); got != 19*TileSpan {
		t.Errorf("west from the east shore: %.1f m, want %.1f", got, 19*TileSpan)
	}
	if got := at(dir(0, 1), 20, 5); got != 9*TileSpan {
		t.Errorf("south from the north shore: %.1f m, want %.1f", got, 9*TileSpan)
	}
	if got, want := at(dir(1, 1), 10, 5), 9*TileSpan*math.Sqrt2; math.Abs(got-want) > 1e-3 {
		t.Errorf("south-east from the corner: %.1f m, want %.1f", got, want)
	}
	if got := at(dir(1, 0), 30, 10); got != 0 {
		t.Errorf("east from the ground beyond the east shore: %.1f m", got)
	}

	open := pond(40, 20, 0, 29, 5, 14, false)
	if got := float64(open.fetches()[dir(-1, 0)][open.Index(geom.Pos{X: 29, Y: 10})]); got != openFetch {
		t.Errorf("west to a sea that runs off the map: %.1f m, want the open ocean", got)
	}

	round := pond(40, 20, 0, 39, 5, 14, true)
	rf := round.fetches()
	if got := float64(rf[dir(1, 0)][round.Index(geom.Pos{X: 7, Y: 10})]); got != openFetch {
		t.Errorf("east round a globe that is sea all the way: %.1f m", got)
	}
	round.Height[round.Index(geom.Pos{X: 3, Y: 10})] = 12
	if got := float64(round.fetches()[dir(1, 0)][round.Index(geom.Pos{X: 5, Y: 10})]); got != 37*TileSpan {
		t.Errorf("east round a globe to an island just west: %.1f m, want %.1f", got, 37*TileSpan)
	}
}

// exposure is a coast with a beach along it, open to the ocean off the west edge
// of the map in its northern half and sheltered behind a bar of ground a few
// hundred metres offshore in its southern half, with the same soil everywhere
// and a wind that blows onshore.
func exposure() (*Grid, func(i, k int) (float64, float64)) {
	g := NewGrid(60, 40)
	g.sea = 10
	for i := range g.Tiles {
		p := g.PosOf(i)
		t := &g.Tiles[i]
		g.Height[i], g.Soil[i], t.Sand, t.Clay = 10.2, 1, 0.4, 0.2
		switch {
		case p.X < 30 && !(p.X == 20 && p.Y >= 16):
			g.Height[i], t.Terrain = 9, Water
		case p.X > 30:
			g.Height[i] = 14
		}
	}
	return g, func(i, k int) (float64, float64) { return 8, 0 }
}

// The more a shore is exposed to the sea, the more the waves take the fines out
// of its beach, and the sandier it is; and what they take is what goes to the
// sea, to the grain.
func TestAnExposedBeachIsSandier(t *testing.T) {
	g, wind := exposure()
	s := g.surfOf(wind)
	before := 0.0
	for i := range g.Tiles {
		before += g.Height[i]
	}
	g.winnow(s, 5*yr)
	after := 0.0
	for i := range g.Tiles {
		after += g.Height[i]
	}
	sand := func(y0, y1 int) float64 {
		sum := 0.0
		for y := y0; y <= y1; y++ {
			sum += g.At(geom.Pos{X: 30, Y: y}).Sand
		}
		return sum / float64(y1-y0+1)
	}
	open, sheltered := sand(4, 12), sand(26, 30)
	if !(open > sheltered+0.1) || !(sheltered > 0.4) {
		t.Errorf("the open beach is %.2f sand and the sheltered one %.2f, from 0.40", open, sheltered)
	}
	if deep := g.At(geom.Pos{X: 45, Y: 10}).Sand; deep != 0.4 {
		t.Errorf("ground well above the beach was sorted to %.2f sand", deep)
	}
	gone := g.exported[Sand] + g.exported[Silt] + g.exported[Clay]
	if gone <= 0 || math.Abs(before-after-gone) > 1e-9*gone {
		t.Errorf("the ground lost %.6f and %.6f went to the sea", before-after, gone)
	}
	if g.exported[Sand] != 0 {
		t.Errorf("the waves carried %.4f of sand out to sea", g.exported[Sand])
	}
}
