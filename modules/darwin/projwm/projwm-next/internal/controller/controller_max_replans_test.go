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

// selectiveSpawnFailAdapter fails the spawn for exactly one desired window
// identity and delegates every other spawn to the embedded Fake. It models a
// single broken window (e.g. a crashing app) amid otherwise-healthy windows.
type selectiveSpawnFailAdapter struct {
	*wm.Fake
	failDesired w.DesiredWindowID
}

func (a *selectiveSpawnFailAdapter) Spawn(ctx context.Context, r wm.SpawnRequest) (w.LiveWindowID, error) {
	if r.Desired == a.failDesired {
		return "", errors.New("test: this window's spawn intentionally fails")
	}
	return a.Fake.Spawn(ctx, r)
}

// TestSSOTSection68_GracefulDegradation_OneSpawnFailOthersContinue is the owner
// test for SSOT §6.8 (previously §10.9 GAP-17): a single window spawn failure
// must NOT abort the whole transaction. The user-visible contract: every other
// window still spawns, and the broken one is surfaced as a [INVARIANT] card.
// (The transaction ultimately fails to converge — the broken window never
// appears — and falls through to the §7.1 max-replans path, but the healthy
// windows persist on the WM, which is the "壊れない" steady state §6.8 promises.)
func TestSSOTSection68_GracefulDegradation_OneSpawnFailOthersContinue(t *testing.T) {
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Slots: []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}, {ID: "W", Workspace: "W", Order: 2}},
			Workspaces: []w.WorkspaceSpec{
				{ID: "Q", RawName: "Q", DisplayName: "Q", Role: w.WorkspaceProject},
				{ID: "W", RawName: "W", DisplayName: "W", Role: w.WorkspaceProject},
			},
		},
	}
	brokenID := w.DesiredWindowID{Project: "projA", Kind: w.WindowShell, Index: 1}
	healthyID := w.DesiredWindowID{Project: "projB", Kind: w.WindowShell, Index: 1}
	desired := w.DesiredWorld{
		ActiveProfile: "default",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"default": {ID: "default", Assignments: map[w.SlotID]w.ProjectID{"Q": "projA", "W": "projB"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"projA": {ID: "projA", Windows: []w.DesiredWindow{{ID: brokenID, Kind: w.WindowShell}}},
			"projB": {ID: "projB", Windows: []w.DesiredWindow{{ID: healthyID, Kind: w.WindowShell}}},
		},
	}
	st := store.NewMemoryStore(desired)
	fake := wm.NewFake(env)
	adapter := &selectiveSpawnFailAdapter{Fake: fake, failDesired: brokenID}
	ctrl := New(env, desired, adapter, st)
	ctrl.MaxReplans = 2

	// The transaction will not converge (broken window never appears) and
	// returns an error via the §7.1 max-replans path — expected, not the point.
	_, _ = ctrl.ApplyIntent(context.Background(), intent.Reconcile{})

	// §6.8 bullet 1: the healthy window spawned despite the broken sibling.
	obs, err := fake.Observe(context.Background())
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	healthySpawned, brokenSpawned := false, false
	for _, ow := range obs.Windows {
		if ow.MatchedTo != nil && *ow.MatchedTo == healthyID {
			healthySpawned = true
		}
		if ow.MatchedTo != nil && *ow.MatchedTo == brokenID {
			brokenSpawned = true
		}
	}
	if !healthySpawned {
		t.Errorf("SSOT §6.8①: healthy window must still spawn when a sibling spawn fails; observed=%+v", obs.Windows)
	}
	if brokenSpawned {
		t.Fatal("fixture invalid: the broken window should never have spawned")
	}

	// §6.8 bullet 3: a degraded spawn-failure [INVARIANT] card surfaced.
	degraded := false
	for _, c := range ctrl.state.Meta.ActiveCards {
		if c.Type == w.CardTypeInvariant && c.Context["degraded"] == "true" {
			degraded = true
			if c.Context["window"] == "" {
				t.Errorf("§6.8③: degraded card must name the failed window; ctx=%+v", c.Context)
			}
		}
	}
	if !degraded {
		t.Errorf("SSOT §6.8③: expected a degraded spawn-failure card among %+v", ctrl.state.Meta.ActiveCards)
	}
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

	// Behavior 3: the commit-fail [INVARIANT] card surfaced. Under §6.8
	// graceful degradation the failing spawn first emits a per-window
	// degraded [INVARIANT] card (no "reason", carries "degraded"=true), so
	// the commit-fail card is specifically the one carrying a non-empty
	// "reason" (e.g. max-replans-exceeded). Require at least one such card.
	postCards := ctrl.state.Meta.ActiveCards
	if len(postCards) <= preCards {
		t.Fatalf("SSOT §7.1 (3): expected a new card, count went %d → %d", preCards, len(postCards))
	}
	found := false
	for _, c := range postCards[preCards:] {
		if c.Type == w.CardTypeInvariant && c.Context["reason"] != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("SSOT §7.1 (3): no commit-fail [INVARIANT] card (with reason) emitted among %d new cards", len(postCards)-preCards)
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
