package lifecyclecontract

import (
	"fmt"
	"path/filepath"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	w "github.com/yuu-th/projwm-next/internal/world"
)

type UnsavedChangeState string

const (
	UnsavedChangeUnknown UnsavedChangeState = ""
	UnsavedChangeClean   UnsavedChangeState = "clean"
	UnsavedChangeDirty   UnsavedChangeState = "dirty"
)

type ExactDisappearanceEvidence struct {
	TargetLiveWindow  w.LiveWindowID
	BeforePresent     bool
	AfterPresent      bool
	MatchingRemaining int
}

func ValidateExactDisappearance(e ExactDisappearanceEvidence) error {
	if e.TargetLiveWindow == "" {
		return fmt.Errorf("exact disappearance: target live window is required")
	}
	if !e.BeforePresent {
		return fmt.Errorf("exact disappearance: target %q was not present before close", e.TargetLiveWindow)
	}
	if e.AfterPresent {
		return fmt.Errorf("exact disappearance: target %q is still present after close", e.TargetLiveWindow)
	}
	if e.MatchingRemaining != 0 {
		return fmt.Errorf("exact disappearance: %d matching identity windows remain", e.MatchingRemaining)
	}
	return nil
}

type ZedProjectScopedRemovalEvidence struct {
	Desired        w.DesiredWindowID
	Policy         w.ManagedAppPolicy
	ObservedBundle string

	ProjectRoot        string
	AdapterProjectRoot string
	AdapterSessionID   string
	AdapterWindowID    string
	UnsavedChanges     UnsavedChangeState

	Disappearance ExactDisappearanceEvidence
}

func ValidateZedProjectScopedRemoval(e ZedProjectScopedRemovalEvidence) error {
	if e.Desired.Kind != w.WindowEditor {
		return fmt.Errorf("zed removal contract: desired editor window id is required")
	}
	if e.Policy.Capability != w.CapabilityEditor || e.Policy.BundleID == "" {
		return fmt.Errorf("zed removal contract: managed editor app policy is required")
	}
	if e.Policy.BundleID != e.ObservedBundle {
		return fmt.Errorf("zed removal contract: observed bundle %q does not match policy bundle %q", e.ObservedBundle, e.Policy.BundleID)
	}
	if !e.Policy.LifecycleRemoval.Allowed || e.Policy.LifecycleRemoval.Method != w.LifecycleRemovalProjectScopedApp {
		return fmt.Errorf("zed removal contract: project-scoped-app policy must be explicitly allowed")
	}
	projectRoot, err := canonicalProjectRoot(e.ProjectRoot)
	if err != nil {
		return fmt.Errorf("zed removal contract: project root: %w", err)
	}
	adapterRoot, err := canonicalProjectRoot(e.AdapterProjectRoot)
	if err != nil {
		return fmt.Errorf("zed removal contract: adapter project root: %w", err)
	}
	if projectRoot != adapterRoot {
		return fmt.Errorf("zed removal contract: adapter project root does not match desired project root")
	}
	if e.AdapterSessionID == "" || e.AdapterWindowID == "" {
		return fmt.Errorf("zed removal contract: app-specific session/window evidence is required")
	}
	if e.UnsavedChanges != UnsavedChangeClean {
		return fmt.Errorf("zed removal contract: clean unsaved-change proof is required")
	}
	if err := ValidateExactDisappearance(e.Disappearance); err != nil {
		return fmt.Errorf("zed removal contract: %w", err)
	}
	return nil
}

func canonicalProjectRoot(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

type VivaldiBrowserWindowCloseEvidence struct {
	Desired        w.DesiredWindowID
	Policy         w.ManagedAppPolicy
	ObservedBundle string

	Profile              string
	PayloadToken         string
	ObservedPayloadToken string
	BrowserWindowID      string
	CorrelatedBrowserID  string
	LiveWindow           w.LiveWindowID
	CorrelatedLiveWindow w.LiveWindowID
	TabPayloadCorrelated bool
	UserProfileIsolated  bool

	Disappearance ExactDisappearanceEvidence
}

func ValidateVivaldiBrowserWindowClose(e VivaldiBrowserWindowCloseEvidence) error {
	if e.Desired.Kind != w.WindowBrowser {
		return fmt.Errorf("vivaldi close contract: desired browser window id is required")
	}
	if e.Policy.Capability != w.CapabilityBrowser || e.Policy.BundleID == "" {
		return fmt.Errorf("vivaldi close contract: managed browser app policy is required")
	}
	if e.Policy.BundleID != e.ObservedBundle {
		return fmt.Errorf("vivaldi close contract: observed bundle %q does not match policy bundle %q", e.ObservedBundle, e.Policy.BundleID)
	}
	if !e.Policy.LifecycleRemoval.Allowed || e.Policy.LifecycleRemoval.Method != w.LifecycleRemovalBrowserWindowClose {
		return fmt.Errorf("vivaldi close contract: browser-window-close policy must be explicitly allowed")
	}
	if e.Profile != browser.VivaldiAutomationProfile {
		return fmt.Errorf("vivaldi close contract: automation-owned non-default profile %q is required", browser.VivaldiAutomationProfile)
	}
	if e.PayloadToken == "" || e.ObservedPayloadToken == "" || e.PayloadToken != e.ObservedPayloadToken {
		return fmt.Errorf("vivaldi close contract: private payload token correlation is required")
	}
	if e.BrowserWindowID == "" || e.CorrelatedBrowserID == "" || e.BrowserWindowID != e.CorrelatedBrowserID {
		return fmt.Errorf("vivaldi close contract: browser window to adapter evidence correlation is required")
	}
	if e.LiveWindow == "" || e.CorrelatedLiveWindow == "" || e.LiveWindow != e.CorrelatedLiveWindow {
		return fmt.Errorf("vivaldi close contract: browser window to WM live window correlation is required")
	}
	if !e.TabPayloadCorrelated {
		return fmt.Errorf("vivaldi close contract: tab/payload correlation is required")
	}
	if !e.UserProfileIsolated {
		return fmt.Errorf("vivaldi close contract: user profile isolation is required")
	}
	if err := ValidateExactDisappearance(e.Disappearance); err != nil {
		return fmt.Errorf("vivaldi close contract: %w", err)
	}
	return nil
}
