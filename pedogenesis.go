package terra

import "math"

// What time does to a soil.
//
// A soil was a depth and a mixture: how much the rock had made less what had
// been taken, and how much of it was sand and how much clay. Nothing on the
// map knew how long any of it had been lying there, and so nothing knew what
// the lying there had done. Jenny (1941) put time among the five things a
// soil is made by, beside the climate, the organisms, the relief and the
// rock, and it is the one that tells the soils of the world apart more than
// any other: the same basalt under the same rain is a young brown soil full
// of everything a crop wants after two thousand years, and a red clay the
// rain has taken everything out of after four million. Chadwick and others
// (1999) read exactly that down the Hawaiian islands, flows of one rock under
// one climate from three hundred years old to 4.1 million: the bases held
// through the first twenty thousand years, fell away over the next hundred
// and fifty thousand as the minerals that make them weathered out, and were
// all but gone on Kauai. The old shields of Africa, Brazil and Australia
// carry the poorest soils on earth for no reason but that nothing has
// renewed their surface in millions of years.
//
// So each tile keeps a little more than its depth. How long its surface has
// been forming soil - Exposed - which the ground's own coming and going sets
// and resets: water or sea over it, the water or the creep cutting through
// the soil into the rock, a slide baring its scar, all start it again; what
// is laid on it is younger ground mixed into older, and brings the age down
// in the proportion it comes in. And three things that age does, each a
// stock that climbs toward what the tile's climate and cover would make of it
// and is taken off or diluted with the soil that holds it:
//
//   - Leached: how much of the bases the rock gave the soil - calcium,
//     magnesium, potassium, sodium - the water going through it has carried
//     away, which is base saturation turned round and as near as the map
//     comes to a pH. Water drives it; the rock's weathering buys it back for
//     as long as there are minerals left to weather.
//   - Lime and Salt: what the water brings down into a soil and cannot carry
//     out of it again, where the air takes back more than the rain gives.
//     Carbonate first, in the semi-arid country - the calcic horizons of the
//     steppes - and the salts, which dissolve far more readily,
//     only in the deserts where almost nothing runs through at all.
//   - Carbon: the organic matter, as a stock and not a multiplier: what grows
//     puts it in, the soil's life burns it off twice as fast for every ten
//     degrees, and what grows decides how much and where. Grass puts its
//     carbon deep, in roots; conifers drop a litter that turns the soil sour.
//
// A tile's state is five numbers in padding it already had, so it costs the
// map nothing: see Tile.

// Leaching is how much of the bases this tile's soil once held the water has
// carried off, 0 to 1: base saturation turned round.
func (t *Tile) Leaching() float64 { return float64(t.Leached) / math.MaxUint16 }

// Carbonate is the carbonate built up in this tile's soil, in kilograms a
// square metre.
func (t *Tile) Carbonate() float64 { return float64(t.Lime) * limeUnit }

// Salinity is the salt built up in this tile's soil, in kilograms a square
// metre.
func (t *Tile) Salinity() float64 { return float64(t.Salt) * saltUnit }

// What a unit of Lime and of Salt is, in kilograms a square metre. The most
// either can hold is 65535 of them: 655 of carbonate, which is past the
// petrocalcic horizons of the oldest desert soils, and 65 of salt.
const (
	limeUnit = 0.01
	saltUnit = 0.001
)

func (t *Tile) setLeaching(v float64) { t.Leached = uint16(math.Round(clamp01(v) * math.MaxUint16)) }
func (t *Tile) setCarbonate(v float64) {
	t.Lime = uint16(math.Round(math.Max(0, math.Min(math.MaxUint16, v/limeUnit))))
}
func (t *Tile) setSalinity(v float64) {
	t.Salt = uint16(math.Round(math.Max(0, math.Min(math.MaxUint16, v/saltUnit))))
}

// Exposure.
//
// A map is made with no history of its soil to read, so how long each tile has
// been forming soil is read off what the ground is doing now, as the
// cosmogenic nuclide studies read it: a surface lowering steadily at E, under
// a weathered layer Z deep, has been in that layer for Z/E years (Lal 1991;
// Heimsath and others 1997 read soil residence times of a few thousand to a
// few tens of thousands of years off it). The layer is the soil itself and
// the weathered rock under it the soil is still being made from, regolithDepth
// of it; E is the soil production function at the soil's steady depth, which
// is what the ground is lowering at where the soil is making what it loses,
// and never less than soilDeepest. So a crest going fast is a few thousand
// years old, a middling hillside some thirty thousand, and a hollow the creep
// only fills, lowering at Portenga and Bierman's bare outcrop rate, seven
// hundred thousand.
//
// regolithDepth is chosen, not measured: saprolite runs from nothing on a
// fresh scarp to tens of metres on a tropical shield, and two metres puts a
// middling valley's hillsides at the late Pleistocene, which is where the
// soils of the mid-latitudes date.
const regolithDepth = 2.0

// A valley floor is what its river has laid, and a flood renews it. How old it
// is goes with how far above the water it stands, which is a river terrace
// sequence: what a flood still reaches is a few hundred years old, and ground
// FloodDepth above the water was the river's floor when the river last cut
// down that far. riverYears is that last age: fourteen metres at a little
// under a millimetre a year of cutting down. Chosen and not fitted; the
// terraces of the world run from a tenth of a millimetre a year to several.
const (
	riverYoung = 300.0
	riverYears = 20e3
)

// Leaching.
//
// The bases go with the water that goes through the soil and are bought back
// by the weathering of the minerals that hold them. Written as a rate, with
// L the share lost,
//
//	dL/dt = a·(1 − L) − b·L,    a = q/leachWater,
//
// with q the water through the soil in millimetres a year, and b the
// weathering's buying back, set so that where it keeps up the soil comes to
// L = 1 − s·M: s the share of the loss the rock's minerals can make good while
// they last, and M how much of them is left, which falls as e^(−W·age/
// mineralYears) with W the weathering (see soil.go). Over a step it is taken
// exactly: L goes to its level at the rate a + b.
//
// leachWater and mineralYears are set off Chadwick and others' (1999) wettest
// Hawaiian sites: some two and a half metres of rain, perhaps 1.8 of it
// through the soil, at a mean of sixteen degrees, where the bases have gone
// by an e-fold at twenty thousand years and the primary minerals at fifty. At
// that climate W reads about 3.2, so an e-fold of the minerals is 160 thousand
// years at W of one; and 1.8 metres a year for twenty thousand years is 36
// kilometres of water. At the map's middling runoff the bases take 120
// thousand years to an e-fold, and in a wet temperate country half that.
const (
	leachWater   = 3.6e7 // mm of water through the soil to carry off an e-fold of its bases
	mineralYears = 160e3 // years for an e-fold of the weatherable minerals, at W of one
)

// baseSupply is, for each rock, the share of what the water takes that its
// minerals can make good while they last: plenty in basalt and in the lime of
// a limestone, little in the quartz of a sandstone or a granite.
//
// It is a placeholder. The rock's chemistry - its base cations and its
// carbonate - belongs to bedrock.go, and when the rocks carry it this table
// should be read off it there, and Lime seeded from a limestone's own
// carbonate as well as from the rain.
var baseSupply = [BedrockCount]float64{
	Basalt:    0.8,
	Limestone: 0.9,
	Shale:     0.6,
	Schist:    0.5,
	Granite:   0.3,
	Sandstone: 0.2,
}

// limeBuffer is how much carbonate, in kilograms a square metre, holds the
// soil's bases at half of what they would otherwise lose: while there is
// carbonate in a soil it dissolves before anything else goes, and a
// calcareous soil is base saturated whatever the rain.
const limeBuffer = 5.0

// Carbonate and salt.
//
// Both come down in the rain and the dust and are carried into the soil by
// the water that soaks in, and both are carried on out of it by the water
// that goes through. Where the air takes back nearly all the rain there is
// none of that last, and they build up: carbonate where the rain is less than
// the air could take up by some half (Royer 1999 has pedogenic carbonate in
// soils getting less than 760 millimetres a year), and the salts only where
// it is less than a fifth, the UNEP line of the arid.
//
// limeRate is Machette's (1985) middle: the calcic soils of the south-western
// United States gathered carbonate at a few kilograms a square metre in a
// thousand years. saltRate is a quarter of it, chosen. Each is carried out
// again at the rate the water through the soil would carry an e-fold of it:
// limeWater is set so that a humid soil at the map's middling runoff loses an
// e-fold of its carbonate in ten thousand years, and the salts, which are far
// the more soluble, go ten times faster. Neither is fitted.
const (
	limeRate   = 2e-3 // kg/m² a year, at full dryness
	saltRate   = 5e-4
	limeWater  = 3e6 // mm of water through the soil to dissolve an e-fold of what is there
	saltWater  = 3e5
	limeWetter = 0.6 // rain over what the air could take, past which no carbonate builds
	limeDrier  = 0.3 // and under which it builds at limeRate
	saltWetter = 0.25
	saltDrier  = 0.1
)

// Carbon.
//
// What grows puts carbon into the soil and the soil's life takes it out, at a
// rate that doubles for every ten degrees (the Q10 of two of Raich and
// Schlesinger 1992, as organic has it), and slows where the ground is
// waterlogged, which is how a peat builds. So the stock comes to input over
// decay, and it gets there in the decay's own time: carbonYears at the map's
// middling warmth, a century, the turnover of the part of a soil's organic
// matter that the plough and the crop live off.
//
// carbonMiddle is what open grass on a metre of soil under the map's middling
// climate comes to. Jobbágy and Jackson (2000), over 2700 profiles, have a
// temperate grassland's top metre holding 11.7 kilograms a square metre, a
// temperate deciduous forest's 17.4, a boreal forest's 9.3 and a desert's 6.2.
//
// carbonDepth is how fast the carbon thins with depth, as an e-fold in metres:
// a soil thinner than a metre holds the share of a metre's carbon that is in
// its thickness.
const (
	carbonYears  = 100.0
	carbonMiddle = 11.0 // kg C/m²
	carbonDepth  = 0.3  // m
)

// cover is what is growing on a tile, as the soil feels it: how much carbon it
// puts in against open grass, how fast what it puts in is burned off against
// grass, and how much faster than bare water it takes the bases.
//
// Grass puts most of what it grows below ground, in roots, and deep: Jobbágy
// and Jackson (2000) have 42 per cent of a grassland's top metre of carbon in
// its top twenty centimetres against 50 per cent of a forest's, and it is the
// roots that build the dark, deep, base-rich topsoils of the steppes. A
// forest's carbon is more of it litter on the top, where it burns off faster.
//
// Trees take the bases faster than grass: Jobbágy and Jackson (2003) found
// grassland planted to trees losing exchangeable calcium and going sour within
// decades, the trees drawing the bases up into their wood and sending organic
// acids down; and conifers most of all, whose needles make an acid litter
// (Augusto and others 2002, over the temperate forests of Europe). How much is
// chosen: a fifth faster under broadleaf, four fifths under conifer.
//
// A field loses its carbon to the plough: Guo and Gifford (2002), over 74
// studies, have pasture turned to crop losing 59 per cent of its soil carbon
// and forest turned to crop 42.
type cover struct{ input, decay, acid float64 }

var (
	grassCover   = cover{1, 1, 1}
	leafCover    = cover{1.3, 1.4, 1.2}
	needleCover  = cover{1.0, 1.1, 1.8}
	fieldCover   = cover{0.55, 1.35, 1}
	nothingCover = cover{0, 1, 1}
)

// coverOf is what grows on tile i. A wood is conifer where the year is cold
// and broadleaf where it is warm, with the mixed woods between: the boreal
// forests stand at yearly means under two or three degrees and the broadleaf
// deciduous from about ten.
func (g *Grid) coverOf(i int, temp float64) cover {
	t := &g.Tiles[i]
	switch t.Terrain {
	case Grass:
		return grassCover
	case Field:
		return fieldCover
	case Forest:
		leaf := ramp(temp, 2, 10)
		return cover{
			input: leaf*leafCover.input + (1-leaf)*needleCover.input,
			decay: leaf*leafCover.decay + (1-leaf)*needleCover.decay,
			acid:  leaf*leafCover.acid + (1-leaf)*needleCover.acid,
		}
	}
	return nothingCover
}

// forms reports whether tile t forms soil at all: dry ground with something on
// it that is not a crust of salt or bare stone.
func forms(t *Tile) bool {
	return !t.Wet() && !t.Terrain.Tidal() && t.Terrain != Rock && t.Terrain != Pan
}

// pedoClimate is what tile i's climate does to its soil: the water through
// it, how dry the air is against the rain, the weathering, the year's mean
// warmth, and how waterlogged the ground is.
type pedoClimate struct {
	water, wetness, weathering, temp, sodden float64
}

func (g *Grid) pedoClimateOf(i int) pedoClimate {
	c := pedoClimate{water: middleRunoff, wetness: 1, weathering: g.weathering(i), temp: g.meanTempOf(i)}
	if len(g.runoff) == len(g.Tiles) {
		c.water = math.Max(0, g.runoff[i])
		if g.air != nil {
			c.wetness = g.rain[i] / math.Max(1, g.pet(i))
		}
	}
	// Ground within a metre or two of the water it drains into is wet through
	// for much of the year.
	c.sodden = clamp01(1 - g.Tiles[i].Drain/2)
	return c
}

// leachLevel is where the bases come to on tile i and how fast: the level L
// heads for and the rate it heads there at, given how long the surface has
// been exposed.
func (g *Grid) leachLevel(i int, c pedoClimate, cv cover, age float64) (level, rate float64) {
	t := &g.Tiles[i]
	a := c.water * cv.acid / leachWater
	left := math.Exp(-c.weathering * age / mineralYears)
	level = (1 - baseSupply[t.Bedrock]*left) / (1 + t.Carbonate()/limeBuffer)
	if level <= 0 || a <= 0 {
		return 0, a
	}
	return level, a / level
}

// dryness is how far a climate reading wet over dry of what the air could take
// is past the line a stock starts building at, to where it builds fully.
func dryness(wetness, wetter, drier float64) float64 {
	return clamp01((wetter - wetness) / (wetter - drier))
}

// carbonLevel is what the carbon on tile i comes to and how fast: input over
// decay, for the soil it has, and the decay.
func (g *Grid) carbonLevel(i int, c pedoClimate, cv cover) (level, rate float64) {
	t := &g.Tiles[i]
	grow := growthOf(c.temp) * ramp(c.temp, -5, 5) * c.wetnessShare()
	middle := growthOf(MeanTemp) * ramp(MeanTemp, -5, 5) * (1 - math.Exp(-middleRunoff/weatherRunoff))
	decay := math.Pow(2, (c.temp-MeanTemp)/10) * (1 - 0.6*c.sodden) * cv.decay / carbonYears
	held := -math.Expm1(-float64(t.Soil)/carbonDepth) / -math.Expm1(-1/carbonDepth)
	return carbonMiddle * grow / middle * cv.input * held / (decay * carbonYears), decay
}

// wetnessShare is the water the growing has, as West's runoff term the organic
// matter was always read with (see organic).
func (c pedoClimate) wetnessShare() float64 {
	return 1 - math.Exp(-c.water/weatherRunoff)
}

// toward is v after years heading for level at rate: the exact step of
// dv/dt = rate·(level − v).
func toward(v, level, rate, years float64) float64 {
	if rate <= 0 {
		return v
	}
	return level + (v-level)*math.Exp(-rate*years)
}

// gathered is a stock that gains in a year and loses a share of itself to the
// water through the soil, after years from v: the exact step again, and plain
// accumulation where nothing goes through.
func gathered(v, gain, water, solute, years float64) float64 {
	k := water / solute
	if k*years < 1e-9 {
		return v + gain*years
	}
	return toward(v, gain/k, k, years)
}

// ripenSoil moves tile i's soil state on by years under the climate and cover
// it has now: the surface a little older, and the leaching, the carbonate, the
// salt and the carbon each a step nearer what they would come to.
func (g *Grid) ripenSoil(i int, years float64) {
	t := &g.Tiles[i]
	if !forms(t) {
		clearSoil(t)
		if t.Terrain == Pan {
			t.setSalinity(math.Inf(1))
		}
		return
	}
	c := g.pedoClimateOf(i)
	cv := g.coverOf(i, c.temp)
	age := float64(t.Exposed) + years
	// Order matters a little: the carbonate is what holds the bases, so it is
	// moved first.
	t.setCarbonate(gathered(t.Carbonate(), limeRate*dryness(c.wetness, limeWetter, limeDrier), c.water, limeWater, years))
	t.setSalinity(gathered(t.Salinity(), saltRate*dryness(c.wetness, saltWetter, saltDrier), c.water, saltWater, years))
	level, rate := g.leachLevel(i, c, cv, age)
	t.setLeaching(toward(t.Leaching(), level, rate, years))
	level, rate = g.carbonLevel(i, c, cv)
	t.Carbon = float32(toward(float64(t.Carbon), level, rate, years))
	t.Exposed = float32(age)
}

// drownSoils clears the soil state of every tile that does not form soil: the
// ground the rivers and the tide have moved onto since the weather read it.
func (g *Grid) drownSoils() {
	if !g.pedons {
		return
	}
	for i := range g.Tiles {
		if t := &g.Tiles[i]; !forms(t) {
			clearSoil(t)
			if t.Terrain == Pan {
				t.setSalinity(math.Inf(1))
			}
		}
	}
}

// clearSoil starts a tile's soil state again: bare ground, or no ground.
func clearSoil(t *Tile) {
	t.Exposed, t.Leached, t.Lime, t.Salt, t.Carbon = 0, 0, 0, 0, 0
}

// buryIn works d metres of fresh ground into the held metres of soil on t, as
// mix does the grains: the surface is younger by the share of it that is new,
// and so are the bases. The carbonate, the salt and the carbon are stocks
// over the square metre and stay what they were.
func buryIn(t *Tile, held, d float64) {
	if d <= 0 {
		return
	}
	keep := math.Max(0, held) / (math.Max(0, held) + d)
	t.Exposed = float32(float64(t.Exposed) * keep)
	t.setLeaching(t.Leaching() * keep)
}

// strip takes the share gone of t's soil off it, from above: the stocks it
// held go with it, and if the whole of it went the surface is fresh rock.
func strip(t *Tile, gone float64) {
	if gone >= 1 {
		clearSoil(t)
		return
	}
	if gone <= 0 {
		return
	}
	keep := 1 - gone
	t.setCarbonate(t.Carbonate() * keep)
	t.setSalinity(t.Salinity() * keep)
	t.Carbon = float32(float64(t.Carbon) * keep)
}

// exposure is how long tile i's surface is taken to have been forming soil
// when the map is made, h metres of it lowering at pace metres a year. made is
// the history's bound on it, or +Inf where there was no history: see
// deepExposure.
func (g *Grid) exposure(i int, h, pace, made float64) float64 {
	t := &g.Tiles[i]
	if h <= 0 || !forms(t) {
		return 0
	}
	e := math.Max(pace, soilDeepest)
	age := h/e + math.Min(made, regolithDepth/e)
	if t.Drain < FloodDepth {
		age = math.Min(age, riverYoung+riverYears*math.Max(0, t.Drain)/FloodDepth)
	}
	return age
}

// laySoilState sets tile i's soil state as the years it has been exposed
// would have left it under the climate and cover it has now, h metres of soil
// lowering at pace. It is ripenSoil from bare rock taken over the whole age in
// one step, with the minerals read at the end of it; so a surface ten
// thousand years old is what ten thousand years make, and one a million years
// old is where the climate would hold it.
func (g *Grid) laySoilState(i int, h, pace, made float64) {
	t := &g.Tiles[i]
	age := g.exposure(i, h, pace, made)
	clearSoil(t)
	if !forms(t) {
		if t.Terrain == Pan {
			t.setSalinity(math.Inf(1))
		}
		return
	}
	// ripenSoil adds the years to Exposed, so it starts from nothing.
	g.ripenSoil(i, age)
}

// deepExposure moves a history's surface ages on by an epoch of years: ground
// under the sea starts again, and ground above it ages by the epoch, but is
// never older than the time its lowering, the fall in its height the epoch's
// weather made, takes to go through regolithDepth. So a range coming down a
// millimetre a year is two thousand years old however long it has stood, and
// a shield wearing at a few metres in a million years is as old as the
// history. What the epoch buried and floored anew is started again after the
// epoch's book is kept: see restartBuried.
func (g *Grid) deepExposure(i int, lowered, years float64) {
	t := &g.Tiles[i]
	if t.Wet() || t.Height <= g.base {
		t.Exposed = 0
		return
	}
	age := float64(t.Exposed) + years
	if lowered > 0 {
		age = math.Min(age, regolithDepth*years/lowered)
	}
	t.Exposed = float32(age)
}

// restartBuried starts again the surface age of every tile whose rock this
// epoch made: a bed of lava over it, a basin's fill, the sea's mud. See
// history.go.
//
// Not at nothing. The epoch is four million years and nothing says when in it
// the lava came; the surface is on average half an epoch old at its end. And a
// basin's fill does not bury its ground once but all the epoch long, at
// fillRate, so what stands at the top of it has been there for as long as the
// fill takes to lay regolithDepth over it: twenty thousand years. Read as
// nothing, the first small globe had 55 in a hundred of its soils under a
// thousand years old, and read this way 39; nearly all of the rest was ground
// the history last saw under its sea, which laySoil leaves to the ground's
// own reading, and with it they are 1.6 in a hundred.
func (g *Grid) restartBuried(epoch int) {
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if int(t.Formed) != epoch {
			continue
		}
		age := math.Min(float64(t.Exposed), epochYears/2)
		if !t.Wet() && t.Height > g.base && t.Drain < FloodDepth/2 {
			age = math.Min(age, regolithDepth/fillRate) // the fill: see keepBook
		}
		t.Exposed = float32(age)
	}
}

// soilChemistry is what the soil's chemistry does to what a tile will grow,
// against the map's middling soil at one. It is gentle on purpose: the depth,
// the water and the mixture are most of what a field is, and the chemistry is
// what tells two fields that are alike in those apart.
//
// The bases: a soil the rain has stripped of them is sour, holds nothing a
// crop is fed, and locks its phosphorus up with the iron and aluminium left
// behind (Walker and Syers 1976, the phosphorus of a chronosequence falling
// and going into forms no root can take). A leached soil grows a quarter less
// than one that has kept everything, read against leachMiddle.
//
// The salt: crops lose yield steadily past a threshold of salt in the soil
// water (Maas and Hoffman 1977), and a salt crust grows nothing; halved at
// saltHarm kilograms a square metre. The carbonate: a little is a sweet soil,
// and a great deal is a cemented petrocalcic pan the roots do not get through,
// which costs up to a sixth.
//
// What it did to the ground a settlement is handed, with the carbon in humus,
// on the default valley over seeds one to five (see TestSoilReadings): the
// mean fertility of the dry ground, the riverbank's and inland's, the valley
// floor's (Drain under two metres) and the hillside's (over twenty), and the
// share of the ground at 0.6 or better.
//
//	                        mean     bank    inland   floor    hill    good
//	fresh, before          0.2031   0.2651   0.1938   0.9228  0.1794  0.0217
//	fresh, after           0.2000   0.2634   0.1905   0.9254  0.1767  0.0212
//	twenty ages, before    0.1998   0.2457   0.1925   0.8010  0.1789  0.0195
//	twenty ages, after     0.1966   0.2436   0.1892   0.8003  0.1762  0.0188
//
// A sixtieth off the mean, from the old leached soils of the hollows and
// shoulders; the floors, which the rivers keep young, hold what they had. On
// the made valleys of seeds one and two the mean went from 0.2018 and 0.2383
// to 0.1988 and 0.2351, and on the small globes from 0.1718 and 0.1669 to
// 0.1712 and 0.1661.
func (t *Tile) soilChemistry() float64 {
	bases := 1 + baseWeight*(leachMiddle-t.Leaching())
	salt := 1 / (1 + t.Salinity()/saltHarm)
	pan := 1 - 0.15*ramp(t.Carbonate(), 50, 300)
	return bases * salt * pan
}

const (
	baseWeight = 0.25
	saltHarm   = 10.0 // kg/m²
	// leachMiddle is the leaching the chemistry is read against: the middle of
	// the default valley's soils, which is 0.154 on the first seed and 0.138
	// on its made twin. The mean is higher, 0.30, pulled up by the old deep
	// soils of the hollows; read against the middle, those are what the
	// chemistry costs.
	leachMiddle = 0.15
	// carbonWeight is how far the carbon a tile holds, against what open grass
	// would hold under the same sky, moves what it grows: a field ploughed
	// down to two fifths of the grass's carbon grows a sixth less.
	carbonWeight = 0.3
)

// humus is the organic matter of tile i's soil against the middling climate's
// at one, as organic reads it, with the carbon the tile actually holds in it:
// the climate's reading, moved by how much of the carbon open grass would
// hold under that climate the tile holds.
func (g *Grid) humus(i int) float64 {
	o := g.organic(i)
	if !g.pedons {
		return o
	}
	c := g.pedoClimateOf(i)
	grass, _ := g.carbonLevel(i, c, grassCover)
	if grass <= 0 {
		return o
	}
	share := math.Min(2, float64(g.Tiles[i].Carbon)/grass)
	return math.Max(0.5, math.Min(1.5, o*(1+carbonWeight*(share-1))))
}
