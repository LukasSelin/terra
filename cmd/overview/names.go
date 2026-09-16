package main

import (
	"strings"

	"github.com/LukasSelin/terra"
)

// Names for the features, from the seed.
//
// terra names nothing: a name is a culture's, and a game gives its own. The
// overview has no culture, so it gives each feature a made-up word from the
// seed and the feature's id - the same seed names the same range the same
// way every time - and the word the feature's kind goes by.

// namerFor is a namer that names every feature of the world made from seed.
func namerFor(seed uint64) func(terra.Feature) string {
	return func(f terra.Feature) string {
		word := wordFor(seed, uint64(f.ID)*7919+uint64(f.Kind))
		switch f.Kind {
		case terra.UpliftBelt:
			switch f.Meeting {
			case terra.Rift:
				return "the " + word + " Rift"
			case terra.Islands:
				return "the " + word + " Islands"
			case terra.Hotspot:
				return "the " + word + " Volcanoes"
			}
			if f.Count < 10 {
				return "the " + word + " Hills"
			}
			return "the " + word + " Mountains"
		case terra.DrainageBasin:
			return "the " + word
		case terra.StandingLake:
			return "Lake " + word
		case terra.CrustPlate:
			return "the " + word + " Plate"
		case terra.ClimateRegion:
			switch f.Group {
			case 'A':
				return "the " + word + " Tropics"
			case 'B':
				return "the " + word + " Drylands"
			case 'C':
				return "the " + word + " Lowlands"
			case 'D':
				return "the " + word + " Uplands"
			}
			return "the " + word + " Wastes"
		}
		return word
	}
}

var (
	onsets  = []string{"", "b", "d", "f", "g", "h", "k", "l", "m", "n", "r", "s", "t", "v", "th", "br", "kr", "st"}
	vowels  = []string{"a", "e", "i", "o", "u", "ai", "au", "or", "en"}
	closers = []string{"", "n", "r", "l", "s", "k", "th", "nd", "st", "rn"}
)

// wordFor is a two- or three-syllable word from a seed and a key, the same
// every time, made with its first letter up.
func wordFor(seed, key uint64) string {
	x := seed*0x9E3779B97F4A7C15 ^ key*0xBF58476D1CE4E5B9
	next := func(n int) int {
		x ^= x >> 31
		x *= 0x94D049BB133111EB
		x ^= x >> 29
		return int(x % uint64(n))
	}
	var b strings.Builder
	syllables := 2 + next(2)
	for s := 0; s < syllables; s++ {
		b.WriteString(onsets[next(len(onsets))])
		b.WriteString(vowels[next(len(vowels))])
		if s == syllables-1 || next(3) == 0 {
			b.WriteString(closers[next(len(closers))])
		}
	}
	w := b.String()
	if w == "" {
		w = "ur"
	}
	return strings.ToUpper(w[:1]) + w[1:]
}
