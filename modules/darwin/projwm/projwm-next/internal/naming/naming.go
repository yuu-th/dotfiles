// Package naming は (kind, id, project) から ghostty title / tmux session 名 /
// viewer title / viewer tmux 名 を決定的に算出するための関数群を提供する。
//
// 派生文字列は state.json には保存しない（projwm-design.md §5.3 / §6.3.2）。
//
// projwm-next の DesiredWindow.TitleContract.Expected は Reducer 側で
// `<kind>-<index>:<project>` 形式に固定されているので、本パッケージでは
// title を逆 parse して tmux session 名を導く helper も提供する
// （SESS.1 spawn-time wiring が SpawnRequest.Title から session 名を決定するため）。
package naming

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Kind は windows[].kind の取り得る値（terminal class のみ）。
type Kind string

const (
	KindAI    Kind = "ai"
	KindShell Kind = "shell"
)

// AI は AI 種別。
type AI string

const (
	AIClaude  AI = "claude"
	AICopilot AI = "copilot"
)

// AICommand は AI 種別から起動コマンドを返す。
func AICommand(a AI) string {
	switch a {
	case AIClaude:
		return "claude"
	case AICopilot:
		return "copilot"
	}
	return ""
}

// GhosttyTitle は ai/shell window の Ghostty title を返す。
//
//	(KindAI, 1, "dotfiles")    → "ai-1:dotfiles"
//	(KindShell, 2, "dotfiles") → "shell-2:dotfiles"
func GhosttyTitle(kind Kind, id int, project string) string {
	return fmt.Sprintf("%s-%d:%s", kind, id, project)
}

// TmuxSession は ai/shell window の tmux session 名を返す。
//
//	(KindAI, 1, "dotfiles")    → "ai-1/dotfiles"
//	(KindShell, 2, "dotfiles") → "shell-2/dotfiles"
//
// `:` は tmux session 名で `_` に置換されるため `/` を採用。
func TmuxSession(kind Kind, id int, project string) string {
	return fmt.Sprintf("%s-%d/%s", kind, id, project)
}

// ViewerGhosttyTitle は AI viewer window の title を返す。
//
//	(1, "dotfiles") → "ai-view-1:dotfiles"
func ViewerGhosttyTitle(id int, project string) string {
	return fmt.Sprintf("ai-view-%d:%s", id, project)
}

// ViewerTmuxSession は viewer 用 grouped clone の session 名を返す。
//
//	(1, "dotfiles") → "ai-1/dotfiles_v"
//
// session 名内の `:` 自動置換を避けるため `_v` 末尾を採用（POC-13）。
func ViewerTmuxSession(id int, project string) string {
	return fmt.Sprintf("ai-%d/%s_v", id, project)
}

// ZedTitle は editor (Zed) window の title を返す（cwd basename）。
func ZedTitle(cwd string) string {
	return filepath.Base(cwd)
}

// TmuxSessionFromTitle は controller-owned title からから対応する tmux session 名を導く。
// 戻り値: (sessionName, ok)。
//
//	"ai-1:dotfiles"      → "ai-1/dotfiles", true
//	"shell-2:dotfiles"   → "shell-2/dotfiles", true
//	"ai-view-1:dotfiles" → "ai-1/dotfiles_v", true (viewer は AI grouped clone)
//	"dotfiles"           → "", false (Zed natural title など)
func TmuxSessionFromTitle(title string) (string, bool) {
	colon := strings.IndexByte(title, ':')
	if colon < 0 {
		return "", false
	}
	prefix, project := title[:colon], title[colon+1:]
	if project == "" {
		return "", false
	}
	if strings.HasPrefix(prefix, "ai-view-") {
		idx := strings.TrimPrefix(prefix, "ai-view-")
		if idx == "" {
			return "", false
		}
		return fmt.Sprintf("ai-%s/%s_v", idx, project), true
	}
	// `<kind>-<id>` shape (ai-1, shell-2 など)
	dash := strings.IndexByte(prefix, '-')
	if dash <= 0 || dash == len(prefix)-1 {
		return "", false
	}
	return fmt.Sprintf("%s/%s", prefix, project), true
}

// SourceAITmuxSessionFromViewerTitle は viewer title から source AI session 名を導く。
//
//	"ai-view-1:dotfiles" → "ai-1/dotfiles", true
func SourceAITmuxSessionFromViewerTitle(viewerTitle string) (string, bool) {
	const prefix = "ai-view-"
	if !strings.HasPrefix(viewerTitle, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(viewerTitle, prefix)
	colon := strings.IndexByte(rest, ':')
	if colon <= 0 {
		return "", false
	}
	return fmt.Sprintf("ai-%s/%s", rest[:colon], rest[colon+1:]), true
}

// IsViewerTitle は title が viewer (workspace A 用 grouped clone) のものかを返す。
func IsViewerTitle(title string) bool {
	return strings.HasPrefix(title, "ai-view-")
}

// IsAITitle は title が AI window (kind=ai, viewer ではない) のものかを返す。
func IsAITitle(title string) bool {
	return strings.HasPrefix(title, "ai-") && !IsViewerTitle(title)
}
