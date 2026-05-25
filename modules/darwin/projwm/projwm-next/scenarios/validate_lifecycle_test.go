package scenarios

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/event"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/scenario"
)

// §3.7 IntentValidateEnvironment + Lifecycle transactions (S7.1 - S7.5)
func TestValidateAndLifecycle(t *testing.T) {
	scenario.RunOnAllBackends(t, makeFixture, scenario.Scenario{
		Name: "validate-and-lifecycle",
		Setup: func(b *scenario.Backend) {
			_ = b.ApplyIntent(intent.Reconcile{})
		},
		Steps: []scenario.Step{
			{
				Name: "S7.1 LifecycleBootstrap",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyEvent(event.Event{Kind: event.KindStartup, Source: event.SourceSystem}); err != nil {
						t.Fatal(err)
					}
				},
				Command: "lifecycle:bootstrap",
			},
			{
				Name: "S7.2 LifecycleWakeRecovery",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyEvent(event.Event{Kind: event.KindWake, Source: event.SourceSystem}); err != nil {
						t.Fatal(err)
					}
				},
				Command: "lifecycle:wake-recovery",
			},
			{
				Name: "S7.3 LifecycleDisplayReconfigure",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyEvent(event.Event{Kind: event.KindDisplayChanged, Source: event.SourceSystem}); err != nil {
						t.Fatal(err)
					}
				},
				Command: "lifecycle:display-reconfigure",
			},
			{
				Name: "S7.4 LifecycleFullReconcile",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyEvent(event.Event{Kind: event.KindSafetyTimer, Source: event.SourceTimer}); err != nil {
						t.Fatal(err)
					}
				},
				Command: "lifecycle:full-reconcile",
			},
			{
				Name: "S7.5 IntentValidateEnvironment",
				Apply: func(t *testing.T, b *scenario.Backend) {
					if err := b.ApplyIntent(intent.ValidateEnvironment{}); err != nil {
						t.Fatal(err)
					}
				},
			},
		},
	})
}
