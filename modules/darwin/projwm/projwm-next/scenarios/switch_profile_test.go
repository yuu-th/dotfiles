package scenarios

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/scenario"
)

// §3.1 IntentSwitchProfile (S1.1 - S1.4)
func TestSwitchProfile(t *testing.T) {
	scenario.RunOnAllBackends(t, makeFixture, scenario.Scenario{
		Name: "switch-profile",
		Setup: func(b *scenario.Backend) {
			// bootstrap to active profile A
			_ = b.ApplyIntent(intent.Reconcile{})
		},
		Steps: []scenario.Step{
			{
				Name: "S1.1 A->B",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyIntent(intent.SwitchProfile{To: "B"}); err != nil {
						t.Fatal(err)
					}
				},
			},
			{
				Name: "S1.2 B->B re-issue",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyIntent(intent.SwitchProfile{To: "B"}); err != nil {
						t.Fatal(err)
					}
				},
			},
			{
				Name: "S1.3 B->A",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyIntent(intent.SwitchProfile{To: "A"}); err != nil {
						t.Fatal(err)
					}
				},
			},
			{
				Name: "S1.4 A->E (empty)",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyIntent(intent.SwitchProfile{To: "E"}); err != nil {
						t.Fatal(err)
					}
				},
			},
		},
	})
}
