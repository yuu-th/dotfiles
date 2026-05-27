// Package ssottest maintains the SSOT requirement ledger and audits it
// against the actual test owners in the repository.
//
// LEDGER PROMOTION WORKFLOW (queue/ssot-slice-plan.md DoD #9):
// Each slice completes by promoting its requirement(s) here.
//
//  1. Slice implements the SSOT behavior (intent → reducer → planner →
//     adapter → CLI/TUI).
//  2. Slice's L0/L1/L2/L3 test owners must directly assert observable
//     behavior — NOT just "test function exists" gates (Coverage gates
//     are flagged by referencesMetaOnlyTest below).
//  3. Slice updates the matching ledger row:
//     - Status: statusRed → statusCovered (or statusRealOnly when the
//       only proof requires real_ops/integration tags).
//     - Evidence: evidenceMixed / evidenceMeta → evidenceBehavior. Strip
//       *CoverageGate markers from TestName so only behavior owners
//       remain.
//  4. The TestSSOTLedger* asserts in this file then enforce that the
//     promotion was genuine.
//
// Items with Status==statusRed are tolerated indefinitely as long as
// every Subject is honestly documented; do NOT mark statusCovered until
// the cited test owners actually fail when the behavior regresses.
package ssottest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

type ledgerStatus string
type ledgerEvidence string

const (
	statusCovered  ledgerStatus = "covered"
	statusPartial  ledgerStatus = "partial"
	statusRed      ledgerStatus = "red"
	statusMissing  ledgerStatus = "missing"
	statusRealOnly ledgerStatus = "real-only"
)

const (
	evidenceBehavior ledgerEvidence = "behavior"
	evidenceMeta     ledgerEvidence = "meta"
	evidenceMixed    ledgerEvidence = "mixed"
	evidenceConflict ledgerEvidence = "conflict"
)

type ledgerItem struct {
	ID       string
	Section  string
	Layer    string
	Subject  string
	TestPath string
	TestName string
	Status   ledgerStatus
	Evidence ledgerEvidence
}

func (item ledgerItem) evidenceKind() ledgerEvidence {
	if item.Evidence == "" {
		return evidenceBehavior
	}
	return item.Evidence
}

var ssotLedger = []ledgerItem{
	// §3.4 invariants.
	{ID: "INV-01", Section: "§3.4", Layer: "L0/L1", Subject: "same (project, kind, id) window is globally unique — duplicates: focus-tiebreak winner + orphan list + [INVARIANT] card via Check14", TestPath: "internal/identity/identify_winner_test.go internal/invariant/ssot_l1_invariant_test.go internal/planner/planner_test.go", TestName: "TestIdentifyWinnerAndOrphans_PicksFocusedCandidate/TestIdentifyWinnerAndOrphans_FallsBackToSmallestWhenFocusedNotInCandidates/TestResolveWithFocusTiebreak_ConvertsAmbiguousToUniqueStrong/TestSSOTInvariantCheck14DuplicateWindowFires/TestSSOTInvariantCheck14SkipsViewerPairing/TestSSOTInvariantCheck14SkipsArchivedProject/TestPlanAcceptsAmbiguousActiveDesiredWindowViaFocusTiebreak", Status: statusCovered, Evidence: evidenceBehavior},
	{ID: "INV-02", Section: "§3.4", Layer: "L0/L1", Subject: "active profile windows are on the correct slot", TestPath: "internal/planner/ssot_l0_planner_test.go", TestName: "TestSSOTDriftedActiveWindowPlansMoveNotRespawn", Status: statusCovered},
	{ID: "INV-03", Section: "§3.4", Layer: "L2/L3", Subject: "tmux session is recreated if absent", TestPath: "internal/adapter/session/tmux_test.go internal/adapter/session/ssot_l3_session_real_ops_test.go", TestName: "TestEnsureSessionCreates/TestRealOpsTmuxEnsureSession", Status: statusCovered},
	{ID: "INV-04", Section: "§3.4", Layer: "L0/L1", Subject: "archived project windows do not exist", TestPath: "internal/invariant/ssot_l1_invariant_test.go scenarios/archive_project_test.go", TestName: "TestSSOTInvariantArchivedProjectMustHaveNoLiveWindows", Status: statusCovered},
	{ID: "INV-05", Section: "§3.4", Layer: "L0/L1", Subject: "viewer shows only active profile AI streams", TestPath: "internal/invariant/ssot_l1_invariant_test.go internal/planner", TestName: "TestSSOTInvariantViewerShowsOnlyActiveProfileAIStreams/TestPlanDefersViewerSpawnUntilSourceAIIsObservedUniqueStrong", Status: statusCovered},
	{ID: "INV-06", Section: "§3.4", Layer: "L0/L1/L3", Subject: "cockpit always exists on park workspace CP1 (drift → Check15 invariant card + planner KindMoveCockpitToParkWorkspace op)", TestPath: "internal/reducer/reducer_cockpit_test.go internal/planner/planner_cockpit_test.go internal/invariant/ssot_l1_invariant_test.go internal/adapter/wm/ssot_l3_wm_spec_test.go", TestName: "TestReduceIntent_SyncCockpit_AlwaysOneEntry_RegardlessOfDisplayCount/TestReduceIntent_SyncCockpit_SetsParkWorkspace/TestPlanner_Cockpit_SpawnWhenMissing/TestSSOTInvariantCheck15CockpitOffParkWorkspaceFires/TestSSOTInvariantCheck15CockpitOnParkWorkspaceIsSilent/TestSpawnCockpit", Status: statusCovered, Evidence: evidenceBehavior},
	{ID: "INV-07", Section: "§3.4", Layer: "L0/L3", Subject: "Zed title equals basename(cwd) (L3 real_ops claim: zed_test.go currently lacks the real_ops build tag — honest deferral tracked in gateAuditAllowlist for S29)", TestPath: "internal/naming/ssot_l0_identity_test.go internal/adapter/zed/zed_test.go", TestName: "TestSSOTZedTitleIsProjectRootBasename/TestCollectCloseObservationGathersPresenceAndProjectRootCorrelation", Status: statusCovered},
	{ID: "INV-08", Section: "§3.4", Layer: "L0", Subject: "single profile cannot assign multiple projects to same slot", TestPath: "internal/reducer/ssot_l0_state_test.go", TestName: "TestSSOTProfileSlotAssignmentIsExclusive", Status: statusCovered},
	{ID: "INV-09", Section: "§3.4", Layer: "L0", Subject: "active profile must exist in profiles map", TestPath: "internal/invariant/ssot_l1_invariant_test.go", TestName: "TestSSOTInvariantActiveProfileMustExist", Status: statusCovered},
	{ID: "INV-10", Section: "§3.4", Layer: "L0", Subject: "managed window identity is recoverable from title", TestPath: "internal/naming/ssot_l0_identity_test.go", TestName: "TestSSOTIdentityRestorationFromManagedTitles", Status: statusCovered},
	{ID: "NAMI-01", Section: "§7.3", Layer: "L0", Subject: "AI/window titles are canonical ai-<id>:<project> without AI-name suffix", TestPath: "internal/reducer/ssot_l0_state_test.go", TestName: "TestSSOTDefaultProjectWindowsUseOneBasedStableIDsAndCanonicalTitles", Status: statusCovered},
	{ID: "NAMI-02", Section: "§7.3", Layer: "L0", Subject: "tmux session names use the one-based DesiredWindowID directly", TestPath: "internal/semop/ssot_l1_semop_test.go", TestName: "TestSSOTTerminalSessionFieldsUseOneBasedDesiredIDDirectly", Status: statusCovered},
	{ID: "MANI-01", Section: "§10.7", Layer: "L0", Subject: "testdata/manifest.json uses the SSOT minimal manifest shape", TestPath: "internal/manifest/ssot_l0_manifest_test.go", TestName: "TestSSOTTestManifestParses/TestSSOTTestManifestMatchesSectionTenPointSevenShape", Status: statusCovered},
	{ID: "INV-11", Section: "§3.4", Layer: "L0/L1", Subject: "managed candidate windows outside managed workspace do not create Tier 1 cards", TestPath: "internal/planner/ssot_l0_planner_test.go internal/planner_external_test.go", TestName: "TestSSOTExternalWindowsNeverBecomeTierOneManagedCards/TestPlanner_SkipsWindowExternalOnManagedWS", Status: statusCovered},
	{ID: "INV-12", Section: "§3.4", Layer: "L0/L1", Subject: "viewer order follows slot order", TestPath: "internal/invariant/ssot_l1_invariant_test.go", TestName: "TestSSOTInvariantViewerOrderFollowsSlotOrder", Status: statusCovered},

	// §4.1 user operations.
	{ID: "OP-01", Section: "§4.1/§10.6", Layer: "L1/L2/L3/L4", Subject: "slot shell jump summons shell-1 and cycles shells", TestPath: "internal/intent/ssot_l0_intent_test.go internal/planner/planner_summon_window_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestSSOTUserOperationsHaveIntentSurface/TestPlanner_SummonShell_FirstPressTargetsIndex1/TestPlanner_SummonShell_CycleNextWhenAlreadyOnShell/TestPlanner_SummonShell_CycleWrapsAtEnd/TestPlanner_SummonShell_NoTargetWhenNotSpawned/TestHumanE2ESSOTJumpOperationsFocusExpectedWindowsSteps/TestHumanE2ESSOTSummonIdempotencySteps", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-02", Section: "§4.1/§10.6", Layer: "L1/L2/L3/L4", Subject: "slot editor jump summons editor-1 and cycles editors", TestPath: "internal/intent/ssot_l0_intent_test.go internal/planner/planner_summon_window_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestSSOTUserOperationsHaveIntentSurface/TestPlanner_SummonEditor_TargetsEditorOfSlotsProject/TestHumanE2ESSOTJumpOperationsFocusExpectedWindowsSteps", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-03", Section: "§4.1/§10.6", Layer: "L1/L2/L3/L4", Subject: "slot browser jump summons browser-1 and cycles browsers", TestPath: "internal/intent/ssot_l0_intent_test.go internal/planner/planner_summon_window_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestSSOTUserOperationsHaveIntentSurface/TestPlanner_SummonBrowser_NoTargetWhenSlotUnassigned/TestHumanE2ESSOTJumpOperationsFocusExpectedWindowsSteps", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-04", Section: "§4.1/§10.6", Layer: "L1/L4", Subject: "project switch focuses target slot workspace (omniwm MRU restores last focused window)", TestPath: "internal/intent/ssot_l0_intent_test.go internal/planner/planner_switch_project_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestSSOTUserOperationsHaveIntentSurface/TestPlanner_SwitchProject_EmitsFocusWorkspace/TestPlanner_SwitchProject_NoOpWhenAlreadyOnTargetWorkspace/TestPlanner_SwitchProject_NoOpWhenSlotUnassigned/TestPlanner_SwitchProject_NoOpWhenSlotUnknown/TestHumanE2ESSOTProjectSwitchAndSameSlotWindowSwitchSteps", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-05", Section: "§4.1/§10.6", Layer: "L1/L2/L4", Subject: "same-slot window switch changes focus without workspace change", TestPath: "internal/intent/ssot_l0_intent_test.go internal/planner/planner_cycle_slot_window_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestSSOTUserOperationsHaveIntentSurface/TestPlanner_CycleSlotWindow_SwitchesFromShellToEditor/TestPlanner_CycleSlotWindow_CyclesWithinSameKind/TestPlanner_CycleSlotWindow_NoOpWhenAlreadyOnTarget/TestPlanner_CycleSlotWindow_NoOpWhenKindNotInProject/TestHumanE2ESSOTProjectSwitchAndSameSlotWindowSwitchSteps", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-06", Section: "§4.1/§10.6", Layer: "L1/L3/L4", Subject: "viewer jump focuses viewer workspace A + viewer of last-focused AI (fallback: first slot's viewer)", TestPath: "internal/intent/ssot_l0_intent_test.go internal/planner/planner_summon_viewer_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestSSOTUserOperationsHaveIntentSurface/TestPlanner_SummonViewer_FromFocusedAITargetsItsViewer/TestPlanner_SummonViewer_FromNonAIFallsBackToFirstSlot/TestPlanner_SummonViewer_WhenAlreadyFocusedNoFocusOp/TestPlanner_SummonViewer_WhenViewerNotSpawnedNoOp/TestHumanE2ESSOTJumpOperationsFocusExpectedWindowsSteps", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-07", Section: "§4.1", Layer: "L1/L3/L4", Subject: "cockpit show/hide returns to prior workspace and refocuses prior window", TestPath: "internal/reducer/reducer_cockpit_test.go internal/planner/planner_cockpit_test.go internal/executor/executor_test.go internal/adapter/wm/sigwm_test.go internal/adapter/wm/ssot_l3_wm_spec_test.go", TestName: "TestReduceIntent_SetCockpitVisibility_Shown_PopulatesPrior/TestReduceIntent_SetCockpitVisibility_Hidden_PreservesPrior/TestPlanner_Cockpit_ShowEmitsWhenHidden/TestPlanner_Cockpit_HideEmitsWhenShownAndPriorKnown/TestExecuteHideCockpitRestoresPriorWindowFocus/TestExecuteHideCockpitWithoutPriorWindowSkipsFocus/TestSigWM_ShowCockpitOnDisplay_HappyPath/TestSigWM_HideCockpitOnDisplay_HappyPath/TestCockpitShowHideRestoresPriorWorkspaceAndWindow", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-08", Section: "§4.1", Layer: "L0/L1/L4", Subject: "profile switch updates active profile and closes/spawns via transaction (L4 claim: scenarios/ssot_l4_acceptance_spec_test.go lacks `integration` build tag — honest deferral tracked in gateAuditAllowlist for S29)", TestPath: "internal/reducer/reducer_switch_profile_test.go scenarios/switch_profile_test.go scenarios/ssot_l4_acceptance_spec_test.go", TestName: "TestReduceIntent_SwitchProfile_FlipsActive/TestReduceIntent_SwitchProfile_PreservesBothAssignments/TestSwitchProfile/TestSSOTL4S1SwitchProfile", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-09", Section: "§4.1", Layer: "L0/L1/L4", Subject: "project create with default windows (ai-1/shell-1/editor-1) and canonical title contract", TestPath: "internal/reducer/reducer_v3_test.go cmd/projwm/ssot_cli_surface_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestReduceIntent_CreateProject_Default/TestReduceIntent_CreateProject_RejectsDuplicate/TestReduceIntent_CreateProject_RewritesWindowProject/TestSSOTCLIUpUsesCanonicalDefaultWindowTitles/TestHumanE2ESSOTProjectAddFirstFreeSlotSteps", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-10", Section: "§4.1/§4.5", Layer: "L0/L1/L4", Subject: "project archive removes assignment and closes windows/kills sessions; unarchive returns to park state without slot assignment (L4 claim: scenarios/ssot_l4_acceptance_spec_test.go lacks `integration` build tag — honest deferral tracked in gateAuditAllowlist for S29)", TestPath: "internal/reducer/ssot_l0_state_test.go scenarios/archive_project_test.go scenarios/unarchive_project_test.go internal/invariant/ssot_l1_invariant_test.go scenarios/ssot_l4_acceptance_spec_test.go", TestName: "TestSSOTUnarchiveProjectReturnsToParkStateWithoutSlotAssignment/TestArchiveProject/TestUnarchiveProject/TestSSOTInvariantArchivedProjectMustHaveNoLiveWindows/TestSSOTL4S2ArchiveProject/TestSSOTL4S3UnarchiveProject", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-10B", Section: "§4.5", Layer: "L0", Subject: "unarchive updates state only and does not auto-assign a slot", TestPath: "internal/reducer/ssot_l0_state_test.go", TestName: "TestSSOTUnarchiveProjectReturnsToParkStateWithoutSlotAssignment", Status: statusCovered},
	{ID: "OP-11", Section: "§4.1/§10.6", Layer: "L1/L3/L4", Subject: "scratch shell open/close restores prior focus (intent → reducer → planner → executor → adapter end-to-end)", TestPath: "internal/intent/ssot_l0_intent_test.go internal/reducer/reducer_scratch_test.go internal/planner/planner_scratch_test.go internal/executor/executor_test.go internal/adapter/wm/ssot_l3_wm_spec_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestSSOTUserOperationsHaveIntentSurface/TestReduceIntent_ShowScratchShell_FreshBuild/TestReduceIntent_HideScratchShell_TogglesVisibility/TestPlanner_Scratch_ShowEmitsWhenNotFocused/TestPlanner_Scratch_HideEmitsWhenFocused/TestExecuteShowScratchShell/TestExecuteHideScratchShellRestoresPriorFocus/TestScratchShellShowReturnsLiveWindowID/TestHumanE2ESSOTScratchShellOpenCloseSteps", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-12", Section: "§4.1/§10.6", Layer: "L0/L1/L2/L3/L4", Subject: "add-window uses max id + 1 without hole reuse + AI-name routes send-keys command", TestPath: "internal/reducer/ssot_l0_state_test.go internal/reducer/reducer_v3_test.go internal/semop/ssot_l1_semop_test.go internal/adapter/wm/ssot_l3_wm_real_ops_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestSSOTWindowIndexAllocationNeverReusesDeletedIDs/TestReduceIntent_AddWindow_AutoIndex/TestReduceIntent_AddWindow_AIRequiresName/TestSSOTAICommandRoutesFromDesiredAISession/TestRealOpsSpawnAISendsAICommand/TestHumanE2ESSOTAddWindowCreatesNextShellAndPlacesItSteps", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-13", Section: "§4.1/§10.6", Layer: "L0/L1/L2/L3/L4", Subject: "remove-window removes desired identity and closes/kills target + project keeps empty Windows on last removal", TestPath: "internal/reducer/reducer_v3_test.go internal/planner/planner_remove_window_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestReduceIntent_RemoveWindow/TestPlanner_RemoveWindow_EmitsCloseOpForRemovedShell/TestPlanner_RemoveWindow_LastWindowKeepsProject/TestHumanE2ESSOTRemoveWindowClosesAndKillsSessionSteps", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-14", Section: "§4.1/§10.6", Layer: "L1/L4", Subject: "browser add-tab routes URL through PrivatePayloadStore (controller Put → opaque token in DesiredWorld, raw URL never persisted)", TestPath: "internal/intent/ssot_l0_intent_test.go internal/controller/controller_browser_payload_test.go internal/reducer/reducer_browser_test.go cmd/projwm/ssot_cli_surface_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestSSOTUserOperationsHaveIntentSurface/TestControllerBrowserAddTab_RoutesURLThroughPayloadStore/TestControllerBrowserAddTab_NilPayloadStoreStoresLiteral/TestReduceIntent_BrowserAddTab_AppendsURL/TestReduceIntent_BrowserOps_RejectNonBrowserWindow/TestSSOTCLIExposesBrowserTabOperations/TestHumanE2ESSOTBrowserTabOperationsUpdatePrivateMetadataSteps", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-15", Section: "§4.1/§10.6", Layer: "L1/L4", Subject: "browser remove-tab Forgets removed ref via PrivatePayloadStore + reducer drops the entry (last-tab → window close path lands in S20 Step 3 observer)", TestPath: "internal/intent/ssot_l0_intent_test.go internal/controller/controller_browser_payload_test.go internal/reducer/reducer_browser_test.go cmd/projwm/ssot_cli_surface_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestSSOTUserOperationsHaveIntentSurface/TestControllerBrowserRemoveTab_ForgetsRemovedRef/TestReduceIntent_BrowserRemoveTab_RemovesAtIndex/TestReduceIntent_BrowserRemoveTab_OutOfRangeErrors/TestSSOTCLIExposesBrowserTabOperations/TestHumanE2ESSOTBrowserTabOperationsUpdatePrivateMetadataSteps", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-16", Section: "§4.1/§10.6", Layer: "L1/L4", Subject: "browser tab URL change rotates PrivatePayloadStore ref (Forget old + Put new) so DesiredWorld carries only opaque tokens", TestPath: "internal/intent/ssot_l0_intent_test.go internal/controller/controller_browser_payload_test.go internal/reducer/reducer_browser_test.go cmd/projwm/ssot_cli_surface_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestSSOTUserOperationsHaveIntentSurface/TestControllerBrowserChangeTabURL_RotatesPayload/TestReduceIntent_BrowserChangeTabURL_Replaces/TestSSOTCLIExposesBrowserTabOperations/TestHumanE2ESSOTBrowserTabOperationsUpdatePrivateMetadataSteps", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-17", Section: "§4.1/§10.6", Layer: "L1/L4", Subject: "browser tab reorder rearranges refs without Put/Forget (idempotent reorder)", TestPath: "internal/intent/ssot_l0_intent_test.go internal/controller/controller_browser_payload_test.go internal/reducer/reducer_browser_test.go cmd/projwm/ssot_cli_surface_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestSSOTUserOperationsHaveIntentSurface/TestControllerBrowserReorderTabs_NoPayloadStoreCalls/TestReduceIntent_BrowserReorderTabs_MovesFromTo/TestReduceIntent_BrowserReorderTabs_SameFromToIsNoop/TestSSOTCLIExposesBrowserTabOperations/TestHumanE2ESSOTBrowserTabOperationsUpdatePrivateMetadataSteps", Status: statusRealOnly, Evidence: evidenceBehavior},

	// §4.2 / §10.6 system operations.
	{ID: "SYS-ALL", Section: "§4.2/§10.6", Layer: "L1/L2/L3/L4", Subject: "move-window-to-workspace, reorder-columns, close-window, spawn-*, kill-session", TestPath: "internal/planner internal/adapter/wm internal/adapter/session internal/ssottest/real_ops_coverage_test.go scenarios/ssot_l4_acceptance_coverage_test.go", TestName: "TestSSOTRealOpsCoverageGate/TestSSOTL4AcceptanceCoverageGate", Status: statusRed, Evidence: evidenceMixed},

	// §10.4 single-operation groups.
	{ID: "L2-S", Section: "§10.4", Layer: "L2", Subject: "spawn settle timeout deterministic harness S12-S13", TestPath: "internal/adapter/wm/ssot_l2_mock_executor_test.go", TestName: "TestSpawnSettleTimeoutProcessAlive/TestSpawnSettleTimeoutProcessDead", Status: statusCovered},
	{ID: "L2-M", Section: "§10.4", Layer: "L2", Subject: "move retry/focus-drift/vanished deterministic harness M3-M5", TestPath: "internal/adapter/wm/ssot_l2_mock_executor_test.go", TestName: "TestMoveToWorkspaceFocusDrift/TestMoveToWorkspaceRetry/TestMoveToWorkspaceWindowVanished", Status: statusCovered},
	{ID: "L2-C", Section: "§10.4", Layer: "L2", Subject: "close fallback/retry deterministic harness C2-C3", TestPath: "internal/adapter/wm/ssot_l2_mock_executor_test.go", TestName: "TestLifecycleRemovalFallbackCloseSurface/TestCloseWindowRetry", Status: statusCovered},
	{ID: "L2-F", Section: "§10.4", Layer: "L2", Subject: "focus command-order deterministic harness F5", TestPath: "internal/adapter/wm/ssot_l2_mock_executor_test.go internal/adapter/wm/sigwm_test.go", TestName: "TestFocusWindowNavigationBeforeFocus/TestSigWM_FocusWindow", Status: statusCovered, Evidence: evidenceBehavior},
	{ID: "L3-S", Section: "§10.4", Layer: "L3", Subject: "spawn real operation table S1-S11", TestPath: "internal/adapter/wm/ssot_l3_wm_real_ops_test.go", TestName: "TestRealOpsSpawnShell", Status: statusRed},
	{ID: "L3-M", Section: "§10.4", Layer: "L3", Subject: "move real operation table M1-M2", TestPath: "internal/adapter/wm/ssot_l3_wm_spec_test.go", TestName: "TestMoveToWorkspace/TestMoveToWorkspaceAlreadyOnTarget", Status: statusRed},
	{ID: "L3-R", Section: "§10.4", Layer: "L3", Subject: "reorder real operation table R1-R4", TestPath: "internal/adapter/wm/ssot_l3_wm_spec_test.go", TestName: "TestReorderColumns/TestReorderColumnsAlreadyCorrect/TestReorderColumnsPartialMatch/TestReorderColumnsEmptyWorkspace", Status: statusRed},
	{ID: "L3-C", Section: "§10.4", Layer: "L3", Subject: "close real operation table C1/C4/C5", TestPath: "internal/adapter/wm/ssot_l3_wm_spec_test.go", TestName: "TestLifecycleRemovalPrimaryCloseSurfaces/TestCloseWindowAlreadyGone/TestCloseCockpit", Status: statusRed},
	{ID: "L3-F", Section: "§10.4", Layer: "L3", Subject: "focus real operation table F1-F4", TestPath: "internal/adapter/wm/ssot_l3_wm_real_ops_test.go internal/adapter/wm/ssot_l3_wm_spec_test.go", TestName: "TestRealOpsFocusWorkspace/TestRealOpsFocusWorkspaceNonExistent/TestFocusWindow/TestFocusWindowVanished", Status: statusRed},
	{ID: "L3-I", Section: "§10.4", Layer: "L3", Subject: "identity restoration table I1-I3 (I3 shell-only t.Skip — omniwm app-rule set in modules/darwin/omniwm/app-rules.nix catalogs only managed-title Ghostty windows; a non-managed `random-window-*` is filtered and never appears in omniwmctl query, so the L3 row cannot be realised. The pure-function contract for unknown titles is covered exhaustively at L0 by ssot_l0_identity_test.go.)", TestPath: "internal/naming/ssot_l3_identity_real_ops_test.go", TestName: "TestIdentityFromTitle/TestIdentityFromTitleViewer/TestIdentityFromTitleUnknown", Status: statusCovered},
	{ID: "L3-T", Section: "§10.4", Layer: "L3", Subject: "tmux operation table T1-T4", TestPath: "internal/adapter/session/ssot_l3_session_real_ops_test.go", TestName: "TestRealOpsTmuxEnsureSession/TestRealOpsTmuxEnsureSessionAlreadyExists/TestRealOpsTmuxEnsureGroupedSession/TestRealOpsTmuxKillSession", Status: statusCovered},
	{ID: "L3-B", Section: "§10.4", Layer: "L3", Subject: "startup recovery table B1-B4", TestPath: "internal/ssottest/real_ops_coverage_test.go", TestName: "TestSSOTRealOpsCoverageGate", Status: statusRed, Evidence: evidenceMeta},
	{ID: "L3-U", Section: "§10.4", Layer: "L3", Subject: "scratch shell show/hide real operation U1", TestPath: "internal/adapter/wm/ssot_l3_wm_spec_test.go", TestName: "TestScratchShellShowHideRestoresPriorFocus", Status: statusRed},

	// §7.5 WindowManagerAdapter contract: missing methods filled by S27.
	{ID: "WM-MOVECP", Section: "§7.5/§3.4 INV-06 / §10.9 GAP-20", Layer: "L1/L2", Subject: "MoveCockpitToParkWorkspace adapter method (cockpit park-workspace invariant repair)", TestPath: "internal/adapter/wm/fake_test.go internal/adapter/wm/ssot_l2_mock_executor_test.go", TestName: "TestFakeMoveCockpitToParkWorkspaceMovesWindow/TestFakeMoveCockpitToParkWorkspaceUnknownWindowErrors/TestMoveCockpitToParkWorkspaceCommandSequence/TestMoveCockpitToParkWorkspaceUnknownParkErrors", Status: statusCovered, Evidence: evidenceBehavior},
	{ID: "WM-SHOWSCRATCH", Section: "§7.5/§4.1 OP11", Layer: "L1/L2", Subject: "ShowScratchShell adapter method (idempotent global scratch spawn/focus)", TestPath: "internal/adapter/wm/fake_test.go internal/adapter/wm/ssot_l2_mock_executor_test.go internal/adapter/wm/ssot_l3_wm_spec_test.go", TestName: "TestFakeShowScratchShellIsIdempotent/TestFakeShowScratchShellFocusesIt/TestShowScratchShellExistingWindowReusesIt/TestShowScratchShellSpawnsWhenAbsent/TestScratchShellShowHideRestoresPriorFocus", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "WM-HIDESCRATCH", Section: "§7.5/§4.1 OP11", Layer: "L1/L2", Subject: "HideScratchShell adapter method (focus restore to prior window)", TestPath: "internal/adapter/wm/fake_test.go internal/adapter/wm/ssot_l2_mock_executor_test.go internal/adapter/wm/ssot_l3_wm_spec_test.go", TestName: "TestFakeHideScratchShellRestoresPriorFocus/TestFakeHideScratchShellEmptyPriorIsNoop/TestHideScratchShellNavigatesAndFocusesPrior/TestHideScratchShellEmptyPriorIsNoop/TestScratchShellShowHideRestoresPriorFocus", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "NAMI-03", Section: "§7.3", Layer: "L1/L2", Subject: "cockpit ghostty title uses projwm-cockpit-<display> (no D prefix)", TestPath: "internal/adapter/wm/sigwm_test.go internal/planner/planner_cockpit_test.go internal/reducer/reducer_cockpit_test.go", TestName: "TestSigWM_Close_CockpitBypassesBlock", Status: statusCovered, Evidence: evidenceBehavior},

	// §9.1 L4 acceptance.
	{ID: "ACC-S1", Section: "§9.1", Layer: "L4", Subject: "SwitchProfile: close old windows, summon new", TestPath: "scenarios/ssot_l4_acceptance_coverage_test.go", TestName: "TestSSOTL4AcceptanceCoverageGate", Status: statusRed, Evidence: evidenceMeta},
	{ID: "ACC-S2", Section: "§9.1", Layer: "L4", Subject: "ArchiveProject: close windows and kill tmux", TestPath: "scenarios/ssot_l4_acceptance_coverage_test.go", TestName: "TestSSOTL4AcceptanceCoverageGate", Status: statusRed, Evidence: evidenceMeta},
	{ID: "ACC-S3", Section: "§9.1", Layer: "L4", Subject: "UnarchiveProject: returns to park state", TestPath: "scenarios/ssot_l4_acceptance_coverage_test.go", TestName: "TestSSOTL4AcceptanceCoverageGate", Status: statusRed, Evidence: evidenceMeta},
	{ID: "ACC-S4", Section: "§9.1", Layer: "L4", Subject: "Assign/Unassign: slot assignment and release", TestPath: "scenarios/ssot_l4_acceptance_coverage_test.go", TestName: "TestSSOTL4AcceptanceCoverageGate", Status: statusRed, Evidence: evidenceMeta},
	{ID: "ACC-S5", Section: "§9.1", Layer: "L4", Subject: "Reconcile: repairs diff", TestPath: "scenarios/ssot_l4_acceptance_coverage_test.go", TestName: "TestSSOTL4AcceptanceCoverageGate", Status: statusRed, Evidence: evidenceMeta},
	{ID: "ACC-S6", Section: "§9.1", Layer: "L4", Subject: "macOS restart recovery", TestPath: "scenarios/ssot_l4_acceptance_coverage_test.go", TestName: "TestSSOTL4AcceptanceCoverageGate", Status: statusRed, Evidence: evidenceMeta},
	{ID: "ACC-S7", Section: "§9.1", Layer: "L4", Subject: "OmniWM restart recovery", TestPath: "scenarios/ssot_l4_acceptance_coverage_test.go", TestName: "TestSSOTL4AcceptanceCoverageGate", Status: statusRed, Evidence: evidenceMeta},
	{ID: "ACC-S8", Section: "§9.1", Layer: "L4", Subject: "summon idempotency", TestPath: "scenarios/ssot_l4_acceptance_coverage_test.go", TestName: "TestSSOTL4AcceptanceCoverageGate", Status: statusRed, Evidence: evidenceMeta},
	{ID: "ACC-S9", Section: "§9.1", Layer: "L4", Subject: "drift repair from outside slot", TestPath: "scenarios/ssot_l4_acceptance_coverage_test.go", TestName: "TestSSOTL4AcceptanceCoverageGate", Status: statusRed, Evidence: evidenceMeta},
	{ID: "ACC-S10", Section: "§9.1", Layer: "L4", Subject: "tmux/Ghostty/Zed crash recovery", TestPath: "scenarios/ssot_l4_acceptance_coverage_test.go", TestName: "TestSSOTL4AcceptanceCoverageGate", Status: statusRed, Evidence: evidenceMeta},

	// Phase 5 S26-S29 horizontal coverage.
	{ID: "COMMIT-FAIL-NOTIFY", Section: "§7.1 / §10.9 GAP-19 (+15/16/17/18)", Layer: "L1", Subject: "any commit-fail path (max-replans-exceeded, executor-error, settler-error, planner-error) MUST: (1) refuse commit, (2) rollback Desired while preserving ActiveCards + DirtyScopes, (3) append [INVARIANT] card with NoCommitReason, (4) record global dirty scope so next intent retries", TestPath: "internal/controller/controller_max_replans_test.go internal/controller/controller_test.go scenarios/transaction_contract_test.go", TestName: "TestSSOTSection71_CommitFailEmitsCardAndDirtyScope/TestControllerRollsBackMemoryOnInvariantFailure/TestControllerMarksDirtyWhenFailureRefreshObserveFails/TestTransactionContractS8C_VerifierReplanGating", Status: statusCovered, Evidence: evidenceBehavior},
	{ID: "STORE-CRASH-SAFE", Section: "§8.1/§8.3 / §10.9 GAP-22 GAP-23", Layer: "L1", Subject: "FileStore concurrent writer serialization (flock); interrupted write preserves prior CURRENT; reader sees prior generation during staging; Abort cleans staging dir", TestPath: "internal/store/file_crash_safe_test.go", TestName: "TestFileStoreConcurrentWritersAreSerialized/TestFileStoreInterruptedWriteLeavesCurrentPointingAtPriorGeneration/TestFileStoreReaderSeesPriorGenerationDuringStaging/TestFileStoreStagingDirCleanedByAbort", Status: statusCovered, Evidence: evidenceBehavior},

	{ID: "ORDER-PHASE", Section: "§6.10 / §7.1 / §10.9 GAP-18", Layer: "L1", Subject: "planner emits ops in phase order removals → observe-barrier → spawns → observe-barrier → layout, so closed slots are vacated (and observed gone) before spawns, and spawned windows observed before move/reorder; archive close→kill ordering via scenarios; settle→verify pipeline via transaction contract", TestPath: "internal/planner/planner_test.go scenarios/transaction_contract_test.go", TestName: "TestPlanPhaseOrderRemovalBarrierSpawnBarrierLayout/TestTransactionContractS8C_VerifierReplanGating", Status: statusCovered, Evidence: evidenceBehavior},

	{ID: "HIER-PRIORITY", Section: "§6.3 / §10.9 GAP-15", Layer: "L1", Subject: "state hierarchy L1(identity:spawn/close) > L2(placement:move) > L3(ordering:reorder): a single reconcile needing all three emits every spawn before every move and every move before every reorder; ordering (L3) is deferred when any window in the workspace is still missing (L1 unresolved)", TestPath: "internal/planner/planner_test.go", TestName: "TestPlanHierarchyL1BeforeL2BeforeL3/TestPlanHierarchyDefersOrderingUntilIdentityResolved", Status: statusCovered, Evidence: evidenceBehavior},

	{ID: "DRIFT-NOTIFY", Section: "§4.3 / §10.9 GAP-04", Layer: "L1", Subject: "manual drift is auto-corrected with post-hoc cockpit cards: user-close emits [CLOSED] card, user cross-workspace move emits [MOVED] card; grace period — 2 closes of the same window within 60s makes the 3rd emit a rateLimited=true warning card + user-close-suppress DirtyScope (reducer side) and the planner suppresses the respawn (planner T4.4)", TestPath: "internal/reducer/reducer_tier_test.go internal/planner/planner_rate_limit_test.go", TestName: "TestReactToEvent_Tier4_MovedCardEmit/TestReactToEvent_Tier4_ClosedCardEmit/TestReactToEvent_Tier4_GracePeriodSuppressesAndWarns/TestPlanner_T4_4_SuppressesRespawnAfterTwoCloses/TestPlanner_T4_4_OneCloseDoesNotSuppress/TestPlanner_T4_4_StaleClosesDoNotSuppress", Status: statusCovered, Evidence: evidenceBehavior},

	{ID: "ORPHAN-ACTION", Section: "§4.3 / §10.9 GAP-05", Layer: "L1", Subject: "orphan suggestion UI: promoted [NEW] orphan card carries Enter(adopt)/c(close)/t(carry-to-TUI) actions; AdoptOrphanWindow [Enter] appends a managed DesiredWindow under the target project (rematched next reconcile) and rejects unknown project; DismissOrphanWindow [c] leaves DesiredWorld untouched (close happens at controller/executor)", TestPath: "internal/controller/controller_cards_test.go internal/reducer/reducer_v3_test.go", TestName: "TestPromoteOrphans_CardCarriesActionSet/TestReduceIntent_AdoptOrphanWindow_AppendsDesiredWindow/TestReduceIntent_AdoptOrphanWindow_UnknownProjectRejects/TestReduceIntent_DismissOrphanWindow_NoDesiredMutation", Status: statusCovered, Evidence: evidenceBehavior},

	{ID: "GRACEFUL-DEGRADE", Section: "§6.8 / §10.9 GAP-17", Layer: "L1", Subject: "graceful degradation: a single per-window spawn (terminal/editor/browser/viewer) failure does NOT abort the transaction — the controller surfaces a per-window degraded [INVARIANT] card, continues remaining ops so other windows still spawn (§6.8①③), and the still-missing window rejoins the replan path, falling through to §7.1 max-replans if it never recovers while healthy windows persist; removal/layout failures keep hard-abort (executor-error) because §6.10 ordering depends on them", TestPath: "internal/controller/controller_max_replans_test.go internal/controller/controller_test.go", TestName: "TestSSOTSection68_GracefulDegradation_OneSpawnFailOthersContinue/TestControllerRecordsNoCommitTraceOnExecutorError/TestControllerMarksDirtyWhenFailureRefreshObserveFails", Status: statusCovered, Evidence: evidenceBehavior},

	// Phase 4 S21-S25 cockpit TUI + status/doctor + error-notification audits.
	{ID: "TUI-SNAPSHOT", Section: "§5.4 / §10.9 GAP-11", Layer: "L0/L1", Subject: "cockpit TUI topbar + 5 tabs render-shape audit (gen/epoch/profile/convergence/cards, Slots/Cards/Archived/Profiles/Trace tab labels, active-tab highlight)", TestPath: "cmd/projwm-cockpit/tui/view_snapshot_test.go", TestName: "TestSSOTSection54_TopbarContainsAllRequiredFields/TestSSOTSection54_TabBarShowsAllFiveTabs/TestSSOTSection54_CardsTabLabelShowsCount/TestSSOTSection54_ActiveTabIsHighlightedOnTabBar/TestSSOTSection54_SlotsTabShowsAssignmentsAndPark/TestSSOTSection54_ArchivedTabShowsArchivedProjects/TestSSOTSection54_ProfilesTabShowsAllProfilesWithActiveMarker", Status: statusCovered, Evidence: evidenceBehavior},
	{ID: "CARD-OMNIWM-RECOVERY", Section: "§5.4 cards 6 種", Layer: "L1", Subject: "OMNIWM-RECOVERY card surface (Controller.EmitOmniwmRecoveryCard) wired into runOmniwmRecovery self-heal ladder (Lv1/Lv2/Lv3) so cockpit reflects each ladder step", TestPath: "internal/controller/controller_omniwm_recovery_card_test.go", TestName: "TestEmitOmniwmRecoveryCard_AppendsActiveCard/TestEmitOmniwmRecoveryCard_OmitsEmptyDetail/TestEmitOmniwmRecoveryCard_MultipleLadderStepsAccumulate", Status: statusCovered, Evidence: evidenceBehavior},
	{ID: "STATUS-CONVERGENCE-DIGEST", Section: "§5.6 / §10.9 GAP-13", Layer: "L1", Subject: "projwm status --json surfaces Convergence (store-derived CONVERGED/CONVERGING) and ManifestDigest (OK/MISMATCH/UNCHECKED) per SSOT §5.6 items #8 and #9", TestPath: "cmd/projwm/cmd_status_test.go", TestName: "TestCmdStatus_JSON", Status: statusCovered, Evidence: evidenceBehavior},
	{ID: "ERROR-NOTIF-SURFACES", Section: "§5.5", Layer: "L0", Subject: "SSOT §5.5 error notification surfaces (cockpit card / topbar convergence / doctor PASS-WARN-FAIL) all present; macOS notification surfaces absent", TestPath: "internal/ssottest/ssot_section_audit_test.go", TestName: "TestSSOTSection55ErrorNotificationSurfacesExist/TestSSOTSection55NoMacOSNotificationUsage", Status: statusCovered, Evidence: evidenceBehavior},

	// S20 Step 3: browser tab observer + Vivaldi health probe + orphan GC.
	{ID: "BR-TAB-OBS", Section: "§4.4 BR-TAB-OBS / §4.1 OP14-17 / §10.9 GAP09", Layer: "L0/L1", Subject: "BrowserTabsSync polls Vivaldi automation profile per-window tabs, diffs snapshots, emits granular BrowserAddTab/RemoveTab/ChangeTabURL intents; skips User-profile windows; non-fatal on InspectTabsByWindow error; purges snapshot of disappeared windows", TestPath: "internal/adapter/browser/inspect_tabs_by_window_test.go internal/adapter/observer/browser_tabs_test.go", TestName: "TestParseInspectTabsByWindow_HappyPath/TestParseInspectTabsByWindow_EmptyInput/TestParseInspectTabsByWindow_UnmanagedWindowKept/TestParseInspectTabsByWindow_WindowWithoutTabs/TestParseInspectTabsByWindow_MalformedRecordSkipped/TestBrowserTabsSync_FirstObservationDoesNotEmit/TestBrowserTabsSync_EmitsBrowserAddTabOnAppendedURL/TestBrowserTabsSync_EmitsBrowserRemoveTabAtPosition/TestBrowserTabsSync_EmitsBrowserChangeTabURLOnURLEdit/TestBrowserTabsSync_SkipsUserProfileWindow/TestBrowserTabsSync_InspectErrorIsNonFatal/TestBrowserTabsSync_ResetsErrorCountOnRecovery/TestBrowserTabsSync_PurgesSnapshotForDisappearedWindow/TestParseBrowserTitle/TestDiffTabs", Status: statusCovered, Evidence: evidenceBehavior},
	{ID: "PRIV-ORPHAN-GC", Section: "§4.4 BR-PRIV-NOSTORE / §4.5 ARCHIVE", Layer: "L1", Subject: "Controller post-commit GC: any PrivatePayloadStore ref dropped from DesiredWorld (archived projects, removed browser windows) is Forgotten so on-disk payload files do not leak; unrelated projects' refs are preserved", TestPath: "internal/controller/controller_browser_payload_test.go", TestName: "TestControllerArchiveProject_ForgetsOrphanedBrowserPayloads/TestControllerArchiveProject_DoesNotForgetUnrelatedProjectRefs", Status: statusCovered, Evidence: evidenceBehavior},

	// S20 Step 2 robustness: projwmevent at-least-once delivery via
	// on-disk queue + retry. Guarantees no event loss when projwmd is
	// transiently down (launchd KeepAlive restart within ThrottleInterval).
	{ID: "ROBUST-EVENT-QUEUE", Section: "§3.5/§7.1 (robustness)", Layer: "L0/L1", Subject: "projwmevent at-least-once delivery: retry 3x with backoff, on total failure persist to event-queue dir; next invocation drains queue before submitting fresh event (no event loss across projwmd restart window)", TestPath: "internal/eventqueue/queue_test.go cmd/projwmevent/main_test.go", TestName: "TestQueue_EnqueueThenFlushDrainsInOrder/TestQueue_FlushStopsAndPreservesFailingRecord/TestQueue_FlushDropsPoisonRecord/TestQueue_FlushIgnoresTmpFiles/TestSubmitWithRetry_SucceedsAfterTransientFailures/TestSubmitWithRetry_StopsAfterMaxAttempts/TestRunMain_QueuesEventWhenSubmitFails/TestRunMain_DrainsQueueOnSuccess/TestRunMain_PartialDrainPreservesPendingAndQueuesNew", Status: statusCovered, Evidence: evidenceBehavior},

	// §10.8 test environment isolation.
	// statusRed because the audit currently allowlists two pre-existing
	// L4 acceptance files (scenarios/real_acceptance_test.go and
	// scenarios/ssot_real_acceptance_test.go) that hard-code "dotfiles"
	// / "manaflow" as test-daemon project IDs. The audit prevents NEW
	// violations from creeping in; promotion to statusCovered requires
	// rewriting those two files to use projwm-next-test-* (tracked under
	// slice S29 in queue/ssot-slice-plan.md).
	{ID: "ISO-01", Section: "§10.8/§10.9 GAP-26", Layer: "L0", Subject: "L3/L4 tests use projwm-next-test prefix for project IDs, sessions, and titles", TestPath: "internal/ssottest/test_isolation_audit_test.go", TestName: "TestSSOTTestIsolationAuditEnforcesPrefixes", Status: statusRed, Evidence: evidenceBehavior},
}

func TestSSOTLedgerItemsAreExplicitlyClassified(t *testing.T) {
	if len(ssotLedger) == 0 {
		t.Fatal("SSOT ledger is empty")
	}
	seen := map[string]bool{}
	for _, item := range ssotLedger {
		if item.ID == "" || item.Section == "" || item.Layer == "" || item.Subject == "" {
			t.Fatalf("ledger item must identify source and subject: %+v", item)
		}
		if seen[item.ID] {
			t.Fatalf("duplicate ledger item %q", item.ID)
		}
		seen[item.ID] = true
		switch item.Status {
		case statusCovered, statusPartial, statusRed, statusMissing, statusRealOnly:
		default:
			t.Fatalf("ledger item %s has unknown status %q", item.ID, item.Status)
		}
		switch item.evidenceKind() {
		case evidenceBehavior, evidenceMeta, evidenceMixed, evidenceConflict:
		default:
			t.Fatalf("ledger item %s has unknown evidence kind %q", item.ID, item.Evidence)
		}
		if item.Status != statusMissing && (item.TestPath == "" || item.TestName == "") {
			t.Fatalf("non-missing ledger item must name test path and test name: %+v", item)
		}
	}
}

func TestSSOTLedgerSeparatesMetaAndBehaviorEvidence(t *testing.T) {
	for _, item := range ssotLedger {
		evidence := item.evidenceKind()
		if referencesMetaOnlyTest(item) && evidence == evidenceBehavior {
			t.Fatalf("%s cites meta-only gate(s) as behavior evidence: %s", item.ID, item.TestName)
		}
		if strings.Contains(item.TestName, "*") {
			t.Fatalf("%s uses wildcard test owner instead of explicit behavior evidence: %s", item.ID, item.TestName)
		}
		switch item.Status {
		case statusCovered, statusRealOnly:
			if evidence != evidenceBehavior {
				t.Fatalf("%s is %s but evidence is %s; covered rows must cite behavior owners only", item.ID, item.Status, evidence)
			}
			if referencesMetaOnlyTest(item) {
				t.Fatalf("%s is %s but cites meta-only gate(s): %s", item.ID, item.Status, item.TestName)
			}
		}
	}
}

func TestSSOTLedgerHasNoMissingOrPartialCoverage(t *testing.T) {
	var missing, partial, red int
	for _, item := range ssotLedger {
		switch item.Status {
		case statusMissing:
			missing++
		case statusPartial:
			partial++
		case statusRed:
			red++
		}
	}
	if missing > 0 || partial > 0 {
		t.Fatalf("SSOT ledger coverage is incomplete: missing=%d partial=%d red=%d", missing, partial, red)
	}
	t.Logf("SSOT ledger has no missing/partial rows; red rows remain explicit: missing=%d partial=%d red=%d", missing, partial, red)
}

func TestSSOTLedgerReferencesExistingTestFunctions(t *testing.T) {
	funcs := ledgerTestFunctions(t)
	for _, item := range ssotLedger {
		if item.Status == statusMissing {
			continue
		}
		for _, name := range strings.Split(item.TestName, "/") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if !funcs[name] {
				t.Fatalf("%s references missing test function %s", item.ID, name)
			}
		}
	}
}

func referencesMetaOnlyTest(item ledgerItem) bool {
	for _, marker := range []string{
		"TestSSOTCoverageDeclares",
		"TestSSOTRealOpsCoverageGate",
		"TestSSOTRealOpsCoverageReferencesExistingTestFunctions",
		"TestSSOTAcceptanceCoverageGate",
		"TestSSOTL4AcceptanceCoverageGate",
		"TestSSOTL4AcceptanceCoverageReferencesExistingTestFunctions",
		"TestSSOTRequiredLayerMatrix",
	} {
		if strings.Contains(item.TestName, marker) {
			return true
		}
	}
	return false
}

func ledgerTestFunctions(t *testing.T) map[string]bool {
	t.Helper()
	funcs := map[string]bool{}
	root := "../.."
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}
			funcs[fn.Name.Name] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return funcs
}
