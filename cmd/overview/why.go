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

// The why page: some tiles of the world, and the world's account of each.
//
// terra.Why hands back a chain of causes with numbers and units; this is a
// renderer over it, one sentence a cause, and nothing here knows anything
// the chain does not say. The tiles are chosen from the world itself, the
// same way every time: the highest ground, the driest land, the shore of
// the largest lake, the mouth of the largest river, on a globe the
// strongest warm western current, the desert beside the strongest upwelling
// and the coast a current warms the most, and four spread evenly across the
// middle of the map. Each tile's features are listed with their relations
// (terra.Features.RelationsOf), one sentence a relation.

// whyPlace is one tile on the page.
// It is also what the server answers a click on a tile with, as JSON.
type whyPlace struct {
	Title    string   `json:"title,omitempty"`
	Pos      geom.Pos `json:"pos"`
	Terrain  string   `json:"terrain"`
	Height   string   `json:"height"`
	Features []string `json:"features"`
	// Relations is what each of those features does to others and has
	// done to it, one sentence a relation: see relationSentences.
	Relations []string    `json:"relations,omitempty"`
	Aspects   []whyAspect `json:"aspects"`
}

type whyAspect struct {
	Name      string   `json:"name"`
	Sentences []string `json:"sentences"`
}

// whyPlaces picks the tiles. Where a world has no lake, the shore is the
// lowest dry land instead, and says so.
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
	var lake, river, current *terra.Feature
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
			case terra.SeaCurrent:
				if fe.Class == terra.WesternBoundary && fe.Warmth > 0 && (current == nil || fe.Transport > current.Transport) {
					current = fe
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
	// The middle of the warm western current that carries the most water.
	if current != nil && len(current.Path) > 0 {
		add("The strongest warm western current", int(current.Path[len(current.Path)/2]))
	}
	// The desert the strongest upwelling dries, and the coast a current warms
	// the most, each at its tile nearest the water that does it.
	if f := g.Features(); f != nil {
		var up *terra.Feature
		for k := range f.All {
			if fe := &f.All[k]; fe.Kind == terra.Upwelling && (up == nil || fe.Flow > up.Flow) {
				up = fe
			}
		}
		if up != nil {
			var desert *terra.Feature
			for _, r := range f.RelationsOf(up.ID) {
				if to := g.Feature(r.To); r.Kind == terra.Dries && r.From == up.ID && to.Group == 'B' && (desert == nil || to.Count > desert.Count) {
					desert = to
				}
			}
			if desert != nil {
				add("The desert beside the strongest upwelling", nearestOf(g, desert.Tiles, int(up.First)))
			}
		}
		var warms *terra.Relation
		for k, r := range f.Relations() {
			if r.Kind == terra.Warms && (warms == nil || r.Quantity > warms.Quantity) {
				warms = &f.Relations()[k]
			}
		}
		if warms != nil {
			cur, coast := g.Feature(warms.From), g.Feature(warms.To)
			if len(cur.Path) > 0 {
				add("The coast a current warms the most", nearestOf(g, coast.Tiles, int(cur.Path[len(cur.Path)/2])))
			}
		}
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
		title := places[k].Title
		places[k] = describe(g, places[k].Pos)
		places[k].Title = title
	}
	return places
}

// describe is the world's account of the tile at p, which must be on the
// map: what it is, what it is part of, and why, aspect by aspect.
func describe(g *terra.Grid, p geom.Pos) whyPlace {
	i := g.Index(p)
	pl := whyPlace{Pos: p, Terrain: terrainName(g.Tiles[i].Terrain), Height: fmt.Sprintf("%.0f m", g.Height[i])}
	for _, id := range g.FeaturesAt(p) {
		pl.Features = append(pl.Features, featureName(g, id))
		pl.Relations = append(pl.Relations, relationSentences(g, id)...)
	}
	for a := terra.OfHeight; a <= terra.OfWarmth; a++ {
		pl.Aspects = append(pl.Aspects, whyAspect{Name: a.String(), Sentences: sentences(g, g.Why(p, a))})
	}
	return pl
}

// nearestOf is the tile of tiles nearest tile to, in tiles across the map
// and round its seam, the first on a tie; or -1 for none.
func nearestOf(g *terra.Grid, tiles []int32, to int) int {
	best, most := -1, 0
	tx, ty := to%g.W, to/g.W
	for _, t := range tiles {
		dx, dy := int(t)%g.W-tx, int(t)/g.W-ty
		if dx < 0 {
			dx = -dx
		}
		if g.Wrap && g.W-dx < dx {
			dx = g.W - dx
		}
		if d := dx*dx + dy*dy; best < 0 || d < most {
			best, most = int(t), d
		}
	}
	return best
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

// relationsShown is the most relations of one feature a tile lists: a
// current can water a hundred small basins along a coast.
const relationsShown = 8

// relationSentences is the relations of feature id, one sentence each, the
// most first of each kind as the registry orders them, and how many more
// there are past relationsShown.
func relationSentences(g *terra.Grid, id terra.FeatureID) []string {
	rels := g.Features().RelationsOf(id)
	var out []string
	for k, r := range rels {
		if k == relationsShown {
			out = append(out, fmt.Sprintf("And %d more of %s.", len(rels)-k, featureName(g, id)))
			break
		}
		out = append(out, capital(relationSentence(g, r))+".")
	}
	return out
}

// relationSentence is one relation as the page says it.
func relationSentence(g *terra.Grid, r terra.Relation) string {
	from, to := featureName(g, r.From), featureName(g, r.To)
	switch r.Kind {
	case terra.Warms:
		return fmt.Sprintf("%s warms %s by %s %s", from, to, num(r.Quantity), r.Unit)
	case terra.Cools:
		return fmt.Sprintf("%s cools %s by %s %s", from, to, num(-r.Quantity), r.Unit)
	case terra.Dries:
		return fmt.Sprintf("%s dries %s: the air held down over its cold water keeps back %s of the rain", from, to, percent(r.Quantity))
	case terra.Waters:
		return fmt.Sprintf("%s waters %s: %s of its rain was taken up off it", from, to, percent(r.Quantity))
	case terra.PartOf:
		return fmt.Sprintf("%s runs in %s", from, to)
	case terra.Feeds:
		return fmt.Sprintf("%s feeds %s, up to %s %s", from, to, num(r.Quantity), r.Unit)
	}
	return fmt.Sprintf("%s %s %s: %s %s", from, r.Kind, to, num(r.Quantity), r.Unit)
}

// percent is a share as the page says it.
func percent(v float64) string { return fmt.Sprintf("%.0f%%", 100*v) }

// warmer is "warmer" or "colder" for a number of degrees, and the degrees
// without their sign.
func warmer(v float64) (string, string) {
	if v < 0 {
		return "colder", num(-v)
	}
	return "warmer", num(v)
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
			if c.Feature != 0 {
				s += ", in " + fe()
			}
		case terra.OffshoreCurrent:
			w, d := warmer(c.Quantity)
			s = fmt.Sprintf("the sea the air came off in %s is %s, whose water stands %s %s %s than its latitude's mean", c.Note, fe(), d, c.Unit, w)
		case terra.SeaDamp:
			s = fmt.Sprintf("so the sea there gives the air %s times the water it would at its latitude's warmth", q)
		case terra.Inversion:
			s = fmt.Sprintf("the air held down over the cold water off the coast keeps back %s of the rain over this cell", percent(c.Quantity))
			if c.Feature != 0 {
				s += ", and the water that chills it most is " + fe() + "'s"
			}
		case terra.Latitude:
			ns := "north"
			if c.Quantity < 0 {
				ns = "south"
			}
			s = fmt.Sprintf("it lies %s degrees %s of the equator", num(math.Abs(c.Quantity)), ns)
		case terra.LatitudeWarmth:
			s = fmt.Sprintf("the year's mean at sea level on that latitude is %s %s", q, c.Unit)
		case terra.SeaAbout:
			w, d := warmer(c.Quantity)
			s = fmt.Sprintf("the sea about it, against the sea about its row, makes it %s %s %s", d, c.Unit, w)
		case terra.CoastWarmth:
			w, d := warmer(c.Quantity)
			s = fmt.Sprintf("the currents offshore make it %s %s %s", d, c.Unit, w)
			if c.Feature != 0 {
				s += ", " + fe() + " the most"
			}
		case terra.OffCoast:
			w, d := warmer(c.Quantity)
			s = fmt.Sprintf("%s makes it %s %s %s", fe(), d, c.Unit, w)
		case terra.Altitude:
			w, d := warmer(c.Quantity)
			s = fmt.Sprintf("its height makes it %s %s %s", d, c.Unit, w)
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
<p class="meta">Some tiles, and the world's account of each: the chain terra.Why hands back, rendered one sentence a cause, and what the features the tile is part of do to one another. The metres of a meeting are the history's own, a planet's, and not the map's; the map's heights are handed over by rank.</p>
{{range .Places}}
<h2>{{.Title}}</h2>
<p class="meta">Tile ({{.Pos.X}}, {{.Pos.Y}}): {{.Terrain}}, {{.Height}}{{if .Features}}; part of {{range $i, $f := .Features}}{{if $i}}, {{end}}{{$f}}{{end}}{{end}}.</p>
{{if .Relations}}<details><summary class="meta">What those features do</summary>
{{range .Relations}}<p>{{.}}</p>
{{end}}</details>{{end}}
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
