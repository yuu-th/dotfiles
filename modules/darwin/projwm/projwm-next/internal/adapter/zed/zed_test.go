package zed

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

type fakeQuerier struct {
	wins []OmniWMWindow
	err  error
}

func (s *fakeQuerier) QueryZedWindows(ctx context.Context) ([]OmniWMWindow, error) {
	return s.wins, s.err
}

type fakeProber struct {
	dirty bool
	err   error
}

func (s *fakeProber) ProbeUnsavedChanges(ctx context.Context, win OmniWMWindow) (bool, error) {
	return s.dirty, s.err
}

type fakeCloser struct {
	called bool
	err    error
}

func (s *fakeCloser) CloseZedWindow(ctx context.Context, win OmniWMWindow) error {
	s.called = true
	return s.err
}

func TestCollectCloseObservationGathersPresenceAndProjectRootCorrelation(t *testing.T) {
	root := t.TempDir()
	// Use a child path to ensure our basename test correlates a Zed window
	// titled with the project basename.
	projectName := filepath.Base(root)
	live := w.LiveWindowID("zed-live-1")
	q := &fakeQuerier{wins: []OmniWMWindow{
		{LiveWindow: live, PID: 4242, Title: projectName, BundleID: ZedBundleID},
	}}
	a := NewAdapter(q, &fakeCloser{}, &fakeProber{dirty: false})

	obs, err := a.CollectCloseObservation(context.Background(), CloseObservationParams{
		ProjectRoot: root,
		LiveWindow:  live,
	})
	if err != nil {
		t.Fatalf("CollectCloseObservation: %v", err)
	}
	if !obs.Present || obs.MatchingRemaining != 1 {
		t.Fatalf("expected presence + matching=1, got %+v", obs)
	}
	if obs.AdapterWindowID != string(live) {
		t.Fatalf("AdapterWindowID = %q, want %q", obs.AdapterWindowID, string(live))
	}
	if obs.AdapterSessionID == "" {
		t.Fatalf("AdapterSessionID should be set")
	}
	if obs.AdapterProjectRoot == "" {
		t.Fatalf("AdapterProjectRoot should correlate when title contains project basename, got empty")
	}
	if obs.UnsavedChanges != UnsavedChangeClean {
		t.Fatalf("UnsavedChanges = %q, want clean", obs.UnsavedChanges)
	}
}

func TestCollectCloseObservationReportsDirtyOnProberDirty(t *testing.T) {
	root := t.TempDir()
	projectName := filepath.Base(root)
	live := w.LiveWindowID("zed-live-1")
	q := &fakeQuerier{wins: []OmniWMWindow{
		{LiveWindow: live, PID: 4242, Title: projectName, BundleID: ZedBundleID},
	}}
	a := NewAdapter(q, &fakeCloser{}, &fakeProber{dirty: true})

	obs, err := a.CollectCloseObservation(context.Background(), CloseObservationParams{
		ProjectRoot: root,
		LiveWindow:  live,
	})
	if err != nil {
		t.Fatalf("CollectCloseObservation: %v", err)
	}
	if obs.UnsavedChanges != UnsavedChangeDirty {
		t.Fatalf("UnsavedChanges = %q, want dirty", obs.UnsavedChanges)
	}
}

func TestCollectCloseObservationReportsUnknownOnProbeFailure(t *testing.T) {
	root := t.TempDir()
	projectName := filepath.Base(root)
	live := w.LiveWindowID("zed-live-1")
	q := &fakeQuerier{wins: []OmniWMWindow{
		{LiveWindow: live, PID: 4242, Title: projectName, BundleID: ZedBundleID},
	}}
	a := NewAdapter(q, &fakeCloser{}, &fakeProber{err: errors.New("AX permission denied")})

	obs, err := a.CollectCloseObservation(context.Background(), CloseObservationParams{
		ProjectRoot: root,
		LiveWindow:  live,
	})
	if err != nil {
		t.Fatalf("CollectCloseObservation: %v", err)
	}
	if obs.UnsavedChanges != UnsavedChangeUnknown {
		t.Fatalf("UnsavedChanges = %q, want unknown when prober errors", obs.UnsavedChanges)
	}
}

func TestCollectCloseObservationLeavesProjectRootEmptyOnTitleMismatch(t *testing.T) {
	root := t.TempDir()
	live := w.LiveWindowID("zed-live-1")
	q := &fakeQuerier{wins: []OmniWMWindow{
		{LiveWindow: live, PID: 4242, Title: "completely-unrelated-title", BundleID: ZedBundleID},
	}}
	a := NewAdapter(q, &fakeCloser{}, &fakeProber{dirty: false})

	obs, err := a.CollectCloseObservation(context.Background(), CloseObservationParams{
		ProjectRoot: root,
		LiveWindow:  live,
	})
	if err != nil {
		t.Fatalf("CollectCloseObservation: %v", err)
	}
	if obs.AdapterProjectRoot != "" {
		t.Fatalf("AdapterProjectRoot should remain empty when title does not match project basename, got %q", obs.AdapterProjectRoot)
	}
}

func TestCollectCloseObservationReturnsAbsentWhenLiveMissing(t *testing.T) {
	root := t.TempDir()
	q := &fakeQuerier{wins: []OmniWMWindow{
		{LiveWindow: "other", PID: 1, Title: "x", BundleID: ZedBundleID},
	}}
	a := NewAdapter(q, &fakeCloser{}, &fakeProber{})

	obs, err := a.CollectCloseObservation(context.Background(), CloseObservationParams{
		ProjectRoot: root,
		LiveWindow:  "missing",
	})
	if err != nil {
		t.Fatalf("CollectCloseObservation: %v", err)
	}
	if obs.Present || obs.MatchingRemaining != 0 {
		t.Fatalf("expected absent observation, got %+v", obs)
	}
}

func TestCollectCloseObservationRequiresLiveWindowID(t *testing.T) {
	a := NewAdapter(&fakeQuerier{}, &fakeCloser{}, &fakeProber{})
	if _, err := a.CollectCloseObservation(context.Background(), CloseObservationParams{}); err == nil {
		t.Fatalf("expected missing-live-window error")
	}
}

func TestCloseLiveWindowDispatchesToWindowCloserAndWaitsGone(t *testing.T) {
	live := w.LiveWindowID("zed-live-1")
	q := &fakeMutatingQuerier{
		states: [][]OmniWMWindow{
			{{LiveWindow: live, PID: 4242, Title: "x", BundleID: ZedBundleID}}, // findZedWindow
			{}, // waitForZedWindowGone
		},
	}
	closer := &fakeCloser{}
	a := NewAdapter(q, closer, &fakeProber{})

	if err := a.CloseLiveWindow(context.Background(), live); err != nil {
		t.Fatalf("CloseLiveWindow: %v", err)
	}
	if !closer.called {
		t.Fatalf("expected WindowCloser to be invoked")
	}
}

func TestCloseLiveWindowIsIdempotentWhenAlreadyGone(t *testing.T) {
	q := &fakeQuerier{wins: []OmniWMWindow{}}
	closer := &fakeCloser{}
	a := NewAdapter(q, closer, &fakeProber{})

	if err := a.CloseLiveWindow(context.Background(), w.LiveWindowID("missing")); err != nil {
		t.Fatalf("CloseLiveWindow: %v", err)
	}
	if closer.called {
		t.Fatalf("WindowCloser must not be invoked when target is already gone")
	}
}

// fakeMutatingQuerier returns a different window list per call. The Adapter
// queries once to find the target and again (potentially many times) to wait
// for disappearance.
type fakeMutatingQuerier struct {
	states [][]OmniWMWindow
	calls  int
}

func (s *fakeMutatingQuerier) QueryZedWindows(ctx context.Context) ([]OmniWMWindow, error) {
	idx := s.calls
	if idx >= len(s.states) {
		idx = len(s.states) - 1
	}
	s.calls++
	if idx < 0 {
		return nil, nil
	}
	return s.states[idx], nil
}
