# projwm-next + omniwm Overhaul — Orchestration State

> このドキュメントは context compaction を跨いで作業の同一性を保つための
> snapshot。AI orchestrator (Claude) が再起動/compaction 後にこれを読み、
> 自分が誰で何をやっていたかを即座に取り戻すために存在する。
>
> Last updated: 2026-05-25 (SSOT v1.11 29 切片計画 Phase 2 完了 + Phase 3 部分完了 — 14/29 slices)

---

## 0ter. 2026-05-25 セッション handoff (議論可能な状態に整理)

### このドキュメントの目的

次の担当者 (= 別セッションの自分、または別人) が、本セッションで進んだ作業を「指示通り続行する」のではなく「**なぜそうしたかを理解し、必要なら議論・修正できる**」状態で引き継げるようにする。事実列挙より、user が大事にした視点 + 設計判断の根拠 + 残された議論余地を優先する。

---

### A. user がこのセッションで明示・強調した視点 (内面化すべき方針)

これらは「言われたら従う」ではなく、**判断の前提として常に背景にある**もの。

#### A1. 「ただやることを受動的に理解するのではなく動的に全体像を理解」(2026-05-25 セッション冒頭)

context compaction 後、私が引継ぎ文書を読んで「Phase 2 進めます」と着手しようとしたとき、user は明示的に止めた。**まず SSOT 主要文書を全部読み、用語と切片の意味を内面化、わからないことは確認質問してから着手すること**。

実例: この介入によって、commit 着手前に red test 2 件を spot-check し、それが Phase 0.2 取りこぼし (cockpit title) と F5 contract 未実装 (FocusWindow navigate→focus) であることを発見。S27 が「3 missing methods だけ」ではなく「5 work items」になった。**この user 介入無しなら、broken adapter contract のまま S07 に着手していた**。

→ 教訓: 引継ぎ後の最初の着手は急がない。状況把握 + 質問が高 leverage。

#### A2. 「コレで進めれるよね？ちゃんと作業」 (2026-05-25 commit 直前)

私が「S27 territory の red 2 件を後で見ます」と言って commit に進もうとしたとき、user が止めた。**red の正体を理解せずにコミットすべきではない**。

→ 教訓: 未解決の red を残したまま phase 移行しない。「後で」は遅延の言い訳になりやすい。

#### A3. 「自律的に進めて良い、ただし honest 振り返りを各段階で」(2026-05-25 中盤)

私が S07-S19 を順に進めた背景。user は autonomy を granted したが、condition は「正直に振り返って問題なく、対応が不要だと判断したら」。

これは **partial 成功を成功と報告するの嫌う / ギャップを honest 列挙** という長期 preference の現れ ([[feedback-orchestrator-responsibility]] と整合)。

実例: S14 で PrivatePayloadStore proper wiring を「第一段階」として明示的に deferral、TODO コメント・commit message・ledger・matrix の 4 箇所に gap を honest 明記して S20 territory として記録。**「動いてるように見えるが SSOT 違反」を隠さない**。

→ 教訓: 各 slice 完了時に「何が動く / 何が動かない / なぜそうした」を 3 箇所以上に書き残す。

#### A4. 「次の担当者が理解できて議論できるレベルで」(本セクション執筆の trigger)

本ドキュメント自体の trigger。事実列挙ではなく、**why と alternatives を一緒に書く**。次の担当者は plan に従う drone ではなく、judgment を持って push back できるべき。

→ 教訓: status report と context document は別物。引き継ぎは後者。

---

### B. 完成 slice (14 件、commit と「何ができた / なぜ」)

各 slice は SSOT slice plan §4 の DoD 13 条項に向けて作業。Layer は SSOT §10.6 row を厳格に守る (要件範囲を逸脱しない、ledger.Layer 欄)。

| Phase | Slice | Commit | 何ができたか (= 動作する SSOT 整合機能) | 重要な判断 |
|---|---|---|---|---|
| 0 | import | `f8ac9da` `c4b2d16` | projwm-next 初回 git 取り込み (178 files) + legacy patchwork 削除 | 旧 projwm v2.x を残さず削除。「Deprecated 注釈で残置」は patchwork に逆戻りとの判断 |
| 5 先取 | **S27** | `d270f6d` | adapter §7.5 contract 完成 (5 work items 内訳は B.1 参照) | red 2 件を「後で」にしない user 介入で territory 拡張 |
| 2 | **S07** | `76b1558` | scratch shell の intent → reducer → planner → executor → adapter 全層配線 | reducer で SystemWindow.Visibility を flip するモデル (cockpit と同じ pattern を踏襲) |
| 2 | **S08** | `fe54231` | summon-viewer: 直前 focus AI に対応する viewer を resolve して focus | **「planner branch on commandKey」 pattern を確立**。reducer no-op、planner が observed を読んで target を resolve (B.4 参照) |
| 2 | **S09** | `54b47d6` | summon-shell / editor / browser × index cycle | **commandKey suffix encoding** ("intent:summon-shell:Q") で payload を planner に伝達。state を持たない |
| 2 | **S10** | `b5b5cda` | switch-project: target slot の workspace に focus 切替 | **omniwm の per-workspace MRU に focus 復帰を完全委譲**。projwm-next は「ws 切替命令を出す」だけ (B.5 参照) |
| 2 | **S11** | `3d78e42` | cycle-slot-window: 同 slot 内 kind 切替 (current_ws 不変) | SSOT 「current_ws 変わらない」契約のため **focus-workspace op を禁止**、focus-window のみ emit |
| 2 | **S12** | `b79eb0d` | add-window 完成 + AI 名 routing 実機 verify | Phase 1 S03 で deferral された L3 real_ops を実機 omniwm で走らせて `tmux capture-pane` で send-keys 結果直接確認 (Observe race 回避) |
| 2 | **S13** | `7b3b7bb` | remove-window + planner バグ修正 | **planner.lifecycleRemovalAllowed の bug 発見・修正**: 削除済 DesiredWindow への close が unconditionally false 返してた (B.7 参照) |
| 2 | **S14** | `07203ca` | browser tab CRUD 第一段階 (reducer + IPC + CLI) | **PrivatePayloadStore proper wiring を S20 deferral と honest 明記**。SSOT 違反を隠さない選択 (B.6 参照) |
| 2 | **S15** | `2842327` | OP08-10 audit (switch-profile / create-project / archive/unarchive) | 新規 code なし、既存実装の SSOT 整合確認 + ledger 整理。「audit 切片」と「実装切片」の区別を意図的に作る |
| 3 | **S16** | `d99ab21` | INV-01 duplicate window resolver (focus tiebreak + [INVARIANT] card 経路) | identity package を「分類のみ pure」のまま保ち、tiebreak は別 helper として export。SSOT INV-01 が「auto-close せず card 通知」と言う意図を尊重 (B.3 参照) |
| 3 | **S17** | `b7e5e94` | INV-06 cockpit park-workspace checker | 既存 planner が move op emit 経路を持つことを前提に、invariant 側の Check15 を追加。planner と invariant の責任分離 |
| 3 | **S18** | (matrix) | §4.3 11 項目を audit → matrix fill | 「実装はあるが ledger に書かれてなかった」を honest に整理。observer sidecar 部分は S20 territory と明記 |
| 3 | **S19** | (matrix) | §3.5 12 項目を audit → matrix fill | macOS/OmniWM/tmux/Ghostty/Zed/Vivaldi/cockpit/display + BOOT-A/B/C/D の復旧経路の現状を整理。timing 計測は S29 deferral |

#### B.1 S27 の honest scope (5 items、user 介入で拡張)

当初引継ぎ書は「3 missing methods (ShowScratchShell / HideScratchShell / MoveCockpitToParkWorkspace)」と書いていた。私が red 2 件を spot-check した結果:

1. **Phase 0.2 取りこぼし**: cockpit title が `projwm-cockpit-D0` から `projwm-cockpit-0` に SSOT §7.3 で変更されたが、sigwm.go の 11 箇所 + controller.go:782 + simulator.go:28 で stale だった (S27a)
2. **F5 contract 未実装**: FocusWindow が `window focus <id>` だけだった。SSOT §7.5 F5「navigate → focus の command-order contract」を満たしてない (S27b)
3-5. ShowScratchShell / HideScratchShell / MoveCockpitToParkWorkspace の interface 追加 + sigwm/fake 実装

**議論余地**: F5 で navigate を best-effort (error swallow) としたが、これは妥当か? sigwm.go FocusWindow 内のコメントで判断根拠を書いた (navigate は workspace hint、focus が authoritative、SSOT 1431「実 focus 結果は L3」)。別の解釈: navigate 失敗を伝播すべきという立場もあり得る。次の担当者が必要なら見直す。

#### B.2 「planner branch on commandKey」 pattern (S08 で確立、S09/S10/S11 で再利用)

**問題**: SummonViewer / SummonShell / SwitchProject / CycleSlotWindow は「ある window に focus する」transient な操作。DesiredWorld の永続 state には収まらない (Cockpit.Visibility のような persistent な desired state ではない)。

**検討した選択肢**:
- A) DesiredWorld に PendingFocus *DesiredWindowID を追加 → 永続 state を持つ → DesiredWorld の semantics と合わない
- B) ControllerMeta.PendingFocus を transient field として追加 → 1 transaction 内だけ生きる → state を controller layer に置く必要、reducer 純粋性損なう
- C) Plan() に intent 引数を追加 → invasive、planner が intent package に依存
- D) **commandKey suffix encoding**: commandKey ("intent:summon-shell:Q") に slot/kind を suffix で encode、planner が parse → controller-pure 維持、planner が observed を読んで target を計算

選択 D を採用。**reducer は no-op (no DesiredWorld change)、planner が observed.Focus を見て target を resolve + focus ops を emit**。S08-S11 全てに同 pattern。

**議論余地**: commandKey は本来 FocusPolicy.FinalFocus のキーとして使う opaque key。suffix encoding でデータを混ぜるのは設計純度を下げる。alternative としては DesiredWorld に「ephemeral」フィールド枠を作る (例: `EphemeralActions []EphemeralAction`) のもあり。今のところ commandKey 方式で破綻はないが、将来 S22-S23 (Wizard) が複雑な ephemeral state を必要としたら見直し対象。

#### B.3 INV-01 design (S16)

**SSOT §2.5 EC4 / §3.4 INV-01**: 同 (project, kind, id) の window が複数 observed されたら、「最 recently focused を正とし、他は orphan、cockpit に [INVARIANT] カード通知」。

**SSOT が言わないこと**: 「auto-close すべきか」「focus 履歴をどう持つか」。私の解釈:
- 「最 recently focused」= 「現在 focused」(履歴を持たず、observed.Focus.Window で代用)
- 「正とする」= 「identity resolver の Live を winner にする」(auto-close は SSOT 文言にない、card 通知が user に判断を委ねる責任分離)
- 「orphan」= 「ResolveResult の Candidates に残るが Live ではない側」

**実装**:
1. `identity.IdentifyWinnerAndOrphans(candidates, focused) → (winner, orphans)` を helper として追加 (identity.Resolve 自体は ambiguous 判定のまま、purity 維持)
2. `identity.ResolveWithFocusTiebreak()` で 1 を経由して Ambiguous → UniqueStrong 変換
3. planner の 3 callsite を ResolveWithFocusTiebreak に差し替え → 自動収束継続
4. `invariant.Check15` ではなく `Check14DuplicateWindow` を separately 追加 (existing card 経路で [INVARIANT] emit)

**選択肢**: 全部 identity.Resolve 内で tiebreak してしまう案 (シンプル) もあったが、identity の責任分離 (= 分類のみ) を保ったほうが他用途 (テスト・dry-run analysis) で「ambiguous は ambiguous のまま見たい」ケースに対応できる。

**議論余地**: 「focus 履歴を持つ」(ControllerMeta.FocusHistory) という案は実装してない。理由は SSOT が要求してない + 状態増加コスト。ただし将来「focus が候補外」のとき deterministic に smallest LiveWindowID 選んでる挙動は、user の使用パターンによっては不自然かも。次の担当者が user 不満を聞いたら見直す。

#### B.4 S13 で発見した planner.lifecycleRemovalAllowed バグ

**症状**: RemoveWindow 後、planner が removed DesiredWindow に対応する live window の close op を **emit しなかった**。SSOT §4.1 OP13「tmux session が kill、ウィンドウが close」を満たさない。

**根本原因**: `lifecycleRemovalAllowed(env, target, id, observed)` の冒頭で `desiredWindowByID(target, id)` を required check していた。RemoveWindow 直後は target からエントリが消えているので false 返す → close op 出ない。

**修正**: desired lookup 失敗時は observed.App.BundleID / observed.Kind / observed.Title.Value を fallback として使う。AXCloseGuarded の title contract check は observed.Title.Value 非空チェックで safety 担保 (controller 命名規約 prefix が code-of-conduct)。

**議論余地**: この fallback は「observed が controller 命名規約に従う」前提に立っている。もし observed window の title が壊れている (外部 process が同 title 偽装) 場合の risk は理屈上ある。実害は無さそうだが、より堅牢化したいなら「removed DesiredWindow を一時 marker として保持」する設計に移れる (= "pending-cleanup" state)。S20 で観測 sidecar が入るとき同時に検討余地。

#### B.5 S10 switch-project が omniwm MRU に委譲する判断

SSOT §4.1 OP04 = 「直前にフォーカスしていた `<target project>` のウィンドウに focus 復帰」。これを実装する 2 つの選択肢:

- A) projwm-next 自身で per-project focus 履歴を持つ → ControllerMeta に `LastFocusedPerProject map[ProjectID]DesiredWindowID` 追加 → DirtyScope 連携・persist 等の複雑性
- B) **omniwm の per-workspace MRU に完全委譲** → projwm-next は「workspace を切り替える」だけ、omniwm が「その workspace で最後 focused だった window」を自動復帰

選択 B。omniwm の MRU はすでに動いている (実機 verify 済) ので、projwm-next の責任を最小化、「state ownership は WM の責務」という設計原則 (§6 系) と整合。

**議論余地**: もし omniwm が将来 MRU を廃止 / バグ化したら projwm-next 側で fallback 持つ必要が出る。今のところ omniwm 依存で OK と判断したが、SSOT 487「直前にフォーカスしていた manaflow のウィンドウ」を厳密に projwm-next が保証する立場もあり得る。

#### B.6 S14 PrivatePayloadStore deferral の判断

**SSOT §4.1 OP14-17 + 650**: 「URL 本文は PrivatePayloadStore に保存し、DesiredWorld には opaque ref と URLCount だけを残す」。

**現実装 (第一段階)**: URLPayloadRefs に URL を **literal で格納** (型は PrivatePayloadRef だが中身は raw URL)。これは SSOT 650 違反。

**なぜ第一段階で deferral したか**:
1. proper 実装は **Controller に PrivatePayloadStore field を thread**し、ApplyIntent の冒頭で BrowserAddTab/RemoveTab/ChangeTabURL/ReorderTabs を special-case 処理 (Put + Forget + ref 注入後 reducer 呼び出し) する必要がある
2. これは **S20 (observer sidecar 実体化) で同じ Controller-level wiring が必要**なので、S14 と S20 で重複作業になる
3. かつ、S20 の browser tab observer は user の Vivaldi 内手動操作も BrowserAddTab/etc intent に変換する責任を持つ → 一緒に設計したほうが整合性高い

**選択肢**:
- A) S14 で proper 実装 → S20 で重複したり再修正したりする可能性
- B) **S14 で第一段階、S20 で proper** ← 選択

**議論余地**: 「SSOT 違反を一時的にでも導入するな」という強い立場もあり得る。私の判断は「動く状態 (=テスト green、CLI 動作) を維持しつつ、deferral を 4 箇所 (reducer コメント / ledger / matrix / commit message) に honest 明記すれば SSOT-honest」。S20 で必ず解消する。

#### B.7 「audit slice」 vs 「実装 slice」 の区別

S15 / S18 / S19 は **新規 code なし**で audit のみ実施した。slice plan が「audit」を slice として明示してた (S15) ものは自然だが、S18 / S19 は「impl が必要そう」に見える名前だった。

判断: **既存実装が SSOT 整合だが ledger / matrix に書かれてなかった**ことを発見したら、新規 code を書かず「audit 完了」commit (matrix のみ更新) を出す。これは「コードはあるが文書化されてない」を負債として残さない設計。

**議論余地**: audit slice を「Phase 3 完成」にカウントするのは progress として水増し感がある。Honest にカウントすると Phase 3 で実装が必要な slice は S16/S17/S20 の 3 件で、本セッションでは S16/S17 のみ完了。次の担当者は「14 slice」を「14 件の実装」と誤読しないこと。matrix coverage 77% も「audit 込み」の数字。

---

### C. 完成したのではなく「動くようになった」だけのもの (= 注意して引き継ぐべき非完全部分)

#### C.1 「partial」と marker されている ledger / matrix エントリ

- **OP-14/15/16/17**: status `statusRealOnly` + Evidence `behavior` だが、matrix 側に `partial: PrivatePayloadStore proper wiring は S20` と明記。**動くが SSOT 完全準拠ではない**。
- **§3.5 timing 系**: 「1 分以内 / 5 秒以内」の数値要件は behavior 検証されてない。S29 で実 E2E timing assert。
- **§4.3 ORPHAN-ENTER/C/T**: card action は emit してるが cockpit TUI 側の Enter/c/t 押下 → intent 提出の配線は S22-S23 territory。**「user が押せば動く」になってない**。

#### C.2 L3 omniwm registration race

`TestScratchShellShowHideRestoresPriorFocus` が現環境で fail する (long-title が omniwm registration timing と race する). S27 で `TestScratchShellShowReturnsLiveWindowID` という short-title spot-check を別途追加 (omniwm Observe を skip、tmux + ghostty 直接検証) で side-step。

**根本対応**: sigwm.go の SettleTimeout / process-alive fallback 政策の調整。S20 で observer sidecar 入るとき omniwm 観測体系も見直すので、その時に合わせて修正予定。**次の担当者は S20 着手時にこの race を頭に入れる**。

#### C.3 nix build と darwin-rebuild

- nix build (`pkgs.buildGoModule` の checkPhase) は S14 第一段階で CLI handler 本実装になって fail 脱出済。
- ただし darwin-rebuild switch (production deploy) は **Phase 5 完了 + checkPhase 全 green 後の user 承認下で実施**という handoff doc 方針を継承。**現在 production daemon は bootout 状態**、store は G012038 (旧 digest) に rollback 済。私の修正は実 production には反映されてない。

**議論余地**: 「Phase 5 まで deploy しない」は handoff 文書から継承した方針だが、本セッションで Phase 2-3 を進めた今、`/tmp/projwmd-test` ベースで部分 deploy する選択肢もある。次の担当者は user の運用感想を聞いて決める。

---

### D. SSOT カバレッジ (Phase 1 末 67% → 本セッション末 77%)

| 節 | 要求数 | 完了 | 部分 | 残 | 主な変化要因 |
|---|---|---|---|---|---|
| §2 mental model | 41 | 約 38 | 約 2 | 0 | INV-01 (S16) + 既存 |
| §3 system state | 37 | 約 33 | 約 3 | 0 | INV-01/06 完成 (S16/S17) |
| **§4.1 17 user ops** | **19** | **17** | **2** | **0** | **S06-S15 完成 (本セッション最大の進捗)** |
| §4.2-4.6 | 66 | 約 58 | 約 6 | 約 2 | §4.5 archive/unarchive 既存実装の audit 完了 (S15) |
| §5 UI | 72 | 約 30 | 約 42 | 0 | Phase 4 territory、変化なし |
| §6 設計原則 | 17 | 約 12 | 約 5 | 0 | 個別 slice の test owner 設計原則 check 効果 |
| **§7 architecture** | **71** | **約 71** | **0** | **0** | **S27 で adapter contract 全完成** |
| §8 state mgmt | 11 | 約 10 | 1 | 0 | 既存 |
| §9 acceptance | 15 | 約 10 | 5 | 0 | L4 hardcode は S29 で |
| §10.9 GAP | 26 | 約 10 | 約 11 | 約 5 | GAP-20 (MoveCP) S27 で解消 |
| **合計** | **375** | **約 289 (77%)** | **約 77 (21%)** | **約 9 (2%)** | **+10pt 進捗** |

「完了」は ledger で `statusCovered` or `statusRealOnly` + Evidence `behavior` の項目。matrix 「partial」は本物の機能不足、または S20/S22/S23/S29 territory の deferral。

---

### E. 残作業 9 slice + それぞれの「議論余地」

| Slice | 規模 | やること | 設計判断が要求される論点 |
|---|---|---|---|
| **S20** | 大 | observer sidecar 実体化 + S14 PrivatePayloadStore proper wiring + browser tab observer | (1) projwmevent CLI を実 daemon 化するか sidecar process 化か (2) windows-changed event 検出を omniwm IPC poll vs 通知購読で (3) Controller への PrivatePayloadStore thread の wiring 形式 |
| S21 | 中 | §5.4 cockpit TUI 5 tabs snapshot test | snapshot diff の差分許容度 (動的時刻含む) |
| S22 | 中-大 | §5.4 card 6 種 (NEW/CLOSED/MOVED/INVARIANT/MANIFEST/OMNIWM-RECOVERY) | 既存 PromoteOrphans (NEW) と他種類 card の発火経路統合 |
| S23 | 大 | Wizard + Palette + Mode (Proposal/Navigation/Management) | mode transition の state machine 設計、palette fuzzy backend |
| S24 | 中 | §5.6 status / doctor 完全出力 | STATUS 9 + DOC 7 項目をどの level (env/daemon/store/transaction) から集める |
| S25 | 小-中 | §5.5 エラー通知 + macOS notification 不使用検証 | grep negative test 範囲 (CLI binary も含めるか) |
| S26 | 中 | §7.1 max replans 4 挙動 (fail/rollback/card/dirty scope) | rollback の database レベル整合 (generation revert?) |
| S28 | 大 | §8 store crash-safe + concurrent writer + interrupted write | crash 再現テストの作り方 (process kill simulate) |
| S29 | 大 | §9.1/9.2 L4 acceptance + timing + `dotfiles`/`manaflow` hardcode 解消 | test isolation 戦略 (専用 test daemon プロセス? tmpdir?) |

**注: S20 は Phase 3 完成のために必須**。S21-S25 は Phase 4、S26/S28/S29 は Phase 5。

---

### F. 次セッション着手の選択肢 (議論材料付き)

#### F1. 推奨: S20 (observer sidecar 実体化)

**Pros**:
- Phase 3 を完成させると plan の semantic な境界が綺麗
- S14 deferral の PrivatePayloadStore proper wiring がここで決着
- browser tab observer が入ると user の Vivaldi 内手動操作も daemon に届く → 本格的なユーザビリティ
- L3 omniwm registration race の根本対応 chance

**Cons**:
- 規模大 (おそらく 1 セッションで完了しない、複数の commit)
- Controller の thread 構造に手を入れる → 既存 transaction loop と並行する sidecar の lock 設計が必要

**議論余地**: Phase 3 完成を急ぐ価値 vs Phase 4 軽め (S24/S25) で UI に手を出して user に「動く感」を見せる価値、user の優先順位次第。

#### F2. S29 (L4 acceptance + hardcode 解消) を先に

**Pros**:
- ISO-01 ledger の statusRed (allowlist 違反 2 件) を解消できる → ledger 100% honest 化
- L4 acceptance test が走るようになれば「実機検証」の自動化が前進
- timing assert を追加すれば SSOT §9.2 数値要件 (1 分 / 5 秒) も埋まる

**Cons**:
- hardcode 修正自体は機械的だが、置換後の test 設計を考えると S20 以後の方が観測経路が整って楽
- timing は実機 omniwm 必須で flaky になりがち

#### F3. Phase 4 軽め (S24 status/doctor, S25 macOS notif) から

**Pros**:
- UI 領域に踏み込める (user が見るレベル)
- S24/S25 は規模小-中、1 セッションで終わる可能性
- S20 を skip すると Phase 3 が「未完」のまま Phase 4 に入る違和感

**Cons**:
- Phase 3 残置のまま進むと「§4.3 観測経路」が緩いまま UI が乗る → bug の温床
- 順序として slice plan が S20 → S22 を想定してる (S22 の card system は observer から trigger されるものを含む)

#### F4. 一旦休止 + review

**Pros**:
- 14 slice 完成済、これだけで Phase 2 完了 + Phase 3 中盤達成 → 一定の達成感
- user が本ドキュメントを読んでから方針を決められる

**Cons**:
- 勢いが落ちると context compaction を跨ぐ無駄が出る (引き継ぎコスト)

---

### G. このセッションで意識的に「やらなかった」こと (= 次の人が判断する余地)

1. **per-project focus 履歴の controller meta 化** — B.5 で omniwm MRU 委譲を選んだので未実装
2. **identity.Resolve 自体への focus tiebreak embed** — B.3 で別 helper にしたので未実装
3. **scratch shell の "kill on hide" オプション** — SSOT 643 は「kill しない」と言ってるので未実装、ただしテストで明示してない (ユーザが「scratch は毎回 fresh で欲しい」と言ったら要再検討)
4. **darwin-rebuild switch 試行** — handoff 方針継承で deferral
5. **production daemon の再 bootstrap** — store G012038 のまま、私の修正は production 未反映

---

### H. 引き継ぎ手順 (次セッションでこのドキュメントを読んだ人へ)

1. **本ドキュメント §0ter を全部読む** (= you are here)
2. memory `[[project-projwm-overhaul]]` を読む (本ドキュメントと sync 済、被るが要点 recap)
3. `git log --oneline -15` で `f8ac9da..b7e5e94` の commit 流れを確認
4. `cd modules/darwin/projwm/projwm-next && go test ./...` で全 31 パッケージ green 確認
5. `queue/projwm-next-spec.md` v1.11 を再読 (2079 行) — 用語の内面化が必須
6. `queue/ssot-coverage-matrix.md` で残 unfilled 行を確認 (Phase 4-5 territory)
7. `queue/ssot-slice-plan.md` §4 DoD 13 条項を再読
8. **着手前に user に聞く**: F1-F4 のどれを選ぶか、または別案がないか。**自分の判断で進めない** ([[feedback-orchestrator-responsibility]] と整合)

---

## 0bis. 2026-05-23 SSOT-driven restructure (進行中)

過去の patchwork 実装 (要件 v2.6 cockpit/karabiner 中心 plan) は一旦凍結。
新 SSOT `queue/projwm-next-spec.md` v1.11 (2026-05-23) を起点に **上位
レイヤーから破壊的に書き直す** 方針にユーザ合意 (2026-05-23)。テストを
通すための bottom-up 修正は禁止、SSOT 整合のための top-down destructive
restructure のみ。

### 新 Phase plan (TaskCreate #1-#9 と一致)

- **Phase 0.1** [完了 2026-05-23]: intent / op / world 型レイヤー
  - intent.go を SSOT §4.1 17 ops に整列 (新規 12 intent kind: summon-shell/
    editor/browser, switch-project, cycle-slot-window, summon-viewer,
    show/hide-scratch-shell, browser-add/remove/change/reorder-tab)
  - N-06 で廃止された ToggleCockpit/FocusCockpit、N-12 で廃止された
    AcceptManualLayout、SyncBrowserTabs、RespawnOrphanGhostty に `Deprecated:`
    コメントを付けて intent.go に残置 (consumer cleanup は Phase 0.3 で実施)
  - WindowScratch を world.WindowKind に追加 (SSOT §7.4)
  - UnarchiveProject.TargetSlot を完全削除 (SSOT §4.5 park-state 復帰)
  - ssot_l0_intent_test を full kind allowlist 形式に書き換え

- **Phase 0.2** [完了 2026-05-23]: title contract / 命名規約
  - AI title から AI 名 (claude) 除去 → `ai-N:project` (SSOT §7.3)
  - viewer title 同様 → `ai-view-N:project`
  - cockpit title: `projwm-cockpit-D0` → `projwm-cockpit-0` (SSOT §7.3)
  - semop.terminalSessionFields の `displayID = Index+1` を `id = Index` に
    修正 (SSOT §7.3: 1 始まり stable ID をそのまま使う)
  - reducer の SetCockpitVisibility/ToggleCockpit が PriorWindow を
    state.Observed.Focus.Window から populate (SSOT §5.4 / §7.5)
  - planner の hide-cockpit op に PriorWindow → Target.LiveWindow 伝播

- **Phase 0.3** [完了 S02 で吸収 2026-05-24]: deprecated intent purge
  (5 種 intent type + reducer/planner/TUI/scenarios の consumer 全削除)。

## 29 切片計画

詳細: `queue/ssot-slice-plan.md`。29 切片の DoD 13 条項 (§4 参照)。

### Phase 1: 足場 [完了 2026-05-24]

- **S01** [完了]: manifest §10.7 shape 統一 (Go + Nix で top-level array)
- **S02** [完了]: deprecated intent purge (5 種完全削除、type + consumer 100%)
- **S03** [完了]: AI 名 routing 修正 (DesiredAISession field, semop.aiCommandFor)
- **S04** [完了]: ssottest ledger 真化 (workflow doc + 4 items behavior 昇格:
  NAMI-01, NAMI-02, MANI-01, OP-10B)
- **S05** [完了]: テスト環境分離 audit (TestSSOTTestIsolationAuditEnforcesPrefixes、
  ISO-01 ledger 登録、既存違反は S29 で fix 予定として allowlist)

Phase 1 残ギャップ:
- L4 acceptance テストの "dotfiles"/"manaflow" 使用 (S29 で解消)
- 残 red 3 件: TestSSOTCLIExposesBrowserTabOperations (S14)、
  TestSigWM_Close_CockpitBypassesBlock + TestFocusWindowNavigationBeforeFocus (S27)

### Phase 2 部分着手 [2026-05-24]

- **S06** [完了]: OP07 cockpit show/hide end-to-end
  - executor の hide-cockpit op に `FocusWindow(PriorWindow)` 復帰追加
  - L1 behavior test 2 件追加 (TestExecuteHideCockpitRestoresPriorWindowFocus
    + TestExecuteHideCockpitWithoutPriorWindowSkipsFocus)
  - OP-07 ledger を `statusRealOnly + evidenceBehavior` に昇格

### Phase 1 + S06 後の dead code / honest gap 整理 [2026-05-24]

- **ManualLayoutCandidate machinery 完全削除** (SSOT N-12 反映):
  - world.ManualLayoutCandidate type 削除
  - world.ControllerMeta.ManualLayoutCandidates field 削除
  - event.Reaction.ManualLayoutCandidate field 削除
  - controller.{captureObservedManualLayoutCandidate, hasManualLayoutCandidate,
    cloneManualLayoutCandidates} 削除
  - planner.hasManualLayoutCandidate 削除 + planner.go:252 skip-replan logic 撤去
  - invariant.CheckOptions.AllowManualLayoutCandidates + invariant.check9LayoutSemantics
    の short-circuit 削除
  - store.ControllerCheckpoint.ManualLayoutCandidates 削除
  - reducer.ReactToEvent の KindUserReorderedColumns ハンドラを ManualLayoutCandidate
    生成から layout-sync DirtyScope 発行に変更
  - controller.ApplyIntent に applyTier2AutoSyncLayout 呼び出しを追加
    (drainUserEvents 由来の layout-sync DirtyScope を user intent 経路でも消化)
  - observedColumnsForProject の guard を「ws 上の windows 数」基準に変更
    (viewer は viewer workspace に居るので slot workspace の guard には含めない)
  - S8.D test を ManualLayoutCandidate 検証から AcceptedLayouts mutation 検証に rewrite
  - S8.E test の UserReorderedColumns ケース除外 (Tier 2 経路は意図的に DesiredWorld
    を書く)
  - scenario/acceptance.go の S8.D / S8.F / EVT.4.4 description を N-12 に整合

- **observer/browser_tabs 機能停止** (一時的、S14/S20 で proper 実装):
  - SyncBrowserTabs intent 削除に伴い emit body を no-op に
  - InspectTabs によるスナップショット観測のみ継続 (changeFire 無し)
  - 結果として **browser tab の自動観測機能は事実上停止中**。
    granular な BrowserAddTab/RemoveTab/ChangeTabURL/ReorderTabs intents は
    S14 (browser tab CRUD) + S20 (observer sidecar) で実装予定

### Phase 2: §4.1 17 user ops end-to-end [pending]

S06 cockpit show/hide 完成 / S07 scratch shell / S08 viewer / S09 summon-*/
S10 switch-project / S11 cycle-slot / S12 add-window / S13 remove-window /
S14 browser tab CRUD / S15 OP08-10 既存 audit。

### Phase 3-5 [pending]

S16-S20 不変条件+障害復帰+observer / S21-S25 §5 UI / S26-S29 横串完成。

### 進捗指標

| パッケージ | Phase 1 後 |
|---|---|
| internal/intent | 0 red |
| internal/reducer | 0 red |
| internal/planner | 0 red |
| internal/semop | 0 red |
| internal/naming | 0 red |
| internal/manifest | 0 red |
| internal/clientauth | 0 red |
| internal/cockpitsnap | 0 red |
| internal/ssottest | 0 red |
| cmd/projwm | 1 red (S14 territory) |
| cmd/projwmd | 0 red |
| internal/adapter/wm | 2 red (S27 territory) |
| nix build | green |

### ユーザの今回明示した方針 (2026-05-23)

- 「テストを通すための実装」は嫌う。**SSOT に基づいた、最上層からの
  破壊的修正** が好み
- 既存実装はパッチワーク済み → 残すべき足場は無く、SSOT に合わない
  ものは積極的に破壊して良い
- テストも SSOT に合わない expectation を持つものは捨てる対象
- 「実装が未達で red」は許容、「テストが空虚で green」は不可
  (SSOT §10.9 行 5)

---

---

## 0. 同一性 (Orchestrator Stance)

私は **projwm-next + omniwm の overhaul を行う orchestrator** である。ユーザに
代わって sub-agent を駆使しながら、全体像と要件遵守の責任を負う。

### 私の責任 (ユーザに何度も求められたこと)

1. **要件文書を唯一の真実として扱う** — `queue/projwm-cockpit-requirements.md` (v2.6 現行)
2. **完璧主義で厳しい基準** — 動くだけでは不可、要件に対して 1 行ずつ検証
3. **sub-agent に丸投げしない** — 結果を字面で信用せず、必ず実機 + コードで
   独立検証する。不一致があれば re-dispatch
4. **完了マークは自分が verify してから** — 「全 pass」を agent が言っても、
   私が要件と実機状態を突き合わせるまで完了にしない
5. **責任を取る** — タスクの最終状態に責任を持つ。途中で止めずやり遂げる
6. **時間とトークンを贅沢に使う** — ユーザは「石油王で富豪」と言った、
   コスト懸念なく深く考え、何度でも sub-agent を回す

### ユーザの好み・スタイル

- 日本語で対話。技術用語は英語混在 OK
- 簡潔だが情報密度高めの応答を好む
- 部分的成功を「成功」と報告するのは嫌う。**ギャップを honest に列挙**
- 仕様改訂は要件文書側に必ず反映 (v2.X で改版履歴を残す)
- git commit は user が明示的に求めるまでしない

---

## 1. プロジェクト概要

**projwm-next + omniwm = macOS 上の AI コーディング workspace 管理基盤**

- **omniwm** (`/opt/homebrew/Caskroom/omniwm`): scrollable tiling window manager (niri-inspired)
- **projwm-next** (`modules/darwin/projwm/projwm-next/`): omniwm 上で AI/shell/editor windows を 1 project=1 slot で管理する Go 製 daemon + CLI + TUI
- **karabiner-elements**: space-leader keybinding を提供
- **3 layer 構成**:
  - Layer 1: `projwmctl-next` (低レベル IPC client、debug 用)
  - Layer 2: `projwm` (CLI、user-facing)
  - Layer 3: `projwm tui` (常駐 TUI cockpit、projwm-managed monitor に 1 つ常駐)

### 主要な要件文書

| ファイル | 役割 |
|---|---|
| `queue/projwm-cockpit-requirements.md` (v2.6) | **唯一の真実**。projwm-cockpit/CLI 統合要件 |
| `queue/projwm-cockpit-unified-design.md` (v3 partial) | 実装統合設計 (要件 v2.6 への追従未完) |
| `queue/projwm-cockpit-implementation-design.md` (v3, **古い**) | 旧設計 (park-workspace + shell scripts、現実装と乖離あり) |
| `queue/projwm-spec.md` | 元 projwm 仕様 |
| `queue/projwm-ux.md` | UX ストーリー |
| `queue/orchestration-state.md` | **本ファイル**、作業状態 snapshot |

---

## 2. 確定したアーキテクチャ決定 (v2.6 までの帰結)

### 2.1 Karabiner (要件 §11.1 v2.5)

**設計**: variable_if + to_if_alone (time-limit 無し)

```nix
spaceLeader = {
  from = { key_code = "spacebar"; modifiers = { optional = []; }; };
  parameters = { "basic.to_if_alone_timeout_milliseconds" = 5000; };
  to = [{ set_variable = { name = "space_held"; value = 1; }; }];
  to_after_key_up = [{ set_variable = { name = "space_held"; value = 0; }; }];
  to_if_alone = [{ key_code = "spacebar"; }];
};
```

`optional = []` のおかげで modifier+space (ctrl+space etc.) は素通し、race ゼロ。
letter→space ロール打鍵も variable=0 中なので無干渉。

### 2.2 Keybinding 体系 (要件 §11.3 / §11.6 v2.6)

**原則:**
- **位置 / focus / move 系 → space-leader** (karabiner で実装、対応 OmniWM 内蔵を Unassigned)
- **size / 構造 / UI 系 → option 維持** (OmniWM 内蔵で直接ハンドル)

#### Space-base (47 個 + 既存 44 個 = 91 binding)

```
# workspace 切替
space + 1..9                          → workspace 1..9
space + q,w,e,r,t,y,u,i,o,p           → projwm slot Q-P
space + a/m/b                         → workspace A/M/B
space + tab                           → switch-workspace back-and-forth
space + ]                             → switch-workspace next
space + [                             → switch-workspace prev

# window move (workspace 並び替え)
space + shift + 1..9 / letter         → move-to-workspace
space + shift + ]/[                   → move-to-workspace down/up

# focus 方向 (within workspace) — vim hjkl
space + h/j/k/l                       → command focus left/down/up/right
space + ;                             → command focus previous

# focus column
space + ctrl + 1..9                   → command focus-column 0..8 (0-based)
space + ctrl + [/]                    → command focus-column first/last

# focus monitor
space + ctrl + h/j/k/l                → omniwm-focus-monitor-dir left/down/up/right
space + ctrl + tab                    → command focus-monitor next

# window move 方向
space + shift + h/j/k/l               → command move left/down/up/right

# column move 方向
space + ctrl + shift + h/l            → command move-column left/right

# column → workspace 並び替え
space + ctrl + shift + 1..9           → command move-column-to-workspace 0..8
space + ctrl + shift + ]/[            → command move-column-to-workspace down/up

# 機能
space + f                             → projwm cockpit toggle
space + ctrl + m                      → omniwm-setup-media-workspace
space + s / space + c                 → ws-launch Spotify / Discord
```

#### Option-base (維持)

`Option+. / Option+,` (column 幅 cycle), `Option+= / Option+-` (set column width),
`Option+Shift+= / Option+Shift+-` (set window height), `Control+Option+F`
(expand column to available width), `Option+Shift+F` (toggle column full-width),
`Option+T` (toggle column tabbed), `Option+Return` (toggle fullscreen),
`Option+Shift+Space` (toggle floating), `Option+/` (balance sizes),
`Option+L` (toggle workspace layout), `Option+Shift+O` (toggle overview),
`Option+Shift+R` (raise all floating), `Control+Option+Shift+R` (rescue offscreen),
`Control+Option+R` (reset window height), `Control+Option+Space` (open command palette)

### 2.3 Cockpit (要件 §8 v2.4)

- **常駐 cockpit = projwm-managed monitor に 1 つだけ** (workspace A / Q-P が
  住むディスプレイ)
- spawn 方式: omniwm の app-rule で title=`projwm-cockpit-D0` を CP1 ワークスペースに
  bind。CP1 を所有するモニタは monitor-profiles で「workspace A と同じ display」
  に固定
- show: CP1 が active workspace になる (display switch)
- hide: 元の workspace に戻る (`omniwm-cockpit-show/hide` shell scripts は **廃止済み** —
  sigwm の `Show/HideCockpitOnDisplay` で omniwmctl 経由)
- visibility は SystemWindow.Visibility (Shown/Hidden) + PriorWorkspace で管理
- back-and-forth fallback: PriorWorkspace が空のとき omniwm の per-display
  back-and-forth history を使う

### 2.4 設計と現実の差分 (注意点)

| 項目 | 設計書 | 実装 |
|---|---|---|
| TUI ライブラリ | implementation-design T10 で `bubbletea v0.25+` 指定 | **使ってない** — 素の `fmt`/`io`/`strings` で文字列描画 |
| cockpit lifecycle | unified-design で「planner/sigwm 経由で 1 個」 | **多重起動** している (2 個以上が同時に live) |
| omniwm-cockpit-{show,hide} | unified-design で「削除」 | shell script は削除済だが TUI コードに deadcode 参照残存 |
| omniwm 内蔵 hotkeys | 要件で 144 全部の binding 定義 | hotkeys.nix で defaults+overrides 構造、v2.6 で位置/move 系を全 Unassigned 化 |

---

## 3. 現在のタスク状況

### 完了 (verify 済) ✅

- **A1 (G10 fix)**: cockpit を 1 個常駐に収束 (要件 §8.1 / §8.8) 🎯**実機 verify 済 2026-05-18**
  - `sigwm.ReapStaleCockpit` / `ShutdownCockpit` 追加 (sigwm.go 末尾)
  - `wm.CockpitReaper` interface (adapter.go) — optional capability
  - `cmd/projwmd/main.go`: startup 前に reap、shutdown 時 (`<-ctx.Done()` 後) に reap (5s timeout)
  - 実機 50 秒間: daemons=1, cockpit binary=1, cockpit ghostty=1, tmux 1 group 安定
- **A1+**: 旧 alt 系完全移行 (47 新規 space binding、205 OmniWM internal を Unassigned)
- **A2**: ctrl+space race 解消 (variable_if approach で構造的に消滅)
- **A3**: space 長押し modifier 永続化 (variable_if + 5000ms timeout)
- **B1**: cards spam (70件→0件、reducer + controller の 2 段 dedup)
- **Step B (§8.8 行 7)**: `projwm tui` を IPC-only 化 (cmd_tui.go 全書換) 🎯
  - 直接 binary 起動を廃止、`SetCockpitVisibility{Shown}` intent を送るだけ
  - daemon の planner が「無ければ spawn / あれば show」を判断 (planCockpitOps 既存)
  - 実機 verify: `projwm tui` 実行で count 変化なし、ok return
- **G5**: TUI 内 deadcode 削除 (main.go enterProposalMode, prompt.go hideCockpit) 🎯
  - `omniwm-cockpit-{show,hide}` 参照を `submitIntent(SetCockpitVisibility{...})` に置換
  - os/exec import も削除
- **Phase 2.1**: bubbletea 依存追加 + Nix vendorHash 反映 🎯
  - `go get github.com/charmbracelet/{bubbletea,bubbles,lipgloss}` 実行済
  - go.mod に v1.3.10/v1.0.0/v1.1.0 として追加
  - `default.nix` の vendorHash を null → `sha256-hFvi3LQol0cFtYTDDKJqGaWTZHm4bQxDEVEx5+1wenc=` に
  - `darwin-rebuild` deploy 成功

### B2 完了マークするが gaps 発見 (要件 §9 / §10 / §8.4-8.6 verification)

10 件のギャップ:
- **G1**: §10.1 カード発生時刻が view.go で表示されない (Card.CreatedAt は構造体にある)
- **G2**: §10.2 Esc=カード個別 dismiss が未実装 (現状 Esc → cockpit hide)
- **G3**: §10.2 letter alternate action key (c 等) が filter に吸われる
- **G4**: §9.5 t carry over の detail view が存在しない (status メッセージのみ)
- **G5**: `omniwm-cockpit-show/hide` の deadcode 参照が TUI 内 (main.go:222, prompt.go:200, 195)
- **G6**: `term.go` rawMode の `stty` に `-icrnl` 欠如 (Enter が ctrl-j 化する PTY あり)
- **G7**: §10.4 「最新カード上」順序の保証なし
- **G8**: §9.5 Esc 階層化が proposal mode で要件と乖離 (Mode 1 で元 visibility に戻る要件)
- **G9**: §5.9 `projwm status` 出力に convergence/digest/cards/park 不足
- **G10**: cockpit プロセス多重起動 (要件 §8.1「1 個」違反)。**A1 incomplete**

### 残タスク (pending)

- B3: socket エラーの再現と修正
- C1: Vivaldi / Zed コントロール問題 (原初の懸念)
- C2: windows on M クラスタリング観測
- D1: unified-design v3 文書反映
- D2: requirements v2.6 整合性チェック
- D3: queue/ gitignore 判断 (現状 ignored)
- D4: 意味あるまとまりで commit

### 追加されるべきタスク (ユーザ示唆)

- **TUI bubbletea migration** (大改修): 設計書 T10 に従い、現状の素描画から
  charmbracelet/bubbletea を導入。これで G1-G8 の多くが標準機能で解消可能
  - **進捗**: ユーザ承認済 (2026-05-18) — Plan 永続化 (`queue/projwm-cockpit-bubbletea-plan.md`)
  - Phase 2.1 (依存追加) 完了
  - Phase 2.2 以降 (skeleton, components 実装) は次セッションで継続
- ~~G10 cockpit multi-instance 解消~~ → **A1 で完了** (2026-05-18)

---

## 4. 次のアクション計画 (Phase ベース)

### Phase 1: Foundation cleanup ✅ 完了 (2026-05-18)

- A1 (G10): cockpit lifecycle reaper 実装 + 実機 verify
- Step B: `projwm tui` を IPC-only 化
- G5: TUI 内 deadcode 削除

### Phase 2: TUI 全面再設計 (bubbletea migration) — 進行中

ユーザ承認済 (2026-05-18): 「後回しなし、bubbletea 導入は設計書通り」。
**詳細 plan は `queue/projwm-cockpit-bubbletea-plan.md` を参照** (永続化済)。

進捗:
- ✅ Phase 2.1: 依存追加 + Nix vendorHash 反映
- ⏳ Phase 2.2: tui/ skeleton (model/update/view) — 次セッション着手
- ⏳ Phase 2.3-2.7: components 実装
- ⏳ Phase 2.8: 旧コード削除

要件 §9 を 1 から再評価。設計書 T10 (bubbletea + tea.Program.Send) に従い、
ユーザの workflow / experience を中心に UI を組み立て直す。これで G1-G8
の大半が消える。

- UX 観点 (user's intent):
  - cockpit は projwm-managed monitor に 1 つ常駐
  - 通常は hidden、space+f で召喚
  - 召喚時すぐ操作開始 (filter / cursor / action key 即時応答)
  - カード/slot/project/profile を 1 画面で俯瞰
  - キーボードのみで完結 (mouse 不要)
- 設計タスク:
  - bubbletea の Model/Update/View でレンダリング再構築
  - keymap 専用パッケージ
  - カード/slot/window のリスト component
  - filter (fzf-like) component
  - modal prompt component
  - IPC subscribe による rerender 駆動

### Phase 3: 残機能ギャップ

- B3 socket エラー
- C1 Vivaldi/Zed
- C2 windows on M

### Phase 4: 文書 + commit

- D1 unified-design v3 完成
- D2 requirements v2.6 整合性
- D3 gitignore 判断
- D4 commit

---

## 5. Sub-agent 駆使方針

- 大規模変更 (TUI bubbletea migration 等) は Plan agent でまず構造を組み、
  Implementation agent で実装
- 各 agent の出力は **私が独立検証** してから完了マーク
  - 検証手順:
    1. agent の変更ファイル一覧を `git diff` で確認
    2. ビルド: `cd modules/darwin/projwm/projwm-next && go build ./...`
    3. テスト: `go test ./...`
    4. デプロイ: `git add -A; sudo darwin-rebuild switch --flake .#yuta`
    5. **実機で要件文書に書かれた挙動を私自身が観察**
- 不一致を見つけたら同じ agent に SendMessage で re-dispatch、または
  新 agent を投入

---

## 6. 環境情報

- macOS, `/Users/yuta/dev/dotfiles` リポジトリ (nix-darwin + home-manager)
- daemon socket: `/Users/yuta/.local/state/projwm-next/projwmd.sock`
- daemon logs: `/Users/yuta/.local/state/projwm-next/logs/projwmd.err.log`
- store: `/Users/yuta/.local/state/projwm-next/store/`
- karabiner config: `~/.config/karabiner/karabiner.json` (home-manager symlink)
- karabiner logs: `/var/log/karabiner/core_service.log`
- omniwm: `/Applications/omniwm.app`, CLI `/opt/homebrew/bin/omniwmctl`

### よく使うコマンド

```bash
# daemon restart
launchctl kickstart -k gui/$(id -u)/org.nixos.projwmd-next

# darwin-rebuild
cd /Users/yuta/dev/dotfiles && sudo darwin-rebuild switch --flake .#yuta

# テスト
cd /Users/yuta/dev/dotfiles/modules/darwin/projwm/projwm-next && go test ./...

# 状態確認
omniwmctl query displays --format json | python3 -m json.tool
omniwmctl query windows --bundle-id com.mitchellh.ghostty --format json
projwm status
projwm trace --last
```

---

## 7. 既知の罠 / 落とし穴

- **karabiner config の symlink**: home-manager が `~/.config/karabiner/karabiner.json`
  を symlink で更新するが、karabiner が fsevent でも reload しないことがある。
  shell の `cp -L` で実体に書き換えるか、karabiner-elements を再起動する
- **nix-darwin build cache**: 同じハッシュなら rebuild されない。
  カラビナルール等を変更しても、入力が変わらないと旧ビルドが使われる。
  作業中ファイル変更は `git add` してから darwin-rebuild する
- **projwm-cockpit wrapper PATH**: omniwm モジュールの projwmCli wrapper は
  `/etc/profiles/per-user/$USER/bin/projwm` フォールバックを含んでいないと、
  karabiner の launchd 環境では PATH 解決に失敗 (修正済、A1+ で fix)
- **omniwm scratchpad**: single-window pool。N 個 cockpit を共存させるのは
  実装不可 (v2 unified-design で確定、park-workspace 方式に移行済み)
- **viewer-set invariant**: AI window が DesiredWorld にあるなら viewer も
  observed 上で workspace A に居ること。M 等に取り残されると全 transaction が
  失敗する。B2 fix で planner が revert op を出すようになった (要件 §3.2 Tier 4)
- **TUI のフォーカス挙動**: show/hide で priorFocus を restore すると omniwm の
  focus-follows-window で display が再切替する。show/hide では focus 復元
  しない (sigwm.ShowCockpitOnDisplay)

---

## 8. このドキュメントの更新

context compaction が起きるたびに、または重要な進捗があるたびに、私
(orchestrator) はこのドキュメントを更新する。「Last updated」の日時を
書き換える。
