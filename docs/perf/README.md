# Measuring how long a world takes to make

The yardsticks are `BenchmarkNewLand` in [bench_test.go](../../bench_test.go),
one sub-benchmark per world:

| name       | terms                          | tiles   | one run  |
|------------|--------------------------------|---------|----------|
| `valley`   | `DefaultTerms` (80x36, drawn)  | 2 880   | ~0.16 s  |
| `ancient`  | `AncientTerms` (80x36, 16 epochs) | 2 880 | ~0.6 s   |
| `globe256` | `GlobeTerms` at 256x128        | 32 768  | ~9 s     |
| `globe`    | `GlobeTerms` (1024x512)        | 524 288 | ~95 s    |

`globe256` runs every pass the globe does; iterate against it and confirm on
`globe`. Besides `ns/op` each reports `ns/tile`, `B/op` and `allocs/op`.

The work log - what was measured, what was found, what was changed and what
it bought - is [worklog.md](worklog.md). Raw benchmark output that later runs
are compared against lives in [baseline/](baseline/).

The module depends on nothing but the standard library, so benchstat is run
with `go run` rather than added to `go.mod`.

## A before/after comparison

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
go run golang.org/x/perf/cmd/benchstat@latest docs/perf/baseline/2026-09-15-small.txt new.txt
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
  `GODEBUG=gctrace=1` output (`X->Y->Z MB`, the middle number is live heap
  after a cycle).

## Wall clock per pass

The profiles do not say how long each pass took on the clock. The
2026-09-15 entry in the work log did it with throwaway instrumentation: a
`defer phase("name")()` at the top of each pass in `Generate` and `history`,
summing `time.Since` into a map under a mutex, and printing the map after one
`NewLand(1, GlobeTerms())`. It was not committed; redo it the same way when
the table in the log needs refreshing.
