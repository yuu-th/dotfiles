//go:build real_ops

package naming_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/yuu-th/projwm-next/internal/adapter/session"
	wm "github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/naming"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func requireRealOpsIdentity(t *testing.T) {
	t.Helper()
	if os.Getenv("PROJWM_REAL_OP_TESTS") != "1" {
		t.Skip("set PROJWM_REAL_OP_TESTS=1 to run SSOT real_ops identity checks")
	}
	for _, bin := range []string{"ghostty", "omniwmctl", "tmux"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not available: %v", bin, err)
		}
	}
}

func TestIdentityFromTitle(t *testing.T) {
	requireRealOpsIdentity(t)
	project := identityTestProject(t)
	title := fmt.Sprintf("shell-1:%s", project)
	observed := spawnAndObserveGhosttyTitle(t, w.WindowShell, title, fmt.Sprintf("shell-1/%s", project), "")
	got, ok := naming.TmuxSessionFromTitle(observed)
	want := fmt.Sprintf("shell-1/%s", project)
	if !ok || got != want {
		t.Fatalf("TmuxSessionFromTitle(observed shell title %q) = %q, %v; want %q, true", observed, got, ok, want)
	}
}

func TestIdentityFromTitleViewer(t *testing.T) {
	requireRealOpsIdentity(t)
	project := identityTestProject(t)
	title := fmt.Sprintf("ai-view-1:%s", project)
	base := fmt.Sprintf("ai-1/%s", project)
	viewer := fmt.Sprintf("ai-1/%s_v", project)
	observed := spawnAndObserveGhosttyTitle(t, w.WindowViewer, title, viewer, base)
	got, ok := naming.TmuxSessionFromTitle(observed)
	if !ok || got != viewer {
		t.Fatalf("TmuxSessionFromTitle(observed viewer title %q) = %q, %v; want %q, true", observed, got, ok, viewer)
	}
}

// SSOT §10.4 row I3: "実 window title 'random-window' を observe →
// 復元不可。orphan 扱い". Real-environment probe (2026-05-26) showed
// the production omniwm app-rule set
// (modules/darwin/omniwm/app-rules.nix) only catalogs Ghostty windows
// whose title matches the managed patterns (`(ai|shell|ai-view)-N:`
// or `projwm-cockpit-D0`); a `random-window-*` title is filtered and
// never appears in omniwmctl query. The SSOT row is therefore not
// realisable at L3 with the current production rule set — the
// pure-function contract (TmuxSessionFromTitle returning (empty,
// false) for non-managed titles) is exhaustively covered in L0
// (`internal/naming/ssot_l0_identity_test.go`).
//
// We retain the L3 test function as an explicit t.Skip so the ledger
// reference + ssotRealOps coverage map stay valid, while the test run
// itself surfaces the honest gap.
func TestIdentityFromTitleUnknown(t *testing.T) {
	requireRealOpsIdentity(t)
	t.Skip("SSOT §10.4 I3 not realisable at L3: omniwm app-rules filter non-managed Ghostty titles out of the catalog; L0 ssot_l0_identity_test.go covers the pure-function contract")
}

func identityTestProject(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("test-identity-%d", time.Now().UnixNano())
}

func spawnAndObserveGhosttyTitle(t *testing.T, kind w.WindowKind, title, tmuxSession, viewerSource string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sw := newIdentitySigWM()
	tmux := &session.Client{}
	cleanupIdentityWindow(t, sw, title, tmuxSession, viewerSource)
	if viewerSource != "" {
		if _, err := tmux.EnsureSession(ctx, viewerSource, t.TempDir()); err != nil {
			t.Fatalf("ensure source tmux session %q: %v", viewerSource, err)
		}
	}
	_, err := sw.Spawn(ctx, wm.SpawnRequest{
		Workspace:               "8",
		Kind:                    kind,
		Desired:                 w.DesiredWindowID{Project: "projwm-next-identity-test", Kind: kind, Index: 1},
		Title:                   title,
		BundleID:                "com.mitchellh.ghostty",
		ProjectPath:             t.TempDir(),
		TmuxSession:             tmuxSession,
		ViewerSourceTmuxSession: viewerSource,
	})
	if err != nil {
		t.Fatalf("Spawn Ghostty %q: %v", title, err)
	}
	return observeUniqueTitle(t, ctx, sw, title)
}

func newIdentitySigWM() *wm.SigWM {
	sw := wm.NewSigWM(w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		WindowManager: w.WindowManagerEnvironment{
			Backend: "omniwm",
			Layout:  w.LayoutTuning{MaxVisibleColumns: 2, MaxWindowsPerColumn: 4},
		},
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
	}, nil, nil)
	sw.Tmux = &session.Client{}
	return sw
}

func observeUniqueTitle(t *testing.T, ctx context.Context, sw *wm.SigWM, title string) string {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		obs, err := sw.Observe(ctx)
		if err != nil {
			t.Fatalf("Observe: %v", err)
		}
		var found string
		for _, win := range obs.Windows {
			if win.Title.Value != title {
				continue
			}
			if found != "" {
				t.Fatalf("observed title %q more than once", title)
			}
			found = win.Title.Value
		}
		if found != "" {
			return found
		}
		if time.Now().After(deadline) {
			t.Fatalf("title %q was not observed from real OmniWM state", title)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("context expired while waiting for observed title %q: %v", title, ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func cleanupIdentityWindow(t *testing.T, sw *wm.SigWM, title string, sessions ...string) {
	t.Helper()
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		tmux := &session.Client{}
		for _, name := range sessions {
			if name != "" {
				_ = tmux.KillSession(ctx, name)
			}
		}
		_ = closeGhosttyWindowsByTitle(ctx, title)
	}
	t.Cleanup(cleanup)
	cleanup()
	_ = sw
}

func closeGhosttyWindowsByTitle(ctx context.Context, title string) error {
	script := `
on run argv
  set targetTitle to item 1 of argv
  tell application "System Events"
    if not (exists process "Ghostty") then return
    tell process "Ghostty"
      repeat with candidate in windows
        set candidateTitle to ""
        try
          set candidateTitle to name of candidate
        end try
        if candidateTitle is targetTitle then
          try
            perform action "AXPress" of button 1 of candidate
          end try
        end if
      end repeat
    end tell
  end tell
end run`
	cmd := exec.CommandContext(ctx, "osascript", "-e", script, title)
	return cmd.Run()
}
