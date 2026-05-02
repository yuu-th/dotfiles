// Package config は ~/.config/projwm/config.toml を読み込む。
//
// state は runtime 状態のみ、固定値（slot 名群、viewer WS 名）は config に置く
// （projwm-design.md §6.1, §6.2）。
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config は config.toml の内容。
type Config struct {
	ViewerWorkspace string   `toml:"viewer_workspace"`
	SlotNames       []string `toml:"slot_names"`
}

// Default は config.toml が無い時のデフォルト値（projwm-design.md §6.2）。
func Default() Config {
	return Config{
		ViewerWorkspace: "A",
		SlotNames:       []string{"Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"},
	}
}

// DefaultPath は config.toml の既定 path を返す（XDG_CONFIG_HOME ベース）。
func DefaultPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("user home dir: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "projwm", "config.toml"), nil
}

// Load は path から config.toml を読み込み、不在なら Default を返す。
//
// fallback ポリシー (projwm-design.md §6.2.1):
//   - ファイル不在: Default を返し、UsedDefault=true
//   - パース失敗: エラー終了
//   - 必須フィールド欠落: エラー終了
//   - 未知フィールド: 警告のみ（forward compat）。本実装では toml パッケージが
//     undecoded フィールドを保持するので、呼び出し側で UndecodedKeys() を確認可。
type LoadResult struct {
	Config       Config
	UsedDefault  bool
	UndecodedKeys []string
	SourcePath   string
}

func Load(path string) (LoadResult, error) {
	res := LoadResult{SourcePath: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			res.Config = Default()
			res.UsedDefault = true
			return res, nil
		}
		return res, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	md, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return res, fmt.Errorf("parse config (%s): %w", path, err)
	}
	for _, k := range md.Undecoded() {
		res.UndecodedKeys = append(res.UndecodedKeys, k.String())
	}
	if err := validate(&cfg); err != nil {
		return res, fmt.Errorf("config validation: %w", err)
	}
	res.Config = cfg
	return res, nil
}

func validate(c *Config) error {
	if len(c.SlotNames) == 0 {
		return errors.New("slot_names must not be empty")
	}
	if c.ViewerWorkspace == "" {
		return errors.New("viewer_workspace must not be empty")
	}
	// slot と viewer が衝突していないか
	for _, s := range c.SlotNames {
		if s == c.ViewerWorkspace {
			return fmt.Errorf("viewer_workspace %q must not appear in slot_names", s)
		}
	}
	return nil
}

// LoadFromDefaultPath は DefaultPath() から読み込む短縮形。
func LoadFromDefaultPath() (LoadResult, error) {
	p, err := DefaultPath()
	if err != nil {
		return LoadResult{}, err
	}
	return Load(p)
}
