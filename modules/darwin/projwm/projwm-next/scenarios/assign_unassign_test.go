package scenarios

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/scenario"
)

// §3.4 IntentAssignProject / IntentUnassignSlot (S4.1, S4.2, S4.3)
func TestAssignUnassign(t *testing.T) {
	scenario.RunOnAllBackends(t, makeFixture, scenario.Scenario{
		Name: "assign-unassign",
		Setup: func(b *scenario.Backend) {
			_ = b.ApplyIntent(intent.Reconcile{})
		},
		Steps: []scenario.Step{
			{
				Name: "S4.1 unassign W (was p2)",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyIntent(intent.UnassignSlot{Slot: "W"}); err != nil {
						t.Fatal(err)
					}
				},
			},
			{
				Name: "S4.2 assign p3 to W",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyIntent(intent.AssignProject{Slot: "W", Project: "p3"}); err != nil {
						t.Fatal(err)
					}
				},
			},
			{
				Name: "S4.3 reconcile (no-op)",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyIntent(intent.Reconcile{}); err != nil {
						t.Fatal(err)
					}
				},
			},
		},
	})
}
