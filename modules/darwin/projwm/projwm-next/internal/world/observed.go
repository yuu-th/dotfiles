package world

// ObservedTitle. design.md §5.4.
type ObservedTitle struct {
	Value string
}

// ObservedAppRef. design.md §5.3.
type ObservedAppRef struct {
	BundleID string
	AppPath  string
}

// Frame is observation only. design.md §5.5.
type Frame struct {
	X, Y, W, H float64
}

// ObservedWindow. design.md §5.3.
type ObservedWindow struct {
	ID         LiveWindowID
	App        ObservedAppRef
	Title      ObservedTitle
	Workspace  WorkspaceID
	Visibility string
	Focused    bool
	Frame      *Frame
	// MatchedTo is a hint set by the identity resolver. Not an authoritative truth.
	MatchedTo *DesiredWindowID
	// Kind classification (filled by identity resolver / admission).
	Kind WindowKind
	// SystemMatchedTo links an observed window back to a SystemWindow
	// (when set). Distinct from MatchedTo which targets project DesiredWindows.
	SystemMatchedTo *SystemWindowID
}

// ObservedColumn. design.md §5.5.
type ObservedColumn struct {
	Windows []LiveWindowID
	Mode    ColumnMode
}

// ObservedLayout. design.md §5.5.
type ObservedLayout struct {
	Workspace WorkspaceID
	Columns   []ObservedColumn
}

// ObservedFocus.
type ObservedFocus struct {
	Workspace WorkspaceID
	Window    LiveWindowID
}

// ObservedDisplay.
type ObservedDisplay struct {
	ID              DisplayID
	Connected       bool
	// ActiveWorkspace is the workspace currently displayed on this monitor.
	// Populated by sigwm's queryDisplays. Used by the planner to detect
	// show/hide convergence for cockpit park-workspace model (unified design v2).
	ActiveWorkspace WorkspaceID
}

type ObservedDisplayState struct {
	Displays map[DisplayID]ObservedDisplay
	Primary  *DisplayID
	// WorkspaceToDisplay maps a WorkspaceID to the DisplayID that currently
	// owns it. Populated by sigwm.Observe from live window placement and
	// display-activeWorkspace data. Used by the planner/reducer/executor to
	// resolve DisplayIdx → DisplayID via park-workspace ownership rather than
	// alphabetical sort, which breaks when display IDs are not contiguous.
	WorkspaceToDisplay map[WorkspaceID]DisplayID
}

type ObservedWorkspace struct {
	ID   WorkspaceID
	Role WorkspaceRole
}

// SSOT N-12 (2026-05-20): ManualLayoutCandidate type removed. The
// equivalent semantics are now expressed via intent.AutoSyncLayout (a
// controller-internal intent emitted in response to layout-sync
// DirtyScopes).

// CardType discriminates ActiveCards. Mirrors requirements §10.
type CardType string

const (
	CardTypeNew            CardType = "NEW"
	CardTypeClosed         CardType = "CLOSED"
	CardTypeMoved          CardType = "MOVED"
	CardTypeReplan         CardType = "REPLAN"
	CardTypeInvariant      CardType = "INVARIANT"
	CardTypeManifest       CardType = "MANIFEST"
	CardTypeOrphan         CardType = "ORPHAN"
	CardTypeOmniwmRecovery CardType = "OMNIWM-RECOVERY"
)

// CardID is an in-memory identifier for a Card. Not persisted across
// daemon restart — cards regenerate from re-observation.
type CardID string

// CardAction is one key-bound action attached to a Card.
type CardAction struct {
	Key     string // "Enter", "c", "t", "Esc", ...
	Label   string
	Intent  string // intent.Kind that the cockpit submits when the user
	         // selects this action. May be empty for inert acks.
}

// Card is a cockpit-displayable proposal/notification.
type Card struct {
	ID        CardID
	Type      CardType
	Subject   string
	Context   map[string]string
	Actions   []CardAction
	CreatedAt int64 // unix nano; controller fills in
}

// OrphanCandidate tracks a not-yet-cardified live window that has been
// observed inside a managed workspace and may need to become a [NEW] card.
type OrphanCandidate struct {
	LiveID     LiveWindowID
	Kind       WindowKind
	Workspace  WorkspaceID
	BundleID   string
	Title      string // observed title; cockpit uses it for project-suggestion heuristics
	DetectedAt int64  // unix nano
}

type ObservedWorld struct {
	Displays   ObservedDisplayState
	Workspaces map[WorkspaceID]ObservedWorkspace
	Windows    map[LiveWindowID]ObservedWindow
	Layouts    map[WorkspaceID]ObservedLayout
	Focus      ObservedFocus
}

// PredictedWorld.
type PredictedWorld struct {
	ObservedWorld
	BasedOnEpoch Epoch
}
