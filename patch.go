package terra

import "github.com/LukasSelin/terra/geom"

// What kind of ground lies where, at a size worth asking about.
//
// A patch is a small square of the map with one count per kind of ground
// on it. It exists for one question - is there any wood, water, rock
// within so far of here - and that question exists because the commonest
// and dearest thing an agent does is look for ground that is not there.
// Such a search walks every ring out to its radius before it can say so.
// Asked of the patches first, it does not have to be walked at all.
//
// The size is the whole of the tuning. A patch is consulted whole, so the
// ground it answers for is the ground the search would cover rounded up to
// the patch, and every tile of that difference is a search walked for
// nothing. Chunks were tried first, since they were already counting: at
// sixty-four tiles against a radius of forty they answered for five and a
// half times the ground the search covered, said yes wherever a settlement
// had any wood at all in the district, and turned away only four searches
// in a hundred. At sixteen the overshoot is under half again.
//
// It is not a second copy of what the chunks know. The chunks stopped
// counting kinds when this began; they count what is built and what is
// owned, which is what the passes over the ground ask them, and how much
// ground of a kind lies where is asked here and nowhere else. (What kind
// one tile is, the day's pass reads off Layers.Kinds, which is the tile's
// own kind kept beside it and not a count of anything.)

// PatchSide is how many tiles a patch is across and down.
const PatchSide = 16

// patch is how many tiles of each kind of ground one patch holds. It is
// int32 because a patch cannot hold more tiles than it has.
type patch [TerrainCount]int32

// layPatches divides the grid into patches, all counts empty. Recount
// fills them.
func (g *Grid) layPatches() {
	g.PW, g.PH = (g.W+PatchSide-1)/PatchSide, (g.H+PatchSide-1)/PatchSide
	g.patches = make([]patch, g.PW*g.PH)
}

// PatchOf is the patch the tile kept at i is in.
func (g *Grid) PatchOf(i int) int {
	return (i/g.W/PatchSide)*g.PW + (i%g.W)/PatchSide
}

// mark adds d to the patch's tally of the kind of ground at i. It is
// called on either side of every change to what a tile is, as the chunk's
// own counts are: see Grid.Turn.
func (g *Grid) mark(i int, t *Tile, d int32) {
	if t.Terrain < TerrainCount {
		g.patches[g.PatchOf(i)][t.Terrain] += d
	}
}

// repatch takes every patch's counts afresh from the ground.
func (g *Grid) repatch() {
	if len(g.patches) != g.PW*g.PH {
		g.layPatches()
	}
	for i := range g.patches {
		g.patches[i] = patch{}
	}
	for i := range g.Tiles {
		g.mark(i, &g.Tiles[i], 1)
	}
}

// eachInPatch visits every tile of patch p, row by row.
func (g *Grid) eachInPatch(p int, f func(i int, t *Tile)) {
	x0, y0 := (p%g.PW)*PatchSide, (p/g.PW)*PatchSide
	x1, y1 := min(g.W, x0+PatchSide), min(g.H, y0+PatchSide)
	for y := y0; y < y1; y++ {
		row := y * g.W
		for i := row + x0; i < row+x1; i++ {
			f(i, &g.Tiles[i])
		}
	}
}

// chunkOfPatch is the chunk patch p lies in: a chunk is a whole number of
// patches across and down, so a patch is never in two.
func (g *Grid) chunkOfPatch(p int) int {
	const per = ChunkSide / PatchSide
	px, py := p%g.PW, p/g.PW
	return (py/per)*g.CW + px/per
}

// patchAt is the patch p is in. p must be on the map and normalised.
func (g *Grid) patchAt(p geom.Pos) int {
	return (p.Y/PatchSide)*g.PW + p.X/PatchSide
}

// patchHolds reports whether patch i holds any tile of these kinds.
func (g *Grid) patchHolds(i int, kinds KindSet) bool {
	held := &g.patches[i]
	for t := Terrain(0); t < TerrainCount; t++ {
		if kinds.Has(t) && held[t] > 0 {
			return true
		}
	}
	return false
}

// The patches, asked about from outside.
//
// A patch is the land's own banding of the map and its counts are the
// land's, but what a game does with them is the game's: the day's fences
// walk the patches that hold any worked ground and leave the rest, and
// that walk is the settlement's. So the questions are answered here and
// the walking is done there.

// Patches is how many patches the map is cut into.
func (g *Grid) Patches() int { return len(g.patches) }

// PatchHolds is how many tiles of kind t lie in patch pi. It is the count
// the patch keeps rather than a walk over its tiles, which is the whole
// point of a patch.
func (g *Grid) PatchHolds(pi int, t Terrain) int { return int(g.patches[pi][t]) }

// PatchAwake reports whether the day has passed over the ground patch pi
// lies on.
func (g *Grid) PatchAwake(pi int) bool { return g.Awake(g.chunkOfPatch(pi)) }

// EachInPatch visits every tile of patch pi, row by row and along each row.
func (g *Grid) EachInPatch(pi int, f func(i int, t *Tile)) { g.eachInPatch(pi, f) }
