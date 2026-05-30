package controller

import (
	"context"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// L2 behavioral tests for SSOT §6.9.1 attribution (ATTR-*), framed at the
// USER-EXPERIENCE / world-state boundary (not internal provenance state): given
// a modeled window world, assert the observable outcome the user would see —
// their own windows are never touched, the managed editor lands in its slot, no
// duplicates. These are implementation-independent (provenance is one way to
// satisfy them). The authoritative guarantee is the matching L4 real_ops test;
// this layer gives fast deterministic feedback.

func attrEnv() w.ManagedEnvironment {
	return w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Slots: []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
			Workspaces: []w.WorkspaceSpec{
				{ID: "Q", RawName: "Q", DisplayName: "Q", Role: w.WorkspaceProject},
				// A user-owned (non-slot) workspace.
				{ID: "U", RawName: "U", DisplayName: "U", Role: w.WorkspaceGeneral},
			},
		},
	}
}

// attrDesiredWithEditor builds a desired world with project p1 (one Zed editor)
// assigned to slot Q. The editor title is the basename "p1".
func attrDesiredWithEditor() w.DesiredWorld {
	ed := w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 1}
	return w.DesiredWorld{
		ActiveProfile: "default",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"default": {ID: "default", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{
				{
					ID:   ed,
					Kind: w.WindowEditor,
					App:  w.AppRequirement{BundleID: "dev.zed.Zed"},
					TitleContract: w.TitleContract{
						Authority: w.TitleAppOwned,
						Expected:  "p1",
						Drift:     w.TitleDriftRematch,
					},
					MatchHints: []w.MatchHint{
						{Kind: w.MatchByBundleID, Pattern: "dev.zed.Zed", Confidence: w.MatchStrong},
					},
				},
			}},
		},
	}
}

// ATTR-B3 (user-experience guarantee): a same-title Zed window on the USER's own
// (non-slot) workspace must NOT be adopted — it must not be moved to the slot or
// closed. projwm must instead spawn its own managed editor into the slot.
func TestZedAttr_B3_UserWindowOnOwnWorkspaceUntouched(t *testing.T) {
	env := attrEnv()
	desired := attrDesiredWithEditor()
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))

	// User already has their own "p1" Zed open on their own workspace U.
	fake.InjectExternalWindow("user-zed", "U", "p1", "dev.zed.Zed")

	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	obs, err := fake.Observe(context.Background())
	if err != nil {
		t.Fatalf("observe: %v", err)
	}

	// Guarantee 1: the user's window is still there, still on workspace U.
	uw, ok := obs.Windows["user-zed"]
	if !ok {
		t.Fatalf("ATTR-B3: user's own Zed window was removed by reconcile (must be untouched)")
	}
	if uw.Workspace != "U" {
		t.Fatalf("ATTR-B3: user's own Zed window was moved to %q (must stay on its own workspace U)", uw.Workspace)
	}
	// Guarantee 2: a managed editor exists in the slot Q (projwm spawned its own).
	managedInQ := false
	for _, win := range obs.Windows {
		if win.Workspace == "Q" && win.MatchedTo != nil && win.MatchedTo.Kind == w.WindowEditor && win.MatchedTo.Project == "p1" {
			managedInQ = true
		}
	}
	if !managedInQ {
		t.Fatalf("ATTR-B3: no managed editor in slot Q (projwm must spawn its own, not adopt the user's)")
	}
}

// ATTR-B1 (user-experience guarantee): after projwm owns its editor in the slot,
// the user opening a same-title Zed window on their own workspace must not cause
// that window to be moved or closed.
func TestZedAttr_B1_UserOpensSameNameAfterOwnership(t *testing.T) {
	env := attrEnv()
	desired := attrDesiredWithEditor()
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))

	// projwm establishes ownership first.
	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (establish ownership): %v", err)
	}
	// Then the user opens their own same-title Zed on their workspace U.
	fake.InjectExternalWindow("user-zed", "U", "p1", "dev.zed.Zed")
	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (after user opened window): %v", err)
	}

	obs, err := fake.Observe(context.Background())
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	uw, ok := obs.Windows["user-zed"]
	if !ok {
		t.Fatalf("ATTR-B1: user's window was closed after they opened it (must be untouched)")
	}
	if uw.Workspace != "U" {
		t.Fatalf("ATTR-B1: user's window was moved to %q (must stay on U)", uw.Workspace)
	}
}

// attrEditorID is the desired identity used by attrDesiredWithEditor.
func attrEditorID() w.DesiredWindowID {
	return w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 1}
}

// attrFindManagedEditor returns the live ID of the window matched to editor-1:p1.
func attrFindManagedEditor(obs w.ObservedWorld) (w.LiveWindowID, w.ObservedWindow, bool) {
	for id, win := range obs.Windows {
		if win.MatchedTo != nil && win.MatchedTo.Kind == w.WindowEditor && win.MatchedTo.Project == "p1" {
			return id, win, true
		}
	}
	return "", w.ObservedWindow{}, false
}

// ATTR-A1 (mechanism): the controller must RECORD provenance for the editor it
// spawned — (identity → live window ID) — so it can later distinguish its own
// window from a user's colliding one. RED until provenance capture is wired.
func TestZedAttr_A1_SpawnRecordsProvenance(t *testing.T) {
	env := attrEnv()
	desired := attrDesiredWithEditor()
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))

	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (spawn editor): %v", err)
	}
	obs, err := fake.Observe(context.Background())
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	edLive, _, ok := attrFindManagedEditor(obs)
	if !ok {
		t.Fatalf("ATTR-A1: editor was not spawned")
	}
	prov := ctrl.State().Meta.WindowProvenance
	if got := prov[attrEditorID()]; got != edLive {
		t.Fatalf("ATTR-A1: WindowProvenance[%v]=%q, want the spawned editor's live ID %q (controller must record what it spawned)", attrEditorID(), got, edLive)
	}
}

// ATTR-A2 (controller, user-experience): after projwm owns its editor and a user
// opens a same-title editor on their own workspace, the MANAGED editor must
// remain projwm's own spawned window (in slot Q) — not the user's, and the
// resolution must not collapse to ambiguous-and-refuse. RED if title collision
// makes the controller pick/refuse wrongly.
func TestZedAttr_A2_ManagedEditorStaysOursUnderTitleCollision(t *testing.T) {
	env := attrEnv()
	desired := attrDesiredWithEditor()
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))

	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (establish ownership): %v", err)
	}
	obsBefore, _ := fake.Observe(context.Background())
	ours, _, ok := attrFindManagedEditor(obsBefore)
	if !ok {
		t.Fatalf("ATTR-A2: editor not spawned")
	}

	// User opens a colliding same-title editor on their own workspace.
	fake.InjectExternalWindow("user-zed", "U", "p1", "dev.zed.Zed")
	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (after collision): %v", err)
	}
	obs, _ := fake.Observe(context.Background())
	matched, mw, ok := attrFindManagedEditor(obs)
	if !ok {
		t.Fatalf("ATTR-A2: editor identity lost its match under title collision (must stay bound to our window)")
	}
	if matched != ours {
		t.Fatalf("ATTR-A2: managed editor switched to %q (want our original %q); the user's window must never become the managed one", matched, ours)
	}
	if mw.Workspace != "Q" {
		t.Fatalf("ATTR-A2: managed editor is on %q, want slot Q", mw.Workspace)
	}
}

// ATTR-A3 (controller): when our provenance window disappears (user closed /
// crash), the controller must respawn a managed editor (not get stuck). Likely
// GREEN guard today (missing identity respawns), kept to protect the behavior.
func TestZedAttr_A3_RespawnAfterOurWindowGone(t *testing.T) {
	env := attrEnv()
	desired := attrDesiredWithEditor()
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))

	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (spawn): %v", err)
	}
	obs, _ := fake.Observe(context.Background())
	ours, _, ok := attrFindManagedEditor(obs)
	if !ok {
		t.Fatalf("ATTR-A3: editor not spawned")
	}
	fake.SimulateUserClose(ours)
	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (after close): %v", err)
	}
	obs2, _ := fake.Observe(context.Background())
	if _, _, ok := attrFindManagedEditor(obs2); !ok {
		t.Fatalf("ATTR-A3: no managed editor after our window went away (must respawn)")
	}
}

// ATTR-B2 (controller, user-experience): on cold start, a same-title editor that
// is already on the managed slot workspace is adopted (becomes the managed
// editor) WITHOUT spawning a duplicate. (Adoption within slot territory.)
func TestZedAttr_B2_AdoptSameTitleOnSlotNoDuplicate(t *testing.T) {
	env := attrEnv()
	desired := attrDesiredWithEditor()
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))

	// A pre-existing same-title editor sits on the slot Q (cold start, no spawn).
	fake.InjectExternalWindow("pre-zed", "Q", "p1", "dev.zed.Zed")
	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	obs, _ := fake.Observe(context.Background())
	// Exactly one editor for p1 on Q (the adopted one), no duplicate spawn.
	count := 0
	for _, win := range obs.Windows {
		if win.App.BundleID == "dev.zed.Zed" && win.Workspace == "Q" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("ATTR-B2: %d Zed windows on slot Q, want exactly 1 (adopt the pre-existing one, do not spawn a duplicate)", count)
	}
}

// ATTR-C1 (controller): two editors of the SAME project share the basename
// title, so title alone cannot tell editor-1 from editor-2. The controller must
// still spawn and DISTINCTLY manage both (provenance gives each its own live
// ID). RED today: same-title pair resolves ambiguous so the pair cannot both be
// uniquely managed.
func TestZedAttr_C1_TwoEditorsSameProjectBothManaged(t *testing.T) {
	env := attrEnv()
	ed1 := w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 1}
	ed2 := w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 2}
	mkEd := func(id w.DesiredWindowID) w.DesiredWindow {
		return w.DesiredWindow{
			ID: id, Kind: w.WindowEditor,
			App: w.AppRequirement{BundleID: "dev.zed.Zed"},
			TitleContract: w.TitleContract{Authority: w.TitleAppOwned, Expected: "p1", Drift: w.TitleDriftRematch},
			MatchHints:    []w.MatchHint{{Kind: w.MatchByBundleID, Pattern: "dev.zed.Zed", Confidence: w.MatchStrong}},
		}
	}
	desired := w.DesiredWorld{
		ActiveProfile: "default",
		Profiles:      map[w.ProfileID]w.DesiredProfile{"default": {ID: "default", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}}},
		Projects:      map[w.ProjectID]w.DesiredProject{"p1": {ID: "p1", Windows: []w.DesiredWindow{mkEd(ed1), mkEd(ed2)}}},
	}
	fake := wm.NewFake(env)
	ctrl := New(env, desired, fake, store.NewMemoryStore(desired))
	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("reconcile (spawn two editors): %v", err)
	}
	obs, _ := fake.Observe(context.Background())
	matched1, matched2 := false, false
	for _, win := range obs.Windows {
		if win.MatchedTo == nil {
			continue
		}
		if *win.MatchedTo == ed1 {
			matched1 = true
		}
		if *win.MatchedTo == ed2 {
			matched2 = true
		}
	}
	if !matched1 || !matched2 {
		t.Fatalf("ATTR-C1: editor-1 matched=%v editor-2 matched=%v; both same-title editors must be distinctly managed (needs provenance)", matched1, matched2)
	}
}
