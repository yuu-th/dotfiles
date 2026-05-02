// Package tui は bubbletea ベースの projwm cockpit TUI（projwm-design.md §8.2）。
//
// 最小実装 (Phase 4): active profile の slot/project 一覧、profile 切替、jump、
// fzf 風フィルタ。アーカイブ/park セクション、新規作成プロンプト、リアルタイム
// fsnotify 更新は将来の拡張。
package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yuu-th/projwm/internal/config"
	"github.com/yuu-th/projwm/internal/omniwm"
	"github.com/yuu-th/projwm/internal/state"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700"))
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7AA2F7"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370"))
	hlStyle     = lipgloss.NewStyle().Background(lipgloss.Color("#3B4261")).Foreground(lipgloss.Color("#FFFFFF"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F7768E"))
)

type entryKind int

const (
	entryProfile entryKind = iota
	entrySlot
	entryParked
	entryArchived
)

type entry struct {
	kind    entryKind
	slot    string // "Q" / ""
	project string // project 名 / ""
	profile string // profile 名 / ""
	desc    string // 表示用補助
}

func (e entry) display() string {
	switch e.kind {
	case entryProfile:
		return fmt.Sprintf("[profile] %s  %s", e.profile, dimStyle.Render(e.desc))
	case entrySlot:
		return fmt.Sprintf("[%s] %s  %s", e.slot, e.project, dimStyle.Render(e.desc))
	case entryParked:
		return fmt.Sprintf("[parked] %s  %s", e.project, dimStyle.Render(e.desc))
	case entryArchived:
		return fmt.Sprintf("[archived] %s  %s", e.project, dimStyle.Render(e.desc))
	}
	return ""
}

// Model は bubbletea の状態。
type Model struct {
	store    *state.Store
	cfg      config.Config
	st       *state.State
	cli      *omniwm.Client
	entries  []entry
	filter   string
	cursor   int
	width    int
	height   int
	lastErr  string
	lastInfo string
}

func NewModel(store *state.Store, cfg config.Config) (*Model, error) {
	st, err := store.Load()
	if err != nil {
		return nil, err
	}
	m := &Model{
		store: store,
		cfg:   cfg,
		st:    st,
		cli:   omniwm.New(nil),
	}
	m.refresh()
	return m, nil
}

func (m *Model) Init() tea.Cmd { return nil }

// refresh は entries を state から再構築。
func (m *Model) refresh() {
	st, err := m.store.Load()
	if err != nil {
		m.lastErr = err.Error()
		return
	}
	m.st = st
	var es []entry

	// active profile の slots
	if st.ActiveProfile != "" {
		prof := st.Profiles[st.ActiveProfile]
		slots := make([]string, 0, len(prof.Assignments))
		for s := range prof.Assignments {
			slots = append(slots, s)
		}
		sort.Strings(slots)
		for _, s := range slots {
			name := prof.Assignments[s]
			p := st.Projects[name]
			desc := fmt.Sprintf("%d windows  %s", len(p.Windows), p.CWD)
			es = append(es, entry{kind: entrySlot, slot: s, project: name, desc: desc})
		}
	}

	// 他 profile
	pnames := make([]string, 0, len(st.Profiles))
	for n := range st.Profiles {
		if n != st.ActiveProfile {
			pnames = append(pnames, n)
		}
	}
	sort.Strings(pnames)
	for _, n := range pnames {
		p := st.Profiles[n]
		es = append(es, entry{kind: entryProfile, profile: n, desc: fmt.Sprintf("%d assignments", len(p.Assignments))})
	}

	// parked
	parkedNames := []string{}
	for n := range st.Projects {
		if state.IsParked(st, n) {
			parkedNames = append(parkedNames, n)
		}
	}
	sort.Strings(parkedNames)
	for _, n := range parkedNames {
		p := st.Projects[n]
		es = append(es, entry{kind: entryParked, project: n, desc: p.CWD})
	}

	// archived
	archivedNames := []string{}
	for n, p := range st.Projects {
		if p.Archived {
			archivedNames = append(archivedNames, n)
		}
	}
	sort.Strings(archivedNames)
	for _, n := range archivedNames {
		p := st.Projects[n]
		es = append(es, entry{kind: entryArchived, project: n, desc: p.CWD})
	}
	m.entries = es
}

// filtered は filter 文字列で絞り込んだ entries を返す。
func (m *Model) filtered() []entry {
	if m.filter == "" {
		return m.entries
	}
	q := strings.ToLower(m.filter)
	var out []entry
	for _, e := range m.entries {
		if strings.Contains(strings.ToLower(e.display()), q) {
			out = append(out, e)
		}
	}
	return out
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		ents := m.filtered()
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			if m.cursor < len(ents) {
				return m, m.activate(ents[m.cursor])
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
		case "r":
			if m.filter == "" {
				m.refresh()
				m.lastInfo = "refreshed"
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
	}
	return m, nil
}

func (m *Model) activate(e entry) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		switch e.kind {
		case entrySlot:
			if err := m.cli.FocusWorkspaceByName(ctx, e.slot); err != nil {
				m.lastErr = err.Error()
			}
		case entryProfile:
			err := m.store.Mutate(func(st *state.State) error {
				st.ActiveProfile = e.profile
				return nil
			})
			if err != nil {
				m.lastErr = err.Error()
			}
			m.refresh()
		}
		return nil
	}
}

func (m *Model) cycleProfile() tea.Cmd {
	return func() tea.Msg {
		names := []string{}
		for n := range m.st.Profiles {
			names = append(names, n)
		}
		sort.Strings(names)
		if len(names) == 0 {
			return nil
		}
		next := names[0]
		for i, n := range names {
			if n == m.st.ActiveProfile {
				next = names[(i+1)%len(names)]
				break
			}
		}
		_ = m.store.Mutate(func(st *state.State) error {
			st.ActiveProfile = next
			return nil
		})
		m.refresh()
		return nil
	}
}

func (m *Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("projwm cockpit"))
	b.WriteString("\n")

	st := m.st
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
	b.WriteString(headerStyle.Render(fmt.Sprintf("> %s_", m.filter)))
	b.WriteString("\n\n")

	ents := m.filtered()
	if len(ents) == 0 {
		b.WriteString(dimStyle.Render("  (no entries — use `projwm profile create` and `projwm up` to begin)\n"))
	}
	for i, e := range ents {
		line := e.display()
		if i == m.cursor {
			line = hlStyle.Render("▶ " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↵ jump / activate   tab cycle profile   r refresh   esc quit\n"))
	if m.lastErr != "" {
		b.WriteString(errStyle.Render("error: " + m.lastErr + "\n"))
	}
	if m.lastInfo != "" {
		b.WriteString(dimStyle.Render(m.lastInfo + "\n"))
	}
	return b.String()
}

func dim(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
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
