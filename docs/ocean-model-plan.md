# The ocean as a body of water: a modelling roadmap

Written 2026-10-06 against main at `2a00ecb`, as the physics half of
`docs/ocean-currents-plan.md`. It replaces that plan's O4. O1 to O3 (keep
the fields, currents as features, relations) stand, and read whatever this
track gives them.

## Why the sea is one-dimensional now

Read `internal/atmos/ocean.go` and `internal/atmos/ebm.go` together and the
ocean is flat in three ways:

1. **Row by row.** The gyres are solved one parallel at a time: Sverdrup's
   balance along each row, a western wall to close it, then a second
   row-by-row sweep for the east-west flow, then `gyreRows` of smoothing to
   hide that neighbouring rows disagree, and `cornerReach` so a current can
   find water round a slanting coast. A current never turns a corner because
   the model has no corners. Islands, straits, an ocean all the way round
   (`gyreOpen`) and the equator (`gyreCalm`) are all skipped, because the
   row balance has nothing to say about them.
2. **One layer, one tracer.** The water is a single surface layer carrying
   only temperature. There is no thermocline, so upwelling brings up a
   fixed `upwellContrast` from nowhere in particular; no salt, so no density;
   no depth, so no overturning and no deep water.
3. **An anomaly on a latitude.** The planet's warmth by latitude comes
   from the zonal energy balance (`ebm.go`), whose single diffusion carries
   "the heat the air and the sea carry" together. The currents then add a
   ±10 °C anomaly relaxed back toward that latitude's mean. The ocean
   moves no heat of its own: a world whose Gulf Stream runs twice as strong
   has the same equator-to-pole contrast. There is also no season in the
   sea: it is solved once, under the year's mean wind.

The aim is the class of ocean an Earth-system model of intermediate
complexity carries: two-dimensional, a few layers, temperature and salt,
an overturning, sea ice, and heat it actually transports. Fast, because the
whole weather runs in seconds. Not an ocean GCM.

## What it is measured against

The worlds are not Earth, so every yardstick is a ratio or a rule that holds
on any geometry, with Earth's figure as the check where the geometry is
Earth-like:

| Reading | Earth's figure | Source |
|---|---|---|
| Western boundary transport = interior Sverdrup transport | closes to within friction | Stommel 1948; Munk 1950 |
| Total poleward heat transport, peak | ~5.5-6 PW at 35° | Trenberth & Caron 2001 |
| Ocean's share of it at 35° | 22% north, 8% south | Trenberth & Caron 2001 |
| Overturning of the most overturning basin | 17.2 Sv, 1.22 PW at 26.5°N | RAPID, McCarthy et al. 2015 |
| A circumpolar current, where a gap allows one | 173 Sv at Drake Passage | Donohue et al. 2016 |
| Flow between an island and the far shore | island rule; Indonesian throughflow 16 ± 4 Sv | Godfrey 1989 |
| Equatorial west-east SST contrast | ~4-6 °C, warm pool to cold tongue | WOA climatology |
| Annual-mean sea ice, area-weighted | ~5% of the surface | NSIDC |
| Subtropical surface salinity maxima | ~37 psu, against ~35 mean | WOA climatology |
| Surface speed of the strongest current | 1-2 m/s | drifter climatology |

## The track

Each step is its own branch. Each moves the world, so each rewrites the
digest, says so in the commit, and runs the yardsticks against main's
failure list. Each reports `airEnv.currents` in the phase timings, before
and after.

```
O1 (fields kept) ─► M1 2D flow ─┬─► M2 thermocline ──┐
                                ├─► M3 heat it carries ┼─► M4 salt ─► M5 overturning
                                │                      │
                                └──────────────────────┴─► M6 seasons ─► M7 sea ice
```

### M1. The flow in two dimensions

Replace the row-by-row gyres with the barotropic vorticity equation for a
transport streamfunction ψ over the real coastline, on the air's cells:

    r ∇²ψ − A_H ∇⁴ψ + β ∂ψ/∂x = curl(τ) / ρ

Stommel's bottom friction `r` and Munk's lateral viscosity `A_H` close the
western boundary in two dimensions, so a current turns corners, runs round
islands and through straits. ψ is held at nought on the largest continent
and at an unknown constant on every other; each island's constant comes
from the island rule (Godfrey 1989): one extra solve per island and one
circulation condition. A ring of sea all the way round becomes a
circumpolar current by the same means, with its strength set by the
friction against its wind; that is what replaces `gyreOpen`. Near the
equator, β no longer vanishes in this form, so `gyreCalm` goes too.

The solver is conjugate gradients or multigrid on the air grid, in a fixed
order, independent of `Workers`. The velocity from ψ (over the depth the
layers of M2 give, or `gyreDepth` until then) adds to the Ekman drift,
as now. `gyreRows`, `cornerReach` and the zonal closure sweep are deleted.

Optional M1b: terra now has real sea-floor depths (the abyss and shelf
work). Solving with `f/H` in place of `f` steers the flow along the depth
contours, as the real deep currents are. Do it only if the reading shows
the shelves matter at the air grid's scale.

### M2. The thermocline

A reduced-gravity layer of warm water over a cold, still deep: its
thickness `h` is pumped down by the Ekman convergence in the subtropics,
pulled up in the subpolar gyres and along the equator, and tilted east to
west by the trades (Zebiak & Cane 1987's 1½-layer ocean, at its steady
state). What it gives:

- the temperature that upwelling brings up comes from the depth of the
  thermocline where it happens, not from `upwellContrast`: shallow off
  Peru, deep in the western Pacific;
- open-ocean upwelling, from Ekman pumping, which the coast-only
  upwelling now misses: the equatorial cold tongue and the subpolar gyres;
- the equatorial countercurrent and the undercurrent under it.

Done at its steady state, it adds one more elliptic solve per year's wind.

### M3. The heat the sea carries

The dynamical slab of Codron (2012), as the Generic-PCM carries it: a
mixed layer of 50 m over a deep layer of 150 m. The Ekman mass flux moves
surface water one way and the return flow at depth moves it back, M1's
gyres move both layers, and eddies are a horizontal diffusion. The mixed
layer's temperature is solved with these, as `seaLinks` now solves it,
with the diffusion added.

The change of substance: the ocean's heat convergence, averaged by latitude
over the sea, is handed to `ebm.go` as a flux into each band's sea column,
and the EBM's own diffusion is narrowed to the air's share. The latitude
profile and the currents are then one budget, and the ocean's meridional
heat transport becomes a reading with a yardstick (table above). The
warmth anomaly `Warm` becomes the solved temperature less its band's
mean, with no clamp: the clamp is a yardstick (±10 °C) instead.

This is the step most likely to move every climate yardstick: the EBM was
fitted with the ocean inside its diffusion (`docs/climate-next-steps.md`).
Re-fit `ebmDiffusion` to the air's share, with its source.

### M4. Salt

Surface salinity, carried with the temperature. Sources and sinks, all
already in terra:

- evaporation less rain over each sea cell, from `vapour.go`'s `Evap` and
  `Rain`;
- the rivers: each drainage basin's outlet on the sea pours its `Flow` in
  fresh;
- sea ice, once M7 has it: brine when it forms, fresh water when it melts.

Density from a linear equation of state, ρ(T, S). Readings: the
subtropical maxima, an enclosed sea in a dry climate saltier than the open
ocean (the Mediterranean), a sea that many rivers run into fresher (the
Baltic).

### M5. The overturning

Where the surface is densest and the winter mixed layer deepest, water
sinks. A per-ocean overturning in the manner of a Stommel (1961) box pair
for each ocean basin with a sinking site: its strength from the density
contrast between the sinking site and the basin's tropics, its heat carried
north in the upper layer and back at depth. The salt-advection feedback
gives the box two states; take the thermally driven one, from a fixed
start, so the world is deterministic.

Whether the overturning's heat is added to the western boundary current
(the Atlantic's extra warmth at sixty degrees) or only read is the owner's
decision, as in the currents plan. Do the reading first.

### M6. The sea's year

The air already runs four phases. Give the sea them too: the mixed layer's
temperature per phase, its depth deepened by winter storms and shoaled by
summer warming (a Kraus-Turner balance at its simplest), and the currents
of M1 to M3 under each phase's wind. The monsoon seas reverse (the Somali
current), seasonal upwelling switches on and off, and the air's `sst[k]`
in `air.go` reads the sea's own season instead of the latitude's mean plus
one year's anomaly. Four solves where there was one: measure before
committing to it, and keep the annual mean solve if the phases buy nothing
the yardsticks see.

### M7. Sea ice

Semtner's (1976) zero-layer ice on M6's mixed layer: it grows where the
water is at the freezing point and losing heat, melts where it is gaining,
drifts with the wind (about 2% of the wind's speed, turned to the right in
the north) and with the current, insulates the water under it, and sends
the albedo of ice back to the EBM. This replaces `SeaFreeze` against the
air's mean (`docs/climate-next-steps.md` step 2). Without M6 it can be done
on the annual solve, with the coldest phase's air, as a first cut.

## What it gives the features (O2, O3)

Each step adds kinds of things to point at and relations to draw:

- M1: `Class` equatorial and circumpolar; a throughflow between islands,
  with its transport; `Gyre` read off ψ's closed contours, which is exact
  where O2's heading-agreement finding is a guess.
- M2: open-ocean upwelling zones; the undercurrent.
- M3: `Carries` (current → band, PW of heat).
- M4: `Freshens` (basin → sea, from its outlet's flow); salty and fresh
  seas.
- M5: `Sinking` sites as features; `Overturns` (sinking site → basin).
- M7: `Freezes` (current → sea ice); `Keeps open` (warm current → sea).

## Budget

The weather today is seconds on a globe and `airEnv.currents` is 2.3 s
serial of it (`docs/perf/worklog.md`). M1 and M2 are an elliptic solve
each; M3 is the current sweep with a diffusion; M6 multiplies all of them by
up to four. Proposed budget, for the owner to set: the ocean no more than a
fifth of the weather's time on GlobeTerms, measured quiet.

## Owner decisions

1. The time budget above.
2. Whether M3's coupling into the EBM goes in, knowing it re-fits the
   latitude profile and moves every climate yardstick at once.
3. Whether M5's overturning may move the Gulf Stream's warmth.
4. Whether M6's seasons are worth four solves, once measured.
