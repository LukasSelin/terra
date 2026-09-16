# World creation performance work log

Newest entry first. Each entry says what was measured, on what, and what it
means; changes say what they bought in benchstat terms. How to take the
measurements is in [README.md](README.md).

---

## 2026-09-16 - cmd/overview: wetness, woods, growth and glacial, on the flags and the form

**What this is.** The four terms `cmd/overview` could not set, on
`claude/world-generator-web-ui-c36c93`: `-wetness` (rain against the real
world's; 0 keeps the preset's), `-woods` and `-growth` (`tuned` or
`climate`; empty is the map's own, climate where it wraps and the rules
where it does not) and `-glacial`. The form has them under "Climate and
cover": the empty rule shows which one the map would take, and the glacial
box is off unless the map is drawn (epochs 0), which is the only map it
cuts. They go into `settings.json`, the list of runs and "tune from this",
and the map page's summary line names them where they are set. Runs from
before read them as unset.

**What it measured.** Nothing about world creation. No file of the root
package changed. The command line's stdout and every png and `why.html` on
the default valley are what they were byte for byte. A 48x32 valley at
wetness 2 rains more than half again what it does at 1, which the test
holds; `go test -short ./cmd/overview` runs in under four seconds.

---


---

## 2026-09-16 - A world into a Zarr v3 store: zarr/, sharding, cmd/zarr

**What this is.** On `claude/docker-tree-resources-a9e433`: a way to keep a
world as a column store, one chunked array for each thing a tile has, that
Go and Python both read. Three pieces, one commit each:

- `zarr/` (4cd6007, db585d5) is `github.com/LukasSelin/zarr`, a Zarr v3
  module of its own, standard library only, which terra does not import
  and which is meant to leave for its own repository as it stands. Arrays,
  groups, the bytes, gzip and crc32c codecs, and `sharding_indexed` with
  partial reads through a `RangeGetter`. `TestZarrPython` (skipped unless
  `ZARR_PYTHON` names a Python with zarr and numpy) has zarr-python 3.4.0
  write twelve arrays for the module to read and read twelve the module
  wrote; a shard is laid out byte for byte as zarr-python lays it.
- `readout.go` (28b7cb2) in the root package: `AppendBeds`, `Record` and
  `FeatureOf`, read-outs of the beds, the book and the registry, which were
  kept in shapes of the map's own. They copy and change nothing.
- `cmd/zarr` (28b7cb2), a module of its own so that terra's `go.mod` stays
  the standard library's: makes a world and writes 67 arrays in seven
  groups, in the types the world keeps them in.

**Every world is as it was.** Nothing in the making of a world changed.
`TERRA_DIGEST=check` passes, and the short tier passes (69.9 s). The budget
was not rerun, as nothing it measures was touched; the read-outs allocate
only when called, and nothing in the making calls them.

**What an export costs.** One run, AMD Ryzen 9 3900X, 24 threads, with other
sessions on the machine - a reading, not a baseline:

| | |
|---|---|
| making the 1024 by 512 globe (`-preset globe`) | 49.9 s |
| writing it: 67 arrays, gzip 5, chunks of 64, shards of 16 chunks | 0.8 s |
| on disk | 143 files, 41.2 MiB |

The export is two percent of the making, so it is not worth a benchmark of
its own yet. Unsharded, the same store is 67 arrays of 128 chunks: some
8 600 files; the shards are what keep a large world to a file count a
directory or an object store is comfortable with. A shard is written whole,
so writing costs a shard's worth of memory per array at once (a 1024-square
shard of float64 is 8 MiB), and the arrays are written over GOMAXPROCS
goroutines. Reading a 3 by 3 region of a 256-square array in 128-square
shards fetches two ranges - the index and one 16-square chunk - and not the
shard (`TestReadingAChunkReadsOnlyItsPartOfTheShard`).

**Held.** The export reads back tile for tile as the world
(`TestAMadeWorldReadsBackAsItIs`); the same world writes the same store
byte for byte on one goroutine or eight
(`TestTheSameWorldWritesTheSameStore`); zarr-python reads the globe's
heights to the same digits the Go module does.

**Not yet.** The export is the world as made, at tick 0, not a day of its
weather. Features are not named, as terra names nothing without a namer
and Zarr v3 has no core string type. xarray has not been tried on a store.

---


## 2026-09-16 - cmd/overview -serve: worlds made in the background

**What this is.** The fourth step of the web page, on
`claude/world-generator-web-ui-c36c93`. A press of the button no longer
holds the request open while the world is made: it queues a job and sends
the browser to `/jobs/<id>`, which asks `/jobs/<id>/status` every second
and goes on to the world's page when it is drawn. One worker makes the
jobs in the order they came (up to 64 waiting), under the same lock as a
tile's world made again. The page shows the stage `generate` reports
(making the world, running the weather, drawing each layer), the time so
far, and, once a job of the same kind (history or not, globe or not) has
been made, a guess at the time left from its seconds a tile. A job still
waiting can be called off; one running is made to the end, since nothing
in making a world can stop part way. A failed job says why and links back
to the form filled in with its settings. The home page lists the jobs
being made, and leaves their half-drawn directories out of the runs.
Jobs live as long as the server.

**What it measured.** Nothing about world creation. No file of the root
package changed. On this machine, with a 128x64 globe made first to learn
the pace, a 512x256 globe guessed 20 s at 6 s in and was drawn at about
20 s. `go test -short -race ./cmd/overview` passes in 11 s: it holds the
worker off to check the order and the count ahead, calls a job off, and
fails one that asks for more memory than there is.

---

## 2026-09-16 - Generate in stages, and a history kept in a file

**What this is.** The first step of phase 3 of the scaling plan ("stages as
values"), on `claude/app-performance-structural-956966` from main at
ad943a2. Two commits' worth, neither moving any world: `TERRA_DIGEST=check`
passes after each, the budget passes unchanged, the pinned pass counts
hold, and the short tier passes (70 s).

**The stages.** `Generate` was one function of three hundred lines. It is
six methods on `Land` run from a table in `stages.go` -
`ground -> sea -> shape -> cut -> coast -> cover` - each over the grid the
one before it left; the hand-off is the `Grid` and the position of
`Land.RNG`, and no stage reads another's locals (the one that did,
`poured`, is `Terms.poured`). With `TERRA_PHASES=1` each is timed as
`stage.<name>`. The globe, quiet machine, one run:

| stage | wall s | share |
|---|---:|---:|
| `stage.ground` (the history) | 36.74 | 78% |
| `stage.cut` | 4.00 | 8% |
| `stage.coast` | 3.14 | 7% |
| `stage.shape` | 2.76 | 6% |
| `stage.cover` | 0.37 | 1% |
| `stage.sea` | 0.05 | 0% |
| `Generate` | 47.14 | 100% |

**The history file.** `historyfile.go`: `MakeLandKeepingHistory(seed,
terms, w)` makes a world and writes it, stopped between the ground stage
and the sea, to `w`; `LandFromHistory(r)` runs the other five stages on
what it reads. `cmd/overview -keep-history f` and `-from-history f` use
them. What is kept is every field of the `Grid` found by reflection, less
the ten `historyDropped` names with a reason each (the seven scratch
slices, the router, the landmarks, the features), so that a field added
later is kept without anybody remembering to, and a field of a kind the
file cannot hold fails the write rather than being left out. Fields are
written as their memory, flat runs of numbers as one run of bytes; the
header carries the architecture and a fingerprint of the layout of every
type held, and a reader refuses anything else. It is a cache of a history,
not an interchange format: nothing in it says whether the history code
that wrote it is today's.

What had to be kept that a hand-written list would have missed: the
weather's winds with the vapour budget each reading warm-starts from, and
`aired`, which the weather gate compares against - without them the first
drain after the history rebuilds the weather, which it does not do in a
world made straight through, and the world moves. And the chance: `Land`
keeps its `*rand.PCG` now, whose state is sixteen bytes of the header.

**What it measured.** The full globe, quiet machine:

| | wall |
|---|---:|
| made straight through, keeping the history | 47.1 s |
| made from the kept history | 10.5 s |

The file is 198 MB (396 bytes a tile); the stages' times against the whole
put the write and the read at about a tenth of a second each, on a warm
page cache. Every one of the 21 maps
`cmd/overview` draws is byte-identical between the two runs, as are the
summary and the why page. `TestAWorldResumedFromItsHistoryIsTheSameWorld`
holds valley, ancient and globe128 to the digest and every kept field bit
for bit (NaN included, which the deep floor marks tiles with), the
features and the chance; `TestAHistoryResumesTheSameOverAnyGoroutines`
resumes ancient over 1, 3 and 8; `TestAHistoryFileIsRefusedWhenItIsNotOne`
feeds it nothing, a PNG header, half a file, another version and another
layout. History sizes: valley 0.6 MiB, ancient 1.1 MiB, globe128 4.9 MiB.

**What it is for.** A change to anything after the history - the shaping,
the cutting, the coast, the woods, the soil - is run on a kept history in
a fifth of the time, and the history grid of phase 3 plugs in at the same
boundary: the ground stage's output is what a coarse history will have to
hand the map.

**What is next.** The yardsticks read many small globes each made from
scratch; making their histories once per run and resuming is the test
suite's share of this. A second boundary kept (after `cut`) would do the
same for the coast and the cover.

**`scripts/perf.sh check`**, quiet machine, against the 07:18 baseline:
passes, valley -5.9%, ancient -4.3%, globe256 -5.4% (p=0.002, intervals
±1-3%), with B/op -18..-31%. The branch adds six timer calls to a world and
nothing else to `NewLand`'s path, so the gain is what main has merged since
07:18 (the hydrology's scratch on the grid, among it), not this change; the
baseline stands until a change of its own moves it.

---

## 2026-09-16 - cmd/overview -serve: click a tile to ask why it is so

**What this is.** The third step of the web page, on
`claude/world-generator-web-ui-c36c93`. A click on a served run's map (a
press that moves less than four pixels; more is a pan) marks the tile and
asks `GET /runs/<run>/tile?x=&y=`, which answers with the world's account
of that tile as JSON: its terrain, height and features, and `terra.Why`'s
chain for height, rock, rain and cover, rendered by the same sentences
`why.html` uses. The account shows under the map. `why.html`'s eight tiles
are now built by the same `describe`.

**How the world is found.** `generate` became `makeWorld`, which makes the
world and runs its weather to the asked day, and `draw`. The server keeps
the worlds it made last, up to 2^20 tiles together (a few valleys or one
globe; the newest always), and makes a world it has let go again from the
run's `settings.json`, under the same lock as making one. The test holds
that the answer from a world made again is the answer from the one kept,
byte for byte. Runs from before `settings.json` say they cannot answer.
A page opened from disk, without the server, says it needs `-serve`.

**What it measured.** Nothing about world creation. No file of the root
package changed, so the digest, the budget and the yardsticks are what
main's are. The command line's stdout and every png and `why.html` on the
default valley are what they were byte for byte; `index.html` gains the
click script. `go test -short ./cmd/overview` runs in under two seconds.

---

## 2026-09-16 - cmd/overview -serve: the options on a form

**What this is.** The second step of the web page, on
`claude/world-generator-web-ui-c36c93`. The home page is now a form of
`cmd/overview`'s options - preset, seed (with a random one a click away),
width, height, epochs, sea share, water, day, scale and wrap - filled in
from the flags the server was started with. An empty field is the
preset's own value, shown greyed, as a flag left off is. What the terms
would refuse (a globe not a whole number of chunks round, a sea share
past 1, a picture over 16384 pixels a side, a world that will not fit in
memory) comes back as 422 with the reason above the form as it was
filled in, and nothing is made. Each run keeps `settings.json`; the list
of runs says what each was made from and links "tune from this", which
fills the form in with it. `-max` is not on the form. No file of the
root package changed, so the digest, the budget and the yardsticks are
what main's are and were not re-run.

**What it measured.** Nothing about world creation. `go test -short
./cmd/overview` makes a 32x24 world through the form, turns away six
forms that cannot be made, and reads options back from the fields they
wrote, in under two seconds.

---

## 2026-09-16 - cmd/overview -serve: a page with a button that makes a world

**What this is.** The first step toward making worlds from a browser, on
`claude/world-generator-web-ui-c36c93`. `go run ./cmd/overview -serve :8080`
serves a page with one button; each press makes the world the other flags
describe into its own directory under `-runs` (default `runs/`) and sends
the browser to its `index.html`. The body of `main` became
`generate(options, out)`, which the command line and the server share. A
mutex keeps the server to one world at a time, because `terra.SetNamer` is
the package's. No file of the root package changed, so the digest, the
budget and the yardsticks are what main's are and were not re-run.

**What it measured.** Nothing about world creation. The command line's
output on the default valley is what it was before the split: stdout and
every png the same byte for byte, and `index.html` differs only in its
"made in" time. `go test -short ./cmd/overview` makes a 32x32 valley
through the server in under half a second.

---

## 2026-09-16 - U1: cmd/unreal, the Landscape export at 25 m

**What this is.** Milestone U1 of the scaling plan's level 2, on
`claude/unreal-export`: a command with `cmd/overview`'s flags that writes
the world for Unreal's Landscape import. New files only under
`cmd/unreal`; no file of the root package changed, so the digest, the
budget and the yardsticks are what main's are and were not re-run. The
tests (`go test ./cmd/unreal`) make the valley on seed 1 once and export
it once, cut into 17-vertex tiles so that tile edges can be checked, and
run in under two seconds.

**What it writes.** A 16-bit heightmap per Landscape tile (1009, 2017,
4033 or 8129 vertices, the largest not wider than the world, edges
shared, padded by edge extension, the seam column of a globe written
twice), ten 8-bit weightmaps per tile summing to 255 a vertex, the sea,
lakes and river reaches as JSON with Finnegan's width and Manning's depth
at the mean flow, one tree per forest tile as CSV with a species by
overview's Köppen reading, and a manifest with the scales and the exact
formula back to metres. The valley export is 14 files and 0.2 MB; globe256
is 14 files and 0.3 MB; both are one tile of 1009.

**What it measured.** Nothing about world creation. The export itself is
about 0.1 s on the valley and on globe256, under load, and is not the
point. The README's "What the metre level needs" lists what the root
package would have to expose for U2: the channel constants, a lake tile
index, the ebb per tile, the abyssal flag, the strata's bed tops, the
meander phase, a shared Köppen reading, a gradient vector, and
`DetailChunk` itself.


## 2026-09-16 - P1: the causal record. The book kept, features, and Why

**What this is.** Track P of the plan (`docs/perf/scaling-plan.md`), the
first milestone, on `claude/causal-record`: a history keeps a book of what
it did to each tile instead of dropping it at `settleRock`; a registry
joins the tiles into features; `Why(p, aspect)` reads the book and the
registry back as a chain of causes; `cmd/overview` renders eight of them as
sentences on a `why.html`. The principle held throughout: an explanation is
a reading of recorded quantities, never a heuristic invented afterwards.
The items land one commit each and this entry is extended as they do.
Every world is as it was: `TERRA_DIGEST=check` passes after every commit.

**The book (item 1).** `ledger.go`: twelve bytes a tile - the meeting that
did most to its height (the two plates, its kind, its epoch, the metres, as
`float32`) and the wear since (tens of metres, `uint16`), and the epoch and
kind of its last burial - written by `tectonics`, `hotspot`, `keepBook` and
`wear` where they compute the lift, the burial and the wear. The seam
carries the plate on its far side now (`seam.with`), which the book needs
and nothing else reads. The metres are the history's own and are not
rescaled with the heights: a seam raises tens of kilometres in an epoch of
four million years and the weather and the settling take most of it back,
while the ground stands a few kilometres high and is handed to the map by
rank; carrying an increment larger than any height through the rescaling
of standing heights made a 45 km lift into 4.5 km on a 258 m map, so the
book says whose metres it gives instead. The final weld map of the plates
(`plateRoot`, a byte a plate) is kept so a plate number the book wrote
down in an early epoch can be followed to the plate that stands.

Budget, `TERRA_PERF_UPDATE=1` before and after item 1 (bytes are checked to
1%, allocations to 3%):

| world | bytes before | bytes after | per history tile | allocs before | allocs after |
|---|---|---|---|---|---|
| valley (no history) | 10 348 688 | 10 354 136 | - (run noise) | 1 304 | 1 312 |
| ancient | 58 195 848 | 58 232 048 | +12.6 | 10 230 | 10 226 |
| globe128 | 437 453 760 | 437 547 440 | +11.4 | 33 617 | 33 606 |

Twelve bytes a tile is the struct's size, and the rest of the difference is
the run-to-run noise the budget's slack covers. Nothing per epoch beyond the
writes; the `record` is not grown, since anything added to it costs its
bytes too.

**The features (item 2).** `features.go`: a registry built on one goroutine
in tile order at the end of `Generate` and of `Erode` - uplift belts (the
tiles whose strongest meeting was one pair of plates in one epoch, joined
where they touch), drainage basins (the route trees by where the water
ends, each with its trunk from the outlet up by the most water at every
fork), lakes, plates and climate regions (connected dry land of one Köppen
group). The Köppen reading moved from `cmd/overview` into the package as
`Grid.Koppen` and `KoppenOf`, computing what it computed there, with a
letters-only form so the registry allocates nothing a tile. Ids are the
order of the lowest tile, kind by kind. `SetNamer` hands the naming out;
`cmd/overview` names from the seed.

Two things the counts say. Belts fragment along a range by epoch, since a
tile's strongest epoch varies along a seam: on ancient 241 belts over 1 930
tiles, 91 of them one tile; on globe256 1 827 over 19 459. Basins are every
route tree, and most of a coast's are a tile or two: globe256 has 1 465, of
which 200 hold ten tiles or more. Both are the record as it is; a floor on
size is a reader's choice and is left to the reader.

Budget after item 2. The registry's churn is the labels (three ids a tile),
the ends and the tributary table the basins are read with, and the feature
list growing by doubling; the first alloc-per-feature version was found by
`testing.AllocsPerRun` and put right (the feature is appended and filled in
place, not built through a pointer that escapes):

| world | bytes before | bytes after | per tile | allocs before | allocs after |
|---|---|---|---|---|---|
| valley | 10 354 136 | 10 473 792 | +41.5 | 1 312 | 1 333 |
| ancient | 58 232 048 | 58 455 880 | +77.7 | 10 226 | 10 265 |
| globe128 | 437 547 440 | 438 381 216 | +101.8 | 33 606 | 33 662 |

Feature counts, seed 1, with the belt keyed by the kind of meeting as well
as the pair and the epoch (one pair can be closing at one end of its seam
and parting at the other, and an arc and a rift are not one meeting; found
by TestABeltIsRaisedByOneMeeting): ancient 316 (278 belts, 26 basins, 2
lakes, 9 plates, 1 climate region); globe128 1 204; globe256 3 669 (2 075
belts, 1 465 basins, 3 lakes, 16 plates, 110 climate regions).

**Why (item 3).** `why.go`: `Why(p, aspect)` for `OfHeight`, `OfRock`,
`OfRain` and `OfCover` (`Rock` was taken by the terrain), a `[]Cause` of
`{Feature, Kind, Quantity, Unit, When, Note}`, `When` in years before the
present from the epoch and `epochYears`. Height: the meeting, its belt, its
two plates, the wear since, and where the tile stands on the map. Rock: the
bed at the surface with its thickness and epoch, what that rock is in this
world, and the book's last burial. Rain: the tile's year against its row's
mean, the orographic term of the phase carrying most of the cell's water as
the budget kept it, and the sea's distance upwind in that phase's wind over
the air's cells. Cover: `WoodsAt` and the readings it was made from, with
their numbers. No rain shadow is claimed; that is P2's rule. A chain is
microseconds and allocates only itself.

**The page (item 4).** `cmd/overview/why.go` writes `why.html` beside the
maps: eight tiles chosen from the world - the highest, the driest land, the
largest lake's shore, the largest river's last tile of land, four across
the middle - and one sentence a cause. Found on the way: the weather
drawing indexed one column past the east edge of any unwrapped map and
panicked on the valley presets; fixed in the same commit.

**Tests (item 5).** `why_test.go`: `TestWhyIsDeterministic` (one goroutine
twice and four, every chain and feature equal), `TestABeltIsRaisedByOneMeeting`,
`TestTheBookCostsWhatItSays` (twelve bytes a line, one a tile, none on a
drawn world) and a golden chain for tile 1000 of the ancient valley. All
in the short tier; the four take under two seconds.

What the pages say, seed 1. Ancient, the shore of the largest lake: *The
Unelbist Rift dropped this ground by 6186 m of the history's own, 48
million years ago, in a rift. The history's weather has taken 4580 m of
that off since. It stands at 38.9 m on the map, riding the Dornven Plate.*
Globe256, the driest land: *14.3 mm of rain falls here in a year, in the
Kenvak Drylands. The mean over its row of the map is 792 mm. In the spring
quarter, which carries most of the air's water here, the ground's lift
wrung nothing out of the air over this cell. The sea lies 303 km upwind in
that quarter's wind.*

Budget after items 3 to 5: unchanged; nothing in them runs while a world is
made. `TERRA_DIGEST=check` passes on every commit of the branch; the short
tier passes; `cmd/overview`'s tests pass.

---


## 2026-09-16 - S2 to the end: the view, and Flow, Drain, Soil, Sand and Clay beside the map

**What this is.** The owner chose the read-only view over moving the game,
so the rest of S2 followed on `claude/simd-kernels`, one field per commit
in the plan's order, each with the digest on both builds, the budget
rewritten, `TestMakingAWorldDoesNotDependOnTheGoroutines`, the full suite,
and globe256 interleaved n=6 against the commit before it with
`TERRA_PHASES=1`. Every world is as it was: `TERRA_DIGEST=check` passes
after every commit.

**The view.** `g.Tile(i)` and `g.TileAt(p)` are a `TileView`: the `*Tile`
embedded, so its fields and methods come through as they are, and a
method for each field the map keeps beside it - `Height()`, `Flow()`,
`Drain()`, `Soil()`, `Sand()`, `Clay()` - plus `Silt()`, `Loam()` and
`Wash()`, which were methods on the tile reading its sand and clay. It is
read-only and a reading, not a copy; what is beside the map is written on
the map. A game reads `g.Tile(i).Height()` whatever holds the height.

**The texture helpers.** `Silt`, `Loam` and `Wash` read two fields that
moved, so they are `siltAt`, `loamAt` and `washAt` on the Grid by index,
the view's `Silt`, `Loam` and `Wash` for a reader by tile, and `siltOf`,
`loamOf` and `washOf` as the pure statements the tests hold (`silt` was
taken: it is the pass). `parts`, `hold`, `blend` and `mix` take an index
too, and the creep's depth reads by index.

**The crust.** When the plates move, the tile went with the ground and
took its soil, sand and clay with it. Height had a copy on the crust
already; Soil moved the digest on every world with a history until the
crust kept a copy of it too, and Sand and Clay went in the same way. Flow
and Drain did not need it: the drain reads them afresh before anything
does.

**globe256, each field against the commit before it** (interleaved,
n=6, `TERRA_PHASES=1`; only passes that moved with p < 0.05):

| field | world | passes that moved |
|---|---|---|
| Flow | 4.020 s -> 4.050 s, ~ (p=0.065) | `waterStep` +6.6%, `wear` +3.9% |
| Drain | 4.077 s -> 4.040 s, ~ (p=0.180) | `move` -11.8%, `airEnv.currents` -1.7%, `reshape` +7.0% |
| Soil | 4.012 s -> 4.051 s, ~ (p=0.065) | `keepBook` +12.1%, `flow` +4.2%, `waterStep` +2.2%, `wear` +1.9% |
| Sand | 4.043 s -> 4.031 s, ~ (p=0.240) | `waterStep` -7.5%, `fluvial.solve` -13.1%, `keepBook` -5.8%, `wear` -3.4%, `flow` -3.1%, `move` +2.3% |
| Clay | 4.023 s -> 4.036 s, ~ (p=0.589) | `joinUp` +11.1%, `basins` +6.1%, `move` -1.5%, `orographic` +1.2%, `waterStep` +1.1% |

No field moved the world as a whole. The passes go both ways: a pass that
reads one moved field inside a loop that still walks the tile for another
touches two lines where it touched one (`waterStep` after Flow), and one
that reads the moved fields in a run of their own gets them in order
(`waterStep` after Sand, `move` after Drain, whose tile shrank by a third
before it was copied round the crust). Bytes per world did not move
beyond a tenth of a percent at any step.

**The budget** (workers 4), from Height's rewrite to Clay's:

| world | bytes after Height | bytes after Clay | | allocs after Height | allocs after Clay |
|---|---|---|---|---|---|
| valley | 10 328 112 | 10 348 688 | +0.20% | 1299 | 1304 |
| ancient | 58 196 144 | 58 195 848 | -0.00% | 10214 | 10230 |
| globe128 | 437 518 152 | 437 453 760 | -0.01% | 33605 | 33617 |

The tile is 32 bytes, from 72 when the day started: the six fields took
44 and left 28, which pads to 32, so a tile costs four bytes more than
its fields and the drawn valley, which has no crust copy to lose, is the
one that shows it. The worlds with a history give the crust's copy of the
tiles back what the slices cost.

**The suite** (`go test -timeout 60m .`), the commit after each field,
in a second checkout: Flow ok (386 s), Drain ok (386 s), Soil ok (386 s),
Sand ok (384 s), Clay ok (381 s). Main at 3124515 is ok with no failures.

**`scripts/perf.sh check`** at the branch's end, quiet machine, against
the 07:18 baseline:



**What lreat has to do.** Its reads of `Tiles[i].Height`, `At(p).Height`
and `t.Height` (and `.Flow`, `.Drain`, `.Sand`, `.Clay`) become
`g.Tile(i).Height()`, `g.TileAt(p).Height()` and so on; its test writes
(`Tiles[i].Height = 0`) become `g.Height[i] = 0`. About twenty-five sites.

---

## 2026-09-16 - The kernel layer, and Height off the Tile (track S)

**What this is.** The SIMD track of the scaling plan, on
`claude/simd-kernels` from main at 3124515, in two halves. S1 is a kernel
layer: `kernel.go` is the statement of each kernel one number at a time,
`kernel_simd_amd64.go` the same four lanes at a time under
`GOEXPERIMENT=simd` on a processor with AVX2, `kernel_noasm.go` every other
build, and `kernel_test.go` one fuzz test per kernel holding every lane to
the statement over runs of every length to 4096 with negative noughts,
NaNs, infinities and numbers of very different size in them. The day's
`fade` and `grow` and the transform's butterflies moved into it from
`pass_simd_amd64.go` and `fft_simd_amd64.go`, which are gone, and six
kernels are new: `axpy`, `lerp`, `clamp`, `sumTree`, `stencil5`,
`minmaxSelect`. S2 moved `Height` off the `Tile` and onto the `Grid` as a
slice beside the map. Everything here leaves every world as it was:
`TERRA_DIGEST=check` passes on both builds after every commit.

**What the fuzzing found in the kernels that were there.** Two things,
neither of which a world made today can hit.

- The vector `grow` was wrong when no kind ages. Compared against no kind
  at all it compared the lanes against nought, which is a kind - open
  grass with nothing on it - and aged every such tile.
  `TestGrowIsRipenAndReplenishTileByTile` failed under `GOEXPERIMENT=simd`
  on main for it; nothing that makes a world calls `Grow`, so the digest
  never saw it. It takes the scalar path now.
- The vector butterflies keep a different NaN from the scalar ones where
  both sides of the complex product's sum are NaN. The processor keeps the
  first operand's bits, and the compiler is free to put either side of a
  sum first, so the bits are not a fact about the statement: a lane is
  held to be a NaN where the statement has one, and to every bit
  everywhere else. Laying the product out in the compiler's order (one
  permute more) was tried and does not hold either, for the same reason.
  No field a map transforms holds a NaN.

**The kernels, scalar against vector** (`PERF_BENCH='Kernel|FFT'
scripts/perf.sh simd`, n=6 each, interleaved; the full table is
[baseline/2026-09-16-simd-kernels.txt](baseline/2026-09-16-simd-kernels.txt)):

| kernel, 1024 entries | scalar | simd | |
|---|---|---|---|
| FFT/64 | 1136 ns | 839 ns | -26% |
| FFT/256 | 5.59 µs | 3.53 µs | -37% |
| FFT/1024 | 26.2 µs | 15.5 µs | -41% |
| fade | 331 ns | 139 ns | -58% |
| grow | 771 ns | 490 ns | -36% |
| axpy | 519 ns | 200 ns | -61% |
| lerp | 522 ns | 272 ns | -48% |
| clamp | 983 ns | 199 ns | -80% |
| sumTree | 691 ns | 187 ns | -73% |
| stencil5 | 978 ns | 502 ns | -49% |
| minmaxSelect | 1208 ns | 259 ns | -79% |

All p=0.002. The transform reads a little less than on 2026-09-15 (-41% at
1024 against -49%) because the scalar side was measured on a quieter
machine this time; the lanes are the same code.

**What calls what.** `fade` and `grow` are the day's pass as before, the
butterflies are the transform's, and `axpy` sums the upland mask's
octaves a row at a time in `relief.go`, which is the one place in the
package that had that exact loop. The other five have no caller yet.
`sumTree` adds in the vector's order, which is not a loop's order, so it
is for sums that are new or mean to move the world. `clamp` keeps a
negative nought where `clamp01` turns it positive, and every `clamp01` is
one number inside a larger expression. `stencil5` reads old values into
new, and the package's stencils today are sweeps: `creep` is implicit and
`relaxPotential` over-relaxes in place, which the plan leaves to the
owner. `minmaxSelect` fixes the order among equals, and the scans in the
package walk every third tile of one plate or filter as they go. No
`float32` kernel was written: no caller has a loop of that shape
(`ocean.go` clamps a float64 difference into a float32 store).

**`scripts/perf.sh simd`** builds the test binary with and without
`GOEXPERIMENT=simd`, runs the benchmarks on each turn and turn about
`PERF_COUNT` times, and prints benchstat with the scalar build as the old
column; it never fails. Documented in [README.md](README.md). The worlds
on this machine, n=6, at 12697a6:



**Height off the Tile.** `g.Height[i]` is what was `g.Tiles[i].Height`:
allocated in `NewGrid`, copied in `Clone`, 344 sites in 55 files updated
including `cmd/overview`. The crust keeps a copy of the heights beside
its copy of the tiles while the plates move; `bankAt` gives the meander
the bank's index; `Grid.Height(p)` the method is `Grid.HeightAt(p)` so
the field can carry the name. `TestMakingAWorldDoesNotDependOnTheGoroutines`
passes. Two binaries, before (ddb8bc8) and after (12697a6), globe256
interleaved n=6, `TERRA_PHASES=1` for the passes:

| | before | after | |
|---|---|---|---|
| globe256 | 4.043 s ± 1% | 4.047 s ± 1% | ~ (p=0.937) |
| `pool` | 185.3 ms ± 2% | 176.2 ms ± 3% | -4.9% (p=0.002) |
| `flow` | 215.8 ms ± 2% | 208.6 ms ± 3% | -3.4% (p=0.011) |
| `joinUp` | 48.5 ms ± 6% | 41.0 ms ± 17% | -15% (p=0.015) |
| `silt` | 172.7 ms ± 3% | 169.5 ms ± 4% | -1.9% (p=0.041) |
| `move` | 235.4 ms ± 2% | 241.1 ms ± 1% | +2.4% (p=0.002) |
| `waterStep` | 278.5 ms ± 2% | 283.8 ms ± 2% | +1.9% (p=0.002) |
| `windsFor` | 577.8 ms ± 1% | 589.5 ms ± 2% | +2.0% (p=0.015) |
| B/op | 1.517 GiB | 1.517 GiB | ~ |

The pool's sort reads eight-byte strides now rather than sixty-four, and
is measurably faster, as the plan expected; the passes that got slower
read the height inside loops that still walk the tile for its other
fields, so they touch two lines where they touched one. The world as a
whole is unchanged in time and in bytes. Budget rewritten (workers 4):

| world | bytes before | bytes after | allocs before | allocs after |
|---|---|---|---|---|
| valley | 10 333 272 | 10 328 112 | 1302 | 1299 |
| ancient | 58 201 832 | 58 196 144 | 10222 | 10214 |
| globe128 | 437 509 648 | 437 518 152 | 33585 | 33605 |

A tile lost eight bytes and the map gained a slice of them; the bytes
move by a few thousandths of a percent.

**Where it stops, and the lreat question.** `Height` is a public field,
and the lreat game reads it through its replace directive - `Tiles[i].Height`
and `At(p).Height` in its tests, `t.Height` in `ui/ascii` - so lreat does
not build against this branch until it moves with it. The plan's order
was Height, Flow, Drain, Soil, Sand, Clay, and lreat's game code also reads
`Flow`, `Drain`, `Sand` and `Clay` off the tile, so each further field
deepens the break. The brief says to stop after Height if the owner has
not decided, and this stops there. Two ways on:

1. A read-only view: a `Grid.Tile(i)` that gathers a tile's slices and
   fields into a value with a `Height()` on it, so a game reads
   `g.Tile(i).Height()` whatever moves. Writes in lreat's tests
   (`Tiles[i].Height = 0`) would still have to become `g.Height[i] = 0`.
2. Move the game: lreat's sites are few (about ten for Height, another
   fifteen for the other four), and a sed like the one used here does
   most of it.

The first keeps terra's public surface stable across the rest of S2; the
second is less code and is what this branch did to `cmd/overview`.

**`scripts/perf.sh check`** on the branch at 12697a6, quiet machine,
against the 07:18 baseline: ok.

| world | baseline | branch | |
|---|---|---|---|
| valley | 82.50 ms ± 3% | 79.34 ms ± 3% | -3.8% (p=0.004) |
| ancient | 344.9 ms ± 1% | 339.9 ms ± 2% | -1.5% (p=0.015) |
| globe256 | 4.117 s ± 2% | 4.019 s ± 0% | -2.4% (p=0.002) |

The bytes read -18 to -31% against that baseline; that is the hydrology
merge (bf5997c) between the baseline and this branch's base, not this
branch, whose bytes are the budget's above.

**The suite** (`go test -timeout 60m .`), branch at 12697a6 against main
at 3124515 in a second checkout, run at the same time:



---
## 2026-09-16 - The weather gate through the yardsticks (session A)

**What this is.** The gate from `claude/world-creation-profiling-e734ef`
(97e8e42, "Read the weather again only when the ground has moved from under
it") brought onto `claude/weather-gate` as handed - the three code files
only: `grid.go` (`aired`), `lake.go` (`drain` asks `weatherStale`) and
`weather.go` (`weatherFlips` 0.005, `weatherDrift` 0.01, `airedGround`,
`weatherStale`) - and taken through the counts, the yardsticks, the budget
and the digest, as [briefs/A-weather-gate.md](briefs/A-weather-gate.md)
asks. That commit's README, baseline and `perf.sh` changes belong to other
sessions' files and were left behind. Base commit ec23816 (main 1ad4985
plus the plan branch); gate b464eaa. The thresholds are the values the patch
came with: nothing was tuned.

**Skip counts** (a temporary counter in `drain`, removed; seed 1,
`Workers` 4):

| world | drains | weather rebuilt | skipped |
|---|---|---|---|
| valley | 6 | 1 | 5 |
| ancient | 24 | 19 | 5 |
| globe256 | 30 | 20 | 10 |
| globe | 30 | 19 | 11 |

The plan expected about 14 of 31 on the globe to skip; it is 11 of 30. The
rebuilds are the history's per-epoch drains, which really do move the
ground, plus the first after the pour.

**`Clone`.** `aired` is written by `weather` and read only by `weatherStale`,
behind a length check; `Clone` does not copy it, so a copy's first drain
reads the weather afresh (checked: a cloned valley reports stale, the
made map does not). Draining a clone then panics in `rainOn` at
`g.dayRange[i]`, because `Clone` does not copy `dayRange` either - on the
base commit too, so it is not the gate's and `dayRange` is not this
session's field. Nothing in the package drains a clone.

**Yardsticks.** Full suite on base (1701 s) and on the branch (1507 s),
both under load with two other sessions running:

| test | base | branch |
|---|---|---|
| `TestRealNumbers/Hack_exponent,_globe` | fails, 0.6005 against 0.54-0.6 | fails, 0.6005 - the known failure on main |
| `TestTheRealWorld/meander_wavelength,_small_globe` | passes on its known-gap marker (B, 14.6 widths) | 13.6 widths, **inside** 10-14: "the gap has closed, take the marker off" |
| `TestTheRealWorld/valley_floor_over_hillslope_soil_depth,_small_globe` | passes on its known-gap marker (I, 2.96x) | 3.015x, **inside** 3-50: "the gap has closed, take the marker off" |

Both differences are readings that moved inside their real range, not out
of it, and both were already sitting at the edge. The meander reading rests
on about 21 reaches of 16 steps over 8 small globes (see the yardstick
sample-size notes), so a one-width move is within its noise. The soil ratio
is a mean over the floor and hillslope tiles of the same 8 globes and stood
at 2.96 against a floor of 3; the gate's world lays the flats' mud under
rain read a few drains earlier, and the ratio crept over. Halving
`weatherDrift` or `weatherFlips` to push either back outside its range
would be tuning a threshold against one reading to keep a known gap open,
so the thresholds stay and the markers are left for the owner's decision
(`realism_test.go` and `realism_soil_test.go` are not this session's).
Everything else passes, `TestMakingAWorldDoesNotDependOnTheGoroutines`
among it.

**Budget and digest**, rewritten and committed (0911d8f):

| world | bytes before | bytes after | allocs before | allocs after |
|---|---|---|---|---|
| valley | 25.5 MB | 14.9 MB | 3124 | 1518 |
| ancient | 95.0 MB | 84.4 MB | 13643 | 12048 |
| globe128 | 652 MB | 509 MB | 45640 | 34611 |

Digest: valley fcd0ba48 -> 83bd9375, ancient aa5e4657 -> de10fa92,
globe128 71ad0b0e -> 6a85c8de. The world moves on purpose: a drain that
skips the weather keeps the rain of the last reading.

**Benchstat, taken under load** - two other sessions' suites and this one's
were running throughout, `GOMAXPROCS=8`, so none of these gates a merge.
Base in a second checkout of ec23816, count 6:

| world | base | gate | | B/op | allocs/op |
|---|---|---|---|---|---|
| valley | 200.9 ms ± 38% | 138.8 ms ± 14% | -30.9% (p=0.002) | -41.8% | -51.8% |
| ancient | 725.0 ms ± 13% | 589.4 ms ± 15% | -18.7% (p=0.009) | -11.2% | -11.8% |
| globe256 | 10.13 s ± 5% | 8.10 s ± 5% | -20.1% (p=0.002) | -24.2% | -28.7% |

Globe, `-benchtime 1x -count 3`: 124.0 / 108.4 / 109.3 s base against
105.7 / 94.9 / 102.6 s gate (benchstat ~, p=0.100 at n=3); 18.67 GiB to
15.40 GiB and 933 k to 685 k allocs. The bytes and allocs are exact; the
seconds are the quiet-machine job of `scripts/perf.sh check` in the morning.

**After the rebase onto main at 59cdb90.** Main moved under the branch
overnight: the deep sea floor laid from the crust's age and the land shaped
to the history's uplift (124d5a1), each rock's chemistry (c4b7b3a), the
meander marker taken off (6d3a069) and the concavity marker put back
(59cdb90). Since 124d5a1 the rain reads its height off `laidHeight`, so
`airedGround` and `weatherStale` now read the same (14cc3f6): the deep
floor is always under the sea and kept as -1 either way, so no world moves
by it. Digest and budget were rewritten again (d9cb769): ancient and
globe128 move with the deep floor, the drawn valley does not. Skip counts
on the rebased branch: valley 1 of 6 rebuilt, ancient 19 of 24, globe256
20 of 30, globe 20 of 30 (10 skipped, one fewer than on ec23816).

Full suite on main itself (59cdb90, in a second checkout, 1357 s) and on
the rebased branch (1181 s), both under load:

| test | main 59cdb90 | branch |
|---|---|---|
| `TestTheTideLaysFlatsOnlyWhereItReaches` | fails: small globe 4 has no flats | fails the same |
| `TestRealNumbers/drainage_area_exceedance_exponent,_small_globe` | fails, 0.4927 against 0.39-0.46 | fails, 0.4894 |
| `TestRealNumbers/discharge_exceedance_exponent,_small_globe` | fails, 0.5164 against 0.40-0.46 | fails, 0.4912 |
| `TestRealNumbers/Hack_exponent,_globe` | passes now | passes |
| `TestTheRealWorld/valley_floor_over_hillslope_soil_depth,_small_globe` | passes on its marker (I, 2.96x) | 3.08x, inside 3-50: "the gap has closed" |

The one difference is the soil-floor marker, as before the rebase, one
notch further inside its range. The tide flats and the two exceedance
exponents are main's own failures; the exceedance readings, one basin per
globe over 16 globes (standard error about 0.034), moved toward their range
under the gate and are not the branch's to judge either way. The meander
marker is no longer a difference because main took it off.

**Rebased once more, onto main at b9fbba3.** Main merged the profiling
branch itself meanwhile (41bd904: the gate, and both markers taken off) and
read the tide's flats off the third small globe (b9fbba3). The rebase
dropped this branch's copy of the gate as already upstream, so what the
branch adds to main is the plan branch (briefs, scaling plan, the digest
test), the two-line laidHeight read in the gate, and the budget, digest and
this log. The digest is as committed with and without that read. Full suite
on main b9fbba3 (1054 s) and on the branch (1055 s), both under load: each
fails exactly `drainage_area_exceedance_exponent,_small_globe` (0.4894)
and `discharge_exceedance_exponent,_small_globe` (0.4912), main's own. The
branch's list is main's list.

**Rebased a third time, onto main at 3cf543d.** Main took the plan branch
and the clock by pass (3cf543d) and the sea's warmth read against its row
(b8d3856), which moved globe128 again; the rebase dropped the plan branch's
commit as already upstream. Digest rewritten (0bddde9) and equal to main's
own; the budget rewritten on the same commit, differing from main's by a few
hundred bytes of allocation noise, and passing here where main's memory
says it fails on globe128 since b8d3856. The laidHeight read is checked
digest-identical on this main too. The clock by pass now counts the gate:
`weather` calls against `drain` calls, one seed, `go run ./cmd/overview`
with `TERRA_PHASES=1` (one weather call on each world is closedBasins'
own, not a drain's):

| world | drains | weather calls | rebuilt by drain | skipped |
|---|---|---|---|---|
| valley | 6 | 2 | 1 | 5 |
| ancient | 24 | 20 | 19 | 5 |
| globe256 | 30 | 21 | 20 | 10 |
| globe | 30 | 21 | 20 | 10 |

The full suite was not run again on this base: the branch's only code
against main is the two-line read proven a no-op, so its list is main's
at 3cf543d by construction, and that is the morning's `perf.sh check`.

**The morning, on a quiet machine** (2026-09-16 07:18, nothing else
running, all 24 threads). `scripts/perf.sh check` on the branch at main
639d7d5 passed: valley -17.6%, ancient -20.4%, globe256 -23.6% against
[baseline/2026-09-16-0039-small.txt](baseline/2026-09-16-0039-small.txt),
bytes and allocs within 0.5%. Both sides carry the gate, so that gap is the
night's load in the old baseline, not the branch, and the check said to take
a new one: [baseline/2026-09-16-0718-small.txt](baseline/2026-09-16-0718-small.txt)
(CIs ±1-3%) and, for the globe,
[baseline/2026-09-16-0718-globe.txt](baseline/2026-09-16-0718-globe.txt).
Against the last quiet pre-gate baseline, 2026-09-15-2230, which also
predates the sweep precompute and the SIMD butterflies:

| world | 2230 (pre-gate) | 0718 (quiet) | |
|---|---|---|---|
| valley | 135.0 ms ± 6% | 82.5 ms ± 3% | -38.9% (p=0.002) |
| ancient | 419.6 ms ± 4% | 344.9 ms ± 1% | -17.8% (p=0.002) |
| globe256 | 7.34 s ± 5% | 4.12 s ± 2% | -43.9% (p=0.002) |
| globe (count 3) | 81.4 / 81.5 / 91.7 s, 19.9 GiB | 54.7 / 55.0 / 56.1 s, 16.9 GiB | about -33% |

These are the numbers a check of B and C should be read against.

Rebased last onto main at 43d76a8, with B's hydrology and C's guards in it:
budget and digest are main's own, `TestPassCountsArePinned` passes with the
clock on, the yardsticks pass outright on main's tip and on the branch, and
`scripts/perf.sh check` against the 07:18 baseline reads valley -3.8%,
ancient -1.7%, globe256 -1.3%, within the limit, so that baseline stands.

---

## 2026-09-16 - The guards: a scaling benchmark, the peak in the budget, pinned pass counts, a CLAUDE.md, and a suite in two tiers

Session C of the overnight briefs (`briefs/C-guards.md`), on
`claude/perf-guards`. Nothing here changes how a world is made: no `.go`
file without `_test` in its name differs from main, and the digest for
valley, ancient and globe128 was as `digest.json` says before and after
every item. The branch was rebased twice as main moved under it: onto
59cdb90 (the deep floor), where the ancient and globe128 digests fell
behind the file and the tide-flats test failed on main and branch alike,
then onto 639d7d5, where main had merged the clock by pass, the weather
gate, a rewritten digest and a rewritten budget, and then onto bf5997c,
session B's hydrology. On bf5997c the digest matches, the pass counts
hold, the tide-flats test reads the third small globe as main now has it,
and the budget was rewritten once more to carry the peak: with the weather
gate and B's scratch kept on the grid the valley churns 9.9 MiB, ancient
55.5 and globe128 417.2.
Every timing below was taken with sessions A and B running on the same
machine at `GOMAXPROCS=8`; they are under load and are not the point.

**Scaling.** `BenchmarkNewLand` gains `globe128` and `globe512` beside
`globe256` and `globe`, and `scripts/perf.sh scaling` reads the median
`ns/tile` of three runs of each against the next, failing when 512 is more
than 1.3x 256. First run, under load:

| world | ns/tile (median of 3) |
|---|---|
| globe128 | 480 615 |
| globe256 | 238 907 |
| globe512 | 275 553 |

512/256 = 1.153, ok. 256/128 = 0.50: the bottom rung is still mostly the
constant each world carries, which is why the check reads 512 against 256.
An n log n pass costs 1.13 per doubling of width, so the limit is a
tripwire for a quadratic step, not a proof of linearity; a slower constant
moves every rung alike and is `check`'s to catch.

**Peak.** `TestWorldCreationBudget` samples the heap's live-objects metric
every millisecond while each world is made and writes the highest reading
over the pre-world baseline as `peak` in `budget.json`. It is logged
against the budget like the time and not held to a slack. Three runs of
each reading, under load, in MiB:

| world | sampled HeapAlloc | live at the pacer's marks | live at a forced mark every MiB |
|---|---|---|---|
| valley | 3.8, 3.8, 6.1 | 1.8, 2.1, 2.7 | 2.1, 3.8, 2.1 |
| ancient | 6.2, 14.1, 8.3 | 3.3, 6.3, 8.2 | 7.2, 6.7, 4.1 |
| globe128 | 21.0, 21.6, 22.1 | 11.3, 11.3, 15.8 | 20.7, 16.4, 15.0 |

A spread of 40-130% however it is read, and the same with the collector's
percent at 25 and 10 (which also cost globe128 two seconds and a few
hundred allocations). The cause is the concurrent collector: what the world
allocates during a mark is counted live, and a mark takes longer when the
machine is busy. On a quiet run the sampled figure was within 1-5%, so it is
the one kept. A steady peak needs the world to hold still while the heap is
read, which is a hook at each pass boundary; see "needs `phases.go`"
below. Chosen slack: none, until then.

**Pass counts.** `TestPassCountsArePinned` holds valley, ancient and
globe128 to a table of how many times `drain`, `weather`, `wear` and
`landslide` run, read off the clock in `phases.go` (run it with
`TERRA_PHASES=1 -count=1`). Written during the night against
`claude/perf-instrument` at e946062, where the weather ran 7, 25 and 31
times; on main at 639d7d5, after the weather gate (41bd904, drain reads
the weather only when the ground has moved from under it):

| world | drain | weather | wear | landslide |
|---|---|---|---|---|
| valley | 6 | 2 | 4 | 5 |
| ancient | 24 | 20 | 20 | 6 |
| globe128 | 30 | 23 | 20 | 6 |

Every other pass held, which is the test doing what it is for: the gate's
saving is five, five and eight weather passes, in a diff.

**CLAUDE.md.** Fifty-nine lines at the root: the determinism contract, the
merge checklist, the timeout, the stash rule, and where these docs are.

**The suite in two tiers.** `go test -json` of the whole root package,
under load: 1701 s for 226 tests, the one failure main has (`TestRealNumbers/Hack exponent, globe`). The thirty slowest are in `suite.md`; the top ten:

| # | seconds | test |
|---|---|---|
| 1 | 339.4 | `TestThePolarSeaIsIce` |
| 2 | 315.4 | `TestTheIceEdgeIsNotALineOfLatitude` |
| 3 | 258.2 | `TestTheRealWorld` |
| 4 | 111.5 | `TestTheColdKeepsToThePoles` |
| 5 | 90.4 | `TestAGlobeHasASeaItsRiversReach` |
| 6 | 77.4 | `TestSaltLakesStandInDryCountry` |
| 7 | 71.7 | `TestRealNumbers` |
| 8 | 62.6 | `TestMakingAWorldDoesNotDependOnTheGoroutines` |
| 9 | 50.3 | `TestTheSeaIsTheWorldsToSay` |
| 10 | 29.9 | `TestTheUplandMaskIsFinerThanTheMap` |

Where the time goes is worlds: a full globe is a minute and a half here,
a small one seven to nine seconds, and the slowest tests were the ones
that happened to make a world first, or made one the registry already
held. So:

- `yardWorlds` holds the whole `Land` now (`yardLand`), and the plate
  tests, the two polar tests, the climate's globe, the sea test, the river,
  rain, runoff and tide tests take their small globe or globe from it
  rather than making their own. `TestAGlobeHasASeaItsRiversReach` still
  makes its globe, because it times the making, and keeps it
  (`keepLand`) for the rest. That is three full globes and some twenty small ones not made twice, about eight minutes of the run under load.
- Under `-short`, everything that makes a globe is skipped: the
  goroutine-independence test (five small globes), the salt-lake and sea
  tests (eight and six), and every yardstick that reads small globes
  (`slow: true`, eighteen of them, which until now meant the full globe
  only). Nothing asserts differently; it runs later.

The short tier: 128 s under load (227 tests, 17
skipped), against 326 s before the gates. What is left in it is the plate
tests' three small globes (27 s under load, shared among five tests and
under the 60 s line each), the tidal-coast test (one small globe it must
wear itself, 20 s), the tide test's small globe 4 (8 s), the settlement
history (11 s), and the drawn valleys.

**Needs `phases.go`.** A pass-boundary hook for the peak: when the budget
test asks (a package-level `func(name string)` set from the test, or a
callback on the phase timer's stop), run `runtime.GC()` and read
`HeapAlloc` at the end of every pass. The largest reading is the peak with
nothing allocating, the same on any machine; then `peakSlack` in
`budget_test.go` turns the check on at 1% like the bytes.
## 2026-09-16 - Session B: the hydrology, exactly as it was, cheaper

The four items of [briefs/B-hydrology.md](briefs/B-hydrology.md), on branch
`claude/hydrology-exact`, one commit each. Every one leaves every world
bit-for-bit as it was: the digest of valley, ancient and globe128
(`TERRA_DIGEST=check`) matched the base after each item, and
`TestMakingAWorldDoesNotDependOnTheGoroutines` passed after each. Base:
main at 1ad4985 with the plan branch merged (51db354).

**Timing was taken under load and gates nothing.** Sessions A and C ran on
the same desktop all night, at `GOMAXPROCS=8` each. The first globe256
reading, base then item 1 in two separate count-6 runs, came out +18%
(p=0.002) for item 1 - and vanished when the binaries were interleaved,
one run each in turn for six rounds, which is how every table below was
taken. A sequential A/B under load measures the load's drift, not the
change. The interleaved table, globe256, n=6 each, all five binaries in
one session:

| binary | sec/op | vs base | B/op | vs base | allocs/op |
|---|---|---|---|---|---|
| base | 8.806 ± 10% | | 2.416 GiB | | 276.3 k |
| item 1, 4-ary heaps | 8.850 ± 5% | ~ (p=0.699) | 2.354 GiB | -2.58% | 275.2 k |
| item 2, scratch on the grid | 8.749 ± 7% | ~ (p=0.818) | 2.130 GiB | -11.87% | 272.2 k |
| item 3, pool repairs its order | 8.724 ± 8% | ~ (p=0.937) | 2.129 GiB | -11.89% | 272.2 k |
| item 4, solve by outlet tree | 8.607 ± 6% | ~ (p=0.937) | 2.083 GiB | -13.80% | 272.6 k |

Nothing in the clock is significant at these confidence intervals; the
bytes are exact. A quiet count-6 run in the morning is what will say what
the items bought on the clock; the budget says what they bought in memory.

**Budget** (`TestWorldCreationBudget`, Workers 4), bytes and allocations,
rewritten after items 1, 2 and 4 (item 3 left it unchanged):

| world | base | item 1 | item 2 | item 4 (final) | bytes | allocs |
|---|---|---|---|---|---|---|
| valley | 25.53 MB, 3124 | 24.61 MB, 2993 | 21.56 MB, 2905 | 21.00 MB, 2910 | **-17.7%** | -6.9% |
| ancient | 95.03 MB, 13643 | 91.29 MB, 13065 | 72.35 MB, 11814 | 68.79 MB, 11798 | **-27.6%** | -13.5% |
| globe128 | 651.85 MB, 45640 | 637.00 MB, 44799 | 576.52 MB, 43108 | 564.29 MB, 43255 | **-13.4%** | -5.2% |

### Item 1: a 4-ary heap for the floods and the slides (b40c3e2)

`floodQueue` (lake.go) and `slideQueue` (slide.go) are 4-ary heaps: a pop
walks half the ladder, and the four children it asks at each rung sit in
one cache line. Both comparisons are total orders - height, then the push
sequence or the tile's index - so which entries come out, and in what
order, does not depend on the heap's shape; `slideAt.less` was checked
before starting (two entries that compare equal are the same tile at the
same height, and the second is passed over as stale). Each queue's backing
slice is kept on the `Grid` (`floodScratch`, `slideScratch`); `fillFrom` in
shape.go, which floods with a `slideQueue` too, uses the same one. Budget
-3.6% / -3.9% / -2.3% bytes on valley / ancient / globe128. globe256:
8.850 s ± 5% against 8.806 ± 10%, ~ (under load); B/op -2.58%.

### Item 2: scratch on the grid (c53ffbc)

Every tile-sized `make` in `pool`, `flow`, `waterStep` (with `receivers`,
`deepReceivers`, `stackOf`, `fillFrom`, `edgeWork` and `stillWork`) and
`creep` is a slice kept on the `Grid` - `poolScratch`, `flowScratch`,
`stepScratch`, `creepScratch`, `fillScratch` - fitted by `sized`, which
remakes a slice only when its length is not the tile count. Where a pass
relied on `make` zeroing, it clears first: `gathered`, the flood's
`stand`/`reached`/`from`, the lakes' pools, the step's `f`, `drop`,
`settle`, `rock`, `eff`, `abrade`, `stackOf`'s tally, creep's `k` and
`gives`. `stand`, which `pool` calls while its own stack still has basins
on it, walks the tree on a slice of its own; the first draft shared it,
and the digest would have caught that. `Clone` leaves all of it nil.
Budget -12.4% / -20.8% / -9.5% bytes against item 1. globe256: 8.749 s
± 7%, ~ (under load); B/op -11.87% against the base.

### Item 3: pool repairs the last drain's order (39c2a0b)

The order is total, so the last call's `order`, its heights read again, is
put right with an insertion pass, and sorted afresh where more than a tenth
of the entries are out of place or the shifting has run to four times the
tiles. **Measured on globe256, 30 pools:** every one of the history's 22
drains gave up - an epoch moves the ground by kilometres - hitting the
shift bound with under a tenth of the entries moved, so the bound was set
where giving up costs about a tenth of the sort it then does (with the
bound at sixteen times the tiles the aborted pass cost as much as the
sort). The seven drains after the history found 0, 124, 171, 0, 0, 0 and
0 of 32768 entries out of place, shifted under a fifth of the tiles, and
were put right in under half the sort's time. So the item buys a little
on the drains outside a history and nothing inside one; the brief's
premise that the ground moves little between drains holds for a
settlement's decade and the making of a map, not for an epoch. The sort
itself (3 s of the globe's 100) is what is left; a radix sort on the
height's bits with the index as tie-break would give the same total order
and is the next thing to try there. Budget unchanged. globe256: 8.724 s
± 8%, ~ (under load).

### Item 4: `fluvial.solve` by outlet tree (cfdbc51)

Read `solve` and `account`. A tile's implicit step reads its receiver's new
height and what its donors passed it, and writes its own height, cut, load
and rate: nothing in `solve` reads a tile outside its own tree. `forest`
lays the stack out tree by tree, each tree's tiles in the order the whole
stack had them - the stack is breadth-first from all roots at once, and
its restriction to one tree is that tree's own breadth-first order, so
what arrives at a tile is summed in the order it always was - cuts the
trees into runs of about 4096 tiles, and deals the runs to `InParallel`;
each run does all its sweeps on its own. Digest unchanged at Workers 1, 4
and 24 (a throwaway test, not committed). `account` stays one pass on one
goroutine, and the comment says why: `lay` puts what a river lays over its
banks onto ground beside it (`overbank`), which may be in another tree, and
`exported`, the bays' pools and the surf's sands are sums over the whole
map in stack order. The solve's `next`, `cut`, `load`, the forest and
`account`'s load live in a `solveScratch` the step hands the fluvial from
the grid. Budget -2.9% / -4.9% / -2.1% bytes against item 3; allocations
+0.4% on globe128 for the goroutines dealt. globe256: 8.607 s ± 6% against
8.806 ± 10%, ~ (under load); B/op -13.80% against the base.

The fifth item, `fillFrom` as a bucketed flood, was not started.

### After the rebase onto main (639d7d5)

Main took the deep sea floor, the rock chemistry, the weather gate, the
clock by pass and the softened winters while this ran, and those move the
world, so `budget.json` was rewritten at the tip - not because these items
moved anything. The proof is the same as before, taken again on the new
base: main's own `digest.json` (current: written again from main's tree
and identical to the committed file) is what the tip makes, at Workers 1,
2, 3, 4, 8 and 16 (`TERRA_DIGEST=check` and the goroutine test both pass
at the tip). Budget at the tip against main, Workers 4: valley 14.86 ->
10.33 MB (**-30%**), 1519 -> 1299 allocs; ancient 84.44 -> 58.20 MB
(**-31%**), 12066 -> 10217 allocs; globe128 525.3 -> 437.5 MB (**-17%**),
36198 -> 33584 allocs. The percentages are larger than before the rebase
because the weather gate took much of the other allocation away. An
earlier rebase, onto 41bd904, read globe256 plan commit against tip,
interleaved n=6, under load: 6.105 s ± 20% against 6.427 s ± 22%, ~
(p=1.000); B/op -18.2%.

The full suite on the pre-rebase branch failed only
`TestRealNumbers/Hack_exponent,_globe` (0.6005 against 0.54-0.6), which is
main's known failure; see the base run below.

### What needs attention next

- A quiet count-6 run of globe256 and a count-3 globe, to read what items
  1-4 bought on the clock; the tables above only show they cost nothing
  measurable under load.
- `wear` still makes `change`, `gained` and `lost` afresh each age, and
  `solve`'s forest is rebuilt every step though the receivers change only
  with the ground; both are easy scratch.
- `pool`'s sort on a history's drains: a radix sort by the height's bits
  (with -0 folded to +0, so the order is exactly `heightBefore`'s).


---

## 2026-09-16 - The clock by pass, committed; the night's timings were taken under load

**Branch:** `claude/perf-instrument` from main 1ad4985, plus
`claude/work-trees-performance-plan-4b623c` ([scaling-plan.md](scaling-plan.md),
[briefs/](briefs/), `TestWorldDigest` and [digest.json](digest.json)), then
main again at 41bd904 when the weather gate (the entry below) landed while
this session ran. The gate moved every world, so the digest was rewritten
to the merged worlds; main's own digest, taken with the same test in a
worktree at 41bd904, is identical to the branch's, which is the proof that
the timer leaves the worlds as they were. A, B and C's start-of-session
digests match the committed file only if they branch from main after this
merge. Main moved once more, to b8d3856 (the sea read against its row, more
heat traded on a globe), before this branch could land; it was merged in
turn, the digest rewritten again (globe128 moved, valley and ancient did
not), and main's own digest at b8d3856 is again identical to the branch's.
The suite was not run a third time: the code the branch adds is the same,
and the two runs below are its proof. One thing b8d3856 brought is
`TestWorldCreationBudget` failing on globe128, +3.27% bytes and +4.51%
allocations against [budget.json](budget.json), on main itself as much as
here, with the instrument off or on. That budget is the climate change's
to rewrite, so it is left as it is.

**The timer.** [phases.go](../../phases.go) makes the throwaway
`defer phase("name")()` of the first entry permanent: 27 passes, from
`Generate` and `history` down to `airEnv.vapour` and `fluvial.solve`, summed
by name under a mutex when `TERRA_PHASES=1`. `BenchmarkNewLand` reports each
pass as `s/<pass>` and `cmd/overview` prints the table. Three things learned
while making it hold the acceptance checks:

- The first version returned a fresh closure per call, and with the
  instrument on the budget test failed on allocations (valley +4.4%,
  ancient +3.8%). The entries are now made at init with one stop function
  and a stack of start times each, so a call allocates nothing: the budget
  passes with the instrument off and on (valley 1520 / 1513 allocations
  against 1524, within the goroutines' noise), and the digest is identical
  both ways.
- The environment is read at package init, before `go test` starts
  recording what a test reads, so the test cache does not know the setting
  changed: switching `TERRA_PHASES` needs `-count=1`. The first "on" runs
  came back `(cached)` from the "off" ones.
- The testing package keeps only the first lines of a benchmark's log, so
  the table logged by the benchmark is cut after eight rows. The
  `s/<pass>` metrics on the result line are complete; `cmd/overview` prints
  the whole table with the calls.

**Off, the instrument costs nothing measurable.** Main's test binary (at
1ad4985) and the branch's, interleaved on the same loaded machine:

| world | main | branch | | n |
|---|---|---|---|---|
| valley | 143.0 ms ± 13% | 167.4 ms ± 10% | +17% (p=0.015) | 6 |
| ancient | 526 ms ± 15% | 505 ms ± 14% | ~ (p=0.94) | 6 |
| globe256 | 7.27 s ± 8% | 7.10 s ± 5% | ~ (p=0.39) | 6 |
| valley | 162.1 ms ± 6% | 162.5 ms ± 7% | ~ (p=0.81) | 15 |
| ancient | 528 ms ± 10% | 534 ms ± 6% | ~ (p=0.78) | 15 |

The n=6 valley reading did not survive n=15; the off path is one bool read
and a deferred no-op per pass call, a few hundred per valley.

**The suite** (`go test -timeout 60m .`): the branch fails the same tests as main, both times it was run. At 1ad4985 (before the gate) branch and main each failed one test, `TestRealNumbers/Hack_exponent,_globe` at 0.6005 against 0.54-0.60 (1639 s and 1622 s under load). At 41bd904 (the merged branch, and main in a worktree at the same commit) both fail `TestTheTideLaysFlatsOnlyWhereItReaches` (small globe 4 has no flats), `TestRealNumbers/drainage_area_exceedance_exponent,_small_globe` (0.4894 against 0.39-0.46) and `TestRealNumbers/discharge_exceedance_exponent,_small_globe` (0.4912 against 0.40-0.46), and the globe Hack exponent passes (1092 s and 1090 s). Those three are main's, from the deep floor and the rock chemistry that came in with the gate's merge; see the memory notes on the coasts workstream.

**The machine was not quiet.** Four to ten test processes from other
sessions ran the whole night (their suites, one at a 120 min timeout), so
every timing here is under load. Before the merge, `scripts/perf.sh check`
against the quiet [baseline/2026-09-15-2230-small.txt](baseline/2026-09-15-2230-small.txt)
read valley +9.9%, ancient +12.4% (fails the 10% limit), globe256 -7.5%
(the precomputes, as measured when they went in); the A/B table above says
the ancient reading is load, not the timer. That count-6 run is kept as
[baseline/2026-09-16-small-under-load.txt](baseline/2026-09-16-small-under-load.txt),
named so that the script's newest-baseline glob (`*-small.txt`) skips it:
against the quiet baseline it reads valley +24%, ancient +26%, globe256 ~.

**The baseline for the morning is [baseline/2026-09-16-0039-small.txt](baseline/2026-09-16-0039-small.txt)**,
count 6, at the merged branch (timer and gate), taken in the quietest hour
of the night (four foreign test processes, mostly idle). Against the gate's
own [2026-09-15-2333](baseline/2026-09-15-2333-small.txt) it reads valley ~
(p=0.31), ancient +7.6% (p=0.009), globe256 -4.0% (p=0.026), with intervals
of ±9-11% on both sides, which is load on both sides. **Retake it with
`scripts/perf.sh baseline` on a quiet machine before checking A, B and C**;
a quiet run on this machine has intervals of ±4-6%, and a baseline taken
under load hides regressions of the load's size.

**The globe by pass**, from `TERRA_PHASES=1 go run ./cmd/overview -preset globe`
at the merged branch (83.2 s, with two suites running; the same run before
the gate, on a lighter load, made the globe in 77.0 s and is the second
column). Inclusive; a pass's time includes the passes it calls. The two
columns were taken under different loads, so read the calls and the shares,
not the seconds, across them.

| pass | wall s | calls | share | before the gate | notes |
|---|---:|---:|---:|---:|---|
| `Generate` | 83.2 | 1 | 100% | 77.0 (1) | |
| `history` | 62.7 | 1 | 75% | 51.6 (1) | 16 epochs |
| `drain` | 33.0 | 30 | 40% | 34.1 (30) | = weather + pool + flow |
| `wear` | 20.9 | 20 | 25% | 16.9 (20) | |
| `weather` | 17.9 | 21 | 22% | 22.4 (31) | the gate: 21 readings, 18 of them the epochs' |
| `rainOn` | 14.1 | 21 | 17% | 17.8 (31) | |
| `waterStep` | 9.2 | 26 | 11% | 7.1 (26) | |
| `flow` | 8.2 | 30 | 10% | 6.5 (30) | serial |
| `airEnv.vapour` | 8.2 | 66 | 10% | 9.2 (96) | on the workers under rainOn; summed over goroutines |
| `silt` | 7.4 | 1 | 9% | 9.6 (1) | includes drains and tides |
| `pool` | 7.4 | 30 | 9% | 5.9 (30) | full sort per call |
| `cutValleys` | 7.3 | 1 | 9% | 9.6 (1) | |
| `orographic` | 7.1 | 63 | 8% | 8.9 (93) | 3 wind phases per rain |
| `move` | 6.1 | 16 | 7% | 5.2 (16) | |
| `fluvial.solve` | 6.0 | 26 | 7% | 4.5 (26) | serial |
| `creep` | 5.0 | 20 | 6% | 4.6 (20) | serial stencil |
| `tectonics` | 4.6 | 16 | 6% | 4.0 (16) | |
| `windsFor` | 3.7 | 21 | 4% | 4.6 (31) | |
| `landslide` | 3.0 | 6 | 4% | 4.1 (6) | |
| `tides` | 2.4 | 7 | 3% | 2.1 (7) | |
| `airEnv.currents` | 2.3 | 21 | 3% | 2.7 (31) | serial |
| `shape` | 2.2 | 1 | 3% | 2.1 (1) | |
| `reshape` | 1.9 | 16 | 2% | 1.4 (16) | |
| `joinUp` | 1.9 | 30 | 2% | 1.5 (30) | |
| `keepBook` | 1.0 | 16 | 1% | 0.6 (16) | |
| `basins` | 0.3 | 1 | 0% | 0.3 (1) | |
| `settleRock` | 0.03 | 1 | 0% | 0.02 (1) | |

The shape of the first entry holds after the gate: `drain` is 40% of the
clock, and what is left of it is the 18 per-epoch weather readings the gate
keeps (brief A's remaining question, a model decision) and the serial
hydrology, `pool` + `flow` + `waterStep`, now 30% (brief B).

---

## 2026-09-15 - drain reads the weather only when the ground has moved

**Change:** `drain` used to call `weather()` (winds for every phase, then the
vapour budget and orographic rain) every time. It now calls it only when
`weatherStale` says the ground the air reads has drifted since the last
reading: more than 0.5% of tiles have gone under the air's sea or come out of
it (`weatherFlips`), or the height over that sea has changed by more than 1% of
its total, summed tile by tile (`weatherDrift`). The drift is measured against
the last *reading*, not the last drain, so small changes cannot pile up
unnoticed. `weather()` stores the snapshot (`Grid.aired`, one float32 per
tile).

**Why these limits:** I probed how much the ground moves between consecutive
drains on `globe256`:

| drain site | calls | tiles flipped | height drift |
|---|---|---|---|
| history, per epoch | 16 | 20-35% | 45-99% |
| history end, Generate after pour | 2 | 35-64% | large |
| cutValleys | 4 | 0.03-0.48% | 0.2-0.4% |
| Generate after relevel | 1 | 0% | 0% |
| silt | 6 | 0-0.35% | ~0% |

The limits sit well above what valley cutting and silt do and far below what
an epoch of the plates does. With them, `globe256` refreshes 20 of its 30
drains (all 18 history drains, the first after pour and one in
cutValleys); `ancient` refreshes 19 of 24; `valley` 1 of 6.

**What it bought** (benchstat, count 6, against
[baseline/2026-09-15-2230-small.txt](baseline/2026-09-15-2230-small.txt)):

| world | sec/op | B/op | allocs/op |
|---|---|---|---|
| `valley` | 135 ms -> 95 ms, **-29.5%** (p=0.002) | -41.7% | -52.2% |
| `ancient` | 420 ms -> 402 ms, ~ (p=0.093) | -11.2% | -11.9% |
| `globe256` | 7.34 s -> 5.62 s, **-23.5%** (p=0.002) | -24.0% | -28.6% |
| `globe` (count 2) | 81.5 s -> 67.3 / 68.5 s, ~-17% | 18.5 -> 16.5 GiB | 932 k -> 686 k |

`ancient` hardly moves because its drains are almost all history drains.
Other sessions' test binaries were running on the machine during these runs,
so the new timings are, if anything, pessimistic.
New baselines: [baseline/2026-09-15-2333-small.txt](baseline/2026-09-15-2333-small.txt),
[baseline/2026-09-15-2333-globe.txt](baseline/2026-09-15-2333-globe.txt).
`budget.json` was rewritten (globe128: 614 -> 480 MiB).

**Realism:** full suite before (33c942b) and after:

- `TestRealNumbers/Hack_exponent,_globe` fails both times (after: 0.6005
  against 0.54-0.6). This is the known failure on main.
- `TestTheRealWorld/meander_wavelength,_small_globe` now reads 13.6 widths,
  **inside** 10-14. Its known-gap marker (B, 14.6) therefore fails with "the
  gap has closed". This yardstick rests on about 21 reaches (see the
  yardstick sample-size notes), so a one-width move is within its noise. At the
  user's call the marker was taken off, and the yardstick now passes.
- After merging main's soil work (a60e3a5): `valley floor over hillslope
  soil depth, small globe` read 3.015x against 3-50. That is past its
  known-gap marker (I, 2.96x on main without this change). At the user's call
  it was taken off like the meander one. The margin is 0.5%, so the next
  change to soils or rain may put it back.
- Everything else passes. `TestMakingAWorldDoesNotDependOnTheGoroutines`
  passes: the gate is serial arithmetic and draws nothing.

**Where the weather time is now:** 18 of the globe's remaining ~20 weather
readings are the per-epoch history drains. Each epoch really does move the
ground a lot, so skipping more there is a model decision (for example,
reading winds every other epoch while keeping rain every epoch) rather than
a free speedup. It would need the full suite as its judge.

---

## 2026-09-15 - Precompute in `airEnv.vapour` and `fluvial.solve`

The same idea as the sea-warmth links: take the work that is constant during
a solve out of the sweep, keep the sweep order, and keep the bits. The world
digests for valley, glacial valley, ancient and globe128 are identical
before and after.

**`airEnv.vapour`** already built its upwind links once. What was left per
visit:

- The rain slope's constant factor `rainScale·rainSteep·toStep·rainMost/rainSteps`:
  three multiplies and a divide on every visit. It is now computed once per
  cell as `slope`, in the same order.
- `math.Max`/`math.Min`: out-of-line assembly calls on amd64 (`archMax`/`archMin`
  were 3.9 s + 2.1 s flat on the globe). The builtin `max`/`min` have the
  same NaN and signed-zero rules and compile inline.
- Locality: each cell's `give`, `lose`, `toStep`, `rainScale`, `slope`, four
  sources and four shares now sit in one `vapourCell`, so a visit reads one
  run of memory instead of eight scattered arrays. `zonalCorrection` reads
  the same cells.

**`fluvial.solve`:**

- Every step of a history has no settling anywhere (deep tiles skip it; see
  `waterStep`). The per-tile settle sums (`Σ settle·parts`,
  `Σ settle·(load+supply)`) and the `·(1−settle)` on what is passed on are
  now skipped when `settles()` finds nothing to settle. Leaving them out
  gives the same bits for any finite load: +0 times something finite added
  to +0 is +0, and multiplying by 1 changes nothing.
- The builtin `max` replaces `math.Max` in `rate`, `below` and `cutAt`.
- A precomputed `share` array for valleys was tried and dropped. It cost
  n x 8 bytes per solve and pushed the budget over, while history, the
  target, doesn't use it.

**CPU, full globe profile** (one run each; wall clock too noisy to read, see
below):

| function | before (cum) | after (cum) | |
|---|---|---|---|
| `airEnv.vapour` | 15.09 s | 9.82 s | -35% |
| `fluvial.solve` | 10.59 s | 5.06 s | -52% |
| `airEnv.zonalCorrection` | 1.96 s | 1.05 s | -46% |
| `math.archMax` + `archMin` | 5.98 s | 3.31 s | the rest is outside these passes |

**World** (old and new binaries interleaved, n=6; quiet run, CIs ±2-6%):

| world | before | after | |
|---|---|---|---|
| ancient | 0.467 s ± 6% | 0.468 s ± 4% | ~ |
| globe256 | 6.61 s ± 2% | 6.24 s ± 2% | **-5.63% (p=0.002)** |
| globe (2 runs each) | 76.9, 76.4 s | 82.8, 69.9 s | inconclusive |

The globe's two new runs are 13 s apart, so load noise swamps the effect at
n=2. `fluvial.solve`'s saving is serial and should show up on the clock. It
needs a quiet count-6 globe run to confirm.

**Memory: the budget was rewritten on purpose.** globe128 came out +1.30%
bytes against the budget, over the 1% slack. `vapourCell` holds one more
float per air cell than the arrays it replaced (`slope`), across every
vapour call of every drain. I judged a divide per visit worth 8 bytes per
air cell and rewrote `budget.json` with `TERRA_PERF_UPDATE=1`: valley
+0.41%, ancient +0.33%, globe128 +1.30% (this includes the +0.3% from the
sea links). Allocation counts went down 3-3.5% (fewer separate arrays).

---

## 2026-09-15 - Jacobi for the sea's warmth: tried, rejected; the equations precomputed instead

The sea-warmth solve in `airEnv.currents` was the largest serial pass left
on globe256 (4.5 of 27 profiled s). It sweeps each cell's upwind equation in
four orders, Gauss-Seidel style. The question was whether Jacobi ordering,
where every cell reads the round before, would make it vectorizable and
parallel.

**First, the constant work.** The equation for each cell does not change
during the solve, but the sweep recomputed it every time: the upwind cell
along the row, the corner search down the column within `cornerReach`, and
the weights. `seaLinks` now builds the equations once and the sweep only
reads them. Sums are taken in the same order as before, so worlds are
bit-identical (ancient and globe128 digests unchanged).

**Jacobi, measured** (sweeps summed over the whole world, not the time of
one solve):

| world | Gauss-Seidel rounds (x4 sweeps) | Jacobi rounds | world time GS / Jacobi |
|---|---|---|---|
| globe128 | 369 (1476 sweeps) | 3169 | 3.32 s / 3.50 s |
| globe256 | 775 (3100 sweeps) | 14571 | 7.27 s / 7.77 s |

The Jacobi rounds were spread over rows with `InParallel`. It still lost, for
three reasons:

1. Warmth moves one cell per Jacobi round, whereas one Gauss-Seidel sweep in
   the current's direction carries it the length of an ocean. It took 4.7x
   the sweeps. AVX2 is at most 4 lanes and needs a gather for the upwind
   reads, so vectors cannot win that back.
2. The air grid is coarse, so a row is too little work to be worth a
   goroutine.
3. It changes the world a lot. At globe256, 6.5% of tiles have different
   terrain, the maximum height difference is 75 m, and sea warmth on a tile
   is up to 3.7 C apart (7.6 C at globe128). Part of that is chaos over 16
   epochs. Part is that the same "settled" threshold (1e-3 C change in a
   round) stops a slow Jacobi solve while it is still far from the answer,
   so it would also need a different stopping rule.

Jacobi was removed. A note on it stays in `gaussSeidel`'s comment.

**The precompute, measured** (old and new binaries interleaved, n=6):

| world | before | after | |
|---|---|---|---|
| ancient | 0.542 s ± 13% | 0.522 s ± 22% | ~ (no sea currents) |
| globe256 | 8.24 s ± 6% | 7.70 s ± 5% | **-6.51% (p=0.009)** |
| globe256 B/op | 2.398 GiB | 2.406 GiB | +0.32% |

In the profile, `currents` went from 4.50 to 2.09 CPU s cumulative (-54%).

The first version allocated six new tile arrays per solve and failed
`TestWorldCreationBudget` at +1.58% bytes on globe128, which is the budget
test doing its job. The links now write over `cu`, `cv`, `rise` and `deep`,
which are dead once the solve starts, and only the two int32 index arrays
are new.

**Takeaway for the other serial sweeps** (`airEnv.vapour`, `fluvial.solve`):
before reordering a Gauss-Seidel sweep, pull the constant per-cell work out
of it. That keeps the world bit-identical and is where the time actually
was. Reordering pays only if the transport is local (diffusion-like);
upwind transport along a flow is exactly what Gauss-Seidel in flow order
does fast.

---

## 2026-09-15 - SIMD for the transform's butterflies

**Change:** `fft_simd_amd64.go` does the FFT butterflies two complex numbers
per AVX2 vector under `GOEXPERIMENT=simd` (following `pass_simd_amd64.go`).
The complex product is written as `x*p + swap(x)*q`, which gives the same
bits as Go's complex multiply. `TestTheButterfliesAreTheScalarOnes` checks
every length from 1 to 4096 in both directions, including negative zeros and
values of very different sizes. The whole-world digests (ancient, globe128)
match between the two builds.

**Kernel** (`BenchmarkFFT`, forward and inverse together):

| length | scalar | simd | |
|---|---|---|---|
| 64 | 1212 ns | 1050 ns | -13% |
| 256 | 7837 ns | 4320 ns | -45% |
| 1024 | 34701 ns | 17698 ns | -49% |

**World** (scalar and simd binaries interleaved, n=6 each):

| world | scalar | simd | |
|---|---|---|---|
| ancient | 0.491 s ± 17% | 0.459 s ± 12% | ~ (p=0.093) |
| globe256 | 7.33 s ± 3% | 7.24 s ± 4% | ~ (p=0.065) |

The kernel is twice as fast, but world creation gains 1-4%, which is not
significant. This is Finding 1 of the first entry again: the FFT runs on
the `InParallel` workers, and wall-clock time is set by the serial passes.
On globe256 the main goroutine does 19 of the 27 profiled seconds, and the
largest serial pieces are `airEnv.currents` (17% of wall), the rest of
`rainOn`, `windsFor`, `slideQueue`, `pool`/`flow` and the priority floods.

**Why the serial passes are not vectorized:** `currents`, `airEnv.vapour`,
`fluvial.solve` and the floods are Gauss-Seidel sweeps or priority-queue
walks. Each tile reads the value its neighbour was just given in the same
sweep, so doing four at once changes the result. Making them vectorizable
(Jacobi or red-black ordering) would also make them parallel, but it changes
the world and moves the realism tests. That decision needs an owner.

---

## 2026-09-15 - Drift guards

**Heap budget in the suite.** `TestWorldCreationBudget` holds `valley`,
`ancient` and `globe128` to [budget.json](budget.json) (1% bytes, 3% allocs,
Workers pinned to 4). Before settling on those limits I measured the spread
at Workers 1/4/24, two runs each:

| world    | bytes spread | allocs spread | time |
|----------|--------------|---------------|------|
| valley   | 25.41-25.42 MB (0.05%) | 3219-3243 (0.7%) | 0.13 s |
| ancient  | 94.708-94.715 MB (<0.01%) | 14025-14045 (0.1%) | 0.43 s |
| globe128 | 643.50-643.67 MB (0.03%) | 45871-46698 (1.8%, most of it from the worker count) | 3.2 s |

To check it catches something, I lowered valley's budget by 5.6%: the test
failed with `+5.93%`, then passed again once the file was restored. It adds
~4 s to the suite.

**Timing drift by script.** `scripts/perf.sh check` compares 6 runs against
the newest baseline with benchstat and fails at a significant +10%. I tested
it on a synthetic run with `ancient` and `globe256` scaled by 1.25: both were
flagged at `+25.00%` (p=0.004 / p=0.002) and the script exited 1. The
unchanged baseline passed with "no significant change" on every world.

**The first baseline was taken on a loaded machine, and has been replaced.**
The first real `check` on unchanged code came back 26-33% *faster* than the
committed baseline (valley 0.199 -> 0.135 s, ancient 0.631 -> 0.420 s,
globe256 9.90 -> 7.34 s), and its confidence intervals narrowed from
±6-16% to ±4-6%. Other sessions were running on this desktop while the first
baseline was taken. `baseline/2026-09-15-2230-small.txt` now holds the quieter
run, and the table below uses it.

The lesson for the tool: a baseline taken under load hides regressions of up
to the load's size, and a check run under load fails on nothing. Before
taking a baseline or trusting a failed check, close other heavy work, look
at the CIs (a quiet run on this machine is ±4-6%), and run it again if they
are wide.

---

## 2026-09-15 - First baseline and where the time goes

**Commit:** 7dca920 (main, after globe crust balance)
**Machine:** AMD Ryzen 9 3900X, 12 cores / 24 threads, Windows 11, go1.27.0
**Raw output:** [baseline/2026-09-15-2230-small.txt](baseline/2026-09-15-2230-small.txt) (count 6),
[baseline/2026-09-15-2230-globe.txt](baseline/2026-09-15-2230-globe.txt) (count 3)

### Baseline (benchstat)

| world      | sec/op        | ns/tile | B/op      | allocs/op |
|------------|---------------|---------|-----------|-----------|
| `valley`   | 0.135 ± 6%    | 47 k    | 24.2 MiB  | 3.2 k     |
| `ancient`  | 0.420 ± 4%    | 146 k   | 90.3 MiB  | 14.0 k    |
| `globe256` | 7.34 ± 5%     | 224 k   | 2.39 GiB  | 277 k     |
| `globe`    | 81.5 (81-112 over 5 runs) | 155 k | 18.5 GiB | 932 k |

- The globe is noisy: five single runs through the day read 93.5, 111.5,
  91.7, 81.5 and 81.4 s. Background load on a desktop moves it by 30%.
  Trust `globe256` with count 6 for decisions; treat the globe as
  confirmation.
- Allocation counts and bytes are deterministic (± 0%), so any change in
  B/op or allocs/op is real even on one run.
- Peak live heap for the globe (from `GODEBUG=gctrace=1`) is ~460 MB, heap
  size peaks ~900 MB, across 75 GC cycles. GC CPU is negligible (reported 0%);
  what the 18.5 GiB of allocation costs is zeroing and page faulting
  (`memclrNoHeapPointers` 3.2 s, `sysUnusedOS` 1.5 s of CPU), not marking.

### Finding 1: the wall clock is serial

The globe CPU profile: 111.6 s duration, 180 s of samples - **161% of one
core on a 24-thread machine**. Of that, 94 s is on the main goroutine (under
`Generate`) and 78 s is on `InParallel` workers. The spread passes finish
their 78 CPU-seconds in roughly 15 wall-seconds; the serial passes are the
other ~85% of the clock. By Amdahl, making every parallel pass free would
save at most ~15%. **The attention belongs on serial work and on how often it
is repeated.**

### Finding 2: wall clock by pass (globe, one run, 100.9 s total)

Throwaway `defer phase(...)()` timers, inclusive (a pass's time includes the
passes it calls), not committed:

| pass            | wall s | calls | share | notes |
|-----------------|-------:|------:|------:|-------|
| `Generate`      | 100.9  | 1     | 100%  | |
| `history`       | 68.0   | 1     | 67%   | 16 epochs |
| **`drain`**     | **44.6** | **30** | **44%** | = weather + pool + flow, ~1.5 s each |
| - `weather`     | 29.3   | 31    | 29%   | winds and rain recomputed from scratch every drain |
| -- `rainOn`     | 21.0   | 31    | 21%   | |
| --- `orographic`| 9.9    | 93    | 10%   | 3 wind phases per rain; FFT per patch |
| -- `windsFor`   | 8.3    | 31    | 8%    | |
| - `flow`        | 8.8    | 30    | 9%    | serial |
| - `pool`        | 7.4    | 30    | 7%    | full sort of all tiles by height each call |
| **`wear`**      | **23.3** | **20** | **23%** | |
| - `waterStep`   | 8.6    | 26    | 9%    | mostly `deepReceivers` 7.5 s / `fillFrom` 8.1 s (priority flood) |
| - `fluvial.solve` | 8.7  | 26    | 9%    | serial |
| - `creep`       | 5.5    | 20    | 5%    | serial stencil |
| `silt`          | 13.2   | 1     | 13%   | includes 6.5 s of drain, 2.2 s of tides |
| `cutValleys`    | 11.9   | 1     | 12%   | drain 4.2, landslide 3.7, wear 3.1 |
| `move`          | 6.7    | 16    | 7%    | |
| `tectonics`     | 5.0    | 16    | 5%    | |
| `landslide`     | 4.8    | 6     | 5%    | `slideQueue.pop` 6 s CPU flat |
| `turn`          | 4.8    | 16    | 5%    | |
| `tides`         | 2.8    | 7     | 3%    | |
| `shape`         | 2.6    | 1     | 3%    | |
| everything else | < 2 each |     |       | firstPlates, joinUp, reshape, closedBasins, keepBook, carve, ... |

### Finding 3: CPU hot spots (flat)

| function | flat s | cum s | where it runs |
|----------|-------:|------:|---------------|
| `fft` | 18.8 | 20.3 | workers (orographic) - complex128 butterflies |
| `slideQueue.pop` | 6.1 | 7.5 | main (landslide) - heap pop |
| `airEnv.vapour` | 6.1 | 14.4 | workers (rainOn) |
| `liftField` | 5.9 | 35.0 | workers - transfer function loop |
| `fluvial.solve` | 5.9 | 9.1 | main |
| `Grid.orographic` | 5.5 | 5.9 | **main** - the patch sum-back loop with `%` per tile |
| `math.archExp` | 5.5 | | both |
| `airEnv.currents` | 4.5 | 6.6 | main |
| `Grid.creep` | 4.2 | 5.8 | main |
| `runtime.complex128div` | 4.0 | 4.5 | workers (liftField) |
| `Grid.flow` | 3.4 | 8.8 | main |
| `Grid.pool` | 3.2 | 7.8 | main (incl. 3 s of `slices.SortFunc`) |
| `math.archMax` / `archMin` | 3.1 / 1.9 | | both - `math.Max`/`Min` not intrinsified here |

### Finding 4: allocation (globe, 18.5 GiB over the run)

| site | alloc | note |
|------|------:|------|
| `orographic.func1` `sum := make([]float32, box*box)` | 2.17 GB | one per patch per phase per rain; also 60% of all objects |
| `airEnv.vapour` | 1.70 GB (2.94 cum) | per rain |
| `waterStep` | 1.42 GB (2.36 cum) | per wear step |
| `airEnv.box` | 0.96 GB | |
| `Grid.pool` | 0.84 GB | `order` and `own`, n-sized, per drain |
| `Grid.creep` | 0.78 GB | |
| `Grid.rainOn` | 0.70 GB | |
| `Grid.orographic` | 0.60 GB | `out`, `acc` per call |
| `floodQueue.push` / `slideQueue.push` | 0.60 / 0.58 GB | queue growth |
| `newAirEnv` | 0.52 GB | rebuilt per windsFor |

By pass: `drain` 4.8 GB, `wear` 3.9 GB, `weather` 3.2 GB, `silt` 2.5 GB,
`cutValleys` 1.7 GB, `windsFor` 1.7 GB. Nearly all of it is tile-sized
scratch slices made fresh on every call of a pass that is called 20-90 times.

### What needs attention, in order

1. **`drain` recomputes the weather 31 times (29 s, 29%).** `weather()` builds
   the winds (`windsFor`, a fresh `airEnv`) and the rain from nothing on
   every drain. Between two drains in a history the ground has moved by one
   epoch's erosion. Options, in order of payoff: reuse the air environment
   and wind solution as a warm start; only refresh winds every k-th drain or
   when the coast/relief changed by more than a threshold; skip `weather`
   where the caller only needs the routing (`pool`/`flow`) on unchanged rain.
   Any of these touches realism tests - check the climate yardsticks before
   and after. This is the largest single lever.
2. **Serial hydrology: `pool` + `flow` + `deepReceivers`/`fillFrom` (~24 s).**
   `pool` sorts all 524k tiles by height every call (30x); `fillFrom` is a
   priority flood repeated 18x. Reuse the previous order (the ground moves
   little, so an insertion-sort pass or a bucketed flood is near linear), and
   keep the n-sized scratch (`order`, `own`) on the grid.
3. **`wear` is all serial (23 s).** `creep` is a stencil and splits by rows
   with `EachRow` the way the read-only passes do; `fluvial.solve` and
   `account` are worth a look for the same. Must keep
   `TestMakingAWorldDoesNotDependOnTheGoroutines` green: split arithmetic
   only, never draws.
4. **Orographic rain (10 s wall, ~40 s CPU, 2.8 GB).** Cheap wins: write patch
   results into a per-worker pooled buffer instead of `make` per patch; take
   the `%` wrap out of the sum-back inner loop on the main goroutine (5.5 s
   flat); replace `math.Max`/`Min` with the builtins where NaN cannot occur;
   hoist the complex division in `liftField`. Deeper: `fft` in complex128 is
   18.8 s of CPU - split real/imaginary float64 arrays or a precomputed
   per-level root table. This mostly buys CPU rather than wall clock (see
   Finding 1) unless item 1 is done first.
5. **`silt` and `cutValleys` (25 s together, run once).** Most of their time
   is the drains and wears they call, so items 1-3 carry them.
6. **Tectonics (`move` + `tectonics` + `turn`, ~16 s).** Serial per epoch;
   profile further before touching.
7. **Allocation churn (18.5 GiB, peak live only ~460 MB).** Keep tile-sized
   scratch slices on the `Grid` (or a per-pass arena) across calls. The GC is
   not the cost; zeroing and page faults are, so expect a few percent, not
   tens. Do it alongside items 1-4 rather than as its own project.

Smaller observations:

- `globe256` costs 302 µs/tile against the globe's 155 µs/tile: at that size
  fixed and per-call costs (air environment, FFT plans, goroutine fan-out)
  dominate. Don't read per-tile wins on `globe256` as proportional on the
  globe.
- `landslide`'s `slideQueue.pop` is 6 s of flat CPU for 6 calls; a 4-ary heap
  or a bucket queue is worth trying.

### Next entry should

- Pick item 1 or 2, change it, and record `globe256` count 6 benchstat
  against [baseline/2026-09-15-2230-small.txt](baseline/2026-09-15-2230-small.txt), plus
  the globe count 3, plus whether the realism/climate tests still pass.

## 2026-09-16: the softened winters move the heap, not the code

Merging `claude/amazing-tesla-0fc728` (the sea about a place read against its
row, and a stronger land-sea exchange on globes) failed
`TestWorldCreationBudget`: `globe128` at +3.26% bytes and +4.52% allocations.

It is the world and not a new pass. Taking the valley's second energy-balance
solve out changed the count by 25 allocations of the 1567, and the time came
out 14% *faster* in the same run. Softer winters leave more water on the
ground, so a globe carries more lakes and channels to allocate for.

The budget was rewritten at the user's call: `globe128` 525.3 MB in 36198
allocations, against 508.7 MB in 34646. `valley` moved 0.03%, which is the
run-to-run noise the README quotes; it reads its day's range at `contValley`
and is otherwise untouched.
