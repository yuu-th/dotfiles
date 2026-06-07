// Package planner is a deterministic rule-based planner. design.md §11.
// Pure: no observe / execute / sleep / store write. Map iteration is sorted.
package planner

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yuu-th/projwm-next/internal/identity"
	"github.com/yuu-th/projwm-next/internal/op"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// plannerTraceEnabled gates the spawn-browser convergence diagnostic. Default
// OFF; enable with PROJWM_NEXT_PLANNER_TRACE=1. The trace prints, per Plan call
// (i.e. per converge-loop replan), every observed WindowBrowser window the
// planner sees (live id, workspace, title, bundleID, MatchedTo) and, for each
// active desired browser window, the identity.Resolve Class + Candidates that
// drives the spawn-browser-vs-converge decision. Keepable diagnostic
// (handoff §9.3): it is read-only and writes only to stderr when enabled.
func plannerTraceEnabled() bool {
	return os.Getenv("PROJWM_NEXT_PLANNER_TRACE") == "1"
}

func plannerTracef(format string, args ...interface{}) {
	if !plannerTraceEnabled() {
		return
	}
	// Wall-clock timestamp (handoff §14.10) so the planner trace can be
	// correlated against an external `omniwmctl query windows` time-series
	// recorder and the WM_TRACE / settle trace — disambiguating "the 75s
	// browser settle never ran" vs "it ran but daemon Observe never surfaced
	// the managed Vivaldi window".
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[PLANNER_TRACE %s] %s\n", time.Now().Format("15:04:05.000"), msg)
}

// traceObservedBrowsers logs every observed window classified WindowBrowser
// (and, for context, every com.vivaldi.Vivaldi-bundle window regardless of
// kind, since a managed Vivaldi misclassified as WindowExternal is the exact
// failure mode under investigation). Read-only.
func traceObservedBrowsers(observed w.ObservedWorld) {
	if !plannerTraceEnabled() {
		return
	}
	browserCount := 0
	vivaldiCount := 0
	for _, id := range sortedLiveIDs(observed.Windows) {
		ow := observed.Windows[id]
		isVivaldi := ow.App.BundleID == "com.vivaldi.Vivaldi"
		if ow.Kind != w.WindowBrowser && !isVivaldi {
			continue
		}
		if ow.Kind == w.WindowBrowser {
			browserCount++
		}
		if isVivaldi {
			vivaldiCount++
		}
		matched := "<nil>"
		if ow.MatchedTo != nil {
			matched = fmt.Sprintf("%s/%s/%d", ow.MatchedTo.Project, ow.MatchedTo.Kind, ow.MatchedTo.Index)
		}
		plannerTracef("observed-window live=%s kind=%s bundle=%s ws=%s vis=%s focused=%v title=%q matchedTo=%s",
			id, ow.Kind, ow.App.BundleID, ow.Workspace, ow.Visibility, ow.Focused, ow.Title.Value, matched)
	}
	plannerTracef("observed-summary WindowBrowser=%d vivaldiBundle=%d totalWindows=%d", browserCount, vivaldiCount, len(observed.Windows))
}

func traceResolveResult(dw w.DesiredWindow, res identity.Resolution, workspace w.WorkspaceID) {
	if !plannerTraceEnabled() {
		return
	}
	cands := make([]string, 0, len(res.Candidates))
	for _, c := range res.Candidates {
		cands = append(cands, string(c))
	}
	ev := make([]string, 0, len(res.Evidence))
	for _, e := range res.Evidence {
		ev = append(ev, fmt.Sprintf("%s:%s@%s", e.Kind, e.Strength, e.Window))
	}
	miss := make([]string, 0, len(res.MissingEvidence))
	for _, m := range res.MissingEvidence {
		miss = append(miss, string(m))
	}
	plannerTracef("resolve desired=%s/%s/%d kind=%s slotWS=%s -> Class=%s Live=%s Candidates=%v Evidence=%v Missing=%v",
		dw.ID.Project, dw.ID.Kind, dw.ID.Index, dw.Kind, workspace, res.Class, res.Live, cands, ev, miss)
}

// userCloseRateLimited returns true when the user has closed the given
// DesiredWindow at least twice in the last 60 seconds (requirements T4.4).
// Planner consults this before emitting a spawn op so we don't loop on
// a window the user keeps closing.
func userCloseRateLimited(history map[w.DesiredWindowID][]int64, id w.DesiredWindowID) bool {
	ts, ok := history[id]
	if !ok {
		return false
	}
	cutoff := time.Now().UnixNano() - int64(60*time.Second)
	count := 0
	for _, t := range ts {
		if t >= cutoff {
			count++
		}
	}
	return count >= 2
}

// CommandKey is an opaque key into FocusPolicySet.FinalFocus.
// Must be supplied by Controller (it knows the originating intent / lifecycle).
type CommandKey string

// Plan returns a deterministic Plan that converges WorldState toward target DesiredWorld.
// Pure; uses sorted iteration.
//
// The emitted operation sequence is phase-separated so the executor can refresh
// observation between phases. This avoids stale-observation races where, for
// example, a Phase A close-window completes (the macOS AX/SkyLight tree drops
// the window asynchronously) but the next operation's precondition check sees
// the still-listed window and rejects the mutation as ambiguous. Phases:
//
//	Phase A — removals  : KindCloseWindow / KindKillSession
//	Phase B — spawns    : KindSpawnTerminal / KindSpawnEditor / KindSpawnBrowser / KindSpawnViewer
//	Phase C — layout    : KindMoveWindowToWorkspace / KindReorderColumns / KindFocusWorkspace / KindFocusWindow
//
// KindObserveBarrier is injected between any two consecutive phases that both
// produce ops. design.md §10, specs §7.
func Plan(state w.WorldState, target w.DesiredWorld, command CommandKey, reason op.PlanReason) (op.Plan, error) {
	var phaseRemovals []op.Operation
	var phaseSpawns []op.Operation
	var phaseLayout []op.Operation
	idCounter := 0
	mkID := func(kindHint string) w.OperationID {
		idCounter++
		return w.OperationID(fmt.Sprintf("op-%s-%d", kindHint, idCounter))
	}
	if plannerTraceEnabled() {
		plannerTracef("=== Plan() command=%s reason=%s epoch=%d activeProfile=%s ===", command, reason, state.Meta.Epoch, target.ActiveProfile)
		traceObservedBrowsers(state.Observed)
	}
	protectedLive := liveCandidatesForActiveDesired(state.Observed, target, state.Meta.WindowProvenance)
	closeBlocked := closeWindowBlocked(state.Environment)
	removalOperation := func(kindHint string, id w.LiveWindowID, desired *w.DesiredWindowID) (op.Operation, bool) {
		opTarget := op.Target{LiveWindow: idPtr(id)}
		kind := op.KindCloseWindow
		risk := op.RiskMedium
		if closeBlocked {
			if desired == nil {
				return op.Operation{}, false
			}
			ow, ok := state.Observed.Windows[id]
			if !ok || !lifecycleRemovalAllowed(state.Environment, target, *desired, ow) {
				return op.Operation{}, false
			}
			d := *desired
			opTarget.DesiredWindow = &d
			kind = op.KindKillSession
			risk = op.RiskHigh
		}
		method := w.LifecycleRemovalMethod("")
		if kind == op.KindKillSession {
			ow := state.Observed.Windows[id]
			if dw, ok := desiredWindowByID(target, *desired); ok {
				if app, ok := state.Environment.ManagedAppByBundle(dw.App.BundleID); ok {
					method = app.LifecycleRemoval.Method
				}
			}
			if method == "" && ow.App.BundleID != "" {
				if app, ok := state.Environment.ManagedAppByBundle(ow.App.BundleID); ok {
					method = app.LifecycleRemoval.Method
				}
			}
		}
		return op.Operation{
			ID:                     mkID(kindHint),
			Kind:                   kind,
			Target:                 opTarget,
			LifecycleRemovalMethod: method,
			Preconditions: []op.Precondition{
				{Kind: op.PreUniqueStrong, Target: opTarget},
			},
			ExpectedEffects: []op.Effect{{Kind: op.EffectCloseWindow, Window: idPtr(id)}},
			Risk:            risk,
		}, true
	}

	// 1) Close windows that are NOT desired.
	//    A live window is desired iff its MatchedTo points to a (project, kind, index) that:
	//      - exists in target.Projects[p].Windows
	//      - the project is active in target (assigned to a slot in target.ActiveProfile, not archived)
	//    Or if the window is a viewer that mirrors an active AI window (handled below).
	closeOrder := sortedLiveIDs(state.Observed.Windows)
	for _, id := range closeOrder {
		ow := state.Observed.Windows[id]
		if ow.Kind == w.WindowExternal {
			continue
		}
		if protectedLive[id] {
			continue
		}
		if ow.Kind == w.WindowViewer {
			// Viewer windows are managed below; only close if not needed.
			continue
		}
		var match w.DesiredWindowID
		if ow.MatchedTo != nil {
			match = *ow.MatchedTo
		} else if inferred, ok := managedDesiredByObserved(ow, target); ok {
			match = inferred
		} else {
			continue
		}
		// Should this window survive in target?
		if !desiredHas(target, match) || !target.IsProjectActive(match.Project) {
			if oper, ok := removalOperation("remove-managed", id, &match); ok {
				phaseRemovals = append(phaseRemovals, oper)
			}
		}
	}

	// 2) Spawn missing desired windows for active projects, on their slot's workspace.
	//    Iterate slots in deterministic order (by Order then ID), then DesiredWindowIDs sorted.
	prof, profOK := target.Profiles[target.ActiveProfile]
	if profOK {
		slotIDs := state.Environment.SlotOrder()
		for _, slotID := range slotIDs {
			pid, assigned := prof.Assignments[slotID]
			if !assigned {
				continue
			}
			pr, ok := target.Projects[pid]
			if !ok || pr.Archived {
				continue
			}
			slot, _ := state.Environment.SlotByID(slotID)
			workspace := slot.Workspace
			// Sort windows by (Kind, Index) for deterministic spawn order.
			windows := append([]w.DesiredWindow(nil), pr.Windows...)
			sort.Slice(windows, func(i, j int) bool {
				if windows[i].ID.Kind != windows[j].ID.Kind {
					return windows[i].ID.Kind < windows[j].ID.Kind
				}
				return windows[i].ID.Index < windows[j].ID.Index
			})
			for _, dw := range windows {
				// SSOT §2.5 EC4 / INV-01: duplicate live windows are resolved by
				// focus-tiebreak so the planner can continue converging; the
				// duplicate is reported separately as a Check14 invariant card.
				// B-05: browser windows are matched by kind+bundleID (Vivaldi
				// can't carry a projwm title). We deliberately do NOT pin
				// ExpectedWorkspace here: a browser that spawned on / drifted to
				// the wrong workspace must still resolve UniqueStrong so the
				// layout phase MOVES it to its slot workspace rather than
				// re-spawning a duplicate. Workspace disambiguates browsers only
				// at the layout/invariant check, where final placement matters.
				// SSOT §6.9.1: thread provenance so a managed editor with a
				// colliding same-title user window resolves UniqueStrong to OUR
				// window (C1/A2) instead of hard-erroring as Ambiguous below.
				res := identity.ResolveWithFocusTiebreak(dw, state.Observed, identity.ResolveOptions{Provenance: state.Meta.WindowProvenance})
				if dw.Kind == w.WindowBrowser {
					traceResolveResult(dw, res, workspace)
				}
				if res.Class == identity.ClassUniqueStrong {
					// Window exists. May need to be moved to its slot workspace.
					ow := state.Observed.Windows[res.Live]
					if ow.Workspace != workspace {
						lid := res.Live
						ws := workspace
						dwid := dw.ID
						phaseLayout = append(phaseLayout, op.Operation{
							ID:     mkID("move"),
							Kind:   op.KindMoveWindowToWorkspace,
							Target: op.Target{DesiredWindow: &dwid, LiveWindow: &lid, Workspace: &ws},
							Preconditions: []op.Precondition{
								{Kind: op.PreUniqueStrong, Target: op.Target{DesiredWindow: &dwid, LiveWindow: &lid}},
								{Kind: op.PreWorkspaceExists, Target: op.Target{Workspace: &ws}},
							},
							ExpectedEffects: []op.Effect{{Kind: op.EffectMoveWindow, Window: &lid, Workspace: &ws}},
							Risk:            op.RiskMedium,
						})
					}
					continue
				}
				if res.Class == identity.ClassMissing {
					dwid := dw.ID
					// SSOT §6.9.1 B2 (slot-territory adopt): on cold start /
					// recovery, a same-title same-bundle window already sitting
					// on THIS slot's workspace is the user's project editor we
					// should adopt rather than spawn a duplicate beside. Only
					// slot territory qualifies — windows on the user's own
					// workspaces stay inviolable (B3/G2/G3), which holds because
					// the search is scoped to the slot workspace. Suppress the
					// spawn; the controller records provenance for the adopted
					// window on the next observe/capture.
					if adoptableOnSlot(dw, state.Observed, workspace, state.Meta.WindowProvenance) {
						continue
					}
					// T4.4: if the user closed this DesiredWindow twice
					// within the last 60 seconds, suppress the spawn so
					// they don't end up in a respawn loop. The cockpit
					// warning card surfaces the suppression.
					if userCloseRateLimited(state.Meta.UserCloseHistory, dwid) {
						continue
					}
					// Browser windows are spawn-blocked until they have
					// a private URL payload to restore. spawnVivaldi's
					// privacy contract refuses any "blank window"
					// fallback, so emitting spawn-browser before the
					// user has populated tabs (via SyncBrowserTabs)
					// would only fail every reconcile. Defer the spawn:
					// when the next SyncBrowserTabs adds a payload ref,
					// the planner re-evaluates and emits spawn-browser
					// with token in hand.
					if dw.Kind == w.WindowBrowser {
						if dw.Browser == nil || len(dw.Browser.URLPayloadRefs) == 0 {
							continue
						}
					}
					ws := workspace
					title := dw.TitleContract.Expected
					phaseSpawns = append(phaseSpawns, op.Operation{
						ID:     mkID(spawnKindHint(dw.Kind)),
						Kind:   spawnKindFor(dw.Kind),
						Target: op.Target{DesiredWindow: &dwid, Workspace: &ws},
						Preconditions: []op.Precondition{
							{Kind: op.PreWorkspaceExists, Target: op.Target{Workspace: &ws}},
						},
						ExpectedEffects: []op.Effect{
							{Kind: op.EffectSpawnWindow, Desired: &dwid, Workspace: &ws, WindowKind: dw.Kind},
						},
						Risk:           op.RiskMedium,
						IdempotencyKey: fmt.Sprintf("spawn:%s:%s:%d:%s", dwid.Project, dwid.Kind, dwid.Index, title),
					})
				}
				if res.Class != identity.ClassMissing {
					return op.Plan{}, fmt.Errorf("planner: identity for active desired window %s/%s/%d classified %s, refusing mutation without unique-strong evidence", dw.ID.Project, dw.ID.Kind, dw.ID.Index, res.Class)
				}
			}
		}
	}

	// 3) Layout: for each active project and its workspace, ensure semantic columns match desired.
	//    Desired columns are: AcceptedLayouts override > project default Layouts.
	if profOK {
		slotIDs := state.Environment.SlotOrder()
		for _, slotID := range slotIDs {
			pid, ok := prof.Assignments[slotID]
			if !ok {
				continue
			}
			pr, exists := target.Projects[pid]
			if !exists || pr.Archived {
				continue
			}
			slot, _ := state.Environment.SlotByID(slotID)
			ws := slot.Workspace
			// SSOT N-12: ManualLayoutCandidate skip-replan logic removed.
			// Tier 2 user reorders are reduced to AutoSyncLayout which
			// writes AcceptedLayouts directly; the planner respects
			// AcceptedLayouts unconditionally below.
			_ = reason
			desiredCols := projectDesiredColumns(pr, ws, target.AcceptedLayouts)
			if len(desiredCols) == 0 {
				continue
			}
			// Translate desired DesiredWindowIDs to live IDs via identity.
			liveCols := [][]w.LiveWindowID{}
			allFound := true
			for _, dc := range desiredCols {
				stack := []w.LiveWindowID{}
				for _, dwid := range dc.Windows {
					dw := findDesiredWindow(pr, dwid)
					if dw == nil {
						allFound = false
						break
					}
					layoutOpts := identity.ResolveOptions{Provenance: state.Meta.WindowProvenance}
					if dw.Kind == w.WindowBrowser {
						layoutOpts.ExpectedWorkspace = ws
					}
					res := identity.ResolveWithFocusTiebreak(*dw, state.Observed, layoutOpts)
					if res.Class != identity.ClassUniqueStrong {
						allFound = false
						break
					}
					stack = append(stack, res.Live)
				}
				if !allFound {
					break
				}
				liveCols = append(liveCols, stack)
			}
			if !allFound {
				continue // spawn ops above will run first; layout settles next round
			}
			// Compare against observed. SSOT §6.3 L3 / §4.3: external windows
			// (kind=external; the close loop above deliberately leaves them in
			// place) are NOT part of DesiredLayout.Columns. Filter them out of
			// the observed columns before comparing, so a drifted external
			// window does not make the planner perpetually re-emit a reorder
			// that can never match — which would burn MaxReplans and fail the
			// transaction. Mirrors the adapter-side managedOrderSettled.
			managed := map[w.LiveWindowID]bool{}
			for _, col := range liveCols {
				for _, id := range col {
					managed[id] = true
				}
			}
			obs := state.Observed.Layouts[ws]
			obsCols := managedObservedColumns(obs.Columns, managed)
			match := sameSemanticLayout(obsCols, liveCols)
			if plannerTraceEnabled() {
				obsSizes := make([]int, len(obsCols))
				for i, c := range obsCols {
					obsSizes[i] = len(c.Windows)
				}
				wantSizes := make([]int, len(liveCols))
				for i, c := range liveCols {
					wantSizes[i] = len(c)
				}
				plannerTracef("reorder-decision ws=%s match=%v obsN=%d obsSizes=%v wantN=%d wantSizes=%v", ws, match, len(obsCols), obsSizes, len(liveCols), wantSizes)
			}
			if !match {
				wsCopy := ws
				phaseLayout = append(phaseLayout, op.Operation{
					ID:     mkID("reorder"),
					Kind:   op.KindReorderColumns,
					Target: op.Target{Workspace: &wsCopy},
					Preconditions: []op.Precondition{
						{Kind: op.PreWorkspaceExists, Target: op.Target{Workspace: &wsCopy}},
					},
					ExpectedEffects: []op.Effect{
						{Kind: op.EffectReorderColumns, Workspace: &wsCopy, Columns: dcCopy(desiredCols)},
					},
					Risk: op.RiskMedium,
				})
			}
		}
	}

	// 4) Viewer maintenance: ensure viewer workspace contains exactly one viewer window per active AI window,
	//    in slot order. Spawn missing, close stale, reorder if out-of-order.
	viewerWS := state.Environment.Workspaces.Viewer
	if profOK && viewerWS != "" {
		// Active AI desired window IDs in slot order.
		var desiredViewerIDs []w.DesiredWindowID
		desiredViewerTitles := map[w.DesiredWindowID]string{}
		for _, slotID := range state.Environment.SlotOrder() {
			pid, ok := prof.Assignments[slotID]
			if !ok {
				continue
			}
			pr, exists := target.Projects[pid]
			if !exists || pr.Archived {
				continue
			}
			// Sort by index.
			ais := []w.DesiredWindow{}
			for _, dw := range pr.Windows {
				if dw.ID.Kind == w.WindowAI {
					ais = append(ais, dw)
				}
			}
			sort.Slice(ais, func(i, j int) bool { return ais[i].ID.Index < ais[j].ID.Index })
			for _, dw := range ais {
				if identity.ResolveWithFocusTiebreak(dw, state.Observed, identity.ResolveOptions{}).Class != identity.ClassUniqueStrong {
					continue
				}
				desiredViewerIDs = append(desiredViewerIDs, dw.ID)
				desiredViewerTitles[dw.ID] = viewerTitleForAI(dw.TitleContract.Expected)
			}
		}
		// Revert viewers stranded on non-viewer workspaces (T4 revert).
		// For each viewer at the wrong workspace:
		//   - if it matches an active desired AI: emit MoveWindowToWorkspace to viewerWS
		//   - if it does not match any active desired AI: emit removal (stale viewer at wrong ws)
		// Viewers already at viewerWS are handled by the residue/reorder logic below.
		viewersBeingMoved := map[w.LiveWindowID]bool{}
		for _, id := range sortedLiveIDs(state.Observed.Windows) {
			ow := state.Observed.Windows[id]
			if ow.Kind != w.WindowViewer || ow.Workspace == viewerWS {
				continue
			}
			matched, isActive := viewerMatchedDesired(ow, desiredViewerIDs, desiredViewerTitles)
			if isActive {
				// Viewer exists but is on the wrong workspace: move it back.
				lid := id
				ws := viewerWS
				_ = matched // viewer identity resolved by title/kind; no DesiredWindow needed
				phaseLayout = append(phaseLayout, op.Operation{
					ID:   mkID("move-viewer"),
					Kind: op.KindMoveWindowToWorkspace,
					Target: op.Target{
						LiveWindow: &lid,
						Workspace:  &ws,
					},
					Preconditions: []op.Precondition{
						{Kind: op.PreUniqueStrong, Target: op.Target{LiveWindow: &lid}},
						{Kind: op.PreWorkspaceExists, Target: op.Target{Workspace: &ws}},
					},
					ExpectedEffects: []op.Effect{{Kind: op.EffectMoveWindow, Window: &lid, Workspace: &ws}},
					Risk:            op.RiskMedium,
				})
				viewersBeingMoved[id] = true
			} else {
				// Stale viewer on wrong workspace: close it.
				var desired *w.DesiredWindowID
				if d, inferred := viewerMatchedAnyDesiredAI(ow, target); inferred {
					dCopy := d
					desired = &dCopy
				}
				// Only remove when it does not belong to an active project
				// (same guard as the residue-close block below).
				if desired == nil || !target.IsProjectActive(desired.Project) {
					if oper, removalOk := removalOperation("remove-viewer-stranded", id, desired); removalOk {
						phaseRemovals = append(phaseRemovals, oper)
					}
				}
			}
		}

		// Existing viewer windows on viewer workspace.
		liveViewers := []w.LiveWindowID{}
		viewerByDesired := map[w.DesiredWindowID]w.LiveWindowID{}
		for _, id := range sortedLiveIDs(state.Observed.Windows) {
			ow := state.Observed.Windows[id]
			if ow.Kind != w.WindowViewer || ow.Workspace != viewerWS {
				continue
			}
			liveViewers = append(liveViewers, id)
			if matched, ok := viewerMatchedDesired(ow, desiredViewerIDs, desiredViewerTitles); ok {
				viewerByDesired[matched] = id
			}
		}
		// Close viewers whose mirror is no longer active. Track which
		// residue viewers will actually be removed by this plan so the
		// reorder block can reason about post-close presence.
		want := map[w.DesiredWindowID]bool{}
		for _, d := range desiredViewerIDs {
			want[d] = true
		}
		viewerResidueScheduledForRemoval := map[w.LiveWindowID]bool{}
		viewerResidueUnremovable := map[w.LiveWindowID]bool{}
		for _, id := range liveViewers {
			ow := state.Observed.Windows[id]
			matched, ok := viewerMatchedDesired(ow, desiredViewerIDs, desiredViewerTitles)
			var desired *w.DesiredWindowID
			if !ok || !want[matched] {
				if ok {
					d := matched
					desired = &d
				} else if d, inferred := viewerMatchedAnyDesiredAI(ow, target); inferred {
					desired = &d
				}
				// Refuse to emit a removal whose desired identity points
				// at an active project's still-desired AI: the executor's
				// IsProjectActive guard would reject it, leaking the
				// viewer onto the workspace permanently. This case
				// arises transiently when the source AI is not yet
				// observed unique-strong (so the viewer is missing from
				// desiredViewerIDs even though its mirror is desired);
				// the next plan iteration will pick it up correctly.
				//
				// HOWEVER: if the source AI window itself is no longer
				// in target.Projects[Project].Windows (e.g., RemoveWindow
				// was applied), the viewer is a true orphan and must be
				// removed — otherwise it sits forever as residue and
				// keeps ReorderColumns failing (got=N+1 want=N). This
				// realises the Tier 4 "managed window close" follow-up
				// for the viewer twin.
				if desired != nil && target.IsProjectActive(desired.Project) && desiredAIWindowExists(target, *desired) {
					viewerResidueUnremovable[id] = true
					continue
				}
				if oper, removalOk := removalOperation("remove-viewer", id, desired); removalOk {
					phaseRemovals = append(phaseRemovals, oper)
					viewerResidueScheduledForRemoval[id] = true
				} else {
					// We observed a residue viewer we cannot prove
					// identity for (or that lifecycle removal does
					// not authorize). Mark it so the reorder block
					// can defer reorder rather than emit a
					// guaranteed-failing op.
					viewerResidueUnremovable[id] = true
				}
			}
		}
		// Spawn missing viewers.
		for _, d := range desiredViewerIDs {
			existing := viewerLiveMatches(state.Observed.Windows, d, desiredViewerTitles[d])
			if len(existing) > 1 {
				return op.Plan{}, fmt.Errorf("planner: viewer identity for %s/%d matched %d windows, refusing duplicate-prone spawn", d.Project, d.Index, len(existing))
			}
			if _, ok := viewerByDesired[d]; ok {
				continue
			}
			if len(existing) == 1 {
				continue
			}
			dCopy := d
			ws := viewerWS
			phaseSpawns = append(phaseSpawns, op.Operation{
				ID:     mkID("spawn-viewer"),
				Kind:   op.KindSpawnViewer,
				Target: op.Target{DesiredWindow: &dCopy, Workspace: &ws},
				Preconditions: []op.Precondition{
					{Kind: op.PreWorkspaceExists, Target: op.Target{Workspace: &ws}},
				},
				ExpectedEffects: []op.Effect{
					{Kind: op.EffectSpawnWindow, Desired: &dCopy, Workspace: &ws, WindowKind: w.WindowViewer},
				},
				IdempotencyKey: fmt.Sprintf("spawn-viewer:%s:%d", d.Project, d.Index),
			})
		}
		// Reorder viewers only if observed order differs from desired (idempotency).
		// We additionally defer reorder when:
		//  - any unremovable residue viewer remains on the viewer workspace
		//    (the reorder would inevitably fail with `got > want` because
		//    waitColumnOrder counts every window in the workspace, not just
		//    the desired ones), or
		//  - any spawn op is queued in this same plan (the spawned
		//    viewer's live ID is not yet known so reorder cannot bind
		//    its want list).
		if len(desiredViewerIDs) > 0 && len(viewerResidueUnremovable) == 0 {
			ws := viewerWS
			obsCols := state.Observed.Layouts[ws].Columns
			// Build desired live cols (one per AI, in slot order). If any AI is not
			// yet observed as a viewer (e.g. spawn pending), defer reorder.
			desiredLive := [][]w.LiveWindowID{}
			complete := true
			for _, d := range desiredViewerIDs {
				lid, ok := viewerByDesired[d]
				if !ok {
					complete = false
					break
				}
				desiredLive = append(desiredLive, []w.LiveWindowID{lid})
			}
			// Account for residue viewers that are slated for removal in
			// this plan: by the time the reorder op runs, those windows
			// will be gone (the executor settles between ops), so count
			// the post-close obsCols rather than the planning-time
			// snapshot.
			postCloseObsCols := make([]w.ObservedColumn, 0, len(obsCols))
			for _, col := range obsCols {
				stillThere := []w.LiveWindowID{}
				for _, id := range col.Windows {
					if viewerResidueScheduledForRemoval[id] {
						continue
					}
					stillThere = append(stillThere, id)
				}
				if len(stillThere) > 0 {
					postCloseObsCols = append(postCloseObsCols, w.ObservedColumn{Windows: stillThere, Mode: col.Mode})
				}
			}
			if complete && !sameSemanticLayout(postCloseObsCols, desiredLive) {
				cols := []w.DesiredColumn{}
				for _, d := range desiredViewerIDs {
					dCopy := d
					cols = append(cols, w.DesiredColumn{Windows: []w.DesiredWindowID{dCopy}, Mode: w.ColumnSolo})
				}
				phaseLayout = append(phaseLayout, op.Operation{
					ID:     mkID("reorder-viewer"),
					Kind:   op.KindReorderColumns,
					Target: op.Target{Workspace: &ws},
					Preconditions: []op.Precondition{
						{Kind: op.PreWorkspaceExists, Target: op.Target{Workspace: &ws}},
					},
					ExpectedEffects: []op.Effect{{Kind: op.EffectReorderColumns, Workspace: &ws, Columns: cols}},
				})
			}
		}
	}

	// Cockpit SystemWindows (unified design v2 — park-workspace model §5):
	//   1) For each desired SystemWindow, find the live ghostty whose
	//      title equals the controller-owned Title. If absent → spawn op.
	//   2) Check show/hide convergence: if display active workspace != desired
	//      (ParkWorkspace for Shown, PriorWorkspace for Hidden), emit op.
	//   3) For displays that no longer have a SystemWindow (unplug) ->
	//      close any leftover cockpit window with that title prefix.
	planCockpitOps(state, target, &phaseRemovals, &phaseSpawns, &phaseLayout, mkID)
	planScratchOps(state, target, &phaseLayout, mkID)
	planSummonViewerOps(state, target, command, &phaseLayout, mkID)
	planSummonWindowOps(state, target, command, &phaseLayout, mkID)
	planSwitchProjectOps(state, target, command, &phaseLayout, mkID)
	planCycleSlotWindowOps(state, target, command, &phaseLayout, mkID)

	// Final focus: per command policy. Emitted LAST so it is the final layout
	// op — after cockpit/scratch/summon ops, which themselves switch a display's
	// workspace (notably hide-cockpit switches the cockpit's display back to its
	// prior workspace, stealing global focus). Placing final-focus earlier let
	// hide-cockpit override it (ACC-S7 INV-10 flake: focus landed on M instead of
	// the policy's A whenever a hide-cockpit op was emitted that recovery).
	// Idempotency: skip if already focused. Only emits when FinalFocus[command]
	// is set, so summon/cycle (which set their own focus and have no policy
	// entry) are unaffected.
	if ws, ok := target.FocusPolicy.FinalFocus[string(command)]; ok && ws != "" {
		if state.Observed.Focus.Workspace != ws {
			wsCopy := ws
			phaseLayout = append(phaseLayout, op.Operation{
				ID:     mkID("focus-ws"),
				Kind:   op.KindFocusWorkspace,
				Target: op.Target{Workspace: &wsCopy},
				Preconditions: []op.Precondition{
					{Kind: op.PreWorkspaceExists, Target: op.Target{Workspace: &wsCopy}},
				},
				ExpectedEffects: []op.Effect{{Kind: op.EffectFocusWorkspace, FocusedWS: &wsCopy}},
				Risk:            op.RiskLow,
			})
		}
	}

	// Assemble the phase-separated operation sequence with KindObserveBarrier
	// inserted between consecutive phases that both produce ops. The barrier
	// op is non-mutating (empty effects) and does not alter the simulator's
	// PredictedWorld; the executor handles it by sleeping briefly + re-Observe
	// so the next op's precondition runs against fresh evidence rather than
	// the snapshot captured before any mutation in the current phase.
	mkBarrier := func() op.Operation {
		return op.Operation{
			ID:   mkID("observe-barrier"),
			Kind: op.KindObserveBarrier,
			// No Target, no Preconditions, no ExpectedEffects.
			Risk: op.RiskMedium,
		}
	}
	ops := []op.Operation{}
	ops = append(ops, phaseRemovals...)
	if len(phaseRemovals) > 0 && (len(phaseSpawns) > 0 || len(phaseLayout) > 0) {
		ops = append(ops, mkBarrier())
	}
	ops = append(ops, phaseSpawns...)
	if len(phaseSpawns) > 0 && len(phaseLayout) > 0 {
		ops = append(ops, mkBarrier())
	}
	ops = append(ops, phaseLayout...)

	if plannerTraceEnabled() {
		kinds := make([]string, 0, len(ops))
		for _, o := range ops {
			kinds = append(kinds, string(o.Kind))
		}
		plannerTracef("emitted-ops count=%d kinds=%v (removals=%d spawns=%d layout=%d)",
			len(ops), kinds, len(phaseRemovals), len(phaseSpawns), len(phaseLayout))
	}

	return op.Plan{
		ID:         w.PlanID(fmt.Sprintf("plan-e%d", state.Meta.Epoch)),
		BaseEpoch:  state.Meta.Epoch,
		Operations: ops,
		Reason:     reason,
	}, nil
}

// desiredAIWindowExists returns true if target.Projects[id.Project].Windows
// contains the AI window twin that corresponds to a viewer DesiredWindowID
// (same Project + Index, Kind=WindowAI). Used by the residue removal guard:
// a viewer whose source AI has been removed from desired is a true orphan
// and must be removable even though the project is still active.
func desiredAIWindowExists(target w.DesiredWorld, id w.DesiredWindowID) bool {
	proj, ok := target.Projects[id.Project]
	if !ok {
		return false
	}
	for _, win := range proj.Windows {
		if win.Kind == w.WindowAI && win.ID.Index == id.Index {
			return true
		}
	}
	return false
}

func closeWindowBlocked(env w.ManagedEnvironment) bool {
	switch env.WindowManager.Backend {
	case "real", "omniwm":
		return true
	default:
		return false
	}
}

func viewerMatchedDesired(ow w.ObservedWindow, desired []w.DesiredWindowID, titles map[w.DesiredWindowID]string) (w.DesiredWindowID, bool) {
	// Trust ow.MatchedTo only when it still references a currently-desired
	// viewer. A stale MatchedTo (set when the project was active, kept
	// after archive/remove) would otherwise lock this live window into
	// the "matched but unwanted" branch and the planner would refuse to
	// remove it via IsProjectActive guard, causing ReorderColumns to
	// loop on got=N+1 want=N until the daemon is restarted. Drop into
	// the title fallback so orphan viewers can be removed cleanly.
	if ow.MatchedTo != nil {
		matched := *ow.MatchedTo
		for _, d := range desired {
			if d == matched {
				return matched, true
			}
		}
		// MatchedTo is stale — fall through to title matching.
	}
	// Title-based fallback: only return a match when exactly one desired
	// title equals the observed title. If two desired AIs share a title
	// (which violates uniqueness but can transiently happen during a
	// project rename), refuse to disambiguate so the planner does not
	// emit a removal targeting a wrong-project AI.
	var matched w.DesiredWindowID
	matches := 0
	for _, d := range desired {
		if ow.Title.Value == titles[d] {
			matched = d
			matches++
		}
	}
	if matches == 1 {
		return matched, true
	}
	return w.DesiredWindowID{}, false
}

// parseViewerTitle returns the (index, project) parsed out of a
// `ai-view-<N>:<project>` Ghostty viewer title, plus a boolean telling whether
// the parse succeeded. The Ghostty viewer title contract is a literal string
// produced by `viewerTitleForAI(expected)` where expected is itself the
// `ai-<N>:<project>` controller-owned AI title; the inverse here lets the
// planner correlate a residue viewer back to a specific project rather than
// guessing across projects.
func parseViewerTitle(title string) (index int, project string, ok bool) {
	const prefix = "ai-view-"
	if !strings.HasPrefix(title, prefix) {
		return 0, "", false
	}
	rest := title[len(prefix):]
	colon := strings.IndexByte(rest, ':')
	if colon <= 0 || colon == len(rest)-1 {
		return 0, "", false
	}
	idxStr := rest[:colon]
	proj := rest[colon+1:]
	idx, err := strconv.Atoi(idxStr)
	if err != nil || idx <= 0 {
		return 0, "", false
	}
	return idx, proj, true
}

func viewerLiveMatches(windows map[w.LiveWindowID]w.ObservedWindow, desired w.DesiredWindowID, title string) []w.LiveWindowID {
	var out []w.LiveWindowID
	for _, id := range sortedLiveIDs(windows) {
		ow := windows[id]
		if ow.Kind != w.WindowViewer {
			continue
		}
		if ow.MatchedTo != nil && *ow.MatchedTo == desired {
			out = append(out, id)
			continue
		}
		if ow.Title.Value == title {
			out = append(out, id)
		}
	}
	return out
}

func viewerMatchedAnyDesiredAI(ow w.ObservedWindow, target w.DesiredWorld) (w.DesiredWindowID, bool) {
	if ow.Kind != w.WindowViewer {
		return w.DesiredWindowID{}, false
	}
	if ow.MatchedTo != nil && desiredHas(target, *ow.MatchedTo) {
		return *ow.MatchedTo, true
	}
	// Parse the viewer title `ai-view-<N>:<project>` so the planner can
	// scope the match to the project encoded in the title rather than
	// scanning every project. Without scoping, a residue viewer left
	// over from a removed/renamed project could be matched against the
	// WRONG desired AI, causing the planner to emit a kill-session
	// targeting a project that is still active in the desired world
	// (which the executor refuses with `desired project ... is still
	// active`).
	_, projID, ok := parseViewerTitle(ow.Title.Value)
	if !ok {
		// Fall back to the original cross-project scan only when we
		// cannot parse a project hint out of the title. This preserves
		// the legacy behavior for titles whose authority is non-
		// controller-owned (defensive: production never emits these).
		for _, projectID := range sortedProjectIDs(target.Projects) {
			project := target.Projects[projectID]
			windows := append([]w.DesiredWindow(nil), project.Windows...)
			sort.Slice(windows, func(i, j int) bool {
				if windows[i].ID.Kind != windows[j].ID.Kind {
					return windows[i].ID.Kind < windows[j].ID.Kind
				}
				return windows[i].ID.Index < windows[j].ID.Index
			})
			for _, desired := range windows {
				if desired.ID.Kind != w.WindowAI {
					continue
				}
				if ow.Title.Value == viewerTitleForAI(desired.TitleContract.Expected) {
					return desired.ID, true
				}
			}
		}
		return w.DesiredWindowID{}, false
	}
	pr, exists := target.Projects[w.ProjectID(projID)]
	if !exists {
		return w.DesiredWindowID{}, false
	}
	// Within the project, find the unique desired AI window whose
	// expected title maps to this viewer title. Refuse to disambiguate
	// when more than one AI window collides on the same viewer title
	// (a title-contract violation) so the planner cannot mistakenly
	// emit a kill against an ambiguous identity.
	var matched w.DesiredWindowID
	matches := 0
	for _, desired := range pr.Windows {
		if desired.ID.Kind != w.WindowAI {
			continue
		}
		if viewerTitleForAI(desired.TitleContract.Expected) != ow.Title.Value {
			continue
		}
		matched = desired.ID
		matches++
	}
	if matches != 1 {
		return w.DesiredWindowID{}, false
	}
	return matched, true
}

func managedDesiredByObserved(ow w.ObservedWindow, target w.DesiredWorld) (w.DesiredWindowID, bool) {
	for _, projectID := range sortedProjectIDs(target.Projects) {
		project := target.Projects[projectID]
		windows := append([]w.DesiredWindow(nil), project.Windows...)
		sort.Slice(windows, func(i, j int) bool {
			if windows[i].ID.Kind != windows[j].ID.Kind {
				return windows[i].ID.Kind < windows[j].ID.Kind
			}
			return windows[i].ID.Index < windows[j].ID.Index
		})
		for _, desired := range windows {
			if desired.Kind != ow.Kind {
				continue
			}
			if desired.App.BundleID == "" || desired.App.BundleID != ow.App.BundleID {
				continue
			}
			// Browser windows have app-owned titles (URL-dependent), so we
			// permit bundle-only inference for them. The identity resolver
			// (used at mutation time) still requires unique-strong, so a
			// bundle-only inference here only enables planning when the
			// observed bundle is unambiguously the desired browser.
			if desired.TitleContract.Authority != w.TitleControllerOwned {
				if desired.Kind == w.WindowBrowser {
					return desired.ID, true
				}
				continue
			}
			if desired.TitleContract.Expected == "" {
				continue
			}
			if ow.Title.Value == desired.TitleContract.Expected {
				return desired.ID, true
			}
		}
	}
	return w.DesiredWindowID{}, false
}

// adoptableOnSlot reports whether a same-title, same-bundle window already sits
// on the desired window's slot workspace and should be ADOPTED instead of
// spawning a duplicate beside it (SSOT §6.9.1 B2 slot-territory adopt). This is
// the cold-start / recovery title→identity adoption that is gated STRICTLY to
// slot territory: windows on the user's own workspaces are never adopted (B3 /
// G2 / G3). It applies to single-process editor-class windows whose title is the
// project basename (TitleAppOwned with an Expected value); browser windows
// (B-05, URL-dependent titles) are excluded. A window already claimed by
// provenance for a DIFFERENT identity is not adoptable (it is a sibling's).
func adoptableOnSlot(dw w.DesiredWindow, observed w.ObservedWorld, slotWS w.WorkspaceID, provenance map[w.DesiredWindowID]w.LiveWindowID) bool {
	if dw.Kind == w.WindowBrowser {
		return false
	}
	if dw.App.BundleID == "" {
		return false
	}
	if dw.TitleContract.Expected == "" {
		return false
	}
	owned := map[w.LiveWindowID]bool{}
	for id, live := range provenance {
		if id != dw.ID && live != "" {
			owned[live] = true
		}
	}
	for _, id := range sortedLiveIDs(observed.Windows) {
		ow := observed.Windows[id]
		if ow.Workspace != slotWS {
			continue
		}
		if ow.App.BundleID != dw.App.BundleID {
			continue
		}
		if ow.Title.Value != dw.TitleContract.Expected {
			continue
		}
		if owned[id] {
			continue // a sibling identity's window
		}
		// A window already matched to a DIFFERENT active managed identity is
		// not a free adoption target.
		if ow.MatchedTo != nil && *ow.MatchedTo != dw.ID {
			continue
		}
		return true
	}
	return false
}

func viewerTitleForAI(aiTitle string) string {
	return "ai-view" + strings.TrimPrefix(aiTitle, "ai")
}

func sortedLiveIDs(m map[w.LiveWindowID]w.ObservedWindow) []w.LiveWindowID {
	out := make([]w.LiveWindowID, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func sortedProjectIDs(m map[w.ProjectID]w.DesiredProject) []w.ProjectID {
	out := make([]w.ProjectID, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// hasManualLayoutCandidate is removed (SSOT N-12). The planner no longer
// short-circuits replans based on candidate state; AcceptedLayouts is the
// sole authority for column placement.

func desiredHas(target w.DesiredWorld, id w.DesiredWindowID) bool {
	pr, ok := target.Projects[id.Project]
	if !ok {
		return false
	}
	for _, dw := range pr.Windows {
		if dw.ID == id {
			return true
		}
	}
	return false
}

func desiredWindowByID(target w.DesiredWorld, id w.DesiredWindowID) (*w.DesiredWindow, bool) {
	pr, ok := target.Projects[id.Project]
	if !ok {
		return nil, false
	}
	if dw := findDesiredWindow(pr, id); dw != nil {
		return dw, true
	}
	return nil, false
}

func lifecycleRemovalAllowed(env w.ManagedEnvironment, target w.DesiredWorld, id w.DesiredWindowID, observed w.ObservedWindow) bool {
	// SSOT §4.1 OP13 remove-window: 削除済 DesiredWindow に対する close も
	// 許可する必要がある。observed.App.BundleID / observed.Kind /
	// observed.Title をフォールバックとして使う。
	dw, hasDW := desiredWindowByID(target, id)

	switch observed.Kind {
	case w.WindowAI, w.WindowShell, w.WindowViewer, w.WindowEditor, w.WindowBrowser:
	default:
		return false
	}
	if hasDW && observed.Kind != w.WindowViewer && dw.Kind != observed.Kind {
		return false
	}

	// bundleID: desired を優先、なければ observed から
	bundleID := ""
	if hasDW {
		bundleID = dw.App.BundleID
	}
	if bundleID == "" {
		bundleID = observed.App.BundleID
	}
	if bundleID == "" {
		return false
	}
	app, ok := env.ManagedAppByBundle(bundleID)
	if !ok || !app.LifecycleRemoval.Allowed {
		return false
	}
	switch app.LifecycleRemoval.Method {
	case w.LifecycleRemovalAXCloseGuarded,
		w.LifecycleRemovalProjectScopedApp,
		w.LifecycleRemovalBrowserWindowClose:
		// Production-shaped close-window primitives are wired in the executor.
	default:
		return false
	}
	if !windowKindAllowed(app.LifecycleRemoval.AllowedKinds, observed.Kind) {
		return false
	}
	switch app.LifecycleRemoval.Method {
	case w.LifecycleRemovalAXCloseGuarded:
		// desired がある場合は controller-owned title 契約を要求 (規約上の安全
		// チェック)。desired が削除済の場合は observed.Title.Value が controller
		// 命名規約に従う prefix (e.g. "shell-N:project") であることで safety を担保。
		if hasDW {
			return dw.TitleContract.Authority == w.TitleControllerOwned && dw.TitleContract.Expected != ""
		}
		return observed.Title.Value != ""
	case w.LifecycleRemovalProjectScopedApp,
		w.LifecycleRemovalBrowserWindowClose:
		// app-owned title contracts (Zed/Vivaldi) do not require controller-owned title authority;
		// the executor relies on bundle-id + project/window correlation evidence collected by adapters.
		return true
	}
	return false
}

func windowKindAllowed(allowed []w.WindowKind, kind w.WindowKind) bool {
	for _, candidate := range allowed {
		if candidate == kind {
			return true
		}
	}
	return false
}

func findDesiredWindow(pr w.DesiredProject, id w.DesiredWindowID) *w.DesiredWindow {
	for i := range pr.Windows {
		if pr.Windows[i].ID == id {
			return &pr.Windows[i]
		}
	}
	return nil
}

func projectDesiredColumns(pr w.DesiredProject, ws w.WorkspaceID, accepted map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout) []w.DesiredColumn {
	if accepted != nil {
		if m, ok := accepted[pr.ID]; ok {
			if l, ok2 := m[ws]; ok2 {
				return l.Columns
			}
		}
	}
	if l, ok := pr.Layouts[ws]; ok {
		return l.Columns
	}
	// Default: each window (including AI) is its own solo column, sorted by (Kind, Index).
	all := append([]w.DesiredWindow(nil), pr.Windows...)
	sort.Slice(all, func(i, j int) bool {
		if all[i].ID.Kind != all[j].ID.Kind {
			return all[i].ID.Kind < all[j].ID.Kind
		}
		return all[i].ID.Index < all[j].ID.Index
	})
	out := make([]w.DesiredColumn, 0, len(all))
	for _, dw := range all {
		out = append(out, w.DesiredColumn{Windows: []w.DesiredWindowID{dw.ID}, Mode: w.ColumnSolo})
	}
	return out
}

// managedObservedColumns filters external/unmanaged windows out of the observed
// columns, keeping only windows in the managed set and dropping any column that
// becomes empty. SSOT §6.3 L3 reorder concerns only managed (desired) windows;
// the adapter-side counterpart is managedOrderSettled in the wm adapter.
func managedObservedColumns(cols []w.ObservedColumn, managed map[w.LiveWindowID]bool) []w.ObservedColumn {
	out := make([]w.ObservedColumn, 0, len(cols))
	for _, col := range cols {
		kept := make([]w.LiveWindowID, 0, len(col.Windows))
		for _, id := range col.Windows {
			if managed[id] {
				kept = append(kept, id)
			}
		}
		if len(kept) > 0 {
			out = append(out, w.ObservedColumn{Windows: kept, Mode: col.Mode})
		}
	}
	return out
}

func sameSemanticLayout(obs []w.ObservedColumn, want [][]w.LiveWindowID) bool {
	if len(obs) != len(want) {
		return false
	}
	for i := range obs {
		if len(obs[i].Windows) != len(want[i]) {
			return false
		}
		if len(want[i]) == 1 {
			if obs[i].Windows[0] != want[i][0] {
				return false
			}
			continue
		}
		seen := map[w.LiveWindowID]int{}
		for _, id := range obs[i].Windows {
			seen[id]++
		}
		for _, id := range want[i] {
			seen[id]--
		}
		for _, n := range seen {
			if n != 0 {
				return false
			}
		}
	}
	return true
}

func dcCopy(in []w.DesiredColumn) []w.DesiredColumn {
	out := make([]w.DesiredColumn, 0, len(in))
	for _, c := range in {
		out = append(out, w.DesiredColumn{
			Windows: append([]w.DesiredWindowID(nil), c.Windows...),
			Mode:    c.Mode,
		})
	}
	return out
}

func liveCandidatesForActiveDesired(observed w.ObservedWorld, target w.DesiredWorld, provenance map[w.DesiredWindowID]w.LiveWindowID) map[w.LiveWindowID]bool {
	protected := map[w.LiveWindowID]bool{}
	for _, pid := range sortedProjectIDs(target.Projects) {
		pr := target.Projects[pid]
		if pr.Archived || !target.IsProjectActive(pid) {
			continue
		}
		for _, dw := range pr.Windows {
			res := identity.Resolve(dw, observed)
			// SSOT §6.9.1: when an active identity resolves AMBIGUOUS over
			// several same-title siblings (e.g. two Zed editors titled by the
			// project basename), provenance tells us WHICH live window actually
			// belongs to this identity. Protect ONLY that provenance window so a
			// sibling orphan — one whose desired identity has been removed
			// (intent.RemoveWindow), archived, or deactivated — is left
			// unprotected and the removal loop can close it. Without this, every
			// same-title candidate is protected and the orphan never closes
			// (RemoveWindow silently fails to close the removed window).
			//
			// Narrowing is ONLY safe when provenance unambiguously names one of
			// the live candidates; otherwise (cold start / stale provenance with
			// no live match) we fall back to protecting EVERY candidate so we
			// never close a window we cannot prove is a sibling rather than this
			// identity's own (preserves B3/G2/G3 inviolability).
			if res.Class == identity.ClassAmbiguous && provenance != nil {
				if prov, ok := provenance[dw.ID]; ok && prov != "" {
					provIsCandidate := false
					for _, live := range res.Candidates {
						if live == prov {
							provIsCandidate = true
							break
						}
					}
					if provIsCandidate {
						protected[prov] = true
						continue
					}
				}
			}
			for _, live := range res.Candidates {
				protected[live] = true
			}
			if res.Live != "" {
				protected[res.Live] = true
			}
		}
	}
	return protected
}

func spawnKindFor(k w.WindowKind) op.Kind {
	switch k {
	case w.WindowEditor:
		return op.KindSpawnEditor
	case w.WindowBrowser:
		return op.KindSpawnBrowser
	case w.WindowAI:
		// AI windows are spawned as terminal-class (controller-owned title).
		return op.KindSpawnTerminal
	case w.WindowShell:
		return op.KindSpawnTerminal
	case w.WindowViewer:
		return op.KindSpawnViewer
	default:
		return op.KindSpawnTerminal
	}
}

func spawnKindHint(k w.WindowKind) string {
	return string(k)
}

func idPtr(id w.LiveWindowID) *w.LiveWindowID {
	c := id
	return &c
}

// displayForParkWorkspace resolves the physical DisplayID that owns a given
// park workspace (e.g. "CP2"). It first consults ObservedDisplayState.WorkspaceToDisplay
// (populated by sigwm.Observe from live window placement, which is authoritative
// for app-rule-assigned CPn workspaces). Falls back to scanning display active
// workspaces when the window-placement map is unavailable (e.g. fake adapter in tests).
// Returns ("", false) when the display cannot be determined.
func displayForParkWorkspace(obs w.ObservedDisplayState, parkWs w.WorkspaceID) (w.DisplayID, bool) {
	if parkWs == "" {
		return "", false
	}
	// Primary source: window-placement map (set by sigwm from actual window data).
	if obs.WorkspaceToDisplay != nil {
		if id, ok := obs.WorkspaceToDisplay[parkWs]; ok {
			return id, true
		}
	}
	// Fallback: find a display whose activeWorkspace matches (works when the
	// display is currently showing its park workspace).
	for id, d := range obs.Displays {
		if d.ActiveWorkspace == parkWs {
			return id, true
		}
	}
	return "", false
}

// sortedDisplayIDs returns the display IDs from ObservedDisplayState in a
// stable order (main first, then by ID string) matching the order used by
// sigwm.sortDisplaysForCockpit so DisplayIdx maps correctly.
func sortedDisplayIDs(obs w.ObservedDisplayState) []w.DisplayID {
	ids := make([]w.DisplayID, 0, len(obs.Displays))
	for id := range obs.Displays {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		isPrimaryI := obs.Primary != nil && *obs.Primary == ids[i]
		isPrimaryJ := obs.Primary != nil && *obs.Primary == ids[j]
		if isPrimaryI != isPrimaryJ {
			return isPrimaryI
		}
		return ids[i] < ids[j]
	})
	return ids
}

// planCockpitOps walks DesiredWorld.SystemWindows + ObservedWorld and
// emits the necessary spawn / show-cockpit / hide-cockpit / close
// operations. Unified design v2 §5 — park-workspace model.
func planCockpitOps(state w.WorldState, target w.DesiredWorld,
	phaseRemovals, phaseSpawns, phaseLayout *[]op.Operation,
	mkID func(string) w.OperationID,
) {
	// Index observed cockpit windows by title for O(1) lookup.
	observedCockpit := map[string]w.ObservedWindow{}
	for _, ow := range state.Observed.Windows {
		if ow.Kind != w.WindowCockpit {
			continue
		}
		observedCockpit[ow.Title.Value] = ow
	}

	// Track which observed cockpit titles still belong to a SystemWindow,
	// so we can close leftovers (display unplug).
	stillWanted := map[string]bool{}

	// 1) For each desired cockpit, spawn-if-missing; then check show/hide convergence.
	for _, sw := range target.SystemWindows {
		if sw.Kind != w.WindowCockpit {
			continue
		}
		stillWanted[sw.Title] = true
		ow, ok := observedCockpit[sw.Title]
		if !ok {
			// Spawn op. Executor reads SystemWindow target to build open -na + tmux args.
			swID := sw.ID
			*phaseSpawns = append(*phaseSpawns, op.Operation{
				ID:   mkID("spawn-cockpit"),
				Kind: op.KindSpawnCockpit,
				Target: op.Target{
					SystemWindow: &swID,
				},
				ExpectedEffects: []op.Effect{
					{Kind: op.EffectSpawnWindow, WindowKind: w.WindowCockpit, SystemWindow: &swID},
				},
				Risk:           op.RiskMedium,
				IdempotencyKey: fmt.Sprintf("spawn-cockpit:%d", sw.DisplayIdx),
			})
			continue
		}

		// v2.8 §8.10 Cockpit invariant — Tier 4 強制 revert.
		// cockpit window が ParkWorkspace 以外に居る (omniwm restart 後の
		// app-rule 未発火 / ユーザの手動 drag / 何らかの move 副作用) なら、
		// MoveCockpitToParkWorkspace op を emit して必ず CP1 へ戻す。
		// この op は ShowCockpit (display.activeWorkspace 切替) より優先度高く、
		// phaseLayout の先頭で実行されるべきだが、現状の phase 分離では
		// phaseLayout に enqueue する。executor 内で window 矯正→display 切替
		// の順で並ぶことで、CP1 が active になった display に cockpit window
		// が居る正しい状態に収束する。
		if sw.ParkWorkspace != "" && ow.Workspace != sw.ParkWorkspace {
			swID := sw.ID
			parkCopy := sw.ParkWorkspace
			liveID := ow.ID
			*phaseLayout = append(*phaseLayout, op.Operation{
				ID:   mkID("move-cockpit-to-park"),
				Kind: op.KindMoveCockpitToParkWorkspace,
				Target: op.Target{
					SystemWindow: &swID,
					LiveWindow:   &liveID,
					Workspace:    &parkCopy,
				},
				Risk:           op.RiskMedium,
				IdempotencyKey: fmt.Sprintf("move-cockpit-to-park:%d", sw.DisplayIdx),
			})
		}

		// Window exists. Resolve the display ID for this SystemWindow.
		// Primary: use park-workspace ownership via WorkspaceToDisplay
		// (populated by sigwm.Observe from live window placement data).
		// This correctly handles non-contiguous display IDs like display:1,
		// display:2, display:5 where alphabetical order ≠ CP ownership order.
		// Fallback: use sorted display index (works for fake adapter in tests
		// and for backward compatibility when WorkspaceToDisplay is not set).
		sortedDisplays := sortedDisplayIDs(state.Observed.Displays)
		var displayID w.DisplayID
		foundDisplay := false
		if id, ok := displayForParkWorkspace(state.Observed.Displays, sw.ParkWorkspace); ok {
			displayID = id
			foundDisplay = true
		} else if sw.DisplayIdx >= 0 && sw.DisplayIdx < len(sortedDisplays) {
			displayID = sortedDisplays[sw.DisplayIdx]
			foundDisplay = true
		}
		if !foundDisplay {
			// Display not yet observed — defer until next reconcile.
			continue
		}
		obsDisplay, hasDisplay := state.Observed.Displays.Displays[displayID]
		if !hasDisplay {
			continue
		}
		observedActiveWs := obsDisplay.ActiveWorkspace

		swID := sw.ID
		parkWs := sw.ParkWorkspace
		priorWs := sw.PriorWorkspace

		switch sw.Visibility {
		case w.CockpitShown:
			// Want cockpit visible: display should be on ParkWorkspace.
			if parkWs != "" && observedActiveWs != parkWs {
				wsCopy := parkWs
				*phaseLayout = append(*phaseLayout, op.Operation{
					ID:   mkID("show-cockpit"),
					Kind: op.KindShowCockpit,
					Target: op.Target{
						SystemWindow: &swID,
						Workspace:    &wsCopy, // ParkWorkspace
					},
					Risk:           op.RiskLow,
					IdempotencyKey: fmt.Sprintf("show-cockpit:%d", sw.DisplayIdx),
				})
			}
		case w.CockpitHidden:
			// Want cockpit hidden: display should NOT be on ParkWorkspace.
			if parkWs != "" && observedActiveWs == parkWs {
				// PriorWorkspace may be empty — that's fine. The executor's
				// HideCockpitOnDisplay falls back to omniwm's per-display
				// `switch-workspace back-and-forth` history, which records
				// the natural workspace prior to SpawnCockpit's pre-focus.
				//
				// SSOT §7.5 HideCockpitOnDisplay: focus restoration target is
				// the PriorWindow captured in SystemWindow. Propagating it
				// here as Target.LiveWindow lets the executor restore focus
				// after the workspace switch.
				wsCopy := priorWs
				op := op.Operation{
					ID:   mkID("hide-cockpit"),
					Kind: op.KindHideCockpit,
					Target: op.Target{
						SystemWindow: &swID,
						Workspace:    &wsCopy, // PriorWorkspace (empty → back-and-forth fallback)
					},
					Risk:           op.RiskLow,
					IdempotencyKey: fmt.Sprintf("hide-cockpit:%d", sw.DisplayIdx),
				}
				if sw.PriorWindow != "" {
					priorWin := sw.PriorWindow
					op.Target.LiveWindow = &priorWin
				}
				*phaseLayout = append(*phaseLayout, op)
			}
		}
	}

	// 2) Close cockpit windows whose title no longer maps to a
	//    SystemWindow (display unplug / DisplayCount shrink).
	for title, ow := range observedCockpit {
		if stillWanted[title] {
			continue
		}
		lid := ow.ID
		*phaseRemovals = append(*phaseRemovals, op.Operation{
			ID:     mkID("close-cockpit"),
			Kind:   op.KindCloseCockpit,
			Target: op.Target{LiveWindow: idPtr(lid)},
			Risk:   op.RiskMedium,
		})
	}
}

// fmtDWIDPtr renders a *DesiredWindowID for diagnostics ("nil" when absent).
func fmtDWIDPtr(p *w.DesiredWindowID) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprintf("%s/%v/%d", p.Project, p.Kind, p.Index)
}

// planSummonViewerOps emits focus ops realising SSOT §4.1 OP06.
//
// 動作:
//  1. 起動時点で focused window が AI なら、その (project, index) に対応する
//     viewer DesiredWindow を target にする。
//  2. それ以外 (shell/editor/browser/cockpit/scratch/未管理) のときは、
//     active profile の slot 順序の最初の slot に住む project の viewer-1 を
//     target にする (INV-12 viewer order follows slot order)。
//  3. target に該当する live viewer window が observed に居れば、
//     focus-workspace (viewer ws) + focus-window (viewer live) op を emit。
//  4. 該当 viewer が未だ spawn されていない場合は no-op (transaction loop の
//     次の cycle で spawn が走る前提)。
//
// command != "intent:summon-viewer" のときは何もしない。
func planSummonViewerOps(state w.WorldState, target w.DesiredWorld, command CommandKey,
	phaseLayout *[]op.Operation, mkID func(string) w.OperationID,
) {
	if string(command) != "intent:summon-viewer" {
		return
	}
	viewerWs := state.Environment.Workspaces.Viewer
	if viewerWs == "" {
		return
	}

	// Step 1+2: identify the AI window whose viewer to focus. A viewer mirror
	// SHARES its AI source's DesiredWindowID (internal/identity: "Viewer mirrors
	// share DesiredWindowID with their AI source but have Kind=WindowViewer"), so
	// the target is identified by the AI's DesiredWindowID (Kind=AI) + the live
	// window's Kind=Viewer — NOT a Kind=Viewer DesiredWindowID. No observed window
	// ever carries a Kind=Viewer MatchedTo: viewers are not part of a project's
	// Windows (they are derived from its AI windows by the viewer-maintenance
	// section above), so the previous Kind=Viewer lookup matched nothing and the
	// summon-viewer focus op was never emitted (OP-06 jump root).
	var targetAI *w.DesiredWindowID
	if focusedID := state.Observed.Focus.Window; focusedID != "" {
		// Focused on an AI window (or its viewer mirror, which shares the AI's
		// DesiredWindowID): summon that AI's viewer.
		if ow, ok := state.Observed.Windows[focusedID]; ok && ow.MatchedTo != nil && ow.MatchedTo.Kind == w.WindowAI {
			targetAI = ow.MatchedTo
		}
	}
	if targetAI == nil {
		// Fallback: first AI in slot order of the active profile (INV-12: viewer
		// order follows slot order).
		prof, ok := target.ActiveProfileObj()
		if !ok {
			return
		}
		for _, slotID := range state.Environment.SlotOrder() {
			projID, assigned := prof.Assignments[slotID]
			if !assigned {
				continue
			}
			proj, ok := target.Projects[projID]
			if !ok || proj.Archived {
				continue
			}
			var firstAI *w.DesiredWindowID
			for i := range proj.Windows {
				if proj.Windows[i].Kind != w.WindowAI {
					continue
				}
				if firstAI == nil || proj.Windows[i].ID.Index < firstAI.Index {
					id := proj.Windows[i].ID
					firstAI = &id
				}
			}
			if firstAI != nil {
				targetAI = firstAI
				break
			}
		}
	}
	if targetAI == nil {
		return
	}

	// Step 3: resolve the live VIEWER window mirroring targetAI. Real viewers are
	// NOT in a project's desired Windows, so identity never populates their
	// MatchedTo (it stays nil); they are identified by their mirror TITLE —
	// viewerTitleForAI of the AI's expected title — exactly as the viewer-
	// maintenance section above does (viewerMatchedDesired). Accept either signal:
	// MatchedTo == targetAI (flows/fixtures that link the mirror) OR the mirror
	// title on the viewer workspace.
	viewerTitle := ""
	if proj, ok := target.Projects[targetAI.Project]; ok {
		for i := range proj.Windows {
			if proj.Windows[i].ID == *targetAI {
				viewerTitle = viewerTitleForAI(proj.Windows[i].TitleContract.Expected)
				break
			}
		}
	}
	var targetLive w.LiveWindowID
	for id, ow := range state.Observed.Windows {
		if ow.Kind != w.WindowViewer {
			continue
		}
		byMatch := ow.MatchedTo != nil && *ow.MatchedTo == *targetAI
		byTitle := viewerTitle != "" && ow.Title.Value == viewerTitle && ow.Workspace == viewerWs
		if byMatch || byTitle {
			targetLive = id
			break
		}
	}
	if plannerTraceEnabled() {
		plannerTracef("summon-viewer: targetAI=%s viewerTitle=%q targetLive=%q", fmtDWIDPtr(targetAI), viewerTitle, targetLive)
	}
	if targetLive == "" {
		// Viewer not yet spawned — let transaction loop spawn on next cycle.
		return
	}

	// Step 4: emit focus-workspace + focus-window ops.
	wsCopy := viewerWs
	if state.Observed.Focus.Workspace != viewerWs {
		*phaseLayout = append(*phaseLayout, op.Operation{
			ID:     mkID("focus-viewer-ws"),
			Kind:   op.KindFocusWorkspace,
			Target: op.Target{Workspace: &wsCopy},
			Preconditions: []op.Precondition{
				{Kind: op.PreWorkspaceExists, Target: op.Target{Workspace: &wsCopy}},
			},
			ExpectedEffects: []op.Effect{{Kind: op.EffectFocusWorkspace, FocusedWS: &wsCopy}},
			Risk:            op.RiskLow,
			IdempotencyKey:  "summon-viewer:focus-ws",
		})
	}
	if state.Observed.Focus.Window != targetLive {
		liveCopy := targetLive
		*phaseLayout = append(*phaseLayout, op.Operation{
			ID:     mkID("focus-viewer-window"),
			Kind:   op.KindFocusWindow,
			Target: op.Target{LiveWindow: &liveCopy},
			Preconditions: []op.Precondition{
				{Kind: op.PreWindowExists, Target: op.Target{LiveWindow: &liveCopy}},
			},
			ExpectedEffects: []op.Effect{{Kind: op.EffectFocusWindow, FocusedWin: &liveCopy}},
			Risk:            op.RiskLow,
			IdempotencyKey:  "summon-viewer:focus-window",
		})
	}
}

// planSummonWindowOps realises SSOT §4.1 OP01-03 (summon-shell / summon-editor
// / summon-browser). 共通ロジック:
//
//  1. commandKey "intent:summon-<kind>:<slot>" を parse して target kind + slot
//     を取得する。
//  2. slot に assigned された project を active profile から resolve。
//  3. 既存 focus が同じ (project, kind) なら次の index に cycle (最後の index
//     なら 1 にラップ)。それ以外は index=1 を target にする。
//  4. target DesiredWindowID を observed.Windows.MatchedTo で LiveWindow に
//     逆引きし、focus-workspace + focus-window op を emit。
//  5. target window が未 spawn なら no-op (transaction loop の次の cycle で
//     spawn が走る前提)。
func planSummonWindowOps(state w.WorldState, target w.DesiredWorld, command CommandKey,
	phaseLayout *[]op.Operation, mkID func(string) w.OperationID,
) {
	cmd := string(command)
	var kind w.WindowKind
	var prefix string
	switch {
	case strings.HasPrefix(cmd, "intent:summon-shell:"):
		kind, prefix = w.WindowShell, "intent:summon-shell:"
	case strings.HasPrefix(cmd, "intent:summon-editor:"):
		kind, prefix = w.WindowEditor, "intent:summon-editor:"
	case strings.HasPrefix(cmd, "intent:summon-browser:"):
		kind, prefix = w.WindowBrowser, "intent:summon-browser:"
	default:
		return
	}
	slotID := w.SlotID(strings.TrimPrefix(cmd, prefix))
	if slotID == "" {
		return
	}

	// Step 2: resolve slot → project via active profile.
	prof, ok := target.ActiveProfileObj()
	if !ok {
		return
	}
	projID, assigned := prof.Assignments[slotID]
	if !assigned {
		return
	}
	proj, ok := target.Projects[projID]
	if !ok {
		return
	}

	// Collect candidate window indices for the kind, sorted ascending.
	indices := []int{}
	for _, win := range proj.Windows {
		if win.Kind == kind {
			indices = append(indices, win.ID.Index)
		}
	}
	if len(indices) == 0 {
		return
	}
	sort.Ints(indices)

	// Step 3: cycle resolution. The "am I already on this (project,kind) →
	// cycle to the next index" decision must be anchored to the focus as it
	// was when the user pressed the key (transaction start), NOT the live
	// focus — the converge loop replans against the focus this very op sets,
	// so reading live focus here would advance the target every replan and
	// alternate forever across 2+ windows (max-replans fail). Meta.Summon
	// FocusAnchor carries the frozen transaction-start focus; outside a
	// transaction (planner unit tests) it is empty and we fall back to live
	// observed focus.
	targetIndex := indices[0]
	// In a transaction the anchor is authoritative (frozen at start, may be
	// empty if the user had no focus); only standalone planner unit tests
	// (no Transaction) fall back to live observed focus.
	focusedID := state.Observed.Focus.Window
	if state.Meta.Transaction != nil {
		focusedID = state.Meta.SummonFocusAnchor
	}
	if focusedID != "" {
		if ow, ok := state.Observed.Windows[focusedID]; ok && ow.MatchedTo != nil &&
			ow.MatchedTo.Project == projID && ow.MatchedTo.Kind == kind {
			// Currently focused is in target (project, kind). Cycle next.
			currentIdx := ow.MatchedTo.Index
			for i, idx := range indices {
				if idx == currentIdx {
					targetIndex = indices[(i+1)%len(indices)]
					break
				}
			}
		}
	}
	targetDesired := w.DesiredWindowID{Project: projID, Kind: kind, Index: targetIndex}

	// Step 4: resolve target DesiredWindowID to LiveWindowID.
	var targetLive w.LiveWindowID
	var targetWS w.WorkspaceID
	for id, ow := range state.Observed.Windows {
		if ow.MatchedTo == nil {
			continue
		}
		if *ow.MatchedTo == targetDesired {
			targetLive = id
			targetWS = ow.Workspace
			break
		}
	}
	if targetLive == "" {
		// Target window not yet observed — let transaction loop spawn later.
		return
	}

	// Step 5: emit focus-workspace + focus-window ops if not already there.
	if targetWS != "" && state.Observed.Focus.Workspace != targetWS {
		wsCopy := targetWS
		*phaseLayout = append(*phaseLayout, op.Operation{
			ID:     mkID("focus-summon-ws"),
			Kind:   op.KindFocusWorkspace,
			Target: op.Target{Workspace: &wsCopy},
			Preconditions: []op.Precondition{
				{Kind: op.PreWorkspaceExists, Target: op.Target{Workspace: &wsCopy}},
			},
			ExpectedEffects: []op.Effect{{Kind: op.EffectFocusWorkspace, FocusedWS: &wsCopy}},
			Risk:            op.RiskLow,
			IdempotencyKey:  "summon-window:focus-ws:" + string(slotID),
		})
	}
	if state.Observed.Focus.Window != targetLive {
		liveCopy := targetLive
		*phaseLayout = append(*phaseLayout, op.Operation{
			ID:     mkID("focus-summon-window"),
			Kind:   op.KindFocusWindow,
			Target: op.Target{LiveWindow: &liveCopy},
			Preconditions: []op.Precondition{
				{Kind: op.PreWindowExists, Target: op.Target{LiveWindow: &liveCopy}},
			},
			ExpectedEffects: []op.Effect{{Kind: op.EffectFocusWindow, FocusedWin: &liveCopy}},
			Risk:            op.RiskLow,
			IdempotencyKey:  "summon-window:focus-window:" + string(slotID) + ":" + string(kind),
		})
	}
}

// planSwitchProjectOps realises SSOT §4.1 OP04: target slot の workspace に
// focus を移す。omniwm の per-workspace MRU が「直前 focused だった managed
// window」への復帰を担当するので、ここでは focus-workspace op のみを emit。
//
// Slot が active profile で assign されていない場合、target project が
// 居住する workspace が存在しないので no-op (no focus-workspace に).
func planSwitchProjectOps(state w.WorldState, target w.DesiredWorld, command CommandKey,
	phaseLayout *[]op.Operation, mkID func(string) w.OperationID,
) {
	cmd := string(command)
	const prefix = "intent:switch-project:"
	if !strings.HasPrefix(cmd, prefix) {
		return
	}
	slotID := w.SlotID(strings.TrimPrefix(cmd, prefix))
	if slotID == "" {
		return
	}
	// Resolve slot → workspace via environment.
	spec, ok := state.Environment.SlotByID(slotID)
	if !ok || spec.Workspace == "" {
		return
	}
	if state.Observed.Focus.Workspace == spec.Workspace {
		return // already on target workspace
	}
	// Sanity: slot must be assigned in active profile (else nothing to switch to).
	prof, ok := target.ActiveProfileObj()
	if !ok {
		return
	}
	if _, assigned := prof.Assignments[slotID]; !assigned {
		return
	}
	wsCopy := spec.Workspace
	*phaseLayout = append(*phaseLayout, op.Operation{
		ID:     mkID("focus-switch-project"),
		Kind:   op.KindFocusWorkspace,
		Target: op.Target{Workspace: &wsCopy},
		Preconditions: []op.Precondition{
			{Kind: op.PreWorkspaceExists, Target: op.Target{Workspace: &wsCopy}},
		},
		ExpectedEffects: []op.Effect{{Kind: op.EffectFocusWorkspace, FocusedWS: &wsCopy}},
		Risk:            op.RiskLow,
		IdempotencyKey:  "switch-project:focus-ws:" + string(slotID),
	})
}

// planCycleSlotWindowOps realises SSOT §4.1 OP05: 同じ slot 内で kind 別に
// focus を移す (workspace は変えない)。
//
// commandKey format: "intent:cycle-slot-window:<slot>:<kind>"
//
//   - slot から active profile 経由で project を resolve
//   - target = (project, kind, 1) — または現 focus が同 (project, kind, N)
//     なら index N+1 にラップ cycle (OP01-03 と同様)
//   - target DesiredWindowID → LiveWindowID 逆引き
//   - focus-window op のみ emit (focus-workspace は SSOT 「current_ws 変わら
//     ない」契約により emit しない)
func planCycleSlotWindowOps(state w.WorldState, target w.DesiredWorld, command CommandKey,
	phaseLayout *[]op.Operation, mkID func(string) w.OperationID,
) {
	cmd := string(command)
	const prefix = "intent:cycle-slot-window:"
	if !strings.HasPrefix(cmd, prefix) {
		return
	}
	rest := strings.TrimPrefix(cmd, prefix)
	// rest = "<slot>:<kind>"
	colonIdx := strings.IndexByte(rest, ':')
	if colonIdx <= 0 || colonIdx == len(rest)-1 {
		return
	}
	slotID := w.SlotID(rest[:colonIdx])
	kind := w.WindowKind(rest[colonIdx+1:])

	prof, ok := target.ActiveProfileObj()
	if !ok {
		return
	}
	projID, assigned := prof.Assignments[slotID]
	if !assigned {
		return
	}
	proj, ok := target.Projects[projID]
	if !ok {
		return
	}

	indices := []int{}
	for _, win := range proj.Windows {
		if win.Kind == kind {
			indices = append(indices, win.ID.Index)
		}
	}
	if len(indices) == 0 {
		return
	}
	sort.Ints(indices)

	// Cycle anchored to transaction-start focus (see planSummonWindowOps):
	// reading live focus here would re-cycle off the focus we just set and
	// alternate forever across 2+ candidates inside the replan loop.
	targetIndex := indices[0]
	focusedID := state.Observed.Focus.Window
	if state.Meta.Transaction != nil {
		focusedID = state.Meta.SummonFocusAnchor
	}
	if focusedID != "" {
		if ow, ok := state.Observed.Windows[focusedID]; ok && ow.MatchedTo != nil &&
			ow.MatchedTo.Project == projID && ow.MatchedTo.Kind == kind {
			currentIdx := ow.MatchedTo.Index
			for i, idx := range indices {
				if idx == currentIdx {
					targetIndex = indices[(i+1)%len(indices)]
					break
				}
			}
		}
	}
	targetDesired := w.DesiredWindowID{Project: projID, Kind: kind, Index: targetIndex}

	var targetLive w.LiveWindowID
	for id, ow := range state.Observed.Windows {
		if ow.MatchedTo != nil && *ow.MatchedTo == targetDesired {
			targetLive = id
			break
		}
	}
	if targetLive == "" || state.Observed.Focus.Window == targetLive {
		return
	}
	liveCopy := targetLive
	*phaseLayout = append(*phaseLayout, op.Operation{
		ID:     mkID("focus-cycle-slot-window"),
		Kind:   op.KindFocusWindow,
		Target: op.Target{LiveWindow: &liveCopy},
		Preconditions: []op.Precondition{
			{Kind: op.PreWindowExists, Target: op.Target{LiveWindow: &liveCopy}},
		},
		ExpectedEffects: []op.Effect{{Kind: op.EffectFocusWindow, FocusedWin: &liveCopy}},
		Risk:            op.RiskLow,
		IdempotencyKey:  "cycle-slot-window:focus:" + string(slotID) + ":" + string(kind),
	})
}

// planScratchOps emits show/hide ops for the scratch SystemWindow.
// SSOT §4.1 OP11 + §7.5 ShowScratchShell/HideScratchShell.
//
// 収束ロジック:
//   - Visibility=Shown かつ observed.Focus.Window != scratch live window →
//     KindShowScratchShell を emit
//   - Visibility=Hidden かつ observed.Focus.Window == scratch live window →
//     KindHideScratchShell を emit (Target.LiveWindow = PriorWindow)
//   - それ以外は no-op (収束済)
//
// scratch ghostty が未存在 (observed に projwm-scratch-shell title が無い)
// 場合は ShowScratchShell adapter 自身が spawn を担当するので、Visibility=Shown
// のときは常に show op を発行する (adapter が冪等)。
func planScratchOps(state w.WorldState, target w.DesiredWorld,
	phaseLayout *[]op.Operation,
	mkID func(string) w.OperationID,
) {
	var scratchSW *w.SystemWindow
	for i := range target.SystemWindows {
		if target.SystemWindows[i].Kind == w.WindowScratch {
			scratchSW = &target.SystemWindows[i]
			break
		}
	}
	if scratchSW == nil {
		return
	}

	// Resolve scratch live window from observed (by canonical title).
	var scratchLive w.LiveWindowID
	for id, ow := range state.Observed.Windows {
		if ow.Title.Value == "projwm-scratch-shell" {
			scratchLive = id
			break
		}
	}
	focusedOnScratch := scratchLive != "" && state.Observed.Focus.Window == scratchLive

	switch scratchSW.Visibility {
	case w.CockpitShown:
		// 既に scratch にフォーカスがある場合は no-op (収束済)。
		if focusedOnScratch {
			return
		}
		*phaseLayout = append(*phaseLayout, op.Operation{
			ID:             mkID("show-scratch-shell"),
			Kind:           op.KindShowScratchShell,
			Risk:           op.RiskLow,
			IdempotencyKey: "show-scratch-shell",
		})
	case w.CockpitHidden:
		// scratch がそもそも focus されていなければ何もしない。
		if !focusedOnScratch {
			return
		}
		opv := op.Operation{
			ID:             mkID("hide-scratch-shell"),
			Kind:           op.KindHideScratchShell,
			Risk:           op.RiskLow,
			IdempotencyKey: "hide-scratch-shell",
		}
		if scratchSW.PriorWindow != "" {
			prior := scratchSW.PriorWindow
			opv.Target.LiveWindow = &prior
		}
		*phaseLayout = append(*phaseLayout, opv)
	}
}
