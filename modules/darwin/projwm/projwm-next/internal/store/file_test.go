package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestFileStoreInitializesGenerationDirectory(t *testing.T) {
	root := t.TempDir()
	initial := testDesiredWorld()
	fs, err := OpenFileStore(context.Background(), root, StoreKindTest, initial)
	if err != nil {
		t.Fatalf("OpenFileStore: %v", err)
	}
	got, err := fs.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("LoadCurrentGeneration: %v", err)
	}
	if got.ID != "G000001" {
		t.Fatalf("generation ID = %s, want G000001", got.ID)
	}
	if got.Desired.ActiveProfile != initial.ActiveProfile {
		t.Fatalf("active profile = %s, want %s", got.Desired.ActiveProfile, initial.ActiveProfile)
	}
	for _, rel := range []string{
		".store_identity.json",
		"CURRENT",
		filepath.Join("generations", "G000001", "manifest.json"),
		filepath.Join("generations", "G000001", "desired_world.json"),
		filepath.Join("generations", "G000001", "checkpoint.json"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("expected %s: %v", rel, err)
		}
	}
}

func TestFileStoreCommitPublishesImmutableGeneration(t *testing.T) {
	root := t.TempDir()
	fs, err := OpenFileStore(context.Background(), root, StoreKindTest, testDesiredWorld())
	if err != nil {
		t.Fatalf("OpenFileStore: %v", err)
	}
	current, err := fs.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("LoadCurrentGeneration: %v", err)
	}
	nextDesired := current.Desired
	nextDesired.ActiveProfile = "B"
	nextDesired.Profiles["B"] = w.DesiredProfile{ID: "B", Assignments: map[w.SlotID]w.ProjectID{}}
	staged, err := fs.BeginCommit(context.Background(), ControllerCommit{
		Parent:     current.ID,
		Desired:    nextDesired,
		Checkpoint: ControllerCheckpoint{Epoch: 2},
	})
	if err != nil {
		t.Fatalf("BeginCommit: %v", err)
	}
	gen, err := fs.Commit(context.Background(), staged)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if gen != "G000002" {
		t.Fatalf("Commit generation = %s, want G000002", gen)
	}
	loaded, err := fs.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("LoadCurrentGeneration: %v", err)
	}
	if loaded.ID != "G000002" || loaded.Parent == nil || *loaded.Parent != "G000001" {
		t.Fatalf("bad generation relation: id=%s parent=%v", loaded.ID, loaded.Parent)
	}
	if loaded.Desired.ActiveProfile != "B" {
		t.Fatalf("active profile = %s, want B", loaded.Desired.ActiveProfile)
	}
	if _, err := os.Stat(filepath.Join(root, ".staging", string(gen))); err == nil {
		t.Fatalf("staging dir should not remain for %s", gen)
	}

	var manifest struct {
		TransactionID string `json:"transactionId"`
		CommitKind    string `json:"commitKind"`
	}
	if err := readJSON(filepath.Join(root, "generations", string(gen), "manifest.json"), &manifest); err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if manifest.TransactionID == "" {
		t.Fatal("manifest missing transactionId")
	}
	var journal struct {
		TransactionID string           `json:"transactionId"`
		CommitKind    string           `json:"commitKind"`
		Trace         TransactionTrace `json:"trace"`
	}
	journalBytes, err := os.ReadFile(filepath.Join(root, "generations", string(gen), "journal.jsonl"))
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	if err := json.Unmarshal(journalBytes[:len(journalBytes)-1], &journal); err != nil {
		t.Fatalf("parse journal: %v", err)
	}
	if journal.TransactionID != manifest.TransactionID || journal.CommitKind != manifest.CommitKind {
		t.Fatalf("journal=%v manifest=%+v", journal, manifest)
	}
	if journal.Trace.TransactionID == "" || journal.Trace.CommittedGeneration != gen {
		t.Fatalf("journal trace missing commit evidence: %+v", journal.Trace)
	}
}

func TestFileStoreRejectsKindMismatch(t *testing.T) {
	root := t.TempDir()
	if _, err := OpenFileStore(context.Background(), root, StoreKindTest, testDesiredWorld()); err != nil {
		t.Fatalf("OpenFileStore test: %v", err)
	}
	if _, err := OpenFileStore(context.Background(), root, StoreKindProduction, testDesiredWorld()); err == nil {
		t.Fatal("expected kind mismatch error")
	}
}

func TestOpenExistingFileStoreRequiresBootstrappedStore(t *testing.T) {
	root := t.TempDir()
	if _, err := OpenExistingFileStore(context.Background(), root, StoreKindProduction); err == nil {
		t.Fatal("expected empty store to be rejected")
	}
	if _, err := os.Stat(filepath.Join(root, ".store_identity.json")); !os.IsNotExist(err) {
		t.Fatalf("OpenExistingFileStore must not write identity before bootstrap, stat err=%v", err)
	}
	if _, err := OpenFileStore(context.Background(), root, StoreKindProduction, testDesiredWorld()); err != nil {
		t.Fatalf("OpenFileStore bootstrap: %v", err)
	}
	fs, err := OpenExistingFileStore(context.Background(), root, StoreKindProduction)
	if err != nil {
		t.Fatalf("OpenExistingFileStore: %v", err)
	}
	got, err := fs.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("LoadCurrentGeneration: %v", err)
	}
	if got.ID != "G000001" {
		t.Fatalf("generation ID = %s, want G000001", got.ID)
	}
}

func TestFileStoreRecordsFailedTransactionTraceOutsideCurrentGeneration(t *testing.T) {
	root := t.TempDir()
	fs, err := OpenFileStore(context.Background(), root, StoreKindTest, testDesiredWorld())
	if err != nil {
		t.Fatalf("OpenFileStore: %v", err)
	}
	trace := TransactionTrace{
		TransactionID:       "txn-failed",
		Command:             "intent:switch-profile",
		Reason:              "intent",
		VerifierRan:         true,
		VerifierDiffEntries: 1,
		AttemptedOperations: 2,
		ExecutedMutations:   2,
	}
	if err := fs.RecordTransactionTrace(context.Background(), trace); err != nil {
		t.Fatalf("RecordTransactionTrace: %v", err)
	}
	var got TransactionTrace
	if err := readJSON(filepath.Join(root, "traces", "txn-failed.json"), &got); err != nil {
		t.Fatalf("read trace: %v", err)
	}
	if got.TransactionID != trace.TransactionID || got.CommittedGeneration != "" {
		t.Fatalf("bad failed trace: %+v", got)
	}
	current, err := fs.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("LoadCurrentGeneration: %v", err)
	}
	if current.ID != "G000001" {
		t.Fatalf("failed trace must not advance CURRENT, got %s", current.ID)
	}
}

func TestFileStoreBrowserPrivacyArtifactsKeepOnlyOpaqueRefs(t *testing.T) {
	const rawURL = "https://secret.example/private?token=raw-browser-secret"
	storeRoot := t.TempDir()
	privateRoot := filepath.Join(t.TempDir(), "private-payloads")
	privateStore, err := browser.NewFilePrivatePayloadStore(privateRoot)
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	token, err := privateStore.Put(context.Background(), browser.PrivatePayload{URLs: []string{rawURL}})
	if err != nil {
		t.Fatalf("Put private payload: %v", err)
	}
	desired := testBrowserDesiredWorld(w.PrivatePayloadRef(token))
	fs, err := OpenFileStoreWithBootstrapTrace(context.Background(), storeRoot, StoreKindProduction, desired, TransactionTrace{
		Reason:                  "admin-bootstrap",
		TriggerSource:           "admin",
		TriggerKind:             "desired-world-bootstrap",
		BootstrapManifestDigest: "manifest-digest",
	})
	if err != nil {
		t.Fatalf("OpenFileStoreWithBootstrapTrace: %v", err)
	}
	current, err := fs.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("LoadCurrentGeneration: %v", err)
	}
	nextDesired := current.Desired
	nextDesired.AcceptedLayouts = map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout{
		"dotfiles": {
			"Q": {
				Workspace: "Q",
				Source:    w.LayoutAuthorityAcceptedManual,
				Columns: []w.DesiredColumn{{
					Windows: []w.DesiredWindowID{{Project: "dotfiles", Kind: w.WindowBrowser, Index: 1}},
					Mode:    w.ColumnSolo,
				}},
			},
		},
	}
	staged, err := fs.BeginCommit(context.Background(), ControllerCommit{
		Parent:          current.ID,
		Desired:         nextDesired,
		AcceptedLayouts: nextDesired.AcceptedLayouts,
		Checkpoint:      ControllerCheckpoint{Epoch: 1},
		Trace: TransactionTrace{
			TransactionID:       "txn-privacy",
			Command:             "intent:accept-manual-layout",
			Reason:              "intent",
			TriggerSource:       "ipc",
			TriggerKind:         "intent",
			AttemptedOperations: 0,
		},
	})
	if err != nil {
		t.Fatalf("BeginCommit: %v", err)
	}
	if _, err := fs.Commit(context.Background(), staged); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := fs.RecordTransactionTrace(context.Background(), TransactionTrace{
		TransactionID:       "txn-no-commit-privacy",
		Command:             "intent:reconcile",
		Reason:              "intent",
		NoCommitReason:      "privacy-negative-control",
		AttemptedOperations: 0,
	}); err != nil {
		t.Fatalf("RecordTransactionTrace: %v", err)
	}

	scanPersistentStoreArtifacts(t, storeRoot, rawURL, token)
	payload, err := privateStore.Get(context.Background(), token)
	if err != nil {
		t.Fatalf("private payload ref should restore: %v", err)
	}
	if len(payload.URLs) != 1 || payload.URLs[0] != rawURL {
		t.Fatalf("private payload = %+v, want raw URL available only through private store", payload)
	}
}

func scanPersistentStoreArtifacts(t *testing.T, root, rawURL, token string) {
	t.Helper()
	tokenInDesired := false
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		body := string(data)
		if strings.Contains(body, rawURL) || strings.Contains(body, "raw-browser-secret") {
			t.Fatalf("PersistentStore artifact leaked raw browser payload: %s", path)
		}
		if strings.Contains(body, token) {
			if filepath.Base(path) != artifactDesiredWorld {
				t.Fatalf("PersistentStore artifact leaked opaque private payload ref outside DesiredWorld: %s", path)
			}
			tokenInDesired = true
		}
		return nil
	}); err != nil {
		t.Fatalf("walk store: %v", err)
	}
	if !tokenInDesired {
		t.Fatal("DesiredWorld artifact should retain the opaque private payload ref needed for restore")
	}
}

func testDesiredWorld() w.DesiredWorld {
	return w.DesiredWorld{
		ActiveProfile: "A",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"A": {ID: "A", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{},
	}
}

func testBrowserDesiredWorld(ref w.PrivatePayloadRef) w.DesiredWorld {
	browserID := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowBrowser, Index: 1}
	return w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "dotfiles"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"dotfiles": {
				ID:   "dotfiles",
				Root: "/Users/yuta/dev/dotfiles",
				Windows: []w.DesiredWindow{{
					ID:   browserID,
					Kind: w.WindowBrowser,
					App:  w.AppRequirement{Capability: w.CapabilityBrowser, BundleID: "com.vivaldi.Vivaldi", AppPath: "/Applications/Vivaldi.app"},
					TitleContract: w.TitleContract{
						Authority: w.TitleControllerOwned,
						Expected:  "browser-1:dotfiles",
						Drift:     w.TitleDriftRepair,
					},
					Browser: &w.DesiredBrowserSession{
						PrivacyMode:       w.BrowserSnapshotPrivateContent,
						URLPayloadRefs:    []w.PrivatePayloadRef{ref},
						URLCount:          1,
						RestoreURLs:       false,
						RedactionPolicyID: "privacy-test",
					},
				}},
				Layouts: map[w.WorkspaceID]w.DesiredLayout{
					"Q": {
						Workspace: "Q",
						Source:    w.LayoutAuthorityImported,
						Columns: []w.DesiredColumn{{
							Windows: []w.DesiredWindowID{browserID},
							Mode:    w.ColumnSolo,
						}},
					},
				},
			},
		},
	}
}

// TestFileStoreCommitRoundTripsWindowProvenance is the regression owner for the
// 2026-05-31 production-breaking bug: ControllerCheckpoint.WindowProvenance was a
// map[DesiredWindowID]LiveWindowID; DesiredWindowID is a struct, and Go's
// encoding/json CANNOT marshal a struct-keyed map ("json: unsupported type"),
// so every real (FileStore) daemon commit failed at "marshal checkpoint.json"
// and the daemon served degraded IPC. The deterministic tests missed it because
// they used MemoryStore (no JSON). Fix: persist provenance as a SLICE.
func TestFileStoreCommitRoundTripsWindowProvenance(t *testing.T) {
	root := t.TempDir()
	fs, err := OpenFileStore(context.Background(), root, StoreKindTest, testDesiredWorld())
	if err != nil {
		t.Fatalf("OpenFileStore: %v", err)
	}
	current, err := fs.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("LoadCurrentGeneration: %v", err)
	}
	ed := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowEditor, Index: 1}
	sh := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowShell, Index: 2}
	prov := map[w.DesiredWindowID]w.LiveWindowID{ed: "live-zed-1", sh: "live-shell-2"}
	staged, err := fs.BeginCommit(context.Background(), ControllerCommit{
		Parent:     current.ID,
		Desired:    current.Desired,
		Checkpoint: ControllerCheckpoint{Epoch: 2, WindowProvenance: ProvenanceEntriesFromMap(prov)},
	})
	if err != nil {
		t.Fatalf("BeginCommit with WindowProvenance must marshal (struct-keyed map regression): %v", err)
	}
	if _, err := fs.Commit(context.Background(), staged); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	loaded, err := fs.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("LoadCurrentGeneration: %v", err)
	}
	got := ProvenanceMapFromEntries(loaded.Checkpoint.WindowProvenance)
	if got[ed] != "live-zed-1" || got[sh] != "live-shell-2" || len(got) != 2 {
		t.Fatalf("provenance did not round-trip through FileStore: %v", got)
	}
}
