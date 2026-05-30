package controller

import (
	"context"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// attrDesiredWithTwoEditors builds a desired world with project p1 containing
// TWO same-title Zed editors (editor-1, editor-2), both with the Zed basename
// title "p1", assigned to slot Q. This is the precondition for the
// RemoveWindow repro: two indistinguishable-by-title live windows, of which
// one is removed from the desired set.
func attrDesiredWithTwoEditors() w.DesiredWorld {
	mk := func(idx int) w.DesiredWindow {
		return w.DesiredWindow{
			ID:   w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: idx},
			Kind: w.WindowEditor,
			App:  w.AppRequirement{BundleID: "dev.zed.Zed"},
			TitleContract: w.TitleContract{
				Authority: w.TitleAppOwned,
				Expected:  "p1",
				Drift:     w.TitleDriftRematch,
			},
			MatchHints: []w.MatchHint{
				{Kind: w.MatchByBundleID, Pattern: "dev.zed.Zed", Confidence: w.MatchStrong},
			},
		}
	}
	return w.DesiredWorld{
		ActiveProfile: "default",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"default": {ID: "default", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{mk(1), mk(2)}},
		},
	}
}

// TestRemoveWindow_ClosesRemovedWindow is the deterministic reproduction of the
// confirmed bug: project p1 has TWO same-title Zed editors reconciled (2 live
// windows). intent.RemoveWindow drops editor-2 from the desired set; the next
// reconcile MUST close the orphaned live window so exactly ONE Zed window
// remains. RED before the fix (the removed window was never closed — the close
// op was either not emitted or rejected by PreUniqueStrong), GREEN after.
func TestRemoveWindow_ClosesRemovedWindow(t *testing.T) {
	env := attrEnv()
	desired := attrDesiredWithTwoEditors()
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))

	// Reconcile: spawn both editors into slot Q.
	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (spawn two editors): %v", err)
	}
	obs, _ := fake.Observe(context.Background())
	if n := attrCountEditorsInSlot(obs, "Q"); n != 2 {
		t.Fatalf("precondition failed: %d Zed windows on slot Q after reconcile, want exactly 2", n)
	}

	// Find the live window owning editor-2 so we can assert it is the one closed.
	editor2 := w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 2}
	var editor2Live w.LiveWindowID
	for id, win := range obs.Windows {
		if win.MatchedTo != nil && *win.MatchedTo == editor2 {
			editor2Live = id
			break
		}
	}
	if editor2Live == "" {
		t.Fatalf("precondition failed: could not find live window matched to editor-2")
	}

	// Remove editor-2 from the desired set. The reducer drops the desired
	// window; the reconcile inside ApplyIntent must converge by closing the
	// now-orphaned live window.
	if _, err := ctrl.ApplyIntent(context.Background(), intent.RemoveWindow{Project: "p1", WindowID: editor2}); err != nil {
		t.Fatalf("remove-window editor-2: %v", err)
	}

	obs2, _ := fake.Observe(context.Background())
	// THE BUG: previously this stayed at 2 (the removed window was never closed).
	if n := attrCountEditorsInSlot(obs2, "Q"); n != 1 {
		t.Fatalf("TestRemoveWindow_ClosesRemovedWindow: %d Zed windows on slot Q after removing editor-2, want exactly 1 (the removed window must be closed)", n)
	}
	// The window that survives must NOT be editor-2's old live window.
	if _, stillThere := obs2.Windows[editor2Live]; stillThere {
		t.Fatalf("TestRemoveWindow_ClosesRemovedWindow: editor-2's live window %q is still present after removal (must be closed)", editor2Live)
	}
}
