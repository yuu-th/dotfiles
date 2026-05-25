package wm

import (
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// fakeVivaldi returns a test helper that swaps vivaldiInspectFunc so
// classifyLiveWindow can be exercised without spawning a real ps.
func fakeVivaldi(t *testing.T, byPID map[int]bool) {
	t.Helper()
	orig := vivaldiInspectFunc
	t.Cleanup(func() {
		vivaldiInspectFunc = orig
		vivaldiCacheMu.Lock()
		vivaldiCache = map[int]bool{}
		vivaldiCacheMu.Unlock()
	})
	vivaldiInspectFunc = func(pid int) bool {
		v, ok := byPID[pid]
		if !ok {
			return true
		}
		return v
	}
	vivaldiCacheMu.Lock()
	vivaldiCache = map[int]bool{}
	vivaldiCacheMu.Unlock()
}

func TestClassifyLiveWindow_VivaldiAutomationProfile(t *testing.T) {
	fakeVivaldi(t, map[int]bool{42: true})
	cw := ctlWindow{
		App: struct {
			BundleID string `json:"bundleId"`
			Name     string `json:"name"`
		}{BundleID: "com.vivaldi.Vivaldi"},
		PID:   42,
		Title: "GitHub - example",
	}
	if got := classifyLiveWindow(cw); got != w.WindowBrowser {
		t.Errorf("automation profile classify = %s, want WindowBrowser", got)
	}
}

func TestClassifyLiveWindow_VivaldiUserProfile_External(t *testing.T) {
	fakeVivaldi(t, map[int]bool{99: false})
	cw := ctlWindow{
		App: struct {
			BundleID string `json:"bundleId"`
			Name     string `json:"name"`
		}{BundleID: "com.vivaldi.Vivaldi"},
		PID:   99,
		Title: "Personal",
	}
	if got := classifyLiveWindow(cw); got != w.WindowExternal {
		t.Errorf("user profile classify = %s, want WindowExternal", got)
	}
}

func TestClassifyLiveWindow_VivaldiCache(t *testing.T) {
	calls := 0
	fakeVivaldi(t, nil) // default-managed
	orig := vivaldiInspectFunc
	vivaldiInspectFunc = func(pid int) bool {
		calls++
		return orig(pid)
	}
	cw := ctlWindow{App: struct {
		BundleID string `json:"bundleId"`
		Name     string `json:"name"`
	}{BundleID: "com.vivaldi.Vivaldi"}, PID: 7}
	_ = classifyLiveWindow(cw)
	_ = classifyLiveWindow(cw)
	_ = classifyLiveWindow(cw)
	if calls != 1 {
		t.Errorf("expected 1 inspect call (cached), got %d", calls)
	}
}

func TestClassifyLiveWindow_NonVivaldiUnaffected(t *testing.T) {
	fakeVivaldi(t, map[int]bool{1: false})
	cw := ctlWindow{App: struct {
		BundleID string `json:"bundleId"`
		Name     string `json:"name"`
	}{BundleID: "com.mitchellh.ghostty"}, PID: 1, Title: "shell-1:dotfiles"}
	if got := classifyLiveWindow(cw); got != w.WindowShell {
		t.Errorf("ghostty classify = %s, want WindowShell", got)
	}
}
