package planner

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/op"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §4.1 OP11: scratch shell の planner ops emission を verify する。

// helper: build a scratch SystemWindow with the given visibility/prior.
func scratchSW(vis w.CockpitVisibility, prior w.LiveWindowID) w.SystemWindow {
	return w.SystemWindow{
		ID:          w.SystemWindowID{Kind: w.WindowScratch, Index: 0},
		Kind:        w.WindowScratch,
		Title:       "projwm-scratch-shell",
		Visibility:  vis,
		PriorWindow: prior,
	}
}

// TestPlanner_Scratch_ShowEmitsWhenNotFocused — Visibility=Shown 且つ
// observed.Focus が scratch じゃない場合、show op が出る。
func TestPlanner_Scratch_ShowEmitsWhenNotFocused(t *testing.T) {
	env := minimalEnv()
	desired := minimalDesired([]w.SystemWindow{scratchSW(w.CockpitShown, "")})
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"shell-omni-1": {ID: "shell-omni-1", Title: w.ObservedTitle{Value: "shell-1:p1"}, Kind: w.WindowShell},
			},
			Focus: w.ObservedFocus{Window: "shell-omni-1"},
		},
	}
	plan, err := Plan(state, desired, "intent:show-scratch", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, o := range plan.Operations {
		if o.Kind == op.KindShowScratchShell {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 show-scratch-shell op, got %d (plan: %+v)", count, plan.Operations)
	}
}

// TestPlanner_Scratch_ShowNoOpWhenAlreadyFocused — scratch が既に focus
// されているなら show op は出ない (収束済)。
func TestPlanner_Scratch_ShowNoOpWhenAlreadyFocused(t *testing.T) {
	env := minimalEnv()
	desired := minimalDesired([]w.SystemWindow{scratchSW(w.CockpitShown, "")})
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"scratch-omni": {ID: "scratch-omni", Title: w.ObservedTitle{Value: "projwm-scratch-shell"}, Kind: w.WindowScratch},
			},
			Focus: w.ObservedFocus{Window: "scratch-omni"},
		},
	}
	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Operations {
		if o.Kind == op.KindShowScratchShell {
			t.Errorf("show-scratch-shell emitted when already focused: %+v", o)
		}
	}
}

// TestPlanner_Scratch_HideEmitsWhenFocused — Visibility=Hidden 且つ
// observed.Focus が scratch ID の場合、hide op が出る (PriorWindow を
// Target.LiveWindow に伝播)。
func TestPlanner_Scratch_HideEmitsWhenFocused(t *testing.T) {
	env := minimalEnv()
	desired := minimalDesired([]w.SystemWindow{scratchSW(w.CockpitHidden, "shell-omni-1")})
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"scratch-omni": {ID: "scratch-omni", Title: w.ObservedTitle{Value: "projwm-scratch-shell"}, Kind: w.WindowScratch},
			},
			Focus: w.ObservedFocus{Window: "scratch-omni"},
		},
	}
	plan, err := Plan(state, desired, "intent:hide-scratch", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	var hide *op.Operation
	for i := range plan.Operations {
		if plan.Operations[i].Kind == op.KindHideScratchShell {
			hide = &plan.Operations[i]
			break
		}
	}
	if hide == nil {
		t.Fatalf("expected hide-scratch-shell op, got plan: %+v", plan.Operations)
	}
	if hide.Target.LiveWindow == nil || *hide.Target.LiveWindow != "shell-omni-1" {
		t.Errorf("hide op Target.LiveWindow = %+v, want shell-omni-1", hide.Target.LiveWindow)
	}
}

// TestPlanner_Scratch_HideNoOpWhenNotFocused — scratch が既に focus を
// 失っているなら hide op は不要 (収束済)。
func TestPlanner_Scratch_HideNoOpWhenNotFocused(t *testing.T) {
	env := minimalEnv()
	desired := minimalDesired([]w.SystemWindow{scratchSW(w.CockpitHidden, "shell-omni-1")})
	state := w.WorldState{
		Environment: env, Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"shell-omni-1": {ID: "shell-omni-1", Title: w.ObservedTitle{Value: "shell-1:p1"}, Kind: w.WindowShell},
				"scratch-omni": {ID: "scratch-omni", Title: w.ObservedTitle{Value: "projwm-scratch-shell"}, Kind: w.WindowScratch},
			},
			Focus: w.ObservedFocus{Window: "shell-omni-1"},
		},
	}
	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Operations {
		if o.Kind == op.KindHideScratchShell {
			t.Errorf("hide-scratch-shell emitted while scratch not focused: %+v", o)
		}
	}
}

// TestPlanner_Scratch_NoSystemWindowEmitsNothing — scratch SystemWindow が
// DesiredWorld に存在しないなら show/hide op どちらも emit しない。
func TestPlanner_Scratch_NoSystemWindowEmitsNothing(t *testing.T) {
	env := minimalEnv()
	desired := minimalDesired(nil)
	state := w.WorldState{Environment: env, Desired: desired, Observed: w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{}}}
	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Operations {
		if o.Kind == op.KindShowScratchShell || o.Kind == op.KindHideScratchShell {
			t.Errorf("scratch op emitted without scratch SystemWindow: %+v", o)
		}
	}
}
