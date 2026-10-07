package terra

import (
	"math"
	"slices"
	"testing"

	"github.com/LukasSelin/terra/internal/atmos"
)

// The snow and the ice held against the earth's.
//
// The snow (snow.go, atmos.BucketCold) decides when a cold country's water
// reaches its rivers, how much of its ground is white in each season, and
// where its ground is under ice. Read off it: whether the rivers its melt
// feeds run high in the spring and early summer, how much of the north's
// land the snow covers season by season, and how much of the land is ice.
func init() {
	realYardsticks = append(realYardsticks, snowYardsticks...)
}

var snowYardsticks = []realYardstick{
	{yardstick: yardstick{
		name: "snow-fed land whose runoff peaks in spring or early summer, globe", unit: "", scale: "water", lo: 0.8, hi: 1, slow: true,
		source:  "Barnett, Adam & Lettenmaier 2005: where snowmelt dominates the runoff, the rivers peak in spring and early summer, months after the precipitation",
		measure: func() float64 { return landSnow(globes()).springPeak },
	}},
}

// snowReading is what landSnow reads off the land.
type snowReading struct {
	// Where the snow's melt is half the year's runoff or more, the share of
	// the tiles whose runoff is highest in the hemisphere's spring or
	// summer phase, and how many tiles that is.
	springPeak float64
	snowFed    int
	// The share of the north's land, by area, the snow covers on the mean
	// through each phase; and the same by bands of latitude.
	northCover [atmos.Phases]float64
	bands      [9][atmos.Phases]float64
	bandLand   [9]float64
	// The share of the land, by area, under ice by its mass balance; under
	// it by Ohmura's line as Barren read it before; and how much of the
	// two is the same ground, the land both call ice over the land either
	// does.
	ice, ohmura, agree float64
	// Where the year has a phase under freezing, the land's runoff over
	// what Budyko-Fu's curve leaves of its rain.
	coldOverFu float64
}

// landSnow reads the snow off gs's dry land, by area.
func landSnow(gs []*Grid) snowReading {
	return remember("landSnow", gs, func() snowReading {
		var r snowReading
		var north, land, ice, ohm, both, either float64
		var peaked int
		var cold, fu float64
		for _, g := range gs {
			for i := range g.Tiles {
				if g.sunk(i) || g.Tiles[i].Wet() || g.Rain(i) <= 0 {
					continue
				}
				y := i / g.W
				lat := g.air.Lat[y]
				area := g.air.Dx[y] * g.air.Dy
				land += area
				isIce, isOhm := g.IceBalance(i) > 0, len(g.warm) == len(g.Tiles) && g.ohmuraIce(i)
				if isIce {
					ice += area
				}
				if isOhm {
					ohm += area
				}
				if isIce && isOhm {
					both += area
				}
				if isIce || isOhm {
					either += area
				}
				if lat > 0 {
					north += area
					b := min(int(lat/10), len(r.bands)-1)
					r.bandLand[b] += area
					for k := range atmos.Phases {
						r.northCover[k] += area * g.SnowCover(i, k)
						r.bands[b][k] += area * g.SnowCover(i, k)
					}
				}
				var melt, shed float64
				for k := range atmos.Phases {
					melt += g.MeltIn(i, k)
					shed += g.RunoffIn(i, k)
				}
				if mean, swing := g.snowYear(i, 0); mean-math.Abs(swing) < 0 {
					cold += g.Runoff(i)
					fu += g.Rain(i) - atmos.Fu(g.Rain(i), g.pet(i))
				}
				if shed < 1 || melt < shed/2 || isIce {
					continue
				}
				r.snowFed++
				most := 0
				for k := range atmos.Phases {
					if g.RunoffIn(i, k) > g.RunoffIn(i, most) {
						most = k
					}
				}
				spring, summer := 1, 2
				if lat < 0 {
					spring, summer = 3, 0
				}
				if most == spring || most == summer {
					peaked++
				}
			}
		}
		if r.snowFed > 0 {
			r.springPeak = float64(peaked) / float64(r.snowFed)
		}
		for k := range atmos.Phases {
			if north > 0 {
				r.northCover[k] /= north
			}
			for b := range r.bands {
				if r.bandLand[b] > 0 {
					r.bands[b][k] /= r.bandLand[b]
				}
			}
		}
		for b := range r.bandLand {
			r.bandLand[b] /= math.Max(north, 1e-9)
		}
		if land > 0 {
			r.ice, r.ohmura = ice/land, ohm/land
		}
		if either > 0 {
			r.agree = both / either
		}
		if fu > 0 {
			r.coldOverFu = cold / fu
		}
		return r
	})
}

// logSnow writes what landSnow reads off gs.
func logSnow(t *testing.T, what string, gs []*Grid) snowReading {
	r := landSnow(gs)
	t.Logf("%s: %d snow-fed tiles, %.3f peaking in spring or early summer", what, r.snowFed, r.springPeak)
	t.Logf("%s: the north's land snow-covered by phase (winter, spring, summer, autumn): %.3f %.3f %.3f %.3f",
		what, r.northCover[0], r.northCover[1], r.northCover[2], r.northCover[3])
	for b := range r.bands {
		if r.bandLand[b] == 0 {
			continue
		}
		t.Logf("%s:   %2d-%2d N, %.3f of the north's land: %.2f %.2f %.2f %.2f", what, 10*b, 10*b+10, r.bandLand[b],
			r.bands[b][0], r.bands[b][1], r.bands[b][2], r.bands[b][3])
	}
	t.Logf("%s: ice by its balance %.4f of the land, by Ohmura's line %.4f; the same ground %.3f of either", what, r.ice, r.ohmura, r.agree)
	t.Logf("%s: where the year freezes, runoff %.3f times what Budyko-Fu leaves", what, r.coldOverFu)
	return r
}

// The ice's line against Ohmura's, on the land's own years. No globe today
// has ground high or cold enough to hold ice (the energy balance's poles are
// eleven degrees under freezing, not thirty and more), so each land tile is
// lifted until its snow outlasts the year, with its rain and its seasons as
// they are, and the height that takes is held against the height at which
// its summer is Ohmura, Kasser and Funk's (1992) for its precipitation. The
// air's take is shared over the phases by their warmth.
func TestTheIceIsOhmuras(t *testing.T) {
	if testing.Short() {
		t.Skip("needs a small globe")
	}
	var diffs []float64
	for _, g := range smallGlobes(2) {
		for i := 0; i < len(g.Tiles); i += 7 {
			if g.sunk(i) || g.Tiles[i].Wet() || g.Rain(i) < 200 {
				continue
			}
			mean, swing := g.snowYear(i, 0)
			var rain [atmos.Phases]float64
			for k := range atmos.Phases {
				rain[k] = g.RainIn(i, k)
			}
			iced := func(lift float64) bool {
				m := mean - Lapse*lift
				pet := petByWarmth(m, swing, g.pet(i))
				return atmos.BucketCold(100, &rain, &pet, m, swing).Ice > 0
			}
			lo, hi := -3000.0, 12000.0
			if !iced(hi) || iced(lo) {
				continue
			}
			for range 40 {
				mid := (lo + hi) / 2
				if iced(mid) {
					hi = mid
				} else {
					lo = mid
				}
			}
			ohmura := (mean + atmos.SummerPeak*math.Abs(swing) - atmos.IceSummer(g.Rain(i))) / Lapse
			diffs = append(diffs, (lo+hi)/2-ohmura)
		}
	}
	if len(diffs) == 0 {
		t.Fatal("no tile read")
	}
	slices.Sort(diffs)
	q := func(f float64) float64 { return diffs[int(f*float64(len(diffs)-1))] }
	t.Logf("%d tiles: the ice's line over Ohmura's, m: median %.0f, quartiles %.0f and %.0f, tenths %.0f and %.0f",
		len(diffs), q(0.5), q(0.25), q(0.75), q(0.1), q(0.9))
	if math.Abs(q(0.5)) > 300 {
		t.Errorf("the ice's line stands %.0f m off Ohmura's on the median", q(0.5))
	}
}

// petByWarmth is pet mm a year shared over the phases of a year with the
// given mean and signed swing as their warmth over freezing is.
func petByWarmth(mean, swing, pet float64) [atmos.Phases]float64 {
	var each [atmos.Phases]float64
	var total float64
	for k, s := range [atmos.Phases]float64{-1, 0, 1, 0} {
		each[k] = math.Max(0, mean+swing*s)
		total += each[k]
	}
	for k := range each {
		if total > 0 {
			each[k] *= pet / total
		}
	}
	return each
}

// The snow on a made valley: its winter lies, its spring melts, and the
// water its melt brings reaches the rivers.
func TestTheValleysSnow(t *testing.T) {
	g := yardWorld("valley", 1, DefaultTerms())
	r := logSnow(t, "valley", []*Grid{g})
	if r.northCover[0] < r.northCover[2] {
		t.Errorf("the valley's winter covered %.3f of it and its summer %.3f", r.northCover[0], r.northCover[2])
	}
}

// The snow on the small globes and the globe, logged against the earth's:
// the north's snow-covered land is some 46 million km² in February and
// 2-3 million in August (Robinson & Frei 2000; Rutgers Global Snow Lab),
// of some 100 million km² of land north of the equator, so a half at the
// depth of the winter and a fortieth at the height of the summer; and ice
// covers a tenth of the earth's land with the ice sheets and a two
// hundredth without them (RGI Consortium 2017; Pfeffer and others 2014).
// The globe's land is not the earth's, nor at the earth's latitudes, so
// these are readings and not yardsticks.
func TestTheSnowOnTheGlobes(t *testing.T) {
	if testing.Short() {
		t.Skip("needs a small globe")
	}
	logSnow(t, "small globes", smallGlobes(4))
	logSnow(t, "globe", globes())
	r := landSnow(smallGlobes(4))
	if r.snowFed > 0 && r.springPeak < 0.5 {
		t.Errorf("of %d snow-fed tiles only %.2f peaked in spring or early summer", r.snowFed, r.springPeak)
	}
}
