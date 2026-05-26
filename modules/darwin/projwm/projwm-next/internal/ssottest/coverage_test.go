package ssottest

import "testing"

type coverageItem struct {
	ID      string
	Layer   string
	Subject string
	Path    string
}

// ssotCoverage is an inventory of where evidence should live. It is not
// behavioral evidence by itself; ledger rows must cite concrete owner tests.
var ssotCoverage = []coverageItem{
	{ID: "L0-naming", Layer: "L0", Subject: "SSOT §7.3 naming derivation and title parsing", Path: "internal/naming"},
	{ID: "L0-identity", Layer: "L0", Subject: "SSOT §2.2/§3.4 identity uniqueness", Path: "internal/identity"},
	{ID: "L0-reducer", Layer: "L0", Subject: "SSOT §4.1 user operations mutate DesiredWorld only", Path: "internal/reducer"},
	{ID: "L0-planner", Layer: "L0", Subject: "SSOT §6.3 L1/L2/L3 drift planning", Path: "internal/planner"},
	{ID: "L0-verifier", Layer: "L0", Subject: "SSOT §7.1 predicted vs observed verification", Path: "internal/verifier"},
	{ID: "L0-invariant", Layer: "L0", Subject: "SSOT §3.4 invariant checker", Path: "internal/invariant"},

	{ID: "L1-transaction-loop", Layer: "L1", Subject: "SSOT §7.1 observe-plan-execute-observe-replan", Path: "internal/controller"},
	{ID: "L1-scenarios", Layer: "L1", Subject: "SSOT §9.1 fake acceptance scenarios", Path: "scenarios"},
	{ID: "L1-fake-wm", Layer: "L1", Subject: "SSOT §10.1 fake operation backend", Path: "internal/adapter/wm"},

	{ID: "L2-wm-mock", Layer: "L2", Subject: "SSOT §10.3 mock executor retry/timeout/error handling", Path: "internal/adapter/wm"},
	{ID: "L2-executor", Layer: "L2", Subject: "SSOT §7.5 executor precondition enforcement", Path: "internal/executor"},
	{ID: "L2-session-mock", Layer: "L2", Subject: "SSOT §7.5 tmux session adapter contract", Path: "internal/adapter/session"},
	{ID: "L2-browser-mock", Layer: "L2", Subject: "SSOT §4.4 browser privacy and lifecycle contract", Path: "internal/adapter/browser"},
	{ID: "L2-zed-mock", Layer: "L2", Subject: "SSOT §4.4 Zed project launch and close contract", Path: "internal/adapter/zed"},

	{ID: "L3-spawn", Layer: "L3", Subject: "SSOT §10.4 S1-S11 real spawn operations", Path: "internal/adapter/wm"},
	{ID: "L3-move", Layer: "L3", Subject: "SSOT §10.4 M1-M2 real move operations", Path: "internal/adapter/wm"},
	{ID: "L3-reorder", Layer: "L3", Subject: "SSOT §10.4 R1-R4 real reorder operations", Path: "internal/adapter/wm"},
	{ID: "L3-close", Layer: "L3", Subject: "SSOT §10.4 C1/C4/C5 real close operations", Path: "internal/adapter/wm"},
	{ID: "L3-focus", Layer: "L3", Subject: "SSOT §10.4 F1-F4 real focus operations", Path: "internal/adapter/wm"},
	{ID: "L3-identity", Layer: "L3", Subject: "SSOT §10.4 I1-I3 identity restoration", Path: "internal/naming"},
	{ID: "L3-tmux", Layer: "L3", Subject: "SSOT §10.4 T1-T4 real tmux operations", Path: "internal/adapter/session/ssot_l3_session_real_ops_test.go"},
	{ID: "L3-startup", Layer: "L3", Subject: "SSOT §10.4 B1-B4 startup recovery operations", Path: "internal/controller"},

	{ID: "L4-acceptance", Layer: "L4", Subject: "SSOT §9.1 S1-S10 real E2E scenarios", Path: "scenarios"},
}

var ssotUserOperations = []string{
	"op01-shell-jump",
	"op02-editor-jump",
	"op03-browser-jump",
	"op04-project-switch",
	"op05-same-slot-window-switch",
	"op06-viewer-jump",
	"op07-cockpit-show-hide",
	"op08-profile-switch",
	"op09-project-add",
	"op10-project-archive-unarchive",
	"op11-scratch-shell",
	"op12-add-window",
	"op13-remove-window",
	"op14-browser-add-tab",
	"op15-browser-remove-tab",
	"op16-browser-change-tab-url",
	"op17-browser-reorder-tabs",
}

var ssotAcceptanceScenarios = []string{
	"S1-switch-profile",
	"S2-archive-project",
	"S3-unarchive-project",
	"S4-assign-unassign",
	"S5-reconcile",
	"S6-macos-restart-recovery",
	"S7-omniwm-restart-recovery",
	"S8-summon-idempotency",
	"S9-drift-repair",
	"S10-tmux-ghostty-zed-crash-recovery",
}

var ssotRealOps = []string{
	"S1-spawn-shell",
	"S2-spawn-shell-already-exists",
	"S3-spawn-editor",
	"S4-spawn-editor-empty-project-cleanup",
	"S5-spawn-editor-already-exists",
	"S6-spawn-browser",
	"S7-spawn-browser-already-exists",
	"S8-spawn-viewer",
	"S9-spawn-viewer-already-exists",
	"S10-spawn-cockpit",
	"S11-spawn-cockpit-already-exists",
	"M1-move-to-workspace",
	"M2-move-to-workspace-already-on-target",
	"R1-reorder-columns",
	"R2-reorder-columns-already-correct",
	"R3-reorder-columns-partial-match",
	"R4-reorder-columns-empty-workspace",
	"C1-lifecycle-removal-primary-close-surfaces",
	"C4-close-window-already-gone",
	"C5-close-cockpit",
	"F1-focus-workspace",
	"F2-focus-workspace-nonexistent",
	"F3-focus-window",
	"F4-focus-window-vanished",
	"I1-identity-from-title",
	"I2-identity-from-title-viewer",
	"I3-identity-from-title-unknown", // shell test t.Skip — see TestIdentityFromTitleUnknown comment
	"T1-tmux-ensure-session",
	"T2-tmux-ensure-session-already-exists",
	"T3-tmux-ensure-grouped-session",
	"T4-tmux-kill-session",
	"B1-startup-normal",
	"B2-startup-missing-window",
	"B3-startup-orphan-window",
	"B4-startup-state-corrupted",
	"U1-scratch-shell-show-hide",
}

var ssotL2HarnessOps = []string{
	"S12-spawn-settle-timeout-process-alive",
	"S13-spawn-settle-timeout-process-dead",
	"M3-move-to-workspace-focus-drift",
	"M4-move-to-workspace-retry",
	"M5-move-to-workspace-window-vanished",
	"C2-lifecycle-removal-fallback-close-surface",
	"C3-close-window-retry",
	"F5-focus-window-navigation-before-focus",
}

func TestSSOTCoverageDeclaresEveryLayer(t *testing.T) {
	want := map[string]bool{"L0": false, "L1": false, "L2": false, "L3": false, "L4": false}
	for _, item := range ssotCoverage {
		if item.ID == "" || item.Subject == "" || item.Path == "" {
			t.Fatalf("coverage item must be fully specified: %+v", item)
		}
		if _, ok := want[item.Layer]; !ok {
			t.Fatalf("unknown SSOT test layer %q in %+v", item.Layer, item)
		}
		want[item.Layer] = true
	}
	for layer, seen := range want {
		if !seen {
			t.Fatalf("SSOT test layer %s has no declared coverage", layer)
		}
	}
}

func TestSSOTCoverageDeclaresAllUserOperations(t *testing.T) {
	if len(ssotUserOperations) != 17 {
		t.Fatalf("SSOT §4.1 defines 17 user operations, got %d", len(ssotUserOperations))
	}
	assertUnique(t, "user operation", ssotUserOperations)
}

func TestSSOTCoverageDeclaresAllAcceptanceScenarios(t *testing.T) {
	if len(ssotAcceptanceScenarios) != 10 {
		t.Fatalf("SSOT §9.1 defines 10 acceptance scenarios, got %d", len(ssotAcceptanceScenarios))
	}
	assertUnique(t, "acceptance scenario", ssotAcceptanceScenarios)
}

func TestSSOTCoverageDeclaresAllL3RealOps(t *testing.T) {
	if len(ssotRealOps) != 36 {
		t.Fatalf("SSOT §10.4 defines 36 L3 real operation checks, got %d", len(ssotRealOps))
	}
	assertUnique(t, "L3 real op", ssotRealOps)
}

func TestSSOTCoverageDeclaresAllL2HarnessOps(t *testing.T) {
	if len(ssotL2HarnessOps) != 8 {
		t.Fatalf("SSOT §10.4 defines 8 L2 deterministic harness checks, got %d", len(ssotL2HarnessOps))
	}
	assertUnique(t, "L2 harness op", ssotL2HarnessOps)
	assertUnique(t, "single-operation contract", append(append([]string{}, ssotRealOps...), ssotL2HarnessOps...))
}

func assertUnique(t *testing.T, label string, items []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, item := range items {
		if item == "" {
			t.Fatalf("%s coverage key must not be empty", label)
		}
		if seen[item] {
			t.Fatalf("duplicate %s coverage key %q", label, item)
		}
		seen[item] = true
	}
}
