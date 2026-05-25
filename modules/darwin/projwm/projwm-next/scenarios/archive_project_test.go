package scenarios

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/scenario"
)

// §3.2 IntentArchiveProject (S2.1, S2.2)
func TestArchiveProject(t *testing.T) {
	scenario.RunOnAllBackends(t, makeFixture, scenario.Scenario{
		Name: "archive-project",
		Setup: func(b *scenario.Backend) {
			_ = b.ApplyIntent(intent.Reconcile{})
		},
		Steps: []scenario.Step{
			{
				Name: "S2.1 archive p1",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyIntent(intent.ArchiveProject{Project: "p1"}); err != nil {
						t.Fatal(err)
					}
				},
			},
			{
				Name: "S2.2 reconcile after archive",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyIntent(intent.Reconcile{}); err != nil {
						t.Fatal(err)
					}
				},
			},
		},
	})
}
