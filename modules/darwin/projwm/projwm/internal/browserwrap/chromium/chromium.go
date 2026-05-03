// Package chromium は Chromium 系 browser (Vivaldi/Brave/Edge/Chrome/Arc) を
// chrome-cli + osascript で外部制御する driver (paradigm C)。
//
// 設計指針 (POC, projwm-history.md "v12 paradigm 変遷"):
//
//   - **read 系 (`chrome-cli list windows/tabs`) は focus 奪わない** ので
//     状態 query / URL snapshot は自由に呼べる。
//   - **write 系 (`chrome-cli close` / `open -na ... --new-window`) は focus を奪う**
//     ため、destructive 操作の前後で frontmost app を保存・復帰する。
//   - **per-project window**: 各 project の browser window は独立。Chromium の
//     `--profile-directory` で user profile (cookies/login) を分離。
//   - **window 識別**: spawn 時に projwm 側で chrome-cli list windows の差分から
//     新 window-id を捕捉。再起動で wid が変わるので、永続化はせず in-memory map +
//     marker URL 照合 (1st tab が `file://.../<project>-<id>.html`) で再 establish。
//
// 通常運用では projwm はこの driver を **何も呼ばない**。reconcile は idempotent な
// no-op で、user の作業中に focus を奪うことはない。
package chromium

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
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
	// MarkersDir は marker HTML を置く path（既定 ~/.cache/projwm/browser-markers）。
	MarkersDir string
	// ChromeCli は chrome-cli の binary path（既定 /opt/homebrew/bin/chrome-cli）。
	ChromeCli string
	// Logger は trace 出力先（既定 io.Discard）。
	Logger io.Writer
}

// New は Vivaldi 既定値で Driver を返す。
func New() *Driver {
	home, _ := os.UserHomeDir()
	return &Driver{
		AppPath:    "/Applications/Vivaldi.app",
		BundleID:   naming.VivaldiBundleID,
		MarkersDir: filepath.Join(home, ".cache", "projwm", "browser-markers"),
		ChromeCli:  "/opt/homebrew/bin/chrome-cli",
		Logger:     io.Discard,
	}
}

func (d *Driver) logf(format string, args ...any) {
	if d.Logger == nil {
		return
	}
	fmt.Fprintf(d.Logger, "[chromium] "+format+"\n", args...)
}

// MarkerFilePath は project / id 用 marker HTML の path を返す。
func (d *Driver) MarkerFilePath(project string, id int) string {
	return filepath.Join(d.MarkersDir, fmt.Sprintf("%s-%d.html", project, id))
}

// MarkerURL は marker file の file:// URL を返す。
func (d *Driver) MarkerURL(project string, id int) string {
	p := d.MarkerFilePath(project, id)
	return "file://" + (&url.URL{Path: p}).EscapedPath()
}

// EnsureMarkerFile は marker HTML を生成する（既にあれば overwrite）。
func (d *Driver) EnsureMarkerFile(project string, id int) (string, error) {
	if err := os.MkdirAll(d.MarkersDir, 0o755); err != nil {
		return "", err
	}
	p := d.MarkerFilePath(project, id)
	title := naming.BrowserMarkerTitle(id, project)
	html := fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>%s</title>
<style>body{font-family:system-ui;padding:2em;color:#888;background:#fafafa}h2{color:#444}code{background:#eee;padding:.1em .4em;border-radius:3px}</style>
</head><body><h2>projwm marker</h2>
<p>This window belongs to projwm.</p>
<p>project: <code>%s</code><br>browser id: <code>%d</code></p>
<p style="font-size:.8em;color:#aaa">Closing this tab desyncs the window from projwm. Run <code>projwm reconcile</code> to recover.</p>
</body></html>`, htmlEscape(title), htmlEscape(project), id)
	return p, os.WriteFile(p, []byte(html), 0o644)
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&#39;")
	return r.Replace(s)
}

// Frontmost は現在 frontmost な application プロセス名を返す（focus 復帰用）。
// 例: "Ghostty", "Finder", "Vivaldi"。
func Frontmost() (string, error) {
	out, err := exec.Command("osascript", "-e",
		`tell application "System Events" to get name of first application process whose frontmost is true`).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Activate は指定 application を frontmost にする（focus 復帰）。
// 失敗しても致命でないので error は warn 用。
func Activate(name string) error {
	if name == "" {
		return nil
	}
	return exec.Command("osascript", "-e",
		fmt.Sprintf(`tell application %q to activate`, name)).Run()
}

// withFocusRestore は fn 実行の前後で frontmost を保存・復帰する。fn が失敗しても
// frontmost 復帰は試みる（defer）。
//
// Vivaldi は new-window 後に「welcome page loading 完了」「onboarding modal 表示」等
// で遅延して focus を奪い返してくる。対策: defer 内で複数回（合計 ~3 秒）に渡って
// Activate を試みる。
func (d *Driver) withFocusRestore(fn func() error) error {
	prev, err := Frontmost()
	if err != nil {
		d.logf("WARN: get frontmost: %v", err)
	}
	defer func() {
		if prev == "" {
			return
		}
		if isVivaldiProc(prev) {
			return // 元々 Vivaldi が前面なら何もしない
		}
		// Vivaldi が遅延 grab する分も含めて押し戻す。
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

func isVivaldiProc(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "vivaldi")
}

// Window は chrome-cli list windows の 1 行をパースした結果。
type Window struct {
	ID    string // chrome-cli の window-id（数値文字列）
	Title string // active tab title or window title
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
		// chrome-cli は browser 未起動時に "Waiting for chrome to start..." で
		// 15 秒 block するため、その前に process check する設計が望ましい。
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
// chrome-cli の `list links -w <wid>` 出力をパース。
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

// FindWindowByMarker は marker URL を tab に持つ window を探す。
// 再起動で wid が変わった後の re-establish 用（focus 奪わない）。
func (d *Driver) FindWindowByMarker(ctx context.Context, project string, id int) (string, bool, error) {
	wins, err := d.ListWindows(ctx)
	if err != nil {
		return "", false, err
	}
	target := d.MarkerURL(project, id)
	for _, w := range wins {
		urls, err := d.ListTabLinksInWindow(ctx, w.ID)
		if err != nil {
			continue
		}
		for _, u := range urls {
			if strings.HasPrefix(u, target) {
				return w.ID, true, nil
			}
		}
	}
	return "", false, nil
}

// SpawnProjectWindow は new window を作成して指定 profile + URL list を開く。
// 一瞬 Vivaldi に focus が奪われるが、frontmost は事前に保存されており呼び出し
// 側で復帰する想定。
//
// Vivaldi (Chromium) は新 profile の初回起動で welcome 画面を表示し、
// `--new-window URL` で渡した URL を hijack して無視する場合がある。確実に
// marker URL と URL list を tab として登録するため、二段階で行う:
//
//  1. `open -na Vivaldi --profile-directory=X --new-window <markerURL>`
//     新 window を作る、marker URL を 1st tab に
//  2. 残り URL を `chrome-cli open URL -w <wid>` で追加
//
// 復帰した window-id を返す（後続の chrome-cli 操作で使う）。
func (d *Driver) SpawnProjectWindow(ctx context.Context, project string, id int, profile string, urls []string) (string, error) {
	// 既存 window がある？
	if wid, found, _ := d.FindWindowByMarker(ctx, project, id); found {
		d.logf("spawn: existing window %s for %s-%d", wid, project, id)
		return wid, nil
	}

	if _, err := d.EnsureMarkerFile(project, id); err != nil {
		return "", fmt.Errorf("marker file: %w", err)
	}
	markerURL := d.MarkerURL(project, id)

	// spawn 前の window-id 集合
	preWins, _ := d.ListWindows(ctx)
	preIDs := map[string]bool{}
	for _, w := range preWins {
		preIDs[w.ID] = true
	}

	// Phase 1: new window with marker URL only
	args := []string{"-na", d.AppPath, "--args"}
	if profile != "" {
		args = append(args, "--profile-directory="+profile)
	}
	args = append(args, "--new-window", markerURL)

	cmd := exec.CommandContext(ctx, "open", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("open -na Vivaldi: %w (out=%s)", err, strings.TrimSpace(string(out)))
	}

	// 新 window-id を待つ（最大 ~6 秒）
	var wid string
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		wins, err := d.ListWindows(ctx)
		if err != nil {
			continue
		}
		for _, w := range wins {
			if !preIDs[w.ID] {
				wid = w.ID
				break
			}
		}
		if wid != "" {
			break
		}
	}
	// fallback: marker URL で identify
	if wid == "" {
		if found_id, found, _ := d.FindWindowByMarker(ctx, project, id); found {
			wid = found_id
		}
	}
	if wid == "" {
		return "", fmt.Errorf("spawn succeeded but new window-id not detected")
	}
	d.logf("spawn: new window %s for %s-%d", wid, project, id)

	// Phase 2: marker URL が確実に tab として存在するかを確認、無ければ追加
	tabsURLs, _ := d.ListTabLinksInWindow(ctx, wid)
	hasMarker := false
	for _, u := range tabsURLs {
		if strings.HasPrefix(u, markerURL) {
			hasMarker = true
			break
		}
	}
	if !hasMarker {
		// marker URL が welcome に hijack された → chrome-cli で再度追加
		if err := d.openInWindow(ctx, markerURL, wid); err != nil {
			d.logf("WARN: re-open marker URL: %v", err)
		}
	}

	// Phase 3: 残り URL を chrome-cli で追加
	for _, u := range urls {
		if err := d.openInWindow(ctx, u, wid); err != nil {
			d.logf("WARN: open URL %q: %v", u, err)
		}
	}

	return wid, nil
}

// openInWindow は指定 window に新 tab で URL を開く（focus 一瞬奪う）。
func (d *Driver) openInWindow(ctx context.Context, openURL, wid string) error {
	cmd := exec.CommandContext(ctx, d.ChromeCli, "open", openURL, "-w", wid)
	cmd.Env = append(os.Environ(), "CHROME_BUNDLE_IDENTIFIER="+d.BundleID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("chrome-cli open: %w (out=%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SnapshotURLs は指定 window の現在 tab URL 一覧を返す（focus 奪わない）。
// marker URL は除外する（再 spawn 時に projwm が再注入する）。
func (d *Driver) SnapshotURLs(ctx context.Context, wid string, project string, id int) ([]string, error) {
	urls, err := d.ListTabLinksInWindow(ctx, wid)
	if err != nil {
		return nil, err
	}
	marker := d.MarkerURL(project, id)
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if strings.HasPrefix(u, marker) {
			continue
		}
		// chrome-extension://... の welcome 等は除外（user-meaningful URL のみ）
		if strings.HasPrefix(u, "chrome-extension://") {
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

// SpawnAndRestoreFocus は SpawnProjectWindow を frontmost 復帰付きで呼ぶ。
func (d *Driver) SpawnAndRestoreFocus(ctx context.Context, project string, id int, profile string, urls []string) (string, error) {
	var wid string
	err := d.withFocusRestore(func() error {
		w, e := d.SpawnProjectWindow(ctx, project, id, profile, urls)
		wid = w
		return e
	})
	return wid, err
}

// SnapshotAndCloseAndRestoreFocus は wid の URL snapshot → close を 1 セットで行い
// frontmost を復帰する。snapshot 失敗時は close も skip。
func (d *Driver) SnapshotAndCloseAndRestoreFocus(ctx context.Context, wid string, project string, id int) ([]string, error) {
	var urls []string
	err := d.withFocusRestore(func() error {
		var snapErr error
		urls, snapErr = d.SnapshotURLs(ctx, wid, project, id)
		if snapErr != nil {
			return snapErr
		}
		return d.CloseWindow(ctx, wid)
	})
	return urls, err
}

// IsRunning は browser が走っているかを返す（chrome-cli wait を回避するため事前 check）。
func (d *Driver) IsRunning() bool {
	out, err := exec.Command("pgrep", "-x", filepath.Base(strings.TrimSuffix(d.AppPath, ".app"))).Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}
