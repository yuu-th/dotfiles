// Package op declares the Operation contract. design.md §10.
package op

import (
	w "github.com/yuu-th/projwm-next/internal/world"
)

type Kind string

const (
	KindObserveWorld            Kind = "observe-world"
	KindEnsureSession           Kind = "ensure-session"
	KindKillSession             Kind = "kill-session"
	KindSpawnTerminal           Kind = "spawn-terminal"
	KindSpawnEditor             Kind = "spawn-editor"
	KindSpawnBrowser            Kind = "spawn-browser"
	KindSpawnViewer             Kind = "spawn-viewer"
	KindCloseWindow             Kind = "close-window"
	KindFocusWorkspace          Kind = "focus-workspace"
	KindFocusWindow             Kind = "focus-window"
	KindMoveWindowToWorkspace   Kind = "move-window-to-workspace"
	KindMoveColumn              Kind = "move-column"
	KindMoveStackMember         Kind = "move-stack-member"
	KindToggleTabbed            Kind = "toggle-tabbed"
	KindReorderColumns          Kind = "reorder-columns"
	KindAcceptLayoutObservation Kind = "accept-layout-observation"
	KindValidateEnvironment     Kind = "validate-environment"
	// KindObserveBarrier is a non-mutating sequencing primitive emitted by the
	// planner between distinct mutation phases (removal | spawn | layout).
	// The executor handles it by sleeping briefly and then re-querying the
	// adapter, which forces the next operation's precondition check to run
	// against fresh evidence rather than the observation captured before any
	// mutation in the current phase. This is a production primitive (not
	// test-only): production daemons need the barrier just as much as test
	// scenarios because OmniWM/AX disappearance/appearance propagation is not
	// strictly synchronous with the close/spawn syscall.
	// design.md §10, specs §7.
	KindObserveBarrier Kind = "observe-barrier"

	// Cockpit operations (unified design v2 — park-workspace model).
	// KindSpawnCockpit: spawn a per-display cockpit Ghostty (open -na +
	// tmux grouped clone of `projwm-cockpit` base session). The window
	// is permanently bound to its CPn park workspace via omniwm app-rule
	// assignToWorkspace, so it never occupies a regular workspace slot.
	KindSpawnCockpit Kind = "spawn-cockpit"
	// KindCloseCockpit: kill the tmux clone session backing a leftover
	// cockpit Ghostty (display unplug or DisplayCount shrink). Unlike
	// KindCloseWindow this is not subject to the close-block safety
	// matrix because cockpit windows hold no user data and are
	// planner-owned (unified design v2 §6.5).
	KindCloseCockpit Kind = "close-cockpit"
	// KindShowCockpit: switch a specific display's active workspace to
	// its CPn park workspace, making the cockpit window visible.
	// Target.Workspace = ParkWorkspace; Target.SystemWindow identifies the display.
	KindShowCockpit Kind = "show-cockpit"
	// KindHideCockpit: switch a specific display's active workspace back
	// from CPn to PriorWorkspace, hiding the cockpit.
	// Target.Workspace = PriorWorkspace; Target.SystemWindow identifies the display.
	KindHideCockpit Kind = "hide-cockpit"
	// KindMoveCockpitToParkWorkspace: force-move a cockpit Ghostty window
	// back to its ParkWorkspace (CP1) when observed.workspace has drifted
	// off the park (e.g., omniwm restart, manual drag, app-rule miss).
	// Realises requirements v2.8 §8.10 cockpit invariant: cockpit window
	// is **always** on its ParkWorkspace; violations are Tier 4 forced
	// reverts. Target.LiveWindow = the cockpit ghostty window id;
	// Target.Workspace = ParkWorkspace.
	KindMoveCockpitToParkWorkspace Kind = "move-cockpit-to-park"
)

// PreconditionKind. design.md §10.2.
type PreconditionKind string

const (
	PreWindowExists      PreconditionKind = "window-exists"
	PreWorkspaceExists   PreconditionKind = "workspace-exists"
	PreAnchorVisible     PreconditionKind = "anchor-visible"
	PreColumnBudget      PreconditionKind = "column-budget"
	PreStackCapacity     PreconditionKind = "stack-capacity"
	PreAdapterCapability PreconditionKind = "adapter-capability"
	PreEnvironmentReady  PreconditionKind = "environment-ready"
	// PreUniqueStrong asserts the resolver classifies the target as unique-strong.
	// design.md §7.1, specs §2-B.
	PreUniqueStrong PreconditionKind = "identity-unique-strong"
)

type Precondition struct {
	Kind PreconditionKind
	// Target is the operation target this precondition applies to.
	Target Target
}

// RiskClass. design.md §10.
type RiskClass string

const (
	RiskLow    RiskClass = "low"
	RiskMedium RiskClass = "medium"
	RiskHigh   RiskClass = "high"
)

// SettlePolicy. design.md §10.
type SettlePolicy struct {
	// Stable count semantics: settler waits until target observation is stable for N consecutive polls.
	StableCount int
	TimeoutMS   int
}

// Target identifies what the operation acts upon.
type Target struct {
	// One of these is set depending on Kind.
	DesiredWindow *w.DesiredWindowID
	LiveWindow    *w.LiveWindowID
	Workspace     *w.WorkspaceID
	Project       *w.ProjectID
	SystemWindow  *w.SystemWindowID // cockpit spawn ops
}

// Effect declares an expected effect on PredictedWorld.
type Effect struct {
	Kind       EffectKind
	Window     *w.LiveWindowID
	Desired    *w.DesiredWindowID
	Workspace  *w.WorkspaceID
	Columns    []w.DesiredColumn
	WindowKind w.WindowKind
	FocusedWS  *w.WorkspaceID
	FocusedWin *w.LiveWindowID
	// SystemWindow is set when the spawn target is a SystemWindow
	// (cockpit overlay) rather than a project DesiredWindow. The
	// simulator treats this as a Desired-less spawn: it produces a
	// predicted ObservedWindow keyed off the SystemWindowID instead
	// of validating Desired/Workspace.
	SystemWindow *w.SystemWindowID
}

type EffectKind string

const (
	EffectSpawnWindow       EffectKind = "spawn-window"
	EffectCloseWindow       EffectKind = "close-window"
	EffectMoveWindow        EffectKind = "move-window"
	EffectFocusWorkspace    EffectKind = "focus-workspace"
	EffectFocusWindow       EffectKind = "focus-window"
	EffectReorderColumns    EffectKind = "reorder-columns"
	EffectAcceptObservation EffectKind = "accept-observation"
)

// Operation. design.md §10.
type Operation struct {
	ID              w.OperationID
	Kind            Kind
	Scope           []w.WorldScope
	Target          Target
	Preconditions   []Precondition
	ExpectedEffects []Effect
	Settle          SettlePolicy
	Risk            RiskClass
	IdempotencyKey  string
	// LifecycleRemovalMethod records the app-contract method for KindKillSession
	// so audits do not hide guarded AX close behind a semantic operation name.
	LifecycleRemovalMethod w.LifecycleRemovalMethod
}

// PlanReason describes why a plan was generated.
type PlanReason string

const (
	ReasonIntent    PlanReason = "intent"
	ReasonReconcile PlanReason = "reconcile"
	ReasonLifecycle PlanReason = "lifecycle"
	ReasonEvent     PlanReason = "event"
)

// Plan. design.md §11.
type Plan struct {
	ID         w.PlanID
	BaseEpoch  w.Epoch
	Scope      []w.WorldScope
	Operations []Operation
	Reason     PlanReason
}
