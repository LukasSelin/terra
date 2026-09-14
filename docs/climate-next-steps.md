# The climate: where it stands and what comes next

Written 2026-09-14, after fitting the globe's warmth by latitude to the
observed zonal means (commit `b15fff9` on `claude/cool-wu-1adbf4`, not merged).
It is a list of what that fit exposed, in the order the work wants doing.
The figures were measured against main at `ee9ea9a`, before the layered-rock
merge.

## Where it stands

`warmth(lat)` is now a Legendre series in the sine of the latitude,
`LatP2*(P2 - P2(45)) + LatP4*(P4 - P4(45))`, with `LatP2 = -26.5` and
`LatP4 = -4.5`. That is the energy-balance climate's form (North, Cahalan &
Coakley 1981; North 1975 for the fourth term), fitted by least squares with
forty-five degrees held at `MeanTemp` so the valley is unchanged.

| lat | observed (NH zonal, sea level) | old cosine | fitted |
|---|---|---|---|
| 0  | 26.5 | 18.8 | 26.4 |
| 30 | 20.5 | 16.0 | 19.3 |
| 45 | 11   | 10   | 10   |
| 60 | 1    | 4.2  | -1.9 |
| 75 | -11  | -3   | -12.1 |
| 90 | -18  | -11  | -16.2 |

The equator yardstick closed. The forest, frost, ice and water thresholds
were set against the old, warm poles, and the fit moves several of them out
of range. On workstream A's branch, GlobeTerms seeds 3 and 1, as a share of
land by tile count:

| | before | after |
|---|---|---|
| ice cap | 0 / 1.5% | 3.3 / 5.1% |
| tundra | 4.9 / 7.2% | 10.2 / 13.1% |
| desert | 3.1 / 4.1% | 9.6 / 9.8% |
| forest | 56 / 56% | 47 / 47% |
| sea ice, of the map | 7.6 / 7.5% | 19.7 / 18.7% |
| permafrost | 8.7 / 8.8% | 21.3 / 25.8% |

Peel, Finlayson & McMahon (2007) put Earth's land at 19% A, 30% B, 13% C,
25% D and 13% E.

## Next steps

### 1. Land workstream A first

On main alone the fit breaks the forest. `Grid.Frozen` calls ground frozen
wherever the annual mean is under `Frost` (4 °C). With real polar
temperatures that covers everything past about 55 degrees, so the humid
land at 55-70 degrees went from 934 wooded tiles to 40. The forest-by-aridity
yardstick went from 0.38 to 5.8. The real taiga grows at annual means of -5
to +5 °C. Workstream A (`claude/climate-thresholds-a`) reads the tree line off
the warmest month (Körner) rather than the mean. The fit cherry-picks cleanly
onto it and should be rebased there once A is in.

### 2. Freeze the sea on the sea, not on the air

`SeaFreeze` (-1.8 °C, the real freezing point of sea water) is compared with
the air's annual mean at the surface. Sea ice is a matter of the water's
temperature through the winter, and the air over a sea-ice edge averages
well under the water's freezing point. With the fitted curve crossing -1.8 at
sixty degrees, a fifth of the map is sea ice, where Earth's annual mean
extent, some 25 million km² between the hemispheres (NSIDC), is about 5% of
the surface. Options, in order of preference:

- read the freeze off the ocean model's own water temperature
  (`ocean.go`'s `temp`), which already carries the currents;
- read it off the coldest month rather than the annual mean, as the tree
  line is read off the warmest;
- as a stopgap, move the threshold to the air mean at the observed
  annual-mean ice edge, with a source for the figure.

### 3. Give the maritime warming the steeper gradient it now faces

`TestTheIceEdgeIsNotALineOfLatitude` fails on seed 2: 308 of 1024 columns
turn green on one row. The fitted gradient near sixty degrees is about
0.8 °C a degree, twice the cosine's 0.4. The same `Maritime` warming now
moves the edge half as many rows, so the edge is ruled again. Either the
maritime and coast warmth should scale with the gradient, or their
magnitudes should be checked against a source (the real tree line swings
some twelve to fifteen degrees of latitude between Norway and Labrador).

### 4. Revisit the permafrost bound

`TestTheColdKeepsToThePoles` allows 25% of land as permafrost, and the fit
gives 25.9%. Real permafrost underlies about 24% of the northern
hemisphere's exposed land (Zhang et al. 2008) and some 15% of the world's
(Obu et al. 2019). The bound is right for a planet. The count is not: it is
of tiles, and a tile at seventy degrees is a third the area of one at the
equator. Fix step 5 before deciding whether the ground is too cold.

### 5. Weigh the shares by area

The overview and the land-share tests count tiles on an equirectangular
grid, which overweights the poles by 1/cos(lat). Every comparison with
Earth's Köppen shares, sea-ice cover or permafrost should weight a tile by
the cosine of its latitude, or the polar classes will read too large
whatever the climate does.

### 6. Close the 60-degree cold bias

The fit is 3 °C too cold at sixty degrees, and that is where the tree line,
the permafrost edge and the sea-ice edge all fall. With forty-five pinned,
two terms cannot bend any closer; an area-weighted fit only gets to -1.5.
Options:

- add P6 to the series;
- let forty-five go to its observed 11 °C and move the valley's reference
  latitude to where the curve crosses `MeanTemp`;
- accept the bias and let the maritime and current warmth carry it, since
  sixty degrees on Earth is mostly ocean-warmed coast.

### 7. The southern hemisphere

The fit is to the northern hemisphere and is mirrored. Antarctica is some
-50 °C at the pole (Peixoto & Oort 1992), and the poles yardstick is still a
known gap. A hemisphere term, or elevation-driven cold on a polar continent
through `Lapse`, would close it. That is only worth doing once the
thresholds above are settled, because it makes every polar class larger.

### 8. Recalibrate the water

The PET table, runoff and river networks follow the new temperatures by
themselves (`petTable` is keyed by the mean), but the tuned outputs moved:

- `TestSaltLakesStandInDryCountry`: small globe seed 2 has no salt lake left.
- Small-globe discharge exceedance exponent 0.45 to 0.51, drainage-area
  exponent 0.45 to 0.47, Hack exponent 0.604 (range 0.40-0.46, 0.54-0.60).
- Globe drainage-area exponent 0.29 to 0.46, into range on main and just over
  on A.
- Runoff at 65-90 degrees rose from ~40 to ~150 mm, because the cold air
  takes back less. The greatest river on GlobeTerms seed 1 doubled
  (77 to 158 m³/s).

Revisit these after steps 1-5, since the sea ice and frozen ground move the
same catchments.

## Baselines already failing on main

Not caused by any of this: `TestAGlobeHasASeaItsRiversReach` (water 0.78),
`TestEveryRockAHistoryMakesTurnsUp` (seed 10 schist), and the three tide-flat
tests.
