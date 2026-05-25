package invariant

import (
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestViewerInvariantUsesTitleContractNotDesiredIndex(t *testing.T) {
	aiID := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowAI, Index: 1}
	viewerID := w.LiveWindowID("viewer")
	state := w.WorldState{
		Environment: w.ManagedEnvironment{
			Workspaces: w.WorkspaceEnvironment{
				Viewer:     "A",
				Slots:      []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
				Workspaces: []w.WorkspaceSpec{{ID: "A", Role: w.WorkspaceViewer}, {ID: "Q", Role: w.WorkspaceProject}},
			},
		},
		Desired: w.DesiredWorld{
			ActiveProfile: "work",
			Profiles: map[w.ProfileID]w.DesiredProfile{
				"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "dotfiles"}},
			},
			Projects: map[w.ProjectID]w.DesiredProject{
				"dotfiles": {ID: "dotfiles", Windows: []w.DesiredWindow{{
					ID:   aiID,
					Kind: w.WindowAI,
					TitleContract: w.TitleContract{
						Authority: w.TitleControllerOwned,
						Expected:  "ai-1:dotfiles",
					},
				}}},
			},
		},
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				viewerID: {ID: viewerID, Kind: w.WindowViewer, Workspace: "A", Title: w.ObservedTitle{Value: "ai-view-1:dotfiles"}},
			},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{
				"A": {Workspace: "A", Columns: []w.ObservedColumn{{Windows: []w.LiveWindowID{viewerID}}}},
			},
		},
	}
	if v := Check7ViewerSet(state); v != nil {
		t.Fatalf("Check7ViewerSet = %v", v)
	}
	if v := Check8ViewerOrder(state); v != nil {
		t.Fatalf("Check8ViewerOrder = %v", v)
	}
}
