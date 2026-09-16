# cmd/zarr

Makes a world and writes everything it knows, tile by tile, into a Zarr v3
store: one array for each thing a tile has, cut into chunks and kept in
shards. Go reads it with `github.com/LukasSelin/zarr`; Python reads it with
zarr-python or xarray.

It is a module of its own, so run it from here:

    go run . -out valley.zarr                     the valley, drawn
    go run . -preset ancient -out ancient.zarr    made from its history
    go run . -preset globe -out globe.zarr

## Terms

`-preset` gives the terms to start from, and `-terms file.json` a whole
`terra.Terms` as JSON instead (as the store's `terms` attribute holds
them). The flags override either: `-w`, `-h`, `-epochs`, `-sea`, `-water`,
`-wrap`, `-wetness`, `-woods` and `-growth` (`shape`, `tuned` or
`climate`), `-glacial` and `-max`.

    go run . -preset globe -wetness 1.5 -woods tuned -out wet.zarr
    go run . -terms wet.json -glacial -out wet-glacial.zarr

## The experiment loop

A tuning idea is tried by writing the world before and after it, and
asking where they differ. Three things make that fast.

**Keep the history once.** The history (the ground stage) is two thirds
of making a globe. `-keep-history file` writes it as the world is made;
`-from-history file` makes the world again from it, on the seed and terms
the file carries, in the time the stages after it take. A flag that would
change the seed or the terms is refused with `-from-history`.

    go run . -preset globe -keep-history globe.history -out base.zarr

A kept history is only valid for changes **after the ground stage**
(`stages.go`: ground -> sea -> shape -> cut -> coast -> cover). The file
is the grid as the code that wrote it left it: a change to the history
itself, the plates, or anything the ground stage runs, is not in it, and a
world made from it is the old history with the new later stages. The file
also refuses to load once the Grid's layout changes (a field added or
retyped). A change to the terms is a new history too: keep one per terms.

**Export base and tweak from it, and diff.**

    go run . -from-history globe.history -out base.zarr     # before the change
    # ... change the shaping, cutting, coast or cover ...
    go run . -from-history globe.history -out tweak.zarr
    go run ./zarrdiff base.zarr tweak.zarr

**Bisect by stage.** `-stages dir` writes a store at the end of every
stage, `dir/1-ground.zarr` to `dir/6-cover.zarr` (and `-out` only when it
is given). Each has a root attribute `stage`, and leaves out the groups
the grid has nothing for yet: `climate/` until the coast stage, `features/`
until the end of the cover, and `strata/` and `book/` where the map has
none. `6-cover.zarr` is the store `-out` writes, byte for byte. From a
history, `1-ground.zarr` is the grid as the file kept it.

    go run . -from-history globe.history -stages base      # before
    go run . -from-history globe.history -stages tweak     # after
    go run . -stages-diff base tweak

`-stages-diff a b` compares the two directories stage by stage, file by
file (a store is written the same every time from the same grid), leaving
out the root `zarr.json` so that a change of terms is not a difference
until it reaches the grid. It prints each stage as the same or the arrays
that differ, then the first stage that differs and the `zarrdiff` command
that says by how much. It exits 0 when every stage is the same, 1 when one
differs and 2 on an error. Both directories must be written with the same
`-chunk`, `-shard` and `-gzip`.

```
1-ground the same
2-sea    the same
3-shape  differs: ground/flow
4-cut    differs: ground/clay, ground/drain, ground/flow, ground/height, ...
...
first differs at 3-shape; to see how:
  go run ./zarrdiff base/3-shape.zarr tweak/3-shape.zarr
```

(A valley at `-wetness 2` against 1: the wetter air first shows in the
flow the shape stage drains.)

What it buys, on a globe (`-preset globe`, 1024 by 512; Ryzen 9 3900X, 24
threads, other sessions on the machine):

| run | making | writing |
|---|---|---|
| `-preset globe -out` | 76 s | 1.4 s |
| `-from-history globe.history -out` | 16-17 s | 1.1-1.8 s |
| `-from-history globe.history -stages` | 22.5 s, the six stores included (0.6-0.9 s each) | |

The history file is 198 MiB.

## Layout

Every array of the map is H by W, dimensions `y` (the row) and `x` (the
column); the strata add `bed`, from the top down. Every group with those
dimensions has them as coordinate arrays of its own, so `xr.open_zarr(...,
group=...)` finds them whichever group it opens.

| group | arrays |
|---|---|
| `/` | attributes only: `stage` (`cover` for a finished world), `seed`, `terms` (JSON text), `width`, `height`, `wrap` (0 or 1), `sea_level`, `chunk_side`, `tile_span` |
| `ground/` | `height`, `flow`, `drain`, `soil`, `sand`, `clay` |
| `tile/` | `terrain`, `bedrock`, `mark`, `owner`, `fenced`, `plate`, `formed`, `leached`, `exposed`, `lime`, `salt`, `carbon` |
| `layers/` | `traffic`, `age`, `fish`, `wood`, `wild`, `fertility`, `rich`, `sward`, `kinds` |
| `climate/` | `mean`, `coldest`, `warmest`, `rain`, `runoff`, `rain_warm`, `koppen` |
| `strata/` | `count` by tile; `top`, `rock`, `formed`, `sand` by bed |
| `book/` | `lift`, `worn`, `plate_a`, `plate_b`, `meeting`, `epoch`, `burial`, `buried_in` - made worlds only; a drawn map has no `book` group |
| `features/` | `belt`, `basin`, `lake`, `plate`, `climate`: each tile's feature id, 0 for none |
| `features/table/` | one entry a feature along `feature`, whose coordinate is the id |

**Coordinates.** `y` and `x` are float64 metres of the centre of a row or
column from the centre of tile (0, 0): the index times `terra.TileSpan`,
25 m. On a globe (`wrap` 1) column W is column 0 again; the `x`
coordinate's `comment` says so. They are map metres, not latitude and
longitude: the root package does not expose the latitude its climate
reads a row at. `bed` counts from 0 at the top; `feature` is the id the
map's feature arrays hold.

**Attributes** follow CF where it is cheap:

- `long_name` and, where there is one, `units` (UDUNITS spellings: `m`,
  `m3 s-1`, `mm year-1`, `degC`, `kg m-2`, `1`). `book/lift`, `book/worn`
  and the table's `lift` are in the history's metres, which their
  `comment` says.
- Coded arrays carry `flag_values` and `flag_meanings` over **every** code
  the world can hold, not just the ones present. A meaning's spaces are
  underscores (`salt_lake`). There is no `legend` any more: a JSON object
  attribute cannot be written to netCDF, so an xarray `to_netcdf` of a
  store with it failed, and two tables of one thing can disagree.
- `climate/koppen` codes are one fixed table: 0 water, then `Af Am Aw BWh
  BWk BSh BSk Csa Csb Csc Cwa Cwb Cwc Cfa Cfb Cfc Dsa Dsb Dsc Dwa Dwb Dwc
  Dfa Dfb Dfc ET EF` as 1 to 27. The same code is the same type in every
  world. New types are only ever added at the end.
- `scale_factor` on the arrays the world keeps as whole numbers of a
  fraction: `tile/leached` (1/65535), `tile/lime` (0.01 kg m-2),
  `tile/salt` (0.001 kg m-2), `strata/sand` (1/255). xarray reads the
  amount; zarr-python and Go read the stored integer.
- `_FillValue` 255 on `strata/rock` and `strata/formed`, which hold 255
  past the bottom of a pile, so xarray reads those beds as NaN. `strata/top`
  is NaN there already. `strata/sand` is 0 there, which a bed can be too:
  mask it with `top` or `count`.

Every array is kept in the type the world keeps it in (uint8 codes, uint16
soil chemistry, float32 and float64 fields); the CF attributes change what
xarray decodes, not what is stored.

The root `zarr.json` carries every node's metadata inline under
`consolidated_metadata`, as zarr-python writes it, so xarray opens a store
from one key. That field says `must_understand: false`; a reader that does
not know it reads each node's own `zarr.json`, which is still written.

## xarray

```python
import xarray as xr

tree = xr.open_datatree("globe.zarr", engine="zarr")     # every group
ground = xr.open_zarr("globe.zarr", group="ground")
climate = xr.open_zarr("globe.zarr", group="climate")

# A tile by metres, and a region by slice.
ground.height.sel(x=2500.0, y=1000.0)
ground.height.sel(x=slice(0, 5000), y=slice(0, 5000)).mean()

# Names for codes.
k = climate.koppen
names = dict(zip(k.flag_values, k.flag_meanings.split()))
(k == k.flag_values[k.flag_meanings.split().index("Cfb")]).sum()

# Beds past a pile are NaN.
strata = xr.open_zarr("globe.zarr", group="strata")
strata.rock.count("bed")        # beds under each tile
```

With `cf_xarray` imported, `k.cf == "Cfb"` does the lookup.

`TestXarrayReadsAStore` checks a store with xarray. It is skipped unless
`ZARR_PYTHON` names a Python with xarray, zarr and numpy:

    python -m venv .venv && .venv/bin/pip install xarray zarr numpy
    ZARR_PYTHON=.venv/bin/python go test -run Xarray
