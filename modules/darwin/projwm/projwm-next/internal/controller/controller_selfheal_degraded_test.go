package controller

import (
	"context"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// P1 self-recovery (2026-06-08 degraded-IPC incident). The degraded flag
// (IsDegraded) is set when a transaction exhausts MaxReplans without converging
// and cleared on the next converged commit. runOmniwmRecoveryTicker reads it to
// RE-DRIVE a startup transaction once OmniWM is healthy again, so the daemon
// always leaves "serving degraded IPC" once conditions clear instead of staying
// stuck forever (the ticker previously only health-probed, never re-drove).

func selfhealEnv() w.ManagedEnvironment {
	return w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Slots:      []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
			Workspaces: []w.WorkspaceSpec{{ID: "Q", RawName: "Q", DisplayName: "Q", Role: w.WorkspaceProject}},
		},
	}
}

func TestControllerIsDegraded_SetOnMaxReplans(t *testing.T) {
	env := selfhealEnv()
	shell := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	desired := w.DesiredWorld{
		ActiveProfile: "default",
		Profiles:      map[w.ProfileID]w.DesiredProfile{"default": {ID: "default", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}}},
		Projects:      map[w.ProjectID]w.DesiredProject{"p1": {ID: "p1", Windows: []w.DesiredWindow{{ID: shell, Kind: w.WindowShell}}}},
	}
	ctrl := New(env, desired, &neverSpawnsAdapter{Fake: wm.NewFake(env)}, store.NewMemoryStore(desired))
	ctrl.MaxReplans = 2

	if ctrl.IsDegraded() {
		t.Fatal("fresh controller must not report degraded")
	}
	// Spawn always fails → never converges → MaxReplans exhausted.
	_, _ = ctrl.ApplyIntent(context.Background(), intent.Reconcile{})
	if !ctrl.IsDegraded() {
		t.Fatal("a max-replans (non-converged) transaction must set IsDegraded()=true so the self-heal ticker re-drives")
	}
}

func TestControllerIsDegraded_ClearedOnConverge(t *testing.T) {
	env := selfhealEnv()
	// Empty profile → nothing to spawn/place → reconcile converges immediately.
	desired := w.DesiredWorld{
		ActiveProfile: "default",
		Profiles:      map[w.ProfileID]w.DesiredProfile{"default": {ID: "default", Assignments: map[w.SlotID]w.ProjectID{}}},
	}
	ctrl := New(env, desired, wm.NewFake(env), store.NewMemoryStore(desired))
	ctrl.degraded.Store(true) // simulate a prior degraded transaction

	if _, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{}); err != nil {
		t.Fatalf("empty reconcile should converge cleanly: %v", err)
	}
	if ctrl.IsDegraded() {
		t.Fatal("a converged transaction must clear IsDegraded()=false (leave the degraded state)")
	}
}
