package planner

import (
	"strings"
	"testing"

	"github.com/yuu-th/projwm-next/internal/op"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestPlanDoesNotCloseLiveWindowWithoutDesiredIdentityEvidence(t *testing.T) {
	desired := w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{},
	}
	plan, err := Plan(w.WorldState{
		Environment: w.ManagedEnvironment{
			Workspaces: w.WorkspaceEnvironment{
				Workspaces: []w.WorkspaceSpec{{ID: "Q", Role: w.WorkspaceProject}},
			},
		},
		Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"orphan": {ID: "orphan", Kind: w.WindowShell, Workspace: "Q"},
			},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, operation := range plan.Operations {
		if operation.Kind == op.KindCloseWindow {
			t.Fatalf("planner must not close orphan live window without identity evidence: %+v", operation)
		}
	}
}

// TestPlanRejectsAmbiguousActiveDesiredWindow was renamed: SSOT §2.5 EC4 /
// INV-01 mandates that the planner *does not* reject the plan on duplicates,
// it picks the focus-tiebreak winner and continues. The duplicate is surfaced
// as a Check14 invariant violation post-convergence (= [INVARIANT] card).
//
// Renamed to TestPlanAcceptsAmbiguousActiveDesiredWindowViaFocusTiebreak to
// reflect the new behavior.
func TestPlanAcceptsAmbiguousActiveDesiredWindowViaFocusTiebreak(t *testing.T) {
	desiredID := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	desiredWindow := w.DesiredWindow{
		ID:   desiredID,
		Kind: w.WindowShell,
		App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
		TitleContract: w.TitleContract{
			Authority: w.TitleControllerOwned,
			Expected:  "shell-1:p1",
		},
	}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{desiredWindow}},
		},
	}
	plan, err := Plan(w.WorldState{
		Environment: w.ManagedEnvironment{
			Workspaces: w.WorkspaceEnvironment{
				Slots: []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
			},
		},
		Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"live-1": {ID: "live-1", Kind: w.WindowShell, Workspace: "Q", App: w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"}, Title: w.ObservedTitle{Value: "shell-1:p1"}},
				"live-2": {ID: "live-2", Kind: w.WindowShell, Workspace: "Q", App: w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"}, Title: w.ObservedTitle{Value: "shell-1:p1"}},
			},
			// "live-2" is focused — focus-tiebreak should pick it as winner.
			Focus:   w.ObservedFocus{Window: "live-2", Workspace: "Q"},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatalf("Plan errored on duplicate (should focus-tiebreak): %v", err)
	}
	// Verify the planner identified live-2 (the focused one) as the winner —
	// no close-window op is emitted for live-2; live-1 isn't auto-closed either
	// (orphan handling is via Check14 invariant card, not planner removal).
	for _, o := range plan.Operations {
		if o.Kind == op.KindCloseWindow || o.Kind == op.KindKillSession {
			t.Errorf("planner auto-closed candidate on duplicate (SSOT INV-01 forbids auto-close, must use [INVARIANT] card): %+v", o)
		}
	}
}

func TestPlanRejectsAmbiguousViewerBeforeSpawningDuplicate(t *testing.T) {
	aiID := w.DesiredWindowID{Project: "p1", Kind: w.WindowAI, Index: 1}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{{
				ID:   aiID,
				Kind: w.WindowAI,
				App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
				TitleContract: w.TitleContract{
					Authority: w.TitleControllerOwned,
					Expected:  "ai-1:p1",
				},
			}}},
		},
	}
	_, err := Plan(w.WorldState{
		Environment: w.ManagedEnvironment{
			Workspaces: w.WorkspaceEnvironment{
				Viewer:     "A",
				Slots:      []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
				Workspaces: []w.WorkspaceSpec{{ID: "A", Role: w.WorkspaceViewer}, {ID: "Q", Role: w.WorkspaceProject}},
			},
		},
		Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"ai-live":  {ID: "ai-live", Kind: w.WindowAI, Workspace: "Q", App: w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"}, Title: w.ObservedTitle{Value: "ai-1:p1"}, MatchedTo: &aiID},
				"viewer-1": {ID: "viewer-1", Kind: w.WindowViewer, Workspace: "A", Title: w.ObservedTitle{Value: "ai-view-1:p1"}},
				"viewer-2": {ID: "viewer-2", Kind: w.WindowViewer, Workspace: "Q", Title: w.ObservedTitle{Value: "ai-view-1:p1"}},
			},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "intent:reconcile", op.ReasonIntent)
	if err == nil {
		t.Fatal("Plan succeeded, want ambiguous viewer rejection before duplicate spawn")
	}
	if !strings.Contains(err.Error(), "viewer identity") || !strings.Contains(err.Error(), "refusing duplicate-prone spawn") {
		t.Fatalf("Plan error = %v", err)
	}
}

func TestPlanDefersViewerSpawnUntilSourceAIIsObservedUniqueStrong(t *testing.T) {
	aiID := w.DesiredWindowID{Project: "p1", Kind: w.WindowAI, Index: 1}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{{
				ID:   aiID,
				Kind: w.WindowAI,
				App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
				TitleContract: w.TitleContract{
					Authority: w.TitleControllerOwned,
					Expected:  "ai-1:p1",
				},
			}}},
		},
	}
	plan, err := Plan(w.WorldState{
		Environment: w.ManagedEnvironment{
			Workspaces: w.WorkspaceEnvironment{
				Viewer:     "A",
				Slots:      []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
				Workspaces: []w.WorkspaceSpec{{ID: "A", Role: w.WorkspaceViewer}, {ID: "Q", Role: w.WorkspaceProject}},
			},
		},
		Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, operation := range plan.Operations {
		if operation.Kind == op.KindSpawnViewer {
			t.Fatalf("planner must not spawn viewer before source AI is observed unique-strong: %+v", plan.Operations)
		}
	}
	if !hasOperationKind(plan.Operations, op.KindSpawnTerminal) {
		t.Fatalf("planner should still spawn the source AI terminal: %+v", plan.Operations)
	}
}

func TestPlanBlocksCloseWindowForProductionBackends(t *testing.T) {
	desiredID := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	desiredWindow := w.DesiredWindow{
		ID:   desiredID,
		Kind: w.WindowShell,
		App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
		TitleContract: w.TitleContract{
			Authority: w.TitleControllerOwned,
			Expected:  "shell-1:p1",
		},
	}
	desired := w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{desiredWindow}},
		},
	}
	plan, err := Plan(w.WorldState{
		Environment: w.ManagedEnvironment{
			WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"},
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
		},
		Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"live": {ID: "live", Kind: w.WindowShell, Workspace: "Q", Title: w.ObservedTitle{Value: "shell-1:p1"}, MatchedTo: &desiredID},
			},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, operation := range plan.Operations {
		if operation.Kind == op.KindCloseWindow {
			t.Fatalf("planner must not emit close-window for production backend: %+v", operation)
		}
	}
	if !hasOperationKind(plan.Operations, op.KindKillSession) {
		t.Fatalf("planner should emit semantic lifecycle removal for controller-owned managed shell: %+v", plan.Operations)
	}
}

func TestPlanRemovesInactiveControllerOwnedGhosttyByTitleWithoutMatchedToEvidence(t *testing.T) {
	desiredID := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	desiredWindow := w.DesiredWindow{
		ID:   desiredID,
		Kind: w.WindowShell,
		App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
		TitleContract: w.TitleContract{
			Authority: w.TitleControllerOwned,
			Expected:  "shell-1:p1",
		},
	}
	desired := w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{desiredWindow}},
		},
	}
	plan, err := Plan(w.WorldState{
		Environment: w.ManagedEnvironment{
			WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"},
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
		},
		Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"live": {
					ID:        "live",
					Kind:      w.WindowShell,
					Workspace: "Q",
					App:       w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"},
					Title:     w.ObservedTitle{Value: "shell-1:p1"},
				},
			},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "intent:switch-profile", op.ReasonIntent)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !hasOperationKind(plan.Operations, op.KindKillSession) {
		t.Fatalf("planner should infer controller-owned Ghostty desired identity and emit lifecycle removal: %+v", plan.Operations)
	}
}

func TestPlanMoveCarriesDesiredIdentityForWorkspaceDrift(t *testing.T) {
	desiredID := w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1}
	desiredWindow := w.DesiredWindow{
		ID:   desiredID,
		Kind: w.WindowShell,
		App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
		TitleContract: w.TitleContract{
			Authority: w.TitleControllerOwned,
			Expected:  "shell-1:p1",
		},
	}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{desiredWindow}},
		},
	}
	plan, err := Plan(w.WorldState{
		Environment: w.ManagedEnvironment{
			Workspaces: w.WorkspaceEnvironment{
				Slots: []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
			},
		},
		Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"live": {
					ID:        "live",
					Kind:      w.WindowShell,
					Workspace: "3",
					App:       w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"},
					Title:     w.ObservedTitle{Value: "shell-1:p1"},
				},
			},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "event:external", op.ReasonEvent)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, operation := range plan.Operations {
		if operation.Kind != op.KindMoveWindowToWorkspace {
			continue
		}
		if operation.Target.DesiredWindow == nil || *operation.Target.DesiredWindow != desiredID {
			t.Fatalf("move operation must carry desired identity evidence: %+v", operation)
		}
		for _, precondition := range operation.Preconditions {
			if precondition.Kind == op.PreUniqueStrong && precondition.Target.DesiredWindow != nil && *precondition.Target.DesiredWindow == desiredID {
				return
			}
		}
		t.Fatalf("move operation PreUniqueStrong must carry desired identity evidence: %+v", operation)
	}
	t.Fatalf("planner did not emit move operation: %+v", plan.Operations)
}

func TestPlanRemovesInactiveViewerByTitleWithoutMatchedToEvidence(t *testing.T) {
	aiID := w.DesiredWindowID{Project: "p1", Kind: w.WindowAI, Index: 1}
	desiredWindow := w.DesiredWindow{
		ID:   aiID,
		Kind: w.WindowAI,
		App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
		TitleContract: w.TitleContract{
			Authority: w.TitleControllerOwned,
			Expected:  "ai-1:p1",
		},
	}
	desired := w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{desiredWindow}},
		},
	}
	plan, err := Plan(w.WorldState{
		Environment: w.ManagedEnvironment{
			WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"},
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
				Viewer:     "A",
				Workspaces: []w.WorkspaceSpec{{ID: "A", Role: w.WorkspaceViewer}},
			},
		},
		Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"viewer": {
					ID:        "viewer",
					Kind:      w.WindowViewer,
					Workspace: "A",
					App:       w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"},
					Title:     w.ObservedTitle{Value: "ai-view-1:p1"},
				},
			},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "intent:switch-profile", op.ReasonIntent)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !hasOperationKind(plan.Operations, op.KindKillSession) {
		t.Fatalf("planner should infer stale viewer desired identity by title and emit lifecycle removal: %+v", plan.Operations)
	}
}

func TestPlanDoesNotEmitLifecycleRemovalForUnprovenProductionAppKinds(t *testing.T) {
	for _, kind := range []w.WindowKind{w.WindowEditor, w.WindowBrowser} {
		t.Run(string(kind), func(t *testing.T) {
			desiredID := w.DesiredWindowID{Project: "p1", Kind: kind, Index: 1}
			desiredWindow := w.DesiredWindow{
				ID:   desiredID,
				Kind: kind,
				App:  w.AppRequirement{BundleID: "com.example." + string(kind)},
				TitleContract: w.TitleContract{
					Authority: w.TitleControllerOwned,
					Expected:  string(kind) + "-1:p1",
				},
			}
			desired := w.DesiredWorld{
				ActiveProfile: "empty",
				Profiles: map[w.ProfileID]w.DesiredProfile{
					"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
				},
				Projects: map[w.ProjectID]w.DesiredProject{
					"p1": {ID: "p1", Windows: []w.DesiredWindow{desiredWindow}},
				},
			}
			plan, err := Plan(w.WorldState{
				Environment: w.ManagedEnvironment{
					WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"},
					Apps: w.AppEnvironment{ManagedApps: []w.ManagedAppPolicy{{
						Capability: w.AppCapability(kind),
						BundleID:   "com.example." + string(kind),
						LifecycleRemoval: w.LifecycleRemovalPolicy{
							Allowed: false,
							Method:  w.LifecycleRemovalProjectScopedApp,
						},
					}}},
					Workspaces: w.WorkspaceEnvironment{
						Workspaces: []w.WorkspaceSpec{{ID: "Q", Role: w.WorkspaceProject}},
					},
				},
				Desired: desired,
				Observed: w.ObservedWorld{
					Windows: map[w.LiveWindowID]w.ObservedWindow{
						"live": {ID: "live", Kind: kind, Workspace: "Q", Title: w.ObservedTitle{Value: string(kind) + "-1:p1"}, MatchedTo: &desiredID},
					},
					Layouts: map[w.WorkspaceID]w.ObservedLayout{},
				},
			}, desired, "intent:reconcile", op.ReasonIntent)
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if hasOperationKind(plan.Operations, op.KindCloseWindow) || hasOperationKind(plan.Operations, op.KindKillSession) {
				t.Fatalf("planner must not remove unproven production app kind %s: %+v", kind, plan.Operations)
			}
		})
	}
}

// TestPlanBlocksZedAndVivaldiWhenLifecyclePolicyIsNotAllowed verifies that when the
// ManagedAppPolicy explicitly denies lifecycle removal (Allowed=false), the planner
// does not emit a removal operation even though the production-shaped close-window
// primitives (project-scoped-app / browser-window-close) are now wired in the executor.
func TestPlanBlocksZedAndVivaldiWhenLifecyclePolicyIsNotAllowed(t *testing.T) {
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
			titleContract := w.TitleContract{
				Authority: w.TitleAppOwned,
				Expected:  "app-owned-title",
			}
			observedTitle := "app-owned-title"
			if tc.kind == w.WindowBrowser {
				titleContract = w.TitleContract{
					Authority: w.TitleControllerOwned,
					Expected:  "browser-1:p1",
					Drift:     w.TitleDriftRepair,
				}
				observedTitle = "browser-1:p1"
			}
			desiredWindow := w.DesiredWindow{
				ID:            desiredID,
				Kind:          tc.kind,
				App:           w.AppRequirement{Capability: tc.capability, BundleID: tc.bundle},
				TitleContract: titleContract,
			}
			desired := w.DesiredWorld{
				ActiveProfile: "empty",
				Profiles: map[w.ProfileID]w.DesiredProfile{
					"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
				},
				Projects: map[w.ProjectID]w.DesiredProject{
					"p1": {ID: "p1", Windows: []w.DesiredWindow{desiredWindow}},
				},
			}
			plan, err := Plan(w.WorldState{
				Environment: w.ManagedEnvironment{
					WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"},
					Apps: w.AppEnvironment{ManagedApps: []w.ManagedAppPolicy{{
						Capability: tc.capability,
						BundleID:   tc.bundle,
						LifecycleRemoval: w.LifecycleRemovalPolicy{
							Allowed:      false,
							Method:       tc.method,
							AllowedKinds: []w.WindowKind{tc.kind},
						},
					}}},
					Workspaces: w.WorkspaceEnvironment{
						Workspaces: []w.WorkspaceSpec{{ID: "Q", Role: w.WorkspaceProject}},
					},
				},
				Desired: desired,
				Observed: w.ObservedWorld{
					Windows: map[w.LiveWindowID]w.ObservedWindow{
						"live": {ID: "live", Kind: tc.kind, Workspace: "Q", App: w.ObservedAppRef{BundleID: tc.bundle}, Title: w.ObservedTitle{Value: observedTitle}, MatchedTo: &desiredID},
					},
					Layouts: map[w.WorkspaceID]w.ObservedLayout{},
				},
			}, desired, "intent:reconcile", op.ReasonIntent)
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if hasOperationKind(plan.Operations, op.KindCloseWindow) || hasOperationKind(plan.Operations, op.KindKillSession) {
				t.Fatalf("planner must respect ManagedAppPolicy.Allowed=false for %s: %+v", tc.name, plan.Operations)
			}
		})
	}
}

// TestPlanEmitsZedProjectScopedRemovalWhenAllowed verifies that with the production-shaped
// close-window primitive unblocked (Allowed=true, Method=ProjectScopedApp), the planner
// emits a KindKillSession operation carrying the project-scoped-app removal method.
func TestPlanEmitsZedProjectScopedRemovalWhenAllowed(t *testing.T) {
	desiredID := w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 1}
	desiredWindow := w.DesiredWindow{
		ID:   desiredID,
		Kind: w.WindowEditor,
		App:  w.AppRequirement{Capability: w.CapabilityEditor, BundleID: "dev.zed.Zed"},
		TitleContract: w.TitleContract{
			Authority: w.TitleAppOwned,
			Expected:  "app-owned-title",
		},
	}
	desired := w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{desiredWindow}},
		},
	}
	plan, err := Plan(w.WorldState{
		Environment: w.ManagedEnvironment{
			WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"},
			Apps: w.AppEnvironment{ManagedApps: []w.ManagedAppPolicy{{
				Capability: w.CapabilityEditor,
				BundleID:   "dev.zed.Zed",
				LifecycleRemoval: w.LifecycleRemovalPolicy{
					Allowed:      true,
					Method:       w.LifecycleRemovalProjectScopedApp,
					AllowedKinds: []w.WindowKind{w.WindowEditor},
				},
			}}},
			Workspaces: w.WorkspaceEnvironment{
				Workspaces: []w.WorkspaceSpec{{ID: "Q", Role: w.WorkspaceProject}},
			},
		},
		Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"live": {ID: "live", Kind: w.WindowEditor, Workspace: "Q", App: w.ObservedAppRef{BundleID: "dev.zed.Zed"}, Title: w.ObservedTitle{Value: "app-owned-title"}, MatchedTo: &desiredID},
			},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !hasOperationKind(plan.Operations, op.KindKillSession) {
		t.Fatalf("planner should emit kill-session for Zed editor when production primitive is allowed: %+v", plan.Operations)
	}
	for _, operation := range plan.Operations {
		if operation.Kind == op.KindKillSession && operation.LifecycleRemovalMethod != w.LifecycleRemovalProjectScopedApp {
			t.Fatalf("Zed kill-session must carry project-scoped-app method, got %q: %+v", operation.LifecycleRemovalMethod, operation)
		}
	}
}

// TestPlanEmitsVivaldiBrowserWindowCloseWhenAllowed verifies that with the production-shaped
// close-window primitive unblocked (Allowed=true, Method=BrowserWindowClose), the planner
// emits a KindKillSession operation carrying the browser-window-close removal method.
func TestPlanEmitsVivaldiBrowserWindowCloseWhenAllowed(t *testing.T) {
	desiredID := w.DesiredWindowID{Project: "p1", Kind: w.WindowBrowser, Index: 1}
	desiredWindow := w.DesiredWindow{
		ID:   desiredID,
		Kind: w.WindowBrowser,
		App:  w.AppRequirement{Capability: w.CapabilityBrowser, BundleID: "com.vivaldi.Vivaldi"},
		TitleContract: w.TitleContract{
			Authority: w.TitleControllerOwned,
			Expected:  "browser-1:p1",
			Drift:     w.TitleDriftRepair,
		},
	}
	desired := w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{desiredWindow}},
		},
	}
	plan, err := Plan(w.WorldState{
		Environment: w.ManagedEnvironment{
			WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"},
			Apps: w.AppEnvironment{ManagedApps: []w.ManagedAppPolicy{{
				Capability: w.CapabilityBrowser,
				BundleID:   "com.vivaldi.Vivaldi",
				LifecycleRemoval: w.LifecycleRemovalPolicy{
					Allowed:      true,
					Method:       w.LifecycleRemovalBrowserWindowClose,
					AllowedKinds: []w.WindowKind{w.WindowBrowser},
				},
			}}},
			Workspaces: w.WorkspaceEnvironment{
				Workspaces: []w.WorkspaceSpec{{ID: "Q", Role: w.WorkspaceProject}},
			},
		},
		Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"live": {ID: "live", Kind: w.WindowBrowser, Workspace: "Q", App: w.ObservedAppRef{BundleID: "com.vivaldi.Vivaldi"}, Title: w.ObservedTitle{Value: "browser-1:p1"}, MatchedTo: &desiredID},
			},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !hasOperationKind(plan.Operations, op.KindKillSession) {
		t.Fatalf("planner should emit kill-session for Vivaldi browser when production primitive is allowed: %+v", plan.Operations)
	}
	for _, operation := range plan.Operations {
		if operation.Kind == op.KindKillSession && operation.LifecycleRemovalMethod != w.LifecycleRemovalBrowserWindowClose {
			t.Fatalf("Vivaldi kill-session must carry browser-window-close method, got %q: %+v", operation.LifecycleRemovalMethod, operation)
		}
	}
}

// TestPlanner_Viewer_RevertsToViewerWorkspace verifies that when an active viewer
// window is observed at a non-viewer workspace, the planner emits a
// MoveWindowToWorkspace op targeting the viewer workspace (T4 revert).
func TestPlanner_Viewer_RevertsToViewerWorkspace(t *testing.T) {
	aiID := w.DesiredWindowID{Project: "p1", Kind: w.WindowAI, Index: 1}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{{
				ID:   aiID,
				Kind: w.WindowAI,
				App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
				TitleContract: w.TitleContract{
					Authority: w.TitleControllerOwned,
					Expected:  "ai-1:p1",
				},
			}}},
		},
	}
	plan, err := Plan(w.WorldState{
		Environment: w.ManagedEnvironment{
			Workspaces: w.WorkspaceEnvironment{
				Viewer:     "A",
				Slots:      []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
				Workspaces: []w.WorkspaceSpec{{ID: "A", Role: w.WorkspaceViewer}, {ID: "Q", Role: w.WorkspaceProject}, {ID: "M", Role: w.WorkspaceProject}},
			},
		},
		Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"ai-live": {ID: "ai-live", Kind: w.WindowAI, Workspace: "Q",
					App: w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"}, Title: w.ObservedTitle{Value: "ai-1:p1"}, MatchedTo: &aiID},
				// viewer is stranded on workspace M, not on viewer workspace A
				"viewer-1": {ID: "viewer-1", Kind: w.WindowViewer, Workspace: "M",
					Title: w.ObservedTitle{Value: "ai-view-1:p1"}},
			},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	// Must contain a MoveWindowToWorkspace op targeting workspace A.
	for _, oper := range plan.Operations {
		if oper.Kind != op.KindMoveWindowToWorkspace {
			continue
		}
		if oper.Target.Workspace == nil || *oper.Target.Workspace != "A" {
			continue
		}
		if oper.Target.LiveWindow == nil || *oper.Target.LiveWindow != "viewer-1" {
			continue
		}
		return // found expected revert op
	}
	t.Fatalf("planner did not emit MoveWindowToWorkspace for stranded viewer: %+v", plan.Operations)
}

// TestPlanner_Viewer_NoSpuriousSpawnWhenViewerAtWrongWorkspace verifies that when
// an active viewer window exists at a non-viewer workspace, the planner does NOT
// emit a spawn-viewer op (the viewer already exists; spawning would create a duplicate).
func TestPlanner_Viewer_NoSpuriousSpawnWhenViewerAtWrongWorkspace(t *testing.T) {
	aiID := w.DesiredWindowID{Project: "p1", Kind: w.WindowAI, Index: 1}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{{
				ID:   aiID,
				Kind: w.WindowAI,
				App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
				TitleContract: w.TitleContract{
					Authority: w.TitleControllerOwned,
					Expected:  "ai-1:p1",
				},
			}}},
		},
	}
	plan, err := Plan(w.WorldState{
		Environment: w.ManagedEnvironment{
			Workspaces: w.WorkspaceEnvironment{
				Viewer:     "A",
				Slots:      []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
				Workspaces: []w.WorkspaceSpec{{ID: "A", Role: w.WorkspaceViewer}, {ID: "Q", Role: w.WorkspaceProject}, {ID: "M", Role: w.WorkspaceProject}},
			},
		},
		Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"ai-live": {ID: "ai-live", Kind: w.WindowAI, Workspace: "Q",
					App: w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"}, Title: w.ObservedTitle{Value: "ai-1:p1"}, MatchedTo: &aiID},
				// viewer is stranded on workspace M, not on viewer workspace A
				"viewer-1": {ID: "viewer-1", Kind: w.WindowViewer, Workspace: "M",
					Title: w.ObservedTitle{Value: "ai-view-1:p1"}},
			},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, oper := range plan.Operations {
		if oper.Kind == op.KindSpawnViewer {
			t.Fatalf("planner must not spawn a viewer when the viewer already exists at wrong workspace: %+v", plan.Operations)
		}
	}
}

// TestPlanner_Viewer_StaleViewerAtWrongWorkspaceRemoved verifies that a stale viewer
// (no active AI match) observed at a non-viewer workspace is removed.
func TestPlanner_Viewer_StaleViewerAtWrongWorkspaceRemoved(t *testing.T) {
	aiID := w.DesiredWindowID{Project: "p1", Kind: w.WindowAI, Index: 1}
	desired := w.DesiredWorld{
		// "empty" profile: p1 is not active → its AI has no viewer slot
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{{
				ID:   aiID,
				Kind: w.WindowAI,
				App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
				TitleContract: w.TitleContract{
					Authority: w.TitleControllerOwned,
					Expected:  "ai-1:p1",
				},
			}}},
		},
	}
	plan, err := Plan(w.WorldState{
		Environment: w.ManagedEnvironment{
			WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"},
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
				Viewer:     "A",
				Workspaces: []w.WorkspaceSpec{{ID: "A", Role: w.WorkspaceViewer}, {ID: "M", Role: w.WorkspaceProject}},
			},
		},
		Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				// stale viewer on wrong workspace: project p1 is not active
				"viewer-stale": {ID: "viewer-stale", Kind: w.WindowViewer, Workspace: "M",
					App: w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"}, Title: w.ObservedTitle{Value: "ai-view-1:p1"}},
			},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, oper := range plan.Operations {
		if oper.Kind == op.KindKillSession || oper.Kind == op.KindCloseWindow {
			if oper.Target.LiveWindow != nil && *oper.Target.LiveWindow == "viewer-stale" {
				return // found expected removal
			}
		}
	}
	t.Fatalf("planner should remove stale viewer at wrong workspace: %+v", plan.Operations)
}

// opPhase classifies an op.Kind into one of the SSOT §6.10 / §7.1 planner
// phases (removal → barrier → spawn → barrier → layout). Returns "" for
// kinds that are not phase-classified here.
func opPhase(k op.Kind) string {
	switch k {
	case op.KindKillSession, op.KindCloseWindow, op.KindCloseCockpit:
		return "removal"
	case op.KindSpawnTerminal, op.KindSpawnEditor, op.KindSpawnBrowser,
		op.KindSpawnViewer, op.KindSpawnCockpit, op.KindEnsureSession:
		return "spawn"
	case op.KindMoveWindowToWorkspace, op.KindReorderColumns, op.KindMoveColumn,
		op.KindMoveStackMember, op.KindToggleTabbed, op.KindFocusWorkspace,
		op.KindFocusWindow, op.KindMoveCockpitToParkWorkspace,
		op.KindShowCockpit, op.KindHideCockpit:
		return "layout"
	default:
		return ""
	}
}

// TestPlanPhaseOrderRemovalBarrierSpawnBarrierLayout is the owner test for
// SSOT §6.10 (operation order) / §7.1 (planner phase separation) — previously
// §10.9 GAP-18. A single reconcile that simultaneously requires a removal, a
// spawn, AND a layout move must emit them in the order:
//
//	removals… → observe-barrier → spawns… → observe-barrier → layout…
//
// so that closed slots are vacated (and observed gone) before new windows are
// spawned, and spawned windows are observed before being moved/reordered.
func TestPlanPhaseOrderRemovalBarrierSpawnBarrierLayout(t *testing.T) {
	shell := func(project string) w.DesiredWindow {
		return w.DesiredWindow{
			ID:   w.DesiredWindowID{Project: w.ProjectID(project), Kind: w.WindowShell, Index: 1},
			Kind: w.WindowShell,
			App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
			TitleContract: w.TitleContract{
				Authority: w.TitleControllerOwned,
				Expected:  "shell-1:" + project,
			},
		}
	}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			// p_new and p_drift are active; p_old is NOT assigned → inactive.
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "p_new", "W": "p_drift"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p_new":   {ID: "p_new", Windows: []w.DesiredWindow{shell("p_new")}},
			"p_drift": {ID: "p_drift", Windows: []w.DesiredWindow{shell("p_drift")}},
			"p_old":   {ID: "p_old", Windows: []w.DesiredWindow{shell("p_old")}},
		},
	}
	plan, err := Plan(w.WorldState{
		Environment: w.ManagedEnvironment{
			WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"},
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
				Slots: []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}, {ID: "W", Workspace: "W", Order: 2}},
			},
		},
		Desired: desired,
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				// p_old (inactive) live window → must be removed.
				"live-old": {ID: "live-old", Kind: w.WindowShell, Workspace: "Q",
					App: w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"}, Title: w.ObservedTitle{Value: "shell-1:p_old"}},
				// p_drift's window exists but on the wrong workspace ("3" ≠ "W") → must be moved (layout).
				"live-drift": {ID: "live-drift", Kind: w.WindowShell, Workspace: "3",
					App: w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"}, Title: w.ObservedTitle{Value: "shell-1:p_drift"}},
				// p_new's window is absent → must be spawned.
			},
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	// Collect the index of each phase op + barriers.
	lastRemoval, firstSpawn, lastSpawn, firstLayout := -1, -1, -1, -1
	var barrierIdx []int
	sawPhase := map[string]bool{}
	for i, oper := range plan.Operations {
		if oper.Kind == op.KindObserveBarrier {
			barrierIdx = append(barrierIdx, i)
			continue
		}
		switch opPhase(oper.Kind) {
		case "removal":
			lastRemoval = i
			sawPhase["removal"] = true
		case "spawn":
			if firstSpawn == -1 {
				firstSpawn = i
			}
			lastSpawn = i
			sawPhase["spawn"] = true
		case "layout":
			if firstLayout == -1 {
				firstLayout = i
			}
			sawPhase["layout"] = true
		}
	}

	// The fixture is designed to exercise all three phases at once.
	for _, p := range []string{"removal", "spawn", "layout"} {
		if !sawPhase[p] {
			t.Fatalf("fixture failed to produce a %q-phase op; plan=%+v", p, plan.Operations)
		}
	}

	// §6.10: every removal precedes every spawn, every spawn precedes every layout.
	if lastRemoval >= firstSpawn {
		t.Errorf("SSOT §6.10: removal (idx %d) must precede spawn (idx %d)", lastRemoval, firstSpawn)
	}
	if lastSpawn >= firstLayout {
		t.Errorf("SSOT §6.10: spawn (idx %d) must precede layout (idx %d)", lastSpawn, firstLayout)
	}

	// §7.1: an observe-barrier separates removal→spawn and spawn→layout.
	hasBarrierBetween := func(lo, hi int) bool {
		for _, b := range barrierIdx {
			if b > lo && b < hi {
				return true
			}
		}
		return false
	}
	if !hasBarrierBetween(lastRemoval, firstSpawn) {
		t.Errorf("SSOT §7.1: observe-barrier required between removal (idx %d) and spawn (idx %d); barriers=%v", lastRemoval, firstSpawn, barrierIdx)
	}
	if !hasBarrierBetween(lastSpawn, firstLayout) {
		t.Errorf("SSOT §7.1: observe-barrier required between spawn (idx %d) and layout (idx %d); barriers=%v", lastSpawn, firstLayout, barrierIdx)
	}
}

func hasOperationKind(ops []op.Operation, kind op.Kind) bool {
	for _, operation := range ops {
		if operation.Kind == kind {
			return true
		}
	}
	return false
}
