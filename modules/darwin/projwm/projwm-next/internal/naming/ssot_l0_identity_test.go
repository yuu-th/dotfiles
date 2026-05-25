package naming

import "testing"

func TestSSOTIdentityRestorationFromManagedTitles(t *testing.T) {
	tests := []struct {
		title      string
		wantTmux   string
		wantSource string
	}{
		{title: "shell-1:dotfiles", wantTmux: "shell-1/dotfiles"},
		{title: "ai-2:manaflow", wantTmux: "ai-2/manaflow"},
		{title: "ai-view-1:dotfiles", wantTmux: "ai-1/dotfiles_v", wantSource: "ai-1/dotfiles"},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got, ok := TmuxSessionFromTitle(tt.title)
			if !ok || got != tt.wantTmux {
				t.Fatalf("TmuxSessionFromTitle(%q) = %q, %v; want %q, true", tt.title, got, ok, tt.wantTmux)
			}
			if tt.wantSource != "" {
				src, ok := SourceAITmuxSessionFromViewerTitle(tt.title)
				if !ok || src != tt.wantSource {
					t.Fatalf("SourceAITmuxSessionFromViewerTitle(%q) = %q, %v; want %q, true", tt.title, src, ok, tt.wantSource)
				}
			}
		})
	}
}

func TestSSOTIdentityRestorationRejectsUnknownTitle(t *testing.T) {
	if got, ok := TmuxSessionFromTitle("random-window"); ok || got != "" {
		t.Fatalf("unknown title resolved to %q, %v; want empty, false", got, ok)
	}
}

func TestSSOTZedTitleIsProjectRootBasename(t *testing.T) {
	tests := []struct {
		cwd  string
		want string
	}{
		{cwd: "/Users/yuta/dev/dotfiles", want: "dotfiles"},
		{cwd: "/tmp/projwm-next", want: "projwm-next"},
	}
	for _, tt := range tests {
		t.Run(tt.cwd, func(t *testing.T) {
			if got := ZedTitle(tt.cwd); got != tt.want {
				t.Fatalf("ZedTitle(%q) = %q, want %q", tt.cwd, got, tt.want)
			}
		})
	}
}
