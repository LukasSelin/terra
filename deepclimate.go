package terra

import (
	"math"
	"sync"

	"github.com/LukasSelin/terra/internal/atmos"
	"github.com/LukasSelin/terra/internal/phase"
)

// The climate of each epoch: the prototype of X1's cost study
// (docs/deep-time-climate.md, #57).
//
// A history erodes its sixteen epochs under today's air. The winds and the
// rain are read again on each epoch's ground already - the drainage asks for
// the weather whenever the ground has moved far enough, and an epoch of the
// plates always has - but the warmth they are read under, g.air.Mean, is the
// energy balance of a planet three tenths land in every band, today's, and
// the evaporation, the rain the vapour budget gives and the lime on a warm
// sea floor all read it. Here each epoch's warmth is its own: the balance of
// zonal.go under the epoch's land share, band by band, so that a continent
// at a pole cools its epoch.
//
// It is a study and not a world. deepClimate is off, and nothing but the
// study's test turns it on; with it off a history is what it was, to the bit
// (TestWorldDigest).
var deepClimate deepClimateTerms

// deepClimateTerms are how the prototype works an epoch's climate out. The
// zero value is off.
type deepClimateTerms struct {
	// on runs the prototype at all.
	on bool
	// every is how many epochs one climate lasts: one works out every
	// epoch's, two every other one's, the epochs between keeping the last.
	every int
	// years is how many years of the balance an epoch runs: ZonalYears from
	// a uniform start, or fewer carried on from the last epoch's (warm).
	years int
	warm  bool
	// shift reads no balance at all: today's, moved band by band by how far
	// the band's land share stands from today's, at a sensitivity worked out
	// once (see shiftSensitivity).
	shift bool
	// coarser makes the air's cells this many times wider, while the history
	// runs; one or nought leaves them.
	coarser int
	// keepWeather does not ask for the weather again when the climate
	// changes: the epoch's warmth waits for the ground to move it.
	keepWeather bool
	// forcing is how many W/m² more than today the air holds back, every
	// epoch: a stand-in for A0's carbon and orbit (X2 sets it per epoch).
	forcing float64
	// today keeps today's climate, for a variant that changes only the air's
	// cells.
	today bool
	// nudge is added to today's warmth everywhere, where today is set: the
	// least change, which measures how far any change moves a history.
	nudge float64
	// watch, where it is set, is told of each epoch's climate as it is laid.
	watch func(g *Grid, epoch int, land, sea *[atmos.ZonalBands]float64, air *Air)
}

// epochClimate is the prototype's state over one history.
type epochClimate struct {
	terms deepClimateTerms
	today *Air
	last  *atmos.ZonalYear
	air   *Air
}

// newEpochClimate is the prototype over a history on g, or nil where it is
// off or g is not a globe (a valley's air is one latitude's).
func newEpochClimate(g *Grid) *epochClimate {
	if !deepClimate.on || !g.Wrap || g.air == nil {
		return nil
	}
	c := &epochClimate{terms: deepClimate, today: g.air}
	if c.terms.every < 1 {
		c.terms.every = 1
	}
	if c.terms.years < 1 {
		c.terms.years = atmos.ZonalYears
	}
	if c.terms.coarser > 1 {
		atmos.SetCellCoarsen(c.terms.coarser)
	}
	return c
}

// epoch lays epoch e's climate on g, whose ground and sea level are the
// epoch's.
func (c *epochClimate) epoch(g *Grid, e int) {
	if (c.terms.today && c.terms.nudge == 0) || e%c.terms.every != 0 {
		return
	}
	defer phase.Start("deepClimate")()
	land, sea := landBands(g)
	a := *c.today
	a.Mean = make([]float64, g.H)
	if c.terms.today {
		for y := range a.Mean {
			a.Mean[y] = c.today.Mean[y] + c.terms.nudge
		}
	} else if c.terms.shift {
		today, sens := shiftSensitivity()
		for y, lat := range a.Lat {
			k := zonalBand(lat)
			a.Mean[y] = today.Mean(lat) + sens[k]*(land[k]-todayLand)
		}
	} else {
		stop := phase.Start("deepClimate.ebm")
		var from *atmos.ZonalYear
		years := atmos.ZonalYears
		if c.terms.warm && c.last != nil {
			from, years = c.last, c.terms.years
		}
		z := atmos.SolveZonalForced(&land, &sea, years, from, c.terms.forcing)
		stop()
		c.last = z
		for y, lat := range a.Lat {
			a.Mean[y] = z.Mean(lat)
		}
	}
	c.air = &a
	g.air = &a
	if !c.terms.keepWeather {
		g.aired = g.aired[:0] // the weather is read again under it: see weatherStale
	}
	if c.terms.watch != nil {
		c.terms.watch(g, e, &land, &sea, &a)
	}
}

// done gives g today's air back once the history is over, and has the
// weather read again under it.
func (c *epochClimate) done(g *Grid) {
	g.air = c.today
	g.aired = g.aired[:0]
	if c.terms.coarser > 1 {
		atmos.SetCellCoarsen(1)
	}
}

// todayLand is the land share of every band of today's balance.
var todayLand = func() float64 { l, _ := atmos.TodaysLand(); return l[0] }()

// zonalBand is the balance's band at a latitude in degrees.
func zonalBand(lat float64) int {
	x := math.Sin(lat * math.Pi / 180)
	return clampInt(int((x+1)/2*atmos.ZonalBands), 0, atmos.ZonalBands-1)
}

// landBands is the share of each of the balance's bands that is above g's
// sea: each row's share spread over the bands its span of latitude covers,
// by how much of each it covers in the sine of the latitude, which is area.
func landBands(g *Grid) (land, sea [atmos.ZonalBands]float64) {
	var w [atmos.ZonalBands]float64
	half := 90 / float64(g.H)
	for y := 0; y < g.H; y++ {
		var up int
		for i := y * g.W; i < (y+1)*g.W; i++ {
			if !g.sunk(i) {
				up++
			}
		}
		share := float64(up) / float64(g.W)
		lat := g.air.Lat[y]
		x0 := math.Sin(math.Max(-90, lat-half) * math.Pi / 180)
		x1 := math.Sin(math.Min(90, lat+half) * math.Pi / 180)
		for k := zonalBand(lat - half); k <= zonalBand(lat+half); k++ {
			b0 := -1 + 2*float64(k)/atmos.ZonalBands
			b1 := b0 + 2.0/atmos.ZonalBands
			if o := math.Min(x1, b1) - math.Max(x0, b0); o > 0 {
				land[k] += share * o
				w[k] += o
			}
		}
	}
	for k := range land {
		if w[k] > 0 {
			land[k] /= w[k]
		} else {
			land[k] = todayLand
		}
		sea[k] = 1 - land[k]
	}
	return land, sea
}

// shiftSensitivity is today's balance and how many degrees each band's mean
// moves for each whole of its land share: read off a second balance with
// every band's land share doubled. A band's warmth is not its own land's
// alone - the air carries heat between them - so this is the cheap variant's
// approximation, and the study reads how far it falls from the solve.
var shiftSensitivity = sync.OnceValues(func() (*atmos.ZonalYear, [atmos.ZonalBands]float64) {
	land, sea := atmos.TodaysLand()
	today := atmos.SolveZonal(&land, &sea, atmos.ZonalYears, nil)
	for k := range land {
		land[k], sea[k] = 2*todayLand, 1-2*todayLand
	}
	more := atmos.SolveZonal(&land, &sea, atmos.ZonalYears, nil)
	var s [atmos.ZonalBands]float64
	for k := range s {
		lat := math.Asin(-1+(float64(k)+0.5)*2/atmos.ZonalBands) * 180 / math.Pi
		s[k] = (more.Mean(lat) - today.Mean(lat)) / todayLand
	}
	return today, s
})
