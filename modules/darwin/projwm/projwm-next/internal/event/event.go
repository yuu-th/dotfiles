// Package event declares external event types. design.md §9.2, §3.
package event

import w "github.com/yuu-th/projwm-next/internal/world"

type Source string

const (
	SourceUser       Source = "user"
	SourceWindowMgr  Source = "window-manager"
	SourceSystem     Source = "system"
	SourceTimer      Source = "timer"
	SourceController Source = "controller"
)

type Authority string

const (
	AuthorityHint     Authority = "hint"
	AuthorityEvidence Authority = "evidence"
)

type Kind string

const (
	KindWindowsChanged       Kind = "windows-changed"
	KindLayoutChanged        Kind = "layout-changed"
	KindFocusChanged         Kind = "focus-changed"
	KindDisplayChanged       Kind = "display-changed"
	KindWake                 Kind = "wake"
	KindStartup              Kind = "startup"
	KindSafetyTimer          Kind = "safety-timer"
	KindLegacyAgentDetected  Kind = "legacy-agent-detected"
	KindUserMovedWindow      Kind = "user-moved-window"
	KindUserClosedWindow     Kind = "user-closed-window"
	KindUserReorderedColumns Kind = "user-reordered-columns"
)

// Event. design.md §9.2.
type Event struct {
	ID        w.EventID
	Source    Source
	Authority Authority
	Kind      Kind
	Epoch     w.Epoch
	Data      Data
}

// Data carries kind-specific payload.
type Data struct {
	Window         *w.LiveWindowID
	Workspace      *w.WorkspaceID
	Project        *w.ProjectID
	Columns        []w.DesiredColumn
	LegacyAgent    string
	TargetWS       *w.WorkspaceID
}

// UserCloseRecord captures one user-close event for the 60s rate limiter.
// Controller appends these into ControllerMeta.UserCloseHistory.
type UserCloseRecord struct {
	Window    w.LiveWindowID
	DesiredID w.DesiredWindowID
	At        int64
}

// Reaction. design.md §13. SSOT N-12 (2026-05-20): ManualLayoutCandidate
// field removed; user-reordered-columns events emit a layout-sync
// DirtyScope which the controller converts to AutoSyncLayout intent.
type Reaction struct {
	DirtyScopes   []w.DirtyScope
	ObserveScopes []w.WorldScope
	Lifecycle     w.LifecycleTransactionKind
	// NewCards is appended to ControllerMeta.ActiveCards after the reaction
	// is applied. Tier 4 [MOVED] / [CLOSED] cards are emitted here.
	NewCards []w.Card
	// OrphanAdds is appended to ControllerMeta.PendingOrphans. Used by
	// Tier 1 detection (5-second grace window).
	OrphanAdds []w.OrphanCandidate
	// UserCloseRecords is appended to ControllerMeta.UserCloseHistory.
	// Used by the T4.4 60-second 2-close rate limiter.
	UserCloseRecords []UserCloseRecord
	// Discard signals stale-epoch / external workspace events that should be dropped or kept as evidence only.
	Discard bool
}
