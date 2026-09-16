package terra

import (
	"github.com/LukasSelin/terra/geom"
	"math"
	"sort"
)

// A world made by what happened to it.
//
// The land used to be a picture: two fields of noise, one for the lie of the
// country and one for the ridges, with the high ground masked in where a
// third field said so. It makes handsome maps and it makes them all at once,
// out of nothing, in the shape somebody decided mountains ought to be.
// Nothing on such a map has a reason - a range is where it is because a
// lattice was high there, and the rock under it is what another lattice said.
//
// Here the land is the leavings of a history instead, run forward from a
// world too hot to have a surface worth the name. It is not geology and does
// not pretend to be: there is no mantle in it, no heat budget, no isostasy.
// What it has that a picture cannot have is causes - a range stands where two
// plates met, the rock in it is what that meeting made of what was there, the
// basin beside it is full of the range's own debris, and the good ground at
// the mouth of the valley is what the rivers carried out of both. Anything a
// later change wants to put on a map - where the ore is, where the coal is,
// where the ground still shakes - can then be asked of what happened rather
// than painted on afterwards.
//
// It runs behind Terms.Epochs. The globe is made by sixteen epochs of it,
// because a world that size has no picture that passes for it, and so is
// AncientTerms, which is the valley arrived at rather than composed; the
// default valley is still drawn. The constants the whole settlement model is
// tuned against - the sixty metres of lowland, the share of a map that can be
// ploughed - were measured on the picture, so a history has to be shown to
// hand a valley the same kind of map before it can be allowed to make the
// default one too. See normalise, which is where that join is.

// The three eras. Molten is a world with no rigid crust at all, where the
// surface is convection and nothing that forms outlasts the forming of it;
// then the crust goes rigid and breaks; and the rest is the plate era, which
// is all the history a map actually keeps.
//
// moltenChurns is how many times the molten world turns itself over. Each
// churn half forgets the one before, so the era leaves the residue of all of
// them and the shape of none, which is the honest way to draw a surface that
// had no time to become anything.
const (
	moltenChurns = 6
	moltenMemory = 0.5
	// How much of a lowland's worth of relief the churning leaves behind. It
	// is small on purpose: what survives the molten era is a floor with a
	// swell in it, not country. Left at a whole Relief the lumps were sixty
	// metres over five tiles, and normalise then stretched them into a map
	// twice as steep as a drawn one, which nothing downstream could plough
	// or walk.
	moltenRelief = 0.8 * Relief
)

// How many plates a world breaks into: five at the width the rest of the
// map's constants are quoted at, growing with the map, and never more than
// plateMost.
//
// The cap is what makes a continent a continent. Without it a globe broke into
// sixty-four, each some ninety tiles across, and a continent is one plate or
// two: so the land came out in pieces a person could walk across in a
// fortnight, with sea on both sides of everything. The median land tile stood
// seventeen tiles from salt water. There is no inland on a map like that, and
// so nowhere for an inland mountain to be - which is the whole of why every
// range on it was a coast range, quite apart from where the arcs were put.
//
// Sixteen, because that is about what a world has. The earth is seven large
// plates and half a dozen smaller ones, and what that number is really setting
// here is how wide a piece of continent gets to be before the sea interrupts
// it.
//
// oceanShare is how many of them are ocean floor - thin, dense, low, and the
// first to go down when two plates meet. More than half, because ocean floor
// is most of a world's surface, and it is the term deciding whether a map
// comes out a continent with seas in it or an ocean with islands in it.
const (
	plateCount = 5
	plateMost  = 32
	plateSpan  = DefaultWidth
	// oceanShare is how many of a world's plates are ocean floor, and it is
	// read off how much sea the finished map asks for. A world is mostly
	// ocean floor; a valley cut out of one and asking for no sea at all is a
	// piece of it that was never under any, so its plates should be the
	// continent they plainly are. Fixed at a world's own share, the default
	// map came out with seven tiles in ten standing on ocean floor and a
	// bedrock map that was mostly basalt - true of Iceland and of nowhere a
	// valley like this one is.
	oceanFloor  = 0.25
	oceanPerSea = 1.0
	// plateCap is the most plates a world may ever have had, rifts included.
	// It is what keeps the count inside the byte a tile stores its plate in,
	// with room above it for noPlate.
	plateCap = 120
	// beltReach is how far from a seam the ground is raised or dropped by
	// what is happening at it, in metres. A collision does not make a wall one
	// tile wide: it thickens a belt of country either side, which is why a
	// range has flanks and foothills and a pass through it. Applying the lift
	// only where two plates actually touch made a map of knife edges - the
	// steep ground doubled and there was nothing to walk up.
	//
	// It was ten tiles on every world, which was 250 metres of a field and
	// meant nothing of a planet. The lift falls to nothing at the reach, so
	// it stands at half its height halfway out: 550 kilometres puts that at
	// 275, which is half the width of the Andes and short of Tibet's. It is
	// fifteen tiles of the globe preset's deep span and five of a small
	// globe's. See beltOn.
	//
	// Where in the real range it sits was read off the small globes, which at
	// 375 had a belt three and a half tiles wide: fewer arcs grew into
	// continent, plate sizes fell off as A^-0.37 against Bird's 0.15-0.35, and
	// the drainage exponent (0.379), Hack's exponent (0.512) and Flint's fit
	// (0.823) all fell out of their ranges. At 550 all four are in; at 750 the
	// plates were at 0.39 and the drainage at 0.50.
	beltReach = 550 * km
	// beltGrain is how long a stretch of a range keeps its character, in
	// tiles, and beltVary is how much of the range's height and width that
	// character is worth either way. A seam applied at one width and one
	// strength the whole way along it raises the same cross-section over and
	// over - an extruded ridge, the same shape at every step, which is what a
	// world of them looks like from above and is the last thing on this map
	// that reads as drawn. Varied, a range is broad here and narrow there,
	// with high ground where the two plates are biting and passes where they
	// are not.
	//
	// It runs down to a ninth of that, three or four tiles, and the finest
	// octave is not decoration. A belt raised smoothly is an inclined plane,
	// and an inclined plane drains in parallel lines: every tile on it sends
	// its water the same way, so the rills run side by side down the flank and
	// no two of them can ever meet. Counted on a globe, ground steeper than
	// 0.3 had convergence - three or more neighbours draining into a tile - on
	// four tiles in a thousand against twenty-seven in a thousand on gentle
	// ground, and the mountainsides came out combed: even, parallel streams at
	// a tile's spacing running straight into the sea.
	//
	// Water has to have somewhere to gather. With the spurs and hollows this
	// puts on a flank, the streams on steep ground join at twenty-four in a
	// hundred where they joined at eight, and a quarter of the river tiles up
	// there go away with the comb, because they were the same water drawn over
	// and over in parallel.
	//
	// A tenth of the way and not a quarter, which was tried first and is the
	// better-looking map. Rough flanks are ground nobody can build on or
	// plough, and the globe paid for them: over twenty-four seeds a quarter
	// took the settlements that held their founding size from seventeen to
	// eleven, the gate a birth has to pass from 0.202 to 0.145, and the
	// middling settlement from a hundred and twenty-seven people to seventeen.
	// At a tenth the joining is twenty-four in a hundred against the quarter's
	// twenty-seven - nearly all of it - and the batch comes back to sixteen
	// settlements held and 0.189, which is inside the noise of where it stood.
	// The eight-seed batch on the quarter said nought extinct and looked well;
	// it took twenty-four to see the eleven. See the note on batch size in
	// docs/baseline-globe.md.
	//
	// Routing the water differently does not answer this, which was tried
	// before any of it. D-infinity - the water leaving across the steepest of
	// the eight triangles round a tile, shared between the two neighbours it
	// falls between, after Tarboton - is the published answer to parallel flow
	// and it moved the joining from eight in a hundred to nine. It cannot do
	// more: a plane drains in parallel because it is a plane, and that is
	// correct drainage of the wrong ground. With the flanks given their spurs,
	// the plain single steepest step joined at twenty-seven in a hundred and
	// D-infinity at twenty-three, so it was taken out again.
	beltGrain = 30.0
	beltVary  = 0.55
	// arcGapReach is how far behind the trench the arc stands, in metres. The
	// country it raises reaches a belt's width beyond that, so an arc's works
	// are spread over the gap and the belt together.
	//
	// A line of volcanoes does not stand on the trench. The floor going down
	// has to reach a depth before it gives up what melts, and it has travelled
	// a long way under the other plate by then, so what comes up comes up well
	// inland of where the two actually meet, with a plain between: it is the
	// difference between the Andes and a wall at the water's edge.
	//
	// Raised at the seam, as it was, every arc on a map was a range along a
	// coast - and since an arc is what most meetings are, that was every range
	// on the map. The highest tenth of the land stood nine tiles from the sea
	// where the land itself averaged eighteen, so the mountains were twice as
	// coastal as the ground they stood on, and a world of them looked like a
	// world of rimmed islands.
	//
	// It is a distance on the ground and it is also capped at a quarter of
	// the way between two plate middles, because a map can be too small to
	// hold both halves of a subduction. The default valley is two kilometres
	// of country and a real arc stands a couple of hundred from its trench:
	// on ground that size you would see the one or the other and never the
	// gap between them. Left at its full width there, the arc's fire was
	// recorded a quarter of a plate away from the meeting that caused it, the
	// granite on a made valley fell to a hundredth of the ground and the
	// basalt rose past half of it - a map of untouched sea floor with the
	// works of its own edges landed somewhere else.
	//
	// Ten tiles and not more because of what a tile is here: the whole belt
	// is that plus ten, which is half a kilometre of ground, and the map is a
	// toy beside the thing it is named after. What is kept is the order of it
	// - trench, then a plain, then the mountains - and not the scale.
	//
	// That was when a tile was 25 metres in a history too. At deepSpan it is
	// the real gap: two hundred kilometres, where the slab under the arc is
	// a hundred deep (Gill 1981; Syracuse and Abers 2006 have the volcanic
	// front 150 to 300 kilometres from the trench).
	arcGapReach = 200 * km
	// seamLeast is the fewest tiles a belt or a gap is drawn over, whatever
	// it is in metres. It is the grid's and not the ground's: on a valley's
	// history a tile is two hundred kilometres, the arc's gap is one tile and
	// its belt two, and one tile between a trench and its arc is no plain,
	// nor two tiles a flank. Below three the ranges came back as walls on the
	// sea's edge.
	seamLeast = 3.0
	// axisWidth is how near the seam itself a tile has to be for what is
	// happening there to change what it is made of, rather than only how
	// high it stands. A collision lifts a belt of country ten tiles wide and
	// cooks the rock in the middle of it; a rift drops a valley and floors
	// the axis of it with melt. Recording the rock across the whole belt made
	// seven tiles in ten of a map igneous or metamorphic, which is a map of
	// nothing but seams.
	axisWidth = 2.0
)

// What the crust is held to while the history runs.
//
// A plate here is drawn and not derived, and a drawn thing comes out as
// whatever the drawing was: nearest-middle gives Voronoi cells, which are
// convex polygons with straight edges, and a world of them looks like a world
// of them. At the width of a valley you see one corner of one seam and it
// passes for country. At the width of a globe you see the whole diagram - a
// tiling of straight-sided rooms with a straight range along every wall - and
// it is the most artificial thing on the map.
//
// A warped nearest-middle was the first answer, and it bends the walls without
// changing the rooms: every piece still came out a blob about as big as every
// other, because the middles were scattered evenly and each held the ground
// nearest it. The earth is not tiled like that. Its plates run from the
// Pacific, a fifth of the world, down through a long tail of small ones, and
// the sizes follow a power law (Bird 2003; Sornette and Pisarenko 2003) - a
// handful of great plates and many little ones, the little ones crowded where
// plates are going down under each other (Mallard et al. 2016).
//
// So a plate is grown and not measured. Every middle floods outward over the
// map at a rate of its own, across ground that is harder going in some places
// than others and along a grain the plate has, and a tile belongs to whichever
// flood reached it first. A fast flood is a great plate; a slow one caught
// between two fast ones is a microplate; the rough ground frays the walls; the
// grain draws a plate out long. See partition.
//
// And the pieces are held to conditions rather than left to the draw: a piece
// too small to be a plate is taken into the one it leans on; a piece too large
// is rifted in two; and continents that run into each other weld and stop
// being two plates at all.
const (
	// plateRough is how much harder the hardest ground is to grow across than
	// the easiest, as the power of e either way, and roughGrain is how long a
	// stretch of it is, as a share of the spacing between middles. Three
	// octaves: a seam bends on the scale of a plate, wanders on the scale of a
	// country and frays on the scale of a range.
	//
	// Like the grain below it is drawn once and stays where it is while the
	// plates travel over it: it is the old weaknesses in the crust, and a
	// boundary finds them rather than carrying them along.
	plateRough = 1.1
	roughGrain = 0.7
	// plateJitter is how much harder one tile may be to cross than the next,
	// whatever the country round it is like. Smooth rough ground bends a seam
	// and leaves it smooth, and a small plate on it came out a disc; a flood
	// over ground that differs tile by tile has a ragged front at every size,
	// which is what the random flood fills of Red Blob Games and Experilous
	// are after.
	plateJitter = 0.8
	// plateStretch is the most a plate is drawn out along its own grain: a
	// flood goes this much more slowly across the grain than along it, so a
	// plate at the full stretch is about that much longer than it is wide.
	// Real plates are not discs - the Nazca is a strip and the Pacific a
	// wedge - and a flood that goes the same speed every way makes discs.
	plateStretch = 1.2
	// majorShare is how much of a world its great plates hold between them,
	// and majorsPer how many plates a world has for each great one, up to
	// majorsMost. The earth is seven or eight great plates holding most of it
	// (Morra et al. 2013), which is what majorsMost is.
	majorShare = 0.65
	majorsPer  = 4
	majorsMost = 8
	majorVary  = 0.35
	// minorTail is the power the lesser plates' sizes fall off with, and
	// minorRange how many times the smallest the largest of them may be. A
	// tail this heavy puts most of the lesser ground in a few middling plates
	// and leaves the rest as microplates.
	minorTail  = 1.0
	minorRange = 12.0
	// plateLeast is the smallest share a plate is drawn at. Under this it is
	// nothing a belt could stand on: the belt is ten tiles either side of a
	// seam.
	plateLeast = 0.008
	// calibrations is how many times the first floods are rerun, each time
	// speeding the plates that came out short of what they were drawn at and
	// slowing the ones that came out over, before the history starts.
	calibrations = 12
	// enclaveMost is the largest share of a world a plate may hold and still
	// be taken into the one plate it touches. See reshape.
	enclaveMost = 0.05
	// breakFrom is the least share of the world a plate must hold for a
	// small one to break off it.
	breakFrom = 0.04
	// breakStretch is how much more slowly a broken-off piece floods toward
	// the middle of its plate than along the plate's edge. See breakOff.
	breakStretch = 4.0
	// breakTries is how many times a piece that came out too small is put back
	// and broken off again with a faster flood.
	breakTries = 6
	// plateFloor and plateCeiling are how much of a world one piece of crust
	// may hold before something is done about it. Without a floor the drift
	// squeezes pieces down to slivers and then to nothing, and a sliver is a
	// seam that raises a range no wider than itself; without a ceiling a
	// world that has done a lot of welding ends as one plate with a rim, and
	// a single plate has no edges in it and so no country.
	//
	// The floor is under the least a plate is drawn at, so a microplate is
	// left to be one. At a share and a half of a hundred, the floor this had
	// when every plate was the same size, it would have taken in every small
	// plate the first epoch, and the tail of the sizes with them.
	plateFloor   = 0.005
	plateCeiling = 0.22
	// crustFloor is how many pieces a world must be left with. Welding only
	// goes one way, so without a floor a long history ends as one continent
	// and nothing is happening anywhere.
	crustFloor = 4
	// weldEnough is how much closing two continents have to do, added up
	// along their contact and over the epochs, before they stop being two
	// plates: it is quoted against how far apart two middles stand, so that
	// a wide world welds on the same terms a narrow one does. Welding on one
	// epoch's closing alone made a world that had joined itself up by the
	// third age; this asks for a collision that lasted.
	weldEnough = 2.5
)

// How far a plate moves in an epoch, in tiles, at the start of the plate era
// and at the end of it. A young world convects hard and its plates race; an
// old one has cooled and slowed. That decay is the whole of what cooling
// means here, and it is why the ranges raised early are worn down to shields
// by the end while the ones raised late still stand.
// historySea is how much of a young world is under water while its history
// runs. A world has oceans whatever the map cut out of it at the end does -
// the default valley asks for no sea at all - and what lay under one age
// after age is where limestone comes from. So a history floods itself to this
// share, records who was drowned, and hands the finished ground to the map's
// own sea share, which is why a valley with no sea in it can still have
// limestone country: that ground was a seabed once.
//
// A share of the map and not a level read off where the two kinds of crust
// are riding, which was tried and is the better-sounding rule: the water
// fills what is low because what is low is ocean floor. It is worse in the
// one way that matters. The plates are moved about by everything else that
// happens to them, so on two seeds of five the level came out under
// everything and nothing was ever drowned - no seabed, and so no limestone
// anywhere on the map. A share always drowns something.
const historySea = 0.35

// marineMud is how much a sea bed off a shore takes in an epoch, against the
// one an epoch of burial on land is worth. fillEnough is how much has to have
// fallen on a tile before what it is made of is the fill rather than whatever
// was underneath - a couple of epochs' worth, so that ground which dipped
// below its river once or twice is still the basement it always was. And
// Which of the two a fill makes is settled by the sand in it against the
// clay, which is the sorting asked the only question it can answer: what
// stopped here, and what went on past.
// madeEnough is how much has to have happened to a tile before what happened
// to it decides what it is made of, in metres of ground moved.
//
// The fill has always had one - see fillEnough - and the fire and the crushing
// had none, so any trace of either beat a continent: a seam that brushed past
// a tile once, raising it by a hand's breadth, made that tile igneous forever.
// One epoch of a rift working at its full rate, which is the scale at which
// something has actually happened to the ground rather than merely happened
// near it: six kilometres.
//
// It is the second half of what stopped a made world paving itself. With melt
// taken as a level rather than a total the basalt on a made valley falls from
// fifty-four parts in a hundred to thirty-four; with this as well it falls to
// twenty-four, and the sandstone, the shale and the schist all rise. No rock
// owns the map and all six are on it in quantity.
const madeEnough = -riftRate * epochYears

// accreteEnough is how many metres of volcanic ground have to be raised on a
// tile of ocean floor, over the whole history, before it is continent: four
// epochs of a rift's flooring, as it was tuned at.
const accreteEnough = 4 * madeEnough

// fillRate is how fast a basin fills, in metres a year, and so what an epoch
// of burial lays on a tile: a tenth of a millimetre, which is what sediment
// piles up at when it is measured over millions of years rather than over one
// flood (Sadler 1981 - a rate read over a longer span is a slower one, because
// it has more gaps in it). Four hundred metres an epoch is a fifteenth of what
// makes a rock, as the one an epoch was of the fifteen metres before.
const fillRate = 0.1 * mm / yr

const (
	marineMud  = 0.5
	fillEnough = 3 * fillRate * epochYears
	// coarseShare is how much of a world's filled ground comes out sandstone
	// rather than shale: the sandiest third of it. It is a share and not a
	// cutoff for the reason every other share on this map is - see
	// forestShare - and here the reason is sharper than usual. What a deposit
	// is made of hardly varies while a history is running, because until the
	// rock is settled at the end every tile is weathering the same basalt,
	// so the sand in one basin and the next differ by a few hundredths and
	// any fixed line puts nearly all of them on one side of it: at sand
	// against clay the map came out a third sandstone, and one step stricter
	// it came out with none at all. Ranking the fills against each other asks
	// the only thing the sorting can actually answer - which of these
	// stopped soonest - and a third is about the share of the world's
	// sedimentary rock that is sandstone.
	coarseShare = 0.35
)

// The beds a history lays. bedPerFill is how many metres of rock an epoch's
// fill makes - a unit of what keepBook counts as laid - and limeBed how many
// an epoch under quiet water leaves. They are metres of the history's own
// ground, which is handed the drawn map's spread at the end, and they are
// set against the lift of a range in an epoch - see orogeny - because what
// makes a scarp is a bed as thick as a hillside is high: a pile of beds a
// hand's breadth thick is one rock to anything the weather can do.
//
// They were 9 and 6 metres an epoch against a lift of 45, and they are real
// rates now, as the lift is: a basin's fill at fillRate, four hundred metres an
// epoch, and lime at a fifth of that, which is what a carbonate shelf keeps
// over millions of years (Schlager 1981 has 0.01 to 0.1 mm/yr at that span).
// Held in proportion to the lift instead - 3.6 and 2.4 kilometres an epoch -
// sixteen epochs of them were deeper than the history's whole relief, which
// real erosion keeps to kilometres and not to what the plates lift, and every
// tile on a made valley was the last epoch's fill. The depths the fire and the
// crushing reach are real too: greenschist from some eight kilometres down,
// and plutons set at five to fifteen.
//
// sandyBed is the sand a fill needs to be laid as sandstone while the
// history runs, before settleRock ranks the fills against each other.
//
// foldShare is how far a collision buckles the beds under it against how far
// it lifts the ground, and foldWave how many tiles it is from one fold's
// crest to the next.
//
// cookDepth and plutonDepth are how far below the finished ground a
// collision has squeezed the pile into schist and an arc melted it into
// granite: what lies shallower is what the weather has yet to take off it.
//
// limeWarm and limeCold are the sea's yearly mean, in degrees, at which a
// quiet floor makes all of limeBed and none of it, and quietMud the rate the
// fine mud reaches it at whatever the warmth: see quietFloor. Five millimetres
// a thousand years is between the millimetre of the red clay of the deep
// ocean and the centimetres of the hemipelagic mud off a margin (Kennett,
// Marine Geology, 1982), and a history's quiet floor is a tile - tens of
// kilometres - from the nearest land.
const (
	bedPerFill  = fillRate * epochYears
	limeBed     = 0.02 * mm / yr * epochYears
	limeWarm    = 20.0
	limeCold    = -8.0
	quietMud    = 0.005 * mm / yr
	sandyBed    = 0.45
	foldShare   = 0.5
	foldWave    = 5.0
	cookDepth   = 8 * km
	plutonDepth = 12 * km
)

// Deep time: how long an epoch is, and how big a tile is while it lasts.
//
// An epoch was 120 ages of weather, twelve hundred years, and in it a plate
// crossed a tile - 25 metres, two centimetres a year - while a collision
// raised its range 45 metres, which is 37 millimetres a year: four to ten
// times the fastest rock uplift measured anywhere (Lavé and Avouac 2001 have
// 4 to 8 across the front of the Himalaya). A history of sixteen epochs was
// nineteen thousand years, and nothing it described - a continent rifting, an
// ocean closing, an arc growing into a continent - takes less than tens of
// millions. The figures were right for each other and wrong for the world.
//
// So an epoch is epochYears, and every rate in it is a real one: plates in
// centimetres a year, uplift and subsidence in millimetres a year, the water
// cutting at Erodibility and the ground creeping at Diffusivity, the same K
// and the same D a settlement's valley wears at, on the same clock of years.
// What that fixes is the size of a tile. A plate here crosses driftFast tiles
// an epoch at the speed of the fastest plates, times driftScale on a wider
// world, because that is what keeps a history's plates meeting and parting
// as often on a globe as on a valley; at driftFastRate for epochYears that
// is deepSpan metres a tile. On the globe preset it is 37.5 kilometres, and
// the globe is 38,400 kilometres round - within four in a hundred of the
// planet its air already reads it as. On a small globe it is 106 kilometres,
// and on the valley 200: a history's world is a planet in a handful of tiles.
//
// A history reads its tiles at that span - the fall of the ground over a tile,
// the ground the water comes off, how far the creep reaches - and hands the
// finished heights to the map by rank alone (see normalise and basins), which
// shape.go then lays again at TileSpan as the water would have worn it. That
// rank, and the rock, are all a 25 metre tile keeps of its history; which is
// honest, since a 25 metre tile's worth of anything is below what an epoch on
// a 37 kilometre tile can say. A watered history hands on two things more,
// which are a planet's and not a hillside's: the depth of its sea floor, from
// the age of the crust, and how fast its rock was rising, which the shaping
// grades the land's rivers to. See abyss.go.
//
// The river's wandering is not run in deep time. A bend is metres to hundreds
// of metres across, which is inside one of these tiles, and a height moved
// sideways off a tile two hundred kilometres wide is not a bank.
//
// What this is not yet is a coarse grid of its own. The design that would be:
// run the history on a grid of deepTarget tiles - one to five kilometres, so
// that an orogen's flanks and an arc's gap from its trench are tens of tiles
// and not one - with the plates carried a tile at a time as now but many
// steps to an epoch (at 5 cm/yr and 2 km a tile, a hundred steps), wearing
// every step; then downscale onto the playable grid by bilinear interpolation
// of the coarse heights as shape's uplift field, with the rock and the plate
// taken by nearest coarse tile, and the fine relief the shaping's own. The
// cost that stops it here is the move: shifting a crust a tile is a pass over
// the whole grid, and a hundred of them an epoch on a coarse globe of 20,000 by
// 10,000 two-kilometre tiles is a thousand times what a history costs now.
// The unit plumbing is what it needs first, and is what this is.
const epochYears = 4 * myr

// deepSpan is how wide, in metres, a tile of g is read as while a history
// runs. See epochYears.
func deepSpan(g *Grid) float64 {
	return driftFastRate * epochYears / (driftFast * driftScale(g))
}

// smoothing is how many times a finished history is softened before its
// heights are matched to a drawn map's spread. A seam raises a range narrower
// than the country a drawn map puts its high ground over, and since the
// matching hands out the same heights either way, a narrower range means a
// steeper one: on one seed of three the steepest tenth came out at 1.03
// against a drawn 0.38. Softening spreads the extremes over the ground around
// them, and the matching afterwards puts the heights back exactly, so what it
// costs is sharpness and not scale.
// One pass and not two: softening the ground also softens the bends a river
// has cut into it, and at two the made worlds ran a fifth straighter than at
// one for no gain in how gentle they were.
const smoothing = 1

// marginRamp is how far the step at the edge of a plate is spread, in passes
// of a nine-tile average - so a handful of tiles either side, which is a
// continental margin at this scale.
const marginRamp = 6

// How fast a plate goes at the start of the plate era and at the end of it, in
// metres a year: five centimetres, which is a quick plate today - the Pacific
// goes at seven to ten, the Atlantic's at two (DeMets, Gordon and Argus 2010)
// - and one, which is a slow one. A young world convects hard and its plates
// race; an old one has cooled and slowed.
//
// driftFast and driftSlow are the same two speeds in the units a plate's DX
// and DY are kept in: tiles an epoch on the valley, at the fastest. At a tile
// an epoch a seam crosses a fifth of a default map over a whole history, which
// leaves plate interiors that were never touched by anything. Faster than
// that and every tile on the map has been in a mountain range at some point,
// which is the same uniformity this was meant to replace. What those tiles
// are in metres follows from the speed: see deepSpan.
const (
	driftFastRate = 5 * cm / yr
	driftSlowRate = 1 * cm / yr
)

const (
	driftFast = 1.0
	driftSlow = driftFast * driftSlowRate / driftFastRate
	// feedShare is how much of how hard two plates meet is read off the crust
	// that has actually gone down between them, rather than off how fast they
	// are closing, and feedBlur how many passes of a nine-tile average that
	// crust is spread over. See tectonics.
	feedShare = 0.5
	feedBlur  = 3
	// tidyPasses is how many times a moved world's plate edges are tidied,
	// and tidyMost how many of a tile's eight neighbours another plate has to
	// hold to take it. See tidy.
	//
	// Six and once. A tile a tooth one tile wide sticks out of has six of the
	// other plate round it, and a tile on a bend of an honest boundary has at
	// most five; at five and twice, run every epoch, it was a smoothing of
	// every edge on the map thirty-two times over, and a history's plates
	// came out as round as pebbles.
	tidyPasses = 1
	tidyMost   = 6
	// spinRim is how fast a plate's spin carries its rim, as a share of
	// driftFast, at the most it is drawn at; and spinMost is the fastest a
	// plate may turn at all, in radians an epoch. See Plate.Spin.
	//
	// It is quoted at the rim and not as an angle because that is what a
	// boundary feels: a great plate turning by a degree an epoch moves its far
	// edge further than a microplate turning by ten. So the small plates are
	// the ones that spin fast, which is what the earth's do - the Easter and
	// Juan Fernandez microplates, caught between two ridges, have turned
	// through tens of degrees in a few million years - and the great ones
	// mostly slide. Half, so that on a great plate the drift is still most of
	// what happens at its edges and a range is still longer than an epoch.
	spinRim  = 0.5
	spinMost = 0.2
)

// What a meeting of plates does to the ground, in metres a year where two
// plates close head-on at driftFastRate. Continents crumple and stay up
// because they are too light to go down; ocean floor meeting a continent goes
// under it, which trenches the one and lifts an arc of volcanoes on the
// other; two floors meeting make islands out of open water. Parting drops the
// ground and floors it with what comes up.
//
// Rock uplift, before the weather takes any of it: 4 to 8 millimetres a year
// across the Himalayan front (Lavé and Avouac 2001), 1 to 3 in the forearcs
// and arcs of the Andes and Japan, up to 5 on the islands of Vanuatu (Taylor
// and others 1980), and 0.1 to 2 of subsidence in a young rift or at the
// outer wall of a trench (Allen and Allen 2013). The five are kept in the
// proportion they were tuned to - 45, 28, -20, 18 and -15 - which is what
// decides which rock a meeting leaves, and the whole set is put where the
// collision's own figure is real.
const (
	orogenyRate = 4.5 * mm / yr
	arcRate     = 2.8 * mm / yr
	trenchRate  = -2.0 * mm / yr
	islandRate  = 1.8 * mm / yr
	riftRate    = -1.5 * mm / yr
)

// And the same, as metres in an epoch: what liftOf hands out.
const (
	orogeny  = orogenyRate * epochYears
	arcLift  = arcRate * epochYears
	trench   = trenchRate * epochYears
	islandUp = islandRate * epochYears
	rifting  = riftRate * epochYears
)

// How high a plate floats before anything happens at its edges: a continent
// stands above the ocean floor because it is thicker and lighter, and that
// one fact is what gives a world coasts at all rather than an even skin of
// water. The floor is not at nothing, because a height of nothing is where
// the ground stops being allowed to fall - see wear - and a sea floor pinned
// against that stop comes out as a dead flat plain: half the map at a slope
// of a hundredth, against a tenth on a drawn one. What matters is the
// distance between the two levels and not either figure, since normalise
// rescales the lot.
//
// settling is how much of the way to its own level a plate comes in an
// epoch, so that crust which changes hands rises or sinks over an age rather
// than jumping. It moves a whole plate by what its middle is short of, so a
// plate keeps the country it is carrying; see tectonics. It is written as a
// time, settleTime, which is no measured figure: a load on the mantle is
// compensated in some ten thousand years, and what is slow here is a plate as
// a whole coming to the level of the crust it has gathered, which stands in
// for the isostasy this model does not have. Twenty-five million years is the
// fifteen in a hundred an epoch it was tuned at.
//
// The distance between the levels is the real one: the continents' mean
// stands 4.5 kilometres above the abyssal floor, +0.8 against -3.7 (Cogley
// 1984). It was 240 metres, beside a range that rose 45 an epoch; with the
// range at its real rate the step has to be real too, or every coast is a
// seam. The floor stands six kilometres above nothing, which is about how far
// the deepest trenches go below it.
const (
	oceanFreeboard     = 6 * km
	continentFreeboard = oceanFreeboard + 4.5*km
	settleTime         = 25 * myr
	// bowRise is how far a plate's own ground stands off its level, either
	// way, and bowSpan is how broad one of those swells or basins is, in
	// tiles. bowPull is how much of the way to that shape the ground comes in
	// an epoch.
	//
	// A plate does not ride flat. Without saying so, the only thing that ever
	// varies a plate's height is its edges, so a continent is a table two
	// hundred and forty metres above the floor beside it and the sea fills
	// the floor exactly: the coastline is the plate boundary, tile for tile,
	// on every map. And since the boundary is also where every mountain is
	// built, every mountain on the map stands on a coast - which is not what
	// a world looks like. Given a bow of the same order as that freeboard,
	// the sea reaches into the low parts of continents and leaves the high
	// parts of ocean floor standing, so where the water stops and where the
	// plates meet are two different questions again.
	//
	// The span is in tiles and not in plate widths. Drawn at two and a half
	// plate widths, which was the first try, it is not a bow but a tilt: the
	// whole of one plate up and the whole of the next down, which moved the
	// coast nowhere and put long smooth ramps beside the ranges. What is
	// wanted is country inside a plate, so it is drawn at the size of the
	// country.
	//
	// bowPull is a pull toward a shape and not toward a level, which is the
	// whole difference between this and the settling above it. Settling had
	// to be applied to a plate whole, because pulling every tile toward a
	// level of its own takes a share of whatever texture the ground has every
	// epoch, and sixteen of those is all of it. Pulling toward a bow puts
	// texture in rather than taking it out.
	//
	// A kilometre, which is the dynamic topography a mantle holds a plate's
	// ground up or down by (Hager and others 1985; Braun 2010). It was two
	// hundred metres beside a freeboard of 240; the freeboard is real now, and
	// so is this.
	bowRise = 1 * km
	bowSpan = 64.0
	// bowLeast is the finest octave of it: below this the weather and the
	// water are already saying what the ground does at that size.
	bowLeast = 6.0
	bowPull  = 0.30
)

// settling is the share of the way to its level a plate comes in an epoch of
// settleTime.
var settling = 1 - math.Exp(-epochYears/settleTime)

// How many places in a world are fed from below rather than at their edges,
// and how far each one's works reach. They stay where they are while the
// plates come and go over them, and since the plates carry their ground, what
// a hotspot leaves is a chain of old cones drawn out behind it along the way
// its plate is going, the newest over the hotspot itself.
const (
	hotspots    = 2
	hotspotSpan = DefaultWidth
	// hotspotReach is how far a hotspot's works reach from it, in metres: the
	// flanks and moat of a volcanic pile like Hawaii's, some 250 kilometres
	// out (Watts 2001). See hotspotOn.
	hotspotReach = 250 * km
	// hotspotLift is what one does in an epoch it is awake, at its middle: a
	// millimetre a year, in the proportion to a collision it was tuned at, which
	// is about the rate the Hawaiian volcanoes pile up their edifices at over a
	// million years (a few kilometres of shield; Clague and Dalrymple 1987).
	hotspotLift = 1 * mm / yr * epochYears
	// hotspotWakes is how often one of them is awake in an epoch. A volcano
	// is not a thing that happens continuously for the age of a world: it
	// goes off, and then it is quiet for longer than anybody watching it will
	// be alive. Left erupting every epoch at forty-five metres a time, two
	// hotspots built seven-hundred-metre cones five tiles across - three
	// perfect circles that were, on two seeds of four, the steepest ground on
	// the map by a wide margin and the whole of what made a made world
	// steeper than a drawn one.
	hotspotWakes = 0.35
)

// Plate is one piece of a world's crust: where it is going, what it is made
// of, and - once it has welded onto another - which piece it has become part
// of. Where it is is the tiles that ride it: they are grown from middles once,
// when the crust first breaks, and after that a plate is its ground, carried
// along with it. See move.
type Plate struct {
	// DX, DY is how its middle goes, and Spin how fast it turns about that
	// middle, in radians an epoch. A tile of it at p goes
	//
	//	v = (DX, DY) + Spin × (p - middle)
	//
	// which on this cylinder is the flat shadow of what a plate on a sphere
	// does, turning about an Euler pole. With the drift alone every tile of a
	// plate went the same way at the same speed, so a boundary was the same
	// kind of boundary the whole way along it and nothing ever swung round:
	// no seam that closes at one end and opens at the other, no arc that bows,
	// no plate that wheels into a neighbour it had been sliding past.
	//
	// The drift turns with the plate, so that the pole stays where it is
	// relative to the plate and the plate goes round it on an arc, as a
	// plate does between one reorganisation of the earth and the next.
	DX, DY float64
	Spin   float64
	Ocean  bool
	// cx, cy is where the middle of its ground is now, in tiles, which is what
	// it turns about. See locate.
	cx, cy float64
	// grow is how fast this plate's flood spreads, and so how much ground it
	// holds against its neighbours; leanX, leanY is the grain it is drawn out
	// along and stretch how far. See partition.
	grow, leanX, leanY, stretch float64
	// into is this plate, unless it has been welded into another, in which
	// case it is the one it went into - which may itself have gone into a
	// third. See rootOf. A plate that has gone into another is never revived:
	// crust that has joined has joined.
	into uint8
}

// middle is a point a plate's flood starts from, in tiles, and the plate it
// floods for: the first plates are grown from them, and a rifting plate is
// split by flooding from two. See partition and split.
type middle struct {
	X, Y float64
	at   uint8
}

// rootOf is the plate a middle actually belongs to now: follow the welds.
func rootOf(plates []Plate, k uint8) uint8 {
	for plates[k].into != k {
		k = plates[k].into
	}
	return k
}

// standing counts the pieces of crust a world still has, which is what
// crustFloor is held against.
func standing(plates []Plate) int {
	n := 0
	for k := range plates {
		if plates[k].into == uint8(k) {
			n++
		}
	}
	return n
}

// seam is what is happening at the nearest place two plates meet: how much
// the ground there rises or falls in an epoch, and how far off that place is.
// It is the generator's working, kept on the grid only so that a history does
// not allocate a map's worth of it every epoch.
type seam struct {
	lift  float64
	away  float64
	makes made
	found bool
	// side is the plate this belt belongs to, and stay says it may not be
	// carried onto any other. A collision is two plates doing the work and
	// its belt crumples both of them; a subduction is one plate going under
	// another, and everything it raises - the arc - is on the plate that
	// stayed up, while the trench is on the one that went down. Carried
	// across, an arc put its mountains on the sea floor it was consuming.
	side uint8
	stay bool
	// with is the plate on the other side of the meeting: what the book
	// writes down beside side as the pair that raised the ground. See ledger.
	with uint8
}

// raisedBy is what kind of meeting this seam is, for the book: made tells a
// collision from an arc, and the sign of the lift tells a rift, where the
// plates part and the ground drops, from islands coming up between two
// floors that are closing.
func (s *seam) raisedBy() MeetingKind {
	switch s.makes {
	case crushed:
		return Collision
	case arc:
		return Arc
	case melt:
		if s.lift < 0 {
			return Rift
		}
		return Islands
	}
	return NoMeeting
}

// made is what a meeting makes of the rock at its axis, as against what it
// does to the height of the country round it. A trench makes nothing: it is
// ground going down and away.
type made uint8

const (
	nothing made = iota
	crushed
	arc
	melt
)

// record is what has been done to a tile over the whole history, which is
// what decides the rock it ends up being. It is the generator's working and
// is not kept: the tile keeps the answer.
type record struct {
	// melt is what came up and cooled at the surface, and pluton what melted
	// under an arc and cooled at depth. They are the same fire and they make
	// different rock, which is the whole reason for keeping them apart: what
	// reaches the air is basalt, and what stops on the way is the granite
	// that a few million years of weather then lays bare.
	//
	// melt is the deepest single flooring and not the sum of all of them,
	// which is the one of these four that is not a thickness. Crust is
	// thickened by a collision and a basin is filled by a river, and both of
	// those add up; ground is floored by a rift, and flooring it twice leaves
	// it floored, not floored twice as much. Added up it ran away: a rift axis
	// drifts across the map, so a tile that was ever near one accrued a total
	// nothing else could outweigh and could never lose again. A made valley
	// came out fifteen per cent basalt after one epoch and fifty-four after
	// sixteen, with its granite falling from forty-nine to five - the longer
	// the history ran the more of the continent it turned to ocean floor,
	// which is the opposite of what running it longer should do.
	melt   float64
	pluton float64
	crush  float64 // metres raised by two plates meeting, and cooked doing it
	laid   [Grains]float64
	// submerged is how many epochs this tile spent under the sea a history
	// floods itself to. Ground that lay there quietly, with no river mud
	// reaching it, is where limestone comes from.
	submerged int
	// On a history grid coarser than the map, where the melt, the fire and
	// the crushing lay across the tile: whole, where any of that kind covered
	// all of it; otherwise the most of it a band of that kind covered, and
	// how far the tile's centre stood from that band's middle then, in the
	// map's tiles. The rock is then decided on the map, tile by tile, and not
	// on the history's tiles: see handDown.
	whole [makings]bool
	share [makings]float32
	near  [makings]float32
}

// The three makings of rock a history writes into bands beside a seam.
const (
	makingMelt = iota
	makingPluton
	makingCrush
	makings
)

// banded notes that a band of making k covered share of the tile, its middle
// near map tiles from the tile's centre. See record.
func (r *record) banded(k int, share, near float64) {
	if share >= 1 {
		r.whole[k] = true
		return
	}
	if float32(share) > r.share[k] {
		r.share[k], r.near[k] = float32(share), float32(near)
	}
}

// history makes a world by running one. It leaves every tile with a height, a
// rock, the plate it rides and the epoch that rock dates from, and it leaves
// the drainage worked out, so that everything after it in Generate - the
// woods, the outcrops, the soils, the market - reads the same kind of ground
// it would have read from the picture.
func (w *Land) history(g *Grid, epochs int, sea, water float64) *deepStage {
	defer phase("history")()
	// The tiles are pieces of a planet until the history is over. See
	// epochYears.
	g.deep = deepSpan(g)
	w.molten(g)
	// The water has to have something to carry. Soil is made from the rock
	// beneath it, and at the end of the molten era that is basalt everywhere;
	// what the epochs then do is sort it, which is what makes the fill of one
	// basin coarse and the next one fine - and so which of them becomes
	// sandstone and which shale.
	g.drain()
	g.soilTexture()
	fl := w.flood(g, spacing(g, plateTotal(g)))
	plates := w.firstPlates(g, sea, water > 0, fl)
	cr := newCrust(g)
	for i := range g.Tiles {
		cr.ocean[i] = plates[g.Tiles[i].Plate].Ocean
	}
	grain := w.grain(g)
	bow := w.bow(g)
	book := make([]record, len(g.Tiles))
	// And the part of it that is kept when the history is over. See ledger.
	g.openBook(epochs)
	// What the plates have done to each other, kept across the epochs because
	// welding is something that happens over an age and not in one.
	touch := make([]float64, plateCap*plateCap)
	weld := make([]float64, plateCap*plateCap)

	if epochWatch != nil {
		epochWatch(g, cr, plates, -1) // the first plates, before any has moved
	}
	for e := 0; e < epochs; e++ {
		// How far through the era we are, which is how far the world has
		// cooled: the plates slow as it goes.
		through := float64(e) / math.Max(1, float64(epochs-1))
		// The plates carry their ground on: where they run into each other
		// one goes down, and where they part new floor comes up. See move.
		w.move(g, plates, cr, book, e)
		g.joinUp(fl)
		cr.kinds(g, plates)
		// How much boundary each pair shares is a fact about this epoch and
		// is taken afresh; how hard they have driven into each other is what
		// welds, and that adds up over the whole history.
		for i := range touch {
			touch[i] = 0
		}
		w.tectonics(g, plates, cr, book, e, arcGapOn(g, standing(plates)), touch, weld, grain, bow)
		// An age of weather between the ages of the earth. What was raised
		// this epoch starts coming down in the next, and what comes off it is
		// what fills the basins - which is where a finished map's sandstone
		// and shale come from.
		// The water is worked out on the ground the plates have just moved
		// before it is asked to cut it: the cutting walks from each tile to
		// the one its water goes to, and the hollows the plates have made
		// hold what the water brings them: see stillWork.
		g.base = g.historyBase()
		g.drain()
		g.wear(epochYears)
		g.keepBook(book, e)
		// What the epoch floored with lava, filled or silted over is a new
		// surface, and its soil starts from nothing. See pedogenesis.go.
		g.restartBuried(e)
		// And the weather next epoch meets the rock this one has bared.
		g.expose()
		plates = w.reshape(g, plates, fl, touch, weld)
		slow(plates, float64(max(0, e-1))/math.Max(1, float64(epochs-1)), through)
		if epochWatch != nil {
			epochWatch(g, cr, plates, e)
		}
	}

	// The ages of the floor and how fast the ground is rising are read while
	// the tiles are still pieces of a planet. See floorDepths and upliftOf.
	d := &deepStage{ocean: cr.ocean}
	if water > 0 {
		d.depths, d.shares = g.floorDepths(cr, epochs)
		d.uplift = g.upliftOf(cr)
	}
	g.base, g.deep = -1, 0
	g.keepPlates(plates)
	g.settleRock(book, cr.ocean)
	if g.planet > 0 {
		d.book = book // the foot of every pile is laid on the map
	}
	for k := 0; k < g.passes(smoothing); k++ {
		g.soften()
	}
	return d
}

// deepStage is what a history hands the map beside the grid it ran on: which
// tiles are ocean crust, and, for a world given water, how deep the floor lies
// by its age, how far each tile is from the continent's edge, and how fast the
// rock is rising. See historygrid.go for how it crosses to a map of another
// size.
type deepStage struct {
	ocean                  []bool
	depths, shares, uplift []float64
	// book is the history's book of what was done to each tile, where the
	// feet of the piles are still to be laid on the map: a history run on a
	// grid coarser than the map. See handDown.
	book []record
}

// settleHistory lays a finished history on the map: its heights handed the
// spread of a map's, the ground too steep for a map's tiles brought down, the
// deep floor laid and the water read. g is the map, which the history has
// been handed down onto if it ran on a grid of its own.
func (w *Land) settleHistory(g *Grid, d *deepStage, water float64) {
	// A world given water keeps the basins its plates made, for the water to
	// fill; one that is not keeps the drawn map's spread whole. See basins.
	// The beds are carried through the rescaling with the ground over them.
	was := g.heights()
	if water > 0 {
		w.basins(g, d.ocean)
		g.restrata(was, g.heights(), d.ocean)
		g.uplift = d.uplift
	} else {
		w.normalise(g)
		g.restrata(was, g.heights(), nil)
	}
	// The rescaling hands a narrow range the whole height of a drawn one, and
	// what that stands up steeper than ground can stand on comes down. See
	// slide.go.
	g.landslide(false)
	// And the deep sea floor is laid at the depth its age puts it, below the
	// ground the slides reach. See abyss.
	if d.depths != nil {
		g.layAbyss(d.depths, d.shares)
	}
	g.expose()
	g.drain()
}

// molten is the world before it had a crust. Each churn is a new surface half
// blended into the last, so the era leaves a lumpy floor with no lasting
// shape - no ranges, no basins, nothing that outlives the making of it. All
// of it is basalt, because that is what a world cools into, and any that is
// still basalt at the end is the oldest ground on the map.
func (w *Land) molten(g *Grid) {
	h := make([]float64, len(g.Tiles))
	for c := 0; c < moltenChurns; c++ {
		// Fine cells rather than broad swells, and finer as the churns go on:
		// convection at this stage is small and furious.
		cell := w.lattice(g, g.inTiles(math.Max(4, float64(g.planetSpan())/float64(2+2*c))))
		for i := range h {
			h[i] = moltenMemory*h[i] + (1-moltenMemory)*cell[i]
		}
	}
	g.strata = make([]column, len(g.Tiles))
	for i := range g.Tiles {
		t := &g.Tiles[i]
		g.Height[i] = moltenRelief * h[i]
		t.Bedrock, t.Formed = Basalt, 0
		g.strata[i] = basement(Basalt, 0, g.Height[i])
	}
}

// firstPlates is the crust going rigid: it breaks, and the pieces start
// moving. Where the breaks fall is drawn rather than derived - a world's
// first plates are an accident of how it cooled and there is nothing to read
// them off - but which pieces are ocean and which are continent decides the
// shape of everything after.
//
// How big the pieces are is drawn the way the earth's are: the great plates
// first, spread out over the world and holding most of it, and then the lesser
// ones, sized off a power law and set down where the great ones are driving
// into each other - which is where the small plates of the earth are, broken
// off the edges of the slabs going down. The floods are then run a few times
// and each plate's rate put right until the plates hold the ground they were
// drawn at, since where two floods meet depends on every other flood as well
// as on those two.
func (w *Land) firstPlates(g *Grid, sea float64, poured bool, fl *flooding) []Plate {
	n := plateTotal(g)
	ocean := clamp01(oceanFloor + oceanPerSea*sea)
	plates := make([]Plate, n, plateCap)
	mids := make([]middle, n, plateCap)
	share := plateShares(w, n)
	majors := majorCount(n)
	for i := range plates {
		a := 2 * math.Pi * w.RNG.Float64()
		plates[i] = Plate{
			DX:    driftFast * math.Cos(a),
			DY:    driftFast * math.Sin(a),
			Ocean: w.RNG.Float64() < ocean,
			into:  uint8(i),
			grow:  math.Sqrt(share[i]),
		}
		w.lean(&plates[i])
	}
	// The great plates, each as far from the others as a handful of tries
	// finds: set down at random they bunch, and two great plates side by side
	// is one great plate with a seam through it.
	for i := 0; i < majors; i++ {
		mids[i] = w.apart(g, mids[:i], nil)
		mids[i].at = uint8(i)
	}
	// The lesser ones, on the seams where the great plates are closing.
	g.partition(plates[:majors], mids[:majors], fl)
	g.locate(plates)
	closing := g.closingSeams(plates)
	for i := majors; i < n; i++ {
		mids[i] = w.apart(g, mids[:i], closing)
		mids[i].at = uint8(i)
	}
	for c := 0; c < calibrations; c++ {
		g.partition(plates, mids, fl)
		held := make([]float64, n)
		for i := range g.Tiles {
			held[g.Tiles[i].Plate]++
		}
		for i := range plates {
			// Half of the square root: area goes as the square of how far a
			// flood gets, and a whole step overshoots, because a plate that
			// grows takes its ground from neighbours that are growing too.
			want := share[i] * float64(len(g.Tiles))
			p := &plates[i]
			p.grow *= math.Pow(want/math.Max(1, held[i]), 0.35)
			p.grow = math.Min(8, math.Max(0.05, p.grow))
		}
	}
	g.partition(plates, mids, fl)
	if poured {
		w.balanceCrust(g, plates, ocean)
	}
	// A world has both kinds in it. Left to the draw, a valley - which asks
	// for no sea and so for few ocean plates - came out on two seeds of five
	// with nothing but continent, and a world with no floor anywhere has no
	// arcs in it, nothing going under anything, and so no granite and no
	// islands: three of the six rocks lose the only place they come from.
	ocean, land := 0, 0
	for i := range plates {
		if plates[i].Ocean {
			ocean++
		} else {
			land++
		}
	}
	switch {
	case ocean == 0:
		plates[0].Ocean = true
	case land == 0:
		plates[0].Ocean = false
	}
	// The spins are drawn last, after everything the first plates were
	// already drawn with, so that giving plates a spin did not hand every
	// seed a different world: drawn in with the drift, it moved which plates
	// came out ocean on every seed, and a valley that had been four
	// continents driving into each other came out three with two floors
	// between them and not a tile of schist.
	for i := range plates {
		plates[i].Spin = w.spin(g, math.Sqrt(share[i]*float64(len(g.Tiles))/math.Pi))
	}
	return plates
}

// crustSlack is how far the share of a world's first crust that is ocean floor
// may come out from the share its terms ask for before the draw is put right.
//
// The kinds are drawn plate by plate, and the plates are not the same size: a
// great plate is a fifth of a world, so the draw that says a half of the plates
// are floor can say a half or four fifths of the ground is. How much of a
// planet is continent is a fact about how much light rock it has melted out of
// its mantle, which is a matter of its chemistry and its age and not of which
// way one plate's coin fell (Taylor and McLennan 1995; the earth's continental
// crust is about four tenths of it, shelves and all). The crust is set once
// at the first plates and carried from there, so a world given its water and
// not its sea - see water.go - came out with what that one coin said: the
// first globe was eight tenths floor, and eight tenths sea.
//
// So the draw stands inside a band either side of the asked share, and is only
// put right when it falls outside: inside it, how much continent a world has is
// still its own luck, and the plates still decide where it goes.
//
// It is only done for a world given water. A world given a share of sea is
// flooded to that share whatever its crust, so the draw does not decide how
// much of it is drowned, and a made valley's ground is matched to the drawn
// one's: put right there as well, the twelfth valley came out with nearly four
// times the drawn map's water.
const crustSlack = 0.15

// balanceCrust turns plates of the kind there is too much of into the other,
// one at a time and each the one that brings the ground nearest the share
// wanted, until the ocean floor is within crustSlack of it. It never turns the
// last plate of a kind, which is the rule below it in firstPlates to keep.
func (w *Land) balanceCrust(g *Grid, plates []Plate, want float64) {
	n := len(g.Tiles)
	if n == 0 {
		return
	}
	held := make([]float64, len(plates))
	for i := range g.Tiles {
		if k := int(g.Tiles[i].Plate); k < len(plates) {
			held[k]++
		}
	}
	for {
		floor, kinds := 0.0, 0
		for k := range plates {
			if plates[k].Ocean {
				floor += held[k] / float64(n)
				kinds++
			}
		}
		over := floor > want+crustSlack
		if !over && floor >= want-crustSlack {
			return
		}
		if (over && kinds <= 1) || (!over && kinds >= len(plates)-1) {
			return
		}
		best, near := -1, math.Abs(floor-want)
		for k := range plates {
			if plates[k].Ocean != over || held[k] == 0 {
				continue
			}
			to := floor + held[k]/float64(n)
			if over {
				to = floor - held[k]/float64(n)
			}
			if d := math.Abs(to - want); d < near {
				best, near = k, d
			}
		}
		if best < 0 {
			return
		}
		plates[best].Ocean = !over
	}
}

// spacing is how far apart two neighbouring middles stand on this world, in
// tiles: the side of the square each of them would hold if they held equal
// shares. It is the one length a plate has, and every condition the crust is
// held to is quoted in it so that the same rules mean the same thing on a
// valley and on a globe.
func spacing(g *Grid, mids int) float64 {
	return math.Sqrt(float64(len(g.Tiles)) / math.Max(1, float64(mids)))
}

// plateTotal is how many plates a world first breaks into. See plateCount.
func plateTotal(g *Grid) int {
	return min(plateMost, max(3, plateCount*g.planetSpan()/plateSpan))
}

// majorCount is how many of a world's first plates are great ones.
func majorCount(n int) int {
	return min(n, max(2, min(majorsMost, n/majorsPer)))
}

// plateShares is how much of the world each first plate is drawn to hold:
// the great plates first, near enough equal, and then the lesser ones off a
// power law. Every share is between plateLeast and a little under the ceiling,
// so nothing is rifted or taken in for being what it was drawn as.
func plateShares(w *Land, n int) []float64 {
	majors := majorCount(n)
	share := make([]float64, n)
	for i := range share {
		if i < majors {
			share[i] = 1 + majorVary*(2*w.RNG.Float64()-1)
		} else {
			// A Pareto draw, cut off at minorRange: most of them small, a
			// few of them not.
			share[i] = math.Min(minorRange, math.Pow(1-w.RNG.Float64(), -1/minorTail))
		}
	}
	scale := func(from, to int, total float64) {
		sum := 0.0
		for _, s := range share[from:to] {
			sum += s
		}
		for i := from; i < to; i++ {
			share[i] *= total / math.Max(1e-9, sum)
		}
	}
	if majors == n {
		scale(0, n, 1)
	} else {
		scale(0, majors, majorShare)
		scale(majors, n, 1-majorShare)
	}
	// Held inside the bounds, and the whole put back to one, a few times
	// over since each undoes a little of the other.
	for k := 0; k < 4; k++ {
		for i := range share {
			share[i] = math.Min(0.8*plateCeiling, math.Max(plateLeast, share[i]))
		}
		scale(0, n, 1)
	}
	return share
}

// spin draws how fast a plate as wide as radius tiles turns: its rim at up to
// spinRim of the fastest drift either way, and never past spinMost. See
// spinRim.
func (w *Land) spin(g *Grid, radius float64) float64 {
	rim := spinRim * driftFast * driftScale(g) * (2*w.RNG.Float64() - 1)
	return math.Max(-spinMost, math.Min(spinMost, rim/math.Max(1, radius)))
}

// lean gives a plate the grain it is drawn out along.
func (w *Land) lean(p *Plate) {
	a := math.Pi * w.RNG.Float64()
	p.leanX, p.leanY = math.Cos(a), math.Sin(a)
	p.stretch = plateStretch * w.RNG.Float64()
}

// apart is a place for a new middle as far from the ones already set down as
// a handful of tries finds, taken from among the tiles of from where that is
// given and anywhere on the map where it is not.
func (w *Land) apart(g *Grid, mids []middle, from []int32) middle {
	best, far := middle{}, -1.0
	for try := 0; try < 16; try++ {
		var x, y float64
		if len(from) > 0 {
			i := int(from[w.RNG.IntN(len(from))])
			x, y = float64(i%g.W)+0.5, float64(i/g.W)+0.5
		} else {
			x, y = w.RNG.Float64()*float64(g.W), w.RNG.Float64()*float64(g.H)
		}
		near := math.Inf(1)
		for _, m := range mids {
			dx, dy := m.X-x, m.Y-y
			if g.Wrap {
				dx = math.Mod(math.Abs(dx), float64(g.W))
				dx = math.Min(dx, float64(g.W)-dx)
			}
			near = math.Min(near, dx*dx+dy*dy)
		}
		if near > far {
			best, far = middle{X: x, Y: y}, near
		}
	}
	return best
}

// closingSeams is every tile on the edge of its plate where the plate beside
// it is coming towards it.
func (g *Grid) closingSeams(plates []Plate) []int32 {
	var out []int32
	scale := driftScale(g)
	for i := range g.Tiles {
		p := g.PosOf(i)
		for _, d := range Dirs {
			q := geom.Pos{X: p.X + d.X, Y: p.Y + d.Y}
			if g.Wrap {
				q = g.Norm(q)
			}
			if !g.In(q) {
				continue
			}
			k := g.At(q).Plate
			if k == g.Tiles[i].Plate {
				continue
			}
			if g.closing(plates, g.Tiles[i].Plate, k, p.X, p.Y, d, scale) > 0 {
				out = append(out, int32(i))
				break
			}
		}
	}
	return out
}

// flooding is what partition works with: how hard each tile is to grow
// across, and the working of the floods themselves, kept for the whole
// history so that an epoch does not allocate a map's worth of it.
type flooding struct {
	cost []float32
	dist []float32
	from []uint8
	done []bool
	// queue is the floods' frontier, by how far each tile on it has been
	// reached in steps of floodStep. A flood only ever reaches further, so the
	// frontier is walked from the nearest step outwards and never back.
	queue [][]int32
}

// floodStep is how finely the frontier is sorted, in tiles of easy ground
// reached at a plate's own rate. Coarser is faster and rounds where two floods
// meet by up to this much.
const floodStep = 0.5

// flood draws how hard the crust is to grow across. See plateRough.
func (w *Land) flood(g *Grid, reach float64) *flooding {
	coarse := w.lattice(g, roughGrain*reach)
	fine := w.lattice(g, roughGrain*reach/3)
	fray := w.lattice(g, g.inTiles(math.Max(4, roughGrain*reach/9*g.coarseness())))
	fl := &flooding{
		cost: make([]float32, len(g.Tiles)),
		dist: make([]float32, len(g.Tiles)),
		from: make([]uint8, len(g.Tiles)),
		done: make([]bool, len(g.Tiles)),
	}
	for i := range fl.cost {
		v := ((coarse[i] - 0.5) + 0.5*(fine[i]-0.5) + 0.35*(fray[i]-0.5)) / 0.925 * 2
		fl.cost[i] = float32(math.Exp(plateRough*v) * (1 + plateJitter*(w.RNG.Float64()-0.5)))
	}
	return fl
}

// bow is the shape a plate rides in, in metres off its own level: swells and
// basins the size of a country. Like the rough ground and the grain it is drawn once
// for a world and stays where it is, so the ground keeps its shape from age to
// age rather than being redrawn under itself every epoch.
//
// It is a field over the map and not over each plate, which is the wrong way
// round for a raft and the right way round for what is being modelled - what
// holds a continent's middle up or lets it down is underneath it, so a plate
// that drifts over a swell is lifted while it is over it. Drawn per plate it
// carried the inland seas about with the continents, which looked like the
// continents were holding them.
func (w *Land) bow(g *Grid) []float64 {
	// Octaves down to a few tiles, and not the one broad swell it started as.
	// A single lattice is a field of round lumps, and since the sea now finds
	// its coast in this rather than at the plate boundaries, round lumps is
	// what the coastlines came out as - bays like bites and headlands like
	// thumbs, all of them the same size. The finer octaves are what make a
	// coast a coast.
	out := make([]float64, len(g.Tiles))
	amp, total := 1.0, 0.0
	for span := math.Min(float64(g.Span())/2, bowSpan/g.coarseness()); span >= bowLeast/g.coarseness(); span, amp = span/2, amp/2 {
		l := w.lattice(g, span)
		for i := range out {
			out[i] += amp * (l[i] - 0.5)
		}
		total += amp / 2
	}
	for i := range out {
		out[i] *= bowRise / math.Max(1e-9, total)
	}
	return out
}

// grain is how hard a seam is working at each place along it, in [1-beltVary,
// 1+beltVary], down to the spurs and hollows of a single flank. Like the rough ground under the floods it is drawn once for a world and stays where it
// is, so a range keeps the same shape age after age instead of shimmering
// between them, and two seams that cross the same ground are strong and weak
// in the same places - which is what an inherited weakness in the crust
// actually does.
func (w *Land) grain(g *Grid) []float64 {
	coarse := w.lattice(g, g.inTiles(beltGrain))
	fine := w.lattice(g, g.inTiles(beltGrain/3))
	spur := w.lattice(g, g.inTiles(beltGrain/9))
	out := make([]float64, len(g.Tiles))
	for i := range out {
		v := (coarse[i] - 0.5) + 0.5*(fine[i]-0.5) + 0.10*(spur[i]-0.5)
		out[i] = 1 + beltVary*v/0.80
	}
	return out
}

// slow slows every plate by how far the world has cooled. What changes is how
// fast it goes and not the shape of the path: its drift and its spin are slowed
// together, so a plate goes on round the same pole, and the only thing that
// turns its bearing is its own turning. A plate that hardly spins goes on
// driving into the same neighbour for the whole history and the range it
// raises is longer than the epoch that raised it.
//
// The spin is slowed by how much the world has cooled since it was last slowed and
// not by how fast the plate was going, since a plate just rifted off another
// drives away faster than the era's speed and is brought back to it here; read
// off that, a new half lost half its spin the moment it was made.
func slow(plates []Plate, before, through float64) {
	speed := driftFast + (driftSlow-driftFast)*through
	cooled := speed / (driftFast + (driftSlow-driftFast)*before)
	for k := range plates {
		p := &plates[k]
		if d := math.Hypot(p.DX, p.DY); d > 0 {
			p.DX, p.DY = p.DX/d*speed, p.DY/d*speed
		}
		p.Spin *= cooled
	}
}

// driftScale is how many tiles a plate moving at a speed of one travels in an
// epoch on this world. The speeds are quoted on the default valley, whose
// plates stand some two dozen tiles apart; a globe's stand five times that,
// and a plate crossing a tile an epoch there went ten tiles in a whole
// history - nothing opened, nothing closed, and a continent that rifted came
// out of it still touching its other half. So a plate goes as far in plate
// widths on any world, and how hard two of them meet is still read off the
// speeds.
func driftScale(g *Grid) float64 {
	quoted := math.Sqrt(float64(DefaultWidth*DefaultHeight) / plateCount)
	// Read at the planet's scale and then in g's tiles, so that a history
	// grid coarser than the map moves its plates as far over the planet.
	c := g.coarseness()
	return math.Max(1, spacing(g, plateTotal(g))*c/quoted) / c
}

// crust is what the plates carry about with them from epoch to epoch beside
// the tiles and the book: when each tile's crust was made, how much crust has
// gone down at each place this epoch, and how far each plate is short of its
// next whole tile. The rest is a move's working, kept so that an epoch does
// not allocate a map's worth of it.
type crust struct {
	born []uint8
	// ocean is whether each tile's crust is ocean floor rather than
	// continent. It is the tile's and not its plate's: crust is what it is
	// made of wherever it is carried and whichever plate it is carried on,
	// and a plate's own kind is only what most of its crust is. See kinds.
	ocean []bool
	// built is how much volcanic ground has been raised on each tile of
	// ocean crust, in metres over the whole history. Enough of it and the
	// crust is continent. See accrete.
	built []float64
	// rise is how fast each tile's rock has lately been rising, in metres a
	// year, carried with the crust; lifted is what the epoch being run has
	// raised it by, in metres. See upliftOf.
	rise, nrise []float64
	lifted      []float64
	// fed is how many tiles of crust went down, or were crumpled up, at each
	// place this epoch. It is what feeds the arcs and the ranges: see
	// tectonics.
	fed []float64
	acc [plateCap][2]float64
	// now is the epoch being moved, which is when any floor opened in it was
	// made.
	now uint8

	plate, nplate []uint8
	org, norg     []int32
	fresh, nfresh []bool
	nborn         []uint8
	nocean        []bool
	nbuilt        []float64
	// off is how far each tile's crust truly stands from the tile it is
	// shown on, which only a turn makes anything of: a turn puts a tile on
	// the tile nearest where it goes, and without the remainder kept a
	// place a third of a tile from the middle of a slow turn was put back
	// where it was every step and never went round at all.
	off, noff  [][2]float32
	mark       []bool
	ring, next []int32
	tiles      []Tile
	height     []float64 // the tiles' heights, soil, sand and clay, beside them as on the Grid
	soil       []float32
	sand, clay []float64
	book       []record
	strata     []column
}

// kinds says of every plate that holds any ground whether it is an ocean
// plate: whether most of the crust it carries is floor. What welds, what a
// sliver may be taken into and what a broken-off piece is are asked of the
// plate; what a meeting does and where the ground floats are asked of the
// crust.
func (cr *crust) kinds(g *Grid, plates []Plate) {
	var floor, all [256]int
	for i := range g.Tiles {
		k := g.Tiles[i].Plate
		all[k]++
		if cr.ocean[i] {
			floor[k]++
		}
	}
	for k := range plates {
		if all[k] > 0 {
			plates[k].Ocean = 2*floor[k] > all[k]
		}
	}
}

// accrete makes continent of the ocean crust volcanism has built up enough
// of. An island arc, or the pile a hotspot leaves, is floor that has been
// melted and raised and thickened until it is too light to go down: carried
// to a trench, it does not sink but is scraped onto the edge of whatever is
// there, and that is how continents grow. Left as floor, an arc stood out of
// the sea as a range for as long as the history lasted and then went under a
// continent with the rest of its plate, and no continent ever got any bigger.
func (cr *crust) accrete() {
	for i, b := range cr.built {
		if cr.ocean[i] && b >= accreteEnough {
			cr.ocean[i] = false
		}
	}
}

// noPlate is a tile no crust has come to yet.
const noPlate = 255

func newCrust(g *Grid) *crust {
	n := len(g.Tiles)
	return &crust{
		born: make([]uint8, n), fed: make([]float64, n),
		ocean: make([]bool, n), nocean: make([]bool, n),
		built: make([]float64, n), nbuilt: make([]float64, n),
		rise: make([]float64, n), nrise: make([]float64, n), lifted: make([]float64, n),
		plate: make([]uint8, n), nplate: make([]uint8, n),
		org: make([]int32, n), norg: make([]int32, n),
		fresh: make([]bool, n), nfresh: make([]bool, n),
		nborn: make([]uint8, n), mark: make([]bool, n),
		off: make([][2]float32, n), noff: make([][2]float32, n),
		tiles: make([]Tile, n), height: make([]float64, n), soil: make([]float32, n), sand: make([]float64, n), clay: make([]float64, n), book: make([]record, n), strata: make([]column, n),
	}
}

// move is an epoch of the plates carrying their ground: every tile goes with
// its plate, and its height, its rock, what the book has on it and how old its
// crust is go with it.
//
// A plate goes a whole tile at a time, when what it has travelled comes to
// one, so that nothing is ever resampled or smeared. Where that puts two
// crusts on one tile, one of them goes down: ocean floor under continent,
// because it is the heavier, and between two of a kind the older, because old
// crust is cold and cold crust is dense. Two continents do not really
// subduct, but what one of them loses at the front of a collision is still
// crust taken off the map and piled into the range, so it is counted the same
// way. Where a plate pulls off a tile and nothing comes onto it, the floor
// opens and new crust comes up: a ridge, made this epoch, and given to
// whichever of the plates beside it holds most of the ground round it - which,
// where two are pulling apart, is the nearer of them.
func (w *Land) move(g *Grid, plates []Plate, cr *crust, book []record, epoch int) {
	defer phase("move")()
	g.piles()
	scale := driftScale(g)
	cr.now = uint8(epoch)
	for i := range g.Tiles {
		cr.plate[i], cr.org[i], cr.fresh[i] = g.Tiles[i].Plate, int32(i), false
	}
	for k := range plates {
		p := &plates[k]
		if p.into == uint8(k) {
			cr.acc[k][0] += p.DX * scale
			cr.acc[k][1] += p.DY * scale
			// The drift is how the middle goes, and it turns with the plate.
			s, c := math.Sin(p.Spin), math.Cos(p.Spin)
			p.DX, p.DY = c*p.DX-s*p.DY, s*p.DX+c*p.DY
		}
	}
	var step [plateCap][2]int
	for {
		moving := false
		for k := range plates {
			step[k] = [2]int{}
			if plates[k].into != uint8(k) {
				continue
			}
			for a := 0; a < 2; a++ {
				switch {
				case cr.acc[k][a] >= 1:
					step[k][a] = 1
				case cr.acc[k][a] <= -1:
					step[k][a] = -1
				}
				cr.acc[k][a] -= float64(step[k][a])
			}
			moving = moving || step[k] != [2]int{}
		}
		if !moving {
			break
		}
		cr.shift(g, plates, &step)
	}
	// What each plate has travelled short of a whole tile is not held back:
	// it goes into how far its crust stands off its tiles, and the turn below
	// reads it there, so a plate stands where its travel has taken it and not
	// up to a tile behind. Held back, the lag was a tile of the grid whatever
	// the grid, and a history on a grid twice as coarse opened a tenth less
	// floor in its first epoch, and more of its partings left a continent
	// against the ocean than fresh floor between them. See historygrid.go.
	var shift [plateCap][2]float64
	for k := range plates {
		if plates[k].into != uint8(k) || cr.acc[k] == [2]float64{} {
			continue
		}
		shift[k] = cr.acc[k]
		cr.acc[k] = [2]float64{}
	}
	for i := range cr.plate {
		if sh := shift[cr.plate[i]]; sh != [2]float64{} {
			cr.off[i][0] += float32(sh[0])
			cr.off[i][1] += float32(sh[1])
		}
	}
	// And the epoch's turning, all at once. A turn about a plate's middle
	// and a slide of the whole plate come to the same thing in either order,
	// so what the slides above did not do is the turns, and what is left of
	// the slides.
	cr.turn(g, plates, &shift)

	copy(cr.tiles, g.Tiles)
	copy(cr.height, g.Height)
	copy(cr.soil, g.Soil)
	copy(cr.sand, g.Sand)
	copy(cr.clay, g.Clay)
	copy(cr.book, book)
	copy(cr.strata, g.strata)
	for j := range g.Tiles {
		t := cr.tiles[cr.org[j]]
		// The soil goes with the tile, fresh floor taking its neighbour's.
		g.Soil[j], g.Sand[j], g.Clay[j] = cr.soil[cr.org[j]], cr.sand[cr.org[j]], cr.clay[cr.org[j]]
		if cr.fresh[j] {
			// New floor, with the soil of the tile beside it: basalt, dated
			// from now, at the level ocean floor rides at. Set higher, as a
			// ridge fresh from below really stands, every parting seam on a
			// globe left a line of it above the sea, and the oceans came out
			// drawn over with the outlines of where plates used to be.
			g.Height[j] = oceanFreeboard
			t.Bedrock, t.Formed = Basalt, uint8(epoch)
			book[j] = record{melt: 2 * madeEnough, whole: [makings]bool{makingMelt: true}}
			g.strata[j] = basement(Basalt, uint8(epoch), g.Height[j])
		} else {
			g.Height[j] = cr.height[cr.org[j]]
			book[j] = cr.book[cr.org[j]]
			// The pile goes with the ground on top of it.
			g.strata[j] = cr.strata[cr.org[j]]
		}
		t.Plate = cr.plate[j]
		g.Tiles[j] = t
	}
	cr.tidy(g)
}

// tidy gives a tile to another plate where that plate holds nearly all the
// ground round it. A plate going a tile at a time eats into what it runs over
// a row at a time, and not every row goes at once, so a front of collision is
// left with teeth a tile or two wide sticking out of it into the other plate,
// and a seam raises a comb of ranges along them.
func (cr *crust) tidy(g *Grid) {
	for pass := 0; pass < tidyPasses; pass++ {
		for i := range g.Tiles {
			cr.nplate[i] = g.Tiles[i].Plate
			var seen [8]uint8
			var count [8]int
			n := 0
			g.eachNear(i, func(k int) {
				p := g.Tiles[k].Plate
				for m := 0; m < n; m++ {
					if seen[m] == p {
						count[m]++
						return
					}
				}
				seen[n], count[n] = p, 1
				n++
			})
			for m := 0; m < n; m++ {
				if seen[m] != g.Tiles[i].Plate && count[m] >= tidyMost {
					cr.nplate[i] = seen[m]
				}
			}
		}
		for i := range g.Tiles {
			g.Tiles[i].Plate = cr.nplate[i]
		}
	}
}

// sinks reports whether crust of plate p made in epoch bp goes down under
// crust of plate q made in epoch bq. It is an order and not a judgement, so
// that where three crusts come to one tile the answer does not depend on which
// of them arrived first.
//
// It is asked of the crust and not of the plates carrying it: a continent
// riding an ocean plate does not go down because its plate is mostly floor.
func sinks(p uint8, op bool, bp uint8, q uint8, oq bool, bq uint8) bool {
	if op != oq {
		return op
	}
	if bp != bq {
		return bp < bq
	}
	return p < q
}

// shift moves every plate that has a step one tile along it, all at once, and
// settles what that does: crust meeting crust, and crust leaving a tile bare.
func (cr *crust) shift(g *Grid, plates []Plate, step *[plateCap][2]int) {
	for j := range cr.nplate {
		cr.nplate[j] = noPlate
	}
	for i := range cr.plate {
		j := i
		if s := step[cr.plate[i]]; s != [2]int{} {
			x, y := i%g.W+s[0], i/g.W+s[1]
			if y < 0 || y >= g.H {
				continue // carried off the map
			}
			if x < 0 || x >= g.W {
				if !g.Wrap {
					continue
				}
				x = g.WrapX(x)
			}
			j = y*g.W + x
		}
		cr.land(plates, i, j)
	}
	cr.settle(g, nil)
}

// land puts the crust on tile i down on tile j, where it goes down under
// whatever is there already or that goes down under it.
func (cr *crust) land(plates []Plate, i, j int) {
	if cr.nplate[j] != noPlate {
		cr.fed[j]++
		if sinks(cr.plate[i], cr.ocean[i], cr.born[i], cr.nplate[j], cr.nocean[j], cr.nborn[j]) {
			return
		}
	}
	cr.nplate[j], cr.norg[j], cr.nfresh[j], cr.nborn[j] = cr.plate[i], cr.org[i], cr.fresh[i], cr.born[i]
	cr.nocean[j], cr.nbuilt[j], cr.nrise[j] = cr.ocean[i], cr.built[i], cr.rise[i]
	cr.noff[j] = cr.off[i]
}

// settle opens floor on whatever a step has left bare and makes what the step
// put down the crust as it now stands. See openFloor for shun.
func (cr *crust) settle(g *Grid, shun func(k uint8) bool) {
	cr.openFloor(g, shun)
	cr.plate, cr.nplate = cr.nplate, cr.plate
	cr.org, cr.norg = cr.norg, cr.org
	cr.fresh, cr.nfresh = cr.nfresh, cr.fresh
	cr.born, cr.nborn = cr.nborn, cr.born
	cr.ocean, cr.nocean = cr.nocean, cr.ocean
	cr.built, cr.nbuilt = cr.nbuilt, cr.built
	cr.rise, cr.nrise = cr.nrise, cr.rise
	cr.off, cr.noff = cr.noff, cr.off
}

// turn turns every plate by its spin for the epoch, about its middle.
//
// A turn cannot go a whole tile at a time the way a slide does, because no two
// tiles of a turning plate go the same distance. So it is read backwards: a
// tile is the turning plate's if the turn undone lands on ground of that plate,
// which gives the plate back its own area and shape; and what it carries there
// is whichever of the plate's tiles the turn truly takes nearest to it. Each
// tile keeps how far its crust stands off it, so a place a third of a tile from
// the middle of a slow turn still goes round over the epochs - put down each
// time on the tile nearest where it started, it never moved at all.
//
// It is done once an epoch and not a tile's worth at a time, and that is what
// keeps it honest. Carried forward a tile at a time, the rim rounded outward
// took its neighbour's ground while the tiles rounded inward left holes shared
// out as new floor, and a disc turning in still water grew by a third in a
// quarter turn; read backwards a tile's worth at a time, every step lost a few
// places and copied a few more, and fifty steps of that left the ground of a
// quarter turn three tiles on average from where the turn put it.
//
// Where a turning plate comes onto another plate's ground the two are settled
// as any meeting is, by which crust goes down, and where it leaves ground no
// turning plate covers the floor opens. What goes down in a turn is not counted
// as crust fed to a seam: along the rim of a plate turning in place the
// rounding trades a tile here for a tile there, and an arc fed by that would
// be an arc fed by arithmetic. How hard the seam is closing still counts - see
// tectonics.
func (cr *crust) turn(g *Grid, plates []Plate, shift *[plateCap][2]float64) {
	// A plate is read backwards where it turns or where its crust has been
	// slid short of a whole tile.
	moves := func(k uint8) bool { return plates[k].Spin != 0 || shift[k] != [2]float64{} }
	turning := false
	for k := range plates {
		turning = turning || (plates[k].into == uint8(k) && moves(uint8(k)))
	}
	if !turning {
		return
	}
	g.middleOf(plates, func(i int) uint8 { return cr.plate[i] })
	var sin, cos [plateCap]float64
	var lo, hi [plateCap][2]float64
	for k := range plates {
		sin[k], cos[k] = math.Sin(plates[k].Spin), math.Cos(plates[k].Spin)
		lo[k] = [2]float64{math.Inf(1), math.Inf(1)}
		hi[k] = [2]float64{math.Inf(-1), math.Inf(-1)}
	}
	// spun is an offset from plate k's middle turned by the plate's turn, or
	// by the turn undone where back is set.
	spun := func(k uint8, rx, ry float64, back bool) (float64, float64) {
		s := sin[k]
		if back {
			s = -s
		}
		return cos[k]*rx - s*ry, s*rx + cos[k]*ry
	}
	// place is where the crust on tile i truly stands, off plate k's middle.
	place := func(k uint8, i int) (float64, float64) {
		p, o := &plates[k], cr.off[i]
		return g.across(float64(i%g.W) + float64(o[0]) - p.cx), float64(i/g.W) + float64(o[1]) - p.cy
	}

	for j := range cr.nplate {
		cr.nplate[j] = noPlate
	}
	for i := range cr.plate {
		k := cr.plate[i]
		if !moves(k) {
			// What is not turning stays where it is.
			cr.land(plates, i, i)
			continue
		}
		// And what is, is looked for over the ground it is turned onto.
		rx, ry := place(k, i)
		x, y := spun(k, rx, ry, false)
		lo[k] = [2]float64{math.Min(lo[k][0], x), math.Min(lo[k][1], y)}
		hi[k] = [2]float64{math.Max(hi[k][0], x), math.Max(hi[k][1], y)}
	}
	for n := range plates {
		if lo[n][0] > hi[n][0] {
			continue // turning nothing
		}
		k, p := uint8(n), &plates[n]
		x0, x1 := int(math.Floor(p.cx+lo[k][0]))-1, int(math.Ceil(p.cx+hi[k][0]))+1
		if g.Wrap && x1-x0 >= g.W {
			x1 = x0 + g.W - 1
		}
		y0, y1 := max(0, int(math.Floor(p.cy+lo[k][1]))-1), min(g.H-1, int(math.Ceil(p.cy+hi[k][1]))+1)
		for y := y0; y <= y1; y++ {
			for xx := x0; xx <= x1; xx++ {
				x := xx
				if x < 0 || x >= g.W {
					if !g.Wrap {
						continue
					}
					x = g.WrapX(x)
				}
				j := y*g.W + x
				// Where this tile was before the turn and what is left of the
				// slide, and the tile that is.
				bx, by := spun(k, g.across(float64(x)-p.cx), float64(y)-p.cy, true)
				fx, fy := int(math.Round(p.cx+bx-shift[k][0])), int(math.Round(p.cy+by-shift[k][1]))
				if fy < 0 || fy >= g.H || (!g.Wrap && (fx < 0 || fx >= g.W)) {
					continue
				}
				b := fy*g.W + g.WrapX(fx)
				best, near := b, math.Inf(1)
				var dx, dy float64
				// boxed is whether a candidate must stand within the tile's own
				// half-open square to be taken: see below.
				boxed := false
				try := func(i int) {
					if cr.plate[i] != k {
						return
					}
					qx, qy := place(k, i)
					ex, ey := g.across(qx-bx), qy-by
					if boxed && (ex <= -0.5 || ex > 0.5 || ey <= -0.5 || ey > 0.5) {
						return
					}
					if d := ex*ex + ey*ey; d < near {
						best, near, dx, dy = i, d, ex, ey
					}
				}
				if shift[k] == [2]float64{} {
					if cr.plate[b] != k {
						continue
					}
					try(b)
					g.eachNear(b, try)
				} else {
					// The crust stands off its tile by the slide as well as by
					// what earlier turns left, so the tile under it is looked for
					// two tiles round where the slide alone puts it, and it is
					// this plate's ground where some of its crust stands within
					// the tile's own square - half open, so that crust standing
					// exactly half a tile off is the ground of one tile and not of
					// both or neither - the nearest such crust.
					boxed = true
					for oy := -2; oy <= 2; oy++ {
						qy := fy + oy
						if qy < 0 || qy >= g.H {
							continue
						}
						for ox := -2; ox <= 2; ox++ {
							qx := fx + ox
							if !g.Wrap && (qx < 0 || qx >= g.W) {
								continue
							}
							try(qy*g.W + g.WrapX(qx))
						}
					}
					if math.IsInf(near, 1) {
						continue
					}
				}
				if cur := cr.nplate[j]; cur == k || (cur != noPlate && !sinks(cur, cr.nocean[j], cr.nborn[j], k, cr.ocean[best], cr.born[best])) {
					continue
				}
				cr.nplate[j], cr.norg[j], cr.nfresh[j], cr.nborn[j] = k, cr.org[best], cr.fresh[best], cr.born[best]
				cr.nocean[j], cr.nbuilt[j], cr.nrise[j] = cr.ocean[best], cr.built[best], cr.rise[best]
				// It stands where the turn put it, but never further off than
				// its own tile: crust carried here because nothing nearer was
				// is standing in for ground the rounding lost.
				rx, ry := spun(k, dx, dy, false)
				cr.noff[j] = [2]float32{
					float32(math.Max(-0.5, math.Min(0.5, rx))),
					float32(math.Max(-0.5, math.Min(0.5, ry))),
				}
			}
		}
	}
	// What is bare now no turning plate covers, so none of them is offered it
	// while any other plate is: floor a turn leaves is the other side's.
	// Shared out by the count alone, the tiles a disc's rim gave up in still
	// water went back to the disc as new floor it had not made.
	cr.settle(g, func(k uint8) bool { return plates[k].Spin != 0 })
}

// openFloor makes new crust on every tile a shift has left bare, from the
// edges of the bare ground inwards a ring at a time: each tile goes to the
// plate that holds most of the tiles round it and takes its soil from one of
// them. A ring is decided whole before any of it is written, so that the order
// the tiles are looked at in does not lean the new floor toward one side.
//
// A plate shun says yes to is given a tile only where every plate round it is
// one shun says yes to.
func (cr *crust) openFloor(g *Grid, shun func(k uint8) bool) {
	ring, next := cr.ring[:0], cr.next[:0]
	for j := range cr.mark {
		cr.mark[j] = false
	}
	for j := range cr.nplate {
		if cr.nplate[j] != noPlate {
			continue
		}
		g.eachNear(j, func(k int) {
			if cr.nplate[k] != noPlate {
				cr.mark[j] = true
			}
		})
		if cr.mark[j] {
			ring = append(ring, int32(j))
		}
	}
	type pick struct {
		plate uint8
		from  int32
		off   [2]float32
	}
	var picks []pick
	for len(ring) > 0 {
		picks = picks[:0]
		for _, j := range ring {
			var seen [8]uint8
			var count [8]int
			var from [8]int32
			var off [8][2]float32
			n := 0
			g.eachNear(int(j), func(k int) {
				p := cr.nplate[k]
				if p == noPlate {
					return
				}
				for m := 0; m < n; m++ {
					if seen[m] == p {
						count[m]++
						return
					}
				}
				seen[n], count[n], from[n], off[n] = p, 1, cr.norg[k], cr.noff[k]
				n++
			})
			best := 0
			for m := 1; m < n; m++ {
				if shun != nil && shun(seen[m]) != shun(seen[best]) {
					if !shun(seen[m]) {
						best = m
					}
					continue
				}
				if count[m] > count[best] || (count[m] == count[best] && seen[m] < seen[best]) {
					best = m
				}
			}
			picks = append(picks, pick{seen[best], from[best], off[best]})
		}
		for r, j := range ring {
			cr.nplate[j], cr.norg[j], cr.nfresh[j], cr.nborn[j], cr.nocean[j] = picks[r].plate, picks[r].from, true, cr.now, true
			cr.nbuilt[j], cr.nrise[j] = 0, 0
			// New floor stands off its tile as the crust of the plate it
			// joins does, so that a plate's ground stays one lattice: at
			// nought beside crust that stood off by a slide, the floor stood
			// closer to the ground ahead of it than a tile, and took it.
			cr.noff[j] = picks[r].off
		}
		next = next[:0]
		for _, j := range ring {
			g.eachNear(int(j), func(k int) {
				if cr.nplate[k] == noPlate && !cr.mark[k] {
					cr.mark[k] = true
					next = append(next, int32(k))
				}
			})
		}
		ring, next = next, ring
	}
	cr.ring, cr.next = ring, next
}

// partition says which plate each tile rides when the crust first breaks:
// whichever middle's flood got to it first.
//
// Every middle floods out across the map at once, each at its own plate's
// rate, slowed by rough ground and by going across its plate's grain, and a
// tile is taken by the first flood to reach it and then passed on from there.
// So a plate is always one piece - a flood can only go on from ground it holds
// - and it holds more ground the faster it goes and less the more it is hemmed
// in. Nearest-middle, which this replaced, is the same thing with every flood
// at one speed over even ground, and that is a Voronoi diagram: convex rooms
// of much the same size however the walls are bent.
func (g *Grid) partition(plates []Plate, mids []middle, fl *flooding) {
	g.floodOver(plates, mids, fl, -1)
	g.joinUp(fl)
}

// floodOver is the flooding partition does, over the whole map where within is
// below nothing and over the tiles of plate within where it is not - which is
// how a plate is rifted in two. See split.
func (g *Grid) floodOver(plates []Plate, mids []middle, fl *flooding, within int) {
	// What a step in each of the eight directions costs each middle, before
	// the ground: the length of the step, slowed across the grain, over the
	// plate's rate.
	step := make([][8]float32, len(mids))
	at := make([]uint8, len(mids))
	for k := range mids {
		p := plates[mids[k].at]
		for d, dir := range Dirs {
			l := math.Hypot(float64(dir.X), float64(dir.Y))
			across := (float64(dir.X)*-p.leanY + float64(dir.Y)*p.leanX) / l
			step[k][d] = float32(l * (1 + p.stretch*across*across) / math.Max(1e-3, p.grow))
		}
		at[k] = rootOf(plates, mids[k].at)
	}
	for i := range fl.dist {
		fl.dist[i] = float32(math.Inf(1))
		fl.done[i] = false
	}
	for b := range fl.queue {
		fl.queue[b] = fl.queue[b][:0]
	}
	push := func(i int32, d float32) {
		b := int(d / floodStep)
		for len(fl.queue) <= b {
			fl.queue = append(fl.queue, nil)
		}
		fl.queue[b] = append(fl.queue[b], i)
	}
	for k, m := range mids {
		// The tile the middle stands on. A middle can drift off a map that
		// does not go round, and then its flood starts at the nearest edge,
		// already as late as the walk in from where the middle is.
		x, y := int(math.Floor(m.X)), int(math.Floor(m.Y))
		off := 0
		if y < 0 {
			off, y = -y, 0
		} else if y >= g.H {
			off, y = y-g.H+1, g.H-1
		}
		if g.Wrap {
			x = g.WrapX(x)
		} else if x < 0 {
			off, x = off-x, 0
		} else if x >= g.W {
			off, x = off+x-g.W+1, g.W-1
		}
		i := int32(y*g.W + x)
		d := float32(off) * step[k][4]
		if d < fl.dist[i] {
			fl.dist[i], fl.from[i] = d, uint8(k)
			push(i, d)
		}
	}
	for b := 0; b < len(fl.queue); b++ {
		// Indexed and not ranged: a step shorter than floodStep lands back in
		// the bucket being walked, and has to be walked too.
		for n := 0; n < len(fl.queue[b]); n++ {
			i := fl.queue[b][n]
			if fl.done[i] {
				continue
			}
			fl.done[i] = true
			k := fl.from[i]
			x, y := int(i)%g.W, int(i)/g.W
			for d, dir := range Dirs {
				qx, qy := x+dir.X, y+dir.Y
				if qy < 0 || qy >= g.H {
					continue
				}
				if qx < 0 || qx >= g.W {
					if !g.Wrap {
						continue
					}
					qx = g.WrapX(qx)
				}
				j := int32(qy*g.W + qx)
				if fl.done[j] || (within >= 0 && int(g.Tiles[j].Plate) != within) {
					continue
				}
				nd := fl.dist[i] + step[k][d]*(fl.cost[i]+fl.cost[j])/2
				if nd < fl.dist[j] {
					fl.dist[j], fl.from[j] = nd, k
					push(j, nd)
				}
			}
		}
		fl.queue[b] = fl.queue[b][:0]
	}
	for i := range g.Tiles {
		if within < 0 || fl.done[i] {
			g.Tiles[i].Plate = at[fl.from[i]]
		}
	}
}

// joinUp makes every plate one piece. A flood is one piece, but a plate that
// is carried about can be cut in two - a plate running into another along a
// front can be eaten through where the front is narrow - and a piece left on
// the far side is a scrap of one plate adrift in another. Every piece of a
// plate but its largest goes to the plate it borders most.
//
// Borders most among the pieces that stay where they are, and not among
// everything round it, because the scraps are all handed out at once and a
// scrap can border another scrap that is going somewhere else. The second
// globe of TestEveryPlateIsOnePiece did exactly that on its last epoch: a
// single tile of plate 22 was cut off against a piece of plate 3 nine hundred
// tiles big, five of its neighbours on that piece and three on plate 23, so it
// was given to 3 - in the same pass as the piece of 3 was given to 23. What
// was left was one tile of plate 3 on the far side of plate 23, and since the
// last epoch has no move after it and so no joinUp, the world ended with it.
// Counting only the largest pieces, a scrap always joins ground that is going
// to stay its plate's; one whose every neighbour is another scrap waits until
// those have been handed out, and is handed out the time after.
func (g *Grid) joinUp(fl *flooding) {
	defer phase("joinUp")()
	piece := fl.dist // the distances are spent; the space is reused for labels
	largest := make([]int, plateCap)
	var sizes []int
	var stack []int32
	for {
		for i := range piece {
			piece[i] = -1
		}
		sizes = sizes[:0]
		for s := range g.Tiles {
			if piece[s] >= 0 {
				continue
			}
			label, of := float32(len(sizes)), g.Tiles[s].Plate
			piece[s], stack = label, append(stack[:0], int32(s))
			n := 0
			for len(stack) > 0 {
				i := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				n++
				g.eachNear(int(i), func(j int) {
					if piece[j] < 0 && g.Tiles[j].Plate == of {
						piece[j] = label
						stack = append(stack, int32(j))
					}
				})
			}
			sizes = append(sizes, n)
		}
		for i := range largest {
			largest[i] = -1
		}
		for s := range g.Tiles {
			l, of := int(piece[s]), g.Tiles[s].Plate
			if largest[of] < 0 || sizes[l] > sizes[largest[of]] {
				largest[of] = l
			}
		}
		// The pieces cut off, and which of the pieces that stay each of them
		// borders; and whether any of them borders another that does not.
		border := map[int]map[uint8]int{}
		waiting := false
		for i := range g.Tiles {
			l, of := int(piece[i]), g.Tiles[i].Plate
			if largest[of] == l {
				continue
			}
			g.eachNear(i, func(j int) {
				o := g.Tiles[j].Plate
				switch {
				case o == of:
				case largest[o] != int(piece[j]):
					waiting = true
				default:
					if border[l] == nil {
						border[l] = map[uint8]int{}
					}
					border[l][o]++
				}
			})
		}
		to := map[int]uint8{}
		for l, near := range border {
			best, most := uint8(0), -1
			for o, n := range near {
				if n > most || (n == most && o < best) {
					best, most = o, n
				}
			}
			to[l] = best
		}
		for i := range g.Tiles {
			if o, ok := to[int(piece[i])]; ok {
				g.Tiles[i].Plate = o
			}
		}
		// A map always has a piece that stays, and so a scrap beside one, so
		// every pass hands out at least one and this ends.
		if !waiting {
			return
		}
	}
}

// eachNear calls f with each of the eight tiles round i that is on the map.
func (g *Grid) eachNear(i int, f func(j int)) {
	x, y := i%g.W, i/g.W
	for _, d := range Dirs {
		qx, qy := x+d.X, y+d.Y
		if qy < 0 || qy >= g.H {
			continue
		}
		if qx < 0 || qx >= g.W {
			if !g.Wrap {
				continue
			}
			qx = g.WrapX(qx)
		}
		f(qy*g.W + qx)
	}
}

// tectonics is one epoch of what the plates do to the ground they carry.
//
// Every tile is asked what is happening at its own edges: for each neighbour
// riding a different plate, how fast the two are closing or parting along the
// line between them. Closing raises ground, and some of what it raises comes
// up as melt; parting drops it and fills the drop from below. Away from an
// edge nothing happens at all, which is why the middle of a plate is the
// oldest, flattest ground on a map and everything worth looking at is at the
// seams.
func (w *Land) tectonics(g *Grid, plates []Plate, cr *crust, book []record, epoch int, gap float64, touch, weld, grain, bow []float64) {
	defer phase("tectonics")()
	g.piles()
	n := len(g.Tiles)
	scale := driftScale(g)
	belt := beltOn(g)
	g.locate(plates)
	// Where crust has gone down this epoch, spread a few tiles: it goes down on
	// one side of a seam and the seam is on both, and a plate moves a whole
	// tile at a time, so the tiles it went down on are a dotted line.
	fed := cr.fed
	for k := 0; k < feedBlur; k++ {
		fed = g.spread(fed)
	}
	defer func() {
		for i := range cr.fed {
			cr.fed[i] = 0
		}
	}()
	// And what that is worth as a closing. It is scaled so that over the
	// seams that are closing it comes to what their closing does, which is
	// the one reading of it that means the same on a valley and on a globe
	// and in an epoch when the plates went five tiles as in one when they
	// went none: the crust says where a meeting is fed, and the speeds say
	// how hard meetings are.
	var closed, sank float64
	for i := range g.Tiles {
		if worst, _, ok := g.meeting(plates, i, scale); ok && worst > 0 {
			closed += worst / driftFast
			sank += fed[i]
		}
	}
	gain := 0.0
	if sank > 0 {
		gain = closed / sank
	}
	if len(g.seam) != n {
		g.seam = make([]seam, n)
		g.seamQueue = make([]int32, 0, n)
	}
	for i := range g.seam {
		g.seam[i] = seam{}
	}
	g.seamQueue = g.seamQueue[:0]

	// Where the plates actually touch, and what is happening there.
	for i := range g.Tiles {
		t := &g.Tiles[i]
		mine := plates[t.Plate]
		worst, j, ok := g.meetingAt(plates, i, scale)
		if !ok {
			continue
		}
		worstAt := g.Tiles[j].Plate
		at := &plates[worstAt]
		// What the two of them have done to each other, for the crust to be
		// reshaped by afterwards: how much boundary they share at all, and -
		// where both are continent - how hard they have been driving into
		// each other. The first says which plate a piece too small to go on
		// leans against; the second is what welds. See reshape.
		a, b := t.Plate, worstAt
		if a > b {
			a, b = b, a
		}
		touch[int(a)*plateCap+int(b)]++
		if !mine.Ocean && !at.Ocean && worst > 0 {
			weld[int(a)*plateCap+int(b)] += worst / driftFast
		}
		closing := worst / driftFast
		if worst > 0 && gain > 0 {
			// Half of how hard a meeting works is how fast the two are
			// closing, and half is how much crust has actually gone into it:
			// the arc is fed by the floor going down under it and the range
			// by the continent crumpled into it. The two agree over an age
			// and not in an epoch - a plate goes a tile at a time - so the
			// closing keeps a range rising steadily while the crust says
			// where it is really being fed.
			closing = (1-feedShare)*closing + feedShare*fed[i]*gain
		}
		// What the meeting does is what the crust on either side of it is made
		// of, whatever the plates carrying it mostly are.
		lift, makes := liftOf(cr.ocean[i], cr.ocean[j], closing)
		// An arc and a trench are the two halves of one plate going under
		// another, and each belongs to its own side of it.
		under := worst > 0 && cr.ocean[i] != cr.ocean[j]
		g.seam[i] = seam{lift: lift, makes: makes, found: true, side: t.Plate, stay: under, with: worstAt}
		g.seamQueue = append(g.seamQueue, int32(i))
	}

	// And how far its works reach either side of it. The lift falls off with
	// distance from the seam, so a range has flanks rather than sides.
	for k := 0; k < len(g.seamQueue); k++ {
		i := g.seamQueue[k]
		here := g.seam[i]
		p := geom.Pos{X: int(i) % g.W, Y: int(i) / g.W}
		for _, off := range Dirs {
			q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
			if g.Wrap {
				q = g.Norm(q)
			}
			if !g.In(q) {
				continue
			}
			j := g.Index(q)
			step := here.away + math.Hypot(float64(off.X), float64(off.Y))
			if step >= reachOf(here.makes, gap, belt) {
				continue
			}
			if here.stay && g.Tiles[j].Plate != here.side {
				continue
			}
			if g.seam[j].found && g.seam[j].away <= step {
				continue
			}
			g.seam[j] = seam{lift: here.lift, away: step, makes: here.makes,
				found: true, side: here.side, stay: here.stay, with: here.with}
			g.seamQueue = append(g.seamQueue, int32(j))
		}
	}

	// Where each plate floats. A plate rides at its own level because of what
	// it is made of, and it carries whatever country it has on its back while
	// it does: so the whole plate is moved by what its middle is short of,
	// and not each tile by what it is short of itself. Pulling every tile
	// toward the level directly was the first way this was written, and over
	// sixteen epochs it left nothing but plains and cliffs - a tenth of a
	// slope was the ninetieth percentile of the drawn map and this could not
	// manage it at the fiftieth, because each pass took another seventh of
	// whatever texture the ground had.
	//
	// The level a plate floats at is that of the crust it carries, tile for
	// tile: a continent riding a plate that is mostly floor holds it up by
	// its share of it, and stands above the floor round it by as much as it
	// always did, because a plate is moved whole.
	var sum, count, want [256]float64
	for i := range g.Tiles {
		k := g.Tiles[i].Plate
		sum[k] += g.Height[i]
		count[k]++
		if cr.ocean[i] {
			want[k] += oceanFreeboard
		} else {
			want[k] += continentFreeboard
		}
	}
	var shift [256]float64
	for k := range plates {
		if count[k] > 0 {
			shift[k] = (want[k] - sum[k]) / count[k] * settling
		}
	}
	// Spread the step at the edge of a plate into a ramp. A continent stands
	// a quarter of a kilometre above the floor beside it, and where the two
	// meet is a margin - a shelf, and a slope down off it - rather than a
	// wall. Applied as a step, that one boundary was the steepest ground on
	// the map by a mile: on two seeds of four the steepest tenth came out at
	// three times a drawn map's, and no amount of softening the finished
	// heights could undo it, because a step a tile wide is where the height
	// actually is.
	// And the shape it rides in. The step above says where the raft floats;
	// this says how it is warped, and it is what a plate has on it that is
	// not a seam. See bowRise.
	var mean [256]float64
	for k := range plates {
		if count[k] > 0 {
			mean[k] = sum[k] / count[k]
		}
	}
	rise := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		k := g.Tiles[i].Plate
		rise[i] = shift[k] + bowPull*settling*(bow[i]-(g.Height[i]-mean[k]))
	}
	for k := 0; k < g.passes(marginRamp); k++ {
		rise = g.spread(rise)
	}

	for i := range g.Tiles {
		t := &g.Tiles[i]
		col := &g.strata[i]
		g.Height[i] += rise[i]
		cr.lifted[i] = rise[i]
		// What floats the ground up floats the beds under it with it.
		col.lift(rise[i])

		if s := g.seam[i]; s.found {
			// Eased and not cut, so that a range has feet. The drawn
			// generator learned the same thing about its upland mask: cut
			// straight, the high country began at a wall with no approach to
			// it, and the ninetieth percentile of slope came out at twice
			// what a drawn map has.
			// How wide the belt is here and how hard it is pushing are the
			// same reading, because a range that is working harder is both
			// taller and broader: a seam under a strong stretch of crust
			// raises a wide massif and one under a weak stretch raises a
			// ridge with a pass in it.
			wide := reachOf(s.makes, gap, belt) * grain[i]
			if s.away >= wide {
				continue
			}
			by := s.lift * grain[i] * profile(s.makes, s.away, wide, gap*grain[i])
			g.Height[i] += by
			cr.lifted[i] += by
			// And the book, where this is the most any meeting has done to
			// the tile. See ledger.
			g.ledger[i].meet(s.side, s.with, s.raisedBy(), epoch, by)
			// And the beds go up with it, by as much as the ground over them.
			// A belt is raised most at its axis and least at its feet, so the
			// beds on its flanks are left tipped away from it: the hogbacks
			// and the scarps of an old range are those beds, worn into.
			col.lift(by)
			if s.makes == crushed {
				// A collision does not only lift its beds, it crumples them:
				// they buckle into folds running along the belt, a few tiles
				// from crest to crest, while the ground over them rises as it
				// would have anyway. Deeper beds are folded as much as shallow
				// ones - it is the pile that buckles.
				fold := foldShare * math.Abs(by) * math.Sin(2*math.Pi*s.away/g.inTiles(foldWave))
				for k := 1; k < int(col.n); k++ {
					col.top[k] += float32(fold)
				}
			}
			if s.makes == melt && by > 0 && cr.ocean[i] {
				cr.built[i] += by
			}
			// Where the fire is, which for an arc is under the arc and not at
			// the trench six tiles in front of it. Left at the seam, moving
			// the arc inland took the granite off the map with it: what a
			// world had of it fell from eight thousand tiles to six hundred,
			// because the rock was still being recorded where the lift no
			// longer was.
			//
			// And where the crushing is, which for an arc is in front of the
			// fire and not under it: the ground between the trench and the
			// arc is squeezed by the plate going down and never gets hot,
			// while the ground under the arc gets hot and is not squeezed -
			// the paired belts of Miyashiro 1961, schist to seaward and
			// granite to landward. Recorded under the arc with the fire, the
			// crushing was always the lesser two parts in three and could not
			// once come out as rock, so a world whose continents never met
			// head on - seed 10 of the ancient terms carries its two on past
			// each other for the whole age - had no schist on it at all. It is as
			// much crushing as the arc's crest takes, since it is fed by the
			// same floor going down.
			//
			// Both bands are a few of the map's tiles wide, and on a history
			// grid coarser than the map a tile is wider than that: there a
			// tile takes the share of the band that lies across it, or every
			// band would be as wide as a tile and the rock it makes as common.
			// See bandShare.
			axis := axisOf(s.makes, gap, grain[i])
			// The book takes what a tile in the band takes, and on a coarser
			// grid notes how much of the tile the band covers: the rock is
			// decided on the map's tiles. See record.
			c := g.coarseness()
			if s.makes == arc {
				if front := g.bandShare(s.away <= math.Max(0, axis-axisWidth), s.away, 0, math.Max(0, axis*c-axisWidth)); front > 0 {
					book[i].crush += math.Abs(s.lift*grain[i]) / 3
					if g.planet > 0 {
						book[i].banded(makingCrush, front, s.away*c)
					}
				}
			}
			if share := g.bandShare(math.Abs(s.away-axis) <= axisWidth, s.away, axis*c-axisWidth, axis*c+axisWidth); share > 0 {
				if g.planet > 0 {
					k := makingCrush
					switch s.makes {
					case arc:
						k = makingPluton
					case melt:
						k = makingMelt
					}
					book[i].banded(k, share, math.Abs(s.away-axis)*c)
				}
				switch s.makes {
				case crushed:
					book[i].crush += math.Abs(by)
				case arc:
					// An arc cooks what it pushes up and melts what goes
					// under it, and it is mostly the melting: two parts fire
					// to one of crushing, where a collision is all crushing
					// and no fire at all. Split evenly, an arc could never
					// come out as anything but the crushed rock, and the
					// granite it should leave had nowhere to come from. The
					// one part of crushing is laid in front of it: see above.
					book[i].pluton += 2 * math.Abs(by) / 3
				case melt:
					book[i].melt = math.Max(book[i].melt, math.Abs(by))
					// What comes up floods what is there: a bed of lava
					// over the pile, which is the hard cap a plateau of
					// basalt stands on long after the rift has gone quiet.
					if share >= 0.5 {
						col.bury(Basalt, uint8(epoch), 0, g.Height[i], math.Abs(by))
						g.ledger[i].bury(byLava, epoch)
					}
				}
				if s.makes != nothing && share >= 0.5 {
					t.Formed = uint8(epoch)
				}
			}
		}
		g.Height[i] = math.Max(0, g.Height[i])
	}
	w.hotspot(g, book, cr, epoch)
	cr.accrete()
	// What the epoch raised, folded into how fast the ground has lately been
	// rising. See upliftMemory.
	keep := math.Exp(-epochYears / upliftMemory)
	for i, by := range cr.lifted {
		cr.rise[i] = keep*cr.rise[i] + (1-keep)*by/epochYears
	}
}

// reshape is the crust answering for itself at the end of an epoch: what has
// welded, what has grown too big to hold together, and what has been ground
// down too small to be a plate. It is where the conditions the world is held
// to are actually applied - see plateRough and the block it sits in - and it is
// the one thing the old drawing could not do, because its plate count was
// fixed at the making and nothing that happened afterwards could change it.
//
// The order matters and is the order these things happen in. Welding first,
// because two continents that have run into each other are one plate and
// everything else should be asked of the plate they now are; then the pieces
// too small to be plates, which welding has just made more of; then the ones
// too large, which welding has also just made more of.
func (w *Land) reshape(g *Grid, plates []Plate, fl *flooding, touch, weld []float64) []Plate {
	defer phase("reshape")()
	stride := len(plates)
	reach := spacing(g, standing(plates))
	scale := driftScale(g)

	area := make([]float64, plateCap)
	for i := range g.Tiles {
		area[g.Tiles[i].Plate]++
	}

	// Welding. Two continents driving into each other stop being two plates
	// when they have closed far enough for long enough: the seam between them
	// goes quiet, the range it raised is left to the weather, and what is left
	// is one continent with an old worn spine down the middle of it. That
	// spine is the whole reason for doing this - it is a mountain range with a
	// cause that is over, which is most of the mountain ranges there are.
	for a := 0; a < stride; a++ {
		for b := a + 1; b < stride; b++ {
			if weld[a*plateCap+b] < weldEnough*reach {
				continue
			}
			ra, rb := rootOf(plates, uint8(a)), rootOf(plates, uint8(b))
			if ra == rb || standing(plates) <= crustFloor {
				continue
			}
			// The bigger piece keeps its name, and the drift of the two is
			// averaged by how much crust each brings: a continent that takes
			// on a small one hardly changes course.
			if area[ra] < area[rb] {
				ra, rb = rb, ra
			}
			wa, wb := math.Max(1, area[ra]), math.Max(1, area[rb])
			plates[ra].DX = (plates[ra].DX*wa + plates[rb].DX*wb) / (wa + wb)
			plates[ra].DY = (plates[ra].DY*wa + plates[rb].DY*wb) / (wa + wb)
			plates[ra].Spin = (plates[ra].Spin*wa + plates[rb].Spin*wb) / (wa + wb)
			plates[rb].into = ra
			area[ra] += area[rb]
			area[rb] = 0
			g.welds++
		}
	}

	// Pieces too small to be plates. A sliver's seam raises a range no wider
	// than the sliver, and a world that has been drifting for a while makes
	// them faster than it makes anything else. It goes into whichever
	// neighbour it leans on hardest, and only into one of its own kind:
	// continent taken into ocean floor would sink a continent's worth of
	// country to the floor's level in a few epochs, and floor taken into
	// continent would raise an ocean.
	floor := plateFloor * float64(len(g.Tiles))
	for k := 0; k < stride; k++ {
		r := rootOf(plates, uint8(k))
		if int(r) != k || area[r] == 0 || standing(plates) <= crustFloor {
			continue
		}
		// A piece with only one neighbour is a hole in that neighbour, and it
		// goes into it whatever either of them is made of: a plate is a piece
		// of a world's skin with edges against several others, and a disc
		// adrift in the middle of one has lost every seam it had.
		if into, alone := enclosedBy(plates, touch, r, stride); alone && area[r] < enclaveMost*float64(len(g.Tiles)) {
			plates[r].into = into
			area[into] += area[r]
			area[r] = 0
			continue
		}
		if area[r] >= floor {
			continue
		}
		best, most := r, 0.0
		for j := 0; j < stride; j++ {
			o := rootOf(plates, uint8(j))
			if o == r || plates[o].Ocean != plates[r].Ocean {
				continue
			}
			a, b := r, o
			if a > b {
				a, b = b, a
			}
			if t := touch[int(a)*plateCap+int(b)]; t > most {
				best, most = o, t
			}
		}
		if best == r {
			continue // wedged among crust of the other kind; it stays
		}
		plates[r].into = best
		area[best] += area[r]
		area[r] = 0
	}
	// A plate is its tiles, so what has gone into another now rides it.
	for i := range g.Tiles {
		g.Tiles[i].Plate = rootOf(plates, g.Tiles[i].Plate)
	}

	// Pieces too large. A continent that has swallowed its neighbours has no
	// edges left inside it and nothing happening on it, so it rifts: it is
	// split in two along a ragged line across it, and the new half drives away
	// from the old, so the floor opens between them and the sea finds it.
	//
	// Both halves are made of what they were made of. Which kind the new half
	// is was once how a world was held to a share of continent, and a half
	// made ocean went on carrying the continent's country: sunk a seventh of
	// the way an epoch, its ranges stood out of the sea to the end of the
	// history as islands of granite in the middle of an ocean.
	ceiling := plateCeiling * float64(len(g.Tiles))
	for k := 0; k < stride; k++ {
		r := rootOf(plates, uint8(k))
		if int(r) != k || area[r] <= ceiling || len(plates) >= plateCap {
			continue
		}
		to := uint8(len(plates))
		plates = append(plates, Plate{Ocean: plates[r].Ocean, into: to, grow: plates[r].grow, Spin: plates[r].Spin})
		w.lean(&plates[to])
		g.locate(plates)
		whole := plates[r]
		ux, uy, ok := w.split(g, fl, plates, r, to)
		if !ok {
			plates = plates[:to]
			continue
		}
		// Each half goes on as the whole was going where the half's middle
		// is, which on a plate that turns is not how its old middle was
		// going; and the new half drives away from the old besides.
		g.locate(plates)
		plates[r].DX, plates[r].DY = g.velocity(&whole, plates[r].cx, plates[r].cy, scale)
		plates[to].DX, plates[to].DY = g.velocity(&whole, plates[to].cx, plates[to].cy, scale)
		plates[to].DX += driftFast * ux
		plates[to].DY += driftFast * uy
		area[r] /= 2
	}

	// And small pieces breaking off. Nothing above makes a plate smaller than
	// half of one too large, while welding and taking in slivers take small
	// plates away every epoch; so a world that started with a tail of small
	// plates lost it in a few ages, and ended as ten plates of much the same
	// size. On the earth small plates are made all the time, and made where
	// plates are going down under each other - the bending of the slab at a
	// trench breaks pieces off the plate above it (Mallard et al. 2016). So
	// while a world has fewer plates than it broke into, a piece breaks off
	// the edge of a plate where another is closing on it.
	if standing(plates) < plateTotal(g) && len(plates) < plateCap {
		g.locate(plates)
		plates = w.breakOff(g, fl, plates, area)
	}
	return plates
}

// breakOff breaks a small plate off the edge of a larger one, at a place on
// its edge where the plate beside it is closing: a flood from that place at a
// slow rate and one from deep in the plate at its own rate, over the plate's
// ground, and what the slow one takes is the new plate. How slow is drawn off
// the same tail the first lesser plates were: a flood that fraction as fast
// as the other, a distance off, takes a disc about that fraction of the
// distance across.
func (w *Land) breakOff(g *Grid, fl *flooding, plates []Plate, area []float64) []Plate {
	n := float64(len(g.Tiles))
	var edge []int32
	for _, i := range g.closingSeams(plates) {
		if area[g.Tiles[i].Plate] >= breakFrom*n {
			edge = append(edge, i)
		}
	}
	if len(edge) == 0 {
		return plates
	}
	at := int(edge[w.RNG.IntN(len(edge))])
	of := g.Tiles[at].Plate
	// The far side of the plate from where it breaks.
	far, deep := -1, -1.0
	for i := 0; i < len(g.Tiles); i += 3 {
		if g.Tiles[i].Plate != of {
			continue
		}
		dx := float64(i%g.W - at%g.W)
		if g.Wrap {
			dx = math.Abs(dx)
			dx = math.Min(dx, float64(g.W)-dx)
		}
		dy := float64(i/g.W - at/g.W)
		if d := dx*dx + dy*dy; d > deep {
			far, deep = i, d
		}
	}
	if far < 0 || deep <= 0 {
		return plates
	}
	share := plateLeast * math.Min(minorRange, math.Pow(1-w.RNG.Float64(), -1/minorTail))
	across := math.Sqrt(share * n / math.Pi)
	to := uint8(len(plates))
	a := 2 * math.Pi * w.RNG.Float64()
	parent := plates[of]
	plates = append(plates, Plate{
		DX:    parent.DX + driftFast*math.Cos(a),
		DY:    parent.DY + driftFast*math.Sin(a),
		Ocean: parent.Ocean,
		into:  to,
		grow:  parent.grow * math.Min(0.9, across/math.Sqrt(deep)),
		// A piece this small turns fast: see spinRim.
		Spin: parent.Spin + w.spin(g, across),
	})
	// Drawn out along the seam and not round: the flood goes slowly toward the
	// middle of the plate and quickly along its edge, so what breaks off is a
	// strip of the margin, which is the shape a piece broken off the edge of a
	// trench is. A round one was a disc bitten out of the coast.
	ax, ay := float64(far%g.W-at%g.W), float64(far/g.W-at/g.W)
	if g.Wrap && math.Abs(ax) > float64(g.W)/2 {
		ax -= math.Copysign(float64(g.W), ax)
	}
	d := math.Max(1e-9, math.Hypot(ax, ay))
	plates[to].leanX, plates[to].leanY, plates[to].stretch = -ay/d, ax/d, breakStretch
	seeds := []middle{
		{X: float64(far%g.W) + 0.5, Y: float64(far/g.W) + 0.5, at: of},
		{X: float64(at%g.W) + 0.5, Y: float64(at/g.W) + 0.5, at: to},
	}
	// Where the two floods meet depends on the shape of the plate and the
	// ground it is on as well as on the rates, so the piece is measured, and
	// put back and broken again faster, until it is near what it was drawn at.
	want := share * n
	for try := 0; ; try++ {
		g.floodOver(plates, seeds, fl, int(of))
		got := 0.0
		for i := range g.Tiles {
			if g.Tiles[i].Plate == to {
				got++
			}
		}
		if got >= want/2 || try == breakTries {
			break
		}
		for i := range g.Tiles {
			if g.Tiles[i].Plate == to {
				g.Tiles[i].Plate = of
			}
		}
		plates[to].grow *= math.Min(2, math.Sqrt(want/math.Max(1, got)))
	}
	// It goes as its parent was going where it broke off, and its own way on
	// top of that: at the rim of a turning plate that can be well off how the
	// parent's middle goes.
	g.locate(plates)
	vx, vy := g.velocity(&parent, plates[to].cx, plates[to].cy, driftScale(g))
	plates[to].DX += vx - parent.DX
	plates[to].DY += vy - parent.DY
	return plates
}

// enclosedBy is the one plate r touches, if it touches only one.
func enclosedBy(plates []Plate, touch []float64, r uint8, stride int) (uint8, bool) {
	// The touching was counted before this epoch's welds, so what r touched
	// is followed to what it is now.
	var only uint8
	n := 0
	for j := 0; j < stride; j++ {
		a, b := r, uint8(j)
		if a > b {
			a, b = b, a
		}
		o := rootOf(plates, uint8(j))
		if o == r || touch[int(a)*plateCap+int(b)] == 0 {
			continue
		}
		if n == 0 || o != only {
			only, n = o, n+1
		}
	}
	return only, n == 1
}

// split rifts plate of in two, giving the part of it on one side to plate to,
// and says which way that part lies from the rest. A line is drawn across the
// plate at random, the tiles of it furthest along that line either way are
// flooded from at the same rate over the plate's own ground, and whatever the
// far flood reaches first is the new plate - so the rift is ragged, and the
// two halves are near enough halves.
func (w *Land) split(g *Grid, fl *flooding, plates []Plate, of, to uint8) (ux, uy float64, ok bool) {
	a := 2 * math.Pi * w.RNG.Float64()
	ux, uy = math.Cos(a), math.Sin(a)
	first, lo, hi := -1, -1, -1
	low, high := math.Inf(1), math.Inf(-1)
	for i := 0; i < len(g.Tiles); i += 3 {
		if g.Tiles[i].Plate != of {
			continue
		}
		if first < 0 {
			first = i
		}
		// Measured from one tile of the plate, the short way round a globe.
		dx := float64(i%g.W - first%g.W)
		if g.Wrap {
			if dx > float64(g.W)/2 {
				dx -= float64(g.W)
			} else if dx < -float64(g.W)/2 {
				dx += float64(g.W)
			}
		}
		along := dx*ux + float64(i/g.W-first/g.W)*uy
		if along < low {
			low, lo = along, i
		}
		if along > high {
			high, hi = along, i
		}
	}
	if first < 0 || lo == hi {
		return 0, 0, false
	}
	seeds := []middle{
		{X: float64(lo%g.W) + 0.5, Y: float64(lo/g.W) + 0.5, at: of},
		{X: float64(hi%g.W) + 0.5, Y: float64(hi/g.W) + 0.5, at: to},
	}
	// The new plate floods at its parent's rate and along its parent's grain,
	// so neither half is favoured.
	plates[to].grow, plates[to].leanX, plates[to].leanY, plates[to].stretch =
		plates[of].grow, plates[of].leanX, plates[of].leanY, plates[of].stretch
	g.floodOver(plates, seeds, fl, int(of))
	return ux, uy, true
}

// axisOf is how far from the seam a meeting's fire is: under the arc, where
// what melted is cooling, and at the seam itself for everything else.
func axisOf(m made, gap, grain float64) float64 {
	if m == arc {
		return gap * grain
	}
	return 0
}

// reachOf is how far a meeting's works are carried from the seam. An arc
// stands back from the trench and needs room behind it for the country it
// raises; everything else falls away from where it happened.
func reachOf(m made, gap, belt float64) float64 {
	if m == arc {
		return belt + gap
	}
	return belt
}

// beltOn is how far from a seam, in tiles, a meeting's works reach on this
// world: beltReach at its deep span, and never under seamLeast.
func beltOn(g *Grid) float64 {
	return math.Max(g.inTiles(seamLeast), tilesAcross(beltReach, deepSpan(g)))
}

// hotspotOn is how far, in tiles, a hotspot's works reach on this world.
func hotspotOn(g *Grid) float64 {
	return math.Max(g.inTiles(seamLeast), tilesAcross(hotspotReach, deepSpan(g)))
}

// arcGapOn is how far behind the trench this world's arcs stand: the quoted
// distance, or a quarter of the way between two plate middles where that is
// less, and never under seamLeast. See arcGapReach.
func arcGapOn(g *Grid, mids int) float64 {
	gap := math.Max(g.inTiles(seamLeast), tilesAcross(arcGapReach, deepSpan(g)))
	return math.Min(gap, spacing(g, mids)/4)
}

// meeting is the hardest thing happening at tile i's edges: how fast it and
// the neighbour closing on it or parting from it fastest are closing, as a
// share of a head-on meeting of one tile an epoch, and which plate that
// neighbour rides. It is not ok where nothing is closing or parting at all.
func (g *Grid) meeting(plates []Plate, i int, scale float64) (worst float64, with uint8, ok bool) {
	worst, j, ok := g.meetingAt(plates, i, scale)
	if ok {
		with = g.Tiles[j].Plate
	}
	return worst, with, ok
}

// meetingAt is meeting, saying which tile the hardest meeting is with.
func (g *Grid) meetingAt(plates []Plate, i int, scale float64) (worst float64, at int, ok bool) {
	here := g.Tiles[i].Plate
	x, y := i%g.W, i/g.W
	for _, off := range Dirs {
		qx, qy := x+off.X, y+off.Y
		if qy < 0 || qy >= g.H {
			continue
		}
		if qx < 0 || qx >= g.W {
			if !g.Wrap {
				continue
			}
			qx = g.WrapX(qx)
		}
		k := g.Tiles[qy*g.W+qx].Plate
		if k == here {
			continue
		}
		closing := g.closing(plates, here, k, x, y, off, scale)
		if math.Abs(closing) > math.Abs(worst) {
			worst, at, ok = closing, qy*g.W+qx, true
		}
	}
	return worst, at, ok
}

// closing is how fast plates a, riding the tile at x, y, and b, riding the one
// off from it, are closing across the line between them: positive closing,
// negative parting. Both are asked how they are going at the edge the two
// tiles share, since a plate that turns goes a different way at every place
// on it.
func (g *Grid) closing(plates []Plate, a, b uint8, x, y int, off geom.Pos, scale float64) float64 {
	ex, ey := float64(x)+float64(off.X)/2, float64(y)+float64(off.Y)/2
	ax, ay := g.velocity(&plates[a], ex, ey, scale)
	bx, by := g.velocity(&plates[b], ex, ey, scale)
	return ((ax-bx)*float64(off.X) + (ay-by)*float64(off.Y)) / math.Hypot(float64(off.X), float64(off.Y))
}

// velocity is how a place on plate p at x, y is going, in the units the drift
// is quoted in. The spin is in radians an epoch and the offset in tiles, so
// what it adds is tiles an epoch and comes back to the quoted units over the
// world's scale. See Plate.
func (g *Grid) velocity(p *Plate, x, y, scale float64) (vx, vy float64) {
	rx, ry := g.across(x-p.cx), y-p.cy
	return p.DX - p.Spin*ry/scale, p.DY + p.Spin*rx/scale
}

// across is an offset along the map the short way round, where the map goes
// round.
func (g *Grid) across(dx float64) float64 {
	if g.Wrap {
		w := float64(g.W)
		dx = math.Mod(dx, w)
		if dx > w/2 {
			dx -= w
		} else if dx < -w/2 {
			dx += w
		}
	}
	return dx
}

// locate finds the middle of every plate's ground as its tiles lie now. See
// middleOf.
func (g *Grid) locate(plates []Plate) {
	g.middleOf(plates, func(i int) uint8 { return g.Tiles[i].Plate })
}

// middleOf sets every plate's middle from the plate each tile rides, as of
// says. Across a map that goes round, the middle along it is the mean of the
// tiles as angles round the world, so that a plate over the seam of the map
// has its middle on it and not on the far side of the world.
func (g *Grid) middleOf(plates []Plate, of func(i int) uint8) {
	var sx, sy, cy, n [plateCap]float64
	turn := 2 * math.Pi / float64(g.W)
	for i := range g.Tiles {
		k := of(i)
		if k == noPlate {
			continue
		}
		x := float64(i % g.W)
		if g.Wrap {
			sx[k] += math.Cos(x * turn)
			sy[k] += math.Sin(x * turn)
		} else {
			sx[k] += x
		}
		cy[k] += float64(i / g.W)
		n[k]++
	}
	for k := range plates {
		if n[k] == 0 {
			continue
		}
		p := &plates[k]
		if g.Wrap {
			a := math.Atan2(sy[k], sx[k])
			if a < 0 {
				a += 2 * math.Pi
			}
			p.cx = a / turn
		} else {
			p.cx = sx[k] / n[k]
		}
		p.cy = cy[k] / n[k]
	}
}

// profile is the shape of a belt across itself, in [0,1]: where the high
// ground of it stands and how it comes down either side.
//
// Everything but an arc is highest at the seam, because the seam is where the
// work is being done. An arc is highest a way behind it, with a plain between
// the two - so the ground goes trench, shore, flat, and then mountains, which
// is the order a coast facing a subduction actually comes in.
func profile(m made, away, wide, gap float64) float64 {
	if m != arc {
		return smooth(1 - away/wide)
	}
	if away < gap {
		return smooth(away / gap)
	}
	return smooth(1 - (away-gap)/math.Max(1e-9, wide-gap))
}

// liftOf is what a meeting does to the ground at the seam itself, in metres
// an epoch: which of the four kinds of meeting this is, times how hard. A
// negative closing is a parting.
func liftOf(mineOcean, otherOcean bool, closing float64) (float64, made) {
	switch {
	case closing <= 0:
		// They are parting: the ground drops and melt fills the axis.
		return rifting * -closing, melt
	case !mineOcean && !otherOcean:
		// Two continents. Neither will go down, so both go up, and the rock
		// in the middle of it is cooked and squeezed.
		return orogeny * closing, crushed
	case !mineOcean:
		// The floor goes under us and melts on the way: an arc of volcanoes
		// on a rising edge, half crush and half fire.
		return arcLift * closing, arc
	case !otherOcean:
		// We are the floor going under them, and what we are made of goes
		// down with us.
		return trench * closing, nothing
	default:
		// Two floors: islands come up out of open water.
		return islandUp * closing, melt
	}
}

// hotspot is melt coming up in the middle of a plate rather than at its edge,
// and it is what puts a volcano where nothing is colliding. Where they are is
// drawn once for a world and does not move, so the same places go on erupting
// age after age under whatever crust is passing over them.
func (w *Land) hotspot(g *Grid, book []record, cr *crust, epoch int) {
	g.piles()
	if g.hot == nil {
		// Two of them on a valley, and as many again for every valley's width
		// of world: a hotspot is a place and not a share, so a map a dozen
		// times as wide has a dozen times as many. Left at two, a globe had
		// two volcanic provinces in half a million tiles and its plate
		// interiors were ground that nothing had ever happened to.
		g.hot = make([]geom.Pos, max(hotspots, hotspots*g.planetSpan()/hotspotSpan))
		for i := range g.hot {
			g.hot[i] = geom.Pos{X: w.RNG.IntN(g.W), Y: w.RNG.IntN(g.H)}
		}
	}
	reach := hotspotOn(g)
	r := int(reach)
	for _, h := range g.hot {
		if w.RNG.Float64() > hotspotWakes {
			continue // quiet this age
		}
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				d := math.Hypot(float64(dx), float64(dy))
				if d > reach {
					continue
				}
				q := geom.Pos{X: h.X + dx, Y: h.Y + dy}
				if g.Wrap {
					q = g.Norm(q)
				}
				if !g.In(q) {
					continue
				}
				lift := hotspotLift * smooth(1-d/reach)
				j := g.Index(q)
				g.Height[j] += lift
				cr.lifted[j] += lift
				book[j].melt += lift
				book[j].whole[makingMelt] = true
				// The cone is lava laid on whatever was there.
				g.strata[j].lay(Basalt, g.Tiles[j].Formed, 0, g.Height[j]-lift, g.Height[j])
				// The book: a hotspot is one plate's, with no other.
				g.ledger[j].meet(g.Tiles[j].Plate, noPlate, Hotspot, epoch, lift)
				g.ledger[j].bury(byLava, epoch)
				if cr.ocean[j] {
					cr.built[j] += lift
				}
			}
		}
	}
}

// historyBase is the sea a history is running against: the lowest historySea
// of the ground as it stands, which is what the book counts as under water and
// what the air takes its fill from while there is no sea of the map's own.
func (g *Grid) historyBase() float64 {
	h := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		h[i] = g.Height[i]
	}
	return quantile(h, historySea)
}

// keepBook writes down, after an epoch of weather, what the epoch left on
// each tile: what was buried, and what lay under water. The soil's own
// make-up carries the first - the water sorted what it laid down, so a tile
// buried in sand reads as sand - and the second is simply counted.
func (g *Grid) keepBook(book []record, epoch int) {
	defer phase("keepBook")()
	g.piles()
	sea := g.base
	fill := fillRate * epochYears // metres of burial an epoch: see fillRate
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if t.Wet() || g.Height[i] <= sea {
			book[i].submerged++
			// What a sea bed gets depends on whether anything is being
			// washed into it. Off a shore there is mud, and mud makes shale;
			// out where no land is near enough to send any, the water is
			// quiet and what settles is what lived there, which makes
			// limestone. Nothing here knows how far the shore is, only
			// whether it is next door, which is enough to tell a bed that
			// silts up from one that does not. Out there it is the warmth
			// of the water that decides whether what settles is lime or
			// mud: see quietFloor.
			if g.offshore(geom.Pos{X: i % g.W, Y: i / g.W}, sea) {
				book[i].laid[Clay] += marineMud * 0.7 * fill
				book[i].laid[Silt] += marineMud * 0.3 * fill
				t.Formed = uint8(epoch)
				g.strata[i].bury(Shale, uint8(epoch), 0, g.Height[i], marineMud*bedPerFill)
				g.ledger[i].bury(byMud, epoch)
			} else {
				rock, thick := g.quietFloor(i)
				g.strata[i].bury(rock, uint8(epoch), 0, g.Height[i], thick)
				if rock == Limestone {
					g.ledger[i].bury(byLime, epoch)
				} else {
					g.ledger[i].bury(byMud, epoch)
				}
			}
			continue
		}
		// Ground below the water it drains into is ground being filled in,
		// and rock made of what is falling on it now dates from now.
		if g.Drain[i] < FloodDepth/2 {
			book[i].laid[Sand] += g.Sand[i] * fill
			book[i].laid[Silt] += g.siltAt(i) * fill
			book[i].laid[Clay] += g.Clay[i] * fill
			t.Formed = uint8(epoch)
			// The epoch's fill is a bed, coarse or fine as the water sorted
			// it. Which of the two it finally counts as is read against the
			// world's other fills at the end; see settleRock.
			rock := Shale
			if g.Sand[i] >= sandyBed {
				rock = Sandstone
			}
			g.strata[i].bury(rock, uint8(epoch), uint8(max(1, 255*clamp01(g.Sand[i]))), g.Height[i], bedPerFill)
			g.ledger[i].bury(byFill, epoch)
		}
	}
}

// quietFloor is the bed an epoch leaves on sea floor no land is near enough
// to send mud to, and how many metres thick it is.
//
// It was limestone at limeBed everywhere, from the equator to the ice, and a
// history's valley came out 37 per cent limestone and a small globe's land 30,
// the same in every band of latitude. Carbonate rock is a tenth to an eighth
// of the land (13.4 per cent, Amiotte Suchet, Probst and Ludwig 2003; 10.4 in
// the GLiM map of Hartmann and Moosdorf 2012), and where it is made is set by
// the warmth of the water. The reefs and the lime muds of the tropical
// shelves stop where the coldest month of the sea falls below about 18
// degrees (Kleypas, McManus and Meñez 1999), which is near thirty degrees of
// latitude; the cool-water carbonates of shells and bryozoa go on beyond
// that, made more slowly than a reef makes its lime, as far as the polar
// shelves (James 1997); and out in the open ocean the lime ooze of the
// plankton gives way to diatom ooze and glacial mud under the polar fronts,
// at fifty to sixty degrees.
//
// So the lime an epoch makes is limeBed where the sea's year averages
// limeWarm or more, and falls in proportion to nothing at limeCold, and it
// settles into the fine mud that reaches every floor however quietly, at
// quietMud. What is laid is limestone while lime is most of it - which is
// what limestone is, a rock more than half carbonate (Pettijohn 1975) - and
// shale once it is not. The line falls where the lime is a quarter of its
// warmest, a degree below freezing: some sixty degrees of latitude on a globe,
// the cold side of the polar front, and a valley's temperate sea at ten
// degrees keeps its limestone, seven parts lime in ten.
//
// Where between the front's fifty and sixty the line is drawn is a choice
// made on the whole of the suite. With limeCold at nothing the line lay at
// fifty-two degrees, and the small globes' land came out 14 per cent
// limestone, 27 in the tropics and none beyond sixty-seven; their drainage and
// discharge exponents came nearer the real range and their sinuosity,
// hypsometry and meander migration into it. But the one full globe's drainage
// area exponent fell from 0.409 to 0.315, out of the 0.40 to 0.46 of real
// networks. At limeCold of eight below, with the line at sixty, it reads 0.457
// over much the same shale - 24 per cent of the land against 28 - so the
// reading is the luck of where the soft rock fell on one seed more than the
// amount of it, and it is one globe: count its samples before moving it.
//
// Depth is not asked. The factory of a carbonate shelf is the lit water of its
// top hundred metres, but the plankton's ooze lays lime on the floor under it
// at about the same long rate - one to three centimetres a thousand years
// against the 0.01 to 0.1 mm a year Schlager (1981) has for shelves - down
// to the depth where the deep water dissolves it, four to five kilometres,
// and a history's quiet floors lie at a tenth of a kilometre to two and a
// half. The floor's depth would change nothing it can say.
//
// Nor are evaporites laid, the salt and gypsum of a dry, closed sea. They are
// a real rock and a climate's rock, and the arid belts are where they are
// found, but they are under half a per cent of the land, and a rock on this
// map has to show on every history's valley and make a fiftieth of them - see
// TestEveryRockAHistoryMakesTurnsUp - which a valley of one temperate sea
// could only meet by making salt where no salt is made.
func (g *Grid) quietFloor(i int) (Bedrock, float64) {
	warm := MeanTemp
	if g.air != nil && i/g.W < len(g.air.mean) {
		warm = g.air.mean[i/g.W]
	}
	lime := limeBed * clamp01((warm-limeCold)/(limeWarm-limeCold))
	mud := quietMud * epochYears
	if lime >= mud {
		return Limestone, lime + mud
	}
	return Shale, lime + mud
}

// settleRock is the history read back as geology. The beds each epoch laid
// are already in the pile under every tile - see keepBook - so what is left
// to decide is what the water's fills finally count as, what the pile stands
// on, and what the fire and the crushing have made of its lower part. Then
// every tile reads the bed its ground lies in.
func (g *Grid) settleRock(book []record, ocean []bool) {
	defer phase("settleRock")()
	g.piles()
	// Where the line between a coarse fill and a fine one falls on this
	// world, read off its own fills rather than fixed. See coarseShare. It is
	// read off each tile's whole fill and not bed by bed: what one epoch's
	// fill is made of hardly differs from the next, and the difference the
	// sorting makes is between one basin and another.
	sandy := make([]float64, 0, len(g.Tiles))
	for i := range g.Tiles {
		if fill := carrying(book[i].laid); fill > fillEnough {
			sandy = append(sandy, book[i].laid[Sand]/fill)
		}
	}
	coarse := math.Inf(1)
	if len(sandy) > 0 {
		coarse = quantile(sandy, 1-coarseShare)
	}

	for i := range g.Tiles {
		c := &g.strata[i]
		b := book[i]
		fill := carrying(b.laid)
		// Coarse fill is the near end of a basin, where what came off the
		// hill did not travel far before it was dropped; fine fill is what did
		// travel. The mud off a shore is shale whatever it is ranked against,
		// and is laid with no sand to say so.
		river := Shale
		if fill > 0 && b.laid[Sand]/fill >= coarse {
			river = Sandstone
		}
		for k := 0; k+1 < int(c.n); k++ {
			if r := c.rock[k]; (r == Sandstone || r == Shale) && c.sand[k] > 0 {
				c.rock[k] = river
			}
		}
		if g.planet > 0 {
			continue // the foot is laid on the map: see handDown
		}
		g.cookFoot(i, b, ocean[i])
	}
	g.expose()
}

// cookFoot lays the foot of tile i's pile, and remakes what lies over it, by
// what the book says was done to it. See settleRock.
func (g *Grid) cookFoot(i int, b record, ocean bool) {
	t := &g.Tiles[i]
	c := &g.strata[i]
	fill := carrying(b.laid)
	// The foot of the pile is what the crust itself is, and the fire and
	// the crushing remake what lies over it. The order is the order that
	// decides it: what came up as melt is what it is, whatever was done to
	// it after; short of that, what an arc melted at depth; short of that,
	// what was cooked and squeezed by a collision; and short of everything
	// at all, basalt if it is ocean floor, granite if it is the old body
	// of a continent. Where the fire or the crushing outweighs everything
	// the water laid, it has remade the whole pile; where it does not, only
	// the beds it buried deep enough.
	foot := &c.rock[c.n-1]
	switch {
	case b.melt > madeEnough && b.melt > b.crush && b.melt > b.pluton:
		*foot = Basalt
	case b.pluton > madeEnough && b.pluton > b.crush:
		// It melted under an arc and cooled at depth, and the weather has
		// since taken off what stood over it. This is where granite comes
		// from, and saying so is what gave the rock a place on the map at
		// all: as the leftover case - ground nothing ever happened to - it
		// never came up once in sixteen epochs, because something happens
		// to everything.
		below := g.Height[i] - plutonDepth
		if b.pluton > fill {
			below = math.Inf(1)
		}
		c.cook(below, Granite, t.Formed)
	case b.crush > madeEnough:
		below := g.Height[i] - cookDepth
		if b.crush > fill {
			below = math.Inf(1)
		}
		c.cook(below, Schist, t.Formed)
	case ocean:
		*foot = Basalt
	default:
		// The old body of a continent, which almost nothing on a made map
		// is: over an age every tile is remade, or buried, or spends half
		// its life under the sea this history floods itself to. It is left
		// standing as the statement of what the leftover is, and it should
		// be read as an admission: a continental shield showing through is
		// a thing this model cannot make, and letting old crust survive an
		// age is the change that would.
		*foot = Granite
	}
	c.tidy()
}

// normalise brings a history's relief back to the scale the rest of the world
// is built to, and this is the join between a world that made itself and a
// world that has to be liveable. It is the part most likely to be wrong, so
// it is here in the open rather than folded into the passes above.
//
// The history says where the high ground is; the drawn map says how high a
// map's ground is spread. Every tile keeps its place in the order - the
// hundredth-highest tile of a history is the hundredth-highest tile of the
// map that comes out - and takes the height the drawn generator would have
// put at that place in its own order. So the causes are the history's and
// the scale is the one every constant downstream was measured against: the
// sixty metres of lowland, the high country standing on a fifth of it,
// fourteen metres to the top of a flood plain.
//
// Stretching each band onto its own range was the first way this was written,
// and it made plains and cliffs: the lowland was squeezed four to one while
// the mountains were left at one to one, so half the map came out at a slope
// of a hundredth and the steepest tenth at twice what a drawn map has.
// Matching the whole spread rather than its ends is what fixed that.
func (w *Land) normalise(g *Grid) {
	spread := w.relief(g)
	sort.Float64s(spread)

	order := make([]int32, len(g.Tiles))
	for i := range order {
		order[i] = int32(i)
	}
	sort.Slice(order, func(a, b int) bool {
		ha, hb := g.Height[order[a]], g.Height[order[b]]
		if ha != hb {
			return ha < hb
		}
		return order[a] < order[b] // ties by position, so a world repeats
	})
	for rank, i := range order {
		g.Height[i] = spread[rank]
	}
}

// soften eases the finished ground: the same blur the river valleys are cut
// with, run over the heights. It is the one pass here that is not a process -
// nothing in the earth averages a hillside with its neighbours - and it is
// here because a history raises narrower ranges than a drawn map does. What
// it costs is sharpness and not scale, since the matching afterwards hands
// out the same heights either way. See smoothing.
func (g *Grid) soften() {
	h := make([]float64, len(g.Tiles))
	for i := range g.Tiles {
		h[i] = g.Height[i]
	}
	h = g.spread(h)
	for i := range g.Tiles {
		if g.strata != nil {
			g.strata[i].lift(h[i] - g.Height[i])
		}
		g.Height[i] = h[i]
	}
}

// offshore reports whether any of the eight tiles around p stands above the
// sea. It is how a sea bed is told from a shore: what is next to land gets
// what the land sends it.
func (g *Grid) offshore(p geom.Pos, sea float64) bool {
	for _, off := range Dirs {
		q := geom.Pos{X: p.X + off.X, Y: p.Y + off.Y}
		if g.Wrap {
			q = g.Norm(q)
		}
		if g.In(q) && !g.At(q).Wet() && g.Height[g.Index(q)] > sea {
			return true
		}
	}
	return false
}
