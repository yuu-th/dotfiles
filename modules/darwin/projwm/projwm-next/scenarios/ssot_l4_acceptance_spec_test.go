//go:build integration

// SSOT §10.5 L4 build tag: this file contains acceptance tests that
// require the real environment (omniwm + ghostty + tmux + Zed +
// Vivaldi). Without the `integration` build tag the file is excluded
// from `go test ./...` so the bare-baseline run cannot silently claim
// L4 coverage. To run the L4 suite:
//
//   PROJWM_NEXT_REAL_ACCEPTANCE=1 go test -tags integration ./scenarios/...
//
// (Honest deferral note: the test bodies still embed "dotfiles" /
// "manaflow" project IDs as L4 fixtures; the projwm-next-test prefix
// rewrite is tracked under S29 separately from the gate enforcement.)
package scenarios

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func requireSSOTL4Acceptance(t *testing.T) {
	t.Helper()
	if os.Getenv("PROJWM_NEXT_REAL_ACCEPTANCE") != "1" {
		t.Skip("set PROJWM_NEXT_REAL_ACCEPTANCE=1 to run SSOT §9.1 L4 acceptance tests")
	}
}

func TestSSOTL4S1SwitchProfile(t *testing.T) {
	runSSOTL4Scenario(t, "S1")
}

func TestSSOTL4S2ArchiveProject(t *testing.T) {
	runSSOTL4Scenario(t, "S2")
}

func TestSSOTL4S3UnarchiveProject(t *testing.T) {
	runSSOTL4Scenario(t, "S3")
}

func TestSSOTL4S4AssignUnassign(t *testing.T) {
	runSSOTL4Scenario(t, "S4")
}

func TestSSOTL4S5Reconcile(t *testing.T) {
	runSSOTL4Scenario(t, "S5")
}

func TestSSOTL4S6MacOSRestartRecovery(t *testing.T) {
	runSSOTL4Scenario(t, "S6")
}

func TestSSOTL4S7OmniWMRestartRecovery(t *testing.T) {
	runSSOTL4Scenario(t, "S7")
}

func TestSSOTL4S8SummonIdempotency(t *testing.T) {
	runSSOTL4Scenario(t, "S8")
}

func TestSSOTL4S9DriftRepair(t *testing.T) {
	runSSOTL4Scenario(t, "S9")
}

func TestSSOTL4S10CrashRecovery(t *testing.T) {
	runSSOTL4Scenario(t, "S10")
}

func runSSOTL4Scenario(t *testing.T, id string) {
	t.Helper()
	spec, ok := ssotL4SpecByID(id)
	if !ok {
		t.Fatalf("unknown SSOT §9.1 L4 scenario %s", id)
	}
	runSSOTL4IntegrationOwner(t, spec)
}

func runSSOTL4IntegrationOwner(t *testing.T, spec ssotL4AcceptanceSpec) {
	t.Helper()
	requireSSOTL4Acceptance(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	ownerPattern := strings.Join(spec.RealIntegrationPatterns, "|")
	pattern := "^(" + ownerPattern + ")$"
	cmd := exec.CommandContext(ctx, "go", "test", "-tags", "integration", ".", "-run", pattern, "-count=1", "-v")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("SSOT §9.1 L4 %s %s timed out: %v\ncontract: %s\n%s", spec.ID, spec.Name, ctx.Err(), spec.Contract, tailSSOTL4Output(out))
	}
	if err != nil {
		t.Fatalf("SSOT §9.1 L4 %s %s owner %s failed: %v\ncontract: %s\n%s", spec.ID, spec.Name, ownerPattern, err, spec.Contract, tailSSOTL4Output(out))
	}
	if !strings.Contains(string(out), "PASS") {
		t.Fatalf("SSOT §9.1 L4 %s %s owner %s produced no PASS evidence\ncontract: %s\n%s", spec.ID, spec.Name, ownerPattern, spec.Contract, tailSSOTL4Output(out))
	}
	if strings.Contains(string(out), "no tests to run") {
		t.Fatalf("SSOT §9.1 L4 %s %s owner %s matched no real tests\ncontract: %s\n%s", spec.ID, spec.Name, ownerPattern, spec.Contract, tailSSOTL4Output(out))
	}
}

func tailSSOTL4Output(out []byte) string {
	const max = 12000
	if len(out) <= max {
		return string(out)
	}
	return string(out[len(out)-max:])
}
