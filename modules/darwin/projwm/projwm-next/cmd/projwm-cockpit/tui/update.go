package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// Update is the tea.Model.Update entry. Routing order (most-specific
// first):
//
//  1. Window resize / snapshot / subscription / tick / intent reply →
//     non-key messages, handled and re-armed if needed.
//  2. Prompt active → forward to handlePromptKey; prompts consume Esc
//     / Enter / runes; unhandled keys fall through.
//  3. Help open → any key closes it.
//  4. Normal idle dispatch: action keys, filter keys, navigation.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		return m, nil

	case snapshotMsg:
		if msg.Err != nil {
			m.status = "snapshot error: " + msg.Err.Error()
		} else {
			m.snap = msg.Snap
			m.rebuildItems()
		}
		// Re-arm periodic refresh — note the tickMsg case below also
		// re-fetches the snapshot, so this is a one-shot reaction.
		return m, nil

	case subscriptionMsg:
		if msg.Err == nil {
			// K1.5: card-added forces cockpit visible + mode=proposal +
			// opens the full-screen card modal (ユーザ要望 2026-05-18).
			// v2.9 §9.4: if the user is idle (no prompt / filter / modal
			// already open), auto-hop to the Cards tab so the new card
			// is front and center. If they're busy, don't disrupt — the
			// `Cards (N)` tab counter shows the new count and they can
			// switch when ready.
			if msg.Push.Kind == "card-added" {
				m.uiMode = ModeProposal
				m.cardModalActive = true
				m.cardModalCursor = 0
				idle := m.prompt == promptNone && m.filter.Value() == "" && m.activeTab != TabCards
				if idle {
					m.previousTab = m.activeTab
					m.activeTab = TabCards
				}
				cmds := []tea.Cmd{
					setVisibilityCmd(m.cfg.Client, w.CockpitShown),
					loadSnapshotCmd(m.cfg.Client, m.cfg.StoreDir, m.cfg.ManifestPath),
				}
				if m.cfg.SubscribeCh != nil {
					cmds = append(cmds, listenSubscribeCmd(m.cfg.SubscribeCh))
				}
				return m, tea.Batch(cmds...)
			}
			// Any other push: refresh the snapshot and listen again.
			cmds := []tea.Cmd{
				loadSnapshotCmd(m.cfg.Client, m.cfg.StoreDir, m.cfg.ManifestPath),
			}
			if m.cfg.SubscribeCh != nil {
				cmds = append(cmds, listenSubscribeCmd(m.cfg.SubscribeCh))
			}
			return m, tea.Batch(cmds...)
		}
		// On error, sit out; main goroutine will reconnect and re-fill
		// the channel.
		return m, nil

	case tickMsg:
		// Periodic refresh: re-fetch snapshot + re-arm tick. The View
		// recomputes relative times from snap.ActiveCards.CreatedAt so
		// no further state mutation is needed here.
		return m, tea.Batch(
			loadSnapshotCmd(m.cfg.Client, m.cfg.StoreDir, m.cfg.ManifestPath),
			tickEveryCmd(refreshDuration(m.cfg.RefreshInterval)),
		)

	case intentSubmittedMsg:
		if msg.Err != nil {
			m.status = msg.Kind + " rejected: " + msg.Err.Error()
		} else {
			m.status = msg.Kind + " ok"
		}
		// After an intent the world changed; refresh.
		return m, loadSnapshotCmd(m.cfg.Client, m.cfg.StoreDir, m.cfg.ManifestPath)

	case quitMsg:
		return m, tea.Quit

	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

// updateKey is the keyboard router. Esc-hierarchy (G8) is encoded here:
//
//	prompt active   → prompt absorbs Esc (cancel prompt)
//	help open       → Esc closes help
//	filter non-empty→ Esc clears filter
//	otherwise       → Esc hides cockpit (= Quit)
func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// 0a) Ctrl-P opens the palette from any context except when already
	// open (which would toggle off — handled inside paletteHandleKey).
	if msg.String() == "ctrl+p" && !m.paletteActive {
		m.paletteOpen()
		return m, nil
	}
	// 0b) Palette overlay absorbs everything when open (§9.8).
	if m.paletteActive {
		newM, cmd := m.paletteHandleKey(msg.String())
		return newM, cmd
	}
	// 0c) Wizard overlay absorbs all keys next (§9.7).
	if m.wizardActive {
		newM, cmd := m.wizardHandleKey(msg.String())
		return newM, cmd
	}

	// 1) Profile picker overlay.
	if m.profilePickerActive {
		return m.updateProfilePickerKey(msg)
	}

	// 2) Prompt absorbs everything next.
	if m.prompt != promptNone {
		return m.updatePromptKey(msg)
	}

	// 2) Help open: any key closes it.
	if m.showHelp {
		m.showHelp = false
		return m, nil
	}

	// 3) v2.9 §9.4 tab switching — MUST run before the card modal handler
	//    so 1-5 / Shift+]/[ / o work even when the modal is open. Filter
	//    text input is respected: when the filter has content or focus,
	//    digits go to the filter (so the user can still type "1" or
	//    search by digit).
	if m.filter.Value() == "" && !m.filterFocused && m.prompt == promptNone {
		switch msg.String() {
		case "1":
			m = m.switchTab(TabSlots)
			return m, nil
		case "2":
			m = m.switchTab(TabCards)
			return m, nil
		case "3":
			m = m.switchTab(TabArchived)
			return m, nil
		case "4":
			m = m.switchTab(TabProfiles)
			return m, nil
		case "5":
			m = m.switchTab(TabTrace)
			return m, nil
		case "shift+]", "tab":
			i := tabIndex(m.activeTab)
			m = m.switchTab(tabsOrder[(i+1)%len(tabsOrder)])
			return m, nil
		case "shift+[", "shift+tab":
			i := tabIndex(m.activeTab)
			m = m.switchTab(tabsOrder[(i-1+len(tabsOrder))%len(tabsOrder)])
			return m, nil
		case "o":
			m = m.switchTab(m.previousTab)
			return m, nil
		case ";":
			m.profilePickerActive = true
			m.profilePickerCursor = 0
			return m, nil
		}
	}

	// 4) Card modal: absorbs nav (←/→), close (q/Esc), and forwards
	//    action keys (Enter/c/t/letter) to the focused card. Realises
	//    requirements §10 + ユーザ提案 2026-05-18 modal-popup UX.
	if m.cardModalActive {
		return m.updateCardModalKey(msg)
	}

	// 3) Global commands.
	switch msg.String() {
	case "ctrl+c":
		// Ctrl+C: hide cockpit + quit. (§9.5)
		return m.hideAndQuit()
	case "esc":
		// G8 / §9.5 Esc-hierarchy: trace detail → filter → cockpit hide.
		if m.traceDetailActive {
			m.traceDetailActive = false
			m.traceDetail = ""
			m.uiMode = ModeIdle
			return m, nil
		}
		if m.filter.Value() != "" {
			m.filter.SetValue("")
			m.filterFocused = false
			m.rebuildItems()
			return m, nil
		}
		return m.hideAndQuit()
	case "up", "ctrl+k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "ctrl+j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
		return m, nil
	case "enter":
		return m.activateSelected()
	case "ctrl+l":
		// §9.5 Ctrl+L: open the dismiss-all-cards confirmation prompt.
		m.beginPrompt(promptConfirmClear, listItem{})
		return m, nil
	case "backspace":
		// Backspace shrinks the filter (only when filter has content).
		if v := m.filter.Value(); v != "" {
			m.filter.SetValue(v[:len(v)-1])
			if m.filter.Value() == "" {
				m.filterFocused = false
			}
			m.rebuildItems()
		}
		return m, nil
	}

	// 4) Action runes vs filter runes (G3).
	//
	// Rule: when the filter is empty AND no filterFocused state, action
	// runes (n/d/a/u/r/t/?) trigger their action. Any other printable
	// rune starts the filter and the rune flows into it. Once the
	// filter has content, action runes also extend the filter — but
	// that case is unreachable through this branch because action
	// runes also handle backspace via the dedicated path above; once
	// the filter is non-empty filter-focused becomes true.
	str := msg.String()
	if len(str) == 1 && m.filter.Value() == "" && !m.filterFocused {
		r := rune(str[0])
		if m.keys.ActionRunes()[r] {
			return m.actionByRune(r)
		}
	}

	// 5) Filter consumes the rune (G3 fzf path).
	if len(msg.Runes) > 0 {
		v := m.filter.Value() + string(msg.Runes)
		m.filter.SetValue(v)
		m.filterFocused = true
		m.rebuildItems()
	}
	return m, nil
}

// updateCardModalKey handles keys while the full-screen card modal is
// active. Realises requirements §10 + ユーザ提案 2026-05-18 modal popup.
//
// Key contract:
//
//	←  /  h     → previous card (wraps)
//	→  /  l     → next card (wraps)
//	q  / Esc    → close modal (returns to list view)
//	Enter / c / t / 英字 → forward to activateCardAction
//	Ctrl+C      → hide cockpit
func (m Model) updateCardModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If a prompt is active inside the modal (project / kind input),
	// forward all keys to the prompt handler so the user can type into
	// the input. Bug 2026-05-18: previously the modal absorbed Enter
	// before the prompt handler could see it, producing "project
	// required" with no visible cause.
	if m.prompt != promptNone {
		return m.updatePromptKey(msg)
	}
	cards := sortedActiveCards(m.snap.ActiveCards)
	if len(cards) == 0 {
		// v2.9 §9.4: no cards left — close the modal and return to the
		// previous tab (typically Slots). Without this the user sits on
		// an empty Cards tab and must press 1 manually.
		m.cardModalActive = false
		m.uiMode = ModeIdle
		if m.activeTab == TabCards && m.previousTab != TabCards {
			m.activeTab = m.previousTab
		}
		return m, nil
	}
	// Clamp cursor.
	if m.cardModalCursor < 0 {
		m.cardModalCursor = 0
	}
	if m.cardModalCursor >= len(cards) {
		m.cardModalCursor = len(cards) - 1
	}
	focused := cards[m.cardModalCursor]
	switch msg.String() {
	case "ctrl+c":
		return m.hideAndQuit()
	case "q", "esc":
		// Close modal but keep cockpit visible (user can reopen via card list).
		m.cardModalActive = false
		m.uiMode = ModeIdle
		return m, nil
	case "left", "h":
		m.cardModalCursor = (m.cardModalCursor - 1 + len(cards)) % len(cards)
		return m, nil
	case "right", "l":
		m.cardModalCursor = (m.cardModalCursor + 1) % len(cards)
		return m, nil
	case "t":
		// G4 / §10.2 — carry-over detail. Open the carry-over prompt
		// for the focused card. Must be a dedicated branch (not via
		// activateCardAction) because the card-type-specific dispatcher
		// would otherwise treat `t` as just "another letter".
		m.uiMode = ModeManagement
		m.beginPrompt(promptCarryOver, listItem{
			Kind:        itemCard,
			CardID:      focused.ID,
			CardType:    focused.Type,
			CardActions: focused.Actions,
			CardContext: focused.Context,
		})
		m.carryCardID = focused.ID
		return m, nil
	}
	// Build a virtual listItem for activateCardAction so the existing
	// action dispatcher can be reused.
	it := listItem{
		Kind:        itemCard,
		CardID:      focused.ID,
		CardType:    focused.Type,
		CardActions: focused.Actions,
		CardContext: focused.Context,
	}
	// "enter" forwards as "Enter" (canonical case used by activateCardAction).
	key := msg.String()
	if key == "enter" {
		key = "Enter"
	}
	return m.activateCardAction(it, key)
}

// actionByRune dispatches single-char action keys per §9.5.
// actionByRune dispatches single-char action keys (§9.5) to the active
// tab's handler. `?` is universal; everything else is tab-aware so the
// same letter (`n`, `d`, …) means the right thing in each context.
//
// v2.9 §9.4: bug 2026-05-19 fix — previously every tab ran the Slots
// handler and "n on Profiles" still launched the project wizard.
func (m Model) actionByRune(r rune) (tea.Model, tea.Cmd) {
	if r == '?' {
		m.showHelp = true
		return m, nil
	}
	switch m.activeTab {
	case TabProfiles:
		return m.actionProfilesTab(r)
	case TabArchived:
		return m.actionArchivedTab(r)
	case TabTrace:
		return m.actionTraceTab(r)
	}
	return m.actionSlotsTab(r)
}

// actionSlotsTab handles letter keys on the Slots tab. Matches the
// historical behaviour (n new project / d unassign / a archive / u
// unarchive prompt / r remove window / t carry-over).
func (m Model) actionSlotsTab(r rune) (tea.Model, tea.Cmd) {
	switch r {
	case 'n':
		// v2.9 §9.7: full wizard B2 form instead of the legacy single-
		// line prompt. The old promptNewProject path is still wired so
		// palette / scripts can use it, but the user-facing `n` opens
		// the wizard.
		m.wizardOpenNewProject()
		return m, nil
	case 'd':
		return m.unassignOrDismissCard()
	case 'a':
		return m.archiveSelected()
	case 'u':
		// Slots tab: jump to Archived so the user can pick the project
		// to unarchive in its proper context (full archived list).
		m = m.switchTab(TabArchived)
		return m, nil
	case 'r':
		m.beginPrompt(promptRemoveWindow, m.Selected())
		return m, nil
	case 't':
		sel := m.Selected()
		if sel.Kind == itemCard {
			m.uiMode = ModeManagement
			m.beginPrompt(promptCarryOver, sel)
			m.carryCardID = sel.CardID
			return m, nil
		}
		m.uiMode = ModeManagement
		m.status = "t: select a card to carry over"
		return m, nil
	case '+':
		sel := m.Selected()
		if sel.Kind != itemSlot || sel.Project == "" {
			m.status = "+: select a slot with an assigned project to add a window"
			return m, nil
		}
		m.wizardOpenAddWindow(sel.Project)
		return m, nil
	}
	return m, nil
}

// actionProfilesTab: n=new profile / d=delete / r=rename. The Enter row
// (switch profile) is handled by activateSelected.
func (m Model) actionProfilesTab(r rune) (tea.Model, tea.Cmd) {
	sel := m.Selected()
	switch r {
	case 'n':
		m.wizardOpenNewProfile()
		return m, nil
	case 'd':
		if sel.Kind != itemProfile {
			m.status = "d: select a profile to delete"
			return m, nil
		}
		if sel.Profile == m.snap.ActiveProfile {
			m.status = "cannot delete the active profile — switch first"
			return m, nil
		}
		m.beginPrompt(promptDeleteProfile, sel)
		return m, nil
	case 'r':
		if sel.Kind != itemProfile {
			m.status = "r: select a profile to rename"
			return m, nil
		}
		m.beginPrompt(promptRenameProfile, sel)
		return m, nil
	}
	return m, nil
}

// actionArchivedTab: u=unarchive (prompts for target slot), x=purge
// (destructive confirm), Enter handled by activateSelected (also
// unarchive prompt for symmetry).
func (m Model) actionArchivedTab(r rune) (tea.Model, tea.Cmd) {
	sel := m.Selected()
	switch r {
	case 'u':
		if sel.Kind != itemArchive {
			m.status = "u: select an archived project"
			return m, nil
		}
		m.beginPrompt(promptUnarchive, sel)
		return m, nil
	case 'x':
		if sel.Kind != itemArchive {
			m.status = "x: select an archived project to purge"
			return m, nil
		}
		m.beginPrompt(promptPurgeProject, sel)
		return m, nil
	}
	return m, nil
}

// actionTraceTab: r=reload trace list from disk. Enter (open detail) is
// handled by activateSelected so it shares the cursor-target convention
// with the other tabs.
func (m Model) actionTraceTab(r rune) (tea.Model, tea.Cmd) {
	if r == 'r' {
		m.traces = loadTracesFromDisk(m.cfg.StoreDir, 50)
		m.rebuildItems()
		m.status = "trace list reloaded"
		return m, nil
	}
	return m, nil
}

// activateCardAction submits the intent for a card's Enter / letter
// action per §10.3.
func (m Model) activateCardAction(it listItem, key string) (tea.Model, tea.Cmd) {
	_ = key // reserved for explicit letter actions
	switch it.CardType {
	case w.CardTypeMoved, w.CardTypeClosed, w.CardTypeInvariant, w.CardTypeManifest, w.CardTypeReplan:
		m.uiMode = ModeManagement
		return m, submitIntentCmd(m.cfg.Client, intent.DismissCard{CardID: intent.CardID(it.CardID)})
	case w.CardTypeNew, w.CardTypeOrphan:
		liveID := ""
		bundleID := ""
		kindHint := ""
		if it.CardContext != nil {
			liveID = it.CardContext["live"]
			bundleID = it.CardContext["bundleID"]
			kindHint = it.CardContext["kind"]
		}
		m.uiMode = ModeManagement
		if bundleID == "com.mitchellh.ghostty" {
			m.beginPromptWithLive(promptRespawnGhostty, liveID, kindHint, it)
		} else {
			m.beginPromptWithLive(promptAdoptOrphan, liveID, kindHint, it)
		}
		return m, nil
	}
	// Unknown card type — default to dismiss.
	return m, submitIntentCmd(m.cfg.Client, intent.DismissCard{CardID: intent.CardID(it.CardID)})
}

// cycleActiveProfile rotates to the next profile in sorted order.
func (m Model) cycleActiveProfile() tea.Cmd {
	cur := m.snap.ActiveProfile
	ids := make([]w.ProfileID, 0, len(m.snap.Profiles))
	for id := range m.snap.Profiles {
		ids = append(ids, id)
	}
	if len(ids) < 2 {
		return nil
	}
	// Sort deterministically.
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	for i, id := range ids {
		if id == cur {
			next := ids[(i+1)%len(ids)]
			return submitIntentCmd(m.cfg.Client, intent.SwitchProfile{To: next})
		}
	}
	return nil
}

// jumpToSlot shells out to omniwmctl. (Implementation lives in this
// package so the tui is the single source of truth for §9.5 Enter row.)
func (m Model) jumpToSlot(slot w.SlotID) tea.Cmd {
	var ws w.WorkspaceID
	for _, s := range m.snap.Slots {
		if s.ID == slot {
			ws = s.Workspace
			break
		}
	}
	if ws == "" {
		return nil
	}
	return func() tea.Msg {
		cmd := exec.Command("omniwmctl", "workspace", "focus-name", string(ws))
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			return statusMsg{Text: "jump failed: " + err.Error()}
		}
		// Mode 2 (Navigation) auto-hide (§8.5): the actual hide is
		// performed by the daemon's observed→desired Visibility sync
		// (v2.7 §8.3.1) — `omniwmctl workspace focus-name` switches the
		// cockpit display away from CP1, the reducer observes the drift
		// and flips Visibility=Hidden. The TUI itself stays alive so
		// re-summoning the cockpit (space+f) finds it already running.
		// Bug 2026-05-18: returning quitMsg{} here used to terminate
		// the cockpit binary and tear down the ghostty window, breaking
		// the §8.1 "1-instance persistent" requirement.
		return statusMsg{Text: "jumped to " + string(ws)}
	}
}

// unassignOrDismissCard handles `d` polymorphically.
func (m Model) unassignOrDismissCard() (tea.Model, tea.Cmd) {
	sel := m.Selected()
	switch sel.Kind {
	case itemSlot:
		return m, submitIntentCmd(m.cfg.Client, intent.UnassignSlot{Slot: sel.Slot})
	case itemCard:
		return m, submitIntentCmd(m.cfg.Client, intent.DismissCard{CardID: intent.CardID(sel.CardID)})
	}
	m.status = "d: select a slot to unassign or a card to dismiss"
	return m, nil
}

// archiveSelected handles `a`.
func (m Model) archiveSelected() (tea.Model, tea.Cmd) {
	sel := m.Selected()
	if sel.Project == "" {
		m.status = "a: select a project to archive"
		return m, nil
	}
	return m, submitIntentCmd(m.cfg.Client, intent.ArchiveProject{Project: sel.Project})
}

// updateProfilePickerKey handles keys while the v2.9 §9.5 profile
// picker overlay is open. ↑↓ / k/j to move, Enter to switch to the
// selected profile, Esc / ; to cancel.
func (m Model) updateProfilePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	profiles := sortedProfiles(m.snap.Profiles)
	if len(profiles) == 0 {
		m.profilePickerActive = false
		return m, nil
	}
	if m.profilePickerCursor < 0 {
		m.profilePickerCursor = 0
	}
	if m.profilePickerCursor >= len(profiles) {
		m.profilePickerCursor = len(profiles) - 1
	}
	switch msg.String() {
	case "esc", ";":
		m.profilePickerActive = false
		return m, nil
	case "up", "k":
		m.profilePickerCursor = (m.profilePickerCursor - 1 + len(profiles)) % len(profiles)
		return m, nil
	case "down", "j":
		m.profilePickerCursor = (m.profilePickerCursor + 1) % len(profiles)
		return m, nil
	case "enter":
		target := profiles[m.profilePickerCursor]
		m.profilePickerActive = false
		return m, submitIntentCmd(m.cfg.Client, intent.SwitchProfile{To: target})
	}
	return m, nil
}

// sortedProfiles returns profile IDs in deterministic order (active
// first, then alphabetical) for the picker overlay.
func sortedProfiles(profs map[w.ProfileID]w.DesiredProfile) []w.ProfileID {
	ids := make([]w.ProfileID, 0, len(profs))
	for id := range profs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// switchTab is the canonical tab-change primitive. Stores the previous
// tab so `o` can toggle back. v2.9 §9.4.
func (m Model) switchTab(t tabKind) Model {
	if t == m.activeTab {
		return m
	}
	m.previousTab = m.activeTab
	m.activeTab = t
	// When entering Cards tab, open the modal automatically so the
	// per-card detail view is shown.
	if t == TabCards {
		m.cardModalActive = true
		if m.cardModalCursor < 0 {
			m.cardModalCursor = 0
		}
	} else {
		// Leaving Cards collapses the modal so the underlying tab is
		// visible at full width.
		m.cardModalActive = false
	}
	// Leaving the Trace tab also drops any open detail overlay so the
	// next time the user comes back the list view is on top.
	if t != TabTrace {
		m.traceDetailActive = false
	}
	// Entering the Trace tab refreshes the on-disk trace list so it
	// stays roughly current without burning daemon queries.
	if t == TabTrace {
		m.traces = loadTracesFromDisk(m.cfg.StoreDir, 50)
	}
	return m
}

// hideAndQuit submits SetCockpitVisibility{Hidden}; the cockpit window
// and TUI process stay alive (§8.1 1-instance persistent requirement).
// The "Quit" in the name is historical — the actual hide is a display-
// only operation performed by the daemon's planner (§8.3 switching the
// projwm-managed monitor's active workspace away from CP1).
//
// Bug 2026-05-18: previous impl also issued tea.Quit, which tore down
// the cockpit ghostty and forced a SpawnCockpit on every show. Now the
// TUI persists across hide/show cycles and re-summon (space+f) finds
// the existing process.
func (m Model) hideAndQuit() (tea.Model, tea.Cmd) {
	return m, setVisibilityCmd(m.cfg.Client, w.CockpitHidden)
}

// beginPrompt opens a modal prompt with item as operand.
func (m *Model) beginPrompt(kind promptKind, item listItem) {
	m.prompt = kind
	m.promptTarget = item
	m.promptStep = 0
	m.promptScratch = map[string]string{}
	m.promptInput.SetValue("")
	m.promptInput.Focus()
	m.promptInput.Placeholder = promptPlaceholder(kind, 0)
}

// beginPromptWithLive opens a two-step prompt for a [NEW] card, stashing
// the orphan's LiveID + kind hint in scratch.
func (m *Model) beginPromptWithLive(kind promptKind, liveID, kindHint string, src listItem) {
	m.beginPrompt(kind, src)
	m.promptScratch["live"] = liveID
	m.promptScratch["hint"] = kindHint
}

// cancelPrompt clears prompt state.
func (m *Model) cancelPrompt() {
	m.prompt = promptNone
	m.promptStep = 0
	m.promptScratch = nil
	m.promptInput.SetValue("")
	m.promptInput.Blur()
}

// updatePromptKey processes one key while a modal prompt is active.
func (m Model) updatePromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.status = "prompt cancelled"
		m.cancelPrompt()
		return m, nil
	case "enter":
		return m.submitPrompt()
	case "ctrl+c":
		// Same as outside prompts: hide + quit.
		return m.hideAndQuit()
	}
	// Forward to textinput so it handles backspace/runes/cursor.
	var cmd tea.Cmd
	m.promptInput, cmd = m.promptInput.Update(msg)
	return m, cmd
}

// submitPrompt acts on promptInput.Value() based on prompt kind.
func (m Model) submitPrompt() (tea.Model, tea.Cmd) {
	buf := strings.TrimSpace(m.promptInput.Value())
	switch m.prompt {
	case promptNewProject:
		if buf == "" {
			m.status = "new-project: ID required"
			return m, nil
		}
		home, _ := os.UserHomeDir()
		path := home + "/dev/" + buf
		cmd := submitIntentCmd(m.cfg.Client, intent.CreateProject{ID: w.ProjectID(buf), Path: path})
		m.cancelPrompt()
		return m, cmd

	case promptUnarchive:
		// SSOT §4.5: unarchive only clears Archived; slot assignment is
		// a separate operation (use the assign prompt afterwards). The
		// `buf` input is ignored — kept here only for prompt symmetry
		// until the cockpit is rewritten for bubbletea (Phase 6).
		_ = buf
		if m.promptTarget.Project == "" {
			m.status = "unarchive: select an archived project first"
			m.cancelPrompt()
			return m, nil
		}
		// The executor cd's into the project root when spawning tmux
		// sessions; recreate the dir first so the executor has somewhere
		// to land if a later assign triggers a respawn.
		if proj, ok := m.snap.Projects[m.promptTarget.Project]; ok && proj.Root != "" {
			if err := os.MkdirAll(proj.Root, 0o755); err != nil {
				m.status = "unarchive: mkdir " + proj.Root + ": " + err.Error()
				return m, nil
			}
		}
		cmd := submitIntentCmd(m.cfg.Client, intent.UnarchiveProject{
			Project: m.promptTarget.Project,
		})
		m.cancelPrompt()
		return m, cmd

	case promptRemoveWindow:
		if m.promptTarget.Project == "" {
			m.status = "remove-window: select a slot with a project first"
			m.cancelPrompt()
			return m, nil
		}
		kind, idx, err := parseWindowSpec(buf)
		if err != nil {
			m.status = "remove-window: " + err.Error()
			return m, nil
		}
		cmd := submitIntentCmd(m.cfg.Client, intent.RemoveWindow{
			Project: m.promptTarget.Project,
			WindowID: w.DesiredWindowID{
				Project: m.promptTarget.Project,
				Kind:    kind,
				Index:   idx,
			},
		})
		m.cancelPrompt()
		return m, cmd

	case promptConfirmClear:
		lower := strings.ToLower(buf)
		if lower == "y" || lower == "yes" {
			cmd := submitIntentCmd(m.cfg.Client, intent.DismissAllCards{})
			m.cancelPrompt()
			return m, cmd
		}
		m.status = "cancelled"
		m.cancelPrompt()
		return m, nil

	case promptAdoptOrphan:
		return m.submitTwoStepOrphan(buf, false)

	case promptRespawnGhostty:
		return m.submitTwoStepOrphan(buf, true)

	case promptCarryOver:
		// G4: detail view modal. Enter dismisses the modal but keeps
		// the card in the list (uiMode stays Management).
		m.cancelPrompt()
		m.status = "carry-over closed"
		return m, nil

	case promptNewProfile:
		if buf == "" {
			m.status = "profile id required"
			return m, nil
		}
		cmd := submitIntentCmd(m.cfg.Client, intent.CreateProfile{
			ID:             w.ProfileID(buf),
			Description:    "",
			InactivePolicy: w.InactivePolicyRemove,
		})
		m.cancelPrompt()
		return m, cmd

	case promptDeleteProfile:
		// promptTarget.Profile is the target; buf must equal the ID for
		// a destructive-confirm safety.
		target := m.promptTarget.Profile
		if buf != string(target) {
			m.status = "delete: type the profile id exactly to confirm"
			return m, nil
		}
		cmd := submitIntentCmd(m.cfg.Client, intent.DeleteProfile{ID: target})
		m.cancelPrompt()
		return m, cmd

	case promptRenameProfile:
		if buf == "" {
			m.status = "new profile id required"
			return m, nil
		}
		old := m.promptTarget.Profile
		cmd := submitIntentCmd(m.cfg.Client, intent.RenameProfile{Old: old, New: w.ProfileID(buf)})
		m.cancelPrompt()
		return m, cmd

	case promptPurgeProject:
		target := m.promptTarget.Project
		if buf != string(target) {
			m.status = "purge: type the project id exactly to confirm"
			return m, nil
		}
		cmd := submitIntentCmd(m.cfg.Client, intent.DeleteProject{ID: target, Purge: true})
		m.cancelPrompt()
		return m, cmd
	}
	return m, nil
}

// submitTwoStepOrphan handles step 0 (collect project) → step 1
// (collect kind) → submit. ghostty=true selects RespawnOrphanGhostty.
func (m Model) submitTwoStepOrphan(buf string, ghostty bool) (tea.Model, tea.Cmd) {
	switch m.promptStep {
	case 0:
		if buf == "" {
			m.status = "project required"
			return m, nil
		}
		m.promptScratch["project"] = buf
		m.promptStep = 1
		m.promptInput.SetValue("")
		m.promptInput.Placeholder = promptPlaceholder(m.prompt, 1)
		return m, nil
	case 1:
		if buf == "" {
			m.status = "kind required"
			return m, nil
		}
		liveID := w.LiveWindowID(m.promptScratch["live"])
		pid := w.ProjectID(m.promptScratch["project"])
		kind := w.WindowKind(buf)
		// SSOT §4.3 has only three orphan card actions ([Enter] adopt,
		// [c] close, [t] open TUI). RespawnOrphanGhostty is not in the
		// SSOT surface — both ghostty and non-ghostty orphans take the
		// AdoptOrphanWindow path here. ghostty-specific "kill old + spawn
		// new under proper tmux session" is the executor's responsibility
		// after adopt resolves to a new DesiredWindowID.
		_ = ghostty
		in := intent.AdoptOrphanWindow{LiveID: liveID, AsProject: pid, AsWindowKind: kind}
		cmd := submitIntentCmd(m.cfg.Client, in)
		m.cancelPrompt()
		return m, cmd
	}
	return m, nil
}

// activateSelected branches on the active tab and the cursor item Kind
// to produce the canonical "Enter" action for that context.
// v2.9 §9.4 — Phase γ.0 fix: previously Slots-style handler ran on every
// tab and "Enter on Profiles" silently did nothing for the active profile.
func (m Model) activateSelected() (tea.Model, tea.Cmd) {
	sel := m.Selected()
	switch m.activeTab {
	case TabProfiles:
		if sel.Kind != itemProfile {
			return m, nil
		}
		if sel.Profile == m.snap.ActiveProfile {
			m.status = "already on profile " + string(sel.Profile)
			return m, nil
		}
		m.uiMode = ModeNavigation
		return m, submitIntentCmd(m.cfg.Client, intent.SwitchProfile{To: sel.Profile})

	case TabArchived:
		if sel.Kind != itemArchive {
			return m, nil
		}
		m.uiMode = ModeManagement
		m.beginPrompt(promptUnarchive, sel)
		return m, nil

	case TabTrace:
		if sel.Kind != itemTrace || sel.TracePath == "" {
			return m, nil
		}
		data, err := os.ReadFile(sel.TracePath)
		if err != nil {
			m.status = "trace: read failed: " + err.Error()
			return m, nil
		}
		// Pretty-print the JSON so the modal stays readable. Fall back to
		// the raw payload if re-formatting fails.
		var generic any
		if err := json.Unmarshal(data, &generic); err == nil {
			pretty, perr := json.MarshalIndent(generic, "", "  ")
			if perr == nil {
				data = pretty
			}
		}
		m.traceDetail = string(data)
		m.traceDetailActive = true
		m.uiMode = ModeManagement
		return m, nil

	default:
		// Slots / Cards (when items list is non-empty, e.g. cards listed
		// inline) — historical Slots behaviour preserved.
		switch sel.Kind {
		case itemSlot, itemViewer:
			if sel.Slot == "" {
				return m, nil
			}
			m.uiMode = ModeNavigation
			return m, m.jumpToSlot(sel.Slot)
		case itemProfile:
			m.uiMode = ModeNavigation
			return m, submitIntentCmd(m.cfg.Client, intent.SwitchProfile{To: sel.Profile})
		case itemParked:
			m.uiMode = ModeManagement
			m.status = "parked project: switch to Archived tab (3) and press u"
			return m, nil
		case itemArchive:
			m.uiMode = ModeManagement
			m.beginPrompt(promptUnarchive, sel)
			return m, nil
		case itemCard:
			return m.activateCardAction(sel, "Enter")
		}
		return m, nil
	}
}

// promptPlaceholder gives the textinput a human-friendly hint.
func promptPlaceholder(k promptKind, step int) string {
	switch k {
	case promptNewProject:
		return "project ID (path defaults to $HOME/dev/<id>)"
	case promptUnarchive:
		return "slot id (e.g. Q)"
	case promptRemoveWindow:
		return "KIND-N (e.g. ai-1)"
	case promptConfirmClear:
		return "y/n — confirm dismiss all cards"
	case promptAdoptOrphan:
		if step == 0 {
			return "project to adopt as"
		}
		return "kind (ai/shell/editor/browser)"
	case promptRespawnGhostty:
		if step == 0 {
			return "project to respawn as"
		}
		return "kind (ai/shell)"
	case promptCarryOver:
		return "press Enter / Esc to close detail"
	case promptNewProfile:
		return "new profile id (e.g. work)"
	case promptDeleteProfile:
		return "type the profile id to confirm delete"
	case promptRenameProfile:
		return "new profile id"
	case promptPurgeProject:
		return "type the project id to confirm purge"
	}
	return ""
}

// parseWindowSpec accepts "ai-1", "shell-3", "editor-1", "browser-2".
func parseWindowSpec(spec string) (w.WindowKind, int, error) {
	dash := strings.LastIndex(spec, "-")
	if dash <= 0 {
		return "", 0, fmt.Errorf("window spec must be KIND-N (e.g. ai-1)")
	}
	kindStr := spec[:dash]
	var n int
	if _, err := fmt.Sscanf(spec[dash+1:], "%d", &n); err != nil || n < 1 {
		return "", 0, fmt.Errorf("window spec must have positive numeric suffix")
	}
	switch kindStr {
	case "ai":
		return w.WindowAI, n, nil
	case "shell":
		return w.WindowShell, n, nil
	case "editor":
		return w.WindowEditor, n, nil
	case "browser":
		return w.WindowBrowser, n, nil
	}
	return "", 0, fmt.Errorf("unknown window kind %s", kindStr)
}

// relativeTime renders a CreatedAt (unix nano) as a human relative
// string. Requirements §10.1 / G1: each card must surface its
// creation time. We render both relative + clock so the user sees
// "12:34:56 · 1m ago" even on long-running sessions.
func relativeTime(unixNano int64, now time.Time) string {
	if unixNano <= 0 {
		return "—"
	}
	t := time.Unix(0, unixNano)
	delta := now.Sub(t)
	var rel string
	switch {
	case delta < 5*time.Second:
		rel = "just now"
	case delta < time.Minute:
		rel = fmt.Sprintf("%ds ago", int(delta.Seconds()))
	case delta < time.Hour:
		rel = fmt.Sprintf("%dm ago", int(delta.Minutes()))
	case delta < 24*time.Hour:
		rel = fmt.Sprintf("%dh ago", int(delta.Hours()))
	default:
		rel = t.Format("Jan 2")
	}
	return t.Format("15:04:05") + " · " + rel
}
