package scenarios

import (
	"encoding/json"
	"testing"

	"github.com/yuu-th/projwm-next/internal/event"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/op"
	"github.com/yuu-th/projwm-next/internal/planner"
	"github.com/yuu-th/projwm-next/internal/reducer"
	"github.com/yuu-th/projwm-next/internal/verifier"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestDeterminismEvidenceReducerIntentAndEvent(t *testing.T) {
	env, desired := makeFixture()
	state := w.WorldState{
		Environment: env,
		Desired:     desired,
		Meta:        w.ControllerMeta{Epoch: 7},
	}
	beforeDesired := canonicalJSON(t, state.Desired)
	var intentWant string
	for i := 0; i < 20; i++ {
		got, err := reducer.ReduceIntent(state, intent.SwitchProfile{To: "B"})
		if err != nil {
			t.Fatalf("ReduceIntent: %v", err)
		}
		if after := canonicalJSON(t, state.Desired); after != beforeDesired {
			t.Fatalf("ReduceIntent mutated input DesiredWorld\nbefore=%s\nafter=%s", beforeDesired, after)
		}
		body := canonicalJSON(t, got)
		if i == 0 {
			intentWant = body
			continue
		}
		if body != intentWant {
			t.Fatalf("ReduceIntent is not deterministic\nwant=%s\ngot=%s", intentWant, body)
		}
	}

	project := w.ProjectID("p1")
	workspace := w.WorkspaceID("ws-q")
	ev := event.Event{
		ID:     "event-layout",
		Source: event.SourceUser,
		Kind:   event.KindUserReorderedColumns,
		Epoch:  7,
		Data: event.Data{
			Project:   &project,
			Workspace: &workspace,
			Columns: []w.DesiredColumn{{
				Windows: []w.DesiredWindowID{{Project: "p1", Kind: w.WindowEditor, Index: 1}, {Project: "p1", Kind: w.WindowAI, Index: 1}},
				Mode:    w.ColumnStacked,
			}},
		},
	}
	var eventWant string
	for i := 0; i < 20; i++ {
		got, err := reducer.ReactToEvent(state, ev)
		if err != nil {
			t.Fatalf("ReactToEvent: %v", err)
		}
		if after := canonicalJSON(t, state.Desired); after != beforeDesired {
			t.Fatalf("ReactToEvent mutated input DesiredWorld\nbefore=%s\nafter=%s", beforeDesired, after)
		}
		body := canonicalJSON(t, got)
		if i == 0 {
			eventWant = body
			continue
		}
		if body != eventWant {
			t.Fatalf("ReactToEvent is not deterministic\nwant=%s\ngot=%s", eventWant, body)
		}
	}
}

func TestDeterminismEvidencePlannerAndFinalFocus(t *testing.T) {
	env, desired := makeFixture()
	state := w.WorldState{
		Environment: env,
		Desired:     desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
			Focus:   w.ObservedFocus{Workspace: "ws-other", Window: "outside-1"},
		},
		Meta: w.ControllerMeta{Epoch: 11},
	}
	var want string
	for i := 0; i < 20; i++ {
		plan, err := planner.Plan(state, desired, planner.CommandKey("intent:reconcile"), op.ReasonIntent)
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		body := canonicalJSON(t, plan)
		if i == 0 {
			want = body
			continue
		}
		if body != want {
			t.Fatalf("planner output is not deterministic\nwant=%s\ngot=%s", want, body)
		}
	}

	otherFocus := state
	otherFocus.Observed.Focus = w.ObservedFocus{Workspace: "ws-w", Window: "outside-2"}
	planA, err := planner.Plan(state, desired, planner.CommandKey("intent:reconcile"), op.ReasonIntent)
	if err != nil {
		t.Fatalf("Plan A: %v", err)
	}
	planB, err := planner.Plan(otherFocus, desired, planner.CommandKey("intent:reconcile"), op.ReasonIntent)
	if err != nil {
		t.Fatalf("Plan B: %v", err)
	}
	if canonicalJSON(t, planA) != canonicalJSON(t, planB) {
		t.Fatalf("final-focus plan should not depend on pre-focus window/workspace\nA=%s\nB=%s", canonicalJSON(t, planA), canonicalJSON(t, planB))
	}
}

func TestDeterminismEvidenceVerifierDiff(t *testing.T) {
	desired := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	displayA := w.DisplayID("display-A")
	displayB := w.DisplayID("display-B")
	predicted := w.ObservedWorld{
		Displays: w.ObservedDisplayState{Displays: map[w.DisplayID]w.ObservedDisplay{displayA: {ID: displayA, Connected: true}}, Primary: &displayA},
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			"predicted-2": {ID: "predicted-2", Kind: w.WindowShell, Workspace: "ws-q", Title: w.ObservedTitle{Value: "shell-2:p1"}, MatchedTo: &desired},
			"predicted-1": {ID: "predicted-1", Kind: w.WindowShell, Workspace: "ws-q", Title: w.ObservedTitle{Value: "shell-1:p1"}, MatchedTo: &desired},
		},
		Layouts: map[w.WorkspaceID]w.ObservedLayout{
			"ws-q": {Workspace: "ws-q", Columns: []w.ObservedColumn{{Windows: []w.LiveWindowID{"predicted-1", "predicted-2"}, Mode: w.ColumnStacked}}},
		},
		Focus: w.ObservedFocus{Workspace: "ws-q", Window: "predicted-1"},
	}
	observed := w.ObservedWorld{
		Displays: w.ObservedDisplayState{Displays: map[w.DisplayID]w.ObservedDisplay{displayB: {ID: displayB, Connected: true}}, Primary: &displayB},
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			"observed-2": {ID: "observed-2", Kind: w.WindowShell, Workspace: "ws-w", Title: w.ObservedTitle{Value: "shell-2:p1"}, MatchedTo: &desired},
			"observed-1": {ID: "observed-1", Kind: w.WindowShell, Workspace: "ws-w", Title: w.ObservedTitle{Value: "shell-1:p1"}, MatchedTo: &desired},
		},
		Layouts: map[w.WorkspaceID]w.ObservedLayout{
			"ws-q": {Workspace: "ws-q", Columns: []w.ObservedColumn{{Windows: []w.LiveWindowID{"observed-2", "observed-1"}, Mode: w.ColumnStacked}}},
		},
		Focus: w.ObservedFocus{Workspace: "ws-w", Window: "observed-2"},
	}
	var want string
	for i := 0; i < 20; i++ {
		body := canonicalJSON(t, verifier.Diff(predicted, observed))
		if i == 0 {
			want = body
			continue
		}
		if body != want {
			t.Fatalf("verifier diff is not deterministic\nwant=%s\ngot=%s", want, body)
		}
	}
}

func canonicalJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal canonical JSON: %v", err)
	}
	return string(data)
}
