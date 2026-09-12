package terra

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

// Making a world is mostly the same arithmetic done half a million times -
// the ground drawn, then measured, then read - and most of those passes are
// spread over goroutines. What comes out must not know that.
//
// The chance is what makes this worth saying. A seed means the order the
// world's chance comes out in, and the passes that draw - the corners of a
// lattice, the luck thrown on a tile's suitability for trees - are kept in
// walks of their own with only the arithmetic spread. If any draw ever
// wandered into a spread pass, these hashes would part company on the
// second run.
//
// The tile is written out field by field rather than printed whole, for the
// same reason the golden digest in package system is: where a field is kept
// is not what is being checked.
func digest(l *Land) string {
	h := sha256.New()
	g := l.Grid
	for i := range g.Tiles {
		t := &g.Tiles[i]
		fmt.Fprintf(h, "{%v %v %v %v %v %v %v %v %v %v %v %v %v %v %v %v %v %v %v}",
			t.Terrain, t.Mark, t.Owner, g.Fertility[i], g.Rich[i], g.Wood[i], g.Wild[i], g.Fish[i],
			t.Height, t.Flow, t.Drain, t.Bedrock, t.Sand, t.Clay, t.Plate, t.Formed,
			g.Age[i], t.Fenced, g.Traffic[i])
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func madeOver(t *testing.T, seed uint64, terms Terms, workers int) string {
	t.Helper()
	was := Workers
	Workers = workers
	defer func() { Workers = was }()
	return digest(NewLand(seed, terms))
}

func TestMakingAWorldDoesNotDependOnTheGoroutines(t *testing.T) {
	// A quarter of the globe preset's width and a quarter of its height, and
	// everything else about it the same: wrapped, salted with a sea, and made
	// out of sixteen epochs of its own history. What is being asked here is
	// whether a world depends on how the work was dealt out between
	// goroutines, and every pass that deals work out is exercised by a
	// sixteenth of the tiles exactly as well as by all of them - while the
	// full preset, made five times over, is a minute and a half of it.
	small := GlobeTerms()
	small.Width, small.Height = 256, 128
	worlds := map[string]Terms{
		"globe":   small,
		"ancient": AncientTerms(),
		"valley":  DefaultTerms(),
	}
	// Not in parallel with one another: what is being varied is a package
	// variable, and two subtests varying it at once would be measuring each
	// other.
	for name, terms := range worlds {
		t.Run(name, func(t *testing.T) {
			one := madeOver(t, 1, terms, 1)
			for _, workers := range []int{2, 3, 8, 16} {
				if got := madeOver(t, 1, terms, workers); got != one {
					t.Fatalf("%s: %d goroutines made a different world than 1 did", name, workers)
				}
			}
		})
	}
}
