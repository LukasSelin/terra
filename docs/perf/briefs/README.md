# Overnight briefs, 2026-09-15

Four sessions that carry out phase 0 and the exact half of phase 1 of
[../scaling-plan.md](../scaling-plan.md), and the first of its guards. One
runs alone before the others start; three run at once, on disjoint files.

| brief | runs | owns | merges to main? |
|---|---|---|---|
| [0-instrument.md](0-instrument.md) | alone, first | new file `phases.go`, the two branches it merges, `docs/perf/baseline/` | yes, when its acceptance passes |
| [A-weather-gate.md](A-weather-gate.md) | with B and C | `weather.go`, `drain` in `lake.go`, the `aired` field in `grid.go` | no |
| [B-hydrology.md](B-hydrology.md) | with A and C | `lake.go` (pool, flow, floodQueue), `fluvial.go`, `slide.go`, `waterStep` and `creep` scratch in `erode.go`, scratch fields in `grid.go` | no |
| [C-guards.md](C-guards.md) | with A and B | `bench_test.go`, `budget_test.go`, `scripts/perf.sh`, new `CLAUDE.md`, new test files, `-short` gates in tests | no |

## Rules every session keeps

- **Timing never gates a merge tonight.** Three sessions share the machine,
  so every benchmark is taken under load. Record benchstat tables, say they
  were taken under load, and merge on the deterministic checks only: the
  digest, the heap budget, the goroutine test, the yardsticks.
- **`GOMAXPROCS=8`** in every shell for A, B and C (`$env:GOMAXPROCS=8`),
  so three test runs share 24 threads. `Workers` reads the CPU count and is
  not affected, so the goroutine test still spreads.
- **`-timeout 60m`** on every `go test` of the root package. The plain
  command panics at 600 s.
- **Write your own digest at the start** (`TERRA_DIGEST=write`) on your
  base commit before touching anything, and check against it
  (`TERRA_DIGEST=check`) after every item. A session whose change is meant
  to move the world (A) rewrites it and says so in the work log.
- **Do not touch files another brief owns.** If an item needs one, write
  what it needs in the work log and skip the item.
- **Stop rule:** two failed attempts at one item, then write what was tried
  and what was measured under a heading in `docs/perf/worklog.md` and move
  to the next item. Never tune a constant against a single yardstick
  reading; several of the river yardsticks rest on a handful of samples.
- **Commit after every item** on your own branch, with a message that says
  what it bought. Rebase onto main before you stop. The morning merges on a
  quiet machine after `scripts/perf.sh check` on each branch.
- **Never `git stash` bare.** The stash is shared with other worktrees.
  Prefer a WIP commit.
- **The work-log entry** goes at the top of `docs/perf/worklog.md`: what was
  measured, on what, what changed, what it bought, in the style of the
  entries there.
