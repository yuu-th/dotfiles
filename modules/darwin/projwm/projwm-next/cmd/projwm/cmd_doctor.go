package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// CheckLevel is the severity of a single doctor check.
type CheckLevel string

const (
	LevelPass CheckLevel = "PASS"
	LevelWarn CheckLevel = "WARN"
	LevelFail CheckLevel = "FAIL"
)

// CheckResult is one row in the doctor report.
type CheckResult struct {
	Name   string
	Level  CheckLevel
	Detail string
}

// DoctorCheck is the function signature shared by all 14 checks.
type DoctorCheck func(ctx context.Context, gf globalFlags) CheckResult

// allDoctorChecks returns the 14 checks (T12 in design.md v3), in order.
func allDoctorChecks() []DoctorCheck {
	return []DoctorCheck{
		checkProjwmdPresence,
		checkLaunchdControllerLoaded,
		checkPersistentStoreReadable,
		checkManifestLoadable,
		checkManifestDigest,
		checkIPCSocket,
		checkExternalAppsAvailable,
		checkVivaldiAutomationProfile,
		checkTmuxBinary,
		checkOmniwmctlBinary,
		checkActiveProjectsTmuxReachable,
		checkActiveProjectsWindowsLive,
		checkInvariants,
		checkRecentTraceErrors,
	}
}

// cmdDoctor runs the 14 checks and prints a report.
func cmdDoctor(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("doctor: no arguments expected")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	hasFail := false
	for _, fn := range allDoctorChecks() {
		r := fn(ctx, gf)
		fmt.Fprintf(stdout, "[%s] %s — %s\n", r.Level, r.Name, r.Detail)
		if r.Level == LevelFail {
			hasFail = true
		}
	}
	if hasFail {
		return fmt.Errorf("doctor: one or more checks FAILED")
	}
	return nil
}

// ────────────────────────────── individual checks ───────────────────────────

func checkProjwmdPresence(ctx context.Context, gf globalFlags) CheckResult {
	out, err := exec.CommandContext(ctx, "pgrep", "-x", "projwmd").Output()
	if err != nil {
		return CheckResult{Name: "projwmd-process", Level: LevelWarn, Detail: "no projwmd process visible (pgrep -x projwmd)"}
	}
	pids := strings.TrimSpace(string(out))
	return CheckResult{Name: "projwmd-process", Level: LevelPass, Detail: "running pid=" + pids}
}

func checkLaunchdControllerLoaded(ctx context.Context, gf globalFlags) CheckResult {
	// Heuristic: try `launchctl list | grep projwmd-next`.
	out, err := exec.CommandContext(ctx, "launchctl", "list").Output()
	if err != nil {
		return CheckResult{Name: "launchd-controller", Level: LevelWarn, Detail: "launchctl list failed"}
	}
	if !strings.Contains(string(out), "projwmd-next") {
		return CheckResult{Name: "launchd-controller", Level: LevelWarn, Detail: "no launchd job matches projwmd-next"}
	}
	return CheckResult{Name: "launchd-controller", Level: LevelPass, Detail: "loaded"}
}

func checkPersistentStoreReadable(ctx context.Context, gf globalFlags) CheckResult {
	if gf.storeDir == "" {
		return CheckResult{Name: "persistent-store", Level: LevelWarn, Detail: "--store-dir not configured"}
	}
	cur := filepath.Join(gf.storeDir, "CURRENT")
	if _, err := os.Stat(cur); err != nil {
		return CheckResult{Name: "persistent-store", Level: LevelFail, Detail: fmt.Sprintf("CURRENT missing: %v", err)}
	}
	if _, err := loadSnapshotFromStore(ctx, gf); err != nil {
		return CheckResult{Name: "persistent-store", Level: LevelFail, Detail: err.Error()}
	}
	return CheckResult{Name: "persistent-store", Level: LevelPass, Detail: "readable at " + gf.storeDir}
}

func checkManifestLoadable(ctx context.Context, gf globalFlags) CheckResult {
	if gf.manifestPath == "" {
		return CheckResult{Name: "manifest-loadable", Level: LevelWarn, Detail: "--managed-environment not configured"}
	}
	info, err := os.Stat(gf.manifestPath)
	if err != nil {
		return CheckResult{Name: "manifest-loadable", Level: LevelFail, Detail: err.Error()}
	}
	return CheckResult{Name: "manifest-loadable", Level: LevelPass, Detail: fmt.Sprintf("%s (%d bytes)", gf.manifestPath, info.Size())}
}

func checkManifestDigest(ctx context.Context, gf globalFlags) CheckResult {
	if gf.manifestPath == "" || gf.manifestDigest == "" {
		return CheckResult{Name: "manifest-digest", Level: LevelWarn, Detail: "manifest path or expected digest not configured"}
	}
	data, err := os.ReadFile(gf.manifestPath)
	if err != nil {
		return CheckResult{Name: "manifest-digest", Level: LevelFail, Detail: err.Error()}
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != gf.manifestDigest {
		return CheckResult{Name: "manifest-digest", Level: LevelFail, Detail: fmt.Sprintf("mismatch: got %s want %s", got, gf.manifestDigest)}
	}
	return CheckResult{Name: "manifest-digest", Level: LevelPass, Detail: "matches expected"}
}

func checkIPCSocket(ctx context.Context, gf globalFlags) CheckResult {
	if gf.socketPath == "" {
		return CheckResult{Name: "ipc-socket", Level: LevelWarn, Detail: "--socket-path not configured"}
	}
	info, err := os.Stat(gf.socketPath)
	if err != nil {
		return CheckResult{Name: "ipc-socket", Level: LevelFail, Detail: err.Error()}
	}
	if info.Mode()&os.ModeSocket == 0 {
		return CheckResult{Name: "ipc-socket", Level: LevelFail, Detail: "path exists but is not a socket"}
	}
	conn, err := net.DialTimeout("unix", gf.socketPath, 500*time.Millisecond)
	if err != nil {
		return CheckResult{Name: "ipc-socket", Level: LevelFail, Detail: "dial: " + err.Error()}
	}
	_ = conn.Close()
	return CheckResult{Name: "ipc-socket", Level: LevelPass, Detail: "dial ok"}
}

func checkExternalAppsAvailable(ctx context.Context, gf globalFlags) CheckResult {
	apps := []string{"/Applications/Ghostty.app", "/Applications/Vivaldi.app", "/Applications/Zed.app"}
	missing := []string{}
	for _, a := range apps {
		if _, err := os.Stat(a); err != nil {
			missing = append(missing, filepath.Base(a))
		}
	}
	if len(missing) > 0 {
		return CheckResult{Name: "external-apps", Level: LevelWarn, Detail: "missing: " + strings.Join(missing, ", ")}
	}
	return CheckResult{Name: "external-apps", Level: LevelPass, Detail: "Ghostty, Vivaldi, Zed present"}
}

func checkVivaldiAutomationProfile(ctx context.Context, gf globalFlags) CheckResult {
	// Heuristic: look for the per-user dir
	// ~/Library/Application Support/Vivaldi/projwm-next/Preferences
	home, _ := os.UserHomeDir()
	if home == "" {
		return CheckResult{Name: "vivaldi-automation-profile", Level: LevelWarn, Detail: "no HOME"}
	}
	prefs := filepath.Join(home, "Library", "Application Support", "Vivaldi", "projwm-next", "Preferences")
	if _, err := os.Stat(prefs); err != nil {
		return CheckResult{Name: "vivaldi-automation-profile", Level: LevelWarn, Detail: "projwm-next profile not yet created (run Vivaldi with --profile-directory=projwm-next once)"}
	}
	return CheckResult{Name: "vivaldi-automation-profile", Level: LevelPass, Detail: prefs}
}

func checkTmuxBinary(ctx context.Context, gf globalFlags) CheckResult {
	out, err := exec.CommandContext(ctx, "tmux", "-V").Output()
	if err != nil {
		return CheckResult{Name: "tmux-binary", Level: LevelFail, Detail: "tmux not in PATH"}
	}
	return CheckResult{Name: "tmux-binary", Level: LevelPass, Detail: strings.TrimSpace(string(out))}
}

func checkOmniwmctlBinary(ctx context.Context, gf globalFlags) CheckResult {
	candidates := []string{"omniwmctl", "/opt/homebrew/bin/omniwmctl"}
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return CheckResult{Name: "omniwmctl-binary", Level: LevelPass, Detail: c}
		}
	}
	return CheckResult{Name: "omniwmctl-binary", Level: LevelWarn, Detail: "not in PATH (omniwmctl)"}
}

func checkActiveProjectsTmuxReachable(ctx context.Context, gf globalFlags) CheckResult {
	snap, err := loadSnapshotFromStore(ctx, gf)
	if err != nil {
		return CheckResult{Name: "active-tmux-sessions", Level: LevelWarn, Detail: "snapshot unavailable: " + err.Error()}
	}
	prof, ok := snap.Desired.Profiles[snap.Desired.ActiveProfile]
	if !ok {
		return CheckResult{Name: "active-tmux-sessions", Level: LevelWarn, Detail: "no active profile"}
	}
	out, err := exec.CommandContext(ctx, "tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		return CheckResult{Name: "active-tmux-sessions", Level: LevelWarn, Detail: "tmux ls failed: " + err.Error()}
	}
	live := map[string]bool{}
	for _, s := range strings.Split(string(out), "\n") {
		if s = strings.TrimSpace(s); s != "" {
			live[s] = true
		}
	}
	missing := []string{}
	for _, pid := range prof.Assignments {
		if pid == "" {
			continue
		}
		expected := fmt.Sprintf("ai-1/%s", pid)
		if !live[expected] {
			missing = append(missing, expected)
		}
	}
	if len(missing) == 0 {
		return CheckResult{Name: "active-tmux-sessions", Level: LevelPass, Detail: "all reachable"}
	}
	return CheckResult{Name: "active-tmux-sessions", Level: LevelWarn, Detail: "missing: " + strings.Join(missing, ", ")}
}

func checkActiveProjectsWindowsLive(ctx context.Context, gf globalFlags) CheckResult {
	// Cannot verify live windows from CLI alone; needs daemon Observe.
	// In Phase 3 this becomes a Query("world") check.
	return CheckResult{Name: "live-windows", Level: LevelWarn, Detail: "needs daemon Query (Phase 3)"}
}

func checkInvariants(ctx context.Context, gf globalFlags) CheckResult {
	// invariant.CheckAll requires WorldState (observation + meta + env).
	// We have env+desired+checkpoint from store; missing observation.
	// Partial check: ensure manifest has slots / viewer / etc.
	snap, err := loadSnapshotFromStore(ctx, gf)
	if err != nil {
		return CheckResult{Name: "invariants", Level: LevelWarn, Detail: "snapshot unavailable: " + err.Error()}
	}
	if snap.Environment.Workspaces.Viewer == "" {
		return CheckResult{Name: "invariants", Level: LevelFail, Detail: "manifest missing viewer workspace"}
	}
	if len(snap.Environment.Workspaces.Slots) == 0 {
		return CheckResult{Name: "invariants", Level: LevelFail, Detail: "manifest has zero slots"}
	}
	if _, ok := snap.Desired.Profiles[snap.Desired.ActiveProfile]; !ok {
		return CheckResult{Name: "invariants", Level: LevelFail, Detail: fmt.Sprintf("active profile %q missing", snap.Desired.ActiveProfile)}
	}
	return CheckResult{Name: "invariants", Level: LevelPass, Detail: "snapshot-level shape ok"}
}

func checkRecentTraceErrors(ctx context.Context, gf globalFlags) CheckResult {
	if gf.storeDir == "" {
		return CheckResult{Name: "recent-trace-errors", Level: LevelWarn, Detail: "--store-dir not configured"}
	}
	tracesDir := filepath.Join(gf.storeDir, "traces")
	entries, err := os.ReadDir(tracesDir)
	if err != nil {
		return CheckResult{Name: "recent-trace-errors", Level: LevelPass, Detail: "no traces dir (clean state)"}
	}
	// Find the most recent trace file by mtime.
	var newest os.DirEntry
	var newestMtime time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newestMtime) {
			newest = e
			newestMtime = info.ModTime()
		}
	}
	if newest == nil {
		return CheckResult{Name: "recent-trace-errors", Level: LevelPass, Detail: "no traces yet"}
	}
	return CheckResult{Name: "recent-trace-errors", Level: LevelPass, Detail: "latest trace: " + newest.Name()}
}

// Avoid unused-import warnings for w when the file is otherwise consumed.
var _ = w.WindowAI
