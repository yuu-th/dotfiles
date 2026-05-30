// Package semop owns semantic app/window operations between Executor and Adapter.
package semop

import (
	"context"
	"fmt"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/identity"
	"github.com/yuu-th/projwm-next/internal/naming"
	w "github.com/yuu-th/projwm-next/internal/world"
)

type Runner struct {
	Adapter wm.Adapter
	Env     w.ManagedEnvironment
	// Provenance is the controller's validated (desired identity → live window)
	// cache (SSOT §6.9.1). The pre-spawn existing-state check threads it so a
	// sibling editor's colliding same-title window is never mistaken for "this
	// identity already exists" (C1): the resolver excludes other-identity-owned
	// windows. Nil when provenance is off.
	Provenance map[w.DesiredWindowID]w.LiveWindowID
}

func (r Runner) SpawnProjectTerminal(ctx context.Context, desiredID w.DesiredWindowID, workspace w.WorkspaceID, observed w.ObservedWorld, target w.DesiredWorld) (w.LiveWindowID, error) {
	return r.spawnProjectWindow(ctx, desiredID, workspace, observed, target, w.WindowKind(""))
}

func (r Runner) SpawnProjectEditor(ctx context.Context, desiredID w.DesiredWindowID, workspace w.WorkspaceID, observed w.ObservedWorld, target w.DesiredWorld) (w.LiveWindowID, error) {
	return r.spawnProjectWindow(ctx, desiredID, workspace, observed, target, w.WindowEditor)
}

func (r Runner) SpawnProjectBrowser(ctx context.Context, desiredID w.DesiredWindowID, workspace w.WorkspaceID, observed w.ObservedWorld, target w.DesiredWorld) (w.LiveWindowID, error) {
	return r.spawnProjectWindow(ctx, desiredID, workspace, observed, target, w.WindowBrowser)
}

func (r Runner) MoveResolvedWindowToWorkspace(ctx context.Context, live w.LiveWindowID, workspace w.WorkspaceID, expectedDesired *w.DesiredWindowID, observed w.ObservedWorld, target w.DesiredWorld) error {
	if r.Adapter == nil {
		return fmt.Errorf("semop: adapter is required")
	}
	if err := r.requireWorkspace(workspace); err != nil {
		return err
	}
	ow, ok := observed.Windows[live]
	if !ok {
		return fmt.Errorf("semop.MoveResolvedWindowToWorkspace: live window %s missing", live)
	}
	desired := expectedDesired
	if ow.MatchedTo != nil {
		desired = ow.MatchedTo
	}
	if desired != nil {
		resolved, err := r.resolveDesired(*desired, observed, target)
		if err != nil {
			return fmt.Errorf("semop.MoveResolvedWindowToWorkspace: %w", err)
		}
		if resolved != live {
			return fmt.Errorf("semop.MoveResolvedWindowToWorkspace: live window %s resolved desired identity to %s", live, resolved)
		}
	} else if ow.Kind != w.WindowViewer {
		return fmt.Errorf("semop.MoveResolvedWindowToWorkspace: live window %s has no desired identity evidence", live)
	}
	if err := r.Adapter.MoveWindowToWorkspace(ctx, live, workspace); err != nil {
		return err
	}
	after, err := r.Adapter.Observe(ctx)
	if err != nil {
		return fmt.Errorf("semop.MoveResolvedWindowToWorkspace: verify observe: %w", err)
	}
	got, ok := after.Windows[live]
	if !ok {
		// Missing post-state is a verifier/replan concern; returning an executor
		// error here would bypass the controller's bounded replan contract.
		return nil
	}
	if got.Workspace != workspace {
		return fmt.Errorf("semop.MoveResolvedWindowToWorkspace: verify workspace=%s, want %s", got.Workspace, workspace)
	}
	return nil
}

func (r Runner) spawnProjectWindow(ctx context.Context, desiredID w.DesiredWindowID, workspace w.WorkspaceID, observed w.ObservedWorld, target w.DesiredWorld, expectedKind w.WindowKind) (w.LiveWindowID, error) {
	if r.Adapter == nil {
		return "", fmt.Errorf("semop: adapter is required")
	}
	if err := r.requireWorkspace(workspace); err != nil {
		return "", err
	}
	project, desired, err := findDesiredWindow(target, desiredID)
	if err != nil {
		return "", err
	}
	if expectedKind == "" {
		if desired.Kind != w.WindowShell && desired.Kind != w.WindowAI {
			return "", fmt.Errorf("semop.SpawnProjectWindow: desired kind=%s is not terminal-class", desired.Kind)
		}
	} else if desired.Kind != expectedKind {
		return "", fmt.Errorf("semop.SpawnProjectWindow: desired kind=%s, want %s", desired.Kind, expectedKind)
	}
	if live, ok, err := r.preSpawnExistingState(desired, workspace, observed); err != nil || ok {
		return live, err
	}
	tmuxSession, viewerSource, aiCommand := terminalSessionFields(desired)
	// Controller-owned title: for TitlePrefixOwned the ghostty adapter
	// is launched with --title=<value>; the live title later grows an
	// appendix from the shell PS1. TitleContract.Expected only carries
	// the *observed* full title (set post-spawn by the verifier), so
	// for the first spawn we fall back to TitleContract.Prefix, which
	// is the controller's source of truth. (Bug 2026-05-19: empty
	// Expected was being passed straight through, tripping the
	// "SpawnRequest.Title is required" guard in appcontract.go.)
	spawnTitle := desired.TitleContract.Expected
	if spawnTitle == "" && desired.TitleContract.Authority == w.TitlePrefixOwned {
		spawnTitle = desired.TitleContract.Prefix
	}
	// BundleID fallback by Kind. Pre-v2.9 reducer's defaultWindowForKind
	// did not populate App.BundleID, so projects created before the fix
	// land here with App.BundleID == "". The AppLauncher requires
	// either AppPath or BundleID; deriving from Kind avoids forcing a
	// full store migration for the (small) population of legacy rows.
	bundleID := desired.App.BundleID
	if bundleID == "" {
		switch desired.Kind {
		case w.WindowAI, w.WindowShell, w.WindowViewer:
			bundleID = "com.mitchellh.ghostty"
		case w.WindowEditor:
			bundleID = "dev.zed.Zed"
		case w.WindowBrowser:
			bundleID = "com.vivaldi.Vivaldi"
		}
	}
	live, err := r.Adapter.Spawn(ctx, wm.SpawnRequest{
		Workspace:               workspace,
		Kind:                    desired.Kind,
		Desired:                 desired.ID,
		Title:                   spawnTitle,
		BundleID:                bundleID,
		AppPath:                 desired.App.AppPath,
		ProjectPath:             project.Root,
		BrowserProfile:          browser.VivaldiAutomationProfile,
		BrowserPayloadToken:     browserPayloadToken(desired),
		TmuxSession:             tmuxSession,
		ViewerSourceTmuxSession: viewerSource,
		AICommand:               aiCommand,
	})
	if err != nil {
		return "", err
	}
	if err := r.verifySpawnedWindow(ctx, live, workspace, desired); err != nil {
		return "", err
	}
	return live, nil
}

// terminalSessionFields derives tmux session wiring from a DesiredWindow.
// Returns (tmuxSession, viewerSourceTmuxSession, aiCommand).
//
// SSOT §7.3: tmux session names use the 1-based stable
// DesiredWindowID.Index directly (no +1 displayID shift).
//
// SSOT §4.4 ai: per-AI launch command is routed from
// DesiredWindow.AI.Name via naming.AICommand. Default ("") is "claude"
// for backwards compatibility with projects created before AIName
// persistence; new projects always supply an explicit name.
//
// Viewer windows never auto-launch an AI command — they attach to the
// source AI session's tmux group and inherit its running process.
func terminalSessionFields(desired *w.DesiredWindow) (tmuxSession, viewerSourceTmuxSession, aiCommand string) {
	id := desired.ID.Index
	switch desired.Kind {
	case w.WindowAI:
		tmuxSession = naming.TmuxSession(naming.KindAI, id, string(desired.ID.Project))
		aiCommand = aiCommandFor(desired)
	case w.WindowShell:
		tmuxSession = naming.TmuxSession(naming.KindShell, id, string(desired.ID.Project))
	case w.WindowViewer:
		tmuxSession = naming.ViewerTmuxSession(id, string(desired.ID.Project))
		viewerSourceTmuxSession = naming.TmuxSession(naming.KindAI, id, string(desired.ID.Project))
	}
	return tmuxSession, viewerSourceTmuxSession, aiCommand
}

// aiCommandFor resolves the per-AI launch command from DesiredWindow.AI.
// Unknown names fall back to "claude" so a typo in AddWindow.AIName
// produces a working AI session rather than a black box.
func aiCommandFor(desired *w.DesiredWindow) string {
	if desired.AI != nil && desired.AI.Name != "" {
		if cmd := naming.AICommand(naming.AI(desired.AI.Name)); cmd != "" {
			return cmd
		}
	}
	return naming.AICommand(naming.AIClaude)
}

func browserPayloadToken(desired *w.DesiredWindow) string {
	if desired.Kind != w.WindowBrowser || desired.Browser == nil || len(desired.Browser.URLPayloadRefs) == 0 {
		return ""
	}
	return string(desired.Browser.URLPayloadRefs[0])
}

func (r Runner) preSpawnExistingState(desired *w.DesiredWindow, workspace w.WorkspaceID, observed w.ObservedWorld) (w.LiveWindowID, bool, error) {
	res := identity.ResolveWithOptions(*desired, observed, identity.ResolveOptions{ExpectedWorkspace: workspace, Provenance: r.Provenance})
	switch res.Class {
	case identity.ClassMissing:
		return "", false, nil
	case identity.ClassUniqueStrong:
		return res.Live, true, nil
	default:
		return "", false, fmt.Errorf("semop.SpawnProjectWindow: pre-spawn identity for %v classified %s", desired.ID, res.Class)
	}
}

func (r Runner) verifySpawnedWindow(ctx context.Context, live w.LiveWindowID, workspace w.WorkspaceID, desired *w.DesiredWindow) error {
	after, err := r.Adapter.Observe(ctx)
	if err != nil {
		return fmt.Errorf("semop.SpawnProjectWindow: verify observe: %w", err)
	}
	ow, ok := after.Windows[live]
	if !ok {
		// Missing post-state is a verifier/replan concern; returning an executor
		// error here would bypass the controller's bounded replan contract.
		return nil
	}
	if ow.Kind != desired.Kind {
		return fmt.Errorf("semop.SpawnProjectWindow: verify kind=%s, want %s", ow.Kind, desired.Kind)
	}
	if ow.Workspace != workspace {
		return fmt.Errorf("semop.SpawnProjectWindow: verify workspace=%s, want %s", ow.Workspace, workspace)
	}
	if desired.App.BundleID != "" && ow.App.BundleID != desired.App.BundleID {
		return fmt.Errorf("semop.SpawnProjectWindow: verify bundle=%s, want %s", ow.App.BundleID, desired.App.BundleID)
	}
	if desired.TitleContract.Authority == w.TitleControllerOwned && ow.Title.Value != desired.TitleContract.Expected {
		return fmt.Errorf("semop.SpawnProjectWindow: verify title=%q, want %q", ow.Title.Value, desired.TitleContract.Expected)
	}
	return nil
}

func (r Runner) resolveDesired(desiredID w.DesiredWindowID, observed w.ObservedWorld, target w.DesiredWorld) (w.LiveWindowID, error) {
	_, desired, err := findDesiredWindow(target, desiredID)
	if err != nil {
		return "", err
	}
	res := identity.Resolve(*desired, observed)
	if res.Class != identity.ClassUniqueStrong {
		return "", fmt.Errorf("semop: identity for %v classified %s", desiredID, res.Class)
	}
	return res.Live, nil
}

func (r Runner) requireWorkspace(workspace w.WorkspaceID) error {
	if _, ok := r.Env.WorkspaceByID(workspace); !ok {
		return fmt.Errorf("semop: workspace %q not in environment", workspace)
	}
	return nil
}

func findDesiredWindow(target w.DesiredWorld, desiredID w.DesiredWindowID) (*w.DesiredProject, *w.DesiredWindow, error) {
	project, ok := target.Projects[desiredID.Project]
	if !ok {
		return nil, nil, fmt.Errorf("semop: desired project %q unknown", desiredID.Project)
	}
	for i := range project.Windows {
		if project.Windows[i].ID == desiredID {
			return &project, &project.Windows[i], nil
		}
	}
	return nil, nil, fmt.Errorf("semop: desired window %v unknown", desiredID)
}
