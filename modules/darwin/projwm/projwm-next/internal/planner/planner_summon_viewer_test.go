package planner

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/op"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §4.1 OP06 + §3.4 INV-05/INV-12 verification at planner level.

// helper: build an environment with 1 slot Q backed by workspace WS1, viewer "A".
func summonViewerEnv() w.ManagedEnvironment {
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

func summonViewerDesired() w.DesiredWorld {
	aiID1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowAI, Index: 1}
	viewerID1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowViewer, Index: 1}
	aiID2 := w.DesiredWindowID{Project: "p2", Kind: w.WindowAI, Index: 1}
	viewerID2 := w.DesiredWindowID{Project: "p2", Kind: w.WindowViewer, Index: 1}
	return w.DesiredWorld{
		ActiveProfile: "prof",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"prof": {ID: "prof", Assignments: map[w.SlotID]w.ProjectID{
				"Q": "p1",
				"W": "p2",
			}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{
				{ID: aiID1, Kind: w.WindowAI},
				{ID: viewerID1, Kind: w.WindowViewer},
			}},
			"p2": {ID: "p2", Windows: []w.DesiredWindow{
				{ID: aiID2, Kind: w.WindowAI},
				{ID: viewerID2, Kind: w.WindowViewer},
			}},
		},
	}
}

// TestPlanner_SummonViewer_FromFocusedAITargetsItsViewer — 直前 focus が AI
// なら、その (project, index) の viewer が target になる。
func TestPlanner_SummonViewer_FromFocusedAITargetsItsViewer(t *testing.T) {
	env := summonViewerEnv()
	desired := summonViewerDesired()
	aiID := w.DesiredWindowID{Project: "p2", Kind: w.WindowAI, Index: 1}
	viewerID := w.DesiredWindowID{Project: "p2", Kind: w.WindowViewer, Index: 1}
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"ai-omni-p2": {ID: "ai-omni-p2", Kind: w.WindowAI, MatchedTo: &aiID, Workspace: "WS2"},
				"v-omni-p2":  {ID: "v-omni-p2", Kind: w.WindowViewer, MatchedTo: &viewerID, Workspace: "A"},
			},
			Focus: w.ObservedFocus{Window: "ai-omni-p2", Workspace: "WS2"},
		},
	}
	plan, err := Plan(state, desired, "intent:summon-viewer", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	var focusWS, focusWin *op.Operation
	for i := range plan.Operations {
		o := &plan.Operations[i]
		if o.Kind == op.KindFocusWorkspace && o.Target.Workspace != nil && *o.Target.Workspace == "A" {
			focusWS = o
		}
		if o.Kind == op.KindFocusWindow && o.Target.LiveWindow != nil && *o.Target.LiveWindow == "v-omni-p2" {
			focusWin = o
		}
	}
	if focusWS == nil {
		t.Errorf("expected focus-workspace A op, plan: %+v", plan.Operations)
	}
	if focusWin == nil {
		t.Errorf("expected focus-window op targeting v-omni-p2, plan: %+v", plan.Operations)
	}
}

// TestPlanner_SummonViewer_FromNonAIFallsBackToFirstSlot — 直前 focus が shell
// など AI 以外なら、slot 順 (Q→W) の最初の slot=Q の project p1 の viewer-1
// が target。
func TestPlanner_SummonViewer_FromNonAIFallsBackToFirstSlot(t *testing.T) {
	env := summonViewerEnv()
	desired := summonViewerDesired()
	viewerID1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowViewer, Index: 1}
	shellID := w.DesiredWindowID{Project: "p2", Kind: w.WindowShell, Index: 1}
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"shell-omni": {ID: "shell-omni", Kind: w.WindowShell, MatchedTo: &shellID, Workspace: "WS2"},
				"v-omni-p1":  {ID: "v-omni-p1", Kind: w.WindowViewer, MatchedTo: &viewerID1, Workspace: "A"},
			},
			Focus: w.ObservedFocus{Window: "shell-omni", Workspace: "WS2"},
		},
	}
	plan, err := Plan(state, desired, "intent:summon-viewer", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range plan.Operations {
		if o.Kind == op.KindFocusWindow && o.Target.LiveWindow != nil && *o.Target.LiveWindow == "v-omni-p1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected focus-window targeting p1 viewer (slot order fallback), plan: %+v", plan.Operations)
	}
}

// TestPlanner_SummonViewer_WhenAlreadyFocusedNoFocusOp — viewer が既に focus
// されているなら focus-window op は出ない (冪等)。
func TestPlanner_SummonViewer_WhenAlreadyFocusedNoFocusOp(t *testing.T) {
	env := summonViewerEnv()
	desired := summonViewerDesired()
	viewerID1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowViewer, Index: 1}
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"v-omni-p1": {ID: "v-omni-p1", Kind: w.WindowViewer, MatchedTo: &viewerID1, Workspace: "A"},
			},
			// 既に viewer に focus
			Focus: w.ObservedFocus{Window: "v-omni-p1", Workspace: "A"},
		},
	}
	plan, err := Plan(state, desired, "intent:summon-viewer", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Operations {
		if o.Kind == op.KindFocusWindow {
			t.Errorf("focus-window emitted when target already focused: %+v", o)
		}
		if o.Kind == op.KindFocusWorkspace && o.Target.Workspace != nil && *o.Target.Workspace == "A" {
			t.Errorf("focus-workspace A emitted when already on A: %+v", o)
		}
	}
}

// TestPlanner_SummonViewer_WhenViewerNotSpawnedNoOp — target viewer が
// observed に未だ存在しないなら focus op は出ない (spawn は別経路で発火する)。
func TestPlanner_SummonViewer_WhenViewerNotSpawnedNoOp(t *testing.T) {
	env := summonViewerEnv()
	desired := summonViewerDesired()
	aiID := w.DesiredWindowID{Project: "p1", Kind: w.WindowAI, Index: 1}
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"ai-omni-p1": {ID: "ai-omni-p1", Kind: w.WindowAI, MatchedTo: &aiID, Workspace: "WS1"},
				// viewer not spawned
			},
			Focus: w.ObservedFocus{Window: "ai-omni-p1", Workspace: "WS1"},
		},
	}
	plan, err := Plan(state, desired, "intent:summon-viewer", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Operations {
		if o.Kind == op.KindFocusWindow {
			t.Errorf("focus-window emitted while viewer not yet spawned: %+v", o)
		}
	}
}

// TestPlanner_SummonViewer_OnlyFiresOnIntentCommandKey — commandKey が
// summon-viewer 以外なら何もしない (reconcile, switch-profile 等で焼け)。
func TestPlanner_SummonViewer_OnlyFiresOnIntentCommandKey(t *testing.T) {
	env := summonViewerEnv()
	desired := summonViewerDesired()
	aiID := w.DesiredWindowID{Project: "p1", Kind: w.WindowAI, Index: 1}
	viewerID := w.DesiredWindowID{Project: "p1", Kind: w.WindowViewer, Index: 1}
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"ai-omni":  {ID: "ai-omni", Kind: w.WindowAI, MatchedTo: &aiID, Workspace: "WS1"},
				"v-omni":   {ID: "v-omni", Kind: w.WindowViewer, MatchedTo: &viewerID, Workspace: "A"},
			},
			Focus: w.ObservedFocus{Window: "ai-omni", Workspace: "WS1"},
		},
	}
	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Operations {
		if o.IdempotencyKey == "summon-viewer:focus-ws" || o.IdempotencyKey == "summon-viewer:focus-window" {
			t.Errorf("summon-viewer op leaked into non-summon command: %+v", o)
		}
	}
}
