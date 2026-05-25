package invariant

import (
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestSSOTInvariantActiveProfileMustExist(t *testing.T) {
	state := w.WorldState{
		Desired: w.DesiredWorld{
			ActiveProfile: "missing",
			Profiles:      map[w.ProfileID]w.DesiredProfile{"work": {ID: "work"}},
		},
	}
	if v := Check2ActiveProfile(state); v == nil {
		t.Fatal("SSOT INV-09 requires violation when active profile key is missing")
	}
}

func TestSSOTInvariantArchivedProjectMustHaveNoLiveWindows(t *testing.T) {
	archivedID := w.DesiredWindowID{Project: "old", Kind: w.WindowShell, Index: 1}
	state := w.WorldState{
		Desired: w.DesiredWorld{
			Projects: map[w.ProjectID]w.DesiredProject{
				"old": {ID: "old", Archived: true},
			},
		},
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"live-old": {ID: "live-old", Kind: w.WindowShell, MatchedTo: &archivedID},
			},
		},
	}
	if v := Check5ArchivedAbsent(state); v == nil {
		t.Fatal("SSOT INV-04 requires violation when archived project has a live matched window")
	}
}

func TestSSOTInvariantViewerShowsOnlyActiveProfileAIStreams(t *testing.T) {
	state := ssotViewerState()
	stale := w.DesiredWindowID{Project: "personal", Kind: w.WindowAI, Index: 1}
	state.Observed.Windows["viewer-personal"] = w.ObservedWindow{
		ID:        "viewer-personal",
		Kind:      w.WindowViewer,
		Workspace: "A",
		Title:     w.ObservedTitle{Value: "ai-view-1:personal"},
		MatchedTo: &stale,
	}
	state.Observed.Layouts["A"] = w.ObservedLayout{
		Workspace: "A",
		Columns: []w.ObservedColumn{
			{Windows: []w.LiveWindowID{"viewer-dotfiles"}},
			{Windows: []w.LiveWindowID{"viewer-manaflow"}},
			{Windows: []w.LiveWindowID{"viewer-personal"}},
		},
	}

	if v := Check7ViewerSet(state); v == nil {
		t.Fatal("SSOT INV-05 requires stale inactive-profile viewer streams to be rejected")
	}
}

func TestSSOTInvariantViewerOrderFollowsSlotOrder(t *testing.T) {
	state := ssotViewerState()
	state.Observed.Layouts["A"] = w.ObservedLayout{
		Workspace: "A",
		Columns: []w.ObservedColumn{
			{Windows: []w.LiveWindowID{"viewer-manaflow"}},
			{Windows: []w.LiveWindowID{"viewer-dotfiles"}},
		},
	}

	if v := Check8ViewerOrder(state); v == nil {
		t.Fatal("SSOT INV-12 requires viewer columns to follow active profile slot order")
	}
}

func ssotViewerState() w.WorldState {
	dotfilesAI := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowAI, Index: 1}
	manaflowAI := w.DesiredWindowID{Project: "manaflow", Kind: w.WindowAI, Index: 1}
	return w.WorldState{
		Environment: w.ManagedEnvironment{
			SchemaVersion: 1,
			Authority:     "nix",
			Workspaces: w.WorkspaceEnvironment{
				Viewer: "A",
				Slots: []w.SlotSpec{
					{ID: "Q", Workspace: "Q", Order: 1},
					{ID: "W", Workspace: "W", Order: 2},
				},
				Workspaces: []w.WorkspaceSpec{
					{ID: "A", Role: w.WorkspaceViewer},
					{ID: "Q", Role: w.WorkspaceProject},
					{ID: "W", Role: w.WorkspaceProject},
				},
			},
		},
		Desired: w.DesiredWorld{
			ActiveProfile: "work",
			Profiles: map[w.ProfileID]w.DesiredProfile{
				"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "dotfiles", "W": "manaflow"}},
			},
			Projects: map[w.ProjectID]w.DesiredProject{
				"dotfiles": {
					ID: "dotfiles",
					Windows: []w.DesiredWindow{{
						ID:   dotfilesAI,
						Kind: w.WindowAI,
						TitleContract: w.TitleContract{
							Authority: w.TitleControllerOwned,
							Expected:  "ai-1:dotfiles",
						},
					}},
				},
				"manaflow": {
					ID: "manaflow",
					Windows: []w.DesiredWindow{{
						ID:   manaflowAI,
						Kind: w.WindowAI,
						TitleContract: w.TitleContract{
							Authority: w.TitleControllerOwned,
							Expected:  "ai-1:manaflow",
						},
					}},
				},
			},
		},
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"viewer-dotfiles": {
					ID:        "viewer-dotfiles",
					Kind:      w.WindowViewer,
					Workspace: "A",
					Title:     w.ObservedTitle{Value: "ai-view-1:dotfiles"},
					MatchedTo: &dotfilesAI,
				},
				"viewer-manaflow": {
					ID:        "viewer-manaflow",
					Kind:      w.WindowViewer,
					Workspace: "A",
					Title:     w.ObservedTitle{Value: "ai-view-1:manaflow"},
					MatchedTo: &manaflowAI,
				},
			},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{
				"A": {
					Workspace: "A",
					Columns: []w.ObservedColumn{
						{Windows: []w.LiveWindowID{"viewer-dotfiles"}},
						{Windows: []w.LiveWindowID{"viewer-manaflow"}},
					},
				},
			},
		},
	}
}
