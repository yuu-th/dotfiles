package zed

import (
	"context"
	"path/filepath"
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestSSOTL2ZedObservationCorrelatesProjectRootByBasenameTitle(t *testing.T) {
	root := t.TempDir()
	live := w.LiveWindowID("zed-live-1")
	q := &fakeQuerier{wins: []OmniWMWindow{
		{LiveWindow: live, PID: 4242, Title: filepath.Base(root), BundleID: ZedBundleID},
	}}
	a := NewAdapter(q, &fakeCloser{}, &fakeProber{dirty: false})

	obs, err := a.CollectCloseObservation(context.Background(), CloseObservationParams{
		ProjectRoot: root,
		LiveWindow:  live,
	})
	if err != nil {
		t.Fatalf("CollectCloseObservation: %v", err)
	}
	if !obs.Present || obs.AdapterProjectRoot == "" {
		t.Fatalf("SSOT L2 Zed observation must correlate basename title to project root, got %+v", obs)
	}
}
