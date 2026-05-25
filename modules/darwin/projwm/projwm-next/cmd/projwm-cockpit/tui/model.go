// Package tui implements the projwm-cockpit TUI on top of charmbracelet/
// bubbletea. Requirements §8 (cockpit lifecycle), §9 (TUI keymap +
// information elements), §10 (cards) live entirely here. The package
// is import-clean of cmd/projwm-cockpit/main so unit tests can drive
// the model directly.
package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/yuu-th/projwm-next/internal/cockpitclient"
	"github.com/yuu-th/projwm-next/internal/cockpitsnap"
	"github.com/yuu-th/projwm-next/internal/ipc"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// Config wires the model to its env: socket path, manifest, store
// directory, plus the subscribe channel fed by main's goroutine.
type Config struct {
	Client       *cockpitclient.Client
	StoreDir     string
	ManifestPath string
	// SubscribeCh receives every daemon push. Buffered so the producer
	// (main goroutine) doesn't stall waiting for tea to consume.
	SubscribeCh <-chan ipc.SubscriptionPush
	// RefreshInterval drives both periodic snapshot refresh and the
	// relative-time tick for card CreatedAt.
	RefreshInterval int // seconds; 0 → 2s default
}

// itemKind labels the type of selectable row.
type itemKind string

const (
	itemCard    itemKind = "card"
	itemSlot    itemKind = "slot"
	itemViewer  itemKind = "viewer"
	itemParked  itemKind = "parked"
	itemArchive itemKind = "archived"
	itemProfile itemKind = "profile"
	itemTrace   itemKind = "trace"
)

// traceSummary is the minimum a Trace-tab row needs: enough to render
// the one-line list entry plus the absolute path to load full detail
// on Enter. Loaded directly from the on-disk traces dir.
type traceSummary struct {
	TxID      string // e.g. txn-1779063585758011000
	Command   string
	Reason    string
	StartedAt string
	Converged bool
	Discarded bool
	Path      string // absolute trace file path
}

// listItem is one flattened row.
type listItem struct {
	Kind        itemKind
	Label       string
	Detail      string
	Slot        w.SlotID
	Project     w.ProjectID
	Profile     w.ProfileID
	CardID      w.CardID
	CardType    w.CardType
	CardActions []w.CardAction
	// Card-specific:
	CardCreatedAt int64
	CardContext   map[string]string
	// Trace-specific (itemTrace only):
	TraceID   string
	TracePath string
}

// tabKind enumerates the v2.9 §9.4 top tabs.
type tabKind string

const (
	TabSlots    tabKind = "slots"
	TabCards    tabKind = "cards"
	TabArchived tabKind = "archived"
	TabProfiles tabKind = "profiles"
	TabTrace    tabKind = "trace"
)

// tabsOrder is the canonical L→R order; used by 1-5 keys and prev/next.
var tabsOrder = []tabKind{TabSlots, TabCards, TabArchived, TabProfiles, TabTrace}

// tabIndex returns the 0-based position of t in tabsOrder; -1 if unknown.
func tabIndex(t tabKind) int {
	for i, x := range tabsOrder {
		if x == t {
			return i
		}
	}
	return -1
}

// uiMode discriminates the cockpit §8.4 visibility modes.
type uiMode string

const (
	ModeIdle       uiMode = "idle"
	ModeProposal   uiMode = "proposal"
	ModeNavigation uiMode = "navigation"
	ModeManagement uiMode = "management"
)

// promptKind tags which modal substate is active.
type promptKind string

const (
	promptNone           promptKind = ""
	promptNewProject     promptKind = "new-project"     // n on Slots
	promptUnarchive      promptKind = "unarchive"       // u on Archived
	promptRemoveWindow   promptKind = "remove-window"   // r on Slots
	promptConfirmClear   promptKind = "confirm-clear"   // Ctrl+L
	promptAdoptOrphan    promptKind = "adopt-orphan"    // Enter on [NEW] Vivaldi/Zed
	promptRespawnGhostty promptKind = "respawn-ghostty" // Enter on [NEW] Ghostty
	promptCarryOver      promptKind = "carry-over"      // t on card
	// v2.9 §9.4 — Profiles tab.
	promptNewProfile    promptKind = "new-profile"    // n on Profiles
	promptDeleteProfile promptKind = "delete-profile" // d on Profiles
	promptRenameProfile promptKind = "rename-profile" // r on Profiles
	// v2.9 §9.4 — Archived tab.
	promptPurgeProject promptKind = "purge-project" // x on Archived
)

// Model is bubbletea's tea.Model. Public so tests can construct it
// directly.
type Model struct {
	cfg Config

	// World snapshot.
	snap cockpitsnap.Snapshot

	// Flattened, filtered item list. rebuildItems() refreshes it.
	items  []listItem
	cursor int

	// Filter (§9.4): fzf-style live substring AND across whitespace
	// tokens, matching label+detail case-insensitively.
	filter        textinput.Model
	filterFocused bool

	// Modal prompt substate.
	prompt        promptKind
	promptInput   textinput.Model
	promptTarget  listItem
	promptStep    int
	promptScratch map[string]string

	// UI mode (§8.4).
	uiMode uiMode

	// Window dimensions.
	width, height int

	// Help (?).
	help     help.Model
	keys     KeyMap
	showHelp bool

	// Status line ("switched profile to X", "intent error: ...").
	status string

	// Carry-over card id (when promptCarryOver is active).
	carryCardID w.CardID

	// Card modal (full-screen overlay, requirements §10 + ユーザ提案 2026-05-18).
	// When activeCards is non-empty and cardModalActive is true, the view
	// switches to a 2-column layout:
	//   Left: focused card detail + actions
	//   Right: workspace zoom-out diagram (slots, viewer streams, park)
	// Triggered automatically by `card-added` subscription pushes (proposal
	// mode K1.5) and manually by Enter on a card item.
	cardModalActive bool
	cardModalCursor int // index into snap.ActiveCards sorted desc by CreatedAt

	// v2.9 §9.4 top tabs.
	activeTab   tabKind
	previousTab tabKind // for `o` toggle and Cards-tab auto-hop return

	// v2.9 §9.5 profile picker overlay (`;` key opens this).
	profilePickerActive bool
	profilePickerCursor int // index into sorted profile list

	// v2.9 §9.7 Wizard B2 form (new project / new profile). When
	// wizardActive, the View switches to a dedicated overlay and Update
	// routes all keys to wizardHandleKey.
	wizardActive bool
	wizardKind   wizardKind
	wizardFields []wizardField
	wizardCursor int

	// v2.9 §9.8 Command palette (Ctrl-P). paletteActions is rebuilt
	// every open so it sees the current snapshot + cursor.
	paletteActive  bool
	paletteQuery   string
	paletteActions []paletteAction
	paletteCursor  int

	// v2.9 §9.4 Trace tab. traces is loaded from disk by loadTracesFromDisk
	// on switchTab(TabTrace) or `r`; the cached list backs buildTraceItems.
	// traceDetail/traceDetailActive overlay the full JSON for the Enter-
	// selected trace; Esc closes it.
	traces            []traceSummary
	traceDetail       string
	traceDetailActive bool

	// Quitting flag — set by hideCockpit so the program returns on the
	// next tea.Quit cmd cycle.
	quitting bool
}

// New constructs a fresh Model. Call Model.Init() from tea.Program.
func New(cfg Config) Model {
	fi := textinput.New()
	fi.Prompt = "/"
	fi.Placeholder = "fzf filter — type to narrow…"
	fi.CharLimit = 256

	pi := textinput.New()
	pi.Prompt = "▶ "
	pi.CharLimit = 256

	h := help.New()

	return Model{
		cfg:         cfg,
		filter:      fi,
		promptInput: pi,
		help:        h,
		keys:        DefaultKeyMap(),
		uiMode:      ModeIdle,
		activeTab:   TabSlots,
		previousTab: TabSlots,
	}
}

// Init satisfies tea.Model. Kicks off the first snapshot fetch + tick
// + (if a channel was provided) subscribe listener.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		loadSnapshotCmd(m.cfg.Client, m.cfg.StoreDir, m.cfg.ManifestPath),
		tickEveryCmd(refreshDuration(m.cfg.RefreshInterval)),
	}
	if m.cfg.SubscribeCh != nil {
		cmds = append(cmds, listenSubscribeCmd(m.cfg.SubscribeCh))
	}
	return tea.Batch(cmds...)
}

// Selected returns the cursor row, or zero value when empty.
func (m *Model) Selected() listItem {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return listItem{}
	}
	return m.items[m.cursor]
}

// UIMode returns the current cockpit mode (exposed for tests).
func (m *Model) UIMode() uiMode { return m.uiMode }

// Snapshot returns the current snapshot (exposed for tests).
func (m *Model) Snapshot() cockpitsnap.Snapshot { return m.snap }

// Items returns the filtered item slice (exposed for tests).
func (m *Model) Items() []listItem { return m.items }

// Cursor returns the current cursor (exposed for tests).
func (m *Model) Cursor() int { return m.cursor }

// Status returns the status line (exposed for tests).
func (m *Model) Status() string { return m.status }

// FilterValue returns the current filter buffer (exposed for tests).
func (m *Model) FilterValue() string { return m.filter.Value() }

// Prompt returns the current prompt kind (exposed for tests).
func (m *Model) Prompt() promptKind { return m.prompt }

// rebuildItems flattens the snapshot into the visible list, applying
// the current filter. Implements §9.2 ordering: cards (§9.3, newest
// first) → slots (active profile, sorted by Order) → park → archived
// → viewer AI streams (§9.2 last bullet) → other profiles.
//
// G7 (newest card on top): m.snap.ActiveCards is already controller-
// emitted; we sort descending by CreatedAt here so independently of
// daemon order the newest card lands at position 0.
func (m *Model) rebuildItems() {
	// v2.9 §9.4: items list scope is tab-aware. Each tab only shows the
	// rows that are actionable in that tab so cursor + actions are
	// always coherent. Cards tab uses a dedicated modal renderer so its
	// items list is left empty (modal cursor is m.cardModalCursor).
	var all []listItem
	switch m.activeTab {
	case TabSlots:
		all = m.buildSlotsItems()
	case TabCards:
		// Cards modal owns its own cursor; items stays empty so the
		// shared Up/Down handlers don't fight the modal navigation.
		all = nil
	case TabArchived:
		all = m.buildArchivedItems()
	case TabProfiles:
		all = m.buildProfilesItems()
	case TabTrace:
		all = m.buildTraceItems()
	default:
		all = m.buildSlotsItems()
	}

	// Apply filter (case-insensitive AND across whitespace tokens).
	q := m.filter.Value()
	if q == "" {
		m.items = all
	} else {
		tokens := strings.Fields(strings.ToLower(q))
		filtered := all[:0]
		for _, it := range all {
			hay := strings.ToLower(it.Label + " " + it.Detail)
			match := true
			for _, t := range tokens {
				if !strings.Contains(hay, t) {
					match = false
					break
				}
			}
			if match {
				filtered = append(filtered, it)
			}
		}
		m.items = filtered
	}

	// Clamp cursor.
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// buildSlotsItems composes the Slots-tab listing: active-profile slots,
// park, viewer AI streams, other profiles. (Archived list moved to
// dedicated Archived tab.)
func (m *Model) buildSlotsItems() []listItem {
	all := make([]listItem, 0, 32)
	slots := append([]w.SlotSpec(nil), m.snap.Slots...)
	sort.Slice(slots, func(i, j int) bool {
		if slots[i].Order != slots[j].Order {
			return slots[i].Order < slots[j].Order
		}
		return slots[i].ID < slots[j].ID
	})
	prof := m.snap.Profiles[m.snap.ActiveProfile]
	for _, sl := range slots {
		pid := prof.Assignments[sl.ID]
		assign := "(unassigned)"
		if pid != "" {
			assign = string(pid)
		}
		all = append(all, listItem{
			Kind:    itemSlot,
			Label:   string(sl.ID) + "  ws=" + string(sl.Workspace) + "  →  " + assign,
			Detail:  string(sl.Workspace),
			Slot:    sl.ID,
			Project: pid,
		})
	}
	parked := append([]w.ProjectID(nil), m.snap.Parked...)
	sort.Slice(parked, func(i, j int) bool { return parked[i] < parked[j] })
	for _, pid := range parked {
		all = append(all, listItem{
			Kind:    itemParked,
			Label:   "parked: " + string(pid),
			Detail:  "park",
			Project: pid,
		})
	}
	if m.snap.ActiveProfile != "" {
		for _, sl := range slots {
			pid, ok := prof.Assignments[sl.ID]
			if !ok || pid == "" {
				continue
			}
			all = append(all, listItem{
				Kind:    itemViewer,
				Label:   "viewer A · " + string(pid) + " ai-stream (slot=" + string(sl.ID) + ")",
				Detail:  "viewer ai-stream " + string(pid),
				Slot:    sl.ID,
				Project: pid,
			})
		}
	}
	return all
}

// buildArchivedItems composes the Archived-tab listing.
func (m *Model) buildArchivedItems() []listItem {
	out := make([]listItem, 0, len(m.snap.Archived))
	archived := append([]w.ProjectID(nil), m.snap.Archived...)
	sort.Slice(archived, func(i, j int) bool { return archived[i] < archived[j] })
	for _, pid := range archived {
		out = append(out, listItem{
			Kind:    itemArchive,
			Label:   string(pid),
			Detail:  "archived project " + string(pid),
			Project: pid,
		})
	}
	return out
}

// buildProfilesItems composes the Profiles-tab listing. Each profile is
// one row showing: active marker (★), id, inactive-policy, description,
// and the slot→project assignments inline so the user can see what
// each profile is configured for without switching.
func (m *Model) buildProfilesItems() []listItem {
	ids := make([]w.ProfileID, 0, len(m.snap.Profiles))
	for id := range m.snap.Profiles {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]listItem, 0, len(ids))
	for _, id := range ids {
		p := m.snap.Profiles[id]
		mark := " "
		if id == m.snap.ActiveProfile {
			mark = "★"
		}
		label := mark + " " + string(id)
		if p.Description != "" {
			label += "  " + p.Description
		}
		// Build a compact assignments suffix: "Q=foo W=bar".
		var assigns []string
		for _, sl := range m.snap.Slots {
			if pid, ok := p.Assignments[sl.ID]; ok && pid != "" {
				assigns = append(assigns, string(sl.ID)+"="+string(pid))
			}
		}
		sort.Strings(assigns)
		detail := "policy=" + string(p.InactivePolicy)
		if len(assigns) > 0 {
			detail += "  " + strings.Join(assigns, " ")
		} else {
			detail += "  (no assignments)"
		}
		out = append(out, listItem{
			Kind:    itemProfile,
			Label:   label,
			Detail:  detail,
			Profile: id,
		})
	}
	return out
}

// buildTraceItems composes the Trace-tab listing from the in-memory
// m.traces cache (populated by loadTracesFromDisk on tab switch / `r`).
// Most recent first.
func (m *Model) buildTraceItems() []listItem {
	out := make([]listItem, 0, len(m.traces))
	for _, t := range m.traces {
		label := t.TxID
		if t.Command != "" {
			label += "  " + t.Command
		}
		status := "ok"
		if t.Discarded {
			status = "discarded"
		} else if !t.Converged {
			status = "diverged"
		}
		detail := t.StartedAt + "  " + status
		if t.Reason != "" {
			detail += "  reason=" + t.Reason
		}
		out = append(out, listItem{
			Kind:      itemTrace,
			Label:     label,
			Detail:    detail,
			TraceID:   t.TxID,
			TracePath: t.Path,
		})
	}
	if len(out) == 0 {
		out = append(out, listItem{
			Kind:   itemTrace,
			Label:  "(no traces on disk yet — `r` to reload)",
			Detail: "trace",
		})
	}
	return out
}

func refreshDuration(s int) time.Duration {
	if s <= 0 {
		s = 2
	}
	return time.Duration(s) * time.Second
}

// loadTracesFromDisk reads the <storeDir>/traces directory, returns the
// most recent `limit` transactions (newest first). Each file is parsed
// only for the summary fields; the full JSON is loaded lazily on Enter.
// All errors collapse to an empty list so the Trace tab degrades to a
// "(no traces)" hint rather than blowing up the TUI.
func loadTracesFromDisk(storeDir string, limit int) []traceSummary {
	if storeDir == "" || limit <= 0 {
		return nil
	}
	tracesDir := filepath.Join(storeDir, "traces")
	entries, err := os.ReadDir(tracesDir)
	if err != nil {
		return nil
	}
	type entryWithMtime struct {
		name  string
		mtime time.Time
	}
	all := make([]entryWithMtime, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		all = append(all, entryWithMtime{name: e.Name(), mtime: info.ModTime()})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].mtime.After(all[j].mtime) })
	if len(all) > limit {
		all = all[:limit]
	}
	out := make([]traceSummary, 0, len(all))
	for _, e := range all {
		path := filepath.Join(tracesDir, e.name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		// Parse only the fields we render; ignore unknown fields.
		var t struct {
			TransactionID string `json:"transactionId"`
			Command       string `json:"command"`
			Reason        string `json:"reason"`
			StartedAt     string `json:"startedAt"`
			Converged     bool   `json:"converged"`
			Discarded     bool   `json:"discarded"`
		}
		if err := json.Unmarshal(data, &t); err != nil {
			continue
		}
		txID := t.TransactionID
		if txID == "" {
			txID = strings.TrimSuffix(e.name, ".json")
		}
		out = append(out, traceSummary{
			TxID:      txID,
			Command:   t.Command,
			Reason:    t.Reason,
			StartedAt: t.StartedAt,
			Converged: t.Converged,
			Discarded: t.Discarded,
			Path:      path,
		})
	}
	return out
}
