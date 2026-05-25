package scenarios

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type ssotL4AcceptanceSpec struct {
	ID                      string
	Name                    string
	Contract                string
	WrapperOwner            string
	RealIntegrationPatterns []string
}

// ssotL4AcceptanceSpecs is the test-side transcription of
// queue/projwm-next-spec.md §9.1. Do not satisfy these rows with the older
// specs.md acceptance matrix unless the real owner body has been audited
// against the current SSOT wording.
var ssotL4AcceptanceSpecs = []ssotL4AcceptanceSpec{
	{
		ID:                      "S1",
		Name:                    "SwitchProfile",
		Contract:                "old profile windows close; new profile windows summon; final observed world satisfies all invariants",
		WrapperOwner:            "scenarios.TestSSOTL4S1SwitchProfile",
		RealIntegrationPatterns: []string{"TestHumanE2ESwitchProfileSteps"},
	},
	{
		ID:                      "S2",
		Name:                    "ArchiveProject",
		Contract:                "archived project windows disappear and tmux sessions are killed",
		WrapperOwner:            "scenarios.TestSSOTL4S2ArchiveProject",
		RealIntegrationPatterns: []string{"TestHumanE2EArchiveUnarchiveSteps", "TestHumanE2EProductionRemovalWithoutCloseWindowSteps"},
	},
	{
		ID:                      "S3",
		Name:                    "UnarchiveProject",
		Contract:                "unarchived project returns to park state and does not auto-assign or auto-spawn",
		WrapperOwner:            "scenarios.TestSSOTL4S3UnarchiveProject",
		RealIntegrationPatterns: []string{"TestHumanE2ESSOTUnarchiveProjectParkStateSteps"},
	},
	{
		ID:                      "S4",
		Name:                    "Assign/Unassign",
		Contract:                "slot assignment and unassignment update DesiredWorld and converge without stale windows",
		WrapperOwner:            "scenarios.TestSSOTL4S4AssignUnassign",
		RealIntegrationPatterns: []string{"TestHumanE2EAssignUnassignSteps"},
	},
	{
		ID:                      "S5",
		Name:                    "Reconcile",
		Contract:                "reconcile repairs observed drift and is zero-mutation when already converged",
		WrapperOwner:            "scenarios.TestSSOTL4S5Reconcile",
		RealIntegrationPatterns: []string{"TestHumanE2EReconcileStabilitySteps", "TestHumanE2EReconcileZeroMutationTraceSteps"},
	},
	{
		ID:                      "S6",
		Name:                    "macOS Restart Recovery",
		Contract:                "after macOS restart, projwmd bootstrap recreates all sessions/windows and reaches convergence within one minute",
		WrapperOwner:            "scenarios.TestSSOTL4S6MacOSRestartRecovery",
		RealIntegrationPatterns: []string{"TestHumanE2ESSOTMacOSRestartRecoverySteps"},
	},
	{
		ID:                      "S7",
		Name:                    "OmniWM Restart Recovery",
		Contract:                "after OmniWM restart, live sessions are reused, missing windows are recreated, and windows return to their slots",
		WrapperOwner:            "scenarios.TestSSOTL4S7OmniWMRestartRecovery",
		RealIntegrationPatterns: []string{"TestHumanE2ESSOTOmniWMRestartRecoverySteps"},
	},
	{
		ID:                      "S8",
		Name:                    "Summon Idempotency",
		Contract:                "repeated summon of the same identity focuses/reuses the existing window and never duplicates it",
		WrapperOwner:            "scenarios.TestSSOTL4S8SummonIdempotency",
		RealIntegrationPatterns: []string{"TestHumanE2ESSOTSummonIdempotencySteps"},
	},
	{
		ID:                      "S9",
		Name:                    "Drift Repair",
		Contract:                "window moved outside its slot is detected as drift and automatically returned without respawn",
		WrapperOwner:            "scenarios.TestSSOTL4S9DriftRepair",
		RealIntegrationPatterns: []string{"TestHumanE2EManagedWindowCrossWorkspaceMoveSteps", "TestHumanE2ESameWorkspaceReorderEventSteps"},
	},
	{
		ID:                      "S10",
		Name:                    "tmux/Ghostty/Zed Crash Recovery",
		Contract:                "tmux, Ghostty, and Zed crash paths recover through the transaction loop and preserve DesiredWorld authority",
		WrapperOwner:            "scenarios.TestSSOTL4S10CrashRecovery",
		RealIntegrationPatterns: []string{"TestHumanE2ESSOTCrashRecoverySteps"},
	},
}

func TestSSOTL4AcceptanceCoverageGate(t *testing.T) {
	if len(ssotL4AcceptanceSpecs) != 10 {
		t.Fatalf("SSOT §9.1 L4 acceptance scenario count = %d, want 10", len(ssotL4AcceptanceSpecs))
	}
	seen := map[string]bool{}
	for _, spec := range ssotL4AcceptanceSpecs {
		if seen[spec.ID] {
			t.Fatalf("duplicate SSOT §9.1 L4 scenario %s", spec.ID)
		}
		seen[spec.ID] = true
		if spec.Name == "" || spec.Contract == "" || spec.WrapperOwner == "" || len(spec.RealIntegrationPatterns) == 0 {
			t.Fatalf("SSOT §9.1 L4 scenario %+v is incomplete", spec)
		}
	}
	for i := 1; i <= 10; i++ {
		id := "S" + strconv.Itoa(i)
		if !seen[id] {
			t.Fatalf("missing SSOT §9.1 L4 scenario %s", id)
		}
	}
}

func TestSSOTL4AcceptanceCoverageReferencesExistingTestFunctions(t *testing.T) {
	wrapperFuncs := testFunctionsInScenarioFiles(t, "*_test.go", "scenarios.")
	realFuncs := testFunctionsInScenarioFiles(t, "*real_acceptance_test.go", "")
	var missing []string
	for _, spec := range ssotL4AcceptanceSpecs {
		if !wrapperFuncs[spec.WrapperOwner] {
			missing = append(missing, spec.ID+" wrapper "+spec.WrapperOwner)
		}
		for _, owner := range spec.RealIntegrationPatterns {
			if !realFuncs[owner] {
				missing = append(missing, spec.ID+" real "+owner)
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("SSOT §9.1 L4 acceptance references missing dedicated test functions: %v", missing)
	}
}

func TestSSOTL4AcceptanceDoesNotUseOldSpecsMatrixAsAuthority(t *testing.T) {
	for _, spec := range ssotL4AcceptanceSpecs {
		for _, owner := range spec.RealIntegrationPatterns {
			if owner == "TestHumanE2EAcceptanceCoverageGate" || owner == "AcceptanceCoverageMatrix" {
				t.Fatalf("%s uses old meta coverage owner %q instead of a real SSOT §9.1 body", spec.ID, owner)
			}
		}
	}
}

func testFunctionsInScenarioFiles(t *testing.T, glob, prefix string) map[string]bool {
	t.Helper()
	funcs := map[string]bool{}
	matches, err := filepath.Glob(glob)
	if err != nil {
		t.Fatalf("glob scenario tests %s: %v", glob, err)
	}
	for _, path := range matches {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}
			funcs[prefix+fn.Name.Name] = true
		}
	}
	return funcs
}

func ssotL4SpecByID(id string) (ssotL4AcceptanceSpec, bool) {
	for _, spec := range ssotL4AcceptanceSpecs {
		if spec.ID == id {
			return spec, true
		}
	}
	return ssotL4AcceptanceSpec{}, false
}
