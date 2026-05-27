package controller

import (
	"context"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// TestSummonShell_TwoWindows_ConvergesWithoutReplanLoop is the regression
// owner for the summon/jump/cycle replan-loop bug (SSOT §4.1 OP01 + §7.1).
//
// summon-shell decides "am I already on a shell of this slot → cycle to the
// next one". That decision must be anchored to the focus as observed when the
// user pressed the key, NOT to the live focus the converge loop keeps re-
// observing. With 2+ candidate windows, reading live focus made every replan
// re-cycle off the focus the previous iteration just set (shell-1 → shell-2 →
// shell-1 → …), so the planner never emitted 0 ops and the transaction died
// via the §7.1 max-replans path (surfacing to the CLI as a spurious failure).
//
// The fix freezes the transaction-start focus in Meta.SummonFocusAnchor; the
// cycle reads that, so the target stays fixed and the transaction converges.
// This test fails (max-replans error) without the anchor and passes with it.
func TestSummonShell_TwoWindows_ConvergesWithoutReplanLoop(t *testing.T) {
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Slots: []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
			Workspaces: []w.WorkspaceSpec{
				{ID: "Q", RawName: "Q", DisplayName: "Q", Role: w.WorkspaceProject},
			},
		},
	}
	shell1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	shell2 := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 2}
	desired := w.DesiredWorld{
		ActiveProfile: "default",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"default": {ID: "default", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{
				{ID: shell1, Kind: w.WindowShell},
				{ID: shell2, Kind: w.WindowShell},
			}},
		},
	}
	st := store.NewMemoryStore(desired)
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, st)

	// Spawn the two shells so both candidates exist + are matched.
	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (spawn shells): %v", err)
	}

	// summon-shell must converge. Pre-fix this hit max-replans because the
	// cycle alternated shell-1/shell-2 on every replan.
	if _, err := ctrl.ApplyIntent(context.Background(), intent.SummonShell{Slot: "Q"}); err != nil {
		t.Fatalf("summon-shell must converge for a slot with 2 shells, got: %v", err)
	}

	// Repeated summon is idempotent (SSOT §9.1 S8): still converges, no error.
	if _, err := ctrl.ApplyIntent(context.Background(), intent.SummonShell{Slot: "Q"}); err != nil {
		t.Fatalf("repeated summon-shell must stay convergent, got: %v", err)
	}

	// The focused window is one of the slot's shells.
	obs, err := fake.Observe(context.Background())
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	focused := obs.Focus.Window
	fw, ok := obs.Windows[focused]
	if !ok || fw.MatchedTo == nil || fw.MatchedTo.Project != "p1" || fw.MatchedTo.Kind != w.WindowShell {
		t.Fatalf("after summon-shell the focused window must be a p1 shell, got focus=%q window=%+v", focused, fw)
	}
}
