package cockpitsnap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func minimalManifestJSON() []byte {
	return []byte(`{
  "schemaVersion": 1,
  "authority": "nix",
  "source": "test:cockpit",
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
}`)
}

func newFixture(t *testing.T) (storeDir, manifestPath string) {
	t.Helper()
	tmp := t.TempDir()
	storeDir = filepath.Join(tmp, "store")
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "dotfiles"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"dotfiles": {ID: "dotfiles"},
		},
	}
	if _, err := store.OpenFileStore(context.Background(), storeDir, store.StoreKindTest, desired); err != nil {
		t.Fatal(err)
	}
	manifestPath = filepath.Join(tmp, "manifest.json")
	if err := os.WriteFile(manifestPath, minimalManifestJSON(), 0o644); err != nil {
		t.Fatal(err)
	}
	return storeDir, manifestPath
}

func TestLoadFromStore_HappyPath(t *testing.T) {
	storeDir, manifestPath := newFixture(t)
	snap, err := LoadFromStore(context.Background(), storeDir, manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if snap.ActiveProfile != "work" {
		t.Errorf("ActiveProfile = %q", snap.ActiveProfile)
	}
	if snap.Source != "store" {
		t.Errorf("Source = %q", snap.Source)
	}
	// parked / archived computations.
	if len(snap.Parked) != 0 {
		t.Errorf("expected 0 parked, got %v", snap.Parked)
	}
	if len(snap.Archived) != 0 {
		t.Errorf("expected 0 archived, got %v", snap.Archived)
	}
}

func TestSnapshot_JSONShape(t *testing.T) {
	storeDir, manifestPath := newFixture(t)
	snap, err := LoadFromStore(context.Background(), storeDir, manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	var probe map[string]any
	if err := json.Unmarshal(b, &probe); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"activeProfile", "slots", "profiles"} {
		if _, ok := probe[k]; !ok {
			t.Errorf("missing JSON key %q", k)
		}
	}
}

func TestTmuxSessionForWindow(t *testing.T) {
	cases := []struct {
		kind w.WindowKind
		idx  int
		pid  w.ProjectID
		want string
	}{
		{w.WindowAI, 1, "foo", "ai-1/foo"},
		{w.WindowShell, 3, "bar", "shell-3/bar"},
		{w.WindowViewer, 2, "baz", "ai-2/baz_v"},
		{w.WindowEditor, 1, "qux", ""},
	}
	for _, c := range cases {
		dw := w.DesiredWindow{
			Kind: c.kind,
			ID:   w.DesiredWindowID{Project: c.pid, Kind: c.kind, Index: c.idx},
		}
		got := TmuxSessionForWindow(c.pid, dw)
		if got != c.want {
			t.Errorf("TmuxSessionForWindow(%s, %s-%d, %s) = %q, want %q",
				c.pid, c.kind, c.idx, c.pid, got, c.want)
		}
	}
}

func TestParkedAndArchived(t *testing.T) {
	d := w.DesiredWorld{
		ActiveProfile: "alpha",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"alpha": {ID: "alpha", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1"},
			"p2": {ID: "p2"},                 // parked (no assignment, not archived)
			"p3": {ID: "p3", Archived: true}, // archived
		},
	}
	parkedIDs := parked(d)
	archivedIDs := archived(d)
	if len(parkedIDs) != 1 || parkedIDs[0] != "p2" {
		t.Errorf("parked = %v, want [p2]", parkedIDs)
	}
	if len(archivedIDs) != 1 || archivedIDs[0] != "p3" {
		t.Errorf("archived = %v, want [p3]", archivedIDs)
	}
}
