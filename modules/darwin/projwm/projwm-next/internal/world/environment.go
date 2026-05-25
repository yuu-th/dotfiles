package world

// ManagedEnvironment is the environment contract authored by Nix. design.md §3, impl-design §3.
type ManagedEnvironment struct {
	SchemaVersion    int
	Authority        string // must equal "nix"
	Source           string
	MinDaemonVersion string

	WindowManager WindowManagerEnvironment
	Workspaces    WorkspaceEnvironment
	Apps          AppEnvironment
	Daemons       DaemonEnvironment
}

type WindowManagerEnvironment struct {
	Backend string // e.g. "omniwm"
	Layout  LayoutTuning
	Focus   FocusTuning
}

type LayoutTuning struct {
	DefaultColumnWidth  float64
	ColumnWidthPresets  []float64
	MaxVisibleColumns   int
	MaxWindowsPerColumn int
	CenterFocusedColumn string
	AlwaysCenterSingle  bool
}

type FocusTuning struct {
	FollowsMouse             bool
	FollowsWindowToMonitor   bool
	MoveMouseToFocusedWindow bool
}

type WorkspaceEnvironment struct {
	Workspaces []WorkspaceSpec
	Slots      []SlotSpec
	Viewer     WorkspaceID
}

type WorkspaceSpec struct {
	ID          WorkspaceID
	RawName     string
	DisplayName string
	Role        WorkspaceRole
}

type SlotSpec struct {
	ID        SlotID
	Workspace WorkspaceID
	Order     int
}

type AppEnvironment struct {
	ManagedApps []ManagedAppPolicy
}

type ManagedAppPolicy struct {
	Capability       AppCapability
	BundleID         string
	AppPath          string
	LifecycleRemoval LifecycleRemovalPolicy
}

type LifecycleRemovalMethod string

const (
	LifecycleRemovalBlocked            LifecycleRemovalMethod = "blocked"
	LifecycleRemovalAXCloseGuarded     LifecycleRemovalMethod = "ax-close-guarded"
	LifecycleRemovalProjectScopedApp   LifecycleRemovalMethod = "project-scoped-app"
	LifecycleRemovalBrowserWindowClose LifecycleRemovalMethod = "browser-window-close"
	LifecycleRemovalSessionTerminate   LifecycleRemovalMethod = "session-terminate"
)

type LifecycleRemovalPolicy struct {
	Allowed          bool
	Method           LifecycleRemovalMethod
	AllowedKinds     []WindowKind
	RequiredEvidence []string
}

type DaemonEnvironment struct {
	ControllerLabel string
	SocketPath      string
	LegacyAgents    []LegacyAgentPolicy
	EventSources    []EventSourceSpec
}

type LegacyAgentPolicy struct {
	Label  string
	Action string // "remove" or "report"
}

type EventSourceSpec struct {
	Kind      string
	Source    string
	Mode      string
	Authority string
	Label     string
}

func (e ManagedEnvironment) ManagedAppByBundle(bundleID string) (ManagedAppPolicy, bool) {
	for _, app := range e.Apps.ManagedApps {
		if app.BundleID == bundleID {
			return app, true
		}
	}
	return ManagedAppPolicy{}, false
}
