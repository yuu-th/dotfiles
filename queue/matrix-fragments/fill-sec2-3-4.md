# SSOT v1.11 Coverage Matrix Fragment (§2, §3, §4)

Generated: 2026-05-23

---

## §2 メンタルモデル

### SSOT-2.1-P1
- 実装: internal/world/ids.go:23-27 (DesiredWindowID に Project + Kind + Index)
- status: implemented
- テスト owner: scenarios/fixture_test.go (fixture setup で identity 検証)
- evidence: behavior (DesiredWindowID 構造が identity を実装)
- メモ: (project, kind, id) は DesiredWindowID struct で符号化。world/desired.go でも全 window が ID を持つ

### SSOT-2.1-P2
- 実装: internal/world/ids.go:23-27 (DesiredWindowID.Project + Kind + Index の組合せで一意)
- status: implemented
- テスト owner: scenarios/fixture_test.go (同 identity の重複作成をテスト)
- evidence: behavior (identity tuple で window を識別する構造)
- メモ: Project, Kind, Index が DesiredWindowID の全要素。組合せで一意に識別

### SSOT-2.1-P3
- 実装: internal/invariant/invariant.go (INV-01 で同一 identity window は 1 つのみ)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestINVOrphan (orphan 検出・修正テスト)
- evidence: behavior (observer が duplicate identity 検出、planner が orphan 論理で修正)
- メモ: invariant loop で違反時に recently_focused を正とし、他を orphan 扱い

### SSOT-2.1-P4
- 実装: internal/planner/planner.go (MoveWindowToWorkspace op で slot に戻す)
- status: implemented
- テスト owner: scenarios/accept_manual_layout_test.go (drift 修正テスト)
- evidence: behavior (manual drag 後 loop が move-to-workspace で正 slot へ復帰)
- メモ: transaction loop observe→plan→execute で drift 自動修正。slot は DesiredProfile.Assignments で定義


### SSOT-2.1-P5a
- 実装: internal/planner/planner.go:144+ (spawn missing desired windows 時に既存確認)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestSummonShell (existing window を focus する動作)
- evidence: behavior (reducer が既存 session を check して再利用)
- メモ: planner が protectedLive で既存 window を識別し、新規作成を避ける

### SSOT-2.1-P5b
- 実装: internal/planner/planner.go:144+ (desiredHas check で無い場合 spawn)
- status: implemented
- テスト owner: scenarios/fixture_test.go (初回 spawn をテスト)
- evidence: behavior (desired に無い window は spawn される)
- メモ: Phase B で KindSpawnTerminal / KindSpawnEditor が emit される

### SSOT-2.1-P5c
- 実装: internal/intent/intent.go (KindSummonShell など summon-* 全体で WS 依存なし)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestSummonAcrossWorkspace (任意の WS からの summon)
- evidence: behavior (summon は WS location に無関係)
- メモ: intent payload に slot しか含まない。daemon が ActiveProfile で project を解決、WS 関係なく focus

### SSOT-2.1-P5d
- 実装: internal/planner/planner.go + internal/executor/executor.go (identity で既存検出して同一 op を再実行)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestIdempotentSummon (何回呼んでも同じ window)
- evidence: behavior (DesiredWindowID で identity 確認、重複 spawn せず)
- メモ: executor が spawn settle 後、identity で window を match して focus。重複防止は observe → MatchedTo で実装

### SSOT-2.1-P6a
- 実装: internal/controller/controller.go:126-130 (observe フェーズで現実を observe)
- status: implemented
- テスト owner: scenarios/fixture_test.go (observe precision テスト)
- evidence: behavior (每 transaction が observe で実際の window state を読む)
- メモ: c.observe(ctx) が WM adapter を通じて ObservedWorld を refresh

### SSOT-2.1-P6b
- 実装: internal/controller/controller.go:109+ (ApplyIntent の transaction loop 全体)
- status: implemented
- テスト owner: scenarios/transaction_contract_test.go (observe→plan→execute→settle→verify→commit)
- evidence: behavior (controller が loop を実行)
- メモ: line 126-130 observe、139 reduce、200+ plan、execute、settle、verify、commit の順序が厳密

### SSOT-2.1-P6c
- 実装: internal/controller/controller.go:200+ (replan loop with MaxReplans check)
- status: implemented
- テスト owner: scenarios/transaction_contract_test.go (maxreplan exceeded 時の card emit)
- evidence: behavior (MaxReplans=4, loop 超過時に fail→rollback→card)
- メモ: verifier が Last WorldDiff で差分確認、差分あれば replan

### SSOT-2.1-P6d
- 実装: internal/planner/planner.go + internal/controller/controller.go (move-to-workspace phase で slot に配置)
- status: implemented
- テスト owner: scenarios/accept_manual_layout_test.go (drift 修正で正 slot に自動復帰)
- evidence: behavior (loop が move op で window を正 slot へ移動)
- メモ: Phase C で KindMoveWindowToWorkspace を emit し、desired slot に window を move

### SSOT-2.2-ID-PROJECT
- 実装: internal/world/ids.go:23-27 (DesiredWindowID.Project)
- status: implemented
- テスト owner: scenarios/fixture_test.go (project 名でfixture 構築)
- evidence: behavior (project name が identity の一部)
- メモ: ProjectID string type、DesiredWindowID に含まれ

### SSOT-2.2-ID-KIND
- 実装: internal/world/enums.go (WindowKind enum: ai, shell, editor, browser, viewer, cockpit, scratch)
- status: implemented
- テスト owner: scenarios/fixture_test.go (各 kind の window を生成)
- evidence: behavior (WindowKind enum で 7 種類定義)
- メモ: internal/world/ids.go で DesiredWindowID.Kind フィールド

### SSOT-2.2-ID-INDEX
- 実装: internal/world/ids.go:23-27 (DesiredWindowID.Index: int, 永続)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestWindowIndexing (index の連番性、削除での穴)
- evidence: behavior (Index は kind 内の連番, 1 始まり)
- メモ: reducer で add-window 時に最大 index+1 で採番。削除で穴がある場合も再利用しない

### SSOT-2.3-FL1
- 実装: internal/op/op.go + internal/executor/executor.go (KindSpawnTerminal で tmux session 作成)
- status: implemented
- テスト owner: scenarios/ssot_l4_acceptance_spec_test.go (AI / shell session 作成)
- evidence: behavior (spawn op で tmux -new-session -d -s ai-N/project)
- メモ: executor が session 名を生成し tmux に pass、session 無ければ create

### SSOT-2.3-FL2
- 実装: internal/op/op.go (KindSpawnEditor で tmux 不要)
- status: implemented
- テスト owner: scenarios/fixture_test.go (editor spawn に tmux 不要)
- evidence: meta (operation kind で tmux-less path)
- メモ: Zed launch path は tmux 経由しない。ghostty command も standalone

### SSOT-2.3-FL3
- 実装: internal/op/op.go (KindSpawnBrowser で tmux 不要)
- status: implemented
- テスト owner: scenarios/fixture_test.go (browser spawn に tmux 不要)
- evidence: meta (operation kind で tmux-less path)
- メモ: Vivaldi は automation profile で直接起動

### SSOT-2.3-FL4
- 実装: internal/adapter/session/tmux.go (cockpit tmux session check)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestCockpitTmux (projwm-cockpit session)
- evidence: behavior (EnsureSession で projwm-cockpit session 確認)
- メモ: session 名は hardcoded "projwm-cockpit"

### SSOT-2.3-FL5
- 実装: internal/planner/planner.go (window existence check before spawn)
- status: implemented
- テスト owner: scenarios/fixture_test.go (重複 spawn 禁止 test)
- evidence: behavior (planner が MatchedTo / identity で既存確認してから spawn)
- メモ: Phase B の spawn loop 内で protectedLive check

### SSOT-2.3-FL6
- 実装: internal/op/op.go (KindFocusWindow / KindFocusWorkspace)
- status: implemented
- テスト owner: scenarios/fixture_test.go (summon-shell で既存 window focus)
- evidence: behavior (executor が window/workspace を focus)
- メモ: focus は WS 切替 + window focus の 2-op になる

### SSOT-2.3-FL7
- 実装: internal/identity/identity.go + internal/planner/planner.go (重複 check)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestDuplicateIdentityPrevention
- evidence: behavior (MatchedTo で同 identity を detect)
- メモ: observer が title から identity resolve して MatchedTo に set。planner が同 identity 2 個目を作らない

### SSOT-2.4-SLOT-PROJECT
- 実装: internal/world/environment.go:39+ (Slots: SlotSpec array でデフォルト Q-P)
- status: implemented
- テスト owner: scenarios/fixture_test.go (fixture env に 10 slot)
- evidence: behavior (ManagedEnvironment.Workspaces.Slots に Q-P)
- メモ: SlotID は string、Q～P の 10 個がデフォルト

### SSOT-2.4-SLOT-VIEWER
- 実装: internal/world/environment.go (Slots に A 含む)
- status: implemented
- テスト owner: scenarios/fixture_test.go (viewer slot A の window management)
- evidence: behavior (environment に slot A 定義)
- メモ: viewer slot も SlotSpec として environment に含まれる

### SSOT-2.4-SLOT-COCKPIT
- 実装: internal/world/environment.go (Slots に CP1 含む)
- status: implemented
- テスト owner: scenarios/fixture_test.go (cockpit park workspace CP1)
- evidence: behavior (cockpit move target が CP1)
- メモ: cockpit 用の dedicated workspace

### SSOT-2.4-SLOT-NONBOUNDARY
- 実装: internal/planner/planner.go (window が任意 WS に飛んでも move で修正)
- status: implemented
- テスト owner: scenarios/accept_manual_layout_test.go (manual drag 後の auto-revert)
- evidence: behavior (planner が move-to-workspace で correct slot に強制配置)
- メモ: slot は概念的な target、実装は workspace move で実現

### SSOT-2.5-EC1
- 実装: internal/planner/planner.go (summon で window いずれの slot に move)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestSummonFromOtherSlot
- evidence: behavior (summon が window を引き寄せ、move で配置)
- メモ: planner が move op emit、executor が workspace 切替

### SSOT-2.5-EC2
- 実装: internal/planner/planner.go (slot 外 (M, B, 1-9) window も move で修正)
- status: implemented
- テスト owner: scenarios/accept_manual_layout_test.go
- evidence: behavior (planner が move-to-workspace で正 slot に戻す)
- メモ: workspace が任意でも identity で track、loop で修正

### SSOT-2.5-EC3
- 実装: internal/executor/executor.go (window が同 slot にあれば focus only)
- status: implemented
- テスト owner: scenarios/fixture_test.go (same-slot summon)
- evidence: behavior (focus op で window を前面に出す)
- メモ: move skip、focus だけ emit される

### SSOT-2.5-EC4
- 実装: internal/invariant/invariant.go:INV-01 (duplicate identity 検出、recently focused を正)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestINVOrphan
- evidence: behavior (observer が duplicate 検出、planner が orphan close)
- メモ: LastFocused timestamp で最新を正、他は orphan card として cockpit に表示

### SSOT-2.5-EC5
- 実装: internal/scenario/scenario.go + internal/controller/controller.go (macOS 再起動時)
- status: implemented
- テスト owner: scenarios/ssot_real_acceptance_test.go (macos reboot scenario)
- evidence: behavior (state store から recover、全 window 再作成)
- メモ: LifecycleBootstrap で checkpoint から DesiredWorld 復元、transaction loop が spawn

### SSOT-2.5-EC6
- 実装: internal/planner/planner.go (OmniWM 再起動後 window 再検出)
- status: implemented
- テスト owner: scenarios/ssot_real_acceptance_test.go
- evidence: behavior (observer が window list refresh、loop が spawn/move)
- メモ: tmux session 継続、window 消失時だけ再作成、存在時は正 slot に move

### SSOT-2.5-EC7
- 実装: internal/observer/observer.go (長時間放置後の window 再確認)
- status: implemented
- テスト owner: scenarios/fixture_test.go (window zombie 検出)
- evidence: behavior (observer が window liveness check)
- メモ: tmux session 継続性保証

### SSOT-2.5-EC8
- 実装: internal/planner/planner.go:Phase C (move-to-workspace で correct slot へ)
- status: implemented
- テスト owner: scenarios/accept_manual_layout_test.go
- evidence: behavior (Phase C move op emit)
- メモ: planner が DesiredProfile.Assignments で slot 確定、move で配置

### SSOT-2.5-EC9
- 実装: internal/planner/planner.go (viewer window 消失時 spawn)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestViewerSpawn
- evidence: behavior (viewer spawn op emit)
- メモ: viewer=grouped tmux session + Ghostty window、両者の status check

### SSOT-2.5-EC10
- 実装: internal/planner/planner.go (archived project window close)
- status: implemented
- テスト owner: scenarios/archive_project_test.go (archive 時 window auto-close)
- evidence: behavior (planner が IsProjectActive check で false なら close)
- メモ: Phase A で KindCloseWindow / KindKillSession emit

### SSOT-2.6-NR01
- 実装: internal/reducer/reducer.go (KindSwitchProfile で唯一の active profile)
- status: implemented
- テスト owner: scenarios/switch_profile_test.go (profile 1 つだけ active)
- evidence: behavior (DesiredWorld.ActiveProfile が 1 つのみ)
- メモ: reducer が profile switch 時に互斥更新

### SSOT-2.6-NR02
- 実装: internal/reducer/reducer.go (KindUnarchiveProject は state 更新のみ)
- status: implemented
- テスト owner: scenarios/unarchive_project_test.go
- evidence: behavior (unarchive 時に auto-spawn なし)
- メモ: SSOT §4.5 で明示「state 更新のみ、自動再展開しない (park 状態)」

### SSOT-2.6-NR03
- 実装: internal/intent/intent.go (command palette / mode なし、flat intent 設計)
- status: implemented
- テスト owner: scenarios/fixture_test.go (flat intent routing)
- evidence: meta (intent enum で modal/leader-key なし)
- メモ: シンプルな flat intent で実現。hotkey layer で summon-* selector

### SSOT-2.6-NR04
- 実装: internal/adapter/zed/zed.go (editor launch で -n flag を mandatory に)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestEditorNonTmux
- evidence: behavior (Zed は standalone launch)
- メモ: zed -n <cwd> で new window、tmux 不要

### SSOT-2.6-NR05
- 実装: internal/store/file.go (JSON で state 保存)
- status: implemented
- テスト owner: scenarios/fixture_test.go (store I/O test)
- evidence: behavior (PersistentStore が JSON marshaling)
- メモ: SQLite 等使わず JSON file で十分


---

## §3 システム状態

### SSOT-3.1-ST-INIT
- 実装: internal/scenario/scenario.go + internal/controller/controller.go (empty desired world)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestInitialState (project 0)
- evidence: behavior (controller state init で Desired.Projects=[], ActiveProfile なし)
- メモ: macOS 起動直後は empty state から start

### SSOT-3.1-ST-NORMAL
- 実装: internal/verifier/verifier.go (all windows in correct workspace)
- status: implemented
- テスト owner: scenarios/fixture_test.go (convergence test で CONVERGED)
- evidence: behavior (verifier が LastDiff = empty で確認)
- メモ: Desired = Observed、move/spawn/close op なし

### SSOT-3.1-ST-DRIFT
- 実装: internal/planner/planner.go (Phase C で move を emit)
- status: implemented
- テスト owner: scenarios/accept_manual_layout_test.go (manual drag 後の drift state)
- evidence: behavior (planner が move op で修正)
- メモ: window が slot 外、loop が自動修正

### SSOT-3.1-ST-RECOVERING
- 実装: internal/controller/controller.go (spawn phase で一時的に window 無い)
- status: implemented
- テスト owner: scenarios/ssot_real_acceptance_test.go (macos reboot recovery)
- evidence: behavior (spawn settle timeout で window 出現待機)
- メモ: SettleTimeout=30s で window 復帰待つ

### SSOT-3.1-ST-PARTFAIL
- 実装: internal/planner/planner.go (1 window spawn fail でも loop continue)
- status: implemented
- テスト owner: scenarios/fixture_test.go (partial spawn failure)
- evidence: behavior (planner が個別 window 失敗で他に影響なし、次 iteration で retry)
- メモ: 全体失敗でなく，个別 replan 継続

### SSOT-3.1-ST-PROFSWITCH
- 実装: internal/controller/controller.go (reducer で profile switch 時 old close→new spawn)
- status: implemented
- テスト owner: scenarios/switch_profile_test.go (profile 切替時の window transition)
- evidence: behavior (old window close→observe-barrier→new window spawn)
- メモ: Phase A/B で close/spawn separate

### SSOT-3.1-ST-COCKPIT
- 実装: internal/controller/controller.go + internal/op/op.go (cockpit visibility state)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestCockpitVisibility
- evidence: behavior (cockpit focus/unfocus op で WS 切替)
- メモ: cockpit window は CP1 permanent、visibility は WS switch で実現

### SSOT-3.1-ST-ERROR
- 実装: internal/controller/controller.go:200+ (MaxReplans超過で fail)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestMaxReplansExceeded (INVARIANT card emit)
- evidence: behavior (loop が replan 超過時に card + cockpit display)
- メモ: convergence status = REPLAN_FAILED、cockpit に [INVARIANT] card

### SSOT-3.3-OPMAT-INIT
- 実装: internal/reducer/reducer.go (project 0 なら summon/profile 操作拒否)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestInitialStateNoOp
- evidence: behavior (reducer が intent validation で check)
- メモ: intent 検証で project/profile existence check

### SSOT-3.3-OPMAT-DRIFT
- 実装: internal/controller/controller.go (all operations executable)
- status: implemented
- テスト owner: scenarios/accept_manual_layout_test.go
- evidence: behavior (drift state でも summon/profile/project など all op 受け付け)
- メモ: loop が自動修正するため，ユーザー操作は drift state でも OK

### SSOT-3.3-OPMAT-RECOVERING
- 実装: internal/settler/settler.go (settle phase で window 復帰待機中)
- status: implemented
- テスト owner: scenarios/ssot_real_acceptance_test.go
- evidence: behavior (settle timeout で待つ、その間 op 遅延)
- メモ: 復旧中は operation accept はしるが実行遅延

### SSOT-3.3-OPMAT-PARTFAIL
- 実装: internal/planner/planner.go (1 window fail でも他 window operation 継続)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (planner loop が個別失敗で全体 stop しない)
- メモ: graceful degradation by design

### SSOT-3.3-OPMAT-COCKPIT
- 実装: internal/controller/controller.go (cockpit show で WS 切替後も operation 受け付け)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestCockpitNavigation
- evidence: behavior (cockpit visible でも intent handle)
- メモ: cockpit はただの window、WS 切替で元の WS に戻る

### SSOT-3.4-INV01
- 実装: internal/invariant/invariant.go (CheckDuplicateIdentity)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestINVDuplicateIdentity
- evidence: behavior (orphan detection + last-focused 正とする logic)
- メモ: observer が duplicate MatchedTo detect、planner が orphan close

### SSOT-3.4-INV02
- 実装: internal/invariant/invariant.go (CheckWindowPlacement)
- status: implemented
- テスト owner: scenarios/accept_manual_layout_test.go
- evidence: behavior (planner が move op で window を correct slot に place)
- メモ: Phase C で move-to-workspace

### SSOT-3.4-INV03
- 実装: internal/adapter/session/tmux.go (EnsureSession)
- status: implemented
- テスト owner: scenarios/ssot_real_acceptance_test.go (tmux crash recovery)
- evidence: behavior (session 再作成 op emit)
- メモ: tmux kill 時にはｃontroller が recreate

### SSOT-3.4-INV04
- 実装: internal/planner/planner.go (archived project window close)
- status: implemented
- テスト owner: scenarios/archive_project_test.go
- evidence: behavior (Phase A で archived window close)
- メモ: IsProjectActive = false なら removal op

### SSOT-3.4-INV05
- 実装: internal/planner/planner.go (viewer spawn/close logic)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestViewerUpdate
- evidence: behavior (active profile の AI のみ viewer)
- メモ: phase B で AI 1 つにつき viewer 1 つ

### SSOT-3.4-INV06
- 実装: internal/planner/planner.go (cockpit move to CP1)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestCockpitParkWorkspace
- evidence: behavior (planner が cockpit を CP1 に park)
- メモ: cockpit window は CP1 always

### SSOT-3.4-INV07
- 実装: internal/naming/naming.go (Zed title = basename(cwd))
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestZedTitleContract
- evidence: behavior (Zed launch title が cwd basename)
- メモ: title contract で管理、再起動時 자연復帰

### SSOT-3.4-INV08
- 実装: internal/reducer/reducer.go (DesiredProfile.Assignments map)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestSlotExclusivity
- evidence: behavior (state map で同 slot 1 project のみ)
- メモ: reducer が assign-project 時に互斥チェック

### SSOT-3.4-INV09
- 実装: internal/reducer/reducer.go (KindSwitchProfile で validate)
- status: implemented
- テスト owner: scenarios/switch_profile_test.go
- evidence: behavior (reducer が profile existence check)
- メモ: intent validation で profiles[activeProfile]存在確認

### SSOT-3.4-INV10
- 実装: internal/identity/identity.go (Resolve function)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestIdentityResolve
- evidence: behavior (title から (project, kind, id) 復元)
- メモ: naming contract で title から identity復元可能

### SSOT-3.4-INV11
- 実装: internal/planner/planner.go (Tier 1 proposal card suppression for non-managed workspace)
- status: partial
- テスト owner: scenarios/fixture_test.go (管理対象外 WS window は card なし)
- evidence: behavior (planner が workspace role check で skip)
- メモ: managed workspace 判定があるが、proposal card emit logic との詳細連携不明瞭

### SSOT-3.4-INV12
- 実装: internal/planner/planner.go (viewer window reorder)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestViewerOrder
- evidence: behavior (Phase C で reorder-columns op)
- メモ: viewer windows を slot order で reorder

### SSOT-3.5-MACOS
- 実装: internal/scenario/scenario.go + internal/lifecyclecontract/contract.go (LifecycleBootstrap)
- status: implemented
- テスト owner: scenarios/validate_lifecycle_test.go (macOS restart scenario)
- evidence: behavior (checkpoint load → full spawn cycle → correct placement, <60s)
- メモ: projwmd auto-start、checkpoint から recover、transaction loop full run

### SSOT-3.5-OMNIWM
- 実装: internal/controller/controller.go:observe + planner (window 再検出、slot に配置)
- status: implemented
- テスト owner: scenarios/ssot_real_acceptance_test.go
- evidence: behavior (observe で window list refresh、move op emit, <30s)
- メモ: tmux session 継続、window 再生成かmove

### SSOT-3.5-TMUX
- 実装: internal/adapter/session/tmux.go (EnsureSession で recreate)
- status: implemented
- テスト owner: scenarios/ssot_real_acceptance_test.go (tmux crash)
- evidence: behavior (tmux kill 後 KindSpawnTerminal で recreate, <10s)
- メモ: tmux session 재작성、ghostty 재접속

### SSOT-3.5-GHOSTTY
- 実装: internal/op/op.go (KindSpawnTerminal で Ghostty only 재작성)
- status: implemented
- テスト owner: scenarios/ssot_real_acceptance_test.go
- evidence: behavior (Ghostty crash 시 window rebuild, <5s)
- メモ: tmux session 살아있음, Ghostty만 재시작

### SSOT-3.5-ZED
- 実装: internal/op/op.go (KindSpawnEditor で zed -n 재시작)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (empty project auto-close AX)
- メモ: Zed 재시작, empty project window auto-close, <10s

### SSOT-3.5-VIVALDI
- 実装: internal/adapter/browser/vivaldi.go (automation profile relaunch)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestBrowserRestore
- evidence: behavior (Vivaldi crash 时 relaunch + tab restore from PrivatePayloadStore, <10s)
- メモ: PrivatePayloadStore から URL 復원、tab 구조 재설정

### SSOT-3.5-COCKPIT
- 実装: internal/health/health.go (30s health probe) + adapter (cockpit restart)
- status: partial
- テスト owner: scenarios/fixture_test.go:TestCockpitCrashRecovery
- evidence: behavior (health probe で detect、재시작, <30s)
- メモ: health probe interval 정확성 및 재시작 logic 세부 확인 필요

### SSOT-3.5-DISPLAY
- 実装: internal/op/op.go + internal/adapter/wm/adapter.go (window 재배치, cockpit park add)
- status: partial
- テスト owner: scenarios/fixture_test.go:TestDisplayChange
- evidence: behavior (display 절단시 창 재배치, <5s)
- メモ: display 변경 감지 및 window migration 로직 세부 불명확

### SSOT-3.5-BOOT-A
- 実装: internal/planner/planner.go (state 有, actual 無 → spawn)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestBootcaseA
- evidence: behavior (KindSpawnTerminal / KindSpawnEditor emit)
- メモ: window 부재시 재작성

### SSOT-3.5-BOOT-B
- 実装: internal/identity/identity.go (orphan window title で identity 복원)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestBootcaseB
- evidence: behavior (orphan window를 title에서 identity 복원, 성공시 재등록, 실패시 orphan card)
- メモ: Resolve() 함수가 title parsing → identity 복원

### SSOT-3.5-BOOT-C
- 実装: internal/planner/planner.go (state = actual → noop)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (planned = observed 시 op 안 emit)
- メモ: 일치시 아무것도 하지 않음

### SSOT-3.5-BOOT-D
- 実装: internal/store/file.go (state corruption recovery)
- status: partial
- テスト owner: scenarios/fixture_test.go (state restore from bak)
- evidence: behavior (bak 복구 로직)
- メモ: bak 파일에서 restore, bak도 없으면 observed window로 재구축 (세부 로직 불명확)


---

## §4 操作の定義

### SSOT-4.1-OP01
- 実装: internal/intent/intent.go (KindSummonShell) + internal/controller/controller.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestSummonShellCycle
- evidence: behavior (summon-shell intent で shell-1:<project> focus、連打で循環)
- メモ: daemon が slot → project resolve、kind=shell で reduce/plan/execute

### SSOT-4.1-OP02
- 実装: internal/intent/intent.go (KindSummonEditor) + internal/controller/controller.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestSummonEditorCycle
- evidence: behavior (summon-editor intent で editor-1:<project> focus、複数時循環)
- メモ: KindSummonEditor → reduce → plan(KindSpawnEditor) → execute

### SSOT-4.1-OP03
- 実装: internal/intent/intent.go (KindSummonBrowser) + internal/controller/controller.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestSummonBrowserCycle
- evidence: behavior (summon-browser intent で browser-1:<project> focus)
- メモ: KindSummonBrowser → 同様の cycle logic

### SSOT-4.1-OP04
- 実装: internal/intent/intent.go (KindSwitchProject) + internal/controller/controller.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestSwitchProject
- evidence: behavior (intent payload {slot} で project 切替、前回 focus window に復帰)
- メモ: daemon が last-focused window をtrack、focus restore

### SSOT-4.1-OP05
- 実装: internal/intent/intent.go (KindCycleSlotWindow) + internal/controller/controller.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestCycleSlotWindow
- evidence: behavior (intent {slot, kind} で 同 slot 内 kind window 循環、WS 変えず)
- メモ: planner が複数 window (same slot, same kind) を cycle

### SSOT-4.1-OP06
- 実装: internal/intent/intent.go (KindSummonViewer) + internal/controller/controller.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestSummonViewer
- evidence: behavior (intent で viewer WS A focus、前回 focused viewer restore)
- メモ: viewer slot management

### SSOT-4.1-OP07-SHOW
- 実装: internal/intent/intent.go (KindSetCockpitVisibility) + internal/controller/controller.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestCockpitShow
- evidence: behavior (intent Visibility=Shown で CP1 focus、current_ws 記録)
- メモ: cockpit WS 切替op

### SSOT-4.1-OP07-HIDE
- 実装: internal/intent/intent.go (KindSetCockpitVisibility) + internal/controller/controller.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestCockpitHide
- evidence: behavior (intent Visibility=Hidden または Esc で PriorWorkspace restore)
- メモ: PriorWorkspace field で復帰)

### SSOT-4.1-OP08
- 実装: internal/intent/intent.go (KindSwitchProfile) + internal/controller/controller.go
- status: implemented
- テスト owner: scenarios/switch_profile_test.go
- evidence: behavior (profile switch で tmux kill せず、old close→new spawn)
- メモ: reduce で ActiveProfile update、plan で phase A/B/C

### SSOT-4.1-OP09
- 実装: internal/intent/intent.go (KindCreateProject) + internal/controller/controller.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestCreateProject
- evidence: behavior (新 slot 割当、focus shell-1:new-project)
- メモ: reducer が slot assign、planner が spawn

### SSOT-4.1-OP10
- 実装: internal/intent/intent.go (KindArchiveProject) + internal/controller/controller.go
- status: implemented
- テスト owner: scenarios/archive_project_test.go
- evidence: behavior (slot 解放、focus なし)
- メモ: reducer が Archived=true、planner が close/kill-session

### SSOT-4.1-OP11-SHOW
- 実装: internal/intent/intent.go (KindShowScratchShell) + internal/controller/controller.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestScratchShell
- evidence: behavior (global 1 つ、冪等、tmux+title=projwm-scratch-shell、新規or focus)
- メモ: scratch 용 dedicated window kind

### SSOT-4.1-OP11-HIDE
- 実装: internal/intent/intent.go (KindHideScratchShell) + internal/controller/controller.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestScratchShellHide
- evidence: behavior (focused-before window restore)
- メモ: prior WS restore

### SSOT-4.1-OP12
- 実装: internal/intent/intent.go (KindAddWindow) + internal/reducer/reducer.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestAddWindow
- evidence: behavior (existing max id+1 で 생成，삭제 穴 재利用 안 함)
- メモ: Index auto-increment logic 있음

### SSOT-4.1-OP13
- 実装: internal/intent/intent.go (KindRemoveWindow) + internal/controller/controller.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestRemoveWindow
- evidence: behavior (tmux kill+window close、last window 삭제시 --purge-if-empty로 project 삭제)
- メモ: remove window op로 life-cycle 종료

### SSOT-4.1-OP14
- 実装: internal/intent/intent.go (KindBrowserAddTab) + internal/adapter/browser/private_store.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestBrowserAddTab
- evidence: behavior (intent {project, window, url} で tab 추가, URL은 PrivatePayloadStore, DesiredWorld는 opaque ref+URLCount)
- メモ: browser tab CRUD는 private store로 管理

### SSOT-4.1-OP15
- 実装: internal/intent/intent.go (KindBrowserRemoveTab) + internal/controller/controller.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestBrowserRemoveTab
- evidence: behavior (last tab 삭제시 window close)
- メモ: browser window lifecycle과 연동

### SSOT-4.1-OP16
- 実装: internal/intent/intent.go (KindBrowserChangeTabURL) + internal/adapter/browser/browser_tabs.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestBrowserChangeTabURL
- evidence: behavior (Vivaldi 내 직接 입력도 auto-observed, 같은 업데이트로 수렴)
- メモ: observer가 Vivaldi UI 변경 감지 → PrivatePayloadStore 업데이트

### SSOT-4.1-OP17
- 実装: internal/intent/intent.go (KindBrowserReorderTabs) + internal/adapter/observer/browser_tabs.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestBrowserReorderTabs
- evidence: behavior (Vivaldi 수동 drag auto-observed, DesiredWorld에 새 순서 반영 (Level 3 auto-overwrite))
- メモ: observer→intent→reducer→plan의 loop

### SSOT-4.2-SYS-MOVE
- 実装: internal/op/op.go (KindMoveWindowToWorkspace)
- status: implemented
- テスト owner: scenarios/accept_manual_layout_test.go
- evidence: behavior (drift 수정용, Phase C에 emit)
- メモ: user manual drag/display change/OmniWM restart後의 auto-revert

### SSOT-4.2-SYS-REORDER
- 実装: internal/op/op.go (KindReorderColumns)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestReorderColumns
- evidence: behavior (viewer 순서 drift 대응)
- メモ: Level 3 layout ordering correction

### SSOT-4.2-SYS-CLOSE
- 実装: internal/op/op.go (KindCloseWindow)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (archived project window, orphan viewer 등)
- メモ: Phase A에서 emit

### SSOT-4.2-SYS-SPAWN
- 実装: internal/op/op.go (KindSpawnTerminal / KindSpawnEditor / KindSpawnBrowser / KindSpawnViewer)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (project 추가/restart후 복귀/viewer 재작성)
- メモ: Phase B에서 emit

### SSOT-4.2-SYS-KILL
- 実装: internal/op/op.go (KindKillSession)
- status: implemented
- テスト owner: scenarios/switch_profile_test.go (profile 切替時, archive 시)
- evidence: behavior (tmux session 종료)
- メモ: tmux kill-session 명령어 연동

### SSOT-4.3-CROSSWS
- 実装: internal/planner/planner.go (Tier 4: move-to-workspace で元 slot へ강제 revert)
- status: implemented
- テスト owner: scenarios/accept_manual_layout_test.go
- evidence: behavior (manual drag to other workspace → auto-revert)
- メモ: Phase C move op (Tier 4 recovery)

### SSOT-4.3-SAMEWS
- 実装: internal/planner/planner.go (Tier 2: reorder-columns 로 자동 desired layout 상覆)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestManualReorderSameWS
- evidence: behavior (same WS 내 window reorder → observer detect → reorder op emit)
- メモ: Level 3 ordering correction (Tier 2)

### SSOT-4.3-DRIFT-L2L3
- 実装: internal/planner/planner.go (Phase C move/reorder) + cockpit card
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (move-to-workspace / reorder-columns + [MOVED] card 通知)
- メモ: drift 감지시 cockpit에 사후 카드 표시

### SSOT-4.3-MISSING
- 実装: internal/planner/planner.go (Phase B spawn) + cockpit card
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (spawn + [CLOSED] card 통지 (respawn 알림))
- メモ: missing window 감지시 spawn + cockpit 알림

### SSOT-4.3-ORPHAN
- 実装: internal/identity/identity.go + internal/invariant/invariant.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestOrphanDetection
- evidence: behavior (identity 미판정 window → orphan card 제안)
- メモ: title parsing 실패 → orphan card with action

### SSOT-4.3-STALE
- 実装: internal/planner/planner.go (Phase A close) + cockpit card
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (archived project window close + [CLOSED] card)
- メモ: 불필요한 window 종료 + cockpit 통지

### SSOT-4.3-GRACE
- 실装: internal/controller/controller.go (60초 이내 동일 작업 2회 → grace period)
- status: partial
- テスト owner: scenarios/fixture_test.go:TestGracePeriod (grace period 발동)
- evidence: behavior (rate-limit history check → warning card)
- メモ: close rate limiting logic 있음, grace period exact 로직 세부 불명확

### SSOT-4.3-ORPHAN-ENTER
- 実装: internal/reducer/reducer.go (KindAdoptOrphanWindow)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestAdoptOrphan
- evidence: behavior ([Enter] → new project로 등록, slot assign)
- メモ: orphan card [Enter] action

### SSOT-4.3-ORPHAN-C
- 実装: internal/reducer/reducer.go (KindDismissOrphanWindow)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestDismissOrphan
- evidence: behavior ([c] → orphan window close)
- メモ: orphan card [c] action

### SSOT-4.3-ORPHAN-T
- 実装: internal/cockpitclient/client.go (orphan card detail TUI)
- status: partial
- テスト owner: scenarios/fixture_test.go (orphan TUI detail mode)
- evidence: behavior (cockpit에서 [t] → detail 표시)
- メモ: TUI에서 orphan 상세 조작 로직 세부 불명확

### SSOT-4.3-ORPHAN-IDKNOWN
- 実装: internal/planner/planner.go (MatchedTo != nil → orphan 扱いせず drift로 수정)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (identity 판정됨 → auto fix, orphan card 안 열림)
- メモ: observer가 title parsing 성공 → MatchedTo set → drift로만 처리

### SSOT-4.4-AI-TMUX
- 実装: internal/op/op.go (KindSpawnTerminal) + internal/adapter/session/tmux.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestAISpawn
- evidence: behavior (tmux new-session -d -s ai-N/project)
- メモ: session 名 형식 명시

### SSOT-4.4-AI-VIEWER-TMUX
- 実装: internal/adapter/session/tmux.go (grouped session for viewer)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestAIViewerTmux
- evidence: behavior (tmux new-session -d -t ai-N/project -s ai-N/project_v)
- メモ: viewer용 grouped session

### SSOT-4.4-AI-GHOSTTY
- 実装: internal/op/op.go + internal/executor/executor.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestAIGhosttty
- evidence: behavior (ghostty --title="ai-N:project" --working-directory=<cwd> -e tmux new-session -A -s ai-N/project)
- メモ: title contract fixed

### SSOT-4.4-AI-VIEWER-GHOSTTY
- 実装: internal/op/op.go (KindSpawnViewer)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestAIViewerGhosttty
- evidence: behavior (ghostty --title="ai-view-N:project" ... -e tmux attach -r -t ai-N/project_v)
- メモ: viewer ghostty 실행 명령

### SSOT-4.4-AI-EXIST
- 実装: internal/adapter/observer/observer.go (ghostty title 로 기존 검색)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (observer가 title matching으로 existing 감지)
- メモ: identity resolve로 구현

### SSOT-4.4-AI-CMD-FIRST
- 実装: internal/op/op.go (KindSpawnTerminal 시 tmux send-keys로 AI CLI 실행)
- status: partial
- テスト owner: scenarios/fixture_test.go (AI 첫 spawn 시 claude/copilot CLI)
- evidence: placeholder (spawn spec에 send-keys command 있으나 routing 불명확)
- メモ: SSOT §4.4 명시: "초회 spawn시 tmux send-keys" → DesiredWindow에 저장 필요 (Phase 0.1 전에 미완료)

### SSOT-4.4-AI-CMD-ATTACH
- 実装: internal/op/op.go (2회째 기존 session attach only)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (existing session detect → focus only, spawn skip)
- メモ: session 존재시 new-session-A (attach-or-create) skip

### SSOT-4.4-AI-MULTI
- 実装: internal/reducer/reducer.go (KindAddWindow: id auto-increment)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestMultiAI
- evidence: behavior (각 AI 독립 tmux+viewer, 모두 대등, primary없음)
- メモ: add-ai intent로 add-window 구현

### SSOT-4.4-AI-REMOVE
- 実装: internal/op/op.go (KindKillSession + KindCloseWindow)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestRemoveAI
- evidence: behavior (AI tmux+viewer grouped session kill, 양 window close)
- メモ: remove-window intent로 kill-session emit

### SSOT-4.4-SHELL-TMUX
- 実装: internal/adapter/session/tmux.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestShellSpawn
- evidence: behavior (tmux new-session -d -s shell-N/project)
- メモ: shell tmux session 명명 규칙

### SSOT-4.4-SHELL-GHOSTTY
- 実装: internal/op/op.go
- status: implemented
- テス트owner: scenarios/fixture_test.go
- evidence: behavior (title=shell-N:project)
- メモ: shell ghostty title contract

### SSOT-4.4-SHELL-EXIST
- 実装: internal/adapter/observer/observer.go
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (title matching)
- メモ: existing shell 감지

### SSOT-4.4-SHELL-MULTI
- 実装: internal/reducer/reducer.go + internal/world/desired.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestMultiShell
- evidence: behavior (여러 shell 공존, id 자동 채번)
- メモ: 複数 shell 동시 운영

### SSOT-4.4-SHELL-REMOVE
- 実装: internal/op/op.go
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (tmux kill+window close)
- メモ: shell 생명주기 종료

### SSOT-4.4-ED-N
- 実装: internal/adapter/zed/zed.go (zed -n <cwd> mandatory)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestZedLaunch
- evidence: behavior (-n 필수, 없으면 기존 workspace 재사용 위험)
- メモ: Zed new-window flag 강제

### SSOT-4.4-ED-DATADIR
- 実装: internal/executor/executor.go (--user-data-dir ~/.cache/projwm-next/zed-data)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (Zed config 분리)
- メモ: 通常 Zed와 설정 분리

### SSOT-4.4-ED-CONFIG
- 実装: internal/executor/executor.go (Zed config: restore_on_startup=none, auto_install=empty)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (Zed startup 설정)
- メモ: config 초기화로 부작용 방지

### SSOT-4.4-ED-EXIST
- 実装: internal/adapter/observer/observer.go (bundleId + title matching)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (Zed 식별: bundleId + title=basename(cwd))
- メモ: Zed editor 식별 기준

### SSOT-4.4-ED-EMPTYPROJ
- 実装: internal/executor/executor.go (spawn후 "empty project" window AXClose)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestZedEmptyProject
- evidence: behavior (pre-existing은 건드리지 않음)
- メモ: Zed "empty project" auto-close 로직

### SSOT-4.4-ED-MULTI
- 実装: internal/world/desired.go (bundleId+title+workspace로 식별)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (여러 editor 식별 가능, basic 1만 권장)
- メモ: 複數 Zed editor 동시 운영 (추천 아님)

### SSOT-4.4-ED-REMOVE
- 実装: internal/op/op.go (window AXClose, Zed app 종료 안 함)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (editor window만 닫기)
- メモ: Zed app은 유지

### SSOT-4.4-BR-PROFILE
- 実装: internal/adapter/browser/vivaldi.go (projwm-next automation profile)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestBrowserProfile
- evidence: behavior (Vivaldi automation profile 사용)
- メモ: dedicated profile for projwm

### SSOT-4.4-BR-TITLE
- 実装: internal/naming/naming.go
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (title=browser-N:project)
- メモ: browser title contract

### SSOT-4.4-BR-EXIST
- 実装: internal/adapter/observer/observer.go
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (automation profile + title matching)
- メモ: existing browser 감지

### SSOT-4.4-BR-USERPROF-EXTERNAL
- 実装: internal/planner/planner.go (user profile Vivaldi는 External 扱い)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestBrowserExternal
- evidence: behavior (user profile 창은 관리 대상 아님)
- メモ: External window로 처리하지 않음 (close/move 안 함)

### SSOT-4.4-BR-MULTI
- 実装: internal/world/desired.go (window별 독립)
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestMultiBrowser
- evidence: behavior (탭 아닌 독립 window)
- メモ: 複數 browser는 separate window

### SSOT-4.4-BR-REMOVE
- 실装: internal/op/op.go (lifecycle=browser-window-close)
- status: partial
- テスト owner: scenarios/fixture_test.go (pre-observe→pre-validate→close→post-observe→post-validate)
- evidence: behavior (VivaldiCloser로 clean close)
- メモ: browser 特殊한 close sequence (세부 로직 확인 필요)

### SSOT-4.4-BR-TAB-MANAGED
- 실装: internal/world/desired.go (browser window 내 탭)
- status: implemented
- テスト owner: scenarios/fixture_test.go (§6.3 Level 3와 동일)
- evidence: behavior (탭 구조 관리)
- メモ: system managed tabs

### SSOT-4.4-BR-TAB-OBS
- 실装: internal/adapter/observer/browser_tabs.go
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (user manual 변경 자동 감지)
- メモ: Vivaldi tab 변경 observer

### SSOT-4.4-BR-TAB-DESIRED
- 실装: internal/world/desired.go (DesiredBrowserSession)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (tab 구조만 저장, URL은 PrivatePayloadStore)
- メモ: URL 본체는 저장 안 함 (privacy)

### SSOT-4.4-BR-TAB-ADD
- 실装: internal/intent/intent.go (KindBrowserAddTab)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (automation profile에서 URL 열기)
- メモ: intended tab 추가

### SSOT-4.4-BR-TAB-REMOVE
- 実装: internal/intent/intent.go (KindBrowserRemoveTab)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (마지막 tab 삭제시 window close)
- メモ: browser lifecycle과 연동

### SSOT-4.4-BR-TAB-URL
- 実装: internal/adapter/observer/browser_tabs.go
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (tab URL 변경 감지, Vivaldi 직접 입력도)
- メモ: auto-observed

### SSOT-4.4-BR-TAB-REORDER
- 実装: internal/adapter/observer/browser_tabs.go
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (Vivaldi 수동 drag auto-observed, DesiredWorld 반영 (Level 3 auto-overwrite))
- メモ: tab ordering update

### SSOT-4.4-BR-RESTORE
- 実装: internal/adapter/browser/private_store.go
- status: implemented
- テスト owner: scenarios/fixture_test.go:TestBrowserRestore
- evidence: behavior (profile 절단후 재개시 tab 구조 복원, URL은 PrivatePayloadStore에서)
- メモ: restore 실패시 empty tab

### SSOT-4.4-BR-PRIV-NOSTORE
- 実装: internal/adapter/browser/private_store.go (URL/cookie/session은 PrivatePayloadStore)
- status: implemented
- テスト owner: scenarios/fixture_test.go
- evidence: behavior (PersistentStore에 저장 안 함)
- メモ: privacy 분리

### SSOT-4.4-BR-PRIV-REDACT
- 実装: internal/adapter/browser/private_store.go (log/trace/status에서 URL redact)
- status: implemented
- テスト owner: scenarios/fixture_test.go (redacted URL display)
- evidence: behavior (URL mask)
- メモ: privacy 보호

