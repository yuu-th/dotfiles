package mutation

import "errors"

type ResolveStatus string

const (
	ResolveUniqueStrong ResolveStatus = "unique-strong"
	ResolveAmbiguous    ResolveStatus = "ambiguous"
	ResolveWeak         ResolveStatus = "weak"
	ResolveNone         ResolveStatus = "none"
)

type Evidence string

const (
	EvidenceLiveWindowID Evidence = "live-window-id"
	EvidenceTitle        Evidence = "title"
	EvidenceBundleID     Evidence = "bundle-id"
	EvidenceSavedURLs    Evidence = "saved-urls"
	EvidenceFrontmost    Evidence = "frontmost"
	EvidenceLastFocused  Evidence = "last-focused"
)

type OperationKind string

const (
	OperationQueryWindows       OperationKind = "query-windows"
	OperationResolverDryRun     OperationKind = "resolver-dry-run"
	OperationGhosttySpawn       OperationKind = "ghostty-spawn"
	OperationZedSpawn           OperationKind = "zed-spawn"
	OperationVivaldiSpawn       OperationKind = "vivaldi-spawn"
	OperationMoveManagedWindow  OperationKind = "move-managed-window"
	OperationMoveExternalApp    OperationKind = "move-external-app"
	OperationCloseWindow        OperationKind = "close-window"
	OperationFullLayoutRestore  OperationKind = "full-layout-restore"
	OperationFrameGrouping      OperationKind = "frame-grouping"
	OperationExposedFocusWindow OperationKind = "exposed-focus-window"
	OperationBestEffortMutation OperationKind = "best-effort-mutation"
)

type ResolveResult struct {
	Status            ResolveStatus
	Candidates        []string
	Selected          string
	Confidence        float64
	RequiredEvidence  []Evidence
	ForbiddenEvidence []Evidence
}

type Operation struct {
	Kind         OperationKind
	Precondition func() error
	Resolve      func() ResolveResult
	Execute      func(string) error
	Settle       func() error
	Verify       func() error
	Commit       func() error
}

var (
	ErrUnsafeResolution = errors.New("unsafe resolution")
	ErrVerifyFailed     = errors.New("verification failed")
	ErrBlockedOperation = errors.New("blocked operation")
)

var blockedFirstImplementation = map[OperationKind]bool{
	OperationMoveExternalApp:    true,
	OperationCloseWindow:        true,
	OperationFullLayoutRestore:  true,
	OperationFrameGrouping:      true,
	OperationExposedFocusWindow: true,
	OperationBestEffortMutation: true,
}

func CanMutate(r ResolveResult) bool {
	return r.Status == ResolveUniqueStrong &&
		r.Confidence == 1.0 &&
		len(r.Candidates) == 1 &&
		r.Selected != "" &&
		len(r.RequiredEvidence) > 0 &&
		len(r.ForbiddenEvidence) == 0
}

func Run(op Operation) error {
	if blockedFirstImplementation[op.Kind] {
		return ErrBlockedOperation
	}
	if op.Precondition != nil {
		if err := op.Precondition(); err != nil {
			return err
		}
	}
	resolved := op.Resolve()
	if !CanMutate(resolved) {
		return ErrUnsafeResolution
	}
	if op.Execute != nil {
		if err := op.Execute(resolved.Selected); err != nil {
			return err
		}
	}
	if op.Settle != nil {
		if err := op.Settle(); err != nil {
			return err
		}
	}
	if op.Verify != nil {
		if err := op.Verify(); err != nil {
			return ErrVerifyFailed
		}
	}
	if op.Commit != nil {
		return op.Commit()
	}
	return nil
}
