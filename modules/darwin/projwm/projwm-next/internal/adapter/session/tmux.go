// Package session implements the tmux SessionCapabilityAdapter primitive
// used by the production daemon to back Ghostty windows with named tmux
// sessions (design.md §7.2 / projwm-spec FR-21 / specs §5.1).
//
// All methods shell out to the tmux binary; if Bin is empty "tmux" is used.
// Methods are safe for use from multiple goroutines (tmux serializes server
// commands internally).
package session

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Executor runs `tmux <args...>` and returns combined output. Tests inject a
// fake; production uses CmdExecutor.
type Executor interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

// CmdExecutor uses os/exec.
type CmdExecutor struct {
	Bin string
}

func (c CmdExecutor) Run(ctx context.Context, args ...string) ([]byte, error) {
	bin := c.Bin
	if bin == "" {
		bin = "tmux"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.Bytes(), err
}

// Client is the tmux SessionCapabilityAdapter implementation.
type Client struct {
	Bin  string
	Exec Executor
}

func (c *Client) executor() Executor {
	if c.Exec != nil {
		return c.Exec
	}
	return CmdExecutor{Bin: c.Bin}
}

// HasSession reports whether a tmux session of exact name `name` exists.
// The `=` exact-match prefix is required; without it tmux treats the target
// as a prefix match which causes false positives between names like
// `ai-1/dotfiles` and `ai-1/dotfiles_v`.
func (c *Client) HasSession(ctx context.Context, name string) (bool, error) {
	_, err := c.executor().Run(ctx, "has-session", "-t", "="+name)
	if err == nil {
		return true, nil
	}
	// tmux exits non-zero if the session does not exist.
	if _, isExitErr := err.(*exec.ExitError); isExitErr {
		return false, nil
	}
	return false, fmt.Errorf("tmux has-session: %w", err)
}

// EnsureSession creates a detached tmux session named `name` rooted at `cwd`
// if one does not already exist. created=true when a fresh session was made.
func (c *Client) EnsureSession(ctx context.Context, name, cwd string) (created bool, err error) {
	exists, err := c.HasSession(ctx, name)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	args := []string{"new-session", "-d", "-s", name}
	if cwd != "" {
		args = append(args, "-c", cwd)
	}
	if out, err := c.executor().Run(ctx, args...); err != nil {
		return false, fmt.Errorf("tmux new-session %q: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return true, nil
}

// EnsureGroupedSession creates `clone` as a tmux *grouped* session pointing
// at `base` (no-op if `clone` already exists). Grouped sessions share window
// state with the source session so the viewer pane mirrors the AI pane.
func (c *Client) EnsureGroupedSession(ctx context.Context, base, clone string) error {
	exists, err := c.HasSession(ctx, clone)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if out, err := c.executor().Run(ctx, "new-session", "-d", "-t", base, "-s", clone); err != nil {
		return fmt.Errorf("tmux new-session -t %q -s %q: %w: %s", base, clone, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SendKeys sends literal keystrokes to the named session followed by an
// Enter (C-m) so the command executes.
func (c *Client) SendKeys(ctx context.Context, session string, keys ...string) error {
	args := []string{"send-keys", "-t", session}
	args = append(args, keys...)
	args = append(args, "C-m")
	if out, err := c.executor().Run(ctx, args...); err != nil {
		return fmt.Errorf("tmux send-keys %q: %w: %s", session, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// KillSession terminates the named tmux session.
func (c *Client) KillSession(ctx context.Context, name string) error {
	if out, err := c.executor().Run(ctx, "kill-session", "-t", "="+name); err != nil {
		return fmt.Errorf("tmux kill-session %q: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ListSessions returns the set of live session names.
func (c *Client) ListSessions(ctx context.Context) ([]string, error) {
	out, err := c.executor().Run(ctx, "list-sessions", "-F", "#{session_name}")
	if err != nil {
		// tmux exits non-zero with "no server running" when there are no
		// sessions; treat as empty.
		if strings.Contains(string(out), "no server running") {
			return nil, nil
		}
		return nil, fmt.Errorf("tmux list-sessions: %w: %s", err, strings.TrimSpace(string(out)))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var sessions []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			sessions = append(sessions, l)
		}
	}
	return sessions, nil
}
