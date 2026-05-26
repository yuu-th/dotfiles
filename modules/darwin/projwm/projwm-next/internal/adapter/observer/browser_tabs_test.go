package observer

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

type recordingSubmitter struct {
	mu sync.Mutex
	in []intent.Intent
}

func (r *recordingSubmitter) ApplyIntent(_ context.Context, i intent.Intent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.in = append(r.in, i)
	return nil
}

func (r *recordingSubmitter) intents() []intent.Intent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]intent.Intent(nil), r.in...)
}

type fakeWorld struct{ project w.ProjectID }

func (f fakeWorld) ActiveProject() (w.ProjectID, bool) {
	if f.project == "" {
		return "", false
	}
	return f.project, true
}

type fakeInspector struct {
	snapshots [][]WindowSnapshot
	calls     int
	err       error
}

func (f *fakeInspector) InspectTabsByWindow(_ context.Context) ([]WindowSnapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.calls >= len(f.snapshots) {
		return nil, nil
	}
	out := f.snapshots[f.calls]
	f.calls++
	return out, nil
}

func newSyncFor(t *testing.T, snapshots ...[]WindowSnapshot) (*BrowserTabsSync, *recordingSubmitter) {
	t.Helper()
	sub := &recordingSubmitter{}
	b := &BrowserTabsSync{
		Vivaldi:   &fakeInspector{snapshots: snapshots},
		Submitter: sub,
		World:     fakeWorld{project: "dotfiles"},
	}
	return b, sub
}

// SSOT §4.4 BR-TAB-OBS: the very first observation seeds the snapshot
// without emitting (the observer cannot tell "this was here all along"
// from "user just added"). Only subsequent diffs emit.
func TestBrowserTabsSync_FirstObservationDoesNotEmit(t *testing.T) {
	b, sub := newSyncFor(t,
		[]WindowSnapshot{{Title: "browser-1:dotfiles", URLs: []string{"https://a", "https://b"}}},
	)
	b.pollOnce(context.Background())
	if got := sub.intents(); len(got) != 0 {
		t.Errorf("first poll emitted %v, want []", got)
	}
}

// User adds a tab inside Vivaldi.
func TestBrowserTabsSync_EmitsBrowserAddTabOnAppendedURL(t *testing.T) {
	b, sub := newSyncFor(t,
		[]WindowSnapshot{{Title: "browser-1:dotfiles", URLs: []string{"https://a", "https://b"}}},
		[]WindowSnapshot{{Title: "browser-1:dotfiles", URLs: []string{"https://a", "https://b", "https://c"}}},
	)
	b.pollOnce(context.Background()) // seed
	b.pollOnce(context.Background()) // diff

	got := sub.intents()
	if len(got) != 1 {
		t.Fatalf("emit count = %d, want 1: %#v", len(got), got)
	}
	add, ok := got[0].(intent.BrowserAddTab)
	if !ok {
		t.Fatalf("intent type = %T, want BrowserAddTab", got[0])
	}
	if add.URL != "https://c" || add.Project != "dotfiles" || add.WindowID.Index != 1 || add.WindowID.Kind != w.WindowBrowser {
		t.Errorf("AddTab fields wrong: %+v", add)
	}
}

// User closes a middle tab.
func TestBrowserTabsSync_EmitsBrowserRemoveTabAtPosition(t *testing.T) {
	b, sub := newSyncFor(t,
		[]WindowSnapshot{{Title: "browser-1:dotfiles", URLs: []string{"https://a", "https://b", "https://c"}}},
		[]WindowSnapshot{{Title: "browser-1:dotfiles", URLs: []string{"https://a", "https://c"}}},
	)
	b.pollOnce(context.Background())
	b.pollOnce(context.Background())

	got := sub.intents()
	if len(got) != 1 {
		t.Fatalf("emit count = %d, want 1", len(got))
	}
	rm, ok := got[0].(intent.BrowserRemoveTab)
	if !ok {
		t.Fatalf("intent type = %T, want BrowserRemoveTab", got[0])
	}
	if rm.Tab != 2 || rm.Project != "dotfiles" {
		t.Errorf("RemoveTab = %+v, want Tab=2 Project=dotfiles", rm)
	}
}

// User edits URL of an existing tab.
func TestBrowserTabsSync_EmitsBrowserChangeTabURLOnURLEdit(t *testing.T) {
	b, sub := newSyncFor(t,
		[]WindowSnapshot{{Title: "browser-1:dotfiles", URLs: []string{"https://old", "https://stable"}}},
		[]WindowSnapshot{{Title: "browser-1:dotfiles", URLs: []string{"https://new", "https://stable"}}},
	)
	b.pollOnce(context.Background())
	b.pollOnce(context.Background())

	got := sub.intents()
	if len(got) != 1 {
		t.Fatalf("emit count = %d, want 1: %#v", len(got), got)
	}
	ch, ok := got[0].(intent.BrowserChangeTabURL)
	if !ok {
		t.Fatalf("intent type = %T, want BrowserChangeTabURL", got[0])
	}
	if ch.Tab != 1 || ch.URL != "https://new" {
		t.Errorf("ChangeTabURL = %+v, want Tab=1 URL=https://new", ch)
	}
}

// SSOT §4.4 BR-USERPROF-EXTERNAL: user-profile Vivaldi windows have
// arbitrary titles. Observer must skip them (not crash, not emit).
func TestBrowserTabsSync_SkipsUserProfileWindow(t *testing.T) {
	b, sub := newSyncFor(t,
		[]WindowSnapshot{
			{Title: "browser-1:dotfiles", URLs: []string{"https://a"}},
			{Title: "Random Personal Window", URLs: []string{"https://gmail.com"}},
		},
		[]WindowSnapshot{
			{Title: "browser-1:dotfiles", URLs: []string{"https://a", "https://b"}},
			{Title: "Random Personal Window", URLs: []string{"https://twitter.com"}},
		},
	)
	b.pollOnce(context.Background())
	b.pollOnce(context.Background())

	got := sub.intents()
	if len(got) != 1 {
		t.Fatalf("emit count = %d, want 1 (only the managed window): %#v", len(got), got)
	}
	add, ok := got[0].(intent.BrowserAddTab)
	if !ok || add.Project != "dotfiles" {
		t.Errorf("emitted intent was not the managed-window AddTab: %+v", got[0])
	}
}

// G6 resilience: InspectTabsByWindow error does not crash the loop.
func TestBrowserTabsSync_InspectErrorIsNonFatal(t *testing.T) {
	insp := &fakeInspector{err: errors.New("osascript timed out")}
	sub := &recordingSubmitter{}
	b := &BrowserTabsSync{
		Vivaldi:   insp,
		Submitter: sub,
		World:     fakeWorld{project: "dotfiles"},
	}
	// 3 consecutive failures should NOT panic, NOT emit.
	for i := 0; i < 3; i++ {
		b.pollOnce(context.Background())
	}
	if b.consecutiveErrors != 3 {
		t.Errorf("consecutiveErrors = %d, want 3", b.consecutiveErrors)
	}
	if got := sub.intents(); len(got) != 0 {
		t.Errorf("submitter received %v despite all-error inspector", got)
	}
}

// Recovery: consecutiveErrors resets when inspect succeeds.
func TestBrowserTabsSync_ResetsErrorCountOnRecovery(t *testing.T) {
	insp := &fakeInspector{
		err:       errors.New("temporarily down"),
		snapshots: nil,
	}
	sub := &recordingSubmitter{}
	b := &BrowserTabsSync{
		Vivaldi:   insp,
		Submitter: sub,
		World:     fakeWorld{project: "dotfiles"},
	}
	b.pollOnce(context.Background())
	b.pollOnce(context.Background())
	if b.consecutiveErrors != 2 {
		t.Fatalf("setup: consecutiveErrors = %d, want 2", b.consecutiveErrors)
	}
	// Switch to success path.
	insp.err = nil
	insp.snapshots = [][]WindowSnapshot{{{Title: "browser-1:dotfiles", URLs: []string{"https://a"}}}}
	b.pollOnce(context.Background())
	if b.consecutiveErrors != 0 {
		t.Errorf("consecutiveErrors = %d after recovery, want 0", b.consecutiveErrors)
	}
}

// Window that disappears from observation is purged from snapshots —
// so a re-spawned same-identity window starts fresh without emitting
// stale-diff intents.
func TestBrowserTabsSync_PurgesSnapshotForDisappearedWindow(t *testing.T) {
	b, sub := newSyncFor(t,
		[]WindowSnapshot{{Title: "browser-1:dotfiles", URLs: []string{"https://a"}}},
		[]WindowSnapshot{}, // window closed
		[]WindowSnapshot{{Title: "browser-1:dotfiles", URLs: []string{"https://different"}}},
	)
	b.pollOnce(context.Background()) // seed
	b.pollOnce(context.Background()) // window vanished
	if _, alive := b.snapshots[managedKey{Project: "dotfiles", Index: 1}]; alive {
		t.Errorf("snapshot for disappeared window was not purged")
	}
	b.pollOnce(context.Background()) // window re-appears: should NOT emit ChangeURL "a→different"
	if got := sub.intents(); len(got) != 0 {
		t.Errorf("re-spawned window emitted stale diff: %v", got)
	}
}

func TestParseBrowserTitle(t *testing.T) {
	cases := []struct {
		title   string
		wantKey managedKey
		ok      bool
	}{
		{"browser-1:dotfiles", managedKey{Project: "dotfiles", Index: 1}, true},
		{"browser-7:manaflow", managedKey{Project: "manaflow", Index: 7}, true},
		{"browser-0:project", managedKey{}, false}, // 0-based rejected (SSOT 1-based)
		{"browser-:project", managedKey{}, false},
		{"browser-1:", managedKey{}, false},
		{"ai-1:project", managedKey{}, false},
		{"some user window", managedKey{}, false},
	}
	for _, tc := range cases {
		got, ok := parseBrowserTitle(tc.title)
		if ok != tc.ok || (ok && got != tc.wantKey) {
			t.Errorf("parseBrowserTitle(%q) = %+v, %v; want %+v, %v", tc.title, got, ok, tc.wantKey, tc.ok)
		}
	}
}

// Pure diff function tests — independent of the observer struct.
func TestDiffTabs(t *testing.T) {
	cases := []struct {
		name string
		prev []string
		curr []string
		want []intent.Intent
	}{
		{
			name: "no change",
			prev: []string{"a", "b"},
			curr: []string{"a", "b"},
			want: nil,
		},
		{
			name: "append one",
			prev: []string{"a"},
			curr: []string{"a", "b"},
			want: []intent.Intent{intent.BrowserAddTab{URL: "b"}},
		},
		{
			name: "remove middle",
			prev: []string{"a", "b", "c"},
			curr: []string{"a", "c"},
			want: []intent.Intent{intent.BrowserRemoveTab{Tab: 2}},
		},
		{
			name: "remove last",
			prev: []string{"a", "b"},
			curr: []string{"a"},
			want: []intent.Intent{intent.BrowserRemoveTab{Tab: 2}},
		},
		{
			name: "change one url",
			prev: []string{"a", "b"},
			curr: []string{"a", "B"},
			want: []intent.Intent{intent.BrowserChangeTabURL{Tab: 2, URL: "B"}},
		},
		{
			name: "multi-step bulk skipped",
			prev: []string{"a", "b", "c"},
			curr: []string{"x"},
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := diffTabs(tc.prev, tc.curr)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("diffTabs = %#v, want %#v", got, tc.want)
			}
		})
	}
}
