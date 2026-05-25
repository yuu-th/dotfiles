// Package wm declares the WindowManagerAdapter contract. design.md §7.1.
package wm

import (
	"context"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// Capabilities. design.md §7.1.
type Capabilities struct {
	MaxVisibleColumns             int
	MaxWindowsPerColumn           int
	SupportsSummonRight           bool
	SupportsTabbedColumn          bool
	SupportsMoveToWorkspaceByName bool
}

// SpawnRequest is what Executor sends to spawn a managed window.
//
// Optional fields (ProjectPath, AppPath, ExtraArgs) are honored by the SigWM
// backend's app-contract helpers (see appcontract.go). BrowserPayloadToken is
// required for browser restore; blank fallback windows are not a valid restore.
type SpawnRequest struct {
	Workspace w.WorkspaceID
	Kind      w.WindowKind
	Desired   w.DesiredWindowID
	Title     string
	BundleID  string

	// ProjectPath is the cwd / project root passed to editor-kind apps (Zed).
	// Zero-value is invalid for editor restore.
	ProjectPath string
	// AppPath is the absolute .app path used by `open -na` on macOS. If empty
	// SigWM falls back to invoking the app by bundle id.
	AppPath string
	// ExtraArgs is appended after `--args` for `open -na` on macOS.
	ExtraArgs []string
	// BrowserProfile and BrowserPayloadToken are used by browser-kind semantic
	// operations. Payload token points at PrivatePayloadStore; raw URLs must not
	// be placed in SpawnRequest.
	BrowserProfile      string
	BrowserPayloadToken string

	// TmuxSession is the tmux session name that should back a Ghostty window.
	// Empty for non-Ghostty kinds. The SigWM adapter ensures the session
	// exists (or grouped-clones from ViewerSourceTmuxSession for viewers)
	// before launching Ghostty with `-e tmux new-session -A -s <name>`.
	TmuxSession string
	// ViewerSourceTmuxSession, when set together with TmuxSession on a
	// WindowViewer kind, names the source AI session that the viewer
	// session is grouped from.
	ViewerSourceTmuxSession string
	// AICommand is the AI runner keystroke (e.g. "claude" / "copilot")
	// sent into a freshly-created AI tmux session via tmux send-keys.
	// Only honored when TmuxSession is set, the session was newly created
	// by this Spawn call, and Kind is WindowAI.
	AICommand string
}

// TerminateManagedAppInstanceRequest identifies a managed lifecycle target.
//
// This is intentionally not the raw Close primitive. The executor may construct
// it only after DesiredWindow identity evidence resolves uniquely to LiveWindow.
type TerminateManagedAppInstanceRequest struct {
	LiveWindow w.LiveWindowID
	Desired    w.DesiredWindowID
	Kind       w.WindowKind
	Title      string
	BundleID   string
}

// Adapter is the only authoritative window-manager mutation surface. design.md §7.1.
type Adapter interface {
	Capabilities(ctx context.Context) (Capabilities, error)

	// Observation.
	Observe(ctx context.Context) (w.ObservedWorld, error)

	// Mutation surface. Wrappers (Executor) own preconditions and ordering.
	Spawn(ctx context.Context, r SpawnRequest) (w.LiveWindowID, error)
	TerminateManagedAppInstance(ctx context.Context, r TerminateManagedAppInstanceRequest) error
	Close(ctx context.Context, id w.LiveWindowID) error
	MoveWindowToWorkspace(ctx context.Context, id w.LiveWindowID, ws w.WorkspaceID) error
	ReorderColumns(ctx context.Context, ws w.WorkspaceID, columns [][]w.LiveWindowID) error
	FocusWorkspace(ctx context.Context, ws w.WorkspaceID) error
	FocusWindow(ctx context.Context, id w.LiveWindowID) error

	// Cockpit operations (unified design v2 — park-workspace model).
	// SpawnCockpit launches a per-display projwm-cockpit Ghostty
	// attached to a tmux grouped clone of the base session
	// `projwm-cockpit`. The window is permanently bound to its CPn park
	// workspace via omniwm app-rule assignToWorkspace. Idempotent:
	// returns nil if a window with the given title already exists.
	// Focus is restored to the pre-call focused window (NFR-15).
	SpawnCockpit(ctx context.Context, displayIdx int, title string) error
	// ShowCockpitOnDisplay switches the specified display's active
	// workspace to parkWsName, making the cockpit window on that
	// display visible. Saves and restores focus (NFR-15).
	ShowCockpitOnDisplay(ctx context.Context, displayID w.DisplayID, parkWsName string) error
	// HideCockpitOnDisplay switches the specified display's active
	// workspace back to priorWsName, hiding the cockpit. Saves and
	// restores focus (NFR-15).
	HideCockpitOnDisplay(ctx context.Context, displayID w.DisplayID, priorWsName string) error
}

// OmniwmSelfHealer is an optional capability for adapters that can probe
// and heal the omniwm side. Implements requirements v2.8 §8.9 self-heal
// contract. SigWM implements it; fake/simulator adapters do not.
//
// The contract is "best-effort, never panic, log via returned errors".
// All operations are idempotent so the controller can retry safely.
type OmniwmSelfHealer interface {
	// ProbeOmniwmHealth queries omniwmctl to assemble a health snapshot.
	// Safe even if omniwm is unreachable (returns Reachable=false).
	ProbeOmniwmHealth(ctx context.Context) OmniwmHealth
	// RedeployOmniwmRules pushes the Nix-defined app-rules into omniwm
	// runtime by invoking the omniwm-deploy binary. Lv1 — no side effects
	// beyond rule list refresh.
	RedeployOmniwmRules(ctx context.Context) error
	// RelaunchManagedApp quits and re-launches the named app via
	// AppKit + `open -na <path>`. Lv2 — visible app flicker.
	RelaunchManagedApp(ctx context.Context, appName, appPath string) error
	// RestartOmniwm hard-restarts the omniwm LaunchAgent via
	// `launchctl kickstart -k`. Lv3 — all app workspace assignments
	// re-apply (app-rule re-fires) but column order is not preserved.
	RestartOmniwm(ctx context.Context) error
}

// CockpitReaper is an optional capability for adapters that own the
// cockpit process lifecycle. SigWM implements it; the fake/simulator
// adapters do not. projwmd uses this to enforce requirements §8.1 / §8.8:
// at most one cockpit while projwmd runs, zero when it stops.
type CockpitReaper interface {
	// ReapStaleCockpit kills every cockpit binary, ghostty cockpit window,
	// and cockpit tmux session before startup reconcile. Defense against
	// orphans accumulated from earlier crashes or direct `projwm tui`
	// invocations. Safe to call when nothing exists.
	ReapStaleCockpit(ctx context.Context)
	// ShutdownCockpit performs the same reap as part of graceful daemon
	// shutdown so a follow-up projwmd start re-spawns from a clean state.
	ShutdownCockpit(ctx context.Context)
	// ReapDuplicateCockpits enforces requirements v2.8 §8.10 "exactly one
	// cockpit" by killing duplicate ghostty processes with the cockpit
	// title. Called at startup AND every 30s by the recovery ticker, so
	// duplicates that appear mid-run are force-collapsed within one tick.
	// Keeps the ghostty with descendant children (= the one backing the
	// tmux pane) and kills the others.
	ReapDuplicateCockpits(ctx context.Context)
}
