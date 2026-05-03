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
	"crypto/rand"
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

// SpawnResult は SpawnProjectWindow の戻り値。
//
// MarkerTitle は OmniWM 側で window 識別するための文字列 (window title が
// "<MarkerTitle> - Vivaldi" になる時間がある = spawn 直後 active tab が marker)。
// 並列 spawn でも同 uuid が他 spawn と衝突しないため race-free。
type SpawnResult struct {
	WindowID    string // chrome-cli の window-id
	MarkerTitle string // 識別用 page title (Vivaldi window title prefix)
}

// SpawnProjectWindow は new window を作成して指定 profile + URL list を開く。
//
// **完全 race-free 並列識別 (uuid marker 戦略)**:
//  1. spawn URL list の **先頭** に projwm 専用 uuid marker (file:// HTML) を置く
//  2. Vivaldi は先頭 URL を active tab にするため、 spawn 直後 window title = "projwm-spawn-<uuid> - Vivaldi"
//  3. chrome-cli list links で marker URL を含む window を識別 (race-free)
//  4. 同時に呼び出し側で OmniWM windows query で title 一致 window を識別 (race-free)
//  5. 識別後 marker tab を close → active が saved_urls[0] に自動 fallback
//
// 並列に 2 windows spawn しても uuid が異なるため互いに区別可能。
// 一瞬 browser に focus が奪われるが、frontmost は事前保存・後で復帰する。
func (d *Driver) SpawnProjectWindow(ctx context.Context, profile string, urls []string) (*SpawnResult, error) {
	markerURL, markerPath, markerTitle, err := writeSpawnMarker()
	if err != nil {
		return nil, fmt.Errorf("create spawn marker: %w", err)
	}
	// 削除は遅延 goroutine で (defer だと Vivaldi が file を load する前に消えて
	// <title> が反映されず、識別失敗の致命バグになる)。
	// 60 秒後に削除すれば、Vivaldi が title を read 済の保証になる。
	go func() {
		time.Sleep(60 * time.Second)
		_ = os.Remove(markerPath)
	}()

	args := []string{"-na", d.AppPath, "--args"}
	if profile != "" {
		args = append(args, "--profile-directory="+profile)
	}
	args = append(args, "--new-window", markerURL) // 先頭に marker (active tab になる)
	args = append(args, urls...)
	if len(urls) == 0 {
		// saved_urls 0 でも user に見せるための fallback (marker close 後に空 tab 状態)
		args = append(args, "about:blank")
	}

	cmd := exec.CommandContext(ctx, "open", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("open -na: %w (out=%s)", err, strings.TrimSpace(string(out)))
	}

	// 新 window-id を polling: 全 windows の tab links を見て markerURL を含む
	// window を採用 (race-free、並列 spawn 安全)。最大 ~6 秒、150ms 間隔。
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(150 * time.Millisecond)
		wins, err := d.ListWindows(ctx)
		if err != nil {
			continue
		}
		for _, w := range wins {
			links, err := d.ListTabLinksInWindow(ctx, w.ID)
			if err != nil {
				continue
			}
			for _, u := range links {
				if u == markerURL {
					return &SpawnResult{WindowID: w.ID, MarkerTitle: markerTitle}, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("spawn succeeded but marker-tagged window not detected within 6s")
}

// CloseMarkerTab は SpawnProjectWindow が active tab に置いた marker を close する。
// move-to-slot 等の OmniWM 識別作業が終わったタイミングで呼ぶ。
// 失敗しても致命でない (marker file 自体は短時間で OS 再起動時に消える)。
func (d *Driver) CloseMarkerTab(ctx context.Context, wid, markerTitle string) error {
	// markerTitle = "projwm-spawn-<uuid>" (page title 全体)
	// closeTabByTitle で url 経由じゃなく title 経由で探す方が確実 (URL は file://...)
	return d.closeTabByTitle(ctx, wid, markerTitle)
}

func (d *Driver) closeTabByTitle(ctx context.Context, wid, target string) error {
	tabsOut, err := exec.CommandContext(ctx, d.ChromeCli, "list", "tabs", "-w", wid).Output()
	if err != nil {
		return err
	}
	tabs := parseListing(string(tabsOut))
	for _, t := range tabs {
		// title は marker page の <title>projwm-spawn-uuid</title>
		if t.Title == target {
			cmd := exec.CommandContext(ctx, d.ChromeCli, "close", "-t", t.ID)
			cmd.Env = append(os.Environ(), "CHROME_BUNDLE_IDENTIFIER="+d.BundleID)
			return cmd.Run()
		}
	}
	return nil
}

// writeSpawnMarker は uuid 入りの一時 HTML を ~/.cache/projwm/spawn-markers/
// 配下に書き出して file:// URL と pageTitle を返す。識別終わったら呼び出し側で os.Remove。
//
// pageTitle = "projwm-spawn-<uuid>" は OmniWM 側 window title 識別に使う。
func writeSpawnMarker() (markerURL, path, pageTitle string, err error) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".cache", "projwm", "spawn-markers")
	if e := os.MkdirAll(dir, 0o755); e != nil {
		return "", "", "", e
	}
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", "", "", e
	}
	uuid := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	pageTitle = "projwm-spawn-" + uuid
	p := filepath.Join(dir, uuid+".html")
	html := fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8">
<title>%s</title></head><body style="background:#fafafa;color:#888;font-family:system-ui;padding:2em">
<h2>projwm spawn marker</h2><p>This tab is auto-closed by projwm.</p></body></html>`, pageTitle)
	if e := os.WriteFile(p, []byte(html), 0o644); e != nil {
		return "", "", "", e
	}
	return "file://" + p, p, pageTitle, nil
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
// 識別用 marker title も返す (呼び出し側が OmniWM 識別に使う)。
func (d *Driver) SpawnAndRestoreFocus(ctx context.Context, profile string, urls []string) (*SpawnResult, error) {
	var res *SpawnResult
	err := d.withFocusRestore(func() error {
		r, e := d.SpawnProjectWindow(ctx, profile, urls)
		res = r
		return e
	})
	return res, err
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
