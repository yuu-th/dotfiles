# projwm-next SSOT 実装 切片計画 (Honest Assessment, 2026-05-23)

> 入力: `queue/ssot-coverage-matrix.md` (atomic 要求 375 件) +
> `queue/matrix-fragments/fill-sec{2-3-4,5-6,7-10}.md` の 3 agent 出力 +
> orchestrator (=私) によるスポットチェック補正。
>
> Agent 1 (§2-§4) は §4.1 新規追加 intent の 12 件で implementation status と
> test owner を systematic に捏造していたため、それらは **手動 MISSING に再分類**。
> Agent 2/3 はおおむね honest だが test owner 主張に部分捏造あり、信頼度
> 中程度。

---

## 1. Agent reliability summary

| Agent | 担当 | 信頼度 | 補正内容 |
|---|---|---|---|
| Agent 1 | §2 / §3 / §4 | 低 | §4.1 OP01-06, OP11, OP14-17 を MISSING に再分類。reducer.go / planner.go の `case intent.Summon*` 等が 1 つも無いことを確認済 |
| Agent 2 | §5 / §6 | 中-高 | §5.4 TUI 系 partial 判定は事実、§5.7 CLI implemented 判定は概ね事実 (TestWizard_* 等の test owner は実在確認済) |
| Agent 3 | §7 / §8 / §9 / §10 | 中 | implementation 主張は accurate (`flockExclusive` file.go:722 等)、test owner の一部 (`TestFileStoreConcurrentWrite` 等) は捏造 |

## 2. 補正後の SSOT カバレッジ集計

§4.1 OP01-06, OP11, OP14-17 を MISSING に再分類した上での集計:

| 節 | 要求数 | implemented | partial | missing | unknown |
|---|---|---|---|---|---|
| §2 (mental model) | 41 | 約 35 | 約 4 | 約 2 | 0 |
| §3 (system state) | 37 | 約 30 | 約 5 | 約 2 | 0 |
| §4.1 (17 user ops) | 19 | 5 (op7-10, 12-13 一部) | 2 | **12** | 0 |
| §4.2-4.6 | 66 | 約 55 | 約 8 | 約 3 | 0 |
| §5 (UI) | 72 | 27 | 44 | 1 | 0 |
| §6 (design principles) | 17 | 11 | 6 | 0 | 0 |
| §7 (architecture) | 71 | 67 | 0 | 4 | 0 |
| §8 (state mgmt) | 11 | 10 | 1 | 0 | 0 |
| §9 (acceptance) | 15 | 10 | 5 | 0 | 0 |
| §10.9 GAP | 26 | 0 | 17 | 9 | 0 |
| **合計** | **375** | **約 250** (67%) | **約 92** (24%) | **約 33** (9%) | 0 |

**重要な現実**:
- 「architecture 骨格 (§7.1-§7.4)」は **95%+ 整合** — 温存する足場が確実にある
- 「§4.1 ユーザー操作 17 件」は **5/17 のみ implemented** (op7-10, 部分的 12-13)、残り 12 件は intent kind だけあって reducer/planner 動線がない
- 「§5.4 cockpit TUI」は **27 implemented / 44 partial** — bubbletea 全体構造はあるが各 widget の behavior 検証が大半 partial
- 「§10.9 GAP」は **0 implemented / 17 partial / 9 missing** — テスト保証境界が完全に下りていない

---

## 3. 切片計画 (29 切片)

各切片は SSOT 要求 ID のクラスタ。1 切片で完結し、end-to-end (intent → reducer → planner → executor → adapter → CLI/TUI → L3 test owner + ledger evidence 昇格) を達成する。

### Phase 1: 足場確立 (5 切片)

| # | 切片名 | 主要 SSOT ID | 目的 |
|---|---|---|---|
| **S01** | manifest §10.7 shape 統一 | MANI-01 | 既存 red を解消、後続 lifecycle/provenance テストの前提整備 |
| **S02** | Phase 0.3 deprecated intent purge | (整理対象) ToggleCockpit/FocusCockpit/AcceptManualLayout/SyncBrowserTabs/RespawnOrphanGhostty | reducer/planner/TUI/scenarios の consumer を削除、intent.go から完全削除 |
| **S03** | AI 名 routing 修正 | SSOT-4.4-AI-CMD-FIRST, SSOT-4.4-AI-MULTI | DesiredWindow に AIName field、semop の hardcoded "claude" 解消、AddWindow.AIName routing |
| **S04** | ssottest ledger 真化 (evidence Meta → Behavior 昇格手順) | SSOT-10.9-GAP25 | 「テスト関数存在」check を「実 behavior 検証」に変える手順とユーティリティを確立 |
| **S05** | testdata fixture と test 環境分離の audit | SSOT-10.9-GAP26 | store/socket/log/manifest/tmux/title/workspace の本番分離検証 |

### Phase 2: §4.1 17 user operations end-to-end 実装 (10 切片)

| # | 切片名 | 主要 SSOT ID | DoD |
|---|---|---|---|
| **S06** | OP07 cockpit show/hide 完成 | SSOT-4.1-OP07-SHOW, OP07-HIDE, SSOT-3.4-INV06 | PriorWindow restore 含めて end-to-end、L3 real_ops owner、ledger 昇格 |
| **S07** | OP11 scratch shell 実装 | SSOT-4.1-OP11-SHOW/HIDE, SSOT-7.5-WM-SHOWSCRATCH/HIDESCRATCH, SSOT-7.3-SCRATCH-* | intent → reducer (scratch SystemWindow) → planner (show/hide op) → adapter (ShowScratchShell/HideScratchShell)、L3 U1 owner 昇格 |
| **S08** | OP06 summon-viewer 実装 | SSOT-4.1-OP06, SSOT-3.4-INV05/INV12 | viewer 復帰、viewer order 維持、L3 test owner |
| **S09** | OP01-03 summon-shell/editor/browser 実装 | SSOT-4.1-OP01, OP02, OP03, SSOT-2.3-FL* | identity-based summon + cycle 状態管理、controller に last-focused window tracker、L3 test owner |
| **S10** | OP04 switch-project 実装 | SSOT-4.1-OP04 | last-focused window restore、L3 test owner |
| **S11** | OP05 cycle-slot-window 実装 | SSOT-4.1-OP05 | 現 slot 内 kind cycle、WS 変えない、L3 test owner |
| **S12** | OP12 add-window 完成 (AI 含む) | SSOT-4.1-OP12, SSOT-4.4-AI-MULTI, SSOT-4.4-AI-CMD-* | S03 の AIName routing 完了前提、id auto-increment、L3 test owner |
| **S13** | OP13 remove-window 完成 | SSOT-4.1-OP13, --purge-if-empty | window kill-session + close、最後の window 削除時の挙動、L3 test owner |
| **S14** | OP14-17 browser tab CRUD (CLI + PrivatePayloadStore wiring) | SSOT-4.1-OP14/15/16/17, SSOT-5.7-BR-*, SSOT-4.4-BR-TAB-* | CLI surface 追加、PrivatePayloadStore write/read、browser tab observer 含む |
| **S15** | OP08-10 既存操作の SSOT 整合 audit | SSOT-4.1-OP08, OP09, OP10, SSOT-4.5-PROF-SWITCH/ARCHIVE/UNARCHIVE | phase order (close → barrier → spawn) 検証、§4.5 複合操作と SSOT 突き合わせ |

### Phase 3: 不変条件 + 障害復帰 + 観測 (5 切片)

| # | 切片名 | 主要 SSOT ID | DoD |
|---|---|---|---|
| **S16** | INV-01 duplicate window 最 recently-focused 選択 | SSOT-2.5-EC4, SSOT-3.4-INV01, SSOT-10.9-GAP01 | identity.Resolve 拡張、orphan + [INVARIANT] card 経路、L3 test owner |
| **S17** | INV-04, INV-05, INV-12 残不変条件の behavior 検証 | SSOT-3.4-INV04/05/12 | archived 残留 close、viewer 整合、viewer order = slot order、L3 test owner |
| **S18** | drift 検出 + grace period + orphan card 3 action | SSOT-4.3-* 全件 (CROSSWS/SAMEWS/DRIFT/MISSING/ORPHAN/STALE/GRACE/ORPHAN-ENTER/C/T) | observer sidecar 実体化、cockpit カード接続、L3 test owner |
| **S19** | 障害復帰 lifecycle | SSOT-3.5-* 全件 (MACOS/OMNIWM/TMUX/GHOSTTY/ZED/VIVALDI/COCKPIT/DISPLAY/BOOT-A/B/C/D) | LifecycleBootstrap/Wake/DisplayReconfigure の動線、Zed empty project 自動 close、cockpit 復帰 health probe |
| **S20** | observer sidecar 実体化 (windows-changed / display-changed / wake / safety-timer) | SSOT-4.4-BR-TAB-OBS, SSOT-10.9-GAP09 | projwmevent CLI を呼ぶ実 daemon、event → reducer.ReactToEvent → DirtyScope の動線 |

### Phase 4: §5 UI 完成 (5 切片)

| # | 切片名 | 主要 SSOT ID | DoD |
|---|---|---|---|
| **S21** | §5.4 cockpit TUI topbar + 5 tabs SSOT snapshot test | SSOT-5.4-TOPBAR, TAB-SLOTS/CARDS/ARCHIVED/PROFILES/TRACE, SSOT-10.9-GAP11 | 各 tab の content render が SSOT §5.4 wording と一致する snapshot test |
| **S22** | §5.4 カード system 6 種 (NEW/CLOSED/MOVED/INVARIANT/MANIFEST/OMNIWM-RECOVERY) | SSOT-5.4-CARD-* 全件 | 各 card type の発火条件 + 表示 + dismiss、behavior test |
| **S23** | §5.4 Wizard + Palette + Mode (Proposal/Navigation/Management) | SSOT-5.4-WIZARD, PALETTE, MODE-PROP/NAV/MGMT, SSOT-10.9-GAP12 | wizard submit → intent 発行、palette fuzzy backend、mode transition、L3 owner |
| **S24** | §5.6 status / doctor 完全出力 | SSOT-5.6-* 全件 (STATUS x 9 + DOC x 7) + SSOT-10.9-GAP13 | 各項目の presence 検証 + PASS/WARN/FAIL classification |
| **S25** | §5.5 エラー通知 + macOS notification 不使用検証 | SSOT-5.5-* 全件 | cockpit card 経路、topbar convergence、doctor LEVEL、macOS notification 全使用箇所 grep negative test |

### Phase 5: §7 / §8 / §9 / §10 横串完成 (4 切片)

| # | 切片名 | 主要 SSOT ID | DoD |
|---|---|---|---|
| **S26** | §7.1 max replans 超過時の 4 挙動 (fail/rollback/card/dirty scope) | SSOT-7.1-MAXREPLAN-* 全件, SSOT-10.9-GAP19 | 統合 behavior test |
| **S27** | §7.5 adapter contract 漏れ method の実装 (MoveCockpitToParkWorkspace 等) | SSOT-7.5-WM-MOVECP, SSOT-10.9-GAP20 | adapter method + L2/L3 owner |
| **S28** | §8 PersistentStore 完全性 + 排他制御 | SSOT-8.1-* 全件 + SSOT-8.3-* 全件 + SSOT-10.9-GAP22/23 | crash-safe generation, concurrent writer, interrupted write の behavior test |
| **S29** | §9.2 時間要件 + §9.1 S1-S10 L4 受入完成 | SSOT-9.1-* 全件 + SSOT-9.2-DOD1/2/3/4/5 + SSOT-10.9-GAP24 | 1 分以内復帰 + 5 秒以内 profile switch の実 E2E timing assert、`ssotAcceptanceImplemented` 空 map 埋め |

---

## 4. 各切片の標準 DoD (Definition of Done)

旧 9 phase 計画より厳密化。各切片が完了するには以下を全部満たす:

1. **SSOT 該当節再読**: 切片に紐づく要求 ID を 1 つずつ確認
2. **対応 intent 実装** (該当する場合): reducer handler + planner branch
3. **adapter / executor 実装** (該当する場合): SSOT §7.5 契約に整合
4. **CLI / TUI 接続** (該当する場合): §5.7 / §5.4
5. **L0 unit test**: 純粋関数の入出力
6. **L1 fake test**: state 遷移、不変条件
7. **L2 mock harness** (該当する場合): retry/timeout/fallback
8. **L3 real_ops test owner**: behavior 検証として書く
9. **ssottest ledger 昇格**: `evidenceMeta` → `evidenceBehavior` に切り替え、対応する SSOT 要求 ID をマトリクスで「implemented」に更新
10. **§6 設計原則チェック**: §6.1-§6.10 のうち該当する原則 (例 §6.6 idempotency, §6.10 操作順序) を切片の test owner で明示検証
11. **該当 INV (§3.4) チェック**: 切片で関連する不変条件を invariant checker で behavior 検証
12. **orchestrator (=私) 実機 verify**: `darwin-rebuild switch` + 実機で SSOT wording どおりの挙動を私自身が観察
13. **user 報告**: 「この切片で何が出来るようになった」を 2-3 行、+ 既知の残ギャップを honest に列挙

---

## 5. 切片の依存関係

```
Phase 1 (S01-05) … 足場
  │
  ├──→ Phase 2 (S06-15) … §4.1 17 user operations
  │     │
  │     └──→ Phase 4 (S21-25) … §5 UI 完成
  │
  ├──→ Phase 3 (S16-20) … 不変条件 + 障害復帰 + 観測
  │     │
  │     └──→ Phase 5 (S26-29) … 横串完成
  │
  └──→ Phase 5 直接依存もあり (S28 store completeness は S01 manifest と独立)
```

並行可能な切片:
- S06 (cockpit) と S07 (scratch) は独立 → 並行可
- S08-11 (summon-* / switch / cycle) は共通の identity-based jump 動線を必要 → S08 で動線確立後 S09-11 を並行
- S16 (INV-01) と S17 (残 INV) は独立 → 並行可
- Phase 4 (S21-25) は Phase 2 完了後ならどこからでも並行可

---

## 6. 期待スケジュール感

切片あたりの推定:
- 小 (S01, S05, S20): 1 切片 = 30 分〜 2 時間 (audit 中心)
- 中 (S06, S07, S08-11): 1 切片 = 2-4 時間 (実装あり)
- 大 (S14 browser tab CRUD, S19 lifecycle, S22 cards, S29 L4): 1 切片 = 4-8 時間

29 切片合計の作業時間オーダー: 80-150 時間 (= 数週間〜1 ヶ月の対人工作)。一日 4-8 時間進める想定で 2-4 週間。

実機 verify のために `darwin-rebuild switch` が必要なので、user 環境を頻繁に rebuild する点に注意。

---

## 7. このまま実装に入る前の最終確認事項

ユーザに確認してほしい点:

1. **29 切片の順序と粒度**: もっと細かく / 大きく? 並行する切片の組み合わせ?
2. **Phase 1 で S02 (deprecated intent purge) を先に潰すか、S06/S07 と並行か**: 私の推奨は S02 を S03 と並列に消化してから Phase 2
3. **§10.9 GAP の test owner 追加方針**: 各切片の DoD に組み込まれているので別途切片不要、と私は判断したが、user に確認
4. **私の実機 verify 後の user 報告頻度**: 各切片完了時 / Phase 完了時 / 全体完成時、どこで報告?
5. **L4 受入 (S29) を最後にやる順序で OK か**: Phase 2/3/4 が完成しないと L4 は通らないので最後にせざるを得ない、と私は判断

---

## 8. 私 (orchestrator) からの honest 警告

- **Agent 1 が hallucinate した経緯から、私自身が agent 出力を spot-check しないと「green と書いてあるが実は missing」が混入する** ことが実証された。各切片の DoD #12「orchestrator 実機 verify」は省略不可。
- **2 週間〜1 ヶ月の長期作業**になる。context compaction を跨ぐので、各切片完了時に `orchestration-state.md` と `ssot-coverage-matrix.md` を更新して継続性確保が必須
- **既存実装の 70-95% は実は SSOT 整合に近い**が、ledger evidence が Meta のままなので「動いてるか保証できない」状態。S04 の ledger 真化が後続全部の前提条件
- **Phase 0.1/0.2 の私の作業** (intent kind 追加 + title 修正 + PriorWindow populate) は **型レベルでは正しいが behavior レベルでは未完**。S06-S15 の Phase 2 切片群がその仕上げ
