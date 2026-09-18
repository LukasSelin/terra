# zarrdiff

`TestWorldDigest` says whether a change moved a world. zarrdiff says where
and by how much: it compares two Zarr v3 stores, as `cmd/zarr` writes them,
array by array and tile by tile.

```sh
cd cmd/zarr
go run ./zarrdiff base.zarr change.zarr               # a summary, most changed first
go run ./zarrdiff -json base.zarr change.zarr         # the same, for a script
go run ./zarrdiff -png where base.zarr change.zarr    # and a picture of each changed map array
go run ./zarrdiff -by book/meeting base.zarr change.zarr    # each array's changes by the meeting under each tile
go run ./zarrdiff -expect tweak.json base.zarr change.zarr  # did the change do what it was meant to?
```

It exits **0** when the stores hold the same, **1** when they differ and **2**
when they cannot be compared (a store missing, a flag wrong, a chunk that
does not decode, an `-expect` file that does not parse or names a code no
array has). With `-expect` it exits **0** when every check holds, however
much the stores differ, and **3** when one does not. `go run` turns every
non-zero status into 1, so a script that gates on the status builds it
first: `go build -o zarrdiff ./zarrdiff`.

## What it compares

Both stores are walked a group at a time (`Group.Children`), not a key at a
time, so a walk does not read the chunks; the arrays and groups in one alone
are listed. For each group in both, the attributes (`seed`, `terms`,
`sea_level`...). For each array in both:

- **shape, data type, attributes**, dimension names and fill value. An array
  whose shape or type differs is listed as *not compared*, first.
- **elements**, where shape and type agree: how many differ, NaN equal to
  NaN; the largest and mean `|a-b|` of those that have one (NaN against a
  number, and an infinite difference, are counted apart); for an array
  of amounts (not codes, not bools) the **signed change** `b-a` of those
  same elements: its mean, least and most, its 5th, 50th and 95th
  percentiles, and how many went up and how many down; the share of
  *tiles* changed, a tile being a cell of the first two dimensions (`y`,
  `x`), changed if any of its elements did, so a bed of `strata/` counts
  once for its tile; and the bounding box of the changed tiles.
- **codes**, for an integer array with CF `flag_values` and
  `flag_meanings` (or a `legend`, as stores before the xarray layout have):
  the commonest changes of code, each named from its own side's flags, so
  that a store whose codes were numbered differently still reads right.

How the arrays are chunked, sharded and compressed is not compared: a store
written with `-shard 0 -gzip -1` is the same as one written with the
defaults if its elements are.

The summary has a row for each array that differs, most tiles changed
first:

```
array                      tiles    max|a-b|   mean|a-b|  y            x
ground/height             47.20%       101.8       2.154  0-511        0-1023
                        247450 of 524288 tiles, 247450 of 524288 elements
                        b-a: mean -0.1414, 5% -6.8, 50% +0.001308, 95% +5.359, least -61.5, most +101.8; 131048 up, 116402 down
tile/terrain               2.09%                          0-511        0-1023
                        10973 of 524288 tiles, 10973 of 524288 elements
                            2211  0 open -> 1 wood
                            2161  1 wood -> 0 open
                            1383  0 open -> 2 water
```

(globe seed 1 with `-water 8` against the default 7.5, the example at the
end.)

`-png dir` writes `dir/<group>_<array>.png` for every changed array with two
or more dimensions: black where nothing changed, grey from 64 rising with
`|a-b|` to 255 at the array's largest change, and 255 for a change with no
amount (a code, a bool, NaN). Beds are folded into their tile. A map wider
than `-png-side` (4096) is shrunk, a pixel lit if any tile under it changed.

The percentiles are read off a histogram, not a sort, so that nothing is
held but a fixed count per tally: 32 bins to each doubling of `|b-a|` from
2^-64 to 2^64, either side of zero, 8192 counts in all (64 KiB). A
percentile is its nearest rank's bin, at the bin's geometric middle,
clamped to the least and most change: within 1.1% of the true value, and
exact where every change is the same. The mean, least and most are exact.

## By cause: -by, -only, -mask

`-by group/array` (repeatable) breaks every compared array's changes down
by the category of each tile in a map of codes: `-by book/meeting`,
`-by climate/koppen`, `-by tile/terrain`, `-by tile/bedrock`,
`-by features/belt`. The map is read from the base store, **a**;
`-by-side b` reads it, and the maps of `-mask` and of the checks' `where`,
from the changed one. It must be an integer or bool array of two
dimensions. Every changed array whose first two dimensions are the same
(the strata's beds folding into their tile) gets a table; the rest - the
feature table, the coordinates - none. For each category: its tiles, those
changed and the share, and for an array of amounts the signed change's
mean and 5th, 50th and 95th percentiles.

```
ground/height     47.20%       101.8       2.154  0-511        0-1023
                247450 of 524288 tiles, 247450 of 524288 elements
                b-a: mean -0.1414, 5% -6.8, 50% +0.001308, 95% +5.359, least -61.5, most +101.8; 131048 up, 116402 down
                by book/meeting in a
                  category          tiles    changed     share    mean b-a          5%         50%         95%
                  0 no_meeting     181664     100731    55.45%    -0.05592      -5.131   +0.003864      +4.605
                  1 collision       24426      17379    71.15%     -0.4313      -7.914  +0.0001708      +3.551
                  2 arc             70586      39723    56.28%     -0.1148      -8.819  +0.0001532        +6.8
                  3 islands         74957      25541    34.07%      -0.166      -8.819   +0.001458       +8.63
                  4 rift           171649      63470    36.98%     -0.2037      -7.416   +0.001031      +5.359
                  5 hotspot          1006        606    60.24%     -0.2021      -5.718  +0.0002935      +4.706
```

A coded map (CF `flag_values` and `flag_meanings`, or a `legend`) lists
every category it has tiles of, in order of code, named from its flags. A
map of ids (`features/belt`, `basin`, `lake`, `plate`, `climate`) may have
tens of thousands: it lists the `-top` (10) with most tiles changed and
sums the rest on one line. Feature ids are numbered afresh in every world,
so read them from the side whose features you mean.

`-only group/array` (repeatable) compares only that array, or, given a
group, every array under it. Groups' attributes are not compared then, and
arrays outside `-only` are not listed as in one store alone.

`-mask group/array=code[,code]` (repeatable) compares only the tiles whose
code in that map is one of those, by name (`collision`, `salt lake`: case
and spaces aside) or by number; a tile must be inside every mask. Counts,
shares, signed changes, code changes, pictures and checks are then of those
tiles alone: `-mask book/meeting=collision` asks what changed in the
collision belts. An array the masks do not lie over (not a map of their
shape) is left out, and listed as such.

## Expectations: -expect

`-expect file` evaluates checks against the report, and prints each with
what it measured. The file is JSON:

```json
{"checks": [
  {"name": "only the collisions", "array": "ground/height",
   "where": {"by": "book/meeting", "not": ["collision"]}, "measure": "share", "max": 0.001},
  {"array": "ground/height", "where": {"by": "book/meeting", "is": ["collision"]}, "measure": "mean", "above": 0},
  {"array": "climate/koppen", "measure": "share", "max": 0.02},
  {"array": "ground/flow", "measure": "changed", "max": 0},
  {"array": "tile/terrain", "measure": "code", "from": "water", "to": "open", "max": 1000}
]}
```

A check is of one `array`, held by an optional `where` to the tiles whose
code in the map `by` is one of `is`, or none of `not` (names or numbers, as
for `-mask`). Its `measure` is one of:

| measure | what |
|---|---|
| `changed` | elements changed; `"max": 0` is "unchanged" |
| `tiles` | tiles changed |
| `share` | tiles changed over tiles, 0 to 1 |
| `mean`, `p5`, `p50`, `p95` | the signed change `b-a` of an array of amounts |
| `up`, `down` | elements of an array of amounts that rose, and that fell |
| `code` | elements whose code went `from` one `to` another, each named from its own store's flags; takes no `where` |

It holds when the value is within every bound it gives: `min` and `max`
inclusive, `above` and `below` not. A check that can measure nothing - an
array in one store alone, a mean where nothing measured changed, a `where`
map that does not lie over the array - fails with a note saying why. A
check's array is compared whatever `-only` says.

## Memory

Nothing is read whole. An array is read in blocks of whole chunks along `y`
and `x`: one chunk high, and as many chunks wide as `-budget` elements
(2^20) allow, each chunk decoded once. Arrays are compared side by side, one
a processor, so the most held is about `2 × budget × element size ×
processors`: some 400 MiB for float64 over 24 processors, whatever the
size of the worlds. Two globes (1024 × 512) compare in about a second at a
peak of 120 MiB. A lower `-budget` holds less.

The maps of `-by`, `-mask` and `where` are read a chunk at a time, and the
decoded chunks shared between the arrays, which all read the same maps (on
Windows, opening their files for every array afresh took ten times as
long): at most `budget × processors` codes of 8 bytes are kept, the oldest
let go first. A tally is a few counts, and with its histogram 64 KiB: one
for each array of amounts, each category of a coded `-by` map (a few dozen
at most) and each `where`. The categories of a `-by` map of ids keep only
counts; a changed array is read a second time to bin its `-top` alone.
Two globes compare by four maps at once (`book/meeting`, `climate/koppen`,
`tile/terrain`, `features/belt`) in about 5 s against 4 s without, on a
loaded machine.

## Did my change move the world, and where?

Export the same world on the base commit and after the change, then diff.
A worktree keeps the base out of the way:

```sh
git worktree add ../terra-base main
(cd ../terra-base/cmd/zarr && go run . -preset globe -seed 1 -out /tmp/base.zarr)
cd cmd/zarr
go run . -preset globe -seed 1 -out /tmp/change.zarr
go build -o /tmp/zarrdiff ./zarrdiff
/tmp/zarrdiff -png /tmp/where /tmp/base.zarr /tmp/change.zarr
git worktree remove ../terra-base
```

Exit 0 is the digest's "unchanged", for every field the store holds. Exit 1
says which fields moved, over how much of the map and by how much; put the
summary beside the yardsticks in the work log. When the export layout
changed between the commits too, the arrays it added or dropped show as in
one store alone.

## Did my tweak do what I meant?

Say the tweak is `-water 8` against the default 7.5, on globe seed 1. Half
a metre more water is meant to raise the sea over the lowest shores, and to
leave the history, the mountains and the dry ground alone. Write that down
before looking, as `water.json`:

```json
{"checks": [
  {"name": "the history is untouched", "array": "book/meeting", "measure": "changed", "max": 0},
  {"name": "the sea rose", "array": "tile/terrain", "measure": "code", "from": "open", "to": "water", "min": 1},
  {"name": "and did not fall", "array": "tile/terrain", "measure": "code", "from": "water", "to": "open", "max": 100},
  {"name": "climates barely move", "array": "climate/koppen", "measure": "share", "max": 0.02},
  {"name": "mountains left alone", "array": "ground/height",
   "where": {"by": "book/meeting", "is": ["collision"]}, "measure": "share", "max": 0.01},
  {"name": "the dry ground does not sink", "array": "ground/height",
   "where": {"by": "tile/terrain", "not": ["water", "ice"]}, "measure": "mean", "min": 0},
  {"name": "plates keep their ids", "array": "features/plate", "measure": "changed", "max": 0}
]}
```

```sh
go run . -preset globe -seed 1 -out /tmp/base.zarr
go run . -preset globe -seed 1 -water 8 -out /tmp/water8.zarr
go build -o /tmp/zarrdiff ./zarrdiff
/tmp/zarrdiff -only book/meeting -expect water.json /tmp/base.zarr /tmp/water8.zarr; echo $?
```

`-only` keeps the report to the checks' arrays; the checks follow it:

```
checks: 3 of 7 hold
  pass  the history is untouched: book/meeting changed = 0, expected <= 0
  pass  the sea rose: tile/terrain code open -> water = 1383, expected >= 1
  FAIL  and did not fall: tile/terrain code water -> open = 397, expected <= 100
  pass  climates barely move: climate/koppen share = 0.01991, expected <= 0.02
  FAIL  mountains left alone: ground/height where book/meeting is collision share = 0.7115, expected <= 0.01
  FAIL  the dry ground does not sink: ground/height where tile/terrain is not water,ice mean = -0.2326, expected >= 0
  FAIL  plates keep their ids: features/plate changed = 524288, expected <= 0
3
```

The sea rose, but the tweak did more than was meant. Ask where:

```sh
/tmp/zarrdiff -only ground/height -by tile/terrain /tmp/base.zarr /tmp/water8.zarr
```

```
                by tile/terrain in a
                  category         tiles    changed     share    mean b-a          5%         50%         95%
                  0 open          145215     145213   100.00%     -0.3097      -8.819  +1.445e-05      +6.236
                  1 wood           45797      45797   100.00%    +0.05504      -7.257  +1.115e-05      +7.416
                  2 water         238328      40618    17.04%     +0.2334     -0.3005   +0.004696     +0.9892
                  4 outcrop         2213       2213   100.00%      -1.678      -13.02     -0.2697      +2.859
                  5 ice            90322      11245    12.45%    +0.09094     -0.2878  +0.0008123     +0.8501
```

Every tile of dry ground moved. The median moved by hundredths of a
millimetre, but a tenth moved by metres either way: the water reaches how
the ground wears, not only the shore. The plates did not move, but their
ids did, by 106 on every tile, as the features numbered before them
changed. `-mask tile/terrain=open`, or `-mask book/meeting=collision`, then
holds the whole report to those tiles, and `-png` shows where they are.
