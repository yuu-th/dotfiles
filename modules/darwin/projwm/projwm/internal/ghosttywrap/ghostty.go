// Package ghosttywrap は projwm の terminal driver（純正 Ghostty.app、v11.6）。
//
// 経緯:
//   - v11.3 で OmniWM 0.4.8 が SwiftUI WindowGroup の Ghostty を AX 列挙できない
//     と判断、kitty を user-space copy する方式に逃げた
//   - v11.6 で根本原因が判明: OmniWM の rule engine は app-rules で
//     `titleRegex` を指定しないと Ghostty の hidden helper windows と main window を
//     区別できず disposition=.unmanaged になる（modules/darwin/omniwm/app-rules.nix
//     に `titleRegex = "^(ai|shell|ai-view)-[0-9]+:"` の rule を追加することで解決）
//   - これにより純正 Ghostty.app での運用に戻せた
//
// 起動規約 (Ghostty CLI):
//
//	open -na /Applications/Ghostty.app --args \
//	     --title=<title> --working-directory=<cwd> -e tmux new-session -A -s <session>
package ghosttywrap

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type Spawner interface {
	Spawn(ctx context.Context, title, cwd, tmuxSession string) error
}

// CmdSpawner は実プロセス実行。
type CmdSpawner struct {
	// AppPath は terminal .app への path。`~` で始まれば $HOME 展開する。
	AppPath string // 既定 "/Applications/Ghostty.app"
	TmuxBin string // 既定 "tmux"
}

func (s CmdSpawner) Spawn(ctx context.Context, title, cwd, tmuxSession string) error {
	app := expandTilde(s.AppPath)
	if app == "" {
		app = "/Applications/Ghostty.app"
	}
	tb := s.TmuxBin
	if tb == "" {
		tb = "tmux"
	}
	// open -na <Ghostty.app> --args --title=<title> --working-directory=<cwd> -e tmux new-session -A -s <session>
	args := []string{
		"-na", app, "--args",
		"--title=" + title,
		"--working-directory=" + cwd,
		"-e", tb, "new-session", "-A", "-s", tmuxSession,
	}
	cmd := exec.CommandContext(ctx, "open", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("spawn terminal (open -na %s): %w (output: %s)",
			app, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// expandTilde は `~/foo` → `$HOME/foo`。空文字や絶対 path はそのまま。
func expandTilde(p string) string {
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "~/") {
		home, err := homeDir()
		if err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	if p == "~" {
		home, err := homeDir()
		if err == nil {
			return home
		}
	}
	return p
}

func homeDir() (string, error) {
	// shell 経由を避けて os.UserHomeDir 相当
	return userHomeDir()
}
