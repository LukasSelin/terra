package terra

// Ground that is spoken for.
//
// The land has to know that a piece of ground has been claimed, because its
// own slow work leaves such ground alone: a hillside somebody is farming does
// not creep downhill and a river does not wander through a holding. What it
// does not have to know is who did the claiming. A settlement claims ground
// for a farmer; another game might claim it for a faction, a guild or a
// keep, and the land can carry any of them because it never asks.
//
// So a claim is an identity and nothing else. The land compares two of them
// and never looks inside one. What a Holder means - and how it is got from
// whoever the game thinks owns things - is stated where the game is stated,
// at the one line that converts.

// Holder is whoever a piece of ground is spoken for by, and whoever a route
// is being walked for. Zero is nobody: unclaimed ground, and a walker with
// no fields of their own.
//
// It is a bare number on purpose. The land tests it for zero and tests two
// of them for equality, which between them are the only two questions it
// has: is this ground anybody's, and is it this walker's. Anything more -
// who the holder is, whether they are still alive, what else they hold - is
// asked on the game's side of the split, of the game's own record.
type Holder int
