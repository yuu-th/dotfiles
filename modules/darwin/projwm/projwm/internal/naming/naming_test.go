package naming

import "testing"

func TestGhosttyTitle(t *testing.T) {
	cases := []struct {
		kind    Kind
		id      int
		project string
		want    string
	}{
		{KindAI, 1, "dotfiles", "ai-1:dotfiles"},
		{KindAI, 7, "manaflow", "ai-7:manaflow"},
		{KindShell, 1, "dotfiles", "shell-1:dotfiles"},
	}
	for _, c := range cases {
		got := GhosttyTitle(c.kind, c.id, c.project)
		if got != c.want {
			t.Errorf("GhosttyTitle(%q,%d,%q) = %q, want %q", c.kind, c.id, c.project, got, c.want)
		}
	}
}

func TestGhosttyTitleEditorPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for editor kind")
		}
	}()
	GhosttyTitle(KindEditor, 1, "x")
}

func TestTmuxSession(t *testing.T) {
	if got := TmuxSession(KindAI, 1, "dotfiles"); got != "ai-1/dotfiles" {
		t.Errorf("TmuxSession AI: got %q", got)
	}
	if got := TmuxSession(KindShell, 3, "blog"); got != "shell-3/blog" {
		t.Errorf("TmuxSession shell: got %q", got)
	}
}

func TestViewerNames(t *testing.T) {
	if got := ViewerGhosttyTitle(2, "dotfiles"); got != "ai-view-2:dotfiles" {
		t.Errorf("ViewerGhosttyTitle: got %q", got)
	}
	// tmux 内では `:` 不可 → `_v` 末尾（v11.2）
	if got := ViewerTmuxSession(2, "dotfiles"); got != "ai-2/dotfiles_v" {
		t.Errorf("ViewerTmuxSession: got %q", got)
	}
}

func TestZedTitle(t *testing.T) {
	cases := map[string]string{
		"/Users/yuta/dev/dotfiles":  "dotfiles",
		"/tmp/proj-alpha":           "proj-alpha",
		"/tmp/proj-alpha/":          "proj-alpha", // trailing slash
		"relative/path/foo":         "foo",
	}
	for cwd, want := range cases {
		if got := ZedTitle(cwd); got != want {
			t.Errorf("ZedTitle(%q) = %q, want %q", cwd, got, want)
		}
	}
}

func TestIsValidKind(t *testing.T) {
	for _, k := range []Kind{KindAI, KindShell, KindEditor} {
		if !IsValidKind(k) {
			t.Errorf("IsValidKind(%q) = false, want true", k)
		}
	}
	for _, k := range []Kind{Kind(""), Kind("term"), Kind("nvim")} {
		if IsValidKind(k) {
			t.Errorf("IsValidKind(%q) = true, want false", k)
		}
	}
}
