package terra

import (
	"math"
	"testing"
)

// plateau is an ocean globe with a round plateau height metres high on land
// centred at the latitude lat, a quarter of the way round, some two
// thousand kilometres across.
func plateau(lat, height float64) *Grid {
	g := oceanGlobe(256, 128)
	c := Climate{rows: g.H, globe: true}
	for i := range g.Tiles {
		x, y := i%g.W, i/g.W
		dl := c.latitude(y) - lat
		dx := (float64(x) - 64) * 360 / float64(g.W) * math.Cos(lat*math.Pi/180)
		if r := math.Hypot(dl, dx); r < 18 {
			g.Height[i] = 60 + height*math.Exp(-r*r/(2*7*7))
		}
	}
	return g
}

// Air blowing over a plateau is squeezed under the tropopause and turns
// anticyclonically over it, and comes off its lee stretched and turning the
// other way: a ridge aloft over the plateau and a trough downstream of it,
// as the East Asian trough lies east of Tibet (Hoskins and Karoly, 1981).
// Held in the north's winter, the westerlies' strongest.
func TestAPlateauStandsATroughDownstream(t *testing.T) {
	g := plateau(35, 3000)
	g.weather()
	e := g.winds.Env
	at := e.W / 4 // the plateau's column of cells
	var row int
	for cy := range e.H {
		if math.Abs(g.air.Lat[cy*e.Cell]-40) < math.Abs(g.air.Lat[row*e.Cell]-40) {
			row = cy
		}
	}
	aloft := e.Waves[0].Aloft
	read := func(deg float64) float64 {
		cx := (at + int(math.Round(deg/360*float64(e.W))) + e.W) % e.W
		return float64(aloft[row*e.W+cx])
	}
	trough, where := math.Inf(1), 0.0
	for deg := 0.0; deg <= 90; deg += 360 / float64(e.W) {
		if v := read(deg); v < trough {
			trough, where = v, deg
		}
	}
	for _, deg := range []float64{-60, -30, -15, 0, 15, 30, 45, 60, 90, 120} {
		t.Logf("40N, %+4.0f degrees of longitude from the plateau: the 300 hPa surface stands %+.1f m over its row", deg, read(deg))
	}
	t.Logf("the deepest trough within 90 degrees downstream: %.1f m, %.0f degrees east", trough, where)
	if trough >= 0 || where < 10 {
		t.Errorf("no trough downstream of the plateau: the least height within 90 degrees east is %+.1f m, %.0f degrees east", trough, where)
	}
	if trough >= read(0) {
		t.Errorf("the trough downstream, %+.1f m, is no lower than over the plateau, %+.1f m", trough, read(0))
	}
}

// coastCells is the air cells of land on rows between lo and hi degrees of
// latitude whose neighbour dx cells along the row and dy down it (north is
// the row before) is sea.
func coastCells(o *oceanCells, lo, hi float64, dx, dy int) []int {
	var out []int
	for _, cy := range o.rowsBetween(lo, hi) {
		ny := cy + dy
		if ny < 0 || ny >= o.e.H {
			continue
		}
		for cx := range o.e.W {
			if o.e.Sea[cy*o.e.W+cx] < 0.3 && o.e.Sea[o.at(cx+dx, ny)] > 0.7 {
				out = append(out, cy*o.e.W+cx)
			}
		}
	}
	return out
}

// onshore is the mean over cells of the wind of phase k toward the land
// from the sea dx, dy of it, area-weighted.
func onshore(o *oceanCells, cells []int, k, dx, dy int) float64 {
	var s, w float64
	for _, i := range cells {
		cy := i / o.e.W
		into := -float64(dx)*float64(o.w.U[k][i]) + float64(dy)*float64(o.w.V[k][i])
		s, w = s+o.weight(cy)*into, w+o.weight(cy)
	}
	return s / w
}

// phaseRain is the share of the year's rain over cells that falls in phase
// k, area-weighted.
func phaseRain(o *oceanCells, cells []int, k int) float64 {
	var in, all float64
	for _, i := range cells {
		b := o.w.Budget
		for p := range b {
			r := (b[p].Rain[i] + b[p].Oro[i]) * o.weight(i/o.e.W)
			all += r
			if p == k {
				in += r
			}
		}
	}
	return in / all
}

// winterCold is the mean over the land tiles of cells of the coldest of
// their year, degrees.
func winterCold(w *Land, o *oceanCells, cells []int) float64 {
	g := w.Grid
	e := o.e
	var s, n float64
	for _, c := range cells {
		cx, cy := c%e.W, c/e.W
		for y := cy * e.Cell; y < (cy+1)*e.Cell; y++ {
			for x := cx * e.Cell; x < (cx+1)*e.Cell; x++ {
				i := y*g.W + x
				if g.sunk(i) {
					continue
				}
				mean, swing := w.yearAt(i)
				s, n = s+mean-math.Abs(swing), n+1
			}
		}
	}
	return s / n
}

// The readings the stationary waves and the monsoons are held to on the
// yardsticks' globe (#35), logged: the summer's monsoon onto the coasts of
// the subtropical continents that face the equator and the east; the
// trough the westerlies stand downstream of the highest plateau in their
// winter; and the cold of the winter on the eastern coasts of the middle
// latitudes against the western, which on the earth are colder by ten to
// twenty degrees at forty-five to sixty north (Boston and Halifax against
// Oregon and Brittany, Vladivostok against Bergen).
func TestTheGlobesWavesAndMonsoons(t *testing.T) {
	if testing.Short() {
		t.Skip("needs the full globe")
	}
	land := yardLand("globe", 1, GlobeTerms())
	o := oceanOn(t, land.Grid)
	e := o.e
	for _, h := range []struct {
		name           string
		sign           float64
		summer, winter int
	}{{"north", 1, 2, 0}, {"south", -1, 0, 2}} {
		lo, hi := 10.0, 35.0
		if h.sign < 0 {
			lo, hi = -35, -10
		}
		dy := int(h.sign) // the sea toward the equator: the row after in the north
		equatorward := coastCells(o, lo, hi, 0, dy)
		east := coastCells(o, lo, hi, 1, 0)
		t.Logf("the %s's subtropical coasts facing the equator (%d cells): %+.2f m/s onshore in summer, %+.2f in winter; %.0f%% of their rain in the summer's quarter",
			h.name, len(equatorward), onshore(o, equatorward, h.summer, 0, dy), onshore(o, equatorward, h.winter, 0, dy), 100*phaseRain(o, equatorward, h.summer))
		t.Logf("the %s's subtropical coasts facing the east (%d cells): %+.2f m/s onshore in summer, %+.2f in winter; %.0f%% of their rain in the summer's quarter",
			h.name, len(east), onshore(o, east, h.summer, 1, 0), onshore(o, east, h.winter, 1, 0), 100*phaseRain(o, east, h.summer))

		lo, hi = 45, 60
		if h.sign < 0 {
			lo, hi = -60, -45
		}
		ec, wc := coastCells(o, lo, hi, 1, 0), coastCells(o, lo, hi, -1, 0)
		if len(ec) > 0 && len(wc) > 0 {
			te, tw := winterCold(land, o, ec), winterCold(land, o, wc)
			t.Logf("the %s's coasts at 45-60 degrees in winter: the eastern (%d cells) %.1f C, the western (%d cells) %.1f C, the east %+.1f degrees against the west",
				h.name, len(ec), te, len(wc), tw, te-tw)
		}
	}

	// The highest plateau: the most height, smoothed over some five hundred
	// kilometres, between twenty and seventy degrees.
	mean := func(cx, cy int) float64 {
		var s, n float64
		for dy := -3; dy <= 3; dy++ {
			y := cy + dy
			if y < 0 || y >= e.H {
				continue
			}
			r := int(math.Ceil(3 * e.Dy / e.Dx[y]))
			for dx := -r; dx <= r; dx++ {
				s, n = s+e.Height[o.at(cx+dx, y)], n+1
			}
		}
		return s / n
	}
	best, bh := -1, 0.0
	for _, cy := range append(o.rowsBetween(20, 70), o.rowsBetween(-70, -20)...) {
		for cx := range e.W {
			if h := mean(cx, cy); h > bh {
				best, bh = cy*e.W+cx, h
			}
		}
	}
	px, py := best%e.W, best/e.W
	k := 0
	if o.lat[py] < 0 {
		k = 2
	}
	aloft := e.Waves[k].Aloft
	step := 360 / float64(e.W)
	read := func(deg float64) float64 {
		return float64(aloft[o.at(px+int(math.Round(deg/step)), py)])
	}
	trough, tdeg := math.Inf(1), 0.0
	for deg := 0.0; deg <= 90; deg += step {
		if v := read(deg); v < trough {
			trough, tdeg = v, deg
		}
	}
	ridge, rdeg := math.Inf(-1), 0.0
	for deg := -60.0; deg <= 30; deg += step {
		if v := read(deg); v > ridge {
			ridge, rdeg = v, deg
		}
	}
	var zonal float64
	for cx := range e.W {
		zonal += float64(o.w.P[k][py*e.W+cx]) / float64(e.W)
	}
	surface := float64(o.w.P[k][o.at(px+int(math.Round(tdeg/step)), py)]) - zonal
	t.Logf("the highest plateau stands %.0f m high, smoothed, at %.1f degrees; in its winter the 300 hPa surface stands %+.0f m over its row at most, %+.0f degrees of longitude from it, and %+.0f m at least, %+.0f degrees downstream, where the sea-level pressure stands %+.1f hPa over its row's",
		bh, o.lat[py], ridge, rdeg, trough, tdeg, surface)
	for _, deg := range []float64{-45, -30, -15, 0, 15, 30, 45, 60, 90} {
		t.Logf("  %+4.0f degrees: %+.0f m", deg, read(deg))
	}
	if trough >= 0 || tdeg < 5 {
		t.Errorf("no trough downstream of the highest plateau: the least height within 90 degrees east is %+.0f m, %.0f degrees east", trough, tdeg)
	}
}
