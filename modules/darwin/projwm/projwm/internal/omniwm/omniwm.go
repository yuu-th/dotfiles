// Package omniwm は omniwmctl を薄くラップする。
//
// 設計: queue/projwm-design.md §3.2 / §7 / §8。
// move-to-workspace は number 引数のみなので、name → number は query workspaces で解決する。
package omniwm

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Executor は omniwmctl を実行する抽象。テスト時はモック可能。
type Executor interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

// CmdExecutor は実プロセス実行。
type CmdExecutor struct {
	Bin string // "omniwmctl"
}

func (c CmdExecutor) Run(ctx context.Context, args ...string) ([]byte, error) {
	bin := c.Bin
	if bin == "" {
		bin = "omniwmctl"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		return out, fmt.Errorf("omniwmctl %s: %w (stderr: %s)", strings.Join(args, " "), err, stderr)
	}
	return out, nil
}

// Client は型付き operation を提供する。
type Client struct {
	exec Executor
}

func New(e Executor) *Client {
	if e == nil {
		e = CmdExecutor{}
	}
	return &Client{exec: e}
}

// Window は omniwmctl query windows の 1 要素（必要 fields のみ）。
type Window struct {
	ID        string
	Title     string
	BundleID  string
	AppName   string
	PID       int
	Workspace WorkspaceRef
	IsFocused bool
	IsVisible bool
}

type WorkspaceRef struct {
	ID          string
	Number      int
	RawName     string
	DisplayName string
}

// rawQueryResponse は omniwmctl の標準出力をデコードする。
type rawQueryResponse struct {
	OK     bool `json:"ok"`
	Result struct {
		Kind    string          `json:"kind"`
		Payload json.RawMessage `json:"payload"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

// QueryWindows は windows query の結果を返す。selectors は --bundle-id, --workspace 等。
func (c *Client) QueryWindows(ctx context.Context, selectors ...string) ([]Window, error) {
	args := append([]string{"query", "windows", "--json"}, selectors...)
	out, err := c.exec.Run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var resp rawQueryResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	var payload struct {
		Windows []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			App   struct {
				BundleID string `json:"bundleId"`
				Name     string `json:"name"`
			} `json:"app"`
			PID       int  `json:"pid"`
			IsFocused bool `json:"isFocused"`
			IsVisible bool `json:"isVisible"`
			Workspace struct {
				ID          string `json:"id"`
				Number      int    `json:"number"`
				RawName     string `json:"rawName"`
				DisplayName string `json:"displayName"`
			} `json:"workspace"`
		} `json:"windows"`
	}
	if err := json.Unmarshal(resp.Result.Payload, &payload); err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	out2 := make([]Window, 0, len(payload.Windows))
	for _, w := range payload.Windows {
		out2 = append(out2, Window{
			ID:        w.ID,
			Title:     w.Title,
			BundleID:  w.App.BundleID,
			AppName:   w.App.Name,
			PID:       w.PID,
			IsFocused: w.IsFocused,
			IsVisible: w.IsVisible,
			Workspace: WorkspaceRef{
				ID: w.Workspace.ID, Number: w.Workspace.Number,
				RawName: w.Workspace.RawName, DisplayName: w.Workspace.DisplayName,
			},
		})
	}
	return out2, nil
}

// Workspace は workspaces query の 1 要素。
type Workspace struct {
	ID          string
	RawName     string
	DisplayName string
	Number      int
	IsFocused   bool
	IsVisible   bool
	IsCurrent   bool
}

// QueryWorkspaces は全 WS を返す。
func (c *Client) QueryWorkspaces(ctx context.Context) ([]Workspace, error) {
	out, err := c.exec.Run(ctx, "query", "workspaces", "--json")
	if err != nil {
		return nil, err
	}
	var resp rawQueryResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	var payload struct {
		Workspaces []struct {
			ID          string `json:"id"`
			RawName     string `json:"rawName"`
			DisplayName string `json:"displayName"`
			Number      int    `json:"number"`
			IsFocused   bool   `json:"isFocused"`
			IsVisible   bool   `json:"isVisible"`
			IsCurrent   bool   `json:"isCurrent"`
		} `json:"workspaces"`
	}
	if err := json.Unmarshal(resp.Result.Payload, &payload); err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	res := make([]Workspace, 0, len(payload.Workspaces))
	for _, w := range payload.Workspaces {
		res = append(res, Workspace{
			ID: w.ID, RawName: w.RawName, DisplayName: w.DisplayName,
			Number: w.Number, IsFocused: w.IsFocused, IsVisible: w.IsVisible, IsCurrent: w.IsCurrent,
		})
	}
	return res, nil
}

// WorkspaceNumberByName は name → number を解決する（"Q" → 14 等）。
func (c *Client) WorkspaceNumberByName(ctx context.Context, name string) (int, error) {
	wss, err := c.QueryWorkspaces(ctx)
	if err != nil {
		return 0, err
	}
	for _, ws := range wss {
		if ws.RawName == name || ws.DisplayName == name {
			return ws.Number, nil
		}
	}
	return 0, fmt.Errorf("workspace %q not found", name)
}

// FocusWorkspaceByName は workspace focus-name <name> を実行（jump 用）。
func (c *Client) FocusWorkspaceByName(ctx context.Context, name string) error {
	_, err := c.exec.Run(ctx, "workspace", "focus-name", name)
	return err
}

// FocusWindow は window focus <id>。
func (c *Client) FocusWindow(ctx context.Context, windowID string) error {
	_, err := c.exec.Run(ctx, "window", "focus", windowID)
	return err
}

// MoveWindowToWorkspaceByName は window を name の WS に送る（内部で number 解決）。
//
// omniwmctl の move-to-workspace は number しか受け付けないので、name → number を
// query workspaces で解決してから move-column-to-workspace を発行する。
func (c *Client) MoveWindowToWorkspaceByName(ctx context.Context, windowID, wsName string) error {
	num, err := c.WorkspaceNumberByName(ctx, wsName)
	if err != nil {
		return err
	}
	// focus 該当 window → move-column-to-workspace で送る（move-to-workspace は focused 前提）
	if err := c.FocusWindow(ctx, windowID); err != nil {
		return err
	}
	_, err = c.exec.Run(ctx, "command", "move-column-to-workspace", fmt.Sprint(num))
	return err
}
