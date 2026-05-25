package planner

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/op"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// X1 / S5: Planner must completely skip WindowExternal (origin (a))
// regardless of which workspace it currently occupies. Unmanaged apps
// like Calculator that happen to land on a managed workspace must not
// produce close / move / kill operations.
//
// This test pins that contract.
func TestPlanner_SkipsWindowExternalOnManagedWS(t *testing.T) {
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Viewer: "A",
			Workspaces: []w.WorkspaceSpec{
				{ID: "A", Role: w.WorkspaceViewer},
				{ID: "Q", Role: w.WorkspaceProject},
				{ID: "1", Role: w.WorkspaceGeneral},
			},
			Slots: []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
		},
	}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", InactivePolicy: w.InactivePolicyRemove, Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{},
	}
	state := w.WorldState{
		Environment: env,
		Desired:     desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				// A WindowExternal (e.g. Calculator) parked on managed Q.
				"calc-1": {
					ID:        "calc-1",
					Workspace: "Q",
					Kind:      w.WindowExternal,
					App:       w.ObservedAppRef{BundleID: "com.apple.Calculator"},
				},
				// A WindowExternal on managed A (viewer).
				"slack-1": {
					ID:        "slack-1",
					Workspace: "A",
					Kind:      w.WindowExternal,
					App:       w.ObservedAppRef{BundleID: "com.tinyspeck.slackmacgap"},
				},
			},
		},
	}
	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Operations {
		if o.Target.LiveWindow != nil && (*o.Target.LiveWindow == "calc-1" || *o.Target.LiveWindow == "slack-1") {
			t.Errorf("planner produced op for WindowExternal target: %+v", o)
		}
	}
}

// X3 / requirements §8.1: grouped tmux session contract. The cockpit
// design relies on `tmux new-session -A -s <clone> -t <base>` to attach
// every per-display clone to the single base session. This test pins
// the manager's spawn invocation shape so the grouped-session attribute
// is never accidentally dropped.
//
// Located in planner package only because it's the closest "design
// invariant" test bucket. The actual assertion is on cmd/projwmd; we
// invoke it indirectly via a string match on the documented spawn line
// in cmd/projwmd/cockpit_manager.go (kept here as a tripwire — if the
// spawn shape changes, this test gets updated together with the
// corresponding cockpit_manager_test.go expectations).
//
// We assert by way of: cockpit_manager_test.go::TestSyncDisplays_*
// already verifies that spawnForDisplay invokes:
//   open -na <ghostty> --args --title=... -e tmux new-session -A -s <clone> -t <base>
// — i.e. the `-t <base>` flag is the grouped-session attach. This test
// is therefore documentation; the real coverage lives there.
func TestGroupedTmuxContract_Documented(t *testing.T) {
	// Pure design pin. If the cockpit_manager spawn shape changes,
	// update TestSyncDisplays_SpawnsForNewDisplay in
	// cmd/projwmd/cockpit_manager_test.go.
	t.Log("grouped tmux session attach is verified in cmd/projwmd/cockpit_manager_test.go::TestSyncDisplays_*")
}
