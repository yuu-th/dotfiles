package browser

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	w "github.com/yuu-th/projwm-next/internal/world"
)

type fakeVivaldiQuerier struct {
	mu    sync.Mutex
	queue [][]VivaldiOmniWMWindow
	calls int
}

func (f *fakeVivaldiQuerier) QueryVivaldiWindows(ctx context.Context) ([]VivaldiOmniWMWindow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if len(f.queue) == 0 {
		return nil, nil
	}
	out := f.queue[0]
	f.queue = f.queue[1:]
	return out, nil
}

func (f *fakeVivaldiQuerier) push(wins []VivaldiOmniWMWindow) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queue = append(f.queue, wins)
}

type fakeVivaldiCloser struct {
	called bool
	last   VivaldiOmniWMWindow
	err    error
}

func (f *fakeVivaldiCloser) CloseVivaldiWindow(ctx context.Context, win VivaldiOmniWMWindow) error {
	f.called = true
	f.last = win
	return f.err
}

type recordingAppOpener struct {
	appPath string
	args    []string
	err     error
}

func (o *recordingAppOpener) Open(ctx context.Context, appPath string, args ...string) error {
	o.appPath = appPath
	o.args = append([]string(nil), args...)
	return o.err
}

func TestVivaldiAdapterOpenInProfileUsesPrivatePayload(t *testing.T) {
	ctx := context.Background()
	store, err := NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	token, err := store.Put(ctx, PrivatePayload{URLs: []string{"https://example.test/one", "https://example.test/two"}})
	if err != nil {
		t.Fatalf("put payload: %v", err)
	}
	opener := &recordingAppOpener{}
	adapter := NewVivaldiAdapter(store, opener, "/Applications/Vivaldi.app")

	if _, err := adapter.OpenInProfile(ctx, VivaldiAutomationProfile, token); err != nil {
		t.Fatalf("OpenInProfile: %v", err)
	}

	want := []string{"--new-window", "--profile-directory=" + VivaldiAutomationProfile, "https://example.test/one", "https://example.test/two"}
	if opener.appPath != "/Applications/Vivaldi.app" || !reflect.DeepEqual(opener.args, want) {
		t.Fatalf("open args = path=%q args=%v, want path=/Applications/Vivaldi.app args=%v", opener.appPath, opener.args, want)
	}
}

func TestVivaldiAdapterOpenErrorDoesNotExposePayloadToken(t *testing.T) {
	ctx := context.Background()
	store, err := NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	const secretURL = "https://SHOULD_NOT_APPEAR.example/private"
	token, err := store.Put(ctx, PrivatePayload{URLs: []string{secretURL}})
	if err != nil {
		t.Fatalf("put payload: %v", err)
	}
	adapter := NewVivaldiAdapter(store, &recordingAppOpener{err: errors.New("open failed for " + secretURL + " token " + token)}, "")

	_, err = adapter.OpenInProfile(ctx, VivaldiAutomationProfile, token)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if strings.Contains(msg, secretURL) || strings.Contains(msg, token) {
		t.Fatalf("error leaked private payload data: %s", msg)
	}
}

func TestVivaldiAdapterRejectsDefaultProfileAndMissingToken(t *testing.T) {
	ctx := context.Background()
	store, err := NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	token, err := store.Put(ctx, PrivatePayload{URLs: []string{"https://example.test/"}})
	if err != nil {
		t.Fatalf("put payload: %v", err)
	}
	adapter := NewVivaldiAdapter(store, &recordingAppOpener{}, "")
	for _, tc := range []struct {
		name    string
		profile string
		token   string
		want    string
	}{
		{name: "blank-profile", profile: "", token: token, want: "automation-owned non-default profile"},
		{name: "default-profile", profile: "default", token: token, want: "automation-owned non-default profile"},
		{name: "missing-token", profile: VivaldiAutomationProfile, token: "", want: "private payload token is required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := adapter.OpenInProfile(ctx, tc.profile, tc.token); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("OpenInProfile error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestVivaldiAdapterOpenInProfilePopulatesBrowserWindowIDFromOmniWMDiff(t *testing.T) {
	ctx := context.Background()
	store, err := NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	token, err := store.Put(ctx, PrivatePayload{URLs: []string{"https://example.test/one"}})
	if err != nil {
		t.Fatalf("put payload: %v", err)
	}
	querier := &fakeVivaldiQuerier{}
	// Pre-Open snapshot: one existing Vivaldi window for an unrelated profile.
	querier.push([]VivaldiOmniWMWindow{
		{LiveWindow: "vw-existing", PID: 100, Title: "Other - other-profile", BundleID: VivaldiBundleID},
	})
	// Post-Open snapshot: existing + the new automation-profile window.
	querier.push([]VivaldiOmniWMWindow{
		{LiveWindow: "vw-existing", PID: 100, Title: "Other - other-profile", BundleID: VivaldiBundleID},
		{LiveWindow: "vw-new", PID: 200, Title: "https://example.test/one - " + VivaldiAutomationProfile, BundleID: VivaldiBundleID},
	})
	opener := &recordingAppOpener{}
	adapter := NewVivaldiAdapterWithWM(store, opener, "/Applications/Vivaldi.app", querier, &fakeVivaldiCloser{})
	adapter.SettleTimeout = 200 * time.Millisecond

	res, err := adapter.OpenInProfile(ctx, VivaldiAutomationProfile, token)
	if err != nil {
		t.Fatalf("OpenInProfile: %v", err)
	}
	if res.BrowserWindowID != "vw-new" || res.LiveWindow != w.LiveWindowID("vw-new") {
		t.Fatalf("OpenResult = %+v, want BrowserWindowID/LiveWindow=vw-new", res)
	}
	if querier.calls < 2 {
		t.Fatalf("expected at least 2 querier calls (before+after), got %d", querier.calls)
	}
}

func TestVivaldiAdapterCollectCloseObservationReportsCorrelationAndIsolation(t *testing.T) {
	ctx := context.Background()
	store, err := NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	token, err := store.Put(ctx, PrivatePayload{URLs: []string{"https://example.test/x"}})
	if err != nil {
		t.Fatalf("put payload: %v", err)
	}
	t.Run("isolated", func(t *testing.T) {
		querier := &fakeVivaldiQuerier{}
		querier.push([]VivaldiOmniWMWindow{
			{LiveWindow: "vw-target", PID: 200, Title: "Page - " + VivaldiAutomationProfile, BundleID: VivaldiBundleID},
		})
		adapter := NewVivaldiAdapterWithWM(store, &recordingAppOpener{}, "", querier, &fakeVivaldiCloser{})
		obs, err := adapter.CollectCloseObservation(ctx, CloseObservationParams{
			Profile: VivaldiAutomationProfile, PayloadToken: token, LiveWindow: "vw-target",
		})
		if err != nil {
			t.Fatalf("CollectCloseObservation: %v", err)
		}
		if !obs.Present || obs.ObservedBundle != VivaldiBundleID || obs.CorrelatedBrowserID != "vw-target" {
			t.Fatalf("observation = %+v", obs)
		}
		if !obs.UserProfileIsolated {
			t.Fatalf("expected UserProfileIsolated, got %+v", obs)
		}
		if !obs.TabPayloadCorrelated || obs.ObservedPayloadToken != token {
			t.Fatalf("expected payload correlation, got %+v", obs)
		}
	})
	t.Run("not-isolated-when-other-profile-window-shares-profile", func(t *testing.T) {
		querier := &fakeVivaldiQuerier{}
		querier.push([]VivaldiOmniWMWindow{
			{LiveWindow: "vw-target", PID: 200, Title: "Page - " + VivaldiAutomationProfile, BundleID: VivaldiBundleID},
			{LiveWindow: "vw-other", PID: 201, Title: "Other - " + VivaldiAutomationProfile, BundleID: VivaldiBundleID},
		})
		adapter := NewVivaldiAdapterWithWM(store, &recordingAppOpener{}, "", querier, &fakeVivaldiCloser{})
		obs, err := adapter.CollectCloseObservation(ctx, CloseObservationParams{
			Profile: VivaldiAutomationProfile, PayloadToken: token, LiveWindow: "vw-target",
		})
		if err != nil {
			t.Fatalf("CollectCloseObservation: %v", err)
		}
		if obs.UserProfileIsolated {
			t.Fatalf("expected NOT isolated when another window shares profile: %+v", obs)
		}
	})
	t.Run("absent-after-close", func(t *testing.T) {
		querier := &fakeVivaldiQuerier{}
		querier.push([]VivaldiOmniWMWindow{
			{LiveWindow: "vw-other", PID: 201, Title: "Other - other", BundleID: VivaldiBundleID},
		})
		adapter := NewVivaldiAdapterWithWM(store, &recordingAppOpener{}, "", querier, &fakeVivaldiCloser{})
		obs, err := adapter.CollectCloseObservation(ctx, CloseObservationParams{
			Profile: VivaldiAutomationProfile, PayloadToken: token, LiveWindow: "vw-target",
		})
		if err != nil {
			t.Fatalf("CollectCloseObservation: %v", err)
		}
		if obs.Present || obs.MatchingRemaining != 0 {
			t.Fatalf("expected target absent, got %+v", obs)
		}
	})
}

func TestVivaldiAdapterCloseLiveWindowInvokesCloserAndWaitsForDisappearance(t *testing.T) {
	ctx := context.Background()
	store, err := NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	querier := &fakeVivaldiQuerier{}
	// findVivaldiWindow lookup before close (target present).
	querier.push([]VivaldiOmniWMWindow{
		{LiveWindow: "vw-target", PID: 200, Title: "Page - " + VivaldiAutomationProfile, BundleID: VivaldiBundleID},
	})
	// waitForVivaldiWindowGone first poll: target gone.
	querier.push([]VivaldiOmniWMWindow{})
	closer := &fakeVivaldiCloser{}
	adapter := NewVivaldiAdapterWithWM(store, &recordingAppOpener{}, "", querier, closer)
	adapter.DisappearWait = 200 * time.Millisecond
	if err := adapter.CloseLiveWindow(ctx, "vw-target"); err != nil {
		t.Fatalf("CloseLiveWindow: %v", err)
	}
	if !closer.called || closer.last.LiveWindow != "vw-target" || closer.last.PID != 200 {
		t.Fatalf("close mutation not invoked correctly: %+v", closer)
	}
}

func TestVivaldiAdapterCloseLiveWindowReturnsErrorIfWindowStillPresent(t *testing.T) {
	ctx := context.Background()
	store, err := NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	querier := &fakeVivaldiQuerier{}
	// Pre-close lookup
	querier.push([]VivaldiOmniWMWindow{
		{LiveWindow: "vw-stuck", PID: 250, Title: "Page - " + VivaldiAutomationProfile, BundleID: VivaldiBundleID},
	})
	// All subsequent polls show target still present
	for i := 0; i < 10; i++ {
		querier.push([]VivaldiOmniWMWindow{
			{LiveWindow: "vw-stuck", PID: 250, Title: "Page - " + VivaldiAutomationProfile, BundleID: VivaldiBundleID},
		})
	}
	adapter := NewVivaldiAdapterWithWM(store, &recordingAppOpener{}, "", querier, &fakeVivaldiCloser{})
	adapter.DisappearWait = 80 * time.Millisecond
	err = adapter.CloseLiveWindow(ctx, "vw-stuck")
	if err == nil || !strings.Contains(err.Error(), "still present after close") {
		t.Fatalf("CloseLiveWindow expected disappearance failure, got %v", err)
	}
}
