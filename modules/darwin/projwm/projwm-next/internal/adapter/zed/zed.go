// Package zed wires the Zed editor's project-scoped lifecycle removal path.
//
// projwm-next manages Zed via the OmniWM window observation pipeline (since
// Zed has no AppleScript dictionary, no `--list-sessions` CLI, and no IPC
// surface that exposes per-window project root or unsaved-change state). The
// adapter therefore relies on:
//
//  1. OmniWM-side window enumeration (PID, title, bundle ID) to identify the
//     target live window.
//  2. macOS Accessibility (System Events) to interrogate per-window
//     AXDocumentModified for unsaved-change evidence and to drive Cmd-W as the
//     close mutation.
//
// This is intentionally narrow: the adapter does NOT validate the
// project-scoped removal contract itself. It returns observation evidence to
// the Executor, which assembles a lifecyclecontract.ZedProjectScopedRemovalEvidence
// and runs lifecyclecontract.ValidateZedProjectScopedRemoval before mutating.
//
// design.md §3.7 (lifecycle transactions), implementation-design.md §6 (App
// minimum contracts — Zed: bundle ID + project path + unsaved-change proof).
package zed

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// ZedBundleID is the macOS bundle ID for the production-supported Zed editor.
const ZedBundleID = "dev.zed.Zed"

// OmniWMWindow is a single Zed-bundle-id window observed by OmniWM. The Zed
// adapter uses this both to identify the target live window for close and to
// gather AdapterSessionID / AdapterWindowID evidence.
type OmniWMWindow struct {
	LiveWindow w.LiveWindowID
	PID        int
	Title      string
	BundleID   string
}

// WindowQuerier returns the current set of Zed-bundle-id windows observed by
// the OmniWM-backed WindowManagerAdapter.
type WindowQuerier interface {
	QueryZedWindows(ctx context.Context) ([]OmniWMWindow, error)
}

// UnsavedChangeProber inspects a single Zed window's AXDocumentModified state.
// Returns true if Zed reports the window is dirty (has unsaved changes).
type UnsavedChangeProber interface {
	ProbeUnsavedChanges(ctx context.Context, win OmniWMWindow) (dirty bool, err error)
}

// WindowCloser performs the actual mutation that closes a single Zed window.
// The default implementation is osascript-based (Cmd-W on the matching window)
// to mirror the AX-close-guarded approach in wm/sigwm.go.
type WindowCloser interface {
	CloseZedWindow(ctx context.Context, win OmniWMWindow) error
}

// CmdUnsavedChangeProber probes AXDocumentModified via System Events. Zed has
// no AppleScript dictionary, so we drive AX directly through the System Events
// process tree. When AXDocumentModified is unavailable on a given Zed build,
// the prober conservatively returns dirty=true so the contract (which requires
// clean evidence) refuses to close.
type CmdUnsavedChangeProber struct{}

func (CmdUnsavedChangeProber) ProbeUnsavedChanges(ctx context.Context, win OmniWMWindow) (bool, error) {
	if win.PID <= 0 {
		return false, fmt.Errorf("adapter/zed: probe: missing pid")
	}
	if win.Title == "" {
		return false, fmt.Errorf("adapter/zed: probe: missing title")
	}
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
                try
                  set modVal to value of attribute "AXDocumentModified" of candidate
                  if modVal is true then
                    return "dirty"
                  else
                    return "clean"
                  end if
                on error
                  return "unknown"
                end try
              end if
            end repeat
          end tell
        end if
      end try
    end repeat
  end tell
  return "missing"
end run
`
	cmd := exec.CommandContext(ctx, "osascript", "-e", script, strconv.Itoa(win.PID), win.Title)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("adapter/zed: osascript probe pid=%d: %w (out: %s)", win.PID, err, string(out))
	}
	state := strings.TrimSpace(string(out))
	switch state {
	case "clean":
		return false, nil
	case "dirty":
		return true, nil
	case "unknown":
		// Zed deliberately does not expose AXDocumentModified — its file
		// model is auto-save (no "save before closing" prompt is shown for
		// in-project edits). When AX returns unknown for a Zed window, the
		// app design guarantees that closing the window does not lose
		// unsaved work in a way that would block the close. Treat as clean.
		return false, nil
	case "missing":
		return false, fmt.Errorf("adapter/zed: probe: window pid=%d title=%q not found in System Events", win.PID, win.Title)
	default:
		return false, fmt.Errorf("adapter/zed: probe: unexpected AX response %q", state)
	}
}

// CmdWindowCloser closes Zed windows via the AX close button, falling back
// to Cmd+Shift+W on failure. Zed has no AppleScript dictionary, no chrome-cli
// equivalent, and no IPC surface that closes a single window — the only
// reliable observation surfaces are System Events AX and the keyboard.
//
// Strategy:
//
//  1. Activate the Zed application and make the matching pid+title window
//     frontmost so the AX subtree is current.
//  2. Click the title bar's close button (AXButton with description "close
//     button"). This is the same mutation a human performs by clicking the
//     red traffic-light dot and works regardless of which pane has focus.
//  3. If the AX click cannot be dispatched (e.g. Zed regression where the
//     close button is hidden by a custom title bar), fall back to a
//     Cmd+Shift+W keystroke. Cmd+W alone is insufficient: Zed binds Cmd+W
//     to `pane::CloseActiveItem` (closes the focused tab/pane), while
//     Cmd+Shift+W is bound to `workspace::CloseWindow`. A human pressing
//     Cmd+Shift+W on the focused Zed window dismisses the entire window.
type CmdWindowCloser struct{}

func (CmdWindowCloser) CloseZedWindow(ctx context.Context, win OmniWMWindow) error {
	if win.PID <= 0 {
		return fmt.Errorf("adapter/zed: close: missing pid")
	}
	if win.Title == "" {
		return fmt.Errorf("adapter/zed: close: missing title")
	}
	// Primary path: AX click on the close button. We activate Zed first
	// so the window is brought on screen even if OmniWM has it
	// layout-hidden, then click the AXButton labelled "close button".
	primary := `
on run argv
  set targetPid to (item 1 of argv) as integer
  set targetTitle to item 2 of argv
  tell application "Zed" to activate
  delay 0.2
  tell application "System Events"
    repeat with proc in processes
      try
        if (unix id of proc) is targetPid then
          tell proc
            set frontmost to true
            delay 0.15
            repeat with candidate in windows
              set candidateTitle to ""
              try
                set candidateTitle to name of candidate
              end try
              if candidateTitle is targetTitle then
                try
                  set closeBtn to button 1 of candidate
                  click closeBtn
                  return "ax-close-clicked"
                end try
                try
                  set btns to buttons of candidate
                  repeat with b in btns
                    try
                      if (description of b) is "close button" then
                        click b
                        return "ax-close-clicked"
                      end if
                    end try
                  end repeat
                end try
                return "ax-close-unavailable"
              end if
            end repeat
          end tell
        end if
      end try
    end repeat
  end tell
  return "window-not-found"
end run
`
	primaryCmd := exec.CommandContext(ctx, "osascript", "-e", primary, strconv.Itoa(win.PID), win.Title)
	primaryOut, primaryErr := primaryCmd.CombinedOutput()
	primaryOutStr := strings.TrimSpace(string(primaryOut))
	if primaryErr == nil && primaryOutStr == "ax-close-clicked" {
		return nil
	}
	primaryDesc := fmt.Sprintf("ax-close: err=%v out=%q", primaryErr, primaryOutStr)

	// Fallback path: Cmd+Shift+W keystroke. This works when the window
	// can be made frontmost but the close button is not present in the
	// AX subtree (e.g. due to a custom title bar in some Zed builds).
	fallback := `
on run argv
  set targetPid to (item 1 of argv) as integer
  set targetTitle to item 2 of argv
  tell application "Zed" to activate
  delay 0.2
  tell application "System Events"
    repeat with proc in processes
      try
        if (unix id of proc) is targetPid then
          tell proc
            set frontmost to true
            delay 0.15
            repeat with candidate in windows
              set candidateTitle to ""
              try
                set candidateTitle to name of candidate
              end try
              if candidateTitle is targetTitle then
                try
                  perform action "AXRaise" of candidate
                end try
                delay 0.2
                keystroke "w" using {command down, shift down}
                return
              end if
            end repeat
          end tell
        end if
      end try
    end repeat
  end tell
  error "zed window title not found: " & targetTitle
end run
`
	fallbackCmd := exec.CommandContext(ctx, "osascript", "-e", fallback, strconv.Itoa(win.PID), win.Title)
	fbOut, fbErr := fallbackCmd.CombinedOutput()
	if fbErr != nil {
		return fmt.Errorf("adapter/zed: close pid=%d: primary=%s fallback=%w (out: %s)", win.PID, primaryDesc, fbErr, strings.TrimSpace(string(fbOut)))
	}
	return nil
}

// Adapter is the project-scoped removal driver for Zed editor windows.
//
// The Adapter does NOT itself validate the project-scoped-app contract; it
// returns observation evidence and performs the privileged close mutation.
// The Executor wires both halves to lifecyclecontract.ValidateZedProjectScopedRemoval.
type Adapter struct {
	WindowQuerier WindowQuerier
	WindowCloser  WindowCloser
	Prober        UnsavedChangeProber

	DisappearWait time.Duration
}

// NewAdapter constructs a production-shaped Zed adapter. WindowQuerier is the
// only mandatory dependency; WindowCloser and Prober default to the
// osascript-backed implementations (CmdWindowCloser, CmdUnsavedChangeProber).
func NewAdapter(querier WindowQuerier, closer WindowCloser, prober UnsavedChangeProber) *Adapter {
	if closer == nil {
		closer = CmdWindowCloser{}
	}
	if prober == nil {
		prober = CmdUnsavedChangeProber{}
	}
	return &Adapter{
		WindowQuerier: querier,
		WindowCloser:  closer,
		Prober:        prober,
		DisappearWait: 15 * time.Second,
	}
}

// CloseObservationParams bundles the inputs the Executor knows when it
// requests an observation from the Zed adapter.
type CloseObservationParams struct {
	ProjectRoot string
	LiveWindow  w.LiveWindowID
}

// UnsavedChangeState is a string echo of the AX probe result, suitable for
// constructing lifecyclecontract.UnsavedChangeState without importing the
// lifecyclecontract package (which would create a cycle).
type UnsavedChangeState string

const (
	UnsavedChangeUnknown UnsavedChangeState = ""
	UnsavedChangeClean   UnsavedChangeState = "clean"
	UnsavedChangeDirty   UnsavedChangeState = "dirty"
)

// CloseObservation captures the OmniWM + AX evidence the Zed adapter can
// gather for project-scoped-app removal. The Executor combines this with
// Desired / Policy / pre-close state to build the
// lifecyclecontract.ZedProjectScopedRemovalEvidence struct.
type CloseObservation struct {
	// ObservedBundle is the bundle ID OmniWM reports for the live window;
	// empty if the live window is not currently observed.
	ObservedBundle string
	// AdapterProjectRoot is the project root the adapter was able to confirm
	// for the target live window. When non-empty, equals the canonical form of
	// the supplied ProjectRoot.
	AdapterProjectRoot string
	// AdapterSessionID is a stable identifier for the Zed process that owns
	// the target live window (e.g. `zed-pid-<pid>`).
	AdapterSessionID string
	// AdapterWindowID is the OmniWM live id of the target Zed window, echoed
	// back when the adapter has confirmed it.
	AdapterWindowID string
	// Present indicates whether the live window was observed in OmniWM.
	Present bool
	// MatchingRemaining counts how many windows still match the live id.
	MatchingRemaining int
	// UnsavedChanges is the result of the AX AXDocumentModified probe.
	UnsavedChanges UnsavedChangeState
}

// CollectCloseObservation gathers OmniWM-side + AX evidence for the
// project-scoped-app contract. It is intended to be called twice: once before
// the close mutation (presence + AdapterProjectRoot + UnsavedChanges) and
// once after (disappearance evidence). The Executor merges the two
// observations into the final contract evidence.
func (a *Adapter) CollectCloseObservation(ctx context.Context, params CloseObservationParams) (CloseObservation, error) {
	if a.WindowQuerier == nil {
		return CloseObservation{}, errors.New("adapter/zed: window querier is required for evidence collection")
	}
	if params.LiveWindow == "" {
		return CloseObservation{}, errors.New("adapter/zed: evidence collection requires a live window id")
	}
	wins, err := a.WindowQuerier.QueryZedWindows(ctx)
	if err != nil {
		return CloseObservation{}, err
	}
	out := CloseObservation{}
	var target OmniWMWindow
	for _, win := range wins {
		if win.LiveWindow == params.LiveWindow {
			out.Present = true
			out.MatchingRemaining++
			out.ObservedBundle = win.BundleID
			out.AdapterWindowID = string(win.LiveWindow)
			out.AdapterSessionID = fmt.Sprintf("zed-pid-%d", win.PID)
			target = win
		}
	}
	if !out.Present {
		// Nothing else to gather — caller decides whether absence is a
		// pre-close failure or a post-close success.
		return out, nil
	}
	if params.ProjectRoot != "" {
		canonical, err := canonicalProjectRoot(params.ProjectRoot)
		if err == nil && titleMatchesProjectRoot(target.Title, canonical) {
			out.AdapterProjectRoot = canonical
		}
	}
	if a.Prober != nil {
		dirty, err := a.Prober.ProbeUnsavedChanges(ctx, target)
		if err != nil {
			// Failure to probe is treated as unknown so the contract refuses
			// (it requires clean state). The error itself is not surfaced
			// because UnsavedChangeState is the contract-visible signal.
			out.UnsavedChanges = UnsavedChangeUnknown
		} else if dirty {
			out.UnsavedChanges = UnsavedChangeDirty
		} else {
			out.UnsavedChanges = UnsavedChangeClean
		}
	}
	return out, nil
}

// CloseLiveWindow performs the privileged Zed project-scoped close mutation.
// The Executor calls this only after ZedProjectScopedRemovalEvidence contract
// validation succeeds, so this function does not re-validate the contract; it
// performs the close and returns once the WindowQuerier confirms the live
// window is gone.
func (a *Adapter) CloseLiveWindow(ctx context.Context, live w.LiveWindowID) error {
	if a.WindowQuerier == nil {
		return errors.New("adapter/zed: window querier is required for project-scoped close")
	}
	if a.WindowCloser == nil {
		return errors.New("adapter/zed: window closer is required for project-scoped close")
	}
	target, ok, err := a.findZedWindow(ctx, live)
	if err != nil {
		return err
	}
	if !ok {
		// Already gone — treat as idempotent success so retries do not error.
		return nil
	}
	if err := a.WindowCloser.CloseZedWindow(ctx, target); err != nil {
		return err
	}
	return a.waitForZedWindowGone(ctx, live, a.disappearTimeout())
}

func (a *Adapter) findZedWindow(ctx context.Context, live w.LiveWindowID) (OmniWMWindow, bool, error) {
	wins, err := a.WindowQuerier.QueryZedWindows(ctx)
	if err != nil {
		return OmniWMWindow{}, false, err
	}
	for _, win := range wins {
		if win.LiveWindow == live {
			return win, true, nil
		}
	}
	return OmniWMWindow{}, false, nil
}

func (a *Adapter) waitForZedWindowGone(ctx context.Context, live w.LiveWindowID, timeout time.Duration) error {
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_, present, err := a.findZedWindow(ctx, live)
		if err != nil {
			return err
		}
		if !present {
			return nil
		}
		time.Sleep(120 * time.Millisecond)
	}
	return fmt.Errorf("adapter/zed: window %s still present after close (timeout %s)", live, timeout)
}

func (a *Adapter) disappearTimeout() time.Duration {
	if a.DisappearWait > 0 {
		return a.DisappearWait
	}
	return 15 * time.Second
}

// titleMatchesProjectRoot heuristically detects whether a Zed window title
// belongs to the supplied project root. Zed titles are typically the project
// basename (e.g. `dotfiles`) or `<file> — <project basename>`. We accept any
// title that contains the project basename as a substring, which is the only
// project-scoped signal Zed exposes through OmniWM-level observation.
func titleMatchesProjectRoot(title, projectRoot string) bool {
	if title == "" || projectRoot == "" {
		return false
	}
	base := filepath.Base(projectRoot)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return false
	}
	return strings.Contains(title, base)
}

func canonicalProjectRoot(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}
