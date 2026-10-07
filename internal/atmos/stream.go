package atmos

// The sea's current read as a whole: where it turns, and round what.
//
// The current on each cell says which way the water there goes and nothing
// about the ocean it goes round. What does is the streamfunction: the current
// drawn as the contours of one surface, the water running along them, fast
// where they crowd and round and round a hill or a hollow of it. A gyre is a
// hill or a hollow; its western current is where the contours crowd against
// the shore.
//
// O2 (#66) first worked ψ out again from the current's spin, by a Poisson
// solve with every shore held at nought, because the current was then
// closed row by row and had no ψ of its own. Since M1 (#73) the gyres are
// solved for ψ itself (flow.go), and Stream reads that: the one ψ the
// current is made from. (This is the integration's glue, #82, brought to
// M1 when O2 reached main.)

// GyreDepth is how deep, in metres, the water the gyres drive round goes: the
// depth a current's speed is taken over for the water it carries. See
// gyreDepth.
const GyreDepth = gyreDepth

// Stream is the sea's current as a transport streamfunction ψ over each
// cell, in sverdrups: the current toward the east is -∂ψ/∂y and toward the
// north ∂ψ/∂x, so the water goes round a hill of ψ clockwise, seen from
// above, and round a hollow the other way, and between two contours the
// whole depth of the moving water carries their difference. It is M1's Psi
// (see flow.go). The Ekman drift, which has no ψ, is not in it.
//
// It is nil where there is no current.
func (e *Env) Stream() []float64 {
	if e.Psi == nil {
		return nil
	}
	psi := make([]float64, len(e.Psi))
	for i, p := range e.Psi {
		psi[i] = float64(p)
	}
	return psi
}
