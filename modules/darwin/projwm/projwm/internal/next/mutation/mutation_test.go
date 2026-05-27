package mutation

import (
	"errors"
	"testing"
)

func TestUniqueStrongResolutionAllowsMutation(t *testing.T) {
	r := ResolveResult{
		Status:           ResolveUniqueStrong,
		Candidates:       []string{"ow_1"},
		Selected:         "ow_1",
		Confidence:       1.0,
		RequiredEvidence: []Evidence{EvidenceLiveWindowID},
	}
	if !CanMutate(r) {
		t.Fatal("expected unique strong resolution to allow mutation")
	}
}

func TestForbiddenEvidenceBlocksMutation(t *testing.T) {
	for _, ev := range []Evidence{EvidenceTitle, EvidenceBundleID, EvidenceSavedURLs, EvidenceFrontmost, EvidenceLastFocused} {
		r := ResolveResult{
			Status:            ResolveUniqueStrong,
			Candidates:        []string{"ow_1"},
			Selected:          "ow_1",
			Confidence:        1.0,
			RequiredEvidence:  []Evidence{EvidenceLiveWindowID},
			ForbiddenEvidence: []Evidence{ev},
		}
		if CanMutate(r) {
			t.Fatalf("expected %s to block mutation", ev)
		}
	}
}

func TestAmbiguousResolutionBlocksMutation(t *testing.T) {
	r := ResolveResult{
		Status:           ResolveAmbiguous,
		Candidates:       []string{"ow_1", "ow_2"},
		Selected:         "ow_1",
		Confidence:       1.0,
		RequiredEvidence: []Evidence{EvidenceLiveWindowID},
	}
	if CanMutate(r) {
		t.Fatal("ambiguous result must not mutate")
	}
}

func TestSemanticOperationDoesNotCommitOnVerifyFailure(t *testing.T) {
	committed := false
	err := Run(Operation{
		Resolve: func() ResolveResult {
			return ResolveResult{
				Status:           ResolveUniqueStrong,
				Candidates:       []string{"ow_1"},
				Selected:         "ow_1",
				Confidence:       1.0,
				RequiredEvidence: []Evidence{EvidenceLiveWindowID},
			}
		},
		Verify: func() error { return errors.New("not settled") },
		Commit: func() error {
			committed = true
			return nil
		},
	})
	if !errors.Is(err, ErrVerifyFailed) {
		t.Fatalf("err = %v, want ErrVerifyFailed", err)
	}
	if committed {
		t.Fatal("commit must not run after verify failure")
	}
}

func TestFirstImplementationBlockedOperations(t *testing.T) {
	for _, kind := range []OperationKind{
		OperationMoveExternalApp,
		OperationCloseWindow,
		OperationFullLayoutRestore,
		OperationFrameGrouping,
		OperationExposedFocusWindow,
		OperationBestEffortMutation,
	} {
		err := Run(Operation{
			Kind: kind,
			Resolve: func() ResolveResult {
				return ResolveResult{
					Status:           ResolveUniqueStrong,
					Candidates:       []string{"ow_1"},
					Selected:         "ow_1",
					Confidence:       1.0,
					RequiredEvidence: []Evidence{EvidenceLiveWindowID},
				}
			},
		})
		if !errors.Is(err, ErrBlockedOperation) {
			t.Fatalf("%s err = %v, want ErrBlockedOperation", kind, err)
		}
	}
}

func TestAllowedOperationStillRequiresUniqueStrongResolution(t *testing.T) {
	err := Run(Operation{
		Kind: OperationMoveManagedWindow,
		Resolve: func() ResolveResult {
			return ResolveResult{
				Status:            ResolveWeak,
				Candidates:        []string{"ow_1"},
				Selected:          "ow_1",
				Confidence:        0.5,
				RequiredEvidence:  []Evidence{EvidenceTitle},
				ForbiddenEvidence: []Evidence{EvidenceTitle},
			}
		},
	})
	if !errors.Is(err, ErrUnsafeResolution) {
		t.Fatalf("err = %v, want ErrUnsafeResolution", err)
	}
}
