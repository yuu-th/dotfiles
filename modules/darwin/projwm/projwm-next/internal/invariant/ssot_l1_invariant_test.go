package invariant

import (
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §2.5 EC4 / §3.4 INV-01: when two live windows match the same desired
// identity (project/kind/index), Check14 fires so the controller emits a
// [INVARIANT] cockpit card. Only counts same-Kind matches (viewer pairing
// with AI is not a duplicate).
func TestSSOTInvariantCheck14DuplicateWindowFires(t *testing.T) {
	did := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	state := w.WorldState{
		Desired: w.DesiredWorld{
			Projects: map[w.ProjectID]w.DesiredProject{
				"p1": {ID: "p1"},
			},
		},
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"live-a": {ID: "live-a", Kind: w.WindowShell, MatchedTo: &did},
				"live-b": {ID: "live-b", Kind: w.WindowShell, MatchedTo: &did},
			},
		},
	}
	v := Check14DuplicateWindow(state)
	if v == nil {
		t.Fatal("Check14 expected to fire on two live shell windows with same MatchedTo")
	}
	if v.ID != 14 {
		t.Errorf("violation ID = %d, want 14", v.ID)
	}
}

func TestSSOTInvariantCheck14SkipsViewerPairing(t *testing.T) {
	aiID := w.DesiredWindowID{Project: "p1", Kind: w.WindowAI, Index: 1}
	// Viewer convention: MatchedTo points to the AI's DesiredWindowID but
	// observed.Kind = WindowViewer. Should NOT count as duplicate of the AI.
	state := w.WorldState{
		Desired: w.DesiredWorld{
			Projects: map[w.ProjectID]w.DesiredProject{"p1": {ID: "p1"}},
		},
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"ai-1":    {ID: "ai-1", Kind: w.WindowAI, MatchedTo: &aiID},
				"viewer-1": {ID: "viewer-1", Kind: w.WindowViewer, MatchedTo: &aiID},
			},
		},
	}
	if v := Check14DuplicateWindow(state); v != nil {
		t.Fatalf("Check14 fired for viewer-pairing (should not be duplicate): %s", v.Message)
	}
}

// SSOT §3.4 INV-06: cockpit live window must always be on its ParkWorkspace.
// Check15 fires when observed cockpit window drifted off ParkWorkspace.
func TestSSOTInvariantCheck15CockpitOffParkWorkspaceFires(t *testing.T) {
	state := w.WorldState{
		Desired: w.DesiredWorld{
			SystemWindows: []w.SystemWindow{{
				ID:            w.SystemWindowID{Kind: w.WindowCockpit, Index: 0},
				Kind:          w.WindowCockpit,
				Title:         "projwm-cockpit-0",
				ParkWorkspace: "CP1",
			}},
		},
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"cockpit-1": {
					ID:        "cockpit-1",
					Kind:      w.WindowCockpit,
					Title:     w.ObservedTitle{Value: "projwm-cockpit-0"},
					Workspace: "8", // drifted off CP1
				},
			},
		},
	}
	v := Check15CockpitOnParkWorkspace(state)
	if v == nil {
		t.Fatal("Check15 expected to fire on cockpit not on ParkWorkspace")
	}
	if v.ID != 15 {
		t.Errorf("ID = %d, want 15", v.ID)
	}
}

func TestSSOTInvariantCheck15CockpitOnParkWorkspaceIsSilent(t *testing.T) {
	state := w.WorldState{
		Desired: w.DesiredWorld{
			SystemWindows: []w.SystemWindow{{
				ID:            w.SystemWindowID{Kind: w.WindowCockpit, Index: 0},
				Kind:          w.WindowCockpit,
				Title:         "projwm-cockpit-0",
				ParkWorkspace: "CP1",
			}},
		},
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"cockpit-1": {
					ID:        "cockpit-1",
					Kind:      w.WindowCockpit,
					Title:     w.ObservedTitle{Value: "projwm-cockpit-0"},
					Workspace: "CP1", // on park
				},
			},
		},
	}
	if v := Check15CockpitOnParkWorkspace(state); v != nil {
		t.Fatalf("Check15 fired when cockpit is on park workspace: %s", v.Message)
	}
}

func TestSSOTInvariantCheck14SkipsArchivedProject(t *testing.T) {
	did := w.DesiredWindowID{Project: "archived", Kind: w.WindowShell, Index: 1}
	state := w.WorldState{
		Desired: w.DesiredWorld{
			Projects: map[w.ProjectID]w.DesiredProject{"archived": {ID: "archived", Archived: true}},
		},
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"a": {ID: "a", Kind: w.WindowShell, MatchedTo: &did},
				"b": {ID: "b", Kind: w.WindowShell, MatchedTo: &did},
			},
		},
	}
	// archived の duplicate は Check5 が close 指示するので Check14 は静か。
	if v := Check14DuplicateWindow(state); v != nil {
		t.Fatalf("Check14 fired for archived project (Check5 handles): %s", v.Message)
	}
}

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
