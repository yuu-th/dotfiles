package controller

import (
	"context"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestSSOTL1FakeTransactionLoopCommitsConvergedReconcile(t *testing.T) {
	desired := w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{},
	}
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{{ID: "A", RawName: "A", Role: w.WorkspaceViewer}},
		},
	}
	st := store.NewMemoryStore(desired)
	ctrl := New(env, desired, wm.NewFake(env), st)

	result, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{})
	if err != nil {
		t.Fatalf("SSOT L1 fake transaction loop reconcile: %v", err)
	}
	if result.CommittedGeneration == "" || !result.Trace.Converged {
		t.Fatalf("SSOT L1 fake transaction loop did not commit converged trace: %+v", result)
	}
}
