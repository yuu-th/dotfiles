package scenarios

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/scenario"
)

// SSOT §4.5: unarchive returns the project to park state; no slot is
// assigned automatically. The follow-up assignment is an explicit
// AssignProject intent.
func TestUnarchiveProject(t *testing.T) {
	scenario.RunOnAllBackends(t, makeFixture, scenario.Scenario{
		Name: "unarchive-project",
		Setup: func(b *scenario.Backend) {
			_ = b.ApplyIntent(intent.Reconcile{})
		},
		Steps: []scenario.Step{
			{
				Name: "S3.1 unarchive p4 (returns to park state)",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyIntent(intent.UnarchiveProject{Project: "p4"}); err != nil {
						t.Fatal(err)
					}
				},
			},
			{
				Name: "S3.2 unarchive p4 again (idempotent)",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyIntent(intent.UnarchiveProject{Project: "p4"}); err != nil {
						t.Fatal(err)
					}
				},
			},
			{
				Name: "S3.3 explicit slot assignment via AssignProject",
				Apply: func(t *testing.T, b *scenario.Backend) {
					_ = b.ApplyIntent(intent.UnassignSlot{Slot: "W"})
					if err := b.ApplyIntent(intent.AssignProject{Slot: "W", Project: "p4"}); err != nil {
						t.Fatal(err)
					}
				},
			},
		},
	})
}
