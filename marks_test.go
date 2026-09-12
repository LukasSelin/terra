package terra

// The marks these tests build with.
//
// The land carries whatever marks a game registers and knows nothing about
// any of them, so its own tests cannot borrow a settlement's houses and
// roads: they register their own. The numbers are a settlement's, because
// the tests that read them were written against a settlement's ground and
// what they are checking is the land's arithmetic rather than the figures.
const (
	blocking Mark = iota + 1 // something standing in the way of a walker
	paving                   // something laid to be walked on
	rooted                   // the middle of a place, which cannot come down
)

// A variable and not an init, for the reason given in mark.go: anything
// worked out at the making of a variable would read a table an init has not
// filled yet.
var testMarks = func() bool {
	SetMark(blocking, MarkDef{Cost: 1.3, Roofs: true})
	SetMark(paving, MarkDef{Cost: 0.5, Drain: 0.7, Way: true})
	SetMark(rooted, MarkDef{Cost: 1, Roofs: true, Settles: true, Fixed: true})
	return true
}()
