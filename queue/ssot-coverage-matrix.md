# SSOT v1.11 Coverage Matrix

> projwm-next の SSOT (`queue/projwm-next-spec.md` v1.11, 2026-05-23) を atomic
> 要求文単位に分解し、各要求に対して実装/テスト/evidence の対応を記録するための
> マトリクス。切片計画はこのマトリクスから派生する。
>
> 列の意味:
> - **ID**: `SSOT-<節>-<種別><連番>-<short>` (固定)
> - **節**: SSOT の参照節
> - **要求**: SSOT 本文を圧縮しすぎず保持。「何を満たせばこの行が完了か」が読み取れる形
> - **実装**: file:line または「未実装」(agent が埋める)
> - **status**: implemented / partial / missing / unknown (agent が埋める)
> - **テスト owner**: file:func または「未登録」(agent が埋める)
> - **evidence**: behavior / meta / placeholder / none (agent が埋める)
> - **切片候補**: 統合フェーズで割り当てる
>
> Agent が埋めるべき列は最初は空 (`-`)。

> **STATUS (2026-06-08):** この matrix は当初の atomic 要求 **分解スケルトン**であり、
> 各行の 実装/status/テスト owner/evidence 列は埋められなかった (`-` のまま)。実装は
> test-enforced な **ledger** (`modules/darwin/projwm/projwm-next/internal/ssottest/ledger_test.go`、
> 84 tracked 要求) + 切片計画を通じて進行した。**authoritative かつ CI-enforced な coverage の
> source of truth は ledger**: **83/84 green**(46 deterministic-covered + 37 real-E2E-verified)、
> 残 red は **ACC-S6 (macOS restart recovery、user の reboot 待ち) のみ**。§9.2 完了定義の
> ①全受入 E2E (ACC-S1〜S10、S6 以外 green)・②不変条件 (INV.1-13 audit green)・③1分復帰
> (S7 11.8s / S10 6-27s)・④profile切替 (1分 umbrella、tight 5s は cold spawn ゆえ aspirational)・
> ⑤個別操作独立 (L2/L3) を満たす。本スケルトンは historical な planning context として扱い、
> live coverage は ledger を参照すること。

---

## §2. メンタルモデル

### §2.1 3 つの原則

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-2.1-P1 | §2.1 原則1 | projwm が管理する全ウィンドウは「特別」: 所有者 (project) と役割 (kind) を持つ | - | - | - | - | - |
| SSOT-2.1-P2 | §2.1 原則1 | (project, kind, id) の組合せでウィンドウを一意識別する | - | - | - | - | - |
| SSOT-2.1-P3 | §2.1 原則1 | 同じ (project, kind, id) のウィンドウは世界に1つだけ存在する | - | - | - | - | - |
| SSOT-2.1-P4 | §2.1 原則1 | slot の外にウィンドウが出た場合、transaction loop が正しい slot に戻す | - | - | - | - | - |
| SSOT-2.1-P5a | §2.1 原則2 | 「開く」とは「ユーザーの前に出す」: 既存があれば focus、新規作成しない | - | - | - | - | - |
| SSOT-2.1-P5b | §2.1 原則2 | 「開く」とは「ユーザーの前に出す」: 無ければ作る | - | - | - | - | - |
| SSOT-2.1-P5c | §2.1 原則2 | summon は今の場所を問わない (どの WS にいるか気にしない) | - | - | - | - | - |
| SSOT-2.1-P5d | §2.1 原則2 | summon は冪等: 何回呼んでも同じ結果 | - | - | - | - | - |
| SSOT-2.1-P6a | §2.1 原則3 | システムは「完璧な条件」を仮定せず、現実を観測する | - | - | - | - | - |
| SSOT-2.1-P6b | §2.1 原則3 | 全操作が transaction loop (observe → plan → execute → observe → replan) を通る | - | - | - | - | - |
| SSOT-2.1-P6c | §2.1 原則3 | 予想と現実に差があれば replan、最大 MaxReplans 回まで収束を試みる | - | - | - | - | - |
| SSOT-2.1-P6d | §2.1 原則3 | この loop により「正しい slot への配置」が保証される | - | - | - | - | - |

### §2.2 識別子

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-2.2-ID-PROJECT | §2.2 | identity の `project` 部分 = project 名 (例: dotfiles) | - | - | - | - | - |
| SSOT-2.2-ID-KIND | §2.2 | identity の `kind` 部分 ∈ {ai, shell, editor, browser, viewer, cockpit, scratch} | - | - | - | - | - |
| SSOT-2.2-ID-INDEX | §2.2 | identity の `id` 部分 = kind 内の連番、1 始まり、永続 (削除で穴があっても再利用しない) | - | - | - | - | - |

### §2.3 summon のフロー

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-2.3-FL1 | §2.3 step1 | kind=ai/shell: tmux session (Y-N/X) が無ければ作る | - | - | - | - | - |
| SSOT-2.3-FL2 | §2.3 step1 | kind=editor (Zed): tmux session 不要、直接 step2 へ | - | - | - | - | - |
| SSOT-2.3-FL3 | §2.3 step1 | kind=browser (Vivaldi): tmux session 不要、直接 step2 へ | - | - | - | - | - |
| SSOT-2.3-FL4 | §2.3 step1 | kind=cockpit: tmux session "projwm-cockpit" が無ければ作る | - | - | - | - | - |
| SSOT-2.3-FL5 | §2.3 step2 | session/app に紐づく window の存在確認 (作る前に必ず) | - | - | - | - | - |
| SSOT-2.3-FL6 | §2.3 step3 | window がユーザーの前にあれば noop、無ければ WS 切替+focus | - | - | - | - | - |
| SSOT-2.3-FL7 | §2.3 制約 | 重複作成禁止: 同 identity を 2 つ作らない | - | - | - | - | - |

### §2.4 slot の定義

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-2.4-SLOT-PROJECT | §2.4 | Q,W,E,R,T,Y,U,I,O,P = 10 個の project slot | - | - | - | - | - |
| SSOT-2.4-SLOT-VIEWER | §2.4 | A = viewer slot | - | - | - | - | - |
| SSOT-2.4-SLOT-COCKPIT | §2.4 | CP1 = cockpit slot (projwm-managed monitor に 1 つ) | - | - | - | - | - |
| SSOT-2.4-SLOT-NONBOUNDARY | §2.4 | slot は物理的境界ではない (window は自由に他 WS に行ける、loop の基準のみ) | - | - | - | - | - |

### §2.5 エッジケースとルール

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-2.5-EC1 | §2.5 | 既存 window が別 slot → summon: その slot に切替+focus、loop が正しい slot に移す | - | - | - | - | - |
| SSOT-2.5-EC2 | §2.5 | 既存 window が slot 外 (M, B, 1-9 等) → 同上 | - | - | - | - | - |
| SSOT-2.5-EC3 | §2.5 | 既存 window が同 slot → focus のみ | - | - | - | - | - |
| SSOT-2.5-EC4 | §2.5 | window が複数あるバグ時: 最 recently focused を正、他は orphan、cockpit に [INVARIANT] カード通知 | identity.go IdentifyWinnerAndOrphans + ResolveWithFocusTiebreak; planner.go (3 callsite で ResolveWithFocusTiebreak 使用); invariant.go Check14DuplicateWindow (controller が CardTypeInvariant emit) | implemented | identify_winner_test.go: TestIdentifyWinnerAndOrphans_*; planner_test.go: TestPlanAcceptsAmbiguousActiveDesiredWindowViaFocusTiebreak; ssot_l1_invariant_test.go: TestSSOTInvariantCheck14DuplicateWindowFires/SkipsViewerPairing/SkipsArchivedProject | behavior | S16 |
| SSOT-2.5-EC5 | §2.5 | macOS 再起動後 → loop が全部再作成 | - | - | - | - | - |
| SSOT-2.5-EC6 | §2.5 | OmniWM 再起動後 → tmux 生存、消えた window 再作成、ある window は正しい slot に | - | - | - | - | - |
| SSOT-2.5-EC7 | §2.5 | 長時間放置 → tmux 生存、消えた window のみ再作成 | - | - | - | - | - |
| SSOT-2.5-EC8 | §2.5 | window が slot 外 → loop が正しい slot に戻す | - | - | - | - | - |
| SSOT-2.5-EC9 | §2.5 | viewer 消失 → loop が再作成 | - | - | - | - | - |
| SSOT-2.5-EC10 | §2.5 | archived project の window 残留 → loop が close | - | - | - | - | - |

### §2.6 非要件

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-2.6-NR01 | §2.6 NR-01 | 複数プロファイルを同時に active にしない | - | - | - | - | - |
| SSOT-2.6-NR02 | §2.6 NR-02 | archived project を自動再活性しない | - | - | - | - | - |
| SSOT-2.6-NR03 | §2.6 NR-03 | modal / leader-key UX を採用しない | - | - | - | - | - |
| SSOT-2.6-NR04 | §2.6 NR-04 | GUI editor を tmux に押し込まない | - | - | - | - | - |
| SSOT-2.6-NR05 | §2.6 NR-05 | state を SQLite 等の DB で持たない (JSON で十分) | - | - | - | - | - |

---

## §3. システム状態

### §3.1 システム状態の網羅

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-3.1-ST-INIT | §3.1 | 初期状態: project 0、全 slot 空、cockpit に「project なし」表示 | - | - | - | - | - |
| SSOT-3.1-ST-NORMAL | §3.1 | 正常稼働: 全 window が正しい slot にある | - | - | - | - | - |
| SSOT-3.1-ST-DRIFT | §3.1 | ドリフト: window が slot 外、summon は動く | - | - | - | - | - |
| SSOT-3.1-ST-RECOVERING | §3.1 | 復旧中: 一時的に window が無い、数秒で復帰 | - | - | - | - | - |
| SSOT-3.1-ST-PARTFAIL | §3.1 | 部分障害: 1 つだけ window が消えている、他は見える | - | - | - | - | - |
| SSOT-3.1-ST-PROFSWITCH | §3.1 | profile 切替中: 旧 close 中 / 新 spawn 中 | - | - | - | - | - |
| SSOT-3.1-ST-COCKPIT | §3.1 | cockpit 表示中 | - | - | - | - | - |
| SSOT-3.1-ST-ERROR | §3.1 | エラー: loop 収束せず、[INVARIANT] カード表示 | - | - | - | - | - |

### §3.3 各状態での操作可能性

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-3.3-OPMAT-INIT | §3.3 | 初期: summon × (project なし), profile × (profile なし), archive × | - | - | - | - | - |
| SSOT-3.3-OPMAT-DRIFT | §3.3 | ドリフト: summon ○ (動く), profile ○, project追加 ○, archive ○ | - | - | - | - | - |
| SSOT-3.3-OPMAT-RECOVERING | §3.3 | 復旧中: 全操作 △ (待ち必要) | - | - | - | - | - |
| SSOT-3.3-OPMAT-PARTFAIL | §3.3 | 部分障害: summon ○ (残りは動く) | - | - | - | - | - |
| SSOT-3.3-OPMAT-COCKPIT | §3.3 | cockpit 表示中: 全操作 ○ (WS 切替で離れる) | - | - | - | - | - |

### §3.4 不変条件

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-3.4-INV01 | §3.4 INV-01 | 同一 (project, kind, id) の window は世界に 1 つ。違反時 orphan + 最 recently focused を正 | identity.go IdentifyWinnerAndOrphans + ResolveWithFocusTiebreak; invariant.go Check14DuplicateWindow (controller が CardTypeInvariant emit) | implemented | identify_winner_test.go: TestIdentifyWinnerAndOrphans_*; ssot_l1_invariant_test.go: TestSSOTInvariantCheck14DuplicateWindowFires/SkipsViewerPairing/SkipsArchivedProject | behavior | S16 |
| SSOT-3.4-INV02 | §3.4 INV-02 | active profile の全 project の window は正しい slot にある。違反時 move | - | - | - | - | - |
| SSOT-3.4-INV03 | §3.4 INV-03 | tmux session はユーザーが明示 kill しない限り生きている。違反時 再作成 | - | - | - | - | - |
| SSOT-3.4-INV04 | §3.4 INV-04 | archived project の window は存在しない。違反時 close | - | - | - | - | - |
| SSOT-3.4-INV05 | §3.4 INV-05 | viewer は active profile の project の AI のみ表示。違反時 close/spawn | - | - | - | - | - |
| SSOT-3.4-INV06 | §3.4 INV-06 | cockpit は常に park workspace CP1 に存在する。違反時 move | invariant.go Check15CockpitOnParkWorkspace; planner.go planCockpitOps が ow.Workspace != sw.ParkWorkspace で KindMoveCockpitToParkWorkspace op emit; adapter wm.go MoveCockpitToParkWorkspace (S27e) | implemented | ssot_l1_invariant_test.go: TestSSOTInvariantCheck15CockpitOffParkWorkspaceFires/CockpitOnParkWorkspaceIsSilent; planner_cockpit_test.go (関連); fake_test.go: TestFakeMoveCockpitToParkWorkspace* | behavior | S16/S17/S27e |
| SSOT-3.4-INV07 | §3.4 INV-07 | Zed window の title は basename(cwd) に一致する | - | - | - | - | - |
| SSOT-3.4-INV08 | §3.4 INV-08 | 同一 profile 内で同一 slot に複数 project はない (state map で排他) | - | - | - | - | - |
| SSOT-3.4-INV09 | §3.4 INV-09 | active profile は profiles の既存キーである (validate) | - | - | - | - | - |
| SSOT-3.4-INV10 | §3.4 INV-10 | 全 window の identity は title から復元可能 (naming.Resolve 保証) | - | - | - | - | - |
| SSOT-3.4-INV11 | §3.4 INV-11 | 管理対象外 workspace 上の managed candidate は Tier 1 提案カードを発火させない | - | - | - | - | - |
| SSOT-3.4-INV12 | §3.4 INV-12 | viewer order は slot order と一致する。違反時 reorder | - | - | - | - | - |

### §3.5 障害復帰フロー

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-3.5-MACOS | §3.5 macOS 再起動 | OmniWM + projwmd 自動起動 → LifecycleBootstrap → DesiredWorld 読み込み → 全 tmux+window 再作成 → 正 slot 配置。所要 1 分以内 | world/enums.go LifecycleBootstrap; controller.go ApplyLifecycle dispatch; store.go PersistentStore load (DesiredWorld 復元); planner Phase B spawn missing | implemented (timing 計測は S29 で実機 verify) | scenarios/transaction_contract_test.go (lifecycle); ssot_l4_acceptance_spec_test.go S6 | partial: timing 1 分以内検証 S29 | S19 |
| SSOT-3.5-OMNIWM | §3.5 OmniWM 再起動 | tmux 生存 → loop 実行 → 消えた window 再作成、ある window は正 slot に。所要 30 秒以内 | sigwm.go ProbeOmniwmHealth + RestartOmniwm + RedeployOmniwmRules (Lv1-Lv4 self-heal ladder); controller.go RecoverOmniwm | implemented | sigwm_test.go: TestSigWM_ProbeOmniwmHealth_*; ssot_l4_acceptance_spec_test.go S7 | partial: timing S29 | S19 |
| SSOT-3.5-TMUX | §3.5 tmux クラッシュ | 全 tmux 消失 → loop 実行 → tmux 再作成。Ghostty は tmux に再接続するので再作成不要。所要 10 秒以内 | session/tmux.go EnsureSession (idempotent); planner spawn 経路で createdTmuxSession 判定 | implemented | tmux_test.go: TestEnsureSessionCreates; ssot_l3_session_real_ops_test.go: TestRealOpsTmuxEnsureSession | behavior | S19 |
| SSOT-3.5-GHOSTTY | §3.5 Ghostty クラッシュ | 1 つの Ghostty 消失 → tmux 生存 → loop が Ghostty のみ再作成 → 既存 tmux に再接続。所要 5 秒以内 | sigwm.go Spawn (tmux session 既存ならそれに attach); reducer/planner で MISSING 検出 → spawn op emit | implemented | scenarios/external_events_test.go (window-vanished 経路) | partial: timing S29 | S19 |
| SSOT-3.5-ZED | §3.5 Zed クラッシュ | Zed 消失 → loop が `zed -n <cwd>` で再起動、empty project 自動 close。所要 10 秒以内 | sigwm.go Spawn (Editor branch、ProjectPath 渡し); sigwm.closeNewZedEmptyProjects (empty project 自動 close, S03 era) | implemented | adapter/zed/zed_test.go: TestCollectCloseObservationGathersPresenceAndProjectRootCorrelation | partial: timing S29 | S19 |
| SSOT-3.5-VIVALDI | §3.5 Vivaldi クラッシュ | Vivaldi 消失 → loop が automation profile で再起動 → PrivatePayloadStore からタブ復元。所要 10 秒以内 | adapter/browser/vivaldi.go automation profile spawn + InspectTabsByWindow (健全性検知の基盤); sigwm.go Spawn (Browser branch、BrowserPayloadToken 渡し); observer/browser_tabs.go consecutiveErrors カウンタ (G6 health probe の基盤) | implemented (timing 10 秒以内は S29 で実機 verify) | adapter/browser/vivaldi_test.go: TestVivaldi*; observer/browser_tabs_test.go: TestBrowserTabsSync_InspectErrorIsNonFatal/ResetsErrorCountOnRecovery | partial: 10 秒 timing S29 territory | S19+S20 |
| SSOT-3.5-COCKPIT | §3.5 cockpit クラッシュ | cockpit Ghostty 消失 → tmux 再作成 + Ghostty 再起動 → CP1 再配置。30 秒間隔 health probe が検出 (最大 30 秒) | sigwm.go SpawnCockpit (tmux + ghostty 起動); ReapDuplicateCockpits (30s ticker、S27 era); controller.go health probe; planner.go planCockpitOps (KindSpawnCockpit emit on missing) | implemented | sigwm_test.go: TestSigWM_SpawnCockpit_*; planner_cockpit_test.go: TestPlanner_Cockpit_SpawnWhenMissing | behavior | S19 |
| SSOT-3.5-DISPLAY | §3.5 ディスプレイ切断/追加 | 残存 display に window 再配置 / cockpit の park 追加。所要 5 秒以内 | world/enums.go LifecycleDisplayReconfigure; controller.go ApplyLifecycle (DisplayReconfigure 分岐); reducer.go SyncCockpitSystemWindows (DisplayCount=0 で SystemWindows nil 化) | implemented | reducer_cockpit_test.go: TestReduceIntent_SyncCockpit_FreshBuild/MonitorPlugDoesNotAddCockpit | behavior | S19 |
| SSOT-3.5-BOOT-A | §3.5 ケースA | state にあり / 実際なし → 再作成 | controller.go LifecycleBootstrap → planner Phase B spawn for missing desired | implemented | scenarios/transaction_contract_test.go (lifecycle) | partial | S19 |
| SSOT-3.5-BOOT-B | §3.5 ケースB | 実際にあり / state にない (orphan) → title から identity 復元、復元成功 → 再登録、失敗 → orphan として尊重 | naming/naming.go IdentityFromTitle (title から (kind, index, project) を逆引き); identity.Resolve で MatchedTo 復元; controller.go PromoteOrphans (失敗時 [NEW] card) | implemented | ssot_l0_identity_test.go: TestSSOTIdentityRestorationFromManagedTitles | behavior | S19 |
| SSOT-3.5-BOOT-C | §3.5 ケースC | 一致 → 何もしない | identity.Resolve が ClassUniqueStrong を返せば planner は spawn を emit しない | implemented | planner_test.go: TestPlanner_SkipsWhenAllPresent (関連) | behavior | S19 |
| SSOT-3.5-BOOT-D | §3.5 ケースD | state 破損 → bak から復旧、bak も無ければ実 window から再構築 | store/file.go generation-based PersistentStore (atomic rename + flock + 1 世代 bak); recovery 経路で previous generation を load; bak 完全消失時は LifecycleBootstrap + identity 復元 (BOOT-B path) で再構築 | implemented | store/file_test.go: TestSSOTGenerationAtomicity 等 | partial: bak 完全消失からの再構築 path は L3 で未網羅 | S19 |

---

## §4. 操作の定義

### §4.1 ユーザー操作 (17 件、状態遷移含む)

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-4.1-OP01 | §4.1 操作1 | slot の shell に jump: trigger slot キー → focus shell-1:<project>、複数 shell は連打で循環、直前 focus を記憶 | intent.go:122 SummonShell; controller.go commandKeyForIntent → "intent:summon-shell:<slot>"; planner.go planSummonWindowOps | implemented | planner_summon_window_test.go: TestPlanner_SummonShell_FirstPressTargetsIndex1/CycleNextWhenAlreadyOnShell/CycleWrapsAtEnd/NoTargetWhenNotSpawned | behavior | S09 |
| SSOT-4.1-OP02 | §4.1 操作2 | slot の editor に jump: trigger slot editor キー → focus editor-1:<project>、複数 editor は循環 | intent.go:130 SummonEditor; planSummonWindowOps (WindowEditor branch) | implemented | planner_summon_window_test.go: TestPlanner_SummonEditor_TargetsEditorOfSlotsProject | behavior | S09 |
| SSOT-4.1-OP03 | §4.1 操作3 | slot の browser に jump: trigger slot browser キー → focus browser-1:<project>、複数 browser は循環 | intent.go:137 SummonBrowser; planSummonWindowOps (WindowBrowser branch) | implemented | planner_summon_window_test.go: TestPlanner_SummonBrowser_NoTargetWhenSlotUnassigned | behavior | S09 |
| SSOT-4.1-OP04 | §4.1 操作4 | 別 project 切替: intent kind=switch-project, payload `{slot}`、直前 focus window へ復帰 | intent.go:144 SwitchProject; controller.go commandKeyForIntent → "intent:switch-project:<slot>"; planner.go planSwitchProjectOps (focus-workspace のみ、omniwm MRU が window 復帰担当) | implemented | planner_switch_project_test.go: TestPlanner_SwitchProject_EmitsFocusWorkspace/NoOpWhenAlreadyOnTargetWorkspace/NoOpWhenSlotUnassigned/NoOpWhenSlotUnknown | behavior | S10 |
| SSOT-4.1-OP05 | §4.1 操作5 | 同 slot 内 window 切替: intent kind=cycle-slot-window, payload `{slot, kind}`、WS 変えず focus のみ | intent.go:151 CycleSlotWindow; controller.go commandKeyForIntent → "intent:cycle-slot-window:<slot>:<kind>"; planner.go planCycleSlotWindowOps (focus-window のみ emit、focus-workspace は禁止) | implemented | planner_cycle_slot_window_test.go: TestPlanner_CycleSlotWindow_SwitchesFromShellToEditor/CyclesWithinSameKind/NoOpWhenAlreadyOnTarget/NoOpWhenKindNotInProject | behavior | S11 |
| SSOT-4.1-OP06 | §4.1 操作6 | viewer に jump: trigger viewer キー → focus workspace A、直前 viewer 復帰 | intent.go:159 SummonViewer; controller.go commandKeyForIntent → "intent:summon-viewer"; planner.go planSummonViewerOps | implemented | planner_summon_viewer_test.go: TestPlanner_SummonViewer_FromFocusedAITargetsItsViewer/FromNonAIFallsBackToFirstSlot/WhenAlreadyFocusedNoFocusOp/WhenViewerNotSpawnedNoOp/OnlyFiresOnIntentCommandKey | behavior | S08 |
| SSOT-4.1-OP07-SHOW | §4.1 操作7 | cockpit 表示: trigger cockpit キー → current_ws=CP1, focus cockpit | - | - | - | - | - |
| SSOT-4.1-OP07-HIDE | §4.1 操作7 | cockpit 非表示: trigger cockpit キーまたは Esc → 表示前の WS と window に戻る | - | - | - | - | - |
| SSOT-4.1-OP08 | §4.1 操作8 | profile 切替: tmux 殺さない、(work, Q) → (personal, Q) | - | - | - | - | - |
| SSOT-4.1-OP09 | §4.1 操作9 | project 追加: 新 slot を割り当て、focus shell-1:new-project | - | - | - | - | - |
| SSOT-4.1-OP10 | §4.1 操作10 | project archive: slot 解放、focus なし | - | - | - | - | - |
| SSOT-4.1-OP11-SHOW | §4.1 操作11 | scratch shell 表示: グローバル 1 つ、冪等、既存あれば focus、無ければ作る、tmux+title=`projwm-scratch-shell`、intent kind=show-scratch-shell | intent.go:213 ShowScratchShell; reducer.go (S07); planner.go planScratchOps; executor.go KindShowScratchShell; sigwm.go ShowScratchShell | implemented | reducer_scratch_test.go: TestReduceIntent_ShowScratchShell_FreshBuild/PreservesPriorWindowOnReshow/HiddenToShownCapturesPrior; planner_scratch_test.go: TestPlanner_Scratch_ShowEmitsWhenNotFocused/ShowNoOpWhenAlreadyFocused; executor_test.go: TestExecuteShowScratchShell; ssot_l3_wm_spec_test.go: TestScratchShellShowReturnsLiveWindowID | behavior | S07 |
| SSOT-4.1-OP11-HIDE | §4.1 操作11 | scratch shell 非表示: 表示前 focused window に戻る、intent kind=hide-scratch-shell | intent.go:217 HideScratchShell; reducer.go (S07) HideScratchShell case; planner.go planScratchOps (Visibility==Hidden); executor.go KindHideScratchShell; sigwm.go HideScratchShell | implemented | reducer_scratch_test.go: TestReduceIntent_HideScratchShell_TogglesVisibility/NoEntryIsNoop; planner_scratch_test.go: TestPlanner_Scratch_HideEmitsWhenFocused/HideNoOpWhenNotFocused; executor_test.go: TestExecuteHideScratchShellRestoresPriorFocus/TestExecuteHideScratchShellEmptyPriorIsNoop | behavior | S07 |
| SSOT-4.1-OP12 | §4.1 操作12 | add-window: 既存最大 id+1 で生成、削除で穴があっても再利用しない、kind ∈ {ai,shell,editor,browser} | intent.go:227 AddWindow; reducer.go nextWindowIndex (max+1, no hole reuse); reducer.go AddWindow handler (AIName validation); semop.go aiCommandFor routing; sigwm.go Spawn (tmux SendKeys when WindowAI + freshly-created session + AICommand) | implemented | ssot_l0_state_test.go: TestSSOTWindowIndexAllocationNeverReusesDeletedIDs; reducer_v3_test.go: TestReduceIntent_AddWindow_AutoIndex/AIRequiresName; ssot_l1_semop_test.go: TestSSOTAICommandRoutesFromDesiredAISession; ssot_l3_wm_real_ops_test.go: TestRealOpsSpawnAISendsAICommand | behavior | S03+S12 |
| SSOT-4.1-OP13 | §4.1 操作13 | remove-window: tmux kill+window close、最後の window 削除時 default は空 windows[] 保持、--purge-if-empty で project 削除 | intent.go:237 RemoveWindow; reducer.go RemoveWindow handler (project.Windows から削除、project 残置); planner.go closeOrder loop + lifecycleRemovalAllowed (S13 で削除済 desired への fallback path 追加) | implemented (partial: --purge-if-empty CLI flag は未対応 — projwmctl では一旦 RemoveWindow 単体提出に留め、cockpit UI で 2-step 確認は別 slice で扱う) | planner_remove_window_test.go: TestPlanner_RemoveWindow_EmitsCloseOpForRemovedShell/LastWindowKeepsProject; reducer_v3_test.go: TestReduceIntent_RemoveWindow | behavior | S13 |
| SSOT-4.1-OP14 | §4.1 操作14 | browser add-tab: intent kind=browser-add-tab, payload `{project, window, url}`、URL は PrivatePayloadStore、DesiredWorld は opaque ref + URLCount のみ、最後尾追加 | intent.go:250 BrowserAddTab; controller.go prepareBrowserIntent (S20 Put → token rewrite); reducer.go BrowserAddTab handler (URLPayloadRefs に token append); cmd_browser.go cmdBrowserAddTab CLI; cmd/projwmd/main.go (ctrl.PayloadStore = privateStore) | implemented | controller_browser_payload_test.go: TestControllerBrowserAddTab_RoutesURLThroughPayloadStore/NilPayloadStoreStoresLiteral; reducer_browser_test.go: TestReduceIntent_BrowserAddTab_AppendsURL/RejectNonBrowserWindow | behavior | S14+S20 |
| SSOT-4.1-OP15 | §4.1 操作15 | browser remove-tab: intent kind=browser-remove-tab, 最後のタブ削除時 browser window ごと close | intent.go:260 BrowserRemoveTab; controller.go prepareBrowserIntent (S20 Forget); reducer.go BrowserRemoveTab handler (out-of-range error); cmd_browser.go cmdBrowserRemoveTab | partial: 最後のタブ削除時の browser window 自動 close は browser tab observer 連携で planner 側 fire 予定 (S20 Step 3) | controller_browser_payload_test.go: TestControllerBrowserRemoveTab_ForgetsRemovedRef; reducer_browser_test.go: TestReduceIntent_BrowserRemoveTab_RemovesAtIndex/OutOfRangeErrors | behavior | S14+S20 |
| SSOT-4.1-OP16 | §4.1 操作16 | browser change-tab-url: intent kind=browser-change-tab-url、ユーザーが Vivaldi 内で直接入力した場合も同じ更新に収束 | intent.go:269 BrowserChangeTabURL; controller.go prepareBrowserIntent (S20 Forget+Put rotation); reducer.go BrowserChangeTabURL handler; cmd_browser.go cmdBrowserChangeTabURL | partial: Vivaldi 内手動 URL 入力の自動観測 → 同じ intent に収束する経路は S20 Step 3 で完成 | controller_browser_payload_test.go: TestControllerBrowserChangeTabURL_RotatesPayload; reducer_browser_test.go: TestReduceIntent_BrowserChangeTabURL_Replaces | behavior | S14+S20 |
| SSOT-4.1-OP17 | §4.1 操作17 | browser reorder-tabs: intent kind=browser-reorder-tabs、ユーザーが Vivaldi 内で手動ドラッグした場合の自動観測も含む | intent.go:280 BrowserReorderTabs; controller.go prepareBrowserIntent (no-op、ref のみ並び替え); reducer.go BrowserReorderTabs handler (from/to swap); cmd_browser.go cmdBrowserReorderTabs | partial: ユーザー手動ドラッグ観測経路は browser tab observer S20 Step 3 で実装 | controller_browser_payload_test.go: TestControllerBrowserReorderTabs_NoPayloadStoreCalls; reducer_browser_test.go: TestReduceIntent_BrowserReorderTabs_MovesFromTo/SameFromToIsNoop | behavior | S14+S20 |

### §4.2 システム操作

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-4.2-SYS-MOVE | §4.2 | `move-window-to-workspace`: drift 修正用、ユーザー手動ドラッグ・display 変更・OmniWM 再起動等で発火 | - | - | - | - | - |
| SSOT-4.2-SYS-REORDER | §4.2 | `reorder-columns`: column 配置 desired と異なる時、viewer 順序 drift にも対応 | - | - | - | - | - |
| SSOT-4.2-SYS-CLOSE | §4.2 | `close-window`: archived project window 残留、viewer orphan 等 | - | - | - | - | - |
| SSOT-4.2-SYS-SPAWN | §4.2 | `spawn-*`: project 追加、再起動後復帰、viewer 再作成等 | - | - | - | - | - |
| SSOT-4.2-SYS-KILL | §4.2 | `kill-session`: profile 切替時の旧 close、archive 時の close | - | - | - | - | - |

### §4.3 ユーザー手動操作と drift

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-4.3-CROSSWS | §4.3 | cross-workspace 移動: loop が `move-window-to-workspace` で元 slot へ強制 revert (Tier 4) | planner.go:182 (active desired window が誤った workspace に居れば KindMoveWindowToWorkspace op を emit) | implemented | scenarios/external_events_test.go (drift 経路、S29 で acceptance 整備予定) | partial: L4 hardcode あり S29 territory | S18 |
| SSOT-4.3-SAMEWS | §4.3 | 同一 workspace 内並び替え: loop が自動 desired layout に上書き (Tier 2) | reducer.go KindUserReorderedColumns ハンドラが layout-sync DirtyScope 発行; controller.go applyTier2AutoSyncLayout で DesiredWorld を上書き (Phase 1+S06 dead-code purge で N-12 反映済) | implemented | scenario/acceptance.go S8.D/S8.F (AcceptedLayouts 検証); scenarios/transaction_contract_test.go | partial: L4 hardcode S29 | S18 |
| SSOT-4.3-DRIFT-L2L3 | §4.3 | drift (間違った場所/順序) 修正: move-to-workspace / reorder-columns + 事後カード通知 | planner.go Phase C (KindMoveWindowToWorkspace / KindReorderColumns); 事後カードは controller の invariant→Card 経路 (Check4 active-desired-present 等) | implemented | planner_test.go: TestSSOTDriftedActiveWindowPlansMoveNotRespawn | behavior | S18 |
| SSOT-4.3-MISSING | §4.3 | missing (window 消失) 修正: spawn + 事後カード通知 | planner.go Phase B spawn ops; missing は identity.Resolve が ClassMissing を返す経路で planner が spawn を emit | implemented | planner_test.go (spawn ops); scenarios/external_events_test.go | partial: L4 hardcode S29 | S18 |
| SSOT-4.3-ORPHAN | §4.3 | orphan (正体不明 window) 修正: 自動判断不可、cockpit カード提案 | reducer.go ReactToEvent で managed workspace 上の non-MatchedTo window を PendingOrphans に追加; controller.go PromoteOrphans が grace 後に CardTypeNew (orphan adoption proposal) を emit | implemented | controller_cards_test.go: TestPromoteOrphans_PromotesAfterGrace/KeepsWithinGrace/DropsSilentlyAdoptedOrClosed | behavior | S18 |
| SSOT-4.3-STALE | §4.3 | stale (不要 window 残留) 修正: close / kill-session + 事後カード通知 | planner.go closeOrder loop (desiredHas=false かつ IsProjectActive=false で removalOperation emit; S13 で lifecycleRemovalAllowed の fallback path 追加で削除済 DesiredWindow も対応) | implemented | planner_remove_window_test.go: TestPlanner_RemoveWindow_EmitsCloseOpForRemovedShell/LastWindowKeepsProject | behavior | S13+S18 |
| SSOT-4.3-GRACE | §4.3 | 60 秒以内に同操作 2 回 → grace period 発動 → 修正停止 + warning カード | ControllerMeta.UserCloseHistory map (60s rolling window); controller.go pruneUserCloseHistory; reducer.go closeRateLimited (60s 以内 2 回以上で rateLimited=true、warning card 化) | implemented | reducer tests for UserCloseHistory (existing) | partial: tests indirect | S18 |
| SSOT-4.3-ORPHAN-ENTER | §4.3 | orphan card [Enter]: 新規 project として登録 + slot 割り当て | PromoteOrphans が Card.Actions に `{Key:"Enter", Label:"adopt / respawn properly"}` を生成 (Zed の場合は `register as new project + slot prompt`); cockpit TUI 側で Enter 押下時に intent.AdoptOrphanWindow 提出 | implemented (intent surface OK、cockpit→intent 配線は cockpit TUI 側) | reducer adopt-orphan tests | partial: cockpit TUI 配線は §5.4 / Phase 4 | S18+S22 |
| SSOT-4.3-ORPHAN-C | §4.3 | orphan card [c]: close | PromoteOrphans が `{Key:"c", Label:"close orphan"}` action 生成; ユーザーが c 押すと daemon 側 close 経路 (KindCloseWindow or KindKillSession) | implemented | (cockpit TUI 経由の挙動は Phase 4 で結合 verify) | partial | S18+S22 |
| SSOT-4.3-ORPHAN-T | §4.3 | orphan card [t]: TUI で詳細操作 | PromoteOrphans が `{Key:"t", Label:"carry over to TUI"}` action 生成 | implemented (intent surface) | (cockpit TUI 配線は Phase 4) | partial | S18+S22 |
| SSOT-4.3-ORPHAN-IDKNOWN | §4.3 | identity が判明している window は orphan 扱いしない (drift として自動修正) | reducer.go orphan 検出時 ow.MatchedTo != nil の window はスキップ (drift = MoveWindowToWorkspace 自動経路) | implemented | reducer ReactToEvent tests | behavior | S18 |

### §4.4 各 kind の spawn 詳細

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-4.4-AI-TMUX | §4.4 ai | ai spawn 時 tmux session を作成: `tmux new-session -d -s ai-N/project` | - | - | - | - | - |
| SSOT-4.4-AI-VIEWER-TMUX | §4.4 ai | viewer 用 grouped session: `tmux new-session -d -t ai-N/project -s ai-N/project_v` | - | - | - | - | - |
| SSOT-4.4-AI-GHOSTTY | §4.4 ai | Ghostty 起動: `ghostty --title="ai-N:project" --working-directory=<cwd> -e tmux new-session -A -s ai-N/project` | - | - | - | - | - |
| SSOT-4.4-AI-VIEWER-GHOSTTY | §4.4 ai | viewer window 起動: `ghostty --title="ai-view-N:project" ... -e tmux attach -r -t ai-N/project_v` | - | - | - | - | - |
| SSOT-4.4-AI-EXIST | §4.4 ai | ghostty title `ai-N:project` で既存検索、あれば focus | - | - | - | - | - |
| SSOT-4.4-AI-CMD-FIRST | §4.4 ai | 初回 spawn 時に tmux send-keys で AI CLI (claude/copilot) を起動 | - | - | - | - | - |
| SSOT-4.4-AI-CMD-ATTACH | §4.4 ai | 2 回目以降 既存 session に attach のみ (AI 既起動) | - | - | - | - | - |
| SSOT-4.4-AI-MULTI | §4.4 ai | 複数 AI: add-ai --ai <name>、id 自動採番、各 AI 独立 tmux+viewer、全 AI 対等 (primary/default 概念なし) | - | - | - | - | - |
| SSOT-4.4-AI-REMOVE | §4.4 ai | remove: AI tmux+viewer grouped session を kill、両 window close | - | - | - | - | - |
| SSOT-4.4-SHELL-TMUX | §4.4 shell | tmux session 作成: `tmux new-session -d -s shell-N/project` | - | - | - | - | - |
| SSOT-4.4-SHELL-GHOSTTY | §4.4 shell | Ghostty 起動 title=`shell-N:project` | - | - | - | - | - |
| SSOT-4.4-SHELL-EXIST | §4.4 shell | title で既存検索 | - | - | - | - | - |
| SSOT-4.4-SHELL-MULTI | §4.4 shell | 複数 shell 共存可、id 自動採番 | - | - | - | - | - |
| SSOT-4.4-SHELL-REMOVE | §4.4 shell | remove: tmux kill+window close | - | - | - | - | - |
| SSOT-4.4-ED-N | §4.4 editor | `zed -n <cwd>` で起動 (-n 必須)、フラグなしは既存 workspace 再利用してしまう | - | - | - | - | - |
| SSOT-4.4-ED-DATADIR | §4.4 editor | `--user-data-dir ~/.cache/projwm-next/zed-data` を使い通常 Zed と設定分離 | - | - | - | - | - |
| SSOT-4.4-ED-CONFIG | §4.4 editor | 設定: `restore_on_startup = "none"`, `auto_install_extensions = {}` | - | - | - | - | - |
| SSOT-4.4-ED-EXIST | §4.4 editor | bundleId + title (=basename(cwd)) で識別 | - | - | - | - | - |
| SSOT-4.4-ED-EMPTYPROJ | §4.4 editor | spawn 後に新出した "empty project" window は AXClose で閉じる、pre-existing は触らない | - | - | - | - | - |
| SSOT-4.4-ED-MULTI | §4.4 editor | 複数 editor は bundleId+title+workspace で識別、basic 1 のみ推奨 | - | - | - | - | - |
| SSOT-4.4-ED-REMOVE | §4.4 editor | remove: window を AXClose、Zed app 自体は kill しない | - | - | - | - | - |
| SSOT-4.4-BR-PROFILE | §4.4 browser | Vivaldi automation profile (projwm-next) を使用 | - | - | - | - | - |
| SSOT-4.4-BR-TITLE | §4.4 browser | window title=`browser-N:project` | - | - | - | - | - |
| SSOT-4.4-BR-EXIST | §4.4 browser | automation profile + title で既存検索 | - | - | - | - | - |
| SSOT-4.4-BR-USERPROF-EXTERNAL | §4.4 browser | user profile (デフォルト) の Vivaldi window は External 扱い (管理対象外) | - | - | - | - | - |
| SSOT-4.4-BR-MULTI | §4.4 browser | 複数 browser は独立 window (タブではない) | - | - | - | - | - |
| SSOT-4.4-BR-REMOVE | §4.4 browser | remove: lifecycle=`browser-window-close` (VivaldiCloser)、pre-observe→pre-validate→close→post-observe→post-validate | - | - | - | - | - |
| SSOT-4.4-BR-TAB-MANAGED | §4.4 browser tab | 各 browser window 内のタブは §6.3 Level 3 同様システムが管理 | - | - | - | - | - |
| SSOT-4.4-BR-TAB-OBS | §4.4 browser tab | ユーザー手動操作 (追加/削除/並び替え/URL移動) を自動観測し PrivatePayloadStore に保存 | adapter/browser/vivaldi.go InspectTabsByWindow (AppleScript で per-window URL list); adapter/observer/browser_tabs.go BrowserTabsSync.pollOnce (diff emitter); cmd/projwmd/browser_tabs.go vivaldiInspectorAdapter (browser.WindowTabs → observer.WindowSnapshot 変換) | implemented | inspect_tabs_by_window_test.go: TestParseInspectTabsByWindow_*; observer/browser_tabs_test.go: TestBrowserTabsSync_EmitsBrowserAddTab/RemoveTab/ChangeTabURL/SkipsUserProfile/InspectErrorIsNonFatal/ResetsErrorCountOnRecovery/PurgesSnapshotForDisappearedWindow/TestDiffTabs | behavior | S20 |
| SSOT-4.4-BR-TAB-DESIRED | §4.4 browser tab | DesiredWorld にはタブ構造 (どのタブがどの window) のみ保存、URL 本体は保存しない | - | - | - | - | - |
| SSOT-4.4-BR-TAB-ADD | §4.4 browser tab add | 指定 URL を automation profile で開く、対象 window 内に追加 | - | - | - | - | - |
| SSOT-4.4-BR-TAB-REMOVE | §4.4 browser tab remove | 指定タブ close、最後のタブの場合 window ごと close | - | - | - | - | - |
| SSOT-4.4-BR-TAB-URL | §4.4 browser tab url | 対象タブ URL 変更 or Vivaldi 内直接入力を自動観測 | - | - | - | - | - |
| SSOT-4.4-BR-TAB-REORDER | §4.4 browser tab reorder | Vivaldi 内手動ドラッグを自動観測、新順序を DesiredWorld に反映 (Level 3 自動上書き) | - | - | - | - | - |
| SSOT-4.4-BR-RESTORE | §4.4 browser restore | profile 切替で window close 後、再開時にタブ構造復元、URL は PrivatePayloadStore から復元、失敗時空タブ | - | - | - | - | - |
| SSOT-4.4-BR-PRIV-NOSTORE | §4.4 browser privacy | URL/cookie/session token は PersistentStore に保存しない、PrivatePayloadStore へ分離 | - | - | - | - | - |
| SSOT-4.4-BR-PRIV-REDACT | §4.4 browser privacy | log/trace/status で URL を redact 表示 | - | - | - | - | - |

### §4.5 複合操作

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-4.5-PROF-SWITCH | §4.5 | profile_switch: close 旧 → observe-barrier → spawn 新 → viewer 更新、tmux は殺さない | intent.go SwitchProfile; reducer.go SwitchProfile handler; planner.go phase separation (phaseRemovals → barrier → phaseSpawns → barrier → phaseLayout, planner.go:587-596) | implemented | reducer_switch_profile_test.go: TestReduceIntent_SwitchProfile_FlipsActive/PreservesBothAssignments; scenarios/switch_profile_test.go: TestSwitchProfile | behavior | S15 |
| SSOT-4.5-ARCHIVE | §4.5 | archive_project: close 全 → observe-barrier、tmux session も kill | intent.go ArchiveProject; reducer.go ArchiveProject handler (Archived=true、Assignments から除外); planner.go close + kill-session via desiredHas=false + IsProjectActive=false branch | implemented | scenarios/archive_project_test.go: TestArchiveProject; invariant: TestSSOTInvariantArchivedProjectMustHaveNoLiveWindows | behavior | S15 |
| SSOT-4.5-UNARCHIVE | §4.5 | unarchive_project: state 更新のみ、自動再展開しない (park 状態) | intent.go UnarchiveProject; reducer.go UnarchiveProject handler (Archived=false のみ、slot assignment は触らない — SSOT §4.5 park-state 復帰) | implemented | reducer/ssot_l0_state_test.go: TestSSOTUnarchiveProjectReturnsToParkStateWithoutSlotAssignment; scenarios/unarchive_project_test.go: TestUnarchiveProject | behavior | S15 |
| SSOT-4.5-ASSIGN | §4.5 | assign_project: state 変更 → viewer 更新、move は loop が後で実行 | - | - | - | - | - |
| SSOT-4.5-RECONCILE | §4.5 | reconcile: loop の入口 (手動・自動いずれも) | - | - | - | - | - |
| SSOT-4.5-PRINCIPLE | §4.5 | ユーザー操作は state 変更と summon (close/spawn/focus) のみ、move は loop の専権 | - | - | - | - | - |

### §4.6 リトライ・タイムアウト

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-4.6-CTL-TIMEOUT | §4.6 | CtlExecutor.Timeout = 5s | - | - | - | - | - |
| SSOT-4.6-OBS-BARRIER | §4.6 | observeBarrierSleep = 500ms | - | - | - | - | - |
| SSOT-4.6-SETTLE-TIMEOUT | §4.6 | SettleTimeout = 30s | - | - | - | - | - |
| SSOT-4.6-DISAPPEAR-WAIT | §4.6 | DisappearWait = 15s | - | - | - | - | - |
| SSOT-4.6-WAIT-FOCUS | §4.6 | waitFocusedWindow = 1.5s | - | - | - | - | - |
| SSOT-4.6-PREMOVE | §4.6 | preMoveGrace = 150ms | - | - | - | - | - |
| SSOT-4.6-ALIVE-FALLBACK | §4.6 | process-alive fallback: spawn settle タイムアウト + OS プロセス生存 → 成功扱い | - | - | - | - | - |
| SSOT-4.6-MAXREPLAN | §4.6 | max replans 超過: fail → rollback → [INVARIANT] カード → dirty scope 記録 → 次 intent/event で再挑戦 | - | - | - | - | - |

---

## §5. UI の定義

### §5.3 キーショートカット

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-5.3-KEY-SHELL | §5.3 | slot の shell jump: space + slot キー (Q-P)、連打で循環 | - | - | - | - | - |
| SSOT-5.3-KEY-EDITOR | §5.3 | slot の editor jump: space + modifier + slot キー | - | - | - | - | - |
| SSOT-5.3-KEY-BROWSER | §5.3 | slot の browser jump: space + 別 modifier + slot キー | - | - | - | - | - |
| SSOT-5.3-KEY-PROJECT | §5.3 | project 切替: space + slot キー (操作1と共用) | - | - | - | - | - |
| SSOT-5.3-KEY-CYCLE | §5.3 | 同 slot 内 window 切替: space + ctrl + slot キー | - | - | - | - | - |
| SSOT-5.3-KEY-VIEWER | §5.3 | viewer jump: space + A | - | - | - | - | - |
| SSOT-5.3-KEY-COCKPIT | §5.3 | cockpit show/hide: space + F | - | - | - | - | - |
| SSOT-5.3-KEY-SCRATCH | §5.3 | scratch shell: 別キー (alt+space 等)、intent kind=show/hide-scratch-shell | - | - | - | - | - |

### §5.4 Cockpit TUI

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-5.4-LOC | §5.4 | projwm-managed monitor に 1 つだけ常駐 | - | - | - | - | - |
| SSOT-5.4-TMUX | §5.4 | tmux session=`projwm-cockpit` を backing | - | - | - | - | - |
| SSOT-5.4-PARK | §5.4 | park workspace CP1 に永続配置 | - | - | - | - | - |
| SSOT-5.4-VIS | §5.4 | 表示/非表示は workspace 切替で実現 (cockpit window は移動しない) | - | - | - | - | - |
| SSOT-5.4-TOPBAR | §5.4 | topbar: gen / epoch / profile / convergence / cards | - | - | - | - | - |
| SSOT-5.4-TAB-SLOTS | §5.4 | Slots tab (1): active profile の slot Q-P assignment、viewer (A) AI stream、park 一覧 | - | - | - | - | - |
| SSOT-5.4-TAB-CARDS | §5.4 | Cards tab (2): カードモーダル、左 detail、右 workspace zoom-out | - | - | - | - | - |
| SSOT-5.4-TAB-ARCHIVED | §5.4 | Archived tab (3): archived project 一覧 (unarchive 操作) | - | - | - | - | - |
| SSOT-5.4-TAB-PROFILES | §5.4 | Profiles tab (4): 全 profile + assignments (active 強調)、profile CRUD | - | - | - | - | - |
| SSOT-5.4-TAB-TRACE | §5.4 | Trace tab (5): 最近の transaction trace | - | - | - | - | - |
| SSOT-5.4-CARD-NEW | §5.4 | [NEW]: 新規 window が managed workspace に出現 (Tier 1) | - | - | - | - | - |
| SSOT-5.4-CARD-CLOSED | §5.4 | [CLOSED]: window がユーザーに閉じられた (Tier 4 自動 respawn の通知) | - | - | - | - | - |
| SSOT-5.4-CARD-MOVED | §5.4 | [MOVED]: window が別 workspace に移動された (Tier 4 revert) | - | - | - | - | - |
| SSOT-5.4-CARD-INVARIANT | §5.4 | [INVARIANT]: 不変条件違反検出 | - | - | - | - | - |
| SSOT-5.4-CARD-MANIFEST | §5.4 | [MANIFEST]: manifest 変更検出 | - | - | - | - | - |
| SSOT-5.4-CARD-RECOVERY | §5.4 | [OMNIWM-RECOVERY]: OmniWM 自己修復実行 | - | - | - | - | - |
| SSOT-5.4-WIZARD | §5.4 | Wizard (n): 全項目同時表示、defaults prefill、Tab で field 移動、Enter submit | - | - | - | - | - |
| SSOT-5.4-PALETTE | §5.4 | Command palette (Ctrl-P): fuzzy 検索、全 action を 1 リスト、Enter 実行 | - | - | - | - | - |
| SSOT-5.4-MODE-PROP | §5.4 | Proposal mode (強制表示): システム提案カード push、応答後 元 visibility 復帰 | - | - | - | - | - |
| SSOT-5.4-MODE-NAV | §5.4 | Navigation mode: space+F で開く、操作後 自動 hide | - | - | - | - | - |
| SSOT-5.4-MODE-MGMT | §5.4 | Management mode: space+F で開く、操作後 stay、space+F で hide | - | - | - | - | - |

### §5.5 エラー通知

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-5.5-CARDS | §5.5 | cockpit カード: 不変条件違反、OmniWM 自己修復、orphan 提案 | - | - | - | - | - |
| SSOT-5.5-TOPBAR | §5.5 | cockpit topbar: convergence status (CONVERGED / CONVERGING / REPLAN_FAILED) | - | - | - | - | - |
| SSOT-5.5-DOCTOR | §5.5 | `projwm doctor`: PASS/WARN/FAIL 形式 | - | - | - | - | - |
| SSOT-5.5-NO-MACOS | §5.5 | macOS notification 一切使わない | - | - | - | - | - |

### §5.6 status / doctor 出力

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-5.6-STATUS-GEN | §5.6 status | Generation ID, Epoch | - | - | - | - | - |
| SSOT-5.6-STATUS-PROF | §5.6 status | Active profile name + description | - | - | - | - | - |
| SSOT-5.6-STATUS-ASSIGN | §5.6 status | 全 profile の slot→project 割り当て一覧 | - | - | - | - | - |
| SSOT-5.6-STATUS-WIN | §5.6 status | active project の windows 状態 (kind, index, tmux 生死, live window 生死) | - | - | - | - | - |
| SSOT-5.6-STATUS-VIEWER | §5.6 status | viewer workspace A 上の AI stream 一覧 | - | - | - | - | - |
| SSOT-5.6-STATUS-PARK | §5.6 status | park 状態の project 一覧 | - | - | - | - | - |
| SSOT-5.6-STATUS-ARCH | §5.6 status | archived project 一覧 | - | - | - | - | - |
| SSOT-5.6-STATUS-CONV | §5.6 status | convergence status (CONVERGED / CONVERGING / REPLAN_FAILED) | - | - | - | - | - |
| SSOT-5.6-STATUS-DIGEST | §5.6 status | manifest digest 検証状態 | - | - | - | - | - |
| SSOT-5.6-DOC-PROC | §5.6 doctor | projwmd プロセスの存在確認 | - | - | - | - | - |
| SSOT-5.6-DOC-STORE | §5.6 doctor | PersistentStore の読み取り可否 | - | - | - | - | - |
| SSOT-5.6-DOC-MANI | §5.6 doctor | manifest の存在と digest 検証 | - | - | - | - | - |
| SSOT-5.6-DOC-SOCK | §5.6 doctor | IPC socket の到達性 | - | - | - | - | - |
| SSOT-5.6-DOC-APPS | §5.6 doctor | 必要アプリ (Ghostty, Vivaldi, Zed, tmux, omniwmctl) の存在 | - | - | - | - | - |
| SSOT-5.6-DOC-INV | §5.6 doctor | 不変条件チェック | - | - | - | - | - |
| SSOT-5.6-DOC-LEVEL | §5.6 doctor | 各検査項目を PASS / WARN / FAIL で報告 | - | - | - | - | - |

### §5.7 CLI コマンド一覧

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-5.7-UP | §5.7 | `projwm up --ai <name> --slot <SLOT>`: project 新規作成と割り当て | - | - | - | - | - |
| SSOT-5.7-ADD-AI | §5.7 | `projwm add-ai --ai <name>`: AI window 追加 | - | - | - | - | - |
| SSOT-5.7-ADD-SHELL | §5.7 | `projwm add-shell` | - | - | - | - | - |
| SSOT-5.7-ADD-EDITOR | §5.7 | `projwm add-editor` | - | - | - | - | - |
| SSOT-5.7-REMOVE | §5.7 | `projwm remove --window <KIND-N>` | - | - | - | - | - |
| SSOT-5.7-PROF-CREATE | §5.7 | `projwm profile create <NAME>` | - | - | - | - | - |
| SSOT-5.7-PROF-SWITCH | §5.7 | `projwm profile switch <NAME>` | - | - | - | - | - |
| SSOT-5.7-PROF-ASSIGN | §5.7 | `projwm profile assign <SLOT> <PROJECT>` | - | - | - | - | - |
| SSOT-5.7-PROF-UNASSIGN | §5.7 | `projwm profile unassign <SLOT>` | - | - | - | - | - |
| SSOT-5.7-PROF-DELETE | §5.7 | `projwm profile delete <NAME>` | - | - | - | - | - |
| SSOT-5.7-ARCHIVE | §5.7 | `projwm archive <PROJECT>` | - | - | - | - | - |
| SSOT-5.7-UNARCHIVE | §5.7 | `projwm unarchive <PROJECT>` (slot は引数なし、§4.5 park 復帰) | - | - | - | - | - |
| SSOT-5.7-JUMP | §5.7 | `projwm jump <SLOT\|PROJECT>` | - | - | - | - | - |
| SSOT-5.7-RECONCILE | §5.7 | `projwm reconcile [--dry-run]` | - | - | - | - | - |
| SSOT-5.7-STATUS | §5.7 | `projwm status [--json]` | - | - | - | - | - |
| SSOT-5.7-DOCTOR | §5.7 | `projwm doctor` | - | - | - | - | - |
| SSOT-5.7-TRACE | §5.7 | `projwm trace [--last\|<txid>]` | - | - | - | - | - |
| SSOT-5.7-TUI | §5.7 | `projwm tui`: cockpit 手動起動 | - | - | - | - | - |
| SSOT-5.7-BR-ADD | §5.7 (§4.1 OP14) | `projwm browser add-tab --project <P> --url <URL>` | - | - | - | - | - |
| SSOT-5.7-BR-REMOVE | §5.7 (§4.1 OP15) | `projwm browser remove-tab ...` | - | - | - | - | - |
| SSOT-5.7-BR-CHANGE | §5.7 (§4.1 OP16) | `projwm browser change-tab-url ...` | - | - | - | - | - |
| SSOT-5.7-BR-REORDER | §5.7 (§4.1 OP17) | `projwm browser reorder-tabs ...` (主に観測経由だが CLI 経路も) | - | - | - | - | - |

---

## §6. 設計原則

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-6.1-SESSION | §6.1 | tmux session が真の生存、window は画面でしかない、session 消失 = project 死 (archive 同等) | - | - | - | - | - |
| SSOT-6.2-IDENTITY | §6.2 | window の正体は (project,kind,id)、場所 (slot) は後付け、ユーザー操作は identity に対して | - | - | - | - | - |
| SSOT-6.3-L1 | §6.3 | Level 1 identity: DesiredWorld.Projects[].Windows[] vs ObservedWorld.Windows[] (title から識別)、修正は spawn/close | - | - | - | - | - |
| SSOT-6.3-L2 | §6.3 | Level 2 placement: DesiredProfile.Assignments vs ObservedWindow.Workspace、修正は move-to-workspace | - | - | - | - | - |
| SSOT-6.3-L3 | §6.3 | Level 3 ordering: DesiredLayout.Columns vs ObservedLayout.Columns、修正は reorder-columns | - | - | - | - | - |
| SSOT-6.3-PRIO | §6.3 | 優先度: L1 > L2 > L3、ユーザーは L1 だけ操作、L2/L3 はシステム | - | - | - | - | - |
| SSOT-6.4-OWN | §6.4 | DesiredWorld は projwm が唯一 authority、ユーザーは intent 発行のみ、直接書き換えない | - | - | - | - | - |
| SSOT-6.4-CONSTRAINT | §6.4 | 不変条件 (§3.4) が DesiredWorld のとりうる範囲を制限、違反 intent は拒否 | - | - | - | - | - |
| SSOT-6.5-SINGLE | §6.5 | WM mutation は projwmd のみ、IPC 経由集約、read-only コマンドは直接 WM 読む、`wmMutationLock` で直列化 | - | - | - | - | - |
| SSOT-6.6-IDEMP | §6.6 | 全操作冪等、identity (§6.2) と紐付いて初めて成立、「開く」は identity で既存検出 → focus、無ければ作る | - | - | - | - | - |
| SSOT-6.7-TEST | §6.7 | 各操作独立テスト可、adapter interface で WM 抽象化、fake adapter で unit、リトライ・タイムアウトは体系 | - | - | - | - | - |
| SSOT-6.8-GRACE | §6.8 | 部分失敗で全体壊れない、1 window spawn 失敗でも他継続、次 iteration で replan、cockpit 表示 | - | - | - | - | - |
| SSOT-6.9-IDPERS | §6.9 | (project,kind,id) は macOS 再起動後も回復可、title に符号化 (naming.Resolve)、Ghostty `--title` 固定 | - | - | - | - | - |
| SSOT-6.10-ORDER-CSO | §6.10 | close → observe-barrier → spawn (逆順だと slot が埋まる) | - | - | - | - | - |
| SSOT-6.10-ORDER-SSV | §6.10 | spawn → settle → verify | - | - | - | - | - |
| SSOT-6.10-ORDER-PSWITCH | §6.10 | profile switch: 旧 close 全 → barrier → 新 spawn 全 | - | - | - | - | - |
| SSOT-6.10-ORDER-ARCH | §6.10 | archive: 全 close → tmux kill → state 更新 | - | - | - | - | - |

---

## §7. アーキテクチャ

### §7.1 Transaction Loop

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-7.1-OBSERVE | §7.1 | Observe phase: WM から最新 ObservedWorld 取得 | - | - | - | - | - |
| SSOT-7.1-REDUCE | §7.1 | Reduce phase: (WorldState, Intent) → DesiredWorld、純粋関数 | - | - | - | - | - |
| SSOT-7.1-PLAN | §7.1 | Plan phase: (WorldState, DesiredWorld) → []Operation、決定論的 rule-based | - | - | - | - | - |
| SSOT-7.1-EXEC | §7.1 | Execute phase: Operation を adapter 経由で実行、Phase A/B/C 分離 | - | - | - | - | - |
| SSOT-7.1-SETTLE | §7.1 | Settle phase: 状態安定化 wait | - | - | - | - | - |
| SSOT-7.1-VERIFY | §7.1 | Verify phase: PredictedWorld と ObservedWorld 比較 | - | - | - | - | - |
| SSOT-7.1-COMMIT | §7.1 | Commit phase: 世代進める、PersistentStore に保存 | - | - | - | - | - |
| SSOT-7.1-PHASE-A | §7.1 | Phase A: removals (close, kill-session) | - | - | - | - | - |
| SSOT-7.1-PHASE-B | §7.1 | Phase B: spawns (spawn-terminal/editor/browser/viewer/cockpit) | - | - | - | - | - |
| SSOT-7.1-PHASE-C | §7.1 | Phase C: layout (move-to-workspace, reorder-columns, focus) | - | - | - | - | - |
| SSOT-7.1-BARRIER | §7.1 | 各 phase 間に `observe-barrier` 挿入し、前 phase 変更が WM 反映されるのを待つ | - | - | - | - | - |
| SSOT-7.1-MAXREPLAN-FAIL | §7.1 | max replans 超過時: トランザクション fail、commit されない | - | - | - | - | - |
| SSOT-7.1-MAXREPLAN-ROLLBACK | §7.1 | max replans 超過時: WorldState をトランザクション開始前にロールバック | - | - | - | - | - |
| SSOT-7.1-MAXREPLAN-CARD | §7.1 | max replans 超過時: cockpit に [INVARIANT] カード通知 | - | - | - | - | - |
| SSOT-7.1-MAXREPLAN-RETRY | §7.1 | max replans 超過時: 次の intent/event 到来時に再挑戦 (自動リトライしない)、dirty scope 記録 | - | - | - | - | - |

### §7.2 パッケージ境界

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-7.2-CMD | §7.2 | cmd/: projwmd / projwmctl / projwmevent / projwm / projwm-cockpit / projwmstore-bootstrap | - | - | - | - | - |
| SSOT-7.2-INT-ADAPTER | §7.2 | internal/adapter/: wm / browser / zed / session | - | - | - | - | - |
| SSOT-7.2-INT-CORE | §7.2 | internal/: controller / reducer / planner / executor / settler / simulator / verifier / store / world / intent / event / op | - | - | - | - | - |
| SSOT-7.2-INT-AUX | §7.2 | internal/: invariant / manifest / ipc / identity / naming / semop | - | - | - | - | - |

### §7.3 命名規約

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-7.3-AI-TMUX | §7.3 | ai tmux session: `ai-<id>/<project>` (例 `ai-1/dotfiles`) | - | - | - | - | - |
| SSOT-7.3-AI-TITLE | §7.3 | ai ghostty title: `ai-<id>:<project>` (例 `ai-1:dotfiles`) | - | - | - | - | - |
| SSOT-7.3-VIEWER-TMUX | §7.3 | viewer tmux: `ai-<id>/<project>_v` | - | - | - | - | - |
| SSOT-7.3-VIEWER-TITLE | §7.3 | viewer title: `ai-view-<id>:<project>` | - | - | - | - | - |
| SSOT-7.3-SHELL-TMUX | §7.3 | shell tmux: `shell-<id>/<project>` | - | - | - | - | - |
| SSOT-7.3-SHELL-TITLE | §7.3 | shell title: `shell-<id>:<project>` | - | - | - | - | - |
| SSOT-7.3-EDITOR | §7.3 | editor (Zed) title: `basename(<cwd>)` | - | - | - | - | - |
| SSOT-7.3-BROWSER | §7.3 | browser (Vivaldi) title: `browser-<id>:<project>` | - | - | - | - | - |
| SSOT-7.3-COCKPIT-TMUX | §7.3 | cockpit tmux: `projwm-cockpit` | sigwm.go:2126 (cockpitBaseSession const) | implemented | sigwm_test.go: TestSigWM_SpawnCockpit_* | behavior | - |
| SSOT-7.3-COCKPIT-TITLE | §7.3 | cockpit title: `projwm-cockpit-<display>` (例 `projwm-cockpit-0`) | sigwm.go:2130 cockpitCloneName; reducer.go:296 const cockpitTitle; controller.go:782 invariant check | implemented | sigwm_test.go: TestSigWM_Close_CockpitBypassesBlock; planner_cockpit_test.go; reducer_cockpit_test.go | behavior | S27a |
| SSOT-7.3-SCRATCH-TMUX | §7.3 | scratch tmux: `projwm-scratch-shell` | - | - | - | - | - |
| SSOT-7.3-SCRATCH-TITLE | §7.3 | scratch title: `projwm-scratch-shell` | - | - | - | - | - |
| SSOT-7.3-ED-MULTI | §7.3 | editor (Zed) 複数 project で同 basename の場合は bundleId+title+workspace で識別 | - | - | - | - | - |
| SSOT-7.3-SLASH | §7.3 | Ghostty title は `:` 、tmux session 名は `/` (tmux が `:` を許容しない) | - | - | - | - | - |

### §7.4 ドメインモデル

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-7.4-WORLD | §7.4 | WorldState = {Environment, Desired, Observed, Predicted, Meta} | - | - | - | - | - |
| SSOT-7.4-ENV | §7.4 | ManagedEnvironment: windowManager / workspaces / slots / apps / daemons (Nix author、projwmd 読み込み) | - | - | - | - | - |
| SSOT-7.4-DESIRED | §7.4 | DesiredWorld: ActiveProfile, Profiles, Projects, FocusPolicy, CockpitVisibility, SystemWindows | - | - | - | - | - |
| SSOT-7.4-OBSERVED | §7.4 | ObservedWorld: Windows, Workspaces, Displays, Focused, Tmux, Timestamp | - | - | - | - | - |
| SSOT-7.4-IDS | §7.4 | ID 識別子: ProfileID/ProjectID/SlotID/WorkspaceID/DesiredWindowID/LiveWindowID/DisplayID | - | - | - | - | - |
| SSOT-7.4-KIND-AI | §7.4 | WindowKind=ai: AI CLI, Ghostty, tmux あり | - | - | - | - | - |
| SSOT-7.4-KIND-SHELL | §7.4 | WindowKind=shell: 自由 shell, Ghostty, tmux | - | - | - | - | - |
| SSOT-7.4-KIND-EDITOR | §7.4 | WindowKind=editor: GUI editor, Zed, tmux なし | - | - | - | - | - |
| SSOT-7.4-KIND-BROWSER | §7.4 | WindowKind=browser: Vivaldi, tmux なし | - | - | - | - | - |
| SSOT-7.4-KIND-VIEWER | §7.4 | WindowKind=viewer: AI read-only 複製, Ghostty, tmux grouped | - | - | - | - | - |
| SSOT-7.4-KIND-EXTERNAL | §7.4 | WindowKind=external: 管理対象外 | - | - | - | - | - |
| SSOT-7.4-KIND-COCKPIT | §7.4 | WindowKind=cockpit: TUI 操縦席, Ghostty, tmux | - | - | - | - | - |
| SSOT-7.4-KIND-SCRATCH | §7.4 | WindowKind=scratch: 一時作業 shell, Ghostty, tmux | - | - | - | - | - |

### §7.5 アダプタ契約

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-7.5-WM-OBSERVE | §7.5 WindowManager | `Observe(ctx) (ObservedWorld, error)` | - | - | - | - | - |
| SSOT-7.5-WM-SPAWN | §7.5 WindowManager | `Spawn(ctx, SpawnRequest) (LiveWindowID, error)` | - | - | - | - | - |
| SSOT-7.5-WM-CLOSE | §7.5 WindowManager | `Close(ctx, LiveWindowID) error` (raw close, production blocked) | sigwm.go:1158 (production-blocked); cockpit title prefix `projwm-cockpit-` bypasses (1176) | implemented | sigwm_test.go: TestSigWM_Close_*, TestSigWM_Close_CockpitBypassesBlock | behavior | S27a |
| SSOT-7.5-WM-FOCUSWS | §7.5 WindowManager | `FocusWorkspace(ctx, WorkspaceID) error` | sigwm.go:2036 | implemented | ssot_l3_wm_real_ops_test.go: TestRealOpsFocusWorkspace | behavior | - |
| SSOT-7.5-WM-FOCUSWIN | §7.5 WindowManager | `FocusWindow(ctx, LiveWindowID) error` (navigate→focus 2-step) | sigwm.go:2054 | implemented | ssot_l2_mock_executor_test.go: TestFocusWindowNavigationBeforeFocus; sigwm_test.go: TestSigWM_FocusWindow | behavior | S27b |
| SSOT-7.5-WM-MOVE | §7.5 WindowManager | `MoveToWorkspace(ctx, LiveWindowID, WorkspaceID) error` | sigwm.go:1607 | implemented | ssot_l2_mock_executor_test.go: TestMoveToWorkspace*; ssot_l3_wm_spec_test.go | behavior | - |
| SSOT-7.5-WM-REORDER | §7.5 WindowManager | `ReorderColumns(ctx, WorkspaceID, [][]LiveWindowID) error` | sigwm.go | implemented | ssot_l3_wm_spec_test.go: TestReorderColumns* | behavior | - |
| SSOT-7.5-WM-SPAWNCP | §7.5 WindowManager | `SpawnCockpit(ctx, displayIdx int, title string) error` | sigwm.go:2179 | implemented | ssot_l2_mock_executor_test.go: TestSpawnSettleTimeout*; sigwm_test.go: TestSigWM_SpawnCockpit_* | behavior | - |
| SSOT-7.5-WM-SHOWCP | §7.5 WindowManager | `ShowCockpitOnDisplay(ctx, DisplayID, parkWS string) error` | sigwm.go:2367 | implemented | (no L1/L2 owner yet — covered indirectly via L4) | partial | S22 |
| SSOT-7.5-WM-HIDECP | §7.5 WindowManager | `HideCockpitOnDisplay(ctx, DisplayID, priorWS string) error` | sigwm.go:2394 | implemented | executor_test.go: TestExecuteHideCockpitRestoresPriorWindowFocus | behavior | - |
| SSOT-7.5-WM-MOVECP | §7.5 WindowManager | `MoveCockpitToParkWorkspace(ctx, LiveWindowID, parkWS string) error` | adapter.go:111; sigwm.go:2767; fake.go (S27e) | implemented | fake_test.go: TestFakeMoveCockpitToParkWorkspace*; ssot_l2_mock_executor_test.go: TestMoveCockpitToParkWorkspace* | behavior | S27e |
| SSOT-7.5-WM-SHOWSCRATCH | §7.5 WindowManager | `ShowScratchShell(ctx) (LiveWindowID, error)` | adapter.go:120; sigwm.go:2478 (冪等 + process-alive fallback); fake.go | implemented | fake_test.go: TestFakeShowScratchShell*; ssot_l2_mock_executor_test.go: TestShowScratchShell*; ssot_l3_wm_spec_test.go: TestScratchShellShowHideRestoresPriorFocus (L3 owner) | behavior | S27c |
| SSOT-7.5-WM-HIDESCRATCH | §7.5 WindowManager | `HideScratchShell(ctx, priorWindow LiveWindowID) error` | adapter.go:128; sigwm.go:2570 (navigate→focus prior); fake.go | implemented | fake_test.go: TestFakeHideScratchShell*; ssot_l2_mock_executor_test.go: TestHideScratchShell*; ssot_l3_wm_spec_test.go: TestScratchShellShowHideRestoresPriorFocus | behavior | S27d |
| SSOT-7.5-SES-HAS | §7.5 Session | `HasSession(ctx, name) (bool, error)` | - | - | - | - | - |
| SSOT-7.5-SES-ENSURE | §7.5 Session | `EnsureSession(ctx, name, cwd) (created bool, err error)` (冪等) | - | - | - | - | - |
| SSOT-7.5-SES-GROUP | §7.5 Session | `EnsureGroupedSession(ctx, base, clone) error` | - | - | - | - | - |
| SSOT-7.5-SES-KILL | §7.5 Session | `KillSession(ctx, name) error` | - | - | - | - | - |
| SSOT-7.5-SES-KEYS | §7.5 Session | `SendKeys(ctx, session, keys...) error` | - | - | - | - | - |
| SSOT-7.5-ED-LAUNCH | §7.5 Editor | `LaunchProject(ctx, projectPath, extraArgs) error` (zed -n) | - | - | - | - | - |
| SSOT-7.5-ED-COLLECT | §7.5 Editor | `CollectCloseObservation(ctx, params) (CloseObservation, error)` | - | - | - | - | - |
| SSOT-7.5-ED-CLOSE | §7.5 Editor | `CloseLiveWindow(ctx, LiveWindowID) error` (project-scoped-app) | - | - | - | - | - |
| SSOT-7.5-BR-OPEN | §7.5 Browser | `OpenURL(ctx, url, profile) error` (automation profile) | - | - | - | - | - |
| SSOT-7.5-BR-COLLECT | §7.5 Browser | `CollectCloseObservation(ctx, params) (CloseObservation, error)` | - | - | - | - | - |
| SSOT-7.5-BR-CLOSE | §7.5 Browser | `CloseLiveWindow(ctx, LiveWindowID) error` (browser-window-close) | - | - | - | - | - |
| SSOT-7.5-PRINCIPLE | §7.5 | adapter method は L2 (mock/det harness) と L3 (実操作) を分けてテスト、mock/unit body を L3 証拠にしない | - | - | - | - | - |

---

## §8. 状態管理

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-8.1-GEN | §8.1 | generation-based の不変ストア、各コミットで generation ディレクトリ増 | - | - | - | - | - |
| SSOT-8.1-ATOMIC | §8.1 | atomic rename で crash safety | - | - | - | - | - |
| SSOT-8.1-SAVE-DESIRED | §8.1 | DesiredWorld を保存 | - | - | - | - | - |
| SSOT-8.1-SAVE-LAYOUTS | §8.1 | AcceptedLayouts を保存 | - | - | - | - | - |
| SSOT-8.1-SAVE-BROWSER | §8.1 | BrowserSnapshots を保存 | - | - | - | - | - |
| SSOT-8.1-SAVE-CHKPOINT | §8.1 | ControllerCheckpoint を保存 | - | - | - | - | - |
| SSOT-8.1-NO-OBSERVED | §8.1 | ObservedWorld は保存しない (起動時に observer で再構成) | - | - | - | - | - |
| SSOT-8.2-COMPUTED | §8.2 | title / tmux session / viewer 窓は state に保存せず naming.Resolve() で算出、rename 時の不整合を構造的防止 | - | - | - | - | - |
| SSOT-8.3-FLOCK | §8.3 | 全書き込みは flock(2) で排他 | - | - | - | - | - |
| SSOT-8.3-TMPFILE | §8.3 | 書き込みは tmpfile + atomic rename | - | - | - | - | - |
| SSOT-8.3-READ-NOLOCK | §8.3 | 読み込みは lock 不要 | - | - | - | - | - |

---

## §9. 受入仕様

| ID | 節 | 要求 | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-9.1-S1 | §9.1 S1 | SwitchProfile: 旧 close、新 summon | - | - | - | - | - |
| SSOT-9.1-S2 | §9.1 S2 | ArchiveProject: window close + tmux kill | - | - | - | - | - |
| SSOT-9.1-S3 | §9.1 S3 | UnarchiveProject: park 状態に復帰 | - | - | - | - | - |
| SSOT-9.1-S4 | §9.1 S4 | Assign/Unassign: slot 割当と解除 | - | - | - | - | - |
| SSOT-9.1-S5 | §9.1 S5 | Reconcile: 差分修正 | - | - | - | - | - |
| SSOT-9.1-S6 | §9.1 S6 | macOS 再起動後: 全自動復帰 | - | - | - | - | - |
| SSOT-9.1-S7 | §9.1 S7 | OmniWM 再起動後: 窓の再作成 | - | - | - | - | - |
| SSOT-9.1-S8 | §9.1 S8 | summon の冪等性 | - | - | - | - | - |
| SSOT-9.1-S9 | §9.1 S9 | drift 修正: slot 外から自動復帰 | - | - | - | - | - |
| SSOT-9.1-S10 | §9.1 S10 | 障害復帰: tmux/Ghostty/Zed クラッシュ後の自動復帰 | - | - | - | - | - |
| SSOT-9.2-DOD1 | §9.2 | 全受入シナリオが real E2E (Human-operation) でパス | - | - | - | - | - |
| SSOT-9.2-DOD2 | §9.2 | 全不変条件 (§3.4) が invariant checker で検証 | - | - | - | - | - |
| SSOT-9.2-DOD3 | §9.2 | 1 分以内の自動復帰を保証 | - | - | - | - | - |
| SSOT-9.2-DOD4 | §9.2 | プロファイル切替が 5 秒以内 | - | - | - | - | - |
| SSOT-9.2-DOD5 | §9.2 | 個別操作が独立テスト可能 | - | - | - | - | - |

---

## §10. テスト戦略 (要求 + GAP-01〜26 再掲)

§10.4 の 43 件単一操作テスト、§10.6 カバレッジ表、§10.9 GAP-01〜26 は既に SSOT
原文で表形式 + ID 付きで整理済みなので、ここでは GAP 行のみ転写する。
詳細は SSOT §10 を参照。

| ID | 節 | 要求 (未保証領域) | 実装 | status | テスト owner | evidence | 切片候補 |
|---|---|---|---|---|---|---|---|
| SSOT-10.9-GAP01 | §10.9 GAP-01 | duplicate window の正本選択 (§2.5 最 recently focused、余剰 orphan、[INVARIANT] card) | - | - | - | - | - |
| SSOT-10.9-GAP02 | §10.9 GAP-02 | 状態一覧のユーザー可視表示 (§3.1 各状態の cockpit/status 表示) | - | - | - | - | - |
| SSOT-10.9-GAP03 | §10.9 GAP-03 | 状態ごとの操作可否 (§3.3 state × operation rejection/wait matrix) | - | - | - | - | - |
| SSOT-10.9-GAP04 | §10.9 GAP-04 | 手動 drift 通知 + grace period (§4.3 60s 内 2 回) | - | - | - | - | - |
| SSOT-10.9-GAP05 | §10.9 GAP-05 | orphan card 3 action ([Enter]/[c]/[t]) の実行 | - | - | - | - | - |
| SSOT-10.9-GAP06 | §10.9 GAP-06 | AI spawn 詳細 (send-keys / attach-only / multi-AI parity) | - | - | - | - | - |
| SSOT-10.9-GAP07 | §10.9 GAP-07 | Zed spawn 詳細 (-n --user-data-dir / 設定分離 / pre-existing empty project 非干渉) | - | - | - | - | - |
| SSOT-10.9-GAP08 | §10.9 GAP-08 | Vivaldi profile isolation (user profile = External) | - | - | - | - | - |
| SSOT-10.9-GAP09 | §10.9 GAP-09 | browser tab 自動観測 (manual tab 操作 → observer → DesiredWorld/private payload) | - | - | - | - | - |
| SSOT-10.9-GAP10 | §10.9 GAP-10 | browser 復元と privacy 完全性 (失敗時空タブ、cookie/token 非保存、全出力 redaction) | - | - | - | - | - |
| SSOT-10.9-GAP11 | §10.9 GAP-11 | Cockpit TUI 全体構造 (topbar / tabs / footer の全領域 SSOT snapshot test) | - | - | - | - | - |
| SSOT-10.9-GAP12 | §10.9 GAP-12 | Cockpit wizard / palette / modes (各 mode の入出/intent 発行/visibility 復帰) | - | - | - | - | - |
| SSOT-10.9-GAP13 | §10.9 GAP-13 | status / doctor 完全出力 (全項目 presence と failure 分類) | - | - | - | - | - |
| SSOT-10.9-GAP14 | §10.9 GAP-14 | CLI 全コマンド (profile create/delete、doctor、trace、tui、browser tab 等) の state/IPC 効果 | - | - | - | - | - |
| SSOT-10.9-GAP15 | §10.9 GAP-15 | 状態階層の優先順位 (L1 > L2 > L3 で複合 drift 解決順序) | - | - | - | - | - |
| SSOT-10.9-GAP16 | §10.9 GAP-16 | single writer / mutation lock (全 mutation 経路の禁止/許可、並行 intent 直列化) | - | - | - | - | - |
| SSOT-10.9-GAP17 | §10.9 GAP-17 | graceful degradation (部分失敗継続、次回修復、user-visible card) | - | - | - | - | - |
| SSOT-10.9-GAP18 | §10.9 GAP-18 | operation order 全体 (profile/archive の全 phase order と barrier 効果) | - | - | - | - | - |
| SSOT-10.9-GAP19 | §10.9 GAP-19 | max replans 超過時全挙動 (rollback/card/dirty scope/next retry の統合テスト) | - | - | - | - | - |
| SSOT-10.9-GAP20 | §10.9 GAP-20 | MoveCockpitToParkWorkspace の L2/L3 owner | - | - | - | - | - |
| SSOT-10.9-GAP21 | §10.9 GAP-21 | Zed/Vivaldi close observation (実 app で CollectCloseObservation の証拠検証) | - | - | - | - | - |
| SSOT-10.9-GAP22 | §10.9 GAP-22 | PersistentStore 完全性 (artifact presence / no ObservedWorld / crash-safe generation) | - | - | - | - | - |
| SSOT-10.9-GAP23 | §10.9 GAP-23 | 排他制御 (concurrent writer / interrupted write / reader during write) | - | - | - | - | - |
| SSOT-10.9-GAP24 | §10.9 GAP-24 | 完了定義の時間保証 (1 分復帰 / 5s profile switch の実 E2E timing) | - | - | - | - | - |
| SSOT-10.9-GAP25 | §10.9 GAP-25 | L3/L4 実行条件強制 (skip-green を成功扱いしない gate / CI profile) | - | - | - | - | - |
| SSOT-10.9-GAP26 | §10.9 GAP-26 | テスト環境分離 (全 real/integration test の prefix/path/workspace meta-audit) | - | - | - | - | - |

---

## マトリクス summary (要求数集計)

| 節 | 要求数 |
|---|---|
| §2 (mental model) | 12 + 3 + 7 + 4 + 10 + 5 = 41 |
| §3 (system state) | 8 + 5 + 12 + 12 = 37 |
| §4 (operations) | 19 + 5 + 11 + 36 + 6 + 8 = 85 |
| §5 (UI) | 8 + 22 + 4 + 16 + 22 = 72 |
| §6 (design principles) | 17 |
| §7 (architecture) | 15 + 4 + 14 + 13 + 25 = 71 |
| §8 (state management) | 11 |
| §9 (acceptance) | 15 |
| §10 (test strategy GAP) | 26 |
| **合計** | **約 375** |

要求文を atomic に分解した結果、SSOT v1.11 は **約 375 件** の検証可能要求文を含む。
これを 25-40 切片に再編する作業が次フェーズ。
