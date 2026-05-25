package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/yuu-th/projwm-next/internal/cockpitsnap"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// fixtureSnap builds a minimal Snapshot for unit tests.
func fixtureSnap() cockpitsnap.Snapshot {
	return cockpitsnap.Snapshot{
		Generation:    "G0001",
		Epoch:         w.Epoch(7),
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "dotfiles"}, Description: "Work"},
			"home": {ID: "home", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"dotfiles": {ID: "dotfiles"},
			"park-a":   {ID: "park-a"},                 // parked
			"arch-a":   {ID: "arch-a", Archived: true}, // archived
		},
		Slots: []w.SlotSpec{
			{ID: "Q", Workspace: "Q", Order: 1},
		},
		Parked:   []w.ProjectID{"park-a"},
		Archived: []w.ProjectID{"arch-a"},
		ActiveCards: []w.Card{
			{
				ID:        "card-1",
				Type:      w.CardTypeNew,
				Subject:   "ghostty needs respawn",
				Context:   map[string]string{"live": "live-7", "bundleID": "com.mitchellh.ghostty"},
				CreatedAt: time.Now().Add(-30 * time.Second).UnixNano(),
				Actions: []w.CardAction{
					{Key: "Enter", Label: "respawn properly"},
					{Key: "c", Label: "close"},
				},
			},
			{
				ID:        "card-2",
				Type:      w.CardTypeClosed,
				Subject:   "shell-2 was closed",
				CreatedAt: time.Now().Add(-2 * time.Minute).UnixNano(),
				Actions:   []w.CardAction{{Key: "Enter", Label: "keep restored"}},
			},
		},
		Source: "store",
	}
}

// newTestModel builds a Model with the fixture snap loaded so tests
// don't need a working IPC client.
func newTestModel(t *testing.T) Model {
	t.Helper()
	m := New(Config{})
	m.snap = fixtureSnap()
	m.rebuildItems()
	return m
}

// v2.9 §9.4 (Phase γ.0): each tab's items list is scoped — cards live
// in the Cards modal (sortedActiveCards), Slots tab shows slots/park/
// viewer, Archived tab shows archived, Profiles tab shows profiles,
// Trace tab is empty state. The old "one list with everything" model
// was removed when Phase α introduced tabs.
func TestRebuildItems_SlotsTabHasSlotsParkViewer(t *testing.T) {
	m := newTestModel(t)
	kinds := map[itemKind]bool{}
	for _, it := range m.items {
		kinds[it.Kind] = true
	}
	for _, k := range []itemKind{itemSlot, itemParked, itemViewer} {
		if !kinds[k] {
			t.Errorf("Slots tab missing %s section", k)
		}
	}
	// Cards / Archived / Profiles must NOT appear in Slots-tab items.
	for _, k := range []itemKind{itemCard, itemArchive, itemProfile} {
		if kinds[k] {
			t.Errorf("Slots tab unexpectedly has %s rows", k)
		}
	}
}

// v2.9 §9.4 — Cards tab uses the modal renderer; items list is empty.
// Card cursor / order is owned by sortedActiveCards (verified separately).
func TestRebuildItems_CardsTabUsesModal(t *testing.T) {
	m := newTestModel(t)
	m = m.switchTab(TabCards)
	m.rebuildItems()
	if len(m.items) != 0 {
		t.Errorf("Cards tab items must be empty (modal owns cursor); got %d", len(m.items))
	}
	// The modal's card ordering must still be CreatedAt desc (G7 / §10.4).
	cards := sortedActiveCards(m.snap.ActiveCards)
	if len(cards) < 2 {
		t.Fatal("fixture must have >= 2 cards")
	}
	if cards[0].ID != "card-1" {
		t.Errorf("cards[0]=%s, want card-1 (newest)", cards[0].ID)
	}
}

// v2.9 §9.4 — Archived tab lists only archived projects.
func TestRebuildItems_ArchivedTab(t *testing.T) {
	m := newTestModel(t)
	m = m.switchTab(TabArchived)
	m.rebuildItems()
	if len(m.items) == 0 {
		t.Fatal("Archived tab must show archived projects")
	}
	for _, it := range m.items {
		if it.Kind != itemArchive {
			t.Errorf("Archived tab row kind=%s, want %s", it.Kind, itemArchive)
		}
	}
}

// v2.9 §9.4 — Profiles tab lists every profile (active included).
func TestRebuildItems_ProfilesTab(t *testing.T) {
	m := newTestModel(t)
	m = m.switchTab(TabProfiles)
	m.rebuildItems()
	if len(m.items) == 0 {
		t.Fatal("Profiles tab must show profiles")
	}
	for _, it := range m.items {
		if it.Kind != itemProfile {
			t.Errorf("Profiles tab row kind=%s, want %s", it.Kind, itemProfile)
		}
	}
}

// §9.4 — fzf filter narrows the visible items.
func TestFilter_Narrows(t *testing.T) {
	m := newTestModel(t)
	m.filter.SetValue("dotfiles")
	m.rebuildItems()
	if len(m.items) == 0 {
		t.Fatal("expected at least 1 item matching dotfiles")
	}
	for _, it := range m.items {
		hay := strings.ToLower(it.Label + " " + it.Detail)
		if !strings.Contains(hay, "dotfiles") {
			t.Errorf("post-filter row %q has no 'dotfiles' token", it.Label)
		}
	}
}

func TestFilter_NoMatchClears(t *testing.T) {
	m := newTestModel(t)
	m.filter.SetValue("zzz-no-such-token-xxx")
	m.rebuildItems()
	if len(m.items) != 0 {
		t.Errorf("expected empty filter result, got %d items", len(m.items))
	}
}

// §10.1 / G1 — relative time formatter.
func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	t.Run("just now", func(t *testing.T) {
		ts := now.Add(-2 * time.Second).UnixNano()
		got := relativeTime(ts, now)
		if !strings.Contains(got, "just now") {
			t.Errorf("got %q", got)
		}
	})
	t.Run("seconds ago", func(t *testing.T) {
		ts := now.Add(-30 * time.Second).UnixNano()
		got := relativeTime(ts, now)
		if !strings.Contains(got, "s ago") {
			t.Errorf("got %q", got)
		}
	})
	t.Run("minutes ago", func(t *testing.T) {
		ts := now.Add(-5 * time.Minute).UnixNano()
		got := relativeTime(ts, now)
		if !strings.Contains(got, "m ago") {
			t.Errorf("got %q", got)
		}
	})
	t.Run("hours ago", func(t *testing.T) {
		ts := now.Add(-2 * time.Hour).UnixNano()
		got := relativeTime(ts, now)
		if !strings.Contains(got, "h ago") {
			t.Errorf("got %q", got)
		}
	})
	t.Run("zero unix nano renders dash", func(t *testing.T) {
		if got := relativeTime(0, now); got != "—" {
			t.Errorf("zero ts: got %q", got)
		}
	})
}

// §9.4 — highlight reverses the first matching token in a label.
// In a non-TTY test env lipgloss may no-op styling, so we only check
// that the matched token survives (no stripping) and the surrounding
// label is intact.
func TestHighlight(t *testing.T) {
	out := highlight("dotfiles project", "dot")
	if !strings.Contains(out, "dot") {
		t.Errorf("highlight stripped match: %q", out)
	}
	if !strings.Contains(out, "files project") {
		t.Errorf("highlight altered surrounding text: %q", out)
	}
	// Empty filter is a passthrough.
	if got := highlight("hello", ""); got != "hello" {
		t.Errorf("empty filter should be passthrough, got %q", got)
	}
	// No match is a passthrough.
	if got := highlight("hello", "xyz"); got != "hello" {
		t.Errorf("no-match should be passthrough, got %q", got)
	}
}

// §10.5 — destructive action detection (close/remove/dismiss).
func TestIsDestructiveAction(t *testing.T) {
	cases := []struct {
		a    w.CardAction
		want bool
	}{
		{w.CardAction{Key: "c", Label: "close"}, true},
		{w.CardAction{Key: "k", Label: "keep removed"}, true},
		{w.CardAction{Key: "Enter", Label: "adopt"}, false},
		{w.CardAction{Key: "x", Label: "Remove forever"}, true},
		{w.CardAction{Key: "y", Label: "purge orphan"}, true},
	}
	for _, c := range cases {
		if got := isDestructiveAction(c.a); got != c.want {
			t.Errorf("isDestructiveAction(%+v) = %v, want %v", c.a, got, c.want)
		}
	}
}

// §9.6 / §13.3 — context kv redacts private keys.
func TestRedactedContext(t *testing.T) {
	got := redactedContext(map[string]string{"workspace": "Q", "url": "https://secret/path"})
	if !strings.Contains(got, "workspace=Q") {
		t.Errorf("non-private key dropped: %q", got)
	}
	if strings.Contains(got, "https://secret/path") {
		t.Errorf("private URL leaked: %q", got)
	}
	if !strings.Contains(got, "<redacted>") {
		t.Errorf("missing <redacted> marker: %q", got)
	}
}
