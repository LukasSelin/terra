package terra

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// A world written down as a hash, so that a change which is meant to leave
// every world exactly as it was can prove it in one command, and one that is
// meant to move them says so by rewriting the file.
//
//	TERRA_DIGEST=write go test -run TestWorldDigest .
//	TERRA_DIGEST=check go test -run TestWorldDigest .
//
// The worlds are the budgeted ones - a drawn valley, a valley run through its
// history, and a globe at an eighth of the preset's width - made over
// budgetWorkers goroutines, though the hash does not depend on that: see
// TestMakingAWorldDoesNotDependOnTheGoroutines. The hash is digest's, which
// reads every tile, every bed under it, the rain and the wind. Without
// TERRA_DIGEST the test is skipped, so the suite does not fail on a change
// that meant to move the world and had not yet said so here.
//
// A change that moves the world commits the rewritten file with the budget
// and the work-log entry, so that a reviewer sees that it did. See
// docs/perf/README.md.
const digestFile = "docs/perf/digest.json"

func TestWorldDigest(t *testing.T) {
	mode := os.Getenv("TERRA_DIGEST")
	if mode == "" {
		t.Skip("set TERRA_DIGEST=check or TERRA_DIGEST=write")
	}
	if mode != "check" && mode != "write" {
		t.Fatalf("TERRA_DIGEST=%q: want check or write", mode)
	}
	was := Workers
	Workers = budgetWorkers
	defer func() { Workers = was }()

	got := map[string]string{}
	for _, w := range budgetWorlds {
		got[w.name] = digest(NewLand(1, w.terms()))
	}
	if mode == "write" {
		out, err := json.MarshalIndent(got, "", "\t")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(digestFile, append(out, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s: %v", digestFile, got)
		return
	}
	raw, err := os.ReadFile(digestFile)
	if err != nil {
		t.Fatalf("no digest to check against: %v (TERRA_DIGEST=write makes one)", err)
	}
	var want map[string]string
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	for name, h := range got {
		switch w, ok := want[name]; {
		case !ok:
			t.Errorf("%s: not in %s", name, digestFile)
		case w != h:
			t.Errorf("%s: the world is %s, and %s says %s", name, h, digestFile, w)
		}
	}
	if !t.Failed() {
		t.Log(fmt.Sprintf("every world is as %s says", digestFile))
	}
}
