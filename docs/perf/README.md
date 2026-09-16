# Measuring how long a world takes to make

The yardsticks are `BenchmarkNewLand` in [bench_test.go](../../bench_test.go),
one sub-benchmark per world:

| name       | terms                          | tiles   | one run  |
|------------|--------------------------------|---------|----------|
| `valley`   | `DefaultTerms` (80x36, drawn)  | 2 880   | ~0.14 s  |
| `ancient`  | `AncientTerms` (80x36, 16 epochs) | 2 880 | ~0.42 s  |
| `globe128` | `GlobeTerms` at 128x64         | 8 192   | ~1.2 s   |
| `globe256` | `GlobeTerms` at 256x128        | 32 768  | ~7.3 s   |
| `globe512` | `GlobeTerms` at 512x256        | 131 072 | ~35 s    |
| `globe`    | `GlobeTerms` (1024x512)        | 524 288 | ~80-110 s |

Every globe on the ladder runs every pass the full globe does; iterate
against `globe256` and confirm on `globe`. Besides `ns/op` each reports
`ns/tile`, `B/op` and `allocs/op`.

The work log - what was measured, what was found, what was changed and what
it bought - is [worklog.md](worklog.md). Raw benchmark output that later runs
are compared against lives in [baseline/](baseline/).

The module depends on nothing but the standard library, so benchstat is run
with `go run` rather than added to `go.mod`.

## Keeping it from drifting

There are three guards: the heap, the clock, and how the clock grows with the map.

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

The file also carries a `peak`: the most the heap held at once while the
world was made, over what it held before. `bytes` is churn, what the world
asks of the allocator over its whole run; `peak` is what a machine has to
have, and so what bounds the size of world it can make. A sampler goroutine
reads the heap's live-objects metric every millisecond while the world is
made and keeps the highest reading, less the reading after the collection
that precedes the world.

**The peak is logged (`peak -3%, not checked`) and not enforced**, because
no reading taken from outside the world holds still under load. Three runs
on 2026-09-15 with two other test runs on the machine:

| world | sampled `HeapAlloc`, MiB | live at the collector's marks, MiB | live at a forced mark every MiB allocated, MiB |
|---|---|---|---|
| valley | 3.8, 3.8, 6.1 | 1.8, 2.1, 2.7 | 2.1, 3.8, 2.1 |
| ancient | 6.2, 14.1, 8.3 | 3.3, 6.3, 8.2 | 7.2, 6.7, 4.1 |
| globe128 | 21.0, 21.6, 22.1 | 11.3, 11.3, 15.8 | 20.7, 16.4, 15.0 |

Three ways of reading it spread by 40-130%, so it is not the sampler
missing the top and not the pacer's choice of when to collect. It is that
the collector is concurrent: whatever the world allocates during a mark is
counted live, and a mark takes longer when the machine is busy. Turning the
collector's percent down (25, 10) did not narrow the spread, and cost the
globe two seconds and a few hundred allocations. On a quiet machine the
sampled `HeapAlloc` came out within 1-5% run to run, which is why it is
the reading kept in the file: it is the honest one on the machine the
budget is written on, and the one to compare when a change means to move
the peak.

A steady peak needs the world to hold still while the heap is read: a hook
at each pass boundary (the phase timer in `phases.go`) that runs a
collection and reads `HeapAlloc` when the budget test asks it to. Every
pass then contributes one reading taken with nothing allocating, and the
largest of them is the peak, the same on any machine. When that exists,
set `peakSlack` in `budget_test.go` and the test holds it like the bytes.

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

### Scaling: `scripts/perf.sh scaling` (by hand, when a pass changes shape)

```bash
scripts/perf.sh scaling
```

This makes `globe128`, `globe256` and `globe512` three times each, one world
per run, and reads the median `ns/tile` of each width against the next. It
**exits 1 when a tile at 512 costs more than 1.3 times a tile at 256**
(`PERF_SCALING`), printing the three figures and the two ratios. About four
minutes on a quiet machine.

What it catches is a superlinear step: a pass whose cost per tile grows with
the map, which a quadratic pass shows as a ratio near 4 and a pass that
walks the whole map per row as one near 2. An `n log n` pass, the FFT, costs
about 1.13 per doubling of width, well inside the limit, so the limit is
not a proof of linearity: it is a tripwire for a step that would take a
user's big map from minutes to hours. What it does not catch is a slower
constant. A pass that got twice as slow at every size moves both medians
alike and leaves the ratio where it was; that is `check`'s to catch against
the baseline.

The check is 512 against 256 and not 256 against 128 because the bottom
rung is still paid for mostly by the constant each world carries (the
2026-09-15 run, under load, had `globe128` at twice the ns/tile of
`globe256`); `256/128` is printed for the record only.

The ratio is read from three runs' medians, not one run, because one
interrupted run of `globe512` would otherwise fail it. It still moves under
load, which slows a long run more than a short one: a failure taken on a
busy machine is run again before it is believed, like `check`.

### The vector build: `scripts/perf.sh simd` (by hand, when a kernel changes)

```bash
scripts/perf.sh simd
```

The kernels in [kernel.go](../../kernel.go) - the day's fade and grow, the
transform's butterflies, axpy, lerp, clamp, sumTree, stencil5 and
minmaxSelect - are each written twice: the statement one number at a time,
and the same four lanes at a time in `kernel_simd_amd64.go`, which is built
only under `GOEXPERIMENT=simd` and used only on a processor with AVX2.
`kernel_test.go` holds every lane to the statement bit for bit (a NaN to a
NaN), so the two builds make the same worlds; `TERRA_DIGEST=check` on both
is the proof for a change to a kernel.

This mode builds the test binary both ways, runs `valley`, `ancient` and
`globe256` on each turn and turn about `PERF_COUNT` times, and prints
benchstat with the scalar build as the old column. It **never fails**: it
is the vector build's gain as a number, and the scalar build run beside it
so that neither path rots unmeasured. A world is set by its serial passes,
so the gain on a world is small (see the work log); the kernels on their
own are

```bash
PERF_BENCH='Kernel|FFT' scripts/perf.sh simd
```

The two binaries run alternately rather than one after the other so that
a drift in the machine's load falls on both alike. On a processor without
AVX2 the two columns are the same code, and the table says so.

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

The profiles do not say how long each pass took on the clock. The timer
in [phases.go](../../phases.go) does: every pass worth a line in the work
log starts with `defer phase("name")()`, and with `TERRA_PHASES=1` in the
environment the wall time and the calls are summed under that name. Off, a
pass pays one bool read; on, nothing is allocated while a world is made, so
the heap budget holds either way.

The table for the globe, in the columns the work log uses:

```bash
TERRA_PHASES=1 go run ./cmd/overview -preset globe
```

The benchmark reports each pass as a metric, `s/<name>`, so benchstat can
compare passes between runs, and logs the table with the calls:

```bash
TERRA_PHASES=1 go test -run '^$' -bench 'NewLand/globe$' -benchtime 1x -count 3 -timeout 60m .
```

The times are inclusive: `drain` includes the `weather`, `pool` and `flow`
it calls, so the shares do not add up to 100%. A pass that runs on several
goroutines at once (`airEnv.vapour` under `rainOn`) sums its time on each of
them, so it can read as more than the wall clock of the pass that spread it.
The environment is read once, when the package is initialised, which is
before `go test` starts watching the environment for its cache: a test run
that switches `TERRA_PHASES` needs `-count=1`, or the cache answers for the
other setting.

## Working on the stages after the history

`Generate` runs in six stages (`stages.go`), and with `TERRA_PHASES=1` each
is a row, `stage.ground` to `stage.cover`. The ground stage is the history,
about four fifths of a globe. To change anything after it without paying
for it every run, keep the history once and make the world from it:

```bash
go run ./cmd/overview -preset globe -keep-history globe.history
```

```bash
TERRA_PHASES=1 go run ./cmd/overview -from-history globe.history
```

The world made from the file is the world made straight through, to the
bit (`TestAWorldResumedFromItsHistoryIsTheSameWorld`). A history file is
refused by a build whose grid is laid out differently, but not by one whose
history code has changed: after a change to anything the ground stage runs,
keep the history again.
