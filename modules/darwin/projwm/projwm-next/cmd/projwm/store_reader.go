package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/yuu-th/projwm-next/internal/manifest"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// WorldSnapshot is the combined view a projwm CLI command needs to render
// status / list / show output. It can be loaded either from the daemon's
// IPC (future) or directly from the persistent store (current fallback).
type WorldSnapshot struct {
	Generation      w.GenerationID
	ParentGeneration *w.GenerationID
	Environment     w.ManagedEnvironment
	Desired         w.DesiredWorld
	AcceptedLayouts map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout
	Checkpoint      store.ControllerCheckpoint
}

// loadSnapshotFromStore reads the current generation from the FileStore
// at gf.storeDir and combines it with the manifest at gf.manifestPath.
//
// This path runs without contacting projwmd, so `projwm status / doctor /
// list` work even when the daemon is stopped.
func loadSnapshotFromStore(ctx context.Context, gf globalFlags) (WorldSnapshot, error) {
	if gf.storeDir == "" {
		return WorldSnapshot{}, fmt.Errorf("store-dir is required (--store-dir or PROJWM_NEXT_STORE_DIR)")
	}
	if gf.manifestPath == "" {
		return WorldSnapshot{}, fmt.Errorf("managed-environment manifest is required (--managed-environment or PROJWM_NEXT_MANAGED_ENVIRONMENT)")
	}
	env, err := manifest.LoadFromFile(gf.manifestPath)
	if err != nil {
		return WorldSnapshot{}, err
	}
	// Detect the store kind from the .store_identity.json file so we can
	// open the store read-only regardless of whether it was bootstrapped as
	// production or test.
	kind, err := detectStoreKind(gf.storeDir)
	if err != nil {
		return WorldSnapshot{}, err
	}
	fs, err := store.OpenExistingFileStore(ctx, gf.storeDir, kind)
	if err != nil {
		return WorldSnapshot{}, err
	}
	gen, err := fs.LoadCurrentGeneration(ctx)
	if err != nil {
		return WorldSnapshot{}, err
	}
	return WorldSnapshot{
		Generation:       gen.ID,
		ParentGeneration: gen.Parent,
		Environment:      env,
		Desired:          gen.Desired,
		AcceptedLayouts:  gen.AcceptedLayouts,
		Checkpoint:       gen.Checkpoint,
	}, nil
}

// detectStoreKind reads .store_identity.json from a store root and returns
// the recorded StoreKind. Used so projwm CLI doesn't need to know upfront
// whether a store was bootstrapped as test or production.
func detectStoreKind(root string) (store.StoreKind, error) {
	path := filepath.Join(root, ".store_identity.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("store identity: %w", err)
	}
	var id struct {
		StoreKind store.StoreKind `json:"storeKind"`
	}
	if err := json.Unmarshal(b, &id); err != nil {
		return "", fmt.Errorf("store identity: parse: %w", err)
	}
	if id.StoreKind == "" {
		return "", fmt.Errorf("store identity: storeKind missing")
	}
	return id.StoreKind, nil
}

// loadSnapshotWithTimeout wraps loadSnapshotFromStore with a context deadline.
func loadSnapshotWithTimeout(gf globalFlags, timeout time.Duration) (WorldSnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return loadSnapshotFromStore(ctx, gf)
}

// archivedProjects returns every project marked Archived=true, sorted by ID.
func (s WorldSnapshot) archivedProjects() []w.ProjectID {
	var ids []w.ProjectID
	for id, p := range s.Desired.Projects {
		if p.Archived {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// parkedProjects returns every non-archived project that is not assigned
// to any profile's slot. Park is a UI-only concept; the DesiredWorld has
// no explicit "park" state.
func (s WorldSnapshot) parkedProjects() []w.ProjectID {
	assigned := map[w.ProjectID]bool{}
	for _, prof := range s.Desired.Profiles {
		for _, pid := range prof.Assignments {
			assigned[pid] = true
		}
	}
	var ids []w.ProjectID
	for id, p := range s.Desired.Projects {
		if p.Archived {
			continue
		}
		if assigned[id] {
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// activeAssignments returns the active profile's slot → project map, in
// slot Order then slot ID for deterministic iteration.
func (s WorldSnapshot) activeAssignments() []slotAssignment {
	prof, ok := s.Desired.Profiles[s.Desired.ActiveProfile]
	if !ok {
		return nil
	}
	slots := s.Environment.Workspaces.Slots
	orderedSlots := append([]w.SlotSpec(nil), slots...)
	sort.Slice(orderedSlots, func(i, j int) bool {
		if orderedSlots[i].Order != orderedSlots[j].Order {
			return orderedSlots[i].Order < orderedSlots[j].Order
		}
		return orderedSlots[i].ID < orderedSlots[j].ID
	})
	out := make([]slotAssignment, 0, len(orderedSlots))
	for _, sl := range orderedSlots {
		out = append(out, slotAssignment{
			Slot:      sl.ID,
			Workspace: sl.Workspace,
			Project:   prof.Assignments[sl.ID], // empty if unassigned
		})
	}
	return out
}

// slotAssignment is a UI helper struct: a slot and the project assigned to it.
// Project may be empty (slot is unassigned in the active profile).
type slotAssignment struct {
	Slot      w.SlotID
	Workspace w.WorkspaceID
	Project   w.ProjectID
}

// resolveManifestDigestPath returns the manifest path likely used by the
// daemon, falling back to gf.manifestPath when set. For diagnostics only.
func resolveManifestDigestPath(gf globalFlags) string {
	if gf.manifestPath != "" {
		return gf.manifestPath
	}
	// Probe likely defaults: launchd plist commonly points here.
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), "Library", "Caches", "projwm-next", "managed-environment.json"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
