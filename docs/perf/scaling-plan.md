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

### Track S: SIMD throughout (cross-cutting, from the first morning after the overnight run)

Added 2026-09-16. The owner wants the vectors used everywhere they can be.
What the toolchain offers, what the profile allows, and what the contract
demands together decide how.

**What there is.** Go 1.27's `simd/archsimd`, behind `GOEXPERIMENT=simd`:
`Float64x4` and `Float32x8` on AVX2, `Float64x8` and `Float32x16` on
AVX-512, `Float64x2` and `Float32x4` on arm64. Add, sub, mul, div, sqrt,
min, max, fused multiply-add, compares, masked select, permutes. No
gather, no scatter, no exp, log or pow. The package is experimental and
its API may move between Go versions. Two kernels exist already: the day's
pass (`pass_simd_amd64.go`) and the FFT butterflies, each with a scalar
twin and a test that holds the two bit-equal, and each clearing the upper
register halves on the way out (golang/go#80835).

**What it means.**

1. *Lanes load slices, not structs.* Without a gather, a vector kernel
   reads four consecutive float64s. `Tile` is 72 bytes with `Height`,
   `Flow`, `Drain`, `Soil`, `Sand` and `Clay` interleaved among bytes, so
   nothing on it can be vectorised as it stands. `Height` is read in 346
   places across 55 files; the six hot fields together in about 530. The
   day's fields already live in `Layers` as slices for this reason. The
   hot ground fields have to move the same way, and every pass and the
   `pool` sort get a cache win from it whether or not they use lanes: a
   sort over heights reads 8-byte strides instead of 72.
2. *Lanes need independent work, and the serial passes have none.* A
   priority flood pops one tile at a time; a Gauss-Seidel sweep reads what
   the last tile wrote; a heap and a sort are comparisons. Those are the
   85 s. SIMD in the arithmetic passes is worth about 10% of today's wall
   clock (the parallel share is 15 s of CPU-bound work on the workers)
   until the solvers are written in a shape that has lanes, which is a
   change of algorithm and of world, and the owner's decision.
3. *The contract holds if kernels come in pairs.* Add, sub, mul, div and
   sqrt round the same in a lane as in a register; fused multiply-add does
   not, and a vector `MulAdd` has to be matched by `math.FMA` in the twin
   or not used; reductions have to sum in the same tree order on both
   paths; `min`/`max` on a signed zero are written as compare-and-select.
   The existing kernels follow all of that. The rule becomes: no vector
   code outside a kernel, no kernel without a scalar twin, no twin without
   a test that fuzzes the pair bit-equal, including NaN, signed zero and
   values of wildly different size.

**The four shapes.** Every pass over the ground is declared as one of
these, in its comment, so that where lanes can go is decided once and
read by anyone:

| shape | what it is | threads | lanes | today |
|---|---|---|---|---|
| map | each tile from its own fields | rows | yes | the `EachRow` passes (33 of them), the day's pass, the wear update, `airedGround` |
| stencil | each tile from its neighbours' old values | rows with a halo | yes | slopes, `soften`, taper and sum-back in `orographic`, `creep` if made explicit |
| tree | each tile from its receiver, in dependency order | independent trees | across a level | `fluvial.solve` and `account`, `stackOf` |
| flood | one tile at a time from a priority queue | tiled with merged edges | none | `pool`, `flow`, `fillFrom`, `deepReceivers`, `landslide` |

The tree shape is the one that turns a serial solver into a vector one
without changing the world: a tile's implicit step reads only its
receiver, so every tile at the same depth from its outlet can be done at
once, level by level, and the levels are exact as long as donors are
summed into a receiver in index order. That is phase 1's per-basin item
taken one step further, and it is where the lanes in `wear` come from.

**The work, in order.** Each step is digest-exact unless marked, and each
lands behind the guards.

- **S1, the kernel layer** (one week, after the overnight merges).
  A `kernel_*.go` set: `axpy`, `lerp`, `clamp`, `fade`, `sumTree`,
  `stencil5`, `minmaxSelect`, later `expPoly`. Each with `_simd_amd64.go`
  and `_noasm.go` twins and one fuzzing test. Move the day's pass and the
  FFT into it. Add a `perf.sh simd` mode that benchmarks both builds, so
  the scalar path does not rot and the vector build's gain is a number.
- **S2, struct of arrays** (one to two weeks, sequential, its own branch).
  `Height`, then `Flow`, `Drain`, `Soil`, `Sand`, `Clay`, one field per
  commit, out of `Tile` into slices on the `Grid` beside `Layers`. The
  compiler finds the sites. `Tile` keeps the cold fields. Digest-exact.
  Breaks `lreat`, which reads `Tiles[i].Height` through the replace
  directive: decide whether to keep a `Tile` view for readers or to move
  it too.
- **S3, the map and stencil passes as kernels** (one to two weeks). The
  33 `EachRow` passes and the per-tile update in `wear`, one at a time,
  each measured with the phase timer. This is the 15% and the batch
  throughput; expect the parallel share to fall by two thirds.
- **S4, the tree shape for the water** (two weeks, after phase 1's
  per-basin item). Level-order `fluvial.solve`, with the lanes across a
  level. Exact. Then `creep` explicit with a stencil kernel, which is a
  world change and the owner's decision, or kept implicit and serial.
- **S5, transcendental kernels** (owner's decision). `math.Exp` and
  `math.Pow` are 5.5 s of the globe's CPU and have no vector form. A
  polynomial `expPoly` in both twins is bit-equal between them but not to
  `math.Exp`, so it changes every world once, on purpose, and is accepted
  by the yardsticks. Do it after S3, when the map passes are the ones
  waiting on it.

**Precision.** State stays float64; derived read-only fields are float32
where they already are (`warm`, `swing`, `tidal`, `rain` buffers) and
where a measurement argues for it, which doubles the lanes. Heights in
float32 lose the tenth of a millimetre at 8 km, so `Height` stays double.

**What it does not do.** It does not touch the wall-clock share of the
floods. Their scaling is threads over tiles (phase 1) and, beyond that,
the deep grid (phase 3), which shrinks the tile count the floods run
over. SIMD throughout and the deep grid are complements, not
alternatives.

### Level 2: the metre, and the export to Unreal (the end goal)

Added 2026-09-16. The owner's end goal is ground at one metre a tile
instead of twenty-five, exported to Unreal for use in game engines. The
metre is for looking, colliding, placing and exporting; the simulation
does not act at it. That settles the design: the metre is a level derived
from the map, never a state.

**The arithmetic.** `TileSpan` is one constant, and the map's physics
reads through it. At one metre the same ground is 625 times the tiles.

| world | tiles at 25 m | tiles at 1 m | peak today (880 B/tile) | at 400 B/tile |
|---|---:|---:|---:|---:|
| valley, 2 km x 0.9 km | 2,880 | 1.8 M | 1.6 GB | 0.7 GB |
| globe, 25.6 km x 12.8 km | 524 k | 328 M | 290 GB | 130 GB |

The valley at a metre is a change of one constant. Anything larger cannot
be one grid in memory on any plan. So the metre is a third level.

**Three levels, each derived from the one above.**

- Level 0, the planet, kilometres a tile: the history (phase 3).
- Level 1, the map, 25 m a tile: the state. Flow, drainage, wear, soils,
  the daily passes, routing, waking, the game. The finest thing ever whole
  in memory.
- Level 2, the metre: a pure function of the seed, a chunk, and its 25 m
  neighbourhood with a halo. Made per chunk, streamed to disk or cached,
  never whole. A 64 by 64 map chunk is 1600 by 1600 cells, 10 MB of
  float32 heights; the whole globe at a metre is 328 M cells, seconds of
  CPU in kernels spread over chunks, and one chunk per worker in memory.

**Why the metre keeps realism.** Every yardstick is a catchment-scale
fact - Hack, concavity, hypsometry, meander wavelength, drainage density -
and the 25 m level answers all of them; the metre cannot change them. What
is real at a metre is a different list, and each item is a function of the
25 m state: channel width and depth from the discharge, the banks and
levees the overbank pass already lays, terraces from the sea-level
history, ledges where the strata's beds crop out, talus at the angle of
repose under cliffs (`stand` in slide.go), the smoothness creep gives a
hillslope, beach and flat profiles from the waves and tides, and
microrelief conditioned on slope, soil and cover. Large scales simulated,
small scales conditioned, which is how real terrain is downscaled too.

**How a chunk is derived**, as map and stencil kernels on float32:

1. Heights by bicubic interpolation of the 25 m tiles, two tiles of halo,
   so hillslopes are C1 and show no facets.
2. Channels: a centreline through the tile centres along `down`, bent by
   the meander state; width and depth from `Flow` by the hydraulic
   geometry the fluvial step already uses; a parabolic bed cut below the
   surface; levees from what overbank laid.
3. Lakes flat at `lakeLevel`; the sea at `sea`; tidal flats at `ebb`.
4. Rock: where the soil is thin or the ground is `Rock`, ledges at the
   strata's bed tops with their dip, talus below at the angle of repose.
5. Microrelief: noise with amplitude by slope and thinness of soil and a
   spectrum by process, drawn from the seed and the global metre
   coordinates, so two chunks agree on their shared edge to the bit.
6. Cover at a metre: the weight of each surface (grass, forest floor,
   rock, sand, mud, snow and ice, salt, field, river bed) from `Terrain`,
   `Wood`, `Sward`, the soil's texture and the same conditioned noise.

**What Unreal takes.** A Landscape is a grid of quads, 100 units (1 m) a
quad by default, so a metre level imports one to one and the 25 m level
imports faceted at an XY scale of 2500. Heights are 16-bit, 32768 at zero,
with a range of 512 m times Z scale over 100: the globe's relief needs a Z
scale near 1000, which is steps of 8 cm, fine for looking at. One
Landscape actor is at most 8129 vertices a side (the recommended sizes are
1009, 2017, 4033, 8129); the globe at a metre is 25,600 by 12,800, which is
World Partition and a tiled import, 7 by 4 tiles of 4033 named
`name_x0_y0`. Weightmaps are 8-bit, one per surface layer, at the
heightmap's resolution. Rivers go to the Water plugin as splines with a
width and depth per point; lakes as water bodies with a level; the sea as
an ocean body at `sea`. Trees go as instance points (position, species by
biome, scale by `Age` and `Wood`) for foliage or PCG. A manifest carries
the seed, the terms, the origin, the scales and the sea level.

terra stays standard-library only: `cmd/unreal` writes 16-bit PNGs
(`image.Gray16`), 8-bit PNGs, JSON and CSV. The editor-side import is a
Python script that lives with the game, not here.

**Milestones.**

- **U1, the pipeline at 25 m** (days, new files only, can run alongside
  anything). `cmd/unreal` exports today's map as a tiled Landscape at 25 m
  quads with weightmaps, water bodies, foliage points and the manifest.
  Faceted, but it proves the whole path in the editor before the metre
  exists.
- **U2, the metre level** (two weeks, after S1 lands, since it is written
  in kernels). A `detail` stage: `DetailChunk(seed, chunk) -> heights,
  weights` from the six steps above. `cmd/unreal -m 1` streams the level
  chunk by chunk into the tiles. A golden-digest test per chunk, a seam
  test that two neighbours agree on their shared column, and a time and
  memory budget per chunk.
- **U3, the editor side.** The import script, the Water plugin bodies, PCG
  foliage, in the game's repository.
- **Later, edits.** If the game ever changes the ground at a metre - a dig,
  a wall - it is a per-chunk overlay of deltas over the derived level, not
  a metre state.

**What it changes elsewhere.** The hand-down from level 0 to 1 and from 1
to 2 are the same mechanism, a level made from the level above with a
halo: build it once, use it twice. The kernel layer (S1) and the
struct-of-arrays move (S2) are the foundation of level 2 as much as of
anything, because a chunk at a metre is millions of cells of float32
arithmetic and nothing else. Larger areas come from phases 3 and 4; finer
detail from level 2; a large world at a metre needs both.

### Track P: the causal record (why is this desert here?)

Added 2026-09-16. The owner wants every feature to carry its causal chain
and the world to answer for itself: *the Arken Mountains formed 18.4
million years ago from the collision of the Northern and Western plates;
they block the westerlies, and the country in their lee gets 240 mm a
year.*

**The principle.** The world is already causal: plates meet, the crust
crumples, the ground rises, the air climbs it and rains, the lee dries,
the woods thin. Every pass computes an effect from causes it reads. A
causal record is those passes writing down, beside the effect, which
cause and how much, at the level where the cause acts. It is never a
second model and never a narrative heuristic: an explanation is a reading
of recorded quantities, so it is as true as the simulation and no truer.
That is what keeps it honest, and it is what keeps it cheap.

**What exists already.** A history keeps a `record` per tile - melt,
pluton, crush, what was laid, epochs under the sea - and throws it away at
`settleRock`, leaving the tile its `Plate` and `Formed`. The strata keep
every bed with its epoch. The vapour budget keeps, per air cell and phase,
the water in the air, the evaporation, the rain and the orographic term.
Lakes know whether they are closed and salt. `cmd/overview` classifies
landforms and Köppen climates. The facts are mostly there; they are not
kept, not joined into things, and not askable.

**Three layers.**

1. *Facts, the ledger.* A few bytes per element at the level the cause
   acts, written by the pass that computes the effect:
   - the history (level 0): per tile, the meeting that raised it most -
     the two plates, its kind (collision, arc, rift, hotspot), the epoch,
     the metres - and the epoch and kind of its last burial. The `record`
     is kept as part of the history stage's output instead of dropped.
   - the air cells: per phase, beside the budget already kept, the upwind
     cell the water came from and the relief that wrung it out (the
     orographic patch's ridge cell). Cells are a small fraction of the
     tiles, so this costs almost nothing.
   - the map (level 1): nothing new per tile. The woods, the soil and the
     terrain are functions with known inputs, and an explanation reads
     the inputs: rain, warmth, drain, soil.
   - game time: an append-only list of events per feature - a lake gone
     to salt, a coast moved, a range worn - written by `Erode` and the
     sea-level cycle. Later.
2. *Features, the registry.* Things with an id, an extent and a few
   numbers, made once at the end of `Generate` and again after each
   `Erode`: uplift belts (the tiles raised by one meeting, which is what
   "the Arken Mountains" is), basins and rivers (the route trees by
   outlet), lakes, dry regions and other climate components, forests,
   coasts, plates. Ids are deterministic (ordered by lowest tile). terra
   does not name them - a name is a culture's - but it exposes a naming
   hook, and `cmd/overview` carries a default namer from the seed.
3. *Questions.* `Why(p, aspect)` walks the features at a tile and the
   ledger behind them and returns a chain: `[]Cause{Feature, Kind,
   Quantity, Unit, When}`. Structured first, so a game renders it its own
   way, so tests assert the chain rather than prose, and so the game can
   append its own causes (why the settlement is here) to the world's. The
   sentence is a renderer over the chain, in `cmd/overview` and in the
   game.

**Attribution has to be a rule, or it is a story.** "Rain shadow" is
claimed only by a defined reading of the recorded budget: a cell whose
orographic term is negative in the phase that carries most of its year's
water, or whose upwind cell's water was taken by a ridge within the
patch's reach, is in that ridge's shadow, by the share the ledger says.
And the counterfactual can be computed rather than asserted: `weather()`
costs about a second on the globe, so `WhatIf(feature)` flattens a belt
to the ground around it, reruns the weather on a copy, and reports the
rain the lee would have had. That is the one place the record does run
the model again, and it runs it on request, never during creation.

**The cost.** Roughly ten bytes per history tile and per air cell, held
by the heap budget; one union-find over the map per registry build,
about half a second on the globe; nothing per epoch beyond writes the
passes already make. The chain walk is microseconds.

**Where it lands in the architecture.** Each stage's output (phase 3)
carries its facts, so the ledger is the stages remembering what they
did. The registry is built from level 1 and hands the metre level and
the Unreal export their feature polygons and chains, so a game can show
them. The determinism contract covers the chain: the same seed gives the
same answer.

**Milestones.**

- **P1** (one week, after the overnight merges; touches `keepBook` and
  `settleRock`, which the abyss branch also touches, so rebase on it).
  Keep the book with the meeting and the burial. Registry for belts,
  basins, lakes, plates and climate components. `Why(p, aspect)` for
  height, rock, rain and cover. A "why" page in `cmd/overview` for a
  handful of tiles per preset. Tests: a fixed chain on a fixed seed; the
  budget diff.
- **P2**: the air-cell ledger, the shadow rule, and `WhatIf` by rerunning
  the weather.
- **P3**: events over game time, and the naming hook.
- **Export**: features and chains as JSON beside the Unreal manifest.

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
9. **Kernel pairs.** No vector code outside `kernel_*.go`; every kernel
   has a scalar twin and a fuzz test holding them bit-equal; `perf.sh
   simd` benchmarks both builds; the digest is written and checked under
   both `GOEXPERIMENT=simd` and the plain build.

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
- **The `Tile` API.** Moving the hot fields to slices (S2) breaks
  `Tiles[i].Height` for `lreat`. Keep a read-only `Tile` view, or move the
  game with it.
- **Bits that change once.** A polynomial exp in place of `math.Exp` (S5),
  an explicit `creep` (S4): each changes every world once and is then
  held by the pair test. Yes or no to each, judged by the yardsticks.

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
