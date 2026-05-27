package contracts

type Layer string

const (
	LayerManifest       Layer = "manifest-schema"
	LayerStore          Layer = "persistent-store"
	LayerIPC            Layer = "ipc-single-writer"
	LayerWorld          Layer = "world-validation"
	LayerPlanner        Layer = "planner-simulator"
	LayerMutation       Layer = "mutation-safety"
	LayerBrowserPrivacy Layer = "browser-privacy"
	LayerIntegration    Layer = "real-integration"
)

type LegacySpec struct {
	ID     string
	Value  string
	Layers []Layer
}

var LegacySpecs = []LegacySpec{
	{
		ID:    "T01",
		Value: "profile switch clears inactive slot workspaces, restores active profile, lands focus on viewer workspace, and preserves isolated windows",
		Layers: []Layer{
			LayerWorld,
			LayerStore,
			LayerIPC,
			LayerPlanner,
			LayerMutation,
			LayerIntegration,
		},
	},
	{
		ID:    "T02",
		Value: "profile switch is idempotent across repeated round trips",
		Layers: []Layer{
			LayerStore,
			LayerPlanner,
			LayerIntegration,
		},
	},
	{
		ID:    "T03-T05",
		Value: "archive/unarchive removes and restores one project, updates viewer membership, and preserves focus contracts",
		Layers: []Layer{
			LayerWorld,
			LayerStore,
			LayerIPC,
			LayerPlanner,
			LayerMutation,
			LayerIntegration,
		},
	},
	{
		ID:    "T06",
		Value: "archive all projects and unarchive in slot order converges without leaking old layout or focus",
		Layers: []Layer{
			LayerWorld,
			LayerStore,
			LayerPlanner,
			LayerMutation,
			LayerIntegration,
		},
	},
	{
		ID:    "T07-T08",
		Value: "unassign/assign clears and restores slot projects with documented focus contracts",
		Layers: []Layer{
			LayerWorld,
			LayerStore,
			LayerIPC,
			LayerPlanner,
			LayerMutation,
			LayerIntegration,
		},
	},
	{
		ID:    "T09",
		Value: "profile rename is state-only and does not mutate focus or GUI layout",
		Layers: []Layer{
			LayerWorld,
			LayerStore,
			LayerIPC,
			LayerPlanner,
		},
	},
	{
		ID:    "T10",
		Value: "reconcile/status no-op reports zero diff and restores original focus",
		Layers: []Layer{
			LayerPlanner,
			LayerMutation,
			LayerIntegration,
		},
	},
	{
		ID:    "T11",
		Value: "one missing managed terminal/editor window is respawned and verified in the correct workspace without current-focus dependence",
		Layers: []Layer{
			LayerPlanner,
			LayerMutation,
			LayerIntegration,
		},
	},
	{
		ID:    "T12",
		Value: "intentional desired-window removal changes DesiredWorld through the single writer; first implementation must not perform raw live close mutation",
		Layers: []Layer{
			LayerWorld,
			LayerStore,
			LayerIPC,
			LayerMutation,
		},
	},
	{
		ID:    "T13-T17",
		Value: "rebuild/restart/wake/monitor/login lifecycle events enqueue hints and converge through projwmd, not sidecar direct mutation",
		Layers: []Layer{
			LayerManifest,
			LayerIPC,
			LayerPlanner,
			LayerIntegration,
		},
	},
	{
		ID:    "T18",
		Value: "manual accepted layout change is stored as accepted semantic layout through controller commit, not frame/title-derived raw mutation",
		Layers: []Layer{
			LayerStore,
			LayerMutation,
			LayerIntegration,
		},
	},
	{
		ID:    "T19",
		Value: "manual workspace drift is corrected only after unique-strong resolve and verify",
		Layers: []Layer{
			LayerPlanner,
			LayerMutation,
			LayerIntegration,
		},
	},
	{
		ID:    "T20",
		Value: "manual close of desired managed window is treated as missing observation and respawned via safe spawn contract",
		Layers: []Layer{
			LayerPlanner,
			LayerMutation,
			LayerIntegration,
		},
	},
	{
		ID:    "T21",
		Value: "isolated/unmanaged apps are observe-only and stable across projwm operations",
		Layers: []Layer{
			LayerManifest,
			LayerMutation,
			LayerIntegration,
		},
	},
	{
		ID:    "TF-Snapshot",
		Value: "snapshot/archive behavior is focus-independent and cannot commit unverified layout restore",
		Layers: []Layer{
			LayerStore,
			LayerMutation,
			LayerIntegration,
		},
	},
	{
		ID:    "TF-ViewerOrder",
		Value: "viewer order is derived from active profile/slot topology and remains stable from all focus states",
		Layers: []Layer{
			LayerManifest,
			LayerWorld,
			LayerPlanner,
			LayerIntegration,
		},
	},
	{
		ID:    "TF-StatusZeroDiff",
		Value: "status/diagnostics report zero diff without leaking browser URL/title content",
		Layers: []Layer{
			LayerPlanner,
			LayerBrowserPrivacy,
			LayerIntegration,
		},
	},
}
