package terra

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

// planetReading is what a made world says of its planet: how long it took,
// how many plates it rides, how much of it is land and ocean crust and belt,
// the rain and the forest on its land, and what rock the land stands on.
type planetReading struct {
	secs, plates, land, oceanCrust, belts, rain, forest float64
	rock [BedrockCount]float64
}

// readPlanet reads l as a planetReading, secs being how long it took.
func readPlanet(l *Land, secs float64) planetReading {
	g := l.Grid
	r := planetReading{secs: secs}
	seen := map[uint8]bool{}
	land := 0.0
	for i := range g.Tiles {
		seen[g.Tiles[i].Plate] = true
		if g.abyssal(i) {
			r.oceanCrust++
		}
		if g.ledger != nil && g.ledger[i].lift > 1000 {
			r.belts++
		}
		if g.underSea(i) {
			continue
		}
		land++
		r.rock[g.Tiles[i].Bedrock]++
		r.rain += g.Rain(i)
	}
	n := float64(len(g.Tiles))
	r.plates = float64(len(seen))
	r.land, r.oceanCrust, r.belts = land/n, r.oceanCrust/n, r.belts/n
	r.rain /= land
	r.forest = float64(g.Forest()) / land
	for k := range r.rock {
		r.rock[k] /= land
	}
	return r
}

// The same planet on history grids of other sizes, printed rather than
// asserted: small globes and the globe with the history on the map, on a grid
// half as fine and on one a quarter as fine, each reading as a mean and a
// spread over seeds. It is how phase 3 of docs/perf/scaling-plan.md judges
// whether a coarser history is the same planet read more coarsely. Minutes,
// so only when asked:
//
//	TERRA_PLANET=1 go test -run TestAPlanetOnCoarserHistories -v -timeout 60m .
func TestAPlanetOnCoarserHistories(t *testing.T) {
	if os.Getenv("TERRA_PLANET") == "" {
		t.Skip("set TERRA_PLANET=1 to print the planet on coarser histories")
	}
	cases := []struct {
		name  string
		terms Terms
		seeds int
	}{{"small", smallGlobe(), 8}, {"globe", GlobeTerms(), 2}}
	for _, c := range cases {
		for _, s := range []int{1, 2, 4} {
			historyShrink = s
			var rs []planetReading
			for seed := 1; seed <= c.seeds; seed++ {
				start := time.Now()
				l := NewLand(uint64(seed), c.terms)
				rs = append(rs, readPlanet(l, time.Since(start).Seconds()))
			}
			historyShrink = 1
			stat := func(f func(planetReading) float64) string {
				var m, v float64
				for _, r := range rs {
					m += f(r)
				}
				m /= float64(len(rs))
				for _, r := range rs {
					v += (f(r) - m) * (f(r) - m)
				}
				return fmt.Sprintf("%.3g±%.2g", m, math.Sqrt(v/float64(len(rs))))
			}
			var rock []string
			for k := Bedrock(0); k < BedrockCount; k++ {
				rock = append(rock, fmt.Sprintf("%v %s", k, stat(func(r planetReading) float64 { return r.rock[k] })))
			}
			t.Logf("%s 1/%d: secs %s plates %s land %s oceancrust %s belts %s rain %s forest %s | %s", c.name, s,
				stat(func(r planetReading) float64 { return r.secs }), stat(func(r planetReading) float64 { return r.plates }),
				stat(func(r planetReading) float64 { return r.land }), stat(func(r planetReading) float64 { return r.oceanCrust }),
				stat(func(r planetReading) float64 { return r.belts }), stat(func(r planetReading) float64 { return r.rain }),
				stat(func(r planetReading) float64 { return r.forest }), strings.Join(rock, ", "))
		}
	}
}
