package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/yuu-th/projwm-next/internal/cockpitsnap"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// View implements tea.Model. It composes the cockpit pane: topbar
// (§9.1) → cards (§9.3 / §10) → itemlist (§9.2) → filter (§9.4) →
// status line. The View is total (no side effects); only the styled
// strings change per frame.
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	parts := []string{
		m.topbarView(),
		m.tabBarView(), // v2.9 §9.4 top tabs
	}
	if m.showHelp {
		parts = append(parts, m.helpView())
		parts = append(parts, m.bottomMenuView())
		return strings.Join(parts, "\n")
	}
	// v2.9 §9.7 wizard overlay (project / profile creation form).
	if m.wizardActive {
		parts = append(parts, m.wizardView())
		parts = append(parts, m.statusView())
		parts = append(parts, m.bottomMenuView())
		return strings.Join(parts, "\n")
	}
	// v2.9 §9.8 Command palette overlay — replaces main content.
	if m.paletteActive {
		parts = append(parts, m.paletteView())
		parts = append(parts, m.statusView())
		parts = append(parts, m.bottomMenuView())
		return strings.Join(parts, "\n")
	}
	// v2.9 §9.5 profile picker overlay — replaces main content while open.
	if m.profilePickerActive {
		parts = append(parts, m.profilePickerView())
		parts = append(parts, m.statusView())
		parts = append(parts, m.bottomMenuView())
		return strings.Join(parts, "\n")
	}
	// v2.9 §9.4 Trace tab detail overlay — Enter on a trace row fills
	// m.traceDetail with the formatted JSON; Esc closes.
	if m.traceDetailActive {
		parts = append(parts, m.traceDetailView())
		parts = append(parts, m.statusView())
		parts = append(parts, m.bottomMenuView())
		return strings.Join(parts, "\n")
	}
	// Render the active tab's content.
	parts = append(parts, m.activeTabContent())
	parts = append(parts,
		m.filterOrPromptView(),
		m.statusView(),
		m.bottomMenuView(), // v2.9 §9.9 context-aware bottom menu
	)
	return strings.Join(parts, "\n")
}

// tabBarView renders the v2.9 §9.4 top tab indicator. The active tab is
// highlighted; the cards tab shows the count when N > 0.
func (m Model) tabBarView() string {
	cardsCount := len(m.snap.ActiveCards)
	labels := map[tabKind]string{
		TabSlots:    "Slots",
		TabCards:    fmt.Sprintf("Cards (%d)", cardsCount),
		TabArchived: "Archived",
		TabProfiles: "Profiles",
		TabTrace:    "Trace",
	}
	cells := []string{}
	for i, t := range tabsOrder {
		label := labels[t]
		key := fmt.Sprintf("%d", i+1)
		cell := fmt.Sprintf("%s %s", styleDim.Render(key), label)
		if t == m.activeTab {
			cell = styleHeading.Render("[ " + label + " ]")
		}
		cells = append(cells, cell)
	}
	return strings.Join(cells, "  ") + "\n" + strings.Repeat("─", 60)
}

// activeTabContent dispatches to the per-tab content renderer.
// Slots / Archived / Profiles / Trace all share itemsView() so cursor +
// rendering is consistent; only Cards uses its dedicated modal view.
func (m Model) activeTabContent() string {
	switch m.activeTab {
	case TabCards:
		if len(m.snap.ActiveCards) == 0 {
			return styleDim.Render("(no cards)")
		}
		return m.cardModalView()
	case TabSlots:
		return m.cardsView() + "\n" + m.itemsView()
	default:
		return m.itemsView()
	}
}

// profilePickerView renders the v2.9 §9.5 profile picker overlay
// triggered by `;`. Lists all profiles, highlights the current and the
// cursor, and binds Enter to SwitchProfile.
func (m Model) profilePickerView() string {
	profiles := sortedProfiles(m.snap.Profiles)
	if len(profiles) == 0 {
		return styleDim.Render("(no profiles defined)")
	}
	lines := []string{styleHeading.Render("Switch profile"), ""}
	for i, id := range profiles {
		marker := "  "
		if id == m.snap.ActiveProfile {
			marker = "★ "
		}
		desc := ""
		if p, ok := m.snap.Profiles[id]; ok {
			desc = p.Description
		}
		row := fmt.Sprintf("%s%-12s  %s", marker, id, styleDim.Render(desc))
		if i == m.profilePickerCursor {
			row = styleCursor.Render("▸ " + row)
		} else {
			row = "  " + row
		}
		lines = append(lines, row)
	}
	lines = append(lines, "", styleDim.Render("↑↓ select   Enter switch   Esc/; cancel"))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("117")).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))
}

// archivedTabView is a Phase γ placeholder. Will surface
// (archivedTabView / profilesTabView / traceTabView placeholders
// removed in γ.0 — all four non-Cards tabs now render via itemsView
// over their tab-specific items list, so cursor handling, filtering,
// and prompt overlays work uniformly.)

// bottomMenuView renders the v2.9 §9.9 context-aware bottom menu.
// Three rows: Navigate (固定) / Actions (context) / Help (固定).
// The Actions row content is calculated by actionsForContext().
func (m Model) bottomMenuView() string {
	sep := strings.Repeat("─", 60)
	nav := styleDim.Render(" Navigate:") + "  [↑↓] cursor  [Tab/Shift+Tab] next/prev tab  [1-5] jump tab  [/] filter"
	actions := styleDim.Render(" Actions: ") + "  " + m.actionsForContext()
	help := styleDim.Render(" Help:    ") + "  [?] help  [Ctrl-P] palette  [;] switch profile  [o] swap tab  [Esc] hide"
	return sep + "\n" + nav + "\n" + actions + "\n" + help
}

// actionsForContext returns a space-separated list of [key] action pairs
// representing all actions the user can perform RIGHT NOW given the
// current tab + cursor. Phase α = placeholder strings per tab; Phase ζ
// will compute fully context-aware (cursor item + snapshot).
func (m Model) actionsForContext() string {
	if m.paletteActive {
		return "[type] filter  [↑↓] cursor  [Enter] run  [Esc/Ctrl-P] close"
	}
	if m.wizardActive {
		return "[Tab] next field  [Shift+Tab] prev  [←/→] choose  [Space] toggle  [Enter] submit  [Esc] cancel"
	}
	if m.profilePickerActive {
		return "[↑↓] cursor  [Enter] switch  [Esc/;] close"
	}
	if m.traceDetailActive {
		return "[Esc] close detail"
	}
	sel := m.Selected()
	switch m.activeTab {
	case TabSlots:
		return m.actionsSlots(sel)
	case TabCards:
		return m.actionsCards()
	case TabArchived:
		return m.actionsArchived(sel)
	case TabProfiles:
		return m.actionsProfiles(sel)
	case TabTrace:
		return m.actionsTrace(sel)
	}
	return ""
}

// actionsSlots returns the context-aware Slots-tab action row. The set
// depends on what's under the cursor:
//   - itemSlot with project → activate / unassign / archive / remove-win
//   - itemSlot empty        → activate / new project
//   - itemViewer            → activate viewer
//   - itemParked            → unarchive jump / archive (parked) / new project
//   - empty list            → just `n new project`
func (m Model) actionsSlots(sel listItem) string {
	parts := []string{}
	switch sel.Kind {
	case itemSlot:
		if sel.Project != "" {
			parts = append(parts,
				"[Enter] jump to "+string(sel.Slot),
				"[d] unassign "+string(sel.Slot),
				"[a] archive "+string(sel.Project),
				"[r] remove window from "+string(sel.Project),
				"[+] add window to "+string(sel.Project),
			)
		} else {
			parts = append(parts, "[Enter] jump to "+string(sel.Slot))
		}
	case itemViewer:
		parts = append(parts, "[Enter] focus viewer "+string(sel.Slot))
	case itemParked:
		parts = append(parts,
			"[Enter] (parked — no jump)",
			"[a] archive "+string(sel.Project),
		)
	}
	// Always-available on Slots.
	parts = append(parts, "[n] new project", "[u] go to Archived", "[Ctrl-P] palette")
	return strings.Join(parts, "  ")
}

func (m Model) actionsCards() string {
	if len(m.snap.ActiveCards) == 0 {
		return styleDim.Render("(no cards — switch tab with 1-5)")
	}
	return "[Enter] adopt  [n] new project + adopt  [c] close  [t] carry over  [←/→] prev/next  [Ctrl-L] dismiss all"
}

func (m Model) actionsArchived(sel listItem) string {
	if sel.Kind != itemArchive {
		return styleDim.Render("(select an archived project — `↑↓` to move)")
	}
	return "[Enter/u] unarchive " + string(sel.Project) + "  [x] purge " + string(sel.Project)
}

func (m Model) actionsProfiles(sel listItem) string {
	parts := []string{"[n] new profile"}
	if sel.Kind == itemProfile && sel.Profile != "" {
		if sel.Profile != m.snap.ActiveProfile {
			parts = append(parts, "[Enter] switch to "+string(sel.Profile))
			parts = append(parts, "[d] delete "+string(sel.Profile))
		} else {
			parts = append(parts, styleDim.Render("(active — cannot delete)"))
		}
		parts = append(parts, "[r] rename "+string(sel.Profile))
	}
	parts = append(parts, "[;] picker")
	return strings.Join(parts, "  ")
}

func (m Model) actionsTrace(sel listItem) string {
	if sel.Kind == itemTrace && sel.TracePath != "" {
		return "[Enter] view detail  [r] reload"
	}
	return "[r] reload  " + styleDim.Render("(select a trace row to view detail)")
}

var (
	styleBold     = lipgloss.NewStyle().Bold(true)
	styleHeading  = lipgloss.NewStyle().Bold(true).Underline(true)
	styleDim      = lipgloss.NewStyle().Faint(true)
	styleHi       = lipgloss.NewStyle().Reverse(true)
	styleCursor   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	styleWarn     = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleErr      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleOK       = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	styleCardNew  = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
	styleAction   = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	styleDestruct = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

// topbarView renders requirements §9.1. Single-line: gen / epoch /
// active profile / convergence / digest / cards count. A second line
// surfaces the profile description and InactivePolicy.
func (m Model) topbarView() string {
	s := m.snap
	conv := s.ConvergenceStatus
	if conv == "" {
		conv = "CONVERGED"
	}
	digest := styleOK.Render("OK")
	if s.ManifestDigestMismatch {
		digest = styleErr.Render("MISMATCH")
	}
	header := fmt.Sprintf("%s  src=%s  gen=%s  epoch=%d  prof=%s  %s  digest=%s  cards=%d",
		styleHeading.Render("projwm-cockpit"),
		s.Source,
		shortGen(s.Generation),
		s.Epoch,
		s.ActiveProfile,
		convStyle(conv).Render(conv),
		digest,
		len(s.ActiveCards),
	)

	var policyLine string
	if active, ok := s.Profiles[s.ActiveProfile]; ok {
		policy := string(active.InactivePolicy)
		if policy == "" {
			policy = "remove(default)"
		}
		desc := ""
		if active.Description != "" {
			desc = " — " + active.Description
		}
		policyLine = fmt.Sprintf("  policy=%s%s", policy, desc)
	} else {
		policyLine = styleWarn.Render("  active profile not in manifest")
	}
	return header + "\n" + policyLine
}

func convStyle(conv string) lipgloss.Style {
	switch {
	case strings.HasPrefix(conv, "CONVERGED"):
		return styleOK
	case strings.HasPrefix(conv, "CONVERGING"):
		return styleWarn
	default:
		return styleErr
	}
}

func shortGen(g w.GenerationID) string {
	s := string(g)
	if len(s) > 10 {
		return s[:10]
	}
	return s
}

// traceDetailView renders the v2.9 §9.4 Trace tab Enter-detail overlay:
// pretty-printed JSON of the selected trace, scrollable nothing (Esc
// closes). Width is constrained so long arrays wrap on the right gutter
// rather than spilling off-screen.
func (m Model) traceDetailView() string {
	if !m.traceDetailActive {
		return ""
	}
	body := m.traceDetail
	if body == "" {
		body = styleDim.Render("(no trace loaded)")
	}
	header := styleHeading.Render("Trace detail")
	hint := styleDim.Render("Esc: close")
	return header + "  " + hint + "\n" + strings.Repeat("─", 60) + "\n" + body
}

// cardModalView renders the full-screen 2-column card modal.
// Left: focused card detail + actions. Right: workspace zoom-out diagram.
// Realises requirements §10 + ユーザ提案 2026-05-18 (workspace 全体俯瞰
// view を見ながらカード判断ができる)。
func (m Model) cardModalView() string {
	cards := sortedActiveCards(m.snap.ActiveCards)
	if len(cards) == 0 {
		return ""
	}
	cursor := m.cardModalCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(cards) {
		cursor = len(cards) - 1
	}
	card := cards[cursor]
	now := time.Now()
	leftWidth := m.width / 2
	if leftWidth < 40 {
		leftWidth = 40
	}
	rightWidth := m.width - leftWidth - 3
	if rightWidth < 30 {
		rightWidth = 30
	}
	left := m.cardModalLeft(card, cursor, len(cards), now, leftWidth)
	right := m.cardModalRight(card, rightWidth)
	// lipgloss.JoinHorizontal aligns top of both columns; we pad with a
	// 1-char gutter so the divider is visible.
	gutter := lipgloss.NewStyle().Faint(true).Render(strings.Repeat("│\n", strings.Count(left, "\n")+1))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", gutter, " ", right)
}

func (m Model) cardModalLeft(card w.Card, cursor, total int, now time.Time, width int) string {
	header := fmt.Sprintf("%s  %s · %s   (%d/%d)",
		cardTypeStyle(card.Type).Render("["+string(card.Type)+"]"),
		time.Unix(0, card.CreatedAt).Format("15:04:05"),
		relativeTime(card.CreatedAt, now),
		cursor+1, total,
	)
	body := []string{
		styleHeading.Render("Card Detail"),
		header,
		"",
		card.Subject,
		"",
		styleDim.Render("Context:"),
		redactedContext(card.Context),
		"",
	}
	// If a prompt is active inside the modal, surface it inline so the
	// user can answer without leaving the popup. Without this the prompt
	// is invisible and an empty Enter triggers "project required" with
	// no obvious cause (bug 2026-05-18 reported by user).
	if m.prompt != promptNone {
		body = append(body,
			styleHeading.Render(promptHeaderText(m.prompt, m.promptStep)),
			m.promptInput.View(),
			styleDim.Render("  Enter: submit   Esc: cancel"),
		)
		if m.status != "" {
			body = append(body, styleWarn.Render("  "+m.status))
		}
	} else {
		body = append(body, styleHeading.Render("Actions"))
		for _, a := range card.Actions {
			var style lipgloss.Style
			if isDestructiveAction(a) {
				style = styleDestruct
			} else {
				style = styleAction
			}
			body = append(body, style.Render(fmt.Sprintf("  <%s>  %s", a.Key, a.Label)))
		}
		body = append(body,
			"",
			styleDim.Render("← prev card    → next card    <q> close modal"),
		)
	}
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("213")).
		Padding(0, 1).
		Render(strings.Join(body, "\n"))
}

// promptHeaderText returns a human-readable heading for the inline
// prompt inside the card modal. Mirrors promptPlaceholder but is
// rendered as a heading so the user sees what step they are on.
func promptHeaderText(k promptKind, step int) string {
	switch k {
	case promptRespawnGhostty:
		if step == 0 {
			return "Adopt as project (step 1/2)"
		}
		return "Adopt as kind (step 2/2)"
	case promptAdoptOrphan:
		if step == 0 {
			return "Adopt as project (step 1/2)"
		}
		return "Adopt as kind (step 2/2)"
	case promptNewProject:
		return "New project"
	case promptUnarchive:
		return "Unarchive — pick slot"
	case promptRemoveWindow:
		return "Remove window"
	case promptConfirmClear:
		return "Confirm: dismiss all cards?"
	case promptCarryOver:
		return "Card detail (carry over)"
	case promptNewProfile:
		return "New profile — type id"
	case promptDeleteProfile:
		return "Delete profile — type id to confirm"
	case promptRenameProfile:
		return "Rename profile — new id"
	case promptPurgeProject:
		return "Purge archived project — type id to confirm"
	}
	return "Prompt"
}

// cardModalRight renders the workspace zoom-out diagram. Each managed
// slot (Q-P) is one row showing project assignment + per-window state
// (✓ live, ✗ desired-but-missing, ? observed-only). The slot related
// to the focused card is highlighted with ★.
func (m Model) cardModalRight(card w.Card, width int) string {
	body := []string{
		styleHeading.Render("Workspace Overview"),
		fmt.Sprintf("active profile: %s", m.snap.ActiveProfile),
		"",
	}
	relatedSlot := cardRelatedSlot(card, m.snap)
	prof, ok := m.snap.Profiles[m.snap.ActiveProfile]
	if !ok {
		body = append(body, styleDim.Render("(no active profile)"))
	} else {
		for _, slot := range m.snap.Slots {
			pid, assigned := prof.Assignments[slot.ID]
			mark := " "
			if slot.ID == relatedSlot {
				mark = styleCardNew.Render("★")
			}
			row := fmt.Sprintf(" %s %s  %s", mark, styleBold.Render(string(slot.ID)), renderWindowStates(m.snap, pid))
			if assigned {
				row = fmt.Sprintf(" %s %s  proj=%s  %s", mark, styleBold.Render(string(slot.ID)), pid, renderWindowStates(m.snap, pid))
			} else {
				row = fmt.Sprintf(" %s %s  %s", mark, styleBold.Render(string(slot.ID)), styleDim.Render("(unassigned)"))
			}
			body = append(body, row)
		}
	}
	body = append(body, "")
	body = append(body, styleDim.Render("Viewer (workspace A):"))
	viewers := []string{}
	for _, lw := range m.snap.LiveWindows {
		if lw.Kind == w.WindowViewer && lw.MatchedTo != nil {
			viewers = append(viewers, fmt.Sprintf("ai-%d(%s)", lw.MatchedTo.Index, lw.MatchedTo.Project))
		}
	}
	if len(viewers) == 0 {
		body = append(body, styleDim.Render("  (empty)"))
	} else {
		body = append(body, "  "+strings.Join(viewers, " "))
	}
	if len(m.snap.Parked) > 0 {
		body = append(body, "")
		body = append(body, styleDim.Render("Park projects:"))
		for _, pid := range m.snap.Parked {
			body = append(body, "  "+string(pid))
		}
	}
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("117")).
		Padding(0, 1).
		Render(strings.Join(body, "\n"))
}

// cardRelatedSlot inspects the card context for a project / slot hint
// and returns the slot id if the active profile assigns that project,
// else empty. Used by cardModalRight to highlight the relevant slot.
func cardRelatedSlot(card w.Card, snap cockpitsnap.Snapshot) w.SlotID {
	pid, ok := card.Context["project"]
	if !ok {
		return ""
	}
	prof, ok := snap.Profiles[snap.ActiveProfile]
	if !ok {
		return ""
	}
	for slot, p := range prof.Assignments {
		if string(p) == pid {
			return slot
		}
	}
	return ""
}

// sortedActiveCards returns a copy of cards sorted by CreatedAt descending
// so the newest is index 0 (matches the §10.4 "newest on top" rule).
func sortedActiveCards(cards []w.Card) []w.Card {
	out := make([]w.Card, len(cards))
	copy(out, cards)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

// cardsView renders the top-of-screen card stack. Requirements §9.3
// (cards above everything) + §10 (type / CreatedAt / summary /
// context / actions). The newest card is at the top (G7, rebuildItems
// sort).
func (m Model) cardsView() string {
	var cardItems []listItem
	for _, it := range m.items {
		if it.Kind == itemCard {
			cardItems = append(cardItems, it)
		}
	}
	if len(cardItems) == 0 {
		return styleDim.Render("(no cards)")
	}
	var b strings.Builder
	b.WriteString(styleHeading.Render("Cards (newest first):"))
	b.WriteString("\n")
	now := time.Now()
	for i, it := range cardItems {
		cursor := "  "
		if m.cursorOnItem(it) {
			cursor = styleCursor.Render("▸ ")
		}
		// Header line: cursor [TYPE] time · summary
		header := fmt.Sprintf("%s%s  %s  %s",
			cursor,
			cardTypeStyle(it.CardType).Render("["+string(it.CardType)+"]"),
			styleDim.Render(relativeTime(it.CardCreatedAt, now)),
			highlight(strings.TrimPrefix(it.Label, "["+string(it.CardType)+"] "), m.filter.Value()),
		)
		b.WriteString(header)
		b.WriteString("\n")
		// Context (kv map, redacted).
		if ctx := redactedContext(it.CardContext); ctx != "" {
			b.WriteString("       ")
			b.WriteString(styleDim.Render(ctx))
			b.WriteString("\n")
		}
		// Actions — Enter / letter / Esc / t.
		for _, a := range it.CardActions {
			risk := styleAction.Render("[~]")
			if isDestructiveAction(a) {
				risk = styleDestruct.Render("[!]")
			}
			b.WriteString(fmt.Sprintf("       <%s> %s %s\n",
				a.Key, risk, a.Label,
			))
		}
		// Always-available t (carry-over) and Esc (dismiss).
		b.WriteString(styleDim.Render(fmt.Sprintf("       <t> carry-over   <Esc> dismiss")))
		b.WriteString("\n")
		_ = i
	}
	return b.String()
}

func cardTypeStyle(t w.CardType) lipgloss.Style {
	switch t {
	case w.CardTypeNew, w.CardTypeOrphan:
		return styleCardNew
	case w.CardTypeMoved, w.CardTypeClosed, w.CardTypeOmniwmRecovery:
		return styleWarn
	case w.CardTypeReplan, w.CardTypeInvariant, w.CardTypeManifest:
		return styleErr
	}
	return styleBold
}

// redactedContext returns "k1=v1 k2=v2" with private fields redacted.
// Requirements §9.6 / §13.3: never surface raw URLs or PrivatePayloadRefs.
func redactedContext(ctx map[string]string) string {
	if len(ctx) == 0 {
		return ""
	}
	var keys []string
	for k := range ctx {
		keys = append(keys, k)
	}
	// Stable order without sort import noise — small map.
	if len(keys) > 1 {
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[j] < keys[i] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}
	}
	var pairs []string
	for _, k := range keys {
		v := ctx[k]
		if isPrivateKey(k) {
			v = "<redacted>"
		}
		pairs = append(pairs, k+"="+v)
	}
	return strings.Join(pairs, " ")
}

func isPrivateKey(k string) bool {
	switch k {
	case "url", "URL", "private", "payload", "privatePayload", "privatePayloadRef":
		return true
	}
	return false
}

// cursorOnItem returns true when the global cursor sits on this item.
// View doesn't have indices for sublists so we compare by id-ish fields.
func (m Model) cursorOnItem(it listItem) bool {
	sel := m.Selected()
	return sel.Kind == it.Kind && sel.CardID == it.CardID && sel.Slot == it.Slot && sel.Project == it.Project && sel.Profile == it.Profile
}

// itemsView renders the §9.2 list (slots/parked/archived/viewer/other-
// profiles). Cards are skipped because they live in cardsView.
func (m Model) itemsView() string {
	var b strings.Builder
	// Header per active tab (§9.4 / v2.9): Slots view shows "Items:" with
	// internal section subheads; the Profiles/Archived/Trace tabs use a
	// single per-tab heading so the user immediately knows what is listed.
	switch m.activeTab {
	case TabProfiles:
		b.WriteString(styleHeading.Render("Profiles"))
	case TabArchived:
		b.WriteString(styleHeading.Render("Archived projects"))
	case TabTrace:
		b.WriteString(styleHeading.Render("Trace"))
	default:
		b.WriteString(styleHeading.Render("Items:"))
	}
	b.WriteString("\n")
	if len(m.items) == 0 {
		b.WriteString(styleDim.Render("  (no items match filter)"))
		b.WriteString("\n")
		return b.String()
	}
	var lastKind itemKind
	for _, it := range m.items {
		if it.Kind == itemCard {
			continue
		}
		// Section sub-header only on the Slots tab (mixed kinds).
		if m.activeTab == TabSlots && it.Kind != lastKind {
			b.WriteString(styleDim.Render("  -- " + sectionLabel(it.Kind) + " --"))
			b.WriteString("\n")
			lastKind = it.Kind
		}
		cursor := "  "
		if m.cursorOnItem(it) {
			cursor = styleCursor.Render("▸ ")
		}
		b.WriteString(cursor)
		b.WriteString(highlight(it.Label, m.filter.Value()))
		b.WriteString("\n")
		// Per-window state for slot rows (tmux + live + focused).
		if it.Kind == itemSlot && it.Project != "" {
			b.WriteString(renderWindowStates(m.snap, it.Project))
		}
		// Profile rows: show inline detail (policy + assignments).
		if it.Kind == itemProfile && it.Detail != "" {
			b.WriteString(styleDim.Render("    " + it.Detail))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func sectionLabel(k itemKind) string {
	switch k {
	case itemSlot:
		return "Active profile slots"
	case itemParked:
		return "Park projects"
	case itemArchive:
		return "Archived"
	case itemViewer:
		return "Viewer (workspace A) AI streams"
	case itemProfile:
		return "Other profiles"
	}
	return string(k)
}

// renderWindowStates prints one row per DesiredWindow showing tmux
// session liveness + observed window liveness + focused flag.
func renderWindowStates(s cockpitsnap.Snapshot, pid w.ProjectID) string {
	pr, ok := s.Projects[pid]
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, dw := range pr.Windows {
		tmuxOK := s.TmuxSessions[cockpitsnap.TmuxSessionForWindow(pid, dw)]
		liveOK := false
		focused := false
		for _, lw := range s.LiveWindows {
			if lw.MatchedTo != nil && *lw.MatchedTo == dw.ID {
				liveOK = true
				if lw.Focused {
					focused = true
				}
				break
			}
		}
		mark := func(ok bool, ch string) string {
			if ok {
				return styleOK.Render(ch)
			}
			return styleErr.Render(ch)
		}
		focusMark := ""
		if focused {
			focusMark = " " + styleWarn.Render("◀ focus")
		}
		fmt.Fprintf(&b, "       %s-%d  tmux:%s  live:%s%s\n",
			dw.Kind, dw.ID.Index,
			mark(tmuxOK, "✓"),
			mark(liveOK, "✓"),
			focusMark,
		)
	}
	return b.String()
}

// filterOrPromptView renders the bottom command line: prompt if a
// modal is active, otherwise the §9.4 fzf filter.
func (m Model) filterOrPromptView() string {
	if m.prompt != promptNone {
		return styleBold.Render("▶ "+promptHeader(m.prompt, m.promptStep)) + " " + m.promptInput.View()
	}
	prefix := "/filter > "
	val := m.filter.Value()
	if val == "" {
		return styleDim.Render(prefix + "(type to filter)")
	}
	return prefix + styleHi.Render(val) + styleDim.Render("  [Esc clears]")
}

func promptHeader(k promptKind, step int) string {
	switch k {
	case promptNewProject:
		return "New project ID:"
	case promptUnarchive:
		return "Unarchive → slot id:"
	case promptRemoveWindow:
		return "Remove window (KIND-N):"
	case promptConfirmClear:
		return "Dismiss all cards? (y/n):"
	case promptAdoptOrphan:
		if step == 0 {
			return "Adopt orphan → project:"
		}
		return "Adopt orphan → kind:"
	case promptRespawnGhostty:
		if step == 0 {
			return "Respawn ghostty → project:"
		}
		return "Respawn ghostty → kind:"
	case promptCarryOver:
		return "Card detail (Enter to close):"
	}
	return string(k)
}

// statusView prints the last status message + a one-line cheat sheet.
func (m Model) statusView() string {
	var b strings.Builder
	if m.status != "" {
		b.WriteString(styleWarn.Render(m.status))
		b.WriteString("\n")
	}
	cheat := "[? help · Enter act · Tab profile · Esc clear/hide · Ctrl+C quit · Ctrl+L dismiss-all]"
	b.WriteString(styleDim.Render(cheat))
	return b.String()
}

// helpView renders the ? screen — a full text dump of §9.5.
func (m Model) helpView() string {
	lines := []string{
		styleHeading.Render("projwm-cockpit help (requirements §9.5)"),
		"",
		"↑ / ↓ / Ctrl+J / Ctrl+K   move cursor",
		"any printable rune        start/extend the fzf filter",
		"Backspace                 shrink the filter",
		"Enter                     activate selected (jump / SwitchProfile / first card action)",
		"Tab                       cycle active profile",
		"n                         new project prompt",
		"d                         unassign slot / dismiss card",
		"a                         archive selected project",
		"u                         unarchive prompt",
		"r                         remove window prompt",
		"t                         carry over card to TUI detail",
		"Ctrl+L                    dismiss all cards (with confirmation)",
		"?                         this help (any key closes)",
		"Esc                       clear filter → close modal → hide cockpit",
		"Ctrl+C                    hide cockpit",
	}
	return strings.Join(lines, "\n")
}

// highlight reverses the first match of filter's first token in the
// label, so the user sees why a row matched.
func highlight(label, filter string) string {
	if filter == "" {
		return label
	}
	first := strings.Fields(filter)
	if len(first) == 0 {
		return label
	}
	tok := first[0]
	idx := strings.Index(strings.ToLower(label), strings.ToLower(tok))
	if idx < 0 {
		return label
	}
	return label[:idx] + styleHi.Render(label[idx:idx+len(tok)]) + label[idx+len(tok):]
}

// isDestructiveAction marks close/remove/dismiss keys as destructive.
func isDestructiveAction(a w.CardAction) bool {
	switch a.Key {
	case "c", "k":
		return true
	}
	label := strings.ToLower(a.Label)
	for _, hit := range []string{"close", "remove", "delete", "dismiss", "purge"} {
		if strings.Contains(label, hit) {
			return true
		}
	}
	return false
}
