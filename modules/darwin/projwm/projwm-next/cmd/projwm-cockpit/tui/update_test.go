package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yuu-th/projwm-next/internal/ipc"
)

// helper: send a key string ("n", "esc", "enter", "ctrl+c", "?", letters)
// through Update, return the new model.
func sendKey(t *testing.T, m Model, key string) Model {
	t.Helper()
	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "ctrl+c":
		msg = tea.KeyMsg{Type: tea.KeyCtrlC}
	case "ctrl+l":
		msg = tea.KeyMsg{Type: tea.KeyCtrlL}
	case "ctrl+p":
		msg = tea.KeyMsg{Type: tea.KeyCtrlP}
	case "shift+tab":
		msg = tea.KeyMsg{Type: tea.KeyShiftTab}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "backspace":
		msg = tea.KeyMsg{Type: tea.KeyBackspace}
	default:
		// Single-rune key.
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, _ := m.Update(msg)
	return updated.(Model)
}

// G3 — letter action vs filter: from idle, "n" opens the new-project
// wizard (v2.9 §9.7 replaced the legacy single-line prompt); "z" (not
// an action) starts the filter.
func TestKey_LetterActionVsFilter(t *testing.T) {
	m := newTestModel(t)
	// n triggers new-project wizard (action rune).
	m2 := sendKey(t, m, "n")
	if !m2.wizardActive || m2.wizardKind != WizardNewProject {
		t.Errorf("n did not open new-project wizard: active=%v kind=%s", m2.wizardActive, m2.wizardKind)
	}

	// z is not an action key → starts the filter. (Was `x`, now `x`
	// is the Archived-tab purge action so we use a guaranteed-letter
	// rune that no tab claims.)
	m3 := newTestModel(t)
	m3 = sendKey(t, m3, "z")
	if m3.FilterValue() != "z" {
		t.Errorf("z did not start filter, got %q", m3.FilterValue())
	}
}

// §9.5 — n on Slots opens the new-project wizard (v2.9 §9.7).
func TestKey_N_OpensNewProjectPrompt(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(t, m, "n")
	if !m.wizardActive || m.wizardKind != WizardNewProject {
		t.Errorf("expected new-project wizard, got active=%v kind=%s", m.wizardActive, m.wizardKind)
	}
}

// v2.9 §9.4 — n on Profiles tab opens new-PROFILE wizard (not project).
// Bug 2026-05-19 reproduction: pre-γ.0 n was tab-blind so it always
// opened the project wizard.
func TestKey_N_OnProfilesOpensNewProfilePrompt(t *testing.T) {
	m := newTestModel(t)
	m = m.switchTab(TabProfiles)
	m.rebuildItems()
	m = sendKey(t, m, "n")
	if !m.wizardActive || m.wizardKind != WizardNewProfile {
		t.Errorf("Profiles tab `n` did not open new-profile wizard: active=%v kind=%s", m.wizardActive, m.wizardKind)
	}
}

// v2.9 §9.4 — d on Profiles deletes the cursor profile (active profile is
// protected with a status message).
func TestKey_D_OnProfilesOpensDeletePrompt(t *testing.T) {
	m := newTestModel(t)
	m = m.switchTab(TabProfiles)
	m.rebuildItems()
	// Find a non-active profile to put cursor on.
	target := -1
	for i, it := range m.items {
		if it.Kind == itemProfile && it.Profile != m.snap.ActiveProfile {
			target = i
			break
		}
	}
	if target < 0 {
		t.Skip("fixture has only one profile; skipping delete test")
	}
	m.cursor = target
	m = sendKey(t, m, "d")
	if m.Prompt() != promptDeleteProfile {
		t.Errorf("d on non-active profile did not open delete prompt: %s", m.Prompt())
	}
}

// v2.9 §9.4 — r on Profiles opens rename prompt.
func TestKey_R_OnProfilesOpensRenamePrompt(t *testing.T) {
	m := newTestModel(t)
	m = m.switchTab(TabProfiles)
	m.rebuildItems()
	if len(m.items) == 0 {
		t.Skip("no profiles in fixture")
	}
	m.cursor = 0
	m = sendKey(t, m, "r")
	if m.Prompt() != promptRenameProfile {
		t.Errorf("r on profile did not open rename prompt: %s", m.Prompt())
	}
}

// v2.9 §9.4 — Enter on Profiles tab submits SwitchProfile for the
// cursor's profile (when it's not the active one).
func TestKey_Enter_OnProfilesSwitches(t *testing.T) {
	m := newTestModel(t)
	m = m.switchTab(TabProfiles)
	m.rebuildItems()
	// Find non-active profile under cursor.
	target := -1
	for i, it := range m.items {
		if it.Kind == itemProfile && it.Profile != m.snap.ActiveProfile {
			target = i
			break
		}
	}
	if target < 0 {
		t.Skip("only one profile; switch is no-op")
	}
	m.cursor = target
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil {
		t.Error("Enter on non-active profile should return a SwitchProfile cmd")
	}
	if m.UIMode() != ModeNavigation {
		t.Errorf("uiMode = %s, want Navigation", m.UIMode())
	}
}

// v2.9 §9.4 — u on Archived opens unarchive prompt.
func TestKey_U_OnArchivedOpensUnarchivePrompt(t *testing.T) {
	m := newTestModel(t)
	m = m.switchTab(TabArchived)
	m.rebuildItems()
	if len(m.items) == 0 {
		t.Skip("no archived projects in fixture")
	}
	m.cursor = 0
	m = sendKey(t, m, "u")
	if m.Prompt() != promptUnarchive {
		t.Errorf("u on archived did not open unarchive prompt: %s", m.Prompt())
	}
}

// v2.9 §9.4 — x on Archived opens purge prompt (destructive confirm).
func TestKey_X_OnArchivedOpensPurgePrompt(t *testing.T) {
	m := newTestModel(t)
	m = m.switchTab(TabArchived)
	m.rebuildItems()
	if len(m.items) == 0 {
		t.Skip("no archived projects in fixture")
	}
	m.cursor = 0
	m = sendKey(t, m, "x")
	if m.Prompt() != promptPurgeProject {
		t.Errorf("x on archived did not open purge prompt: %s", m.Prompt())
	}
}

// v2.9 §9.4 — Enter on Archived also opens unarchive prompt (symmetry
// with the recommended action shown in the bottom menu).
func TestKey_Enter_OnArchivedOpensUnarchivePrompt(t *testing.T) {
	m := newTestModel(t)
	m = m.switchTab(TabArchived)
	m.rebuildItems()
	if len(m.items) == 0 {
		t.Skip("no archived projects in fixture")
	}
	m.cursor = 0
	m = sendKey(t, m, "enter")
	if m.Prompt() != promptUnarchive {
		t.Errorf("Enter on archived did not open unarchive prompt: %s", m.Prompt())
	}
}

// v2.9 §9.4 — r on Trace tab reloads the on-disk list (no-op when the
// store dir has no traces, but the call must not panic and must set a
// status message so the user sees something happened).
func TestKey_R_OnTraceReloads(t *testing.T) {
	m := newTestModel(t)
	m = m.switchTab(TabTrace)
	m = sendKey(t, m, "r")
	if m.Status() == "" {
		t.Errorf("r on Trace did not set a status message")
	}
}

// v2.9 §9.4 — Enter on a Trace row with no TracePath is a no-op; with
// a path it opens the detail overlay. Tested via the empty-state row
// (no path → no overlay).
func TestKey_Enter_OnTracePlaceholderDoesNothing(t *testing.T) {
	m := newTestModel(t)
	m = m.switchTab(TabTrace)
	m.rebuildItems()
	if len(m.items) == 0 {
		t.Fatal("Trace tab produced no items (empty-state expected)")
	}
	m.cursor = 0
	m = sendKey(t, m, "enter")
	if m.traceDetailActive {
		t.Errorf("Enter on empty-state row should not open detail overlay")
	}
}

// v2.9 §9.4 — Tab cycles through all 5 tabs back to start.
func TestKey_Tab_CyclesAllFiveTabs(t *testing.T) {
	m := newTestModel(t)
	starts := m.activeTab
	for i := 0; i < 5; i++ {
		m = sendKey(t, m, "tab")
	}
	if m.activeTab != starts {
		t.Errorf("5 Tabs did not return to start: %s != %s", m.activeTab, starts)
	}
}

// §9.5 — Ctrl+L opens the confirm-dismiss-all-cards prompt.
func TestKey_CtrlL_OpensConfirmClear(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(t, m, "ctrl+l")
	if m.Prompt() != promptConfirmClear {
		t.Errorf("expected confirm-clear prompt, got %s", m.Prompt())
	}
}

// §9.5 — ? toggles help screen.
func TestKey_Question_TogglesHelp(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(t, m, "?")
	if !m.showHelp {
		t.Error("? did not open help")
	}
	// Any key closes it.
	m = sendKey(t, m, "x")
	if m.showHelp {
		t.Error("any key did not close help")
	}
}

// G2 / G8 / §9.5 — Esc hierarchy: filter clears first.
func TestKey_Esc_ClearsFilterBeforeQuitting(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(t, m, "x")
	m = sendKey(t, m, "y")
	if m.FilterValue() == "" {
		t.Fatal("expected filter to be populated")
	}
	m = sendKey(t, m, "esc")
	if m.FilterValue() != "" {
		t.Errorf("Esc did not clear filter: %q", m.FilterValue())
	}
	if m.quitting {
		t.Errorf("Esc with non-empty filter unexpectedly quit")
	}
}

// G8 — Esc inside the wizard cancels the wizard, not the cockpit.
// (v2.9 §9.7 replaced the legacy `n` → prompt path with `n` → wizard.)
func TestKey_Esc_CancelsPromptBeforeQuit(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(t, m, "n")
	if !m.wizardActive {
		t.Fatalf("setup: wizardActive = false")
	}
	m = sendKey(t, m, "esc")
	if m.wizardActive {
		t.Errorf("Esc did not cancel wizard")
	}
	if m.quitting {
		t.Errorf("Esc on wizard unexpectedly quit")
	}
}

// v2.9 §9.7 — wizard form: typing into ID auto-fills Path with the
// $HOME/dev/<id> default.
func TestWizard_IDTypingPopulatesPath(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(t, m, "n")
	if !m.wizardActive {
		t.Fatalf("setup: wizardActive = false")
	}
	m = sendKey(t, m, "f")
	m = sendKey(t, m, "o")
	m = sendKey(t, m, "o")
	if m.wizardFields[0].TextValue != "foo" {
		t.Errorf("ID field = %q, want %q", m.wizardFields[0].TextValue, "foo")
	}
	if !strings.HasSuffix(m.wizardFields[1].TextValue, "/dev/foo") {
		t.Errorf("Path field did not derive from ID: %q", m.wizardFields[1].TextValue)
	}
}

// v2.9 §9.7 — wizard form: Tab cycles to next field, Shift+Tab back.
func TestWizard_TabCyclesFields(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(t, m, "n")
	if m.wizardCursor != 0 {
		t.Fatalf("setup: cursor = %d, want 0", m.wizardCursor)
	}
	m = sendKey(t, m, "tab")
	if m.wizardCursor != 1 {
		t.Errorf("Tab did not advance cursor: %d", m.wizardCursor)
	}
	m = sendKey(t, m, "shift+tab")
	if m.wizardCursor != 0 {
		t.Errorf("Shift+Tab did not retreat cursor: %d", m.wizardCursor)
	}
}

// v2.9 §9.7 — wizard form: Enter with empty ID stays open with status
// "ID required".
func TestWizard_SubmitWithoutIDFails(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(t, m, "n")
	m = sendKey(t, m, "enter")
	if !m.wizardActive {
		t.Errorf("empty-ID submit closed the wizard")
	}
	if !strings.Contains(m.Status(), "required") {
		t.Errorf("status did not mention required: %q", m.Status())
	}
}

// v2.9 §9.8 — Ctrl+P opens the command palette, Esc closes it.
func TestPalette_CtrlPOpensAndEscCloses(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(t, m, "ctrl+p")
	if !m.paletteActive {
		t.Fatalf("Ctrl+P did not open palette")
	}
	if len(m.paletteActions) < 5 {
		t.Errorf("palette has only %d actions; expected the built-in set", len(m.paletteActions))
	}
	m = sendKey(t, m, "esc")
	if m.paletteActive {
		t.Errorf("Esc did not close palette")
	}
}

// v2.9 §9.8 — typing into the palette filters the visible actions.
func TestPalette_QueryFilters(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(t, m, "ctrl+p")
	m = sendKey(t, m, "n")
	m = sendKey(t, m, "e")
	m = sendKey(t, m, "w")
	visible := m.paletteVisibleActions()
	if len(visible) == 0 {
		t.Fatal("`new` filter produced no visible actions")
	}
	for _, a := range visible {
		if !strings.Contains(strings.ToLower(a.Label+a.Hint), "new") {
			t.Errorf("palette filter leaked %q (does not match 'new')", a.Label)
		}
	}
}

// v2.9 §9.8 — Enter on `new project` action opens the wizard.
func TestPalette_EnterRunsAction(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(t, m, "ctrl+p")
	for _, c := range "new project" {
		m = sendKey(t, m, string(c))
	}
	visible := m.paletteVisibleActions()
	if len(visible) == 0 {
		t.Fatal("filter produced no visible actions")
	}
	// First match is `new project`; Enter should fire it.
	m = sendKey(t, m, "enter")
	if m.paletteActive {
		t.Errorf("Enter did not close palette")
	}
	if !m.wizardActive || m.wizardKind != WizardNewProject {
		t.Errorf("Enter on `new project` did not open wizard")
	}
}

// v2.9 §9.9 — actionsForContext reflects the cursor item. On Profiles
// tab pointing at a non-active profile we expect both "switch" and
// "delete" tokens; on the active profile, delete is suppressed.
func TestBottomMenu_ContextAwareOnProfiles(t *testing.T) {
	m := newTestModel(t)
	m = m.switchTab(TabProfiles)
	m.rebuildItems()
	if len(m.items) == 0 {
		t.Skip("no profiles in fixture")
	}
	// Find a non-active profile.
	target := -1
	for i, it := range m.items {
		if it.Kind == itemProfile && it.Profile != m.snap.ActiveProfile {
			target = i
			break
		}
	}
	if target < 0 {
		t.Skip("fixture has only the active profile")
	}
	m.cursor = target
	got := m.actionsForContext()
	if !strings.Contains(got, "switch to") {
		t.Errorf("Profiles-tab action row missing `switch to`: %q", got)
	}
	if !strings.Contains(got, "delete") {
		t.Errorf("Profiles-tab action row missing `delete`: %q", got)
	}
	// Now move cursor onto the active profile and re-check.
	for i, it := range m.items {
		if it.Kind == itemProfile && it.Profile == m.snap.ActiveProfile {
			m.cursor = i
			break
		}
	}
	got = m.actionsForContext()
	if strings.Contains(got, "[d] delete") {
		t.Errorf("active profile row should suppress delete: %q", got)
	}
}

// §9.5 cursor navigation.
func TestKey_DownMovesCursor(t *testing.T) {
	m := newTestModel(t)
	before := m.Cursor()
	m = sendKey(t, m, "down")
	if m.Cursor() <= before {
		t.Errorf("down did not advance cursor (was %d, is %d)", before, m.Cursor())
	}
}

// G4 / v2.9 §9.4 — `t` inside the Cards-tab modal opens carry-over.
// (Pre-v2.9 the test ran on Slots tab cursor; cards now live in modal.)
func TestKey_T_CarryOverOpensDetail(t *testing.T) {
	m := newTestModel(t)
	m = m.switchTab(TabCards)
	m.rebuildItems()
	if !m.cardModalActive {
		t.Fatal("setup: Cards tab should auto-open the modal")
	}
	m = sendKey(t, m, "t")
	if m.Prompt() != promptCarryOver {
		t.Errorf("t did not open carry-over: %s", m.Prompt())
	}
	if m.UIMode() != ModeManagement {
		t.Errorf("t did not set ModeManagement: %s", m.UIMode())
	}
}

// §9.5 — backspace shrinks the filter. We use non-action runes (z, y)
// so the action-rune dispatcher (n/d/a/u/r/t/x/?) doesn't intercept them.
func TestKey_BackspaceShrinksFilter(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(t, m, "z")
	m = sendKey(t, m, "y")
	if m.FilterValue() != "zy" {
		t.Fatalf("setup: filter = %q", m.FilterValue())
	}
	m = sendKey(t, m, "backspace")
	if m.FilterValue() != "z" {
		t.Errorf("backspace did not shrink filter: %q", m.FilterValue())
	}
}

// §8.4 — proposal mode auto-fires on a card-added subscription push.
func TestSubscription_CardAddedEntersProposalMode(t *testing.T) {
	m := newTestModel(t)
	if m.UIMode() != ModeIdle {
		t.Fatalf("setup: mode = %s", m.UIMode())
	}
	updated, _ := m.Update(subscriptionMsg{
		Push: pushFor("card-added"),
	})
	m = updated.(Model)
	if m.UIMode() != ModeProposal {
		t.Errorf("card-added did not enter proposal mode, got %s", m.UIMode())
	}
}

// SSOT §5.4 Proposal mode: "応答後、元の visibility 状態へ復帰". When
// the user was on Slots tab and a card arrives, the cockpit auto-hops
// to Cards. When all cards are dismissed, it must return to Slots (the
// previousTab) and the mode must drop back to Idle. This guards the
// auto-restore path that the unit test infrastructure has never
// exercised before — without it, future refactors could silently
// strand the user on an empty Cards tab.
func TestProposalMode_ReturnsToPreviousTabAfterAllCardsDismissed(t *testing.T) {
	m := newTestModel(t)
	if m.activeTab != TabSlots {
		t.Fatalf("setup: activeTab = %s, want Slots", m.activeTab)
	}

	// Drive into Proposal mode via the same card-added push that the
	// production daemon uses.
	updated, _ := m.Update(subscriptionMsg{Push: pushFor("card-added")})
	m = updated.(Model)
	if m.UIMode() != ModeProposal {
		t.Fatalf("after card-added, mode = %s, want Proposal", m.UIMode())
	}
	if m.activeTab != TabCards {
		t.Fatalf("after card-added, activeTab = %s, want Cards (auto-hop)", m.activeTab)
	}
	if m.previousTab != TabSlots {
		t.Fatalf("previousTab not preserved across auto-hop: got %s, want Slots", m.previousTab)
	}

	// Simulate "all cards dismissed" by emptying ActiveCards and
	// pumping any key — the Cards-modal branch checks len(cards)==0
	// and triggers the restore.
	m.snap.ActiveCards = nil
	m = sendKey(t, m, "esc")

	if m.UIMode() != ModeIdle {
		t.Errorf("after all dismissed, mode = %s, want Idle (SSOT §5.4 Proposal exit)", m.UIMode())
	}
	if m.activeTab != TabSlots {
		t.Errorf("after all dismissed, activeTab = %s, want Slots (SSOT §5.4 'visibility 復帰')", m.activeTab)
	}
}

// §10.4 — confirm-clear with "y" submits DismissAllCards (we can't
// observe the intent locally without a client, but the prompt should
// close).
func TestPrompt_ConfirmClearClosesOnY(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(t, m, "ctrl+l")
	if m.Prompt() != promptConfirmClear {
		t.Fatalf("setup: prompt = %s", m.Prompt())
	}
	m = sendKey(t, m, "y")
	m = sendKey(t, m, "enter")
	if m.Prompt() != promptNone {
		t.Errorf("confirm-clear did not close on y+Enter: %s", m.Prompt())
	}
}

// §10.4 — confirm-clear with "n" cancels.
func TestPrompt_ConfirmClearCancelsOnN(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(t, m, "ctrl+l")
	m = sendKey(t, m, "n")
	m = sendKey(t, m, "enter")
	if m.Prompt() != promptNone {
		t.Errorf("confirm-clear did not close on n+Enter: %s", m.Prompt())
	}
}

// §6 / v2.9 §9.4 — adopt-orphan is a two-step prompt; the first Enter
// advances promptStep without closing the prompt. Now driven through
// the Cards-tab modal (cardModalActive=true) instead of the legacy
// "card in flat items list" path.
func TestPrompt_AdoptOrphanTwoStep(t *testing.T) {
	m := newTestModel(t)
	// Simulate a [NEW] Vivaldi card so we exercise AdoptOrphan path
	// (the fixture's first card is ghostty).
	m.snap.ActiveCards[0].Context = map[string]string{"live": "live-9", "bundleID": "com.vivaldi.Vivaldi"}
	m = m.switchTab(TabCards)
	m.cardModalCursor = 0
	m.rebuildItems()
	// Enter on the modal → activateCardAction → AdoptOrphan prompt.
	m = sendKey(t, m, "enter")
	if m.Prompt() != promptAdoptOrphan {
		t.Fatalf("expected adopt-orphan prompt, got %s", m.Prompt())
	}
	if m.promptScratch["live"] != "live-9" {
		t.Errorf("live id not stashed: %+v", m.promptScratch)
	}
	// Step 0: type project, Enter → step 1.
	for _, r := range "dotfiles" {
		m = sendKey(t, m, string(r))
	}
	m = sendKey(t, m, "enter")
	if m.promptStep != 1 {
		t.Errorf("step did not advance: %d", m.promptStep)
	}
	if m.Prompt() != promptAdoptOrphan {
		t.Errorf("step 0 Enter closed the prompt prematurely")
	}
	// Step 1: type kind, Enter → close.
	for _, r := range "browser" {
		m = sendKey(t, m, string(r))
	}
	m = sendKey(t, m, "enter")
	if m.Prompt() != promptNone {
		t.Errorf("step 1 Enter did not close prompt: %s", m.Prompt())
	}
}

// pushFor builds a minimal ipc.SubscriptionPush for tests.
func pushFor(kind string) ipc.SubscriptionPush {
	return ipc.SubscriptionPush{Kind: kind}
}
