// Package intent declares the user-visible intent surface.
//
// SSOT: queue/projwm-next-spec.md §4.1 — 17 user operations + §4.3 orphan
// actions + system-level intents for state-ownership (assign, profile CRUD,
// project deletion, card dismissal) and internal controller helpers
// (auto-sync, cockpit sync).
//
// Anything outside this surface is either a deprecated intent kept until the
// reducer/planner Phase 0.3 cleanup, or an event (see internal/event).
package intent

import (
	w "github.com/yuu-th/projwm-next/internal/world"
)

type Kind string

const (
	// --------------------------------------------------------------------
	// SSOT §4.1 — 17 user operations
	// --------------------------------------------------------------------

	// Operation 1-3: summon the slot's specific kind window. Daemon resolves
	// slot → project via ActiveProfile.Assignments and either focuses the
	// existing kind-window or, on repeat press inside the same slot, cycles
	// to the next kind-window of the same project.
	KindSummonShell   Kind = "summon-shell"
	KindSummonEditor  Kind = "summon-editor"
	KindSummonBrowser Kind = "summon-browser"

	// Operation 4: switch project — focus the slot's last-focused managed
	// window. Trigger shares the slot key with operation 1-3; the karabiner /
	// hotkey layer chooses which intent to emit based on whether the active
	// workspace already matches the target slot.
	KindSwitchProject Kind = "switch-project"

	// Operation 5: cycle within the current slot for a given window kind.
	KindCycleSlotWindow Kind = "cycle-slot-window"

	// Operation 6: viewer jump — focus workspace A, restoring the last
	// focused viewer.
	KindSummonViewer Kind = "summon-viewer"

	// Operation 7: cockpit show / hide. Visibility=Shown switches the
	// projwm-managed display to CP1; Hidden returns to PriorWorkspace.
	KindSetCockpitVisibility Kind = "set-cockpit-visibility"

	// Operation 8: profile switch.
	KindSwitchProfile Kind = "switch-profile"

	// Operation 9: project add (with optional slot pre-assignment).
	KindCreateProject Kind = "create-project"

	// Operation 10: archive / unarchive.
	KindArchiveProject   Kind = "archive-project"
	KindUnarchiveProject Kind = "unarchive-project"

	// Operation 11: scratch shell.
	KindShowScratchShell Kind = "show-scratch-shell"
	KindHideScratchShell Kind = "hide-scratch-shell"

	// Operation 12-13: per-window add / remove.
	KindAddWindow    Kind = "add-window"
	KindRemoveWindow Kind = "remove-window"

	// Operation 14-17: browser tab CRUD + reorder.
	KindBrowserAddTab       Kind = "browser-add-tab"
	KindBrowserRemoveTab    Kind = "browser-remove-tab"
	KindBrowserChangeTabURL Kind = "browser-change-tab-url"
	KindBrowserReorderTabs  Kind = "browser-reorder-tabs"

	// --------------------------------------------------------------------
	// SSOT §6.4 — state-ownership intents (cockpit / CLI surface §5.7)
	// --------------------------------------------------------------------

	KindAssignProject Kind = "assign-project"
	KindUnassignSlot  Kind = "unassign-slot"

	KindCreateProfile Kind = "create-profile"
	KindDeleteProfile Kind = "delete-profile"
	KindRenameProfile Kind = "rename-profile"

	KindDeleteProject Kind = "delete-project"

	// --------------------------------------------------------------------
	// SSOT §4.3 — orphan card actions + card dismissal
	// --------------------------------------------------------------------

	KindAdoptOrphanWindow   Kind = "adopt-orphan-window"   // [Enter]
	KindDismissOrphanWindow Kind = "dismiss-orphan-window" // [c]
	KindDismissCard         Kind = "dismiss-card"
	KindDismissAllCards     Kind = "dismiss-all-cards"

	// --------------------------------------------------------------------
	// SSOT §2.3 / §7.1 — system convergence intents
	// --------------------------------------------------------------------

	KindReconcile           Kind = "reconcile"
	KindValidateEnvironment Kind = "validate-environment"

	// --------------------------------------------------------------------
	// Internal intents — controller submits to itself. Not part of §4.1.
	// --------------------------------------------------------------------

	// AutoSyncLayout: Tier 2 same-workspace column reorder feedback.
	KindAutoSyncLayout Kind = "auto-sync-layout"

	// SyncCockpitSystemWindows: display-topology bookkeeping for §3.4 INV-06.
	KindSyncCockpitSystemWindows Kind = "sync-cockpit-system-windows"

)

// Intent is the union type.
type Intent interface {
	Kind() Kind
}

// ---------------------------------------------------------------------------
// SSOT §4.1 operations 1-7
// ---------------------------------------------------------------------------

// SummonShell — operation 1.
type SummonShell struct {
	Slot w.SlotID
}

func (SummonShell) Kind() Kind { return KindSummonShell }

// SummonEditor — operation 2.
type SummonEditor struct {
	Slot w.SlotID
}

func (SummonEditor) Kind() Kind { return KindSummonEditor }

// SummonBrowser — operation 3.
type SummonBrowser struct {
	Slot w.SlotID
}

func (SummonBrowser) Kind() Kind { return KindSummonBrowser }

// SwitchProject — operation 4.
type SwitchProject struct {
	Slot w.SlotID
}

func (SwitchProject) Kind() Kind { return KindSwitchProject }

// CycleSlotWindow — operation 5.
type CycleSlotWindow struct {
	Slot       w.SlotID
	WindowKind w.WindowKind
}

func (CycleSlotWindow) Kind() Kind { return KindCycleSlotWindow }

// SummonViewer — operation 6. Payload-less; daemon restores last-focused viewer.
type SummonViewer struct{}

func (SummonViewer) Kind() Kind { return KindSummonViewer }

// SetCockpitVisibility — operation 7.
type SetCockpitVisibility struct {
	Visibility w.CockpitVisibility
}

func (SetCockpitVisibility) Kind() Kind { return KindSetCockpitVisibility }

// ---------------------------------------------------------------------------
// SSOT §4.1 operations 8-13
// ---------------------------------------------------------------------------

// SwitchProfile — operation 8.
type SwitchProfile struct {
	To w.ProfileID
}

func (SwitchProfile) Kind() Kind { return KindSwitchProfile }

// CreateProject — operation 9.
//
// Slot is optional: when zero the project enters park state (per SSOT §4.5
// unarchive semantics, projects are not auto-assigned to a slot). Windows
// is optional: when empty the reducer fills in a default ai+shell+editor.
type CreateProject struct {
	ID      w.ProjectID
	Path    string
	Slot    w.SlotID
	Windows []w.DesiredWindow
}

func (CreateProject) Kind() Kind { return KindCreateProject }

// ArchiveProject — operation 10 (archive half).
type ArchiveProject struct {
	Project w.ProjectID
}

func (ArchiveProject) Kind() Kind { return KindArchiveProject }

// UnarchiveProject — operation 10 (unarchive half).
//
// SSOT §4.5: unarchive returns the project to park state. No slot assignment
// is performed; the user must explicitly assign via AssignProject afterwards.
type UnarchiveProject struct {
	Project w.ProjectID
}

func (UnarchiveProject) Kind() Kind { return KindUnarchiveProject }

// ShowScratchShell / HideScratchShell — operation 11.
type ShowScratchShell struct{}

func (ShowScratchShell) Kind() Kind { return KindShowScratchShell }

type HideScratchShell struct{}

func (HideScratchShell) Kind() Kind { return KindHideScratchShell }

// AddWindow — operation 12.
//
// WindowKind ∈ {ai, shell, editor, browser}. AIName is required for
// WindowKind==ai (used by the tmux session to send the AI launch command on
// first spawn; see SSOT §4.4 ai). Index is the desired ordinal; <=0 means
// "auto-pick next unused" per the §4.4 id-allocation rule.
type AddWindow struct {
	Project    w.ProjectID
	WindowKind w.WindowKind
	Index      int
	AIName     string
}

func (AddWindow) Kind() Kind { return KindAddWindow }

// RemoveWindow — operation 13.
type RemoveWindow struct {
	Project  w.ProjectID
	WindowID w.DesiredWindowID
}

func (RemoveWindow) Kind() Kind { return KindRemoveWindow }

// ---------------------------------------------------------------------------
// SSOT §4.1 operations 14-17 — browser tab CRUD
// ---------------------------------------------------------------------------

// BrowserAddTab — operation 14. URL body is stored in PrivatePayloadStore;
// only opaque ref + URLCount land in DesiredWorld.
type BrowserAddTab struct {
	Project  w.ProjectID
	WindowID w.DesiredWindowID
	URL      string
}

func (BrowserAddTab) Kind() Kind { return KindBrowserAddTab }

// BrowserRemoveTab — operation 15. Tab index is 1-based per SSOT example.
// Removing the last tab closes the browser window (planner-level decision).
type BrowserRemoveTab struct {
	Project  w.ProjectID
	WindowID w.DesiredWindowID
	Tab      int
}

func (BrowserRemoveTab) Kind() Kind { return KindBrowserRemoveTab }

// BrowserChangeTabURL — operation 16.
type BrowserChangeTabURL struct {
	Project  w.ProjectID
	WindowID w.DesiredWindowID
	Tab      int
	URL      string
}

func (BrowserChangeTabURL) Kind() Kind { return KindBrowserChangeTabURL }

// BrowserReorderTabs — operation 17. Observed when the user reorders tabs
// inside Vivaldi; system auto-syncs (SSOT §6.3 Level 3 auto-overwrite).
type BrowserReorderTabs struct {
	Project  w.ProjectID
	WindowID w.DesiredWindowID
	From     int
	To       int
}

func (BrowserReorderTabs) Kind() Kind { return KindBrowserReorderTabs }

// ---------------------------------------------------------------------------
// State-ownership intents (CLI / cockpit, not in §4.1 but required by §6.4)
// ---------------------------------------------------------------------------

type AssignProject struct {
	Slot    w.SlotID
	Project w.ProjectID
}

func (AssignProject) Kind() Kind { return KindAssignProject }

type UnassignSlot struct {
	Slot w.SlotID
}

func (UnassignSlot) Kind() Kind { return KindUnassignSlot }

type CreateProfile struct {
	ID             w.ProfileID
	Description    string
	InactivePolicy w.InactivePolicy
}

func (CreateProfile) Kind() Kind { return KindCreateProfile }

type DeleteProfile struct {
	ID w.ProfileID
}

func (DeleteProfile) Kind() Kind { return KindDeleteProfile }

type RenameProfile struct {
	Old w.ProfileID
	New w.ProfileID
}

func (RenameProfile) Kind() Kind { return KindRenameProfile }

// DeleteProject — final removal after archive. Purge=true (default
// semantics) drops PrivatePayloadStore artifacts as well.
type DeleteProject struct {
	ID    w.ProjectID
	Purge bool
}

func (DeleteProject) Kind() Kind { return KindDeleteProject }

// ---------------------------------------------------------------------------
// SSOT §4.3 — orphan card actions + card dismissal
// ---------------------------------------------------------------------------

// AdoptOrphanWindow — §4.3 [Enter]: register the orphan as a new project +
// slot assignment in one step.
type AdoptOrphanWindow struct {
	LiveID       w.LiveWindowID
	AsProject    w.ProjectID
	AsWindowKind w.WindowKind
}

func (AdoptOrphanWindow) Kind() Kind { return KindAdoptOrphanWindow }

// DismissOrphanWindow — §4.3 [c]: close the orphan without adopting.
type DismissOrphanWindow struct {
	LiveID w.LiveWindowID
	Action string // currently always "close"
}

func (DismissOrphanWindow) Kind() Kind { return KindDismissOrphanWindow }

// CardID is the in-memory handle for an ActiveCard.
type CardID string

type DismissCard struct {
	CardID CardID
}

func (DismissCard) Kind() Kind { return KindDismissCard }

type DismissAllCards struct{}

func (DismissAllCards) Kind() Kind { return KindDismissAllCards }

// ---------------------------------------------------------------------------
// System convergence
// ---------------------------------------------------------------------------

type Reconcile struct{}

func (Reconcile) Kind() Kind { return KindReconcile }

type ValidateEnvironment struct{}

func (ValidateEnvironment) Kind() Kind { return KindValidateEnvironment }

// ---------------------------------------------------------------------------
// Internal intents (controller-to-self)
// ---------------------------------------------------------------------------

// AutoSyncLayout — Tier 2 same-workspace column reorder feedback. Preserves
// single-writer: external events emit a DirtyScope; controller converts to
// AutoSyncLayout; reducer writes DesiredWorld.AcceptedLayouts.
type AutoSyncLayout struct {
	Project   w.ProjectID
	Workspace w.WorkspaceID
	Columns   []w.DesiredColumn
}

func (AutoSyncLayout) Kind() Kind { return KindAutoSyncLayout }

// SyncCockpitSystemWindows — display-topology refresh. Keeps
// DesiredWorld.SystemWindows length at 1 cockpit entry on the
// projwm-managed display (SSOT §3.4 INV-06).
type SyncCockpitSystemWindows struct {
	DisplayCount int
}

func (SyncCockpitSystemWindows) Kind() Kind { return KindSyncCockpitSystemWindows }

// SSOT N-06 / N-12 alignment: ToggleCockpit, FocusCockpit,
// AcceptManualLayout, SyncBrowserTabs, RespawnOrphanGhostty have been
// removed. Their behaviours are replaced respectively by
// SetCockpitVisibility{Shown} (N-06), AutoSyncLayout (N-12), the
// granular Browser*Tab intents (§4.1 OP14-17), and AdoptOrphanWindow
// (§4.3 [Enter]).
