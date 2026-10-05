package terra

import (
	"cmp"
	"math"
	"slices"

	"github.com/LukasSelin/terra/geom"
)

// The deep sea floor, and how fast the land is rising.
//
// A watered history used to hand its sea floor to the map the way it hands the
// land: by rank, from the foot of the continents down to BasinDepth under it.
// So the whole ocean floor of a planet lay within twenty metres of its sea, and
// nothing the history knew about the floor - how old its crust was, where the
// ridges had been - reached the map. The earth's floor is not like that. New
// crust at a ridge stands two and a half kilometres under the sea and sinks as
// it cools, as the root of its age, until the plate has cooled to the
// thickness it can hold and the floor flattens. Stein and Stein's GDH1 (1992)
// is
//
//	d = 2600 + 365 √t metres, t in millions of years, to 20 Myr
//	d = 5651 − 2473 exp(−0.0278 t) past it
//
// It was Parsons and Sclater's (1977), 2500 + 350 √t to 70 Myr and 6400 −
// 3200 exp(−t/62.8) past it, which is within a couple of hundred metres of
// GDH1 on young floor. On old floor it is not: Parsons and Sclater's comes
// toward 6.4 km and GDH1's toward 5.65, and GDH1 was fitted to the floor of
// the North Pacific and the Northwest Atlantic out to 160 Myr, which Parsons
// and Sclater's is too deep for. With the earth's spread of ages the old
// floor is most of the ocean, and laid at Parsons and Sclater's depths it
// heaped up at 5.5 to 6 km. The history dates every tile of crust by the
// epoch it was made in - see crust.born - so the floor is laid at the depth
// its age gives it.
//
// Not all of it. A continent does not end at a wall four kilometres high: its
// crust thins under a shelf and falls off the shelf's edge down the
// continental slope and rise to the abyssal plain over another hundred and
// fifty kilometres or so (Kennett 1982). How wide the shelf is is the margin's
// own story. Where the continent and the floor beside it ride one plate, as
// the Atlantic's do, the margin has been quiet since it rifted, its stretched
// crust sinking and filling with the land's mud, and the shelf is wide:
// Harris and others (2014) have 88 kilometres on the mean of the earth's
// passive margins (Shepard 1963 had 78). Where the floor is another plate's,
// going down a trench or grinding past, as round the Pacific, the margin is
// young and steep and the shelf narrow: 31 kilometres on the mean of the
// active ones. So the floor keeps its rank-laid depth for quietShelf or
// activeShelf out from continental crust, by which plate the nearest of that
// crust rides, and comes down to its age's depth over the slopeWidth beyond,
// all read in the metres a history's tile is. A tile is thirty-seven
// kilometres of the globe, so a quiet shelf is two and a third of them, an
// active one the least a shelf is laid, a tile and a half (see shelfLeast),
// and the slope four; on a small globe, a tile of a hundred, both are a tile
// and a half and the slope one and a half.
//
// The ground a settlement lives on cannot carry that. A map's tile is also
// TileSpan wide, twenty-five metres, and every pass that reads a slope reads
// it at that: a floor four kilometres down beside a shelf twenty metres down
// is a fall of a hundred and sixty in one, and the slides would cut the shelf,
// the coast and the continent behind it back to Critical off it, which is
// every tile within three kilometres of height of the abyss - all the land
// there is. So the deep floor is out of reach of what the air and the rain do,
// which is true of the earth's too: nothing creeps or slides or is cut by a
// river under four kilometres of water, and what does slide down a continental
// slope is a thing a tile a quarter of a hundred metres wide says nothing true
// about. See abyssal, and the slides and the creep that ask it.
//
// The sea is still poured, and the level is still read off how much water
// there is. The deep floor holds its own water: a basin laid below the level
// the sea stands at holds the room it was deepened by, whole, and the water a
// world is given - Terms.Water - is the rest, which is what stands over the
// shelves and the low ground of the continents and says where the coast is.
// That is the earth's arrangement as well as the map's: the sea has stood
// within a couple of hundred metres of the continents' edges for as long as
// there have been continents, whatever its floor was doing (Wise 1974 on the
// constancy of continental freeboard), because the sea is most of the way to
// the top of its basins and the edges are where it runs out of basin.
const (
	ridgeDepth  = 2600.0 // metres under the sea, at the ridge
	sinkRate    = 365.0  // metres per root million years
	flattenAge  = 20.0   // million years
	oldDepth    = 5651.0 // metres, what the old floor comes toward
	oldSpan     = 2473.0
	oldTime     = 1 / 0.0278 // million years
	quietShelf  = 88.2 * km
	activeShelf = 31 * km
	slopeWidth  = 150 * km
)

// shelfLeast is the narrowest a shelf is laid, in tiles: wide enough that
// every tile round the edge of a continent is shelf, the four beside it and
// the four at its corners, a root of two away. The ground beside the sea is
// laid at a map's tile, and the deep floor beside it would be a fall of
// kilometres in one: see above, and laidHeight. It was one tile when awayFrom
// counted a diagonal step as one, and one tile was then the whole ring; at the
// true distance the corners of the ring lie past it, and a small globe, whose
// shelf is a tile, had its floor laid a third of the way down its slope at the
// corner of every coast: the shaping, which lays the land over the water it
// drains to, laid all of it under the sea.
const shelfLeast = 1.5

// floorDepth is how far under the sea floor of an age of t million years
// lies, in metres. See GDH1 above.
func floorDepth(t float64) float64 {
	t = math.Max(0, t)
	if t <= flattenAge {
		return ridgeDepth + sinkRate*math.Sqrt(t)
	}
	return oldDepth - oldSpan*math.Exp(-t/oldTime)
}

// floorDepths is, for each tile of ocean crust at the end of a history of
// epochs, how deep under the sea its age lays its floor and how much of that
// depth it is given for how far it is from continental crust: nothing on the
// shelf, all of it past the slope. The shelf is a quiet margin's where the
// nearest continental crust rides the tile's own plate, or one welded to it,
// and an active margin's where it rides another. Continental crust is given
// nothing. It is read while the tiles are still a history's, at their deep
// span, and once the plates are kept: see keepPlates.
//
// A tile's age is the middle of the epoch its crust was made in, to the end of
// the history. The crust the first plates broke is the whole history old, and
// as old again as it already was when the history began: see firstFloorAges.
// ages is each tile's, in millions of years, and NaN on continental crust,
// and sediment the metres of it the floor carries: see floorSediment. The
// depth is the floor's under that sediment.
func (g *Grid) floorDepths(cr *crust, epochs int) (depth, share, ages, sediment []float64) {
	n := len(g.Tiles)
	depth, share, ages, sediment = make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
	away, near := g.nearestTo(func(i int) bool { return !cr.ocean[i] }, true)
	span := g.span()
	quiet := math.Max(shelfLeast, tilesAcross(quietShelf, span))
	active := math.Max(shelfLeast, tilesAcross(activeShelf, span))
	slope := math.Max(1, tilesAcross(slopeWidth, span))
	for i := range g.Tiles {
		ages[i] = math.NaN()
		if !cr.ocean[i] {
			continue
		}
		shelf := quiet
		if k := near[i]; k >= 0 && g.rootPlate(g.Tiles[k].Plate) != g.rootPlate(g.Tiles[i].Plate) {
			shelf = active
		}
		age := (float64(epochs) - float64(cr.born[i]) - 0.5) * epochYears
		if cr.born[i] == 0 {
			age = float64(epochs)*epochYears + float64(cr.aged[i])
		}
		ages[i] = age / myr
		beyond := math.Max(0, away[i]-shelf-slope) * span
		sediment[i] = floorSediment(age, g.seaWarmth(i), beyond)
		depth[i] = floorDepth(ages[i]) - sedimentLoad*sediment[i]
		share[i] = smooth(clamp01((away[i] - shelf) / slope))
	}
	return depth, share, ages, sediment
}

// The floor's sediment.
//
// Parsons and Sclater's depths are the basement's, the basalt, which is what
// they fitted once the sediment over it was taken off. The floor a ship
// sounds is the top of that sediment, and it is not a few metres: the deep
// sea floor carries hundreds of metres of it, and the abyssal plains off the
// continents kilometres (Divins 2003; Straume and others 2019). What the
// history lays on its floor each epoch is squeezed with the rest of its beds
// into the drawn map's spread of heights, and a globe's floor came out with
// some twenty metres. So the floor is given what its age would have gathered.
//
// Three things settle on it. The shells of the plankton, the calcareous ooze,
// at oozeRate, a centimetre a thousand years, where the sea over it is warm -
// as the history's lime is, limeCold to limeWarm - and only while the floor
// is shallower than the carbonate compensation depth: below some four and a
// half kilometres the deep water dissolves the shells as fast as they fall
// (Berger and Winterer 1974), and new floor, which rises at the ridge 2.6 km
// down, sinks past it in some twenty-seven million years. The red clay, the dust
// and the ash that reach every floor, at clayRate, a millimetre a thousand
// years (Kennett 1982). And off the continents the turbidites the rivers'
// mud comes down the slopes in, which build the abyssal plains: apronThick at
// the foot of the slope, thinning by e over apronWidth and laid over the
// first apronYears of the floor's life. The apron's figures are chosen, not
// measured.
//
// A floor under sediment stands higher than one without it, and not by the
// sediment's whole thickness: the sediment weighs on the plate and pushes it
// down into the mantle by what the water it put out weighed less. So the
// surface stands sedimentLoad of the sediment higher, (3300 - 2000) / (3300 -
// 1030) for a sediment of 2000 kilograms a cubic metre.
const (
	oozeRate     = 0.01 * mm / yr
	clayRate     = 0.001 * mm / yr
	carbonateCCD = 4500.0 // metres under the sea
	apronThick   = 1500.0 // metres
	apronWidth   = 300 * km
	apronYears   = 20 * myr
	sedimentLoad = (3300.0 - 2000.0) / (3300.0 - 1030.0)
)

// floorSediment is how many metres of sediment floor age years old gathers
// under a sea whose year averages warm degrees, beyond metres out from the
// foot of the nearest continent's slope.
func floorSediment(age, warm, beyond float64) float64 {
	apron := apronThick * math.Exp(-beyond/apronWidth) * math.Min(1, age/apronYears)
	return oozeOf(age, warm) + clayRate*age + apron
}

// oozeOf is the metres of that which are ooze: what fell while the floor was
// above the compensation depth.
func oozeOf(age, warm float64) float64 {
	return oozeRate * clamp01((warm-limeCold)/(limeWarm-limeCold)) * math.Min(age, sinksPastCCD)
}

// sinksPastCCD is how old floor is when it sinks past carbonateCCD: floorDepth
// turned round.
var sinksPastCCD = func() float64 {
	if t := math.Pow((carbonateCCD-ridgeDepth)/sinkRate, 2); t <= flattenAge {
		return t * myr
	}
	return -oldTime * math.Log((oldDepth-carbonateCCD)/oldSpan) * myr
}()

// seaWarmth is the year's mean over tile i's row, which is what the sea over
// it is read at: see quietFloor.
func (g *Grid) seaWarmth(i int) float64 {
	if g.air != nil && i/g.W < len(g.air.Mean) {
		return g.air.Mean[i/g.W]
	}
	return MeanTemp
}

// oldestFloor is the age of the oldest ocean floor a planet keeps: older floor
// has gone down a trench. Müller and others (2008) date the earth's oldest in
// place at some 180 million years, in the western Pacific.
const oldestFloor = 180 * myr

// firstFloorAges dates the first plates' ocean floor. The history takes up a
// planet that has had oceans for as long as it has had plates, and the floor
// the first plates carry was made by ridges before the history began; dated
// all from its start, half of a globe's sea floor came out one age, and all of
// it lay in one band of depth. So it is given ages as the earth's floor has
// them: the area of floor falls with its age as a straight line to nothing at
// the oldest (Sclater and others 1980; Parsons 1982), which puts a tile's age
// at T(1 - sqrt(1 - u)) for u the share of the floor younger than it. And they
// lie as the earth's do, youngest along the ridges and oldest furthest from
// them: the first plates' ocean floor is ranked by how far each tile is from
// the nearest seam between two ocean plates. T is the oldest the floor can be
// less the history still to come, so that none of it is older than
// oldestFloor at the end.
func (cr *crust) firstFloorAges(g *Grid, epochs int) {
	oldest := float64(oldestFloor) - float64(epochs)*epochYears
	if oldest <= 0 {
		return
	}
	seam := func(i int) bool {
		if !cr.ocean[i] {
			return false
		}
		p, k := g.PosOf(i), g.Tiles[i].Plate
		for _, off := range Dirs {
			q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
			if g.Wrap {
				q = g.Norm(q)
			}
			if !g.In(q) {
				continue
			}
			if j := g.Index(q); cr.ocean[j] && g.Tiles[j].Plate != k {
				return true
			}
		}
		return false
	}
	away := g.awayFrom(seam)
	var floor []int32
	for i := range g.Tiles {
		if cr.ocean[i] {
			floor = append(floor, int32(i))
		}
	}
	slices.SortStableFunc(floor, func(a, b int32) int { return cmp.Compare(away[a], away[b]) })
	for r, i := range floor {
		u := (float64(r) + 0.5) / float64(len(floor))
		cr.aged[i] = float32(oldest * (1 - math.Sqrt(1-u)))
	}
}

// awayFrom is how many tiles each tile is from the nearest tile from says
// yes to, straight across the map and round a globe's seam: the exact
// distance, by Felzenszwalb and Huttenlocher's (2012) two passes of lower
// envelopes, down the columns and then along the rows. A map with no such
// tile is everywhere as far away as the map is wide.
//
// It was a walk to the eight tiles round each, which counts a diagonal step
// as one, so that what it read was the larger of the two distances across:
// the shelf and the slope floorDepths lays at those distances came out an
// octagon round every coast and a square round every islet. Round coasts
// drawn as a Brownian relief's, which lean to neither the map's axes nor its
// diagonals, the floor it laid leaned to them by 0.22 to 0.25 (gridLock, in
// realism_shape_test.go); laid at the exact distance it leans as its coast
// does and no more, 0.013 to 0.066 round coasts that read 0.014 to 0.067.
func (g *Grid) awayFrom(from func(i int) bool) []float64 {
	away, _ := g.nearestTo(from, false)
	return away
}

// nearestTo is awayFrom, and where near is asked for, which of the tiles from
// says yes to lies nearest each tile: -1 where there is none.
func (g *Grid) nearestTo(from func(i int) bool, near bool) (away []float64, nearest []int32) {
	W, H := g.W, g.H
	span := W
	if g.Wrap {
		span = 3 * W // a row laid three times over, so the envelope goes round
	}
	m := max(span, H)
	f, d, v, z := make([]float64, m), make([]float64, m), make([]int, m), make([]float64, m+1)
	col := make([]float64, W*H)
	var site []int
	var row []int32 // the row of the nearest down each column
	if near {
		site, row = make([]int, m), make([]int32, W*H)
	}
	for x := 0; x < W; x++ {
		for y := 0; y < H; y++ {
			f[y] = farAway
			if from(y*W + x) {
				f[y] = 0
			}
		}
		envelope(f[:H], d, v, z, site)
		for y := 0; y < H; y++ {
			col[y*W+x] = d[y]
			if near {
				row[y*W+x] = int32(site[y])
			}
		}
	}
	away = make([]float64, W*H)
	if near {
		nearest = make([]int32, W*H)
	}
	for y := 0; y < H; y++ {
		for x := 0; x < span; x++ {
			f[x] = col[y*W+x%W]
		}
		envelope(f[:span], d, v, z, site)
		for x := 0; x < W; x++ {
			at := x
			if g.Wrap {
				at += W
			}
			away[y*W+x] = math.Sqrt(d[at])
			switch {
			case d[at] >= farAway:
				away[y*W+x] = float64(max(W, H))
				if near {
					nearest[y*W+x] = -1
				}
			case near:
				c := site[at] % W // the column, back from the row laid three times over
				nearest[y*W+x] = row[y*W+c]*int32(W) + int32(c)
			}
		}
	}
	return away, nearest
}

// farAway is the squared distance envelope gives a point with nothing to be
// near.
const farAway = 1e20

// envelope is the squared distance transform of f along one line: the lower
// envelope of a parabola standing on every point of f under farAway. Where
// site is given, it is where along the line each point's parabola stands.
func envelope(f, d []float64, v []int, z []float64, site []int) {
	cross := func(p, q int) float64 {
		return ((f[q] + float64(q*q)) - (f[p] + float64(p*p))) / float64(2*q-2*p)
	}
	n, k := len(f), -1
	for q := 0; q < n; q++ {
		if f[q] >= farAway {
			continue
		}
		if k < 0 {
			k, v[0], z[0], z[1] = 0, q, math.Inf(-1), math.Inf(1)
			continue
		}
		s := cross(v[k], q)
		for s <= z[k] {
			k--
			s = cross(v[k], q)
		}
		k++
		v[k], z[k], z[k+1] = q, s, math.Inf(1)
	}
	if k < 0 {
		for q := range n {
			d[q] = farAway
		}
		return
	}
	k = 0
	for q := 0; q < n; q++ {
		for z[k+1] < float64(q) {
			k++
		}
		d[q] = float64((q-v[k])*(q-v[k])) + f[v[k]]
		if site != nil {
			site[q] = v[k]
		}
	}
}

// layAbyss lays the deep floor, once the land has been handed its heights:
// each tile of ocean crust is brought from where basins put it toward
// BasinDepth under its age's depth - which is its age's depth under the sea,
// give or take the couple of metres of water the basins are left short of -
// by its share, and the beds under it go down with it. The height each tile
// stood at before is kept as the abyss: see laidHeight.
//
// The sediment the floor was given is laid into the pile under it, the
// basement sinking under it: the ooze as limestone and the clay and the
// turbidites as shale, in the order they came, so that floor that has sunk
// past the compensation depth carries its red clay over its chalk, as the
// deep Pacific does. What the pile already holds - the beds the history laid
// there - is not laid twice.
func (g *Grid) layAbyss(depth, share, sediment []float64) {
	g.abyss = make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		g.abyss[i] = math.NaN()
		if share[i] <= 0 {
			continue
		}
		to := g.Height[i] + share[i]*((BasinDepth-depth[i])-g.Height[i])
		by := g.Height[i] - to
		if by <= 0 {
			continue
		}
		g.abyss[i] = g.Height[i]
		g.Height[i] = to
		if g.strata == nil {
			continue
		}
		c := &g.strata[i]
		c.lift(-by)
		if sediment == nil {
			continue
		}
		thick := share[i]*sediment[i] - g.sedimentOn(i)
		age := g.FloorAge(i) * myr
		if !(thick > 0) || math.IsNaN(age) || sediment[i] <= 0 {
			continue
		}
		ooze := thick * oozeOf(age, g.seaWarmth(i)) / sediment[i]
		epoch := g.floorEpoch(i)
		if age > sinksPastCCD {
			c.bury(Limestone, epoch, 0, g.Height[i], ooze)
			c.bury(Shale, epoch, 0, g.Height[i], thick-ooze)
		} else {
			c.bury(Shale, epoch, 0, g.Height[i], thick-ooze)
			c.bury(Limestone, epoch, 0, g.Height[i], ooze)
		}
	}
}

// floorEpoch is the epoch tile i's floor was made in, and the first for floor
// made before the history began.
func (g *Grid) floorEpoch(i int) uint8 {
	age := g.FloorAge(i)
	if math.IsNaN(age) || g.epochs == 0 {
		return 0
	}
	return uint8(max(0, int(g.epochs)-1-int(age/(epochYears/myr))))
}

// FloorAge is how old tile i's ocean crust is, in millions of years: NaN
// where the crust is continent, or the map was not made by a watered history.
func (g *Grid) FloorAge(i int) float64 {
	if g.floorAge == nil || i < 0 || i >= len(g.floorAge) {
		return math.NaN()
	}
	return g.floorAge[i]
}

// FloorSediment is how many metres of beds lie over tile i's ocean basement,
// and NaN where the crust is continent: see floorSediment.
func (g *Grid) FloorSediment(i int) float64 {
	if math.IsNaN(g.FloorAge(i)) {
		return math.NaN()
	}
	return g.sedimentOn(i)
}

// sedimentOn is how many metres of beds lie over the basalt at the foot of
// tile i's pile, and nothing where the pile does not stand on basalt.
func (g *Grid) sedimentOn(i int) float64 {
	if g.strata == nil {
		return 0
	}
	c := &g.strata[i]
	foot := int(c.n) - 1
	if foot < 0 || c.rock[foot] != Basalt {
		return 0
	}
	return math.Max(0, float64(c.top[0])-float64(c.top[foot]))
}

// abyssal reports whether tile i is deep sea floor: ground out of reach of the
// rain, the creep and the slides. See above.
func (g *Grid) abyssal(i int) bool { return g.abyss != nil && !math.IsNaN(g.abyss[i]) }

// laidHeight is the height tile i stood at before the deep floor was laid,
// and laidSlope the fall from p read over those heights. A map reads some
// of what it is off its own spread of heights and slopes - how steep its
// steepest tenth is, how high its high ground - and the deep floor is not in
// that spread: its steps from the floor of one age to the next are
// kilometres over a tile, and counted in, the steepest tenth of a small globe
// came out at a fall of 33 in one, every tree and every outcrop on it read
// against that. Read so, they are what they were. On dry ground, which never
// stands beside the deep floor, both are the ground's own.
//
// And the air stands on the water, not on the floor under it. Over the deep
// floor what the weather reads a height off - how warm the air is by the
// lapse, how much it could take up, how high a chunk stands - is this height,
// within BasinDepth of the sea as the rest of the sea's floor is, and not five
// kilometres down: read off the floor, the open ocean came out thirty degrees
// warmer than its coast, and the rain off it moved on every map.
func (g *Grid) laidHeight(i int) float64 {
	if !g.abyssal(i) {
		return g.Height[i]
	}
	return g.abyss[i]
}

func (g *Grid) laidSlope(p geom.Pos) float64 {
	if g.abyss == nil {
		return g.Slope(p)
	}
	h := g.laidHeight(g.Index(p))
	steepest := 0.0
	for _, off := range Dirs {
		q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
		if !g.In(q) {
			continue
		}
		run := g.span()
		if off.X != 0 && off.Y != 0 {
			run *= math.Sqrt2
		}
		if d := (h - g.laidHeight(g.Index(q))) / run; d > steepest {
			steepest = d
		}
	}
	return steepest
}

// abyssWater is the water the deep floor holds of its own, as the depth it
// would come to spread over every tile: the room it was deepened by.
func (g *Grid) abyssWater() float64 {
	if g.abyss == nil {
		return 0
	}
	sum := 0.0
	for i, was := range g.abyss {
		if !math.IsNaN(was) {
			sum += was - g.Height[i]
		}
	}
	return sum / float64(len(g.Tiles))
}

// upliftMemory is how long a landscape takes to come to terms with a change in
// how fast its rock rises: the response time of a river network, which
// Whipple and Tucker (1999) put at a quarter of a million to two and a half
// million years for real ranges and Whittaker and Boulton (2012) measured at
// one to three in the Apennines. The rate a history hands the shaping is its
// rock uplift eased over that, so what the land has been doing for the last
// few million years is what its rivers are graded to, and a range that
// stopped rising two epochs ago is not.
const upliftMemory = 2.5 * myr

// upliftOf is how fast a history has lately been raising each tile's rock, in
// metres a year: the plate floating to its level, the bow it rides in, the
// seams and the hotspots, and none of what the weather took off. It is
// softened as the heights are, so that it lies where they do. See smoothing.
func (g *Grid) upliftOf(cr *crust) []float64 {
	u := append([]float64(nil), cr.rise...)
	for k := 0; k < g.passes(smoothing); k++ {
		u = g.spread(u)
	}
	return u
}
