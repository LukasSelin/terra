package terra

import (
	"math"

	"github.com/LukasSelin/terra/geom"
)

// Why: the world answering for itself.
//
// Why(p, aspect) is a chain of causes for one thing about one tile - its
// height, its rock, its rain, its cover - read off what the passes wrote
// down: the book a history kept (ledger.go), the beds under the tile
// (strata.go), the air's budget (vapour.go) and the readings the woods were
// scored on (woods.go). Every cause carries the number the pass computed
// and the unit it computed it in. Nothing here runs a model or guesses: an
// explanation is a reading of recorded quantities, so it is as true as the
// world and no truer, and the same seed gives the same chain.
//
// It is structured first - a game renders the chain its own way, a test
// asserts the kinds in order, and a game may append its own causes (why the
// settlement is here) to the world's - and the sentence is a renderer over
// it; cmd/overview has one.
//
// What is not claimed: a rain shadow. The recorded budget says what the
// ground's lift wrung out of each phase's air and how far the sea lies
// upwind, and the chain reports those; whether the lee is dry because a
// range took the water is a rule over the record (the plan's P2), not a
// reading of it, and it is not made here.

// Aspect is what about a tile is being asked.
type Aspect uint8

const (
	OfHeight Aspect = iota // why the ground stands where it does
	OfRock                 // why this rock is at the surface
	OfRain                 // why this much rain falls
	OfCover                // why the ground suits trees as well as it does
	aspects
)

var aspectNames = [aspects]string{"height", "rock", "rain", "cover"}

func (a Aspect) String() string {
	if int(a) < len(aspectNames) {
		return aspectNames[a]
	}
	return "aspect?"
}

// CauseKind is what kind of cause a link in the chain is.
type CauseKind uint8

const (
	NoCause CauseKind = iota

	// Height. RaisedBy is the meeting that did most to the tile, with the
	// belt it makes: Quantity the metres, in the history's own (see
	// ledger), When the epoch; Note names the kind of meeting. BetweenPlate
	// follows it once for each plate of the meeting, with the plate's
	// feature. WornSince is the metres the history's weather took off the
	// tile after that meeting, in the same metres. Stands is the tile's
	// height on the map, with the plate it rides where there is one.
	RaisedBy
	BetweenPlate
	WornSince
	Stands

	// Rock. BedOf is the bed the surface lies in: Note the rock, Quantity
	// its thickness - nothing for the basement, which has none - When the
	// epoch it was laid on a made world. One of
	// LaidAsFill, LaidAsMud, LaidAsLime, LaidAsLava, Cooked, MeltedAtDepth
	// and OceanFloor follows, which is what that rock is in this world:
	// what the water laid, coarse or fine as it sorted it; mud off a shore;
	// what lived in a quiet warm sea; a rift's or a hotspot's lava; a
	// collision's schist; an arc's granite; the basalt the crust was.
	// BuriedLast, on a made world, is the last burial the book has for the
	// tile, whether or not it is still at the surface: Note the kind, When
	// the epoch.
	BedOf
	LaidAsFill
	LaidAsMud
	LaidAsLime
	LaidAsLava
	Cooked
	MeltedAtDepth
	OceanFloor
	BuriedLast

	// Rain. RainOf is the tile's year of rain, with its climate region;
	// LatitudeRain the mean over its row of the map. Orographic is what the
	// ground's lift wrung out of the air over the tile's cell in the phase
	// of the year that carries most of the cell's water, as the budget kept
	// it, a year's worth of it; Note names the phase. UpwindSea is how far
	// the sea lies upwind of the cell in that phase's wind, in kilometres,
	// over the cells the air read.
	RainOf
	LatitudeRain
	Orographic
	UpwindSea

	// Cover. Suits is WoodsAt, with the climate region. The rest are the
	// readings it was made from, each with its number: Slope against the
	// limit a wood holds on; Treeless above the tree line and Barren under
	// the ice, where they hold; Warmth the year's mean at the tile;
	// Evaporation what the air could take up; Wetness the tile's
	// topographic wetness against the map's mean; WaterRatio the water the
	// tile has against what a forest needs; Drains how far the tile stands
	// over the water it drains into; SoilDepth the soil over its rock.
	Suits
	Slope
	Treeless
	Barren
	Warmth
	Evaporation
	Wetness
	WaterRatio
	Drains
	SoilDepth

	causeKinds
)

var causeNames = [causeKinds]string{
	"no cause", "raised by", "between plate", "worn since", "stands",
	"bed of", "laid as fill", "laid as mud", "laid as lime", "laid as lava", "cooked", "melted at depth", "ocean floor", "buried last",
	"rain of", "latitude rain", "orographic", "upwind sea",
	"suits", "slope", "treeless", "barren", "warmth", "evaporation", "wetness", "water ratio", "drains", "soil depth",
}

func (k CauseKind) String() string {
	if int(k) < len(causeNames) {
		return causeNames[k]
	}
	return "cause?"
}

// Cause is one link in a chain: the feature it is about, if any; what kind
// of cause; the number the pass computed and its unit; when, in years before
// the present, or nothing where the cause has no time; and a note for the
// renderer, which never carries a number the Quantity does not.
type Cause struct {
	Feature  FeatureID
	Kind     CauseKind
	Quantity float64
	Unit     string
	When     float64
	Note     string
}

// Why is the chain of causes for the given aspect of the tile at p, first
// cause first, or nothing off the map.
func (g *Grid) Why(p geom.Pos, a Aspect) []Cause {
	if !g.In(p) {
		return nil
	}
	i := g.Index(p)
	switch a {
	case OfHeight:
		return g.whyHeight(i)
	case OfRock:
		return g.whyRock(i)
	case OfRain:
		return g.whyRain(i)
	case OfCover:
		return g.whyCover(i, p)
	}
	return nil
}

// yearsAgo is how long before the present epoch e began: the last epoch of
// a history of n is the one epochYears before now.
func (g *Grid) yearsAgo(e uint8) float64 {
	return float64(int(g.epochs)-int(e)) * epochYears
}

func (g *Grid) whyHeight(i int) []Cause {
	var chain []Cause
	plate := FeatureID(0)
	if g.ledger != nil {
		plate = g.featureAt(i, CrustPlate)
		l := &g.ledger[i]
		if l.raised() != NoMeeting {
			chain = append(chain, Cause{
				Feature: g.featureAt(i, UpliftBelt), Kind: RaisedBy,
				Quantity: float64(l.lift), Unit: "m", When: g.yearsAgo(l.epoch),
				Note: l.raised().String(),
			})
			for _, k := range l.plates {
				if k == noPlate {
					continue
				}
				chain = append(chain, Cause{Feature: g.PlateOf(k), Kind: BetweenPlate, Quantity: float64(k), Note: "plate number as the history wrote it"})
			}
			chain = append(chain, Cause{Kind: WornSince, Quantity: l.wornMetres(), Unit: "m"})
		}
	}
	note := ""
	switch {
	case g.ledger == nil:
		note = "a drawn map"
	case len(chain) == 0:
		note = "no meeting raised it, so it stands at its plate's level"
	}
	return append(chain, Cause{Feature: plate, Kind: Stands, Quantity: g.Height[i], Unit: "m", Note: note})
}

func (g *Grid) whyRock(i int) []Cause {
	t := &g.Tiles[i]
	rock, formed, thick, basement := t.Bedrock, t.Formed, math.Inf(1), true
	if g.strata != nil {
		c := &g.strata[i]
		rock, formed = c.rock[0], c.formed[0]
		if c.n > 1 {
			thick, basement = float64(c.top[0]-c.top[1]), false
		}
	}
	bed := Cause{Kind: BedOf, Quantity: thick, Unit: "m", Note: rock.String()}
	if basement {
		// The basement goes down for ever: no thickness, and said so.
		bed.Quantity, bed.Note = 0, rock.String()+", the basement"
	}
	if g.ledger != nil {
		bed.When = g.yearsAgo(formed)
	}
	chain := []Cause{bed}
	var how CauseKind
	switch rock {
	case Sandstone:
		how = LaidAsFill
	case Shale:
		how = LaidAsMud
		if g.strata != nil && g.strata[i].sand[0] > 0 {
			how = LaidAsFill
		}
	case Limestone:
		how = LaidAsLime
	case Basalt:
		how = LaidAsLava
		if basement {
			how = OceanFloor
		}
	case Schist:
		how = Cooked
	case Granite:
		how = MeltedAtDepth
	}
	if how != NoCause {
		chain = append(chain, Cause{Kind: how, Note: rock.String()})
	}
	if g.ledger != nil {
		if l := &g.ledger[i]; l.buried() != unburied {
			chain = append(chain, Cause{Kind: BuriedLast, When: g.yearsAgo(l.buriedIn), Note: buriedNames[l.buried()]})
		}
	}
	return chain
}

var buriedNames = [...]string{"nothing", "a river's fill", "mud off a shore", "lime of a quiet sea", "lava"}

// phaseNamesOfYear names the four phases of the year the wind and the rain
// are worked out in, by the sun: see wind.go.
var phaseNamesOfYear = [phases]string{"the northern midwinter quarter", "the spring quarter", "the northern midsummer quarter", "the autumn quarter"}

func (g *Grid) whyRain(i int) []Cause {
	if len(g.rain) != len(g.Tiles) {
		return nil
	}
	chain := []Cause{{Feature: g.featureAt(i, ClimateRegion), Kind: RainOf, Quantity: g.rain[i], Unit: "mm"}}
	y := i / g.W
	row := 0.0
	for j := y * g.W; j < (y+1)*g.W; j++ {
		row += g.rain[j]
	}
	chain = append(chain, Cause{Kind: LatitudeRain, Quantity: row / float64(g.W), Unit: "mm"})

	w := g.winds
	if w == nil || w.airEnv == nil {
		return chain
	}
	e := w.airEnv
	c := e.cellOfTile(g, i)
	if len(w.budget[1].rain) <= c {
		return chain
	}
	// The phase that carries most of the cell's water.
	k, most := 0, -1.0
	for ph := range phases {
		if len(w.budget[ph].rain) <= c || len(w.budget[ph].oro) <= c {
			continue
		}
		if r := w.budget[ph].rain[c] + w.budget[ph].oro[c]; r > most {
			k, most = ph, r
		}
	}
	if most < 0 {
		return chain
	}
	chain = append(chain, Cause{Kind: Orographic, Quantity: w.budget[k].oro[c] * secondsPerYear, Unit: "mm", Note: phaseNamesOfYear[k]})
	if d, ok := g.upwindSea(e, w.u[k], w.v[k], c); ok {
		chain = append(chain, Cause{Kind: UpwindSea, Quantity: d, Unit: "km", Note: phaseNamesOfYear[k]})
	}
	return chain
}

// upwindSea is how far, in kilometres, the sea lies upwind of cell c under
// the wind u, v: the cells walked back against the wind until one is more
// than half sea, or the air's rows run out. A cell already more than half
// sea is nothing away; a wind of nothing has no upwind.
func (g *Grid) upwindSea(e *airEnv, u, v []float32, c int) (float64, bool) {
	if len(e.sea) <= c || len(u) <= c || len(v) <= c {
		return 0, false
	}
	if e.sea[c] > 0.5 {
		return 0, true
	}
	uu, vv := float64(u[c]), float64(v[c])
	speed := math.Hypot(uu, vv)
	if speed < 1e-6 {
		return 0, false
	}
	// Against the wind, a cell at a time: north is up the rows, so v goes
	// against cy.
	dx, dy := -uu/speed, vv/speed
	fx, fy := float64(c%e.w)+0.5, float64(c/e.w)+0.5
	km := 0.0
	for steps := 0; steps < 2*(e.w+e.h); steps++ {
		cy := int(fy)
		if cy < 0 || cy >= e.h {
			return km, false
		}
		km += math.Hypot(dx*e.dx[cy], dy*e.dy) / 1000
		fx, fy = fx+dx, fy+dy
		cx, cy := int(math.Floor(fx)), int(math.Floor(fy))
		if cy < 0 || cy >= e.h {
			return km, false
		}
		if e.wrap {
			cx = ((cx % e.w) + e.w) % e.w
		} else if cx < 0 || cx >= e.w {
			return km, false
		}
		if e.sea[cy*e.w+cx] > 0.5 {
			return km, true
		}
	}
	return km, false
}

func (g *Grid) whyCover(i int, p geom.Pos) []Cause {
	chain := []Cause{{Feature: g.featureAt(i, ClimateRegion), Kind: Suits, Quantity: g.WoodsAt(p)}}
	if !g.woodsRead {
		g.readWoods()
	}
	slope := g.Slope(p)
	if g.climateWoods {
		chain = append(chain, Cause{Kind: Slope, Quantity: slope, Unit: "m/m", Note: "the limit soil holds on is " + ftoa(soilCritical)})
		if g.Treeless(p) {
			chain = append(chain, Cause{Kind: Treeless})
		}
		if g.Barren(p) {
			chain = append(chain, Cause{Kind: Barren})
		}
		if len(g.warm) == len(g.Tiles) {
			chain = append(chain, Cause{Kind: Warmth, Quantity: g.meanOn(i, g.Height[i]), Unit: "°C"})
		}
		if i < len(g.rain) && g.air != nil {
			chain = append(chain,
				Cause{Kind: RainOf, Quantity: g.rain[i], Unit: "mm"},
				Cause{Kind: Evaporation, Quantity: g.pet(i), Unit: "mm"})
			if g.twiMean > 0 {
				chain = append(chain, Cause{Kind: Wetness, Quantity: g.twi(i) / g.twiMean})
			}
			chain = append(chain, Cause{Kind: WaterRatio, Quantity: g.waterRatio(i), Note: "a half at " + ftoa(1/climateLine)})
		}
		return append(chain, Cause{Kind: SoilDepth, Quantity: float64(g.Soil[i]), Unit: "m"})
	}
	return append(chain,
		Cause{Kind: Drains, Quantity: g.Drain[i], Unit: "m", Note: "damp down to nothing at " + ftoa(2*FloodDepth) + " m"},
		Cause{Kind: Slope, Quantity: slope, Unit: "m/m", Note: "against the map's steep ground at " + ftoa(g.steepAt) + " and its line at " + ftoa(g.steepLine)})
}

// ftoa is a number for a note, short.
func ftoa(v float64) string {
	switch {
	case v == math.Trunc(v) && math.Abs(v) < 1e6:
		return itoa(int(v))
	case math.Abs(v) >= 100:
		return itoa(int(math.Round(v)))
	}
	// Two decimals, by hand, so that the package stays what it is.
	neg := v < 0
	if neg {
		v = -v
	}
	whole := int(v)
	frac := int(math.Round((v - float64(whole)) * 100))
	if frac == 100 {
		whole, frac = whole+1, 0
	}
	s := itoa(whole) + "."
	if frac < 10 {
		s += "0"
	}
	s += itoa(frac)
	if neg {
		return "-" + s
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	k := len(b)
	for n > 0 {
		k--
		b[k] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		k--
		b[k] = '-'
	}
	return string(b[k:])
}
