package atmos

import "math"

// The snow.
//
// The soil's bucket took a phase's rain as water whatever the phase's
// warmth, so a winter's precipitation on a frozen country went into a soil
// the air took nothing from, filled it, and ran off in the winter: the
// rivers of the cold countries ran high in the dead of the year and the
// spring had nothing to melt.
//
// Here the precipitation of a cold day falls as snow and lies, and the
// warmth of the days after melts it by degree-days (Hock, 2003): a
// millimetre of water for every so many degrees over freezing a day, which
// is how the melt of a snowpack is reckoned wherever there is no measured
// energy balance to reckon it by. The melt goes into the soil's bucket in
// the fortnight it happens, so a snow-fed river runs high in the spring and
// early summer, after the cold that laid its water down.
//
// A day's temperature is not the season's: it scatters about it, and a
// fortnight whose mean is two degrees under freezing still has days that
// rain and days that melt. The days are taken as scattered normally about
// the fortnight's mean, by snowSpread, which is how the positive degree-day
// models of the ice sheets take them (Reeh, 1991; Calov and Greve, 2005).
// The share of the precipitation that falls as snow is then the mean over
// those days of a ramp from all snow at snowCold to all rain at snowWarm,
// and the degree-days are the mean of the days' warmth over freezing: both
// exact for a normal scatter, so the steps cost nothing for being exact.
//
// Snow that outlasts the year is a glacier. Where more snow falls in a year
// than the year can melt, the pack never melts out and the ground under it
// grows nothing: the ground's mass balance, what the year lays down less
// what it melts and the air takes, is over nothing there. A glacier does not
// pile up for ever - its ice flows down to where it melts - but the flow is
// not this map's to work out (that is the ice through the history's); the
// year's surplus is what leaves the ground as ice, and it is sent to the
// rivers in the season the ice melts, which is where the water of a
// glaciated basin goes.
const (
	// snowCold and snowWarm are the day's mean under which all of the
	// precipitation falls as snow and over which all of it is rain.
	// Jennings and others (2018), from seventeen million observations over
	// the northern hemisphere's land, find the temperature at which half of
	// it falls as snow at a degree on the mean, and most of them between
	// freezing and two degrees.
	snowCold = 0.0
	snowWarm = 2.0
	// snowSpread is how far, in degrees, the days of a fortnight scatter
	// about its mean: the standard deviation of a day's mean temperature
	// about the month's. Reeh (1991) took five for the Greenland ice sheet
	// the year round; measured, it is two and a half to four in the summer
	// and more in the winter (Fausto and others, 2009; Seguinot, 2013), and
	// the summer's is the one the melt answers to.
	snowSpread = 3.5
	// meltFactor is the snow's degree-day factor, the mm of water a day
	// melts for every degree it stands over freezing. Hock (2003) gathers
	// those measured on snow from 2.5 to 11.6, most of them from three to
	// six.
	//
	// The two are taken where the snow's equilibrium line comes out
	// Ohmura's: the precipitation at which a year's snow just outlasts it,
	// against the warmth of its summer, is what Ohmura, Kasser and Funk
	// (1992) fitted at the equilibrium lines of seventy glaciers, P = 645 +
	// 296T + 9T², to within two fifths and mostly a sixth where the year
	// swings ten degrees or more, from summers at freezing to summers at
	// six; a maritime year of six either way melts faster for its warm
	// summers than Ohmura's glaciers do, and wants up to twice their snow
	// at six degrees (TestSnowLineIsOhmuras). Five degrees and four
	// millimetres, near Reeh's figures, put it half again to twice as wet as
	// Ohmura's everywhere.
	meltFactor = 3.0
	// coverDepth is the snow water, in mm, over which a tile is mostly
	// covered: the share covered is tanh of the snow water over it. Roesch
	// and others (2001) fitted the share of the ground snow covers to the
	// satellites' at 0.95·tanh(100·SWE) for SWE in metres, ten millimetres;
	// the 0.95 is the forest's canopy, which is not the snow's to say.
	coverDepth = 10.0
	// snowYears is how many years a pack is run through before it is taken
	// to be at its steady year: one in which it has melted out.
	snowYears = 50
)

// daysPerYear is the days of the year in which the snow melts: the planet's,
// not the calendar's.
const daysPerYear = secondsPerYear / 86400

// normalMean is the mean of max(0, x) over x normally scattered about mu by
// sigma: σφ(μ/σ) + μΦ(μ/σ).
func normalMean(mu, sigma float64) float64 {
	z := mu / sigma
	return sigma*math.Exp(-z*z/2)/math.Sqrt(2*math.Pi) + mu*0.5*math.Erfc(-z/math.Sqrt2)
}

// snowShare is the share of the precipitation of days scattered about a mean
// of t degrees that falls as snow.
func snowShare(t float64) float64 {
	// The ramp from snowCold to snowWarm is the difference of two hinges.
	w := snowWarm - snowCold
	return clamp01((normalMean(snowWarm-t, snowSpread) - normalMean(snowCold-t, snowSpread)) / w)
}

// degreeDays is the mean warmth over freezing a day of days scattered about
// a mean of t degrees has.
func degreeDays(t float64) float64 {
	return normalMean(t, snowSpread)
}

// The two are read off a table, every snowStep of a degree from snowLo to
// snowHi: a land's every tile asks them at every step of its year each time
// the rain is read, and the exponentials and error functions were half of
// what reading the rain cost. Six spreads of the days below snowCold no day
// of a fortnight is warm enough to rain or to melt, and six above snowWarm
// none is cold enough to snow or to lie under freezing, to a part in a
// thousand million.
const (
	snowLo   = snowCold - 6*snowSpread
	snowHi   = snowWarm + 6*snowSpread
	snowStep = 1.0 / 64
	// snowLeast is the share of a fortnight's precipitation under which
	// it is taken that none of it fell as snow, so that a year with no day
	// near freezing is the bucket's without snow, exactly.
	snowLeast = 1e-6
)

var snowTable = func() [][2]float64 {
	out := make([][2]float64, int((snowHi-snowLo)/snowStep)+2)
	for k := range out {
		t := snowLo + float64(k)*snowStep
		share := snowShare(t)
		if share < snowLeast {
			share = 0
		}
		out[k] = [2]float64{share, degreeDays(t)}
	}
	return out
}()

// snowAt is snowShare and degreeDays at t, off the table.
func snowAt(t float64) (share, days float64) {
	switch {
	case t <= snowLo:
		return 1, 0
	case t >= snowHi:
		return 0, t
	}
	f := (t - snowLo) / snowStep
	k := min(int(f), len(snowTable)-2)
	f -= float64(k)
	a, b := &snowTable[k], &snowTable[k+1]
	return a[0] + (b[0]-a[0])*f, a[1] + (b[1]-a[1])*f
}

// SnowFree reports whether a year whose mean at the ground is mean, swinging
// swing either side of it, is too warm for snow ever to fall.
func SnowFree(mean, swing float64) bool {
	return mean-math.Abs(swing) >= snowNone
}

// snowNone is the warmth over which the table says no snow falls at all.
var snowNone = func() float64 {
	last := 0
	for k, v := range snowTable {
		if v[0] > 0 {
			last = k
		}
	}
	return snowLo + float64(last+1)*snowStep
}()

// stepAngle is the place in the year of step j of steps of phase k: the
// angle whose sine is how far into the north's summer the year stands (see
// phaseSin), the phase's middle at its own.
func stepAngle(k, j, steps int) float64 {
	return float64(k-1)*math.Pi/2 + ((float64(j)+0.5)/float64(steps)-0.5)*math.Pi/2
}

// stepSins is the sine of every step's place in the year at bucketSteps a
// phase, in the year's order.
var stepSins = func() (out [Phases * bucketSteps]float64) {
	s := 0
	for _, k := range yearOrder {
		for j := range bucketSteps {
			out[s] = math.Sin(stepAngle(k, j, bucketSteps))
			s++
		}
	}
	return out
}()

// snowYear is a snowpack's steady year under a year's precipitation, phase by
// phase in rain, with the air able to take up pet in each, on ground whose
// year has a mean of mean degrees swinging swing either side of it, signed by
// hemisphere as SwingAt is. It is taken in steps steps a phase.
//
// It writes into in and dry, step by step in the year's order, the water
// that reaches the soil - the rain and the melt - and what the air can still
// take up from the soil once the snow has given it what it gives, both in
// mm; and the phases' snow into out. Where the snow outlasts the year it is
// a glacier, and out.Ice is its mass balance; the surplus is in neither in
// nor dry but in out.Runoff, to be added to what the bucket sheds.
//
// It reports whether any snow fell. Where none did it writes nothing, and
// the year is the bucket's without snow.
func snowYear(rain, pet *[Phases]float64, mean, swing float64, steps int, in, dry []float64, out *BucketYear) bool {
	n := Phases * steps
	// Each step's precipitation share as snow and its potential melt, in
	// the year's order.
	var fall, melt []float64
	var buf [2 * Phases * bucketSteps]float64
	if 2*n <= len(buf) {
		fall, melt = buf[:n], buf[n:2*n]
	} else {
		fall, melt = make([]float64, n), make([]float64, n)
	}
	days := daysPerYear / float64(n)
	// Each phase's precipitation and evaporation a step.
	var pk, ek [Phases]float64
	for k := range Phases {
		pk[k] = max(0, rain[k]) / float64(steps)
		ek[k] = max(0, pet[k]) / float64(steps)
	}
	var balance, fell float64 // the year's, under a pack that never melts out
	s := 0
	for _, k := range yearOrder {
		for j := range steps {
			sin := stepSins[min(s, len(stepSins)-1)]
			if steps != bucketSteps {
				sin = math.Sin(stepAngle(k, j, steps))
			}
			share, dd := snowAt(mean + swing*sin)
			p := pk[k]
			fall[s] = share * p
			melt[s] = meltFactor * dd * days
			balance += fall[s] - melt[s] - ek[k]
			fell += fall[s]
			s++
		}
	}
	if fell <= 0 {
		return false
	}
	out.Ice = 0
	if balance > 0 {
		// The pack never melts out: it covers the ground the year round,
		// the air takes its fill of it, and the days melt all they can.
		// The year's surplus leaves as ice, and melts with the melt.
		out.Ice = balance
		var melted float64
		for s := range n {
			melted += melt[s]
		}
		var pack, least float64
		s = 0
		for _, k := range yearOrder {
			var water, molten, fell float64
			for range steps {
				p := pk[k]
				e := ek[k]
				in[s] = p - fall[s] + melt[s]
				dry[s] = 0
				pack += fall[s] - e - melt[s]
				least = min(least, pack)
				water += pack
				molten += melt[s]
				fell += fall[s]
				share := 1.0 / float64(n)
				if melted > 0 {
					share = melt[s] / melted
				}
				out.Runoff[k] += balance * share
				out.Evap[k] += e
				s++
			}
			out.Snow[k] = water / float64(steps)
			out.Melt[k], out.Snowfall[k], out.Cover[k] = molten, fell, 1
		}
		// The snow over the ice: the year's pack above its least.
		for k := range Phases {
			out.Snow[k] -= least
		}
		return true
	}

	// A pack that melts out. Its steady year starts where the year it
	// first melts out in ends, which a pack that does not grow from year
	// to year reaches: from there on each year is the one before. It is
	// run from the end of the summer, the north's autumn or the south's,
	// when a pack that melts out has done so, so that the year read is
	// most often the first year run.
	from := 0
	if swing > 0 {
		from = 2 * steps
	}
	pack, least := 0.0, 0.0
	run := func(write bool) bool {
		gone := false
		least = math.Inf(1)
		var water, molten, fell, covered [Phases]float64
		for j := range n {
			s := from + j
			if s >= n {
				s -= n
			}
			k := yearOrder[s/steps]
			p := pk[k]
			e := ek[k]
			pack += fall[s]
			// The share covered is only wanted where the air takes up
			// something, or where it is read.
			c := 0.0
			if pack > 0 && (write || e > 0) {
				c = 1
				if pack < 20*coverDepth {
					c = math.Tanh(pack / coverDepth)
				}
			}
			up := min(pack, c*e)
			pack -= up
			m := min(pack, melt[s])
			pack -= m
			if pack <= 0 {
				pack, gone = 0, true
			}
			least = min(least, pack)
			if write {
				in[s] = p - fall[s] + m
				dry[s] = e - up
				water[k] += pack
				molten[k] += m
				fell[k] += fall[s]
				covered[k] += c
				out.Evap[k] += up
			}
		}
		if write {
			for k := range Phases {
				out.Snow[k] = water[k] / float64(steps)
				out.Melt[k], out.Snowfall[k], out.Cover[k] = molten[k], fell[k], covered[k]/float64(steps)
			}
		}
		return gone
	}
	if run(true); pack == 0 {
		// It ended the year where it started it, with nothing.
		return true
	}
	out.Evap = [Phases]float64{}
	// A thin pack the air takes the last of at less than its fill can
	// waste away a year at a time without melting out. The year after it
	// is much the same year lower down, so it is let down by the least it
	// held, which is where it would melt out; and a pack the air takes
	// less of than the year lays down at its thinnest is steady once it
	// ends a year where it started it.
	for range snowYears {
		was := pack
		if run(false) || math.Abs(pack-was) <= 1e-9*(1+pack) {
			break
		}
		if pack < was {
			pack = math.Max(0, pack-least)
		}
	}
	was := pack
	run(true)
	// A pack that still gains at the end of it stands within a few mm a
	// year of the line, gaining at its thinnest what the air would have
	// taken of a thicker pack. It is a glacier, of the balance it keeps,
	// and its surplus goes as a glacier's does.
	if gain := pack - was; gain > 1e-9*(1+pack) {
		out.Ice = gain
		var melted float64
		for k := range Phases {
			melted += out.Melt[k]
		}
		for k := range Phases {
			share := 1.0 / Phases
			if melted > 0 {
				share = out.Melt[k] / melted
			}
			out.Runoff[k] += gain * share
		}
	}
	return true
}
