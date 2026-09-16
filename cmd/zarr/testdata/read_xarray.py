"""xarray on a store cmd/zarr wrote: see xarray_test.go.

    python read_xarray.py STORE EXPECT    open STORE with xarray and check it
                                     against what EXPECT, a JSON file the
                                     test wrote from the world, says

Every warning xarray raises while it opens and loads the store fails the
check, as does any array whose decoded values are not zarr-python's raw
values with the CF scale and fill applied.
"""

import json
import sys
import warnings

import numpy as np
import xarray as xr
import zarr

store, expect = sys.argv[1], json.loads(open(sys.argv[2], encoding="utf-8").read())
failures = []


def check(ok, what):
    if not ok:
        failures.append(what)


with warnings.catch_warnings(record=True) as caught:
    warnings.simplefilter("always")
    tree = xr.open_datatree(store, engine="zarr")
    groups = {g: xr.open_zarr(store, group=g).load() for g in expect["groups"]}
for w in caught:
    failures.append(f"warning: {w.category.__name__}: {w.message}")
check(set(expect["groups"]) <= {n.path.lstrip("/") for n in tree.subtree}, "the tree lacks a group")

# Every array decodes to its raw elements, scaled and masked as its
# attributes say.
for g, ds in groups.items():
    for name, v in ds.variables.items():
        raw = zarr.open_array(store, path=f"{g}/{name}", mode="r")[...]
        want = raw
        if "_FillValue" in v.encoding or "scale_factor" in v.encoding:
            want = raw.astype("f8")
            if "_FillValue" in v.encoding:
                want = np.where(raw == v.encoding["_FillValue"], np.nan, want)
            want = want * v.encoding.get("scale_factor", 1)
        if v.dtype.kind == "f":
            ok = np.allclose(v.values, want, rtol=1e-6, equal_nan=True)
        else:
            ok = v.dtype == raw.dtype and np.array_equal(v.values, want)
        check(ok, f"{g}/{name} decodes as {v.dtype}, not as its elements")

# A coded array: the CF flags name the world's own type at a tile.
climate = groups["climate"]
koppen = climate.koppen
meanings = dict(zip(koppen.attrs["flag_values"], koppen.attrs["flag_meanings"].split()))
y, x = expect["koppen"]["y"], expect["koppen"]["x"]
got = meanings[int(koppen.isel(y=y, x=x))]
check(got == expect["koppen"]["type"], f"koppen at ({x}, {y}) is {got}, not {expect['koppen']['type']}")
check(meanings == {int(k): v for k, v in expect["koppen"]["flags"].items()}, "koppen's flags are not the table")

# A coordinate: x and y in metres, and a tile found by them.
ground = groups["ground"]
check(ground.x.attrs["units"] == "m" and float(ground.x[-1]) == expect["x_last"], f"x ends at {float(ground.x[-1])}")
h = float(ground.height.sel(x=x * expect["tile_span"], y=y * expect["tile_span"]))
check(h == expect["height"], f"height by metres is {h}, not {expect['height']}")

# A fill value: beds past a pile read as NaN, the beds in it as their rock.
strata = groups["strata"]
check(strata.rock.dtype.kind == "f", f"strata/rock decodes as {strata.rock.dtype}")
check(int(strata.rock.isnull().sum()) == expect["beds_missing"], f"{int(strata.rock.isnull().sum())} beds past the piles")
check(int(strata.top.isnull().sum()) == expect["beds_missing"], "top and rock miss different beds")
check(float(strata.rock.isel(y=y, x=x, bed=0)) == expect["rock_top"], "the top bed's rock")

# A scaled array reads as the share it is.
check(abs(float(groups["tile"].leached.isel(y=y, x=x)) - expect["leached"]) < 1e-9, "leached is not a share")

if failures:
    print("\n".join(failures))
    sys.exit(1)
print(f"xarray {xr.__version__}, zarr {zarr.__version__}: {len(groups)} groups read as the world")
