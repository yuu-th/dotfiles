package controller

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

type fakeRuntimeValidator struct {
	reports  []store.RuntimeValidationReport
	blocking bool
	err      error
}

func (s fakeRuntimeValidator) ValidateEnvironment(ctx context.Context, env w.ManagedEnvironment) ([]store.RuntimeValidationReport, bool, error) {
	if s.err != nil {
		return nil, false, s.err
	}
	return append([]store.RuntimeValidationReport(nil), s.reports...), s.blocking, nil
}

func TestControllerCommitsRedactedTransactionTrace(t *testing.T) {
	desired := w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{},
	}
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{{ID: "A", RawName: "A", DisplayName: "A", Role: w.WorkspaceViewer}},
		},
	}
	st := store.NewMemoryStore(desired)
	ctrl := New(env, desired, wm.NewFake(env), st)

	result, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{})
	if err != nil {
		t.Fatalf("ApplyIntent: %v", err)
	}
	gen, err := st.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("LoadCurrentGeneration: %v", err)
	}
	trace := gen.Trace
	if trace.TransactionID == "" || result.TransactionID != trace.TransactionID || result.CommittedGeneration != gen.ID {
		t.Fatalf("transaction evidence mismatch: trace=%+v result=%+v generation=%s", trace, result, gen.ID)
	}
	if trace.Command != "intent:reconcile" || trace.TriggerSource != "user" || trace.TriggerKind != string(intent.KindReconcile) {
		t.Fatalf("unexpected trace trigger/command: %+v", trace)
	}
	if !trace.Converged || trace.VerifierRan || trace.VerifierMode != "self-diff-diagnostic" || trace.TotalOperations != 0 || trace.MutationOperations != 0 || trace.AttemptedOperations != 0 || trace.ExecutedMutations != 0 || trace.VerifierDiffEntries != 0 {
		t.Fatalf("converged zero-mutation reconcile evidence not recorded: %+v", trace)
	}
	if len(trace.PlanIterations) == 0 || trace.PlanIterations[0].PlannedOperations != 0 {
		t.Fatalf("missing zero-op plan iteration evidence: %+v", trace.PlanIterations)
	}
}

func TestControllerRecordsValidateEnvironmentLegacyAgentReports(t *testing.T) {
	desired := w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{},
	}
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{{ID: "A", RawName: "A", DisplayName: "A", Role: w.WorkspaceViewer}},
		},
	}
	st := store.NewMemoryStore(desired)
	ctrl := New(env, desired, wm.NewFake(env), st)
	ctrl.RuntimeValidator = fakeRuntimeValidator{reports: []store.RuntimeValidationReport{{
		Kind:    "legacy-agent",
		Subject: "org.nixos.projwm-reconcile-watch",
		Policy:  "remove",
		Status:  "absent",
		Action:  "removal-satisfied",
	}}}

	if _, err := ctrl.ApplyIntent(context.Background(), intent.ValidateEnvironment{}); err != nil {
		t.Fatalf("ApplyIntent: %v", err)
	}
	gen, err := st.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("LoadCurrentGeneration: %v", err)
	}
	trace := gen.Trace
	if trace.Command != "intent:validate-environment" || trace.RuntimeValidationBlocking || len(trace.RuntimeValidationReports) != 1 {
		t.Fatalf("validate-environment trace missing runtime validation evidence: %+v", trace)
	}
	if got := trace.RuntimeValidationReports[0]; got.Subject != "org.nixos.projwm-reconcile-watch" || got.Action != "removal-satisfied" {
		t.Fatalf("unexpected legacy report: %+v", got)
	}
}

func TestControllerBlocksMutationsWhenRemovePolicyLegacyAgentActive(t *testing.T) {
	desired := w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{},
	}
	env := w.ManagedEnvironment{SchemaVersion: 1, Authority: "nix"}
	st := store.NewMemoryStore(desired)
	ctrl := New(env, desired, wm.NewFake(env), st)
	ctrl.RuntimeValidator = fakeRuntimeValidator{
		reports: []store.RuntimeValidationReport{{
			Kind:     "legacy-agent",
			Subject:  "org.nixos.projwm-reconcile-watch",
			Policy:   "remove",
			Status:   "active",
			Action:   "remove-by-nix-rebuild",
			Blocking: true,
		}},
		blocking: true,
	}

	result, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{})
	if err == nil {
		t.Fatal("ApplyIntent should block when a remove-policy legacy writer is active")
	}
	if result.CommittedGeneration != "" || result.Trace.NoCommitReason != "runtime-validation-blocked" || !result.Trace.RuntimeValidationBlocking {
		t.Fatalf("blocking trace mismatch: result=%+v err=%v", result, err)
	}
	if len(result.Trace.RuntimeValidationReports) != 1 || !result.Trace.RuntimeValidationReports[0].Blocking {
		t.Fatalf("blocking legacy report missing: %+v", result.Trace.RuntimeValidationReports)
	}
}

func TestControllerRecordsRuntimeValidationErrorsAsNoCommit(t *testing.T) {
	desired := w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{},
	}
	env := w.ManagedEnvironment{SchemaVersion: 1, Authority: "nix"}
	st := store.NewMemoryStore(desired)
	ctrl := New(env, desired, wm.NewFake(env), st)
	ctrl.RuntimeValidator = fakeRuntimeValidator{err: fmt.Errorf("validator exploded")}

	result, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{})
	if err == nil {
		t.Fatal("ApplyIntent should fail when runtime validation errors")
	}
	if result.Trace.NoCommitReason != "runtime-validation-error" || result.Trace.Command != "intent:reconcile" {
		t.Fatalf("runtime validation error trace mismatch: %+v", result.Trace)
	}
}

func TestControllerRollsBackMemoryOnInvariantFailure(t *testing.T) {
	desired := w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{},
		FocusPolicy: w.FocusPolicySet{
			FinalFocus: map[string]w.WorkspaceID{"intent:reconcile": "missing-workspace"},
		},
	}
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{{ID: "A", RawName: "A", DisplayName: "A", Role: w.WorkspaceViewer}},
		},
	}
	st := store.NewMemoryStore(desired)
	ctrl := New(env, desired, wm.NewFake(env), st)
	ctrl.state.Meta.Epoch = 7
	ctrl.state.Meta.DirtyScopes = []w.DirtyScope{{Kind: "project", Key: "p1"}}
	beforeDesired := cloneDesiredWorld(ctrl.state.Desired)
	beforeMeta := cloneControllerMeta(ctrl.state.Meta)

	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err == nil {
		t.Fatal("ApplyIntent should fail invariant final-focus")
	}
	if !reflect.DeepEqual(beforeDesired, ctrl.state.Desired) {
		t.Fatalf("DesiredWorld changed after invariant failure\nbefore=%+v\nafter=%+v", beforeDesired, ctrl.state.Desired)
	}
	// SSOT §7.1 step 3+4 carve-out: rollback restores most ControllerMeta
	// fields but preserves ActiveCards (user notification) and grows
	// DirtyScopes (next-retry trigger). Compare only the rollback-target
	// fields here.
	if ctrl.state.Meta.Epoch != beforeMeta.Epoch {
		t.Fatalf("Epoch changed across rollback: before=%d after=%d", beforeMeta.Epoch, ctrl.state.Meta.Epoch)
	}
	if ctrl.state.Meta.Transaction != nil {
		t.Fatalf("Transaction not cleared after rollback: %+v", ctrl.state.Meta.Transaction)
	}
	// ActiveCards MUST have grown by exactly the [INVARIANT] card
	// emitted by failNoCommitTrace (SSOT §7.1 step 3).
	if len(ctrl.state.Meta.ActiveCards) != len(beforeMeta.ActiveCards)+1 {
		t.Fatalf("expected one INVARIANT card after rollback (SSOT §7.1 step 3); cards before=%d after=%d", len(beforeMeta.ActiveCards), len(ctrl.state.Meta.ActiveCards))
	}
	if ctrl.state.Meta.ActiveCards[len(ctrl.state.Meta.ActiveCards)-1].Type != w.CardTypeInvariant {
		t.Errorf("post-rollback card type = %s, want INVARIANT", ctrl.state.Meta.ActiveCards[len(ctrl.state.Meta.ActiveCards)-1].Type)
	}
	// DirtyScopes MUST have grown by the markGlobalDirty entry (SSOT §7.1 step 4).
	if len(ctrl.state.Meta.DirtyScopes) <= len(beforeMeta.DirtyScopes) {
		t.Fatalf("expected new DirtyScope after rollback (SSOT §7.1 step 4); scopes before=%d after=%d", len(beforeMeta.DirtyScopes), len(ctrl.state.Meta.DirtyScopes))
	}
	gen, err := st.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("LoadCurrentGeneration: %v", err)
	}
	if gen.ID != "G000001" {
		t.Fatalf("invariant failure must not commit, got %s", gen.ID)
	}
}

func TestControllerRecordsNoCommitTraceOnExecutorError(t *testing.T) {
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{{ID: "A", RawName: "A", DisplayName: "A", Role: w.WorkspaceProject}},
			Slots:      []w.SlotSpec{{ID: "Q", Workspace: "A", Order: 1}},
		},
	}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {
				ID: "p1",
				Windows: []w.DesiredWindow{{
					ID:   w.DesiredWindowID{Project: "p1", Kind: w.WindowAI, Index: 1},
					Kind: w.WindowAI,
					App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
					TitleContract: w.TitleContract{
						Authority: w.TitleControllerOwned,
						Expected:  "ai-1:p1",
					},
				}},
			},
		},
	}
	st := store.NewMemoryStore(desired)
	adapter := &failingMoveAdapter{Fake: wm.NewFake(env)}
	ctrl := New(env, desired, adapter, st)

	result, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{})
	if err == nil {
		t.Fatal("ApplyIntent should fail executor move")
	}
	if result.TransactionID == "" {
		t.Fatalf("failed transaction response missing transaction id: %+v", result)
	}
	traces := st.TransactionTraces()
	if len(traces) != 1 {
		t.Fatalf("executor failure should record one no-commit trace, got %d", len(traces))
	}
	trace := traces[0]
	if trace.TransactionID != result.TransactionID || trace.NoCommitReason != "executor-error" || trace.AttemptedOperations == 0 || trace.CommittedGeneration != "" || trace.CurrentGeneration != "G000001" {
		t.Fatalf("bad executor failure trace: %+v result=%+v", trace, result)
	}
	gen, err := st.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("LoadCurrentGeneration: %v", err)
	}
	if gen.ID != "G000001" {
		t.Fatalf("executor failure must not commit, got %s", gen.ID)
	}
}

func TestControllerRecordsNoCommitTraceOnEarlyObserveError(t *testing.T) {
	desired := w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{},
	}
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{{ID: "A", RawName: "A", DisplayName: "A", Role: w.WorkspaceViewer}},
		},
	}
	st := store.NewMemoryStore(desired)
	ctrl := New(env, desired, &failingObserveAdapter{Fake: wm.NewFake(env)}, st)

	result, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{})
	if err == nil {
		t.Fatal("ApplyIntent should fail observe")
	}
	if result.TransactionID == "" {
		t.Fatalf("early failed transaction response missing transaction id: %+v", result)
	}
	traces := st.TransactionTraces()
	if len(traces) != 1 {
		t.Fatalf("observe failure should record one no-commit trace, got %d", len(traces))
	}
	trace := traces[0]
	if trace.TransactionID != result.TransactionID || trace.NoCommitReason != "observe-error" || !trace.ObservationRefreshFailed || trace.ObservationRefreshError == "" || trace.CurrentGeneration != "G000001" {
		t.Fatalf("bad observe failure trace: %+v result=%+v", trace, result)
	}
	if len(ctrl.state.Meta.DirtyScopes) != 1 || ctrl.state.Meta.DirtyScopes[0] != (w.DirtyScope{Kind: "global", Key: "observation-refresh-failed"}) {
		t.Fatalf("observe failure should mark global dirty scope, got %+v", ctrl.state.Meta.DirtyScopes)
	}
}

func TestControllerMarksDirtyWhenFailureRefreshObserveFails(t *testing.T) {
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{{ID: "A", RawName: "A", DisplayName: "A", Role: w.WorkspaceProject}},
			Slots:      []w.SlotSpec{{ID: "Q", Workspace: "A", Order: 1}},
		},
	}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {
				ID: "p1",
				Windows: []w.DesiredWindow{{
					ID:   w.DesiredWindowID{Project: "p1", Kind: w.WindowAI, Index: 1},
					Kind: w.WindowAI,
					App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
					TitleContract: w.TitleContract{
						Authority: w.TitleControllerOwned,
						Expected:  "ai-1:p1",
					},
				}},
			},
		},
	}
	st := store.NewMemoryStore(desired)
	adapter := &failingMoveThenObserveAdapter{Fake: wm.NewFake(env)}
	ctrl := New(env, desired, adapter, st)
	ctrl.state.Meta.DirtyScopes = []w.DirtyScope{{Kind: "project", Key: "p1"}}

	result, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{})
	if err == nil {
		t.Fatal("ApplyIntent should fail executor move")
	}
	if result.TransactionID == "" || !result.Trace.ObservationRefreshFailed || result.Trace.ObservationRefreshError == "" {
		t.Fatalf("failed transaction response missing observation refresh failure evidence: %+v", result)
	}
	traces := st.TransactionTraces()
	if len(traces) != 1 {
		t.Fatalf("executor failure should record one no-commit trace, got %d", len(traces))
	}
	trace := traces[0]
	if trace.TransactionID != result.TransactionID || trace.NoCommitReason != "executor-error" || !trace.ObservationRefreshFailed || trace.ObservationRefreshError == "" || trace.CommittedGeneration != "" {
		t.Fatalf("bad executor refresh failure trace: %+v result=%+v", trace, result)
	}
	wantDirty := []w.DirtyScope{
		{Kind: "project", Key: "p1"},
		{Kind: "global", Key: "commit-fail:executor-error"}, // SSOT §7.1 step 4 — fail path marks dirty
		{Kind: "global", Key: "observation-refresh-failed"}, // observe failure after rollback also marks dirty
	}
	if !reflect.DeepEqual(wantDirty, ctrl.state.Meta.DirtyScopes) {
		t.Fatalf("rollback should preserve prior dirty scopes and add fail-path + refresh-failure dirty scopes\nwant=%+v\ngot=%+v", wantDirty, ctrl.state.Meta.DirtyScopes)
	}
}

type failingObserveAdapter struct {
	*wm.Fake
}

func (a *failingObserveAdapter) Observe(ctx context.Context) (w.ObservedWorld, error) {
	return w.ObservedWorld{}, errors.New("forced observe failure")
}

// movableWindowObservation injects a single managed AI window that matches
// ai-1:p1 but sits on the WRONG workspace ("B" instead of its slot workspace),
// so the planner emits a move-window-to-workspace (layout phase) op. A move is
// NOT degradable under §6.8 (only per-window spawns are), so failing it
// exercises the genuine executor-error hard-abort path that remains for
// removal/layout ops.
func movableWindowObservation() w.ObservedWorld {
	return w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			"live-1": {
				ID:        "live-1",
				Kind:      w.WindowAI,
				Workspace: "B",
				App:       w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"},
				Title:     w.ObservedTitle{Value: "ai-1:p1"},
			},
		},
		Workspaces: map[w.WorkspaceID]w.ObservedWorkspace{
			"A": {ID: "A"}, "B": {ID: "B"},
		},
		Layouts: map[w.WorkspaceID]w.ObservedLayout{},
	}
}

type failingMoveThenObserveAdapter struct {
	*wm.Fake
	observes atomic.Int32
}

func (a *failingMoveThenObserveAdapter) Observe(ctx context.Context) (w.ObservedWorld, error) {
	if a.observes.Add(1) > 1 {
		return w.ObservedWorld{}, errors.New("forced refresh observe failure")
	}
	return movableWindowObservation(), nil
}

func (a *failingMoveThenObserveAdapter) MoveToWorkspace(ctx context.Context, id w.LiveWindowID, ws w.WorkspaceID) error {
	return errors.New("forced move failure")
}

type failingMoveAdapter struct {
	*wm.Fake
}

func (a *failingMoveAdapter) Observe(ctx context.Context) (w.ObservedWorld, error) {
	return movableWindowObservation(), nil
}

func (a *failingMoveAdapter) MoveToWorkspace(ctx context.Context, id w.LiveWindowID, ws w.WorkspaceID) error {
	return errors.New("forced move failure")
}
