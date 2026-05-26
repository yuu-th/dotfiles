package ssottest

import (
	"path/filepath"
	"strings"
	"testing"
)

// gateAuditAllowlist is the honest deferral list: ledger rows whose
// Layer claims L3 or L4 but whose owner files do not (yet) carry the
// `real_ops` / `integration` build tag. SSOT §10.6 demands these layers
// and the ledger rightfully claims them, but the real-environment
// promotion is deferred to slice S29 (L4 acceptance + hardcode 解消).
//
// Removing an entry from this map means the corresponding test file
// has been promoted: it now carries the proper build tag and exercises
// the real environment. The audit will then enforce the promotion.
//
// NEVER add a new entry without a written justification in the ledger
// row's Subject — the audit is supposed to catch unnoticed
// false-green claims.
var gateAuditAllowlist = map[string]string{
	"INV-07": "L3 zed_test.go runs against fake adapter; real_ops promotion needs Zed-spawn S29 work",
	// OP-08 / OP-10: promoted out of the allowlist in Phase 5 S29 partial
	// when ssot_l4_acceptance_spec_test.go gained `//go:build integration`.
	// Enforcement now active; if either ledger row re-loses its integration
	// tag the audit catches it.
}

// SSOT §10.2 / §10.5 / GAP-25: L3 real_ops tests MUST be gated by the
// `real_ops` build tag, and L4 acceptance tests by `integration`. A
// ledger row that claims L3 (or L4) status without having any owner
// file behind the appropriate tag would mean the "real operation"
// promise is unenforced — `go test ./...` would silently consider those
// suites green even when the real environment is absent.
//
// This L0 audit walks every ledger row, parses its declared Layer, and
// asserts the TestPath includes at least one file that carries the
// expected build tag. Honest deferrals are kept in gateAuditAllowlist.
func TestSSOTLedgerL3OwnersAreGatedByRealOpsTag(t *testing.T) {
	repoRoot := findRepoRoot(t)
	if repoRoot == "" {
		t.Skip("repo root not located")
		return
	}
	// findRepoRoot anchors on go.mod, which lives inside the projwm-next
	// module directory, so repoRoot already IS the projwm-next root for
	// the purpose of resolving ledger TestPath entries.
	pjwmRoot := repoRoot

	for _, item := range ssotLedger {
		layer := item.Layer
		// Only items that declare L3 or L4 need build-tag gating.
		// Skip statusRed/statusMissing items — they are not yet
		// behavior-honored anyway, and the gate is concerned with
		// false-green claims.
		if item.Status == statusRed || item.Status == statusMissing {
			continue
		}
		needsRealOps := strings.Contains(layer, "L3")
		needsIntegration := strings.Contains(layer, "L4")
		if !needsRealOps && !needsIntegration {
			continue
		}

		// Each TestPath entry is a relative path from projwm-next root.
		// Look up at least one matching file carrying the right tag.
		paths := strings.Fields(item.TestPath)
		gotRealOps := false
		gotIntegration := false
		for _, rel := range paths {
			abs := filepath.Join(pjwmRoot, rel)
			body, err := readSmallFile(abs)
			if err != nil {
				continue
			}
			if hasBuildTag(body, "real_ops") {
				gotRealOps = true
			}
			if hasBuildTag(body, "integration") {
				gotIntegration = true
			}
		}

		if _, isAllowlisted := gateAuditAllowlist[item.ID]; isAllowlisted {
			// Honest deferral — recorded but does not fail the build.
			// Logged so future audits see the count and the entries
			// remain visible in CI output.
			t.Logf("%s allowed (Layer=%s): %s", item.ID, layer, gateAuditAllowlist[item.ID])
			continue
		}

		if needsRealOps && needsIntegration {
			if !gotRealOps && !gotIntegration {
				t.Errorf("%s (Layer=%s): claims L3+L4 but no owner file has `real_ops` or `integration` build tag", item.ID, layer)
			}
		} else if needsRealOps {
			if !gotRealOps {
				t.Errorf("%s (Layer=%s): claims L3 but no owner file has `//go:build real_ops`", item.ID, layer)
			}
		} else if needsIntegration {
			if !gotIntegration {
				t.Errorf("%s (Layer=%s): claims L4 but no owner file has `//go:build integration`", item.ID, layer)
			}
		}
	}
}

// L2 deterministic harness owners MUST NOT live behind real_ops/integration
// build tags — they are supposed to run in the always-on `go test ./...`
// baseline. If a row claims L2 but its only owner is behind a tag, the
// row is misclassified.
func TestSSOTLedgerL2OwnersAreNotBehindRealEnvTag(t *testing.T) {
	repoRoot := findRepoRoot(t)
	if repoRoot == "" {
		t.Skip("repo root not located")
		return
	}
	// findRepoRoot anchors on go.mod, which lives inside the projwm-next
	// module directory, so repoRoot already IS the projwm-next root for
	// the purpose of resolving ledger TestPath entries.
	pjwmRoot := repoRoot

	for _, item := range ssotLedger {
		if item.Status == statusRed || item.Status == statusMissing {
			continue
		}
		// Look at items whose layer contains L2 but neither L3 nor L4.
		// Items spanning L2+L3 are expected to have multiple owner files
		// (some tagged, some not) — they are not the audit target.
		layer := item.Layer
		if !strings.Contains(layer, "L2") {
			continue
		}
		if strings.Contains(layer, "L3") || strings.Contains(layer, "L4") {
			continue
		}
		paths := strings.Fields(item.TestPath)
		if len(paths) == 0 {
			continue
		}
		anyUngated := false
		for _, rel := range paths {
			abs := filepath.Join(pjwmRoot, rel)
			body, err := readSmallFile(abs)
			if err != nil {
				continue
			}
			if !hasBuildTag(body, "real_ops") && !hasBuildTag(body, "integration") {
				anyUngated = true
				break
			}
		}
		if !anyUngated {
			t.Errorf("%s (Layer=%s): pure L2 owner is locked behind real_ops/integration tag — L2 must be deterministic + always-on", item.ID, layer)
		}
	}
}

// hasBuildTag returns true if the file body has a `//go:build <tag>` or
// `// +build <tag>` directive near the top. Walks the first 20 lines
// (build directives MUST precede package clause).
func hasBuildTag(body, tag string) bool {
	lines := strings.SplitN(body, "\n", 25)
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "package ") {
			break
		}
		if strings.HasPrefix(ln, "//go:build ") || strings.HasPrefix(ln, "// +build ") {
			if strings.Contains(ln, tag) {
				return true
			}
		}
	}
	return false
}
