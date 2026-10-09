# terra

A world generator in Go, standard library only. `NewLand(seed, terms)` makes
a world; the root package is the whole of it. Read `README.md` for what a
world knows and `docs/perf/README.md` for how it is measured.

## The determinism contract

- A world is independent of `Workers`, always. The same seed and terms give
  the same bits over one goroutine or twenty-four; a pass spread with
  `InParallel` must reduce in a fixed order. `TestMakingAWorldDoesNotDependOnTheGoroutines`
  holds it.
- Worlds are not held bit-identical across intended algorithm changes. The
  realism yardsticks (`TestRealNumbers`, `TestTheRealWorld`) are the
  acceptance for those: a change that moves the world says so, and the
  yardsticks say whether it moved the right way.

A change that means to leave every world as it was proves it with
`TERRA_DIGEST=write` on its base commit and `TERRA_DIGEST=check` after
(`TestWorldDigest`). One that means to move the world rewrites the digest
and says so in its commit.

## Before merging

1. The PR description says what was measured, on what, what changed and
   what it bought. There is no work-log file to append to: an entry every
   branch writes at the top of one file conflicted with every other branch.
2. The budget diff: `TERRA_PERF_UPDATE=1 go test -run TestWorldCreationBudget .`
   when the heap is meant to move, committed with the change.
3. The digest written or checked (above).
4. `scripts/perf.sh check` on a quiet machine. Timing taken under load is
   noise; it never gates a merge on its own.
5. The yardsticks: `go test -run 'TestRealNumbers|TestTheRealWorld' -timeout 60m .`
   with the same failure list as main, or a reason for each new one.

## Running the tests

- Every `go test` of the root package takes `-timeout 60m`. The whole suite
  runs for twenty minutes and more; the default 600 s panics half-way.
- `go test -short -timeout 60m .` is the tier to run while working: under
  two minutes, nothing that makes a globe.
- Tests that need a world share one through `yardWorld` in
  `yardstick_test.go` rather than making their own; a test that needs its
  own copy calls `madeLand`, not `NewLand`, unless the making is what it
  tests.
- Histories are kept between runs in `.cache/histories`
  (`histories_test.go`), keyed by the package's source: a run with no
  change outside the tests reads them. `TERRA_HISTORIES=trust` reads them
  across a change too, for work on the stages after the history only;
  `TERRA_HISTORIES=off` makes every world afresh.
- `docs/perf/suite.md` lists the slowest tests.

## Worktrees

The git stash is shared with every worktree, and other sessions push and
pop it. Never use bare `git stash` or `git stash pop`. Set work aside with a
WIP commit; if a stash is unavoidable, `git stash push -u -m "<tag>"`, note
its SHA from `git stash list --format='%H %gs'`, and restore with
`git stash apply <sha>`.

## Performance

`docs/perf/README.md` is how a world is measured and held: the benchmarks,
the heap budget, `scripts/perf.sh check` and `scaling`, the digest, and the
profiles. `docs/perf/scaling-plan.md` is where world creation goes from
here and why. What each change measured and bought is in its PR and its
commit message; the work log kept until 2026-10-07 is in git history
(`git show 434c511:docs/perf/worklog.md`).

`Generate` runs in stages (`stages.go`); the first is the history. Work on
the later stages from a kept history (`cmd/overview -keep-history` once,
then `-from-history`); see the end of `docs/perf/README.md`.

## Duplication

[doppel](https://github.com/LukasSelin/doppel) reads every function in the
repo and reports the pairs that share shape and role. The repo enables its
Claude Code plugin (`.claude/settings.json`); it needs the `doppel` binary
on `PATH` (`go install github.com/LukasSelin/doppel@latest`). The plugin
gives each session the corpus at the start, names a file's twins before it
is edited, and says at the end of a turn which new merge-worthy pairs the
session made. `.doppel.json` holds the settings; `hook-notify` set to
`user` stops the end-of-turn note from costing a turn.

- Before writing a helper, ask whether terra has one:
  `doppel query --near <package> .` reads the draft on stdin and names its
  nearest relatives. The two commands under `cmd/` are both `main` to
  doppel, so `--near main` covers either.
- A new pair the Stop hook names is merged or the PR says why the two stay
  apart. The pairs that were there before the branch are not its business.
- `doppel analyze . -o doppel.html` draws the whole map; `doppel families .`
  lists the groups of three and more.
