# The coupling graph

Generated from `couplings.go` by `TERRA_COUPLINGS=write go test -run TestCouplingsDoc -timeout 60m .`; do not edit it by hand.

Which pass of world creation reads which of the world's fields and writes which, and the feedback loops that makes. The declaration is held to the code by `couplings_test.go`: each pass's source is read for the fields it touches, and worlds are made stage by stage and every field a stage changes has to be one of that stage's passes' writes. A loop a later change adds - the climate of each epoch (#57), the carbon thermostat (#58), albedo from the surface (#59), land and air trading water (#60), the ice ages (#61), mountains and climate (#62), fog over cold water (#29) - is one line in `couplings`, and a field it brings one line in `worldFields`; a loop one pass closes within one field, as the weather closes the sea and the air's (#28), is one entry in `innerLoops`.

## The fields

| System | Field | What it is |
|---|---|---|
| air | energy | the energy balance row by row: the year's mean warmth, what the air can take up, how wet it is |
| air | wind | the wind, the pressure, the air's budget of water and the sea's currents, as package atmos works them out |
| air | rain | each tile's year of rain, what runs off, its summer's share and its day's range |
| air | year | each tile's year at sea level: its mean and its swing |
| sea | sea | the level of the sea and of the water the air takes its fill from |
| sea | tide | the tide's reach and the flats it lays bare |
| rock | height | the ground's height |
| rock | floor | the deep sea floor: where it stood before its age laid it down, how fast the rock rose, how old the crust is |
| rock | rock | the rock: each tile's bed, the epoch it was laid in, and the beds under it |
| rock | plates | which plate each tile rides, which plate each has been welded into, and the hotspots |
| rock | book | the book the history kept of what it did to each tile, and how many epochs it ran |
| water | drainage | where each tile's water goes and how much runs through it, and how far it stands over it |
| water | moisture | the soil's water through the year: each phase's rain, what the soil holds and sheds of it, and how much it can hold |
| water | load | the ground the water carries: off the land, and off the banks of its bends |
| water | lakes | the standing water: the lakes, their level, and the salt pans |
| land and life | soil | the soil: how deep, what it is made of, and what time has made of it |
| land and life | cover | what each tile is: open ground, forest, water, ice, outcrop, tidal flat, salt |
| land and life | woods | where trees will take, and the map's measure of its ground they are read against |
| land and life | fertility | what the soil will grow |
| land and life | stocks | what stands to be taken: the fish, the grass, and how long what stands has grown |
| registry | features | the registry of the things the tiles make up, and their relations |

## The systems

System to system: an arrow is a pass that reads a field of the one and writes a field of the other, numbered with how many such pairs of fields there are.

```mermaid
flowchart LR
  air[air]
  sea[sea]
  rock[rock]
  water[water]
  land_and_life[land and life]
  registry[registry]
  air -- 7 --> sea
  air -- 13 --> rock
  air -- 13 --> water
  air -- 20 --> land_and_life
  air -- 4 --> registry
  sea -- 3 --> air
  sea -- 6 --> rock
  sea -- 5 --> water
  sea -- 10 --> land_and_life
  sea -- 1 --> registry
  rock -- 7 --> air
  rock -- 7 --> sea
  rock -- 11 --> water
  rock -- 19 --> land_and_life
  rock -- 3 --> registry
  water -- 2 --> air
  water -- 3 --> sea
  water -- 8 --> rock
  water -- 11 --> land_and_life
  water -- 2 --> registry
  land_and_life -- 2 --> air
  land_and_life -- 4 --> sea
  land_and_life -- 8 --> rock
  land_and_life -- 4 --> water
  land_and_life -- 1 --> registry
```

## The graph

Field to field: each field, and the fields the passes that read it write.

| Field | Drives |
|---|---|
| energy | wind (weather); rain (weather); sea (history, cutThroughCycle); tide (tides); height (history, settleHistory, shape, landslide, cutThroughCycle, wear); floor (settleHistory); rock (history, keepBook, settleHistory, shape); plates (history); book (history, keepBook, wear); drainage (flow); moisture (weather); load (wear, waterStep); lakes (pool, flow); soil (history, landslide, cutThroughCycle, wear, soilTexture, laySoil); cover (tides, cover); woods (tides, cover, readWoods); fertility (tides, cover); stocks (tides, cover); features (readFeatures) |
| wind | rain (weather); year (year); sea (cutThroughCycle); tide (tides); height (shape, landslide, cutThroughCycle, wear); rock (shape); book (wear); drainage (flow); moisture (weather); load (wear, waterStep); lakes (pool, flow); soil (landslide, cutThroughCycle, wear, soilTexture, laySoil); cover (tides, cover); woods (tides, cover, readWoods); fertility (tides, cover); stocks (tides, cover); features (readFeatures) |
| rain | wind (weather); sea (cutThroughCycle); tide (tides); height (shape, landslide, cutThroughCycle, wear); rock (shape); book (wear); drainage (flow); moisture (weather); load (wear, waterStep); lakes (pool, flow); soil (landslide, cutThroughCycle, wear, soilTexture, laySoil); cover (tides, cover); woods (tides, cover, readWoods); fertility (tides, cover); stocks (tides, cover); features (readFeatures) |
| year | tide (tides); height (wear); book (wear); load (wear, waterStep); soil (wear, laySoil); cover (freeze, tides, cover); woods (tides, cover, readWoods); fertility (tides, cover); stocks (freeze, tides, cover); features (readFeatures) |
| sea | wind (weather); rain (weather); year (year); tide (tides); height (history, tectonics, shape, texture, denude, cutThroughCycle, wear, silt); rock (history, tectonics, keepBook, shape); plates (history, tectonics); book (history, tectonics, keepBook, wear); drainage (flow); moisture (weather); load (wear, waterStep); lakes (pool, flow); soil (history, cutThroughCycle, wear, silt); cover (pour, level, carve, tides); woods (pour, level, carve, tides); fertility (tides); stocks (pour, level, carve, tides); features (readFeatures) |
| tide | height (wear, silt); book (wear); load (wear, waterStep); soil (wear, silt); cover (tides); woods (tides); fertility (tides); stocks (tides) |
| height | wind (weather); rain (weather); year (year); sea (history, pour, level, cutThroughCycle); tide (tides); floor (settleHistory); rock (history, move, tectonics, keepBook, settleRock, handDown, settleHistory, layBedrock, expose, shape); plates (history, move, tectonics, settleRock, handDown); book (history, tectonics, keepBook, handDown, wear); drainage (flow, height); moisture (weather); load (wear, waterStep); lakes (pool, flow); soil (history, move, handDown, landslide, cutThroughCycle, wear, silt, soilTexture, laySoil); cover (move, handDown, pour, level, carve, freeze, tides, cover); woods (pour, level, carve, tides, cover, readWoods); fertility (tides, cover); stocks (pour, level, carve, freeze, tides, cover); features (readFeatures) |
| floor | wind (weather); rain (weather); sea (pour, cutThroughCycle); tide (tides); height (settleHistory, shape, texture, landslide, cutThroughCycle, wear); rock (settleHistory, shape); book (wear); drainage (flow, height); moisture (weather); load (wear); lakes (pool, flow); soil (landslide, cutThroughCycle, wear, laySoil); cover (pour, freeze, tides, cover); woods (pour, tides, cover, readWoods); fertility (tides, cover); stocks (pour, freeze, tides, cover) |
| rock | wind (weather); rain (weather); sea (history, cutThroughCycle); tide (tides); height (history, move, tectonics, handDown, settleHistory, shape, denude, landslide, cutThroughCycle, wear); floor (settleHistory); plates (history, move, tectonics, settleRock, handDown); book (history, tectonics, keepBook, handDown, wear); moisture (weather); load (wear, waterStep); soil (history, move, handDown, landslide, cutThroughCycle, wear, soilTexture, laySoil); cover (move, handDown, tides); woods (tides); fertility (tides); stocks (tides) |
| plates | sea (history); height (history, move, tectonics, handDown); rock (history, move, tectonics, settleRock, handDown); book (history, tectonics, handDown); soil (history, move, handDown); cover (move, handDown); features (readFeatures) |
| book | height (tectonics, handDown, settleHistory, wear); floor (settleHistory); rock (tectonics, keepBook, handDown, settleHistory); plates (tectonics, handDown); load (wear); soil (handDown, wear); cover (handDown); features (readFeatures) |
| drainage | sea (history); tide (tides); height (history, wear); rock (history, keepBook); plates (history); book (history, keepBook, wear); load (wear, waterStep); lakes (flow); soil (history, wear, laySoil); cover (carve, tides, cover); woods (carve, tides, cover, readWoods); fertility (tides, cover); stocks (carve, tides, cover); features (readFeatures) |
| moisture | wind (weather); rain (weather) |
| load | height (wear); book (wear); soil (wear) |
| lakes | tide (tides); height (denude, wear); book (wear); drainage (flow, height); load (wear, waterStep); soil (wear); cover (carve, freeze, tides); woods (carve, tides); fertility (tides); stocks (carve, freeze, tides); features (readFeatures) |
| soil | wind (weather); rain (weather); sea (history, cutThroughCycle); tide (tides); height (history, move, handDown, landslide, cutThroughCycle, wear, silt); rock (history, move, keepBook, handDown); plates (history, move, handDown); book (history, keepBook, handDown, wear); moisture (weather); load (wear, waterStep); cover (move, handDown, tides, cover); woods (tides, cover); fertility (tides, cover); stocks (tides, cover) |
| cover | sea (history, pour, level, cutThroughCycle); tide (tides); height (history, shape, denude, landslide, cutThroughCycle, wear); rock (history, keepBook, shape); plates (history); book (history, keepBook, wear); drainage (height); load (wear, waterStep); soil (history, landslide, cutThroughCycle, wear, laySoil); woods (pour, level, carve, tides, cover, readWoods); fertility (tides, cover); stocks (pour, level, carve, freeze, tides, cover); features (readFeatures) |
| woods | cover (cover); fertility (cover); stocks (cover) |
| fertility | cover (cover); woods (cover); stocks (cover) |
| stocks |  |
| features |  |

## The passes

| Pass | Stages | Reads | Writes |
|---|---|---|---|
| newGround | ground |  | energy |
| historyGround | ground |  | energy |
| history | ground | energy, sea, height, rock, plates, drainage, soil, cover | sea, height, rock, plates, book, soil |
| flood | ground |  |  |
| move | ground | height, rock, plates, soil | height, rock, plates, soil, cover |
| joinUp | ground | plates | plates |
| tectonics | ground | sea, height, rock, plates, book | height, rock, plates, book |
| reshape | ground | plates | plates |
| keepBook | ground | energy, sea, height, rock, book, drainage, soil, cover | rock, book |
| settleRock | ground | height, plates, rock | plates, rock |
| handDown | ground | height, rock, plates, book, soil | height, rock, plates, book, soil, cover |
| settleHistory | ground | energy, height, floor, rock, book | height, floor, rock |
| basins | ground | height | height |
| raise | ground |  | height |
| layBedrock | ground | height, rock | rock |
| expose | ground, shape, cut, coast | height, rock | rock |
| pour | sea, cut, coast | sea, height, floor, cover | sea, cover, woods, stocks |
| level | sea, cut, coast | sea, height, cover | sea, cover, woods, stocks |
| shape | shape | energy, wind, rain, sea, height, floor, rock, cover | height, rock |
| texture | shape | floor, height, sea | height |
| denude | shape | cover, height, lakes, rock, sea | height |
| landslide | ground, shape, cut | cover, energy, floor, height, rain, rock, soil, wind | height, soil |
| drain | ground, shape, cut, coast | floor, height, rain, sea, wind |  |
| weather | ground, shape, cut, coast | energy, wind, rain, sea, height, floor, rock, moisture, soil | wind, rain, moisture |
| defaultAir | ground, shape, cut, coast | energy | energy |
| pool | ground, shape, cut, coast | energy, wind, rain, sea, height, floor, lakes | lakes |
| flow | ground, shape, cut, coast | energy, wind, rain, sea, height, floor, drainage, lakes | drainage, lakes |
| cutValleys | cut |  |  |
| cutThroughCycle | cut | cover, energy, floor, height, rain, rock, sea, soil, wind | height, sea, soil |
| carve | cut | sea, height, drainage, lakes, cover | cover, woods, stocks |
| height | cut, coast | cover, drainage, floor, height, lakes | drainage |
| wear | ground, cut | energy, wind, rain, year, sea, tide, height, floor, rock, book, drainage, load, lakes, soil, cover | height, book, load, soil |
| creep | ground, cut | cover, drainage, floor, height, soil |  |
| waterStep | ground, cut, coast | energy, wind, rain, year, sea, tide, height, rock, drainage, load, lakes, soil, cover | load |
| year | coast | height, sea, wind | year |
| freeze | coast | cover, floor, height, lakes, year | cover, stocks |
| tides | coast | energy, wind, rain, year, sea, tide, height, floor, rock, drainage, lakes, soil, cover | tide, cover, woods, fertility, stocks |
| silt | coast | sea, tide, height, soil | height, soil |
| cover | cover | energy, wind, rain, year, height, floor, drainage, soil, cover, woods, fertility | cover, woods, fertility, stocks |
| readWoods | cover | energy, wind, rain, year, height, floor, drainage, cover, woods | woods |
| soilTexture | ground, cover | energy, height, rain, rock, wind | soil |
| laySoil | cover | cover, drainage, energy, floor, height, rain, rock, soil, wind, year | soil |
| recount | cover | height, floor |  |
| readFeatures | cover | energy, wind, rain, year, sea, height, plates, book, drainage, lakes, cover | features |
| readRelations | cover | energy, wind, rain, sea, height, plates, lakes |  |

## The loops

Fields that drive one another round some loop, each set by Tarjan's strongly connected components:

- wind, rain, year, sea, tide, height, floor, rock, plates, book, drainage, moisture, load, lakes, soil, cover, woods, fertility

Every loop of up to 3 fields, found by walking the graph: each field drives the next and the last the first, through the passes named, with the systems it goes through. A loop one pass makes alone, reading and writing its own fields, is not counted. 300 loops: 43 of two fields and 257 of three.

- wind -(cutThroughCycle)-> sea -(weather)-> wind [air, sea]
- wind -(shape, landslide, cutThroughCycle, wear)-> height -(weather)-> wind [air, rock]
- wind -(shape)-> rock -(weather)-> wind [air, rock]
- wind -(landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(weather)-> wind [air, land and life]
- rain -(cutThroughCycle)-> sea -(weather)-> rain [air, sea]
- rain -(shape, landslide, cutThroughCycle, wear)-> height -(weather)-> rain [air, rock]
- rain -(shape)-> rock -(weather)-> rain [air, rock]
- rain -(landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(weather)-> rain [air, land and life]
- year -(wear)-> height -(year)-> year [air, rock]
- sea -(history, tectonics, shape, texture, denude, cutThroughCycle, wear, silt)-> height -(history, pour, level, cutThroughCycle)-> sea [sea, rock]
- sea -(history, tectonics, keepBook, shape)-> rock -(history, cutThroughCycle)-> sea [sea, rock]
- sea -(history, tectonics)-> plates -(history)-> sea [sea, rock]
- sea -(flow)-> drainage -(history)-> sea [sea, water]
- sea -(history, cutThroughCycle, wear, silt)-> soil -(history, cutThroughCycle)-> sea [sea, land and life]
- sea -(pour, level, carve, tides)-> cover -(history, pour, level, cutThroughCycle)-> sea [sea, land and life]
- tide -(wear, silt)-> height -(tides)-> tide [sea, rock]
- tide -(wear, silt)-> soil -(tides)-> tide [sea, land and life]
- height -(settleHistory)-> floor -(settleHistory, shape, texture, landslide, cutThroughCycle, wear)-> height [rock]
- height -(history, move, tectonics, keepBook, settleRock, handDown, settleHistory, layBedrock, expose, shape)-> rock -(history, move, tectonics, handDown, settleHistory, shape, denude, landslide, cutThroughCycle, wear)-> height [rock]
- height -(history, move, tectonics, settleRock, handDown)-> plates -(history, move, tectonics, handDown)-> height [rock]
- height -(history, tectonics, keepBook, handDown, wear)-> book -(tectonics, handDown, settleHistory, wear)-> height [rock]
- height -(flow, height)-> drainage -(history, wear)-> height [rock, water]
- height -(wear, waterStep)-> load -(wear)-> height [rock, water]
- height -(pool, flow)-> lakes -(denude, wear)-> height [rock, water]
- height -(history, move, handDown, landslide, cutThroughCycle, wear, silt, soilTexture, laySoil)-> soil -(history, move, handDown, landslide, cutThroughCycle, wear, silt)-> height [rock, land and life]
- height -(move, handDown, pour, level, carve, freeze, tides, cover)-> cover -(history, shape, denude, landslide, cutThroughCycle, wear)-> height [rock, land and life]
- floor -(settleHistory, shape)-> rock -(settleHistory)-> floor [rock]
- floor -(wear)-> book -(settleHistory)-> floor [rock]
- rock -(history, move, tectonics, settleRock, handDown)-> plates -(history, move, tectonics, settleRock, handDown)-> rock [rock]
- rock -(history, tectonics, keepBook, handDown, wear)-> book -(tectonics, keepBook, handDown, settleHistory)-> rock [rock]
- rock -(history, move, handDown, landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(history, move, keepBook, handDown)-> rock [rock, land and life]
- rock -(move, handDown, tides)-> cover -(history, keepBook, shape)-> rock [rock, land and life]
- plates -(history, tectonics, handDown)-> book -(tectonics, handDown)-> plates [rock]
- plates -(history, move, handDown)-> soil -(history, move, handDown)-> plates [rock, land and life]
- plates -(move, handDown)-> cover -(history)-> plates [rock, land and life]
- book -(handDown, wear)-> soil -(history, keepBook, handDown, wear)-> book [rock, land and life]
- book -(handDown)-> cover -(history, keepBook, wear)-> book [rock, land and life]
- drainage -(flow)-> lakes -(flow, height)-> drainage [water]
- drainage -(carve, tides, cover)-> cover -(height)-> drainage [water, land and life]
- load -(wear)-> soil -(wear, waterStep)-> load [water, land and life]
- soil -(move, handDown, tides, cover)-> cover -(history, landslide, cutThroughCycle, wear, laySoil)-> soil [land and life]
- cover -(pour, level, carve, tides, cover, readWoods)-> woods -(cover)-> cover [land and life]
- cover -(tides, cover)-> fertility -(cover)-> cover [land and life]

<details><summary>The 257 loops of three fields</summary>

- wind -(weather)-> rain -(cutThroughCycle)-> sea -(weather)-> wind [air, sea]
- wind -(weather)-> rain -(shape, landslide, cutThroughCycle, wear)-> height -(weather)-> wind [air, rock]
- wind -(weather)-> rain -(shape)-> rock -(weather)-> wind [air, rock]
- wind -(weather)-> rain -(landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(weather)-> wind [air, land and life]
- wind -(year)-> year -(wear)-> height -(weather)-> wind [air, rock]
- wind -(year)-> year -(wear, laySoil)-> soil -(weather)-> wind [air, land and life]
- wind -(cutThroughCycle)-> sea -(weather)-> rain -(weather)-> wind [air, sea]
- wind -(cutThroughCycle)-> sea -(history, tectonics, shape, texture, denude, cutThroughCycle, wear, silt)-> height -(weather)-> wind [air, sea, rock]
- wind -(cutThroughCycle)-> sea -(history, tectonics, keepBook, shape)-> rock -(weather)-> wind [air, sea, rock]
- wind -(cutThroughCycle)-> sea -(weather)-> moisture -(weather)-> wind [air, sea, water]
- wind -(cutThroughCycle)-> sea -(history, cutThroughCycle, wear, silt)-> soil -(weather)-> wind [air, sea, land and life]
- wind -(tides)-> tide -(wear, silt)-> height -(weather)-> wind [air, sea, rock]
- wind -(tides)-> tide -(wear, silt)-> soil -(weather)-> wind [air, sea, land and life]
- wind -(shape, landslide, cutThroughCycle, wear)-> height -(weather)-> rain -(weather)-> wind [air, rock]
- wind -(shape, landslide, cutThroughCycle, wear)-> height -(history, pour, level, cutThroughCycle)-> sea -(weather)-> wind [air, sea, rock]
- wind -(shape, landslide, cutThroughCycle, wear)-> height -(settleHistory)-> floor -(weather)-> wind [air, rock]
- wind -(shape, landslide, cutThroughCycle, wear)-> height -(history, move, tectonics, keepBook, settleRock, handDown, settleHistory, layBedrock, expose, shape)-> rock -(weather)-> wind [air, rock]
- wind -(shape, landslide, cutThroughCycle, wear)-> height -(weather)-> moisture -(weather)-> wind [air, rock, water]
- wind -(shape, landslide, cutThroughCycle, wear)-> height -(history, move, handDown, landslide, cutThroughCycle, wear, silt, soilTexture, laySoil)-> soil -(weather)-> wind [air, rock, land and life]
- wind -(shape)-> rock -(weather)-> rain -(weather)-> wind [air, rock]
- wind -(shape)-> rock -(history, cutThroughCycle)-> sea -(weather)-> wind [air, sea, rock]
- wind -(shape)-> rock -(history, move, tectonics, handDown, settleHistory, shape, denude, landslide, cutThroughCycle, wear)-> height -(weather)-> wind [air, rock]
- wind -(shape)-> rock -(settleHistory)-> floor -(weather)-> wind [air, rock]
- wind -(shape)-> rock -(weather)-> moisture -(weather)-> wind [air, rock, water]
- wind -(shape)-> rock -(history, move, handDown, landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(weather)-> wind [air, rock, land and life]
- wind -(wear)-> book -(tectonics, handDown, settleHistory, wear)-> height -(weather)-> wind [air, rock]
- wind -(wear)-> book -(settleHistory)-> floor -(weather)-> wind [air, rock]
- wind -(wear)-> book -(tectonics, keepBook, handDown, settleHistory)-> rock -(weather)-> wind [air, rock]
- wind -(wear)-> book -(handDown, wear)-> soil -(weather)-> wind [air, rock, land and life]
- wind -(flow)-> drainage -(history)-> sea -(weather)-> wind [air, sea, water]
- wind -(flow)-> drainage -(history, wear)-> height -(weather)-> wind [air, rock, water]
- wind -(flow)-> drainage -(history, keepBook)-> rock -(weather)-> wind [air, rock, water]
- wind -(flow)-> drainage -(history, wear, laySoil)-> soil -(weather)-> wind [air, water, land and life]
- wind -(wear, waterStep)-> load -(wear)-> height -(weather)-> wind [air, rock, water]
- wind -(wear, waterStep)-> load -(wear)-> soil -(weather)-> wind [air, water, land and life]
- wind -(pool, flow)-> lakes -(denude, wear)-> height -(weather)-> wind [air, rock, water]
- wind -(pool, flow)-> lakes -(wear)-> soil -(weather)-> wind [air, water, land and life]
- wind -(landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(weather)-> rain -(weather)-> wind [air, land and life]
- wind -(landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(history, cutThroughCycle)-> sea -(weather)-> wind [air, sea, land and life]
- wind -(landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(history, move, handDown, landslide, cutThroughCycle, wear, silt)-> height -(weather)-> wind [air, rock, land and life]
- wind -(landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(history, move, keepBook, handDown)-> rock -(weather)-> wind [air, rock, land and life]
- wind -(landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(weather)-> moisture -(weather)-> wind [air, water, land and life]
- wind -(tides, cover)-> cover -(history, pour, level, cutThroughCycle)-> sea -(weather)-> wind [air, sea, land and life]
- wind -(tides, cover)-> cover -(history, shape, denude, landslide, cutThroughCycle, wear)-> height -(weather)-> wind [air, rock, land and life]
- wind -(tides, cover)-> cover -(history, keepBook, shape)-> rock -(weather)-> wind [air, rock, land and life]
- wind -(tides, cover)-> cover -(history, landslide, cutThroughCycle, wear, laySoil)-> soil -(weather)-> wind [air, land and life]
- rain -(cutThroughCycle)-> sea -(history, tectonics, shape, texture, denude, cutThroughCycle, wear, silt)-> height -(weather)-> rain [air, sea, rock]
- rain -(cutThroughCycle)-> sea -(history, tectonics, keepBook, shape)-> rock -(weather)-> rain [air, sea, rock]
- rain -(cutThroughCycle)-> sea -(weather)-> moisture -(weather)-> rain [air, sea, water]
- rain -(cutThroughCycle)-> sea -(history, cutThroughCycle, wear, silt)-> soil -(weather)-> rain [air, sea, land and life]
- rain -(tides)-> tide -(wear, silt)-> height -(weather)-> rain [air, sea, rock]
- rain -(tides)-> tide -(wear, silt)-> soil -(weather)-> rain [air, sea, land and life]
- rain -(shape, landslide, cutThroughCycle, wear)-> height -(history, pour, level, cutThroughCycle)-> sea -(weather)-> rain [air, sea, rock]
- rain -(shape, landslide, cutThroughCycle, wear)-> height -(settleHistory)-> floor -(weather)-> rain [air, rock]
- rain -(shape, landslide, cutThroughCycle, wear)-> height -(history, move, tectonics, keepBook, settleRock, handDown, settleHistory, layBedrock, expose, shape)-> rock -(weather)-> rain [air, rock]
- rain -(shape, landslide, cutThroughCycle, wear)-> height -(weather)-> moisture -(weather)-> rain [air, rock, water]
- rain -(shape, landslide, cutThroughCycle, wear)-> height -(history, move, handDown, landslide, cutThroughCycle, wear, silt, soilTexture, laySoil)-> soil -(weather)-> rain [air, rock, land and life]
- rain -(shape)-> rock -(history, cutThroughCycle)-> sea -(weather)-> rain [air, sea, rock]
- rain -(shape)-> rock -(history, move, tectonics, handDown, settleHistory, shape, denude, landslide, cutThroughCycle, wear)-> height -(weather)-> rain [air, rock]
- rain -(shape)-> rock -(settleHistory)-> floor -(weather)-> rain [air, rock]
- rain -(shape)-> rock -(weather)-> moisture -(weather)-> rain [air, rock, water]
- rain -(shape)-> rock -(history, move, handDown, landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(weather)-> rain [air, rock, land and life]
- rain -(wear)-> book -(tectonics, handDown, settleHistory, wear)-> height -(weather)-> rain [air, rock]
- rain -(wear)-> book -(settleHistory)-> floor -(weather)-> rain [air, rock]
- rain -(wear)-> book -(tectonics, keepBook, handDown, settleHistory)-> rock -(weather)-> rain [air, rock]
- rain -(wear)-> book -(handDown, wear)-> soil -(weather)-> rain [air, rock, land and life]
- rain -(flow)-> drainage -(history)-> sea -(weather)-> rain [air, sea, water]
- rain -(flow)-> drainage -(history, wear)-> height -(weather)-> rain [air, rock, water]
- rain -(flow)-> drainage -(history, keepBook)-> rock -(weather)-> rain [air, rock, water]
- rain -(flow)-> drainage -(history, wear, laySoil)-> soil -(weather)-> rain [air, water, land and life]
- rain -(wear, waterStep)-> load -(wear)-> height -(weather)-> rain [air, rock, water]
- rain -(wear, waterStep)-> load -(wear)-> soil -(weather)-> rain [air, water, land and life]
- rain -(pool, flow)-> lakes -(denude, wear)-> height -(weather)-> rain [air, rock, water]
- rain -(pool, flow)-> lakes -(wear)-> soil -(weather)-> rain [air, water, land and life]
- rain -(landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(history, cutThroughCycle)-> sea -(weather)-> rain [air, sea, land and life]
- rain -(landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(history, move, handDown, landslide, cutThroughCycle, wear, silt)-> height -(weather)-> rain [air, rock, land and life]
- rain -(landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(history, move, keepBook, handDown)-> rock -(weather)-> rain [air, rock, land and life]
- rain -(landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(weather)-> moisture -(weather)-> rain [air, water, land and life]
- rain -(tides, cover)-> cover -(history, pour, level, cutThroughCycle)-> sea -(weather)-> rain [air, sea, land and life]
- rain -(tides, cover)-> cover -(history, shape, denude, landslide, cutThroughCycle, wear)-> height -(weather)-> rain [air, rock, land and life]
- rain -(tides, cover)-> cover -(history, keepBook, shape)-> rock -(weather)-> rain [air, rock, land and life]
- rain -(tides, cover)-> cover -(history, landslide, cutThroughCycle, wear, laySoil)-> soil -(weather)-> rain [air, land and life]
- year -(tides)-> tide -(wear, silt)-> height -(year)-> year [air, sea, rock]
- year -(wear)-> height -(history, pour, level, cutThroughCycle)-> sea -(year)-> year [air, sea, rock]
- year -(wear)-> book -(tectonics, handDown, settleHistory, wear)-> height -(year)-> year [air, rock]
- year -(wear, waterStep)-> load -(wear)-> height -(year)-> year [air, rock, water]
- year -(wear, laySoil)-> soil -(history, cutThroughCycle)-> sea -(year)-> year [air, sea, land and life]
- year -(wear, laySoil)-> soil -(history, move, handDown, landslide, cutThroughCycle, wear, silt)-> height -(year)-> year [air, rock, land and life]
- year -(freeze, tides, cover)-> cover -(history, pour, level, cutThroughCycle)-> sea -(year)-> year [air, sea, land and life]
- year -(freeze, tides, cover)-> cover -(history, shape, denude, landslide, cutThroughCycle, wear)-> height -(year)-> year [air, rock, land and life]
- sea -(tides)-> tide -(wear, silt)-> height -(history, pour, level, cutThroughCycle)-> sea [sea, rock]
- sea -(tides)-> tide -(wear, silt)-> soil -(history, cutThroughCycle)-> sea [sea, land and life]
- sea -(tides)-> tide -(tides)-> cover -(history, pour, level, cutThroughCycle)-> sea [sea, land and life]
- sea -(history, tectonics, shape, texture, denude, cutThroughCycle, wear, silt)-> height -(settleHistory)-> floor -(pour, cutThroughCycle)-> sea [sea, rock]
- sea -(history, tectonics, shape, texture, denude, cutThroughCycle, wear, silt)-> height -(history, move, tectonics, keepBook, settleRock, handDown, settleHistory, layBedrock, expose, shape)-> rock -(history, cutThroughCycle)-> sea [sea, rock]
- sea -(history, tectonics, shape, texture, denude, cutThroughCycle, wear, silt)-> height -(history, move, tectonics, settleRock, handDown)-> plates -(history)-> sea [sea, rock]
- sea -(history, tectonics, shape, texture, denude, cutThroughCycle, wear, silt)-> height -(flow, height)-> drainage -(history)-> sea [sea, rock, water]
- sea -(history, tectonics, shape, texture, denude, cutThroughCycle, wear, silt)-> height -(history, move, handDown, landslide, cutThroughCycle, wear, silt, soilTexture, laySoil)-> soil -(history, cutThroughCycle)-> sea [sea, rock, land and life]
- sea -(history, tectonics, shape, texture, denude, cutThroughCycle, wear, silt)-> height -(move, handDown, pour, level, carve, freeze, tides, cover)-> cover -(history, pour, level, cutThroughCycle)-> sea [sea, rock, land and life]
- sea -(history, tectonics, keepBook, shape)-> rock -(history, move, tectonics, handDown, settleHistory, shape, denude, landslide, cutThroughCycle, wear)-> height -(history, pour, level, cutThroughCycle)-> sea [sea, rock]
- sea -(history, tectonics, keepBook, shape)-> rock -(settleHistory)-> floor -(pour, cutThroughCycle)-> sea [sea, rock]
- sea -(history, tectonics, keepBook, shape)-> rock -(history, move, tectonics, settleRock, handDown)-> plates -(history)-> sea [sea, rock]
- sea -(history, tectonics, keepBook, shape)-> rock -(history, move, handDown, landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(history, cutThroughCycle)-> sea [sea, rock, land and life]
- sea -(history, tectonics, keepBook, shape)-> rock -(move, handDown, tides)-> cover -(history, pour, level, cutThroughCycle)-> sea [sea, rock, land and life]
- sea -(history, tectonics)-> plates -(history, move, tectonics, handDown)-> height -(history, pour, level, cutThroughCycle)-> sea [sea, rock]
- sea -(history, tectonics)-> plates -(history, move, tectonics, settleRock, handDown)-> rock -(history, cutThroughCycle)-> sea [sea, rock]
- sea -(history, tectonics)-> plates -(history, move, handDown)-> soil -(history, cutThroughCycle)-> sea [sea, rock, land and life]
- sea -(history, tectonics)-> plates -(move, handDown)-> cover -(history, pour, level, cutThroughCycle)-> sea [sea, rock, land and life]
- sea -(history, tectonics, keepBook, wear)-> book -(tectonics, handDown, settleHistory, wear)-> height -(history, pour, level, cutThroughCycle)-> sea [sea, rock]
- sea -(history, tectonics, keepBook, wear)-> book -(settleHistory)-> floor -(pour, cutThroughCycle)-> sea [sea, rock]
- sea -(history, tectonics, keepBook, wear)-> book -(tectonics, keepBook, handDown, settleHistory)-> rock -(history, cutThroughCycle)-> sea [sea, rock]
- sea -(history, tectonics, keepBook, wear)-> book -(tectonics, handDown)-> plates -(history)-> sea [sea, rock]
- sea -(history, tectonics, keepBook, wear)-> book -(handDown, wear)-> soil -(history, cutThroughCycle)-> sea [sea, rock, land and life]
- sea -(history, tectonics, keepBook, wear)-> book -(handDown)-> cover -(history, pour, level, cutThroughCycle)-> sea [sea, rock, land and life]
- sea -(flow)-> drainage -(history, wear)-> height -(history, pour, level, cutThroughCycle)-> sea [sea, rock, water]
- sea -(flow)-> drainage -(history, keepBook)-> rock -(history, cutThroughCycle)-> sea [sea, rock, water]
- sea -(flow)-> drainage -(history)-> plates -(history)-> sea [sea, rock, water]
- sea -(flow)-> drainage -(history, wear, laySoil)-> soil -(history, cutThroughCycle)-> sea [sea, water, land and life]
- sea -(flow)-> drainage -(carve, tides, cover)-> cover -(history, pour, level, cutThroughCycle)-> sea [sea, water, land and life]
- sea -(wear, waterStep)-> load -(wear)-> height -(history, pour, level, cutThroughCycle)-> sea [sea, rock, water]
- sea -(wear, waterStep)-> load -(wear)-> soil -(history, cutThroughCycle)-> sea [sea, water, land and life]
- sea -(pool, flow)-> lakes -(denude, wear)-> height -(history, pour, level, cutThroughCycle)-> sea [sea, rock, water]
- sea -(pool, flow)-> lakes -(flow, height)-> drainage -(history)-> sea [sea, water]
- sea -(pool, flow)-> lakes -(wear)-> soil -(history, cutThroughCycle)-> sea [sea, water, land and life]
- sea -(pool, flow)-> lakes -(carve, freeze, tides)-> cover -(history, pour, level, cutThroughCycle)-> sea [sea, water, land and life]
- sea -(history, cutThroughCycle, wear, silt)-> soil -(history, move, handDown, landslide, cutThroughCycle, wear, silt)-> height -(history, pour, level, cutThroughCycle)-> sea [sea, rock, land and life]
- sea -(history, cutThroughCycle, wear, silt)-> soil -(history, move, keepBook, handDown)-> rock -(history, cutThroughCycle)-> sea [sea, rock, land and life]
- sea -(history, cutThroughCycle, wear, silt)-> soil -(history, move, handDown)-> plates -(history)-> sea [sea, rock, land and life]
- sea -(history, cutThroughCycle, wear, silt)-> soil -(move, handDown, tides, cover)-> cover -(history, pour, level, cutThroughCycle)-> sea [sea, land and life]
- sea -(pour, level, carve, tides)-> cover -(history, shape, denude, landslide, cutThroughCycle, wear)-> height -(history, pour, level, cutThroughCycle)-> sea [sea, rock, land and life]
- sea -(pour, level, carve, tides)-> cover -(history, keepBook, shape)-> rock -(history, cutThroughCycle)-> sea [sea, rock, land and life]
- sea -(pour, level, carve, tides)-> cover -(history)-> plates -(history)-> sea [sea, rock, land and life]
- sea -(pour, level, carve, tides)-> cover -(height)-> drainage -(history)-> sea [sea, water, land and life]
- sea -(pour, level, carve, tides)-> cover -(history, landslide, cutThroughCycle, wear, laySoil)-> soil -(history, cutThroughCycle)-> sea [sea, land and life]
- sea -(pour, level, carve, tides)-> woods -(cover)-> cover -(history, pour, level, cutThroughCycle)-> sea [sea, land and life]
- sea -(tides)-> fertility -(cover)-> cover -(history, pour, level, cutThroughCycle)-> sea [sea, land and life]
- tide -(wear, silt)-> height -(settleHistory)-> floor -(tides)-> tide [sea, rock]
- tide -(wear, silt)-> height -(history, move, tectonics, keepBook, settleRock, handDown, settleHistory, layBedrock, expose, shape)-> rock -(tides)-> tide [sea, rock]
- tide -(wear, silt)-> height -(flow, height)-> drainage -(tides)-> tide [sea, rock, water]
- tide -(wear, silt)-> height -(pool, flow)-> lakes -(tides)-> tide [sea, rock, water]
- tide -(wear, silt)-> height -(history, move, handDown, landslide, cutThroughCycle, wear, silt, soilTexture, laySoil)-> soil -(tides)-> tide [sea, rock, land and life]
- tide -(wear, silt)-> height -(move, handDown, pour, level, carve, freeze, tides, cover)-> cover -(tides)-> tide [sea, rock, land and life]
- tide -(wear)-> book -(tectonics, handDown, settleHistory, wear)-> height -(tides)-> tide [sea, rock]
- tide -(wear)-> book -(settleHistory)-> floor -(tides)-> tide [sea, rock]
- tide -(wear)-> book -(tectonics, keepBook, handDown, settleHistory)-> rock -(tides)-> tide [sea, rock]
- tide -(wear)-> book -(handDown, wear)-> soil -(tides)-> tide [sea, rock, land and life]
- tide -(wear)-> book -(handDown)-> cover -(tides)-> tide [sea, rock, land and life]
- tide -(wear, waterStep)-> load -(wear)-> height -(tides)-> tide [sea, rock, water]
- tide -(wear, waterStep)-> load -(wear)-> soil -(tides)-> tide [sea, water, land and life]
- tide -(wear, silt)-> soil -(history, move, handDown, landslide, cutThroughCycle, wear, silt)-> height -(tides)-> tide [sea, rock, land and life]
- tide -(wear, silt)-> soil -(history, move, keepBook, handDown)-> rock -(tides)-> tide [sea, rock, land and life]
- tide -(wear, silt)-> soil -(move, handDown, tides, cover)-> cover -(tides)-> tide [sea, land and life]
- tide -(tides)-> cover -(history, shape, denude, landslide, cutThroughCycle, wear)-> height -(tides)-> tide [sea, rock, land and life]
- tide -(tides)-> cover -(history, keepBook, shape)-> rock -(tides)-> tide [sea, rock, land and life]
- tide -(tides)-> cover -(height)-> drainage -(tides)-> tide [sea, water, land and life]
- tide -(tides)-> cover -(history, landslide, cutThroughCycle, wear, laySoil)-> soil -(tides)-> tide [sea, land and life]
- tide -(tides)-> woods -(cover)-> cover -(tides)-> tide [sea, land and life]
- tide -(tides)-> fertility -(cover)-> cover -(tides)-> tide [sea, land and life]
- height -(settleHistory)-> floor -(settleHistory, shape)-> rock -(history, move, tectonics, handDown, settleHistory, shape, denude, landslide, cutThroughCycle, wear)-> height [rock]
- height -(settleHistory)-> floor -(wear)-> book -(tectonics, handDown, settleHistory, wear)-> height [rock]
- height -(settleHistory)-> floor -(flow, height)-> drainage -(history, wear)-> height [rock, water]
- height -(settleHistory)-> floor -(wear)-> load -(wear)-> height [rock, water]
- height -(settleHistory)-> floor -(pool, flow)-> lakes -(denude, wear)-> height [rock, water]
- height -(settleHistory)-> floor -(landslide, cutThroughCycle, wear, laySoil)-> soil -(history, move, handDown, landslide, cutThroughCycle, wear, silt)-> height [rock, land and life]
- height -(settleHistory)-> floor -(pour, freeze, tides, cover)-> cover -(history, shape, denude, landslide, cutThroughCycle, wear)-> height [rock, land and life]
- height -(history, move, tectonics, keepBook, settleRock, handDown, settleHistory, layBedrock, expose, shape)-> rock -(settleHistory)-> floor -(settleHistory, shape, texture, landslide, cutThroughCycle, wear)-> height [rock]
- height -(history, move, tectonics, keepBook, settleRock, handDown, settleHistory, layBedrock, expose, shape)-> rock -(history, move, tectonics, settleRock, handDown)-> plates -(history, move, tectonics, handDown)-> height [rock]
- height -(history, move, tectonics, keepBook, settleRock, handDown, settleHistory, layBedrock, expose, shape)-> rock -(history, tectonics, keepBook, handDown, wear)-> book -(tectonics, handDown, settleHistory, wear)-> height [rock]
- height -(history, move, tectonics, keepBook, settleRock, handDown, settleHistory, layBedrock, expose, shape)-> rock -(wear, waterStep)-> load -(wear)-> height [rock, water]
- height -(history, move, tectonics, keepBook, settleRock, handDown, settleHistory, layBedrock, expose, shape)-> rock -(history, move, handDown, landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(history, move, handDown, landslide, cutThroughCycle, wear, silt)-> height [rock, land and life]
- height -(history, move, tectonics, keepBook, settleRock, handDown, settleHistory, layBedrock, expose, shape)-> rock -(move, handDown, tides)-> cover -(history, shape, denude, landslide, cutThroughCycle, wear)-> height [rock, land and life]
- height -(history, move, tectonics, settleRock, handDown)-> plates -(history, move, tectonics, settleRock, handDown)-> rock -(history, move, tectonics, handDown, settleHistory, shape, denude, landslide, cutThroughCycle, wear)-> height [rock]
- height -(history, move, tectonics, settleRock, handDown)-> plates -(history, tectonics, handDown)-> book -(tectonics, handDown, settleHistory, wear)-> height [rock]
- height -(history, move, tectonics, settleRock, handDown)-> plates -(history, move, handDown)-> soil -(history, move, handDown, landslide, cutThroughCycle, wear, silt)-> height [rock, land and life]
- height -(history, move, tectonics, settleRock, handDown)-> plates -(move, handDown)-> cover -(history, shape, denude, landslide, cutThroughCycle, wear)-> height [rock, land and life]
- height -(history, tectonics, keepBook, handDown, wear)-> book -(settleHistory)-> floor -(settleHistory, shape, texture, landslide, cutThroughCycle, wear)-> height [rock]
- height -(history, tectonics, keepBook, handDown, wear)-> book -(tectonics, keepBook, handDown, settleHistory)-> rock -(history, move, tectonics, handDown, settleHistory, shape, denude, landslide, cutThroughCycle, wear)-> height [rock]
- height -(history, tectonics, keepBook, handDown, wear)-> book -(tectonics, handDown)-> plates -(history, move, tectonics, handDown)-> height [rock]
- height -(history, tectonics, keepBook, handDown, wear)-> book -(wear)-> load -(wear)-> height [rock, water]
- height -(history, tectonics, keepBook, handDown, wear)-> book -(handDown, wear)-> soil -(history, move, handDown, landslide, cutThroughCycle, wear, silt)-> height [rock, land and life]
- height -(history, tectonics, keepBook, handDown, wear)-> book -(handDown)-> cover -(history, shape, denude, landslide, cutThroughCycle, wear)-> height [rock, land and life]
- height -(flow, height)-> drainage -(history, keepBook)-> rock -(history, move, tectonics, handDown, settleHistory, shape, denude, landslide, cutThroughCycle, wear)-> height [rock, water]
- height -(flow, height)-> drainage -(history)-> plates -(history, move, tectonics, handDown)-> height [rock, water]
- height -(flow, height)-> drainage -(history, keepBook, wear)-> book -(tectonics, handDown, settleHistory, wear)-> height [rock, water]
- height -(flow, height)-> drainage -(wear, waterStep)-> load -(wear)-> height [rock, water]
- height -(flow, height)-> drainage -(flow)-> lakes -(denude, wear)-> height [rock, water]
- height -(flow, height)-> drainage -(history, wear, laySoil)-> soil -(history, move, handDown, landslide, cutThroughCycle, wear, silt)-> height [rock, water, land and life]
- height -(flow, height)-> drainage -(carve, tides, cover)-> cover -(history, shape, denude, landslide, cutThroughCycle, wear)-> height [rock, water, land and life]
- height -(wear, waterStep)-> load -(wear)-> book -(tectonics, handDown, settleHistory, wear)-> height [rock, water]
- height -(wear, waterStep)-> load -(wear)-> soil -(history, move, handDown, landslide, cutThroughCycle, wear, silt)-> height [rock, water, land and life]
- height -(pool, flow)-> lakes -(wear)-> book -(tectonics, handDown, settleHistory, wear)-> height [rock, water]
- height -(pool, flow)-> lakes -(flow, height)-> drainage -(history, wear)-> height [rock, water]
- height -(pool, flow)-> lakes -(wear, waterStep)-> load -(wear)-> height [rock, water]
- height -(pool, flow)-> lakes -(wear)-> soil -(history, move, handDown, landslide, cutThroughCycle, wear, silt)-> height [rock, water, land and life]
- height -(pool, flow)-> lakes -(carve, freeze, tides)-> cover -(history, shape, denude, landslide, cutThroughCycle, wear)-> height [rock, water, land and life]
- height -(history, move, handDown, landslide, cutThroughCycle, wear, silt, soilTexture, laySoil)-> soil -(history, move, keepBook, handDown)-> rock -(history, move, tectonics, handDown, settleHistory, shape, denude, landslide, cutThroughCycle, wear)-> height [rock, land and life]
- height -(history, move, handDown, landslide, cutThroughCycle, wear, silt, soilTexture, laySoil)-> soil -(history, move, handDown)-> plates -(history, move, tectonics, handDown)-> height [rock, land and life]
- height -(history, move, handDown, landslide, cutThroughCycle, wear, silt, soilTexture, laySoil)-> soil -(history, keepBook, handDown, wear)-> book -(tectonics, handDown, settleHistory, wear)-> height [rock, land and life]
- height -(history, move, handDown, landslide, cutThroughCycle, wear, silt, soilTexture, laySoil)-> soil -(wear, waterStep)-> load -(wear)-> height [rock, water, land and life]
- height -(history, move, handDown, landslide, cutThroughCycle, wear, silt, soilTexture, laySoil)-> soil -(move, handDown, tides, cover)-> cover -(history, shape, denude, landslide, cutThroughCycle, wear)-> height [rock, land and life]
- height -(move, handDown, pour, level, carve, freeze, tides, cover)-> cover -(history, keepBook, shape)-> rock -(history, move, tectonics, handDown, settleHistory, shape, denude, landslide, cutThroughCycle, wear)-> height [rock, land and life]
- height -(move, handDown, pour, level, carve, freeze, tides, cover)-> cover -(history)-> plates -(history, move, tectonics, handDown)-> height [rock, land and life]
- height -(move, handDown, pour, level, carve, freeze, tides, cover)-> cover -(history, keepBook, wear)-> book -(tectonics, handDown, settleHistory, wear)-> height [rock, land and life]
- height -(move, handDown, pour, level, carve, freeze, tides, cover)-> cover -(height)-> drainage -(history, wear)-> height [rock, water, land and life]
- height -(move, handDown, pour, level, carve, freeze, tides, cover)-> cover -(wear, waterStep)-> load -(wear)-> height [rock, water, land and life]
- height -(move, handDown, pour, level, carve, freeze, tides, cover)-> cover -(history, landslide, cutThroughCycle, wear, laySoil)-> soil -(history, move, handDown, landslide, cutThroughCycle, wear, silt)-> height [rock, land and life]
- height -(pour, level, carve, tides, cover, readWoods)-> woods -(cover)-> cover -(history, shape, denude, landslide, cutThroughCycle, wear)-> height [rock, land and life]
- height -(tides, cover)-> fertility -(cover)-> cover -(history, shape, denude, landslide, cutThroughCycle, wear)-> height [rock, land and life]
- floor -(settleHistory, shape)-> rock -(history, tectonics, keepBook, handDown, wear)-> book -(settleHistory)-> floor [rock]
- floor -(wear)-> book -(tectonics, keepBook, handDown, settleHistory)-> rock -(settleHistory)-> floor [rock]
- floor -(flow, height)-> drainage -(history, keepBook)-> rock -(settleHistory)-> floor [rock, water]
- floor -(flow, height)-> drainage -(history, keepBook, wear)-> book -(settleHistory)-> floor [rock, water]
- floor -(wear)-> load -(wear)-> book -(settleHistory)-> floor [rock, water]
- floor -(pool, flow)-> lakes -(wear)-> book -(settleHistory)-> floor [rock, water]
- floor -(landslide, cutThroughCycle, wear, laySoil)-> soil -(history, move, keepBook, handDown)-> rock -(settleHistory)-> floor [rock, land and life]
- floor -(landslide, cutThroughCycle, wear, laySoil)-> soil -(history, keepBook, handDown, wear)-> book -(settleHistory)-> floor [rock, land and life]
- floor -(pour, freeze, tides, cover)-> cover -(history, keepBook, shape)-> rock -(settleHistory)-> floor [rock, land and life]
- floor -(pour, freeze, tides, cover)-> cover -(history, keepBook, wear)-> book -(settleHistory)-> floor [rock, land and life]
- rock -(history, move, tectonics, settleRock, handDown)-> plates -(history, tectonics, handDown)-> book -(tectonics, keepBook, handDown, settleHistory)-> rock [rock]
- rock -(history, move, tectonics, settleRock, handDown)-> plates -(history, move, handDown)-> soil -(history, move, keepBook, handDown)-> rock [rock, land and life]
- rock -(history, move, tectonics, settleRock, handDown)-> plates -(move, handDown)-> cover -(history, keepBook, shape)-> rock [rock, land and life]
- rock -(history, tectonics, keepBook, handDown, wear)-> book -(tectonics, handDown)-> plates -(history, move, tectonics, settleRock, handDown)-> rock [rock]
- rock -(history, tectonics, keepBook, handDown, wear)-> book -(handDown, wear)-> soil -(history, move, keepBook, handDown)-> rock [rock, land and life]
- rock -(history, tectonics, keepBook, handDown, wear)-> book -(handDown)-> cover -(history, keepBook, shape)-> rock [rock, land and life]
- rock -(wear, waterStep)-> load -(wear)-> book -(tectonics, keepBook, handDown, settleHistory)-> rock [rock, water]
- rock -(wear, waterStep)-> load -(wear)-> soil -(history, move, keepBook, handDown)-> rock [rock, water, land and life]
- rock -(history, move, handDown, landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(history, move, handDown)-> plates -(history, move, tectonics, settleRock, handDown)-> rock [rock, land and life]
- rock -(history, move, handDown, landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(history, keepBook, handDown, wear)-> book -(tectonics, keepBook, handDown, settleHistory)-> rock [rock, land and life]
- rock -(history, move, handDown, landslide, cutThroughCycle, wear, soilTexture, laySoil)-> soil -(move, handDown, tides, cover)-> cover -(history, keepBook, shape)-> rock [rock, land and life]
- rock -(move, handDown, tides)-> cover -(history)-> plates -(history, move, tectonics, settleRock, handDown)-> rock [rock, land and life]
- rock -(move, handDown, tides)-> cover -(history, keepBook, wear)-> book -(tectonics, keepBook, handDown, settleHistory)-> rock [rock, land and life]
- rock -(move, handDown, tides)-> cover -(height)-> drainage -(history, keepBook)-> rock [rock, water, land and life]
- rock -(move, handDown, tides)-> cover -(history, landslide, cutThroughCycle, wear, laySoil)-> soil -(history, move, keepBook, handDown)-> rock [rock, land and life]
- rock -(tides)-> woods -(cover)-> cover -(history, keepBook, shape)-> rock [rock, land and life]
- rock -(tides)-> fertility -(cover)-> cover -(history, keepBook, shape)-> rock [rock, land and life]
- plates -(history, tectonics, handDown)-> book -(handDown, wear)-> soil -(history, move, handDown)-> plates [rock, land and life]
- plates -(history, tectonics, handDown)-> book -(handDown)-> cover -(history)-> plates [rock, land and life]
- plates -(history, move, handDown)-> soil -(history, keepBook, handDown, wear)-> book -(tectonics, handDown)-> plates [rock, land and life]
- plates -(history, move, handDown)-> soil -(move, handDown, tides, cover)-> cover -(history)-> plates [rock, land and life]
- plates -(move, handDown)-> cover -(history, keepBook, wear)-> book -(tectonics, handDown)-> plates [rock, land and life]
- plates -(move, handDown)-> cover -(height)-> drainage -(history)-> plates [rock, water, land and life]
- plates -(move, handDown)-> cover -(history, landslide, cutThroughCycle, wear, laySoil)-> soil -(history, move, handDown)-> plates [rock, land and life]
- book -(wear)-> load -(wear)-> soil -(history, keepBook, handDown, wear)-> book [rock, water, land and life]
- book -(handDown, wear)-> soil -(wear, waterStep)-> load -(wear)-> book [rock, water, land and life]
- book -(handDown, wear)-> soil -(move, handDown, tides, cover)-> cover -(history, keepBook, wear)-> book [rock, land and life]
- book -(handDown)-> cover -(height)-> drainage -(history, keepBook, wear)-> book [rock, water, land and life]
- book -(handDown)-> cover -(wear, waterStep)-> load -(wear)-> book [rock, water, land and life]
- book -(handDown)-> cover -(history, landslide, cutThroughCycle, wear, laySoil)-> soil -(history, keepBook, handDown, wear)-> book [rock, land and life]
- drainage -(flow)-> lakes -(carve, freeze, tides)-> cover -(height)-> drainage [water, land and life]
- drainage -(history, wear, laySoil)-> soil -(move, handDown, tides, cover)-> cover -(height)-> drainage [water, land and life]
- drainage -(carve, tides, cover, readWoods)-> woods -(cover)-> cover -(height)-> drainage [water, land and life]
- drainage -(tides, cover)-> fertility -(cover)-> cover -(height)-> drainage [water, land and life]
- load -(wear)-> soil -(move, handDown, tides, cover)-> cover -(wear, waterStep)-> load [water, land and life]
- soil -(tides, cover)-> woods -(cover)-> cover -(history, landslide, cutThroughCycle, wear, laySoil)-> soil [land and life]
- soil -(tides, cover)-> fertility -(cover)-> cover -(history, landslide, cutThroughCycle, wear, laySoil)-> soil [land and life]
- cover -(pour, level, carve, tides, cover, readWoods)-> woods -(cover)-> fertility -(cover)-> cover [land and life]
- cover -(tides, cover)-> fertility -(cover)-> woods -(cover)-> cover [land and life]

</details>

## Loops within a pass

Loops a pass closes within one field, between quantities the graph above holds as one, each driving the next and the last the first, and what solves them.

- **weather**, within wind (#28): the sea's warmth (atmos.Env.Warm, WaterTemp) -> the air's pressure: over the sea in the warmth it is read off, the trades' layer's and the rain's heating's in the tropics (Winds.P, Env.Walk) -> the wind (Winds.U, Winds.V) -> the currents, the thermocline and the upwelling (Env.Cu, Cv, Psi, Thermocline, Rise) -> back to the first. Solved by atmos.Winds.couple: coupleRounds rounds a reading of the weather, damped, from where the last reading over the same map left the sea's warmth.
