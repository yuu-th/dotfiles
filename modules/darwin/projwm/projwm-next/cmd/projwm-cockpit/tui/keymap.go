// Keymap for the cockpit TUI. Requirements §9.5.
//
// The §9.5 table covers every binding the user can type while the
// cockpit window is focused. The map is structured around two
// concerns:
//
//  1. Cursor + filter mechanics (Up/Down/Ctrl+J/K/Backspace).
//  2. Action keys (Enter / n / d / a / u / r / t / Ctrl+L / Ctrl+C /
//     Esc / Tab / ?).
//
// When `filterFocused == false`, action keys are dispatched as
// actions. When filterFocused is true, runes flow into the textinput
// (the filter is started by the first non-action rune; see
// update.go).
package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap groups the §9.5 bindings.
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	CtrlJ      key.Binding
	CtrlK      key.Binding
	Enter      key.Binding
	Esc        key.Binding
	Tab        key.Binding
	NewProj    key.Binding // n
	Unassign   key.Binding // d
	Archive    key.Binding // a
	Unarchive  key.Binding // u
	Remove     key.Binding // r
	Help       key.Binding // ?
	DismissAll key.Binding // Ctrl+L
	Quit       key.Binding // Ctrl+C (= hide cockpit)
	CarryOver  key.Binding // t (carry card to detail view)
}

// DefaultKeyMap returns the keymap that matches requirements §9.5 verbatim.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "ctrl+k"),
			key.WithHelp("↑/Ctrl+K", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "ctrl+j"),
			key.WithHelp("↓/Ctrl+J", "down"),
		),
		CtrlJ: key.NewBinding(
			key.WithKeys("ctrl+j"),
			key.WithHelp("Ctrl+J", "down"),
		),
		CtrlK: key.NewBinding(
			key.WithKeys("ctrl+k"),
			key.WithHelp("Ctrl+K", "up"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("Enter", "activate"),
		),
		Esc: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("Esc", "clear/close/hide"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("Tab", "cycle profile"),
		),
		NewProj: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new project"),
		),
		Unassign: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "unassign/dismiss"),
		),
		Archive: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "archive"),
		),
		Unarchive: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "unarchive"),
		),
		Remove: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "remove window"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		DismissAll: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("Ctrl+L", "dismiss all cards"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("Ctrl+C", "hide cockpit"),
		),
		CarryOver: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "carry over"),
		),
	}
}

// ActionKeys returns the runes that act as action keys (so the filter
// textinput knows not to consume them when the filter is empty).
// Requirements §9.5 letter-set + "?".
func (k KeyMap) ActionRunes() map[rune]bool {
	return map[rune]bool{
		'n': true, // new project / new profile (context-aware)
		'd': true, // unassign / delete profile / dismiss card
		'a': true, // archive
		'u': true, // unarchive (or jump to Archived tab from Slots)
		'r': true, // remove window / rename profile / refresh trace
		't': true, // carry-over (card modal)
		'x': true, // purge archived project (Archived tab)
		'+': true, // add window (Slots tab on assigned slot)
		'?': true, // help overlay
	}
}
