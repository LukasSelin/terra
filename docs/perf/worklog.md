# World creation performance work log

Newest entry first. Each entry says what was measured, on what, and what it
means; changes say what they bought in benchstat terms. How to take the
measurements is in [README.md](README.md).

---

## 2026-09-16 - The guards: a scaling benchmark, the peak in the budget, pinned pass counts, a CLAUDE.md, and a suite in two tiers

Session C of the overnight briefs (`briefs/C-guards.md`), on
`claude/perf-guards`. Nothing here changes how a world is made: no `.go`
file without `_test` in its name differs from main, and the digest for
valley, ancient and globe128 was as `digest.json` says before and after
every item. After the rebase onto main at 59cdb90 the ancient and globe128
digests no longer match the file, and the valley's still does: main's
deep-floor merge (cb2fa77) moved them, and `digest.json`, written on the
plan branch before it, wants rewriting on main when the plan branch lands.
The budget still holds on the rebased branch (globe128 +0.06% bytes,
+0.24% allocations).
Every timing below was taken with sessions A and B running on the same
machine at `GOMAXPROCS=8`; they are under load and are not the point.

**Scaling.** `BenchmarkNewLand` gains `globe128` and `globe512` beside
`globe256` and `globe`, and `scripts/perf.sh scaling` reads the median
`ns/tile` of three runs of each against the next, failing when 512 is more
than 1.3x 256. First run, under load:

| world | ns/tile (median of 3) |
|---|---|
| globe128 | 480 615 |
| globe256 | 238 907 |
| globe512 | 275 553 |

512/256 = 1.153, ok. 256/128 = 0.50: the bottom rung is still mostly the
constant each world carries, which is why the check reads 512 against 256.
An n log n pass costs 1.13 per doubling of width, so the limit is a
tripwire for a quadratic step, not a proof of linearity; a slower constant
moves every rung alike and is `check`'s to catch.

**Peak.** `TestWorldCreationBudget` samples the heap's live-objects metric
every millisecond while each world is made and writes the highest reading
over the pre-world baseline as `peak` in `budget.json`. It is logged
against the budget like the time and not held to a slack. Three runs of
each reading, under load, in MiB:

| world | sampled HeapAlloc | live at the pacer's marks | live at a forced mark every MiB |
|---|---|---|---|
| valley | 3.8, 3.8, 6.1 | 1.8, 2.1, 2.7 | 2.1, 3.8, 2.1 |
| ancient | 6.2, 14.1, 8.3 | 3.3, 6.3, 8.2 | 7.2, 6.7, 4.1 |
| globe128 | 21.0, 21.6, 22.1 | 11.3, 11.3, 15.8 | 20.7, 16.4, 15.0 |

A spread of 40-130% however it is read, and the same with the collector's
percent at 25 and 10 (which also cost globe128 two seconds and a few
hundred allocations). The cause is the concurrent collector: what the world
allocates during a mark is counted live, and a mark takes longer when the
machine is busy. On a quiet run the sampled figure was within 1-5%, so it is
the one kept. A steady peak needs the world to hold still while the heap is
read, which is a hook at each pass boundary; see "needs session 0's file"
below. Chosen slack: none, until then.

**Pass counts.** `TestPassCountsArePinned` holds valley, ancient and
globe128 to a table of how many times `drain`, `weather`, `wear` and
`landslide` run, read from `claude/perf-instrument` at e946062:

| world | drain | weather | wear | landslide |
|---|---|---|---|---|
| valley | 6 | 7 | 4 | 5 |
| ancient | 24 | 25 | 20 | 6 |
| globe128 | 30 | 31 | 20 | 6 |

`phases.go` is not on main, so the reader is nil and the test skips; the
top of `passes_test.go` says the one line that turns it on.

**CLAUDE.md.** Fifty-nine lines at the root: the determinism contract, the
merge checklist, the timeout, the stash rule, and where these docs are.

**The suite in two tiers.** `go test -json` of the whole root package,
under load: 1701 s for 226 tests, the one failure main has (`TestRealNumbers/Hack exponent, globe`). The thirty slowest are in `suite.md`; the top ten:

| # | seconds | test |
|---|---|---|
| 1 | 339.4 | `TestThePolarSeaIsIce` |
| 2 | 315.4 | `TestTheIceEdgeIsNotALineOfLatitude` |
| 3 | 258.2 | `TestTheRealWorld` |
| 4 | 111.5 | `TestTheColdKeepsToThePoles` |
| 5 | 90.4 | `TestAGlobeHasASeaItsRiversReach` |
| 6 | 77.4 | `TestSaltLakesStandInDryCountry` |
| 7 | 71.7 | `TestRealNumbers` |
| 8 | 62.6 | `TestMakingAWorldDoesNotDependOnTheGoroutines` |
| 9 | 50.3 | `TestTheSeaIsTheWorldsToSay` |
| 10 | 29.9 | `TestTheUplandMaskIsFinerThanTheMap` |

Where the time goes is worlds: a full globe is a minute and a half here,
a small one seven to nine seconds, and the slowest tests were the ones
that happened to make a world first, or made one the registry already
held. So:

- `yardWorlds` holds the whole `Land` now (`yardLand`), and the plate
  tests, the two polar tests, the climate's globe, the sea test, the river,
  rain, runoff and tide tests take their small globe or globe from it
  rather than making their own. `TestAGlobeHasASeaItsRiversReach` still
  makes its globe, because it times the making, and keeps it
  (`keepLand`) for the rest. That is three full globes and some twenty small ones not made twice, about eight minutes of the run under load.
- Under `-short`, everything that makes a globe is skipped: the
  goroutine-independence test (five small globes), the salt-lake and sea
  tests (eight and six), and every yardstick that reads small globes
  (`slow: true`, eighteen of them, which until now meant the full globe
  only). Nothing asserts differently; it runs later.

The short tier: 128 s under load (227 tests, 17
skipped), against 326 s before the gates. What is left in it is the plate
tests' three small globes (27 s under load, shared among five tests and
under the 60 s line each), the tidal-coast test (one small globe it must
wear itself, 20 s), the tide test's small globe 4 (8 s), the settlement
history (11 s), and the drawn valleys.

**Needs session 0's file (`phases.go`).**

1. A pass-boundary hook for the peak: when the budget test asks (a
   package-level `func(name string)` set from the test, or a callback on
   the phase timer's stop), run `runtime.GC()` and read `HeapAlloc` at the
   end of every pass. The largest reading is the peak with nothing
   allocating, the same on any machine; then `peakSlack` in
   `budget_test.go` turns the check on at 1% like the bytes.
2. `TestPassCountsArePinned` wants `passCounts` and `resetPassCounts` set
   to read `Phases()` and `ResetPhases()`, and `TERRA_PHASES=1 -count=1`
   when run.
## 2026-09-16 - Session B: the hydrology, exactly as it was, cheaper

The four items of [briefs/B-hydrology.md](briefs/B-hydrology.md), on branch
`claude/hydrology-exact`, one commit each. Every one leaves every world
bit-for-bit as it was: the digest of valley, ancient and globe128
(`TERRA_DIGEST=check`) matched the base after each item, and
`TestMakingAWorldDoesNotDependOnTheGoroutines` passed after each. Base:
main at 1ad4985 with the plan branch merged (51db354).

**Timing was taken under load and gates nothing.** Sessions A and C ran on
the same desktop all night, at `GOMAXPROCS=8` each. The first globe256
reading, base then item 1 in two separate count-6 runs, came out +18%
(p=0.002) for item 1 - and vanished when the binaries were interleaved,
one run each in turn for six rounds, which is how every table below was
taken. A sequential A/B under load measures the load's drift, not the
change. The interleaved table, globe256, n=6 each, all five binaries in
one session:

| binary | sec/op | vs base | B/op | vs base | allocs/op |
|---|---|---|---|---|---|
| base | 8.806 ± 10% | | 2.416 GiB | | 276.3 k |
| item 1, 4-ary heaps | 8.850 ± 5% | ~ (p=0.699) | 2.354 GiB | -2.58% | 275.2 k |
| item 2, scratch on the grid | 8.749 ± 7% | ~ (p=0.818) | 2.130 GiB | -11.87% | 272.2 k |
| item 3, pool repairs its order | 8.724 ± 8% | ~ (p=0.937) | 2.129 GiB | -11.89% | 272.2 k |
| item 4, solve by outlet tree | 8.607 ± 6% | ~ (p=0.937) | 2.083 GiB | -13.80% | 272.6 k |

Nothing in the clock is significant at these confidence intervals; the
bytes are exact. A quiet count-6 run in the morning is what will say what
the items bought on the clock; the budget says what they bought in memory.

**Budget** (`TestWorldCreationBudget`, Workers 4), bytes and allocations,
rewritten after items 1, 2 and 4 (item 3 left it unchanged):

| world | base | item 1 | item 2 | item 4 (final) | bytes | allocs |
|---|---|---|---|---|---|---|
| valley | 25.53 MB, 3124 | 24.61 MB, 2993 | 21.56 MB, 2905 | 21.00 MB, 2910 | **-17.7%** | -6.9% |
| ancient | 95.03 MB, 13643 | 91.29 MB, 13065 | 72.35 MB, 11814 | 68.79 MB, 11798 | **-27.6%** | -13.5% |
| globe128 | 651.85 MB, 45640 | 637.00 MB, 44799 | 576.52 MB, 43108 | 564.29 MB, 43255 | **-13.4%** | -5.2% |

### Item 1: a 4-ary heap for the floods and the slides (b40c3e2)

`floodQueue` (lake.go) and `slideQueue` (slide.go) are 4-ary heaps: a pop
walks half the ladder, and the four children it asks at each rung sit in
one cache line. Both comparisons are total orders - height, then the push
sequence or the tile's index - so which entries come out, and in what
order, does not depend on the heap's shape; `slideAt.less` was checked
before starting (two entries that compare equal are the same tile at the
same height, and the second is passed over as stale). Each queue's backing
slice is kept on the `Grid` (`floodScratch`, `slideScratch`); `fillFrom` in
shape.go, which floods with a `slideQueue` too, uses the same one. Budget
-3.6% / -3.9% / -2.3% bytes on valley / ancient / globe128. globe256:
8.850 s ± 5% against 8.806 ± 10%, ~ (under load); B/op -2.58%.

### Item 2: scratch on the grid (c53ffbc)

Every tile-sized `make` in `pool`, `flow`, `waterStep` (with `receivers`,
`deepReceivers`, `stackOf`, `fillFrom`, `edgeWork` and `stillWork`) and
`creep` is a slice kept on the `Grid` - `poolScratch`, `flowScratch`,
`stepScratch`, `creepScratch`, `fillScratch` - fitted by `sized`, which
remakes a slice only when its length is not the tile count. Where a pass
relied on `make` zeroing, it clears first: `gathered`, the flood's
`stand`/`reached`/`from`, the lakes' pools, the step's `f`, `drop`,
`settle`, `rock`, `eff`, `abrade`, `stackOf`'s tally, creep's `k` and
`gives`. `stand`, which `pool` calls while its own stack still has basins
on it, walks the tree on a slice of its own; the first draft shared it,
and the digest would have caught that. `Clone` leaves all of it nil.
Budget -12.4% / -20.8% / -9.5% bytes against item 1. globe256: 8.749 s
± 7%, ~ (under load); B/op -11.87% against the base.

### Item 3: pool repairs the last drain's order (39c2a0b)

The order is total, so the last call's `order`, its heights read again, is
put right with an insertion pass, and sorted afresh where more than a tenth
of the entries are out of place or the shifting has run to four times the
tiles. **Measured on globe256, 30 pools:** every one of the history's 22
drains gave up - an epoch moves the ground by kilometres - hitting the
shift bound with under a tenth of the entries moved, so the bound was set
where giving up costs about a tenth of the sort it then does (with the
bound at sixteen times the tiles the aborted pass cost as much as the
sort). The seven drains after the history found 0, 124, 171, 0, 0, 0 and
0 of 32768 entries out of place, shifted under a fifth of the tiles, and
were put right in under half the sort's time. So the item buys a little
on the drains outside a history and nothing inside one; the brief's
premise that the ground moves little between drains holds for a
settlement's decade and the making of a map, not for an epoch. The sort
itself (3 s of the globe's 100) is what is left; a radix sort on the
height's bits with the index as tie-break would give the same total order
and is the next thing to try there. Budget unchanged. globe256: 8.724 s
± 8%, ~ (under load).

### Item 4: `fluvial.solve` by outlet tree (cfdbc51)

Read `solve` and `account`. A tile's implicit step reads its receiver's new
height and what its donors passed it, and writes its own height, cut, load
and rate: nothing in `solve` reads a tile outside its own tree. `forest`
lays the stack out tree by tree, each tree's tiles in the order the whole
stack had them - the stack is breadth-first from all roots at once, and
its restriction to one tree is that tree's own breadth-first order, so
what arrives at a tile is summed in the order it always was - cuts the
trees into runs of about 4096 tiles, and deals the runs to `InParallel`;
each run does all its sweeps on its own. Digest unchanged at Workers 1, 4
and 24 (a throwaway test, not committed). `account` stays one pass on one
goroutine, and the comment says why: `lay` puts what a river lays over its
banks onto ground beside it (`overbank`), which may be in another tree, and
`exported`, the bays' pools and the surf's sands are sums over the whole
map in stack order. The solve's `next`, `cut`, `load`, the forest and
`account`'s load live in a `solveScratch` the step hands the fluvial from
the grid. Budget -2.9% / -4.9% / -2.1% bytes against item 3; allocations
+0.4% on globe128 for the goroutines dealt. globe256: 8.607 s ± 6% against
8.806 ± 10%, ~ (under load); B/op -13.80% against the base.

The fifth item, `fillFrom` as a bucketed flood, was not started.

### After the rebase onto main (639d7d5)

Main took the deep sea floor, the rock chemistry, the weather gate, the
clock by pass and the softened winters while this ran, and those move the
world, so `budget.json` was rewritten at the tip - not because these items
moved anything. The proof is the same as before, taken again on the new
base: main's own `digest.json` (current: written again from main's tree
and identical to the committed file) is what the tip makes, at Workers 1,
2, 3, 4, 8 and 16 (`TERRA_DIGEST=check` and the goroutine test both pass
at the tip). Budget at the tip against main, Workers 4: valley 14.86 ->
10.33 MB (**-30%**), 1519 -> 1299 allocs; ancient 84.44 -> 58.20 MB
(**-31%**), 12066 -> 10217 allocs; globe128 525.3 -> 437.5 MB (**-17%**),
36198 -> 33584 allocs. The percentages are larger than before the rebase
because the weather gate took much of the other allocation away. An
earlier rebase, onto 41bd904, read globe256 plan commit against tip,
interleaved n=6, under load: 6.105 s ± 20% against 6.427 s ± 22%, ~
(p=1.000); B/op -18.2%.

The full suite on the pre-rebase branch failed only
`TestRealNumbers/Hack_exponent,_globe` (0.6005 against 0.54-0.6), which is
main's known failure; see the base run below.

### What needs attention next

- A quiet count-6 run of globe256 and a count-3 globe, to read what items
  1-4 bought on the clock; the tables above only show they cost nothing
  measurable under load.
- `wear` still makes `change`, `gained` and `lost` afresh each age, and
  `solve`'s forest is rebuilt every step though the receivers change only
  with the ground; both are easy scratch.
- `pool`'s sort on a history's drains: a radix sort by the height's bits
  (with -0 folded to +0, so the order is exactly `heightBefore`'s).

---

## 2026-09-16 - The clock by pass, committed; the night's timings were taken under load

**Branch:** `claude/perf-instrument` from main 1ad4985, plus
`claude/work-trees-performance-plan-4b623c` ([scaling-plan.md](scaling-plan.md),
[briefs/](briefs/), `TestWorldDigest` and [digest.json](digest.json)), then
main again at 41bd904 when the weather gate (the entry below) landed while
this session ran. The gate moved every world, so the digest was rewritten
to the merged worlds; main's own digest, taken with the same test in a
worktree at 41bd904, is identical to the branch's, which is the proof that
the timer leaves the worlds as they were. A, B and C's start-of-session
digests match the committed file only if they branch from main after this
merge. Main moved once more, to b8d3856 (the sea read against its row, more
heat traded on a globe), before this branch could land; it was merged in
turn, the digest rewritten again (globe128 moved, valley and ancient did
not), and main's own digest at b8d3856 is again identical to the branch's.
The suite was not run a third time: the code the branch adds is the same,
and the two runs below are its proof. One thing b8d3856 brought is
`TestWorldCreationBudget` failing on globe128, +3.27% bytes and +4.51%
allocations against [budget.json](budget.json), on main itself as much as
here, with the instrument off or on. That budget is the climate change's
to rewrite, so it is left as it is.

**The timer.** [phases.go](../../phases.go) makes the throwaway
`defer phase("name")()` of the first entry permanent: 27 passes, from
`Generate` and `history` down to `airEnv.vapour` and `fluvial.solve`, summed
by name under a mutex when `TERRA_PHASES=1`. `BenchmarkNewLand` reports each
pass as `s/<pass>` and `cmd/overview` prints the table. Three things learned
while making it hold the acceptance checks:

- The first version returned a fresh closure per call, and with the
  instrument on the budget test failed on allocations (valley +4.4%,
  ancient +3.8%). The entries are now made at init with one stop function
  and a stack of start times each, so a call allocates nothing: the budget
  passes with the instrument off and on (valley 1520 / 1513 allocations
  against 1524, within the goroutines' noise), and the digest is identical
  both ways.
- The environment is read at package init, before `go test` starts
  recording what a test reads, so the test cache does not know the setting
  changed: switching `TERRA_PHASES` needs `-count=1`. The first "on" runs
  came back `(cached)` from the "off" ones.
- The testing package keeps only the first lines of a benchmark's log, so
  the table logged by the benchmark is cut after eight rows. The
  `s/<pass>` metrics on the result line are complete; `cmd/overview` prints
  the whole table with the calls.

**Off, the instrument costs nothing measurable.** Main's test binary (at
1ad4985) and the branch's, interleaved on the same loaded machine:

| world | main | branch | | n |
|---|---|---|---|---|
| valley | 143.0 ms ± 13% | 167.4 ms ± 10% | +17% (p=0.015) | 6 |
| ancient | 526 ms ± 15% | 505 ms ± 14% | ~ (p=0.94) | 6 |
| globe256 | 7.27 s ± 8% | 7.10 s ± 5% | ~ (p=0.39) | 6 |
| valley | 162.1 ms ± 6% | 162.5 ms ± 7% | ~ (p=0.81) | 15 |
| ancient | 528 ms ± 10% | 534 ms ± 6% | ~ (p=0.78) | 15 |

The n=6 valley reading did not survive n=15; the off path is one bool read
and a deferred no-op per pass call, a few hundred per valley.

**The suite** (`go test -timeout 60m .`): the branch fails the same tests as main, both times it was run. At 1ad4985 (before the gate) branch and main each failed one test, `TestRealNumbers/Hack_exponent,_globe` at 0.6005 against 0.54-0.60 (1639 s and 1622 s under load). At 41bd904 (the merged branch, and main in a worktree at the same commit) both fail `TestTheTideLaysFlatsOnlyWhereItReaches` (small globe 4 has no flats), `TestRealNumbers/drainage_area_exceedance_exponent,_small_globe` (0.4894 against 0.39-0.46) and `TestRealNumbers/discharge_exceedance_exponent,_small_globe` (0.4912 against 0.40-0.46), and the globe Hack exponent passes (1092 s and 1090 s). Those three are main's, from the deep floor and the rock chemistry that came in with the gate's merge; see the memory notes on the coasts workstream.

**The machine was not quiet.** Four to ten test processes from other
sessions ran the whole night (their suites, one at a 120 min timeout), so
every timing here is under load. Before the merge, `scripts/perf.sh check`
against the quiet [baseline/2026-09-15-2230-small.txt](baseline/2026-09-15-2230-small.txt)
read valley +9.9%, ancient +12.4% (fails the 10% limit), globe256 -7.5%
(the precomputes, as measured when they went in); the A/B table above says
the ancient reading is load, not the timer. That count-6 run is kept as
[baseline/2026-09-16-small-under-load.txt](baseline/2026-09-16-small-under-load.txt),
named so that the script's newest-baseline glob (`*-small.txt`) skips it:
against the quiet baseline it reads valley +24%, ancient +26%, globe256 ~.

**The baseline for the morning is [baseline/2026-09-16-0039-small.txt](baseline/2026-09-16-0039-small.txt)**,
count 6, at the merged branch (timer and gate), taken in the quietest hour
of the night (four foreign test processes, mostly idle). Against the gate's
own [2026-09-15-2333](baseline/2026-09-15-2333-small.txt) it reads valley ~
(p=0.31), ancient +7.6% (p=0.009), globe256 -4.0% (p=0.026), with intervals
of ±9-11% on both sides, which is load on both sides. **Retake it with
`scripts/perf.sh baseline` on a quiet machine before checking A, B and C**;
a quiet run on this machine has intervals of ±4-6%, and a baseline taken
under load hides regressions of the load's size.

**The globe by pass**, from `TERRA_PHASES=1 go run ./cmd/overview -preset globe`
at the merged branch (83.2 s, with two suites running; the same run before
the gate, on a lighter load, made the globe in 77.0 s and is the second
column). Inclusive; a pass's time includes the passes it calls. The two
columns were taken under different loads, so read the calls and the shares,
not the seconds, across them.

| pass | wall s | calls | share | before the gate | notes |
|---|---:|---:|---:|---:|---|
| `Generate` | 83.2 | 1 | 100% | 77.0 (1) | |
| `history` | 62.7 | 1 | 75% | 51.6 (1) | 16 epochs |
| `drain` | 33.0 | 30 | 40% | 34.1 (30) | = weather + pool + flow |
| `wear` | 20.9 | 20 | 25% | 16.9 (20) | |
| `weather` | 17.9 | 21 | 22% | 22.4 (31) | the gate: 21 readings, 18 of them the epochs' |
| `rainOn` | 14.1 | 21 | 17% | 17.8 (31) | |
| `waterStep` | 9.2 | 26 | 11% | 7.1 (26) | |
| `flow` | 8.2 | 30 | 10% | 6.5 (30) | serial |
| `airEnv.vapour` | 8.2 | 66 | 10% | 9.2 (96) | on the workers under rainOn; summed over goroutines |
| `silt` | 7.4 | 1 | 9% | 9.6 (1) | includes drains and tides |
| `pool` | 7.4 | 30 | 9% | 5.9 (30) | full sort per call |
| `cutValleys` | 7.3 | 1 | 9% | 9.6 (1) | |
| `orographic` | 7.1 | 63 | 8% | 8.9 (93) | 3 wind phases per rain |
| `move` | 6.1 | 16 | 7% | 5.2 (16) | |
| `fluvial.solve` | 6.0 | 26 | 7% | 4.5 (26) | serial |
| `creep` | 5.0 | 20 | 6% | 4.6 (20) | serial stencil |
| `tectonics` | 4.6 | 16 | 6% | 4.0 (16) | |
| `windsFor` | 3.7 | 21 | 4% | 4.6 (31) | |
| `landslide` | 3.0 | 6 | 4% | 4.1 (6) | |
| `tides` | 2.4 | 7 | 3% | 2.1 (7) | |
| `airEnv.currents` | 2.3 | 21 | 3% | 2.7 (31) | serial |
| `shape` | 2.2 | 1 | 3% | 2.1 (1) | |
| `reshape` | 1.9 | 16 | 2% | 1.4 (16) | |
| `joinUp` | 1.9 | 30 | 2% | 1.5 (30) | |
| `keepBook` | 1.0 | 16 | 1% | 0.6 (16) | |
| `basins` | 0.3 | 1 | 0% | 0.3 (1) | |
| `settleRock` | 0.03 | 1 | 0% | 0.02 (1) | |

The shape of the first entry holds after the gate: `drain` is 40% of the
clock, and what is left of it is the 18 per-epoch weather readings the gate
keeps (brief A's remaining question, a model decision) and the serial
hydrology, `pool` + `flow` + `waterStep`, now 30% (brief B).

---

## 2026-09-15 - drain reads the weather only when the ground has moved

**Change:** `drain` used to call `weather()` (winds for every phase, then the
vapour budget and orographic rain) every time. It now calls it only when
`weatherStale` says the ground the air reads has drifted since the last
reading: more than 0.5% of tiles have gone under the air's sea or come out of
it (`weatherFlips`), or the height over that sea has changed by more than 1% of
its total, summed tile by tile (`weatherDrift`). The drift is measured against
the last *reading*, not the last drain, so small changes cannot pile up
unnoticed. `weather()` stores the snapshot (`Grid.aired`, one float32 per
tile).

**Why these limits:** I probed how much the ground moves between consecutive
drains on `globe256`:

| drain site | calls | tiles flipped | height drift |
|---|---|---|---|
| history, per epoch | 16 | 20-35% | 45-99% |
| history end, Generate after pour | 2 | 35-64% | large |
| cutValleys | 4 | 0.03-0.48% | 0.2-0.4% |
| Generate after relevel | 1 | 0% | 0% |
| silt | 6 | 0-0.35% | ~0% |

The limits sit well above what valley cutting and silt do and far below what
an epoch of the plates does. With them, `globe256` refreshes 20 of its 30
drains (all 18 history drains, the first after pour and one in
cutValleys); `ancient` refreshes 19 of 24; `valley` 1 of 6.

**What it bought** (benchstat, count 6, against
[baseline/2026-09-15-2230-small.txt](baseline/2026-09-15-2230-small.txt)):

| world | sec/op | B/op | allocs/op |
|---|---|---|---|
| `valley` | 135 ms -> 95 ms, **-29.5%** (p=0.002) | -41.7% | -52.2% |
| `ancient` | 420 ms -> 402 ms, ~ (p=0.093) | -11.2% | -11.9% |
| `globe256` | 7.34 s -> 5.62 s, **-23.5%** (p=0.002) | -24.0% | -28.6% |
| `globe` (count 2) | 81.5 s -> 67.3 / 68.5 s, ~-17% | 18.5 -> 16.5 GiB | 932 k -> 686 k |

`ancient` hardly moves because its drains are almost all history drains.
Other sessions' test binaries were running on the machine during these runs,
so the new timings are, if anything, pessimistic.
New baselines: [baseline/2026-09-15-2333-small.txt](baseline/2026-09-15-2333-small.txt),
[baseline/2026-09-15-2333-globe.txt](baseline/2026-09-15-2333-globe.txt).
`budget.json` was rewritten (globe128: 614 -> 480 MiB).

**Realism:** full suite before (33c942b) and after:

- `TestRealNumbers/Hack_exponent,_globe` fails both times (after: 0.6005
  against 0.54-0.6). This is the known failure on main.
- `TestTheRealWorld/meander_wavelength,_small_globe` now reads 13.6 widths,
  **inside** 10-14. Its known-gap marker (B, 14.6) therefore fails with "the
  gap has closed". This yardstick rests on about 21 reaches (see the
  yardstick sample-size notes), so a one-width move is within its noise. At the
  user's call the marker was taken off, and the yardstick now passes.
- After merging main's soil work (a60e3a5): `valley floor over hillslope
  soil depth, small globe` read 3.015x against 3-50. That is past its
  known-gap marker (I, 2.96x on main without this change). At the user's call
  it was taken off like the meander one. The margin is 0.5%, so the next
  change to soils or rain may put it back.
- Everything else passes. `TestMakingAWorldDoesNotDependOnTheGoroutines`
  passes: the gate is serial arithmetic and draws nothing.

**Where the weather time is now:** 18 of the globe's remaining ~20 weather
readings are the per-epoch history drains. Each epoch really does move the
ground a lot, so skipping more there is a model decision (for example,
reading winds every other epoch while keeping rain every epoch) rather than
a free speedup. It would need the full suite as its judge.

---

## 2026-09-15 - Precompute in `airEnv.vapour` and `fluvial.solve`

The same idea as the sea-warmth links: take the work that is constant during
a solve out of the sweep, keep the sweep order, and keep the bits. The world
digests for valley, glacial valley, ancient and globe128 are identical
before and after.

**`airEnv.vapour`** already built its upwind links once. What was left per
visit:

- The rain slope's constant factor `rainScale·rainSteep·toStep·rainMost/rainSteps`:
  three multiplies and a divide on every visit. It is now computed once per
  cell as `slope`, in the same order.
- `math.Max`/`math.Min`: out-of-line assembly calls on amd64 (`archMax`/`archMin`
  were 3.9 s + 2.1 s flat on the globe). The builtin `max`/`min` have the
  same NaN and signed-zero rules and compile inline.
- Locality: each cell's `give`, `lose`, `toStep`, `rainScale`, `slope`, four
  sources and four shares now sit in one `vapourCell`, so a visit reads one
  run of memory instead of eight scattered arrays. `zonalCorrection` reads
  the same cells.

**`fluvial.solve`:**

- Every step of a history has no settling anywhere (deep tiles skip it; see
  `waterStep`). The per-tile settle sums (`Σ settle·parts`,
  `Σ settle·(load+supply)`) and the `·(1−settle)` on what is passed on are
  now skipped when `settles()` finds nothing to settle. Leaving them out
  gives the same bits for any finite load: +0 times something finite added
  to +0 is +0, and multiplying by 1 changes nothing.
- The builtin `max` replaces `math.Max` in `rate`, `below` and `cutAt`.
- A precomputed `share` array for valleys was tried and dropped. It cost
  n x 8 bytes per solve and pushed the budget over, while history, the
  target, doesn't use it.

**CPU, full globe profile** (one run each; wall clock too noisy to read, see
below):

| function | before (cum) | after (cum) | |
|---|---|---|---|
| `airEnv.vapour` | 15.09 s | 9.82 s | -35% |
| `fluvial.solve` | 10.59 s | 5.06 s | -52% |
| `airEnv.zonalCorrection` | 1.96 s | 1.05 s | -46% |
| `math.archMax` + `archMin` | 5.98 s | 3.31 s | the rest is outside these passes |

**World** (old and new binaries interleaved, n=6; quiet run, CIs ±2-6%):

| world | before | after | |
|---|---|---|---|
| ancient | 0.467 s ± 6% | 0.468 s ± 4% | ~ |
| globe256 | 6.61 s ± 2% | 6.24 s ± 2% | **-5.63% (p=0.002)** |
| globe (2 runs each) | 76.9, 76.4 s | 82.8, 69.9 s | inconclusive |

The globe's two new runs are 13 s apart, so load noise swamps the effect at
n=2. `fluvial.solve`'s saving is serial and should show up on the clock. It
needs a quiet count-6 globe run to confirm.

**Memory: the budget was rewritten on purpose.** globe128 came out +1.30%
bytes against the budget, over the 1% slack. `vapourCell` holds one more
float per air cell than the arrays it replaced (`slope`), across every
vapour call of every drain. I judged a divide per visit worth 8 bytes per
air cell and rewrote `budget.json` with `TERRA_PERF_UPDATE=1`: valley
+0.41%, ancient +0.33%, globe128 +1.30% (this includes the +0.3% from the
sea links). Allocation counts went down 3-3.5% (fewer separate arrays).

---

## 2026-09-15 - Jacobi for the sea's warmth: tried, rejected; the equations precomputed instead

The sea-warmth solve in `airEnv.currents` was the largest serial pass left
on globe256 (4.5 of 27 profiled s). It sweeps each cell's upwind equation in
four orders, Gauss-Seidel style. The question was whether Jacobi ordering,
where every cell reads the round before, would make it vectorizable and
parallel.

**First, the constant work.** The equation for each cell does not change
during the solve, but the sweep recomputed it every time: the upwind cell
along the row, the corner search down the column within `cornerReach`, and
the weights. `seaLinks` now builds the equations once and the sweep only
reads them. Sums are taken in the same order as before, so worlds are
bit-identical (ancient and globe128 digests unchanged).

**Jacobi, measured** (sweeps summed over the whole world, not the time of
one solve):

| world | Gauss-Seidel rounds (x4 sweeps) | Jacobi rounds | world time GS / Jacobi |
|---|---|---|---|
| globe128 | 369 (1476 sweeps) | 3169 | 3.32 s / 3.50 s |
| globe256 | 775 (3100 sweeps) | 14571 | 7.27 s / 7.77 s |

The Jacobi rounds were spread over rows with `InParallel`. It still lost, for
three reasons:

1. Warmth moves one cell per Jacobi round, whereas one Gauss-Seidel sweep in
   the current's direction carries it the length of an ocean. It took 4.7x
   the sweeps. AVX2 is at most 4 lanes and needs a gather for the upwind
   reads, so vectors cannot win that back.
2. The air grid is coarse, so a row is too little work to be worth a
   goroutine.
3. It changes the world a lot. At globe256, 6.5% of tiles have different
   terrain, the maximum height difference is 75 m, and sea warmth on a tile
   is up to 3.7 C apart (7.6 C at globe128). Part of that is chaos over 16
   epochs. Part is that the same "settled" threshold (1e-3 C change in a
   round) stops a slow Jacobi solve while it is still far from the answer,
   so it would also need a different stopping rule.

Jacobi was removed. A note on it stays in `gaussSeidel`'s comment.

**The precompute, measured** (old and new binaries interleaved, n=6):

| world | before | after | |
|---|---|---|---|
| ancient | 0.542 s ± 13% | 0.522 s ± 22% | ~ (no sea currents) |
| globe256 | 8.24 s ± 6% | 7.70 s ± 5% | **-6.51% (p=0.009)** |
| globe256 B/op | 2.398 GiB | 2.406 GiB | +0.32% |

In the profile, `currents` went from 4.50 to 2.09 CPU s cumulative (-54%).

The first version allocated six new tile arrays per solve and failed
`TestWorldCreationBudget` at +1.58% bytes on globe128, which is the budget
test doing its job. The links now write over `cu`, `cv`, `rise` and `deep`,
which are dead once the solve starts, and only the two int32 index arrays
are new.

**Takeaway for the other serial sweeps** (`airEnv.vapour`, `fluvial.solve`):
before reordering a Gauss-Seidel sweep, pull the constant per-cell work out
of it. That keeps the world bit-identical and is where the time actually
was. Reordering pays only if the transport is local (diffusion-like);
upwind transport along a flow is exactly what Gauss-Seidel in flow order
does fast.

---

## 2026-09-15 - SIMD for the transform's butterflies

**Change:** `fft_simd_amd64.go` does the FFT butterflies two complex numbers
per AVX2 vector under `GOEXPERIMENT=simd` (following `pass_simd_amd64.go`).
The complex product is written as `x*p + swap(x)*q`, which gives the same
bits as Go's complex multiply. `TestTheButterfliesAreTheScalarOnes` checks
every length from 1 to 4096 in both directions, including negative zeros and
values of very different sizes. The whole-world digests (ancient, globe128)
match between the two builds.

**Kernel** (`BenchmarkFFT`, forward and inverse together):

| length | scalar | simd | |
|---|---|---|---|
| 64 | 1212 ns | 1050 ns | -13% |
| 256 | 7837 ns | 4320 ns | -45% |
| 1024 | 34701 ns | 17698 ns | -49% |

**World** (scalar and simd binaries interleaved, n=6 each):

| world | scalar | simd | |
|---|---|---|---|
| ancient | 0.491 s ± 17% | 0.459 s ± 12% | ~ (p=0.093) |
| globe256 | 7.33 s ± 3% | 7.24 s ± 4% | ~ (p=0.065) |

The kernel is twice as fast, but world creation gains 1-4%, which is not
significant. This is Finding 1 of the first entry again: the FFT runs on
the `InParallel` workers, and wall-clock time is set by the serial passes.
On globe256 the main goroutine does 19 of the 27 profiled seconds, and the
largest serial pieces are `airEnv.currents` (17% of wall), the rest of
`rainOn`, `windsFor`, `slideQueue`, `pool`/`flow` and the priority floods.

**Why the serial passes are not vectorized:** `currents`, `airEnv.vapour`,
`fluvial.solve` and the floods are Gauss-Seidel sweeps or priority-queue
walks. Each tile reads the value its neighbour was just given in the same
sweep, so doing four at once changes the result. Making them vectorizable
(Jacobi or red-black ordering) would also make them parallel, but it changes
the world and moves the realism tests. That decision needs an owner.

---

## 2026-09-15 - Drift guards

**Heap budget in the suite.** `TestWorldCreationBudget` holds `valley`,
`ancient` and `globe128` to [budget.json](budget.json) (1% bytes, 3% allocs,
Workers pinned to 4). Before settling on those limits I measured the spread
at Workers 1/4/24, two runs each:

| world    | bytes spread | allocs spread | time |
|----------|--------------|---------------|------|
| valley   | 25.41-25.42 MB (0.05%) | 3219-3243 (0.7%) | 0.13 s |
| ancient  | 94.708-94.715 MB (<0.01%) | 14025-14045 (0.1%) | 0.43 s |
| globe128 | 643.50-643.67 MB (0.03%) | 45871-46698 (1.8%, most of it from the worker count) | 3.2 s |

To check it catches something, I lowered valley's budget by 5.6%: the test
failed with `+5.93%`, then passed again once the file was restored. It adds
~4 s to the suite.

**Timing drift by script.** `scripts/perf.sh check` compares 6 runs against
the newest baseline with benchstat and fails at a significant +10%. I tested
it on a synthetic run with `ancient` and `globe256` scaled by 1.25: both were
flagged at `+25.00%` (p=0.004 / p=0.002) and the script exited 1. The
unchanged baseline passed with "no significant change" on every world.

**The first baseline was taken on a loaded machine, and has been replaced.**
The first real `check` on unchanged code came back 26-33% *faster* than the
committed baseline (valley 0.199 -> 0.135 s, ancient 0.631 -> 0.420 s,
globe256 9.90 -> 7.34 s), and its confidence intervals narrowed from
±6-16% to ±4-6%. Other sessions were running on this desktop while the first
baseline was taken. `baseline/2026-09-15-2230-small.txt` now holds the quieter
run, and the table below uses it.

The lesson for the tool: a baseline taken under load hides regressions of up
to the load's size, and a check run under load fails on nothing. Before
taking a baseline or trusting a failed check, close other heavy work, look
at the CIs (a quiet run on this machine is ±4-6%), and run it again if they
are wide.

---

## 2026-09-15 - First baseline and where the time goes

**Commit:** 7dca920 (main, after globe crust balance)
**Machine:** AMD Ryzen 9 3900X, 12 cores / 24 threads, Windows 11, go1.27.0
**Raw output:** [baseline/2026-09-15-2230-small.txt](baseline/2026-09-15-2230-small.txt) (count 6),
[baseline/2026-09-15-2230-globe.txt](baseline/2026-09-15-2230-globe.txt) (count 3)

### Baseline (benchstat)

| world      | sec/op        | ns/tile | B/op      | allocs/op |
|------------|---------------|---------|-----------|-----------|
| `valley`   | 0.135 ± 6%    | 47 k    | 24.2 MiB  | 3.2 k     |
| `ancient`  | 0.420 ± 4%    | 146 k   | 90.3 MiB  | 14.0 k    |
| `globe256` | 7.34 ± 5%     | 224 k   | 2.39 GiB  | 277 k     |
| `globe`    | 81.5 (81-112 over 5 runs) | 155 k | 18.5 GiB | 932 k |

- The globe is noisy: five single runs through the day read 93.5, 111.5,
  91.7, 81.5 and 81.4 s. Background load on a desktop moves it by 30%.
  Trust `globe256` with count 6 for decisions; treat the globe as
  confirmation.
- Allocation counts and bytes are deterministic (± 0%), so any change in
  B/op or allocs/op is real even on one run.
- Peak live heap for the globe (from `GODEBUG=gctrace=1`) is ~460 MB, heap
  size peaks ~900 MB, across 75 GC cycles. GC CPU is negligible (reported 0%);
  what the 18.5 GiB of allocation costs is zeroing and page faulting
  (`memclrNoHeapPointers` 3.2 s, `sysUnusedOS` 1.5 s of CPU), not marking.

### Finding 1: the wall clock is serial

The globe CPU profile: 111.6 s duration, 180 s of samples - **161% of one
core on a 24-thread machine**. Of that, 94 s is on the main goroutine (under
`Generate`) and 78 s is on `InParallel` workers. The spread passes finish
their 78 CPU-seconds in roughly 15 wall-seconds; the serial passes are the
other ~85% of the clock. By Amdahl, making every parallel pass free would
save at most ~15%. **The attention belongs on serial work and on how often it
is repeated.**

### Finding 2: wall clock by pass (globe, one run, 100.9 s total)

Throwaway `defer phase(...)()` timers, inclusive (a pass's time includes the
passes it calls), not committed:

| pass            | wall s | calls | share | notes |
|-----------------|-------:|------:|------:|-------|
| `Generate`      | 100.9  | 1     | 100%  | |
| `history`       | 68.0   | 1     | 67%   | 16 epochs |
| **`drain`**     | **44.6** | **30** | **44%** | = weather + pool + flow, ~1.5 s each |
| - `weather`     | 29.3   | 31    | 29%   | winds and rain recomputed from scratch every drain |
| -- `rainOn`     | 21.0   | 31    | 21%   | |
| --- `orographic`| 9.9    | 93    | 10%   | 3 wind phases per rain; FFT per patch |
| -- `windsFor`   | 8.3    | 31    | 8%    | |
| - `flow`        | 8.8    | 30    | 9%    | serial |
| - `pool`        | 7.4    | 30    | 7%    | full sort of all tiles by height each call |
| **`wear`**      | **23.3** | **20** | **23%** | |
| - `waterStep`   | 8.6    | 26    | 9%    | mostly `deepReceivers` 7.5 s / `fillFrom` 8.1 s (priority flood) |
| - `fluvial.solve` | 8.7  | 26    | 9%    | serial |
| - `creep`       | 5.5    | 20    | 5%    | serial stencil |
| `silt`          | 13.2   | 1     | 13%   | includes 6.5 s of drain, 2.2 s of tides |
| `cutValleys`    | 11.9   | 1     | 12%   | drain 4.2, landslide 3.7, wear 3.1 |
| `move`          | 6.7    | 16    | 7%    | |
| `tectonics`     | 5.0    | 16    | 5%    | |
| `landslide`     | 4.8    | 6     | 5%    | `slideQueue.pop` 6 s CPU flat |
| `turn`          | 4.8    | 16    | 5%    | |
| `tides`         | 2.8    | 7     | 3%    | |
| `shape`         | 2.6    | 1     | 3%    | |
| everything else | < 2 each |     |       | firstPlates, joinUp, reshape, closedBasins, keepBook, carve, ... |

### Finding 3: CPU hot spots (flat)

| function | flat s | cum s | where it runs |
|----------|-------:|------:|---------------|
| `fft` | 18.8 | 20.3 | workers (orographic) - complex128 butterflies |
| `slideQueue.pop` | 6.1 | 7.5 | main (landslide) - heap pop |
| `airEnv.vapour` | 6.1 | 14.4 | workers (rainOn) |
| `liftField` | 5.9 | 35.0 | workers - transfer function loop |
| `fluvial.solve` | 5.9 | 9.1 | main |
| `Grid.orographic` | 5.5 | 5.9 | **main** - the patch sum-back loop with `%` per tile |
| `math.archExp` | 5.5 | | both |
| `airEnv.currents` | 4.5 | 6.6 | main |
| `Grid.creep` | 4.2 | 5.8 | main |
| `runtime.complex128div` | 4.0 | 4.5 | workers (liftField) |
| `Grid.flow` | 3.4 | 8.8 | main |
| `Grid.pool` | 3.2 | 7.8 | main (incl. 3 s of `slices.SortFunc`) |
| `math.archMax` / `archMin` | 3.1 / 1.9 | | both - `math.Max`/`Min` not intrinsified here |

### Finding 4: allocation (globe, 18.5 GiB over the run)

| site | alloc | note |
|------|------:|------|
| `orographic.func1` `sum := make([]float32, box*box)` | 2.17 GB | one per patch per phase per rain; also 60% of all objects |
| `airEnv.vapour` | 1.70 GB (2.94 cum) | per rain |
| `waterStep` | 1.42 GB (2.36 cum) | per wear step |
| `airEnv.box` | 0.96 GB | |
| `Grid.pool` | 0.84 GB | `order` and `own`, n-sized, per drain |
| `Grid.creep` | 0.78 GB | |
| `Grid.rainOn` | 0.70 GB | |
| `Grid.orographic` | 0.60 GB | `out`, `acc` per call |
| `floodQueue.push` / `slideQueue.push` | 0.60 / 0.58 GB | queue growth |
| `newAirEnv` | 0.52 GB | rebuilt per windsFor |

By pass: `drain` 4.8 GB, `wear` 3.9 GB, `weather` 3.2 GB, `silt` 2.5 GB,
`cutValleys` 1.7 GB, `windsFor` 1.7 GB. Nearly all of it is tile-sized
scratch slices made fresh on every call of a pass that is called 20-90 times.

### What needs attention, in order

1. **`drain` recomputes the weather 31 times (29 s, 29%).** `weather()` builds
   the winds (`windsFor`, a fresh `airEnv`) and the rain from nothing on
   every drain. Between two drains in a history the ground has moved by one
   epoch's erosion. Options, in order of payoff: reuse the air environment
   and wind solution as a warm start; only refresh winds every k-th drain or
   when the coast/relief changed by more than a threshold; skip `weather`
   where the caller only needs the routing (`pool`/`flow`) on unchanged rain.
   Any of these touches realism tests - check the climate yardsticks before
   and after. This is the largest single lever.
2. **Serial hydrology: `pool` + `flow` + `deepReceivers`/`fillFrom` (~24 s).**
   `pool` sorts all 524k tiles by height every call (30x); `fillFrom` is a
   priority flood repeated 18x. Reuse the previous order (the ground moves
   little, so an insertion-sort pass or a bucketed flood is near linear), and
   keep the n-sized scratch (`order`, `own`) on the grid.
3. **`wear` is all serial (23 s).** `creep` is a stencil and splits by rows
   with `EachRow` the way the read-only passes do; `fluvial.solve` and
   `account` are worth a look for the same. Must keep
   `TestMakingAWorldDoesNotDependOnTheGoroutines` green: split arithmetic
   only, never draws.
4. **Orographic rain (10 s wall, ~40 s CPU, 2.8 GB).** Cheap wins: write patch
   results into a per-worker pooled buffer instead of `make` per patch; take
   the `%` wrap out of the sum-back inner loop on the main goroutine (5.5 s
   flat); replace `math.Max`/`Min` with the builtins where NaN cannot occur;
   hoist the complex division in `liftField`. Deeper: `fft` in complex128 is
   18.8 s of CPU - split real/imaginary float64 arrays or a precomputed
   per-level root table. This mostly buys CPU rather than wall clock (see
   Finding 1) unless item 1 is done first.
5. **`silt` and `cutValleys` (25 s together, run once).** Most of their time
   is the drains and wears they call, so items 1-3 carry them.
6. **Tectonics (`move` + `tectonics` + `turn`, ~16 s).** Serial per epoch;
   profile further before touching.
7. **Allocation churn (18.5 GiB, peak live only ~460 MB).** Keep tile-sized
   scratch slices on the `Grid` (or a per-pass arena) across calls. The GC is
   not the cost; zeroing and page faults are, so expect a few percent, not
   tens. Do it alongside items 1-4 rather than as its own project.

Smaller observations:

- `globe256` costs 302 µs/tile against the globe's 155 µs/tile: at that size
  fixed and per-call costs (air environment, FFT plans, goroutine fan-out)
  dominate. Don't read per-tile wins on `globe256` as proportional on the
  globe.
- `landslide`'s `slideQueue.pop` is 6 s of flat CPU for 6 calls; a 4-ary heap
  or a bucket queue is worth trying.

### Next entry should

- Pick item 1 or 2, change it, and record `globe256` count 6 benchstat
  against [baseline/2026-09-15-2230-small.txt](baseline/2026-09-15-2230-small.txt), plus
  the globe count 3, plus whether the realism/climate tests still pass.

## 2026-09-16: the softened winters move the heap, not the code

Merging `claude/amazing-tesla-0fc728` (the sea about a place read against its
row, and a stronger land-sea exchange on globes) failed
`TestWorldCreationBudget`: `globe128` at +3.26% bytes and +4.52% allocations.

It is the world and not a new pass. Taking the valley's second energy-balance
solve out changed the count by 25 allocations of the 1567, and the time came
out 14% *faster* in the same run. Softer winters leave more water on the
ground, so a globe carries more lakes and channels to allocate for.

The budget was rewritten at the user's call: `globe128` 525.3 MB in 36198
allocations, against 508.7 MB in 34646. `valley` moved 0.03%, which is the
run-to-run noise the README quotes; it reads its day's range at `contValley`
and is otherwise untouched.
