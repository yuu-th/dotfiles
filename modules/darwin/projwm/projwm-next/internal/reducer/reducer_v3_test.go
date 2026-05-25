package reducer

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// baseState produces a small but realistic WorldState for v3 intent tests.
func baseState() w.WorldState {
	return w.WorldState{
		Desired: w.DesiredWorld{
			ActiveProfile: "work",
			Profiles: map[w.ProfileID]w.DesiredProfile{
				"work": {
					ID:             "work",
					InactivePolicy: w.InactivePolicyRemove,
					Assignments: map[w.SlotID]w.ProjectID{
						"Q": "dotfiles",
					},
				},
				"misc": {
					ID:             "misc",
					InactivePolicy: w.InactivePolicyKeep,
					Assignments:    map[w.SlotID]w.ProjectID{},
				},
			},
			Projects: map[w.ProjectID]w.DesiredProject{
				"dotfiles": {
					ID: "dotfiles",
					Windows: []w.DesiredWindow{
						{ID: w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowShell, Index: 1}, Kind: w.WindowShell},
					},
				},
			},
		},
	}
}

func TestReduceIntent_CreateProject_Default(t *testing.T) {
	s := baseState()
	d, err := ReduceIntent(s, intent.CreateProject{ID: "newproj", Path: "/tmp/newproj"})
	if err != nil {
		t.Fatal(err)
	}
	p, ok := d.Projects["newproj"]
	if !ok {
		t.Fatal("project missing")
	}
	if p.Root != "/tmp/newproj" {
		t.Errorf("Root = %q", p.Root)
	}
	if len(p.Windows) != 3 {
		t.Errorf("expected 3 default windows, got %d", len(p.Windows))
	}
}

func TestReduceIntent_CreateProject_RejectsDuplicate(t *testing.T) {
	s := baseState()
	_, err := ReduceIntent(s, intent.CreateProject{ID: "dotfiles"})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestReduceIntent_CreateProject_RewritesWindowProject(t *testing.T) {
	s := baseState()
	mismatched := []w.DesiredWindow{
		{ID: w.DesiredWindowID{Project: "wrong", Kind: w.WindowShell, Index: 1}, Kind: w.WindowShell},
	}
	d, err := ReduceIntent(s, intent.CreateProject{ID: "p1", Windows: mismatched})
	if err != nil {
		t.Fatal(err)
	}
	if d.Projects["p1"].Windows[0].ID.Project != "p1" {
		t.Errorf("window project not rewritten: %s", d.Projects["p1"].Windows[0].ID.Project)
	}
}

func TestReduceIntent_DeleteProject_RemovesAssignments(t *testing.T) {
	s := baseState()
	d, err := ReduceIntent(s, intent.DeleteProject{ID: "dotfiles"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d.Projects["dotfiles"]; ok {
		t.Error("project not removed")
	}
	if _, ok := d.Profiles["work"].Assignments["Q"]; ok {
		t.Error("assignment not removed")
	}
}

func TestReduceIntent_AddWindow_AutoIndex(t *testing.T) {
	s := baseState()
	d, err := ReduceIntent(s, intent.AddWindow{Project: "dotfiles", WindowKind: w.WindowShell})
	if err != nil {
		t.Fatal(err)
	}
	// shell-1 existed; shell-2 added
	found := false
	for _, win := range d.Projects["dotfiles"].Windows {
		if win.Kind == w.WindowShell && win.ID.Index == 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected shell-2, got windows %+v", d.Projects["dotfiles"].Windows)
	}
}

func TestReduceIntent_AddWindow_AIRequiresName(t *testing.T) {
	s := baseState()
	d, err := ReduceIntent(s, intent.AddWindow{Project: "dotfiles", WindowKind: w.WindowAI, AIName: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	added := false
	for _, win := range d.Projects["dotfiles"].Windows {
		if win.Kind == w.WindowAI && win.ID.Index == 1 {
			if win.TitleContract.Authority != w.TitleControllerOwned {
				t.Errorf("authority = %q, want controller-owned", win.TitleContract.Authority)
			}
			if win.TitleContract.Expected != "ai-1:dotfiles" {
				t.Errorf("title expected = %q", win.TitleContract.Expected)
			}
			added = true
		}
	}
	if !added {
		t.Error("ai-1 not added")
	}
}

func TestReduceIntent_RemoveWindow(t *testing.T) {
	s := baseState()
	d, err := ReduceIntent(s, intent.RemoveWindow{
		Project:  "dotfiles",
		WindowID: w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowShell, Index: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Projects["dotfiles"].Windows) != 0 {
		t.Errorf("expected 0 windows, got %d", len(d.Projects["dotfiles"].Windows))
	}
}

func TestReduceIntent_CreateProfile(t *testing.T) {
	s := baseState()
	d, err := ReduceIntent(s, intent.CreateProfile{
		ID:             "spike",
		Description:    "experiments",
		InactivePolicy: w.InactivePolicyKeep,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, ok := d.Profiles["spike"]
	if !ok {
		t.Fatal("profile not added")
	}
	if p.InactivePolicy != w.InactivePolicyKeep {
		t.Errorf("policy = %q", p.InactivePolicy)
	}
}

func TestReduceIntent_DeleteProfile_RefusesActive(t *testing.T) {
	s := baseState()
	_, err := ReduceIntent(s, intent.DeleteProfile{ID: "work"})
	if err == nil || err.Error() == "" {
		t.Errorf("expected error deleting active profile, got %v", err)
	}
}

func TestReduceIntent_RenameProfile_UpdatesActive(t *testing.T) {
	s := baseState()
	d, err := ReduceIntent(s, intent.RenameProfile{Old: "work", New: "primary"})
	if err != nil {
		t.Fatal(err)
	}
	if d.ActiveProfile != "primary" {
		t.Errorf("ActiveProfile = %q", d.ActiveProfile)
	}
	if _, gone := d.Profiles["work"]; gone {
		t.Error("old profile still present")
	}
}

func TestReduceIntent_AutoSyncLayout(t *testing.T) {
	s := baseState()
	cols := []w.DesiredColumn{
		{Windows: []w.DesiredWindowID{{Project: "dotfiles", Kind: w.WindowShell, Index: 1}}, Mode: w.ColumnSolo},
	}
	d, err := ReduceIntent(s, intent.AutoSyncLayout{
		Project:   "dotfiles",
		Workspace: "Q",
		Columns:   cols,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := d.AcceptedLayouts["dotfiles"]["Q"]
	if got.Source != w.LayoutAuthorityAcceptedManual {
		t.Errorf("Source = %q", got.Source)
	}
	if len(got.Columns) != 1 {
		t.Errorf("Columns len = %d", len(got.Columns))
	}
}

// SSOT: SyncBrowserTabs is removed in favor of the granular
// BrowserAddTab / BrowserRemoveTab / BrowserChangeTabURL /
// BrowserReorderTabs intents (SSOT §4.1 OP14-17). The test will be
// rebuilt as part of S14 (browser tab CRUD) once the reducer handlers
// for those intents land.

func TestReduceIntent_DismissCard_NoMutation(t *testing.T) {
	s := baseState()
	d, err := ReduceIntent(s, intent.DismissCard{CardID: "C1"})
	if err != nil {
		t.Fatal(err)
	}
	if d.ActiveProfile != "work" {
		t.Errorf("DismissCard should not mutate DesiredWorld, ActiveProfile = %q", d.ActiveProfile)
	}
}
