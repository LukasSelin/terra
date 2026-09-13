# terra

A world, without a game on it.

The land is made out of its own history: plates that collided, rock that
dates from the collision, water that has had an age to find its way down it.
What comes out is a map with heights and rivers and bedrock and soil, weather
that goes by latitude and by height, woods that stand where trees would
stand, and a way across all of it for anybody who wants to walk somewhere.

There is nobody on it. That is the point. A world is worth more than the
first game played on it, and a settlement of farmers, a party of adventurers
and a map nobody plays on at all want the same continents.

```go
land := terra.NewLand(seed, terra.DefaultTerms())
for day := 0; ; day++ {
	land.Tick++
	land.Wake(terra.Waking{Peopled: func(chunk int) bool { return false }})
}
```

## What it knows

The ground - height, the water running over it in cubic metres a second,
drainage, the rock under it and the sand and clay over that, which plate it
rides and which age its rock dates from. The weather over it, by latitude and
by height and by season, and the rain: carried off the sea by the wind belts,
wrung out over the ranges, and taken back by the warmth of the air, so that
rivers are where the water runs off and has the power to cut, and the ground
wears the way stream power says it does. What grows on it
and how far along it has come. How worn a path is and how fast it fades.
Where a walker can get to, and the cheapest way there, with landmark bounds
and a wrapping map if the world is a globe.

## What it does not know

What a house is, what a market is, what anybody wants, or that a settlement
exists anywhere. It carries what a game puts on the ground as a number it
never looks inside, and it is told, once, what those numbers do:

```go
terra.SetMark(door, terra.MarkDef{Cost: 1.2, Roofs: true})
terra.SetGrowth(terra.None, terra.Forest, []terra.Growth{{Full: 6 * terra.Year, Rate: 0.0004, Stock: timber}})
terra.SetWear(1, 1)
```

and two things it is handed by whoever is walking: a `Holder`, which is a
bare identity it compares and never looks inside, and a load.

## Building on it

It depends on nothing but the standard library, and it is expected to sit
beside whatever is built on it until it is published:

```
repos/
  terra/
  yourgame/
```

with `replace github.com/LukasSelin/terra => ../terra` in your `go.mod`.

The day's pass over the ground can be done four tiles at a time on a
processor with AVX2, through Go's experimental vector packages, which exist
only when the go command is asked for them:

```bash
GOEXPERIMENT=simd go test ./...
```

What a world does is the same either way, to the last bit, and a test holds
the vector arithmetic to the tile-by-tile statement of it.

## Looking at one

```bash
go run ./cmd/overview -preset globe -seed 3
```

makes a world and writes `overview/index.html`: its terrain, biomes, landforms, height,
drainage, rain, runoff, bedrock, plates, rock age, soil, temperature and woods, one map
each, beside the numbers. `-preset` is `valley`, `ancient` or `globe`, and
`-w`, `-h`, `-epochs`, `-sea`, `-wrap`, `-scale` and `-out` override it.
`-max` makes the world as big as the free memory allows, in the shape the
preset or `-w` and `-h` give it.

## Where it came from

It was the world half of [lreat](https://github.com/LukasSelin/lreat), a
simulation of people living in one, and its history up to the split is
there. It was taken out because the settlement was only ever the first game
on it.
