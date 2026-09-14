package terra

import (
	"cmp"
	"github.com/LukasSelin/terra/geom"
	"math"
	"slices"
	"sort"
)

// The shape of the land, and the water that shape decides.
//
// Everything in this file happens once, before anybody is born, but it is
// written to be run again: heights are a field, drainage is derived from
// heights, and the rivers are derived from drainage. Nothing about the water
// is drawn by hand, so if the ground ever moves - silt, erosion, a dam - the
// rivers follow it by being recomputed rather than by being redrawn.
//
// The order is the one a landscape actually obeys. Raise the ground; fill the
// hollows, each until it spills or until the air takes off it what runs into
// it, which is a lake or a salt lake; send every tile's water down the ground
// and every lake's out of its outlet; add up what passes through each tile;
// and call the tiles that carry enough of it a river. Fertility, woods and
// outcrops then read off the finished land instead of being scattered over it.

// Relief is the fall of the lowland in metres, from the lowest ground a map
// can have to the shoulders of the valley: sixty metres over eighty tiles is
// a river valley with sides to it, enough that walking uphill is felt and
// that water knows where to go. It is what the whole map used to be, and the
// ground a settlement lives on is still made to exactly this figure.
const Relief = 60.0

// Upland is how far the high country stands above the valley it stands in,
// and uplandShare is how much of a map it covers. Two hundred and sixty
// metres is a wall rather than a slope: ground a route goes round because
// going over it costs twenty tiles of climbing, and that is the whole
// difference between a map with somewhere on it and a map without.
//
// It is quoted here and scaled to the ground the high country is spread over
// rather than to the width of the map - see Grid.UplandRise - because what
// makes a mountain a mountain is the slope and not the number on top of it,
// and a rise spread over more ground is a gentler thing. Scaled to the map, as
// it was, a globe sixteen chunks round raised its high country three thousand
// three hundred metres: ground a tenth of which fell more than a metre for
// every metre crossed, which is not a mountainside but a cliff, and all of it
// in four or five domes with a dead flat plain between them. Scaled to the
// range, a map with room for a whole range gets a whole range, and a map with
// room for one mountainside gets the mountainside it always had.
//
// Relief is deliberately not scaled with it. The lowland is where a settlement
// lives, and what makes it liveable is measured in metres and not in tiles:
// FloodDepth says the valley floor is the ground within fourteen metres of its
// river, and the soil reads off that. Stretching the lowland to match a wider
// map would put most of it above the flood and take its soil down to the floor
// of 0.15, which is the thing that went wrong when the valley and the
// mountains were one field scaled together.
//
// The share is what keeps a map habitable, and it is a share rather than a
// height for the reason everything else here is: how much of a map comes out
// above a fixed line depends entirely on the shape of that map, and what is
// wanted is that every map has both a lowland to live in and a skyline behind
// it. See forestShare, cut the same way and for the same reason.
const (
	Upland      = 260.0
	uplandShare = 0.22
	// uplandMass is how much of the rise is the bulk of the high country and
	// how much is the ridges standing on it.
	uplandMass = 0.45
	// uplandSpan is the map width the default valley is founded on unless
	// somebody says otherwise, and the width the rest of this was measured at.
	uplandSpan = DefaultWidth
)

// rangeSpan is how far apart the ridges of a range stand, and rangeFoot is how
// much ground the range itself covers: a kilometre between the ridges and
// three between the feet of the thing, so that a range is several summits with
// saddles between them and not one hill.
//
// They are lengths on the ground and not shares of the map, and that is the
// whole of what was wrong. Every octave here used to start at half the map's
// own width - on the default eighty tiles that is rangeSpan exactly, and on a
// globe it is five hundred. The coarsest octave of a ridged field carries most
// of its height, so a globe drew every mountain it had off a lattice two
// corners round and three deep: one ridge the size of a hemisphere. The mask
// that says where the high country stands went the same way, at the square
// root of the map and the quoted span, and set that ridge in a round
// two-hundred-tile blank. What came out was the complaint - one blob, too
// round, too high, over far too much ground, with nothing in it.
//
// Held at the size of a range, a wider map gets more ranges rather than one
// range drawn wider, and the grain inside each of them is the grain the
// valley's own high ground has. The foot is three ridges across because a
// massif drawn at the span of its own ridges holds one crest and is a cone
// again.
const (
	rangeSpan = uplandSpan / 2
	rangeFoot = 3 * rangeSpan
)

// UplandLattice is how far apart the corners of the mask that says where the
// high country stands are: the foot of a range, or half the map where the map
// is smaller than that, which the default valley is. A valley is one
// mountainside seen close to; it has no room for a whole range and never had.
//
// A corner's width is the mountainside, because the whole of the high
// country's rise happens across one of them - which is why this is held to the
// size of a range and not to the size of the world. Scaled with the world, as
// it was, a globe spread the same rise over two hundred tiles and got a slope
// of four in a hundred: a swell so broad that standing on it you would not
// know.
func (g *Grid) UplandLattice() float64 {
	return math.Min(float64(g.Span())/2, rangeFoot)
}

// Span is how many tiles across the map is at its widest. It is what the lie
// of the land is measured in - the octaves of the broad swell start at half of
// it - and it is the ceiling on everything else, because no feature can be
// wider than the map it is drawn on. The mountains themselves are measured in
// their own lengths instead: see rangeSpan.
func (g *Grid) Span() int { return max(g.W, g.H) }

// UplandRise is how far this map's high country stands above its valley: the
// quoted rise, in proportion to how much ground that country is spread over.
// A range with three times the footing stands three times as tall and its
// flanks come out at the same slope either way, which is the point of it - a
// mountain is known by how steeply it goes up and not by the number on top.
// On a map too small to hold a whole range it is Upland exactly.
func (g *Grid) UplandRise() float64 { return Upland * g.UplandLattice() / (uplandSpan / 2) }

// Skyline is the top of the map: the valley's own relief plus the high
// country standing on it.
func (g *Grid) Skyline() float64 { return Relief + g.UplandRise() }

// rockShare and rockSteep are how much of a map is bare stone, and how steep
// ground has to be, in its own terms, to be a candidate for it.
const (
	rockShare   = 0.015
	forestShare = 0.13
)

// channelSteep is the fall past which more fall stops helping the water cut a
// channel and starts to hinder it. It is there for the flanks of a range.
//
// The slope-area law is a law about soil-mantled hillsides. Past about two in
// three the ground does not gather its water into a channel: it sheds it, and
// its soil with it, straight down the face, and a slope that steep is a scree
// and not a stream bed. Read without a limit, A·S made every tile of a
// history's mountain wall - falls of three and six in one, against the six in
// ten of the valley's own upland streams - out-score a river on the plain with
// a hundred times its water, so every line of tiles down every flank became a
// channel of its own. Laid side by side a tile apart and cutting as channels
// cut, they combed each range into a row of parallel trenches.
//
// So above channelSteep the fall counts against the reading as fast as it
// counted for it below, and a head needs some ground above it however steep it
// is. The valley's upland streams run below the limit and drain more than the
// floor, and are left as they were: over three seeds the share of the valley's
// river in its high fifth goes from 20.7, 24.3 and 13.8 per cent to 14.6, 22.9
// and 14.2. A half globe's high fifth, after sixty ages, goes from 13.4, 11.5
// and 11.9 per cent river to 8.4, 4.8 and 8.5.
const channelSteep = 0.7

// bankRise is how far above its own channel a great river's flood reaches, in
// metres. Nothing: a river spreads onto the ground beside it that is no higher
// than the water, and no further.
//
// It was a metre, and the line above it said "no higher" - the comment and the
// code had disagreed since it was written, and the code was the generous one.
// A metre is a great deal of flood plain when the ground is flat, and it went
// unnoticed for as long as the reading that picks a great river was wrong,
// because a river picked by how hard it was cutting is a rill near a ridge and
// a rill near a ridge has no flat ground beside it to give away. Corrected to
// read off the flow, the rule began firing where it should - on flood plains,
// which is where the markets are - and took an eighth of a settlement's
// building ground with it.
//
// Counted within ten tiles of a market over eight globes, by how far the flood
// is let rise:
//
//	rise    water   fish   fertility   buildable   river, share of map
//	1.00    149.5   127.3    232.2       240.4           7.18%
//	0.50    146.0   124.6    237.8       244.1           7.05%
//	0.25    142.5   121.2    242.1       247.9           6.85%
//	0.00    130.1   111.2    254.2       256.4           5.68%
//
// Nothing gives back half of the ground the correction cost - 240 to 256,
// against 272 when the rule was firing on ridges - while still leaving a
// settlement more water and more fish than it had then. It also brings the
// share of a map that comes out as watercourse back toward the four and a half
// in a hundred it was then asked for: the banks are laid after the channels
// are counted, so whatever they add is over the top of it, and at a metre they
// were adding two thirds again.
const bankRise = 0.0

// FloodDepth is how far above its river ground stops being valley floor, in
// metres. Below it the soil is what the water left; above it the ground is
// what the weather gives it.
const FloodDepth = 14.0

// SoilAt is what the land at p will hold: good on the damp flat of a valley
// facing the sun, over a mixture that keeps what it is given; poor on a steep
// dry hillside, and poor on sand however well it lies. It is read off the
// drainage and the soil's own make-up rather than stored, so that when the
// ground moves the soil that the ground can carry moves with it. The map is
// made with it and every age of weather pulls the soil that is actually
// there toward it.
//
// The mixture enters as a multiplier and not as a term of its own, because
// that is what it is: a loam on a dry shoulder is still a dry shoulder, and
// the best-lying ground in the valley grows little if it is sand that will
// not hold water or clay that will not give it up. It is centred on a middling
// loam, so that a map's soils average to what they averaged before there was
// any such thing as a mixture.
func (g *Grid) SoilAt(p geom.Pos) float64 {
	t := g.At(p)
	damp := clamp01(1 - t.Drain/FloodDepth)
	steep := clamp01(g.Slope(p) / 0.25)
	lie := damp * (1 - 0.7*steep) * (0.75 + 0.5*g.Sunlight(p))
	return clamp01(0.15 + 0.85*lie*(0.75+0.5*t.Loam()))
}

// clamp01 holds a share inside [0,1].
func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// quantile returns the value at f through a sorted copy of v. It is how the
// generator turns "the steepest tenth" or "the wettest twentieth" into a
// number for this particular map.
func quantile(v []float64, f float64) float64 {
	return quantiles(v, f)[0]
}

// quantiles is several of those at once, off one sorted copy. Two readings
// of the same measure - the steep ground and the very steep ground, the
// rivers and the great rivers - are asked for together all through the
// making of a world, and each of them was sorting half a million numbers
// over again to answer a second question about the same list.
func quantiles(v []float64, fs ...float64) []float64 {
	c := append([]float64(nil), v...)
	out := make([]float64, len(fs))
	// A quantile is one number out of half a million, and sorting the other
	// half million to find it is work nobody asked for. nth partitions
	// instead and throws away the side the answer is not on. Where the list
	// has a NaN in it there is no order to partition by and the sort still
	// answers; nothing a map is measured by should ever have one, and the
	// look for it is one pass against a sort's twenty.
	if hasNaN(c) {
		sort.Float64s(c)
		for k, f := range fs {
			out[k] = c[int(f*float64(len(c)-1))]
		}
		return out
	}
	for k, f := range fs {
		out[k] = nth(c, int(f*float64(len(c)-1)))
	}
	return out
}

func hasNaN(v []float64) bool {
	for _, x := range v {
		if math.IsNaN(x) {
			return true
		}
	}
	return false
}

// nth is the kth smallest of v, and it shuffles v about to find it. The
// answer is the same number a sorted copy would have at k - which of the
// equal ones it is cannot be told apart, because they are equal - so a
// reading taken this way is the reading that was always taken.
//
// The partition is three-way, which matters more here than anywhere: a map
// with a plain on it has tens of thousands of tiles of slope exactly zero,
// and a partition that only knows less and not-less walks all of them one
// at a time. Split into less, equal and greater, a run of equal values is
// finished in the pass that finds it.
func nth(v []float64, k int) float64 {
	lo, hi := 0, len(v)-1
	for lo < hi {
		p := med3(v, lo, hi)
		// Dutch flag: v[lo:lt] below p, v[lt:gt+1] equal to it, v[gt+1:] above.
		lt, i, gt := lo, lo, hi
		for i <= gt {
			switch {
			case v[i] < p:
				v[lt], v[i] = v[i], v[lt]
				lt++
				i++
			case v[i] > p:
				v[gt], v[i] = v[i], v[gt]
				gt--
			default:
				i++
			}
		}
		switch {
		case k < lt:
			hi = lt - 1
		case k <= gt:
			return p
		default:
			lo = gt + 1
		}
	}
	return v[k]
}

// med3 is the middle of the first, last and middle values of the range: a
// pivot that costs nothing and keeps ground that is already in order - a
// height field read row by row very nearly is - from being the worst case.
func med3(v []float64, lo, hi int) float64 {
	a, b, c := v[lo], v[(lo+hi)/2], v[hi]
	if a > b {
		a, b = b, a
	}
	if b > c {
		b = c
		if a > b {
			b = a
		}
	}
	return b
}

// Height is the height of a tile in metres. Off the map it is the sea the
// water eventually reaches, which is what makes every hollow drain somewhere.
func (g *Grid) Height(p geom.Pos) float64 {
	if !g.In(p) {
		return -1
	}
	return g.At(p).Height
}

// Slope is how steeply the ground falls away from a tile: the greatest drop
// to any neighbour, as a rise over a run. A tenth is a gentle hill, a half is
// ground you would not plough.
func (g *Grid) Slope(p geom.Pos) float64 {
	h := g.Height(p)
	steepest := 0.0
	for _, off := range Dirs {
		q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
		if !g.In(q) {
			continue
		}
		run := TileSpan
		if off.X != 0 && off.Y != 0 {
			run *= math.Sqrt2
		}
		if d := (h - g.Height(q)) / run; d > steepest {
			steepest = d
		}
	}
	return steepest
}

// Aspect is the way a slope faces: the step down the steepest fall, which is
// the way water leaves and the way the ground looks. The zero step means level
// ground, or a hollow with nowhere lower to go.
//
// The steepest fall and not the lowest neighbour, which is what this asked for
// until it was looked at. A diagonal neighbour is half again as far off as a
// straight one, so on ground that falls evenly it is lower by half again -
// and picking the lowest therefore picked a diagonal every time, on every
// even slope, and the same diagonal every time, because the first one the
// direction list offers wins a tie. What that draws is not drainage. It is a
// set of parallel lines at forty-five degrees, ruled across every plain on the
// map: two river tiles in three left their tile cornerways, where an honest
// surface gives about one in two, and the long straight rivers on the flats of
// a globe were all of them this and none of them ground.
//
// Dividing the drop by the distance is what Grid.Slope has always done, three
// functions above. This is the one reading of the same eight neighbours that
// did not.
func (g *Grid) Aspect(p geom.Pos) geom.Pos {
	h := g.Height(p)
	best, steepest := geom.Pos{}, 0.0
	for _, off := range Dirs {
		q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
		if !g.In(q) {
			continue
		}
		run := 1.0
		if off.X != 0 && off.Y != 0 {
			run = math.Sqrt2
		}
		if d := (h - g.Height(q)) / run; d > steepest {
			best, steepest = off, d
		}
	}
	return best
}

// Sunlight is how much of the day's warmth a tile's face catches, in [0,1].
// North is up the map, so ground that falls away southward looks at the sun
// and ground that falls away northward stands in its own shadow. Level ground
// is halfway between. Steep ground makes more of whichever it is.
func (g *Grid) Sunlight(p geom.Pos) float64 {
	a := g.Aspect(p)
	if a == (geom.Pos{}) {
		return 0.5
	}
	// a.Y is positive southward, and a southward-facing slope is the sunny one.
	lean := float64(a.Y) / math.Sqrt(float64(a.X*a.X+a.Y*a.Y))
	return 0.5 + 0.5*lean*math.Min(1, g.Slope(p)/0.3)
}

// raise builds the height field. The land is made of two things, because a
// country is: the lie of it - a broad smooth swell, the valley and its
// shoulders - and the high country standing on part of that.
//
// They are made differently because they are different. The lowland is a sum
// of octaves, each half the span and half the height of the last, which is
// the shape gentle ground has: swells with no edge to them. The high ground
// is the same octaves folded at their middle - the crest of each ridge is
// where the noise crossed its own centre - which is the shape ground has
// where it was pushed up and then cut into rather than laid down: ridges with
// a line along the top, and sides that fall away from them.
//
// A single texture at a single amplitude was what this used to be, and it
// gave every seed the same gentle bowl: nine tiles in ten under a tenth of a
// slope on most maps and under a sixth on all of them, with the highest
// ground only the largest of the same lumps. There was nothing to walk round
// and nothing to look up at.
func (w *Land) raise(g *Grid) {
	h := w.relief(g)
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			g.Tiles[i].Height = h[i]
		}
	})
}

// relief is the drawn height field itself, without putting it on the map. It
// is what raise writes down, and it is also what a history is measured
// against: a made world says where its high ground is, and this says how high
// a map's ground is spread. See Grid.normalise in history.go.
func (w *Land) relief(g *Grid) []float64 {
	// The lie of the land is drawn at the size of the map, because a swell is
	// whatever the country it lies on is; the ridges are drawn at the size of
	// a mountain range, because a range is not.
	lie := w.fold(g, float64(g.Span())/2, false)
	crest := w.fold(g, math.Min(float64(g.Span())/2, rangeSpan), true)

	where := w.upland(g)
	rise := g.UplandRise()
	h := make([]float64, len(g.Tiles))
	q := quantiles(where, 1-uplandShare, 1)
	foot, top := q[0], q[1]
	reach := math.Max(1e-9, top-foot)

	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			// The mask is eased rather than cut, so that the mountains have feet:
			// ground just past the line rises a little and is a hill, ground at
			// the top of it rises the whole way. Cut straight, the high country
			// began at a wall with no approach to it.
			m := smooth(clamp01((where[i] - foot) / reach))
			// Part of the rise is the mass of the upland and part of it is the
			// ridges on that mass, because a mountain is both and the two do not
			// peak in the same places. Given entirely to the ridges, a seed whose
			// crests happened to fall away from its high ground came out with no
			// mountains at all - two of the first five did, and were the same
			// gentle bowl the whole thing was meant to stop being.
			h[i] = Relief*lie[i] + rise*m*(uplandMass+(1-uplandMass)*crest[i])
		}
	})
	return h
}

// upland is where the high country stands: a mask far coarser than anything in
// the octaves above, so that upland is a region of the map rather than a
// speckle through it, taken down in halves to the span of a ridge so that the
// region has an outline.
//
// The outline is the whole of the finer octaves' work. Drawn from one lattice
// the mask is a bilinear blend of a few corners and what it lets through is a
// disc: a round massif with a smooth edge the whole way round and one summit
// in the middle, which is the blob a globe kept coming out as. The octaves
// under it put arms on that, and saddles in it, and outlying hills beside it,
// so that what stands above the line is a range with a shape rather than a
// hill with a radius.
//
// It stops at the ridges because below that the ridges are already saying
// where the peaks are, and a mask any finer would only argue with them. On a
// map too small to hold a whole range the first octave is already that fine,
// and this is the single lattice it always was.
func (w *Land) upland(g *Grid) []float64 {
	out := make([]float64, len(g.Tiles))
	amp := 1.0
	for span := g.UplandLattice(); ; span, amp = span/2, amp/2 {
		l := w.lattice(g, span)
		g.EachRow(func(y int) {
			for i := y * g.W; i < (y+1)*g.W; i++ {
				out[i] += amp * l[i]
			}
		})
		if span/2 < rangeSpan {
			return out
		}
	}
}

// fold sums the octaves, the coarsest of them span tiles across, in [0,1]
// before it is scaled. Folded, each octave is turned inside out at its middle
// and weighted by how high the coarser ones left it, which is what puts the
// fine detail on the flanks of the big ridges instead of spreading it evenly
// over everything: a mountain gets gullies and a plain stays a plain.
func (w *Land) fold(g *Grid, span float64, ridged bool) []float64 {
	out := make([]float64, len(g.Tiles))
	carry := make([]float64, len(g.Tiles))
	for i := range carry {
		carry[i] = 1
	}
	// The octaves run until they are finer than a tile rather than for a
	// fixed count, so that a bigger map gets more detail rather than the same
	// detail stretched over it. From forty tiles down that is five of them,
	// which is what the default valley always had.
	amp, step := 1.0, span
	for step >= 2 {
		lattice := w.lattice(g, step)
		g.EachRow(func(y int) {
			for i := y * g.W; i < (y+1)*g.W; i++ {
				v := lattice[i]
				if ridged {
					v = 1 - math.Abs(2*v-1)
					v *= carry[i]
					carry[i] = clamp01(0.4 + 0.6*v)
				}
				out[i] += amp * v
			}
		})
		amp, step = amp/2, step/2
	}
	// Stretched to fill [0,1], so that the top of a ridge means the top of
	// the ridge on this map rather than whatever fraction of its octaves
	// happened to agree there. Without it a crest reached a third of the
	// height it was given and every mountain came out a hill.
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, v := range out {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	reach := math.Max(1e-9, hi-lo)
	g.EachRow(func(y int) {
		for i := y * g.W; i < (y+1)*g.W; i++ {
			out[i] = (out[i] - lo) / reach
		}
	})
	return out
}

// lattice is one octave: random corners span tiles apart, smoothly blended.
func (w *Land) lattice(g *Grid, span float64) []float64 {
	cols, rows := int(float64(g.W)/span)+2, int(float64(g.H)/span)+2
	// On a globe the corners go round: a whole number of them fit the
	// width, and the last blends into the first, so that the ground on one
	// side of the seam is the same ground as on the other. The spacing is
	// nudged to make them fit, by less than a corner over the whole width.
	across := span
	if g.Wrap {
		cols = max(1, int(math.Ceil(float64(g.W)/span)))
		across = float64(g.W) / float64(cols)
	}
	corner := make([]float64, cols*rows)
	for i := range corner {
		corner[i] = w.RNG.Float64()
	}
	// The corners are drawn above, in one order, on this goroutine. What
	// follows is a blend of them and draws nothing, so it is spread over the
	// rows. See Grid.EachRow.
	out := make([]float64, len(g.Tiles))
	g.EachRow(func(y int) {
		for x := 0; x < g.W; x++ {
			fx, fy := float64(x)/across, float64(y)/span
			cx, cy := int(fx), int(fy)
			tx, ty := smooth(fx-float64(cx)), smooth(fy-float64(cy))
			at := func(dx, dy int) float64 {
				c := cx + dx
				if g.Wrap {
					c %= cols
				}
				return corner[(cy+dy)*cols+c]
			}
			top := at(0, 0)*(1-tx) + at(1, 0)*tx
			bot := at(0, 1)*(1-tx) + at(1, 1)*tx
			out[y*g.W+x] = top*(1-ty) + bot*ty
		}
	})
	return out
}

// smooth is the ease that turns a lattice of corners into hills rather than
// facets: flat where it meets a corner, steepest halfway between.
func smooth(t float64) float64 { return t * t * (3 - 2*t) }

// Incise is how far the water has cut into the ground it has been running
// over, in metres, along a great river: see greatFlow. It is what makes a
// valley a valley rather than a dip: raised and left alone, a river lies on
// the surface of the country like a line drawn on it, and the ground falls
// away from the water at a slope nobody can see. Cut down, the river sits at
// the bottom of something and the ground beside it is a bank.
//
// Twelve metres, and not more, because of what is beside the channel rather
// than what is in it. FloodDepth says the valley floor is the ground within
// fourteen metres of its river, which is where the soil is and where a
// settlement feeds itself. Cut deeper than that and the river's own banks
// stand above its flood plain: at thirty-four metres the soil on the gentle
// ground of all five seeds tried sat on its floor of 0.15, which is a gorge
// with nothing growing in it and not a valley.
const Incise = 12.0

// incise deepens the ways the water has already found. It runs on the first
// drainage, before the rivers are drawn, so the channels are drawn into
// ground that has been cut rather than onto ground that has not - and the
// heights are settled again afterwards, because ground that has moved drains
// differently.
//
// The cut is charged as the root of how much water crosses a tile: a gully
// cuts nearly as deep as the river it feeds, and the difference between a
// great river and a small one is far less than the difference in what they
// carry.
//
// There is no fall in that and there should not be, which is worth setting
// down because it looks like an omission and is not. The erosion in erode.go
// is E = K·A^m·S^n, the stream power law, with m a half and n one - see wear -
// and this is the same water on the same ground and takes only the A of it.
// The difference is that wear runs an age at a time, over and over, and this
// runs once. Stream power says how fast a channel is cutting now; a channel on
// its own flood plain, carrying everything and falling nowhere, is cutting
// nothing now and still lies at the bottom of a valley, because it spent ages
// getting there. What this pass wants is the depth at the end of that and not
// the rate at the start of it.
//
// Measured rather than argued: giving this the S term takes the mean cut on
// the low half of a default valley from 2.49 metres to 0.60 and puts it on the
// top fifth instead, from 1.42 to 2.83; and the valley's own trunk - the tile
// where the river leaves the map, whose fall is exactly zero because there is
// nothing below it - goes from 26.7 metres of cut to 0.03. The valley the
// settlement lives in stops existing. A globe does the same, harder: 0.78 to
// 0.10 on the low half and 1.22 to 4.96 on the top fifth.
//
// It is then spread over the ground either side before it is taken off, which
// is what makes this a valley and not a trench. Applied where it was
// computed, the whole depth landed in a channel one tile wide with walls
// standing straight up out of the flood plain, and the map got steeper
// everywhere without looking like anything.
func (g *Grid) incise() {
	cut := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		if g.standing(i) {
			continue // still water cuts nothing: it is where the cutting stops
		}
		// Charged by the water, and paid by the rock: the same river cuts a
		// gorge through shale and is turned aside by granite.
		cut[i] = Incise * math.Sqrt(greatShare(g.Tiles[i].Flow)) / g.Tiles[i].Hard()
	}
	for pass := 0; pass < valleyWidth; pass++ {
		cut = g.spread(cut)
	}
	for i := range g.Tiles {
		g.Tiles[i].Height -= cut[i]
	}
}

// valleyWidth is how far the cut is carried out from the channel, in passes
// of the blur below and so roughly in tiles. Three is a valley a few hundred
// metres across, with sides that can be walked up.
const valleyWidth = 3

// spread is one pass of a blur: every tile becomes the mean of itself and the
// eight around it, with the edge of the map reflecting rather than pulling
// toward nothing.
func (g *Grid) spread(v []float64) []float64 {
	out := make([]float64, len(v))
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			sum, n := 0.0, 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					q := geom.Pos{X: x + dx, Y: y + dy}
					if !g.In(q) {
						continue
					}
					sum, n = sum+v[g.Index(q)], n+1
				}
			}
			out[y*g.W+x] = sum / float64(n)
		}
	}
	return out
}

// outlet reports whether water leaves the map at a tile: the edge of a
// valley, and on a globe nothing at all, because a globe has no edge to leave
// by. Its water leaves at the sea, which the drainage already counts as
// somewhere water leaves.
//
// The poles used to count. They are the two rows a cylinder stops at, so they
// looked like edges and were treated as ones - which made every tile of them
// a drain a thousand tiles long, pinned to its own height, never filling and
// never holding a lake, with the whole of the map's drainage biased toward
// whichever of them was nearer. The sea was put in to stop rivers running to
// a pole and cutting the country in two; this stops them wanting to.
//
// A globe with no sea has nowhere else for its water to go, and on a map with
// no outlet anywhere every drop that falls stays on it - the whole of it one
// basin, standing as full as the weather keeps it. So that map, and only that
// map, still drains at its poles.
func (g *Grid) outlet(x, y int) bool {
	if !g.Wrap {
		return x == 0 || y == 0 || x == g.W-1 || y == g.H-1
	}
	return g.sea < 0 && (y == 0 || y == g.H-1)
}

// spreadUntil is how much ground, in tiles, the water running off a hillside
// gathers before it keeps to one way down. It is counted in ground and not in
// water because what it guards against is the grid's and not the weather's: a
// wet country gathers a stream's worth of water off fewer tiles, and read in
// water the chessboard below came back wherever it rained hard.
//
// Sent whole to the steepest neighbour from the first drop, the water on a
// smooth face split into lines a tile apart that never met, and on a face
// that fell diagonally, tiles here and there stepped sideways into the next
// line - so neighbouring lines carried thirty tiles' water and two, turn and
// turn about. Every other one cleared the line a river is picked by, and the
// foot of every range was drawn as a chessboard. Water on a hillside is a
// sheet and goes down every way that falls; it is only once it has gathered
// that it runs in one bed.
//
// The share each lower neighbour takes goes as its fall, which is Quinn's
// multiple-flow reading (Quinn and others, Hydrological Processes, 1991).
// Freeman's power of 1.1 on the fall draws the same map and made a globe take a
// sixth longer, all of it spent in the power. Sixty-four tiles is four hectares, past the one a channel
// head needs - see channelHead - so a head is still picked from water that
// has come together rather than from a sheet. Over two half globes, wet tiles
// with water on three corners and none beside them went from 12.0 and 10.5 in
// a thousand to 2.4 and 5.3 with this alone, and to none with the corners of
// diagonal steps filled in as well - see carve.
const spreadUntil = 64.0

// crowdSpace is how near, in tiles, a small stream may start to another
// channel it does not join, and crowdUntil is how much ground's rain, in
// tiles, makes a stream no longer small.
//
// Nothing kept the streams apart, so a range whose flank fell evenly drew a
// stream down every third or fourth line of tiles, each cut into the ground
// beside the next: a mountain carved into ribs rather than drained. On the
// ground a hillside between two streams sheds its water into one or the other,
// and a third between them has no ground of its own to gather from - which is
// what sets how far apart a range's streams stand.
//
// A stream that meets the other within three times the spacing is a
// tributary, and those are left alone: a river's branches draw together
// because they are going to the same place. And a stream that has gathered
// crowdUntil has earned its bed wherever it is. Over two half globes, river
// tiles with an unjoined channel within three tiles went from 41 and 34 per
// cent of the river to 17 and 16.
const (
	crowdSpace = 3
	crowdUntil = 256.0
)

// channelPower is the power, in watts on each metre of bed, that water has to
// spend on its bed to have cut one, and channelHead is the least ground, in
// tiles, a channel's head has to drain. Between them they are where a river
// starts: see carve.
//
// A river is not a share of the map. It is where the water running down the
// ground has the power to keep a bed open against the soil creeping into it,
// and that is rho*g*Q*S - stream power, the reading channel initiation and
// incision are both measured in (Bagnold, 1966; Montgomery and Dietrich,
// 1992). Read off real discharge it puts rivers where the rain runs off and
// the ground falls, so a wet country has more of them and a dry one fewer, and
// the ranges of a desert stand dry.
//
// Stream power and not bed shear, which was tried first. Shear goes as the
// depth, which goes as the flow to the power 0.39, and a reading that weak in
// the water turns rivers on and off like a switch: over five seeds of the
// valley, with the threshold set to give eight tiles in a hundred of it river,
// half the rain gave none at all.
//
// The figures in the tables below were read with the water off twelve hundred
// metres of catchment a tile, 2304 times the tile's own ground - see
// weather.go - and the power goes as the water, so each is 2304 times the
// power the same rivers spend at their real discharge. That real figure,
// under a watt on a metre of bed, is a litre a second falling one in ten: the
// order of the mean flow off the few thousand square metres a channel head in
// soil-mantled country drains (Montgomery and Dietrich 1988, 1992). Over the
// first five seeds of the valley, and three small globes, at a head of eight:
//
//	power    valley river   upland share   half the rain   twice   small globe
//	  700       13.1%          17.2%           4.1%        22.6%      3.1%
//	 1200        7.8%          12.3%           3.0%        17.6%      2.1%
//	 1500        6.1%           7.1%           2.0%        15.8%      1.7%
//	 2000        4.5%           1.5%           1.9%        13.9%      1.2%
//
// Fifteen hundred keeps the valley near the six in a hundred it had when the
// rivers were a share, and a globe - far less of it steep, and a third of its
// land desert or ice - comes out with a river network about a quarter as
// dense, which is what the drier world should have.
//
// Shaped - see shape.go - the valley's lowland stands higher and steeper, and
// every tile that gathers water falls toward its river at the fall that water
// holds a channel at, so more of the ground cuts a bed at the same power. At
// fifteen hundred the first valley came out 12.2 in a hundred water; asked the
// same of the first three valleys, by power:
//
//	power    seed 1   old river turning, seed 1   migration, widths a year
//	 1500    12.2%            0.468                      0.0015
//	 1800    11.1%            0.414                      0.0023
//	 2100     9.9%            0.414                      0.0034
//	 2400      -              0.445                      0.0038
//
// Twenty-one hundred is the least that keeps the first valley a valley and not
// a marsh; at twenty-four hundred the third seed's river came out with fewer
// bends after forty ages than it started with.
//
// The head is counted in ground because it guards against the grid and not
// against the weather: see spreadUntil. At sixteen tiles, and a power of 800,
// the high fifth of five valleys held a thousandth of their river: the water
// spread over a hillside rarely gathers from that many before it reaches the
// foot.
const (
	channelPower = 2100.0 / 2304 // watts a metre
	channelHead  = 8.0
)

// bankShare is how much of a map's river, by how much water it carries, is
// great enough to spread onto its banks.
const bankShare = 0.25

// streamPower is the power the water spends on each metre of its bed where q
// cubic metres a second run down a fall of s, in watts a metre: ρ·g·Q·S.
func streamPower(q, s float64) float64 {
	if q <= 0 || s <= 0 {
		return 0
	}
	return 1000 * 9.81 * q * s
}

// carve puts the water where the flow says it goes: a river starts wherever the
// water has the power to hold a bed open - see channelPower - and runs from
// there to the sea, and the greatest of them spread onto the lower bank beside
// them, as a river does. Ground the water has left goes back to grass.
//
// It runs at the making of the map and again after every age of weather, so a
// river can take a course it did not have. It will not run through anything
// anybody has built or claimed: a settlement embanks what it stands on, and a
// river that swallowed the market would be the end of a run rather than an
// event in it.
func (g *Grid) carve(rng interface{ Float64() float64 }) {
	// What the water has the power to do here: see channelPower.
	cutting := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		s := g.Slope(g.PosOf(i))
		if s > channelSteep { // a scree, not a stream bed: see channelSteep
			s = channelSteep * channelSteep / s
		}
		cutting[i] = streamPower(g.Tiles[i].Flow, s)
	}
	cut := channelPower
	// Whether a river is great enough to spread onto its banks is a question
	// about how much water it is carrying and not about how hard it is
	// cutting, so it is read off the flow and not off the work. They are not
	// the same question and the answers point opposite ways: the hardest
	// cutting on a map is a steep rill near a ridge, which carries nothing.
	// Read off the work, nine tenths of the tiles allowed to flood their banks
	// on a globe stood in the top fifth of the ground - mountainsides in
	// flood, with the flood plains dry.

	// Hysteresis, which is what cut/2 below is for. Ground becomes river when
	// the water really gathers there, and stops being river only when the
	// water has largely gone - not the moment it dips below the line. Without
	// it a settlement wipes out its own river: it holds the ground the
	// shifting channel wants, so the new course cannot form, while the old one
	// dries the instant it falls under the threshold.
	// A channel is laid from its head down, and goes on being a channel until
	// it reaches the sea. Where the water has cut is read tile by tile above,
	// and read that way alone a river comes apart: the reading is flow against
	// fall, so a trunk crossing its own flood plain - all the water on the map
	// and no fall at all - drops under the line its own headwaters cleared.
	// What that draws is a mountain full of streams, a plain with nothing on
	// it, and a scatter of blue dashes in between where the ground happened to
	// tilt. Water does not do that; it goes somewhere.
	//
	// So the heads are taken hardest-working first and each is followed down
	// to the sea.
	//
	// The lakes are water before any channel is laid, as the sea is, and a
	// channel that reaches one stops there: what goes on is the river out of
	// its outlet, which is a head of its own. A salt flat is where a channel
	// stops too, because the water that reaches one goes no further.
	wet := make([]bool, len(g.Tiles))
	land := 0
	for i := range g.Tiles {
		if g.underSea(i) || g.lakeOf[i] >= 0 {
			wet[i] = true
		} else {
			land++
		}
	}
	// The least ground a head may start from: see channelHead.
	head := channelHead
	lay := func(from int) {
		for j := from; !wet[j] && !g.pans[j]; {
			wet[j] = true
			if g.down[j] < 0 {
				return // off the map, which is where a valley's water goes
			}
			p := g.PosOf(j)
			q := g.PosOf(int(g.down[j]))
			a := g.flowStep(j)
			// A diagonal step joins its two tiles at a corner only, and two
			// rivers doing it a tile apart are the chessboard again. So the
			// lower of the two tiles either side of the step is wet too: the
			// bed a river cuts round a corner and not through the point of it.
			// It counts for part of a tile against the share: see cornerWeight.
			if a.X != 0 && a.Y != 0 {
				side := geom.Pos{X: p.X + a.X, Y: p.Y}
				if other := (geom.Pos{X: p.X, Y: p.Y + a.Y}); g.Height(other) < g.Height(side) {
					side = other
				}
				wet[g.Index(side)] = true
			}
			j = g.Index(q)
		}
	}
	// Hardest-working first, ties by position so that the same map comes out
	// however the sort happened to run.
	order := make([]heightNode, 0, land)
	for i := range g.Tiles {
		if !wet[i] && !g.pans[i] {
			order = append(order, heightNode{h: cutting[i], idx: int32(i)})
		}
	}
	slices.SortFunc(order, func(a, b heightNode) int {
		if a.h != b.h {
			return cmp.Compare(b.h, a.h)
		}
		return cmp.Compare(a.idx, b.idx)
	})
	// Where each tile's water goes, for asking whether two streams meet. -1
	// is nowhere: the sea, a salt flat, or off the map.
	down := g.down
	// crowded says whether a small stream starting at i would run beside a
	// channel it does not join: see crowdSpace.
	crowdFlow := crowdUntil
	reach := crowdSpace * 3
	path := make([]int32, 0, reach+1)
	on := func(j int32) bool { return slices.Contains(path, j) }
	crowded := func(i int32) bool {
		if g.area[i] >= crowdFlow {
			return false
		}
		path = path[:0]
		for j, k := i, 0; j >= 0 && k <= reach; j, k = down[j], k+1 {
			path = append(path, j)
		}
		p := g.PosOf(int(i))
		for dy := -crowdSpace; dy <= crowdSpace; dy++ {
			for dx := -crowdSpace; dx <= crowdSpace; dx++ {
				q := geom.Pos{X: p.X + dx, Y: p.Y + dy}
				if !g.In(q) {
					continue
				}
				w := int32(g.Index(q))
				if !wet[w] || g.underSea(int(w)) || g.lakeOf[w] >= 0 || on(w) {
					continue
				}
				joins := false
				for j, k := w, 0; j >= 0 && k <= reach; j, k = down[j], k+1 {
					if on(j) {
						joins = true
						break
					}
				}
				if !joins {
					return true
				}
			}
		}
		return false
	}
	// A channel does not flicker, so a bed that is still being cut at half
	// the rate keeps its water whether or not it would be chosen afresh. See
	// the remark on hysteresis above.
	for _, nd := range order {
		if g.Tiles[nd.idx].Wet() && nd.h >= cut/2 && g.area[nd.idx] >= head && !crowded(nd.idx) {
			lay(int(nd.idx))
		}
	}
	for _, nd := range order {
		if nd.h < cut {
			break
		}
		if g.area[nd.idx] < head || wet[nd.idx] || crowded(nd.idx) {
			continue
		}
		lay(int(nd.idx))
	}
	// The great rivers spread onto the ground beside them that the flood
	// reaches: see bankRise. Whether a river is great enough is a question
	// about how much water it is carrying and not about how hard it is
	// cutting, so it is read off the flow and not off the work: the hardest
	// cutting on a map is a steep rill near a ridge, which carries nothing.
	var network []float64
	for i := range g.Tiles {
		if wet[i] && !g.underSea(i) && g.lakeOf[i] < 0 {
			network = append(network, g.Tiles[i].Flow)
		}
	}
	big := math.Inf(1)
	if len(network) > 0 {
		big = quantile(network, 1-bankShare)
	}
	for i := range g.Tiles {
		if g.Tiles[i].Flow < big || g.standing(i) {
			continue
		}
		p := geom.Pos{X: i % g.W, Y: i / g.W}
		for _, off := range Dirs {
			c := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
			if g.In(c) && !g.pans[g.Index(c)] && g.Height(c) <= g.Height(p)+bankRise {
				wet[g.Index(c)] = true
			}
		}
	}
	for i := range g.Tiles {
		t := &g.Tiles[i]
		held := t.Mark != None || t.Owner != 0
		// What the ground should be: a salt lake where a lake has no outlet,
		// water wherever else the water is, a salt flat on the dry floor of a
		// closed basin, and ground everywhere else.
		want := Grass
		switch {
		case g.closedLake(i):
			want = Salt
		case wet[i]:
			want = Water
		case g.pans[i]:
			want = Pan
		}
		switch {
		case want == t.Terrain:
		case (want == Water || want == Salt) && t.Terrain == Ice:
			// Frozen over; freeze says whether it still is.
		case want == Water || want == Salt:
			if !t.Wet() {
				if held || (t.Terrain == Flat && g.underSea(i)) {
					// A flat under mean sea is the tide's, and the sea is
					// not laid over it here: see shore.go.
					continue
				}
				g.Wood[i], g.Wild[i], g.Age[i] = 0, 0, 0
			}
			t.Terrain, g.Fish[i] = want, 0
			if want == Water {
				g.Fish[i] = 0.7 + 0.3*rng.Float64()
			}
		case want == Pan:
			if held || t.Terrain == Field || t.Terrain.Tidal() {
				continue
			}
			t.Terrain, g.Fish[i], g.Wood[i], g.Wild[i], g.Age[i] = Pan, 0, 0, 0, 0
		case t.Wet() || t.Terrain == Pan:
			t.Terrain, g.Fish[i] = Grass, 0
		}
	}
}

// height reads how far a tile stands above the water it drains into, in
// metres, and writes it into Drain. It is the truest thing the land can say
// about how wet a place is, and much truer than how much water passes through
// it: a tile on a valley floor beside the river carries hardly any flow of its
// own and is still a water meadow, while a tile halfway up a hillside may
// carry a whole gully's worth and still be dry as a bone. Flood plains sit at
// nothing, terraces a few metres up, hillsides tens.
//
// It is computed by following each tile's water down to the water it joins and
// adding up the fall on the way. Tiles are settled in the order the drainage
// reached them, so the tile downstream is always finished before the one that
// drains into it. A salt flat is where water stands when it stands at all, so
// it is at its water; and ground lying in a hollow too shallow to hold a lake
// is at the water that crosses it, and not below it.
func (g *Grid) height() {
	lowest := math.Inf(1)
	for i := range g.Tiles {
		lowest = math.Min(lowest, g.Tiles[i].Height)
	}
	for _, i := range g.route {
		t := &g.Tiles[i]
		if t.Wet() || g.pans[i] {
			t.Drain = 0
			continue
		}
		d := g.down[i]
		if d < 0 {
			t.Drain = t.Height - lowest // the water leaves the map here
			continue
		}
		t.Drain = math.Max(0, t.Height-g.Surface(int(d))+g.Tiles[d].Drain)
	}
}

// heightNode is a tile with a number beside it to sort it by: its height, or
// how hard the water is cutting it.
type heightNode struct {
	h   float64
	idx int32
}

// The sea. A valley has none: its water leaves at the edges of the map. A
// globe has no edges but the poles, and a globe with no sea is a globe
// where every river runs to a pole and cuts the country in two from top to
// bottom, which no laden walker can get round. So a share of the lowest
// ground on a globe is put under the sea before the water finds its way
// down, and the sea is where it finds its way to.

// underSea reports whether the tile at i lies at or below sea level. A map
// with no sea has a sea level below all its ground.
func (g *Grid) underSea(i int) bool {
	return g.sea >= 0 && g.Tiles[i].Height <= g.sea
}

// seaNear is, for each tile, how much of the country within reach of it lies
// under the sea, in [0,1]: nothing deep inside a continent, a half on an even
// coast, nearly all of it on a rock in the open ocean. It is the reading the
// frost takes of the water - see Maritime - and it is a share of a square
// window rather than a distance because what warms a place is how much sea is
// about it and not how few steps to the nearest drop of it.
//
// It is summed once over the whole map and then read off in constant time
// per tile, so a window sixty-four tiles wide costs no more than one four
// tiles wide. The window goes round the seam and is clipped at the poles,
// where there is nothing beyond to count.
func (g *Grid) seaNear(reach int) []float64 {
	stride := g.W + 1
	sum := make([]float64, stride*(g.H+1))
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			v := 0.0
			if g.underSea(y*g.W + x) {
				v = 1
			}
			sum[(y+1)*stride+x+1] = v + sum[y*stride+x+1] + sum[(y+1)*stride+x] - sum[y*stride+x]
		}
	}
	// box is how many tiles of the sea lie in the inclusive rectangle, whose
	// columns must already be on the map.
	box := func(x0, x1, y0, y1 int) float64 {
		return sum[(y1+1)*stride+x1+1] - sum[y0*stride+x1+1] - sum[(y1+1)*stride+x0] + sum[y0*stride+x0]
	}
	near := make([]float64, len(g.Tiles))
	g.EachRow(func(y int) {
		y0, y1 := max(0, y-reach), min(g.H-1, y+reach)
		rows := y1 - y0 + 1
		for x := 0; x < g.W; x++ {
			var wet float64
			var n int
			switch {
			case !g.Wrap:
				x0, x1 := max(0, x-reach), min(g.W-1, x+reach)
				wet, n = box(x0, x1, y0, y1), (x1-x0+1)*rows
			case 2*reach+1 >= g.W:
				// A window wider than the map reads each column once.
				wet, n = box(0, g.W-1, y0, y1), g.W*rows
			default:
				x0, x1 := x-reach, x+reach
				switch {
				case x0 < 0:
					wet = box(0, x1, y0, y1) + box(x0+g.W, g.W-1, y0, y1)
				case x1 >= g.W:
					wet = box(x0, g.W-1, y0, y1) + box(0, x1-g.W, y0, y1)
				default:
					wet = box(x0, x1, y0, y1)
				}
				n = (2*reach + 1) * rows
			}
			near[y*g.W+x] = wet / float64(n)
		}
	})
	return near
}

// relevel reads the sea level off the ground again, so that the share of the
// map under it is the share asked for. flood takes it before the rivers have
// cut their valleys, because the drainage the valleys are cut by needs a sea
// to run to; incise then takes up to Incise metres off the ground either side
// of every river, and where a lowland lies a few metres above the sea the
// whole of it goes under.
//
// That had always happened a little. It came to matter when the plates began
// to grow rather than be measured, because they leave broad flat lowlands, and
// on a flat lowland the sea's quantile falls in the middle of one. Measured
// after flood and after incise, of seeds 1 to 3:
//
//	                 after flood        after incise
//	globe            .300 .300 .300     .354 .304 .304
//	256 by 128       .300 .301 .300     .307 .429 .400
//
// Asked for three tenths, one small globe came out more than two fifths sea.
// Levelled again, the sea is where the share puts it on the ground as it will
// stand; carve then gives back to the land what is no longer under it and
// takes what now is, as it does after every age of weather.
func (g *Grid) relevel(share float64) {
	if g.sea < 0 || share <= 0 {
		return
	}
	heights := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		heights[i] = g.Tiles[i].Height
	}
	g.sea = quantile(heights, share)
	g.base = g.sea
}

// flood puts the lowest share of the ground under the sea, and reads the
// sea level off the ground so that erosion can move the coast.
func (g *Grid) flood(share float64, rng interface{ Float64() float64 }) {
	if share <= 0 {
		g.seaAt(-1, rng)
		return
	}
	heights := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		heights[i] = g.Tiles[i].Height
	}
	g.seaAt(quantile(heights, share), rng)
}

// seaAt puts the sea at level, and everything at or under it under the sea,
// with its fish. A level below nothing is no sea at all. It is where the level
// is found that differs between a sea given as a share of the map and one given
// as water - see flood and pour - and not what the level then does.
func (g *Grid) seaAt(level float64, rng interface{ Float64() float64 }) {
	g.sea, g.base = -1, -1
	if level < 0 {
		return
	}
	g.sea, g.base = level, level
	for i := range g.Tiles {
		if t := &g.Tiles[i]; g.underSea(i) {
			t.Terrain, g.Wood[i], g.Wild[i], g.Age[i] = Water, 0, 0, 0
			g.Fish[i] = 0.7 + 0.3*rng.Float64()
		}
	}
}
