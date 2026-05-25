package scenario

import "testing"

func TestAcceptanceMatrixCoversSpecsSteps(t *testing.T) {
	got := map[string]AcceptanceStep{}
	for _, step := range AcceptanceMatrix() {
		if _, exists := got[step.ID]; exists {
			t.Fatalf("duplicate acceptance step %s", step.ID)
		}
		got[step.ID] = step
	}
	want := []string{
		"S1.1", "S1.2", "S1.3", "S1.4",
		"S2.1", "S2.2",
		"S3.1", "S3.2",
		"S4.1", "S4.2", "S4.3",
		"S5.1", "S5.2",
		"S6.1", "S6.2", "S6.3",
		"S7.1", "S7.2", "S7.3", "S7.4", "S7.5",
		"S8.A", "S8.B", "S8.C", "S8.D", "S8.E", "S8.F",
	}
	for _, id := range want {
		step, ok := got[id]
		if !ok {
			t.Fatalf("missing acceptance step %s", id)
		}
		if len(step.RequiredBackends) == 0 {
			t.Fatalf("%s has no required backends", id)
		}
		if step.RealMode == "" {
			t.Fatalf("%s has no real backend mode", id)
		}
	}
}

func TestStateScenariosIncludeRealBackend(t *testing.T) {
	coverage := map[string]AcceptanceCoverageRequirement{}
	for _, req := range AcceptanceCoverageMatrix() {
		coverage[req.ID] = req
	}
	for _, step := range AcceptanceMatrix() {
		switch step.RealMode {
		case RealModeUserE2E, RealModeAudit:
			if !hasBackend(step.RequiredBackends, BackendReal) {
				t.Fatalf("%s (%s) must include real backend acceptance", step.ID, step.Name)
			}
		case RealModeUnsafe, RealModeNoReal:
			req, ok := coverage[step.ID]
			if !ok {
				t.Fatalf("%s has no coverage row", step.ID)
			}
			if req.AuthorityStatus == CoverageCovered {
				t.Fatalf("%s cannot be final-authority covered while real mode is %s", step.ID, step.RealMode)
			}
		default:
			t.Fatalf("%s has invalid real mode %q", step.ID, step.RealMode)
		}
	}
}

func TestAcceptanceCoverageMatrixCoversCompletionDefinition(t *testing.T) {
	got := map[string]AcceptanceCoverageRequirement{}
	for _, req := range AcceptanceCoverageMatrix() {
		if _, exists := got[req.ID]; exists {
			t.Fatalf("duplicate coverage requirement %s", req.ID)
		}
		got[req.ID] = req
	}
	want := []string{
		"INV.1", "INV.2", "INV.3", "INV.4", "INV.5", "INV.6", "INV.7", "INV.8", "INV.9", "INV.10", "INV.11", "INV.12", "INV.13",
		"S1.1", "S1.2", "S1.3", "S1.4",
		"S2.1", "S2.2",
		"S3.1", "S3.2",
		"S4.1", "S4.2", "S4.3",
		"S5.1", "S5.2",
		"S6.1", "S6.2", "S6.3",
		"S7.1", "S7.2", "S7.3", "S7.4", "S7.5",
		"S8.A", "S8.B", "S8.C", "S8.D", "S8.E", "S8.F",
		"EVT.4.1", "EVT.4.2", "EVT.4.3", "EVT.4.4", "EVT.4.5",
		"DET.5.1", "DET.5.2", "DET.5.3", "DET.5.4", "DET.5.5",
		"PRIV.6.1", "PRIV.6.2", "PRIV.6.3", "PRIV.6.4", "PRIV.6.5",
		"AUTH.7.1", "AUTH.7.2",
		"DONE.9.1", "DONE.9.2", "DONE.9.3", "DONE.9.4", "DONE.9.5",
		// Window-content semantics red rows (tmux session, AI auto-launch,
		// viewer grouped tmux, Vivaldi tab URL inspection). See
		// scenarios/window_content_red_test.go.
		"SESS.1", "SESS.2", "SESS.3", "PRIV.6.5b",
	}
	for _, id := range want {
		if _, ok := got[id]; !ok {
			t.Fatalf("missing coverage requirement %s", id)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("coverage matrix has %d rows, want %d", len(got), len(want))
	}
}

func TestAcceptanceCoverageRequirementsHaveExecutableOwners(t *testing.T) {
	for _, req := range AcceptanceCoverageMatrix() {
		if req.Source == "" {
			t.Fatalf("%s has no source", req.ID)
		}
		if req.Owner == "" {
			t.Fatalf("%s has no diagnostic/fake-simulator owner", req.ID)
		}
		if req.Owner == "none yet" {
			t.Fatalf("%s hides missing coverage behind a non-executable owner", req.ID)
		}
		if req.RealOwner == "" {
			t.Fatalf("%s has no real acceptance owner", req.ID)
		}
		if req.RealOwner == "none yet" {
			t.Fatalf("%s hides missing real coverage behind a non-executable owner", req.ID)
		}
		switch req.RealStatus {
		case CoverageCovered, CoveragePartial, CoverageBlocked:
		default:
			t.Fatalf("%s has invalid real coverage status %q", req.ID, req.RealStatus)
		}
		if req.Description == "" {
			t.Fatalf("%s has no coverage description", req.ID)
		}
		if req.AuthorityOwner == "" {
			t.Fatalf("%s has no final authority owner", req.ID)
		}
		if req.AuthorityOwner == "none yet" || req.AuthorityOwner == "TestHumanE2EAcceptanceCoverageGate" {
			t.Fatalf("%s hides missing final authority coverage behind a surrogate owner", req.ID)
		}
		switch req.AuthorityStatus {
		case CoverageCovered, CoveragePartial, CoverageBlocked:
		default:
			t.Fatalf("%s has invalid final authority coverage status %q", req.ID, req.AuthorityStatus)
		}
		if req.AuthorityDescription == "" {
			t.Fatalf("%s has no final authority coverage description", req.ID)
		}
	}
}

func TestFinalAuthorityIsStricterThanVisibleRealDiagnostics(t *testing.T) {
	for _, req := range AcceptanceCoverageMatrix() {
		if req.RealStatus == CoverageCovered && req.AuthorityStatus == CoverageCovered && req.AuthorityOwner == req.RealOwner {
			t.Fatalf("%s is marked final-authority covered solely because diagnostic real coverage is green; production provenance/full invariant proof must be explicit", req.ID)
		}
	}
}

func hasBackend(backends []BackendKind, want BackendKind) bool {
	for _, got := range backends {
		if got == want {
			return true
		}
	}
	return false
}
