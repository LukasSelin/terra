# World creation performance work log

Newest entry first. Each entry says what was measured, on what, and what it
means; changes say what they bought in benchstat terms. How to take the
measurements is in [README.md](README.md).

---

## 2026-10-07 - Land L4: fire and disturbance (#52)

**What this is.** On `claude/land-fire`, stacked on `claude/land-vegetation`
(L3, #85) at d3cad33. Fire inside L3's yearly dynamics, SPITFIRE-lite
(Thonicke et al. 2010; `internal/veg/fire.go`): fuel from the grass above
the ground and the litter the leaves drop (rotting by a Q10 of two); dryness
from L1's bucket under a herb's roots, phase by phase, read over five years
of a phase's water (±1/3 swing) and nothing under snow; ignitions from
lightning, the flashes going with the convective share of a phase's rain
(0.035 flashes/km² a mm, a fifth to the ground, 4% lighting a fire), a sixth
of each phase's falling on each neighbour's fuel; fires that run with the
wind for SPITFIRE's dryness-limited duration, as ellipses, and spread only
where the flammable cover is continuous (Archibald et al. 2009). Fire kills
by bark (savanna trees keep 0.9 of their cover, rainforest 0.4, boreal 0.15,
grass resprouts) and holds the trees' young down (gain × e^(-3·burned)).
Drought dieback for woody types under their establishment water (up to 5%/yr)
and windthrow as a mean rate from synoptic.go's storm rules
(`atmos.Env.Throw`: cyclones within reach of summer sea over 26.5 °C,
gales at 30-70°). A new map's vegetation is laid as the last glacial's
drier year (0.75 of the rain) left it and then spun under today's, so
which of a savanna and a forest a bistable place has is its history's.
`Grid.Burned` is new; the game API is unchanged, and the tuned valley's
woods rule is untouched.

**Bytes a tile.** 38: L3's 36 and two for the burned share (65535ths).

**The digest.** Rewritten, as meant: valley 45a8959a1ca0468b, ancient
0254d53d801725ba, globe128 4fc693b3a16ca85c.

**The heap.** Budget rewritten: valley 10.44 MiB in 1326 allocations (10.42
in 1323), ancient 56.87 MiB in 8807 (56.85 in 8802), globe128 399.2 MiB in
29466 (399.07 in 29468). The full globe allocates 11.04 GB, as before.

**Time.** `TERRA_PHASES=1`, `NewLand/globe`, the base at d3cad33 in its own
copy then this branch, the machine shared with other sessions (noise of
±10 s on Generate): Generate 70.7-83.2 s on the base, 79.1-81.8 here;
growVegetation 0.26-0.29 s against 1.6-1.7 s at first. Two thirds of that
was the storms' throw, a 600 km window searched over every cell; a chamfer
sweep (Borgefors 1986) made it 20 ms, and growVegetation now reads 0.76-0.78
s laid fresh (in a test on globe 1: two spins, glacial and today's, where L3
had one) and 0.28 s for an age's 25 years. The valley's making on the
budget's four workers: 194 ms -> 392 ms, under load. `scripts/perf.sh check`
was not run: the machine was never quiet.

**What it reads** (GlobeTerms seed 1):
- Burned a year: savanna 0.281 (GFED 0.2-0.4), boreal forest 0.0076
  (0.005-0.01), tropical rainforest 0.0000. All the land 0.079, 11.8 Mkm²
  scaled to the earth's (GFED ~4: temperate grassland 0.12 and shrubland 0.08
  burn too much).
- Tropical tree cover at 1000-2500 mm, by tenths: 0.29 0.08 0.10 0.07 0.02
  0.01 0.00 0.02 0.33 0.08; Sarle's bimodality 0.81 (5/9 uniform). 11% of
  that land holds either state by where it starts, and two thirds of it is
  savanna as laid.
- Vegetation carbon 453 Gt C (510 on L3; band 450-650). Shrubland 0.236 of
  the land (0.265). Temperate broadleaf LAI 2.96, conifer 2.90 (3.0, 2.9).

**Yardsticks.** Base and branch with #80's interval rule applied and not
committed: the same four failures (midlatitude/subtropical rain, rain 2x/1x,
Aridisols, Gelisols). Every river and relief interval is identical to the
digit but the advisory discharge exceedance (0.442 -> 0.449).

---

## 2026-10-07 - Land L3: vegetation as a state (#51)

**What this is.** On `claude/land-vegetation`, stacked on `claude/land-snow`
(L2, #81) at 3d00f2b. Nine plant functional types (tropical evergreen and
raingreen, temperate broadleaf and needleleaf, boreal needleleaf, C3 and C4
grass, shrub, tundra) on every land tile, each with its cover, its carbon and
its leaf area (`internal/veg`, `vegetation.go`). Each type's bioclimatic
limits are LPJ's (Sitch et al. 2003, table 3) and BIOME1's α limits
(Prentice et al. 1992); its leaf area is BIOME4's (Kaplan et al. 2003): the
one that leaves it the most to grow by, light by Beer's law against leaf
upkeep and turnover, bounded phase by phase by the water L1's bucket gives
at a herb's root depth (1 m) or a woody plant's (2 m), with L2's snow
shading the short types. BIOME4's ranking gives the start; LPJ-style
establishment, growth, mortality (age, starving, frost, drought) and crowding
then run to a steady state (`veg.Spin`, 30 years a year at a time, then five
to a step). `Erode` runs it on for the age and, under the climate's rules,
turns a wood whose trees have died back under a fifth of the ground to open
ground (`dieBackWoods`). `WoodsAt` under the climate's rules is the trees'
share of the canopy's room, so `Forest`/`Grass`, `HoldsWood`, `SetGrowth`,
`SeedTakes` and the Wood and Wild stocks are read as before. L1's root depth
reads the vegetation's woody cover; the history still reads the dryness, so
**the history stage is bit-identical** (owner decision 3: present-day
vegetation only). The cover stage lays the vegetation, runs every tile's
bucket again under its roots (`rewater`) and re-pools the rivers on it.
`BiomeAt` names a tile's biome off its state; cmd/overview's biome map reads
it, and the Köppen map stays its own. pedogenesis's forest cover reads the
broadleaf share of the trees' carbon.

**Bytes a tile.** 36: a byte a type of cover (255ths), two of carbon (g/m²),
a byte of leaf area (twentieths). 18.9 MB on the 1024×512 globe. Plus the air
cells' PET shares by phase, kept from rainOn (a few kB).

**The digest.** Rewritten, as meant: valley d759fbc9581ccfaf, ancient
52b6ef24ae5e3a8b, globe128 6804c8fe58f69f22.

**The heap.** Budget rewritten: valley 10.42 MiB in 1323 allocations (10.29
in 1310), ancient 56.85 MiB in 8802 (56.72 in 8787), globe128 399.07 MiB in
29468 (398.72 in 29417). The full globe allocates 11.04 GB against 11.02.

**Time.** `TERRA_PHASES=1`, `NewLand/globe`, two runs each, the base at
3d00f2b in its own worktree and then this branch, the machine otherwise
quiet: Generate 62.8-63.7 s on the base and 59.2-62.5 here (noise);
stage.ground 50.2-51.6 against 47.6-49.4 (the same history); stage.cover
0.40-0.45 s against 0.85-0.91: growVegetation 0.24-0.25 s, rewater 0.02 s,
and the re-pooling the rest. On the valley growVegetation was 0.23 s of a
0.36 s making until the spin took strides of five years once the first
thirty were done; it is 0.08 s now (budget valley 96 ms -> 194 ms on the
budget's four workers). `scripts/perf.sh check` was not run.

**What it reads** (GlobeTerms seed 1; all new yardsticks, all in band):
- Vegetation carbon 510 Gt C scaled to the earth's 148.9 Mkm² of land
  (IPCC AR6: 450-650). Small globes 1-2: 319.
- Leaf area index by biome: tropical rainforest 4.07 (MODIS 4.5-6; band 4-7),
  boreal forest 2.76 (2-4), temperate grassland 1.26 (0.5-2.5), deserts 0.16
  (< 0.5).
- Land in its biome's Whittaker (1975) envelope, area-weighted: 0.847.
  Biomes by area: shrubland 0.265, tropical seasonal forest 0.170, tundra
  0.110, temperate grassland 0.108, boreal forest 0.106, savanna 0.092,
  temperate broadleaf 0.045, tropical rainforest 0.043, temperate conifer
  0.029, hot desert 0.026, cold desert 0.006. Inside their envelopes: boreal
  1.00, deserts 1.00, savanna 0.94, shrubland 0.94, temperate grassland 0.86,
  tundra 0.85, tropical rainforest 0.72, tropical seasonal 0.68, temperate
  broadleaf 0.57, temperate conifer 0.54.
- Woods die back: small globe 1 on a third of its rain for five ages, tree
  cover 2237 tiles' worth to 366, and 2169 of its 2652 woods turned to open
  ground (`TestWoodsDieBackWhereTheGroundStopsSuitingThem`).
- The existing woods yardsticks: forest share of arid land 0; dry over
  humid 0.047 -> 0.247 (band < 0.5); wooded 82.6% of humid ground, 11.0% of
  dry, 0.05% of desert.

The yardsticks, `TestRealNumbers|TestTheRealWorld`: the failure list is
L2's eight, the same eight (small-globe concavity 0.293, concavity 2x-1x
0.207, C1 0.0731, ridge-valley wavelength 400 m, midlatitude over
subtropical rain 0.963, mean rain 2x over 1x 1.250, Aridisols 0.084 ->
0.081, Gelisols 0.113 -> 0.113). With #80's seeded readings
(`spread_test.go` and its test changes applied for the run, not committed)
both read the same four failures (midlatitude rain, rain 2x/1x, Aridisols,
Gelisols), and every seeded river and relief reading is the base's to the
digit, interval and all: the histories are the base's, and the cover stage
moves no ground.

---

## 2026-10-07 - Land L2: snowpack and glaciers on today's map (#50)

**What this is.** On `claude/land-snow`, stacked on `claude/land-soil-water`
(L1, #74) at 104d710. A phase's precipitation falls as snow on its cold
days and lies, and melts by degree-days into L1's bucket in the fortnight it
melts in (`internal/atmos/snow.go`, `atmos.BucketCold`). Each of a phase's
six steps reads the year's sine at its own place, with the days scattered
normally about it by 3.5 °C: the snow's share is the mean of a ramp from all
snow at 0 °C to all rain at 2 °C (Jennings et al. 2018), the melt 3 mm a
degree-day (Hock 2003), both exact for the normal scatter and read off a
table. The air takes its PET off the snow in proportion to the ground the
snow covers, tanh(SWE/10 mm) (Roesch et al. 2001), and off the soil for the
rest. Snow that outlasts the year is a glacier: its balance is what the
year lays down less what it melts and the air takes, the surplus goes to
the rivers with the melt, and `Barren` is that balance over nothing, in
place of Ohmura's line. The air cells' buckets in the vapour budget take
the same snow. `Grid.SnowWater`, `SnowCover`, `MeltIn` and `IceBalance`
keep it, 52 bytes a tile. No ice flow (G8).

**The digest.** Rewritten, as meant: valley 7e92ba58566301c4, ancient
1fbbf274d45930f3, globe128 3194a451a139d1f9.

**The heap.** Budget rewritten: valley 10.3 MiB in 1310 allocations (10.1
in 1308), ancient 56.7 MiB in 8787 (56.6 in 8791), globe128 398.7 MiB in
29417 (397.8 in 29092): the four phases' snow, cover and melt and the
year's ice, kept per tile. The full globe allocates 11.02 GB against 10.99.

**Time.** `TERRA_PHASES=1`, `NewLand/globe`, three runs each, under other
sessions' load: Generate 54.8-56.2 s on 104d710 and 54.8-60.5 here; rainOn
11.9-12.2 against 13.2-13.9 (+13 %); weather 14.7-15.2 against 16.0-17.1;
stage.ground 43.1-45.4 against 43.7-47.8. The first cut doubled rainOn
(25 s): the exponentials and error functions of every step, the snow spun
two years from the spring, the PET-less winter's tanh and math.Max's call.
A table at 1/64 of a degree, a year run from the end of the summer (most
packs are steady in the one year read), the cover only where it is read or
the air takes something, and the builtin max and min brought it here.
`scripts/perf.sh check` was not run on a quiet machine.

**What it reads.**
- The snow's equilibrium line is Ohmura's: the precipitation at which a
  year's snow just lasts is 0.74-1.35 of Ohmura, Kasser and Funk's (1992)
  over summers of 0-6 °C where the year swings 10 °C or more; a maritime
  year of 6 °C wants up to twice their snow at a 6 °C summer
  (`TestSnowLineIsOhmuras`). Lifting each land tile of small globes 1-2 till
  its snow outlasts the year puts the ice's line 188 m over Ohmura's on the
  median, quartiles 88 and 338 m (`TestTheIceIsOhmuras`).
- Snow-fed land (melt half its runoff or more): 0.973 of the globe's and
  0.902 of the small globes' peaks in its hemisphere's spring or summer
  (Barnett et al. 2005; new yardstick, 0.8-1). A unit continental year,
  mean -2 °C, swing 16, 600 mm even: runoff 0/323/36/32 mm by phase
  (winter, spring, summer, autumn) with snow, 132/147/37/74 without.
- The north's land snow-covered, by phase (winter, spring, summer,
  autumn): globe 0.407, 0.276, 0.041, 0.210; small globes 0.430, 0.259,
  0.030, 0.193. The earth's: some 46 of 100 million km2 in February and
  2-3 in August (Robinson & Frei; Rutgers Global Snow Lab), 0.46 and 0.03.
  By band on the globe, 40-50 N is 0.73 covered in the winter and 50-60 N
  0.98.
- Ice: none on any globe, by the balance or by Ohmura's line, as on
  104d710 (the earth: a tenth of the land with the ice sheets, a two
  hundredth without). The energy balance's poles are -10.5 °C and no
  globe's ground stands high enough; a gap of the air and the rock, logged.
- Where the year freezes, the land's runoff over what Budyko-Fu leaves of
  its rain: small globes 1.098 without snow, 1.042 with it; globe 1.035 and
  0.995; valley 0.988 and 0.972. Half of L1's cold surplus was the winter's
  rain filling a bucket the air took nothing from.

The yardsticks, `TestRealNumbers|TestTheRealWorld`, 104d710 against this:

| yardstick | cfdd360 | L1 104d710 | L2 | real |
|---|---|---|---|---|
| Flint's law fit R2, small globe | 0.960 pass | 0.832 fail | **0.957 pass** | 0.85-1 |
| channel concavity, 2x less 1x | 0.085 pass | 0.186 fail | **0.207 fail** | -0.1-0.1 |
| land relief intermittency C1 | 0.0867 pass | 0.0672 fail | **0.0731 fail** | 0.08-0.18 |
| channel concavity, small globe | 0.3499 fail | 0.245 fail | 0.293 fail | 0.35-0.6 |
| drainage area exceedance, small globe | 0.500 fail | 0.468 fail | 0.459 pass | 0.39-0.46 |
| ridge-valley wavelength, small globe | 400 m fail | 178 m pass | 400 m fail | 24-224 |
| midlatitude over subtropical rain, globe | 0.959 fail | 0.946 fail | 0.963 fail | 1.1-2 |
| mean land rain, 2x over 1x | 1.260 fail | 1.251 fail | 1.250 fail | 0.85-1.15 |
| land share of Aridisols | 0.065 fail | 0.082 fail | 0.084 fail | 0.09-0.15 |
| land share of Gelisols | 0.117 fail | 0.121 fail | 0.113 fail | 0.06-0.11 |
| Hack exponent, small globe | - | 0.576 pass | 0.548 pass | 0.54-0.6 |
| seasonal land peaking in or after its wet season | - | 0.998 pass | 0.898 pass | 0.8-1 |
| land evaporation over land rain, globe | - | 0.584 pass | 0.593 pass | 0.55-0.70 |
| Budyko-Fu ω fitted to the land, globe | - | 2.34 pass | 2.46 pass | 1.8-3.6 |
| snow-fed land peaking in spring or early summer (new) | - | - | 0.973 pass | 0.8-1 |

Of L1's three new failures, Flint's fit is back (0.957 against cfdd360's
0.960); C1 is two fifths of the way back; the concavity's difference between
the scales moved further off. The one new failure, the ridge-valley
wavelength, is the reading cfdd360 had: the spectrum's peak over its power
law is an argmax over 30 bins, and 400 m is the window's longest admissible
wavelength, where it sat before L1 moved it. The seasonal land's share
peaking in or after its wet season falls from 0.998 to 0.898 because
snowmelt rivers whose wet season is the autumn or the winter peak in the
spring, which Dettinger & Diaz's own caveat ("snowmelt rivers later") says
they do.

**The tests it moved.** The ancient chain reads tile 1871, since lime was
laid over 1791's pluton. The tide's flats are read on small globe 8 (0, 0, 0,
0, 0, 0, 3 and 6 flats on globes 1-8). `TestSeasonalRiversRunAfterTheRain`
reads only tiles no snow lies on, since a mild winter's snow sends its water
to the spring and the phases either side of the wet one are no longer
alike; its unfrozen test also took the hemisphere's sign off the swing,
which counted the south's tiles as unfrozen by their mean plus their swing.
After over before reads 1.76.

---

## 2026-10-07 - Land L1: soil moisture and the seasons of water (#49)

**What this is.** On `claude/land-soil-water`, stacked on
`claude/air-calendar` (A1, #71) at cfdd360. Each land tile's runoff is what
a bucket (Manabe 1969) sheds through the air's four phases, in place of the
year's P - Fu(P, PET). The bucket fills with the phase's rain, gives the air
its PET while over 0.8 full and in proportion under that, and sheds the
rain times its fill squared (HBV's soil routine, Bergström 1992), all of it
when full: unseasonal, it is Fu's curve at ω = 2.6 to 2.6 % of the rain
(`internal/atmos/bucket.go`). Its size is the soil's plant-available water
(Saxton & Rawls 2006, from sand and clay) over the roots' depth, 1 m under
open ground and 2 m under forest, with the climate's dryness standing in
for the cover until L3, plus 5 % of the rock the roots reach below the soil
(`atmos.Hold`, `soilwater.go`). The vapour budget's land evaporation is the
same bucket's, phase by phase, on the air cells. Each phase is six implicit
steps; the year read follows two years of spin and an Aitken jump, the
steady year to 1e-4 of the rain. `Grid.RainIn`, `SoilWater`, `RunoffIn`
and `SoilHold` keep it, 52 bytes a tile. The phases' PET share read the
summer's sun for the autumn (`dayOf[min(k, 2)]` from an older order of the
phases); it reads each phase's own.

**The digest.** Rewritten, as meant: valley 370bf276e6a662b5, ancient
183a13b2920d004e, globe128 cfb3d4761bff9aca.

**The heap.** Budget rewritten: valley 10.1 MiB in 1308 allocations
(10.0 in 1330), ancient 56.6 MiB in 8791 (56.1 in 8700), globe128 397.8
MiB in 29092 (393.9 in 29301): the four phases' rain, water and runoff
and the bucket, kept per tile. The full globe allocates 10.99 GB against
11.22.

**Time.** `TERRA_PHASES=1`, `NewLand/globe`, three runs each, under other
sessions' load: Generate 54.1 s on cfdd360 and 54.1 here; stage.ground
42.2 against 43.4; weather 14.1 against 14.5; rainOn 11.09 against 11.51
(+4 %: 72 steps of a bucket a tile, and the cells' buckets each round of
the recycling); windsFor 2.97 against 2.94. `scripts/perf.sh check` was not
run on a quiet machine.

**What it reads.** The globe's land gives the air 0.584 of its rain (Oki &
Kanae 0.59); Fu's ω fitted to its tiles is 2.34; of its land with half its
rain in one phase, 0.998 sheds most in that phase or the next. On the small
globes the phase after the wet one sheds 1.12 times the phase before it,
on the same rain and sun, where the year never freezes. The valley: a
bucket of 100 mm holding 69 on the mean, evaporation 0.528 of the rain (0.524
before), ω 2.63. The small globes' runoff is about 12 % over Fu's, evenly
with height: half their land has a month under freezing, and with no snow
yet (L2) the winter's rain fills a bucket the air takes nothing from.

The yardsticks, `TestRealNumbers|TestTheRealWorld`, cfdd360 against this:

| yardstick | cfdd360 | L1 | real |
|---|---|---|---|
| channel concavity, small globe | 0.3499 fail | 0.2450 fail | 0.35-0.6 |
| Flint's law fit R2, small globe | 0.960 pass | 0.832 fail | 0.85-1 |
| channel concavity, 2x less 1x | 0.085 pass | 0.186 fail | -0.1-0.1 |
| land relief intermittency C1 | 0.0867 pass | 0.0672 fail | 0.08-0.18 |
| midlatitude over subtropical rain, globe | 0.959 fail | 0.946 fail | 1.1-2 |
| mean land rain, 2x over 1x | 1.260 fail | 1.251 fail | 0.85-1.15 |
| land share of Aridisols | 0.065 fail | 0.082 fail | 0.09-0.15 |
| land share of Gelisols | 0.117 fail | 0.121 fail | 0.06-0.11 |
| drainage area exceedance, small globe | 0.500 fail | 0.468 fail | 0.39-0.46 |
| discharge exceedance, small globe | 0.480 fail | 0.435 pass | 0.40-0.46 |
| ridge-valley wavelength, small globe | 400 m fail | 178 m pass | 24-224 |
| hypsometric integral, 2x less 1x | -0.070 fail | -0.011 pass | -0.05-0.05 |
| land evaporation over land rain, globe (new) | - | 0.584 pass | 0.55-0.70 |
| Budyko-Fu ω fitted to the land, globe (new) | - | 2.34 pass | 1.8-3.6 |
| seasonal land peaking in or after its wet season (new) | - | 0.998 pass | 0.8-1 |

Three new failures, none tuned away; three old ones pass. The concavity and
Flint's fit are one pooled fit over eight small globes whose own readings
scatter from -0.03 to 0.95 (cfdd360: 0.15 to 0.96). Two controls on this
branch: the bucket at one size everywhere reads 0.243 and R2 0.71, so the
soil's depth is not it; the tiles' runoff put back to Fu's, with the rest
of the change kept, reads 0.340 and 0.88. So it is the bucket's runoff,
which is some 12 % over Fu's on the small globes and no different with
height: the runoff moved every history, and the pooled fit is as noisy as
its globes. The concavity's difference between scales and C1 move with it.

**The tests it moved.** The ancient chain reads tile 1791, since lime was
laid over 1307's pluton. The tree line's share on valley seed 1 is read
against the land that suits trees at all, 0.160 of it, where it read
against eight tenths of woodsShare (0.161 on cfdd360, 0.160 here):
readHolds already says such a map takes what there is.

---

## 2026-10-07 - Air A1: one calendar and one seasonal swing (#33)

**What this is.** On `claude/air-calendar`, stacked on `claude/air-forcings`
(A0, #65) at 1b4ef76. The energy balance steps the clock's year: 360 days
from the spring equinox, each a 360th of a real year (`secondsPerYear` is
unchanged), so its lags are calendar days and the daily sun's declination
crosses the equator on tick zero. Today's forcing is the 1950 orbit
(`OrbitBefore(0)`), not FAO-56's rounding of it. The wind's thermal swing
and the evaporation's seasonal swing read the balance's sea and land swing
at each row's latitude, between in proportion to the cell's continentality,
as `SwingAt` does for the ground; `swingSea`/`swingLand` (0.35 and 1.6 of
`Swing`) and the cap at Temperate's are gone. `PetTable` reads the
forcing's sun, the swing and the lag, on three continentalities (sea,
middle, continent) that `PetAt` reads between; the seasonal share of the
evaporation reads the forcing's sun. The lags read the forcing
(`LagUnder`). A valley's evaporation reads its ground's year (`Swing` at
`ContMiddling`); its wind reads the balance at Temperate. `Fu` is held
under min(P, PET): with the new PET a runoff a rounding under nothing on one
tile of the doubled small globe sent its ground to NaN.

**The digest.** Rewritten, as meant: valley abdcc91960ba951d, ancient
2826cb480d2ca6e1, globe128 d74e8635d01b38cd.

**The heap.** `TestWorldCreationBudget` passes unmoved within its
tolerance: valley 10.0 MiB in 1296 allocations (budget 1330: a valley's rows
share one evaporation table now), ancient 56.1 MiB in 8729 (8700), globe128
393.9 MiB in 29330 (29301). No budget diff is committed. The full globe
allocates 11.22 GB against 10.91 (+3%), which is the moved world's lakes and
ages and not the tables, which globe128 would show.

**Time.** `TERRA_PHASES=1`, `NewLand/globe`, three runs each, with other
sessions on the machine: Generate 49.3/49.7/49.4 s on 1b4ef76 against
49.8/52.7/50.6 here; history 37.5 against 38.1 (mean); weather 12.7
against 13.3; rainOn 9.9 against 10.5 (+5%: two tables read a tile and the
tile's continentality sampled for its evaporation); windsFor 2.70 against
2.79; airEnv.currents 1.66 against 1.70. Under load; `scripts/perf.sh check`
was not run on a quiet machine.

**What it reads.** The balance on the calendar: land lag at Temperate 29.4
calendar days (30.1 of 365.25 before, 29.7 in the calendar's), sea 87.9
(89.5); swings at 45 degrees 16.2 land and 3.3 sea, as before; global mean
16.24 C, as before. The air's swing, sea/land, against what it was: 10
degrees 0.5/2.4 (0.9/4.3), 20 degrees 1.2/6.0 (1.9/8.5), 30 2.0/9.8
(2.8/12.8), 45 3.3/16.2 (4.2/19.2), 55 5.0/21.5 (4.2/19.2), 65 10.6/17.3
(4.2/19.2), 75 14.9/16.4 (4.2/19.2): smaller in the tropics, larger over
the high seas where the ice comes and goes. The valley's land rain falls
some seven parts in a hundred (seed 1: 1191 to 1111 mm), its evaporation
from 774 to 727 mm; the small globes' mean land rain 594 to 573 mm.

The yardsticks, `TestRealNumbers|TestTheRealWorld`, 1b4ef76 against this:

| yardstick | 1b4ef76 | A1 | real |
|---|---|---|---|
| channel concavity, small globe | 0.2455 fail | 0.3499 fail | 0.35-0.6 |
| midlatitude over subtropical rain, globe | 0.904 fail | 0.959 fail | 1.1-2 |
| mean land rain, 2x over 1x | 1.246 fail | 1.260 fail | 0.85-1.15 |
| land share of Aridisols | 0.059 fail | 0.065 fail | 0.09-0.15 |
| land share of Gelisols | 0.130 fail | 0.117 fail | 0.06-0.11 |
| drainage area exceedance, small globe | 0.486 fail | 0.500 fail | 0.39-0.46 |
| discharge exceedance, small globe | 0.462 fail | 0.480 fail | 0.40-0.46 |
| hypsometric integral, small globe | 0.3055 fail | 0.3583 pass | 0.32-0.6 |
| hypsometric integral, 2x less 1x | +0.038 pass | -0.070 fail | -0.05-0.05 |
| ridge-valley wavelength, small globe | 133 m pass | 400 m fail | 24-224 |
| land relief intermittency C1 (gap K) | 0.075 gap | 0.087 closed | 0.08-0.18 |

Three new failures, none tuned away. The hypsometric integral is the mean
over the highest tile, globe by globe: the 1x globes' rose (0.31, 0.28,
0.33, 0.33 to 0.41, 0.30, 0.37, 0.37) and the 2x globes' fell (0.38, 0.38,
0.33, 0.31 to 0.34, 0.29, 0.29, 0.24) as their highest tiles moved, and the
difference crossed the band's lower edge. The ridge-valley wavelength is
the strongest residual peak of the pooled spectrum, a whole number of
windows: the three globes read 320, 55 and 145 m one by one (133, 107, 133
before), and pooled, the first one's long wave wins at k = 2, the longest
the window has. The C1 gap closed with nothing in the shaping changed; its
marker is off, and it sits near the floor.

**The tests it moved.** Fixtures and golden readings, each saying so where
it stands: the step lakes' dry window is 0.97-1.01 of the valley's rain and
the wet case reads 1.1 (`lake_test.go`); the ocean's warmth hash is taken
again; the flats are read on small globe 3, since globe 1 has none; the
ancient chain reads tile 1307, since lime was laid over 987's pluton; the
pole test reads both pole rows for wood and crops instead of one tile for
rock or ice, which was open ground on 1b4ef76 three and nine hundred tiles
along; the ice edge allows three of six poles on one row (61, 59, 64, 58,
58, 58 here). One is a real loss: the hot continent's summer wind at its
south coast is offshore, -0.8 m/s (0.5 before), because the balance's land
between five and fifteen degrees swings under four degrees where the air
swung up to six. The turn from winter to summer is still held; the onshore
summer is logged as a gap for A3's monsoon (#35).

---

## 2026-10-06 - Air A0: the forcings as variables (#32)

**What this is.** On `claude/air-forcings`, from `main` at 2c51bea. The
energy balance's sun, orbit and carbon are a `Forcing` on `Terms`
(`internal/atmos/forcing.go`); its outgoing longwave gains Myhre et al.'s
-5.35 ln(C/C₀); the daily sun reads the obliquity, the eccentricity and the
longitude of perihelion through FAO-56's form it always used; the
`sync.Once` is a `sync.Once` for today's and a `sync.Map` of once-solved
balances for any other; and `Forcing.OrbitBefore` gives Berger's (1978)
orbit of a time before 1950 (`internal/atmos/orbit.go`, the full 47 + 19 +
78 term series).

**The digest.** Unchanged, as meant: `TERRA_DIGEST=write` on 2c51bea left
`docs/perf/digest.json` as committed, and `TERRA_DIGEST=check` on the change
passes for the valley, the ancient valley and globe128. Today's forcing goes
through the daily sun to the bit (`TestTodaysForcingIsTheWrittenOne` holds
every half degree of latitude on every quarter day against the formula as it
was written), and the carbon term is ln 1 = 0 exactly.

**The heap.** `TestWorldCreationBudget` unmoved: valley 10.0 MiB in 1335
allocations, ancient 56.1 MiB in 8697, globe128 393.8 MiB in 29296, each
within a few allocations of its budget. No budget diff is committed.

**Time.** Nothing per tile: a globe's `MeanAt` and swing read the balance
through one comparison of the forcing with the zero one before the slot
they read before. A forcing other than today's costs one solve of the
balance, some 0.6 s, the first time a process asks for it. Timings from
`TestWorldCreationBudget` (globe128 1.98 s against 2.11 s budgeted) were
taken with other sessions running and are noise; `scripts/perf.sh check`
was not run on a quiet machine.

**What it reads.** Doubling the carbon (330 to 660 ppm) warms the
balance's global mean 1.89 degrees (16.24 to 18.13 C); halving it cools it
1.95. That is 3.71 W/m² over B = 2.09, 1.77 degrees, and a little ice
albedo: under the 2-4 the issue expected, because Budyko's B has no water
vapour or cloud feedback for the balance to add (A5's work). Tilt 22.1
against 24.5 degrees gives 65N's summer quarter 417 W/m² against 449.
`OrbitBefore` reads PMIP's orbits to their printed digits: 1950 at 0.016724,
23.446, 102.04; 21 ka at 0.018994, 22.949, 114.42; 6 ka at 0.018682, 24.105,
0.87.

---

## 2026-10-06 - Two short-tier tests that read the draw

**What this is.** On `claude/affectionate-roentgen-ccfb0d`, from `main` at
53eb8bf. Tests and the work log only. `go test -short` on 53eb8bf failed
two tests that 2a00ecb passed: `TestAHistoryLeavesAMapTheSettlementCanUse`
(seed 8, 310 tiles of water against 80) and
`TestAHistoryLeavesItsBedsInLayers` (49% of the map on more than one bed).

**Which change.** The fracture bend, 69d6e2e, alone: 2a00ecb passes both
and 69d6e2e fails both with the same messages. `claude/relief-intermittency`
changes no code (35f586d and ff2b847 are a test's messages and this log),
and 53eb8bf's tree is 9d271cb's but for those. The entry below names both as
the draw on the seed each reads; this is the measurement of whether they
are.

**The reading.** Both read the default valley run through sixteen epochs
(`AncientTerms`, which is `historyConfig(16)`), whose fractures went from
0.10 radians over a spacing to 0.30. `fractureBend` at 0.1 in the new code
makes the old valley tile for tile (two hundred seeds, every reading the
same), so on the valley the change is the bend and nothing else. Seeds
1-600, drawn valley and made:

| | before (0.1) | after (0.3) |
| --- | --- | --- |
| made valley's water, tiles, mean (drawn 175) | 169 | 174 |
| made valley's water, middle (drawn 166) | 158 | 158 |
| correlation of a made valley's water with its twin's | 0.03 | 0.09 |
| seeds over three times their twin's water | 17 | 19 |
| share on more than one bed, mean | 0.567 | 0.564 |
| seeds under a half | 85 | 86 |
| beds a tile, mean | 2.33 | 2.33 |
| seeds under two | 22 | 27 |
| steepest tenth over the twin's, mean, seeds 1-200 | 0.845 | 0.841 |
| ploughland over the twin's, mean, seeds 1-200 | 1.000 | 1.001 |

The made valley did not move on anything these tests read. Over seeds 1-200
the bend at 0.2 and at 0.45 reads the same within the seeds' spread (share
0.556 and 0.560, water 173 and 171 tiles).

**Why they tripped.** The water was asked of each seed against its drawn
twin, and the twins share a seed number and nothing else. The bar was three
times whatever that draw left - 72 tiles where the drawn map had 24, 2556
where it had 852 - and it tripped wherever a dry drawn map met a wet made
one: of the fifty dozens in seeds 1-600, 14 before the bend and 16 after.
Seeds 1-12 cleared it before by that much luck. The layers were asked of
seed 1 alone, of a share that runs 0.41-0.74 over the seeds with one in
seven under a half: seed 1 read 0.517 before and 0.489 after.

**What changed.** The bars stay; what they are asked of moves, as the
steepest tenth in the same test already is:

- The water is asked of the dozen together: the made valleys' water in all
  no more than twice the drawn ones'. A dozen reads 0.60 to 1.50 of its
  twins' over the hundred dozens before and after, seeds 1-12 1.07 (0.91
  before). The made valleys are, if anything, less often drowned: 3 and 4
  in 600 hold three times the drawn maps' middle, where 11 drawn maps do.
  One drowned valley is the ploughland check's, which is still per seed:
  water is not ploughed, and no made valley of the 1200 read under 0.85
  of its twin's ploughland (the bar is 0.8). Holding each made valley
  against the middling drawn map at three times still failed 4 and 6
  dozens in fifty, which is why the dozen's sum is read.
- The layers' share and depth are asked of the middle of the same dozen,
  at the same half and two beds; the per-tile check and the five kinds of
  rock are still asked of every valley. The middle of a dozen reads 0.50 to
  0.62 over the hundred dozens and 0.543 on seeds 1-12 (0.555 before); its
  depth 2.10 to 2.46, and 2.28 (2.24).
- The settlement test makes its valleys through `madeLand`, so the two tests
  share the dozen's histories: 6.7 s -> 2.0 s with them kept, and the layers
  test 0.5 s -> 1.0 s for twelve valleys and not one.

Still in the steepest tenth's comment: over thirty seeds the made valley
ran 1.42 times its twin's on the middle. Over seeds 1-200 it is 0.82 before
the bend and 0.83 after, so a made valley is no longer the steeper place;
the comment is left for whoever next reads that bar.

**Digest, budget, time.** `TERRA_DIGEST=write` on 53eb8bf leaves
`digest.json` as it is, and `TERRA_DIGEST=check` passes after: no world
moves. The budget and `scripts/perf.sh` read nothing this touches and are
not run. `go test -short` passes whole, 66 s (on 53eb8bf it failed these
two). The yardsticks fail what the fracture-bend entry leaves failing on
`main`, at its figures: channel concavity (0.246), drainage area (0.486) and
discharge (0.462) exceedance exponents and the hypsometric integral
(0.305), small globe; mean land rain 2x over 1x (1.25), midlatitude over
subtropical rain (0.90), Aridisols and Gelisols. 281 s.

---

## 2026-10-06 - The sea keeps its currents

**What this is.** On `claude/ocean-fields`, from `main` (53eb8bf):
workstream O1 of the ocean-currents plan. `(*Env).currents` worked out the
current, the upwelling and the water's temperature, then kept only the
clamped warmth: `seaLinks` wrote its weights over `cu`, `cv` and `rise`, and
`temp` was dropped. Now `Env` keeps `Cu`, `Cv`, `Rise` and `WaterTemp`
(°C, unclamped; `SeaTemp` was already the name of the storms' seasonal
reading) as `[]float32` on the air's cells, nil on a valley. `seaLinks` takes
its own `base`, `wa` and `wb`. `Grid` reads them per tile, as it reads
`SeaWarmth`: `SeaCurrent`, `Upwelling`, `SeaTemp`. `cmd/overview` draws
`sea-currents.png`: the current as streamlines (the wind's, drawn full length
at 0.5 m/s) over the water's warmth against its latitude, darkened where it
upwells.

**The world.** Unchanged. `TERRA_DIGEST=write` on 53eb8bf left
`docs/perf/digest.json` as committed, and `TERRA_DIGEST=check` passes after,
with `TERRA_HISTORIES=off` too. `TestKeepingTheCurrentsLeavesTheWarmthAsItWas`
pins an FNV hash of `Warm` and `Coast` on `twoOceans`, taken on 53eb8bf
(0xafdf8947c074a07c), and checks that the kept temperature over its
latitude's mean, clamped to ±10, is `Warm` to within float32 rounding.
The yardsticks were not rerun because every world is bit for bit as it was.

**The heap.** globe128 allocates 405.8 -> 413.0 MB (+1.78%, over the 1%
slack, so the budget is rewritten), with 152 more allocations. That is 40 B
an air cell each time the winds are made: the three float64 slices
`seaLinks` no longer borrows and the four float32 fields kept. The valley
and the ancient valley make no currents, and their budget entries are left
as they were. Letting `seaLinks` keep borrowing `cu`, `cv` and `rise`
after they are narrowed to float32 would save the 24 B of scratch. It was
left undone so that the solve writes over nothing it is handed.

**Timing.** `scripts/perf.sh check` fails against the 2026-09-16 07:18
baseline: valley +25%, ancient +13%, globe256 +30%, with spreads of ±10-24%.
The valleys make no currents, so this is the machine's load (and main's drift
since then), not the change. Interleaved, main's test binary and this one
turn and turn about on globe256, six runs each, on the same loaded machine:

| globe256 | main | ocean-fields | |
| --- | --- | --- | --- |
| sec/op | 4.926 ± 38% | 4.775 ± 17% | ~ (p=0.818) |
| B/op | 1.521 Gi | 1.546 Gi | +1.68% (p=0.002) |
| allocs/op | 192.5k | 192.6k | ~ (p=0.699) |

No time to be seen; the bytes are the 40 B an air cell above. A quiet-machine
`check` is still owed before merging.

---

## 2026-10-06 - A reading is kept for its own grid

**What this is.** Test code only. The yardsticks' memo, `remember` in
`realism_test.go`, kept a reading under the address of the grid it was read
off (`%p`), and for most readings only the first grid's address and how many
there were. A grid the registry keeps (`yardWorld`) lives the whole run and
is never mistaken for another; a map drawn in a test and let go can have its
address given to the next map drawn, which was then served the forgotten
map's reading. Drawn in a loop of 32, with a collection between, 14 maps
were read as an earlier map's land; on `claude/fracture-bend`, `cornerLock`
read 0.046 for six of eight Brownian maps until they were kept alive.

A reading is now kept under its name and a serial for every grid it is read
off, each grid numbered by a `weak.Pointer` to it: equal only for the one
grid, even once it has gone and another lies where it lay, and not keeping a
drawn map alive for the rest of the run as a `*Grid` key would.
`TestAReadingIsKeptForItsOwnGrid` draws the 32 maps; on the address key it
fails at the second. The `runtime.KeepAlive(kept)` that
`claude/fracture-bend` put in `TestTheShapeMeasuresReadDrawnShapes` for it
is taken out.

It was failing the short tier on main, and keeping maps alive did not mend
it. Run with the rest of the tier at 2a00ecb,
`TestTheShapeMeasuresReadDrawnShapes` read the spectrum of its Brownian
relief of H 0.8 as 1.964 - the H 0.5 relief's, 1.96379, drawn and let go the
turn before - against 2.6 within 0.2; run alone, it was given another
address and read its own 2.591. At 53eb8bf, with the three-tenths maps kept,
it read the H 0.8 relief's continents as gathering 0.479 at right angles,
against its own 0.067: a test can keep its own maps, but not the ones the
tests before it let go. Eight Brownian maps drawn in a loop and let go read
corners of 0.059 and then one value seven times over, 0.0254 run alone and
0.0334 in the tier; they now read 0.020 to 0.077.
It passes now, alone and in the tier.

**What it moved.** No world. No non-test code changed, so the heap budget and
`TERRA_DIGEST=check` stand as they were. Against main at 53eb8bf, the twelve
three-globe shape readings and the twenty-two readings of the drawn shapes
(each map kept alive on main, so that main reads its own) are the same to the
last digit, and the short tier passes, skips and fails the same tests with
the same messages, but for `TestTheShapeMeasuresReadDrawnShapes`, which now
passes. `TestAHistoryLeavesAMapTheSettlementCanUse` and
`TestAHistoryLeavesItsBedsInLayers` fail the tier on main as they do here.

---

## 2026-10-06 - The fractures bend by the plate, and the continents are less square

**What this is.** On `claude/fracture-bend`, from `main`. The globes'
continents came out cut in rectangles: long straight coasts meeting at
corners near a right angle. How far a fracture's bearing wanders was
`fractureCreep`, 0.05 radians a tile, which over the spacing between plate
middles is 0.14 on a small globe (sixteen plates, 45 tiles apart), 0.23 on
a full one (32 plates, 128 tiles apart) and 0.10 on the ancient valley. It
is now `fractureBend`, the spread over a spacing, which is what
`fractureLong` and `fractureShort` are quoted in, at 0.3: the full globe's
faults bend a third more than they did and every map's bend alike.

**The reading.** `cornerLock`, a new yardstick, "right angles of the
continents' coasts": for each continent, |<e^{4iθ}>| of the directions its
eased coast faces, which is high for a rectangle whichever way it is turned
and low for anything with its corners at other angles, averaged over the
continents by size within 60 degrees of the equator. Off drawn shapes, a
square and a diamond read 1, a hexagon and a disc nothing, and Brownian
coasts 0.02-0.07 (eight of them; a continent's few long runs of rough coast
read that much by chance). No figure for the earth's has been read this
way, so the band, 0-0.15, is not a measured one.

| | globe 1 | globe 2 | globe 3 | three |
| --- | --- | --- | --- | --- |
| before | 0.239 | 0.145 | 0.181 | 0.181 |
| after | 0.208 | 0.053 | 0.113 | **0.125** |

On the continental crust at the end of the history, over five globes, 0.176
-> 0.128.

**The search.** Bends in radians over a spacing, and the fracture's wall;
the full globes' continents (crust at the end of the history), and the plate
tests on the small globes, which are where the straight walls of the
2026-09-19 entry are held:

| bend, wall | continents, globes 1/2/3 | straight wall (1.05 and over) | plate area exponent, three | plate tests |
| --- | --- | --- | --- | --- |
| as made (0.23, 6.5) | 0.29 / 0.17 / 0.20 | 1.167 | 0.341 | pass |
| 0.23, 6.5 | 0.19 / 0.17 / 0.16 | 1.215 | 0.253 | pass |
| **0.3, 6.5** | 0.18 / 0.07 / 0.16 | 1.159 | 0.383 | pass |
| 0.35, 6.5 | 0.05 / 0.09 / 0.15 | 1.094 | 0.217 | a plate of 0.38 of the world |
| 0.4, 6.5 | 0.14 / 0.08 / 0.12 | 0.961 | 0.206 | wall |
| 0.6, 6.5 | 0.18 / 0.17 / 0.16 | 0.957 | 0.304 | wall; a plate of 0.36 |
| 0.9, 6.5 | 0.08 / 0.11 / 0.15 | 0.924 | 0.280 | wall |
| 0.6, 5 | 0.12 / 0.11 / 0.12 | 1.027 | 0.476 | wall; a plate of 0.35 |
| 0.9, 5 | 0.07 / 0.17 / 0.10 | 0.949 | 0.319 | wall |

The 0.23 row bends the full globe as it was bent and still reads
differently: one globe's continents move by a tenth with any change to where
the fractures run, so only what holds over several is a reading. Past 0.35 the
straight walls go before the right angles do: a straight wall is what a
right angle is made of, and the right angles are not the fractures' own -
three sets a sixth of a turn apart make sixty degrees - but grow over the
history. 0.3 is the most bend that keeps the walls, over eight small globes
1.080 against 1.182 before.

Kept a rate a tile and raised by the same third (0.065), the full globe gets
the same bend and the small worlds a third more and not twice as much; it
failed fourteen tests and yardsticks to this one's thirteen, among them the
right angles themselves (0.158).

**The plate area exponent** is read over eight small globes and not three.
The suite makes small globes 1-16 for its rivers, so it costs nothing, and
over three it read anywhere from 0.21 to 0.48 across the settings above;
over eight it is 0.354 before (out of 0.15-0.35 by a hair, where three read
0.341) and 0.296 after.

**What else moved.** Every made world's plates are different plates. The
whole suite against `main` with the shelves of the two entries below:

- In now: ridge-valley wavelength, small globe, 533 -> 133 m;
  `TestAGlobeHasASeaItsRiversReach`, the globe 0.67 water; the plate area
  exponent, over eight; the floor's grid lock off the coasts, 0.084 ->
  0.015, its marker off, since the coasts lean less to the diagonals (0.085
  -> 0.040) and the floor is laid at the true distance from them; and the
  land share, furthest of three globes, 0.449 -> 0.377 (0.33, 0.38, 0.37).
  That last is the draw's luck and not a fix - the crust is still drawn at
  45% continent within crustSlack - but a gap that closes takes its marker
  off.
- Out now: channel concavity, small globe, 0.353 -> 0.246; drainage area
  exceedance exponent, small globe, 0.421 -> 0.486; mean land rain, 2x over
  1x, 1.13 -> 1.25; and shelf width, quiet margins over active, 1.88 ->
  1.24 (2.00, 0.94, 1.23), which takes its marker back: most coasts with a
  seam within 150 km are quiet margins as laid, so which side of 1.8 it
  reads is where the seams fall.
- Three tests that read one seed or one storm: the ancient valley stands
  49% on more than one bed (`TestAHistoryLeavesItsBedsInLayers`, 50%); made
  valley 8 has 310 tiles of water against its drawn twin's 80
  (`TestAHistoryLeavesAMapTheSettlementCanUse`, three times); and a storm on
  small globe 3 deepens to 866 hPa (`TestTheWeatherChangesFromDayToDay`,
  870, Typhoon Tip). Each is the draw on the seed it reads.
- Still out as on `main`: midlatitude over subtropical rain, Aridisols,
  Gelisols, the small globe's hypsometric integral (0.305) and discharge
  exponent (0.462).
- The other K gaps: C1 0.053 -> 0.075, mean shelf 292 -> 276 km.
  `TestThePolarSeaIsIce` passes.
- `TestTheChainForOneTileOfTheAncientValley`, the golden test, moves from
  tile 2628 to 987, raised by an arc in the first epoch by some 12 km.

**Digest, budget, time.** `TERRA_DIGEST=write`: `ancient` and `globe128`
move, `valley` does not. `globe128` came in under its budget and the budget
is rewritten: 32056 -> 29149 allocations and 440 -> 406 MB, peak as it was;
`ancient` 8848 -> 8700. `main` against this, before the shelves merged, two
rounds of six turn about:
no significant change on any world (valley +18%, ancient +11%, globe256 -1%,
all p > 0.2; the valley's world is not touched by this and it moved the most,
which is the machine's load). `scripts/perf.sh check` is still not readable
against `2026-09-16-0718-small` (see the shelf entries on
`claude/floor-exact-distance`).

---

## 2026-10-06 - How square the plates are

**What this is.** On `claude/relief-intermittency`. The globes' continents
look cut out in rectangles: long straight sides meeting at corners near a
right angle. This reads how square they are and what makes them so, and
changes nothing in the making.

**The reading.** For each plate or piece of continent of 2000 tiles and
more, how strongly its edges gather at four bearings and at six: c4 =
|<e^{4iθ}>| and c6 = |<e^{6iθ}>| of the direction its eased edge faces,
weighted by the edge, then averaged over the pieces by their size. It does
not care which way a piece is turned. Off drawn shapes:

| shape | c4 | c6 |
| --- | --- | --- |
| square | 0.92 | 0.00 |
| rectangle 2:1 | 0.97 | 0.32 |
| hexagon, triangle | 0.02 | 0.91 |
| disc | 0.01 | 0.00 |
| Voronoi of 24 middles | 0.45 | 0.45 |
| the land of Brownian reliefs, H 0.5 and 0.8 | 0.01 | 0.01 |

**The globes.** Seeds 1 and 3, through a history:

| | first plates | plates at the end | first continents | continents at the end |
| --- | --- | --- | --- | --- |
| globe 1, c4 / c6 | 0.29 / 0.22 | 0.21 / 0.12 | 0.38 / 0.31 | 0.29 / 0.19 |
| globe 3, c4 / c6 | 0.27 / 0.26 | 0.28 / 0.15 | 0.14 / 0.24 | 0.20 / 0.15 |

The continents end twenty times as polygonal as a Brownian coast, and more
at right angles than at sixty degrees. The three fracture sets stand a sixth
of a turn apart, which makes sixty degrees, and that fades over the history
while the right angles hold or grow. That is what plates carried whole
would do - part across the way they go and slide along it, the two meeting
square - though nothing here has taken that apart from the rest of the
history.

**What moves it.** Probes, the continents at the end, seeds 1 and 3:

| | c4 | c6 |
| --- | --- | --- |
| as made | 0.29, 0.20 | 0.19, 0.15 |
| `fractureCreep` 0.05 -> 0.2 | 0.11, 0.12 | 0.08, 0.12 |
| `fractureWall` 6.5 -> 4 | 0.11, 0.16 | 0.15, 0.23 |
| the floods over sixteen neighbours | 0.16, 0.16 | 0.17, 0.20 |
| `fractureCreep` 1.0 | 0.06, 0.06 | 0.07, 0.04 |

How straight the fractures run is most of it; how the floods step is
little. But the creep is in radians a tile and not a kilometre, so it bends
a small globe's faults differently from a full one's, and the plate tests
are read on the small globes:

| `fractureCreep` | `TestAPlatesWallRunsStraight` (1.05 and over) | the other plate tests |
| --- | --- | --- |
| 0.05 | 1.167 | pass |
| 0.10 | 1.056 | pass |
| 0.15 | 1.003 | a plate of 0.39 of the world against a ceiling of 0.22 |
| 0.20 | 1.085 | a plate of 0.35; the plate area exponent 0.436, out of 0.15-0.35 |

Squaring the continents less is a search of the fractures' bend, read per
kilometre, and their wall, against the plate tests, not a change of one
constant.

That search is the entry above: the bend is now quoted per plate spacing
(`fractureBend`), and `cornerLock` holds the right angles.

---

## 2026-10-06 - What decides the land share

**What this is.** On `claude/relief-intermittency`. The K gap "land share
of the surface, furthest of three globes" reads 0.449 against the earth's
0.292. This finds what sets a globe's land and what moving it would cost,
and changes nothing but the gap's message.

**The land is the crust.** Globes 1-3 have 0.225, 0.441 and 0.462 of the
sphere in continental crust and 0.223, 0.431 and 0.449 in land: the sea
stands at 20.29, 22.57 and 24.77 m on basins 20 m deep and covers almost
nothing of the continents. The shelves are laid on the ocean crust beside
them (`floorDepths`), so a globe's continental crust is what the earth's
land is, not the four tenths the earth's continental crust is with its
shelves. The crust is the first plates' draw, at `oceanFloor` +
`oceanPerSea` x `SeaShare` = 0.55 ocean on `GlobeTerms`, and put right only
past `crustSlack`, 0.15, counted in tiles of the cylinder: drawn, the three
are 0.311, 0.448 and 0.546 continent by tile and 0.227, 0.504 and 0.492 by
area, and the history takes them to 0.225, 0.441 and 0.462 by area.

**The sea is the wrong lever.** The continents stand all but flat over the
sea, and unevenly from one globe to the next. Raised over the ground as the
history left it:

| | land, globes 1/2/3 | water it takes, m |
| --- | --- | --- |
| as poured | 0.225 / 0.440 / 0.455 | 7.5 |
| 10 m higher | 0.207 / 0.384 / 0.360 | 14.9 / 13.6 / 13.0 |
| 20 m higher | 0.134 / 0.220 / 0.215 | 22.6 / 20.7 / 20.0 |

**The crust, drawn as the earth's.** Probes, the shape yardsticks over the
three globes, the crust balanced by area on the sphere within 0.05:

| | land, globes 1/2/3 | remoteness |
| --- | --- | --- |
| as made | 0.223 / 0.431 / 0.449 | 0.520 |
| balanced to 0.70 ocean, drawn as before | 0.262 / 0.289 / 0.357 | 0.312 |
| drawn and balanced at 0.70 ocean | 0.257 / 0.312 / 0.310 | 0.438 |
| drawn and balanced at 0.66 ocean | 0.257 / 0.378 / 0.380 | 0.543 |

The remoteness that fell with the balance alone was five continents read,
the rest touching a pole.

**The water has to fill the basins it is given.** `DefaultWater` was
chosen to all but fill basins of half to three fifths of the map. With
seven tenths of floor it does not: small globe 3, 0.764 of its tiles ocean
crust, had its coasts stand beside the deep floor, the shaping laid its land
kilometres down, and the sea poured again stood at -2782 m. The first globe
as made is 0.731 ocean crust by tile and already has 229 dry tiles beside the
deep floor. At 9 m the sea stands 1.8 to 3.2 m over the basins on every
seed, as 7.5 m stood it on the crust it was chosen for.

**What it buys and costs.** Drawn and balanced at 0.70 ocean, with 9 m of
water, every yardstick against the base:

| | base | crust and water |
| --- | --- | --- |
| land share, furthest of three globes | 0.449 | **0.253** (0.253 / 0.308 / 0.306) |
| hypsometric integral, small globe | 0.312, out | **0.362** |
| discharge exceedance exponent, small globe | 0.464, out | **0.436** |
| valley floor over hillslope soil depth, small globe | 1.71, gap | **3.49**, in |
| remoteness, islands, coast dimension, coast lock | 0.52, 0.68, 1.13, 0.084 | 0.42, 0.82, 1.12, 0.060 |
| plate area cumulative exponent | 0.341 | **0.454**, out of 0.15-0.35 |
| meander wavelength, small globe | 11.7 | **14.1**, out of 10-14 |
| mean land rain, 2x over 1x | 1.13 | **1.36**, out of 0.85-1.15 |
| channel concavity, 2x less 1x | -0.07 | **0.18**, out of -0.1-0.1 |

With the slack left at 0.15 the third globe comes out 0.417 and the same
four go out, with Flint's R2 besides, so it is the ocean and not the
balancing. And
`TestAGlobeHasASeaItsRiversReach` holds a globe's sea to 0.4-0.7 by tile and
one tile of the pole bare, both of which an earth's crust moves.

---

## 2026-10-06 - Where the relief's intermittency goes

**What this is.** On `claude/relief-intermittency`, on top of
`claude/statistical-output-verification`. The K gap "land relief
intermittency C1" reads 0.053 against the earth's 0.12. This finds where a
globe loses it and what closing it would cost, and changes nothing but the
gap's message: `TERRA_DIGEST=check` passes.

**Where it goes.** Globes 1-3, read on the same land windows at every step,
and the land cut into boxes of 8 tiles, each with its mean |grad h|:

| step | C1 | boxes, q90 over q10 | boxes ranked against the shaping's input |
| --- | --- | --- | --- |
| the history's heights, before `basins` | 0.154 | | |
| as the shaping takes them up (after `basins`) | 0.175 | 98 | 1.00 |
| graded, before the rescale | 0.059 | 2.0 | 0.04 |
| rescaled to the map's spread | 0.065 | 3.0 | 0.03 |
| after `texture` and `denude` | 0.068 | 2.9 | 0.05 |
| after the stage's slides | 0.053 | 2.6 | -0.02 |
| the finished globe | 0.053 | | |

`basins`' rank mapping keeps the gathering; the shape stage loses it, and
not only its strength but its place: after the grading, how rough a box is
says nothing about how rough it was. The cut, coast and cover stages change
nothing after it.

**Why.** Three things, each measured:

- The grading lays every tile off the rivers as a hillslope of one fall. The
  drop to the steepest neighbour is 7.1 m a tile at the median and 8.5 at
  the ninetieth centile, and ranks with the uplift at 0.06. The uplift it
  grades by is the history's rate by rank onto `shapeFloor`..1, 1.55 times
  end to end, less than the rock's hardness moves a fall; and the rate is
  not gathered to begin with - 0.21 mm/yr over the land at the median, 0.27
  to 0.39 at the ninetieth centile, 1.2 to 1.4 at the ninety-ninth. The
  history's heights are what is gathered.
- The lift, 1-(1-x)^`shapeLift`, steepens low ground up to 2.65 times and
  flattens the tops of ranges.
- The slides at the stage's end hold what is steep at one threshold, 0.068
  -> 0.053: the globe's land stands at a median fall of 0.5 at `TileSpan`,
  much of it near `Critical`.

**What closing it would take.** Probes, C1 at the end of the shape stage on
the same windows; none is kept:

| the grading's uplift | lift | C1 |
| --- | --- | --- |
| as made | 2.65 | 0.053 |
| by rank, floor 0.1 | 2.65 | 0.052 |
| by rank, floor 0.1 | 1 | 0.064 |
| the rate over its 99th centile, floor 0.1 | 2.65 | 0.052 |
| by rank, floor 0.1, the graded height scaled by the history's | 1 | 0.080 |
| the history's local relief within 2 tiles (Ahnert 1970), floor 0.1 | 2.65 | 0.062 |
| the same | 1 | 0.087 |
| the same, floor 0.3 | 2.65 | 0.050 |

The last but one, taken through every stage with the rivers graded from the
sea's level (below), gives the finished globes 0.077 and moves the small
globes' rivers. Against the base, every yardstick:

| | base | relief uplift |
| --- | --- | --- |
| land relief intermittency C1 | 0.053 | 0.077 |
| Hack exponent, small globe | 0.581 | **0.620**, out of 0.54-0.60 |
| channel concavity, 2x less 1x, small globe | -0.075 | **0.225**, out of -0.1-0.1 |
| land share of Mollisols | 0.078 | **0.119**, out of 0.05-0.09 |
| hypsometric integral, small globe | 0.312, out | 0.234 |
| discharge exceedance exponent, small globe | 0.464, out | 0.473 |
| chi-plot linearity, small globe | 0.960 | 0.990 |
| Horton bifurcation, small globe | 3.25 | 4.37 |
| land share, remoteness, coasts, sea floor | | within a few hundredths |

Read over 150 km rather than two tiles, so that a small globe at twice the
resolution reads the same ground, the relief uplift gives C1 0.058 at a
floor of 0.1 and 0.053 at 0.2: the gathering the yardstick reads lies within
one to four tiles of a full globe, which is less than a tile of a small one.

**The sea under the coast.** `shape` grades each tile up from the height of
the root its water ends in, and for the sea that is the floor under the
coast, a few metres below the surface. It does no harm while every hillslope
falls seven metres a tile. With the plains graded gentle it laid much of the
land under the sea: the land share's reading, the globe furthest from the
earth's, went from 0.449 to 0.167. Any
change that makes plains gentle needs the grading to start at the sea's
level.

**Two conflicts in the yardsticks.** The hypsometric integral is read over
each whole map, against Strahler's band for single drainage basins; a
continent with its relief gathered into ranges reads low over the whole of
it (the earth's land, 840 m on the mean against 8848 at the top, about 0.1),
so any closing of this gap lowers it. And the shaping's constants were
searched against the small globes' rivers with an uplift all but even:
gathering it is a search of its own, not a change.

---

## 2026-10-05 - A shelf is as wide as its margin is quiet

**What this is.** On `claude/shelf-margins`, on top of
`claude/floor-exact-distance`. Every margin was given one shelf,
`shelfWidth`, 80 km, and the K gap "shelf width, quiet margins over active"
read 1.67 against the earth's 2.85 (Harris and others 2014: 88.2 km on
passive margins, 31 on active ones). A margin is now passive where the
continent and the ocean floor beside it ride one plate - the textbook
definition, the Atlantic's margins - and active where the floor is another
plate's, going down a trench or grinding past. `floorDepths` finds the
nearest continental tile to each tile of floor (`nearestTo`, the same
transform as `awayFrom`, keeping where each parabola stood), and lays the
shelf `quietShelf`, 88.2 km, or `activeShelf`, 31 km, by whether the two
tiles' plates are one once welds are followed (`rootPlate`; `keepPlates`
now runs before `floorDepths` for it).

**How narrow an active shelf can be.** 31 km is under a tile of a full globe,
37.5 km, and a shelf is laid no narrower than `shelfLeast`, a tile and a half
(see the entry below): the deep floor laid inside the ring of eight round a
coast is land graded under the sea. So on a full globe an active shelf is a
tile and a half and a quiet one two and a third, and the first floor under
200 m is two tiles out of an active coast and three out of a quiet one. On a
small globe, a tile of 150 km, both are the least there is, and nothing
changes: small globes, double globes and the digest's worlds are the worlds
they were to the bit (`TERRA_DIGEST=write` leaves `digest.json` as it was).
Only maps whose tile is under 59 km - 88.2 km over a tile and a half - see
this change, which on the presets is the full globe.

**The shaping that was proposed with it, and was not needed.** The plan was
to let the active shelf go to a tile, and have `shape` grade the land over
the water and not the floor under it (`laidHeight`), which keeps a full
globe's coasts where the floor comes beside them: globe 1 had 0.2249 of its
surface dry when the sea was poured and 0.2231 when it was made, against
0.2232 with the floor kept back. It was measured both ways over every
yardstick:

| | floor kept a tile and a half away (taken) | a tile, and shaped over the water |
| --- | --- | --- |
| quiet over active, three globes | 1.88 | 2.04 |
| mean shelf width | 292 km | 286 km |
| channel concavity, small globe | 0.353 | **0.312**, out of 0.35-0.6 |
| discharge exceedance exponent, small globe | 0.464, out of 0.40-0.46 as on the base | 0.435 |
| drainage area exceedance exponent, small globe | 0.421 | 0.395 |
| Horton bifurcation ratio, small globe | 3.25 | 3.70 |

A shelf of one tile puts the deep floor beside the corners of every small
globe's coast, and the small globes' rivers move: one yardstick out and one
in, for a full globe's ratio a little further inside. Not taken; the gap
closes without it.

**What the globes read.** Seeds 1-3:

| | before (exact distance) | after |
| --- | --- | --- |
| shelf, quiet over active | 1.67 (1.02, 3.03, 0.98) | **1.88** (1.18, 3.58, 1.06) |
| mean shelf width | 279 km (149, 411, 266) | 292 km (151, 434, 277) |
| sea floor within 200 m | 0.094 | 0.097 |
| grid lock of the floor off the coasts | 0.082 | 0.084 |

The quiet-over-active gap has closed and its marker is off: 1.88 is inside
1.8-4.5, near the floor of the band, and pooled over globes that read 1.18,
3.58 and 1.06. The yardstick reads a coast as active where a seam runs within
150 km of it, and most such coasts are not active margins as laid: 1048 of
7041 on globe 1, 1251 of 4712 on globe 2, 1056 of 5554 on globe 3. The rest
are quiet margins with a seam near them, inland or offshore past a strip of
their own plate's floor, and they read as quiet shelves in the active column.
The mean
width goes the wrong way, 279 -> 292 km: the quiet shelves, most of the
coast, are Harris's 88 km now where they were 80, and the active ones narrow
from 80 to 56 at most. It was never the mean's cause: an eighth of the coasts
it reads are shores of hollows inside the continents with no deep floor in
them (see the gap message).

**The sea floor's yardsticks** read beyond a quiet shelf and a slope, now
`quietShelf+slopeWidth`, and do not move: ridge 2.772 km, subsidence 317.4,
flattening 0.207, the modes as they were. The full globe's networks:
drainage area 0.388 -> 0.395, Hack 0.584 -> 0.582, Horton 4.46 -> 4.20.
`TestThePolarSeaIsIce` passes.

**Tests.** `TestAShelfIsWideWhereItsMarginIsQuiet`: a strip of continent
with its own plate's floor to the west and another's to the east; the floor
goes down two tiles out of the east coast and three out of the west, and
welded, the east is three too.

**Heap and time.** `TestWorldCreationBudget` passes and is not rewritten
(ancient +10 allocations, which the valley, untouched, wanders by too;
globe128 +5; bytes +0.05%). `nearestTo` keeps two more `int32` maps for the
one call that asks. Timing against the base, twelve runs each, turn about:
valley and globe256 within noise, ancient -5.2% (p 0.014) on a world that is
the base's to the bit, so drift; `scripts/perf.sh check` fails against
`2026-09-16-0718-small` here as on the base (see the entry below).

**The yardsticks.** The whole suite, `go test -timeout 60m .`, fails the
base's six yardsticks (midlatitude over subtropical rain; Aridisols and
Gelisols; the small globe's hypsometric integral, discharge exponent and
ridge-valley wavelength) and two tests that fail on the base (cc67797) as
well: `TestTheTideLaysFlatsOnlyWhereItReaches`, small globe 2 has no flats,
and `TestAGlobeHasASeaItsRiversReach`, the globe 0.73 water. Nothing new.

---

## 2026-10-05 - The sea floor is laid at the true distance from the continents

**What this is.** On `claude/floor-exact-distance`, on top of
`claude/statistical-output-verification`. `awayFrom` - what `floorDepths`
lays the shelf and the slope at, and what `firstFloorAges` ranks the first
plates' floor by - walked to the eight tiles round each and counted a
diagonal step as one. What it read was the larger of the two distances
across, and the floor laid at it was an octagon round every coast and a
square round every islet: the K gap "grid lock of the sea floor off the
coasts", 0.104. It is now the exact distance, Felzenszwalb and
Huttenlocher's two passes of lower envelopes, which is the transform
`realism_shape_test.go` already read the shelves with (`distanceFrom` there
is gone; the tests call `awayFrom`).

**The cause, measured before the fix.** A floor laid by each distance,
`4000 * smooth((d - 2)/4)`, round coasts that lean to nothing - Brownian
reliefs drawn on a 1024x512 map - and its grid lock read as the yardstick
reads it:

| coasts | the coasts' own lock | floor by the walk | floor by the true distance |
| --- | --- | --- | --- |
| H 0.5, 30% land | 0.014 | 0.230 | 0.013 |
| H 0.5, 45% land | 0.020 | 0.233 | 0.033 |
| H 0.8, 30% land | 0.067 | 0.251 | 0.066 |
| H 0.8, 45% land | 0.018 | 0.216 | 0.035 |

The walk locks a floor to the grid round coasts that have no lock; the true
distance gives the floor its coast's lock and no more. That is now
`TestTheFloorLeansAsItsCoastDoes`, with a check of `awayFrom` against
Pythagoras round the seam (short tier, half a second).

**The shelf's least width.** The shelf was floored at one tile, which under
the walk was the whole ring of eight round a continent. At the true distance
the ring's corners are a root of two out, and on a small globe, whose shelf
is that one tile, they were laid a third of the way down the slope beside the
corner of every coast. `shape` lays every tile over the water its drainage
ends in, and there that was the floor, a kilometre and more down: in the first
draft of this change the shaping laid the whole of small globes 1 and 2 under
the sea (land 0.47 -> 0.00), the sea poured again on what it left stood at
-691 and -712 m, and ten of the small globes' yardsticks went out of their
bands (Hack 0.581 -> 0.427, Flint's R2 0.939 -> 0.731, channel concavity 0.353
-> 0.260, the 99th centile slope 1.06 -> 2.05). `shelfLeast`, a tile and a
half, keeps the whole ring shelf, and with it every small globe and valley is
the world it was to the bit.

**What the globes read.** Seeds 1-3, the shape yardsticks' globes:

| | before | after |
| --- | --- | --- |
| grid lock of the floor off the coasts | 0.104 (0.124, 0.088, 0.124) | **0.082** (0.066, 0.085, 0.103) |
| its terms, cos 4θ and cos 8θ | +0.072, +0.104 | -0.082, +0.023 |
| grid lock of the coasts | 0.084 (cos 4θ -0.084) | 0.087 (cos 4θ -0.087) |
| mean shelf width | 305 km | 279 km |
| shelf, quiet over active | 1.60 | 1.67 |
| sea floor within 200 m | 0.103 | 0.094 |

The octagons are gone (cos 8θ 0.104 -> 0.023). What is left is cos 4θ, the
floor leaning to the diagonals as the coasts do: the floor is now laid as its
coast lies, and the coasts lean 0.087, which their own yardstick (0-0.1)
allows. The gap stays open on that: the floor's band, 0-0.05, is tighter
than a floor laid exactly can read round Brownian coasts of H 0.8 (0.066).
The lean is born with the first plates: on globe 3 their seams read cos 4θ
-0.16 before anything has moved. Flooding the plates over sixteen neighbours
rather than eight did not take it away, so it is not the flood's metric
alone.

**The sea floor's yardsticks** read the first globe, beyond a shelf and a
slope of the continents, and hardly notice: ridge 2.772 km, subsidence 317.4
m per root Myr, flattening 0.201 -> 0.207, the hypsometric modes +0.125 and
-5.375 km as they were. `TestThePolarSeaIsIce` passes.

**The digest is rewritten**: `globe128` moves, `valley` and `ancient` do not
(the ancient valley has no water, so no floor to lay).

**The yardsticks.** `go test -run 'TestRealNumbers|TestTheRealWorld'` here
and on the base (cc67797) fail the same six: midlatitude over subtropical
rain, the land shares of Aridisols and Gelisols, and the small globe's
hypsometric integral, discharge exponent and ridge-valley wavelength. The
full globe's networks move a little and stay in: drainage area exponent
0.395 -> 0.388, Hack 0.583 -> 0.584, Horton 4.20 -> 4.46.

**Heap and time.** `TestWorldCreationBudget` passes and is not rewritten:
ancient +5 allocations, globe128 -2, bytes +0.03%. `scripts/perf.sh check`
fails against `2026-09-16-0718-small` on this commit and on its base alike
(+12 to +17% on all three worlds; the base moved 12% between two rounds of
its own), so it cannot judge this. Base against this commit, twelve runs
each, taken turn about on a quiet machine: valley, ancient and globe256 all
within noise, geomean -2.8%.

---

## 2026-10-05 - The tide's flats are read off the first small globe

**What this is.** On `claude/interesting-hellman`. Changes only a test.
`TestTheTideLaysFlatsOnlyWhereItReaches` has failed on `main` since d18d65d
with "small globe 2 has no flats", and it fails the same way with
`TERRA_HISTORIES=off`, so a stale history is not the cause. I ran `git bisect`
from 6c232a4 (good) to 0da2c55 (bad), running only that test with histories
off. It stops on df5b2c1, "The crust breaks before its plates are grown". That
commit changes `history.go` and nothing else outside the tests and docs, and
its own entry below already lists this failure among the readings it moved.

**Is the tide still right?** I counted tiles on small globes 1-8 through each
gate that `tides` puts in front of a flat: does the tide reach the tile, does
it stand within `f*flatTide` of mean sea, is it no steeper than `deanSlope`,
is it tide-dominated, can its terrain turn, and is it unheld. The counts are
from 9416a1f (before) and df5b2c1 (after). 0da2c55 reads the same as df5b2c1
on all eight globes, so the permafrost fringe and the atmos move did not touch
the flats.

| | before | after |
| --- | --- | --- |
| flats, globes 1-8 | 2, 3, 2, 3, 1, 0, 1, 3 (15) | 2, 0, 1, 0, 1, 3, 1, 0 (8) |
| within a spring's reach, eight globes | 857 | 622 |
| of those, no steeper than Dean's slope | 29 (3.4%) | 17 (2.7%) |
| globe 2: within reach / gentle enough / flats | 106 / 4 / 3 | 72 / 0 / 0 |
| globe 2: gentlest in reach, over Dean's slope | under 1 | 6.35 |

The tide's own factor in that band sits at a median of 1.00 before and after.
The gates let about the same share through, about 3%. What fell is the amount
of ground near sea level, by a quarter across the eight globes and by a third
on globe 2. Every one of globe 2's 72 tiles in reach is at least six times
steeper than the sea grades its grain. The plates now break on straight
fractures, so the coasts stand somewhere else. The tide still lays flats
wherever there is gentle ground, and on this seed there is none. This is a
consequence of the world change, not a fault in the tide.

**The change.** The test reads small globe 1. It kept its two flats through
the crust change. The short tier already makes it for `TestRainFallsInBelts`,
so the switch adds no world to that tier. The test's comment carries the new
per-globe counts and says why globe 2 lost its flats. No source outside
`shore_test.go` changed.

**Checked.** `TERRA_DIGEST=write` on 0da2c55 leaves `docs/perf/digest.json` as
committed, and `TERRA_DIGEST=check` passes after the change. The test passes
with and without `TERRA_HISTORIES=off`. `go vet ./...` and
`go test -short -timeout 60m ./...` are green. The budget, `scripts/perf.sh
check` and the yardsticks were not run. No code that makes a world changed,
and the digest shows that no world moved, so none of them can read
differently. On a full run, the yardstick list should lose this test and
gain nothing.

---

## 2026-10-05 - The shape of the world held against the earth's

**What this is.** On `claude/statistical-output-verification`. A world can
answer to the yardsticks and still not look like the earth, because they read
how much of a thing there is and how it goes with latitude, and not how it is
laid out. `realism_shape_test.go` adds eleven that read the layout, each a
figure measured on the earth and read off a world the way it was read there,
at the globe's planetary scale (deepSpan, 37.5 km a tile) or with no scale in
it. They join `realYardsticks` as the soil's do, so `TestTheRealWorld` holds
them and `TestCalibrate` prints them. Nothing outside the tests changes.

They read the three full globes `TestThePolarSeaIsIce` already makes, seeds 1
to 3, pooled: a globe has a handful of continents, and one globe's handful
mostly says which globe it was. Every measure is read first off drawn shapes
whose answers are known, in `TestTheShapeMeasuresReadDrawnShapes` (short tier,
4 s):

| measure | drawn shape | reads | true |
| --- | --- | --- | --- |
| remoteness | disc, square | 0.992, 0.883 | 1, 0.886 |
| Richardson divider dimension | disc, square, Koch island (depth 5) | 1.004, 1.004, 1.235 | 1, 1, 1.262 |
| grid lock | disc, square, diamond; Brownian coasts H 0.5, 0.8 | 0.025, 0.985, 0.991; 0.013, 0.035 | 0, 1, 1; 0, 0 |
| spectral exponent | Brownian relief H 0.5, 0.8 | 1.964, 2.591 | 2.0, 2.6 |
| C1 by trace moments | lognormal cascade sigma 0.4; Brownian relief H 0.5, 0.8 | 0.109; 0.036, 0.037 | 0.115; 0 |

The Brownian relief's 0.037 is the floor of the C1 reading: a relief rough
everywhere alike reads that, and the earth's reads 0.12.

**What the globes read.**

| yardstick | globe 1 | globe 2 | globe 3 | pooled | earth | |
| --- | --- | --- | --- | --- | --- | --- |
| land share of the surface | 0.223 | 0.431 | 0.449 | 0.449 furthest | 0.292; 0.2-0.4 | gap |
| remoteness of the continents | 0.63 | 0.33-0.58 | 0.40-0.68 | 0.521 median | 0.45-0.65 (G-C & Lombardo 2007) | holds |
| island size exponent (Korcak) | 0.684 | 0.667 | 0.433 | 0.683 | 0.65; 0.5-0.75 (Mandelbrot) | holds |
| coast dimension, 75-1200 km | 1.137 | 1.134 | 1.116 | 1.127 | 1.02-1.25 (Richardson) | holds |
| grid lock of the coasts | 0.069 | 0.041 | 0.137 | 0.085 | nothing; 0.1 | holds, near the edge |
| sea floor within 200 m | 0.083 | 0.108 | 0.126 | 0.103 | 0.073 (GEBCO 2019) | holds |
| mean shelf width, km | 165 | 453 | 286 | 305 | 57 (Harris et al. 2014) | gap |
| shelf, quiet over active margins | 0.95 | 2.78 | 0.97 | 1.60 | 2.85 (Harris et al. 2014) | gap |
| grid lock of the sea floor off the coasts | 0.124 | 0.088 | 0.124 | 0.104 | nothing; 0.05 | gap |
| land relief spectral exponent | 2.206 | 2.127 | 2.175 | 2.166 | 2.1 (Gagnon et al. 2006) | holds |
| land relief intermittency C1 | 0.059 | 0.062 | 0.041 | 0.053 | 0.12 (Gagnon et al. 2006) | gap |

Six hold and five are known gaps, under the letter K. The gaps are the three
things that make a globe look made rather than found:

- *The halo.* Every coast has the same pale band of shelf, five to twelve
  times the earth's width, octagonal round a continent and square round an
  islet. `floorDepths` lays the shelf and slope at `shelfWidth` and
  `slopeWidth` from the edge of the continental crust, by `awayFrom`, which
  counts a diagonal step as one: the same width on every margin, and a
  Chebyshev ball at each.
- *The carpet.* The land's relief has the earth's spectrum and half its
  intermittency: it is rough in the same way at every scale, as the earth's
  is, but rough everywhere alike, where the earth's roughness is gathered
  into ranges with plains between.
- *The land.* Two globes of three are 43 and 45 per cent land.

**What it does not read.** The straight-edged polygon continents that
`fractureWall` makes on purpose. Two readings were tried and neither tells
them from natural coasts at this scale. The share of the coast lying in
straight runs of 16 tiles inside a two-tile corridor is 0.17 to 0.26 on the
globes, against 0.18 to 0.21 for Brownian coasts of H 0.8. Richardson's
dimension at 300 to 2400 km openings is 1.15 to 1.16, against 1.15 to 1.18
for the same Brownian coasts, and the earth's continental margins have H
0.77. What the eye catches is the turning gathered into corners, and no
published figure for the earth reads that. It would want the earth's coast
read with the same instrument.

**What it cost.** Nothing in the making: no file outside the tests changed,
and `TERRA_DIGEST=check` passes. The readings take some three seconds over
the three globes the suite already makes.

---

## 2026-09-19 - The crust breaks before its plates are grown

**What this is.** On `claude/world-roundness-plate-smoothness`. A world felt
too round, and the plates were where it came from. They are grown by flooding
outward from middles over a cost field of three octaves of value noise plus
per-tile jitter (`plateRough`, `plateJitter`), and smooth isotropic noise bends
a boundary without ever breaking it: every plate came out a convex patch with a
crinkled edge, and the continents, the coasts and the ranges inherited it.
`history.go` now lays a fracture network over that field before any plate is
grown - straight lines in three conjugate sets, lengths drawn log-uniform, each
stopping on one laid before it - and the floods stall on them. See
`fractureWall` and `fractures`.

**How round is measured.** `TestAPlatesWallRunsStraight` (`plate_test.go`): the
longest stretch of a plate's edge that stays inside a straight corridor two
tiles wide, over the square root of the ground the plate holds, median over the
plates holding a fiftieth of the map. It carries its own scale - a circle's
longest such chord is 2*sqrt(2*r*w), so the reading is the same for a disc of
any size - and it was calibrated on drawn shapes: a disc 0.48, a square 1.00, a
Voronoi of sixteen middles 1.33, the blocks a fracture network cuts 1.42.

| | before | after |
| --- | --- | --- |
| plate wall, three small globes (the test) | 0.91 | **1.17** |
| plate wall, eight small globes, finished | 0.980 +- 0.033 | **1.182 +- 0.047** |
| plate wall, eight small globes, first plates | 0.960 +- 0.043 | **1.313 +- 0.064** |
| coast, eight small globes | 1.08 +- 0.08 | 1.16 +- 0.15 |

The first plates come out at a Voronoi's reading, which is what a broken shell
should give. Two thirds of that is then worn off by the history itself: the
plates are carried bodily, eaten at the fronts and welded, and after sixteen
epochs only a quarter of the boundary still lies on a fracture. That is the
honest limit of this change - it makes the crust break in lines, and the
history goes on rounding what it is given.

**The tuning.** `fractureWall` at 6.5 was picked over 3.0, 4.5, 5.0 and 8.0 on
the eight-globe reading. Under 5 the yardstick is not met (4.5 reads 1.02); at
8 the first plates are straighter still, 1.372, but the finished ones are not
(1.146) and the coast is worse. The wall multiplies the rough ground rather
than adding to it, which is what makes it work at all: a flood's boundary
settles on the dearest ground there is, and an additive wall worth forty tiles
of ordinary going is decisive on the cheap ground where no boundary falls and
worth a tile where they all do. Added, the eight-globe reading was 1.053 at
best; multiplied, 1.313.

**What it cost the heap, which is nothing.** The multiplicative wall makes the
cost field span four decades, and `floodOver` sorts its frontier into buckets a
fixed width apart: a world now wants some hundreds of thousands of buckets
where it wanted a few hundred. A slice per bucket allocates the first time each
bucket is pushed to, so the first draft of this added four thousand allocations
to the making of a valley - so the frontier is now one arena, `head`/`next`/`at`
on `flooding`, which is a linked list per bucket out of one growing slice. That
is a saving in its own right and the budget moves down with the fractures in
place:

| | allocations | peak | bytes |
| --- | --- | --- | --- |
| ancient | 10420 -> **8848** (-15%) | 9.7 -> **8.6 MiB** (-12%) | +0.13% |
| globe128 | 34090 -> **32056** (-6%) | 28.2 -> 28.6 MiB (+1.4%) | +0.02% |
| valley | 1333 -> 1336 | 3.00 -> 3.06 MiB | +0% |

The bytes are the fracture pass's own working, which is a map's worth of
`float64` and a map's worth of `int32` held while the cost field is drawn and
dropped after. Nothing is kept.

**The digest is rewritten and the world has moved.** This is a change that
means to move every world and says so. `TERRA_DIGEST=write` on this commit.

**The yardsticks.** `go test -timeout 60m .` against the same run on `main`
(9416a1f), both on this machine. Four of main's failures pass now:

- `meander wavelength` 14.32 -> inside 10-14
- `channel concavity, small globe` 0.2766 -> 0.3528, inside 0.35-0.6. The
  "known gap: B" marker is taken off it, with a note that the reading is
  barely inside and swings with the coast, so a change that puts it back under
  0.35 has reopened an old gap rather than broken anything new
- `drainage area exceedance exponent, small globe` 0.4641 -> inside
- `Horton bifurcation ratio, globe` 5.311 -> inside 3-5
- `TestAHistoryLeavesItsBedsInLayers`

and these are new. Every one of them is a reading over a handful of worlds
whose continents now stand somewhere else:

- `land share of Gelisols` 0.148 against 0.06-0.11, and `land share of
  Aridisols` 0.063 against 0.09-0.15: more of the land is polar and less of it
  is desert on these seeds. Read over the globes there are, not over a
  distribution.
- `midlatitude over subtropical rain, globe` 0.81 against 1.1-2: the same
  cause - where the land is decides where the rain is counted.
- `ridge-valley wavelength, small globe` 533 m against 24-224: the reading is
  a spectrum over runs of sixty-four land tiles in a row, and a world has as
  many of those as its continents happen to give it. This is the one worth
  looking at again: at 4.5 it fails too, so it is not only the strength of the
  wall, and a wall does concentrate a world's seams onto fewer, longer lines
  and leave the blocks between them flatter.
- `hypsometric integral, small globe` 0.3121 against 0.32-0.6 and `discharge
  exceedance exponent, small globe` 0.4638 against 0.4-0.46: both a hair
  outside, both inside on main by a hair.

That is eight failures against main's four, and it is the part of this change
a reader should weigh: it buys a world whose plates are polygons and it costs
four readings that were inside and are now outside, three of them by a hair or
by where the continents fell and one - the valley spacing - by a factor of two
and for a reason the wall is at least partly responsible for.
- `TestAGlobeHasASeaItsRiversReach`: the globe is 0.73 water against a ceiling
  of 0.70. How much of a globe is sea is the plates' to say (see `crustSlack`),
  and the plates are different plates.
- `TestTheTideLaysFlatsOnlyWhereItReaches`: small globe 2 has no tidal flats at
  all. Small maps have tiny tides (see the coasts work); this seed's coast no
  longer has anywhere shallow enough.

**Also moved.** `TestTheChainForOneTileOfTheAncientValley` is a golden test and
names a tile; tile 257 is no longer one an arc raised, and tile 2628 is.

**Timing.** `scripts/perf.sh check` against `2026-09-16-0718-small`: no
significant change on any of the three worlds, geomean -1.00% (valley 82.5 ->
80.2 ms p=0.065, ancient 344.9 -> 344.0 ms p=1.000, globe256 4.117 -> 4.119 s
p=0.589). Bytes and allocations against that baseline are down 26% and 15% in
geomean, most of which predates this branch; the honest before/after for this
change is the budget table above.

---

## 2026-09-18 - the permafrost edge is a fringe, not a line

**What this is.** On `claude/permafrost-border-realism`. `Grid.Frozen` is a
threshold: the year's mean on the tile under `atmos.Permafrost`, which is -2
C. Every field behind that mean is smooth and large - the latitude's mean,
the sea about the tile blurred over `maritimeSpan`, the currents off its
coast, the lapse off its height - so the level set of the threshold is a
smooth curve, and the map of the frozen ground stops at a ruled edge with
nothing but terrain roughness to make it ragged.

The real thing has no edge. Permafrost is mapped in four zones, told apart by
the share of the ground each holds - continuous over nine tenths,
discontinuous a half to nine tenths, sporadic a tenth to a half, isolated
patches under a tenth (Brown and others, 1997; Obu and others, 2019) - and in
central Siberia and northern Canada the walk from the first zone to the last
is some hundreds of kilometres. What settles a hectare inside that band is
the snow that drifts over it, the peat on it, which way its slope faces and
whether there is a lake on it, none of which a tile kilometres across knows.
What it can know is the share, and that is what was missing.

**What was added.** `atmos.FrostShare(mean, swing)` and `Grid.FrostShare(i)`:
the share of a tile's ground that is permafrost. It is read off the frost
index the line already comes from - Nelson and Outcalt's F, which was in
`year.go` as the derivation of the -2 and used nowhere - as a smoothstep from
none at F = 0.5, the outer limit, to all of it at F = 0.71. `frostAll` is not
picked: 0.71 is what puts F = 0.67, Nelson and Outcalt's continuous zone, at
the nine tenths of the ground the continuous zone is mapped at.

Its shape comes out right without tuning, because the index carries the
swing. The zone the share crosses is narrow in a maritime year and wide in a
continental one: nine tenths of the ground is reached 5.5 C under the line at
a swing of 8 and 10.5 C under it at a swing of 20. That is the real pattern -
the continuous limit stands at a warmer mean by the sea than inside a
continent, and the Siberian transition is the wide one.

**What it costs.** Nothing that is made. `FrostShare` is read-only and no
stage calls it; `Frozen` is untouched and still what `pedogenesis`, `shore`
and the yardsticks ask. `TERRA_DIGEST=check` passes on `docs/perf/digest.json`
as `main` leaves it after the merge below, so every world is bit for bit what
it would be without this branch, and the budget is unmoved (valley 10.0 MiB in
1329 allocations, ancient 56.2 MiB, globe128 419.4 MiB, each against the same
budget; time is +4 to +13% on a machine under load and is not a reading).

**What it shows.** Read after `main` was merged in, so on the world the crust
change of 2026-09-19 leaves. On the yardstick globe (seed 1, `GlobeTerms`), by
the land's area: permafrost reaches 18.6% of the land and covers 11.4% of it,
and 80% of what it reaches is fringe rather than continuous. Earth's permafrost
region is some fifteen per cent of the exposed land and about three quarters of
that region is actually underlain (Obu and others, 2021); this globe reaches a
little wider and covers three fifths of what it reaches, so its continuous zone
is still the thin part - as is its cold, the same globe putting 0.0% of its land
under ice. How cold the poles run is the open question of the latitude profile
work and not of this change. On `cmd/overview -preset globe`, counted by the
tile, permafrost reaches 45.7% of the land and covers 35.9%.

Before that merge the readings were 9.0% reached and 4.9% covered. The crust
change moved how much ground is cold; it did not move the fringe, which held at
80% of the reach across a world change of that size.

**The map.** `overview`'s temperature map hatched the frozen ground on every
third anti-diagonal. It now dithers that hatch by the share, through a 4x4
ordered matrix whose thresholds are all under one, so the continuous zone is
hatched exactly as before and only the fringe thins - solid lines in the
north breaking into scattered dots over a wide band before they stop. The
printed summary and the page carry both numbers now: what permafrost reaches
and what it covers.

**Left alone.** The share is a share of the ground within a tile and not a
chance the tile is frozen: the same tile gives the same answer, which is what
makes a fringe rather than a dice roll. What the fringe still cannot do is
say *which* hectare - there is no snow depth, no peat, no aspect, no talik
under a lake, and no relict permafrost carrying a colder past forward, which
is the other reason the real edge does not sit on today's isotherm. Those are
their own work and their own yardsticks.

**The record.** `TestTheColdKeepsToThePoles` cites its readings in its own
comment but printed only the area figure, which is how the by-tile one went
stale twice over. It logs both now, and the comment carries today's - 18.6% of
the land's area, 46.1% by the tile - with the crust change named as what took
it there from 9.0%.

**Checked.** `go vet ./...`; `go test -short -timeout 60m ./...`, green but
for `TestTheTideLaysFlatsOnlyWhereItReaches` ("small globe 2 has no flats"),
which fails the same way on `main` at d18d65d; `TERRA_DIGEST=check`;
`TestWorldCreationBudget`; `TestTheColdKeepsToThePoles` and the new
`TestThePermafrostEdgeIsAFringe` on the globe. The failure this branch was
written against - `TestAHistoryLeavesItsBedsInLayers`, "only 49% of the map
stands on more than one bed" - is gone with the crust change. The yardsticks
are untouched by construction: the digest says no world moved and nothing they
read has changed, so they were not re-run.

---

## 2026-09-18 - cmd/zarr takes zarr v0.3.0, and zarrdiff walks with it

**What this is.** On `claude/zarr-version-upgrade`. `cmd/zarr` required
`github.com/LukasSelin/zarr v0.1.0`; the module is now at `v0.3.0`, which
brought `Append`, `Resize` and `Refresh` (v0.2.0) and then listing,
`Group.Children` and `Delete` (v0.3.0). Nothing in the root package imports
it - its `go.mod` is still the standard library's alone - so no world
moves: no digest, budget or yardstick run is owed.

**What the version cost.** One break: `Store` gained `List` in v0.3.0, so
the `discard` store of `memory_test.go` - which keeps the metadata and
counts the rest away, so that `TestExportPeak` reads the export's heap and
not a store's - needed one. It lists the metadata, which is all it holds.

**What the version bought.** `zarrdiff`'s `walk` was a hand-rolled walk of
the directories for `zarr.json`, under a comment saying `zarr.Store` cannot
list its keys. It can now, so `walk` takes a `zarr.Store` rather than a
path and recurses on `Group.Children`: a listing of one level and a read of
each name's metadata per group, in place of `os.ReadDir` and the package's
own parse of `node_type`. Eleven lines net and five imports (`os`,
`filepath`, `io/fs`, `encoding/json`, `errors`) go, and the walk no longer knows that a
store is a directory - the same `walk` would do for a bucket. A store whose
root holds no `zarr.json` used to be walked for children anyway and is now
an error naming the store, which is what `cmd/zarr` writes and `zarrdiff`
compares in any case.

**Left alone.** `consolidate` (`export.go`) reads the nodes the export
recorded rather than listing the store, which is fewer requests, not more.
`storeFiles` (`stages.go`) walks files to compare their bytes; `List` would
make it store-agnostic, but it is not working around a gap in the package.

**Checked.** `go vet ./...` and `go test -timeout 60m ./...` in `cmd/zarr`:
`cmd/zarr` 22.3 s, `cmd/zarr/zarrdiff` 2.6 s, both ok. The root package is
untouched.

---

## 2026-09-16 - The air leaves the root package for internal/atmos

**What this is.** The sixth move of splitting the root package, on
`claude/atmos`, stacked on `claude/clock-moon`, in three commits each held
to the digest. `wind.go`, `vapour.go`, `orographic.go`, `ocean.go`,
`synoptic.go`, `ebm.go`, `year.go`, and the air's arithmetic out of
`weather.go` (now `atmos/air.go`), are `internal/atmos`: the climate of the
wind, the water in the air, the currents, the energy balance, the shape of
the year and the day's weather. The root keeps the grid's side - the
weather pass, the rain and runoff on each tile, and in `sky.go` the grid's
and the land's readers of the wind and the day - with the old public names
as aliases.

1. The air stopped reading the grid. The environment is built from the
   map's size, the air and the ground as the air reads it, handed over as
   two slices the grid fills; the orographic pass takes the map and the
   air; the rain pass's air-cell half is `RainCells`; the day's weather is
   made and advanced by methods of its own.
2. What the root reads was exported with `gopls rename`: some ninety
   fields, methods and functions, `airEnv` becoming `Env`.
3. The files moved. The tests that ask only the air went with them
   (`ebm_test.go`, and the year's, the storm's, the hypsometric and the
   day's range tests); the tests that make a grid or a land stayed. The
   goroutine loop is `internal/par`, and the land hands the air its
   `WorkersFor` at init.

**Found on the way.** Moving the ground loops out of the environment into
`Grid.airGround` added two allocations to every reading of the air: its
slices were named results, and a named result shared with a goroutine's
closure is put on the heap. Found by diffing `-memprofilerate 1` profiles of
`TestWorldCreationBudget` against the parent branch; as locals, the counts
are the parent's (valley 1329-1331, ancient 10412-10415, globe 34095-34104
against 1330-1331, 10413-10414 and 34088-34092).

**Checked.** `TERRA_DIGEST=check` passes on the scalar build and under
`GOEXPERIMENT=simd`, with `TestMakingAWorldDoesNotDependOnTheGoroutines`, the
heap budget and, under `TERRA_PHASES=1`, the pinned pass counts. The 34
weather, wind, current, storm and year tests in the root pass without
`-short`, and `internal/atmos`'s own. `go test -short ./...` fails only
`TestAHistoryLeavesItsBedsInLayers`, as main does. The yardsticks were not
run: no world moved.

**Timing.** `scripts/perf.sh check` against `2026-09-16-0718-small.txt`
passes: valley -5.8 %, ancient -4.6 %, globe256 -5.5 % a tile, within the
spread the moves before it read.

---

## 2026-09-16 - The moon and the tide go to clock

**What this is.** The fifth move of splitting the root package, on
`claude/clock-moon`, stacked on `claude/internal-phase-sysmem`. The moon
and the tide at an open coast were a function of the tick and the founding
moon and nothing else, so they are the calendar's: `clock/moon.go` holds
`Moon`, `Tide`, `Epoch`, `EpochOf`, `MoonOn`, `TideOn` and the tide's
strengths, with the two tests that ask only the moon. `tide.go` keeps the
day's sea as the land reads it - `Land.Moon`, `Land.Tide`, `Grid.Tide`,
`Grid.SetTide` - and the old names as aliases. The seed hash the moon was
drawn with is also the bedrock's, so the root keeps it and `clock` has a
copy of its own.

**Checked.** `TERRA_DIGEST=check` passes, with
`TestMakingAWorldDoesNotDependOnTheGoroutines` and the heap budget. The moon
tests pass in `clock` and the land's two tide tests in the root.
`go test -short ./...` fails only `TestAHistoryLeavesItsBedsInLayers`, as
main does.

**Timing.** `scripts/perf.sh check` against `2026-09-16-0718-small.txt`
passes: valley -3.8 %, ancient no significant change, globe256 -2.7 % a
tile. Nothing here runs while a world is made.

---

## 2026-09-16 - A clone reads as its original

**What this is.** A fix to `Grid.Clone`, on `claude/clone-day-range`. Clone
dropped `dayRange`, so `pet` on a clone left out the day's temperature range.
The soil climate's wetness read differently too, and laying the soil state
again on a clone of the 1024x512 globe (seed 1) gave different lime, salt
and leaching on 1-8 % of tiles. With `dayRange` copied they match bit for bit.

Checking every `Grid` field against Clone turned up more that it dropped:
- `floorAge`, shared now as `abyss` and `uplift` are.
- `bankLoad` and `exported`, so the next Erode carries the bank load on.
- The woods readings (`steepAt`, `steepLine`, `woodsLine`, `woodsRead`,
  `twiMean`, `holds`). A clone used to take these again from the ground as
  it stands.
- `Active` and each chunk's wake state (`Trodden`, `Trod`, `Grown`,
  `Weathered`), which `layChunks` used to reset.
- `waters`, `welds`, `hot`, `deep` and `planet`.

Clone still leaves some fields out on purpose, and its comment now lists
them: the scratch (with a history's `seam` and `seamQueue`), `aired`, and
what a clone takes again from its own ground (patches, lenders, regions,
landmarks, router). `islanded` is left out too.
`TestACloneReadsAsItsOriginal` checks that `pet`, `Runoff`, `YearAt`, `SoilAt`,
the soil climate and `FloorAge` match on the valley and on small globe 1. The
small globe is skipped under `-short`. Without the `dayRange` copy, 9999 of
the small globe's 32768 tiles read differently.

**Checked.** World making doesn't use Clone. `TERRA_DIGEST=check` passes.
`go test -short` fails only `TestAHistoryLeavesItsBedsInLayers`, as main
does. Timing and the heap budget weren't taken, because nothing that makes a
world changed.

---

## 2026-09-16 - The pass clock and the memory probe leave the root package

**What this is.** The fourth move of splitting the root package, on
`claude/internal-phase-sysmem`.

- The per-pass clock is `internal/phase`. A pass starts it with
  `defer phase.Start("drain")()`. The list of passes whose entries are made
  ahead stays in `phases.go`, which hands it to `phase.Prepare` at init; the
  root keeps `Phase`, `Phases`, `ResetPhases` and `PhaseTable` for
  `cmd/overview` and the benchmarks.
- `memory_windows.go`, `memory_linux.go` and `memory_other.go` are
  `internal/sysmem`, whose `Free` is what `memory.go` holds a world against.

**Checked.** `TERRA_DIGEST=check` passes, with
`TestMakingAWorldDoesNotDependOnTheGoroutines` and the heap budget.
With `TERRA_PHASES=1`, `TestPassCountsArePinned` and the budget test pass,
so the clock still counts every pass and its peak readings still come.
`go vet` passes for Windows, Linux and macOS. `go test -short ./...` fails
only `TestAHistoryLeavesItsBedsInLayers`, as main does.

**Timing.** `scripts/perf.sh check` against `2026-09-16-0718-small.txt`
passes: valley -4.6 %, ancient -2.8 %, globe256 -3.5 % a tile, as the moves
before it read.

---

## 2026-09-16 - The kernels leave the root package for internal/kernel

**What this is.** The third move of splitting the root package, on
`claude/internal-kernel`. `kernel.go`, `kernel_noasm.go`,
`kernel_simd_amd64.go`, `fft.go` and their tests are now
`internal/kernel`. What the root calls is exported: `Fade`, `Grow`, `Axpy`,
`FFT`, `FFT2`, `PowerOfTwo`, and `Lerp`, `Clamp`, `SumTree`, `Stencil5` and
`MinmaxSelect` beside them for the passes to come.

The one tie to the land was `grow`, which read the day's pass's tables of
which kinds age. `Grow` is now handed them, `ages` by kind and `aging` the
same kinds listed, and `FuzzGrow` draws its own: from none to ten kinds of
256, so both of the cases the vectors hand back to the statement are tried.
`scripts/perf.sh simd` builds the package `PERF_PKG` names, `.` by default;
the kernels alone are
`PERF_PKG=./internal/kernel PERF_BENCH='Kernel|FFT' scripts/perf.sh simd`.

**Checked.** `TERRA_DIGEST=check` passes on the scalar build and under
`GOEXPERIMENT=simd`, with `TestMakingAWorldDoesNotDependOnTheGoroutines` and
the heap budget, which needs no update. The kernel tests pass on both
builds, and `FuzzGrow` ran 30 s under `GOEXPERIMENT=simd` clean.
`go test -short ./...` fails only `TestAHistoryLeavesItsBedsInLayers`, as
main does.

**Timing.** `BenchmarkGrowRow`, the day's fade and grow over a chunk's row,
eight runs of main against eight of this, turn about, Ryzen 9 3900X:

| build | main | this | |
|---|---|---|---|
| scalar | 661.1 ns ± 4 % | 653.8 ns ± 2 % | ~ (p=0.398) |
| simd | 641.9 ns ± 2 % | 626.6 ns ± 3 % | -2.4 % (p=0.005) |

Handing `Grow` a slice where it indexed an array costs nothing measurable.
`BenchmarkKernel/grow` is not comparable across the move: before it, the
kinds that aged were whatever the root's tests had registered.
`scripts/perf.sh check` against `2026-09-16-0718-small.txt` passes: valley
-4.7 %, ancient -1.8 %, globe256 -3.7 % a tile, the same as the move
before it read against that older baseline.

---

## 2026-09-16 - geom.Map and package tile leave the root package

**What this is.** The first two moves of splitting the root package into
sub-packages. On `claude/root-file-organization-d117ef`.

- `globe.go` is now `geom.Map` (`geom/map.go`): `W`, `H`, `Wrap` and the
  arithmetic on them, `In` included. `Grid` embeds one, so `g.Index` and
  `g.W` read as before; a Grid made field by field names its map.
- `kind.go`, `mark.go` and `claim.go`, `Tile` and `Terrain` from `grid.go`,
  the rock kinds, their strength and chemistry from `bedrock.go`, the move
  cost table from `move.go` and the soil-chemistry fields' accessors from
  `pedogenesis.go` are now package `tile`. Every method on `Tile` had to go
  with it, so the tables those methods read went too. `tile.go` in the root
  keeps the old names as aliases, so `terra.Tile` and `terra.Forest` mean
  what they meant.
- Table lookups outside the package became method calls: `markCost[m]` is
  `m.Cost()`, `moveCost[t]` is `t.Cost()`, `hardness[b]` is
  `b.Hardness()`. `Terrain.Cost` keeps the range guard it already had, so
  the router's step pays one predicted branch it did not before.

**Checked.** `TERRA_DIGEST=check` passes on the base commit and after each
move, with `TestMakingAWorldDoesNotDependOnTheGoroutines`.
`TestWorldCreationBudget` passes without an update. `go vet ./...` and
`go test -short ./...` pass. The yardsticks were not run: no world moved.

**Timing.** `scripts/perf.sh check` against `2026-09-16-0718-small.txt`
passes: valley -4.2 %, ancient -2.6 %, globe256 -4.5 % a tile. Nothing
here makes a world faster, and the baseline is older than work merged since
(its bytes and allocations a run are 18-31 % higher than now), so this reads
as no slower and nothing more.

---

## 2026-09-16 - Phase 3, step 2: a plate is carried the part of a tile a step leaves over

**What this is.** The fix to the lag of the step before, on
`claude/history-km`, and the world it moves. `move` stepped a plate a whole
tile of its grid once its travel came to one and held the rest back, so a
plate stood most of a tile of its grid behind its travel whatever the grid.

**The change.** A plate steps to the nearest whole tile of its travel, and
the rest - under half a tile either way - is the plate's: `turn` reads the
whole plate that far off its tiles, from the tile the slide rounds to, the
way it reads a turn, and a tile's own offset keeps only what turns leave.
`TestASlowSlideIsNotHeldBack` holds a continent sliding 1, 0.4 and 0.25 tiles
an epoch into the ocean to its travel within half a tile, whole, for ten
epochs; on the old move the slow slides lagged.

The first form of it (71e7f1a) put the rest into every tile's own offset and
looked for a tile's crust two tiles round. It failed the suite: the offsets
turns had left each tile no longer fitted together, lone tiles of the other
crust came out ten times as many (37-187 a small globe against 3-9), `shape`
graded lone continent tiles down to the deep floor beside them, the slides
took a sixth of small globe 5 after them, and its sea poured at -3,100 m,
which reads as no sea (`TestTheSeaIsTheWorldsToSay`: -2,599 m held). Held by
the plate, lone tiles are 2-11 a small globe again.

**What it measured.** Globes 1-8, the map against a half-size history
(`TERRA_HISTORY_SHRINK=2`), before the per-plate form: collision ground
+38% (t 1.6), schist +17% (t 1.0), granite -23% (t -1.8), nothing past t 2,
where the same comparison before the fix had schist at t 5.3 and the
collisions at t 2.8. The globe on the map with the per-plate form: 48.8 s
(main 47.5 s, three interleaved runs); the first form's search had it at
55.4 s. Digest (ancient, globe128) and budget rewritten; the drawn valley is
as it was. The golden chain reads tile 257 of the ancient valley, raised by
an arc in the first epoch; the tide's flats are read off small globe 4,
which has 2 (small globes 1-8 now have 1, 1, 1, 2, 0, 0, 0 and 2).

**The suite** (`go test -json -timeout 60m .`, quiet, one binary a side,
histories not kept). Main at 3a02e42, 399 s, fails
`TestTheIceEdgeIsNotALineOfLatitude` and `TestTheWeatherChangesFromDayToDay`;
the branch passes both, in 387 s, and fails six, each at its edge:

| test | reading | range | why it is let stand |
|---|---|---|---|
| Hack exponent, globe | 0.6029 | 0.54-0.60 | at the edge, as it was at 0.6005 after T8 |
| Horton bifurcation ratio, small globe | 5.009 | 3-5 | 0.2% over |
| meander wavelength, small globe | 14.73 widths | 10-14 | some 21 reaches over 8 globes; one width is its noise |
| Hack exponent, 2x less 1x, small globe | 0.079 | -0.05-0.05 | the resolution comparison moved with the plates; not yet looked into |
| channel concavity, 2x less 1x, small globe | 0.077 | -0.1-0.1 | its known gap (B) closed; the marker is taken off |
| `TestAHistoryLeavesItsBedsInLayers` | 49% layered | 50% | one seed: seeds 1-8 average 0.521 (0.538 before), and seeds 4 and 7 were under 50% before |

Merged by the owner's word with these standing.

---

## 2026-09-16 - Phase 3, step 2: a plate lags a tile of its grid, and opens less floor

**What this is.** The move on a half-size history against the map's, globes
1-8 made both ways, read through `epochWatch` each epoch; a throwaway test on
`claude/history-km`. The lag is how far a plate has travelled and not yet
moved - `cr.acc`, what is left over below a whole tile of its grid - in the
map's tiles; fresh floor is the share of the planet `move` opened that epoch.

| epoch | fresh floor, map / half | t | lag, mean over plates, map / half | t | plate speed, map tiles an epoch |
|---:|---|---:|---|---:|---:|
| 0 | 0.0414 / 0.0369 (-11%) | -3.5 | 0.65 / 1.43 | 21 | 5.33 / 5.33 |
| 4 | 0.0365 / 0.0367 (+1%) | 0.2 | 0.76 / 1.31 | 12 | 4.20 / 4.20 |
| 8 | 0.0291 / 0.0266 (-9%) | -1.8 | 0.71 / 1.48 | 17 | 3.06 / 3.06 |
| 10 | 0.0261 / 0.0231 (-11%) | -3.4 | 0.74 / 1.50 | 13 | 2.49 / 2.49 |
| 13 | 0.0185 / 0.0166 (-10%) | -3.0 | 0.75 / 1.42 | 30 | 1.64 / 1.64 |
| 15 | 0.0138 / 0.0135 (-2%) | -0.5 | 0.71 / 1.42 | 15 | 1.07 / 1.07 |
| all | 0.477 / 0.451 (**-5%**) | | 0.72 / 1.41 (**+95%**) | | 51.2 / 51.2 |

The plates go as fast on both grids, to the digit. What they have not yet
moved is a tile of their grid's worth, whatever the grid: 0.72 of the map's
tiles on the map and 1.41 on a half-size history, every epoch. The floor a
parting opens comes 5% short over the history, 11% in the first epoch and
9-11% in the slow late epochs, where a plate's step is a larger share of
what it travels. That is the first move's missing floor of the step before,
and the rifts and islands short throughout.

The move is right in taking many steps an epoch: it moves every plate a tile
and settles, over and over, until the travel is used. What makes the grid's
size matter is that a step is a tile of the grid, and what is below one is
held back. The turn does not: it is read backwards once an epoch and each
tile keeps how far its crust stands off it (`cr.off`), so a slow turn still
goes round.

---

## 2026-09-16 - Phase 3, step 2: the boundary is read alike; the first move changes it

**What this is.** Whether a half-size history reads the same boundary
between plates as closing or parting as the map does, globes 1-16, read
through `epochWatch` at the first plates (nothing moved) and after the first
epoch; a throwaway test, on `claude/history-km`. Map tiles of boundary per
map tile of area; the difference and t are seed by seed.

**At the first plates, nothing moved**, edge by edge by the crust either side
and the sign of `closing`, and tile by tile as `tectonics` reads it
(`meetingAt`'s hardest neighbour, `liftOf`'s kind):

| | map | half | | t |
|---|---:|---:|---:|---:|
| continent-continent closing | 0.00192 | 0.00209 | +9% | 0.8 |
| continent-continent parting | 0.00182 | 0.00222 | +22% | 1.9 |
| continent-ocean closing / parting | 0.00498 / 0.00499 | 0.00499 / 0.00503 | 0% / +1% | 0.0 / 0.2 |
| ocean-ocean closing / parting | 0.00302 / 0.00321 | 0.00281 / 0.00268 | -7% / -16% | -0.8 / -1.9 |
| tiles read as collision / arc / trench / rift and islands | 0.00188 / 0.00246 / 0.00247 / 0.01275 | 0.00201 / 0.00246 / 0.00246 / 0.01241 | +7% / 0 / 0 / -3% | 0.5 / 0 / 0 / -1.0 |

`closing`, `meetingAt` and `liftOf` read the same boundary the same way on
both grids: the drift, the spin and the offsets are scale-free, and nothing
there differs beyond chance.

**After the first epoch:**

| | map | half | | t |
|---|---:|---:|---:|---:|
| continent-ocean closing | 0.00304 | 0.00378 | +24% | 3.5 |
| continent-ocean parting | 0.00130 | 0.00213 | **+64%** | **7.0** |
| ocean-ocean closing | 0.00514 | 0.00419 | -19% | -5.3 |
| ocean-ocean parting | 0.00746 | 0.00608 | -19% | -5.3 |
| continent-continent closing / parting | 0.00072 / 0.00047 | 0.00084 / 0.00049 | +16% / +4% | 0.8 / 0.2 |
| seams found: collision / arc / trench / rift and islands | 0.00115 / 0.00209 / 0.00177 / 0.01401 | 0.00145 / 0.00238 / 0.00206 / 0.01299 | +27% / +14% / +16% / -7% | 1.7 / 2.6 / 3.1 / -3.2 |

The boundary is about as long after the move on both grids (0.0181 and
0.0175), but what lies along it is not: on the coarser grid a continent
still meets ocean where on the map fresh floor has opened between them and
the plates meet ocean against ocean. The move is where they part: a plate
goes a whole tile of the grid at a time once what it has travelled comes to
one (`move`, `cr.acc`), so on a grid twice as coarse a slow parting has not
yet opened a tile of floor that on the map has opened one, and the floor a
parting makes comes a tile of the grid at a time. Next: the fresh floor made
each epoch on both grids, and the lag between what a plate has travelled and
where its tiles are.

---

## 2026-09-16 - Phase 3, step 2: where the half-size history's collisions part, epoch by epoch

**What this is.** When the collisions' extra ground on a half-size history
appears, over globes 1-8 made both ways, on `claude/history-km`. `epochWatch`
(historygrid.go), nil but for tests, is told of the first plates (as epoch
-1) and of each epoch's end; throwaway tests read it. No world moves:
`TERRA_DIGEST=check` passes. Boundaries are in the map's tiles of boundary
per map tile of area, so the two sizes read alike; t is the seed-by-seed
difference over its standard error.

**The first plates** (16 seeds, before anything has moved):

| | map | half-size | difference | t |
|---|---:|---:|---:|---:|
| plates | 32 | 32 | 0 | |
| continent | 0.432 | 0.460 | +6% | 1.1 |
| boundary, all | 0.0199 | 0.0198 | -1% | -0.5 |
| continent against continent | 0.0037 | 0.0043 | +16% | 1.5 |
| continent against ocean | 0.0100 | 0.0100 | 0% | 0.1 |
| ocean against ocean | 0.0062 | 0.0055 | -12% | -1.4 |

**Each epoch** (8 seeds; the book's ground raised most by a collision,
cumulative, and the seams of the epoch):

| epoch | book collision, map / half | t | collision seam, map / half | t | rift and island seam, map / half | t | plates standing, map / half |
|---:|---|---:|---|---:|---|---:|---|
| 0 | 0.021 / 0.037 | 3.0 | 0.0011 / 0.0017 | 2.7 | 0.0138 / 0.0130 | -1.6 | 29.0 / 28.3 |
| 2 | 0.030 / 0.050 | 2.8 | 0.0005 / 0.0009 | 1.8 | 0.0141 / 0.0121 | -3.8 | 27.4 / 24.8 |
| 4 | 0.037 / 0.055 | 1.9 | 0.0005 / 0.0004 | -0.1 | 0.0144 / 0.0121 | -4.7 | 27.6 / 25.3 |
| 8 | 0.045 / 0.069 | 2.2 | 0.0004 / 0.0005 | 0.5 | 0.0155 / 0.0124 | -4.9 | 28.4 / 25.4 |
| 12 | 0.050 / 0.079 | 2.6 | 0.0004 / 0.0008 | 2.5 | 0.0161 / 0.0131 | -5.0 | 28.8 / 26.4 |
| 15 | 0.053 / 0.087 | 2.8 | 0.0005 / 0.0009 | 2.7 | 0.0166 / 0.0136 | -5.7 | 30.1 / 27.5 |

Half of the collisions' extra ground is there at the end of the first
epoch: the first move turns a continent-against-continent boundary that is
16% longer (not beyond chance) into collision seams 61% longer, so on the
coarser grid more of that boundary is read as closing. The seams then run
alike for eight epochs and part again from the twelfth. The partings - rift
and island seams - are shorter on the coarser grid from the first epoch to
the last, by 13-18%, and it keeps some two and a half fewer plates standing
from the second epoch. What is left to read is the first move: how
`meetingAt` and `closing` read the same boundary on the coarser grid, and
the crust kinds `cr.kinds` gives its tiles.

---

## 2026-09-16 - Phase 3, step 2: a half-size history, eight globes

**What this is.** Whether the globe on a half-size history (512x256, belts
7.3 history tiles wide) is the globe on the map, over globes 1-8 made both
ways, on `claude/history-km` at f8c1438. A throwaway test; no code changed.
The difference is taken seed by seed (each seed's two worlds share their
first draws, not their bits) with its standard error over the eight.

| measure | map | half-size history | difference | se | t |
|---|---:|---:|---:|---:|---:|
| schist (share of land) | 0.062 ±0.003 | 0.087 ±0.013 | **+0.024** | 0.005 | 5.3 |
| basalt | 0.019 | 0.026 | **+0.006** | 0.001 | 4.8 |
| collision ground (book) | 0.053 ±0.016 | 0.087 ±0.028 | **+0.034** | 0.012 | 2.8 |
| granite | 0.337 | 0.358 | +0.021 | 0.025 | 0.8 |
| limestone | 0.282 | 0.270 | -0.012 | 0.015 | -0.8 |
| shale | 0.233 | 0.203 | -0.030 | 0.022 | -1.3 |
| land | 0.397 | 0.423 | +0.026 | 0.026 | 1.0 |
| ocean crust | 0.536 | 0.532 | -0.003 | 0.030 | -0.1 |
| rain, mm | 766 | 810 | +44 | 31 | 1.4 |
| forest | 0.217 | 0.229 | +0.012 | 0.008 | 1.5 |
| seconds | 55.9 | 27.9 | -28.0 | 1.9 | |

Schist, basalt and the collisions' ground are larger on a half-size history,
and not by chance: the collisions' by 64%. Seed 1, the one the step before
read, had them 7% apart and was the least of the eight (0.050 against
0.047); seeds 5 to 8 had them two to three times the map's. The rest -
granite, the sedimentary rock, land, the ocean floor, rain and forest -
hold within the spread. A half-size history halves the globe's time, and
does not yet make the same belts: the spread of a belt is not only a matter
of a belt a handful of tiles wide.

---

## 2026-09-16 - Phase 3, step 2: the collision belts spread on a coarse history

**What this is.** The first use of main's `cmd/zarr -stages` and `zarrdiff
-by` on the schist left over, on `claude/history-km` after merging main
(d1856d3). `TERRA_HISTORY_SHRINK=n`, read once at start-up, runs a history
on a grid n times coarser than the map, so that `cmd/zarr` can write a world
on one; nothing else changes and no world moves.

**What it found.** The book's `meeting` - the kind of meeting that raised
each tile most - counted in the ground stage's store of each world, seed 1:

| world, history grid | a belt, history tiles | collision | arc | islands | rift | no meeting |
|---|---:|---:|---:|---:|---:|---:|
| small globe, 256x128 | 5.2 | 2 079 | 3 786 | 3 098 | 10 420 | 13 309 |
| small globe, 64x32 | 1.3 | **4 592** | 3 024 | 2 384 | 10 544 | 12 144 |
| globe, 1024x512 | 14.7 | 24 426 | 70 586 | 74 957 | 171 649 | 181 664 |
| globe, 512x256 | 7.3 | 26 232 | 82 388 | 62 100 | 147 864 | 204 500 |
| globe, 256x128 | 3.7 | **40 400** | 75 088 | 47 632 | 161 264 | 198 544 |

(`zarrdiff -only book/meeting -by book/meeting -by-side b`; a belt's
reach is `beltOn` in the map's tiles over the coarseness.) Where a belt is
a few history tiles wide the ground a collision raises most spreads: by
65% on the globe at 3.7 tiles and 120% on the small globe at 1.3, and by 7%
on the globe at 7.3, inside one seed's noise. The seam is a whole tile of
the grid on either side of the line between the plates, and a belt's reach
is cut at whole tiles of it, so a belt resolved by a handful of tiles is as
wide as those tiles and not as its reach. The schist is the crushing in
that ground. This is a floor on how coarse a history can be for the belts it
raises, rather than a count still made per tile; the arcs and islands lose
ground to it.

---

## 2026-09-16 - Phase 3, step 2: the rock decided on the map, and the band share made exact

**What this is.** Two changes toward a coarser history making the same
rock, on `claude/history-km`. No world moves: `TERRA_DIGEST=check` and the
short tier pass.

**The rock on the map.** A history on a coarser grid no longer lays the foot
of its piles. Its book notes, for the melt, the arc's fire and the crushing,
whether a tile was covered whole or the most of it a band covered and how
far the tile's centre stood from the band's middle (`record.banded`), and
the amounts are the amounts a tile in the band takes. `settleRock`'s switch
is `cookFoot`; on the map it runs as it did, and on a coarser history
`layFeet` runs it on the map after the hand-down: of the map tiles under a
history tile, a band covering a share of it reaches that share, the nearest
the band's middle by the distance read between the history's tiles, and
each tile's foot is laid from the book less what does not reach it.
`TestABandLaidOnTheMapKeepsItsShare` holds a quarter-covered history tile to
four schist tiles of sixteen and a covered one to sixteen.

**The share made exact.** Counting it, the step before's `bandShare` was
wrong at the scale of one: set against the map's rule over the same four
small globes' histories, it gave an arc's crushing 54 501 tile-epochs to the
rule's 43 101, the arc's fire 103 100 to 81 197, a collision 19 138 to
17 907 and the melt 371 429 to 344 748, because a band with edges between
two tiles took a tile more than the rule. It now counts the map tiles a
coarse tile at distance A stands for, A c to A c+c-1, that the rule takes:
at a coarseness of one it is the rule, and the four sums came out equal.

**What it measured** (`TERRA_PLANET=1`, small globes 1-8, globes 1-4):

| world, history grid | granite | schist | basalt | limestone | shale |
|---|---:|---:|---:|---:|---:|
| small, 256x128 | 0.42 | 0.11 | 0.08 | 0.19 | 0.16 |
| small, 128x64 | 0.37 | 0.16 | 0.11 | 0.16 | 0.16 |
| small, 64x32 | 0.25 | 0.23 | 0.13 | 0.14 | 0.21 |
| globe, 1024x512 | 0.345 | 0.062 | 0.017 | 0.29 | 0.23 |
| globe, 512x256 | 0.338 | 0.085 | 0.025 | 0.25 | 0.24 |
| globe, 256x128 | 0.349 | 0.100 | 0.027 | 0.25 | 0.22 |

Against the step before (small schist 0.17 and 0.29, globe 0.085 and 0.110)
the rock on the map took the quarter-size small globe's schist from 0.29 to
0.23, and the exact share moved nothing further. The leak is upstream of
the rock. Read off the books (small globes 1-8, the share each making
covers, summed over tiles):

| history grid | any crushing | any fire | any melt | crushed past 6 km | fire over crushing |
|---|---:|---:|---:|---:|---:|
| 256x128 (on the coarse path at one) | 0.085 | 0.104 | 0.514 | 0.061 | 0.054 |
| 128x64 | 0.096 | 0.092 | 0.545 | 0.078 | 0.050 |
| 64x32 | 0.127 | 0.059 | 0.525 | 0.108 | 0.034 |

The bands themselves drift: an arc's crushing grows and its fire shrinks
with the coarseness, though each epoch's share is now the map's rule at
one, the boundaries per area of each kind of meeting hold (an earlier
count: collisions 0.0019, 0.0018, 0.0025 a map tile), and the arc's gap in
map tiles is the same at full and half size (3 on the small globe, 5.33 on
the globe). What is left to find: where on a coarser grid an arc's front and
its axis fall against the tiles (the seam is a tile of the grid either side
of the line between the plates, and a coarse tile of the axis band reads the
front's distance), and that a tile's book sums every epoch, so one coarse
tile that was front in one epoch and axis in another carries both.

---

## 2026-09-16 - Phase 3, step 2: the seam's bands read by the share of a tile

**What this is.** The schist found by the step before, on
`claude/history-km`. No world moves: `TERRA_DIGEST=check` and the short
tier pass.

**The leak.** The crushing that makes schist, the fire that makes granite
and the melt that lays basalt are written into bands beside a seam, a few
of the map's tiles wide (`axisWidth` 2, and an arc's front out to its
axis). The distance from the seam is counted in the history grid's tiles,
and the seam itself is two tiles thick, one on each plate, so a band was
some 6 map tiles wide on the map, 8 on a half-size grid and 16 on a
quarter-size one, where `axisWidth` cannot be narrower than a tile.

**The change.** `bandShare`: on a coarser grid a tile takes the share of
each band that lies across it - its reach from the seam in the map's
tiles, [away c, (away+1) c], against the band's in [lo, hi+1] - and the
crush, the fire and the melt are weighted by it; a lava bed and the rock's
epoch go to a tile at least half in the band. On the map the map's rule
runs as it was.

**What it measured** (`TERRA_PLANET=1`, small globes 1-8, globes now 1-4,
not a quiet machine; before the change in brackets, globes then over two
seeds):

| world, history grid | granite | schist | basalt | limestone | land | ocean crust | rain mm |
|---|---:|---:|---:|---:|---:|---:|---:|
| small, 256x128 | 0.42 | 0.11 | 0.08 | 0.19 | 0.38 | 0.53 | 648 |
| small, 128x64 | 0.375 (0.36) | 0.17 (0.19) | 0.10 (0.11) | 0.16 | 0.36 | 0.55 | 702 |
| small, 64x32 | 0.25 (0.25) | 0.29 (0.32) | 0.11 (0.11) | 0.13 | 0.41 | 0.44 | 679 |
| globe, 1024x512 | 0.345 | 0.062 | 0.017 | 0.29 | 0.41 | 0.52 | 750±130 |
| globe, 512x256 | 0.346 (0.34) | 0.085 (0.10) | 0.021 | 0.25 | 0.38 | 0.58 | 805±110 |
| globe, 256x128 | 0.352 (0.31) | 0.110 (0.14) | 0.023 (0.05) | 0.25 | 0.42 | 0.52 | 804±54 |

The globe's granite and basalt now hold; schist is a third lower but still
grows with the coarseness, and the small globes barely moved. What is left
is the threshold: a tile is schist once its crushing passes `madeEnough`,
six kilometres, and a collision raises tens of kilometres an epoch, so a
coarse tile a quarter in the band is crushed past it and is schist whole.
The share is weighted and the rock it makes is still counted by the tile.
The globe's rain over four seeds is inside the spread at every size.

---

## 2026-09-16 - Phase 3, step 2: the history's tiles counted against the planet

**What this is.** Step 2 of the history grid, on `claude/history-km` from
main at 91daf2d. No world moves: `TERRA_DIGEST=check` passes and the short
tier passes.

**What changed.** A history grid remembers the Span of the map it is the
planet of (`Grid.planet`), and `historygrid.go` reads the history's counts
through it: `planetSpan` for the plate and hotspot counts, `coarseness`
for how many map tiles a history tile is, `inTiles` for a length counted in
map tiles (never under one tile), `passes` for a softening's passes (over
the coarseness squared). Through them: `plateTotal`, `driftScale` (and so
`deepSpan`, which grows with the coarseness), the molten era's cells, the
crust's fray, the bow's octaves, the belts' grain, `seamLeast`, `axisWidth`,
`foldWave`, `marginRamp`, `smoothing` in `soften` and `upliftOf`, the
hotspot count. On the map every one is the number it was to the bit.

**What it measured** (`TERRA_PLANET=1 go test -run TestAPlanetOnCoarserHistories`,
mean ± spread over seeds, small globes 1-8 and globes 1-2; the machine was
not quiet for the globes, so read their seconds as relative):

| world, history grid | s | plates | land | ocean crust | belts | rain mm | forest | granite | schist | basalt | limestone | shale |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| small, 256x128 | 4.1 | 16 | 0.38±0.05 | 0.53±0.05 | 0.28±0.03 | 648±55 | 0.21 | 0.42 | 0.11 | 0.08 | 0.19 | 0.16 |
| small, 128x64 | 2.6 | 16 | 0.36±0.06 | 0.55±0.07 | 0.31±0.03 | 703±58 | 0.22 | 0.36 | 0.19 | 0.11 | 0.15 | 0.16 |
| small, 64x32 | 1.5 | 16.1 | 0.41±0.03 | 0.44±0.05 | 0.32±0.04 | 679±52 | 0.21 | 0.25 | 0.32 | 0.11 | 0.11 | 0.17 |
| globe, 1024x512 | 69 | 29.5 | 0.45±0.05 | 0.48±0.05 | 0.31±0.02 | 671±130 | 0.20 | 0.36 | 0.06 | 0.02 | 0.25 | 0.24 |
| globe, 512x256 | 38 | 29.5 | 0.39±0.06 | 0.58±0.06 | 0.30±0.01 | 837±140 | 0.23 | 0.34 | 0.10 | 0.02 | 0.24 | 0.22 |
| globe, 256x128 | 23 | 31 | 0.40±0.03 | 0.54±0.03 | 0.29±0.00 | 835±45 | 0.23 | 0.31 | 0.14 | 0.05 | 0.20 | 0.23 |

What holds: the plate count (the quarter-size globe had 31 against 27
before this step), the land share, the belts, the forest, the shale and the
sandstone, within the spread between seeds. What does not: **the rock**.
Schist doubles at half size and doubles again at a quarter on both worlds,
at granite's and limestone's expense, and basalt rises. Schist is ground
buried and squeezed; something that decides how deep a tile is buried or
how much a meeting crushes is still counted per tile. The globe's rain at
half size (837 against 671) is inside two seeds' spread and wants more
seeds before it is believed. That is the next thing step 2 finds before a
history size is chosen.

---

## 2026-09-16 - zarr/ leaves for github.com/LukasSelin/zarr

**What this is.** `zarr/` moved to its own repository,
[LukasSelin/zarr](https://github.com/LukasSelin/zarr), with its history
(`git subtree split`), and is tagged `v0.1.0`. `cmd/zarr` now requires
`github.com/LukasSelin/zarr v0.1.0` in place of the `../../zarr` replace.
Nothing in the root package imported it, so no world moves: no digest,
budget or yardstick run is owed.

**Checked.** `go test -short ./...` in the new repository; `go vet ./...` and
`go test -short ./...` in `cmd/zarr` against the published tag. Entries
below that name `zarr/` mean that module, now in its own repository.

---

## 2026-09-16 - What cmd/zarr's export holds beside the world

**What this is.** `cmd/zarr` (merged 6ede026) had only been timed, on the
1024x512 globe. A `-max` world is sized to fill free memory, so what the
export holds on top of the land is what decides whether such a world can be
written at all. On `claude/zarr-export-memory`; `cmd/zarr/export.go` only
(`run`, `put`, `strata`), nothing in the root package or in `zarr/`.

**How it is measured.** `TestExportPeak` in `cmd/zarr/memory_test.go`
(`TERRA_ZARR_PEAK=WxH`, or `max`; `TERRA_ZARR_HISTORY=<file>` keeps the
world's history so a second run skips the making) makes a globe, collects,
and then exports it into a store that keeps nothing, at 1, 4 and 24
goroutines. A sampler reads `/memory/classes/heap/objects` every
millisecond, as the root's budget test does; "peak" is the highest reading
over the one after the collection before the export. It is read twice:
at the default GOGC, where it is mostly the garbage the pacer lets pile up
(6.8 GiB allocated for a 41 MiB store on the globe, almost all of it in
`zarr/`), and at `GOGC=5`, where it is close to what the export truly
holds. The "mapped" reading (runtime memory not released to the OS, the
nearest to RSS Go gives) is logged too, but it mostly reflects what the
runtime already held from the making, and swings from 0 to 4 GiB between
runs, so it is not tabled. `BenchmarkExport` reports `peak-B/op` for a
512x256 globe: 567 MiB at 24 goroutines, default GOGC, 0.25 s/op.

Machine: AMD Ryzen 9 3900X (12 cores, 24 threads), 64 GiB, Windows 11.
Every reading below was taken with the CPU at 1-12 % before and after.
A first set was spoilt by another session's `zarrexport` and 25 `zarr.test`
processes holding the CPU at 100 % (making took 31 s, not 12), and was
thrown away.

**What it found.** On the 1024x512 globe the land holds 424 MiB. Before the
change, the export held 355-362 MiB of its own at 24 goroutines (85 % of
the land) and reached 1.29-1.62 GiB at the default GOGC. Three things made
up what it held:

- every running job builds a whole copy of the map, up to 8 B a tile, and
  `zarr.Write` fills a whole shard, 1024x1024 elements whatever the map, and
  encodes it; at 24 jobs at once that is 24 copies;
- `strata` built its four arrays up front, 57 B a tile (BedsMax 8), and held
  them until the last job finished;
- every closure stayed reachable from `e.jobs` until `run` returned, so
  terrain, the Köppen types (16 B a tile) and the book's records (24 B a
  tile) outlived their jobs.

On a -max world the copies grow with the map and the shards do not, so the
first two dominate.

**What changed.** The store is byte-for-byte as before: sha256 of every key
matched main's code for valley, ancient and a 256x128 globe, each at chunk
16/shard 2/gzip 1, the defaults, 64/unsharded/uncompressed and 32/3/0 (so
shards cut by the map's edge are covered), and
`TestTheSameWorldWritesTheSameStore` passes unchanged.

- `run` starts jobs in order when there is a free processor *and* the bytes
  they declare fit in 256 MiB + 16 B a tile (a job bigger than that runs
  alone). `put` declares the map's elements plus a shard twice; the first
  bound keeps every job side by side on the globe.
- The strata arrays are no longer built: each job reads the piles again and
  writes a shard at a time through `zarr.Write` with a start and a shape,
  holding a shard three times (built, filled, encoded). The rock legend's
  top value comes from one pass over the piles when the job is queued.
- A job is dropped from `e.jobs` when it starts.

Peak MiB over the land, two runs each where two are given:

| world | goroutines | before, GOGC 100 | after, GOGC 100 | before, GOGC 5 | after, GOGC 5 |
|---|---:|---:|---:|---:|---:|
| 1024x512 (424 MiB) | 1 | 675, 718 | 603, 591 | 125, 117 | 99, 99 |
| | 4 | 780, 749 | 642, 664 | 155, 154 | 138, 146 |
| | 24 | 1289, 1394 | 877, 941 | 362, 355 | 215, 208 |
| 4096x2048 (6286 MiB) | 1 | 9811 | 7998, 8785 | 1601 | 870 |
| | 4 | 9654 | 8931, 8836 | 1629 | 1146 |
| | 24 | 10611 | 7647, 7444 | 2190 | 1049 |

Export time, same runs, default GOGC: globe 1.71-1.72 / 0.64-0.66 / 0.67-0.72 s
(1 / 4 / 24) before, and 1.78-1.79 / 0.66-0.67 / 0.67-0.80 s after, which is
within noise. 4096x2048: 24.1 / 9.0 / 9.2 s before, and 21.6-24.0 / 8.5-9.1 /
8.7-9.1 s after. At GOGC 5 the globe at 24 goroutines went from 0.90 to
1.07-1.08 s, because the jobs wait on bytes when the collector is also busy.

At 4096x2048 and 24 goroutines, what the export truly holds fell from 35 %
to 17 % of the land, about 131 B a tile. That is the 256 MiB + 16 B a tile
the jobs may hold, plus the Köppen types and book records built up front
(~44 B a tile, in `climate` and `book`, left alone because another session
is editing those definitions), plus gzip state.

**Not measured: a real -max world.** `Largest` would size one at about
63 M tiles here (7.5x the 4096x2048 world, hours to make), and the reading
above says it would not have fit even before the export: a kept
4096x2048 land holds 6286 MiB, 786 B a tile, while `Largest` budgets
`bytesRun` = 640 B a tile for the making's peak. That is for the root
package to look into; nothing here touches it. The per-tile figures above
scale linearly, since the shard-sized parts are fixed.

**Left for others.**

- *Default GOGC.* At GOGC 100 the peak is about the land's size again,
  whatever the export holds, because the heap goal is twice what is live.
  `main` sets no memory limit, so a -max export needs `GOMEMLIMIT` (or `main`
  setting `debug.SetMemoryLimit` from the budget `Largest` used) before any
  of this matters.
- *`zarr/` (not changed, another session owns it).* The 90 GiB allocated
  for a 582 MiB store is churn inside `zarr.Write` and its codecs: a fresh
  `filled` buffer of the whole shard grid per shard (the full 1024x1024
  even when the map is 512 tall), a fresh gzip writer per chunk, an
  `extract` copy per chunk in `ShardingCodec.EncodeArray`, and `body`
  grown by append. Pooling the gzip writers and the shard buffer (or a
  `Write` that takes a caller's buffer) would cut both the churn and the
  pacer peak. A write that encodes chunk by chunk from a source function
  would let `put` stop building whole-map copies too.
- *`climate` and `book`.* Building the Köppen types and records inside
  their jobs instead of up front saves ~40 B a tile.

**After merging main (abaf1ee).** Main had since kept gzip writers in
`zarr/` (6fe433e), made the Köppen codes a fixed table built inside their
job, given the strata a `noBed` fill, and consolidated the metadata
(so the discarding store in `memory_test.go` now keeps `zarr.json` keys).
The change was redone over that; the store is byte-identical to main's
code again (sha256 of all 4424 keys, same worlds and options as above).
Globe, quiet machine (CPU 0 % before and after), two runs each, peak MiB
over the land (428 MiB) at 1 / 4 / 24 goroutines, and time at 24:

| | GOGC 100 | GOGC 5 | time at 24, GOGC 100 |
|---|---|---|---:|
| main | 532, 532 / 568, 552 / 984, 974 | 107, 104 / 163, 139 / 428, 446 | 0.33, 0.32 s |
| this branch | 490, 487 / 518, 511 / 748, 771 | 95, 97 / 129, 130 / 222, 245 | 0.33, 0.32 s |

Allocation fell from 6.8 to 1.2 GiB with the pooled gzip writers, so the
first `zarr/` item above is partly done. The whole-shard `filled` buffer
and the per-chunk `extract` copy are still there.

---

## 2026-09-16 - the deep floor at GDH1's depths

**What this is.** On `claude/missing-yardsticks-simulation-b40e20`.
`floorDepth` (`abyss.go`) is Stein and Stein's GDH1 (1992), 2600 + 365
sqrt(t) m to 20 Myr and 5651 - 2473 exp(-0.0278 t) past it, where it was
Parsons and Sclater's (1977), which comes toward 6.4 km on old floor where
GDH1 comes toward 5.65. With the earth's ages the old floor is most of the
ocean. `sinksPastCCD` is the curve turned round: 27.5 Myr (was 32.7).

**Readings.** Globe, seed 1: oceanic hypsometric mode -5.375 km (-5.625),
still a gap, a sixth of the deep floor at 5.25-5.5 km; ridge 2.77 km (2.51),
subsidence to 70 Myr 317 m/sqrt(Myr) (347), flattening 0.19 (0.48), all in
range; sediment 773 m on the mean (797). Every other reading is the
sediment commit's: the land does not move, since the floor is out of
`meanHard`.

**Held.** `go test -short`, `cmd/zarr` tests, the yardsticks: no failures.
Digest: `globe128` rewritten. `perf.sh check` not run.

---

## 2026-09-16 - sediment on the deep floor, and the floor out of the land's mean hardness

**What this is.** On `claude/missing-yardsticks-simulation-b40e20`, after
main was merged in for the zarr experiment loop. The deep floor carried some
twenty metres of sediment: what the history lays on it is squeezed with its
other beds into the map's spread of heights. `floorSediment` (`abyss.go`)
gives it what its age gathers: calcareous ooze at 1 cm/kyr under a warm sea
while the floor is above a 4.5 km compensation depth, red clay at 1 mm/kyr,
and a turbidite apron off the continents (1.5 km at the slope's foot, e-fold
300 km, laid over 20 Myr; chosen). The floor stands 0.57 of it higher, for
the load. `layAbyss` lays what the pile lacks as limestone and shale in the
order they came. `cmd/zarr` writes `ground/floor_age` and
`ground/floor_sediment`.

`seafloorSubsidence` reads the basement, the sounded depth with the
sediment's lift put back, as Parsons and Sclater's depths were: read at the
sediment's top, flattening came out 0.63.

**What the zarr loop found.** With the sediment, the small globes' Flint R2
(0.71), Hack exponent (0.523) and Hack at 2x less 1x (0.101) failed. A globe
before and after, `-stages` and `-stages-diff`: the first stage differs at
ground, on the floor only; by shape, 95 % of land tiles had moved by up to
10 m. `meanHard` averaged the rock's hardness over every tile, and the floor
turned from basalt to limestone and shale softened the land's mean. The
deep floor, which the weather does not reach, is now left out of it. That
moves the land once; every yardstick then passes.

`TestTheTideLaysFlatsOnlyWhereItReaches` reads flats off small globe 2 and
not 3: the tide reads the sea's depth, and the six small globes hold none to
two flats each.

**Readings.** Globe, seed 1: sediment on ocean crust 797 m on the mean, 745
on the deep floor; the floor's top rock 28 % limestone. Ridge 2.51 km,
subsidence 347 m/sqrt(Myr), flattening 0.48, all as before. The oceanic
hypsometric mode is -5.625 km (was -5.875), still a gap. Gap readings moved
by the land's move: small-globe concavity 0.312 (0.294), its 2x less 1x
0.151 (0.199), valley floor soil 0.41 m (0.48), floor over hillslope 1.79x
(2.11), Oxisols 0.023 (0.024).

**Held.** `go test -short`, `cmd/zarr` tests, and the yardsticks: no
failures. Digest: `globe128` rewritten; `valley` and `ancient`, which have no
deep floor, unchanged. `perf.sh check` not run.

---

## 2026-09-16 - the first plates' ocean floor has ages: flattening closes

**What this is.** On `claude/missing-yardsticks-simulation-b40e20`. The
first plates' ocean crust was all dated from the start of the history, so
half a globe's deep floor was 64 Myr old and 5.3 km down, and no floor was
old enough to flatten. `firstFloorAges` (`abyss.go`) now gives it the ages
it had before the history began: the earth's age-area law (Sclater and
others 1980; Parsons 1982), area falling linearly to nothing at 180 Myr
(Müller and others 2008) less the history still to come, ranked by distance
from the seams between ocean plates. The crust carries them (`crust.aged`),
`floorDepths` lays the floor by them, and the grid keeps each tile's age as
`floorAge`, which the history file keeps by reflection and `handDown` reads
by the nearest tile.

The ages do not feed the subduction. Letting the older of two first-plate
crusts sink first changed which crust went down, and with it every globe's
continents: the sea on the globe covered 6 % less, and the mean land carbon
(8.99), the small globe's Hack exponent at 2x less 1x (0.083), its discharge
exceedance exponent (0.481) and the tide flats of small globe 3 all failed.
Without it the land is the soil commit's to the bit, and only the deep floor
moves: the digest's `globe128` changes, and `valley` and `ancient`, which
have no ocean, do not.

`seafloorSubsidence` reads each tile's age off `floorAge`, in bins of an
epoch, and only on floor a shelf and a slope's width (230 km, six globe
tiles) from continental crust, which is the floor `floorDepths` lays at its
age's depth. Read over the margins too, the floor of 50 to 75 Myr, much of
it on the first rifts' margins, came out a kilometre shallow and the old
floor sank 0.83 as fast as the young.

**Readings.** Globe, seed 1:

| yardstick | before | now | real |
|---|---|---|---|
| ridge crest depth, km | in range | 2.51 | 2.0-3.0 |
| subsidence to 70 Myr, m/sqrt(Myr) | in range | 347 | 250-450 |
| flattening past 70 Myr | NaN (gap) | 0.48 | -0.2-0.6 |
| oceanic hypsometric mode, km | -5.375 (gap) | -5.875 (gap) | -5.0 to -3.8 |

The mode goes the wrong way, and stays a gap: with the earth's ages the
Parsons and Sclater depths heap up at 5.5-6 km. The earth's floor stands
shallower under its sediment and its plateaus and swells; a globe's carries
some twenty metres.

**Held.** `go test -short` passes. `TestRealNumbers|TestTheRealWorld`: no
failures, and every other reading is the soil commit's. `scripts/perf.sh
check` not run: the machine was loaded.

---

## 2026-09-16 - soil orders and soil carbon: six soil yardsticks close

**What this is.** On `claude/missing-yardsticks-simulation-b40e20`. Two
changes to the soil (workstream I), and the world moves: the digest is
rewritten.

- `carbonLevel` (`pedogenesis.go`): what grows into the soil is the Miami
  model's warmth term times its rain term, where it was West's runoff term
  and nothing below -5 C. The decay goes by a Q10 of 1.4 (Mahecha and
  others 2010) where it was 2, slows with drought as the rain term to 0.7,
  and runs at a fifth on permafrost. Half the carbon (0.8 on permafrost)
  lies in the litter and needs no mineral soil under it. The constants were
  fitted offline against the globe's forest, grass, desert and tundra.
- `SoilOrderOf` (`soilorder.go`, new): Soil Taxonomy's key asked of what a
  tile carries, for Gelisols, Histosols, Oxisols, Aridisols, Ultisols,
  Mollisols, Alfisols, Inceptisols and Entisols, and no soil.
- `carbonByBiome` reads the biome ratios over tiles with soil, as Jobbágy
  and Jackson's pits were dug; the land mean still counts bare ground at
  nothing. Half the desert tiles have no soil, so without this forest over
  desert reads 3.7.

**Readings.** Globe, seed 1:

| yardstick | main | this | real |
|---|---|---|---|
| carbon, forest over desert | 143x | 1.93x | 1.5-3.2 |
| carbon, tundra over desert | 0.19x | 1.81x | 1.5-3.2 |
| mean land carbon, kg C/m2 | 1.96 | 9.54 | 9-13 |
| Aridisols | - | 0.111 | 0.09-0.15 |
| Gelisols | - | 0.098 | 0.06-0.11 |
| Mollisols | - | 0.064 | 0.05-0.09 |
| Oxisols | - | 0.024 (gap) | 0.05-0.10 |

The rest of the land: no soil 0.125, Alfisols 0.397, Inceptisols 0.145,
Histosols 0.020, Ultisols 0.013, Entisols 0.004. Alfisols stand at half
again Earth's share and Ultisols and Oxisols at a fraction, for one reason:
the warm humid land's surfaces are a median of fourteen thousand years old,
too young to be leached.

**Held.** `go test -run 'TestRealNumbers|TestTheRealWorld' -timeout 60m .`
on this branch and on its base (312900f), side by side: no failures on
either, and every other gap reads the same. `go test -short` passes, the
budget test with it, so the heap budget is not rewritten. The digest was
checked on the base (it holds) and rewritten here: all three budget worlds
move, as fertility reads the carbon through `humus`. `scripts/perf.sh check`
was not run: the machine was loaded by other sessions.

---

## 2026-09-16 - cmd/overview: salt lakes drawn, and maps of the soil and of Köppen–Geiger

**What this is.** `cmd/overview` and `README.md` only, on
`claude/overview-maps-review-0119b3` off b137614. Nothing in terra is
touched: every world is as it was, and the digest, budget and yardsticks do
not see it. `go test -timeout 60m ./cmd/overview` passes.

**Fixed.** The terrain map had no colour for `Salt` and `Pan` (transparent
black); the biome and landform maps named them as a fresh lake, or the sea,
and as their climate. The greatest river was printed to the unit and read
0 m³/s on the seed 1 globe, where it is 0.062.

**Added.** Five maps (21 to 26): soil depth, soil chemistry (saline over
1 kg/m² salt, calcic over 25 kg/m² carbonate, leached at a fifth of the
bases gone, strongly at half, else base-rich), soil carbon, surface age
(`Exposed`, log scale 100 yr to 1 Myr) and the full Köppen–Geiger type in
Beck et al. (2018)'s colours. Drawing them adds nothing measurable beside
making the globe (56 s made).

**Seen while looking, not fixed.** On the seed 1 globe dry ground runs only
23-266 m and the greatest river is 0.062 m³/s; Woods and Soil texture carry
straight row-aligned edges near 15% and 85% of the height; Height shows
square blocks of shelf round small islands; Drainage hatches flats
diagonally. Rock age is two values (epoch 0 or 15) and is in epochs, not
years.

---

## 2026-09-16 - zarr/: fuzzed, held to what its metadata implies, and measured

**What this is.** `zarr/` only, on `claude/zarr-robust` off 3973917: no
feature added, no public API changed, nothing in terra or `cmd/zarr`
touched. Every world is as it was; the digest, budget and yardsticks do
not see this module.

**Limits** (1123a63). A store could make the module panic or allocate
without bound. Now an array does not open if its shape counts more
elements than an int, or if a chunk, a shard or a shard's index would be
more than 2 GiB (`maxStoredBytes`, the one constant: a chunk is made whole
when read, and a chunk never written is made of the fill). A gzip chunk
inflates to no more than its chunk spec implies, carried through the codecs
before it (the bytes codec exactly, crc32c +4, gzip +1% +1 KiB; an unknown
codec falls back to the 2 GiB). A shard's index must put every chunk inside
the shard, clear of the index and of every other chunk. Region ends and
`NumChunks` no longer overflow near the top of an int. `limit_test.go`
holds each; on main's code they were:

| case | main | branch |
|---|---|---|
| shape [MaxInt64, 4], `Read` | panic: makeslice len out of range | does not open |
| chunk shape [2^62, 2^62] | panic: slice bounds out of range | does not open |
| chunk shape [65536, 65536] int16 | opens and reads (8 GiB a chunk) | does not open |
| shard of 2^32 one-element chunks | 48 GB allocated at open (killed) | does not open |
| shape [MaxInt64] in chunks of 1024, `Read` of all; a region whose end overflows | panic: makeslice len out of range | error |
| a 16-byte chunk as 1 MiB of gzip zeros | inflates it whole (5.3 MB allocated), then errors | stops at 16 bytes |
| shard index entries overlapping, or the same chunk twice | read as data | error |

**Fuzzing** (2f9e5c2). Native Go fuzz targets: `FuzzMetadata` (array and
group zarr.json; what opens must write and reopen as the same metadata),
`FuzzOpenAndRead` (zarr.json and two fuzzed keys in a MemoryStore, a region
and a chunk read by range and with shards read whole), `FuzzBytesCodec`,
`FuzzGzipCodec`, `FuzzCRC32CCodec` (each round-trips what decodes),
`FuzzShard` (three sharding layouts; what decodes re-encodes to the same
elements) and `FuzzShardIndex` (`checkIndex` against a pairwise check).
The corpus is seeded from all twelve `testdata/interop` cases as this module
writes them, three more arrays, and the other tests. Each fails on more
than 256 MiB allocated per input, with chunks lowered to 64 KiB for the run.

| target | branch, 5 min | after the gzip pools, 4 min | main's code, 3 min |
|---|---:|---:|---:|
| FuzzOpenAndRead | 24.8 M execs, nothing | 11.5 M, nothing | **found**: chunk grid 6 x 8888888 uint32, 853 MB for a 16-element read |
| FuzzMetadata | 38.1 M, nothing | | 17.2 M, nothing |
| FuzzShard | 36.0 M, nothing | 31.0 M, nothing | 21.8 M, nothing |
| FuzzGzipCodec | 22.1 M, nothing | 18.0 M, nothing | (needs the limit) |
| FuzzBytesCodec | 28.0 M, nothing | | 27.1 M, nothing |
| FuzzShardIndex | 37.6 M, nothing | | (new) |
| FuzzCRC32CCodec | 45.2 M, nothing | | |

The one crasher is kept in `testdata/fuzz/FuzzOpenAndRead` (3dc1115). The
byte mutator rarely makes a JSON number huge, so the overflow panics above
were found by reading the code and are held by `limit_test.go`, not by the
fuzzer.

**Benchmarks** (fb5b111, `bench_test.go`). 512 x 1024 float64, chunks of
64, gzip 5, MemoryStore; shards are 4 x 4 chunks; the region is 3 x 3
across four chunks of one shard in a DirStore. AMD Ryzen 9 3900X, 24
threads, Windows 11, go1.27.0, `-count 6`, main's module and the branch
back to back with no other test running:

| benchmark | main time/op | branch time/op | main B/op | branch B/op | main allocs | branch allocs |
|---|---:|---:|---:|---:|---:|---:|
| Write, chunks | 101.2 ms | 81.2 ms (-20%) | 119.4 MiB | 20.6 MiB (-83%) | 4 868 | 2 709 |
| Write, shards | 100.8 ms | 89.6 ms (-11%) | 142.2 MiB | 44.5 MiB (-69%) | 4 573 | 2 443 |
| Read, chunks | 41.9 ms | 38.7 ms (-8%) | 35.9 MiB | 16.1 MiB (-55%) | 4 614 | 2 443 |
| Read, shards | 41.7 ms | 40.7 ms (-3%) | 35.8 MiB | 16.1 MiB (-55%) | 3 942 | 1 795 |
| ReadRegion (DirStore, shards) | 1.73 ms | 1.56 ms (-10%) | 1 025 KiB | 394 KiB (-62%) | 173 | 108 |
| ReadChunk, chunks | 317 µs | 294 µs (~) | 255 KiB | 97 KiB (-62%) | 36 | 19 |
| ReadChunk, shards | 326 µs | 302 µs (~) | 256 KiB | 98 KiB (-62%) | 42 | 28 |
| WriteChunk, chunks | 783 µs | 658 µs (-16%) | 923 KiB | 132 KiB (-86%) | 36 | 19 |
| WriteChunk, shards | 18.0 ms | 17.2 ms (~) | 21.7 MiB | 7.2 MiB (-67%) | 992 | 468 |

(~ is benchstat's no significant difference at p < 0.05.) The waste B/op
pointed at was gzip: a new
`gzip.Writer` for every chunk (most of a megabyte of compressor state) and
a new reader, inflating through `io.ReadAll`'s doublings. 6fe433e keeps
writers per level and readers in `sync.Pool`s and inflates into one buffer
sized from the gzip trailer, held to the chunk's bound. A reset writer
writes what a new one does (`TestAKeptGzipWriterWritesWhatANewOneDoes`),
and the store `cmd/zarr` writes for seed 3 is the same byte for byte, all
1 406 keys under four sets of options (chunk 16 shard 2 gzip 1; 64, 0, none;
32, 4, 5; 16, 0, 9), before and after. Not changed: a sharded WriteChunk
still decodes and re-encodes its whole shard, as zarr-python does, and the
ranged read already fetched only the index and the chunks it needs.

**Checked.** `go test ./...` and `go vet ./...` in `zarr/`;
`TestZarrPython` against zarr-python 3.4.0 and numpy 2.5.3; `cd cmd/zarr &&
go test -short ./...`, and `TestTheSameWorldWritesTheSameStore`.

---

## 2026-09-16 - zarrdiff: signed change, by cause, expectations

**What this is.** On `claude/zarrdiff-signed`, inside `cmd/zarr/zarrdiff`
only: zarrdiff said how much an array changed as `|a-b|` over the whole
map; now it says which way, where by cause, and whether that is what the
change was meant to do. No world, the digest, the budget or `perf.sh`
moved: nothing outside `cmd/zarr/zarrdiff` changed but this entry, and the
yardsticks were not run.

**What it adds.**
- *Signed change* for arrays of amounts: mean, least, most, 5/50/95th
  percentiles of `b-a` over the changed elements, and how many went up and
  down. The percentiles come off a fixed histogram (32 bins an octave of
  `|b-a|`, 2^-64 to 2^64 each side of zero, 64 KiB), within 1.1% of the
  sorted value and clamped to the exact least and most.
- `-by group/array` (repeatable, `-by-side a|b`): every map-shaped array's
  changes by the category of each tile in a map of codes. A coded map
  names every category from its CF flags; a map of feature ids lists the
  `-top` N by tiles changed, reading the array a second time to bin just
  those. `-only` limits the arrays; `-mask group/array=code[,code]` limits
  the tiles.
- `-expect file.json`: checks of `changed`, `tiles`, `share`, `mean`,
  `p5/p50/p95`, `up`, `down` or `code` from/to, on an array or within a
  `where` of a map's codes, with `min`/`max`/`above`/`below`. Exit 0 all
  hold, 3 one does not; 1 and 2 as before.

**Measured.** Globe seed 1 (1024 by 512, 83 arrays), `-water 7.5`
(default) against `-water 8`, both exported from this branch; Ryzen 9
3900X, 24 threads, other sessions loading the machine. Wall times
interleaved with main's zarrdiff built from a temporary worktree:

| run | wall |
|---|---|
| main's zarrdiff, plain | 3.8, 3.9, 4.2 s |
| this branch, plain | 4.0, 3.8, 3.8 s |
| `-by book/meeting`, before the chunk cache | 29 s |
| `-by features/belt`, before the chunk cache | 26 s |
| `-by book/meeting`, with the cache | 4.3 s |
| `-by features/belt`, with the cache | 6.4 s |
| four `-by` (meeting, koppen, terrain, belt), with the cache | 4.9 s |
| `-expect` of 7 checks, `-only book/meeting` | 0.4 s |

Plain runs match main. (The 0.9 s of the entry below was on a quieter
machine.) The first `-by` build re-read the map for every block of every
array. A CPU profile put 94% of the time in `cgocall`, nearly all of it
file `Close` in the directory store, from 24 goroutines opening the same
map's shards. The decoded chunks of the `-by`/`-mask`/`where` maps are now
read once and shared between the arrays, at most `budget × processors`
codes of 8 bytes held, the oldest dropped first. Peak working set, polled
from PowerShell, was too noisy to compare: main's plain run read 113 MiB
once and 1.2 GiB another time. The four `-by` run read 525 MiB once. Treat
those as unmeasured.

**What it showed about `-water 8`.** Checks written down first: history
untouched (holds, `book/meeting` 0 changed); sea rose (holds, 1 383 open
to water); did not fall back (fails, 397 water to open); Köppen share ≤ 2%
(holds, 1.99%); collision belts' height unchanged (fails, 71% changed);
dry ground not lowered on average (fails, mean -0.23 m); plate ids kept
(fails, every tile's id down by 106, as the features numbered before
plates changed). By `tile/terrain`: every open and wood tile's height
moved, median +1e-5 m, 5-95% from -8.8 to +6.2 m. Half a metre of water
reaches the ground's wearing everywhere, not just the shore. That is a
finding for the water stage, not a fault in the tool. The worked example
in `cmd/zarr/zarrdiff/README.md` is this run.

**Held.** `cd cmd/zarr && go test -short ./...`. New tests on small stores
written with the zarr package: histogram percentiles against a sort over
three magnitudes; signed change on a known tweak (+10 on a collision, -1
to -10 on a rift); `-by` a coded map and a map of ids with `-top`, the same
report at `-budget 1` (the cache evicting on nearly every read), `-by-side b`,
and the maps `-by` refuses; `-only`, one and two `-mask`s, masks by name
and number; `-expect` all holding (exit 0), failing (3), a check that
cannot measure, and malformed files and unknown code names (2).

---

## 2026-09-16 - cmd/zarr experiment loop: kept histories, every term, a store per stage

**What this is.** On `claude/zarr-experiment-loop`: `cmd/zarr` gains
`-keep-history` and `-from-history` (as `cmd/overview` has them),
`-wetness`, `-woods`, `-growth`, `-glacial` and `-terms file.json`, and
`-stages dir`, which writes a store at the end of every stage, with
`-stages-diff a b` to name the first stage two experiments differ at. The
recipe is the "experiment loop" section of `cmd/zarr/README.md`.

**The root package.** One hook and nothing else: `StageWatch`, a
`func(stage string, l *Land, g *Grid)`, which `generateFrom` calls after
each stage when it is not nil; `Stages()`, the names; and
`MakeLandWatching(seed, t, history io.Writer, watch)` and
`LandFromHistoryWatching(in, watch)`, of which `MakeLandKeepingHistory` and
`LandFromHistory` are now the nil-watch cases. `Generate` passes nil: an
unwatched making does one nil compare per stage and allocates nothing more.
`TestAWatchedWorldIsTheSameWorld` holds that a watched world, from the
plates and from its history, is NewLand's, and that both see the same
grid at every stage.

**Digest.** `TERRA_DIGEST=write` on the base commit (312900f) rewrote
`docs/perf/digest.json` to the bytes already committed; `TERRA_DIGEST=check`
after the change passes. No world moved.

**Budget.** `TestWorldCreationBudget` passes unchanged; not rewritten.
`perf.sh` and the yardsticks were not run: nothing in the root package but
the hook changed. `go test -short -timeout 60m .` passes (95 s).

**Demonstrated.** Ryzen 9 3900X, 24 threads, other sessions on the machine,
so the times are indicative. A globe (`-preset globe`, 1024 by 512):

| run | making | writing |
|---|---|---|
| made, `-out` | 76.3 s | 1.4 s |
| made, `-keep-history` (198 MiB file) | 72.4 s | 1.9 s |
| `-from-history`, twice | 17.5 s, 16.3 s | 1.8 s, 1.1 s |
| `-from-history -stages` | 22.5 s with the six stores (0.6-0.9 s each) | |
| made, `-stages` | 93.8 s with the six stores | |

A globe re-exported from its history takes the later stages' 16-17 s, a
quarter of the 72-76 s of making it. `zarrdiff` finds the store made from
the history the same as the made one in all 83 arrays, and `-stages-diff`
finds every stage's store the same whether the world was made from the
plates or from the kept history, the ground stage included.

---

## 2026-09-16 - zarrdiff: where and by how much two worlds differ

**What this is.** On `claude/zarrdiff`: `cmd/zarr/zarrdiff`, a command in
the `cmd/zarr` module that compares two stores `cmd/zarr` wrote. The digest
says whether a change moved a world; this says which fields moved, over how
much of the map, by how much, and where, as evidence to set beside the
yardsticks. Recipe and output in `cmd/zarr/zarrdiff/README.md`. Nothing
outside `cmd/zarr` changed besides this entry, and nothing in `zarr/`,
`main.go` or `export.go`: every world, the digest and the budget are main's,
and `perf.sh` and the yardsticks were not run.

**What it does.** Walks both directory stores for `zarr.json` (the store
interface cannot list); lists the arrays and groups in one alone; compares
group attributes; for each array in both, shape, type, attributes, then
elements: count changed with NaN equal to NaN, max and mean `|a-b|`, share
of tiles changed (beds folded into their tile), the bounding box in y/x,
and for arrays with a legend the commonest code changes named from each
side's legend. Text by share of tiles changed, `-json`, `-png dir`. Exit 0
the same, 1 different, 2 error.

**Demonstrated.** One run, Ryzen 9 3900X, 24 threads, other sessions on
the machine. Three globes (`-preset globe`, 1024 by 512, 67 arrays, 41 MiB
each) exported from this branch, whose world code is main's (3973917):

| | exit | wall | peak working set |
|---|---|---|---|
| seed 1 against seed 1, made and written twice | 0 | 1.3 s | ~120 MiB |
| seed 1 against seed 2 | 1 | 0.9 s | ~130 MiB |

Seed 1 made twice is the same store in every element, a second check,
from outside the package, of what the digest holds. Seed 2 moves 60 of 67
arrays: the climate means and rain on every tile (`climate/mean` max 41 °C,
mean 19 °C), `ground/height` on 96 % (mean 2 927 m), `tile/terrain` on 58 %
(commonest: 76 755 water to open, 55 583 ice to open), `features/lake` on
2 %, and all 15 columns of `features/table` not compared, as the tables are
28 576 and 27 820 features long. A seed is the loudest change there is; an
algorithm change is expected to light a few arrays over part of the map.

**Memory.** Arrays are read in blocks of whole chunks: one chunk high and as
wide as `-budget` (2^20 elements) allows, one array a processor, so the most
held is about 2 x 2^20 x 8 B x 24, some 400 MiB, however large the world;
the two globes whole would be several hundred MiB. Not run on two `-max`
worlds, which take this machine's memory to make.

**Held.** `cd cmd/zarr && go test -short ./...`: the stores written with
the zarr package (identical; one element and one code changed, at a budget
of one element and of 2^20; NaN fills against numbers; a shape mismatch;
an array in one store alone; attributes; arguments that cannot be
compared), and `TestZarrdiffSeesTheSameWorldAsTheSame`, which builds the
command, exports an ancient world twice with `export` and another seed
once, and expects exit 0 and 1.

---

## 2026-09-16 - cmd/zarr stores that xarray reads: coordinates, CF flags, stable Köppen codes

**What this is.** On `claude/zarr-xarray`, off main at 3973917: the first
time xarray was pointed at a store, and the fixes to `cmd/zarr` for what
it found. Only `cmd/zarr` changed; `zarr/` and the root package did not, so
every world, the digest and the budget are main's.

**What xarray reported before.** xarray 2026.7.0, zarr-python 3.4.0,
numpy 2.5.3, on the seed 1 globe (1024 by 512) and the default valley,
both written by main's command:

- No errors, and every array's values came back equal to zarr-python's.
  The valley has no `book` group, which is right for a drawn map.
- Every default `open_zarr` and `open_datatree` warned: *Failed to open
  Zarr store with consolidated metadata* (RuntimeWarning).
- No coordinates: `Dimensions without coordinates: y, x`, so no `.sel` by
  place, and no tile size anywhere in the store.
- `to_netcdf` of `tile` or `climate` failed: `Invalid value for attr
  'legend'` (a JSON object). The whole datatree's failed with `Object
  dtype dtype('O') has no native HDF5 equivalent`, and h5netcdf refuses a
  bool attribute such as the root's `wrap`.
- Misdecoded meaning, not bits: `strata/rock` and `strata/formed` held 0
  past the bottom of a pile, which reads as granite laid in epoch 0 (70 %
  of the globe's beds); `climate/koppen` numbered only the types present,
  so code 9 was `Cfb` in the globe and nothing in the valley (whose
  `Cfb` was 1); the legends of the other coded arrays stopped at the
  largest code present.
- xarray does not use a Zarr v3 `fill_value` as a mask unless told to,
  so the only masking it would do is by a `_FillValue` attribute.

**What changed.** `y`/`x` coordinate arrays (float64 metres of the tile
centre, index times `terra.TileSpan`) in every group of the map, `bed` in
strata and `feature` (the id) in the features table; `long_name` for
`about` and UDUNITS spellings of units; `flag_values`/`flag_meanings`
over every code the type has, instead of `legend` (dropped: a dict attribute
cannot go to netCDF, and two tables of one thing can disagree); one fixed
table of the 27 Köppen types `koppenCode` can give, checked by a sweep of
`KoppenOf`, with an export error for a type not in it; `_FillValue` 255
(and fill 255) on `strata/rock` and `strata/formed`; `scale_factor` on
`leached`, `lime`, `salt` and `strata/sand`; the root's `terms` as JSON
text, `wrap` as 0/1, and `tile_span`; and the root's metadata written
again last with every node's inline under `consolidated_metadata`
(`must_understand: false`), as zarr-python consolidates v3. Every array
keeps the world's type. `cmd/zarr/README.md` documents the layout.

**What xarray reports after.** On both stores, opening and loading every
group with defaults raises no warning (also under `python -W error`), every
array decodes to zarr-python's elements with its scale and mask applied,
`.sel(x=, y=)` finds tiles by metres, `cf_xarray` sees the coded arrays as
flag variables (`koppen.cf == "Cfb"` works), and the whole datatree writes
to netCDF with h5netcdf. That write still warns once each for the four
scaled integer arrays (no `_FillValue` to keep for NaN): they have no
spare value, as 65535 and 255 are real shares.

`TestXarrayReadsAStore` (skipped unless `ZARR_PYTHON` names a Python with
xarray) writes a small made world and runs `testdata/read_xarray.py`; it
fails when the strata mask is removed. `TestEveryKoppenTypeHasACode` and
`TestAStoreHasCoordinatesAndConsolidatedMetadata` are Go-only.

**What it costs.** The globe store is 83 arrays, 176 files and 41.4 MiB
(was 67, 143 and 41.2 MiB), written in 1.26 s against 0.76 s, one run
with other sessions on the machine: a reading, not a baseline.

**Existing stores.** The layout change breaks readers of stores written
before it, not the stores: Go and zarr-python still open them. What
changed under a reader: `legend` is gone for `flag_values`/`flag_meanings`
(and meanings use underscores), Köppen codes are renumbered, `terms` is
a string and `wrap` a number, `about` is `long_name` on arrays, units are
spelt differently, and `strata/rock`/`formed` past a pile are 255, not 0.
Old stores have no coordinates; rewrite them to get them.

---

## 2026-09-16 - Phase 3, step 1: the history on a grid of its own

**What this is.** The first step of the history grid (phase 3 of
[scaling-plan.md](scaling-plan.md)), on `claude/history-grid` from main at
3973917. No world moves: `TERRA_DIGEST=check` passes, the budget and pinned
pass counts hold, the goroutine-independence test and the short tier pass.

**What changed.** `history` ends where the history's own work ends - the
floor's depth and the rock's rise read off the crust, the plates kept, the
rock settled, the heights softened - and returns a `deepStage` (the ocean
crust, the floor depths and shares, the uplift). The rest of what it did,
which is the map's and not the planet's, is `settleHistory` on the map: the
rescaling by rank (`basins`, `normalise`, `restrata`), the slides, the deep
floor, `expose` and the drain. Between the two, `historyGround` gives the
grid the history runs on and `handDown` (`historygrid.go`) lays a finished
history onto the map when that grid is not the map: heights, uplift, floor
depth and share read between the history's tiles (bilinear, wrapping east
to west on a globe); the tile, its soil, its line of the book and its pile
of beds from the nearest history tile, the beds moved with the ground; the
ocean crust by nearest; the water, weather and lakes read afresh on the map.
The history grid is the map (`historyShrink` is 1) everywhere but in tests.

Found on the way, which step 2 is: `deepSpan`, the kilometres a history
tile is, is not a fact about the planet. It follows from the plate count and
spacing, which follow from the grid's width, and about fifteen constants
of the history are counted in tiles (the plate count, the molten era's
cells, the floods' grain, the bow and grain of the ranges, `seamLeast`,
`marginRamp`, the fold's wave). A history on a coarser grid is so far a
history of another planet, laid onto this map.

**Tests.** `TestAHistoryHandedDownOntoItsOwnSizeIsItself`: handed down onto
a map of its own size by the reading between tiles, a history is itself to
the bit (ancient, globe128). `TestAHistoryHandedDownFromACoarserGridIsTheHistoryReadFiner`:
from half the size, no height outside the history's range, every tile the
nearest history tile's, beds where they stood against the ground, every
plate on the map. `TestAWorldOnACoarserHistoryIsAWorld`: ancient on a
half-size history is independent of the goroutines and resumes from its
history file to the bit.

**What it measured**, for scale only, since these worlds are not today's
(the full globe, one run each, quiet machine):

| history grid | globe wall | land share | plates | forest tiles |
|---|---:|---:|---:|---:|
| 1024x512 (the map) | 49.7 s | 0.394 | 48 | 45 797 |
| 512x256 | 26.7 s | 0.329 | 46 | 41 797 |
| 256x128 | 15.7 s | 0.375 | 27 | 41 792 |

The quarter-size history has 27 plates to the map's 48 because the plate
count is read off the grid's width: the size-dependence step 2 takes out
before any of these is judged by the yardsticks.

---

## 2026-09-16 - The suite's histories kept between runs

**What this is.** The yardsticks' share of the history file, on
`claude/app-performance-structural-956966` after the stages merged to main
(b4a0d3a). Test code only: no non-test file of the package changed, so the
digest, the budget and every world are main's.

**What changed.** `madeLand(seed, terms)` in `histories_test.go` is NewLand
for a test that needs a world and not the making of one: it reads the
history kept in `.cache/histories/<code>/` and runs the stages after it,
or makes the world and keeps its history there. `<code>` is a hash of every
non-test Go file of the module outside `cmd`, go.mod, the Go version and
the build settings, so a history is never read by code that could have
made a different one, and directories for any other code are removed on
the first use of a run. `TERRA_HISTORIES=trust` keys by seed and terms
alone, for work on the stages after the history; `TERRA_HISTORIES=off`
keeps nothing. `yardLand` (every shared world), the tidal coast, the
slides on a small globe, the day-to-day weather, the rock and strata tests
on made valleys, and the calibration table's untimed readings go through
it. The tests of the making itself keep NewLand: the goroutine
independence, the digest, the budget, the timed globe, the why chains over
goroutines, the same seed running the same history.
`TestAKeptHistoryIsTheWorld` holds a made, a read and a remade-over-a-spoilt-file
world to NewLand's digest.

**What it measured.** The whole suite (`go test -json -timeout 60m .`),
one test binary per side, quiet machine, 24 threads:

| run | wall | failures |
|---|---:|---|
| main before (b4a0d3a's tests) | 386 s | none |
| branch, nothing kept | 375 s | none |
| branch, histories kept | **187 s** | none |

The slowest tests, seconds, in the same three runs:

| test | before | nothing kept | kept |
|---|---:|---:|---:|
| `TestTheIceEdgeIsNotALineOfLatitude` (three globes) | 96.9 | 97.8 | 20.6 |
| `TestTheRealWorld` | 69.0 | 69.9 | 15.9 |
| `TestAGlobeHasASeaItsRiversReach` (times the making) | 48.5 | 47.3 | 46.9 |
| `TestRealNumbers` | 32.8 | 33.0 | 8.4 |
| `TestSaltLakesStandInDryCountry` | 32.3 | 28.4 | 6.3 |
| `TestMakingAWorldDoesNotDependOnTheGoroutines` | 28.6 | 29.8 | 28.2 |
| `TestTheUplandMaskIsFinerThanTheMap` | 12.2 | 12.2 | 12.1 |

The kept histories are 1.1 GB on disk, in the worktree. What is left of
the kept run is the making the suite has to do: the timed globe, the
goroutine-independence test's five small globes made four ways, and the
upland mask's drawn globe. A run after any change to a non-test file makes
everything again, so the kept run is the suite's time while a test is
being tuned, or with `trust` while a later stage is being worked on.

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
