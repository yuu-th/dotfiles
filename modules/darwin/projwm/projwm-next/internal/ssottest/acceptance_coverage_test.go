package ssottest

import (
	"os"
	"sort"
	"testing"
)

// The existing scenarios package contains useful acceptance-style tests, but
// this map is intentionally reserved for tests audited against
// queue/projwm-next-spec.md §9.1 specifically. Old-spec scenario names do not
// count here until they are checked against the current SSOT wording.
var ssotAcceptanceImplemented = map[string]string{}

func TestSSOTAcceptanceCoverageGate(t *testing.T) {
	if os.Getenv("PROJWM_NEXT_REAL_ACCEPTANCE") != "1" {
		t.Skip("set PROJWM_NEXT_REAL_ACCEPTANCE=1 to enforce SSOT §9.1 real E2E coverage")
	}
	var missing []string
	for _, scenario := range ssotAcceptanceScenarios {
		if ssotAcceptanceImplemented[scenario] == "" {
			missing = append(missing, scenario)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("SSOT §9.1 acceptance coverage is incomplete: implemented=%d required=%d missing=%v", len(ssotAcceptanceImplemented), len(ssotAcceptanceScenarios), missing)
	}
}
