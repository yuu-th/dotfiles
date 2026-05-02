// Package zedwrap は zed CLI 起動を薄くラップする。
//
// projwm-design.md §5.4 v11.2: `zed -n <cwd>`（`-n/--new` は必須、デフォルトは
// 既存 workspace を再利用してしまうため。POC-17）。
package zedwrap

import (
	"context"
	"fmt"
	"os/exec"
)

type Spawner interface {
	Spawn(ctx context.Context, cwd string) error
}

type CmdSpawner struct {
	Bin string // "zed"
}

func (s CmdSpawner) Spawn(ctx context.Context, cwd string) error {
	bin := s.Bin
	if bin == "" {
		bin = "zed"
	}
	cmd := exec.CommandContext(ctx, bin, "-n", cwd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn zed: %w", err)
	}
	go cmd.Wait()
	return nil
}
