package controller

import (
	"context"
	"testing"
	"time"

	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// minimalControllerEnv returns a Controller with the smallest WorldState
// shape that lets ApplyIntent succeed for ControllerMeta-only intents
// (DismissCard / DismissAllCards). No daemon-level concerns wired.
func minimalControllerEnv(t *testing.T) *Controller {
	t.Helper()
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Viewer: "A",
			Workspaces: []w.WorkspaceSpec{
				{ID: "A", Role: w.WorkspaceViewer},
				{ID: "Q", Role: w.WorkspaceProject},
			},
			Slots: []w.SlotSpec{
				{ID: "Q", Workspace: "Q", Order: 1},
			},
		},
	}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{},
	}
	st := store.NewMemoryStore(desired)
	return New(env, desired, &wm.Fake{}, st)
}

func TestApplyIntent_DismissCard_RemovesById(t *testing.T) {
	c := minimalControllerEnv(t)
	c.state.Meta.ActiveCards = []w.Card{
		{ID: "c1", Type: w.CardTypeMoved, Subject: "moved A"},
		{ID: "c2", Type: w.CardTypeClosed, Subject: "closed B"},
	}
	_, err := c.ApplyIntent(context.Background(), intent.DismissCard{CardID: "c1"})
	if err != nil {
		t.Fatalf("ApplyIntent: %v", err)
	}
	got := c.State().Meta.ActiveCards
	if len(got) != 1 || got[0].ID != "c2" {
		t.Errorf("expected [c2], got %+v", got)
	}
}

func TestApplyIntent_DismissAllCards_Clears(t *testing.T) {
	c := minimalControllerEnv(t)
	c.state.Meta.ActiveCards = []w.Card{
		{ID: "c1", Type: w.CardTypeMoved},
		{ID: "c2", Type: w.CardTypeClosed},
	}
	_, err := c.ApplyIntent(context.Background(), intent.DismissAllCards{})
	if err != nil {
		t.Fatalf("ApplyIntent: %v", err)
	}
	if got := c.State().Meta.ActiveCards; len(got) != 0 {
		t.Errorf("expected empty, got %+v", got)
	}
}

func TestPromoteOrphans_PromotesAfterGrace(t *testing.T) {
	c := minimalControllerEnv(t)
	c.state.Observed.Windows = map[w.LiveWindowID]w.ObservedWindow{
		"live-1": {ID: "live-1", Workspace: "Q", Kind: w.WindowShell},
	}
	c.state.Meta.PendingOrphans = []w.OrphanCandidate{
		{LiveID: "live-1", Kind: w.WindowShell, Workspace: "Q",
			DetectedAt: time.Now().Add(-10 * time.Second).UnixNano()},
	}
	c.PromoteOrphans(5 * time.Second)
	if got := c.State().Meta.PendingOrphans; len(got) != 0 {
		t.Errorf("expected pending drained, got %d", len(got))
	}
	cards := c.State().Meta.ActiveCards
	if len(cards) != 1 || cards[0].Type != w.CardTypeNew {
		t.Errorf("expected [NEW] card, got %+v", cards)
	}
}

func TestPromoteOrphans_KeepsWithinGrace(t *testing.T) {
	c := minimalControllerEnv(t)
	c.state.Observed.Windows = map[w.LiveWindowID]w.ObservedWindow{
		"live-1": {ID: "live-1", Workspace: "Q", Kind: w.WindowShell},
	}
	c.state.Meta.PendingOrphans = []w.OrphanCandidate{
		{LiveID: "live-1", Kind: w.WindowShell, Workspace: "Q",
			DetectedAt: time.Now().UnixNano()},
	}
	c.PromoteOrphans(5 * time.Second)
	if got := c.State().Meta.PendingOrphans; len(got) != 1 {
		t.Errorf("expected still pending, got %d", len(got))
	}
	if got := c.State().Meta.ActiveCards; len(got) != 0 {
		t.Errorf("expected no cards (within grace), got %+v", got)
	}
}

// TestController_ActiveCards_NoDuplicateForSameLiveID_AndType covers the bug
// where PromoteOrphans would create a [NEW] card, drop the orphan from
// PendingOrphans, and then the next windows-changed event would re-enqueue
// the same live window as a new orphan candidate — leading to a second card
// being promoted after another 5-second grace period. §3.1 / §3.6 dedup.
func TestController_ActiveCards_NoDuplicateForSameLiveID_AndType(t *testing.T) {
	c := minimalControllerEnv(t)
	c.state.Observed.Windows = map[w.LiveWindowID]w.ObservedWindow{
		"live-1": {ID: "live-1", Workspace: "Q", Kind: w.WindowShell},
	}

	// Promote orphan: simulate first grace-period expiry.
	c.state.Meta.PendingOrphans = []w.OrphanCandidate{
		{LiveID: "live-1", Kind: w.WindowShell, Workspace: "Q",
			DetectedAt: time.Now().Add(-10 * time.Second).UnixNano()},
	}
	c.PromoteOrphans(5 * time.Second)
	if len(c.State().Meta.ActiveCards) != 1 {
		t.Fatalf("expected 1 card after first promotion, got %d", len(c.State().Meta.ActiveCards))
	}
	if len(c.State().Meta.PendingOrphans) != 0 {
		t.Fatalf("expected PendingOrphans drained, got %d", len(c.State().Meta.PendingOrphans))
	}

	// Simulate a second grace-period expiry for the same window (orphan
	// re-enqueued after the first promotion). appendActiveCards must dedup.
	c.state.Meta.PendingOrphans = []w.OrphanCandidate{
		{LiveID: "live-1", Kind: w.WindowShell, Workspace: "Q",
			DetectedAt: time.Now().Add(-10 * time.Second).UnixNano()},
	}
	c.PromoteOrphans(5 * time.Second)
	// §10.4: no duplicate card must be added.
	if got := len(c.State().Meta.ActiveCards); got != 1 {
		t.Errorf("dedup failed: expected 1 card, got %d", got)
	}
}

// TestCardAlreadyActive_LiveKey verifies that appendActiveCards deduplicates
// [NEW] cards using the "live" context key (not "window"). §3.1 / §10.4.
func TestCardAlreadyActive_LiveKey(t *testing.T) {
	c := minimalControllerEnv(t)
	card := w.Card{
		Type:    w.CardTypeNew,
		Subject: "manual shell window on managed workspace Q",
		Context: map[string]string{"live": "live-1", "kind": "shell", "workspace": "Q"},
	}
	c.appendActiveCards([]w.Card{card})
	if len(c.State().Meta.ActiveCards) != 1 {
		t.Fatalf("first add: expected 1 card")
	}

	// Second add of same card (same live key) must be deduped.
	c.appendActiveCards([]w.Card{card})
	if got := len(c.State().Meta.ActiveCards); got != 1 {
		t.Errorf("dedup by 'live' key failed: expected 1 card, got %d", got)
	}
}

func TestPromoteOrphans_DropsSilentlyAdoptedOrClosed(t *testing.T) {
	c := minimalControllerEnv(t)
	id := w.DesiredWindowID{Project: "p", Kind: w.WindowShell, Index: 1}
	c.state.Observed.Windows = map[w.LiveWindowID]w.ObservedWindow{
		"live-adopted": {ID: "live-adopted", Workspace: "Q", Kind: w.WindowShell, MatchedTo: &id},
	}
	c.state.Meta.PendingOrphans = []w.OrphanCandidate{
		{LiveID: "live-adopted", Kind: w.WindowShell, Workspace: "Q",
			DetectedAt: time.Now().Add(-10 * time.Second).UnixNano()},
		{LiveID: "live-vanished", Kind: w.WindowShell, Workspace: "Q",
			DetectedAt: time.Now().Add(-10 * time.Second).UnixNano()},
	}
	c.PromoteOrphans(5 * time.Second)
	if got := c.State().Meta.PendingOrphans; len(got) != 0 {
		t.Errorf("expected pending drained, got %d", len(got))
	}
	if got := c.State().Meta.ActiveCards; len(got) != 0 {
		t.Errorf("expected no cards for adopted/vanished, got %+v", got)
	}
}
