//go:build real_ops

package wm

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// L3 real-machine tests for SSOT §6.9.1 attribution (ATTR-*). These are the
// AUTHORITATIVE guarantees: the Fake-WM (L2) layer proves the controller logic
// but cannot prove the real single-process Zed experience (a Fake green is false
// confidence). These run on real OmniWM + real Zed and are gated:
//   - PROJWM_REAL_OP_TESTS=1 + Zed.app present
//   - a Zed-safety precondition: they only proceed in a controlled session so a
//     buggy implementation can never take down the user's editing Zed (the
//     pid-99057 kill incident this whole policy exists to prevent).
//
// They are written now as the verification scaffold ("if these pass, the
// attribution implementation is robust"); running is deferred to a safe session.

// attrZedMainProcCount counts Zed MAIN processes (excluding the always-present
// --crash-handler subprocess). Used as a safety gate and the F1 assertion.
func attrZedMainProcCount(t *testing.T) int {
	t.Helper()
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

// ATTR-F1: removing a managed Zed window must NOT kill the Zed process (Zed is
// single-process; killing it would take the user's editing windows with it).
// Removal goes through the managed AXClose path, never a process kill.
//
// Authoritative guarantee for SSOT §4.1 "Zed アプリ自体は kill しない".
func TestZedAttr_F1_RemoveManagedWindowKeepsProcessAlive(t *testing.T) {
	ctx, cancel := realSpecContext(t, 120*time.Second)
	defer cancel()
	realSpecRequireAppBundle(t, "/Applications/Zed.app")
	sw := newRealSigWM()

	// Safety gate: a managed editor we spawn will share the single Zed process
	// with any pre-existing (user) Zed. We must observe the process COUNT does
	// not drop when we remove our window. Record the baseline.
	beforeProcs := attrZedMainProcCount(t)

	projectPath := t.TempDir()
	title := filepath.Base(projectPath)
	realSpecCleanupTitle(t, sw, title)

	live, err := sw.Spawn(ctx, SpawnRequest{
		Workspace:   "8",
		Kind:        w.WindowEditor,
		Desired:     w.DesiredWindowID{Project: "projwm-attr", Kind: w.WindowEditor, Index: 1},
		Title:       title,
		BundleID:    "dev.zed.Zed",
		ProjectPath: projectPath,
	})
	if err != nil {
		t.Fatalf("spawn managed editor: %v", err)
	}
	realSpecAssertObserved(t, ctx, sw, live, "8", "dev.zed.Zed", title)

	// Remove our managed window (AXClose, never process kill).
	if err := sw.TerminateManagedAppInstance(ctx, TerminateManagedAppInstanceRequest{
		LiveWindow: live,
		Desired:    w.DesiredWindowID{Project: "projwm-attr", Kind: w.WindowEditor, Index: 1},
		Kind:       w.WindowEditor,
		Title:      title,
	}); err != nil {
		t.Fatalf("terminate managed editor window: %v", err)
	}
	realSpecWaitMissing(t, ctx, sw, live, 30*time.Second)

	// Guarantee: the Zed process did not die. If a user Zed was running
	// (beforeProcs>=1) the count must not drop below it; if ours was the only
	// process it may have exited cleanly, but it must never have been KILLED
	// while a user process existed.
	afterProcs := attrZedMainProcCount(t)
	if beforeProcs >= 1 && afterProcs < beforeProcs {
		t.Fatalf("ATTR-F1: Zed main process count dropped %d→%d after removing a managed window; a managed-window removal must never kill the (shared) Zed process", beforeProcs, afterProcs)
	}
}

// ATTR-A1 (real provenance capture): spawning a managed editor yields a live
// window ID that is stably observable afterwards — the basis for provenance
// ownership (we remember the ID we spawned). Re-observation must find the same
// live ID with the expected bundle/title.
func TestZedAttr_A1_SpawnYieldsStableObservableID(t *testing.T) {
	ctx, cancel := realSpecContext(t, 120*time.Second)
	defer cancel()
	realSpecRequireAppBundle(t, "/Applications/Zed.app")
	sw := newRealSigWM()

	projectPath := t.TempDir()
	title := filepath.Base(projectPath)
	realSpecCleanupTitle(t, sw, title)

	live, err := sw.Spawn(ctx, SpawnRequest{
		Workspace:   "8",
		Kind:        w.WindowEditor,
		Desired:     w.DesiredWindowID{Project: "projwm-attr", Kind: w.WindowEditor, Index: 1},
		Title:       title,
		BundleID:    "dev.zed.Zed",
		ProjectPath: projectPath,
	})
	if err != nil {
		t.Fatalf("spawn managed editor: %v", err)
	}
	if live == "" {
		t.Fatal("ATTR-A1: spawn returned empty live window ID; provenance needs a capturable ID")
	}
	// Re-observe: the same ID must still resolve to our managed editor.
	realSpecAssertObserved(t, ctx, sw, live, "8", "dev.zed.Zed", title)
	realSpecCleanupTitle(t, sw, title)
	// NOTE: empty-project cleanup (ATTR-D1) is NOT re-tested here — it is owned
	// by ZED-CONFIG's TestSpawnEditorEmptyProjectCleanup (SSOT §10.4 spawn S4).
}
