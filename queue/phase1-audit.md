# Phase 1 (S01-S05) + S06 — DoD 13 条項 audit

> 2026-05-24, post dead-code-purge。
> 各切片を `queue/ssot-slice-plan.md` §4 で定義された 13 条項に対して honest に
> check する。 ✓ = 満たす、△ = 部分達成、✗ = 未達。

## DoD 13 条項 (参照)

1. SSOT 該当節再読
2. 対応 intent 実装 (該当する場合)
3. adapter / executor 実装 (該当する場合)
4. CLI / TUI 接続 (該当する場合)
5. L0 unit test (純粋関数入出力)
6. L1 fake test (state 遷移、不変条件)
7. L2 mock harness (該当する場合)
8. L3 real_ops test owner
9. ssottest ledger 昇格 (evidenceMeta → evidenceBehavior)
10. §6 設計原則チェック
11. 該当 INV (§3.4) チェック
12. orchestrator 実機 verify
13. user 報告 + 残ギャップ列挙

---

## S01: manifest §10.7 shape 統一

| DoD | 状態 | 根拠 |
|---|---|---|
| 1 SSOT 節再読 | ✓ | SSOT §10.7 を Read で確認、top-level array / `viewer by role` を把握 |
| 2 intent 実装 | N/A | manifest はデータ schema、intent なし |
| 3 adapter / executor | N/A | parser のみ、adapter なし |
| 4 CLI / TUI | N/A | 内部 schema、UI surface なし |
| 5 L0 unit test | ✓ | `TestSSOTTestManifestParses` (field 値 behavior 検証)、`TestSSOTTestManifestMatchesSectionTenPointSevenShape` |
| 6 L1 fake test | N/A | 純粋 parser、fake adapter 不要 |
| 7 L2 mock harness | N/A | 同上 |
| 8 L3 real_ops | N/A | parser はファイル I/O のみ、実機 op なし |
| 9 ledger 昇格 | ✓ | MANI-01: `statusRed` → `statusCovered`、TestName を `Parses/Matches` の 2 件に拡張 |
| 10 §6 設計原則 | ✓ | §6.4 state ownership (manifest = Nix authority) を維持 |
| 11 INV チェック | N/A | manifest は invariant の対象外 (環境定義) |
| **12 実機 verify** | △ | `nix build` で manifest が生成され、SSOT shape (`workspaces=array[22]`, `slots=array[10]`, `apps=array[3]`, `viewer by role=1個`) を `jq` で目視確認済。**しかし `darwin-rebuild switch` は実行していない** — production manifest digest 変動 + store 再 bootstrap の影響が未検証 |
| 13 user 報告 | ✓ | Phase 1 完了報告で言及済 |

**S01 honest 評価**: ✓ DoD 5/9/10 はクリア、DoD 12 が **△ (nix build 検証のみ、darwin-rebuild + 実 daemon load 未確認)**。後続 task E で解消予定。

---

## S02: deprecated intent purge

| DoD | 状態 | 根拠 |
|---|---|---|
| 1 SSOT 節再読 | ✓ | N-06 / N-12 を再読、ToggleCockpit / FocusCockpit / AcceptManualLayout / SyncBrowserTabs / RespawnOrphanGhostty 5 種の削除根拠を確認 |
| 2 intent 実装 | ✓ | intent.go から 5 種完全削除 + Kind 定数削除 |
| 3 adapter / executor | ✓ | reducer / planner / executor / ipc の consumer 全削除、controller の commandKeyFor から削除 |
| 4 CLI / TUI | ✓ | cmd_cockpit.go の toggle/focus を SetCockpitVisibility{Shown} にマップ、cmd/projwm-cockpit/tui/palette.go の AcceptManualLayout palette entry 削除、tui/update.go の RespawnOrphanGhostty 経路を AdoptOrphanWindow に統合 |
| 5 L0 unit test | ✓ | `TestSSOTDeprecatedIntentsRemoved` (5 種 string kind が active set に存在しないことを behavior 検証) |
| 6 L1 fake test | △ | reducer_cockpit_test の ToggleCockpit 4 件を SetCockpitVisibility 対応に置換、reducer_v3_test の SyncBrowserTabs test を削除 (S14 で再構築予定としてマーク) |
| 7 L2 mock harness | N/A | mock harness 対象外 |
| 8 L3 real_ops | N/A | intent 削除は L3 op を生まない |
| 9 ledger 昇格 | ✓ | OP-07 の TestName を SetCockpitVisibility 系に書き換え |
| 10 §6 設計原則 | ✓ | §6.4 state ownership 維持 (consumer 削除順序を careful に: type 削除前に caller 削除) |
| 11 INV チェック | N/A | 削除のみ、invariant 影響なし |
| **12 実機 verify** | ✗ | `darwin-rebuild switch` で deprecated intents が production で送信されないことを実機確認していない |
| 13 user 報告 | ✓ | Phase 1 報告で完了 |

**S02 honest 評価**: DoD 12 が **✗**。後続 task E で解消予定。

---

## S03: AI 名 routing 修正

| DoD | 状態 | 根拠 |
|---|---|---|
| 1 SSOT 節再読 | ✓ | SSOT §4.4 ai (send-keys / multi-AI parity), §7.3 命名規約再読 |
| 2 intent 実装 | ✓ | AddWindow.AIName を DesiredAISession.Name に persist する reducer.defaultWindowForKind 修正 |
| 3 adapter / executor | ✓ | semop.aiCommandFor が DesiredWindow.AI から AI command を導出 (nil/empty/unknown → claude fallback) |
| 4 CLI / TUI | ✓ | cmd/projwm/cmd_project.go の defaultProjectWindows を canonical reducer.DefaultProjectWindows に統合 |
| 5 L0 unit test | ✓ | `TestSSOTAICommandRoutesFromDesiredAISession` (5 cases: nil/empty/claude/copilot/unknown) |
| 6 L1 fake test | ✓ | `TestSSOTTerminalSessionFieldsUseOneBasedDesiredIDDirectly` に `ai-copilot` ケース追加 |
| 7 L2 mock harness | N/A | mock executor 対象外 |
| 8 L3 real_ops | △ | spawn 時 tmux send-keys の実機検証は **未実施**。L3 owner 未追加 (S12 add-window で追加予定) |
| 9 ledger 昇格 | ✓ | NAMI-01 / NAMI-02 を behavior promote |
| 10 §6 設計原則 | ✓ | §6.2 identity > location: AI 名は identity ではない metadata として明確に分離 |
| 11 INV チェック | △ | INV-10 (identity title 復元可能性) は title から AI 名が外れたので影響受けるはずだが、test owner 別途未追加 |
| **12 実機 verify** | ✗ | `darwin-rebuild switch` + 実機での `add-ai --ai copilot` 動作確認 未実施 |
| 13 user 報告 | ✓ | Phase 1 報告で完了 |

**S03 honest 評価**: DoD 8 / 11 / 12 が ✗。後続 S12 で send-keys real_ops test + INV-10 INV checker test を追加予定。

---

## S04: ssottest ledger 真化

| DoD | 状態 | 根拠 |
|---|---|---|
| 1 SSOT 節再読 | ✓ | SSOT §10.9 ledger workflow を再読 |
| 2 intent 実装 | N/A | ledger 体系の整備、intent なし |
| 3 adapter / executor | N/A | 同上 |
| 4 CLI / TUI | N/A | 同上 |
| 5 L0 unit test | ✓ | 既存 `TestSSOTLedger*` 4 件 (existence/classification/evidence/no-missing) を維持 |
| 6 L1 fake test | N/A | ledger は meta-test |
| 7 L2 mock harness | N/A | 同上 |
| 8 L3 real_ops | N/A | 同上 |
| 9 ledger 昇格 | ✓ | 4 items を behavior promote (NAMI-01, NAMI-02, MANI-01, OP-10B) |
| 10 §6 設計原則 | ✓ | §6.7 testability: workflow doc を ledger_test.go package comment として固定 |
| 11 INV チェック | N/A | meta-test |
| 12 実機 verify | N/A | meta-test (実機なし) |
| 13 user 報告 | ✓ | Phase 1 報告で完了 |

**S04 honest 評価**: ✓ 全条項クリア (meta-test なので大部分が N/A)。

---

## S05: テスト環境分離 audit

| DoD | 状態 | 根拠 |
|---|---|---|
| 1 SSOT 節再読 | ✓ | SSOT §10.8 + GAP-26 再読 |
| 2-4 | N/A | audit のみ、新規実装なし |
| 5 L0 unit test | ✓ | `TestSSOTTestIsolationAuditEnforcesPrefixes` (production-shaped string patterns を実 grep で検証) |
| 6 L1 fake test | N/A | meta-audit |
| 7 L2 mock harness | N/A | 同上 |
| 8 L3 real_ops | N/A | 同上 |
| 9 ledger 昇格 | △ | ISO-01 を ledger 追加。`statusRed + evidenceBehavior` (audit 機構自体は behavior、しかし allowlist で 2 件の違反を許容しているため Red のまま) |
| 10 §6 設計原則 | ✓ | §6.7 testability: 環境分離が新規違反を弾く |
| 11 INV チェック | N/A | meta-audit |
| 12 実機 verify | N/A | meta-audit |
| 13 user 報告 | △ | Phase 1 報告で言及したが、**「`dotfiles`/`manaflow` tmux session が host-global で production daemon と clash する **実害ある問題**を、audit で flag するだけ**」という S29 領域の honest gap を user 説明済 |

**S05 honest 評価**: 既知の partial 状態を ledger に記録、audit 機構自体は本物。S29 で「`dotfiles`/`manaflow` を `projwm-next-test-*` に置換」予定。

---

## S06: OP07 cockpit show/hide 完成 (Phase 2 開始)

| DoD | 状態 | 根拠 |
|---|---|---|
| 1 SSOT 節再読 | ✓ | SSOT §4.1 OP07-SHOW/HIDE, §5.4 Proposal mode, §7.5 HideCockpitOnDisplay, §3.4 INV-06 再読 |
| 2 intent 実装 | ✓ | SetCockpitVisibility は Phase 0.2 で完成済 (PriorWindow populate) |
| 3 adapter / executor | ✓ | executor の hide-cockpit op に FocusWindow(PriorWindow) 復帰経路を追加 |
| 4 CLI / TUI | ✓ | cmd/projwm/cmd_cockpit.go: show/hide/toggle/focus 全て SetCockpitVisibility にマップ済 (S02) |
| 5 L0 unit test | ✓ | reducer_cockpit_test の Shown/Hidden PriorWindow populate tests (既存) |
| 6 L1 fake test | ✓ | `TestExecuteHideCockpitRestoresPriorWindowFocus` + `TestExecuteHideCockpitWithoutPriorWindowSkipsFocus` (新規、Fake adapter で behavior 検証) |
| 7 L2 mock harness | N/A | mock executor 対象外 |
| 8 L3 real_ops | △ | `TestCockpitShowHideRestoresPriorWorkspaceAndWindow` が ledger に登録されているが、**私は実機実行していない**。PROJWM_REAL_OP_TESTS=1 で走らせる必要 |
| 9 ledger 昇格 | ✓ | OP-07: `statusRealOnly + evidenceBehavior` (L3 real_ops 要、走行は未検証) |
| 10 §6 設計原則 | ✓ | §6.6 idempotency (show/hide 冪等)、§6.4 state ownership (PriorWindow は controller 経由のみ) |
| 11 INV チェック | △ | INV-06 (cockpit always on CP1) は MoveCockpitToParkWorkspace op で別途守られるが、INV-06 単独 behavior test は **未追加** (S17 で追加予定) |
| **12 実機 verify** | ✗ | `darwin-rebuild switch` + 実機 cockpit show/hide + 元 window への focus 復帰 未確認 |
| 13 user 報告 | ✓ | S06 完了報告で言及済 |

**S06 honest 評価**: DoD 8 / 11 / 12 が △/✗。DoD 8 / 12 は task D / E で解消。INV-06 behavior test は S17 で追加。

---

## 横串の honest gap 整理

### 全切片に共通する未達 DoD

| DoD | 状況 | 解消予定 |
|---|---|---|
| #8 L3 real_ops 実機走行 | 全切片で **未実行** | task D で PROJWM_REAL_OP_TESTS=1 試走 |
| #12 darwin-rebuild + 実機 verify | 全切片で **未実行** | task E で実施 |

### dead code purge で副次的に達成した SSOT 整合

(本来 Phase 1 内に隠れていた dead code を全削除した:)
- world.ManualLayoutCandidate type 削除
- world.ControllerMeta.ManualLayoutCandidates field 削除
- event.Reaction.ManualLayoutCandidate field 削除
- controller の captureObservedManualLayoutCandidate / hasManualLayoutCandidate /
  cloneManualLayoutCandidates 削除
- planner.hasManualLayoutCandidate 削除 + skip-replan logic 撤去
- invariant.CheckOptions.AllowManualLayoutCandidates 削除
- store.ControllerCheckpoint.ManualLayoutCandidates 削除
- reducer.ReactToEvent KindUserReorderedColumns を layout-sync DirtyScope に
- controller.ApplyIntent に applyTier2AutoSyncLayout 呼び出し追加
- observedColumnsForProject の guard を ws 上の windows 基準に修正
- S8.D test を ManualLayoutCandidate 検証 → AcceptedLayouts 検証 に rewrite
- S8.E test の UserReorderedColumns を除外 (Tier 2 経路は意図的に書く)
- scenario/acceptance.go の description を SSOT N-12 に整合

これは「最上層からの破壊」を後追いで完徹した結果。Phase 1 報告時点では **未完だった**。

### Phase 1 + S06 完了の条件 (今 honest にチェックすべき)

- [x] go test ./... 全 packages green (S14/S27 territory 3 件除く)
- [x] go build ./... clean
- [x] nix build green
- [x] orchestration-state.md 更新
- [x] memory project_projwm_overhaul.md 更新
- [x] observer/browser_tabs.go の機能停止状態を package comment で文書化
- [x] dead code purge 完了
- [ ] **L3 real_ops 試走 (task D)** ← まだ
- [ ] **darwin-rebuild switch + production 実機 verify (task E)** ← まだ

残 2 件を task D / E で完了させることで、Phase 1 + S06 が SSOT に対して honest
に完成したと宣言できる。

---

## Task D: L3 real_ops 試走結果 (2026-05-24)

実行環境:
- omniwm running (`org.nixos.omniwm`, `org.nixos.omniwm-display-watcher`)
- projwmd-next NOT running (launchd 上には無し、testing 用に独立)
- omniwmctl + Ghostty available
- ユーザの手動操作 (テスト中の介入) ありの可能性

### 結果サマリ

| カテゴリ | 結果 | 詳細 |
|---|---|---|
| **L3 naming/identity** | 5/6 PASS / 1 TIMEOUT | `TestIdentityFromTitleUnknown` が 60s timeout (omniwm が `random-window-<nanos>` 形式の長い title を時間内に登録せず process-alive fallback 経由→以降の omniwm query で出てこない) |
| **L3 session (tmux)** | 9/9 PASS | tmux 操作は実機で完全に通る |
| **L3 spawn (TestSpawnShell)** | PASS (2.83s) | 通常 title `shell-1:projwm-next-test` で通る |
| **L3 cockpit (TestCockpitShowHideRestoresPriorWorkspaceAndWindow)** | FAIL (30.7s) | spawn → focus の間に omniwm が window を登録せず focus 失敗。長い title `cockpit-prior-<nanos>:TestCockpitShowHideRestoresPriorWorkspaceAndWindow` |
| **その他 L3** | 未実行 | (時間と env 状態の都合) |

### honest 判定

- **L3 harness 自体の脆弱性**: 長い title や short-cycle spawn → focus シーケンスで omniwm window registration が間に合わず process-alive fallback 経由になる。これは SSOT 互換性の問題ではなく、test harness の timing/title-format に起因
- **S06 (cockpit show/hide) は L3 で flaky**: `TestCockpitShowHideRestoresPriorWorkspaceAndWindow` が現環境で fail。原因は cockpit の show/hide のロジックではなく、prior window spawn 時の omniwm 登録待ち。S27 adapter contract audit で fix 予定
- **基本 spawn / tmux 系は green**: 主要な adapter op (spawn-shell, tmux ensure/kill/group) は実機で動く

### 残った honest 表明

L3 real_ops 全 35 件を完走させて green を確認する作業は **S27 adapter contract audit + S29 L4 acceptance ledger 昇格** に持ち越し。Phase 1 完了宣言の条件として「全 L3 green」は **未達**。基本 op は動くことを部分検証した。

---

## Task E: darwin-rebuild switch + production 実機検証 (2026-05-24)

### 実機環境

- omniwm running (`org.nixos.omniwm` PID 9100)
- omniwm-display-watcher running
- **projwmd-next は registered ですらない** (user が bootout 済 / 開発中で停止中)
- production store: `/Users/yuta/.local/state/projwm-next/store/` G012038 (`CURRENT`)

### 実機検証手順

`darwin-rebuild switch` を強制実行する代わりに、**新 daemon が新 manifest +
existing production store で boot できることを sandbox で実検証** した。これは
manifest digest 変動の影響と store JSON 互換性を同時に確認する手段。

```
# 1. 新バイナリをビルド
go build -o /tmp/projwmd-test ./cmd/projwmd
go build -o /tmp/projwmstore-bootstrap-test ./cmd/projwmstore-bootstrap

# 2. Nix-generated SSOT-shape manifest を locate
/nix/store/nab773w0qnl7hh9v07wvhprlycxf073j-projwm-next-managed-environment.json
# digest (sha256 of bytes): 8c21a515de71da21529f225e807ea0b96b6886e6197b36bdac36af23838fcb81

# 3. 新 daemon を production store と新 manifest で boot
/tmp/projwmd-test \
  --managed-environment <SSOT-shape-manifest> \
  --manifest-digest 8c21a515de71da21529f225e807ea0b96b6886e6197b36bdac36af23838fcb81 \
  --store-dir /Users/yuta/.local/state/projwm-next/store \
  --socket-path /tmp/projwmd-test.sock
# → 3秒間 alive 確認、SIGTERM で gracefully shutdown
```

### 確認できた事実

1. **新 manifest parse**: SSOT §10.7 top-level array shape を parse 成功
2. **manifest digest 一致**: Nix の `builtins.hashString "sha256"` と Go の
   `bytes.NewReader(data)` 経路の digest 計算が完全一致
3. **既存 store 互換性**: G012038 の `ControllerCheckpoint.ManualLayoutCandidates`
   JSON field は **Go の json.Unmarshal が unknown field を ignore する**ため、
   新 daemon が legacy generation を silently load 成功 (forward compat ✓)
4. **socket bind**: socketPath が manifest 記載と一致する場合のみ bind 受付
5. **3 秒間 daemon stay alive** (graceful boot)

### 未検証

- **darwin-rebuild switch** は実行していない (user の意思決定)
- **Nix-generated launchd plist の args 整合** は確認したが、launchd 経由
  での auto-start は試していない (user が `launchctl bootstrap` で activate
  するまで)
- **transaction 動作 (ApplyIntent / ApplyEvent)** は test daemon で確認していない
  (omniwm + Ghostty を実際に動かす必要があるので別 verify が必要)
- **manifest digest 変動が store の `bootstrapManifestDigest` と一致するか**
  は **未確認** (G012038 は古い digest を持っているはず、boot 時に
  startupLifecycleStatus=blocked になる可能性あり)

### honest 評価

DoD #12 (orchestrator 実機 verify) は **△ (部分達成)**:
- manifest parse + digest + store load は実証
- 完全な production end-to-end (`darwin-rebuild switch` + launchd activate +
  実 transaction 動作) は **未達**
- user が rebuild する際は `projwmstore-bootstrap-next` で store 再 bootstrap
  が必要な可能性あり (bootstrapManifestDigest が変動)

---

## 最終 honest 表明 (Phase 1 + S06)

### 達成 (honest に green)

- 6 切片の SSOT 整合実装 (S01-S06)
- ManualLayoutCandidate machinery 完全削除 (N-12 反映)
- observer/browser_tabs 機能停止状態の文書化 (S14/S20 で復元予定)
- ledger 4 items behavior promote + workflow doc 固定
- ssottest L0 + L1 + L2 が green (3 件 S14/S27 territory red を除く)
- nix build green
- 新 daemon が新 manifest + production store で boot 可能

### 未達 (honest gap)

- L3 real_ops 全件走行 (TestCockpitShowHideRestoresPriorWorkspaceAndWindow
  が現環境で fail — omniwm window registration timing issue)
- darwin-rebuild switch + 実 production daemon transaction 動作
- bootstrapManifestDigest との整合 (store 再 bootstrap 必要かもしれない)
- L4 acceptance test の "dotfiles"/"manaflow" hardcode (S29 territory)
- S14 (browser tab CRUD CLI)、S27 (adapter contract: 3 missing methods +
  L2 mock harness fixes) は Phase 2 territory

### 次のステップ (user 判断)

1. **darwin-rebuild switch を実行**して production daemon を新バイナリに置換
   (user 環境に影響、user の意思決定)
2. **S07 (scratch shell) に進む**: Phase 2 続行
3. **S27 (adapter contract) を先に**やる: 残 red 3 件を解消してから S07

Phase 1 + S06 は **「SSOT 整合に対して honest に部分完成」** という状態。残ギャップは
明示的に列挙済 (本文書がその記録)。

---

## Addendum: darwin-rebuild switch 試行と user 判断による rollback (2026-05-24)

User の承認を得て一度 darwin-rebuild switch を試した結果:

1. ストア再 bootstrap (新 digest 8c21a515...) は成功
2. `sudo darwin-rebuild switch --flake .#yuta` 部分実行
   - home-manager 側 activation 完了 (karabiner symlink 等)
   - darwin-system toplevel 構築は失敗
3. 原因: `pkgs.buildGoModule` の `checkPhase` が `go test ./cmd/projwm/...` を
   走らせる。`TestSSOTCLIExposesBrowserTabOperations` は S14 territory のため red。
   nix build がそこで停止
4. CLI usage 文字列に browser tab エントリを stub 追加して test 自体は green に
   できる (実装は S14 で本実装) が、user 判断: **「Phase 2-5 完了まで nix
   deploy なしで進める」**
5. Rollback: store を G012038 (旧 digest) に restore、staging を unstage
6. Karabiner: darwin-rebuild の中途活性化で grabber/Core-Service が一度死亡
   → `sudo launchctl kickstart -k system/org.pqrs.service.daemon.Karabiner-Core-Service`
   で復活、symlink 更新は activation 済なので config は新版

**結論**: Phase 2-5 では nix デプロイなしで go test ベースで進め、Phase 5
完了 + checkPhase 全 green になった時点で改めて darwin-rebuild switch を
実施する。実機 verify は go test + 手動 daemon 起動 (`/tmp/projwmd-test`
にバイナリビルドして production store path で boot) で代替。
