package wm

import (
	"encoding/json"
	"testing"
)

// TestZedManagedSettingsIsolatesFromUserZed is an owner test for SSOT §4.4
// editor config separation (§10.9 GAP-07): the projwm-managed Zed settings
// disable session restore and extension auto-install so the managed instance
// never reopens the user's unrelated windows or shares extension state.
func TestZedManagedSettingsIsolatesFromUserZed(t *testing.T) {
	var m map[string]any
	if err := json.Unmarshal([]byte(zedManagedSettingsJSON), &m); err != nil {
		t.Fatalf("zedManagedSettingsJSON is not valid JSON: %v", err)
	}
	if m["restore_on_startup"] != "none" {
		t.Errorf("restore_on_startup = %v, want \"none\" (managed Zed must not reopen prior windows)", m["restore_on_startup"])
	}
	ext, ok := m["auto_install_extensions"].(map[string]any)
	if !ok || len(ext) != 0 {
		t.Errorf("auto_install_extensions = %v, want {} (managed Zed isolated from user extensions)", m["auto_install_extensions"])
	}
}

// TestZedLaunchArgsForceNewWindowAndPrivateDataDir asserts the managed Zed
// launch always passes -n (new window, not reuse) and --user-data-dir <dir>
// (private data dir), followed by extra args then the project path. SSOT §4.4.
func TestZedLaunchArgsForceNewWindowAndPrivateDataDir(t *testing.T) {
	got := zedLaunchArgs("/private/zed-data", "/proj/root", []string{"--foo"})
	want := []string{"-n", "--user-data-dir", "/private/zed-data", "--foo", "/proj/root"}
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args = %v, want %v", got, want)
		}
	}
}
