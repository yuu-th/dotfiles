//go:build real_ops

package ssottest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var ssotRealOpsImplemented = map[string]string{
	"S1-spawn-shell":                              "internal/adapter/wm.TestSpawnShell",
	"S2-spawn-shell-already-exists":               "internal/adapter/wm.TestSpawnShellAlreadyExists",
	"S3-spawn-editor":                             "internal/adapter/wm.TestSpawnEditor",
	"S4-spawn-editor-empty-project-cleanup":       "internal/adapter/wm.TestSpawnEditorEmptyProjectCleanup",
	"S5-spawn-editor-already-exists":              "internal/adapter/wm.TestSpawnEditorAlreadyExists",
	"S6-spawn-browser":                            "internal/adapter/wm.TestSpawnBrowser",
	"S7-spawn-browser-already-exists":             "internal/adapter/wm.TestSpawnBrowserAlreadyExists",
	"S8-spawn-viewer":                             "internal/adapter/wm.TestSpawnViewer",
	"S9-spawn-viewer-already-exists":              "internal/adapter/wm.TestSpawnViewerAlreadyExists",
	"S10-spawn-cockpit":                           "internal/adapter/wm.TestSpawnCockpit",
	"S11-spawn-cockpit-already-exists":            "internal/adapter/wm.TestSpawnCockpitAlreadyExists",
	"M1-move-to-workspace":                        "internal/adapter/wm.TestMoveToWorkspace",
	"M2-move-to-workspace-already-on-target":      "internal/adapter/wm.TestMoveToWorkspaceAlreadyOnTarget",
	"R1-reorder-columns":                          "internal/adapter/wm.TestReorderColumns",
	"R2-reorder-columns-already-correct":          "internal/adapter/wm.TestReorderColumnsAlreadyCorrect",
	"R3-reorder-columns-partial-match":            "internal/adapter/wm.TestReorderColumnsPartialMatch",
	"R4-reorder-columns-empty-workspace":          "internal/adapter/wm.TestReorderColumnsEmptyWorkspace",
	"C1-lifecycle-removal-primary-close-surfaces": "internal/adapter/wm.TestLifecycleRemovalPrimaryCloseSurfaces",
	"C4-close-window-already-gone":                "internal/adapter/wm.TestCloseWindowAlreadyGone",
	"C5-close-cockpit":                            "internal/adapter/wm.TestCloseCockpit",
	"F1-focus-workspace":                          "internal/adapter/wm.TestFocusWorkspace",
	"F2-focus-workspace-nonexistent":              "internal/adapter/wm.TestFocusWorkspaceNonExistent",
	"F3-focus-window":                             "internal/adapter/wm.TestFocusWindow",
	"F4-focus-window-vanished":                    "internal/adapter/wm.TestFocusWindowVanished",
	"I1-identity-from-title":                      "internal/naming.TestIdentityFromTitle",
	"I2-identity-from-title-viewer":               "internal/naming.TestIdentityFromTitleViewer",
	"I3-identity-from-title-unknown":              "internal/naming.TestIdentityFromTitleUnknown", // honest t.Skip — see test comment
	"T1-tmux-ensure-session":                      "internal/adapter/session.TestTmuxEnsureSession",
	"T2-tmux-ensure-session-already-exists":       "internal/adapter/session.TestTmuxEnsureSessionAlreadyExists",
	"T3-tmux-ensure-grouped-session":              "internal/adapter/session.TestTmuxEnsureGroupedSession",
	"T4-tmux-kill-session":                        "internal/adapter/session.TestTmuxKillSession",
	"B1-startup-normal":                           "internal/controller.TestStartupNormal",
	"B2-startup-missing-window":                   "internal/controller.TestStartupMissingWindow",
	"B3-startup-orphan-window":                    "internal/controller.TestStartupOrphanWindow",
	"B4-startup-state-corrupted":                  "internal/controller.TestStartupStateCorrupted",
	"U1-scratch-shell-show-hide":                  "internal/adapter/wm.TestScratchShellShowHideRestoresPriorFocus",
}

func TestSSOTRealOpsCoverageGate(t *testing.T) {
	if os.Getenv("PROJWM_REAL_OP_TESTS") != "1" {
		t.Skip("set PROJWM_REAL_OP_TESTS=1 to enforce SSOT §10.4 real operation coverage")
	}
	var missing, extra []string
	required := map[string]bool{}
	for _, op := range ssotRealOps {
		required[op] = true
		if ssotRealOpsImplemented[op] == "" {
			missing = append(missing, op)
		}
	}
	for op := range ssotRealOpsImplemented {
		if !required[op] {
			extra = append(extra, op)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("SSOT §10.4 real operation coverage mismatch: implemented=%d required=%d missing=%v extra=%v", len(ssotRealOpsImplemented), len(ssotRealOps), missing, extra)
	}
}

func TestSSOTRealOpsCoverageExcludesL2HarnessContracts(t *testing.T) {
	for _, op := range ssotL2HarnessOps {
		if owner := ssotRealOpsImplemented[op]; owner != "" {
			t.Fatalf("%s is an L2 deterministic harness contract but is registered as L3 real_ops owner %s", op, owner)
		}
	}
}

func TestSSOTRealOpsCoverageReferencesExistingTestFunctions(t *testing.T) {
	funcs := map[string]bool{}
	for pkg, rel := range map[string]string{
		"internal/adapter/wm":      "../adapter/wm",
		"internal/adapter/session": "../adapter/session",
		"internal/naming":          "../naming",
		"internal/controller":      "../controller",
	} {
		for _, name := range testFunctionsInDir(t, rel) {
			funcs[pkg+"."+name] = true
		}
	}
	for op, owner := range ssotRealOpsImplemented {
		if !funcs[owner] {
			t.Fatalf("%s references missing test function %s", op, owner)
		}
	}
}

func testFunctionsInDir(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		t.Fatalf("glob %s: %v", dir, err)
	}
	var out []string
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
			out = append(out, fn.Name.Name)
		}
	}
	return out
}
