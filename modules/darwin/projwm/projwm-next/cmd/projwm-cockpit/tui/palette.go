// Command palette (requirements §9.8). Triggered by Ctrl-P from any
// tab / wizard / prompt. Enumerates every TUI-reachable action so
// discovery does not depend on memorising tab-specific keys.
//
// Each paletteAction owns its Execute closure so the palette doesn't
// need to mirror the dispatch tables in update.go — the Update can
// stay terse and the palette can grow new entries by appending to
// buildPaletteActions().
package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yuu-th/projwm-next/internal/cockpitsnap"
	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

type paletteAction struct {
	Label   string // shown left
	Hint    string // shown right, dimmed
	Execute func(m Model) (Model, tea.Cmd)
}

// paletteOpen initialises and shows the palette.
func (m *Model) paletteOpen() {
	m.paletteActive = true
	m.paletteCursor = 0
	m.paletteQuery = ""
	m.paletteActions = m.buildPaletteActions()
}

func (m *Model) paletteClose() {
	m.paletteActive = false
	m.paletteQuery = ""
	m.paletteActions = nil
	m.paletteCursor = 0
}

// paletteVisibleActions returns the subset of actions matching the
// current query (case-insensitive substring across Label+Hint).
func (m Model) paletteVisibleActions() []paletteAction {
	q := strings.ToLower(strings.TrimSpace(m.paletteQuery))
	if q == "" {
		return m.paletteActions
	}
	out := make([]paletteAction, 0, len(m.paletteActions))
	for _, a := range m.paletteActions {
		hay := strings.ToLower(a.Label + " " + a.Hint)
		if strings.Contains(hay, q) {
			out = append(out, a)
		}
	}
	return out
}

// paletteHandleKey dispatches palette-specific keys. All other keys
// (printable) extend the query.
func (m Model) paletteHandleKey(key string) (Model, tea.Cmd) {
	switch key {
	case "esc":
		m.paletteClose()
		return m, nil
	case "ctrl+p":
		// Toggle off so the same chord closes it.
		m.paletteClose()
		return m, nil
	case "up", "ctrl+k":
		if m.paletteCursor > 0 {
			m.paletteCursor--
		}
		return m, nil
	case "down", "ctrl+j":
		v := m.paletteVisibleActions()
		if m.paletteCursor < len(v)-1 {
			m.paletteCursor++
		}
		return m, nil
	case "enter":
		v := m.paletteVisibleActions()
		if m.paletteCursor < 0 || m.paletteCursor >= len(v) {
			m.paletteClose()
			return m, nil
		}
		fn := v[m.paletteCursor].Execute
		m.paletteClose()
		if fn == nil {
			return m, nil
		}
		return fn(m)
	case "backspace":
		if len(m.paletteQuery) > 0 {
			m.paletteQuery = m.paletteQuery[:len(m.paletteQuery)-1]
			m.paletteCursor = 0
		}
		return m, nil
	}
	if isPrintable(key) {
		m.paletteQuery += key
		m.paletteCursor = 0
	}
	return m, nil
}

// buildPaletteActions enumerates everything the user can invoke. The
// list is built per-call (cheap) so it can reflect snapshot state
// (e.g. include one action per existing profile for fast switching).
func (m Model) buildPaletteActions() []paletteAction {
	var out []paletteAction

	out = append(out, paletteAction{
		Label: "new project",
		Hint:  "create + assign + windows (wizard)",
		Execute: func(m Model) (Model, tea.Cmd) {
			m.wizardOpenNewProject()
			return m, nil
		},
	})
	out = append(out, paletteAction{
		Label: "new profile",
		Hint:  "create profile (wizard)",
		Execute: func(m Model) (Model, tea.Cmd) {
			m.wizardOpenNewProfile()
			return m, nil
		},
	})
	out = append(out, paletteAction{
		Label: "reconcile",
		Hint:  "force planner reconcile",
		Execute: func(m Model) (Model, tea.Cmd) {
			return m, submitIntentCmd(m.cfg.Client, intent.Reconcile{})
		},
	})
	out = append(out, paletteAction{
		Label: "validate environment",
		Hint:  "re-run runtime validation",
		Execute: func(m Model) (Model, tea.Cmd) {
			return m, submitIntentCmd(m.cfg.Client, intent.ValidateEnvironment{})
		},
	})
	out = append(out, paletteAction{
		Label: "go to Slots",
		Hint:  "tab 1",
		Execute: func(m Model) (Model, tea.Cmd) {
			m = m.switchTab(TabSlots)
			return m, nil
		},
	})
	out = append(out, paletteAction{
		Label: "go to Cards",
		Hint:  "tab 2",
		Execute: func(m Model) (Model, tea.Cmd) {
			m = m.switchTab(TabCards)
			return m, nil
		},
	})
	out = append(out, paletteAction{
		Label: "go to Archived",
		Hint:  "tab 3",
		Execute: func(m Model) (Model, tea.Cmd) {
			m = m.switchTab(TabArchived)
			return m, nil
		},
	})
	out = append(out, paletteAction{
		Label: "go to Profiles",
		Hint:  "tab 4",
		Execute: func(m Model) (Model, tea.Cmd) {
			m = m.switchTab(TabProfiles)
			return m, nil
		},
	})
	out = append(out, paletteAction{
		Label: "go to Trace",
		Hint:  "tab 5",
		Execute: func(m Model) (Model, tea.Cmd) {
			m = m.switchTab(TabTrace)
			return m, nil
		},
	})
	out = append(out, paletteAction{
		Label: "dismiss all cards",
		Hint:  "Ctrl+L",
		Execute: func(m Model) (Model, tea.Cmd) {
			m.beginPrompt(promptConfirmClear, listItem{})
			return m, nil
		},
	})
	out = append(out, paletteAction{
		Label: "hide cockpit",
		Hint:  "Esc",
		Execute: func(m Model) (Model, tea.Cmd) {
			tm, cmd := m.hideAndQuit()
			return tm.(Model), cmd
		},
	})

	// Selected-item-aware actions.
	sel := m.Selected()
	if sel.Kind == itemSlot && sel.Project != "" {
		pid := sel.Project
		out = append(out, paletteAction{
			Label: "archive selected (" + string(pid) + ")",
			Hint:  string(pid) + " → archived",
			Execute: func(m Model) (Model, tea.Cmd) {
				return m, submitIntentCmd(m.cfg.Client, intent.ArchiveProject{Project: pid})
			},
		})
		out = append(out, paletteAction{
			Label: "unassign selected slot",
			Hint:  string(sel.Slot) + " → park",
			Execute: func(m Model) (Model, tea.Cmd) {
				return m, submitIntentCmd(m.cfg.Client, intent.UnassignSlot{Slot: sel.Slot})
			},
		})
		out = append(out, paletteAction{
			Label: "add window to " + string(pid),
			Hint:  "ai / shell / editor / browser (wizard)",
			Execute: func(m Model) (Model, tea.Cmd) {
				m.wizardOpenAddWindow(pid)
				return m, nil
			},
		})
		// SSOT N-12 (2026-05-20): AcceptManualLayout is removed in favor
		// of Tier 2 auto-overwrite via AutoSyncLayout. Manual layout is
		// no longer a user-facing palette action.
		_ = pid
	}
	// Orphan card → DismissOrphanWindow direct shortcut. Cards carry
	// their backing live window id via Context["live"] (set in
	// controller.PromoteOrphans).
	if sel.Kind == itemCard && sel.CardContext != nil {
		if liveStr, ok := sel.CardContext["live"]; ok && liveStr != "" {
			lid := w.LiveWindowID(liveStr)
			out = append(out, paletteAction{
				Label: "close orphan window (selected card)",
				Hint:  "DismissOrphanWindow → close live",
				Execute: func(m Model) (Model, tea.Cmd) {
					return m, submitIntentCmd(m.cfg.Client, intent.DismissOrphanWindow{LiveID: lid, Action: "close"})
				},
			})
		}
	}
	if sel.Kind == itemArchive && sel.Project != "" {
		pid := sel.Project
		out = append(out, paletteAction{
			Label: "unarchive selected (" + string(pid) + ")",
			Hint:  "prompt for target slot",
			Execute: func(m Model) (Model, tea.Cmd) {
				m.beginPrompt(promptUnarchive, listItem{Kind: itemArchive, Project: pid})
				return m, nil
			},
		})
	}

	// One "switch to <profile>" per non-active profile.
	profIDs := make([]w.ProfileID, 0, len(m.snap.Profiles))
	for id := range m.snap.Profiles {
		profIDs = append(profIDs, id)
	}
	sort.Slice(profIDs, func(i, j int) bool { return profIDs[i] < profIDs[j] })
	for _, id := range profIDs {
		if id == m.snap.ActiveProfile {
			continue
		}
		target := id
		out = append(out, paletteAction{
			Label: "switch profile → " + string(target),
			Hint:  "SwitchProfile",
			Execute: func(m Model) (Model, tea.Cmd) {
				return m, submitIntentCmd(m.cfg.Client, intent.SwitchProfile{To: target})
			},
		})
	}

	return out
}

// projectHasBrowser returns true if the project's DesiredWindows
// include any WindowBrowser entry. SyncBrowserTabs only makes sense
// when there's a browser window to sync against.
func projectHasBrowser(snap cockpitsnap.Snapshot, pid w.ProjectID) bool {
	pr, ok := snap.Projects[pid]
	if !ok {
		return false
	}
	for _, dw := range pr.Windows {
		if dw.Kind == w.WindowBrowser {
			return true
		}
	}
	return false
}

// paletteView renders the overlay (header + query line + visible
// actions with cursor + dimmed hint).
func (m Model) paletteView() string {
	if !m.paletteActive {
		return ""
	}
	var b strings.Builder
	b.WriteString(styleHeading.Render("┌─ Command palette ──────────────────────────────────────┐"))
	b.WriteString("\n")
	q := m.paletteQuery
	if q == "" {
		q = styleDim.Render("(type to filter — Esc to close)")
	}
	b.WriteString("│ ▶ " + q + styleCursor.Render("▌"))
	b.WriteString("\n│\n")
	visible := m.paletteVisibleActions()
	if len(visible) == 0 {
		b.WriteString("│ " + styleDim.Render("(no actions match)"))
		b.WriteString("\n")
	}
	for i, a := range visible {
		marker := "  "
		row := a.Label
		if i == m.paletteCursor {
			marker = styleCursor.Render("▶ ")
			row = styleCursor.Render(row)
		}
		hint := ""
		if a.Hint != "" {
			hint = "  " + styleDim.Render(a.Hint)
		}
		b.WriteString("│ " + marker + row + hint)
		b.WriteString("\n")
	}
	b.WriteString("└" + strings.Repeat("─", 56) + "┘")
	return b.String()
}
