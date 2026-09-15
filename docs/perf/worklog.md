# World creation performance work log

Newest entry first. Each entry says what was measured, on what, and what it
means; changes say what they bought in benchstat terms. How to take the
measurements is in [README.md](README.md).

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
