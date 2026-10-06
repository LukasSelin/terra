# The world as one system: atmosphere, rock, land and life, and what joins them

Written 2026-10-06 against main at `e833601`. A companion to
`docs/ocean-currents-plan.md` and `docs/ocean-model-plan.md`, from three
surveys of the code: the air, the rock, and the land and its life. It names
where each is simplified, the research that would fill the gaps, and the
order to do it in. The couplings between systems come last, because they
are what makes a world rather than four maps on top of each other.

## What the surveys found

Each system is well sourced piece by piece, but most pieces are readings of
the present climate, and almost nothing feeds back.

- **The air** (`internal/atmos`)
  - The energy balance is zonal: 90 bands, each 30% land whatever the map
    is (`ebmLand`), and solved once. The 2D climate is that profile plus
    offsets.
  - The pressure belts are drawn at fixed latitudes (`beltPressure`), so the
    Hadley cell, the ITCZ and the jets are written in, not worked out.
  - There are no stationary waves.
  - Radiation is Budyko's line, with no CO₂, vapour or clouds.
  - Albedo is a function of latitude, blind to snow, forests and the map's
    ice.
  - Interannual variability is one global noise.
  - The calendar is 360 days, the energy balance 365.25, and PET and the
    wind carry their own fixed swing.
- **The rock** (`history.go` and around it)
  - Crust is one bit, ocean or continent.
  - Isostasy is two fixed freeboards and a 25 Myr relaxation (`settleTime`,
    "a stand-in for the isostasy this model does not have").
  - Plate speeds are drawn and decay linearly.
  - Hotspots are random.
  - Sea level is a 35% quantile in the history and a prescribed sawtooth
    after it.
  - The history's real heights are rank-mapped onto the drawn map's spread
    and thrown away.
  - Erosion through 64 Myr feels the moving mountains, but always under the
    present day's climate: no epoch's own warmth, CO₂ or ice. There are no
    ice sheets, no glacial erosion and no carbon cycle.
- **The land and its life** (`woods.go`, `soil.go`, `pedogenesis.go`,
  `grow.go`)
  - Woods are a Forest/Grass label read off dryness times wetness, placed
    once, and never dying back.
  - A biome is a name for a Köppen code.
  - Water is Budyko-Fu with one ω for the whole planet: no soil moisture,
    no seasons, no snowpack.
  - Permafrost, glaciers and peat are labels.
  - NPP is a growth multiplier, not a stock.
  - Vegetation reaches erosion through two constants per terrain, and
    reaches the climate not at all.
  - The sea's life is fish drawn uniformly.
  - Soil is the richest part (Heimsath, West, Jenny, a carbon relaxation),
    and it reads a climate that never moves.

None of the GitHub issues so far covers any of this; #16–#31 are the ocean.

## Tracks

Four tracks, each in its own files so they can run side by side, and a fifth
of couplings that joins them. Issue numbers are in the epics.

### A. The air

| | What | Reference | Moves the world |
|---|---|---|---|
| A0 | **Forcings as variables**: CO₂ (OLR − 5.35 ln C/C₀, Myhre et al. 1998), the orbit (eccentricity, obliquity, precession, Berger 1978), the solar constant, kept on the planet's terms, defaulting to today's so nothing moves | Myhre 1998; Berger 1978; Berger & Loutre 1991 | no |
| A1 | **One calendar**: the energy balance's year, the clock's, and the swing PET and the wind read, made one; the fixed 12 °C swing gone | — | yes |
| A2 | **The circulation worked out**: the Hadley cell's edge from Held & Hou (1980), the ITCZ on the energy-flux equator and following the season, the subtropical highs and the storm track placed from them, replacing `beltPressure`'s fixed latitudes | Held & Hou 1980; Lindzen & Nigam 1987; Bischoff & Schneider 2014 | yes |
| A3 | **Stationary waves and monsoons**: the linear response to orography and to heating, Gill (1980) in the tropics and Hoskins & Karoly (1981) beyond, so a plateau bends the jet and a continent's summer draws a monsoon | Gill 1980; Hoskins & Karoly 1981; Held, Ting & Wang 2002 | yes |
| A4 | **Two-dimensional energy balance** on the air cells, with the map's land, ice and albedo, after M3 hands it the ocean's heat; the zonal model stays as the check | North, Short & Mengel 1983 | yes |
| A5 | **Radiation with water vapour and clouds**: a grey two-band column per cell, low cloud over cold water and in the subtropical highs, high cloud in the ITCZ | Manabe & Wetherald 1967; Slingo 1987 | yes |
| A6 | **Interannual variability with a shape**: a recharge-oscillator ENSO on M2's thermocline, with teleconnections on the rain and the warmth, in place of one global noise | Jin 1997; Zebiak & Cane 1987 | yes |
| A7 | **Rain from the weather**: the synoptic systems carry fronts and rain, so a day's rain falls where the lows go | Richardson 1981 (WGEN), driven by synoptic.go | no (weather only) |

### G. The rock

| | What | Reference | Moves the world |
|---|---|---|---|
| G1 | **Real heights kept**: the history's hypsometry carried onto the map instead of rank-mapped onto the drawn 60/260 m spread, checked against Earth's (Cogley 1984); `shape.go` keeps the detail below the history's grid | Cogley 1984; Willett 1999 | yes |
| G2 | **Crust thickness and isostasy**: a thickness field thickened by collision and thinned by rifting and erosion; Airy compensation with elastic flexure; rebound from erosion and from ice and sediment loads. Replaces the fixed freeboards and `settleTime` | Turcotte & Schubert; Watts 2001; Molnar & England 1990 | yes |
| G3 | **Sea level from the ocean basins and the ice**: the ocean's volume against the ridges' (young floor stands high) and the ice sheets' water, replacing the 35% quantile and the drawn sawtooth | Pitman 1978; Müller et al. 2008 | yes |
| G4 | **Plates driven by forces**: slab pull, ridge push and drag set the speeds (Forsyth & Uyeda 1975), and a supercontinent's breakup and assembly follow (the Wilson cycle). The cylinder stays (owner decision 2026-09-15) | Forsyth & Uyeda 1975; Nance & Murphy | yes |
| G5 | **Plumes, flood basalts and outgassing**: hotspots from plumes that track and wane, large igneous provinces, and the CO₂ each puts out (to X2) | Morgan 1971; Coffin & Eldholm 1994; Marty & Tolstikhin 1998 | yes |
| G6 | **Basins that subside**: rift stretching (McKenzie 1978) and foreland flexure (Beaumont 1981, on G2), filled at the rate sediment arrives rather than 0.1 mm/yr everywhere | McKenzie 1978; Beaumont 1981 | yes |
| G7 | **Rocks of an epoch's climate**: evaporites, coal and carbonate belts from that epoch's own sea and air (needs X1); joins #30 | Ziegler et al. 2003; Kleypas 1999; Warren 2010 | yes |
| G8 | **Ice sheets and glaciers through the history**: shallow-ice flow, mass balance from the epoch's climate, glacial erosion, and their load on G2 and their water on G3 | Hutter 1983; Pollard & DeConto 2009; Harbor 1992; Herman et al. 2015 | yes |
| G9 | **Cold and wind on the ground**: frost cracking, loess blown off outwash and deserts | Anderson 2002; Pye 1995 | yes |

### L. The land and its life

| | What | Reference | Moves the world |
|---|---|---|---|
| L1 | **Soil moisture and the seasons of water**: a bucket per tile run through the four phases (Manabe 1969), ω from the vegetation and the rooting depth (Zhang et al. 2001), replacing the one planet-wide ω | Manabe 1969; Zhang et al. 2001; Donohue et al. 2012 | yes |
| L2 | **Snowpack and glaciers on today's map**: snow water by degree-days, melt timing into the rivers, snow cover for the albedo (to X3), glacier mass balance in place of the `Barren` label | Hock 2003; Ohmura 1992; Oerlemans | yes |
| L3 | **Vegetation as a state**: plant functional types with biomass and leaf area, established, growing and dying by climate, water and competition. BIOME4's equilibrium first, then LPJ-style dynamics; woods that die back where the ground stops suiting them | Kaplan et al. 2003 (BIOME4); Sitch et al. 2003 (LPJ) | yes |
| L4 | **Fire and disturbance**: fire from fuel, dryness and season, windthrow and drought dieback; the savanna-forest balance as two stable states | Thonicke et al. 2010 (SPITFIRE); Staver et al. 2011; Bond 2005 | yes |
| L5 | **Vegetation holds the ground**: critical shear and creep scaled by cover and leaf area instead of two constants a terrain, root cohesion in slope stability | Istanbulluoglu & Bras 2005; Collins et al. 2004; Schmidt et al. 2001 | yes |
| L6 | **Permafrost, wetlands and peat**: an active layer that thaws (Stefan), wetland extent from TOPMODEL, peat that accumulates and rises | Clymo 1984; Frolking et al. 2010; Stocker et al. 2014 | yes |
| L7 | **Carbon as stocks**: vegetation, litter, soil (fast and slow), peat and permafrost pools in place of NPP as a multiplier | Parton et al. 1987 (CENTURY); Coleman & Jenkinson (RothC) | yes |
| L8 | **Life in the sea**: productivity from upwelling, mixing and light (on M2's thermocline), fish stocks that follow it, and the ooze it lays, calcareous or siliceous, in place of temperature-only limestone. This replaces the currents plan's "not in this plan" | Eppley 1972; Behrenfeld & Falkowski 1997 | yes |

### X. The couplings: the big factors on one another

These are why the world is one world. Each joins tracks and declares its
loop in #27's coupling graph.

| | Loop | Needs |
|---|---|---|
| X1 | **The climate of each epoch.** Run the air (and M1's flow, coarsely) on each epoch's own geography, with A0's orbit and CO₂, so the history erodes, weathers and lays rock under its own climate, not today's. Includes the cost study: the weather is about a second on a globe; 16 epochs on the history's coarse grid must be measured before committing | A0, M1; cost figure first |
| X2 | **The carbon thermostat.** Outgassing from ridges, arcs and plumes (G5) against silicate weathering (soil.go's Arrhenius × runoff, boosted by plants, L3), plus organic burial (L7, coal from G7), sets the CO₂ of each epoch, which sets its warmth through A0. The long-term thermostat that keeps a planet habitable | Walker, Hays & Kasting 1981; Berner 2006 (GEOCARB); Raymo & Ruddiman 1992; Maher & Chamberlain 2014 |
| X3 | **Albedo from the surface.** Snow (L2), sea ice (M7), ice sheets (G8), deserts and forests (L3) set the albedo the energy balance reads, cell by cell (A4). Ice-albedo, snow-albedo and Charney's desert feedback (1975) | A4 |
| X4 | **Land and air trade water.** Soil moisture (L1) and transpiration (L3) set the evaporation the vapour budget recycles, and canopy sets the roughness the wind reads. A wet spring makes a wet summer | Koster et al. 2004; Eltahir & Bras 1996; Bonan 2008 |
| X5 | **Ice ages.** The orbit (A0) sets the summer sun at 65°N; the ice sheets grow and shrink (G8), take the sea down and up (G3), load and unload the crust (G2), and move the coasts and the rivers' base level. Replaces the drawn sawtooth with the cause of it | Milankovitch; Imbrie & Imbrie 1980; Lisiecki & Raymo 2005 |
| X6 | **Mountains make climate, climate makes mountains.** Uplift (G2) wrings out rain (orographic.go), the rain erodes (fluvial.go), the erosion unloads the crust and it rises (G2), and the weathering of fresh rock draws down CO₂ (X2). Raymo's uplift-weathering hypothesis, made a loop rather than a story | Raymo & Ruddiman 1992; Willett 1999; Whipple 2009 |

## How it is run

Files decide what can run together; a branch that touches another's files
waits. Lanes:

- **Ocean lane** (`internal/atmos/ocean.go`): M1 → M2 → M3 → M4 → M5; M6;
  M7. Running.
- **Air lane** (`ebm.go`, `wind.go`, `year.go`): A0 → A1 → A2 → A3; A4
  after M3; A5; A6 after M2 and R2.
- **Rock lane** (`history.go`, `strata.go`, `sealevel.go`): G1 → G2 → G3 →
  G6 → G5 → G4; G7, G8 and G9 after X1.
- **Land lane** (`woods.go`, `soil.go`, `pedogenesis.go`, `terrain.go`,
  `weather.go`): L1 → L2 → L3 → L5 → L4 → L6 → L7; L8 after M2.
- **Couplings**: X1 after A0 and M1; X3 after A4; X4 after L1 and L3; X2
  after X1, G5 and L7; X5 after X1, G8 and G3; X6 after G2 and X2.

At most three heavy branches at once on this machine. Every world-moving
step runs the yardsticks against main's failure list and rewrites the
digest; every step keeps the ocean's, the air's and the history's phase
times in the work log.

## Owner decisions

1. **The deep-time climate's cost (X1).** A coarse weather per epoch
   multiplies the history's weather work by up to sixteen. Default: measure
   first, and accept up to doubling the history's time.
2. **Real heights (G1)** were provisionally globe-only (2026-09-15). Default:
   globe-only still; valleys keep their drawn spread.
3. **L3's vegetation in deep time.** Default: present-day biomes only at
   first; plants in the epochs come with X2, where they matter for
   weathering.
4. **L8 and the game's fish.** Default: productivity sets the wild stock's
   ceiling per water tile; the game's harvest rules are unchanged.
