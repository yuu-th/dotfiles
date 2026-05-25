package planner

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/op"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestSSOTMissingActiveWindowPlansSpawnBeforePlacement(t *testing.T) {
	dwid := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowShell, Index: 1}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "dotfiles"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"dotfiles": {ID: "dotfiles", Windows: []w.DesiredWindow{{
				ID:   dwid,
				Kind: w.WindowShell,
				App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
				TitleContract: w.TitleContract{
					Authority: w.TitleControllerOwned,
					Expected:  "shell-1:dotfiles",
				},
			}}},
		},
	}
	plan, err := Plan(w.WorldState{
		Environment: ssotEnv(),
		Desired:     desired,
		Observed:    w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{}, Layouts: map[w.WorkspaceID]w.ObservedLayout{}},
	}, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !containsKind(plan.Operations, op.KindSpawnTerminal) {
		t.Fatalf("missing active shell must emit spawn-terminal: %+v", plan.Operations)
	}
	if containsKind(plan.Operations, op.KindMoveWindowToWorkspace) {
		t.Fatalf("missing active shell must not emit move before identity exists: %+v", plan.Operations)
	}
}

func TestSSOTDriftedActiveWindowPlansMoveNotRespawn(t *testing.T) {
	dwid := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowShell, Index: 1}
	desiredWindow := w.DesiredWindow{
		ID:   dwid,
		Kind: w.WindowShell,
		App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
		TitleContract: w.TitleContract{
			Authority: w.TitleControllerOwned,
			Expected:  "shell-1:dotfiles",
		},
	}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "dotfiles"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"dotfiles": {ID: "dotfiles", Windows: []w.DesiredWindow{desiredWindow}},
		},
	}
	plan, err := Plan(w.WorldState{
		Environment: ssotEnv(),
		Desired:     desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"live-shell": {
					ID:        "live-shell",
					Kind:      w.WindowShell,
					App:       w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"},
					Title:     w.ObservedTitle{Value: "shell-1:dotfiles"},
					Workspace: "9",
				},
			},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "intent:reconcile", op.ReasonReconcile)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !containsKind(plan.Operations, op.KindMoveWindowToWorkspace) {
		t.Fatalf("drifted active shell must emit move-window-to-workspace: %+v", plan.Operations)
	}
	if containsKind(plan.Operations, op.KindSpawnTerminal) {
		t.Fatalf("drifted active shell must reuse identity, not respawn: %+v", plan.Operations)
	}
}

func TestSSOTExternalWindowsNeverBecomeTierOneManagedCards(t *testing.T) {
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{},
	}
	plan, err := Plan(w.WorldState{
		Environment: ssotEnv(),
		Desired:     desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"external-managed-ws": {
					ID:        "external-managed-ws",
					Kind:      w.WindowExternal,
					Workspace: "Q",
					App:       w.ObservedAppRef{BundleID: "com.apple.Calculator"},
				},
				"external-general-ws": {
					ID:        "external-general-ws",
					Kind:      w.WindowExternal,
					Workspace: "9",
					App:       w.ObservedAppRef{BundleID: "com.apple.TextEdit"},
				},
			},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "intent:reconcile", op.ReasonReconcile)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if containsLiveTarget(plan.Operations, "external-managed-ws") || containsLiveTarget(plan.Operations, "external-general-ws") {
		t.Fatalf("SSOT INV-11 forbids Tier 1 operations/cards for external windows: %+v", plan.Operations)
	}
}

func ssotEnv() w.ManagedEnvironment {
	return w.ManagedEnvironment{
		WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"},
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{
				{ID: "Q", RawName: "Q", Role: w.WorkspaceProject},
				{ID: "9", RawName: "9", Role: w.WorkspaceGeneral},
			},
			Slots: []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
		},
	}
}

func containsKind(ops []op.Operation, kind op.Kind) bool {
	for _, oper := range ops {
		if oper.Kind == kind {
			return true
		}
	}
	return false
}

func containsLiveTarget(ops []op.Operation, id w.LiveWindowID) bool {
	for _, oper := range ops {
		if oper.Target.LiveWindow != nil && *oper.Target.LiveWindow == id {
			return true
		}
	}
	return false
}
