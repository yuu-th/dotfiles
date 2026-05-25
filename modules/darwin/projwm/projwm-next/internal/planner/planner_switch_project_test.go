package planner

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/op"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §4.1 OP04 switch-project の planner branch を verify する。
// 「target slot の workspace に focus を移す」ことだけを planner が担当し、
// 「直前 focused だった window への復帰」は omniwm の per-workspace MRU に任せる。

// TestPlanner_SwitchProject_EmitsFocusWorkspace — target slot の workspace
// に focus-workspace op が emit される。
func TestPlanner_SwitchProject_EmitsFocusWorkspace(t *testing.T) {
	env := summonWindowEnv()
	desired := summonWindowDesired()
	// slot W は dotfiles 用に追加 (summonWindowDesired は Q→p1 のみ assign)
	desired.Profiles["prof"].Assignments["W"] = "p2"
	desired.Projects["p2"] = w.DesiredProject{ID: "p2", Windows: nil}

	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{},
			Focus:   w.ObservedFocus{Workspace: "WS1"},
		},
	}
	plan, err := Plan(state, desired, "intent:switch-project:W", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range plan.Operations {
		if o.Kind == op.KindFocusWorkspace && o.Target.Workspace != nil && *o.Target.Workspace == "WS2" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected focus-workspace WS2, plan: %+v", plan.Operations)
	}
}

// TestPlanner_SwitchProject_NoOpWhenAlreadyOnTargetWorkspace — 既に target
// workspace にいるなら op を emit しない。
func TestPlanner_SwitchProject_NoOpWhenAlreadyOnTargetWorkspace(t *testing.T) {
	env := summonWindowEnv()
	desired := summonWindowDesired()
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{},
			Focus:   w.ObservedFocus{Workspace: "WS1"},
		},
	}
	plan, err := Plan(state, desired, "intent:switch-project:Q", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Operations {
		if o.IdempotencyKey == "switch-project:focus-ws:Q" {
			t.Errorf("focus-workspace emitted when already on target: %+v", o)
		}
	}
}

// TestPlanner_SwitchProject_NoOpWhenSlotUnassigned — slot が active profile
// で assign されていなければ何もしない (switch 先がない)。
func TestPlanner_SwitchProject_NoOpWhenSlotUnassigned(t *testing.T) {
	env := summonWindowEnv()
	desired := summonWindowDesired()
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{},
			Focus:   w.ObservedFocus{Workspace: "WS1"},
		},
	}
	// slot W は未 assign (summonWindowDesired は Q のみ)
	plan, err := Plan(state, desired, "intent:switch-project:W", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Operations {
		if o.IdempotencyKey == "switch-project:focus-ws:W" {
			t.Errorf("focus-workspace emitted for unassigned slot: %+v", o)
		}
	}
}

// TestPlanner_SwitchProject_NoOpWhenSlotUnknown — 存在しない slot ID が来た
// ら no-op (error にせず graceful)。
func TestPlanner_SwitchProject_NoOpWhenSlotUnknown(t *testing.T) {
	env := summonWindowEnv()
	desired := summonWindowDesired()
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{},
			Focus:   w.ObservedFocus{Workspace: "WS1"},
		},
	}
	plan, err := Plan(state, desired, "intent:switch-project:XYZ", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Operations {
		if o.IdempotencyKey == "switch-project:focus-ws:XYZ" {
			t.Errorf("focus-workspace emitted for unknown slot: %+v", o)
		}
	}
}
