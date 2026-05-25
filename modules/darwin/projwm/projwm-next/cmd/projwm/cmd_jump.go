package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// cmdJump implements `projwm jump <TARGET>`.
//
// Resolves TARGET in 4 steps (T9 in design.md v3):
//  1. SLOT id (Q/W/E/R/T/Y/U/I/O/P/A) → workspace from manifest
//  2. PROFILE id → switch profile + focus active profile's "first" slot
//  3. PROJECT id → workspace of slot assigned to project in active profile
//  4. WORKSPACE id (A/M/B/1-9/E/...) → workspace direct
//
// Jump uses omniwmctl directly, bypassing projwmd (since it's pure focus,
// not a transaction). T9 also notes Profile-jump still requires the
// SwitchProfile intent for state, but the focus afterward is omniwm-only.
func cmdJump(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("jump: usage: projwm jump <TARGET>")
	}
	target := args[0]
	snap, err := loadSnapshotWithTimeout(gf, 3*time.Second)
	if err != nil {
		// If the store can't be loaded (daemon never ran), fall back to
		// treating the arg as a raw workspace ID. omniwmctl will reject
		// unknown workspaces with a clearer error than us.
		return jumpToWorkspace(stdout, stderr, w.WorkspaceID(target))
	}
	ws, kind, ok := resolveJumpTarget(snap, target)
	if !ok {
		// Last resort: treat as raw workspace id.
		return jumpToWorkspace(stdout, stderr, w.WorkspaceID(target))
	}
	if kind == "profile" {
		// Profile switch needs daemon; then focus first slot's workspace.
		if err := submitProfileSwitch(gf, w.ProfileID(target)); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "switched to profile %s\n", target)
	}
	return jumpToWorkspace(stdout, stderr, ws)
}

// resolveJumpTarget walks the 4-step resolution and reports the workspace.
// kind ∈ {"slot","profile","project","workspace"}.
func resolveJumpTarget(snap WorldSnapshot, target string) (w.WorkspaceID, string, bool) {
	// 1. SLOT
	if spec, ok := snap.Environment.SlotByID(w.SlotID(target)); ok {
		return spec.Workspace, "slot", true
	}
	// 2. PROFILE
	if _, ok := snap.Desired.Profiles[w.ProfileID(target)]; ok {
		// Use the profile's "first" slot in environment order, or fall back to viewer.
		prof := snap.Desired.Profiles[w.ProfileID(target)]
		for _, sid := range snap.Environment.SlotOrder() {
			if _, assigned := prof.Assignments[sid]; assigned {
				if spec, ok := snap.Environment.SlotByID(sid); ok {
					return spec.Workspace, "profile", true
				}
			}
		}
		return snap.Environment.Workspaces.Viewer, "profile", true
	}
	// 3. PROJECT
	if _, ok := snap.Desired.Projects[w.ProjectID(target)]; ok {
		if slot, ok := snap.Desired.ProjectAssignedSlot(w.ProjectID(target)); ok {
			if spec, ok := snap.Environment.SlotByID(slot); ok {
				return spec.Workspace, "project", true
			}
		}
	}
	// 4. WORKSPACE direct
	if _, ok := snap.Environment.WorkspaceByID(w.WorkspaceID(target)); ok {
		return w.WorkspaceID(target), "workspace", true
	}
	return "", "", false
}

func jumpToWorkspace(stdout, stderr io.Writer, ws w.WorkspaceID) error {
	cmd := exec.Command("omniwmctl", "workspace", "focus-name", string(ws))
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("jump: omniwmctl workspace focus-name %s: %w", ws, err)
	}
	return nil
}

func submitProfileSwitch(gf globalFlags, to w.ProfileID) error {
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := c.SubmitIntent(ctx, intent.SwitchProfile{To: to})
	return err
}
