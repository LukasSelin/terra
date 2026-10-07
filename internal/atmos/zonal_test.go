package atmos

import "testing"

// The epoch's balance under today's land is today's balance, to the bit: the
// copy in zonal.go is held to ebm.go's.
func TestZonalYearOfTodaysLandIsTodays(t *testing.T) {
	land, sea := TodaysLand()
	z := SolveZonal(&land, &sea, ZonalYears, nil)
	if *z.c != *ebm() {
		t.Fatal("the zonal balance under a uniform ebmLand is not ebm.go's")
	}
}

// More land toward a pole is a colder pole: the land holds less heat and
// snow lies on it longer.
func TestPolarLandCoolsItsPole(t *testing.T) {
	land, sea := TodaysLand()
	today := SolveZonal(&land, &sea, ZonalYears, nil)
	for k := range land {
		if k >= ZonalBands*5/6 { // north of about 42 degrees
			land[k], sea[k] = 0.9, 0.1
		}
	}
	polar := SolveZonal(&land, &sea, ZonalYears, nil)
	if d := polar.Mean(80) - today.Mean(80); d >= 0 {
		t.Fatalf("a northern continent at 80N moved the mean by %+.2f degrees", d)
	}
	t.Logf("80N %+.2f, 60N %+.2f, equator %+.2f, global %+.2f",
		polar.Mean(80)-today.Mean(80), polar.Mean(60)-today.Mean(60), polar.Mean(0)-today.Mean(0), polar.GlobalMean()-today.GlobalMean())
	t.Logf("the land's own column at 70N: today %.2f, under the continent %.2f; the sea's %.2f and %.2f; the band's mean %.2f and %.2f",
		today.MeanLand(70), polar.MeanLand(70), today.MeanSea(70), polar.MeanSea(70), today.Mean(70), polar.Mean(70))
}
