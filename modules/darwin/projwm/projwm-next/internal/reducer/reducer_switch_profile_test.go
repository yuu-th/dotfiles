package reducer

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/event"
	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// switchProfileState is a self-contained state where two profiles assign
// the same slot to different projects. The reducer is purely
// DesiredWorld-shaped; planner-side observable side effects (close /
// spawn / viewer restore / URL restore / tmux retention / final focus)
// are verified by the planner package's own tests.
func switchProfileState() w.WorldState {
	return w.WorldState{
		Environment: w.ManagedEnvironment{
			SchemaVersion: 1,
			Authority:     "nix",
			Workspaces: w.WorkspaceEnvironment{
				Viewer: "A",
				Workspaces: []w.WorkspaceSpec{
					{ID: "A", Role: w.WorkspaceViewer},
					{ID: "Q", Role: w.WorkspaceProject},
				},
				Slots: []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
			},
		},
		Desired: w.DesiredWorld{
			ActiveProfile: "work",
			Profiles: map[w.ProfileID]w.DesiredProfile{
				"work":    {ID: "work", InactivePolicy: w.InactivePolicyRemove, Assignments: map[w.SlotID]w.ProjectID{"Q": "dotfiles"}},
				"spike":   {ID: "spike", InactivePolicy: w.InactivePolicyKeep, Assignments: map[w.SlotID]w.ProjectID{"Q": "exp1"}},
			},
			Projects: map[w.ProjectID]w.DesiredProject{
				"dotfiles": {ID: "dotfiles"},
				"exp1":     {ID: "exp1"},
			},
		},
	}
}

func TestReduceIntent_SwitchProfile_FlipsActive(t *testing.T) {
	s := switchProfileState()
	d, err := ReduceIntent(s, intent.SwitchProfile{To: "spike"})
	if err != nil {
		t.Fatal(err)
	}
	if d.ActiveProfile != "spike" {
		t.Errorf("ActiveProfile = %q, want spike", d.ActiveProfile)
	}
}

func TestReduceIntent_SwitchProfile_PreservesBothAssignments(t *testing.T) {
	// Per requirements §6.3, SwitchProfile changes which profile is
	// active but never modifies either profile's Assignments. Removal /
	// spawn of windows is the planner's job downstream.
	s := switchProfileState()
	d, err := ReduceIntent(s, intent.SwitchProfile{To: "spike"})
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Profiles["work"].Assignments["Q"]; got != "dotfiles" {
		t.Errorf("work profile Q = %q, want dotfiles (unchanged)", got)
	}
	if got := d.Profiles["spike"].Assignments["Q"]; got != "exp1" {
		t.Errorf("spike profile Q = %q, want exp1 (unchanged)", got)
	}
}

func TestReduceIntent_SwitchProfile_UnknownRejected(t *testing.T) {
	s := switchProfileState()
	if _, err := ReduceIntent(s, intent.SwitchProfile{To: "no-such"}); err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

// Tier 4 (T4.2): a managed window moved to managed→unmanaged or
// unmanaged→managed should produce a [MOVED] card emit + DirtyScope
// regardless of the source/destination categorization. The planner-side
// revert is tested elsewhere; here we lock the reducer contract.
func TestReactToEvent_Tier4_ManagedToUnmanaged(t *testing.T) {
	s := switchProfileState()
	win := w.LiveWindowID("L1")
	from := w.WorkspaceID("Q")
	to := w.WorkspaceID("1")
	r, err := ReactToEvent(s, event.Event{
		Kind: event.KindUserMovedWindow,
		Data: event.Data{Window: &win, Workspace: &from, TargetWS: &to},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.NewCards) != 1 || r.NewCards[0].Type != w.CardTypeMoved {
		t.Errorf("expected MOVED card, got %+v", r.NewCards)
	}
	if r.NewCards[0].Context["from"] != "Q" || r.NewCards[0].Context["to"] != "1" {
		t.Errorf("MOVED card context = %+v", r.NewCards[0].Context)
	}
}

func TestReactToEvent_Tier4_UnmanagedToManaged(t *testing.T) {
	s := switchProfileState()
	win := w.LiveWindowID("L1")
	from := w.WorkspaceID("1")
	to := w.WorkspaceID("Q")
	r, err := ReactToEvent(s, event.Event{
		Kind: event.KindUserMovedWindow,
		Data: event.Data{Window: &win, Workspace: &from, TargetWS: &to},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.NewCards) != 1 || r.NewCards[0].Type != w.CardTypeMoved {
		t.Errorf("expected MOVED card, got %+v", r.NewCards)
	}
	if r.NewCards[0].Context["from"] != "1" || r.NewCards[0].Context["to"] != "Q" {
		t.Errorf("MOVED card context = %+v", r.NewCards[0].Context)
	}
}

// A3.2 / A3.3: Zed orphan on managed ws should produce a [NEW] card.
// The actual project-path heuristic lives in the controller's
// PromoteOrphans which we test here for Zed bundleID.
func TestReactToEvent_Tier1_ZedOrphan_HasBundleIDInCard(t *testing.T) {
	s := switchProfileState()
	s.Observed.Windows = map[w.LiveWindowID]w.ObservedWindow{
		"zed-1": {
			ID:        "zed-1",
			Workspace: "Q",
			Kind:      w.WindowEditor,
			App:       w.ObservedAppRef{BundleID: "dev.zed.Zed"},
			Title:     w.ObservedTitle{Value: "myproj"},
		},
	}
	r, err := ReactToEvent(s, event.Event{Kind: event.KindWindowsChanged})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.OrphanAdds) != 1 {
		t.Fatalf("expected 1 OrphanAdd, got %d", len(r.OrphanAdds))
	}
	oc := r.OrphanAdds[0]
	if oc.BundleID != "dev.zed.Zed" {
		t.Errorf("BundleID = %q", oc.BundleID)
	}
	if oc.Title != "myproj" {
		t.Errorf("Title = %q", oc.Title)
	}
}

// SSOT N-12 (2026-05-20): AcceptManualLayout intent deleted in favor of
// AutoSyncLayout (Tier 2 auto-overwrite). The legacy
// TestReduceIntent_AcceptManualLayoutStillWorks_Legacy test is removed;
// AutoSyncLayout coverage is in reducer_tier_test.go.
