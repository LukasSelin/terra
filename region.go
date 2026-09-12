package terra

import "github.com/LukasSelin/terra/geom"

// Where a laden walker can get to. Open water is shut to anybody carrying
// anything, so the ground is cut into regions by its rivers and its sea,
// and a laden walker on one side of a river with no bridge cannot reach the
// other however far it goes round. A search for a way between two such
// places finds none, and finds none only after opening every tile it could
// reach: on a coast that is most of the window, every time somebody
// carrying a sack wants something on the far bank.
//
// So the regions are labelled once, and a laden search between two labels
// is answered without being run. The labelling is exact - a walker who
// cannot enter deep water cannot leave the region it stands in except into
// the tile it is going to - so nothing a search would have found is lost,
// and the answer is the one the search would have given. It is redone when
// the water moves or a bridge goes up or comes down, and before anybody
// decides, so that deciding side by side reads it and never writes it.

// Region is the label of the laden-walkable ground p stands on, or 0 for
// deep water, which is no region at all.
func (g *Grid) Region(p geom.Pos) int32 {
	return g.Regions()[g.Index(p)]
}

// Regions is the label of every tile, worked out afresh if the water has
// moved since it was.
func (g *Grid) Regions() []int32 {
	if (len(g.regions) != len(g.Tiles) || g.regionsStale) && !g.islanded {
		g.label()
	}
	return g.regions
}

// label floods each region from the first of its tiles in row order, so the
// labels are the same on every run.
func (g *Grid) label() {
	n := len(g.Tiles)
	if len(g.regions) != n {
		g.regions = make([]int32, n)
	}
	clear(g.regions)
	var next int32
	stack := g.regionStack[:0]
	for i := 0; i < n; i++ {
		if g.regions[i] != 0 || g.Tiles[i].Deep() {
			continue
		}
		next++
		g.regions[i] = next
		stack = append(stack, int32(i))
		for len(stack) > 0 {
			j := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			p := g.PosOf(int(j))
			for _, off := range Dirs {
				q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
				if !g.In(q) {
					continue
				}
				k := g.Index(q)
				if g.regions[k] != 0 || g.Tiles[k].Deep() {
					continue
				}
				g.regions[k] = next
				stack = append(stack, int32(k))
			}
		}
	}
	g.regionStack = stack[:0]
	g.regionsStale = false
}

// wet notes that the water at i may have moved: a tile has become or ceased
// to be deep water, so the regions are to be labelled again.
func (g *Grid) wet() {
	g.regionsStale = true
	g.waters++
	// Water moving opens ways to a laden walker as well as shutting them:
	// a bridge, or a tile the river has risen under, from which the water
	// beside it may now be swum. The laden landmark tables cannot say by
	// how much, so they are not read until they are taken again.
	g.landmarks.ladenOK = false
}

// Waters is how many times the water has moved since the map was made: a
// bridge raised or fallen, a river cut or dried. Whether a laden walker can
// get from one place to another changes only when it does.
func (g *Grid) Waters() int { return g.waters }

// reachesLaden reports whether a laden walker standing on dry ground at
// from could ever reach stop: stop stands on the same ground, or stop is
// deep water with that ground on one of the eight sides of it, the one
// way a laden walker enters water. A search that this says no to would
// open everything it could reach and find nothing.
func (g *Grid) reachesLaden(from, stop geom.Pos) bool {
	regions := g.Regions()
	here := regions[g.Index(from)]
	if !g.Tiles[g.Index(stop)].Deep() {
		return regions[g.Index(stop)] == here
	}
	for _, off := range Dirs {
		q := geom.Pos{X: stop.X + off.X, Y: stop.Y + off.Y}
		if g.In(q) && regions[g.Index(q)] == here {
			return true
		}
	}
	return false
}

// WaterMoved tells the map that the water has moved somewhere, so that what
// is remembered about where a laden walker can get to is taken again. The
// map says it to itself wherever building or turning ground covers water or
// uncovers it; it is exported for whoever moves water another way.
func (g *Grid) WaterMoved() { g.wet() }

// RegionsStale reports whether the map is carrying a reading of where a
// laden walker can get to that the water has since made wrong.
func (g *Grid) RegionsStale() bool { return g.regionsStale }
