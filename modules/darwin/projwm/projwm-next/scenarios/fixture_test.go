// Package scenarios contains the 8 acceptance scenarios from specs.md §3.
// Each scenario runs on both the fake and the simulator backends.
package scenarios

import (
	w "github.com/yuu-th/projwm-next/internal/world"
)

// makeFixture returns a deterministic ManagedEnvironment + initial DesiredWorld:
//   - 2 slots: "Q" (workspace "ws-q"), "W" (workspace "ws-w") — both role=project
//   - 1 viewer workspace: "ws-viewer"
//   - 2 profiles: "A" (Q→p1, W→p2) and "B" (Q→p3) and empty profile "E"
//   - 4 projects: p1, p2, p3, p4 (p4 archived)
//     each project has 1 ai window + 1 editor window
//   - FocusPolicy maps every command key to ws-viewer
func makeFixture() (w.ManagedEnvironment, w.DesiredWorld) {
	env := w.ManagedEnvironment{
		SchemaVersion:    1,
		Authority:        "nix",
		Source:           "test-fixture",
		MinDaemonVersion: "0.0.1",
		WindowManager: w.WindowManagerEnvironment{
			Backend: "fake",
			Layout:  w.LayoutTuning{MaxVisibleColumns: 4, MaxWindowsPerColumn: 2},
		},
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{
				{ID: "ws-q", Role: w.WorkspaceProject},
				{ID: "ws-w", Role: w.WorkspaceProject},
				{ID: "ws-viewer", Role: w.WorkspaceViewer},
				{ID: "ws-other", Role: w.WorkspaceGeneral},
			},
			Slots: []w.SlotSpec{
				{ID: "Q", Workspace: "ws-q", Order: 1},
				{ID: "W", Workspace: "ws-w", Order: 2},
			},
			Viewer: "ws-viewer",
		},
		Apps: w.AppEnvironment{
			ManagedApps: []w.ManagedAppPolicy{
				{Capability: w.CapabilityTerminal, BundleID: "com.example.term"},
				{Capability: w.CapabilityEditor, BundleID: "com.example.editor"},
			},
		},
	}

	mkProject := func(id w.ProjectID, archived bool) w.DesiredProject {
		ai := w.DesiredWindow{
			ID:   w.DesiredWindowID{Project: id, Kind: w.WindowAI, Index: 1},
			Kind: w.WindowAI,
			App:  w.AppRequirement{Capability: w.CapabilityTerminal, BundleID: "com.example.term"},
			TitleContract: w.TitleContract{
				Authority: w.TitleControllerOwned,
				Expected:  "ai-1:" + string(id),
				Drift:     w.TitleDriftRepair,
			},
		}
		ed := w.DesiredWindow{
			ID:   w.DesiredWindowID{Project: id, Kind: w.WindowEditor, Index: 1},
			Kind: w.WindowEditor,
			App:  w.AppRequirement{Capability: w.CapabilityEditor, BundleID: "com.example.editor"},
			TitleContract: w.TitleContract{
				Authority: w.TitleControllerOwned,
				Expected:  string(id),
				Drift:     w.TitleDriftRepair,
			},
		}
		return w.DesiredProject{
			ID:       id,
			Archived: archived,
			Windows:  []w.DesiredWindow{ai, ed},
			Layouts:  map[w.WorkspaceID]w.DesiredLayout{},
		}
	}

	desired := w.DesiredWorld{
		ActiveProfile: "A",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"A": {ID: "A", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1", "W": "p2"}, InactivePolicy: w.InactivePolicyRemove},
			"B": {ID: "B", Assignments: map[w.SlotID]w.ProjectID{"Q": "p3"}, InactivePolicy: w.InactivePolicyRemove},
			"E": {ID: "E", Assignments: map[w.SlotID]w.ProjectID{}, InactivePolicy: w.InactivePolicyRemove},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": mkProject("p1", false),
			"p2": mkProject("p2", false),
			"p3": mkProject("p3", false),
			"p4": mkProject("p4", true),
		},
		FocusPolicy: w.FocusPolicySet{
			FinalFocus: map[string]w.WorkspaceID{
				"intent:switch-profile":         "ws-viewer",
				"intent:archive-project":        "ws-viewer",
				"intent:unarchive-project":      "ws-viewer",
				"intent:assign-project":         "ws-viewer",
				"intent:unassign-slot":          "ws-viewer",
				"intent:reconcile":              "ws-viewer",
				"intent:accept-manual-layout":   "",
				"intent:validate-environment":   "ws-viewer",
				"lifecycle:bootstrap":           "ws-viewer",
				"lifecycle:wake-recovery":       "ws-viewer",
				"lifecycle:display-reconfigure": "ws-viewer",
				"lifecycle:full-reconcile":      "ws-viewer",
				"event:external":                "ws-viewer",
			},
		},
	}
	return env, desired
}
