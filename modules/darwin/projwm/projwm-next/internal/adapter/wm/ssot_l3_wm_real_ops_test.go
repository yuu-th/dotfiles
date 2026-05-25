//go:build real_ops

package wm

import (
	"context"
	"os"
	"os/exec"
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
			},
		},
		Apps: w.AppEnvironment{ManagedApps: []w.ManagedAppPolicy{{
			Capability: w.CapabilityTerminal,
			BundleID:   "com.mitchellh.ghostty",
			LifecycleRemoval: w.LifecycleRemovalPolicy{
				Allowed:      true,
				Method:       w.LifecycleRemovalAXCloseGuarded,
				AllowedKinds: []w.WindowKind{w.WindowAI, w.WindowShell, w.WindowViewer},
			},
		}}},
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
