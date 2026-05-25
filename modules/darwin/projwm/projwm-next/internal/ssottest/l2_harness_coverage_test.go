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

var ssotL2HarnessImplemented = map[string]string{
	"S12-spawn-settle-timeout-process-alive":      "internal/adapter/wm.TestSpawnSettleTimeoutProcessAlive",
	"S13-spawn-settle-timeout-process-dead":       "internal/adapter/wm.TestSpawnSettleTimeoutProcessDead",
	"M3-move-to-workspace-focus-drift":            "internal/adapter/wm.TestMoveToWorkspaceFocusDrift",
	"M4-move-to-workspace-retry":                  "internal/adapter/wm.TestMoveToWorkspaceRetry",
	"M5-move-to-workspace-window-vanished":        "internal/adapter/wm.TestMoveToWorkspaceWindowVanished",
	"C2-lifecycle-removal-fallback-close-surface": "internal/adapter/wm.TestLifecycleRemovalFallbackCloseSurface",
	"C3-close-window-retry":                       "internal/adapter/wm.TestCloseWindowRetry",
	"F5-focus-window-navigation-before-focus":     "internal/adapter/wm.TestFocusWindowNavigationBeforeFocus",
}

func TestSSOTL2HarnessCoverageGate(t *testing.T) {
	var missing, extra []string
	required := map[string]bool{}
	for _, op := range ssotL2HarnessOps {
		required[op] = true
		if ssotL2HarnessImplemented[op] == "" {
			missing = append(missing, op)
		}
	}
	for op := range ssotL2HarnessImplemented {
		if !required[op] {
			extra = append(extra, op)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("SSOT §10.4 L2 harness coverage mismatch: implemented=%d required=%d missing=%v extra=%v", len(ssotL2HarnessImplemented), len(ssotL2HarnessOps), missing, extra)
	}
}

func TestSSOTL2HarnessCoverageReferencesExistingNonRealOpsTestFunctions(t *testing.T) {
	funcs := testFunctionsByPackage(t, map[string]string{
		"internal/adapter/wm": "../adapter/wm",
	})
	for op, owner := range ssotL2HarnessImplemented {
		info, ok := funcs[owner]
		if !ok {
			t.Fatalf("%s references missing L2 harness test function %s", op, owner)
		}
		if info.realOps {
			t.Fatalf("%s references %s in real_ops-gated file %s; L2 harness tests must run without real_ops", op, owner, info.path)
		}
	}
}

type testFunctionInfo struct {
	path    string
	realOps bool
}

func testFunctionsByPackage(t *testing.T, pkgs map[string]string) map[string]testFunctionInfo {
	t.Helper()
	funcs := map[string]testFunctionInfo{}
	for pkg, rel := range pkgs {
		matches, err := filepath.Glob(filepath.Join(rel, "*_test.go"))
		if err != nil {
			t.Fatalf("glob %s: %v", rel, err)
		}
		for _, path := range matches {
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			realOps := strings.Contains(string(src), "//go:build real_ops")
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") {
					continue
				}
				funcs[pkg+"."+fn.Name.Name] = testFunctionInfo{path: path, realOps: realOps}
			}
		}
	}
	return funcs
}
