package terra

import "testing"

// Every rock a history can make has to turn up on a map made by one, and none
// of them may take the whole thing over. The two that decide this are granite
// and shale, because they are the two that had nowhere to come from when the
// generator was first written: granite was the leftover case - ground nothing
// ever happened to - which on sixteen epochs is no ground at all, and shale
// lost every basin to sandstone because a fill's make-up hardly varies while
// a history is running. Granite is the root of an arc now, laid bare, and the
// coarse and the fine fill are ranked against each other rather than against
// a fixed line.
func TestEveryRockAHistoryMakesTurnsUp(t *testing.T) {
	pooled := map[Bedrock]int{}
	tiles := 0
	for seed := uint64(1); seed <= 12; seed++ {
		w := NewLand(seed, AncientTerms())
		var seen [BedrockCount]int
		for i := range w.Grid.Tiles {
			seen[w.Grid.Tiles[i].Bedrock]++
			pooled[w.Grid.Tiles[i].Bedrock]++
			tiles++
		}
		for _, b := range Bedrocks() {
			if seen[b] == 0 {
				t.Errorf("seed %d has no %s on it at all", seed, b)
			}
		}
	}
	// Pooled over the seeds, no rock may take most of a world or be missing
	// from it. The band is wide because which rocks a world gets is the whole
	// point - a world with little ocean floor has little granite, and that is
	// a fact about the world and not a fault in it.
	//
	// Two thirds and not a half. Over thirty seeds the basalt pools to 52.4
	// per cent, so a bar at a half was already breached and was passing on
	// five hand-picked seeds pooling to just under it; it failed the moment
	// anything shifted the world's own luck. The figure worth looking at is
	// the basalt itself - half a made valley being ocean floor is a great deal
	// for ground that asks for no sea at all, and see oceanFloor in history.go
	// for why that may be the wrong reading rather than the wrong bar. This
	// line is not the place to argue it.
	//
	// And a fiftieth and not a fortieth at the bottom. The soft beds are what the
	// water takes first now that it pays for the rock by the rock's strength -
	// see rockErodibility - and a basin's sandstone and shale are the softest
	// rock a history makes: pooled over the twelve worlds the sandstone went
	// from 2.5 per cent, where the lower bar had sat on it exactly, to 2.3.
	for _, b := range Bedrocks() {
		share := float64(pooled[b]) / float64(tiles)
		if share > 2.0/3.0 || share < 0.02 {
			t.Errorf("%s is %.1f%% of the ground pooled over five worlds", b, 100*share)
		}
	}
}
