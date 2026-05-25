package planner

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/op"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §4.1 OP13 remove-window: reducer が DesiredWindow を削除した結果、
// planner が観測されている残存 live window に対して KindCloseWindow か
// KindKillSession op を emit することを verify する。

// helper: env with shell lifecycle removal allowed (kill-session)
func removeWindowEnv() w.ManagedEnvironment {
	return w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		WindowManager: w.WindowManagerEnvironment{
			Backend: "omniwm", // omniwm backend は close-window をブロック → kill-session 経路に流れる
		},
		Workspaces: w.WorkspaceEnvironment{
			Viewer:     "A",
			Workspaces: []w.WorkspaceSpec{{ID: "A", Role: w.WorkspaceViewer}, {ID: "WS1", RawName: "1"}},
			Slots:      []w.SlotSpec{{ID: "Q", Workspace: "WS1", Order: 1}},
		},
		Apps: w.AppEnvironment{
			ManagedApps: []w.ManagedAppPolicy{{
				Capability: w.CapabilityTerminal,
				BundleID:   "com.mitchellh.ghostty",
				AppPath:    "/Applications/Ghostty.app",
				LifecycleRemoval: w.LifecycleRemovalPolicy{
					Allowed:      true,
					Method:       w.LifecycleRemovalAXCloseGuarded,
					AllowedKinds: []w.WindowKind{w.WindowAI, w.WindowShell, w.WindowViewer},
				},
			}},
		},
	}
}

// TestPlanner_RemoveWindow_EmitsCloseOpForRemovedShell — DesiredWorld から
// shell-2 を削除した後、observed の shell-2 live window に対して removal op
// (KillSession か CloseWindow) が emit される。
func TestPlanner_RemoveWindow_EmitsCloseOpForRemovedShell(t *testing.T) {
	env := removeWindowEnv()
	shell1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	// shell-2 was previously desired, but the user just removed it.
	desired := w.DesiredWorld{
		ActiveProfile: "prof",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"prof": {ID: "prof", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{
				{ID: shell1, Kind: w.WindowShell, App: w.AppRequirement{BundleID: "com.mitchellh.ghostty"}},
			}},
		},
	}
	shell2 := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 2}
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"shell-1-p1": {ID: "shell-1-p1", Kind: w.WindowShell, MatchedTo: &shell1,
					App: w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"}, Workspace: "WS1",
					Title: w.ObservedTitle{Value: "shell-1:p1"}},
				"shell-2-p1": {ID: "shell-2-p1", Kind: w.WindowShell, MatchedTo: &shell2,
					App: w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"}, Workspace: "WS1",
					Title: w.ObservedTitle{Value: "shell-2:p1"}},
			},
		},
	}
	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range plan.Operations {
		if (o.Kind == op.KindKillSession || o.Kind == op.KindCloseWindow) &&
			o.Target.LiveWindow != nil && *o.Target.LiveWindow == "shell-2-p1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected removal op for shell-2-p1 (removed from desired), plan: %+v", plan.Operations)
	}
	// shell-1 should NOT be removed (still desired)
	for _, o := range plan.Operations {
		if (o.Kind == op.KindKillSession || o.Kind == op.KindCloseWindow) &&
			o.Target.LiveWindow != nil && *o.Target.LiveWindow == "shell-1-p1" {
			t.Errorf("removal op emitted for shell-1-p1 which is still desired: %+v", o)
		}
	}
}

// TestPlanner_RemoveWindow_LastWindowKeepsProject — 最後の window を削除して
// project.Windows が空になっても、project は DesiredWorld に残り、observed
// に存在する windows は全て removal target になる (SSOT 631 デフォルト)。
func TestPlanner_RemoveWindow_LastWindowKeepsProject(t *testing.T) {
	env := removeWindowEnv()
	// project p1 has zero desired windows (all removed by user)
	desired := w.DesiredWorld{
		ActiveProfile: "prof",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"prof": {ID: "prof", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{}}, // empty
		},
	}
	shell1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"shell-1-p1": {ID: "shell-1-p1", Kind: w.WindowShell, MatchedTo: &shell1,
					App: w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"}, Workspace: "WS1",
					Title: w.ObservedTitle{Value: "shell-1:p1"}},
			},
		},
	}
	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	// project still exists in DesiredWorld
	if _, ok := desired.Projects["p1"]; !ok {
		t.Fatal("project p1 should still exist with empty Windows")
	}
	// observed shell-1 should be marked for removal
	found := false
	for _, o := range plan.Operations {
		if (o.Kind == op.KindKillSession || o.Kind == op.KindCloseWindow) &&
			o.Target.LiveWindow != nil && *o.Target.LiveWindow == "shell-1-p1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected removal of orphan shell-1-p1 after last desired window removed, plan: %+v", plan.Operations)
	}
}
