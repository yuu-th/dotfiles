// Package chromium は Chromium 系 browser (Vivaldi/Brave/Edge/Chrome/Arc) を
// chrome-cli + osascript で外部制御する driver (paradigm C, marker なし版)。
//
// 設計指針:
//
//   - read 系 (`chrome-cli list windows/tabs`) は focus 奪わない (Scripting Bridge)
//   - write 系 (`chrome-cli close` / `open -na ... --new-window`) は focus を奪う
//     ため destructive 操作の前後で frontmost app を保存・復帰する
//   - per-project window: Chromium の `--profile-directory` で cookies/login を分離
//   - **window 識別は chrome-cli の window-id (string) を state.Window.LiveWindowID
//     に保存して引き回す**。marker tab は不要 (paradigm C 改訂、user 邪魔感ゼロ)
//   - 再起動跨ぎで LiveWindowID は stale になるが close 時 not-found で no-op
//
// projwm の reconcile はこの driver を **何も呼ばない**。明示イベントでのみ操作。
package chromium

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuu-th/projwm/internal/naming"
)

// Driver は Chromium browser 制御の依存。
type Driver struct {
	// AppPath は browser の .app path（既定 /Applications/Vivaldi.app）。
	AppPath string
	// BundleID は chrome-cli の CHROME_BUNDLE_IDENTIFIER に渡す bundle id。
	BundleID string
	// ChromeCli は chrome-cli の binary path（既定 /opt/homebrew/bin/chrome-cli）。
	ChromeCli string
	// Logger は trace 出力先（既定 io.Discard）。
	Logger io.Writer
}

// New は Vivaldi 既定値で Driver を返す。
func New() *Driver {
	return &Driver{
		AppPath:   "/Applications/Vivaldi.app",
		BundleID:  naming.VivaldiBundleID,
		ChromeCli: "/opt/homebrew/bin/chrome-cli",
		Logger:    io.Discard,
	}
}

func (d *Driver) logf(format string, args ...any) {
	if d.Logger == nil {
		return
	}
	fmt.Fprintf(d.Logger, "[chromium] "+format+"\n", args...)
}

// IsRunning は browser が走っているかを返す。
func (d *Driver) IsRunning() bool {
	out, err := exec.Command("pgrep", "-x", filepath.Base(strings.TrimSuffix(d.AppPath, ".app"))).Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

// Frontmost は現在 frontmost な application プロセス名を返す。
func Frontmost() (string, error) {
	out, err := exec.Command("osascript", "-e",
		`tell application "System Events" to get name of first application process whose frontmost is true`).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Activate は指定 application を frontmost にする。
func Activate(name string) error {
	if name == "" {
		return nil
	}
	return exec.Command("osascript", "-e",
		fmt.Sprintf(`tell application %q to activate`, name)).Run()
}

func isChromiumProc(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "vivaldi") || strings.Contains(n, "chrome") || strings.Contains(n, "brave") || strings.Contains(n, "edge") || strings.Contains(n, "arc")
}

// withFocusRestore は fn 実行の前後で frontmost を保存・復帰する。
//
// browser は new-window 後に「welcome page loading 完了」「onboarding modal 表示」等
// で遅延して focus を奪い返してくる。defer 内で複数回 (合計 ~3 秒) Activate を試みる。
func (d *Driver) withFocusRestore(fn func() error) error {
	prev, _ := Frontmost()
	defer func() {
		if prev == "" || isChromiumProc(prev) {
			return
		}
		for _, delay := range []time.Duration{0, 500 * time.Millisecond, 1 * time.Second, 1500 * time.Millisecond} {
			if delay > 0 {
				time.Sleep(delay)
			}
			if rerr := Activate(prev); rerr != nil {
				d.logf("WARN: restore frontmost %q: %v", prev, rerr)
			}
		}
	}()
	return fn()
}

// Window は chrome-cli list windows の 1 行をパースした結果。
type Window struct {
	ID    string
	Title string
}

// Tab は chrome-cli list tabs の 1 行をパース結果。
type Tab struct {
	ID    string
	Title string
}

// ListWindows は現在 browser に存在する全 window を返す（focus 奪わない）。
func (d *Driver) ListWindows(ctx context.Context) ([]Window, error) {
	cmd := exec.CommandContext(ctx, d.ChromeCli, "list", "windows")
	cmd.Env = append(os.Environ(), "CHROME_BUNDLE_IDENTIFIER="+d.BundleID)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("chrome-cli list windows: %w", err)
	}
	return parseListing(string(out)), nil
}

// ListTabsInWindow は指定 window の tab 一覧を返す（focus 奪わない）。
func (d *Driver) ListTabsInWindow(ctx context.Context, wid string) ([]Tab, error) {
	cmd := exec.CommandContext(ctx, d.ChromeCli, "list", "tabs", "-w", wid)
	cmd.Env = append(os.Environ(), "CHROME_BUNDLE_IDENTIFIER="+d.BundleID)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("chrome-cli list tabs: %w", err)
	}
	rows := parseListing(string(out))
	tabs := make([]Tab, len(rows))
	for i, r := range rows {
		tabs[i] = Tab{ID: r.ID, Title: r.Title}
	}
	return tabs, nil
}

// ListTabLinksInWindow は指定 window の tab URL 一覧を返す（focus 奪わない）。
func (d *Driver) ListTabLinksInWindow(ctx context.Context, wid string) ([]string, error) {
	cmd := exec.CommandContext(ctx, d.ChromeCli, "list", "links", "-w", wid)
	cmd.Env = append(os.Environ(), "CHROME_BUNDLE_IDENTIFIER="+d.BundleID)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("chrome-cli list links: %w", err)
	}
	var urls []string
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// 形式: "[id] URL"
		if i := strings.Index(line, "] "); i >= 0 {
			u := strings.TrimSpace(line[i+2:])
			if u != "" {
				urls = append(urls, u)
			}
		}
	}
	return urls, nil
}

// parseListing は chrome-cli の "[<id>] <text>" 出力をパースする。
func parseListing(s string) []Window {
	var out []Window
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || !strings.HasPrefix(line, "[") {
			continue
		}
		end := strings.Index(line, "]")
		if end < 0 {
			continue
		}
		id := line[1:end]
		title := strings.TrimSpace(line[end+1:])
		out = append(out, Window{ID: id, Title: title})
	}
	return out
}

// SpawnProjectWindow は new window を作成して指定 profile + URL list を開く。
//
// marker tab は使わない。spawn 直前後の window-id 集合 diff で **新規 wid** を
// 取得して返す（呼び出し側が state.Window.LiveWindowID に保存する）。
//
// 一瞬 browser に focus が奪われるが、frontmost は事前に保存されており
// 呼び出し側 (SpawnAndRestoreFocus) で復帰する。
func (d *Driver) SpawnProjectWindow(ctx context.Context, profile string, urls []string) (string, error) {
	preWins, _ := d.ListWindows(ctx)
	preIDs := map[string]bool{}
	for _, w := range preWins {
		preIDs[w.ID] = true
	}

	args := []string{"-na", d.AppPath, "--args"}
	if profile != "" {
		args = append(args, "--profile-directory="+profile)
	}
	args = append(args, "--new-window")
	if len(urls) > 0 {
		args = append(args, urls...)
	} else {
		// URL なしだと Vivaldi が welcome / start page を出す。新 window 指示の効果を
		// 確実にするため about:blank を渡す。
		args = append(args, "about:blank")
	}

	cmd := exec.CommandContext(ctx, "open", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("open -na: %w (out=%s)", err, strings.TrimSpace(string(out)))
	}

	// 新 window-id を待つ（最大 ~6 秒）
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		wins, err := d.ListWindows(ctx)
		if err != nil {
			continue
		}
		for _, w := range wins {
			if !preIDs[w.ID] {
				return w.ID, nil
			}
		}
	}
	return "", fmt.Errorf("spawn succeeded but new window-id not detected within 6s")
}

// SnapshotURLs は指定 window の現在 tab URL 一覧を返す（focus 奪わない）。
func (d *Driver) SnapshotURLs(ctx context.Context, wid string) ([]string, error) {
	urls, err := d.ListTabLinksInWindow(ctx, wid)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		// chrome-extension://... の welcome 等は除外
		if strings.HasPrefix(u, "chrome-extension://") {
			continue
		}
		// about:blank も除外（ユーザに restore する意味なし）
		if u == "about:blank" {
			continue
		}
		out = append(out, u)
	}
	return out, nil
}

// CloseWindow は指定 window を閉じる（focus 一瞬奪う）。
func (d *Driver) CloseWindow(ctx context.Context, wid string) error {
	cmd := exec.CommandContext(ctx, d.ChromeCli, "close", "-w", wid)
	cmd.Env = append(os.Environ(), "CHROME_BUNDLE_IDENTIFIER="+d.BundleID)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chrome-cli close -w: %w (out=%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// WindowExists は wid の window が現存するか返す（focus 奪わない、stale wid 検出用）。
func (d *Driver) WindowExists(ctx context.Context, wid string) bool {
	wins, err := d.ListWindows(ctx)
	if err != nil {
		return false
	}
	for _, w := range wins {
		if w.ID == wid {
			return true
		}
	}
	return false
}

// SpawnAndRestoreFocus は SpawnProjectWindow を frontmost 復帰付きで呼ぶ。
func (d *Driver) SpawnAndRestoreFocus(ctx context.Context, profile string, urls []string) (string, error) {
	var wid string
	err := d.withFocusRestore(func() error {
		w, e := d.SpawnProjectWindow(ctx, profile, urls)
		wid = w
		return e
	})
	return wid, err
}

// SnapshotAndCloseAndRestoreFocus は wid の snapshot → close を frontmost 復帰
// 付きで実行する。snapshot 失敗時は close も skip。
//
// wid が stale (window 不在) なら no-op で nil 返す（saved_urls は前回値が残る）。
func (d *Driver) SnapshotAndCloseAndRestoreFocus(ctx context.Context, wid string) ([]string, error) {
	if !d.WindowExists(ctx, wid) {
		d.logf("close: window %s not found (stale wid, no-op)", wid)
		return nil, nil
	}
	var urls []string
	err := d.withFocusRestore(func() error {
		var snapErr error
		urls, snapErr = d.SnapshotURLs(ctx, wid)
		if snapErr != nil {
			return snapErr
		}
		return d.CloseWindow(ctx, wid)
	})
	return urls, err
}
