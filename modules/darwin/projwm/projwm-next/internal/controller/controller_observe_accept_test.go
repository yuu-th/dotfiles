package controller

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §4.3 / N-15 Tier-2 observe-accept (recovery-gate) contract.
//
// A user reorder (same window SET, different ORDER) on a STEADY workspace
// (managed handle-set unchanged since the last converged commit) is adopted into
// AcceptedLayouts. A divergence whose handle-set changed (OmniWM restart re-mints
// handles) or that has no converged snapshot yet (startup/recovery) is NOT
// adopted — the planner restores the saved layout instead (ACC-S7 / §3.5).

// buildReorderFixture: project p1 on slot/ws Q with shell-1 + editor-1, OBSERVED
// in shell-then-editor order. Canonical desired sorts by Kind ("editor" <
// "shell"), so the observed order is a genuine reorder. Returns the two live IDs.
func buildReorderFixture(t *testing.T) (*Controller, w.LiveWindowID, w.LiveWindowID) {
	t.Helper()
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Slots:      []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
			Workspaces: []w.WorkspaceSpec{{ID: "Q", RawName: "Q", DisplayName: "Q", Role: w.WorkspaceProject}},
		},
	}
	shell := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	editor := w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 1}
	desired := w.DesiredWorld{
		ActiveProfile: "default",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"default": {ID: "default", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{{ID: shell, Kind: w.WindowShell}, {ID: editor, Kind: w.WindowEditor}}},
		},
	}
	st := store.NewMemoryStore(desired)
	ctrl := New(env, desired, wm.NewFake(env), st)

	const L1 w.LiveWindowID = "ow_live_shell"
	const L2 w.LiveWindowID = "ow_live_editor"
	sCopy, eCopy := shell, editor
	ctrl.state.Observed.Windows = map[w.LiveWindowID]w.ObservedWindow{
		L1: {ID: L1, Workspace: "Q", MatchedTo: &sCopy},
		L2: {ID: L2, Workspace: "Q", MatchedTo: &eCopy},
	}
	// Observed columns in REORDER order: shell column first (canonical = editor first).
	ctrl.state.Observed.Layouts = map[w.WorkspaceID]w.ObservedLayout{
		"Q": {Workspace: "Q", Columns: []w.ObservedColumn{
			{Windows: []w.LiveWindowID{L1}, Mode: w.ColumnSolo},
			{Windows: []w.LiveWindowID{L2}, Mode: w.ColumnSolo},
		}},
	}
	return ctrl, L1, L2
}

func acceptedKindOrder(ctrl *Controller) []w.WindowKind {
	al, ok := ctrl.state.Desired.AcceptedLayouts["p1"]["Q"]
	if !ok {
		return nil
	}
	var ks []w.WindowKind
	for _, c := range al.Columns {
		for _, id := range c.Windows {
			ks = append(ks, id.Kind)
		}
	}
	return ks
}

func hasAccepted(ctrl *Controller) bool {
	_, ok := ctrl.state.Desired.AcceptedLayouts["p1"]["Q"]
	return ok
}

// Steady-state reorder, handle-set unchanged → adopt observed order.
func TestAutoAcceptObservedReorder_SteadyAccept(t *testing.T) {
	ctrl, L1, L2 := buildReorderFixture(t)
	ctrl.state.Meta.ConvergedLayoutHandles = map[w.WorkspaceID][]w.LiveWindowID{"Q": {L1, L2}}

	ctrl.autoAcceptObservedReorders()

	got := acceptedKindOrder(ctrl)
	want := []w.WindowKind{w.WindowShell, w.WindowEditor} // observed (reordered) order
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("§4.3/N-15: steady same-set reorder must be adopted into AcceptedLayouts in observed order; got %v want %v", got, want)
	}
}

// No converged snapshot (startup / recovery) → do NOT accept; planner enforces.
func TestAutoAcceptObservedReorder_NoSnapshotSkips(t *testing.T) {
	ctrl, _, _ := buildReorderFixture(t)
	ctrl.state.Meta.ConvergedLayoutHandles = nil

	ctrl.autoAcceptObservedReorders()

	if hasAccepted(ctrl) {
		t.Fatalf("§4.3/N-15: with no converged handle snapshot the divergence must NOT be accepted (recovery → restore); got AcceptedLayouts=%+v", ctrl.state.Desired.AcceptedLayouts)
	}
}

// Handle-set changed since converge (OmniWM restart re-minted handles) → skip.
func TestAutoAcceptObservedReorder_RestartHandlesSkip(t *testing.T) {
	ctrl, _, _ := buildReorderFixture(t)
	// Snapshot holds the PRE-restart handles; current observation has different
	// live IDs — exactly what an OmniWM restart (new instance-UUID) produces.
	ctrl.state.Meta.ConvergedLayoutHandles = map[w.WorkspaceID][]w.LiveWindowID{"Q": {"ow_pre_a", "ow_pre_b"}}

	ctrl.autoAcceptObservedReorders()

	if hasAccepted(ctrl) {
		t.Fatalf("§4.3/N-15: a changed handle-set (restart) must NOT be accepted as a reorder; got %+v", ctrl.state.Desired.AcceptedLayouts)
	}
}

// cloneControllerMeta must deep-clone ConvergedLayoutHandles (map AND slices) so
// a rollback snapshot is isolated from recordConvergedLayoutHandles' in-place
// mutation — mirrors WindowProvenance. (Review wf_db45e3c1-5b0 confirmed finding.)
func TestCloneControllerMeta_DeepClonesConvergedLayoutHandles(t *testing.T) {
	meta := w.ControllerMeta{
		ConvergedLayoutHandles: map[w.WorkspaceID][]w.LiveWindowID{"Q": {"ow_a", "ow_b"}},
	}
	snap := cloneControllerMeta(meta)

	// Mutate the LIVE map after the snapshot: in-place slice element + new key.
	meta.ConvergedLayoutHandles["Q"][0] = "ow_mutated"
	meta.ConvergedLayoutHandles["W"] = []w.LiveWindowID{"ow_new"}

	got := snap.ConvergedLayoutHandles["Q"]
	if len(got) != 2 || got[0] != "ow_a" || got[1] != "ow_b" {
		t.Fatalf("snapshot slice leaked live mutation (slice not deep-cloned); got %v want [ow_a ow_b]", got)
	}
	if _, leaked := snap.ConvergedLayoutHandles["W"]; leaked {
		t.Fatalf("snapshot gained key W from live mutation — map not deep-cloned")
	}
}

// Idempotence: once the observed order is accepted, a second pass is a no-op.
func TestAutoAcceptObservedReorder_Idempotent(t *testing.T) {
	ctrl, L1, L2 := buildReorderFixture(t)
	ctrl.state.Meta.ConvergedLayoutHandles = map[w.WorkspaceID][]w.LiveWindowID{"Q": {L1, L2}}

	ctrl.autoAcceptObservedReorders()
	first := acceptedKindOrder(ctrl)
	ctrl.autoAcceptObservedReorders()
	second := acceptedKindOrder(ctrl)

	if len(first) != len(second) || first[0] != second[0] || first[1] != second[1] {
		t.Fatalf("autoAccept must be idempotent once converged; first=%v second=%v", first, second)
	}
}
