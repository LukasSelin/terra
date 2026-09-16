# zarrdiff

`TestWorldDigest` says whether a change moved a world. zarrdiff says where
and by how much: it compares two Zarr v3 stores, as `cmd/zarr` writes them,
array by array and tile by tile.

```sh
cd cmd/zarr
go run ./zarrdiff base.zarr change.zarr               # a summary, most changed first
go run ./zarrdiff -json base.zarr change.zarr         # the same, for a script
go run ./zarrdiff -png where base.zarr change.zarr    # and a picture of each changed map array
```

It exits **0** when the stores hold the same, **1** when they differ and **2**
when they cannot be compared (a store missing, a flag wrong, a chunk that
does not decode). `go run` turns every non-zero status into 1, so a script
that gates on the status builds it first: `go build -o zarrdiff ./zarrdiff`.

## What it compares

Both stores are walked for `zarr.json`; the arrays and groups in one alone
are listed. For each group in both, the attributes (`seed`, `terms`,
`sea_level`...). For each array in both:

- **shape, data type, attributes**, dimension names and fill value. An array
  whose shape or type differs is listed as *not compared*, first.
- **elements**, where shape and type agree: how many differ, NaN equal to
  NaN; the largest and mean `|a-b|` of those that have one (NaN against a
  number, and an infinite difference, are counted apart); the share of
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
ground/height             96.44%        5517        2927  0-511        0-1023
                        505605 of 524288 tiles, 505605 of 524288 elements
tile/terrain              57.84%                          0-511        0-1023
                        303274 of 524288 tiles, 303274 of 524288 elements
                           76755  2 water -> 0 open
                           55583  5 ice -> 0 open
```

`-png dir` writes `dir/<group>_<array>.png` for every changed array with two
or more dimensions: black where nothing changed, grey from 64 rising with
`|a-b|` to 255 at the array's largest change, and 255 for a change with no
amount (a code, a bool, NaN). Beds are folded into their tile. A map wider
than `-png-side` (4096) is shrunk, a pixel lit if any tile under it changed.

## Memory

Nothing is read whole. An array is read in blocks of whole chunks along `y`
and `x`: one chunk high, and as many chunks wide as `-budget` elements
(2^20) allow, each chunk decoded once. Arrays are compared side by side, one
a processor, so the most held is about `2 × budget × element size ×
processors`: some 400 MiB for float64 over 24 processors, whatever the
size of the worlds. Two globes (1024 × 512) compare in about a second at a
peak of 120 MiB. A lower `-budget` holds less.

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
