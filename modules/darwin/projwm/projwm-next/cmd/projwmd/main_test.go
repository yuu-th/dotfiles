package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestLoadEnvironmentVerifiesManifestDigest(t *testing.T) {
	path := writeTestManifest(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])

	if _, gotDigest, err := loadEnvironment(path, digest); err != nil {
		t.Fatalf("loadEnvironment: %v", err)
	} else if gotDigest != digest {
		t.Fatalf("digest = %s, want %s", gotDigest, digest)
	}

	if _, _, err := loadEnvironment(path, "not-"+digest); err == nil || !strings.Contains(err.Error(), "manifest digest mismatch") {
		t.Fatalf("expected digest mismatch, got %v", err)
	}
	if _, _, err := loadEnvironment(path, ""); err == nil || !strings.Contains(err.Error(), "--manifest-digest is required") {
		t.Fatalf("expected required digest error, got %v", err)
	}
}

func TestSelectAdapterRejectsBackendEnvOverride(t *testing.T) {
	path := writeTestManifest(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	sum := sha256.Sum256(data)
	env, _, err := loadEnvironment(path, hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatalf("loadEnvironment: %v", err)
	}

	t.Setenv("PROJWM_NEXT_BACKEND", "real")
	if _, _, _, _, err := selectAdapter(env, nil); err == nil || !strings.Contains(err.Error(), "PROJWM_NEXT_BACKEND is not allowed") {
		t.Fatalf("expected backend override rejection, got %v", err)
	}
}

func TestSelectAdapterWiresVivaldiPrivatePayloadStore(t *testing.T) {
	path := writeTestManifest(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	sum := sha256.Sum256(data)
	env, _, err := loadEnvironment(path, hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatalf("loadEnvironment: %v", err)
	}
	privateStore, err := browser.NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	adapter, vivaldi, zedAdapter, backend, err := selectAdapter(env, privateStore)
	if err != nil {
		t.Fatalf("selectAdapter: %v", err)
	}
	sig, ok := adapter.(*wm.SigWM)
	if !ok || backend != "real" || sig.Browser == nil {
		t.Fatalf("adapter = %T backend=%q browser=%v", adapter, backend, ok && sig.Browser != nil)
	}
	if vivaldi == nil {
		t.Fatalf("expected vivaldi adapter to be returned alongside the WindowManagerAdapter")
	}
	if sig.Browser != vivaldi {
		t.Fatalf("expected SigWM.Browser to be the same VivaldiAdapter instance returned by selectAdapter")
	}
	if vivaldi.WindowQuerier == nil {
		t.Fatalf("vivaldi adapter must be wired with a WindowQuerier so OpenInProfile can populate BrowserWindowID via OmniWM diff")
	}
	if zedAdapter == nil {
		t.Fatalf("expected zed adapter to be returned alongside the WindowManagerAdapter")
	}
	if zedAdapter.WindowQuerier == nil {
		t.Fatalf("zed adapter must be wired with a WindowQuerier so project-scoped-app removal can correlate via OmniWM observation")
	}
}

func TestDefaultPrivatePayloadDirIsStoreSibling(t *testing.T) {
	got := defaultPrivatePayloadDir("/var/lib/projwm-next/store")
	if got != "/var/lib/projwm-next/private-payloads" {
		t.Fatalf("defaultPrivatePayloadDir = %q", got)
	}
}

func TestStartupProvenanceRecordsProductionInputs(t *testing.T) {
	path := writeTestManifest(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	env, _, err := loadEnvironment(path, digest)
	if err != nil {
		t.Fatalf("loadEnvironment: %v", err)
	}
	storeDir := filepath.Join(t.TempDir(), "store")
	privatePayloadDir := filepath.Join(filepath.Dir(storeDir), "private-payloads")
	launchdProof := []launchdServiceProof{{Role: "controller", Label: "org.nixos.projwmd-next", Loaded: true, Running: true, PID: os.Getpid(), PIDMatches: true}}
	bootstrap := testBootstrapGeneration(digest)
	provenance, err := buildStartupProvenance(env, path, digest, storeDir, privatePayloadDir, store.StoreKindProduction, bootstrap, bootstrap, env.Daemons.SocketPath, "real", "org.nixos.projwmd-next", "verified", launchdProof)
	if err != nil {
		t.Fatalf("buildStartupProvenance: %v", err)
	}
	if provenance.Mode != "production" || provenance.ManifestDigest != digest || provenance.StoreKind != store.StoreKindProduction || provenance.CurrentGeneration != w.GenerationID("G000001") || provenance.ManagedByManifest || provenance.DesiredWorldInjected {
		t.Fatalf("bad startup provenance: %+v", provenance)
	}
	if !filepath.IsAbs(provenance.ManifestPath) || !filepath.IsAbs(provenance.StoreDir) || !filepath.IsAbs(provenance.PrivatePayloadDir) {
		t.Fatalf("provenance paths must be absolute: %+v", provenance)
	}
	outPath := filepath.Join(t.TempDir(), "startup-provenance.json")
	if err := writeStartupProvenance(outPath, provenance); err != nil {
		t.Fatalf("writeStartupProvenance: %v", err)
	}
	var decoded startupProvenance
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode provenance: %v", err)
	}
	for _, secret := range []string{"https://secret.example", "browser-payload-v1-", "raw-browser-secret"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("startup provenance leaked browser secret material %q: %s", secret, string(raw))
		}
	}
	if decoded.ManifestDigest != digest || decoded.StoreKind != store.StoreKindProduction || decoded.PrivatePayloadDir != provenance.PrivatePayloadDir || decoded.LaunchdLabel != "org.nixos.projwmd-next" || !decoded.RequiredEventSourcesDeclared || decoded.RuntimeLaunchdEventSourceProof != "verified" || !decoded.ProductionAdminBootstrap || len(decoded.LaunchdRuntimeProof) != 1 || !decoded.LaunchdRuntimeProof[0].PIDMatches {
		t.Fatalf("decoded provenance mismatch: %+v", decoded)
	}
}

func TestStartupProvenanceRequiresNixStoreManifestPathForManagedProof(t *testing.T) {
	path := writeTestManifest(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	env, _, err := loadEnvironment(path, digest)
	if err != nil {
		t.Fatalf("loadEnvironment: %v", err)
	}
	storeDir := filepath.Join(t.TempDir(), "store")
	bootstrap := testBootstrapGeneration(digest)
	provenance, err := buildStartupProvenance(env, "/nix/store/00000000000000000000000000000000-projwm-next-managed-environment.json", digest, storeDir, filepath.Join(filepath.Dir(storeDir), "private-payloads"), store.StoreKindProduction, bootstrap, bootstrap, env.Daemons.SocketPath, "real", "org.nixos.projwmd-next", "not-observed", nil)
	if err != nil {
		t.Fatalf("buildStartupProvenance: %v", err)
	}
	if !provenance.ManagedByManifest {
		t.Fatalf("expected Nix store manifest path to be reported as managed proof: %+v", provenance)
	}
}

func TestStartupProvenanceRequiresAdminBootstrapEvidence(t *testing.T) {
	path := writeTestManifest(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	env, _, err := loadEnvironment(path, digest)
	if err != nil {
		t.Fatalf("loadEnvironment: %v", err)
	}
	badBootstrap := store.CommittedGeneration{ID: "G000001", Trace: store.TransactionTrace{CommitKind: "migration-bootstrap", CommittedBy: "controller"}}
	storeDir := filepath.Join(t.TempDir(), "store")
	if _, err := buildStartupProvenance(env, path, digest, storeDir, filepath.Join(filepath.Dir(storeDir), "private-payloads"), store.StoreKindProduction, badBootstrap, badBootstrap, env.Daemons.SocketPath, "real", "org.nixos.projwmd-next", "not-observed", nil); err == nil || !strings.Contains(err.Error(), "production store bootstrap evidence") {
		t.Fatalf("expected missing bootstrap evidence rejection, got %v", err)
	}
}

func TestStartupProvenanceAcceptsBootstrappedAncestryWithControllerCommitCurrent(t *testing.T) {
	path := writeTestManifest(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	env, _, err := loadEnvironment(path, digest)
	if err != nil {
		t.Fatalf("loadEnvironment: %v", err)
	}
	bootstrap := testBootstrapGeneration(digest)
	parent := bootstrap.ID
	current := store.CommittedGeneration{
		ID:     "G000002",
		Parent: &parent,
		Trace: store.TransactionTrace{
			CommitKind:              "controller-commit",
			CommittedBy:             "controller",
			ParentGeneration:        parent,
			CommittedGeneration:     "G000002",
			BootstrapManifestDigest: "",
		},
	}
	storeDir := filepath.Join(t.TempDir(), "store")
	provenance, err := buildStartupProvenance(env, path, digest, storeDir, filepath.Join(filepath.Dir(storeDir), "private-payloads"), store.StoreKindProduction, current, bootstrap, env.Daemons.SocketPath, "real", "org.nixos.projwmd-next", "not-observed", nil)
	if err != nil {
		t.Fatalf("buildStartupProvenance: %v", err)
	}
	if provenance.CurrentGeneration != current.ID || provenance.BootstrapGeneration != bootstrap.ID || !provenance.ProductionAdminBootstrap {
		t.Fatalf("provenance should separate current and bootstrap generations: %+v", provenance)
	}
}

func TestStartupProvenanceRejectsLabelMismatchAndTmpPath(t *testing.T) {
	path := writeTestManifest(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	sum := sha256.Sum256(data)
	env, _, err := loadEnvironment(path, hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatalf("loadEnvironment: %v", err)
	}
	digest := hex.EncodeToString(sum[:])
	bootstrap := testBootstrapGeneration(digest)
	storeDir := filepath.Join(t.TempDir(), "store")
	privatePayloadDir := filepath.Join(filepath.Dir(storeDir), "private-payloads")
	if _, err := buildStartupProvenance(env, path, digest, storeDir, privatePayloadDir, store.StoreKindProduction, bootstrap, bootstrap, "/var/run/projwmd.sock", "real", "wrong.label", "not-observed", nil); err == nil || !strings.Contains(err.Error(), "does not match manifest controller") {
		t.Fatalf("expected launchd label mismatch, got %v", err)
	}
	if _, err := buildStartupProvenance(env, path, digest, storeDir, privatePayloadDir, store.StoreKindProduction, bootstrap, bootstrap, "/var/run/other.sock", "real", "org.nixos.projwmd-next", "not-observed", nil); err == nil || !strings.Contains(err.Error(), "does not match manifest socketPath") {
		t.Fatalf("expected socket path mismatch, got %v", err)
	}
	if err := writeStartupProvenance("/tmp/projwmd-startup-provenance.json", startupProvenance{}); err == nil || !strings.Contains(err.Error(), "refusing /tmp startup provenance path") {
		t.Fatalf("expected tmp provenance rejection, got %v", err)
	}
}

func TestStartupProvenanceRequiresPrivatePayloadOutsideStore(t *testing.T) {
	path := writeTestManifest(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	sum := sha256.Sum256(data)
	env, _, err := loadEnvironment(path, hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatalf("loadEnvironment: %v", err)
	}
	storeDir := filepath.Join(t.TempDir(), "store")
	digest := hex.EncodeToString(sum[:])
	bootstrap := testBootstrapGeneration(digest)
	if _, err := buildStartupProvenance(env, path, digest, storeDir, "", store.StoreKindProduction, bootstrap, bootstrap, env.Daemons.SocketPath, "real", "org.nixos.projwmd-next", "not-observed", nil); err == nil || !strings.Contains(err.Error(), "--private-payload-dir is required") {
		t.Fatalf("expected private payload dir required, got %v", err)
	}
	if _, err := buildStartupProvenance(env, path, digest, storeDir, filepath.Join(storeDir, "private-payloads"), store.StoreKindProduction, bootstrap, bootstrap, env.Daemons.SocketPath, "real", "org.nixos.projwmd-next", "not-observed", nil); err == nil || !strings.Contains(err.Error(), "outside PersistentStore") {
		t.Fatalf("expected private payload outside store rejection, got %v", err)
	}
}

func testBootstrapGeneration(manifestDigest string) store.CommittedGeneration {
	return store.CommittedGeneration{
		ID: "G000001",
		Trace: store.TransactionTrace{
			CommitKind:              "migration-bootstrap",
			CommittedBy:             "controller",
			TriggerSource:           "admin",
			TriggerKind:             "desired-world-bootstrap",
			BootstrapManifestDigest: manifestDigest,
		},
	}
}

func TestLaunchdPIDParsesLaunchctlOutput(t *testing.T) {
	if got := launchdPID("state = running\npid = 12345\n"); got != 12345 {
		t.Fatalf("launchdPID = %d, want 12345", got)
	}
	if got := launchdPID("state = waiting\n"); got != 0 {
		t.Fatalf("launchdPID waiting = %d, want 0", got)
	}
}

func TestVerifyLaunchdRuntimeProofAuditsControllerAndSidecars(t *testing.T) {
	path := writeTestManifest(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	sum := sha256.Sum256(data)
	env, _, err := loadEnvironment(path, hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatalf("loadEnvironment: %v", err)
	}
	old := inspectLaunchdServiceFunc
	t.Cleanup(func() { inspectLaunchdServiceFunc = old })
	seen := map[string]bool{}
	inspectLaunchdServiceFunc = func(ctx context.Context, role, label, kind, source string) (launchdServiceProof, error) {
		seen[label] = true
		pid := 9000
		running := true
		if role == "controller" {
			pid = os.Getpid()
		}
		if kind == "safety-timer" {
			running = false
		}
		return launchdServiceProof{
			Role:    role,
			Label:   label,
			Kind:    kind,
			Source:  source,
			Loaded:  true,
			Running: running,
			PID:     pid,
		}, nil
	}

	status, proofs, err := verifyLaunchdRuntimeProof(context.Background(), env, env.Daemons.ControllerLabel, true)
	if err != nil {
		t.Fatalf("verifyLaunchdRuntimeProof: %v", err)
	}
	if status != "verified" || len(proofs) != 1+len(env.Daemons.EventSources) || !proofs[0].PIDMatches {
		t.Fatalf("bad proof status=%q proofs=%+v", status, proofs)
	}
	for _, src := range env.Daemons.EventSources {
		if !seen[src.Label] {
			t.Fatalf("sidecar label %s was not inspected; seen=%v", src.Label, seen)
		}
	}
}

func TestVerifyLaunchdRuntimeProofRejectsControllerPIDMismatch(t *testing.T) {
	path := writeTestManifest(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	sum := sha256.Sum256(data)
	env, _, err := loadEnvironment(path, hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatalf("loadEnvironment: %v", err)
	}
	old := inspectLaunchdServiceFunc
	t.Cleanup(func() { inspectLaunchdServiceFunc = old })
	inspectLaunchdServiceFunc = func(ctx context.Context, role, label, kind, source string) (launchdServiceProof, error) {
		return launchdServiceProof{Role: role, Label: label, Kind: kind, Source: source, Loaded: true, Running: true, PID: os.Getpid() + 1}, nil
	}

	_, _, err = verifyLaunchdRuntimeProof(context.Background(), env, env.Daemons.ControllerLabel, true)
	if err == nil || !strings.Contains(err.Error(), "controller proof failed") {
		t.Fatalf("expected controller PID mismatch rejection, got %v", err)
	}
}

func TestPrepareUnixSocketPathRemovesOnlyStaleSockets(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "pwm-")
	if err != nil {
		t.Fatalf("create short temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "projwmd.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	if err := prepareUnixSocketPath(socket); err == nil || !strings.Contains(err.Error(), "active unix socket") {
		_ = l.Close()
		t.Fatalf("expected active socket rejection, got %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	if err := prepareUnixSocketPath(socket); err != nil {
		t.Fatalf("prepare stale socket: %v", err)
	}
	if _, err := os.Lstat(socket); !os.IsNotExist(err) {
		t.Fatalf("expected stale socket removed, stat err=%v", err)
	}
	regular := filepath.Join(dir, "regular")
	if err := os.WriteFile(regular, []byte("not a socket"), 0o644); err != nil {
		t.Fatalf("write regular file: %v", err)
	}
	if err := prepareUnixSocketPath(regular); err == nil || !strings.Contains(err.Error(), "refusing to replace non-socket") {
		t.Fatalf("expected regular file rejection, got %v", err)
	}
}

func writeTestManifest(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "managed-environment.json")
	const doc = `{
  "schemaVersion": 1,
  "authority": "nix",
  "source": "test",
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
  "apps": [
    {"capability": "terminal", "bundleId": "com.mitchellh.ghostty", "appPath": "/Applications/Ghostty.app"}
  ],
  "daemons": {
    "controller": "org.nixos.projwmd-next",
    "socketPath": "/var/run/projwmd.sock",
    "legacyAgents": "remove",
    "eventSources": [
      {"kind": "windows-changed", "source": "window-manager", "mode": "sidecar", "authority": "hint", "label": "org.nixos.projwm-next-windows-changed"},
      {"kind": "display-changed", "source": "system", "mode": "sidecar", "authority": "hint", "label": "org.nixos.projwm-next-display-changed"},
      {"kind": "layout-changed", "source": "user", "mode": "sidecar", "authority": "hint", "label": "org.nixos.projwm-next-layout-changed"},
      {"kind": "safety-timer", "source": "timer", "mode": "sidecar", "authority": "hint", "label": "org.nixos.projwm-next-safety-timer"},
      {"kind": "wake", "source": "system", "mode": "sidecar", "authority": "hint", "label": "org.nixos.projwm-next-wake"}
    ],
    "agents": []
  }
}
`
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}
