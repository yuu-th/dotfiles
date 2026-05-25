// Package store provides PersistentStore. impl-design.md §4. design.md §8.1.
// MemoryStore is for tests; FileStore is the production generation store.
package store

import (
	"context"
	"fmt"
	"sync"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// CommittedGeneration is an immutable snapshot of committed durable records. design.md §8.1.
type CommittedGeneration struct {
	ID              w.GenerationID
	Parent          *w.GenerationID
	Desired         w.DesiredWorld
	AcceptedLayouts map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout
	Checkpoint      ControllerCheckpoint
	Trace           TransactionTrace
	StoreVersion    w.StoreVersion
}

type ControllerCheckpoint struct {
	Epoch       w.Epoch
	LastClean   *w.TransactionID
	DirtyScopes []w.DirtyScope
}

// ControllerCommit is the controller's request to begin a commit.
type ControllerCommit struct {
	TransactionID   w.TransactionID
	Parent          w.GenerationID
	Desired         w.DesiredWorld
	AcceptedLayouts map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout
	Checkpoint      ControllerCheckpoint
	Trace           TransactionTrace
}

// TransactionTrace is the durable, redacted transaction audit evidence required
// by specs.md §9.2 and design.md §12. It records semantic operation metadata
// only; live IDs may appear as operation targets elsewhere, but browser URL/title
// content must never be written here.
type TransactionTrace struct {
	TransactionID               w.TransactionID           `json:"transactionId"`
	Command                     string                    `json:"command,omitempty"`
	Reason                      string                    `json:"reason,omitempty"`
	TriggerSource               string                    `json:"triggerSource,omitempty"`
	TriggerKind                 string                    `json:"triggerKind,omitempty"`
	EventID                     w.EventID                 `json:"eventId,omitempty"`
	EventEpoch                  w.Epoch                   `json:"eventEpoch,omitempty"`
	ControllerEpoch             w.Epoch                   `json:"controllerEpochAtReceive,omitempty"`
	CurrentGeneration           w.GenerationID            `json:"currentGenerationAtReceive,omitempty"`
	StartedAt                   string                    `json:"startedAt"`
	FinishedAt                  string                    `json:"finishedAt"`
	Converged                   bool                      `json:"converged"`
	Discarded                   bool                      `json:"discarded,omitempty"`
	DiscardReason               string                    `json:"discardReason,omitempty"`
	PlanIterations              []PlanTrace               `json:"planIterations"`
	TotalOperations             int                       `json:"totalOperations"`
	MutationOperations          int                       `json:"mutationOperations"`
	AttemptedOperations         int                       `json:"attemptedOperations"`
	ExecutedMutations           int                       `json:"executedMutations"`
	VerifierMode                string                    `json:"verifierMode,omitempty"`
	VerifierRan                 bool                      `json:"verifierRan"`
	VerifierDiffEntries         int                       `json:"verifierDiffEntries"`
	LastUnacceptableDiffEntries int                       `json:"lastUnacceptableDiffEntries,omitempty"`
	NoCommitReason              string                    `json:"noCommitReason,omitempty"`
	ObservationRefreshFailed    bool                      `json:"observationRefreshFailed,omitempty"`
	ObservationRefreshError     string                    `json:"observationRefreshError,omitempty"`
	RuntimeValidationReports    []RuntimeValidationReport `json:"runtimeValidationReports,omitempty"`
	RuntimeValidationBlocking   bool                      `json:"runtimeValidationBlocking,omitempty"`
	InvariantViolations         []string                  `json:"invariantViolations,omitempty"`
	CommittedGeneration         w.GenerationID            `json:"committedGeneration,omitempty"`
	ParentGeneration            w.GenerationID            `json:"parentGeneration,omitempty"`
	CommitKind                  string                    `json:"commitKind,omitempty"`
	CommittedBy                 string                    `json:"committedBy,omitempty"`
	BootstrapManifestDigest     string                    `json:"bootstrapManifestDigest,omitempty"`
}

// RuntimeValidationReport is durable, redacted evidence from environment
// validation. It intentionally records labels/policies/status, not process
// command lines or user data.
type RuntimeValidationReport struct {
	Kind     string `json:"kind"`
	Subject  string `json:"subject"`
	Policy   string `json:"policy,omitempty"`
	Status   string `json:"status"`
	Action   string `json:"action,omitempty"`
	Blocking bool   `json:"blocking,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

type PlanTrace struct {
	Iteration           int              `json:"iteration"`
	PlanID              w.PlanID         `json:"planId"`
	BaseEpoch           w.Epoch          `json:"baseEpoch"`
	Reason              string           `json:"reason"`
	PlannedOperations   int              `json:"plannedOperations"`
	MutationOperations  int              `json:"mutationOperations"`
	AttemptedOperations int              `json:"attemptedOperations"`
	ExecutedMutations   int              `json:"executedMutations"`
	VerifierRan         bool             `json:"verifierRan"`
	Operations          []OperationTrace `json:"operations"`
	VerifierDiffEntries int              `json:"verifierDiffEntries"`
}

type OperationTrace struct {
	ID                     w.OperationID            `json:"id"`
	Kind                   string                   `json:"kind"`
	Risk                   string                   `json:"risk,omitempty"`
	LifecycleRemovalMethod w.LifecycleRemovalMethod `json:"lifecycleRemovalMethod,omitempty"`
	Mutation               bool                     `json:"mutation"`
	Attempted              bool                     `json:"attempted"`
	Executed               bool                     `json:"executed"`
	StartedAt              string                   `json:"startedAt,omitempty"`
	FinishedAt             string                   `json:"finishedAt,omitempty"`
}

// StagedCommit is returned by BeginCommit.
type StagedCommit struct {
	id   string
	body ControllerCommit
}

// PersistentStore. design.md §8.1.
type PersistentStore interface {
	LoadCurrentGeneration(ctx context.Context) (CommittedGeneration, error)
	BeginCommit(ctx context.Context, commit ControllerCommit) (StagedCommit, error)
	Commit(ctx context.Context, staged StagedCommit) (w.GenerationID, error)
	Abort(ctx context.Context, staged StagedCommit) error
}

// TransactionTraceRecorder stores transaction audit evidence that may not have a
// committed generation, such as verifier-replan aborts. It is intentionally
// separate from CURRENT so failure evidence cannot become DesiredWorld truth.
type TransactionTraceRecorder interface {
	RecordTransactionTrace(ctx context.Context, trace TransactionTrace) error
}

// MemoryStore is an in-memory PersistentStore.
type MemoryStore struct {
	mu      sync.Mutex
	current CommittedGeneration
	staged  map[string]ControllerCommit
	nextID  uint64
	traces  []TransactionTrace
}

// NewMemoryStore creates a store seeded with an initial DesiredWorld.
func NewMemoryStore(initial w.DesiredWorld) *MemoryStore {
	gid := w.GenerationID("G000001")
	return &MemoryStore{
		current: CommittedGeneration{
			ID:           gid,
			Desired:      initial,
			Checkpoint:   ControllerCheckpoint{Epoch: 0},
			StoreVersion: "1",
		},
		staged: map[string]ControllerCommit{},
		nextID: 2,
	}
}

func (s *MemoryStore) LoadCurrentGeneration(ctx context.Context) (CommittedGeneration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current, nil
}

func (s *MemoryStore) BeginCommit(ctx context.Context, c ControllerCommit) (StagedCommit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.Parent != s.current.ID {
		return StagedCommit{}, fmt.Errorf("store: parent generation mismatch (got %s, current %s)", c.Parent, s.current.ID)
	}
	id := fmt.Sprintf("staged-%d", s.nextID)
	s.staged[id] = c
	return StagedCommit{id: id, body: c}, nil
}

func (s *MemoryStore) Commit(ctx context.Context, staged StagedCommit) (w.GenerationID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.staged[staged.id]; !ok {
		return "", fmt.Errorf("store: unknown staged commit %s", staged.id)
	}
	delete(s.staged, staged.id)
	parent := s.current.ID
	gid := w.GenerationID(fmt.Sprintf("G%06d", s.nextID))
	s.nextID++
	trace := staged.body.Trace
	trace.ParentGeneration = parent
	trace.CommittedGeneration = gid
	trace.CommitKind = "controller-commit"
	trace.CommittedBy = "controller"
	s.current = CommittedGeneration{
		ID:              gid,
		Parent:          &parent,
		Desired:         staged.body.Desired,
		AcceptedLayouts: staged.body.AcceptedLayouts,
		Checkpoint:      staged.body.Checkpoint,
		Trace:           trace,
		StoreVersion:    "1",
	}
	return gid, nil
}

func (s *MemoryStore) Abort(ctx context.Context, staged StagedCommit) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.staged, staged.id)
	return nil
}

func (s *MemoryStore) RecordTransactionTrace(ctx context.Context, trace TransactionTrace) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traces = append(s.traces, trace)
	return nil
}

func (s *MemoryStore) TransactionTraces() []TransactionTrace {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]TransactionTrace(nil), s.traces...)
}

var _ TransactionTraceRecorder = (*MemoryStore)(nil)
