package veg

import "math"

// Fire, and what else takes a tile's plants before their time.
//
// Nothing burned, blew down or died of a drought, and where trees stood
// against grass was a line of dryness. Here a year has its fires, run the
// way SPITFIRE runs them (Thonicke and others, 2010), cut down to a year's
// mean:
//
//   - The fuel is the grass standing on the ground and the litter the
//     plants drop, which lies until it rots, the faster the warmer.
//   - It is dry enough to burn in a phase as far as the soil under a herb's
//     roots gives the air less than it could take up, and not under snow.
//     The standing grass carries a fire only as far as it has cured: a
//     fire through a green sward goes out, and the early rains' storms
//     burn the dry season's grass, not the summer's.
//   - The fires are lit by lightning, and the lightning comes with the
//     showers: a phase's flashes go with its convective rain.
//   - A fire runs as far as the wind drives it, through the hours of a
//     day's burning, and spreads across a country only where its fuel is
//     unbroken: grass carries a fire, a closed canopy of broadleaf trees
//     does not.
//
// What a fire kills goes by what it burns. A savanna's trees have the thick
// bark that sees a grass fire through and a rainforest's do not; a boreal
// forest's fires are crown fires that take the stand; a grass is burned to
// the ground and comes again from its roots. And a fire kills the young trees
// coming up through the grass before they are tall enough to stand it, which
// is the trap that holds a savanna open where its rain would grow a forest:
// the more grass, the more fire, and the more fire, the fewer trees to
// shade the grass out. Over a band of rain, a forest and a savanna are each
// a state that holds itself (Staver and others, 2011; Hirota and others,
// 2011), and which a place has is what its history left it.
//
// Two more take trees: a drought, which kills where the year's water falls
// under what a type would establish on (Allen and others, 2010), and the
// wind of the storms, as a mean rate the storms' climate gives (see
// Climate.Throw).

// Fire is what a tile's year does to what stands on it, apart from what
// each type makes of it: its fires and its storms.
type Fire struct {
	// Reach is the share of the ground the year's fires would burn were its
	// fuel full, unbroken and dead: over the phases, the fires lit on a
	// square kilometre times what each burns in that phase's dryness and
	// wind. Cured and Cured2 are the same sum with each fire's area times
	// the standing grass's curing factor, and times its square: what a fire
	// burns through a fuel bed of the grass alone, its rate of spread cut
	// by the curing, and the cross term of a bed of grass and dead fuel
	// together. See Burned.
	Reach, Cured, Cured2 float64
	// Litter is how many years what the plants drop lies before it rots.
	Litter float64
	// Throw is the share of a canopy the storms blow down in a year.
	Throw float64
}

// The kinds' fire traits: resist is the share of a type's cover a fire
// leaves standing, and flame how much its cover carries a fire across the
// ground. A savanna's trees have the bark to see a grass fire through, a
// rainforest's not: Hoffmann and others (2012) found savanna trees' bark
// some three times a forest tree's at the same size, and a first fire
// through an Amazonian forest kills two in five of its trees (Barlow and
// others, 2003). A boreal forest burns in crown fires that take the stand
// (Rogers and others, 2015). A grass burns to the ground and comes again from
// its roots. The flames are the fuels' as SPITFIRE weighs them: grass and
// the needleleaf trees' litter and crowns carry a fire, the broadleaf
// trees' leaves less, a rainforest's damp shade not at all.
var fireTraits = [PFTs]struct{ resist, flame float64 }{
	TropicalEvergreen:   {0.4, 0},
	TropicalRaingreen:   {0.9, 0.3},
	TemperateBroadleaf:  {0.6, 0.15},
	TemperateNeedleleaf: {0.7, 0.5},
	BorealNeedleleaf:    {0.15, 0.6},
	C3Grass:             {1, 1},
	C4Grass:             {1, 1},
	Shrub:               {0.5, 0.7},
	Tundra:              {0.8, 0.4},
}

// Survives is the share of type p's cover a fire through it leaves
// standing: see fireTraits.
func Survives(p PFT) float64 { return fireTraits[p].resist }

// The lightning. A phase's convective rain is the share of its rain that
// falls as showers out of towering cloud, which is most of a hot country's
// rain and little of a cold one's: convMost of it at convHot degrees and
// over, none at convCold and under, a smooth step between (Dai, 2001, finds
// the showers seven tenths and more of the tropics' rain). The flashes go
// with it: flashPerMM flashes a square kilometre for each mm, which puts a
// savanna's 800 mm of showers at the twenty to forty flashes a square
// kilometre a year the satellites count over the tropics' dry land (Cecil
// and others, 2014); Romps and others (2014) found the flashes to go with
// the rain and the air's instability together. Of the flashes, groundShare
// strike the
// ground (Prentice and Mackerras, 1977), and lightShare of those light a
// fire, SPITFIRE's share.
const (
	convMost    = 0.8
	convCold    = 5.0
	convHot     = 27.0
	flashPerMM  = 0.035
	groundShare = 0.2
	lightShare  = 0.04
)

// edge is the share of a phase's lightning that falls in the weeks it
// shares with each of its neighbours: the half month at each end of its
// three, on whichever side's fuel lies there.
const edge = 1.0 / 6

// yearsWater is the years a phase's water swings through, as shares of its
// mean: the fifths of a normal spread whose deviation is a third of the
// mean, the swing of a season's rain from one year to the next over most of
// the land (Fatichi and others, 2012). A country's fires come in its dry
// years, and a boreal forest's in the summer of a drought.
var yearsWater = [...]float64{1 - 1.28*yearsSwing, 1 - 0.52*yearsSwing, 1, 1 + 0.52*yearsSwing, 1 + 1.28*yearsSwing}

const yearsSwing = 1.0 / 3

// The spread. A fire runs at rosCalm plus rosWind times the wind, in metres
// a second, and burns an ellipse ellipse times as long as it is wide:
// π/(4·ellipse) times the square of how far it ran (Thonicke and others,
// 2010). It runs as long as the fuel's dryness, d, lets it: burnMinutes /
// (1 + 240·e^(-11.06·d)), SPITFIRE's duration, which is near its four hours
// in a dry season and minutes in a damp one. In a moderate wind of five
// metres a second that is three tenths of a metre a second, a savanna's
// grass fire as Rothermel's model runs it in SPITFIRE, and some seven km²
// burned through a dry season's afternoon. The rate is the one figure here
// set against the burned area itself: at twice it, the savannas burned half
// their ground a year and the land five times GFED's.
const (
	rosCalm     = 0.05
	rosWind     = 0.05
	burnMinutes = 241.0
	ellipse     = 2.0
)

// The fuel. Under fuelLeast kg of carbon a square metre a fire does not
// carry, and from fuelFull it carries as far as it can: SPITFIRE's 200 g of
// fuel a square metre, half of it carbon, and twice that. Litter rots in
// litterYears at ten degrees, faster in the warm by the Q10 of two, which is
// a year or so in the tropics and several in the boreal forest (Zhang and
// others, 2008). Of a grass's carbon, HerbAbove stands above the ground as
// fuel, and burnHerb of it goes with a fire through it; of a shrub's, its
// leaves and twigs, burnShrub.
const (
	fuelLeast   = 0.1
	fuelFull    = 0.4
	litterYears = 2.0
	burnHerb    = 0.4
	burnShrub   = 0.3
)

// HerbAbove is the share of a grass's carbon above the ground, which is the
// fuel a grass fire runs through: a grassland's roots hold some two thirds
// of its carbon (Mokany and others, 2006, whose root to shoot ratios run
// about two under grass, and more under the temperate grasslands). It was
// LPJ's grass leaves to its roots, half, while the carbon's pools read
// Mokany's third (carbon.go); the fuel the fires run through and the carbon
// they send to the air now read the same grass.
const HerbAbove = 1.0 / 3

// The fuel's continuity. Where the cover that carries a fire, each type's
// weighed by its flame, is under contLeast of the ground, a fire goes out
// where it is lit, and from contFull it runs unbroken: the burned area of
// Africa's fires falls away as the trees close over forty per cent of the
// ground (Archibald and others, 2009, 2010).
const (
	contLeast = 0.4
	contFull  = 0.75
)

// trap is how hard the fires hold the trees' young down: the trees' gain in
// cover is cut by e^(-trap·burned), the share of their saplings that grows
// out of the flames' reach between fires. A savanna's saplings, burned back
// to the ground every few years, take decades to escape (Hoffmann and
// others, 2009; Bond, 2008). It is set where a wet savanna's year holds
// either state (see the tests), its savanna's trees on a third of the
// ground: three while a fire ran through green grass as through cured, and
// four and a half since the grass's curing cuts the early rains' fires
// (see curing). At twice it a savanna held barely a tree.
const trap = 4.5

// droughtMost is the share of its cover a woody type loses in a year whose
// water is at the least it survives on, and nothing where the water is what
// it establishes on: a drought kills a tree a few times faster than its age
// does (Allen and others, 2010).
const droughtMost = 0.05

// showers is the share of rain that falls as showers at a phase's mean, t.
func showers(t float64) float64 {
	x := math.Max(0, math.Min(1, (t-convCold)/(convHot-convCold)))
	return convMost * x * x * (3 - 2*x)
}

// Flashes is the lightning, flashes a square kilometre, of a phase whose rain
// is rain mm at a mean of t degrees.
func Flashes(rain, t float64) float64 { return flashPerMM * showers(t) * rain }

// spread is the area, in km², a fire burns in a wind of wind m/s where the
// fuel's dryness is dry.
func spread(wind, dry float64) float64 {
	minutes := burnMinutes / (1 + 240*math.Exp(-11.06*dry))
	run := (rosCalm + rosWind*math.Max(0, wind)) * minutes * 60 / 1000 // km
	return math.Pi / (4 * ellipse) * run * run
}

// Fire is the year's fires and storms.
func (y *Year) Fire() Fire {
	c := &y.c
	f := Fire{Throw: c.Throw, Litter: litterYears / q10(c.Mean)}
	var flashes [Phases]float64
	for k := range Phases {
		flashes[k] = Flashes(c.Rain[k], y.phase[k])
	}
	for k := range Phases {
		if y.phase[k] <= 0 {
			continue
		}
		// A phase is three months, and the storms at either end of it are
		// its neighbours' as much as its own: the first storms of a wet
		// season fall on the dry season's grass.
		lit := ((1-2*edge)*flashes[k] + edge*(flashes[(k+1)%Phases]+flashes[(k+Phases-1)%Phases])) * groundShare * lightShare
		// The fuel's dryness, SPITFIRE's fire danger: the soil under a
		// herb's roots giving the air less than it could take, and no snow
		// lying on it, read over the years a phase's water swings through.
		// As many of the fires lit take as the dryness says, and they run as
		// long as it lets them.
		for _, m := range yearsWater {
			w := math.Min(1, m*c.Herb[k])
			dry := (1 - w) * (1 - c.Snow[k])
			r := lit * dry * spread(c.Wind[k], dry) / float64(len(yearsWater))
			cure := curing(w)
			f.Reach += r
			f.Cured += r * cure
			f.Cured2 += r * cure * cure
		}
	}
	return f
}

// Burn adds to the year's fires the ones people light, burning the ground
// every so many years: the reach that would burn full, unbroken fuel that
// often, through cured grass, as a dormant season's fires burn it (a
// prairie's are lit before the grass greens; Knapp and others, 1998). It is
// on top of the lightning's, and what it burns goes by the state's own fuel
// and its continuity like any fire (see Burned). Every year or more often
// is read as 0.99 of the ground a year; every of zero or less is no fire.
func (f *Fire) Burn(every float64) {
	if every <= 0 {
		return
	}
	share := math.Min(0.99, 1/every)
	r := -math.Log(1 - share)
	f.Reach += r
	f.Cured += r
	f.Cured2 += r
}

// Burned is the share of the ground the year's fires burn under a state:
// 1 - e^(-reach·fuel·continuity), the fires falling where they fall.
//
// The reach is the fuel bed's: a fire runs through it at the mean of its
// fuels' rates of spread, each weighed by its load, as SPITFIRE weighs a
// fuel bed's properties (Thonicke and others, 2010). The dead fuel - the
// litter and the shrubs' twigs - runs at the rate Fire reads; the standing
// grass at that rate cut by its curing (see curing), so that the area a
// fire burns, which goes as the square of its run, goes as the square of
// the bed's weighed rate: (d²·Reach + 2dg·Cured + g²·Cured2)/(d+g)², d and
// g the dead fuel and the grass.
func Burned(s *State, pot *[PFTs]Potential, f *Fire) float64 {
	if f.Reach <= reachLeast {
		return 0
	}
	var dead, grass, carry float64
	for p := range PFTs {
		k := &Kinds[p]
		carry += s.Cover[p] * fireTraits[p].flame
		// What it drops a year, as its leaves turn over, and lies.
		dead += s.Cover[p] * pot[p].LAI * k.leafTurn / 1000 * f.Litter
		switch {
		case !k.Woody:
			grass += HerbAbove * s.Mass[p]
		case !k.Tree:
			dead += burnShrub * s.Mass[p]
		}
	}
	fuel := dead + grass
	if fuel <= 0 {
		return 0
	}
	reach := (dead*dead*f.Reach + 2*dead*grass*f.Cured + grass*grass*f.Cured2) / (fuel * fuel)
	return 1 - math.Exp(-reach*smooth(fuelLeast, fuelFull, fuel)*smooth(contLeast, contFull, carry))
}

// curing is how much of its rate of spread a grass fire keeps where the
// herbs' soil gives the air w of what it could take. A grass holds the
// water its roots have: SPITFIRE gives the live grass a moisture of
// 10/9·w - 1/9 of what it holds wet (Thonicke and others, 2010), read here
// as the share of the grass still green, and the rest cured. A grassfire
// spreads through green grass barely at all until half or so of it has
// cured, and at its full rate once all of it has: Cruz and others' (2015)
// curing coefficient, 1.036 / (1 + 103.99·e^(-0.0996·(C - 20))), C the
// per cent cured, fitted to their field fires.
func curing(w float64) float64 {
	green := math.Max(0, math.Min(1, (10*w-1)/9))
	c := 100 * (1 - green)
	return math.Min(1, 1.036/(1+103.99*math.Exp(-0.0996*(c-20))))
}

// reachLeast is the reach under which a year's fires are read as none: a
// fire a hundred thousand years.
const reachLeast = 1e-5

// smooth is 0 below lo, 1 above hi, and a smooth step between.
func smooth(lo, hi, x float64) float64 {
	t := max(0, min(1, (x-lo)/(hi-lo)))
	return t * t * (3 - 2*t)
}
