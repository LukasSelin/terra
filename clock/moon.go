package clock

import "math"

// The moon, and the sea it pulls.
//
// A day is the smallest thing that happens here, and the sea rises and falls
// twice in one. So a tide is not something a walker watches come in: it is a
// fact about the day, the way the weather is. Each day has a moon, a height the
// water reaches at high tide and a height it falls to at low, and anything on
// the shore asks the day whether it is dry. What changes from one day to the
// next is how far the water goes, and that is the moon's doing and the sun's.
//
// The tide is the sum of two: the moon's, twice a lunar day, and the sun's,
// twice a solar day, a little under half as strong. When the two pull
// together - at new moon and at full - they add, and the tides are springs:
// the highest high water and the lowest low. At the quarters they pull
// across each other and the tides are neaps, with the water hardly moving.
// The swing from one to the other takes half a synodic month, fourteen and
// three quarter days, and a world with a coast lives by it: the flats are
// open at springs and shut at neaps, and a ford across them is a ford one
// fortnight in two.
//
// It is all a function of the day and nothing else. No chance is drawn for
// it, so a world's stream of chance is exactly what it was before there was
// a moon; and every copy of the map that asks the same day gets the same
// answer.

// The strengths of the tide at an open coast, in metres of rise above mean sea.
// TideM2 and TideS2 are the moon's and the sun's semidiurnal tides, the two
// that make almost the whole of a tide on most of the world's coasts, at the
// size they come to at an open coast; TideN2Share is how much the moon's
// changes between its nearest and furthest; SpringLag is how many days after
// a new or full moon the springs come, because the ocean takes time to answer.
//
// What is left out, and could be put in: the diurnal tides the moon's
// declination makes, which part the two high waters of a day, and the
// eighteen-year wobble of the moon's orbit. Neither changes what a day's low
// water uncovers by more than a hand's breadth at an open coast, and a day is
// all anybody here lives by.
const (
	TideM2      = 0.54
	TideS2      = 0.25
	TideN2Share = 0.19
	SpringLag   = 1.5
)

// TideMax is the highest the sea ever rises above its mean at an open coast,
// and so the lowest it ever falls: the moon at its nearest, at springs.
// MeanHigh is how high an ordinary high water comes.
const (
	TideMax  = TideM2*(1+TideN2Share) + TideS2
	MeanHigh = TideM2
)

// Moon is the moon on a day: how many days since it was new, and how far round
// its month that is, from nothing at new moon through a half at full.
type Moon struct {
	Age   float64
	Phase float64
}

// Name is what somebody looking up would call it.
func (m Moon) Name() string {
	names := [8]string{"new", "waxing crescent", "first quarter", "waxing gibbous",
		"full", "waning gibbous", "last quarter", "waning crescent"}
	return names[int(math.Floor(m.Phase*8+0.5))%8]
}

// Tide is the sea on a day, at an open coast: how high the water comes and how
// low it falls, in metres about its mean, and where between the weakest neap
// and the strongest spring the day is, from nothing to one. A coast that
// gathers the tide - see the land's Grid.TidalRange - multiplies all of it.
type Tide struct {
	Tick      int
	Moon      Moon
	High, Low float64
	Springs   float64
}

// Epoch is where the moon's two months stood on the founding day: how many days
// into its month of phases the moon was, and how many into its month of
// distances. Each world has its own, taken from its seed.
type Epoch struct {
	Synodic, Anomalistic float64
}

// EpochOf is the moon a world was founded under. It is read off the seed by
// hashing it rather than drawn from the world's chance, so that the moon takes
// nothing from the stream everything else is drawn from in order.
func EpochOf(seed uint64) Epoch {
	return Epoch{
		Synodic:     unit(splitmix(seed^0x6d6f6f6e)) * SynodicMonth,
		Anomalistic: unit(splitmix(seed^0x70657269)) * AnomalisticMonth,
	}
}

// splitmix is one step of SplitMix64, which turns one number into another that
// looks nothing like it.
func splitmix(x uint64) uint64 {
	x += 0x9E3779B97F4A7C15
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	return x ^ (x >> 31)
}

// unit is a number in [0, 1) made of the top bits of x.
func unit(x uint64) float64 { return float64(x>>11) / (1 << 53) }

// MoonOn is the moon on tick, for a world whose moon stood at e when it was
// founded.
func MoonOn(tick int, e Epoch) Moon {
	age := mod(float64(tick)+e.Synodic, SynodicMonth)
	return Moon{Age: age, Phase: age / SynodicMonth}
}

// TideOn is the sea on tick at an open coast.
//
// The day's range is the envelope of the moon's tide and the sun's beating
// against each other: they add when the sun and moon are in line, twice a
// month, and take away from each other at the quarters. It is written as the
// length of the sum of two turning arrows rather than as the moon's tide plus
// a cosine of the sun's, because that is exactly M2 + S2 at springs and
// exactly M2 - S2 at neaps, and the cosine form is only near either.
func TideOn(tick int, e Epoch) Tide {
	m := MoonOn(tick, e)
	apart := 2 * math.Pi * (m.Age - SpringLag) / SynodicMonth
	near := math.Cos(2 * math.Pi * mod(float64(tick)+e.Anomalistic, AnomalisticMonth) / AnomalisticMonth)
	moon := TideM2 * (1 + TideN2Share*near)
	h := math.Sqrt(moon*moon + TideS2*TideS2 + 2*moon*TideS2*math.Cos(2*apart))
	weakest := TideM2*(1-TideN2Share) - TideS2
	return Tide{
		Tick: tick, Moon: m, High: h, Low: -h,
		Springs: clamp01((h - weakest) / (TideMax - weakest)),
	}
}

// mod is x modulo m, always in [0, m).
func mod(x, m float64) float64 {
	r := math.Mod(x, m)
	if r < 0 {
		r += m
	}
	return r
}

// clamp01 is v held to [0, 1].
func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }
