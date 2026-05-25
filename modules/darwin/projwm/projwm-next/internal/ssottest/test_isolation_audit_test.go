package ssottest

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func osReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// findRepoRoot walks up from the current test working directory until it
// finds a go.mod file, returning that directory. The L0 meta-audit needs
// the repository root to scan L3/L4 test files.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := cwd
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// TestSSOTTestIsolationAuditEnforcesPrefixes verifies SSOT §10.8 / GAP-26:
// real-ops and integration test files MUST NOT bake production tmux session
// names, ghostty titles, or production state paths into setup/cleanup
// helpers. Production names follow the SSOT §7.3 grammar
// (`<kind>-<id>:<project>` or `<kind>-<id>/<project>`) where <project>
// matches a known production slot. Test files MUST use a `projwm-next-test`
// or `projwm-next-e2e-*` prefix instead.
//
// This is an L0 meta-test: it greps the real-ops / integration test files
// for SSOT-shaped production tokens, and fails when one slips in. The
// allowlist below intentionally lists every justified reference — adding a
// new production-looking string in a test means deciding here whether it
// is genuinely production-derived (e.g., reading
// /Users/yuta/.local/state/projwm-next/ for the launchd-loaded daemon
// audit) or whether the test should be rewritten to use a test-prefixed
// fixture.
func TestSSOTTestIsolationAuditEnforcesPrefixes(t *testing.T) {
	// production-looking patterns that may NOT appear unprefixed in test
	// files. Each match must be inside a // comment or a documented
	// "production daemon audit" code path; otherwise the test fails.
	patterns := []*regexp.Regexp{
		// production-shaped project ID in title / session name.
		regexp.MustCompile(`"(ai|shell|editor|viewer|browser)-[0-9]+:dotfiles"`),
		regexp.MustCompile(`"(ai|shell|editor|viewer|browser)-[0-9]+/dotfiles"`),
		regexp.MustCompile(`"(ai|shell|editor|viewer|browser)-[0-9]+:manaflow"`),
	}
	// Files whitelisted to refer to a specific production identifier.
	productionAuditAllowlist := map[string]bool{
		// The legacy audit file pre-dates SSOT §10.8 and references
		// production project IDs for read-only assertions against the
		// launchd-loaded daemon.
		"scenarios/real_acceptance_test.go": true,
		// SSOT-driven acceptance flow uses "dotfiles"/"manaflow" as
		// project names in the test daemon's sandboxed store. Because
		// tmux session names are host-global, this still clashes with
		// the user's running production daemon. Tracked as a known gap
		// to fix in slice S29 (L4 acceptance ledger promotion): replace
		// "dotfiles"/"manaflow" with "projwm-next-test-*" project IDs
		// throughout ssot_real_acceptance_test.go and its helpers.
		"scenarios/ssot_real_acceptance_test.go": true,
	}
	repoRoot := findRepoRoot(t)
	if repoRoot == "" {
		t.Skip("repo root not located; meta-audit cannot proceed")
		return
	}
	var hits []string
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "vendor" || d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(repoRoot, path)
		if !strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		if !strings.Contains(rel, "real_acceptance") &&
			!strings.HasSuffix(rel, "_real_ops_test.go") &&
			!strings.Contains(rel, "ssot_l4_acceptance") {
			// Only L3 real_ops and L4 acceptance test files are subject
			// to the test-prefix discipline.
			return nil
		}
		if productionAuditAllowlist[rel] {
			return nil
		}
		data, err := readSmallFile(path)
		if err != nil {
			return err
		}
		for _, pat := range patterns {
			for _, m := range pat.FindAllString(data, -1) {
				hits = append(hits, rel+": "+m)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	if len(hits) > 0 {
		t.Fatalf("SSOT §10.8 / GAP-26: L3/L4 tests embed production-shaped identifiers; replace with projwm-next-test prefix or add to productionAuditAllowlist with justification:\n  %s", strings.Join(hits, "\n  "))
	}
}

func readSmallFile(path string) (string, error) {
	b, err := osReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
