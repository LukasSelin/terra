# cmd/zarr

Makes a world and writes everything it knows, tile by tile, into a Zarr v3
store: one array for each thing a tile has, cut into chunks and kept in
shards. Go reads it with `github.com/LukasSelin/zarr`; Python reads it with
zarr-python or xarray.

It is a module of its own, so run it from here:

    go run . -out valley.zarr                     the valley, drawn
    go run . -preset ancient -out ancient.zarr    made from its history
    go run . -preset globe -out globe.zarr

## Layout

Every array of the map is H by W, dimensions `y` (the row) and `x` (the
column); the strata add `bed`, from the top down. Every group with those
dimensions has them as coordinate arrays of its own, so `xr.open_zarr(...,
group=...)` finds them whichever group it opens.

| group | arrays |
|---|---|
| `/` | attributes only: `seed`, `terms` (JSON text), `width`, `height`, `wrap` (0 or 1), `sea_level`, `chunk_side`, `tile_span` |
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
