package state

import (
	"path/filepath"
	"testing"

	"github.com/yuu-th/projwm/internal/naming"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	p := Paths{
		Dir:        dir,
		StateFile:  filepath.Join(dir, "state.json"),
		BackupFile: filepath.Join(dir, "state.json.bak"),
		LockFile:   filepath.Join(dir, "lock"),
		LogsDir:    filepath.Join(dir, "logs"),
	}
	return NewStore(p)
}

func TestLoadEmpty(t *testing.T) {
	s := newStore(t)
	st, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.ActiveProfile != "" || len(st.Profiles) != 0 || len(st.Projects) != 0 {
		t.Errorf("empty state expected, got %+v", st)
	}
}

func TestMutateAndReload(t *testing.T) {
	s := newStore(t)
	err := s.Mutate(func(st *State) error {
		st.Projects["dotfiles"] = Project{
			CWD:      "/Users/yuta/dev/dotfiles",
			Archived: false,
			Windows: []Window{
				{ID: 1, Kind: naming.KindAI, AI: naming.AIClaude},
				{ID: 1, Kind: naming.KindShell},
				{ID: 1, Kind: naming.KindEditor},
			},
		}
		st.Profiles["work"] = Profile{
			Assignments: map[string]string{"Q": "dotfiles"},
		}
		st.ActiveProfile = "work"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	st2, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st2.ActiveProfile != "work" {
		t.Errorf("active=%q", st2.ActiveProfile)
	}
	p := st2.Projects["dotfiles"]
	if len(p.Windows) != 3 {
		t.Errorf("windows=%d", len(p.Windows))
	}
}

func TestValidateUnknownActive(t *testing.T) {
	st := New()
	st.ActiveProfile = "nonexistent"
	if err := Validate(st); err == nil {
		t.Error("expected validation error")
	}
}

func TestValidateAssignmentToUnknownProject(t *testing.T) {
	st := New()
	st.Profiles["work"] = Profile{Assignments: map[string]string{"Q": "ghost"}}
	st.ActiveProfile = "work"
	if err := Validate(st); err == nil {
		t.Error("expected error for unknown project")
	}
}

func TestValidateArchivedInActive(t *testing.T) {
	st := New()
	st.Projects["x"] = Project{CWD: "/x", Archived: true}
	st.Profiles["w"] = Profile{Assignments: map[string]string{"Q": "x"}}
	st.ActiveProfile = "w"
	if err := Validate(st); err == nil {
		t.Error("expected error: archived in active profile")
	}
}

func TestValidateDupSlotProject(t *testing.T) {
	st := New()
	st.Projects["x"] = Project{CWD: "/x"}
	st.Profiles["w"] = Profile{Assignments: map[string]string{"Q": "x", "W": "x"}}
	st.ActiveProfile = "w"
	if err := Validate(st); err == nil {
		t.Error("expected error: same project in 2 slots within profile")
	}
}

func TestValidateAIRequiresField(t *testing.T) {
	st := New()
	st.Projects["x"] = Project{
		CWD: "/x",
		Windows: []Window{
			{ID: 1, Kind: naming.KindAI}, // missing AI
		},
	}
	if err := Validate(st); err == nil {
		t.Error("expected error: ai window without ai field")
	}
}

func TestValidateNonAIHasNoAIField(t *testing.T) {
	st := New()
	st.Projects["x"] = Project{
		CWD: "/x",
		Windows: []Window{
			{ID: 1, Kind: naming.KindShell, AI: naming.AIClaude},
		},
	}
	if err := Validate(st); err == nil {
		t.Error("expected error: non-ai window with ai field")
	}
}

func TestValidateDupKindID(t *testing.T) {
	st := New()
	st.Projects["x"] = Project{
		CWD: "/x",
		Windows: []Window{
			{ID: 1, Kind: naming.KindAI, AI: naming.AIClaude},
			{ID: 1, Kind: naming.KindAI, AI: naming.AICopilot},
		},
	}
	if err := Validate(st); err == nil {
		t.Error("expected error: dup (kind,id)")
	}
}

func TestValidateBasenameCollision(t *testing.T) {
	st := New()
	st.Projects["a-dotfiles"] = Project{CWD: "/a/dotfiles"}
	st.Projects["b-dotfiles"] = Project{CWD: "/b/dotfiles"}
	st.Profiles["w"] = Profile{Assignments: map[string]string{"Q": "a-dotfiles", "W": "b-dotfiles"}}
	st.ActiveProfile = "w"
	if err := Validate(st); err == nil {
		t.Error("expected error: basename collision")
	}
}

func TestNextWindowID(t *testing.T) {
	p := Project{Windows: []Window{
		{ID: 1, Kind: naming.KindAI, AI: naming.AIClaude},
		{ID: 3, Kind: naming.KindAI, AI: naming.AICopilot},
		{ID: 1, Kind: naming.KindShell},
	}}
	if got := NextWindowID(p, naming.KindAI); got != 4 {
		t.Errorf("NextWindowID AI = %d, want 4 (max+1, gaps not reused)", got)
	}
	if got := NextWindowID(p, naming.KindShell); got != 2 {
		t.Errorf("NextWindowID shell = %d, want 2", got)
	}
	if got := NextWindowID(p, naming.KindEditor); got != 1 {
		t.Errorf("NextWindowID editor = %d, want 1 (no existing)", got)
	}
}

func TestIsParked(t *testing.T) {
	st := New()
	st.Projects["assigned"] = Project{CWD: "/a"}
	st.Projects["parked"] = Project{CWD: "/p"}
	st.Projects["archived"] = Project{CWD: "/ar", Archived: true}
	st.Profiles["w"] = Profile{Assignments: map[string]string{"Q": "assigned"}}
	st.ActiveProfile = "w"
	if IsParked(st, "assigned") {
		t.Error("assigned should not be parked")
	}
	if !IsParked(st, "parked") {
		t.Error("parked should be parked")
	}
	if IsParked(st, "archived") {
		t.Error("archived should not be reported as parked")
	}
}

func TestSortedWindows(t *testing.T) {
	p := Project{Windows: []Window{
		{ID: 1, Kind: naming.KindEditor},
		{ID: 2, Kind: naming.KindAI, AI: naming.AIClaude},
		{ID: 1, Kind: naming.KindShell},
		{ID: 1, Kind: naming.KindAI, AI: naming.AIClaude},
	}}
	got := SortedWindows(p)
	want := []struct {
		kind naming.Kind
		id   int
	}{
		{naming.KindAI, 1},
		{naming.KindAI, 2},
		{naming.KindShell, 1},
		{naming.KindEditor, 1},
	}
	for i, w := range want {
		if got[i].Kind != w.kind || got[i].ID != w.id {
			t.Errorf("[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}
