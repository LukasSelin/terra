# Session 0: land what is ready, and commit the instrument

Run this one alone, before A, B and C start. It merges two branches that
are measured and bit-identical, adds the per-pass timer every later
measurement uses, and takes the quiet baseline the morning compares
against. Because it runs alone, this is the one session whose timings can
be trusted.

## Start

- Branch from main: `claude/perf-instrument`.
- Merge `claude/work-trees-performance-plan-4b623c` (the plan, these
  briefs, `TestWorldDigest`).
- Merge `claude/suspicious-austin-578279` (the `currents` precompute and the
  SIMD FFT; the work-log entries come with it). If a file named
  `zz_digest_test.go` appears, delete it: `TestWorldDigest` replaces it.
- `TERRA_DIGEST=write go test -run TestWorldDigest -timeout 60m .` and
  commit `docs/perf/digest.json`. This is the digest of main plus two
  changes that were measured bit-identical, and it is what A, B and C's
  start-of-session digests will match.

## The phase timer

The per-pass wall-clock table in the 2026-09-15 work-log entry was made
with throwaway `defer phase("name")()` instrumentation and thrown away.
Make it permanent, in a new file `phases.go`:

- `phase(name string) func()` returns a function that, deferred, adds the
  elapsed time and one call to the entry for `name`. When the instrument is
  off it returns a shared no-op and does no work beyond one package-level
  bool read; the world must cost nothing extra with it off.
- On with `TERRA_PHASES=1` in the environment, read once at init.
- A package-level `Phases() []Phase` (name, calls, wall seconds, inclusive)
  and `ResetPhases()`, so a benchmark or a test can read the table.
- A mutex round the map; passes run on the main goroutine, but `phase` must
  be safe if a spread pass ever calls it.
- Instrument, inclusive: `Generate`, `history`, `move`, `joinUp`,
  `tectonics`, `reshape`, `keepBook`, `settleRock`, `basins`, `drain`,
  `weather`, `windsFor`, `rainOn`, `orographic`, `airEnv.vapour`,
  `airEnv.currents`, `pool`, `flow`, `wear`, `waterStep`, `fluvial.solve`,
  `creep`, `landslide`, `shape`, `cutValleys`, `silt`, `tides`. One line at
  the top of each: `defer phase("drain")()`.
- `BenchmarkNewLand` reports each phase as a metric `s/<name>` when the
  instrument is on, so `benchstat` can compare passes between runs.
- `cmd/overview` prints the table after making a world when the instrument
  is on, sorted by wall time, in the same columns as the work-log table.
- Regenerate the "wall clock by pass" table for the globe from it and put
  it in the work-log entry, so the next person has a table that came from
  the tool rather than from a hand.

## Acceptance

1. `TERRA_DIGEST=check` passes with the instrument off and with it on.
2. `go test -run 'TestWorldCreationBudget|TestMakingAWorldDoesNotDependOnTheGoroutines' -timeout 60m .` passes. If the budget moved (it should
   not; the timer allocates nothing per tile), say why.
3. The full suite, `go test -timeout 60m .`, has the same failure list as
   main. Run main's first if you do not have the list; the known failures
   on main at 6c35a66 are recorded in the memory notes as globe water
   share, schist on seed 10, the tide-flat tests, and small-globe discharge
   and area exceedance, but take the list from a run, not from this.
4. `scripts/perf.sh check` against `docs/perf/baseline/2026-09-15-small.txt`
   reports no significant slowdown. Nothing else may be running.
5. Then `scripts/perf.sh baseline` to write tonight's baseline, and one
   `go test -run '^$' -bench 'NewLand/globe$' -benchtime 1x -count 3 -timeout 60m .`
   with `TERRA_PHASES=1` for the table.

## Finish

- Work-log entry at the top of `docs/perf/worklog.md`: the merge, the
  timer, the benchstat table, the phase table.
- Merge `claude/perf-instrument` into main with a merge commit that says
  what came in. This is the one overnight session that merges, because A, B
  and C build on it.
- Report: the branch and merge commit, the failure list, the baseline
  file's name, and the phase table.
