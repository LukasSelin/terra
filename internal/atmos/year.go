package atmos

import "math"

// The shape of the year. MeanTemp is the annual mean in degrees, Swing half
// the distance from midwinter to midsummer. Tick zero is early spring:
// people arrive with the growing season ahead of them, not behind them.
const (
	MeanTemp = 10.0
	Swing    = 12.0
)

// The globe's weather. Temperate is the latitude the default map's weather
// is the weather of. A globe's year is the energy balance's, by latitude: see
// ebm.go.
const Temperate = 45.0

// The lapse rate: how much colder the air is for standing higher up.
//
// The weather is one temperature for a latitude, and it was one temperature
// for a latitude at every height, which made the top of a mountain exactly as
// warm as the valley it stands over. Height was the one thing the ground
// carried that the weather never read - see Tile.Height, off which the
// rivers, the soil and the going underfoot are all already read - so the high
// country was hard to live on for its slope alone, and a wood grew on a peak
// as readily as on the valley floor.
//
// Lapse is the real figure, six and a half degrees a kilometre, and it is
// deliberately not tuned. What it is worth depends on what a map has standing
// on it, and that follows from the map's own size: on the default valley the
// skyline is Relief plus Upland, some three hundred metres, so the highest
// ground on it is two degrees colder than the river and no more - which is
// why nothing measured on the valley moves much, and why the want of this was
// never felt there. On a globe the same rule over mountains ten times as high
// is the difference between a tree line and no tree line.
const Lapse = 0.0065

// The shape of the year at a place. The weather of a latitude is a mean with a
// swing round it, and the swing is not the same everywhere: it is nothing at
// the equator, where the sun stands as high in December as in June, and grows
// toward the poles, where the difference between the two is the difference
// between a day without a night and a night without a day. And it is larger
// inland than at sea, because water is a store of heat and rock is not.
//
// Everything that reads the year - the temperature of a tile on a day, the air
// the wind is worked out over, the water the air could take up, the lines
// where the ground stays frozen and where no tree will stand - reads it here,
// so that they agree about what summer is.

// solarSwing is how much of the temperate latitudes' swing the sun alone
// gives the year at lat, signed by hemisphere. The sun's reach at the top of
// the air swings over the year by very nearly the sine of the latitude (it is
// the declination's cosine term in the daily insolation; see Hartmann, Global
// Physical Climatology, 2016, ch. 2), so the swing is quoted against its value
// at Temperate. The wind still reads the year this way - see seasonTemp - and
// the ground reads the energy balance's: see swingAt.
func solarSwing(lat float64) float64 {
	return math.Sin(math.Abs(lat)*math.Pi/180) / math.Sin(Temperate*math.Pi/180) * math.Copysign(1, lat)
}

// SwingAt is half the distance from the coldest day of the year to the warmest
// at latitude lat, on ground cont of whose country round about is land. It is
// the energy balance's year (see ebm.go): the swing of the balance's sea at
// that latitude where the country is all water, its land's where it is all
// land, and between in proportion. It is signed by hemisphere, as solarSwing
// is.
//
// It used to be the sun's swing times a share written down for each - a third
// of it over the open sea and one and three fifths deep in a continent. The
// balance's land, holding the heat of its air and half a metre of ground, and
// its sea, holding fifty metres of water, give nineteen degrees and under three
// at forty-five against the old nineteen and four, and are what they are for a
// reason.
func SwingAt(lat, cont float64) float64 {
	e := ebm()
	sea, land := e.at(&e.swingS, lat), e.at(&e.swingL, lat)
	return math.Copysign(sea+(land-sea)*clamp01(cont), lat)
}

// seasonTemp is what the year adds to the mean at a place whose sun has solar
// of the temperate swing, phase of the way from its mean to its crest, on
// ground cont continental: the swing the wind and the storms read. See
// swingSea.
func seasonTemp(solar, phase, cont float64) float64 {
	return solar * Swing * phase * (swingSea + (swingLand-swingSea)*cont)
}

// ContMiddling is the continentality at which a place keeps exactly the
// temperate swing: the ground a latitude's weather is the weather of when
// nothing is known about the ground. terra.Climate.TempAt's year is read at it.
var ContMiddling = middlingOf(ebm())

// ContValley is the same for a valley, which has no ground round it to be
// continental or not and reads its day's range at it: see rangeCont. It is
// the balance's at the land and the sea's old exchange, valleyExchange,
// because a valley's rivers and lakes were tuned on the day's range that
// gave, and the stronger exchange a globe's winters want is a fact about
// continents a valley does not have.
var ContValley = middlingOf(solveEBMWith(ebmParams{ebmDiffusion, albedoA0, albedoA2, heatLand, heatSea, valleyExchange}))

// valleyExchange is landSeaExchange as it stood when the valley was tuned.
const valleyExchange = 3.0

// middlingOf is the continentality at which balance e keeps the temperate
// swing at Temperate.
func middlingOf(e *ebmClimate) float64 {
	sea, land := e.at(&e.swingS, Temperate), e.at(&e.swingL, Temperate)
	return clamp01((Swing - sea) / (land - sea))
}

// The lag of the seasons. Ground heated by a sun that swings sinusoidally warms
// with the same period and lags it: for a surface of heat capacity C losing
// heat at B per degree, C dT/dt = F sin ωt - B T gives a lag of atan(ωC/B)/ω
// (North and Coakley, 1979). The lags are the energy balance's at Temperate,
// where its land's year peaks a month after the solstice and its sea's nearly
// three - the real lag is a month over the continents and two to three over
// the open ocean - and a place partly both has a heat capacity partly each.
var (
	lagLand = ebm().at(&ebm().lagL, Temperate) // days, in a year of 365.25
	lagSea  = ebm().at(&ebm().lagS, Temperate)
)

var (
	yearOmega = 2 * math.Pi / 365.25
	tauLand   = math.Tan(yearOmega*lagLand) / yearOmega
	tauSea    = math.Tan(yearOmega*lagSea) / yearOmega
)

// LagAt is how many days of the calendar the warmest day falls after the
// sun's highest on ground cont of whose country round about is land.
func LagAt(cont float64) float64 {
	cont = clamp01(cont)
	tau := tauSea + (tauLand-tauSea)*cont
	return math.Atan(yearOmega*tau) / yearOmega * Year / 365.25
}

// SeasonAt is how far into its swing the year is on day tick at ground whose
// seasons lag by lag days: -1 at the depth of the northern winter and 1 at the
// height of its summer. The sun is highest in the north a quarter of the way
// into the calendar, where yearSin is one.
func SeasonAt(tick int, lag float64) float64 {
	return math.Sin(2 * math.Pi * (float64(tick) - lag) / Year)
}

// MonthPeak is what the warmest month of a sinusoidal year stands over its
// mean, as a share of the swing: the mean of a sine over the twelfth of a
// period round its crest, sin(π/12)/(π/12). Köppen's and every other
// classification's thresholds are thresholds on monthly means.
var MonthPeak = math.Sin(math.Pi/12) / (math.Pi / 12)

// SummerPeak is the same for the warmest quarter, which is what a glacier's
// melt answers to: sin(π/4)/(π/4).
var SummerPeak = math.Sin(math.Pi/4) / (math.Pi / 4)

// aboveMean is the year's mean of max(0, x + a sin θ): the degrees a day over
// a base, averaged over a year that swings a either side of x above it. It is
// what growing degree-days are, a day at a time, and it is exact for a sine.
func aboveMean(x, a float64) float64 {
	a = math.Abs(a)
	if a <= math.Abs(x) {
		return math.Max(0, x)
	}
	c := -x / a
	s := math.Asin(c)
	return (x*(math.Pi-2*s) + 2*a*math.Sqrt(1-c*c)) / (2 * math.Pi)
}

// The frozen ground. Permafrost is ground that stays at or below freezing for
// two years running, and the line it keeps to is not the frost that stops a
// plant growing - see Frost - but a colder one.
//
// Nelson and Outcalt (1987) read it off the frost index F = √DDF/(√DDF+√DDT),
// the freezing and thawing degree-days of the year, with permafrost where F is
// over a half. Read off the air, DDT-DDF is just the year's mean times its
// days, so F over a half is a mean under freezing whatever the swing; what
// keeps real permafrost further poleward than that is the snow, which blankets
// the ground in the winter and leaves the surface warmer than the air over it
// by a couple of degrees (Smith and Riseborough, 2002). So the index is taken
// over the surface, and the line it gives is the one the discontinuous
// permafrost is mapped to, a mean annual air temperature of some two degrees
// under freezing.
const (
	Permafrost    = -2.0
	surfaceOffset = -Permafrost // how much warmer the ground's surface stands than the air
)

// frostIndex is Nelson and Outcalt's F for a year of mean air temperature mean
// swinging swing either side of it, read at the ground's surface.
func frostIndex(mean, swing float64) float64 {
	t := mean + surfaceOffset
	ddt := aboveMean(t, swing)
	ddf := aboveMean(-t, swing)
	if ddt+ddf == 0 {
		return 0.5
	}
	return math.Sqrt(ddf) / (math.Sqrt(ddf) + math.Sqrt(ddt))
}

// How much of it. Permafrost has no edge. The maps of it (Brown and others,
// 1997; Obu and others, 2019) are maps of four zones, told apart by the share
// of the ground each holds: continuous over nine tenths, discontinuous a half
// to nine tenths, sporadic a tenth to a half, and isolated patches under a
// tenth. In central Siberia and northern Canada the walk from the first zone
// to the last is some hundreds of kilometres wide.
//
// What settles a given hectare inside that band is not its latitude. It is
// the snow that drifts over it - a windswept ridge freezes while the hollow
// beside it thaws - the peat and the moss on it, which way its slope faces,
// how it drains, and whether there is a lake on it, since a lake holds
// unfrozen ground under it in the middle of the continuous zone. None of that
// is on this map, and a tile is kilometres across in any case. What a tile
// can say is the share, which is what a zone is, and Nelson and Outcalt read
// the zones off the same index the line comes from: F over 0.67 is the
// continuous permafrost, and F over a half is the outer limit, the last of
// the isolated patches.
//
// So the share runs from none at the line to all of it a little past the
// continuous one, as a smoothstep. frostAll is not picked: it is set so that
// 0.67 comes out at the nine tenths the continuous zone is mapped at.
//
// It is a share of the ground within a tile and not a chance that the tile is
// frozen. A tile asked twice gives the same answer, and half a share is half
// the ground, which is what makes a fringe a fringe rather than a dice roll.
const (
	frostOut = 0.5  // the outer limit: the last of the isolated patches
	frostAll = 0.71 // all of it, set by the continuous zone's 0.67
)

// FrostShare is the share of the ground that is permafrost at a place whose
// year has the given mean and swing: none where the year is warmer than
// Permafrost, and one deep inside the continuous zone. It is zero exactly
// where the permafrost line is not crossed, so ground holding any share of it
// is the ground Grid.Frozen calls frozen and no other.
func FrostShare(mean, swing float64) float64 {
	if mean >= Permafrost {
		return 0
	}
	t := (frostIndex(mean, swing) - frostOut) / (frostAll - frostOut)
	switch {
	case t <= 0:
		return 0
	case t >= 1:
		return 1
	}
	return t * t * (3 - 2*t)
}

// The tree line. It is not set by the year's mean: a coast with a mild winter
// and a cool summer is treeless at a mean a continent grows forest at. It is
// set by the summer. Köppen drew the line at a warmest month of ten degrees,
// and Körner and Paulsen (2004), from root-zone loggers at the tree line on
// every continent, at a mean of 6.4 degrees over the growing season - the days
// whose mean is over 0.9. Both are asked, and ground either says is too cold
// holds no tree.
const (
	treeMonth  = 10.0
	treeSeason = 6.4
	seasonBase = 0.9
)

// seasonMean is the mean temperature over the days of the year warmer than
// seasonBase, and false where there are none.
func seasonMean(mean, swing float64) (float64, bool) {
	swing = math.Abs(swing)
	if swing == 0 {
		return mean, mean > seasonBase
	}
	c := (seasonBase - mean) / swing
	switch {
	case c >= 1:
		return 0, false
	case c <= -1:
		return mean, true
	}
	s := math.Asin(c)
	return mean + swing*2*math.Sqrt(1-c*c)/(math.Pi-2*s), true
}

// treeMean is the year's mean below which ground swinging swing either side of
// it has no tree line to be above: whichever of Köppen's and Körner's lines is
// the warmer.
func treeMean(swing float64) float64 {
	swing = math.Abs(swing)
	month := treeMonth - MonthPeak*swing
	// The season's mean rises with the year's, so the line is found by halving.
	lo, hi := -60.0, treeSeason
	for range 40 {
		mid := (lo + hi) / 2
		if m, ok := seasonMean(mid, swing); ok && m >= treeSeason {
			hi = mid
		} else {
			lo = mid
		}
	}
	return math.Max(month, hi)
}

// treeMeans is treeMean at every tenth of a degree of swing up to treeSwings,
// read by the tile.
const treeSwings = 60.0

var treeMeans = func() []float64 {
	out := make([]float64, int(treeSwings*10)+2)
	for k := range out {
		out[k] = treeMean(float64(k) / 10)
	}
	return out
}()

// TreeLineMean reads treeMeans at swing.
func TreeLineMean(swing float64) float64 {
	f := math.Min(math.Abs(swing)*10, float64(len(treeMeans)-2))
	k := int(f)
	return treeMeans[k] + (treeMeans[k+1]-treeMeans[k])*(f-float64(k))
}

// The ice. A glacier stands where more snow falls than the summer melts, and
// how warm a summer melts how much was fitted by Ohmura, Kasser and Funk
// (1992) at the equilibrium lines of seventy glaciers: P = 645 + 296T + 9T²,
// the year's precipitation in mm against the mean of the summer's three
// months. Ground whose summer is colder than that for the snow it gets keeps
// its snow, and grows nothing under it.
func IceSummer(rain float64) float64 {
	d := 296*296 - 4*9*(645-math.Max(0, rain))
	return (-296 + math.Sqrt(math.Max(0, d))) / 18
}

// Holdridge's biotemperature: the year's mean with every reading under
// freezing counted as freezing and every one over thirty as thirty, because
// plants do nothing more for being colder than frozen or hotter than hot
// (Holdridge, 1967). Under three degrees is his polar belt, which is tundra
// and ice.
const HoldridgePolar = 3.0

func Biotemperature(mean, swing float64) float64 {
	return aboveMean(mean, swing) - aboveMean(mean-30, swing)
}

// The Miami model (Lieth, 1975): what land grows in a year, in grams of dry
// matter a square metre, is the lesser of what its warmth allows and what its
// rain does. It is the climate's answer to how much grows, and what the rules
// of GrowthClimate share out over the warm days of the year.
func miamiNPP(mean, rain float64) float64 {
	warm := 3000 / (1 + math.Exp(1.315-0.119*mean))
	wet := 3000 * (1 - math.Exp(-0.000664*math.Max(0, rain)))
	return math.Min(warm, wet)
}

// growBase is the base of the growing degree-days, in degrees: the five that
// the temperate crops and most temperate trees begin growing at.
const growBase = 5.0

// nppRef is what the Miami model gives the temperate year the valley was tuned
// on - the mean of MeanTemp and a metre of rain - so that ground with that
// climate grows, over a year, what it always grew.
var nppRef = miamiNPP(MeanTemp, 1000)

// ClimateGrowth is how much green things grow on a day of temp degrees at a
// place whose year has the given mean, swing and rain: the day's share of the
// year's growing degree-days, times what the Miami model lets that year grow
// against the temperate one. Averaged over a year it is the place's NPP over
// the temperate NPP, so the valley's year still averages about one.
func ClimateGrowth(temp, mean, swing, rain float64) float64 {
	gdd := aboveMean(mean-growBase, swing)
	if gdd <= 0 {
		return 0
	}
	return math.Max(0, temp-growBase) / gdd * miamiNPP(mean, rain) / nppRef
}
