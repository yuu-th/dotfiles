// Package tui は bubbletea ベースの projwm cockpit TUI（projwm-design.md §8.2）。
//
// 機能:
//   - active profile の全 slot を windows 詳細付きで表示（ai-N/shell-N/editor 別、
//     tmux/window/AI status 各色付きインジケータ）
//   - empty slots の明示
//   - 他 profile・parked・archived セクション
//   - fzf 風 incremental filter
//   - 操作: ↑↓ Tab Enter / n d a u r s ? esc
//   - fsnotify による state.json リアクティブ更新
package tui

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
	"github.com/yuu-th/projwm/internal/config"
	"github.com/yuu-th/projwm/internal/naming"
	"github.com/yuu-th/projwm/internal/omniwm"
	"github.com/yuu-th/projwm/internal/state"
	"github.com/yuu-th/projwm/internal/tmuxwrap"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700"))
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7AA2F7"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370"))
	hlStyle       = lipgloss.NewStyle().Background(lipgloss.Color("#3B4261")).Foreground(lipgloss.Color("#FFFFFF"))
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#F7768E"))
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ECE6A"))
	warnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0AF68"))
	slotStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#BB9AF7"))
	emptySlotStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B4261"))
	infoStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#7DCFFF")).Italic(true)
	keyStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF9E64"))
)

type entryKind int

const (
	entrySlot entryKind = iota
	entryEmptySlot
	entryProfile
	entryParked
	entryArchived
	entryAction // n / r 等のアクションエントリ（filter から実行用）
)

type entry struct {
	kind     entryKind
	slot     string
	project  string
	profile  string
	desc     string
	dim      bool
	subLines []string // windows 詳細の追加行
}

// Model は bubbletea の状態。
type Model struct {
	store     *state.Store
	cfg       config.Config
	st        *state.State
	cli       *omniwm.Client
	tmux      *tmuxwrap.Client
	tmuxSet   map[string]bool          // 現在ある tmux session 名集合
	winSet    map[string]bool          // OmniWM 視点で window 存在する title 集合
	zedSet    map[string]bool          // Zed の title 集合
	entries   []entry
	filter    string
	cursor    int
	width     int
	height    int
	mode      mode
	prompt    promptState
	lastErr   string
	lastInfo  string
	stopped   bool
	dirty     bool // state 再読み込み要
}

type mode int

const (
	modeNormal mode = iota
	modePrompt // 文字入力中（new project, archive 等）
	modeConfirm
)

type promptState struct {
	purpose  string // 例: "new-project", "assign-slot", "unarchive-to-active", "move-project", "add-ai-<proj>", ...
	question string
	value    string
	aux1     string // purpose に応じた追加引数 (例: assign 対象 project / move 対象 project)
	hidden   bool
}

func NewModel(store *state.Store, cfg config.Config) (*Model, error) {
	m := &Model{
		store:   store,
		cfg:     cfg,
		cli:     omniwm.New(nil),
		tmux:    tmuxwrap.New(nil),
		tmuxSet: map[string]bool{},
		winSet:  map[string]bool{},
		zedSet:  map[string]bool{},
	}
	m.refresh()
	return m, nil
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		watchStateFile(),
		periodicProbe(),
	)
}

// fsnotifyMsg / probeMsg はリアクティブ更新の signal。
type fsnotifyMsg struct{}
type probeMsg struct{}
type infoMsg string

// watchStateFile は state.json の変化を fsnotify で監視。
func watchStateFile() tea.Cmd {
	return func() tea.Msg {
		w, err := fsnotify.NewWatcher()
		if err != nil {
			return nil
		}
		paths, err := state.DefaultPaths()
		if err != nil {
			return nil
		}
		_ = w.Add(paths.Dir)
		select {
		case <-w.Events:
			return fsnotifyMsg{}
		case <-time.After(60 * time.Second):
			return fsnotifyMsg{}
		}
	}
}

// periodicProbe は 2 秒ごとに tmux/window 状態を更新。
func periodicProbe() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return probeMsg{} })
}

// refresh は state を再読み込みし entries を再構築する。
func (m *Model) refresh() {
	st, err := m.store.Load()
	if err != nil {
		m.lastErr = err.Error()
		return
	}
	m.st = st

	// tmux session list（毎回 query、軽い）
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	if sessions, err := m.tmux.ListSessions(ctx); err == nil {
		newSet := map[string]bool{}
		for _, s := range sessions {
			newSet[s] = true
		}
		m.tmuxSet = newSet
	}

	// OmniWM windows（terminal + Zed）
	if wins, err := m.cli.QueryWindows(ctx, "--bundle-id", naming.TerminalBundleID); err == nil {
		newSet := map[string]bool{}
		for _, w := range wins {
			newSet[w.Title] = true
		}
		m.winSet = newSet
	}
	if zwins, err := m.cli.QueryWindows(ctx, "--bundle-id", naming.ZedBundleID); err == nil {
		newSet := map[string]bool{}
		for _, w := range zwins {
			newSet[w.Title] = true
		}
		m.zedSet = newSet
	}

	m.rebuildEntries()
}

// rebuildEntries は state + 観測状態から entries を組む。
func (m *Model) rebuildEntries() {
	st := m.st
	var es []entry

	// === active profile の slot ===
	if st.ActiveProfile != "" {
		prof := st.Profiles[st.ActiveProfile]
		filledSlots := map[string]bool{}
		// 順序固定: config の slot_names 順
		for _, slot := range m.cfg.SlotNames {
			projName, ok := prof.Assignments[slot]
			if !ok {
				continue
			}
			filledSlots[slot] = true
			p := st.Projects[projName]
			subLines := m.windowSubLines(projName, p)
			es = append(es, entry{
				kind:     entrySlot,
				slot:     slot,
				project:  projName,
				desc:     filepath.Base(p.CWD),
				subLines: subLines,
			})
		}
		// 空 slot
		for _, slot := range m.cfg.SlotNames {
			if !filledSlots[slot] {
				es = append(es, entry{kind: entryEmptySlot, slot: slot, dim: true})
			}
		}
	}

	// === 他 profile ===
	pnames := make([]string, 0, len(st.Profiles))
	for n := range st.Profiles {
		if n != st.ActiveProfile {
			pnames = append(pnames, n)
		}
	}
	sort.Strings(pnames)
	for _, n := range pnames {
		p := st.Profiles[n]
		es = append(es, entry{
			kind:    entryProfile,
			profile: n,
			desc:    fmt.Sprintf("%d assignments — %s", len(p.Assignments), p.Description),
		})
	}

	// === parked ===
	parkedNames := []string{}
	for n := range st.Projects {
		if state.IsParked(st, n) {
			parkedNames = append(parkedNames, n)
		}
	}
	sort.Strings(parkedNames)
	for _, n := range parkedNames {
		p := st.Projects[n]
		es = append(es, entry{kind: entryParked, project: n, desc: filepath.Base(p.CWD)})
	}

	// === archived ===
	archivedNames := []string{}
	for n, p := range st.Projects {
		if p.Archived {
			archivedNames = append(archivedNames, n)
		}
	}
	sort.Strings(archivedNames)
	for _, n := range archivedNames {
		p := st.Projects[n]
		es = append(es, entry{kind: entryArchived, project: n, desc: filepath.Base(p.CWD)})
	}

	m.entries = es
	if m.cursor >= len(m.filtered()) {
		m.cursor = 0
	}
}

// windowSubLines は project の windows[] を 1 行ずつ整形する。
//
//	  ai-1     claude    tmux●  win●
//	  shell-1            tmux●  win●
//	  editor             —      win✓
func (m *Model) windowSubLines(projName string, p state.Project) []string {
	var lines []string
	wins := state.SortedWindows(p)
	for _, w := range wins {
		var line strings.Builder
		switch w.Kind {
		case naming.KindAI:
			line.WriteString(fmt.Sprintf("  ai-%d", w.ID))
			line.WriteString(strings.Repeat(" ", max(0, 8-len(fmt.Sprintf("ai-%d", w.ID)))))
			line.WriteString(string(w.AI))
			line.WriteString(strings.Repeat(" ", max(0, 10-len(string(w.AI)))))
			line.WriteString(m.statusFor(naming.TmuxSession(w.Kind, w.ID, projName), naming.GhosttyTitle(w.Kind, w.ID, projName), false))
		case naming.KindShell:
			line.WriteString(fmt.Sprintf("  shell-%d", w.ID))
			line.WriteString(strings.Repeat(" ", max(0, 8-len(fmt.Sprintf("shell-%d", w.ID)))))
			line.WriteString("          ")
			line.WriteString(m.statusFor(naming.TmuxSession(w.Kind, w.ID, projName), naming.GhosttyTitle(w.Kind, w.ID, projName), false))
		case naming.KindEditor:
			zedTitle := naming.ZedTitle(p.CWD)
			line.WriteString("  editor  ")
			line.WriteString("            ")
			line.WriteString(m.statusFor("", zedTitle, true))
		case naming.KindBrowser:
			label := fmt.Sprintf("browser-%d", w.ID)
			line.WriteString("  ")
			line.WriteString(label)
			line.WriteString(strings.Repeat(" ", max(0, 8-len(label))))
			profile := w.BrowserProfile
			if profile == "" {
				profile = "(no profile)"
			}
			line.WriteString(profile)
		}
		lines = append(lines, line.String())
	}
	if len(wins) == 0 {
		lines = append(lines, dimStyle.Render("  (no windows)"))
	}
	return lines
}

// statusFor は tmux●/win● 等のステータスインジケータを色付き文字列で返す。
func (m *Model) statusFor(tmuxSession, windowTitle string, isEditor bool) string {
	var b strings.Builder
	if !isEditor {
		ok := m.tmuxSet[tmuxSession]
		if ok {
			b.WriteString(okStyle.Render("tmux● "))
		} else {
			b.WriteString(errStyle.Render("tmux○ "))
		}
	} else {
		b.WriteString(dimStyle.Render("—     "))
	}
	winSet := m.winSet
	if isEditor {
		winSet = m.zedSet
	}
	if winSet[windowTitle] {
		b.WriteString(okStyle.Render("win●"))
	} else {
		b.WriteString(errStyle.Render("win○"))
	}
	return b.String()
}

// filtered は filter による絞り込み結果を返す。
func (m *Model) filtered() []entry {
	if m.filter == "" {
		return m.entries
	}
	q := strings.ToLower(m.filter)
	var out []entry
	for _, e := range m.entries {
		text := strings.ToLower(e.slot + " " + e.project + " " + e.profile + " " + e.desc)
		if strings.Contains(text, q) {
			out = append(out, e)
		}
	}
	return out
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case fsnotifyMsg:
		m.refresh()
		return m, watchStateFile()
	case probeMsg:
		m.refresh()
		return m, periodicProbe()
	case infoMsg:
		m.lastInfo = string(msg)
	case tea.KeyMsg:
		if m.mode == modePrompt {
			return m.updatePrompt(msg)
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

func (m *Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ents := m.filtered()
	var cur entry
	if m.cursor < len(ents) {
		cur = ents[m.cursor]
	}
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit
	case "q":
		if m.filter != "" {
			m.filter += msg.String()
			m.cursor = 0
			return m, nil
		}
		return m, tea.Quit
	case "enter":
		if m.cursor < len(ents) {
			return m.activate(cur)
		}
	case "up", "ctrl+k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "ctrl+j":
		if m.cursor < len(ents)-1 {
			m.cursor++
		}
	case "tab":
		return m, m.cycleProfile()
	// ── 既存操作 ─────────────────────────────────────────────────────
	case "n":
		if m.filter == "" {
			return m.startPrompt("new-project", "new project name (cwd basename, ~/dev で探す):")
		}
	case "a":
		if m.filter == "" && cur.project != "" && cur.kind != entryArchived {
			return m, m.archiveCurrent(cur.project)
		}
	case "u":
		if m.filter == "" && cur.kind == entryArchived {
			return m.startPrompt("unarchive-to-active",
				fmt.Sprintf("unarchive %s — slot to assign (active profile %q, blank=parked):", cur.project, m.st.ActiveProfile))
		}
	case "d":
		if m.filter == "" && cur.kind == entrySlot {
			return m, m.unassignCurrent(cur.slot)
		}
	case "r":
		if m.filter == "" {
			return m, m.runReconcileCmd()
		}
	// ── 新規: assign / move / window 操作 ────────────────────────────
	case "m":
		if m.filter == "" && (cur.kind == entrySlot || cur.kind == entryParked) {
			return m.startPrompt("move-project",
				fmt.Sprintf("move %s — target slot (Q,W,E,R,T,Y,U,I,O,P):", cur.project))
		}
	case "A":
		if m.filter == "" && cur.project != "" && cur.kind != entryArchived {
			return m.startPrompt("add-ai-"+cur.project, "add-ai claude|copilot:")
		}
	case "S":
		if m.filter == "" && cur.project != "" && cur.kind != entryArchived {
			return m, m.execProjwm("add-shell", "--project="+cur.project)
		}
	case "E":
		if m.filter == "" && cur.project != "" && cur.kind != entryArchived {
			return m, m.execProjwm("add-editor", "--project="+cur.project)
		}
	case "B":
		if m.filter == "" && cur.project != "" && cur.kind != entryArchived {
			return m.startPrompt("add-browser-"+cur.project, "add-browser <profile> [url1 url2 ...]:")
		}
	case "X":
		if m.filter == "" && cur.project != "" {
			return m.startPrompt("remove-window-"+cur.project, "remove window (e.g. ai-2 / shell-1 / editor-1 / browser-1):")
		}
	// ── profile management ──────────────────────────────────────────
	case "P":
		if m.filter == "" {
			return m.startPrompt("profile-create", "new profile name:")
		}
	case "!":
		if m.filter == "" && cur.kind == entryProfile {
			return m, m.execProjwm("profile", "delete", cur.profile)
		}
	case "ctrl+r":
		if m.filter == "" && cur.kind == entryProfile {
			return m.startPrompt("profile-rename-"+cur.profile, "rename profile "+cur.profile+" → ?:")
		}
	case "?":
		if m.filter == "" {
			m.lastInfo = "enter=activate  n=new-project  a=archive  u=unarchive  d=unassign  m=move  r=reconcile  A/S/E/B=add ai/shell/editor/browser  X=remove-window  P=new-profile  !=delete-profile  ctrl+r=rename-profile  tab=cycle-profile  esc=quit"
			return m, nil
		}
		fallthrough
	case "backspace":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
			m.cursor = 0
		}
	default:
		if len(msg.String()) == 1 {
			m.filter += msg.String()
			m.cursor = 0
		}
	}
	return m, nil
}

func (m *Model) startPrompt(purpose, question string) (tea.Model, tea.Cmd) {
	m.mode = modePrompt
	m.prompt = promptState{purpose: purpose, question: question}
	return m, nil
}

func (m *Model) startPromptWithAux(purpose, question, aux1 string) (tea.Model, tea.Cmd) {
	m.mode = modePrompt
	m.prompt = promptState{purpose: purpose, question: question, aux1: aux1}
	return m, nil
}

func (m *Model) updatePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.mode = modeNormal
		m.prompt = promptState{}
	case "enter":
		v := strings.TrimSpace(m.prompt.value)
		purpose := m.prompt.purpose
		aux1 := m.prompt.aux1
		m.mode = modeNormal
		m.prompt = promptState{}
		switch {
		case purpose == "new-project":
			return m, m.createProject(v)
		case purpose == "assign-slot":
			// value = project name, aux1 = slot
			if v == "" {
				return m, infoCmd("assign cancelled (empty project)")
			}
			return m, m.execProjwm("profile", "assign", aux1, v)
		case purpose == "assign-parked":
			// value = slot, aux1 = project
			if v == "" {
				return m, infoCmd("assign cancelled (empty slot)")
			}
			return m, m.execProjwm("profile", "assign", v, aux1)
		case purpose == "unarchive-to-active":
			// value = slot ("" なら parked), aux1 = project
			if v == "" {
				return m, m.execProjwm("unarchive", aux1)
			}
			return m, m.execProjwm("unarchive", aux1, "--profile="+m.st.ActiveProfile, "--slot="+v)
		case purpose == "move-project":
			// value = target slot, aux1 = project
			if v == "" {
				return m, infoCmd("move cancelled")
			}
			return m, tea.Sequence(
				m.execProjwm("profile", "unassign", aux1),
				m.execProjwm("profile", "assign", v, aux1),
			)
		case strings.HasPrefix(purpose, "add-ai-"):
			project := strings.TrimPrefix(purpose, "add-ai-")
			ai := v
			if ai == "" {
				ai = "claude"
			}
			return m, m.execProjwm("add-ai", "--project="+project, "--ai="+ai)
		case strings.HasPrefix(purpose, "add-browser-"):
			project := strings.TrimPrefix(purpose, "add-browser-")
			fields := strings.Fields(v)
			if len(fields) == 0 {
				return m, infoCmd("add-browser cancelled (need profile)")
			}
			args := []string{"add-browser", "--project=" + project, "--profile=" + fields[0]}
			for _, u := range fields[1:] {
				args = append(args, "--url="+u)
			}
			return m, m.execProjwm(args...)
		case strings.HasPrefix(purpose, "remove-window-"):
			project := strings.TrimPrefix(purpose, "remove-window-")
			if v == "" {
				return m, infoCmd("remove cancelled")
			}
			return m, m.execProjwm("remove", "--project="+project, "--window="+v)
		case purpose == "profile-create":
			if v == "" {
				return m, infoCmd("profile-create cancelled")
			}
			return m, m.execProjwm("profile", "create", v)
		case strings.HasPrefix(purpose, "profile-rename-"):
			old := strings.TrimPrefix(purpose, "profile-rename-")
			if v == "" {
				return m, infoCmd("rename cancelled")
			}
			return m, m.execProjwm("profile", "rename", old, v)
		}
	case "backspace":
		if len(m.prompt.value) > 0 {
			m.prompt.value = m.prompt.value[:len(m.prompt.value)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.prompt.value += msg.String()
		}
	}
	return m, nil
}

// execProjwm は外部 projwm cmd を spawn して結果を info msg として返す tea.Cmd。
// paradigm C ライフサイクル hook (cmd 層) を一元的に呼ぶ。
func (m *Model) execProjwm(args ...string) tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("projwm", args...).CombinedOutput()
		s := strings.TrimSpace(string(out))
		if err != nil {
			return infoMsg("ERROR " + strings.Join(args, " ") + ": " + s)
		}
		// 短く表示するため改行を ` | ` に圧縮
		s = strings.ReplaceAll(s, "\n", " | ")
		if len(s) > 200 {
			s = s[:200] + "…"
		}
		// state 再読み込みのため refresh を呼ぶ（次回 fsnotify でも反映されるが即時のため）
		m.refresh()
		if s == "" {
			return infoMsg(strings.Join(args, " ") + ": ok")
		}
		return infoMsg(strings.Join(args, " ") + ": " + s)
	}
}

// runReconcileCmd は projwm reconcile を外部 spawn (focus を奪わず情報だけ更新)。
func (m *Model) runReconcileCmd() tea.Cmd {
	return m.execProjwm("reconcile")
}

func infoCmd(s string) tea.Cmd {
	return func() tea.Msg { return infoMsg(s) }
}

// activate は entry の種類に応じた既定アクションを実行する (enter キー)。
//
// paradigm C 対応:
//   - entrySlot: workspace に jump (focus)
//   - entryEmptySlot: prompt で project 名 → assign（assign cmd は paradigm C を通す）
//   - entryProfile: 外部 cmd で profile switch（paradigm C 通す）
//   - entryParked: prompt で slot → assign
//   - entryArchived: prompt で slot → unarchive --profile=active --slot=X
func (m *Model) activate(e entry) (tea.Model, tea.Cmd) {
	switch e.kind {
	case entrySlot:
		ctx := context.Background()
		if err := m.cli.FocusWorkspaceByName(ctx, e.slot); err != nil {
			return m, infoCmd("ERROR: " + err.Error())
		}
		return m, infoCmd(fmt.Sprintf("→ %s [%s]", e.project, e.slot))
	case entryEmptySlot:
		return m.startPromptWithAux("assign-slot",
			fmt.Sprintf("assign project to slot [%s] — project name:", e.slot), e.slot)
	case entryProfile:
		return m, m.execProjwm("profile", "switch", e.profile)
	case entryParked:
		return m.startPromptWithAux("assign-parked",
			fmt.Sprintf("assign %s — slot (Q,W,E,R,T,Y,U,I,O,P):", e.project), e.project)
	case entryArchived:
		return m.startPromptWithAux("unarchive-to-active",
			fmt.Sprintf("unarchive %s — slot (active profile %q, blank=parked):", e.project, m.st.ActiveProfile), e.project)
	}
	return m, nil
}

// cycleProfile は次の profile に switch する。paradigm C 対応のため外部
// `projwm profile switch` 経由で呼ぶ（state mutate のみだと browser が close されない）。
func (m *Model) cycleProfile() tea.Cmd {
	names := []string{}
	for n := range m.st.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return infoCmd("(no profiles)")
	}
	next := names[0]
	for i, n := range names {
		if n == m.st.ActiveProfile {
			next = names[(i+1)%len(names)]
			break
		}
	}
	if next == m.st.ActiveProfile {
		return infoCmd("(only one profile)")
	}
	return m.execProjwm("profile", "switch", next)
}

func (m *Model) archiveCurrent(name string) tea.Cmd {
	return func() tea.Msg {
		// projwm cmd 経由で実行: archive_top.go の archive-project が
		// paradigm C 対応 (browser snapshot+close → state mutate → reconcile) を
		// 一貫して行う。TUI 内の重複実装を避ける。
		out, err := exec.Command("projwm", "archive-project", name).CombinedOutput()
		if err != nil {
			return infoMsg("ERROR archive: " + strings.TrimSpace(string(out)))
		}
		m.refresh()
		return infoMsg("archived: " + name)
	}
}

// (unarchiveCurrent removed; u 直接呼び出しは廃止、unarchive prompt 経由に統合)

func (m *Model) unassignCurrent(slot string) tea.Cmd {
	return func() tea.Msg {
		// projwm cmd 経由で実行: paradigm C の browser close + reconcile を一貫処理
		out, err := exec.Command("projwm", "profile", "unassign", slot).CombinedOutput()
		if err != nil {
			return infoMsg("ERROR unassign: " + strings.TrimSpace(string(out)))
		}
		m.refresh()
		return infoMsg("slot " + slot + " unassigned (parked)")
	}
}

func (m *Model) createProject(name string) tea.Cmd {
	return func() tea.Msg {
		if name == "" {
			return infoMsg("new-project: name required")
		}
		// 簡易: ~/dev/<name> をデフォ cwd
		home, _ := state.DefaultPaths()
		_ = home
		// 単に空 windows[] で project を登録するだけ。`projwm up` で windows を追加する
		err := m.store.Mutate(func(st *state.State) error {
			if _, exists := st.Projects[name]; exists {
				return fmt.Errorf("%s already exists", name)
			}
			st.Projects[name] = state.Project{
				CWD:      filepath.Join(homeDir(), "dev", name),
				Archived: false,
				Windows:  nil,
			}
			return nil
		})
		if err != nil {
			return infoMsg("ERROR: " + err.Error())
		}
		m.refresh()
		return infoMsg("project " + name + " created (cwd guess: ~/dev/" + name + "; run `projwm up --ai claude` to launch)")
	}
}

func homeDir() string {
	if h, err := state.DefaultPaths(); err == nil {
		// Use the dir parent up to home
		base := h.Dir
		// strip ".local/state/projwm"
		return filepath.Dir(filepath.Dir(filepath.Dir(base)))
	}
	return "/Users/" + "yuta"
}

func (m *Model) View() string {
	var b strings.Builder
	st := m.st

	// ── ヘッダ ────────────────────────────────────────────────
	b.WriteString(titleStyle.Render("projwm cockpit"))
	b.WriteString("\n")

	parked := 0
	archived := 0
	for n, p := range st.Projects {
		if p.Archived {
			archived++
		} else if state.IsParked(st, n) {
			parked++
		}
	}
	b.WriteString(fmt.Sprintf("profile: %s    parked: %d    archived: %d\n",
		dim(st.ActiveProfile, "—"), parked, archived))

	// ── filter / prompt 行 ──
	if m.mode == modePrompt {
		b.WriteString(infoStyle.Render(m.prompt.question))
		b.WriteString("\n")
		b.WriteString(headerStyle.Render("> " + m.prompt.value + "_"))
	} else {
		b.WriteString(headerStyle.Render("> " + m.filter + "_"))
	}
	b.WriteString("\n\n")

	ents := m.filtered()
	if len(ents) == 0 {
		b.WriteString(dimStyle.Render("  (no entries match filter, or no projects yet)\n"))
	}

	// ── セクションラベル付きで描画 ──
	currentSection := ""
	for i, e := range ents {
		section := sectionFor(e)
		if section != currentSection {
			if currentSection != "" {
				b.WriteString("\n")
			}
			b.WriteString(slotStyle.Render(section) + "\n")
			currentSection = section
		}
		var line string
		switch e.kind {
		case entrySlot:
			line = fmt.Sprintf("  [%s] %s    %s",
				slotStyle.Render(e.slot), e.project, dimStyle.Render(e.desc))
		case entryEmptySlot:
			line = fmt.Sprintf("  [%s] %s",
				emptySlotStyle.Render(e.slot), emptySlotStyle.Render("(empty)"))
		case entryProfile:
			line = fmt.Sprintf("  ● %s  %s", e.profile, dimStyle.Render(e.desc))
		case entryParked:
			line = fmt.Sprintf("  • %s  %s", e.project, dimStyle.Render(e.desc))
		case entryArchived:
			line = fmt.Sprintf("  ▼ %s  %s", e.project, dimStyle.Render(e.desc))
		}
		if i == m.cursor {
			line = hlStyle.Render("▶" + strings.TrimPrefix(line, " "))
		}
		b.WriteString(line + "\n")
		// sub lines (windows 詳細) — entrySlot のみ
		if e.kind == entrySlot {
			for _, sl := range e.subLines {
				b.WriteString("    " + sl + "\n")
			}
		}
	}

	// ── viewer (WS A) summary ──
	if st.ActiveProfile != "" {
		viewerCount := 0
		var viewerTitles []string
		for _, projName := range st.Profiles[st.ActiveProfile].Assignments {
			p := st.Projects[projName]
			if p.Archived {
				continue
			}
			for _, w := range p.Windows {
				if w.Kind == naming.KindAI {
					viewerCount++
					viewerTitles = append(viewerTitles, fmt.Sprintf("%s ai-%d", projName, w.ID))
				}
			}
		}
		b.WriteString("\n")
		b.WriteString(slotStyle.Render("viewer (WS A)") + "\n")
		if viewerCount == 0 {
			b.WriteString(dimStyle.Render("  (no AI windows in active profile)") + "\n")
		} else {
			b.WriteString(fmt.Sprintf("  [A] %d ai stream(s): %s\n", viewerCount, dimStyle.Render(strings.Join(viewerTitles, ", "))))
		}
	}

	// ── 操作ヘルプ ──
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(
		keyStyle.Render("↵") + " activate  " +
			keyStyle.Render("tab") + " cycle-prof  " +
			keyStyle.Render("n") + " new  " +
			keyStyle.Render("a") + " archive  " +
			keyStyle.Render("u") + " unarch  " +
			keyStyle.Render("d") + " unassign  " +
			keyStyle.Render("m") + " move  " +
			keyStyle.Render("r") + " reconcile",
	))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(
		keyStyle.Render("A/S/E/B") + " add ai/shell/editor/browser  " +
			keyStyle.Render("X") + " rm-window  " +
			keyStyle.Render("P") + " new-prof  " +
			keyStyle.Render("ctrl+r") + " ren-prof  " +
			keyStyle.Render("!") + " del-prof  " +
			keyStyle.Render("?") + " help  " +
			keyStyle.Render("esc") + " quit",
	))
	b.WriteString("\n")

	// ── status / error 行 ──
	if m.lastErr != "" {
		b.WriteString(errStyle.Render("error: " + m.lastErr + "\n"))
	}
	if m.lastInfo != "" {
		b.WriteString(infoStyle.Render(m.lastInfo) + "\n")
	}
	return b.String()
}

func sectionFor(e entry) string {
	switch e.kind {
	case entrySlot:
		return "active slots"
	case entryEmptySlot:
		return "active slots"
	case entryProfile:
		return "other profiles"
	case entryParked:
		return "parked (no slot, tmux alive)"
	case entryArchived:
		return "archived"
	case entryAction:
		return "actions"
	}
	return ""
}

func dim(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Run は TUI を実行する。
func Run(store *state.Store, cfg config.Config) error {
	m, err := NewModel(store, cfg)
	if err != nil {
		return err
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
