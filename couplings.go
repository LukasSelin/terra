package terra

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// The couplings: which pass reads which of the world's fields and writes
// which.
//
// A world is a handful of big systems that drive one another - the sun and
// the air, the sea, the rock and its history, the water on the land, the
// soil and what grows on it - and each pass of Generate is where one system
// reads others and writes itself or another. This is the declaration of
// that: every pass, the stages it runs in, the fields it reads and the
// fields it writes, and every field, the system it belongs to and the
// fields of the Grid that hold it. The graph the declaration makes, field
// to field through a pass, is the world's coupling graph, and a cycle in it
// is a feedback loop: the rain wears the ground, the ground lifts the air,
// the air rains.
//
// The declaration is held to the code by couplings_test.go, two ways: the
// passes' own source is read for the fields each touches, and a world is
// made stage by stage and every field a stage changes has to be one a pass
// of that stage declares it writes. docs/couplings.md is generated from it
// (TERRA_COUPLINGS=write), and cmd/overview draws it.
//
// A pass or a loop that is not here yet - the climate of each epoch, the
// carbon thermostat, albedo from the ice - joins the graph with one line in
// couplings below, and a field it brings with one line in worldFields.

// A coupling is one pass as the graph has it: its name (phases.go's, where
// the pass is timed), the functions it is, the stages it runs in, and the
// world's fields it reads and writes.
type coupling struct {
	pass          string
	funcs         []string
	stages        []string
	reads, writes []string
}

// is, in, reads and writes spell a coupling's lists, so that each is one
// line.
func is(funcs ...string) []string     { return funcs }
func in(stages ...string) []string    { return stages }
func reads(fields ...string) []string { return fields }
func writes(fields ...string) []string {
	return fields
}

// couplings is every pass of Generate, by the order the stages first run
// it. A function named in funcs is the pass; what it calls is the pass too,
// down to a function another pass is.
var couplings = []coupling{
	{"newGround", is("Land.newGround"), in("ground"), reads(), writes("energy")},
	{"historyGround", is("Land.historyGround"), in("ground"), reads(), writes("energy")},
	{"history", is("Land.history"), in("ground"), reads("energy", "wind", "sea", "height", "rock", "plates", "book", "drainage", "load", "soil", "cover"), writes("energy", "wind", "sea", "height", "rock", "plates", "book", "load", "soil")},
	{"flood", is("Land.flood"), in("ground"), reads(), writes()},
	{"move", is("Land.move"), in("ground"), reads("height", "rock", "plates", "soil"), writes("height", "rock", "plates", "soil", "cover")},
	{"joinUp", is("Grid.joinUp"), in("ground"), reads("plates"), writes("plates")},
	{"tectonics", is("Land.tectonics"), in("ground"), reads("sea", "height", "rock", "plates", "book"), writes("height", "rock", "plates", "book")},
	{"reshape", is("Land.reshape"), in("ground"), reads("plates"), writes("plates")},
	{"keepBook", is("Grid.keepBook"), in("ground"), reads("energy", "sea", "height", "rock", "book", "drainage", "soil", "cover"), writes("rock", "book")},
	{"settleRock", is("Grid.settleRock"), in("ground"), reads("height", "plates", "rock"), writes("plates", "rock")},
	{"handDown", is("handDown"), in("ground"), reads("height", "rock", "plates", "book", "soil"), writes("height", "rock", "plates", "book", "soil", "cover")},
	{"settleHistory", is("Land.settleHistory"), in("ground"), reads("energy", "height", "floor", "rock", "book"), writes("height", "floor", "rock")},
	{"basins", is("Land.basins"), in("ground"), reads("height"), writes("height")},
	{"raise", is("Land.raise"), in("ground"), reads(), writes("height")},
	{"layBedrock", is("Land.layBedrock"), in("ground"), reads("height", "rock"), writes("rock")},
	{"expose", is("Grid.expose"), in("ground", "shape", "cut", "coast"), reads("height", "rock"), writes("rock")},
	{"pour", is("Grid.pour", "Grid.repour"), in("sea", "cut", "coast"), reads("sea", "height", "floor", "cover"), writes("sea", "cover", "woods", "stocks")},
	{"level", is("Grid.flood", "Grid.relevel"), in("sea", "cut", "coast"), reads("sea", "height", "cover"), writes("sea", "cover", "woods", "stocks")},
	{"layCountry", is("Grid.layCountry"), in("sea"), reads("height", "sea"), writes("height")},
	{"shape", is("Grid.shape"), in("shape"), reads("energy", "wind", "rain", "sea", "height", "floor", "rock", "cover"), writes("height", "rock")},
	{"texture", is("Land.texture"), in("shape"), reads("floor", "height", "sea"), writes("height")},
	{"denude", is("Grid.denude"), in("shape"), reads("cover", "height", "lakes", "rock", "sea"), writes("height")},
	{"landslide", is("Grid.landslide"), in("ground", "shape", "cut"), reads("energy", "wind", "rain", "sea", "height", "floor", "rock", "soil", "cover", "plants"), writes("height", "soil")},
	{"drain", is("Grid.drain"), in("ground", "shape", "cut", "coast"), reads("floor", "height", "rain", "sea", "wind"), writes()},
	{"weather", is("Grid.weather"), in("ground", "shape", "cut", "coast"), reads("energy", "wind", "rain", "year", "sea", "height", "floor", "rock", "moisture", "snow", "soil", "plants"), writes("wind", "rain", "moisture", "snow")},
	{"defaultAir", is("Grid.ensureAir"), in("ground", "shape", "cut", "coast"), reads("energy"), writes("energy")},
	{"pool", is("Grid.pool"), in("ground", "shape", "cut", "coast", "cover"), reads("energy", "wind", "rain", "sea", "height", "floor", "lakes"), writes("lakes")},
	{"flow", is("Grid.flow"), in("ground", "shape", "cut", "coast", "cover"), reads("energy", "wind", "rain", "sea", "height", "floor", "drainage", "lakes"), writes("drainage", "lakes")},
	{"gradeCountry", is("Grid.gradeCountry"), in("shape", "cut", "coast"), reads("sea", "height", "drainage"), writes()},
	{"cutValleys", is("Grid.cutValleys"), in("cut"), reads(), writes()},
	{"cutThroughCycle", is("Grid.cutThroughCycle"), in("cut"), reads("cover", "energy", "floor", "height", "rain", "rock", "sea", "soil", "wind"), writes("height", "sea", "soil")},
	{"carve", is("Grid.carve"), in("cut"), reads("sea", "height", "drainage", "lakes", "cover"), writes("cover", "woods", "stocks")},
	{"height", is("Grid.height"), in("cut", "coast"), reads("cover", "drainage", "floor", "height", "lakes"), writes("drainage")},
	{"wear", is("Grid.wear"), in("ground", "cut"), reads("energy", "wind", "rain", "year", "sea", "tide", "height", "floor", "rock", "book", "drainage", "load", "lakes", "soil", "cover", "plants"), writes("height", "book", "load", "soil")},
	{"creep", is("Grid.creep"), in("ground", "cut"), reads("energy", "wind", "rain", "sea", "height", "floor", "drainage", "soil", "cover", "plants"), writes()},
	{"waterStep", is("Grid.waterStep"), in("ground", "cut", "coast"), reads("energy", "wind", "rain", "year", "sea", "tide", "height", "floor", "rock", "drainage", "load", "lakes", "soil", "cover", "plants"), writes("load")},
	{"year", is("Land.stageCoast"), in("coast"), reads("wind", "sea", "height"), writes("year")},
	{"freeze", is("Grid.freeze"), in("coast"), reads("cover", "floor", "height", "lakes", "sea", "year"), writes("cover", "stocks")},
	{"tides", is("Grid.tides"), in("coast"), reads("energy", "wind", "rain", "year", "sea", "tide", "height", "floor", "rock", "drainage", "lakes", "soil", "cover"), writes("tide", "cover", "woods", "fertility", "stocks")},
	{"silt", is("Grid.silt"), in("coast"), reads("sea", "tide", "height", "soil"), writes("height", "soil")},
	{"cover", is("Land.stageCover"), in("cover"), reads("energy", "wind", "rain", "year", "sea", "height", "floor", "rock", "drainage", "moisture", "snow", "soil", "cover", "woods", "plants", "fertility"), writes("rain", "moisture", "snow", "cover", "woods", "plants", "fertility", "stocks")},
	{"readWoods", is("Grid.readWoods"), in("cover"), reads("energy", "wind", "rain", "year", "sea", "height", "floor", "drainage", "snow", "cover", "woods", "plants"), writes("woods")},
	{"soilTexture", is("Grid.soilTexture"), in("ground", "cover"), reads("cover", "energy", "height", "rain", "rock", "sea", "wind"), writes("soil")},
	{"laySoil", is("Grid.laySoil"), in("cover"), reads("energy", "wind", "rain", "year", "sea", "height", "floor", "rock", "drainage", "soil", "cover", "plants"), writes("soil")},
	{"recount", is("Grid.Recount"), in("cover"), reads("sea", "height", "floor"), writes()},
	{"readFeatures", is("Grid.readFeatures"), in("cover"), reads("energy", "wind", "rain", "year", "sea", "height", "plates", "book", "drainage", "snow", "lakes", "cover"), writes("features")},
	{"readRelations", is("Grid.readRelations"), in("cover"), reads("energy", "wind", "rain", "sea", "height", "plates", "lakes"), writes()},
}

// A worldField is one of the world's fields as the graph has it: its name,
// its system, what it is, and the fields of the Grid that hold it - a
// tile's as "Tiles.Bedrock".
type worldField struct {
	name, system, about string
	grid                []string
}

// The systems, in the order the graph lays them out.
var systems = []string{"air", "sea", "rock", "water", "land and life", "registry"}

// worldFields is every field of the world the passes couple through.
var worldFields = []worldField{
	{"energy", "air", "the energy balance row by row: the year's mean warmth, what the air can take up, how wet it is", []string{"air"}},
	{"wind", "air", "the wind, the pressure, the air's budget of water and the sea's currents, as package atmos works them out", []string{"winds", "aired"}},
	{"rain", "air", "each tile's year of rain, what runs off, its summer's share and its day's range, and each air cell's share of the year's evaporation by phase", []string{"rain", "runoff", "rainWarm", "dayRange", "petShare"}},
	{"year", "air", "each tile's year at sea level: its mean and its swing", []string{"warm", "swing"}},
	{"sea", "sea", "the level of the sea and of the water the air takes its fill from", []string{"sea", "base"}},
	{"tide", "sea", "the tide's reach and the flats it lays bare", []string{"tide", "tidal", "ebb"}},
	{"height", "rock", "the ground's height, and on a globe the height of the country it lies in at the planet's scale", []string{"Height", "country", "planetHeight"}},
	{"floor", "rock", "the deep sea floor: where it stood before its age laid it down, how fast the rock rose, how old the crust is", []string{"abyss", "uplift", "floorAge"}},
	{"rock", "rock", "the rock: each tile's bed, the epoch it was laid in, and the beds under it", []string{"Tiles.Bedrock", "Tiles.Formed", "strata"}},
	{"plates", "rock", "which plate each tile rides, which plate each has been welded into, and the hotspots", []string{"Tiles.Plate", "plateRoot", "hot", "welds"}},
	{"book", "rock", "the book the history kept of what it did to each tile, and how many epochs it ran", []string{"ledger", "epochs"}},
	{"drainage", "water", "where each tile's water goes and how much runs through it, and how far it stands over it", []string{"Flow", "Drain", "area", "water", "down", "route"}},
	{"load", "water", "the ground the water carries: off the land, and off the banks of its bends", []string{"exported", "bankLoad", "toSea"}},
	{"moisture", "water", "the soil's water through the year: each phase's rain into it, what it holds and what it sheds, and the most it holds", []string{"rainIn", "soilWater", "runoffIn", "soilHold"}},
	{"snow", "water", "the snow through the year: the water lying in it each phase, the share of the ground it covers, what melts, and the mass balance of the snow that outlasts the year", []string{"snowWater", "snowCover", "meltIn", "ice"}},
	{"lakes", "water", "the standing water: the lakes, their level, and the salt pans", []string{"Lakes", "lakeLevel", "lakeOf", "pans"}},
	{"soil", "land and life", "the soil: how deep, what it is made of, and what time has made of it", []string{"Soil", "Sand", "Clay", "Tiles.Leached", "Tiles.Exposed", "Tiles.Lime", "Tiles.Salt", "Tiles.Carbon", "pedons"}},
	{"cover", "land and life", "what each tile is: open ground, forest, water, ice, outcrop, tidal flat, salt", []string{"Tiles.Terrain"}},
	{"woods", "land and life", "where trees will take, and the map's measure of its ground they are read against", []string{"Wood", "Wild", "holds", "steepAt", "steepLine", "woodsLine", "woodsRead", "climateWoods", "twiMean"}},
	{"plants", "land and life", "what grows on each tile, type by type: the share of its ground each covers, the carbon it holds and its leaf area, and the share of its ground its fires burn in a year", []string{"vegCover", "vegLeaf", "vegMass", "burned"}},
	{"fertility", "land and life", "what the soil will grow", []string{"Fertility", "Rich"}},
	{"stocks", "land and life", "what stands to be taken: the fish, the grass, and how long what stands has grown", []string{"Fish", "Age", "Sward"}},
	{"features", "registry", "the registry of the things the tiles make up, and their relations", []string{"features"}},
}

// frameFields are the Grid's fields that are none of the world's: the map's
// size and shape, the scale a history reads it at, a settlement's marks and
// tallies, and every pass's working memory. A Grid field is either here or
// in one world field.
var frameFields = []string{
	"W", "H", "Wrap", "Tiles", "deep", "planet",
	"Tiles.Mark", "Tiles.Owner", "Tiles.Fenced", "Traffic", "Kinds", "CW", "CH", "Chunks", "PW", "PH", "patches", "Active", "lenders",
	"regions", "regionStack", "regionsStale", "waters", "router", "landmarks", "islanded",
	"seam", "seamQueue", "floodScratch", "slideScratch", "fillScratch", "poolScratch",
	"flowScratch", "stepScratch", "creepScratch", "paw",
}

// Coupling is one pass of world creation as the coupling graph has it: the
// stages it runs in, and the world's fields it reads and writes (see
// WorldFields).
type Coupling struct {
	Pass          string
	Stages        []string
	Reads, Writes []string
}

// Couplings is every pass of world creation, by the order the stages first
// run it.
func Couplings() []Coupling {
	out := make([]Coupling, len(couplings))
	for k, c := range couplings {
		out[k] = Coupling{Pass: c.pass, Stages: slices.Clone(c.stages), Reads: slices.Clone(c.reads), Writes: slices.Clone(c.writes)}
	}
	return out
}

// WorldField is one of the world's fields the passes couple through: its
// name, the system it belongs to, and what it is.
type WorldField struct {
	Name, System, About string
}

// WorldFields is every field of the world the coupling graph joins, system
// by system.
func WorldFields() []WorldField {
	out := make([]WorldField, len(worldFields))
	for k, f := range worldFields {
		out[k] = WorldField{Name: f.name, System: f.system, About: f.about}
	}
	return out
}

// CouplingSystems is the world's systems, in the order the graph lays them
// out.
func CouplingSystems() []string { return slices.Clone(systems) }

// A CouplingLoop is a feedback loop in the coupling graph: each field
// drives the next, the last the first, and By[k] is the passes that read
// Fields[k] and write the field after it.
type CouplingLoop struct {
	Fields []string
	By     [][]string
}

// loopFields is the longest loop CouplingLoops looks for, in fields: a
// longer one goes round through a shorter one's fields, and there are
// thousands of them.
const loopFields = 3

// CouplingLoops is every feedback loop of the coupling graph of up to
// loopFields fields, shortest first and then in the order of their fields,
// each started at its first field in the order of WorldFields. A pass that
// reads a field and writes it again is not a loop: it is one system working
// on itself.
func CouplingLoops() []CouplingLoop {
	edges := couplingEdges()
	at := map[string]int{}
	for k, f := range worldFields {
		at[f.name] = k
	}
	var loops []CouplingLoop
	var path []string
	var walk func(start string)
	walk = func(start string) {
		last := path[len(path)-1]
		for _, next := range edges.from(last) {
			switch {
			case next == start && len(path) > 1:
				l := CouplingLoop{Fields: slices.Clone(path)}
				for k := range path {
					l.By = append(l.By, edges.by[[2]string{path[k], path[(k+1)%len(path)]}])
				}
				if !onePass(l.By) {
					loops = append(loops, l)
				}
			case at[next] > at[start] && !slices.Contains(path, next) && len(path) < loopFields:
				path = append(path, next)
				walk(start)
				path = path[:len(path)-1]
			}
		}
	}
	for _, f := range worldFields {
		path = append(path[:0], f.name)
		walk(f.name)
	}
	slices.SortStableFunc(loops, func(a, b CouplingLoop) int { return cmp.Compare(len(a.Fields), len(b.Fields)) })
	return loops
}

// onePass says one pass alone is every step of a loop: the loop is that
// pass reading and writing its own fields, and not one pass driving
// another.
func onePass(by [][]string) bool {
	for _, b := range by {
		if len(b) != 1 || b[0] != by[0][0] {
			return false
		}
	}
	return true
}

// edgeSet is the coupling graph field to field: by is the passes that read
// the first and write the second.
type edgeSet struct {
	by map[[2]string][]string
}

func couplingEdges() edgeSet {
	e := edgeSet{by: map[[2]string][]string{}}
	for _, c := range couplings {
		for _, r := range c.reads {
			for _, w := range c.writes {
				if r != w {
					k := [2]string{r, w}
					e.by[k] = append(e.by[k], c.pass)
				}
			}
		}
	}
	return e
}

// from is the fields f drives, in the order of WorldFields.
func (e edgeSet) from(f string) []string {
	var out []string
	for _, w := range worldFields {
		if _, ok := e.by[[2]string{f, w.name}]; ok {
			out = append(out, w.name)
		}
	}
	return out
}

// CouplingSets is the strongly connected sets of the coupling graph: each
// is fields every one of which drives every other, round some loop, in the
// order of WorldFields, and only those of more than one field. A field in
// none drives others or is driven, and is never driven back.
func CouplingSets() [][]string {
	edges := couplingEdges()
	// Tarjan's, over the fields in the order of WorldFields.
	index, low := map[string]int{}, map[string]int{}
	on := map[string]bool{}
	var stack []string
	var sets [][]string
	next := 0
	var visit func(f string)
	visit = func(f string) {
		index[f], low[f] = next, next
		next++
		stack = append(stack, f)
		on[f] = true
		for _, t := range edges.from(f) {
			if _, seen := index[t]; !seen {
				visit(t)
				low[f] = min(low[f], low[t])
			} else if on[t] {
				low[f] = min(low[f], index[t])
			}
		}
		if low[f] == index[f] {
			var set []string
			for {
				t := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				on[t] = false
				set = append(set, t)
				if t == f {
					break
				}
			}
			if len(set) > 1 {
				sets = append(sets, set)
			}
		}
	}
	for _, f := range worldFields {
		if _, seen := index[f.name]; !seen {
			visit(f.name)
		}
	}
	at := map[string]int{}
	for k, f := range worldFields {
		at[f.name] = k
	}
	for _, s := range sets {
		slices.SortFunc(s, func(a, b string) int { return cmp.Compare(at[a], at[b]) })
	}
	slices.SortFunc(sets, func(a, b []string) int { return cmp.Compare(at[a[0]], at[b[0]]) })
	return sets
}

// systemOf is the system each world field is of.
func systemOf(field string) string {
	for _, f := range worldFields {
		if f.name == field {
			return f.system
		}
	}
	return ""
}

// loopSystems is the systems a loop goes through, in the order of systems.
func loopSystems(l CouplingLoop) []string {
	var out []string
	for _, s := range systems {
		for _, f := range l.Fields {
			if systemOf(f) == s {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// CouplingsMarkdown is docs/couplings.md: the graph, its passes and its
// loops, as the declaration has them.
func CouplingsMarkdown() string {
	var b strings.Builder
	p := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	p("# The coupling graph\n\n")
	p("Generated from `couplings.go` by `TERRA_COUPLINGS=write go test -run TestCouplingsDoc -timeout 60m .`; do not edit it by hand.\n\n")
	p("Which pass of world creation reads which of the world's fields and writes which, and the feedback loops that makes. ")
	p("The declaration is held to the code by `couplings_test.go`: each pass's source is read for the fields it touches, ")
	p("and worlds are made stage by stage and every field a stage changes has to be one of that stage's passes' writes. ")
	p("A loop a later change adds - the climate of each epoch (#57), the carbon thermostat (#58), albedo from the surface (#59), ")
	p("land and air trading water (#60), the ice ages (#61), mountains and climate (#62), sea and air together (#28), fog over cold water (#29) - ")
	p("is one line in `couplings`, and a field it brings one line in `worldFields`.\n\n")

	p("## The fields\n\n| System | Field | What it is |\n|---|---|---|\n")
	for _, s := range systems {
		for _, f := range worldFields {
			if f.system == s {
				p("| %s | %s | %s |\n", s, f.name, f.about)
			}
		}
	}

	edges := couplingEdges()
	// The systems' graph: an arrow for each pair a pass joins, with how many
	// of the fields' arrows it stands for.
	between := map[[2]string]int{}
	for k := range edges.by {
		from, to := systemOf(k[0]), systemOf(k[1])
		if from != to {
			between[[2]string{from, to}]++
		}
	}
	p("\n## The systems\n\nSystem to system: an arrow is a pass that reads a field of the one and writes a field of the other, ")
	p("numbered with how many such pairs of fields there are.\n\n```mermaid\nflowchart LR\n")
	id := func(s string) string { return strings.ReplaceAll(s, " ", "_") }
	for _, s := range systems {
		p("  %s[%s]\n", id(s), s)
	}
	for _, from := range systems {
		for _, to := range systems {
			if n := between[[2]string{from, to}]; n > 0 {
				p("  %s -- %d --> %s\n", id(from), n, id(to))
			}
		}
	}
	p("```\n\n")

	p("## The graph\n\nField to field: each field, and the fields the passes that read it write.\n\n| Field | Drives |\n|---|---|\n")
	for _, f := range worldFields {
		var to []string
		for _, t := range edges.from(f.name) {
			to = append(to, fmt.Sprintf("%s (%s)", t, strings.Join(edges.by[[2]string{f.name, t}], ", ")))
		}
		p("| %s | %s |\n", f.name, strings.Join(to, "; "))
	}

	p("\n## The passes\n\n| Pass | Stages | Reads | Writes |\n|---|---|---|---|\n")
	for _, c := range couplings {
		p("| %s | %s | %s | %s |\n", c.pass, strings.Join(c.stages, ", "), strings.Join(c.reads, ", "), strings.Join(c.writes, ", "))
	}

	p("\n## The loops\n\n")
	sets := CouplingSets()
	p("Fields that drive one another round some loop, each set by Tarjan's strongly connected components:\n\n")
	for _, s := range sets {
		p("- %s\n", strings.Join(s, ", "))
	}
	loops := CouplingLoops()
	var two, three []CouplingLoop
	for _, l := range loops {
		if len(l.Fields) == 2 {
			two = append(two, l)
		} else {
			three = append(three, l)
		}
	}
	p("\nEvery loop of up to %d fields, found by walking the graph: each field drives the next and the last the first, through the passes named, ", loopFields)
	p("with the systems it goes through. A loop one pass makes alone, reading and writing its own fields, is not counted. ")
	p("%d loops: %d of two fields and %d of three.\n\n", len(loops), len(two), len(three))
	line := func(l CouplingLoop) {
		var parts []string
		for k, f := range l.Fields {
			parts = append(parts, fmt.Sprintf("%s -(%s)->", f, strings.Join(l.By[k], ", ")))
		}
		p("- %s %s [%s]\n", strings.Join(parts, " "), l.Fields[0], strings.Join(loopSystems(l), ", "))
	}
	for _, l := range two {
		line(l)
	}
	p("\n<details><summary>The %d loops of three fields</summary>\n\n", len(three))
	for _, l := range three {
		line(l)
	}
	p("\n</details>\n")
	return b.String()
}
