package scenarios

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/scenario"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// §4 external event reactions (§4.1 forced-close, §4.2 cross-ws move, §4.3 user-close,
// §4.4 same-ws reorder, §4.5 isolated apps).
func TestExternalEventReactions(t *testing.T) {
	scenario.RunOnAllBackends(t, makeFixture, scenario.Scenario{
		Name: "external-events",
		Setup: func(b *scenario.Backend) {
			_ = b.ApplyIntent(intent.Reconcile{})
		},
		Steps: []scenario.Step{
			{
				Name: "§4.1 force-close one managed window then reconcile",
				Apply: func(t *testing.T, b *scenario.Backend) {
					// Find a managed window (p1 ai) and close it externally.
					obs := b.Controller.State().Observed
					var victim w.LiveWindowID
					for id, ow := range obs.Windows {
						if ow.MatchedTo != nil && ow.MatchedTo.Project == "p1" && ow.MatchedTo.Kind == w.WindowAI {
							victim = id
							break
						}
					}
					if victim != "" {
						b.Fake.SimulateUserClose(victim)
					}
					if err := b.ApplyIntent(intent.Reconcile{}); err != nil {
						t.Fatal(err)
					}
				},
			},
			{
				Name: "§4.2 user moves window cross-workspace then reconcile",
				Apply: func(t *testing.T, b *scenario.Backend) {
					obs := b.Controller.State().Observed
					var victim w.LiveWindowID
					for id, ow := range obs.Windows {
						if ow.MatchedTo != nil && ow.MatchedTo.Project == "p1" && ow.MatchedTo.Kind == w.WindowEditor {
							victim = id
							break
						}
					}
					if victim != "" {
						b.Fake.SimulateUserMove(victim, "ws-w")
					}
					if err := b.ApplyIntent(intent.Reconcile{}); err != nil {
						t.Fatal(err)
					}
				},
			},
			{
				Name: "§4.5 external app on general workspace stays untouched",
				Apply: func(t *testing.T, b *scenario.Backend) {
					b.Fake.InjectExternalWindow("ext-1", "ws-other", "Random App", "com.unknown.app")
					if err := b.ApplyIntent(intent.Reconcile{}); err != nil {
						t.Fatal(err)
					}
					obs := b.Controller.State().Observed
					if _, ok := obs.Windows["ext-1"]; !ok {
						t.Fatalf("external window was incorrectly removed by controller")
					}
				},
			},
		},
	})
}
