package reducer

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/event"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// tierState produces a WorldState with the manifest shape needed by
// isManagedWorkspaceForEnv / identifyProjectForWorkspace.
func tierState() w.WorldState {
	return w.WorldState{
		Environment: w.ManagedEnvironment{
			Workspaces: w.WorkspaceEnvironment{
				Viewer: "A",
				Workspaces: []w.WorkspaceSpec{
					{ID: "A", Role: w.WorkspaceViewer},
					{ID: "Q", Role: w.WorkspaceProject},
					{ID: "1", Role: w.WorkspaceGeneral},
				},
				Slots: []w.SlotSpec{
					{ID: "Q", Workspace: "Q", Order: 1},
				},
			},
		},
		Desired: w.DesiredWorld{
			ActiveProfile: "work",
			Profiles: map[w.ProfileID]w.DesiredProfile{
				"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "dotfiles"}},
			},
			Projects: map[w.ProjectID]w.DesiredProject{
				"dotfiles": {ID: "dotfiles"},
			},
		},
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{},
		},
	}
}

func TestReactToEvent_Tier1_OrphanOnManagedWS(t *testing.T) {
	s := tierState()
	s.Observed.Windows["live-1"] = w.ObservedWindow{
		ID:        "live-1",
		Workspace: "Q",
		Kind:      w.WindowShell,
		// MatchedTo nil → unmatched, becomes an orphan candidate
	}
	r, err := ReactToEvent(s, event.Event{Kind: event.KindWindowsChanged})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.OrphanAdds) != 1 {
		t.Fatalf("expected 1 OrphanAdd, got %d", len(r.OrphanAdds))
	}
	if r.OrphanAdds[0].LiveID != "live-1" || r.OrphanAdds[0].Workspace != "Q" {
		t.Errorf("OrphanAdd = %+v", r.OrphanAdds[0])
	}
}

func TestReactToEvent_Tier1_SkipsManagedWS_WhenUnmanagedKind(t *testing.T) {
	s := tierState()
	s.Observed.Windows["live-1"] = w.ObservedWindow{
		ID:        "live-1",
		Workspace: "Q",
		Kind:      w.WindowExternal,
	}
	r, err := ReactToEvent(s, event.Event{Kind: event.KindWindowsChanged})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.OrphanAdds) != 0 {
		t.Errorf("WindowExternal should never become orphan, got %d", len(r.OrphanAdds))
	}
}

func TestReactToEvent_Tier1_IgnoresUnmanagedWS(t *testing.T) {
	s := tierState()
	s.Observed.Windows["live-1"] = w.ObservedWindow{
		ID:        "live-1",
		Workspace: "1", // general
		Kind:      w.WindowShell,
	}
	r, err := ReactToEvent(s, event.Event{Kind: event.KindWindowsChanged})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.OrphanAdds) != 0 {
		t.Errorf("unmanaged workspace should not produce orphans, got %d", len(r.OrphanAdds))
	}
}

func TestReactToEvent_Tier1_IgnoresAlreadyMatched(t *testing.T) {
	s := tierState()
	id := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowShell, Index: 1}
	s.Observed.Windows["live-1"] = w.ObservedWindow{
		ID:        "live-1",
		Workspace: "Q",
		Kind:      w.WindowShell,
		MatchedTo: &id,
	}
	r, err := ReactToEvent(s, event.Event{Kind: event.KindWindowsChanged})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.OrphanAdds) != 0 {
		t.Errorf("matched window should not produce orphan")
	}
}

func TestReactToEvent_Tier1_DedupesAgainstPending(t *testing.T) {
	s := tierState()
	s.Meta.PendingOrphans = []w.OrphanCandidate{
		{LiveID: "live-1"},
	}
	s.Observed.Windows["live-1"] = w.ObservedWindow{
		ID:        "live-1",
		Workspace: "Q",
		Kind:      w.WindowShell,
	}
	r, err := ReactToEvent(s, event.Event{Kind: event.KindWindowsChanged})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.OrphanAdds) != 0 {
		t.Errorf("already-tracked orphan should not duplicate, got %d", len(r.OrphanAdds))
	}
}

// TestReactToEvent_Tier1_SkipsWhenActiveCardExists verifies that a live window
// already represented by a [NEW] card in ActiveCards does NOT get re-enqueued
// as a PendingOrphan candidate. This is the primary guard against the
// promote→discard→re-enqueue→re-promote spam cycle. §3.1 / §3.6.
func TestReactToEvent_Tier1_SkipsWhenActiveCardExists(t *testing.T) {
	s := tierState()
	s.Observed.Windows["live-1"] = w.ObservedWindow{
		ID:        "live-1",
		Workspace: "Q",
		Kind:      w.WindowShell,
		// MatchedTo nil → unmatched orphan
	}
	// Simulate: the orphan was already promoted to a [NEW] card.
	s.Meta.ActiveCards = []w.Card{
		{
			Type:    w.CardTypeNew,
			Subject: "manual shell window on managed workspace Q",
			Context: map[string]string{"live": "live-1"},
		},
	}
	r, err := ReactToEvent(s, event.Event{Kind: event.KindWindowsChanged})
	if err != nil {
		t.Fatal(err)
	}
	// §3.1 / §3.6 dedup: no new OrphanAdd because the card already exists.
	if len(r.OrphanAdds) != 0 {
		t.Errorf("expected 0 OrphanAdds (active card exists), got %d: %+v", len(r.OrphanAdds), r.OrphanAdds)
	}
}

func TestReactToEvent_Tier2_LayoutSyncDirtyScope(t *testing.T) {
	s := tierState()
	ws := w.WorkspaceID("Q")
	r, err := ReactToEvent(s, event.Event{
		Kind: event.KindLayoutChanged,
		Data: event.Data{Workspace: &ws},
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ds := range r.DirtyScopes {
		if ds.Kind == "layout-sync" && ds.Key == "dotfiles|Q" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected layout-sync DirtyScope with key dotfiles|Q, got %+v", r.DirtyScopes)
	}
}

func TestReactToEvent_Tier4_MovedCardEmit(t *testing.T) {
	s := tierState()
	from := w.WorkspaceID("Q")
	to := w.WorkspaceID("1")
	win := w.LiveWindowID("live-1")
	r, err := ReactToEvent(s, event.Event{
		Kind: event.KindUserMovedWindow,
		Data: event.Data{Window: &win, Workspace: &from, TargetWS: &to},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.NewCards) != 1 || r.NewCards[0].Type != w.CardTypeMoved {
		t.Errorf("expected one MOVED card, got %+v", r.NewCards)
	}
}

func TestReactToEvent_Tier4_ClosedCardEmit(t *testing.T) {
	s := tierState()
	win := w.LiveWindowID("live-1")
	ws := w.WorkspaceID("Q")
	r, err := ReactToEvent(s, event.Event{
		Kind: event.KindUserClosedWindow,
		Data: event.Data{Window: &win, Workspace: &ws},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.NewCards) != 1 || r.NewCards[0].Type != w.CardTypeClosed {
		t.Errorf("expected one CLOSED card, got %+v", r.NewCards)
	}
}
