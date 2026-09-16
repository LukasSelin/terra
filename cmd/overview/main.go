// overview makes a world and draws it: one picture for each thing the land
// knows about itself - its ground, its height, the rock under it, the plates
// it rides and the age they date from, its soil, its weather and its woods -
// laid out on one page beside the numbers that describe the whole.
//
//	go run ./cmd/overview                      the valley, drawn
//	go run ./cmd/overview -preset ancient      the valley, made from its history
//	go run ./cmd/overview -preset globe -scale 1
//	go run ./cmd/overview -seed 7 -w 256 -h 128 -epochs 16 -sea 0.3
//	go run ./cmd/overview -preset globe -max    the biggest globe memory allows
//
// It writes into -out (overview/ by default) an index.html and a png per
// layer, and prints a summary to the terminal.
//
//	go run ./cmd/overview -serve :8080         a page with a form that makes one
//
// With -serve it makes no world until asked: the page's form holds the
// options above, filled in with the flags given; each press of its button
// makes that world into its own directory under -runs, and the browser is
// sent to its page. See serve.go.
package main

import (
	"flag"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/terra/geom"
)

// options is what a world is made and drawn from: the flags on the command
// line, or what the server was started with.
type options struct {
	Seed   uint64
	Preset string
	// W, H, Epochs, Sea and Water override the preset where they are set:
	// W and H above nought, the rest at nought or above.
	W, H       int
	Epochs     int
	Sea, Water float64
	Wrap       bool
	// Scale is pixels per tile, 0 to pick one; Day is the day whose weather
	// is drawn; Max makes the world as big as memory allows.
	Scale, Day int
	Max        bool
}

func main() {
	var o options
	flag.Uint64Var(&o.Seed, "seed", 1, "the seed the world is made from")
	flag.StringVar(&o.Preset, "preset", "valley", "valley, ancient or globe")
	flag.IntVar(&o.W, "w", 0, "width in tiles (overrides the preset)")
	flag.IntVar(&o.H, "h", 0, "height in tiles (overrides the preset)")
	flag.IntVar(&o.Epochs, "epochs", -1, "ages of history to run (overrides the preset)")
	flag.Float64Var(&o.Sea, "sea", -1, "share of the ground under the sea, for a drawn map or a made one given no water (overrides the preset)")
	flag.Float64Var(&o.Water, "water", -1, "metres of water a made world is given, spread over the whole map; 0 floods by -sea instead (overrides the preset)")
	flag.BoolVar(&o.Wrap, "wrap", false, "join the east edge to the west (forced on by -preset globe)")
	flag.IntVar(&o.Scale, "scale", 0, "pixels per tile (0 picks one)")
	flag.IntVar(&o.Day, "day", 30, "the day of the world whose weather is drawn")
	flag.BoolVar(&o.Max, "max", false, "make the world as big as memory allows, in the shape of the preset or of -w and -h")
	var (
		out   = flag.String("out", "overview", "directory to write into")
		serve = flag.String("serve", "", "serve a page that makes worlds at this address (e.g. :8080) instead of making one; the other flags are what it makes")
		runs  = flag.String("runs", "runs", "directory the server writes each world into, one directory a world")
	)
	flag.Parse()

	if *serve != "" {
		fail(listen(*serve, *runs, o))
	}
	if _, _, err := generate(o, *out, nil); err != nil {
		fail(err)
	}
}

// terms is the preset the options name, with their overrides laid over it.
func (o options) terms() (terra.Terms, error) {
	var t terra.Terms
	switch o.Preset {
	case "valley":
		t = terra.DefaultTerms()
	case "ancient":
		t = terra.AncientTerms()
	case "globe":
		t = terra.GlobeTerms()
	default:
		return t, fmt.Errorf("unknown preset %q: want valley, ancient or globe", o.Preset)
	}
	if o.W > 0 {
		t.Width = o.W
	}
	if o.H > 0 {
		t.Height = o.H
	}
	if o.Epochs >= 0 {
		t.Epochs = o.Epochs
	}
	if o.Sea >= 0 {
		t.SeaShare = o.Sea
	}
	if o.Water >= 0 {
		t.Water = o.Water
	}
	if o.Wrap {
		t.Wrap = true
	}
	if o.Max {
		return t.Largest()
	}
	return t, nil
}

// generate makes the world the options describe and draws it into out: an
// index.html, a why.html and a png per layer. It prints a summary to the
// terminal as it goes and returns the world and the path of index.html.
// Where stage is not nil it is told what is being done, as it starts.
//
// It sets the package's namer, so no two may run at once.
func generate(o options, out string, stage func(string)) (*terra.Land, string, error) {
	if stage == nil {
		stage = func(string) {}
	}
	land, t, took, err := makeWorld(o, stage)
	if err != nil {
		return nil, "", err
	}
	page, err := draw(land, o, t, took, out, stage)
	return land, page, err
}

// makeWorld makes the world the options describe and runs its weather up to
// the day they ask for: everything a drawing or a tile's account reads. The
// same options make the same world, every time.
//
// It sets the package's namer, so no two may run at once.
func makeWorld(o options, stage func(string)) (*terra.Land, terra.Terms, time.Duration, error) {
	t, err := o.terms()
	if err != nil {
		return nil, t, 0, err
	}

	fmt.Printf("making a %dx%d world from seed %d (epochs %d, sea %.2f, water %.1f m, wrap %v)...\n", t.Width, t.Height, o.Seed, t.Epochs, t.SeaShare, t.Water, t.Wrap)
	stage("making the world")
	start := time.Now()
	terra.SetNamer(namerFor(o.Seed))
	land, err := terra.MakeLand(o.Seed, t)
	if err != nil {
		return nil, t, 0, err
	}
	took := time.Since(start)
	fmt.Printf("made in %v\n\n", took.Round(time.Millisecond))

	// With TERRA_PHASES=1, the clock by pass, longest first.
	if phases := terra.Phases(); len(phases) > 0 {
		fmt.Print(terra.PhaseTable(phases), "\n")
	}

	// The day's weather, run from the founding up to the day asked for.
	stage(fmt.Sprintf("running the weather to day %d", o.Day))
	for tick := 0; tick <= max(o.Day, 0); tick++ {
		land.Tick = tick
		land.Climate.Advance(tick, land.RNG)
		land.AdvanceWeather()
	}
	return land, t, took, nil
}

// draw draws a made world into out. See generate.
func draw(land *terra.Land, o options, t terra.Terms, took time.Duration, out string, stage func(string)) (string, error) {
	px := o.Scale
	if px <= 0 {
		px = max(1, min(12, 1024/max(t.Width, 1)))
	}

	if err := os.MkdirAll(out, 0o755); err != nil {
		return "", err
	}
	g := land.Grid
	stage("measuring the world")
	stats := measure(land)
	cls := classify(land)
	stats.Biomes = legendOf(cls.Biome, biomeClasses, biomeOf)
	stats.Forms = legendOf(cls.Form, formClasses, formOf)
	stats.print()

	var layers []layer
	all := drawings(land, stats, cls)
	for k, l := range all {
		stage(fmt.Sprintf("drawing %s, %d of %d", l.title, k+1, len(all)))
		img := render(g, px, l.color)
		if l.overlay != nil {
			l.overlay(img, px)
		}
		name := l.file + ".png"
		if err := writePNG(filepath.Join(out, name), img); err != nil {
			return "", err
		}
		layers = append(layers, layer{Title: l.title, File: name, About: l.about, Legend: l.legend})
	}

	page := filepath.Join(out, "index.html")
	f, err := os.Create(page)
	if err != nil {
		return "", err
	}
	err = pageTmpl.Execute(f, struct {
		Seed   uint64
		Terms  terra.Terms
		Preset string
		Took   string
		Stats  summary
		Layers []layer
		Width  int
		W, H   int
	}{o.Seed, t, o.Preset, took.Round(time.Millisecond).String(), stats, layers, t.Width * px, t.Width, t.Height})
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	// And the world's account of eight of its tiles. See why.go.
	stage("writing why.html")
	if err := writeWhy(filepath.Join(out, "why.html"), land, o.Seed, o.Preset); err != nil {
		return "", err
	}
	abs, _ := filepath.Abs(page)
	fmt.Printf("\nwrote %d maps, why.html and %s\n", len(layers), abs)
	return page, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "overview:", err)
	os.Exit(1)
}

// summary is the world in numbers.
type summary struct {
	Tiles                           int
	Terrain                         []share
	Bedrock                         []share
	Plates                          int
	HeightMin, HeightP50, HeightMax float64
	TempMin, TempMax                float64
	Frozen                          share
	Rivers                          share
	FlowMax                         float64
	LandRain, LandRunoff            float64
	Moon                            string
	RangeP50, RangeP90, RangeMax    float64
	Flats                           share
	SeaPct                          float64
	AgeMax                          int
	Waterfalls                      int
	Biomes, Forms                   []share
	// FloorMin is the deepest ground under the water, which on a watered
	// globe is kilometres below the dry ground HeightMin reads.
	FloorMin float64
}

type share struct {
	Name  string
	Count int
	Pct   float64
	Color string
}

func measure(land *terra.Land) summary {
	g := land.Grid
	n := len(g.Tiles)
	s := summary{Tiles: n, HeightMin: math.Inf(1), HeightMax: math.Inf(-1), FloorMin: math.Inf(1), TempMin: math.Inf(1), TempMax: math.Inf(-1)}
	terr := make([]int, terra.TerrainCount)
	rock := make([]int, terra.BedrockCount)
	plates := map[uint8]bool{}
	heights := make([]float64, 0, n)
	frozen, rivers, dry := 0, 0, 0
	for i := range g.Tiles {
		t := &g.Tiles[i]
		p := g.PosOf(i)
		terr[t.Terrain]++
		if !t.Wet() {
			rock[t.Bedrock]++
			heights = append(heights, g.Height[i])
			s.HeightMin = math.Min(s.HeightMin, g.Height[i])
			s.LandRain += g.Rain(i)
			s.LandRunoff += g.Runoff(i)
			dry++
		}
		plates[t.Plate] = true
		s.FloorMin = math.Min(s.FloorMin, g.Height[i])
		s.HeightMax = math.Max(s.HeightMax, g.Height[i])
		s.FlowMax = math.Max(s.FlowMax, g.Flow[i])
		s.AgeMax = max(s.AgeMax, int(t.Formed))
		temp := land.TempAt(p)
		s.TempMin = math.Min(s.TempMin, temp)
		s.TempMax = math.Max(s.TempMax, temp)
		if g.Frozen(p) {
			frozen++
		}
		if t.Terrain == terra.Water && g.Flow[i] > riverFlow {
			rivers++
		}
	}
	if dry > 0 {
		s.LandRain /= float64(dry)
		s.LandRunoff /= float64(dry)
	}
	moon := land.Moon()
	s.Moon = fmt.Sprintf("%s, %.1f days old", moon.Name(), moon.Age)
	var ranges []float64
	for i := range g.Tiles {
		if t := &g.Tiles[i]; !t.Wet() || t.Terrain == terra.Flat {
			if r := g.TidalRange(g.PosOf(i)); r > 0 {
				ranges = append(ranges, r)
			}
		}
	}
	sort.Float64s(ranges)
	if len(ranges) > 0 {
		s.RangeP50, s.RangeP90, s.RangeMax = ranges[len(ranges)/2], ranges[len(ranges)*9/10], ranges[len(ranges)-1]
	}
	s.Flats = shareOf("tidal flats", terr[terra.Flat], n, "")
	under := 0
	for i := range g.Tiles {
		if level := g.SeaLevel(); level >= 0 && g.Height[i] <= level {
			under++
		}
	}
	s.SeaPct = 100 * float64(under) / float64(max(n, 1))
	sort.Float64s(heights)
	if len(heights) > 0 {
		s.HeightP50 = heights[len(heights)/2]
	}
	for _, k := range terra.Terrains() {
		s.Terrain = append(s.Terrain, shareOf(k.String(), terr[k], n, hex(terrainColor[k])))
	}
	land0 := max(1, n-terr[terra.Water]-terr[terra.Ice])
	for _, b := range terra.Bedrocks() {
		s.Bedrock = append(s.Bedrock, shareOf(b.String(), rock[b], land0, hex(rockColor[b])))
	}
	// Waterfalls: running water that leaves a hard bed for a softer one down a
	// real fall, which is where a river drops over the edge of the rock.
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if t.Terrain != terra.Water || g.Flow[i] <= riverFlow {
			continue
		}
		p := g.PosOf(i)
		low := -1
		for _, d := range terra.Dirs {
			q := g.Norm(geom.Pos{X: p.X + d.X, Y: p.Y + d.Y})
			if j := g.Index(q); g.In(q) && (low < 0 || g.Height[j] < g.Height[low]) {
				low = j
			}
		}
		if low >= 0 && g.Tiles[low].Hard() <= t.Hard()-softer && g.Height[i]-g.Height[low] >= waterfallDrop {
			s.Waterfalls++
		}
	}
	s.Plates = len(plates)
	s.Frozen = shareOf("permafrost", frozen, land0, "")
	s.Rivers = shareOf("flowing water", rivers, n, "")
	return s
}

// waterfallDrop is how many metres a river has to fall in one tile, off a
// hard bed onto a softer one, to be counted a waterfall.
const waterfallDrop = 2.0

// riverFlow is how much water, in cubic metres a second, a channel has to
// carry to be drawn as running water rather than standing: a fifth of a litre
// a second, a runnel and not a seep.
const riverFlow = 2e-4

func shareOf(name string, c, of int, col string) share {
	return share{Name: name, Count: c, Pct: 100 * float64(c) / float64(max(of, 1)), Color: col}
}

func (s summary) print() {
	fmt.Printf("%d tiles, %d plates, height %.0f..%.0f m (land median %.0f m), temperature %.1f..%.1f C now\n",
		s.Tiles, s.Plates, s.HeightMin, s.HeightMax, s.HeightP50, s.TempMin, s.TempMax)
	fmt.Println("\nground:")
	for _, x := range s.Terrain {
		fmt.Printf("  %-8s %6.1f%%  %s\n", x.Name, x.Pct, bar(x.Pct))
	}
	fmt.Println("\nbedrock (of dry land):")
	for _, x := range s.Bedrock {
		fmt.Printf("  %-9s %6.1f%%  %s\n", x.Name, x.Pct, bar(x.Pct))
	}
	fmt.Println("\nbiome:")
	for _, x := range s.Biomes {
		fmt.Printf("  %-20s %6.1f%%  %s\n", x.Name, x.Pct, bar(x.Pct))
	}
	fmt.Println("\nlandform:")
	for _, x := range s.Forms {
		fmt.Printf("  %-20s %6.1f%%  %s\n", x.Name, x.Pct, bar(x.Pct))
	}
	fmt.Printf("\npermafrost %.1f%% of land, flowing water %.1f%% of map, %d waterfalls\n", s.Frozen.Pct, s.Rivers.Pct, s.Waterfalls)
	fmt.Printf("rain on land %.0f mm a year, of which %.0f runs off; greatest river %.0f m3/s\n", s.LandRain, s.LandRunoff, s.FlowMax)
	fmt.Printf("moon %s; open coast springs %.2f m, neaps %.2f m; spring range on the coast %.1f m middling, %.1f m tenth highest, %.1f m most; flats %.1f%% of map\n",
		s.Moon, 2*(terra.TideM2+terra.TideS2), 2*(terra.TideM2-terra.TideS2), s.RangeP50, s.RangeP90, s.RangeMax, s.Flats.Pct)
}

func bar(pct float64) string {
	n := int(math.Round(pct / 2.5))
	out := make([]rune, n)
	for i := range out {
		out[i] = '#'
	}
	return string(out)
}

// A drawing is one layer: how each tile is coloured, and what it shows.
type drawing struct {
	file, title, about string
	legend             []share
	color              func(i int, p geom.Pos, t *terra.Tile) color.RGBA
	// overlay, where there is one, draws over the tiles once they are
	// coloured, px pixels to a tile.
	overlay func(img *image.RGBA, px int)
}

type layer struct {
	Title, File, About string
	Legend             []share
}

var terrainColor = [terra.TerrainCount]color.RGBA{
	terra.Grass:  {136, 176, 92, 255},
	terra.Forest: {46, 104, 58, 255},
	terra.Water:  {52, 104, 170, 255},
	terra.Field:  {214, 190, 110, 255},
	terra.Rock:   {150, 140, 130, 255},
	terra.Ice:    {226, 238, 246, 255},
	terra.Flat:   {158, 146, 118, 255},
}

var rockColor = [terra.BedrockCount]color.RGBA{
	terra.Granite:   {196, 150, 150, 255},
	terra.Limestone: {230, 222, 180, 255},
	terra.Sandstone: {214, 160, 96, 255},
	terra.Shale:     {110, 110, 130, 255},
	terra.Basalt:    {60, 60, 66, 255},
	terra.Schist:    {130, 170, 150, 255},
}

func drawings(land *terra.Land, s summary, cls classes) []drawing {
	g := land.Grid
	wet := color.RGBA{40, 70, 110, 255}
	span := math.Max(s.HeightMax-s.HeightMin, 1)
	flowLog := math.Log10(math.Max(s.FlowMax, 1e-9))

	mean := windOf(g, -1)

	shade := func(p geom.Pos) float64 {
		// light from the north-west, off the height difference across the tile
		a := g.HeightAt(g.Norm(geom.Pos{X: p.X - 1, Y: p.Y - 1}))
		b := g.HeightAt(g.Norm(geom.Pos{X: p.X + 1, Y: p.Y + 1}))
		return clamp(1+(a-b)/(span*0.15), 0.75, 1.2)
	}

	var terrLegend, rockLegend []share
	for _, x := range s.Terrain {
		terrLegend = append(terrLegend, x)
	}
	for _, x := range s.Bedrock {
		rockLegend = append(rockLegend, x)
	}

	return []drawing{
		{
			file: "terrain", title: "Terrain", legend: terrLegend,
			about: "What each tile is, shaded by the lie of the land, with the larger rivers picked out.",
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				c := terrainColor[t.Terrain]
				if t.Terrain == terra.Water && g.Flow[i] > riverFlow {
					c = color.RGBA{80, 150, 220, 255}
				}
				if t.Wet() {
					return c
				}
				return scaleRGB(c, shade(p))
			},
		},
		{
			file: "biome", title: "Biome", legend: s.Biomes,
			about: "What the weather makes of each tile, by Köppen–Geiger: the warmest and coldest month at its latitude and height, and the rain on it and when in the year it falls. Desert is under half the dry line and steppe under it; the tree line is a warmest month of ten degrees. Low ground by a river is wetland, standing woods are drawn a little darker, and the sea is shallow within reach of land.",
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				c := biomeOf(cls.Biome[i]).col
				if k := cls.Biome[i]; k == cShelf || k == cDeep {
					return cls.seaColor(i)
				}
				if t.Wet() {
					return c
				}
				if t.Terrain == terra.Forest {
					c = scaleRGB(c, 0.85)
				}
				return scaleRGB(c, shade(p))
			},
		},
		{
			file: "landform", title: "Landform", legend: s.Forms,
			about: "What the ground makes of each tile, read off the country round it: peaks stand above their neighbours and valleys below them, mountains and plateaus are the highest ground, rough or flat, hills the rough or steep ground below that, and a coast or a cliff is ground beside the sea.",
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				c := formOf(cls.Form[i]).col
				if k := cls.Form[i]; k == cShelf || k == cDeep {
					return cls.seaColor(i)
				}
				if t.Wet() {
					return c
				}
				return scaleRGB(c, shade(p))
			},
		},
		{
			file: "height", title: "Height",
			about: fmt.Sprintf("Metres, the dry ground from %.0f to %.0f, hillshaded. Water in blue, to %.0f.", s.HeightMin, s.HeightMax, s.FloorMin),
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				if t.Wet() {
					return lerpRGB(color.RGBA{20, 40, 90, 255}, color.RGBA{70, 130, 200, 255}, (g.Height[i]-s.FloorMin)/math.Max(s.HeightMax-s.FloorMin, 1))
				}
				return scaleRGB(ramp(elevation, (g.Height[i]-s.HeightMin)/span), shade(p))
			},
		},
		{
			file: "flow", title: "Drainage",
			about: fmt.Sprintf("Water through each tile on a log scale, up to %.0f m³/s: the rivers the land has had an age to find.", s.FlowMax),
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				if g.Flow[i] <= 0 {
					return color.RGBA{10, 14, 24, 255}
				}
				v := clamp((math.Log10(g.Flow[i])-(flowLog-4))/4, 0, 1)
				return ramp(water, v)
			},
		},
		{
			file: "rain", title: "Rain",
			about: "Rain in a year, on a log scale from 100 mm (pale) to 4 m (deep blue), hillshaded. Sea dark.",
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				v := clamp(math.Log10(math.Max(g.Rain(i), 1)/100)/math.Log10(40), 0, 1)
				c := ramp(rainfall, v)
				if t.Wet() && g.Runoff(i) == 0 {
					return scaleRGB(c, 0.45)
				}
				return scaleRGB(c, shade(p))
			},
		},
		{
			file: "wind", title: "Wind",
			about: fmt.Sprintf("The year's mean wind near the ground, hue for the way it blows - east red, north yellow-green, west cyan, south violet - and brightness for how hard, full at %.0f m/s. Worked out from the pressure the planet's belts and the warmth of the land and sea lay down, the turning of the planet, the drag of the ground and the ranges in the way.", speedMost),
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				c := windColor(mean.u[i], mean.v[i])
				if t.Wet() {
					return scaleRGB(c, 0.8)
				}
				return scaleRGB(c, shade(p))
			},
		},
		weatherDrawing(land, shade),
		streamDrawing("wind-mean", "Wind, the year", "The year's mean wind near the ground, as streamlines over its speed: pale where it is calm, deep where it blows hard.", g, mean, shade),
		streamDrawing("wind-midwinter", "Wind, midwinter", "The wind at the north's midwinter. A continent in its winter sits under a high and blows out to sea; in its summer it draws a low and the sea wind in.", g, windOf(g, 3*terra.Year/4), shade),
		streamDrawing("wind-midsummer", "Wind, midsummer", "The wind at the north's midsummer.", g, windOf(g, terra.Year/4), shade),
		{
			file: "tides", title: "Tides",
			about: fmt.Sprintf("Spring tidal range where the tide reaches, from nothing (pale) to %.1f m (deep violet); tidal flats in mud brown. No tide in grey.", 2*terra.TideFactorMax*(terra.TideM2+terra.TideS2)),
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				if t.Terrain == terra.Flat {
					return terrainColor[terra.Flat]
				}
				r := g.TidalRange(p)
				if r <= 0 {
					return scaleRGB(color.RGBA{128, 128, 128, 255}, shade(p))
				}
				c := ramp(tidal, r/(2*terra.TideFactorMax*(terra.TideM2+terra.TideS2)))
				if t.Wet() {
					return scaleRGB(c, 0.8)
				}
				return scaleRGB(c, shade(p))
			},
		},
		{
			file: "runoff", title: "Runoff",
			about: "What the rain leaves after the air takes its share back, in a year: 0 (sand) to 2 m (deep blue), on a log scale from 10 mm.",
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				r := g.Runoff(i)
				if r < 10 {
					return scaleRGB(color.RGBA{225, 205, 150, 255}, shade(p))
				}
				v := clamp(math.Log10(r/10)/math.Log10(200), 0, 1)
				return scaleRGB(ramp(rainfall, v), shade(p))
			},
		},
		{
			file: "bedrock", title: "Bedrock", legend: rockLegend,
			about: "The rock under the soil.",
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				if t.Wet() {
					return scaleRGB(rockColor[t.Bedrock], 0.45)
				}
				return scaleRGB(rockColor[t.Bedrock], shade(p))
			},
		},
		{
			file: "plates", title: "Plates",
			about: fmt.Sprintf("Which of the %d pieces of crust each tile rides. A world drawn rather than made from history has only one.", s.Plates),
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				c := hue(float64(t.Plate)*0.618034, 0.55, 0.85)
				if t.Wet() {
					c = scaleRGB(c, 0.5)
				}
				return c
			},
		},
		{
			file: "formed", title: "Rock age",
			about: fmt.Sprintf("The epoch each tile's rock dates from, 0 (dark, oldest) to %d (bright, youngest).", s.AgeMax),
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				c := ramp(magma, float64(t.Formed)/float64(max(s.AgeMax, 1)))
				if t.Wet() {
					c = scaleRGB(c, 0.5)
				}
				return c
			},
		},
		{
			file: "soil", title: "Soil",
			about: "How good the soil is (SoilAt), from poor (brown) to rich (green). Water dark.",
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				if t.Wet() {
					return wet
				}
				return ramp(soil, g.SoilAt(p))
			},
		},
		{
			file: "texture", title: "Soil texture",
			about: "Sand in red, clay in blue, silt in green.",
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				if t.Wet() {
					return color.RGBA{20, 20, 20, 255}
				}
				return color.RGBA{byte(255 * clamp(g.Sand[i], 0, 1)), byte(255 * clamp(g.Tile(i).Silt(), 0, 1)), byte(255 * clamp(g.Clay[i], 0, 1)), 255}
			},
		},
		{
			file: "temperature", title: "Temperature",
			about: fmt.Sprintf("Today's temperature, by latitude and height: %.1f to %.1f C. Frozen ground hatched white.", s.TempMin, s.TempMax),
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				c := ramp(thermal, (land.TempAt(p)-s.TempMin)/math.Max(s.TempMax-s.TempMin, 1e-9))
				if g.Frozen(p) && (p.X+p.Y)%3 == 0 {
					c = color.RGBA{250, 250, 255, 255}
				}
				if t.Wet() {
					c = scaleRGB(c, 0.55)
				}
				return c
			},
		},
		{
			file: "currents", title: "Sea warmth",
			about: "How much warmer (red) or colder (blue) than its latitude the currents make the sea, and the coast beside it, over the year: ±8 C at full colour.",
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				warm := g.CoastWarmth(i)
				if t.Wet() {
					warm = g.SeaWarmth(i)
				}
				k := clamp(math.Abs(warm)/8, 0, 1)
				c := lerpRGB(color.RGBA{240, 240, 236, 255}, color.RGBA{200, 40, 36, 255}, k)
				if warm < 0 {
					c = lerpRGB(color.RGBA{240, 240, 236, 255}, color.RGBA{36, 80, 200, 255}, k)
				}
				if !t.Wet() {
					c = scaleRGB(c, 0.8)
				}
				return c
			},
		},
		{
			file: "woods", title: "Woods",
			about: "How well each tile suits trees (WoodsAt), with the woods standing now outlined dark.",
			color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
				if t.Wet() {
					return wet
				}
				c := lerpRGB(color.RGBA{230, 225, 200, 255}, color.RGBA{60, 140, 70, 255}, g.WoodsAt(p))
				if t.Terrain == terra.Forest {
					c = scaleRGB(c, 0.55)
				}
				return c
			},
		},
	}
}

// streamDrawing is a map of the wind f as streamlines over its speed.
func streamDrawing(file, title, about string, g *terra.Grid, f *windField, shade func(geom.Pos) float64) drawing {
	return drawing{
		file: file, title: title, about: about,
		color: func(i int, p geom.Pos, t *terra.Tile) color.RGBA {
			c := ramp(breeze, math.Hypot(f.u[i], f.v[i])/speedMost)
			if t.Wet() {
				return scaleRGB(c, 0.88)
			}
			return scaleRGB(c, shade(p))
		},
		overlay: func(img *image.RGBA, px int) { streamlines(img, f, px) },
	}
}

func render(g *terra.Grid, px int, col func(int, geom.Pos, *terra.Tile) color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, g.W*px, g.H*px))
	for i := range g.Tiles {
		p := g.PosOf(i)
		c := col(i, p, &g.Tiles[i])
		for dy := 0; dy < px; dy++ {
			off := img.PixOffset(p.X*px, p.Y*px+dy)
			for dx := 0; dx < px; dx++ {
				copy(img.Pix[off+4*dx:], []byte{c.R, c.G, c.B, c.A})
			}
		}
	}
	return img
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Colour ramps, as evenly spaced stops.
var (
	elevation = []color.RGBA{{70, 120, 60, 255}, {150, 170, 90, 255}, {200, 180, 120, 255}, {150, 110, 80, 255}, {240, 240, 240, 255}}
	water     = []color.RGBA{{10, 14, 24, 255}, {30, 60, 110, 255}, {60, 140, 210, 255}, {200, 240, 255, 255}}
	magma     = []color.RGBA{{20, 10, 40, 255}, {110, 30, 100, 255}, {220, 80, 60, 255}, {250, 220, 120, 255}}
	soil      = []color.RGBA{{120, 80, 50, 255}, {190, 160, 90, 255}, {60, 150, 60, 255}}
	tidal     = []color.RGBA{{235, 235, 245, 255}, {150, 170, 220, 255}, {110, 80, 180, 255}, {60, 20, 90, 255}}
	rainfall  = []color.RGBA{{240, 232, 200, 255}, {170, 210, 170, 255}, {80, 160, 200, 255}, {20, 60, 150, 255}}
	breeze    = []color.RGBA{{236, 238, 232, 255}, {196, 222, 214, 255}, {150, 196, 214, 255}, {196, 160, 210, 255}}
	thermal   = []color.RGBA{{40, 60, 160, 255}, {100, 180, 220, 255}, {240, 230, 140, 255}, {220, 70, 40, 255}}
)

func ramp(stops []color.RGBA, v float64) color.RGBA {
	v = clamp(v, 0, 1) * float64(len(stops)-1)
	i := min(int(v), len(stops)-2)
	return lerpRGB(stops[i], stops[i+1], v-float64(i))
}

func lerpRGB(a, b color.RGBA, f float64) color.RGBA {
	f = clamp(f, 0, 1)
	l := func(x, y byte) byte { return byte(float64(x) + (float64(y)-float64(x))*f) }
	return color.RGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 255}
}

func scaleRGB(c color.RGBA, f float64) color.RGBA {
	s := func(x byte) byte { return byte(clamp(float64(x)*f, 0, 255)) }
	return color.RGBA{s(c.R), s(c.G), s(c.B), 255}
}

func hue(h, s, v float64) color.RGBA {
	h = (h - math.Floor(h)) * 6
	i := int(h)
	f := h - float64(i)
	p, q, t := v*(1-s), v*(1-s*f), v*(1-s*(1-f))
	var r, g, b float64
	switch i {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	default:
		r, g, b = v, p, q
	}
	return color.RGBA{byte(r * 255), byte(g * 255), byte(b * 255), 255}
}

func hex(c color.RGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

var pageTmpl = template.Must(template.New("page").Funcs(template.FuncMap{
	"pct": func(v float64) string { return fmt.Sprintf("%.1f%%", v) },
	"m":   func(v float64) string { return fmt.Sprintf("%.0f", v) },
	"c":   func(v float64) string { return fmt.Sprintf("%.1f", v) },
	"css": func(s string) template.CSS { return template.CSS(s) },
}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<title>terra · seed {{.Seed}}</title>
<style>
:root{--bg:#f6f5f1;--fg:#1d1d1b;--mut:#6b6a64;--card:#fff;--line:#e2e0d8}
@media (prefers-color-scheme:dark){:root{--bg:#141412;--fg:#ecebe6;--mut:#9a9890;--card:#1e1e1b;--line:#2e2e2a}}
body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.45 system-ui,sans-serif}
main{max-width:1200px;margin:0 auto;padding:28px 20px 60px}
h1{font-size:22px;margin:0 0 4px} .mut{color:var(--mut)}
.stats{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:10px;margin:20px 0}
.stat{background:var(--card);border:1px solid var(--line);border-radius:8px;padding:10px 12px}
.stat b{display:block;font-size:18px;font-variant-numeric:tabular-nums}
.tabs{display:flex;flex-wrap:wrap;gap:6px;margin:18px 0 12px}
.tabs button{font:inherit;border:1px solid var(--line);background:var(--card);color:var(--fg);border-radius:6px;padding:5px 10px;cursor:pointer}
.tabs button[aria-pressed=true]{background:var(--fg);color:var(--bg)}
figure{margin:0;background:var(--card);border:1px solid var(--line);border-radius:8px;padding:12px}
.map{overflow:auto;max-height:78vh;cursor:grab;overscroll-behavior:contain} .map.drag{cursor:grabbing}
.map img{display:block;image-rendering:pixelated;max-width:none;user-select:none;-webkit-user-drag:none}
.zoom{display:flex;align-items:center;gap:6px;margin:0 0 8px}
.zoom button{font:inherit;border:1px solid var(--line);background:var(--card);color:var(--fg);border-radius:6px;padding:3px 10px;cursor:pointer;min-width:34px}
.zoom output{font-variant-numeric:tabular-nums;min-width:52px;text-align:center}
figcaption{margin-top:8px}
.legend{display:flex;flex-wrap:wrap;gap:4px 14px;margin-top:6px}
.legend span{display:inline-flex;align-items:center;gap:5px;font-variant-numeric:tabular-nums}
.legend i{width:11px;height:11px;border-radius:2px;display:inline-block}
.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(260px,1fr));gap:10px;margin-top:22px}
.grid figure{padding:8px;cursor:pointer} .grid img{width:100%;image-rendering:pixelated;display:block}
.grid figcaption{margin-top:4px;font-size:13px}
.map{position:relative}
.pick{position:absolute;pointer-events:none;box-sizing:border-box;border:2px solid #fff;outline:2px solid #000;border-radius:2px}
#tile{background:var(--card);border:1px solid var(--line);border-radius:8px;padding:12px 14px;margin-top:10px}
#tile h2{font-size:15px;margin:0 0 2px} #tile h3{font-size:13px;margin:12px 0 2px;text-transform:capitalize}
#tile p{margin:2px 0}
</style></head><body><main data-w="{{.W}}" data-h="{{.H}}">
<h1>A world from seed {{.Seed}}</h1>
<div class="mut">{{.Terms.Width}}×{{.Terms.Height}} tiles · preset {{.Preset}} · {{.Terms.Epochs}} epochs · {{if and (gt .Terms.Epochs 0) (gt .Terms.Water 0.0)}}water {{.Terms.Water}} m, {{pct .Stats.SeaPct}} sea{{else}}sea {{.Terms.SeaShare}}{{end}} · {{if .Terms.Wrap}}globe{{else}}valley{{end}} · made in {{.Took}}</div>
<div class="stats">
 <div class="stat"><span class="mut">Tiles</span><b>{{.Stats.Tiles}}</b></div>
 <div class="stat"><span class="mut">Plates</span><b>{{.Stats.Plates}}</b></div>
 <div class="stat"><span class="mut">Height (m)</span><b>{{m .Stats.HeightMin}}–{{m .Stats.HeightMax}}</b></div>
 <div class="stat"><span class="mut">Land median (m)</span><b>{{m .Stats.HeightP50}}</b></div>
 <div class="stat"><span class="mut">Temperature (°C)</span><b>{{c .Stats.TempMin}} – {{c .Stats.TempMax}}</b></div>
 <div class="stat"><span class="mut">Frozen land</span><b>{{pct .Stats.Frozen.Pct}}</b></div>
 <div class="stat"><span class="mut">Flowing water</span><b>{{pct .Stats.Rivers.Pct}}</b></div>
 <div class="stat"><span class="mut">Rain / runoff on land (mm)</span><b>{{m .Stats.LandRain}} / {{m .Stats.LandRunoff}}</b></div>
 <div class="stat"><span class="mut">Greatest river (m³/s)</span><b>{{m .Stats.FlowMax}}</b></div>
 <div class="stat"><span class="mut">Moon on day one</span><b>{{.Stats.Moon}}</b></div>
 <div class="stat"><span class="mut">Coastal spring range (m)</span><b>{{c .Stats.RangeP50}} / {{c .Stats.RangeMax}}</b></div>
 <div class="stat"><span class="mut">Tidal flats</span><b>{{pct .Stats.Flats.Pct}}</b></div>
</div>
<div class="tabs" id="tabs">{{range $i, $l := .Layers}}<button data-i="{{$i}}" aria-pressed="{{if eq $i 0}}true{{else}}false{{end}}">{{$l.Title}}</button>{{end}}</div>
<div class="zoom"><button id="zout" title="Zoom out (-)">−</button><output id="zlevel">100%</output><button id="zin" title="Zoom in (+)">+</button><button id="zfit" title="Fit to width (0)">Fit</button><button id="zone" title="Actual size (1)">1:1</button>
 <span class="mut">Scroll to zoom, drag to pan, click a tile to ask why it is so.</span></div>
{{range $i, $l := .Layers}}<figure class="big" data-i="{{$i}}"{{if ne $i 0}} hidden{{end}}>
 <div class="map"><img src="{{$l.File}}" width="{{$.Width}}" alt="{{$l.Title}}"><div class="pick" hidden></div></div>
 <figcaption><b>{{$l.Title}}</b> <span class="mut">{{$l.About}}</span>
 {{if $l.Legend}}<div class="legend">{{range $l.Legend}}<span><i style="background:{{css .Color}}"></i>{{.Name}} <span class="mut">{{pct .Pct}}</span></span>{{end}}</div>{{end}}
 </figcaption></figure>{{end}}
<section id="tile" aria-live="polite" hidden></section>
<div class="grid">{{range $i, $l := .Layers}}<figure data-i="{{$i}}"><img src="{{$l.File}}" alt="{{$l.Title}}"><figcaption>{{$l.Title}}</figcaption></figure>{{end}}</div>
</main>
<script>
// Every layer is the same size, so they share one zoom and one view: switching
// layers keeps the same spot in front of you.
const bigs=[...document.querySelectorAll('figure.big')];
const base=+bigs[0].querySelector('img').getAttribute('width');
let zoom=1;
const active=()=>bigs.find(f=>!f.hidden).querySelector('.map');
function setZoom(z,cx,cy){
 const m=active();
 if(cx===undefined){cx=m.clientWidth/2;cy=m.clientHeight/2}
 z=Math.min(64,Math.max(0.1,z));
 const x=(m.scrollLeft+cx)/zoom, y=(m.scrollTop+cy)/zoom;
 zoom=z;
 bigs.forEach(f=>f.querySelector('img').style.width=(base*zoom)+'px');
 m.scrollLeft=x*zoom-cx; m.scrollTop=y*zoom-cy;
 document.getElementById('zlevel').value=Math.round(zoom*100)+'%';
 placePick();
}
// A click on a tile asks the server why it is so; see serve.go. The page
// written by the command line alone has no server to ask, and says so.
const W=+document.querySelector('main').dataset.w;
let picked=null;
function placePick(){
 const s=base*zoom/W, side=Math.max(s,8);
 bigs.forEach(f=>{
  const d=f.querySelector('.pick');
  d.hidden=!picked; if(!picked)return;
  d.style.width=d.style.height=side+'px';
  d.style.left=((picked.x+.5)*s-side/2)+'px'; d.style.top=((picked.y+.5)*s-side/2)+'px';
 });
}
function el(tag,text,cls){const e=document.createElement(tag);e.textContent=text;if(cls)e.className=cls;return e}
async function pick(x,y){
 picked={x,y}; placePick();
 const box=document.getElementById('tile');
 box.hidden=false; box.replaceChildren(el('h2','Tile ('+x+', '+y+')'),el('p','Asking…','mut'));
 let a;
 try{
  if(location.protocol==='file:')throw new Error('Tiles answer only when the page is served: go run ./cmd/overview -serve :8080');
  const r=await fetch('tile?x='+x+'&y='+y);
  a=await r.json();
  if(!r.ok)throw new Error(a.error||r.statusText);
 }catch(err){
  if(picked.x!==x||picked.y!==y)return;
  box.replaceChildren(el('h2','Tile ('+x+', '+y+')'),el('p',err.message,'mut'));
  return;
 }
 if(picked.x!==x||picked.y!==y)return;
 const head=[el('h2','Tile ('+x+', '+y+'): '+a.terrain+', '+a.height)];
 if(a.features&&a.features.length)head.push(el('p','Part of '+a.features.join(', '),'mut'));
 box.replaceChildren(...head);
 for(const as of a.aspects){
  box.append(el('h3',as.name));
  for(const s of as.sentences||[])box.append(el('p',s));
 }
}
function show(i){
 // Read before hiding: a hidden element has no scroll position.
 const was=active(), l=was.scrollLeft, t=was.scrollTop;
 bigs.forEach(f=>f.hidden=f.dataset.i!=i);
 const m=active(); m.scrollLeft=l; m.scrollTop=t;
 document.querySelectorAll('#tabs button').forEach(b=>b.setAttribute('aria-pressed',b.dataset.i==i));
}
const fit=()=>setZoom(active().clientWidth/base);
document.getElementById('zin').onclick=()=>setZoom(zoom*1.5);
document.getElementById('zout').onclick=()=>setZoom(zoom/1.5);
document.getElementById('zfit').onclick=fit;
document.getElementById('zone').onclick=()=>setZoom(1);
bigs.forEach(f=>{
 const m=f.querySelector('.map');
 m.addEventListener('wheel',e=>{
  e.preventDefault();
  const r=m.getBoundingClientRect();
  setZoom(zoom*Math.pow(1.0015,-e.deltaY),e.clientX-r.left,e.clientY-r.top);
 },{passive:false});
 let drag=null;
 m.addEventListener('pointerdown',e=>{drag={x:e.clientX,y:e.clientY,l:m.scrollLeft,t:m.scrollTop};m.setPointerCapture(e.pointerId);m.classList.add('drag')});
 m.addEventListener('pointermove',e=>{if(drag){m.scrollLeft=drag.l-(e.clientX-drag.x);m.scrollTop=drag.t-(e.clientY-drag.y)}});
 const end=()=>{drag=null;m.classList.remove('drag')};
 // A press that barely moved is a click, not a pan.
 m.addEventListener('pointerup',e=>{
  if(drag&&Math.hypot(e.clientX-drag.x,e.clientY-drag.y)<4){
   const r=f.querySelector('img').getBoundingClientRect(), s=r.width/W;
   const x=Math.floor((e.clientX-r.left)/s), y=Math.floor((e.clientY-r.top)/s);
   if(x>=0&&y>=0&&x<W&&y<+document.querySelector('main').dataset.h)pick(x,y);
  }
  end();
 });
 m.addEventListener('pointercancel',end);
});
document.querySelectorAll('#tabs button, .grid figure').forEach(e=>e.onclick=()=>{show(e.dataset.i);window.scrollTo({top:0,behavior:'smooth'})});
document.addEventListener('keydown',e=>{
 const n=document.querySelectorAll('#tabs button').length;
 const cur=+document.querySelector('#tabs button[aria-pressed=true]').dataset.i;
 if(e.key==='ArrowRight')show((cur+1)%n); if(e.key==='ArrowLeft')show((cur+n-1)%n);
 if(e.key==='+'||e.key==='=')setZoom(zoom*1.5); if(e.key==='-')setZoom(zoom/1.5);
 if(e.key==='0')fit(); if(e.key==='1')setZoom(1);
});
</script></body></html>
`))
