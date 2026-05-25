package scenario

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAcceptanceOwnersResolveToRealTestFunctions(t *testing.T) {
	funcs := acceptanceTestFunctions(t, realAcceptancePath(t), redAcceptancePath(t))
	for _, req := range AcceptanceCoverageMatrix() {
		for _, owner := range testOwnerTokens(req.RealOwner + "/" + req.AuthorityOwner) {
			if _, ok := funcs[owner]; !ok {
				t.Fatalf("%s owner %s does not resolve to a real Test function", req.ID, owner)
			}
		}
	}
}

func TestCoveredRowsRequireGreenHumanE2EOwner(t *testing.T) {
	realPath := realAcceptancePath(t)
	redPath := redAcceptancePath(t)
	funcs := acceptanceTestFunctions(t, realPath, redPath)
	for _, req := range AcceptanceCoverageMatrix() {
		if req.RealStatus != CoverageCovered {
			continue
		}
		for _, owner := range testOwnerTokens(req.RealOwner) {
			if owner == "TestHumanE2EAcceptanceCoverageGate" {
				t.Fatalf("%s uses coverage gate as covered evidence", req.ID)
			}
			path, ok := funcs[owner]
			if !ok {
				t.Fatalf("%s covered owner %s does not exist", req.ID, owner)
			}
			if path != realPath {
				t.Fatalf("%s covered owner %s is not in green real acceptance file: %s", req.ID, owner, path)
			}
			fn := findFuncDecl(t, realPath, owner)
			if !hasCall(fn, "newHumanE2E") {
				t.Fatalf("%s covered owner %s must enter the real Human E2E harness with newHumanE2E", req.ID, owner)
			}
		}
	}
}

func TestAuthorityCoveredRowsRequireGreenHumanE2EProof(t *testing.T) {
	realPath := realAcceptancePath(t)
	redPath := redAcceptancePath(t)
	funcs := acceptanceTestFunctions(t, realPath, redPath)
	for _, req := range AcceptanceCoverageMatrix() {
		if req.AuthorityStatus != CoverageCovered {
			continue
		}
		owners := testOwnerTokens(req.AuthorityOwner)
		if len(owners) == 0 {
			t.Fatalf("%s final-authority covered row must name executable Test owners", req.ID)
		}
		for _, owner := range owners {
			if owner == "TestHumanE2EAcceptanceCoverageGate" {
				t.Fatalf("%s uses coverage gate as final-authority covered evidence", req.ID)
			}
			path, ok := funcs[owner]
			if !ok {
				t.Fatalf("%s final-authority owner %s does not exist", req.ID, owner)
			}
			if path != realPath {
				t.Fatalf("%s final-authority owner %s is still a red audit in %s", req.ID, owner, path)
			}
			fn := findFuncDecl(t, realPath, owner)
			if !hasCall(fn, "newHumanE2E") {
				t.Fatalf("%s final-authority owner %s must enter the real Human E2E harness with newHumanE2E", req.ID, owner)
			}
		}
	}
}

func TestFailAcceptanceAlwaysFatal(t *testing.T) {
	fn := findFuncDecl(t, realAcceptancePath(t), "failAcceptance")
	if !hasSelectorCall(fn, "t", "Fatal") && !hasSelectorCall(fn, "t", "Fatalf") {
		t.Fatal("failAcceptance must call t.Fatal or t.Fatalf")
	}
	for _, name := range []string{"Skip", "Skipf", "SkipNow", "Log", "Logf"} {
		if hasSelectorCall(fn, "t", name) {
			t.Fatalf("failAcceptance must not call t.%s", name)
		}
	}
}

func TestRedAcceptanceTestsNeverSkipAfterOptIn(t *testing.T) {
	path := redAcceptancePath(t)
	f := parseGoFile(t, path)
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !strings.HasPrefix(fn.Name.Name, "TestHumanE2E") {
			continue
		}
		for _, name := range []string{"Skip", "Skipf", "SkipNow"} {
			if hasSelectorCall(fn, "t", name) {
				t.Fatalf("%s must not call t.%s; red tests may skip only through requireHumanE2EOptIn", fn.Name.Name, name)
			}
		}
		if !hasCall(fn, "failAcceptance") {
			t.Fatalf("%s must call failAcceptance so opt-in red tests cannot silently pass", fn.Name.Name)
		}
	}
}

func TestHumanE2EProductionProvenanceSourceShape(t *testing.T) {
	src := readFile(t, realAcceptancePath(t))
	for _, forbidden := range []string{
		"--test-mode",
		"StoreKindTest",
		"BackendFake",
		"BackendSimulator",
		"PROJWM_NEXT_BACKEND=real",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("real acceptance source contains forbidden final-authority shortcut %q", forbidden)
		}
	}
	if strings.Contains(funcSource(t, realAcceptancePath(t), "startHumanDaemon"), "--desired-world") {
		t.Fatalf("startHumanDaemon must not inject DesiredWorld through projwmd startup")
	}
	for _, required := range []string{
		`"--managed-environment"`,
		`"--manifest-digest"`,
		`"--store-kind", "production"`,
		`"--socket-path"`,
		"projwmstoreBootstrap",
		"productionSocketPath",
		"initializeProductionStore",
		`strings.Contains(socket, "/tmp/")`,
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("real acceptance source missing production provenance guard %q", required)
		}
	}
}

func TestNoFakeSimulatorOrTestModeInProjwmdStartup(t *testing.T) {
	src := readFile(t, projwmdMainPath(t))
	for _, forbidden := range []string{
		"--test-mode",
		"NewMemoryStore",
		"OpenFileStore(context.Background()",
		"projwmd-default",
		"falling back to fake",
		"SetUseSimulator(true)",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("projwmd startup source contains forbidden final-authority shortcut %q", forbidden)
		}
	}
	for _, required := range []string{
		"OpenExistingFileStore",
		"fake backend is not allowed for daemon startup",
		"--managed-environment is required",
		"refusing test store",
		"refusing /tmp socket",
		"NewFilePrivatePayloadStore",
		"NewVivaldiAdapter",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("projwmd startup source missing production guard %q", required)
		}
	}
}

func TestBrowserRestoreHasNoBlankWindowFallback(t *testing.T) {
	src := funcSource(t, wmAppContractPath(t), "spawnVivaldi")
	for _, forbidden := range []string{
		"about:blank",
		"Launcher.Launch(ctx, appPath, bundleID",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("browser restore path contains forbidden fallback %q", forbidden)
		}
	}
	for _, required := range []string{
		"BrowserCapabilityAdapter is required",
		"private browser payload token is required",
		"automation-owned non-default profile",
		"OpenInProfile",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("browser restore path missing guard %q", required)
		}
	}
}

func TestCompletionCannotBeCoveredByCoverageGateItself(t *testing.T) {
	for _, req := range AcceptanceCoverageMatrix() {
		if !strings.HasPrefix(req.ID, "DONE.") && req.ID != "AUTH.7.1" {
			continue
		}
		if strings.Contains(req.AuthorityOwner, "TestHumanE2EAcceptanceCoverageGate") {
			t.Fatalf("%s final authority owner must not be the coverage gate itself", req.ID)
		}
		if req.AuthorityStatus == CoverageCovered && strings.Contains(req.Owner, "AcceptanceCoverageMatrix") {
			t.Fatalf("%s cannot become final-authority covered from matrix bookkeeping alone", req.ID)
		}
	}
}

func acceptanceTestFunctions(t *testing.T, paths ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, path := range paths {
		f := parseGoFile(t, path)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && strings.HasPrefix(fn.Name.Name, "Test") {
				out[fn.Name.Name] = path
			}
		}
	}
	return out
}

func testOwnerTokens(owner string) []string {
	raw := strings.Split(owner, "/")
	out := make([]string, 0, len(raw))
	for _, token := range raw {
		token = strings.TrimSpace(token)
		if strings.HasPrefix(token, "Test") {
			out = append(out, token)
		}
	}
	return out
}

func findFuncDecl(t *testing.T, path, name string) *ast.FuncDecl {
	t.Helper()
	f := parseGoFile(t, path)
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name {
			return fn
		}
	}
	t.Fatalf("function %s not found in %s", name, path)
	return nil
}

func funcSource(t *testing.T, path, name string) string {
	t.Helper()
	src := readFile(t, path)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name {
			continue
		}
		start := fset.Position(fn.Pos()).Offset
		end := fset.Position(fn.End()).Offset
		return src[start:end]
	}
	t.Fatalf("function %s not found in %s", name, path)
	return ""
}

func hasCall(fn *ast.FuncDecl, name string) bool {
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == name {
			found = true
			return false
		}
		return true
	})
	return found
}

func hasSelectorCall(fn *ast.FuncDecl, receiver, method string) bool {
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != method {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if ok && ident.Name == receiver {
			found = true
			return false
		}
		return true
	})
	return found
}

func parseGoFile(t *testing.T, path string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return f
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func realAcceptancePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(moduleRoot(t), "scenarios", "real_acceptance_test.go")
}

func redAcceptancePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(moduleRoot(t), "scenarios", "real_acceptance_red_test.go")
}

func projwmdMainPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(moduleRoot(t), "cmd", "projwmd", "main.go")
}

func wmAppContractPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(moduleRoot(t), "internal", "adapter", "wm", "appcontract.go")
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}
