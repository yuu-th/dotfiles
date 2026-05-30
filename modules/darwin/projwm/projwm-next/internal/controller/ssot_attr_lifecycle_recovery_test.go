package controller

import (
	"context"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// L2 behavioral tests for SSOT §6.9.1 attribution (ATTR-*) — the LIFECYCLE (E*),
// RECOVERY (G*) and CONCURRENT-SPAWN (D5) rows. Framed at the USER-EXPERIENCE /
// observable world-state boundary wherever possible (which managed editors exist,
// in which slot, untouched user windows, no duplicates); only where the row is
// fundamentally a mechanism (provenance persistence across a daemon restart) do
// we additionally assert state.Meta.WindowProvenance.
//
// Reuses the helpers from ssot_attr_test.go (same package): attrEnv,
// attrDesiredWithEditor, attrFindManagedEditor, attrEditorID.
//
// Honesty note (pre-implementation):
//   - The managed editor already spawns into its slot today (see ATTR-B3), so the
//     "editor appears in slot" half of E4/G2/G3 is guard-green; the provenance
//     half (E2/E3/E5 clearing, G1 persistence) is the true RED.

// attrCountEditorsInSlot counts live Zed windows on a given workspace.
func attrCountEditorsInSlot(obs w.ObservedWorld, ws w.WorkspaceID) int {
	n := 0
	for _, win := range obs.Windows {
		if win.App.BundleID == "dev.zed.Zed" && win.Workspace == ws {
			n++
		}
	}
	return n
}

// attrFindManagedEditorFor returns the live ID of the window matched to the
// editor identity of the named project (the package helper attrFindManagedEditor
// is hard-wired to "p1"; multi-project cases need this).
func attrFindManagedEditorFor(obs w.ObservedWorld, project w.ProjectID) (w.LiveWindowID, w.ObservedWindow, bool) {
	for id, win := range obs.Windows {
		if win.MatchedTo != nil && win.MatchedTo.Kind == w.WindowEditor && win.MatchedTo.Project == project {
			return id, win, true
		}
	}
	return "", w.ObservedWindow{}, false
}

// attrTwoSlotEnv builds a 2-slot environment (slots Q and R, plus a user
// workspace U) for the concurrent multi-spawn case (ATTR-D5).
func attrTwoSlotEnv() w.ManagedEnvironment {
	return w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Slots: []w.SlotSpec{
				{ID: "Q", Workspace: "Q", Order: 1},
				{ID: "R", Workspace: "R", Order: 2},
			},
			Workspaces: []w.WorkspaceSpec{
				{ID: "Q", RawName: "Q", DisplayName: "Q", Role: w.WorkspaceProject},
				{ID: "R", RawName: "R", DisplayName: "R", Role: w.WorkspaceProject},
				{ID: "U", RawName: "U", DisplayName: "U", Role: w.WorkspaceGeneral},
			},
		},
	}
}

// attrZedDesiredWindow builds a desired Zed editor window (index 1) whose Zed
// title is the project basename.
func attrZedDesiredWindow(project w.ProjectID) w.DesiredWindow {
	return w.DesiredWindow{
		ID:   w.DesiredWindowID{Project: project, Kind: w.WindowEditor, Index: 1},
		Kind: w.WindowEditor,
		App:  w.AppRequirement{BundleID: "dev.zed.Zed"},
		TitleContract: w.TitleContract{
			Authority: w.TitleAppOwned,
			Expected:  string(project),
			Drift:     w.TitleDriftRematch,
		},
		MatchHints: []w.MatchHint{
			{Kind: w.MatchByBundleID, Pattern: "dev.zed.Zed", Confidence: w.MatchStrong},
		},
	}
}

// ATTR-E1: remove-window (intent.RemoveWindow) drops the desired editor identity;
// the next converge closes its managed live window AND the provenance entry is
// cleared. Observable: no managed editor for p1 after the window is removed.
// RED today (no provenance clear-on-close logic).
func TestZedAttr_E1_RemoveWindowClearsProvenance(t *testing.T) {
	env := attrEnv()
	desired := attrDesiredWithEditor()
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))

	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (spawn editor): %v", err)
	}
	obs, _ := fake.Observe(context.Background())
	edLive, _, ok := attrFindManagedEditor(obs)
	if !ok {
		t.Fatalf("ATTR-E1: precondition failed — editor was not spawned before remove")
	}
	// Seed provenance for the spawned editor (same-package direct write, like G2).
	ctrl.state.Meta.WindowProvenance = map[w.DesiredWindowID]w.LiveWindowID{
		attrEditorID(): edLive,
	}

	// Remove the editor window (operation 13). The reducer drops the desired
	// window; the next converge MUST close the now-orphaned live window. This used
	// to be a provenance-INDEPENDENT gap (the planner protected the orphan as an
	// ambiguous candidate of the active identity, then PreUniqueStrong re-resolved
	// its stale MatchedTo against a target that no longer held it), so the close
	// silently failed. That gap is now fixed (provenance narrows the protected set
	// + the executor permits a removal close of a non-active-provenance window), so
	// this test asserts BOTH the close AND the provenance-clear contract.
	if _, err := ctrl.ApplyIntent(context.Background(), intent.RemoveWindow{Project: "p1", WindowID: attrEditorID()}); err != nil {
		t.Fatalf("ATTR-E1: remove-window: %v", err)
	}

	// Observable (un-tolerated now): the removed editor's live window is GONE.
	obs2, _ := fake.Observe(context.Background())
	if _, stillThere := obs2.Windows[edLive]; stillThere {
		t.Fatalf("ATTR-E1: removed editor's live window %q is still present after remove-window (must be closed)", edLive)
	}
	if _, _, ok := attrFindManagedEditor(obs2); ok {
		t.Fatalf("ATTR-E1: a managed editor for p1 still exists after remove-window (its window must be closed)")
	}

	// Mechanism: the editor identity is no longer in the desired set, so its
	// provenance entry MUST be cleared — asserted UNCONDITIONALLY.
	if got := ctrl.State().Meta.WindowProvenance[attrEditorID()]; got != "" {
		t.Fatalf("ATTR-E1: WindowProvenance[%v]=%q still set after remove-window dropped the desired identity (entry must be cleared)", attrEditorID(), got)
	}
}

// ATTR-A6: spawn dedup — an idempotent re-reconcile must NOT double-record nor
// drop provenance. After two reconciles exactly ONE editor exists and its
// provenance entry equals that single live ID (stable across the no-op replan).
// RED today: provenance is never captured, so the entry is empty after spawn.
func TestZedAttr_A6_SpawnDedupProvenanceStable(t *testing.T) {
	env := attrEnv()
	desired := attrDesiredWithEditor()
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))

	// Reconcile twice — the second pass is idempotent (editor already spawned).
	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (spawn editor, pass 1): %v", err)
	}
	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (idempotent, pass 2): %v", err)
	}

	obs, _ := fake.Observe(context.Background())
	// Exactly ONE editor window on slot Q (no duplicate from the re-reconcile).
	if n := attrCountEditorsInSlot(obs, "Q"); n != 1 {
		t.Fatalf("ATTR-A6: %d Zed windows on slot Q after re-reconcile, want exactly 1 (idempotent spawn)", n)
	}
	edLive, _, ok := attrFindManagedEditor(obs)
	if !ok {
		t.Fatalf("ATTR-A6: managed editor not found after re-reconcile")
	}
	// Mechanism (RED today): provenance records the single live ID — captured on
	// the first spawn and STABLE (not cleared, not duplicated) across the no-op
	// second reconcile.
	if got := ctrl.State().Meta.WindowProvenance[attrEditorID()]; got != edLive {
		t.Fatalf("ATTR-A6: WindowProvenance[%v]=%q after idempotent re-reconcile, want the single live editor %q (must record exactly once, stable)", attrEditorID(), got, edLive)
	}
}

// ATTR-E2: archive project → INV-04 closes ALL its windows AND the provenance
// entry is cleared. Observable: the managed editor for p1 is gone after archive.
func TestZedAttr_E2_ArchiveClearsManagedEditor(t *testing.T) {
	env := attrEnv()
	desired := attrDesiredWithEditor()
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))

	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (spawn editor): %v", err)
	}
	obs, _ := fake.Observe(context.Background())
	edLive, _, ok := attrFindManagedEditor(obs)
	if !ok {
		t.Fatalf("ATTR-E2: precondition failed — editor was not spawned before archive")
	}
	// Seed provenance for the spawned editor (same-package direct write, like G2).
	// Production code does not yet record provenance, so we establish the entry
	// the lifecycle op is required to clear.
	ctrl.state.Meta.WindowProvenance = map[w.DesiredWindowID]w.LiveWindowID{
		attrEditorID(): edLive,
	}

	// Archive p1 (intent.ArchiveProject{Project: ...}).
	if _, err := ctrl.ApplyIntent(context.Background(), intent.ArchiveProject{Project: "p1"}); err != nil {
		t.Fatalf("archive-project: %v", err)
	}

	obs2, _ := fake.Observe(context.Background())
	// Observable: no managed editor for p1 remains anywhere.
	if id, _, ok := attrFindManagedEditor(obs2); ok {
		t.Fatalf("ATTR-E2: managed editor %q still present after archive (INV-04 must AXClose all windows of an archived project)", id)
	}
	// Mechanism (RED today): the provenance entry for the closed editor MUST be
	// cleared. Asserted UNCONDITIONALLY — the window is gone, so no live ID may
	// remain claimed for its identity.
	if got := ctrl.State().Meta.WindowProvenance[attrEditorID()]; got != "" {
		t.Fatalf("ATTR-E2: WindowProvenance[%v]=%q still set after archive (entry must be cleared when the window is closed)", attrEditorID(), got)
	}
}

// ATTR-E3: profile switch moves a project OUT of the active profile → its
// managed editor is closed/relocated per plan and provenance cleared. Observable:
// after switching to a profile that does NOT assign p1 to slot Q, no managed
// editor for p1 sits in slot Q.
func TestZedAttr_E3_ProfileSwitchClosesLeavingEditor(t *testing.T) {
	env := attrEnv()
	desired := attrDesiredWithEditor()
	// Add a second profile that assigns NOTHING to slot Q (p1 leaves the active set).
	desired.Profiles["empty"] = w.DesiredProfile{
		ID:          "empty",
		Assignments: map[w.SlotID]w.ProjectID{},
	}
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))

	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (spawn editor under default profile): %v", err)
	}
	obs, _ := fake.Observe(context.Background())
	edLive, win, ok := attrFindManagedEditor(obs)
	if !ok || win.Workspace != "Q" {
		t.Fatalf("ATTR-E3: precondition failed — managed editor not in slot Q before profile switch (ok=%v ws=%q)", ok, win.Workspace)
	}
	// Seed provenance for the spawned editor (same-package direct write, like G2).
	ctrl.state.Meta.WindowProvenance = map[w.DesiredWindowID]w.LiveWindowID{
		attrEditorID(): edLive,
	}

	// Switch to the profile that does not assign p1.
	if _, err := ctrl.ApplyIntent(context.Background(), intent.SwitchProfile{To: "empty"}); err != nil {
		t.Fatalf("switch-profile: %v", err)
	}

	obs2, _ := fake.Observe(context.Background())
	// Observable: p1's managed editor is no longer occupying slot Q.
	if _, win, ok := attrFindManagedEditorFor(obs2, "p1"); ok && win.Workspace == "Q" {
		t.Fatalf("ATTR-E3: managed editor for p1 still on slot Q after it left the active profile (must be closed/relocated)")
	}
	// Mechanism (RED today): with the empty profile p1 has no slot to relocate to,
	// so its managed editor is CLOSED. Provenance for the closed identity MUST be
	// cleared — asserted UNCONDITIONALLY. (Guard against the prior vacuous form
	// that hid behind an always-empty map.)
	if _, _, stillManaged := attrFindManagedEditorFor(obs2, "p1"); stillManaged {
		t.Fatalf("ATTR-E3: precondition broke — p1's editor was not closed by the profile switch (cannot assert provenance clear)")
	}
	if got := ctrl.State().Meta.WindowProvenance[attrEditorID()]; got != "" {
		t.Fatalf("ATTR-E3: WindowProvenance[%v]=%q still set after p1's editor was closed on profile switch (entry must be cleared)", attrEditorID(), got)
	}
}

// ATTR-E4: slot activation triggers attribution — assigning a project to a slot
// (intent.AssignProject) makes its managed editor appear in that slot, with
// provenance established. Observable: editor in slot Q after AssignProject.
func TestZedAttr_E4_SlotActivationSpawnsEditor(t *testing.T) {
	env := attrEnv()
	// Start with NO assignment so the slot is initially inactive for p1.
	desired := attrDesiredWithEditor()
	desired.Profiles["default"] = w.DesiredProfile{
		ID:          "default",
		Assignments: map[w.SlotID]w.ProjectID{},
	}
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))

	// With no assignment, reconcile should not place a managed editor in slot Q.
	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (no assignment): %v", err)
	}
	obs0, _ := fake.Observe(context.Background())
	if _, win, ok := attrFindManagedEditorFor(obs0, "p1"); ok && win.Workspace == "Q" {
		t.Fatalf("ATTR-E4: precondition failed — editor already in slot Q before activation")
	}

	// Activate: assign p1 to slot Q.
	if _, err := ctrl.ApplyIntent(context.Background(), intent.AssignProject{Slot: "Q", Project: "p1"}); err != nil {
		t.Fatalf("assign-project: %v", err)
	}

	obs, _ := fake.Observe(context.Background())
	_, win, ok := attrFindManagedEditorFor(obs, "p1")
	if !ok {
		t.Fatalf("ATTR-E4: no managed editor for p1 after slot activation (assign must spawn/adopt into the slot)")
	}
	if win.Workspace != "Q" {
		t.Fatalf("ATTR-E4: managed editor for p1 is on %q after activation, want slot Q", win.Workspace)
	}
	// Mechanism: provenance established for the activated editor.
	if got := ctrl.State().Meta.WindowProvenance[attrEditorID()]; got == "" {
		t.Fatalf("ATTR-E4: WindowProvenance[%v] empty after slot activation (provenance must be established)", attrEditorID())
	}
}

// ATTR-E5: slot deactivation (intent.UnassignSlot) → the managed window is
// handled per inactive policy; with the default "remove" policy the editor is
// closed and provenance cleared. Observable: no managed editor for p1 in slot Q
// after unassign.
func TestZedAttr_E5_SlotDeactivationHandlesEditor(t *testing.T) {
	env := attrEnv()
	desired := attrDesiredWithEditor()
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))

	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (spawn editor): %v", err)
	}
	obs, _ := fake.Observe(context.Background())
	edLive, win, ok := attrFindManagedEditor(obs)
	if !ok || win.Workspace != "Q" {
		t.Fatalf("ATTR-E5: precondition failed — editor not in slot Q before deactivation")
	}
	// Seed provenance for the spawned editor (same-package direct write, like G2).
	ctrl.state.Meta.WindowProvenance = map[w.DesiredWindowID]w.LiveWindowID{
		attrEditorID(): edLive,
	}

	// Deactivate slot Q.
	if _, err := ctrl.ApplyIntent(context.Background(), intent.UnassignSlot{Slot: "Q"}); err != nil {
		t.Fatalf("unassign-slot: %v", err)
	}

	obs2, _ := fake.Observe(context.Background())
	// Observable (default remove policy): no managed editor for p1 occupies slot Q.
	if _, win, ok := attrFindManagedEditorFor(obs2, "p1"); ok && win.Workspace == "Q" {
		t.Fatalf("ATTR-E5: managed editor for p1 still on slot Q after deactivation (inactive policy must handle it)")
	}
	// Mechanism (RED today): the default remove policy CLOSES p1's editor, so its
	// provenance entry MUST be cleared — asserted UNCONDITIONALLY.
	if _, _, stillManaged := attrFindManagedEditorFor(obs2, "p1"); stillManaged {
		t.Fatalf("ATTR-E5: precondition broke — p1's editor was not closed by the default remove policy (cannot assert provenance clear)")
	}
	if got := ctrl.State().Meta.WindowProvenance[attrEditorID()]; got != "" {
		t.Fatalf("ATTR-E5: WindowProvenance[%v]=%q still set after deactivation closed p1's editor (entry must be cleared)", attrEditorID(), got)
	}
}

// ATTR-G1: daemon-only restart with Zed still alive and provenance persisted.
// Reconstruct a NEW controller from the SAME store + SAME Fake (windows still
// alive), then reconcile. The restarted controller must RE-MATCH the live editor
// via persisted provenance and NOT respawn a duplicate. RED-tests provenance
// persistence: today provenance is not carried in the committed generation, so a
// fresh controller has no provenance to restore.
func TestZedAttr_G1_DaemonRestartNoRespawn(t *testing.T) {
	env := attrEnv()
	desired := attrDesiredWithEditor()
	st := store.NewMemoryStore(desired)
	fake := wm.NewFake(env)

	// First controller incarnation: spawn and own the editor.
	ctrl1 := New(env, desired, fake, st)
	if _, err := ctrl1.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (incarnation 1 spawn): %v", err)
	}
	obs1, _ := fake.Observe(context.Background())
	live1, _, ok := attrFindManagedEditor(obs1)
	if !ok {
		t.Fatalf("ATTR-G1: precondition failed — editor not spawned in incarnation 1")
	}
	if got := ctrl1.State().Meta.WindowProvenance[attrEditorID()]; got != live1 {
		t.Fatalf("ATTR-G1: incarnation 1 provenance[%v]=%q, want %q", attrEditorID(), got, live1)
	}

	// Daemon restart: NEW controller, SAME store (provenance must persist), SAME
	// Fake (the Zed window is still alive).
	gen, err := st.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("LoadCurrentGeneration: %v", err)
	}
	ctrl2 := NewFromGeneration(env, gen, fake, st)

	// Mechanism (RED): provenance must be restored from the persisted generation.
	if got := ctrl2.State().Meta.WindowProvenance[attrEditorID()]; got != live1 {
		t.Fatalf("ATTR-G1: after daemon restart provenance[%v]=%q, want persisted %q (provenance must survive a daemon restart)", attrEditorID(), got, live1)
	}

	if _, err := ctrl2.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (incarnation 2): %v", err)
	}
	obs2, _ := fake.Observe(context.Background())
	// Observable: exactly ONE Zed window on slot Q — the original live window was
	// re-matched, not duplicated.
	if n := attrCountEditorsInSlot(obs2, "Q"); n != 1 {
		t.Fatalf("ATTR-G1: %d Zed windows on slot Q after daemon restart, want exactly 1 (must re-match the live window, not respawn)", n)
	}
	// And it must still be the SAME live window (no churn).
	live2, _, ok := attrFindManagedEditor(obs2)
	if !ok || live2 != live1 {
		t.Fatalf("ATTR-G1: managed editor live ID after restart = %q (ok=%v), want the persisted %q", live2, ok, live1)
	}
}

// ATTR-G2: macOS reboot — both daemon and Zed died (provenance stale). A FRESH
// controller + a FRESH Fake (no live managed windows). A user same-title window
// is auto-restored by macOS onto a NON-slot workspace. The controller must drop
// the stale provenance, NOT adopt the user's non-slot window, and fresh-spawn its
// own managed editor into the slot. Observable: user window untouched on U, a
// managed editor in slot Q.
func TestZedAttr_G2_RebootStaleProvenanceFreshSpawn(t *testing.T) {
	env := attrEnv()
	desired := attrDesiredWithEditor()
	// Simulate a prior incarnation's stale provenance in the persisted generation
	// by seeding it onto a fresh controller's state below; here the fresh Fake has
	// no such live window, so the entry is stale.
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))

	// User's Zed auto-restored by macOS on their own (non-slot) workspace U.
	fake.InjectExternalWindow("user-zed", "U", "p1", "dev.zed.Zed")

	// Seed a stale provenance entry pointing at a window that no longer exists
	// (the dead pre-reboot live ID). A correct impl must validate-and-drop it.
	// (Same-package test seeds controller state directly; no public seam needed.)
	ctrl.state.Meta.WindowProvenance = map[w.DesiredWindowID]w.LiveWindowID{
		attrEditorID(): "dead-prereboot-zed",
	}

	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (post-reboot): %v", err)
	}
	obs, _ := fake.Observe(context.Background())

	// Observable 1: the user's window is untouched, still on U.
	uw, ok := obs.Windows["user-zed"]
	if !ok {
		t.Fatalf("ATTR-G2: user's auto-restored Zed was removed (non-slot windows are inviolable)")
	}
	if uw.Workspace != "U" {
		t.Fatalf("ATTR-G2: user's Zed moved to %q (must stay on its own workspace U)", uw.Workspace)
	}
	// Observable 2: a managed editor for p1 exists in slot Q (fresh spawn).
	_, win, ok := attrFindManagedEditorFor(obs, "p1")
	if !ok || win.Workspace != "Q" {
		t.Fatalf("ATTR-G2: no managed editor in slot Q after reboot (must fresh-spawn; ok=%v ws=%q)", ok, win.Workspace)
	}
	// The managed editor must NOT be the user's window.
	if win.ID == "user-zed" {
		t.Fatalf("ATTR-G2: managed editor became the user's auto-restored window (must not adopt a non-slot window)")
	}
}

// ATTR-G3: reboot with the user's own Zed auto-restored to its own (non-slot)
// workspace, on a COLD start with NO provenance. The user window must stay
// untouched; projwm reconstructs its own managed editor into the slot. This is
// the reboot-specific framing of B3 (cold start, no provenance, non-slot
// inviolable) — overlaps B3 but asserts the cold-start (empty provenance) entry
// path explicitly.
func TestZedAttr_G3_RebootUserZedUntouchedColdStart(t *testing.T) {
	env := attrEnv()
	desired := attrDesiredWithEditor()
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))

	// Cold start: provenance map is empty (nothing persisted). Assert that.
	if got := ctrl.State().Meta.WindowProvenance[attrEditorID()]; got != "" {
		t.Fatalf("ATTR-G3: cold start must have no provenance, got %q", got)
	}

	// macOS auto-restored the user's Zed on their own non-slot workspace U.
	fake.InjectExternalWindow("user-zed", "U", "p1", "dev.zed.Zed")

	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (cold-start reboot): %v", err)
	}
	obs, _ := fake.Observe(context.Background())

	// Observable: user window untouched on U.
	uw, ok := obs.Windows["user-zed"]
	if !ok {
		t.Fatalf("ATTR-G3: user's auto-restored Zed was removed on cold start (non-slot inviolable)")
	}
	if uw.Workspace != "U" {
		t.Fatalf("ATTR-G3: user's auto-restored Zed moved to %q (must stay on U)", uw.Workspace)
	}
	// Observable: projwm reconstructed its own managed editor into slot Q.
	_, win, ok := attrFindManagedEditorFor(obs, "p1")
	if !ok || win.Workspace != "Q" {
		t.Fatalf("ATTR-G3: no managed editor in slot Q after cold-start reboot (projwm must reconstruct; ok=%v ws=%q)", ok, win.Workspace)
	}
	if win.ID == "user-zed" {
		t.Fatalf("ATTR-G3: managed editor became the user's window (cold start must not adopt a non-slot window)")
	}
}

// ATTR-D5: concurrent multi-spawn (profile switch spawning many Zeds at once).
// Two projects, each with one editor, assigned to two distinct slots. Both
// editors must spawn AND match correctly with NO cross-contamination: each
// project's editor lands in its own slot and resolves to its own identity.
func TestZedAttr_D5_ConcurrentMultiSpawnNoCrossContamination(t *testing.T) {
	env := attrTwoSlotEnv()
	desired := w.DesiredWorld{
		ActiveProfile: "default",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"default": {ID: "default", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1", "R": "p2"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{attrZedDesiredWindow("p1")}},
			"p2": {ID: "p2", Windows: []w.DesiredWindow{attrZedDesiredWindow("p2")}},
		},
	}
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))

	// One reconcile fans out to spawn both editors (batch attribution).
	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (concurrent multi-spawn): %v", err)
	}
	obs, _ := fake.Observe(context.Background())

	// Each project's editor exists and lands in its OWN slot (no cross-contamination).
	id1, win1, ok1 := attrFindManagedEditorFor(obs, "p1")
	if !ok1 {
		t.Fatalf("ATTR-D5: p1 editor not spawned/matched")
	}
	if win1.Workspace != "Q" {
		t.Fatalf("ATTR-D5: p1 editor on %q, want slot Q (cross-contamination)", win1.Workspace)
	}
	id2, win2, ok2 := attrFindManagedEditorFor(obs, "p2")
	if !ok2 {
		t.Fatalf("ATTR-D5: p2 editor not spawned/matched")
	}
	if win2.Workspace != "R" {
		t.Fatalf("ATTR-D5: p2 editor on %q, want slot R (cross-contamination)", win2.Workspace)
	}
	// The two managed editors must be DISTINCT live windows.
	if id1 == id2 {
		t.Fatalf("ATTR-D5: p1 and p2 resolved to the SAME live window %q (batch attribution must keep them distinct)", id1)
	}
	// Exactly one Zed per slot — no extra duplicates from the batch spawn.
	if n := attrCountEditorsInSlot(obs, "Q"); n != 1 {
		t.Fatalf("ATTR-D5: %d Zed on slot Q, want exactly 1", n)
	}
	if n := attrCountEditorsInSlot(obs, "R"); n != 1 {
		t.Fatalf("ATTR-D5: %d Zed on slot R, want exactly 1", n)
	}
}
