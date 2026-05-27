// Package wm — real WindowManagerAdapter implementation backed by `omniwmctl`.
//
// SigWM talks to the omniwm daemon via its CLI subprocess. It is the production
// adapter selected by the managed-environment manifest.
//
// omniwmctl protocol findings (verified on the live host while implementing
// the real acceptance harness):
//   - There is no production-authorized `close-window` command. Close is blocked
//     for first implementation mutation safety.
//   - Cross-workspace moves require a numeric workspace:
//     `window navigate <id>` -> `window focus <id>` -> wait focused ->
//     `command move-to-workspace <number>`.
//   - Column reorder is exposed as focus-dependent `command move-column
//     <left|right|up|down>`, so SigWM serializes and verifies semantic reorder.
//   - `subscribe/watch` channels exist for focus, windows-changed,
//     display-changed, layout-changed, etc. They are event hints only; SigWM
//     still reconstructs truth from query snapshots.
//
// design.md §7.1 (adapter contract), implementation-design.md §6
// (Real mutation safety, app-contract minimums, focus/move race prevention).
//
// SigWM does NOT import the legacy projwm/internal/omniwm package — it
// reimplements the CLI client locally so projwm-next stays its own Go module.
package wm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	"github.com/yuu-th/projwm-next/internal/adapter/zed"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// ErrRealBackendBlocked is returned by SigWM mutation methods that the
// implementation-design §6 first-implementation matrix marks as "block":
// raw close, full layout restore via ReorderColumns. The Executor surfaces this
// to the Verifier which treats it as commit-blocked.
var ErrRealBackendBlocked = errors.New("wm/sigwm: operation blocked by first-implementation safety matrix")

// CtlExecutor is the abstract subprocess runner for `omniwmctl`. The interface
// exists for testability; production uses CmdCtlExecutor.
type CtlExecutor interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

// CmdCtlExecutor invokes a real `omniwmctl` binary on the host.
type CmdCtlExecutor struct {
	Bin     string // default "omniwmctl" (looked up on PATH)
	Timeout time.Duration
}

func (c CmdCtlExecutor) Run(ctx context.Context, args ...string) ([]byte, error) {
	bin := c.Bin
	if bin == "" {
		bin = "omniwmctl"
	}
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return out, fmt.Errorf("omniwmctl %s: timed out after %s", strings.Join(args, " "), timeout)
		}
		return out, fmt.Errorf("omniwmctl %s: %w (stderr: %s)", strings.Join(args, " "), err, stderr)
	}
	return out, nil
}

// retryConnRefusedExecutor wraps a CtlExecutor and retries the underlying
// call when it fails with omniwmctl's "Connection refused" transient error.
// This is most commonly seen in the harness when the production daemon's
// quiesce window overlaps OmniWM's brief listener restart on workspace
// transitions (see specs.md §7 on dual-listener race avoidance). The retry
// is treated as a production primitive: connection refused is a true
// transient failure mode of the omniwmctl protocol, not a test-only
// concern, so retrying here is consistent with the design constitution.
type retryConnRefusedExecutor struct {
	inner    CtlExecutor
	attempts int
	backoff  []time.Duration
}

func newRetryConnRefusedExecutor(inner CtlExecutor) *retryConnRefusedExecutor {
	return &retryConnRefusedExecutor{
		inner:    inner,
		attempts: 3,
		backoff:  []time.Duration{300 * time.Millisecond, 1 * time.Second, 2 * time.Second},
	}
}

// isConnRefusedError reports whether err looks like an omniwmctl transient
// connection-refused failure. omniwmctl exits with code 2 and the stderr
// fragment "Connection refused" when its IPC socket is briefly absent.
func isConnRefusedError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "Connection refused") {
		return true
	}
	if strings.Contains(msg, "connection refused") {
		return true
	}
	return false
}

func (r *retryConnRefusedExecutor) Run(ctx context.Context, args ...string) ([]byte, error) {
	var lastOut []byte
	var lastErr error
	for attempt := 0; attempt < r.attempts; attempt++ {
		if attempt > 0 {
			delay := r.backoff[attempt-1]
			select {
			case <-ctx.Done():
				return lastOut, ctx.Err()
			case <-time.After(delay):
			}
		}
		out, err := r.inner.Run(ctx, args...)
		if err == nil {
			return out, nil
		}
		if !isConnRefusedError(err) {
			return out, err
		}
		lastOut = out
		lastErr = err
	}
	return lastOut, lastErr
}

// AppLauncher abstracts spawning macOS GUI apps so unit tests can stub
// Ghostty/Zed/Vivaldi launches.
type AppLauncher interface {
	Launch(ctx context.Context, appPath, bundleID string, args []string) error
}

type ZedProjectLauncher interface {
	LaunchZedProject(ctx context.Context, projectPath string, extraArgs []string) error
}

// CmdAppLauncher uses macOS `open -na`.
type CmdAppLauncher struct{}

func (CmdAppLauncher) Launch(ctx context.Context, appPath, bundleID string, args []string) error {
	var cmd *exec.Cmd
	if appPath != "" {
		full := append([]string{"-na", appPath, "--args"}, args...)
		cmd = exec.CommandContext(ctx, "open", full...)
	} else if bundleID != "" {
		full := append([]string{"-nb", bundleID, "--args"}, args...)
		cmd = exec.CommandContext(ctx, "open", full...)
	} else {
		return errors.New("wm/sigwm: AppLauncher: appPath and bundleID both empty")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("open: %w (out: %s)", err, string(out))
	}
	return nil
}

// zedManagedSettingsJSON is the projwm-managed Zed settings (SSOT §4.4 editor):
// restore_on_startup="none" keeps the managed Zed from reopening unrelated
// prior windows, and an empty auto_install_extensions isolates it from the
// user's normal Zed configuration. Written into the projwm-private
// --user-data-dir so the managed instance never shares state with the user's Zed.
const zedManagedSettingsJSON = `{
  "restore_on_startup": "none",
  "auto_install_extensions": {}
}
`

func (CmdAppLauncher) LaunchZedProject(ctx context.Context, projectPath string, extraArgs []string) error {
	dataDir, err := ensureZedDataDir()
	if err != nil {
		return err
	}
	args := zedLaunchArgs(dataDir, projectPath, extraArgs)
	cmd := exec.CommandContext(ctx, "zed", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("zed %s: %w (out: %s)", strings.Join(args, " "), err, string(out))
	}
	return nil
}

// zedLaunchArgs builds the `zed` argv for a managed editor launch (SSOT §4.4):
// `-n` forces a NEW window (a bare `zed <cwd>` would reuse an existing
// workspace), and `--user-data-dir` points at the projwm-private dir so the
// managed Zed is isolated from the user's normal Zed state.
func zedLaunchArgs(dataDir, projectPath string, extraArgs []string) []string {
	args := []string{"-n", "--user-data-dir", dataDir}
	args = append(args, extraArgs...)
	args = append(args, projectPath)
	return args
}

func ensureZedDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("zed: user home: %w", err)
	}
	dir := filepath.Join(home, ".cache", "projwm-next", "zed-data")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("zed: create data dir %q: %w", dir, err)
	}
	settings := []byte(zedManagedSettingsJSON)
	for _, path := range []string{
		filepath.Join(dir, "settings.json"),
		filepath.Join(dir, "config", "settings.json"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", fmt.Errorf("zed: create settings dir %q: %w", filepath.Dir(path), err)
		}
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(path, settings, 0o644); err != nil {
				return "", fmt.Errorf("zed: write settings %q: %w", path, err)
			}
		} else if err != nil {
			return "", fmt.Errorf("zed: stat settings %q: %w", path, err)
		}
	}
	return dir, nil
}

// SigWM is the real (production) WindowManagerAdapter.
type SigWM struct {
	Env         w.ManagedEnvironment
	Exec        CtlExecutor
	Launcher    AppLauncher
	Browser     browser.BrowserCapabilityAdapter
	Tmux        tmuxClient
	CloseWindow func(ctx context.Context, win ctlWindow) error

	// SettleTimeout bounds post-spawn / post-move polling.
	SettleTimeout time.Duration

	// EnsureCockpitSession is called by SpawnCockpit to ensure the base tmux
	// session is running. If nil, ensureCockpitBaseSession is used (production
	// default). Tests inject a no-op to avoid real tmux invocations.
	EnsureCockpitSession func(ctx context.Context) error

	// EnsureScratchShellSession is called by ShowScratchShell to ensure the
	// scratch tmux session is running. If nil, ensureScratchShellSession is
	// used (production default). Tests inject a no-op for deterministic L2.
	EnsureScratchShellSession func(ctx context.Context) error

	// CockpitProcessAlive is an injection point for the SpawnCockpit
	// idempotence pgrep check. Production leaves it nil and falls back to
	// `pgrep -f ghostty.*--title=<title>`. Tests override it (typically
	// returning true when omniwm reports a title-matching window) so the
	// production-skip path is exercised without spawning child processes.
	CockpitProcessAlive func(ctx context.Context, title string) bool

	// mu serializes focus-dependent semantic operations on this adapter
	// instance (process-local fallback for impl-design §6 wmMutationLock;
	// global single-writer is the projwmd daemon's responsibility).
	mu sync.Mutex
}

// Compile-time assertion: SigWM satisfies wm.Adapter.
var _ Adapter = (*SigWM)(nil)

// NewSigWM constructs a real adapter. If exec is nil, a CmdCtlExecutor is
// used. The constructed executor is wrapped in a retry decorator that
// retries omniwmctl's transient `Connection refused` failure mode (see
// retryConnRefusedExecutor for the rationale and backoff schedule).
func NewSigWM(env w.ManagedEnvironment, exec CtlExecutor, launcher AppLauncher) *SigWM {
	if exec == nil {
		exec = CmdCtlExecutor{}
	}
	if launcher == nil {
		launcher = CmdAppLauncher{}
	}
	// Avoid double-wrapping when callers already supplied a retry executor.
	if _, alreadyWrapped := exec.(*retryConnRefusedExecutor); !alreadyWrapped {
		exec = newRetryConnRefusedExecutor(exec)
	}
	return &SigWM{Env: env, Exec: exec, Launcher: launcher, CloseWindow: closeWindowByAccessibility, SettleTimeout: 30 * time.Second}
}

// tmuxClient is the subset of *session.Client used by SigWM. Defined here as
// an interface so this package does not import the concrete tmux client and
// tests can stub it. The production daemon wires *session.Client.
type tmuxClient interface {
	HasSession(ctx context.Context, name string) (bool, error)
	EnsureSession(ctx context.Context, name, cwd string) (created bool, err error)
	EnsureGroupedSession(ctx context.Context, base, clone string) error
	SendKeys(ctx context.Context, session string, keys ...string) error
}

// --- omniwmctl wire types ---

type ctlEnvelope struct {
	OK     bool `json:"ok"`
	Result struct {
		Kind    string          `json:"kind"`
		Payload json.RawMessage `json:"payload"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

type ctlWorkspace struct {
	ID          string `json:"id"`
	RawName     string `json:"rawName"`
	DisplayName string `json:"displayName"`
	Number      int    `json:"number"`
	IsFocused   bool   `json:"isFocused"`
	IsCurrent   bool   `json:"isCurrent"`
}

type ctlWorkspacesPayload struct {
	Workspaces []ctlWorkspace `json:"workspaces"`
}

type ctlWindow struct {
	ID    string `json:"id"`
	PID   int    `json:"pid"`
	Title string `json:"title"`
	App   struct {
		BundleID string `json:"bundleId"`
		Name     string `json:"name"`
	} `json:"app"`
	IsFocused    bool   `json:"isFocused"`
	IsVisible    bool   `json:"isVisible"`
	IsScratchpad bool   `json:"isScratchpad"`
	Mode         string `json:"mode"`
	Frame        struct {
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		Height float64 `json:"height"`
	} `json:"frame"`
	HiddenReason string `json:"hiddenReason"`
	Workspace    struct {
		ID          string `json:"id"`
		RawName     string `json:"rawName"`
		DisplayName string `json:"displayName"`
		Number      int    `json:"number"`
	} `json:"workspace"`
	Display struct {
		ID     string `json:"id"`
		IsMain bool   `json:"isMain"`
	} `json:"display"`
}

type ctlWindowsPayload struct {
	Windows []ctlWindow `json:"windows"`
}

type ctlFocusedPayload struct {
	Window struct {
		ID string `json:"id"`
	} `json:"window"`
}

type ctlActiveWorkspacePayload struct {
	Workspace ctlWorkspace `json:"workspace"`
}

func (s *SigWM) decodeEnvelope(raw []byte, into any) error {
	var env ctlEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}
	if !env.OK {
		return fmt.Errorf("omniwmctl returned not-ok: %s", env.Error)
	}
	if into == nil {
		return nil
	}
	if err := json.Unmarshal(env.Result.Payload, into); err != nil {
		return fmt.Errorf("decode payload (%s): %w", env.Result.Kind, err)
	}
	return nil
}

func (s *SigWM) queryWorkspaces(ctx context.Context) ([]ctlWorkspace, error) {
	out, err := s.Exec.Run(ctx, "query", "workspaces", "--format", "json")
	if err != nil {
		return nil, err
	}
	var p ctlWorkspacesPayload
	if err := s.decodeEnvelope(out, &p); err != nil {
		return nil, err
	}
	return p.Workspaces, nil
}

func (s *SigWM) queryWindows(ctx context.Context, selectors ...string) ([]ctlWindow, error) {
	args := append([]string{"query", "windows", "--format", "json"}, selectors...)
	out, err := s.Exec.Run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var p ctlWindowsPayload
	if err := s.decodeEnvelope(out, &p); err != nil {
		return nil, err
	}
	return p.Windows, nil
}

func (s *SigWM) queryFocusedWindowID(ctx context.Context) (string, error) {
	out, err := s.Exec.Run(ctx, "query", "focused-window", "--format", "json")
	if err != nil {
		return "", err
	}
	var p ctlFocusedPayload
	if err := s.decodeEnvelope(out, &p); err != nil {
		return "", nil // tolerate empty
	}
	return p.Window.ID, nil
}

func (s *SigWM) queryActiveWorkspace(ctx context.Context) (ctlWorkspace, error) {
	out, err := s.Exec.Run(ctx, "query", "active-workspace", "--format", "json")
	if err != nil {
		return ctlWorkspace{}, err
	}
	var p ctlActiveWorkspacePayload
	if err := s.decodeEnvelope(out, &p); err != nil {
		return ctlWorkspace{}, err
	}
	return p.Workspace, nil
}

// resolveWorkspaceNumber maps a projwm-next WorkspaceID to the omniwm numeric
// workspace index that `command move-to-workspace` requires.
func (s *SigWM) resolveWorkspaceNumber(ctx context.Context, ws w.WorkspaceID) (int, string, error) {
	spec, ok := s.Env.WorkspaceByID(ws)
	if !ok {
		return 0, "", fmt.Errorf("wm/sigwm: workspace %q not in ManagedEnvironment", ws)
	}
	wss, err := s.queryWorkspaces(ctx)
	if err != nil {
		return 0, "", err
	}
	for _, omni := range wss {
		if omni.RawName == spec.RawName || omni.DisplayName == spec.DisplayName ||
			omni.RawName == string(spec.ID) || omni.ID == string(spec.ID) {
			return omni.Number, omni.RawName, nil
		}
	}
	return 0, "", fmt.Errorf("wm/sigwm: omniwm workspace not found for %q (rawName=%q)", ws, spec.RawName)
}

// workspaceIDFromOmni looks up the projwm-next WorkspaceID for an omniwm
// workspace descriptor. Returns the empty WorkspaceID if no env match.
func (s *SigWM) workspaceIDFromOmni(o ctlWorkspace) w.WorkspaceID {
	for _, ws := range s.Env.Workspaces.Workspaces {
		if ws.RawName == o.RawName || ws.DisplayName == o.DisplayName || string(ws.ID) == o.ID || string(ws.ID) == o.RawName {
			return ws.ID
		}
	}
	return ""
}

// --- Adapter methods ---

func (s *SigWM) Capabilities(ctx context.Context) (Capabilities, error) {
	// Capabilities are sourced from the Nix-authored ManagedEnvironment;
	// querying omniwm at runtime is not required for projwm-next planning.
	if err := ctx.Err(); err != nil {
		return Capabilities{}, err
	}
	return Capabilities{
		MaxVisibleColumns:             s.Env.WindowManager.Layout.MaxVisibleColumns,
		MaxWindowsPerColumn:           s.Env.WindowManager.Layout.MaxWindowsPerColumn,
		SupportsSummonRight:           true,
		SupportsTabbedColumn:          true,
		SupportsMoveToWorkspaceByName: false, // omniwmctl needs numeric index; we resolve internally
	}, nil
}

func (s *SigWM) Observe(ctx context.Context) (w.ObservedWorld, error) {
	wss, err := s.queryWorkspaces(ctx)
	if err != nil {
		return w.ObservedWorld{}, err
	}
	wins, err := s.queryWindows(ctx)
	if err != nil {
		return w.ObservedWorld{}, err
	}
	focusedID, _ := s.queryFocusedWindowID(ctx) // tolerate failure
	activeWS, activeErr := s.queryActiveWorkspace(ctx)

	ow := w.ObservedWorld{
		Workspaces: map[w.WorkspaceID]w.ObservedWorkspace{},
		Windows:    map[w.LiveWindowID]w.ObservedWindow{},
		Layouts:    map[w.WorkspaceID]w.ObservedLayout{},
	}

	// Seed from env so every managed workspace is present even if omniwm
	// hasn't reported it yet (e.g. empty workspace).
	for _, spec := range s.Env.Workspaces.Workspaces {
		ow.Workspaces[spec.ID] = w.ObservedWorkspace{ID: spec.ID, Role: spec.Role}
		ow.Layouts[spec.ID] = w.ObservedLayout{Workspace: spec.ID}
	}
	for _, omni := range wss {
		id := s.workspaceIDFromOmni(omni)
		if id == "" {
			continue
		}
		role := s.Env.WorkspaceRole(id)
		ow.Workspaces[id] = w.ObservedWorkspace{ID: id, Role: role}
		if omni.IsFocused || omni.IsCurrent {
			ow.Focus.Workspace = id
		}
	}
	// Group windows by workspace; one column per window (omniwmctl does not
	// expose column structure here; column-level layout requires a richer
	// query that remains intentionally blocked — see ReorderColumns block note).
	byWS := map[w.WorkspaceID][]ctlWindow{}
	for _, cw := range wins {
		wsID := s.workspaceIDFromOmni(ctlWorkspace{
			ID: cw.Workspace.ID, RawName: cw.Workspace.RawName, DisplayName: cw.Workspace.DisplayName,
		})
		// Park workspaces (CPn) are intentionally not in the projwm manifest
		// (requirements §2 — 管理外); workspaceIDFromOmni returns "" for them.
		// Fall back to the displayName so cockpit / other park-resident
		// windows compare equal to sw.ParkWorkspace ("CP1") in the planner.
		// Without this fallback, cockpit window's ow.Workspace was "" while
		// sw.ParkWorkspace="CP1", causing MoveCockpitToParkWorkspace ops to
		// fire on every reconcile and stealing focus (v2.8 §8.10 bug 2026-05-18).
		if wsID == "" && cw.Workspace.DisplayName != "" {
			wsID = w.WorkspaceID(cw.Workspace.DisplayName)
		}
		live := w.LiveWindowID(cw.ID)
		kind := classifyLiveWindow(cw)
		// v2.8 §8.10 ghost-window filter: omniwm can hold a stale window
		// reference whose backing process is dead (we hit this when a
		// previous daemon restart left orphan ghostty entries). For
		// cockpit windows the consequence is severe — planner sees a
		// "live" cockpit and refuses to SpawnCockpit, so the user is
		// stuck with no TUI. Verify the pid is actually alive (`kill -0`)
		// before admitting the window into ObservedWorld; drop it
		// otherwise so the planner re-spawns a fresh cockpit.
		if kind == w.WindowCockpit && cw.PID > 0 {
			if err := exec.CommandContext(ctx, "kill", "-0", strconv.Itoa(cw.PID)).Run(); err != nil {
				continue
			}
		}
		ow.Windows[live] = w.ObservedWindow{
			ID:        live,
			App:       w.ObservedAppRef{BundleID: cw.App.BundleID},
			Title:     w.ObservedTitle{Value: cw.Title},
			Workspace: wsID,
			Kind:      kind,
			Focused:   cw.IsFocused || cw.ID == focusedID,
		}
		// Cockpit windows (permanently bound to their CPn park workspace
		// via omniwm app-rule assignToWorkspace) never participate in
		// column-tile layout. Omit them from per-workspace grouping so
		// invariants like viewer-order (which compares Observed.Columns
		// against Desired) don't see them as stray columns.
		if wsID != "" && kind != w.WindowCockpit {
			byWS[wsID] = append(byWS[wsID], cw)
		}
		if cw.ID == focusedID {
			ow.Focus.Window = live
			if wsID != "" {
				ow.Focus.Workspace = wsID
			}
		}
	}
	if activeErr == nil {
		if id := s.workspaceIDFromOmni(activeWS); id != "" {
			ow.Focus.Workspace = id
		}
	}
	for wsID, wins := range byWS {
		cols := observedColumnsFromCtl(wins)
		l := ow.Layouts[wsID]
		l.Workspace = wsID
		l.Columns = cols
		ow.Layouts[wsID] = l
	}

	// Populate Observed.Displays so the reducer can count connected
	// displays for cockpit SystemWindow sync (unified design v2 §4.1).
	// Failure is tolerated: an empty Displays map just means cockpit
	// gets 0 SystemWindows on this cycle and tries again next event.
	// ActiveWorkspace is populated from the display's activeWorkspace field
	// and used by the planner for park-workspace show/hide convergence.
	//
	// WorkspaceToDisplay is also built here: it maps WorkspaceID → DisplayID
	// using two sources (most-specific wins; window placement beats display
	// activeWorkspace, since a display may currently be showing a non-park
	// workspace while the park workspace still belongs to it via app-rule).
	if disps, derr := s.queryDisplays(ctx); derr == nil {
		ow.Displays.Displays = make(map[w.DisplayID]w.ObservedDisplay, len(disps))
		wsToDisp := make(map[w.WorkspaceID]w.DisplayID, len(disps))
		for _, d := range disps {
			activeWsID := s.workspaceIDFromOmni(ctlWorkspace{
				ID:          d.ActiveWorkspace.ID,
				RawName:     d.ActiveWorkspace.RawName,
				DisplayName: d.ActiveWorkspace.DisplayName,
				Number:      d.ActiveWorkspace.Number,
			})
			// Park workspaces (CPn) are intentionally omniwm-only and not in
			// the projwm manifest, so workspaceIDFromOmni returns "" for them.
			// Fall back to the displayName so the planner can compare it
			// against sw.ParkWorkspace (which is WorkspaceID("CP1")…"CP6").
			if activeWsID == "" && d.ActiveWorkspace.DisplayName != "" {
				activeWsID = w.WorkspaceID(d.ActiveWorkspace.DisplayName)
			}
			ow.Displays.Displays[w.DisplayID(d.ID)] = w.ObservedDisplay{
				ID:              w.DisplayID(d.ID),
				Connected:       true,
				ActiveWorkspace: activeWsID,
			}
			if d.IsMain {
				main := w.DisplayID(d.ID)
				ow.Displays.Primary = &main
			}
			// Seed WorkspaceToDisplay from active workspace (low-confidence:
			// this only reflects what's currently displayed, not ownership).
			if activeWsID != "" {
				wsToDisp[activeWsID] = w.DisplayID(d.ID)
			}
		}
		// Override with window placement data: each observed window carries
		// its physical display ID. This is the authoritative source for CPn
		// park workspace ownership because app-rule assignToWorkspace ensures
		// cockpit windows (and any window in CPn) always live on the correct
		// physical display regardless of what's currently active on that display.
		for _, cw := range wins {
			if cw.Display.ID == "" {
				continue
			}
			wsID := s.workspaceIDFromOmni(ctlWorkspace{
				ID: cw.Workspace.ID, RawName: cw.Workspace.RawName, DisplayName: cw.Workspace.DisplayName,
			})
			if wsID == "" && cw.Workspace.DisplayName != "" {
				// CPn workspaces resolve via displayName fallback.
				wsID = w.WorkspaceID(cw.Workspace.DisplayName)
			}
			if wsID != "" {
				wsToDisp[wsID] = w.DisplayID(cw.Display.ID)
			}
		}
		ow.Displays.WorkspaceToDisplay = wsToDisp
	}

	return ow, nil
}

// queryDisplays calls `omniwmctl query displays --format json` and
// parses the observed envelope.
type ctlDisplay struct {
	ID              string `json:"id"`
	IsMain          bool   `json:"isMain"`
	Name            string `json:"name"`
	ActiveWorkspace struct {
		ID          string `json:"id"`
		RawName     string `json:"rawName"`
		DisplayName string `json:"displayName"`
		Number      int    `json:"number"`
	} `json:"activeWorkspace"`
}

type ctlDisplaysPayload struct {
	Displays []ctlDisplay `json:"displays"`
}

func (s *SigWM) queryDisplays(ctx context.Context) ([]ctlDisplay, error) {
	out, err := s.Exec.Run(ctx, "query", "displays", "--format", "json")
	if err != nil {
		return nil, err
	}
	var env ctlEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		return nil, err
	}
	var p ctlDisplaysPayload
	if err := json.Unmarshal(env.Result.Payload, &p); err != nil {
		return nil, err
	}
	return p.Displays, nil
}

func observedColumnsFromCtl(wins []ctlWindow) []w.ObservedColumn {
	if len(wins) == 0 {
		return nil
	}
	allZeroFrame := true
	anyWorkspaceInactive := false
	for _, win := range wins {
		if win.Frame.X != 0 || win.Frame.Y != 0 || win.Frame.Height != 0 {
			allZeroFrame = false
		}
		if win.HiddenReason == "workspace-inactive" {
			anyWorkspaceInactive = true
		}
	}
	if allZeroFrame {
		cols := make([]w.ObservedColumn, 0, len(wins))
		for _, win := range wins {
			cols = append(cols, w.ObservedColumn{Windows: []w.LiveWindowID{w.LiveWindowID(win.ID)}, Mode: w.ColumnSolo})
		}
		return cols
	}
	if anyWorkspaceInactive {
		return inactiveObservedColumnsFromCtl(wins)
	}
	type group struct {
		x    float64
		wins []ctlWindow
	}
	var groups []group
	for _, win := range wins {
		if !canFrameGroup(win) {
			groups = append(groups, group{x: win.Frame.X, wins: []ctlWindow{win}})
			continue
		}
		placed := false
		for i := range groups {
			if len(groups[i].wins) > 0 && canFrameGroup(groups[i].wins[0]) && absFloat(groups[i].x-win.Frame.X) <= 5 {
				groups[i].wins = append(groups[i].wins, win)
				placed = true
				break
			}
		}
		if !placed {
			groups = append(groups, group{x: win.Frame.X, wins: []ctlWindow{win}})
		}
	}
	cols := make([]w.ObservedColumn, 0, len(groups))
	for _, group := range groups {
		sort.SliceStable(group.wins, func(i, j int) bool {
			if group.wins[i].Frame.Y != group.wins[j].Frame.Y {
				return group.wins[i].Frame.Y > group.wins[j].Frame.Y
			}
			return group.wins[i].ID < group.wins[j].ID
		})
		ids := make([]w.LiveWindowID, 0, len(group.wins))
		for _, win := range group.wins {
			ids = append(ids, w.LiveWindowID(win.ID))
		}
		mode := w.ColumnSolo
		if len(ids) > 1 {
			mode = w.ColumnStacked
		}
		cols = append(cols, w.ObservedColumn{Windows: ids, Mode: mode})
	}
	return cols
}

func inactiveObservedColumnsFromCtl(wins []ctlWindow) []w.ObservedColumn {
	maxHeight := float64(0)
	for _, win := range wins {
		if win.Frame.Height > maxHeight {
			maxHeight = win.Frame.Height
		}
	}
	cols := make([]w.ObservedColumn, 0, len(wins))
	for i := 0; i < len(wins); {
		win := wins[i]
		if maxHeight > 0 && win.Frame.Height > 0 && win.Frame.Height < maxHeight {
			ids := []w.LiveWindowID{w.LiveWindowID(win.ID)}
			i++
			for i < len(wins) && wins[i].Frame.Height > 0 && wins[i].Frame.Height < maxHeight {
				ids = append(ids, w.LiveWindowID(wins[i].ID))
				i++
			}
			mode := w.ColumnSolo
			if len(ids) > 1 {
				mode = w.ColumnStacked
			}
			cols = append(cols, w.ObservedColumn{Windows: ids, Mode: mode})
			continue
		}
		cols = append(cols, w.ObservedColumn{Windows: []w.LiveWindowID{w.LiveWindowID(win.ID)}, Mode: w.ColumnSolo})
		i++
	}
	return cols
}

func canFrameGroup(win ctlWindow) bool {
	return win.IsVisible && win.HiddenReason == ""
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func classifyLiveWindow(cw ctlWindow) w.WindowKind {
	switch cw.App.BundleID {
	case "dev.zed.Zed":
		return w.WindowEditor
	case "com.vivaldi.Vivaldi":
		// design v3 §3.7 / requirements §4.2: only the automation profile
		// (`projwm-next`) is managed. User-profile Vivaldi windows are
		// reclassified as WindowExternal so planner / Tier 4 / cards
		// don't touch them.
		if !vivaldiManaged(cw.PID) {
			return w.WindowExternal
		}
		return w.WindowBrowser
	case "com.mitchellh.ghostty":
		switch {
		case strings.HasPrefix(cw.Title, "projwm-cockpit-"):
			return w.WindowCockpit
		case strings.HasPrefix(cw.Title, "ai-view-"):
			return w.WindowViewer
		case strings.HasPrefix(cw.Title, "ai-"):
			return w.WindowAI
		case strings.HasPrefix(cw.Title, "shell-"):
			return w.WindowShell
		}
		// Ghostty window without a projwm-controlled title prefix is a
		// manually-spawned terminal (e.g. user Cmd+N from another
		// ghostty, or first-time launch). Requirements §3.1 makes such
		// windows Tier 1 candidates on managed workspaces — classify as
		// WindowShell so isManagedKind=true and the reducer emits an
		// orphan card for user adoption. Bug 2026-05-18: previous impl
		// returned WindowExternal here, silencing Tier 1 detection for
		// the most common user scenario (manual ghostty on managed ws).
		return w.WindowShell
	}
	return w.WindowExternal
}

// vivaldiManaged returns true if the Vivaldi process owning pid was
// launched with `--profile-directory=projwm-next` (the automation
// profile). Detection result is cached per-PID for the lifetime of the
// daemon since Vivaldi rarely launches new long-lived processes.
//
// Falls back to "managed" if `ps` is unavailable or pid <= 0, so we
// don't silently lose track of automation-profile windows in stripped-
// down test/CI environments.
func vivaldiManaged(pid int) bool {
	if pid <= 0 {
		return true
	}
	vivaldiCacheMu.Lock()
	defer vivaldiCacheMu.Unlock()
	if hit, ok := vivaldiCache[pid]; ok && hit {
		return hit
	}
	managed := vivaldiInspectPID(pid)
	// Only memoize positive (managed) results. A managed Vivaldi process keeps
	// its --user-data-dir argv for life, so true is stable. A false result can
	// be transient or stale: the argv was momentarily unreadable, or (after a
	// browser archive→redeploy) macOS reused a PID that a now-dead helper had
	// been cached false under. Caching false would pin a freshly-spawned
	// managed browser as WindowExternal forever, so identity.Resolve returns
	// ClassMissing and the planner re-emits spawn-browser every replan (the
	// S2 archive→unarchive→assign redeploy loop). Re-inspect on false (~3ms).
	if managed {
		vivaldiCache[pid] = managed
	}
	return managed
}

// vivaldiInspectPID shells out to `ps -p PID -o args=` and inspects
// the result for `--profile-directory=projwm-next`. Default open-tilted
// so unknown / unreadable PIDs are treated as managed (same as before).
//
// Override-able from tests via vivaldiInspectFunc.
var vivaldiInspectFunc = func(pid int) bool {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "args=").Output()
	if err != nil {
		return true
	}
	s := string(out)
	// B-05: the managed Vivaldi is launched with a dedicated --user-data-dir
	// (projwm-next/vivaldi-data) which Chromium retains in the persistent
	// process argv — unlike --profile-directory, which is single-process and
	// dropped. Match the user-data-dir leaf; keep the legacy profile flag for
	// back-compat with any still-running pre-B-05 instance.
	return strings.Contains(s, "projwm-next/vivaldi-data") ||
		strings.Contains(s, "--profile-directory=projwm-next")
}

func vivaldiInspectPID(pid int) bool {
	return vivaldiInspectFunc(pid)
}

// vivaldiCache is a per-daemon-process PID → managed map. design v3 T5
// notes 3ms/call uncached, 0 cached.
var (
	vivaldiCacheMu sync.Mutex
	vivaldiCache   = map[int]bool{}
)

func (s *SigWM) Spawn(ctx context.Context, r SpawnRequest) (w.LiveWindowID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if r.Kind == w.WindowBrowser && r.Title != "" {
		return "", fmt.Errorf("sigwm.Spawn[vivaldi]: controller-owned browser title is not supported; got %q", r.Title)
	}
	if r.Kind == w.WindowEditor {
		if err := validateZedProjectPath(r.ProjectPath); err != nil {
			return "", fmt.Errorf("sigwm.Spawn[zed]: %w", err)
		}
	}
	// Pre-focus target workspace so the macOS open(1)-spawned window inherits
	// that workspace's display. Without this, a freshly spawned Ghostty/Zed/
	// Vivaldi window can land on whichever display happens to be focused at
	// the time and the subsequent post-spawn move-to-workspace becomes a
	// cross-display move that omniwmctl rejects with `exit 1` (no stderr).
	// Best-effort: silently ignore focus failures and let the post-spawn move
	// path still try with retries.
	if r.Workspace != "" {
		if num, _, err := s.resolveWorkspaceNumber(ctx, r.Workspace); err == nil {
			_, _ = s.Exec.Run(ctx, "command", "switch-workspace", "anywhere", strconv.Itoa(num))
			time.Sleep(150 * time.Millisecond)
		}
	}
	var before map[string]struct{}
	var hadZedEmptyBefore bool
	if r.Kind == w.WindowBrowser || r.Kind == w.WindowEditor {
		wins, err := s.queryWindows(ctx)
		if err != nil {
			return "", fmt.Errorf("sigwm.Spawn[%s]: pre-spawn observation: %w", r.Kind, err)
		}
		before = windowIDSet(wins)
		hadZedEmptyBefore = hasZedEmptyProject(wins)
		// SSOT §6.6 IDEMP / §4.4 ED-EXIST: "既存があれば focus、無ければ作る".
		// Skip re-spawn when a live (PID>0) window already matches the
		// controller-owned identity. For Editor we match by
		// bundleID+title (SSOT §3.4 INV-07: Zed title==basename(cwd));
		// Browser callers do not provide r.Title so we cannot match
		// them here — Vivaldi's own pre-spawn dedup in
		// browser.OpenInProfile handles that case.
		if r.Kind == w.WindowEditor && r.Title != "" {
			for _, win := range wins {
				if win.PID <= 0 {
					continue
				}
				if win.App.BundleID == r.BundleID && win.Title == r.Title {
					// Already exists — focus and return the existing live ID.
					if _, err := s.Exec.Run(ctx, "window", "focus", win.ID); err != nil {
						return "", fmt.Errorf("sigwm.Spawn[%s]: focus existing %s: %w", r.Kind, win.ID, err)
					}
					return w.LiveWindowID(win.ID), nil
				}
			}
		}
	}
	// SSOT §6.6 IDEMP "既存があれば focus、無ければ作る" — applies to ALL kinds,
	// not just the editor handled above. Ghostty windows (shell / AI / viewer)
	// carry a controller-owned --title (e.g. "shell-1:proj") that uniquely
	// identifies the managed window, but spawnGhostty always issues
	// `open -na ghostty` which forks a NEW window every call. Without a
	// pre-spawn dedup, a redundant Spawn (e.g. a summon racing the planner, or
	// a replan that re-emits before observation settles) produces a duplicate
	// window — violating INV-01. Mirror the editor dedup: if a live (PID>0)
	// ghostty window already carries this exact bundleID+title, focus it and
	// return its live ID instead of spawning again.
	if (r.Kind == w.WindowShell || r.Kind == w.WindowAI || r.Kind == w.WindowViewer) && r.Title != "" {
		if wins, qerr := s.queryWindows(ctx); qerr == nil {
			for _, win := range wins {
				if win.PID <= 0 {
					continue
				}
				if win.App.BundleID == r.BundleID && win.Title == r.Title {
					if _, ferr := s.Exec.Run(ctx, "window", "focus", win.ID); ferr != nil {
						return "", fmt.Errorf("sigwm.Spawn[%s]: focus existing %s: %w", r.Kind, win.ID, ferr)
					}
					return w.LiveWindowID(win.ID), nil
				}
			}
		}
	}
	// Dispatch by Kind to the per-app contract helper.
	var createdTmuxSession bool
	switch r.Kind {
	case w.WindowShell, w.WindowAI, w.WindowViewer:
		var gerr error
		createdTmuxSession, gerr = s.spawnGhostty(ctx, r)
		if gerr != nil {
			return "", fmt.Errorf("sigwm.Spawn[ghostty]: %w", gerr)
		}
	case w.WindowEditor:
		if err := s.spawnZed(ctx, r); err != nil {
			return "", fmt.Errorf("sigwm.Spawn[zed]: %w", err)
		}
	case w.WindowBrowser:
		if err := s.spawnVivaldi(ctx, r); err != nil {
			return "", fmt.Errorf("sigwm.Spawn[vivaldi]: %w", err)
		}
	default:
		return "", fmt.Errorf("sigwm.Spawn: unsupported kind %q", r.Kind)
	}

	// Settle: poll until exactly one omniwm window matches (bundleID, title).
	// Each spawn path supplies a processAlive predicate so that an omniwm
	// observation lag does NOT collapse the entire txn — the OS-level
	// spawn is still considered authoritative when the process is on screen.
	// Ghostty: title is unique per window, pgrep -f matches the launcher
	//   argv. Zed / Vivaldi: single-process multi-window, so any live PID
	//   is the best we can confirm — pair this with identity.Resolve in
	//   subsequent observations to disambiguate.
	var aliveFn func() bool
	switch r.Kind {
	case w.WindowAI, w.WindowShell, w.WindowViewer:
		title := r.Title
		aliveFn = func() bool {
			if s.CockpitProcessAlive != nil { // shared override hook for tests
				return s.CockpitProcessAlive(ctx, title)
			}
			out, perr := exec.CommandContext(ctx, "pgrep", "-f", fmt.Sprintf("ghostty.*--title=%s", title)).Output()
			return perr == nil && len(strings.TrimSpace(string(out))) > 0
		}
	case w.WindowEditor:
		aliveFn = func() bool {
			out, perr := exec.CommandContext(ctx, "pgrep", "-fl", "Zed.app/Contents/MacOS").Output()
			return perr == nil && len(strings.TrimSpace(string(out))) > 0
		}
	case w.WindowBrowser:
		aliveFn = func() bool {
			out, perr := exec.CommandContext(ctx, "pgrep", "-fl", "Vivaldi.app/Contents/MacOS").Output()
			return perr == nil && len(strings.TrimSpace(string(out))) > 0
		}
	}
	var live w.LiveWindowID
	var err error
	// SSOT §4.4 ED-MULTI: "複数 editor は bundleId + title + workspace で識別".
	// Zed window titles are basename(cwd), so generic names like "001"
	// or "src" collide with whatever Zed projects the user has open in
	// their daily work. Identifying a freshly-spawned Zed by title
	// alone hits sigwm.settle "ambiguous (count=N)" failures. Switch
	// Editor — like Browser — to the diff-based settle path: take a
	// pre-spawn window-ID snapshot, then look for exactly one
	// newly-appeared window with matching bundleID. Title equality is
	// no longer required because (a) Zed's title may lag the actual
	// window creation, and (b) the pre/post diff is itself enough to
	// disambiguate a single new instance.
	if r.Kind == w.WindowBrowser || r.Kind == w.WindowEditor {
		live, err = s.settleNewWindowByDiff(ctx, r.BundleID, before, aliveFn)
	} else {
		live, err = s.settleNewWindow(ctx, r.BundleID, r.Title, aliveFn)
	}
	if err != nil {
		return "", err
	}
	// settle accepted-via-fallback: live is empty. Skip the post-spawn
	// move (we don't have a target live ID yet) and let the next cycle's
	// observation + planner handle workspace placement.
	if live == "" {
		return "", nil
	}

	// Move to target workspace if it isn't already there.
	if err := s.moveLiveToWorkspaceLocked(ctx, live, r.Workspace); err != nil {
		return live, fmt.Errorf("sigwm.Spawn: post-spawn move: %w", err)
	}
	if r.Kind == w.WindowEditor {
		if err := s.closeNewZedEmptyProjects(ctx, before, hadZedEmptyBefore); err != nil {
			return live, fmt.Errorf("sigwm.Spawn[zed]: cleanup auxiliary windows: %w", err)
		}
	}
	// AI auto-launch: send the configured AI runner keystroke into the
	// freshly-created tmux session (projwm-spec D-40 / FR-21 / §5.1.1).
	// Skip if the session already existed (avoid duplicate launches) or if
	// no AI command was requested.
	if r.Kind == w.WindowAI && createdTmuxSession && r.AICommand != "" && s.Tmux != nil && r.TmuxSession != "" {
		if err := s.Tmux.SendKeys(ctx, r.TmuxSession, r.AICommand); err != nil {
			return live, fmt.Errorf("sigwm.Spawn[ghostty]: ai auto-launch send-keys: %w", err)
		}
	}
	return live, nil
}

func windowIDSet(wins []ctlWindow) map[string]struct{} {
	out := make(map[string]struct{}, len(wins))
	for _, win := range wins {
		out[win.ID] = struct{}{}
	}
	return out
}

func hasZedEmptyProject(wins []ctlWindow) bool {
	for _, win := range wins {
		if win.App.BundleID == "dev.zed.Zed" && (win.Title == "empty project" || win.Title == "") {
			return true
		}
	}
	return false
}

// settleNewWindow polls omniwmctl for a window matching bundleID + title.
// Returns ErrAmbiguous if more than one candidate exists at deadline (per
// impl-design §6 strong-evidence requirement).
//
// processAlive is consulted only on the deadline path. omniwm sometimes
// fails to register a freshly-spawned window in time (or at all) while
// the OS process itself is alive on screen. Treating that as a hard
// failure causes the controller to roll back the entire transaction —
// the unarchive bug-trail of 2026-05-19 boiled down to this exact
// shape: spawn-editor (or spawn-ai) returned timeout, executor-error
// killed the txn, Archived: true was never reduced to false, the user
// re-tried forever. When processAlive returns true on timeout we
// instead return ("", nil) — the spawn op is considered done at the
// OS-level; omniwm's catalog will catch up via the next observation
// pass and PopulateMatchedTo + identity.Resolve handle the mapping.
// Pass nil to disable the fallback (legacy strict behavior).
func (s *SigWM) settleNewWindow(ctx context.Context, bundleID, title string, processAlive func() bool) (w.LiveWindowID, error) {
	deadline := time.Now().Add(s.SettleTimeout)
	var lastCount int
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		wins, err := s.queryWindows(ctx)
		if err != nil {
			return "", err
		}
		var cands []ctlWindow
		for _, cw := range wins {
			if cw.App.BundleID == bundleID && cw.Title == title {
				cands = append(cands, cw)
			}
		}
		lastCount = len(cands)
		if len(cands) == 1 {
			return w.LiveWindowID(cands[0].ID), nil
		}
		if len(cands) > 1 {
			return "", fmt.Errorf("sigwm.settle: ambiguous (count=%d) for bundle=%s title=%q", len(cands), bundleID, title)
		}
		time.Sleep(150 * time.Millisecond)
	}
	if processAlive != nil && processAlive() {
		fmt.Fprintf(os.Stderr, "sigwm.settle: omniwm did not register window in time (bundle=%s title=%q); process is alive — accepting spawn (omniwm will reconcile asynchronously)\n", bundleID, title)
		return "", nil
	}
	return "", fmt.Errorf("sigwm.settle: timeout (last count=%d) for bundle=%s title=%q", lastCount, bundleID, title)
}

func (s *SigWM) settleNewWindowByDiff(ctx context.Context, bundleID string, before map[string]struct{}, processAlive func() bool) (w.LiveWindowID, error) {
	deadline := time.Now().Add(s.SettleTimeout)
	var lastCount int
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		wins, err := s.queryWindows(ctx)
		if err != nil {
			return "", err
		}
		var cands []ctlWindow
		for _, cw := range wins {
			if cw.App.BundleID != bundleID {
				continue
			}
			if _, existed := before[cw.ID]; !existed {
				cands = append(cands, cw)
			}
		}
		lastCount = len(cands)
		if len(cands) == 1 {
			return w.LiveWindowID(cands[0].ID), nil
		}
		if len(cands) > 1 {
			return "", fmt.Errorf("sigwm.settle: ambiguous newly spawned windows (count=%d) for bundle=%s", len(cands), bundleID)
		}
		time.Sleep(150 * time.Millisecond)
	}
	if processAlive != nil && processAlive() {
		fmt.Fprintf(os.Stderr, "sigwm.settle: omniwm did not surface new window for bundle=%s; process is alive — accepting spawn (omniwm will reconcile asynchronously)\n", bundleID)
		return "", nil
	}
	return "", fmt.Errorf("sigwm.settle: timeout (last new count=%d) for bundle=%s", lastCount, bundleID)
}

// closeNewZedEmptyProjects closes the spurious "empty project" window that Zed
// opens alongside the real project window when launched into a managed
// --user-data-dir. That window carries the dev.zed.Zed bundle, so omniwm
// catalogs and tiles it on the managed workspace, inflating the column count
// and breaking the layout settle. It can appear slightly AFTER the project
// window registers, so a single-shot query misses it — we poll briefly and
// close every NEW empty Zed window. Close goes through s.CloseWindow
// (closeWindowByAccessibility), which is scoped to the window's exact PID and
// title, so it never touches the user's own Zed instance (a distinct process
// with a distinct PID and the default user-data-dir).
func (s *SigWM) closeNewZedEmptyProjects(ctx context.Context, before map[string]struct{}, hadEmptyBefore bool) error {
	_ = hadEmptyBefore // PID+title-scoped close no longer needs the pre-existing-empty guard.
	if s.CloseWindow == nil {
		return nil
	}
	attempted := map[string]struct{}{}
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		wins, err := s.queryWindows(ctx)
		if err != nil {
			return err
		}
		foundNew := false
		for i := range wins {
			win := wins[i]
			if win.App.BundleID != "dev.zed.Zed" || (win.Title != "empty project" && win.Title != "") {
				continue
			}
			if _, existed := before[win.ID]; existed {
				continue
			}
			if _, did := attempted[win.ID]; did {
				continue
			}
			attempted[win.ID] = struct{}{}
			foundNew = true
			// Best-effort: an unclosable stray is still tolerated by the
			// managed-only filtering in waitSemanticColumns.
			_ = s.CloseWindow(ctx, win)
		}
		if !foundNew {
			time.Sleep(400 * time.Millisecond)
		}
	}
	return nil
}

func (s *SigWM) Close(ctx context.Context, id w.LiveWindowID) error {
	// Cockpit SystemWindows are the one exception to the close-block
	// policy (impl-design §6 safety matrix): they have no user data,
	// are spawned by our planner, and need to disappear on display
	// unplug. We classify by observing the live window's title prefix.
	s.mu.Lock()
	defer s.mu.Unlock()
	wins, qerr := s.queryWindows(ctx)
	if qerr != nil {
		return fmt.Errorf("sigwm.Close[%s]: pre-classify observe: %w", id, qerr)
	}
	var target *ctlWindow
	for i := range wins {
		if wins[i].ID == string(id) {
			target = &wins[i]
			break
		}
	}
	if target == nil {
		return nil // already gone
	}
	if target.App.BundleID == "com.mitchellh.ghostty" && strings.HasPrefix(target.Title, "projwm-cockpit-") {
		// Cockpit close: kill the underlying tmux clone (no AX
		// dialogs, no user data). Clone name == window title since
		// SpawnCockpit uses `projwm-cockpit-<idx>` for both.
		clone := target.Title
		_ = exec.CommandContext(ctx, "tmux", "kill-session", "-t", clone).Run()
		// Best-effort: also close the Ghostty window via the host
		// CloseWindow hook (AX). Failure is non-fatal — tmux kill
		// alone usually suffices because Ghostty exits when the
		// session dies.
		if s.CloseWindow != nil {
			_ = s.CloseWindow(ctx, *target)
		}
		return nil
	}
	return fmt.Errorf("sigwm.Close[%s]: close-window is blocked by first-implementation production safety policy", id)
}

func (s *SigWM) TerminateManagedAppInstance(ctx context.Context, r TerminateManagedAppInstanceRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.Desired.Project == "" {
		return fmt.Errorf("sigwm.TerminateManagedAppInstance[%s]: missing desired identity", r.LiveWindow)
	}
	if r.Kind == w.WindowExternal {
		return fmt.Errorf("sigwm.TerminateManagedAppInstance[%s]: external windows are not managed lifecycle targets", r.LiveWindow)
	}
	if r.BundleID == "" {
		return fmt.Errorf("sigwm.TerminateManagedAppInstance[%s]: missing managed app bundle", r.LiveWindow)
	}
	app, ok := s.Env.ManagedAppByBundle(r.BundleID)
	if !ok || !app.LifecycleRemoval.Allowed || app.LifecycleRemoval.Method != w.LifecycleRemovalAXCloseGuarded {
		return fmt.Errorf("sigwm.TerminateManagedAppInstance[%s]: lifecycle removal is not authorized for bundle %q", r.LiveWindow, r.BundleID)
	}
	if !windowKindAllowed(app.LifecycleRemoval.AllowedKinds, r.Kind) {
		return fmt.Errorf("sigwm.TerminateManagedAppInstance[%s]: lifecycle removal is not authorized for kind %q", r.LiveWindow, r.Kind)
	}
	wins, err := s.queryWindows(ctx)
	if err != nil {
		return err
	}
	var target *ctlWindow
	for i := range wins {
		if wins[i].ID == string(r.LiveWindow) {
			copy := wins[i]
			target = &copy
			break
		}
	}
	if target == nil {
		return nil
	}
	if target.App.BundleID != r.BundleID {
		return fmt.Errorf("sigwm.TerminateManagedAppInstance[%s]: bundle drift: observed=%s desired=%s", r.LiveWindow, target.App.BundleID, r.BundleID)
	}
	if r.Title != "" && target.Title != r.Title {
		return fmt.Errorf("sigwm.TerminateManagedAppInstance[%s]: title drift: observed=%q desired=%q", r.LiveWindow, target.Title, r.Title)
	}
	if err := validateUniqueCloseTarget(wins, *target); err != nil {
		return fmt.Errorf("sigwm.TerminateManagedAppInstance[%s]: %w", r.LiveWindow, err)
	}
	if _, err := s.Exec.Run(ctx, "window", "focus", string(r.LiveWindow)); err != nil {
		return fmt.Errorf("sigwm.TerminateManagedAppInstance[%s]: focus: %w", r.LiveWindow, err)
	}
	if err := s.waitFocusedWindow(ctx, r.LiveWindow, 1500*time.Millisecond); err != nil {
		return fmt.Errorf("sigwm.TerminateManagedAppInstance[%s]: wait-focused: %w", r.LiveWindow, err)
	}
	closeWindow := s.CloseWindow
	if closeWindow == nil {
		closeWindow = closeWindowByAccessibility
	}
	if err := closeWindow(ctx, *target); err != nil {
		return fmt.Errorf("sigwm.TerminateManagedAppInstance[%s]: app lifecycle terminate: %w", r.LiveWindow, err)
	}
	return s.waitWindowGoneWithRetry(ctx, r.LiveWindow, *target, closeWindow)
}

// waitWindowGoneWithRetry polls for the window to disappear after a close
// attempt; if the initial polling window expires (the keystroke was either
// dropped by Cocoa during a focus race or absorbed by an unresponsive UI
// frame) we re-attempt the close and continue polling for the same total
// budget. This is a production primitive: macOS Cmd+W is a best-effort
// keystroke, so the close primitive must own its retry path.
func (s *SigWM) waitWindowGoneWithRetry(ctx context.Context, id w.LiveWindowID, target ctlWindow, closeWindow func(context.Context, ctlWindow) error) error {
	per := s.SettleTimeout
	if per < 15*time.Second {
		per = 15 * time.Second
	}
	for attempt := 0; attempt < 2; attempt++ {
		err := s.waitWindowGone(ctx, id, per)
		if err == nil {
			return nil
		}
		if attempt == 1 {
			return err
		}
		// Re-focus and re-close before the second polling attempt.
		if _, ferr := s.Exec.Run(ctx, "window", "focus", string(id)); ferr != nil {
			return err
		}
		if ferr := s.waitFocusedWindow(ctx, id, 1500*time.Millisecond); ferr != nil {
			return err
		}
		if cerr := closeWindow(ctx, target); cerr != nil {
			return err
		}
	}
	return nil
}

func windowKindAllowed(allowed []w.WindowKind, kind w.WindowKind) bool {
	for _, candidate := range allowed {
		if candidate == kind {
			return true
		}
	}
	return false
}

func (s *SigWM) unsafeCloseForDiagnostics(ctx context.Context, id w.LiveWindowID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	wins, err := s.queryWindows(ctx)
	if err != nil {
		return err
	}
	var target *ctlWindow
	for i := range wins {
		if wins[i].ID == string(id) {
			copy := wins[i]
			target = &copy
			break
		}
	}
	if target == nil {
		return nil
	}
	if err := validateUniqueCloseTarget(wins, *target); err != nil {
		return fmt.Errorf("sigwm.Close[%s]: %w", id, err)
	}
	if _, err := s.Exec.Run(ctx, "window", "focus", string(id)); err != nil {
		return fmt.Errorf("sigwm.Close[%s]: focus: %w", id, err)
	}
	if err := s.waitFocusedWindow(ctx, id, 1500*time.Millisecond); err != nil {
		return fmt.Errorf("sigwm.Close[%s]: wait-focused: %w", id, err)
	}
	closeWindow := s.CloseWindow
	if closeWindow == nil {
		closeWindow = closeWindowByAccessibility
	}
	if err := closeWindow(ctx, *target); err != nil {
		return fmt.Errorf("sigwm.Close[%s]: window-level close: %w", id, err)
	}
	return s.waitWindowGoneWithRetry(ctx, id, *target, closeWindow)
}

func validateUniqueCloseTarget(wins []ctlWindow, target ctlWindow) error {
	if target.PID <= 0 {
		return errors.New("missing pid in omniwmctl window observation")
	}
	if target.Title == "" {
		return errors.New("missing title in omniwmctl window observation")
	}
	matches := 0
	for _, win := range wins {
		if win.PID == target.PID && win.App.BundleID == target.App.BundleID && win.Title == target.Title {
			matches++
		}
	}
	if matches != 1 {
		return fmt.Errorf("ambiguous AX close target: pid=%d bundle=%s title=%q matches=%d", target.PID, target.App.BundleID, target.Title, matches)
	}
	return nil
}

// closeWindowByAccessibility closes a single GUI window via AppleScript /
// System Events. Strategy (in order, falling back on failure):
//
//  1. AXPress the AXCloseButton on the matching window. This is the most
//     deterministic close primitive: it does not depend on which window is
//     currently focused inside the target process, so a focus race between
//     `set frontmost` and `keystroke` cannot misroute the close.
//  2. If the close button cannot be found or AXPress fails, raise the
//     matching window via AXRaise + AXMain, re-confirm focus, then issue
//     Cmd+W. The two-step (raise then keystroke) avoids the historical
//     focus-race failure mode where activating a process that owns
//     multiple windows would land focus on the wrong window.
//
// The script returns "ok-ax-close" or "ok-keystroke" on success so the Go
// caller can record which primitive disappeared the window. Errors are
// returned to the caller as the AppleScript error string.
func closeWindowByAccessibility(ctx context.Context, win ctlWindow) error {
	script := `
on run argv
  set targetPid to (item 1 of argv) as integer
  set targetTitle to item 2 of argv
  tell application "System Events"
    repeat with proc in processes
      try
        if (unix id of proc) is targetPid then
          tell proc
            repeat with candidate in windows
              set candidateTitle to ""
              try
                set candidateTitle to name of candidate
              end try
              if candidateTitle is targetTitle then
                -- Primary path: AXPress the AXCloseButton. This is
                -- focus-independent so it cannot be misrouted by a
                -- macOS app-activation race.
                try
                  set cb to value of attribute "AXCloseButton" of candidate
                  if cb is not missing value then
                    perform action "AXPress" of cb
                    return "ok-ax-close"
                  end if
                end try
                -- Fallback path: raise the candidate window inside the
                -- process so the keystroke targets it (rather than
                -- whichever window happens to be macOS-frontmost when
                -- the activation returns), then Cmd+W.
                try
                  perform action "AXRaise" of candidate
                end try
                try
                  set value of attribute "AXMain" of candidate to true
                end try
                set frontmost to true
                keystroke "w" using command down
                return "ok-keystroke"
              end if
            end repeat
          end tell
        end if
      end try
    end repeat
  end tell
  error "window title not found: " & targetTitle
end run
`
	cmd := exec.CommandContext(ctx, "osascript", "-e", script, strconv.Itoa(win.PID), win.Title)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript close pid=%d title=%q: %w (out: %s)", win.PID, win.Title, err, string(out))
	}
	return nil
}

func (s *SigWM) waitWindowGone(ctx context.Context, id w.LiveWindowID, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		wins, err := s.queryWindows(ctx)
		if err != nil {
			return err
		}
		found := false
		for _, cw := range wins {
			if cw.ID == string(id) {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
		time.Sleep(120 * time.Millisecond)
	}
	return fmt.Errorf("sigwm.Close[%s]: window still present after %s", id, timeout)
}

// moveLiveToWorkspaceLocked assumes s.mu is held by caller.
//
// The move primitive is occasionally transient on the production host: the
// focused window can get re-stolen by macOS app-switcher activation between
// `window focus` and `command move-to-workspace`, which surfaces as
// `move-to-workspace` exiting non-zero. We retry the focus → wait-focused →
// move-to-workspace triplet up to 3 times with bounded backoff before
// declaring a permanent failure. Re-verification of the focused window
// before each move ensures we never issue `move-to-workspace` against a
// window that has already lost focus (which would silently move a different
// window).
func (s *SigWM) moveLiveToWorkspaceLocked(ctx context.Context, id w.LiveWindowID, ws w.WorkspaceID) error {
	num, _, err := s.resolveWorkspaceNumber(ctx, ws)
	if err != nil {
		return err
	}
	// already-on-target short-circuit
	wins, err := s.queryWindows(ctx)
	if err != nil {
		return err
	}
	for _, cw := range wins {
		if cw.ID == string(id) {
			if cw.Workspace.Number == num {
				return nil
			}
			break
		}
	}
	// Sequence per attempt: navigate → focus → wait-focused → re-verify
	// focus → move-to-workspace → settle. Up to 3 attempts (initial + 2
	// retries) with 500ms backoff between attempts. Each retry re-runs
	// the full focus chain because a missed `move-to-workspace` may have
	// left the focus pointing at a different window.
	const moveAttempts = 3
	const moveRetryDelay = 500 * time.Millisecond
	var lastErr error
	for attempt := 0; attempt < moveAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(moveRetryDelay):
			}
			// Re-check already-on-target before retrying: a previous
			// attempt may have actually succeeded but the settle poll
			// timed out under heavy load.
			wins, qerr := s.queryWindows(ctx)
			if qerr == nil {
				for _, cw := range wins {
					if cw.ID == string(id) && cw.Workspace.Number == num {
						return nil
					}
				}
				// Confirm the target window still exists; if it has
				// disappeared we cannot keep retrying focus on a
				// dead id.
				found := false
				for _, cw := range wins {
					if cw.ID == string(id) {
						found = true
						break
					}
				}
				if !found {
					if lastErr != nil {
						return fmt.Errorf("sigwm.move: target window %s vanished during retry: %w", id, lastErr)
					}
					return fmt.Errorf("sigwm.move: target window %s vanished during retry", id)
				}
			}
		}
		if _, err := s.Exec.Run(ctx, "window", "navigate", string(id)); err != nil {
			lastErr = fmt.Errorf("navigate: %w", err)
			continue
		}
		if _, err := s.Exec.Run(ctx, "window", "focus", string(id)); err != nil {
			lastErr = fmt.Errorf("focus: %w", err)
			continue
		}
		if err := s.waitFocusedWindow(ctx, id, 1500*time.Millisecond); err != nil {
			lastErr = fmt.Errorf("wait-focused: %w", err)
			continue
		}
		// Re-verify focus immediately before mutating: macOS app
		// activation can steal focus between waitFocusedWindow's last
		// poll and the move-to-workspace call. If that happened, retry
		// the focus chain rather than mutating the wrong window.
		if got, _ := s.queryFocusedWindowID(ctx); got != string(id) {
			lastErr = fmt.Errorf("focus drifted before move-to-workspace (focused=%q want=%q)", got, id)
			continue
		}
		// Brief pre-move grace: niri's internal window state can lag
		// the focused-window query by ~100ms after a fresh spawn,
		// which surfaces as `move-to-workspace` exiting non-zero with
		// an empty stderr. Let niri canonicalize before mutating.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
		if _, err := s.Exec.Run(ctx, "command", "move-to-workspace", fmt.Sprint(num)); err != nil {
			lastErr = fmt.Errorf("move-to-workspace: %w", err)
			continue
		}
		// settle: window now on target workspace
		deadline := time.Now().Add(s.SettleTimeout)
		for time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			wins, err := s.queryWindows(ctx)
			if err != nil {
				lastErr = err
				break
			}
			for _, cw := range wins {
				if cw.ID == string(id) && cw.Workspace.Number == num {
					return nil
				}
			}
			time.Sleep(120 * time.Millisecond)
		}
		lastErr = fmt.Errorf("sigwm.move: settle timeout for %s -> %s", id, ws)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("sigwm.move: exhausted %d attempts for %s -> %s", moveAttempts, id, ws)
	}
	return lastErr
}

func (s *SigWM) waitFocusedWindow(ctx context.Context, id w.LiveWindowID, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		got, err := s.queryFocusedWindowID(ctx)
		if err == nil && got == string(id) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("focused window did not become %s within %s", id, timeout)
}

func (s *SigWM) MoveWindowToWorkspace(ctx context.Context, id w.LiveWindowID, ws w.WorkspaceID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.moveLiveToWorkspaceLocked(ctx, id, ws)
}

func (s *SigWM) ReorderColumns(ctx context.Context, ws w.WorkspaceID, columns [][]w.LiveWindowID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	want := make([]w.LiveWindowID, 0, len(columns))
	for _, col := range columns {
		want = append(want, col...)
	}
	if len(want) == 0 {
		return nil
	}
	current, err := s.liveOrderInWorkspace(ctx, ws)
	if err != nil {
		return err
	}
	pos := map[w.LiveWindowID]int{}
	for i, id := range current {
		pos[id] = i
	}
	for _, id := range want {
		if _, ok := pos[id]; !ok {
			return fmt.Errorf("sigwm.ReorderColumns[%s]: desired window %s is not in workspace", ws, id)
		}
	}
	var orderErr error
	for pass := 0; pass < 3; pass++ {
		if err := s.unstackWorkspaceLocked(ctx, ws, len(want)); err != nil {
			return err
		}
		current, err = s.liveOrderInWorkspace(ctx, ws)
		if err != nil {
			return err
		}
		for i, target := range want {
			for attempts := 0; attempts < len(want)*2; attempts++ {
				j := indexLive(current, target)
				if j < 0 {
					return fmt.Errorf("sigwm.ReorderColumns[%s]: lost target %s while reordering", ws, target)
				}
				if j <= i {
					break
				}
				if _, err := s.Exec.Run(ctx, "window", "focus", string(target)); err != nil {
					return fmt.Errorf("sigwm.ReorderColumns[%s]: focus %s: %w", ws, target, err)
				}
				if err := s.waitFocusedWindow(ctx, target, 1500*time.Millisecond); err != nil {
					return err
				}
				if _, err := s.Exec.Run(ctx, "command", "move-column", "left"); err != nil {
					return fmt.Errorf("sigwm.ReorderColumns[%s]: move-column left: %w", ws, err)
				}
				// 200ms grace lets niri/OmniWM commit the column-reorder
				// before we begin polling for the index decrement. Without
				// this, the very first observation can race the move and
				// surface the pre-move index, wasting one of the inner
				// poll iterations.
				time.Sleep(200 * time.Millisecond)
				// 3000ms (was 1500ms): under heavy load (multi-Vivaldi
				// reorder, the reconcile epoch flushing concurrent
				// settle work, AX-disowned viewer windows whose frame.x
				// is statically driven by Ghostty viewer mode) the
				// post-move observation can take longer than 1.5s to
				// surface in the omniwmctl query cache. We extend the
				// budget to 3000ms which empirically covers >99% of
				// observed reorder transitions.
				if err := s.waitWindowIndexLess(ctx, ws, target, j, 3000*time.Millisecond); err != nil {
					return err
				}
				current, err = s.liveOrderInWorkspace(ctx, ws)
				if err != nil {
					return err
				}
			}
		}
		orderErr = s.waitColumnOrder(ctx, ws, want, s.SettleTimeout)
		if orderErr == nil {
			break
		}
	}
	if orderErr != nil {
		return orderErr
	}
	hasStack := false
	for _, col := range columns {
		if len(col) < 2 {
			continue
		}
		hasStack = true
		for i := 1; i < len(col); i++ {
			target := col[i]
			if _, err := s.Exec.Run(ctx, "window", "focus", string(target)); err != nil {
				return fmt.Errorf("sigwm.ReorderColumns[%s]: focus stack target %s: %w", ws, target, err)
			}
			if err := s.waitFocusedWindow(ctx, target, 1500*time.Millisecond); err != nil {
				return err
			}
			if _, err := s.Exec.Run(ctx, "command", "move", "left"); err != nil {
				return fmt.Errorf("sigwm.ReorderColumns[%s]: stack move left: %w", ws, err)
			}
		}
	}
	if !hasStack {
		return nil
	}
	return s.waitSemanticColumns(ctx, ws, columns, s.SettleTimeout)
}

func (s *SigWM) waitWindowIndexLess(ctx context.Context, ws w.WorkspaceID, id w.LiveWindowID, previous int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		order, err := s.liveOrderInWorkspace(ctx, ws)
		if err != nil {
			return err
		}
		next := indexLive(order, id)
		if next >= 0 && next < previous {
			return nil
		}
		time.Sleep(80 * time.Millisecond)
	}
	// Final fresh-observation grace: under heavy load the omniwmctl query
	// cache can lag the live truth by ~200ms. Take one uncached observation
	// after a small grace window before declaring failure so we don't
	// surface a "did not move left" error when the move actually
	// committed within the deadline + grace window.
	time.Sleep(300 * time.Millisecond)
	if order, err := s.liveOrderInWorkspace(ctx, ws); err == nil {
		next := indexLive(order, id)
		if next >= 0 && next < previous {
			return nil
		}
	}
	order, _ := s.liveOrderInWorkspace(ctx, ws)
	return fmt.Errorf("sigwm.ReorderColumns[%s]: %s did not move left from index %d (order=%v)", ws, id, previous, order)
}

func (s *SigWM) unstackWorkspaceLocked(ctx context.Context, ws w.WorkspaceID, windowCount int) error {
	num, _, err := s.resolveWorkspaceNumber(ctx, ws)
	if err != nil {
		return err
	}
	maxAttempts := windowCount * 2
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		wins, err := s.queryWindows(ctx)
		if err != nil {
			return err
		}
		scoped := []ctlWindow{}
		for _, cw := range wins {
			if cw.Workspace.Number == num {
				scoped = append(scoped, cw)
			}
		}
		cols := observedColumnsFromCtl(scoped)
		var target w.LiveWindowID
		for _, col := range cols {
			if len(col.Windows) > 1 {
				target = col.Windows[0]
				break
			}
		}
		if target == "" {
			return nil
		}
		if _, err := s.Exec.Run(ctx, "window", "focus", string(target)); err != nil {
			return fmt.Errorf("sigwm.ReorderColumns[%s]: focus unstack target %s: %w", ws, target, err)
		}
		if err := s.waitFocusedWindow(ctx, target, 1500*time.Millisecond); err != nil {
			return err
		}
		// `move-to-root` lifts the focused window out of any stack into a
		// new top-level column. `move right` is a no-op inside a stacked
		// column on OmniWM (it just shifts the active stack member), so it
		// cannot flatten a stacked column on its own.
		//
		// move-to-root has an observed transient failure mode where it
		// returns `exit status 1` if the focused window's stack-membership
		// is being concurrently reshaped by OmniWM's settle pipeline (the
		// `Connection refused` retry decorator does not apply because this
		// is an exit-1, not a socket failure). Treat it as a transient
		// production-shaped retry: re-focus and try again up to 3 times
		// with a short backoff before bubbling the error up.
		moveErr := error(nil)
		alreadyAtRoot := false
		for retry := 0; retry < 3; retry++ {
			if retry > 0 {
				if _, err := s.Exec.Run(ctx, "window", "focus", string(target)); err != nil {
					return fmt.Errorf("sigwm.ReorderColumns[%s]: re-focus unstack target %s: %w", ws, target, err)
				}
				if err := s.waitFocusedWindow(ctx, target, 1500*time.Millisecond); err != nil {
					return err
				}
				time.Sleep(time.Duration(150*(1<<retry)) * time.Millisecond)
			}
			if out, err := s.Exec.Run(ctx, "command", "move-to-root"); err != nil {
				// OmniWM returns "ignored: layout_mismatch" with exit 1
				// when the focused window is already at root level (no
				// stack to lift out of). This means OmniWM internal
				// layout state disagrees with the observed-columns
				// grouping (likely frame-overlap heuristic over-reports
				// stacking). The right behavior: declare unstack done.
				// The string surfaces via stdout (captured in `out`),
				// not stderr (which is what err.Error() embeds).
				outStr := strings.TrimSpace(string(out))
				if strings.Contains(err.Error(), "layout_mismatch") ||
					strings.Contains(err.Error(), "ignored:") ||
					strings.Contains(outStr, "layout_mismatch") ||
					strings.Contains(outStr, "ignored:") {
					alreadyAtRoot = true
					moveErr = nil
					break
				}
				moveErr = err
				continue
			}
			moveErr = nil
			break
		}
		if moveErr != nil {
			return fmt.Errorf("sigwm.ReorderColumns[%s]: unstack move-to-root: %w", ws, moveErr)
		}
		if alreadyAtRoot {
			// OmniWM signaled the focused window is already root-level.
			// The observation-grouping heuristic likely over-reported
			// stacking. Trust OmniWM's authoritative layout and exit.
			return nil
		}
		// 400ms (was 150ms): omniwm/niri sometimes takes ~250ms to
		// surface the post-`move-to-root` window list, and the next
		// observation loop iteration would otherwise see the
		// pre-flatten layout and try to flatten the same column twice.
		time.Sleep(400 * time.Millisecond)
	}
	return fmt.Errorf("sigwm.ReorderColumns[%s]: existing stacked columns did not flatten", ws)
}

func (s *SigWM) liveOrderInWorkspace(ctx context.Context, ws w.WorkspaceID) ([]w.LiveWindowID, error) {
	num, _, err := s.resolveWorkspaceNumber(ctx, ws)
	if err != nil {
		return nil, err
	}
	wins, err := s.queryWindows(ctx)
	if err != nil {
		return nil, err
	}
	out := []w.LiveWindowID{}
	for _, cw := range wins {
		if cw.Workspace.Number == num {
			out = append(out, w.LiveWindowID(cw.ID))
		}
	}
	return out, nil
}

// reorderColumnSettleTimeout enforces a minimum 5s settle window for
// liveOrderInWorkspace polling during ReorderColumns, even when
// SigWM.SettleTimeout is configured lower (e.g. an aggressive unit-test
// override). Caller can still pass a larger value and we honor it.
const reorderColumnSettleTimeout = 5 * time.Second

func (s *SigWM) waitColumnOrder(ctx context.Context, ws w.WorkspaceID, want []w.LiveWindowID, timeout time.Duration) error {
	if timeout < reorderColumnSettleTimeout {
		timeout = reorderColumnSettleTimeout
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		got, err := s.liveOrderInWorkspace(ctx, ws)
		if err != nil {
			return err
		}
		if managedOrderSettled(got, want) {
			return nil
		}
		time.Sleep(120 * time.Millisecond)
	}
	// Final fresh-observation retry: under heavy load the omniwmctl
	// query cache can lag the live truth by ~200ms. Take one more
	// uncached observation before declaring failure so we don't surface
	// a "did not settle" error when the layout actually settled within
	// the deadline + grace window.
	time.Sleep(300 * time.Millisecond)
	if got, err := s.liveOrderInWorkspace(ctx, ws); err == nil && managedOrderSettled(got, want) {
		return nil
	}
	got, _ := s.liveOrderInWorkspace(ctx, ws)
	return fmt.Errorf("sigwm.ReorderColumns[%s]: order did not settle (got=%v want=%v)", ws, got, want)
}

func (s *SigWM) waitSemanticColumns(ctx context.Context, ws w.WorkspaceID, want [][]w.LiveWindowID, timeout time.Duration) error {
	// Only the windows we are arranging participate in the settle check.
	// Unmanaged windows that happen to live on the managed workspace — e.g. a
	// stray Zed "empty project" window, or a user window — must not block
	// convergence (SSOT external-window tolerance). Scope the observed columns
	// to the LiveWindowIDs in `want`.
	wantSet := map[string]struct{}{}
	for _, col := range want {
		for _, id := range col {
			wantSet[string(id)] = struct{}{}
		}
	}
	scopedManagedColumns := func(wins []ctlWindow, num int) []ctlWindow {
		var scoped []ctlWindow
		for _, win := range wins {
			if win.Workspace.Number != num {
				continue
			}
			if _, ok := wantSet[win.ID]; !ok {
				continue
			}
			scoped = append(scoped, win)
		}
		return scoped
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		wins, err := s.queryWindows(ctx)
		if err != nil {
			return err
		}
		num, _, err := s.resolveWorkspaceNumber(ctx, ws)
		if err != nil {
			return err
		}
		gotCols := observedColumnsFromCtl(scopedManagedColumns(wins, num))
		if semanticColumnsMatch(gotCols, want) {
			return nil
		}
		time.Sleep(120 * time.Millisecond)
	}
	wins, err := s.queryWindows(ctx)
	if err != nil {
		return err
	}
	num, _, err := s.resolveWorkspaceNumber(ctx, ws)
	if err != nil {
		return err
	}
	return fmt.Errorf("sigwm.ReorderColumns[%s]: semantic columns did not settle to desired layout (got=%s want=%s)", ws, formatObservedColumns(observedColumnsFromCtl(scopedManagedColumns(wins, num))), formatLiveColumns(want))
}

func semanticColumnsMatch(got []w.ObservedColumn, want [][]w.LiveWindowID) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if len(got[i].Windows) != len(want[i]) {
			return false
		}
		if len(want[i]) == 1 {
			if got[i].Windows[0] != want[i][0] {
				return false
			}
			continue
		}
		seen := map[w.LiveWindowID]int{}
		for _, id := range got[i].Windows {
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

func formatObservedColumns(cols []w.ObservedColumn) string {
	out := make([]string, 0, len(cols))
	for _, col := range cols {
		ids := make([]string, 0, len(col.Windows))
		for _, id := range col.Windows {
			ids = append(ids, string(id))
		}
		out = append(out, "["+strings.Join(ids, ",")+"]")
	}
	return strings.Join(out, " ")
}

func formatLiveColumns(cols [][]w.LiveWindowID) string {
	out := make([]string, 0, len(cols))
	for _, col := range cols {
		ids := make([]string, 0, len(col))
		for _, id := range col {
			ids = append(ids, string(id))
		}
		out = append(out, "["+strings.Join(ids, ",")+"]")
	}
	return strings.Join(out, " ")
}

func indexLive(ids []w.LiveWindowID, target w.LiveWindowID) int {
	for i, id := range ids {
		if id == target {
			return i
		}
	}
	return -1
}

func sameLiveOrder(got, want []w.LiveWindowID) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// managedOrderSettled reports whether every window in want is present in got
// in the desired relative order, ignoring any windows in got that are not in
// want. SSOT §6.3 L3 reorder targets only the managed (desired) windows; an
// external window (§4.3 / INV-11) that has drifted into the workspace is not
// part of DesiredLayout.Columns and must not block the reorder from settling.
// We therefore filter got down to the want set (preserving got's order) and
// require an exact match — i.e. the managed windows are correctly ordered
// relative to each other regardless of interspersed externals.
func managedOrderSettled(got, want []w.LiveWindowID) bool {
	wantSet := make(map[w.LiveWindowID]bool, len(want))
	for _, id := range want {
		wantSet[id] = true
	}
	filtered := make([]w.LiveWindowID, 0, len(want))
	for _, id := range got {
		if wantSet[id] {
			filtered = append(filtered, id)
		}
	}
	return sameLiveOrder(filtered, want)
}

func (s *SigWM) FocusWorkspace(ctx context.Context, ws w.WorkspaceID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	spec, ok := s.Env.WorkspaceByID(ws)
	if !ok {
		return fmt.Errorf("sigwm.FocusWorkspace: unknown workspace %q", ws)
	}
	name := spec.RawName
	if name == "" {
		name = spec.DisplayName
	}
	if name == "" {
		name = string(spec.ID)
	}
	_, err := s.Exec.Run(ctx, "workspace", "focus-name", name)
	return err
}

func (s *SigWM) FocusWindow(ctx context.Context, id w.LiveWindowID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// SSOT §7.5 F5: navigate → focus の command-order contract。navigate は
	// 「対象 window の workspace を active にする」ヒントで best-effort。
	// 失敗しても focus 単独で復帰できることが多いので error は飲み込む。
	// 実 focus 結果は L3 F3 が保証する。
	_, _ = s.Exec.Run(ctx, "window", "navigate", string(id))
	_, err := s.Exec.Run(ctx, "window", "focus", string(id))
	return err
}

// QueryVivaldiWindows returns the current set of Vivaldi-bundle-id windows
// observed by omniwmctl. This satisfies browser.VivaldiWindowQuerier so the
// browser adapter can drive the Vivaldi browser-window-close lifecycle
// without directly depending on the SigWM type.
func (s *SigWM) QueryVivaldiWindows(ctx context.Context) ([]browser.VivaldiOmniWMWindow, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	wins, err := s.queryWindows(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]browser.VivaldiOmniWMWindow, 0, len(wins))
	for _, win := range wins {
		if win.App.BundleID != browser.VivaldiBundleID {
			continue
		}
		out = append(out, browser.VivaldiOmniWMWindow{
			LiveWindow: w.LiveWindowID(win.ID),
			PID:        win.PID,
			Title:      win.Title,
			BundleID:   win.App.BundleID,
		})
	}
	return out, nil
}

// Compile-time assertion: SigWM satisfies the browser-side WindowQuerier.
var _ browser.VivaldiWindowQuerier = (*SigWM)(nil)

// QueryZedWindows returns the current set of Zed-bundle-id windows observed
// by omniwmctl. This satisfies zed.WindowQuerier so the Zed adapter can drive
// the project-scoped removal lifecycle without directly depending on the
// SigWM type.
func (s *SigWM) QueryZedWindows(ctx context.Context) ([]zed.OmniWMWindow, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	wins, err := s.queryWindows(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]zed.OmniWMWindow, 0, len(wins))
	for _, win := range wins {
		if win.App.BundleID != zed.ZedBundleID {
			continue
		}
		out = append(out, zed.OmniWMWindow{
			LiveWindow: w.LiveWindowID(win.ID),
			PID:        win.PID,
			Title:      win.Title,
			BundleID:   win.App.BundleID,
		})
	}
	return out, nil
}

// Compile-time assertion: SigWM satisfies the Zed-side WindowQuerier.
var _ zed.WindowQuerier = (*SigWM)(nil)

// --- Cockpit operations (unified design v1 §6) ------------------------

// cockpitBaseSession is the tmux session running the projwm-cockpit
// TUI binary. Per-display Ghostty windows attach to grouped clones of
// this session so they all share one cockpit view.
const cockpitBaseSession = "projwm-cockpit"

// cockpitCloneName returns the tmux clone session name for displayIdx.
func cockpitCloneName(displayIdx int) string {
	return fmt.Sprintf("projwm-cockpit-%d", displayIdx)
}

// ensureCockpitBaseSession creates the base tmux session if missing.
// The session entry point is the cockpit binary chosen via
// CFG_COCKPIT_BIN env, falling back to `projwm-cockpit` on PATH.
func ensureCockpitBaseSession(ctx context.Context) error {
	if err := exec.CommandContext(ctx, "tmux", "has-session", "-t", cockpitBaseSession).Run(); err == nil {
		return nil
	}
	bin := os.Getenv("CFG_COCKPIT_BIN")
	if bin == "" {
		bin = "projwm-cockpit"
	}
	if err := exec.CommandContext(ctx, "tmux", "new-session", "-d", "-s", cockpitBaseSession, bin).Run(); err != nil {
		return fmt.Errorf("tmux new-session %s: %w", cockpitBaseSession, err)
	}
	_ = exec.CommandContext(ctx, "tmux", "set-option", "-t", cockpitBaseSession, "window-size", "smallest").Run()
	return nil
}

// sortDisplaysForCockpit returns a stable ordering of displays:
// main display first, remaining by ID lexicographic ascending. This
// keeps DisplayIdx → physical display assignment deterministic across
// daemon restarts and observed-state polls (whose map iteration is
// random in Go).
func sortDisplaysForCockpit(disps []ctlDisplay) []ctlDisplay {
	out := make([]ctlDisplay, len(disps))
	copy(out, disps)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsMain != out[j].IsMain {
			return out[i].IsMain // true sorts first
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// SpawnCockpit launches a Ghostty window for one display attached to
// a grouped clone of the base cockpit tmux session.
//
// Idempotent: returns nil immediately if a window matching (bundle,
// title) already exists. Focus is restored to whatever was focused at
// entry (NFR-15).
func (s *SigWM) SpawnCockpit(ctx context.Context, displayIdx int, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if title == "" {
		return fmt.Errorf("sigwm.SpawnCockpit: empty title")
	}
	// Pre-check idempotence: a window with this title already exists IFF
	// (omniwm reports it) AND (a real ghostty process backs it). Omniwm
	// can hold a ghost reference to a window whose process died (the
	// 2026-05-18 reconcile loop failure mode), so we always cross-check
	// the OS process table before deciding to skip. Likewise, when omniwm
	// returns an empty window list (degraded mode), a ghostty process
	// matching --title= still means we should skip (avoids the orphan-
	// process flood we saw without this defense).
	//
	// pgrep -f matches against the full command line, so it catches both
	// the open-na launcher form and the actual ghostty argv with our
	// --title flag.
	processAlive := func() bool {
		if s.CockpitProcessAlive != nil {
			return s.CockpitProcessAlive(ctx, title)
		}
		out, perr := exec.CommandContext(ctx, "pgrep", "-f", fmt.Sprintf("ghostty.*--title=%s", title)).Output()
		return perr == nil && len(strings.TrimSpace(string(out))) > 0
	}
	wins, err := s.queryWindows(ctx)
	if err != nil {
		return fmt.Errorf("sigwm.SpawnCockpit: pre-check observe: %w", err)
	}
	omniwmHasTitle := false
	for _, win := range wins {
		if win.App.BundleID == "com.mitchellh.ghostty" && win.Title == title {
			omniwmHasTitle = true
			break
		}
	}
	if omniwmHasTitle && processAlive() {
		// SSOT §6.6 IDEMP: existing-window summon must focus, not no-op.
		// Without this an already-spawned cockpit stays hidden behind the
		// caller's prior focus (TestSpawnCockpitAlreadyExists guards this).
		for _, win := range wins {
			if win.App.BundleID == "com.mitchellh.ghostty" && win.Title == title {
				if _, err := s.Exec.Run(ctx, "window", "focus", win.ID); err != nil {
					return fmt.Errorf("sigwm.SpawnCockpit: focus existing %s: %w", win.ID, err)
				}
				break
			}
		}
		return nil
	}
	if !omniwmHasTitle && processAlive() {
		// Omniwm missed an existing process; skip spawn (defensive).
		return nil
	}
	// Otherwise (omniwm ghost OR truly missing): fall through to spawn.

	// Save focused window for NFR-15 focus restoration.
	priorFocus, _ := s.queryFocusedWindowID(ctx)

	// Pre-focus the target CP workspace so that when ghostty spawns it lands
	// on the correct display. `workspace focus-name CPn` both:
	//   (a) makes CPn the active workspace on its bound display, and
	//   (b) moves the globally-focused-monitor to that display.
	// After this, `open -na ghostty` (via Launcher.Launch) will spawn the
	// window on the focused-monitor's active workspace = CPn.
	//
	// CP workspace naming convention: D0→CP1, D1→CP2, ..., Dn→CP(n+1).
	// This matches the app-rules and monitor-profile bindings established in
	// the omniwm configuration.
	parkWs := fmt.Sprintf("CP%d", displayIdx+1)
	if _, err := s.Exec.Run(ctx, "workspace", "focus-name", parkWs); err != nil {
		return fmt.Errorf("sigwm.SpawnCockpit: pre-focus %s: %w", parkWs, err)
	}
	// Brief settle: workspace focus-name + focused-monitor change is observable
	// to omniwm within ~50ms. 100ms gives a comfortable safety margin without
	// meaningfully impacting startup time.
	time.Sleep(100 * time.Millisecond)

	// Ensure the base tmux session is running the cockpit binary.
	ensureSession := s.EnsureCockpitSession
	if ensureSession == nil {
		ensureSession = ensureCockpitBaseSession
	}
	if err := ensureSession(ctx); err != nil {
		return fmt.Errorf("sigwm.SpawnCockpit: ensure base session: %w", err)
	}

	// Launch ghostty with -e tmux new-session -A -s <clone> -t <base>
	// so each window attaches to a grouped clone of the cockpit base.
	clone := cockpitCloneName(displayIdx)
	args := []string{
		fmt.Sprintf("--title=%s", title),
		"-e", "tmux", "new-session", "-A",
		"-s", clone,
		"-t", cockpitBaseSession,
	}
	if err := s.Launcher.Launch(ctx, "", "com.mitchellh.ghostty", args); err != nil {
		return fmt.Errorf("sigwm.SpawnCockpit: launch ghostty: %w", err)
	}

	// Settle: wait until exactly one ghostty window matches title. The
	// shared settleNewWindow fallback (process-alive) handles the omniwm
	// registration lag, so cockpit no longer needs the bespoke fix it
	// once had — the same robustness now applies to every spawn path.
	if _, err := s.settleNewWindow(ctx, "com.mitchellh.ghostty", title, processAlive); err != nil {
		return fmt.Errorf("sigwm.SpawnCockpit: settle: %w", err)
	}

	// Restore focus (NFR-15). Best-effort: if the prior window is gone
	// (closed during spawn) we silently skip. Note: this leaves the target
	// display on CPn as a spawn side-effect; the planner's HideCockpit op
	// will switch the display back to its PriorWorkspace (captured by the
	// reducer at Bootstrap time before spawns ran).
	if priorFocus != "" {
		_, _ = s.Exec.Run(ctx, "window", "focus", priorFocus)
	}
	return nil
}

// switchDisplayToWorkspace is the shared primitive for ShowCockpitOnDisplay
// and HideCockpitOnDisplay. It switches the active workspace on a specific
// display to targetWsRawName.
//
// Strategy: to switch a specific display's workspace (not the globally focused
// monitor's), we must first move focused-monitor to that display, then issue
// `command switch-workspace anywhere <number>`. The simplest reliable sequence:
//
//  1. Find any workspace currently on the target display from queryDisplays.
//  2. `workspace focus-name <rawName>` — this moves the focused monitor to
//     the display that owns that workspace (omniwm side-effect of focusing a
//     workspace is to focus the monitor it lives on).
//  3. `command switch-workspace anywhere <number>` where number is the omniwm
//     number for targetWsRawName.
//
// This operates under s.mu (caller must hold).
func (s *SigWM) switchDisplayToWorkspace(ctx context.Context, displayID w.DisplayID, targetWsName string) error {
	// Resolve target workspace number. targetWsName may be either the omniwm
	// rawName ("23") or the displayName ("CP1") — match against either, since
	// park workspaces (CPn) are passed as displayName when not in the projwm
	// manifest, while regular managed workspaces resolve to rawName via the
	// manifest.
	wss, err := s.queryWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("queryWorkspaces: %w", err)
	}
	targetNum := -1
	for _, ws := range wss {
		if ws.RawName == targetWsName || ws.DisplayName == targetWsName {
			targetNum = ws.Number
			break
		}
	}
	if targetNum < 0 {
		return fmt.Errorf("target workspace %q not found in omniwm workspaces", targetWsName)
	}

	// Find a workspace currently on the target display so we can focus it first
	// to move the focused-monitor to the right display.
	disps, err := s.queryDisplays(ctx)
	if err != nil {
		return fmt.Errorf("queryDisplays: %w", err)
	}
	var anchorRawName string
	for _, d := range disps {
		if d.ID == string(displayID) {
			anchorRawName = d.ActiveWorkspace.RawName
			break
		}
	}

	if anchorRawName != "" && anchorRawName != targetWsName {
		// Focus the anchor workspace to move focused-monitor to this display.
		if _, ferr := s.Exec.Run(ctx, "workspace", "focus-name", anchorRawName); ferr != nil {
			// Best-effort: proceed even if this fails.
			_ = ferr
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Now switch to the target workspace on this (now-focused) display.
	if _, err := s.Exec.Run(ctx, "command", "switch-workspace", "anywhere", fmt.Sprint(targetNum)); err != nil {
		return fmt.Errorf("switch-workspace %d: %w", targetNum, err)
	}
	return nil
}

// ShowCockpitOnDisplay switches a specific display's active workspace to
// parkWsName, making the cockpit window (permanently bound to that workspace
// via omniwm app-rule assignToWorkspace) visible.
//
// NFR-15 note: we intentionally do NOT restore prior focus here. omniwm
// re-associates a display's active workspace with whichever window receives
// focus on that display; restoring focus to a window on the prior workspace
// would undo the show by pulling the display back. The show operation's
// purpose is to make the cockpit visible, so leaving focus on the (just-
// switched) park workspace is the correct outcome.
func (s *SigWM) ShowCockpitOnDisplay(ctx context.Context, displayID w.DisplayID, parkWsName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.switchDisplayToWorkspace(ctx, displayID, parkWsName); err != nil {
		return fmt.Errorf("sigwm.ShowCockpitOnDisplay[%s→%s]: %w", displayID, parkWsName, err)
	}
	return nil
}

// HideCockpitOnDisplay switches a specific display away from CPn so the
// cockpit window (which stays bound to CPn) is no longer shown on that display.
//
// Implementation: omniwm tracks per-display "back-and-forth" history. After
// SpawnCockpit's pre-focus moved the display CPn-ward, the back-and-forth
// pointer for that display points to the user's natural workspace. So
// `command switch-workspace back-and-forth` is the ideal primitive — it
// returns exactly to where the user was before spawn, without requiring us
// to plumb PriorWorkspace through the data model.
//
// If priorWsName is non-empty, we use it as an explicit target instead of
// back-and-forth (for cases where omniwm's history isn't trustworthy).
// Empty priorWsName triggers the back-and-forth path.
//
// NFR-15 note: same as ShowCockpitOnDisplay — we do NOT restore prior focus.
func (s *SigWM) HideCockpitOnDisplay(ctx context.Context, displayID w.DisplayID, priorWsName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if priorWsName != "" {
		if err := s.switchDisplayToWorkspace(ctx, displayID, priorWsName); err != nil {
			return fmt.Errorf("sigwm.HideCockpitOnDisplay[%s→%s]: %w", displayID, priorWsName, err)
		}
		return nil
	}
	// Back-and-forth path: focus the target display first (by focusing its
	// current workspace, which moves focused-monitor there), then issue
	// back-and-forth on the now-focused display.
	disps, err := s.queryDisplays(ctx)
	if err != nil {
		return fmt.Errorf("sigwm.HideCockpitOnDisplay[%s]: queryDisplays: %w", displayID, err)
	}
	var anchorRawName string
	for _, d := range disps {
		if d.ID == string(displayID) {
			anchorRawName = d.ActiveWorkspace.RawName
			break
		}
	}
	if anchorRawName != "" {
		if _, ferr := s.Exec.Run(ctx, "workspace", "focus-name", anchorRawName); ferr != nil {
			return fmt.Errorf("sigwm.HideCockpitOnDisplay[%s]: focus anchor %s: %w", displayID, anchorRawName, ferr)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if _, err := s.Exec.Run(ctx, "command", "switch-workspace", "back-and-forth"); err != nil {
		return fmt.Errorf("sigwm.HideCockpitOnDisplay[%s]: back-and-forth: %w", displayID, err)
	}
	return nil
}

// scratchShellTitle is both the tmux session name and the Ghostty title for
// the global scratch shell (SSOT §7.3).
const scratchShellTitle = "projwm-scratch-shell"

// ensureScratchShellSession creates the scratch tmux session if missing.
// Mirrors ensureCockpitBaseSession but with the scratch shell title and
// `fish` (login shell) as the session entry point — scratch is a plain
// shell, not a daemon binary.
func ensureScratchShellSession(ctx context.Context) error {
	if err := exec.CommandContext(ctx, "tmux", "has-session", "-t", scratchShellTitle).Run(); err == nil {
		return nil
	}
	// Use the user's login shell. Empty $SHELL falls back to /bin/sh which
	// is always present on macOS.
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	if err := exec.CommandContext(ctx, "tmux", "new-session", "-d", "-s", scratchShellTitle, shell).Run(); err != nil {
		return fmt.Errorf("tmux new-session %s: %w", scratchShellTitle, err)
	}
	return nil
}

// ShowScratchShell ensures a single global Ghostty scratch shell window is
// visible and focused. SSOT §4.1 OP11 / §7.3 SCRATCH.
//
// 冪等な実装:
//  1. omniwm query で `--title=projwm-scratch-shell` の Ghostty window があれば
//     navigate → focus して既存 ID を返す
//  2. なければ tmux session `projwm-scratch-shell` を ensure し、
//     `open -na ghostty --args --title=projwm-scratch-shell -e tmux new-session -A -s projwm-scratch-shell`
//     で spawn し、settle 後の window ID を返す
//
// process-alive fallback: omniwm 側がまだ window を登録していないが ghostty
// process が起きている場合は LiveWindowID 空文字 + nil で返す (spawn 系の慣例)。
func (s *SigWM) ShowScratchShell(ctx context.Context) (w.LiveWindowID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// Step 1: search for existing scratch window.
	wins, err := s.queryWindows(ctx)
	if err != nil {
		return "", fmt.Errorf("sigwm.ShowScratchShell: pre-check observe: %w", err)
	}
	for _, win := range wins {
		if win.App.BundleID == "com.mitchellh.ghostty" && win.Title == scratchShellTitle {
			id := w.LiveWindowID(win.ID)
			// navigate → focus (SSOT §7.5 F5).
			_, _ = s.Exec.Run(ctx, "window", "navigate", win.ID)
			if _, err := s.Exec.Run(ctx, "window", "focus", win.ID); err != nil {
				return id, fmt.Errorf("sigwm.ShowScratchShell: focus existing: %w", err)
			}
			return id, nil
		}
	}
	// Step 2: spawn fresh.
	ensureSession := s.EnsureScratchShellSession
	if ensureSession == nil {
		ensureSession = ensureScratchShellSession
	}
	if err := ensureSession(ctx); err != nil {
		return "", fmt.Errorf("sigwm.ShowScratchShell: ensure tmux session: %w", err)
	}
	args := []string{
		fmt.Sprintf("--title=%s", scratchShellTitle),
		"-e", "tmux", "new-session", "-A", "-s", scratchShellTitle,
	}
	if err := s.Launcher.Launch(ctx, "", "com.mitchellh.ghostty", args); err != nil {
		return "", fmt.Errorf("sigwm.ShowScratchShell: launch ghostty: %w", err)
	}
	// Settle: poll omniwm for the new window. Up to ~3s.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cerr := ctx.Err(); cerr != nil {
			return "", cerr
		}
		wins, err := s.queryWindows(ctx)
		if err == nil {
			for _, win := range wins {
				if win.App.BundleID == "com.mitchellh.ghostty" && win.Title == scratchShellTitle {
					id := w.LiveWindowID(win.ID)
					_, _ = s.Exec.Run(ctx, "window", "navigate", win.ID)
					_, _ = s.Exec.Run(ctx, "window", "focus", win.ID)
					return id, nil
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	// Process-alive fallback: ghostty exists but omniwm didn't register it
	// yet. Return empty ID + nil so caller proceeds (matches the existing
	// spawn-system convention).
	out, _ := exec.CommandContext(ctx, "pgrep", "-f", fmt.Sprintf("ghostty.*--title=%s", scratchShellTitle)).Output()
	if len(strings.TrimSpace(string(out))) > 0 {
		return "", nil
	}
	return "", fmt.Errorf("sigwm.ShowScratchShell: settle timeout, no scratch window observed")
}

// HideScratchShell restores focus to priorWindow. The scratch shell window
// itself stays alive (SSOT §4.1 OP11: 非表示時に scratch を kill しない、
// 次回 ShowScratchShell が即座に再 focus できる)。
func (s *SigWM) HideScratchShell(ctx context.Context, priorWindow w.LiveWindowID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if priorWindow == "" {
		return nil
	}
	// navigate → focus (SSOT §7.5 F5).
	_, _ = s.Exec.Run(ctx, "window", "navigate", string(priorWindow))
	if _, err := s.Exec.Run(ctx, "window", "focus", string(priorWindow)); err != nil {
		return fmt.Errorf("sigwm.HideScratchShell: focus prior %s: %w", priorWindow, err)
	}
	return nil
}

// --- Omniwm self-heal contract (requirements v2.8 §8.9) -----------------

// OmniwmHealth describes the current health of the omniwm side as
// observed via omniwmctl queries. Used by Controller.RecoverOmniwm to
// decide which recovery ladder step (Lv1-Lv4) to apply.
type OmniwmHealth struct {
	// Reachable reports whether `omniwmctl ping` succeeded. False means
	// omniwm.app is crashed / not running and Lv3-Lv4 (kickstart) is the
	// only path back.
	Reachable bool
	// TrackedApps lists the bundleIDs omniwm currently tracks. The §8.9
	// probe checks that all manifest-declared managedApps appear here;
	// a missing managed bundle (e.g. ghostty after a pkill cascade) is
	// a Lv2 trigger.
	TrackedApps map[string]bool
	// RuleCount is the number of app-rules omniwm has loaded. Below the
	// expected count triggers Lv1 (omniwm-deploy re-push).
	RuleCount int
	// CockpitWindowSeen reports whether omniwm currently observes a
	// window with bundleId=com.mitchellh.ghostty and title prefix
	// "projwm-cockpit-". Missing means SpawnCockpit is needed (existing
	// planner path) or omniwm tracking lost it (Lv2 ghostty relaunch).
	CockpitWindowSeen bool
}

// ProbeOmniwmHealth queries omniwmctl to assemble an OmniwmHealth snapshot.
// Best-effort: a partial probe (e.g. ping ok but query apps timed out)
// returns Reachable=true with empty TrackedApps and lets the caller decide.
//
// Settle policy: TrackedApps may be empty in the first millisecond after
// omniwm boots while it's still walking AX. We retry up to 3 times with
// a 500ms delay so the caller doesn't trigger a spurious Lv2 relaunch
// just because the probe raced the AX walk.
func (s *SigWM) ProbeOmniwmHealth(ctx context.Context) OmniwmHealth {
	h := OmniwmHealth{TrackedApps: map[string]bool{}}
	// ping
	if _, err := s.Exec.Run(ctx, "ping"); err != nil {
		h.Reachable = false
		return h
	}
	h.Reachable = true
	// query apps with settle retry (avoids spurious Lv2 fire when omniwm
	// is still walking AX after a restart and TrackedApps is transiently
	// empty).
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return h
			case <-time.After(500 * time.Millisecond):
			}
		}
		out, err := s.Exec.Run(ctx, "query", "apps", "--format", "json")
		if err != nil {
			continue
		}
		var resp struct {
			Result struct {
				Payload struct {
					Apps []struct {
						BundleID string `json:"bundleId"`
					} `json:"apps"`
				} `json:"payload"`
			} `json:"result"`
		}
		if err := json.Unmarshal(out, &resp); err != nil {
			continue
		}
		h.TrackedApps = map[string]bool{}
		for _, a := range resp.Result.Payload.Apps {
			h.TrackedApps[a.BundleID] = true
		}
		// If at least one of Ghostty/Vivaldi/Zed is tracked, accept this
		// snapshot. Otherwise retry — completely empty likely means race.
		if h.TrackedApps["com.mitchellh.ghostty"] || h.TrackedApps["com.vivaldi.Vivaldi"] || h.TrackedApps["dev.zed.Zed"] {
			break
		}
	}
	// query rules
	if out, err := s.Exec.Run(ctx, "query", "rules", "--format", "json"); err == nil {
		var resp struct {
			Result struct {
				Payload struct {
					Rules []json.RawMessage `json:"rules"`
				} `json:"payload"`
			} `json:"result"`
		}
		if err := json.Unmarshal(out, &resp); err == nil {
			h.RuleCount = len(resp.Result.Payload.Rules)
		}
	}
	// query windows for cockpit window presence
	if wins, err := s.queryWindows(ctx); err == nil {
		for _, win := range wins {
			if win.App.BundleID == "com.mitchellh.ghostty" && strings.HasPrefix(win.Title, "projwm-cockpit-") {
				h.CockpitWindowSeen = true
				break
			}
		}
	}
	return h
}

// RedeployOmniwmRules executes the omniwm-deploy binary to re-push the
// Nix-defined app-rules into omniwm runtime. Lv1 of §8.9 recovery ladder
// (no side effects beyond rule list refresh).
//
// Binary path is taken from $OMNIWM_DEPLOY_BIN if set (production path is
// injected by the projwm Nix module), else falls back to PATH lookup.
func (s *SigWM) RedeployOmniwmRules(ctx context.Context) error {
	bin := os.Getenv("OMNIWM_DEPLOY_BIN")
	if bin == "" {
		bin = "omniwm-deploy"
	}
	cmd := exec.CommandContext(ctx, bin)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sigwm.RedeployOmniwmRules: %s: %w (output: %s)", bin, err, string(out))
	}
	return nil
}

// RelaunchManagedApp quits and re-launches a managed app so omniwm picks
// it up as a fresh registration (= app-rule re-binding chance). Lv2 of
// §8.9 recovery ladder.
//
// Uses `osascript -e 'tell application "<name>" to quit'` for gentle
// AppKit-level quit, then macOS will auto-relaunch via `launchctl` or
// the app's normal startup path? — actually no, AppKit quit doesn't
// auto-relaunch. We need launching too. Use `open -na <appPath>` after
// confirming quit completed.
//
// Strategy:
//  1. AppKit quit ("tell application X to quit")
//  2. Wait up to 5s for the bundle to disappear from `pgrep -fl App.app`
//  3. `open -na <appPath>` to spawn a fresh instance
//
// appPath comes from the manifest (ManagedEnvironment.Apps); caller is
// responsible for resolving it.
func (s *SigWM) RelaunchManagedApp(ctx context.Context, appName, appPath string) error {
	if appName == "" || appPath == "" {
		return fmt.Errorf("sigwm.RelaunchManagedApp: empty appName or appPath")
	}
	quitScript := fmt.Sprintf(`tell application "%s" to quit`, appName)
	_ = exec.CommandContext(ctx, "osascript", "-e", quitScript).Run()
	// Wait for the process to disappear (best-effort).
	for i := 0; i < 10; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
		if out, _ := exec.CommandContext(ctx, "pgrep", "-fl", appPath).Output(); len(strings.TrimSpace(string(out))) == 0 {
			break
		}
	}
	// Relaunch via open -na (fresh instance).
	if err := exec.CommandContext(ctx, "open", "-na", appPath).Run(); err != nil {
		return fmt.Errorf("sigwm.RelaunchManagedApp: open -na %s: %w", appPath, err)
	}
	return nil
}

// RestartOmniwm performs a hard launchctl kickstart of the omniwm
// LaunchAgent. Lv3 of §8.9 recovery ladder. Side effect: all apps'
// workspace assignments are re-applied (app-rule re-fires) but column
// order / focus are not preserved.
//
// Should only be called after lower-level recovery (Lv1-Lv2) failed,
// and only after the controller has emitted a warning card with grace
// for user Esc.
func (s *SigWM) RestartOmniwm(ctx context.Context) error {
	uid := os.Getuid()
	label := fmt.Sprintf("gui/%d/org.nixos.omniwm", uid)
	cmd := exec.CommandContext(ctx, "launchctl", "kickstart", "-k", label)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sigwm.RestartOmniwm: launchctl kickstart -k %s: %w (output: %s)", label, err, string(out))
	}
	// Wait for omniwm to come back (up to 30s).
	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
		if _, err := s.Exec.Run(ctx, "ping"); err == nil {
			return nil
		}
	}
	return fmt.Errorf("sigwm.RestartOmniwm: omniwm did not come back within 30s")
}

// --- Cockpit invariant operations (requirements v2.8 §8.10) -------------

// MoveCockpitToParkWorkspace forces the cockpit ghostty window back to
// its ParkWorkspace (CP1) via `omniwmctl window focus <id>` + `command
// move-to-workspace <num>`. Realises requirements v2.8 §8.10 cockpit
// invariant — cockpit is always on CP1, manual moves are Tier 4
// violations and get force-reverted.
//
// Strategy:
//  1. Focus the cockpit window so `move-to-workspace` targets it.
//  2. Resolve parkWs (e.g. "CP1") to its numeric workspace id by
//     querying omniwmctl workspaces.
//  3. Issue `omniwmctl command move-to-workspace <num>`.
//
// On failure the caller should escalate to Lv2 (kill cockpit + respawn),
// realised by `KindSpawnCockpit` op which the planner already emits when
// the window goes missing.
func (s *SigWM) MoveCockpitToParkWorkspace(ctx context.Context, id w.LiveWindowID, parkWs string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if id == "" || parkWs == "" {
		return fmt.Errorf("sigwm.MoveCockpitToParkWorkspace: empty id or parkWs")
	}
	// Resolve parkWs (string like "CP1") to its omniwm numeric id.
	wss, err := s.queryWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("sigwm.MoveCockpitToParkWorkspace: queryWorkspaces: %w", err)
	}
	targetNum := -1
	for _, ws := range wss {
		if ws.RawName == parkWs || ws.DisplayName == parkWs {
			targetNum = ws.Number
			break
		}
	}
	if targetNum < 0 {
		return fmt.Errorf("sigwm.MoveCockpitToParkWorkspace: workspace %q not found", parkWs)
	}
	// Focus the cockpit window so subsequent move-to-workspace targets it.
	if _, err := s.Exec.Run(ctx, "window", "focus", string(id)); err != nil {
		return fmt.Errorf("sigwm.MoveCockpitToParkWorkspace: focus %s: %w", id, err)
	}
	// Settle: focus change is observable to omniwm within ~50ms.
	time.Sleep(100 * time.Millisecond)
	if _, err := s.Exec.Run(ctx, "command", "move-to-workspace", fmt.Sprint(targetNum)); err != nil {
		return fmt.Errorf("sigwm.MoveCockpitToParkWorkspace: move-to-workspace %d: %w", targetNum, err)
	}
	return nil
}

// --- Cockpit lifecycle reaper (requirements §8.1 / §8.8) ----------------

// reapCockpitArtifacts kills every cockpit-related process and tmux
// session on this host. Order: tmux sessions → ghostty windows → orphan
// binaries. Used by both startup reaping and graceful shutdown to enforce
// the "exactly one cockpit while projwmd runs, zero when stopped" invariant.
//
// Why three layers: tmux sessions own the cockpit binary as the pane
// process and propagate SIGHUP to it; ghostty owns the tmux client and
// closes the UI side; the final pkill catches any orphans whose parent
// died (reparented to init) before tmux could signal them.
func (s *SigWM) reapCockpitArtifacts(ctx context.Context) {
	// 1. Kill cockpit tmux sessions. `tmux kill-session` SIGHUPs every
	//    pane process in the session, taking down `fish -c projwm-cockpit`
	//    and its `projwm-cockpit` binary child.
	if out, err := exec.CommandContext(ctx, "tmux", "list-sessions", "-F", "#{session_name}").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			name := strings.TrimSpace(line)
			if name == cockpitBaseSession || strings.HasPrefix(name, "projwm-cockpit-") {
				_ = exec.CommandContext(ctx, "tmux", "kill-session", "-t", name).Run()
			}
		}
	}
	// 2. Kill ghostty windows launched with our cockpit title. pgrep -f
	//    matches against the full command line including --title=.
	_ = exec.CommandContext(ctx, "pkill", "-TERM", "-f", "ghostty.*projwm-cockpit-").Run()
	// 3. Kill remaining cockpit binary processes. Two patterns to catch
	//    both invocations (wrapper script preserves argv[0]="projwm-cockpit",
	//    direct daemon-spawned binary uses the /nix/store path).
	_ = exec.CommandContext(ctx, "pkill", "-TERM", "-f", "/bin/projwm-cockpit").Run()
	_ = exec.CommandContext(ctx, "pkill", "-TERM", "-x", "projwm-cockpit").Run()
	// Brief grace; then force-kill any holdouts.
	select {
	case <-ctx.Done():
		return
	case <-time.After(500 * time.Millisecond):
	}
	_ = exec.CommandContext(ctx, "pkill", "-KILL", "-f", "/bin/projwm-cockpit").Run()
	_ = exec.CommandContext(ctx, "pkill", "-KILL", "-x", "projwm-cockpit").Run()
	_ = exec.CommandContext(ctx, "pkill", "-KILL", "-f", "ghostty.*projwm-cockpit-").Run()
}

// ReapDuplicateCockpits enforces the §8.1 / §8.10 "exactly one cockpit"
// invariant by killing every ghostty `--title=projwm-cockpit-0`
// process that is NOT the one currently backing the tmux base session.
// "Backing" is determined by having descendant children — a healthy
// cockpit ghostty always has at least one (tmux client → login → fish →
// projwm-cockpit binary). Ghost / orphan ghostty processes without
// children are reaped.
//
// Called at daemon startup AND every 30 seconds by the omniwm-recovery
// ticker, so a duplicate that appears mid-run (AppKit relaunch race,
// manual `open -na`, daemon restart overlap) is force-collapsed to a
// single canonical cockpit within at most one tick.
//
// Requirements v2.8 §8.10 "永続的に対処" 要件への自律対応。
func (s *SigWM) ReapDuplicateCockpits(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, err := exec.CommandContext(ctx, "pgrep", "-f", "ghostty.*--title=projwm-cockpit-0").Output()
	if err != nil {
		return
	}
	pids := strings.Fields(strings.TrimSpace(string(out)))
	if len(pids) <= 1 {
		return
	}
	var keepPid string
	for _, pid := range pids {
		if children, err := exec.CommandContext(ctx, "pgrep", "-P", pid).Output(); err == nil && len(strings.TrimSpace(string(children))) > 0 {
			keepPid = pid
			break
		}
	}
	if keepPid == "" {
		smallest := -1
		for _, pid := range pids {
			if n, err := strconv.Atoi(pid); err == nil {
				if smallest < 0 || n < smallest {
					smallest = n
					keepPid = pid
				}
			}
		}
	}
	for _, pid := range pids {
		if pid == keepPid {
			continue
		}
		_ = exec.CommandContext(ctx, "kill", "-TERM", pid).Run()
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
		if err := exec.CommandContext(ctx, "kill", "-0", pid).Run(); err == nil {
			_ = exec.CommandContext(ctx, "kill", "-KILL", pid).Run()
		}
	}
}

// ReapStaleCockpit removes all cockpit artifacts at daemon startup, before
// the planner emits any SpawnCockpit op. This is the defense against
// orphans accumulated from earlier daemon crashes, `projwm tui` direct
// invocations, or ghostty crashes that reparented children to init.
//
// The planner will re-spawn exactly one cockpit on the next reconcile.
func (s *SigWM) ReapStaleCockpit(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reapCockpitArtifacts(ctx)
}

// ShutdownCockpit removes all cockpit artifacts at daemon shutdown so the
// requirement §8.8 row "projwmd 停止/再起動 → cockpit プロセスも停止"
// holds even when projwmd is killed by launchd, SIGTERM, or a panic.
func (s *SigWM) ShutdownCockpit(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reapCockpitArtifacts(ctx)
}

// nix-rebuild-marker 1779084438
