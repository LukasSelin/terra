package atmos

import "math"

// The storms' climate, as the trees feel it: the share of a canopy their
// wind blows down in a year, as a mean rate. It is read off the rules the
// day's systems are born and die by (synoptic.go), without running their
// years.
//
//   - A tropical cyclone is born over sea of StormSea and warmer between its
//     hemisphere's seventh and twentieth parallels, wanders poleward into
//     the thirties, and dies within a day or so of coming ashore (Kaplan and
//     DeMaria, 1995). The land a cyclone reaches is the land within some
//     hundreds of kilometres of such sea, the less the farther in.
//   - The lows of the middle latitudes are born between the thirty-second
//     parallel and the sixty-second and run down over land. Their gales
//     throw trees all through those latitudes, more on the coasts the lows
//     come in over than deep in a continent.
//
// Where a cyclone's wind strikes, it takes a few to a tenth of the trees
// (Everham and Brokaw, 1996), and a coast in the cyclones' track is struck
// hard every few decades: some four in a thousand of its canopy a year. The
// gales of the middle latitudes take some one and a half in a thousand of
// Europe's growing stock a year (Schelhaas and others, 2003).
const (
	cycloneThrow = 0.004
	galeThrow    = 0.0015
	// cycloneInland is how far, in km, a cyclone's damage reaches inland by
	// e: its wind falls to its floor over land in a day, at the few metres a
	// second a cyclone travels.
	cycloneInland = 150.0
)

// Throw is the share of a canopy the storms blow down in a year over each
// cell: see cycloneThrow and galeThrow.
func (e *Env) Throw() []float64 {
	out := make([]float64, e.W*e.H)
	// Each sea cell's cyclones: how readily one is born on it or passes over
	// it, by its summer's warmth and its latitude.
	sea := make([]float64, e.W*e.H)
	for cy := range e.H {
		lat := e.lat[cy]
		track := smoothstep(4, 8, math.Abs(lat)) * (1 - smoothstep(30, 40, math.Abs(lat)))
		if track <= 0 {
			continue
		}
		summer := math.Copysign(1, lat)
		for cx := range e.W {
			c := cy*e.W + cx
			if e.Sea[c] < 0.8 {
				continue
			}
			sea[c] = track * smoothstep(StormSea-1, StormSea+1, e.SeaTemp(float64(cx), float64(cy), summer))
		}
	}
	// The reach of each into the land round it, falling by e every
	// cycloneInland: the most of e^(-d/cycloneInland) times a sea cell's
	// cyclones over the sea cells, d away, found by sweeping the lattice
	// forward and back, each cell taking the most of its neighbours' less
	// what the step between them takes off (a chamfer distance, Borgefors,
	// 1986), which is within a few per cent of the straight line's.
	cyclone := sea
	stepX := make([]float64, e.H)
	stepD := make([]float64, e.H)
	for cy := range e.H {
		stepX[cy] = math.Exp(-e.Dx[cy] / 1000 / cycloneInland)
		stepD[cy] = math.Exp(-math.Hypot(e.Dx[cy], e.Dy) / 1000 / cycloneInland)
	}
	stepY := math.Exp(-e.Dy / 1000 / cycloneInland)
	at := func(cx, cy int) (int, bool) {
		if cy < 0 || cy >= e.H {
			return 0, false
		}
		if e.Wrap {
			cx = (cx%e.W + e.W) % e.W
		} else if cx < 0 || cx >= e.W {
			return 0, false
		}
		return cy*e.W + cx, true
	}
	take := func(c, cx, cy int, f float64) {
		if n, ok := at(cx, cy); ok {
			cyclone[c] = math.Max(cyclone[c], cyclone[n]*f)
		}
	}
	// Twice each way, so that what comes round a wrapping row is carried on.
	for range 2 {
		for cy := range e.H {
			for cx := range e.W {
				c := cy*e.W + cx
				take(c, cx-1, cy, stepX[cy])
				take(c, cx, cy-1, stepY)
				take(c, cx-1, cy-1, stepD[cy])
				take(c, cx+1, cy-1, stepD[cy])
			}
		}
		for cy := e.H - 1; cy >= 0; cy-- {
			for cx := e.W - 1; cx >= 0; cx-- {
				c := cy*e.W + cx
				take(c, cx+1, cy, stepX[cy])
				take(c, cx, cy+1, stepY)
				take(c, cx+1, cy+1, stepD[cy])
				take(c, cx-1, cy+1, stepD[cy])
			}
		}
	}
	for cy := range e.H {
		lat := math.Abs(e.lat[cy])
		gale := galeThrow * smoothstep(28, 38, lat) * (1 - smoothstep(60, 70, lat))
		for cx := range e.W {
			c := cy*e.W + cx
			out[c] = cycloneThrow*cyclone[c] + gale*(1-0.5*e.Cont[c])
		}
	}
	return out
}
