package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuu-th/projwm-next/internal/migration"
)

func TestBootstrapTraceClassifiesAdminInputs(t *testing.T) {
	desiredTrace := bootstrapTrace("/tmp/desired.json", "", "digest-1")
	if desiredTrace.TriggerSource != "admin" || desiredTrace.TriggerKind != "desired-world-bootstrap" || desiredTrace.Reason != "admin-bootstrap" || desiredTrace.BootstrapManifestDigest != "digest-1" {
		t.Fatalf("desired bootstrap trace mismatch: %+v", desiredTrace)
	}
	legacyTrace := bootstrapTrace("", "/tmp/state.json", "digest-2")
	if legacyTrace.TriggerSource != "admin" || legacyTrace.TriggerKind != "legacy-state-migration" || legacyTrace.Reason != "admin-bootstrap" || legacyTrace.BootstrapManifestDigest != "digest-2" {
		t.Fatalf("legacy bootstrap trace mismatch: %+v", legacyTrace)
	}
}

func TestLoadLegacyDesiredWorldQuarantinesInputAndRedactedReport(t *testing.T) {
	storeDir := t.TempDir()
	legacyPath := filepath.Join(t.TempDir(), "state.json")
	const legacy = `{
  "active_profile": "work",
  "profiles": {"work": {"assignments": {"Q": "dotfiles"}}},
  "projects": {
    "dotfiles": {
      "cwd": "/Users/yuta/dev/dotfiles",
      "windows": [
        {"id": 1, "kind": "browser", "saved_urls": ["https://secret.example"]}
      ]
    }
  }
}`
	if err := os.WriteFile(legacyPath, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	var report *migration.LegacyReport
	var privateReport *migration.LegacyPrivatePayloadReport
	desired, err := loadLegacyDesiredWorld(context.Background(), storeDir, legacyPath, &report, &privateReport)
	if err != nil {
		t.Fatalf("loadLegacyDesiredWorld: %v", err)
	}
	if desired.ActiveProfile != "work" {
		t.Fatalf("ActiveProfile = %q", desired.ActiveProfile)
	}
	entries, err := os.ReadDir(filepath.Join(storeDir, "quarantine"))
	if err != nil {
		t.Fatalf("read quarantine: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("quarantine entries = %d, want 1", len(entries))
	}
	reason, err := os.ReadFile(filepath.Join(storeDir, "quarantine", entries[0].Name(), "reason.json"))
	if err != nil {
		t.Fatalf("read reason: %v", err)
	}
	if strings.Contains(string(reason), "secret.example") {
		t.Fatalf("migration report leaked URL: %s", string(reason))
	}
	if strings.Contains(string(reason), "browser-payload-v1-") {
		t.Fatalf("migration report leaked private payload ref: %s", string(reason))
	}
	if _, err := os.Stat(filepath.Join(storeDir, "quarantine", entries[0].Name(), "state.json")); !os.IsNotExist(err) {
		t.Fatalf("raw legacy state must not be written inside PersistentStore quarantine, stat err=%v", err)
	}
	storeFilesLeaked := false
	if err := filepath.WalkDir(storeDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(b), "secret.example") {
			storeFilesLeaked = true
		}
		return nil
	}); err != nil {
		t.Fatalf("walk store: %v", err)
	}
	if storeFilesLeaked {
		t.Fatal("PersistentStore contained raw legacy URL")
	}
	privateInputPath := filepath.Join(filepath.Dir(storeDir), "private-payloads", "legacy-input-quarantine", entries[0].Name(), "state.json")
	privateInput, err := os.ReadFile(privateInputPath)
	if err != nil {
		t.Fatalf("read private legacy input: %v", err)
	}
	if !strings.Contains(string(privateInput), "secret.example") {
		t.Fatalf("private legacy quarantine missing raw input: %s", privateInput)
	}
	if privateReport == nil || privateReport.Discovered != 1 || privateReport.MigratedToPrivatePayload != 1 || privateReport.CommittedRawURLs != 0 {
		t.Fatalf("private report = %+v", privateReport)
	}
}
