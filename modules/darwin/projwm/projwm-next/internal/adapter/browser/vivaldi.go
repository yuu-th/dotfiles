package browser

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	w "github.com/yuu-th/projwm-next/internal/world"
)

type AppOpener interface {
	Open(ctx context.Context, appPath string, args ...string) error
}

type CmdAppOpener struct{}

func (CmdAppOpener) Open(ctx context.Context, appPath string, args ...string) error {
	if appPath == "" {
		return errors.New("browser/vivaldi: app path is required")
	}
	full := append([]string{"-na", appPath, "--args"}, args...)
	cmd := exec.CommandContext(ctx, "open", full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("browser/vivaldi: open failed: %w (output bytes=%d)", err, len(out))
	}
	return nil
}

// VivaldiOmniWMWindow is a single Vivaldi browser window observed via the
// WindowManagerAdapter (omniwm). chrome-cli does not support Vivaldi, so the
// adapter relies on omniwm window observation to enumerate Vivaldi windows.
type VivaldiOmniWMWindow struct {
	LiveWindow w.LiveWindowID
	PID        int
	Title      string
	BundleID   string
}

// VivaldiWindowQuerier returns the current set of Vivaldi-bundle-id windows
// observed by the WindowManagerAdapter. The adapter uses this both at Open
// time (to diff before/after) and at close time (to validate identity and
// observe disappearance).
type VivaldiWindowQuerier interface {
	QueryVivaldiWindows(ctx context.Context) ([]VivaldiOmniWMWindow, error)
}

// VivaldiWindowCloser performs the actual mutation that closes a single
// Vivaldi window. The default implementation is osascript-based (Cmd-W on the
// matching window). The interface keeps this surface narrow so unit tests can
// stub the privileged mutation without spawning processes.
type VivaldiWindowCloser interface {
	CloseVivaldiWindow(ctx context.Context, win VivaldiOmniWMWindow) error
}

// CmdVivaldiWindowCloser closes Vivaldi windows via AppleScript. Vivaldi has
// a functional AppleScript dictionary that exposes a `close window` verb, so
// we drive the close through `tell application "Vivaldi"` directly: this is
// the most reliable mutation surface and works even when the target window is
// hidden, offscreen, or on a non-focused workspace (which the keyboard-based
// Cmd-W / Cmd+Shift+W path does not, because the keystroke goes to the
// frontmost-visible window of the process rather than the title-matched one).
// If the AppleScript-direct close fails (e.g. the dictionary regresses in a
// future Vivaldi version), we fall back to System Events Cmd+Shift+W against
// the matching pid+title window. Cmd+W alone is insufficient because Vivaldi
// binds Cmd+W to "Close Tab" rather than "Close Window"; Cmd+Shift+W is the
// macOS standard "close window" shortcut and Vivaldi honors it when the
// target window can be made frontmost.
type CmdVivaldiWindowCloser struct{}

func (CmdVivaldiWindowCloser) CloseVivaldiWindow(ctx context.Context, win VivaldiOmniWMWindow) error {
	if win.PID <= 0 {
		return fmt.Errorf("browser/vivaldi: close: missing pid")
	}
	if win.Title == "" {
		return fmt.Errorf("browser/vivaldi: close: missing title")
	}
	// Primary path: Vivaldi's own AppleScript dictionary. We try the
	// observation-time exact title first; if that doesn't match (Vivaldi's
	// internal title can drift away from OmniWM's NSWindow title between
	// observation and close), we fall back to closing the only window of
	// Vivaldi if just one is open. This lenient match keeps lifecycle
	// removal deterministic in single-window automation profiles.
	primary := `
on run argv
  set targetTitle to item 1 of argv
  set toClose to {}
  tell application "Vivaldi"
    repeat with wnd in (every window)
      try
        if (title of wnd) is targetTitle then
          copy wnd to end of toClose
        end if
      end try
    end repeat
    if (count of toClose) is 0 then
      set wins to (every window)
      if (count of wins) is 1 then
        copy item 1 of wins to end of toClose
      end if
    end if
    repeat with wnd in toClose
      try
        close wnd
      end try
    end repeat
  end tell
  return (count of toClose) as text
end run
`
	primaryCmd := exec.CommandContext(ctx, "osascript", "-e", primary, win.Title)
	out, err := primaryCmd.CombinedOutput()
	primaryOut := strings.TrimSpace(string(out))
	var primaryErr error
	if err == nil && primaryOut != "0" {
		// AppleScript-direct closed at least one matching window.
		return nil
	}
	if err == nil {
		primaryErr = fmt.Errorf("vivaldi-direct close: 0 windows matched exact title %q", win.Title)
	} else {
		primaryErr = fmt.Errorf("vivaldi-direct close: %w (out: %s)", err, primaryOut)
	}

	// Fallback path: System Events Cmd+Shift+W against the matching
	// pid+title window. This works when the AppleScript dictionary path
	// is unavailable (e.g. Vivaldi build regression) but only when the
	// window can be made frontmost.
	fallback := `
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
              if candidateTitle is targetTitle or (count of windows) is 1 then
                set frontmost to true
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
  error "vivaldi window title not found: " & targetTitle
end run
`
	fallbackCmd := exec.CommandContext(ctx, "osascript", "-e", fallback, strconv.Itoa(win.PID), win.Title)
	if fbOut, fbErr := fallbackCmd.CombinedOutput(); fbErr != nil {
		return fmt.Errorf("browser/vivaldi: close pid=%d: primary=%v fallback=%w (out: %s)", win.PID, primaryErr, fbErr, strings.TrimSpace(string(fbOut)))
	}
	return nil
}

// VivaldiBundleID is the macOS bundle ID for the production-supported Vivaldi
// browser. Used for identity correlation in OmniWM observations.
const VivaldiBundleID = "com.vivaldi.Vivaldi"

type VivaldiAdapter struct {
	PrivateStore   PrivatePayloadStore
	Opener         AppOpener
	AppPath        string
	WindowQuerier  VivaldiWindowQuerier
	WindowCloser   VivaldiWindowCloser
	SettleTimeout  time.Duration
	DisappearWait  time.Duration
	// ProfileBaseDir is the parent directory that contains Vivaldi profile
	// subdirectories (e.g. ~/Library/Application Support/Vivaldi on macOS).
	// When non-empty, OpenInProfile purges the Session_* / Tabs_* snapshot
	// files before launch so Vivaldi does not restore tabs from a prior run.
	ProfileBaseDir string
	// UserDataDir is a projwm-private Chromium user-data-dir for the managed
	// (automation) Vivaldi instance (B-05). Launching with `--user-data-dir`
	// (instead of `--profile-directory` within the user's data dir) forks a
	// SEPARATE Vivaldi process whose argv retains the flag — so it is both
	// isolated from the user's Vivaldi AND detectable by vivaldiManaged
	// (which inspects process args). Empty disables the isolation (tests).
	UserDataDir string
	// ManagedProcessAlive reports whether a managed Vivaldi process (one
	// carrying the projwm --user-data-dir in its argv) is currently running.
	// OpenInProfile consults it to make a repeated spawn idempotent against an
	// IN-FLIGHT first-run launch: if a managed process is already alive but its
	// window has not surfaced yet (slow first-run profile generation), a fresh
	// `open` would not create a second window (Chromium single-instance per
	// user-data-dir) but WOULD reset the settle clock — so the converge loop's
	// re-emitted spawn-browser must instead WAIT on the in-flight window rather
	// than relaunch. nil falls back to the real pgrep against the user-data-dir
	// leaf; tests inject a stub.
	ManagedProcessAlive func() bool
}

// AutomationUserDataLeaf is the stable path suffix of the managed Vivaldi
// user-data-dir. wm.vivaldiInspectFunc matches process argv against this
// substring to classify automation-profile windows.
const AutomationUserDataLeaf = "projwm-next/vivaldi-data"

const VivaldiAutomationProfile = "projwm-next"

// browserOpenSettleDefault is the default OpenInProfile settle budget. A COLD
// managed --user-data-dir triggers Vivaldi first-run profile generation, which
// delays creation of the new browser window (and thus the omniwm observation
// the diff settle waits on) by ~40s on the observed hardware. The previous 15s
// default timed out on first-run, so OpenInProfile returned an error, the
// spawn was abandoned, and the converge loop re-emitted spawn-browser. Sizing
// the budget above the measured first-run delay lets the FIRST spawn wait out
// generation and return the real BrowserWindowID. Subsequent spawns reuse the
// warm profile and settle in well under a second, so the larger ceiling only
// matters on the rare cold path.
const browserOpenSettleDefault = 75 * time.Second

// NewVivaldiAdapter constructs a VivaldiAdapter without WM-level evidence
// collection. It is suitable for callers that only need the Open path and
// will not request browser-window-close lifecycle removal. For full
// browser-window-close support use NewVivaldiAdapterWithWM.
func NewVivaldiAdapter(privateStore PrivatePayloadStore, opener AppOpener, appPath string) *VivaldiAdapter {
	return NewVivaldiAdapterWithWM(privateStore, opener, appPath, nil, nil)
}

// NewVivaldiAdapterWithWM is the production constructor. WindowQuerier is
// required for OpenInProfile to populate OpenResult.BrowserWindowID /
// LiveWindow via OmniWM diff and for CollectCloseEvidence to gather isolation
// + disappearance evidence. WindowCloser is the privileged mutation surface
// invoked by CloseLiveWindow when the Executor dispatches a
// LifecycleRemovalBrowserWindowClose kill-session.
func NewVivaldiAdapterWithWM(privateStore PrivatePayloadStore, opener AppOpener, appPath string, querier VivaldiWindowQuerier, closer VivaldiWindowCloser) *VivaldiAdapter {
	if opener == nil {
		opener = CmdAppOpener{}
	}
	if appPath == "" {
		appPath = "/Applications/Vivaldi.app"
	}
	if closer == nil {
		closer = CmdVivaldiWindowCloser{}
	}
	profileBaseDir := ""
	userDataDir := ""
	if home, err := os.UserHomeDir(); err == nil {
		profileBaseDir = filepath.Join(home, "Library", "Application Support", "Vivaldi")
		// B-05: dedicated, isolated user-data-dir for the managed Vivaldi.
		userDataDir = filepath.Join(home, ".cache", filepath.FromSlash(AutomationUserDataLeaf))
	}
	return &VivaldiAdapter{
		PrivateStore:   privateStore,
		Opener:         opener,
		AppPath:        appPath,
		WindowQuerier:  querier,
		WindowCloser:   closer,
		SettleTimeout:  browserOpenSettleDefault,
		DisappearWait:  15 * time.Second,
		ProfileBaseDir: profileBaseDir,
		UserDataDir:    userDataDir,
	}
}

func (a *VivaldiAdapter) ObserveWindows(ctx context.Context) ([]WindowSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("browser/vivaldi: direct structure observation is outside this adapter; use WindowManagerAdapter correlation")
}

func (a *VivaldiAdapter) FocusWindow(ctx context.Context, id w.LiveWindowID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New("browser/vivaldi: direct focus is outside this adapter; use WindowManagerAdapter focus")
}

func (a *VivaldiAdapter) OpenInProfile(ctx context.Context, profile string, payloadToken string) (OpenResult, error) {
	if err := ctx.Err(); err != nil {
		return OpenResult{}, err
	}
	if a.PrivateStore == nil {
		return OpenResult{}, errors.New("browser/vivaldi: private payload store is required")
	}
	if a.Opener == nil {
		return OpenResult{}, errors.New("browser/vivaldi: opener is required")
	}
	if payloadToken == "" {
		return OpenResult{}, errors.New("browser/vivaldi: private payload token is required")
	}
	if strings.TrimSpace(profile) == "" || profile == "default" {
		return OpenResult{}, errors.New("browser/vivaldi: automation-owned non-default profile is required")
	}
	payload, err := a.PrivateStore.Get(ctx, payloadToken)
	if err != nil {
		return OpenResult{}, err
	}
	if len(payload.URLs) == 0 {
		return OpenResult{}, errors.New("browser/vivaldi: private payload has no URLs")
	}
	// Snapshot Vivaldi windows before launch so we can identify the new window
	// by diff. WindowQuerier is optional: when absent, we fall back to the
	// pre-Open behaviour (no BrowserWindowID), so the legacy Spawn path stays
	// compatible with mocks that don't wire WM observation.
	//
	// HONEST GAP (SSOT §4.4 BR-EXIST): the production Vivaldi window title
	// is "<page-title> - Vivaldi" — it does NOT encode which Chromium
	// profile the window belongs to. Production omniwmctl observations
	// therefore cannot disambiguate "same automation profile, different
	// window" from "different automation profile, same page" via title
	// alone. To realise BR-EXIST we need the WindowQuerier to return the
	// profile per window (probably by reading the Vivaldi process
	// `--profile-directory` argv). That extension lives in a later slice;
	// for now OpenInProfile always opens a new window, and the
	// scenarios-level test (TestSpawnBrowserAlreadyExists) is allowed to
	// document the duplication.
	var beforeIDs map[w.LiveWindowID]VivaldiOmniWMWindow
	if a.WindowQuerier != nil {
		beforeIDs, err = a.snapshotWindows(ctx)
		if err != nil {
			return OpenResult{}, err
		}
	}
	// Purge leftover session snapshots so Vivaldi opens a fresh window
	// containing only the requested URLs. Only safe when no automation window
	// is currently open (live Vivaldi would be writing to these files).
	if len(beforeIDs) == 0 {
		if a.UserDataDir != "" {
			purgeSessionFiles(filepath.Join(a.UserDataDir, "Default"))
		} else if a.ProfileBaseDir != "" {
			purgeSessionFiles(filepath.Join(a.ProfileBaseDir, profile))
		}
	}
	// ALWAYS issue a --new-window launch (handoff §14.11, experiment-verified).
	//
	// `open -na Vivaldi --new-window --user-data-dir=<dir>` creates a NEW window
	// whether the managed instance is fresh, alive+windowed, OR alive+WINDOWLESS.
	// The last case is the archive→unarchive→assign re-deploy: archive closes
	// the project's browser window (LifecycleRemovalBrowserWindowClose) but the
	// SHARED automation instance keeps running with no window. A new spawn-
	// browser must therefore re-launch to get a window.
	//
	// History: this used to skip the launch when a managed process was alive
	// ("inFlight"), on the premise that re-issuing open "would NOT create a
	// second window — Chromium is single-instance". A direct experiment
	// disproved that premise (open --new-window adds a window to a live
	// instance), and the skip deadlocked the post-archive re-deploy: process
	// alive + windowless → skip launch → settleNewVivaldiWindow waited the full
	// budget for a window that never came → spawn-browser respawn loop (the S2
	// failure, observed twice). The generous settleNewVivaldiWindow budget
	// (browserOpenSettleDefault ≈ cold first-run) means the FIRST OpenInProfile
	// call settles successfully before the converge loop replans, so always
	// launching does not duplicate windows in the first-run case either.
	managedAlive := a.WindowQuerier != nil && a.UserDataDir != "" && a.managedProcessAlive()
	vivTracef("OpenInProfile: beforeIDs=%d managedAlive=%v (WindowQuerier=%v UserDataDir=%q) urls=%d", len(beforeIDs), managedAlive, a.WindowQuerier != nil, a.UserDataDir, len(payload.URLs))
	// B-05: dedicated --user-data-dir forks a SEPARATE process (isolated from the
	// user's Vivaldi) whose argv retains --user-data-dir, so vivaldiManaged can
	// classify the window as a managed browser.
	var args []string
	if a.UserDataDir != "" {
		args = []string{"--new-window", "--user-data-dir=" + a.UserDataDir}
	} else {
		args = []string{"--new-window", "--profile-directory=" + profile}
	}
	args = append(args, payload.URLs...)
	vivTracef("OpenInProfile: LAUNCHING open %s %v", a.AppPath, args)
	if err := a.Opener.Open(ctx, a.AppPath, args...); err != nil {
		vivTracef("OpenInProfile: LAUNCH ERROR: %v", redactedAppOpenError(ctx, err))
		return OpenResult{}, redactedAppOpenError(ctx, err)
	}
	if a.WindowQuerier == nil {
		return OpenResult{}, nil
	}
	live, err := a.settleNewVivaldiWindow(ctx, beforeIDs)
	if err != nil {
		vivTracef("OpenInProfile: settle ERROR: %v", err)
		return OpenResult{}, err
	}
	vivTracef("OpenInProfile: settle FOUND live=%s", live)
	return OpenResult{BrowserWindowID: string(live), LiveWindow: live}, nil
}

// vivTracef is a gated (PROJWM_NEXT_PLANNER_TRACE=1), read-only, timestamped
// diagnostic for the browser-spawn convergence investigation (handoff §14.11).
func vivTracef(format string, args ...interface{}) {
	if os.Getenv("PROJWM_NEXT_PLANNER_TRACE") != "1" {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[VIV_TRACE %s] %s\n", time.Now().Format("15:04:05.000"), msg)
}

// managedProcessAlive reports whether a managed Vivaldi process is currently
// running. Uses the injected ManagedProcessAlive hook when set (tests),
// otherwise pgrep -fl against the user-data-dir leaf in the process argv.
func (a *VivaldiAdapter) managedProcessAlive() bool {
	if a.ManagedProcessAlive != nil {
		return a.ManagedProcessAlive()
	}
	out, err := exec.Command("pgrep", "-fl", AutomationUserDataLeaf).Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

// CloseWindow is the legacy interface satisfaction. The Executor routes
// browser-window-close through CloseLiveWindow + CollectCloseEvidence; raw
// CloseWindow remains intentionally unsupported so callers must go through
// the lifecycle contract path rather than bypassing evidence collection.
func (a *VivaldiAdapter) CloseWindow(ctx context.Context, id w.LiveWindowID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New("browser/vivaldi: direct close is outside this adapter; use lifecycle policy and WindowManagerAdapter")
}

// CloseLiveWindow performs the privileged Vivaldi browser-window-close
// mutation. The Executor calls this only after the
// VivaldiBrowserWindowCloseEvidence contract validation succeeds, so this
// function does not re-validate the contract; it just performs the close and
// returns once the WindowQuerier confirms the live window is gone.
func (a *VivaldiAdapter) CloseLiveWindow(ctx context.Context, live w.LiveWindowID) error {
	if a.WindowQuerier == nil {
		return errors.New("browser/vivaldi: window querier is required for browser-window-close")
	}
	if a.WindowCloser == nil {
		return errors.New("browser/vivaldi: window closer is required for browser-window-close")
	}
	target, ok, err := a.findVivaldiWindow(ctx, live)
	if err != nil {
		return err
	}
	if !ok {
		// Already gone — treat as idempotent success so retries do not error.
		return nil
	}
	if err := a.WindowCloser.CloseVivaldiWindow(ctx, target); err != nil {
		return err
	}
	return a.waitForVivaldiWindowGone(ctx, live, a.disappearTimeout())
}

// VivaldiCloseObservation captures the OmniWM-side facts the adapter can
// gather for browser-window-close evidence. The Executor combines this with
// Desired / Policy / pre-close observations to build the
// lifecyclecontract.VivaldiBrowserWindowCloseEvidence struct. Keeping the
// adapter return type small avoids importing lifecyclecontract from the
// browser package (which would create a dependency cycle, since
// lifecyclecontract already imports browser for VivaldiAutomationProfile).
type VivaldiCloseObservation struct {
	// ObservedBundle is the bundle ID OmniWM reports for the live window;
	// empty if the live window is not currently observed.
	ObservedBundle string
	// CorrelatedBrowserID is the OmniWM live id echoed back when the live
	// window is found. Used to correlate the planner's BrowserWindowID with
	// the adapter's actual observation.
	CorrelatedBrowserID string
	// CorrelatedLiveWindow is the same id but typed as LiveWindowID for the
	// contract's WM correlation field.
	CorrelatedLiveWindow w.LiveWindowID
	// Present indicates whether the live window was observed.
	Present bool
	// MatchingRemaining counts how many windows still match the live id (used
	// by post-close disappearance evidence).
	MatchingRemaining int
	// ObservedPayloadToken is the payload token replayed through the
	// PrivatePayloadStore. Empty if the payload could not be resolved.
	ObservedPayloadToken string
	// TabPayloadCorrelated is true when the payload was successfully
	// resolved. Combined with token equality the contract treats this as
	// tab/payload correlation.
	TabPayloadCorrelated bool
	// UserProfileIsolated is true when no other Vivaldi windows tied to the
	// supplied profile are observed alongside the target.
	UserProfileIsolated bool
}

// CollectCloseObservation gathers the OmniWM-side evidence needed for the
// browser-window-close contract. It is intended to be called twice: once
// before the close mutation (to capture isolation + payload correlation +
// presence) and once after (to capture disappearance). The Executor merges
// the two observations into the final contract evidence.
func (a *VivaldiAdapter) CollectCloseObservation(ctx context.Context, params CloseObservationParams) (VivaldiCloseObservation, error) {
	if a.WindowQuerier == nil {
		return VivaldiCloseObservation{}, errors.New("browser/vivaldi: window querier is required for evidence collection")
	}
	if params.LiveWindow == "" {
		return VivaldiCloseObservation{}, errors.New("browser/vivaldi: evidence collection requires a live window id")
	}
	wins, err := a.WindowQuerier.QueryVivaldiWindows(ctx)
	if err != nil {
		return VivaldiCloseObservation{}, err
	}
	out := VivaldiCloseObservation{}
	otherSameProfile := 0
	for _, win := range wins {
		if win.LiveWindow == params.LiveWindow {
			out.Present = true
			out.MatchingRemaining++
			out.ObservedBundle = win.BundleID
			out.CorrelatedBrowserID = string(win.LiveWindow)
			out.CorrelatedLiveWindow = win.LiveWindow
			continue
		}
		if win.BundleID == VivaldiBundleID && titleBelongsToProfile(win.Title, params.Profile) {
			otherSameProfile++
		}
	}
	out.UserProfileIsolated = out.Present && otherSameProfile == 0
	if a.PrivateStore != nil && params.PayloadToken != "" {
		if payload, err := a.PrivateStore.Get(ctx, params.PayloadToken); err == nil && len(payload.URLs) > 0 {
			out.ObservedPayloadToken = params.PayloadToken
			out.TabPayloadCorrelated = true
		}
	}
	return out, nil
}

// CloseObservationParams bundles the inputs the Executor knows when it
// requests an observation from the adapter.
type CloseObservationParams struct {
	Profile      string
	PayloadToken string
	LiveWindow   w.LiveWindowID
}

// titleBelongsToProfile heuristically detects whether a Vivaldi window title
// suffix matches the supplied automation profile name. Vivaldi appends the
// profile name to window titles when multiple profiles are signed in, in the
// form "Page Title - <profile>". This is the same evidence Vivaldi exposes to
// users for distinguishing windows across profiles, and is the only profile
// signal accessible to OmniWM observation. When the title cannot be
// disambiguated we err on the side of "may belong to this profile" so that
// CollectCloseObservation cannot silently report UserProfileIsolated when
// another profile window of the same bundle id is open.
func titleBelongsToProfile(title, profile string) bool {
	if profile == "" {
		return true
	}
	suffix := " - " + profile
	return strings.HasSuffix(title, suffix) || title == profile || strings.Contains(title, suffix)
}

func (a *VivaldiAdapter) snapshotWindows(ctx context.Context) (map[w.LiveWindowID]VivaldiOmniWMWindow, error) {
	wins, err := a.WindowQuerier.QueryVivaldiWindows(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[w.LiveWindowID]VivaldiOmniWMWindow, len(wins))
	for _, win := range wins {
		out[win.LiveWindow] = win
	}
	return out, nil
}

func (a *VivaldiAdapter) settleNewVivaldiWindow(ctx context.Context, before map[w.LiveWindowID]VivaldiOmniWMWindow) (w.LiveWindowID, error) {
	timeout := a.SettleTimeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	deadline := time.Now().Add(timeout)
	var lastCount int
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		now, err := a.snapshotWindows(ctx)
		if err != nil {
			return "", err
		}
		var candidates []w.LiveWindowID
		for id := range now {
			if _, existed := before[id]; !existed {
				candidates = append(candidates, id)
			}
		}
		lastCount = len(candidates)
		if len(candidates) == 1 {
			return candidates[0], nil
		}
		if len(candidates) > 1 {
			return "", fmt.Errorf("browser/vivaldi: ambiguous newly opened windows count=%d", len(candidates))
		}
		time.Sleep(150 * time.Millisecond)
	}
	return "", fmt.Errorf("browser/vivaldi: timeout waiting for new browser window (last new count=%d)", lastCount)
}

func (a *VivaldiAdapter) findVivaldiWindow(ctx context.Context, live w.LiveWindowID) (VivaldiOmniWMWindow, bool, error) {
	wins, err := a.WindowQuerier.QueryVivaldiWindows(ctx)
	if err != nil {
		return VivaldiOmniWMWindow{}, false, err
	}
	for _, win := range wins {
		if win.LiveWindow == live {
			return win, true, nil
		}
	}
	return VivaldiOmniWMWindow{}, false, nil
}

func (a *VivaldiAdapter) waitForVivaldiWindowGone(ctx context.Context, live w.LiveWindowID, timeout time.Duration) error {
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
		_, present, err := a.findVivaldiWindow(ctx, live)
		if err != nil {
			return err
		}
		if !present {
			return nil
		}
		time.Sleep(120 * time.Millisecond)
	}
	return fmt.Errorf("browser/vivaldi: window %s still present after close (timeout %s)", live, timeout)
}

func (a *VivaldiAdapter) disappearTimeout() time.Duration {
	if a.DisappearWait > 0 {
		return a.DisappearWait
	}
	return 15 * time.Second
}

var _ BrowserCapabilityAdapter = (*VivaldiAdapter)(nil)

// WindowTabs pairs a Vivaldi window's title with its tab URLs in order.
// Used by the browser tab observer (SSOT §4.4 BR-TAB-OBS) to attribute
// user-driven tab changes to a specific managed window — the flat
// URL list returned by InspectTabs loses window boundaries.
type WindowTabs struct {
	// Title is the Vivaldi window's title (e.g., "browser-1:dotfiles").
	// Managed automation-profile windows carry the controller-owned
	// title; user-profile windows have arbitrary titles and are
	// classified as External by naming.IdentityFromTitle.
	Title string
	// URLs preserve Vivaldi tab order (left-to-right).
	URLs []string
}

// InspectTabsByWindow enumerates every Vivaldi window with its title and
// per-window tab URL list. Used by the BrowserTabsSync observer so it
// can map a tab change to the right managed DesiredWindow (SSOT §4.1
// OP14-17 take a WindowID, not a flat tab index).
//
// The AppleScript uses an explicit separator (`\x1f` ASCII unit-separator
// between title and URLs, `\x1e` record-separator between windows) so
// titles containing newlines or colons do not corrupt the parse.
func (a *VivaldiAdapter) InspectTabsByWindow(ctx context.Context) ([]WindowTabs, error) {
	const script = `tell application "Vivaldi"
	set out to ""
	set unit to (ASCII character 31)
	set record_sep to (ASCII character 30)
	repeat with w in windows
		set wt to ""
		try
			set wt to title of w
		end try
		set out to out & wt & unit
		set urlList to {}
		repeat with t in tabs of w
			set end of urlList to URL of t
		end repeat
		set AppleScript's text item delimiters to (ASCII character 29)
		set out to out & (urlList as text) & record_sep
		set AppleScript's text item delimiters to ""
	end repeat
	return out
end tell`
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("browser/vivaldi: osascript inspect tabs by window: %w", err)
	}
	return parseInspectTabsByWindow(string(out)), nil
}

// parseInspectTabsByWindow splits the osascript output produced by
// InspectTabsByWindow into structured WindowTabs. Exported indirectly
// for test purposes — keeps the AppleScript-free parsing logic unit
// testable.
func parseInspectTabsByWindow(raw string) []WindowTabs {
	const (
		groupSep  = "\x1d" // between URLs within one window
		fieldSep  = "\x1f" // between title and url-list
		recordSep = "\x1e" // between windows
	)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	records := strings.Split(raw, recordSep)
	var out []WindowTabs
	for _, rec := range records {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		parts := strings.SplitN(rec, fieldSep, 2)
		if len(parts) != 2 {
			continue
		}
		title := strings.TrimSpace(parts[0])
		var urls []string
		for _, u := range strings.Split(parts[1], groupSep) {
			u = strings.TrimSpace(u)
			if u != "" {
				urls = append(urls, u)
			}
		}
		out = append(out, WindowTabs{Title: title, URLs: urls})
	}
	return out
}

// InspectTabs enumerates the URLs of every tab in every Vivaldi window via
// AppleScript. Returned URLs preserve the order produced by the Vivaldi tab
// AppleScript dictionary; callers must treat them as opaque (the canary URL
// must not be logged outside the PrivatePayloadStore audit boundary).
//
// Used by PRIV.6.5b acceptance to confirm that a payload-token-driven
// browser restore actually populates the live window with the requested
// URLs (chrome-cli is not Vivaldi-compatible).
func (a *VivaldiAdapter) InspectTabs(ctx context.Context) ([]string, error) {
	const script = `tell application "Vivaldi"
	set urlList to {}
	repeat with w in windows
		repeat with t in tabs of w
			set end of urlList to URL of t
		end repeat
	end repeat
	set AppleScript's text item delimiters to linefeed
	return urlList as text
end tell`
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("browser/vivaldi: osascript inspect tabs: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var urls []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			urls = append(urls, l)
		}
	}
	return urls, nil
}

func redactedAppOpenError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return fmt.Errorf("browser/vivaldi: open failed (%T)", err)
}

// purgeSessionFiles removes Vivaldi's auto-restore session snapshots from the
// given profile directory. Vivaldi (Chromium-based) writes Session_* and
// Tabs_* binary files inside the Sessions/ subdirectory on every open/close.
// These are replayed on the next launch, causing URLs from past runs to
// accumulate in the automation window. Purging them before launch guarantees a
// clean window containing only the URLs requested via OpenInProfile.
//
// Best-effort: individual removal errors are silently ignored because a locked
// or missing file is non-fatal — Vivaldi will just restore whatever it can.
func purgeSessionFiles(profileDir string) {
	if profileDir == "" {
		return
	}
	sessionsDir := filepath.Join(profileDir, "Sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "Session_") || strings.HasPrefix(name, "Tabs_") {
			os.Remove(filepath.Join(sessionsDir, name))
		}
	}
}
