package world

// TitleContract. design.md §5.4.
type TitleContract struct {
	Authority TitleAuthority
	Expected  string
	Prefix    string
	Drift     TitleDriftPolicy
}

// MatchHint. design.md §5.4.
type MatchHint struct {
	Kind       MatchHintKind
	Pattern    string
	Confidence MatchConfidence
}

// AppRequirement. design.md §6.
type AppRequirement struct {
	Capability AppCapability
	BundleID   string
	AppPath    string
}

// DesiredWindow. design.md §5.2.
type DesiredWindow struct {
	ID            DesiredWindowID
	Kind          WindowKind
	App           AppRequirement
	TitleContract TitleContract
	MatchHints    []MatchHint
	Browser       *DesiredBrowserSession
	// AI is set when Kind ∈ {ai, viewer}. SSOT §4.4 ai: the AI command
	// is sent via tmux send-keys on first spawn; subsequent attaches
	// reuse the running AI process. AI is not part of the window
	// identity title (title is `ai-N:project` regardless of AI name).
	AI *DesiredAISession
}

// DesiredAISession captures the AI runtime for a WindowAI / WindowViewer.
// SSOT §4.4 ai: all AIs are equal — there is no primary/default. Name
// values today are "claude" / "copilot"; the set is open and routed via
// naming.AICommand.
type DesiredAISession struct {
	Name string
}

type PrivatePayloadRef string

type BrowserSnapshotPrivacyMode string

const (
	BrowserSnapshotStructureOnly   BrowserSnapshotPrivacyMode = "structure-only"
	BrowserSnapshotRedactedContent BrowserSnapshotPrivacyMode = "redacted-content"
	BrowserSnapshotPrivateContent  BrowserSnapshotPrivacyMode = "private-content"
)

type DesiredBrowserSession struct {
	PrivacyMode       BrowserSnapshotPrivacyMode
	URLPayloadRefs    []PrivatePayloadRef
	URLCount          int
	InvalidURLCount   int
	RestoreURLs       bool
	RedactionPolicyID string
}

// DesiredColumn / DesiredLayout. design.md §5.5.
type DesiredColumn struct {
	Windows []DesiredWindowID
	Mode    ColumnMode
}

type DesiredLayout struct {
	Workspace WorkspaceID
	Columns   []DesiredColumn
	Source    LayoutAuthority
}

// DesiredProject. design.md §5.7.
type DesiredProject struct {
	ID       ProjectID
	Root     string
	Archived bool
	Windows  []DesiredWindow
	// Layout per slot workspace (key = workspace where placed when assigned).
	Layouts map[WorkspaceID]DesiredLayout
}

// InactivePolicy. Per Invariant 6 the policy is configurable; default is "remove".
type InactivePolicy string

const (
	InactivePolicyRemove InactivePolicy = "remove"
	InactivePolicyKeep   InactivePolicy = "keep"
)

type DesiredProfile struct {
	ID             ProfileID
	Description    string
	Assignments    map[SlotID]ProjectID
	InactivePolicy InactivePolicy
}

// FocusPolicySet maps Intent kind / Lifecycle kind to final-focus rule.
// design.md §14 (command policy). Concrete table is owned by projwmd internals; specs §2 only requires consistency.
type FocusPolicySet struct {
	// FinalFocus selects which workspace should hold final focus after a transaction commits.
	// Keyed by an opaque "command key" (intent kind name or lifecycle name).
	FinalFocus map[string]WorkspaceID
}

type DesiredWorld struct {
	ActiveProfile ProfileID
	Profiles      map[ProfileID]DesiredProfile
	Projects      map[ProjectID]DesiredProject
	FocusPolicy   FocusPolicySet
	// AcceptedLayouts is per-project semantic layout adopted via IntentAcceptManualLayout.
	// When set, supersedes DesiredProject.Layouts for that project on that workspace.
	AcceptedLayouts map[ProjectID]map[WorkspaceID]DesiredLayout
	// SystemWindows hold profile/project-independent residents (currently
	// just cockpit). Per the unified design v2 (park-workspace model):
	// each cockpit window is permanently bound to a dedicated park
	// workspace CPn via omniwm app-rule assignToWorkspace. Visibility
	// controls whether the display is showing CPn (Shown) or the prior
	// regular workspace (Hidden). Length is normally equal to the number
	// of connected displays; reducer maintains it on Bootstrap /
	// DisplayChanged events.
	SystemWindows []SystemWindow
}

// SystemWindow represents a system-level managed window that lives
// outside the profile/project lifecycle. Currently used only for
// cockpit per the unified design; future status-bar / dashboard
// residents would reuse this slot.
type SystemWindow struct {
	ID             SystemWindowID
	Kind           WindowKind
	DisplayIdx     int               // 0-based, stable across the daemon process lifetime
	Title          string            // controller-owned exact title, e.g. "projwm-cockpit-0"
	ParkWorkspace  WorkspaceID       // "CP{idx+1}" — fixed per-index mapping (park-workspace model)
	Visibility     CockpitVisibility // shown = display on ParkWorkspace, hidden = display on regular workspace
	PriorWorkspace WorkspaceID       // the workspace the display was on before switching to ParkWorkspace; populated only when Visibility==Shown
	PriorWindow    LiveWindowID      // the focused window before switching to ParkWorkspace; used to restore focus on hide
}

// SystemWindowID uniquely identifies a SystemWindow within DesiredWorld.
// Display index is the dispositive key so adding / removing displays
// stays referentially-stable for the surviving entries.
type SystemWindowID struct {
	Kind  WindowKind
	Index int
}

// CockpitVisibility expresses the cockpit's display state. Stored on
// every cockpit SystemWindow uniformly (we keep all displays in sync,
// per requirements §8.2 "全モニタ同期 show/hide").
type CockpitVisibility string

const (
	CockpitShown  CockpitVisibility = "shown"
	CockpitHidden CockpitVisibility = "hidden"
)
