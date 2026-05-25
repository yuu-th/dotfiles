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
	{ID: "INV-01", Section: "§3.4", Layer: "L0/L1", Subject: "same (project, kind, id) window is globally unique", TestPath: "internal/planner internal/identity", TestName: "TestPlanRejectsAmbiguousActiveDesiredWindow", Status: statusCovered},
	{ID: "INV-02", Section: "§3.4", Layer: "L0/L1", Subject: "active profile windows are on the correct slot", TestPath: "internal/planner/ssot_l0_planner_test.go", TestName: "TestSSOTDriftedActiveWindowPlansMoveNotRespawn", Status: statusCovered},
	{ID: "INV-03", Section: "§3.4", Layer: "L2/L3", Subject: "tmux session is recreated if absent", TestPath: "internal/adapter/session/tmux_test.go internal/adapter/session/ssot_l3_session_real_ops_test.go", TestName: "TestEnsureSessionCreates/TestRealOpsTmuxEnsureSession", Status: statusCovered},
	{ID: "INV-04", Section: "§3.4", Layer: "L0/L1", Subject: "archived project windows do not exist", TestPath: "internal/invariant/ssot_l1_invariant_test.go scenarios/archive_project_test.go", TestName: "TestSSOTInvariantArchivedProjectMustHaveNoLiveWindows", Status: statusCovered},
	{ID: "INV-05", Section: "§3.4", Layer: "L0/L1", Subject: "viewer shows only active profile AI streams", TestPath: "internal/invariant/ssot_l1_invariant_test.go internal/planner", TestName: "TestSSOTInvariantViewerShowsOnlyActiveProfileAIStreams/TestPlanDefersViewerSpawnUntilSourceAIIsObservedUniqueStrong", Status: statusCovered},
	{ID: "INV-06", Section: "§3.4", Layer: "L0/L1/L3", Subject: "cockpit always exists on park workspace CP1", TestPath: "internal/reducer/reducer_cockpit_test.go internal/planner/planner_cockpit_test.go internal/adapter/wm/ssot_l3_wm_spec_test.go", TestName: "TestReduceIntent_SyncCockpit_AlwaysOneEntry_RegardlessOfDisplayCount/TestReduceIntent_SyncCockpit_SetsParkWorkspace/TestPlanner_Cockpit_SpawnWhenMissing/TestPlanner_Cockpit_DisplayMappingViaParkWorkspace/TestSpawnCockpit", Status: statusRed},
	{ID: "INV-07", Section: "§3.4", Layer: "L0/L3", Subject: "Zed title equals basename(cwd)", TestPath: "internal/naming/ssot_l0_identity_test.go internal/adapter/zed/zed_test.go", TestName: "TestSSOTZedTitleIsProjectRootBasename/TestCollectCloseObservationGathersPresenceAndProjectRootCorrelation", Status: statusCovered},
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
	{ID: "OP-08", Section: "§4.1", Layer: "L0/L1/L4", Subject: "profile switch updates active profile and closes/spawns via transaction", TestPath: "internal/reducer/reducer_switch_profile_test.go scenarios/switch_profile_test.go scenarios/ssot_l4_acceptance_spec_test.go", TestName: "TestReduceIntent_SwitchProfile_FlipsActive/TestReduceIntent_SwitchProfile_PreservesBothAssignments/TestSwitchProfile/TestSSOTL4S1SwitchProfile", Status: statusRed},
	{ID: "OP-09", Section: "§4.1", Layer: "L0/L1/L4", Subject: "project add allocates first free slot and summons shell-1", TestPath: "cmd/projwm/ssot_cli_surface_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestSSOTCLIUpUsesCanonicalDefaultWindowTitles/TestHumanE2ESSOTProjectAddFirstFreeSlotSteps", Status: statusRed, Evidence: evidenceMixed},
	{ID: "OP-10", Section: "§4.1", Layer: "L0/L1/L4", Subject: "project archive removes assignment and closes windows / kills sessions", TestPath: "scenarios/archive_project_test.go internal/invariant/ssot_l1_invariant_test.go scenarios/ssot_l4_acceptance_spec_test.go", TestName: "TestArchiveProject/TestSSOTInvariantArchivedProjectMustHaveNoLiveWindows/TestSSOTL4S2ArchiveProject/TestSSOTL4S3UnarchiveProject", Status: statusRed},
	{ID: "OP-10B", Section: "§4.5", Layer: "L0", Subject: "unarchive updates state only and does not auto-assign a slot", TestPath: "internal/reducer/ssot_l0_state_test.go", TestName: "TestSSOTUnarchiveProjectReturnsToParkStateWithoutSlotAssignment", Status: statusCovered},
	{ID: "OP-11", Section: "§4.1/§10.6", Layer: "L1/L3/L4", Subject: "scratch shell open/close restores prior focus (intent → reducer → planner → executor → adapter end-to-end)", TestPath: "internal/intent/ssot_l0_intent_test.go internal/reducer/reducer_scratch_test.go internal/planner/planner_scratch_test.go internal/executor/executor_test.go internal/adapter/wm/ssot_l3_wm_spec_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestSSOTUserOperationsHaveIntentSurface/TestReduceIntent_ShowScratchShell_FreshBuild/TestReduceIntent_HideScratchShell_TogglesVisibility/TestPlanner_Scratch_ShowEmitsWhenNotFocused/TestPlanner_Scratch_HideEmitsWhenFocused/TestExecuteShowScratchShell/TestExecuteHideScratchShellRestoresPriorFocus/TestScratchShellShowReturnsLiveWindowID/TestHumanE2ESSOTScratchShellOpenCloseSteps", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-12", Section: "§4.1/§10.6", Layer: "L0/L1/L2/L3/L4", Subject: "add-window uses max id + 1 without hole reuse + AI-name routes send-keys command", TestPath: "internal/reducer/ssot_l0_state_test.go internal/reducer/reducer_v3_test.go internal/semop/ssot_l1_semop_test.go internal/adapter/wm/ssot_l3_wm_real_ops_test.go scenarios/ssot_real_acceptance_test.go", TestName: "TestSSOTWindowIndexAllocationNeverReusesDeletedIDs/TestReduceIntent_AddWindow_AutoIndex/TestReduceIntent_AddWindow_AIRequiresName/TestSSOTAICommandRoutesFromDesiredAISession/TestRealOpsSpawnAISendsAICommand/TestHumanE2ESSOTAddWindowCreatesNextShellAndPlacesItSteps", Status: statusRealOnly, Evidence: evidenceBehavior},
	{ID: "OP-13", Section: "§4.1/§10.6", Layer: "L0/L1/L2/L3/L4", Subject: "remove-window removes desired identity and closes/kills target", TestPath: "internal/reducer/reducer_v3_test.go scenarios/ssot_real_acceptance_test.go internal/ssottest/real_ops_coverage_test.go scenarios/ssot_l4_acceptance_coverage_test.go", TestName: "TestReduceIntent_RemoveWindow/TestHumanE2ESSOTRemoveWindowClosesAndKillsSessionSteps/TestSSOTRealOpsCoverageGate/TestSSOTL4AcceptanceCoverageGate", Status: statusRed, Evidence: evidenceMixed},
	{ID: "OP-14", Section: "§4.1/§10.6", Layer: "L1/L4", Subject: "browser add-tab appends tab to browser window", TestPath: "internal/intent/ssot_l0_intent_test.go cmd/projwm/ssot_cli_surface_test.go scenarios/ssot_real_acceptance_test.go scenarios/ssot_l4_acceptance_coverage_test.go", TestName: "TestSSOTUserOperationsHaveIntentSurface/TestSSOTCLIExposesBrowserTabOperations/TestHumanE2ESSOTBrowserTabOperationsUpdatePrivateMetadataSteps/TestSSOTL4AcceptanceCoverageGate", Status: statusRed, Evidence: evidenceMixed},
	{ID: "OP-15", Section: "§4.1/§10.6", Layer: "L1/L4", Subject: "browser remove-tab removes tab and closes window if last tab", TestPath: "internal/intent/ssot_l0_intent_test.go cmd/projwm/ssot_cli_surface_test.go scenarios/ssot_real_acceptance_test.go scenarios/ssot_l4_acceptance_coverage_test.go", TestName: "TestSSOTUserOperationsHaveIntentSurface/TestSSOTCLIExposesBrowserTabOperations/TestHumanE2ESSOTBrowserTabOperationsUpdatePrivateMetadataSteps/TestSSOTL4AcceptanceCoverageGate", Status: statusRed, Evidence: evidenceMixed},
	{ID: "OP-16", Section: "§4.1/§10.6", Layer: "L1/L4", Subject: "browser tab URL change is observed and stored privately", TestPath: "internal/intent/ssot_l0_intent_test.go cmd/projwm/ssot_cli_surface_test.go scenarios/ssot_real_acceptance_test.go scenarios/ssot_l4_acceptance_coverage_test.go", TestName: "TestSSOTUserOperationsHaveIntentSurface/TestSSOTCLIExposesBrowserTabOperations/TestHumanE2ESSOTBrowserTabOperationsUpdatePrivateMetadataSteps/TestSSOTL4AcceptanceCoverageGate", Status: statusRed, Evidence: evidenceMixed},
	{ID: "OP-17", Section: "§4.1/§10.6", Layer: "L1/L4", Subject: "browser tab reorder updates desired structure", TestPath: "internal/intent/ssot_l0_intent_test.go cmd/projwm/ssot_cli_surface_test.go scenarios/ssot_real_acceptance_test.go scenarios/ssot_l4_acceptance_coverage_test.go", TestName: "TestSSOTUserOperationsHaveIntentSurface/TestSSOTCLIExposesBrowserTabOperations/TestHumanE2ESSOTBrowserTabOperationsUpdatePrivateMetadataSteps/TestSSOTL4AcceptanceCoverageGate", Status: statusRed, Evidence: evidenceMixed},

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
	{ID: "L3-I", Section: "§10.4", Layer: "L3", Subject: "identity restoration table I1-I3", TestPath: "internal/naming/ssot_l3_identity_real_ops_test.go", TestName: "TestIdentityFromTitle/TestIdentityFromTitleViewer/TestIdentityFromTitleUnknown", Status: statusCovered},
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
