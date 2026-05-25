// Wizard B2 form (requirements §9.7). Replaces the legacy single-line
// "type a project id" prompt with a multi-field form so the user sees
// every defaulted value at once and can edit any of them before
// submitting.
//
// Two kinds:
//
//   WizardNewProject  — Slots tab `n` (or palette "new project")
//     fields: ID, Path, Slot, Windows
//   WizardNewProfile  — Profiles tab `n` (or palette "new profile")
//     fields: ID, Description, Inactive policy
//
// Submit order (project):
//   1. CreateProject{ID, Path}
//   2. AssignProject{Profile=active, Slot, Project} (if slot != "")
//   3. AddWindow{} for each checked window template
package tui

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

type wizardKind string

const (
	WizardNone       wizardKind = ""
	WizardNewProject wizardKind = "new-project"
	WizardNewProfile wizardKind = "new-profile"
	WizardAddWindow  wizardKind = "add-window"
)

type wizardFieldKind string

const (
	fieldText     wizardFieldKind = "text"
	fieldChoice   wizardFieldKind = "choice"   // left/right cycles
	fieldChecks   wizardFieldKind = "checks"   // space toggles cursor item
	fieldReadonly wizardFieldKind = "readonly" // derived display, not editable
)

// wizardField is a single form row. Choices / ChoiceIdx are used when
// Kind == fieldChoice. Items / ItemChecked / ItemCursor are used when
// Kind == fieldChecks.
type wizardField struct {
	Label string
	Kind  wizardFieldKind

	// Text (when fieldText / fieldReadonly).
	TextValue   string
	Placeholder string
	HintRight   string // dimmed hint shown right of the value (e.g. "✓ exists")

	// Choice fields.
	Choices   []string
	ChoiceIdx int

	// Check fields.
	Items       []string
	ItemChecked []bool
	ItemCursor  int
}

// wizardOpenNewProject builds the four-field new-project form with
// defaults prefilled from current snapshot state. `activeSlot` is the
// currently focused slot id (used as the recommended target).
func (m *Model) wizardOpenNewProject() {
	home, _ := os.UserHomeDir()
	// Choices: every slot in active profile + "" (= leave unassigned).
	slotChoices := []string{""}
	for _, sl := range m.snap.Slots {
		slotChoices = append(slotChoices, string(sl.ID))
	}
	// Default slot: first free slot under active profile (= no assignment
	// in active profile's map), falling back to cursor's slot.
	defaultSlotIdx := 0
	if prof, ok := m.snap.Profiles[m.snap.ActiveProfile]; ok {
		for i, name := range slotChoices {
			if name == "" {
				continue
			}
			if _, taken := prof.Assignments[w.SlotID(name)]; !taken {
				defaultSlotIdx = i
				break
			}
		}
	}

	m.wizardKind = WizardNewProject
	m.wizardActive = true
	m.wizardCursor = 0
	m.wizardFields = []wizardField{
		{Label: "ID", Kind: fieldText, Placeholder: "project id (lower-kebab)"},
		{Label: "Path", Kind: fieldText, TextValue: home + "/dev/", Placeholder: "$HOME/dev/<id>"},
		{Label: "Slot", Kind: fieldChoice, Choices: slotChoices, ChoiceIdx: defaultSlotIdx},
		{Label: "Windows", Kind: fieldReadonly, TextValue: "ai-1 + shell-1 + editor (default — edit later via [r]/AddWindow)"},
	}
	m.wizardSyncDerived()
}

// wizardOpenAddWindow builds the add-window form for the given project.
// Kind cycles ai/shell/editor/browser; Index auto-defaults to the next
// free slot for the selected Kind (recomputed each time the cursor
// re-enters the Kind field).
func (m *Model) wizardOpenAddWindow(pid w.ProjectID) {
	m.wizardKind = WizardAddWindow
	m.wizardActive = true
	m.wizardCursor = 0
	kindChoices := []string{string(w.WindowAI), string(w.WindowShell), string(w.WindowEditor), string(w.WindowBrowser)}
	m.wizardFields = []wizardField{
		{Label: "Project", Kind: fieldReadonly, TextValue: string(pid)},
		{Label: "Kind", Kind: fieldChoice, Choices: kindChoices, ChoiceIdx: 0},
		{Label: "Index", Kind: fieldText, TextValue: ""},
		{Label: "AI name", Kind: fieldText, TextValue: "claude", Placeholder: "claude"},
	}
	m.wizardSyncDerived()
}

// wizardOpenNewProfile builds the new-profile form.
func (m *Model) wizardOpenNewProfile() {
	m.wizardKind = WizardNewProfile
	m.wizardActive = true
	m.wizardCursor = 0
	m.wizardFields = []wizardField{
		{Label: "ID", Kind: fieldText, Placeholder: "profile id"},
		{Label: "Description", Kind: fieldText, Placeholder: "(optional)"},
		{Label: "Inactive policy", Kind: fieldChoice,
			Choices:   []string{"remove", "keep-visible"},
			ChoiceIdx: 0,
		},
	}
}

func (m *Model) wizardCancel() {
	m.wizardActive = false
	m.wizardKind = WizardNone
	m.wizardFields = nil
	m.wizardCursor = 0
}

// wizardHandleKey is the wizard-only key dispatcher. Returns the
// updated model + cmd; callers must short-circuit Update when active.
func (m Model) wizardHandleKey(key string) (Model, tea.Cmd) {
	if !m.wizardActive {
		return m, nil
	}
	switch key {
	case "esc":
		m.wizardCancel()
		m.status = "wizard cancelled"
		return m, nil
	case "tab":
		m.wizardCursor = (m.wizardCursor + 1) % len(m.wizardFields)
		return m, nil
	case "shift+tab":
		m.wizardCursor = (m.wizardCursor - 1 + len(m.wizardFields)) % len(m.wizardFields)
		return m, nil
	case "enter":
		return m.wizardSubmit()
	}

	if m.wizardCursor < 0 || m.wizardCursor >= len(m.wizardFields) {
		return m, nil
	}
	f := &m.wizardFields[m.wizardCursor]

	switch f.Kind {
	case fieldText:
		switch key {
		case "backspace":
			if len(f.TextValue) > 0 {
				f.TextValue = f.TextValue[:len(f.TextValue)-1]
				m.wizardSyncDerived()
			}
		default:
			if isPrintable(key) {
				f.TextValue += key
				m.wizardSyncDerived()
			}
		}
	case fieldChoice:
		switch key {
		case "left", "h":
			if f.ChoiceIdx > 0 {
				f.ChoiceIdx--
				m.wizardOnChoiceChanged()
			}
		case "right", "l":
			if f.ChoiceIdx < len(f.Choices)-1 {
				f.ChoiceIdx++
				m.wizardOnChoiceChanged()
			}
		}
	case fieldChecks:
		switch key {
		case "left", "h":
			if f.ItemCursor > 0 {
				f.ItemCursor--
			}
		case "right", "l":
			if f.ItemCursor < len(f.Items)-1 {
				f.ItemCursor++
			}
		case " ", "space":
			if f.ItemCursor >= 0 && f.ItemCursor < len(f.ItemChecked) {
				f.ItemChecked[f.ItemCursor] = !f.ItemChecked[f.ItemCursor]
			}
		}
	}
	return m, nil
}

// isPrintable returns true when the bubbletea key string is a single
// rune we can append to a text field. Filters control keys, modifiers,
// arrow names, etc.
func isPrintable(key string) bool {
	if len(key) != 1 {
		return false
	}
	c := key[0]
	return c >= 0x20 && c < 0x7f
}

// wizardSyncDerived re-derives fields that depend on the ID text. For
// new-project, Path stays in sync with `$HOME/dev/<id>` unless the user
// has manually edited the Path away from the default prefix. Also
// re-evaluates the Path existence hint so the user sees up-front
// whether the wizard will create or reuse the dir. For add-window,
// Index defaults to "next free for selected Kind"; AIName is hinted
// dimmed when Kind != ai.
func (m *Model) wizardSyncDerived() {
	switch m.wizardKind {
	case WizardNewProject:
		if len(m.wizardFields) < 2 {
			return
		}
		idField := m.wizardFields[0]
		pathField := &m.wizardFields[1]
		home, _ := os.UserHomeDir()
		defaultPrefix := home + "/dev/"
		if strings.HasPrefix(pathField.TextValue, defaultPrefix) || pathField.TextValue == "" {
			pathField.TextValue = defaultPrefix + idField.TextValue
		}
		pathField.HintRight = pathExistenceHint(pathField.TextValue)
	case WizardAddWindow:
		if len(m.wizardFields) < 4 {
			return
		}
		pidField := m.wizardFields[0]
		kindField := m.wizardFields[1]
		indexField := &m.wizardFields[2]
		aiNameField := &m.wizardFields[3]
		var kind w.WindowKind
		if kindField.ChoiceIdx >= 0 && kindField.ChoiceIdx < len(kindField.Choices) {
			kind = w.WindowKind(kindField.Choices[kindField.ChoiceIdx])
		}
		// Auto-fill Index with the next free for this Kind. Stay
		// editable so the user can override.
		if indexField.TextValue == "" {
			next := m.nextFreeWindowIndex(w.ProjectID(pidField.TextValue), kind)
			indexField.TextValue = fmt.Sprintf("%d", next)
		}
		if kind == w.WindowAI {
			aiNameField.HintRight = ""
		} else {
			aiNameField.HintRight = "(ignored for non-ai kind)"
		}
	}
}

// wizardOnChoiceChanged is invoked after the user cycles a choice
// field; lets dependent text fields (e.g. AddWindow Index that depends
// on Kind) re-derive against the new selection.
func (m *Model) wizardOnChoiceChanged() {
	if m.wizardKind == WizardAddWindow && len(m.wizardFields) >= 3 {
		m.wizardFields[2].TextValue = "" // recompute via syncDerived
	}
	m.wizardSyncDerived()
}

// nextFreeWindowIndex scans the current project's Windows for the
// highest Index of the given Kind and returns max+1 (or 1 when none).
func (m *Model) nextFreeWindowIndex(pid w.ProjectID, kind w.WindowKind) int {
	pr, ok := m.snap.Projects[pid]
	if !ok {
		return 1
	}
	maxIdx := 0
	for _, dw := range pr.Windows {
		if dw.Kind == kind && dw.ID.Index > maxIdx {
			maxIdx = dw.ID.Index
		}
	}
	return maxIdx + 1
}

// pathExistenceHint returns the right-side dimmed annotation describing
// whether the path is on-disk now, will be created, or is invalid.
func pathExistenceHint(p string) string {
	if strings.TrimSpace(p) == "" {
		return ""
	}
	info, err := os.Stat(p)
	if err == nil {
		if info.IsDir() {
			return "✓ exists"
		}
		return "✗ not a directory"
	}
	if os.IsNotExist(err) {
		return "⚠ will be created"
	}
	return "✗ " + err.Error()
}

// wizardSubmit fires the appropriate intent chain and closes the wizard.
func (m Model) wizardSubmit() (Model, tea.Cmd) {
	switch m.wizardKind {
	case WizardNewProject:
		return m.wizardSubmitProject()
	case WizardNewProfile:
		return m.wizardSubmitProfile()
	case WizardAddWindow:
		return m.wizardSubmitAddWindow()
	}
	return m, nil
}

func (m Model) wizardSubmitProject() (Model, tea.Cmd) {
	if len(m.wizardFields) < 4 {
		m.status = "wizard: fields not populated"
		return m, nil
	}
	id := strings.TrimSpace(m.wizardFields[0].TextValue)
	if id == "" {
		m.status = "wizard: ID required"
		return m, nil
	}
	path := strings.TrimSpace(m.wizardFields[1].TextValue)
	if path == "" {
		home, _ := os.UserHomeDir()
		path = home + "/dev/" + id
	}
	// The daemon's executor cd's into the project root when spawning
	// tmux sessions; if the path doesn't exist the spawn fails and the
	// whole intent is rolled back. Create the dir first so the executor
	// has somewhere to land. mkdir -p semantics: no error if it already
	// exists, fails only on real I/O problems (which we surface).
	if err := os.MkdirAll(path, 0o755); err != nil {
		m.status = "wizard: mkdir " + path + ": " + err.Error()
		return m, nil
	}
	slotField := m.wizardFields[2]
	slot := ""
	if slotField.ChoiceIdx >= 0 && slotField.ChoiceIdx < len(slotField.Choices) {
		slot = slotField.Choices[slotField.ChoiceIdx]
	}
	pid := w.ProjectID(id)
	// CreateProject must reach the daemon strictly before AssignProject
	// — otherwise the reducer rejects assign with "unknown project".
	// tea.Batch fires concurrently so the two intents race the socket;
	// tea.Sequence chains them so the second runs only after the first
	// has returned.
	cmds := []tea.Cmd{
		submitIntentCmd(m.cfg.Client, intent.CreateProject{ID: pid, Path: path}),
	}
	if slot != "" {
		cmds = append(cmds, submitIntentCmd(m.cfg.Client, intent.AssignProject{
			Slot:    w.SlotID(slot),
			Project: pid,
		}))
	}
	m.wizardCancel()
	m.status = fmt.Sprintf("creating project %s (slot=%s)", id, slot)
	return m, tea.Sequence(cmds...)
}

func (m Model) wizardSubmitAddWindow() (Model, tea.Cmd) {
	if len(m.wizardFields) < 4 {
		m.status = "wizard: fields not populated"
		return m, nil
	}
	pid := w.ProjectID(strings.TrimSpace(m.wizardFields[0].TextValue))
	if pid == "" {
		m.status = "wizard: project required"
		return m, nil
	}
	kindField := m.wizardFields[1]
	kindStr := ""
	if kindField.ChoiceIdx >= 0 && kindField.ChoiceIdx < len(kindField.Choices) {
		kindStr = kindField.Choices[kindField.ChoiceIdx]
	}
	if kindStr == "" {
		m.status = "wizard: window kind required"
		return m, nil
	}
	kind := w.WindowKind(kindStr)
	idx := 0
	if v := strings.TrimSpace(m.wizardFields[2].TextValue); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			idx = parsed
		} else {
			m.status = "wizard: index must be a number"
			return m, nil
		}
	}
	aiName := strings.TrimSpace(m.wizardFields[3].TextValue)
	if kind == w.WindowAI && aiName == "" {
		aiName = "claude"
	}
	cmd := submitIntentCmd(m.cfg.Client, intent.AddWindow{
		Project:    pid,
		WindowKind: kind,
		Index:      idx,
		AIName:     aiName,
	})
	m.wizardCancel()
	status := fmt.Sprintf("adding %s-%d to %s", kind, idx, pid)
	if kind == w.WindowBrowser {
		// Privacy contract: browser windows can't auto-spawn without
		// a private URL payload. Spell out the next step so the user
		// isn't left wondering why no Vivaldi window appeared.
		status += " — spawn deferred until URLs are populated via the live Vivaldi automation profile"
	}
	m.status = status
	return m, cmd
}

func (m Model) wizardSubmitProfile() (Model, tea.Cmd) {
	if len(m.wizardFields) < 3 {
		m.status = "wizard: fields not populated"
		return m, nil
	}
	id := strings.TrimSpace(m.wizardFields[0].TextValue)
	if id == "" {
		m.status = "wizard: profile ID required"
		return m, nil
	}
	desc := strings.TrimSpace(m.wizardFields[1].TextValue)
	policyField := m.wizardFields[2]
	policy := w.InactivePolicyRemove
	if policyField.ChoiceIdx >= 0 && policyField.ChoiceIdx < len(policyField.Choices) {
		policy = w.InactivePolicy(policyField.Choices[policyField.ChoiceIdx])
	}
	cmd := submitIntentCmd(m.cfg.Client, intent.CreateProfile{
		ID:             w.ProfileID(id),
		Description:    desc,
		InactivePolicy: policy,
	})
	m.wizardCancel()
	m.status = "created profile " + id
	return m, cmd
}

// wizardView renders the form overlay. Layout per requirements §9.7.
func (m Model) wizardView() string {
	if !m.wizardActive {
		return ""
	}
	title := "Create new project"
	switch m.wizardKind {
	case WizardNewProfile:
		title = "Create new profile"
	case WizardAddWindow:
		title = "Add window"
	}
	var b strings.Builder
	b.WriteString(styleHeading.Render("┌─ " + title + " ─────────────────────────┐"))
	b.WriteString("\n")
	for i, f := range m.wizardFields {
		cursor := "  "
		if i == m.wizardCursor {
			cursor = styleCursor.Render("▶ ")
		}
		valueCol := m.renderWizardFieldValue(f, i == m.wizardCursor)
		row := fmt.Sprintf("│ %s%-16s %s", cursor, f.Label, valueCol)
		if f.HintRight != "" {
			row += "  " + styleDim.Render(f.HintRight)
		}
		b.WriteString(row)
		b.WriteString("\n")
	}
	b.WriteString("│\n")
	b.WriteString("│ " + styleDim.Render("Tab/Shift+Tab: field   ←/→: choose   Space: toggle   Enter: submit   Esc: cancel"))
	b.WriteString("\n")
	b.WriteString("└" + strings.Repeat("─", 70) + "┘")
	return b.String()
}

func (m Model) renderWizardFieldValue(f wizardField, focused bool) string {
	switch f.Kind {
	case fieldText:
		v := f.TextValue
		if v == "" {
			v = styleDim.Render(f.Placeholder)
		}
		if focused {
			v += styleCursor.Render("▌")
		}
		return v
	case fieldChoice:
		if len(f.Choices) == 0 {
			return styleDim.Render("(no choices)")
		}
		v := f.Choices[f.ChoiceIdx]
		if v == "" {
			v = styleDim.Render("(unassigned)")
		}
		arrows := ""
		if focused {
			arrows = "  " + styleDim.Render("(←/→)")
		}
		return "▾ " + v + arrows
	case fieldChecks:
		var parts []string
		for i, name := range f.Items {
			box := "☐"
			if i < len(f.ItemChecked) && f.ItemChecked[i] {
				box = "☑"
			}
			part := box + " " + name
			if focused && i == f.ItemCursor {
				part = styleCursor.Render(part)
			}
			parts = append(parts, part)
		}
		return strings.Join(parts, "  ")
	case fieldReadonly:
		return styleDim.Render(f.TextValue)
	}
	return ""
}

// Suppress unused-import-when-debugging warning helpers.
var _ = sort.Slice
