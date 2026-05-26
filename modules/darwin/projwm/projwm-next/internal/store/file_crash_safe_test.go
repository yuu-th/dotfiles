package store

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §8.3 / GAP-23: 全書き込みは flock(2) で排他、tmpfile + atomic
// rename、読み込みは lock 不要。This file groups behavior tests for the
// crash-safe contract (concurrent writer / interrupted write / reader
// during write).

// TestFileStoreConcurrentWritersAreSerialized verifies that two
// goroutines racing BeginCommit + Commit on the same store both
// succeed (one wins the flock first, the other sees the updated
// parent and either retries or fails the parent-mismatch check).
// The store MUST advance monotonically by exactly one generation per
// successful commit and MUST NOT corrupt CURRENT.
func TestFileStoreConcurrentWritersAreSerialized(t *testing.T) {
	root := t.TempDir()
	fs, err := OpenFileStore(context.Background(), root, StoreKindTest, testDesiredWorld())
	if err != nil {
		t.Fatalf("OpenFileStore: %v", err)
	}
	current, err := fs.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("LoadCurrentGeneration: %v", err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	failures := 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			nextDesired := current.Desired
			nextDesired.ActiveProfile = w.ProfileID("worker") // distinct mutation
			staged, beginErr := fs.BeginCommit(context.Background(), ControllerCommit{
				Parent:     current.ID,
				Desired:    nextDesired,
				Checkpoint: ControllerCheckpoint{Epoch: w.Epoch(2 + i)},
			})
			if beginErr != nil {
				mu.Lock()
				failures++
				mu.Unlock()
				return
			}
			if _, err := fs.Commit(context.Background(), staged); err != nil {
				mu.Lock()
				failures++
				mu.Unlock()
				return
			}
			mu.Lock()
			successes++
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	// Exactly one writer must win because both targeted Parent=G000001;
	// the loser's parent-mismatch is the safety net guaranteeing
	// CURRENT cannot point at a stale generation.
	if successes != 1 {
		t.Errorf("expected exactly 1 winning commit, got successes=%d failures=%d", successes, failures)
	}
	loaded, err := fs.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("post-race LoadCurrentGeneration: %v", err)
	}
	if loaded.ID != "G000002" {
		t.Errorf("post-race CURRENT = %s, want G000002 (monotonic advance)", loaded.ID)
	}
}

// TestFileStoreInterruptedWriteLeavesCurrentPointingAtPriorGeneration
// proves SSOT §8.1 atomic rename safety: if a writer crashed AFTER
// staging but BEFORE finalising, CURRENT still references the prior
// committed generation. We simulate this by calling BeginCommit
// (creates .staging/G000002/) but NOT Commit, then re-opening the
// store and asserting it reads G000001.
func TestFileStoreInterruptedWriteLeavesCurrentPointingAtPriorGeneration(t *testing.T) {
	root := t.TempDir()
	fs, err := OpenFileStore(context.Background(), root, StoreKindTest, testDesiredWorld())
	if err != nil {
		t.Fatalf("OpenFileStore: %v", err)
	}
	current, _ := fs.LoadCurrentGeneration(context.Background())

	nextDesired := current.Desired
	nextDesired.ActiveProfile = "B"
	_, err = fs.BeginCommit(context.Background(), ControllerCommit{
		Parent:     current.ID,
		Desired:    nextDesired,
		Checkpoint: ControllerCheckpoint{Epoch: 2},
	})
	if err != nil {
		t.Fatalf("BeginCommit: %v", err)
	}
	// Intentionally skip Commit — simulate process kill mid-write.

	// Re-open the store as a fresh process would.
	reopened, err := OpenExistingFileStore(context.Background(), root, StoreKindTest)
	if err != nil {
		t.Fatalf("OpenExistingFileStore: %v", err)
	}
	got, err := reopened.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("post-crash LoadCurrentGeneration: %v", err)
	}
	if got.ID != "G000001" {
		t.Errorf("post-crash CURRENT = %s, want G000001 (atomic rename safety)", got.ID)
	}
	if got.Desired.ActiveProfile == "B" {
		t.Errorf("post-crash Desired leaked uncommitted state: ActiveProfile=B")
	}
}

// TestFileStoreReaderSeesPriorGenerationDuringStaging proves SSOT §8.3
// "読み込みは lock 不要": a reader opening the store while a writer
// holds the flock and has staged but not committed sees the prior
// generation (atomic rename ensures CURRENT only flips after commit).
func TestFileStoreReaderSeesPriorGenerationDuringStaging(t *testing.T) {
	root := t.TempDir()
	writer, err := OpenFileStore(context.Background(), root, StoreKindTest, testDesiredWorld())
	if err != nil {
		t.Fatalf("OpenFileStore: %v", err)
	}
	current, _ := writer.LoadCurrentGeneration(context.Background())

	nextDesired := current.Desired
	nextDesired.ActiveProfile = "B"
	_, err = writer.BeginCommit(context.Background(), ControllerCommit{
		Parent:     current.ID,
		Desired:    nextDesired,
		Checkpoint: ControllerCheckpoint{Epoch: 2},
	})
	if err != nil {
		t.Fatalf("BeginCommit: %v", err)
	}

	// A separate reader handle. Even though the writer's stage dir
	// exists, CURRENT still points at G000001.
	reader, err := OpenExistingFileStore(context.Background(), root, StoreKindTest)
	if err != nil {
		t.Fatalf("reader OpenExistingFileStore: %v", err)
	}
	got, err := reader.LoadCurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("reader LoadCurrentGeneration: %v", err)
	}
	if got.ID != "G000001" {
		t.Errorf("reader saw uncommitted generation: %s (want G000001)", got.ID)
	}
}

// TestFileStoreStagingDirCleanedByAbort verifies that calling Abort
// after BeginCommit removes the staging directory so subsequent
// re-openings of the store do not see leftover artifacts.
func TestFileStoreStagingDirCleanedByAbort(t *testing.T) {
	root := t.TempDir()
	fs, err := OpenFileStore(context.Background(), root, StoreKindTest, testDesiredWorld())
	if err != nil {
		t.Fatalf("OpenFileStore: %v", err)
	}
	current, _ := fs.LoadCurrentGeneration(context.Background())

	nextDesired := current.Desired
	nextDesired.ActiveProfile = "B"
	staged, err := fs.BeginCommit(context.Background(), ControllerCommit{
		Parent:     current.ID,
		Desired:    nextDesired,
		Checkpoint: ControllerCheckpoint{Epoch: 2},
	})
	if err != nil {
		t.Fatalf("BeginCommit: %v", err)
	}
	if err := fs.Abort(context.Background(), staged); err != nil {
		t.Fatalf("Abort: %v", err)
	}
	stagingDir := filepath.Join(root, ".staging")
	if entries, err := os.ReadDir(stagingDir); err == nil {
		for _, e := range entries {
			if e.Name() == "G000002" {
				t.Errorf("Abort did not clean staging dir for G000002")
			}
		}
	}
}
