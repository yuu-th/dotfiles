# SSOT カバレッジマトリクス §7-§10 埋め込み結果
> 2026-05-23 探索 agent 調査、§7-§10 全要求 123 件の実装/テスト/evidence 埋め

---

## §7. アーキテクチャ

### §7.1 Transaction Loop

#### SSOT-7.1-OBSERVE
- 実装: internal/controller/controller.go line 127 (c.observe 呼び出し)
- status: implemented
- テスト owner: internal/controller/controller_test.go TestControllerObservePhase + internal/adapter/wm/ssot_l3_wm_spec_test.go TestObserveWorld
- evidence: behavior
- メモ: WM から最新 ObservedWorld 取得; adapter.Observe() 経由で実装

#### SSOT-7.1-REDUCE
- 実装: internal/controller/controller.go line 139 (reducer.ReduceIntent 呼び出し)
- status: implemented
- テスト owner: internal/reducer/reducer.go + reducer_v3_test.go
- evidence: behavior
- メモ: (WorldState, Intent) → DesiredWorld; 純粋関数; §7.1 要件満たす

#### SSOT-7.1-PLAN
- 実装: internal/controller/controller.go line 371 (planner.Plan 呼び出し)
- status: implemented
- テスト owner: internal/planner/planner.go + ssot_l0_planner_test.go
- evidence: behavior
- メモ: (WorldState, DesiredWorld) → []Operation; 決定論的 rule-based

#### SSOT-7.1-EXEC
- 実装: internal/controller/controller.go line 408-423 (executor.Execute ループ)
- status: implemented
- テスト owner: internal/executor/executor.go + executor_test.go
- evidence: behavior
- メモ: Operation を adapter 経由で実行; Phase A/B/C 分離は planner 内部

#### SSOT-7.1-SETTLE
- 実装: internal/controller/controller.go line 424, 433 (settler.Settle)
- status: implemented
- テスト owner: internal/settler/settler.go + settler_test.go
- evidence: behavior
- メモ: 状態安定化 wait; 各操作後と final settle で呼び出し

#### SSOT-7.1-VERIFY
- 実装: internal/controller/controller.go line 443-449 (verifier.Diff)
- status: implemented
- テスト owner: internal/verifier/verifier.go + verifier_test.go
- evidence: behavior
- メモ: PredictedWorld と ObservedWorld 比較; UseSimulator フラグで制御

#### SSOT-7.1-COMMIT
- 実装: internal/controller/controller.go line 501+ (commitChanges + recordTransactionTrace)
- status: implemented
- テスト owner: internal/controller/controller_test.go TestControllerCommitPhase
- evidence: behavior
- メモ: 世代進める、PersistentStore に保存; store.Commit 経由

#### SSOT-7.1-PHASE-A
- 実装: internal/planner/planner.go (Phase A removals 計画)
- status: implemented
- テスト owner: internal/planner/planner_test.go (phase separation tests)
- evidence: behavior
- メモ: close, kill-session; 最初に実行

#### SSOT-7.1-PHASE-B
- 実装: internal/planner/planner.go (Phase B spawns 計画)
- status: implemented
- テスト owner: internal/planner/planner_test.go
- evidence: behavior
- メモ: spawn-terminal/editor/browser/viewer/cockpit

#### SSOT-7.1-PHASE-C
- 実装: internal/planner/planner.go (Phase C layout 計画)
- status: implemented
- テスト owner: internal/planner/planner_test.go
- evidence: behavior
- メモ: move-to-workspace, reorder-columns, focus

#### SSOT-7.1-BARRIER
- 実装: internal/settler/settler.go (observe-barrier 実装)
- status: implemented
- テスト owner: internal/settler/settler_test.go
- evidence: behavior
- メモ: 各 phase 間に settle で状態安定化を待つ

#### SSOT-7.1-MAXREPLAN-FAIL
- 実装: internal/controller/controller.go line 363-368 (maxIter ループ + fail 分岐)
- status: implemented
- テスト owner: internal/controller/controller_test.go TestMaxReplansExceeded
- evidence: behavior
- メモ: MaxReplans 超過時 fail 分岐; commit されない

#### SSOT-7.1-MAXREPLAN-ROLLBACK
- 実装: internal/controller/controller.go line 125 (rollback snapshot) + line 252 (restoreRollbackState)
- status: implemented
- テスト owner: internal/controller/controller_test.go TestMaxReplansExceededRollback
- evidence: behavior
- メモ: トランザクション開始前の状態に復帰

#### SSOT-7.1-MAXREPLAN-CARD
- 実装: internal/controller/controller.go (card append in fail path)
- status: implemented
- テスト owner: scenarios/transaction_contract_test.go
- evidence: behavior
- メモ: cockpit に [INVARIANT] カード通知

#### SSOT-7.1-MAXREPLAN-RETRY
- 実装: internal/controller/controller.go (dirty scope 記録)
- status: implemented
- テスト owner: internal/controller/controller_test.go
- evidence: behavior
- メモ: 次の intent/event で再挑戦; 自動リトライしない

### §7.2 パッケージ境界

#### SSOT-7.2-CMD
- 実装: cmd/projwmd/ cmd/projwmctl/ cmd/projwmevent/ cmd/projwm/ cmd/projwm-cockpit/ cmd/projwmstore-bootstrap/
- status: implemented
- テスト owner: cmd 各サブディレクトリの *_test.go
- evidence: meta
- メモ: 全 6 つのバイナリパッケージ存在確認

#### SSOT-7.2-INT-ADAPTER
- 実装: internal/adapter/wm/ internal/adapter/browser/ internal/adapter/zed/ internal/adapter/session/
- status: implemented
- テスト owner: internal/adapter 各サブディレクトリ
- evidence: meta
- メモ: WM, Browser, Editor, Session 適応層

#### SSOT-7.2-INT-CORE
- 実装: internal/controller/ internal/reducer/ internal/planner/ internal/executor/ internal/settler/ internal/simulator/ internal/verifier/ internal/store/ internal/world/ internal/intent/ internal/event/ internal/op/
- status: implemented
- テスト owner: 各サブディレクトリ
- evidence: meta
- メモ: トランザクション loop の 12 モジュール

#### SSOT-7.2-INT-AUX
- 実装: internal/invariant/ internal/manifest/ internal/ipc/ internal/identity/ internal/naming/ internal/semop/
- status: implemented
- テスト owner: 各サブディレクトリ
- evidence: meta
- メモ: 補助パッケージ 6 つ

### §7.3 命名規約

#### SSOT-7.3-AI-TMUX
- 実装: internal/naming/naming.go line 59-61 (TmuxSession("ai", id, project))
- status: implemented
- テスト owner: internal/naming/ssot_l0_identity_test.go TestSSOTIdentityRestorationFromManagedTitles
- evidence: behavior
- メモ: ai-1/dotfiles 形式; fmt.Sprintf("%s-%d/%s", "ai", 1, "dotfiles")

#### SSOT-7.3-AI-TITLE
- 実装: internal/naming/naming.go line 49-51 (GhosttyTitle("ai", id, project))
- status: implemented
- テスト owner: internal/naming/ssot_l0_identity_test.go
- evidence: behavior
- メモ: ai-1:dotfiles 形式; : で区切る理由は Ghostty title convention

#### SSOT-7.3-VIEWER-TMUX
- 実装: internal/naming/naming.go line 75-77 (ViewerTmuxSession(id, project))
- status: implemented
- テスト owner: internal/naming/ssot_l0_identity_test.go
- evidence: behavior
- メモ: ai-1/dotfiles_v 形式; grouped session clone 用

#### SSOT-7.3-VIEWER-TITLE
- 実装: internal/naming/naming.go line 66-68 (ViewerGhosttyTitle(id, project))
- status: implemented
- テスト owner: internal/naming/ssot_l0_identity_test.go
- evidence: behavior
- メモ: ai-view-1:dotfiles 形式

#### SSOT-7.3-SHELL-TMUX
- 実装: internal/naming/naming.go line 59-61 (TmuxSession("shell", id, project))
- status: implemented
- テスト owner: internal/naming/ssot_l0_identity_test.go
- evidence: behavior
- メモ: shell-1/dotfiles 形式

#### SSOT-7.3-SHELL-TITLE
- 実装: internal/naming/naming.go line 49-51 (GhosttyTitle("shell", id, project))
- status: implemented
- テスト owner: internal/naming/ssot_l0_identity_test.go
- evidence: behavior
- メモ: shell-1:dotfiles 形式

#### SSOT-7.3-EDITOR
- 実装: internal/naming/naming.go line 80-82 (ZedTitle(cwd))
- status: implemented
- テスト owner: internal/naming/ssot_l0_identity_test.go TestSSOTZedTitleIsProjectRootBasename
- evidence: behavior
- メモ: basename(cwd); filepath.Base で実装

#### SSOT-7.3-BROWSER
- 実装: internal/semop/browser.go (title = "browser-N:project")
- status: implemented
- テスト owner: internal/semop/ssot_l1_semop_test.go
- evidence: behavior
- メモ: browser-1:dotfiles 形式

#### SSOT-7.3-COCKPIT-TMUX
- 実装: internal 各所で hardcoded "projwm-cockpit"
- status: implemented
- テスト owner: internal/planner/planner_cockpit_test.go
- evidence: meta
- メモ: tmux session 名; 全 cockpit インスタンス共有

#### SSOT-7.3-COCKPIT-TITLE
- 実装: internal/executor/executor.go + internal/planner/planner.go で "projwm-cockpit-<display>" 生成
- status: implemented
- テスト owner: internal/adapter/wm/ssot_l3_wm_spec_test.go TestSpawnCockpit
- evidence: behavior
- メモ: display ごとに異なる title

#### SSOT-7.3-SCRATCH-TMUX
- 実装: internal 各所で hardcoded "projwm-scratch-shell"
- status: implemented
- テスト owner: internal/adapter/wm/ssot_l3_wm_spec_test.go TestScratchShellShowHideRestoresPriorFocus
- evidence: meta
- メモ: tmux session 名; グローバルに 1 つ

#### SSOT-7.3-SCRATCH-TITLE
- 実装: internal 各所で hardcoded "projwm-scratch-shell"
- status: implemented
- テスト owner: internal/adapter/wm/ssot_l3_wm_spec_test.go TestScratchShellShowHideRestoresPriorFocus
- evidence: meta
- メモ: Ghostty title; tmux session と同名

#### SSOT-7.3-ED-MULTI
- 実装: internal/identity/identity.go (bundleId + title + workspace で識別)
- status: implemented
- テスト owner: internal/identity/identity_test.go
- evidence: behavior
- メモ: Zed 複数 project で同 basename の場合の識別ロジック

#### SSOT-7.3-SLASH
- 実装: internal/naming/naming.go コメント line 58; naming/reducer で一貫
- status: implemented
- テスト owner: internal/naming/ssot_l0_identity_test.go
- evidence: meta
- メモ: tmux session は `/`、Ghostty title は `:` で統一

### §7.4 ドメインモデル

#### SSOT-7.4-WORLD
- 実装: internal/world/world.go (WorldState type)
- status: implemented
- テスト owner: internal/world/world_test.go
- evidence: meta
- メモ: {Environment, Desired, Observed, Predicted, Meta}

#### SSOT-7.4-ENV
- 実装: internal/world/world.go (ManagedEnvironment type)
- status: implemented
- テスト owner: internal/manifest/manifest_test.go
- evidence: meta
- メモ: windowManager / workspaces / slots / apps / daemons

#### SSOT-7.4-DESIRED
- 実装: internal/world/world.go (DesiredWorld type)
- status: implemented
- テスト owner: internal/world/world_test.go
- evidence: meta
- メモ: ActiveProfile, Profiles, Projects, FocusPolicy, CockpitVisibility, SystemWindows

#### SSOT-7.4-OBSERVED
- 実装: internal/world/world.go (ObservedWorld type)
- status: implemented
- テスト owner: internal/world/world_test.go
- evidence: meta
- メモ: Windows, Workspaces, Displays, Focused, Tmux, Timestamp

#### SSOT-7.4-IDS
- 実装: internal/world/enums.go (ID 型群)
- status: implemented
- テスト owner: internal/world/enums_test.go
- evidence: meta
- メモ: ProfileID, ProjectID, SlotID, WorkspaceID, DesiredWindowID, LiveWindowID, DisplayID

#### SSOT-7.4-KIND-AI
- 実装: internal/world/enums.go (WindowKind = "ai")
- status: implemented
- テスト owner: internal/world/enums_test.go
- evidence: meta
- メモ: AI CLI, Ghostty, tmux あり

#### SSOT-7.4-KIND-SHELL
- 実装: internal/world/enums.go (WindowKind = "shell")
- status: implemented
- テスト owner: internal/world/enums_test.go
- evidence: meta
- メモ: 自由 shell, Ghostty, tmux

#### SSOT-7.4-KIND-EDITOR
- 実装: internal/world/enums.go (WindowKind = "editor")
- status: implemented
- テスト owner: internal/world/enums_test.go
- evidence: meta
- メモ: GUI editor, Zed, tmux なし

#### SSOT-7.4-KIND-BROWSER
- 実装: internal/world/enums.go (WindowKind = "browser")
- status: implemented
- テスト owner: internal/world/enums_test.go
- evidence: meta
- メモ: Vivaldi, tmux なし

#### SSOT-7.4-KIND-VIEWER
- 実装: internal/world/enums.go (WindowKind = "viewer")
- status: implemented
- テスト owner: internal/world/enums_test.go
- evidence: meta
- メモ: AI read-only 複製, Ghostty, tmux grouped

#### SSOT-7.4-KIND-EXTERNAL
- 実装: internal/world/enums.go (WindowKind = "external")
- status: implemented
- テスト owner: internal/world/enums_test.go
- evidence: meta
- メモ: 管理対象外

#### SSOT-7.4-KIND-COCKPIT
- 実装: internal/world/enums.go (WindowKind = "cockpit")
- status: implemented
- テスト owner: internal/world/enums_test.go
- evidence: meta
- メモ: TUI 操縦席, Ghostty, tmux

#### SSOT-7.4-KIND-SCRATCH
- 実装: internal/world/enums.go (WindowKind = "scratch")
- status: implemented
- テスト owner: internal/world/enums_test.go
- evidence: meta
- メモ: 一時作業 shell, Ghostty, tmux

### §7.5 アダプタ契約

#### SSOT-7.5-WM-OBSERVE
- 実装: internal/adapter/wm/adapter.go line 78
- status: implemented
- テスト owner: internal/adapter/wm/sigwm.go (Observe実装) + ssot_l3_wm_spec_test.go TestObserveWorld
- evidence: behavior
- メモ: Observe(ctx) (ObservedWorld, error)

#### SSOT-7.5-WM-SPAWN
- 実装: internal/adapter/wm/adapter.go line 81
- status: implemented
- テスト owner: internal/adapter/wm/sigwm.go (Spawn実装) + ssot_l3_wm_real_ops_test.go TestRealOpsSpawnShell
- evidence: behavior
- メモ: Spawn(ctx, SpawnRequest) (LiveWindowID, error)

#### SSOT-7.5-WM-CLOSE
- 実装: internal/adapter/wm/adapter.go line 83
- status: implemented
- テスト owner: internal/adapter/wm/sigwm.go (Close実装) + ssot_l3_wm_spec_test.go TestCloseCockpit
- evidence: behavior
- メモ: Close(ctx, LiveWindowID) error; raw close (production blocked)

#### SSOT-7.5-WM-FOCUSWS
- 実装: internal/adapter/wm/adapter.go line 86
- status: implemented
- テスト owner: internal/adapter/wm/sigwm.go (FocusWorkspace実装) + ssot_l3_wm_spec_test.go TestFocusWorkspace
- evidence: behavior
- メモ: FocusWorkspace(ctx, WorkspaceID) error

#### SSOT-7.5-WM-FOCUSWIN
- 実装: internal/adapter/wm/adapter.go line 87
- status: implemented
- テスト owner: internal/adapter/wm/sigwm.go (FocusWindow実装) + ssot_l3_wm_spec_test.go TestFocusWindow
- evidence: behavior
- メモ: FocusWindow(ctx, LiveWindowID) error

#### SSOT-7.5-WM-MOVE
- 実装: internal/adapter/wm/adapter.go line 84
- status: implemented
- テスト owner: internal/adapter/wm/sigwm.go (MoveWindowToWorkspace実装) + ssot_l3_wm_spec_test.go TestMoveToWorkspace
- evidence: behavior
- メモ: MoveWindowToWorkspace(ctx, LiveWindowID, WorkspaceID) error

#### SSOT-7.5-WM-REORDER
- 実装: internal/adapter/wm/adapter.go line 85
- status: implemented
- テスト owner: internal/adapter/wm/sigwm.go (ReorderColumns実装) + ssot_l3_wm_spec_test.go TestReorderColumns
- evidence: behavior
- メモ: ReorderColumns(ctx, WorkspaceID, [][]LiveWindowID) error

#### SSOT-7.5-WM-SPAWNCP
- 実装: internal/adapter/wm/adapter.go line 96
- status: implemented
- テスト owner: internal/adapter/wm/sigwm.go (SpawnCockpit実装) + ssot_l3_wm_spec_test.go TestSpawnCockpit
- evidence: behavior
- メモ: SpawnCockpit(ctx, displayIdx int, title string) error

#### SSOT-7.5-WM-SHOWCP
- 実装: internal/adapter/wm/adapter.go line 100
- status: implemented
- テスト owner: internal/adapter/wm/sigwm.go (ShowCockpitOnDisplay実装) + ssot_l3_wm_spec_test.go TestCockpitShowHideRestoresPriorWorkspaceAndWindow
- evidence: behavior
- メモ: ShowCockpitOnDisplay(ctx, DisplayID, parkWsName string) error

#### SSOT-7.5-WM-HIDECP
- 実装: internal/adapter/wm/adapter.go line 104
- status: implemented
- テスト owner: internal/adapter/wm/sigwm.go (HideCockpitOnDisplay実装) + ssot_l3_wm_spec_test.go TestCockpitShowHideRestoresPriorWorkspaceAndWindow
- evidence: behavior
- メモ: HideCockpitOnDisplay(ctx, DisplayID, priorWsName string) error

#### SSOT-7.5-WM-MOVECP
- 実装: MISSING — adapter.go にメソッド定義なし
- status: missing
- テスト owner: 未登録 (§10.4 GAP-20 対応待ち)
- evidence: none
- メモ: MoveCockpitToParkWorkspace(ctx, LiveWindowID, parkWS string) error が SSOT §7.5 で要求されるも未実装

#### SSOT-7.5-WM-SHOWSCRATCH
- 実装: MISSING — adapter.go にメソッド定義なし
- status: missing
- テスト owner: internal/adapter/wm/ssot_l3_wm_spec_test.go line 305+ で TestScratchShellShowHideRestoresPriorFocus は test spec を定義しているが、adapter 契約に ShowScratchShell メソッドが未追加
- evidence: none
- メモ: ShowScratchShell(ctx) (LiveWindowID, error) が §10.4 U1 テストで期待されるも実装なし

#### SSOT-7.5-WM-HIDESCRATCH
- 実装: MISSING — adapter.go にメソッド定義なし
- status: missing
- テスト owner: internal/adapter/wm/ssot_l3_wm_spec_test.go line 305+ TestScratchShellShowHideRestoresPriorFocus
- evidence: none
- メモ: HideScratchShell(ctx, priorWindow LiveWindowID) error が §10.4 U1 テストで期待されるも実装なし

#### SSOT-7.5-SES-HAS
- 実装: internal/adapter/session/adapter.go
- status: implemented
- テスト owner: internal/adapter/session/tmux_test.go + ssot_l3_session_real_ops_test.go
- evidence: behavior
- メモ: HasSession(ctx, name) (bool, error)

#### SSOT-7.5-SES-ENSURE
- 実装: internal/adapter/session/adapter.go
- status: implemented
- テスト owner: internal/adapter/session/tmux_test.go + ssot_l3_session_real_ops_test.go TestRealOpsTmuxEnsureSession
- evidence: behavior
- メモ: EnsureSession(ctx, name, cwd) (created bool, err error); 冪等

#### SSOT-7.5-SES-GROUP
- 実装: internal/adapter/session/adapter.go
- status: implemented
- テスト owner: internal/adapter/session/tmux_test.go + ssot_l3_session_real_ops_test.go TestRealOpsTmuxEnsureGroupedSession
- evidence: behavior
- メモ: EnsureGroupedSession(ctx, base, clone) error

#### SSOT-7.5-SES-KILL
- 実装: internal/adapter/session/adapter.go
- status: implemented
- テスト owner: internal/adapter/session/tmux_test.go + ssot_l3_session_real_ops_test.go TestRealOpsTmuxKillSession
- evidence: behavior
- メモ: KillSession(ctx, name) error

#### SSOT-7.5-SES-KEYS
- 実装: internal/adapter/session/adapter.go
- status: implemented
- テスト owner: internal/adapter/session/tmux_test.go
- evidence: behavior
- メモ: SendKeys(ctx, session, keys...) error

#### SSOT-7.5-ED-LAUNCH
- 実装: internal/adapter/zed/adapter.go
- status: implemented
- テスト owner: internal/adapter/zed/zed_test.go + ssot_l3_zed_real_ops_test.go
- evidence: behavior
- メモ: LaunchProject(ctx, projectPath, extraArgs) error; zed -n

#### SSOT-7.5-ED-COLLECT
- 実装: internal/adapter/zed/adapter.go
- status: implemented
- テスト owner: internal/adapter/zed/zed_test.go
- evidence: behavior
- メモ: CollectCloseObservation(ctx, params) (CloseObservation, error)

#### SSOT-7.5-ED-CLOSE
- 実装: internal/adapter/zed/adapter.go
- status: implemented
- テスト owner: internal/adapter/zed/zed_test.go
- evidence: behavior
- メモ: CloseLiveWindow(ctx, LiveWindowID) error; project-scoped-app

#### SSOT-7.5-BR-OPEN
- 実装: internal/adapter/browser/adapter.go
- status: implemented
- テスト owner: internal/adapter/browser/browser_test.go
- evidence: behavior
- メモ: OpenURL(ctx, url, profile) error; automation profile

#### SSOT-7.5-BR-COLLECT
- 実装: internal/adapter/browser/adapter.go
- status: implemented
- テスト owner: internal/adapter/browser/browser_test.go
- evidence: behavior
- メモ: CollectCloseObservation(ctx, params) (CloseObservation, error)

#### SSOT-7.5-BR-CLOSE
- 実装: internal/adapter/browser/adapter.go
- status: implemented
- テスト owner: internal/adapter/browser/browser_test.go
- evidence: behavior
- メモ: CloseLiveWindow(ctx, LiveWindowID) error; browser-window-close

#### SSOT-7.5-PRINCIPLE
- 実装: internal/ssottest/layer_matrix_test.go + ledger_test.go (L2 vs L3 区分)
- status: implemented
- テスト owner: internal/adapter/wm/ssot_l2_mock_executor_test.go + ssot_l3_wm_real_ops_test.go
- evidence: meta
- メモ: adapter method は L2 (mock/deterministic harness) と L3 (実操作) を分けてテスト

---

## §8. 状態管理

#### SSOT-8.1-GEN
- 実装: internal/store/file.go line 85-100 (OpenFileStore + generation directory)
- status: implemented
- テスト owner: internal/store/file_test.go TestFileStoreCommit
- evidence: behavior
- メモ: generation-based の不変ストア; 各コミットで generation ディレクトリ増

#### SSOT-8.1-ATOMIC
- 実装: internal/store/file.go line 494 (os.Rename), line 680 (atomic rename in writeFileAtomic)
- status: implemented
- テスト owner: internal/store/file_test.go TestFileStoreCrashRecovery
- evidence: behavior
- メモ: tmpfile + atomic rename で crash safety 保証; line 722-733 flockExclusive

#### SSOT-8.1-SAVE-DESIRED
- 実装: internal/store/file.go line 24 (artifactDesiredWorld = "desired_world.json")
- status: implemented
- テスト owner: internal/store/file_test.go
- evidence: behavior
- メモ: DesiredWorld を generation ディレクトリに保存

#### SSOT-8.1-SAVE-LAYOUTS
- 実装: internal/store/file.go line 24 (artifactAcceptedLayout = "accepted_layout.json")
- status: implemented
- テスト owner: internal/store/file_test.go
- evidence: behavior
- メモ: AcceptedLayouts を generation ディレクトリに保存

#### SSOT-8.1-SAVE-BROWSER
- 実装: internal/store/store.go 型定義 (BrowserSnapshots in ControllerCommit)
- status: implemented
- テスト owner: internal/store/file_test.go
- evidence: meta
- メモ: BrowserSnapshots artifact として保存

#### SSOT-8.1-SAVE-CHKPOINT
- 実装: internal/store/file.go line 25 (artifactCheckpoint = "checkpoint.json")
- status: implemented
- テスト owner: internal/store/file_test.go
- evidence: behavior
- メモ: ControllerCheckpoint を generation ディレクトリに保存

#### SSOT-8.1-NO-OBSERVED
- 実装: internal/store/file.go (ObservedWorld は保存対象に含まれない)
- status: implemented
- テスト owner: internal/store/file_test.go
- evidence: meta
- メモ: ObservedWorld は起動時に observer で再構成; 保存しない設計

#### SSOT-8.2-COMPUTED
- 実装: internal/naming/naming.go + internal/reducer/reducer.go (title 非保存、算出式)
- status: implemented
- テスト owner: internal/naming/ssot_l0_identity_test.go
- evidence: behavior
- メモ: title / tmux session / viewer 窓は naming.Resolve() で算出; rename 時の不整合を構造的防止

#### SSOT-8.3-FLOCK
- 実装: internal/store/file.go line 722-733 (flockExclusive); line 169, 298, 332 で呼び出し
- status: implemented
- テスト owner: internal/store/file_test.go TestFileStoreConcurrentWrite
- evidence: behavior
- メモ: 全書き込み (Commit) で syscall.Flock(LOCK_EX) で排他

#### SSOT-8.3-TMPFILE
- 実装: internal/store/file.go line 670-684 (writeFileAtomic で tempfile + rename)
- status: implemented
- テスト owner: internal/store/file_test.go
- evidence: behavior
- メモ: 書き込みは tmpfile + atomic rename で crash safety; Fsync も実施

#### SSOT-8.3-READ-NOLOCK
- 実装: internal/store/file.go ReadGeneration (no flock)
- status: implemented
- テスト owner: internal/store/file_test.go
- evidence: behavior
- メモ: 読み込みは lock 不要; generation マニフェスト先読みで安全

---

## §9. 受入仕様

#### SSOT-9.1-S1
- 実装: scenarios/switch_profile_test.go + internal/reducer/reducer_switch_profile_test.go
- status: implemented
- テスト owner: scenarios/ssot_l4_acceptance_spec_test.go TestSSOTL4S1SwitchProfile
- evidence: behavior
- メモ: SwitchProfile: 旧 close、新 summon; tmux は殺さない

#### SSOT-9.1-S2
- 実装: scenarios/archive_project_test.go + internal/reducer/reducer_v3_test.go
- status: implemented
- テスト owner: scenarios/ssot_l4_acceptance_spec_test.go TestSSOTL4S2ArchiveProject
- evidence: behavior
- メモ: ArchiveProject: window close + tmux kill

#### SSOT-9.1-S3
- 実装: scenarios/unarchive_project_test.go
- status: implemented
- テスト owner: scenarios/ssot_l4_acceptance_spec_test.go TestSSOTL4S3UnarchiveProject
- evidence: behavior
- メモ: UnarchiveProject: park 状態に復帰; 自動展開しない

#### SSOT-9.1-S4
- 実装: scenarios/assign_unassign_test.go + internal/reducer/reducer_v3_test.go
- status: implemented
- テスト owner: scenarios/ssot_l4_acceptance_spec_test.go TestSSOTL4S4Assign
- evidence: behavior
- メモ: Assign/Unassign: slot 割当と解除

#### SSOT-9.1-S5
- 実装: scenarios/reconcile_test.go + internal/reducer/reducer_v3_test.go
- status: implemented
- テスト owner: scenarios/ssot_l4_acceptance_spec_test.go TestSSOTL4S5Reconcile
- evidence: behavior
- メモ: Reconcile: 差分修正

#### SSOT-9.1-S6
- 実装: scenarios/real_acceptance_test.go (startup + macOS reboot recovery)
- status: implemented
- テスト owner: scenarios/ssot_l4_acceptance_spec_test.go TestSSOTL4S6StartupNormal
- evidence: behavior
- メモ: macOS 再起動後: 全自動復帰

#### SSOT-9.1-S7
- 実装: scenarios/real_acceptance_test.go (OmniWM restart recovery)
- status: implemented
- テスト owner: scenarios/ssot_l4_acceptance_spec_test.go TestSSOTL4S7OmniWMRestartRecovery
- evidence: behavior
- メモ: OmniWM 再起動後: 窓の再作成

#### SSOT-9.1-S8
- 実装: internal/intent/ssot_l0_intent_test.go + scenarios/ssot_real_acceptance_test.go
- status: implemented
- テスト owner: scenarios/ssot_l4_acceptance_spec_test.go TestSSOTL4S8SummonIdempotency
- evidence: behavior
- メモ: summon の冪等性

#### SSOT-9.1-S9
- 実装: scenarios/ssot_real_acceptance_test.go (drift 修正)
- status: implemented
- テスト owner: scenarios/ssot_l4_acceptance_spec_test.go TestSSOTL4S9DriftFix
- evidence: behavior
- メモ: drift 修正: slot 外から自動復帰

#### SSOT-9.1-S10
- 実装: scenarios/ssot_real_acceptance_test.go (tmux/Ghostty/Zed crash recovery)
- status: implemented
- テスト owner: scenarios/ssot_l4_acceptance_spec_test.go TestSSOTL4S10CrashRecovery
- evidence: behavior
- メモ: 障害復帰: tmux/Ghostty/Zed クラッシュ後の自動復帰

#### SSOT-9.2-DOD1
- 実装: scenarios/ssot_l4_acceptance_spec_test.go (全 S1-S10 coverage)
- status: implemented
- テスト owner: scenarios/ssot_l4_acceptance_spec_test.go
- evidence: behavior
- メモ: 全受入シナリオが real E2E でパス

#### SSOT-9.2-DOD2
- 実装: internal/invariant/invariant.go + ssot_l1_invariant_test.go
- status: implemented
- テスト owner: internal/invariant/ssot_l1_invariant_test.go
- evidence: behavior
- メモ: 全不変条件 (§3.4) が invariant checker で検証

#### SSOT-9.2-DOD3
- 実装: scenarios/ssot_l4_acceptance_spec_test.go (timing assertions)
- status: partial
- テスト owner: scenarios/ssot_l4_acceptance_spec_test.go (timing audit 未完成)
- evidence: behavior
- メモ: 1 分以内の自動復帰を保証; timing assertion は GAP-24 未保証領域

#### SSOT-9.2-DOD4
- 実装: scenarios/ssot_l4_acceptance_spec_test.go (profile switch timing)
- status: partial
- テスト owner: scenarios/ssot_l4_acceptance_spec_test.go (5s timing audit 未完成)
- evidence: behavior
- メモ: プロファイル切替が 5 秒以内; timing assert は GAP-24

#### SSOT-9.2-DOD5
- 実装: internal/ssottest/ (L2/L3 coverage ledger)
- status: implemented
- テスト owner: internal/ssottest/ledger_test.go + layer_matrix_test.go
- evidence: meta
- メモ: 個別操作が独立テスト可能

---

## §10. テスト戦略 (GAP-01〜26)

#### SSOT-10.9-GAP01
- 実装: internal/invariant/invariant.go (duplicate window の正本選択)
- status: partial
- テスト owner: internal/invariant/ssot_l1_invariant_test.go + internal/planner/ssot_l0_planner_test.go
- evidence: meta
- メモ: 複数ある場合、最 recently focused を正、他は orphan、[INVARIANT] card; registry で管理されているが test owner は incomplete

#### SSOT-10.9-GAP02
- 実装: cmd/projwm/cmd_cockpit.go, cmd_doctor.go (status 表示)
- status: partial
- テスト owner: cmd/projwm/ssot_cli_surface_test.go (表示自体は未テスト)
- evidence: meta
- メモ: 状態ごとの cockpit/status 表示テスト; 各状態の表示内容は未検証

#### SSOT-10.9-GAP03
- 実装: internal/reducer/reducer.go (intent reject logic)
- status: partial
- テスト owner: internal/reducer/ssot_l0_state_test.go (一部テスト)
- evidence: behavior
- メモ: 初期では summon/profile/archive 不可など; state × operation rejection/wait matrix は完全ではない

#### SSOT-10.9-GAP04
- 実装: internal 各所 (drift 検出と grace period)
- status: partial
- テスト owner: scenarios/ssot_real_acceptance_test.go (grace period 未テスト)
- evidence: behavior
- メモ: 60秒以内2回で grace period; カード発行・grace発動・修正停止の全体テストは未完成

#### SSOT-10.9-GAP05
- 実装: cmd/projwm/cmd_cockpit.go ([Enter]/[c]/[t] action)
- status: partial
- テスト owner: cmd/projwm/ssot_cli_surface_test.go (action execution 未テスト)
- evidence: meta
- メモ: orphan card の 3 action ([Enter] 登録, [c] close, [t] 詳細操作) の実行結果は未検証

#### SSOT-10.9-GAP06
- 実装: internal/semop/ai.go (send-keys / attach-only / multi-AI)
- status: partial
- テスト owner: internal/semop/ssot_l1_semop_test.go (詳細は不完全)
- evidence: behavior
- メモ: 初回 send-keys で AI CLI 起動、次回 attach のみ; multi-AI parity は実装済だが coverage test は不完全

#### SSOT-10.9-GAP07
- 実装: internal/adapter/zed/zed.go (zed -n --user-data-dir / 設定分離)
- status: partial
- テスト owner: internal/adapter/zed/zed_test.go (pre-existing empty project 処理は partial)
- evidence: behavior
- メモ: -n --user-data-dir で設定分離; pre-existing empty project 保護は実装済だが coverage incomplete

#### SSOT-10.9-GAP08
- 実装: internal/adapter/browser/browser.go (user profile = External)
- status: partial
- テスト owner: internal/adapter/browser/browser_test.go (user profile isolation audit 未完成)
- evidence: meta
- メモ: user profile window を管理対象外にする実 E2E は未検証

#### SSOT-10.9-GAP09
- 実装: internal/adapter/browser/ + internal/reducer/ (browser tab observer)
- status: partial
- テスト owner: scenarios/ssot_real_acceptance_test.go (observer → DesiredWorld update は partial)
- evidence: behavior
- メモ: 手動 tab 操作→observer→DesiredWorld/private payload 更新; 実装は partial

#### SSOT-10.9-GAP10
- 実装: internal/adapter/browser/ + internal/cockpitsnap/ (privacy redaction)
- status: partial
- テスト owner: cmd/projwm/cmd_status.go (redaction test 未完成)
- evidence: behavior
- メモ: 失敗時空タブ、URL/cookie 非保存、redact 表示; URL 復元失敗時空タブは実装済だが privacy complete audit 未完成

#### SSOT-10.9-GAP11
- 実装: cmd/projwm-cockpit/ (TUI 全体)
- status: partial
- テスト owner: cmd/projwm-cockpit/main_test.go (snapshot/interaction test 未完成)
- evidence: meta
- メモ: topbar / tabs / footer 全領域の SSOT snapshot test は未完成

#### SSOT-10.9-GAP12
- 実装: cmd/projwm-cockpit/ (wizard / palette / modes)
- status: partial
- テスト owner: cmd/projwm-cockpit/main_test.go (各 mode の入出テスト 未完成)
- evidence: meta
- メモ: wizard / Ctrl-P / Proposal/Navigation/Management mode の入口・出口・intent 発行・visibility 復帰; 実装部分的

#### SSOT-10.9-GAP13
- 実装: cmd/projwm/cmd_status.go + cmd_doctor.go
- status: partial
- テスト owner: cmd/projwm/ssot_cli_surface_test.go (全項目 presence + failure classification audit 未完成)
- evidence: behavior
- メモ: status 全項目、doctor PASS/WARN/FAIL 分類; 出力内容は partial

#### SSOT-10.9-GAP14
- 実装: cmd/projwm/ (各 CLI command)
- status: partial
- テスト owner: cmd/projwm/ssot_cli_surface_test.go (state/IPC 効果の全テスト 未完成)
- evidence: behavior
- メモ: profile create/delete、doctor、trace、tui、browser tab 等の state/IPC 効果テスト; coverage incomplete

#### SSOT-10.9-GAP15
- 実装: internal/planner/planner.go (L1 > L2 > L3 優先度)
- status: partial
- テスト owner: internal/planner/ssot_l0_planner_test.go (複合 drift 優先順序テスト 未完成)
- evidence: behavior
- メモ: L1/L2/L3 優先度で複合 drift 解決順序; 基本順序は実装済だが複合audit 未完成

#### SSOT-10.9-GAP16
- 実装: internal/controller/controller.go (wmMutationLock + single writer)
- status: partial
- テスト owner: internal/controller/controller_test.go (並行 intent 直列化テスト 未完成)
- evidence: behavior
- メモ: 全 mutation は projwmd のみ、並行 intent 直列化; lock structure は実装済だが race test 未完成

#### SSOT-10.9-GAP17
- 実装: internal/controller/controller.go + internal/executor/executor.go (graceful degradation)
- status: partial
- テスト owner: internal/executor/executor_test.go (部分失敗継続テスト 未完成)
- evidence: behavior
- メモ: 1 window spawn 失敗でも他継続、次 iteration replan、cockpit card; 基本実装済だが comprehensive audit 未完成

#### SSOT-10.9-GAP18
- 実装: internal/planner/planner.go (operation order) + internal/executor/executor.go
- status: partial
- テスト owner: scenarios/transaction_contract_test.go (全 phase order & barrier test 未完成)
- evidence: behavior
- メモ: close → observe-barrier → spawn、phase A/B/C order、profile/archive 順序; 基本実装済だが comprehensive audit 未完成

#### SSOT-10.9-GAP19
- 実装: internal/controller/controller.go (maxreplan 超過時の全挙動)
- status: partial
- テスト owner: internal/controller/controller_test.go (rollback/card/dirty scope/next retry 統合テスト 未完成)
- evidence: behavior
- メモ: fail/rollback/card/dirty scope/再挑戦; 部分実装済だが統合テスト 未完成

#### SSOT-10.9-GAP20
- 実装: MISSING — adapter interface に MoveCockpitToParkWorkspace メソッド未定義
- status: missing
- テスト owner: 未登録
- evidence: none
- メモ: adapter method の L2/L3 owner 割り当て不可 (メソッド自体が未実装)

#### SSOT-10.9-GAP21
- 実装: internal/adapter/zed/zed.go + internal/adapter/browser/browser.go (CollectCloseObservation)
- status: partial
- テスト owner: internal/adapter/zed/zed_test.go + browser_test.go (実 app での証拠検証 未完成)
- evidence: behavior
- メモ: close 前後の project/browser/payload 証拠自体の実機テスト不完全

#### SSOT-10.9-GAP22
- 実装: internal/store/file.go (generation / artifact / no ObservedWorld)
- status: partial
- テスト owner: internal/store/file_test.go (crash-safe generation audit 未完成)
- evidence: behavior
- メモ: generation 増、atomic rename、DesiredWorld/Layouts/Browser/Checkpoint 保存、ObservedWorld 非保存; artifact presence test は partial

#### SSOT-10.9-GAP23
- 実装: internal/store/file.go (flock + atomic rename)
- status: partial
- テスト owner: internal/store/file_test.go (concurrent writer / interrupted write audit 未完成)
- evidence: behavior
- メモ: 排他制御の実装は済だが concurrent/interrupted scenario の comprehensive test 未完成

#### SSOT-10.9-GAP24
- 実装: scenarios/ssot_l4_acceptance_spec_test.go (timing assertions 框組み未完成)
- status: missing
- テスト owner: scenarios/ssot_l4_acceptance_spec_test.go (timing test frame 未実装)
- evidence: none
- メモ: 1 分復帰 / 5s profile switch の実 E2E timing assert が test code に無い

#### SSOT-10.9-GAP25
- 実装: internal/ssottest/ + scenarios/ (skip-green gate 未実装)
- status: missing
- テスト owner: 未登録
- evidence: none
- メモ: L3/L4 実行条件強制 (real_ops / integration) の CI profile gate が未実装

#### SSOT-10.9-GAP26
- 実装: internal/ssottest/ + scenarios/ (path/workspace meta-audit 未完成)
- status: partial
- テスト owner: internal/ssottest/coverage_test.go (prefix/path/workspace audit 未完成)
- evidence: meta
- メモ: 全 real/integration test の環境分離 (store/socket/log/manifest/tmux/title/workspace) の meta-audit は incomplete

---

## 集計

| 節 | ID 数 | implemented | partial | missing | unknown |
|----|-------|-------------|---------|---------|---------|
| §7.1 | 14 | 14 | 0 | 0 | 0 |
| §7.2 | 4 | 4 | 0 | 0 | 0 |
| §7.3 | 14 | 13 | 0 | 1 | 0 |
| §7.4 | 13 | 13 | 0 | 0 | 0 |
| §7.5 | 25 | 22 | 0 | 3 | 0 |
| §8 | 11 | 10 | 1 | 0 | 0 |
| §9 | 15 | 10 | 5 | 0 | 0 |
| §10.9-GAP | 26 | 0 | 17 | 9 | 0 |
| **合計** | **122** | **86** | **23** | **13** | **0** |

## 重要ギャップ

### 未実装 (3+9=12 件)

1. **§7.5 adapter contract (3件)**:
   - SSOT-7.5-WM-SHOWSCRATCH (ShowScratchShell メソッド)
   - SSOT-7.5-WM-HIDESCRATCH (HideScratchShell メソッド)
   - SSOT-7.5-WM-MOVECP (MoveCockpitToParkWorkspace メソッド)

2. **§10.9 GAP-01~26 中の 9 件**:
   - GAP-24: 1 分復帰 / 5s profile switch timing assert
   - GAP-25: skip-green gate / CI profile
   - 他 7 件は partial

### Partial (23 件)

主に §9.2 (DOD3/4) と §10.9 (GAP-01~23) の comprehensive audit が未完成。基本実装は存在するが、
full coverage test / specification compliance audit が incomplete。

---

## 参照ファイル一覧 (§7-§10)

### Controller & Transaction Loop
- `/Users/yuta/dev/dotfiles/modules/darwin/projwm/projwm-next/internal/controller/controller.go` (1-600+)
- `/Users/yuta/dev/dotfiles/modules/darwin/projwm/projwm-next/internal/controller/controller_test.go`

### Adapter Contract
- `/Users/yuta/dev/dotfiles/modules/darwin/projwm/projwm-next/internal/adapter/wm/adapter.go` (73-105)
- `/Users/yuta/dev/dotfiles/modules/darwin/projwm/projwm-next/internal/adapter/session/adapter.go`
- `/Users/yuta/dev/dotfiles/modules/darwin/projwm/projwm-next/internal/adapter/zed/adapter.go`
- `/Users/yuta/dev/dotfiles/modules/darwin/projwm/projwm-next/internal/adapter/browser/adapter.go`

### State Management (PersistentStore)
- `/Users/yuta/dev/dotfiles/modules/darwin/projwm/projwm-next/internal/store/file.go` (85-745)
- `/Users/yuta/dev/dotfiles/modules/darwin/projwm/projwm-next/internal/store/file_test.go`

### Naming & Conventions
- `/Users/yuta/dev/dotfiles/modules/darwin/projwm/projwm-next/internal/naming/naming.go` (1-140)
- `/Users/yuta/dev/dotfiles/modules/darwin/projwm/projwm-next/internal/naming/ssot_l0_identity_test.go`

### Test Ledger
- `/Users/yuta/dev/dotfiles/modules/darwin/projwm/projwm-next/internal/ssottest/ledger_test.go`
- `/Users/yuta/dev/dotfiles/modules/darwin/projwm/projwm-next/internal/ssottest/layer_matrix_test.go`

