//go:build real_ops

package wm

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/yuu-th/projwm-next/internal/adapter/session"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func requireRealWM(t *testing.T) {
	t.Helper()
	if os.Getenv("PROJWM_REAL_OP_TESTS") != "1" {
		t.Skip("set PROJWM_REAL_OP_TESTS=1 to run real_ops tests")
	}
	for _, bin := range []string{"omniwmctl", "tmux"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not available: %v", bin, err)
		}
	}
}

func realOpsEnv() w.ManagedEnvironment {
	return w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		WindowManager: w.WindowManagerEnvironment{Backend: "omniwm", Layout: w.LayoutTuning{MaxVisibleColumns: 2, MaxWindowsPerColumn: 4}},
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{
				{ID: "8", RawName: "8", DisplayName: "8", Role: w.WorkspaceGeneral},
				{ID: "9", RawName: "9", DisplayName: "9", Role: w.WorkspaceGeneral},
				// ws13 = Q, on the UNNAMED external display (display:2, name="").
				// Lets the reorder real_ops tests run on the same nameless-display
				// workspace that breaks ACC-S7 / Jump, isolating display-specific
				// reorder behaviour from the named-display ws8.
				{ID: "13", RawName: "13", DisplayName: "Q", Role: w.WorkspaceGeneral},
			},
		},
		Apps: w.AppEnvironment{ManagedApps: []w.ManagedAppPolicy{
			{
				Capability: w.CapabilityTerminal,
				BundleID:   "com.mitchellh.ghostty",
				LifecycleRemoval: w.LifecycleRemovalPolicy{
					Allowed:      true,
					Method:       w.LifecycleRemovalAXCloseGuarded,
					AllowedKinds: []w.WindowKind{w.WindowAI, w.WindowShell, w.WindowViewer},
				},
			},
			{
				// Zed is single-process (ZED-CONFIG/§4.4): removal is AXClose of
				// the window only, never a process kill. Needed for ATTR-F1.
				Capability: w.CapabilityEditor,
				BundleID:   "dev.zed.Zed",
				LifecycleRemoval: w.LifecycleRemovalPolicy{
					Allowed:      true,
					Method:       w.LifecycleRemovalAXCloseGuarded,
					AllowedKinds: []w.WindowKind{w.WindowEditor},
				},
			},
		}},
	}
}

func newRealSigWM() *SigWM {
	sw := NewSigWM(realOpsEnv(), nil, nil)
	sw.Tmux = &session.Client{}
	return sw
}

func TestRealOpsObserveWorld(t *testing.T) {
	requireRealWM(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := newRealSigWM().Observe(ctx); err != nil {
		t.Fatalf("Observe: %v", err)
	}
}

func TestRealOpsFocusWorkspace(t *testing.T) {
	requireRealWM(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := newRealSigWM().FocusWorkspace(ctx, "9"); err != nil {
		t.Fatalf("FocusWorkspace(9): %v", err)
	}
}

func TestRealOpsFocusWorkspaceNonExistent(t *testing.T) {
	requireRealWM(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := newRealSigWM().FocusWorkspace(ctx, "NOPE"); err == nil {
		t.Fatal("FocusWorkspace(NOPE) succeeded, want error")
	}
}

func TestRealOpsSpawnShell(t *testing.T) {
	requireRealWM(t)
	if _, err := exec.LookPath("ghostty"); err != nil {
		t.Skipf("ghostty not available: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	sw := newRealSigWM()
	title := "shell-1:projwm-next-test"
	sessionName := "shell-1/projwm-next-test"
	t.Cleanup(func() {
		_ = (&session.Client{}).KillSession(context.Background(), sessionName)
		_ = closeLiveWindowsWithTitle(context.Background(), sw, title)
	})
	_ = (&session.Client{}).KillSession(ctx, sessionName)
	_ = closeLiveWindowsWithTitle(ctx, sw, title)

	live, err := sw.Spawn(ctx, SpawnRequest{
		Workspace:   "8",
		Kind:        w.WindowShell,
		Desired:     w.DesiredWindowID{Project: "projwm-next-test", Kind: w.WindowShell, Index: 1},
		Title:       title,
		BundleID:    "com.mitchellh.ghostty",
		ProjectPath: t.TempDir(),
		TmuxSession: sessionName,
	})
	if err != nil {
		t.Fatalf("Spawn shell: %v", err)
	}
	obs, err := sw.Observe(ctx)
	if err != nil {
		t.Fatalf("Observe after spawn: %v", err)
	}
	win, ok := obs.Windows[live]
	if !ok {
		t.Fatalf("spawn returned %q but observe did not include it", live)
	}
	if win.Title.Value != title || win.Workspace != "8" {
		t.Fatalf("spawned shell = %+v, want title=%q workspace=8", win, title)
	}
}

// TestRealOpsSpawnAISendsAICommand verifies SSOT §4.4 AI-CMD-FIRST + AI-MULTI:
// when a WindowAI is spawned with a freshly-created tmux session and an
// AICommand, sigwm.Spawn sends that command via tmux send-keys so the AI
// runner launches inside the session.
//
// L3 verification uses `tmux capture-pane` instead of omniwm Observe because
// omniwm window registration races (S27 known gap) often beat the test's
// deadline even though the underlying tmux + spawn surface is correct.
func TestRealOpsSpawnAISendsAICommand(t *testing.T) {
	requireRealWM(t)
	if _, err := exec.LookPath("ghostty"); err != nil {
		t.Skipf("ghostty not available: %v", err)
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skipf("tmux not available: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sw := newRealSigWM()
	title := "ai-1:projwm-next-test"
	sessionName := "ai-1/projwm-next-test"
	t.Cleanup(func() {
		_ = (&session.Client{}).KillSession(context.Background(), sessionName)
		_ = closeLiveWindowsWithTitle(context.Background(), sw, title)
	})
	_ = (&session.Client{}).KillSession(ctx, sessionName)
	_ = closeLiveWindowsWithTitle(ctx, sw, title)

	_, err := sw.Spawn(ctx, SpawnRequest{
		Workspace:   "8",
		Kind:        w.WindowAI,
		Desired:     w.DesiredWindowID{Project: "projwm-next-test", Kind: w.WindowAI, Index: 1},
		Title:       title,
		BundleID:    "com.mitchellh.ghostty",
		ProjectPath: t.TempDir(),
		TmuxSession: sessionName,
		AICommand:   "claude",
	})
	if err != nil {
		t.Fatalf("Spawn AI: %v", err)
	}

	// Wait briefly for tmux to register the send-keys + shell to render
	// the typed command on the pane.
	time.Sleep(800 * time.Millisecond)

	out, err := exec.CommandContext(ctx, "tmux", "capture-pane", "-t", sessionName, "-p").Output()
	if err != nil {
		t.Fatalf("tmux capture-pane: %v", err)
	}
	pane := string(out)
	// The pane should show the literal "claude" string somewhere — either as
	// the command on the prompt line (if `claude` binary isn't found) or as
	// the running process. Either way, send-keys did run.
	if !strings.Contains(pane, "claude") {
		t.Fatalf("tmux pane did not contain 'claude' after AI spawn — send-keys path missed.\npane:\n%s", pane)
	}
}

func closeLiveWindowsWithTitle(ctx context.Context, sw *SigWM, title string) error {
	obs, err := sw.Observe(ctx)
	if err != nil {
		return err
	}
	for id, win := range obs.Windows {
		if win.Title.Value == title {
			_ = sw.Close(ctx, id)
		}
	}
	return nil
}
