// Package vivaldi は Vivaldi browser を osascript / Preferences 編集で外部制御する driver。
//
// 設計指針 (POC 結果, projwm-history.md "v12 POC"):
//   - Workspace 作成: Vivaldi 終了中に Preferences JSON を直接編集
//     (key: vivaldi.workspaces.list = [{id, name, icon}])。
//     Quick Commands 経由の自動作成は web UI が AX 不可視で検証困難なため不採用。
//   - Workspace 切替: System Events で「ウィンドウ → その他のワークスペースとタブ → <name>」
//     menu を click。screenshot で active marker (チェック印) が移動することを確認済。
//     Quick Commands (Cmd+E) は web UI で動作不透明なため副選択肢に留める。
//   - Activate: `open -a Vivaldi` を使う。 `tell application "Vivaldi" to activate` は
//     AppleEvent timeout する (Chromium 系 limitation)。
//
// Vivaldi は Chromium ベース。AX で web 内部は見えないため、active workspace 検出は
// 不能。projwm 側が intent (= state.Project.BrowserWorkspace) を source of truth として扱う。
package vivaldi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Driver は Vivaldi 制御の依存。
type Driver struct {
	AppPath   string // /Applications/Vivaldi.app
	PrefsPath string // ~/Library/Application Support/Vivaldi/Default/Preferences
	BundleID  string // com.vivaldi.Vivaldi
	Logger    io.Writer
}

// New は default 値で Driver を返す。
func New() *Driver {
	home, _ := os.UserHomeDir()
	return &Driver{
		AppPath:   "/Applications/Vivaldi.app",
		PrefsPath: filepath.Join(home, "Library", "Application Support", "Vivaldi", "Default", "Preferences"),
		BundleID:  "com.vivaldi.Vivaldi",
		Logger:    io.Discard,
	}
}

func (d *Driver) logf(format string, args ...any) {
	if d.Logger == nil {
		return
	}
	fmt.Fprintf(d.Logger, "[vivaldi] "+format+"\n", args...)
}

// IsRunning は Vivaldi が走っているかを返す。
func (d *Driver) IsRunning() bool {
	out, err := exec.Command("pgrep", "-x", "Vivaldi").Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

// Activate は Vivaldi を foreground にする（未起動なら起動）。
//
// `tell application` 経由は timeout するため `open -a` で起動 + activate する。
func (d *Driver) Activate(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "open", "-a", d.AppPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("open -a Vivaldi: %w (out=%s)", err, out)
	}
	// 起動直後は menu bar が活性化に時間が必要。短い settling delay。
	if !d.IsRunning() {
		time.Sleep(2 * time.Second)
	} else {
		time.Sleep(300 * time.Millisecond)
	}
	return nil
}

// Quit は Vivaldi を終了する。Preferences 編集前に呼ぶ。
func (d *Driver) Quit(ctx context.Context) error {
	if !d.IsRunning() {
		return nil
	}
	// `osascript -e 'quit app "Vivaldi"'` は Chromium 系で timeout しがちなので
	// SIGTERM を送る。Vivaldi は session を自動保存。
	cmd := exec.CommandContext(ctx, "pkill", "-x", "Vivaldi")
	_ = cmd.Run()
	// 完全終了まで待つ
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !d.IsRunning() {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("vivaldi did not quit within 10s")
}

// EnsureWorkspaces は names で指定された workspace が存在することを保証する。
//
// 動作:
//  1. Preferences を読み込む
//  2. 既存 workspace 名 と names を比較、不足分を追加（Vivaldi 終了中のみ）
//  3. 不足が無ければ no-op
//
// Vivaldi 起動中に実行された場合、変更が無いことを確認できれば no-op、変更が必要なら
// 一旦 quit → 編集 → 起動を行う。
func (d *Driver) EnsureWorkspaces(ctx context.Context, names []string) error {
	if len(names) == 0 {
		return nil
	}
	// Read prefs
	data, err := os.ReadFile(d.PrefsPath)
	if err != nil {
		return fmt.Errorf("read prefs: %w", err)
	}
	var prefs map[string]any
	if err := json.Unmarshal(data, &prefs); err != nil {
		return fmt.Errorf("parse prefs: %w", err)
	}
	existing := extractWorkspaceNames(prefs)
	existingSet := map[string]bool{}
	for _, n := range existing {
		existingSet[n] = true
	}
	missing := []string{}
	for _, n := range names {
		if !existingSet[n] {
			missing = append(missing, n)
		}
	}
	if len(missing) == 0 {
		d.logf("ensure: all %d workspaces present", len(names))
		return nil
	}
	d.logf("ensure: adding %d missing workspaces: %v", len(missing), missing)

	wasRunning := d.IsRunning()
	if wasRunning {
		if err := d.Quit(ctx); err != nil {
			return fmt.Errorf("quit before edit: %w", err)
		}
	}

	// Add missing workspaces
	if err := addWorkspacesToPrefs(d.PrefsPath, missing); err != nil {
		return fmt.Errorf("add workspaces: %w", err)
	}

	if wasRunning {
		if err := d.Activate(ctx); err != nil {
			return fmt.Errorf("relaunch after edit: %w", err)
		}
	}
	return nil
}

// extractWorkspaceNames は prefs map から vivaldi.workspaces.list の name 配列を抽出する。
func extractWorkspaceNames(prefs map[string]any) []string {
	v, ok := prefs["vivaldi"].(map[string]any)
	if !ok {
		return nil
	}
	ws, ok := v["workspaces"].(map[string]any)
	if !ok {
		return nil
	}
	list, ok := ws["list"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		if m, ok := e.(map[string]any); ok {
			if name, ok := m["name"].(string); ok {
				out = append(out, name)
			}
		}
	}
	sort.Strings(out)
	return out
}

// addWorkspacesToPrefs は Preferences ファイルに workspace を追加する（Vivaldi 終了中前提）。
func addWorkspacesToPrefs(path string, names []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var prefs map[string]any
	if err := json.Unmarshal(data, &prefs); err != nil {
		return err
	}
	v, _ := prefs["vivaldi"].(map[string]any)
	if v == nil {
		v = map[string]any{}
		prefs["vivaldi"] = v
	}
	ws, _ := v["workspaces"].(map[string]any)
	if ws == nil {
		ws = map[string]any{
			"button":      map[string]any{"show_name": true},
			"link_routes": []any{},
			"list":        []any{},
		}
		v["workspaces"] = ws
	}
	list, _ := ws["list"].([]any)
	if list == nil {
		list = []any{}
	}
	baseTS := float64(time.Now().UnixMilli())
	for i, name := range names {
		list = append(list, map[string]any{
			"id":   baseTS + float64(i),
			"name": name,
			"icon": "",
		})
	}
	ws["list"] = list

	out, err := json.Marshal(prefs)
	if err != nil {
		return err
	}
	tmp := path + ".projwm.tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// SwitchWorkspace は target 名の workspace に切り替える。
//
// 実装: System Events で Window メニューの「その他のワークスペースとタブ」サブメニューを
// 開き、target 名の項目を click する。Locale (jp/en) を自動検出。
//
// 前提: workspace は事前に EnsureWorkspaces で作成済。
func (d *Driver) SwitchWorkspace(ctx context.Context, target string) error {
	if err := d.Activate(ctx); err != nil {
		return err
	}
	script := buildSwitchScript(target)
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("switch via menu: %w (out=%s)", err, strings.TrimSpace(string(out)))
	}
	d.logf("switched to %q (%s)", target, strings.TrimSpace(string(out)))
	return nil
}

// buildSwitchScript は AppleScript を生成する。Locale 検出 + click 実行。
//
// メニュー名は locale 依存:
//   - ja: ウィンドウ / その他のワークスペースとタブ
//   - en: Window  / Other Workspaces and Tabs
//
// 実装: menu bar item を index で取れない (順序は同じだが locale text で取得が必要)、
// ので候補名 list を順に試す。
func buildSwitchScript(target string) string {
	// Escape target for AppleScript string literal
	esc := strings.ReplaceAll(target, "\\", "\\\\")
	esc = strings.ReplaceAll(esc, "\"", "\\\"")
	return fmt.Sprintf(`
tell application "System Events"
  tell process "Vivaldi"
    set frontmost to true
    delay 0.3
    set winNames to {"ウィンドウ", "Window", "Fenster", "窗口"}
    set wsNames to {"その他のワークスペースとタブ", "Other Workspaces and Tabs", "Andere Arbeitsbereiche und Tabs", "其他工作区和标签页"}
    set winMenuItem to missing value
    repeat with n in winNames
      try
        set winMenuItem to menu bar item n of menu bar 1
        exit repeat
      end try
    end repeat
    if winMenuItem is missing value then
      error "Window menu not found (locale)"
    end if
    click winMenuItem
    delay 0.4
    set wsMenuItem to missing value
    repeat with n in wsNames
      try
        set wsMenuItem to menu item n of menu 1 of winMenuItem
        exit repeat
      end try
    end repeat
    if wsMenuItem is missing value then
      key code 53
      error "Workspaces submenu not found (locale)"
    end if
    click wsMenuItem
    delay 0.5
    try
      click menu item "%s" of menu 1 of wsMenuItem
    on error errMsg
      key code 53
      key code 53
      error "workspace not found: %s (" & errMsg & ")"
    end try
    delay 0.4
  end tell
  -- Vivaldi (Chromium) は menu click 後に menu cascade を閉じない quirk があるため
  -- 一瞬 Finder に focus を渡して menu を強制 dismiss、Vivaldi は active の意図を保持。
  -- (POC で確認: finder swap で確実に menu が閉じる)
  tell process "Finder" to set frontmost to true
  delay 0.15
end tell
return "ok"
`, esc, esc)
}

// ListWorkspaces は Preferences から既存 workspace 名一覧を返す。
func (d *Driver) ListWorkspaces() ([]string, error) {
	data, err := os.ReadFile(d.PrefsPath)
	if err != nil {
		return nil, err
	}
	var prefs map[string]any
	if err := json.Unmarshal(data, &prefs); err != nil {
		return nil, err
	}
	return extractWorkspaceNames(prefs), nil
}
