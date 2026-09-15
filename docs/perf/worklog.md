# World creation performance work log

Newest entry first. Each entry says what was measured, on what, and what it
means; changes say what they bought in benchstat terms. How to take the
measurements is in [README.md](README.md).

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
baseline was taken. `baseline/2026-09-15-small.txt` now holds the quieter
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
**Raw output:** [baseline/2026-09-15-small.txt](baseline/2026-09-15-small.txt) (count 6),
[baseline/2026-09-15-globe.txt](baseline/2026-09-15-globe.txt) (count 3)

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
  against [baseline/2026-09-15-small.txt](baseline/2026-09-15-small.txt), plus
  the globe count 3, plus whether the realism/climate tests still pass.
