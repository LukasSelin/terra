package terra

import "github.com/LukasSelin/terra/clock"

// The moon, and the sea it pulls, are the calendar's: see clock/moon.go. What
// is here is the day's sea as the land reads it, and the names the land has
// always given the moon and the tide.

// The strengths of the tide at an open coast. See clock.TideM2.
const (
	TideM2      = clock.TideM2
	TideS2      = clock.TideS2
	TideN2Share = clock.TideN2Share
	SpringLag   = clock.SpringLag
	TideMax     = clock.TideMax
	MeanHigh    = clock.MeanHigh
)

type (
	// Moon is the moon on a day. See clock.Moon.
	Moon = clock.Moon
	// Tide is the sea on a day, at an open coast. See clock.Tide.
	Tide = clock.Tide
	// Epoch is where the moon's two months stood on the founding day. See
	// clock.Epoch.
	Epoch = clock.Epoch
)

// MoonOn is the moon on tick, for a world whose moon stood at e when it was
// founded.
func MoonOn(tick int, e Epoch) Moon { return clock.MoonOn(tick, e) }

// TideOn is the sea on tick at an open coast.
func TideOn(tick int, e Epoch) Tide { return clock.TideOn(tick, e) }

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

// Moon is the moon today.
func (w *Land) Moon() Moon { return MoonOn(w.Tick, w.moon) }

// Tide is the sea today, at an open coast.
func (w *Land) Tide() Tide { return TideOn(w.Tick, w.moon) }

// Tide is the day's sea the map is being read against: what the land set it to
// when it woke, or what a caller set it to. See SetTide.
func (g *Grid) Tide() Tide { return g.tide }

// SetTide sets the day's sea the map is read against. The land does it itself
// each morning, in Wake; this is for a game that keeps its own days, and for
// anybody asking what the shore would be on another. A copy of the map an
// island is acting on for a day reads the day it was taken on and may not be
// moved to another, for the same reason it may not relabel its water: the
// island's answers have to agree with the ground's. It reports whether the
// tide was set.
func (g *Grid) SetTide(t Tide) bool {
	if g.islanded {
		return false
	}
	g.tide = t
	return true
}
