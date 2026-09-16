package tile

// megapascal is a million pascals: the unit rock strength is read in.
const megapascal = 1e6

// Bedrock is the rock a tile's soil is weathering out of. Four kinds is not
// geology; it is the fewest that give a map somewhere sandy, somewhere heavy,
// somewhere sweet and somewhere sour, which is what the ground needs to stop
// being uniform.
type Bedrock uint8

const (
	// Granite is the hard, sour rock of the high country. It weathers to a
	// coarse gritty soil - grus, what is left when the crystals let go of
	// each other - which drains freely and is nobody's best field.
	Granite Bedrock = iota
	// Limestone dissolves rather than crumbles, so what it leaves behind is
	// the little that would not dissolve: a fine, sticky residue over rock
	// that the water runs away through.
	Limestone
	// Sandstone is sand that was buried and is now sand again.
	Sandstone
	// Shale is mud that was buried and is now mud again: the heaviest ground
	// on the map, and the ground a plough dreads in a wet spring.
	Shale
	// Basalt is what comes up: the floor a world cools into, what fills a
	// rift, and what a volcano leaves. It weathers to a dark soil that holds
	// water and what is dissolved in it, which is why people farm the flanks
	// of volcanoes knowing exactly what they are.
	Basalt
	// Schist is rock that was something else and was then buried, cooked and
	// squeezed by a collision. It is the rock of an old mountain range, and
	// finding it is finding where two plates met.
	Schist
	// BedrockCount is how many kinds there are, for the tables that have to
	// carry a row for each.
	BedrockCount
)

// tensile is each rock's tensile strength, in megapascals. It is what makes a
// landscape have a shape at all: where the rocks differ, the soft ones go and
// the hard ones are left standing, so a scarp is a hard bed with a soft one
// under it and a gorge is a river that found something it could cut.
//
// Tensile strength and not a ranking, because it is the one figure a river's
// wear has been measured against. Sklar and Dietrich (2001) wore discs of
// twenty-odd rocks under saltating gravel in a flume and found the wear going
// as the inverse square of the rock's tensile strength, over more than two
// orders of magnitude of it - from mudstones under a megapascal to quartzites
// over ten. The figures here are middling values of the lithologies in their
// range and the rock-mechanics tables it was drawn from: basalt and granite
// the strongest, schist split along its foliation, limestone and sandstone
// cemented well or badly, and shale barely rock at all.
//
// It was a hardness a quarryman would have ranked, 1.5 for granite down to
// 0.45 for shale.
var tensile = [BedrockCount]float64{
	Basalt:    10 * megapascal,
	Granite:   7 * megapascal,
	Schist:    5 * megapascal,
	Limestone: 4.5 * megapascal,
	Sandstone: 3.5 * megapascal,
	Shale:     2 * megapascal,
}

// Chemistry is what a rock gives the soil over it to live on, as shares of
// the rock's mass. It is read and not yet used: a soil made of a rock is still
// only its sand and its clay - see weathers - and this is what the soil will
// ask of it when it is also sour or sweet, rich or starved.
//
// Quartz is the mineral share that does not weather at all: what a soil keeps
// as sand for ever, and what makes the soil on a rock poor however long it
// has. Bases is the calcium, magnesium, potassium and sodium, counted as their
// oxides the way a rock is analysed - CaO, MgO, K2O and Na2O - which is what
// the weather frees to hold a soil's acidity back and to feed what grows in
// it. Carbonate is the share that dissolves outright rather than weathering to
// clay, and Phosphorus the element itself, not its oxide: the one nutrient a
// soil cannot take from the air and has only from its rock.
type Chemistry struct {
	Quartz, Bases, Carbonate, Phosphorus float64
}

// chemistry is each rock's, from averages of many analyses rather than any one
// rock. The oxides of granite and basalt are Le Maitre's (1976) averages of
// the analyses of each he gathered: 1.84 per cent CaO, 0.71 MgO, 3.68 Na2O,
// 4.07 K2O and 0.12 P2O5 in granite, and 9.47, 6.73, 2.91, 1.10 and 0.35 in
// basalt. Those of limestone, sandstone and shale are
// Clarke's (1924) averages as Pettijohn (Sedimentary Rocks, 1975) tabulates
// them, with their carbonate read off the CO2 - 41.5, 5.0 and 2.6 per cent -
// as calcite, and the magnesia of the limestone as dolomite: limestone 42.6
// CaO, 7.9 MgO, 0.33 K2O, 0.05 Na2O and 0.04 P2O5; sandstone 5.5, 1.2, 1.3,
// 0.45 and 0.08, most of its lime the cement between the grains; shale 3.1,
// 2.4, 3.2, 1.3 and 0.17. Phosphorus is 0.436 of the P2O5.
//
// The quartz is a mineral share, which an analysis does not give. Granite's is
// the third it typically has, inside the fifth to three fifths that the rock's
// definition allows it (Streckeisen 1976); basalt has none; shale's is Shaw
// and Weaver's (1965) average of 31 per cent, with 3.6 per cent carbonate,
// which is taken over Clarke's for the minerals; sandstone's the two thirds of
// an average sandstone, and limestone's the few per cent of its silica that is
// quartz and chert.
//
// Schist is shale cooked. A pelitic schist has the analysis of the mudrock it
// was, less its water and carbon dioxide (Shaw 1956), so its row is shale's
// with the carbonate driven off - the lime stays, in the silicates it has made
// - and a little more quartz for the granite and sandstone the crushing takes
// in with the mud.
var chemistry = [BedrockCount]Chemistry{
	Granite:   {Quartz: 0.30, Bases: 0.103, Carbonate: 0, Phosphorus: 0.00052},
	Limestone: {Quartz: 0.04, Bases: 0.509, Carbonate: 0.91, Phosphorus: 0.00017},
	Sandstone: {Quartz: 0.65, Bases: 0.085, Carbonate: 0.11, Phosphorus: 0.00035},
	Shale:     {Quartz: 0.31, Bases: 0.100, Carbonate: 0.036, Phosphorus: 0.00074},
	Basalt:    {Quartz: 0, Bases: 0.202, Carbonate: 0, Phosphorus: 0.00153},
	Schist:    {Quartz: 0.35, Bases: 0.095, Carbonate: 0.005, Phosphorus: 0.00070},
}

// Chemistry is what this rock gives a soil: see Chemistry.
func (b Bedrock) Chemistry() Chemistry {
	if b >= BedrockCount {
		return Chemistry{}
	}
	return chemistry[b]
}

// tensileHard is the tensile strength a rock of hardness one has: what
// hardness is read against. At five megapascals a drawn map's four rocks come
// out at 1.4, 0.9, 0.7 and 0.4, near the 1.5, 0.65, 0.8 and 0.45 they were
// ranked at, so the slopes the rock holds up - see stand - and the beds a
// history lays keep the contrasts they had.
const tensileHard = 5 * megapascal

// hardness is each rock's tensile strength against tensileHard. Everything
// that asks how hard the rock is asks it against the map's middling rock - see
// meanHard - so it is the contrasts that count and not the scale.
var hardness = func() (h [BedrockCount]float64) {
	for b := range h {
		h[b] = tensile[b] / tensileHard
	}
	return h
}()

// Tensile is this rock's tensile strength, in pascals.
func (b Bedrock) Tensile() float64 { return tensile[b] }

// Hardness is how well this rock stands up to being worn away, against a
// rock of tensileHard: see Tile.Hard.
func (b Bedrock) Hardness() float64 { return hardness[b] }

// String is what a rock is called.
func (b Bedrock) String() string {
	switch b {
	case Granite:
		return "granite"
	case Limestone:
		return "limestone"
	case Sandstone:
		return "sandstone"
	case Shale:
		return "shale"
	case Basalt:
		return "basalt"
	case Schist:
		return "schist"
	}
	return "rock"
}

// Bedrocks is every kind of rock, for the tables that have to cover them all
// and the tests that check they do.
func Bedrocks() []Bedrock {
	out := make([]Bedrock, 0, BedrockCount)
	for b := Bedrock(0); b < BedrockCount; b++ {
		out = append(out, b)
	}
	return out
}
