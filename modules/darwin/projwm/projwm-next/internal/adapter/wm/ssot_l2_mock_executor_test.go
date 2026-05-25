package wm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	browseradapter "github.com/yuu-th/projwm-next/internal/adapter/browser"
	zedadapter "github.com/yuu-th/projwm-next/internal/adapter/zed"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestSSOTL2MockExecutorTimeoutIsBounded(t *testing.T) {
	start := time.Now()
	_, err := CmdCtlExecutor{Bin: "/bin/sleep", Timeout: 50 * time.Millisecond}.Run(context.Background(), "5")
	if err == nil {
		t.Fatal("SSOT L2 requires timeout errors for hung wm executor calls")
	}
	if !strings.Contains(err.Error(), "timed out after") {
		t.Fatalf("timeout error = %v, want bounded timeout message", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("timeout was not bounded; elapsed=%s", elapsed)
	}
}

func TestSpawnSettleTimeoutProcessAlive(t *testing.T) {
	TestSigWM_SpawnCockpit_AcceptsLiveProcessWhenOmniwmMissesIt(t)
}

func TestSpawnSettleTimeoutProcessDead(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	m.set("query windows", okEnvelope("windows", `{"windows":[]}`))
	m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"prev-win"}}`))
	m.set("workspace focus-name", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	sw := NewSigWM(env, m, &mockLauncher{})
	sw.SettleTimeout = 25 * time.Millisecond
	sw.EnsureCockpitSession = func(ctx context.Context) error { return nil }
	sw.CockpitProcessAlive = func(ctx context.Context, title string) bool { return false }
	if err := sw.SpawnCockpit(context.Background(), 0, "projwm-cockpit-0"); err == nil {
		t.Fatal("SpawnCockpit succeeded on settle timeout with no live backing process")
	}
}

func TestMoveToWorkspaceFocusDrift(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	m.set("query workspaces", okEnvelope("workspaces", `{"workspaces":[
		{"id":"omni-A","rawName":"A","displayName":"A","number":1},
		{"id":"omni-B","rawName":"B","displayName":"B","number":2}
	]}`))
	winsOnA := okEnvelope("windows", `{"windows":[
		{"id":"w-drift","title":"drift","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "workspace":{"id":"omni-A","rawName":"A","displayName":"A","number":1}}
	]}`)
	winsOnB := okEnvelope("windows", `{"windows":[
		{"id":"w-drift","title":"drift","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "workspace":{"id":"omni-B","rawName":"B","displayName":"B","number":2}}
	]}`)
	m.setSeq("query windows", winsOnA, winsOnA, winsOnB)
	m.setSeq("query focused-window",
		okEnvelope("focused-window", `{"window":{"id":"w-drift"}}`),
		okEnvelope("focused-window", `{"window":{"id":"other-window"}}`),
		okEnvelope("focused-window", `{"window":{"id":"w-drift"}}`),
		okEnvelope("focused-window", `{"window":{"id":"w-drift"}}`),
	)
	m.set("window navigate", []byte(`{"ok":true,"result":{"kind":"navigate","payload":{}}}`))
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	m.set("command move-to-workspace", []byte(`{"ok":true,"result":{"kind":"move","payload":{}}}`))

	sw := NewSigWM(env, m, &mockLauncher{})
	if err := sw.MoveWindowToWorkspace(context.Background(), "w-drift", "ws-B"); err != nil {
		t.Fatalf("MoveWindowToWorkspace must retry after focus drift: %v", err)
	}
	if got := l2CountCommandPrefix(m, "window navigate"); got < 2 {
		t.Fatalf("focus drift must restart the navigate/focus chain, navigate calls=%d calls=%+v", got, m.calls)
	}
	if got := l2CountCommandPrefix(m, "command move-to-workspace"); got != 1 {
		t.Fatalf("focus drift must not mutate until re-verified focus; move calls=%d calls=%+v", got, m.calls)
	}
}

func TestMoveToWorkspaceRetry(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	m.set("query workspaces", okEnvelope("workspaces", `{"workspaces":[
		{"id":"omni-A","rawName":"A","displayName":"A","number":1},
		{"id":"omni-B","rawName":"B","displayName":"B","number":2}
	]}`))
	winsOnA := okEnvelope("windows", `{"windows":[
		{"id":"w-retry","title":"retry","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "workspace":{"id":"omni-A","rawName":"A","displayName":"A","number":1}}
	]}`)
	winsOnB := okEnvelope("windows", `{"windows":[
		{"id":"w-retry","title":"retry","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "workspace":{"id":"omni-B","rawName":"B","displayName":"B","number":2}}
	]}`)
	m.setSeq("query windows", winsOnA, winsOnA, winsOnA, winsOnB)
	m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"w-retry"}}`))
	m.set("window navigate", []byte(`{"ok":true,"result":{"kind":"navigate","payload":{}}}`))
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	m.setErrSeq("command move-to-workspace", errors.New("exit status 1"), errors.New("exit status 1"), nil)
	m.set("command move-to-workspace", []byte(`{"ok":true,"result":{"kind":"move","payload":{}}}`))

	sw := NewSigWM(env, m, &mockLauncher{})
	if err := sw.MoveWindowToWorkspace(context.Background(), "w-retry", "ws-B"); err != nil {
		t.Fatalf("MoveWindowToWorkspace must succeed on third transient failure attempt: %v", err)
	}
	if got := l2CountCommandPrefix(m, "command move-to-workspace"); got != 3 {
		t.Fatalf("move retry must attempt exactly three move-to-workspace calls, got=%d calls=%+v", got, m.calls)
	}
}

func TestMoveToWorkspaceWindowVanished(t *testing.T) {
	env := newTestEnv()
	m := newMockExec()
	m.set("query workspaces", okEnvelope("workspaces", `{"workspaces":[
		{"id":"omni-A","rawName":"A","displayName":"A","number":1},
		{"id":"omni-B","rawName":"B","displayName":"B","number":2}
	]}`))
	winsOnA := okEnvelope("windows", `{"windows":[
		{"id":"w-vanish","title":"vanish","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "workspace":{"id":"omni-A","rawName":"A","displayName":"A","number":1}}
	]}`)
	m.setSeq("query windows", winsOnA, okEnvelope("windows", `{"windows":[]}`))
	m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"w-vanish"}}`))
	m.set("window navigate", []byte(`{"ok":true,"result":{"kind":"navigate","payload":{}}}`))
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	m.setErr("command move-to-workspace", errors.New("exit status 1"))

	sw := NewSigWM(env, m, &mockLauncher{})
	err := sw.MoveWindowToWorkspace(context.Background(), "w-vanish", "ws-B")
	if err == nil || !strings.Contains(err.Error(), "vanished during retry") {
		t.Fatalf("MoveWindowToWorkspace must stop retrying when target vanishes, got %v", err)
	}
	if got := l2CountCommandPrefix(m, "command move-to-workspace"); got != 1 {
		t.Fatalf("vanished target must stop after first failed move, got move calls=%d calls=%+v", got, m.calls)
	}
}

func TestLifecycleRemovalFallbackCloseSurface(t *testing.T) {
	t.Run("ax-close-guarded/ghostty", func(t *testing.T) {
		logPath := l2InstallFakeOSAScript(t)
		m := newMockExec()
		targetPayload := okEnvelope("windows", `{"windows":[
			{"id":"ghostty-fallback","pid":1111,"title":"ghostty-fallback:test","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
			 "isFocused":true,"workspace":{"id":"omni-A","rawName":"A","displayName":"A","number":1}}
		]}`)
		emptyPayload := okEnvelope("windows", `{"windows":[]}`)
		m.setSeq("query windows", targetPayload, emptyPayload)
		m.set("query windows", emptyPayload)
		m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"ghostty-fallback"}}`))
		m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
		sw := NewSigWM(newTestEnv(), m, &mockLauncher{})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := sw.TerminateManagedAppInstance(ctx, TerminateManagedAppInstanceRequest{
			LiveWindow: "ghostty-fallback",
			Desired:    w.DesiredWindowID{Project: "projwm", Kind: w.WindowShell, Index: 1},
			Kind:       w.WindowShell,
			Title:      "ghostty-fallback:test",
			BundleID:   "com.mitchellh.ghostty",
		}); err != nil {
			t.Fatalf("ax-close-guarded fallback close surface: %v", err)
		}
		if got := l2CountCommandPrefix(m, "query windows"); got < 2 {
			t.Fatalf("ax-close-guarded must pre-observe target and post-observe disappearance, query windows calls=%d calls=%+v", got, m.calls)
		}
		entries := l2OSAScriptEntries(t, logPath)
		if len(entries) != 1 {
			t.Fatalf("ax-close-guarded osascript calls=%d, want 1\n%v", len(entries), entries)
		}
		l2AssertContainsAll(t, entries[0], "AXCloseButton", "AXPress", `keystroke "w" using command down`)
		l2AssertForbidden(t, entries[0], "quit")
	})

	t.Run("project-scoped-app/zed", func(t *testing.T) {
		logPath := l2InstallFakeOSAScript(t)
		live := w.LiveWindowID("zed-fallback")
		q := &l2ZedWindowSequence{windows: [][]zedadapter.OmniWMWindow{
			{{LiveWindow: live, PID: 2222, Title: "zed-fallback-project", BundleID: zedadapter.ZedBundleID}},
			nil,
		}}
		adapter := zedadapter.NewAdapter(q, zedadapter.CmdWindowCloser{}, nil)
		adapter.DisappearWait = 10 * time.Millisecond

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := adapter.CloseLiveWindow(ctx, live); err != nil {
			t.Fatalf("project-scoped-app fallback close surface: %v", err)
		}
		if q.calls < 2 {
			t.Fatalf("project-scoped-app must pre-observe target and post-observe disappearance, query calls=%d", q.calls)
		}
		entries := l2OSAScriptEntries(t, logPath)
		if len(entries) != 2 {
			t.Fatalf("project-scoped-app osascript calls=%d, want primary + fallback\n%v", len(entries), entries)
		}
		l2AssertContainsAll(t, entries[0], `tell application "Zed" to activate`, "close button")
		l2AssertContainsAll(t, entries[1], `tell application "Zed" to activate`, `keystroke "w" using {command down, shift down}`)
		l2AssertForbidden(t, entries[1], `keystroke "w" using command down`, "quit")
	})

	t.Run("browser-window-close/vivaldi", func(t *testing.T) {
		logPath := l2InstallFakeOSAScript(t)
		live := w.LiveWindowID("vivaldi-fallback")
		q := &l2VivaldiWindowSequence{windows: [][]browseradapter.VivaldiOmniWMWindow{
			{{LiveWindow: live, PID: 3333, Title: "vivaldi fallback - projwm-next", BundleID: browseradapter.VivaldiBundleID}},
			nil,
		}}
		adapter := browseradapter.NewVivaldiAdapterWithWM(nil, nil, "/Applications/Vivaldi.app", q, browseradapter.CmdVivaldiWindowCloser{})
		adapter.DisappearWait = 10 * time.Millisecond

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := adapter.CloseLiveWindow(ctx, live); err != nil {
			t.Fatalf("browser-window-close fallback close surface: %v", err)
		}
		if q.calls < 2 {
			t.Fatalf("browser-window-close must pre-observe target and post-observe disappearance, query calls=%d", q.calls)
		}
		entries := l2OSAScriptEntries(t, logPath)
		if len(entries) != 2 {
			t.Fatalf("browser-window-close osascript calls=%d, want primary + fallback\n%v", len(entries), entries)
		}
		l2AssertContainsAll(t, entries[0], `tell application "Vivaldi"`, "close wnd")
		l2AssertContainsAll(t, entries[1], "System Events", `keystroke "w" using {command down, shift down}`)
		l2AssertForbidden(t, entries[1], `keystroke "w" using command down`, "quit")
	})
}

func TestCloseWindowRetry(t *testing.T) {
	m := newMockExec()
	targetPayload := okEnvelope("windows", `{"windows":[
		{"id":"close-retry","pid":123,"title":"close-retry:p1","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"},
		 "isFocused":true,"workspace":{"id":"omni-A","rawName":"A","displayName":"A","number":1}}
	]}`)
	emptyPayload := okEnvelope("windows", `{"windows":[]}`)
	m.set("query windows", targetPayload)
	m.set("query focused-window", okEnvelope("focused-window", `{"window":{"id":"close-retry"}}`))
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	sw := NewSigWM(newTestEnv(), m, &mockLauncher{})
	sw.SettleTimeout = 15 * time.Second
	retryCloseCalls := 0
	target := ctlWindow{ID: "close-retry", PID: 123, Title: "close-retry:p1"}
	target.App.BundleID = "com.mitchellh.ghostty"
	target.App.Name = "Ghostty"
	sw.CloseWindow = func(ctx context.Context, win ctlWindow) error {
		retryCloseCalls++
		m.set("query windows", emptyPayload)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := sw.waitWindowGoneWithRetry(ctx, "close-retry", target, sw.CloseWindow); err != nil {
		t.Fatalf("close retry path must converge after second close: %v", err)
	}
	if retryCloseCalls != 1 {
		t.Fatalf("close retry must invoke one additional close after first wait-gone timeout, got=%d", retryCloseCalls)
	}
}

func TestFocusWindowNavigationBeforeFocus(t *testing.T) {
	m := newMockExec()
	m.set("window focus", []byte(`{"ok":true,"result":{"kind":"focus","payload":{}}}`))
	sw := NewSigWM(newTestEnv(), m, &mockLauncher{})
	if err := sw.FocusWindow(context.Background(), "focus-nav"); err != nil {
		t.Fatalf("FocusWindow: %v", err)
	}
	navigateIdx := l2FirstCommandPrefix(m, "window navigate")
	focusIdx := l2FirstCommandPrefix(m, "window focus")
	if navigateIdx < 0 || focusIdx < 0 || navigateIdx > focusIdx {
		t.Fatalf("FocusWindow must navigate before focus; navigateIdx=%d focusIdx=%d calls=%+v", navigateIdx, focusIdx, m.calls)
	}
}

// TestShowScratchShellExistingWindowReusesIt verifies SSOT §4.1 OP11 冪等性:
// omniwm が既に scratch window を持っているとき、ShowScratchShell は新規 spawn を
// せず navigate → focus だけで完結する。
func TestShowScratchShellExistingWindowReusesIt(t *testing.T) {
	m := newMockExec()
	m.set("query windows", okEnvelope("windows", `{"windows":[
		{"id":"scratch-omni-1","title":"projwm-scratch-shell","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"}}
	]}`))
	m.set("window navigate", okEnvelope("navigate", `{}`))
	m.set("window focus", okEnvelope("focus", `{}`))
	launcher := &mockLauncher{}
	sw := NewSigWM(newTestEnv(), m, launcher)

	id, err := sw.ShowScratchShell(context.Background())
	if err != nil {
		t.Fatalf("ShowScratchShell: %v", err)
	}
	if string(id) != "scratch-omni-1" {
		t.Fatalf("ShowScratchShell returned %q, want scratch-omni-1", id)
	}
	if got := len(launcher.calls); got != 0 {
		t.Fatalf("expected no launcher invocations for existing scratch, got %d", got)
	}
	navigateIdx := l2FirstCommandPrefix(m, "window navigate")
	focusIdx := l2FirstCommandPrefix(m, "window focus")
	if navigateIdx < 0 || focusIdx < 0 || navigateIdx > focusIdx {
		t.Fatalf("ShowScratchShell must navigate before focus; navigateIdx=%d focusIdx=%d", navigateIdx, focusIdx)
	}
}

// TestShowScratchShellSpawnsWhenAbsent verifies SSOT §4.1 OP11 新規生成:
// scratch window が存在しないとき、ShowScratchShell は tmux session ensure と
// Ghostty 起動 (Launcher.Launch) を呼ぶ。
func TestShowScratchShellSpawnsWhenAbsent(t *testing.T) {
	m := newMockExec()
	// First query: empty (no scratch). After spawn settle, scratch appears.
	m.setSeq("query windows",
		okEnvelope("windows", `{"windows":[]}`),
		okEnvelope("windows", `{"windows":[
			{"id":"scratch-omni-1","title":"projwm-scratch-shell","app":{"bundleId":"com.mitchellh.ghostty","name":"Ghostty"}}
		]}`),
	)
	m.set("window navigate", okEnvelope("navigate", `{}`))
	m.set("window focus", okEnvelope("focus", `{}`))
	launcher := &mockLauncher{}
	sw := NewSigWM(newTestEnv(), m, launcher)
	sw.EnsureScratchShellSession = func(ctx context.Context) error { return nil }

	id, err := sw.ShowScratchShell(context.Background())
	if err != nil {
		t.Fatalf("ShowScratchShell: %v", err)
	}
	if string(id) != "scratch-omni-1" {
		t.Fatalf("ShowScratchShell returned %q, want scratch-omni-1 after settle", id)
	}
	if len(launcher.calls) != 1 {
		t.Fatalf("expected 1 launcher call for fresh spawn, got %d", len(launcher.calls))
	}
	gotBundle := launcher.calls[0].bundleID
	if gotBundle != "com.mitchellh.ghostty" {
		t.Fatalf("launcher bundleID = %q, want com.mitchellh.ghostty", gotBundle)
	}
	// Launcher args should contain --title=projwm-scratch-shell
	found := false
	for _, a := range launcher.calls[0].args {
		if a == "--title=projwm-scratch-shell" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("launcher args missing --title=projwm-scratch-shell: %+v", launcher.calls[0].args)
	}
}

// TestHideScratchShellNavigatesAndFocusesPrior verifies SSOT §4.1 OP11:
// HideScratchShell issues navigate → focus on priorWindow.
func TestHideScratchShellNavigatesAndFocusesPrior(t *testing.T) {
	m := newMockExec()
	m.set("window navigate", okEnvelope("navigate", `{}`))
	m.set("window focus", okEnvelope("focus", `{}`))
	sw := NewSigWM(newTestEnv(), m, &mockLauncher{})

	if err := sw.HideScratchShell(context.Background(), "shell-omni-7"); err != nil {
		t.Fatalf("HideScratchShell: %v", err)
	}
	navigateIdx := l2FirstCommandPrefix(m, "window navigate shell-omni-7")
	focusIdx := l2FirstCommandPrefix(m, "window focus shell-omni-7")
	if navigateIdx < 0 || focusIdx < 0 || navigateIdx > focusIdx {
		t.Fatalf("HideScratchShell must navigate before focus; navigateIdx=%d focusIdx=%d calls=%+v", navigateIdx, focusIdx, m.calls)
	}
}

// TestMoveCockpitToParkWorkspaceCommandSequence verifies SSOT §3.4 INV-06:
// MoveCockpitToParkWorkspace は queryWorkspaces で park の numeric id を
// 解決し、cockpit window を focus してから `command move-to-workspace <num>`
// を発行する。
func TestMoveCockpitToParkWorkspaceCommandSequence(t *testing.T) {
	m := newMockExec()
	m.set("query workspaces", okEnvelope("workspaces", `{"workspaces":[
		{"id":"omni-A","rawName":"A","displayName":"A","number":1},
		{"id":"omni-CP1","rawName":"CP1","displayName":"CP1","number":11}
	]}`))
	m.set("window focus", okEnvelope("focus", `{}`))
	m.set("command move-to-workspace", okEnvelope("move", `{}`))
	sw := NewSigWM(newTestEnv(), m, &mockLauncher{})

	if err := sw.MoveCockpitToParkWorkspace(context.Background(), "cockpit-omni-1", "CP1"); err != nil {
		t.Fatalf("MoveCockpitToParkWorkspace: %v", err)
	}
	focusIdx := l2FirstCommandPrefix(m, "window focus cockpit-omni-1")
	moveIdx := l2FirstCommandPrefix(m, "command move-to-workspace 11")
	if focusIdx < 0 || moveIdx < 0 {
		t.Fatalf("expected focus then move; calls=%+v", m.calls)
	}
	if focusIdx > moveIdx {
		t.Fatalf("focus must precede move; focusIdx=%d moveIdx=%d", focusIdx, moveIdx)
	}
}

// TestMoveCockpitToParkWorkspaceUnknownParkErrors verifies the error path:
// park workspace name not present in omniwm → return error rather than silently
// no-op (otherwise INV-06 violation would persist).
func TestMoveCockpitToParkWorkspaceUnknownParkErrors(t *testing.T) {
	m := newMockExec()
	m.set("query workspaces", okEnvelope("workspaces", `{"workspaces":[
		{"id":"omni-A","rawName":"A","displayName":"A","number":1}
	]}`))
	sw := NewSigWM(newTestEnv(), m, &mockLauncher{})
	err := sw.MoveCockpitToParkWorkspace(context.Background(), "cockpit-omni-1", "CP-NONEXISTENT")
	if err == nil {
		t.Fatal("expected error for unknown park workspace, got nil")
	}
	if !strings.Contains(err.Error(), "workspace") {
		t.Fatalf("error should mention workspace lookup, got %v", err)
	}
}

// TestHideScratchShellEmptyPriorIsNoop verifies the safety branch: empty
// priorWindow → no commands issued (matches NFR-15 規約)。
func TestHideScratchShellEmptyPriorIsNoop(t *testing.T) {
	m := newMockExec()
	sw := NewSigWM(newTestEnv(), m, &mockLauncher{})

	if err := sw.HideScratchShell(context.Background(), ""); err != nil {
		t.Fatalf("HideScratchShell empty: %v", err)
	}
	if len(m.calls) != 0 {
		t.Fatalf("expected zero exec calls for empty prior, got %+v", m.calls)
	}
}

func l2CountCommandPrefix(m *mockExec, prefix string) int {
	count := 0
	for _, call := range m.calls {
		if strings.HasPrefix(strings.Join(call.args, " "), prefix) {
			count++
		}
	}
	return count
}

func l2FirstCommandPrefix(m *mockExec, prefix string) int {
	for i, call := range m.calls {
		if strings.HasPrefix(strings.Join(call.args, " "), prefix) {
			return i
		}
	}
	return -1
}

type l2ZedWindowSequence struct {
	windows [][]zedadapter.OmniWMWindow
	calls   int
}

func (q *l2ZedWindowSequence) QueryZedWindows(ctx context.Context) ([]zedadapter.OmniWMWindow, error) {
	q.calls++
	if len(q.windows) == 0 {
		return nil, nil
	}
	out := q.windows[0]
	q.windows = q.windows[1:]
	return out, nil
}

type l2VivaldiWindowSequence struct {
	windows [][]browseradapter.VivaldiOmniWMWindow
	calls   int
}

func (q *l2VivaldiWindowSequence) QueryVivaldiWindows(ctx context.Context) ([]browseradapter.VivaldiOmniWMWindow, error) {
	q.calls++
	if len(q.windows) == 0 {
		return nil, nil
	}
	out := q.windows[0]
	q.windows = q.windows[1:]
	return out, nil
}

func l2InstallFakeOSAScript(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "osascript.log")
	binPath := filepath.Join(dir, "osascript")
	script := fmt.Sprintf(`#!/bin/sh
log=%q
applescript="$2"
{
  printf -- '---CALL---\n'
  printf '%%s\n' "$applescript"
} >> "$log"

case "$applescript" in
  *'tell application "Zed" to activate'*'return "ax-close-unavailable"'*)
    printf 'ax-close-unavailable\n'
    exit 0
    ;;
  *'tell application "Vivaldi"'*)
    printf '0\n'
    exit 0
    ;;
  *'keystroke "w" using {command down, shift down}'*)
    exit 0
    ;;
  *'AXCloseButton'*'keystroke "w" using command down'*)
    printf 'ok-keystroke\n'
    exit 0
    ;;
esac

printf 'unexpected osascript payload\n' >&2
exit 17
`, logPath)
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake osascript: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func l2OSAScriptEntries(t *testing.T, logPath string) []string {
	t.Helper()
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake osascript log: %v", err)
	}
	raw := strings.Split(string(logBytes), "---CALL---")
	entries := make([]string, 0, len(raw))
	for _, entry := range raw {
		entry = strings.TrimSpace(entry)
		if entry != "" {
			entries = append(entries, entry)
		}
	}
	return entries
}

func l2AssertContainsAll(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("osascript payload missing %q\n%s", want, got)
		}
	}
}

func l2AssertForbidden(t *testing.T, got string, forbidden ...string) {
	t.Helper()
	for _, bad := range forbidden {
		if strings.Contains(got, bad) {
			t.Fatalf("osascript payload contains forbidden surface %q\n%s", bad, got)
		}
	}
}
