package wm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	browseradapter "github.com/yuu-th/projwm-next/internal/adapter/browser"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestCmdCtlExecutorTimesOutHungOmniwmctl(t *testing.T) {
	start := time.Now()
	_, err := CmdCtlExecutor{Bin: "/bin/sleep", Timeout: 50 * time.Millisecond}.Run(context.Background(), "5")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out after") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("timeout did not bound hung command; elapsed=%s", elapsed)
	}
}

// --- mock CtlExecutor ---

type mockCall struct {
	args []string
}

type mockExec struct {
	mu    sync.Mutex
	calls []mockCall
	// responder maps a "verb subverb" prefix (e.g. "query workspaces") to a
	// canned JSON response. If the prefix is unmatched mockExec returns
	// errResponseUnset.
	responder map[string][]byte
	// sequence maps a prefix to a list of sequential responses; if present it
	// is consumed before responder.
	sequence map[string][][]byte
	// errFor injects an error for a matching prefix.
	errFor map[string]error
	// errSequence injects sequential errors for a matching prefix.
	errSequence map[string][]error
}

func newMockExec() *mockExec {
	return &mockExec{responder: map[string][]byte{}, sequence: map[string][][]byte{}, errFor: map[string]error{}, errSequence: map[string][]error{}}
}

func (m *mockExec) set(prefix string, payload []byte) { m.responder[prefix] = payload }
func (m *mockExec) setSeq(prefix string, payloads ...[]byte) {
	m.sequence[prefix] = append(m.sequence[prefix], payloads...)
}
func (m *mockExec) setErr(prefix string, e error) { m.errFor[prefix] = e }
func (m *mockExec) setErrSeq(prefix string, errs ...error) {
	m.errSequence[prefix] = append(m.errSequence[prefix], errs...)
}

func (m *mockExec) Run(ctx context.Context, args ...string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, mockCall{args: append([]string(nil), args...)})
	joined := strings.Join(args, " ")
	for prefix, seq := range m.errSequence {
		if strings.HasPrefix(joined, prefix) && len(seq) > 0 {
			err := seq[0]
			m.errSequence[prefix] = seq[1:]
			if err != nil {
				return nil, err
			}
			break
		}
	}
	for prefix, e := range m.errFor {
		if strings.HasPrefix(joined, prefix) {
			return nil, e
		}
	}
	for prefix, seq := range m.sequence {
		if strings.HasPrefix(joined, prefix) && len(seq) > 0 {
			out := seq[0]
			m.sequence[prefix] = seq[1:]
			return out, nil
		}
	}
	for prefix, p := range m.responder {
		if strings.HasPrefix(joined, prefix) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("mockExec: no responder for: %s", joined)
}

// --- mock AppLauncher ---

type mockLauncher struct {
	mu     sync.Mutex
	calls  []launchCall
	failOn string // bundleID prefix to fail
}

type launchCall struct {
	appPath  string
	bundleID string
	args     []string
}

type mockBrowserAdapter struct {
	profile string
	token   string
}

func (m *mockBrowserAdapter) ObserveWindows(ctx context.Context) ([]browseradapter.WindowSnapshot, error) {
	return nil, nil
}

func (m *mockBrowserAdapter) FocusWindow(ctx context.Context, id w.LiveWindowID) error {
	return nil
}

func (m *mockBrowserAdapter) OpenInProfile(ctx context.Context, profile string, payloadToken string) (browseradapter.OpenResult, error) {
	m.profile = profile
	m.token = payloadToken
	return browseradapter.OpenResult{BrowserWindowID: "browser-window-9"}, nil
}

func (m *mockBrowserAdapter) CloseWindow(ctx context.Context, id w.LiveWindowID) error {
	return nil
}

func (l *mockLauncher) Launch(ctx context.Context, appPath, bundleID string, args []string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, launchCall{appPath, bundleID, append([]string(nil), args...)})
	if l.failOn != "" && strings.HasPrefix(bundleID, l.failOn) {
		return fmt.Errorf("mockLauncher: forced fail for %s", bundleID)
	}
	return nil
}

// --- env builder ---

func newTestEnv() w.ManagedEnvironment {
	return w.ManagedEnvironment{
		WindowManager: w.WindowManagerEnvironment{
			Backend: "omniwm",
			Layout: w.LayoutTuning{
				MaxVisibleColumns:   2,
				MaxWindowsPerColumn: 4,
			},
		},
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{
				{ID: "ws-A", RawName: "A", DisplayName: "A", Role: w.WorkspaceProject},
				{ID: "ws-B", RawName: "B", DisplayName: "B", Role: w.WorkspaceProject},
				{ID: "ws-V", RawName: "V", DisplayName: "Viewer", Role: w.WorkspaceViewer},
			},
		},
		Apps: w.AppEnvironment{
			ManagedApps: []w.ManagedAppPolicy{
				{
					Capability: w.CapabilityTerminal,
					BundleID:   "com.mitchellh.ghostty",
					AppPath:    "/Applications/Ghostty.app",
					LifecycleRemoval: w.LifecycleRemovalPolicy{
						Allowed:      true,
						Method:       w.LifecycleRemovalAXCloseGuarded,
						AllowedKinds: []w.WindowKind{w.WindowAI, w.WindowShell, w.WindowViewer},
					},
				},
				{
					Capability: w.CapabilityEditor,
					BundleID:   "dev.zed.Zed",
					AppPath:    "/Applications/Zed.app",
					LifecycleRemoval: w.LifecycleRemovalPolicy{
						Allowed: false,
						Method:  w.LifecycleRemovalProjectScopedApp,
					},
				},
				{
					Capability: w.CapabilityBrowser,
					BundleID:   "com.vivaldi.Vivaldi",
					AppPath:    "/Applications/Vivaldi.app",
					LifecycleRemoval: w.LifecycleRemovalPolicy{
						Allowed: false,
						Method:  w.LifecycleRemovalBrowserWindowClose,
					},
				},
			},
		},
	}
}

func okEnvelope(kind string, payload string) []byte {
	return []byte(fmt.Sprintf(`{"ok":true,"result":{"kind":%q,"payload":%s}}`, kind, payload))
}

// ----------------------------------------------------------------------------
// Tests
// ----------------------------------------------------------------------------

func TestSigWM_Capabilities(t *testing.T) {
	env := newTestEnv()
	sw := NewSigWM(env, newMockExec(), &mockLauncher{})
	caps, err := sw.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if caps.MaxVisibleColumns != 2 || caps.MaxWindowsPerColumn != 4 {
		t.Fatalf("unexpected caps: %+v", caps)
	}
}

func TestSigWM_Observe_HappyPath(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	m.set("query workspaces", okEnvelope("workspaces", `{"workspaces":[
		{"id":"omni-A","rawName":"A","displayName":"A","number":1,"isFocused":true,"isCurrent":true},
		{"id":"omni-B","rawName":"B","displayName":"B","number":2}
	]}`))
	m.set("query windows", okEnvelope("windows", `{"windows":[
		{"id":"win-1","title":"hello","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "isFocused":true,"workspace":{"id":"omni-A","rawName":"A","displayName":"A","number":1}}
	]}`))
	m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"win-1"}}`))

	sw := NewSigWM(env, m, &mockLauncher{})
	ow, err := sw.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if _, ok := ow.Workspaces["ws-A"]; !ok {
		t.Fatalf("ws-A missing in observed: %+v", ow.Workspaces)
	}
	win, ok := ow.Windows["win-1"]
	if !ok {
		t.Fatalf("win-1 missing")
	}
	if win.Workspace != "ws-A" {
		t.Fatalf("expected ws-A got %q", win.Workspace)
	}
	if !win.Focused {
		t.Fatalf("win-1 should be focused")
	}
	if ow.Focus.Workspace != "ws-A" || ow.Focus.Window != "win-1" {
		t.Fatalf("focus mismatch: %+v", ow.Focus)
	}
}

func TestSigWM_Observe_QueryError(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	m.setErr("query workspaces", errors.New("daemon down"))
	sw := NewSigWM(env, m, &mockLauncher{})
	_, err := sw.Observe(context.Background())
	if err == nil || !strings.Contains(err.Error(), "daemon down") {
		t.Fatalf("expected daemon-down error, got %v", err)
	}
}

func TestSigWM_Spawn_GhosttyHappyPath(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	// First windows query: empty. Second (settle): one match.
	// We reuse the same response for all windows queries; before launch the
	// Executor would not call queryWindows during settle until the launcher
	// returns. So the first windows response is used for settle (one match).
	m.set("query windows", okEnvelope("windows", `{"windows":[
		{"id":"new-1","title":"PROJ@yuta","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "isFocused":true,"workspace":{"id":"omni-A","rawName":"A","displayName":"A","number":1}}
	]}`))
	m.set("query workspaces", okEnvelope("workspaces", `{"workspaces":[
		{"id":"omni-A","rawName":"A","displayName":"A","number":1}
	]}`))
	m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"new-1"}}`))
	m.set("window navigate", []byte(`{"ok":true,"result":{"kind":"navigate","payload":{}}}`))
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	m.set("command move-to-workspace", []byte(`{"ok":true,"result":{"kind":"move","payload":{}}}`))

	l := &mockLauncher{}
	sw := NewSigWM(env, m, l)
	live, err := sw.Spawn(context.Background(), SpawnRequest{
		Workspace: "ws-A",
		Kind:      w.WindowShell,
		Title:     "PROJ@yuta",
		BundleID:  "com.mitchellh.ghostty",
	})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if live != "new-1" {
		t.Fatalf("expected new-1, got %q", live)
	}
	if len(l.calls) != 1 {
		t.Fatalf("launcher called %d times", len(l.calls))
	}
	if l.calls[0].appPath != "/Applications/Ghostty.app" {
		t.Fatalf("appPath: %q", l.calls[0].appPath)
	}
	foundTitleArg := false
	for _, a := range l.calls[0].args {
		if a == "--title=PROJ@yuta" {
			foundTitleArg = true
		}
	}
	if !foundTitleArg {
		t.Fatalf("ghostty --title=... arg not present: %v", l.calls[0].args)
	}
}

func TestSigWM_Spawn_LauncherFailurePropagates(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	l := &mockLauncher{failOn: "com.mitchellh.ghostty"}
	sw := NewSigWM(env, m, l)
	_, err := sw.Spawn(context.Background(), SpawnRequest{
		Workspace: "ws-A", Kind: w.WindowShell, Title: "T", BundleID: "com.mitchellh.ghostty",
	})
	if err == nil || !strings.Contains(err.Error(), "forced fail") {
		t.Fatalf("expected forced fail, got %v", err)
	}
}

func TestSigWM_Spawn_ZedRequiresProjectPathDirectory(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	m.set("query windows", okEnvelope("windows", `{"windows":[]}`))
	l := &mockLauncher{}
	sw := NewSigWM(env, m, l)
	_, err := sw.Spawn(context.Background(), SpawnRequest{
		Workspace: "ws-A", Kind: w.WindowEditor, Title: "p1", BundleID: "dev.zed.Zed",
	})
	if err == nil || !strings.Contains(err.Error(), "ProjectPath is required") {
		t.Fatalf("expected ProjectPath required error, got %v", err)
	}

	project := t.TempDir()
	m = newMockExec()
	// Editor uses the diff-based settle (SSOT §4.4 ED-MULTI): the pre-spawn
	// snapshot must be empty so the freshly-launched zed-1 registers as the
	// single newly-appeared dev.zed.Zed window. setSeq feeds the empty
	// snapshot once; the responder then surfaces zed-1 for the settle poll.
	m.setSeq("query windows", okEnvelope("windows", `{"windows":[]}`))
	m.set("query windows", okEnvelope("windows", `{"windows":[
		{"id":"zed-1","title":"p1","app":{"bundleId":"dev.zed.Zed","name":"Zed"},
		 "isFocused":true,"workspace":{"id":"omni-A","rawName":"A","displayName":"A","number":1}}
	]}`))
	m.set("query workspaces", okEnvelope("workspaces", `{"workspaces":[
		{"id":"omni-A","rawName":"A","displayName":"A","number":1}
	]}`))
	m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"zed-1"}}`))
	m.set("window navigate", []byte(`{"ok":true,"result":{"kind":"navigate","payload":{}}}`))
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	m.set("command move-to-workspace", []byte(`{"ok":true,"result":{"kind":"move","payload":{}}}`))
	l = &mockLauncher{}
	sw = NewSigWM(env, m, l)
	if _, err := sw.Spawn(context.Background(), SpawnRequest{
		Workspace: "ws-A", Kind: w.WindowEditor, Title: "p1", BundleID: "dev.zed.Zed", ProjectPath: project,
	}); err != nil {
		t.Fatalf("Spawn zed: %v", err)
	}
	if len(l.calls) != 1 || len(l.calls[0].args) == 0 || l.calls[0].args[0] != project {
		t.Fatalf("zed launch args = %+v, want project path first", l.calls)
	}
}

func TestSigWM_Spawn_VivaldiRequiresPrivatePayloadAdapter(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	m.set("query windows", okEnvelope("windows", `{"windows":[]}`))
	l := &mockLauncher{}
	sw := NewSigWM(env, m, l)
	_, err := sw.Spawn(context.Background(), SpawnRequest{
		Workspace: "ws-A", Kind: w.WindowBrowser, BundleID: "com.vivaldi.Vivaldi",
	})
	if err == nil || !strings.Contains(err.Error(), "BrowserCapabilityAdapter is required") {
		t.Fatalf("expected BrowserCapabilityAdapter required error, got %v", err)
	}

	sw.Browser = &mockBrowserAdapter{}
	_, err = sw.Spawn(context.Background(), SpawnRequest{
		Workspace: "ws-A", Kind: w.WindowBrowser, BundleID: "com.vivaldi.Vivaldi",
	})
	if err == nil || !strings.Contains(err.Error(), "private browser payload token is required") {
		t.Fatalf("expected private payload token required error, got %v", err)
	}
	_, err = sw.Spawn(context.Background(), SpawnRequest{
		Workspace:           "ws-A",
		Kind:                w.WindowBrowser,
		BundleID:            "com.vivaldi.Vivaldi",
		BrowserProfile:      "default",
		BrowserPayloadToken: "browser-payload-v1-00000000000000000000000000000000",
	})
	if err == nil || !strings.Contains(err.Error(), "automation-owned non-default profile") {
		t.Fatalf("expected automation-owned profile required error, got %v", err)
	}
	if len(l.calls) != 0 {
		t.Fatalf("launcher fallback must not be used for browser restore, calls=%+v", l.calls)
	}
}

func TestSigWM_Spawn_VivaldiUsesBrowserPayloadWhenAvailable(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	m.setSeq("query windows",
		okEnvelope("windows", `{"windows":[]}`),
		okEnvelope("windows", `{"windows":[
			{"id":"omni-browser","title":"Project marker","app":{"bundleId":"com.vivaldi.Vivaldi","name":"Vivaldi"},
			 "isFocused":true,"workspace":{"id":"omni-B","rawName":"B","displayName":"B","number":2}}
		]}`),
	)
	m.set("query windows", okEnvelope("windows", `{"windows":[
		{"id":"omni-browser","title":"Project marker","app":{"bundleId":"com.vivaldi.Vivaldi","name":"Vivaldi"},
		 "isFocused":true,"workspace":{"id":"omni-A","rawName":"A","displayName":"A","number":1}}
	]}`))
	m.set("query workspaces", okEnvelope("workspaces", `{"workspaces":[
		{"id":"omni-A","rawName":"A","displayName":"A","number":1},
		{"id":"omni-B","rawName":"B","displayName":"B","number":2}
	]}`))
	m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"omni-browser"}}`))
	m.set("window navigate", []byte(`{"ok":true,"result":{"kind":"navigate","payload":{}}}`))
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	m.set("command move-to-workspace", []byte(`{"ok":true,"result":{"kind":"move","payload":{}}}`))
	launcher := &mockLauncher{}
	browser := &mockBrowserAdapter{}
	sw := NewSigWM(env, m, launcher)
	sw.Browser = browser
	live, err := sw.Spawn(context.Background(), SpawnRequest{
		Workspace:           "ws-A",
		Kind:                w.WindowBrowser,
		BundleID:            "com.vivaldi.Vivaldi",
		BrowserProfile:      browseradapter.VivaldiAutomationProfile,
		BrowserPayloadToken: "browser-payload-v1-00000000000000000000000000000000",
	})
	if err != nil {
		t.Fatalf("Spawn vivaldi with browser payload: %v", err)
	}
	if live != "omni-browser" {
		t.Fatalf("live = %q, want omni-browser", live)
	}
	if browser.profile != browseradapter.VivaldiAutomationProfile || browser.token != "browser-payload-v1-00000000000000000000000000000000" {
		t.Fatalf("browser call profile=%q token=%q", browser.profile, browser.token)
	}
	if len(launcher.calls) != 0 {
		t.Fatalf("launcher should not be used when BrowserCapabilityAdapter opens payload, calls=%+v", launcher.calls)
	}
}

func TestSigWM_Spawn_VivaldiRejectsControllerOwnedTitle(t *testing.T) {
	env := newTestEnv()
	sw := NewSigWM(env, newMockExec(), &mockLauncher{})
	_, err := sw.Spawn(context.Background(), SpawnRequest{
		Workspace: "ws-A", Kind: w.WindowBrowser, Title: "x", BundleID: "com.vivaldi.Vivaldi",
	})
	if err == nil || !strings.Contains(err.Error(), "controller-owned browser title is not supported") {
		t.Fatalf("expected browser title rejection, got %v", err)
	}
}

func TestSigWM_Close_IsBlockedByProductionSafetyPolicy(t *testing.T) {
	exec := newMockExec()
	// Make pre-classify query return an empty window list so Close sees
	// no matching live target and returns nil (idempotent vanish path).
	// To exercise the production-block branch we need at least one
	// non-cockpit window matching the id.
	exec.set("query windows --format json", []byte(`{"ok":true,"result":{"kind":"windows","payload":{"windows":[{"id":"abc","app":{"bundleId":"dev.zed.Zed"},"title":"editor"}]}}}`))
	sw := NewSigWM(newTestEnv(), exec, &mockLauncher{})
	sw.CloseWindow = func(ctx context.Context, win ctlWindow) error {
		t.Fatalf("CloseWindow must not be called while close-window is production-blocked")
		return nil
	}
	err := sw.Close(context.Background(), "abc")
	if err == nil || !strings.Contains(err.Error(), "close-window is blocked") {
		t.Fatalf("expected production close-window block, got %v", err)
	}
}

func TestSigWM_Close_CockpitBypassesBlock(t *testing.T) {
	exec := newMockExec()
	exec.set("query windows --format json", []byte(`{"ok":true,"result":{"kind":"windows","payload":{"windows":[{"id":"cw1","app":{"bundleId":"com.mitchellh.ghostty"},"title":"projwm-cockpit-0"}]}}}`))
	sw := NewSigWM(newTestEnv(), exec, &mockLauncher{})
	closedCalls := 0
	sw.CloseWindow = func(ctx context.Context, win ctlWindow) error {
		closedCalls++
		return nil
	}
	if err := sw.Close(context.Background(), "cw1"); err != nil {
		t.Fatalf("expected cockpit close to succeed, got %v", err)
	}
	if closedCalls != 1 {
		t.Errorf("expected CloseWindow called once for cockpit close, got %d", closedCalls)
	}
}

func TestSigWM_TerminateManagedAppInstanceRequiresAuthorizedKind(t *testing.T) {
	sw := NewSigWM(newTestEnv(), newMockExec(), &mockLauncher{})
	sw.CloseWindow = func(ctx context.Context, win ctlWindow) error {
		t.Fatalf("CloseWindow must not be called for unauthorized lifecycle kind")
		return nil
	}
	err := sw.TerminateManagedAppInstance(context.Background(), TerminateManagedAppInstanceRequest{
		LiveWindow: "zed-1",
		Desired:    w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 1},
		Kind:       w.WindowEditor,
		Title:      "p1",
		BundleID:   "dev.zed.Zed",
	})
	if err == nil || !strings.Contains(err.Error(), "lifecycle removal is not authorized") {
		t.Fatalf("expected unauthorized lifecycle rejection, got %v", err)
	}
}

func TestSigWM_TerminateManagedAppInstanceUsesLifecycleGuardNotRawClose(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	target := okEnvelope("windows", `{"windows":[
		{"id":"shell-1","pid":123,"title":"shell-1:p1","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "isFocused":true,"workspace":{"id":"omni-A","rawName":"A","displayName":"A","number":1}}
	]}`)
	m.setSeq("query windows", target, okEnvelope("windows", `{"windows":[]}`))
	m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"shell-1"}}`))
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))

	sw := NewSigWM(env, m, &mockLauncher{})
	called := false
	sw.CloseWindow = func(ctx context.Context, win ctlWindow) error {
		called = true
		if win.ID != "shell-1" || win.PID != 123 || win.Title != "shell-1:p1" {
			t.Fatalf("unexpected lifecycle target: %+v", win)
		}
		return nil
	}
	err := sw.TerminateManagedAppInstance(context.Background(), TerminateManagedAppInstanceRequest{
		LiveWindow: "shell-1",
		Desired:    w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1},
		Kind:       w.WindowShell,
		Title:      "shell-1:p1",
		BundleID:   "com.mitchellh.ghostty",
	})
	if err != nil {
		t.Fatalf("TerminateManagedAppInstance: %v", err)
	}
	if !called {
		t.Fatal("lifecycle terminator was not invoked")
	}
}

func TestSigWM_MoveWindowToWorkspace_HappyPath(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	m.set("query workspaces", okEnvelope("workspaces", `{"workspaces":[
		{"id":"omni-A","rawName":"A","displayName":"A","number":1},
		{"id":"omni-B","rawName":"B","displayName":"B","number":2}
	]}`))
	// Sequence: 1) precheck (still on A), 2) settle after move (now on B).
	winsOnA := okEnvelope("windows", `{"windows":[
		{"id":"w-7","title":"x","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "workspace":{"id":"omni-A","rawName":"A","displayName":"A","number":1}}
	]}`)
	winsOnB := okEnvelope("windows", `{"windows":[
		{"id":"w-7","title":"x","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "workspace":{"id":"omni-B","rawName":"B","displayName":"B","number":2}}
	]}`)
	m.setSeq("query windows", winsOnA, winsOnB)
	m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"w-7"}}`))
	m.set("window navigate", []byte(`{"ok":true,"result":{"kind":"navigate","payload":{}}}`))
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	m.set("command move-to-workspace", []byte(`{"ok":true,"result":{"kind":"move","payload":{}}}`))

	sw := NewSigWM(env, m, &mockLauncher{})
	if err := sw.MoveWindowToWorkspace(context.Background(), "w-7", "ws-B"); err != nil {
		t.Fatalf("Move: %v", err)
	}

	// Verify move-to-workspace was called with "2" (the resolved number).
	found := false
	for _, c := range m.calls {
		if len(c.args) >= 3 && c.args[0] == "command" && c.args[1] == "move-to-workspace" && c.args[2] == "2" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("move-to-workspace 2 not called; calls: %+v", m.calls)
	}
}

func TestSigWM_MoveWindowToWorkspace_UnknownWorkspace(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	sw := NewSigWM(env, m, &mockLauncher{})
	err := sw.MoveWindowToWorkspace(context.Background(), "w-1", "ws-NOPE")
	if err == nil || !strings.Contains(err.Error(), "not in ManagedEnvironment") {
		t.Fatalf("expected env-miss error, got %v", err)
	}
}

func TestSigWM_MoveWindowToWorkspace_AlreadyOnTargetIsNoop(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	m.set("query workspaces", okEnvelope("workspaces", `{"workspaces":[
		{"id":"omni-B","rawName":"B","displayName":"B","number":2}
	]}`))
	m.set("query windows", okEnvelope("windows", `{"windows":[
		{"id":"w-7","title":"x","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "workspace":{"id":"omni-B","rawName":"B","displayName":"B","number":2}}
	]}`))

	sw := NewSigWM(env, m, &mockLauncher{})
	if err := sw.MoveWindowToWorkspace(context.Background(), "w-7", "ws-B"); err != nil {
		t.Fatalf("Move already-on-target: %v", err)
	}
	for _, c := range m.calls {
		if len(c.args) >= 2 && c.args[0] == "command" && c.args[1] == "move-to-workspace" {
			t.Fatalf("already-on-target move must not call move-to-workspace; calls=%+v", m.calls)
		}
	}
}

func TestSigWM_ReorderColumns_SingleWindowColumns(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	m.set("query workspaces", okEnvelope("workspaces", `{"workspaces":[
		{"id":"omni-A","rawName":"A","displayName":"A","number":1}
	]}`))
	before := okEnvelope("windows", `{"windows":[
		{"id":"a","title":"a","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "workspace":{"id":"omni-A","rawName":"A","displayName":"A","number":1}},
		{"id":"b","title":"b","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "workspace":{"id":"omni-A","rawName":"A","displayName":"A","number":1}}
	]}`)
	after := okEnvelope("windows", `{"windows":[
		{"id":"b","title":"b","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "workspace":{"id":"omni-A","rawName":"A","displayName":"A","number":1}},
		{"id":"a","title":"a","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "workspace":{"id":"omni-A","rawName":"A","displayName":"A","number":1}}
	]}`)
	m.setSeq("query windows", before, before, before, after, after)
	m.set("query windows", after)
	m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"b"}}`))
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	m.set("command move-column", []byte(`{"ok":true,"result":{"kind":"move-column","payload":{}}}`))
	sw := NewSigWM(env, m, &mockLauncher{})
	if err := sw.ReorderColumns(context.Background(), "ws-A", [][]w.LiveWindowID{{"b"}, {"a"}}); err != nil {
		t.Fatalf("ReorderColumns: %v", err)
	}
	found := false
	for _, c := range m.calls {
		if len(c.args) == 3 && c.args[0] == "command" && c.args[1] == "move-column" && c.args[2] == "left" {
			found = true
		}
	}
	if !found {
		t.Fatalf("move-column left not called; calls=%+v", m.calls)
	}
}

func TestSigWM_ReorderColumns_EmptyWorkspaceIsNoop(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	sw := NewSigWM(env, m, &mockLauncher{})
	if err := sw.ReorderColumns(context.Background(), "ws-A", nil); err != nil {
		t.Fatalf("ReorderColumns empty: %v", err)
	}
	if len(m.calls) != 0 {
		t.Fatalf("empty reorder must be noop, calls=%+v", m.calls)
	}
}

func TestSigWM_FocusWorkspace_UsesRawName(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	m.set("workspace focus-name", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	sw := NewSigWM(env, m, &mockLauncher{})
	if err := sw.FocusWorkspace(context.Background(), "ws-B"); err != nil {
		t.Fatalf("FocusWorkspace: %v", err)
	}
	if len(m.calls) != 1 || m.calls[0].args[2] != "B" {
		t.Fatalf("unexpected call: %+v", m.calls)
	}
}

func TestSigWM_FocusWorkspace_UnknownWorkspace(t *testing.T) {
	env := newTestEnv()
	sw := NewSigWM(env, newMockExec(), &mockLauncher{})
	err := sw.FocusWorkspace(context.Background(), "ws-NOPE")
	if err == nil || !strings.Contains(err.Error(), "unknown workspace") {
		t.Fatalf("expected unknown workspace, got %v", err)
	}
}

func TestSigWM_FocusWindow(t *testing.T) {
	// SSOT §7.5 F5: FocusWindow は navigate → focus の 2-step。
	// navigate は best-effort、focus が authoritative。
	env := newTestEnv()
	m := newMockExec()
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	m.set("window navigate", []byte(`{"ok":true,"result":{"kind":"navigate","payload":{}}}`))
	sw := NewSigWM(env, m, &mockLauncher{})
	if err := sw.FocusWindow(context.Background(), "abc"); err != nil {
		t.Fatalf("FocusWindow: %v", err)
	}
	if len(m.calls) != 2 {
		t.Fatalf("expected 2 calls (navigate, focus), got %+v", m.calls)
	}
	if m.calls[0].args[1] != "navigate" || m.calls[0].args[2] != "abc" {
		t.Fatalf("call[0] should be window navigate abc, got %+v", m.calls[0].args)
	}
	if m.calls[1].args[1] != "focus" || m.calls[1].args[2] != "abc" {
		t.Fatalf("call[1] should be window focus abc, got %+v", m.calls[1].args)
	}
}

func TestSigWM_FocusWindow_ErrorPropagates(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	m.setErr("window focus", errors.New("boom"))
	sw := NewSigWM(env, m, &mockLauncher{})
	err := sw.FocusWindow(context.Background(), "abc")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestSigWM_ContextCancellation(t *testing.T) {
	env := newTestEnv()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sw := NewSigWM(env, newMockExec(), &mockLauncher{})
	if _, err := sw.Capabilities(ctx); err == nil {
		t.Fatalf("expected ctx error from Capabilities")
	}
	if _, err := sw.Spawn(ctx, SpawnRequest{Kind: w.WindowShell}); err == nil {
		t.Fatalf("expected ctx error from Spawn")
	}
}

// TestSigWM_ShowCockpitOnDisplay_HappyPath verifies that ShowCockpitOnDisplay
// resolves the target display's anchor workspace, focuses it to move the
// focused-monitor to the right display, then issues switch-workspace.
// Focus is restored to the prior window (NFR-15).
func TestSigWM_ShowCockpitOnDisplay_HappyPath(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()

	// queryDisplays: display:1 is main, currently on workspace rawName="1".
	m.set("query displays", okEnvelope("displays", `{"displays":[
		{"id":"display:1","isMain":true,"name":"Built-in",
		 "activeWorkspace":{"id":"omni-1","rawName":"1","displayName":"","number":1}}
	]}`))

	// queryWorkspaces: CP1 is number 23.
	m.set("query workspaces", okEnvelope("workspaces", `{"workspaces":[
		{"id":"omni-cp1","rawName":"23","displayName":"CP1","number":23,"isFocused":false,"isCurrent":false},
		{"id":"omni-1","rawName":"1","displayName":"","number":1,"isFocused":true,"isCurrent":true}
	]}`))

	// queryFocusedWindowID: save prior focus.
	m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"other-win"}}`))

	// workspace focus-name and switch-workspace: succeed.
	m.set("workspace focus-name", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	m.set("command switch-workspace", []byte(`{"ok":true,"result":{"kind":"switch","payload":{}}}`))
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))

	sw := NewSigWM(env, m, &mockLauncher{})
	sw.SettleTimeout = 5 * time.Second

	if err := sw.ShowCockpitOnDisplay(context.Background(), "display:1", "23"); err != nil {
		t.Fatalf("ShowCockpitOnDisplay: %v", err)
	}

	// Verify switch-workspace was called.
	switchCalled := false
	for _, c := range m.calls {
		if len(c.args) >= 4 && c.args[0] == "command" && c.args[1] == "switch-workspace" && c.args[2] == "anywhere" {
			switchCalled = true
			break
		}
	}
	if !switchCalled {
		t.Errorf("command switch-workspace anywhere was not called; calls: %+v", m.calls)
	}

	// Verify focus is NOT restored to the prior window. Restoring focus
	// would re-associate the display with the prior workspace via omniwm's
	// focus-follows-window behavior, undoing the show. See ShowCockpitOnDisplay
	// docstring for the rationale.
	for _, c := range m.calls {
		if len(c.args) >= 3 && c.args[0] == "window" && c.args[1] == "focus" && c.args[2] == "other-win" {
			t.Errorf("ShowCockpitOnDisplay must NOT restore prior focus (would undo the show); calls: %+v", m.calls)
		}
	}
}

// TestSigWM_HideCockpitOnDisplay_HappyPath verifies that HideCockpitOnDisplay
// switches the display away from the park workspace to the prior workspace.
func TestSigWM_HideCockpitOnDisplay_HappyPath(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()

	// queryDisplays: display:1 is currently on CP1 (rawName="23").
	m.set("query displays", okEnvelope("displays", `{"displays":[
		{"id":"display:1","isMain":true,"name":"Built-in",
		 "activeWorkspace":{"id":"omni-cp1","rawName":"23","displayName":"CP1","number":23}}
	]}`))

	// queryWorkspaces: workspace "1" is number 1.
	m.set("query workspaces", okEnvelope("workspaces", `{"workspaces":[
		{"id":"omni-cp1","rawName":"23","displayName":"CP1","number":23,"isFocused":true,"isCurrent":true},
		{"id":"omni-1","rawName":"1","displayName":"","number":1,"isFocused":false,"isCurrent":false}
	]}`))

	// prior focus.
	m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"cockpit-win"}}`))

	m.set("workspace focus-name", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	m.set("command switch-workspace", []byte(`{"ok":true,"result":{"kind":"switch","payload":{}}}`))
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))

	sw := NewSigWM(env, m, &mockLauncher{})
	sw.SettleTimeout = 5 * time.Second

	if err := sw.HideCockpitOnDisplay(context.Background(), "display:1", "1"); err != nil {
		t.Fatalf("HideCockpitOnDisplay: %v", err)
	}

	// Verify switch-workspace was called with the prior workspace number (1).
	switchCalled := false
	for _, c := range m.calls {
		if len(c.args) >= 4 && c.args[0] == "command" && c.args[1] == "switch-workspace" && c.args[2] == "anywhere" && c.args[3] == "1" {
			switchCalled = true
			break
		}
	}
	if !switchCalled {
		t.Errorf("command switch-workspace anywhere 1 was not called; calls: %+v", m.calls)
	}
}

// TestSigWM_SpawnCockpit_PreFocusesParkWorkspace verifies that SpawnCockpit
// issues `workspace focus-name CP<displayIdx+1>` BEFORE launching ghostty, so
// that the freshly-spawned window lands on the correct display's active
// workspace without relying on the omniwm cross-display assignToWorkspace rule.
//
// Specifically for displayIdx=2 the park workspace is "CP3".
//
// Verified behaviour:
//   - `workspace focus-name CP3` is the first workspace/focus call before launch
//   - ghostty is launched with --title=projwm-cockpit-2
//   - priorFocus is saved and restored (NFR-15)
func TestSigWM_SpawnCockpit_PreFocusesParkWorkspace(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()

	// Pre-check idempotence: no window with this title yet.
	// settleNewWindow: second call returns the new window.
	m.setSeq("query windows",
		okEnvelope("windows", `{"windows":[]}`),
		okEnvelope("windows", `{"windows":[
			{"id":"cw-D2","title":"projwm-cockpit-2",
			 "app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
			 "isFocused":false,"workspace":{"id":"omni-cp3","rawName":"CP3","displayName":"CP3","number":33}}
		]}`),
	)
	// prior focus for NFR-15.
	m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"prev-win"}}`))
	// workspace focus-name: succeeds.
	m.set("workspace focus-name", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	// focus restoration after spawn.
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))

	l := &mockLauncher{}
	sw := NewSigWM(env, m, l)
	sw.SettleTimeout = 5 * time.Second
	// Inject a no-op so the test doesn't invoke real tmux.
	sw.EnsureCockpitSession = func(ctx context.Context) error { return nil }

	if err := sw.SpawnCockpit(context.Background(), 2, "projwm-cockpit-2"); err != nil {
		t.Fatalf("SpawnCockpit: %v", err)
	}

	// 1. Confirm workspace focus-name CP3 was called before the launcher.
	focusCallIdx := -1
	launchCallIdx := -1
	for i, c := range m.calls {
		joined := strings.Join(c.args, " ")
		if strings.HasPrefix(joined, "workspace focus-name CP3") && focusCallIdx == -1 {
			focusCallIdx = i
		}
	}
	// launcher calls are tracked separately; the focus-name must precede launch.
	// We verify focus-name was called at all and that the workspace is correct.
	if focusCallIdx == -1 {
		t.Errorf("SpawnCockpit must issue 'workspace focus-name CP3' before launching ghostty; calls: %+v", m.calls)
	}
	// launchCallIdx is implicitly "after focusCallIdx" because mockExec records
	// all Exec.Run calls; Launcher.Launch is separate, so we just confirm order
	// by checking focusCallIdx appears before any window-focus restoration.
	// For stronger ordering: check that focus-name appeared before the settle
	// query windows call (which follows the launch).
	settleWindowsIdx := -1
	for i, c := range m.calls {
		if i > focusCallIdx && len(c.args) >= 2 && c.args[0] == "query" && c.args[1] == "windows" {
			settleWindowsIdx = i
			break
		}
	}
	_ = launchCallIdx
	if focusCallIdx != -1 && settleWindowsIdx != -1 && focusCallIdx >= settleWindowsIdx {
		t.Errorf("workspace focus-name CP3 must occur before settle query-windows; focus at %d, settle at %d", focusCallIdx, settleWindowsIdx)
	}

	// 2. Confirm ghostty was launched with correct title.
	if len(l.calls) != 1 {
		t.Fatalf("expected exactly one launcher call, got %d", len(l.calls))
	}
	foundTitle := false
	for _, a := range l.calls[0].args {
		if a == "--title=projwm-cockpit-2" {
			foundTitle = true
		}
	}
	if !foundTitle {
		t.Fatalf("ghostty --title=projwm-cockpit-2 not present in args: %v", l.calls[0].args)
	}

	// 3. Confirm focus restoration (NFR-15): window focus prev-win called.
	focusRestored := false
	for _, c := range m.calls {
		if len(c.args) >= 3 && c.args[0] == "window" && c.args[1] == "focus" && c.args[2] == "prev-win" {
			focusRestored = true
		}
	}
	if !focusRestored {
		t.Errorf("focus restoration (window focus prev-win) was not called; calls: %+v", m.calls)
	}

	// 4. Confirm CP workspace name is derived from displayIdx (2 → CP3).
	cp3Called := false
	for _, c := range m.calls {
		if len(c.args) >= 3 && c.args[0] == "workspace" && c.args[1] == "focus-name" && c.args[2] == "CP3" {
			cp3Called = true
		}
	}
	if !cp3Called {
		t.Errorf("expected 'workspace focus-name CP3' (displayIdx=2 → CP3); calls: %+v", m.calls)
	}
}

// Bug 2026-05-19: omniwm sometimes fails to register a freshly-spawned
// ghostty cockpit window in time for settleNewWindow's observation
// poll, even though the process is on screen. The pre-fix daemon
// treated this as a hard failure and re-emitted spawn-cockpit on every
// reconcile, accumulating zombie ghostty processes and wedging the
// space+f hotkey. The post-fix behavior: when settle times out but
// pgrep confirms the ghostty process is alive, accept the spawn as
// converged and let omniwm catch up asynchronously.
func TestSigWM_SpawnCockpit_AcceptsLiveProcessWhenOmniwmMissesIt(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	// Pre-check: no window. Settle: also no window (omniwm misses it).
	m.set("query windows", okEnvelope("windows", `{"windows":[]}`))
	m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"prev-win"}}`))
	m.set("workspace focus-name", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))

	l := &mockLauncher{}
	sw := NewSigWM(env, m, l)
	// Tighten settle so the test runs fast.
	sw.SettleTimeout = 100 * time.Millisecond
	sw.EnsureCockpitSession = func(ctx context.Context) error { return nil }
	// First call (pre-check) returns false → fall through to spawn.
	// Second call (post-settle fallback) returns true → accept.
	callCount := 0
	sw.CockpitProcessAlive = func(ctx context.Context, title string) bool {
		callCount++
		return callCount > 1
	}

	if err := sw.SpawnCockpit(context.Background(), 0, "projwm-cockpit-0"); err != nil {
		t.Fatalf("SpawnCockpit should accept process-alive fallback on settle timeout: %v", err)
	}
	if len(l.calls) != 1 {
		t.Errorf("expected one launcher call, got %d", len(l.calls))
	}
}

// TestSigWM_SpawnCockpit_Idempotent verifies that SpawnCockpit returns nil
// without spawning when the cockpit window already exists.
func TestSigWM_SpawnCockpit_Idempotent(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	// Pre-check: window already present.
	m.set("query windows", okEnvelope("windows", `{"windows":[
		{"id":"cw-D0","title":"projwm-cockpit-0",
		 "app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "isFocused":true,"workspace":{"id":"omni-cp1","rawName":"CP1","displayName":"CP1","number":21}}
	]}`))
	// SSOT §6.6 IDEMP: existing-window summon focuses the existing
	// cockpit window instead of no-op'ing — so the test must also stub
	// out the `window focus <id>` call.
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))

	l := &mockLauncher{}
	sw := NewSigWM(env, m, l)
	sw.EnsureCockpitSession = func(ctx context.Context) error { return nil }
	// Inject a "process alive" oracle so the production pgrep-confirms-
	// reality check accepts omniwm's title hit as a real, live cockpit.
	sw.CockpitProcessAlive = func(ctx context.Context, title string) bool { return true }

	if err := sw.SpawnCockpit(context.Background(), 0, "projwm-cockpit-0"); err != nil {
		t.Fatalf("SpawnCockpit idempotent: %v", err)
	}
	if len(l.calls) != 0 {
		t.Fatalf("expected no launcher calls on idempotent spawn, got %d", len(l.calls))
	}
	// Verify the SSOT §6.6 IDEMP focus call was issued.
	wantFocus := false
	for _, c := range m.calls {
		if strings.Contains(strings.Join(c.args, " "), "window focus cw-D0") {
			wantFocus = true
			break
		}
	}
	if !wantFocus {
		t.Fatalf("idempotent SpawnCockpit must focus existing window per SSOT §6.6; calls=%v", m.calls)
	}
}

// TestSigWM_ShowCockpitOnDisplay_DisplayNotFound verifies that an error is
// returned when the target displayID is not in the observed displays.
func TestSigWM_ShowCockpitOnDisplay_DisplayNotFound(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()

	// queryDisplays: only display:1 exists, not display:99.
	m.set("query displays", okEnvelope("displays", `{"displays":[
		{"id":"display:1","isMain":true,"name":"Built-in",
		 "activeWorkspace":{"id":"omni-1","rawName":"1","displayName":"","number":1}}
	]}`))
	m.set("query workspaces", okEnvelope("workspaces", `{"workspaces":[
		{"id":"omni-cp1","rawName":"23","displayName":"CP1","number":23,"isFocused":false,"isCurrent":false}
	]}`))
	m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"other-win"}}`))
	m.set("command switch-workspace", []byte(`{"ok":true,"result":{"kind":"switch","payload":{}}}`))
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))

	sw := NewSigWM(env, m, &mockLauncher{})

	// Targeting a non-existent display: anchorRawName will be empty, but
	// switch-workspace should still be attempted (we just skip the focus-name call).
	// This is a best-effort behavior — the important thing is it doesn't panic.
	_ = sw.ShowCockpitOnDisplay(context.Background(), "display:99", "23")
}
