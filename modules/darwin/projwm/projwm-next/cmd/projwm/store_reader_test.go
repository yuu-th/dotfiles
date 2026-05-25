package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// minimalManifestJSON returns an SSOT-shaped manifest.
func minimalManifestJSON(t *testing.T) []byte {
	t.Helper()
	const doc = `{
  "schemaVersion": 1,
  "authority": "nix",
  "source": "test:unit",
  "minProjwmdVersion": "0.1.0",
  "windowManager": {
    "backend": "omniwm",
    "layout": {
      "defaultColumnWidth": 0.5,
      "columnWidthPresets": [0.4, 0.5],
      "maxVisibleColumns": 4,
      "maxWindowsPerColumn": 4,
      "centerFocusedColumn": "never",
      "alwaysCenterSingle": true
    },
    "focus": {
      "followsMouse": false,
      "followsWindowToMonitor": true,
      "moveMouseToFocusedWindow": true
    }
  },
  "workspaces": [
    {"id": "A", "rawName": "12", "displayName": "A", "role": "viewer"},
    {"id": "Q", "rawName": "13", "displayName": "Q", "role": "project"}
  ],
  "slots": [
    {"id": "Q", "workspace": "Q", "order": 1}
  ],
  "apps": [],
  "daemons": {
    "controller": "org.nixos.projwmd-next",
    "socketPath": "/tmp/sock",
    "legacyAgents": "remove",
    "eventSources": [],
    "agents": []
  }
}`
	return []byte(doc)
}

func newTestStore(t *testing.T, desired w.DesiredWorld) (storeDir, manifestPath string) {
	t.Helper()
	tmp := t.TempDir()
	storeDir = filepath.Join(tmp, "store")
	if _, err := store.OpenFileStore(context.Background(), storeDir, store.StoreKindTest, desired); err != nil {
		t.Fatal(err)
	}
	manifestPath = filepath.Join(tmp, "manifest.json")
	if err := os.WriteFile(manifestPath, minimalManifestJSON(t), 0o644); err != nil {
		t.Fatal(err)
	}
	return storeDir, manifestPath
}

func TestLoadSnapshot_FailsWithoutStoreDir(t *testing.T) {
	_, err := loadSnapshotFromStore(context.Background(), globalFlags{})
	if err == nil {
		t.Fatal("expected error without store-dir")
	}
}

func TestLoadSnapshot_FailsWithoutManifest(t *testing.T) {
	storeDir, _ := newTestStore(t, w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{},
	})
	_, err := loadSnapshotFromStore(context.Background(), globalFlags{storeDir: storeDir})
	if err == nil {
		t.Fatal("expected error without manifest path")
	}
}

func TestLoadSnapshot_HappyPath(t *testing.T) {
	storeDir, manifestPath := newTestStore(t, w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{},
	})
	gf := globalFlags{storeDir: storeDir, manifestPath: manifestPath}
	snap, err := loadSnapshotFromStore(context.Background(), gf)
	if err != nil {
		t.Fatalf("loadSnapshot: %v", err)
	}
	if snap.Generation == "" {
		t.Errorf("expected non-empty generation")
	}
	if snap.Desired.ActiveProfile != "empty" {
		t.Errorf("ActiveProfile = %q", snap.Desired.ActiveProfile)
	}
	if len(snap.Environment.Workspaces.Slots) != 1 {
		t.Errorf("expected 1 slot, got %d", len(snap.Environment.Workspaces.Slots))
	}
}

func TestArchivedAndParkedHelpers(t *testing.T) {
	snap := WorldSnapshot{
		Desired: w.DesiredWorld{
			ActiveProfile: "work",
			Profiles: map[w.ProfileID]w.DesiredProfile{
				"work": {
					ID: "work",
					Assignments: map[w.SlotID]w.ProjectID{
						"Q": "dotfiles",
					},
				},
			},
			Projects: map[w.ProjectID]w.DesiredProject{
				"dotfiles":  {ID: "dotfiles", Archived: false},
				"old-thing": {ID: "old-thing", Archived: true},
				"spike-x":   {ID: "spike-x", Archived: false},
			},
		},
	}
	archived := snap.archivedProjects()
	if len(archived) != 1 || archived[0] != "old-thing" {
		t.Errorf("archived: %v", archived)
	}
	parked := snap.parkedProjects()
	if len(parked) != 1 || parked[0] != "spike-x" {
		t.Errorf("parked: %v (expected [spike-x])", parked)
	}
}
