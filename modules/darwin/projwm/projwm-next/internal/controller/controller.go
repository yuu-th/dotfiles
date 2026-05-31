// Package controller orchestrates the transaction loop: observe → reduce → plan → execute → settle → verify → commit.
// design.md §12. Single writer; serializes mutations through wmMutationLock.
package controller

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/event"
	"github.com/yuu-th/projwm-next/internal/executor"
	"github.com/yuu-th/projwm-next/internal/identity"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/invariant"
	"github.com/yuu-th/projwm-next/internal/op"
	"github.com/yuu-th/projwm-next/internal/planner"
	"github.com/yuu-th/projwm-next/internal/reducer"
	"github.com/yuu-th/projwm-next/internal/runtimevalidation"
	"github.com/yuu-th/projwm-next/internal/settler"
	"github.com/yuu-th/projwm-next/internal/simulator"
	"github.com/yuu-th/projwm-next/internal/store"
	"github.com/yuu-th/projwm-next/internal/verifier"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// Controller. design.md §12.
type Controller struct {
	wmMutationLock sync.Mutex

	Adapter          wm.Adapter
	Executor         *executor.Executor
	Settler          *settler.Settler
	Store            store.PersistentStore
	RuntimeValidator RuntimeValidator

	// PayloadStore holds URL bodies / cookies for browser tab CRUD (SSOT
	// §4.1 OP14-17 + §4.4 BR-PRIV-NOSTORE). When set, ApplyIntent
	// rewrites BrowserAddTab.URL / BrowserChangeTabURL.URL into opaque
	// tokens before reducer runs, and Forgets refs on BrowserRemoveTab /
	// BrowserChangeTabURL. When nil, falls back to literal storage in
	// URLPayloadRefs (S14 第一段階 — keeps existing tests compiling).
	PayloadStore browser.PrivatePayloadStore

	// State (owned by Controller).
	state w.WorldState

	// UseSimulator: if true, Verifier compares observed against PredictedWorld;
	// if false, Verifier is skipped (fake backend trusts adapter observation).
	UseSimulator bool

	// MaxReplans is the maximum plan→execute retries inside one transaction.
	MaxReplans int

	// LastDiff is set by the simulator-backed Verifier on each iteration; surfaced
	// for tests and diagnostics. specs §2-C.
	LastDiff verifier.WorldDiff

	// OnBroadcast, if non-nil, is invoked after each commit / card change
	// so the daemon can fan-out subscription pushes. The string is a
	// kind identifier ("card-added", "card-removed", "generation-committed")
	// and the payload is kind-specific.
	OnBroadcast func(kind string, payload any, generation string)
}

type RuntimeValidator interface {
	ValidateEnvironment(context.Context, w.ManagedEnvironment) ([]store.RuntimeValidationReport, bool, error)
}

// TransactionResult is request-scoped transaction evidence. Callers must use
// this return value rather than shared "last transaction" controller state, so
// concurrent IPC handlers cannot mix responses across requests.
type TransactionResult struct {
	TransactionID       w.TransactionID
	CommittedGeneration w.GenerationID
	Trace               store.TransactionTrace
}

// New constructs a Controller with initial state.
func New(env w.ManagedEnvironment, desired w.DesiredWorld, adapter wm.Adapter, st store.PersistentStore) *Controller {
	c := &Controller{
		Adapter:          adapter,
		Executor:         &executor.Executor{Adapter: adapter, Env: env},
		Settler:          &settler.Settler{Adapter: adapter},
		Store:            st,
		RuntimeValidator: runtimevalidation.NewLaunchctlValidator(),
		MaxReplans:       4,
		state: w.WorldState{
			Environment: env,
			Desired:     desired,
			Meta:        w.ControllerMeta{Epoch: 1},
		},
	}
	return c
}

// NewFromGeneration restores Controller-owned durable checkpoint metadata from
// the current PersistentStore generation. Predicted/Observed remain process-local.
func NewFromGeneration(env w.ManagedEnvironment, gen store.CommittedGeneration, adapter wm.Adapter, st store.PersistentStore) *Controller {
	c := New(env, gen.Desired, adapter, st)
	c.state.Meta.Epoch = gen.Checkpoint.Epoch
	c.state.Meta.DirtyScopes = append([]w.DirtyScope(nil), gen.Checkpoint.DirtyScopes...)
	// SSOT §6.9.1 G1: restore the persisted provenance cache so a daemon
	// restart re-matches its live windows instead of respawning. The on-disk
	// form is a slice (struct-keyed maps are not JSON-marshalable); rebuild the
	// runtime map.
	c.state.Meta.WindowProvenance = store.ProvenanceMapFromEntries(gen.Checkpoint.WindowProvenance)
	return c
}

// State returns a (shallow) snapshot of current WorldState.
func (c *Controller) State() w.WorldState { return c.state }

// SetDesired replaces the DesiredWorld (used for fixture initialization).
func (c *Controller) SetDesired(d w.DesiredWorld) { c.state.Desired = d }

// SetUseSimulator toggles simulator-backed Verifier.
func (c *Controller) SetUseSimulator(b bool) { c.UseSimulator = b }

// ApplyIntent runs the full transaction loop for a user intent. design.md §12.
func (c *Controller) ApplyIntent(ctx context.Context, in intent.Intent) (TransactionResult, error) {
	c.wmMutationLock.Lock()
	defer c.wmMutationLock.Unlock()

	trace := earlyIntentTrace(in)
	reports, blocking, err := c.validateRuntimeEnvironment(ctx)
	if err != nil {
		return c.recordEarlyNoCommitTrace(ctx, trace, "runtime-validation-error", err, false)
	}
	trace.RuntimeValidationReports = reports
	trace.RuntimeValidationBlocking = blocking
	if blocking && in.Kind() != intent.KindValidateEnvironment {
		return c.recordEarlyNoCommitTrace(ctx, trace, "runtime-validation-blocked", fmt.Errorf("controller: runtime validation blocked %s because a remove-policy legacy writer is active", in.Kind()), false)
	}

	rollback := c.snapshotRollbackState()
	// 1. Observe latest world.
	if err := c.observe(ctx); err != nil {
		result, recordErr := c.recordEarlyNoCommitTrace(ctx, trace, "observe-error", err, true)
		c.markGlobalDirty("observation-refresh-failed")
		return result, recordErr
	}
	// 2. Drain any user-origin events from the adapter and feed through Reducer.
	ackUserEvents := c.drainUserEvents()
	// SSOT N-12: AcceptManualLayout / ManualLayoutCandidate are deprecated;
	// Tier 2 layout sync is handled via AutoSyncLayout intent emitted by
	// applyTier2AutoSyncLayout after reduce.

	// 2b. Browser tab CRUD: Put URL into PrivatePayloadStore and replace
	// intent.URL with the opaque token before reducer (SSOT §4.1 OP14-17 +
	// §4.4 BR-PRIV-NOSTORE). On BrowserRemoveTab / BrowserChangeTabURL also
	// Forget the old ref. Reducer stays pure (no store I/O).
	in, err = c.prepareBrowserIntent(ctx, in)
	if err != nil {
		result, recordErr := c.recordEarlyNoCommitTrace(ctx, trace, "payload-store-error", err, false)
		c.restoreRollbackState(rollback)
		return result, recordErr
	}

	// 3. Reduce intent → new DesiredWorld.
	preReduceDesired := c.state.Desired
	newDesired, err := reducer.ReduceIntent(c.state, in)
	if err != nil {
		result, recordErr := c.recordEarlyNoCommitTrace(ctx, trace, "reducer-error", err, false)
		c.restoreRollbackState(rollback)
		return result, recordErr
	}
	c.state.Desired = newDesired

	// SSOT §6.9.1 E1/E2/E3/E5: an intent that removes a project/window or
	// switches/unassigns a profile drops desired identities. Their provenance
	// entries must clear as a consequence of the intent — independent of
	// whether the converge loop later succeeds (the close path may have an
	// unrelated gap). Prune against the freshly reduced desired set, and
	// (below) preserve the result across a converge rollback.
	c.pruneProvenanceInactive()

	// 3a. ControllerMeta-only intents (DismissCard / DismissAllCards) act
	// on the in-memory cards list, not DesiredWorld. Applied after the
	// reducer because reducer.ReduceIntent intentionally never sees
	// ControllerMeta — it's pure.
	c.applyCardIntent(in)

	// 3b. SSOT N-12 Tier 2 layout sync: process layout-sync DirtyScopes
	// drained from user-reordered-columns events. Mirrors the ApplyEvent
	// path so the user's manual column rearrange always converges to
	// DesiredWorld.AcceptedLayouts before plan/execute, regardless of
	// whether the kicking transaction came in via an intent or an event.
	c.applyTier2AutoSyncLayout()

	commandKey := commandKeyForIntent(in)
	trace.Command = commandKey
	trace.Reason = string(op.ReasonIntent)
	trace.TriggerSource = "user"
	trace.TriggerKind = string(in.Kind())
	result, err := c.runConvergeLoop(ctx, commandKey, op.ReasonIntent, trace)
	if err != nil {
		c.restoreRollbackState(rollback)
		if result.Trace.ObservationRefreshFailed {
			c.markGlobalDirty("observation-refresh-failed")
		}
	}
	if err == nil {
		// Commit succeeded — safe to GC orphan PrivatePayloadStore refs
		// (SSOT §4.4 BR-PRIV-NOSTORE + §4.5 ARCHIVE). Done post-commit so a
		// rollback (executor failure) does not leave Forget-only files +
		// re-instated DesiredWorld out of sync.
		c.forgetOrphanedBrowserPayloads(ctx, preReduceDesired, c.state.Desired)
		if ackUserEvents != nil {
			ackUserEvents()
		}
	}
	return result, err
}

// ApplyEvent runs a transaction triggered by an external event.
// Lifecycle events become reasons; user events translate via reducer. design.md §12.
func (c *Controller) ApplyEvent(ctx context.Context, ev event.Event) (TransactionResult, error) {
	c.wmMutationLock.Lock()
	defer c.wmMutationLock.Unlock()

	controllerEpoch := c.state.Meta.Epoch
	if isStaleEvent(ev, controllerEpoch) {
		return c.recordDiscardedEventTrace(ctx, ev, controllerEpoch, "stale-epoch")
	}
	trace := earlyEventTrace(ev, op.ReasonEvent)
	reports, blocking, err := c.validateRuntimeEnvironment(ctx)
	if err != nil {
		result, recordErr := c.recordEarlyNoCommitTrace(ctx, trace, "runtime-validation-error", err, false)
		return result, recordErr
	}
	trace.RuntimeValidationReports = reports
	trace.RuntimeValidationBlocking = blocking
	if blocking {
		result, recordErr := c.recordEarlyNoCommitTrace(ctx, trace, "runtime-validation-blocked", fmt.Errorf("controller: runtime validation blocked %s because a remove-policy legacy writer is active", ev.Kind), false)
		return result, recordErr
	}
	rollback := c.snapshotRollbackState()
	if err := c.observe(ctx); err != nil {
		result, recordErr := c.recordEarlyNoCommitTrace(ctx, trace, "observe-error", err, true)
		c.markGlobalDirty("observation-refresh-failed")
		return result, recordErr
	}
	ackUserEvents := c.drainUserEvents()

	r, err := reducer.ReactToEvent(c.state, ev)
	if err != nil {
		result, recordErr := c.recordEarlyNoCommitTrace(ctx, trace, "reducer-error", err, false)
		c.restoreRollbackState(rollback)
		return result, recordErr
	}
	if r.Discard {
		c.restoreRollbackState(rollback)
		return c.recordDiscardedEventTrace(ctx, ev, controllerEpoch, "reducer-discard")
	}
	c.state.Meta.DirtyScopes = append(c.state.Meta.DirtyScopes, r.DirtyScopes...)
	c.appendActiveCards(r.NewCards)
	c.state.Meta.PendingOrphans = append(c.state.Meta.PendingOrphans, r.OrphanAdds...)
	c.absorbUserCloseRecords(r.UserCloseRecords)

	// Tier 2 single-writer dispatch (design v3 §3.5): for each layout-sync
	// DirtyScope emitted by the reducer, immediately apply an internal
	// AutoSyncLayout intent against the same WorldState so DesiredWorld
	// is updated before the converge loop plans. This keeps the
	// invariant "DesiredWorld is only written by intents" intact —
	// external events still don't mutate it, only the controller does.
	c.applyTier2AutoSyncLayout()
	// Cockpit (unified design v1 §4): Bootstrap / Wake / DisplayChanged
	// events emit a cockpit-sync DirtyScope. Apply the internal
	// SyncCockpitSystemWindows intent so SystemWindows length tracks
	// display count.
	c.applyCockpitSync()
	// Cockpit visibility bidirectional sync (v2.7 §8.3.1): when the
	// reducer notices observed.activeWorkspace and Desired.Visibility
	// drift (e.g., user pressed space+1 to leave CP1 while Visibility
	// was Shown), apply an internal SetCockpitVisibility intent so
	// DesiredWorld follows the user's manual workspace switch. This
	// makes every workspace-move binding act as an implicit cockpit
	// toggle, and stops the planner from re-emitting ShowCockpit ops
	// that fight the user.
	c.applyCockpitVisibilitySync()
	// SSOT §3.5 case B/D / INV-10: on Bootstrap, re-register managed windows
	// observed in the live world whose identity is absent from DesiredWorld
	// (state lost/corrupted with no backup, or a parseable-title orphan).
	// Single-writer: the startup event signals Bootstrap; the controller
	// converts it into the internal ReconstructFromObserved intent so only
	// the controller writes DesiredWorld.
	if r.Lifecycle == w.LifecycleBootstrap {
		c.applyStartupReconstruction()
	}

	commandKey := commandKeyForLifecycle(r.Lifecycle)
	reason := op.ReasonEvent
	if r.Lifecycle != w.LifecycleNone {
		reason = op.ReasonLifecycle
	}
	trace.Command = commandKey
	trace.Reason = string(reason)
	trace.TriggerSource = string(ev.Source)
	trace.TriggerKind = string(ev.Kind)
	result, err := c.runConvergeLoop(ctx, commandKey, reason, trace)
	if err != nil {
		c.restoreRollbackState(rollback)
		if result.Trace.ObservationRefreshFailed {
			c.markGlobalDirty("observation-refresh-failed")
		}
	}
	if err == nil && ackUserEvents != nil {
		ackUserEvents()
	}
	return result, err
}

func isStaleEvent(ev event.Event, controllerEpoch w.Epoch) bool {
	return ev.Epoch != 0 && ev.Epoch < controllerEpoch
}

func earlyIntentTrace(in intent.Intent) store.TransactionTrace {
	return store.TransactionTrace{
		Command:       commandKeyForIntent(in),
		Reason:        string(op.ReasonIntent),
		TriggerSource: "user",
		TriggerKind:   string(in.Kind()),
	}
}

func earlyEventTrace(ev event.Event, reason op.PlanReason) store.TransactionTrace {
	return store.TransactionTrace{
		Command:       commandKeyForLifecycle(w.LifecycleNone),
		Reason:        string(reason),
		TriggerSource: string(ev.Source),
		TriggerKind:   string(ev.Kind),
		EventID:       ev.ID,
		EventEpoch:    ev.Epoch,
	}
}

func (c *Controller) validateRuntimeEnvironment(ctx context.Context) ([]store.RuntimeValidationReport, bool, error) {
	if c.RuntimeValidator == nil {
		return nil, false, nil
	}
	return c.RuntimeValidator.ValidateEnvironment(ctx, c.state.Environment)
}

func (c *Controller) recordDiscardedEventTrace(ctx context.Context, ev event.Event, controllerEpoch w.Epoch, reason string) (TransactionResult, error) {
	txn := w.TransactionID(fmt.Sprintf("txn-%d", time.Now().UnixNano()))
	now := time.Now().UTC().Format(time.RFC3339Nano)
	trace := store.TransactionTrace{
		TransactionID:   txn,
		Reason:          string(op.ReasonEvent),
		TriggerSource:   string(ev.Source),
		TriggerKind:     string(ev.Kind),
		EventID:         ev.ID,
		EventEpoch:      ev.Epoch,
		ControllerEpoch: controllerEpoch,
		StartedAt:       now,
		FinishedAt:      now,
		Discarded:       true,
		DiscardReason:   reason,
		NoCommitReason:  reason,
	}
	if gen, err := c.currentGeneration(ctx); err == nil {
		trace.CurrentGeneration = gen
		trace.ParentGeneration = gen
	} else {
		return TransactionResult{}, err
	}
	if err := c.recordTransactionTrace(ctx, trace); err != nil {
		return TransactionResult{}, fmt.Errorf("controller: record discarded event trace: %w", err)
	}
	return TransactionResult{TransactionID: txn, Trace: trace}, nil
}

// ReplanExceededError indicates the converge loop hit MaxReplans without
// reaching a state where Planner has nothing to do (specs §2-C).
// Commit MUST NOT happen when this is returned.
type ReplanExceededError struct {
	MaxReplans int
	LastDiff   verifier.WorldDiff
	LastPlan   op.Plan
}

func (e *ReplanExceededError) Error() string {
	kinds := make([]string, 0, len(e.LastPlan.Operations))
	for _, oper := range e.LastPlan.Operations {
		target := ""
		if oper.Target.Workspace != nil {
			target = fmt.Sprintf(":%s", *oper.Target.Workspace)
		}
		kinds = append(kinds, fmt.Sprintf("%s%s", oper.Kind, target))
	}
	return fmt.Sprintf("controller: failed to converge after %d replans (last diff entries=%d %v, last ops=%v)", e.MaxReplans, len(e.LastDiff.Entries), e.LastDiff.Entries, kinds)
}

func (c *Controller) runConvergeLoop(ctx context.Context, command string, reason op.PlanReason, trace store.TransactionTrace) (TransactionResult, error) {
	txn := w.TransactionID(fmt.Sprintf("txn-%d", time.Now().UnixNano()))
	c.state.Meta.Transaction = &txn
	// Freeze the focus as observed when this transaction began. summon/jump/
	// cycle planning reads this anchor for its "cycle to the next window"
	// decision so replans don't keep re-cycling off the focus we just set.
	c.state.Meta.SummonFocusAnchor = c.state.Observed.Focus.Window
	trace.TransactionID = txn
	if trace.Command == "" {
		trace.Command = command
	}
	if trace.Reason == "" {
		trace.Reason = string(reason)
	}
	trace.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
	trace.VerifierMode = "self-diff-diagnostic"
	if c.UseSimulator {
		trace.VerifierMode = "predicted-observed"
	}
	defer func() {
		c.state.Meta.Transaction = nil
		c.state.Meta.SummonFocusAnchor = ""
	}()

	maxIter := c.MaxReplans
	if maxIter < 1 {
		maxIter = 1
	}
	var lastDiff verifier.WorldDiff
	var lastUnacceptable verifier.WorldDiff
	converged := false
	for i := 0; i < maxIter; i++ {
		plan, err := planner.Plan(c.state, c.state.Desired, planner.CommandKey(command), reason)
		if err != nil {
			return c.failNoCommitTrace(ctx, trace, "planner-error", err, lastDiff)
		}
		iterTrace := planTrace(i, plan)
		trace.TotalOperations += iterTrace.PlannedOperations
		trace.MutationOperations += iterTrace.MutationOperations
		if len(plan.Operations) == 0 {
			if c.UseSimulator {
				predicted := simulator.FromObserved(c.state.Observed, c.state.Meta.Epoch)
				lastDiff = verifier.Diff(predicted.ObservedWorld, c.state.Observed)
				c.LastDiff = lastDiff
				trace.VerifierRan = true
				trace.VerifierDiffEntries = len(lastDiff.Entries)
				iterTrace.VerifierRan = true
				iterTrace.VerifierDiffEntries = len(lastDiff.Entries)
				if !lastDiff.Empty() {
					lastUnacceptable = lastDiff
					trace.LastUnacceptableDiffEntries = len(lastUnacceptable.Entries)
				}
			}
			trace.PlanIterations = append(trace.PlanIterations, iterTrace)
			converged = len(lastUnacceptable.Entries) == 0
			break
		}

		// Build PredictedWorld from current observation, evolve via simulator per op.
		predicted := simulator.FromObserved(c.state.Observed, c.state.Meta.Epoch)
		for _, oper := range plan.Operations {
			predicted, err = simulator.Apply(predicted, oper)
			if err != nil {
				trace.PlanIterations = append(trace.PlanIterations, iterTrace)
				return c.failNoCommitTrace(ctx, trace, "simulator-error", err, lastDiff)
			}
		}

		// Execute each op; refresh observation between mutations.
		for idx, oper := range plan.Operations {
			iterTrace.AttemptedOperations++
			trace.AttemptedOperations++
			iterTrace.Operations[idx].Attempted = true
			iterTrace.Operations[idx].StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
			// SSOT §6.9.1 C1: forward the up-to-date provenance cache so the
			// semop pre-spawn check excludes a sibling editor's colliding
			// same-title window when spawning the next editor of the same
			// project. Refreshed per-op because captureProvenance records the
			// just-spawned sibling before the next op runs.
			c.Executor.Provenance = c.state.Meta.WindowProvenance
			if err := c.Executor.Execute(ctx, oper, c.state.Observed, c.state.Desired); err != nil {
				iterTrace.Operations[idx].FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
				// SSOT §6.8 graceful degradation: a single window spawn
				// failure must NOT abort the whole transaction. Surface a
				// per-window [INVARIANT] card (§6.8 bullet 3), refresh
				// observation, and continue with the remaining ops so other
				// windows still spawn (bullet 1). The still-missing window
				// keeps generating a spawn op next iteration, so it rejoins
				// the replan path (bullet 2) and, if it never recovers, the
				// converge loop falls through to the §7.1 max-replans path.
				// Removal/layout failures keep hard-abort because §6.10
				// ordering depends on them.
				if isDegradableSpawn(oper.Kind) {
					c.appendActiveCards([]w.Card{spawnFailureCard(oper, err)})
					if obs, oerr := c.Settler.Settle(ctx); oerr == nil {
						c.state.Observed = identity.PopulateMatchedToWithProvenance(c.state.Desired, obs, c.state.Meta.WindowProvenance)
					}
					continue
				}
				trace.PlanIterations = append(trace.PlanIterations, iterTrace)
				return c.failNoCommitTrace(ctx, trace, "executor-error", fmt.Errorf("controller: op %s: %w", oper.ID, err), lastDiff)
			}
			iterTrace.Operations[idx].FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if isMutationOperation(oper.Kind) {
				iterTrace.ExecutedMutations++
				trace.ExecutedMutations++
				iterTrace.Operations[idx].Executed = true
			}
			obs, oerr := c.Settler.Settle(ctx)
			if oerr != nil {
				trace.PlanIterations = append(trace.PlanIterations, iterTrace)
				return c.failNoCommitTrace(ctx, trace, "settler-error", oerr, lastDiff)
			}
			c.state.Observed = identity.PopulateMatchedToWithProvenance(c.state.Desired, obs, c.state.Meta.WindowProvenance)
			// SSOT §6.9.1 CAPTURE: after a successful spawn (or adopt-by-move
			// into a slot), record which live window we produced for the op's
			// target identity. Subsequent resolves prefer this window over a
			// colliding same-title sibling. Guarded against double-record (A6).
			c.captureProvenance(oper)
		}

		// Final settle.
		obs, err := c.Settler.Settle(ctx)
		if err != nil {
			trace.PlanIterations = append(trace.PlanIterations, iterTrace)
			return c.failNoCommitTrace(ctx, trace, "settler-error", err, lastDiff)
		}
		c.state.Observed = identity.PopulateMatchedToWithProvenance(c.state.Desired, obs, c.state.Meta.WindowProvenance)
		c.captureSlotAdoptions()
		// SSOT §6.9 / §6.9.1 validated-cache prune: drop any provenance entry
		// whose desired identity is no longer an active (non-archived,
		// slot-assigned) desired window, or whose remembered live ID is no
		// longer observed. Self-heals stale entries and clears on
		// archive/remove/profile-switch/unassign (E1/E2/E3/E5).
		c.pruneProvenance()

		// Verifier gates the transaction (specs §2-C). If predicted vs observed
		// diverge, replan the next iteration. Skipped for fake backend, which
		// trusts adapter observation as authoritative.
		if c.UseSimulator {
			trace.VerifierRan = true
			iterTrace.VerifierRan = true
			lastDiff = verifier.Diff(predicted.ObservedWorld, c.state.Observed)
			c.LastDiff = lastDiff
			iterTrace.VerifierDiffEntries = len(lastDiff.Entries)
			trace.VerifierDiffEntries = len(lastDiff.Entries)
			if lastDiff.Empty() {
				lastUnacceptable = verifier.WorldDiff{}
				trace.LastUnacceptableDiffEntries = 0
			} else {
				lastUnacceptable = lastDiff
				trace.LastUnacceptableDiffEntries = len(lastUnacceptable.Entries)
			}
			// Either empty (success) or non-empty (fall through to next iter).
		}
		trace.PlanIterations = append(trace.PlanIterations, iterTrace)
		// Next iteration: planner re-evaluates (WorldState, DesiredWorld). Loop ends
		// when planner returns 0 ops.
	}

	var lastPlan op.Plan
	if !converged {
		plan, err := planner.Plan(c.state, c.state.Desired, planner.CommandKey(command), reason)
		if err != nil {
			return c.failNoCommitTrace(ctx, trace, "planner-error", err, lastDiff)
		}
		lastPlan = plan
		if len(plan.Operations) == 0 {
			if c.UseSimulator {
				lastDiff = verifier.Diff(simulator.FromObserved(c.state.Observed, c.state.Meta.Epoch).ObservedWorld, c.state.Observed)
				c.LastDiff = lastDiff
				trace.VerifierRan = true
				trace.VerifierDiffEntries = len(lastDiff.Entries)
				if lastDiff.Empty() {
					lastUnacceptable = verifier.WorldDiff{}
					trace.LastUnacceptableDiffEntries = 0
				}
			}
			converged = true
		}
	}

	if !converged || len(lastUnacceptable.Entries) > 0 {
		// Specs §2-C / SSOT §7.1 max replans 超過時の 4 挙動:
		//   1. commit されない (here)
		//   2. ApplyIntent caller が rollback (restoreRollbackState)
		//   3. cockpit に [INVARIANT] カード通知 (emit below)
		//   4. dirty scope 記録 + 次 intent で再挑戦 (markGlobalDirty)
		trace.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		trace.VerifierDiffEntries = len(lastDiff.Entries)
		trace.LastUnacceptableDiffEntries = len(lastUnacceptable.Entries)
		trace.NoCommitReason = "max-replans-exceeded"
		if len(lastUnacceptable.Entries) > 0 {
			trace.NoCommitReason = "verifier-diff-unacceptable"
			lastDiff = lastUnacceptable
		}
		// SSOT §7.1 step 3: surface a [INVARIANT] card so the user
		// sees why the transaction did not commit. Subject mentions
		// the transaction command + replan exhaustion so the cockpit
		// modal carries actionable context.
		c.appendActiveCards([]w.Card{{
			Type:    w.CardTypeInvariant,
			Subject: fmt.Sprintf("transaction %q did not converge after %d replans", command, maxIter),
			Context: map[string]string{
				"reason":      trace.NoCommitReason,
				"command":     command,
				"diffEntries": fmt.Sprintf("%d", len(lastDiff.Entries)),
				"maxReplans":  fmt.Sprintf("%d", maxIter),
			},
			Actions: []w.CardAction{
				{Key: "Enter", Label: "acknowledge"},
				{Key: "Esc", Label: "dismiss"},
			},
		}})
		// SSOT §7.1 step 4: record a global dirty scope so the next
		// intent / event forces a fresh observe + plan cycle.
		c.markGlobalDirty("max-replans-exceeded")
		if gen, err := c.currentGeneration(ctx); err == nil {
			trace.CurrentGeneration = gen
			trace.ParentGeneration = gen
		} else {
			return TransactionResult{}, err
		}
		if err := c.recordTransactionTrace(ctx, trace); err != nil {
			return TransactionResult{}, fmt.Errorf("controller: record failed transaction trace: %w", err)
		}
		return TransactionResult{TransactionID: txn, Trace: trace}, &ReplanExceededError{MaxReplans: maxIter, LastDiff: lastDiff, LastPlan: lastPlan}
	}
	trace.Converged = true
	if !trace.VerifierRan {
		lastDiff = verifier.Diff(c.state.Observed, c.state.Observed)
		c.LastDiff = lastDiff
		trace.VerifierDiffEntries = len(lastDiff.Entries)
		if len(trace.PlanIterations) > 0 {
			last := len(trace.PlanIterations) - 1
			trace.PlanIterations[last].VerifierDiffEntries = len(lastDiff.Entries)
		}
	}

	// Clear processed dirty scopes after successful convergence.
	c.state.Meta.DirtyScopes = nil
	c.state.Meta.Epoch++

	// Run invariants. SSOT N-12: AllowManualLayoutCandidates removed.
	_ = reason
	vs := invariant.CheckAll(c.state, invariant.CheckOptions{
		FinalFocusCommandKey: command,
	})
	if len(vs) > 0 {
		// Emit one [INVARIANT] cockpit card per violation so the user
		// gets surfaced notice (requirements E2.3).
		invariantCards := make([]w.Card, 0, len(vs))
		for _, v := range vs {
			trace.InvariantViolations = append(trace.InvariantViolations, v.Error())
			invariantCards = append(invariantCards, w.Card{
				Type:    w.CardTypeInvariant,
				Subject: "invariant violation",
				Context: map[string]string{"detail": v.Error()},
				Actions: []w.CardAction{
					{Key: "Enter", Label: "show details"},
					{Key: "Esc", Label: "dismiss"},
				},
			})
		}
		c.appendActiveCards(invariantCards)
		trace.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		trace.VerifierDiffEntries = len(lastDiff.Entries)
		trace.NoCommitReason = "invariant-violation"
		if gen, err := c.currentGeneration(ctx); err == nil {
			trace.CurrentGeneration = gen
			trace.ParentGeneration = gen
		}
		if err := c.recordTransactionTrace(ctx, trace); err != nil {
			return TransactionResult{}, fmt.Errorf("controller: record invariant failure trace: %w", err)
		}
		// Return only the first violation for clarity.
		return TransactionResult{TransactionID: txn, Trace: trace}, vs[0]
	}

	// Commit a checkpoint.
	result := TransactionResult{TransactionID: txn, Trace: trace}
	if c.Store != nil {
		gen, err := c.Store.LoadCurrentGeneration(ctx)
		if err != nil {
			return c.failNoCommitTrace(ctx, trace, "store-load-error", fmt.Errorf("controller: load current generation before commit: %w", err), lastDiff)
		}
		trace.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		trace.VerifierDiffEntries = len(lastDiff.Entries)
		commit := store.ControllerCommit{
			TransactionID:   txn,
			Parent:          gen.ID,
			Desired:         c.state.Desired,
			AcceptedLayouts: c.state.Desired.AcceptedLayouts,
			Checkpoint: store.ControllerCheckpoint{
				Epoch:            c.state.Meta.Epoch,
				LastClean:        &txn,
				WindowProvenance: store.ProvenanceEntriesFromMap(c.state.Meta.WindowProvenance),
			},
			Trace: trace,
		}
		staged, err := c.Store.BeginCommit(ctx, commit)
		if err != nil {
			return c.failNoCommitTrace(ctx, trace, "store-begin-error", fmt.Errorf("controller: begin commit: %w", err), lastDiff)
		}
		committed, err := c.Store.Commit(ctx, staged)
		if err != nil {
			_ = c.Store.Abort(ctx, staged)
			return c.failNoCommitTrace(ctx, trace, "store-commit-error", fmt.Errorf("controller: commit generation: %w", err), lastDiff)
		}
		trace.CommittedGeneration = committed
		trace.ParentGeneration = gen.ID
		trace.CommitKind = "controller-commit"
		trace.CommittedBy = "controller"
		result = TransactionResult{TransactionID: txn, CommittedGeneration: committed, Trace: trace}
	}
	return result, nil
}

func (c *Controller) recordTransactionTrace(ctx context.Context, trace store.TransactionTrace) error {
	recorder, ok := c.Store.(store.TransactionTraceRecorder)
	if !ok || recorder == nil {
		return nil
	}
	return recorder.RecordTransactionTrace(ctx, trace)
}

func (c *Controller) failNoCommitTrace(ctx context.Context, trace store.TransactionTrace, reason string, cause error, lastDiff verifier.WorldDiff) (TransactionResult, error) {
	if err := c.bestEffortObserve(ctx); err != nil {
		trace.ObservationRefreshFailed = true
		trace.ObservationRefreshError = err.Error()
	}
	trace.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	trace.NoCommitReason = reason
	trace.VerifierDiffEntries = len(lastDiff.Entries)
	if len(lastDiff.Entries) > 0 {
		trace.LastUnacceptableDiffEntries = len(lastDiff.Entries)
	}
	if gen, err := c.currentGeneration(ctx); err == nil {
		trace.CurrentGeneration = gen
		trace.ParentGeneration = gen
	}
	// SSOT §7.1 fail 時の user 通知 + 次回 retry trigger:
	//   step 3 — surface an [INVARIANT] card with the fail reason
	//   step 4 — record a global dirty scope so the next intent retries
	// The card and scope ride out of the rollback via the
	// restoreRollbackState carve-out (ActiveCards + DirtyScopes are
	// preserved across rollback).
	c.appendActiveCards([]w.Card{{
		Type:    w.CardTypeInvariant,
		Subject: fmt.Sprintf("transaction did not commit: %s", reason),
		Context: map[string]string{
			"reason":      reason,
			"diffEntries": fmt.Sprintf("%d", len(lastDiff.Entries)),
			"cause":       cause.Error(),
		},
		Actions: []w.CardAction{
			{Key: "Enter", Label: "acknowledge"},
			{Key: "Esc", Label: "dismiss"},
		},
	}})
	c.markGlobalDirty("commit-fail:" + reason)
	if err := c.recordTransactionTrace(ctx, trace); err != nil {
		return TransactionResult{TransactionID: trace.TransactionID, Trace: trace}, fmt.Errorf("%w; additionally failed to record no-commit trace: %v", cause, err)
	}
	return TransactionResult{TransactionID: trace.TransactionID, Trace: trace}, cause
}

func (c *Controller) recordEarlyNoCommitTrace(ctx context.Context, trace store.TransactionTrace, reason string, cause error, observationFailed bool) (TransactionResult, error) {
	trace.TransactionID = w.TransactionID(fmt.Sprintf("txn-%d", time.Now().UnixNano()))
	now := time.Now().UTC().Format(time.RFC3339Nano)
	trace.StartedAt = now
	trace.FinishedAt = now
	trace.VerifierMode = "not-started"
	trace.NoCommitReason = reason
	trace.ObservationRefreshFailed = observationFailed
	if observationFailed {
		trace.ObservationRefreshError = cause.Error()
	}
	if gen, err := c.currentGeneration(ctx); err == nil {
		trace.CurrentGeneration = gen
		trace.ParentGeneration = gen
	}
	if err := c.recordTransactionTrace(ctx, trace); err != nil {
		return TransactionResult{TransactionID: trace.TransactionID, Trace: trace}, fmt.Errorf("%w; additionally failed to record no-commit trace: %v", cause, err)
	}
	return TransactionResult{TransactionID: trace.TransactionID, Trace: trace}, cause
}

func (c *Controller) bestEffortObserve(ctx context.Context) error {
	return c.observe(ctx)
}

func (c *Controller) markGlobalDirty(key string) {
	scope := w.DirtyScope{Kind: "global", Key: key}
	for _, existing := range c.state.Meta.DirtyScopes {
		if existing == scope {
			return
		}
	}
	c.state.Meta.DirtyScopes = append(c.state.Meta.DirtyScopes, scope)
}

func (c *Controller) currentGeneration(ctx context.Context) (w.GenerationID, error) {
	if c.Store == nil {
		return "", nil
	}
	gen, err := c.Store.LoadCurrentGeneration(ctx)
	if err != nil {
		return "", fmt.Errorf("controller: load current generation: %w", err)
	}
	return gen.ID, nil
}

type rollbackState struct {
	desired w.DesiredWorld
	meta    w.ControllerMeta
}

func (c *Controller) snapshotRollbackState() rollbackState {
	return rollbackState{
		desired: cloneDesiredWorld(c.state.Desired),
		meta:    cloneControllerMeta(c.state.Meta),
	}
}

func (c *Controller) restoreRollbackState(s rollbackState) {
	c.state.Desired = cloneDesiredWorld(s.desired)
	// SSOT §7.1 carve-out: rollback restores Desired + most Meta
	// fields, but it MUST NOT erase user-facing notifications or the
	// post-failure work record:
	//   - ActiveCards: emitted by the failed transaction to inform the
	//     user (max-replans [INVARIANT] card, manifest-mismatch card,
	//     omniwm-recovery card). Wiping them on rollback would hide
	//     the very signal the user needs to act on.
	//   - DirtyScopes: SSOT §7.1 step 4 demands the next intent/event
	//     re-tries the same scope. Erasing the scope on rollback
	//     defeats that contract.
	preservedCards := append([]w.Card(nil), c.state.Meta.ActiveCards...)
	preservedDirty := append([]w.DirtyScope(nil), c.state.Meta.DirtyScopes...)
	// SSOT §6.9.1 carve-out: WindowProvenance is a validated cache, and the
	// intent-driven clear (pruneProvenanceInactive) of identities removed from
	// the desired set must survive a converge rollback (E1: the close path may
	// error, but the user's intent to remove the window must still clear its
	// provenance). Captures that get rolled back are self-healed on the next
	// observe's pruneProvenance (the live ID won't validate) — so preserving the
	// current map is safe.
	preservedProvenance := c.state.Meta.WindowProvenance
	c.state.Meta = cloneControllerMeta(s.meta)
	c.state.Meta.ActiveCards = preservedCards
	c.state.Meta.DirtyScopes = preservedDirty
	c.state.Meta.WindowProvenance = preservedProvenance
}

func cloneControllerMeta(meta w.ControllerMeta) w.ControllerMeta {
	out := meta
	if meta.Transaction != nil {
		txn := *meta.Transaction
		out.Transaction = &txn
	}
	out.PendingEvents = append([]w.EventID(nil), meta.PendingEvents...)
	out.DirtyScopes = append([]w.DirtyScope(nil), meta.DirtyScopes...)
	out.ActiveCards = append([]w.Card(nil), meta.ActiveCards...)
	out.PendingOrphans = append([]w.OrphanCandidate(nil), meta.PendingOrphans...)
	// Deep-clone WindowProvenance so a rollback snapshot is owner-independent
	// (SSOT §6.9.1 §5 CLONE — rollback safety; shallow copy would alias the
	// live map and a rolled-back transaction's capture would leak back).
	if meta.WindowProvenance != nil {
		prov := make(map[w.DesiredWindowID]w.LiveWindowID, len(meta.WindowProvenance))
		for k, v := range meta.WindowProvenance {
			prov[k] = v
		}
		out.WindowProvenance = prov
	}
	return out
}

// captureProvenance records the live window produced by a successful spawn (or
// adopt-by-move into a slot) for the op's target DesiredWindow identity (SSOT
// §6.9.1 CAPTURE). It resolves the target against the fresh observation and, on
// UniqueStrong, stamps WindowProvenance[target] = live — but only if absent or
// changed, so an idempotent re-reconcile does not churn the entry (A6).
func (c *Controller) captureProvenance(oper op.Operation) {
	switch oper.Kind {
	case op.KindSpawnEditor, op.KindSpawnTerminal, op.KindSpawnBrowser, op.KindSpawnViewer,
		op.KindMoveWindowToWorkspace:
	default:
		return
	}
	if oper.Target.DesiredWindow == nil {
		return
	}
	target := *oper.Target.DesiredWindow
	dw := c.findDesiredWindow(target)
	if dw == nil {
		return
	}
	// Resolve against fresh observation threading the CURRENT provenance so a
	// sibling editor's already-captured colliding same-title window is excluded
	// (C1) — we never re-capture another identity's window. We do not rely on
	// this identity's OWN entry being present yet (it usually is not, this being
	// the capture); focus-tiebreak collapses any residual same-title ambiguity.
	res := identity.ResolveWithFocusTiebreak(*dw, c.state.Observed, identity.ResolveOptions{Provenance: c.state.Meta.WindowProvenance})
	if res.Class != identity.ClassUniqueStrong || res.Live == "" {
		return
	}
	if c.state.Meta.WindowProvenance == nil {
		c.state.Meta.WindowProvenance = map[w.DesiredWindowID]w.LiveWindowID{}
	}
	if existing, ok := c.state.Meta.WindowProvenance[target]; ok && existing == res.Live {
		return // double-record guard (A6)
	}
	c.state.Meta.WindowProvenance[target] = res.Live
}

// pruneProvenance drops WindowProvenance entries that no longer reflect a live,
// active managed window (SSOT §6.9 validated cache). An entry is removed when
// (a) its desired identity is no longer an active — non-archived, slot-assigned
// — desired window, or (b) its remembered live ID is no longer observed. This
// self-heals stale entries and clears provenance on
// archive/remove/profile-switch/unassign (E1/E2/E3/E5).
func (c *Controller) pruneProvenance() {
	if len(c.state.Meta.WindowProvenance) == 0 {
		return
	}
	for id, live := range c.state.Meta.WindowProvenance {
		if !c.isActiveDesiredWindow(id) {
			delete(c.state.Meta.WindowProvenance, id)
			continue
		}
		if _, observed := c.state.Observed.Windows[live]; !observed {
			delete(c.state.Meta.WindowProvenance, id)
		}
	}
}

// captureSlotAdoptions establishes provenance for cold-start / recovery
// slot-territory adoption (SSOT §6.9.1 B2): when an active desired editor-class
// identity has no usable provenance and a same-title same-bundle window already
// sits on its slot workspace, projwm adopts that window (records it as ours)
// rather than spawning a duplicate beside it. STRICTLY slot territory — windows
// on the user's own workspaces are never adopted (B3 / G2 / G3), which holds
// because the candidate search is scoped to the slot workspace.
func (c *Controller) captureSlotAdoptions() {
	prof, ok := c.state.Desired.Profiles[c.state.Desired.ActiveProfile]
	if !ok {
		return
	}
	for _, slotID := range c.state.Environment.SlotOrder() {
		pid, assigned := prof.Assignments[slotID]
		if !assigned {
			continue
		}
		pr, ok := c.state.Desired.Projects[pid]
		if !ok || pr.Archived {
			continue
		}
		slot, ok := c.state.Environment.SlotByID(slotID)
		if !ok {
			continue
		}
		for i := range pr.Windows {
			dw := pr.Windows[i]
			// Skip windows that already resolve uniquely (spawned/owned).
			res := identity.ResolveWithOptions(dw, c.state.Observed, identity.ResolveOptions{Provenance: c.state.Meta.WindowProvenance})
			if res.Class == identity.ClassUniqueStrong {
				continue
			}
			live, ok := adoptableSlotWindow(dw, c.state.Observed, slot.Workspace, c.state.Meta.WindowProvenance)
			if !ok {
				continue
			}
			if c.state.Meta.WindowProvenance == nil {
				c.state.Meta.WindowProvenance = map[w.DesiredWindowID]w.LiveWindowID{}
			}
			c.state.Meta.WindowProvenance[dw.ID] = live
			// Re-populate MatchedTo so the adopted window now carries the
			// resolver-truthful identity hint downstream.
			c.state.Observed = identity.PopulateMatchedToWithProvenance(c.state.Desired, c.state.Observed, c.state.Meta.WindowProvenance)
		}
	}
}

// adoptableSlotWindow returns the live ID of a same-title same-bundle window on
// the slot workspace eligible for slot-territory adoption, mirroring the
// planner's adoptableOnSlot gate. Browser windows (B-05) are excluded.
func adoptableSlotWindow(dw w.DesiredWindow, observed w.ObservedWorld, slotWS w.WorkspaceID, provenance map[w.DesiredWindowID]w.LiveWindowID) (w.LiveWindowID, bool) {
	if dw.Kind == w.WindowBrowser || dw.App.BundleID == "" || dw.TitleContract.Expected == "" {
		return "", false
	}
	owned := map[w.LiveWindowID]bool{}
	for id, live := range provenance {
		if id != dw.ID && live != "" {
			owned[live] = true
		}
	}
	ids := make([]w.LiveWindowID, 0, len(observed.Windows))
	for id := range observed.Windows {
		ids = append(ids, id)
	}
	sortLiveIDs(ids)
	for _, id := range ids {
		ow := observed.Windows[id]
		if ow.Workspace != slotWS || ow.App.BundleID != dw.App.BundleID || ow.Title.Value != dw.TitleContract.Expected {
			continue
		}
		if owned[id] {
			continue
		}
		if ow.MatchedTo != nil && *ow.MatchedTo != dw.ID {
			continue
		}
		return id, true
	}
	return "", false
}

func sortLiveIDs(ids []w.LiveWindowID) {
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
}

// pruneProvenanceInactive drops provenance entries whose desired identity is no
// longer in the active desired set (archive/remove/profile-switch/unassign). It
// does NOT consult observation, so it is safe to call right after reduce (before
// the converge loop re-observes). The full validated-cache prune
// (pruneProvenance) additionally drops entries whose live ID is gone.
func (c *Controller) pruneProvenanceInactive() {
	if len(c.state.Meta.WindowProvenance) == 0 {
		return
	}
	for id := range c.state.Meta.WindowProvenance {
		if !c.isActiveDesiredWindow(id) {
			delete(c.state.Meta.WindowProvenance, id)
		}
	}
}

// isActiveDesiredWindow reports whether id is a desired window of an active
// (non-archived, slot-assigned) project that still declares this identity.
func (c *Controller) isActiveDesiredWindow(id w.DesiredWindowID) bool {
	if !c.state.Desired.IsProjectActive(id.Project) {
		return false
	}
	return c.findDesiredWindow(id) != nil
}

// findDesiredWindow returns the DesiredWindow for id, or nil if the project no
// longer declares it.
func (c *Controller) findDesiredWindow(id w.DesiredWindowID) *w.DesiredWindow {
	pr, ok := c.state.Desired.Projects[id.Project]
	if !ok {
		return nil
	}
	for i := range pr.Windows {
		if pr.Windows[i].ID == id {
			return &pr.Windows[i]
		}
	}
	return nil
}

// absorbUserCloseRecords merges reducer-emitted close events into
// ControllerMeta.UserCloseHistory and prunes anything older than 60s.
// T4.4 rate limit.
func (c *Controller) absorbUserCloseRecords(recs []event.UserCloseRecord) {
	if len(recs) == 0 && len(c.state.Meta.UserCloseHistory) == 0 {
		return
	}
	if c.state.Meta.UserCloseHistory == nil {
		c.state.Meta.UserCloseHistory = map[w.DesiredWindowID][]int64{}
	}
	for _, r := range recs {
		if (r.DesiredID == w.DesiredWindowID{}) {
			continue
		}
		c.state.Meta.UserCloseHistory[r.DesiredID] = append(c.state.Meta.UserCloseHistory[r.DesiredID], r.At)
	}
	// Prune older than 60s.
	cutoff := time.Now().UnixNano() - int64(60*time.Second)
	for id, ts := range c.state.Meta.UserCloseHistory {
		kept := ts[:0]
		for _, t := range ts {
			if t >= cutoff {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(c.state.Meta.UserCloseHistory, id)
		} else {
			c.state.Meta.UserCloseHistory[id] = kept
		}
	}
}

// applyCockpitSync walks DirtyScopes for "cockpit-sync" entries emitted
// by reducer.ReactToEvent on Bootstrap / Wake / DisplayChanged and
// submits the internal SyncCockpitSystemWindows intent to keep
// DesiredWorld.SystemWindows aligned with requirements v2.4 §8.1 (always
// exactly 1 cockpit on the projwm-managed monitor).
//
// No-op guard: skip the intent only when DesiredWorld.SystemWindows
// already has exactly 1 cockpit entry with DisplayIdx=0 and ParkWorkspace=CP1
// (i.e., already in the required state). DisplayCount is intentionally
// ignored for the skip decision per requirements v2.4 §8.1 —
// the number of physical displays does not affect how many cockpits we want.
// applyStartupReconstruction converts the Bootstrap lifecycle into the
// internal ReconstructFromObserved intent (SSOT §3.5 case B/D, INV-10),
// updating DesiredWorld in place under the single-writer lock.
func (c *Controller) applyStartupReconstruction() {
	newDesired, err := reducer.ReduceIntent(c.state, intent.ReconstructFromObserved{})
	if err != nil {
		return
	}
	c.state.Desired = newDesired
}

func (c *Controller) applyCockpitSync() {
	if len(c.state.Meta.DirtyScopes) == 0 {
		return
	}
	keep := c.state.Meta.DirtyScopes[:0]
	for _, ds := range c.state.Meta.DirtyScopes {
		if ds.Kind != "cockpit-sync" {
			keep = append(keep, ds)
			continue
		}
		var count int
		if _, err := fmt.Sscanf(ds.Key, "%d", &count); err != nil || count < 0 {
			continue
		}
		// No-op guard: skip only when already in the desired state.
		// For count==0: desired state is no cockpit entries.
		// For count>=1: desired state is exactly 1 cockpit with D0/CP1 (v2.4 §8.1).
		// We must NOT use the old "cockpits == count" check because we always want
		// exactly 1 (not N), regardless of display count (requirements v2.4 §8.1).
		alreadyCorrect := func() bool {
			sws := c.state.Desired.SystemWindows
			if count == 0 {
				// No physical display: desired = no cockpit.
				for _, sw := range sws {
					if sw.Kind == w.WindowCockpit {
						return false
					}
				}
				return true
			}
			cockpits := 0
			for _, sw := range sws {
				if sw.Kind == w.WindowCockpit {
					cockpits++
					if sw.DisplayIdx != 0 || sw.ParkWorkspace != "CP1" || sw.Title != "projwm-cockpit-0" {
						return false // needs migration to canonical form
					}
				}
			}
			return cockpits == 1
		}()
		if alreadyCorrect {
			continue
		}
		newDesired, err := reducer.ReduceIntent(c.state, intent.SyncCockpitSystemWindows{DisplayCount: count})
		if err != nil {
			continue
		}
		c.state.Desired = newDesired
	}
	c.state.Meta.DirtyScopes = keep
}

// applyCockpitVisibilitySync walks DirtyScopes for "cockpit-visibility-sync"
// entries emitted by reducer.ReactToEvent (KindWindowsChanged /
// KindDisplayChanged) and applies the internal SetCockpitVisibility intent
// so DesiredWorld.SystemWindows[cockpit].Visibility follows the observed
// active workspace.
//
// Realises requirements v2.7 §8.3.1: ユーザの手動 workspace 切替を暗黙の
// Visibility flip として扱う。これにより space+1 / space+q 等の workspace
// 移動コマンドが自動的に cockpit toggle 相当の意味を持ち、planner が
// 「Visibility=Shown だが observed≠CP1」を見て ShowCockpit op を再発火
// する loop を構造的に止める。
//
// Scope is consumed (dropped from DirtyScopes) so a follow-up reconcile
// in the same transaction does not re-fire.
func (c *Controller) applyCockpitVisibilitySync() {
	if len(c.state.Meta.DirtyScopes) == 0 {
		return
	}
	keep := c.state.Meta.DirtyScopes[:0]
	for _, ds := range c.state.Meta.DirtyScopes {
		if ds.Kind != "cockpit-visibility-sync" {
			keep = append(keep, ds)
			continue
		}
		var vis w.CockpitVisibility
		switch ds.Key {
		case string(w.CockpitShown):
			vis = w.CockpitShown
		case string(w.CockpitHidden):
			vis = w.CockpitHidden
		default:
			continue
		}
		newDesired, err := reducer.ReduceIntent(c.state, intent.SetCockpitVisibility{Visibility: vis})
		if err != nil {
			continue
		}
		c.state.Desired = newDesired
	}
	c.state.Meta.DirtyScopes = keep
}

// applyTier2AutoSyncLayout walks DirtyScopes for "layout-sync" entries
// emitted by reducer.ReactToEvent (KindLayoutChanged /
// KindUserReorderedColumns under SSOT N-12) and applies the internal
// AutoSyncLayout intent for each. Drops the layout-sync scopes after
// consumption so they don't repeatedly fire.
func (c *Controller) applyTier2AutoSyncLayout() {
	if len(c.state.Meta.DirtyScopes) == 0 {
		return
	}
	keep := c.state.Meta.DirtyScopes[:0]
	for _, ds := range c.state.Meta.DirtyScopes {
		if ds.Kind != "layout-sync" {
			keep = append(keep, ds)
			continue
		}
		// Key format: "<projectID>|<workspaceID>".
		proj, ws := splitLayoutSyncKey(ds.Key)
		if proj == "" || ws == "" {
			continue
		}
		cols, ok := observedColumnsForProject(c.state, proj, ws)
		if !ok {
			// Even when observed mapping is incomplete (project hasn't
			// fully reconciled yet), still emit an AutoSyncLayout intent
			// with whatever desired columns we have, so the event isn't
			// silently dropped. The reducer treats empty columns as a
			// no-op clear; partial mappings still record the user's
			// intent at the workspace level.
			cols = projectDesiredColumnsForWS(c.state.Desired, proj, ws)
		}
		newDesired, err := reducer.ReduceIntent(c.state, intent.AutoSyncLayout{
			Project:   proj,
			Workspace: ws,
			Columns:   cols,
		})
		if err != nil {
			continue
		}
		c.state.Desired = newDesired
	}
	c.state.Meta.DirtyScopes = keep
}

// projectDesiredColumnsForWS returns the current AcceptedLayouts/desired
// columns for (project, ws) as a fallback when observation is incomplete.
func projectDesiredColumnsForWS(d w.DesiredWorld, proj w.ProjectID, ws w.WorkspaceID) []w.DesiredColumn {
	if al := d.AcceptedLayouts[proj]; al != nil {
		if layout, ok := al[ws]; ok {
			return append([]w.DesiredColumn(nil), layout.Columns...)
		}
	}
	return nil
}

// allLiveOnWorkspace returns the IDs of every observed window currently
// placed on the named workspace.
func allLiveOnWorkspace(obs w.ObservedWorld, ws w.WorkspaceID) []w.LiveWindowID {
	var out []w.LiveWindowID
	for id, ow := range obs.Windows {
		if ow.Workspace == ws {
			out = append(out, id)
		}
	}
	return out
}

// splitLayoutSyncKey parses "proj|ws" produced by reducer's
// KindLayoutChanged handler. Returns empty strings on malformed input.
func splitLayoutSyncKey(key string) (w.ProjectID, w.WorkspaceID) {
	for i := 0; i < len(key); i++ {
		if key[i] == '|' {
			return w.ProjectID(key[:i]), w.WorkspaceID(key[i+1:])
		}
	}
	return "", ""
}

// observedColumnsForProject maps the observed layout for ws into
// DesiredColumns referring back to the project's DesiredWindow IDs.
// Mirrors Controller.observedDesiredColumnsForProject but operates on
// state-level data without mutation.
func observedColumnsForProject(state w.WorldState, proj w.ProjectID, ws w.WorkspaceID) ([]w.DesiredColumn, bool) {
	pr, ok := state.Desired.Projects[proj]
	if !ok {
		return nil, false
	}
	layout, ok := state.Observed.Layouts[ws]
	if !ok {
		return nil, false
	}
	// Build live→desired map by matching titles. We rely on the identity
	// resolver hint stored in ObservedWindow.MatchedTo; without it we
	// can't safely auto-sync (a partial match would re-order other
	// projects' windows).
	liveToDesired := map[w.LiveWindowID]w.DesiredWindowID{}
	for liveID, ow := range state.Observed.Windows {
		if ow.MatchedTo == nil || ow.MatchedTo.Project != proj {
			continue
		}
		liveToDesired[liveID] = *ow.MatchedTo
	}
	// Count project windows expected on THIS workspace. Viewers live on
	// the viewer workspace, not the project's slot workspace, so they
	// don't participate in the slot-workspace layout. Without this filter
	// the len-match guard below would never pass when a project has a
	// viewer mirror.
	expectedOnWS := 0
	for _, dw := range pr.Windows {
		if dw.Kind == w.WindowViewer {
			continue
		}
		expectedOnWS++
	}
	// Count observed project windows actually placed on the target ws.
	observedOnWS := 0
	for _, liveID := range allLiveOnWorkspace(state.Observed, ws) {
		if _, ok := liveToDesired[liveID]; ok {
			observedOnWS++
		}
	}
	if observedOnWS != expectedOnWS {
		return nil, false
	}
	cols := make([]w.DesiredColumn, 0, len(layout.Columns))
	for _, oc := range layout.Columns {
		col := w.DesiredColumn{Mode: oc.Mode}
		if col.Mode == "" {
			col.Mode = w.ColumnSolo
		}
		for _, live := range oc.Windows {
			id, ok := liveToDesired[live]
			if !ok {
				continue
			}
			col.Windows = append(col.Windows, id)
		}
		if len(col.Windows) > 0 {
			cols = append(cols, col)
		}
	}
	if len(cols) == 0 {
		return nil, false
	}
	return cols, true
}

// applyCardIntent mutates ControllerMeta.ActiveCards in response to
// DismissCard / DismissAllCards intents. Other intents are no-ops here.
func (c *Controller) applyCardIntent(in intent.Intent) {
	switch v := in.(type) {
	case intent.DismissCard:
		out := c.state.Meta.ActiveCards[:0]
		for _, card := range c.state.Meta.ActiveCards {
			if w.CardID(v.CardID) == card.ID {
				continue
			}
			out = append(out, card)
		}
		c.state.Meta.ActiveCards = out
	case intent.DismissAllCards:
		c.state.Meta.ActiveCards = nil
	}
}

// appendActiveCards adds reducer-emitted cards to ActiveCards with a
// generated ID + monotonically increasing timestamp. Duplicate cards
// (same Type + Subject + window context) are merged into the existing
// entry instead of stacking up.
func (c *Controller) appendActiveCards(cards []w.Card) {
	if len(cards) == 0 {
		return
	}
	now := time.Now().UnixNano()
	for _, card := range cards {
		if c.cardAlreadyActive(card) {
			continue
		}
		if card.ID == "" {
			card.ID = w.CardID(fmt.Sprintf("card-%d-%d", now, len(c.state.Meta.ActiveCards)))
		}
		if card.CreatedAt == 0 {
			card.CreatedAt = now
		}
		c.state.Meta.ActiveCards = append(c.state.Meta.ActiveCards, card)
		if c.OnBroadcast != nil {
			c.OnBroadcast("card-added", card, "")
		}
	}
}

// EmitOmniwmRecoveryCard surfaces a [OMNIWM-RECOVERY] card when the
// daemon's self-heal ladder (Lv1-Lv4) takes an action — restart
// omniwmctl, restart omniwm, redeploy rules. SSOT §5.4 cards 6 種
// requires this surface (alongside NEW / CLOSED / MOVED / INVARIANT /
// MANIFEST). Action key Enter dismisses; the card is informational.
func (c *Controller) EmitOmniwmRecoveryCard(level, action, detail string) {
	c.wmMutationLock.Lock()
	defer c.wmMutationLock.Unlock()
	subject := fmt.Sprintf("OmniWM self-heal %s: %s", level, action)
	ctx := map[string]string{
		"level":  level,
		"action": action,
	}
	if detail != "" {
		ctx["detail"] = detail
	}
	c.appendActiveCards([]w.Card{{
		Type:    w.CardTypeOmniwmRecovery,
		Subject: subject,
		Context: ctx,
		Actions: []w.CardAction{
			{Key: "Enter", Label: "acknowledge"},
			{Key: "Esc", Label: "dismiss"},
		},
	}})
}

// EmitManifestMismatchCard surfaces a [MANIFEST] card when the daemon
// detects that the runtime manifest digest no longer matches the digest
// it was started with (requirements E2.1).
func (c *Controller) EmitManifestMismatchCard(expected, observed string) {
	c.wmMutationLock.Lock()
	defer c.wmMutationLock.Unlock()
	c.appendActiveCards([]w.Card{{
		Type:    w.CardTypeManifest,
		Subject: "manifest digest mismatch",
		Context: map[string]string{
			"expected": expected,
			"observed": observed,
		},
		Actions: []w.CardAction{
			{Key: "Enter", Label: "open validation report"},
			{Key: "Esc", Label: "dismiss"},
		},
	}})
}

// PromoteOrphans walks ControllerMeta.PendingOrphans, removing entries
// whose live window now has MatchedTo (silent adoption succeeded) and
// promoting entries older than gracePeriod into [NEW] cards.
//
// design v3 §3.6 — call from the daemon on a 1s tick after the controller
// post-cycle so the 5-second grace is honored.
func (c *Controller) PromoteOrphans(gracePeriod time.Duration) {
	c.wmMutationLock.Lock()
	defer c.wmMutationLock.Unlock()
	now := time.Now().UnixNano()
	out := c.state.Meta.PendingOrphans[:0]
	for _, oc := range c.state.Meta.PendingOrphans {
		ow, exists := c.state.Observed.Windows[oc.LiveID]
		if !exists {
			// Window closed before grace expired; drop silently.
			continue
		}
		if ow.MatchedTo != nil {
			// Silent adoption (e.g. spawn-settle race) — drop.
			continue
		}
		if now-oc.DetectedAt < gracePeriod.Nanoseconds() {
			out = append(out, oc)
			continue
		}
		// Promote to [NEW] card. Per-app suggested action set varies:
		// requirements §4.1 (Ghostty) / §4.2 (Vivaldi) / §4.3 (Zed).
		actions := []w.CardAction{
			{Key: "Enter", Label: "adopt / respawn properly"},
			{Key: "c", Label: "close orphan"},
			{Key: "t", Label: "carry over to TUI"},
			{Key: "Esc", Label: "dismiss"},
		}
		subject := fmt.Sprintf("manual %s window on managed workspace %s", oc.Kind, oc.Workspace)
		if oc.BundleID == "dev.zed.Zed" {
			subject = "Zed window on managed workspace (suggest registering as project)"
			actions = []w.CardAction{
				{Key: "Enter", Label: "register as new project + slot prompt"},
				{Key: "c", Label: "close"},
				{Key: "t", Label: "carry over to TUI"},
				{Key: "Esc", Label: "dismiss"},
			}
		}
		card := w.Card{
			Type:    w.CardTypeNew,
			Subject: subject,
			Context: map[string]string{
				"live":      string(oc.LiveID),
				"kind":      string(oc.Kind),
				"workspace": string(oc.Workspace),
				"bundleID":  oc.BundleID,
				"title":     oc.Title,
			},
			Actions: actions,
		}
		c.appendActiveCards([]w.Card{card})
	}
	c.state.Meta.PendingOrphans = out
}

func (c *Controller) cardAlreadyActive(candidate w.Card) bool {
	for _, existing := range c.state.Meta.ActiveCards {
		if existing.Type != candidate.Type {
			continue
		}
		if existing.Subject != candidate.Subject {
			continue
		}
		// Compare context maps shallowly. Cards that share either the "window"
		// key (Tier 4 / CLOSED / MOVED cards) or the "live" key ([NEW] orphan
		// cards) with the same value are treated as duplicates.
		// §10.4 / §3.1 dedup.
		if existing.Context["window"] != "" && existing.Context["window"] == candidate.Context["window"] {
			return true
		}
		if existing.Context["live"] != "" && existing.Context["live"] == candidate.Context["live"] {
			return true
		}
	}
	return false
}

func cloneDesiredWorld(d w.DesiredWorld) w.DesiredWorld {
	out := d
	out.Profiles = make(map[w.ProfileID]w.DesiredProfile, len(d.Profiles))
	for id, profile := range d.Profiles {
		profile.Assignments = cloneAssignments(profile.Assignments)
		out.Profiles[id] = profile
	}
	out.Projects = make(map[w.ProjectID]w.DesiredProject, len(d.Projects))
	for id, project := range d.Projects {
		project.Windows = cloneDesiredWindows(project.Windows)
		project.Layouts = cloneLayouts(project.Layouts)
		out.Projects[id] = project
	}
	out.FocusPolicy.FinalFocus = cloneFocusPolicy(d.FocusPolicy.FinalFocus)
	out.AcceptedLayouts = cloneAcceptedLayouts(d.AcceptedLayouts)
	out.SystemWindows = append([]w.SystemWindow(nil), d.SystemWindows...)
	return out
}

func cloneAssignments(in map[w.SlotID]w.ProjectID) map[w.SlotID]w.ProjectID {
	if in == nil {
		return nil
	}
	out := make(map[w.SlotID]w.ProjectID, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneFocusPolicy(in map[string]w.WorkspaceID) map[string]w.WorkspaceID {
	if in == nil {
		return nil
	}
	out := make(map[string]w.WorkspaceID, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneDesiredWindows(in []w.DesiredWindow) []w.DesiredWindow {
	out := make([]w.DesiredWindow, len(in))
	for i, win := range in {
		win.MatchHints = append([]w.MatchHint(nil), win.MatchHints...)
		out[i] = win
	}
	return out
}

func cloneLayouts(in map[w.WorkspaceID]w.DesiredLayout) map[w.WorkspaceID]w.DesiredLayout {
	if in == nil {
		return nil
	}
	out := make(map[w.WorkspaceID]w.DesiredLayout, len(in))
	for k, layout := range in {
		out[k] = cloneDesiredLayout(layout)
	}
	return out
}

func cloneAcceptedLayouts(in map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout) map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout {
	if in == nil {
		return nil
	}
	out := make(map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout, len(in))
	for project, layouts := range in {
		out[project] = cloneLayouts(layouts)
	}
	return out
}

func cloneDesiredLayout(layout w.DesiredLayout) w.DesiredLayout {
	layout.Columns = cloneDesiredColumns(layout.Columns)
	return layout
}

func cloneDesiredColumns(in []w.DesiredColumn) []w.DesiredColumn {
	out := make([]w.DesiredColumn, len(in))
	for i, col := range in {
		col.Windows = append([]w.DesiredWindowID(nil), col.Windows...)
		out[i] = col
	}
	return out
}

// SSOT N-12: cloneManualLayoutCandidates removed alongside the
// ManualLayoutCandidate type and Meta.ManualLayoutCandidates field.

func planTrace(iteration int, plan op.Plan) store.PlanTrace {
	out := store.PlanTrace{
		Iteration:         iteration,
		PlanID:            plan.ID,
		BaseEpoch:         plan.BaseEpoch,
		Reason:            string(plan.Reason),
		PlannedOperations: len(plan.Operations),
	}
	for _, oper := range plan.Operations {
		mutation := isMutationOperation(oper.Kind)
		if mutation {
			out.MutationOperations++
		}
		out.Operations = append(out.Operations, store.OperationTrace{
			ID:                     oper.ID,
			Kind:                   string(oper.Kind),
			Risk:                   string(oper.Risk),
			LifecycleRemovalMethod: oper.LifecycleRemovalMethod,
			Mutation:               mutation,
		})
	}
	return out
}

func isMutationOperation(kind op.Kind) bool {
	switch kind {
	case op.KindObserveWorld, op.KindValidateEnvironment, op.KindAcceptLayoutObservation, op.KindObserveBarrier:
		return false
	default:
		return true
	}
}

// isDegradableSpawn reports whether a failed op of this kind should be
// tolerated under SSOT §6.8 graceful degradation: a single window spawn
// failure must not abort the whole transaction. Scoped to the per-window
// spawns (terminal/editor/browser/viewer) because §6.8 speaks specifically of
// "1 つのウィンドウの spawn"; these are mutually independent within the spawn
// phase (§6.10), so continuing siblings is safe. Removal and layout failures
// keep hard-abort because §6.10 ordering depends on them, and cockpit spawn is
// infrastructure (INV-06) rather than a per-project window.
func isDegradableSpawn(kind op.Kind) bool {
	switch kind {
	case op.KindSpawnTerminal, op.KindSpawnEditor, op.KindSpawnBrowser, op.KindSpawnViewer:
		return true
	default:
		return false
	}
}

// spawnFailureCard builds the user-visible [INVARIANT] card for a window that
// failed to spawn under graceful degradation (SSOT §6.8 bullet 3). The card is
// keyed by the desired window identity so retries across replan iterations
// dedup into a single surfaced card.
func spawnFailureCard(oper op.Operation, cause error) w.Card {
	win := "?"
	if oper.Target.DesiredWindow != nil {
		d := *oper.Target.DesiredWindow
		win = fmt.Sprintf("%s/%s/%d", d.Project, d.Kind, d.Index)
	}
	return w.Card{
		Type:    w.CardTypeInvariant,
		Subject: fmt.Sprintf("window %s failed to spawn — other windows continue, will retry", win),
		Context: map[string]string{
			"window":   win,
			"cause":    cause.Error(),
			"degraded": "true",
		},
		Actions: []w.CardAction{
			{Key: "Enter", Label: "acknowledge"},
			{Key: "Esc", Label: "dismiss"},
		},
	}
}

// observe synchronously refreshes ObservedWorld. PopulateMatchedTo
// stamps each live window's MatchedTo from identity.Resolve so the
// Tier-1 orphan detector in reducer.ReactToEvent (§3.5) and any other
// consumer of ObservedWindow.MatchedTo sees a resolver-truthful hint
// instead of nil.
func (c *Controller) observe(ctx context.Context) error {
	o, err := c.Adapter.Observe(ctx)
	if err != nil {
		return err
	}
	c.state.Observed = identity.PopulateMatchedToWithProvenance(c.state.Desired, o, c.state.Meta.WindowProvenance)
	// SSOT §6.9.1 B2: adopt a same-title same-bundle window already on a slot
	// (cold start / recovery) so it becomes ours instead of being duplicated.
	c.captureSlotAdoptions()
	return nil
}

// drainUserEvents peeks user-origin events from a Fake adapter (if any) and
// translates them through the Reducer.ReactToEvent contract. The returned ack
// must be called only after the surrounding controller transaction commits.
func (c *Controller) drainUserEvents() func() {
	fake, ok := c.Adapter.(*wm.Fake)
	if !ok {
		return nil
	}
	events := fake.PeekUserEvents()
	if len(events) == 0 {
		return nil
	}
	for _, ue := range events {
		ev := event.Event{
			Source: event.SourceUser,
			Epoch:  c.state.Meta.Epoch,
		}
		switch ue.Kind {
		case "user-reordered-columns":
			ev.Kind = event.KindUserReorderedColumns
			ev.Data.Project = ue.Project
			ev.Data.Workspace = ue.Workspace
			// Translate live columns to desired columns via current observed.
			ev.Data.Columns = liveToDesiredColumns(ue.Columns, c.state.Observed)
		case "user-moved-window":
			ev.Kind = event.KindUserMovedWindow
			ev.Data.Window = ue.Window
			ev.Data.Workspace = ue.Workspace
			ev.Data.TargetWS = ue.TargetWS
		case "user-closed-window":
			ev.Kind = event.KindUserClosedWindow
			ev.Data.Window = ue.Window
			ev.Data.Workspace = ue.Workspace
		default:
			continue
		}
		r, err := reducer.ReactToEvent(c.state, ev)
		if err != nil || r.Discard {
			continue
		}
		c.state.Meta.DirtyScopes = append(c.state.Meta.DirtyScopes, r.DirtyScopes...)
		c.appendActiveCards(r.NewCards)
		c.state.Meta.PendingOrphans = append(c.state.Meta.PendingOrphans, r.OrphanAdds...)
	}
	return func() {
		fake.AckUserEvents(len(events))
	}
}

// SSOT N-12 (2026-05-20): captureObservedManualLayoutCandidate +
// hasManualLayoutCandidate removed alongside the ManualLayoutCandidate
// machinery. Tier 2 layout sync now travels through AutoSyncLayout via
// applyTier2AutoSyncLayout above.

func (c *Controller) observedDesiredColumnsForProject(workspace w.WorkspaceID, project w.DesiredProject) ([]w.DesiredColumn, bool) {
	layout, ok := c.state.Observed.Layouts[workspace]
	if !ok {
		return nil, false
	}
	liveToDesired := map[w.LiveWindowID]w.DesiredWindowID{}
	for _, desired := range project.Windows {
		res := identity.Resolve(desired, c.state.Observed)
		if res.Class == identity.ClassUniqueStrong {
			liveToDesired[res.Live] = desired.ID
		}
	}
	if len(liveToDesired) != len(project.Windows) {
		return nil, false
	}
	seen := map[w.DesiredWindowID]bool{}
	columns := make([]w.DesiredColumn, 0, len(layout.Columns))
	for _, observedColumn := range layout.Columns {
		column := w.DesiredColumn{Mode: observedColumn.Mode}
		if column.Mode == "" {
			column.Mode = w.ColumnSolo
		}
		for _, live := range observedColumn.Windows {
			desired, ok := liveToDesired[live]
			if !ok {
				continue
			}
			if seen[desired] {
				return nil, false
			}
			seen[desired] = true
			column.Windows = append(column.Windows, desired)
		}
		if len(column.Windows) == 0 {
			continue
		}
		if len(column.Windows) > 1 {
			column.Mode = w.ColumnStacked
		}
		columns = append(columns, column)
	}
	if len(seen) != len(project.Windows) {
		return nil, false
	}
	return columns, true
}

func liveToDesiredColumns(cols [][]w.LiveWindowID, obs w.ObservedWorld) []w.DesiredColumn {
	out := make([]w.DesiredColumn, 0, len(cols))
	for _, c := range cols {
		dc := w.DesiredColumn{}
		for _, id := range c {
			ow, ok := obs.Windows[id]
			if !ok || ow.MatchedTo == nil {
				continue
			}
			dc.Windows = append(dc.Windows, *ow.MatchedTo)
		}
		dc.Mode = w.ColumnSolo
		if len(dc.Windows) > 1 {
			dc.Mode = w.ColumnStacked
		}
		out = append(out, dc)
	}
	return out
}

// commandKeyForIntent maps an Intent to its FocusPolicy lookup key.
// design.md §14 — concrete policy table.
func commandKeyForIntent(in intent.Intent) string {
	switch in.(type) {
	case intent.SwitchProfile:
		return "intent:switch-profile"
	case intent.ArchiveProject:
		return "intent:archive-project"
	case intent.UnarchiveProject:
		return "intent:unarchive-project"
	case intent.AssignProject:
		return "intent:assign-project"
	case intent.UnassignSlot:
		return "intent:unassign-slot"
	case intent.Reconcile:
		return "intent:reconcile"
	case intent.ValidateEnvironment:
		return "intent:validate-environment"
	case intent.SummonViewer:
		return "intent:summon-viewer"
	case intent.SummonShell:
		// SSOT §4.1 OP01: slot を payload に持つので commandKey に encode して
		// planner に伝える ("intent:summon-shell:<slot>")。planner は接頭辞で
		// dispatch、接尾辞から slot を抜く。
		s := in.(intent.SummonShell)
		return "intent:summon-shell:" + string(s.Slot)
	case intent.SummonEditor:
		s := in.(intent.SummonEditor)
		return "intent:summon-editor:" + string(s.Slot)
	case intent.SummonBrowser:
		s := in.(intent.SummonBrowser)
		return "intent:summon-browser:" + string(s.Slot)
	case intent.SwitchProject:
		// SSOT §4.1 OP04: target slot key を suffix に encode。
		s := in.(intent.SwitchProject)
		return "intent:switch-project:" + string(s.Slot)
	case intent.CycleSlotWindow:
		// SSOT §4.1 OP05: slot + kind を suffix に encode (":<slot>:<kind>")。
		s := in.(intent.CycleSlotWindow)
		return "intent:cycle-slot-window:" + string(s.Slot) + ":" + string(s.WindowKind)
	}
	return ""
}

func commandKeyForLifecycle(k w.LifecycleTransactionKind) string {
	if k == w.LifecycleNone {
		return "event:external"
	}
	return "lifecycle:" + string(k)
}

// prepareBrowserIntent enforces SSOT §4.1 OP14-17 + §4.4 BR-PRIV-NOSTORE:
// URL bodies must live in PrivatePayloadStore, DesiredWorld only holds opaque
// refs. Called before reducer so reducer stays pure (no I/O, no store access).
//
// Behavior:
//   - PayloadStore == nil: returns in unchanged (S14 第一段階 fallback —
//     URLPayloadRefs holds literal URL strings; existing tests rely on this).
//   - BrowserAddTab: Put(URL) → token, returns BrowserAddTab{URL: token}.
//   - BrowserChangeTabURL: Forget(old ref at Tab-1) + Put(new URL) → token,
//     returns BrowserChangeTabURL{URL: token}.
//   - BrowserRemoveTab: Forget(ref at Tab-1), returns in unchanged
//     (reducer removes the ref).
//   - BrowserReorderTabs: returns in unchanged (Put/Forget not needed).
//
// Failure modes (堅牢性 stance): Put failure aborts the transaction so the
// user sees an error. Forget failure is logged but does not abort — an
// orphan payload file is recoverable (next profile switch / restart clears
// it) and is preferable to losing the user-visible mutation.
func (c *Controller) prepareBrowserIntent(ctx context.Context, in intent.Intent) (intent.Intent, error) {
	if c.PayloadStore == nil {
		return in, nil
	}
	switch v := in.(type) {
	case intent.BrowserAddTab:
		token, err := c.PayloadStore.Put(ctx, browser.PrivatePayload{URLs: []string{v.URL}})
		if err != nil {
			return nil, fmt.Errorf("controller: payload-store put (browser-add-tab): %w", err)
		}
		v.URL = token
		return v, nil
	case intent.BrowserChangeTabURL:
		if ref, ok := c.lookupBrowserRef(v.Project, v.WindowID, v.Tab); ok && browser.IsPayloadToken(ref) {
			if err := c.PayloadStore.Forget(ctx, ref); err != nil {
				log.Printf("controller: payload-store forget (browser-change-tab-url): %v", err)
			}
		}
		token, err := c.PayloadStore.Put(ctx, browser.PrivatePayload{URLs: []string{v.URL}})
		if err != nil {
			return nil, fmt.Errorf("controller: payload-store put (browser-change-tab-url): %w", err)
		}
		v.URL = token
		return v, nil
	case intent.BrowserRemoveTab:
		if ref, ok := c.lookupBrowserRef(v.Project, v.WindowID, v.Tab); ok && browser.IsPayloadToken(ref) {
			if err := c.PayloadStore.Forget(ctx, ref); err != nil {
				log.Printf("controller: payload-store forget (browser-remove-tab): %v", err)
			}
		}
		return in, nil
	default:
		return in, nil
	}
}

// forgetOrphanedBrowserPayloads computes (pre-reduce refs) \ (post-reduce refs)
// and calls PayloadStore.Forget for each opaque token in the difference.
// Triggers when RemoveWindow / ArchiveProject / SwitchProfile (with InactivePolicyRemove)
// closes a browser window: the DesiredBrowserSession is gone from new Desired
// but its tokens point at PrivatePayloadStore files that would otherwise leak.
//
// Literal-URL refs (S14 fallback) are not Forget()ed — IsPayloadToken returns
// false for them, so they are silently filtered out.
func (c *Controller) forgetOrphanedBrowserPayloads(ctx context.Context, pre, post w.DesiredWorld) {
	if c.PayloadStore == nil {
		return
	}
	preTokens := collectBrowserRefs(pre)
	if len(preTokens) == 0 {
		return
	}
	postTokens := collectBrowserRefs(post)
	for t := range preTokens {
		if postTokens[t] {
			continue
		}
		if !browser.IsPayloadToken(t) {
			continue
		}
		if err := c.PayloadStore.Forget(ctx, t); err != nil {
			log.Printf("controller: payload-store forget (orphan GC): %v", err)
		}
	}
}

// collectBrowserRefs returns the set of every URLPayloadRefs entry across
// all LIVE projects + windows in d. Archived projects are excluded so
// archive (SSOT §4.5) becomes a GC trigger: archived browser refs are no
// longer reachable by UnarchiveProject (SSOT §4.5 park-state, no auto
// restore) and would otherwise leak in PrivatePayloadStore forever.
func collectBrowserRefs(d w.DesiredWorld) map[string]bool {
	out := map[string]bool{}
	for _, pr := range d.Projects {
		if pr.Archived {
			continue
		}
		for _, win := range pr.Windows {
			if win.Kind != w.WindowBrowser || win.Browser == nil {
				continue
			}
			for _, ref := range win.Browser.URLPayloadRefs {
				out[string(ref)] = true
			}
		}
	}
	return out
}

// lookupBrowserRef returns the URLPayloadRefs entry at 1-based tab index for
// the given project/window. Used by prepareBrowserIntent to read the old ref
// before reducer mutates DesiredWorld.
func (c *Controller) lookupBrowserRef(project w.ProjectID, windowID w.DesiredWindowID, tab int) (string, bool) {
	pr, ok := c.state.Desired.Projects[project]
	if !ok {
		return "", false
	}
	for _, win := range pr.Windows {
		if win.ID != windowID || win.Kind != w.WindowBrowser || win.Browser == nil {
			continue
		}
		if tab < 1 || tab > len(win.Browser.URLPayloadRefs) {
			return "", false
		}
		return string(win.Browser.URLPayloadRefs[tab-1]), true
	}
	return "", false
}
