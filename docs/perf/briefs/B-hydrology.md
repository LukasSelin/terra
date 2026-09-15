# Session B: the hydrology, exactly as it was, cheaper

Eighty-five of the globe's hundred seconds are one goroutine, and most of
that goroutine is the water: `pool` sorting every tile, `flow` and the
floods popping a binary heap, `fluvial.solve` walking the stack, and every
one of them making its scratch afresh on each of thirty calls. This session
does the items of phase 1 that leave every world bit-for-bit as it was, and
proves it with the digest after each one. It does not reorder any sweep and
it does not touch `creep`'s arithmetic; those need the owner's decision.

## Start

- Branch from main: `claude/hydrology-exact`. Merge main first; if
  `TestWorldDigest` is not there, merge `claude/work-trees-performance-plan-4b623c`.
- `$env:GOMAXPROCS=8`. Every `go test` on the root package takes
  `-timeout 60m`.
- `TERRA_DIGEST=write go test -run TestWorldDigest .` on the base commit
  before touching anything. Every item below ends with
  `TERRA_DIGEST=check`, and a digest that differs means the item is not
  done.
- Read the 2026-09-15 entries in `docs/perf/worklog.md` first, in
  particular Finding 2, Finding 4 and "What needs attention". Read the
  Jacobi entry too: it is the record of why reordering is not on this list.

## Files you own

`lake.go` except the `drain` function (which session A owns); `fluvial.go`;
`slide.go`; in `erode.go`, `waterStep` and the scratch that `creep`
allocates, not `creep`'s arithmetic; the scratch fields you add to `Grid`
in `grid.go` and their lines in `Clone`; `docs/perf/worklog.md` (your
entry) and `docs/perf/budget.json`. Do not touch `weather.go`,
`orographic.go`, `vapour.go` or `wind.go`.

## Items, in order

Each is one commit. After each: digest check, goroutine test, budget
(rewrite with `TERRA_PERF_UPDATE=1` when it falls, and say by how much),
and a benchstat of globe256 with count 6 recorded as taken under load.

1. **A cheaper heap for the floods.** `floodQueue` in `lake.go` and
   `slideQueue` in `slide.go` are binary heaps popped millions of times
   (`slideQueue.pop` is 6 s of flat CPU over 6 calls). Make each a 4-ary
   heap. The pop order is a total order - `floodBefore` compares height then
   sequence; check `slideAt.less` is total too before you start - so the
   order of pops does not depend on the heap's shape and the world is
   unchanged. Confirm with the digest. Also stop each queue growing from
   nothing: keep its backing slice on the `Grid` between calls.
2. **Scratch on the grid.** Every tile-sized `make` in `pool` (`order`,
   `own`), `flow` (`stand`, `reached`, `from`, `exit`), `waterStep` and the
   `fillFrom`/`deepReceivers` it calls, and the scratch `creep` allocates
   becomes a slice kept on the `Grid`, resized only when its length is not
   the tile count, cleared where the pass relied on `make` zeroing it.
   Update `Clone` so a copy does not share scratch with its original (nil
   is fine; the pass remakes it). Finding 4 in the work log lists the sites
   and their bytes. Expect the budget to fall a lot and the clock a little.
3. **`pool` without a fresh sort.** The order is total (height, then
   index), so it is the same whoever produces it. Keep the previous call's
   `order` on the grid and, on the next call, refresh each entry's height
   and repair the order with an insertion pass (near linear when the ground
   has moved little, which between drains it has). Fall back to a full sort
   on the first call or when the insertion pass moved more than, say, a
   tenth of the entries. Digest must match. Measure how many entries move
   per drain on globe256 and put it in the work log.
4. **`fluvial.solve` by basin.** Read `solve` and `account`. Each tile's
   implicit step reads only its receiver's new value, so each outlet's tree
   is independent of every other. Split the stack by outlet, deal the trees
   to goroutines with `InParallel` in outlet order, and do every reduction
   that crosses trees (`exported`, anything summed over the map) in a fixed
   order on the main goroutine afterwards. If any part of `solve` reads a
   tile outside its own tree, stop and write down which; do not
   approximate. Digest must match with `Workers` at 1, 4 and 24.

If items 1-4 are done before morning, one more, only if the digest has
matched on every item so far: **`fillFrom` as a bucketed flood**, keyed on
height with the index as tie-break, only where the pops come out in the
same total order. Otherwise stop.

## Stop rules

- A digest that differs after two attempts at an item: revert the item,
  write what was tried and where the first differing tile was (compare
  heights on globe128 and print the index), and go to the next item.
- Any item that needs a file you do not own: write it down and skip.
- No changes to `creep`'s arithmetic, `currents`, `vapour`, or any
  Gauss-Seidel sweep's order.

## Acceptance

- Digest unchanged from the base on every committed item.
- `TestMakingAWorldDoesNotDependOnTheGoroutines` passes.
- Budget rewritten, with the fall stated in the work log.
- The full suite (`go test -timeout 60m .`) has the same failure list as
  main; with the digest unchanged it must, and if it does not, something
  reads scratch it should not.
- Work-log entry: one paragraph per item with the benchstat globe256 line,
  marked as taken under load, and the budget's bytes and allocs before and
  after.

## Finish

Rebase onto main. Do not merge. Report: the branch, the list of item
commits, the budget before and after, and the benchstat lines.
