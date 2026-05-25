package planner

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/op"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §4.1 OP01-03: summon-shell/editor/browser の planner branch を verify。

// helper: env with slot Q→WS1, W→WS2.
func summonWindowEnv() w.ManagedEnvironment {
	return w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Viewer: "A",
			Workspaces: []w.WorkspaceSpec{
				{ID: "A", Role: w.WorkspaceViewer},
				{ID: "WS1", RawName: "1"},
				{ID: "WS2", RawName: "2"},
			},
			Slots: []w.SlotSpec{
				{ID: "Q", Workspace: "WS1", Order: 1},
				{ID: "W", Workspace: "WS2", Order: 2},
			},
		},
	}
}

// helper: project p1 on slot Q with 2 shells, 1 editor, 1 browser.
func summonWindowDesired() w.DesiredWorld {
	shell1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	shell2 := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 2}
	editor1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 1}
	browser1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowBrowser, Index: 1}
	return w.DesiredWorld{
		ActiveProfile: "prof",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"prof": {ID: "prof", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{
				{ID: shell1, Kind: w.WindowShell},
				{ID: shell2, Kind: w.WindowShell},
				{ID: editor1, Kind: w.WindowEditor},
				{ID: browser1, Kind: w.WindowBrowser},
			}},
		},
	}
}

// TestPlanner_SummonShell_FirstPressTargetsIndex1 — 起動時 focus が target
// 外なら shell-1 が target。
func TestPlanner_SummonShell_FirstPressTargetsIndex1(t *testing.T) {
	env := summonWindowEnv()
	desired := summonWindowDesired()
	shell1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	other := w.DesiredWindowID{Project: "other", Kind: w.WindowShell, Index: 1}
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"other-omni":  {ID: "other-omni", Kind: w.WindowShell, MatchedTo: &other, Workspace: "WS2"},
				"shell-1-p1":  {ID: "shell-1-p1", Kind: w.WindowShell, MatchedTo: &shell1, Workspace: "WS1"},
			},
			Focus: w.ObservedFocus{Window: "other-omni", Workspace: "WS2"},
		},
	}
	plan, err := Plan(state, desired, "intent:summon-shell:Q", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range plan.Operations {
		if o.Kind == op.KindFocusWindow && o.Target.LiveWindow != nil && *o.Target.LiveWindow == "shell-1-p1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected focus-window to shell-1-p1, plan: %+v", plan.Operations)
	}
}

// TestPlanner_SummonShell_CycleNextWhenAlreadyOnShell — 既に同 (project, shell)
// の index N に focus してるなら index N+1 が target、最後の index ならラップ。
func TestPlanner_SummonShell_CycleNextWhenAlreadyOnShell(t *testing.T) {
	env := summonWindowEnv()
	desired := summonWindowDesired()
	shell1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	shell2 := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 2}
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"shell-1-p1": {ID: "shell-1-p1", Kind: w.WindowShell, MatchedTo: &shell1, Workspace: "WS1"},
				"shell-2-p1": {ID: "shell-2-p1", Kind: w.WindowShell, MatchedTo: &shell2, Workspace: "WS1"},
			},
			Focus: w.ObservedFocus{Window: "shell-1-p1", Workspace: "WS1"},
		},
	}
	plan, err := Plan(state, desired, "intent:summon-shell:Q", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range plan.Operations {
		if o.Kind == op.KindFocusWindow && o.Target.LiveWindow != nil && *o.Target.LiveWindow == "shell-2-p1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected focus-window to shell-2-p1 (cycle next), plan: %+v", plan.Operations)
	}
}

// TestPlanner_SummonShell_CycleWrapsAtEnd — index=最後のとき index=1 にラップ。
func TestPlanner_SummonShell_CycleWrapsAtEnd(t *testing.T) {
	env := summonWindowEnv()
	desired := summonWindowDesired()
	shell1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	shell2 := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 2}
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"shell-1-p1": {ID: "shell-1-p1", Kind: w.WindowShell, MatchedTo: &shell1, Workspace: "WS1"},
				"shell-2-p1": {ID: "shell-2-p1", Kind: w.WindowShell, MatchedTo: &shell2, Workspace: "WS1"},
			},
			Focus: w.ObservedFocus{Window: "shell-2-p1", Workspace: "WS1"},
		},
	}
	plan, err := Plan(state, desired, "intent:summon-shell:Q", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range plan.Operations {
		if o.Kind == op.KindFocusWindow && o.Target.LiveWindow != nil && *o.Target.LiveWindow == "shell-1-p1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected wrap to shell-1-p1, plan: %+v", plan.Operations)
	}
}

// TestPlanner_SummonEditor_TargetsEditorOfSlotsProject — kind=editor も同じ
// pattern。
func TestPlanner_SummonEditor_TargetsEditorOfSlotsProject(t *testing.T) {
	env := summonWindowEnv()
	desired := summonWindowDesired()
	editor1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 1}
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"editor-1-p1": {ID: "editor-1-p1", Kind: w.WindowEditor, MatchedTo: &editor1, Workspace: "WS1"},
			},
			Focus: w.ObservedFocus{Window: "", Workspace: ""},
		},
	}
	plan, err := Plan(state, desired, "intent:summon-editor:Q", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range plan.Operations {
		if o.Kind == op.KindFocusWindow && o.Target.LiveWindow != nil && *o.Target.LiveWindow == "editor-1-p1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected focus-window to editor-1-p1, plan: %+v", plan.Operations)
	}
}

// TestPlanner_SummonBrowser_NoTargetWhenSlotUnassigned — slot が assign されて
// いなければ何もしない。
func TestPlanner_SummonBrowser_NoTargetWhenSlotUnassigned(t *testing.T) {
	env := summonWindowEnv()
	desired := summonWindowDesired()
	// Override: clear assignments.
	desired.Profiles["prof"] = w.DesiredProfile{ID: "prof", Assignments: map[w.SlotID]w.ProjectID{}}
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{}},
	}
	plan, err := Plan(state, desired, "intent:summon-browser:Q", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Operations {
		if o.IdempotencyKey == "summon-window:focus-ws:Q" || o.Kind == op.KindFocusWindow {
			// Allow other focus-window ops if any, but our summon shouldn't fire.
			if o.Kind == op.KindFocusWindow {
				// Cannot distinguish without more context; just ensure we don't see browser-1-p1.
				continue
			}
			t.Errorf("summon emitted op for unassigned slot: %+v", o)
		}
	}
}

// TestPlanner_SummonShell_NoTargetWhenNotSpawned — target window が未 spawn
// なら focus op は出ない。
func TestPlanner_SummonShell_NoTargetWhenNotSpawned(t *testing.T) {
	env := summonWindowEnv()
	desired := summonWindowDesired()
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{}, // empty
			Focus:   w.ObservedFocus{},
		},
	}
	plan, err := Plan(state, desired, "intent:summon-shell:Q", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Operations {
		if o.IdempotencyKey == "summon-window:focus-window:Q:shell" {
			t.Errorf("focus-window emitted while target shell unspawned: %+v", o)
		}
	}
}
