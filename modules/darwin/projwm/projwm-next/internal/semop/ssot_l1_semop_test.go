package semop

import (
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §4.4 ai: aiCommand is routed from DesiredWindow.AI.Name. Default
// (nil or empty Name) is "claude". Unknown names fall back to claude
// rather than emitting an empty command.
func TestSSOTAICommandRoutesFromDesiredAISession(t *testing.T) {
	tests := []struct {
		name    string
		session *w.DesiredAISession
		want    string
	}{
		{name: "nil-defaults-to-claude", session: nil, want: "claude"},
		{name: "empty-defaults-to-claude", session: &w.DesiredAISession{Name: ""}, want: "claude"},
		{name: "explicit-claude", session: &w.DesiredAISession{Name: "claude"}, want: "claude"},
		{name: "explicit-copilot", session: &w.DesiredAISession{Name: "copilot"}, want: "copilot"},
		{name: "unknown-falls-back-to-claude", session: &w.DesiredAISession{Name: "gpt-oss"}, want: "claude"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &w.DesiredWindow{
				ID:   w.DesiredWindowID{Project: "p", Kind: w.WindowAI, Index: 1},
				Kind: w.WindowAI,
				AI:   tt.session,
			}
			got := aiCommandFor(d)
			if got != tt.want {
				t.Fatalf("aiCommandFor = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSSOTTerminalSessionFieldsUseOneBasedDesiredIDDirectly(t *testing.T) {
	tests := []struct {
		name          string
		window        w.DesiredWindow
		wantSession   string
		wantSource    string
		wantAICommand string
	}{
		{
			name: "ai-default",
			window: w.DesiredWindow{
				ID:   w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowAI, Index: 1},
				Kind: w.WindowAI,
			},
			wantSession:   "ai-1/dotfiles",
			wantAICommand: "claude",
		},
		{
			name: "ai-copilot",
			window: w.DesiredWindow{
				ID:   w.DesiredWindowID{Project: "manaflow", Kind: w.WindowAI, Index: 2},
				Kind: w.WindowAI,
				AI:   &w.DesiredAISession{Name: "copilot"},
			},
			wantSession:   "ai-2/manaflow",
			wantAICommand: "copilot",
		},
		{
			name: "shell",
			window: w.DesiredWindow{
				ID:   w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowShell, Index: 2},
				Kind: w.WindowShell,
			},
			wantSession: "shell-2/dotfiles",
		},
		{
			name: "viewer",
			window: w.DesiredWindow{
				ID:   w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowViewer, Index: 1},
				Kind: w.WindowViewer,
			},
			wantSession: "ai-1/dotfiles_v",
			wantSource:  "ai-1/dotfiles",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSession, gotSource, gotAICommand := terminalSessionFields(&tt.window)
			if gotSession != tt.wantSession || gotSource != tt.wantSource || gotAICommand != tt.wantAICommand {
				t.Fatalf("terminalSessionFields = (%q, %q, %q), want (%q, %q, %q)",
					gotSession, gotSource, gotAICommand, tt.wantSession, tt.wantSource, tt.wantAICommand)
			}
		})
	}
}
