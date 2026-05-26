package reducer

import (
	"strings"
	"testing"

	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §3.3 "各状態での操作可能性" matrix: certain (state, operation)
// combinations MUST be rejected by the reducer. This is the L0
// behavioral spec for that matrix — purely tests reducer.ReduceIntent's
// reject paths without requiring a real adapter / controller.
//
// Matrix subset enforced here (SSOT §3.3):
//   - 初期状態 (no project / no profile): summon → reject, archive → reject
//   - unknown project への操作: reject
//   - unknown profile への switch: reject
//   - active profile 不在で assign/unassign: reject

func emptyWorld() w.WorldState {
	return w.WorldState{
		Desired: w.DesiredWorld{
			Profiles: map[w.ProfileID]w.DesiredProfile{},
			Projects: map[w.ProjectID]w.DesiredProject{},
		},
	}
}

func worldWithEmptyProfile() w.WorldState {
	return w.WorldState{
		Desired: w.DesiredWorld{
			ActiveProfile: "default",
			Profiles: map[w.ProfileID]w.DesiredProfile{
				"default": {ID: "default", Assignments: map[w.SlotID]w.ProjectID{}},
			},
			Projects: map[w.ProjectID]w.DesiredProject{},
		},
	}
}

// Initial state: no projects exist → archive must reject.
func TestSSOTState33_ArchiveUnknownProjectRejects(t *testing.T) {
	state := emptyWorld()
	_, err := ReduceIntent(state, intent.ArchiveProject{Project: "ghost"})
	if err == nil {
		t.Fatal("SSOT §3.3: archive of unknown project must reject in initial state")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error %q should mention 'unknown'", err.Error())
	}
}

// Initial state: no projects → unarchive of nonexistent rejects.
func TestSSOTState33_UnarchiveUnknownProjectRejects(t *testing.T) {
	state := emptyWorld()
	_, err := ReduceIntent(state, intent.UnarchiveProject{Project: "ghost"})
	if err == nil {
		t.Fatal("SSOT §3.3: unarchive of unknown project must reject")
	}
}

// Initial state: no profile → switch-profile to non-existent rejects.
func TestSSOTState33_SwitchProfileUnknownRejects(t *testing.T) {
	state := emptyWorld()
	_, err := ReduceIntent(state, intent.SwitchProfile{To: "nope"})
	if err == nil {
		t.Fatal("SSOT §3.3: switch-profile to unknown name must reject")
	}
}

// Initial state without active profile: assign-project rejects with
// "no active profile" error.
func TestSSOTState33_AssignWithNoActiveProfileRejects(t *testing.T) {
	state := emptyWorld()
	// Add a project so the project-lookup doesn't short-circuit first.
	state.Desired.Projects["p1"] = w.DesiredProject{ID: "p1"}
	_, err := ReduceIntent(state, intent.AssignProject{Slot: "Q", Project: "p1"})
	if err == nil {
		t.Fatal("SSOT §3.3: assign with no active profile must reject")
	}
	if !strings.Contains(err.Error(), "no active profile") {
		t.Errorf("error %q should mention 'no active profile'", err.Error())
	}
}

// Unassign rejects without active profile.
func TestSSOTState33_UnassignWithNoActiveProfileRejects(t *testing.T) {
	state := emptyWorld()
	_, err := ReduceIntent(state, intent.UnassignSlot{Slot: "Q"})
	if err == nil {
		t.Fatal("SSOT §3.3: unassign with no active profile must reject")
	}
}

// assign of unknown project rejects (project must exist before assign).
func TestSSOTState33_AssignUnknownProjectRejects(t *testing.T) {
	state := worldWithEmptyProfile()
	_, err := ReduceIntent(state, intent.AssignProject{Slot: "Q", Project: "ghost"})
	if err == nil {
		t.Fatal("SSOT §3.3: assign of unknown project must reject")
	}
}

// AddWindow rejects on unknown project.
func TestSSOTState33_AddWindowUnknownProjectRejects(t *testing.T) {
	state := emptyWorld()
	_, err := ReduceIntent(state, intent.AddWindow{Project: "ghost", WindowKind: w.WindowShell})
	if err == nil {
		t.Fatal("SSOT §3.3: add-window on unknown project must reject")
	}
}

// RemoveWindow rejects on unknown project.
func TestSSOTState33_RemoveWindowUnknownProjectRejects(t *testing.T) {
	state := emptyWorld()
	_, err := ReduceIntent(state, intent.RemoveWindow{
		Project:  "ghost",
		WindowID: w.DesiredWindowID{Project: "ghost", Kind: w.WindowShell, Index: 1},
	})
	if err == nil {
		t.Fatal("SSOT §3.3: remove-window on unknown project must reject")
	}
}

// DeleteProject rejects on unknown project.
func TestSSOTState33_DeleteUnknownProjectRejects(t *testing.T) {
	state := emptyWorld()
	_, err := ReduceIntent(state, intent.DeleteProject{ID: "ghost"})
	if err == nil {
		t.Fatal("SSOT §3.3: delete unknown project must reject")
	}
}

// DeleteProfile rejects on unknown profile.
func TestSSOTState33_DeleteUnknownProfileRejects(t *testing.T) {
	state := emptyWorld()
	_, err := ReduceIntent(state, intent.DeleteProfile{ID: "ghost"})
	if err == nil {
		t.Fatal("SSOT §3.3: delete unknown profile must reject")
	}
}

// Browser tab operations on a non-browser window kind must reject
// (SSOT §4.1 OP14-17 implicit: tabs only make sense on browser kind).
func TestSSOTState33_BrowserAddTabRejectsOnUnknownProject(t *testing.T) {
	state := emptyWorld()
	_, err := ReduceIntent(state, intent.BrowserAddTab{
		Project:  "ghost",
		WindowID: w.DesiredWindowID{Project: "ghost", Kind: w.WindowBrowser, Index: 1},
		URL:      "https://example.com",
	})
	if err == nil {
		t.Fatal("SSOT §3.3 + §4.1 OP14: browser-add-tab on unknown project must reject")
	}
}
