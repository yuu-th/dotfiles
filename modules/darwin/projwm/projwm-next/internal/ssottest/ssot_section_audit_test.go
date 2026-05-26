package ssottest

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSSOTSection55NoMacOSNotificationUsage verifies SSOT §5.5:
// "macOS notification は一切使わない。すべて cockpit または CLI に集約."
//
// This is an L0 meta-audit: it greps the entire Go + Swift source for
// known macOS notification surfaces and fails if any production code
// (not test/comment) references them. Catches "user notification" leaks
// even when the surface is buried under build tags or platform-conditional code.
//
// If you have a legitimate need to surface a real notification (e.g. a
// new launchd-driven sidecar emitting a system alert), add the file
// path + line to the allowlist below WITH a written justification —
// the SSOT violation must be deliberate.
func TestSSOTSection55NoMacOSNotificationUsage(t *testing.T) {
	// Surfaces banned by SSOT §5.5.
	banned := []*regexp.Regexp{
		regexp.MustCompile(`\bNSUserNotification\b`),
		regexp.MustCompile(`\bUNUserNotification\b`),
		regexp.MustCompile(`\bUNMutableNotificationContent\b`),
		regexp.MustCompile(`\bterminal-notifier\b`),
		regexp.MustCompile(`\bnotify-send\b`),
		// osascript "display notification" string literal.
		regexp.MustCompile(`display notification`),
		regexp.MustCompile(`display dialog`),
	}

	// Files whose mention of a banned surface is documented and OK
	// (e.g., this audit itself, ARCHITECTURE.md describing what's banned).
	allowlist := map[string]bool{
		"modules/darwin/projwm/projwm-next/internal/ssottest/ssot_section_audit_test.go": true,
	}

	repoRoot := findRepoRoot(t)
	if repoRoot == "" {
		t.Skip("repo root not located")
		return
	}

	var hits []string
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == ".git" || name == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(repoRoot, path)
		// Restrict to source under modules/darwin/projwm/projwm-next (the
		// daemon + CLI we own) and the Swift wake watcher.
		if !strings.HasPrefix(rel, "modules/darwin/projwm/projwm-next/") &&
			!strings.HasSuffix(rel, ".swift") {
			return nil
		}
		if !strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, ".swift") {
			return nil
		}
		if allowlist[rel] {
			return nil
		}
		data, err := readSmallFile(path)
		if err != nil {
			return err
		}
		for _, pat := range banned {
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
		t.Fatalf("SSOT §5.5 violation: production code references macOS notification surfaces; remove the call or route the user feedback through cockpit cards / CLI doctor:\n  %s", strings.Join(hits, "\n  "))
	}
}

// TestSSOTSection44BR_PRIV_REDACT_AuditAuditsURLLiterals verifies SSOT
// §4.4 BR-PRIV-REDACT: "log / trace / status 出力では URL を redact 表示する".
//
// This is a heuristic L0 audit: it greps production Go code for fmt /
// log calls that interpolate fields named like a URL (`URL`, `url`,
// `Href`, etc.) without redacting them. False positives are accepted
// (the audit is informational + advisory, not gating) — that's why this
// test is a `t.Log` warning rather than `t.Fatal`. Once a real redaction
// pipeline lands (S20 follow-up), this can be tightened.
//
// Scope: opaque payload tokens (`browser-payload-v1-<32hex>`) are NOT
// URLs and are safe to log.
func TestSSOTSection44BR_PRIV_REDACT_AuditAuditsURLLiterals(t *testing.T) {
	// Patterns that strongly indicate a raw URL is being interpolated.
	urlInterp := []*regexp.Regexp{
		// fmt.*Printf("...%s...", ..., URL, ...) — flag any printf-family
		// call that mentions a variable whose name ends in URL / Url / Href.
		regexp.MustCompile(`(?i)(Printf|Sprintf|Errorf|Println|Print)\b[^(]*\([^)]*\b(URL|Url|Href)\b`),
	}
	// Files where URL interpolation is auditable / documented:
	// adapter/browser internal handling (URL is the operand by design,
	// not a leak); test files (test inputs/expectations).
	allowlist := []string{
		"internal/adapter/browser/",
		"_test.go",
		"cmd/projwm/cmd_browser.go", // CLI surface — user explicitly typed the URL.
		"internal/ssottest/ssot_section_audit_test.go",
	}

	repoRoot := findRepoRoot(t)
	if repoRoot == "" {
		t.Skip("repo root not located")
		return
	}
	var hits []string
	_ = filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				name := d.Name()
				if name == "vendor" || name == ".git" {
					return fs.SkipDir
				}
			}
			return nil
		}
		rel, _ := filepath.Rel(repoRoot, path)
		if !strings.HasPrefix(rel, "modules/darwin/projwm/projwm-next/") {
			return nil
		}
		if !strings.HasSuffix(rel, ".go") {
			return nil
		}
		for _, frag := range allowlist {
			if strings.Contains(rel, frag) {
				return nil
			}
		}
		data, err := readSmallFile(path)
		if err != nil {
			return nil
		}
		for _, pat := range urlInterp {
			for _, m := range pat.FindAllString(data, -1) {
				hits = append(hits, rel+": "+strings.TrimSpace(m))
			}
		}
		return nil
	})
	if len(hits) > 0 {
		t.Logf("SSOT §4.4 BR-PRIV-REDACT advisory: %d call site(s) may interpolate raw URL into log/trace output. Audit and route through a redact helper if any leak production URLs:\n  %s", len(hits), strings.Join(hits, "\n  "))
	}
}
