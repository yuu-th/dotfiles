package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSSOTTestManifestParses(t *testing.T) {
	env, err := LoadFromFile(filepath.Join("..", "..", "testdata", "manifest.json"))
	if err != nil {
		t.Fatalf("LoadFromFile(testdata/manifest.json): %v", err)
	}
	if env.Authority != "nix" || env.WindowManager.Backend != "omniwm" {
		t.Fatalf("unexpected environment authority/backend: %+v", env)
	}
	if len(env.Workspaces.Workspaces) != 2 {
		t.Fatalf("test manifest should declare exactly workspaces 8 and 9, got %+v", env.Workspaces.Workspaces)
	}
	if len(env.Workspaces.Slots) != 0 {
		t.Fatalf("L3 test manifest must not declare production slots, got %+v", env.Workspaces.Slots)
	}
}

func TestSSOTTestManifestMatchesSectionTenPointSevenShape(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "manifest.json"))
	if err != nil {
		t.Fatalf("read test manifest: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode test manifest: %v", err)
	}
	if _, ok := doc["workspaces"].([]any); !ok {
		t.Fatalf("SSOT §10.7 defines top-level workspaces as an array; got %T", doc["workspaces"])
	}
	if _, ok := doc["slots"].([]any); !ok {
		t.Fatalf("SSOT §10.7 defines top-level slots as an array; got %T", doc["slots"])
	}
	if _, ok := doc["apps"].([]any); !ok {
		t.Fatalf("SSOT §10.7 defines top-level apps as an array; got %T", doc["apps"])
	}
}
