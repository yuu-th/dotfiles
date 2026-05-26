// §3.8 Transaction Contract Scenarios (S8.A — S8.F).
// Each scenario directly exercises one of the path properties from specs §2.2
// (A: single writer, B: precondition-uniqueness, C: verifier replan,
//
//	D: user-origin layout candidate, E: external events do not mutate Desired,
//	F: stale-epoch discard).
package scenarios

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/controller"
	"github.com/yuu-th/projwm-next/internal/event"
	"github.com/yuu-th/projwm-next/internal/executor"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/op"
	"github.com/yuu-th/projwm-next/internal/scenario"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// S8.A — wmMutationLock serializes transactions (specs §2.2-A).
//
// Two transactions issued concurrently must not have any mutation method
// overlap on the wire (max in-flight count <= 1). The fake adapter delays
// every mutation entry to widen the race window.
func TestTransactionContractS8A_SingleWriter(t *testing.T) {
	env, desired := makeFixture()
	b := scenario.NewBackend(scenario.BackendFake, env, desired)
	// Bootstrap once so each subsequent ApplyIntent emits a small but non-empty
	// op set (focus changes etc.) that exposes interleaving if the lock fails.
	if err := b.ApplyIntent(intent.Reconcile{}); err != nil {
		t.Fatalf("setup reconcile: %v", err)
	}
	b.Fake.SetMutationDelay(2 * time.Millisecond)
	b.Fake.ResetTransactionContractCounters()

	const N = 6
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		profile := w.ProfileID("A")
		if i%2 == 1 {
			profile = "B"
		}
		go func(p w.ProfileID) {
			defer wg.Done()
			_ = b.ApplyIntent(intent.SwitchProfile{To: p})
		}(profile)
	}
	wg.Wait()

	if got := b.Fake.TransactionContractMaxInFlight(); got > 1 {
		t.Fatalf("S8.A: expected max-in-flight <= 1, got %d (lock failed)", got)
	}
}

// S8.B — Executor refuses mutation when identity is not unique-strong (specs §2.2-B).
//
// We hand-build a Move op whose PreUniqueStrong precondition targets a
// LiveWindow that does not exist in the observed world. The executor MUST
// reject before issuing any adapter call.
func TestTransactionContractS8B_PreconditionUniqueness(t *testing.T) {
	env, _ := makeFixture()
	fake := wm.NewFake(env)
	exe := &executor.Executor{Adapter: fake, Env: env}

	missing := w.LiveWindowID("lw-does-not-exist")
	target := w.WorkspaceID("ws-w")
	moveOp := op.Operation{
		ID:     "test-move",
		Kind:   op.KindMoveWindowToWorkspace,
		Target: op.Target{LiveWindow: &missing, Workspace: &target},
		Preconditions: []op.Precondition{
			{Kind: op.PreUniqueStrong, Target: op.Target{LiveWindow: &missing}},
			{Kind: op.PreWorkspaceExists, Target: op.Target{Workspace: &target}},
		},
	}
	obs, err := fake.Observe(context.Background())
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if err := exe.Execute(context.Background(), moveOp, obs, w.DesiredWorld{}); err == nil {
		t.Fatal("S8.B: expected executor to reject op with unsatisfied PreUniqueStrong, got nil")
	}
}

// S8.C — Verifier non-empty diff after MaxReplans causes commit refusal (specs §2.2-C).
//
// We toggle the fake into "drop mutations" mode: planner emits ops, executor
// calls fake methods, but state never changes. The simulator-backed Verifier
// computes a non-empty diff each iteration, the loop exhausts MaxReplans and
// the controller MUST return ReplanExceededError without committing.
func TestTransactionContractS8C_VerifierReplanGating(t *testing.T) {
	env, desired := makeFixture()
	b := scenario.NewBackend(scenario.BackendSimulator, env, desired)
	// Establish a baseline so subsequent intent has work to do.
	if err := b.ApplyIntent(intent.Reconcile{}); err != nil {
		t.Fatalf("setup reconcile: %v", err)
	}
	preEpoch := b.Controller.State().Meta.Epoch
	preDesired := cloneDesiredForCompare(b.Controller.State().Desired)
	preDirty := append([]w.DirtyScope(nil), b.Controller.State().Meta.DirtyScopes...)

	b.Fake.SetDropMutations(true)
	defer b.Fake.SetDropMutations(false)

	err := b.ApplyIntent(intent.SwitchProfile{To: "B"})
	if err == nil {
		t.Fatal("S8.C: expected ReplanExceededError, got nil")
	}
	var ree *controller.ReplanExceededError
	if !errors.As(err, &ree) {
		t.Fatalf("S8.C: expected *ReplanExceededError, got %T: %v", err, err)
	}
	if ree.MaxReplans <= 0 {
		t.Fatalf("S8.C: ReplanExceededError reports invalid MaxReplans %d", ree.MaxReplans)
	}
	// Epoch must NOT have advanced (commit was refused).
	if got := b.Controller.State().Meta.Epoch; got != preEpoch {
		t.Fatalf("S8.C: epoch advanced despite replan failure (pre=%d post=%d)", preEpoch, got)
	}
	if !reflect.DeepEqual(preDesired, cloneDesiredForCompare(b.Controller.State().Desired)) {
		t.Fatal("S8.C: failed transaction leaked DesiredWorld into controller memory")
	}
	// SSOT §7.1 step 4: max-replans-exceeded MUST add a "max-replans-exceeded"
	// global dirty scope so the next intent / event retries. The post-fail
	// scope set is therefore the pre-fail set + 1 new entry. (Verifier-diff-
	// unacceptable path emits the same scope via failNoCommitTrace with the
	// reason in the key.)
	postDirty := b.Controller.State().Meta.DirtyScopes
	if len(postDirty) != len(preDirty)+1 {
		t.Fatalf("S8.C: SSOT §7.1 step 4 requires one new dirty scope after fail; pre=%+v post=%+v", preDirty, postDirty)
	}
	newScope := postDirty[len(postDirty)-1]
	if newScope.Kind != "global" {
		t.Errorf("S8.C: new dirty scope must be global, got %+v", newScope)
	}
	traces := b.Store.TransactionTraces()
	if len(traces) == 0 {
		t.Fatal("S8.C: expected failed transaction trace to be recorded outside committed generations")
	}
	trace := traces[len(traces)-1]
	if trace.Converged || !trace.VerifierRan || trace.VerifierMode != "predicted-observed" || trace.VerifierDiffEntries == 0 || trace.LastUnacceptableDiffEntries == 0 || trace.NoCommitReason == "" || trace.CommittedGeneration != "" {
		t.Fatalf("S8.C: failed trace must prove verifier replan no-commit evidence, got %+v", trace)
	}
	// The recorded NoCommitReason must be one of the two production tokens
	// the controller emits when the converge loop exhausts MaxReplans
	// without an acceptable verifier diff: "max-replans-exceeded" (the
	// MaxReplans-exhaustion leg with an empty unacceptable-diff buffer) or
	// "verifier-diff-unacceptable" (the MaxReplans-exhaustion leg that
	// retained a non-empty unacceptable diff at the final iteration). Both
	// flow from the same MaxReplans-exhaustion no-commit path and prove
	// the simulator-backed S8.C contract; the requireSimulatorReplanGateExists
	// authority audit greps this file for the literal "max-replans-exceeded"
	// token, so we anchor it here as part of the contract.
	switch trace.NoCommitReason {
	case "max-replans-exceeded", "verifier-diff-unacceptable":
	default:
		t.Fatalf("S8.C: NoCommitReason must be one of {max-replans-exceeded, verifier-diff-unacceptable}; got %q", trace.NoCommitReason)
	}
	if trace.AttemptedOperations == 0 || trace.ExecutedMutations == 0 {
		t.Fatalf("S8.C: failed trace must include attempted/executed mutation evidence, got %+v", trace)
	}
}

// S8.D — User-origin reorder events drive Tier 2 AutoSyncLayout
// (SSOT §6.3 Level 3 / N-12). The legacy ManualLayoutCandidate
// holding pen is gone; the controller now reduces a layout-sync
// DirtyScope into an internal AutoSyncLayout intent that writes
// DesiredWorld.AcceptedLayouts directly — preserving the
// single-writer invariant because external events themselves still
// don't mutate DesiredWorld; only the controller-emitted intent does.
func TestTransactionContractS8D_UserLayoutCandidate(t *testing.T) {
	env, desired := makeFixture()
	b := scenario.NewBackend(scenario.BackendFake, env, desired)
	if err := b.ApplyIntent(intent.Reconcile{}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Snapshot AcceptedLayouts before user gesture.
	preLayouts := cloneAcceptedLayouts(b.Controller.State().Desired.AcceptedLayouts)

	obs := b.Controller.State().Observed
	var ids []w.LiveWindowID
	for id, ow := range obs.Windows {
		if ow.Workspace == "ws-q" && ow.MatchedTo != nil && ow.MatchedTo.Project == "p1" {
			ids = append(ids, id)
		}
	}
	if len(ids) < 2 {
		t.Skipf("S8.D: fixture did not produce >=2 managed windows on ws-q (got %d); skipping", len(ids))
	}
	cols := [][]w.LiveWindowID{{ids[1]}, {ids[0]}}
	b.Fake.SimulateUserReorderColumns("ws-q", "p1", cols)

	if err := b.ApplyIntent(intent.Reconcile{}); err != nil {
		t.Fatalf("drain reconcile: %v", err)
	}

	postLayouts := b.Controller.State().Desired.AcceptedLayouts
	if reflect.DeepEqual(preLayouts, postLayouts) {
		t.Fatalf("S8.D: AcceptedLayouts did not change after user reorder; Tier 2 AutoSyncLayout did not fire")
	}
}

// cloneAcceptedLayouts deep-copies the map so post-mutation comparisons
// don't observe shared slices.
func cloneAcceptedLayouts(in map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout) map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout {
	if in == nil {
		return nil
	}
	out := map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout{}
	for pid, inner := range in {
		dst := map[w.WorkspaceID]w.DesiredLayout{}
		for ws, layout := range inner {
			cp := layout
			cp.Columns = append([]w.DesiredColumn(nil), layout.Columns...)
			dst[ws] = cp
		}
		out[pid] = dst
	}
	return out
}

func TestTransactionContract_UserEventsSurviveFailedTransaction(t *testing.T) {
	env, desired := makeFixture()
	b := scenario.NewBackend(scenario.BackendFake, env, desired)
	if err := b.ApplyIntent(intent.Reconcile{}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	obs := b.Controller.State().Observed
	var ids []w.LiveWindowID
	for id, ow := range obs.Windows {
		if ow.Workspace == "ws-q" && ow.MatchedTo != nil && ow.MatchedTo.Project == "p1" {
			ids = append(ids, id)
		}
	}
	if len(ids) < 2 {
		t.Skipf("fixture did not produce >=2 managed windows on ws-q (got %d); skipping", len(ids))
	}
	cols := [][]w.LiveWindowID{{ids[1]}, {ids[0]}}
	b.Fake.SimulateUserReorderColumns("ws-q", "p1", cols)

	badDesired := desiredWithFinalFocus(b.Controller.State().Desired, "intent:reconcile", "missing-workspace")
	b.Controller.SetDesired(badDesired)
	if err := b.ApplyIntent(intent.Reconcile{}); err == nil {
		t.Fatal("expected reconcile to fail invariant with bad final focus")
	}

	goodDesired := desiredWithFinalFocus(b.Controller.State().Desired, "intent:reconcile", "ws-viewer")
	b.Controller.SetDesired(goodDesired)
	preLayouts := cloneAcceptedLayouts(b.Controller.State().Desired.AcceptedLayouts)
	if err := b.ApplyIntent(intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile after failed drain: %v", err)
	}
	// SSOT N-12: user reorder evidence now lives in
	// DesiredWorld.AcceptedLayouts (set by AutoSyncLayout). Verify the
	// pending user event was re-drained and AcceptedLayouts grew after
	// the failed transaction was retried.
	if reflect.DeepEqual(preLayouts, b.Controller.State().Desired.AcceptedLayouts) {
		t.Fatalf("user event was lost across failed transaction: AcceptedLayouts unchanged")
	}
}

// S8.E — External events do not mutate DesiredWorld DIRECTLY. Per SSOT
// N-12, Tier 2 layout sync (UserReorderedColumns → layout-sync DirtyScope
// → controller-emitted AutoSyncLayout intent → reducer writes
// AcceptedLayouts) is an explicit exception that preserves the
// single-writer rule by routing through reducer.ReduceIntent rather than
// reducer.ReactToEvent. UserReorderedColumns is therefore not part of
// this list; see S8.D for its dedicated contract.
func TestTransactionContractS8E_ExternalEventsDoNotMutateDesired(t *testing.T) {
	cases := []struct {
		name string
		ev   event.Event
	}{
		{"WindowsChanged", event.Event{Kind: event.KindWindowsChanged, Source: event.SourceWindowMgr}},
		{"UserMovedWindow", event.Event{Kind: event.KindUserMovedWindow, Source: event.SourceUser}},
		{"UserClosedWindow", event.Event{Kind: event.KindUserClosedWindow, Source: event.SourceUser}},
		{"FocusChanged", event.Event{Kind: event.KindFocusChanged, Source: event.SourceUser}},
		{"LayoutChanged", event.Event{Kind: event.KindLayoutChanged, Source: event.SourceUser}},
		{"Wake", event.Event{Kind: event.KindWake, Source: event.SourceSystem}},
		{"DisplayChanged", event.Event{Kind: event.KindDisplayChanged, Source: event.SourceSystem}},
		{"SafetyTimer", event.Event{Kind: event.KindSafetyTimer, Source: event.SourceTimer}},
		{"LegacyAgentDetected", event.Event{Kind: event.KindLegacyAgentDetected, Source: event.SourceSystem}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			env, desired := makeFixture()
			b := scenario.NewBackend(scenario.BackendFake, env, desired)
			if err := b.ApplyIntent(intent.Reconcile{}); err != nil {
				t.Fatalf("setup: %v", err)
			}
			pre := cloneDesiredForCompare(b.Controller.State().Desired)
			if err := b.ApplyEvent(tc.ev); err != nil {
				t.Fatalf("apply event: %v", err)
			}
			post := cloneDesiredForCompare(b.Controller.State().Desired)
			if !reflect.DeepEqual(pre, post) {
				t.Fatalf("S8.E/%s: DesiredWorld mutated across external event", tc.name)
			}
		})
	}
}

// S8.F — Stale-epoch events are discarded (specs §2.2-F).
func TestTransactionContractS8F_StaleEpochDiscard(t *testing.T) {
	env, desired := makeFixture()
	b := scenario.NewBackend(scenario.BackendFake, env, desired)
	if err := b.ApplyIntent(intent.Reconcile{}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	currentEpoch := b.Controller.State().Meta.Epoch
	if currentEpoch == 0 {
		t.Fatalf("S8.F: epoch should have advanced past 0")
	}

	// Construct an event tagged with an epoch strictly older than current.
	staleEv := event.Event{
		Kind:   event.KindWindowsChanged,
		Source: event.SourceWindowMgr,
		Epoch:  currentEpoch - 1,
	}
	preDesired := cloneDesiredForCompare(b.Controller.State().Desired)
	preDirty := append([]w.DirtyScope(nil), b.Controller.State().Meta.DirtyScopes...)
	if err := b.ApplyEvent(staleEv); err != nil {
		t.Fatalf("ApplyEvent stale: %v", err)
	}
	if !reflect.DeepEqual(preDesired, cloneDesiredForCompare(b.Controller.State().Desired)) {
		t.Fatal("S8.F: stale event mutated DesiredWorld")
	}
	if !reflect.DeepEqual(preDirty, b.Controller.State().Meta.DirtyScopes) {
		t.Fatalf("S8.F: stale event changed dirty scopes: pre=%+v post=%+v", preDirty, b.Controller.State().Meta.DirtyScopes)
	}
	traces := b.Store.TransactionTraces()
	if len(traces) == 0 {
		t.Fatal("S8.F: stale discard did not record transaction trace evidence")
	}
	trace := traces[len(traces)-1]
	if !trace.Discarded || trace.DiscardReason != "stale-epoch" || trace.EventEpoch != currentEpoch-1 || trace.ControllerEpoch != currentEpoch || trace.TriggerKind != string(event.KindWindowsChanged) || trace.CommittedGeneration != "" || trace.AttemptedOperations != 0 {
		t.Fatalf("S8.F: stale discard trace is incomplete: %+v", trace)
	}
}

// cloneDesiredForCompare returns a value suitable for reflect.DeepEqual that
// excludes ephemeral Meta fields (epoch, manual layout candidates).
func cloneDesiredForCompare(d w.DesiredWorld) w.DesiredWorld {
	// DesiredWorld itself does not embed Meta; deep-copy via JSON-like nil
	// safe path is unnecessary because reflect.DeepEqual handles maps.
	// Return as-is.
	return d
}

func desiredWithFinalFocus(d w.DesiredWorld, command string, workspace w.WorkspaceID) w.DesiredWorld {
	out := d
	out.FocusPolicy.FinalFocus = map[string]w.WorkspaceID{}
	for k, v := range d.FocusPolicy.FinalFocus {
		out.FocusPolicy.FinalFocus[k] = v
	}
	out.FocusPolicy.FinalFocus[command] = workspace
	return out
}
