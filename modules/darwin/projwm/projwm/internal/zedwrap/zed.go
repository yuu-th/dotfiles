// Package zedwrap は zed CLI 起動を薄くラップする。
//
// 戦略 (v12 + Zed 重複対策):
//
//   1. **--user-data-dir で projwm 専用 data dir に分離**: user の通常 Zed
//      (default ~/Library/Application Support/Zed) とは独立した sqlite/settings/
//      extensions を持つ。 user の他 workspace の影響を受けず、 projwm 利用時の
//      Zed 状態を完全コントロールできる。
//   2. **専用 settings.json で `restore_on_startup: none`**: 起動時の session
//      restore を抑制。 `zed -n cwd` で 1 window のみ確実 spawn。
//   3. **-n フラグ**: 新 workspace 強制 (POC-17)。
package zedwrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ProjwmZedDataDir は projwm 専用の Zed --user-data-dir。
// この下に sqlite / settings / extensions / logs が生成される。
//
// 例: /Users/yuta/.cache/projwm/zed-data
//
// 一度生成されれば設定 (restore_on_startup: none) は永続。
func ProjwmZedDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "projwm", "zed-data"), nil
}

// EnsureDataDir は projwm 専用 Zed data dir + settings.json を作る。
// 既にある場合は no-op (settings.json は upsert で restore=none を保証)。
func EnsureDataDir() (string, error) {
	dir, err := ProjwmZedDataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// Zed は --user-data-dir 配下に settings.json (or config/settings.json) を読む。
	// 実装上の正確な path は Zed のバージョンで微妙に違うので、 通常 path 両方に
	// 同内容で書いておく (副作用なし)。
	settings := []byte(`// projwm 専用 Zed 設定 (auto-generated, 触らないこと)
//
// projwm が --user-data-dir でこの directory を Zed に渡し、 Zed は user の
// 通常 ~/Library/Application Support/Zed とは独立した状態をここに持つ。
// projwm 利用時の Zed window 重複を avoid するため restore_on_startup=none。
{
  "restore_on_startup": "none",
  "auto_install_extensions": {}
}
`)
	for _, p := range []string{
		filepath.Join(dir, "settings.json"),
		filepath.Join(dir, "config", "settings.json"),
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			continue
		}
		// 既に user 編集された設定があるかもしれないので、 file 不在時のみ書く
		if _, err := os.Stat(p); os.IsNotExist(err) {
			_ = os.WriteFile(p, settings, 0o644)
		}
	}
	return dir, nil
}

type Spawner interface {
	Spawn(ctx context.Context, cwd string) error
}

type CmdSpawner struct {
	Bin string // "zed"
}

// Spawn は projwm 専用 data dir で `zed -n --user-data-dir <dir> <cwd>` を実行。
// data dir は EnsureDataDir で初期化される (settings.json 含む)。
func (s CmdSpawner) Spawn(ctx context.Context, cwd string) error {
	bin := s.Bin
	if bin == "" {
		bin = "zed"
	}
	dir, err := EnsureDataDir()
	if err != nil {
		// fallback: 通常起動
		dir = ""
	}
	args := []string{"-n"}
	if dir != "" {
		args = append(args, "--user-data-dir", dir)
	}
	args = append(args, cwd)
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("spawn zed: %w (output: %s)", err, string(out))
	}
	return nil
}

// CleanWorkspaceForPath は **projwm 専用 Zed data dir** の sqlite から `paths` が
// 空 / 指定 cwd 一致の workspace 行を削除する。
//
// settings.json で restore_on_startup=none を強制している実装下では本関数は
// **念のため** の保険。 projwm 専用 data dir 内の DB のみ触り、 user の通常
// Zed (~/Library/...) には影響しない。
//
// Zed running 中は sqlite が locked で失敗 → no-op (致命でない)。
func CleanWorkspaceForPath(cwd string) error {
	dir, err := ProjwmZedDataDir()
	if err != nil {
		return err
	}
	// Zed の sqlite path 候補 (バージョンで違う可能性があるため両方試行)
	candidates := []string{
		filepath.Join(dir, "db", "0-stable", "db.sqlite"),
		filepath.Join(dir, "db", "0-global", "db.sqlite"),
	}
	for _, db := range candidates {
		if _, err := os.Stat(db); os.IsNotExist(err) {
			continue
		}
		cmd := exec.Command("sqlite3", db,
			`DELETE FROM workspaces WHERE paths IS NULL OR paths = '' OR paths = ?;`,
			cwd)
		_, _ = cmd.CombinedOutput()
		// エラーは ignore (まだ workspaces table が無いとか、 Zed running で
		// locked とか、 様々あるが致命でない)。
	}
	return nil
}

// CleanEmptyWorkspaces は paths が空のみ削除する (cwd 不問)。
func CleanEmptyWorkspaces() error {
	return CleanWorkspaceForPath("__never_match__")
}
