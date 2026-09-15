# Measuring how long a world takes to make

The yardsticks are `BenchmarkNewLand` in [bench_test.go](../../bench_test.go),
one sub-benchmark per world:

| name       | terms                          | tiles   | one run  |
|------------|--------------------------------|---------|----------|
| `valley`   | `DefaultTerms` (80x36, drawn)  | 2 880   | ~0.14 s  |
| `ancient`  | `AncientTerms` (80x36, 16 epochs) | 2 880 | ~0.42 s  |
| `globe256` | `GlobeTerms` at 256x128        | 32 768  | ~7.3 s   |
| `globe`    | `GlobeTerms` (1024x512)        | 524 288 | ~80-110 s |

`globe256` runs every pass the globe does; iterate against it and confirm on
`globe`. Besides `ns/op` each reports `ns/tile`, `B/op` and `allocs/op`.

The work log - what was measured, what was found, what was changed and what
it bought - is [worklog.md](worklog.md). Raw benchmark output that later runs
are compared against lives in [baseline/](baseline/).

The module depends on nothing but the standard library, so benchstat is run
with `go run` rather than added to `go.mod`.

## Keeping it from drifting

There are two guards, one for each thing that can drift.

### Heap: `TestWorldCreationBudget` (in the normal suite)

[budget_test.go](../../budget_test.go) makes three small worlds (`valley`,
`ancient`, and `globe128`, a 128x64 globe that goes through every globe pass)
and checks the bytes and allocation count against
[budget.json](budget.json). It fails if the bytes grow by more than 1% or
the allocation count by more than 3%. It takes about 4 s, and `-short` skips
the globe.

The allocations are the part a test can check on any machine. With the
goroutine count pinned to 4, the bytes vary by a few hundredths of a percent
between runs, so a pass that starts making a new tile-sized slice shows up
right away. The time is written to the file and logged (`time +7%, not
checked`), but it is never enforced there.

When a change is meant to move the heap, rewrite the budget and commit the
diff with the change:

```bash
TERRA_PERF_UPDATE=1 go test -run TestWorldCreationBudget .
```

If the test logs `under budget`, rewrite it too, so the next regression is
measured from the lower number.

### Time: `scripts/perf.sh check` (by hand, before merging perf-sensitive work)

```bash
scripts/perf.sh check
```

This runs `valley`, `ancient` and `globe256` 6 times each (about 2.5
minutes). It then runs benchstat against the newest `docs/perf/baseline/*-small.txt`
and **exits 1 if any world is slower by more than 10% and benchstat calls the
difference significant**. It says so when a world got more than 10% faster,
which is the cue to take a new baseline. It refuses to compare against a
baseline taken on a different CPU.

**Machine load is the biggest source of error.** A baseline taken while the
machine is busy hides regressions, and a check run while it is busy fails on
unchanged code. Close other heavy work first. If a check fails, or any CI in
the printed table is wider than about ±8%, run it again before believing it.

| env | default | |
|---|---|---|
| `PERF_THRESHOLD` | `10` | percent slower that fails |
| `PERF_COUNT` | `6` | runs per world |
| `PERF_BENCH` | `NewLand/(valley\|ancient\|globe256)$` | which worlds |
| `PERF_NEW` | | check a file already run instead of running now |

To take a new baseline after a deliberate change (or on a new machine):

```bash
scripts/perf.sh baseline
```

That writes `docs/perf/baseline/<date>-<HHMM>-small.txt`. Commit it with a
work-log entry that says why the baseline moved.

Why 10%: a quiet run on the 2026-09-15 machine has confidence intervals of
±4-6%, and the same code run while other work was loading the desktop came
out 30% slower. A tighter limit would fail on noise.
The significance test is what keeps a single slow run from failing. For
changes smaller than 10%, read the benchstat table the script prints.

## A before/after comparison by hand

Nothing else heavy should be running: these benchmarks use every core.

```bash
go test -run '^$' -bench 'NewLand/(valley|ancient|globe256)$' -benchmem -count 6 -timeout 60m . > old.txt
```

Make the change, then the same into `new.txt`, and:

```bash
go run golang.org/x/perf/cmd/benchstat@latest old.txt new.txt
```

For the full globe use `-bench 'NewLand/globe$' -benchtime 1x -count 3` (about
five minutes). benchstat wants at least 6 samples per side to report a
confidence interval it trusts; for the globe, three runs is a sanity check,
not a proof.

To compare against the committed baseline instead of a fresh `old.txt`:

```bash
go run golang.org/x/perf/cmd/benchstat@latest docs/perf/baseline/2026-09-15-2230-small.txt new.txt
```

Baselines are only comparable on the same machine (see the `cpu:` line).

## CPU and memory profiles

Build the test binary once so that pprof can resolve symbols, then profile
one world:

```bash
go test -c -o terra.test.exe .
./terra.test.exe -test.run '^$' -test.bench 'NewLand/globe$' -test.benchtime 1x -test.benchmem -test.cpuprofile cpu.pprof -test.memprofile mem.pprof -test.timeout 30m
```

Reading them:

```bash
go tool pprof -top -cum -nodecount=40 terra.test.exe cpu.pprof
```

```bash
go tool pprof -http=:8080 terra.test.exe cpu.pprof
```

```bash
go tool pprof -sample_index=alloc_space -top -nodecount=25 terra.test.exe mem.pprof
```

Things that are easy to misread here:

- **Parallel work is not under `Generate`.** Passes spread with `InParallel`
  run on their own goroutines, whose stacks start at `InParallel.func1`.
  `-focus Generate` is the serial (main goroutine) work; `-focus InParallel`
  is the spread work.
- **CPU seconds are not wall seconds.** Compare `Total samples` against
  `Duration`: at 24 cores a fully parallel run would show ~2400%. The
  2026-09-15 globe showed 161%, i.e. the wall clock is almost all serial work.
  A pass with 40 s of CPU spread over 24 goroutines may cost 10 s of wall
  clock; a serial pass with 20 s of CPU costs 20 s.
- **The memory profile is allocation, not peak.** `alloc_space` is what was
  asked of the heap over the whole run. For the peak live heap read the
  `GODEBUG=gctrace=1` output (`X->Y->Z MB`: heap at the start of a cycle, at
  its end, and live after it - the last number is the live heap).

## Wall clock per pass

The profiles do not say how long each pass took on the clock. The
2026-09-15 entry in the work log did it with throwaway instrumentation: a
`defer phase("name")()` at the top of each pass in `Generate` and `history`,
summing `time.Since` into a map under a mutex, and printing the map after one
`NewLand(1, GlobeTerms())`. It was not committed; redo it the same way when
the table in the log needs refreshing.
