# Session C: the guards

The heap budget and `scripts/perf.sh check` exist. What they miss is a
pass that got slower where another got faster, a superlinear step, a peak
that grew while churn fell, and an extra call to `drain` from a realism
branch. This session builds the guards in section 5 of the plan that need
no change to how a world is made, so that it conflicts with nothing A and B
do.

## Start

- Branch from main: `claude/perf-guards`. Merge main first; if
  `TestWorldDigest` is not there, merge `claude/work-trees-performance-plan-4b623c`.
- `$env:GOMAXPROCS=8`. Every `go test` on the root package takes
  `-timeout 60m`.
- Read `docs/perf/README.md`, `budget_test.go`, `bench_test.go`,
  `scripts/perf.sh` and section 5 of `docs/perf/scaling-plan.md`.

## Files you own

`bench_test.go`, `budget_test.go`, `scripts/perf.sh`, `docs/perf/README.md`,
new test files, a new `CLAUDE.md` at the repository root, and `-short`
gates inside existing `_test.go` files. Nothing that makes a world: no
`.go` file without `_test` in its name, except `budget.json` if the peak
field is added.

## Items, in order

Each is one commit.

1. **A scaling benchmark.** `BenchmarkNewLand` gains `globe128` and
   `globe512` beside `globe256` and `globe`, each reporting `ns/tile`.
   `scripts/perf.sh scaling` runs 128, 256 and 512 with count 3 and fails
   when `ns/tile` at 512 is more than 1.3 times that at 256, printing the
   three figures. Say in `docs/perf/README.md` what it catches (a
   superlinear step) and what it does not (a slower constant).
2. **Peak live heap in the budget.** `TestWorldCreationBudget` samples
   `runtime.MemStats.HeapAlloc` while each world is made (a goroutine
   sampling every few milliseconds, or `runtime.ReadMemStats` after each
   pass if the phase timer is on main) and writes `peak` beside `bytes` and
   `allocs` in `budget.json`. Check it with the same 1% slack as bytes if
   the spread between runs allows it; measure the spread over three runs
   first and pick the slack from that, saying so in the README. Peak is
   what bounds resolution; `B/op` is churn.
3. **Pass counts as a test.** If main has the phase timer
   (`Phases()` in `phases.go`), write `TestPassCountsArePinned`: for valley,
   ancient and globe128, the number of calls to `drain`, `weather`, `wear`
   and `landslide` equals a table in the test. A branch that adds a drain
   then shows it in a test diff. If the timer is not on main yet, write the
   test against a stub and mark it skipped with a note, so that session 0's
   merge turns it on.
4. **`CLAUDE.md`.** There is none. Write one, short, with: the two
   sentences of the determinism contract (worlds are independent of
   `Workers`, always; worlds are not held bit-identical across intended
   algorithm changes, and the realism yardsticks are the acceptance for
   those); the merge checklist (work-log entry, budget diff, digest written
   or checked, `scripts/perf.sh check` on a quiet machine, yardsticks);
   the test-suite timeout; the shared stash rule for worktrees; and a
   pointer to `docs/perf/README.md` and `scaling-plan.md`. Keep it under
   sixty lines.
5. **Tier the suite.** Run `go test -json -timeout 60m . > suite.json` in
   your scratch directory and produce a table of the thirty slowest tests
   with their seconds, into `docs/perf/suite.md`. Then gate every test that
   takes more than 60 s and is not already skipped under `-short` with
   `if testing.Short() { t.Skip(...) }`, and where a test makes its own
   `GlobeTerms` world that `yardWorld` already caches, use the cache. Report
   the short tier's time; the target is under two minutes. Do not change
   what any test asserts.

## Stop rules

- Two failed attempts at an item, then write it up and move on.
- Nothing that changes how a world is made. If the peak sampler or the
  pass-count test needs a hook in non-test code, write the hook down in the
  work log for session 0's file and skip.

## Acceptance

- `go test -short -timeout 60m .` passes apart from main's known failures,
  and the full suite has the same failure list as main.
- `scripts/perf.sh scaling` runs end to end (its numbers tonight are under
  load and are not the point).
- `TestWorldCreationBudget` passes with the new field.
- Work-log entry: the suite table's top ten, the short tier's time, the
  peak's spread and chosen slack.

## Finish

Rebase onto main. Do not merge. Report: the branch, the short tier's
time, the thirty-test table's top five, and anything that needs session
0's file.
