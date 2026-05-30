//go:build integration

package scenarios

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yuu-th/projwm-next/internal/event"
	"github.com/yuu-th/projwm-next/internal/scenario"
)

// AUTHORITATIVE user-experience layer for SSOT §6.9 / §6.9.1 (ATTR-*) window
// attribution. These run on the real daemon -> projwmctl -> OmniWM/sigwm ->
// real Zed stack (the humanE2E harness) and are the load-bearing guarantee:
// the L0/L2 Fake-WM tests prove the controller logic, but only a real
// single-process Zed run can prove the user's editing windows are safe (a Fake
// green is false confidence — see internal/adapter/wm/ssot_attr_real_ops_test.go).
//
// Each test maps 1:1 to an ATTR-<ID> row of §6.9.1. They are GATED two ways:
//
//  1. newHumanE2E SKIPs unless PROJWM_NEXT_REAL_ACCEPTANCE=1 (like every other
//     real test in this package).
//  2. CRITICAL Zed-safety precondition: requireSoleTestZed asserts that no Zed
//     MAIN process is running BEFORE the harness spawns anything. Zed is a
//     single-instance app (GPUI): every window — the managed editor AND any
//     "user" window this test opens — lives in ONE shared process. If a user
//     Zed were already editing, a buggy attribution impl could AXClose/kill a
//     window in that shared process and destroy the user's work (the pid-99057
//     kill incident this whole policy exists to prevent). So if ANY Zed is
//     already running we SKIP: the only Zed that may exist during these tests is
//     the test's own. Cleanup is always AXClose (Cmd-W), NEVER a process kill.
//
// ISO names: the managed project is "projwm-test-main" (already an isolated
// test fixture, not a real user project — see projectDotfiles). The simulated
// "user" window is opened against a temp folder whose basename deliberately
// COLLIDES with the managed editor title to exercise title ambiguity, while
// never colliding with any real user project on disk.

const (
	attrZedAppPath = "/Applications/Zed.app"
	attrZedBundle  = "dev.zed.Zed"
	// attrManagedEditorTitle is the title the harness's managed Zed editor for
	// project projwm-test-main carries on slot Q (Zed title == basename(cwd) ==
	// the project id). See projectDotfiles / desiredWindow.
	attrManagedEditorTitle = "projwm-test-main"
	// attrUserWorkspace is a non-slot (user) workspace, i.e. outside the managed
	// A/Q/W/E slots — the same external workspace EVT.4.5 uses for an untouched
	// external app. A window here is the user's own and must be inviolable.
	attrUserWorkspace = "3"
)

// attrZedMainProcCount counts Zed MAIN processes (excluding the always-present
// --crash-handler subprocess). Mirrors the L3 real_ops helper of the same name.
func attrZedMainProcCount() int {
	out, _ := exec.Command("pgrep", "-fl", "Zed.app/Contents/MacOS/zed").Output()
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" || strings.Contains(line, "--crash-handler") {
			continue
		}
		n++
	}
	return n
}

// requireSoleTestZed is the safety gate. It must be called at the very TOP of
// every ATTR Zed test, BEFORE newHumanE2E spawns the managed editor. It SKIPs
// (never fails) when a Zed process is already running so a buggy attribution
// impl can never act on the user's live editing session. It also requires
// Zed.app to be installed (otherwise the managed editor can never spawn).
func requireSoleTestZed(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(attrZedAppPath); err != nil {
		t.Skipf("ATTR: %s is required for the real Zed-attribution gate: %v", attrZedAppPath, err)
	}
	if n := attrZedMainProcCount(); n != 0 {
		t.Skipf("ATTR Zed-safety precondition not met: %d Zed main process(es) already running; this test only runs when the ONLY Zed is the test's own (a user editing Zed must never be at risk)", n)
	}
}

// spawnUserZedWindow opens a simulated USER Zed window on the user (non-slot)
// workspace, pointed at a temp folder whose basename forces the window title.
// Because Zed is single-instance the new window joins the SAME process as the
// managed editor (so PIDs coincide); the window is distinguished by its unique
// live window ID and its workspace. The window is registered for AXClose
// cleanup (Cmd-W, never a process kill). The returned window is freshly
// re-queried so its workspace is authoritative.
func spawnUserZedWindow(t *testing.T, ctx context.Context, basename string) e2eLiveWindow {
	t.Helper()
	// A temp project dir whose basename becomes the Zed window title.
	parent := t.TempDir()
	projectDir := filepath.Join(parent, basename)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		failAcceptance(t, scenario.FailUnsafeToRun, "ATTR/user-zed-spawn",
			fmt.Sprintf("create user project dir %q: %v", projectDir, err))
	}

	// Open the user window on the user workspace so it never lands in a managed
	// slot. We focus the user workspace first; a freshly-opened window appears on
	// the focused workspace.
	focusWorkspaceByName(t, ctx, attrUserWorkspace)

	before := map[string]bool{}
	for _, win := range queryAllWindows(t, ctx) {
		before[win.ID] = true
	}

	// `open -na Zed.app <folder>` opens a new window in the (single-instance) Zed
	// process titled with the folder basename. This is the user's own launch
	// path — NOT the managed `zed -n --user-data-dir` path.
	if err := exec.CommandContext(ctx, "open", "-na", attrZedAppPath, projectDir).Run(); err != nil {
		failAcceptance(t, scenario.FailUnsafeToRun, "ATTR/user-zed-spawn",
			fmt.Sprintf("open user Zed window for %q: %v", projectDir, err))
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		for _, win := range queryAllWindows(t, ctx) {
			if before[win.ID] || win.BundleID != attrZedBundle {
				continue
			}
			// Zed shows the basename (possibly "<file> — <basename>"); accept any
			// title that carries the basename so we tolerate Zed's title styling.
			if win.PID <= 0 || !strings.Contains(win.Title, basename) {
				continue
			}
			found := liveWindowByID(t, ctx, win.ID)
			t.Cleanup(func() {
				// AXClose only — never kill the shared Zed process.
				for _, w := range queryAllWindowsBestEffort(ctx) {
					if w.ID == found.ID {
						userCloseLiveWindowViaAX(t, ctx, found)
						return
					}
				}
			})
			// If Zed placed it somewhere other than the user workspace, relocate
			// it there so the assertion below has a stable "user territory".
			if found.Workspace != attrUserWorkspace {
				moveLiveWindowToWorkspace(t, ctx, found.ID, attrUserWorkspace)
				waitForWindowTitleInWorkspace(t, ctx, found.Title, attrUserWorkspace, 15*time.Second)
				found = liveWindowByID(t, ctx, found.ID)
			}
			return found
		}
		time.Sleep(300 * time.Millisecond)
	}
	failAcceptance(t, scenario.FailUnsafeToRun, "ATTR/user-zed-spawn",
		fmt.Sprintf("user Zed window with basename %q did not appear", basename))
	return e2eLiveWindow{}
}

// attrAssertUserWindowUntouched re-observes a previously-captured user window
// and fails if it was moved off the user workspace, lost its identity, or was
// closed. This is the user-experience guarantee the ATTR contract protects.
func attrAssertUserWindowUntouched(t *testing.T, ctx context.Context, step string, before e2eLiveWindow) {
	t.Helper()
	var after *e2eLiveWindow
	for _, win := range queryAllWindows(t, ctx) {
		if win.ID == before.ID {
			w := win
			after = &w
			break
		}
	}
	if after == nil {
		failAcceptance(t, scenario.FailInvariant, step,
			fmt.Sprintf("user's own Zed window (id=%s title=%q) was CLOSED by projwm; non-provenance windows must never be touched", before.ID, before.Title))
	}
	if after.Workspace != attrUserWorkspace {
		failAcceptance(t, scenario.FailInvariant, step,
			fmt.Sprintf("user's own Zed window (id=%s title=%q) was MOVED %s->%s by projwm; user windows are inviolable", before.ID, before.Title, before.Workspace, after.Workspace))
	}
	if after.PID != before.PID {
		failAcceptance(t, scenario.FailInvariant, step,
			fmt.Sprintf("user's own Zed window (id=%s) changed pid %d->%d; it must be the same untouched window", before.ID, before.PID, after.PID))
	}
}

// attrManagedEditorOnSlotQ returns the managed Zed editor window on slot Q (the
// one projwm spawned for project projwm-test-main), failing if it is absent or
// ambiguous. The managed editor must always be projwm's own window in its slot.
func attrManagedEditorOnSlotQ(t *testing.T, ctx context.Context, step string) e2eLiveWindow {
	t.Helper()
	var matches []e2eLiveWindow
	for _, win := range windowsInWorkspace(t, ctx, "Q") {
		if win.BundleID == attrZedBundle && win.Title == attrManagedEditorTitle {
			matches = append(matches, win)
		}
	}
	if len(matches) != 1 {
		failAcceptance(t, scenario.FailInvariant, step,
			fmt.Sprintf("expected exactly one managed Zed editor %q on slot Q, got %d: %s",
				attrManagedEditorTitle, len(matches), dumpWindows(windowsInWorkspace(t, ctx, "Q"))))
	}
	return matches[0]
}

// TestZedAttr_B3_UserWindowOnOwnWorkspaceUntouched — ATTR-B3.
// Cold-start: a same-basename user Zed window already sits on the user's own
// (non-slot) workspace BEFORE projwm reconciles. projwm must NOT adopt it (not
// move it to the slot, not close it); instead it must spawn its OWN managed
// editor into slot Q. User territory is inviolable.
func TestZedAttr_B3_UserWindowOnOwnWorkspaceUntouched(t *testing.T) {
	requireSoleTestZed(t)
	h := newHumanE2E(t)

	// User opens their own colliding-title Zed on their own workspace FIRST
	// (cold start: projwm has not reconciled / has no provenance yet).
	userWin := spawnUserZedWindow(t, h.ctx, attrManagedEditorTitle)

	// projwm now reconciles to ideal state — it must spawn its own editor on Q.
	h.reconcileIdeal()

	// Guarantee 1: the user's window is untouched (same ws, same pid, still alive).
	attrAssertUserWindowUntouched(t, h.ctx, "ATTR-B3/user-untouched", userWin)
	// Guarantee 2: projwm spawned its OWN managed editor into slot Q.
	attrManagedEditorOnSlotQ(t, h.ctx, "ATTR-B3/managed-in-slot")
	assertFullInvariantAudit(t, h, "INV.1-INV.13/ATTR-B3")
}

// TestZedAttr_B1_UserOpensSameNameAfterOwnership — ATTR-B1.
// After projwm owns its managed editor in slot Q, the user opens a same-title
// Zed window on their own workspace. That window must stay External — never
// adopted, moved, or closed by projwm's subsequent reconcile.
func TestZedAttr_B1_UserOpensSameNameAfterOwnership(t *testing.T) {
	requireSoleTestZed(t)
	h := newHumanE2E(t)

	// projwm establishes ownership of its managed editor first.
	h.reconcileIdeal()
	attrManagedEditorOnSlotQ(t, h.ctx, "ATTR-B1/ownership-established")

	// THEN the user opens their own colliding-title Zed on their workspace.
	userWin := spawnUserZedWindow(t, h.ctx, attrManagedEditorTitle)

	// Drive projwm to re-observe/reconcile after the collision appeared.
	h.sendEvent(event.KindWindowsChanged, event.SourceWindowMgr)
	h.reconcileIdeal()

	// Guarantee: the user's window was not adopted/moved/closed.
	attrAssertUserWindowUntouched(t, h.ctx, "ATTR-B1/user-untouched", userWin)
	// And projwm's own managed editor is still its own window in slot Q.
	attrManagedEditorOnSlotQ(t, h.ctx, "ATTR-B1/managed-still-ours")
	assertFullInvariantAudit(t, h, "INV.1-INV.13/ATTR-B1")
}

// TestZedAttr_A2_ManagedEditorStaysOursUnderTitleCollision — ATTR-A2.
// provenance precedence at the user-experience level: once a same-title user
// window collides, the managed editor identity must remain bound to projwm's
// OWN spawned window in slot Q (resolution must not flip to the user's window
// nor collapse to ambiguous-and-abandon). Observable: exactly one managed
// editor on Q, distinct live window from the user's, and the user's untouched.
func TestZedAttr_A2_ManagedEditorStaysOursUnderTitleCollision(t *testing.T) {
	requireSoleTestZed(t)
	h := newHumanE2E(t)

	h.reconcileIdeal()
	ours := attrManagedEditorOnSlotQ(t, h.ctx, "ATTR-A2/before-collision")

	// User opens a colliding same-title Zed on their own workspace.
	userWin := spawnUserZedWindow(t, h.ctx, attrManagedEditorTitle)
	if userWin.ID == ours.ID {
		failAcceptance(t, scenario.FailFixtureInvalid, "ATTR-A2/distinct-windows",
			fmt.Sprintf("user window and managed editor share live id %s; the collision fixture is invalid", ours.ID))
	}

	h.sendEvent(event.KindWindowsChanged, event.SourceWindowMgr)
	h.reconcileIdeal()

	// Guarantee 1: the managed editor on Q is STILL projwm's original window.
	stillOurs := attrManagedEditorOnSlotQ(t, h.ctx, "ATTR-A2/after-collision")
	if stillOurs.ID != ours.ID {
		failAcceptance(t, scenario.FailInvariant, "ATTR-A2/identity-stable",
			fmt.Sprintf("managed editor on Q switched live id %s->%s under title collision; the user's window must never become the managed one", ours.ID, stillOurs.ID))
	}
	// Guarantee 2: the user's colliding window remains External and untouched.
	attrAssertUserWindowUntouched(t, h.ctx, "ATTR-A2/user-untouched", userWin)
	assertFullInvariantAudit(t, h, "INV.1-INV.13/ATTR-A2")
}

// TestZedAttr_F1_RemoveManagedWindowKeepsProcessAlive — ATTR-F1.
// Removing a managed Zed window (here via archive, the project-scoped lifecycle
// removal path) must close ONLY that window via AXClose — it must never kill the
// single shared Zed process, so a co-existing user Zed window and the process
// itself survive. Authoritative guarantee for SSOT §4.1 "Zed アプリ自体は kill
// しない".
func TestZedAttr_F1_RemoveManagedWindowKeepsProcessAlive(t *testing.T) {
	requireSoleTestZed(t)
	h := newHumanE2E(t)

	// Bring up the managed editor (Zed process now == 1, the test's own).
	h.reconcileIdeal()
	attrManagedEditorOnSlotQ(t, h.ctx, "ATTR-F1/managed-present")

	// A user Zed window co-exists in the SAME single Zed process on the user
	// workspace. If removing the managed window killed the process, this user
	// window (and the user's edits) would vanish — the exact harm ATTR-F1 forbids.
	userWin := spawnUserZedWindow(t, h.ctx, "projwm-attr-userdup")
	procsBefore := attrZedMainProcCount()
	if procsBefore < 1 {
		failAcceptance(t, scenario.FailFixtureInvalid, "ATTR-F1/baseline",
			"expected at least one Zed main process after spawning managed + user windows")
	}

	// Archive the managed project: projwm removes its managed Zed editor window
	// through the project-scoped lifecycle removal (AXClose-class), never kill.
	if _, err := h.runOutput("archive", "projwm-test-main"); err != nil {
		failAcceptance(t, scenario.FailNotImplemented, "ATTR-F1/archive",
			fmt.Sprintf("archive projwm-test-main did not complete: %v", err))
	}
	// The managed editor disappears from slot Q.
	waitForWorkspaceMissing(t, h.ctx, "Q", []e2eWindowMatcher{
		{Title: attrManagedEditorTitle, BundleID: attrZedBundle},
	})

	// Guarantee 1: the shared Zed process is still alive (never killed).
	if procsAfter := attrZedMainProcCount(); procsAfter < procsBefore {
		failAcceptance(t, scenario.FailInvariant, "ATTR-F1/process-alive",
			fmt.Sprintf("Zed main process count dropped %d->%d after removing a managed window; removal must AXClose only, never kill the (shared) process", procsBefore, procsAfter))
	}
	// Guarantee 2: the user's other Zed window survived untouched.
	attrAssertUserWindowUntouched(t, h.ctx, "ATTR-F1/user-window-survives", userWin)
	assertFullInvariantAudit(t, h, "INV.1-INV.13/ATTR-F1")
}
