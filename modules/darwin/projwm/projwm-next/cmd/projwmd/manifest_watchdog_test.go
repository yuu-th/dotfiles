package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/controller"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// minimalCtrl returns a Controller with the smallest valid env+store
// suitable for exercising EmitManifestMismatchCard. Mirrors
// controller_cards_test.go::minimalControllerEnv but local to projwmd
// so we can test the wiring end-to-end.
func minimalCtrl(t *testing.T) *controller.Controller {
	t.Helper()
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Viewer:     "A",
			Workspaces: []w.WorkspaceSpec{{ID: "A", Role: w.WorkspaceViewer}},
			Slots:      []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
		},
	}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles:      map[w.ProfileID]w.DesiredProfile{"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{}}},
		Projects:      map[w.ProjectID]w.DesiredProject{},
	}
	st := store.NewMemoryStore(desired)
	return controller.New(env, desired, &wm.Fake{}, st)
}

// E2.1 wiring: checkManifestDigestOnce emits a [MANIFEST] card when
// the on-disk manifest digest no longer matches the boot value, and
// suppresses duplicate emissions while the drift persists.
func TestManifestWatchdog_EmitsOnDrift(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "manifest.json")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	bootDigestRaw := sha256.Sum256([]byte("v1"))
	bootDigest := hex.EncodeToString(bootDigestRaw[:])

	ctrl := minimalCtrl(t)
	state := &manifestWatchdogState{}

	// Pass 1: no drift — no card.
	checkManifestDigestOnce(ctrl, path, bootDigest, state)
	if got := len(ctrl.State().Meta.ActiveCards); got != 0 {
		t.Fatalf("expected 0 cards before drift, got %d", got)
	}

	// Mutate the manifest on disk.
	if err := os.WriteFile(path, []byte("v2 drift"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Pass 2: drift detected → exactly one card.
	checkManifestDigestOnce(ctrl, path, bootDigest, state)
	cards := ctrl.State().Meta.ActiveCards
	if len(cards) != 1 {
		t.Fatalf("expected 1 manifest card after drift, got %d", len(cards))
	}
	if cards[0].Type != w.CardTypeManifest {
		t.Errorf("type = %s, want MANIFEST", cards[0].Type)
	}
	if cards[0].Context["expected"] != bootDigest {
		t.Errorf("expected = %q", cards[0].Context["expected"])
	}

	// Pass 3: still drifted — no duplicate card.
	checkManifestDigestOnce(ctrl, path, bootDigest, state)
	if got := len(ctrl.State().Meta.ActiveCards); got != 1 {
		t.Errorf("expected 1 card (no dup), got %d", got)
	}
}

func TestManifestWatchdog_RestoreResetsNotified(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "manifest.json")
	os.WriteFile(path, []byte("v1"), 0o644)
	digestRaw := sha256.Sum256([]byte("v1"))
	bootDigest := hex.EncodeToString(digestRaw[:])

	ctrl := minimalCtrl(t)
	state := &manifestWatchdogState{}

	// Drift then restore then drift again → exactly 2 cards (one per
	// drift episode, suppression cleared by intervening match).
	os.WriteFile(path, []byte("drift"), 0o644)
	checkManifestDigestOnce(ctrl, path, bootDigest, state)
	os.WriteFile(path, []byte("v1"), 0o644)
	checkManifestDigestOnce(ctrl, path, bootDigest, state)
	os.WriteFile(path, []byte("drift2"), 0o644)
	checkManifestDigestOnce(ctrl, path, bootDigest, state)

	cards := ctrl.State().Meta.ActiveCards
	if len(cards) != 2 {
		t.Errorf("expected 2 MANIFEST cards (per drift episode), got %d", len(cards))
	}
}

func TestManifestWatchdog_IgnoresReadError(t *testing.T) {
	ctrl := minimalCtrl(t)
	state := &manifestWatchdogState{}
	// Path doesn't exist → no card, no panic.
	checkManifestDigestOnce(ctrl, "/no/such/path", "deadbeef", state)
	if got := len(ctrl.State().Meta.ActiveCards); got != 0 {
		t.Errorf("read error should not emit card, got %d", got)
	}
}
