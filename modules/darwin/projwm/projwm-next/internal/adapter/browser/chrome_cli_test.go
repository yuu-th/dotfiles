package browser

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseChromeCLIOutput(t *testing.T) {
	windows := parseChromeCLIWindowIDs("Window 1:\n  [10] https://example.test Example\nWindow 22\n")
	if len(windows) != 2 || windows[0] != "1" || windows[1] != "22" {
		t.Fatalf("windows = %#v", windows)
	}
	tabs := parseChromeCLITabs("  [10] https://browser-fixture.test/a Title A\n[11] about:blank\n")
	if len(tabs) != 2 || tabs[0].ID != "10" || tabs[0].URL != "https://browser-fixture.test/a" || tabs[0].Title != "Title A" {
		t.Fatalf("tabs = %#v", tabs)
	}
}

func TestChromeCLIAdapterObserveWindowsStoresPrivatePayloadBehindOpaqueToken(t *testing.T) {
	const canary = "SHOULD_NOT_APPEAR"
	privateStore, err := NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	exec := newFakeCLI(map[string][][]byte{
		key("list", "windows"): {
			[]byte("Window 7:\n"),
		},
		key("list", "tabs", "-w", "7"): {
			[]byte("[100] https://browser-fixture.test/" + canary + " Secret title\n[101] https://example.test/ok OK\n"),
		},
	})
	adapter := NewChromeCLIAdapter(exec, privateStore)
	adapter.ObserveContent = true
	got, err := adapter.ObserveWindows(context.Background())
	if err != nil {
		t.Fatalf("ObserveWindows: %v", err)
	}
	if len(got) != 1 || got[0].BrowserWindowID != "7" || got[0].PayloadToken == "" || got[0].TabCount != 2 {
		t.Fatalf("snapshots = %+v", got)
	}
	rawSnapshot, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if strings.Contains(string(rawSnapshot), canary) || strings.Contains(string(rawSnapshot), "browser-fixture.test") {
		t.Fatalf("window snapshot leaked raw browser content: %s", rawSnapshot)
	}
	payload, err := privateStore.Get(context.Background(), got[0].PayloadToken)
	if err != nil {
		t.Fatalf("Get payload: %v", err)
	}
	if len(payload.URLs) != 2 || !strings.Contains(payload.URLs[0], canary) {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestChromeCLIAdapterObserveWindowsDefaultsToStructureOnly(t *testing.T) {
	exec := newFakeCLI(map[string][][]byte{
		key("list", "windows"): {
			[]byte("Window 7:\n"),
		},
	})
	adapter := NewChromeCLIAdapter(exec, nil)
	got, err := adapter.ObserveWindows(context.Background())
	if err != nil {
		t.Fatalf("ObserveWindows: %v", err)
	}
	if len(got) != 1 || got[0].PayloadToken != "" || got[0].TabCount != 0 {
		t.Fatalf("snapshots = %+v", got)
	}
	exec.mustNotHaveCall(t, "list", "tabs", "-w", "7")
}

func TestChromeCLIAdapterOpenInProfileReturnsNewWindowFromPrivatePayload(t *testing.T) {
	privateStore, err := NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	token, err := privateStore.Put(context.Background(), PrivatePayload{URLs: []string{
		"https://browser-fixture.test/one",
		"https://browser-fixture.test/two",
	}})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	exec := newFakeCLI(map[string][][]byte{
		key("list", "windows"): {
			[]byte("Window 1:\n"),
			[]byte("Window 1:\nWindow 2:\n"),
		},
		key("open", "https://browser-fixture.test/one", "-n"): {
			[]byte(""),
		},
		key("open", "https://browser-fixture.test/two", "-w", "2"): {
			[]byte(""),
		},
	})
	adapter := NewChromeCLIAdapter(exec, privateStore)
	adapter.SettleTimeout = 10 * time.Millisecond
	result, err := adapter.OpenInProfile(context.Background(), "default", token)
	if err != nil {
		t.Fatalf("OpenInProfile: %v", err)
	}
	if result.BrowserWindowID != "2" {
		t.Fatalf("browser window id = %q, want 2", result.BrowserWindowID)
	}
	exec.mustHaveCall(t, "open", "https://browser-fixture.test/one", "-n")
	exec.mustHaveCall(t, "open", "https://browser-fixture.test/two", "-w", "2")
}

func TestChromeCLIAdapterRedactsExecutorErrors(t *testing.T) {
	privateStore, err := NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	const canary = "https://SHOULD_NOT_APPEAR.example/private"
	token, err := privateStore.Put(context.Background(), PrivatePayload{URLs: []string{canary}})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	adapter := NewChromeCLIAdapter(erroringCLI{err: errLeakingPrivateData{msg: "failed " + canary + " token " + token}}, privateStore)

	_, err = adapter.OpenInProfile(context.Background(), "default", token)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if strings.Contains(msg, canary) || strings.Contains(msg, token) {
		t.Fatalf("executor error leaked private data: %s", msg)
	}
}

type fakeCLI struct {
	outputs map[string][][]byte
	calls   [][]string
}

func newFakeCLI(outputs map[string][][]byte) *fakeCLI {
	return &fakeCLI{outputs: outputs}
}

func (f *fakeCLI) Run(_ context.Context, args ...string) ([]byte, error) {
	copied := append([]string(nil), args...)
	f.calls = append(f.calls, copied)
	k := key(args...)
	queue := f.outputs[k]
	if len(queue) == 0 {
		return nil, nil
	}
	out := queue[0]
	f.outputs[k] = queue[1:]
	return out, nil
}

func (f *fakeCLI) mustHaveCall(t *testing.T, args ...string) {
	t.Helper()
	want := key(args...)
	for _, call := range f.calls {
		if key(call...) == want {
			return
		}
	}
	t.Fatalf("missing call %q in %#v", want, f.calls)
}

func (f *fakeCLI) mustNotHaveCall(t *testing.T, args ...string) {
	t.Helper()
	want := key(args...)
	for _, call := range f.calls {
		if key(call...) == want {
			t.Fatalf("unexpected call %q in %#v", want, f.calls)
		}
	}
}

func key(args ...string) string {
	return strings.Join(args, "\x00")
}

type erroringCLI struct {
	err error
}

func (e erroringCLI) Run(context.Context, ...string) ([]byte, error) {
	return nil, e.err
}

type errLeakingPrivateData struct {
	msg string
}

func (e errLeakingPrivateData) Error() string { return e.msg }
