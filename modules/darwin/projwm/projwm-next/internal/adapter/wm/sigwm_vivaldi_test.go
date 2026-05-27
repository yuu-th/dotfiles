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

// TestVivaldiManaged_OnlyCachesPositive guards the fix for the S2 browser
// archive→unarchive→assign redeploy loop: a freshly-spawned managed Vivaldi
// whose PID had previously been classified false (transient unreadable argv,
// or a now-dead helper PID that macOS reused) must be re-inspected, not pinned
// WindowExternal forever — otherwise identity.Resolve returns ClassMissing and
// the planner re-emits spawn-browser every replan. true is stable and cached.
func TestVivaldiManaged_OnlyCachesPositive(t *testing.T) {
	orig := vivaldiInspectFunc
	t.Cleanup(func() {
		vivaldiInspectFunc = orig
		vivaldiCacheMu.Lock()
		vivaldiCache = map[int]bool{}
		vivaldiCacheMu.Unlock()
	})
	vivaldiCacheMu.Lock()
	vivaldiCache = map[int]bool{}
	vivaldiCacheMu.Unlock()

	result := false
	calls := 0
	vivaldiInspectFunc = func(int) bool { calls++; return result }

	if vivaldiManaged(7) {
		t.Fatal("first inspect returns false")
	}
	// A false result must NOT be memoized.
	result = true
	if !vivaldiManaged(7) {
		t.Fatal("false must be re-inspected: a managed PID reuse must reclassify to true")
	}
	if calls != 2 {
		t.Fatalf("expected re-inspect after false (2 calls), got %d", calls)
	}
	// A true result IS memoized for the process lifetime.
	result = false
	if !vivaldiManaged(7) {
		t.Fatal("true must be cached: managed classification is stable")
	}
	if calls != 2 {
		t.Fatalf("expected true cached (still 2 calls), got %d", calls)
	}
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
