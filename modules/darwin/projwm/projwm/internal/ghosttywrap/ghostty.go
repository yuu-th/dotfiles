// Package ghosttywrap は ghostty 起動を薄くラップする。
//
// projwm-design.md §5.4: `ghostty --title=<t> --working-directory=<cwd> -e tmux new-session -A -s <session>`
package ghosttywrap

import (
	"context"
	"fmt"
	"os/exec"
)

type Spawner interface {
	Spawn(ctx context.Context, title, cwd, tmuxSession string) error
}

// CmdSpawner は実プロセス実行。
type CmdSpawner struct {
	GhosttyBin string // "ghostty"
	TmuxBin    string // "tmux"
}

func (s CmdSpawner) Spawn(ctx context.Context, title, cwd, tmuxSession string) error {
	gb := s.GhosttyBin
	if gb == "" {
		gb = "ghostty"
	}
	tb := s.TmuxBin
	if tb == "" {
		tb = "tmux"
	}
	args := []string{
		"--title=" + title,
		"--working-directory=" + cwd,
		"-e", tb, "new-session", "-A", "-s", tmuxSession,
	}
	cmd := exec.CommandContext(ctx, gb, args...)
	// ghostty を detach 起動（projwm はバックグラウンドに spawn して即座に戻る）
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn ghostty: %w", err)
	}
	// Release: 親の終了で子も死なないように
	go cmd.Wait()
	return nil
}
