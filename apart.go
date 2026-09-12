package terra

// Acting on the land in pieces.
//
// Several goroutines may act on one map at once where what each of them
// touches is far enough from the rest that none can see another's work. How
// to cut the work up is a question about whoever is acting and not about the
// ground, so the land does not answer it. What has to be copied to act
// apart, and what has to be added up again afterwards, are facts about how
// the map is kept - and so they are here, rather than in whichever game is
// doing the cutting.
//
// A copy shares the tiles. That is the point of it: the pieces are chosen so
// that no two of them write the same ground. What a copy does not share is
// what is kept about the map as a whole - the landmark slack, whether the
// water has moved, whether the labels need taking again - and those are the
// three things Rejoin puts back.

// Slack is the map's landmark slack: how much cheaper the ground has been
// made than the tables know about, by chunk. A caller acting apart keeps its
// own and hands it back.
func (g *Grid) Slack() []float64 { return g.landmarks.slack }

// UseSlack tells the map to keep its landmark slack in s rather than where
// it was keeping it, so that a copy's changes land in a buffer the caller
// holds.
func (g *Grid) UseSlack(s []float64) { g.landmarks.slack = s }

// On points the router at g and forgets whatever it was doing.
func (r *Router) On(g *Grid) *Router {
	r.g = g
	r.Reset()
	return r
}

// Apart makes into a working copy of g that one goroutine may act on beside
// others: the same tiles, its own landmark slack kept in the buffer given,
// and the routers handed over pointed at it. The slack buffer is returned
// because it may have grown.
//
// The copy is marked as one, so that what cannot be done while others are
// acting can say so rather than quietly doing it; see Islanded.
func (g *Grid) Apart(into *Grid, slack []float64, own *Router, more []*Router) []float64 {
	*into = *g
	into.islanded = true
	slack = append(slack[:0], g.landmarks.slack...)
	into.landmarks.slack = slack
	own.On(into)
	own.holder = 0
	into.router = own
	for _, r := range more {
		r.On(into)
	}
	return slack
}

// Rejoin adds what a copy changed into g, where before is g as it stood when
// the copy was made. Everything here is a difference rather than a value,
// because several copies are put back one after another and each of them
// only knows about its own.
func (g *Grid) Rejoin(apart, before *Grid) {
	g.waters += apart.waters - before.waters
	if apart.regionsStale {
		g.regionsStale = true
	}
	// What a copy built that made its ground cheaper goes on the tables'
	// slack. The pieces are chunks apart, so no chunk hears from two.
	for c := range g.landmarks.slack {
		g.landmarks.slack[c] += apart.landmarks.slack[c] - before.landmarks.slack[c]
	}
	if !apart.landmarks.ladenOK {
		g.landmarks.ladenOK = false
	}
	if apart.landmarks.stale {
		g.landmarks.stale = true
	}
}

// Seed is what this land was made from, for a caller that needs a stream of
// chance of its own tied to this world rather than to the clock.
func (l *Land) Seed() uint64 { return l.seed }

// UseRouters tells the land to route on these rather than on its own pool,
// which is how a copy acted on apart keeps its routing to itself.
func (l *Land) UseRouters(rs []*Router) { l.routers = rs }

// RouterPool is the pool as it stands, so that a copy's grown pool can be
// kept for the next day rather than made again.
func (l *Land) RouterPool() []*Router { return l.routers }
