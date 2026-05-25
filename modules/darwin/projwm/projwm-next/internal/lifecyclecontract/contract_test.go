package lifecyclecontract

import (
	"strings"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestValidateZedProjectScopedRemovalRequiresAppSpecificEvidence(t *testing.T) {
	root := t.TempDir()
	valid := ZedProjectScopedRemovalEvidence{
		Desired:            w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowEditor, Index: 1},
		Policy:             zedPolicy(true),
		ObservedBundle:     "dev.zed.Zed",
		ProjectRoot:        root,
		AdapterProjectRoot: root,
		AdapterSessionID:   "zed-session-1",
		AdapterWindowID:    "zed-window-1",
		UnsavedChanges:     UnsavedChangeClean,
		Disappearance:      exactGone("live-zed-1"),
	}
	if err := ValidateZedProjectScopedRemoval(valid); err != nil {
		t.Fatalf("valid zed contract rejected: %v", err)
	}

	tests := []struct {
		name string
		edit func(*ZedProjectScopedRemovalEvidence)
		want string
	}{
		{name: "policy-blocked", edit: func(e *ZedProjectScopedRemovalEvidence) { e.Policy = zedPolicy(false) }, want: "explicitly allowed"},
		{name: "bundle-only", edit: func(e *ZedProjectScopedRemovalEvidence) { e.Desired = w.DesiredWindowID{} }, want: "desired editor window id"},
		{name: "title-or-bundle-only-no-session", edit: func(e *ZedProjectScopedRemovalEvidence) { e.AdapterSessionID = "" }, want: "session/window evidence"},
		{name: "single-candidate-no-adapter-window", edit: func(e *ZedProjectScopedRemovalEvidence) { e.AdapterWindowID = "" }, want: "session/window evidence"},
		{name: "project-root-mismatch", edit: func(e *ZedProjectScopedRemovalEvidence) { e.AdapterProjectRoot = t.TempDir() }, want: "project root does not match"},
		{name: "unsaved-change-unknown", edit: func(e *ZedProjectScopedRemovalEvidence) { e.UnsavedChanges = UnsavedChangeUnknown }, want: "clean unsaved-change proof"},
		{name: "unsaved-change-dirty", edit: func(e *ZedProjectScopedRemovalEvidence) { e.UnsavedChanges = UnsavedChangeDirty }, want: "clean unsaved-change proof"},
		{name: "not-exact-disappearance", edit: func(e *ZedProjectScopedRemovalEvidence) { e.Disappearance.MatchingRemaining = 1 }, want: "matching identity windows remain"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := valid
			tc.edit(&e)
			err := ValidateZedProjectScopedRemoval(e)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateZedProjectScopedRemoval error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestValidateVivaldiBrowserWindowCloseRequiresCorrelationEvidence(t *testing.T) {
	valid := VivaldiBrowserWindowCloseEvidence{
		Desired:              w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowBrowser, Index: 1},
		Policy:               vivaldiPolicy(true),
		ObservedBundle:       "com.vivaldi.Vivaldi",
		Profile:              browser.VivaldiAutomationProfile,
		PayloadToken:         "browser-payload-v1-00000000000000000000000000000000",
		ObservedPayloadToken: "browser-payload-v1-00000000000000000000000000000000",
		BrowserWindowID:      "vivaldi-window-1",
		CorrelatedBrowserID:  "vivaldi-window-1",
		LiveWindow:           "omni-vivaldi-1",
		CorrelatedLiveWindow: "omni-vivaldi-1",
		TabPayloadCorrelated: true,
		UserProfileIsolated:  true,
		Disappearance:        exactGone("omni-vivaldi-1"),
	}
	if err := ValidateVivaldiBrowserWindowClose(valid); err != nil {
		t.Fatalf("valid vivaldi contract rejected: %v", err)
	}

	tests := []struct {
		name string
		edit func(*VivaldiBrowserWindowCloseEvidence)
		want string
	}{
		{name: "policy-blocked", edit: func(e *VivaldiBrowserWindowCloseEvidence) { e.Policy = vivaldiPolicy(false) }, want: "explicitly allowed"},
		{name: "default-profile", edit: func(e *VivaldiBrowserWindowCloseEvidence) { e.Profile = "default" }, want: "automation-owned non-default profile"},
		{name: "token-only-no-browser-window", edit: func(e *VivaldiBrowserWindowCloseEvidence) { e.BrowserWindowID = "" }, want: "browser window to adapter evidence correlation"},
		{name: "browser-window-no-wm-correlation", edit: func(e *VivaldiBrowserWindowCloseEvidence) { e.CorrelatedLiveWindow = "" }, want: "WM live window correlation"},
		{name: "saved-url-or-token-mismatch", edit: func(e *VivaldiBrowserWindowCloseEvidence) { e.ObservedPayloadToken = "other-token" }, want: "private payload token correlation"},
		{name: "tab-count-only", edit: func(e *VivaldiBrowserWindowCloseEvidence) { e.TabPayloadCorrelated = false }, want: "tab/payload correlation"},
		{name: "user-profile-window", edit: func(e *VivaldiBrowserWindowCloseEvidence) { e.UserProfileIsolated = false }, want: "user profile isolation"},
		{name: "not-exact-disappearance", edit: func(e *VivaldiBrowserWindowCloseEvidence) { e.Disappearance.AfterPresent = true }, want: "still present after close"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := valid
			tc.edit(&e)
			err := ValidateVivaldiBrowserWindowClose(e)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateVivaldiBrowserWindowClose error = %v, want %q", err, tc.want)
			}
		})
	}
}

func zedPolicy(allowed bool) w.ManagedAppPolicy {
	return w.ManagedAppPolicy{
		Capability: w.CapabilityEditor,
		BundleID:   "dev.zed.Zed",
		LifecycleRemoval: w.LifecycleRemovalPolicy{
			Allowed: allowed,
			Method:  w.LifecycleRemovalProjectScopedApp,
		},
	}
}

func vivaldiPolicy(allowed bool) w.ManagedAppPolicy {
	return w.ManagedAppPolicy{
		Capability: w.CapabilityBrowser,
		BundleID:   "com.vivaldi.Vivaldi",
		LifecycleRemoval: w.LifecycleRemovalPolicy{
			Allowed: allowed,
			Method:  w.LifecycleRemovalBrowserWindowClose,
		},
	}
}

func exactGone(id w.LiveWindowID) ExactDisappearanceEvidence {
	return ExactDisappearanceEvidence{
		TargetLiveWindow: id,
		BeforePresent:    true,
		AfterPresent:     false,
	}
}
