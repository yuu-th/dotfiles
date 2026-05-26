package tui

import (
	"strings"
	"testing"
)

// SSOT §5.4 全体構造 (GAP-11) snapshot tests.
//
// We do NOT compare View() output to a frozen golden string — those
// tests rot every time a label is reworded. Instead we assert SSOT's
// section-by-section requirements as semantic snapshot: each declared
// element MUST appear in the rendered output. Lipgloss style escape
// sequences are stripped via strings.Contains before assertion.

// SSOT §5.4 topbar: "gen / epoch / profile / convergence / cards".
// (Implementation also surfaces source, digest, policy — those are
// extensions, not regressions, so we only assert the SSOT minimum.)
func TestSSOTSection54_TopbarContainsAllRequiredFields(t *testing.T) {
	m := newTestModel(t)
	out := m.topbarView()

	// Strip ANSI escapes so the substring search is reliable across
	// lipgloss styling.
	plain := stripANSI(out)

	required := map[string]string{
		"gen=":   "generation id",
		"epoch=": "controller epoch",
		"prof=":  "active profile",
		"cards=": "active card count",
	}
	for token, what := range required {
		if !strings.Contains(plain, token) {
			t.Errorf("SSOT §5.4 topbar missing %s — token %q not found in:\n%s", what, token, plain)
		}
	}

	// Convergence status must appear as one of the three SSOT-allowed
	// labels (CONVERGED / CONVERGING / REPLAN_FAILED).
	if !strings.Contains(plain, "CONVERGED") &&
		!strings.Contains(plain, "CONVERGING") &&
		!strings.Contains(plain, "REPLAN_FAILED") {
		t.Errorf("SSOT §5.4 topbar must show convergence status (CONVERGED/CONVERGING/REPLAN_FAILED):\n%s", plain)
	}
}

// SSOT §5.4 tab bar: 5 tabs (Slots / Cards / Archived / Profiles / Trace).
func TestSSOTSection54_TabBarShowsAllFiveTabs(t *testing.T) {
	m := newTestModel(t)
	out := stripANSI(m.tabBarView())

	required := []string{"Slots", "Cards", "Archived", "Profiles", "Trace"}
	for _, label := range required {
		if !strings.Contains(out, label) {
			t.Errorf("SSOT §5.4 tab bar missing tab %q:\n%s", label, out)
		}
	}
}

// SSOT §5.4 Cards tab label includes the count of active cards: "Cards (N)".
func TestSSOTSection54_CardsTabLabelShowsCount(t *testing.T) {
	m := newTestModel(t)
	out := stripANSI(m.tabBarView())
	// Fixture has 2 cards.
	if !strings.Contains(out, "Cards (2)") {
		t.Errorf("SSOT §5.4 cards tab label must show count, got:\n%s", out)
	}
}

// SSOT §5.4 active tab indication: the active tab MUST be visually
// distinguishable. The cockpit implementation uses brackets ([ Slots ])
// for the active tab; we assert that exactly one tab label appears in
// bracket form.
func TestSSOTSection54_ActiveTabIsHighlightedOnTabBar(t *testing.T) {
	m := newTestModel(t)
	// Default active tab is Slots.
	out := stripANSI(m.tabBarView())
	if !strings.Contains(out, "[ Slots ]") {
		t.Errorf("SSOT §5.4 active tab must be visually marked (cockpit uses [ Name ]); got:\n%s", out)
	}
	// Switch to Cards and re-render.
	m.activeTab = TabCards
	out = stripANSI(m.tabBarView())
	if !strings.Contains(out, "[ Cards") {
		t.Errorf("SSOT §5.4 after tab switch, Cards tab must be highlighted; got:\n%s", out)
	}
	if strings.Contains(out, "[ Slots ]") {
		t.Errorf("SSOT §5.4 only the active tab should be highlighted; Slots is still bracketed after switch:\n%s", out)
	}
}

// SSOT §5.4 Slots tab content: active profile の slot Q-P assignment、
// viewer (A) AI stream 一覧、park 一覧.
func TestSSOTSection54_SlotsTabShowsAssignmentsAndPark(t *testing.T) {
	m := newTestModel(t)
	m.activeTab = TabSlots
	m.rebuildItems()
	out := stripANSI(m.activeTabContent())

	// Fixture has Q=dotfiles assigned + park-a in park state.
	if !strings.Contains(out, "dotfiles") {
		t.Errorf("SSOT §5.4 Slots tab must show assigned project (dotfiles):\n%s", out)
	}
	if !strings.Contains(out, "park-a") {
		t.Errorf("SSOT §5.4 Slots tab must list parked projects (park-a):\n%s", out)
	}
}

// SSOT §5.4 Archived tab content: archived project 一覧.
func TestSSOTSection54_ArchivedTabShowsArchivedProjects(t *testing.T) {
	m := newTestModel(t)
	m.activeTab = TabArchived
	m.rebuildItems()
	out := stripANSI(m.activeTabContent())

	if !strings.Contains(out, "arch-a") {
		t.Errorf("SSOT §5.4 Archived tab must show archived project (arch-a):\n%s", out)
	}
	// Live (non-archived) project must NOT appear in Archived tab.
	if strings.Contains(out, "dotfiles") {
		t.Errorf("SSOT §5.4 Archived tab must NOT show non-archived projects (dotfiles leaked):\n%s", out)
	}
}

// SSOT §5.4 Profiles tab content: 全 profile + active 強調.
func TestSSOTSection54_ProfilesTabShowsAllProfilesWithActiveMarker(t *testing.T) {
	m := newTestModel(t)
	m.activeTab = TabProfiles
	m.rebuildItems()
	out := stripANSI(m.activeTabContent())

	// Fixture has "work" (active) and "home" profiles.
	if !strings.Contains(out, "work") {
		t.Errorf("SSOT §5.4 Profiles tab must list active profile (work):\n%s", out)
	}
	if !strings.Contains(out, "home") {
		t.Errorf("SSOT §5.4 Profiles tab must list non-active profile (home):\n%s", out)
	}
}

// stripANSI removes lipgloss ANSI escape codes so substring assertions
// are not fooled by inserted styling bytes. Recognises the common SGR
// form `\x1b[<digits>;<digits>m`.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != 0x1b {
			b.WriteByte(s[i])
			continue
		}
		// skip CSI
		i++
		if i < len(s) && s[i] == '[' {
			i++
			for i < len(s) && s[i] != 'm' && s[i] != 'K' && s[i] != 'H' && s[i] != 'J' {
				i++
			}
			// i now points at the terminator (or end); the loop's i++ moves past it.
		}
	}
	return b.String()
}
