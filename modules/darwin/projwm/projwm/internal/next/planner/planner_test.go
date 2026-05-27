package planner

import "testing"

func TestZeroDiffForSatisfiedWorld(t *testing.T) {
	world := World{Windows: []Window{{ID: "shell-1:dotfiles", Kind: "shell", Project: "dotfiles", Workspace: "Q"}}}
	if diff := Diff(world, world); len(diff) != 0 {
		t.Fatalf("diff = %+v, want zero", diff)
	}
}

func TestMissingWindowPlansSpawn(t *testing.T) {
	desired := World{Windows: []Window{{ID: "shell-1:dotfiles", Kind: "shell", Project: "dotfiles", Workspace: "Q"}}}
	diff := Diff(desired, World{})
	if len(diff) != 1 || diff[0].Op != "spawn" {
		t.Fatalf("diff = %+v, want one spawn", diff)
	}
}

func TestDryRunHasNoSideEffects(t *testing.T) {
	called := false
	p := Planner{Spawn: func(Window) { called = true }}
	p.Apply([]Action{{Op: "spawn", Window: Window{ID: "shell-1:dotfiles"}}}, true)
	if called {
		t.Fatal("dry-run must not spawn")
	}
}

func TestApplyExecutesPlannedSpawn(t *testing.T) {
	var spawned []Window
	p := Planner{Spawn: func(w Window) { spawned = append(spawned, w) }}
	p.Apply([]Action{{Op: "spawn", Window: Window{ID: "shell-1:dotfiles"}}}, false)
	if len(spawned) != 1 || spawned[0].ID != "shell-1:dotfiles" {
		t.Fatalf("spawned = %+v", spawned)
	}
}
