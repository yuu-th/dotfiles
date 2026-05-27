//go:build integration

package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"

	"github.com/yuu-th/projwm-next/internal/event"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/ipc"
	"github.com/yuu-th/projwm-next/internal/scenario"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestHumanE2ESSOTUnarchiveProjectParkStateSteps(t *testing.T) {
	h := newHumanE2E(t)
	h.reconcileIdeal()

	archivedMatchers := []e2eWindowMatcher{
		{Title: "ai-1:projwm-test-main"},
		{Title: "shell-1:projwm-test-main"},
		{Title: "shell-2:projwm-test-main"},
		{Title: "ai-view-1:projwm-test-main"},
	}
	h.run("archive", "projwm-test-main")
	waitForManagedGhosttyMissing(t, h.ctx, archivedMatchers)

	out, err := h.runOutput("unarchive", "projwm-test-main")
	if err != nil {
		failAcceptance(t, scenario.FailNotImplemented, "SSOT-S3/unarchive-park-cli",
			fmt.Sprintf("SSOT §4.5/§9.1 requires unarchive to return a project to park state without a target slot; projwmctl rejected `unarchive dotfiles`: %v\n%s", err, out))
	}
	desired := readCurrentDesiredWorld(t, h.storeDir)
	project := desired.Projects["projwm-test-main"]
	if project.Archived {
		failAcceptance(t, scenario.FailInvariant, "SSOT-S3/project-archived", "unarchive left dotfiles archived")
	}
	active := desired.Profiles[desired.ActiveProfile]
	for slot, projectID := range active.Assignments {
		if projectID == "projwm-test-main" {
			failAcceptance(t, scenario.FailInvariant, "SSOT-S3/park-state",
				fmt.Sprintf("unarchive assigned dotfiles to slot %s; SSOT requires park state with no slot assignment", slot))
		}
	}
	h.run("reconcile")
	waitForManagedGhosttyMissing(t, h.ctx, archivedMatchers)
	assertFullInvariantAudit(t, h, "INV.1-INV.13/SSOT-S3")
}

func TestHumanE2ESSOTMacOSRestartRecoverySteps(t *testing.T) {
	h := newHumanE2E(t)
	h.reconcileIdeal()
	before := currentDesiredWorldKey(t, h.storeDir)

	h.stopDaemon()
	cleanupIdealResidue(t, h.ctx)
	h.daemon, h.daemonStderr = startHumanDaemon(t, h.ctx, h.bins.projwmd, h.manifestPath, h.manifestDigest, h.storeDir, h.privatePayloadDir, h.socketPath, h.provenancePath)
	assertStartupProvenance(t, h)
	waitForAllIdealSlots(t, h.ctx, time.Minute)

	after := currentDesiredWorldKey(t, h.storeDir)
	if before != after {
		failAcceptance(t, scenario.FailInvariant, "SSOT-S6/desired-authority",
			fmt.Sprintf("macOS-restart recovery changed DesiredWorld\nbefore: %s\nafter:  %s", before, after))
	}
	assertFullInvariantAudit(t, h, "INV.1-INV.13/SSOT-S6")
}

func TestHumanE2ESSOTOmniWMRestartRecoverySteps(t *testing.T) {
	h := newHumanE2E(t)
	h.reconcileIdeal()
	before := currentDesiredWorldKey(t, h.storeDir)
	beforeSessions := tmuxListSessions(t)

	label := fmt.Sprintf("gui/%d/org.nixos.omniwm", os.Getuid())
	out, err := exec.CommandContext(h.ctx, "launchctl", "kickstart", "-k", label).CombinedOutput()
	if err != nil {
		failAcceptance(t, scenario.FailUnsafeToRun, "SSOT-S7/omniwm-restart",
			fmt.Sprintf("launchctl kickstart -k %s failed: %v\n%s", label, err, out))
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := exec.CommandContext(h.ctx, "omniwmctl", "query", "windows", "--format", "json").CombinedOutput(); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	h.sendEvent(event.KindSafetyTimer, event.SourceTimer)
	waitForAllIdealSlots(t, h.ctx, 90*time.Second)

	after := currentDesiredWorldKey(t, h.storeDir)
	if before != after {
		failAcceptance(t, scenario.FailInvariant, "SSOT-S7/desired-authority",
			fmt.Sprintf("OmniWM restart recovery changed DesiredWorld\nbefore: %s\nafter:  %s", before, after))
	}
	afterSessions := tmuxListSessions(t)
	for _, name := range []string{"ai-1/projwm-test-main", "shell-1/projwm-test-main", "shell-2/projwm-test-main"} {
		if slices.Contains(beforeSessions, name) && !slices.Contains(afterSessions, name) {
			failAcceptance(t, scenario.FailInvariant, "SSOT-S7/tmux-survives",
				fmt.Sprintf("OmniWM restart must reuse live tmux session %q; before=%v after=%v", name, beforeSessions, afterSessions))
		}
	}
	assertFullInvariantAudit(t, h, "INV.1-INV.13/SSOT-S7")
}

func TestHumanE2ESSOTSummonIdempotencySteps(t *testing.T) {
	h := newHumanE2E(t)
	h.reconcileIdeal()
	before := countWindowsByTitleBundleWorkspace(t, h.ctx, "shell-1:projwm-test-main", "com.mitchellh.ghostty", "Q")
	if before != 1 {
		failAcceptance(t, scenario.FailFixtureInvalid, "SSOT-S8/precondition",
			fmt.Sprintf("expected one shell-1:projwm-test-main before summon, got %d", before))
	}

	for i := 0; i < 3; i++ {
		out, err := h.runOutput("summon-shell", "Q")
		if err != nil {
			failAcceptance(t, scenario.FailNotImplemented, "SSOT-S8/summon-shell-intent",
				fmt.Sprintf("SSOT §2.3/§9.1 requires summon-shell identity reuse; projwmctl rejected summon-shell Q on iteration %d: %v\n%s", i+1, err, out))
		}
	}
	after := countWindowsByTitleBundleWorkspace(t, h.ctx, "shell-1:projwm-test-main", "com.mitchellh.ghostty", "Q")
	if after != 1 {
		failAcceptance(t, scenario.FailInvariant, "SSOT-S8/no-duplicates",
			fmt.Sprintf("summon-shell duplicated shell-1:projwm-test-main: before=%d after=%d", before, after))
	}
	assertFullInvariantAudit(t, h, "INV.1-INV.13/SSOT-S8")
}

func TestHumanE2ESSOTJumpOperationsFocusExpectedWindowsSteps(t *testing.T) {
	h := newHumanE2E(t)
	h.reconcileIdeal()

	cases := []struct {
		step      string
		args      []string
		title     string
		bundleID  string
		workspace string
	}{
		{step: "SSOT-OP01/shell-jump", args: []string{"summon-shell", "Q"}, title: "shell-1:projwm-test-main", bundleID: "com.mitchellh.ghostty", workspace: "Q"},
		{step: "SSOT-OP02/editor-jump", args: []string{"summon-editor", "Q"}, title: "projwm-test-main", bundleID: "dev.zed.Zed", workspace: "Q"},
		{step: "SSOT-OP03/browser-jump", args: []string{"summon-browser", "Q"}, title: "browser-1:projwm-test-main", bundleID: "com.vivaldi.Vivaldi", workspace: "Q"},
		{step: "SSOT-OP06/viewer-jump", args: []string{"summon-viewer"}, title: "ai-view-1:projwm-test-main", bundleID: "com.mitchellh.ghostty", workspace: "A"},
	}
	for _, tc := range cases {
		out, err := h.runOutput(tc.args...)
		if err != nil {
			failAcceptance(t, scenario.FailNotImplemented, tc.step,
				fmt.Sprintf("SSOT §4.1 requires user operation %v to focus/reuse %s on workspace %s; command failed: %v\n%s", tc.args, tc.title, tc.workspace, err, out))
		}
		win := waitForFocusedWindowTitleBundle(t, h.ctx, tc.title, tc.bundleID, 30*time.Second)
		if win.Workspace != tc.workspace {
			failAcceptance(t, scenario.FailInvariant, tc.step,
				fmt.Sprintf("focused %s on workspace %s, want %s: %+v", tc.title, win.Workspace, tc.workspace, win))
		}
	}
	assertFullInvariantAudit(t, h, "INV.1-INV.13/SSOT-OP01-03-06")
}

func TestHumanE2ESSOTProjectSwitchAndSameSlotWindowSwitchSteps(t *testing.T) {
	h := newHumanE2E(t)
	h.reconcileIdeal()

	wShell := liveWindowByTitle(t, h.ctx, "W", "shell-1:projwm-test-alt")
	runOmni(t, h.ctx, "window", "focus", wShell.ID)
	waitForFocusedLiveWindowID(t, h.ctx, wShell.ID, 5*time.Second)
	qShell := liveWindowByTitle(t, h.ctx, "Q", "shell-1:projwm-test-main")
	runOmni(t, h.ctx, "window", "focus", qShell.ID)
	waitForFocusedLiveWindowID(t, h.ctx, qShell.ID, 5*time.Second)

	if _, err := h.submitRawIntent("switch-project", json.RawMessage(`{"slot":"W"}`)); err != nil {
		failAcceptance(t, scenario.FailNotImplemented, "SSOT-OP04/switch-project-intent",
			fmt.Sprintf("SSOT §4.1 operation 4 requires switch-project intent with target slot payload; daemon rejected it: %v", err))
	}
	focusedW := waitForFocusedWindowTitleBundle(t, h.ctx, "shell-1:projwm-test-alt", "com.mitchellh.ghostty", 30*time.Second)
	if focusedW.Workspace != "W" {
		failAcceptance(t, scenario.FailInvariant, "SSOT-OP04/final-focus",
			fmt.Sprintf("switch-project focused workspace %s, want W: %+v", focusedW.Workspace, focusedW))
	}

	runOmni(t, h.ctx, "window", "focus", qShell.ID)
	waitForFocusedLiveWindowID(t, h.ctx, qShell.ID, 5*time.Second)
	if _, err := h.submitRawIntent("cycle-slot-window", json.RawMessage(`{"slot":"Q","kind":"editor"}`)); err != nil {
		failAcceptance(t, scenario.FailNotImplemented, "SSOT-OP05/cycle-slot-window-intent",
			fmt.Sprintf("SSOT §4.1 operation 5 requires cycle-slot-window intent without workspace change; daemon rejected it: %v", err))
	}
	focusedEditor := waitForFocusedWindowTitleBundle(t, h.ctx, "projwm-test-main", "dev.zed.Zed", 30*time.Second)
	if focusedEditor.Workspace != "Q" {
		failAcceptance(t, scenario.FailInvariant, "SSOT-OP05/same-workspace",
			fmt.Sprintf("cycle-slot-window focused workspace %s, want Q: %+v", focusedEditor.Workspace, focusedEditor))
	}
	assertFullInvariantAudit(t, h, "INV.1-INV.13/SSOT-OP04-05")
}

func TestHumanE2ESSOTScratchShellOpenCloseSteps(t *testing.T) {
	h := newHumanE2E(t)
	h.reconcileIdeal()

	prior := liveWindowByTitle(t, h.ctx, "Q", "shell-1:projwm-test-main")
	runOmni(t, h.ctx, "window", "focus", prior.ID)
	waitForFocusedLiveWindowID(t, h.ctx, prior.ID, 5*time.Second)

	if _, err := h.submitRawIntent("show-scratch-shell", json.RawMessage(`{}`)); err != nil {
		failAcceptance(t, scenario.FailNotImplemented, "SSOT-OP11/show-scratch-shell-intent",
			fmt.Sprintf("SSOT §4.1 operation 11 requires a shortcut-routed daemon intent `show-scratch-shell`; daemon rejected it: %v", err))
	}
	scratch := waitForFocusedWindowTitleBundle(t, h.ctx, "projwm-scratch-shell", "com.mitchellh.ghostty", 30*time.Second)
	if got := countWindowsByTitleBundle(t, h.ctx, "projwm-scratch-shell", "com.mitchellh.ghostty"); got != 1 {
		failAcceptance(t, scenario.FailInvariant, "SSOT-OP11/no-duplicate-scratch",
			fmt.Sprintf("scratch shell count after show = %d, want 1", got))
	}

	if _, err := h.submitRawIntent("show-scratch-shell", json.RawMessage(`{}`)); err != nil {
		failAcceptance(t, scenario.FailNotImplemented, "SSOT-OP11/show-scratch-shell-idempotent-intent",
			fmt.Sprintf("second `show-scratch-shell` must be accepted as an idempotent user operation: %v", err))
	}
	if got := countWindowsByTitleBundle(t, h.ctx, "projwm-scratch-shell", "com.mitchellh.ghostty"); got != 1 {
		failAcceptance(t, scenario.FailInvariant, "SSOT-OP11/show-idempotency",
			fmt.Sprintf("second show duplicated scratch shell: count=%d", got))
	}
	waitForFocusedLiveWindowID(t, h.ctx, scratch.ID, 5*time.Second)

	if _, err := h.submitRawIntent("hide-scratch-shell", json.RawMessage(`{}`)); err != nil {
		failAcceptance(t, scenario.FailNotImplemented, "SSOT-OP11/hide-scratch-shell-intent",
			fmt.Sprintf("SSOT §4.1 operation 11 requires `hide-scratch-shell` to restore prior focus; daemon rejected it: %v", err))
	}
	waitForFocusedLiveWindowID(t, h.ctx, prior.ID, 10*time.Second)
	assertFullInvariantAudit(t, h, "INV.1-INV.13/SSOT-OP11")
}

func TestHumanE2ESSOTProjectAddFirstFreeSlotSteps(t *testing.T) {
	h := newHumanE2E(t)
	h.reconcileIdeal()
	h.run("unassign", "E")

	projectID := w.ProjectID("ssot-add-project")
	out, err := h.runOutput("up", "--ai", "claude", "--cwd", t.TempDir(), "--as", string(projectID))
	if err != nil {
		failAcceptance(t, scenario.FailNotImplemented, "SSOT-OP09/project-add-first-free-slot",
			fmt.Sprintf("SSOT §4.1 operation 9 requires project add to allocate the first free slot without a user-supplied slot; command failed: %v\n%s", err, out))
	}

	shell := waitForFocusedWindowTitleBundle(t, h.ctx, "shell-1:"+string(projectID), "com.mitchellh.ghostty", 90*time.Second)
	if shell.Workspace != "E" {
		failAcceptance(t, scenario.FailInvariant, "SSOT-OP09/first-free-slot",
			fmt.Sprintf("new project shell appeared on workspace %s, want first free slot E: %+v", shell.Workspace, shell))
	}
	desired := readCurrentDesiredWorld(t, h.storeDir)
	active := desired.Profiles[desired.ActiveProfile]
	if active.Assignments["E"] != projectID {
		failAcceptance(t, scenario.FailInvariant, "SSOT-OP09/desired-assignment",
			fmt.Sprintf("active profile E assignment = %s, want %s", active.Assignments["E"], projectID))
	}
	if !desiredProjectHasWindow(desired, projectID, w.WindowShell, 1) {
		failAcceptance(t, scenario.FailInvariant, "SSOT-OP09/default-shell",
			fmt.Sprintf("created project %s does not contain shell-1 DesiredWindow", projectID))
	}
	assertFullInvariantAudit(t, h, "INV.1-INV.13/SSOT-OP09")
}

func TestHumanE2ESSOTAddWindowCreatesNextShellAndPlacesItSteps(t *testing.T) {
	h := newHumanE2E(t)
	h.reconcileIdeal()

	out, err := h.runOutput("add-shell", "--project", "projwm-test-main")
	if err != nil {
		failAcceptance(t, scenario.FailNotImplemented, "SSOT-OP12/add-shell",
			fmt.Sprintf("SSOT §4.1 operation 12 requires add-shell to append the next shell window and place it in the active project slot: %v\n%s", err, out))
	}
	waitForWindowTitleInWorkspace(t, h.ctx, "shell-3:projwm-test-main", "Q", 90*time.Second)
	desired := readCurrentDesiredWorld(t, h.storeDir)
	if !desiredProjectHasWindow(desired, "projwm-test-main", w.WindowShell, 3) {
		failAcceptance(t, scenario.FailInvariant, "SSOT-OP12/desired-window",
			"add-shell did not append shell-3 DesiredWindow to dotfiles")
	}
	if !desiredProjectHasWindow(desired, "projwm-test-main", w.WindowShell, 1) || !desiredProjectHasWindow(desired, "projwm-test-main", w.WindowShell, 2) {
		failAcceptance(t, scenario.FailInvariant, "SSOT-OP12/no-id-rewrite",
			"add-shell rewrote or removed existing shell-1/shell-2 identities")
	}
	assertFullInvariantAudit(t, h, "INV.1-INV.13/SSOT-OP12")
}

func TestHumanE2ESSOTRemoveWindowClosesAndKillsSessionSteps(t *testing.T) {
	h := newHumanE2E(t)
	h.reconcileIdeal()

	out, err := h.runOutput("remove", "--window", "shell-2", "--project", "projwm-test-main")
	if err != nil {
		failAcceptance(t, scenario.FailNotImplemented, "SSOT-OP13/remove-window",
			fmt.Sprintf("SSOT §4.1 operation 13 requires remove-window to remove desired identity, close the window, and kill its tmux session: %v\n%s", err, out))
	}
	waitForWorkspaceMissing(t, h.ctx, "Q", []e2eWindowMatcher{{Title: "shell-2:projwm-test-main"}})
	if sessions := tmuxListSessions(t); slices.Contains(sessions, "shell-2/projwm-test-main") {
		failAcceptance(t, scenario.FailInvariant, "SSOT-OP13/tmux-session-killed",
			fmt.Sprintf("remove-window left shell-2/projwm-test-main tmux session alive: %v", sessions))
	}
	desired := readCurrentDesiredWorld(t, h.storeDir)
	if desiredProjectHasWindow(desired, "projwm-test-main", w.WindowShell, 2) {
		failAcceptance(t, scenario.FailInvariant, "SSOT-OP13/desired-window-removed",
			"remove-window left shell-2 DesiredWindow in dotfiles")
	}
	assertFullInvariantAudit(t, h, "INV.1-INV.13/SSOT-OP13")
}

func TestHumanE2ESSOTBrowserTabOperationsUpdatePrivateMetadataSteps(t *testing.T) {
	h := newHumanE2E(t)
	h.reconcileIdeal()

	before := desiredBrowserSession(t, readCurrentDesiredWorld(t, h.storeDir), "projwm-test-main")
	if _, err := h.submitRawIntent("browser-add-tab", json.RawMessage(`{"project":"projwm-test-main","window":"browser-1","url":"https://example.com/ssot-op14"}`)); err != nil {
		failAcceptance(t, scenario.FailNotImplemented, "SSOT-OP14/browser-add-tab",
			fmt.Sprintf("SSOT §4.1 operation 14 requires browser-add-tab to append a tab through private payload metadata: %v", err))
	}
	afterAdd := desiredBrowserSession(t, readCurrentDesiredWorld(t, h.storeDir), "projwm-test-main")
	if afterAdd.URLCount != before.URLCount+1 || slices.Equal(afterAdd.URLPayloadRefs, before.URLPayloadRefs) {
		failAcceptance(t, scenario.FailInvariant, "SSOT-OP14/browser-add-tab-metadata",
			fmt.Sprintf("browser-add-tab metadata before=%+v after=%+v", before, afterAdd))
	}

	if _, err := h.submitRawIntent("browser-change-tab-url", json.RawMessage(`{"project":"projwm-test-main","window":"browser-1","tab":1,"url":"https://example.com/ssot-op16"}`)); err != nil {
		failAcceptance(t, scenario.FailNotImplemented, "SSOT-OP16/browser-change-tab-url",
			fmt.Sprintf("SSOT §4.1 operation 16 requires browser-change-tab-url to update private payload metadata: %v", err))
	}
	afterChange := desiredBrowserSession(t, readCurrentDesiredWorld(t, h.storeDir), "projwm-test-main")
	if afterChange.URLCount != afterAdd.URLCount || slices.Equal(afterChange.URLPayloadRefs, afterAdd.URLPayloadRefs) {
		failAcceptance(t, scenario.FailInvariant, "SSOT-OP16/browser-change-tab-url-metadata",
			fmt.Sprintf("browser-change-tab-url metadata before=%+v after=%+v", afterAdd, afterChange))
	}

	if _, err := h.submitRawIntent("browser-reorder-tabs", json.RawMessage(`{"project":"projwm-test-main","window":"browser-1","from":2,"to":1}`)); err != nil {
		failAcceptance(t, scenario.FailNotImplemented, "SSOT-OP17/browser-reorder-tabs",
			fmt.Sprintf("SSOT §4.1 operation 17 requires browser-reorder-tabs to update private payload metadata: %v", err))
	}
	afterReorder := desiredBrowserSession(t, readCurrentDesiredWorld(t, h.storeDir), "projwm-test-main")
	if afterReorder.URLCount != afterChange.URLCount || slices.Equal(afterReorder.URLPayloadRefs, afterChange.URLPayloadRefs) {
		failAcceptance(t, scenario.FailInvariant, "SSOT-OP17/browser-reorder-tabs-metadata",
			fmt.Sprintf("browser-reorder-tabs metadata before=%+v after=%+v", afterChange, afterReorder))
	}

	if _, err := h.submitRawIntent("browser-remove-tab", json.RawMessage(`{"project":"projwm-test-main","window":"browser-1","tab":2}`)); err != nil {
		failAcceptance(t, scenario.FailNotImplemented, "SSOT-OP15/browser-remove-tab",
			fmt.Sprintf("SSOT §4.1 operation 15 requires browser-remove-tab to remove a tab through private payload metadata: %v", err))
	}
	afterRemove := desiredBrowserSession(t, readCurrentDesiredWorld(t, h.storeDir), "projwm-test-main")
	if afterRemove.URLCount != afterReorder.URLCount-1 || slices.Equal(afterRemove.URLPayloadRefs, afterReorder.URLPayloadRefs) {
		failAcceptance(t, scenario.FailInvariant, "SSOT-OP15/browser-remove-tab-metadata",
			fmt.Sprintf("browser-remove-tab metadata before=%+v after=%+v", afterReorder, afterRemove))
	}
	assertFullInvariantAudit(t, h, "INV.1-INV.13/SSOT-OP14-17")
}

func TestHumanE2ESSOTCrashRecoverySteps(t *testing.T) {
	h := newHumanE2E(t)
	h.reconcileIdeal()
	before := currentDesiredWorldKey(t, h.storeDir)

	victim := liveWindowByTitle(t, h.ctx, "Q", "shell-1:projwm-test-main")
	terminateLiveWindowProcess(t, victim)
	waitForWorkspaceMissing(t, h.ctx, "Q", []e2eWindowMatcher{{Title: "shell-1:projwm-test-main"}})
	h.sendEvent(event.KindWindowsChanged, event.SourceWindowMgr)
	waitForLayout(t, h.ctx, "Q", humanIdealSlots["Q"], 90*time.Second)

	if after := currentDesiredWorldKey(t, h.storeDir); before != after {
		failAcceptance(t, scenario.FailInvariant, "SSOT-S10/ghostty-desired-authority",
			fmt.Sprintf("Ghostty crash recovery changed DesiredWorld\nbefore: %s\nafter:  %s", before, after))
	}

	// SSOT §8.9 tmux-server crash: managed tmux sessions vanish; the
	// transaction loop recreates them (INV-03) and Ghostty reconnects — the
	// windows are NOT respawned. Isolated to the test project's OWN sessions
	// (never the user's tmux server): we kill only the projwm-test-main
	// sessions by name.
	managedSessions := []string{"ai-1/projwm-test-main", "shell-1/projwm-test-main", "shell-2/projwm-test-main"}
	present := map[string]bool{}
	for _, s := range tmuxListSessions(t) {
		present[s] = true
	}
	var killable []string
	for _, s := range managedSessions {
		if present[s] {
			killable = append(killable, s)
		}
	}
	if len(killable) == 0 {
		failAcceptance(t, scenario.FailFixtureInvalid, "SSOT-S10/tmux-precondition",
			fmt.Sprintf("expected managed tmux sessions before crash; have %v", tmuxListSessions(t)))
	}
	for _, s := range killable {
		killTmuxSession(t, s)
	}
	h.sendEvent(event.KindWindowsChanged, event.SourceWindowMgr)
	waitForTmuxSessions(t, killable, 30*time.Second)
	waitForLayout(t, h.ctx, "Q", humanIdealSlots["Q"], 90*time.Second)
	if after := currentDesiredWorldKey(t, h.storeDir); before != after {
		failAcceptance(t, scenario.FailInvariant, "SSOT-S10/tmux-desired-authority",
			fmt.Sprintf("tmux crash recovery changed DesiredWorld\nbefore: %s\nafter:  %s", before, after))
	}
	assertFullInvariantAudit(t, h, "INV.1-INV.13/SSOT-S10-tmux")

	// SSOT §8.9 Zed crash: SAFETY BOUNDARY — Zed is single-process (GPUI
	// ignores --user-data-dir; the managed Zed windows live in the user's
	// own zed process, observed pid is shared). terminateLiveWindowProcess on
	// a managed Zed window would SIGTERM the user's editor and lose unsaved
	// work. A real Zed-crash fixture therefore requires a dedicated session
	// with no user Zed running; it is intentionally NOT exercised here.
	failAcceptance(t, scenario.FailNotImplemented, "SSOT-S10/zed-crash-needs-clean-env",
		"SSOT §9.1 S10 Zed-crash recovery is unsafe to exercise while the user's Zed is running (Zed is single-process; killing the managed window's process would kill the user's editor). Requires a dedicated clean session.")
}

// killTmuxSession kills a single tmux session by exact name (test project
// sessions only — never the user's tmux server).
func killTmuxSession(t *testing.T, name string) {
	t.Helper()
	_ = exec.Command("tmux", "kill-session", "-t", name).Run()
}

// waitForTmuxSessions polls until every wanted session name is present again,
// proving the transaction loop recreated the crashed sessions (SSOT §8.9 /
// INV-03).
func waitForTmuxSessions(t *testing.T, want []string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		have := map[string]bool{}
		for _, s := range tmuxListSessions(t) {
			have[s] = true
		}
		all := true
		for _, w := range want {
			if !have[w] {
				all = false
				break
			}
		}
		if all {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	failAcceptance(t, scenario.FailInvariant, "SSOT-S10/tmux-recreate",
		fmt.Sprintf("tmux sessions not recreated within %s: want %v have %v", timeout, want, tmuxListSessions(t)))
}

func (h *humanE2E) submitRawIntent(kind string, payload json.RawMessage) (ipc.IntentResponse, error) {
	h.t.Helper()
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	conn, err := (&net.Dialer{}).DialContext(h.ctx, "unix", h.socketPath)
	if err != nil {
		return ipc.IntentResponse{}, fmt.Errorf("dial daemon: %w", err)
	}
	defer conn.Close()

	hello, err := ipc.NewEnvelope(ipc.MsgHello, ipc.Hello{
		ProtocolVersion:    ipc.ProtocolVersion,
		StoreSchemaVersion: ipc.StoreSchemaVersion,
		ManifestDigest:     h.manifestDigest,
		ClientName:         "human-e2e-raw-intent",
	})
	if err != nil {
		return ipc.IntentResponse{}, err
	}
	if err := ipc.WriteEnvelope(conn, hello); err != nil {
		return ipc.IntentResponse{}, err
	}
	welcome, err := ipc.ReadEnvelope(conn)
	if err != nil {
		return ipc.IntentResponse{}, err
	}
	if welcome.Type != ipc.MsgWelcome {
		return ipc.IntentResponse{}, fmt.Errorf("expected welcome, got %s", welcome.Type)
	}
	req, err := ipc.NewEnvelope(ipc.MsgIntentRequest, ipc.IntentRequest{
		RequestID: fmt.Sprintf("human-e2e-%d", time.Now().UnixNano()),
		Kind:      intent.Kind(kind),
		Payload:   payload,
	})
	if err != nil {
		return ipc.IntentResponse{}, err
	}
	if err := ipc.WriteEnvelope(conn, req); err != nil {
		return ipc.IntentResponse{}, err
	}
	rawResp, err := ipc.ReadEnvelope(conn)
	if err != nil {
		return ipc.IntentResponse{}, err
	}
	if rawResp.Type != ipc.MsgIntentResponse {
		return ipc.IntentResponse{}, fmt.Errorf("expected intent-response, got %s", rawResp.Type)
	}
	var resp ipc.IntentResponse
	if err := json.Unmarshal(rawResp.Payload, &resp); err != nil {
		return ipc.IntentResponse{}, err
	}
	if resp.Error != nil {
		return resp, resp.Error
	}
	return resp, nil
}

func observedFocusedLiveWindowID(t *testing.T, ctx context.Context) string {
	t.Helper()
	out := runOmniOutput(t, ctx, "query", "focused-window", "--format", "json")
	var env omniEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		failAcceptance(t, scenario.FailObservabilityGap, "SSOT-OP11/focus", fmt.Sprintf("decode focused-window envelope: %v", err))
	}
	if !env.OK {
		failAcceptance(t, scenario.FailObservabilityGap, "SSOT-OP11/focus", fmt.Sprintf("omniwmctl not ok: %s", env.Error))
	}
	var payload struct {
		Window struct {
			ID string `json:"id"`
		} `json:"window"`
	}
	if err := json.Unmarshal(env.Result.Payload, &payload); err != nil {
		failAcceptance(t, scenario.FailObservabilityGap, "SSOT-OP11/focus", fmt.Sprintf("decode focused-window payload: %v", err))
	}
	return payload.Window.ID
}

func waitForFocusedLiveWindowID(t *testing.T, ctx context.Context, id string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if observedFocusedLiveWindowID(t, ctx) == id {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	failAcceptance(t, scenario.FailInvariant, "SSOT-OP11/focus-restore",
		fmt.Sprintf("focused window did not become %s within %s; windows: %s", id, timeout, dumpWindows(queryAllWindows(t, ctx))))
}

func waitForFocusedWindowTitleBundle(t *testing.T, ctx context.Context, title, bundleID string, timeout time.Duration) e2eLiveWindow {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		focusedID := observedFocusedLiveWindowID(t, ctx)
		for _, win := range queryAllWindows(t, ctx) {
			if win.ID == focusedID && win.Title == title && win.BundleID == bundleID {
				return win
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	failAcceptance(t, scenario.FailInvariant, "SSOT-OP11/focus-scratch",
		fmt.Sprintf("focused window did not become %s/%s within %s; windows: %s", bundleID, title, timeout, dumpWindows(queryAllWindows(t, ctx))))
	return e2eLiveWindow{}
}

func countWindowsByTitleBundle(t *testing.T, ctx context.Context, title, bundleID string) int {
	t.Helper()
	count := 0
	for _, win := range queryAllWindows(t, ctx) {
		if win.Title == title && win.BundleID == bundleID {
			count++
		}
	}
	return count
}

func desiredProjectHasWindow(desired w.DesiredWorld, projectID w.ProjectID, kind w.WindowKind, index int) bool {
	project, ok := desired.Projects[projectID]
	if !ok {
		return false
	}
	for _, win := range project.Windows {
		if win.ID.Project == projectID && win.ID.Kind == kind && win.ID.Index == index {
			return true
		}
	}
	return false
}

func desiredBrowserSession(t *testing.T, desired w.DesiredWorld, projectID w.ProjectID) w.DesiredBrowserSession {
	t.Helper()
	project, ok := desired.Projects[projectID]
	if !ok {
		failAcceptance(t, scenario.FailInvariant, "SSOT/browser-session", fmt.Sprintf("project %s not found", projectID))
	}
	for _, win := range project.Windows {
		if win.Kind == w.WindowBrowser {
			if win.Browser == nil {
				failAcceptance(t, scenario.FailInvariant, "SSOT/browser-session", fmt.Sprintf("browser window for %s has nil Browser session", projectID))
			}
			return *win.Browser
		}
	}
	failAcceptance(t, scenario.FailInvariant, "SSOT/browser-session", fmt.Sprintf("project %s has no browser window", projectID))
	return w.DesiredBrowserSession{}
}

func assertProjectParkedInActiveProfile(t *testing.T, desired w.DesiredWorld, projectID w.ProjectID) {
	t.Helper()
	active := desired.Profiles[desired.ActiveProfile]
	for slot, got := range active.Assignments {
		if got == projectID {
			failAcceptance(t, scenario.FailInvariant, "SSOT/project-park-state",
				fmt.Sprintf("project %s is assigned to slot %s, want parked", projectID, slot))
		}
	}
}
