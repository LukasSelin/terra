package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

// An -expect file is JSON: a list of checks, each a measure of one array's
// changes and the bounds it must lie within.
//
//	{"checks": [
//	  {"array": "ground/height", "where": {"by": "book/meeting", "not": ["collision"]},
//	   "measure": "share", "max": 0.001},
//	  {"array": "ground/height", "where": {"by": "book/meeting", "is": ["collision"]},
//	   "measure": "mean", "above": 0},
//	  {"array": "climate/koppen", "measure": "share", "max": 0.02},
//	  {"array": "ground/flow", "measure": "changed", "max": 0},
//	  {"array": "tile/terrain", "measure": "code", "from": "water", "to": "open", "max": 1000}
//	]}
type Expectations struct {
	Checks []Check `json:"checks"`
}

// Check is one expectation.
type Check struct {
	// Name, if given, is printed with the check.
	Name  string `json:"name,omitempty"`
	Array string `json:"array"`
	// Where holds the measure to the tiles of some codes of a map, read
	// from the store -by-side names.
	Where *Where `json:"where,omitempty"`
	// Measure is one of measures.
	Measure string `json:"measure"`
	// From and To are the codes of a "code" measure, by name or number,
	// each named from its own store's flags.
	From *token `json:"from,omitempty"`
	To   *token `json:"to,omitempty"`
	// The bounds: min and max inclusive, above and below not. One at least.
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
	Above *float64 `json:"above,omitempty"`
	Below *float64 `json:"below,omitempty"`
}

// Where is the tiles whose code in the map By is among Is, or not among Not.
type Where struct {
	By  string  `json:"by"`
	Is  []token `json:"is,omitempty"`
	Not []token `json:"not,omitempty"`
}

// measures is what a check can measure: elements changed, tiles changed,
// the share of tiles changed, the signed change's mean and percentiles and
// the elements that went up and down, and the elements whose code changed
// from one code to another.
var measures = []string{"changed", "tiles", "share", "mean", "p5", "p50", "p95", "up", "down", "code"}

// token is a code, by name or number: "collision" or 1.
type token string

func (t *token) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch v := v.(type) {
	case string:
		*t = token(v)
	case float64:
		*t = token(strconv.FormatFloat(v, 'f', -1, 64))
	default:
		return fmt.Errorf("a code is a name or a number, not %s", b)
	}
	return nil
}

func tokenStrings(ts []token) []string {
	s := make([]string, len(ts))
	for i, t := range ts {
		s[i] = string(t)
	}
	return s
}

// Result is a check and how it went: the value it measured, and a note
// where it could measure none.
type Result struct {
	Check Check
	Pass  bool
	Value *float64 `json:",omitempty"`
	Note  string   `json:",omitempty"`
}

// readExpectations reads and checks an -expect file.
func readExpectations(path string) ([]Check, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var x Expectations
	if err := dec.Decode(&x); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for i, c := range x.Checks {
		bad := func(format string, args ...any) error {
			return fmt.Errorf("%s: check %d: %s", path, i+1, fmt.Sprintf(format, args...))
		}
		switch {
		case c.Array == "":
			return nil, bad("no array")
		case !slices.Contains(measures, c.Measure):
			return nil, bad("measure %q is not one of %s", c.Measure, strings.Join(measures, ", "))
		case c.Min == nil && c.Max == nil && c.Above == nil && c.Below == nil:
			return nil, bad("no min, max, above or below")
		case c.Measure == "code" && (c.From == nil || c.To == nil):
			return nil, bad("a code measure needs from and to")
		case c.Measure == "code" && c.Where != nil:
			return nil, bad("a code measure is of the whole array; it takes no where")
		case c.Measure != "code" && (c.From != nil || c.To != nil):
			return nil, bad("from and to are for a code measure")
		case c.Where != nil && c.Where.By == "":
			return nil, bad("a where needs by")
		case c.Where != nil && (len(c.Where.Is) > 0) == (len(c.Where.Not) > 0):
			return nil, bad("a where needs is or not, and not both")
		}
	}
	return x.Checks, nil
}

// holds is whether v lies within the check's bounds.
func (c Check) holds(v float64) bool {
	return (c.Min == nil || v >= *c.Min) && (c.Max == nil || v <= *c.Max) &&
		(c.Above == nil || v > *c.Above) && (c.Below == nil || v < *c.Below)
}

// evaluate measures each check against the report.
func evaluate(r *Report, e *setup, checks []Check) ([]Result, error) {
	var results []Result
	for k, c := range checks {
		v, note, err := measure(r, e, k, c)
		if err != nil {
			return nil, fmt.Errorf("check %d: %w", k+1, err)
		}
		res := Result{Check: c, Note: note}
		if note == "" {
			res.Value = &v
			res.Pass = c.holds(v)
		}
		results = append(results, res)
	}
	return results, nil
}

// measure is what check k measures, or a note saying why it measures
// nothing, which fails the check.
func measure(r *Report, e *setup, k int, c Check) (float64, string, error) {
	var d *Array
	for _, a := range r.Arrays {
		if a.Path == c.Array {
			d = a
		}
	}
	switch {
	case d != nil:
	case slices.Contains(r.Skipped, c.Array):
		return 0, "left out: the masks do not lie over it", nil
	case slices.Contains(r.OnlyA, c.Array):
		return 0, "only in a", nil
	case slices.Contains(r.OnlyB, c.Array):
		return 0, "only in b", nil
	default:
		return 0, "in neither store", nil
	}
	if !d.Compared {
		return 0, "not compared: the shapes or types differ", nil
	}

	tiles, tilesChanged, changed, signed := d.Tiles, d.TilesChanged, d.Changed, d.Signed
	if c.Where != nil {
		if d.scopes == nil {
			return 0, fmt.Sprintf("%s does not lie over it", c.Where.By), nil
		}
		s := e.scopes[e.scopeOf[k]]
		tl := d.scopes[e.scopeOf[k]]
		tiles, tilesChanged, changed, signed = s.tiles(e.cuts[s.cut]), tl.tilesChanged, tl.changed, tl.signed()
	}
	switch c.Measure {
	case "changed":
		return float64(changed), "", nil
	case "tiles":
		return float64(tilesChanged), "", nil
	case "share":
		if tiles == 0 {
			return 0, "no tiles", nil
		}
		return float64(tilesChanged) / float64(tiles), "", nil
	case "code":
		if d.namesA == nil {
			return 0, "not a coded array", nil
		}
		from, err := lookup(d.namesA, string(*c.From))
		if err != nil {
			return 0, "", fmt.Errorf("%s in a: %w", c.Array, err)
		}
		to, err := lookup(d.namesB, string(*c.To))
		if err != nil {
			return 0, "", fmt.Errorf("%s in b: %w", c.Array, err)
		}
		n, ok := d.pairs[[2]int64{from, to}]
		if !ok && d.overflow > 0 {
			return 0, fmt.Sprintf("more than %d changes of code; this one was not counted", codePairs), nil
		}
		return float64(n), "", nil
	}
	if !d.numeric {
		return 0, "codes and bools have no signed change", nil
	}
	up, down := int64(0), int64(0)
	if signed != nil {
		up, down = signed.Up, signed.Down
	}
	switch c.Measure {
	case "up":
		return float64(up), "", nil
	case "down":
		return float64(down), "", nil
	}
	if signed == nil {
		return 0, "nothing measured changed", nil
	}
	switch c.Measure {
	case "mean":
		return signed.Mean, "", nil
	case "p5":
		return signed.P5, "", nil
	case "p50":
		return signed.P50, "", nil
	}
	return signed.P95, "", nil
}

// String is the check as a line: its array and where, measure and bounds.
func (c Check) String() string {
	var b strings.Builder
	if c.Name != "" {
		fmt.Fprintf(&b, "%s: ", c.Name)
	}
	b.WriteString(c.Array)
	if w := c.Where; w != nil {
		if len(w.Is) > 0 {
			fmt.Fprintf(&b, " where %s is %s", w.By, strings.Join(tokenStrings(w.Is), ","))
		} else {
			fmt.Fprintf(&b, " where %s is not %s", w.By, strings.Join(tokenStrings(w.Not), ","))
		}
	}
	b.WriteString(" " + c.Measure)
	if c.Measure == "code" {
		fmt.Fprintf(&b, " %s -> %s", *c.From, *c.To)
	}
	return b.String()
}

func (c Check) bounds() string {
	var s []string
	for _, x := range []struct {
		op string
		v  *float64
	}{{">=", c.Min}, {"<=", c.Max}, {">", c.Above}, {"<", c.Below}} {
		if x.v != nil {
			s = append(s, x.op+" "+number(*x.v))
		}
	}
	return strings.Join(s, ", ")
}
