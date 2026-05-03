// Package zedwrap は zed CLI 起動を薄くラップする。
//
// projwm-design.md §5.4 v11.2: `zed -n <cwd>`（`-n/--new` は必須、デフォルトは
// 既存 workspace を再利用してしまうため。POC-17）。
package zedwrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("spawn zed: %w (output: %s)", err, string(out))
	}
	return nil
}

// CleanWorkspaceForPath は Zed の sqlite から `paths` が空 / 指定 cwd 一致の
// workspace 行を削除する。
//
// 背景: Zed は session restore で前回 windows を復元するため、 projwm が
// `zed -n cwd` で spawn する直前に **同 cwd の workspace 行と空行** を消して
// おかないと、 restore 復元 + 新 spawn で **同じ cwd の window が 2 つ** 出る
// 致命バグ (user 報告)。
//
// 注意: user の他 cwd の workspace 行は残す (paths が cwd と一致しないものは
// 触らない)。 つまり「projwm 管理対象の cwd の workspace 行のみ消す」。
//
// 呼び出しタイミング: ensureZedWindow の spawn 直前 (Zed running 中は sqlite が
// locked で失敗するが warn のみ; projwm の archive で Zed kill 済の経路でのみ
// 実効する)。
//
// Zed 未起動 / DB 不在 / sqlite3 cmd 不在は no-op (致命でない)。
func CleanWorkspaceForPath(cwd string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	db := filepath.Join(home, "Library", "Application Support", "Zed", "db", "0-stable", "db.sqlite")
	if _, err := os.Stat(db); os.IsNotExist(err) {
		return nil
	}
	cmd := exec.Command("sqlite3", db,
		`DELETE FROM workspaces WHERE paths IS NULL OR paths = '' OR paths = ?;`,
		cwd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sqlite cleanup (cwd=%q): %w (out=%s)", cwd, err, string(out))
	}
	return nil
}

// CleanEmptyWorkspaces は paths が空のみ削除する。 archive 一括 cleanup 用。
func CleanEmptyWorkspaces() error {
	return CleanWorkspaceForPath("__never_match__")
}
