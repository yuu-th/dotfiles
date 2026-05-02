// Package tmuxwrap は tmux を薄くラップする（projwm の AI/shell 永続化）。
package tmuxwrap

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Executor は tmux を実行する抽象。
type Executor interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

type CmdExecutor struct {
	Bin string
}

func (c CmdExecutor) Run(ctx context.Context, args ...string) ([]byte, error) {
	bin := c.Bin
	if bin == "" {
		bin = "tmux"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		return out, fmt.Errorf("tmux %s: %w (stderr: %s)", strings.Join(args, " "), err, stderr)
	}
	return out, nil
}

type Client struct {
	exec Executor
}

func New(e Executor) *Client {
	if e == nil {
		e = CmdExecutor{}
	}
	return &Client{exec: e}
}

// HasSession は session 存在確認。
func (c *Client) HasSession(ctx context.Context, name string) (bool, error) {
	_, err := c.exec.Run(ctx, "has-session", "-t", "="+name)
	if err == nil {
		return true, nil
	}
	// has-session は存在しないと exit 1。stderr に "no session" を含む。
	if strings.Contains(err.Error(), "no session") || strings.Contains(err.Error(), "exit status 1") {
		return false, nil
	}
	return false, err
}

// NewSessionDetached は新しい session を detached で作る（既にあれば no-op）。
func (c *Client) NewSessionDetached(ctx context.Context, name, cwd string) error {
	if has, err := c.HasSession(ctx, name); err != nil {
		return err
	} else if has {
		return nil
	}
	args := []string{"new-session", "-d", "-s", name}
	if cwd != "" {
		args = append(args, "-c", cwd)
	}
	_, err := c.exec.Run(ctx, args...)
	return err
}

// NewGroupedSession は base session に grouped clone を作る（pty を共有する別 client 用）。
func (c *Client) NewGroupedSession(ctx context.Context, base, clone string) error {
	if has, err := c.HasSession(ctx, clone); err != nil {
		return err
	} else if has {
		return nil
	}
	_, err := c.exec.Run(ctx, "new-session", "-d", "-t", base, "-s", clone)
	return err
}

// KillSession は session を kill（不在なら no-op）。
func (c *Client) KillSession(ctx context.Context, name string) error {
	if has, err := c.HasSession(ctx, name); err != nil {
		return err
	} else if !has {
		return nil
	}
	_, err := c.exec.Run(ctx, "kill-session", "-t", "="+name)
	return err
}

// ListSessions は全 session 名を返す。
func (c *Client) ListSessions(ctx context.Context) ([]string, error) {
	out, err := c.exec.Run(ctx, "list-sessions", "-F", "#{session_name}")
	if err != nil {
		// no server running の場合は空リスト扱い
		if strings.Contains(err.Error(), "no server running") || strings.Contains(err.Error(), "error connecting") {
			return nil, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var res []string
	for _, l := range lines {
		if l != "" {
			res = append(res, l)
		}
	}
	return res, nil
}
