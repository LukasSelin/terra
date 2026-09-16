package main

import (
	"fmt"
	"html/template"
	"math"
	"os"
	"strings"

	"github.com/LukasSelin/terra"
	"github.com/LukasSelin/terra/geom"
)

// The why page: eight tiles of the world, and the world's account of each.
//
// terra.Why hands back a chain of causes with numbers and units; this is a
// renderer over it, one sentence a cause, and nothing here knows anything
// the chain does not say. The tiles are chosen from the world itself, the
// same way every time: the highest ground, the driest land, the shore of
// the largest lake, the mouth of the largest river, and four spread evenly
// across the middle of the map.

// whyPlace is one tile on the page.
type whyPlace struct {
	Title    string
	Pos      geom.Pos
	Terrain  string
	Height   string
	Features []string
	Aspects  []whyAspect
}

type whyAspect struct {
	Name      string
	Sentences []string
}

// whyPlaces picks the eight tiles. Where a world has no lake, the shore is
// the lowest dry land instead, and says so.
func whyPlaces(g *terra.Grid) []whyPlace {
	n := len(g.Tiles)
	dry := func(i int) bool { return !g.Tiles[i].Wet() && g.Tiles[i].Terrain != terra.Flat }
	var places []whyPlace
	add := func(title string, i int) {
		if i < 0 {
			return
		}
		places = append(places, whyPlace{Title: title, Pos: g.PosOf(i)})
	}

	top, driest, lowest := -1, -1, -1
	for i := 0; i < n; i++ {
		if top < 0 || g.Height[i] > g.Height[top] {
			top = i
		}
		if !dry(i) {
			continue
		}
		if driest < 0 || g.Rain(i) < g.Rain(driest) {
			driest = i
		}
		if lowest < 0 || g.Height[i] < g.Height[lowest] {
			lowest = i
		}
	}
	add("The highest ground", top)
	add("The driest land", driest)

	// The largest lake's shore: the lowest-numbered dry tile beside it.
	var lake, river *terra.Feature
	if f := g.Features(); f != nil {
		for k := range f.All {
			fe := &f.All[k]
			switch fe.Kind {
			case terra.StandingLake:
				if lake == nil || fe.Count > lake.Count {
					lake = fe
				}
			case terra.DrainageBasin:
				if river == nil || fe.Flow > river.Flow {
					river = fe
				}
			}
		}
	}
	shore := -1
	if lake != nil {
		for _, t := range lake.Tiles {
			p := g.PosOf(int(t))
			for _, d := range terra.Dirs {
				q := g.Norm(geom.Pos{X: p.X + d.X, Y: p.Y + d.Y})
				if g.In(q) {
					if j := g.Index(q); dry(j) && (shore < 0 || j < shore) {
						shore = j
					}
				}
			}
		}
	}
	if shore >= 0 {
		add("The shore of the largest lake", shore)
	} else {
		add("The lowest dry land (the world has no lake)", lowest)
	}
	// The largest river's mouth: its last tile of land before the outlet.
	if river != nil {
		mouth := int(river.Outlet)
		for k := len(river.Trunk) - 1; k >= 0; k-- {
			if t := int(river.Trunk[k]); !isSea(g, t) {
				mouth = t
				break
			}
		}
		add("The mouth of the largest river", mouth)
	}
	// Four spread evenly across the middle row, each moved to the nearest
	// dry land.
	for k := 0; k < 4; k++ {
		p := geom.Pos{X: g.W * (2*k + 1) / 8, Y: g.H / 2}
		q, ok := g.Nearest(p, max(g.W, g.H), func(_ geom.Pos, t *terra.Tile) bool { return !t.Wet() && t.Terrain != terra.Flat })
		if !ok {
			continue
		}
		add(fmt.Sprintf("Land %d of 4 across the middle", k+1), g.Index(q))
	}
	for k := range places {
		pl := &places[k]
		i := g.Index(pl.Pos)
		pl.Terrain = terrainName(g.Tiles[i].Terrain)
		pl.Height = fmt.Sprintf("%.0f m", g.Height[i])
		for _, id := range g.FeaturesAt(pl.Pos) {
			pl.Features = append(pl.Features, featureName(g, id))
		}
		for a := terra.OfHeight; a <= terra.OfCover; a++ {
			pl.Aspects = append(pl.Aspects, whyAspect{Name: a.String(), Sentences: sentences(g, g.Why(pl.Pos, a))})
		}
	}
	return places
}

// isSea is whether tile i is under the sea: water with no lake over it.
func isSea(g *terra.Grid, i int) bool {
	if !g.Tiles[i].Wet() {
		return false
	}
	_, lake := g.LakeAt(g.PosOf(i))
	return !lake
}

func terrainName(t terra.Terrain) string {
	return [terra.TerrainCount]string{"open ground", "forest", "water", "field", "outcrop", "ice", "tidal flat", "salt lake", "salt pan"}[t]
}

// featureName is what a feature is called on the page: its name, or its
// kind and id.
func featureName(g *terra.Grid, id terra.FeatureID) string {
	f := g.Feature(id)
	if f == nil {
		return "nothing"
	}
	if f.Name != "" {
		return f.Name
	}
	return fmt.Sprintf("%s %d", f.Kind, f.ID)
}

// sentences renders a chain, one sentence a cause. A cause the renderer has
// no sentence for is given as its kind and its number, so nothing recorded
// is lost.
func sentences(g *terra.Grid, chain []terra.Cause) []string {
	var out []string
	for _, c := range chain {
		fe := func() string { return featureName(g, c.Feature) }
		q := num(c.Quantity)
		var s string
		switch c.Kind {
		case terra.RaisedBy:
			verb := "raised"
			if c.Quantity < 0 {
				verb = "dropped"
			}
			s = fmt.Sprintf("%s %s this ground by %s %s of the history's own, %s, in a %s", capital(fe()), verb, num(math.Abs(c.Quantity)), c.Unit, ago(c.When), c.Note)
		case terra.BetweenPlate:
			s = fmt.Sprintf("one of the plates was %s (number %s as the history wrote it)", fe(), q)
		case terra.WornSince:
			s = fmt.Sprintf("the history's weather has taken %s %s of that off since", q, c.Unit)
		case terra.Stands:
			s = fmt.Sprintf("it stands at %s %s on the map", q, c.Unit)
			if c.Feature != 0 {
				s += ", riding " + fe()
			}
			if c.Note != "" {
				s += "; " + c.Note
			}
		case terra.BedOf:
			s = fmt.Sprintf("the surface lies in a bed of %s", c.Note)
			if c.Quantity > 0 {
				s += fmt.Sprintf(" %s %s thick", q, c.Unit)
			}
			if c.When > 0 {
				s += ", laid " + ago(c.When)
			}
		case terra.LaidAsFill:
			s = "it is what a river filled a basin with, coarse or fine as the water sorted it"
		case terra.LaidAsMud:
			s = "it is mud that settled off a shore"
		case terra.LaidAsLime:
			s = "it is the lime of a quiet sea"
		case terra.LaidAsLava:
			s = "it is lava that flooded the ground"
		case terra.Cooked:
			s = "it was cooked and squeezed under a collision"
		case terra.MeltedAtDepth:
			s = "it melted under an arc and cooled at depth, and the weather has since bared it"
		case terra.OceanFloor:
			s = "it is the floor the crust itself was"
		case terra.BuriedLast:
			s = fmt.Sprintf("the last thing the history laid over this tile was %s, %s", c.Note, ago(c.When))
		case terra.RainOf:
			s = fmt.Sprintf("%s %s of rain falls here in a year", q, c.Unit)
			if c.Feature != 0 {
				s += ", in " + fe()
			}
		case terra.LatitudeRain:
			s = fmt.Sprintf("the mean over its row of the map is %s %s", q, c.Unit)
		case terra.Orographic:
			if c.Quantity > 0 {
				s = fmt.Sprintf("in %s, which carries most of the air's water here, the ground's lift wrung %s %s a year out of the air over this cell", c.Note, q, c.Unit)
			} else {
				s = fmt.Sprintf("in %s, which carries most of the air's water here, the ground's lift wrung nothing out of the air over this cell", c.Note)
			}
		case terra.UpwindSea:
			if c.Quantity == 0 {
				s = "the cell is itself mostly sea"
			} else {
				s = fmt.Sprintf("the sea lies %s %s upwind in that quarter's wind", q, c.Unit)
			}
		case terra.Suits:
			s = fmt.Sprintf("the ground suits trees at %s of one", q)
			if c.Feature != 0 {
				s += ", in " + fe()
			}
		case terra.Slope:
			s = fmt.Sprintf("its slope is %s (%s)", q, c.Note)
		case terra.Treeless:
			s = "it stands above the tree line"
		case terra.Barren:
			s = "it lies under ice"
		case terra.Warmth:
			s = fmt.Sprintf("the year's mean here is %s %s", q, c.Unit)
		case terra.Evaporation:
			s = fmt.Sprintf("the air could take up %s %s a year", q, c.Unit)
		case terra.Wetness:
			s = fmt.Sprintf("its topographic wetness is %s of the map's mean", q)
		case terra.WaterRatio:
			s = fmt.Sprintf("so it has %s of the water a forest needs (%s)", q, c.Note)
		case terra.Drains:
			s = fmt.Sprintf("it stands %s %s over the water it drains into (%s)", q, c.Unit, c.Note)
		case terra.SoilDepth:
			s = fmt.Sprintf("there is %s %s of soil over its rock", q, c.Unit)
		default:
			s = fmt.Sprintf("%s: %s %s %s", c.Kind, q, c.Unit, c.Note)
		}
		out = append(out, capital(strings.TrimSpace(s))+".")
	}
	return out
}

// ago is years before the present as the page says it.
func ago(years float64) string {
	switch {
	case years >= 1e6:
		return fmt.Sprintf("%s million years ago", num(years/1e6))
	case years >= 1e3:
		return fmt.Sprintf("%s thousand years ago", num(years/1e3))
	}
	return fmt.Sprintf("%s years ago", num(years))
}

// num is a quantity with as many decimals as it needs and no more.
func num(v float64) string {
	switch {
	case math.IsInf(v, 0) || math.IsNaN(v):
		return "?"
	case math.Abs(v) >= 100 || v == math.Trunc(v):
		return fmt.Sprintf("%.0f", v)
	case math.Abs(v) >= 1:
		return fmt.Sprintf("%.1f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func capital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

var whyTmpl = template.Must(template.New("why").Parse(`<!doctype html>
<meta charset="utf-8">
<title>Why - seed {{.Seed}}, {{.Preset}}</title>
<style>
body{font:15px/1.5 system-ui,sans-serif;max-width:52em;margin:2em auto;padding:0 1em;color:#222}
h1{font-size:1.4em}h2{font-size:1.15em;margin-top:2em}h3{font-size:1em;margin:1em 0 .2em}
.meta{color:#666}p{margin:.2em 0}
</style>
<h1>Why: seed {{.Seed}}, the {{.Preset}} preset</h1>
<p class="meta">Eight tiles, and the world's account of each: the chain terra.Why hands back, rendered one sentence a cause. The metres of a meeting are the history's own, a planet's, and not the map's; the map's heights are handed over by rank.</p>
{{range .Places}}
<h2>{{.Title}}</h2>
<p class="meta">Tile ({{.Pos.X}}, {{.Pos.Y}}): {{.Terrain}}, {{.Height}}{{if .Features}}; part of {{range $i, $f := .Features}}{{if $i}}, {{end}}{{$f}}{{end}}{{end}}.</p>
{{range .Aspects}}<h3>{{.Name}}</h3>
{{range .Sentences}}<p>{{.}}</p>
{{end}}{{end}}{{end}}
`))

// writeWhy writes the why page for land.
func writeWhy(path string, land *terra.Land, seed uint64, preset string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return whyTmpl.Execute(f, struct {
		Seed   uint64
		Preset string
		Places []whyPlace
	}{seed, preset, whyPlaces(land.Grid)})
}
