package atmos

import (
	"fmt"
	"math"
	"sync"
)

// What drives the energy balance from outside the planet's air and ground:
// the sun's flux, the orbit that brings it nearer and farther and the tilt
// that shares it between the hemispheres, and the carbon dioxide in the air
// that holds back some of the heat the ground sends to space.
//
// They used to be written into ebm.go as today's: an obliquity of 0.409
// radians, the 0.033 of FAO-56's inverse relative distance, the sun of Kopp
// and Lean, and an outgoing longwave with no carbon in it at all. An epoch, or
// another planet, could have none of its own. Here they are a Forcing, which
// the planet's terms carry, and the balance is worked out once for each one
// it is asked under.

// Forcing is the sun, the orbit and the air's carbon a planet's climate is
// worked out under. The zero Forcing is today's, all of it: see Today. One
// that is given is taken as given, every field of it, so a change to one
// figure starts from Today.
type Forcing struct {
	// CO2 is the carbon dioxide in the air, in parts per million by volume.
	// It moves the outgoing longwave by -5.35 ln(CO2/co2Reference) W/m²
	// (Myhre, Highwood, Shine and Stordal, 1998), nothing at the reference.
	CO2 float64
	// Eccentricity is the orbit's. The daily sun reads it to first order, as
	// FAO-56's inverse relative distance does: 1 + 2e cos of the day's angle
	// from perihelion.
	Eccentricity float64
	// Obliquity is the tilt of the spin to the orbit, in radians.
	Obliquity float64
	// Perihelion is the longitude of perihelion measured from the moving
	// vernal equinox, in radians, as Berger (1978) tabulates it: the
	// heliocentric one, so that the sun stands at Perihelion plus π, seen
	// from the ground, on the day the planet is nearest it. Today's is some
	// 102 degrees, which puts perihelion in early January: see insolation
	// for how it falls in the calendar.
	Perihelion float64
	// Solar is the sun's flux at the planet's mean distance, W/m².
	Solar float64
}

// Today's forcing: the orbit of 1950, from Berger's (1978) series, and the
// sun of Kopp and Lean. Before the calendar was one (see insolation) they
// were FAO-56's rounding of it - an obliquity of 0.409 radians, an
// eccentricity of 0.0165 and perihelion on the first of January - kept so
// that the world stayed what it was to the bit. The world moved with the
// calendar, and today's orbit is the real one; Today().OrbitBefore(0) is
// Today's to the last few digits of the series.
const (
	// todayObliquity is 23.446 degrees.
	todayObliquity = 0.409214631315809
	// todayEccentricity is the orbit's eccentricity in 1950.
	todayEccentricity = 0.0167239329967327
	// todayPerihelion is the longitude of perihelion in 1950, 102.04
	// degrees: the planet nearest the sun some seventy-eight days of the
	// calendar before the spring equinox, in early January.
	todayPerihelion = 1.78091737968827
	// co2Reference is the air Budyko's line was fitted under: North and
	// Coakley's A and B are from the satellites of the 1970s (Ellis and Vonder
	// Haar, 1976), when the air held some 330 parts per million (Keeling's
	// Mauna Loa record: 325 in 1970, 339 in 1980). The pre-industrial air was
	// 280, and a world asked for at 280 is a little the colder for it.
	co2Reference = 330.0
	// co2Forcing is Myhre et al.'s (1998) 5.35 W/m² for each e-folding of
	// the carbon in the air.
	co2Forcing = 5.35
)

// Today is the forcing a world is made under when its terms ask for none:
// today's sun and orbit, and the air Budyko's line was fitted to. See the
// constants above.
func Today() Forcing {
	return Forcing{
		CO2:          co2Reference,
		Eccentricity: todayEccentricity,
		Obliquity:    todayObliquity,
		Perihelion:   todayPerihelion,
		Solar:        solarConstant,
	}
}

// OrDefault is f, or Today where f is the zero Forcing.
func (f Forcing) OrDefault() Forcing {
	if f == (Forcing{}) {
		return Today()
	}
	return f
}

// Check reports what is wrong with f as a forcing a balance can be worked out
// under, or nil. The zero Forcing is today's and is fine.
func (f Forcing) Check() error {
	if f == (Forcing{}) {
		return nil
	}
	ok := func(v, lo, hi float64) bool { return v >= lo && v <= hi } // false for NaN
	switch {
	case !ok(f.CO2, math.SmallestNonzeroFloat64, 1e6):
		return forcingError("carbon dioxide", f.CO2, "over nothing and at most a million parts per million")
	case !ok(f.Eccentricity, 0, 0.2):
		return forcingError("eccentricity", f.Eccentricity, "from 0 to 0.2, beyond which the daily sun's first-order distance is no reading of the orbit")
	case !ok(f.Obliquity, 0, math.Pi/2):
		return forcingError("obliquity", f.Obliquity, "from 0 to π/2 radians")
	case math.IsNaN(f.Perihelion) || math.IsInf(f.Perihelion, 0):
		return forcingError("longitude of perihelion", f.Perihelion, "a finite angle")
	case !ok(f.Solar, math.SmallestNonzeroFloat64, 1e5):
		return forcingError("solar constant", f.Solar, "over nothing and at most 100000 W/m²")
	}
	return nil
}

func forcingError(what string, v float64, want string) error {
	return fmt.Errorf("a forcing's %s must be %s: %v", what, want, v)
}

// olrA is the outgoing longwave's constant under f: Budyko's A, less what the
// carbon in the air holds back. At the reference the logarithm is nought to
// the bit and A is A.
func (f Forcing) olrA() float64 {
	return olrA - co2Forcing*math.Log(f.CO2/co2Reference)
}

// The balances worked out so far, one for each forcing they were asked
// under. Today's has a slot of its own, since nearly every world asks for
// nothing else and asks for it from every goroutine at once; any other is
// kept in a map by the bits of its figures, each worked out once by
// whichever goroutine asked first while the rest wait for it. A balance is a
// function of its forcing and nothing else, so which goroutine works it out
// makes no difference to it. What is kept is a few kilobytes a forcing and is
// never let go.
var (
	ebmOnce sync.Once
	ebmOut  *ebmClimate
	ebmMore sync.Map // forcingKey → *ebmEntry
)

type ebmEntry struct {
	once sync.Once
	out  *ebmClimate
}

// forcingKey is a forcing by the bits of its figures: two forcings are one
// balance when every figure is the same number, and a NaN, which Check turns
// away, is at least the same key each time it is asked for.
type forcingKey [5]uint64

func (f Forcing) key() forcingKey {
	return forcingKey{
		math.Float64bits(f.CO2), math.Float64bits(f.Eccentricity), math.Float64bits(f.Obliquity),
		math.Float64bits(f.Perihelion), math.Float64bits(f.Solar),
	}
}

// today reports whether f is Today's forcing, to the bit.
func (f Forcing) today() bool {
	return f == (Forcing{}) || f.key() == Today().key()
}

// ebm is the settled energy-balance year under today's forcing, worked out
// the first time it is asked for.
func ebm() *ebmClimate {
	ebmOnce.Do(func() { ebmOut = solveEBMUnder(Today()) })
	return ebmOut
}

// ebmUnder is the settled energy-balance year under f, worked out the first
// time it is asked for.
func ebmUnder(f Forcing) *ebmClimate {
	if f.today() {
		return ebm()
	}
	v, _ := ebmMore.LoadOrStore(f.key(), &ebmEntry{})
	e := v.(*ebmEntry)
	e.once.Do(func() { e.out = solveEBMUnder(f) })
	return e.out
}

// ZonalMeanUnder is ZonalMean under forcing f.
func ZonalMeanUnder(f Forcing, lat float64) float64 {
	e := ebmUnder(f)
	return e.at(&e.mean, lat)
}

// SwingUnder is SwingAt under forcing f.
func SwingUnder(f Forcing, lat, cont float64) float64 {
	e := ebmUnder(f)
	sea, land := e.at(&e.swingS, lat), e.at(&e.swingL, lat)
	return math.Copysign(sea+(land-sea)*clamp01(cont), lat)
}

// GlobalMeanUnder is the balance's mean over the whole planet under f, in
// degrees: the bands are of equal area, so it is their plain mean.
func GlobalMeanUnder(f Forcing) float64 {
	e := ebmUnder(f)
	var sum float64
	for _, t := range e.mean {
		sum += t
	}
	return sum / ebmBands
}
