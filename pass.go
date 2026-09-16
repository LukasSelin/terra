package terra

import (
	"github.com/LukasSelin/terra/tile"

	"github.com/LukasSelin/terra/internal/kernel"
)

// The day's pass over the ground, as flat loops over the layers.
//
// What the weather does to a tile in a day is stated tile by tile in
// grow.go - Ripen, then Replenish - and that is the definition. This is the
// same arithmetic done to a run of tiles at once, a layer at a time, so
// that the pass streams the layers rather than picking each tile's numbers
// out of the map: what kind of ground each tile is - what stands on it and
// what it is made of, as one word - is kept beside the map as a layer of
// its own, and every loop here is a loop over one slice asking that word
// whether the tile is the kind it is looking for. Nothing in the pass
// reads a tile. It is the shape the pass has to be in for the arithmetic
// to be done several tiles at a time, which is what the kernels in
// internal/kernel/kernel_simd_amd64.go do with it where the build and the processor allow;
// internal/kernel/kernel.go is the same loops one tile at a time, and the helpers here are
// the fillings, which are one tile at a time on every processor.
//
// Nothing here changes a result to the last bit. Each tile is given the
// same operations on the same operands in the same order as Ripen and
// Replenish give it, and no tile's numbers are read by another tile's, so
// the order the tiles are taken in is not a fact about anything. The rate
// of a filling times the day's weather is worked out once for the run
// rather than once per tile; it is the same product. What that does rule
// out is the compiler fusing a multiply into the add that follows it, which
// Go allows and which the amd64 compiler does not do at the default level
// the golden numbers are taken at; a machine that fused it before would not
// fuse it now, and would be off by a last bit from itself. See
// core/system/golden_test.go.

// A kind is what stands on a tile and what it is made of, as one word:
// the structure in the high byte and the terrain in the low, so that the
// terrain alone can be read back off it with a mask. It is a word rather
// than a byte because the pass compares it lane for lane against the
// numbers it gates, and the numbers are eight bytes wide. The map keeps
// one for every tile in Layers.Kinds; Build and Turn write it, and Recount
// takes it afresh.
const (
	kindShift = 8
	kindMask  = 1<<kindShift - 1
	// kindSpan is one more than the largest kind, for the tables by kind.
	kindSpan = int(MarkCount-1)<<kindShift | int(TerrainCount-1) + 1
)

// kindOf is the kind of a tile.
func kindOf(t *Tile) int64 {
	return int64(t.Mark)<<kindShift | int64(t.Terrain)
}

// ages is which kinds of ground carry something growing, and so get older
// with the weather: the alive table read by kind. isWater, isField and
// isGrass are which kinds are water, field and open ground, whatever stands
// on them, which is the question Replenish asks. aging is the kinds that
// age listed out, for a pass that asks the question of several tiles at
// once.
var ages, isWater, isField, isGrass [kindSpan]bool
var aging []int64

// stocking is one process that fills a stock on one kind of ground: which
// kind, how long it takes in full, how much of the stock a growing day puts
// on, and the layer the stock is kept in. It is a Growth with its kind
// beside it, so that the pass works nothing out.
type stocking struct {
	kind       int64
	full, rate float64
	stock      func(*Grid) []float64
}

// stocked is every growth that fills a stock, by kind and then in the order
// the growth table has them: the order Ripen fills them in.
var stocked []stocking

// readGrowth takes all of the pass's tables afresh off the growth table. It
// is called whenever that table is set, rather than worked out once at
// startup, because the table is filled by whatever game is being played and
// there is no ordering of package variables that can be relied on to have
// happened first. See SetGrowth.
// Which kinds are water, field and open ground is a fact about the ground
// and about nothing a game says, so it is settled once, here, and not in
// readGrowth. It was in readGrowth, and on a land nobody had told anything
// to grow the day's pass then thought no tile anywhere was water: the fish
// came back tile by tile and not in the flat pass over the row, and the two
// statements of the same day parted company. A game always registers
// something, which is why that hid.
var kindsRead = func() bool {
	for s := 0; s < MarkCount; s++ {
		for t := 0; t < int(TerrainCount); t++ {
			k := kindOf(&Tile{Mark: Mark(s), Terrain: Terrain(t)})
			isWater[k] = Terrain(t) == Water
			isField[k] = Terrain(t) == Field
			isGrass[k] = Terrain(t) == Grass
		}
	}
	return true
}()

func readGrowth() {
	aging, stocked = aging[:0], stocked[:0]
	for s := 0; s < MarkCount; s++ {
		for t := 0; t < int(TerrainCount); t++ {
			k := kindOf(&Tile{Mark: Mark(s), Terrain: Terrain(t)})
			ages[k] = tile.Alive(Mark(s), Terrain(t))
			if ages[k] {
				aging = append(aging, k)
			}
			for _, f := range growth[s][t] {
				if f.Rate == 0 || f.Stock == nil {
					continue
				}
				stocked = append(stocked, stocking{
					kind:  k,
					full:  f.Full,
					rate:  f.Rate,
					stock: f.Stock,
				})
			}
		}
	}
}

// Fade is the share of its marking a tile keeps from one day to the next.
// It is the ground forgetting, which is the ground's own business and goes
// on whether or not anybody is walking: a way nobody has taken for a season
// is grass again. What puts the marking there in the first place is not the
// land's - see Wear and Haul, where a game says how much of a mark a walker
// of its own makes.
const Fade = 0.995

// FadeWear fades the wear on the tiles [lo, hi) by the given share: a day's
// Fade, or a sleep's worth of days at once. Wear is never below nothing -
// what marks the ground adds to it and what fades it multiplies it by a
// share of one - so ground with none on it is left with none, and there is
// nothing to ask before multiplying.
func (g *Grid) FadeWear(lo, hi int, by float64) {
	kernel.Fade(g.Traffic[lo:hi], by)
}

// Grow gives the tiles [lo, hi) k of growing weather: what grows on them
// gets that much older and fills toward what its age accounts for, the
// water gets its fish back and worn fields rest. It is Ripen and then
// Replenish, tile by tile, done as loops over the layers; see the remarks
// at the top of the file.
func (g *Grid) Grow(lo, hi int, k float64) {
	ks := g.Kinds[lo:hi]
	// A stand ages by the weather it gets, not by the calendar; see Ripen.
	// The age is put on before anything reads it.
	age := g.Age[lo:hi]
	kernel.Grow(age, ks, k, ages[:], aging)
	// Whatever is coming on fills a little further, bounded by the age it
	// has had; see fillStand. Each filling is a pass of its own over the run,
	// in the order the growing table has them.
	for _, e := range stocked {
		fill(e.stock(g)[lo:hi], age, ks, e.kind, e.full, e.rate, k)
	}
	// And what comes back that is not a stand coming on; see Replenish.
	shoal(g.Fish[lo:hi], ks, regrow(FishRegrowth, k))
	rest(g.Fertility[lo:hi], g.Rich[lo:hi], ks, regrow(Fallow, k))
	meadow(g.Sward[lo:hi], ks, regrow(SwardRegrowth, k))
}

// The fillings and the replenishing, one tile at a time on every processor.
// Each is a closed form - a cube root, a logarithm and an exponential for a
// stand, a division for the rest - and none of those is an instruction the
// vectors have; see stand and logistic. Each is the statement in grow.go
// with the tile's kind read off ks rather than off the tile. What is done
// several tiles at once is the ageing and the fading, which touch every
// tile: fade and grow, in package kernel.

// fill fills the stock s on every tile of the given kind over k of
// growing weather, up to what its age over full accounts for; see fillStand.
func fill(s, age []float64, ks []int64, kind int64, full, rate, k float64) {
	for j, kk := range ks {
		if kk == kind {
			s[j] = fillStand(s[j], age[j], k, full, rate)
		}
	}
}

// shoal puts the fish back in the water, up to full, by the run's
// logistic factor; see logistic.
func shoal(fish []float64, ks []int64, fall float64) {
	for j, kk := range ks {
		if isWater[kk] {
			fish[j] = logistic(fish[j], 1, fall)
		}
	}
}

// rest rests the fields, up to what the ground can hold.
func rest(fert, rich []float64, ks []int64, fall float64) {
	for j, kk := range ks {
		if isField[kk] {
			fert[j] = logistic(fert[j], rich[j], fall)
		}
	}
}

// meadow puts the grass back on open ground, up to full.
func meadow(sward []float64, ks []int64, fall float64) {
	for j, kk := range ks {
		if isGrass[kk] {
			sward[j] = logistic(sward[j], 1, fall)
		}
	}
}
