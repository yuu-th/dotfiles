//go:build real_ops

package wm

import (
	"testing"
	"time"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// TestReorderColumnsWhileWorkspaceUnfocused is coverage for the focus-before-
// observe contract: ReorderColumns must focus the target workspace before it
// observes the column order or issues column moves. On OmniWM/niri the column
// ordering/frame.x of a non-active workspace can be stale, so observing or
// moving an unfocused workspace is the root of "order did not settle" failures.
//
// Spawns 5 shells on ws8 (one column stacked in the target layout), switches
// focus away to ws9, then reorders ws8 to an S10-shaped layout (non-trivial
// permutation + a stack). The reorder must still settle to the requested
// layout despite ws8 being inactive at entry.
func TestReorderColumnsWhileWorkspaceUnfocused(t *testing.T) {
	ctx, cancel := realSpecContext(t, 180*time.Second)
	defer cancel()
	realSpecRequireGhostty(t)
	sw := newRealSigWM()

	type win struct{ title, session string }
	defs := []win{
		{realSpecTitle(t, "shell", 1, "reorder-unfocus-a"), realSpecSession(t, "reorder-unfocus-a")},
		{realSpecTitle(t, "shell", 2, "reorder-unfocus-b"), realSpecSession(t, "reorder-unfocus-b")},
		{realSpecTitle(t, "shell", 3, "reorder-unfocus-c"), realSpecSession(t, "reorder-unfocus-c")},
		{realSpecTitle(t, "shell", 4, "reorder-unfocus-d"), realSpecSession(t, "reorder-unfocus-d")},
		{realSpecTitle(t, "shell", 5, "reorder-unfocus-e"), realSpecSession(t, "reorder-unfocus-e")},
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

	// S10-shaped target: a non-trivial column permutation that ALSO contains a
	// stacked column (initial[1] over initial[2] in one column). Flattened this
	// is [e, b, c, a, d]; the stack makes the column-ordering phase and the
	// re-stack phase interact, which is the condition the solo-column R1-R4
	// tests never exercise.
	want := [][]w.LiveWindowID{
		{initial[4]},
		{initial[1], initial[2]},
		{initial[0]},
		{initial[3]},
	}

	// Move focus AWAY from ws8 so its frame.x readings are stale at the moment
	// ReorderColumns takes its first observation.
	if err := sw.FocusWorkspace(ctx, "9"); err != nil {
		t.Fatalf("FocusWorkspace 9 (move focus off ws8): %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	if err := sw.ReorderColumns(ctx, "8", want); err != nil {
		t.Fatalf("ReorderColumns (ws8 unfocused at entry): %v", err)
	}
	realSpecAssertColumns(t, ctx, sw, "8", want)
}
