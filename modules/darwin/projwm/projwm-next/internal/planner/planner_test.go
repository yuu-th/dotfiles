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

func TestPlanRejectsAmbiguousActiveDesiredWindow(t *testing.T) {
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
	_, err := Plan(w.WorldState{
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
			Layouts: map[w.WorkspaceID]w.ObservedLayout{},
		},
	}, desired, "intent:reconcile", op.ReasonIntent)
	if err == nil {
		t.Fatalf("Plan succeeded, want ambiguous active desired identity rejection")
	}
	if !strings.Contains(err.Error(), "classified ambiguous") || !strings.Contains(err.Error(), "unique-strong") {
		t.Fatalf("Plan error = %v, want unique-strong ambiguity evidence", err)
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

func hasOperationKind(ops []op.Operation, kind op.Kind) bool {
	for _, operation := range ops {
		if operation.Kind == kind {
			return true
		}
	}
	return false
}
