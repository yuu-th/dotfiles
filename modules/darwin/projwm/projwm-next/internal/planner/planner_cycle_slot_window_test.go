package planner

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/op"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §4.1 OP05 cycle-slot-window: 同じ slot 内で kind 別に focus 切替
// (workspace は変えない契約)。

// TestPlanner_CycleSlotWindow_SwitchesFromShellToEditor — shell-1 focus 中
// editor を指定すると editor-1 が target、focus-window のみ emit (focus-ws
// は出ない)。
func TestPlanner_CycleSlotWindow_SwitchesFromShellToEditor(t *testing.T) {
	env := summonWindowEnv()
	desired := summonWindowDesired()
	shell1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	editor1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 1}
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"shell-1-p1":  {ID: "shell-1-p1", Kind: w.WindowShell, MatchedTo: &shell1, Workspace: "WS1"},
				"editor-1-p1": {ID: "editor-1-p1", Kind: w.WindowEditor, MatchedTo: &editor1, Workspace: "WS1"},
			},
			Focus: w.ObservedFocus{Window: "shell-1-p1", Workspace: "WS1"},
		},
	}
	plan, err := Plan(state, desired, "intent:cycle-slot-window:Q:editor", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	var focusWin *op.Operation
	for i := range plan.Operations {
		o := &plan.Operations[i]
		if o.Kind == op.KindFocusWindow && o.Target.LiveWindow != nil && *o.Target.LiveWindow == "editor-1-p1" {
			focusWin = o
		}
		if o.Kind == op.KindFocusWorkspace && o.IdempotencyKey != "" &&
			(o.IdempotencyKey == "cycle-slot-window:focus:Q:editor") {
			t.Errorf("cycle-slot-window must NOT emit focus-workspace per SSOT 「current_ws 変わらない」: %+v", o)
		}
	}
	if focusWin == nil {
		t.Errorf("expected focus-window to editor-1-p1, plan: %+v", plan.Operations)
	}
}

// TestPlanner_CycleSlotWindow_CyclesWithinSameKind — shell-1 focus 中、shell
// を指定すると shell-2 (cycle next)。
func TestPlanner_CycleSlotWindow_CyclesWithinSameKind(t *testing.T) {
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
	plan, err := Plan(state, desired, "intent:cycle-slot-window:Q:shell", op.ReasonIntent)
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

// TestPlanner_CycleSlotWindow_NoOpWhenAlreadyOnTarget — 既に editor-1 に focus
// しているのに editor を指定 → cycle (editor は 1 件のみなのでラップして
// editor-1 のまま) → no-op (focus 既に対象)。
func TestPlanner_CycleSlotWindow_NoOpWhenAlreadyOnTarget(t *testing.T) {
	env := summonWindowEnv()
	desired := summonWindowDesired()
	editor1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 1}
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"editor-1-p1": {ID: "editor-1-p1", Kind: w.WindowEditor, MatchedTo: &editor1, Workspace: "WS1"},
			},
			Focus: w.ObservedFocus{Window: "editor-1-p1", Workspace: "WS1"},
		},
	}
	plan, err := Plan(state, desired, "intent:cycle-slot-window:Q:editor", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Operations {
		if o.IdempotencyKey == "cycle-slot-window:focus:Q:editor" {
			t.Errorf("focus-window emitted when already on target: %+v", o)
		}
	}
}

// TestPlanner_CycleSlotWindow_NoOpWhenKindNotInProject — slot.project に
// その kind の window が無いなら no-op。
func TestPlanner_CycleSlotWindow_NoOpWhenKindNotInProject(t *testing.T) {
	env := summonWindowEnv()
	desired := summonWindowDesired()
	// p1 には ai が定義されていない
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{}},
	}
	plan, err := Plan(state, desired, "intent:cycle-slot-window:Q:ai", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Operations {
		if o.IdempotencyKey == "cycle-slot-window:focus:Q:ai" {
			t.Errorf("focus emitted when kind absent from project: %+v", o)
		}
	}
}
