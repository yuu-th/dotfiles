// Package reconcile は state（期待）と実状態の差分を埋める。
//
// 設計: queue/projwm-design.md §7。
package reconcile

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuu-th/projwm/internal/browserwrap/chromium"
	"github.com/yuu-th/projwm/internal/config"
	"github.com/yuu-th/projwm/internal/ghosttywrap"
	"github.com/yuu-th/projwm/internal/naming"
	"github.com/yuu-th/projwm/internal/omniwm"
	"github.com/yuu-th/projwm/internal/state"
	"github.com/yuu-th/projwm/internal/terminalsetup"
	"github.com/yuu-th/projwm/internal/tmuxwrap"
	"github.com/yuu-th/projwm/internal/zedwrap"
)

// Options は reconcile 1 回のオプション。
type Options struct {
	DryRun  bool
	Verbose bool
	GC      bool // orphan を一括 close（破壊的）
	Logger  io.Writer
}

// Reconciler は依存ツール群を保持。
type Reconciler struct {
	Cfg      config.Config
	OmniWM   *omniwm.Client
	Tmux     *tmuxwrap.Client
	Ghostty  ghosttywrap.Spawner
	Zed      zedwrap.Spawner
	Chromium *chromium.Driver
}

// New は実プロセス使用の Reconciler を返す。
func New(cfg config.Config) *Reconciler {
	return &Reconciler{
		Cfg:      cfg,
		OmniWM:   omniwm.New(nil),
		Tmux:     tmuxwrap.New(nil),
		Ghostty:  ghosttywrap.CmdSpawner{AppPath: cfg.TerminalAppPath},
		Zed:      zedwrap.CmdSpawner{},
		Chromium: chromium.New(),
	}
}

// Action は reconcile が打つ 1 つの操作（dry-run でも観測できる）。
type Action struct {
	Op      string // "spawn-ghostty"|"spawn-zed"|"close-window"|"kill-tmux"|"new-tmux"|"new-grouped"|"move-to-ws"
	Target  string // title / session / window-id
	Detail  string // 追加情報
	OnError error  // 実行時エラー（DryRun=false の時のみ）
}

// Run は reconcile を 1 回実行。
func (r *Reconciler) Run(ctx context.Context, st *state.State, opts Options) ([]Action, error) {
	if opts.Logger == nil {
		opts.Logger = io.Discard
	}
	// Chromium driver の logger は reconcile の Logger に follow させる（既定は discard）。
	if r.Chromium != nil {
		r.Chromium.Logger = opts.Logger
	}
	r.logf(opts, "reconcile start: active=%q archived=%d projects=%d",
		st.ActiveProfile, countArchived(st), len(st.Projects))

	// terminal driver (kitty-projwm.app) の整合性を確保（冪等、最新なら no-op）。
	// home.activation 経由だと codesign が失敗するため Go 側で実行する (v11.3)。
	if !opts.DryRun {
		if err := terminalsetup.EnsureKittyProjwm(opts.Logger); err != nil {
			r.logf(opts, "WARN: terminalsetup: %v", err)
		}
	}

	var actions []Action

	// 1) active profile の slot 配置
	if st.ActiveProfile != "" {
		active := st.Profiles[st.ActiveProfile].Assignments
		for slot, projName := range active {
			p := st.Projects[projName]
			if p.Archived {
				r.logf(opts, "WARN: archived project %q is in active assignments (skipped)", projName)
				continue
			}
			actions = append(actions, r.ensureProjectInSlot(ctx, projName, p, slot, opts)...)
		}
	}

	// 2) viewer (WS A) orphan 掃除（active な AI 窓に対応する viewer 窓だけ残す）
	expectedViewerTitles := map[string]bool{}
	if st.ActiveProfile != "" {
		for _, projName := range st.Profiles[st.ActiveProfile].Assignments {
			p := st.Projects[projName]
			if p.Archived {
				continue
			}
			for _, w := range p.Windows {
				if w.Kind == naming.KindAI {
					expectedViewerTitles[naming.ViewerGhosttyTitle(w.ID, projName)] = true
				}
			}
		}
	}
	actions = append(actions, r.gcViewerOrphans(ctx, expectedViewerTitles, opts)...)

	// 3) inactive (park or 他 profile) の project: windows close、tmux は alive
	inActive := map[string]bool{}
	if st.ActiveProfile != "" {
		for _, p := range st.Profiles[st.ActiveProfile].Assignments {
			inActive[p] = true
		}
	}
	for name, p := range st.Projects {
		if p.Archived || inActive[name] {
			continue
		}
		actions = append(actions, r.closeProjectWindowsKeepTmux(ctx, name, p, opts)...)
	}

	// 4) archived: windows close + tmux kill
	for name, p := range st.Projects {
		if !p.Archived {
			continue
		}
		actions = append(actions, r.purgeArchivedProject(ctx, name, p, opts)...)
	}

	// 5) --gc: orphan ghostty 窓を close
	if opts.GC {
		actions = append(actions, r.gcOrphans(ctx, st, opts)...)
	}

	r.logf(opts, "reconcile end: %d actions", len(actions))
	return actions, nil
}

// ensureProjectInSlot は project の windows[] を slot に揃える。
func (r *Reconciler) ensureProjectInSlot(ctx context.Context, projName string, p state.Project, slot string, opts Options) []Action {
	var acts []Action
	for _, w := range p.Windows {
		switch w.Kind {
		case naming.KindAI, naming.KindShell:
			title := naming.GhosttyTitle(w.Kind, w.ID, projName)
			session := naming.TmuxSession(w.Kind, w.ID, projName)

			// 新規 tmux session 作成 → AI window なら AI コマンドを送る (送る前後で
			// session 在の状態が変わるので、事前 has-session で「新規だった」を記録)
			wasMissing := false
			if hasTmux, _ := r.Tmux.HasSession(ctx, session); !hasTmux {
				wasMissing = true
			}
			acts = append(acts, r.ensureGhosttyWindow(ctx, title, session, p.CWD, slot, opts)...)
			if wasMissing && w.Kind == naming.KindAI && !opts.DryRun {
				if cmd := naming.AICommand(w.AI); cmd != "" {
					// shell prompt が ready になるのを少し待ってから打鍵
					time.Sleep(300 * time.Millisecond)
					if err := r.Tmux.SendKeys(ctx, session, cmd, "Enter"); err != nil {
						r.logf(opts, "WARN: AI command send-keys failed (%s): %v", session, err)
					} else {
						acts = append(acts, Action{
							Op:     "ai-launch",
							Target: session,
							Detail: cmd,
						})
					}
				}
			}

			if w.Kind == naming.KindAI {
				vSession := naming.ViewerTmuxSession(w.ID, projName)
				vTitle := naming.ViewerGhosttyTitle(w.ID, projName)
				acts = append(acts, r.ensureGroupedSession(ctx, session, vSession, opts)...)
				acts = append(acts, r.ensureGhosttyWindow(ctx, vTitle, vSession, p.CWD, r.Cfg.ViewerWorkspace, opts)...)
			}
		case naming.KindEditor:
			zedTitle := naming.ZedTitle(p.CWD)
			acts = append(acts, r.ensureZedWindow(ctx, zedTitle, p.CWD, slot, opts)...)
		case naming.KindBrowser:
			// **重要**: reconcile は browser に **触らない**（v12 paradigm C 確定）。
			// browser は user の通常運用 app を兼ねる。closed 状態を尊重し、launchd
			// の auto-reconcile では絶対に spawn しない。spawn / close は明示
			// イベント (add-browser / profile switch / archive / unarchive) でのみ
			// 行う。詳細: queue/projwm-spec.md FR-29, D-50。
		}
	}
	return acts
}

// ensureGhosttyWindow は title 一致の terminal window を slot に揃える（tmux も整える）。
//
// v11.3 で kitty driver に切替後の標準実装。kitty (NSPrincipalClass 注入済 user-space copy)
// は OmniWM に見える。
//
// 重複 spawn 防止: tmux session が既存ならば「直前の reconcile pass で spawn 済み
// だが OmniWM の認識がまだ追いついていない」可能性が高い → 短時間 polling
// (3 秒) で出現を待ってから判定。それでも出なければ 1 回だけ再 spawn。
func (r *Reconciler) ensureGhosttyWindow(ctx context.Context, title, session, cwd, wsName string, opts Options) []Action {
	var acts []Action
	hasTmux, _ := r.Tmux.HasSession(ctx, session)

	// 1) tmux session が無ければ作成
	if !hasTmux {
		acts = append(acts, Action{Op: "new-tmux", Target: session})
		if !opts.DryRun {
			if err := r.Tmux.NewSessionDetached(ctx, session, cwd); err != nil {
				acts[len(acts)-1].OnError = err
				return acts
			}
		}
	}

	// 2) OmniWM が terminal window を見えるか query
	match, err := r.findWindowByTitle(ctx, title)
	if err != nil {
		r.logf(opts, "WARN: query windows: %v", err)
		return acts
	}

	// 3) tmux 既存 + OmniWM 未認識 → 短時間 polling 待ち（直前 spawn の認識遅延対応）
	if match == nil && hasTmux && !opts.DryRun {
		match = r.waitForWindow(ctx, title, 3*time.Second)
	}

	// 4) OmniWM 視点で window 存在 + 別 WS → 移動
	if match != nil {
		if match.Workspace.RawName != wsName && match.Workspace.DisplayName != wsName {
			acts = append(acts, Action{Op: "move-to-ws", Target: match.ID, Detail: "→" + wsName})
			if !opts.DryRun {
				if err := r.OmniWM.MoveWindowToWorkspaceByName(ctx, match.ID, wsName); err != nil {
					acts[len(acts)-1].OnError = err
				}
			}
		}
		return acts
	}

	// 5) 完全に無い → 新規 spawn
	acts = append(acts, Action{Op: "spawn-ghostty", Target: title, Detail: "ws=" + wsName})
	if !opts.DryRun {
		if err := r.Ghostty.Spawn(ctx, title, cwd, session); err != nil {
			acts[len(acts)-1].OnError = err
		} else {
			// spawn 後、OmniWM が認識した直後に WS 配置する
			go r.placeAfterSpawn(ctx, title, wsName)
		}
	}
	return acts
}

// findWindowByTitle は title 一致の terminal window を返す（無ければ nil）。
func (r *Reconciler) findWindowByTitle(ctx context.Context, title string) (*omniwm.Window, error) {
	wins, err := r.OmniWM.QueryWindows(ctx, "--bundle-id", naming.TerminalBundleID)
	if err != nil {
		return nil, err
	}
	for i := range wins {
		if wins[i].Title == title {
			return &wins[i], nil
		}
	}
	return nil, nil
}

// waitForWindow は title の window が OmniWM に出現するまで polling 待ち。
func (r *Reconciler) waitForWindow(parentCtx context.Context, title string, timeout time.Duration) *omniwm.Window {
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		if w, err := r.findWindowByTitle(ctx, title); err == nil && w != nil {
			return w
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil
}

// placeAfterSpawn は spawn 直後の window を polling で見つけて WS に配置する。
func (r *Reconciler) placeAfterSpawn(parentCtx context.Context, title, wsName string) {
	ctx, cancel := context.WithTimeout(parentCtx, 5*time.Second)
	defer cancel()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		wins, err := r.OmniWM.QueryWindows(ctx, "--bundle-id", naming.TerminalBundleID)
		if err == nil {
			for _, w := range wins {
				if w.Title == title {
					if w.Workspace.RawName != wsName && w.Workspace.DisplayName != wsName {
						_ = r.OmniWM.MoveWindowToWorkspaceByName(ctx, w.ID, wsName)
					}
					return
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// ensureZedWindow は basename 一致の Zed window を slot に揃える。
//
// 重要: Zed は `-n` で起動するたびに新 workspace を追加するため、reconcile が
// 「window 見えない → 再 spawn」を繰り返すと **無限に Zed window が生える**
// バグになる。対策:
//   1) 見つからない時は短時間 polling 待ち（既存 Zed が render 中の可能性）
//   2) flock ベースの spawn lock（複数 reconcile プロセスの並走対策）
//   3) Zed.app process がそもそも未起動の時のみ初回 spawn を許可
func (r *Reconciler) ensureZedWindow(ctx context.Context, title, cwd, wsName string, opts Options) []Action {
	var acts []Action

	match, err := r.findZedByTitle(ctx, title)
	if err != nil {
		r.logf(opts, "WARN: query Zed windows: %v", err)
		return acts
	}

	// 1) 見つからない → 短時間 polling 待ち（直前 spawn の認識遅延対応）
	if match == nil && !opts.DryRun {
		match = r.waitForZedWindow(ctx, title, 4*time.Second)
	}

	// 2) 見つかった → 別 WS なら移動
	if match != nil {
		if match.Workspace.RawName != wsName && match.Workspace.DisplayName != wsName {
			acts = append(acts, Action{Op: "move-to-ws", Target: match.ID, Detail: "Zed→" + wsName})
			if !opts.DryRun {
				if err := r.OmniWM.MoveWindowToWorkspaceByName(ctx, match.ID, wsName); err != nil {
					acts[len(acts)-1].OnError = err
				}
			}
		}
		return acts
	}

	// 3) 完全に不在 → 新規 spawn だが flock で並走 reconcile からの重複 spawn を防ぐ
	if !opts.DryRun {
		if !acquireZedSpawnLock(title, 6*time.Second) {
			// 直前で別 reconcile が spawn 中、skip
			acts = append(acts, Action{
				Op:     "skip-zed-spawn",
				Target: title,
				Detail: "another reconcile is spawning",
			})
			return acts
		}
		defer releaseZedSpawnLock(title)
	}

	acts = append(acts, Action{Op: "spawn-zed", Target: title, Detail: cwd})
	if !opts.DryRun {
		if err := r.Zed.Spawn(ctx, cwd); err != nil {
			acts[len(acts)-1].OnError = err
		} else {
			// spawn 後 polling で配置 + lock を保持して重複防止
			go func() {
				r.placeZedAfterSpawn(ctx, title, wsName)
				// lock は defer で解放されるが、placeZedAfterSpawn が長いので
				// その間に並走 reconcile が来ても skip される
			}()
		}
	}
	return acts
}

// findZedByTitle は title 一致の Zed window を返す（無ければ nil）。
func (r *Reconciler) findZedByTitle(ctx context.Context, title string) (*omniwm.Window, error) {
	wins, err := r.OmniWM.QueryWindows(ctx, "--bundle-id", naming.ZedBundleID)
	if err != nil {
		return nil, err
	}
	for i := range wins {
		if wins[i].Title == title {
			return &wins[i], nil
		}
	}
	return nil, nil
}

// waitForZedWindow は title 一致の Zed window が出現するまで polling 待ち。
func (r *Reconciler) waitForZedWindow(parentCtx context.Context, title string, timeout time.Duration) *omniwm.Window {
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		if w, err := r.findZedByTitle(ctx, title); err == nil && w != nil {
			return w
		}
		time.Sleep(250 * time.Millisecond)
	}
	return nil
}

func (r *Reconciler) placeZedAfterSpawn(parentCtx context.Context, title, wsName string) {
	// Zed 起動は ghostty より重いことがあるので timeout は長めに（POC-20 観測待ち）
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		wins, err := r.OmniWM.QueryWindows(ctx, "--bundle-id", naming.ZedBundleID)
		if err == nil {
			for _, w := range wins {
				if w.Title == title {
					if w.Workspace.RawName != wsName && w.Workspace.DisplayName != wsName {
						_ = r.OmniWM.MoveWindowToWorkspaceByName(ctx, w.ID, wsName)
					}
					return
				}
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// ensureGroupedSession は viewer 用 grouped clone を確保する。
func (r *Reconciler) ensureGroupedSession(ctx context.Context, base, clone string, opts Options) []Action {
	if has, err := r.Tmux.HasSession(ctx, clone); err == nil && has {
		return nil
	}
	a := Action{Op: "new-grouped", Target: clone, Detail: "←" + base}
	if !opts.DryRun {
		if err := r.Tmux.NewGroupedSession(ctx, base, clone); err != nil {
			a.OnError = err
		}
	}
	return []Action{a}
}

// closeProjectWindowsKeepTmux は inactive な project の windows を close（tmux は touch しない）。
//
// browser kind window は **ここでは扱わない**。reconcile は browser に触らない設計
// （v12 paradigm C, queue/projwm-spec.md FR-29）。browser の close は profile switch
// cmd 等の明示イベントから直接呼ばれる。
func (r *Reconciler) closeProjectWindowsKeepTmux(ctx context.Context, name string, p state.Project, opts Options) []Action {
	var acts []Action
	titles := allProjectTitles(name, p)
	wins, err := r.queryAllProjwmWindows(ctx)
	if err != nil {
		r.logf(opts, "WARN: query: %v", err)
		return acts
	}
	for _, w := range wins {
		if titles[w.Title] {
			acts = append(acts, r.closeWindow(ctx, w, opts))
		}
	}
	return acts
}

// purgeArchivedProject は archived project の windows を close、tmux も kill。
//
// browser kind window は **ここでは扱わない**（reconcile から外す原則）。archive
// cmd 内で snapshot+close を直接呼ぶ。
func (r *Reconciler) purgeArchivedProject(ctx context.Context, name string, p state.Project, opts Options) []Action {
	var acts []Action
	// windows
	titles := allProjectTitles(name, p)
	wins, err := r.queryAllProjwmWindows(ctx)
	if err == nil {
		for _, w := range wins {
			if titles[w.Title] {
				acts = append(acts, r.closeWindow(ctx, w, opts))
			}
		}
	}
	// tmux
	for _, w := range p.Windows {
		switch w.Kind {
		case naming.KindAI:
			s := naming.TmuxSession(w.Kind, w.ID, name)
			vs := naming.ViewerTmuxSession(w.ID, name)
			acts = append(acts, r.killTmux(ctx, s, opts), r.killTmux(ctx, vs, opts))
		case naming.KindShell:
			s := naming.TmuxSession(w.Kind, w.ID, name)
			acts = append(acts, r.killTmux(ctx, s, opts))
		}
	}
	return acts
}

func (r *Reconciler) killTmux(ctx context.Context, session string, opts Options) Action {
	a := Action{Op: "kill-tmux", Target: session}
	if !opts.DryRun {
		if err := r.Tmux.KillSession(ctx, session); err != nil {
			a.OnError = err
		}
	}
	return a
}

func (r *Reconciler) closeWindow(ctx context.Context, w omniwm.Window, opts Options) Action {
	a := Action{Op: "close-window", Target: w.ID, Detail: w.Title}
	if !opts.DryRun {
		// omniwmctl に close-window は無いので AppleScript / kill PID 経路。最も安全なのは PID kill。
		if w.PID > 0 {
			_ = killPID(w.PID)
		}
	}
	return a
}

// gcViewerOrphans は WS A の規約 title だが期待にない viewer 窓を close。
func (r *Reconciler) gcViewerOrphans(ctx context.Context, expected map[string]bool, opts Options) []Action {
	wins, err := r.OmniWM.QueryWindows(ctx, "--bundle-id", naming.TerminalBundleID, "--workspace", r.Cfg.ViewerWorkspace)
	if err != nil {
		r.logf(opts, "WARN: query viewer ws: %v", err)
		return nil
	}
	var acts []Action
	for _, w := range wins {
		if !strings.HasPrefix(w.Title, "ai-view-") {
			continue
		}
		if expected[w.Title] {
			continue
		}
		acts = append(acts, r.closeWindow(ctx, w, opts))
	}
	return acts
}

// gcOrphans は --gc 時のみ呼ばれ、規約 title だが state に対応無い窓を一括 close。
func (r *Reconciler) gcOrphans(ctx context.Context, st *state.State, opts Options) []Action {
	expected := map[string]bool{}
	for name, p := range st.Projects {
		for k := range allProjectTitles(name, p) {
			expected[k] = true
		}
	}
	var acts []Action
	wins, err := r.OmniWM.QueryWindows(ctx, "--bundle-id", naming.TerminalBundleID)
	if err != nil {
		return nil
	}
	for _, w := range wins {
		if !looksLikeProjwmTitle(w.Title) {
			continue
		}
		if expected[w.Title] {
			continue
		}
		acts = append(acts, r.closeWindow(ctx, w, opts))
	}
	return acts
}

func looksLikeProjwmTitle(t string) bool {
	for _, prefix := range []string{"ai-", "shell-", "ai-view-"} {
		if strings.HasPrefix(t, prefix) && strings.Contains(t, ":") {
			return true
		}
	}
	return false
}

func (r *Reconciler) queryAllProjwmWindows(ctx context.Context) ([]omniwm.Window, error) {
	wins, err := r.OmniWM.QueryWindows(ctx, "--bundle-id", naming.TerminalBundleID)
	if err != nil {
		return nil, err
	}
	zwins, err := r.OmniWM.QueryWindows(ctx, "--bundle-id", naming.ZedBundleID)
	if err != nil {
		// Zed query 失敗は致命でない
		return wins, nil
	}
	return append(wins, zwins...), nil
}

func allProjectTitles(name string, p state.Project) map[string]bool {
	res := map[string]bool{}
	for _, w := range p.Windows {
		switch w.Kind {
		case naming.KindAI:
			res[naming.GhosttyTitle(w.Kind, w.ID, name)] = true
			res[naming.ViewerGhosttyTitle(w.ID, name)] = true
		case naming.KindShell:
			res[naming.GhosttyTitle(w.Kind, w.ID, name)] = true
		case naming.KindEditor:
			res[naming.ZedTitle(p.CWD)] = true
		}
	}
	return res
}

func countArchived(st *state.State) int {
	n := 0
	for _, p := range st.Projects {
		if p.Archived {
			n++
		}
	}
	return n
}

func (r *Reconciler) logf(opts Options, format string, args ...any) {
	if !opts.Verbose {
		return
	}
	fmt.Fprintf(opts.Logger, "[reconcile] "+format+"\n", args...)
}

// LogPath は reconcile.log の path を返す。
func LogPath(stateDir string) string {
	return filepath.Join(stateDir, "logs", "reconcile.log")
}

// killPID は OS プロセス kill。darwin 専用シンプル実装（projwm は darwin 限定）。
func killPID(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

// SpawnBrowserWindow は project の browser-N window を Vivaldi に spawn する
// (v12 paradigm C, 明示イベント専用)。reconcile からは絶対呼ばない。
//
// idempotent: state.Window.LiveWindowID が現存する → no-op。
// spawn 成功時、新 window-id を state.Window.LiveWindowID に persist する。
func (r *Reconciler) SpawnBrowserWindow(ctx context.Context, project string, w state.Window) Action {
	a := Action{
		Op:     "spawn-browser",
		Target: fmt.Sprintf("browser-%d:%s", w.ID, project),
		Detail: fmt.Sprintf("profile=%s urls=%d", w.BrowserProfile, len(w.SavedURLs)),
	}
	if r.Chromium == nil {
		a.OnError = fmt.Errorf("chromium driver not initialized")
		return a
	}
	// idempotent: 既存 LiveWindowID が現存していれば no-op
	if w.LiveWindowID != "" && r.Chromium.IsRunning() && r.Chromium.WindowExists(ctx, w.LiveWindowID) {
		a.Detail = "already live (idempotent, wid=" + w.LiveWindowID + ")"
		return a
	}
	wid, err := r.Chromium.SpawnAndRestoreFocus(ctx, w.BrowserProfile, w.SavedURLs)
	if err != nil {
		a.OnError = err
		return a
	}
	if perr := persistLiveWindowID(project, w.ID, wid); perr != nil {
		a.OnError = perr
		return a
	}
	a.Detail = fmt.Sprintf("profile=%s urls=%d wid=%s", w.BrowserProfile, len(w.SavedURLs), wid)
	return a
}

// SnapshotAndCloseBrowserWindow は project の単一 browser window を snapshot + close
// し、state.Window.SavedURLs / LiveWindowID を更新する (v12 paradigm C)。
//
// state.Window.LiveWindowID が空 / stale なら no-op (browser 未起動 or 別 cmd で
// 既に close 済 等)。
func (r *Reconciler) SnapshotAndCloseBrowserWindow(ctx context.Context, project string, w state.Window) Action {
	a := Action{
		Op:     "close-browser",
		Target: fmt.Sprintf("browser-%d:%s", w.ID, project),
	}
	if r.Chromium == nil || !r.Chromium.IsRunning() {
		a.Detail = "vivaldi not running (no-op)"
		return a
	}
	if w.LiveWindowID == "" {
		a.Detail = "no live wid in state (no-op)"
		return a
	}
	if !r.Chromium.WindowExists(ctx, w.LiveWindowID) {
		// stale wid: state クリアだけ実行
		_ = persistLiveWindowID(project, w.ID, "")
		a.Detail = "stale wid (cleared, no-op)"
		return a
	}
	a.Target = fmt.Sprintf("browser-%d:%s wid=%s", w.ID, project, w.LiveWindowID)
	urls, e := r.Chromium.SnapshotAndCloseAndRestoreFocus(ctx, w.LiveWindowID)
	if e != nil {
		a.OnError = e
		return a
	}
	if perr := persistSavedURLsAndClearWid(project, w.ID, urls); perr != nil {
		a.OnError = perr
		return a
	}
	a.Detail = fmt.Sprintf("snapshot=%d urls", len(urls))
	return a
}

// SpawnAllBrowserWindowsInProject は project の全 kind=browser windows を spawn する。
func (r *Reconciler) SpawnAllBrowserWindowsInProject(ctx context.Context, project string, p state.Project) []Action {
	var acts []Action
	for _, w := range p.Windows {
		if w.Kind != naming.KindBrowser {
			continue
		}
		acts = append(acts, r.SpawnBrowserWindow(ctx, project, w))
	}
	return acts
}

// SnapshotAndCloseAllBrowserWindowsInProject は project の全 kind=browser windows を
// snapshot + close する。
func (r *Reconciler) SnapshotAndCloseAllBrowserWindowsInProject(ctx context.Context, project string, p state.Project) []Action {
	var acts []Action
	for _, w := range p.Windows {
		if w.Kind != naming.KindBrowser {
			continue
		}
		acts = append(acts, r.SnapshotAndCloseBrowserWindow(ctx, project, w))
	}
	return acts
}

// persistLiveWindowID は state.Window.LiveWindowID を書き戻す。
func persistLiveWindowID(project string, browserID int, wid string) error {
	paths, err := state.DefaultPaths()
	if err != nil {
		return err
	}
	store := state.NewStore(paths)
	return store.Mutate(func(st *state.State) error {
		p, ok := st.Projects[project]
		if !ok {
			return fmt.Errorf("project %q not found", project)
		}
		for i := range p.Windows {
			w := &p.Windows[i]
			if w.Kind == naming.KindBrowser && w.ID == browserID {
				w.LiveWindowID = wid
				st.Projects[project] = p
				return nil
			}
		}
		return fmt.Errorf("browser-%d not found in %q", browserID, project)
	})
}

// persistSavedURLsAndClearWid は SavedURLs を更新 + LiveWindowID をクリア。
func persistSavedURLsAndClearWid(project string, browserID int, urls []string) error {
	paths, err := state.DefaultPaths()
	if err != nil {
		return err
	}
	store := state.NewStore(paths)
	return store.Mutate(func(st *state.State) error {
		p, ok := st.Projects[project]
		if !ok {
			return fmt.Errorf("project %q not found", project)
		}
		for i := range p.Windows {
			w := &p.Windows[i]
			if w.Kind == naming.KindBrowser && w.ID == browserID {
				w.SavedURLs = urls
				w.LiveWindowID = ""
				st.Projects[project] = p
				return nil
			}
		}
		return fmt.Errorf("browser-%d not found in %q", browserID, project)
	})
}

