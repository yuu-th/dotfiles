//go:build real_ops

package wm

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// L3 real-machine tests for the remaining SSOT §6.9.1 empty-project edge cases
// (ATTR-D2 / ATTR-D4). These complement the F1/A1 real_ops scaffold and the
// owner of the positive empty-project cleanup contract,
// TestSpawnEditorEmptyProjectCleanup (ATTR-D1 / §10.4 spawn S4). Where D1 proves
// that a NEW spurious empty-project window IS closed, D2 proves the inverse
// safety property: a pre-existing (user) empty-project window must be left
// untouched by a later managed spawn's cleanup. Both run on real OmniWM + real
// Zed and are gated identically to F1/A1:
//   - PROJWM_REAL_OP_TESTS=1 + Zed.app present (via realSpecContext +
//     realSpecRequireAppBundle).
//   - a Zed-safety precondition (ATTR-F2 analog): we only pre-create our own
//     empty-project Zed window in a controlled session where NO other Zed main
//     process is running, so the pre-existing window we model is provably ours
//     to clean up and a buggy implementation can never take down the user's
//     editing Zed.
//
// They are the verification scaffold ("if these pass, the attribution
// implementation is robust"); running is deferred to a safe session.

// attrEmptyProjectWindowIDs returns the live IDs of every observed Zed window
// that the implementation's empty-project heuristic would target — bundle
// dev.zed.Zed with a blank or "empty project"/"untitled" title. Mirrors the
// impl predicate in closeNewZedEmptyProjects (sigwm.go) so the test reasons
// about the exact population the cleanup acts on.
func attrEmptyProjectWindowIDs(t *testing.T, ctx context.Context, sw *SigWM) map[w.LiveWindowID]struct{} {
	t.Helper()
	obs, err := sw.Observe(ctx)
	if err != nil {
		t.Fatalf("Observe (empty-project scan): %v", err)
	}
	out := map[w.LiveWindowID]struct{}{}
	for id, win := range obs.Windows {
		if win.App.BundleID != "dev.zed.Zed" {
			continue
		}
		title := win.Title.Value
		if title == "" || strings.EqualFold(title, "empty project") || strings.EqualFold(title, "untitled") {
			out[id] = struct{}{}
		}
	}
	return out
}

// attrWaitEmptyProjectAppears polls until at least one new Zed empty-project
// window (not in baseline) is observed, returning the set of new IDs. Used to
// confirm our bare-Zed pre-create produced an observable empty-project window
// to model the "pre-existing" case.
func attrWaitEmptyProjectAppears(t *testing.T, ctx context.Context, sw *SigWM, baseline map[w.LiveWindowID]struct{}, timeout time.Duration) map[w.LiveWindowID]struct{} {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		cur := attrEmptyProjectWindowIDs(t, ctx, sw)
		fresh := map[w.LiveWindowID]struct{}{}
		for id := range cur {
			if _, was := baseline[id]; !was {
				fresh[id] = struct{}{}
			}
		}
		if len(fresh) > 0 {
			return fresh
		}
		if time.Now().After(deadline) {
			return nil
		}
		select {
		case <-ctx.Done():
			t.Fatalf("context expired waiting for empty-project window: %v", ctx.Err())
		case <-time.After(300 * time.Millisecond):
		}
	}
}

// ATTR-D2: a Zed "empty project" window present BEFORE a managed Zed spawn is a
// pre-existing (user-owned model) window and must NOT be closed by that spawn's
// auxiliary cleanup — only NEW empty-project windows the spawn itself produced
// are eligible. The user-experience guarantee: the window you already had open
// is still there after projwm spawns its managed editor.
//
// We model the pre-existing window by launching a bare Zed into an ISOLATED
// --user-data-dir (so it opens an empty-project/welcome window without touching
// the user's Zed state), and we only do so in a controlled session with no
// other Zed main process running (the ATTR-F2 safety analog), so the window is
// provably ours and the experiment can never endanger a user's editing Zed.
func TestZedAttr_D2_PreExistingEmptyProjectUntouched(t *testing.T) {
	ctx, cancel := realSpecContext(t, 150*time.Second)
	defer cancel()
	realSpecRequireAppBundle(t, "/Applications/Zed.app")

	// Safety gate (ATTR-F2 analog): only proceed when no Zed main process is
	// already running. Zed is single-instance, so a bare launch while the user
	// has Zed open would attach to THEIR process; refusing here guarantees the
	// pre-existing empty-project window we create is exclusively ours.
	if n := attrZedMainProcCount(t); n != 0 {
		t.Skipf("ATTR-D2: %d Zed main process(es) already running; refusing to pre-create an empty-project window in a shared Zed (single-process safety, ATTR-F2)", n)
	}

	sw := newRealSigWM()

	// Baseline empty-project population (should be empty given the gate, but
	// diff defensively against anything omniwm already catalogs).
	baseline := attrEmptyProjectWindowIDs(t, ctx, sw)

	// Pre-create the "pre-existing" empty-project window: a bare Zed launch
	// (no project path) into an isolated data dir. This is the window D2 says
	// must survive the later managed spawn's cleanup.
	preDataDir := t.TempDir()
	if err := exec.CommandContext(ctx, "open", "-na", "/Applications/Zed.app", "--args", "-n", "--user-data-dir", preDataDir).Run(); err != nil {
		t.Fatalf("ATTR-D2: pre-create bare Zed empty-project window: %v", err)
	}
	pre := attrWaitEmptyProjectAppears(t, ctx, sw, baseline, 30*time.Second)
	if len(pre) == 0 {
		t.Skip("ATTR-D2: bare Zed did not register an observable empty-project window within the settle window (omniwm registration race); cannot model the pre-existing case this run")
	}
	// Teardown: close every window we created, even on failure. Pre-existing
	// empty-project windows we made are ours to AXClose.
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for id := range pre {
			_ = sw.unsafeCloseForDiagnostics(ctx, id)
		}
	})

	// Act: spawn a managed editor. This triggers closeNewZedEmptyProjects,
	// which must scope its close to NEW windows only.
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
		t.Fatalf("ATTR-D2: spawn managed editor: %v", err)
	}
	if live != "" {
		realSpecAssertObserved(t, ctx, sw, live, "8", "dev.zed.Zed", title)
	}

	// Guarantee: every pre-existing empty-project window is STILL observed.
	// (The spawn's cleanup may have closed its OWN spurious empty-project
	// window — that is D1's job — but it must not have touched ours.)
	after := attrEmptyProjectWindowIDs(t, ctx, sw)
	for id := range pre {
		if _, ok := after[id]; !ok {
			t.Fatalf("ATTR-D2: pre-existing empty-project window %q was closed by the managed spawn's cleanup; only NEW windows may be closed (pre-existing/user windows are inviolable)", id)
		}
	}
}

// ATTR-D3 (delayed empty-project backstop): the empty-project cleanup
// (closeNewZedEmptyProjects) runs once, synchronously, inside Spawn's settle
// window. But Zed can register a spurious empty-project window LATE — after that
// grace window has already closed — so the cleanup will miss it. The contract is
// the SECOND line of defense: a stray that slips past cleanup must not break the
// managed layout. ReorderColumns operates on the managed set with
// managed-relative indexing (sigwm.go scopedManagedColumns / managedRelativeIndex,
// SSOT §6.3 "layout concerns only managed windows"), so an interleaved stray is
// transparent and the managed windows still settle into their requested relative
// order.
//
// Relationship to TestReorderColumnsToleratesStrayWindow (reorder_stray_window_test.go):
// that test proves the SAME managed-relative reorder principle, but spawns all
// windows (managed + stray) UP FRONT before the first/only reorder. D3 is the
// distinct TEMPORAL framing the contract names: the stray appears AFTER managed
// columns are already established and verified, modeling the "spurious window
// arrives after the cleanup grace window" race. It then re-runs the reorder to
// the managed layout and asserts convergence despite the late arrival. The
// underlying tolerance is the existing managed-relative ReorderColumns — D3 adds
// no new impl, only the delayed-arrival scenario coverage.
//
// The stray here is a controller-titled ghostty (not a Zed window): a stray's
// transparency to reorder is bundle-agnostic (it is just an unmanaged window on
// the workspace), and using ghostty keeps the test Zed-safe — no managed Zed is
// spawned, so the single-process kill hazard never arises. We still assert the
// Zed-safety precondition (attrZedMainProcCount==0) so that if a buggy reorder
// were ever to escalate to a process action it could not reach a user's editing
// Zed; this also documents D3 as a member of the Zed-safety-gated ATTR family.
func TestZedAttr_D3_DelayedStrayDoesNotBreakReorder(t *testing.T) {
	ctx, cancel := realSpecContext(t, 180*time.Second)
	defer cancel()
	realSpecRequireGhostty(t)

	// Zed-safety precondition (ATTR family gate): refuse to run while a managed
	// Zed could be reachable. No managed Zed is spawned here, but D3 belongs to
	// the §6.9.1 ATTR family and is gated identically so the safety posture is
	// uniform and a future reorder regression cannot endanger a user's Zed.
	if n := attrZedMainProcCount(t); n != 0 {
		t.Skipf("ATTR-D3: %d Zed main process(es) already running; refusing to run a layout-mutating reorder test in a session with a live Zed (single-process safety, ATTR-F2)", n)
	}

	sw := newRealSigWM()

	type win struct{ title, session string }
	managedDefs := []win{
		{realSpecTitle(t, "shell", 1, "d3-managed-a"), realSpecSession(t, "d3-managed-a")},
		{realSpecTitle(t, "shell", 2, "d3-managed-b"), realSpecSession(t, "d3-managed-b")},
		{realSpecTitle(t, "shell", 3, "d3-managed-c"), realSpecSession(t, "d3-managed-c")},
	}
	strayDef := win{realSpecTitle(t, "shell", 4, "d3-stray-late"), realSpecSession(t, "d3-stray-late")}
	for _, d := range managedDefs {
		realSpecCleanupGhostty(t, sw, d.title, d.session)
	}
	realSpecCleanupGhostty(t, sw, strayDef.title, strayDef.session)

	// Establish the managed set FIRST (no stray present yet).
	managed := make([]w.LiveWindowID, 0, len(managedDefs))
	for _, d := range managedDefs {
		id := realSpecSpawnGhostty(t, ctx, sw, w.WindowShell, "8", d.title, d.session, "")
		realSpecAssertObserved(t, ctx, sw, id, "8", "com.mitchellh.ghostty", d.title)
		managed = append(managed, id)
	}
	initial := realSpecObservedOrder(t, ctx, sw, "8", managed...)
	if len(initial) != len(managedDefs) {
		t.Fatalf("ATTR-D3 setup managed order = %v, want %d windows", initial, len(managedDefs))
	}

	// Establish & VERIFY a managed column layout before the stray exists, so the
	// "delayed" arrival is unambiguous: the layout was settled, then perturbed.
	established := [][]w.LiveWindowID{{initial[2]}, {initial[1]}, {initial[0]}}
	if err := sw.ReorderColumns(ctx, "8", established); err != nil {
		t.Fatalf("ATTR-D3: establish managed layout before stray: %v", err)
	}
	realSpecAssertColumns(t, ctx, sw, "8", established)

	// NOW introduce the stray — AFTER the managed columns are established and the
	// (modeled) cleanup grace window has long since closed. This is the
	// delayed-empty-project race the D3 contract names.
	stray := realSpecSpawnGhostty(t, ctx, sw, w.WindowShell, "8", strayDef.title, strayDef.session, "")
	realSpecAssertObserved(t, ctx, sw, stray, "8", "com.mitchellh.ghostty", strayDef.title)

	// Re-run the reorder to a DIFFERENT managed layout. The stray is intentionally
	// omitted from want; managed-relative reorder must converge the managed three
	// regardless of where the late stray sits among them.
	want := [][]w.LiveWindowID{{initial[1]}, {initial[0]}, {initial[2]}}
	if err := sw.ReorderColumns(ctx, "8", want); err != nil {
		t.Fatalf("ATTR-D3: reorder with delayed stray present: %v", err)
	}
	realSpecAssertColumns(t, ctx, sw, "8", want)
}
