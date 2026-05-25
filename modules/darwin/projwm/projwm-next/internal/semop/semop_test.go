package semop

import (
	"context"
	"strings"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestSpawnProjectTerminalVerifiesSemanticContract(t *testing.T) {
	env := testEnv()
	fake := wm.NewFake(env)
	target := testDesiredWorld(w.WindowShell)
	desiredID := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowShell, Index: 1}
	runner := Runner{Adapter: fake, Env: env}

	live, err := runner.SpawnProjectTerminal(context.Background(), desiredID, "Q", w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{}}, target)
	if err != nil {
		t.Fatalf("SpawnProjectTerminal: %v", err)
	}
	after, err := fake.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	got := after.Windows[live]
	if got.Kind != w.WindowShell || got.Workspace != "Q" || got.App.BundleID != "com.mitchellh.ghostty" || got.Title.Value != "shell-1:dotfiles" {
		t.Fatalf("spawned window = %+v", got)
	}
}

func TestSpawnProjectTerminalRejectsAmbiguousPreexistingIdentity(t *testing.T) {
	env := testEnv()
	fake := wm.NewFake(env)
	target := testDesiredWorld(w.WindowShell)
	desiredID := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowShell, Index: 1}
	for i := 0; i < 2; i++ {
		if _, err := fake.Spawn(context.Background(), wm.SpawnRequest{
			Workspace: "Q",
			Kind:      w.WindowShell,
			Desired:   desiredID,
			Title:     "shell-1:dotfiles",
			BundleID:  "com.mitchellh.ghostty",
		}); err != nil {
			t.Fatalf("seed duplicate %d: %v", i, err)
		}
	}
	observed, err := fake.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	runner := Runner{Adapter: fake, Env: env}
	_, err = runner.SpawnProjectTerminal(context.Background(), desiredID, "Q", observed, target)
	if err == nil || !strings.Contains(err.Error(), "classified ambiguous") {
		t.Fatalf("expected ambiguous pre-spawn rejection, got %v", err)
	}
}

func TestSpawnProjectBrowserRequiresOpaquePayloadToken(t *testing.T) {
	env := testEnv()
	fake := wm.NewFake(env)
	adapter := &capturingSpawnAdapter{Fake: fake}
	target := testDesiredWorld(w.WindowBrowser)
	desiredID := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowBrowser, Index: 1}
	target.Projects["dotfiles"].Windows[0].App = w.AppRequirement{Capability: w.CapabilityBrowser, BundleID: "com.vivaldi.Vivaldi", AppPath: "/Applications/Vivaldi.app"}
	target.Projects["dotfiles"].Windows[0].TitleContract = w.TitleContract{
		Authority: w.TitleControllerOwned,
		Expected:  "browser-1:dotfiles",
		Drift:     w.TitleDriftRepair,
	}
	target.Projects["dotfiles"].Windows[0].Browser = &w.DesiredBrowserSession{
		URLPayloadRefs: []w.PrivatePayloadRef{"browser-payload-v1-00000000000000000000000000000000"},
	}
	runner := Runner{Adapter: adapter, Env: env}

	if _, err := runner.SpawnProjectBrowser(context.Background(), desiredID, "Q", w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{}}, target); err != nil {
		t.Fatalf("SpawnProjectBrowser: %v", err)
	}
	if adapter.last.BrowserPayloadToken != "browser-payload-v1-00000000000000000000000000000000" || adapter.last.BrowserProfile != browser.VivaldiAutomationProfile {
		t.Fatalf("browser request missing private payload ref: %+v", adapter.last)
	}

	target.Projects["dotfiles"].Windows[0].Browser = nil
	adapter.last = wm.SpawnRequest{}
	if _, err := runner.SpawnProjectBrowser(context.Background(), desiredID, "Q", w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{}}, target); err == nil {
		t.Fatal("expected browser spawn without private payload token to fail")
	}
}

func TestMoveResolvedWindowToWorkspaceVerifiesPostMoveState(t *testing.T) {
	env := testEnv()
	fake := wm.NewFake(env)
	target := testDesiredWorld(w.WindowShell)
	desiredID := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowShell, Index: 1}
	live, err := fake.Spawn(context.Background(), wm.SpawnRequest{
		Workspace: "Q",
		Kind:      w.WindowShell,
		Desired:   desiredID,
		Title:     "shell-1:dotfiles",
		BundleID:  "com.mitchellh.ghostty",
	})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	observed, err := fake.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	runner := Runner{Adapter: fake, Env: env}
	if err := runner.MoveResolvedWindowToWorkspace(context.Background(), live, "W", nil, observed, target); err != nil {
		t.Fatalf("MoveResolvedWindowToWorkspace: %v", err)
	}
	after, err := fake.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe after: %v", err)
	}
	if got := after.Windows[live].Workspace; got != "W" {
		t.Fatalf("workspace = %s, want W", got)
	}
}

type capturingSpawnAdapter struct {
	*wm.Fake
	last wm.SpawnRequest
}

func (a *capturingSpawnAdapter) Spawn(ctx context.Context, r wm.SpawnRequest) (w.LiveWindowID, error) {
	a.last = r
	if r.Kind == w.WindowBrowser && r.BrowserPayloadToken == "" {
		return "", wm.ErrRealBackendBlocked
	}
	return a.Fake.Spawn(ctx, r)
}

func testEnv() w.ManagedEnvironment {
	return w.ManagedEnvironment{
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{
				{ID: "Q", RawName: "Q", DisplayName: "Q", Role: w.WorkspaceProject},
				{ID: "W", RawName: "W", DisplayName: "W", Role: w.WorkspaceProject},
			},
		},
	}
}

func testDesiredWorld(kind w.WindowKind) w.DesiredWorld {
	desiredID := w.DesiredWindowID{Project: "dotfiles", Kind: kind, Index: 1}
	return w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "dotfiles"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"dotfiles": {
				ID:   "dotfiles",
				Root: "/Users/yuta/dev/dotfiles",
				Windows: []w.DesiredWindow{{
					ID:   desiredID,
					Kind: kind,
					App:  w.AppRequirement{Capability: w.CapabilityTerminal, BundleID: "com.mitchellh.ghostty", AppPath: "/Applications/Ghostty.app"},
					TitleContract: w.TitleContract{
						Authority: w.TitleControllerOwned,
						Expected:  "shell-1:dotfiles",
					},
				}},
			},
		},
	}
}
