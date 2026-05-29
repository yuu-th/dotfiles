//go:build real_ops

package wm

import (
	"testing"
	"time"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// TestReorderColumnsToleratesStrayWindow verifies the principle that reorder
// operates on the MANAGED set only and is transparent to a non-managed window
// interleaved on the workspace (e.g. a Zed "empty project" window or a user
// window that drifted in). want lists only 3 of the 4 live windows; the 4th is
// a "stray" that ReorderColumns is not told about. The 3 managed windows must
// still settle into the requested relative order regardless of where the stray
// ends up.
//
// This is the declarative principle (§6.2 identity, INV-01): unexpected windows
// must not break managed-window convergence.
func TestReorderColumnsToleratesStrayWindow(t *testing.T) {
	ctx, cancel := realSpecContext(t, 180*time.Second)
	defer cancel()
	realSpecRequireGhostty(t)
	sw := newRealSigWM()

	type win struct{ title, session string }
	defs := []win{
		{realSpecTitle(t, "shell", 1, "reorder-stray-a"), realSpecSession(t, "reorder-stray-a")},
		{realSpecTitle(t, "shell", 2, "reorder-stray-b"), realSpecSession(t, "reorder-stray-b")},
		{realSpecTitle(t, "shell", 3, "reorder-stray-c"), realSpecSession(t, "reorder-stray-c")},
		{realSpecTitle(t, "shell", 4, "reorder-stray-x"), realSpecSession(t, "reorder-stray-x")},
	}
	for _, d := range defs {
		realSpecCleanupGhostty(t, sw, d.title, d.session)
	}
	ids := make([]w.LiveWindowID, 0, len(defs))
	for _, d := range defs {
		id := realSpecSpawnGhostty(t, ctx, sw, w.WindowShell, "8", d.title, d.session, "")
		realSpecAssertObserved(t, ctx, sw, id, "8", "com.mitchellh.ghostty", d.title)
		ids = append(ids, id)
	}

	initial := realSpecObservedOrder(t, ctx, sw, "8", ids...)
	if len(initial) != len(defs) {
		t.Fatalf("setup order = %v, want %d windows", initial, len(defs))
	}

	// Treat the window currently in the MIDDLE as the stray (worst case: it sits
	// between managed windows). Reorder the other three into reversed relative
	// order; the stray is intentionally omitted from want.
	stray := initial[1]
	managed := []w.LiveWindowID{}
	for _, id := range initial {
		if id != stray {
			managed = append(managed, id)
		}
	}
	// reverse managed relative order
	want := [][]w.LiveWindowID{
		{managed[2]}, {managed[1]}, {managed[0]},
	}

	if err := sw.ReorderColumns(ctx, "8", want); err != nil {
		t.Fatalf("ReorderColumns with stray window present: %v", err)
	}
	// The 3 managed windows must be in the requested relative order; the stray
	// may sit anywhere among them.
	realSpecAssertColumns(t, ctx, sw, "8", want)
}
