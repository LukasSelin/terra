package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
)

// Basins: the ground that goes down, and what fills it.
//
// A rift used to drop as far as its thinned crust floats, at once, and stay
// there: its crust was thinned and Airy's columns (isostasy.go) did the rest
// in the epoch it was stretched. A rift does not. Stretching the continent
// stretches the whole lithosphere under it, and the hot mantle comes up into
// the room: the light, hot column holds the rift up while the thinned crust
// lets it down, so a rift first drops by less than its crust alone would
// sink it, and then goes on sinking as the heat brought up under it is lost,
// for a hundred million years and more after the stretching has stopped.
// That is McKenzie's (1978) model of a sedimentary basin, and it is what the
// North Sea, the margins of the Atlantic and the basins inside the continents
// are read by (Allen and Allen, Basin Analysis, ch. 3).
//
// So each tile's crust keeps how far it has been stretched, β, and how much
// heat the stretching brought up under it is still to be lost, as the metres
// the column still stands higher for it (rifted). A rift thins the crust by
// what it drops it (tectonics), and β grows by what it was over what it is
// now: that is the crust's own reading of the stretching, read off #77's
// thickness. The heat it brings up is McKenzie's, the uplift the warm mantle
// gives a column stretched by β that is lost as the column cools,
//
//	E₀ β/π sin(π/β),   E₀ = 4 a ρm α T₁ / π² (ρm − ρw)
//
// for a lithosphere a thick whose base is T₁ hotter than its top, expanding
// by α a degree: 3.2 km for a column stretched without end, 2.0 for one
// stretched to twice its length. A stretching in steps adds what each step
// adds to it. And the heat is lost as the lithosphere's first mode of
// cooling is, by e^(−t/τ) with τ = a²/π²κ, 63 million years (cool): what the
// column still stands up by is its level over the crust's own (levelOf).
//
// The crust alone sinks a column stretched by β by wetRise of what it lost,
// 7.7 km (1 − 1/β) under the sea for thirty-five kilometres of it, where
// McKenzie has the stretched lithosphere's initial and final subsidence at
// 4.3 (1 − 1/β) and that plus E₀ β/π sin(π/β): the crust's reading less the
// heat comes to within 0.3 km of McKenzie's at both ends for β of 2 (1.8
// against 2.1 km at once, 3.9 against 4.2 when cold). What is left between
// them is the mantle's own thinning, which this does not keep.
//
// The first plates' margins are rifted continent too (layCrust thins them
// to the floor beside them). They were rifted when the floor beside them was
// made, so they are given the heat a rift that old still has: see
// firstMargins. They go on sinking through the history, as the earth's
// passive margins do, and give the sea room as they go.
//
// A range is a load on a plate, and the plate bends under it into a moat in
// front of it: a foreland basin, which the range sheds its waste into
// (Beaumont 1981; Jordan 1981). isostasy.go bends the plate under every load
// already. What it did not know is that at a collision the plate is broken:
// one of the two plates goes down under the other (sinks), and the range is
// piled onto the end of the plate going down, which bends under it, while the
// plate that stays up rides over the break and is not bent by it. So the
// foredeep lies in front of the range on the plate that went down, away from
// the overriding plate - the Ganges in front of the Himalaya, on India; the
// Molasse in front of the Alps, on Europe - and the plate that stays up
// floats its own columns over the reach of the bend (forelands).
//
// And what fills a basin is what arrives in it. The beds an epoch leaves were
// laid at a fixed rate wherever the ground lay low; they are what the weather
// laid on the land and what the shelves took off the rivers' mouths now,
// metre for metre (keepBook), sorted on the way: the sand settles out within
// a few tens of kilometres of a mouth and the mud is carried out over the
// shelf and down the slope (shelfReach); on the land the sand drops out next
// to the ground it came off and the plains further out are mud (sortedAt).
// A bed is a sandstone or a shale by its own grains (laidAs), and not by its
// rank among the world's.

// rifted is what tile i's crust keeps of being stretched: stretch is β, the
// length it has been stretched to over the length it had, nought read as
// one; warm is how far it still stands, in metres under water, over where it
// will once it has cooled; and at is the epoch it was last stretched in, one
// on, nought for the first plates' margins.
type rifted struct {
	stretch, warm float32
	at            uint8
}

// beta is r's stretching factor.
func (r rifted) beta() float64 { return math.Max(1, float64(r.stretch)) }

// McKenzie's (1978) lithosphere: a is lithosphere thick, its base mantleHot
// degrees over its top, the mantle expanding by thermalExpand a degree, and
// cooling with the time constant thermalTime, a²/π²κ for κ = 0.8 mm²/s.
const (
	lithosphere   = 125 * km
	mantleHot     = 1333.0
	thermalExpand = 3.28e-5
	thermalTime   = 62.8 * myr
)

// riftHeat is E₀, in metres: what a column stretched without end stands up
// by for the heat brought up under it, under water.
var riftHeat = 4 * lithosphere * mantleDensity * thermalExpand * mantleHot / (math.Pi * math.Pi * (mantleDensity - seaDensity))

// heatOf is what a column stretched by beta stands up by, under water, for
// the heat the stretching brought up, before any of it is lost.
func heatOf(beta float64) float64 {
	if beta <= 1 {
		return 0
	}
	return riftHeat * beta / math.Pi * math.Sin(math.Pi/beta)
}

// mcKenzieAtOnce is McKenzie's initial, fault-controlled subsidence of a
// continent of thick metres stretched by beta, under water: what the
// readings hold the rifts to.
func mcKenzieAtOnce(thick, beta float64) float64 {
	at := thermalExpand * mantleHot
	return lithosphere * ((mantleDensity-crustDensity)*thick/lithosphere*(1-at*thick/(2*lithosphere)) - at*mantleDensity/2) *
		(1 - 1/beta) / (mantleDensity*(1-at) - seaDensity)
}

// mcKenzie is McKenzie's whole subsidence of the same column years after it
// was stretched.
func mcKenzie(thick, beta, years float64) float64 {
	return mcKenzieAtOnce(thick, beta) + heatOf(beta)*(1-math.Exp(-years/thermalTime))
}

// stretch notes that tile i's crust has been thinned from was by a rift in
// epoch epoch: its β grows by was over what it is now, and so does the heat
// under it.
func (cr *crust) stretch(i int, was float64, epoch int) {
	now := float64(cr.thick[i])
	if !(now < was) || now <= 0 {
		return
	}
	r := &cr.rift[i]
	b0 := r.beta()
	b1 := b0 * was / now
	r.warm += float32(heatOf(b1) - heatOf(b0))
	r.stretch, r.at = float32(b1), uint8(epoch+1)
}

// cooling is the share of a rift's heat an epoch keeps.
var cooling = math.Exp(-epochYears / thermalTime)

// cool is an epoch of every rift losing its heat.
func (cr *crust) cool() {
	for i := range cr.rift {
		cr.rift[i].warm *= float32(cooling)
	}
}

// firstMargins gives the continent the first plates' margins thinned (see
// layCrust) the stretching that thinned it, from thirty-five kilometres, and
// the heat a rift as old as the floor beside it still has. g's crust is laid
// and not yet floated.
func (cr *crust) firstMargins(g *Grid) {
	any := false
	for i := range g.Tiles {
		if cr.ocean[i] {
			any = true
			break
		}
	}
	if !any {
		return
	}
	_, near := g.nearestTo(func(i int) bool { return cr.ocean[i] }, true)
	for i := range g.Tiles {
		t := float64(cr.thick[i])
		if cr.ocean[i] || t >= continentCrust || t <= 0 || near[i] < 0 {
			continue
		}
		b := continentCrust / t
		age := float64(cr.aged[near[i]])
		cr.rift[i] = rifted{stretch: float32(b), warm: float32(heatOf(b) * math.Exp(-age/thermalTime))}
	}
}

// sourceWear is how fast ground has to be worn, in metres a year, to be where
// a floodplain's load came from: faster than the median of the world's
// basins, 0.054 mm/yr (Portenga and Bierman 2011) - the uplands and the
// ranges, and not the plains that are worn a little everywhere.
const sourceWear = 0.054 * mm / yr

// laidBy writes into into what has been laid on each tile of g since its
// heights were from, in metres, and keeps the heights as they are now in
// step: the weather's deposits since the epoch's heights were taken, and then
// the shelves' since the weather's.
func (cr *crust) laidBy(g *Grid, from []float64, into []float32) {
	for i, h := range g.Height {
		into[i] = float32(math.Max(0, h-from[i]))
	}
	copy(cr.step, g.Height)
}

// foreReach is how far either side of a collision the plates are read as
// broken: across the belt the collision raises (beltReach) and on as far as
// a plate's bend under the belt's edge reaches, four flexural parameters.
var foreReach = beltReach + 4*flexuralParameter

// collide notes that tile i's crust meets tile j's head on at a collision this
// epoch, and which of the two goes under: the one sinks says goes under.
func (g *Grid) collide(cr *crust, i, j int) {
	if sinks(g.Tiles[j].Plate, cr.ocean[j], cr.born[j], g.Tiles[i].Plate, cr.ocean[i], cr.born[i]) {
		cr.collided = append(cr.collided, -int32(i)-1) // i stays up
		return
	}
	cr.collided = append(cr.collided, int32(i))
}

// forelands reads, for every tile within foreReach of a collision on its own
// plate, how far it is from it and on which side: fore is one more than the
// distance in tiles, positive on the plate that goes down and negative on the
// plate that stays up, and nought anywhere else. It is the collisions noted
// this epoch, and isostasy floats the tiles of a plate that stays up on their
// own columns.
func (g *Grid) forelands(cr *crust) {
	clear(cr.fore)
	cr.broken = len(cr.collided) > 0
	if !cr.broken {
		return
	}
	reach := foreReach / g.span()
	queue := g.seamQueue[:0]
	for _, c := range cr.collided {
		i, side := c, float32(1)
		if c < 0 {
			i, side = -c-1, -1
		}
		if cr.fore[i] == 0 {
			cr.fore[i] = side
			queue = append(queue, i)
		}
	}
	cr.collided = cr.collided[:0]
	for k := 0; k < len(queue); k++ {
		i := int(queue[k])
		here := cr.fore[i]
		side := float32(math.Copysign(1, float64(here)))
		away := math.Abs(float64(here)) - 1
		p := geom.Pos{X: i % g.W, Y: i / g.W}
		for _, off := range Dirs {
			q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
			if g.Wrap {
				q = g.Norm(q)
			}
			if !g.In(q) {
				continue
			}
			j := g.Index(q)
			if g.Tiles[j].Plate != g.Tiles[i].Plate {
				continue
			}
			step := away + math.Hypot(float64(off.X), float64(off.Y))
			if step >= reach {
				continue
			}
			if f := cr.fore[j]; f != 0 && math.Abs(float64(f))-1 <= step {
				continue
			}
			cr.fore[j] = side * float32(step+1)
			queue = append(queue, int32(j))
		}
	}
	g.seamQueue = queue[:0]
}
