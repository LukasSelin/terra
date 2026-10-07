# The climate of each epoch: the cost study

Part of #57 (X1 in `docs/earth-system-plan.md`). Owner decision 1: measure
first, and accept up to doubling the history's time. This is the measure.
It changes no world: the prototype is behind `deepClimate` in
`deepclimate.go`, which is off and set only by `deepclimate_test.go`, and
`TERRA_DIGEST=check` passes.

## What the history already reads per epoch, and what it does not

The plan supposed the history ran under one weather made for the present.
It does not, quite. Each epoch the history calls `drain`, and `drain` asks
for the weather whenever `weatherStale` says the ground has moved; after an
epoch of the plates it always has. Every globe history measured here makes
exactly sixteen weather calls in sixteen epochs. So each epoch already has
its own winds (`NewEnv` on the epoch's land, sea and heights, the four
phases solved), its own ocean currents (`airEnv.currents`), and its own
vapour budget and orographic rain, all on the epoch's own geography, on
the map's own air cells (the history grid is the map; `historyShrink` is 1).

What is today's in every epoch is the warmth. `g.air.Mean`, the row's
annual mean at sea level, is the energy balance of `ebm.go` solved once for
a planet with three tenths of every band under land. The history's land is
always 0.65 of the planet (`historySea` is the 35% height quantile), and how
it is shared between the bands moves from epoch to epoch. The evaporation
(`PetAt` on `Mean`), the vapour budget's saturation, the soil's weathering
(`meanTempOf`) and the lime on a warm sea floor (`quietFloor`) all read
`g.air.Mean`. So X1's new work is an energy balance per epoch, on the
epoch's land, under the epoch's forcing; the weather it feeds is already
paid for.

## The prototype

- `internal/atmos/zonal.go`: `SolveZonal(land, sea, years, from)`, the
  seasonal two-column balance of `ebm.go` with a land share per band
  instead of the constant `ebmLand`, which can carry on from the state an
  earlier solve ended in. Under a uniform 0.3 it is `ebm.go`'s balance to
  the bit (`TestZonalYearOfTodaysLandIsTodays`). `SolveZonalForced` adds a
  forcing in W/m² taken off Budyko's A, standing in for A0's `Forcing`
  (`origin/claude/air-forcings`, unmerged).
- `deepclimate.go`: at the start of each epoch, after `historyBase` and
  before `drain`, `landBands` reads each band's land share off the rows
  (each row spread over the bands its span of latitude covers, by area),
  the balance is solved, and `g.air` becomes a copy with that `Mean`. The
  weather is marked stale. After the last epoch today's air is put back.
- `atmos.SetCellCoarsen` makes the air's cells 2x wider for the
  coarse-air variants; it is 1 everywhere else.
- `TestDeepClimateCost` makes a history under each variant and reports
  time (total, per epoch, per pass), each epoch's climate, the ground the
  history leaves, and the cheaper climates' error against a cold solve:

      TERRA_DEEP_CLIMATE=1 TERRA_PHASES=1 go test -run TestDeepClimateCost -count=1 -v -timeout 60m .

M1's barotropic flow (`origin/claude/ocean-flow-2d`) is not merged and
rewrites `ocean.go`, so it is not in the prototype; its cost is estimated
from its own worklog entry below.

## The cost

Seed 1, `history` alone (the ground stage's settling after it not
counted), the minimum of two runs, Windows, 24 threads, machine otherwise
quiet. "EBM s" is the time in `SolveZonal`. Each variant's history also
moves its world, and a different world takes a different time to erode
(the +0.01 C row is the same work as today on another world), so the honest
added cost of a variant is its EBM column, not the difference of totals.

**GlobeTerms (1024x512)**

| variant | history s | x today | per epoch s | weather calls | weather s | windsFor | currents | vapour | orographic | EBM s |
|---|---|---|---|---|---|---|---|---|---|---|
| today (off) | 43.06 | 1.00 | 2.51 | 16 | 12.53 | 2.72 | 1.75 | 5.17 | 5.28 | 0 |
| EBM each epoch, cold (20 yr) | 50.86 | 1.18 | 3.00 | 16 | 11.33 | 2.51 | 1.65 | 4.43 | 4.80 | 10.26 |
| **EBM each epoch, warm (3 yr)** | **44.75** | **1.04** | 2.63 | 16 | 11.81 | 2.59 | 1.68 | 4.94 | 4.89 | **2.17** |
| EBM every other epoch, cold | 46.15 | 1.07 | 2.72 | 16 | 11.21 | 2.46 | 1.61 | 4.51 | 4.77 | 5.14 |
| shift by land share (no solve) | 42.88 | 1.00 | 2.50 | 16 | 11.64 | 2.72 | 1.74 | 4.54 | 4.83 | 0 (1.25 once) |
| EBM each epoch, air 2x coarser | 47.89 | 1.11 | 2.83 | 16 | 6.98 | 0.67 | 0.41 | 1.11 | 4.88 | 10.49 |
| today's air, 2x coarser | 37.86 | 0.88 | 2.19 | 16 | 6.92 | 0.67 | 0.39 | 1.08 | 4.88 | 0 |
| today's air +0.01 C (noise floor) | 42.38 | 0.98 | 2.48 | 16 | 12.09 | 2.67 | 1.72 | 5.10 | 5.02 | 0 |
| EBM each epoch, 4x CO2 (+7.4 W/m²) | 53.88 | 1.25 | 3.20 | 16 | 12.08 | 2.66 | 1.67 | 4.75 | 4.98 | 10.75 |

**Small globe (256x128), seeds 1, 2, 3**

| variant | history s, seed 1 / 2 / 3 | x today | EBM s |
|---|---|---|---|
| today (off) | 3.07 / 3.65 / 3.77 | 1.00 | 0 |
| EBM each epoch, cold | 13.38 / 13.58 / 13.44 | 3.6-4.4 | 10.0-10.2 |
| **EBM each epoch, warm 3 yr** | **5.62 / 5.90 / 5.37** | **1.4-1.8** | **2.0-2.3** |
| EBM every other epoch, cold | 8.57 / 9.28 / 8.33 | 2.2-2.8 | 5.0-5.3 |
| shift by land share | 4.37 / 4.04 / 3.70 | 1.0-1.4 | 0 |
| EBM each epoch, air 2x coarser | 12.74 / 14.22 / 12.58 | 3.3-4.1 | 10.1-11.1 |
| today's air, 2x coarser | 2.48 / 2.89 / 2.59 | 0.7-0.8 | 0 |

What the numbers say:

- **The balance is the whole of the new cost, and it does not scale with
  the map.** A cold solve is 20 years x 2192 steps x 90 bands, serial: 0.64
  s whatever the grid. Sixteen of them are 10.3 s, a quarter of a globe's
  history and three times a small globe's.
- **Warm-started, it is a fifth of that.** Carried on from the last epoch's
  state for three years, a solve is 0.10 s; sixteen epochs (the first cold)
  are 2.2 s: +4% on the globe, +40-80% on the small globe, whose history is
  only 3-4 s. Its climate is the cold solve's to 0.03 degrees at worst in
  any band of any epoch on any of the four worlds. One year is not enough
  (up to 0.35 degrees off).
- **Asking for the weather again costs nothing.** The weather is stale
  every epoch already: "weather kept" and "weather asked again" make the
  same world with the same sixteen calls.
- **Every other epoch** halves the cold cost and is up to 0.61 degrees off;
  it is beaten on both counts by the warm start.
- **The shift by land share** (today's balance plus a per-band sensitivity
  read once off a second balance at twice the land) costs nothing and is up
  to 0.50 degrees off the solve, against 0.6-1.2 for today's air. It cannot
  take a forcing, a carried ice state or a new albedo, which X2, X3 and G8
  all need, so it is a fallback, not the design.
- **A coarser air** saves 4.6 s of the globe's 12.5 s weather (wind, currents
  and vapour all fall by 3-5x; the orographic FFT is on the tiles and does
  not move). That would pay for the cold solve twice over, but it moves the
  world as much as anything else measured and its rain is not the map's: it
  is a separate decision, not part of X1.
- **The ocean.** The current ocean (`airEnv.currents`) is already run each
  epoch: 1.75 s of the globe's history. M1's gyre solve adds some 70 ms a
  call on the globe's 512x256 air cells and saves some 40 ms in the warmth
  sweeps (its worklog entry): about +0.5 s over sixteen epochs, ~1%. On the
  small globe's air grid it is a few ms a call. M1 needs nothing from X1 to
  run per epoch: it rides on the weather that is already there.

## What it changes

**The climate of each epoch.** Over the same ground (the solve's own run;
today's column is today's air on that epoch's ground):

- The planet's mean is 16.07-16.13 against today's 16.24 every epoch on
  every world: the history's land is always 0.65 of the planet, not 0.3,
  and more land is a slightly colder planet in this balance.
- Poleward of 60 degrees, the mean is 0.4-0.7 degrees colder than today's,
  and colder the more land is there: on the small globe seed 1, polar land
  0.63 gives -5.98 and 0.79 gives -6.27; on the globe, 0.60 gives -5.83
  and 0.69 gives -5.92. A polar continent does cool its epoch, as #57 asks,
  but by tenths of a degree. `TestPolarLandCoolsItsPole`: a northern
  hemisphere poleward of 42 degrees nine tenths land is 0.7 colder at 80N.
  The two-column balance trades heat between a band's land and sea at
  6 W/m²K, so a band's annual mean hardly cares what it is made of; the
  land's own column differs from the sea's by 0.15 degrees at 70N. What a
  continent does is in the seasons (the land's swing, `swingL`), and the
  history's air reads the annual mean only.
- The land's mean temperature falls 0.2-0.4 degrees; its rain, on the same
  ground, falls 1-10% in most epochs (globe epoch 0: 1790 to 1699 mm, runoff
  1565 to 1475).
- **Some epochs flip.** In five epochs of the 64 measured, a change of 0.1-0.3
  degrees in the air changed the land's rain by a factor of two or more,
  either way (globe epoch 10: 1603 to 3799 mm; small seed 1 epochs 1 and 14:
  1275 to 615, 1201 to 511; seed 2 epoch 2: 982 to 1625; seed 3 epoch 11:
  558 to 1595). The rain is deterministic (the same air on a copy gives the
  same rain to the millimetre); it is the vapour budget in a history that
  is sensitive: `RainCells` starts from the last epoch's budget and runs
  one recycling round (`recycleRounds` only from a cold start), so the land
  rain an epoch settles to depends on where it started and moves in steps.
  X1 will expose this every epoch; it should be looked at before X1 lands
  (see below).

**The ground the history leaves.** The history is chaotic: today's air
raised by a hundredth of a degree everywhere moves the final heights by
459-683 m RMS, which is as far as any variant moves them (EBM each epoch:
526-627 m). Its hypsometry and drainage, read on four worlds, move within
the spread the +0.01 C run shows:

| world | variant | land h q50 | q90 | hypsometric integral | channel share | largest basin / land | land rain mm | epochs' land rain mm |
|---|---|---|---|---|---|---|---|---|
| globe | today | 338 | 431 | 0.052 | 0.101 | 0.068 | 641 | 2066 |
| globe | +0.01 C | 461 | 522 | 0.049 | 0.090 | 0.053 | 705 | 2535 |
| globe | EBM each epoch | 459 | 609 | 0.050 | 0.091 | 0.055 | 704 | 1858 |
| globe | EBM, 4x CO2 | 426 | 532 | 0.035 | 0.099 | 0.050 | 1039 | 3619 |
| small 1 | today / +0.01 C / EBM / 4x CO2 | 544 / 530 / 604 / 564 | 867 / 810 / 780 / 735 | 0.085 / 0.058 / 0.082 / 0.061 | 0.065 / 0.051 / 0.060 / 0.057 | 0.088 / 0.088 / 0.079 / 0.053 | 663 / 505 / 579 / 619 | 666 / 612 / 613 / 784 |
| small 2 | same | 704 / 673 / 827 / 690 | 834 / 827 / 991 / 863 | 0.091 / 0.095 / 0.104 / 0.098 | 0.051 / 0.044 / 0.039 / 0.037 | 0.072 / 0.070 / 0.049 / 0.033 | 518 / 540 / 564 / 736 | 722 / 714 / 775 / 996 |
| small 3 | same | 649 / 542 / 644 / 577 | 888 / 761 / 928 / 770 | 0.078 / 0.064 / 0.079 / 0.068 | 0.060 / 0.060 / 0.047 / 0.046 | 0.091 / 0.082 / 0.111 / 0.048 | 484 / 538 / 505 / 664 | 990 / 1083 / 996 / 1283 |

(Heights are history metres over the history's own sea level, the 35%
quantile read afresh at the end.) Under today's forcing, an epoch's own
balance does not move the history's erosion beyond its own noise: the
balance's signal, a few tenths of a degree, is smaller than what the
history does with any perturbation. Under a forcing it does: at 4x CO2
(+7.4 W/m², the planet 3.6 degrees warmer), the epochs' land rain is up 18,
38, 30 and 75% on the four worlds, the globe's rivers carry half again as
much water, and the largest basins are smaller on all four. That is X2's
lever, and the cost of it is the same balance.

## Recommendation

Within the owner's budget (up to doubling the history's time):

1. **Solve the balance every epoch, warm-started**: the first epoch from
   today's settled state (or cold), each later one carried on three years
   from the last. +2.2 s a history: +4% on the globe, +40-80% on the small
   globe. The epoch's state is carried (which G8's ice will need anyway).
   Keep the weather as it is: it already runs per epoch, on the map's air
   cells, and asking for it again costs nothing.
2. **Take M1's flow per epoch for free** when it merges: it is part of the
   weather the history already makes (~+1%).
3. **Not** every other epoch, the shift, or a coarser air: the first two
   are less accurate than the warm start for little or no saving, and the
   third is a change of the world's rain, not of its cost.
4. **Before X1 moves the world**, look at the history's vapour budget: one
   recycling round from the last epoch's budget makes some epochs' land
   rain jump by 2x for a 0.1 degree change. Two rounds where the air has
   changed, or a convergence test, would cost at most one more vapour pass
   an epoch (globe: ~5 s/16 = 0.3 s each).
5. **Hand the land its own column.** The history reads one annual mean a
   row; the balance has the land's and the sea's means and their swings
   (`MeanLand`, `swingL`). X1 proper should give `Air` the land's mean and
   swing per row so that `meanTempOf`, the PET and G8's summer read the
   land's year. That is where a polar continent's cold shows.

Total for X1 as recommended: about +2.7 s on a 43 s globe history (1.06x),
and +2.2 s on a 3-4 s small-globe history (1.6x at worst), both inside the
budget. The cold solve each epoch also fits on the globe (1.18x) but not on
the small globe (up to 4.4x).

## What X2, G7 and G8 need from it

- **X2 (the carbon thermostat).** A forcing per epoch: replace
  `SolveZonalForced`'s W/m² with A0's `Forcing` (CO₂, orbit, sun), set by
  X2's outgassing against weathering. A0's cache keyed by forcing does not
  help here (each epoch's land is new), so the warm start is the saving.
  The weathering X2 counts (soil.go, Arrhenius x runoff) reads
  `g.air.Mean` and the runoff, which X1 makes per epoch; X2 also needs each
  epoch's land runoff and land temperature summed per epoch, which the
  study already logs (`readEpoch`). 4x CO2 moved the epochs' land rain by
  18-75%: the loop has teeth.
- **G7 (rocks of an epoch's climate).** `quietFloor`'s lime reads
  `g.air.Mean` and so becomes per epoch with no change. Evaporites and coal
  need each epoch's aridity and wetness (rain over PET, per tile, at the
  epoch) kept in the book (`keepBook`) rather than read off the present at
  `settleRock`. That is a few bytes a tile an epoch, not a new solve.
- **G8 (ice sheets).** The land's summer, not the band's annual mean: the
  balance's land column and its swing per band (`meanL`, `swingL`), the
  snowfall from the epoch's vapour budget, and an albedo that knows the ice
  (X3: the balance's albedo is temperature-only today; an ice sheet's share
  of a band has to enter `iceAlbedoWith`). The carried state of the warm
  start is the right shape for ice's hysteresis: an ice sheet's epoch should
  start where the last one ended.

## Files

- `internal/atmos/zonal.go`, `internal/atmos/zonal_test.go`: the balance
  per band, held to today's to the bit.
- `internal/atmos/wind.go`: `SetCellCoarsen` (1 everywhere but the study).
- `deepclimate.go`: the switch (off), `landBands`, the epoch climate.
- `history.go`: three guarded lines (`newEpochClimate`, `epoch`, `done`).
- `deepclimate_test.go`: the switch is off; the land is read off the rows;
  `TestDeepClimateCost`, run by hand.
