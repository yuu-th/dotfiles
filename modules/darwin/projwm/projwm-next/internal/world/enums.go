package world

// WindowKind classifies windows by capability, not by app name. design.md §5.1.
//
// WindowCockpit は projwm-next-cockpit 統合設計 (v1) で追加された window kind。
// projwm-cockpit Ghostty 窓を表す。Project にも Profile にも属さない、
// display 単位で 1 つ常駐する system-level window。Visibility 状態
// (shown=overlay 表示 / hidden=OmniWM scratchpad pool に格納) で表示制御。
type WindowKind string

const (
	WindowAI       WindowKind = "ai"
	WindowShell    WindowKind = "shell"
	WindowEditor   WindowKind = "editor"
	WindowBrowser  WindowKind = "browser"
	WindowViewer   WindowKind = "viewer"
	WindowExternal WindowKind = "external"
	WindowCockpit  WindowKind = "cockpit"
	// WindowScratch is a project/profile-independent system shell window
	// (SSOT §4.4 / §7.4 / §7.3). There is at most one in existence; its
	// identity is fixed (tmux session "projwm-scratch-shell", ghostty title
	// "projwm-scratch-shell"). Visibility is controlled via
	// intent.ShowScratchShell / intent.HideScratchShell.
	WindowScratch WindowKind = "scratch"
)

// WorkspaceRole. design.md §3.5.
type WorkspaceRole string

const (
	WorkspaceViewer  WorkspaceRole = "viewer"
	WorkspaceProject WorkspaceRole = "project"
	WorkspaceBrowser WorkspaceRole = "browser"
	WorkspaceMedia   WorkspaceRole = "media"
	WorkspaceGeneral WorkspaceRole = "general"
)

// TitleAuthority. design.md §5.4.
type TitleAuthority string

const (
	TitleControllerOwned TitleAuthority = "controller-owned"
	TitlePrefixOwned     TitleAuthority = "prefix-owned"
	TitleAppOwned        TitleAuthority = "app-owned"
	TitleUserOwned       TitleAuthority = "user-owned"
	TitleExternal        TitleAuthority = "external"
)

// TitleDriftPolicy. design.md §5.4.
type TitleDriftPolicy string

const (
	TitleDriftRepair      TitleDriftPolicy = "repair"
	TitleDriftRematch     TitleDriftPolicy = "rematch"
	TitleDriftObserveOnly TitleDriftPolicy = "observe-only"
)

// ColumnMode. design.md §5.5.
type ColumnMode string

const (
	ColumnSolo    ColumnMode = "solo"
	ColumnStacked ColumnMode = "stacked"
	ColumnTabbed  ColumnMode = "tabbed"
)

// MatchHintKind. design.md §5.4.
type MatchHintKind string

const (
	MatchByTitleRegex      MatchHintKind = "title-regex"
	MatchByTitlePrefix     MatchHintKind = "title-prefix"
	MatchByBundleID        MatchHintKind = "bundle-id"
	MatchBySessionID       MatchHintKind = "session-id"
	MatchByBrowserWindowID MatchHintKind = "browser-window-id"
)

// MatchConfidence. design.md §5.4.
type MatchConfidence string

const (
	MatchStrong MatchConfidence = "strong"
	MatchWeak   MatchConfidence = "weak"
)

// LifecycleTransactionKind. design.md §3.7.
type LifecycleTransactionKind string

const (
	LifecycleNone               LifecycleTransactionKind = ""
	LifecycleBootstrap          LifecycleTransactionKind = "bootstrap"
	LifecycleWakeRecovery       LifecycleTransactionKind = "wake-recovery"
	LifecycleDisplayReconfigure LifecycleTransactionKind = "display-reconfigure"
	LifecycleFullReconcile      LifecycleTransactionKind = "full-reconcile"
)

// LayoutAuthority. design.md §5.7.
type LayoutAuthority string

const (
	LayoutAuthorityDefault        LayoutAuthority = "default"
	LayoutAuthorityAcceptedManual LayoutAuthority = "accepted-manual"
	LayoutAuthorityImported       LayoutAuthority = "imported"
)

// AppCapability. design.md §6.
type AppCapability string

const (
	CapabilityTerminal AppCapability = "terminal"
	CapabilityEditor   AppCapability = "editor"
	CapabilityBrowser  AppCapability = "browser"
	CapabilitySession  AppCapability = "session"
	CapabilitySystem   AppCapability = "system"
)
