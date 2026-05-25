// Package executor invokes the WindowManagerAdapter for each Operation.
// Preconditions are checked just before mutation. design.md §11. Single-writer.
package executor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/adapter/zed"
	"github.com/yuu-th/projwm-next/internal/identity"
	"github.com/yuu-th/projwm-next/internal/lifecyclecontract"
	"github.com/yuu-th/projwm-next/internal/naming"
	"github.com/yuu-th/projwm-next/internal/op"
	"github.com/yuu-th/projwm-next/internal/semop"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// observeBarrierSleep is the dwell time the executor waits before re-querying
// the adapter for KindObserveBarrier. It is sized for OmniWM/AX disappearance
// propagation observed in production: the SkyLight/AX trees occasionally
// retain a closed window's identity for a few hundred milliseconds after the
// close syscall returns, which trips PreUniqueStrong on the next op when ops
// of opposite phases are issued back-to-back.
const observeBarrierSleep = 500 * time.Millisecond

// VivaldiCloser is the privileged Vivaldi browser-window-close surface the
// Executor uses when handling kill-session operations whose
// LifecycleRemovalMethod is browser-window-close. It is the subset of
// browser.VivaldiAdapter that the Executor actually needs, kept as an
// interface so unit tests can stub the privileged path.
type VivaldiCloser interface {
	CollectCloseObservation(ctx context.Context, params browser.CloseObservationParams) (browser.VivaldiCloseObservation, error)
	CloseLiveWindow(ctx context.Context, live w.LiveWindowID) error
}

// ZedCloser is the privileged Zed project-scoped removal surface the Executor
// uses when handling kill-session operations whose LifecycleRemovalMethod is
// project-scoped-app for the Zed bundle. It is the subset of *zed.Adapter the
// Executor actually needs, kept as an interface so unit tests can stub the
// privileged path without spawning osascript subprocesses.
type ZedCloser interface {
	CollectCloseObservation(ctx context.Context, params zed.CloseObservationParams) (zed.CloseObservation, error)
	CloseLiveWindow(ctx context.Context, live w.LiveWindowID) error
}

type Executor struct {
	Adapter wm.Adapter
	Env     w.ManagedEnvironment
	// Vivaldi is the lifecycle-removal closer for browser-window-close
	// kill-session ops. When unset, browser-window-close is rejected so the
	// Executor never bypasses the lifecycle contract by silently falling back
	// to the WindowManagerAdapter.
	Vivaldi VivaldiCloser
	// Zed is the lifecycle-removal closer for project-scoped-app kill-session
	// ops targeting the Zed bundle. When unset, project-scoped-app removal is
	// rejected so the Executor never bypasses the lifecycle contract by
	// silently falling back to the WindowManagerAdapter.
	Zed ZedCloser
}

// Execute applies a single Operation against the adapter using the latest observed world.
// Returns nil on success, or an error if a precondition fails or the adapter rejects.
func (e *Executor) Execute(ctx context.Context, oper op.Operation, observed w.ObservedWorld, target w.DesiredWorld) error {
	// KindObserveBarrier is a non-mutating sequencing primitive: sleep briefly
	// to let async AX/SkyLight propagation complete after the previous phase's
	// mutations, then refresh the adapter observation. The result is consumed
	// by the controller's Settler.Settle on the next op (controller refreshes
	// observation between every op), so we just need to ensure the adapter's
	// internal cache (if any) is warm and that the dwell has elapsed before
	// the next precondition check fires. Returning nil signals success.
	if oper.Kind == op.KindObserveBarrier {
		select {
		case <-time.After(observeBarrierSleep):
		case <-ctx.Done():
			return ctx.Err()
		}
		if e.Adapter != nil {
			if _, err := e.Adapter.Observe(ctx); err != nil {
				return fmt.Errorf("executor: observe-barrier: %w", err)
			}
		}
		return nil
	}
	if oper.Kind == op.KindCloseWindow && closeWindowBlocked(e.Env) {
		return fmt.Errorf("executor: close-window is blocked by first-implementation production safety policy")
	}
	findDesiredWindow := func(d w.DesiredWindowID) (*w.DesiredProject, *w.DesiredWindow, error) {
		pr, ok := target.Projects[d.Project]
		if !ok {
			return nil, nil, fmt.Errorf("executor: desired project %q unknown", d.Project)
		}
		for i := range pr.Windows {
			if pr.Windows[i].ID == d {
				return &pr, &pr.Windows[i], nil
			}
		}
		return nil, nil, fmt.Errorf("executor: desired window %v unknown", d)
	}
	// Resolve LiveWindow if op carries DesiredWindow target.
	resolveDesired := func(d w.DesiredWindowID) (w.LiveWindowID, error) {
		_, dw, err := findDesiredWindow(d)
		if err != nil {
			return "", err
		}
		res := identity.Resolve(*dw, observed)
		if res.Class != identity.ClassUniqueStrong {
			return "", fmt.Errorf("executor: identity for %v classified %s, refusing mutation", d, res.Class)
		}
		return res.Live, nil
	}
	resolveViewerDesired := func(d w.DesiredWindowID, ws w.WorkspaceID) (w.LiveWindowID, error) {
		pr, ok := target.Projects[d.Project]
		if !ok {
			return "", fmt.Errorf("executor: desired project %q unknown", d.Project)
		}
		var title string
		for i := range pr.Windows {
			if pr.Windows[i].ID == d {
				title = viewerTitleForAI(pr.Windows[i].TitleContract.Expected)
				break
			}
		}
		if title == "" {
			return "", fmt.Errorf("executor: desired window %v unknown", d)
		}
		var matches []w.LiveWindowID
		for id, ow := range observed.Windows {
			if ow.Workspace == ws && ow.Kind == w.WindowViewer && ow.Title.Value == title {
				matches = append(matches, id)
			}
		}
		if len(matches) != 1 {
			return "", fmt.Errorf("executor: viewer identity for %v on %s matched %d windows", d, ws, len(matches))
		}
		return matches[0], nil
	}
	validateLiveUniqueStrong := func(id w.LiveWindowID, expectedDesired *w.DesiredWindowID) error {
		ow, ok := observed.Windows[id]
		if !ok {
			return fmt.Errorf("executor: PreUniqueStrong: live window %s missing", id)
		}
		if ow.Kind == w.WindowViewer {
			matches := 0
			for _, candidate := range observed.Windows {
				if candidate.Kind == w.WindowViewer && candidate.Workspace == ow.Workspace && candidate.Title.Value == ow.Title.Value {
					matches++
				}
			}
			if matches != 1 {
				return fmt.Errorf("executor: PreUniqueStrong: viewer live window %s matched %d candidates", id, matches)
			}
			return nil
		}
		desired := expectedDesired
		if ow.MatchedTo != nil {
			desired = ow.MatchedTo
		}
		if desired == nil {
			return fmt.Errorf("executor: PreUniqueStrong: live window %s has no desired identity evidence", id)
		}
		resolved, err := resolveDesired(*desired)
		if err != nil {
			return err
		}
		if resolved != id {
			return fmt.Errorf("executor: PreUniqueStrong: live window %s resolved desired identity to %s", id, resolved)
		}
		return nil
	}

	if (oper.Kind == op.KindCloseWindow || oper.Kind == op.KindKillSession) && oper.Target.LiveWindow != nil {
		if _, ok := observed.Windows[*oper.Target.LiveWindow]; !ok {
			return nil
		}
	}

	// Precondition checks.
	for _, pc := range oper.Preconditions {
		switch pc.Kind {
		case op.PreUniqueStrong:
			if pc.Target.LiveWindow != nil {
				if err := validateLiveUniqueStrong(*pc.Target.LiveWindow, pc.Target.DesiredWindow); err != nil {
					return err
				}
			} else if pc.Target.DesiredWindow != nil {
				if _, err := resolveDesired(*pc.Target.DesiredWindow); err != nil {
					return err
				}
			}
		case op.PreWorkspaceExists:
			if pc.Target.Workspace != nil {
				if _, ok := e.Env.WorkspaceByID(*pc.Target.Workspace); !ok {
					return fmt.Errorf("executor: PreWorkspaceExists: workspace %q not in environment", *pc.Target.Workspace)
				}
			}
		}
	}
	semantic := semop.Runner{Adapter: e.Adapter, Env: e.Env}

	switch oper.Kind {
	case op.KindSpawnTerminal:
		if oper.Target.DesiredWindow == nil || oper.Target.Workspace == nil {
			return fmt.Errorf("executor: spawn missing target")
		}
		_, err := semantic.SpawnProjectTerminal(ctx, *oper.Target.DesiredWindow, *oper.Target.Workspace, observed, target)
		if err != nil {
			return fmt.Errorf("executor: spawn-terminal: %w", err)
		}
		return nil

	case op.KindSpawnEditor:
		if oper.Target.DesiredWindow == nil || oper.Target.Workspace == nil {
			return fmt.Errorf("executor: spawn missing target")
		}
		_, err := semantic.SpawnProjectEditor(ctx, *oper.Target.DesiredWindow, *oper.Target.Workspace, observed, target)
		if err != nil {
			return fmt.Errorf("executor: spawn-editor: %w", err)
		}
		return nil

	case op.KindSpawnBrowser:
		if oper.Target.DesiredWindow == nil || oper.Target.Workspace == nil {
			return fmt.Errorf("executor: spawn missing target")
		}
		_, err := semantic.SpawnProjectBrowser(ctx, *oper.Target.DesiredWindow, *oper.Target.Workspace, observed, target)
		if err != nil {
			return fmt.Errorf("executor: spawn-browser: %w", err)
		}
		return nil

	case op.KindSpawnViewer:
		if oper.Target.DesiredWindow == nil || oper.Target.Workspace == nil {
			return fmt.Errorf("executor: spawn missing target")
		}
		d := *oper.Target.DesiredWindow
		pr, dw, err := findDesiredWindow(d)
		if err != nil {
			return fmt.Errorf("executor: spawn-viewer: %w", err)
		}
		title := dw.TitleContract.Expected
		bundle := dw.App.BundleID
		kind := w.WindowViewer
		title = viewerTitleForAI(title)
		matches := 0
		for _, ow := range observed.Windows {
			if ow.Kind == w.WindowViewer && ow.Title.Value == title {
				matches++
			}
		}
		if matches > 0 {
			return fmt.Errorf("executor: spawn-viewer: viewer %q already matched %d live windows", title, matches)
		}
		viewerSession := naming.ViewerTmuxSession(d.Index+1, string(d.Project))
		sourceSession := naming.TmuxSession(naming.KindAI, d.Index+1, string(d.Project))
		_, err = e.Adapter.Spawn(ctx, wm.SpawnRequest{
			Workspace:               *oper.Target.Workspace,
			Kind:                    kind,
			Desired:                 d,
			Title:                   title,
			BundleID:                bundle,
			AppPath:                 dw.App.AppPath,
			ProjectPath:             pr.Root,
			TmuxSession:             viewerSession,
			ViewerSourceTmuxSession: sourceSession,
		})
		return err

	case op.KindKillSession:
		if oper.Target.LiveWindow == nil {
			return fmt.Errorf("executor: kill-session missing live target")
		}
		if err := validateLiveUniqueStrong(*oper.Target.LiveWindow, oper.Target.DesiredWindow); err != nil {
			return err
		}
		ow, ok := observed.Windows[*oper.Target.LiveWindow]
		if !ok {
			return nil
		}
		var desiredID w.DesiredWindowID
		switch {
		case oper.Target.DesiredWindow != nil:
			desiredID = *oper.Target.DesiredWindow
		case ow.MatchedTo != nil:
			desiredID = *ow.MatchedTo
		default:
			return fmt.Errorf("executor: kill-session: live window %s has no desired identity evidence", *oper.Target.LiveWindow)
		}
		pr, dw, err := findDesiredWindow(desiredID)
		if err != nil {
			return fmt.Errorf("executor: kill-session: %w", err)
		}
		if target.IsProjectActive(desiredID.Project) {
			return fmt.Errorf("executor: kill-session: desired project %q is still active in target", desiredID.Project)
		}
		app, ok := e.Env.ManagedAppByBundle(dw.App.BundleID)
		if !ok || !app.LifecycleRemoval.Allowed {
			return fmt.Errorf("executor: kill-session: lifecycle removal is not authorized for bundle %q", dw.App.BundleID)
		}
		kind := dw.Kind
		title := dw.TitleContract.Expected
		if ow.Kind == w.WindowViewer {
			kind = w.WindowViewer
			title = viewerTitleForAI(title)
		}
		if !windowKindAllowed(app.LifecycleRemoval.AllowedKinds, kind) {
			return fmt.Errorf("executor: kill-session: lifecycle removal is not authorized for kind %q", kind)
		}
		switch app.LifecycleRemoval.Method {
		case w.LifecycleRemovalAXCloseGuarded:
			return e.Adapter.TerminateManagedAppInstance(ctx, wm.TerminateManagedAppInstanceRequest{
				LiveWindow: *oper.Target.LiveWindow,
				Desired:    desiredID,
				Kind:       kind,
				Title:      title,
				BundleID:   dw.App.BundleID,
			})
		case w.LifecycleRemovalBrowserWindowClose:
			return e.executeVivaldiBrowserWindowClose(ctx, *oper.Target.LiveWindow, desiredID, dw, app)
		case w.LifecycleRemovalProjectScopedApp:
			return e.executeZedProjectScopedRemoval(ctx, *oper.Target.LiveWindow, desiredID, pr, dw, app)
		default:
			return fmt.Errorf("executor: kill-session: lifecycle removal is not authorized for method %q (no live close authority wired)", app.LifecycleRemoval.Method)
		}

	case op.KindCloseWindow:
		if oper.Target.LiveWindow == nil {
			return fmt.Errorf("executor: close missing live target")
		}
		return e.Adapter.Close(ctx, *oper.Target.LiveWindow)

	case op.KindMoveWindowToWorkspace:
		if oper.Target.LiveWindow == nil || oper.Target.Workspace == nil {
			return fmt.Errorf("executor: move missing target")
		}
		return semantic.MoveResolvedWindowToWorkspace(ctx, *oper.Target.LiveWindow, *oper.Target.Workspace, oper.Target.DesiredWindow, observed, target)

	case op.KindReorderColumns:
		if oper.Target.Workspace == nil || len(oper.ExpectedEffects) == 0 {
			return fmt.Errorf("executor: reorder missing target")
		}
		// Translate desired columns to live columns via current observed (must all resolve).
		var desiredCols []w.DesiredColumn
		for _, ef := range oper.ExpectedEffects {
			if ef.Kind == op.EffectReorderColumns {
				desiredCols = ef.Columns
				break
			}
		}
		liveCols := [][]w.LiveWindowID{}
		for _, dc := range desiredCols {
			stack := []w.LiveWindowID{}
			for _, dwid := range dc.Windows {
				id, err := resolveDesired(dwid)
				ow, ok := observed.Windows[id]
				if err != nil || !ok || ow.Workspace != *oper.Target.Workspace {
					if ws, ok := e.Env.WorkspaceByID(*oper.Target.Workspace); ok && ws.Role == w.WorkspaceViewer {
						id, err = resolveViewerDesired(dwid, *oper.Target.Workspace)
						if err != nil {
							return fmt.Errorf("executor: reorder: %w", err)
						}
					} else {
						if err != nil {
							return fmt.Errorf("executor: reorder: %w", err)
						}
						return fmt.Errorf("executor: reorder: desired %v not currently observed on %s", dwid, *oper.Target.Workspace)
					}
				}
				stack = append(stack, id)
			}
			liveCols = append(liveCols, stack)
		}
		return e.Adapter.ReorderColumns(ctx, *oper.Target.Workspace, liveCols)

	case op.KindFocusWorkspace:
		if oper.Target.Workspace == nil {
			return fmt.Errorf("executor: focus-workspace missing target")
		}
		return e.Adapter.FocusWorkspace(ctx, *oper.Target.Workspace)

	case op.KindFocusWindow:
		if oper.Target.LiveWindow == nil {
			return fmt.Errorf("executor: focus-window missing target")
		}
		return e.Adapter.FocusWindow(ctx, *oper.Target.LiveWindow)

	case op.KindSpawnCockpit:
		// Cockpit spawn: planner-supplied SystemWindowID identifies the
		// desired display index. We resolve title + tmux session names
		// from the index and shell out to the adapter (unified design
		// v1 §6.1).
		if oper.Target.SystemWindow == nil {
			return fmt.Errorf("executor: spawn-cockpit missing SystemWindow target")
		}
		swID := *oper.Target.SystemWindow
		// Find the DesiredWorld SystemWindow to learn the canonical Title.
		var title string
		for _, sw := range target.SystemWindows {
			if sw.ID == swID {
				title = sw.Title
				break
			}
		}
		if title == "" {
			return fmt.Errorf("executor: spawn-cockpit: SystemWindow %+v not in DesiredWorld", swID)
		}
		return e.Adapter.SpawnCockpit(ctx, swID.Index, title)

	case op.KindShowCockpit:
		if oper.Target.SystemWindow == nil || oper.Target.Workspace == nil {
			return fmt.Errorf("executor: show-cockpit missing SystemWindow or Workspace target")
		}
		// Resolve display via park-workspace ownership (not alphabetical sort).
		// oper.Target.Workspace carries the ParkWorkspace set by the planner.
		// oper.Target.SystemWindow.Index is the DisplayIdx, used as fallback.
		parkWs := *oper.Target.Workspace
		displayID, err := resolveDisplayForParkWorkspace(parkWs, oper.Target.SystemWindow.Index, observed)
		if err != nil {
			return fmt.Errorf("executor: show-cockpit: %w", err)
		}
		// Park workspaces (CPn) are intentionally omniwm-only and live outside
		// projwm's managed-environment manifest (per requirements §2). Resolve
		// rawName from the manifest if present (allows test envs to define CPn
		// explicitly); otherwise fall back to the workspace ID itself, which
		// for CP1..CP6 doubles as the omniwmctl workspace name.
		rawName := string(parkWs)
		if spec, ok := e.Env.WorkspaceByID(parkWs); ok {
			rawName = spec.RawName
		}
		return e.Adapter.ShowCockpitOnDisplay(ctx, displayID, rawName)

	case op.KindHideCockpit:
		if oper.Target.SystemWindow == nil {
			return fmt.Errorf("executor: hide-cockpit missing SystemWindow target")
		}
		// Find the park workspace for this SystemWindow from DesiredWorld so we
		// can look up the owning display via WorkspaceToDisplay.
		// oper.Target.SystemWindow.Index is the DisplayIdx, used as fallback.
		swID := *oper.Target.SystemWindow
		parkWs := parkWorkspaceForSystemWindow(swID, target)
		displayID, err := resolveDisplayForParkWorkspace(parkWs, swID.Index, observed)
		if err != nil {
			return fmt.Errorf("executor: hide-cockpit: %w", err)
		}
		// Prior workspace is whatever was active before the show. When it's
		// empty, HideCockpitOnDisplay falls back to omniwm's per-display
		// `switch-workspace back-and-forth` history. Otherwise resolve the
		// rawName via manifest if possible, else use the ID directly.
		rawName := ""
		if oper.Target.Workspace != nil {
			priorWs := *oper.Target.Workspace
			rawName = string(priorWs)
			if spec, ok := e.Env.WorkspaceByID(priorWs); ok {
				rawName = spec.RawName
			}
		}
		if err := e.Adapter.HideCockpitOnDisplay(ctx, displayID, rawName); err != nil {
			return err
		}
		// SSOT §5.4 Proposal mode + §7.5 HideCockpitOnDisplay focus
		// restoration: when the planner captured a PriorWindow on the
		// hide op, focus it explicitly after the workspace switch. omniwm
		// per-workspace focus history covers most cases but explicit
		// FocusWindow handles edge cases where the prior window is
		// itself a freshly-spawned managed window not yet in omniwm's
		// MRU.
		if oper.Target.LiveWindow != nil {
			if err := e.Adapter.FocusWindow(ctx, *oper.Target.LiveWindow); err != nil {
				// Non-fatal: workspace already switched; user can refocus
				// manually. Verifier picks up the residual focus drift on
				// the next reconcile pass.
				return nil
			}
		}
		return nil

	case op.KindCloseCockpit:
		// Cockpit close bypasses the production close-window safety
		// matrix because the window holds no user data and is owned
		// by the planner (unified design v1 §6.5).
		if oper.Target.LiveWindow == nil {
			return fmt.Errorf("executor: close-cockpit missing live target")
		}
		return e.Adapter.Close(ctx, *oper.Target.LiveWindow)

	case op.KindMoveCockpitToParkWorkspace:
		// SSOT §3.4 INV-06: force-move the cockpit window back to its
		// ParkWorkspace. Target.LiveWindow is the cockpit ghostty id;
		// Target.Workspace is the ParkWorkspace.
		if oper.Target.LiveWindow == nil || oper.Target.Workspace == nil {
			return fmt.Errorf("executor: move-cockpit-to-park missing LiveWindow or Workspace target")
		}
		parkWs := *oper.Target.Workspace
		rawName := string(parkWs)
		if spec, ok := e.Env.WorkspaceByID(parkWs); ok {
			rawName = spec.RawName
		}
		return e.Adapter.MoveCockpitToParkWorkspace(ctx, *oper.Target.LiveWindow, rawName)
	}
	return fmt.Errorf("executor: unsupported op kind %q", oper.Kind)
}

func windowKindAllowed(allowed []w.WindowKind, kind w.WindowKind) bool {
	for _, candidate := range allowed {
		if candidate == kind {
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

func viewerTitleForAI(aiTitle string) string {
	if rest, ok := strings.CutPrefix(aiTitle, "ai-"); ok {
		return "ai-view-" + rest
	}
	return "ai-view-" + aiTitle
}

// resolveDisplayForParkWorkspace maps a park workspace (e.g. "CP2") to the
// physical DisplayID that owns it. Uses ObservedDisplayState.WorkspaceToDisplay
// (populated by sigwm from live window placement data) as the primary source,
// with a fallback to sorted-display-index via displayIdx for backward compatibility
// with the fake adapter (which does not populate WorkspaceToDisplay).
// This replaces the old alphabetical-sort-only approach which broke when display IDs
// were not contiguous (e.g. display:1, display:2, display:5).
func resolveDisplayForParkWorkspace(parkWs w.WorkspaceID, displayIdx int, observed w.ObservedWorld) (w.DisplayID, error) {
	if parkWs == "" {
		return "", fmt.Errorf("resolveDisplayForParkWorkspace: empty park workspace")
	}
	// Primary: window-placement map built by sigwm.Observe.
	if observed.Displays.WorkspaceToDisplay != nil {
		if id, ok := observed.Displays.WorkspaceToDisplay[parkWs]; ok {
			return id, nil
		}
	}
	// Fallback: sorted display index (preserves behavior for fake adapter in tests).
	ids := make([]w.DisplayID, 0, len(observed.Displays.Displays))
	for id := range observed.Displays.Displays {
		ids = append(ids, id)
	}
	// Sort: primary first, then lexicographic by ID.
	for i := 0; i < len(ids)-1; i++ {
		for j := i + 1; j < len(ids); j++ {
			iPrimary := observed.Displays.Primary != nil && *observed.Displays.Primary == ids[i]
			jPrimary := observed.Displays.Primary != nil && *observed.Displays.Primary == ids[j]
			swap := false
			if !iPrimary && jPrimary {
				swap = true
			} else if iPrimary == jPrimary && ids[i] > ids[j] {
				swap = true
			}
			if swap {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	if displayIdx >= 0 && displayIdx < len(ids) {
		return ids[displayIdx], nil
	}
	return "", fmt.Errorf("resolveDisplayForParkWorkspace: park workspace %q not found in observed displays (displayIdx=%d, displays=%d)", parkWs, displayIdx, len(ids))
}

// parkWorkspaceForSystemWindow returns the ParkWorkspace for the given
// SystemWindowID by looking it up in DesiredWorld.SystemWindows.
// Returns "" if not found (should not happen in production).
func parkWorkspaceForSystemWindow(swID w.SystemWindowID, target w.DesiredWorld) w.WorkspaceID {
	for _, sw := range target.SystemWindows {
		if sw.ID == swID {
			return sw.ParkWorkspace
		}
	}
	return ""
}

// executeVivaldiBrowserWindowClose drives the
// LifecycleRemovalBrowserWindowClose path. The flow mirrors the AX-close
// guarded branch but routes through the Vivaldi browser adapter rather than
// the WindowManagerAdapter, since the close must run in Vivaldi's profile
// context with private payload correlation evidence.
//
// Sequence:
//
//  1. Resolve the desired browser session and payload token from DesiredWorld.
//  2. Ask the VivaldiCloser to collect "before" observation (presence, bundle
//     correlation, profile isolation, payload correlation).
//  3. Build a partial VivaldiBrowserWindowCloseEvidence and validate it
//     against lifecyclecontract.ValidateVivaldiBrowserWindowClose with a
//     synthesized Disappearance value that the contract rejects unless the
//     observation indicates the target is present pre-close. This forces the
//     pre-close evidence to be strong enough on its own — the close mutation
//     itself is only issued after pre-evidence is contract-clean.
//  4. Issue the privileged close.
//  5. Collect "after" observation, build the final disappearance evidence,
//     and re-validate the full contract before reporting success.
func (e *Executor) executeVivaldiBrowserWindowClose(ctx context.Context, live w.LiveWindowID, desiredID w.DesiredWindowID, dw *w.DesiredWindow, app w.ManagedAppPolicy) error {
	if e.Vivaldi == nil {
		return fmt.Errorf("executor: kill-session: lifecycle removal is not authorized for browser-window-close until VivaldiCloser is wired")
	}
	if dw.Browser == nil || len(dw.Browser.URLPayloadRefs) == 0 {
		return fmt.Errorf("executor: kill-session: browser-window-close requires a desired browser session with private payload token")
	}
	payloadToken := string(dw.Browser.URLPayloadRefs[0])
	if payloadToken == "" {
		return fmt.Errorf("executor: kill-session: browser-window-close requires a non-empty private payload token")
	}
	profile := browser.VivaldiAutomationProfile

	beforeObs, err := e.Vivaldi.CollectCloseObservation(ctx, browser.CloseObservationParams{
		Profile:      profile,
		PayloadToken: payloadToken,
		LiveWindow:   live,
	})
	if err != nil {
		return fmt.Errorf("executor: kill-session: browser-window-close pre-observation: %w", err)
	}
	if !beforeObs.Present {
		return fmt.Errorf("executor: kill-session: browser-window-close: target live window %s is not present pre-close", live)
	}
	browserWindowID := beforeObs.CorrelatedBrowserID
	if browserWindowID == "" {
		browserWindowID = string(live)
	}
	pre := lifecyclecontract.VivaldiBrowserWindowCloseEvidence{
		Desired:              desiredID,
		Policy:               app,
		ObservedBundle:       beforeObs.ObservedBundle,
		Profile:              profile,
		PayloadToken:         payloadToken,
		ObservedPayloadToken: beforeObs.ObservedPayloadToken,
		BrowserWindowID:      browserWindowID,
		CorrelatedBrowserID:  beforeObs.CorrelatedBrowserID,
		LiveWindow:           live,
		CorrelatedLiveWindow: beforeObs.CorrelatedLiveWindow,
		TabPayloadCorrelated: beforeObs.TabPayloadCorrelated,
		UserProfileIsolated:  beforeObs.UserProfileIsolated,
		Disappearance: lifecyclecontract.ExactDisappearanceEvidence{
			TargetLiveWindow: live,
			BeforePresent:    true,
			AfterPresent:     false,
		},
	}
	// Run a "pre-close" contract validation with a synthesized clean
	// disappearance so any non-disappearance contract failure (policy,
	// profile isolation, payload correlation, bundle, identity) is detected
	// before we mutate. This means the real disappearance evidence is the
	// only thing the post-close re-validation can newly invalidate.
	if err := lifecyclecontract.ValidateVivaldiBrowserWindowClose(pre); err != nil {
		return fmt.Errorf("executor: kill-session: browser-window-close pre-evidence: %w", err)
	}

	if err := e.Vivaldi.CloseLiveWindow(ctx, live); err != nil {
		return fmt.Errorf("executor: kill-session: browser-window-close mutation: %w", err)
	}

	afterObs, err := e.Vivaldi.CollectCloseObservation(ctx, browser.CloseObservationParams{
		Profile:      profile,
		PayloadToken: payloadToken,
		LiveWindow:   live,
	})
	if err != nil {
		return fmt.Errorf("executor: kill-session: browser-window-close post-observation: %w", err)
	}
	final := pre
	final.Disappearance = lifecyclecontract.ExactDisappearanceEvidence{
		TargetLiveWindow:  live,
		BeforePresent:     true,
		AfterPresent:      afterObs.Present,
		MatchingRemaining: afterObs.MatchingRemaining,
	}
	if err := lifecyclecontract.ValidateVivaldiBrowserWindowClose(final); err != nil {
		return fmt.Errorf("executor: kill-session: browser-window-close post-evidence: %w", err)
	}
	return nil
}

// executeZedProjectScopedRemoval drives the LifecycleRemovalProjectScopedApp
// path for the Zed editor. The flow mirrors the Vivaldi browser-window-close
// branch but routes through the Zed adapter rather than the
// WindowManagerAdapter, since the close must run with project-scoped identity
// and unsaved-change evidence that omniwmctl does not surface.
//
// Sequence:
//
//  1. Resolve the desired project root from the DesiredProject (passed in via
//     the kill-session dispatcher).
//  2. Ask the ZedCloser to collect "before" observation (presence, project
//     root correlation, session/window evidence, AXDocumentModified probe).
//  3. Build a partial ZedProjectScopedRemovalEvidence and validate it against
//     lifecyclecontract.ValidateZedProjectScopedRemoval with a synthesized
//     Disappearance value that the contract accepts only when the observation
//     proves the target is present pre-close. This forces the pre-close
//     evidence to be strong enough on its own — the close mutation itself is
//     only issued after pre-evidence is contract-clean.
//  4. Issue the privileged close.
//  5. Collect "after" observation, build the final disappearance evidence,
//     and re-validate the full contract before reporting success.
func (e *Executor) executeZedProjectScopedRemoval(ctx context.Context, live w.LiveWindowID, desiredID w.DesiredWindowID, pr *w.DesiredProject, dw *w.DesiredWindow, app w.ManagedAppPolicy) error {
	if e.Zed == nil {
		return fmt.Errorf("executor: kill-session: lifecycle removal is not authorized for project-scoped-app until ZedCloser is wired")
	}
	if pr == nil || pr.Root == "" {
		return fmt.Errorf("executor: kill-session: project-scoped-app requires a desired project root")
	}
	projectRoot := pr.Root

	beforeObs, err := e.Zed.CollectCloseObservation(ctx, zed.CloseObservationParams{
		ProjectRoot: projectRoot,
		LiveWindow:  live,
	})
	if err != nil {
		return fmt.Errorf("executor: kill-session: project-scoped-app pre-observation: %w", err)
	}
	if !beforeObs.Present {
		return fmt.Errorf("executor: kill-session: project-scoped-app: target live window %s is not present pre-close", live)
	}
	pre := lifecyclecontract.ZedProjectScopedRemovalEvidence{
		Desired:            desiredID,
		Policy:             app,
		ObservedBundle:     beforeObs.ObservedBundle,
		ProjectRoot:        projectRoot,
		AdapterProjectRoot: beforeObs.AdapterProjectRoot,
		AdapterSessionID:   beforeObs.AdapterSessionID,
		AdapterWindowID:    beforeObs.AdapterWindowID,
		UnsavedChanges:     lifecyclecontract.UnsavedChangeState(beforeObs.UnsavedChanges),
		Disappearance: lifecyclecontract.ExactDisappearanceEvidence{
			TargetLiveWindow: live,
			BeforePresent:    true,
			AfterPresent:     false,
		},
	}
	// Run a "pre-close" contract validation with a synthesized clean
	// disappearance so any non-disappearance contract failure (policy,
	// project-root mismatch, unsaved changes, identity) is detected before we
	// mutate. The real disappearance evidence is the only thing the
	// post-close re-validation can newly invalidate.
	if err := lifecyclecontract.ValidateZedProjectScopedRemoval(pre); err != nil {
		return fmt.Errorf("executor: kill-session: project-scoped-app pre-evidence: %w", err)
	}

	if err := e.Zed.CloseLiveWindow(ctx, live); err != nil {
		return fmt.Errorf("executor: kill-session: project-scoped-app mutation: %w", err)
	}

	afterObs, err := e.Zed.CollectCloseObservation(ctx, zed.CloseObservationParams{
		ProjectRoot: projectRoot,
		LiveWindow:  live,
	})
	if err != nil {
		return fmt.Errorf("executor: kill-session: project-scoped-app post-observation: %w", err)
	}
	final := pre
	final.Disappearance = lifecyclecontract.ExactDisappearanceEvidence{
		TargetLiveWindow:  live,
		BeforePresent:     true,
		AfterPresent:      afterObs.Present,
		MatchingRemaining: afterObs.MatchingRemaining,
	}
	if err := lifecyclecontract.ValidateZedProjectScopedRemoval(final); err != nil {
		return fmt.Errorf("executor: kill-session: project-scoped-app post-evidence: %w", err)
	}
	return nil
}
