# Session A: finish the weather gate

The largest single lever in the plan, half done and uncommitted. `drain`
rebuilds the winds and the rain from nothing on every call - 31 times over
a globe, 29 of its 101 seconds - and between most of those calls the ground
has moved by decimetres. The gate keeps the ground the air was last read
over and only rebuilds once it has moved enough. This session takes it
through the yardsticks and the budget and leaves a branch ready to merge.

## Start

- Branch from main: `claude/weather-gate`. Merge main first; if
  `TestWorldDigest` is not there, merge `claude/work-trees-performance-plan-4b623c`.
- `$env:GOMAXPROCS=8`. Every `go test` on the root package takes
  `-timeout 60m`.
- **Before applying anything**, take the base readings on this exact
  commit:
  - `TERRA_DIGEST=write go test -run TestWorldDigest .`
  - the full suite: `go test -timeout 60m . 2>&1 | tee base-suite.txt`
    (outside the repo, in your scratch directory), and keep the list of
    `--- FAIL` lines. This is the list the branch is judged against. It
    takes about 26 minutes.
- Bring the uncommitted work over without touching its worktree:

  ```bash
  git -C C:/Source/repos/terra/.claude/worktrees/world-creation-profiling-e734ef diff > weather-gate.patch
  git apply weather-gate.patch
  ```

  It touches `grid.go` (an `aired []float32` field and a comment on
  `Clone`), `lake.go` (`drain` asks `weatherStale`), and `weather.go`
  (`weatherFlips`, `weatherDrift`, `airedGround`, `weatherStale`). Read all
  three. Commit it as it is, first, so that the diff of what you change is
  separate from the diff of what you were handed.

## Files you own

`weather.go`; the `drain` function in `lake.go` and nothing else in it;
the `aired` field, its `Clone` line and its comment in `grid.go`;
`docs/perf/worklog.md` (your entry), `docs/perf/budget.json`,
`docs/perf/digest.json`. Session B owns the rest of `lake.go` and
`erode.go`; do not touch them.

## Steps

1. **Count what the gate does.** With the phase timer if main has it
   (`TERRA_PHASES=1`, `weather` calls against `drain` calls), otherwise a
   temporary counter you remove: for valley, ancient, globe256 and the full
   globe, how many drains rebuilt the weather and how many did not. Put the
   four numbers in the work log. The plan expects about 14 of 31 on the
   globe to skip.
2. **Check that `Clone` does what its comment says.** A copy starts without
   `aired` and reads its weather afresh; confirm that a cloned map's first
   drain rebuilds, and that nothing downstream reads `aired` on a map
   that never had one.
3. **Yardsticks.** Run the full suite on the branch and diff its `--- FAIL`
   list against the base list. Every test that passed on base and fails on
   the branch is yours. For each, read what it measures and how many samples
   it rests on before deciding it is the gate's doing.
4. **If a yardstick moved:** halve `weatherDrift` (0.01 to 0.005) and rerun
   only the failing tests plus the goroutine test. If it still fails, halve
   `weatherFlips` once. Two attempts, then stop, record the readings at
   each setting in the work log, and leave the thresholds at the values the
   patch came with. Do not tune against a single reading.
5. **Budget and digest.** `TERRA_PERF_UPDATE=1 go test -run TestWorldCreationBudget .`
   and `TERRA_DIGEST=write`, and commit both files: the gate moves the
   world on purpose, and the rewritten digest is how it says so.
6. **Benchstat, under load.** `go test -run '^$' -bench 'NewLand/(valley|ancient|globe256)$' -benchmem -count 6 -timeout 60m .`
   before and after (the before can be the base commit in a second
   checkout of your branch, or `git worktree add` in your scratch
   directory), and one globe run with `-bench 'NewLand/globe$' -benchtime 1x -count 3`.
   Say in the work log that they were taken with two other sessions
   running.

## Acceptance

- `TestMakingAWorldDoesNotDependOnTheGoroutines` passes.
- The branch's failure list is the base list, or every difference is
  explained in the work log with the reading and its sample count.
- Budget and digest rewritten and committed.
- Work-log entry with the skip counts, the yardstick table, the
  benchstat tables marked as taken under load.

## Finish

Rebase onto main. Do not merge; the morning merges after
`scripts/perf.sh check` on a quiet machine. Report: the branch, the skip
counts, the yardstick differences, and the benchstat globe256 line.
