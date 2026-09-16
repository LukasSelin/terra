# Where the suite's time goes

The root package's tests, run whole with `go test -json -timeout 60m .` on
2026-09-16, on the 2026-09-15 machine with two other test runs sharing it
(`GOMAXPROCS=8`). Under load, so every figure is about twice a quiet run's;
the order is what matters. The run took 1701 s for 226 tests, one of them
main's known failure (`TestRealNumbers/Hack exponent, globe`).

The times are as `go test` reports them, which for a test that reads a
shared world means: the time it happened to be the first to ask for that
world. A full globe (1024x512) is 90-110 s here, a small one (256x128)
7-9 s, and nearly every test in the table is one or several of those. The
table was taken before the changes below it, so it says what the suite
cost when each test made its own.

| # | seconds | test | what it makes |
|---|---|---|---|
| 1 | 339.4 | `TestThePolarSeaIsIce` | three full globes, seeds 1-3 |
| 2 | 315.4 | `TestTheIceEdgeIsNotALineOfLatitude` | the same three, again |
| 3 | 258.2 | `TestTheRealWorld` | four double globes (512x256), 141 s; the full globe, 109 s |
| 4 | 111.5 | `TestTheColdKeepsToThePoles` | the full globe, for the climate tests |
| 5 | 90.4 | `TestAGlobeHasASeaItsRiversReach` | the full globe, timed |
| 6 | 77.4 | `TestSaltLakesStandInDryCountry` | small globes 1-8, first to ask |
| 7 | 71.7 | `TestRealNumbers` | small globes 9-16, 66 s; the rest read what is held |
| 8 | 62.6 | `TestMakingAWorldDoesNotDependOnTheGoroutines` | a small globe five times over, plus ancient and valley |
| 9 | 50.3 | `TestTheSeaIsTheWorldsToSay` | small globes 1-6, its own |
| 10 | 29.9 | `TestTheUplandMaskIsFinerThanTheMap` | |
| 11 | 25.9 | `TestContinentsWeldIntoOnePlate` | small globes 1-3, its own |
| 12 | 25.8 | `TestMountainsAreNotAllOnTheCoast` | the same, again |
| 13 | 25.6 | `TestPlatesAreNotAllOneSize` | the same, again |
| 14 | 24.7 | `TestNoPieceOfCrustIsASliverOrAHemisphere` | the same, again |
| 15 | 23.3 | `TestEveryPlateIsOnePiece` | the same, again |
| 16 | 18.7 | `TestATidalCoastConservesTheGround` | small globe 2 twice, worn: must be its own |
| 17 | 10.9 | `TestTheWeatherChangesFromDayToDay` | small globe 3 and sixty days of weather: its own |
| 18 | 10.4 | `TestAHistoryLeavesAMapTheSettlementCanUse` | |
| 19 | 8.7 | `TestTheTideLaysFlatsOnlyWhereItReaches` | small globe 4 |
| 20 | 8.7 | `TestLandslidesConserveTheGround` | |
| 21 | 8.7 | `TestAWeldedPlateKeepsTheRangeThatMadeIt` | small globe 1 |
| 22 | 8.6 | `TestRunoffIsWhatTheRainLeaves` | small globe 2 |
| 23 | 8.4 | `TestRainFallsInBelts` | small globe 1 |
| 24 | 8.2 | `TestPlateBoundariesAreNotStraight` | small globe 1 |
| 25 | 8.1 | `TestARiverRunsAllTheWayDown` | small globe 3 and a valley |
| 26 | 6.7 | `TestEveryRockAHistoryMakesTurnsUp` | |
| 27 | 6.1 | `TestLandmarkBoundsNeverOverstateTheWalk` | |
| 28 | 4.3 | `TestWorldCreationBudget` | valley, ancient, globe128 |
| 29 | 4.0 | `TestRiversWanderAsTheyAge` | |
| 30 | 3.7 | `TestWornRiversKeepFlintsLaw` | |

The rest of the suite, 196 tests, is 20 s all told.

## Two tiers

Worlds that are only read are made once and shared through `yardWorld` and
`yardLand` in `yardstick_test.go` (the registry holds the whole `Land`, so
a test that reads the climate shares too). Since 2026-09-16 the plate
tests, the two polar tests, the climate's globe, the sea test, and the
river, rain, runoff and tide tests take theirs from it; the globe that
`TestAGlobeHasASeaItsRiversReach` times the making of is kept for the rest
(`keepLand`). That is three full globes and some twenty small ones not made
twice: on the table above, rows 2, 4 and 11-15 and most of 9, 19 and 21-25
fall to nothing, and the suite whole is about eight minutes shorter under
load. A test that writes to its world (rows 16 and 17, the soil test)
makes its own, and must.

`go test -short -timeout 60m .` is the tier to run while working. It
skips everything that makes a globe, small or full: the tests already
behind `testing.Short()`, and since 2026-09-16 the goroutine-independence
test, the salt-lake and sea tests, and every yardstick in `TestRealNumbers`
and `TestTheRealWorld` that reads small globes (`slow: true`, which until
then meant the full globe only). No test asserts anything different under
`-short`; it runs later. What is left is the plate tests, whose three
small globes (seeds 1-3) are made once for the five of them and are under
the 60 s line that decided the gating, the tidal-coast test, which must
wear a small globe of its own, the tide test's small globe 4, the
settlement history, and the drawn valleys. Gating the plate tests too
would take another twenty-some seconds off the tier.

The whole suite is the merge gate. When a test is added that makes a
world, it reads one from the registry if it can, and goes behind
`testing.Short()` if it cannot; a world made twice is a minute of every
run for as long as the test lives.

To refresh this table:

```bash
go test -json -timeout 60m . > suite.json
```

and sort the `pass`/`fail`/`skip` events without a `/` in their `Test` by
`Elapsed`.

## Kept histories

Since 2026-09-16 a world a test only needs, and not the making of, comes
through `madeLand` (`histories_test.go`), which keeps its history in
`.cache/histories` keyed by the package's source. A second run with no
change outside the tests took 187 s against 375 s. See the work log.
