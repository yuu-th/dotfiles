package clientauth

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyManagedSocketAcceptsManifestAuthorizedSocket(t *testing.T) {
	manifestPath, digest := writeTestManifest(t, "/Users/yuta/Library/Application Support/projwm-next/projwmd.sock")
	err := VerifyManagedSocket("test-client", "/Users/yuta/Library/Application Support/projwm-next/projwmd.sock", manifestPath, digest)
	if err != nil {
		t.Fatalf("VerifyManagedSocket: %v", err)
	}
}

func TestVerifyManagedSocketRejectsArbitrarySocket(t *testing.T) {
	manifestPath, digest := writeTestManifest(t, "/Users/yuta/Library/Application Support/projwm-next/projwmd.sock")
	err := VerifyManagedSocket("test-client", "/Users/yuta/Library/Application Support/projwm-next/other.sock", manifestPath, digest)
	if err == nil || !strings.Contains(err.Error(), "not authorized") {
		t.Fatalf("expected unauthorized socket rejection, got %v", err)
	}
}

func TestVerifyManagedSocketRejectsDigestMismatch(t *testing.T) {
	manifestPath, _ := writeTestManifest(t, "/Users/yuta/Library/Application Support/projwm-next/projwmd.sock")
	err := VerifyManagedSocket("test-client", "/Users/yuta/Library/Application Support/projwm-next/projwmd.sock", manifestPath, "bad-digest")
	if err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("expected digest mismatch, got %v", err)
	}
}

func writeTestManifest(t *testing.T, socketPath string) (string, string) {
	t.Helper()
	data := []byte(`{
  "schemaVersion": 1,
  "authority": "nix",
  "source": "clientauth-test",
  "minProjwmdVersion": "0.1.0",
  "windowManager": {"backend": "real", "layout": {"maxVisibleColumns": 3, "maxWindowsPerColumn": 4}},
  "workspaces": [{"id": "A", "rawName": "A", "displayName": "A", "role": "viewer"}],
  "slots": [],
  "apps": [],
  "daemons": {"controller": "test", "socketPath": "` + socketPath + `", "legacyAgents": "report", "agents": []}
}`)
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return path, hex.EncodeToString(sum[:])
}
