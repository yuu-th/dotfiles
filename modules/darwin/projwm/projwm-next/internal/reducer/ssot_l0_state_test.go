package reducer

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestSSOTDefaultProjectWindowsUseOneBasedStableIDsAndCanonicalTitles(t *testing.T) {
	windows := DefaultProjectWindows("dotfiles", "claude")
	want := map[w.WindowKind]struct {
		index int
		title string
		app   string
	}{
		w.WindowAI:     {index: 1, title: "ai-1:dotfiles", app: "com.mitchellh.ghostty"},
		w.WindowShell:  {index: 1, title: "shell-1:dotfiles", app: "com.mitchellh.ghostty"},
		w.WindowEditor: {index: 1, title: "dotfiles", app: "dev.zed.Zed"},
	}
	if len(windows) != len(want) {
		t.Fatalf("DefaultProjectWindows produced %d windows, want %d", len(windows), len(want))
	}
	for _, win := range windows {
		spec, ok := want[win.Kind]
		if !ok {
			t.Fatalf("unexpected default window kind %q: %+v", win.Kind, win)
		}
		if win.ID.Project != "dotfiles" || win.ID.Kind != win.Kind || win.ID.Index != spec.index {
			t.Fatalf("default %s identity = %+v, want project=dotfiles kind=%s index=%d", win.Kind, win.ID, win.Kind, spec.index)
		}
		if win.TitleContract.Expected != spec.title {
			t.Fatalf("default %s title = %q, want %q", win.Kind, win.TitleContract.Expected, spec.title)
		}
		if win.App.BundleID != spec.app {
			t.Fatalf("default %s bundle = %q, want %q", win.Kind, win.App.BundleID, spec.app)
		}
	}
}

func TestSSOTWindowIndexAllocationNeverReusesDeletedIDs(t *testing.T) {
	project := w.ProjectID("dotfiles")
	windows := []w.DesiredWindow{
		defaultWindowForKind(project, w.WindowShell, 1, ""),
		defaultWindowForKind(project, w.WindowShell, 3, ""),
	}
	if got := nextWindowIndex(windows, w.WindowShell); got != 4 {
		t.Fatalf("nextWindowIndex with a hole = %d, want max+1=4", got)
	}
}

func TestSSOTUnarchiveProjectReturnsToParkStateWithoutSlotAssignment(t *testing.T) {
	state := w.WorldState{
		Desired: w.DesiredWorld{
			ActiveProfile: "work",
			Profiles: map[w.ProfileID]w.DesiredProfile{
				"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "dotfiles"}},
			},
			Projects: map[w.ProjectID]w.DesiredProject{
				"dotfiles": {ID: "dotfiles"},
				"archived": {ID: "archived", Archived: true},
			},
		},
	}
	got, err := ReduceIntent(state, intent.UnarchiveProject{Project: "archived"})
	if err != nil {
		t.Fatalf("ReduceIntent(UnarchiveProject): %v", err)
	}
	if got.Projects["archived"].Archived {
		t.Fatal("unarchive should clear Archived")
	}
	for slot, project := range got.Profiles["work"].Assignments {
		if project == "archived" {
			t.Fatalf("SSOT §4.5 says unarchive returns to park state; got assignment %s=%s", slot, project)
		}
	}
}

func TestSSOTProfileSlotAssignmentIsExclusive(t *testing.T) {
	state := w.WorldState{
		Desired: w.DesiredWorld{
			ActiveProfile: "work",
			Profiles: map[w.ProfileID]w.DesiredProfile{
				"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "dotfiles"}},
			},
			Projects: map[w.ProjectID]w.DesiredProject{
				"dotfiles": {ID: "dotfiles"},
				"manaflow": {ID: "manaflow"},
			},
		},
	}
	got, err := ReduceIntent(state, intent.AssignProject{Slot: "Q", Project: "manaflow"})
	if err != nil {
		t.Fatalf("ReduceIntent(AssignProject): %v", err)
	}
	assignments := got.Profiles["work"].Assignments
	if len(assignments) != 1 || assignments["Q"] != "manaflow" {
		t.Fatalf("SSOT INV-08 requires one project per slot; assignments=%+v", assignments)
	}
}
