package planner

import (
	"testing"
	"time"

	"github.com/yuu-th/projwm-next/internal/op"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// T4.4 (requirements §3.4): a managed window the user closed twice in
// the last 60 seconds must NOT be respawned by the planner. The
// reducer's KindUserClosedWindow case records the close timestamps in
// ControllerMeta.UserCloseHistory; planner reads it before emitting a
// spawn op.
//
// This test drives planner directly with a UserCloseHistory containing
// 2 recent close events and asserts no spawn op is produced.
func TestPlanner_T4_4_SuppressesRespawnAfterTwoCloses(t *testing.T) {
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Viewer: "A",
			Workspaces: []w.WorkspaceSpec{
				{ID: "A", Role: w.WorkspaceViewer},
				{ID: "Q", Role: w.WorkspaceProject},
			},
			Slots: []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
		},
	}
	dwid := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowShell, Index: 1}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {
				ID:             "work",
				InactivePolicy: w.InactivePolicyRemove,
				Assignments:    map[w.SlotID]w.ProjectID{"Q": "dotfiles"},
			},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"dotfiles": {
				ID: "dotfiles",
				Windows: []w.DesiredWindow{
					{ID: dwid, Kind: w.WindowShell, TitleContract: w.TitleContract{Authority: w.TitlePrefixOwned, Prefix: "shell-1:dotfiles"}},
				},
			},
		},
	}
	now := time.Now().UnixNano()
	state := w.WorldState{
		Environment: env,
		Desired:     desired,
		Observed:    w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{}},
		Meta: w.ControllerMeta{
			UserCloseHistory: map[w.DesiredWindowID][]int64{
				dwid: {now - int64(10*time.Second), now - int64(3*time.Second)},
			},
		},
	}
	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Operations {
		if o.Target.DesiredWindow != nil && *o.Target.DesiredWindow == dwid && isSpawnKind(o.Kind) {
			t.Errorf("planner should suppress spawn after 2 closes/60s, got op %+v", o)
		}
	}
}

// Sanity counterpart: only 1 close in 60s → planner DOES spawn.
func TestPlanner_T4_4_OneCloseDoesNotSuppress(t *testing.T) {
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Viewer: "A",
			Workspaces: []w.WorkspaceSpec{
				{ID: "A", Role: w.WorkspaceViewer},
				{ID: "Q", Role: w.WorkspaceProject},
			},
			Slots: []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
		},
	}
	dwid := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowShell, Index: 1}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", InactivePolicy: w.InactivePolicyRemove, Assignments: map[w.SlotID]w.ProjectID{"Q": "dotfiles"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"dotfiles": {
				ID: "dotfiles",
				Windows: []w.DesiredWindow{
					{ID: dwid, Kind: w.WindowShell, TitleContract: w.TitleContract{Authority: w.TitlePrefixOwned, Prefix: "shell-1:dotfiles"}},
				},
			},
		},
	}
	now := time.Now().UnixNano()
	state := w.WorldState{
		Environment: env,
		Desired:     desired,
		Observed:    w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{}},
		Meta: w.ControllerMeta{
			UserCloseHistory: map[w.DesiredWindowID][]int64{
				dwid: {now - int64(3*time.Second)},
			},
		},
	}
	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range plan.Operations {
		if o.Target.DesiredWindow != nil && *o.Target.DesiredWindow == dwid && isSpawnKind(o.Kind) {
			found = true
		}
	}
	if !found {
		t.Errorf("planner should spawn after only 1 close, got plan %+v", plan.Operations)
	}
}

// Stale closes (older than 60s) must not count.
func TestPlanner_T4_4_StaleClosesDoNotSuppress(t *testing.T) {
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Viewer: "A",
			Workspaces: []w.WorkspaceSpec{
				{ID: "A", Role: w.WorkspaceViewer},
				{ID: "Q", Role: w.WorkspaceProject},
			},
			Slots: []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
		},
	}
	dwid := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowShell, Index: 1}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", InactivePolicy: w.InactivePolicyRemove, Assignments: map[w.SlotID]w.ProjectID{"Q": "dotfiles"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"dotfiles": {
				ID: "dotfiles",
				Windows: []w.DesiredWindow{
					{ID: dwid, Kind: w.WindowShell, TitleContract: w.TitleContract{Authority: w.TitlePrefixOwned, Prefix: "shell-1:dotfiles"}},
				},
			},
		},
	}
	now := time.Now().UnixNano()
	state := w.WorldState{
		Environment: env,
		Desired:     desired,
		Observed:    w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{}},
		Meta: w.ControllerMeta{
			UserCloseHistory: map[w.DesiredWindowID][]int64{
				dwid: {now - int64(120*time.Second), now - int64(90*time.Second)},
			},
		},
	}
	plan, err := Plan(state, desired, "intent:reconcile", op.ReasonIntent)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range plan.Operations {
		if o.Target.DesiredWindow != nil && *o.Target.DesiredWindow == dwid && isSpawnKind(o.Kind) {
			found = true
		}
	}
	if !found {
		t.Errorf("stale closes should not suppress spawn")
	}
}

func isSpawnKind(k op.Kind) bool {
	switch k {
	case op.KindSpawnTerminal, op.KindSpawnEditor, op.KindSpawnBrowser, op.KindSpawnViewer:
		return true
	}
	return false
}
