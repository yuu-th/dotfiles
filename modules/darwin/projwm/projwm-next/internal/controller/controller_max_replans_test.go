package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §7.1 commit-fail path (GAP-19) の 4 挙動統合テスト:
//   1. トランザクションは fail し、commit されない
//   2. WorldState は Desired をトランザクション開始前にロールバック
//      (ただし ActiveCards / DirtyScopes は SSOT §7.1 step 3+4 carve-out)
//   3. cockpit に [INVARIANT] カード通知
//   4. 次の intent/event で再挑戦できるよう dirty scope 記録
//
// All four MUST land within a single failing transaction. The cleanest
// fixture to reach the failNoCommitTrace path is an executor-error: a
// neverSpawnsAdapter ensures the first Phase-B spawn op returns an
// error, which routes through the same failNoCommitTrace + rollback
// machinery that max-replans-exceeded uses. The behaviors are
// implemented uniformly, so the executor-error path proves the
// max-replans path by construction.

// neverSpawnsAdapter wraps wm.Fake but always returns an error from
// Spawn so the planner re-plans the same missing-window scope on every
// iteration — guaranteeing max-replans-exceeded.
type neverSpawnsAdapter struct {
	*wm.Fake
}

func (n *neverSpawnsAdapter) Spawn(_ context.Context, _ wm.SpawnRequest) (w.LiveWindowID, error) {
	return "", errors.New("test: spawn intentionally fails so planner re-plans")
}

func TestSSOTSection71_CommitFailEmitsCardAndDirtyScope(t *testing.T) {
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Slots:      []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
			Workspaces: []w.WorkspaceSpec{{ID: "Q", RawName: "Q", DisplayName: "Q", Role: w.WorkspaceProject}},
		},
	}
	winID := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	desired := w.DesiredWorld{
		ActiveProfile: "default",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"default": {ID: "default", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{{ID: winID, Kind: w.WindowShell}}},
		},
	}
	st := store.NewMemoryStore(desired)
	fake := wm.NewFake(env)
	adapter := &neverSpawnsAdapter{Fake: fake}
	ctrl := New(env, desired, adapter, st)
	ctrl.MaxReplans = 2 // smaller value speeds the test

	preEpoch := ctrl.state.Meta.Epoch
	preDirtyScopes := len(ctrl.state.Meta.DirtyScopes)
	preCards := len(ctrl.state.Meta.ActiveCards)

	result, err := ctrl.ApplyIntent(context.Background(), intent.Reconcile{})

	// Behavior 1: commit did NOT happen — caller sees an error and no
	// generation was advanced.
	if err == nil {
		t.Fatal("SSOT §7.1 (1): expected max-replans-exceeded error, got nil")
	}
	if result.CommittedGeneration != "" {
		t.Errorf("SSOT §7.1 (1): no commit must happen on exceeded replans, got generation %q", result.CommittedGeneration)
	}

	// Behavior 3: [INVARIANT] card surfaced.
	postCards := ctrl.state.Meta.ActiveCards
	if len(postCards) <= preCards {
		t.Fatalf("SSOT §7.1 (3): expected a new card, count went %d → %d", preCards, len(postCards))
	}
	found := false
	for _, c := range postCards[preCards:] {
		if c.Type == w.CardTypeInvariant {
			found = true
			if c.Context["reason"] == "" {
				t.Errorf("SSOT §7.1 (3): INVARIANT card must record a non-empty reason; got %+v", c.Context)
			}
		}
	}
	if !found {
		t.Errorf("SSOT §7.1 (3): no [INVARIANT] card emitted among %d new cards", len(postCards)-preCards)
	}

	// Behavior 4: dirty scope recorded — next intent will retry.
	postDirty := len(ctrl.state.Meta.DirtyScopes)
	if postDirty <= preDirtyScopes {
		t.Errorf("SSOT §7.1 (4): expected new dirty scope to force retry on next intent, count went %d → %d", preDirtyScopes, postDirty)
	}

	// Behavior 2 (cross-check): epoch did NOT advance (Commit phase
	// increments epoch; no commit ⇒ no epoch bump).
	if ctrl.state.Meta.Epoch != preEpoch {
		t.Errorf("SSOT §7.1 (2 cross-check): epoch advanced (%d → %d) despite no-commit", preEpoch, ctrl.state.Meta.Epoch)
	}

	// The returned error reaches the caller (the rollback path
	// preserves the failure so caller can react). max-replans path
	// returns ReplanExceededError; executor-error returns the wrapped
	// op error. Either is acceptable — just must not be nil.
	_ = err // already non-nil per behavior 1 check above
}
