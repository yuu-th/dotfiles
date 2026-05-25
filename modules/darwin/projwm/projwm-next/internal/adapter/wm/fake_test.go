package wm

import (
	"context"
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// TestFakeShowScratchShellIsIdempotent verifies SSOT §4.1 OP11 / §7.3 SCRATCH:
// ShowScratchShell must return the same LiveWindowID on repeated calls without
// creating a duplicate scratch window.
func TestFakeShowScratchShellIsIdempotent(t *testing.T) {
	f := NewFake(newTestEnv())
	ctx := context.Background()

	first, err := f.ShowScratchShell(ctx)
	if err != nil {
		t.Fatalf("first ShowScratchShell: %v", err)
	}
	if first == "" {
		t.Fatal("first ShowScratchShell returned empty id")
	}

	second, err := f.ShowScratchShell(ctx)
	if err != nil {
		t.Fatalf("second ShowScratchShell: %v", err)
	}
	if second != first {
		t.Fatalf("idempotency violated: first=%s second=%s", first, second)
	}

	// Count scratch windows in the observed world.
	obs, err := f.Observe(ctx)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	scratchCount := 0
	for _, win := range obs.Windows {
		if win.Title.Value == "projwm-scratch-shell" {
			scratchCount++
		}
	}
	if scratchCount != 1 {
		t.Fatalf("expected exactly 1 scratch window, got %d", scratchCount)
	}
}

// TestFakeShowScratchShellFocusesIt verifies that after ShowScratchShell,
// the observed focus is on the scratch window.
func TestFakeShowScratchShellFocusesIt(t *testing.T) {
	f := NewFake(newTestEnv())
	ctx := context.Background()

	id, err := f.ShowScratchShell(ctx)
	if err != nil {
		t.Fatalf("ShowScratchShell: %v", err)
	}
	obs, err := f.Observe(ctx)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if obs.Focus.Window != id {
		t.Fatalf("focus = %q, want scratch %q", obs.Focus.Window, id)
	}
}

// TestFakeHideScratchShellRestoresPriorFocus verifies SSOT §4.1 OP11:
// 非表示時に scratch 表示前の focused_window に戻る。
func TestFakeHideScratchShellRestoresPriorFocus(t *testing.T) {
	f := NewFake(newTestEnv())
	ctx := context.Background()

	// Spawn a prior "shell-1" window in workspace A and focus it.
	prior, err := f.Spawn(ctx, SpawnRequest{
		Workspace: "ws-A",
		Kind:      w.WindowShell,
		Desired:   w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1},
		Title:     "shell-1:p1",
		BundleID:  "com.mitchellh.ghostty",
	})
	if err != nil {
		t.Fatalf("Spawn prior: %v", err)
	}
	if err := f.FocusWindow(ctx, prior); err != nil {
		t.Fatalf("FocusWindow prior: %v", err)
	}

	// Show scratch — focus shifts to scratch.
	scratch, err := f.ShowScratchShell(ctx)
	if err != nil {
		t.Fatalf("ShowScratchShell: %v", err)
	}
	obs, _ := f.Observe(ctx)
	if obs.Focus.Window != scratch {
		t.Fatalf("after show, focus = %q want %q", obs.Focus.Window, scratch)
	}

	// Hide — focus must return to prior.
	if err := f.HideScratchShell(ctx, prior); err != nil {
		t.Fatalf("HideScratchShell: %v", err)
	}
	obs, _ = f.Observe(ctx)
	if obs.Focus.Window != prior {
		t.Fatalf("after hide, focus = %q want prior %q", obs.Focus.Window, prior)
	}
}

// TestFakeHideScratchShellEmptyPriorIsNoop verifies that HideScratchShell
// with an empty priorWindow does not change focus (NFR-15 規約と一致)。
func TestFakeHideScratchShellEmptyPriorIsNoop(t *testing.T) {
	f := NewFake(newTestEnv())
	ctx := context.Background()

	scratch, err := f.ShowScratchShell(ctx)
	if err != nil {
		t.Fatalf("ShowScratchShell: %v", err)
	}
	obsBefore, _ := f.Observe(ctx)
	if obsBefore.Focus.Window != scratch {
		t.Fatalf("setup: focus should be on scratch, got %q", obsBefore.Focus.Window)
	}
	if err := f.HideScratchShell(ctx, ""); err != nil {
		t.Fatalf("HideScratchShell empty: %v", err)
	}
	obsAfter, _ := f.Observe(ctx)
	if obsAfter.Focus.Window != scratch {
		t.Fatalf("focus changed when prior was empty: %q", obsAfter.Focus.Window)
	}
}

// TestFakeMoveCockpitToParkWorkspaceMovesWindow verifies SSOT §3.4 INV-06:
// MoveCockpitToParkWorkspace forces the cockpit window to the named park
// workspace.
func TestFakeMoveCockpitToParkWorkspaceMovesWindow(t *testing.T) {
	env := newTestEnv()
	// Add a CP1 workspace so the fake's layout map has the key.
	env.Workspaces.Workspaces = append(env.Workspaces.Workspaces,
		w.WorkspaceSpec{ID: "ws-CP1", RawName: "CP1", DisplayName: "CP1", Role: w.WorkspaceGeneral},
	)
	f := NewFake(env)
	ctx := context.Background()

	// Spawn a cockpit window on workspace A (wrong place — invariant violation).
	if err := f.SpawnCockpit(ctx, 0, "projwm-cockpit-0"); err != nil {
		t.Fatalf("SpawnCockpit: %v", err)
	}
	// Find the cockpit window id from Observe.
	obs, _ := f.Observe(ctx)
	var cockpitID w.LiveWindowID
	for id, win := range obs.Windows {
		if win.Title.Value == "projwm-cockpit-0" {
			cockpitID = id
			break
		}
	}
	if cockpitID == "" {
		t.Fatal("no cockpit window observed after SpawnCockpit")
	}

	// Move cockpit to CP1.
	if err := f.MoveCockpitToParkWorkspace(ctx, cockpitID, "CP1"); err != nil {
		t.Fatalf("MoveCockpitToParkWorkspace: %v", err)
	}
	obs2, _ := f.Observe(ctx)
	win, ok := obs2.Windows[cockpitID]
	if !ok {
		t.Fatalf("cockpit %s vanished after move", cockpitID)
	}
	if win.Workspace != "ws-CP1" {
		t.Fatalf("cockpit workspace = %q, want ws-CP1", win.Workspace)
	}
}

// TestFakeMoveCockpitToParkWorkspaceUnknownWindowErrors verifies that calling
// with a non-existent window id returns an error (don't silently no-op).
func TestFakeMoveCockpitToParkWorkspaceUnknownWindowErrors(t *testing.T) {
	f := NewFake(newTestEnv())
	ctx := context.Background()
	err := f.MoveCockpitToParkWorkspace(ctx, "no-such-window", "CP1")
	if err == nil {
		t.Fatal("expected error for unknown window id, got nil")
	}
}
