package terra

import (
	"github.com/LukasSelin/terra/geom"
)

// The map in pieces. A chunk is a square of the ground with a few counts
// kept beside it - what stands on it, what grows on it, whether anybody
// has ever walked it - so that a question about the whole map is a sum
// over the chunks rather than a walk over every tile, and so that the
// passes over the ground that every day makes can pass over the ground
// where something is happening and leave the rest. The tiles themselves
// stay in one row-major slice: a chunk is a reading of the ground, not
// where the ground is kept.
//
// The counts are kept by the writes that change them - Build, Turn, Claim
// - rather than taken afresh, because they are read within the tick they
// change in: how many granaries stand is asked after the day's building
// and before the market's food spoils, and a count a tick behind would
// spoil a different amount.

// ChunkSide is how many tiles a chunk is across and down. Every distance
// anything on the map is looked for over is shorter than this, so the
// three chunks by three around a position hold everything within reach.
const ChunkSide = 64

// Chunk is one square of the map and what is known about it.
type Chunk struct {
	X0, Y0, W, H int
	// Built and Owned are how many tiles here have something standing on
	// them, and how many are somebody's. Ground with neither is ground
	// nobody keeps and nothing falls down on. Places is how many of the
	// built ones make the ground a place rather than a building; see
	// MarkDef.Settles.
	Built, Owned, Places int
	// Marks is how many tiles here carry each mark. The land keeps the
	// tally and does not read it; whoever put the marks there reads it by
	// the marks it chose. See Marked.
	Marks [MarkCount]int32
	// Trodden is whether anybody has crossed this chunk since it was last
	// woken, and Trod the day it was last known to have been. Wear keeps
	// ground awake for a while after the last crossing - see active.go -
	// and ground nobody has walked has no wear to fade and no case for a
	// road.
	Trodden bool
	Trod    int
	// Grown is the growing weather the world had had when this chunk was
	// last passed over, and Weathered the day; see active.go.
	Grown     float64
	Weathered int
	// Height is the mean height of the ground here, in metres. It is what
	// the growing weather of a sleeping chunk is read at - the weather goes
	// by latitude and by height, and a chunk is the finest the sleeping
	// ground is reckoned by, so a chunk that is mostly mountain grows like
	// a mountain. Taken again whenever the ground is counted, because the
	// weather moves the ground. See World.Rates.
	Height float64
}

// layChunks divides the grid into chunks. Chunks along the east and south
// edges are whatever is left over.
func (g *Grid) layChunks() {
	g.CW, g.CH = (g.W+ChunkSide-1)/ChunkSide, (g.H+ChunkSide-1)/ChunkSide
	g.Chunks = make([]Chunk, g.CW*g.CH)
	for cy := 0; cy < g.CH; cy++ {
		for cx := 0; cx < g.CW; cx++ {
			c := &g.Chunks[cy*g.CW+cx]
			c.X0, c.Y0 = cx*ChunkSide, cy*ChunkSide
			c.W, c.H = min(ChunkSide, g.W-c.X0), min(ChunkSide, g.H-c.Y0)
			c.Trod = -1
		}
	}
}

// ChunkOf is the chunk the tile kept at i is in.
func (g *Grid) ChunkOf(i int) int {
	return (i/g.W/ChunkSide)*g.CW + (i%g.W)/ChunkSide
}

// ChunkAt is the chunk p is in. The caller must check In first.
func (g *Grid) ChunkAt(p geom.Pos) *Chunk {
	return &g.Chunks[g.ChunkOf(g.Index(p))]
}

// count adds d to the chunk's tally of whatever t has on it.
func (c *Chunk) count(t *Tile, d int) {
	c.Marks[t.Mark] += int32(d)
	if t.Mark != None {
		c.Built += d
		if markSettles[t.Mark] {
			c.Places += d
		}
	}
	if t.Owner != 0 {
		c.Owned += d
	}
}

// Build puts s on p, or takes down what is there when s is None.
func (g *Grid) Build(p geom.Pos, s Mark) {
	i := g.Index(p)
	t, c := &g.Tiles[i], &g.Chunks[g.ChunkOf(i)]
	c.count(t, -1)
	lent, deep, was := t.lends(), t.Deep(), tableCost(t)
	t.Mark = s
	g.Kinds[i] = kindOf(t)
	c.count(t, 1)
	g.relend(i, lent)
	g.landmarks.cheapened(g.ChunkOf(i), was, tableCost(t))
	if t.Deep() != deep {
		g.wet()
	}
}

// Turn makes the ground at p into terrain tr. What stood or grew on it is
// the caller's to settle.
func (g *Grid) Turn(p geom.Pos, tr Terrain) {
	i := g.Index(p)
	t, c := &g.Tiles[i], &g.Chunks[g.ChunkOf(i)]
	c.count(t, -1)
	// Turning ground is the one thing that changes what kind a tile is,
	// so it is the one place the patches are kept. See patch.go.
	g.mark(i, t, -1)
	deep, was := t.Deep(), tableCost(t)
	t.Terrain = tr
	g.Kinds[i] = kindOf(t)
	if tr != Field {
		t.Fenced = false // a hedge stands round a field and nothing else
	}
	c.count(t, 1)
	g.mark(i, t, 1)
	g.landmarks.cheapened(g.ChunkOf(i), was, tableCost(t))
	if t.Deep() != deep {
		g.wet()
	}
}

// Claim makes p somebody's, or nobody's when id is zero. Who the holder is
// is not the land's business; see claim.go.
func (g *Grid) Claim(p geom.Pos, id Holder) {
	i := g.Index(p)
	t, c := &g.Tiles[i], &g.Chunks[g.ChunkOf(i)]
	c.count(t, -1)
	lent := t.lends()
	t.Owner = id
	c.count(t, 1)
	g.relend(i, lent)
}

// relend settles the neighbours' lender counts after tile i has changed,
// given whether it lent before.
func (g *Grid) relend(i int, lent bool) {
	if now := g.Tiles[i].lends(); now != lent {
		if now {
			g.lend(i, 1)
		} else {
			g.lend(i, -1)
		}
	}
}

// Recount takes every chunk's counts afresh from the ground, for after the
// ground has been made or remade wholesale.
func (g *Grid) Recount() {
	for i := range g.Chunks {
		c := &g.Chunks[i]
		c.Built, c.Owned, c.Places = 0, 0, 0
		clear(c.Marks[:])
		c.Height = 0
	}
	if len(g.lenders) != len(g.Tiles) {
		g.lenders = make([]uint8, len(g.Tiles))
	}
	clear(g.lenders)
	g.repatch()
	g.rekind()
	// Whatever moved enough of the ground to want a recount may have
	// moved it under the landmarks.
	g.landmarks.moved()
	g.wet()
	for i := range g.Tiles {
		g.Chunks[g.ChunkOf(i)].Height += g.Tiles[i].Height
		g.Chunks[g.ChunkOf(i)].count(&g.Tiles[i], 1)
		if g.Tiles[i].lends() {
			g.lend(i, 1)
		}
	}
	for i := range g.Chunks {
		if c := &g.Chunks[i]; c.W*c.H > 0 {
			c.Height /= float64(c.W * c.H)
		}
	}
}

// rekind takes every tile's kind afresh from the tile; see Layers.Kinds.
func (g *Grid) rekind() {
	if len(g.Kinds) != len(g.Tiles) {
		g.Kinds = make([]int64, len(g.Tiles))
	}
	for i := range g.Tiles {
		g.Kinds[i] = kindOf(&g.Tiles[i])
	}
}

// Rekind takes every tile's kind afresh from the tile. The map keeps it
// itself wherever the ground is turned or built on - see Build and Turn -
// and takes it afresh whenever the ground is remade wholesale, in Recount;
// this is for a test that lays tiles by hand and then wants the day to
// pass over them.
func (g *Grid) Rekind() { g.rekind(); g.landmarks.moved() }

// Marked is how many tiles carry mark m, summed over the chunks rather than
// walked over the tiles.
func (g *Grid) Marked(m Mark) int {
	return g.sum(func(c *Chunk) int { return int(c.Marks[m]) })
}

// Fields is how many tiles are under the plough.
func (g *Grid) Fields() int { return g.Kind(Field) }

// Forest is how many tiles are wooded.
func (g *Grid) Forest() int { return g.Kind(Forest) }

// Kind is how many tiles of this ground the map has, summed over the
// patches rather than walked over the tiles.
func (g *Grid) Kind(t Terrain) int {
	n := 0
	for i := range g.patches {
		n += int(g.patches[i][t])
	}
	return n
}

func (g *Grid) sum(of func(*Chunk) int) int {
	n := 0
	for i := range g.Chunks {
		n += of(&g.Chunks[i])
	}
	return n
}

// lends reports whether a tile lends its wear to the tiles beside it: it
// is neither open ground nor a road, so the errands in and out of it are
// walked on the ground around it. See Draw.
func (t *Tile) lends() bool { return !t.Pavable() && !markWay[t.Mark] }

// lend adds d to the lender count of each of i's eight neighbours.
func (g *Grid) lend(i int, d int8) {
	p := g.PosOf(i)
	for _, off := range Dirs {
		q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
		if g.In(q) {
			g.lenders[g.Index(q)] = uint8(int8(g.lenders[g.Index(q)]) + d)
		}
	}
}

// Lenders is how many of p's neighbours lend it their wear.
func (g *Grid) Lenders(i int) int { return int(g.lenders[i]) }

// AnyWithin reports whether the map holds any tile of one of these kinds
// of ground within radius of p. It is answered from the counts of the
// patches the radius reaches into, so it is a handful of comparisons
// whatever the radius is.
//
// It is a question asked to be told no. A search for ground of some kind
// that walks out to its full radius and finds nothing is the most
// expensive thing an agent does and the commonest: over half the searches
// made on the globe find nothing, and each of those proves it the long
// way, tile by tile over every ring. This proves it from the counts
// instead, and the search is only walked when there is something to find.
//
// So a false here has to mean there is truly nothing, while a true need
// only mean there might be: the answer is read off whole patches, and a
// patch the radius clips the corner of counts as reached. That is why the
// search still runs when this says yes, and why nothing it decides can
// change what a search returns, and it is why the patch is as small as it
// is - the ground it consults is the ground the search would cover,
// rounded up to the patch, and every tile of the difference is a search
// walked for nothing.
func (g *Grid) AnyWithin(p geom.Pos, radius int, kinds KindSet) bool {
	if kinds == 0 {
		return false
	}
	p = g.Norm(p)
	y0, y1 := max(0, p.Y-radius), min(g.H-1, p.Y+radius)
	x0, x1 := p.X-radius, p.X+radius
	if !g.Wrap {
		x0, x1 = max(0, x0), min(g.W-1, x1)
	}
	for py := y0 / PatchSide; py <= y1/PatchSide; py++ {
		for px := FloorDiv(x0, PatchSide); px <= FloorDiv(x1, PatchSide); px++ {
			x := px
			if g.Wrap {
				x = ((x % g.PW) + g.PW) % g.PW
			} else if x < 0 || x >= g.PW {
				continue
			}
			held := &g.patches[py*g.PW+x]
			for t := Terrain(0); t < TerrainCount; t++ {
				if kinds.Has(t) && held[t] > 0 {
					return true
				}
			}
		}
	}
	return false
}
