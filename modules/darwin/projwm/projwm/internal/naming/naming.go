// Package naming は (kind, id, project) から ghostty title / tmux session 名 /
// viewer title / viewer tmux 名 を **決定的に算出する唯一の真実関数** を提供する。
// state.json には title 等の派生文字列を保存しないため、リネーム時の不整合バグが
// 構造的に発生しない（projwm-design.md §5.3 / §6.3.2）。
package naming

import (
	"fmt"
	"path/filepath"
)

// Kind は windows[].kind の取り得る値。
type Kind string

const (
	KindAI      Kind = "ai"
	KindShell   Kind = "shell"
	KindEditor  Kind = "editor"
	KindBrowser Kind = "browser" // v12: Vivaldi window per project
)

// IsValidKind は kind が許容値かを返す。
func IsValidKind(k Kind) bool {
	return k == KindAI || k == KindShell || k == KindEditor || k == KindBrowser
}

// AI は windows[].ai の取り得る値（kind="ai" のみ）。
type AI string

const (
	AIClaude  AI = "claude"
	AICopilot AI = "copilot"
)

// IsValidAI は ai が許容値かを返す。
func IsValidAI(a AI) bool {
	return a == AIClaude || a == AICopilot
}

// AICommand は AI 種別から起動コマンドを返す（projwm-design.md §5.1: 「AI 本体（Claude
// or Copilot）」が tmux session で走る）。tmux send-keys で打鍵する文字列。
func AICommand(a AI) string {
	switch a {
	case AIClaude:
		return "claude"
	case AICopilot:
		return "copilot"
	}
	return ""
}

// GhosttyTitle は ai/shell window の ghostty title を返す。
//
//	(KindAI, 1, "dotfiles")    → "ai-1:dotfiles"
//	(KindShell, 2, "dotfiles") → "shell-2:dotfiles"
//	(KindEditor, *, *)         → panic（editor は Zed 自然 title なので呼ぶべきでない）
func GhosttyTitle(kind Kind, id int, project string) string {
	if kind == KindEditor {
		panic("naming.GhosttyTitle: editor kind has no ghostty title (use ZedTitle)")
	}
	return fmt.Sprintf("%s-%d:%s", kind, id, project)
}

// TmuxSession は ai/shell window の tmux session 名を返す。
//
//	(KindAI, 1, "dotfiles")    → "ai-1/dotfiles"
//	(KindShell, 2, "dotfiles") → "shell-2/dotfiles"
//	(KindEditor, *, *)         → panic（editor は tmux ラップしない）
func TmuxSession(kind Kind, id int, project string) string {
	if kind == KindEditor {
		panic("naming.TmuxSession: editor kind has no tmux session")
	}
	return fmt.Sprintf("%s-%d/%s", kind, id, project)
}

// ViewerGhosttyTitle は AI window の viewer (WS A) 側 ghostty title を返す。
//
//	(1, "dotfiles") → "ai-view-1:dotfiles"
func ViewerGhosttyTitle(id int, project string) string {
	return fmt.Sprintf("ai-view-%d:%s", id, project)
}

// ViewerTmuxSession は AI window の viewer 用 grouped clone の tmux session 名を返す。
//
//	(1, "dotfiles") → "ai-1/dotfiles_v"
//
// 注: tmux session 名内の `:` は自動的に `_` 置換されるため、`_v` 末尾を採用
// (POC-13 / projwm-design.md v11.2)。
func ViewerTmuxSession(id int, project string) string {
	return fmt.Sprintf("ai-%d/%s_v", id, project)
}

// ZedTitle は editor (Zed) window の title を返す（cwd の basename）。
//
//	("/Users/yuta/dev/dotfiles") → "dotfiles"
//
// projwm 規約ではなく Zed の自然 title。bundleId `dev.zed.Zed` + title 完全一致で
// omniwmctl から識別する（projwm-design.md §5.3）。
func ZedTitle(cwd string) string {
	return filepath.Base(cwd)
}

// ZedBundleID は Zed の macOS bundleId（不変）。
const ZedBundleID = "dev.zed.Zed"

// TerminalBundleID は projwm の terminal driver (純正 Ghostty.app) の bundleId。
// v11.3 で kitty user-space copy に一旦切替えたが、v11.6 で OmniWM app-rules
// に titleRegex rule を追加することで純正 Ghostty に戻した。
const TerminalBundleID = "com.mitchellh.ghostty"

// VivaldiBundleID は browser kind の bundleId（v12）。
const VivaldiBundleID = "com.vivaldi.Vivaldi"

// BrowserMarkerTitle は browser window の identifying marker tab title を返す。
// projwm は file:// HTML marker を spawn 直後に navigate して、Window menu の
// tab list scan で title 一致 → window 識別する（v12, projwm-history.md）。
//
//	(1, "dotfiles") → "projwm:dotfiles:1"
func BrowserMarkerTitle(id int, project string) string {
	return fmt.Sprintf("projwm:%s:%d", project, id)
}
