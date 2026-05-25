package naming

import "testing"

func TestGhosttyTitleAndSession(t *testing.T) {
	cases := []struct {
		kind    Kind
		id      int
		project string
		title   string
		session string
	}{
		{KindAI, 1, "dotfiles", "ai-1:dotfiles", "ai-1/dotfiles"},
		{KindShell, 2, "dotfiles", "shell-2:dotfiles", "shell-2/dotfiles"},
	}
	for _, c := range cases {
		if got := GhosttyTitle(c.kind, c.id, c.project); got != c.title {
			t.Fatalf("GhosttyTitle(%v,%d,%q)=%q want %q", c.kind, c.id, c.project, got, c.title)
		}
		if got := TmuxSession(c.kind, c.id, c.project); got != c.session {
			t.Fatalf("TmuxSession(%v,%d,%q)=%q want %q", c.kind, c.id, c.project, got, c.session)
		}
		if got, ok := TmuxSessionFromTitle(c.title); !ok || got != c.session {
			t.Fatalf("TmuxSessionFromTitle(%q)=%q,%v want %q,true", c.title, got, ok, c.session)
		}
	}
}

func TestViewerNaming(t *testing.T) {
	if got := ViewerGhosttyTitle(1, "dotfiles"); got != "ai-view-1:dotfiles" {
		t.Fatalf("ViewerGhosttyTitle: %q", got)
	}
	if got := ViewerTmuxSession(1, "dotfiles"); got != "ai-1/dotfiles_v" {
		t.Fatalf("ViewerTmuxSession: %q", got)
	}
	got, ok := TmuxSessionFromTitle("ai-view-1:dotfiles")
	if !ok || got != "ai-1/dotfiles_v" {
		t.Fatalf("TmuxSessionFromTitle viewer: %q ok=%v", got, ok)
	}
	src, ok := SourceAITmuxSessionFromViewerTitle("ai-view-1:dotfiles")
	if !ok || src != "ai-1/dotfiles" {
		t.Fatalf("SourceAITmuxSessionFromViewerTitle: %q ok=%v", src, ok)
	}
}

func TestTmuxSessionFromTitleNonControllerOwned(t *testing.T) {
	// Zed natural title (no colon) should not resolve to a tmux session.
	if _, ok := TmuxSessionFromTitle("dotfiles"); ok {
		t.Fatalf("Zed natural title must not resolve to tmux session")
	}
}

func TestTitleClassification(t *testing.T) {
	if !IsAITitle("ai-1:dotfiles") {
		t.Fatalf("IsAITitle ai-1:dotfiles")
	}
	if IsAITitle("ai-view-1:dotfiles") {
		t.Fatalf("IsAITitle viewer must be false")
	}
	if !IsViewerTitle("ai-view-1:dotfiles") {
		t.Fatalf("IsViewerTitle viewer")
	}
	if IsViewerTitle("ai-1:dotfiles") {
		t.Fatalf("IsViewerTitle ai must be false")
	}
}

func TestAICommand(t *testing.T) {
	if AICommand(AIClaude) != "claude" || AICommand(AICopilot) != "copilot" {
		t.Fatal("AICommand mismatch")
	}
	if AICommand("nope") != "" {
		t.Fatal("AICommand unknown should be empty")
	}
}
