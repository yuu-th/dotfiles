package planner

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/op"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// helper: build a minimal WorldState with two displays
func twoDisplayState(d0ActiveWs, d1ActiveWs w.WorkspaceID) w.WorldState {
	primary := w.DisplayID("d:0")
	return w.WorldState{
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{},
			Displays: w.ObservedDisplayState{
				Primary: &primary,
				Displays: map[w.DisplayID]w.ObservedDisplay{
					"d:0": {ID: "d:0", Connected: true, ActiveWorkspace: d0ActiveWs},
					"d:1": {ID: "d:1", Connected: true, ActiveWorkspace: d1ActiveWs},
				},
			},
		},
	}
}

func minimalDesired(sws []w.SystemWindow) w.DesiredWorld {
	return w.DesiredWorld{
		ActiveProfile: "p",
		Profiles:      map[w.ProfileID]w.DesiredProfile{"p": {ID: "p", Assignments: map[w.SlotID]w.ProjectID{}}},
		Projects:      map[w.ProjectID]w.DesiredProject{},
		SystemWindows: sws,
	}
}

func minimalEnv() w.ManagedEnvironment {
	return w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Viewer: "A",
			Workspaces: []w.WorkspaceSpec{
				{ID: "A", Role: w.WorkspaceViewer},
				// CP1 is the sole cockpit park workspace (requirements v2.4 §8.1).
				// CP2-CP6 removed.
				{ID: "CP1", RawName: "23"},
				{ID: "WS1", RawName: "1"},
				{ID: "WS2", RawName: "2"},
			},
		},
	}
}

// TestPlanner_Cockpit_SpawnWhenMissing verifies that the planner emits
// a spawn-cockpit op for the single desired cockpit SystemWindow when no
// matching live ghostty window exists (requirements v2.4 §8.1: 1 cockpit).
func TestPlanner_Cockpit_SpawnWhenMissing(t *testing.T) {
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Viewer:     "A",
			Workspaces: []w.WorkspaceSpec{{ID: "A", Role: w.WorkspaceViewer}},
		},
	}
	desired := minimalDesired([]w.SystemWindow{
		// Exactly one cockpit on the projwm-managed monitor (requirements v2.4 §8.1).
		{ID: w.SystemWindowID{Kind: w.WindowCockpit, Index: 0}, Kind: w.WindowCockpit, DisplayIdx: 0, Title: "projwm-cockpit-0", ParkWorkspace: "CP1", Visibility: w.CockpitHidden},
	})
	state := w.WorldState{Environment: env, Desired: desired, Observed: w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{}}}

	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	spawns := 0
	for _, o := range plan.Operations {
		if o.Kind == op.KindSpawnCockpit {
			spawns++
			if o.Target.SystemWindow == nil {
				t.Errorf("SpawnCockpit op missing SystemWindow target: %+v", o)
			}
		}
	}
	if spawns != 1 {
		t.Errorf("expected 1 spawn-cockpit op (single cockpit per v2.4 §8.1), got %d (plan ops: %+v)", spawns, plan.Operations)
	}
}

// TestPlanner_Cockpit_ShowEmitsWhenHidden verifies that when a cockpit window
// exists and Visibility=Shown but the display is not on the ParkWorkspace,
// the planner emits a show-cockpit op.
func TestPlanner_Cockpit_ShowEmitsWhenHidden(t *testing.T) {
	primary := w.DisplayID("d:0")
	state := w.WorldState{
		Environment: minimalEnv(),
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"cockpit-live-1": {
					ID:    "cockpit-live-1",
					App:   w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"},
					Title: w.ObservedTitle{Value: "projwm-cockpit-0"},
					Kind:  w.WindowCockpit,
				},
			},
			Displays: w.ObservedDisplayState{
				Primary: &primary,
				Displays: map[w.DisplayID]w.ObservedDisplay{
					"d:0": {ID: "d:0", Connected: true, ActiveWorkspace: "WS1"}, // not on CP1
				},
			},
		},
	}
	desired := minimalDesired([]w.SystemWindow{
		{
			ID:             w.SystemWindowID{Kind: w.WindowCockpit, Index: 0},
			Kind:           w.WindowCockpit,
			DisplayIdx:     0,
			Title:          "projwm-cockpit-0",
			ParkWorkspace:  "CP1",
			Visibility:     w.CockpitShown,
			PriorWorkspace: "WS1",
		},
	})
	state.Desired = desired

	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	shows := 0
	for _, o := range plan.Operations {
		if o.Kind == op.KindShowCockpit {
			shows++
			if o.Target.SystemWindow == nil {
				t.Errorf("show-cockpit op missing SystemWindow target")
			}
			if o.Target.Workspace == nil || *o.Target.Workspace != "CP1" {
				t.Errorf("show-cockpit op workspace = %v, want CP1", o.Target.Workspace)
			}
		}
	}
	if shows != 1 {
		t.Errorf("expected 1 show-cockpit op, got %d", shows)
	}
}

// TestPlanner_Cockpit_HideEmitsWhenShownAndPriorKnown verifies that when a
// cockpit window exists, Visibility=Hidden, and the display is on ParkWorkspace,
// the planner emits a hide-cockpit op with PriorWorkspace.
func TestPlanner_Cockpit_HideEmitsWhenShownAndPriorKnown(t *testing.T) {
	primary := w.DisplayID("d:0")
	state := w.WorldState{
		Environment: minimalEnv(),
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"cockpit-live-1": {
					ID:    "cockpit-live-1",
					App:   w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"},
					Title: w.ObservedTitle{Value: "projwm-cockpit-0"},
					Kind:  w.WindowCockpit,
				},
			},
			Displays: w.ObservedDisplayState{
				Primary: &primary,
				Displays: map[w.DisplayID]w.ObservedDisplay{
					"d:0": {ID: "d:0", Connected: true, ActiveWorkspace: "CP1"}, // currently on park ws
				},
			},
		},
	}
	desired := minimalDesired([]w.SystemWindow{
		{
			ID:             w.SystemWindowID{Kind: w.WindowCockpit, Index: 0},
			Kind:           w.WindowCockpit,
			DisplayIdx:     0,
			Title:          "projwm-cockpit-0",
			ParkWorkspace:  "CP1",
			Visibility:     w.CockpitHidden,
			PriorWorkspace: "WS1",
			PriorWindow:    "shell-live",
		},
	})
	state.Desired = desired

	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	hides := 0
	for _, o := range plan.Operations {
		if o.Kind == op.KindHideCockpit {
			hides++
			if o.Target.Workspace == nil || *o.Target.Workspace != "WS1" {
				t.Errorf("hide-cockpit workspace = %v, want WS1", o.Target.Workspace)
			}
			if o.Target.LiveWindow == nil || *o.Target.LiveWindow != "shell-live" {
				t.Errorf("hide-cockpit prior window = %v, want shell-live", o.Target.LiveWindow)
			}
		}
	}
	if hides != 1 {
		t.Errorf("expected 1 hide-cockpit op, got %d", hides)
	}
}

// TestPlanner_Cockpit_HideEmittedEvenWithEmptyPriorWorkspace verifies that
// when display is on ParkWorkspace and Visibility=Hidden, the planner emits
// a hide-cockpit op even if PriorWorkspace is empty. The executor's
// HideCockpitOnDisplay falls back to omniwm's per-display back-and-forth
// history in that case (post park-workspace redesign).
func TestPlanner_Cockpit_HideEmittedEvenWithEmptyPriorWorkspace(t *testing.T) {
	primary := w.DisplayID("d:0")
	state := w.WorldState{
		Environment: minimalEnv(),
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"cockpit-live-1": {
					ID:    "cockpit-live-1",
					App:   w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"},
					Title: w.ObservedTitle{Value: "projwm-cockpit-0"},
					Kind:  w.WindowCockpit,
				},
			},
			Displays: w.ObservedDisplayState{
				Primary: &primary,
				Displays: map[w.DisplayID]w.ObservedDisplay{
					"d:0": {ID: "d:0", Connected: true, ActiveWorkspace: "CP1"},
				},
			},
		},
	}
	desired := minimalDesired([]w.SystemWindow{
		{
			ID:             w.SystemWindowID{Kind: w.WindowCockpit, Index: 0},
			Kind:           w.WindowCockpit,
			DisplayIdx:     0,
			Title:          "projwm-cockpit-0",
			ParkWorkspace:  "CP1",
			Visibility:     w.CockpitHidden,
			PriorWorkspace: "", // empty — back-and-forth fallback expected
		},
	})
	state.Desired = desired

	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	hides := 0
	for _, o := range plan.Operations {
		if o.Kind == op.KindHideCockpit {
			hides++
			if o.Target.Workspace == nil || *o.Target.Workspace != "" {
				t.Errorf("expected hide-cockpit Target.Workspace to be empty (back-and-forth fallback), got %+v", o.Target)
			}
		}
	}
	if hides != 1 {
		t.Errorf("expected 1 hide-cockpit op even with empty PriorWorkspace, got %d", hides)
	}
}

// TestPlanner_Cockpit_NoOpWhenConverged verifies that no cockpit ops are emitted
// when the display is already on the correct workspace.
func TestPlanner_Cockpit_NoOpWhenConverged(t *testing.T) {
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Viewer:     "A",
			Workspaces: []w.WorkspaceSpec{{ID: "A", Role: w.WorkspaceViewer}},
		},
	}
	primary := w.DisplayID("d:0")
	desired := minimalDesired([]w.SystemWindow{
		{
			ID:             w.SystemWindowID{Kind: w.WindowCockpit, Index: 0},
			Kind:           w.WindowCockpit,
			DisplayIdx:     0,
			Title:          "projwm-cockpit-0",
			ParkWorkspace:  "CP1",
			Visibility:     w.CockpitHidden,
			PriorWorkspace: "WS1",
		},
	})
	state := w.WorldState{
		Environment: env,
		Desired:     desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"cockpit-live-1": {
					ID:    "cockpit-live-1",
					App:   w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"},
					Title: w.ObservedTitle{Value: "projwm-cockpit-0"},
					Kind:  w.WindowCockpit,
				},
			},
			Displays: w.ObservedDisplayState{
				Primary: &primary,
				Displays: map[w.DisplayID]w.ObservedDisplay{
					// display is on WS1 (not CP1), which matches Visibility=Hidden + prior=WS1
					"d:0": {ID: "d:0", Connected: true, ActiveWorkspace: "WS1"},
				},
			},
		},
	}

	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Operations {
		switch o.Kind {
		case op.KindSpawnCockpit, op.KindShowCockpit, op.KindHideCockpit:
			t.Errorf("expected no cockpit ops in converged state, got %s", o.Kind)
		}
	}
}

// TestPlanner_Cockpit_CloseLeftoverOnDisplayUnplug verifies that observed cockpit
// windows whose title is no longer in desired.SystemWindows get a close-cockpit op.
func TestPlanner_Cockpit_CloseLeftoverOnDisplayUnplug(t *testing.T) {
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Viewer:     "A",
			Workspaces: []w.WorkspaceSpec{{ID: "A", Role: w.WorkspaceViewer}},
		},
	}
	desired := minimalDesired([]w.SystemWindow{
		{ID: w.SystemWindowID{Kind: w.WindowCockpit, Index: 0}, Kind: w.WindowCockpit, DisplayIdx: 0, Title: "projwm-cockpit-0", ParkWorkspace: "CP1", Visibility: w.CockpitHidden},
	})
	primary := w.DisplayID("d:0")
	state := w.WorldState{
		Environment: env,
		Desired:     desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"cockpit-live-1": {ID: "cockpit-live-1", App: w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"},
					Title: w.ObservedTitle{Value: "projwm-cockpit-0"}, Kind: w.WindowCockpit},
				// D1 lingers but SystemWindows only has D0 → should be closed
				"cockpit-live-2": {ID: "cockpit-live-2", App: w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"},
					Title: w.ObservedTitle{Value: "projwm-cockpit-1"}, Kind: w.WindowCockpit},
			},
			Displays: w.ObservedDisplayState{
				Primary: &primary,
				Displays: map[w.DisplayID]w.ObservedDisplay{
					"d:0": {ID: "d:0", Connected: true, ActiveWorkspace: "WS1"},
				},
			},
		},
	}
	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	closed := 0
	for _, o := range plan.Operations {
		if o.Kind == op.KindCloseCockpit && o.Target.LiveWindow != nil && *o.Target.LiveWindow == "cockpit-live-2" {
			closed++
		}
	}
	if closed != 1 {
		t.Errorf("expected 1 close-cockpit op for cockpit-live-2, got %d", closed)
	}
}

// TestPlanner_Cockpit_DisplayMappingViaParkWorkspace is the regression test for
// the bug where display IDs are not contiguous and alphabetical sort maps
// DisplayIdx to the wrong physical display.
//
// Scenario (requirements v2.4 §8.1): exactly 1 cockpit on the projwm-managed
// monitor (CP1). Three physical displays exist (display:1=main/primary, display:5,
// display:2), but only display:1 hosts the cockpit (CP1 → display:1).
//
// Regression guard: WorkspaceToDisplay must be used (not alphabetical sort) to
// resolve CP1 → display:1 correctly when display IDs are non-contiguous.
//
// Verification: display:1 is NOT on CP1 (it is on WS1) and Visibility=Shown →
// planner should emit 1 show-cockpit op for CP1 on display:1.
// Other displays are unaffected (no cockpit assigned to them per v2.4).
func TestPlanner_Cockpit_DisplayMappingViaParkWorkspace(t *testing.T) {
	// 3 physical displays with non-contiguous IDs.
	// Only display:1 (primary) owns the single CP1 cockpit workspace.
	primary := w.DisplayID("display:1")
	state := w.WorldState{
		Environment: w.ManagedEnvironment{
			SchemaVersion: 1,
			Authority:     "nix",
			Workspaces: w.WorkspaceEnvironment{
				Viewer: "A",
				Workspaces: []w.WorkspaceSpec{
					{ID: "A", Role: w.WorkspaceViewer},
					// CP1 only — requirements v2.4 §8.1: single cockpit.
					{ID: "CP1"},
					{ID: "WS1"}, {ID: "WS2"}, {ID: "WS3"},
				},
			},
		},
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				// Only D0 cockpit exists (single cockpit per v2.4 §8.1).
				"ck-d0": {ID: "ck-d0", Kind: w.WindowCockpit, Title: w.ObservedTitle{Value: "projwm-cockpit-0"}},
			},
			Displays: w.ObservedDisplayState{
				Primary: &primary,
				Displays: map[w.DisplayID]w.ObservedDisplay{
					// display:1 is NOT on CP1 yet → show-cockpit op expected.
					"display:1": {ID: "display:1", Connected: true, ActiveWorkspace: "WS1"},
					// display:5 and display:2 have no cockpit (v2.4: other monitors unaffected).
					"display:5": {ID: "display:5", Connected: true, ActiveWorkspace: "WS2"},
					"display:2": {ID: "display:2", Connected: true, ActiveWorkspace: "WS3"},
				},
				// WorkspaceToDisplay populated by sigwm.Observe.
				// CP1 is owned by display:1 — the regression guard ensures the planner
				// uses this map rather than alphabetical sort of display IDs.
				WorkspaceToDisplay: map[w.WorkspaceID]w.DisplayID{
					"CP1": "display:1", // KEY: correct park workspace mapping
					"WS1": "display:1",
					"WS2": "display:5",
					"WS3": "display:2",
				},
			},
		},
	}
	// Single cockpit desired shown (requirements v2.4 §8.1 + §8.3).
	desired := minimalDesired([]w.SystemWindow{
		{
			ID:             w.SystemWindowID{Kind: w.WindowCockpit, Index: 0},
			Kind:           w.WindowCockpit,
			DisplayIdx:     0,
			Title:          "projwm-cockpit-0",
			ParkWorkspace:  "CP1",
			Visibility:     w.CockpitShown,
			PriorWorkspace: "WS1",
		},
	})
	state.Desired = desired

	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}

	// Expected: exactly 1 show-cockpit op for CP1 on display:1.
	// No ops for display:5 or display:2 (no cockpit on those monitors).
	var showOps []op.Operation
	for _, o := range plan.Operations {
		if o.Kind == op.KindShowCockpit {
			showOps = append(showOps, o)
		}
	}
	if len(showOps) != 1 {
		t.Fatalf("expected 1 show-cockpit op (for CP1/display:1), got %d: %+v", len(showOps), showOps)
	}
	// Verify the op targets the correct park workspace (CP1).
	if showOps[0].Target.Workspace == nil || *showOps[0].Target.Workspace != "CP1" {
		t.Errorf("show-cockpit workspace = %v, want CP1", showOps[0].Target.Workspace)
	}
}
