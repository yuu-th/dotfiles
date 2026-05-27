package world

import "testing"

func TestValidateUnknownActiveProfile(t *testing.T) {
	w := NewDesiredWorld()
	w.ActiveProfile = "missing"
	if err := Validate(w); err == nil {
		t.Fatal("expected unknown active profile to be rejected")
	}
}

func TestValidateAssignmentToUnknownProject(t *testing.T) {
	w := NewDesiredWorld()
	w.Profiles["work"] = Profile{Assignments: map[string]string{"Q": "ghost"}}
	w.ActiveProfile = "work"
	if err := Validate(w); err == nil {
		t.Fatal("expected unknown project assignment to be rejected")
	}
}

func TestValidateArchivedProjectInActiveProfile(t *testing.T) {
	w := NewDesiredWorld()
	w.Projects["x"] = Project{CWD: "/x", Archived: true}
	w.Profiles["work"] = Profile{Assignments: map[string]string{"Q": "x"}}
	w.ActiveProfile = "work"
	if err := Validate(w); err == nil {
		t.Fatal("expected archived active project to be rejected")
	}
}

func TestValidateDuplicateSlotProject(t *testing.T) {
	w := NewDesiredWorld()
	w.Projects["x"] = Project{CWD: "/x"}
	w.Profiles["work"] = Profile{Assignments: map[string]string{"Q": "x", "W": "x"}}
	w.ActiveProfile = "work"
	if err := Validate(w); err == nil {
		t.Fatal("expected duplicate project assignment to be rejected")
	}
}

func TestValidateAIRequiresAIField(t *testing.T) {
	w := NewDesiredWorld()
	w.Projects["x"] = Project{CWD: "/x", Windows: []Window{{ID: 1, Kind: KindAI}}}
	if err := Validate(w); err == nil {
		t.Fatal("expected ai window without ai field to be rejected")
	}
}

func TestValidateNonAIMustNotHaveAIField(t *testing.T) {
	w := NewDesiredWorld()
	w.Projects["x"] = Project{CWD: "/x", Windows: []Window{{ID: 1, Kind: KindShell, AI: AIClaude}}}
	if err := Validate(w); err == nil {
		t.Fatal("expected non-ai window with ai field to be rejected")
	}
}

func TestValidateDuplicateKindID(t *testing.T) {
	w := NewDesiredWorld()
	w.Projects["x"] = Project{CWD: "/x", Windows: []Window{
		{ID: 1, Kind: KindAI, AI: AIClaude},
		{ID: 1, Kind: KindAI, AI: AICopilot},
	}}
	if err := Validate(w); err == nil {
		t.Fatal("expected duplicate kind/id to be rejected")
	}
}

func TestValidateBasenameCollisionInActiveProfile(t *testing.T) {
	w := NewDesiredWorld()
	w.Projects["a-dotfiles"] = Project{CWD: "/a/dotfiles"}
	w.Projects["b-dotfiles"] = Project{CWD: "/b/dotfiles"}
	w.Profiles["work"] = Profile{Assignments: map[string]string{"Q": "a-dotfiles", "W": "b-dotfiles"}}
	w.ActiveProfile = "work"
	if err := Validate(w); err == nil {
		t.Fatal("expected basename collision to be rejected")
	}
}

func TestNextWindowIDDoesNotReuseGaps(t *testing.T) {
	p := Project{Windows: []Window{
		{ID: 1, Kind: KindAI, AI: AIClaude},
		{ID: 3, Kind: KindAI, AI: AICopilot},
		{ID: 1, Kind: KindShell},
	}}
	if got := NextWindowID(p, KindAI); got != 4 {
		t.Fatalf("NextWindowID(ai) = %d, want 4", got)
	}
	if got := NextWindowID(p, KindShell); got != 2 {
		t.Fatalf("NextWindowID(shell) = %d, want 2", got)
	}
}

func TestIsParked(t *testing.T) {
	w := NewDesiredWorld()
	w.Projects["assigned"] = Project{CWD: "/a"}
	w.Projects["parked"] = Project{CWD: "/p"}
	w.Projects["archived"] = Project{CWD: "/ar", Archived: true}
	w.Profiles["work"] = Profile{Assignments: map[string]string{"Q": "assigned"}}
	w.ActiveProfile = "work"
	if IsParked(w, "assigned") {
		t.Fatal("assigned project must not be parked")
	}
	if !IsParked(w, "parked") {
		t.Fatal("unassigned non-archived project should be parked")
	}
	if IsParked(w, "archived") {
		t.Fatal("archived project must not be parked")
	}
}

func TestSortedWindows(t *testing.T) {
	p := Project{Windows: []Window{
		{ID: 1, Kind: KindEditor},
		{ID: 2, Kind: KindAI, AI: AIClaude},
		{ID: 1, Kind: KindShell},
		{ID: 1, Kind: KindAI, AI: AIClaude},
	}}
	got := SortedWindows(p)
	want := []struct {
		kind Kind
		id   int
	}{
		{KindAI, 1},
		{KindAI, 2},
		{KindShell, 1},
		{KindEditor, 1},
	}
	for i, w := range want {
		if got[i].Kind != w.kind || got[i].ID != w.id {
			t.Fatalf("sorted[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestUserVisibleNames(t *testing.T) {
	if got := GhosttyTitle(KindAI, 1, "dotfiles"); got != "ai-1:dotfiles" {
		t.Fatalf("GhosttyTitle = %q", got)
	}
	if got := GhosttyTitle(KindShell, 1, "dotfiles"); got != "shell-1:dotfiles" {
		t.Fatalf("GhosttyTitle shell = %q", got)
	}
	if got := TmuxSession(KindAI, 1, "dotfiles"); got != "ai-1/dotfiles" {
		t.Fatalf("TmuxSession = %q", got)
	}
	if got := ViewerGhosttyTitle(2, "dotfiles"); got != "ai-view-2:dotfiles" {
		t.Fatalf("ViewerGhosttyTitle = %q", got)
	}
	if got := ViewerTmuxSession(2, "dotfiles"); got != "ai-2/dotfiles_v" {
		t.Fatalf("ViewerTmuxSession = %q", got)
	}
	if got := ZedTitle("/Users/yuta/dev/dotfiles/"); got != "dotfiles" {
		t.Fatalf("ZedTitle = %q", got)
	}
}

func TestEditorHasNoGhosttyOrTmuxIdentity(t *testing.T) {
	assertPanics(t, func() { GhosttyTitle(KindEditor, 1, "x") })
	assertPanics(t, func() { TmuxSession(KindEditor, 1, "x") })
}

func assertPanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}
