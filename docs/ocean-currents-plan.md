# The ocean's currents: from a warmth field to things in the world

Written 2026-10-06 against main at `2a00ecb`. A plan in four workstreams,
O1 to O4, with what each one is for, what it touches, what it must leave
alone, and what has to land before it starts.

## Where it stands

`internal/atmos/ocean.go` works the sea out properly, in the order an
oceanographer reads it: the wind's stress, Sverdrup interior flow and a
western boundary current to close each gyre, the east-west flow that takes
the western current across the ocean, Ekman drift, coastal upwelling, and
the water's warmth carried along all of that by a Gauss-Seidel solve. Then
it keeps only the warmth:

- `cu`, `cv` (the current, m/s) and `rise` (upwelling, m/s) are made and
  then written over by `seaLinks` to hold the solve's weights. They exist for
  the length of one function.
- `temp`, the water's own temperature, is turned into `Warm` (degrees over
  the latitude's mean, clamped to ±10) and dropped.
- `Env.Warm` and `Env.Coast` are all that survive, and `Grid.SeaWarmth` and
  `Grid.CoastWarmth` all a caller can read.

So the world feels its currents - the cold coast is a desert, the subpolar
west coast is mild, a storm dies over cold water (`ocean_test.go`) - but
nothing can see one. There is no current to point at, to draw, to name, to
ask `Why` about, or to join to the coast it warms. A range is a feature
(`UpliftBelt`), a river's country is a feature (`DrainageBasin`), a climate
is a feature (`ClimateRegion`); the Gulf Stream is a blur in a temperature
field.

The model has gaps of its own besides, which the entities will make plain:

- Nothing within `gyreCalm` (5°) of the equator: no equatorial currents, no
  countercurrent.
- An ocean that runs more than `gyreOpen` of the way round is skipped, so a
  Southern Ocean has Ekman drift and no circumpolar current.
- No deep water: no overturning, no sinking where the sea is cold and
  salt.
- The sea's freeze is read off the air (`SeaFreeze` against the air's mean;
  `docs/climate-next-steps.md` step 2), not off the water the currents
  already warm.

## The shape of the work

```
O1 keep the fields ──► O2 currents as features ──► O3 relations and Why
        │
        └────────────► M track (docs/ocean-model-plan.md), world-moving
```

O1 changes no world and goes first, alone. O2 and M1 run side by side on
O1. O3 waits for O2. Every workstream follows `CLAUDE.md`'s before-merging
list; the digest line is given for each.

## O1. Keep what the sea already knows

Branch `claude/ocean-fields`. Digest: **checked, unchanged**.

- Keep the current (`Cu`, `Cv`, m/s), the upwelling (`Rise`, m/s) and the
  water's temperature (`SeaTemp`, °C, unclamped) on `atmos.Env` beside
  `Warm` and `Coast`, as `[]float32` on the air's cells. `seaLinks` takes
  its own scratch rather than writing over them. Nil on a valley, as `Warm`
  is.
- Read them per tile on `Grid`, the way `SeaWarmth` is read (`sky.go`):
  `SeaCurrent(i) (east, north float64)`, `Upwelling(i)`, `SeaTemp(i)`.
- `cmd/overview`: a currents map - streamlines over the water coloured by
  the water's warmth against its latitude, upwelling shaded - beside the
  wind's streamlines, which already exist to copy.
- The heap budget moves by four float32s an air cell; update it
  (`TERRA_PERF_UPDATE=1`).
- Tests: the fields are nil on a valley; the kept warmth equals the old
  `Warm` to the bit; the current under a western boundary runs poleward.

## O2. Currents as features

Branch `claude/ocean-features`, on O1. Digest: **checked, unchanged**
(the registry is a reading, not a change).

New `FeatureKind`s in `features.go`, read off O1's fields in tile order so
the ids are deterministic like every other kind's:

- `SeaCurrent`: a connected run of sea whose current is strong
  and runs one way, joined where neighbouring cells agree in heading
  (within a set angle) and speed (over a floor). Each carries:
  - `Flow` its strongest speed, `Heading` its mean direction;
  - `Transport`, sverdrups, across its narrowest section;
  - `Warmth`, the mean degrees its water stands over its latitude: warm
    or cold is read off this, not assumed from which side of the ocean it
    is on;
  - `Class`: western boundary, eastern boundary, drift (zonal, open
    water), equatorial (once M1 gives it one), circumpolar (likewise);
  - `Path`, its centre line from its head downstream, the way a basin
    carries its `Trunk`.
- `Gyre`: a closed loop of currents in one ocean basin, found as the
  connected sea whose currents turn the same way round one centre: sub-
  tropical or subpolar, clockwise or not. Its currents are its members.
- `Upwelling`: a run of coast where `Rise` is over a floor, with its mean
  rise and how much colder the water is than its latitude.
- `FeaturesAt` gains the sea's: a sea tile can be in a current, a gyre
  and an upwelling at once.
- The ocean basins themselves (sea between continents) are a natural
  `OceanBasin` kind too, and a gyre needs one to belong to; include it if
  it falls out of the gyre finding, leave it for later if not.

Tests: on GlobeTerms seed 3 there is at least one warm western boundary
current per gyre-bearing ocean, it runs poleward, and it is warmer than the
eastern boundary current of the same gyre; ids are stable across runs and
worker counts; a valley has none.

## O3. What the currents do: relations and Why

Branch `claude/ocean-relations`, on O2. Digest: **checked, unchanged**.

The registry today has no links between features, only a tile's
membership in each. This adds them, as recorded readings and never as a
second guess:

- `Relation{From, To FeatureID; Kind RelationKind; Quantity float64; Unit
  string}` on the registry, with `RelationsOf(id)`. Kinds, each with the
  number that makes it true:
  - `Warms` / `Cools`: a current and the climate region whose coast it
    lies off, with the degrees of `CoastWarmth` it is worth there.
  - `Dries`: an upwelling and the climate region (a B, mostly) beside it,
    with the share of rain the inversion takes (`inversion` in
    `ocean.go`).
  - `Waters`: a current and the drainage basins whose rain the air took
    up off it, from the phase's upwind walk already in `whyRain`
    (`upwindSea`), with the share of the basin's rain.
  - `PartOf`: a current and its gyre; a gyre and its ocean basin.
  - `Feeds`: one current into the next downstream along its `Path`, the
    Gulf Stream into the North Atlantic Drift.
  - `Freezes` (after M7's sea ice): a cold current and the sea ice it
    carries.
- `Why`: `OfRain` gains `OffshoreCurrent` (the current, its warmth, the
  extra damp `damp` gives) and `Inversion` (the upwelling, the share it
  takes); a new `OfWarmth` aspect gives `Latitude`, `Height`, and the
  current behind the tile's `CoastWarmth`. `cmd/overview/why.html` shows
  them.
- `cmd/overview`: hovering a current on the currents map lists its
  relations.

Tests: the Atacama case - the desert beside the strongest upwelling is
`Dries`-related to it, and `Why(OfRain)` there names it; the mild subpolar
west coast (`TestTheSubpolarWestCoastIsMild`'s tile) is `Warms`-related to
a drift whose `Feeds` chain reaches back to a western boundary current.

## O4. The physics the currents are missing

Superseded by `docs/ocean-model-plan.md`, the M track: the flow in two
dimensions, the thermocline, the heat the sea carries into the energy
balance, salt, the overturning, the sea's year and sea ice. Its first step,
M1, starts after O1 and runs alongside O2.

## Not in this plan

- Currents that carry anything but warmth: salt, sediment, nutrients, life
  in the sea. Upwelling as fishing grounds is a game's reading of O2's
  `Upwelling`, not terra's.
- Sea routes. terra's router walks; a sailing route would weigh current and
  wind, and that is a game's decision about boats that terra does not make
  (see `README.md`, "What it does not know"). O1's fields are what such a
  router would read.
- Currents through the history's ages. The warm-sea limestone
  (`terra-rock-chemistry-t6`) reads the latitude; reading the epoch's own
  currents would need the weather in every epoch.

## Open decisions for the owner

1. Whether `SeaCurrent` features are on the air's cells (coarse, cheap,
   the field's own resolution) or carried down to tiles like climate
   regions (their `Tiles` list on the map). Default: tiles, so `FeaturesAt`
   works the same for every kind.
2. Decided 2026-10-06 (see `docs/ocean-model-plan.md`): no, a reading
   only for now. Was: whether M5's overturning is allowed to move the Gulf Stream's
   warmth, once read.
3. Whether relations (O3) are the start of a general relation layer for
   every kind (belt `Shadows` basin, belt `Raises` plate boundary, basin
   `Fills` lake) or kept to the sea's for now. Default: built general,
   filled for the sea's first.
