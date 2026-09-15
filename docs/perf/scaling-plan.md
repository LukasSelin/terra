# Making a world fast, and keeping it so

Written 2026-09-15 against main at 6c35a66, from the profiles in
[worklog.md](worklog.md) and the work open in the worktrees that day. It is
a plan in two halves: what to change now to bring the globe down from a
hundred seconds, and what to change in the shape of the code so that a
world of many more epochs, at many more tiles, is a matter of waiting longer
and not of rewriting anything.

The numbers are the 2026-09-15 globe (1024x512, 16 epochs, 100.9 s wall on
a Ryzen 9 3900X). Every pass and figure named here is in the work log.

## 1. Where the worktrees stand

Forty-two worktrees are checked out. Seven carry work main does not have;
the rest are at or behind main with nothing ahead and can be pruned. Of the
seven, two are performance work and five are realism work that raises the
performance bar.

| worktree | branch | state | what it means for performance |
|---|---|---|---|
| `world-creation-profiling-e734ef` | `claude/world-creation-profiling-e734ef` | 3 files uncommitted | **The largest lever, half done.** `drain` now asks `weatherStale` before rebuilding winds and rain, and only rebuilds once 0.5% of tiles have flipped wet/dry or the air-facing relief has moved 1%. Not yet run through the yardsticks or the heap budget. |
| `suspicious-austin-578279` | `claude/suspicious-austin-578279` | 2 commits, 1 stray file | **Ready to merge.** The sea-warmth equations in `currents` are built once per solve (globe256 -6.5%, p=0.009, worlds bit-identical). FFT butterflies under `GOEXPERIMENT=simd` (kernel -45%, world -1..4%, not significant). Jacobi ordering was tried and rejected with measurements. Remove `zz_digest_test.go`. |
| `awesome-poincare-7d6415` | `claude/awesome-poincare-7d6415` | 19 files + `abyss.go` uncommitted | Realism: sea floor sinks with crust age, uplift carried on the crust. Adds three tile-sized arrays to `crust` and two to `Grid`; touches `creep`, `Surface`, `TempAt`. Needs a budget diff. |
| `cool-kare-9bb17e` | `claude/cool-kare-9bb17e` | 1 commit, 17 behind | Realism: rock chemistry, warm-sea limestone. Per-tile work in `keepBook`. |
| `amazing-tesla-0fc728` | `claude/amazing-tesla-0fc728` | 4 commits | Realism: globe warmth exchange and swing. Small. |
| `heuristic-nash-707393` | `claude/heuristic-nash-707393` | 2 commits | Soil yardsticks (tests only). Adds to suite time. |
| `cool-wu-1adbf4` | `claude/cool-wu-1adbf4` | 1 commit, 75 behind | Latitude warmth fit, superseded by later climate work. |

The realism branches are the headwind. Each adds a pass or a field per tile
per epoch, and the root package's test run has gone from 577 s to 1150 s to
1568 s across the last two days of merges. Nothing in them is wrong; they
are the reason the guards in section 5 have to exist before more of them
land.

## 2. What the measurements say

Three findings from the work log decide everything below.

**The clock is serial.** The globe uses 161% of one core on a 24-thread
machine. The passes spread with `InParallel` (orographic FFTs, vapour,
wind) finish their 78 CPU-seconds in about 15 wall-seconds. The other 85
seconds are one goroutine: `pool` sorting every tile, `flow` walking a
priority queue, `fluvial.solve` in stack order, `creep`, `currents`,
`slideQueue`. Making every parallel pass free saves 15%. Making the serial
passes parallel or linear is where the time is.

**The same work is repeated thirty times.** `drain` runs 30 times over a
globe (16 epochs, 4 valley rounds, ~6 in `silt`, and the rest) and rebuilds
the winds and rain from nothing each time: 29 s of the 101. Between two of
those calls the ground has usually moved by decimetres.

**Allocation is churn, not residency.** 18.5 GiB allocated for a 460 MB
peak live heap. Nearly all of it is tile-sized scratch made fresh inside
passes that are called 20-90 times. The GC does not mark it; the cost is
zeroing and page faults (about 5 s of CPU).

| pass | wall s | calls | share |
|---|---:|---:|---:|
| `history` (16 epochs) | 68.0 | 1 | 67% |
| `drain` = weather + pool + flow | 44.6 | 30 | 44% |
| of which `weather` | 29.3 | 31 | 29% |
| of which `flow` (serial) | 8.8 | 30 | 9% |
| of which `pool` (serial sort) | 7.4 | 30 | 7% |
| `wear` (all serial) | 23.3 | 20 | 23% |
| `silt` | 13.2 | 1 | 13% |
| `cutValleys` | 11.9 | 1 | 12% |
| `move` + `tectonics` + `turn` | 16.5 | 16 each | 16% |
| `landslide` | 4.8 | 6 | 5% |

## 3. What stops it scaling

Four couplings in the code fix the cost of a world, and each has to be
undone in turn for a bigger one.

1. **The history runs at the output resolution.** `history` moves plates,
   raises ranges, drains and wears on the same `Grid` the game will play
   on. Every epoch is a full pass over every tile, so a 4096x2048 world
   costs 16x the globe's history before any of the fine-grid work starts.
   The design for a coarse grid of its own is written at `epochYears` in
   history.go and has never been built.
2. **The hydrology is serial and superlinear.** `pool` is an O(n log n)
   sort of all tiles on every call. `flow`, `fillFrom` and `deepReceivers`
   are priority floods on a binary heap. `fluvial.solve` and `creep` are
   Gauss-Seidel sweeps. None spreads over goroutines, and all of them run
   inside the loops that run 20-30 times.
3. **The weather is rebuilt from nothing whenever the ground is asked
   about.** It is the dearest thing `drain` does and the one that changes
   least between calls.
4. **Peak memory is about 880 bytes a tile.** A 72-byte tile, a 60-byte
   strata column, a second copy of the crust while plates move, the air's
   fields, and the per-pass scratch. At that rate 8192x4096 needs 30 GB.
   `Terms.Largest` already sizes a world to free memory, so this is the
   ceiling on resolution today.

What does not limit scale: the int32 tile index caps a map at 2.1 billion
tiles (46,000 on a side), which is beyond the memory ceiling by two orders;
the goroutine-independence contract, which every parallel pass already
keeps and which the plan keeps.

## 4. The plan

Phases 0-2 are the near-term work and take the globe from 101 s to about
50 s without changing what any world looks like. Phases 3-4 are the
architectural work that lets epochs and tiles grow independently. Section 5
is the guards, which land alongside phase 0 and not after.

### Phase 0: land what is in flight (days)

- **Finish the weather gate** on `world-creation-profiling-e734ef`. Run the
  climate and river yardsticks on globe, small globes and valleys before and
  after; rewrite the heap budget; add the work-log entry with the drain-call
  count that still rebuilds. Expected: 14 of 31 rebuilds skipped (the ones
  inside `cutValleys`, `silt` and the closing drains), about -13 s on the
  globe. The realism branches should rebase onto it once merged, because
  each of them changes what "the ground has moved" means.
- **Merge `suspicious-austin-578279`**: the `currents` precompute is
  bit-identical and measured; the SIMD FFT is harmless and pays more once
  the serial passes shrink. Drop the untracked digest test, or turn it into
  the digest tool in section 5.
- **Commit the phase timer.** The per-pass table was made with throwaway
  `defer phase()` instrumentation and thrown away. Keep it, behind an
  environment variable (`TERRA_PHASES=1`), summing into a table that
  `BenchmarkNewLand` can report as sub-metrics. It is the instrument every
  later phase is measured with, and it should not be rebuilt by hand each
  time.

### Phase 1: hydrology in linear time, spread over goroutines (2-3 weeks)

The one goroutine that does 85% of the clock is doing the water. Each item
below keeps the world bit-for-bit, or is marked where it does not.

- **`pool`: stop sorting from scratch.** Keep `order` and `own` on the
  `Grid` between calls. The ground moves little between drains, so an
  insertion pass over the previous order is near linear. Where a fresh sort
  is needed, sort by row in parallel and merge. Exact.
- **`flow` and the floods: a cheaper queue, then a parallel flood.** First a
  4-ary heap or a radix bucket queue keyed on height with the index as
  tie-break (exact, the same pops in the same order). Then Barnes's
  parallel priority flood: tile the map, flood each tile on its own
  goroutine, merge the tile edges in a fixed order. Exact if the merge order
  is fixed; this is the step that makes `drain` scale with cores.
- **`fluvial.solve` per basin.** The implicit stream-power step walks each
  outlet's tree in stack order, and no tile reads a tile outside its own
  tree. Deal the trees to goroutines by outlet, in outlet order, and the
  answer is the same. Exact.
- **`creep`.** It is an implicit stencil. If the coefficients are
  symmetric it can be split by rows with a halo the way `EachRow` passes
  are; if not, this is the one place a red-black ordering would change the
  world and needs the owner's decision (section 6).
- **`landslide`: 4-ary heap** for `slideQueue.pop` (6 s CPU flat over 6
  calls). Exact.
- **Scratch on the grid.** Every tile-sized `make` inside `waterStep`,
  `creep`, `pool`, `flow`, `rainOn`, `orographic` becomes a slice kept on
  the `Grid` and resized only when the map is. Do it pass by pass alongside
  the items above, not as its own project. Expect a few percent of wall
  clock and most of the 18.5 GiB.

Expected together: about -25 s on the globe, and `drain` becoming a pass
whose wall time falls with the core count.

### Phase 2: the weather's own cost (1-2 weeks, in parallel with phase 1)

Once the gate in phase 0 has cut the calls, cut the call.

- `orographic`: write patch sums into per-worker pooled buffers instead of
  `make` per patch (2.17 GB, 60% of all objects); take the `%` wrap out of
  the sum-back loop on the main goroutine (5.5 s flat); `math.Max`/`Min` to
  the builtins where NaN cannot occur.
- `airEnv.vapour`: pull the constant per-cell work out of the sweep, as
  `currents` had done to it. The takeaway in the work log applies: do not
  reorder an upwind Gauss-Seidel sweep, precompute its equations.
- `liftField`: hoist the complex division.

Expected: -5 s wall, more in CPU. After phases 0-2 the globe is about 50 s
and the wall clock is no longer 85% one goroutine.

### Phase 3: the history on a grid of its own (the architectural change)

This is the change that answers "a lot of epochs at a great resolution".
Today the number of tiles the history moves plates over is the number the
game plays on, so every epoch costs the output resolution. It should cost
the physics.

- **Two grids, one world.** `history` runs on a deep grid whose tile span
  is chosen from the physics (`deepSpan` already computes one; the design
  note at `epochYears` asks for 1-5 km) and whose tile count is therefore
  set by the planet and the epoch length, not by `Terms.Width`. Its output
  is a stage: heights, rock columns, plate, crust age, uplift rate. The
  playable grid is then laid from that stage: heights bilinear as the uplift
  field `shape` grades to, rock and plate by nearest deep tile, the fine
  relief the shaping's own. This is what "keep real heights on the globe" in
  the tectonics scope is reaching for, and the two should be one piece of
  work.
- **Stages as values.** Split `Generate` into named stages with an explicit
  hand-off each: `history -> basins -> shape -> cut -> silt -> biota`. Each
  stage is a function of the previous stage and the seed. Give the history
  stage a binary encoding (there is none today; `grep` finds no encoder in
  the package) so a finished history can be written down and the fine
  stages rerun on it. A "history run" is then something you keep, and a
  change to `shape` is tested without 68 s of plates.
- **Many steps to an epoch.** With the deep grid its own size, the move can
  take many steps per epoch (the note wants ~100 at 2 km tiles) at a cost
  that no longer multiplies the output. The move is a full pass per step;
  phase 1's scratch reuse and row-spread arithmetic make it cheap enough.
- **Cost model after this phase:** `epochs x steps x c_h x H + c_r x R`,
  with H the deep tiles and R the playable tiles. The two terms are
  independent knobs, and the second is the one that scales with cores.

This changes what worlds look like and moves the realism yardsticks. It is
gated on the owner's decision in section 6 and should be built on its own
branch with the yardsticks run at every step.

### Phase 4: the fine grid at scale (after phase 3)

- **Memory to ~400 bytes a tile at peak.** Hot per-tile fields out of the
  72-byte `Tile` into slices (`Layers` already does this for the day's
  fields); `Sand`, `Clay`, `Drain` to float32 where the measured precision
  allows (the code already argues `Soil` at 0.1 mm in float32); the strata
  column compacted; scratch reused (phase 1). At 400 bytes, 8192x4096 is
  13 GB and fits a workstation.
- **Fine passes by chunk with halos.** `shape`, `cutValleys`, `silt`,
  `tides`, the woods and soils are local or nearly so. Run them per
  `ChunkSide` chunk with a halo, on goroutines, reducing in chunk order.
  The one global pass is `drain`, which phase 1's parallel flood covers.
- **Batch throughput now, without any of this.** At 161% CPU, eight seeds
  made at once on 24 threads finish in little more than one does. For a
  batch of worlds, run them as processes in parallel today; memory is the
  only limit (about 1 GB each at the globe).

## 5. Guards, so it does not drift back

The heap budget and `scripts/perf.sh check` exist. They catch bytes and
whole-world time on a quiet machine. What they miss is a pass that got
slower where another got faster, a superlinear step, a peak that grew while
churn fell, and an extra call to `drain` from a realism branch. These land
with phase 0.

1. **Per-pass timing as a first-class measurement.** The committed phase
   timer (phase 0) reports each pass as a benchmark sub-metric. The work-log
   table is regenerated from it, not typed.
2. **A scaling benchmark.** `BenchmarkNewLand/globe{128,256,512,1024}`
   reporting `ns/tile`. `perf.sh scaling` fails when `ns/tile` at 512 is
   more than 1.3x that at 256: a superlinear regression shows here before
   it shows on a user's big map.
3. **Peak live heap in the budget.** `B/op` is churn. Sample `HeapAlloc`
   through a world and budget the peak alongside bytes and allocs, because
   peak is the ceiling on resolution.
4. **Pass counts as a test.** `drain`, `wear` and `weather` are called a
   fixed number of times per preset. A test asserts the counts, so that a
   branch adding a drain shows it in the diff rather than in the benchmark.
5. **A digest tool for refactors.** Make the untracked `zz_digest_test.go`
   into `TestWorldDigest`: with `TERRA_DIGEST=write` it records a hash of
   ancient and globe128; with `TERRA_DIGEST=check` it compares. A perf
   change that should not move the world proves it in one command, and one
   that should says so with a budget and yardstick diff.
6. **Keep the determinism contract, and say what it covers.** Worlds are
   independent of `Workers`, always. Worlds are not held bit-identical
   across intended algorithm changes; the realism yardsticks are the
   acceptance for those. Write both sentences in a `CLAUDE.md` (there is
   none in the repository) beside the merge checklist: work-log entry,
   budget diff, `perf.sh check` on a quiet machine, yardsticks.
7. **A nightly perf run.** Timing on shared CI runners is noise. Schedule
   `perf.sh check` nightly on the desktop, writing a dated file to
   `docs/perf/baseline/`, so the manual step is not the step that gets
   skipped.
8. **Tier the test suite.** The root package takes 1568 s against a 600 s
   default timeout. Put every globe-making test behind `-short` or the
   existing `slow` flag, share made worlds through the `yardWorld` registry
   everywhere a test makes its own, and hold the short tier under two
   minutes so it is run.

## 6. Decisions for the owner

- **Reordering serial sweeps.** Recommendation: no for `currents` and
  `vapour` (upwind transport; Gauss-Seidel in flow order is the right
  algorithm and the measurements agree); yes for per-basin `fluvial` and
  the tiled flood, which are exact; `creep` only if its stencil is not
  symmetric, and then with yardsticks.
- **The deep grid (phase 3).** It changes every made world. It is the
  same decision as "keep real heights on the globe" in the tectonics scope,
  and should be taken once for both.
- **A history encoding.** New public surface. Recommendation: yes, private
  to `cmd/overview` and the tests at first.
- **Prune the worktrees.** Thirty-five are at or behind main with nothing
  ahead.

## 7. What it buys

| | now | after phases 0-2 | after phases 3-4 |
|---|---|---|---|
| globe 1024x512, 16 epochs | 101 s | ~50 s | ~30 s, falling with cores |
| 4096x2048, 16 epochs | ~30 min, 7 GB | ~15 min | history unchanged; fine grid ~2 min on 24 threads, ~7 GB |
| 64 epochs on the globe | ~5 min | ~2.5 min | history ~1.5 min at the deep grid's size |
| wall clock on one goroutine | 85% | ~50% | the flood and the move only |
| allocation per globe | 18.5 GiB | < 5 GiB | < 5 GiB |

The figures for phases 3-4 are estimates from the cost model, not
measurements, and the first thing phase 3 does is measure them.
