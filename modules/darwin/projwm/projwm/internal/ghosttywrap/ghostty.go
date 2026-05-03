// Package ghosttywrap (歴史的命名、現在は kitty driver を提供) は projwm の
// terminal driver。
//
// v11.3 で ghostty → kitty に切替（OmniWM 0.4.8 が macOS 26.x Tahoe + SwiftUI 系
// app の window を AX 列挙できないバグのため）。kitty を user-space copy して
// NSPrincipalClass=NSApplication を注入、ad-hoc 再署名する setup-kitty-projwm.sh
// と組み合わせる。
//
// 起動規約 (kitty CLI):
//
//	open -na <terminal_app_path> --args -T <title> -d <cwd> tmux new-session -A -s <session>
//
// kitty の `--single-instance` は使わない（複数 window をそれぞれ別 OmniWM window
// として認識させたいため、各 spawn は単一 instance）。
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
	AppPath string  // 既定 "~/Applications/kitty-projwm.app"
	TmuxBin string  // 既定 "tmux"
}

func (s CmdSpawner) Spawn(ctx context.Context, title, cwd, tmuxSession string) error {
	app := expandTilde(s.AppPath)
	if app == "" {
		app = expandTilde("~/Applications/kitty-projwm.app")
	}
	tb := s.TmuxBin
	if tb == "" {
		tb = "tmux"
	}
	// open -na <App> --args -T <title> -d <cwd> tmux new-session -A -s <session>
	args := []string{
		"-na", app, "--args",
		"-T", title,
		"-d", cwd,
		tb, "new-session", "-A", "-s", tmuxSession,
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
