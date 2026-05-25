package scenarios

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/scenario"
)

// §3.5 IntentReconcile (S5.1, S5.2)
func TestReconcile(t *testing.T) {
	scenario.RunOnAllBackends(t, makeFixture, scenario.Scenario{
		Name: "reconcile",
		Setup: func(b *scenario.Backend) {
			_ = b.ApplyIntent(intent.Reconcile{})
		},
		Steps: []scenario.Step{
			{
				Name: "S5.1 reconcile when already converged",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyIntent(intent.Reconcile{}); err != nil {
						t.Fatal(err)
					}
				},
			},
			{
				Name: "S5.2 reconcile x N",
				Apply: func(t *testing.T, b *scenario.Backend) {
					for i := 0; i < 3; i++ {
						if err := b.ApplyIntent(intent.Reconcile{}); err != nil {
							t.Fatal(err)
						}
					}
				},
			},
		},
	})
}
