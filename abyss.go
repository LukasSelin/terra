package terra

import (
	"math"

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
// it cools, as the root of its age: Parsons and Sclater (1977) fitted
//
//	d = 2500 + 350 √t metres, t in millions of years, to about 70 Myr
//	d = 6400 − 3200 exp(−t/62.8) past it
//
// where the plate has cooled to the thickness it can hold and the floor
// flattens; Stein and Stein's GDH1 (1992) is the same within a couple of
// hundred metres, 2600 + 365 √t to 20 Myr. The history dates every tile of
// crust by the epoch it was made in - see crust.born - so the floor is laid
// at the depth its age gives it.
//
// Not all of it. A continent does not end at a wall four kilometres high: its
// crust thins under a shelf, some eighty kilometres wide on the earth's
// passive margins (Shepard 1963 has a mean of 78), and falls off the shelf's
// edge down the continental slope and rise to the abyssal plain over another
// hundred and fifty or so (Kennett 1982). So the floor keeps its rank-laid
// depth for shelfWidth out from continental crust and comes down to its age's
// depth over the slopeWidth beyond, both read in the metres a history's tile
// is. A tile is thirty-seven kilometres of the globe, so a shelf is two of them
// and the slope four; on a small globe, a tile of a hundred, it is a tile of
// shelf and a slope of one and a half.
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
	ridgeDepth = 2500.0 // metres under the sea, at the ridge
	sinkRate   = 350.0  // metres per root million years
	flattenAge = 70.0   // million years
	oldDepth   = 6400.0 // metres, what the old floor comes toward
	oldSpan    = 3200.0
	oldTime    = 62.8 // million years
	shelfWidth = 80 * km
	slopeWidth = 150 * km
)

// floorDepth is how far under the sea floor of an age of t million years
// lies, in metres. See Parsons and Sclater above.
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
// shelf, all of it past the slope. Continental crust is given nothing. It is
// read while the tiles are still a history's, at their deep span.
//
// A tile's age is the middle of the epoch its crust was made in, to the end of
// the history. The crust the first plates broke is dated from the start of
// the history, which is also when the floor the first epoch opens is dated
// from; that floor is one epoch's in sixteen, and the difference is two
// million years.
func (g *Grid) floorDepths(cr *crust, epochs int) (depth, share []float64) {
	n := len(g.Tiles)
	depth, share = make([]float64, n), make([]float64, n)
	away := g.awayFrom(func(i int) bool { return !cr.ocean[i] })
	span := g.span()
	shelf := math.Max(1, tilesAcross(shelfWidth, span))
	slope := math.Max(1, tilesAcross(slopeWidth, span))
	for i := range g.Tiles {
		if !cr.ocean[i] {
			continue
		}
		age := float64(epochs) - float64(cr.born[i]) - 0.5
		if cr.born[i] == 0 {
			age = float64(epochs)
		}
		depth[i] = floorDepth(age * epochYears / myr)
		share[i] = smooth(clamp01((away[i] - shelf) / slope))
	}
	return depth, share
}

// awayFrom is how many tiles each tile is from the nearest tile from says
// yes to, stepping to any of the eight around it, round a globe's seam. A map
// with no such tile is everywhere as far away as the map is wide.
func (g *Grid) awayFrom(from func(i int) bool) []float64 {
	n := len(g.Tiles)
	away := make([]float64, n)
	queue := make([]int32, 0, n)
	for i := range away {
		away[i] = -1
		if from(i) {
			away[i] = 0
			queue = append(queue, int32(i))
		}
	}
	for k := 0; k < len(queue); k++ {
		i := queue[k]
		p := g.PosOf(int(i))
		for _, off := range Dirs {
			q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
			if g.Wrap {
				q = g.Norm(q)
			}
			if !g.In(q) {
				continue
			}
			if j := g.Index(q); away[j] < 0 {
				away[j] = away[i] + 1
				queue = append(queue, int32(j))
			}
		}
	}
	for i := range away {
		if away[i] < 0 {
			away[i] = float64(max(g.W, g.H))
		}
	}
	return away
}

// layAbyss lays the deep floor, once the land has been handed its heights:
// each tile of ocean crust is brought from where basins put it toward
// BasinDepth under its age's depth - which is its age's depth under the sea,
// give or take the couple of metres of water the basins are left short of -
// by its share, and the beds under it go down with it. The height each tile
// stood at before is kept as the abyss: see laidHeight.
func (g *Grid) layAbyss(depth, share []float64) {
	g.abyss = make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		g.abyss[i] = math.NaN()
		if share[i] <= 0 {
			continue
		}
		t := &g.Tiles[i]
		to := t.Height + share[i]*((BasinDepth-depth[i])-t.Height)
		by := t.Height - to
		if by <= 0 {
			continue
		}
		g.abyss[i] = t.Height
		t.Height = to
		if g.strata != nil {
			g.strata[i].lift(-by)
		}
	}
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
		return g.Tiles[i].Height
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
			sum += was - g.Tiles[i].Height
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
	for k := 0; k < smoothing; k++ {
		u = g.spread(u)
	}
	return u
}
