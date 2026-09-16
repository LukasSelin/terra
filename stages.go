package terra

import "github.com/LukasSelin/terra/internal/phase"

// A world is made in stages, each a function of the grid the stage before
// it left, the world's terms and the land's chance:
//
//	ground -> sea -> shape -> cut -> coast -> cover
//
// ground is the history, or the drawn map and the rock laid under it; sea
// the water poured or flooded; shape the ground laid again at the map's tile
// span, with the drainage read off it; cut the valleys worn and the sea
// levelled on them; coast each tile's year, the sea's ice, the tide and the
// mud; cover the woods, the outcrops, the soil and the features.
//
// The hand-off between two stages is everything on the Grid and the position
// of Land.RNG, and nothing else: no stage reads a local of another. That is
// what lets a stage be started from a hand-off that was kept rather than
// made, which is what a history file is (see historyfile.go): the ground
// stage is two thirds of making a globe, and the stages after it can be run
// again, and changed, without it.
//
// The order is the world's. A stage never moves between two others, and a
// new pass goes inside the stage whose ground it reads.
type stage struct {
	name string
	run  func(w *Land, g *Grid, cfg Terms)
}

// stages are the stages of Generate in the order they run.
var stages = [...]stage{
	{"stage.ground", (*Land).stageGround},
	{"stage.sea", (*Land).stageSea},
	{"stage.shape", (*Land).stageShape},
	{"stage.cut", (*Land).stageCut},
	{"stage.coast", (*Land).stageCoast},
	{"stage.cover", (*Land).stageCover},
}

// Stage indices that are named elsewhere.
const (
	stageGround = 0
	stageSea    = 1
)

// generateFrom runs the stages numbered from up to but not including to on
// g, which has to be the grid the stage before from left, and tells watch,
// where there is one, of each as it ends.
func (w *Land) generateFrom(g *Grid, cfg Terms, from, to int, watch StageWatch) {
	for _, s := range stages[from:to] {
		stop := phase.Start(s.name)
		s.run(w, g, cfg)
		stop()
		if watch != nil {
			watch(s.name[len("stage."):], w, g)
		}
	}
}

// Stages are the names a StageWatch is told, in the order the stages end.
func Stages() []string {
	names := make([]string, len(stages))
	for i, s := range stages {
		names[i] = s.name[len("stage."):]
	}
	return names
}

// A StageWatch is told of each stage of a making as it ends, by name (see
// Stages), with the land and the grid the stage has left: a way to look at a
// world part-way through, and to find the stage a change first shows in.
//
// The grid is the hand-off to the next stage, and a watch only reads it.
// Until the cover stage is over, l.Grid is nil and the grid lacks what the
// later stages add: the climate before the coast, the features before the
// end of the cover, and on a drawn map the strata and the book throughout.
// What it reads it reads before the call returns; the next stage changes it.
type StageWatch func(stage string, l *Land, g *Grid)
