package terra

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/LukasSelin/terra/internal/atmos"
)

// The sea held against the oceans it is meant to be like.
//
// The worlds are not Earth: their oceans are where their plates put them, as
// broad or as narrow as that, and no reading here can ask for the Gulf Stream
// by name. So each yardstick is a rule that holds on any geometry - a western
// current carries back what the interior of its gyre drives the other way, an
// eastern shore in the subtropics is cold - with Earth's figure as the check
// where the geometry allows. The plan they are written for is
// docs/ocean-model-plan.md, "What it is measured against"; issue #17.
//
// Every reading is taken on yardWorld's globe, on the air's own cells, which
// are what the currents are worked out on (atmos.Env.currents): a tile reads
// its current back off them by interpolation, and a reading of the tiles would
// be a reading of the interpolation. Every share and every mean is weighted by
// a cell's area, which on the cylinder goes as the cosine of its latitude.
//
// A reading the present sea can make is asserted. One that needs a step of
// the ocean model that has not been taken - a heat transport the energy
// balance feels, a flow that runs round islands, a thermocline, salt, an
// overturning, a sea-ice model - is read as far as the present fields allow,
// logged, and skipped with the issue that will give it what it lacks. When
// that issue lands, it takes its Skip out and the reading starts holding.
// Each test logs its number every run, so that every step can say which
// readings it moved.

// The ocean's yardsticks, read on cells.
const (
	// oceanBoundary is how many cells from a shore the current against it is
	// read in: the issue's "within 2 cells". The globe's air cells are some
	// 78 km down and 60-75 km across in the subtropics, so two are the 150 km
	// the model gives a western current (atmos.westWall) and about what the
	// Gulf Stream and the Kuroshio are across.
	oceanBoundary = 2
	// oceanInterior is how many cells from any shore a current has to be, along
	// its row, to be the interior's: some 300 km, clear of both boundaries.
	oceanInterior = 4
	// oceanLeast is how broad, in metres, a stretch of sea along a row has to
	// be to have a gyre read in it: a thousand kilometres, the breadth of the
	// narrowest basin with a western boundary current of its own (the
	// Tasman Sea's East Australian Current; the Mozambique Channel's is a
	// string of eddies). A bay or a strait is not an ocean.
	oceanLeast = 1000e3
	// oceanOpen is the share of the way round a parallel a stretch of sea has
	// to run for its shores not to close a gyre: atmos.gyreOpen's.
	oceanOpen = 0.8
	// oceanColumn is the depth, in metres, a surface speed is taken to run
	// to when it is read as a transport, where a reading has nothing better:
	// the old atmos.gyreDepth. Since M2 (#21) the gyres' current is their
	// transport spread over the warm water above the thermocline, never less
	// than atmos.FlowLeast, so a transport read this way is only roughly
	// one; the Sverdrup closure reads the streamfunction instead (see
	// gyreTransport).
	oceanColumn = 300.0
	// sverdrup is a cubic hectometre a second, the oceanographer's unit of
	// transport: 10^6 m^3/s.
	sverdrup = 1e6
	// petawatt is 10^15 W.
	petawatt = 1e15
)

// oceanCells is the sea as its currents were worked out: the air's cells and
// what the currents kept on them.
type oceanCells struct {
	g   *Grid
	w   *atmos.Winds
	e   *atmos.Env
	lat []float64 // each row of cells' latitude, degrees
}

// theGlobesOcean is the sea of yardWorld's globe. It skips under -short: the globe
// is shared with the other yardsticks, but it is still a globe.
func theGlobesOcean(t *testing.T) *oceanCells {
	t.Helper()
	if testing.Short() {
		t.Skip("needs the full globe")
	}
	return oceanOn(t, yardWorld("globe", 1, GlobeTerms()))
}

// oceanOn is the sea of g, which must have had its weather read on a globe.
func oceanOn(t *testing.T, g *Grid) *oceanCells {
	t.Helper()
	if g.winds == nil || g.winds.Env == nil || g.winds.Cu == nil {
		t.Fatal("the world's sea has no currents kept: is it a globe, and has its weather been read?")
	}
	e := g.winds.Env
	o := &oceanCells{g: g, w: g.winds, e: e, lat: make([]float64, e.H)}
	for cy := range o.lat {
		for y := cy * e.Cell; y < (cy+1)*e.Cell; y++ {
			o.lat[cy] += g.air.Lat[y]
		}
		o.lat[cy] /= float64(e.Cell)
	}
	return o
}

// at is the cell at cx, cy, round the seam.
func (o *oceanCells) at(cx, cy int) int {
	w := o.e.W
	return cy*w + ((cx%w)+w)%w
}

// wet reports whether the cell at cx, cy is sea, as the currents read it.
func (o *oceanCells) wet(cx, cy int) bool { return o.e.Sea[o.at(cx, cy)] > 0.5 }

// weight is the area of a cell on row cy, in square metres: what every mean
// and every share here is weighted by.
func (o *oceanCells) weight(cy int) float64 { return o.e.Dx[cy] * o.e.Dy }

// rowsBetween are the rows of cells whose latitude lies between lo and hi
// degrees, either way round, signed: lo -40, hi -15 is the southern
// subtropics.
func (o *oceanCells) rowsBetween(lo, hi float64) []int {
	lo, hi = min(lo, hi), max(lo, hi)
	var rows []int
	for cy, l := range o.lat {
		if l >= lo && l < hi {
			rows = append(rows, cy)
		}
	}
	return rows
}

// stretch is a run of sea along a row of cells from its western shore to its
// eastern: cells first to last, counted eastward and not wrapped, so that
// last may run past the seam (o.at wraps it).
type stretch struct {
	cy          int
	first, last int
}

func (s stretch) cells() int { return s.last - s.first + 1 }

// stretches are the runs of sea between shores on row cy that are broad
// enough to be an ocean and not so nearly all the way round that their
// shores do not close it: the stretches a gyre is read in. A row with no land
// has none.
func (o *oceanCells) stretches(cy int) []stretch {
	w := o.e.W
	start := -1
	for cx := 0; cx < w; cx++ {
		if !o.wet(cx, cy) && o.wet(cx+1, cy) {
			start = cx + 1
			break
		}
	}
	if start < 0 {
		return nil
	}
	var out []stretch
	for k := 0; k < w; {
		if !o.wet(start+k, cy) {
			k++
			continue
		}
		first := k
		for k < w && o.wet(start+k, cy) {
			k++
		}
		s := stretch{cy: cy, first: start + first, last: start + k - 1}
		breadth := float64(s.cells()) * o.e.Dx[cy]
		if breadth >= oceanLeast && float64(s.cells()) <= oceanOpen*float64(w) {
			out = append(out, s)
		}
	}
	return out
}

// current is the sea's current over cell i, metres a second toward the east
// and the north.
func (o *oceanCells) current(i int) (east, north float64) {
	return float64(o.e.Cu[i]), float64(o.e.Cv[i])
}

// gyreTransport is what the gyre carries toward the north across cell
// (cx, cy), in cubic metres a second through the whole of the moving water:
// the rise of its streamfunction across the cell (M1's Psi, flow.go), the
// same central difference atmos.currents reads the current off. It is the
// gyre's own, with no wind's drift in it, and no depth has to be guessed to
// make a transport of a speed: M2 (#21) asked the closure to be read so,
// once the surface current came to be ψ over the thermocline's depth.
func (o *oceanCells) gyreTransport(cx, cy int) float64 {
	return float64(o.e.Psi[o.at(cx+1, cy)]-o.e.Psi[o.at(cx-1, cy)]) / 2 * atmos.Sverdrup
}

// waterTemp is the year's mean temperature of the water over cell i, degrees.
func (o *oceanCells) waterTemp(i int) float64 { return float64(o.e.WaterTemp[i]) }

// zonalSeaMean is the mean of read over the sea of row cy: the zonal mean a
// boundary's anomaly is read against. Every cell on a row has the same area.
func (o *oceanCells) zonalSeaMean(cy int, read func(i int) float64) float64 {
	var s, n float64
	for cx := 0; cx < o.e.W; cx++ {
		if o.wet(cx, cy) {
			s, n = s+read(o.at(cx, cy)), n+1
		}
	}
	if n == 0 {
		return math.NaN()
	}
	return s / n
}

// basins labels the sea within rows: each cell of sea on those rows is given
// the number of the body of water it is part of there, joined across the
// four sides of a cell and round the seam, and -1 elsewhere. A subtropical
// band cut out of the globe this way falls into its gyres' basins.
func (o *oceanCells) basins(rows []int) []int {
	e := o.e
	label := make([]int, e.W*e.H)
	for i := range label {
		label[i] = -1
	}
	in := make([]bool, e.H)
	for _, cy := range rows {
		in[cy] = true
	}
	next := 0
	var queue []int
	for _, cy := range rows {
		for cx := 0; cx < e.W; cx++ {
			i := o.at(cx, cy)
			if label[i] >= 0 || !o.wet(cx, cy) {
				continue
			}
			label[i] = next
			queue = append(queue[:0], i)
			for k := 0; k < len(queue); k++ {
				j := queue[k]
				jx, jy := j%e.W, j/e.W
				for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					ny := jy + d[1]
					if ny < 0 || ny >= e.H || !in[ny] {
						continue
					}
					n := o.at(jx+d[0], ny)
					if label[n] < 0 && o.wet(jx+d[0], ny) {
						label[n] = next
						queue = append(queue, n)
					}
				}
			}
			next++
		}
	}
	return label
}

// gyre is a subtropical gyre's two halves: what its western boundary current
// carries toward the pole and what its interior carries toward the equator,
// each in cubic metres a second, summed over the rows the gyre spans, so a
// gyre's is the mean over its rows times how many there are.
type gyre struct {
	basin    int
	rows     int     // how many rows it has an ocean on
	last     int     // the last of them
	lo, hi   float64 // the latitudes it spans
	boundary float64 // poleward, positive
	interior float64 // equatorward, positive
}

// closure is the gyre's western boundary transport over its interior's: one
// when the boundary carries back all the interior drives.
func (g gyre) closure() float64 { return g.boundary / g.interior }

// gyresOf are the gyres of the band of latitudes between lo and hi, signed:
// each stretch of ocean on each row of it is split at oceanBoundary cells
// from its western shore, and the meridional transport of the two parts is
// summed over the basin the stretch is part of. Only basins with an ocean on
// least rows are kept: a gyre spans more than a bay does. The northward
// transport read is the gyre's own (gyreTransport), not the wind's drift on it.
func (o *oceanCells) gyresOf(lo, hi float64, least int) []gyre {
	hemi := math.Copysign(1, lo+hi)
	rows := o.rowsBetween(lo, hi)
	label := o.basins(rows)
	byBasin := map[int]*gyre{}
	for _, cy := range rows {
		for _, s := range o.stretches(cy) {
			b := label[o.at(s.first, cy)]
			gy := byBasin[b]
			if gy == nil {
				gy = &gyre{basin: b, lo: math.Inf(1), hi: math.Inf(-1)}
				byBasin[b] = gy
			}
			if gy.rows == 0 || gy.last != cy {
				gy.rows, gy.last = gy.rows+1, cy
			}
			gy.lo, gy.hi = min(gy.lo, o.lat[cy]), max(gy.hi, o.lat[cy])
			for cx := s.first; cx <= s.last; cx++ {
				q := o.gyreTransport(cx, cy) * hemi // poleward
				if cx < s.first+oceanBoundary {
					gy.boundary += q
				} else {
					gy.interior -= q
				}
			}
		}
	}
	var out []gyre
	for _, gy := range byBasin {
		if gy.rows >= least {
			out = append(out, *gy)
		}
	}
	slices.SortFunc(out, func(a, b gyre) int { return a.basin - b.basin })
	return out
}

// subtropics are the latitudes the subtropical gyres are read between, each
// hemisphere: from the trades' strongest, where the wind's curl turns, to
// the westerlies' (Hellerman and Rosenstein 1983).
var subtropics = [2][2]float64{{15, 40}, {-40, -15}}

// gyreRowsLeast is how many rows of cells a basin has to have an ocean on,
// in the subtropics, to be read as a gyre: some seven degrees.
const gyreRowsLeast = 10

// 1. Sverdrup closure. What the wind's curl drives toward the equator across
// the interior of a subtropical gyre comes back toward the pole in the
// narrow current against its western shore, and the two balance to within
// what friction takes (Stommel 1948; Munk 1950). On Earth the Florida Current
// carries some 31 Sv at 27°N (Baringer and Larsen 2001) against an interior
// Sverdrup transport of 25-35 Sv (Leetmaa, Niiler and Stommel 1977; Wunsch
// and Roemmich 1985). The reading is the boundary over the interior, gyre by
// gyre, over each hemisphere's subtropics, of the gyre's own transport, read
// off its streamfunction (gyreTransport). It was read off the surface current
// less the wind's drift over a column of oceanColumn, while the gyres were
// worked out row by row; the wind's drift, which on its own runs poleward
// under the trades across the whole ocean, had to be taken out (read with
// the gyre it turned every closure negative, -0.65 together). The transport-weighted
// closure of all the gyres together is held to 0.8-1.25 - a fifth either
// way, the scatter of the Atlantic's own budget over the papers above - and
// every gyre to 0.5-2.
//
// The gyres were made to close row by row until M1 (#19) solved the flow in
// two dimensions (flow.go): what this reads is what it earns, with part of
// the return going round islands at the level the island rule gives them.
func TestOceanSverdrupClosure(t *testing.T) {
	o := theGlobesOcean(t)
	var boundary, interior float64
	n := 0
	for _, band := range subtropics {
		for _, gy := range o.gyresOf(band[0], band[1], gyreRowsLeast) {
			t.Logf("gyre at %.0f to %.0f degrees over %d rows: the western boundary carries %.1f Sv poleward, the interior %.1f Sv equatorward: closure %.2f",
				gy.lo, gy.hi, gy.rows, gy.boundary/sverdrup/float64(gy.rows), gy.interior/sverdrup/float64(gy.rows), gy.closure())
			boundary += gy.boundary
			interior += gy.interior
			n++
			if c := gy.closure(); !(c >= 0.5 && c <= 2) {
				t.Errorf("the gyre at %.0f to %.0f degrees closes at %.2f, real 0.5-2 (Stommel 1948; Munk 1950)", gy.lo, gy.hi, c)
			}
		}
	}
	if n == 0 {
		t.Fatal("the globe has no subtropical gyre to read")
	}
	all := boundary / interior
	t.Logf("%d gyres close at %.3f together", n, all)
	if !(all >= 0.8 && all <= 1.25) {
		t.Errorf("the gyres close at %.3f together, real 0.8-1.25 (Stommel 1948; Munk 1950; Baringer and Larsen 2001; Leetmaa et al. 1977)", all)
	}
}

// westernPeak is the strongest poleward current within oceanBoundary cells
// of a western shore, over the rows between lo and hi degrees, and the
// fastest current of the interior on its row and the rows either side of it.
type westernPeak struct {
	lat, speed, poleward float64
	interior             float64
}

func (o *oceanCells) westernPeak(lo, hi float64) westernPeak {
	hemi := math.Copysign(1, lo+hi)
	var best westernPeak
	bestRow := -1
	for _, cy := range o.rowsBetween(lo, hi) {
		for _, s := range o.stretches(cy) {
			for cx := s.first; cx < s.first+oceanBoundary && cx <= s.last; cx++ {
				u, v := o.current(o.at(cx, cy))
				if v*hemi > best.poleward {
					best = westernPeak{lat: o.lat[cy], speed: math.Hypot(u, v), poleward: v * hemi}
					bestRow = cy
				}
			}
		}
	}
	if bestRow < 0 {
		return westernPeak{lat: math.NaN(), speed: math.NaN(), poleward: math.NaN(), interior: math.NaN()}
	}
	for cy := max(bestRow-1, 0); cy <= min(bestRow+1, o.e.H-1); cy++ {
		for _, s := range o.stretches(cy) {
			for cx := s.first + oceanInterior; cx <= s.last-oceanInterior; cx++ {
				u, v := o.current(o.at(cx, cy))
				best.interior = max(best.interior, math.Hypot(u, v))
			}
		}
	}
	return best
}

// 2. Western intensification. The current a gyre turns back in against its
// western shore is the fastest on its latitude: the Gulf Stream runs at
// 1-2 m/s off the Carolinas, the Kuroshio at 1-1.5, the Agulhas at 1.5-2,
// the Brazil and the East Australian at half a metre to a metre, where the
// interior of the gyres they close drifts at centimetres (Lumpkin and Johnson
// 2013, the drifter climatology; Stommel 1948). The reading is the strongest
// poleward current within two cells of a western shore in each hemisphere's
// subtropics, held to 0.5-2 m/s and to more than any current in the interior
// of the ocean at its latitude. Two is atmos.currentMost, the most any
// current is let run, and a peak that stands on it is a clamp, not a current.
func TestOceanWesternIntensification(t *testing.T) {
	o := theGlobesOcean(t)
	for _, band := range subtropics {
		p := o.westernPeak(band[0], band[1])
		t.Logf("%.0f to %.0f degrees: the strongest western current runs %.2f m/s (%.2f poleward) at %.1f degrees, the interior's fastest there %.2f m/s: %.1fx",
			band[0], band[1], p.speed, p.poleward, p.lat, p.interior, p.speed/p.interior)
		if !(p.speed >= 0.5 && p.speed < 2) {
			t.Errorf("%.0f to %.0f degrees: the western current runs %.2f m/s, real 0.5-2 (Lumpkin and Johnson 2013)", band[0], band[1], p.speed)
		}
		if !(p.speed > p.interior) {
			t.Errorf("%.0f to %.0f degrees: the western current runs %.2f m/s and the interior %.2f (Stommel 1948)", band[0], band[1], p.speed, p.interior)
		}
	}
}

// easternCold is how many degrees the water within oceanBoundary cells of
// the eastern shores of the oceans between lo and hi degrees stands under
// its row's zonal mean, area-weighted, and over how many cells.
func (o *oceanCells) easternCold(lo, hi float64) (anomaly float64, cells int) {
	var s, w float64
	for _, cy := range o.rowsBetween(lo, hi) {
		mean := o.zonalSeaMean(cy, o.waterTemp)
		for _, st := range o.stretches(cy) {
			for cx := max(st.first, st.last-oceanBoundary+1); cx <= st.last; cx++ {
				s += o.weight(cy) * (o.waterTemp(o.at(cx, cy)) - mean)
				w += o.weight(cy)
				cells++
			}
		}
	}
	return s / w, cells
}

// 3. Eastern boundary cold. The water a gyre brings back toward the equator
// down an ocean's eastern side is cold from higher latitudes, and where the
// trades drive the surface water off the shore colder water comes up from
// under it: the Humboldt off Peru and the Benguela off Namibia stand 3-8
// degrees under the zonal mean of their latitude's sea surface (Reynolds et
// al. 2002, OISST; Carr and Kearns 2003 for the four eastern boundary
// upwelling systems), and the Canary and the California a little less. The
// reading is the water within two cells of an eastern shore between 15 and
// 30 degrees, against its row's zonal mean of the sea, each hemisphere,
// area-weighted. The south's read -2.83 with the upwelled water a fixed
// contrast under the mean; the thermocline of #21 (M2), which the trades lift
// toward the east, closed that gap (-3.10 on M2's own branch), and M2 asked
// its marker to come off.
func TestOceanEasternBoundaryCold(t *testing.T) {
	o := theGlobesOcean(t)
	for _, b := range []struct {
		lo, hi float64
		gap    string
	}{
		{15, 30, ""},
		{-30, -15, ""},
	} {
		t.Run(fmt.Sprintf("%.0f to %.0f", b.lo, b.hi), func(t *testing.T) {
			cold, cells := o.easternCold(b.lo, b.hi)
			t.Logf("the water against the eastern shores stands %+.2f degrees on its zonal mean, over %d cells", cold, cells)
			in := -cold >= 3 && -cold <= 8
			switch {
			case b.gap != "" && in:
				t.Errorf("the eastern boundary is %+.2f degrees on the zonal mean, inside -3 to -8: the gap has closed, take the marker off (%s)", cold, b.gap)
			case b.gap != "":
				t.Skipf("%s (got %+.2f degrees, real -3 to -8)", b.gap, cold)
			case !in:
				t.Errorf("the eastern boundary is %+.2f degrees on the zonal mean, real -3 to -8 (Reynolds et al. 2002; Carr and Kearns 2003)", cold)
			}
		})
	}
}

// meridionalHeat is the heat the currents carry across row cy toward the
// north, in watts: the sea's density and heat a degree, the current's speed
// over oceanColumn of water, and the water's temperature against the row's
// mean - which takes out the heat a sea that does not close its mass across
// a parallel would otherwise carry for nothing. It is the gyres' share only:
// a sea of one layer has no overturning under it.
func (o *oceanCells) meridionalHeat(cy int) float64 {
	const heat = 4.1e6 // atmos.seaHeat, J a cubic metre a degree
	mean := o.zonalSeaMean(cy, o.waterTemp)
	var s float64
	for cx := 0; cx < o.e.W; cx++ {
		if !o.wet(cx, cy) {
			continue
		}
		i := o.at(cx, cy)
		_, v := o.current(i)
		s += heat * v * oceanColumn * (o.waterTemp(i) - mean) * o.e.Dx[cy]
	}
	return s
}

// rowAt is the row of cells nearest lat degrees.
func (o *oceanCells) rowAt(lat float64) int {
	best := 0
	for cy, l := range o.lat {
		if math.Abs(l-lat) < math.Abs(o.lat[best]-lat) {
			best = cy
		}
	}
	return best
}

// 4. Ocean meridional heat transport. The ocean carries some 2 PW toward the
// poles at its peak in the tropics, and at 35 degrees, where the atmosphere
// and the ocean together carry their most, 5.5-6 PW, the ocean's share is
// 22% in the north and 8% in the south (Trenberth and Caron 2001). The sea
// here carries its warmth from cell to cell (atmos.seaLinks) but the energy
// balance does not feel it: there is no heat transport of the ocean's for
// the atmosphere's to be a share of. The gyres' transport is read off the
// kept current and temperature as far as it can be, and the test waits on
// #22 (M3), which puts it into the energy balance.
func TestOceanMeridionalHeatTransport(t *testing.T) {
	o := theGlobesOcean(t)
	peak, at := 0.0, math.NaN()
	for cy := range o.lat {
		if q := o.meridionalHeat(cy); math.Abs(q) > math.Abs(peak) {
			peak, at = q, o.lat[cy]
		}
	}
	n35, s35 := o.meridionalHeat(o.rowAt(35)), o.meridionalHeat(o.rowAt(-35))
	t.Logf("the gyres carry %+.3f PW north at 35N and %+.3f at 35S; their peak is %+.3f PW at %.1f degrees",
		n35/petawatt, s35/petawatt, peak/petawatt, at)
	t.Skipf("needs #22 (M3): the energy balance does not feel the ocean's heat transport, so there is no total for it to be a share of; the gyres' own reads %+.2f PW at its peak, real about 2 (Trenberth and Caron 2001)", peak/petawatt)
}

// ringOfSea is the band of rows, between lo and hi degrees, that sea runs all
// the way round with no land on it: the gap a circumpolar current would go
// through. Its rows are first to last, or none at all.
func (o *oceanCells) ringOfSea(lo, hi float64) (rows []int) {
	for _, cy := range o.rowsBetween(lo, hi) {
		open := true
		for cx := 0; cx < o.e.W && open; cx++ {
			open = o.wet(cx, cy)
		}
		if open {
			rows = append(rows, cy)
		}
	}
	return rows
}

// zonalTransport is the transport toward the east through the narrowest
// section of sea across the ring of rows, from shore to shore down its
// column, in m^3/s over oceanColumn; and the column it was read at.
func (o *oceanCells) zonalTransport(ring []int) (transport float64, column int) {
	narrowest := math.MaxInt
	for cx := 0; cx < o.e.W; cx++ {
		top, bottom := ring[0], ring[len(ring)-1]
		for top > 0 && o.wet(cx, top-1) {
			top--
		}
		for bottom < o.e.H-1 && o.wet(cx, bottom+1) {
			bottom++
		}
		if bottom-top < narrowest {
			narrowest, column = bottom-top, cx
			transport = 0
			for cy := top; cy <= bottom; cy++ {
				u, _ := o.current(o.at(cx, cy))
				transport += u * o.e.Dy * oceanColumn
			}
		}
	}
	return transport, column
}

// 5. Circumpolar transport. Where sea runs all the way round a parallel there
// is no shore for a gyre to turn at, and the westerlies drive a current all
// the way round instead, held back only by friction and the form drag of the
// ridges under it: the Antarctic Circumpolar Current carries 173 Sv through
// Drake Passage (Donohue et al. 2016; 130-175 Sv over the estimates since
// Whitworth 1983). The present sea leaves such a ring with the wind's drift
// alone (atmos.gyreOpen): the flow that goes round is M1's, #19.
func TestOceanCircumpolarTransport(t *testing.T) {
	o := theGlobesOcean(t)
	var said []string
	for _, band := range [2][2]float64{{35, 70}, {-70, -35}} {
		ring := o.ringOfSea(band[0], band[1])
		if len(ring) == 0 {
			said = append(said, fmt.Sprintf("no ring of sea at %.0f to %.0f", band[0], band[1]))
			t.Logf("%.0f to %.0f degrees: no parallel runs all the way round in sea", band[0], band[1])
			continue
		}
		q, cx := o.zonalTransport(ring)
		t.Logf("%.0f to %.0f degrees: sea runs all the way round on %d rows (%.1f to %.1f degrees); through the narrowest section, at column %d, %+.1f Sv go east",
			band[0], band[1], len(ring), o.lat[ring[0]], o.lat[ring[len(ring)-1]], cx, q/sverdrup)
		said = append(said, fmt.Sprintf("%+.1f Sv at %.0f to %.0f", q/sverdrup, band[0], band[1]))
	}
	t.Skipf("needs #19 (M1): the present sea has no flow round a ring, only the wind's drift (%s), real 130-175 Sv (Donohue et al. 2016)", strings.Join(said, "; "))
}

// islands are the bodies of land on the air's cells, joined across a cell's
// four sides and round the seam, smaller than the largest - which is a
// continent, whatever else is - and clear of the polar rows: each with its
// cells.
func (o *oceanCells) islands() [][]int {
	e := o.e
	seen := make([]bool, e.W*e.H)
	var bodies [][]int
	for i := range seen {
		if seen[i] || e.Sea[i] > 0.5 {
			continue
		}
		seen[i] = true
		body := []int{i}
		for k := 0; k < len(body); k++ {
			j := body[k]
			jx, jy := j%e.W, j/e.W
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				ny := jy + d[1]
				if ny < 0 || ny >= e.H {
					continue
				}
				n := o.at(jx+d[0], ny)
				if !seen[n] && e.Sea[n] <= 0.5 {
					seen[n] = true
					body = append(body, n)
				}
			}
		}
		bodies = append(bodies, body)
	}
	slices.SortFunc(bodies, func(a, b []int) int { return len(b) - len(a) })
	var out [][]int
	for _, b := range bodies[min(1, len(bodies)):] {
		polar := false
		for _, i := range b {
			if cy := i / e.W; cy == 0 || cy == e.H-1 {
				polar = true
				break
			}
		}
		if !polar {
			out = append(out, b)
		}
	}
	return out
}

// strait is the meridional transport through the sea between an island and
// the nearest land west and east of it along the row through its middle, in
// m^3/s over oceanColumn, toward the north; and how many cells of sea each
// passage is.
func (o *oceanCells) strait(island []int) (west, east float64, westCells, eastCells int) {
	e := o.e
	var sy float64
	for _, i := range island {
		sy += float64(i / e.W)
	}
	cy := int(math.Round(sy / float64(len(island))))
	in := map[int]bool{}
	var xs []int
	for _, i := range island {
		in[i] = true
		if i/e.W == cy {
			xs = append(xs, i%e.W)
		}
	}
	if len(xs) == 0 {
		return math.NaN(), math.NaN(), 0, 0
	}
	// Each passage: walked from the island's shore until land.
	walk := func(from, step int) (q float64, n int) {
		cx := from + step
		for k := 0; k < e.W && o.wet(cx, cy); k++ {
			_, v := o.current(o.at(cx, cy))
			q += v * e.Dx[cy] * oceanColumn
			n++
			cx += step
		}
		return q, n
	}
	westmost, eastmost := slices.Min(xs), slices.Max(xs)
	west, westCells = walk(westmost, -1)
	east, eastCells = walk(eastmost, 1)
	return west, east, westCells, eastCells
}

// 6. Throughflow. The water that passes between an island and the far shore
// is set by the wind round a circuit that takes in the island: Godfrey's
// island rule (Godfrey 1989), which gives the Indonesian Throughflow at
// 16 ± 4 Sv (Gordon et al. 2010, INSTANT: 15 Sv). The present sea's gyres
// are worked out row by row and know nothing of an island's circuit: the
// passages beside the globe's largest islands are read, and the test waits
// on #19 (M1), whose streamfunction carries each island's constant.
func TestOceanThroughflow(t *testing.T) {
	o := theGlobesOcean(t)
	isles := o.islands()
	var said []string
	for k, isle := range isles[:min(3, len(isles))] {
		w, e, wn, en := o.strait(isle)
		t.Logf("island %d, %d cells: %+.1f Sv north through the %d cells of sea west of it, %+.1f through the %d east",
			k+1, len(isle), w/sverdrup, wn, e/sverdrup, en)
		said = append(said, fmt.Sprintf("%+.1f/%+.1f Sv", w/sverdrup, e/sverdrup))
	}
	t.Logf("%d islands on the air's cells", len(isles))
	t.Skipf("needs #19 (M1): no island rule in a sea solved row by row (the three largest islands' passages read %s), real 16 ± 4 Sv for the Indonesian case (Godfrey 1989)", strings.Join(said, ", "))
}

// equatorialContrast is how many degrees warmer the water of the western
// quarter of the broadest ocean on each row within lo degrees of the equator
// is than its eastern quarter, area-weighted over the rows.
func (o *oceanCells) equatorialContrast(lo float64) float64 {
	var s, w float64
	for _, cy := range o.rowsBetween(-lo, lo) {
		var broad stretch
		for _, st := range o.stretches(cy) {
			if st.cells() > broad.cells() {
				broad = st
			}
		}
		q := broad.cells() / 4
		if q == 0 {
			continue
		}
		var west, east float64
		for k := range q {
			west += o.waterTemp(o.at(broad.first+k, cy))
			east += o.waterTemp(o.at(broad.last-k, cy))
		}
		s += o.weight(cy) * (west - east) / float64(q)
		w += o.weight(cy)
	}
	return s / w
}

// equatorialWind is the year's mean wind toward the east, metres a second,
// over the broadest ocean on each row within lo degrees of the equator,
// area-weighted over the rows: what tilts an equatorial thermocline.
func (o *oceanCells) equatorialWind(lo float64) float64 {
	var s, w float64
	for _, cy := range o.rowsBetween(-lo, lo) {
		var broad stretch
		for _, st := range o.stretches(cy) {
			if st.cells() > broad.cells() {
				broad = st
			}
		}
		for cx := broad.first; cx <= broad.last && broad.cells() > 0; cx++ {
			i := o.at(cx, cy)
			for k := range atmos.Phases {
				s += o.weight(cy) * float64(o.w.U[k][i]) / atmos.Phases
			}
			w += o.weight(cy)
		}
	}
	return s / w
}

// 7. Equatorial west-east contrast. The trades pile warm water up in the west
// of an equatorial ocean and the thermocline comes up under the east, so the
// warm pool stands 4-6 degrees over the cold tongue (Locarnini et al. 2018,
// WOA; Wyrtki 1981). The contrast is read across the broadest ocean within
// five degrees of the equator, and the year's mean wind over the same water
// with it. M2 (#21) gave the sea a thermocline that tilts; what it lacks is
// the easterlies to tilt it on the equator itself, where the Pacific's year
// is -4 to -6 m/s: the year's mean ITCZ lies on the equator, in the
// doldrums, and the Walker circulation that the cold tongue itself drives
// (Bjerknes 1969) is #28, the sea and the air solved together, with A3 (#35)
// for the wind's own east-west structure. M2 asked the skip to name the wind
// rather than the thermocline. It fails once the contrast is in the band, to
// have its marker taken off.
func TestOceanEquatorialContrast(t *testing.T) {
	o := theGlobesOcean(t)
	c := o.equatorialContrast(5)
	u := o.equatorialWind(5)
	t.Logf("within 5 degrees of the equator the west of the broadest ocean stands %+.2f degrees over its east, under a year's mean wind of %+.2f m/s toward the east", c, u)
	if c >= 4 && c <= 6 {
		t.Errorf("the equatorial contrast reads %+.2f degrees, inside 4-6: the gap has closed, take the marker off", c)
		return
	}
	t.Skipf("known gap: #28 (sea and air solved together), #35 (A3) - no year's mean easterlies on the equator to tilt the thermocline (%+.2f m/s over the broadest ocean, the Pacific's -4 to -6); the contrast reads %+.2f degrees, real 4-6 (Locarnini et al. 2018)", u, c)
}

// seaIceShare is the share of the globe's whole surface, area-weighted, that
// is frozen sea: under the sea, and ice or freezing over the year.
func seaIceShare(g *Grid) float64 {
	var ice, all float64
	for y := 0; y < g.H; y++ {
		w := math.Cos(g.air.Lat[y] * math.Pi / 180)
		for x := 0; x < g.W; x++ {
			i := y*g.W + x
			all += w
			if g.underSea(i) && (g.Tiles[i].Terrain == Ice || g.Freezing(g.PosOf(i))) {
				ice += w
			}
		}
	}
	return ice / all
}

// 8. Sea ice. Over the year some 5% of the Earth's surface is under sea
// ice: 10-12 million km^2 in the Arctic and as much round Antarctica, of
// 510 (Fetterer et al. 2017, NSIDC Sea Ice Index; Parkinson 2014). The
// present ice is the sea whose year's mean is under SeaFreeze (Grid.Freezing),
// with no season and no thickness: the ice that is there all year. Held to
// 3-8%, the annual mean of the two hemispheres over the satellite record,
// widened to the Arctic's own decline since 1979.
func TestOceanSeaIceShare(t *testing.T) {
	if testing.Short() {
		t.Skip("needs the full globe")
	}
	g := yardWorld("globe", 1, GlobeTerms())
	share := seaIceShare(g)
	t.Logf("sea ice covers %.2f%% of the globe, area-weighted", 100*share)
	if !(share >= 0.03 && share <= 0.08) {
		t.Errorf("sea ice covers %.2f%% of the globe, real 3-8%% (Fetterer et al. 2017)", 100*share)
	}
}

// 9. Subtropical salinity. Where evaporation beats the rain under the
// subtropical highs the surface water is saltiest, some 37 psu in the North
// Atlantic's and 36.5 in the South Pacific's, against a world mean of 34.7
// (Antonov et al. 2010, WOA09). The sea has no salt: #23 (M4).
func TestOceanSubtropicalSalinity(t *testing.T) {
	theGlobesOcean(t)
	t.Skip("needs #23 (M4): the sea has no salinity field; real ~37 psu at the subtropical maxima against a 35 mean (Antonov et al. 2010)")
}

// 10. Overturning. The basin that overturns most carries 17.2 Sv north in its
// upper limb at 26.5°N, and 1.22 PW with it (McCarthy et al. 2015, RAPID).
// The sea has one layer and nothing to overturn: #24 (M5), which reads it
// off the layers of M2 and the salt of M4.
func TestOceanOverturning(t *testing.T) {
	theGlobesOcean(t)
	t.Skip("needs #24 (M5): one layer has no overturning; real 17.2 Sv and 1.22 PW at 26.5N (McCarthy et al. 2015)")
}
