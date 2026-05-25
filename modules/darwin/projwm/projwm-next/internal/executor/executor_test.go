package executor

import (
	"context"
	"strings"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/adapter/zed"
	"github.com/yuu-th/projwm-next/internal/op"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// fakeVivaldiCloser is a unit-test double for the executor's VivaldiCloser
// dependency. It records calls and returns canned observations. This lets us
// drive the executor's KindKillSession + browser-window-close path without
// the real Vivaldi adapter or omniwm.
type fakeVivaldiCloser struct {
	pre        browser.VivaldiCloseObservation
	post       browser.VivaldiCloseObservation
	callCount  int
	closeCalls []w.LiveWindowID
	closeErr   error
}

func (s *fakeVivaldiCloser) CollectCloseObservation(ctx context.Context, params browser.CloseObservationParams) (browser.VivaldiCloseObservation, error) {
	s.callCount++
	if s.callCount == 1 {
		return s.pre, nil
	}
	return s.post, nil
}

func (s *fakeVivaldiCloser) CloseLiveWindow(ctx context.Context, live w.LiveWindowID) error {
	s.closeCalls = append(s.closeCalls, live)
	return s.closeErr
}

// fakeZedCloser is a unit-test double for the executor's ZedCloser dependency.
// It records calls and returns canned observations. This lets us drive the
// executor's KindKillSession + project-scoped-app path without the real Zed
// adapter or osascript.
type fakeZedCloser struct {
	pre        zed.CloseObservation
	post       zed.CloseObservation
	callCount  int
	closeCalls []w.LiveWindowID
	closeErr   error
}

func (s *fakeZedCloser) CollectCloseObservation(ctx context.Context, params zed.CloseObservationParams) (zed.CloseObservation, error) {
	s.callCount++
	if s.callCount == 1 {
		return s.pre, nil
	}
	return s.post, nil
}

func (s *fakeZedCloser) CloseLiveWindow(ctx context.Context, live w.LiveWindowID) error {
	s.closeCalls = append(s.closeCalls, live)
	return s.closeErr
}

func TestExecuteRejectsLiveMutationWithoutDesiredIdentityEvidence(t *testing.T) {
	live := w.LiveWindowID("orphan")
	ex := Executor{}
	err := ex.Execute(context.Background(), op.Operation{
		Kind: op.KindCloseWindow,
		Target: op.Target{
			LiveWindow: &live,
		},
		Preconditions: []op.Precondition{
			{Kind: op.PreUniqueStrong, Target: op.Target{LiveWindow: &live}},
		},
	}, w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			live: {
				ID:        live,
				Kind:      w.WindowShell,
				Workspace: "Q",
			},
		},
	}, w.DesiredWorld{})
	if err == nil || !strings.Contains(err.Error(), "no desired identity evidence") {
		t.Fatalf("expected PreUniqueStrong identity evidence rejection, got %v", err)
	}
}

func TestExecuteBlocksCloseWindowForProductionBackends(t *testing.T) {
	live := w.LiveWindowID("managed")
	ex := Executor{Env: w.ManagedEnvironment{WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"}}}
	err := ex.Execute(context.Background(), op.Operation{
		Kind:   op.KindCloseWindow,
		Target: op.Target{LiveWindow: &live},
	}, w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			live: {ID: live, Kind: w.WindowShell, Workspace: "Q"},
		},
	}, w.DesiredWorld{})
	if err == nil || !strings.Contains(err.Error(), "close-window is blocked") {
		t.Fatalf("expected production close-window block, got %v", err)
	}
}

func TestExecuteKillSessionRejectsStillActiveDesiredProject(t *testing.T) {
	live := w.LiveWindowID("managed")
	desiredID := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	target := desiredWorldForKillSessionTest(desiredID, true)
	ex := Executor{}
	err := ex.Execute(context.Background(), op.Operation{
		Kind: op.KindKillSession,
		Target: op.Target{
			LiveWindow:    &live,
			DesiredWindow: &desiredID,
		},
		Preconditions: []op.Precondition{
			{Kind: op.PreUniqueStrong, Target: op.Target{LiveWindow: &live}},
		},
	}, w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			live: {
				ID:        live,
				Kind:      w.WindowShell,
				App:       w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"},
				Workspace: "Q",
				Title:     w.ObservedTitle{Value: "shell-1:p1"},
				MatchedTo: &desiredID,
			},
		},
	}, target)
	if err == nil || !strings.Contains(err.Error(), "still active") {
		t.Fatalf("expected active-project kill-session rejection, got %v", err)
	}
}

func TestExecuteKillSessionTerminatesInactiveManagedAppInstance(t *testing.T) {
	desiredID := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	target := desiredWorldForKillSessionTest(desiredID, false)
	env := w.ManagedEnvironment{
		Apps: w.AppEnvironment{ManagedApps: []w.ManagedAppPolicy{{
			Capability: w.CapabilityTerminal,
			BundleID:   "com.mitchellh.ghostty",
			LifecycleRemoval: w.LifecycleRemovalPolicy{
				Allowed:      true,
				Method:       w.LifecycleRemovalAXCloseGuarded,
				AllowedKinds: []w.WindowKind{w.WindowAI, w.WindowShell, w.WindowViewer},
			},
		}}},
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{{ID: "Q", Role: w.WorkspaceProject}},
		},
	}
	fake := wm.NewFake(env)
	live, err := fake.Spawn(context.Background(), wm.SpawnRequest{
		Workspace: "Q",
		Kind:      w.WindowShell,
		Desired:   desiredID,
		Title:     "shell-1:p1",
		BundleID:  "com.mitchellh.ghostty",
	})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	observed, err := fake.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	ex := Executor{Adapter: fake, Env: env}
	err = ex.Execute(context.Background(), op.Operation{
		Kind: op.KindKillSession,
		Target: op.Target{
			LiveWindow:    &live,
			DesiredWindow: &desiredID,
		},
		Preconditions: []op.Precondition{
			{Kind: op.PreUniqueStrong, Target: op.Target{LiveWindow: &live}},
		},
	}, observed, target)
	if err != nil {
		t.Fatalf("Execute kill-session: %v", err)
	}
	after, err := fake.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe after: %v", err)
	}
	if _, ok := after.Windows[live]; ok {
		t.Fatalf("kill-session should remove managed fake window: %+v", after.Windows)
	}
}

func TestExecuteKillSessionRejectsZedAndVivaldiAppSpecificContractsUntilLiveAuthorityExists(t *testing.T) {
	for _, tc := range []struct {
		name       string
		kind       w.WindowKind
		capability w.AppCapability
		bundle     string
		method     w.LifecycleRemovalMethod
	}{
		{name: "zed-project-scoped-app", kind: w.WindowEditor, capability: w.CapabilityEditor, bundle: "dev.zed.Zed", method: w.LifecycleRemovalProjectScopedApp},
		{name: "vivaldi-browser-window-close", kind: w.WindowBrowser, capability: w.CapabilityBrowser, bundle: "com.vivaldi.Vivaldi", method: w.LifecycleRemovalBrowserWindowClose},
	} {
		t.Run(tc.name, func(t *testing.T) {
			desiredID := w.DesiredWindowID{Project: "p1", Kind: tc.kind, Index: 1}
			live := w.LiveWindowID("managed")
			observedTitle := "app-owned-title"
			if tc.kind == w.WindowBrowser {
				observedTitle = "browser-1:p1"
			}
			env := w.ManagedEnvironment{
				Apps: w.AppEnvironment{ManagedApps: []w.ManagedAppPolicy{{
					Capability: tc.capability,
					BundleID:   tc.bundle,
					LifecycleRemoval: w.LifecycleRemovalPolicy{
						Allowed:      true,
						Method:       tc.method,
						AllowedKinds: []w.WindowKind{tc.kind},
					},
				}}},
				Workspaces: w.WorkspaceEnvironment{
					Workspaces: []w.WorkspaceSpec{{ID: "Q", Role: w.WorkspaceProject}},
				},
			}
			ex := Executor{Env: env}
			err := ex.Execute(context.Background(), op.Operation{
				Kind: op.KindKillSession,
				Target: op.Target{
					LiveWindow:    &live,
					DesiredWindow: &desiredID,
				},
				Preconditions: []op.Precondition{
					{Kind: op.PreUniqueStrong, Target: op.Target{LiveWindow: &live, DesiredWindow: &desiredID}},
				},
			}, w.ObservedWorld{
				Windows: map[w.LiveWindowID]w.ObservedWindow{
					live: {
						ID:        live,
						Kind:      tc.kind,
						App:       w.ObservedAppRef{BundleID: tc.bundle},
						Workspace: "Q",
						Title:     w.ObservedTitle{Value: observedTitle},
						MatchedTo: &desiredID,
					},
				},
			}, desiredWorldForAppSpecificRemovalTest(desiredID, tc.capability, tc.bundle))
			if err == nil || !strings.Contains(err.Error(), "lifecycle removal is not authorized") {
				t.Fatalf("expected %s to stay blocked until live close contract exists, got %v", tc.name, err)
			}
		})
	}
}

func TestExecuteKillSessionVivaldiBrowserWindowCloseSuccess(t *testing.T) {
	desiredID := w.DesiredWindowID{Project: "p1", Kind: w.WindowBrowser, Index: 1}
	const token = "browser-payload-v1-00000000000000000000000000000000"
	live := w.LiveWindowID("vw-target")
	env := w.ManagedEnvironment{
		Apps: w.AppEnvironment{ManagedApps: []w.ManagedAppPolicy{{
			Capability: w.CapabilityBrowser,
			BundleID:   "com.vivaldi.Vivaldi",
			LifecycleRemoval: w.LifecycleRemovalPolicy{
				Allowed:      true,
				Method:       w.LifecycleRemovalBrowserWindowClose,
				AllowedKinds: []w.WindowKind{w.WindowBrowser},
			},
		}}},
		Workspaces: w.WorkspaceEnvironment{Workspaces: []w.WorkspaceSpec{{ID: "Q", Role: w.WorkspaceProject}}},
	}
	target := w.DesiredWorld{
		ActiveProfile: "main",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"main": {ID: "main", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			desiredID.Project: {
				ID: desiredID.Project,
				Windows: []w.DesiredWindow{{
					ID:   desiredID,
					Kind: desiredID.Kind,
					App:  w.AppRequirement{Capability: w.CapabilityBrowser, BundleID: "com.vivaldi.Vivaldi"},
					TitleContract: w.TitleContract{
						Authority: w.TitleControllerOwned,
						Expected:  "browser-1:p1",
						Drift:     w.TitleDriftRepair,
					},
					Browser: &w.DesiredBrowserSession{URLPayloadRefs: []w.PrivatePayloadRef{w.PrivatePayloadRef(token)}},
				}},
			},
		},
	}
	fake := &fakeVivaldiCloser{
		pre: browser.VivaldiCloseObservation{
			ObservedBundle:       "com.vivaldi.Vivaldi",
			CorrelatedBrowserID:  string(live),
			CorrelatedLiveWindow: live,
			Present:              true,
			MatchingRemaining:    1,
			ObservedPayloadToken: token,
			TabPayloadCorrelated: true,
			UserProfileIsolated:  true,
		},
		post: browser.VivaldiCloseObservation{
			Present:           false,
			MatchingRemaining: 0,
		},
	}
	ex := Executor{Env: env, Vivaldi: fake}
	err := ex.Execute(context.Background(), op.Operation{
		Kind: op.KindKillSession,
		Target: op.Target{
			LiveWindow:    &live,
			DesiredWindow: &desiredID,
		},
		Preconditions: []op.Precondition{
			{Kind: op.PreUniqueStrong, Target: op.Target{LiveWindow: &live, DesiredWindow: &desiredID}},
		},
		LifecycleRemovalMethod: w.LifecycleRemovalBrowserWindowClose,
	}, w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			live: {
				ID:        live,
				Kind:      w.WindowBrowser,
				App:       w.ObservedAppRef{BundleID: "com.vivaldi.Vivaldi"},
				Workspace: "Q",
				Title:     w.ObservedTitle{Value: "browser-1:p1"},
				MatchedTo: &desiredID,
			},
		},
	}, target)
	if err != nil {
		t.Fatalf("Execute kill-session vivaldi: %v", err)
	}
	if fake.callCount != 2 {
		t.Fatalf("expected exactly two CollectCloseObservation calls (before+after), got %d", fake.callCount)
	}
	if len(fake.closeCalls) != 1 || fake.closeCalls[0] != live {
		t.Fatalf("expected single CloseLiveWindow on %s, got %v", live, fake.closeCalls)
	}
}

func TestExecuteKillSessionVivaldiRejectsMissingPayloadToken(t *testing.T) {
	desiredID := w.DesiredWindowID{Project: "p1", Kind: w.WindowBrowser, Index: 1}
	live := w.LiveWindowID("vw-target")
	env := w.ManagedEnvironment{
		Apps: w.AppEnvironment{ManagedApps: []w.ManagedAppPolicy{{
			Capability: w.CapabilityBrowser,
			BundleID:   "com.vivaldi.Vivaldi",
			LifecycleRemoval: w.LifecycleRemovalPolicy{
				Allowed:      true,
				Method:       w.LifecycleRemovalBrowserWindowClose,
				AllowedKinds: []w.WindowKind{w.WindowBrowser},
			},
		}}},
		Workspaces: w.WorkspaceEnvironment{Workspaces: []w.WorkspaceSpec{{ID: "Q", Role: w.WorkspaceProject}}},
	}
	target := w.DesiredWorld{
		ActiveProfile: "main",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"main": {ID: "main", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			desiredID.Project: {
				ID: desiredID.Project,
				Windows: []w.DesiredWindow{{
					ID:   desiredID,
					Kind: desiredID.Kind,
					App:  w.AppRequirement{Capability: w.CapabilityBrowser, BundleID: "com.vivaldi.Vivaldi"},
					TitleContract: w.TitleContract{
						Authority: w.TitleControllerOwned,
						Expected:  "browser-1:p1",
						Drift:     w.TitleDriftRepair,
					},
					// no Browser → missing payload token
				}},
			},
		},
	}
	ex := Executor{Env: env, Vivaldi: &fakeVivaldiCloser{}}
	err := ex.Execute(context.Background(), op.Operation{
		Kind: op.KindKillSession,
		Target: op.Target{
			LiveWindow:    &live,
			DesiredWindow: &desiredID,
		},
		Preconditions: []op.Precondition{
			{Kind: op.PreUniqueStrong, Target: op.Target{LiveWindow: &live, DesiredWindow: &desiredID}},
		},
	}, w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			live: {
				ID:        live,
				Kind:      w.WindowBrowser,
				App:       w.ObservedAppRef{BundleID: "com.vivaldi.Vivaldi"},
				Workspace: "Q",
				Title:     w.ObservedTitle{Value: "browser-1:p1"},
				MatchedTo: &desiredID,
			},
		},
	}, target)
	if err == nil || !strings.Contains(err.Error(), "private payload token") {
		t.Fatalf("expected payload-token rejection, got %v", err)
	}
}

func TestExecuteKillSessionVivaldiRejectsContractFailureWithoutMutation(t *testing.T) {
	desiredID := w.DesiredWindowID{Project: "p1", Kind: w.WindowBrowser, Index: 1}
	const token = "browser-payload-v1-00000000000000000000000000000000"
	live := w.LiveWindowID("vw-target")
	env := w.ManagedEnvironment{
		Apps: w.AppEnvironment{ManagedApps: []w.ManagedAppPolicy{{
			Capability: w.CapabilityBrowser,
			BundleID:   "com.vivaldi.Vivaldi",
			LifecycleRemoval: w.LifecycleRemovalPolicy{
				Allowed:      true,
				Method:       w.LifecycleRemovalBrowserWindowClose,
				AllowedKinds: []w.WindowKind{w.WindowBrowser},
			},
		}}},
		Workspaces: w.WorkspaceEnvironment{Workspaces: []w.WorkspaceSpec{{ID: "Q", Role: w.WorkspaceProject}}},
	}
	target := w.DesiredWorld{
		ActiveProfile: "main",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"main": {ID: "main", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			desiredID.Project: {
				ID: desiredID.Project,
				Windows: []w.DesiredWindow{{
					ID:   desiredID,
					Kind: desiredID.Kind,
					App:  w.AppRequirement{Capability: w.CapabilityBrowser, BundleID: "com.vivaldi.Vivaldi"},
					TitleContract: w.TitleContract{
						Authority: w.TitleControllerOwned,
						Expected:  "browser-1:p1",
						Drift:     w.TitleDriftRepair,
					},
					Browser: &w.DesiredBrowserSession{URLPayloadRefs: []w.PrivatePayloadRef{w.PrivatePayloadRef(token)}},
				}},
			},
		},
	}
	// Pre-observation reports the target present but NOT profile-isolated. The
	// contract should fail before any close mutation runs.
	fake := &fakeVivaldiCloser{
		pre: browser.VivaldiCloseObservation{
			ObservedBundle:       "com.vivaldi.Vivaldi",
			CorrelatedBrowserID:  string(live),
			CorrelatedLiveWindow: live,
			Present:              true,
			MatchingRemaining:    1,
			ObservedPayloadToken: token,
			TabPayloadCorrelated: true,
			UserProfileIsolated:  false, // contract violation
		},
	}
	ex := Executor{Env: env, Vivaldi: fake}
	err := ex.Execute(context.Background(), op.Operation{
		Kind: op.KindKillSession,
		Target: op.Target{
			LiveWindow:    &live,
			DesiredWindow: &desiredID,
		},
		Preconditions: []op.Precondition{
			{Kind: op.PreUniqueStrong, Target: op.Target{LiveWindow: &live, DesiredWindow: &desiredID}},
		},
	}, w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			live: {
				ID:        live,
				Kind:      w.WindowBrowser,
				App:       w.ObservedAppRef{BundleID: "com.vivaldi.Vivaldi"},
				Workspace: "Q",
				Title:     w.ObservedTitle{Value: "browser-1:p1"},
				MatchedTo: &desiredID,
			},
		},
	}, target)
	if err == nil || !strings.Contains(err.Error(), "user profile isolation") {
		t.Fatalf("expected user-profile-isolation contract failure, got %v", err)
	}
	if len(fake.closeCalls) != 0 {
		t.Fatalf("close mutation must not run when pre-evidence fails: %+v", fake.closeCalls)
	}
}

func TestExecuteKillSessionZedProjectScopedRemovalSuccess(t *testing.T) {
	desiredID := w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 1}
	live := w.LiveWindowID("zed-target")
	projectRoot := t.TempDir()
	env := w.ManagedEnvironment{
		Apps: w.AppEnvironment{ManagedApps: []w.ManagedAppPolicy{{
			Capability: w.CapabilityEditor,
			BundleID:   "dev.zed.Zed",
			LifecycleRemoval: w.LifecycleRemovalPolicy{
				Allowed:      true,
				Method:       w.LifecycleRemovalProjectScopedApp,
				AllowedKinds: []w.WindowKind{w.WindowEditor},
			},
		}}},
		Workspaces: w.WorkspaceEnvironment{Workspaces: []w.WorkspaceSpec{{ID: "Q", Role: w.WorkspaceProject}}},
	}
	target := w.DesiredWorld{
		ActiveProfile: "main",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"main": {ID: "main", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			desiredID.Project: {
				ID:   desiredID.Project,
				Root: projectRoot,
				Windows: []w.DesiredWindow{{
					ID:   desiredID,
					Kind: desiredID.Kind,
					App:  w.AppRequirement{Capability: w.CapabilityEditor, BundleID: "dev.zed.Zed"},
					TitleContract: w.TitleContract{
						Authority: w.TitleAppOwned,
					},
				}},
			},
		},
	}
	fake := &fakeZedCloser{
		pre: zed.CloseObservation{
			ObservedBundle:     "dev.zed.Zed",
			AdapterProjectRoot: projectRoot,
			AdapterSessionID:   "zed-pid-12345",
			AdapterWindowID:    string(live),
			Present:            true,
			MatchingRemaining:  1,
			UnsavedChanges:     zed.UnsavedChangeClean,
		},
		post: zed.CloseObservation{
			Present:           false,
			MatchingRemaining: 0,
		},
	}
	ex := Executor{Env: env, Zed: fake}
	err := ex.Execute(context.Background(), op.Operation{
		Kind: op.KindKillSession,
		Target: op.Target{
			LiveWindow:    &live,
			DesiredWindow: &desiredID,
		},
		Preconditions: []op.Precondition{
			{Kind: op.PreUniqueStrong, Target: op.Target{LiveWindow: &live, DesiredWindow: &desiredID}},
		},
		LifecycleRemovalMethod: w.LifecycleRemovalProjectScopedApp,
	}, w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			live: {
				ID:        live,
				Kind:      w.WindowEditor,
				App:       w.ObservedAppRef{BundleID: "dev.zed.Zed"},
				Workspace: "Q",
				Title:     w.ObservedTitle{Value: "app-owned-title"},
				MatchedTo: &desiredID,
			},
		},
	}, target)
	if err != nil {
		t.Fatalf("Execute kill-session zed: %v", err)
	}
	if fake.callCount != 2 {
		t.Fatalf("expected exactly two CollectCloseObservation calls (before+after), got %d", fake.callCount)
	}
	if len(fake.closeCalls) != 1 || fake.closeCalls[0] != live {
		t.Fatalf("expected single CloseLiveWindow on %s, got %v", live, fake.closeCalls)
	}
}

func TestExecuteKillSessionZedRejectsDirtyUnsavedChangesWithoutMutation(t *testing.T) {
	desiredID := w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 1}
	live := w.LiveWindowID("zed-target")
	projectRoot := t.TempDir()
	env := w.ManagedEnvironment{
		Apps: w.AppEnvironment{ManagedApps: []w.ManagedAppPolicy{{
			Capability: w.CapabilityEditor,
			BundleID:   "dev.zed.Zed",
			LifecycleRemoval: w.LifecycleRemovalPolicy{
				Allowed:      true,
				Method:       w.LifecycleRemovalProjectScopedApp,
				AllowedKinds: []w.WindowKind{w.WindowEditor},
			},
		}}},
		Workspaces: w.WorkspaceEnvironment{Workspaces: []w.WorkspaceSpec{{ID: "Q", Role: w.WorkspaceProject}}},
	}
	target := w.DesiredWorld{
		ActiveProfile: "main",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"main": {ID: "main", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			desiredID.Project: {
				ID:   desiredID.Project,
				Root: projectRoot,
				Windows: []w.DesiredWindow{{
					ID:   desiredID,
					Kind: desiredID.Kind,
					App:  w.AppRequirement{Capability: w.CapabilityEditor, BundleID: "dev.zed.Zed"},
					TitleContract: w.TitleContract{
						Authority: w.TitleAppOwned,
					},
				}},
			},
		},
	}
	fake := &fakeZedCloser{
		pre: zed.CloseObservation{
			ObservedBundle:     "dev.zed.Zed",
			AdapterProjectRoot: projectRoot,
			AdapterSessionID:   "zed-pid-12345",
			AdapterWindowID:    string(live),
			Present:            true,
			MatchingRemaining:  1,
			UnsavedChanges:     zed.UnsavedChangeDirty, // contract violation
		},
	}
	ex := Executor{Env: env, Zed: fake}
	err := ex.Execute(context.Background(), op.Operation{
		Kind: op.KindKillSession,
		Target: op.Target{
			LiveWindow:    &live,
			DesiredWindow: &desiredID,
		},
		Preconditions: []op.Precondition{
			{Kind: op.PreUniqueStrong, Target: op.Target{LiveWindow: &live, DesiredWindow: &desiredID}},
		},
	}, w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			live: {
				ID:        live,
				Kind:      w.WindowEditor,
				App:       w.ObservedAppRef{BundleID: "dev.zed.Zed"},
				Workspace: "Q",
				Title:     w.ObservedTitle{Value: "app-owned-title"},
				MatchedTo: &desiredID,
			},
		},
	}, target)
	if err == nil || !strings.Contains(err.Error(), "clean unsaved-change proof") {
		t.Fatalf("expected clean-unsaved-change contract failure, got %v", err)
	}
	if len(fake.closeCalls) != 0 {
		t.Fatalf("close mutation must not run when pre-evidence fails: %+v", fake.closeCalls)
	}
}

func TestExecuteKillSessionZedRejectsProjectRootMismatchWithoutMutation(t *testing.T) {
	desiredID := w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 1}
	live := w.LiveWindowID("zed-target")
	projectRoot := t.TempDir()
	otherRoot := t.TempDir()
	env := w.ManagedEnvironment{
		Apps: w.AppEnvironment{ManagedApps: []w.ManagedAppPolicy{{
			Capability: w.CapabilityEditor,
			BundleID:   "dev.zed.Zed",
			LifecycleRemoval: w.LifecycleRemovalPolicy{
				Allowed:      true,
				Method:       w.LifecycleRemovalProjectScopedApp,
				AllowedKinds: []w.WindowKind{w.WindowEditor},
			},
		}}},
		Workspaces: w.WorkspaceEnvironment{Workspaces: []w.WorkspaceSpec{{ID: "Q", Role: w.WorkspaceProject}}},
	}
	target := w.DesiredWorld{
		ActiveProfile: "main",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"main": {ID: "main", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			desiredID.Project: {
				ID:   desiredID.Project,
				Root: projectRoot,
				Windows: []w.DesiredWindow{{
					ID:   desiredID,
					Kind: desiredID.Kind,
					App:  w.AppRequirement{Capability: w.CapabilityEditor, BundleID: "dev.zed.Zed"},
					TitleContract: w.TitleContract{
						Authority: w.TitleAppOwned,
					},
				}},
			},
		},
	}
	// Pre-observation reports a different (unrelated) project root than the
	// desired project — the contract should fail before any close mutation.
	fake := &fakeZedCloser{
		pre: zed.CloseObservation{
			ObservedBundle:     "dev.zed.Zed",
			AdapterProjectRoot: otherRoot,
			AdapterSessionID:   "zed-pid-12345",
			AdapterWindowID:    string(live),
			Present:            true,
			MatchingRemaining:  1,
			UnsavedChanges:     zed.UnsavedChangeClean,
		},
	}
	ex := Executor{Env: env, Zed: fake}
	err := ex.Execute(context.Background(), op.Operation{
		Kind: op.KindKillSession,
		Target: op.Target{
			LiveWindow:    &live,
			DesiredWindow: &desiredID,
		},
		Preconditions: []op.Precondition{
			{Kind: op.PreUniqueStrong, Target: op.Target{LiveWindow: &live, DesiredWindow: &desiredID}},
		},
	}, w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			live: {
				ID:        live,
				Kind:      w.WindowEditor,
				App:       w.ObservedAppRef{BundleID: "dev.zed.Zed"},
				Workspace: "Q",
				Title:     w.ObservedTitle{Value: "app-owned-title"},
				MatchedTo: &desiredID,
			},
		},
	}, target)
	if err == nil || !strings.Contains(err.Error(), "project root does not match") {
		t.Fatalf("expected project-root-mismatch contract failure, got %v", err)
	}
	if len(fake.closeCalls) != 0 {
		t.Fatalf("close mutation must not run when pre-evidence fails: %+v", fake.closeCalls)
	}
}

func TestExecuteKillSessionZedRejectsMissingProjectRoot(t *testing.T) {
	desiredID := w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 1}
	live := w.LiveWindowID("zed-target")
	env := w.ManagedEnvironment{
		Apps: w.AppEnvironment{ManagedApps: []w.ManagedAppPolicy{{
			Capability: w.CapabilityEditor,
			BundleID:   "dev.zed.Zed",
			LifecycleRemoval: w.LifecycleRemovalPolicy{
				Allowed:      true,
				Method:       w.LifecycleRemovalProjectScopedApp,
				AllowedKinds: []w.WindowKind{w.WindowEditor},
			},
		}}},
		Workspaces: w.WorkspaceEnvironment{Workspaces: []w.WorkspaceSpec{{ID: "Q", Role: w.WorkspaceProject}}},
	}
	target := w.DesiredWorld{
		ActiveProfile: "main",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"main": {ID: "main", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			desiredID.Project: {
				ID: desiredID.Project,
				// Root intentionally empty
				Windows: []w.DesiredWindow{{
					ID:   desiredID,
					Kind: desiredID.Kind,
					App:  w.AppRequirement{Capability: w.CapabilityEditor, BundleID: "dev.zed.Zed"},
					TitleContract: w.TitleContract{
						Authority: w.TitleAppOwned,
					},
				}},
			},
		},
	}
	ex := Executor{Env: env, Zed: &fakeZedCloser{}}
	err := ex.Execute(context.Background(), op.Operation{
		Kind: op.KindKillSession,
		Target: op.Target{
			LiveWindow:    &live,
			DesiredWindow: &desiredID,
		},
		Preconditions: []op.Precondition{
			{Kind: op.PreUniqueStrong, Target: op.Target{LiveWindow: &live, DesiredWindow: &desiredID}},
		},
	}, w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			live: {
				ID:        live,
				Kind:      w.WindowEditor,
				App:       w.ObservedAppRef{BundleID: "dev.zed.Zed"},
				Workspace: "Q",
				Title:     w.ObservedTitle{Value: "app-owned-title"},
				MatchedTo: &desiredID,
			},
		},
	}, target)
	if err == nil || !strings.Contains(err.Error(), "desired project root") {
		t.Fatalf("expected missing project-root rejection, got %v", err)
	}
}

func desiredWorldForKillSessionTest(desiredID w.DesiredWindowID, active bool) w.DesiredWorld {
	assignments := map[w.SlotID]w.ProjectID{}
	if active {
		assignments["slot-a"] = desiredID.Project
	}
	return w.DesiredWorld{
		ActiveProfile: "main",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"main": {ID: "main", Assignments: assignments},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			desiredID.Project: {
				ID: desiredID.Project,
				Windows: []w.DesiredWindow{{
					ID:   desiredID,
					Kind: desiredID.Kind,
					App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
					TitleContract: w.TitleContract{
						Authority: w.TitleControllerOwned,
						Expected:  "shell-1:p1",
					},
				}},
			},
		},
	}
}

func desiredWorldForAppSpecificRemovalTest(desiredID w.DesiredWindowID, capability w.AppCapability, bundle string) w.DesiredWorld {
	titleContract := w.TitleContract{Authority: w.TitleAppOwned}
	if desiredID.Kind == w.WindowBrowser {
		titleContract = w.TitleContract{
			Authority: w.TitleControllerOwned,
			Expected:  "browser-1:p1",
			Drift:     w.TitleDriftRepair,
		}
	}
	return w.DesiredWorld{
		ActiveProfile: "main",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"main": {ID: "main", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			desiredID.Project: {
				ID: desiredID.Project,
				Windows: []w.DesiredWindow{{
					ID:            desiredID,
					Kind:          desiredID.Kind,
					App:           w.AppRequirement{Capability: capability, BundleID: bundle},
					TitleContract: titleContract,
				}},
			},
		},
	}
}

// TestExecuteHideCockpitRestoresPriorWindowFocus verifies SSOT §4.1 OP-07 +
// §5.4 Proposal mode focus restoration. The planner-emitted hide-cockpit
// op carries the SystemWindow's PriorWindow on Target.LiveWindow; the
// executor must call FocusWindow after HideCockpitOnDisplay so the user
// lands back on the window they had focused before summoning cockpit.
func TestExecuteHideCockpitRestoresPriorWindowFocus(t *testing.T) {
	env := w.ManagedEnvironment{
		WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"},
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{
				{ID: "8", RawName: "8", Role: w.WorkspaceGeneral},
				{ID: "CP1", RawName: "CP1", Role: w.WorkspaceGeneral},
			},
		},
	}
	fake := wm.NewFake(env)
	ctx := context.Background()

	// Spawn a "prior" window on workspace 8 and focus it. Then simulate
	// the cockpit show by switching the display to CP1; the prior live id
	// is what we expect to be re-focused after hide.
	desiredPrior := w.DesiredWindowID{Project: "prior-p", Kind: w.WindowShell, Index: 1}
	priorLive, err := fake.Spawn(ctx, wm.SpawnRequest{
		Workspace: "8",
		Kind:      w.WindowShell,
		Desired:   desiredPrior,
		Title:     "shell-1:prior-p",
		BundleID:  "com.mitchellh.ghostty",
	})
	if err != nil {
		t.Fatalf("Spawn prior: %v", err)
	}
	if err := fake.FocusWindow(ctx, priorLive); err != nil {
		t.Fatalf("FocusWindow prior: %v", err)
	}

	// SystemWindow representing the cockpit on display 0 (CP1) — the
	// reducer would normally populate PriorWindow from observed; here we
	// pass it directly via Target.LiveWindow as the planner emits.
	systemID := w.SystemWindowID{Kind: w.WindowCockpit, Index: 0}
	parkWs := w.WorkspaceID("CP1")
	priorWs := w.WorkspaceID("8")
	target := w.DesiredWorld{
		SystemWindows: []w.SystemWindow{{
			ID: systemID, Kind: w.WindowCockpit, DisplayIdx: 0,
			Title: "projwm-cockpit-0", ParkWorkspace: parkWs,
			Visibility: w.CockpitHidden,
			// PriorWorkspace and PriorWindow are populated by the
			// reducer's SetCockpitVisibility handler under real conditions.
			PriorWorkspace: priorWs, PriorWindow: priorLive,
		}},
	}
	observed, err := fake.Observe(ctx)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	// Fake adapter doesn't synthesize ObservedDisplay; provide one so the
	// executor's resolveDisplayForParkWorkspace finds the cockpit's owning
	// display.
	observed.Displays = w.ObservedDisplayState{
		Displays: map[w.DisplayID]w.ObservedDisplay{
			"display:0": {ID: "display:0", Connected: true, ActiveWorkspace: parkWs},
		},
		WorkspaceToDisplay: map[w.WorkspaceID]w.DisplayID{parkWs: "display:0"},
	}

	ex := Executor{Adapter: fake, Env: env}
	priorWsCopy := priorWs
	if err := ex.Execute(ctx, op.Operation{
		Kind: op.KindHideCockpit,
		Target: op.Target{
			SystemWindow: &systemID,
			Workspace:    &priorWsCopy,
			LiveWindow:   &priorLive,
		},
	}, observed, target); err != nil {
		t.Fatalf("Execute hide-cockpit: %v", err)
	}

	after, err := fake.Observe(ctx)
	if err != nil {
		t.Fatalf("Observe after: %v", err)
	}
	if after.Focus.Window != priorLive {
		t.Errorf("hide-cockpit did not restore focus: got %q, want %q (prior)", after.Focus.Window, priorLive)
	}
}

// TestExecuteShowScratchShell verifies SSOT §4.1 OP11: the show-scratch-shell
// op dispatches to Adapter.ShowScratchShell, which spawns / focuses the
// global scratch window.
func TestExecuteShowScratchShell(t *testing.T) {
	env := w.ManagedEnvironment{
		WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"},
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{{ID: "A", RawName: "A", Role: w.WorkspaceViewer}},
		},
	}
	fake := wm.NewFake(env)
	ctx := context.Background()

	ex := Executor{Adapter: fake, Env: env}
	if err := ex.Execute(ctx, op.Operation{Kind: op.KindShowScratchShell}, w.ObservedWorld{}, w.DesiredWorld{}); err != nil {
		t.Fatalf("Execute show-scratch-shell: %v", err)
	}
	obs, _ := fake.Observe(ctx)
	scratchCount := 0
	for _, win := range obs.Windows {
		if win.Title.Value == "projwm-scratch-shell" {
			scratchCount++
		}
	}
	if scratchCount != 1 {
		t.Errorf("expected 1 scratch window after show op, got %d", scratchCount)
	}
}

// TestExecuteHideScratchShellRestoresPriorFocus verifies that hide-scratch-shell
// with Target.LiveWindow set restores focus to that window.
func TestExecuteHideScratchShellRestoresPriorFocus(t *testing.T) {
	env := w.ManagedEnvironment{
		WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"},
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{
				{ID: "A", RawName: "A", Role: w.WorkspaceViewer},
				{ID: "8", RawName: "8", Role: w.WorkspaceGeneral},
			},
		},
	}
	fake := wm.NewFake(env)
	ctx := context.Background()

	// Spawn a "prior" window. Show scratch (focus shifts). Then exec
	// hide-scratch-shell with the prior on Target.LiveWindow.
	priorLive, err := fake.Spawn(ctx, wm.SpawnRequest{
		Workspace: "8",
		Kind:      w.WindowShell,
		Desired:   w.DesiredWindowID{Project: "p", Kind: w.WindowShell, Index: 1},
		Title:     "shell-1:p",
		BundleID:  "com.mitchellh.ghostty",
	})
	if err != nil {
		t.Fatalf("Spawn prior: %v", err)
	}
	if _, err := fake.ShowScratchShell(ctx); err != nil {
		t.Fatalf("ShowScratchShell: %v", err)
	}

	ex := Executor{Adapter: fake, Env: env}
	priorCopy := priorLive
	if err := ex.Execute(ctx, op.Operation{
		Kind:   op.KindHideScratchShell,
		Target: op.Target{LiveWindow: &priorCopy},
	}, w.ObservedWorld{}, w.DesiredWorld{}); err != nil {
		t.Fatalf("Execute hide-scratch-shell: %v", err)
	}
	obs, _ := fake.Observe(ctx)
	if obs.Focus.Window != priorLive {
		t.Errorf("hide-scratch-shell did not restore focus: got %q, want %q", obs.Focus.Window, priorLive)
	}
}

// TestExecuteHideScratchShellEmptyPriorIsNoop verifies the safety branch:
// Target.LiveWindow == nil → no error, no focus change.
func TestExecuteHideScratchShellEmptyPriorIsNoop(t *testing.T) {
	env := w.ManagedEnvironment{
		WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"},
		Workspaces:    w.WorkspaceEnvironment{Workspaces: []w.WorkspaceSpec{{ID: "A", RawName: "A", Role: w.WorkspaceViewer}}},
	}
	fake := wm.NewFake(env)
	ctx := context.Background()
	scratch, err := fake.ShowScratchShell(ctx)
	if err != nil {
		t.Fatalf("ShowScratchShell: %v", err)
	}
	ex := Executor{Adapter: fake, Env: env}
	if err := ex.Execute(ctx, op.Operation{Kind: op.KindHideScratchShell}, w.ObservedWorld{}, w.DesiredWorld{}); err != nil {
		t.Fatalf("Execute hide-scratch-shell with empty prior: %v", err)
	}
	obs, _ := fake.Observe(ctx)
	if obs.Focus.Window != scratch {
		t.Errorf("focus changed unexpectedly with empty prior: got %q, want %q", obs.Focus.Window, scratch)
	}
}

// TestExecuteHideCockpitWithoutPriorWindowSkipsFocus verifies the safety
// path: when Target.LiveWindow is nil (no PriorWindow captured), the
// executor must NOT panic or block; the workspace switch alone is
// sufficient and omniwm's per-workspace MRU handles focus.
func TestExecuteHideCockpitWithoutPriorWindowSkipsFocus(t *testing.T) {
	env := w.ManagedEnvironment{
		WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"},
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{
				{ID: "8", RawName: "8", Role: w.WorkspaceGeneral},
				{ID: "CP1", RawName: "CP1", Role: w.WorkspaceGeneral},
			},
		},
	}
	fake := wm.NewFake(env)
	ctx := context.Background()

	systemID := w.SystemWindowID{Kind: w.WindowCockpit, Index: 0}
	parkWs := w.WorkspaceID("CP1")
	priorWs := w.WorkspaceID("8")
	target := w.DesiredWorld{
		SystemWindows: []w.SystemWindow{{
			ID: systemID, Kind: w.WindowCockpit, DisplayIdx: 0,
			Title: "projwm-cockpit-0", ParkWorkspace: parkWs,
			Visibility: w.CockpitHidden, PriorWorkspace: priorWs,
		}},
	}
	observed, err := fake.Observe(ctx)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	observed.Displays = w.ObservedDisplayState{
		Displays: map[w.DisplayID]w.ObservedDisplay{
			"display:0": {ID: "display:0", Connected: true, ActiveWorkspace: parkWs},
		},
		WorkspaceToDisplay: map[w.WorkspaceID]w.DisplayID{parkWs: "display:0"},
	}
	ex := Executor{Adapter: fake, Env: env}
	priorWsCopy := priorWs
	if err := ex.Execute(ctx, op.Operation{
		Kind: op.KindHideCockpit,
		Target: op.Target{
			SystemWindow: &systemID,
			Workspace:    &priorWsCopy,
			// Target.LiveWindow intentionally nil — no PriorWindow yet.
		},
	}, observed, target); err != nil {
		t.Fatalf("Execute hide-cockpit (no prior window): %v", err)
	}
}
