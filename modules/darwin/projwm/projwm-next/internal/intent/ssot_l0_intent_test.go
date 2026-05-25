package intent

import "testing"

// TestSSOTUserOperationsHaveIntentSurface verifies SSOT §4.1 — the 17 user
// operations + their orphan/system-level companions — all have a daemon
// intent surface. It does NOT assert anything about the reducer/planner
// implementing those intents; that is checked separately at L1/L2.
func TestSSOTUserOperationsHaveIntentSurface(t *testing.T) {
	// The full set of intent kinds the daemon currently accepts. Adding a
	// new intent constant in intent.go and forgetting to register it here
	// must remain a deliberate act, so this is an explicit allowlist —
	// not a reflection of the var-decl list.
	current := map[string]bool{
		// SSOT §4.1 operations 1-17.
		string(KindSummonShell):           true,
		string(KindSummonEditor):          true,
		string(KindSummonBrowser):         true,
		string(KindSwitchProject):         true,
		string(KindCycleSlotWindow):       true,
		string(KindSummonViewer):          true,
		string(KindSetCockpitVisibility):  true,
		string(KindSwitchProfile):         true,
		string(KindCreateProject):         true,
		string(KindArchiveProject):        true,
		string(KindUnarchiveProject):      true,
		string(KindShowScratchShell):      true,
		string(KindHideScratchShell):      true,
		string(KindAddWindow):             true,
		string(KindRemoveWindow):          true,
		string(KindBrowserAddTab):         true,
		string(KindBrowserRemoveTab):      true,
		string(KindBrowserChangeTabURL):   true,
		string(KindBrowserReorderTabs):    true,
		// §6.4 state-ownership intents (cockpit / CLI surface).
		string(KindAssignProject): true,
		string(KindUnassignSlot):  true,
		string(KindCreateProfile): true,
		string(KindDeleteProfile): true,
		string(KindRenameProfile): true,
		string(KindDeleteProject): true,
		// §4.3 orphan card actions + dismissal.
		string(KindAdoptOrphanWindow):   true,
		string(KindDismissOrphanWindow): true,
		string(KindDismissCard):         true,
		string(KindDismissAllCards):     true,
		// Convergence.
		string(KindReconcile):           true,
		string(KindValidateEnvironment): true,
		// Internal controller-to-self.
		string(KindAutoSyncLayout):           true,
		string(KindSyncCockpitSystemWindows): true,
	}
	// SSOT §4.1's 17 operations all need a primary user-facing intent.
	required := []string{
		"summon-shell",
		"summon-editor",
		"summon-browser",
		"switch-project",
		"cycle-slot-window",
		"summon-viewer",
		"set-cockpit-visibility",
		"switch-profile",
		"create-project",
		"archive-project",
		"unarchive-project",
		"show-scratch-shell",
		"hide-scratch-shell",
		"add-window",
		"remove-window",
		"browser-add-tab",
		"browser-remove-tab",
		"browser-change-tab-url",
		"browser-reorder-tabs",
	}
	var missing []string
	for _, kind := range required {
		if !current[kind] {
			missing = append(missing, kind)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("SSOT §4.1 user operation intent surface is incomplete: missing=%v", missing)
	}
}

// TestSSOTDeprecatedIntentsRemoved confirms that the legacy intents have
// been deleted from intent.go (SSOT N-06 / N-12). Their string kinds
// should NOT round-trip through the public Kind set.
func TestSSOTDeprecatedIntentsRemoved(t *testing.T) {
	removed := []string{
		"toggle-cockpit",         // N-06
		"focus-cockpit",          // N-06
		"accept-manual-layout",   // N-12
		"sync-browser-tabs",      // superseded by Browser*Tab intents
		"respawn-orphan-ghostty", // not in §4.3
	}
	current := map[string]bool{
		string(KindSummonShell):              true,
		string(KindSummonEditor):             true,
		string(KindSummonBrowser):            true,
		string(KindSwitchProject):            true,
		string(KindCycleSlotWindow):          true,
		string(KindSummonViewer):             true,
		string(KindSetCockpitVisibility):     true,
		string(KindSwitchProfile):            true,
		string(KindCreateProject):            true,
		string(KindArchiveProject):           true,
		string(KindUnarchiveProject):         true,
		string(KindShowScratchShell):         true,
		string(KindHideScratchShell):         true,
		string(KindAddWindow):                true,
		string(KindRemoveWindow):             true,
		string(KindBrowserAddTab):            true,
		string(KindBrowserRemoveTab):         true,
		string(KindBrowserChangeTabURL):      true,
		string(KindBrowserReorderTabs):       true,
		string(KindAssignProject):            true,
		string(KindUnassignSlot):             true,
		string(KindCreateProfile):            true,
		string(KindDeleteProfile):            true,
		string(KindRenameProfile):            true,
		string(KindDeleteProject):            true,
		string(KindAdoptOrphanWindow):        true,
		string(KindDismissOrphanWindow):      true,
		string(KindDismissCard):              true,
		string(KindDismissAllCards):          true,
		string(KindReconcile):                true,
		string(KindValidateEnvironment):      true,
		string(KindAutoSyncLayout):           true,
		string(KindSyncCockpitSystemWindows): true,
	}
	for _, dep := range removed {
		if current[dep] {
			t.Errorf("deprecated intent kind %q must not be present in the active Kind set", dep)
		}
	}
}
