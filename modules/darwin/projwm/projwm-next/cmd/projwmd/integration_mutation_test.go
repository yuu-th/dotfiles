//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	wm "github.com/yuu-th/projwm-next/internal/adapter/wm"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// TestRealMutationSmoke spawns a Ghostty window with a unique controller-owned
// title via SigWM, moves it to another real workspace, verifies the move, then
// closes it via AppleScript (Close is policy-blocked on SigWM so cleanup is
// out-of-band). This is the only mutation smoke gated by
// PROJWM_NEXT_REAL_MUTATION=1.
//
// IMPORTANT: this disrupts the live macOS GUI session. Do not run unattended.
func TestRealMutationSmoke(t *testing.T) {
	if os.Getenv("PROJWM_NEXT_REAL_MUTATION") != "1" {
		t.Skip("set PROJWM_NEXT_REAL_MUTATION=1 to run real mutation smoke")
	}
	if _, err := exec.LookPath("omniwmctl"); err != nil {
		t.Skipf("omniwmctl not found: %v", err)
	}
	if _, err := os.Stat("/Applications/Ghostty.app"); err != nil {
		t.Skipf("Ghostty.app not installed: %v", err)
	}

	rootCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// 1. Discover live workspaces via omniwmctl directly.
	wss := mustQueryWorkspaces(t, rootCtx)
	if len(wss) < 2 {
		t.Fatalf("need at least 2 omniwm workspaces, got %d", len(wss))
	}
	// Pick source = currently focused, target = first other.
	var src, dst ctlWS
	for _, ws := range wss {
		if ws.IsCurrent {
			src = ws
			break
		}
	}
	for _, ws := range wss {
		if ws.Number != src.Number {
			dst = ws
			break
		}
	}
	if src.Number == 0 || dst.Number == 0 {
		t.Fatalf("could not pick src/dst workspaces: src=%+v dst=%+v", src, dst)
	}
	t.Logf("smoke: src ws=%d (%s), dst ws=%d (%s)", src.Number, src.RawName, dst.Number, dst.RawName)

	// 2. Build a minimal ManagedEnvironment that maps both workspaces.
	srcID := w.WorkspaceID(fmt.Sprintf("smoke-src-%d", src.Number))
	dstID := w.WorkspaceID(fmt.Sprintf("smoke-dst-%d", dst.Number))
	env := w.ManagedEnvironment{
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{
				{ID: srcID, RawName: src.RawName, DisplayName: src.DisplayName, Role: w.WorkspaceGeneral},
				{ID: dstID, RawName: dst.RawName, DisplayName: dst.DisplayName, Role: w.WorkspaceGeneral},
			},
		},
		Apps: w.AppEnvironment{
			ManagedApps: []w.ManagedAppPolicy{
				{BundleID: "com.mitchellh.ghostty", AppPath: "/Applications/Ghostty.app"},
			},
		},
		WindowManager: w.WindowManagerEnvironment{
			Layout: w.LayoutTuning{MaxVisibleColumns: 4, MaxWindowsPerColumn: 1},
		},
	}

	sig := wm.NewSigWM(env, nil, nil) // real omniwmctl + real `open`

	// 3. Spawn Ghostty with unique controller-owned title.
	// Title must match OmniWM's app-rule regex `^(ai|shell|ai-view)-[0-9]+:`
	// (legacy modules/darwin/omniwm/app-rules.nix) — otherwise OmniWM marks
	// the window unmanaged and `query windows` filters it.
	title := fmt.Sprintf("shell-99:projwm-next-smoke-%d", time.Now().UnixNano())
	t.Logf("smoke: spawning Ghostty title=%q on src=%s", title, srcID)
	req := wm.SpawnRequest{
		Kind:      w.WindowShell,
		BundleID:  "com.mitchellh.ghostty",
		AppPath:   "/Applications/Ghostty.app",
		Title:     title,
		Workspace: srcID,
		// `sleep 600` keeps the window open without a shell that would
		// rewrite the terminal title via escape sequences.
		ExtraArgs: []string{"--working-directory=/tmp", "-e", "sleep", "600"},
	}

	// 4. Cleanup MUST run — close any Ghostty window matching our title prefix.
	t.Cleanup(func() {
		closeGhosttyByTitle(t, title)
	})

	live, err := sig.Spawn(rootCtx, req)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	t.Logf("smoke: spawned LiveWindowID=%s", live)

	// 5. Verify it's on the src workspace (Spawn already moved it there).
	verifyOnWorkspace(t, rootCtx, string(live), src.Number, "after spawn")

	// 6. Move to dst.
	t.Logf("smoke: moving %s to dst ws=%d", live, dst.Number)
	if err := sig.MoveWindowToWorkspace(rootCtx, live, dstID); err != nil {
		t.Fatalf("MoveWindowToWorkspace: %v", err)
	}
	verifyOnWorkspace(t, rootCtx, string(live), dst.Number, "after move")

	// 7. FocusWorkspace src to leave the user back where they started.
	if err := sig.FocusWorkspace(rootCtx, srcID); err != nil {
		t.Logf("FocusWorkspace src cleanup (non-fatal): %v", err)
	}

	// 8. Observe end-to-end: ensure SigWM.Observe sees the moved window.
	obs, err := sig.Observe(rootCtx)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	got, ok := obs.Windows[live]
	if !ok {
		t.Fatalf("Observe missing window %s", live)
	}
	if got.Workspace != dstID {
		t.Fatalf("Observe: window workspace = %q, want %q", got.Workspace, dstID)
	}
	t.Logf("smoke: PASS — Observe confirms %s on %s", live, dstID)
}

type ctlWS struct {
	Number      int    `json:"number"`
	RawName     string `json:"rawName"`
	DisplayName string `json:"displayName"`
	IsCurrent   bool   `json:"isCurrent"`
}

func mustQueryWorkspaces(t *testing.T, ctx context.Context) []ctlWS {
	t.Helper()
	out, err := exec.CommandContext(ctx, "omniwmctl", "query", "workspaces", "--format", "json").Output()
	if err != nil {
		t.Fatalf("omniwmctl query workspaces: %v", err)
	}
	var env struct {
		OK     bool `json:"ok"`
		Result struct {
			Payload struct {
				Workspaces []ctlWS `json:"workspaces"`
			} `json:"payload"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("decode workspaces: %v", err)
	}
	return env.Result.Payload.Workspaces
}

func verifyOnWorkspace(t *testing.T, ctx context.Context, windowID string, expectedNum int, phase string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		out, err := exec.CommandContext(ctx, "omniwmctl", "query", "windows", "--format", "json").Output()
		if err != nil {
			t.Fatalf("query windows (%s): %v", phase, err)
		}
		var env struct {
			Result struct {
				Payload struct {
					Windows []struct {
						ID        string `json:"id"`
						Workspace struct {
							Number int `json:"number"`
						} `json:"workspace"`
					} `json:"windows"`
				} `json:"payload"`
			} `json:"result"`
		}
		if err := json.Unmarshal(out, &env); err != nil {
			t.Fatalf("decode windows (%s): %v", phase, err)
		}
		for _, win := range env.Result.Payload.Windows {
			if win.ID == windowID {
				if win.Workspace.Number == expectedNum {
					return
				}
				break
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("verifyOnWorkspace(%s): window %s did not reach ws=%d", phase, windowID, expectedNum)
}

// closeGhosttyByTitle uses AppleScript to close any Ghostty window whose name
// contains the given title prefix. SigWM.Close is policy-blocked, so test
// cleanup runs out-of-band.
func closeGhosttyByTitle(t *testing.T, titleSubstr string) {
	t.Helper()
	script := fmt.Sprintf(`tell application "Ghostty"
	repeat with w in (every window whose name contains "%s")
		try
			close w
		end try
	end repeat
end tell`, titleSubstr)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		t.Logf("cleanup (osascript close ghostty): %v / %s", err, strings.TrimSpace(string(out)))
	} else {
		t.Logf("cleanup: closed ghostty windows matching %q", titleSubstr)
	}
}
